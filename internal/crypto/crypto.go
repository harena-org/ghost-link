// Package crypto provides end-to-end encryption for GhostLink messages
// using NaCl box (Curve25519/XSalsa20/Poly1305) with Ed25519 key conversion.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/nacl/box"
)

const (
	// NonceSize is the size of the NaCl box nonce in bytes.
	NonceSize = 24

	// Poly1305Overhead is the authentication tag size added by NaCl box.
	Poly1305Overhead = box.Overhead // 16 bytes

	// SolanaMemoLimit is the maximum size of a Solana Memo instruction payload.
	SolanaMemoLimit = 512

	// maxCiphertext is the maximum ciphertext size (nonce + encrypted + auth tag).
	maxCiphertext = SolanaMemoLimit
)

var (
	// ErrMessageTooLarge is returned when the encrypted message would exceed
	// the Solana Memo limit of 512 bytes.
	ErrMessageTooLarge = errors.New("crypto: encrypted message exceeds 512 byte Solana Memo limit")

	// ErrDecryptionFailed is returned when NaCl box.Open fails, typically
	// due to an incorrect key or tampered ciphertext.
	ErrDecryptionFailed = errors.New("crypto: decryption failed (wrong key or corrupted data)")

	// ErrInvalidCiphertext is returned when the ciphertext is too short to
	// contain a nonce.
	ErrInvalidCiphertext = errors.New("crypto: ciphertext too short (must be at least 24 bytes)")

	// ErrKeyConversion is returned when Ed25519 to X25519 key conversion fails.
	ErrKeyConversion = errors.New("crypto: failed to convert Ed25519 key to X25519")
)

// MaxMessageSize returns the maximum plaintext message size in bytes that
// will fit within the 512-byte Solana Memo limit after encryption and
// base64 encoding.
//
// The encrypted binary format is: nonce (24 bytes) + ciphertext (plaintext + 16 bytes poly1305 tag).
// The binary data is then base64-encoded (4/3 expansion) to satisfy the Memo
// Program's UTF-8 requirement.
//
// base64_len = ceil(binary_len / 3) * 4
// We need base64_len <= 512, so binary_len <= 384 (512 / 4 * 3).
// max plaintext = 384 - 24 (nonce) - 16 (poly1305) = 344.
func MaxMessageSize() int {
	maxBinaryLen := SolanaMemoLimit / 4 * 3 // 384 bytes of raw binary fit in 512 bytes of base64
	return maxBinaryLen - NonceSize - Poly1305Overhead
}

// Encrypt encrypts a message using the recipient's Ed25519 public key and
// the sender's Ed25519 private key. It converts both keys to X25519 for
// NaCl box operations.
//
// The returned ciphertext has the format: nonce (24 bytes) || encrypted data.
// The total output is guaranteed to be at most 512 bytes.
func Encrypt(message []byte, recipientPubKey ed25519.PublicKey, senderPrivKey ed25519.PrivateKey) ([]byte, error) {
	if len(message) > MaxMessageSize() {
		return nil, fmt.Errorf("%w: message is %d bytes, max is %d", ErrMessageTooLarge, len(message), MaxMessageSize())
	}

	// Convert Ed25519 keys to X25519.
	recipientX25519, err := ed25519PubKeyToX25519(recipientPubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: recipient public key: %v", ErrKeyConversion, err)
	}

	senderX25519, err := ed25519PrivKeyToX25519(senderPrivKey)
	if err != nil {
		return nil, fmt.Errorf("%w: sender private key: %v", ErrKeyConversion, err)
	}

	// Generate a random 24-byte nonce.
	var nonce [NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Encrypt using NaCl box.
	// Seal appends the encrypted message to the first argument.
	// Output: nonce || box.Seal(message)
	out := make([]byte, NonceSize)
	copy(out, nonce[:])

	out = box.Seal(out, message, &nonce, recipientX25519, senderX25519)

	// Base64 encode to produce valid UTF-8 for Solana Memo Program.
	encoded := base64.StdEncoding.EncodeToString(out)

	if len(encoded) > SolanaMemoLimit {
		return nil, ErrMessageTooLarge
	}

	return []byte(encoded), nil
}

// Decrypt decrypts a message that was encrypted with Encrypt. It requires
// the sender's Ed25519 public key and the recipient's Ed25519 private key.
//
// The input is base64-encoded, which is decoded first to recover the binary
// format: nonce (24 bytes) || encrypted data.
func Decrypt(encrypted []byte, senderPubKey ed25519.PublicKey, recipientPrivKey ed25519.PrivateKey) ([]byte, error) {
	// Base64 decode first.
	raw, err := base64.StdEncoding.DecodeString(string(encrypted))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64: %v", ErrInvalidCiphertext, err)
	}

	if len(raw) < NonceSize+Poly1305Overhead {
		return nil, ErrInvalidCiphertext
	}

	// Extract the nonce from the first 24 bytes.
	var nonce [NonceSize]byte
	copy(nonce[:], raw[:NonceSize])
	ciphertext := raw[NonceSize:]

	// Convert Ed25519 keys to X25519.
	senderX25519, err := ed25519PubKeyToX25519(senderPubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: sender public key: %v", ErrKeyConversion, err)
	}

	recipientX25519, err := ed25519PrivKeyToX25519(recipientPrivKey)
	if err != nil {
		return nil, fmt.Errorf("%w: recipient private key: %v", ErrKeyConversion, err)
	}

	// Decrypt using NaCl box.
	plaintext, ok := box.Open(nil, ciphertext, &nonce, senderX25519, recipientX25519)
	if !ok {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// ed25519PubKeyToX25519 converts an Ed25519 public key to an X25519 public key
// using the birational map between the Edwards and Montgomery curves.
//
// This uses filippo.io/edwards25519 to parse the point and then
// BytesMontgomery() to compute u = (1+y)/(1-y).
func ed25519PubKeyToX25519(pub ed25519.PublicKey) (*[32]byte, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: %d", len(pub))
	}

	// Parse the Ed25519 public key as an edwards25519 point.
	point, err := new(edwards25519.Point).SetBytes(pub)
	if err != nil {
		return nil, fmt.Errorf("invalid Ed25519 public key: %w", err)
	}

	// Convert to Montgomery form (X25519 u-coordinate).
	montgomery := point.BytesMontgomery()

	var x25519Pub [32]byte
	copy(x25519Pub[:], montgomery)
	return &x25519Pub, nil
}

// ed25519PrivKeyToX25519 converts an Ed25519 private key to an X25519 private key.
//
// The conversion follows the Ed25519 key derivation: SHA-512 the 32-byte seed,
// take the first 32 bytes, and apply Curve25519 clamping.
func ed25519PrivKeyToX25519(priv ed25519.PrivateKey) (*[32]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d", len(priv))
	}

	// Ed25519 private key is seed (32 bytes) || public key (32 bytes).
	// The seed is the first 32 bytes.
	seed := priv.Seed()

	// Hash the seed with SHA-512, same as Ed25519 key derivation.
	h := sha512.Sum512(seed)

	// Take the first 32 bytes and apply Curve25519 clamping.
	var x25519Priv [32]byte
	copy(x25519Priv[:], h[:32])

	// Clamp: clear the three low bits to make it a multiple of 8,
	// clear the high bit and set the second-highest bit.
	x25519Priv[0] &= 248
	x25519Priv[31] &= 127
	x25519Priv[31] |= 64

	return &x25519Priv, nil
}
