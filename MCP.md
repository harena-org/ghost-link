# GhostLink MCP Server

GhostLink includes a built-in [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that allows AI agents to connect over stdio and call GhostLink tools natively — no shell output parsing or subprocess management required.

## Overview

MCP is a standardized protocol that enables AI agents to:

- **Auto-discover tools** — agents get all available tools and their input/output schemas upon connection
- **Structured I/O** — JSON-formatted parameters and return values, no text parsing needed
- **Persistent connection** — maintain a long-lived connection over stdio, avoiding a new process per operation
- **No shell required** — agents call tool functions directly, no command-line construction needed

## Quick Start

### Start the MCP server

```bash
# Unencrypted wallet
ghostlink mcp --password ""

# Password-protected wallet
ghostlink mcp --password "your-password"

# Use private key (no wallet file needed)
ghostlink mcp --private-key <BASE58_PRIVATE_KEY>

# Specify network
ghostlink mcp --password "" -u mainnet

# Route through Tor proxy
ghostlink mcp --password "" --tor
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `GHOSTLINK_PASSWORD` | Equivalent to `--password` |
| `GHOSTLINK_PRIVATE_KEY` | Equivalent to `--private-key` |

```bash
export GHOSTLINK_PASSWORD=""
ghostlink mcp
```

## Client Configuration

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ghostlink": {
      "command": "/path/to/ghostlink",
      "args": ["mcp", "--password", ""]
    }
  }
}
```

Private key mode (no local wallet file needed):

```json
{
  "mcpServers": {
    "ghostlink": {
      "command": "/path/to/ghostlink",
      "args": ["mcp", "--private-key", "<BASE58_PRIVATE_KEY>"]
    }
  }
}
```

Using environment variables:

```json
{
  "mcpServers": {
    "ghostlink": {
      "command": "/path/to/ghostlink",
      "args": ["mcp"],
      "env": {
        "GHOSTLINK_PASSWORD": "",
        "GHOSTLINK_PRIVATE_KEY": "<BASE58_PRIVATE_KEY>"
      }
    }
  }
}
```

### Claude Code

#### Option 1: CLI command (recommended)

```bash
# Add GhostLink MCP server to your project
claude mcp add ghostlink /path/to/ghostlink -- mcp --password ""

# With private key (no wallet file needed)
claude mcp add ghostlink /path/to/ghostlink -- mcp --private-key "<BASE58_PRIVATE_KEY>"

# Verify the server is registered
claude mcp list
```

#### Option 2: Project config file

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "ghostlink": {
      "command": "/path/to/ghostlink",
      "args": ["mcp", "--password", ""]
    }
  }
}
```

#### Option 3: User-level config

Add to `~/.claude/settings.json` to make GhostLink available across all projects:

```json
{
  "mcpServers": {
    "ghostlink": {
      "command": "/path/to/ghostlink",
      "args": ["mcp", "--password", ""]
    }
  }
}
```

#### Verify connection

After configuration, start Claude Code and check that GhostLink tools are available:

```
> /mcp

