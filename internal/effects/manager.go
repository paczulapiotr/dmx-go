package effects

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// slotBuffer is a small byte buffer owned by exactly one effect goroutine.
// The effect goroutine writes to it via slotWriter; the render loop reads it.
type slotBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func newSlotBuffer(numChannels int) *slotBuffer {
	return &slotBuffer{buf: make([]byte, numChannels)}
}

// slotWriter implements ChannelWriter for an effect goroutine.
// All writes go to the local slot buffer only — the render loop handles I/O.
type slotWriter struct {
	slot *slotBuffer
}

func (w *slotWriter) SetValues(values []byte) {
	w.slot.mu.Lock()
	copy(w.slot.buf, values)
	w.slot.mu.Unlock()
}

func (w *slotWriter) GetValues() []byte {
	w.slot.mu.Lock()
	out := make([]byte, len(w.slot.buf))
	copy(out, w.slot.buf)
	w.slot.mu.Unlock()
	return out
}

// activeEffect holds everything for a running effect goroutine.
type activeEffect struct {
	name      string
	startAddr int
	numChans  int
	slot      *slotBuffer
	cancel    context.CancelFunc
	done      chan struct{}
}

// Manager orchestrates DMX effects across multiple fixtures.
//
// Architecture:
//   - Each active effect runs in its own goroutine and writes only to its
//     isolated slotBuffer — no shared state, no contention between effects.
//   - A single render loop goroutine ticks at renderInterval, reads every
//     slot buffer, composes a complete 512-channel universe, and calls
//     device.SendFrame once per tick. This is the only goroutine that ever
//     touches the DMX device.
type Manager struct {
	device     DMXDevice
	active     map[int]*activeEffect // keyed by fixture start address
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

// renderLoop is the single goroutine that owns all DMX device writes.
// On every tick it composes the full 512-channel universe from the active
// slot buffers and sends it to the device.
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
			universe = [512]byte{} // zero = blackout for any inactive channels

			m.mu.Lock()
			for _, ae := range m.active {
				ae.slot.mu.Lock()
				start := ae.startAddr - 1
				copy(universe[start:start+ae.numChans], ae.slot.buf)
				ae.slot.mu.Unlock()
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

// Apply cancels any running effect at startAddr, then starts the new one.
// The current slot state is passed to the new effect as its starting point,
// allowing smooth transitions from whatever was playing before.
func (m *Manager) Apply(startAddr int, lightType, action string) error {
	fixture, ok := m.fixtures[lightType]
	if !ok {
		return fmt.Errorf("unknown light type: %q", lightType)
	}

	effect, ok := fixture.Actions[action]
	if !ok {
		return fmt.Errorf("unknown action %q for light type %q", action, lightType)
	}

	// Snapshot the current slot state before cancelling the old effect,
	// so the new transition can start from where the previous one left off.
	current := m.currentSlotSnapshot(startAddr, fixture.NumChannels)

	m.cancelExisting(startAddr)

	slot := newSlotBuffer(fixture.NumChannels)
	// Pre-fill the slot with the last known state so the render loop never
	// sees a black flash between cancellation and the first write.
	slot.mu.Lock()
	copy(slot.buf, current)
	slot.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	m.mu.Lock()
	m.active[startAddr] = &activeEffect{
		name:      effect.Name,
		startAddr: startAddr,
		numChans:  fixture.NumChannels,
		slot:      slot,
		cancel:    cancel,
		done:      done,
	}
	m.mu.Unlock()

	m.logger.Printf("[effect] started  addr=%-3d type=%-6s action=%-12s effect=%s",
		startAddr, lightType, action, effect.Name)

	go func() {
		defer close(done)
		defer cancel()

		w := &slotWriter{slot: slot}
		if err := effect.Run(ctx, w, current); err != nil {
			if err != context.Canceled {
				m.logger.Printf("[effect] error     addr=%-3d effect=%s err=%v", startAddr, effect.Name, err)
			}
			m.logger.Printf("[effect] cancelled addr=%-3d effect=%s", startAddr, effect.Name)
			return
		}

		// Transition finished: keep the slot in m.active so the render loop
		// continues outputting the final colour. The entry is only removed
		// when a new action explicitly replaces it via cancelExisting.
		m.logger.Printf("[effect] finished  addr=%-3d effect=%s", startAddr, effect.Name)
	}()

	return nil
}

// currentSlotSnapshot returns a copy of the current slot buffer for startAddr,
// or a zeroed slice if no effect is running there.
func (m *Manager) currentSlotSnapshot(startAddr, numChannels int) []byte {
	m.mu.Lock()
	ae, ok := m.active[startAddr]
	m.mu.Unlock()

	if !ok {
		return make([]byte, numChannels)
	}
	return (&slotWriter{slot: ae.slot}).GetValues()
}

// cancelExisting stops any effect at startAddr, waits for its goroutine to exit,
// and removes the entry from m.active.
func (m *Manager) cancelExisting(startAddr int) {
	m.mu.Lock()
	ae, ok := m.active[startAddr]
	if ok {
		delete(m.active, startAddr)
	}
	m.mu.Unlock()

	if ok {
		ae.cancel()
		<-ae.done
	}
}

// StopAll cancels every active effect, waits for all goroutines to exit,
// then stops the render loop. Call this once on shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	addrs := make([]int, 0, len(m.active))
	for addr := range m.active {
		addrs = append(addrs, addr)
	}
	m.mu.Unlock()

	for _, addr := range addrs {
		m.cancelExisting(addr)
	}

	close(m.renderStop)
	<-m.renderDone
}
