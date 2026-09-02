package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ─── Security Headers ───────────────────────────────────────────────────────

func TestSecurityHeaders_AlwaysSetExceptHSTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	checks := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=()",
		"X-XSS-Protection":       "1; mode=block",
	}
	for header, expected := range checks {
		got := w.Header().Get(header)
		if got == "" {
			t.Errorf("header %s missing", header)
			continue
		}
		if expected != "" && !contains(got, expected) {
			t.Errorf("header %s expected to contain %q, got %q", header, expected, got)
		}
	}
}

func TestSecurityHeaders_CSPPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
	for _, expected := range []string{
		"default-src 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
	} {
		if !contains(csp, expected) {
			t.Errorf("CSP missing %q. Got: %s", expected, csp)
		}
	}
}

func TestSecurityHeaders_HSTSOnlyOnHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// HTTP request → no HSTS
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS should be empty on HTTP, got %q", got)
	}

	// X-Forwarded-Proto: https → HSTS set
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Forwarded-Proto", "https")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	hsts := w2.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("HSTS should be set when X-Forwarded-Proto=https")
	}
	if !contains(hsts, "max-age=31536000") {
		t.Errorf("HSTS missing max-age. Got: %s", hsts)
	}
}

func TestSecurityHeaders_HeadersNotAffectedByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/notfound", func(c *gin.Context) { c.Status(404) })

	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("security headers should be set even on error responses, got %q", got)
	}
}

// ─── Request ID ─────────────────────────────────────────────────────────────

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		rid, _ := c.Get("request_id")
		c.JSON(200, gin.H{"rid": rid})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	rid := w.Header().Get("X-Request-ID")
	if rid == "" {
		t.Fatal("X-Request-ID missing")
	}
}

func TestRequestID_PassesThroughExisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "trace-abc-123" {
		t.Errorf("expected trace-abc-123, got %q", got)
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