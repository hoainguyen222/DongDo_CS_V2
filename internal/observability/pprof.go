package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"time"
)

// PprofServer exposes net/http/pprof on a dedicated port (PprofAddr).
//
// SECURITY: the server MUST be bound to a non-public address. The default in
// config.Observability is 127.0.0.1:6060 so a misconfigured production deploy
// does NOT expose CPU profiles, goroutine dumps, or heap snapshots.
//
// Recommended access from a workstation:
//
//	ssh -L 6060:127.0.0.1:6060 user@prod-host
//	go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
//
// Or run the server inside a Docker network and reach it from Prometheus's
// sidecar / side-pod — never via the public load balancer.
type PprofServer struct {
	srv *http.Server
}

// NewPprofServer wires /debug/pprof/* on addr. Returns nil if addr is empty.
func NewPprofServer(addr string) *PprofServer {
	if addr == "" {
		return nil
	}

	mux := http.NewServeMux()

	// The net/http/pprof init() function already registers handlers on
	// http.DefaultServeMux. We replicate them on our own mux so we can
	// bound the lifecycle to addr.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Convenience named routes (some pprof tooling expects the short form).
	mux.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	mux.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	mux.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	mux.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
	mux.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)

	// Lightweight liveness check so an orchestrator can verify the port
	// is open without dumping goroutine state.
	mux.HandleFunc("/debug/pprof/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &PprofServer{
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			// Pprof endpoints (especially /profile) are long-running.
			// We deliberately do not set a short WriteTimeout here.
		},
	}
}

// Start runs the pprof server until ctx is cancelled, then performs a
// graceful shutdown. Returns nil on graceful shutdown.
func (s *PprofServer) Start(ctx context.Context) error {
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

// Addr returns the listen address. Useful for tests that need to pick a free
// port. Returns "" if the server is not configured.
func (s *PprofServer) Addr() string {
	if s == nil || s.srv == nil {
		return ""
	}
	return s.srv.Addr
}
