package main

import (
	"fmt"
	"log"
	"time"

	"github.com/youruser/dmxmk2/dmx"
)

func main() {
	// Open device via USB (requires sudo)
	controller, err := dmx.OpenUSB()
	if err != nil {
		log.Fatalf("Failed to open USB device: %v\n", err)
		log.Fatalf("Try running with: sudo go run ./cmd/scanner_usb/main.go")
	}
	defer controller.Close()

	fmt.Println("=== DMX Address Scanner (USB) ===")
	fmt.Println("This will test DMX addresses 1-100")
	fmt.Println("Watch your light - when it turns RED, note the address!")
	fmt.Println()
	time.Sleep(2 * time.Second)

	// Scan addresses 1-100
	for addr := 1; addr <= 100; addr++ {
		fmt.Printf("Testing address %d...\n", addr)

		// Set 9 channels starting at this address (for PST-10 QCL)
		// Dimmer=255, Red=255, Green=0, Blue=0, White=0, Strobe=0, ColorPreset=0, Auto=0, DimmerCurve=0
		controller.SetChannel(addr, 255)     // Dimmer
		controller.SetChannel(addr+1, 255)   // Red
		controller.SetChannel(addr+2, 0)     // Green
		controller.SetChannel(addr+3, 0)     // Blue
		controller.SetChannel(addr+4, 0)     // White
		controller.SetChannel(addr+5, 0)     // Strobe
		controller.SetChannel(addr+6, 0)     // Color preset
		controller.SetChannel(addr+7, 0)     // Auto
		controller.SetChannel(addr+8, 0)     // Dimmer curve

		// Send DMX
		if err := controller.SendDMX(); err != nil {
			log.Printf("Error sending at address %d: %v", addr, err)
		}

		// Wait 500ms per address
		time.Sleep(500 * time.Millisecond)

		// Clear all channels before next test
		for i := addr; i <= addr+8; i++ {
			controller.SetChannel(i, 0)
		}
	}

	fmt.Println()
	fmt.Println("Scan complete!")
	fmt.Println("If your light didn't turn red at any address, check:")
	fmt.Println("  1. Is the light in DMX mode? (check fixture menu)")
	fmt.Println("  2. Is the DMX cable properly connected?")
	fmt.Println("  3. Is the light set to 9-channel mode?")
	fmt.Println("  4. Try addresses above 100 manually if needed")
}
