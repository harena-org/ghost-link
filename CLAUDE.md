# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Build binary to build/ghostlink
make test           # Run all tests with -race flag: go test -v -race ./...
make lint           # Run go vet ./...
make clean          # Remove build/ directory
make build-all      # Cross-compile for Linux/macOS/Windows (amd64/arm64)

# Run a single test
go test -v -run TestEncryptDecryptRoundtrip ./internal/crypto

# Run a single package's tests
go test -v ./internal/wallet
```

## Architecture

GhostLink is an E2E-encrypted messaging CLI that stores encrypted messages on the Solana blockchain via the Memo Program. It also includes a built-in MCP (Model Context Protocol) server for native AI agent integration.

### Module Dependency Flow

```
cmd/ghostlink/              CLI commands (Cobra). Each file registers commands via init().
    ├── wallet_cmd.go       Uses: internal/wallet, internal/solana, internal/config
    ├── send_cmd.go         Uses: internal/message, internal/crypto, internal/solana
    ├── receive_cmd.go      Uses: internal/solana, internal/crypto, internal/message, internal/inbox
    ├── inbox_cmd.go        Uses: internal/wallet, internal/inbox, internal/config
    ├── mcp_cmd.go          Uses: internal/mcp (builds ServerConfig from global flags)
    └── status_cmd.go       Uses: internal/wallet, internal/solana, internal/config, internal/crypto

internal/mcp/               MCP server (JSON-RPC over stdio, 9 tools)
    ├── server.go           NewServer() → registers all tools
    ├── helpers.go          ServerConfig, resolveWallet(), resolveClient()
    ├── tools_status.go     status tool
    ├── tools_wallet.go     wallet_create, wallet_import, wallet_balance, wallet_airdrop
    ├── tools_message.go    send_message, receive_messages
    └── tools_inbox.go      inbox_create, inbox_list

internal/crypto/            NaCl box encryption with Ed25519→X25519 key conversion
internal/solana/            Solana RPC client, Memo/System Program instruction builders
internal/wallet/            Ed25519 keypair generation, BIP39 mnemonic, scrypt+AES-256-GCM storage
internal/message/           Message envelope (V1 JSON format with backward-compat for raw text)
internal/inbox/             Inbox store types and I/O (shared by CLI and MCP)
internal/config/            JSON config at ~/.ghostlink/config.json, RPC URL resolution
internal/output/            Dual-mode output (JSON stdout vs human stderr), JSONMode global
internal/exitcode/          Structured exit codes (0-5) with ExitError wrapper
internal/tor/               SOCKS5 proxy connection for Tor routing
```

### Command Registration Pattern

All CLI commands use Go's `init()` for self-registration. Each `*_cmd.go` file declares package-level `cobra.Command` variables and calls `rootCmd.AddCommand()` in `init()`. Global flags (`torFlag`, `urlFlag`, `jsonFlag`, `passwordFlag`, `privateKeyFlag`) are declared in `main.go` and shared across commands.

### Wallet/Identity Resolution Priority

CLI and MCP use the same priority chain but different mechanisms:

```
CLI (wallet_cmd.go):   --private-key flag → GHOSTLINK_PRIVATE_KEY env → wallet file (password prompted if needed)
MCP (helpers.go):      per-call private_key param → cfg.PrivateKey → env → wallet file (NEVER prompts)
```

The MCP server must never prompt interactively — if a wallet is encrypted and no password is available, it returns an error.

### Encryption Flow

**Send**: plaintext → JSON envelope (`message.Encode`) → NaCl box encrypt (sender X25519 priv + recipient X25519 pub) → prepend 24-byte nonce → base64 encode → Solana Memo transaction (≤512 bytes)

**Receive**: fetch Memo transactions → base64 decode → extract nonce → NaCl box decrypt → `message.Decode` (returns nil for legacy raw text, falls back to `string(plaintext)`)

Key conversion: Ed25519 → X25519 via `filippo.io/edwards25519` birational map (public) and SHA-512 + clamping (private).

### Message Envelope Format

```go
type Envelope struct {
    V       int               `json:"v"`       // version, must be 1
    Type    string            `json:"type"`    // e.g. "text"
    Body    string            `json:"body"`    // plaintext content
    ReplyTo string            `json:"reply_to,omitempty"` // tx signature
    Meta    map[string]string `json:"meta,omitempty"`
}
```

`message.Decode()` returns `nil, nil` (not an error) when data is not JSON or has `V == 0`. Both `receive_cmd.go` and `mcp/tools_message.go` use this nil-return pattern to handle legacy pre-envelope messages.

### Message Size Constraint

Max plaintext = 344 bytes. After encryption: +24 (nonce) +16 (Poly1305 tag) = 384 bytes binary → 512 bytes base64 (Solana Memo limit).

### Solana Transaction Structure

Each message creates a transaction with two instructions:
1. **System transfer of 0 lamports** — makes the tx discoverable in the recipient's history
2. **Memo instruction** — carries encrypted payload, sender is the only signer

### Output System

`internal/output` provides dual-mode output controlled by `output.JSONMode` (set from `--json` flag in `PersistentPreRun`):
- **JSON mode**: `PrintResult` writes indented JSON to stdout; `PrintError` writes `{"error":"..."}` to stdout
- **Human mode**: `PrintResult` calls a callback; `PrintError` writes to stderr
- `Status`/`Statusf` always write to stderr (safe for MCP/piped contexts)

### MCP Server

The MCP server (`internal/mcp/`) runs JSON-RPC 2.0 over stdio via `mcpsdk.StdioTransport{}`. Tools are registered using `mcpsdk.AddTool` with typed input structs — the SDK auto-generates JSON schemas from struct tags.

**Important**: `jsonschema` struct tags must be plain description strings (e.g., `jsonschema:"Override RPC URL"`), NOT `key=value` format. Required fields are controlled by presence/absence of `omitempty` in the `json` tag.

**Stdout safety**: No code in `internal/` should write to stdout — the MCP server's stdio transport uses stdout for JSON-RPC. Use `fmt.Fprintf(os.Stderr, ...)` for any diagnostic output.

### Storage Paths

All data under `~/.ghostlink/`:
- `wallet.json` — main keypair (encrypted with scrypt+AES-256-GCM, or plaintext)
- `config.json` — network, RPC URL, Tor settings, default inbox
- `inboxes.json` — inbox metadata (name, address, created_at)
- `inbox_<name>.json` — per-inbox keypair (independent from main wallet)

File permissions: directories 0700, files 0600.

### RPC URL Resolution Priority

`--url` flag → `config.network` → `config.rpc_url` → default devnet. Accepts network names (`devnet`, `testnet`, `mainnet`) or full URLs. Same chain applies in MCP with per-call `rpc_url` param taking highest priority.

### Exit Codes

`0` Success, `1` General, `2` Network, `3` Auth, `4` InsufficientFunds, `5` MessageTooLarge. Wrapped via `exitcode.Wrap(code, err)` and extracted in `main()` via `errors.As`.
