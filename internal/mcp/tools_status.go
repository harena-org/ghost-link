package mcp

import (
	"context"
	"os"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/ghost-link/ghost-link/internal/config"
	ghostcrypto "github.com/ghost-link/ghost-link/internal/crypto"
	"github.com/ghost-link/ghost-link/internal/wallet"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type statusInput struct {
	RPCURL string `json:"rpc_url,omitempty" jsonschema:"override RPC URL"`
}

func registerStatusTool(server *mcpsdk.Server, cfg *ServerConfig) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "status",
		Description: "Show GhostLink system status: RPC health, wallet, balance, config.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input statusInput) (*mcpsdk.CallToolResult, any, error) {
		fileCfg, _ := config.Load()

		rpcURL := input.RPCURL
		if rpcURL == "" {
			rpcURL = cfg.RPCURL
		}
		if rpcURL == "" {
			rpcURL = fileCfg.GetRPCURL("")
		}

		result := map[string]interface{}{
			"rpc_url":          rpcURL,
			"rpc_reachable":    false,
			"wallet_exists":    false,
			"wallet_address":   "",
			"balance_sol":      0.0,
			"balance_lamports": uint64(0),
			"default_inbox":    fileCfg.DefaultInbox,
			"tor_enabled":      cfg.TorEnabled,
			"max_message_size": ghostcrypto.MaxMessageSizeV1(),
		}

		client, clientErr := resolveClient(cfg, input.RPCURL)

		walletPath, _ := wallet.DefaultWalletPath()
		if _, err := os.Stat(walletPath); err == nil {
			result["wallet_exists"] = true

			w, err := resolveWallet(cfg, "")
			if w != nil && err == nil {
				result["wallet_address"] = w.PublicKey()

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

		if !result["rpc_reachable"].(bool) && clientErr == nil {
			zeroAddr := solanago.PublicKey{}
			_, err := client.GetBalance(zeroAddr)
			if err == nil {
				result["rpc_reachable"] = true
			}
		}

		return nil, result, nil
	})
}
