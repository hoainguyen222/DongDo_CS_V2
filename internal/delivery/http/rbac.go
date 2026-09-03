package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// Role-based access control levels
type RoleLevel int

const (
	RoleStaff RoleLevel = iota
	RoleLeader
	RoleAdmin
	RoleOwner
)

func (r RoleLevel) String() string {
	switch r {
	case RoleStaff:
		return "Staff"
	case RoleLeader:
		return "Leader"
	case RoleAdmin:
		return "Admin"
	case RoleOwner:
		return "Owner"
	default:
		return "Staff"
	}
}

// ParseRoleLevel converts role string to RoleLevel
func ParseRoleLevel(role string) RoleLevel {
	switch {
	case containsIgnoreCase(role, "owner"):
		return RoleOwner
	case containsIgnoreCase(role, "admin"):
		return RoleAdmin
	case containsIgnoreCase(role, "leader"):
		return RoleLeader
	default:
		return RoleStaff
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsLower(toLower(s), toLower(substr)))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range b {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func containsLower(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RequireRoles creates a middleware that requires one of the specified roles
// Usage: RequireRoles(RoleOwner, RoleAdmin)
func RequireRoles(allowedRoles ...RoleLevel) gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Unauthorized"})
			return
		}

		user, ok := userVal.(*domain.SessionUser)
		if !ok || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Invalid user session"})
			return
		}

		// Convert UserRole to string and parse
		userRole := ParseRoleLevel(string(user.Role))

		// Check if user's role is in the allowed roles
		for _, allowed := range allowedRoles {
			if userRole >= allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"detail": "Bạn không có quyền truy cập tính năng này.",
		})
	}
}

// RequireAnyRole is a convenience function for RequireRoles
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	var levels []RoleLevel
	for _, r := range roles {
		levels = append(levels, ParseRoleLevel(r))
	}
	return RequireRoles(levels...)
}
