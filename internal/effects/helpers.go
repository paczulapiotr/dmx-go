package effects

import (
	"context"
	"math"
	"time"
)

// Transition smoothly interpolates DMX channel values from `from` to `to` over `duration`.
// It writes intermediate values to the ChannelWriter on each step.
// Returns ctx.Err() if the context is cancelled mid-transition.
func Transition(ctx context.Context, w ChannelWriter, startAddr int, from, to []byte, duration time.Duration) error {
	const steps = 50

	if duration <= 0 {
		w.SetRange(startAddr, to)
		return nil
	}

	stepDuration := duration / steps
	current := make([]byte, len(from))

	ticker := time.NewTicker(stepDuration)
	defer ticker.Stop()

	for step := 1; step <= steps; step++ {
		t := float64(step) / float64(steps)
		for i := range current {
			val := float64(from[i]) + t*(float64(to[i])-float64(from[i]))
			current[i] = byte(math.Round(val))
		}
		w.SetRange(startAddr, current)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

// HSVToRGB converts an HSV colour (h: 0–360°, s: 0–1, v: 0–1) to RGB bytes (0–255 each).
func HSVToRGB(h, s, v float64) (r, g, b byte) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s = math.Max(0, math.Min(1, s))
	v = math.Max(0, math.Min(1, v))

	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}

	r = byte(math.Round((rf + m) * 255))
	g = byte(math.Round((gf + m) * 255))
	b = byte(math.Round((bf + m) * 255))
	return
}
