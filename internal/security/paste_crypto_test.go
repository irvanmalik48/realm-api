package security

import (
	"strings"
	"testing"
)

func TestEncryptDecryptPaste(t *testing.T) {
	plaintext := "const secretApiKey = 'sk-live-1234567890abcdef';"
	password := "super_secret_p@ssw0rd!"

	// 1. Encrypt
	encrypted, err := EncryptPaste(plaintext, password)
	if err != nil {
		t.Fatalf("failed to encrypt paste: %v", err)
	}

	if !strings.HasPrefix(encrypted, "$paste_v1$") {
		t.Errorf("expected $paste_v1$ prefix, got %q", encrypted)
	}

	if !IsEncryptedPaste(encrypted) {
		t.Errorf("IsEncryptedPaste should return true for %q", encrypted)
	}

	if IsEncryptedPaste(plaintext) {
		t.Errorf("IsEncryptedPaste should return false for plaintext")
	}

	// 2. Decrypt with correct password
	decrypted, err := DecryptPaste(encrypted, password)
	if err != nil {
		t.Fatalf("failed to decrypt paste: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected plaintext %q, got %q", plaintext, decrypted)
	}

	// 3. Decrypt with empty password
	_, err = DecryptPaste(encrypted, "")
	if err != ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired for empty password, got %v", err)
	}

	// 4. Decrypt with wrong password
	_, err = DecryptPaste(encrypted, "wrong-password")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword for wrong password, got %v", err)
	}

	// 5. Corrupted envelope
	_, err = DecryptPaste("$paste_v1$not-base64", password)
	if err != ErrCorruptedCiphertext {
		t.Errorf("expected ErrCorruptedCiphertext, got %v", err)
	}
}
