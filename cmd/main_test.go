package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"globar/internal"
)

type mockLightbar struct {
	brightness uint8
	red        uint8
	green      uint8
	blue       uint8

	getStatusErr     error
	setBrightnessErr error
	setRGBErr        error
}

func (m *mockLightbar) GetStatus() (internal.LightbarStatus, error) {
	if m.getStatusErr != nil {
		return internal.LightbarStatus{}, m.getStatusErr
	}
	return internal.LightbarStatus{
		Brightness: m.brightness,
		Red:        m.red,
		Green:      m.green,
		Blue:       m.blue,
	}, nil
}

func (m *mockLightbar) SetBrightness(b uint8) error {
	if m.setBrightnessErr != nil {
		return m.setBrightnessErr
	}
	m.brightness = b
	return nil
}

func (m *mockLightbar) SetRGB(r, g, b uint8) error {
	if m.setRGBErr != nil {
		return m.setRGBErr
	}
	m.red = r
	m.green = g
	m.blue = b
	return nil
}

func TestRun_InvalidBrightness(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"too low", "-5"},
		{"too high", "101"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			lb := &mockLightbar{}
			code := run(context.Background(), []string{"-brightness", tt.val}, &stdout, &stderr, lb)
			if code != 1 {
				t.Errorf("Expected exit code 1, got %d", code)
			}
			if !strings.Contains(stderr.String(), "brightness must be between 0 and 100") {
				t.Errorf("Expected error message in stderr, got: %s", stderr.String())
			}
		})
	}
}

func TestRun_InvalidRGB(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"red too high", []string{"-r", "105"}},
		{"green too high", []string{"-g", "101"}},
		{"blue too high", []string{"-b", "200"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			lb := &mockLightbar{}
			code := run(context.Background(), tt.args, &stdout, &stderr, lb)
			if code != 1 {
				t.Errorf("Expected exit code 1, got %d", code)
			}
			if !strings.Contains(stderr.String(), "RGB values must be between 0 and 100") {
				t.Errorf("Expected error message in stderr, got: %s", stderr.String())
			}
		})
	}
}

func TestRun_PartialRGBPreservesExistingChannels(t *testing.T) {
	lb := &mockLightbar{
		brightness: 50,
		red:        10,
		green:      20,
		blue:       30,
	}

	var stdout, stderr bytes.Buffer
	// Only specify blue
	code := run(context.Background(), []string{"-b", "80"}, &stdout, &stderr, lb)
	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}

	// Red and green should be preserved, blue should be 80
	if lb.red != 10 || lb.green != 20 || lb.blue != 80 {
		t.Errorf("Expected RGB to be (10, 20, 80), got (%d, %d, %d)", lb.red, lb.green, lb.blue)
	}
	if !strings.Contains(stdout.String(), "Lightbar Status: Brightness=50, R=10, G=20, B=80") {
		t.Errorf("Expected status in stdout, got: %s", stdout.String())
	}
}

func TestRun_SetBrightnessAndRGBSuccess(t *testing.T) {
	lb := &mockLightbar{}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"-brightness", "90", "-r", "10", "-g", "20", "-b", "30"}, &stdout, &stderr, lb)
	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}

	if lb.brightness != 90 || lb.red != 10 || lb.green != 20 || lb.blue != 30 {
		t.Errorf("Expected Lightbar settings (90, 10, 20, 30), got (%d, %d, %d, %d)", lb.brightness, lb.red, lb.green, lb.blue)
	}
}

func TestRun_HardwareErrors(t *testing.T) {
	t.Run("brightness error", func(t *testing.T) {
		lb := &mockLightbar{setBrightnessErr: fmt.Errorf("write error")}
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"-brightness", "50"}, &stdout, &stderr, lb)
		if code != 1 {
			t.Errorf("Expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "Error setting brightness") {
			t.Errorf("Expected stderr to contain brightness error, got: %s", stderr.String())
		}
	})

	t.Run("RGB error", func(t *testing.T) {
		lb := &mockLightbar{setRGBErr: fmt.Errorf("sysfs write failed")}
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"-r", "50"}, &stdout, &stderr, lb)
		if code != 1 {
			t.Errorf("Expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "Error setting RGB") {
			t.Errorf("Expected stderr to contain RGB error, got: %s", stderr.String())
		}
	})
}

func TestRun_HelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"-h"}, &stdout, &stderr, &mockLightbar{})
	if code != 0 {
		t.Errorf("Expected exit code 0 for help flag, got %d", code)
	}
}

func TestRun_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"-version"}, &stdout, &stderr, &mockLightbar{})
	if code != 0 {
		t.Fatalf("Expected exit code 0 for -version, got %d", code)
	}
	if !strings.Contains(stdout.String(), "globar version") {
		t.Errorf("Expected version in stdout, got: %s", stdout.String())
	}
}

func TestRun_Interval(t *testing.T) {
	t.Run("invalid negative interval", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"-interval", "-1s"}, &stdout, &stderr, &mockLightbar{})
		if code != 1 {
			t.Errorf("Expected exit code 1 for negative interval, got %d", code)
		}
		if !strings.Contains(stderr.String(), "interval must be a positive duration") {
			t.Errorf("Expected error message in stderr, got: %s", stderr.String())
		}
	})

	t.Run("invalid zero interval", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"-interval", "0s"}, &stdout, &stderr, &mockLightbar{})
		if code != 1 {
			t.Errorf("Expected exit code 1 for zero interval, got %d", code)
		}
	})
}

func TestRun_SingleShot_PrintsRAM(t *testing.T) {
	var stdout, stderr bytes.Buffer
	lb := &mockLightbar{brightness: 50, red: 10, green: 20, blue: 30}
	code := run(context.Background(), []string{}, &stdout, &stderr, lb)
	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	// Verify RAM Usage is included in single-shot output
	if !strings.Contains(stdout.String(), "RAM Usage:") && !strings.Contains(stderr.String(), "Error reading RAM usage:") {
		t.Errorf("Expected RAM Usage in stdout (or error in stderr), got stdout: %s", stdout.String())
	}
}
