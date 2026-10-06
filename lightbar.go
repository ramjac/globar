package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Path to the base directory for the lightbar
const basePath = "/sys/class/leds/amd_halo:multicolor:status/"

// Filename for the file containing the individual Red/Green/Blue intensity settings
const multiIntensityFileName = "multi_intensity"

// Filename for the file containing the brightness setting
const brightnessFileName = "brightness"

// LightbarStatus stores the current settings of the lightbar
type LightbarStatus struct {
	Brightness uint8
	Red        uint8
	Green      uint8
	Blue       uint8
}

// LightbarInterface defines the methods available for controlling the lightbar
type LightbarInterface interface {
	GetStatus() (LightbarStatus, error)
	SetBrightness(brightness uint8) error
	SetRGB(r, g, b uint8) error
}

// Lightbar handles interacting with the lightbar hardware via sysfs
type Lightbar struct {
	basePath string
}

// NewLightbar creates a new Lightbar instance. Use basePath "" to use the default.
func NewLightbar(basePath string) *Lightbar {
	if basePath == "" {
		basePath = "/sys/class/leds/amd_halo:multicolor:status/"
	}
	return &Lightbar{
		basePath: basePath,
	}
}

// GetStatus reads the current brightness and RGB intensity from sysfs
func (l *Lightbar) GetStatus() (LightbarStatus, error) {
	status := LightbarStatus{}

	// Read brightness
	brightnessData, err := os.ReadFile(l.basePath + brightnessFileName)
	if err != nil {
		return status, fmt.Errorf("error reading brightness: %w", err)
	}
	brightnessVal, err := strconv.Atoi(strings.TrimSpace(string(brightnessData)))
	if err != nil {
		return status, fmt.Errorf("error parsing brightness: %w", err)
	}
	status.Brightness = uint8(brightnessVal)

	// Read multi_intensity
	intensityData, err := os.ReadFile(l.basePath + multiIntensityFileName)
	if err != nil {
		return status, fmt.Errorf("error reading multi_intensity: %w", err)
	}
	intensityFields := strings.Fields(string(intensityData))
	if len(intensityFields) < 3 {
		return status, fmt.Errorf("unexpected multi_intensity format: %s", string(intensityData))
	}

	redVal, err := strconv.Atoi(intensityFields[0])
	if err != nil {
		return status, fmt.Errorf("error parsing red intensity: %w", err)
	}
	greenVal, err := strconv.Atoi(intensityFields[1])
	if err != nil {
		return status, fmt.Errorf("error parsing green intensity: %w", err)
	}
	blueVal, err := strconv.Atoi(intensityFields[2])
	if err != nil {
		return status, fmt.Errorf("error parsing blue intensity: %w", err)
	}

	status.Red = uint8(redVal)
	status.Green = uint8(greenVal)
	status.Blue = uint8(blueVal)

	return status, nil
}

// SetBrightness sets the lightbar brightness
func (l *Lightbar) SetBrightness(brightness uint8) error {
	data := []byte(fmt.Sprintf("%d", brightness))
	err := os.WriteFile(l.basePath+brightnessFileName, data, 0644)
	if err != nil {
		return fmt.Errorf("error setting brightness: %w", err)
	}
	return nil
}

// SetRGB sets the lightbar RGB intensities
func (l *Lightbar) SetRGB(r, g, b uint8) error {
	data := []byte(fmt.Sprintf("%d %d %d", r, g, b))
	err := os.WriteFile(l.basePath+multiIntensityFileName, data, 0644)
	if err != nil {
		return fmt.Errorf("error setting RGB: %w", err)
	}
	return nil
}
