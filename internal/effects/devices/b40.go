package devices

// B40Fixture returns the Fixture definition for the Eurolite LED B-40 Laser
// operating in 22-channel mode.
//
// Channel layout (22-channel mode):
//
//	CH1  (offset  0): Master Dimmer     0–255
//	CH2  (offset  1): Strobe            0=open, 1–255=strobe speed
//	CH3  (offset  2): Red               0–255
//	CH4  (offset  3): Green             0–255
//	CH5  (offset  4): Blue              0–255
//	CH6  (offset  5): White             0–255
//	CH7  (offset  6): Amber             0–255
//	CH8  (offset  7): UV                0–255
//	CH9  (offset  8): Pan               0–255
//	CH10 (offset  9): Pan Fine          0–255
//	CH11 (offset 10): Tilt              0–255
//	CH12 (offset 11): Tilt Fine         0–255
//	CH13 (offset 12): Pan/Tilt Speed    0=max, 255=min
//	CH14 (offset 13): Colour Wheel      0=open
//	CH15 (offset 14): Gobo Wheel        0=open
//	CH16 (offset 15): Gobo Rotation     0=no rotation
//	CH17 (offset 16): Prism             0=open
//	CH18 (offset 17): Prism Rotation    0=no rotation
//	CH19 (offset 18): Focus             0–255
//	CH20 (offset 19): Zoom              0–255
//	CH21 (offset 20): Auto Program      0=off
//	CH22 (offset 21): Reset             0=no reset
//
// Supported actions:
//
//	"red", "green", "blue", "white", "amber", "uv", "off" – 300 ms colour transition
//	"rainbow"                                              – infinite hue cycle
//	"strobe"                                               – infinite strobe effect
import (
	"time"

	"github.com/paczulapiotr/quiz-lab/lights/internal/effects"
)

const (
	b40NumChannels = 22
	b40Dimmer      = 0
	b40Strobe      = 1
	b40Red         = 2
	b40Green       = 3
	b40Blue        = 4
	b40White       = 5
	b40Amber       = 6
	b40UV          = 7
	b40Pan         = 8
	b40PanFine     = 9
	b40Tilt        = 10
	b40TiltFine    = 11
	b40PTSpeed     = 12

	b40TransitionDuration = 300 * time.Millisecond

	// Rainbow and strobe visual frame rates.
	b40RainbowPeriod = 33 * time.Millisecond
	b40RainbowStep   = 1.0 // degrees per visual frame
	b40StrobePeriod  = 80 * time.Millisecond
)

func b40Color(name string, r, g, b, w, amber, uv byte) *effects.Effect {
	return &effects.Effect{
		Name:        name,
		Type:        effects.EffectTypeTransition,
		NumChannels: b40NumChannels,
		New: func(current []byte) effects.EffectTicker {
			target := make([]byte, b40NumChannels)
			if r > 0 || g > 0 || b > 0 || w > 0 || amber > 0 || uv > 0 {
				target[b40Dimmer] = 255
			}
			target[b40Red] = r
			target[b40Green] = g
			target[b40Blue] = b
			target[b40White] = w
			target[b40Amber] = amber
			target[b40UV] = uv
			return effects.NewTransition(current, target, b40TransitionDuration)
		},
	}
}

// b40RainbowTicker cycles through all hues, skipping render ticks that arrive
// before the visual frame period has elapsed.
type b40RainbowTicker struct {
	hue      float64
	channels [b40NumChannels]byte
	lastTick time.Time
}

func newB40Rainbow() effects.EffectTicker {
	t := &b40RainbowTicker{lastTick: time.Now()}
	t.channels[b40Dimmer] = 255
	return t
}

func (r *b40RainbowTicker) Tick(w effects.ChannelWriter) bool {
	if time.Since(r.lastTick) < b40RainbowPeriod {
		return true
	}
	r.lastTick = r.lastTick.Add(b40RainbowPeriod)

	rv, gv, bv := effects.HSVToRGB(r.hue, 1.0, 1.0)
	r.channels[b40Red] = rv
	r.channels[b40Green] = gv
	r.channels[b40Blue] = bv
	w.SetValues(r.channels[:])

	r.hue += b40RainbowStep
	if r.hue >= 360.0 {
		r.hue -= 360.0
	}
	return true
}

// b40StrobeTicker alternates the dimmer between full and zero at b40StrobePeriod.
type b40StrobeTicker struct {
	channels [b40NumChannels]byte
	on       bool
	lastTick time.Time
}

func newB40Strobe() effects.EffectTicker {
	t := &b40StrobeTicker{lastTick: time.Now(), on: true}
	t.channels[b40Red] = 255
	t.channels[b40Green] = 255
	t.channels[b40Blue] = 255
	return t
}

func (s *b40StrobeTicker) Tick(w effects.ChannelWriter) bool {
	if time.Since(s.lastTick) < b40StrobePeriod {
		return true
	}
	s.lastTick = s.lastTick.Add(b40StrobePeriod)

	s.on = !s.on
	if s.on {
		s.channels[b40Dimmer] = 255
	} else {
		s.channels[b40Dimmer] = 0
	}
	w.SetValues(s.channels[:])
	return true
}

// B40Fixture returns the fully configured Fixture for the Eurolite LED B-40 Laser.
func B40Fixture() *effects.Fixture {
	return &effects.Fixture{
		Name:        "Eurolite LED B-40 Laser (22ch)",
		NumChannels: b40NumChannels,
		Actions: map[string]*effects.Effect{
			"red":    b40Color("red", 255, 0, 0, 0, 0, 0),
			"green":  b40Color("green", 0, 255, 0, 0, 0, 0),
			"blue":   b40Color("blue", 0, 0, 255, 0, 0, 0),
			"white":  b40Color("white", 0, 0, 0, 255, 0, 0),
			"amber":  b40Color("amber", 0, 0, 0, 0, 255, 0),
			"uv":     b40Color("uv", 0, 0, 0, 0, 0, 255),
			"off":    b40Color("off", 0, 0, 0, 0, 0, 0),
			"rainbow": {
				Name:        "rainbow",
				Type:        effects.EffectTypeInfinite,
				NumChannels: b40NumChannels,
				New:         func(_ []byte) effects.EffectTicker { return newB40Rainbow() },
			},
			"strobe": {
				Name:        "strobe",
				Type:        effects.EffectTypeInfinite,
				NumChannels: b40NumChannels,
				New:         func(_ []byte) effects.EffectTicker { return newB40Strobe() },
			},
		},
	}
}
