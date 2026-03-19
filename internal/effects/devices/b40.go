package devices

// B40Fixture returns the Fixture definition for the Eurolite LED B-40 Laser
// operating in 22-channel mode.
//
// Channel layout (22-channel mode):
//
//	CH1  (offset  0): Group 1 Red          0–255
//	CH2  (offset  1): Group 1 Green        0–255
//	CH3  (offset  2): Group 1 Blue         0–255
//	CH4  (offset  3): Group 1 White        0–255
//	CH5  (offset  4): Group 1 Strobe       0=off, 1–255=speed
//	CH6  (offset  5): Group 2 Red          0–255
//	CH7  (offset  6): Group 2 Green        0–255
//	CH8  (offset  7): Group 2 Blue         0–255
//	CH9  (offset  8): Group 2 White        0–255
//	CH10 (offset  9): Group 2 Strobe       0=off, 1–255=speed
//	CH11 (offset 10): Group 3 Red          0–255
//	CH12 (offset 11): Group 3 Green        0–255
//	CH13 (offset 12): Group 3 Blue         0–255
//	CH14 (offset 13): Group 3 White        0–255
//	CH15 (offset 14): Group 3 Strobe       0=off, 1–255=speed
//	CH16 (offset 15): Colour Macros        0=off
//	CH17 (offset 16): Master Dimmer        0–255
//	CH18 (offset 17): Laser                0–255
//	CH19 (offset 18): Laser Strobe         0=off, 1–255=speed
//	CH20 (offset 19): Auto Program         0=off
//	CH21 (offset 20): Auto Program Speed   0–255
//	CH22 (offset 21): Rotation             0–127=CW, 128=stop, 129–255=CCW
//
// Supported actions:
//
//	"red", "green", "blue", "white", "off" – 300 ms colour transition (all 3 groups)
//	"rainbow"                              – infinite hue cycle, all groups in sync (~30 fps)
//	"strobe"                               – infinite white strobe via per-group strobe channels
//	"default"                              – blue cycles through groups 1→2→3→1 with crossfade
import (
	"time"

	"github.com/paczulapiotr/quiz-lab/lights/internal/effects"
)

const (
	b40NumChannels = 22

	// Per-group channel offsets (group size = 5: R,G,B,W,Strobe)
	b40G1Red    = 0
	b40G1Green  = 1
	b40G1Blue   = 2
	b40G1White  = 3
	b40G1Strobe = 4

	b40G2Red    = 5
	b40G2Green  = 6
	b40G2Blue   = 7
	b40G2White  = 8
	b40G2Strobe = 9

	b40G3Red    = 10
	b40G3Green  = 11
	b40G3Blue   = 12
	b40G3White  = 13
	b40G3Strobe = 14

	// Global channels
	b40ColorMacros      = 15
	b40Dimmer           = 16
	b40Laser            = 17
	b40LaserStrobe      = 18
	b40AutoProgram      = 19
	b40AutoProgramSpeed = 20
	b40Rotation         = 21 // 0–127=CW, 128=stop, 129–255=CCW

	b40TransitionDuration = 300 * time.Millisecond
	b40RainbowPeriod      = 33 * time.Millisecond // ~30 visual fps
	b40RainbowStep        = 1.0                   // degrees per visual frame
	b40StrobePeriod       = 80 * time.Millisecond // ~12 flashes per second

	// "default" blue-cycle timings
	b40CycleHold  = 2500 * time.Millisecond // how long one group stays fully on
	b40CycleFade  = 2500 * time.Millisecond  // crossfade duration between groups
	b40CycleStep  = b40CycleHold + b40CycleFade
	b40CycleTotal = 3 * b40CycleStep

	// Dimmer levels used by effects
	b40ColorDimmer   byte = 255 // master dimmer for colour transition effects
	b40RainbowDimmer byte = 255 // master dimmer for rainbow
	b40DefaultDimmer byte = 120 // master dimmer for the blue-cycle default effect
)

// b40Color builds a transition effect that fades all three LED groups to the
// same RGBW values and sets the master dimmer to full.
func b40Color(name string, r, g, b, w byte) *effects.Effect {
	return &effects.Effect{
		Name:        name,
		Type:        effects.EffectTypeTransition,
		NumChannels: b40NumChannels,
		New: func(current []byte) effects.EffectTicker {
			target := [b40NumChannels]byte{}
			if r > 0 || g > 0 || b > 0 || w > 0 {
				target[b40Dimmer] = b40ColorDimmer
			}
			// Apply the same colour to all three groups.
			target[b40G1Red], target[b40G1Green], target[b40G1Blue], target[b40G1White] = r, g, b, w
			target[b40G2Red], target[b40G2Green], target[b40G2Blue], target[b40G2White] = r, g, b, w
			target[b40G3Red], target[b40G3Green], target[b40G3Blue], target[b40G3White] = r, g, b, w
			return effects.NewTransition(current, target[:], b40TransitionDuration)
		},
	}
}

