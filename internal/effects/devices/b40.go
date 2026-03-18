package devices

// B40Fixture returns the Fixture definition for the Eurolite LED B-40 laser
// operating in 22-channel mode.
//
// Channel layout (22-channel mode):
//
//	CH1  (offset  0): Master Dimmer      0–255
//	CH2  (offset  1): Red                0–255
//	CH3  (offset  2): Green              0–255
//	CH4  (offset  3): Blue               0–255
//	CH5  (offset  4): White/Amber        0–255
//	CH6  (offset  5): UV/Purple          0–255
//	CH7  (offset  6): Strobe             0=open, 1–255=speed
//	CH8  (offset  7): Pan                0–255
//	CH9  (offset  8): Pan Fine           0–255
//	CH10 (offset  9): Tilt               0–255
//	CH11 (offset 10): Tilt Fine          0–255
//	CH12 (offset 11): Pan/Tilt Speed     0=max speed, 255=slowest
//	CH13 (offset 12): Colour Wheel       0=open
//	CH14 (offset 13): Gobo Wheel         0=open
//	CH15 (offset 14): Gobo Rotation      0=no rotation
//	CH16 (offset 15): Prism              0=no prism
//	CH17 (offset 16): Prism Rotation     0=no rotation
//	CH18 (offset 17): Focus              0–255
//	CH19 (offset 18): Zoom               0–255
//	CH20 (offset 19): Iris               0=open, 255=closed
//	CH21 (offset 20): Effects/Macros     0=off
//	CH22 (offset 21): Reset/Programs     0=off, 255=reset
//
// Supported actions:
//
//	"red", "green", "blue", "white", "off"   – 200 ms colour transition
//	"warm"                                   – warm white blend transition
//	"rainbow"                                – infinite hue cycle (~12 s per revolution)
//	"strobe"                                 – white strobe (infinite, 50 % speed)
import (
	"context"
	"time"

	"github.com/paczulapiotr/quiz-lab/lights/internal/effects"
)

const (
	b40NumChannels = 22
	b40Dimmer      = 0
	b40Red         = 1
	b40Green       = 2
	b40Blue        = 3
	b40White       = 4
	b40UV          = 5
	b40Strobe      = 6
	b40Pan         = 7
	b40PanFine     = 8
	b40Tilt        = 9
	b40TiltFine    = 10
	b40PTSpeed     = 11
	b40ColorWheel  = 12
	b40GoboWheel   = 13
	b40GoboRot     = 14
	b40Prism       = 15
	b40PrismRot    = 16
	b40Focus       = 17
	b40Zoom        = 18
	b40Iris        = 19
	b40Effects     = 20
	b40Reset       = 21

	b40TransitionDuration = 200 * time.Millisecond
	b40RainbowTickRate    = 33 * time.Millisecond
	b40RainbowHueStep     = 1.0
)

// b40Color returns a transition Effect that fades to the given RGBW values.
// Channels beyond colour (pan, tilt, zoom, etc.) are left at zero so the
// fixture stays at its default mechanical position.
func b40Color(name string, rv, gv, bv, wv byte) *effects.Effect {
	return &effects.Effect{
		Name:        name,
		NumChannels: b40NumChannels,
		Run: func(ctx context.Context, cw effects.ChannelWriter, startAddr int, current []byte) error {
			target := make([]byte, b40NumChannels)
			if rv > 0 || gv > 0 || bv > 0 || wv > 0 {
				target[b40Dimmer] = 255
			}
			target[b40Red] = rv
			target[b40Green] = gv
			target[b40Blue] = bv
			target[b40White] = wv
			return effects.Transition(ctx, cw, startAddr, current, target, b40TransitionDuration)
		},
	}
}

// B40Fixture returns the fully configured Fixture for the Eurolite B-40 laser.
func B40Fixture() *effects.Fixture {
	rainbow := &effects.Effect{
		Name:        "rainbow",
		NumChannels: b40NumChannels,
		Run: func(ctx context.Context, cw effects.ChannelWriter, startAddr int, current []byte) error {
			hue := 0.0
			ticker := time.NewTicker(b40RainbowTickRate)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					r, g, b := effects.HSVToRGB(hue, 1.0, 1.0)
					channels := make([]byte, b40NumChannels)
					channels[b40Dimmer] = 255
					channels[b40Red] = r
					channels[b40Green] = g
					channels[b40Blue] = b
					cw.SetRange(startAddr, channels)

					hue += b40RainbowHueStep
					if hue >= 360.0 {
						hue -= 360.0
					}
				}
			}
		},
	}

	strobe := &effects.Effect{
		Name:        "strobe",
		NumChannels: b40NumChannels,
		Run: func(ctx context.Context, cw effects.ChannelWriter, startAddr int, current []byte) error {
			channels := make([]byte, b40NumChannels)
			channels[b40Dimmer] = 255
			channels[b40White] = 255
			channels[b40Strobe] = 128 // ~50 % strobe speed
			cw.SetRange(startAddr, channels)
			<-ctx.Done()
			return nil
		},
	}

	return &effects.Fixture{
		Name:        "Eurolite LED B-40 Laser (22ch)",
		NumChannels: b40NumChannels,
		Actions: map[string]*effects.Effect{
			"red":     b40Color("red", 255, 0, 0, 0),
			"green":   b40Color("green", 0, 255, 0, 0),
			"blue":    b40Color("blue", 0, 0, 255, 0),
			"white":   b40Color("white", 0, 0, 0, 255),
			"warm":    b40Color("warm", 220, 60, 0, 200),
			"off":     b40Color("off", 0, 0, 0, 0),
			"rainbow": rainbow,
			"strobe":  strobe,
		},
	}
}
