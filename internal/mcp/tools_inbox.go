package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ghost-link/ghost-link/internal/config"
	"github.com/ghost-link/ghost-link/internal/inbox"
	"github.com/ghost-link/ghost-link/internal/wallet"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type inboxCreateInput struct {
	Name     string `json:"name,omitempty" jsonschema:"Inbox name (default: auto-generated)"`
	Password string `json:"password,omitempty" jsonschema:"Encryption password for inbox key (empty = no encryption)"`
}

type inboxListInput struct{}

func registerInboxTools(server *mcpsdk.Server, cfg *ServerConfig) {
	// inbox_create
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "inbox_create",
		Description: "Create a new stealth inbox with an independent keypair.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input inboxCreateInput) (*mcpsdk.CallToolResult, any, error) {
		name := input.Name
		if name == "" {
			name = fmt.Sprintf("inbox-%d", time.Now().Unix())
		}

		store, err := inbox.LoadStore()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load inbox data: %w", err)
		}

		for _, ib := range store.Inboxes {
			if ib.Name == name {
				return nil, nil, fmt.Errorf("inbox %q already exists", name)
			}
		}

		w, err := wallet.NewWallet()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate inbox address: %w", err)
		}

		dir, err := config.ConfigDir()
		if err != nil {
			return nil, nil, err
		}
		inboxKeyPath := filepath.Join(dir, "inbox_"+name+".json")

		pw := input.Password
		if pw == "" {
			pw = cfg.Password
		}

		if err := w.Save(inboxKeyPath, pw); err != nil {
			return nil, nil, fmt.Errorf("failed to save inbox key: %w", err)
		}

		entry := inbox.Entry{
			Name:      name,
			Address:   w.PublicKey(),
			CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}

		store.Inboxes = append(store.Inboxes, entry)
		if err := inbox.SaveStore(store); err != nil {
			return nil, nil, fmt.Errorf("failed to save inbox list: %w", err)
		}

		return nil, map[string]string{
			"name":     name,
			"address":  w.PublicKey(),
			"key_path": inboxKeyPath,
		}, nil
	})

	// inbox_list
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "inbox_list",
		Description: "List all stealth inboxes.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input inboxListInput) (*mcpsdk.CallToolResult, any, error) {
		store, err := inbox.LoadStore()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load inbox data: %w", err)
		}

		fileCfg, _ := config.Load()

		type inboxListEntry struct {
			Name      string `json:"name"`
			Address   string `json:"address"`
			CreatedAt string `json:"created_at"`
			IsDefault bool   `json:"is_default"`
		}

		var entries []inboxListEntry
		for _, ib := range store.Inboxes {
			entries = append(entries, inboxListEntry{
				Name:      ib.Name,
				Address:   ib.Address,
				CreatedAt: ib.CreatedAt,
				IsDefault: fileCfg.DefaultInbox == ib.Name,
			})
		}

		return nil, map[string]interface{}{
			"inboxes": entries,
		}, nil
	})
}
