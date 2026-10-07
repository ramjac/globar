package internal

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Service handles the background monitoring and lightbar updates
type Service struct {
	lightbar       LightbarInterface
	monitor        MonitorInterface
	verbose        bool
	hasLastApplied bool
	lastBrightness uint8
	lastRed        uint8
	lastGreen      uint8
	lastBlue       uint8
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

	// Clamp resource metrics to maximum 100%
	if gpu > 100 {
		gpu = 100
	}
	if cpu > 100 {
		cpu = 100
	}
	if npu > 100 {
		npu = 100
	}
	if ram > 100 {
		ram = 100
	}

	red := uint8(gpu)
	green := uint8(cpu)
	blue := uint8(npu)

	// giving the lightbar some baseline glow
	if red+green+blue < 45 {
		red += 15
		green += 15
		blue += 15
	}

	// Strictly clamp RGB values to 0-100 hardware range
	if red > 100 {
		red = 100
	}
	if green > 100 {
		green = 100
	}
	if blue > 100 {
		blue = 100
	}

	avgUsage := uint32((cpu + gpu + npu) / 3)
	// + 20 to give the lightbar a little baseline glow
	brightness := uint8((uint32(ram)+avgUsage)/2) + 20

	if brightness > 100 {
		brightness = 100
	}

	// Only write to sysfs if values have changed or on initial run
	rgbChanged := !s.hasLastApplied || red != s.lastRed || green != s.lastGreen || blue != s.lastBlue
	brightnessChanged := !s.hasLastApplied || brightness != s.lastBrightness

	if rgbChanged {
		err = s.lightbar.SetRGB(red, green, blue)
		if err != nil {
			return fmt.Errorf("failed to set RGB: %w", err)
		}
		s.lastRed = red
		s.lastGreen = green
		s.lastBlue = blue
	}

	if brightnessChanged {
		err = s.lightbar.SetBrightness(brightness)
		if err != nil {
			return fmt.Errorf("failed to set brightness: %w", err)
		}
		s.lastBrightness = brightness
	}

	s.hasLastApplied = true

	if s.verbose {
		log.Printf("Updated Lightbar: Brightness=%d, R=%d, G=%d, B=%d (CPU:%d%%, GPU:%d%%, NPU:%d%%, RAM:%d%%)",
			brightness, red, green, blue, cpu, gpu, npu, ram)
	}

	return nil
}
