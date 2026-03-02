# GhostLink Quick Start Guide

GhostLink is an end-to-end encrypted messaging tool that stores messages on the Solana blockchain. Only the intended recipient can decrypt and read your messages.

## Installation

```bash
git clone https://github.com/ghost-link/ghost-link.git
cd ghost-link
make build
```

The binary is at `build/ghostlink`. Add it to your PATH or use `./build/ghostlink` directly.

## Step 1: Create Your Wallet

```bash
ghostlink wallet create
```

You'll be prompted to set a password (press Enter to skip encryption). The output shows:

- **Address** — your public identity on Solana (share this freely)
- **Mnemonic** — 24 secret words to recover your wallet (write these down and keep them safe!)

> **WARNING:** Your mnemonic is the ONLY way to recover your wallet. If you lose it, your wallet is gone forever. Never share it with anyone.

## Step 2: Get Test SOL (Devnet)

Messages cost a tiny amount of SOL for transaction fees. On devnet (the default network), you can get free test SOL:

```bash
ghostlink wallet airdrop
```

This gives you 1 SOL. Check your balance with:

```bash
ghostlink wallet balance
```

## Step 3: Send a Message

You need the recipient's Solana address (their wallet or inbox address):

```bash
ghostlink send --to <RECIPIENT_ADDRESS> -m "Hey, this is a secret message!"
```

The message is encrypted with the recipient's public key before going on-chain. Nobody except the recipient can read it.

### Message Size Limit

Max message body is about 304 bytes (roughly 300 characters). The total encrypted payload must fit in a 512-byte Solana Memo.

### Wait for Confirmation

Add `--wait` to block until the message is confirmed on-chain:

```bash
ghostlink send --to <ADDRESS> -m "Important message" --wait
```

### Send from a Pipe

You can pipe message content from another command:

```bash
echo "Hello from a script" | ghostlink send --to <ADDRESS> --stdin
```

## Step 4: Receive Messages

```bash
ghostlink receive
```

This scans your recent on-chain transactions, decrypts messages addressed to you, and displays them:

```
─────────────────────────────────────────
From:      3aBC...def
Time:      2026-03-01 12:00:00
Signature: 4vJ2...
Type:      text
Message:   Hey, this is a secret message!
─────────────────────────────────────────
1 message(s) decrypted.
```

### Filter Messages

```bash
# Only messages from the last week
ghostlink receive --since 2026-02-22

# Fetch more messages (default is 20)
ghostlink receive --limit 50
```

### Watch Mode

Continuously poll for new messages (useful for live conversations):

```bash
ghostlink receive --watch 10
```

This checks every 10 seconds and prints new messages as they arrive. Press Ctrl+C to stop.

## Step 5: Set Up an Inbox

Inboxes are separate keypairs, independent from your main wallet. Use them to receive messages without revealing your main wallet address.

```bash
# Create an inbox
ghostlink inbox create my-inbox

# Set it as default (so `receive` uses it automatically)
ghostlink inbox set-default my-inbox

# Share the address with others
ghostlink inbox share my-inbox
```

Now when someone sends a message to your inbox address, `ghostlink receive` will pick it up automatically.

### Export QR Code

Generate a QR code image to share your inbox address:

```bash
ghostlink inbox share my-inbox -o my-inbox-qr.png
```

### List Your Inboxes

```bash
ghostlink inbox list
```

## Complete Example: Alice and Bob

### Bob sets up

```bash
# Create wallet
ghostlink wallet create
# Get test SOL
ghostlink wallet airdrop
# Create a private inbox
ghostlink inbox create secret-channel
# Set as default
ghostlink inbox set-default secret-channel
# Share address with Alice (e.g., copy the address from output)
ghostlink inbox share secret-channel
# Output: Address: 9yMN...xyz
```

### Alice sends a message

```bash
# Create wallet and fund it
ghostlink wallet create
ghostlink wallet airdrop
# Send to Bob's inbox address
ghostlink send --to 9yMN...xyz -m "Hi Bob, let's talk privately!" --wait
```

### Bob reads it

```bash
ghostlink receive
# ─────────────────────────────────────────
# From:      3aBC...def
# Time:      2026-03-01 12:00:00
# Signature: 5UfD...
# Type:      text
# Message:   Hi Bob, let's talk privately!
```

### Bob replies

```bash
# Reply using Alice's wallet address (shown in "From" field)
ghostlink send --to 3aBC...def -m "Hi Alice! This channel is secure." --wait
```

