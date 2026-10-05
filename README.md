# Stats Light Bar

This is a fun little service that will update your AMD Halo box light bar color and brightness dynamically based on system resource usage.

The CLI in main.go is provided for trying out the classes more manually. Running `go run . -r 20 -g 90 -b 10 -brightness 90` will set the lightbar. Running `go run . -w` will print the current resource usage and lightbar settings.

Thanks to the AMD folks for releasing the [light bar driver](https://lore.kernel.org/platform-driver-x86/20260427022546.1407923-1-superm1@kernel.org/).
Credit to [xdna-top](https://github.com/boxwrench/xdna-top) for figuring out how to monitor resource usage.

## Running as a Background Service

To run the lightbar service in the background on your AMD Halo dev box:

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd globar
   ```

2. **Install Go:**
   Ensure you have Go installed (version 1.27.1 or later is recommended).

3. **Ensure dependencies are met:**
   The service relies on `amd-smi` being available at `/opt/rocm/bin/amd-smi` for NPU monitoring.

4. **Run the service in the background:**
   You can use `nohup` or a systemd service to keep it running:
   ```bash
   nohup go run . -s > globar.log 2>&1 &
   ```
   Alternatively, for a more permanent setup, create a systemd unit file.

## Permissions

In order for setting the light to work, you'll need to add the user running this application to the "halo-lp" group and reboot: `sudo usermod -aG halo-lp <username>`

## Build

To compile the application into an executable binary for this developer box environment:
```bash
go build -o globar .
```

## System Service (systemd) for AMD Halo Linux Development Box

For persistent operation as a service on Debian 13, create and enable a systemd unit file. **Ensure the user running this service belongs to the `halo-lp` group.**

1.  **Create the service file:** Save the following content as `/etc/systemd/system/globar.service`:

    ```ini
    [Unit]
    Description=Globar Light Bar Service
    After=network.target

    [Service]
    User=<your_username> # IMPORTANT: Replace <your_username> with the actual username running the service.
    Group=halo-lp       # Ensure this group exists and user is a member.
    WorkingDirectory=/home/ramjac/globar # Adjust if not in /home/ramjac/globar
    ExecStart=/path/to/your/globar # Use the absolute path to the compiled binary, e.g., /opt/globar
    Restart=always

    [Install]
    WantedBy=multi-user.target
    ```

2.  **Reload systemd and enable service:** After building the executable (e.g., `go build -o globar .`) and placing it in a suitable location (e.g., `/opt/globar`), run:
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable globar.service
    sudo systemctl start globar.service
    ```

    Checkt the status `sudo systemctl status globar`

### Notes on Development Box Constraints:

*   **OS:** This guide is tailored for Debian 13 (AMD Ryzen Halo Linux Developer Box). Adjust paths and service names as necessary for other environments.
*   **Permissions:** The `halo-lp` group must be present, and the user running the service must be a member of this group to control the LED lightbar.