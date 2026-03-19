package effects

import "context"

// ChannelWriter is the interface that effects use to update their slot buffer.
type ChannelWriter interface {
	// SetValues overwrites the slot buffer with the given values.
	SetValues(values []byte)
	// GetValues returns a snapshot of the current slot buffer.
	GetValues() []byte
}

// Effect describes a single named DMX effect that can be applied to a fixture.
type Effect struct {
	// Name is the human-readable identifier used in log messages.
	Name string
	// NumChannels is the number of DMX channels this effect occupies.
	NumChannels int
	// Run executes the effect.
	// current contains the channel values that were active before this effect started,
	// allowing smooth interpolation from the previous state.
	// Effects must return once complete or when ctx is cancelled.
	Run func(ctx context.Context, w ChannelWriter, current []byte) error
}

// Fixture describes a supported DMX fixture model and its available actions.
type Fixture struct {
	// Name is the human-readable fixture model name.
	Name string
	// NumChannels is the total number of DMX channels the fixture occupies.
	NumChannels int
	// Actions maps action names (e.g. "red", "rainbow") to their Effect definitions.
	Actions map[string]*Effect
}

// DMXDevice is the interface implemented by dmx.USBController.
type DMXDevice interface {
	// SendFrame atomically writes a full 512-channel DMX universe to the device.
	// data must be exactly 512 bytes.
	SendFrame(data []byte) error
}
