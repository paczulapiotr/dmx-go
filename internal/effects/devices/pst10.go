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
//	"red", "green", "blue", "white", "warm", "off" – 200 ms colour transition
//	"rainbow"                                       – infinite hue cycle (~30 fps)
import (
	"time"

	"github.com/paczulapiotr/quiz-lab/lights/internal/effects"
)

const (
	pst10NumChannels = 9
	pst10Dimmer      = 0
	pst10Red         = 2
	pst10Green       = 3
	pst10Blue        = 4
	pst10White       = 5

	pst10TransitionDuration = 200 * time.Millisecond
	pst10RainbowPeriod      = 33 * time.Millisecond // ~30 visual fps
	pst10RainbowStep        = 1.0                   // degrees per visual frame
)

// pst10Color builds a transition effect that fades to the given RGBW values.
func pst10Color(name string, r, g, b, w byte) *effects.Effect {
	return &effects.Effect{
		Name:        name,
		Type:        effects.EffectTypeTransition,
		NumChannels: pst10NumChannels,
		New: func(current []byte) effects.EffectTicker {
			target := [pst10NumChannels]byte{}
			if r > 0 || g > 0 || b > 0 || w > 0 {
				target[pst10Dimmer] = 255
			}
			target[pst10Red] = r
			target[pst10Green] = g
			target[pst10Blue] = b
			target[pst10White] = w
			return effects.NewTransition(current, target[:], pst10TransitionDuration)
		},
	}
}

// newPST10Rainbow creates an infinite hue-cycling effect.
// All state is captured in the closure — no struct required.
func newPST10Rainbow() effects.EffectTicker {
	channels := [pst10NumChannels]byte{pst10Dimmer: 255}
	var hue float64
	return effects.NewThrottledTicker(pst10RainbowPeriod, func(w effects.ChannelWriter) bool {
		channels[pst10Red], channels[pst10Green], channels[pst10Blue] = effects.HSVToRGB(hue, 1.0, 1.0)
		w.SetValues(channels[:])
		hue += pst10RainbowStep
		if hue >= 360 {
			hue -= 360
		}
		return true
	})
}

// PST10Fixture returns the fully configured Fixture for the Eurolite PST-10.
func PST10Fixture() *effects.Fixture {
	return &effects.Fixture{
		Name:        "Eurolite LED PST-10 QCL Spot (9ch)",
		NumChannels: pst10NumChannels,
		Actions: map[string]*effects.Effect{
			"red":     pst10Color("red", 255, 0, 0, 0),
			"green":   pst10Color("green", 0, 255, 0, 0),
			"blue":    pst10Color("blue", 0, 0, 255, 0),
			"white":   pst10Color("white", 0, 0, 0, 255),
			"warm":    pst10Color("warm", 200, 0, 0, 180),
			"off":     pst10Color("off", 0, 0, 0, 0),
			"rainbow": effects.Infinite("rainbow", pst10NumChannels, newPST10Rainbow),
		},
	}
}
