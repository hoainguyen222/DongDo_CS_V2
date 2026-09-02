package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	username = strings.ToLower(strings.TrimSpace(username))
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

// ListUsers returns all users in the system.
func (uc *AuthUseCase) ListUsers(ctx context.Context) ([]*domain.User, error) {
	return uc.userRepo.List(ctx)
}

// CreateUser hashes the password and saves a new user account.
func (uc *AuthUseCase) CreateUser(ctx context.Context, username, password, fullName string, role domain.UserRole) (*domain.User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	existing, _ := uc.userRepo.GetByUsername(ctx, username)
	if existing != nil {
		return nil, fmt.Errorf("Tên đăng nhập / Email [%s] đã tồn tại", username)
	}

	passwordHash, salt, err := security.HashPassword(password, "")
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	return uc.userRepo.Create(ctx, username, passwordHash, salt, fullName, role)
}

// DeleteUser deletes a user by username with Owner protection checks.
func (uc *AuthUseCase) DeleteUser(ctx context.Context, performerRole, username string) error {
	username = strings.ToLower(strings.TrimSpace(username))
	targetUser, _ := uc.userRepo.GetByUsername(ctx, username)
	if targetUser != nil {
		targetNormRole := strings.ToLower(string(targetUser.Role))
		if targetNormRole == "owner" || username == "admin" {
			return fmt.Errorf("Tài khoản Chủ sở hữu (Owner) / Admin tối cao [%s] được bảo vệ tuyệt đối, không thể bị xóa!", username)
		}
	}
	return uc.userRepo.Delete(ctx, username)
}

// UpdateUser updates user details (full_name, role, is_active, password) with Owner protection checks.
func (uc *AuthUseCase) UpdateUser(ctx context.Context, performerRole, targetUsername, fullName string, role domain.UserRole, isActive bool, newPassword string) (*domain.User, error) {
	targetUsername = strings.ToLower(strings.TrimSpace(targetUsername))
	targetUser, err := uc.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil || targetUser == nil {
		return nil, fmt.Errorf("Tài khoản [%s] không tồn tại trong hệ thống", targetUsername)
	}

	performerNormRole := strings.ToLower(strings.TrimSpace(performerRole))
	targetNormRole := strings.ToLower(string(targetUser.Role))

	// Owner protection rules
	if targetNormRole == "owner" && performerNormRole != "owner" {
		return nil, fmt.Errorf("Chỉ có tài khoản Chủ sở hữu (Owner) mới có quyền chỉnh sửa thông tin của tài khoản Owner [%s]!", targetUsername)
	}

	if targetNormRole == "owner" && strings.ToLower(string(role)) != "owner" {
		return nil, fmt.Errorf("Không thể giáng cấp vai trò của tài khoản Owner [%s]!", targetUsername)
	}

	var passwordHash, salt string
	if newPassword != "" {
		hash, s, err := security.HashPassword(newPassword, "")
		if err != nil {
			return nil, fmt.Errorf("Lỗi mã hóa mật khẩu mới: %w", err)
		}
		passwordHash = hash
		salt = s
	}

	return uc.userRepo.UpdateUser(ctx, targetUsername, fullName, role, isActive, passwordHash, salt)
}

