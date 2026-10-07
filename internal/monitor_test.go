package internal

import (
	"context"
	"fmt"
	"testing"
)

func TestGetRAMUsage(t *testing.T) {
	// This test might fail in environments without /proc/meminfo
	usage, err := GetRAMUsage()
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	if usage > 100 {
		t.Errorf("Expected usage <= 100, got %d", usage)
	}
}

func TestGetCPUUsage(t *testing.T) {
	// This test might fail in environments without /proc/stat
	usage, err := GetCPUUsage()
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	if usage > 100 {
		t.Errorf("Expected usage <= 100, got %d", usage)
	}
}

func TestGetGpuAndNpuUsage(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput *AmdSmiOutput
		mockErr    error
		wantGpu    uint16
		wantNpu    uint16
		wantErr    bool
	}{
		{
			name: "success",
			mockOutput: &AmdSmiOutput{
				GpuData: []GpuData{
					{
						Gpu: 0,
						Usage: GpuUsage{
							GfxActivity: MetricValue{Value: 50, Unit: "%"},
							ApuAverageIpuActivity: []MetricValue{
								{Value: 30, Unit: "%"},
								{Value: 40, Unit: "%"},
							},
						},
					},
				},
			},
			mockErr: nil,
			wantGpu: 50,
			wantNpu: 35, // (30 + 40) / 2
			wantErr: false,
		},
		{
			name:       "getAmdSmiData error",
			mockOutput: nil,
			mockErr:    fmt.Errorf("failed to run amd-smi"),
			wantGpu:    0,
			wantNpu:    0,
			wantErr:    true,
		},
		{
			name: "no GPU data",
			mockOutput: &AmdSmiOutput{
				GpuData: nil,
			},
			mockErr: nil,
			wantGpu: 0,
			wantNpu: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMonitorWithRunner(func(ctx context.Context) (*AmdSmiOutput, error) {
				return tt.mockOutput, tt.mockErr
			})

			gpu, npu, err := m.GetGpuAndNpuUsage()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetGpuAndNpuUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gpu != tt.wantGpu {
				t.Errorf("GetGpuAndNpuUsage() gpu = %v, want %v", gpu, tt.wantGpu)
			}
			if npu != tt.wantNpu {
				t.Errorf("GetGpuAndNpuUsage() npu = %v, want %v", npu, tt.wantNpu)
			}
		})
	}
}

func TestParseAmdSmiOutput_EdgeCases(t *testing.T) {
	// Nil output
	if _, _, err := parseAmdSmiOutput(nil); err == nil {
		t.Error("Expected error for nil output, got nil")
	}

	// Empty GPU data
	if _, _, err := parseAmdSmiOutput(&AmdSmiOutput{GpuData: nil}); err == nil {
		t.Error("Expected error for empty GpuData, got nil")
	}

	// Values out of 0-100 range clamped, empty IPU activity defaults to 0
	out := &AmdSmiOutput{
		GpuData: []GpuData{
			{
				Gpu: 0,
				Usage: GpuUsage{
					GfxActivity:           MetricValue{Value: 150, Unit: "%"},
					ApuAverageIpuActivity: nil,
				},
			},
		},
	}
	gpu, npu, err := parseAmdSmiOutput(out)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gpu != 100 {
		t.Errorf("Expected clamped GPU usage 100, got %d", gpu)
	}
	if npu != 0 {
		t.Errorf("Expected NPU usage 0 for empty activity, got %d", npu)
	}
}

func TestParseMemStats(t *testing.T) {
	sampleMeminfo := []byte(`MemTotal:       16000000 kB
MemFree:         4000000 kB
MemAvailable:    8000000 kB
Buffers:          500000 kB
Cached:          3500000 kB
`)

	stats, err := parseMemStats(sampleMeminfo)
	if err != nil {
		t.Fatalf("parseMemStats failed: %v", err)
	}

	if stats.total != 16000000 {
		t.Errorf("Expected total 16000000, got %d", stats.total)
	}
	if stats.available != 8000000 {
		t.Errorf("Expected available 8000000, got %d", stats.available)
	}

	// Test missing MemTotal
	_, err = parseMemStats([]byte("MemFree: 1000 kB\n"))
	if err == nil {
		t.Error("Expected error for missing MemTotal, got nil")
	}
}

