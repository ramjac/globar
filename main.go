package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	watch := flag.Bool("w", false, "Watch mode: keep running and updating usage values once per second")
	runService := flag.Bool("s", false, "Run as a background service")
	verbose := flag.Bool("v", false, "Enable verbose logging")
	red := flag.Int("r", -1, "Set Red intensity (0-100)")
	green := flag.Int("g", -1, "Set Green intensity (0-100)")
	blue := flag.Int("b", -1, "Set Blue intensity (0-100)")
	brightness := flag.Int("brightness", -1, "Set brightness (0-100)")
	flag.Parse()

	lightbar := NewLightbar("")

	if *runService {
		service := NewService(lightbar, NewMonitor(), *verbose)
		service.Run(ctx)
		return
	}

	// Handle manual setting if flags are provided
	if *red != -1 || *green != -1 || *blue != -1 || *brightness != -1 {
		if *brightness != -1 {
			if *brightness < 0 || *brightness > 100 {
				fmt.Println("Error: brightness must be between 0 and 100")
				return
			}
			err := lightbar.SetBrightness(uint8(*brightness))
			if err != nil {
				fmt.Printf("Error setting brightness: %v\n", err)
				return
			}
		}

		if *red != -1 || *green != -1 || *blue != -1 {
			r, g, b := 0, 0, 0
			if *red != -1 {
				r = *red
			}
			if *green != -1 {
				g = *green
			}
			if *blue != -1 {
				b = *blue
			}

			if r < 0 || r > 100 || g < 0 || g > 100 || b < 0 || b > 100 {
				fmt.Println("Error: RGB values must be between 0 and 100")
				return
			}

			err := lightbar.SetRGB(uint8(r), uint8(g), uint8(b))
			if err != nil {
				fmt.Printf("Error setting RGB: %v\n", err)
				return
			}
		}

		// If we just set something and aren't in watch mode, we can exit after printing status
		if !*watch {
			status, err := lightbar.GetStatus()
			if err != nil {
				fmt.Printf("Error reading lightbar status: %v\n", err)
			} else {
				fmt.Printf("Lightbar Status: Brightness=%d, R=%d, G=%d, B=%d\n", status.Brightness, status.Red, status.Green, status.Blue)
			}
			return
		}
	}

	printAll := func() {
		cpu, err := GetCPUUsage()
		if err != nil {
			fmt.Printf("Error reading CPU usage: %v\n", err)
		} else {
			fmt.Printf("CPU Usage: %d\n", cpu)
		}

		gpu, npu, err := GetGpuAndNpuUsage()
		if err != nil {
			fmt.Printf("Error reading GPU or NPU usage: %v\n", err)
		} else {
			fmt.Printf("GPU Usage: %d\n", gpu)
			fmt.Printf("NPU Usage: %d\n", npu)
		}

		status, err := lightbar.GetStatus()
		if err != nil {
			fmt.Printf("Error reading lightbar status: %v\n", err)
		} else {
			fmt.Printf("Lightbar Status: Brightness=%d, R=%d, G=%d, B=%d\n", status.Brightness, status.Red, status.Green, status.Blue)
		}
	}

	if !*watch {
		printAll()
	} else {
		fmt.Println("Starting resource monitoring (watch mode)... Press Ctrl+C to stop.")
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				printAll()
				fmt.Println("---------------------------")
			case <-ctx.Done():
				fmt.Println("Watch mode stopping gracefully:", ctx.Err())
				return
			}
		}
	}
}
