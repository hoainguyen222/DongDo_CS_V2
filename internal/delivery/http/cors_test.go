package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCORSMiddleware_AllowedOrigin_HeadersSet checks allowed origin echoes back.
func TestCORSMiddleware_AllowedOrigin_HeadersSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:3000"}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("expected Allow-Origin http://localhost:3000, got %q", got)
	}
	if got := w.Header().Get("Vary"); !contains(got, "Origin") {
		t.Errorf("expected Vary to include Origin, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Allow-Credentials true, got %q", got)
	}
}

// TestCORSMiddleware_DisallowedOrigin_NoHeaders checks non-whitelisted origin gets no headers.
func TestCORSMiddleware_DisallowedOrigin_NoHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:3000"}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin for non-whitelisted, got %q", got)
	}
}

// TestCORSMiddleware_PreflightAllowed_204 checks OPTIONS preflight with allowed origin returns 204.
func TestCORSMiddleware_PreflightAllowed_204(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:3000"}))
	r.OPTIONS("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", w.Code)
	}
}

// TestCORSMiddleware_PreflightDisallowed_403 checks OPTIONS preflight with disallowed origin returns 403.
func TestCORSMiddleware_PreflightDisallowed_403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:3000"}))
	r.OPTIONS("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed preflight, got %d", w.Code)
	}
}

// TestCORSMiddleware_NoWildcard ensures Access-Control-Allow-Origin is never "*".
func TestCORSMiddleware_NoWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:3000"}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Error("Access-Control-Allow-Origin must never be wildcard")
	}
}

// TestCORSMiddleware_MultipleOrigins checks that any of several origins is accepted.
func TestCORSMiddleware_MultipleOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	whitelist := []string{"http://localhost:3000", "http://localhost:3001", "https://app.example.com"}
	r.Use(CORSMiddleware(whitelist))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	for _, origin := range whitelist {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %s expected %q, got %q", origin, origin, got)
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}