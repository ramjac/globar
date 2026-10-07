package internal

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockLightbar is a mock implementation of LightbarInterface
type MockLightbar struct {
	GetStatusFn     func() (LightbarStatus, error)
	SetBrightnessFn func(brightness uint8) error
	SetRGBFn        func(r, g, b uint8) error
}

func (m *MockLightbar) GetStatus() (LightbarStatus, error) {
	return m.GetStatusFn()
}

func (m *MockLightbar) SetBrightness(brightness uint8) error {
	return m.SetBrightnessFn(brightness)
}

func (m *MockLightbar) SetRGB(r, g, b uint8) error {
	return m.SetRGBFn(r, g, b)
}

// MockMonitor is a mock implementation of MonitorInterface
type MockMonitor struct {
	GetCPUUsageFn       func() (uint16, error)
	GetGpuAndNpuUsageFn func() (uint16, uint16, error)
	GetRAMUsageFn       func() (uint16, error)
}

func (m *MockMonitor) GetCPUUsage() (uint16, error) {
	return m.GetCPUUsageFn()
}

func (m *MockMonitor) GetGpuAndNpuUsage() (uint16, uint16, error) {
	return m.GetGpuAndNpuUsageFn()
}

func (m *MockMonitor) GetRAMUsage() (uint16, error) {
	return m.GetRAMUsageFn()
}

