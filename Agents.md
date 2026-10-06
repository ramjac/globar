# Project: Globar Lightbar Service

## Overview
Stats Light Bar is a service designed to dynamically update the RGB LED lightbar on an AMD Halo Box based on real-time system resource usage.

## Goals
- **Dynamic Color Mapping (Mapping resource usage to lightbar intensity)**:
    - **Red**: Intensity driven by GPU usage.
    - **Green**: Intensity driven by CPU usage.
    - **Blue**: Intensity driven by NPU usage.
- **Dynamic Brightness**:
    - **Brightness**: Set based on the overall system load.
- **Service-Oriented**: The core logic will reside in a background service.

## Design Principles
- **Resource Efficiency**: This application must use extremely minimal system resources.
- **Approximate Accuracy**: Precision is not a requirement as this is a visual feature.
    - Resource usage reads can be within +/- 10% of actual values.
    - Timing lags of up to 300ms are acceptable.
- **Data Types**: Lightbar settings (brightness and intensity) must be whole number values in the range 0-100. This is a constraint of the lightbar hardware interface and is independent of the scale or units of the system resources being monitored. To minimize memory footprint and maximize efficiency, use small integer types (e.g., `uint8`) instead of floating-point numbers.
- **Verification**: An AI agent must always validate its changes by attempting to build the project (e.g., `go build ./cmd/main.go`) and running the unit tests (e.g., `go test ./...`) to ensure no regressions or compilation errors were introduced.

## Current Status
- **Fully Implemented Service**: A background service is implemented that dynamically maps system resource usage to lightbar colors and brightness.
- **Resource Monitoring**: The service monitors CPU, GPU, NPU, and RAM usage in real-time.
- **CLI Tool**: A comprehensive CLI tool is provided for:
    - Manual control of lightbar RGB and brightness.
    - Real-time monitoring of system resources and lightbar status (watch mode).
    - Launching the background service.


## Hardware Interface (Sysfs)
The lightbar is controlled via the following sysfs paths:
- Base Path: `/sys/class/leds/amd_halo:multicolor:status/`
- Brightness: `brightness`
- Multi-intensity: `multi_intensity` (format: `R G B`)
