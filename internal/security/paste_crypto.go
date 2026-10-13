package security

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrPasswordRequired    = errors.New("paste is password-protected and requires a password")
	ErrInvalidPassword     = errors.New("invalid password for encrypted paste")
	ErrCorruptedCiphertext = errors.New("corrupted encrypted paste format")
)

const (
	pasteEnvelopePrefix = "$paste_v1$"

	// Argon2id parameters recommended by RFC 9106
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32        // 256-bit key for XChaCha20-Poly1305
)

// EncryptedEnvelope is the self-describing cryptographic container for password-encrypted pastes.
type EncryptedEnvelope struct {
	Version int    `json:"v"`
	KDF     string `json:"kdf"`
	Cipher  string `json:"cipher"`
	Salt    string `json:"salt"`
	Nonce   string `json:"nonce"`
	Time    uint32 `json:"t"`
	Memory  uint32 `json:"m"`
	Threads uint8  `json:"p"`
	Data    string `json:"data"`
}

// EncryptPaste encrypts plaintext using Argon2id key derivation and XChaCha20-Poly1305 AEAD.
func EncryptPaste(plaintext, password string) (string, error) {
	if password == "" {
		return "", errors.New("encryption password cannot be empty")
	}

	// 16-byte cryptographically secure random salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	// Derive 256-bit key via Argon2id (RFC 9106)
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Create XChaCha20-Poly1305 AEAD cipher (256-bit key, 192-bit nonce)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("failed to initialize AEAD cipher: %w", err)
	}

	// 24-byte (192-bit) cryptographically secure random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)

	envelope := EncryptedEnvelope{
		Version: 1,
		KDF:     "argon2id",
		Cipher:  "xchacha20poly1305",
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Nonce:   base64.StdEncoding.EncodeToString(nonce),
		Time:    argonTime,
		Memory:  argonMemory,
		Threads: argonThreads,
		Data:    base64.StdEncoding.EncodeToString(ciphertext),
	}

	envBytes, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("failed to serialize crypto envelope: %w", err)
	}

	return pasteEnvelopePrefix + base64.StdEncoding.EncodeToString(envBytes), nil
}

// DecryptPaste decrypts an encrypted paste envelope using the provided password.
func DecryptPaste(envelopeStr, password string) (string, error) {
	if password == "" {
		return "", ErrPasswordRequired
	}

	if len(envelopeStr) <= len(pasteEnvelopePrefix) || envelopeStr[:len(pasteEnvelopePrefix)] != pasteEnvelopePrefix {
		return "", ErrCorruptedCiphertext
	}

	rawJSON, err := base64.StdEncoding.DecodeString(envelopeStr[len(pasteEnvelopePrefix):])
	if err != nil {
		return "", ErrCorruptedCiphertext
	}

	var env EncryptedEnvelope
	if err := json.Unmarshal(rawJSON, &env); err != nil {
		return "", ErrCorruptedCiphertext
	}

	if env.Version != 1 || env.KDF != "argon2id" || env.Cipher != "xchacha20poly1305" {
		return "", errors.New("unsupported crypto envelope version or algorithms")
	}

	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil || len(salt) == 0 {
		return "", ErrCorruptedCiphertext
	}

	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil || len(nonce) == 0 {
		return "", ErrCorruptedCiphertext
	}

	ciphertext, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil || len(ciphertext) == 0 {
		return "", ErrCorruptedCiphertext
	}

	// Derive key from password and salt
	key := argon2.IDKey([]byte(password), salt, env.Time, env.Memory, env.Threads, argonKeyLen)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("failed to initialize AEAD cipher: %w", err)
	}

	if len(nonce) != aead.NonceSize() {
		return "", ErrCorruptedCiphertext
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrInvalidPassword
	}

	return string(plaintext), nil
}

// IsEncryptedPaste checks if the content string is formatted as an encrypted paste envelope.
func IsEncryptedPaste(content string) bool {
	return len(content) > len(pasteEnvelopePrefix) && content[:len(pasteEnvelopePrefix)] == pasteEnvelopePrefix
}
