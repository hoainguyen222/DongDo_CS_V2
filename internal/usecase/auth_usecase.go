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
	userRepo     domain.UserRepository
	sessionRepo  domain.SessionRepository
	guestRepo    domain.GuestRepository
	jwtManager   *security.StaffJWTManager
	revokedRepo  domain.RevokedTokenRepository
}

func NewAuthUseCase(
	userRepo domain.UserRepository,
	sessionRepo domain.SessionRepository,
	guestRepo domain.GuestRepository,
	jwtManager *security.StaffJWTManager,
	revokedRepo domain.RevokedTokenRepository,
) *AuthUseCase {
	return &AuthUseCase{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		guestRepo:   guestRepo,
		jwtManager:  jwtManager,
		revokedRepo: revokedRepo,
	}
}

// StaffLogin verifies staff credentials and returns JWT access + refresh tokens.
// Password: bcrypt verify → generate access JWT (15m) + refresh JWT (7d).
func (uc *AuthUseCase) StaffLogin(ctx context.Context, username, password string) (accessToken, refreshToken string, user *domain.SessionUser, err error) {
	if uc.jwtManager == nil {
		return "", "", nil, errors.New("jwt manager not configured")
	}

	userRecord, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get user: %w", err)
	}
	if userRecord == nil || !userRecord.IsActive {
		return "", "", nil, errors.New("tên đăng nhập hoặc mật khẩu không chính xác")
	}

	if !security.VerifyPassword(password, userRecord.Salt, userRecord.PasswordHash) {
		return "", "", nil, errors.New("tên đăng nhập hoặc mật khẩu không chính xác")
	}

	accessToken, _, err = uc.jwtManager.GenerateAccessToken(username, string(userRecord.Role))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, _, err = uc.jwtManager.GenerateRefreshToken(username, string(userRecord.Role))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, &domain.SessionUser{
		Username: userRecord.Username,
		FullName: userRecord.FullName,
		Role:     userRecord.Role,
	}, nil
}

// VerifyStaffToken verifies a JWT access token.
// Checks: signature valid, not expired, jti NOT in revoked_tokens.
func (uc *AuthUseCase) VerifyStaffToken(ctx context.Context, tokenString string) (*domain.SessionUser, error) {
	if tokenString == "" {
		return nil, errors.New("vui lòng đăng nhập để sử dụng tính năng này")
	}

	claims, err := uc.jwtManager.VerifyToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired token: %w", err)
	}

	// Check revocation list
	if uc.revokedRepo != nil {
		revoked, err := uc.revokedRepo.IsRevoked(ctx, claims.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to check revocation: %w", err)
		}
		if revoked {
			return nil, errors.New("token has been revoked")
		}
	}

	return &domain.SessionUser{
		Username: claims.Subject,
		Role:     domain.UserRole(claims.Role),
	}, nil
}

// RefreshStaffToken verifies a refresh token and issues a new access token.
// Only generates access token — refresh token itself is NOT rotated.
func (uc *AuthUseCase) RefreshStaffToken(ctx context.Context, refreshTokenString string) (newAccessToken string, err error) {
	if uc.jwtManager == nil {
		return "", errors.New("jwt manager not configured")
	}

	claims, err := uc.jwtManager.VerifyToken(refreshTokenString)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	if claims.TokenType != security.TokenTypeRefresh {
		return "", errors.New("token type mismatch: expected refresh token")
	}

	// Check revocation
	if uc.revokedRepo != nil {
		revoked, err := uc.revokedRepo.IsRevoked(ctx, claims.ID)
		if err != nil {
			return "", fmt.Errorf("failed to check revocation: %w", err)
		}
		if revoked {
			return "", errors.New("refresh token has been revoked")
		}
	}

	newAccessToken, _, err = uc.jwtManager.GenerateAccessToken(claims.Subject, claims.Role)
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	return newAccessToken, nil
}

// RevokeStaffToken adds the token's jti to the revocation list.
func (uc *AuthUseCase) RevokeStaffToken(ctx context.Context, tokenString string, reason string) error {
	if uc.jwtManager == nil || uc.revokedRepo == nil {
		return errors.New("jwt manager or revoked repo not configured")
	}

	claims, err := uc.jwtManager.VerifyToken(tokenString)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	return uc.revokedRepo.Revoke(ctx, claims.ID, claims.Subject, claims.ExpiresAt.Time, reason)
}

// CleanupExpiredTokens removes revoked token entries that are past their expiry.
func (uc *AuthUseCase) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	if uc.revokedRepo == nil {
		return 0, nil
	}
	return uc.revokedRepo.Cleanup(ctx)
}

// ─── Legacy session-based auth (used by customers / guest, Task 02b) ─────────

// Login verifies staff credentials and creates a session token (valid for 7 days).
// DEPRECATED: Use StaffLogin for staff. This is kept for backward compatibility.
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
