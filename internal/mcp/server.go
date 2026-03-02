package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates an MCP server with all GhostLink tools registered.
func NewServer(cfg *ServerConfig) *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "ghostlink",
		Version: "0.6.0",
	}, nil)

	registerStatusTool(server, cfg)
	registerWalletTools(server, cfg)
	registerMessageTools(server, cfg)
	registerInboxTools(server, cfg)

	return server
}