## JSON Output

Add `--json` to any command for machine-readable output:

```bash
ghostlink wallet balance --json
```

```json
{
  "address": "7xKX...",
  "balance_sol": 1.5,
  "balance_lamports": 1500000000
}
```

```bash
ghostlink receive --json
```

```json
{
  "messages": [
    {
      "from": "3aBC...",
      "time": "2026-03-01 12:00:00",
      "signature": "5UfD...",
      "type": "text",
      "body": "Hi Bob, let's talk privately!"
    }
  ]
}
```

## System Status

Check your setup at any time:

```bash
ghostlink status
```

```
GhostLink Status
─────────────────────────────────────────
RPC URL:          https://api.devnet.solana.com
RPC Status:       reachable
─────────────────────────────────────────
Wallet:           found
Address:          7xKX...abc
Balance:          1.500000000 SOL
─────────────────────────────────────────
Default Inbox:    my-inbox
Tor:              disabled
Max Message Size: 344 bytes
```

## Network Selection

GhostLink defaults to devnet (free test network). Switch networks with `-u`:

```bash
# Use testnet
ghostlink send -u testnet --to <ADDRESS> -m "hello"

# Use mainnet (real SOL required!)
ghostlink send -u mainnet --to <ADDRESS> -m "hello"

# Use a custom RPC endpoint
ghostlink send -u https://my-rpc.example.com --to <ADDRESS> -m "hello"
```

Or set it permanently in `~/.ghostlink/config.json`:

```json
{
  "network": "devnet"
}
```

## Privacy with Tor

Route all traffic through Tor to hide your IP address:

```bash
# Install and start Tor first
# Ubuntu/Debian: sudo apt install tor && sudo systemctl start tor
# macOS: brew install tor && brew services start tor

# Send via Tor
ghostlink send --tor --to <ADDRESS> -m "Anonymous message"

# Receive via Tor
ghostlink receive --tor
```

Enable Tor permanently in `~/.ghostlink/config.json`:

```json
{
  "tor_enabled": true,
  "tor_proxy": "127.0.0.1:9050"
}
```

## Importing an Existing Wallet

```bash
# From a private key
ghostlink wallet import --key <BASE58_PRIVATE_KEY>

# From a mnemonic phrase
ghostlink wallet import --mnemonic "word1 word2 word3 ... word24"
```

## Non-Interactive Usage

For scripts and automation, avoid password prompts with:

```bash
# Provide password via flag
ghostlink wallet balance --password "mypassword"

# Or via environment variable
export GHOSTLINK_PASSWORD="mypassword"
ghostlink wallet balance

# Or use a private key directly (no wallet file needed)
ghostlink wallet balance --private-key <BASE58_KEY>
```

## File Locations

| File | Purpose |
|------|---------|
| `~/.ghostlink/wallet.json` | Your main wallet keypair |
| `~/.ghostlink/config.json` | Settings (network, Tor, default inbox) |
| `~/.ghostlink/inboxes.json` | List of all your inboxes |
| `~/.ghostlink/inbox_<name>.json` | Individual inbox keypairs |

## Command Cheat Sheet

| Task | Command |
|------|---------|
| Create wallet | `ghostlink wallet create` |
| Check balance | `ghostlink wallet balance` |
| Get test SOL | `ghostlink wallet airdrop` |
| Send message | `ghostlink send --to <ADDR> -m "text"` |
| Send and confirm | `ghostlink send --to <ADDR> -m "text" --wait` |
| Receive messages | `ghostlink receive` |
| Watch for new messages | `ghostlink receive --watch 10` |
| Create inbox | `ghostlink inbox create <name>` |
| Set default inbox | `ghostlink inbox set-default <name>` |
| Share inbox QR | `ghostlink inbox share <name>` |
| System health check | `ghostlink status` |
| JSON output | Add `--json` to any command |

## Troubleshooting

**"insufficient balance"** — You need SOL for transaction fees. Run `ghostlink wallet airdrop` on devnet.

**"message too long"** — Keep messages under ~300 characters. The encrypted payload must fit in 512 bytes.

**"failed to load wallet"** — Make sure you've created a wallet first with `ghostlink wallet create`.

**"wallet is encrypted, password required"** — Provide your password when prompted, or use `--password`.

**"RPC Status: unreachable"** — Check your internet connection, or try a different RPC with `-u`.
