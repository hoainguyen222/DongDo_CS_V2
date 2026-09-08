// Package observability provides Prometheus instrumentation, pprof debug
// endpoints and structured logging hooks for the CSKH server.
//
// The package follows Clean-Architecture: it is a thin transport/utility layer
// that other layers (delivery, infrastructure, worker) call into. It does not
// own business state and does not depend on use-case code.
//
// All collectors register themselves against the package-level Registry
// exposed by Default() so a single /metrics endpoint can serve them all.
//
// Cardinality discipline:
//   - HTTP route labels come from gin's FullPath() (templated route, not raw URL).
//   - WS labels distinguish session/user role buckets only.
//   - Business labels are enum-like (case status, agent state).
package observability

import (
	"regexp"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	defaultOnce     sync.Once
	defaultRegistry *prometheus.Registry

	wsMetricsOnce sync.Once
	wsMetricsInst *WSMetrics
)

// SetWSMetrics installs the package-level WSMetrics instance used by the
// websocket delivery package. Calling it more than once is a no-op so
// tests that wire metrics independently don't race.
//
// The websocket package calls OnConnect/OnDisconnect/OnMessageReceived/
// OnMessageSent through the package-level helpers below so we don't have to
// thread a *WSMetrics through every constructor.
func SetWSMetrics(m *WSMetrics) {
	wsMetricsOnce.Do(func() { wsMetricsInst = m })
}

// WS returns the installed WSMetrics (may be nil if observability was not
// initialised).
func WS() *WSMetrics { return wsMetricsInst }

// Default returns the process-wide Prometheus registry. The first call wires
// the standard Go runtime + process collectors; subsequent calls return the
// same instance.
func Default() *prometheus.Registry {
	defaultOnce.Do(func() {
		defaultRegistry = prometheus.NewRegistry()

		// Built-in Go runtime metrics. Available on Go 1.21+; we are on
		// Go 1.26 so the *_latest.go file is always compiled in.
		// MetricsAll matches every runtime/metrics entry; it includes
		// goroutines, heap, GC, threads, mutex/block waits, etc.
		allMetrics := regexp.MustCompile("/.*")
		defaultRegistry.MustRegister(
			collectors.NewGoCollector(
				collectors.WithGoCollectorRuntimeMetrics(
					collectors.GoRuntimeMetricsRule{Matcher: allMetrics},
				),
			),
		)

		// Process-level metrics: CPU seconds, resident memory, FDs, opens.
		defaultRegistry.MustRegister(collectors.NewProcessCollector(
			collectors.ProcessCollectorOpts{Namespace: "process"},
		))
	})
	return defaultRegistry
}

// MustRegister registers collectors against Default(). Panics on duplicate —
// which is the right signal during boot.
func MustRegister(cs ...prometheus.Collector) {
	Default().MustRegister(cs...)
}