# You should see "ghostlink" listed with 9 tools:
# status, wallet_create, wallet_import, wallet_balance, wallet_airdrop,
# send_message, receive_messages, inbox_create, inbox_list
```

### Other MCP Clients

Any client that supports MCP stdio transport can connect to GhostLink. The server is built on the official Go SDK (`github.com/modelcontextprotocol/go-sdk`) and is compatible with MCP protocol version `2024-11-05` and above.

## Tool List

The GhostLink MCP server provides 9 tools:

| Tool | Description | Category |
|------|-------------|----------|
| [`status`](#status) | System status: RPC connectivity, wallet, balance, config | Status |
| [`wallet_create`](#wallet_create) | Create a new wallet | Wallet |
| [`wallet_import`](#wallet_import) | Import wallet from private key or mnemonic | Wallet |
| [`wallet_balance`](#wallet_balance) | Check SOL balance | Wallet |
| [`wallet_airdrop`](#wallet_airdrop) | Request devnet test SOL | Wallet |
| [`send_message`](#send_message) | Encrypt and send a message | Messaging |
| [`receive_messages`](#receive_messages) | Fetch and decrypt messages | Messaging |
| [`inbox_create`](#inbox_create) | Create a stealth inbox | Inbox |
| [`inbox_list`](#inbox_list) | List all inboxes | Inbox |

## Tool Reference

### `status`

Query GhostLink system status including RPC connectivity, wallet info, balance, and configuration. Recommended to call before other operations to verify the environment is ready.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `rpc_url` | string | No | Override RPC URL |

**Example output:**

```json
{
  "rpc_url": "https://api.devnet.solana.com",
  "rpc_reachable": true,
  "wallet_exists": true,
  "wallet_address": "7xKX...abc",
  "balance_sol": 1.5,
  "balance_lamports": 1500000000,
  "default_inbox": "agent-inbox",
  "tor_enabled": false,
  "max_message_size": 340
}
```

---

### `wallet_create`

Create a new Solana wallet with an Ed25519 keypair and BIP39 mnemonic (24 words). The wallet file is saved at `~/.ghostlink/wallet.json`.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `password` | string | No | Wallet encryption password (empty string = no encryption) |

**Example output:**

```json
{
  "address": "7xKX...abc",
  "file": "/home/user/.ghostlink/wallet.json",
  "mnemonic": "word1 word2 word3 ... word24"
}
```

**Note:** The mnemonic is the only way to recover the wallet — store it securely. This operation will fail if the wallet file already exists.

---

### `wallet_import`

Import an existing wallet from a private key or mnemonic. Either `private_key` or `mnemonic` must be provided.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `private_key` | string | No* | Base58-encoded private key |
| `mnemonic` | string | No* | BIP39 mnemonic phrase |
| `password` | string | No | Wallet encryption password (empty string = no encryption) |

*Either `private_key` or `mnemonic` must be provided.

**Example output:**

```json
{
  "address": "7xKX...abc",
  "file": "/home/user/.ghostlink/wallet.json"
}
```

---

### `wallet_balance`

Check the SOL balance of the current wallet.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `private_key` | string | No | Override wallet private key |
| `rpc_url` | string | No | Override RPC URL |

**Example output:**

```json
{
  "address": "7xKX...abc",
  "balance_sol": 1.5,
  "balance_lamports": 1500000000
}
```

**Note:** 1 SOL = 1,000,000,000 lamports.

---

### `wallet_airdrop`

Request test SOL from the Solana devnet faucet. Only available on devnet.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `amount` | number | No | Amount in SOL (default 1, max 2) |
| `private_key` | string | No | Override wallet private key |
| `rpc_url` | string | No | Override RPC URL |

**Example output:**

```json
{
  "signature": "5UfD...xyz",
  "address": "7xKX...abc",
  "amount_sol": 1
}
```

**Note:** The devnet faucet has rate limits. Requests that are too frequent may fail and will be retried automatically.

---

### `send_message`

Encrypt a message and send it to a recipient via a Solana Memo transaction. Messages are encrypted with NaCl box (sender private key + recipient public key) — only the recipient can decrypt.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | **Yes** | Recipient Solana address (Base58 public key) |
| `message` | string | **Yes** | Plaintext message content |
| `type` | string | No | Message type (default `"text"`) |
| `reply_to` | string | No | Transaction signature being replied to |
| `wait` | boolean | No | Wait for transaction confirmation |
| `timeout` | integer | No | Confirmation timeout in seconds (default 30) |
| `private_key` | string | No | Override sender private key |
| `rpc_url` | string | No | Override RPC URL |

**Example output:**

```json
{
  "signature": "5UfD...xyz",
  "recipient": "9yMN...abc",
  "size": 52,
  "encrypted_size": 480,
  "confirmed": true
}
```

**Message size limits:**

- Max uncompressed payload: 340 bytes (after flag byte). With zlib compression, typical text messages of ~600-800 bytes will fit.
- Wire format (V1): `GL1:` prefix (4B) + base64(nonce[24B] + NaCl_box(flag[1B] + payload)) ≤ 512 bytes (Solana Memo limit)
- The `GL1:` prefix enables fast memo filtering without decryption. Compression is applied automatically when beneficial.

**Recommendation:** Always set `wait: true` in automation to ensure the message is confirmed on-chain before proceeding.

---

### `receive_messages`

Fetch and decrypt messages from the blockchain. Can use the main wallet or a named inbox.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `inbox` | string | No | Inbox name or address |
| `limit` | integer | No | Max messages to return (default 20) |
| `since` | string | No | Filter by start date (format `YYYY-MM-DD`) |
| `private_key` | string | No | Override wallet private key |
| `rpc_url` | string | No | Override RPC URL |

**Example output:**

```json
{
  "messages": [
    {
      "from": "3kPQ...def",
      "time": "2026-03-01 14:30:00",
      "signature": "2xYZ...abc",
      "type": "text",
      "body": "Hello from another agent!",
      "reply_to": "",
      "meta": null
    }
  ]
}
```

**Receive address priority:** `inbox` parameter > config `default_inbox` > main wallet address.

**Note:** MCP mode does not support watch mode (continuous polling). Agents should call `receive_messages` repeatedly to check for new messages.

---

### `inbox_create`

Create a new stealth inbox. Each inbox is an independent keypair, isolated from the main wallet.

**Input parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | No | Inbox name (default: auto-generated as `inbox-<timestamp>`) |
| `password` | string | No | Inbox key encryption password (empty = no encryption) |

**Example output:**

```json
{
  "name": "agent-inbox",
  "address": "4zAB...xyz",
  "key_path": "/home/user/.ghostlink/inbox_agent-inbox.json"
}
```

---

### `inbox_list`

List all created inboxes.

**Input parameters:** None.

**Example output:**

```json
{
  "inboxes": [
    {
      "name": "agent-inbox",
      "address": "4zAB...xyz",
      "created_at": "2026-03-01 10:00:00",
      "is_default": true
    },
    {
      "name": "backup",
      "address": "8mNP...abc",
      "created_at": "2026-03-01 12:00:00",
      "is_default": false
    }
  ]
}
```

## Authentication

The MCP server **never** prompts for passwords interactively. Wallet identity is resolved through the following priority chain:

1. **Tool parameter `private_key`** — can be specified independently per call
2. **Startup flag `--private-key`** / `GHOSTLINK_PRIVATE_KEY` — server-level default
3. **Wallet file** — decrypted with `--password` / `GHOSTLINK_PASSWORD`

```
Tool private_key param → Startup --private-key → Env variable → Wallet file + password
```

**Recommendation:** For AI agents, creating an unencrypted wallet with `--password ""` is the simplest approach.

## RPC URL Resolution

Each tool's `rpc_url` parameter can override the default RPC. Resolution priority:

```
Tool rpc_url param → Startup --url → Config file → Default devnet
```

Supported values:
- Network names: `devnet`, `testnet`, `mainnet`
- Full URLs: `https://my-rpc.example.com`

