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

	paths := []*landlock.Path{
		landlock.Dir(cleanStorageDir, "rwc"),
		landlock.Tmp(),
		landlock.Certs(),
		landlock.DNS(),
		landlock.Shared(),
	}

	// Safe system files and essential read-only paths.
	// Note: We deliberately avoid landlock.Stdio() because its inclusion of /dev/stdout
	// and /dev/stdin fails with EBADFD ("file descriptor in bad state") whenever the process
	// stdout/stdin is connected to a pipe, socket, or redirected stream.
	// Inherited standard streams (fd 0, 1, 2) remain fully functional without needing path-beneath rules.
	optionalPaths := []struct {
		path string
		mode string
		dir  bool
	}{
		{"/dev/null", "rw", false},
		{"/dev/zero", "r", false},
		{"/dev/urandom", "r", false},
		{"/usr/share/zoneinfo", "r", true},
		{"/proc/self/cmdline", "r", false},
	}

	for _, op := range optionalPaths {
		if fi, err := os.Stat(op.path); err == nil {
			if op.dir && fi.IsDir() {
				paths = append(paths, landlock.Dir(op.path, op.mode))
			} else if !op.dir && !fi.IsDir() {
				paths = append(paths, landlock.File(op.path, op.mode))
			}
		}
	}

	locker := landlock.New(paths...)

	// OnlyAvailable ensures compatibility with Linux kernels that don't support Landlock
	err := locker.Lock(landlock.OnlyAvailable)
	if err != nil {
		slog.Warn("Landlock sandboxing failed to lock filesystem", "error", err)
		return err
	}

	slog.Info("Linux Landlock filesystem sandboxing applied successfully", "storage_dir", cleanStorageDir)
	return nil
}
