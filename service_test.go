package main

import (
	"fmt"
	"testing"
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
	GetCPUUsageFn func() (uint32, error)
	GetGPUUsageFn func() (uint32, error)
	GetNPUUsageFn func() (uint32, error)
	GetRAMUsageFn func() (uint32, error)
}

func (m *MockMonitor) GetCPUUsage() (uint32, error) {
	return m.GetCPUUsageFn()
}

func (m *MockMonitor) GetGPUUsage() (uint32, error) {
	return m.GetGPUUsageFn()
}

func (m *MockMonitor) GetNPUUsage() (uint32, error) {
	return m.GetNPUUsageFn()
}

func (m *MockMonitor) GetRAMUsage() (uint32, error) {
	return m.GetRAMUsageFn()
}

func TestService_UpdateLightbar_Success(t *testing.T) {
	mockLightbar := &MockLightbar{
		GetStatusFn:     func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error { return nil },
		SetRGBFn:        func(r, g, b uint8) error { return nil },
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn: func() (uint32, error) { return 20, nil },
		GetGPUUsageFn: func() (uint32, error) { return 30, nil },
		GetNPUUsageFn: func() (uint32, error) { return 40, nil },
		GetRAMUsageFn: func() (uint32, error) { return 50, nil },
	}

	service := NewService(mockLightbar, mockMonitor)
	err := service.updateLightbar()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestService_UpdateLightbar_MonitorError(t *testing.T) {
	mockLightbar := &MockLightbar{
		GetStatusFn:     func() (LightbarStatus, error) { return LightbarStatus{}, nil },
		SetBrightnessFn: func(brightness uint8) error { return nil },
		SetRGBFn:        func(r, g, b uint8) error { return nil },
	}
	mockMonitor := &MockMonitor{
		GetCPUUsageFn: func() (uint32, error) { return 0, fmt.Errorf("cpu error") },
	}

	service := NewService(mockLightbar, mockMonitor)
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
		GetCPUUsageFn: func() (uint32, error) { return 20, nil },
		GetGPUUsageFn: func() (uint32, error) { return 30, nil },
		GetNPUUsageFn: func() (uint32, error) { return 40, nil },
		GetRAMUsageFn: func() (uint32, error) { return 50, nil },
	}

	service := NewService(mockLightbar, mockMonitor)
	err := service.updateLightbar()
	if err == nil {
		t.Error("Expected error from lightbar, got nil")
	}
}
