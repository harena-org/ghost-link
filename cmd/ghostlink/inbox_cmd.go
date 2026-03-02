package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghost-link/ghost-link/internal/config"
	"github.com/ghost-link/ghost-link/internal/inbox"
	"github.com/ghost-link/ghost-link/internal/output"
	"github.com/ghost-link/ghost-link/internal/wallet"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Stealth inbox management",
	Long:  "Create and manage stealth inboxes, share addresses via QR code.",
}

var inboxCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a stealth inbox",
	Long:  "Generate a new inbox address. The address is generated locally for privacy.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInboxCreate,
}

var inboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all inboxes",
	RunE:  runInboxList,
}

var inboxShareCmd = &cobra.Command{
	Use:   "share [name]",
	Short: "Generate inbox QR code",
	Long:  "Generate a QR code for the inbox address, display in terminal or export as PNG.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInboxShare,
}

var inboxSetDefaultCmd = &cobra.Command{
	Use:   "set-default [name]",
	Short: "Set default inbox",
	Long:  "Set the default inbox, used by receive when --inbox is not specified.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInboxSetDefault,
}

var (
	shareOutput string // PNG output path
)

func init() {
	inboxShareCmd.Flags().StringVarP(&shareOutput, "output", "o", "", "Export QR code as PNG file")

	inboxCmd.AddCommand(inboxCreateCmd)
	inboxCmd.AddCommand(inboxListCmd)
	inboxCmd.AddCommand(inboxShareCmd)
	inboxCmd.AddCommand(inboxSetDefaultCmd)
	rootCmd.AddCommand(inboxCmd)
}

func runInboxCreate(cmd *cobra.Command, args []string) error {
	name := "default"
	if len(args) > 0 {
		name = args[0]
	}

	// Check for duplicate
	store, err := inbox.LoadStore()
	if err != nil {
		return fmt.Errorf("failed to load inbox data: %w", err)
	}

	for _, ib := range store.Inboxes {
		if ib.Name == name {
			return fmt.Errorf("inbox %q already exists", name)
		}
	}

	// Generate new keypair for inbox
	w, err := wallet.NewWallet()
	if err != nil {
		return fmt.Errorf("failed to generate inbox address: %w", err)
	}

	// Save inbox private key
	password, err := getPassword("Set inbox password (press Enter to skip, no encryption): ")
	if err != nil {
		return err
	}

	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	inboxKeyPath := filepath.Join(dir, "inbox_"+name+".json")

	if err := w.Save(inboxKeyPath, password); err != nil {
		return fmt.Errorf("failed to save inbox key: %w", err)
	}

	// Add to store
	entry := inbox.Entry{
		Name:      name,
		Address:   w.PublicKey(),
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	store.Inboxes = append(store.Inboxes, entry)
	if err := inbox.SaveStore(store); err != nil {
		return fmt.Errorf("failed to save inbox list: %w", err)
	}

	output.PrintResult(map[string]string{
		"name":     name,
		"address":  w.PublicKey(),
		"key_path": inboxKeyPath,
	}, func() {
		fmt.Println("Inbox created successfully!")
		fmt.Printf("Name:    %s\n", name)
		fmt.Printf("Address: %s\n", w.PublicKey())
		fmt.Printf("Key:     %s\n", inboxKeyPath)
		fmt.Println()
		fmt.Println("Use 'ghostlink inbox share' to generate a QR code.")
	})

	return nil
}

func runInboxList(cmd *cobra.Command, args []string) error {
	store, err := inbox.LoadStore()
	if err != nil {
		return fmt.Errorf("failed to load inbox data: %w", err)
	}

	cfg, _ := config.Load()

	type inboxListEntry struct {
		Name      string `json:"name"`
		Address   string `json:"address"`
		CreatedAt string `json:"created_at"`
		IsDefault bool   `json:"is_default"`
	}

	var entries []inboxListEntry
	for _, ib := range store.Inboxes {
		entries = append(entries, inboxListEntry{
			Name:      ib.Name,
			Address:   ib.Address,
			CreatedAt: ib.CreatedAt,
			IsDefault: cfg.DefaultInbox == ib.Name,
		})
	}

	output.PrintResult(map[string]interface{}{
		"inboxes": entries,
	}, func() {
		if len(store.Inboxes) == 0 {
			fmt.Println("No inboxes. Create one with: ghostlink inbox create")
			return
		}

		fmt.Println("Inboxes:")
		fmt.Println("─────────────────────────────────────────")
		for _, ib := range store.Inboxes {
			if cfg.DefaultInbox == ib.Name {
				fmt.Printf("Name:    %s (default)\n", ib.Name)
			} else {
				fmt.Printf("Name:    %s\n", ib.Name)
			}
			fmt.Printf("Address: %s\n", ib.Address)
			if ib.CreatedAt != "" {
				fmt.Printf("Created: %s\n", ib.CreatedAt)
			}
			fmt.Println("─────────────────────────────────────────")
		}
	})

	return nil
}

func runInboxSetDefault(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Verify inbox exists
	store, err := inbox.LoadStore()
	if err != nil {
		return fmt.Errorf("failed to load inbox data: %w", err)
	}

	found := false
	for _, ib := range store.Inboxes {
		if ib.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("inbox %q not found", name)
	}

	// Save to config
	cfg, _ := config.Load()
	cfg.DefaultInbox = name
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	output.PrintResult(map[string]interface{}{
		"name":    name,
		"success": true,
	}, func() {
		fmt.Printf("Set %q as default inbox.\n", name)
	})

	return nil
}

func runInboxShare(cmd *cobra.Command, args []string) error {
	name := "default"
	if len(args) > 0 {
		name = args[0]
	}

	store, err := inbox.LoadStore()
	if err != nil {
		return fmt.Errorf("failed to load inbox data: %w", err)
	}

	var target *inbox.Entry
	for i, ib := range store.Inboxes {
		if ib.Name == name {
			target = &store.Inboxes[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("inbox %q not found", name)
	}

	qrContent := fmt.Sprintf("ghostlink:%s", target.Address)

	if shareOutput != "" {
		// Export as PNG
		if err := qrcode.WriteFile(qrContent, qrcode.Medium, 256, shareOutput); err != nil {
			return fmt.Errorf("failed to generate QR code file: %w", err)
		}

		output.PrintResult(map[string]string{
			"name":    name,
			"address": target.Address,
			"qr_file": shareOutput,
		}, func() {
			fmt.Printf("QR code exported to: %s\n", shareOutput)
		})
	} else {
		output.PrintResult(map[string]string{
			"name":    name,
			"address": target.Address,
		}, func() {
			qr, err := qrcode.New(qrContent, qrcode.Medium)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate QR code: %s\n", err)
				return
			}
			fmt.Printf("QR code for inbox %q:\n", name)
			fmt.Printf("Address: %s\n\n", target.Address)
			fmt.Print(renderQRTerminal(qr))
		})
	}

	return nil
}

// renderQRTerminal renders a QR code as a square in the terminal.
// Each module uses 2 characters width × 1 line height with ANSI background
// colors, producing visually square modules (terminal chars are ~2:1 H:W).
func renderQRTerminal(qr *qrcode.QRCode) string {
	bitmap := qr.Bitmap()
	var buf strings.Builder
	for _, row := range bitmap {
		for _, black := range row {
			if black {
				buf.WriteString("\033[40m  \033[0m")
			} else {
				buf.WriteString("\033[47m  \033[0m")
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
