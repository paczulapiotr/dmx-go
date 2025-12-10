package dmx

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// DMXController controls a Eurolite USB DMX 512 PRO (MK2).
// It implements the MK2 framed packet protocol:
//
//   Header:  0x7E, 0x06, LSB(length), MSB(length), StartCode(0x00)
//   Data:    512 bytes (channels 1..512)
//   Footer:  0xE7
//
// length = 513 (start code + 512 data) -> LSB=0x01, MSB=0x02
type DMXController struct {
	port     *os.File
	mutex    sync.Mutex
	channels [512]byte

	// auto-send
	autoSendInterval time.Duration
	autoSendQuit     chan struct{}
	autoSending      bool
}

// Open opens a serial port to the MK2 device.
// device: on Linux/macOS use /dev/cu.usbserial-xxx or /dev/ttyUSB0; on Windows "COM3".
// It configures the port at 250000, 8 data bits, no parity, 2 stop bits.
func Open(device string) (*DMXController, error) {
	if device == "" {
		return nil, errors.New("device path is required")
	}

	// Open the serial port
	port, err := os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open serial port: %w", err)
	}

	// Configure the serial port using termios
	if err := configurePort(port); err != nil {
		port.Close()
		return nil, fmt.Errorf("configure port: %w", err)
	}

	return &DMXController{
		port: port,
	}, nil
}

// configurePort sets up the serial port with the correct settings for DMX
func configurePort(port *os.File) error {
	fd := int(port.Fd())

	// Get current termios settings
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return fmt.Errorf("get termios: %w", err)
	}

	// Set baud rate to 250000
	// On macOS, use IOSSIOSPEED for arbitrary baud rates
	const IOSSIOSPEED = 0x80045402 // _IOW('T', 2, speed_t)
	speed := uintptr(250000)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), IOSSIOSPEED, uintptr(unsafe.Pointer(&speed)))
	if errno != 0 {
		return fmt.Errorf("set baud rate: %w", errno)
	}

	// Configure for raw mode (manually since unix.Cfmakeraw is not available)
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8

	// 8 data bits, no parity, 2 stop bits
	termios.Cflag |= unix.CLOCAL | unix.CREAD
	termios.Cflag &^= unix.PARENB // No parity
	termios.Cflag |= unix.CSTOPB  // 2 stop bits
	termios.Cflag &^= unix.CSIZE
	termios.Cflag |= unix.CS8 // 8 data bits

	// No hardware flow control
	termios.Cflag &^= unix.CRTSCTS

	// Minimum characters and timeout
	termios.Cc[unix.VMIN] = 0
	termios.Cc[unix.VTIME] = 0

	// Set the attributes
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, termios); err != nil {
		return fmt.Errorf("set termios: %w", err)
	}

	// Clear DTR and RTS (important for FTDI devices)
	status, err := unix.IoctlGetInt(fd, unix.TIOCMGET)
	if err == nil {
		status &^= unix.TIOCM_DTR | unix.TIOCM_RTS
		if err := unix.IoctlSetInt(fd, unix.TIOCMSET, status); err != nil {
			fmt.Printf("Warning: could not clear DTR/RTS: %v\n", err)
		}
	}

	return nil
}

// Close closes the serial port and stops auto-send if running.
func (d *DMXController) Close() error {
	d.stopAutoSend()
	if d.port != nil {
		if err := d.port.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetChannel sets a single DMX channel (1..512).
func (d *DMXController) SetChannel(channel int, value byte) error {
	if channel < 1 || channel > 512 {
		return fmt.Errorf("channel %d out of range", channel)
	}
	d.mutex.Lock()
	d.channels[channel-1] = value
	d.mutex.Unlock()
	return nil
}

// SetChannels sets multiple channels at once. Keys are 1..512.
func (d *DMXController) SetChannels(values map[int]byte) error {
	d.mutex.Lock()
	for ch, val := range values {
		if ch >= 1 && ch <= 512 {
			d.channels[ch-1] = val
		}
	}
	d.mutex.Unlock()
	return nil
}

// SetAll sets all 512 channels from a slice (len must be 512).
func (d *DMXController) SetAll(data []byte) error {
	if len(data) != 512 {
		return fmt.Errorf("data length must be 512")
	}
	d.mutex.Lock()
	copy(d.channels[:], data)
	d.mutex.Unlock()
	return nil
}

// buildPacket builds the MK2 packet (header + start code + 512 bytes + footer).
func (d *DMXController) buildPacket() []byte {
	// Eurolite MK2 frame structure (518 bytes total):
	// Byte 0:   0x7E (Start of Message)
	// Byte 1:   0x06 (DMX Label)
	// Byte 2:   LSB of data length (513 = 0x01)
	// Byte 3:   MSB of data length (513 = 0x02)
	// Byte 4:   0x00 (DMX512 Start Code)
	// Bytes 5-516: 512 bytes of DMX data
	// Byte 517: 0xE7 (End of Message)
	packet := make([]byte, 518)
	
	packet[0] = 0x7E  // Start of Message
	packet[1] = 0x06  // DMX Label
	packet[2] = 0x01  // Length LSB (513 & 0xFF)
	packet[3] = 0x02  // Length MSB (513 >> 8)
	packet[4] = 0x00  // DMX512 Start Code

	d.mutex.Lock()
	copy(packet[5:517], d.channels[:])
	d.mutex.Unlock()

	packet[517] = 0xE7  // End of Message
	return packet
}

// SendDmx writes a single DMX packet to the MK2.
// Returns an error if write fails.
func (d *DMXController) SendDmx() error {
	if d.port == nil {
		return errors.New("port not open")
	}

	packet := d.buildPacket()
	
	// Debug: print first few bytes and last byte
	fmt.Printf("Sending packet: len=%d, header=[%02X %02X %02X %02X %02X], footer=%02X\n",
		len(packet), packet[0], packet[1], packet[2], packet[3], packet[4], packet[len(packet)-1])
	fmt.Printf("First 10 DMX channels: %v\n", packet[5:15])
	
	// write all bytes
	n, err := d.port.Write(packet)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	if n != len(packet) {
		return fmt.Errorf("short write: wrote %d/%d bytes", n, len(packet))
	}
	fmt.Printf("Successfully wrote %d bytes\n", n)
	// small pause to let device process
	time.Sleep(2 * time.Millisecond)
	return nil
}

// StartAutoSend starts continuously sending DMX packets at interval.
// MK2 holds last frame in its hardware; you generally only need to send on changes.
// Auto-send is useful for effects or frequent updates.
func (d *DMXController) StartAutoSend(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Millisecond
	}
	if d.autoSending {
		// already running; update interval
		d.autoSendInterval = interval
		return
	}
	d.autoSendInterval = interval
	d.autoSendQuit = make(chan struct{})
	d.autoSending = true

	go func() {
		t := time.NewTicker(d.autoSendInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_ = d.SendDmx() // ignore write error here; user can use SendDmx directly to check
			case <-d.autoSendQuit:
				return
			}
		}
	}()
}

// stopAutoSend stops the background auto-send if running.
func (d *DMXController) stopAutoSend() {
	if d.autoSending {
		close(d.autoSendQuit)
		d.autoSending = false
	}
}
