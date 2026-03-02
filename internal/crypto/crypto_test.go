package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

// generateTestKeyPair generates a fresh Ed25519 key pair for testing.
func generateTestKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key pair: %v", err)
	}
	return pub, priv
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	senderPub, senderPriv := generateTestKeyPair(t)
	recipientPub, recipientPriv := generateTestKeyPair(t)

	message := []byte("Hello, GhostLink! This is a secret message on Solana.")

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Output should be valid base64.
	_, err = base64.StdEncoding.DecodeString(string(encrypted))
	if err != nil {
		t.Fatalf("encrypted output is not valid base64: %v", err)
	}

	// Output must fit in 512 bytes (Solana Memo limit).
	if len(encrypted) > SolanaMemoLimit {
		t.Fatalf("encrypted output %d bytes exceeds Memo limit %d", len(encrypted), SolanaMemoLimit)
	}

	decrypted, err := Decrypt(encrypted, senderPub, recipientPriv)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, message) {
		t.Errorf("decrypted message = %q, want %q", decrypted, message)
	}
}

func TestEncryptDecryptEmptyMessage(t *testing.T) {
	senderPub, senderPriv := generateTestKeyPair(t)
	recipientPub, recipientPriv := generateTestKeyPair(t)

	message := []byte{}

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed for empty message: %v", err)
	}

	decrypted, err := Decrypt(encrypted, senderPub, recipientPriv)
	if err != nil {
		t.Fatalf("Decrypt failed for empty message: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("decrypted empty message has length %d, want 0", len(decrypted))
	}
}

func TestDecryptWrongRecipientKey(t *testing.T) {
	senderPub, senderPriv := generateTestKeyPair(t)
	recipientPub, _ := generateTestKeyPair(t)
	_, wrongPriv := generateTestKeyPair(t)

	message := []byte("This message should not be readable with the wrong key.")

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, senderPub, wrongPriv)
	if err == nil {
		t.Fatal("Decrypt with wrong recipient key should have failed")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestDecryptWrongSenderKey(t *testing.T) {
	_, senderPriv := generateTestKeyPair(t)
	recipientPub, recipientPriv := generateTestKeyPair(t)
	wrongPub, _ := generateTestKeyPair(t)

	message := []byte("Wrong sender key should fail decryption.")

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, wrongPub, recipientPriv)
	if err == nil {
		t.Fatal("Decrypt with wrong sender key should have failed")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	senderPub, senderPriv := generateTestKeyPair(t)
	recipientPub, recipientPriv := generateTestKeyPair(t)

	message := []byte("Tampered ciphertext should fail.")

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Decode base64, tamper, re-encode.
	raw, _ := base64.StdEncoding.DecodeString(string(encrypted))
	raw[NonceSize+5] ^= 0xff
	tampered := []byte(base64.StdEncoding.EncodeToString(raw))

	_, err = Decrypt(tampered, senderPub, recipientPriv)
	if err == nil {
		t.Fatal("Decrypt of tampered ciphertext should have failed")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestEncryptMessageTooLarge(t *testing.T) {
	_, senderPriv := generateTestKeyPair(t)
	recipientPub, _ := generateTestKeyPair(t)

	// Create a message exactly 1 byte over the limit.
	message := make([]byte, MaxMessageSize()+1)
	for i := range message {
		message[i] = 'A'
	}

	_, err := Encrypt(message, recipientPub, senderPriv)
	if err == nil {
		t.Fatal("Encrypt should have failed for oversized message")
	}
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("expected ErrMessageTooLarge, got: %v", err)
	}
}

func TestEncryptMaxSizeMessage(t *testing.T) {
	senderPub, senderPriv := generateTestKeyPair(t)
	recipientPub, recipientPriv := generateTestKeyPair(t)

	message := make([]byte, MaxMessageSize())
	for i := range message {
		message[i] = byte(i % 256)
	}

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed for max-size message: %v", err)
	}

	if len(encrypted) > SolanaMemoLimit {
		t.Errorf("encrypted output = %d bytes, exceeds Solana Memo limit of %d", len(encrypted), SolanaMemoLimit)
	}

	// Should be exactly 512 bytes (base64 of 384 bytes binary).
	if len(encrypted) != SolanaMemoLimit {
		t.Errorf("encrypted output = %d bytes, want exactly %d for max-size message", len(encrypted), SolanaMemoLimit)
	}

	decrypted, err := Decrypt(encrypted, senderPub, recipientPriv)
	if err != nil {
		t.Fatalf("Decrypt failed for max-size message: %v", err)
	}

	if !bytes.Equal(decrypted, message) {
		t.Error("decrypted max-size message does not match original")
	}
}

func TestMaxMessageSize(t *testing.T) {
	// 512 bytes base64 = 384 bytes binary. 384 - 24 nonce - 16 poly1305 = 344.
	expected := 344
	got := MaxMessageSize()

	if got != expected {
		t.Errorf("MaxMessageSize() = %d, want %d", got, expected)
	}
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	senderPub, _ := generateTestKeyPair(t)
	_, recipientPriv := generateTestKeyPair(t)

	// Base64 of data shorter than nonce + poly1305 overhead.
	shortData := make([]byte, NonceSize+Poly1305Overhead-1)
	encoded := []byte(base64.StdEncoding.EncodeToString(shortData))

	_, err := Decrypt(encoded, senderPub, recipientPriv)
	if err == nil {
		t.Fatal("Decrypt should have failed for too-short ciphertext")
	}
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("expected ErrInvalidCiphertext, got: %v", err)
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	senderPub, _ := generateTestKeyPair(t)
	_, recipientPriv := generateTestKeyPair(t)

	_, err := Decrypt([]byte("not!valid@base64###"), senderPub, recipientPriv)
	if err == nil {
		t.Fatal("Decrypt should have failed for invalid base64")
	}
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("expected ErrInvalidCiphertext, got: %v", err)
	}
}

