package mcp

import (
	"fmt"
	"os"

	"github.com/ghost-link/ghost-link/internal/config"
	ghostsolana "github.com/ghost-link/ghost-link/internal/solana"
	"github.com/ghost-link/ghost-link/internal/wallet"
)

// ServerConfig holds configuration for the MCP server, populated from
// Cobra flags and environment variables at startup.
type ServerConfig struct {
	PrivateKey string // --private-key or GHOSTLINK_PRIVATE_KEY
	Password   string // --password or GHOSTLINK_PASSWORD
	RPCURL     string // resolved RPC URL
	TorEnabled bool
	TorProxy   string
}

// resolveWallet returns a wallet using the priority chain:
// per-call private_key param > cfg.PrivateKey > wallet file with cfg.Password.
// Never prompts interactively.
func resolveWallet(cfg *ServerConfig, perCallPrivateKey string) (*wallet.Wallet, error) {
	// Per-call override
	key := perCallPrivateKey
	if key == "" {
		key = cfg.PrivateKey
	}
	if key == "" {
		key = os.Getenv("GHOSTLINK_PRIVATE_KEY")
	}
	if key != "" {
		return wallet.FromPrivateKey(key)
	}

	// Fall back to wallet file
	path, err := wallet.DefaultWalletPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	encrypted, err := wallet.IsEncrypted(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet: %w", err)
	}

	pw := cfg.Password
	if pw == "" {
		pw = os.Getenv("GHOSTLINK_PASSWORD")
	}

	if encrypted && pw == "" {
		return nil, fmt.Errorf("wallet is encrypted; provide private_key param, --password flag, or GHOSTLINK_PASSWORD env")
	}

	w, err := wallet.Load(path, pw)
	if err != nil {
		return nil, fmt.Errorf("failed to load wallet: %w", err)
	}
	return w, nil
}

// resolveClient creates a Solana RPC client using the priority chain:
// per-call rpc_url param > cfg.RPCURL > config file > default devnet.
func resolveClient(cfg *ServerConfig, perCallRPCURL string) (*ghostsolana.Client, error) {
	rpcURL := perCallRPCURL
	if rpcURL == "" {
		rpcURL = cfg.RPCURL
	}
	if rpcURL == "" {
		fileCfg, _ := config.Load()
		rpcURL = fileCfg.GetRPCURL("")
	}

	proxyAddr := ""
	if cfg.TorEnabled {
		proxyAddr = cfg.TorProxy
	}

	client, err := ghostsolana.NewClient(rpcURL, proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create Solana client: %w", err)
	}
	return client, nil
}
