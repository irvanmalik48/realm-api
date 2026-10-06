package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyLandlockSandbox(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "realm-test-storage")
	defer os.RemoveAll(tempDir)

	err := ApplyLandlockSandbox(tempDir)
	if err != nil {
		t.Fatalf("ApplyLandlockSandbox failed: %v", err)
	}

	// Verify we can write to allowed temp/storage directory
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("Failed to write to allowed storage directory: %v", err)
	}
}
