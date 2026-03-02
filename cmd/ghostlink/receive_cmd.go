package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ghost-link/ghost-link/internal/config"
	ghostcrypto "github.com/ghost-link/ghost-link/internal/crypto"
	"github.com/ghost-link/ghost-link/internal/exitcode"
	"github.com/ghost-link/ghost-link/internal/inbox"
	"github.com/ghost-link/ghost-link/internal/message"
	"github.com/ghost-link/ghost-link/internal/output"
	ghostsolana "github.com/ghost-link/ghost-link/internal/solana"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/spf13/cobra"
)

var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive and decrypt messages",
	Long:  "Scan on-chain transactions, decrypt and display received messages.",
	RunE:  runReceive,
}

var (
	receiveInbox string
	receiveLimit int
	receiveSince string
	receiveWatch int
)

func init() {
	receiveCmd.Flags().StringVar(&receiveInbox, "inbox", "", "Inbox name or address")
	receiveCmd.Flags().IntVar(&receiveLimit, "limit", 20, "Max number of messages to return")
	receiveCmd.Flags().StringVar(&receiveSince, "since", "", "Filter by start date (format: YYYY-MM-DD)")
	receiveCmd.Flags().IntVar(&receiveWatch, "watch", 0, "Polling interval in seconds (0=disabled)")
	rootCmd.AddCommand(receiveCmd)
}

