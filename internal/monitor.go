package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AmdSmiOutput represents the JSON structure from amd-smi metric -u --json
type AmdSmiOutput struct {
	GpuData []GpuData `json:"gpu_data"`
}

type GpuData struct {
	Gpu   int      `json:"gpu"`
	Usage GpuUsage `json:"usage"`
}

type GpuUsage struct {
	GfxActivity           MetricValue   `json:"gfx_activity"`
	ApuAverageIpuActivity []MetricValue `json:"apu_average_ipu_activity"`
}

type MetricValue struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

// cpuStats stores the values from /proc/stat
type cpuStats struct {
	idle  uint64
	total uint64
}

// memStats stores the values from /proc/meminfo
type memStats struct {
	total     uint64
	available uint64
}

// parseMemStats parses memory stats from /proc/meminfo bytes line-by-line
func parseMemStats(data []byte) (memStats, error) {
	var stats memStats
	rem := data
	for len(rem) > 0 {
		var line []byte
		idx := bytes.IndexByte(rem, '\n')
		if idx >= 0 {
			line = rem[:idx]
			rem = rem[idx+1:]
		} else {
			line = rem
			rem = nil
		}

		if bytes.HasPrefix(line, []byte("MemTotal:")) {
			fields := strings.Fields(string(line))
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					stats.total = val
				}
			}
		} else if bytes.HasPrefix(line, []byte("MemAvailable:")) {
			fields := strings.Fields(string(line))
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					stats.available = val
				}
			}
		}

		// Both values are near top of /proc/meminfo; terminate early once found
		if stats.total > 0 && stats.available > 0 {
			break
		}
	}

	if stats.total == 0 {
		return memStats{}, fmt.Errorf("could not read MemTotal from /proc/meminfo")
	}

	return stats, nil
}

// getMemStats reads /proc/meminfo and returns total and available memory in kB
func getMemStats() (memStats, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return memStats{}, err
	}
	return parseMemStats(data)
}

// GetRAMUsage calculates the RAM usage percentage
func GetRAMUsage() (uint16, error) {
	stats, err := getMemStats()
	if err != nil {
		return 0, err
	}

	if stats.total == 0 {
		return 0, nil
	}

	used := stats.total - stats.available
	usage := (used * 100) / stats.total

	return uint16(usage), nil
}

// parseCPUStats parses CPU idle and total ticks from /proc/stat bytes
func parseCPUStats(data []byte) (cpuStats, error) {
	rem := data
	for len(rem) > 0 {
		var line []byte
		idx := bytes.IndexByte(rem, '\n')
		if idx >= 0 {
			line = rem[:idx]
			rem = rem[idx+1:]
		} else {
			line = rem
			rem = nil
		}

		if bytes.HasPrefix(line, []byte("cpu ")) {
			fields := strings.Fields(string(line))
			if len(fields) < 5 {
				return cpuStats{}, fmt.Errorf("unexpected /proc/stat format")
			}

			var total uint64
			var idle uint64

			for i := 1; i < len(fields); i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					return cpuStats{}, err
				}
				total += val
				if i == 4 || i == 5 { // idle and iowait
					idle += val
				}
			}

			return cpuStats{idle: idle, total: total}, nil
		}
	}

	return cpuStats{}, fmt.Errorf("cpu stats not found in /proc/stat")
}

// getCPUStats reads /proc/stat and returns idle and total time
func getCPUStats() (cpuStats, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStats{}, err
	}
	return parseCPUStats(data)
}

// calculateCPUDelta computes the CPU utilization percentage between two samples
func calculateCPUDelta(s1, s2 cpuStats) uint16 {
	totalDiff := s2.total - s1.total
	if totalDiff == 0 {
		return 0
	}

	idleDiff := s2.idle - s1.idle
	if idleDiff > totalDiff {
		idleDiff = totalDiff
	}

	usage := (totalDiff - idleDiff) * 100 / totalDiff
	if usage > 100 {
		usage = 100
	}

	return uint16(usage)
}

