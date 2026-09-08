package observability

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPMetrics holds the HTTP-related collectors. The struct lets the same
// instance be shared across the Gin middleware and the /metrics handler.
type HTTPMetrics struct {
	RequestsTotal   *prometheus.CounterVec   // method, route, status
	RequestDuration *prometheus.HistogramVec // method, route
	Inflight        prometheus.Gauge         // gauge of active requests
	RequestSize     *prometheus.HistogramVec // optional, bytes
	ResponseSize    *prometheus.HistogramVec // optional, bytes
}

// NewHTTPMetrics registers and returns the HTTP collectors.
func NewHTTPMetrics() *HTTPMetrics {
	m := &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests handled, labeled by method, route template and status code.",
			},
			[]string{"method", "route", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request latency in seconds. Use histogram_quantile() to compute P50/P95/P99.",
				Buckets: []float64{
					0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
				},
			},
			[]string{"method", "route"},
		),
		Inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being handled.",
		}),
		RequestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Name:      "request_size_bytes",
				Help:      "HTTP request body size in bytes.",
				Buckets:   prometheus.ExponentialBuckets(100, 4, 7),
			},
			[]string{"method", "route"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "http",
				Name:      "response_size_bytes",
				Help:      "HTTP response body size in bytes.",
				Buckets:   prometheus.ExponentialBuckets(100, 4, 7),
			},
			[]string{"method", "route"},
		),
	}
	MustRegister(m.RequestsTotal, m.RequestDuration, m.Inflight, m.RequestSize, m.ResponseSize)
	return m
}

// GinMiddleware returns a Gin middleware that records the standard HTTP
// metrics. It uses c.FullPath() as the route label to keep cardinality bounded
// — raw URL paths with IDs are NEVER used as labels.
//
// The middleware is a no-op when m is nil so it's safe to wire before the
// observability subsystem has booted in tests.
func (m *HTTPMetrics) GinMiddleware() gin.HandlerFunc {
	if m == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()

		// Increment inflight AFTER context resolution but before handlers run.
		m.Inflight.Inc()
		defer m.Inflight.Dec()

		// Run the chain.
		c.Next()

		// Resolve route label. FullPath returns "" for unmatched routes; in
		// that case fall back to a coarse "unmatched" bucket so 404s still
		// have a bounded cardinality slot.
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		elapsed := time.Since(start).Seconds()

		m.RequestsTotal.WithLabelValues(method, route, status).Inc()
		m.RequestDuration.WithLabelValues(method, route).Observe(elapsed)

		if c.Request.ContentLength > 0 {
			m.RequestSize.WithLabelValues(method, route).Observe(float64(c.Request.ContentLength))
		}
		if c.Writer.Size() > 0 {
			m.ResponseSize.WithLabelValues(method, route).Observe(float64(c.Writer.Size()))
		}
	}
}

// MetricsServer is a standalone HTTP server that exposes the Prometheus
// scrape endpoint. It runs on its own port (MetricsAddr) so the public
// application port never has to expose /metrics.
type MetricsServer struct {
	srv *http.Server
}

// NewMetricsServer wires the /metrics handler and /healthz probe.
//
// addr must be in host:port form. An empty addr disables the server (returns
// nil). Caller is expected to call Start + Wait/Shutdown.
func NewMetricsServer(addr string) *MetricsServer {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(Default(), promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          Default(),
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &MetricsServer{
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

// Start runs the metrics server until ctx is cancelled, then performs a
// graceful shutdown. Returns nil on graceful shutdown.
func (s *MetricsServer) Start(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
