# OPENCLAW.md — GhostLink Integration Guide for AI Agents

This document tells AI agents (OpenClaw and others) how to operate GhostLink programmatically. Every command supports `--json` for machine-readable output and `--password` / `--private-key` to avoid interactive prompts.

## Quick Start

```bash
# 1. Create a wallet (no interactive prompt)
ghostlink wallet create --json --password ""

# 2. Fund it on devnet
ghostlink wallet airdrop --json --password ""

# 3. Send a message
ghostlink send --to <RECIPIENT_ADDRESS> -m "hello" --json --password "" --wait

# 4. Receive messages
ghostlink receive --json --password ""
```

## Global Flags (apply to all commands)

| Flag | Purpose |
|------|---------|
| `--json` | Output structured JSON to stdout (errors too) |
| `--password <pw>` | Provide wallet password non-interactively (use `""` for no encryption) |
| `--private-key <base58>` | Use an in-memory keypair, bypasses wallet file entirely |
| `-u, --url <rpc>` | Solana RPC endpoint: `devnet`, `testnet`, `mainnet`, or a full URL |
| `--tor` | Route traffic through Tor SOCKS5 proxy |
| `--config <path>` | Custom config file path |

### Environment Variables

| Variable | Equivalent Flag |
|----------|----------------|
| `GHOSTLINK_PASSWORD` | `--password` |
| `GHOSTLINK_PRIVATE_KEY` | `--private-key` |

Priority: flag > environment variable > interactive prompt (or wallet file).

## Authentication Priority Chain

When a command needs a wallet, GhostLink resolves identity in this order:

1. `--private-key` flag or `GHOSTLINK_PRIVATE_KEY` env — uses key directly, no file needed
2. Wallet file at `--path` or `~/.ghostlink/wallet.json` — decrypted with password from:
   - `--password` flag
   - `GHOSTLINK_PASSWORD` env
   - Interactive prompt (last resort, avoid in automation)

**Recommendation for agents:** Use `--private-key` for stateless operation, or `--password ""` when creating unencrypted wallet files.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Network / RPC error |
| 3 | Authentication error (bad password, invalid key) |
| 4 | Insufficient funds |
| 5 | Message too large |

In `--json` mode, errors produce `{"error": "message"}` on stdout with the appropriate exit code.

## Message Envelope Format

All messages sent by GhostLink are wrapped in a JSON envelope before encryption:

```json
{
  "v": 1,
  "type": "text",
  "body": "your message here",
  "reply_to": "<tx_signature>",
  "meta": {"key": "value"}
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `v` | yes | Envelope version, always `1` |
| `type` | yes | Message type (default `"text"`, set via `--type`) |
| `body` | yes | The message content |
| `reply_to` | no | Transaction signature this message replies to |
| `meta` | no | Arbitrary key-value metadata |

On receive, GhostLink auto-detects envelopes. Legacy raw-text messages appear with `type: ""`.

### Size Constraint

Max plaintext body depends on envelope overhead (~40 bytes JSON wrapper):
- **Max envelope JSON**: 344 bytes (after that, encryption exceeds 512-byte Solana Memo limit)
- **Effective max body**: ~304 bytes for typical messages

## Commands Reference

### `ghostlink status`

Health check. Always run this first to verify connectivity.

```bash
ghostlink status --json
```

```json
{
  "rpc_url": "https://api.devnet.solana.com",
  "rpc_reachable": true,
  "wallet_exists": true,
  "wallet_address": "7xKX...",
  "balance_sol": 1.5,
  "balance_lamports": 1500000000,
  "default_inbox": "main",
  "tor_enabled": false,
  "tor_proxy": "127.0.0.1:9050",
  "max_message_size": 344
}
```

### `ghostlink wallet create`

```bash
ghostlink wallet create --json --password ""
```

```json
{
  "address": "7xKX...",
  "file": "/home/user/.ghostlink/wallet.json",
  "mnemonic": "word1 word2 ... word24"
}
```

When `--password` is provided via flag or env, the confirmation prompt is skipped.

### `ghostlink wallet import`

```bash
# Import from private key
ghostlink wallet import --key <BASE58_PRIVATE_KEY> --json --password ""

# Import from mnemonic
ghostlink wallet import --mnemonic "word1 word2 ..." --json --password ""
```

```json
{
  "address": "7xKX...",
  "file": "/home/user/.ghostlink/wallet.json"
}
```

### `ghostlink wallet balance`

```bash
ghostlink wallet balance --json --private-key <BASE58_KEY>
```

```json
{
  "address": "7xKX...",
  "balance_sol": 1.5,
  "balance_lamports": 1500000000
}
```

### `ghostlink wallet airdrop`

Devnet only. Max 2 SOL per request.

```bash
ghostlink wallet airdrop 1 --json --password ""
```

```json
{
  "signature": "5UfD...",
  "address": "7xKX...",
  "amount_sol": 1
}
```

### `ghostlink send`

```bash
# Send with inline message
ghostlink send --to <ADDR> -m "hello" --json --password "" --wait

# Send from stdin (useful for piping)
echo "hello from agent" | ghostlink send --to <ADDR> --stdin --json --password "" --wait

# Send with reply reference
ghostlink send --to <ADDR> -m "got it" --reply-to <TX_SIG> --json --password ""

