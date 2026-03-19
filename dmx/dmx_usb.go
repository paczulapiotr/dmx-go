package dmx

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/gousb"
)

const (
	// USB IDs for Eurolite DMX PRO 512 USB MK2 (FTDI FT232R)
	VendorID  = 0x0403
	ProductID = 0x6001

	// FTDI Constants
	FTDISetBaudRate = 0x03
	BaudRate        = 250000
	BaudRateDivisor = 3000000 / BaudRate // = 12

	// Frame constants (same as serial implementation)
	StartOfMessage = 0x7E
	EndOfMessage   = 0xE7
	DMXLabel       = 0x06
	DMXStartCode   = 0x00
	DMXChannels    = 512
	FrameSize      = 518
	USBEndpoint    = 0x02

	// Timeouts
	ControlTimeout = 500 // ms
	BulkTimeout    = 500 // ms
)

// USBController controls the Eurolite USB DMX 512 PRO MK2 via direct USB (libusb).
// This is the preferred method as it matches OLA's implementation exactly.
type USBController struct {
	ctx      *gousb.Context
	dev      *gousb.Device
	cfg      *gousb.Config
	iface    *gousb.Interface
	epOut    *gousb.OutEndpoint
	frame    [FrameSize]byte
	channels [512]byte
	mutex    sync.Mutex

	// auto-send
	autoSendInterval time.Duration
	autoSendQuit     chan struct{}
	autoSending      bool
}

// OpenUSB opens and initializes the Eurolite DMX PRO 512 USB MK2 device via libusb.
// This is the recommended approach as it matches OLA's implementation.
func OpenUSB() (*USBController, error) {
	ctx := gousb.NewContext()

	// Find the device
	dev, err := ctx.OpenDeviceWithVIDPID(VendorID, ProductID)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("could not open device: %w", err)
	}
	if dev == nil {
		ctx.Close()
		return nil, fmt.Errorf("device not found (VID=0x%04X, PID=0x%04X)", VendorID, ProductID)
	}

	// Try to set auto detach kernel driver (may require sudo/permissions)
	// On macOS, this requires the FTDI VCP driver to be unloaded first
	if err := dev.SetAutoDetach(true); err != nil {
		fmt.Printf("Warning: could not enable auto-detach: %v\n", err)
		fmt.Println("This is normal on macOS. Trying to continue anyway...")
		fmt.Println("If this fails, you may need to:")
		fmt.Println("  1. Run with sudo: sudo go run ./cmd/example_usb/main.go")
		fmt.Println("  2. OR unload FTDI driver: sudo kextunload -b com.apple.driver.AppleUSBFTDI")
		// Don't return error - try to continue
	}

	// Get the active config (usually config 1)
	cfg, err := dev.Config(1)
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("could not get config: %w", err)
	}

	// Find the interface with endpoint 0x02 (usually interface 0 or 1)
	var iface *gousb.Interface
	var epOut *gousb.OutEndpoint

	for ifaceNum := 0; ifaceNum < 3; ifaceNum++ {
		iface, err = cfg.Interface(ifaceNum, 0) // interface number, alt setting 0
		if err != nil {
			continue
		}

		// Look for endpoint 0x02
		epOut, err = iface.OutEndpoint(USBEndpoint)
		if err == nil {
			fmt.Printf("Found endpoint 0x%02X on interface %d\n", USBEndpoint, ifaceNum)
			break
		}
		iface.Close()
		iface = nil
	}

	if epOut == nil {
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("could not find endpoint 0x%02X", USBEndpoint)
	}

	dmx := &USBController{
		ctx:   ctx,
		dev:   dev,
		cfg:   cfg,
		iface: iface,
		epOut: epOut,
	}

	// Set baud rate to 250,000 (this is critical for MK2!)
	if err := dmx.setBaudRate(); err != nil {
		dmx.Close()
		return nil, fmt.Errorf("could not set baud rate: %w", err)
	}

	// Initialize frame header (static parts)
	dmx.initFrame()

	fmt.Println("Successfully initialized Eurolite DMX PRO 512 USB MK2 via USB")
	return dmx, nil
}

