package main

import (
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
