package logging

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// Field names for structured logging (consistent across all packages).
const (
	FieldRequestID = "request_id"
	FieldUserID   = "user_id"
	FieldMethod   = "method"
	FieldPath     = "path"
	FieldStatus   = "status"
	FieldDuration = "duration_ms"
	FieldClientIP = "client_ip"
	FieldUserAgent = "user_agent"
	FieldWorker   = "worker"
	FieldJobID    = "job_id"
	FieldModel    = "model"
	FieldTokensIn = "prompt_tokens"
	FieldTokensOut = "completion_tokens"
	FieldLatency  = "latency_ms"
)

// Redaction patterns — compiled once at init.
var (
	// Matches "Authorization: Bearer TOKEN" or "authorization=TOKEN" → redacts token value
	authHeaderRe = regexp.MustCompile(
		`(?i)(authorization[\s:=]+(?:bearer[\s]+)?)([^\s,;"]+)`,
	)
	// Standalone Bearer prefix in JWT-style tokens
	bearerRe     = regexp.MustCompile(`(?i)(bearer[\s]+)([^\s,;"]+)`)
	// Matches "token=abc" or "token: abc"
	tokenRe      = regexp.MustCompile(`(?i)(token[\s:=]+)([^\s,;"]+)`)
	// Matches "password=xxx"
	pwdRe        = regexp.MustCompile(`(?i)(password[\s:=]+)([^\s,;"]+)`)
	// Matches "api_key=xxx" or "api-key: xxx"
	apiKeyRe     = regexp.MustCompile(`(?i)(api[_-]?key[\s:=]+)([^\s,;"]+)`)
	phoneRe      = regexp.MustCompile(`\b0\d{9,10}\b`)  // Vietnamese phone
	emailRe      = regexp.MustCompile(`\b[\w.-]+@[\w.-]+\.\w+\b`)
	creditCardRe = regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`)
)

// Redact replaces known sensitive patterns with [REDACTED].
func Redact(s string) string {
	if s == "" {
		return ""
	}
	s = authHeaderRe.ReplaceAllString(s, "$1[REDACTED]")
	s = bearerRe.ReplaceAllString(s, "$1[REDACTED]")
	s = tokenRe.ReplaceAllString(s, "$1[REDACTED]")
	s = pwdRe.ReplaceAllString(s, "$1[REDACTED]")
	s = apiKeyRe.ReplaceAllString(s, "$1[REDACTED]")
	s = phoneRe.ReplaceAllString(s, "[PHONE]")
	s = emailRe.ReplaceAllString(s, "[EMAIL]")
	s = creditCardRe.ReplaceAllString(s, "[CARD]")
	return s
}

// RedactMap recursively scans a map and redacts sensitive keys.
func RedactMap(m map[string]any) map[string]any {
	sensitive := []string{"password", "token", "secret", "api_key", "apikey", "authorization", "pwd", "authorization"}
	result := make(map[string]any, len(m))
	for k, v := range m {
		lowerK := strings.ToLower(k)
		isSensitive := false
		for _, s := range sensitive {
			if strings.Contains(lowerK, s) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			result[k] = "[REDACTED]"
		} else if vm, ok := v.(map[string]any); ok {
			result[k] = RedactMap(vm)
		} else if vs, ok := v.(string); ok {
			result[k] = Redact(vs)
		} else {
			result[k] = v
		}
	}
	return result
}

// InitLogger initializes structured JSON logging for production.
// Call this early in main() before any other log calls.
// level: "debug", "info", "warn"/"warning", "error" (default: "info").
func InitLogger(level string) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Rename "time" to "timestamp" for ELK compatibility
			if a.Key == "time" {
				a.Key = "timestamp"
			}
			return a
		},
	}
	handler = slog.NewJSONHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// AttrsToAny converts []slog.Attr to []any for slog function calls.
// This is needed because slog.Info/Warn/Error accept ...any, not []slog.Attr.
func AttrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs)*2)
	for i, a := range attrs {
		result[i*2] = a.Key
		result[i*2+1] = a.Value.Any()
	}
	return result
}

// FromContext extracts request_id from context (supports both
// context.WithValue and gin.Context via request_id middleware).
func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID := ctx.Value("request_id"); reqID != nil {
		if s, ok := reqID.(string); ok {
			return s
		}
	}
	return ""
}

// FromString extracts a string request ID from a value.
func FromString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
