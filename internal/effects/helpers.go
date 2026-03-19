package effects

import (
	"math"
	"time"
)

// transition is an EffectTicker that smoothly interpolates channel values
// from an initial state to a target state over a fixed duration.
// It is time-based, so the animation speed is independent of render fps.
type transition struct {
	from     []byte
	to       []byte
	buf      []byte
	start    time.Time
	duration time.Duration
}

// NewTransition creates a transition EffectTicker from current to target over duration.
func NewTransition(current, target []byte, duration time.Duration) EffectTicker {
	from := make([]byte, len(current))
	copy(from, current)
	to := make([]byte, len(target))
	copy(to, target)
	return &transition{
		from:     from,
		to:       to,
		buf:      make([]byte, len(from)),
		start:    time.Now(),
		duration: duration,
	}
}

func (t *transition) Tick(w ChannelWriter) bool {
	if t.duration <= 0 {
		w.SetValues(t.to)
		return false
	}

	ratio := float64(time.Since(t.start)) / float64(t.duration)
	if ratio >= 1.0 {
		w.SetValues(t.to)
		return false
	}

	for i := range t.buf {
		val := float64(t.from[i]) + ratio*(float64(t.to[i])-float64(t.from[i]))
		t.buf[i] = byte(math.Round(val))
	}
	w.SetValues(t.buf)
	return true
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