// newB40Rainbow cycles all three groups through the same hue simultaneously.
func newB40Rainbow() effects.EffectTicker {
	channels := [b40NumChannels]byte{b40Dimmer: b40RainbowDimmer}
	var hue float64
	return effects.NewThrottledTicker(b40RainbowPeriod, func(w effects.ChannelWriter) bool {
		r, g, b := effects.HSVToRGB(hue, 1.0, 1.0)
		channels[b40G1Red], channels[b40G1Green], channels[b40G1Blue] = r, g, b
		channels[b40G2Red], channels[b40G2Green], channels[b40G2Blue] = r, g, b
		channels[b40G3Red], channels[b40G3Green], channels[b40G3Blue] = r, g, b
		w.SetValues(channels[:])
		hue += b40RainbowStep
		if hue >= 360 {
			hue -= 360
		}
		return true
	})
}

// newB40Strobe flashes all three groups white using the per-group strobe channels.
func newB40Strobe() effects.EffectTicker {
	channels := [b40NumChannels]byte{
		b40G1Red: 255, b40G1Green: 255, b40G1Blue: 255,
		b40G2Red: 255, b40G2Green: 255, b40G2Blue: 255,
		b40G3Red: 255, b40G3Green: 255, b40G3Blue: 255,
		b40Dimmer: 255,
	}
	on := true
	return effects.NewThrottledTicker(b40StrobePeriod, func(w effects.ChannelWriter) bool {
		on = !on
		speed := byte(0)
		if on {
			speed = 200
		}
		channels[b40G1Strobe] = speed
		channels[b40G2Strobe] = speed
		channels[b40G3Strobe] = speed
		w.SetValues(channels[:])
		return true
	})
}

// newB40BlueCycle creates an infinite effect that sweeps a single blue group
// across all three groups in sequence with a smooth crossfade between each.
//
// Timeline per step (one of three groups):
//
//	[0 … holdDuration)           active group at full blue, others off
//	[holdDuration … stepDuration) crossfade: active fades out, next fades in
func newB40BlueCycle() effects.EffectTicker {
	channels := [b40NumChannels]byte{b40Dimmer: b40DefaultDimmer}
	start := time.Now()

	// Blue channel offset for each group index (0, 1, 2)
	blueOf := [3]int{b40G1Blue, b40G2Blue, b40G3Blue}

	return effects.NewThrottledTicker(16*time.Millisecond, func(w effects.ChannelWriter) bool {
		elapsed := time.Since(start) % b40CycleTotal

		step := int(elapsed / b40CycleStep) // 0, 1, or 2
		stepElapsed := elapsed - time.Duration(step)*b40CycleStep
		next := (step + 1) % 3

		// Reset all three blue channels each tick.
		channels[b40G1Blue] = 0
		channels[b40G2Blue] = 0
		channels[b40G3Blue] = 0

		if stepElapsed < b40CycleHold {
			// Hold phase: current group fully on.
			channels[blueOf[step]] = 255
		} else {
			// Fade phase: crossfade from current to next group.
			ratio := float64(stepElapsed-b40CycleHold) / float64(b40CycleFade)
			if ratio > 1 {
				ratio = 1
			}
			channels[blueOf[step]] = byte(255 * (1 - ratio))
			channels[blueOf[next]] = byte(255 * ratio)
		}

		w.SetValues(channels[:])
		return true
	})
}

// B40Fixture returns the fully configured Fixture for the Eurolite LED B-40 Laser.
func B40Fixture() *effects.Fixture {
	return &effects.Fixture{
		Name:        "Eurolite LED B-40 Laser (22ch)",
		NumChannels: b40NumChannels,
		Actions: map[string]*effects.Effect{
			"red":     b40Color("red", 255, 0, 0, 0),
			"green":   b40Color("green", 0, 255, 0, 0),
			"blue":    b40Color("blue", 0, 0, 255, 0),
			"white":   b40Color("white", 0, 0, 0, 255),
			"warm":    b40Color("warm", 200, 60, 0, 80),
			"off":     b40Color("off", 0, 0, 0, 0),
			"rainbow": effects.Infinite("rainbow", b40NumChannels, newB40Rainbow),
			"strobe":  effects.Infinite("strobe", b40NumChannels, newB40Strobe),
			"default": effects.Infinite("default", b40NumChannels, newB40BlueCycle),
		},
	}
}
