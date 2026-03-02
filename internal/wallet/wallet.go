// Package wallet provides Solana wallet management for GhostLink,
// including keypair generation, import/export, encrypted storage,
// and on-chain balance queries.
package wallet

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/scrypt"
)

const (
	// DefaultStorageDir is the default directory for wallet files.
	DefaultStorageDir = ".ghostlink"

	// DefaultWalletFile is the default filename for the wallet.
	DefaultWalletFile = "wallet.json"

	// scrypt parameters (N=2^15, r=8, p=1 as recommended for interactive logins).
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32 // AES-256

	// saltSize is the byte length of the random salt for scrypt.
	saltSize = 32

	// mnemonicEntropyBits is the entropy size for a 24-word mnemonic.
	mnemonicEntropyBits = 256
)

// walletFile is the on-disk JSON format for a wallet.
// When encrypted: encrypted_key, salt, nonce are set, private_key is empty.
// When unencrypted: private_key is set, others are empty.
type walletFile struct {
	EncryptedKey string `json:"encrypted_key,omitempty"`
	Salt         string `json:"salt,omitempty"`
	Nonce        string `json:"nonce,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
}

// Wallet holds an Ed25519 keypair for use on Solana.
type Wallet struct {
	privateKey ed25519.PrivateKey
}

// NewWallet generates a new random Ed25519 keypair.
func NewWallet() (*Wallet, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}
	return &Wallet{privateKey: priv}, nil
}

// FromPrivateKey imports a wallet from a Base58-encoded private key.
// The decoded key must be exactly 64 bytes (ed25519 private key size).
func FromPrivateKey(base58Key string) (*Wallet, error) {
	decoded, err := base58.Decode(base58Key)
	if err != nil {
		return nil, fmt.Errorf("base58 decode: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: expected %d, got %d", ed25519.PrivateKeySize, len(decoded))
	}

	priv := ed25519.PrivateKey(decoded)

	// Verify consistency: the public key embedded in the last 32 bytes
	// of the private key must match what ed25519 derives from the seed.
	seed := priv.Seed()
	rebuilt := ed25519.NewKeyFromSeed(seed)
	if !rebuilt.Equal(priv) {
		return nil, fmt.Errorf("private key is internally inconsistent")
	}

	return &Wallet{privateKey: priv}, nil
}

// FromMnemonic imports a wallet from a BIP39 mnemonic phrase.
// It derives the seed using an empty passphrase and takes the first 32 bytes
// as the Ed25519 seed, matching the Solana CLI convention.
func FromMnemonic(mnemonic string) (*Wallet, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	// Derive seed with empty passphrase (Solana convention).
	seed := bip39.NewSeed(mnemonic, "")

	// Use the first 32 bytes as the Ed25519 seed.
	priv := ed25519.NewKeyFromSeed(seed[:32])
	return &Wallet{privateKey: priv}, nil
}

// NewMnemonic generates a new BIP39 mnemonic phrase (24 words, 256-bit entropy).
func NewMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(mnemonicEntropyBits)
	if err != nil {
		return "", fmt.Errorf("generate entropy: %w", err)
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("generate mnemonic: %w", err)
	}

	return mnemonic, nil
}

// PublicKey returns the Base58-encoded public key (Solana address).
func (w *Wallet) PublicKey() string {
	pub := w.privateKey.Public().(ed25519.PublicKey)
	return base58.Encode(pub)
}

// PrivateKey returns the raw 64-byte Ed25519 private key.
func (w *Wallet) PrivateKey() []byte {
	key := make([]byte, len(w.privateKey))
	copy(key, w.privateKey)
	return key
}

// Save writes the private key to the specified path.
// If password is non-empty, the key is encrypted with scrypt + AES-256-GCM.
// If password is empty, the key is stored as plaintext Base58.
func (w *Wallet) Save(path, password string) error {
	var wf walletFile

	if password == "" {
		// Store unencrypted.
		wf.PrivateKey = base58.Encode(w.privateKey)
	} else {
		// Generate random salt.
		salt := make([]byte, saltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return fmt.Errorf("generate salt: %w", err)
		}

		// Derive encryption key via scrypt.
		derivedKey, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
		if err != nil {
			return fmt.Errorf("derive key: %w", err)
		}

		// Encrypt with AES-256-GCM.
		block, err := aes.NewCipher(derivedKey)
		if err != nil {
			return fmt.Errorf("create cipher: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("create GCM: %w", err)
		}

		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return fmt.Errorf("generate nonce: %w", err)
		}

		ciphertext := gcm.Seal(nil, nonce, []byte(w.privateKey), nil)

		wf.EncryptedKey = base58.Encode(ciphertext)
		wf.Salt = base58.Encode(salt)
		wf.Nonce = base58.Encode(nonce)
	}

	data, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal wallet: %w", err)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write wallet file: %w", err)
	}

	return nil
}

// Load reads a wallet file from disk. If the wallet is encrypted, the
// password is used to decrypt it. If the wallet is unencrypted (no
// password was set), the password parameter is ignored.
func Load(path, password string) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wallet file: %w", err)
	}

	var wf walletFile
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse wallet file: %w", err)
	}

	// Unencrypted wallet: private_key field is set.
	if wf.PrivateKey != "" {
		decoded, err := base58.Decode(wf.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("decode private key: %w", err)
		}
		if len(decoded) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("invalid private key length: expected %d, got %d", ed25519.PrivateKeySize, len(decoded))
		}
		return &Wallet{privateKey: ed25519.PrivateKey(decoded)}, nil
	}

	// Encrypted wallet: need password.
	if password == "" {
		return nil, fmt.Errorf("wallet is encrypted, password required")
	}

	ciphertext, err := base58.Decode(wf.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted key: %w", err)
	}

	salt, err := base58.Decode(wf.Salt)
	if err != nil {
		return nil, fmt.Errorf("decode salt: %w", err)
	}

	nonce, err := base58.Decode(wf.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}

	// Derive the same encryption key.
	derivedKey, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// Decrypt with AES-256-GCM.
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt wallet: %w", err)
	}

	if len(plaintext) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("decrypted key has invalid length: expected %d, got %d", ed25519.PrivateKeySize, len(plaintext))
	}

	return &Wallet{privateKey: ed25519.PrivateKey(plaintext)}, nil
}

// IsEncrypted checks if a wallet file at the given path is encrypted.
func IsEncrypted(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var wf walletFile
	if err := json.Unmarshal(data, &wf); err != nil {
		return false, err
	}
	return wf.PrivateKey == "", nil
}

// DefaultWalletPath returns the default wallet file path: ~/.ghostlink/wallet.json
func DefaultWalletPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, DefaultStorageDir, DefaultWalletFile), nil
}

// GetBalance queries the SOL balance (in lamports) for the given address
// via the specified Solana RPC endpoint.
func GetBalance(rpcURL, address string) (uint64, error) {
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return 0, fmt.Errorf("invalid address %q: %w", address, err)
	}

	client := rpc.New(rpcURL)
	result, err := client.GetBalance(
		context.Background(),
		pubKey,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return 0, fmt.Errorf("get balance: %w", err)
	}

	return result.Value, nil
}
