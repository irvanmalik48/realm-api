package telemetry

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CPUStats contains real-time CPU utilization, frequencies, and load averages.
type CPUStats struct {
	UsagePercent     float64   `json:"usage_percent"`
	CoreUsagePercent []float64 `json:"core_usage_percent"`
	AvgFrequencyMHz  float64   `json:"avg_frequency_mhz"`
	CoreFrequencyMHz []float64 `json:"core_frequency_mhz"`
	MinFrequencyMHz  float64   `json:"min_frequency_mhz"`
	MaxFrequencyMHz  float64   `json:"max_frequency_mhz"`
	Load1m           float64   `json:"load_1m"`
	Load5m           float64   `json:"load_5m"`
	Load15m          float64   `json:"load_15m"`
	ModelName        string    `json:"model_name"`
	CoreCount        int32     `json:"core_count"`
}

type cpuSample struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

func (s cpuSample) total() uint64 {
	return s.user + s.nice + s.system + s.idle + s.iowait + s.irq + s.softirq + s.steal
}

func (s cpuSample) idleTotal() uint64 {
	return s.idle + s.iowait
}

// CPUMonitor continuously tracks real-time CPU utilization and frequency.
type CPUMonitor struct {
	mu          sync.RWMutex
	stats       CPUStats
	prevTotal   cpuSample
	prevCores   []cpuSample
	modelName   string
	coreCount   int32
	minFreqMHz  float64
	maxFreqMHz  float64
	stopCh      chan struct{}
	stopped     bool
}

var (
	defaultMonitor *CPUMonitor
	monitorOnce    sync.Once
)

// DefaultCPUMonitor returns the singleton CPU monitor instance.
func DefaultCPUMonitor() *CPUMonitor {
	monitorOnce.Do(func() {
		defaultMonitor = NewCPUMonitor(1 * time.Second)
	})
	return defaultMonitor
}

// NewCPUMonitor creates and starts a new CPUMonitor.
func NewCPUMonitor(interval time.Duration) *CPUMonitor {
	if interval < 200*time.Millisecond {
		interval = 1 * time.Second
	}

	m := &CPUMonitor{
		coreCount: int32(runtime.NumCPU()),
		stopCh:    make(chan struct{}),
	}

	m.initStaticInfo()
	m.sample() // Initial sample to seed baseline

	go m.run(interval)
	return m
}

func (m *CPUMonitor) initStaticInfo() {
	// 1. Detect Model Name and Core Count from /proc/cpuinfo
	if file, err := os.Open("/proc/cpuinfo"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					m.modelName = strings.TrimSpace(parts[1])
					break
				}
			}
		}
		_ = file.Close()
	}

	if m.modelName == "" {
		m.modelName = fmt.Sprintf("Generic CPU (%s)", runtime.GOARCH)
	}

	// 2. Detect Min & Max Frequency from sysfs
	if minBytes, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq"); err == nil {
		if val, parseErr := strconv.ParseFloat(strings.TrimSpace(string(minBytes)), 64); parseErr == nil {
			m.minFreqMHz = val / 1000.0
		}
	}
	if maxBytes, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"); err == nil {
		if val, parseErr := strconv.ParseFloat(strings.TrimSpace(string(maxBytes)), 64); parseErr == nil {
			m.maxFreqMHz = val / 1000.0
		}
	}
}

func (m *CPUMonitor) run(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.sample()
		case <-m.stopCh:
			return
		}
	}
}

