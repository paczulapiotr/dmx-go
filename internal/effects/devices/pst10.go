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
//	"rainbow"                                – infinite hue cycle (~12 s per revolution)
import (
	"context"
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
	pst10RainbowTickRate    = 33 * time.Millisecond
	pst10RainbowHueStep     = 1.0
)

func pst10Color(name string, r, g, b, w byte) *effects.Effect {
	return &effects.Effect{
		Name:        name,
		NumChannels: pst10NumChannels,
		Run: func(ctx context.Context, cw effects.ChannelWriter, startAddr int, current []byte) error {
			target := make([]byte, pst10NumChannels)
			if r > 0 || g > 0 || b > 0 || w > 0 {
				target[pst10Dimmer] = 255
			}
			target[pst10Red] = r
			target[pst10Green] = g
			target[pst10Blue] = b
			target[pst10White] = w
			return effects.Transition(ctx, cw, startAddr, current, target, pst10TransitionDuration)
		},
	}
}

// PST10Fixture returns the fully configured Fixture for the Eurolite PST-10.
func PST10Fixture() *effects.Fixture {
	rainbow := &effects.Effect{
		Name:        "rainbow",
		NumChannels: pst10NumChannels,
		Run: func(ctx context.Context, cw effects.ChannelWriter, startAddr int, current []byte) error {
			hue := 0.0
			channels := make([]byte, pst10NumChannels) // reused every tick
			channels[pst10Dimmer] = 255

			ticker := time.NewTicker(pst10RainbowTickRate)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					r, g, b := effects.HSVToRGB(hue, 1.0, 1.0)
					channels[pst10Red] = r
					channels[pst10Green] = g
					channels[pst10Blue] = b
					cw.SetRange(startAddr, channels)

					hue += pst10RainbowHueStep
					if hue >= 360.0 {
						hue -= 360.0
					}
				}
			}
		},
	}

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
			"rainbow": rainbow,
		},
	}
}
