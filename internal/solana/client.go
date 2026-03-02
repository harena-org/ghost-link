package solana

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"os"
	"net/http"
	"strings"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"golang.org/x/net/proxy"
)

// rpcErrorMessage extracts a human-readable message from an RPC error.
func rpcErrorMessage(err error) string {
	var rpcErr *jsonrpc.RPCError
	if errors.As(err, &rpcErr) {
		msg := rpcErr.Message
		switch {
		case strings.Contains(msg, "no record of a prior credit"):
			return "insufficient balance, please fund your wallet with SOL"
		case strings.Contains(msg, "Blockhash not found"):
			return "blockhash expired, please retry"
		case strings.Contains(msg, "insufficient funds"):
			return "insufficient SOL to cover transaction fees"
		case strings.Contains(msg, "Transaction too large"):
			return "message too long, please shorten the content"
		case strings.Contains(msg, "Too many requests"):
			return "too many requests, please try again later"
		case strings.Contains(msg, "Internal error"):
			return "RPC internal error (possibly rate-limited), please try again later"
		case strings.Contains(msg, "airdrop limit") || strings.Contains(msg, "faucet"):
			return "airdrop limit reached, visit https://faucet.solana.com for alternatives"
		default:
			return msg
		}
	}
	return err.Error()
}

// DefaultDevnetRPC is the default Solana devnet RPC endpoint.
const DefaultDevnetRPC = rpc.DevNet_RPC

// MemoMessage represents a decoded memo from a Solana transaction.
type MemoMessage struct {
	Sender    solanago.PublicKey
	Data      []byte
	Timestamp time.Time
	Signature string
}

// Client wraps the Solana RPC client and provides high-level operations
// for sending and receiving memo-based messages.
type Client struct {
	rpcURL    string
	wsURL     string
	rpcClient *rpc.Client
	proxyAddr string // optional SOCKS5 proxy
}

// NewClient creates a new Solana RPC client. If proxyAddr is non-empty,
// all RPC requests will be routed through the specified SOCKS5 proxy
// (e.g., a Tor SOCKS5 proxy at 127.0.0.1:9050).
func NewClient(rpcURL string, proxyAddr string) (*Client, error) {
	if rpcURL == "" {
		rpcURL = DefaultDevnetRPC
	}

	// Derive WebSocket URL from RPC URL.
	wsURL := rpcToWsURL(rpcURL)

	var httpClient *http.Client

	if proxyAddr != "" {
		// Set up SOCKS5 proxy dialer.
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer for %s: %w", proxyAddr, err)
		}

		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 dialer does not support DialContext")
		}

		transport := &http.Transport{
			DialContext:         contextDialer.DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     90 * time.Second,
			MaxIdleConnsPerHost: 9,
		}

		httpClient = &http.Client{
			Timeout:   5 * time.Minute,
			Transport: transport,
		}
	} else {
		httpClient = &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				IdleConnTimeout:     5 * time.Minute,
				MaxConnsPerHost:     9,
				MaxIdleConnsPerHost: 9,
				Proxy:               http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Minute,
					KeepAlive: 180 * time.Second,
					DualStack: true,
				}).DialContext,
				ForceAttemptHTTP2:   true,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		}
	}

	opts := &jsonrpc.RPCClientOpts{
		HTTPClient: httpClient,
	}
	jsonRPCClient := jsonrpc.NewClientWithOpts(rpcURL, opts)
	rpcClient := rpc.NewWithCustomRPCClient(jsonRPCClient)

	return &Client{
		rpcURL:    rpcURL,
		wsURL:     wsURL,
		rpcClient: rpcClient,
		proxyAddr: proxyAddr,
	}, nil
}

