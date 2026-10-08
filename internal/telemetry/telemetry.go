package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("realm-api")
	meter  = otel.Meter("realm-api")
)

// Tracer returns the global tracer for realm-api.
func Tracer() trace.Tracer {
	return tracer
}

// Meter returns the global meter for realm-api.
func Meter() metric.Meter {
	return meter
}

// InitTracer initializes OpenTelemetry TracerProvider, MeterProvider, and global propagators.
func InitTracer(ctx context.Context, serviceName, environment string) (func(context.Context) error, error) {
	return InitTelemetry(ctx, serviceName, environment)
}

// InitTelemetry initializes OpenTelemetry tracing and metrics subsystems.
func InitTelemetry(ctx context.Context, serviceName, environment string) (func(context.Context) error, error) {
	if serviceName == "" {
		serviceName = "realm-api"
	}
	if environment == "" {
		environment = "production"
	}

	res, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironmentNameKey.String(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	// 1. Trace Exporter & Provider
	var traceExporter sdktrace.SpanExporter
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint != "" {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(otlpEndpoint, "https://"), "http://")),
		}
		if !strings.HasPrefix(otlpEndpoint, "https://") {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		traceExporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlp trace exporter: %w", err)
		}
	} else if os.Getenv("OTEL_STDOUT_TRACING") == "true" {
		traceExporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	}

	var bsp sdktrace.TracerProviderOption
	if traceExporter != nil {
		bsp = sdktrace.WithBatcher(traceExporter)
	} else {
		bsp = sdktrace.WithSampler(sdktrace.AlwaysSample())
	}

	tp := sdktrace.NewTracerProvider(
		bsp,
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	tracer = otel.Tracer("realm-api")

	// 2. Metric Exporter & Provider
	var metricReader sdkmetric.Reader
	if otlpEndpoint != "" {
		mOpts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(otlpEndpoint, "https://"), "http://")),
		}
		if !strings.HasPrefix(otlpEndpoint, "https://") {
			mOpts = append(mOpts, otlpmetrichttp.WithInsecure())
		}
		metricExporter, expErr := otlpmetrichttp.New(ctx, mOpts...)
		if expErr != nil {
			return nil, fmt.Errorf("failed to create otlp metric exporter: %w", expErr)
		}
		metricReader = sdkmetric.NewPeriodicReader(metricExporter)
	} else if os.Getenv("OTEL_STDOUT_METRICS") == "true" {
		metricExporter, expErr := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if expErr != nil {
			return nil, fmt.Errorf("failed to create stdout metric exporter: %w", expErr)
		}
		metricReader = sdkmetric.NewPeriodicReader(metricExporter)
	}

	var meterProviderOpts []sdkmetric.Option
	meterProviderOpts = append(meterProviderOpts, sdkmetric.WithResource(res))
	if metricReader != nil {
		meterProviderOpts = append(meterProviderOpts, sdkmetric.WithReader(metricReader))
	}

	mp := sdkmetric.NewMeterProvider(meterProviderOpts...)
	otel.SetMeterProvider(mp)
	meter = otel.Meter("realm-api")

	// 3. Register CPU & Runtime System Metrics
	_ = RegisterSystemMetrics(meter, DefaultCPUMonitor())

	shutdown := func(shutdownCtx context.Context) error {
		var firstErr error
		if err := tp.Shutdown(shutdownCtx); err != nil {
			firstErr = err
		}
		if err := mp.Shutdown(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
		return firstErr
	}

	return shutdown, nil
}

