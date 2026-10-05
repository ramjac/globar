package main

import (
	"fmt"
	"log"
	"time"
)

// Service handles the background monitoring and lightbar updates
type Service struct {
	lightbar   LightbarInterface
	monitor    MonitorInterface
	cpuHistory []uint32
	gpuHistory []uint32
	npuHistory []uint32
	ramHistory []uint32
}

// NewService creates a new Service instance
func NewService(lightbar LightbarInterface, monitor MonitorInterface) *Service {
	return &Service{
		lightbar:   lightbar,
		monitor:    monitor,
		cpuHistory: make([]uint32, 0, 3),
		gpuHistory: make([]uint32, 0, 3),
		npuHistory: make([]uint32, 0, 3),
		ramHistory: make([]uint32, 0, 3),
	}
}

// Run starts the background service loop
func (s *Service) Run() {
	log.Println("Starting Globar Lightbar Service...")

	for {
		if err := s.updateLightbar(); err != nil {
			log.Printf("Error updating lightbar: %v", err)
		}

		time.Sleep(1 * time.Second)
	}
}

func (s *Service) updateHistory(history *[]uint32, val uint32) uint32 {
	*history = append(*history, val)
	if len(*history) > 3 {
		*history = (*history)[1:]
	}

	var sum uint64
	for _, v := range *history {
		sum += uint64(v)
	}
	return uint32(sum / uint64(len(*history)))
}

func (s *Service) updateLightbar() error {
	cpu, err := s.monitor.GetCPUUsage()
	if err != nil {
		return fmt.Errorf("failed to get CPU usage: %w", err)
	}

	gpu, err := s.monitor.GetGPUUsage()
	if err != nil {
		return fmt.Errorf("failed to get GPU usage: %w", err)
	}

	npu, err := s.monitor.GetNPUUsage()
	if err != nil {
		return fmt.Errorf("failed to get NPU usage: %w", err)
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

	avgCpu := s.updateHistory(&s.cpuHistory, cpu)
	avgGpu := s.updateHistory(&s.gpuHistory, gpu)
	avgNpu := s.updateHistory(&s.npuHistory, npu)
	avgRam := s.updateHistory(&s.ramHistory, ram)

	red := uint8(avgGpu)
	green := uint8(avgCpu)
	blue := uint8(avgNpu)

	avgUsage := uint32((uint64(avgCpu) + uint64(avgGpu) + uint64(avgNpu)) / 3)
	brightness := uint32((uint64(avgRam) + uint64(avgUsage)) / 2)

	err = s.lightbar.SetRGB(red, green, blue)
	if err != nil {
		return fmt.Errorf("failed to set RGB: %w", err)
	}

	err = s.lightbar.SetBrightness(uint8(brightness))
	if err != nil {
		return fmt.Errorf("failed to set brightness: %w", err)
	}

	log.Printf("Updated Lightbar: Brightness=%d, R=%d, G=%d, B=%d (CPU:%d%%, GPU:%d%%, NPU:%d%%, RAM:%d%%)",
		brightness, red, green, blue, cpu, gpu, npu, ram)

	return nil
}
