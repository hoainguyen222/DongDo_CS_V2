package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ─── Header Presence Tests ─────────────────────────────────────────────────

func TestRateLimitByIP_HeadersAlwaysPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// nil Redis client → fails-open (allow all)
	r.Use(RateLimitByIPSimple(nil, RateLimiterConfig{
		RequestsPerMinute: 5,
		KeyPrefix:         "test",
	}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (fail-open with nil redis), got %d", w.Code)
	}
	if got := w.Header().Get("X-RateLimit-Limit"); got != "5" {
		t.Errorf("expected X-RateLimit-Limit: 5, got %s", got)
	}
	if got := w.Header().Get("X-RateLimit-Remaining"); got == "" {
		t.Error("expected X-RateLimit-Remaining header to be set")
	}
	if got := w.Header().Get("X-RateLimit-Reset"); got == "" {
		t.Error("expected X-RateLimit-Reset header to be set")
	}
}

func TestRateLimitByIP_NilRedisFailsOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitByIPSimple(nil, RateLimiterConfig{
		RequestsPerMinute: 1,
		KeyPrefix:         "test",
	}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	// Even with limit=1, we should be allowed multiple times when Redis is nil
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.2")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d expected 200 (fail-open with nil redis), got %d", i+1, w.Code)
		}
	}
}

// ─── Rate Limit ByIP (no auth scoping) ─────────────────────────────────────

func TestRateLimitByIP_StandardHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitByIPSimple(nil, RateLimiterConfig{
		RequestsPerMinute: 100,
		KeyPrefix:         "test",
	}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// All three required headers
	for _, h := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if got := w.Header().Get(h); got == "" {
			t.Errorf("expected header %s to be set", h)
		}
	}
}

// ─── Different limiters isolated ────────────────────────────────────────────

func TestRateLimitByIP_DifferentPrefixes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prefixA := RateLimiterConfig{RequestsPerMinute: 10, KeyPrefix: "a"}
	prefixB := RateLimiterConfig{RequestsPerMinute: 20, KeyPrefix: "b"}

	r := gin.New()
	r.GET("/a", RateLimitByIPSimple(nil, prefixA), func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/b", RateLimitByIPSimple(nil, prefixB), func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("X-RateLimit-Limit") != "10" {
		t.Errorf("expected 10 for /a, got %s", w.Header().Get("X-RateLimit-Limit"))
	}

	req2 := httptest.NewRequest(http.MethodGet, "/b", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Header().Get("X-RateLimit-Limit") != "20" {
		t.Errorf("expected 20 for /b, got %s", w2.Header().Get("X-RateLimit-Limit"))
	}
}