// RegisterSystemMetrics registers real-time CPU, memory, and Go runtime observable gauges.
func RegisterSystemMetrics(m metric.Meter, cpuMon *CPUMonitor) error {
	if cpuMon == nil {
		cpuMon = DefaultCPUMonitor()
	}

	cpuUtil, err := m.Float64ObservableGauge(
		"system.cpu.utilization",
		metric.WithDescription("Overall CPU utilization percentage"),
		metric.WithUnit("%"),
	)
	if err != nil {
		return err
	}

	cpuCoreUtil, err := m.Float64ObservableGauge(
		"system.cpu.core_utilization",
		metric.WithDescription("Per-core CPU utilization percentage"),
		metric.WithUnit("%"),
	)
	if err != nil {
		return err
	}

	cpuFreq, err := m.Float64ObservableGauge(
		"system.cpu.frequency",
		metric.WithDescription("Average CPU clock frequency in MHz"),
		metric.WithUnit("MHz"),
	)
	if err != nil {
		return err
	}

	cpuCoreFreq, err := m.Float64ObservableGauge(
		"system.cpu.core_frequency",
		metric.WithDescription("Per-core CPU clock frequency in MHz"),
		metric.WithUnit("MHz"),
	)
	if err != nil {
		return err
	}

	cpuMinFreq, err := m.Float64ObservableGauge(
		"system.cpu.frequency.min",
		metric.WithDescription("Minimum CPU clock frequency in MHz"),
		metric.WithUnit("MHz"),
	)
	if err != nil {
		return err
	}

	cpuMaxFreq, err := m.Float64ObservableGauge(
		"system.cpu.frequency.max",
		metric.WithDescription("Maximum CPU clock frequency in MHz"),
		metric.WithUnit("MHz"),
	)
	if err != nil {
		return err
	}

	load1m, err := m.Float64ObservableGauge(
		"system.cpu.load_average.1m",
		metric.WithDescription("System load average over 1 minute"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	load5m, err := m.Float64ObservableGauge(
		"system.cpu.load_average.5m",
		metric.WithDescription("System load average over 5 minutes"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	load15m, err := m.Float64ObservableGauge(
		"system.cpu.load_average.15m",
		metric.WithDescription("System load average over 15 minutes"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	cpuCount, err := m.Int64ObservableGauge(
		"system.cpu.count",
		metric.WithDescription("Total logical CPU cores"),
		metric.WithUnit("{cpu}"),
	)
	if err != nil {
		return err
	}

	goroutines, err := m.Int64ObservableGauge(
		"process.runtime.go.goroutines",
		metric.WithDescription("Number of goroutines currently executing"),
		metric.WithUnit("{goroutine}"),
	)
	if err != nil {
		return err
	}

	memAlloc, err := m.Int64ObservableGauge(
		"process.runtime.go.mem.alloc_bytes",
		metric.WithDescription("Allocated heap memory in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	memSys, err := m.Int64ObservableGauge(
		"process.runtime.go.mem.sys_bytes",
		metric.WithDescription("Total system memory obtained in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	gcCycles, err := m.Int64ObservableGauge(
		"process.runtime.go.gc.count",
		metric.WithDescription("Completed garbage collection cycles"),
		metric.WithUnit("{cycle}"),
	)
	if err != nil {
		return err
	}

	_, err = m.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			stats := cpuMon.GetStats()

			o.ObserveFloat64(cpuUtil, stats.UsagePercent)
			o.ObserveFloat64(cpuFreq, stats.AvgFrequencyMHz)
			o.ObserveFloat64(cpuMinFreq, stats.MinFrequencyMHz)
			o.ObserveFloat64(cpuMaxFreq, stats.MaxFrequencyMHz)
			o.ObserveFloat64(load1m, stats.Load1m)
			o.ObserveFloat64(load5m, stats.Load5m)
			o.ObserveFloat64(load15m, stats.Load15m)
			o.ObserveInt64(cpuCount, int64(stats.CoreCount))

			for i, usage := range stats.CoreUsagePercent {
				o.ObserveFloat64(cpuCoreUtil, usage, metric.WithAttributes(
					attribute.String("cpu", strconv.Itoa(i)),
				))
			}

			for i, freq := range stats.CoreFrequencyMHz {
				o.ObserveFloat64(cpuCoreFreq, freq, metric.WithAttributes(
					attribute.String("cpu", strconv.Itoa(i)),
				))
			}

			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			o.ObserveInt64(goroutines, int64(runtime.NumGoroutine()))
			o.ObserveInt64(memAlloc, int64(mem.Alloc))
			o.ObserveInt64(memSys, int64(mem.Sys))
			o.ObserveInt64(gcCycles, int64(mem.NumGC))

			return nil
		},
		cpuUtil,
		cpuCoreUtil,
		cpuFreq,
		cpuCoreFreq,
		cpuMinFreq,
		cpuMaxFreq,
		load1m,
		load5m,
		load15m,
		cpuCount,
		goroutines,
		memAlloc,
		memSys,
		gcCycles,
	)

	return err
}

// InitProfiler starts continuous in-production profiling via Pyroscope if configured.
func InitProfiler(serviceName, environment string) (*pyroscope.Profiler, error) {
	serverAddress := os.Getenv("PYROSCOPE_SERVER_ADDRESS")
	if serverAddress == "" {
		return nil, nil
	}

	if serviceName == "" {
		serviceName = "realm-api"
	}

	return pyroscope.Start(pyroscope.Config{
		ApplicationName: serviceName,
		ServerAddress:   serverAddress,
		AuthToken:       os.Getenv("PYROSCOPE_AUTH_TOKEN"),
		Tags: map[string]string{
			"env": environment,
		},
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
}
