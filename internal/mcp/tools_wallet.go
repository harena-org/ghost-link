package mcp

import (
	"context"
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/ghost-link/ghost-link/internal/wallet"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type walletCreateInput struct {
	Password string `json:"password,omitempty" jsonschema:"Encryption password for the new wallet (empty = no encryption)"`
}

type walletImportInput struct {
	PrivateKey string `json:"private_key,omitempty" jsonschema:"Base58-encoded private key"`
	Mnemonic   string `json:"mnemonic,omitempty" jsonschema:"BIP39 mnemonic phrase"`
	Password   string `json:"password,omitempty" jsonschema:"Encryption password (empty = no encryption)"`
}

type walletBalanceInput struct {
	PrivateKey string `json:"private_key,omitempty" jsonschema:"Override wallet private key"`
	RPCURL     string `json:"rpc_url,omitempty" jsonschema:"Override RPC URL"`
}

type walletAirdropInput struct {
	Amount     float64 `json:"amount,omitempty" jsonschema:"Amount in SOL (default 1, max 2)"`
	PrivateKey string  `json:"private_key,omitempty" jsonschema:"Override wallet private key"`
	RPCURL     string  `json:"rpc_url,omitempty" jsonschema:"Override RPC URL"`
}

func registerWalletTools(server *mcpsdk.Server, cfg *ServerConfig) {
	// wallet_create
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "wallet_create",
		Description: "Create a new Solana wallet with Ed25519 keypair and BIP39 mnemonic.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input walletCreateInput) (*mcpsdk.CallToolResult, any, error) {
		mnemonic, err := wallet.NewMnemonic()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate mnemonic: %w", err)
		}

		w, err := wallet.FromMnemonic(mnemonic)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create wallet: %w", err)
		}

		path, err := wallet.DefaultWalletPath()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get wallet path: %w", err)
		}

		pw := input.Password
		if pw == "" {
			pw = cfg.Password
		}

		if err := w.Save(path, pw); err != nil {
			return nil, nil, fmt.Errorf("failed to save wallet: %w", err)
		}

		return nil, map[string]string{
			"address":  w.PublicKey(),
			"file":     path,
			"mnemonic": mnemonic,
		}, nil
	})

	// wallet_import
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "wallet_import",
		Description: "Import an existing Solana wallet from a private key or mnemonic phrase.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input walletImportInput) (*mcpsdk.CallToolResult, any, error) {
		var w *wallet.Wallet
		var err error

		switch {
		case input.PrivateKey != "":
			w, err = wallet.FromPrivateKey(input.PrivateKey)
		case input.Mnemonic != "":
			w, err = wallet.FromMnemonic(input.Mnemonic)
		default:
			return nil, nil, fmt.Errorf("provide either private_key or mnemonic")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to import wallet: %w", err)
		}

		path, err := wallet.DefaultWalletPath()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get wallet path: %w", err)
		}

		pw := input.Password
		if pw == "" {
			pw = cfg.Password
		}

		if err := w.Save(path, pw); err != nil {
			return nil, nil, fmt.Errorf("failed to save wallet: %w", err)
		}

		return nil, map[string]string{
			"address": w.PublicKey(),
			"file":    path,
		}, nil
	})

	// wallet_balance
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "wallet_balance",
		Description: "Check SOL balance of the current wallet.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input walletBalanceInput) (*mcpsdk.CallToolResult, any, error) {
		w, err := resolveWallet(cfg, input.PrivateKey)
		if err != nil {
			return nil, nil, err
		}

		client, err := resolveClient(cfg, input.RPCURL)
		if err != nil {
			return nil, nil, err
		}

		pubKey, err := solanago.PublicKeyFromBase58(w.PublicKey())
		if err != nil {
			return nil, nil, fmt.Errorf("invalid wallet address: %w", err)
		}

		balance, err := client.GetBalance(pubKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get balance: %w", err)
		}

		return nil, map[string]interface{}{
			"address":          w.PublicKey(),
			"balance_sol":      float64(balance) / 1e9,
			"balance_lamports": balance,
		}, nil
	})

	// wallet_airdrop
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "wallet_airdrop",
		Description: "Request test SOL from the Solana devnet faucet.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input walletAirdropInput) (*mcpsdk.CallToolResult, any, error) {
		amount := input.Amount
		if amount <= 0 {
			amount = 1.0
		}
		if amount > 2 {
			return nil, nil, fmt.Errorf("max 2 SOL per airdrop request")
		}
		lamports := uint64(amount * 1e9)

		w, err := resolveWallet(cfg, input.PrivateKey)
		if err != nil {
			return nil, nil, err
		}

		pubKey, err := solanago.PublicKeyFromBase58(w.PublicKey())
		if err != nil {
			return nil, nil, fmt.Errorf("invalid wallet address: %w", err)
		}

		client, err := resolveClient(cfg, input.RPCURL)
		if err != nil {
			return nil, nil, err
		}

		sig, err := client.RequestAirdrop(pubKey, lamports)
		if err != nil {
			return nil, nil, fmt.Errorf("airdrop failed: %w", err)
		}

		return nil, map[string]interface{}{
			"signature":  sig,
			"address":    w.PublicKey(),
			"amount_sol": amount,
		}, nil
	})
}
