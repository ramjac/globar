# Globar

Globar is a lightweight service and CLI tool designed to dynamically update the RGB LED lightbar on an AMD Halo Developer Box (Debian 13 / AMD Ryzen 395+) based on real-time system resource usage.

### Features
- **Dynamic Color Mapping**:
  - **Red**: Intensity driven by GPU usage.
  - **Green**: Intensity driven by CPU usage.
  - **Blue**: Intensity driven by NPU usage.
- **Dynamic Brightness**: Automatically adjusts brightness based on overall system load (RAM and average CPU/GPU/NPU usage).
- **Comprehensive CLI**: Provides manual control, live status monitoring (watch mode), and headless background service operation.
- **Hardware Integration**: Directly controls the lightbar via Linux sysfs (`/sys/class/leds/amd_halo:multicolor:status/`) and gathers NPU telemetry via ROCm `amd-smi`.

---

## Prerequisites & Permissions

1. **Hardware & OS**: Designed for the AMD Ryzen Halo Linux Developer Box running linux.
2. **Permissions**: Controlling the lightbar requires the user running the tool to belong to the `halo-lp` group:
   ```bash
   sudo usermod -aG halo-lp $USER
   ```
   *Log out and log back in (or reboot) for group membership to take effect.*
3. **NPU Telemetry**: Ensure `amd-smi` is available at `/opt/rocm/bin/amd-smi` for NPU monitoring.

---

## Installation

### Option 1: Download Pre-built Binary (Recommended)

Pre-built statically linked binaries are provided on the [Releases](https://github.com/ramjac/globar/releases) page. No Go installation is required.

```bash
# Download the latest Linux x86_64 binary
curl -LO https://github.com/ramjac/globar/releases/latest/download/globar-linux-amd64

# Make executable and move to PATH
chmod +x globar-linux-amd64
sudo mv globar-linux-amd64 /usr/local/bin/globar
```

### Option 2: Build from Source

If you prefer to compile from source (requires Go 1.27.1 or later):

```bash
# Clone the repository
git clone https://github.com/ramjac/globar.git
cd globar

# Build the executable
go build -o globar ./cmd/main.go

# (Optional) Run unit tests
go test ./...

# Move to PATH
sudo mv globar /usr/local/bin/
```

---

## CLI Usage

Run `globar` with the desired mode or options:

- **Manual Control**: Set specific color intensities and brightness directly:
  ```bash
  globar -r 20 -g 90 -b 10 -brightness 90
  ```
- **Watch Mode (`-w`)**: Monitor system resources (CPU, GPU, NPU, RAM) and lightbar status in real-time in your terminal:
  ```bash
  globar -w
  ```
- **Service Mode (`-s`)**: Run as an automated service that continuously maps system load to the lightbar:
  ```bash
  globar -s
  ```
- **Configurable Refresh Interval (`-interval`)**: Set custom update frequency for watch or service mode (default `1s`):
  ```bash
  globar -w -interval 500ms
  ```
- **Verbose Logging (`-v`)**: Enable detailed logs with service or watch mode:
  ```bash
  globar -s -v
  ```
- **Version (`-version`)**: Check binary version:
  ```bash
  globar -version
  ```

---

## Running as a Background Service

### Systemd Service (Recommended)

For persistent operation on your AMD Halo box across reboots:

1. **Install the systemd unit file** using the tracked `globar.service` template from this repository:

   ```bash
   # Copy the unit file from repository to systemd directory
   sudo cp globar.service /etc/systemd/system/

   # Edit User to your username (must belong to halo-lp group)
   sudo sed -i "s/<your_username>/$USER/" /etc/systemd/system/globar.service
   ```

   The unit file contents:

   ```ini
   [Unit]
   Description=Globar Light Bar Service
   Documentation=https://github.com/ramjac/globar
   After=network.target

   [Service]
   User=<your_username>
   Group=halo-lp
   ExecStart=/usr/local/bin/globar -s
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
   > **Note**: Ensure the user belongs to the `halo-lp` group (`sudo usermod -aG halo-lp $USER`).

2. **Reload systemd, enable, and start the service:**

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now globar.service
   ```

3. **Check service status and logs:**

   ```bash
   systemctl status globar.service
   journalctl -u globar.service -f
   ```

---

## Acknowledgments
- Thanks to AMD for releasing the [light bar driver](https://lore.kernel.org/platform-driver-x86/20260427022546.1407923-1-superm1@kernel.org/).
- Credit to [xdna-top](https://github.com/boxwrench/xdna-top) for figuring out how to monitor resource usage.
