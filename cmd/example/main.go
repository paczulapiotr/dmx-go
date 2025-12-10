package main

import (
	"fmt"
	"log"
	"time"

	"github.com/youruser/dmxmk2/dmx"
)

func main() {
	// Replace with your serial device:
	// macOS: /dev/cu.usbserial-XXXX
	// Linux: /dev/ttyUSB0 or /dev/ttyUSB1
	// Windows: COM3
	device := "/dev/cu.usbserial-AG0KX5S3"

	controller, err := dmx.Open(device)
	if err != nil {
		log.Fatalf("open device: %v", err)
	}
	defer controller.Close()

	// Eurolite LED PST-10 QCL Spot (9-channel mode)
	// IMPORTANT: Check what DMX address your light is set to!
	// The light may be set to DMX address 1, 10, 50, etc.
	// Channels are:
	// CH1: Dimmer (0-255)
	// CH2: Red (0-255)
	// CH3: Green (0-255)
	// CH4: Blue (0-255)
	// CH5: White (0-255)
	// CH6: Strobe (0-10 no strobe)
	// CH7: Mode (0-9 no function)
	// CH8: Speed
	// CH9: Sound

	// Set the DMX starting address of your fixture
	// If your light is set to address 1, use startAddr = 1
	// If your light is set to address 10, use startAddr = 10, etc.
	startAddr := 1

	// Set to Full Red
	_ = controller.SetChannel(startAddr+0, 255) // Dimmer: 100%
	_ = controller.SetChannel(startAddr+1, 255) // Red: 100%
	_ = controller.SetChannel(startAddr+2, 0)   // Green: 0%
	_ = controller.SetChannel(startAddr+3, 0)   // Blue: 0%
	_ = controller.SetChannel(startAddr+4, 0)   // White: 0%
	_ = controller.SetChannel(startAddr+5, 0)   // Strobe: Open/Off
	_ = controller.SetChannel(startAddr+6, 0)   // Mode: None
	_ = controller.SetChannel(startAddr+7, 0)   // Speed: 0
	_ = controller.SetChannel(startAddr+8, 0)   // Sound: 0

	// send one packet (MK2 will hold the frame)
	if err := controller.SendDmx(); err != nil {
		log.Fatalf("send dmx: %v", err)
	}
	fmt.Println("Sent DMX frame (Full Red)")
	fmt.Printf("Light should be red at DMX address %d\n", startAddr)
	fmt.Println("If the light doesn't turn on, check:")
	fmt.Println("1. DMX address setting on the fixture")
	fmt.Println("2. DMX cable connections (XLR connectors)")
	fmt.Println("3. Light is powered on and in DMX mode")

	// Keep light on for 5 seconds
	fmt.Println("Keeping light on for 5 seconds...")
	time.Sleep(5 * time.Second)

	// Optional: start continuous sending (for effects)
	controller.StartAutoSend(30 * time.Millisecond)

	// Ramp Dimmer (CH1) down smoothly to demonstrate control
	fmt.Println("Ramping down dimmer...")
	for i := 255; i >= 0; i -= 5 {
		_ = controller.SetChannel(startAddr+0, byte(i))
		time.Sleep(50 * time.Millisecond)
	}

	// Stop auto-send and exit
	controller.Close()
}
