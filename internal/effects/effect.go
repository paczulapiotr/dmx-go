package effects

import "context"

// ChannelWriter is the interface that effects use to write DMX channel values.
// All channel numbers are 1-based (DMX standard).
type ChannelWriter interface {
	// SetRange writes len(values) channels starting at startChannel.
	SetRange(startChannel int, values []byte)
	// GetRange returns a snapshot of the current tracked values for the given range.
	GetRange(startChannel int, numChannels int) []byte
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
	Run func(ctx context.Context, w ChannelWriter, startAddr int, current []byte) error
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
	// SetChannelRange writes len(values) channels starting at startChannel (1-based).
	SetChannelRange(startChannel int, values []byte) error
}
