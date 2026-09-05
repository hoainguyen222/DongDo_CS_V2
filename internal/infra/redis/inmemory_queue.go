package redis

import (
	"context"
	"sync"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// InMemoryQueueManager (NoOp fallback / tests)
// ============================================================
//
// When Redis is not available we still need the voice usecase to
// work. This implementation is goroutine-safe and provides the same
// surface as the Redis-backed manager.
//
// It is intentionally simple — single-process only. Multi-instance
// deployments MUST use the Redis-backed manager.
type InMemoryQueueManager struct {
	mu sync.Mutex

	queue      []int64
	agentState map[string]domain.AgentStatus
	agentCall  map[string]int64
	idem       map[string]string
}

func NewInMemoryQueueManager() *InMemoryQueueManager {
	return &InMemoryQueueManager{
		queue:      make([]int64, 0, 16),
		agentState: make(map[string]domain.AgentStatus),
		agentCall:  make(map[string]int64),
		idem:       make(map[string]string),
	}
}

func (m *InMemoryQueueManager) EnqueueCall(_ context.Context, callID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, callID)
	for i, id := range m.queue {
		if id == callID {
			return i + 1, nil
		}
	}
	return len(m.queue), nil
}

func (m *InMemoryQueueManager) DequeueCall(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) == 0 {
		return 0, domain.ErrQueueEmpty
	}
	id := m.queue[0]
	m.queue = m.queue[1:]
	return id, nil
}

func (m *InMemoryQueueManager) QueuePosition(_ context.Context, callID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, id := range m.queue {
		if id == callID {
			return i + 1, nil
		}
	}
	return 0, nil
}

func (m *InMemoryQueueManager) AtomicReserveAgent(_ context.Context, callID int64, agentExt string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Validate agent state.
	if cur, ok := m.agentState[agentExt]; ok && cur != domain.AgentAvailable {
		return false, nil
	}
	// Validate queue head.
	if len(m.queue) == 0 || m.queue[0] != callID {
		return false, nil
	}
	m.queue = m.queue[1:]
	m.agentState[agentExt] = domain.AgentReserved
	m.agentCall[agentExt] = callID
	return true, nil
}

func (m *InMemoryQueueManager) ReleaseReservation(_ context.Context, agentExt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.agentState[agentExt]; ok && cur == domain.AgentReserved {
		m.agentState[agentExt] = domain.AgentAvailable
	}
	delete(m.agentCall, agentExt)
	return nil
}

func (m *InMemoryQueueManager) SetAgentState(_ context.Context, agentExt string, status domain.AgentStatus, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentState[agentExt] = status
	return nil
}

func (m *InMemoryQueueManager) GetAgentState(_ context.Context, agentExt string) (domain.AgentStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agentState[agentExt], nil
}

func (m *InMemoryQueueManager) SetAgentCurrentCall(_ context.Context, agentExt string, callID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if callID == 0 {
		delete(m.agentCall, agentExt)
	} else {
		m.agentCall[agentExt] = callID
	}
	return nil
}

func (m *InMemoryQueueManager) ReserveIdempotency(_ context.Context, key, payload string, _ time.Duration) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.idem[key]; ok {
		return existing, true, nil
	}
	m.idem[key] = payload
	return "", false, nil
}
