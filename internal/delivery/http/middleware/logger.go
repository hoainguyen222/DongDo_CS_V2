package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/logging"
)

// StructuredLogger returns a Gin middleware that logs HTTP requests in structured JSON format.
// It captures: method, path, status, duration, client_ip, user_agent, request_id, user_id.
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Collect timing and status
		duration := time.Since(start)

		// Get user_id if authenticated
		userID := ""
		if u, exists := c.Get("user"); exists {
			if user, ok := u.(*domain.SessionUser); ok && user != nil {
				userID = user.Username
			}
		}

		// Get request_id (set by RequestID middleware)
		requestID, _ := c.Get("request_id")
		rid := logging.FromString(requestID)

		status := c.Writer.Status()

		// Build log attributes
		attrs := []slog.Attr{
			slog.String(logging.FieldMethod, c.Request.Method),
			slog.String(logging.FieldPath, path),
			slog.Int(logging.FieldStatus, status),
			slog.Duration(logging.FieldDuration, duration),
			slog.String(logging.FieldClientIP, c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("body_size", c.Writer.Size()),
			slog.String("request_id", rid),
		}
		if userID != "" {
			attrs = append(attrs, slog.String(logging.FieldUserID, userID))
		}
		if query != "" {
			// Redact query params to avoid logging sensitive data
			attrs = append(attrs, slog.String("query", logging.Redact(query)))
		}

		if status >= 500 {
			slog.Error("request completed",
				logging.AttrsToAny(attrs)...,
			)
		} else if status >= 400 {
			slog.Warn("request completed",
				logging.AttrsToAny(attrs)...,
			)
		} else {
			slog.Info("request completed",
				logging.AttrsToAny(attrs)...,
			)
		}
	}
}
