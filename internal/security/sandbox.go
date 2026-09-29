package security

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/shoenig/go-landlock"
)

// ApplyLandlockSandbox restricts the process filesystem access using Linux Landlock LSM.
// It confines filesystem mutations strictly to the configured storage directory and /tmp.
func ApplyLandlockSandbox(storageDir string) error {
	if runtime.GOOS != "linux" {
		slog.Debug("Landlock sandboxing skipped: OS is not Linux", "os", runtime.GOOS)
		return nil
	}

	cleanStorageDir := filepath.Clean(storageDir)
	_ = os.MkdirAll(cleanStorageDir, 0750)

	locker := landlock.New(
		landlock.Dir(cleanStorageDir, "rwc"),
		landlock.Tmp(),
		landlock.Certs(),
		landlock.DNS(),
		landlock.Stdio(),
		landlock.Shared(),
	)

	// OnlyAvailable ensures compatibility with Linux kernels that don't support Landlock
	err := locker.Lock(landlock.OnlyAvailable)
	if err != nil {
		slog.Warn("Landlock sandboxing failed to lock filesystem", "error", err)
		return err
	}

	slog.Info("Linux Landlock filesystem sandboxing applied successfully", "storage_dir", cleanStorageDir)
	return nil
}
