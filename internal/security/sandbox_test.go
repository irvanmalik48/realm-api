package security

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/irvanmalik48/realm-api/internal/telemetry"
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

	if _, err := os.ReadFile("/proc/stat"); err != nil {
		t.Logf("Failed to read /proc/stat after Landlock: %v", err)
	} else {
		t.Logf("Successfully read /proc/stat after Landlock")
	}

	if _, err := os.ReadFile("/proc/loadavg"); err != nil {
		t.Logf("Failed to read /proc/loadavg after Landlock: %v", err)
	} else {
		t.Logf("Successfully read /proc/loadavg after Landlock")
	}

	if data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); err != nil {
		t.Logf("Failed to read scaling_cur_freq after Landlock: %v", err)
	} else {
		t.Logf("Successfully read scaling_cur_freq after Landlock: %s", string(data))
	}

	mon := telemetry.DefaultCPUMonitor()
	time.Sleep(200 * time.Millisecond)
	stats := mon.GetStats()
	t.Logf("CPUMonitor stats after Landlock: Usage=%.2f%%, AvgFreq=%.2fMHz, MinFreq=%.2fMHz, MaxFreq=%.2fMHz, Load1=%.2f, Load5=%.2f, Cores=%d, Model=%s",
		stats.UsagePercent, stats.AvgFrequencyMHz, stats.MinFrequencyMHz, stats.MaxFrequencyMHz, stats.Load1m, stats.Load5m, stats.CoreCount, stats.ModelName)

	if stats.CoreCount <= 0 {
		t.Errorf("expected positive core count, got %d", stats.CoreCount)
	}
	if stats.AvgFrequencyMHz <= 0 {
		t.Errorf("expected positive avg frequency, got %f", stats.AvgFrequencyMHz)
	}
}
