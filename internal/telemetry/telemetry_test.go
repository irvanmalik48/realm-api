package telemetry

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestInitTelemetry(t *testing.T) {
	ctx := context.Background()
	shutdown, err := InitTelemetry(ctx, "realm-test", "test")
	if err != nil {
		t.Fatalf("InitTelemetry failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	if Tracer() == nil {
		t.Error("expected non-nil tracer")
	}

	if Meter() == nil {
		t.Error("expected non-nil meter")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}
}

func TestRegisterSystemMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	customMeter := mp.Meter("test-meter")
	mon := NewCPUMonitor(100 * time.Millisecond)
	defer mon.Stop()

	err := RegisterSystemMetrics(customMeter, mon)
	if err != nil {
		t.Fatalf("RegisterSystemMetrics failed: %v", err)
	}

	// Trigger manual collection to run callbacks
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("expected scope metrics to be collected")
	}

	foundCPUUtil := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "system.cpu.utilization" {
				foundCPUUtil = true
			}
		}
	}

	if !foundCPUUtil {
		t.Errorf("expected system.cpu.utilization metric to be present in collected metrics")
	}
}