// setBaudRate configures the FTDI chip to 250,000 baud via USB control transfer.
// This matches OLA's implementation exactly.
func (u *USBController) setBaudRate() error {
	value := uint16(BaudRateDivisor)                 // divisor = 12
	index := uint16((BaudRateDivisor >> 8) & 0xFF00) // = 0

	fmt.Printf("Setting baud rate: divisor=%d, value=0x%04X, index=0x%04X\n", BaudRateDivisor, value, index)

	// RequestType constants from libusb
	const (
		RequestTypeVendor = 0x40 // (2 << 5)
		RecipientDevice   = 0x00
		ControlOut        = 0x00
	)

	rType := uint8(RequestTypeVendor | RecipientDevice | ControlOut)

	_, err := u.dev.Control(rType, FTDISetBaudRate, value, index, nil)
	if err != nil {
		return fmt.Errorf("control transfer failed: %w", err)
	}

	fmt.Println("Baud rate set successfully to 250,000")
	return nil
}

// initFrame initializes the static parts of the DMX frame
func (u *USBController) initFrame() {
	u.frame[0] = StartOfMessage
	u.frame[1] = DMXLabel
	u.frame[2] = 0x01 // Length LSB (513 & 0xFF)
	u.frame[3] = 0x02 // Length MSB (513 >> 8)
	u.frame[4] = DMXStartCode
	u.frame[FrameSize-1] = EndOfMessage
}

// SetChannel sets a single DMX channel (1..512).
func (u *USBController) SetChannel(channel int, value byte) error {
	if channel < 1 || channel > 512 {
		return fmt.Errorf("channel %d out of range", channel)
	}
	u.mutex.Lock()
	u.channels[channel-1] = value
	u.mutex.Unlock()
	return nil
}

// SetChannels sets multiple channels at once. Keys are 1..512.
func (u *USBController) SetChannels(values map[int]byte) error {
	u.mutex.Lock()
	for ch, val := range values {
		if ch >= 1 && ch <= 512 {
			u.channels[ch-1] = val
		}
	}
	u.mutex.Unlock()
	return nil
}

// SetChannelRange writes len(values) channels starting at startChannel (1-based).
// This is the preferred method for effects as it avoids map allocation.
func (u *USBController) SetChannelRange(startChannel int, values []byte) error {
	u.mutex.Lock()
	for i, v := range values {
		ch := startChannel - 1 + i
		if ch >= 0 && ch < 512 {
			u.channels[ch] = v
		}
	}
	u.mutex.Unlock()
	return nil
}

// SetAll sets all 512 channels from a slice (len must be 512).
func (u *USBController) SetAll(data []byte) error {
	if len(data) != 512 {
		return fmt.Errorf("data length must be 512")
	}
	u.mutex.Lock()
	copy(u.channels[:], data)
	u.mutex.Unlock()
	return nil
}

// SendDMX sends DMX data to the device via USB bulk transfer.
func (u *USBController) SendDMX() error {
	// Copy current channel state into frame
	u.mutex.Lock()
	copy(u.frame[5:517], u.channels[:])
	u.mutex.Unlock()

	// Send bulk transfer
	written, err := u.epOut.Write(u.frame[:])
	if err != nil {
		return fmt.Errorf("bulk transfer failed: %w", err)
	}

	if written != FrameSize {
		return fmt.Errorf("incomplete transfer: wrote %d bytes, expected %d", written, FrameSize)
	}

	// Successfully sent (debug: written %d bytes)
	return nil
}

// StartAutoSend starts continuously sending DMX packets at interval.
func (u *USBController) StartAutoSend(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Millisecond
	}
	if u.autoSending {
		u.autoSendInterval = interval
		return
	}
	u.autoSendInterval = interval
	u.autoSendQuit = make(chan struct{})
	u.autoSending = true

	go func() {
		t := time.NewTicker(u.autoSendInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_ = u.SendDMX()
			case <-u.autoSendQuit:
				return
			}
		}
	}()
}

// StopAutoSend stops the background auto-send if running.
func (u *USBController) StopAutoSend() {
	if u.autoSending {
		close(u.autoSendQuit)
		u.autoSending = false
	}
}

// Close releases all USB resources.
func (u *USBController) Close() error {
	u.StopAutoSend()
	if u.iface != nil {
		u.iface.Close()
	}
	if u.cfg != nil {
		u.cfg.Close()
	}
	if u.dev != nil {
		u.dev.Close()
	}
	if u.ctx != nil {
		u.ctx.Close()
	}
	return nil
}