// decryptedMessage holds a decrypted message with envelope data for output.
type decryptedMessage struct {
	From      string            `json:"from"`
	Time      string            `json:"time,omitempty"`
	Signature string            `json:"signature"`
	Type      string            `json:"type,omitempty"`
	Body      string            `json:"body"`
	ReplyTo   string            `json:"reply_to,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

func runReceive(cmd *cobra.Command, args []string) error {
	var targetAddr solanago.PublicKey
	var recipientPrivKey ed25519.PrivateKey

	// Fall back to default_inbox from config
	inboxName := receiveInbox
	if inboxName == "" {
		cfg, _ := config.Load()
		inboxName = cfg.DefaultInbox
	}

	if inboxName != "" {
		// Look up inbox by name or address
		store, err := inbox.LoadStore()
		if err != nil {
			return fmt.Errorf("failed to load inbox data: %w", err)
		}

		var inboxEntry *inbox.Entry
		for i, ib := range store.Inboxes {
			if ib.Name == inboxName || ib.Address == inboxName {
				inboxEntry = &store.Inboxes[i]
				break
			}
		}
		if inboxEntry == nil {
			return fmt.Errorf("inbox %q not found, create one with: inbox create", inboxName)
		}

		targetAddr, err = solanago.PublicKeyFromBase58(inboxEntry.Address)
		if err != nil {
			return fmt.Errorf("invalid inbox address: %w", err)
		}

		// Load inbox private key for decryption
		dir, err := config.ConfigDir()
		if err != nil {
			return err
		}
		inboxKeyPath := filepath.Join(dir, "inbox_"+inboxEntry.Name+".json")
		inboxWallet, err := loadWallet(inboxKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load inbox key: %w", err)
		}
		recipientPrivKey = ed25519.PrivateKey(inboxWallet.PrivateKey())
	} else {
		// Use main wallet (via flag priority chain)
		w, err := getWalletFromFlags()
		if err != nil {
			return err
		}
		targetAddr, err = solanago.PublicKeyFromBase58(w.PublicKey())
		if err != nil {
			return fmt.Errorf("failed to parse wallet address: %w", err)
		}
		recipientPrivKey = ed25519.PrivateKey(w.PrivateKey())
	}

	// Parse since filter
	var sinceTime time.Time
	if receiveSince != "" {
		var err error
		sinceTime, err = time.Parse("2006-01-02", receiveSince)
		if err != nil {
			return fmt.Errorf("invalid date format (use YYYY-MM-DD): %w", err)
		}
	}

	// Determine proxy
	cfg, _ := config.Load()
	proxyAddr := ""
	if torFlag || cfg.TorEnabled {
		proxyAddr = torProxy
		if proxyAddr == "" {
			proxyAddr = cfg.TorProxy
		}
		output.Status("Receiving via Tor proxy...")
	}

	// Create Solana client
	rpcURL := cfg.GetRPCURL(urlFlag)
	client, err := ghostsolana.NewClient(rpcURL, proxyAddr)
	if err != nil {
		return exitcode.Wrap(exitcode.Network, fmt.Errorf("failed to create Solana client: %w", err))
	}

	if receiveWatch > 0 {
		return runReceiveWatch(client, targetAddr, recipientPrivKey, sinceTime)
	}

	return runReceiveSingle(client, targetAddr, recipientPrivKey, sinceTime)
}

func runReceiveSingle(client *ghostsolana.Client, targetAddr solanago.PublicKey, recipientPrivKey ed25519.PrivateKey, sinceTime time.Time) error {
	output.Statusf("Querying messages for %s...", targetAddr.String())
	memos, err := client.GetMemos(targetAddr, receiveLimit)
	if err != nil {
		return exitcode.Wrap(exitcode.Network, fmt.Errorf("failed to query messages: %w", err))
	}

	messages := decryptMemos(memos, recipientPrivKey, sinceTime)

	output.PrintResult(map[string]interface{}{
		"messages": messages,
	}, func() {
		if len(messages) == 0 {
			fmt.Println("No decryptable messages found.")
			return
		}
		for _, msg := range messages {
			printHumanMessage(msg)
		}
		fmt.Println("─────────────────────────────────────────")
		fmt.Printf("%d message(s) decrypted.\n", len(messages))
	})

	return nil
}

func runReceiveWatch(client *ghostsolana.Client, targetAddr solanago.PublicKey, recipientPrivKey ed25519.PrivateKey, sinceTime time.Time) error {
	seen := make(map[string]bool)

	// Set up signal handler for graceful exit
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	output.Statusf("Watching messages for %s (poll every %ds, Ctrl+C to stop)...", targetAddr.String(), receiveWatch)

	for {
		memos, err := client.GetMemos(targetAddr, receiveLimit)
		if err != nil {
			output.Statusf("Error fetching messages: %s", err)
		} else {
			messages := decryptMemos(memos, recipientPrivKey, sinceTime)
			for _, msg := range messages {
				if seen[msg.Signature] {
					continue
				}
				seen[msg.Signature] = true

				if output.JSONMode {
					// JSONL: one JSON object per line
					data, _ := json.Marshal(msg)
					fmt.Println(string(data))
				} else {
					printHumanMessage(msg)
				}
			}
		}

		select {
		case <-sigCh:
			output.Status("Stopped watching.")
			return nil
		case <-time.After(time.Duration(receiveWatch) * time.Second):
			// continue polling
		}
	}
}

func decryptMemos(memos []ghostsolana.MemoMessage, recipientPrivKey ed25519.PrivateKey, sinceTime time.Time) []decryptedMessage {
	var messages []decryptedMessage

	for _, memo := range memos {
		// Apply time filter
		if !sinceTime.IsZero() && memo.Timestamp.Before(sinceTime) {
			continue
		}

		// Try to decrypt
		senderPubKey := ed25519.PublicKey(memo.Sender[:])
		plaintext, err := ghostcrypto.Decrypt(memo.Data, senderPubKey, recipientPrivKey)
		if err != nil {
			continue
		}

		msg := decryptedMessage{
			From:      memo.Sender.String(),
			Signature: memo.Signature,
		}

		if !memo.Timestamp.IsZero() {
			msg.Time = memo.Timestamp.Format("2006-01-02 15:04:05")
		}

		// Try to decode as envelope
		env, _ := message.Decode(plaintext)
		if env != nil {
			msg.Type = env.Type
			msg.Body = env.Body
			msg.ReplyTo = env.ReplyTo
			msg.Meta = env.Meta
		} else {
			// Legacy raw text message
			msg.Body = string(plaintext)
		}

		messages = append(messages, msg)
	}

	return messages
}

func printHumanMessage(msg decryptedMessage) {
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("From:      %s\n", msg.From)
	if msg.Time != "" {
		fmt.Printf("Time:      %s\n", msg.Time)
	}
	fmt.Printf("Signature: %s\n", msg.Signature)
	if msg.Type != "" {
		fmt.Printf("Type:      %s\n", msg.Type)
	}
	if msg.ReplyTo != "" {
		fmt.Printf("Reply-To:  %s\n", msg.ReplyTo)
	}
	fmt.Printf("Message:   %s\n", msg.Body)
}
