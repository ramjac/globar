package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Service handles the background monitoring and lightbar updates
type Service struct {
	lightbar LightbarInterface
	monitor  MonitorInterface
	verbose  bool
}

// NewService creates a new Service instance
func NewService(lightbar LightbarInterface, monitor MonitorInterface, verbose bool) *Service {
	return &Service{
		lightbar: lightbar,
		monitor:  monitor,
		verbose:  verbose,
	}
}

// Run starts the background service loop
func (s *Service) Run(ctx context.Context) {
	log.Println("Starting Globar Service...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.updateLightbar(); err != nil {
				log.Printf("Error updating lightbar: %v", err)
			}
		case <-ctx.Done():
			log.Println("Service stopping gracefully:", ctx.Err())
			return
		}
	}
}

func (s *Service) updateLightbar() error {
	cpu, err := s.monitor.GetCPUUsage()
	if err != nil {
		return fmt.Errorf("failed to get CPU usage: %w", err)
	}

	gpu, npu, err := s.monitor.GetGpuAndNpuUsage()
	if err != nil {
		return fmt.Errorf("failed to get GPU/NPU usage: %w", err)
	}

	ram, err := s.monitor.GetRAMUsage()
	if err != nil {
		return fmt.Errorf("failed to get RAM usage: %w", err)
	}

	// Mapping rules:
	// GPU usage => Red intensity
	// CPU usage => Green intensity
	// NPU usage => Blue intensity
	// Brightness => half based on RAM usage and half based on an average of CPU/NPU/GPU usage.

	red := uint8(gpu)
	green := uint8(cpu)
	blue := uint8(npu)

	avgUsage := uint32((cpu + gpu + npu) / 3)
	brightness := uint8((uint32(ram) + avgUsage) / 2)

	err = s.lightbar.SetRGB(red, green, blue)
	if err != nil {
		return fmt.Errorf("failed to set RGB: %w", err)
	}

	err = s.lightbar.SetBrightness(brightness)
	if err != nil {
		return fmt.Errorf("failed to set brightness: %w", err)
	}

	if s.verbose {
		log.Printf("Updated Lightbar: Brightness=%d, R=%d, G=%d, B=%d (CPU:%d%%, GPU:%d%%, NPU:%d%%, RAM:%d%%)",
			brightness, red, green, blue, cpu, gpu, npu, ram)
	}

	return nil
}
