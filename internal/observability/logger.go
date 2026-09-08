package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RequestIDHeader is the canonical inbound/outbound request id header.
// We prefer the upstream value (X-Request-ID) when present so distributed
// traces stay correlated across hops; otherwise we mint a fresh UUIDv4.
const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware installs a request id on c and echoes it in the
// response header. The id is also attached to the request context so
// downstream use cases can include it in their own structured logs.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(RequestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set("request_id", rid)
		c.Writer.Header().Set(RequestIDHeader, rid)
		c.Next()
	}
}

// RequestIDFromContext extracts the request id from a gin context, returning
// "" if absent. Use this when building structured log lines from use cases.
func RequestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// AccessLogMiddleware emits ONE structured log line per request, after the
// handler chain has run. The line carries method, route template (not raw
// path), status, latency, request_id, client IP, user-agent and an optional
// user_id from the auth middleware.
//
// Sensitive fields (Authorization header, request body, response body) are
// deliberately NOT logged. Errors from c.Errors are attached as a single
// string field rather than the full error chain.
//
// This middleware replaces gin.Logger() so we keep one canonical access-log
// shape with consistent fields across the system.
func AccessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		elapsed := time.Since(start)

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		evt := log.Info()
		if c.Writer.Status() >= 500 {
			evt = log.Error()
		} else if c.Writer.Status() >= 400 {
			evt = log.Warn()
		}

		evt.
			Str("component", "http_access").
			Str("request_id", RequestIDFromContext(c)).
			Str("method", c.Request.Method).
			Str("route", route).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Int("bytes", c.Writer.Size()).
			Dur("duration", elapsed).
			Str("client_ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent())

		// Attach user id if auth middleware populated it.
		if u, ok := c.Get("user"); ok {
			// We do not depend on a concrete type; just stringify whatever
			// the auth middleware stored. The delivery layer already trims
			// sensitive fields.
			if s, ok := u.(interface{ String() string }); ok {
				evt.Str("user", s.String())
			}
		}

		if len(c.Errors) > 0 {
			// Collapse to the last error message to avoid huge log lines.
			evt.Str("error", c.Errors.Last().Error())
		}

		evt.Msg("http request")
	}
}
