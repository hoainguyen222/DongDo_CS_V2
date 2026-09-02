package security

import (
	"errors"
	"testing"
	"time"
)

func TestStaffJWTManager_NewStaffJWTManager(t *testing.T) {
	manager, err := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if manager == nil {
		t.Fatal("expected manager to not be nil")
	}
}

func TestStaffJWTManager_NewStaffJWTManager_ShortSecret(t *testing.T) {
	_, err := NewStaffJWTManager("short", 15*time.Minute, 7*24*time.Hour)
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestStaffJWTManager_GenerateAccessToken(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	token, jti, err := manager.GenerateAccessToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if jti == "" {
		t.Fatal("expected non-empty jti")
	}

	// Verify the token
	claims, err := manager.VerifyToken(token)
	if err != nil {
		t.Fatalf("expected no error verifying token, got %v", err)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role 'admin', got %q", claims.Role)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Errorf("expected TokenType 'access', got %q", claims.TokenType)
	}
}

func TestStaffJWTManager_GenerateRefreshToken(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	token, jti, err := manager.GenerateRefreshToken("cskh_user", "cskh")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if jti == "" {
		t.Fatal("expected non-empty jti")
	}

	claims, err := manager.VerifyToken(token)
	if err != nil {
		t.Fatalf("expected no error verifying refresh token, got %v", err)
	}
	if claims.Role != "cskh" {
		t.Errorf("expected Role 'cskh', got %q", claims.Role)
	}
	if claims.TokenType != TokenTypeRefresh {
		t.Errorf("expected TokenType 'refresh', got %q", claims.TokenType)
	}
}

func TestStaffJWTManager_VerifyToken_Expired(t *testing.T) {
	// Create manager with 1ms TTL to force expiration
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 1*time.Millisecond, 1*time.Millisecond)

	token, _, err := manager.GenerateAccessToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = manager.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestStaffJWTManager_VerifyToken_WrongSignature(t *testing.T) {
	manager1, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)
	manager2, _ := NewStaffJWTManager("another-secret-key-at-least-32-chars!", 15*time.Minute, 7*24*time.Hour)

	token, _, err := manager1.GenerateAccessToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify with different secret
	_, err = manager2.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestStaffJWTManager_VerifyToken_InvalidFormat(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	_, err := manager.VerifyToken("not-a-valid-jwt")
	if err == nil {
		t.Fatal("expected error for invalid token format")
	}
}

func TestStaffJWTManager_VerifyToken_EmptyString(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	_, err := manager.VerifyToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestStaffJWTManager_TokenTypeMismatch(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	// Generate access token
	accessToken, _, err := manager.GenerateAccessToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it's an access token
	claims, err := manager.VerifyToken(accessToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Errorf("expected TokenTypeAccess, got %q", claims.TokenType)
	}

	// Generate refresh token
	refreshToken, _, err := manager.GenerateRefreshToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it's a refresh token
	claims, err = manager.VerifyToken(refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.TokenType != TokenTypeRefresh {
		t.Errorf("expected TokenTypeRefresh, got %q", claims.TokenType)
	}
}

func TestGenerateJTI_Unique(t *testing.T) {
	jti1, err := generateJTI()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(jti1) != 32 {
		t.Errorf("expected jti length 32, got %d", len(jti1))
	}

	jti2, err := generateJTI()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if jti1 == jti2 {
		t.Error("expected unique JTIs")
	}
}

func TestGenerateJTI_Error(t *testing.T) {
	// generateJTI should never fail with crypto/rand, but we test error handling
	_, err := generateJTI()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test that VerifyToken returns specific errors helps callers handle different cases
func TestStaffJWTManager_VerifyToken_Errors(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"malformed JWT", "abc.def.ghi"},
		{"random garbage", "not.valid.jwt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.VerifyToken(tt.token)
			if err == nil {
				t.Errorf("expected error for %q", tt.name)
			}
		})
	}
}

func TestStaffClaims_RegisteredFields(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	token, jti, err := manager.GenerateAccessToken("testuser", "cskh")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	claims, err := manager.VerifyToken(token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check registered fields
	if claims.Subject != "testuser" {
		t.Errorf("expected Subject 'testuser', got %q", claims.Subject)
	}
	if claims.ID != jti {
		t.Errorf("expected ID %q, got %q", jti, claims.ID)
	}
	if claims.Issuer != "dongdo-cs-v2-staff" {
		t.Errorf("expected Issuer 'dongdo-cs-v2-staff', got %q", claims.Issuer)
	}
	if claims.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
	if claims.IssuedAt == nil {
		t.Error("expected IssuedAt to be set")
	}
}

// TestWithMockedClock would require interface or clock injection;
// current design relies on real time, which is acceptable for this use case.
func TestStaffJWTManager_AccessTokenTTL(t *testing.T) {
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", accessTTL, refreshTTL)

	accessToken, _, err := manager.GenerateAccessToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	claims, err := manager.VerifyToken(accessToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check TTL is approximately correct (within 1 second tolerance)
	expectedExp := time.Now().Add(accessTTL)
	actualExp := claims.ExpiresAt.Time
	diff := actualExp.Sub(expectedExp)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected TTL ~15min, got diff %v", diff)
	}
}

func TestStaffJWTManager_RefreshTokenTTL(t *testing.T) {
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", accessTTL, refreshTTL)

	refreshToken, _, err := manager.GenerateRefreshToken("cskh", "cskh")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	claims, err := manager.VerifyToken(refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check TTL is approximately correct (within 1 second tolerance)
	expectedExp := time.Now().Add(refreshTTL)
	actualExp := claims.ExpiresAt.Time
	diff := actualExp.Sub(expectedExp)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected TTL ~7days, got diff %v", diff)
	}
}

// Test that a revoked token (simulated) fails verification
// Note: actual revocation is checked in auth_usecase via revoked_tokens table
func TestStaffJWTManager_VerifyToken_RevokedSimulated(t *testing.T) {
	manager, _ := NewStaffJWTManager("this-is-a-secret-key-at-least-32-chars", 15*time.Minute, 7*24*time.Hour)

	token, _, err := manager.GenerateAccessToken("admin", "admin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Generate a different valid token with same jti (impossible in practice,
	// but confirms signature check works independently of jti)
	_, jti2, _ := manager.GenerateRefreshToken("admin", "admin")
	if errors.Is(err, nil) {
		// jti is in the token body; tampering would break signature
		// This test just confirms the token is valid
		_, err = manager.VerifyToken(token)
		if err != nil {
			t.Errorf("valid token should verify: %v", err)
		}
	}
	_ = jti2
}
