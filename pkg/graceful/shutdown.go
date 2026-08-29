package graceful

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// CleanupFunc is a function called during shutdown.
type CleanupFunc func(ctx context.Context) error

// ShutdownManager manages graceful shutdown of background services and connections.
type ShutdownManager struct {
	cleanups []namedCleanup
	mu       sync.Mutex
	timeout  time.Duration
}

type namedCleanup struct {
	name string
	fn   CleanupFunc
}

// NewShutdownManager creates a new ShutdownManager with a default timeout.
func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &ShutdownManager{
		cleanups: make([]namedCleanup, 0),
		timeout:  timeout,
	}
}

// Register registers a cleanup function with a descriptive name.
func (sm *ShutdownManager) Register(name string, fn CleanupFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cleanups = append(sm.cleanups, namedCleanup{name: name, fn: fn})
}

// WaitForSignal blocks until an interrupt or termination signal is received,
// then executes all registered cleanups in reverse order of registration.
func (sm *ShutdownManager) WaitForSignal(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigChan:
		log.Printf("🛑 Received shutdown signal: %v. Initiating graceful shutdown...", sig)
	case <-ctx.Done():
		log.Printf("🛑 Context canceled. Initiating graceful shutdown...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), sm.timeout)
	defer cancel()

	sm.executeCleanups(shutdownCtx)
}

func (sm *ShutdownManager) executeCleanups(ctx context.Context) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Execute in reverse order of registration
	for i := len(sm.cleanups) - 1; i >= 0; i-- {
		c := sm.cleanups[i]
		log.Printf("⏳ Shutting down: %s...", c.name)

		done := make(chan error, 1)
		go func(fn CleanupFunc) {
			done <- fn(ctx)
		}(c.fn)

		select {
		case err := <-done:
			if err != nil {
				log.Printf("⚠️ Error shutting down %s: %v", c.name, err)
			} else {
				log.Printf("✅ Shutdown completed: %s", c.name)
			}
		case <-ctx.Done():
			log.Printf("❌ Timeout shutting down %s", c.name)
		}
	}
	log.Println("👋 All services gracefully shut down.")
}
