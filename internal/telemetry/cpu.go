package telemetry

import (
	"bufio"
	"fmt"
	"math"
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
	mu             sync.RWMutex
	stats          CPUStats
	prevTotal      cpuSample
	prevCores      []cpuSample
	prevCgroupUsec uint64
	prevCgroupTime time.Time
	modelName      string
	coreCount      int32
	baseFreqMHz    float64
	minFreqMHz     float64
	maxFreqMHz     float64
	stopCh         chan struct{}
	stopped        bool
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
	// Quick delta sample so real utilization is available immediately on startup
	time.Sleep(50 * time.Millisecond)
	m.sample()

	go m.run(interval)
	return m
}

func (m *CPUMonitor) initStaticInfo() {
	// 1. Detect Model Name and Core Count from /proc/cpuinfo
	if file, err := os.Open("/proc/cpuinfo"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "model name") || strings.HasPrefix(lower, "hardware") || strings.HasPrefix(lower, "processor") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					if _, parseErr := strconv.Atoi(val); parseErr != nil && val != "" {
						m.modelName = val
						break
					}
				}
			}
		}
		_ = file.Close()
	}

	if m.modelName == "" {
		m.modelName = fmt.Sprintf("Generic CPU (%s)", runtime.GOARCH)
	}

	// Extract base frequency from model name if available (e.g. "@ 2.40GHz")
	m.baseFreqMHz = extractFreqFromModel(m.modelName)

	// 2. Detect Min & Max Frequency from sysfs cpufreq paths
	minPaths := []string{
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_min_freq",
		"/sys/devices/system/cpu/cpufreq/policy0/cpuinfo_min_freq",
		"/sys/devices/system/cpu/cpufreq/policy0/scaling_min_freq",
	}
	for _, p := range minPaths {
		if minBytes, err := os.ReadFile(p); err == nil {
			if val, parseErr := strconv.ParseFloat(strings.TrimSpace(string(minBytes)), 64); parseErr == nil && val > 0 {
				m.minFreqMHz = val / 1000.0
				break
			}
		}
	}

	maxPaths := []string{
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq",
		"/sys/devices/system/cpu/cpufreq/policy0/cpuinfo_max_freq",
		"/sys/devices/system/cpu/cpufreq/policy0/scaling_max_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/bios_limit",
	}
	for _, p := range maxPaths {
		if maxBytes, err := os.ReadFile(p); err == nil {
			if val, parseErr := strconv.ParseFloat(strings.TrimSpace(string(maxBytes)), 64); parseErr == nil && val > 0 {
				m.maxFreqMHz = val / 1000.0
				break
			}
		}
	}

	if m.minFreqMHz <= 0 && m.baseFreqMHz > 0 {
		m.minFreqMHz = m.baseFreqMHz * 0.5
	}
	if m.maxFreqMHz <= 0 && m.baseFreqMHz > 0 {
		m.maxFreqMHz = m.baseFreqMHz * 1.5
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
	currTotal, currCores, procErr := readProcStat()
	coreCount := int(m.coreCount)
	if len(currCores) > 0 {
		coreCount = len(currCores)
	}

	var overallUsage float64
	var coreUsages []float64

	if procErr == nil {
		if m.prevTotal.total() > 0 {
			deltaTotal := currTotal.total() - m.prevTotal.total()
			deltaIdle := currTotal.idleTotal() - m.prevTotal.idleTotal()
			if deltaTotal > 0 && deltaTotal >= deltaIdle {
				overallUsage = (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
			}
		}

		coreUsages = make([]float64, len(currCores))
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
	} else {
		// Fallback for non-procfs / container environments: try cgroup v2
		if cgroupUsage, cErr := m.readCgroupCPUUsage(); cErr == nil && cgroupUsage > 0 {
			overallUsage = cgroupUsage
		}
	}

	// Always read CPU frequencies
	coreFreqs, avgFreq := readCPUFrequencies(coreCount)

	// If dynamic frequencies not found, fallback to static bounds or model clock
	if avgFreq <= 0 {
		if m.maxFreqMHz > 0 {
			avgFreq = m.maxFreqMHz
		} else if m.baseFreqMHz > 0 {
			avgFreq = m.baseFreqMHz
		} else if m.minFreqMHz > 0 {
			avgFreq = m.minFreqMHz
		}
		if avgFreq > 0 {
			if len(coreFreqs) == 0 {
				coreFreqs = make([]float64, coreCount)
			}
			for i := range coreFreqs {
				coreFreqs[i] = avgFreq
			}
		}
	}

	// Always read System Load Average
	load1, load5, load15 := readLoadAvg()

	// If overallUsage is 0 and load1 > 0, estimate utilization if procstat was unavailable
	if overallUsage == 0 && procErr != nil && load1 > 0 && coreCount > 0 {
		overallUsage = math.Min(100.0, (load1/float64(coreCount))*100.0)
	}

	minFreq := m.minFreqMHz
	maxFreq := m.maxFreqMHz
	if (minFreq <= 0 || maxFreq <= 0) && len(coreFreqs) > 0 {
		var minVal, maxVal float64
		for _, f := range coreFreqs {
			if f > 0 {
				if minVal == 0 || f < minVal {
					minVal = f
				}
				if f > maxVal {
					maxVal = f
				}
			}
		}
		if minFreq <= 0 {
			minFreq = minVal
		}
		if maxFreq <= 0 {
			maxFreq = maxVal
		}
	}

	m.mu.Lock()
	m.stats = CPUStats{
		UsagePercent:     overallUsage,
		CoreUsagePercent: coreUsages,
		AvgFrequencyMHz:  avgFreq,
		CoreFrequencyMHz: coreFreqs,
		MinFrequencyMHz:  minFreq,
		MaxFrequencyMHz:  maxFreq,
		Load1m:           load1,
		Load5m:           load5,
		Load15m:          load15,
		ModelName:        m.modelName,
		CoreCount:        int32(coreCount),
	}
	m.mu.Unlock()
}

// readCgroupCPUUsage measures container CPU utilization from cgroup v2 cpu.stat.
func (m *CPUMonitor) readCgroupCPUUsage() (float64, error) {
	data, err := os.ReadFile("/sys/fs/cgroup/cpu.stat")
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var usec uint64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "usage_usec") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				usec, _ = strconv.ParseUint(fields[1], 10, 64)
				break
			}
		}
	}
	if usec == 0 {
		return 0, fmt.Errorf("usage_usec not found")
	}

	now := time.Now()
	if m.prevCgroupUsec > 0 && !m.prevCgroupTime.IsZero() {
		deltaUsec := usec - m.prevCgroupUsec
		deltaDuration := now.Sub(m.prevCgroupTime).Microseconds()
		if deltaDuration > 0 && deltaUsec > 0 {
			numCPU := float64(runtime.NumCPU())
			usage := (float64(deltaUsec) / (float64(deltaDuration) * numCPU)) * 100.0
			if usage > 100.0 {
				usage = 100.0
			}
			m.prevCgroupUsec = usec
			m.prevCgroupTime = now
			return usage, nil
		}
	}
	m.prevCgroupUsec = usec
	m.prevCgroupTime = now
	return 0, nil
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

	// 1. Try sysfs per-cpu or policy directories
	for i := 0; i < coreCount; i++ {
		paths := []string{
			filepath.Join("/sys/devices/system/cpu", fmt.Sprintf("cpu%d", i), "cpufreq/scaling_cur_freq"),
			filepath.Join("/sys/devices/system/cpu", fmt.Sprintf("cpu%d", i), "cpufreq/cpuinfo_cur_freq"),
			filepath.Join("/sys/devices/system/cpu/cpufreq", fmt.Sprintf("policy%d", i), "scaling_cur_freq"),
			filepath.Join("/sys/devices/system/cpu/cpufreq", fmt.Sprintf("policy%d", i), "cpuinfo_cur_freq"),
		}
		for _, p := range paths {
			if data, err := os.ReadFile(p); err == nil {
				if khz, parseErr := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); parseErr == nil && khz > 0 {
					mhz := khz / 1000.0
					freqs[i] = mhz
					sum += mhz
					counted++
					break
				}
			}
		}
	}

	// 2. Fallback to /proc/cpuinfo if sysfs didn't find all cores
	if counted < coreCount {
		if file, err := os.Open("/proc/cpuinfo"); err == nil {
			scanner := bufio.NewScanner(file)
			coreIdx := 0
			for scanner.Scan() && coreIdx < coreCount {
				line := scanner.Text()
				lower := strings.ToLower(line)
				if strings.Contains(lower, "cpu mhz") || strings.Contains(lower, "clock") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						valStr := strings.TrimSpace(parts[1])
						valStr = strings.TrimSuffix(valStr, "MHz")
						valStr = strings.TrimSuffix(valStr, "mhz")
						valStr = strings.TrimSpace(valStr)
						if mhz, parseErr := strconv.ParseFloat(valStr, 64); parseErr == nil && mhz > 0 {
							if freqs[coreIdx] <= 0 {
								freqs[coreIdx] = mhz
								sum += mhz
								counted++
							}
							coreIdx++
						}
					}
				}
			}
			_ = file.Close()
		}
	}

	if counted > 0 {
		avg := sum / float64(counted)
		// Propagate average frequency to any cores that lacked direct sensors
		for i := range freqs {
			if freqs[i] <= 0 {
				freqs[i] = avg
			}
		}
		return freqs, avg
	}

	return freqs, 0
}

func extractFreqFromModel(model string) float64 {
	lower := strings.ToLower(model)
	if idx := strings.Index(lower, "@"); idx != -1 {
		part := strings.TrimSpace(lower[idx+1:])
		fields := strings.Fields(part)
		if len(fields) > 0 {
			target := fields[0]
			if strings.HasSuffix(target, "ghz") {
				numStr := strings.TrimSuffix(target, "ghz")
				if val, err := strconv.ParseFloat(numStr, 64); err == nil && val > 0 {
					return val * 1000.0
				}
			} else if strings.HasSuffix(target, "mhz") {
				numStr := strings.TrimSuffix(target, "mhz")
				if val, err := strconv.ParseFloat(numStr, 64); err == nil && val > 0 {
					return val
				}
			}
		}
	}
	return 0
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
