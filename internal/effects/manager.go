package effects

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// directWriter implements ChannelWriter by writing straight into a byte slice.
// Used only inside the render loop (single goroutine), so no mutex is needed.
type directWriter struct {
	buf []byte
}

func (w *directWriter) SetValues(values []byte) {
	copy(w.buf, values)
}

// activeEffect holds a running effect and the channel buffer it writes into.
type activeEffect struct {
	name      string
	startAddr int
	numChans  int
	// ticker is nil once a transition has completed (buf holds the final state).
	ticker EffectTicker
	// buf is the current channel state for this fixture.
	// Written by ticker.Tick; read by the render loop to compose the universe.
	buf []byte
}

// Manager drives all DMX effects from a single render loop.
//
// On every render tick the loop:
//  1. Calls Tick on every active effect (updates each effect's buf in place).
//  2. Composes a full 512-channel universe from all bufs.
//  3. Calls device.SendFrame once.
//
// No per-effect goroutines or per-slot mutexes are needed.
// m.mu is the only lock; it is held briefly during the tick phase and during Apply.
type Manager struct {
	device     DMXDevice
	active     map[int]*activeEffect
	mu         sync.Mutex
	logger     *log.Logger
	fixtures   map[string]*Fixture
	renderStop chan struct{}
	renderDone chan struct{}
}

// NewManager creates a Manager and starts the render loop at renderInterval.
func NewManager(device DMXDevice, logger *log.Logger, renderInterval time.Duration) *Manager {
	m := &Manager{
		device:     device,
		active:     make(map[int]*activeEffect),
		logger:     logger,
		fixtures:   make(map[string]*Fixture),
		renderStop: make(chan struct{}),
		renderDone: make(chan struct{}),
	}
	go m.renderLoop(renderInterval)
	return m
}

func (m *Manager) renderLoop(interval time.Duration) {
	defer close(m.renderDone)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var universe [512]byte

	for {
		select {
		case <-m.renderStop:
			return
		case <-ticker.C:
			universe = [512]byte{}

			m.mu.Lock()
			for _, ae := range m.active {
				if ae.ticker != nil {
					w := &directWriter{buf: ae.buf}
					if !ae.ticker.Tick(w) {
						// Transition finished: freeze at final state.
						ae.ticker = nil
						m.logger.Printf("[effect] finished  addr=%-3d effect=%s", ae.startAddr, ae.name)
					}
				}
				start := ae.startAddr - 1
				copy(universe[start:start+ae.numChans], ae.buf)
			}
			m.mu.Unlock()

			if err := m.device.SendFrame(universe[:]); err != nil {
				m.logger.Printf("[render] send error: %v", err)
			}
		}
	}
}

// RegisterFixture registers a fixture model under the given light-type key.
func (m *Manager) RegisterFixture(lightType string, fixture *Fixture) {
	m.fixtures[lightType] = fixture
}

// Apply starts a new effect at startAddr, replacing any currently active one.
func (m *Manager) Apply(startAddr int, lightType, action string) error {
	fixture, ok := m.fixtures[lightType]
	if !ok {
		return fmt.Errorf("unknown light type: %q", lightType)
	}

	effect, ok := fixture.Actions[action]
	if !ok {
		return fmt.Errorf("unknown action %q for light type %q", action, lightType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Snapshot current buf so the new effect can transition from the live state.
	current := make([]byte, fixture.NumChannels)
	if prev, exists := m.active[startAddr]; exists {
		copy(current, prev.buf)
		m.logger.Printf("[effect] replaced  addr=%-3d effect=%s", startAddr, prev.name)
	}

	buf := make([]byte, fixture.NumChannels)
	copy(buf, current)

	m.active[startAddr] = &activeEffect{
		name:      effect.Name,
		startAddr: startAddr,
		numChans:  fixture.NumChannels,
		ticker:    effect.New(current),
		buf:       buf,
	}

	m.logger.Printf("[effect] started   addr=%-3d type=%-6s action=%-12s effect=%s (%s)",
		startAddr, lightType, action, effect.Name, effect.Type)

	return nil
}

// StopAll removes all active effects and stops the render loop.
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.active = make(map[int]*activeEffect)
	m.mu.Unlock()

	close(m.renderStop)
	<-m.renderDone
}
