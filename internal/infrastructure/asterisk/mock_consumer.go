package asterisk

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// MockWSConsumer is a programmable ARI event source for tests.
// It implements domain.ARIEventConsumer and lets tests enqueue events
// which are then dispatched to the registered handler.
type MockWSConsumer struct {
	mu      sync.Mutex
	handle  EventHandler
	stopped chan struct{}
	queue   chan domain.ARIEvent
	started bool
}

// NewMockWSConsumer returns an unstarted mock consumer.
func NewMockWSConsumer() *MockWSConsumer {
	return &MockWSConsumer{
		queue:   make(chan domain.ARIEvent, 64),
		stopped: make(chan struct{}),
	}
}

// SetHandler registers the event sink. Must be called before Start.
func (m *MockWSConsumer) SetHandler(h EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handle = h
}

// Start blocks on the queue until ctx is cancelled.
func (m *MockWSConsumer) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = true
	m.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			close(m.stopped)
			return ctx.Err()
		case ev := <-m.queue:
			m.mu.Lock()
			h := m.handle
			m.mu.Unlock()
			if h != nil {
				h(ctx, ev)
			}
		}
	}
}

// Push enqueues a synthetic ARI event for tests.
func (m *MockWSConsumer) Push(ev domain.ARIEvent) {
	select {
	case m.queue <- ev:
	default:
	}
}

// PushStasisStart is a convenience helper.
func (m *MockWSConsumer) PushStasisStart(callID uuid.UUID, channelID string) {
	m.Push(domain.ARIEvent{
		Type:      "StasisStart",
		Timestamp: time.Now().UTC(),
		Channel: &domain.ARIChannel{
			ID:    channelID,
			State: "Up",
		},
		Variables: map[string]any{"CALL_ID": callID.String()},
	})
}

// PushChannelAnswered simulates the customer/agent answering.
func (m *MockWSConsumer) PushChannelAnswered(callID uuid.UUID, channelID string) {
	m.Push(domain.ARIEvent{
		Type:      "ChannelStateChange",
		Timestamp: time.Now().UTC(),
		Channel: &domain.ARIChannel{
			ID:    channelID,
			State: "Up",
		},
		Variables: map[string]any{"CALL_ID": callID.String()},
	})
}

// PushStasisEnd simulates hangup from any side.
func (m *MockWSConsumer) PushStasisEnd(callID uuid.UUID) {
	m.Push(domain.ARIEvent{
		Type:      "StasisEnd",
		Timestamp: time.Now().UTC(),
		Variables: map[string]any{"CALL_ID": callID.String()},
	})
}

var (
	_ domain.ARIEventConsumer = (*WSConsumer)(nil)
	_ domain.ARIEventConsumer = (*MockWSConsumer)(nil)
)