func TestEncryptOutputIsValidUTF8(t *testing.T) {
	_, senderPriv := generateTestKeyPair(t)
	recipientPub, _ := generateTestKeyPair(t)

	message := []byte("Test UTF-8 validity for Solana Memo Program")

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Base64 output is always valid ASCII (subset of UTF-8).
	for i, b := range encrypted {
		if b > 127 {
			t.Fatalf("byte %d is non-ASCII (%d), base64 output should be ASCII", i, b)
		}
	}
}

func TestEncryptDecryptDifferentMessages(t *testing.T) {
	senderPub, senderPriv := generateTestKeyPair(t)
	recipientPub, recipientPriv := generateTestKeyPair(t)

	messages := []string{
		"Short",
		"A slightly longer message with some content.",
		"Unicode works too: GhostLink 幽灵链路",
		string(make([]byte, 100)),
		string(make([]byte, 200)),
	}

	for _, msg := range messages {
		message := []byte(msg)

		encrypted, err := Encrypt(message, recipientPub, senderPriv)
		if err != nil {
			t.Fatalf("Encrypt failed for message of length %d: %v", len(message), err)
		}

		decrypted, err := Decrypt(encrypted, senderPub, recipientPriv)
		if err != nil {
			t.Fatalf("Decrypt failed for message of length %d: %v", len(message), err)
		}

		if !bytes.Equal(decrypted, message) {
			t.Errorf("roundtrip failed for message of length %d", len(message))
		}
	}
}

func TestEncryptProducesUniqueOutputs(t *testing.T) {
	_, senderPriv := generateTestKeyPair(t)
	recipientPub, _ := generateTestKeyPair(t)

	message := []byte("Same message encrypted twice should have different nonces.")

	encrypted1, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("First Encrypt failed: %v", err)
	}

	encrypted2, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Second Encrypt failed: %v", err)
	}

	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("two encryptions of the same message produced identical ciphertext")
	}
}

func TestKeyConversionConsistency(t *testing.T) {
	pub, priv := generateTestKeyPair(t)

	x25519Pub1, err := ed25519PubKeyToX25519(pub)
	if err != nil {
		t.Fatalf("first public key conversion failed: %v", err)
	}

	x25519Pub2, err := ed25519PubKeyToX25519(pub)
	if err != nil {
		t.Fatalf("second public key conversion failed: %v", err)
	}

	if *x25519Pub1 != *x25519Pub2 {
		t.Error("public key conversion is not deterministic")
	}

	x25519Priv1, err := ed25519PrivKeyToX25519(priv)
	if err != nil {
		t.Fatalf("first private key conversion failed: %v", err)
	}

	x25519Priv2, err := ed25519PrivKeyToX25519(priv)
	if err != nil {
		t.Fatalf("second private key conversion failed: %v", err)
	}

	if *x25519Priv1 != *x25519Priv2 {
		t.Error("private key conversion is not deterministic")
	}
}

func BenchmarkEncrypt(b *testing.B) {
	_, senderPriv, _ := ed25519.GenerateKey(rand.Reader)
	recipientPub, _, _ := ed25519.GenerateKey(rand.Reader)
	message := []byte("Benchmark message for GhostLink encryption.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(message, recipientPub, senderPriv)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	senderPub, senderPriv, _ := ed25519.GenerateKey(rand.Reader)
	recipientPub, recipientPriv, _ := ed25519.GenerateKey(rand.Reader)
	message := []byte("Benchmark message for GhostLink decryption.")

	encrypted, err := Encrypt(message, recipientPub, senderPriv)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decrypt(encrypted, senderPub, recipientPriv)
		if err != nil {
			b.Fatal(err)
		}
	}
}
