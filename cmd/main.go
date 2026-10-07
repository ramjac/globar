package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"globar/internal"
)

// Version of globar (can be set during build with -ldflags "-X main.Version=...")
var Version = "1.0.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, nil); code != 0 {
		os.Exit(code)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, lightbar internal.LightbarInterface) int {
	fs := flag.NewFlagSet("globar", flag.ContinueOnError)
	fs.SetOutput(stderr)

	watch := fs.Bool("w", false, "Watch mode: keep running and updating usage values periodically")
	runService := fs.Bool("s", false, "Run as a background service")
	verbose := fs.Bool("v", false, "Enable verbose logging")
	version := fs.Bool("version", false, "Print version information and exit")
	interval := fs.Duration("interval", 1*time.Second, "Update interval for watch and service modes (e.g. 1s, 500ms)")
	red := fs.Int("r", -1, "Set Red intensity (0-100)")
	green := fs.Int("g", -1, "Set Green intensity (0-100)")
	blue := fs.Int("b", -1, "Set Blue intensity (0-100)")
	brightness := fs.Int("brightness", -1, "Set brightness (0-100)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	if *version {
		fmt.Fprintf(stdout, "globar version %s\n", Version)
		return 0
	}

	if *interval <= 0 {
		fmt.Fprintln(stderr, "Error: interval must be a positive duration (e.g. 1s, 500ms)")
		return 1
	}

	if lightbar == nil {
		lightbar = internal.NewLightbar("")
	}

	monitor := internal.NewMonitor()

	if *runService {
		service := internal.NewService(lightbar, monitor, *verbose)
		service.SetInterval(*interval)
		service.Run(ctx)
		return 0
	}

	// Handle manual setting if flags are provided
	if *red != -1 || *green != -1 || *blue != -1 || *brightness != -1 {
		if *brightness != -1 {
			if *brightness < 0 || *brightness > 100 {
				fmt.Fprintln(stderr, "Error: brightness must be between 0 and 100")
				return 1
			}
			err := lightbar.SetBrightness(uint8(*brightness))
			if err != nil {
				fmt.Fprintf(stderr, "Error setting brightness: %v\n", err)
				return 1
			}
		}

		if *red != -1 || *green != -1 || *blue != -1 {
			if (*red != -1 && (*red < 0 || *red > 100)) ||
				(*green != -1 && (*green < 0 || *green > 100)) ||
				(*blue != -1 && (*blue < 0 || *blue > 100)) {
				fmt.Fprintln(stderr, "Error: RGB values must be between 0 and 100")
				return 1
			}

			r, g, b := 0, 0, 0
			// Preserve existing channels when only a subset is provided
			if *red == -1 || *green == -1 || *blue == -1 {
				if status, err := lightbar.GetStatus(); err == nil {
					r = int(status.Red)
					g = int(status.Green)
					b = int(status.Blue)
				}
			}

			if *red != -1 {
				r = *red
			}
			if *green != -1 {
				g = *green
			}
			if *blue != -1 {
				b = *blue
			}

			err := lightbar.SetRGB(uint8(r), uint8(g), uint8(b))
			if err != nil {
				fmt.Fprintf(stderr, "Error setting RGB: %v\n", err)
				return 1
			}
		}

		// If we just set something and aren't in watch mode, exit after printing status
		if !*watch {
			status, err := lightbar.GetStatus()
			if err != nil {
				fmt.Fprintf(stderr, "Error reading lightbar status: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "Lightbar Status: Brightness=%d, R=%d, G=%d, B=%d\n", status.Brightness, status.Red, status.Green, status.Blue)
			return 0
		}
	}

	printAll := func() {
		cpu, err := monitor.GetCPUUsage()
		if err != nil {
			fmt.Fprintf(stderr, "Error reading CPU usage: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "CPU Usage: %d\n", cpu)
		}

		gpu, npu, err := monitor.GetGpuAndNpuUsage()
		if err != nil {
			fmt.Fprintf(stderr, "Error reading GPU or NPU usage: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "GPU Usage: %d\n", gpu)
			fmt.Fprintf(stdout, "NPU Usage: %d\n", npu)
		}

		ram, err := monitor.GetRAMUsage()
		if err != nil {
			fmt.Fprintf(stderr, "Error reading RAM usage: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "RAM Usage: %d\n", ram)
		}

		status, err := lightbar.GetStatus()
		if err != nil {
			fmt.Fprintf(stderr, "Error reading lightbar status: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "Lightbar Status: Brightness=%d, R=%d, G=%d, B=%d\n", status.Brightness, status.Red, status.Green, status.Blue)
		}
	}

	if !*watch {
		printAll()
		return 0
	}

	fmt.Fprintln(stdout, "Starting resource monitoring (watch mode)... Press Ctrl+C to stop.")
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			printAll()
			fmt.Fprintln(stdout, "---------------------------")
		case <-ctx.Done():
			fmt.Fprintln(stdout, "Watch mode stopping gracefully.")
			return 0
		}
	}
}