# Send with custom type
ghostlink send --to <ADDR> -m '{"action":"ping"}' --type "command" --json --password ""
```

```json
{
  "signature": "5UfD...",
  "recipient": "9yMN...",
  "size": 52,
  "encrypted_size": 480,
  "confirmed": true
}
```

| Flag | Default | Description |
|------|---------|-------------|
| `--to` | (required) | Recipient Solana address |
| `-m` | | Message text (mutually exclusive with `--stdin`) |
| `--stdin` | false | Read message from stdin instead of `-m` |
| `--wait` | false | Wait for on-chain confirmation before returning |
| `--timeout` | 30 | Confirmation timeout in seconds (with `--wait`) |
| `--type` | `"text"` | Envelope message type field |
| `--reply-to` | | Transaction signature being replied to |

**Recommendation:** Always use `--wait` in automation to ensure the message landed on-chain before proceeding.

### `ghostlink receive`

#### Single-shot (fetch once)

```bash
ghostlink receive --json --password ""
```

```json
{
  "messages": [
    {
      "from": "3kPQ...",
      "time": "2026-02-28 14:30:00",
      "signature": "2xYZ...",
      "type": "text",
      "body": "hello",
      "reply_to": "",
      "meta": null
    }
  ]
}
```

#### Watch mode (continuous polling)

```bash
ghostlink receive --json --password "" --watch 5
```

Outputs one JSON object per line (JSONL) as new messages arrive. Poll interval is in seconds. Send `SIGINT` or `SIGTERM` to stop.

| Flag | Default | Description |
|------|---------|-------------|
| `--inbox` | (default inbox) | Inbox name or address to check |
| `--limit` | 20 | Max messages to fetch per poll |
| `--since` | | Only messages after this date (`YYYY-MM-DD`) |
| `--watch` | 0 | Polling interval in seconds (0 = single-shot) |

### `ghostlink inbox create`

```bash
ghostlink inbox create myinbox --json --password ""
```

```json
{
  "name": "myinbox",
  "address": "4zAB...",
  "key_path": "/home/user/.ghostlink/inbox_myinbox.json"
}
```

### `ghostlink inbox list`

```bash
ghostlink inbox list --json
```

```json
{
  "inboxes": [
    {
      "name": "myinbox",
      "address": "4zAB...",
      "created_at": "2026-03-01 10:00:00",
      "is_default": true
    }
  ]
}
```

### `ghostlink inbox set-default`

```bash
ghostlink inbox set-default myinbox --json
```

```json
{
  "name": "myinbox",
  "success": true
}
```

### `ghostlink inbox share`

```bash
# Get address only (JSON mode omits QR terminal art)
ghostlink inbox share myinbox --json
```

```json
{
  "name": "myinbox",
  "address": "4zAB..."
}
```

## Typical Agent Workflows

### One-time Setup

```bash
# Create wallet, capture address
RESULT=$(ghostlink wallet create --json --password "")
ADDRESS=$(echo "$RESULT" | jq -r '.address')

# Fund on devnet
ghostlink wallet airdrop --json --password ""

# Create an inbox for receiving
INBOX=$(ghostlink inbox create agent-inbox --json --password "")
INBOX_ADDR=$(echo "$INBOX" | jq -r '.address')

# Set as default
ghostlink inbox set-default agent-inbox --json
```

### Send-and-Confirm Loop

```bash
# Send and wait for confirmation
SEND=$(ghostlink send \
  --to "$PEER_ADDRESS" \
  -m "task complete" \
  --type "status" \
  --json --password "" --wait)

# Check result
SIG=$(echo "$SEND" | jq -r '.signature')
CONFIRMED=$(echo "$SEND" | jq -r '.confirmed')
```

### Poll for Incoming Messages

```bash
# Single fetch
MSGS=$(ghostlink receive --json --password "" --limit 5)
COUNT=$(echo "$MSGS" | jq '.messages | length')

# Or use watch mode in background
ghostlink receive --json --password "" --watch 10 > /tmp/messages.jsonl &
WATCH_PID=$!
# ... do other work ...
kill $WATCH_PID
```

### Conversational Reply

```bash
# Receive a message
MSG=$(ghostlink receive --json --password "" --limit 1)
FROM=$(echo "$MSG" | jq -r '.messages[0].from')
SIG=$(echo "$MSG" | jq -r '.messages[0].signature')
BODY=$(echo "$MSG" | jq -r '.messages[0].body')

# Reply to it
ghostlink send --to "$FROM" -m "ack: $BODY" --reply-to "$SIG" --json --password "" --wait
```

## Error Handling

All errors in `--json` mode produce:

```json
{"error": "descriptive message"}
```

Check the exit code to categorize:

```bash
ghostlink send --to "$ADDR" -m "test" --json --password "" --wait
EXIT=$?
if [ $EXIT -eq 0 ]; then
  echo "Success"
elif [ $EXIT -eq 2 ]; then
  echo "Network error, retry"
elif [ $EXIT -eq 4 ]; then
  echo "Need more SOL"
elif [ $EXIT -eq 5 ]; then
  echo "Message too long, truncate"
else
  echo "Other error"
fi
```

## File Locations

| File | Purpose |
|------|---------|
| `~/.ghostlink/wallet.json` | Main wallet keypair |
| `~/.ghostlink/config.json` | Network, RPC, Tor, default inbox settings |
| `~/.ghostlink/inboxes.json` | Inbox metadata (name, address, created_at) |
| `~/.ghostlink/inbox_<name>.json` | Per-inbox keypair |
