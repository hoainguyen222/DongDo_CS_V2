package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// MiddlewareConfig holds configuration for the logging middleware
type MiddlewareConfig struct {
	// Logger is the logger instance to use
	Logger *Logger
	// SkipPaths is a list of paths to skip logging
	SkipPaths []string
	// SkipMethods is a list of HTTP methods to skip logging
	SkipMethods []string
	// LogRequestBody determines if request body should be logged
	LogRequestBody bool
	// LogResponseBody determines if response body should be logged
	LogResponseBody bool
	// MaxBodySize is the maximum size of body to log (in bytes)
	MaxBodySize int
	// SanitizeFields is a list of field names to sanitize (replace with [REDACTED])
	SanitizeFields []string
	// IncludeHeaders is a list of headers to include in logs
	IncludeHeaders []string
	// RequestIDHeader is the header name for request ID (used for tracing)
	RequestIDHeader string
	// GenerateRequestID determines if a new request ID should be generated
	GenerateRequestID bool
}

// responseWriter wraps http.ResponseWriter to capture status code and body
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// bodyCapture wraps an io.ReadCloser to capture the body
type bodyCapture struct {
	io.ReadCloser
	buffer *bytes.Buffer
}

func (bc *bodyCapture) Read(p []byte) (int, error) {
	n, err := bc.ReadCloser.Read(p)
	bc.buffer.Write(p[:n])
	return n, err
}

// defaultMiddlewareConfig returns the default middleware configuration
func defaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		Logger:            Get(),
		SkipPaths:         []string{"/health", "/ready", "/metrics"},
		SkipMethods:       []string{"OPTIONS"},
		LogRequestBody:    true,
		LogResponseBody:   false,
		MaxBodySize:       10 * 1024, // 10KB
		SanitizeFields:   []string{"password", "token", "secret", "authorization", "api_key", "apikey", "access_token", "refresh_token", "session_id", "cookie"},
		IncludeHeaders:    []string{"Content-Type", "User-Agent", "X-Request-ID", "X-Forwarded-For"},
		RequestIDHeader:   "X-Request-ID",
		GenerateRequestID: true,
	}
}

// Middleware creates a new logging middleware
func Middleware(config ...MiddlewareConfig) func(http.Handler) http.Handler {
	cfg := defaultMiddlewareConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	// Pre-compile sanitization patterns
	sanitizePatterns := make([]*regexp.Regexp, len(cfg.SanitizeFields))
	for i, field := range cfg.SanitizeFields {
		sanitizePatterns[i] = regexp.MustCompile(`(?i)` + field)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should be skipped
			if shouldSkip(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if method should be skipped
			if shouldSkipMethod(r.Method, cfg.SkipMethods) {
				next.ServeHTTP(w, r)
				return
			}

			startTime := time.Now()

			// Generate or extract request ID
			requestID := extractOrGenerateRequestID(r, cfg)

			// Create context with request ID
			ctx := WithRequestID(r.Context(), requestID)

			// Create fields for logging
			fields := map[string]interface{}{
				"request_id": requestID,
				"method":     r.Method,
				"path":       r.URL.Path,
				"query":      r.URL.RawQuery,
				"client_ip":  getClientIP(r),
				"user_agent": r.UserAgent(),
				"protocol":   r.Proto,
				"host":       r.Host,
			}

			// Add user ID if available (from auth middleware)
			if userID := GetUserID(ctx); userID != "" {
				fields["user_id"] = userID
			}

			// Add session ID if available
			if sessionID := GetSessionID(ctx); sessionID != "" {
				fields["session_id"] = sessionID
			}

			// Add headers if configured
			for _, header := range cfg.IncludeHeaders {
				if value := r.Header.Get(header); value != "" {
					headerKey := strings.ToLower(header)
					fields["header_"+headerKey] = sanitizeValue(header, value, sanitizePatterns)
				}
			}

			// Capture request body
			var requestBody []byte
			if cfg.LogRequestBody && r.Body != nil {
				bodyBuffer := &bytes.Buffer{}
				r.Body = &bodyCapture{
					ReadCloser: r.Body,
					buffer:     bodyBuffer,
				}
				requestBody = bodyBuffer.Bytes()
			}

			// Wrap response writer
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:      http.StatusOK,
				body:            &bytes.Buffer{},
			}

			// Log request start
			logRequestStart(ctx, fields, requestBody, cfg)

			// Update context in request
			r = r.WithContext(ctx)

			// Process request
			next.ServeHTTP(rw, r)

			// Calculate latency
			latency := time.Since(startTime)

			// Add response fields
			fields["status"] = rw.statusCode
			fields["latency_ms"] = latency.Milliseconds()
			fields["latency"] = latency.String()
			fields["response_size"] = rw.body.Len()

			// Sanitize response body if logged
			if cfg.LogResponseBody && rw.body.Len() > 0 {
				responseBody := rw.body.Bytes()
				if len(responseBody) > cfg.MaxBodySize {
					responseBody = responseBody[:cfg.MaxBodySize]
					fields["response_body_truncated"] = true
				}
				fields["response_body"] = sanitizeJSONBody(responseBody, sanitizePatterns)
			}

			// Log access
			cfg.Logger.LogAccess(
				requestID,
				r.Method,
				r.URL.Path,
				getClientIP(r),
				r.UserAgent(),
				rw.statusCode,
				latency,
				GetUserID(ctx),
			)

			// Log based on status code
			logRequestEnd(ctx, fields, rw.statusCode)
		})
	}
}

