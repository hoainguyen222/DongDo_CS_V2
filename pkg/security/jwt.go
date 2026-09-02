package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType defines the type of JWT token.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// StaffClaims are JWT claims for staff (admin/cskh) users.
// Guest/customer sessions do NOT use JWT — see Task 02b.
type StaffClaims struct {
	Role      string    `json:"role"`     // "admin" | "cskh"
	TokenType TokenType `json:"type"`     // "access" | "refresh"
	jwt.RegisteredClaims
}

// StaffJWTManager handles JWT for staff authentication only.
type StaffJWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// NewStaffJWTManager creates a new JWT manager for staff authentication.
// Fails fast if secret is less than 32 characters.
func NewStaffJWTManager(secret string, accessTTL, refreshTTL time.Duration) (*StaffJWTManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters")
	}
	return &StaffJWTManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     "dongdo-cs-v2-staff",
	}, nil
}

// GenerateAccessToken creates an access token with 15-minute TTL by default.
func (m *StaffJWTManager) GenerateAccessToken(userID, role string) (token string, jtiVal string, err error) {
	jtiVal, err = generateJTI()
	if err != nil {
		return "", "", err
	}
	claims := StaffClaims{
		Role:      role,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jtiVal,
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tokenObj.SignedString(m.secret)
	return signed, jtiVal, err
}

// GenerateRefreshToken creates a refresh token with 7-day TTL by default.
func (m *StaffJWTManager) GenerateRefreshToken(userID, role string) (token string, jtiVal string, err error) {
	jtiVal, err = generateJTI()
	if err != nil {
		return "", "", err
	}
	claims := StaffClaims{
		Role:      role,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jtiVal,
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tokenObj.SignedString(m.secret)
	return signed, jtiVal, err
}

// VerifyToken verifies a JWT token and returns its claims.
// Returns error if token is invalid, expired, or has wrong signing method.
func (m *StaffJWTManager) VerifyToken(tokenString string) (*StaffClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &StaffClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*StaffClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// generateJTI creates a unique JWT ID (jti) for token revocation.
func generateJTI() (string, error) {
	b := make([]byte, 16) // 32 hex characters
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
