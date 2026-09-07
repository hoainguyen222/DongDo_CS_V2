package asterisk_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/infrastructure/asterisk"
	"github.com/rs/zerolog"
)

// TestMockGateway_HappyPath: originate customer + agent, create bridge, add channels.
func TestMockGateway_HappyPath(t *testing.T) {
	mgw := asterisk.NewMockGateway(zerolog.Nop())
	ctx := context.Background()
	callID := uuid.New()

	ch1, err := mgw.OriginateCustomer(ctx, callID, "PJSIP/cust-x")
	if err != nil || ch1 == "" {
		t.Fatalf("originate customer: ch=%q err=%v", ch1, err)
	}
	ch2, err := mgw.OriginateAgent(ctx, callID, "agent-1")
	if err != nil || ch2 == "" {
		t.Fatalf("originate agent: ch=%q err=%v", ch2, err)
	}

	bridge, err := mgw.CreateBridge(ctx, "mixing")
	if err != nil || bridge == "" {
		t.Fatalf("create bridge: id=%q err=%v", bridge, err)
	}
	if err := mgw.BridgeChannels(ctx, bridge, ch1, ch2); err != nil {
		t.Fatalf("bridge channels: %v", err)
	}

	if err := mgw.Hangup(ctx, callID); err != nil {
		t.Fatalf("hangup: %v", err)
	}
	if err := mgw.HealthCheck(ctx); err != nil {
		t.Errorf("healthcheck: %v", err)
	}
}

// TestMockGateway_Unhealthy: when healthy=false, every Originate must fail.
func TestMockGateway_Unhealthy(t *testing.T) {
	mgw := asterisk.NewMockGateway(zerolog.Nop())
	mgw.SetHealthy(false)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := mgw.OriginateCustomer(ctx, uuid.New(), "PJSIP/x"); err == nil {
		t.Errorf("expected ErrAsteriskUnavailable from originate when unhealthy")
	}
	if err := mgw.HealthCheck(ctx); err == nil {
		t.Errorf("expected healthcheck to fail when unhealthy")
	}
}

// TestMockWSConsumer_PushesEvents verifies the mock consumer hands events to the handler.
func TestMockWSConsumer_PushesEvents(t *testing.T) {
	consumer := asterisk.NewMockWSConsumer()

	received := make(chan domain.ARIEvent, 4)
	consumer.SetHandler(func(_ context.Context, ev domain.ARIEvent) {
		received <- ev
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = consumer.Start(ctx)
		close(done)
	}()

	callID := uuid.New()
	consumer.PushStasisStart(callID, "ch-1")
	consumer.PushChannelAnswered(callID, "ch-1")
	consumer.PushStasisEnd(callID)

	deadline := time.After(500 * time.Millisecond)
	count := 0
	for count < 3 {
		select {
		case <-received:
			count++
		case <-deadline:
			t.Fatalf("timeout: got %d events", count)
		}
	}

	cancel()
	<-done
}

// Compile-time assertion: mock implements the domain interface.
var _ domain.AsteriskGateway = (*asterisk.MockGateway)(nil)
