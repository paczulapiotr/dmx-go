package devices

// PST10Fixture returns the Fixture definition for the Eurolite LED PST-10 QCL Spot
// operating in 9-channel mode.
//
// Channel layout (9-channel mode):
//
//	CH1 (offset 0): Master Dimmer   0–255
//	CH2 (offset 1): Strobe          0=open, 1–255=strobe speed
//	CH3 (offset 2): Red             0–255
//	CH4 (offset 3): Green           0–255
//	CH5 (offset 4): Blue            0–255
//	CH6 (offset 5): White           0–255
//	CH7 (offset 6): Colour Macros   0=off
//	CH8 (offset 7): Auto Programs   0=off
//	CH9 (offset 8): Dimmer Curve    0=linear
//
// Supported actions:
//
//	"red", "green", "blue", "white", "off"   – 200 ms colour transition
//	"warm"                                   – soft warm white (R+W mix) transition
//	"rainbow"                                – infinite hue cycle
import (
	"time"

	"github.com/paczulapiotr/quiz-lab/lights/internal/effects"
)

const (
	pst10NumChannels = 9
	pst10Dimmer      = 0
	pst10Strobe      = 1
	pst10Red         = 2
	pst10Green       = 3
	pst10Blue        = 4
	pst10White       = 5
	pst10ColorMacro  = 6
	pst10AutoProgram = 7
	pst10DimmerCurve = 8

	pst10TransitionDuration = 200 * time.Millisecond
	// Rainbow updates its colour at this rate regardless of render fps.
	pst10RainbowPeriod = 33 * time.Millisecond
	pst10RainbowStep   = 1.0 // degrees per visual frame
)

func pst10Color(name string, r, g, b, w byte) *effects.Effect {
	return &effects.Effect{
		Name:        name,
		Type:        effects.EffectTypeTransition,
		NumChannels: pst10NumChannels,
		New: func(current []byte) effects.EffectTicker {
			target := make([]byte, pst10NumChannels)
			if r > 0 || g > 0 || b > 0 || w > 0 {
				target[pst10Dimmer] = 255
			}
			target[pst10Red] = r
			target[pst10Green] = g
			target[pst10Blue] = b
			target[pst10White] = w
			return effects.NewTransition(current, target, pst10TransitionDuration)
		},
	}
}

// pst10RainbowTicker advances a hue cycle on every visual frame (pst10RainbowPeriod).
// Render ticks faster than the period are skipped, keeping animation speed
// independent of the render interval.
type pst10RainbowTicker struct {
	hue      float64
	channels [pst10NumChannels]byte
	lastTick time.Time
}

func newPST10Rainbow() effects.EffectTicker {
	t := &pst10RainbowTicker{lastTick: time.Now()}
	t.channels[pst10Dimmer] = 255
	return t
}

func (r *pst10RainbowTicker) Tick(w effects.ChannelWriter) bool {
	if time.Since(r.lastTick) < pst10RainbowPeriod {
		return true // not yet time for the next visual frame
	}
	r.lastTick = r.lastTick.Add(pst10RainbowPeriod)

	rv, gv, bv := effects.HSVToRGB(r.hue, 1.0, 1.0)
	r.channels[pst10Red] = rv
	r.channels[pst10Green] = gv
	r.channels[pst10Blue] = bv
	w.SetValues(r.channels[:])

	r.hue += pst10RainbowStep
	if r.hue >= 360.0 {
		r.hue -= 360.0
	}
	return true
}

// PST10Fixture returns the fully configured Fixture for the Eurolite PST-10.
func PST10Fixture() *effects.Fixture {
	return &effects.Fixture{
		Name:        "Eurolite LED PST-10 QCL Spot (9ch)",
		NumChannels: pst10NumChannels,
		Actions: map[string]*effects.Effect{
			"red":   pst10Color("red", 255, 0, 0, 0),
			"green": pst10Color("green", 0, 255, 0, 0),
			"blue":  pst10Color("blue", 0, 0, 255, 0),
			"white": pst10Color("white", 0, 0, 0, 255),
			"warm":  pst10Color("warm", 200, 0, 0, 180),
			"off":   pst10Color("off", 0, 0, 0, 0),
			"rainbow": {
				Name:        "rainbow",
				Type:        effects.EffectTypeInfinite,
				NumChannels: pst10NumChannels,
				New:         func(_ []byte) effects.EffectTicker { return newPST10Rainbow() },
			},
		},
	}
}
