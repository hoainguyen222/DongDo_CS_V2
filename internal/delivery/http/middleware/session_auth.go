package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
)

// Context keys for session info.
const (
	CtxKeySession     = "session"
	CtxKeySessionID   = "session_id"
	CtxKeyGuestID     = "guest_id"
	CtxKeyDisplayName = "display_name"
)

// SessionAuth returns a Gin middleware that validates the guest_session cookie.
// On success, sets session info in Gin context for downstream handlers.
// On failure, clears the cookie and returns 401.
func SessionAuth(sessionUC *usecase.SessionUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("guest_session")
		if err != nil || sessionID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing session",
				"code":  "GUEST_SESSION_REQUIRED",
			})
			return
		}

		session, err := sessionUC.ValidateSession(c.Request.Context(), sessionID)
		if err != nil {
			// Clear invalid cookie
			c.SetCookie("guest_session", "", -1, "/", "", false, false)

			switch {
			case errors.Is(err, usecase.ErrSessionExpired):
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "session expired",
					"code":  "SESSION_EXPIRED",
				})
			case errors.Is(err, usecase.ErrSessionInactive):
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "session inactive",
					"code":  "SESSION_INACTIVE",
				})
			case errors.Is(err, usecase.ErrSessionNotFound):
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "session not found",
					"code":  "SESSION_NOT_FOUND",
				})
			default:
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "invalid session",
					"code":  "INVALID_SESSION",
				})
			}
			return
		}

		// Set session info in context
		c.Set(CtxKeySession, session)
		c.Set(CtxKeySessionID, session.SessionID)
		if session.GuestID != nil {
			c.Set(CtxKeyGuestID, *session.GuestID)
		}
		c.Set(CtxKeyDisplayName, session.DisplayName)
		c.Next()
	}
}

// GetSessionFromContext extracts the chat session from Gin context.
func GetSessionFromContext(c *gin.Context) (*domain.ChatSession, bool) {
	v, ok := c.Get(CtxKeySession)
	if !ok {
		return nil, false
	}
	session, ok := v.(*domain.ChatSession)
	return session, ok
}