package main

import (
	"fmt"
	"log"
	"time"

	"github.com/youruser/dmxmk2/dmx"
)

func main() {
	device := "/dev/cu.usbserial-AG0KX5S3"

	controller, err := dmx.Open(device)
	if err != nil {
		log.Fatalf("open device: %v", err)
	}
	defer controller.Close()

	fmt.Println("DMX Address Scanner")
	fmt.Println("This will scan through DMX addresses 1-100 and set full red")
	fmt.Println("Watch your fixture - note which address makes it light up!")
	fmt.Println()

	// Start auto-send
	controller.StartAutoSend(30 * time.Millisecond)

	// Scan addresses 1 to 100
	for addr := 1; addr <= 100; addr++ {
		// Clear all channels first
		for i := 1; i <= 512; i++ {
			_ = controller.SetChannel(i, 0)
		}

		// Set 9 channels starting at this address (for PST-10 QCL 9-ch mode)
		_ = controller.SetChannel(addr+0, 255) // Dimmer: 100%
		_ = controller.SetChannel(addr+1, 255) // Red: 100%
		_ = controller.SetChannel(addr+2, 0)   // Green: 0%
		_ = controller.SetChannel(addr+3, 0)   // Blue: 0%
		_ = controller.SetChannel(addr+4, 0)   // White: 0%
		_ = controller.SetChannel(addr+5, 0)   // Strobe: Off
		_ = controller.SetChannel(addr+6, 0)   // Mode: None
		_ = controller.SetChannel(addr+7, 0)   // Speed: 0
		_ = controller.SetChannel(addr+8, 0)   // Sound: 0

		fmt.Printf("Testing address %3d... ", addr)
		time.Sleep(500 * time.Millisecond)
		fmt.Println("next")
	}

	fmt.Println("\nScan complete! Did the light turn on? If yes, note the address number.")
	controller.Close()
}
