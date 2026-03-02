package main

import (
	"errors"
	"os"

	"github.com/ghost-link/ghost-link/internal/exitcode"
	"github.com/ghost-link/ghost-link/internal/output"
	"github.com/spf13/cobra"
)

var (
	torFlag       bool
	torProxy      string
	configFile    string
	urlFlag       string
	jsonFlag      bool
	passwordFlag  string
	privateKeyFlag string
)

var rootCmd = &cobra.Command{
	Use:   "ghostlink",
	Short: "GhostLink - End-to-end encrypted P2P messaging on Solana",
	Long: `GhostLink
End-to-end encrypted peer-to-peer messaging tool built on the Solana blockchain.
Uses on-chain Memo storage and NaCl box asymmetric encryption for fully private messaging.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		output.JSONMode = jsonFlag
	},
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolVar(&torFlag, "tor", false, "Route requests through Tor proxy")
	rootCmd.PersistentFlags().StringVar(&torProxy, "tor-proxy", "127.0.0.1:9050", "Tor SOCKS5 proxy address")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path")
	rootCmd.PersistentFlags().StringVarP(&urlFlag, "url", "u", "", "Solana RPC (devnet, testnet, mainnet or custom URL)")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output results as JSON")
	rootCmd.PersistentFlags().StringVar(&passwordFlag, "password", "", "Wallet password (avoids interactive prompt)")
	rootCmd.PersistentFlags().StringVar(&privateKeyFlag, "private-key", "", "Base58-encoded private key (bypasses wallet file)")
}

func main() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		code := exitcode.General
		var exitErr *exitcode.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.Code
		}
		output.PrintError(err)
		os.Exit(code)
	}
}