func TestParseCPUStats(t *testing.T) {
	// cpu user nice system idle iowait irq softirq steal guest guest_nice
	sampleStat := []byte(`cpu  100 20 80 800 0 0 0 0 0 0
cpu0 50 10 40 400 0 0 0 0 0 0
intr 123456
`)

	stats, err := parseCPUStats(sampleStat)
	if err != nil {
		t.Fatalf("parseCPUStats failed: %v", err)
	}

	// total = 100 + 20 + 80 + 800 = 1000
	// idle = 800 (field 4) + 0 (field 5) = 800
	if stats.total != 1000 {
		t.Errorf("Expected total 1000, got %d", stats.total)
	}
	if stats.idle != 800 {
		t.Errorf("Expected idle 800, got %d", stats.idle)
	}

	// Test missing cpu line
	_, err = parseCPUStats([]byte("cpu0 50 10 40 400 0 0 0 0 0 0\n"))
	if err == nil {
		t.Error("Expected error for missing aggregate cpu line, got nil")
	}

	// Test malformed line
	_, err = parseCPUStats([]byte("cpu 1 2\n"))
	if err == nil {
		t.Error("Expected error for malformed cpu line, got nil")
	}
}

func TestCalculateCPUDelta(t *testing.T) {
	s1 := cpuStats{idle: 800, total: 1000}
	s2 := cpuStats{idle: 850, total: 1100}

	// totalDiff = 100, idleDiff = 50 -> usage = (100 - 50) * 100 / 100 = 50%
	usage := calculateCPUDelta(s1, s2)
	if usage != 50 {
		t.Errorf("Expected usage 50%%, got %d%%", usage)
	}

	// totalDiff = 0
	if usageZero := calculateCPUDelta(s1, s1); usageZero != 0 {
		t.Errorf("Expected 0%% for identical stats, got %d%%", usageZero)
	}
}

func TestMonitor_Methods(t *testing.T) {
	m := NewMonitor()
	if !m.hasLastCPU {
		t.Log("Note: /proc/stat may not be available in this environment")
	}

	// Test Monitor.GetCPUUsage
	usage, err := m.GetCPUUsage()
	if err == nil {
		if usage > 100 {
			t.Errorf("Expected CPU usage <= 100, got %d", usage)
		}
	}

	// Test Monitor.GetRAMUsage
	ramUsage, err := m.GetRAMUsage()
	if err == nil {
		if ramUsage > 100 {
			t.Errorf("Expected RAM usage <= 100, got %d", ramUsage)
		}
	}
}

func TestFindAmdSmiPath(t *testing.T) {
	// Test AMD_SMI_PATH environment variable override
	t.Setenv("AMD_SMI_PATH", "/custom/path/to/amd-smi")
	if path := findAmdSmiPath(); path != "/custom/path/to/amd-smi" {
		t.Errorf("Expected /custom/path/to/amd-smi, got %s", path)
	}

	// Test default / fallback when AMD_SMI_PATH is empty
	t.Setenv("AMD_SMI_PATH", "")
	path := findAmdSmiPath()
	if path == "" {
		t.Error("Expected non-empty path from findAmdSmiPath()")
	}
}

func TestMonitor_SetAmdSmiRunner(t *testing.T) {
	m := NewMonitor()
	called := false
	m.SetAmdSmiRunner(func(ctx context.Context) (*AmdSmiOutput, error) {
		called = true
		return &AmdSmiOutput{
			GpuData: []GpuData{
				{
					Gpu: 0,
					Usage: GpuUsage{
						GfxActivity: MetricValue{Value: 42, Unit: "%"},
					},
				},
			},
		}, nil
	})

	gpu, npu, err := m.GetGpuAndNpuUsage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected custom runner to be invoked")
	}
	if gpu != 42 || npu != 0 {
		t.Errorf("got gpu=%d npu=%d, want gpu=42 npu=0", gpu, npu)
	}
}
