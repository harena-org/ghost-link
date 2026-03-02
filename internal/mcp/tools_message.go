package mcp

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"path/filepath"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/ghost-link/ghost-link/internal/config"
	ghostcrypto "github.com/ghost-link/ghost-link/internal/crypto"
	"github.com/ghost-link/ghost-link/internal/inbox"
	"github.com/ghost-link/ghost-link/internal/message"
	ghostsolana "github.com/ghost-link/ghost-link/internal/solana"
	"github.com/ghost-link/ghost-link/internal/wallet"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type sendMessageInput struct {
	To         string `json:"to" jsonschema:"recipient Solana address (base58 public key)"`
	Message    string `json:"message" jsonschema:"plaintext message content"`
	Type       string `json:"type,omitempty" jsonschema:"Envelope message type (default: text)"`
	ReplyTo    string `json:"reply_to,omitempty" jsonschema:"Transaction signature being replied to"`
	Wait       bool   `json:"wait,omitempty" jsonschema:"Wait for transaction confirmation"`
	Timeout    int    `json:"timeout,omitempty" jsonschema:"Confirmation timeout in seconds (default: 30)"`
	PrivateKey string `json:"private_key,omitempty" jsonschema:"Override sender private key"`
	RPCURL     string `json:"rpc_url,omitempty" jsonschema:"Override RPC URL"`
}

type receiveMessagesInput struct {
	Inbox      string `json:"inbox,omitempty" jsonschema:"Inbox name or address"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max messages to return (default: 20)"`
	Since      string `json:"since,omitempty" jsonschema:"Filter by start date (YYYY-MM-DD)"`
	PrivateKey string `json:"private_key,omitempty" jsonschema:"Override wallet private key"`
	RPCURL     string `json:"rpc_url,omitempty" jsonschema:"Override RPC URL"`
}

