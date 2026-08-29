package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
)

type AuthUseCase struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
	guestRepo   domain.GuestRepository
}

func NewAuthUseCase(userRepo domain.UserRepository, sessionRepo domain.SessionRepository, guestRepo domain.GuestRepository) *AuthUseCase {
	return &AuthUseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		guestRepo:   guestRepo,
	}
}

// Login verifies staff credentials and creates a session token (valid for 7 days).
func (uc *AuthUseCase) Login(ctx context.Context, username, password string) (*domain.SessionUser, error) {
	user, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, errors.New("tên đăng nhập hoặc mật khẩu không chính xác")
	}

	if !security.VerifyPassword(password, user.Salt, user.PasswordHash) {
		return nil, errors.New("tên đăng nhập hoặc mật khẩu không chính xác")
	}

	token, err := security.GenerateToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = uc.sessionRepo.Create(ctx, token, user.Username, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &domain.SessionUser{
		Username: user.Username,
		FullName: user.FullName,
		Role:     user.Role,
		Token:    token,
	}, nil
}

// VerifyToken validates a Bearer token and returns the current user.
func (uc *AuthUseCase) VerifyToken(ctx context.Context, token string) (*domain.SessionUser, error) {
	if token == "" {
		return nil, errors.New("vui lòng đăng nhập để sử dụng tính năng này")
	}
	user, err := uc.sessionRepo.Verify(ctx, token)
	if err != nil || user == nil {
		return nil, errors.New("phiên đăng nhập đã hết hạn hoặc không hợp lệ")
	}
	return user, nil
}

// Logout removes the session token.
func (uc *AuthUseCase) Logout(ctx context.Context, token string) error {
	return uc.sessionRepo.Delete(ctx, token)
}

// RegisterGuest registers a customer with their name & optional phone number.
func (uc *AuthUseCase) RegisterGuest(ctx context.Context, displayName, phone string) (*domain.Guest, string, error) {
	if displayName == "" {
		displayName = "Khách hàng"
	}

	guestID := uuid.New()
	guest, err := uc.guestRepo.Create(ctx, guestID, displayName, phone)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create guest: %w", err)
	}

	// Generate a guest session token
	token, err := security.GenerateToken(24)
	if err != nil {
		return nil, "", err
	}

	return guest, token, nil
}