func TestService_UpdateLightbar_Success(t *testing.T) {
	var gotBrightness uint8
	var gotR, gotG, gotB uint8

	mockLightbar := &MockLightbar{
		GetStatusFn: func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error {
			gotBrightness = brightness
			return nil
		},
		SetRGBFn: func(r, g, b uint8) error {
			gotR, gotG, gotB = r, g, b
			return nil
		},
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn:       func() (uint16, error) { return 20, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return 40, 50, nil },
		GetRAMUsageFn:       func() (uint16, error) { return 50, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)
	err := service.updateLightbar()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// gpu=40 -> red=40, cpu=20 -> green=20, npu=50 -> blue=50 (sum=110 >= 45)
	if gotR != 40 || gotG != 20 || gotB != 50 {
		t.Errorf("Expected RGB (40, 20, 50), got (%d, %d, %d)", gotR, gotG, gotB)
	}
	// avgUsage = (20+40+50)/3 = 36; load = (50+36)/2 = 43; brightness = 20 + (43*80)/100 = 20 + 34 = 54
	if gotBrightness != 54 {
		t.Errorf("Expected brightness 54, got %d", gotBrightness)
	}
}

func TestService_UpdateLightbar_BaselineGlow(t *testing.T) {
	var gotBrightness uint8
	var gotR, gotG, gotB uint8

	mockLightbar := &MockLightbar{
		GetStatusFn: func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error {
			gotBrightness = brightness
			return nil
		},
		SetRGBFn: func(r, g, b uint8) error {
			gotR, gotG, gotB = r, g, b
			return nil
		},
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn:       func() (uint16, error) { return 0, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return 0, 0, nil },
		GetRAMUsageFn:       func() (uint16, error) { return 0, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)
	err := service.updateLightbar()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// sum = 0 < 30, each channel gets +10
	if gotR != 10 || gotG != 10 || gotB != 10 {
		t.Errorf("Expected baseline glow RGB (10, 10, 10), got (%d, %d, %d)", gotR, gotG, gotB)
	}
	// avgUsage = 0; brightness = 0/2 + 20 = 20
	if gotBrightness != 20 {
		t.Errorf("Expected baseline brightness 20, got %d", gotBrightness)
	}
}

func TestService_UpdateLightbar_Clamping(t *testing.T) {
	var gotBrightness uint8
	var gotR, gotG, gotB uint8

	mockLightbar := &MockLightbar{
		GetStatusFn: func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error {
			gotBrightness = brightness
			return nil
		},
		SetRGBFn: func(r, g, b uint8) error {
			gotR, gotG, gotB = r, g, b
			return nil
		},
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn:       func() (uint16, error) { return 120, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return 150, 200, nil },
		GetRAMUsageFn:       func() (uint16, error) { return 110, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)
	err := service.updateLightbar()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Metrics > 100 must be clamped to 100
	if gotR != 100 || gotG != 100 || gotB != 100 {
		t.Errorf("Expected clamped RGB (100, 100, 100), got (%d, %d, %d)", gotR, gotG, gotB)
	}
	if gotBrightness != 100 {
		t.Errorf("Expected clamped brightness 100, got %d", gotBrightness)
	}
}

func TestService_UpdateLightbar_BrightnessScaling(t *testing.T) {
	testCases := []struct {
		name           string
		cpu            uint16
		gpu            uint16
		npu            uint16
		ram            uint16
		wantBrightness uint8
	}{
		{"0% load -> baseline 20", 0, 0, 0, 0, 20},
		{"25% load -> 40", 25, 25, 25, 25, 40},
		{"50% load -> 60", 50, 50, 50, 50, 60},
		{"75% load -> 80", 75, 75, 75, 75, 80},
		{"100% load -> 100", 100, 100, 100, 100, 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBrightness uint8
			mockLightbar := &MockLightbar{
				GetStatusFn: func() (LightbarStatus, error) { return LightbarStatus{}, nil },
				SetBrightnessFn: func(brightness uint8) error {
					gotBrightness = brightness
					return nil
				},
				SetRGBFn: func(r, g, b uint8) error { return nil },
			}
			mockMonitor := &MockMonitor{
				GetCPUUsageFn:       func() (uint16, error) { return tc.cpu, nil },
				GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return tc.gpu, tc.npu, nil },
				GetRAMUsageFn:       func() (uint16, error) { return tc.ram, nil },
			}

			svc := NewService(mockLightbar, mockMonitor, false)
			if err := svc.updateLightbar(); err != nil {
				t.Fatalf("updateLightbar failed: %v", err)
			}
			if gotBrightness != tc.wantBrightness {
				t.Errorf("For %s, expected brightness %d, got %d", tc.name, tc.wantBrightness, gotBrightness)
			}
		})
	}
}

func TestService_UpdateLightbar_MonitorError(t *testing.T) {
	mockLightbar := &MockLightbar{
		GetStatusFn:     func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error { return nil },
		SetRGBFn:        func(r, g, b uint8) error { return nil },
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn: func() (uint16, error) { return 0, fmt.Errorf("cpu error") },
	}

	service := NewService(mockLightbar, mockMonitor, true)
	err := service.updateLightbar()
	if err == nil {
		t.Error("Expected error from monitor, got nil")
	}
}

func TestService_UpdateLightbar_LightbarError(t *testing.T) {
	mockLightbar := &MockLightbar{
		GetStatusFn:     func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error { return nil },
		SetRGBFn:        func(r, g, b uint8) error { return fmt.Errorf("lightbar error") },
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn:       func() (uint16, error) { return 20, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return 40, 60, nil },
		GetRAMUsageFn:       func() (uint16, error) { return 50, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)
	err := service.updateLightbar()
	if err == nil {
		t.Error("Expected error from lightbar, got nil")
	}
}

func TestService_UpdateLightbar_ConditionalWrites(t *testing.T) {
	rgbWriteCount := 0
	brightnessWriteCount := 0

	mockLightbar := &MockLightbar{
		GetStatusFn: func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error {
			brightnessWriteCount++
			return nil
		},
		SetRGBFn: func(r, g, b uint8) error {
			rgbWriteCount++
			return nil
		},
	}

	cpuVal := uint16(20)
	gpuVal := uint16(40)
	npuVal := uint16(50)
	ramVal := uint16(50)

	mockMonitor := &MockMonitor{
		GetCPUUsageFn:       func() (uint16, error) { return cpuVal, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return gpuVal, npuVal, nil },
		GetRAMUsageFn:       func() (uint16, error) { return ramVal, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)

	// First update: should write to both
	if err := service.updateLightbar(); err != nil {
		t.Fatalf("First update failed: %v", err)
	}
	if rgbWriteCount != 1 || brightnessWriteCount != 1 {
		t.Fatalf("Expected 1 RGB write and 1 brightness write, got rgb=%d brightness=%d", rgbWriteCount, brightnessWriteCount)
	}

	// Second update with identical metrics: should NOT write to sysfs
	if err := service.updateLightbar(); err != nil {
		t.Fatalf("Second update failed: %v", err)
	}
	if rgbWriteCount != 1 || brightnessWriteCount != 1 {
		t.Errorf("Expected writes to be skipped on unchanged values, got rgb=%d brightness=%d", rgbWriteCount, brightnessWriteCount)
	}

	// Third update: only GPU changes -> should write RGB, but skip brightness if brightness didn't change
	// avgUsage = (20 + 41 + 50)/3 = 37 -> brightness = (50+37)/2 + 20 = 43 + 20 = 63 (unchanged!)
	gpuVal = 41
	if err := service.updateLightbar(); err != nil {
		t.Fatalf("Third update failed: %v", err)
	}
	if rgbWriteCount != 2 {
		t.Errorf("Expected RGB write count 2 after GPU changed, got %d", rgbWriteCount)
	}
	if brightnessWriteCount != 1 {
		t.Errorf("Expected brightness write to be skipped when brightness is unchanged, got %d", brightnessWriteCount)
	}
}

func TestService_UpdateLightbar_GpuTelemetryDegradation(t *testing.T) {
	var gotR, gotG, gotB, gotBrightness uint8

	mockLightbar := &MockLightbar{
		GetStatusFn: func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error {
			gotBrightness = brightness
			return nil
		},
		SetRGBFn: func(r, g, b uint8) error {
			gotR, gotG, gotB = r, g, b
			return nil
		},
	}

	gpuErr := fmt.Errorf("amd-smi failed")
	mockMonitor := &MockMonitor{
		GetCPUUsageFn: func() (uint16, error) { return 60, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) {
			if gpuErr != nil {
				return 0, 0, gpuErr
			}
			return 50, 40, nil
		},
		GetRAMUsageFn: func() (uint16, error) { return 40, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)

	// Update when GPU/NPU telemetry is failing
	// CPU=60, RAM=40, GPU/NPU fallback to 0
	// sum = 0 + 60 + 0 = 60 >= 45 (no baseline addition) -> R=0, G=60, B=0
	// avgUsage = (60 + 0 + 0)/3 = 20 -> load = (40 + 20)/2 = 30 -> brightness = 20 + (30*80)/100 = 44
	err := service.updateLightbar()
	if err != nil {
		t.Fatalf("Expected updateLightbar to succeed despite telemetry failure, got: %v", err)
	}
	if gotG != 60 || gotR != 0 || gotB != 0 {
		t.Errorf("Expected RGB (0, 60, 0) during telemetry degradation, got (%d, %d, %d)", gotR, gotG, gotB)
	}
	if gotBrightness != 44 {
		t.Errorf("Expected brightness 44 during telemetry degradation, got %d", gotBrightness)
	}

	// Now recover telemetry
	gpuErr = nil
	err = service.updateLightbar()
	if err != nil {
		t.Fatalf("Expected updateLightbar to succeed after recovery, got: %v", err)
	}
	if gotR != 50 || gotG != 60 || gotB != 40 {
		t.Errorf("Expected restored RGB (50, 60, 40), got (%d, %d, %d)", gotR, gotG, gotB)
	}
}

func TestService_Run_ContextCancel(t *testing.T) {
	mockLightbar := &MockLightbar{
		GetStatusFn:     func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error { return nil },
		SetRGBFn:        func(r, g, b uint8) error { return nil },
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn:       func() (uint16, error) { return 10, nil },
		GetGpuAndNpuUsageFn: func() (uint16, uint16, error) { return 10, 10, nil },
		GetRAMUsageFn:       func() (uint16, error) { return 10, nil },
	}

	service := NewService(mockLightbar, mockMonitor, false)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately to test graceful termination
	cancel()

	done := make(chan struct{})
	go func() {
		service.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Succeeded in stopping gracefully
	case <-time.After(1 * time.Second):
		t.Fatal("Service.Run did not terminate upon context cancellation")
	}
}

func TestService_SetInterval(t *testing.T) {
	service := NewService(&MockLightbar{}, &MockMonitor{}, false)

	// Valid positive interval
	service.SetInterval(250 * time.Millisecond)
	if service.interval != 250*time.Millisecond {
		t.Errorf("Expected interval 250ms, got %v", service.interval)
	}

	// Non-positive interval should be ignored
	service.SetInterval(-1 * time.Second)
	if service.interval != 250*time.Millisecond {
		t.Errorf("Expected interval to remain 250ms when negative interval is passed, got %v", service.interval)
	}
}
