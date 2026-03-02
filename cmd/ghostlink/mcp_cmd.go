package main

import (
	"context"
	"os"

	"github.com/ghost-link/ghost-link/internal/config"
	ghostmcp "github.com/ghost-link/ghost-link/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI agent integration",
	Long:  "Start a Model Context Protocol (MCP) server over stdio. AI agents connect and call GhostLink tools natively.",
	RunE:  runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	rpcURL := cfg.GetRPCURL(urlFlag)

	privKey := privateKeyFlag
	if privKey == "" {
		privKey = os.Getenv("GHOSTLINK_PRIVATE_KEY")
	}

	pw := passwordFlag
	if pw == "" {
		pw = os.Getenv("GHOSTLINK_PASSWORD")
	}

	proxyAddr := ""
	if torFlag || cfg.TorEnabled {
		proxyAddr = torProxy
		if proxyAddr == "" {
			proxyAddr = cfg.TorProxy
		}
	}

	serverCfg := &ghostmcp.ServerConfig{
		PrivateKey: privKey,
		Password:   pw,
		RPCURL:     rpcURL,
		TorEnabled: torFlag || cfg.TorEnabled,
		TorProxy:   proxyAddr,
	}

	server := ghostmcp.NewServer(serverCfg)
	return server.Run(context.Background(), &mcpsdk.StdioTransport{})
}
