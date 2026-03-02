// Package crypto provides end-to-end encryption for GhostLink messages
// using NaCl box (Curve25519/XSalsa20/Poly1305) with Ed25519 key conversion.
package crypto

import (
	"bytes"
	"compress/zlib"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

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

	// MagicPrefix is the 4-byte plaintext prefix for V1 wire format.
	// Enables O(1) filtering of GhostLink memos without decryption.
	MagicPrefix = "GL1:"

	// magicPrefixLen must match len(MagicPrefix).
	magicPrefixLen = 4

	// FlagNoCompression indicates the payload is not compressed.
	FlagNoCompression byte = 0x00

	// FlagZlibCompressed indicates the payload is zlib-compressed.
	FlagZlibCompressed byte = 0x01

	// maxBinaryV1 is the max binary bytes after base64-decoding the payload after the prefix.
	// (512 - 4) / 4 * 3 = 381
	maxBinaryV1 = (SolanaMemoLimit - magicPrefixLen) / 4 * 3

	// MaxPayloadV1 is the max inner plaintext (after NaCl overhead is removed).
	// 381 - 24 - 16 = 341
	MaxPayloadV1 = maxBinaryV1 - NonceSize - Poly1305Overhead

	// zlibDecompressLimit is a defense-in-depth cap on decompressed output.
	zlibDecompressLimit = 4096
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

// MaxMessageSizeV1 returns the maximum plaintext message size for the V1 wire
// format. The flag byte uses 1 byte of the inner plaintext, leaving 340 bytes
// for the (possibly compressed) payload.
func MaxMessageSizeV1() int {
	return MaxPayloadV1 - 1 // 340
}

// HasMagicPrefix returns true if data starts with the GL1: magic prefix.
func HasMagicPrefix(data []byte) bool {
	return bytes.HasPrefix(data, []byte(MagicPrefix))
}

// EncryptV1 encrypts a message using the V1 wire format:
//
//	GL1: + base64(nonce[24] || NaCl_box(flag[1] || payload))
//
// The message is zlib-compressed if compression reduces size. The flag byte
// indicates whether compression was applied. Returns ErrMessageTooLarge if the
// (possibly compressed) payload exceeds the V1 size budget.
func EncryptV1(message []byte, recipientPubKey ed25519.PublicKey, senderPrivKey ed25519.PrivateKey) ([]byte, error) {
	// Try compression.
	flag := FlagZlibCompressed
	payload, err := zlibCompress(message)
	if err != nil || len(payload) >= len(message) {
		// Compression unhelpful or failed — store raw.
		flag = FlagNoCompression
		payload = message
	}

	// 1 byte flag + payload must fit in MaxPayloadV1.
	innerLen := 1 + len(payload)
	if innerLen > MaxPayloadV1 {
		return nil, fmt.Errorf("%w: payload is %d bytes (after compression), max is %d",
			ErrMessageTooLarge, len(payload), MaxPayloadV1-1)
	}

	// Build inner plaintext: flag || payload.
	inner := make([]byte, innerLen)
	inner[0] = flag
	copy(inner[1:], payload)

	// Convert keys.
	recipientX25519, err := ed25519PubKeyToX25519(recipientPubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: recipient public key: %v", ErrKeyConversion, err)
	}
	senderX25519, err := ed25519PrivKeyToX25519(senderPrivKey)
	if err != nil {
		return nil, fmt.Errorf("%w: sender private key: %v", ErrKeyConversion, err)
	}

	// Generate nonce.
	var nonce [NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Encrypt.
	out := make([]byte, NonceSize)
	copy(out, nonce[:])
	out = box.Seal(out, inner, &nonce, recipientX25519, senderX25519)

	// Base64 encode and prepend magic prefix.
	encoded := MagicPrefix + base64.StdEncoding.EncodeToString(out)

	if len(encoded) > SolanaMemoLimit {
		return nil, ErrMessageTooLarge
	}

	return []byte(encoded), nil
}

// DecryptV1 decrypts a V1 wire-format message (must start with GL1:).
func DecryptV1(encrypted []byte, senderPubKey ed25519.PublicKey, recipientPrivKey ed25519.PrivateKey) ([]byte, error) {
	if !HasMagicPrefix(encrypted) {
		return nil, fmt.Errorf("%w: missing GL1: prefix", ErrInvalidCiphertext)
	}

	// Strip prefix and base64-decode.
	b64 := encrypted[magicPrefixLen:]
	raw, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64: %v", ErrInvalidCiphertext, err)
	}

	if len(raw) < NonceSize+Poly1305Overhead+1 { // +1 for flag byte
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce.
	var nonce [NonceSize]byte
	copy(nonce[:], raw[:NonceSize])
	ciphertext := raw[NonceSize:]

	// Convert keys.
	senderX25519, err := ed25519PubKeyToX25519(senderPubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: sender public key: %v", ErrKeyConversion, err)
	}
	recipientX25519, err := ed25519PrivKeyToX25519(recipientPrivKey)
	if err != nil {
		return nil, fmt.Errorf("%w: recipient private key: %v", ErrKeyConversion, err)
	}

	// Decrypt.
	inner, ok := box.Open(nil, ciphertext, &nonce, senderX25519, recipientX25519)
	if !ok {
		return nil, ErrDecryptionFailed
	}

	if len(inner) < 1 {
		return nil, fmt.Errorf("%w: empty inner plaintext", ErrInvalidCiphertext)
	}

	flag := inner[0]
	payload := inner[1:]

	switch flag {
	case FlagNoCompression:
		return payload, nil
	case FlagZlibCompressed:
		return zlibDecompress(payload)
	default:
		return nil, fmt.Errorf("%w: unknown flag byte 0x%02x", ErrInvalidCiphertext, flag)
	}
}

// zlibCompress compresses src using zlib at BestCompression level.
func zlibCompress(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(src); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// zlibDecompress decompresses src with a safety cap of zlibDecompressLimit bytes.
func zlibDecompress(src []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("%w: zlib header: %v", ErrInvalidCiphertext, err)
	}
	defer r.Close()

	data, err := io.ReadAll(io.LimitReader(r, zlibDecompressLimit))
	if err != nil {
		return nil, fmt.Errorf("%w: zlib decompress: %v", ErrInvalidCiphertext, err)
	}
	return data, nil
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
