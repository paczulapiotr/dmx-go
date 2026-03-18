package main

import (
	"fmt"
	"log"
	"time"

	"github.com/youruser/dmxmk2/dmx"
)

func main() {
	// Open device via USB (libusb)
	controller, err := dmx.OpenUSB()
	if err != nil {
		log.Fatalf("Failed to open USB device: %v", err)
	}
	defer controller.Close()

	fmt.Println("Device opened successfully!")

	// Configure for Eurolite LED PST-10 QCL Spot (9-channel mode)
	// Channels: 1=Dimmer, 2=Red, 3=Green, 4=Blue, 5=White, 6=Strobe, 7=Color preset, 8=Auto, 9=Dimmer curve
	startAddr := 1

	// Set full red: Dimmer=255, Red=255, others=0
	controller.SetChannel(startAddr, 255)   // Dimmer
	controller.SetChannel(startAddr+1, 255) // Red
	controller.SetChannel(startAddr+2, 0)   // Green
	controller.SetChannel(startAddr+3, 0)   // Blue
	controller.SetChannel(startAddr+4, 0)   // White
	controller.SetChannel(startAddr+5, 0)   // Strobe
	controller.SetChannel(startAddr+6, 0)   // Color preset
	controller.SetChannel(startAddr+7, 0)   // Auto
	controller.SetChannel(startAddr+8, 0)   // Dimmer curve

	fmt.Println("Sending full red light...")
	if err := controller.SendDMX(); err != nil {
		log.Fatalf("Failed to send DMX: %v", err)
	}

	fmt.Println("Holding for 5 seconds...")
	time.Sleep(5 * time.Second)

	// Ramp dimmer down
	fmt.Println("Ramping dimmer down...")
	for i := 255; i >= 0; i -= 5 {
		controller.SetChannel(startAddr, byte(i))
		if err := controller.SendDMX(); err != nil {
			log.Printf("Error sending: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println("Done!")
}
