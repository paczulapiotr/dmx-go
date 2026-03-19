package dmx

import (
	"fmt"

	"github.com/google/gousb"
)

const (
	// USB IDs for Eurolite DMX PRO 512 USB MK2 (FTDI FT232R)
	VendorID  = 0x0403
	ProductID = 0x6001

	// FTDI baud-rate control
	FTDISetBaudRate = 0x03
	BaudRate        = 250000
	BaudRateDivisor = 3000000 / BaudRate // = 12

	// DMX frame layout
	StartOfMessage = 0x7E
	EndOfMessage   = 0xE7
	DMXLabel       = 0x06
	DMXStartCode   = 0x00
	DMXChannels    = 512
	FrameSize      = 518 // 5-byte header + 512 data bytes + 1-byte footer
	USBEndpoint    = 0x02
)

// USBController controls the Eurolite USB DMX 512 PRO MK2 via direct USB (libusb).
type USBController struct {
	ctx   *gousb.Context
	dev   *gousb.Device
	cfg   *gousb.Config
	iface *gousb.Interface
	epOut *gousb.OutEndpoint
}

// OpenUSB opens and initialises the Eurolite DMX PRO 512 USB MK2 device.
func OpenUSB() (*USBController, error) {
	ctx := gousb.NewContext()

	dev, err := ctx.OpenDeviceWithVIDPID(VendorID, ProductID)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("could not open device: %w", err)
	}
	if dev == nil {
		ctx.Close()
		return nil, fmt.Errorf("device not found (VID=0x%04X, PID=0x%04X)", VendorID, ProductID)
	}

	// On macOS the Apple FTDI driver must be unloaded first; SetAutoDetach
	// handles Linux kernel driver detachment automatically.
	if err := dev.SetAutoDetach(true); err != nil {
		fmt.Printf("Warning: could not enable auto-detach: %v\n", err)
		fmt.Println("On macOS, unload the FTDI driver first:")
		fmt.Println("  sudo kextunload -b com.apple.driver.AppleUSBFTDI")
	}

	cfg, err := dev.Config(1)
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("could not get USB config: %w", err)
	}

	// Search interfaces 0-2 for bulk-out endpoint 0x02.
	var iface *gousb.Interface
	var epOut *gousb.OutEndpoint
	for ifaceNum := 0; ifaceNum < 3; ifaceNum++ {
		iface, err = cfg.Interface(ifaceNum, 0)
		if err != nil {
			continue
		}
		epOut, err = iface.OutEndpoint(USBEndpoint)
		if err == nil {
			fmt.Printf("[dmx] found endpoint 0x%02X on interface %d\n", USBEndpoint, ifaceNum)
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

	u := &USBController{ctx: ctx, dev: dev, cfg: cfg, iface: iface, epOut: epOut}

	if err := u.setBaudRate(); err != nil {
		u.Close()
		return nil, fmt.Errorf("could not set baud rate: %w", err)
	}

	fmt.Println("[dmx] Eurolite DMX PRO 512 USB MK2 ready")
	return u, nil
}

// setBaudRate configures the FTDI chip to 250 kbaud via a USB control transfer.
func (u *USBController) setBaudRate() error {
	const (
		requestTypeVendor = 0x40
		recipientDevice   = 0x00
		controlOut        = 0x00
	)
	value := uint16(BaudRateDivisor)
	index := uint16((BaudRateDivisor >> 8) & 0xFF00)
	rType := uint8(requestTypeVendor | recipientDevice | controlOut)

	if _, err := u.dev.Control(rType, FTDISetBaudRate, value, index, nil); err != nil {
		return fmt.Errorf("control transfer failed: %w", err)
	}
	return nil
}

// SendFrame implements effects.DMXDevice.
// It builds a complete 518-byte FTDI/DMX frame from the provided 512-channel
// universe and delivers it to the device in a single USB bulk write.
// data must be at least 512 bytes.
func (u *USBController) SendFrame(data []byte) error {
	if len(data) < 512 {
		return fmt.Errorf("SendFrame: need at least 512 bytes, got %d", len(data))
	}

	var frame [FrameSize]byte
	frame[0] = StartOfMessage
	frame[1] = DMXLabel
	frame[2] = 0x01 // length LSB (513 & 0xFF)
	frame[3] = 0x02 // length MSB (513 >> 8)
	frame[4] = DMXStartCode
	copy(frame[5:517], data[:512])
	frame[FrameSize-1] = EndOfMessage

	written, err := u.epOut.Write(frame[:])
	if err != nil {
		return fmt.Errorf("USB bulk write failed: %w", err)
	}
	if written != FrameSize {
		return fmt.Errorf("incomplete write: %d/%d bytes", written, FrameSize)
	}
	return nil
}

// Close releases all USB resources.
func (u *USBController) Close() error {
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
