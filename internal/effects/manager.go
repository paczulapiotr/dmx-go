package effects

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// trackingWriter wraps a DMXDevice and keeps a local mirror of channel values
// so effects can read the current state when starting a transition.
type trackingWriter struct {
	device  DMXDevice
	current [512]byte
	mu      sync.RWMutex
}

func (w *trackingWriter) SetRange(startChannel int, values []byte) {
	w.mu.Lock()
	for i, v := range values {
		ch := startChannel - 1 + i
		if ch >= 0 && ch < 512 {
			w.current[ch] = v
		}
	}
	w.mu.Unlock()

	_ = w.device.SetChannelRange(startChannel, values)
}

func (w *trackingWriter) GetRange(startChannel int, numChannels int) []byte {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]byte, numChannels)
	for i := range result {
		ch := startChannel - 1 + i
		if ch >= 0 && ch < 512 {
			result[i] = w.current[ch]
		}
	}
	return result
}

// activeEffect tracks a goroutine running an effect for a specific DMX address.
type activeEffect struct {
	name   string
	cancel context.CancelFunc
	done   chan struct{}
}

// Manager orchestrates DMX effects across multiple fixtures.
// Multiple effects can run concurrently on different address ranges.
// Starting a new effect on an already-active address cancels the previous one first.
type Manager struct {
	writer   *trackingWriter
	active   map[int]*activeEffect // keyed by fixture start address
	mu       sync.Mutex
	logger   *log.Logger
	fixtures map[string]*Fixture
}

// NewManager creates a Manager that drives the given DMXDevice.
func NewManager(device DMXDevice, logger *log.Logger) *Manager {
	return &Manager{
		writer:   &trackingWriter{device: device},
		active:   make(map[int]*activeEffect),
		logger:   logger,
		fixtures: make(map[string]*Fixture),
	}
}

// RegisterFixture registers a fixture model under the given light-type key.
func (m *Manager) RegisterFixture(lightType string, fixture *Fixture) {
	m.fixtures[lightType] = fixture
}

// Apply looks up the fixture and action from the registry, cancels any currently
// running effect for the same start address, then launches the new effect.
func (m *Manager) Apply(startAddr int, lightType, action string) error {
	fixture, ok := m.fixtures[lightType]
	if !ok {
		return fmt.Errorf("unknown light type: %q", lightType)
	}

	effect, ok := fixture.Actions[action]
	if !ok {
		return fmt.Errorf("unknown action %q for light type %q", action, lightType)
	}

	current := m.writer.GetRange(startAddr, fixture.NumChannels)

	m.cancelExisting(startAddr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	m.mu.Lock()
	m.active[startAddr] = &activeEffect{
		name:   effect.Name,
		cancel: cancel,
		done:   done,
	}
	m.mu.Unlock()

	m.logger.Printf("[effect] started  addr=%-3d type=%-6s action=%-12s effect=%s",
		startAddr, lightType, action, effect.Name)

	go func() {
		defer close(done)
		defer cancel()
		defer func() {
			m.mu.Lock()
			if ae, ok := m.active[startAddr]; ok && ae.done == done {
				delete(m.active, startAddr)
			}
			m.mu.Unlock()
		}()

		if err := effect.Run(ctx, m.writer, startAddr, current); err != nil {
			if err != context.Canceled {
				m.logger.Printf("[effect] error     addr=%-3d effect=%s err=%v", startAddr, effect.Name, err)
			}
			m.logger.Printf("[effect] cancelled addr=%-3d effect=%s", startAddr, effect.Name)
			return
		}

		m.logger.Printf("[effect] finished  addr=%-3d effect=%s", startAddr, effect.Name)
	}()

	return nil
}

// cancelExisting stops the currently running effect for startAddr and waits for it to exit.
func (m *Manager) cancelExisting(startAddr int) {
	m.mu.Lock()
	ae, ok := m.active[startAddr]
	m.mu.Unlock()

	if ok {
		ae.cancel()
		<-ae.done
	}
}

// StopAll cancels every active effect and waits for all goroutines to exit.
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
}
