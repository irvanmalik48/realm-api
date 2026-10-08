package telemetry

import (
	"testing"
	"time"
)

func TestCPUMonitor(t *testing.T) {
	monitor := NewCPUMonitor(200 * time.Millisecond)
	defer monitor.Stop()

	statsImmediate := monitor.GetStats()
	if statsImmediate.CoreCount <= 0 {
		t.Errorf("expected positive core count, got %d", statsImmediate.CoreCount)
	}

	// Allow one tick
	time.Sleep(300 * time.Millisecond)

	stats := monitor.GetStats()
	if stats.CoreCount <= 0 {
		t.Errorf("expected positive core count, got %d", stats.CoreCount)
	}

	if stats.ModelName == "" {
		t.Errorf("expected non-empty model name")
	}

	// Usage percent should be between 0 and 100
	if stats.UsagePercent < 0 || stats.UsagePercent > 100 {
		t.Errorf("expected usage percent in [0, 100], got %f", stats.UsagePercent)
	}

	// If frequencies were read, average should be non-negative
	if stats.AvgFrequencyMHz < 0 {
		t.Errorf("expected non-negative avg frequency, got %f", stats.AvgFrequencyMHz)
	}
}
