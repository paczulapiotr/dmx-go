package effects

// EffectType indicates whether an effect runs once or loops indefinitely.
type EffectType string

const (
	EffectTypeTransition EffectType = "transition"
	EffectTypeInfinite   EffectType = "infinite"
)

// ChannelWriter is the interface effects use to update their channel buffer.
type ChannelWriter interface {
	SetValues(values []byte)
}

// EffectTicker is a stateful effect instance driven by the render loop.
// The render loop calls Tick on every frame.
// Transition effects return false when complete; infinite effects always return true.
type EffectTicker interface {
	Tick(w ChannelWriter) bool
}

// Effect is a descriptor and factory for a named DMX effect.
type Effect struct {
	Name        string
	Type        EffectType
	NumChannels int
	// New creates a fresh EffectTicker initialised with the fixture's current channel state.
	New func(current []byte) EffectTicker
}

// Fixture describes a DMX fixture model and its available actions.
type Fixture struct {
	Name        string
	NumChannels int
	Actions     map[string]*Effect
}

// DMXDevice is the interface implemented by dmx.USBController.
type DMXDevice interface {
	SendFrame(data []byte) error
}

// Infinite builds an Effect that runs forever, ignoring the current channel state.
// factory is called once when the effect starts; it must return a fresh EffectTicker.
func Infinite(name string, numChannels int, factory func() EffectTicker) *Effect {
	return &Effect{
		Name:        name,
		Type:        EffectTypeInfinite,
		NumChannels: numChannels,
		New:         func(_ []byte) EffectTicker { return factory() },
	}
}
