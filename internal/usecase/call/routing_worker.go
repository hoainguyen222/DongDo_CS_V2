package call

import (
	"context"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// RoutingWorker drains the call queue and asks the use case to find an agent.
// One worker per process. Wakes up on routerSignal or every PollInterval.
type RoutingWorker struct {
	uc     *UseCase
	logger zerolog.Logger

	PollInterval time.Duration
}

// NewRoutingWorker is the public constructor.
func NewRoutingWorker(uc *UseCase, logger zerolog.Logger, poll time.Duration) *RoutingWorker {
	return &RoutingWorker{uc: uc, logger: logger, PollInterval: poll}
}

// Start blocks until ctx is cancelled.
func (w *RoutingWorker) Start(ctx context.Context) {
	if w.PollInterval <= 0 {
		w.PollInterval = 200 * time.Millisecond
	}
	logger := w.logger.With().Str("worker", "call_routing").Logger()
	logger.Info().Msg("call routing worker started")

	t := time.NewTicker(w.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("call routing worker stopped")
			return
		case <-RouterSignal():
			w.pump(ctx, logger)
		case <-t.C:
			w.pump(ctx, logger)
		}
	}
}

func (w *RoutingWorker) pump(ctx context.Context, logger zerolog.Logger) {
	for {
		// Bound the number of attempts per tick to avoid starvation.
		cctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err := w.uc.TryRouteNext(cctx)
		cancel()
		if err != nil {
			logger.Warn().Err(err).Msg("TryRouteNext failed")
			return
		}
		// If queue is empty, exit the pump and wait for the next wake-up.
		size, _ := w.uc.queue.QueueSize(ctx)
		if size == 0 {
			return
		}
	}
}

// ReconciliationWorker periodically reconciles agent/call state with PG.
type ReconciliationWorker struct {
	uc     *UseCase
	logger zerolog.Logger

	Interval time.Duration
}

// NewReconciliationWorker is the public constructor.
func NewReconciliationWorker(uc *UseCase, logger zerolog.Logger, interval time.Duration) *ReconciliationWorker {
	return &ReconciliationWorker{uc: uc, logger: logger, Interval: interval}
}

// Start blocks until ctx is cancelled.
func (r *ReconciliationWorker) Start(ctx context.Context) {
	if r.Interval <= 0 {
		r.Interval = 30 * time.Second
	}
	logger := r.logger.With().Str("worker", "call_reconcile").Logger()
	logger.Info().Dur("interval", r.Interval).Msg("call reconciliation worker started")

	t := time.NewTicker(r.Interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("call reconciliation worker stopped")
			return
		case <-t.C:
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := r.uc.Reconcile(cctx); err != nil {
				logger.Warn().Err(err).Msg("reconcile pass failed")
			}
			cancel()
		}
	}
}

// agentSubscription is a helper to read the current AgentState of an agent
// through the use case.
func (uc *UseCase) GetAgentSnapshot(ctx context.Context, agentID string) (*domain.Agent, error) {
	return uc.queue.GetAgent(ctx, agentID)
}
