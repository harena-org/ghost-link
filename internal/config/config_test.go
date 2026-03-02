package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RPCURL != DefaultRPCURL {
		t.Errorf("expected RPC URL %s, got %s", DefaultRPCURL, cfg.RPCURL)
	}
	if cfg.TorEnabled {
		t.Error("expected Tor disabled by default")
	}
	if cfg.TorProxy != "127.0.0.1:9050" {
		t.Errorf("expected default tor proxy, got %s", cfg.TorProxy)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Override home dir for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := DefaultConfig()
	cfg.RPCURL = "https://test.example.com"
	cfg.TorEnabled = true

	err := cfg.Save()
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, DefaultConfigDir, DefaultConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file not created")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.RPCURL != "https://test.example.com" {
		t.Errorf("expected loaded RPC URL, got %s", loaded.RPCURL)
	}
	if !loaded.TorEnabled {
		t.Error("expected Tor enabled in loaded config")
	}
}

func TestGetRPCURL_NetworkName(t *testing.T) {
	cfg := DefaultConfig()
	url := cfg.GetRPCURL("mainnet")
	if url != "https://api.mainnet-beta.solana.com" {
		t.Errorf("expected mainnet URL, got %s", url)
	}
}

func TestGetRPCURL_CustomURL(t *testing.T) {
	cfg := DefaultConfig()
	url := cfg.GetRPCURL("https://my-rpc.example.com")
	if url != "https://my-rpc.example.com" {
		t.Errorf("expected custom RPC URL, got %s", url)
	}
}

func TestGetRPCURL_FlagOverridesConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Network = "testnet"
	url := cfg.GetRPCURL("mainnet")
	if url != "https://api.mainnet-beta.solana.com" {
		t.Errorf("expected flag to override config, got %s", url)
	}
}

func TestGetRPCURL_ConfigNetwork(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Network = "testnet"
	url := cfg.GetRPCURL("")
	if url != "https://api.testnet.solana.com" {
		t.Errorf("expected testnet URL, got %s", url)
	}
}

func TestGetRPCURL_FallbackToRPCURL(t *testing.T) {
	cfg := &Config{RPCURL: "https://custom-rpc.example.com"}
	url := cfg.GetRPCURL("")
	if url != "https://custom-rpc.example.com" {
		t.Errorf("expected custom RPC URL, got %s", url)
	}
}

func TestGetRPCURL_DefaultDevnet(t *testing.T) {
	cfg := DefaultConfig()
	url := cfg.GetRPCURL("")
	if url != "https://api.devnet.solana.com" {
		t.Errorf("expected devnet URL, got %s", url)
	}
}

func TestDefaultConfigNetwork(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Network != DefaultNetwork {
		t.Errorf("expected default network %s, got %s", DefaultNetwork, cfg.Network)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RPCURL != DefaultRPCURL {
		t.Error("expected default config for non-existent file")
	}
}