// shouldSkip checks if the path should be skipped
func shouldSkip(path string, skipPaths []string) bool {
	for _, skipPath := range skipPaths {
		if path == skipPath || strings.HasPrefix(path, skipPath+"/") {
			return true
		}
	}
	return false
}

// shouldSkipMethod checks if the method should be skipped
func shouldSkipMethod(method string, skipMethods []string) bool {
	for _, skipMethod := range skipMethods {
		if method == skipMethod {
			return true
		}
	}
	return false
}

// extractOrGenerateRequestID extracts request ID from header or generates a new one
func extractOrGenerateRequestID(r *http.Request, cfg MiddlewareConfig) string {
	// Try to get from header first
	if cfg.RequestIDHeader != "" {
		if reqID := r.Header.Get(cfg.RequestIDHeader); reqID != "" {
			return reqID
		}
	}

	// Try common headers
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}
	if reqID := r.Header.Get("X-Correlation-ID"); reqID != "" {
		return reqID
	}
	if reqID := r.Header.Get("X-Trace-ID"); reqID != "" {
		return reqID
	}

	// Generate new ID if configured
	if cfg.GenerateRequestID {
		return generateRequestID()
	}

	return ""
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return uuid.New().String()
}

// getClientIP extracts the real client IP considering proxies
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Check X-Client-IP header (common in some setups)
	if xci := r.Header.Get("X-Client-IP"); xci != "" {
		return xci
	}

	// Fall back to remote address
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	// Handle IPv6 loopback
	if ip == "::1" {
		return "127.0.0.1"
	}

	return ip
}

// logRequestStart logs the request start with details
func logRequestStart(ctx context.Context, fields map[string]interface{}, body []byte, cfg MiddlewareConfig) {
	zl := With().Ctx(ctx)

	event := zl.Info()
	
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			event = event.Str(k, val)
		case int:
			event = event.Int(k, val)
		case int64:
			event = event.Int64(k, val)
		case bool:
			event = event.Bool(k, val)
		default:
			event = event.Interface(k, val)
		}
	}

	// Add request body if present and configured
	if len(body) > 0 {
		if len(body) > cfg.MaxBodySize {
			body = body[:cfg.MaxBodySize]
			event = event.Bool("request_body_truncated", true)
		}
		event = event.Str("request_body", sanitizeJSONBody(body, nil))
	}

	event.Msg("request started")
}

// logRequestEnd logs the request end with details
func logRequestEnd(ctx context.Context, fields map[string]interface{}, statusCode int) {
	zl := With().Ctx(ctx)

	// Choose log level based on status code
	var event *zerolog.Event
	if statusCode >= 500 {
		event = zl.Error()
	} else if statusCode >= 400 {
		event = zl.Warn()
	} else {
		event = zl.Info()
	}

	for k, v := range fields {
		switch val := v.(type) {
		case string:
			event = event.Str(k, val)
		case int:
			event = event.Int(k, val)
		case int64:
			event = event.Int64(k, val)
		case bool:
			event = event.Bool(k, val)
		default:
			event = event.Interface(k, val)
		}
	}

	event.Msg("request completed")
}

// sanitizeValue sanitizes sensitive values in headers or query params
func sanitizeValue(field, value string, patterns []*regexp.Regexp) string {
	// Check if field name matches any sanitize patterns
	for _, pattern := range patterns {
		if pattern.MatchString(field) {
			return "[REDACTED]"
		}
	}
	return value
}

// sanitizeJSONBody sanitizes sensitive fields in JSON body
func sanitizeJSONBody(body []byte, patterns []*regexp.Regexp) string {
	if len(patterns) == 0 {
		return string(body)
	}

	// Simple JSON sanitization - look for field patterns in the body
	result := string(body)
	for _, pattern := range patterns {
		// Match field patterns like "password":"value" or "password": "value"
		re := regexp.MustCompile(`(?i)("` + pattern.String() + `"\s*:\s*)("[^"]*"|\d+)`)
		result = re.ReplaceAllString(result, `$1"[REDACTED]"`)
	}

	return result
}

