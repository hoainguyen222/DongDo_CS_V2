package usecase

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ─── in-memory mock for ChatSessionRepository ────────────────────────

type mockSessionRepo struct {
	mu       sync.Mutex
	sessions map[string]*domain.ChatSession
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{sessions: make(map[string]*domain.ChatSession)}
}

func (r *mockSessionRepo) Get(ctx context.Context, sessionID string) (*domain.ChatSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[sessionID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *s
	return &cp, nil
}

func (r *mockSessionRepo) Upsert(ctx context.Context, session *domain.ChatSession) (*domain.ChatSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.sessions[session.SessionID]
	if ok {
		existing.LastActiveAt = time.Now()
		if session.ExpiresAt.After(existing.ExpiresAt) {
			existing.ExpiresAt = session.ExpiresAt
		}
		existing.IPAddress = session.IPAddress
		existing.UserAgent = session.UserAgent
		existing.IsActive = true
		cp := *existing
		return &cp, nil
	}
	cp := *session
	r.sessions[session.SessionID] = &cp
	cp2 := *session
	return &cp2, nil
}

func (r *mockSessionRepo) Touch(ctx context.Context, sessionID string, newExpiry time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[sessionID]; ok {
		if newExpiry.After(s.ExpiresAt) {
			s.ExpiresAt = newExpiry
		}
		s.LastActiveAt = time.Now()
	}
	return nil
}

func (r *mockSessionRepo) Deactivate(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[sessionID]; ok {
		s.IsActive = false
	}
	return nil
}

func (r *mockSessionRepo) UpdateGuestID(ctx context.Context, sessionID, guestID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[sessionID]; ok {
		s.GuestID = &guestID
	}
	return nil
}

func (r *mockSessionRepo) UpdateDisplayName(ctx context.Context, sessionID, displayName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[sessionID]; ok {
		s.DisplayName = displayName
	}
	return nil
}

func (r *mockSessionRepo) CleanupExpired(ctx context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for id, s := range r.sessions {
		if !s.IsActive || time.Now().After(s.ExpiresAt) {
			delete(r.sessions, id)
			count++
		}
	}
	return count, nil
}

// ─── tests ───────────────────────────────────────────────────────────

func TestSessionUseCase_EnsureSession_New(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, resumed, err := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resumed {
		t.Error("expected resumed=false for new session")
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if len(session.SessionID) != 32 {
		t.Errorf("expected sessionID length 32, got %d", len(session.SessionID))
	}
	if session.IsActive != true {
		t.Error("expected IsActive=true")
	}
	if session.DisplayName != "Khách" {
		t.Errorf("expected default displayName 'Khách', got %q", session.DisplayName)
	}
	if session.IPAddress != "127.0.0.1" {
		t.Errorf("expected IP '127.0.0.1', got %q", session.IPAddress)
	}
	if !session.ExpiresAt.After(time.Now()) {
		t.Error("expected ExpiresAt in future")
	}
}

func TestSessionUseCase_ValidateSession_Valid(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	validated, err := uc.ValidateSession(context.Background(), session.SessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validated.SessionID != session.SessionID {
		t.Errorf("expected same sessionID, got %q vs %q", validated.SessionID, session.SessionID)
	}
}

func TestSessionUseCase_ValidateSession_NotFound(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	_, err := uc.ValidateSession(context.Background(), "nonexistent")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionUseCase_ValidateSession_Expired(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	// Force expiry
	repo.mu.Lock()
	repo.sessions[session.SessionID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	repo.mu.Unlock()

	_, err := uc.ValidateSession(context.Background(), session.SessionID)
	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestSessionUseCase_ValidateSession_Inactive(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	// Mark inactive
	_ = repo.Deactivate(context.Background(), session.SessionID)

	_, err := uc.ValidateSession(context.Background(), session.SessionID)
	if !errors.Is(err, ErrSessionInactive) {
		t.Errorf("expected ErrSessionInactive, got %v", err)
	}
}

func TestSessionUseCase_LogoutSession(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	if err := uc.LogoutSession(context.Background(), session.SessionID); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	_, err := uc.ValidateSession(context.Background(), session.SessionID)
	if !errors.Is(err, ErrSessionInactive) {
		t.Errorf("expected ErrSessionInactive after logout, got %v", err)
	}
}

func TestSessionUseCase_UpdateDisplayName(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	if err := uc.UpdateDisplayName(context.Background(), session.SessionID, "Anh Nam"); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, _ := uc.GetSession(context.Background(), session.SessionID)
	if got.DisplayName != "Anh Nam" {
		t.Errorf("expected 'Anh Nam', got %q", got.DisplayName)
	}
}

func TestSessionUseCase_UpdateDisplayName_Empty(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	err := uc.UpdateDisplayName(context.Background(), session.SessionID, "")
	if err == nil {
		t.Error("expected error for empty display name")
	}
}

func TestSessionUseCase_AttachGuest(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	if err := uc.AttachGuest(context.Background(), session.SessionID, "guest-uuid-123"); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	got, _ := uc.GetSession(context.Background(), session.SessionID)
	if got.GuestID == nil || *got.GuestID != "guest-uuid-123" {
		t.Errorf("expected guest_id 'guest-uuid-123', got %v", got.GuestID)
	}
}

func TestSessionUseCase_CleanupExpired(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	// Create 2 sessions
	s1, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")
	_, _, _ = uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	// Force s1 expiry
	repo.mu.Lock()
	repo.sessions[s1.SessionID].ExpiresAt = time.Now().Add(-48 * time.Hour)
	repo.mu.Unlock()

	// Mock repo only removes sessions whose ExpiresAt < now (no 1-day grace)
	// Add a manually-expired second session
	repo.mu.Lock()
	repo.sessions[s1.SessionID].ExpiresAt = time.Now().Add(-48 * time.Hour)
	repo.mu.Unlock()

	count, err := uc.CleanupExpired(context.Background())
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 expired session removed, got %d", count)
	}
}

func TestSessionUseCase_SessionID_Unique(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		session, _, err := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if seen[session.SessionID] {
			t.Errorf("duplicate sessionID %q", session.SessionID)
		}
		seen[session.SessionID] = true
	}
}

func TestSessionUseCase_IdleReset_ExtendsExpiry(t *testing.T) {
	repo := newMockSessionRepo()
	uc := NewSessionUseCase(repo)

	session, _, _ := uc.EnsureSession(context.Background(), "127.0.0.1", "test-ua")

	// Force last_active_at to be > 7 days ago (idle)
	repo.mu.Lock()
	oldLast := time.Now().Add(-8 * 24 * time.Hour)
	repo.sessions[session.SessionID].LastActiveAt = oldLast
	oldExpires := repo.sessions[session.SessionID].ExpiresAt
	repo.mu.Unlock()

	// Validate — should extend expiry
	validated, err := uc.ValidateSession(context.Background(), session.SessionID)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if !validated.ExpiresAt.After(oldExpires) {
		t.Errorf("expected expiry extended, got %v (was %v)", validated.ExpiresAt, oldExpires)
	}
}