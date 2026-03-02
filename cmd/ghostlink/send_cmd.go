package main

import (
	"crypto/ed25519"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ghost-link/ghost-link/internal/config"
	ghostcrypto "github.com/ghost-link/ghost-link/internal/crypto"
	"github.com/ghost-link/ghost-link/internal/exitcode"
	"github.com/ghost-link/ghost-link/internal/message"
	"github.com/ghost-link/ghost-link/internal/output"
	ghostsolana "github.com/ghost-link/ghost-link/internal/solana"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an encrypted message",
	Long:  "Encrypt a message with the recipient's public key and send it via Solana Memo.",
	RunE:  runSend,
}

var (
	sendTo      string
	sendMessage string
	sendStdin   bool
	sendWait    bool
	sendTimeout int
	sendType    string
	sendReplyTo string
)

func init() {
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Recipient address (Solana public key)")
	sendCmd.Flags().StringVarP(&sendMessage, "message", "m", "", "Message content (plaintext)")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read message from stdin")
	sendCmd.Flags().BoolVar(&sendWait, "wait", false, "Wait for transaction confirmation")
	sendCmd.Flags().IntVar(&sendTimeout, "timeout", 30, "Confirmation timeout in seconds (with --wait)")
	sendCmd.Flags().StringVar(&sendType, "type", "text", "Envelope message type")
	sendCmd.Flags().StringVar(&sendReplyTo, "reply-to", "", "Transaction signature being replied to")
	sendCmd.MarkFlagRequired("to")
	rootCmd.AddCommand(sendCmd)
}

func runSend(cmd *cobra.Command, args []string) error {
	// Read message from -m or --stdin
	var msgText string
	switch {
	case sendStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		msgText = strings.TrimRight(string(data), "\n")
	case sendMessage != "":
		msgText = sendMessage
	default:
		return fmt.Errorf("provide a message with -m or --stdin")
	}

	// Wrap in envelope
	env := &message.Envelope{
		V:       1,
		Type:    sendType,
		Body:    msgText,
		ReplyTo: sendReplyTo,
	}
	envBytes, err := message.Encode(env)
	if err != nil {
		return fmt.Errorf("failed to encode envelope: %w", err)
	}

	// Validate message size
	maxSize := ghostcrypto.MaxMessageSize()
	if len(envBytes) > maxSize {
		return exitcode.Wrap(exitcode.MessageTooLarge,
			fmt.Errorf("message too long: %d bytes (envelope), max %d bytes", len(envBytes), maxSize))
	}

	// Load wallet
	w, err := getWalletFromFlags()
	if err != nil {
		return err
	}

	// Parse recipient address
	recipientPubKey, err := solanago.PublicKeyFromBase58(sendTo)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	// Encrypt message
	senderPrivKey := ed25519.PrivateKey(w.PrivateKey())
	recipientEdPubKey := ed25519.PublicKey(recipientPubKey[:])

	encrypted, err := ghostcrypto.Encrypt(envBytes, recipientEdPubKey, senderPrivKey)
	if err != nil {
		return exitcode.Wrap(exitcode.MessageTooLarge, fmt.Errorf("failed to encrypt message: %w", err))
	}

	// Determine proxy
	cfg, _ := config.Load()
	proxyAddr := ""
	if torFlag || cfg.TorEnabled {
		proxyAddr = torProxy
		if proxyAddr == "" {
			proxyAddr = cfg.TorProxy
		}
		output.Status("Sending via Tor proxy...")
	}

	// Create Solana client
	rpcURL := cfg.GetRPCURL(urlFlag)
	client, err := ghostsolana.NewClient(rpcURL, proxyAddr)
	if err != nil {
		return exitcode.Wrap(exitcode.Network, fmt.Errorf("failed to create Solana client: %w", err))
	}

	// Send memo transaction
	output.Status("Sending encrypted message...")
	txSig, err := client.SendMemo(senderPrivKey, recipientPubKey, encrypted)
	if err != nil {
		return exitcode.Wrap(exitcode.Network, fmt.Errorf("failed to send message: %w", err))
	}

	// Optionally wait for confirmation
	confirmed := false
	if sendWait {
		output.Status("Waiting for confirmation...")
		timeout := time.Duration(sendTimeout) * time.Second
		if err := client.ConfirmTransaction(txSig, timeout); err != nil {
			return exitcode.Wrap(exitcode.Network, fmt.Errorf("confirmation failed: %w", err))
		}
		confirmed = true
	}

	result := map[string]interface{}{
		"signature":      txSig,
		"recipient":      sendTo,
		"size":           len(envBytes),
		"encrypted_size": len(encrypted),
		"confirmed":      confirmed,
	}

	output.PrintResult(result, func() {
		fmt.Println("Message sent successfully!")
		fmt.Printf("Signature: %s\n", txSig)
		fmt.Printf("Recipient: %s\n", sendTo)
		fmt.Printf("Size:      %d bytes (encrypted %d bytes)\n", len(envBytes), len(encrypted))
		if confirmed {
			fmt.Println("Status:    confirmed")
		}
	})

	return nil
}