// SanitizeQuery sanitizes sensitive query parameters
func SanitizeQuery(query string, sanitizeFields []string) string {
	if query == "" || len(sanitizeFields) == 0 {
		return query
	}

	values, err := url.ParseQuery(query)
	if err != nil {
		return query
	}

	for _, field := range sanitizeFields {
		for key := range values {
			if strings.EqualFold(key, field) || strings.Contains(strings.ToLower(key), field) {
				values.Set(key, "[REDACTED]")
			}
		}
	}

	return values.Encode()
}

// RequestIDMiddleware is a simple middleware to add request ID to context
func RequestIDMiddleware(requestIDHeader string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := ""
			
			// Try to get from header
			if requestIDHeader != "" {
				requestID = r.Header.Get(requestIDHeader)
			}
			
			// Generate if not present
			if requestID == "" {
				requestID = generateRequestID()
			}

			// Set request ID header in response
			w.Header().Set(requestIDHeader, requestID)

			// Add to context
			ctx := WithRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// AuditMiddleware creates a middleware for audit logging specific actions
type AuditConfig struct {
	// Logger is the logger instance to use
	Logger *Logger
	// Actions is a map of path patterns to audit actions
	Actions map[string]string
	// LogRequest determines if request details should be logged
	LogRequest bool
	// LogResponse determines if response details should be logged
	LogResponse bool
}

// AuditMiddleware creates an audit logging middleware
func AuditMiddleware(config AuditConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only log specific actions
			action := config.Actions[r.URL.Path]
			if action == "" {
				next.ServeHTTP(w, r)
				return
			}

			startTime := time.Now()

			// Capture response
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			// Capture request body if configured
			var requestBody []byte
			if config.LogRequest && r.Body != nil {
				bodyBuffer := &bytes.Buffer{}
				r.Body = &bodyCapture{
					ReadCloser: r.Body,
					buffer:     bodyBuffer,
				}
				requestBody = bodyBuffer.Bytes()
			}

			// Process request
			next.ServeHTTP(rw, r)

			// Determine success
			success := rw.statusCode < 400

			// Log audit entry
			details := map[string]interface{}{
				"method":      r.Method,
				"path":         r.URL.Path,
				"status":       rw.statusCode,
				"duration_ms": time.Since(startTime).Milliseconds(),
			}

			if config.LogRequest && len(requestBody) > 0 {
				details["request"] = string(requestBody)
			}

			if config.LogResponse && rw.body.Len() > 0 {
				details["response"] = rw.body.String()
			}

			config.Logger.LogAudit(
				action,
				r.URL.Path,
				GetUserID(r.Context()),
				getClientIP(r),
				formatAuditDetails(details),
				success,
			)
		})
	}
}

// formatAuditDetails formats audit details as a string
func formatAuditDetails(details map[string]interface{}) string {
	var parts []string
	for k, v := range details {
		parts = append(parts, k+"="+formatValue(v))
	}
	return strings.Join(parts, ", ")
}

// formatValue formats a value for logging
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if len(val) > 100 {
			return val[:100] + "..."
		}
		return val
	case int, int64, int32:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ContextLogger provides methods for context-aware logging
type ContextLogger struct {
	logger *Logger
}

// WithContext returns a context-aware logger
func (l *Logger) WithContext() *ContextLogger {
	return &ContextLogger{logger: l}
}

// Ctx returns a zerolog.Logger with context fields
func (cl *ContextLogger) Ctx(ctx context.Context) zerolog.Logger {
	if ctx == nil {
		return cl.logger.zl
	}

	zl := cl.logger.zl.With()

	// Extract common context fields
	if reqID, ok := ctx.Value(ContextKeyRequestID).(string); ok && reqID != "" {
		zl = zl.Str("request_id", reqID)
	}
	if userID, ok := ctx.Value(ContextKeyUserID).(string); ok && userID != "" {
		zl = zl.Str("user_id", userID)
	}
	if sessionID, ok := ctx.Value(ContextKeySessionID).(string); ok && sessionID != "" {
		zl = zl.Str("session_id", sessionID)
	}
	if tenantID, ok := ctx.Value(ContextKeyTenantID).(string); ok && tenantID != "" {
		zl = zl.Str("tenant_id", tenantID)
	}

	return zl.Logger()
}

// Helper for multipart form data sanitization
func sanitizeMultipartForm(f *multipart.Form, sanitizeFields []string) *multipart.Form {
	if f == nil {
		return nil
	}

	sanitized := &multipart.Form{
		Value: make(map[string][]string),
		File:  f.File,
	}

	for key, values := range f.Value {
		sanitizedValues := make([]string, len(values))
		for i, value := range values {
			sanitizedValues[i] = sanitizeValue(key, value, nil)
		}
		sanitized.Value[key] = sanitizedValues
	}

	return sanitized
}