func (m *CPUMonitor) sample() {
	currTotal, currCores, err := readProcStat()
	if err != nil {
		// Fallback for non-procfs platforms
		m.mu.Lock()
		m.stats = CPUStats{
			ModelName: m.modelName,
			CoreCount: m.coreCount,
		}
		m.mu.Unlock()
		return
	}

	// Calculate overall CPU utilization
	var overallUsage float64
	if m.prevTotal.total() > 0 {
		deltaTotal := currTotal.total() - m.prevTotal.total()
		deltaIdle := currTotal.idleTotal() - m.prevTotal.idleTotal()
		if deltaTotal > 0 && deltaTotal >= deltaIdle {
			overallUsage = (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
		}
	}

	// Calculate per-core CPU utilization
	coreUsages := make([]float64, len(currCores))
	if len(m.prevCores) == len(currCores) {
		for i := range currCores {
			deltaTotal := currCores[i].total() - m.prevCores[i].total()
			deltaIdle := currCores[i].idleTotal() - m.prevCores[i].idleTotal()
			if deltaTotal > 0 && deltaTotal >= deltaIdle {
				usage := (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
				if usage < 0 {
					usage = 0
				} else if usage > 100 {
					usage = 100
				}
				coreUsages[i] = usage
			}
		}
	}

	m.prevTotal = currTotal
	m.prevCores = currCores

	// Read CPU Frequencies
	coreFreqs, avgFreq := readCPUFrequencies(len(currCores))

	// Read System Load Average
	load1, load5, load15 := readLoadAvg()

	m.mu.Lock()
	m.stats = CPUStats{
		UsagePercent:     overallUsage,
		CoreUsagePercent: coreUsages,
		AvgFrequencyMHz:  avgFreq,
		CoreFrequencyMHz: coreFreqs,
		MinFrequencyMHz:  m.minFreqMHz,
		MaxFrequencyMHz:  m.maxFreqMHz,
		Load1m:           load1,
		Load5m:           load5,
		Load15m:          load15,
		ModelName:        m.modelName,
		CoreCount:        int32(len(currCores)),
	}
	m.mu.Unlock()
}

// GetStats returns the latest measured CPU stats.
func (m *CPUMonitor) GetStats() CPUStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// Stop terminates the background sampling loop.
func (m *CPUMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.stopped {
		m.stopped = true
		close(m.stopCh)
	}
}

func readProcStat() (cpuSample, []cpuSample, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, nil, err
	}
	defer file.Close()

	var totalSample cpuSample
	var coreSamples []cpuSample

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		if fields[0] == "cpu" {
			totalSample = parseSample(fields[1:])
		} else if strings.HasPrefix(fields[0], "cpu") && len(fields[0]) > 3 {
			coreSamples = append(coreSamples, parseSample(fields[1:]))
		}
	}

	return totalSample, coreSamples, nil
}

func parseSample(fields []string) cpuSample {
	var s cpuSample
	vals := make([]uint64, 8)
	for i := 0; i < len(vals) && i < len(fields); i++ {
		vals[i], _ = strconv.ParseUint(fields[i], 10, 64)
	}
	s.user = vals[0]
	s.nice = vals[1]
	s.system = vals[2]
	s.idle = vals[3]
	s.iowait = vals[4]
	s.irq = vals[5]
	s.softirq = vals[6]
	s.steal = vals[7]
	return s
}

func readCPUFrequencies(coreCount int) ([]float64, float64) {
	if coreCount <= 0 {
		coreCount = runtime.NumCPU()
	}

	freqs := make([]float64, coreCount)
	var sum float64
	var counted int

	// 1. Try sysfs scaling_cur_freq
	hasSysfs := false
	for i := 0; i < coreCount; i++ {
		path := filepath.Join("/sys/devices/system/cpu", fmt.Sprintf("cpu%d", i), "cpufreq/scaling_cur_freq")
		if data, err := os.ReadFile(path); err == nil {
			if khz, parseErr := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); parseErr == nil {
				mhz := khz / 1000.0
				freqs[i] = mhz
				sum += mhz
				counted++
				hasSysfs = true
			}
		}
	}

	if hasSysfs && counted > 0 {
		return freqs, sum / float64(counted)
	}

	// 2. Fallback to /proc/cpuinfo "cpu MHz"
	if file, err := os.Open("/proc/cpuinfo"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		coreIdx := 0
		for scanner.Scan() && coreIdx < coreCount {
			line := scanner.Text()
			if strings.HasPrefix(line, "cpu MHz") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					if mhz, parseErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); parseErr == nil {
						freqs[coreIdx] = mhz
						sum += mhz
						counted++
						coreIdx++
					}
				}
			}
		}
	}

	if counted > 0 {
		return freqs, sum / float64(counted)
	}

	return freqs, 0
}

func readLoadAvg() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}

	var l1, l5, l15 float64
	_, _ = fmt.Sscanf(string(data), "%f %f %f", &l1, &l5, &l15)
	return l1, l5, l15
}