## Agent Workflow Examples

### Environment Setup

```
1. Call status → check if the environment is ready
2. If wallet_exists = false:
   a. Call wallet_create → create a wallet
   b. Call wallet_airdrop → get test SOL
3. Call inbox_create(name: "my-inbox") → create an inbox
4. Call status → confirm everything is working
```

### Sending a Message

```
1. Call send_message(to: "<address>", message: "hello", wait: true)
2. Check that confirmed = true in the response
3. Save the signature for tracking
```

### Receiving and Replying

```
1. Call receive_messages(limit: 10)
2. Iterate over the messages array
3. For each message:
   a. Read from and body
   b. Call send_message(to: from, message: "reply content", reply_to: signature)
```

### Two-Agent Communication

```
Agent A:                              Agent B:
  wallet_create                         wallet_create
  wallet_airdrop                        wallet_airdrop
  inbox_create(name: "a-inbox")         inbox_create(name: "b-inbox")
       |                                     |
       |  ← Exchange inbox addresses →        |
       |                                     |
  send_message(to: B_addr, ...)         receive_messages(inbox: "b-inbox")
                                        send_message(to: A_addr, ...)
  receive_messages(inbox: "a-inbox")
       |                                     |
       ↓                                     ↓
     Continue loop...                    Continue loop...
```

## Error Handling

When a tool call fails, the MCP protocol returns an error message. Common errors:

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `wallet is encrypted; provide private_key param, --password flag, or GHOSTLINK_PASSWORD env` | Wallet is encrypted but no password was provided | Add `--password` at startup or pass the `private_key` parameter |
| `failed to read wallet` | Wallet file does not exist | Call `wallet_create` first |
| `message too long` | Message exceeds size limit after compression | Shorten the message content |
| `invalid recipient address` | Recipient address format is invalid | Check the Base58 address |
| `airdrop failed` | Devnet faucet rate limit | Wait and retry, or visit https://faucet.solana.com |
| `insufficient balance` | Insufficient SOL balance | Call `wallet_airdrop` to fund the wallet |
| `inbox "xxx" not found` | Inbox does not exist | Call `inbox_create` first |
| `inbox "xxx" already exists` | Inbox name is taken | Use a different name |

## Protocol Testing

### Manual JSON-RPC Testing

```bash
# Send initialize + tools/list requests
(
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
  sleep 1
) | ghostlink mcp --password ""
```

### Calling a Tool

```bash
# Call the status tool
(
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"status","arguments":{}}}'
  sleep 1
) | ghostlink mcp --password ""
```

### MCP Inspector

Use [MCP Inspector](https://github.com/modelcontextprotocol/inspector) for interactive testing:

```bash
npx @modelcontextprotocol/inspector ghostlink mcp --password ""
```

## File Locations

| File | Description |
|------|-------------|
| `~/.ghostlink/wallet.json` | Main wallet keypair |
| `~/.ghostlink/config.json` | Configuration (network, RPC, Tor, default inbox) |
| `~/.ghostlink/inboxes.json` | Inbox metadata (name, address, created_at) |
| `~/.ghostlink/inbox_<name>.json` | Per-inbox independent keypair |

## Architecture

```
cmd/ghostlink/mcp_cmd.go        Cobra subcommand, reads global flags, builds ServerConfig
        ↓
internal/mcp/server.go           NewServer() creates MCP server, registers all tools
        ↓
internal/mcp/helpers.go          ServerConfig, resolveWallet(), resolveClient()
        ↓
internal/mcp/tools_*.go          9 tool handler functions
    ├── tools_status.go          status
    ├── tools_wallet.go          wallet_create, wallet_import, wallet_balance, wallet_airdrop
    ├── tools_message.go         send_message, receive_messages
    └── tools_inbox.go           inbox_create, inbox_list
        ↓
internal/                        Reuses existing modules
    ├── wallet/                  Key generation, encrypted storage
    ├── crypto/                  NaCl box encryption/decryption
    ├── solana/                  Solana RPC client, Memo transactions
    ├── inbox/                   Inbox store
    ├── message/                 Message envelope encoding/decoding
    └── config/                  Config file management
```

The MCP server runs over stdio transport using the JSON-RPC 2.0 protocol. Once started, it maintains a persistent connection until stdin is closed or the process is terminated.
