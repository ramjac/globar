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

## Future Work

### 1. Correctness & Bug Fixes
- **Sysfs Path Construction**: Replace string concatenation (`l.basePath + filename`) with `filepath.Join` in `internal/lightbar.go` so paths without trailing slashes resolve correctly.
- **Hardware Value Clamping**: Ensure `red`, `green`, and `blue` values are explicitly clamped to the 0–100 range in `internal/service.go` to strictly adhere to the lightbar hardware interface constraint.
- **CLI Exit Status**: Fix `cmd/main.go` to emit errors to `os.Stderr` and exit with a non-zero status code (`os.Exit(1)`) on validation or hardware failure, rather than returning with status 0.
- **Partial RGB Flag Handling**: In `cmd/main.go`, when only a subset of RGB flags is specified (e.g., `-b 80`), preserve unprovided channels by reading current hardware status instead of resetting them to 0.

### 2. Resource Efficiency & Performance
- **Zero-Sleep CPU Monitoring**: In continuous monitoring mode (`Service` and watch mode), compute CPU usage as a delta between consecutive ticks rather than sleeping 100ms in a goroutine every second. This reduces `/proc/stat` file reads by 50%, eliminates scheduling overhead, and prevents goroutine leak/timeout risks.
- **Conditional Sysfs Writes**: Cache the last written RGB and brightness values in `internal/service.go` and skip writing to `/sys` when values have not changed, saving thousands of redundant file operations during idle/steady states.
- **Low-Allocation Procfs Parsing**: Parse `/proc/stat` and `/proc/meminfo` line-by-line or slice by first newline rather than using `strings.Split` on the entire file, reducing unnecessary heap allocations.

### 3. Resilience & Graceful Degradation
- **Non-blocking Telemetry Degradation**: In `internal/service.go`, prevent `amd-smi` failures or timeouts from blocking CPU and RAM updates to the lightbar; gracefully fallback GPU/NPU readings to 0 (or last known) while keeping other metrics active.
- **Dynamic `amd-smi` Discovery**: Support discovering `amd-smi` from `$PATH` and via an `AMD_SMI_PATH` environment variable in addition to `/opt/rocm/bin/amd-smi`.
- **Log Storm Suppression**: Deduplicate repeating error logs in the service loop during persistent telemetry or hardware access issues.

### 4. CLI Ergonomics & System Integration
- **CLI RAM Display**: Include RAM usage in `cmd/main.go` status output and watch mode (`-w`), matching the service's brightness calculation.
- **Configurable Interval & Versioning**: Add `-interval` (defaulting to 1s) and `-version` flags to the CLI.
- **Systemd Unit Template**: Provide a tracked `globar.service` unit file template in the repository for easier installation.

### 5. Improve Resource Use to Light Output Mapping
- **Currently**: Changes in resource usage or system load have no impact on the lightbar settting if they are below the hardcoded baseline brightness and RGB values.
- **Future State**: Update the mapping of resource usage to that it uses only the usable portion of the lightbar settings values. For example, if the baseline brightness is 20 and the maximum brightness is 100, then the system load percent should map to a range of 20 to 100.

### 6. Testing & Test Coverage
- **Value Assertion**: Verify exact RGB and brightness outputs in `internal/service_test.go` rather than only checking error returns.
- **Edge Case Tests**: Add test coverage for baseline glow logic, clamping limits, and graceful context cancellation in `Service.Run(ctx)`.
- **Direct Monitor Coverage**: Add unit tests exercising `Monitor` methods directly.
- **Dependency Injection**: Refactor the mutable global `getAmdSmiData` into an injectable runner on `Monitor` to ensure thread-safe, race-free testing.

