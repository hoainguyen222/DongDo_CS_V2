package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── WSCheckOrigin tests ─────────────────────────────────────────────────────

func TestWSCheckOrigin_EmptyOrigin_Allowed(t *testing.T) {
	check := WSCheckOrigin([]string{"https://allowed.com"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !check(req) {
		t.Error("expected empty origin (non-browser) to be allowed")
	}
}

func TestWSCheckOrigin_AllowedOrigin(t *testing.T) {
	check := WSCheckOrigin([]string{"https://allowed.com"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://allowed.com")
	if !check(req) {
		t.Error("expected whitelisted origin to be allowed")
	}
}

func TestWSCheckOrigin_DisallowedOrigin(t *testing.T) {
	check := WSCheckOrigin([]string{"https://allowed.com"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	if check(req) {
		t.Error("expected non-whitelisted origin to be rejected")
	}
}

func TestWSCheckOrigin_MultipleAllowedOrigins(t *testing.T) {
	check := WSCheckOrigin([]string{"https://a.com", "https://b.com"})
	for _, origin := range []string{"https://a.com", "https://b.com"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", origin)
		if !check(req) {
			t.Errorf("expected %s to be allowed", origin)
		}
	}
}

func TestWSCheckOrigin_EmptyWhitelist(t *testing.T) {
	// When no origins configured, only non-browser requests (no Origin header) allowed.
	check := WSCheckOrigin([]string{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://a.com")
	if check(req) {
		t.Error("expected request to be rejected when whitelist is empty")
	}
	// But empty Origin (non-browser) still allowed
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	if !check(req2) {
		t.Error("expected non-browser request to be allowed even with empty whitelist")
	}
}

// ─── StaffUpgrader / CustomerUpgrader tests ─────────────────────────────────

func TestStaffUpgrader_OriginWhitelist(t *testing.T) {
	u := StaffUpgrader([]string{"https://example.com"})
	if u.CheckOrigin == nil {
		t.Fatal("expected non-nil CheckOrigin")
	}
	// Allowed origin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	if !u.CheckOrigin(req) {
		t.Error("expected whitelisted origin allowed")
	}
	// Disallowed origin
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://evil.com")
	if u.CheckOrigin(req2) {
		t.Error("expected non-whitelisted origin rejected")
	}
}

func TestCustomerUpgrader_OriginWhitelist(t *testing.T) {
	u := CustomerUpgrader([]string{"https://chat.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://chat.example.com")
	if !u.CheckOrigin(req) {
		t.Error("expected whitelisted origin allowed for customer WS")
	}
}

// ─── Upgrader buffer sizes ──────────────────────────────────────────────────

func TestUpgrader_BufferSizes(t *testing.T) {
	u := StaffUpgrader([]string{"https://example.com"})
	if u.ReadBufferSize != 1024 {
		t.Errorf("expected ReadBufferSize=1024, got %d", u.ReadBufferSize)
	}
	if u.WriteBufferSize != 1024 {
		t.Errorf("expected WriteBufferSize=1024, got %d", u.WriteBufferSize)
	}
}