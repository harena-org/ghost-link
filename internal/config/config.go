package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultRPCURL     = "https://api.devnet.solana.com"
	DefaultConfigDir  = ".ghostlink"
	DefaultConfigFile = "config.json"
	DefaultNetwork    = "devnet"
)

// NetworkRPCURLs maps network names to their Solana RPC endpoints.
var NetworkRPCURLs = map[string]string{
	"devnet":  "https://api.devnet.solana.com",
	"testnet": "https://api.testnet.solana.com",
	"mainnet": "https://api.mainnet-beta.solana.com",
}

type Config struct {
	Network      string `json:"network"`
	RPCURL       string `json:"rpc_url"`
	WalletPath   string `json:"wallet_path"`
	TorEnabled   bool   `json:"tor_enabled"`
	TorProxy     string `json:"tor_proxy"`
	DefaultInbox string `json:"default_inbox"`
}

// GetRPCURL returns the RPC URL based on the --url flag and config.
// The urlFlag can be a network name (devnet, testnet, mainnet) or a full URL.
// Priority: --url flag > config network > config rpc_url > default.
func (c *Config) GetRPCURL(urlFlag string) string {
	if urlFlag != "" {
		if url, ok := NetworkRPCURLs[urlFlag]; ok {
			return url
		}
		return urlFlag
	}
	if c.Network != "" {
		if url, ok := NetworkRPCURLs[c.Network]; ok {
			return url
		}
	}
	if c.RPCURL != "" {
		return c.RPCURL
	}
	return DefaultRPCURL
}

func DefaultConfig() *Config {
	return &Config{
		Network:    DefaultNetwork,
		RPCURL:     DefaultRPCURL,
		TorEnabled: false,
		TorProxy:   "127.0.0.1:9050",
	}
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, DefaultConfigDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func Load() (*Config, error) {
	dir, err := ConfigDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	path := filepath.Join(dir, DefaultConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, DefaultConfigFile), data, 0600)
}
