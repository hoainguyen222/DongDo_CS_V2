package usecase

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionInactive = errors.New("session inactive")
)

// SessionUseCase handles customer chat session lifecycle.
// Separate from staff JWT — customer uses opaque tokens stored in DB.
type SessionUseCase struct {
	sessionRepo    domain.ChatSessionRepository
	sessionExpiry  time.Duration // 30 days
	idleReset      time.Duration // 7 days
}

// NewSessionUseCase creates a new SessionUseCase.
func NewSessionUseCase(sessionRepo domain.ChatSessionRepository) *SessionUseCase {
	return &SessionUseCase{
		sessionRepo:   sessionRepo,
		sessionExpiry: domain.SessionExpiryDays * 24 * time.Hour,
		idleReset:     domain.SessionIdleResetDays * 24 * time.Hour,
	}
}

// EnsureSession creates a new session or resumes existing one.
// Returns (session, resumed, error) where resumed=true means the session already existed.
func (uc *SessionUseCase) EnsureSession(ctx context.Context, ip, userAgent string) (*domain.ChatSession, bool, error) {
	// Generate a unique session ID with collision check
	sessionID, err := uc.generateUniqueSessionID(ctx)
	if err != nil {
		return nil, false, err
	}

	now := time.Now()
	session := &domain.ChatSession{
		SessionID:    sessionID,
		DisplayName:  "Khách",
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(uc.sessionExpiry),
		IPAddress:    ip,
		UserAgent:    userAgent,
		IsActive:     true,
	}

	saved, err := uc.sessionRepo.Upsert(ctx, session)
	if err != nil {
		return nil, false, err
	}
	return saved, false, nil
}

// ValidateSession checks if session is valid (exists, active, not expired).
// On idle > 7 days, extends expiry to 30 days from now.
func (uc *SessionUseCase) ValidateSession(ctx context.Context, sessionID string) (*domain.ChatSession, error) {
	session, err := uc.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	if !session.IsActive {
		return nil, ErrSessionInactive
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	// Reset expiry if idle > idleReset (touch policy)
	if time.Since(session.LastActiveAt) > uc.idleReset {
		newExpiry := time.Now().Add(uc.sessionExpiry)
		if err := uc.sessionRepo.Touch(ctx, sessionID, newExpiry); err == nil {
			session.ExpiresAt = newExpiry
		}
	}

	return session, nil
}

// GetSession reads a session without validation.
func (uc *SessionUseCase) GetSession(ctx context.Context, sessionID string) (*domain.ChatSession, error) {
	return uc.sessionRepo.Get(ctx, sessionID)
}

// LogoutSession marks session as inactive.
func (uc *SessionUseCase) LogoutSession(ctx context.Context, sessionID string) error {
	return uc.sessionRepo.Deactivate(ctx, sessionID)
}

// UpdateDisplayName updates the session's display name.
func (uc *SessionUseCase) UpdateDisplayName(ctx context.Context, sessionID, displayName string) error {
	if displayName == "" {
		return errors.New("display name cannot be empty")
	}
	return uc.sessionRepo.UpdateDisplayName(ctx, sessionID, displayName)
}

// AttachGuest links a guest_id to the session (called after guest registration).
func (uc *SessionUseCase) AttachGuest(ctx context.Context, sessionID, guestID string) error {
	return uc.sessionRepo.UpdateGuestID(ctx, sessionID, guestID)
}

// CleanupExpired removes expired sessions from DB.
func (uc *SessionUseCase) CleanupExpired(ctx context.Context) (int64, error) {
	return uc.sessionRepo.CleanupExpired(ctx)
}

// generateUniqueSessionID creates a 32-hex-char session ID, retrying on collision.
func (uc *SessionUseCase) generateUniqueSessionID(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		b := make([]byte, 16) // 32 hex chars
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		sessionID := hex.EncodeToString(b)

		// Quick existence check (best-effort — race condition acceptable).
		// Repos may return sql.ErrNoRows, ErrSessionNotFound, or nil.
		existing, err := uc.sessionRepo.Get(ctx, sessionID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, ErrSessionNotFound) {
			return "", err
		}
		if existing == nil {
			return sessionID, nil
		}
	}
	return "", errors.New("failed to generate unique session ID after 3 attempts")
}