// SendMemo creates a transaction containing a Memo instruction with the
// provided data, signs it with the given private key, and submits it to
// the Solana network. A 0-lamport transfer to the recipient is included
// so the transaction appears in the recipient's transaction history.
//
// Returns the transaction signature as a base58-encoded string.
func (c *Client) SendMemo(privateKey ed25519.PrivateKey, recipient solanago.PublicKey, data []byte) (string, error) {
	ctx := context.Background()

	// Convert ed25519.PrivateKey to solana-go PrivateKey.
	solPrivKey := solanago.PrivateKey(privateKey)
	senderPubKey := solPrivKey.PublicKey()

	// Build instructions:
	// 1. Memo instruction (sender-only signer, carries the encrypted data)
	// 2. 0-lamport transfer to recipient (makes tx discoverable in recipient's history)
	memoInstruction := BuildMemoInstruction(data, senderPubKey)
	transferInstruction := BuildTransferInstruction(senderPubKey, recipient)

	// Get the latest blockhash.
	blockhashResult, err := c.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get blockhash: %s", rpcErrorMessage(err))
	}

	// Build the transaction with both instructions.
	tx, err := solanago.NewTransaction(
		[]solanago.Instruction{transferInstruction, memoInstruction},
		blockhashResult.Value.Blockhash,
		solanago.TransactionPayer(senderPubKey),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	// Sign the transaction.
	_, err = tx.Sign(func(key solanago.PublicKey) *solanago.PrivateKey {
		if key.Equals(senderPubKey) {
			return &solPrivKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction.
	sig, err := c.rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %s", rpcErrorMessage(err))
	}

	return sig.String(), nil
}

// GetMemos fetches recent transactions for the given address, filters for
// Memo program instructions, and returns a list of MemoMessage values
// containing the sender, raw data, timestamp, and transaction signature.
func (c *Client) GetMemos(address solanago.PublicKey, limit int) ([]MemoMessage, error) {
	ctx := context.Background()

	// Fetch transaction signatures for the address.
	opts := &rpc.GetSignaturesForAddressOpts{
		Limit: &limit,
	}
	signatures, err := c.rpcClient.GetSignaturesForAddressWithOpts(ctx, address, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %s", rpcErrorMessage(err))
	}

	var memos []MemoMessage

	maxVersion := uint64(0)
	for _, sigInfo := range signatures {
		// Skip failed transactions.
		if sigInfo.Err != nil {
			continue
		}

		// Fetch the full transaction.
		txResult, err := c.rpcClient.GetTransaction(ctx, sigInfo.Signature, &rpc.GetTransactionOpts{
			Encoding:                       solanago.EncodingBase64,
			MaxSupportedTransactionVersion: &maxVersion,
		})
		if err != nil {
			// Skip transactions we cannot fetch (e.g., pruned).
			continue
		}

		if txResult == nil || txResult.Transaction == nil {
			continue
		}

		// Decode the transaction.
		tx, err := txResult.Transaction.GetTransaction()
		if err != nil {
			continue
		}

		// Extract memo instructions from the transaction.
		parsedMemos := ParseMemoTransaction(tx, txResult.BlockTime, sigInfo.Signature.String())
		memos = append(memos, parsedMemos...)
	}

	return memos, nil
}

// GetBalance returns the balance in lamports for the given address.
func (c *Client) GetBalance(address solanago.PublicKey) (uint64, error) {
	ctx := context.Background()

	result, err := c.rpcClient.GetBalance(ctx, address, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %s", rpcErrorMessage(err))
	}

	return result.Value, nil
}

// ParseMemoTransaction extracts MemoMessage entries from a parsed Solana
// transaction by scanning for instructions that target the Memo Program.
func ParseMemoTransaction(tx *solanago.Transaction, blockTime *solanago.UnixTimeSeconds, signature string) []MemoMessage {
	if tx == nil {
		return nil
	}

	var memos []MemoMessage

	for _, inst := range tx.Message.Instructions {
		// Resolve the program ID for this instruction.
		progKey, err := tx.ResolveProgramIDIndex(inst.ProgramIDIndex)
		if err != nil {
			continue
		}

		// Check if this instruction targets the Memo Program.
		if !progKey.Equals(MemoProgramID) {
			continue
		}

		// The memo data is the instruction data itself.
		memoData := inst.Data

		// Determine the sender (first signer of the transaction).
		var sender solanago.PublicKey
		if len(tx.Message.AccountKeys) > 0 {
			sender = tx.Message.AccountKeys[0]
		}

		// Determine the timestamp.
		var ts time.Time
		if blockTime != nil {
			ts = blockTime.Time()
		}

		memos = append(memos, MemoMessage{
			Sender:    sender,
			Data:      memoData,
			Timestamp: ts,
			Signature: signature,
		})
	}

	return memos
}

// RequestAirdrop requests an airdrop of the specified amount (in lamports)
// to the given address. Retries up to 3 times on transient errors.
func (c *Client) RequestAirdrop(address solanago.PublicKey, lamports uint64) (string, error) {
	ctx := context.Background()

	var lastErr error
	for i := 0; i < 3; i++ {
		sig, err := c.rpcClient.RequestAirdrop(ctx, address, lamports, rpc.CommitmentFinalized)
		if err == nil {
			return sig.String(), nil
		}
		lastErr = err
		msg := rpcErrorMessage(err)
		if strings.Contains(msg, "Internal error") || strings.Contains(msg, "Too many requests") {
			fmt.Fprintf(os.Stderr, "Rate limited, retrying in %d seconds... (%d/3)\n", (i+1)*5, i+1)
			time.Sleep(time.Duration((i+1)*5) * time.Second)
			continue
		}
		return "", fmt.Errorf("airdrop failed: %s", msg)
	}
	return "", fmt.Errorf("airdrop failed after 3 retries: %s", rpcErrorMessage(lastErr))
}

// ConfirmTransaction polls for transaction confirmation until the signature
// reaches "confirmed" or "finalized" status, or the timeout expires.
func (c *Client) ConfirmTransaction(signature string, timeout time.Duration) error {
	sig, err := solanago.SignatureFromBase58(signature)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("transaction confirmation timed out after %s", timeout)
		case <-ticker.C:
			result, err := c.rpcClient.GetSignatureStatuses(ctx, false, sig)
			if err != nil {
				continue // transient RPC errors, retry
			}
			if result == nil || result.Value == nil || len(result.Value) == 0 || result.Value[0] == nil {
				continue // not yet visible
			}
			status := result.Value[0]
			if status.Err != nil {
				return fmt.Errorf("transaction failed on-chain: %v", status.Err)
			}
			if status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
				status.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
				return nil
			}
		}
	}
}

// rpcToWsURL converts an HTTP(S) RPC URL to a WebSocket URL.
func rpcToWsURL(rpcURL string) string {
	wsURL := rpcURL
	if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	} else if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	}
	return wsURL
}
