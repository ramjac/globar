package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLightbar(t *testing.T) {
	// Create a temporary directory for sysfs simulation
	tmpDir, err := os.MkdirTemp("", "lightbar_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create necessary files
	err = os.WriteFile(filepath.Join(tmpDir, "brightness"), []byte("50"), 0644)
	if err != nil {
		t.Fatalf("failed to create brightness file: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "multi_intensity"), []byte("10 20 30"), 0644)
	if err != nil {
		t.Fatalf("failed to create multi_intensity file: %v", err)
	}

	l := NewLightbar(tmpDir + "/")

	// Test GetStatus
	status, err := l.GetStatus()
	if err != nil {
		t.Errorf("GetStatus failed: %v", err)
	}
	if status.Brightness != 50 {
		t.Errorf("Expected brightness 50, got %d", status.Brightness)
	}
	if status.Red != 10 || status.Green != 20 || status.Blue != 30 {
		t.Errorf("Expected RGB 10 20 30, got %d %d %d", status.Red, status.Green, status.Blue)
	}

	// Test SetBrightness
	err = l.SetBrightness(75)
	if err != nil {
		t.Errorf("SetBrightness failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(tmpDir, "brightness"))
	if string(data) != "75" {
		t.Errorf("Expected brightness 75 in file, got %s", string(data))
	}

	// Test SetRGB
	err = l.SetRGB(40, 50, 60)
	if err != nil {
		t.Errorf("SetRGB failed: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(tmpDir, "multi_intensity"))
	if string(data) != "40 50 60" {
		t.Errorf("Expected RGB 40 50 60 in file, got %s", string(data))
	}
}

func TestLightbar_Errors(t *testing.T) {
	l := NewLightbar("/non/existent/path/")

	if _, err := l.GetStatus(); err == nil {
		t.Error("Expected error for GetStatus on non-existent path, got nil")
	}

	if err := l.SetBrightness(50); err == nil {
		t.Error("Expected error for SetBrightness on non-existent path, got nil")
	}

	if err := l.SetRGB(1, 2, 3); err == nil {
		t.Error("Expected error for SetRGB on non-existent path, got nil")
	}
}
