package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/ghost-link/ghost-link/internal/config"
	"github.com/ghost-link/ghost-link/internal/exitcode"
	"github.com/ghost-link/ghost-link/internal/output"
	ghostsolana "github.com/ghost-link/ghost-link/internal/solana"
	"github.com/ghost-link/ghost-link/internal/wallet"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Wallet management",
	Long:  "Manage Solana wallet keypairs, including create, import, and balance queries.",
}

var walletCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new wallet",
	Long:  "Generate a new Solana Ed25519 keypair and store it locally.",
	RunE:  runWalletCreate,
}

var walletImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import an existing wallet",
	Long:  "Import an existing Solana wallet via private key or mnemonic.",
	RunE:  runWalletImport,
}

var walletBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Check balance",
	Long:  "Query the SOL balance of the current wallet.",
	RunE:  runWalletBalance,
}

var walletAirdropCmd = &cobra.Command{
	Use:   "airdrop [amount]",
	Short: "Request test SOL (devnet only)",
	Long:  "Request test SOL from the Solana devnet faucet, default 1 SOL.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWalletAirdrop,
}

var (
	importKey      string
	importMnemonic string
	walletPath     string
)

func init() {
	walletCreateCmd.Flags().StringVar(&walletPath, "path", "", "Wallet file path (default ~/.ghostlink/wallet.json)")
	walletImportCmd.Flags().StringVar(&importKey, "key", "", "Base58-encoded private key")
	walletImportCmd.Flags().StringVar(&importMnemonic, "mnemonic", "", "BIP39 mnemonic phrase")
	walletImportCmd.Flags().StringVar(&walletPath, "path", "", "Wallet file path")
	walletBalanceCmd.Flags().StringVar(&walletPath, "path", "", "Wallet file path")

	walletAirdropCmd.Flags().StringVar(&walletPath, "path", "", "Wallet file path")

	walletCmd.AddCommand(walletCreateCmd)
	walletCmd.AddCommand(walletImportCmd)
	walletCmd.AddCommand(walletBalanceCmd)
	walletCmd.AddCommand(walletAirdropCmd)
	rootCmd.AddCommand(walletCmd)
}

// getPassword returns the wallet password from the priority chain:
// --password flag > GHOSTLINK_PASSWORD env > interactive prompt.
func getPassword(prompt string) (string, error) {
	if passwordFlag != "" {
		return passwordFlag, nil
	}
	if envPw := os.Getenv("GHOSTLINK_PASSWORD"); envPw != "" {
		return envPw, nil
	}
	return readPassword(prompt)
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		return strings.TrimSpace(string(password)), nil
	}
	// Non-terminal (piped input): read a line from stdin.
	var line string
	fmt.Scanln(&line)
	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(line), nil
}

func getWalletPath() (string, error) {
	if walletPath != "" {
		return walletPath, nil
	}
	return wallet.DefaultWalletPath()
}

// getWalletFromFlags returns a wallet using the priority chain:
// --private-key flag > GHOSTLINK_PRIVATE_KEY env > load from file (with password chain).
func getWalletFromFlags() (*wallet.Wallet, error) {
	// Check --private-key flag
	key := privateKeyFlag
	if key == "" {
		key = os.Getenv("GHOSTLINK_PRIVATE_KEY")
	}
	if key != "" {
		w, err := wallet.FromPrivateKey(key)
		if err != nil {
			return nil, exitcode.Wrap(exitcode.Auth, fmt.Errorf("invalid private key: %w", err))
		}
		return w, nil
	}

	// Fall back to wallet file
	path, err := getWalletPath()
	if err != nil {
		return nil, err
	}
	return loadWallet(path)
}

func loadWallet(path string) (*wallet.Wallet, error) {
	encrypted, err := wallet.IsEncrypted(path)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Auth, fmt.Errorf("failed to read wallet: %w", err))
	}

	var password string
	if encrypted {
		password, err = getPassword("Enter wallet password: ")
		if err != nil {
			return nil, err
		}
	}

	w, err := wallet.Load(path, password)
	if err != nil {
		return nil, exitcode.Wrap(exitcode.Auth, fmt.Errorf("failed to load wallet: %w", err))
	}
	return w, nil
}

func runWalletCreate(cmd *cobra.Command, args []string) error {
	path, err := getWalletPath()
	if err != nil {
		return err
	}

	// Check if wallet already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("wallet file already exists: %s\nUse --path to specify a different path", path)
	}

	// Generate mnemonic
	mnemonic, err := wallet.NewMnemonic()
	if err != nil {
		return fmt.Errorf("failed to generate mnemonic: %w", err)
	}

	// Create wallet from mnemonic
	w, err := wallet.FromMnemonic(mnemonic)
	if err != nil {
		return fmt.Errorf("failed to create wallet: %w", err)
	}

	// Read password (empty = no encryption)
	password, err := getPassword("Set wallet password (press Enter to skip, no encryption): ")
	if err != nil {
		return err
	}

	// Only confirm interactively when password was not provided via flag/env
	if password != "" && passwordFlag == "" && os.Getenv("GHOSTLINK_PASSWORD") == "" {
		confirmPassword, err := readPassword("Confirm password: ")
		if err != nil {
			return err
		}
		if password != confirmPassword {
			return fmt.Errorf("passwords do not match")
		}
	}

	// Save wallet
	if err := w.Save(path, password); err != nil {
		return fmt.Errorf("failed to save wallet: %w", err)
	}

	output.PrintResult(map[string]string{
		"address":  w.PublicKey(),
		"file":     path,
		"mnemonic": mnemonic,
	}, func() {
		fmt.Println("Wallet created successfully!")
		fmt.Println()
		fmt.Printf("Address: %s\n", w.PublicKey())
		fmt.Printf("File:    %s\n", path)
		fmt.Println()
		fmt.Println("Mnemonic (back up safely, cannot be recovered if lost):")
		fmt.Printf("  %s\n", mnemonic)
		fmt.Println()
		fmt.Println("WARNING: Never share your mnemonic with anyone!")
	})

	return nil
}

