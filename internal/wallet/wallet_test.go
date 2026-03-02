package wallet

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
)

func TestNewWallet(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	// Private key should be 64 bytes (ed25519).
	if len(w.PrivateKey()) != ed25519.PrivateKeySize {
		t.Errorf("private key length = %d, want %d", len(w.PrivateKey()), ed25519.PrivateKeySize)
	}

	// Public key should be a valid Base58 string that decodes to 32 bytes.
	pubBytes, err := base58.Decode(w.PublicKey())
	if err != nil {
		t.Fatalf("PublicKey() is not valid Base58: %v", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		t.Errorf("public key decoded length = %d, want %d", len(pubBytes), ed25519.PublicKeySize)
	}
}

func TestNewWallet_Uniqueness(t *testing.T) {
	w1, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}
	w2, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	if w1.PublicKey() == w2.PublicKey() {
		t.Error("two newly generated wallets have the same public key")
	}
}

func TestFromPrivateKey(t *testing.T) {
	// Generate a wallet, export its key, and reimport.
	original, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	b58Key := base58.Encode(original.PrivateKey())
	imported, err := FromPrivateKey(b58Key)
	if err != nil {
		t.Fatalf("FromPrivateKey() error: %v", err)
	}

	if imported.PublicKey() != original.PublicKey() {
		t.Errorf("imported public key = %s, want %s", imported.PublicKey(), original.PublicKey())
	}
}

func TestFromPrivateKey_InvalidLength(t *testing.T) {
	// Encode only 32 bytes, which is too short.
	short := make([]byte, 32)
	b58Key := base58.Encode(short)

	_, err := FromPrivateKey(b58Key)
	if err == nil {
		t.Fatal("FromPrivateKey() expected error for short key, got nil")
	}
}

func TestFromPrivateKey_InvalidBase58(t *testing.T) {
	_, err := FromPrivateKey("not-valid-base58!!!")
	if err == nil {
		t.Fatal("FromPrivateKey() expected error for invalid base58, got nil")
	}
}

func TestNewMnemonic(t *testing.T) {
	mnemonic, err := NewMnemonic()
	if err != nil {
		t.Fatalf("NewMnemonic() error: %v", err)
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		t.Errorf("NewMnemonic() returned an invalid mnemonic: %s", mnemonic)
	}
}

func TestFromMnemonic(t *testing.T) {
	mnemonic, err := NewMnemonic()
	if err != nil {
		t.Fatalf("NewMnemonic() error: %v", err)
	}

	w, err := FromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("FromMnemonic() error: %v", err)
	}

	// Public key must be non-empty and valid.
	pubBytes, err := base58.Decode(w.PublicKey())
	if err != nil {
		t.Fatalf("PublicKey() is not valid Base58: %v", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		t.Errorf("public key decoded length = %d, want %d", len(pubBytes), ed25519.PublicKeySize)
	}
}

func TestFromMnemonic_Deterministic(t *testing.T) {
	mnemonic, err := NewMnemonic()
	if err != nil {
		t.Fatalf("NewMnemonic() error: %v", err)
	}

	w1, err := FromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("FromMnemonic() error: %v", err)
	}
	w2, err := FromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("FromMnemonic() error: %v", err)
	}

	if w1.PublicKey() != w2.PublicKey() {
		t.Error("same mnemonic produced different public keys")
	}

	pk1 := w1.PrivateKey()
	pk2 := w2.PrivateKey()
	if len(pk1) != len(pk2) {
		t.Fatal("same mnemonic produced different private key lengths")
	}
	for i := range pk1 {
		if pk1[i] != pk2[i] {
			t.Error("same mnemonic produced different private keys")
			break
		}
	}
}

