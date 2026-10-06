package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// AmdSmiOutput represents the JSON structure from amd-smi metric -u --json
type AmdSmiOutput struct {
	GpuData []struct {
		Usage struct {
			ApuAverageIpuActivity []struct {
				Value int    `json:"value"`
				Unit  string `json:"unit"`
			} `json:"apu_average_ipu_activity"`
		} `json:"usage"`
	} `json:"gpu_data"`
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

// getMemStats reads /proc/meminfo and returns total and available memory in kB
func getMemStats() (memStats, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return memStats{}, err
	}

	lines := strings.Split(string(data), "\n")
	var stats memStats
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					stats.total = val
				}
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					stats.available = val
				}
			}
		}
	}

	if stats.total == 0 {
		return memStats{}, fmt.Errorf("could not read MemTotal from /proc/meminfo")
	}

	return stats, nil
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

// getCPUStats reads /proc/stat and returns idle and total time
func getCPUStats() (cpuStats, error) {
	// these are large-ish int values
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStats{}, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
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

// GetCPUUsage calculates the CPU usage percentage over a short interval
func GetCPUUsage() (uint16, error) {
	s1, err := getCPUStats()
	if err != nil {
		return 0, err
	}

	// Use a goroutine to avoid blocking the main thread
	done := make(chan struct{})
	var s2 cpuStats
	var err2 error

	go func() {
		defer close(done)
		// Give the system time to gather new data
		time.Sleep(100 * time.Millisecond)
		s2, err2 = getCPUStats()
	}()

	// Wait for completion with timeout
	select {
	case <-done:
		if err2 != nil {
			return 0, err2
		}
	case <-time.After(150 * time.Millisecond): // Slightly longer timeout
		return 0, fmt.Errorf("timeout waiting for second CPU stats measurement")
	}

	totalDiff := s2.total - s1.total
	if totalDiff == 0 {
		return 0, nil
	}

	idleDiff := s2.idle - s1.idle
	usage := (totalDiff - idleDiff) * 100 / totalDiff

	return uint16(usage), nil
}

// GetGPUUsage reads the GPU utilization from sysfs
// TODO - refactor to get GPU and NPU together since both are returned by amd-smi
func GetGPUUsage() (uint16, error) {
	// percent is 0-100
	const gpuUsagePath = "/sys/class/drm/card0/device/gpu_busy_percent"
	data, err := os.ReadFile(gpuUsagePath)
	if err != nil {
		return 0, fmt.Errorf("error reading GPU usage: %w", err)
	}

	usage, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("error parsing GPU usage: %w", err)
	}

	return uint16(usage), nil
}

// GetNPUUsage gets the NPU utilization using amd-smi
func GetNPUUsage() (uint16, error) {
	// TODO - use this read to get the GPU and NPU loads
	// THis command: /opt/rocm/bin/amd-smi metric -u --json
	// Yields json as follows:
	/* "gpu_data":
	   {
	       "gpu": 0,
	       "usage": {
	           "gfx_activity": {
	               "value": 98,
	               "unit": "%"
	           },
	*/
	cmd := exec.Command("/opt/rocm/bin/amd-smi", "metric", "-u", "--json")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error running amd-smi: %w", err)
	}

	var smiOutput AmdSmiOutput
	if err := json.Unmarshal(output, &smiOutput); err != nil {
		return 0, fmt.Errorf("error parsing amd-smi output: %w", err)
	}

	if len(smiOutput.GpuData) == 0 || len(smiOutput.GpuData[0].Usage.ApuAverageIpuActivity) == 0 {
		return 0, fmt.Errorf("NPU usage data not found in amd-smi output")
	}

	// Take the average of all IPU activities reported
	var sum uint16
	for _, activity := range smiOutput.GpuData[0].Usage.ApuAverageIpuActivity {
		sum += uint16(activity.Value)
	}
	avgUsage := sum / uint16(len(smiOutput.GpuData[0].Usage.ApuAverageIpuActivity))

	return uint16(avgUsage), nil
}

// MonitorInterface defines the methods available for monitoring system resources
type MonitorInterface interface {
	GetCPUUsage() (uint16, error)
	GetGPUUsage() (uint16, error)
	GetNPUUsage() (uint16, error)
	GetRAMUsage() (uint16, error)
}

// Monitor handles interacting with system resource monitoring
type Monitor struct{}

// NewMonitor creates a new Monitor instance
func NewMonitor() *Monitor {
	return &Monitor{}
}

// GetCPUUsage calculates the CPU usage percentage
func (m *Monitor) GetCPUUsage() (uint16, error) {
	return GetCPUUsage()
}

// GetGPUUsage reads the GPU utilizat16n from sysfs
func (m *Monitor) GetGPUUsage() (uint16, error) {
	return GetGPUUsage()
}

// GetNPUUsage gets the NPU utilizati16 using amd-smi
func (m *Monitor) GetNPUUsage() (uint16, error) {
	return GetNPUUsage()
}

// GetRAMUsage calculates the RAM usa16 percentage
func (m *Monitor) GetRAMUsage() (uint16, error) {
	return GetRAMUsage()
}
