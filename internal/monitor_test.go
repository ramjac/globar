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
	// Save original getAmdSmiData to restore it later
	originalGetAmdSmiData := getAmdSmiData
	defer func() { getAmdSmiData = originalGetAmdSmiData }()

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
			getAmdSmiData = func(ctx context.Context) (*AmdSmiOutput, error) {
				return tt.mockOutput, tt.mockErr
			}

			gpu, npu, err := GetGpuAndNpuUsage()
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