func runWalletImport(cmd *cobra.Command, args []string) error {
	path, err := getWalletPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("wallet file already exists: %s\nUse --path to specify a different path", path)
	}

	var w *wallet.Wallet

	switch {
	case importKey != "":
		w, err = wallet.FromPrivateKey(importKey)
		if err != nil {
			return fmt.Errorf("failed to import private key: %w", err)
		}
	case importMnemonic != "":
		w, err = wallet.FromMnemonic(importMnemonic)
		if err != nil {
			return fmt.Errorf("failed to import mnemonic: %w", err)
		}
	default:
		return fmt.Errorf("use --key or --mnemonic to specify import method")
	}

	password, err := getPassword("Set wallet password (press Enter to skip, no encryption): ")
	if err != nil {
		return err
	}

	if err := w.Save(path, password); err != nil {
		return fmt.Errorf("failed to save wallet: %w", err)
	}

	output.PrintResult(map[string]string{
		"address": w.PublicKey(),
		"file":    path,
	}, func() {
		fmt.Println("Wallet imported successfully!")
		fmt.Printf("Address: %s\n", w.PublicKey())
		fmt.Printf("File:    %s\n", path)
	})

	return nil
}

func runWalletBalance(cmd *cobra.Command, args []string) error {
	w, err := getWalletFromFlags()
	if err != nil {
		return err
	}

	cfg, _ := config.Load()
	rpcURL := cfg.GetRPCURL(urlFlag)

	output.Status("Querying balance...")
	balance, err := wallet.GetBalance(rpcURL, w.PublicKey())
	if err != nil {
		return exitcode.Wrap(exitcode.Network, fmt.Errorf("failed to query balance: %w", err))
	}

	solBalance := float64(balance) / 1e9

	output.PrintResult(map[string]interface{}{
		"address":         w.PublicKey(),
		"balance_sol":     solBalance,
		"balance_lamports": balance,
	}, func() {
		fmt.Printf("Address: %s\n", w.PublicKey())
		fmt.Printf("Balance: %.9f SOL (%d lamports)\n", solBalance, balance)
	})

	return nil
}

func runWalletAirdrop(cmd *cobra.Command, args []string) error {
	// Parse amount (default 1 SOL)
	amount := 1.0
	if len(args) > 0 {
		var err error
		amount, err = strconv.ParseFloat(args[0], 64)
		if err != nil || amount <= 0 {
			return fmt.Errorf("invalid amount: %s", args[0])
		}
		if amount > 2 {
			return fmt.Errorf("max 2 SOL per airdrop request")
		}
	}
	lamports := uint64(amount * 1e9)

	// Load wallet
	w, err := getWalletFromFlags()
	if err != nil {
		return err
	}

	pubKey, err := solanago.PublicKeyFromBase58(w.PublicKey())
	if err != nil {
		return fmt.Errorf("failed to parse wallet address: %w", err)
	}

	// Use devnet RPC for airdrop
	cfg, _ := config.Load()
	rpcURL := cfg.GetRPCURL(urlFlag)
	if rpcURL == "" {
		rpcURL = ghostsolana.DefaultDevnetRPC
	}

	output.Statusf("Requesting %.2f SOL airdrop from devnet...", amount)

	proxyAddr := ""
	if torFlag {
		cfg2, _ := config.Load()
		proxyAddr = torProxy
		if proxyAddr == "" {
			proxyAddr = cfg2.TorProxy
		}
	}

	solClient, err := ghostsolana.NewClient(rpcURL, proxyAddr)
	if err != nil {
		return exitcode.Wrap(exitcode.Network, fmt.Errorf("failed to create client: %w", err))
	}

	sig, err := solClient.RequestAirdrop(pubKey, lamports)
	if err != nil {
		return exitcode.Wrap(exitcode.Network, err)
	}

	output.PrintResult(map[string]interface{}{
		"signature":  sig,
		"address":    w.PublicKey(),
		"amount_sol": amount,
	}, func() {
		fmt.Println("Airdrop successful!")
		fmt.Printf("Signature: %s\n", sig)
		fmt.Printf("Address:   %s\n", w.PublicKey())
		fmt.Printf("Amount:    %.2f SOL\n", amount)
	})

	return nil
}
