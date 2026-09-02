package logging

import (
	"log/slog"
	"os"
	"testing"
)

func init() {
	// Ensure a default logger is set before tests run
	if slog.Default() == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	}
}

// ─── Redact tests ───────────────────────────────────────────────────────────

func TestRedact_AuthHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Authorization: Bearer sk-ant-xxx", "Authorization: Bearer [REDACTED]"},
		{"authorization: sk-ant-api03-xxx", "authorization: [REDACTED]"},
		{"Bearer abc123", "Bearer [REDACTED]"},
	}
	for _, tc := range tests {
		got := Redact(tc.input)
		if got == tc.input {
			t.Errorf("Redact(%q) returned unchanged — expected redaction", tc.input)
		}
		if got == "" {
			t.Errorf("Redact(%q) returned empty", tc.input)
		}
		// Verify no credential-like content remains
		if contains(got, "sk-ant") || contains(got, "Bearer abc") {
			t.Errorf("Redact(%q) = %q — still contains sensitive content", tc.input, got)
		}
	}
}

func TestRedact_Password(t *testing.T) {
	input := `password=DongDo@2026&username=admin`
	got := Redact(input)
	if contains(got, "DongDo@2026") {
		t.Errorf("password leaked: %s", got)
	}
}

func TestRedact_APIKey(t *testing.T) {
	input := `api_key=sk-ant-xxx&model=claude`
	got := Redact(input)
	if contains(got, "sk-ant-xxx") {
		t.Errorf("api_key leaked: %s", got)
	}
}

func TestRedact_Phone(t *testing.T) {
	input := `SĐT: 0912345678`
	got := Redact(input)
	if contains(got, "0912345678") {
		t.Errorf("phone leaked: %s", got)
	}
}

func TestRedact_Email(t *testing.T) {
	input := `email=john.doe@example.com`
	got := Redact(input)
	if contains(got, "john.doe@example.com") {
		t.Errorf("email leaked: %s", got)
	}
}

func TestRedact_CreditCard(t *testing.T) {
	input := `card=1234 5678 9012 3456`
	got := Redact(input)
	if contains(got, "1234") && contains(got, "5678") {
		t.Errorf("credit card leaked: %s", got)
	}
}

func TestRedact_NoCredentials(t *testing.T) {
	input := `page=1&limit=20&q=hello`
	got := Redact(input)
	if got != input {
		t.Errorf("Redact modified clean input: %q → %q", input, got)
	}
}

func TestRedact_Empty(t *testing.T) {
	if got := Redact(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ─── RedactMap tests ───────────────────────────────────────────────────────

func TestRedactMap_PasswordRedacted(t *testing.T) {
	input := map[string]any{
		"username": "admin",
		"password": "secret123",
	}
	got := RedactMap(input)
	if pw, ok := got["password"].(string); !ok || pw != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %v", got["password"])
	}
	if user, ok := got["username"].(string); !ok || user != "admin" {
		t.Errorf("expected admin, got %v", got["username"])
	}
}

func TestRedactMap_TokenRedacted(t *testing.T) {
	input := map[string]any{
		"access_token": "eyJhbGci...",
	}
	got := RedactMap(input)
	if got["access_token"] != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %v", got["access_token"])
	}
}

func TestRedactMap_Nested(t *testing.T) {
	input := map[string]any{
		"data": map[string]any{
			"token": "secret",
		},
	}
	got := RedactMap(input)
	nested := got["data"].(map[string]any)
	if nested["token"] != "[REDACTED]" {
		t.Errorf("expected nested [REDACTED], got %v", nested["token"])
	}
}

func TestRedactMap_Clean(t *testing.T) {
	input := map[string]any{
		"username": "admin",
		"role":    "cskh",
	}
	got := RedactMap(input)
	if got["username"] != "admin" || got["role"] != "cskh" {
		t.Errorf("unexpected redaction of clean fields: %v", got)
	}
}

// ─── InitLogger tests ───────────────────────────────────────────────────────

func TestInitLogger_ValidLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	for _, l := range levels {
		InitLogger(l) // should not panic
	}
}

func TestInitLogger_UnknownLevel(t *testing.T) {
	InitLogger("trace") // should default to info
}

func TestInitLogger_EmptyLevel(t *testing.T) {
	InitLogger("") // should default to info
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return len(substr) <= len(s) && findSubstr(s, substr)
}

func findSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
