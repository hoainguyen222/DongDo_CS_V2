package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

// gather runs a Gather against the default registry.
func gather(reg *prometheus.Registry) ([]*dto.MetricFamily, error) {
	mfs, err := reg.Gather()
	return mfs, err
}

// promHandlerForTest returns a /metrics handler bound to Default().
func promHandlerForTest() http.Handler {
	return promhttp.HandlerFor(Default(), promhttp.HandlerOpts{Registry: Default()})
}

func TestHTTPMetrics_MiddlewareRecordsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := NewHTTPMetrics()

	r := gin.New()
	r.Use(m.GinMiddleware())
	r.GET("/users/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Scrape the registry and assert the counter has the right labels.
	rr := httptest.NewRecorder()
	Default().Gather() // ensure all collectors are gathered once
	srv := httptest.NewServer(promHandlerForTest())
	defer srv.Close()

	_ = rr // unused; we use the metrics package's own gather path

	// Use the metrics package's own gatherer (it is Default()).
	mfs, err := gather(Default())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	foundReq := false
	for _, mf := range mfs {
		if mf.GetName() == "http_requests_total" {
			for _, mv := range mf.Metric {
				route := ""
				status := ""
				for _, lbl := range mv.Label {
					switch lbl.GetName() {
					case "route":
						route = lbl.GetValue()
					case "status":
						status = lbl.GetValue()
					}
				}
				if route == "/users/:id" && status == "200" {
					foundReq = true
				}
			}
		}
	}
	if !foundReq {
		t.Fatalf("expected http_requests_total{route=/users/:id,status=200} to be present")
	}
}

func TestRequestID_MiddlewareSetsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/p", func(c *gin.Context) {
		c.String(http.StatusOK, RequestIDFromContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	id := w.Header().Get(RequestIDHeader)
	if id == "" {
		t.Fatal("expected request id header to be set")
	}
	if !strings.Contains(w.Body.String(), id) {
		t.Fatalf("expected body to contain request id %q, got %q", id, w.Body.String())
	}
}

func TestNormalizeRole(t *testing.T) {
	cases := map[string]string{
		"guest":  "guest",
		"cskh":   "cskh",
		"admin":  "admin",
		"leader": "leader",
		"owner":  "owner",
		"foo":    "unknown",
		"":       "unknown",
	}
	for in, want := range cases {
		if got := NormalizeRole(in); got != want {
			t.Errorf("NormalizeRole(%q)=%q want %q", in, got, want)
		}
	}
}