// GetCPUUsage calculates the CPU usage percentage over a short interval (standalone/one-shot)
func GetCPUUsage() (uint16, error) {
	s1, err := getCPUStats()
	if err != nil {
		return 0, err
	}

	time.Sleep(100 * time.Millisecond)

	s2, err := getCPUStats()
	if err != nil {
		return 0, err
	}

	return calculateCPUDelta(s1, s2), nil
}

// getAmdSmiData reads and parses the amd-smi output
var getAmdSmiData = func(ctx context.Context) (*AmdSmiOutput, error) {
	cmd := exec.CommandContext(ctx, "/opt/rocm/bin/amd-smi", "metric", "-u", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running amd-smi: %w", err)
	}

	var smiOutput AmdSmiOutput
	if err := json.Unmarshal(output, &smiOutput); err != nil {
		return nil, fmt.Errorf("error parsing amd-smi output: %w", err)
	}

	return &smiOutput, nil
}

// GetGpuAndNpuUsage gets both GPU and NPU utilization using amd-smi
func GetGpuAndNpuUsage() (uint16, uint16, error) {
	// Create a context with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	smiOutput, err := getAmdSmiData(ctx)
	if err != nil {
		return 0, 0, err
	}

	if len(smiOutput.GpuData) == 0 {
		return 0, 0, fmt.Errorf("no GPU data found in amd-smi output")
	}

	// Get GPU usage from gfx_activity
	gpuUsage := uint16(smiOutput.GpuData[0].Usage.GfxActivity.Value)

	// Get NPU usage from apu_average_ipu_activity
	// Take the average of all IPU activities reported
	var sum uint16
	for _, activity := range smiOutput.GpuData[0].Usage.ApuAverageIpuActivity {
		sum += uint16(activity.Value)
	}
	var npuUsage uint16
	if len(smiOutput.GpuData[0].Usage.ApuAverageIpuActivity) > 0 {
		npuUsage = sum / uint16(len(smiOutput.GpuData[0].Usage.ApuAverageIpuActivity))
	}

	return gpuUsage, npuUsage, nil
}

// MonitorInterface defines the methods available for monitoring system resources
type MonitorInterface interface {
	GetCPUUsage() (uint16, error)
	GetGpuAndNpuUsage() (uint16, uint16, error)
	GetRAMUsage() (uint16, error)
}

// Monitor handles interacting with system resource monitoring
type Monitor struct {
	mu         sync.Mutex
	lastCPU    cpuStats
	hasLastCPU bool
}

// NewMonitor creates a new Monitor instance and captures baseline CPU stats
func NewMonitor() *Monitor {
	m := &Monitor{}
	if stats, err := getCPUStats(); err == nil {
		m.lastCPU = stats
		m.hasLastCPU = true
	}
	return m
}

// GetCPUUsage calculates the CPU usage percentage since the previous tick without sleeping
func (m *Monitor) GetCPUUsage() (uint16, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	curr, err := getCPUStats()
	if err != nil {
		return 0, err
	}

	if !m.hasLastCPU || curr.total <= m.lastCPU.total {
		time.Sleep(100 * time.Millisecond)
		next, err := getCPUStats()
		if err != nil {
			m.lastCPU = curr
			m.hasLastCPU = true
			return 0, err
		}
		usage := calculateCPUDelta(curr, next)
		m.lastCPU = next
		m.hasLastCPU = true
		return usage, nil
	}

	usage := calculateCPUDelta(m.lastCPU, curr)
	m.lastCPU = curr
	return usage, nil
}

// GetGpuAndNpuUsage gets the GPU and NPU utilization using amd-smi
func (m *Monitor) GetGpuAndNpuUsage() (uint16, uint16, error) {
	return GetGpuAndNpuUsage()
}

// GetRAMUsage calculates the RAM usage percentage
func (m *Monitor) GetRAMUsage() (uint16, error) {
	return GetRAMUsage()
}
