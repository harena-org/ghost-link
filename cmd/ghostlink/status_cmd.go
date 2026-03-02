package main

import (
	"fmt"
	"os"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/ghost-link/ghost-link/internal/config"
	ghostcrypto "github.com/ghost-link/ghost-link/internal/crypto"
	"github.com/ghost-link/ghost-link/internal/output"
	ghostsolana "github.com/ghost-link/ghost-link/internal/solana"
	"github.com/ghost-link/ghost-link/internal/wallet"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status",
	Long:  "Display wallet, network, and configuration status.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	rpcURL := cfg.GetRPCURL(urlFlag)

	result := map[string]interface{}{
		"rpc_url":          rpcURL,
		"rpc_reachable":    false,
		"wallet_exists":    false,
		"wallet_address":   "",
		"balance_sol":      0.0,
		"balance_lamports": uint64(0),
		"default_inbox":    cfg.DefaultInbox,
		"tor_enabled":      torFlag || cfg.TorEnabled,
		"tor_proxy":        cfg.TorProxy,
		"max_message_size": ghostcrypto.MaxMessageSize(),
	}

	// Check RPC reachability
	proxyAddr := ""
	if torFlag || cfg.TorEnabled {
		proxyAddr = torProxy
		if proxyAddr == "" {
			proxyAddr = cfg.TorProxy
		}
	}

	client, clientErr := ghostsolana.NewClient(rpcURL, proxyAddr)

	// Check wallet
	walletPath, _ := wallet.DefaultWalletPath()
	if _, err := os.Stat(walletPath); err == nil {
		result["wallet_exists"] = true

		// Try to load wallet (non-interactively: only if flag/env provides key or password)
		w, err := tryLoadWalletNonInteractive(walletPath)
		if w != nil && err == nil {
			result["wallet_address"] = w.PublicKey()

			// Check balance if RPC is available
			if clientErr == nil {
				pubKey, pkErr := solanago.PublicKeyFromBase58(w.PublicKey())
				if pkErr == nil {
					balance, balErr := client.GetBalance(pubKey)
					if balErr == nil {
						result["rpc_reachable"] = true
						result["balance_sol"] = float64(balance) / 1e9
						result["balance_lamports"] = balance
					}
				}
			}
		}
	}

	// If we haven't confirmed reachability via balance, try a simple check
	if !result["rpc_reachable"].(bool) && clientErr == nil {
		// Try getting balance of a zero address as a health check
		zeroAddr := solanago.PublicKey{}
		_, err := client.GetBalance(zeroAddr)
		if err == nil {
			result["rpc_reachable"] = true
		}
	}

	output.PrintResult(result, func() {
		fmt.Println("GhostLink Status")
		fmt.Println("─────────────────────────────────────────")
		fmt.Printf("RPC URL:          %s\n", rpcURL)
		if result["rpc_reachable"].(bool) {
			fmt.Println("RPC Status:       reachable")
		} else {
			fmt.Println("RPC Status:       unreachable")
		}
		fmt.Println("─────────────────────────────────────────")
		if result["wallet_exists"].(bool) {
			fmt.Println("Wallet:           found")
			if addr, ok := result["wallet_address"].(string); ok && addr != "" {
				fmt.Printf("Address:          %s\n", addr)
				fmt.Printf("Balance:          %.9f SOL\n", result["balance_sol"])
			} else {
				fmt.Println("Address:          (locked, provide --password or --private-key)")
			}
		} else {
			fmt.Println("Wallet:           not found")
		}
		fmt.Println("─────────────────────────────────────────")
		if cfg.DefaultInbox != "" {
			fmt.Printf("Default Inbox:    %s\n", cfg.DefaultInbox)
		} else {
			fmt.Println("Default Inbox:    (none)")
		}
		if torFlag || cfg.TorEnabled {
			fmt.Printf("Tor:              enabled (%s)\n", cfg.TorProxy)
		} else {
			fmt.Println("Tor:              disabled")
		}
		fmt.Printf("Max Message Size: %d bytes\n", ghostcrypto.MaxMessageSize())
	})

	return nil
}

// tryLoadWalletNonInteractive attempts to load the wallet without prompting.
// Uses --private-key flag, GHOSTLINK_PRIVATE_KEY env, --password flag,
// or GHOSTLINK_PASSWORD env. Returns nil, nil if interactive prompt would be needed.
func tryLoadWalletNonInteractive(path string) (*wallet.Wallet, error) {
	// Check --private-key / env first
	key := privateKeyFlag
	if key == "" {
		key = os.Getenv("GHOSTLINK_PRIVATE_KEY")
	}
	if key != "" {
		return wallet.FromPrivateKey(key)
	}

	// Check if wallet is encrypted
	encrypted, err := wallet.IsEncrypted(path)
	if err != nil {
		return nil, err
	}

	if !encrypted {
		return wallet.Load(path, "")
	}

	// Encrypted: check flag/env for password
	pw := passwordFlag
	if pw == "" {
		pw = os.Getenv("GHOSTLINK_PASSWORD")
	}
	if pw != "" {
		return wallet.Load(path, pw)
	}

	// Would need interactive prompt — skip
	return nil, nil
}