func TestFromMnemonic_InvalidMnemonic(t *testing.T) {
	_, err := FromMnemonic("this is not a valid mnemonic phrase at all")
	if err == nil {
		t.Fatal("FromMnemonic() expected error for invalid mnemonic, got nil")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	walletPath := filepath.Join(tmpDir, "test_wallet.json")
	password := "test-password-123!"

	// Create and save a wallet.
	original, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	if err := original.Save(walletPath, password); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the file exists and is not readable as plaintext key.
	data, err := os.ReadFile(walletPath)
	if err != nil {
		t.Fatalf("failed to read wallet file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("wallet file is empty")
	}

	// Load the wallet back.
	loaded, err := Load(walletPath, password)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.PublicKey() != original.PublicKey() {
		t.Errorf("loaded public key = %s, want %s", loaded.PublicKey(), original.PublicKey())
	}

	origKey := original.PrivateKey()
	loadedKey := loaded.PrivateKey()
	if len(origKey) != len(loadedKey) {
		t.Fatal("loaded private key length differs from original")
	}
	for i := range origKey {
		if origKey[i] != loadedKey[i] {
			t.Error("loaded private key differs from original")
			break
		}
	}
}

func TestSaveAndLoad_WrongPassword(t *testing.T) {
	tmpDir := t.TempDir()
	walletPath := filepath.Join(tmpDir, "test_wallet.json")

	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	if err := w.Save(walletPath, "correct-password"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	_, err = Load(walletPath, "wrong-password")
	if err == nil {
		t.Fatal("Load() expected error with wrong password, got nil")
	}
}

func TestSave_EmptyPassword(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	walletPath := filepath.Join(t.TempDir(), "unencrypted.json")
	err = w.Save(walletPath, "")
	if err != nil {
		t.Fatalf("Save() with empty password should succeed: %v", err)
	}

	// Load without password should work.
	loaded, err := Load(walletPath, "")
	if err != nil {
		t.Fatalf("Load() unencrypted wallet failed: %v", err)
	}

	if loaded.PublicKey() != w.PublicKey() {
		t.Errorf("loaded key mismatch: got %s, want %s", loaded.PublicKey(), w.PublicKey())
	}
}

func TestSave_UnencryptedFileHasPrivateKey(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	walletPath := filepath.Join(t.TempDir(), "plain.json")
	if err := w.Save(walletPath, ""); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(walletPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var wf walletFile
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if wf.PrivateKey == "" {
		t.Error("unencrypted wallet should have private_key field")
	}
	if wf.EncryptedKey != "" || wf.Salt != "" || wf.Nonce != "" {
		t.Error("unencrypted wallet should not have encrypted fields")
	}
}

func TestLoad_EncryptedRequiresPassword(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	walletPath := filepath.Join(t.TempDir(), "encrypted.json")
	if err := w.Save(walletPath, "mypassword"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	_, err = Load(walletPath, "")
	if err == nil {
		t.Fatal("Load() encrypted wallet with empty password should fail")
	}
}

func TestIsEncrypted(t *testing.T) {
	w, _ := NewWallet()

	encPath := filepath.Join(t.TempDir(), "enc.json")
	w.Save(encPath, "pass")
	enc, _ := IsEncrypted(encPath)
	if !enc {
		t.Error("expected encrypted wallet to return true")
	}

	plainPath := filepath.Join(t.TempDir(), "plain.json")
	w.Save(plainPath, "")
	enc, _ = IsEncrypted(plainPath)
	if enc {
		t.Error("expected unencrypted wallet to return false")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/tmp/does_not_exist_wallet_file.json", "password")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent file, got nil")
	}
}

func TestSave_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c", "wallet.json")

	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	if err := w.Save(nested, "password"); err != nil {
		t.Fatalf("Save() should create parent directories: %v", err)
	}

	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Fatal("wallet file was not created")
	}
}

func TestSave_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	walletPath := filepath.Join(tmpDir, "wallet.json")

	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	if err := w.Save(walletPath, "password"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(walletPath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestEncryptedWallet_JSONStructure(t *testing.T) {
	tmpDir := t.TempDir()
	walletPath := filepath.Join(tmpDir, "wallet.json")

	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	if err := w.Save(walletPath, "password"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(walletPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var ew walletFile
	if err := json.Unmarshal(data, &ew); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	if ew.EncryptedKey == "" {
		t.Error("encrypted_key field is empty")
	}
	if ew.Salt == "" {
		t.Error("salt field is empty")
	}
	if ew.Nonce == "" {
		t.Error("nonce field is empty")
	}

	// Verify all fields are valid Base58.
	if _, err := base58.Decode(ew.EncryptedKey); err != nil {
		t.Errorf("encrypted_key is not valid Base58: %v", err)
	}
	if _, err := base58.Decode(ew.Salt); err != nil {
		t.Errorf("salt is not valid Base58: %v", err)
	}
	if _, err := base58.Decode(ew.Nonce); err != nil {
		t.Errorf("nonce is not valid Base58: %v", err)
	}
}

func TestPrivateKey_ReturnsCopy(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	key1 := w.PrivateKey()
	key2 := w.PrivateKey()

	// Mutating the returned slice should not affect the wallet.
	key1[0] ^= 0xFF

	if key1[0] == key2[0] {
		t.Error("PrivateKey() returns the same underlying slice, should return a copy")
	}
}

func TestDefaultWalletPath(t *testing.T) {
	path, err := DefaultWalletPath()
	if err != nil {
		t.Fatalf("DefaultWalletPath() error: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}

	expected := filepath.Join(home, DefaultStorageDir, DefaultWalletFile)
	if path != expected {
		t.Errorf("DefaultWalletPath() = %s, want %s", path, expected)
	}
}

func TestMnemonicRoundTrip(t *testing.T) {
	// Generate a mnemonic, create a wallet, save it, load it,
	// and verify the key matches the mnemonic-derived key.
	mnemonic, err := NewMnemonic()
	if err != nil {
		t.Fatalf("NewMnemonic() error: %v", err)
	}

	w1, err := FromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("FromMnemonic() error: %v", err)
	}

	tmpDir := t.TempDir()
	walletPath := filepath.Join(tmpDir, "wallet.json")
	password := "mnemonic-test-password"

	if err := w1.Save(walletPath, password); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	w2, err := Load(walletPath, password)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Re-derive from same mnemonic.
	w3, err := FromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("FromMnemonic() error: %v", err)
	}

	if w1.PublicKey() != w2.PublicKey() || w2.PublicKey() != w3.PublicKey() {
		t.Error("mnemonic round-trip produced inconsistent public keys")
	}
}

func TestSignAndVerify(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet() error: %v", err)
	}

	message := []byte("hello ghostlink")

	priv := ed25519.PrivateKey(w.PrivateKey())
	sig := ed25519.Sign(priv, message)

	pubBytes, err := base58.Decode(w.PublicKey())
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	pub := ed25519.PublicKey(pubBytes)

	if !ed25519.Verify(pub, message, sig) {
		t.Error("signature verification failed")
	}
}

// TestGetBalance_InvalidAddress verifies that GetBalance returns an error
// for a malformed address. We do not test against a real RPC endpoint
// in unit tests to avoid network dependencies.
func TestGetBalance_InvalidAddress(t *testing.T) {
	_, err := GetBalance("https://api.mainnet-beta.solana.com", "not-a-valid-address!!!")
	if err == nil {
		t.Fatal("GetBalance() expected error for invalid address, got nil")
	}
}