func registerMessageTools(server *mcpsdk.Server, cfg *ServerConfig) {
	// send_message
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "send_message",
		Description: "Encrypt and send a message to a recipient via Solana Memo transaction.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input sendMessageInput) (*mcpsdk.CallToolResult, any, error) {
		if input.To == "" {
			return nil, nil, fmt.Errorf("'to' is required")
		}
		if input.Message == "" {
			return nil, nil, fmt.Errorf("'message' is required")
		}

		msgType := input.Type
		if msgType == "" {
			msgType = "text"
		}

		env := &message.Envelope{
			V:       1,
			Type:    msgType,
			Body:    input.Message,
			ReplyTo: input.ReplyTo,
		}
		envBytes, err := message.Encode(env)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode envelope: %w", err)
		}

		w, err := resolveWallet(cfg, input.PrivateKey)
		if err != nil {
			return nil, nil, err
		}

		recipientPubKey, err := solanago.PublicKeyFromBase58(input.To)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid recipient address: %w", err)
		}

		senderPrivKey := ed25519.PrivateKey(w.PrivateKey())
		recipientEdPubKey := ed25519.PublicKey(recipientPubKey[:])

		encrypted, err := ghostcrypto.EncryptV1(envBytes, recipientEdPubKey, senderPrivKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encrypt message: %w", err)
		}

		client, err := resolveClient(cfg, input.RPCURL)
		if err != nil {
			return nil, nil, err
		}

		txSig, err := client.SendMemo(senderPrivKey, recipientPubKey, encrypted)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to send message: %w", err)
		}

		confirmed := false
		if input.Wait {
			timeout := input.Timeout
			if timeout <= 0 {
				timeout = 30
			}
			if err := client.ConfirmTransaction(txSig, time.Duration(timeout)*time.Second); err != nil {
				return nil, nil, fmt.Errorf("confirmation failed: %w", err)
			}
			confirmed = true
		}

		return nil, map[string]interface{}{
			"signature":      txSig,
			"recipient":      input.To,
			"size":           len(envBytes),
			"encrypted_size": len(encrypted),
			"confirmed":      confirmed,
		}, nil
	})

	// receive_messages
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "receive_messages",
		Description: "Fetch and decrypt messages from the blockchain. Uses main wallet or a named inbox.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input receiveMessagesInput) (*mcpsdk.CallToolResult, any, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 20
		}

		var targetAddr solanago.PublicKey
		var recipientPrivKey ed25519.PrivateKey

		inboxName := input.Inbox
		if inboxName == "" {
			fileCfg, _ := config.Load()
			inboxName = fileCfg.DefaultInbox
		}

		if inboxName != "" {
			store, err := inbox.LoadStore()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load inbox data: %w", err)
			}

			var inboxEntry *inbox.Entry
			for i, ib := range store.Inboxes {
				if ib.Name == inboxName || ib.Address == inboxName {
					inboxEntry = &store.Inboxes[i]
					break
				}
			}
			if inboxEntry == nil {
				return nil, nil, fmt.Errorf("inbox %q not found", inboxName)
			}

			targetAddr, err = solanago.PublicKeyFromBase58(inboxEntry.Address)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid inbox address: %w", err)
			}

			dir, err := config.ConfigDir()
			if err != nil {
				return nil, nil, err
			}
			inboxKeyPath := filepath.Join(dir, "inbox_"+inboxEntry.Name+".json")

			pw := cfg.Password
			if pw == "" {
				// Try loading without password (unencrypted inbox key)
				pw = ""
			}
			inboxWallet, err := wallet.Load(inboxKeyPath, pw)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load inbox key: %w", err)
			}
			recipientPrivKey = ed25519.PrivateKey(inboxWallet.PrivateKey())
		} else {
			w, err := resolveWallet(cfg, input.PrivateKey)
			if err != nil {
				return nil, nil, err
			}
			addr, err := solanago.PublicKeyFromBase58(w.PublicKey())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse wallet address: %w", err)
			}
			targetAddr = addr
			recipientPrivKey = ed25519.PrivateKey(w.PrivateKey())
		}

		var sinceTime time.Time
		if input.Since != "" {
			var err error
			sinceTime, err = time.Parse("2006-01-02", input.Since)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid date format (use YYYY-MM-DD): %w", err)
			}
		}

		client, err := resolveClient(cfg, input.RPCURL)
		if err != nil {
			return nil, nil, err
		}

		memos, err := client.GetMemos(targetAddr, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to query messages: %w", err)
		}

		messages := decryptMemos(memos, recipientPrivKey, sinceTime)

		return nil, map[string]interface{}{
			"messages": messages,
		}, nil
	})
}

type decryptedMsg struct {
	From      string            `json:"from"`
	Time      string            `json:"time,omitempty"`
	Signature string            `json:"signature"`
	Type      string            `json:"type,omitempty"`
	Body      string            `json:"body"`
	ReplyTo   string            `json:"reply_to,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

func decryptMemos(memos []ghostsolana.MemoMessage, recipientPrivKey ed25519.PrivateKey, sinceTime time.Time) []decryptedMsg {
	var messages []decryptedMsg

	for _, memo := range memos {
		if !sinceTime.IsZero() && memo.Timestamp.Before(sinceTime) {
			continue
		}

		senderPubKey := ed25519.PublicKey(memo.Sender[:])
		var plaintext []byte
		var err error
		if ghostcrypto.HasMagicPrefix(memo.Data) {
			plaintext, err = ghostcrypto.DecryptV1(memo.Data, senderPubKey, recipientPrivKey)
		} else {
			plaintext, err = ghostcrypto.Decrypt(memo.Data, senderPubKey, recipientPrivKey)
		}
		if err != nil {
			continue
		}

		msg := decryptedMsg{
			From:      memo.Sender.String(),
			Signature: memo.Signature,
		}

		if !memo.Timestamp.IsZero() {
			msg.Time = memo.Timestamp.Format("2006-01-02 15:04:05")
		}

		env, _ := message.Decode(plaintext)
		if env != nil {
			msg.Type = env.Type
			msg.Body = env.Body
			msg.ReplyTo = env.ReplyTo
			msg.Meta = env.Meta
		} else {
			msg.Body = string(plaintext)
		}

		messages = append(messages, msg)
	}

	return messages
}
