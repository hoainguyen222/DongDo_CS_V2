package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// WSMetrics holds WebSocket-related collectors. Counters are goroutine-safe
// (Prometheus counters use atomic adds internally).
//
// Role labels are coarse on purpose:
//   - role = "guest" | "staff" | "admin" | "unknown"
//   - channel = the broadcast target ("admin_inbox" or a session_id) — used
//     only for the message counters where cardinality is bounded by the
//     number of currently open conversations, not by the URL space.
//   - event_type = enum from internal/domain.WSEventType — finite set.
type WSMetrics struct {
	ActiveConnections    *prometheus.GaugeVec
	ConnectionsTotal     *prometheus.CounterVec
	MessagesReceived     *prometheus.CounterVec
	MessagesSent         *prometheus.CounterVec
	ErrorsTotal          *prometheus.CounterVec
	BufferedDroppedTotal *prometheus.CounterVec
	MessageDuration      *prometheus.HistogramVec
	ConnectedClients     prometheus.Gauge
}

// NewWSMetrics registers and returns the WS collectors.
func NewWSMetrics() *WSMetrics {
	m := &WSMetrics{
		ActiveConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ws",
				Name:      "active_connections",
				Help:      "Number of currently connected WebSocket clients by role.",
			},
			[]string{"role"},
		),
		ConnectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ws",
				Name:      "connections_total",
				Help:      "Total WebSocket connections opened, by role.",
			},
			[]string{"role"},
		),
		MessagesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ws",
				Name:      "messages_received_total",
				Help:      "Total WebSocket messages received from clients, by event type.",
			},
			[]string{"event_type"},
		),
		MessagesSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ws",
				Name:      "messages_sent_total",
				Help:      "Total WebSocket messages broadcast by the server, by event type and channel.",
			},
			[]string{"event_type", "channel"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ws",
				Name:      "errors_total",
				Help:      "Total WebSocket errors, by phase (read|write|upgrade|json).",
			},
			[]string{"phase"},
		),
		BufferedDroppedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ws",
				Name:      "buffered_dropped_total",
				Help:      "Total WS events dropped because a client's send buffer was full (per-session soft-drop policy).",
			},
			[]string{"channel"},
		),
		MessageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ws",
				Name:      "message_duration_seconds",
				Help:      "Time spent serializing+writing a WebSocket batch to the client.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"event_type"},
		),
		ConnectedClients: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ws",
			Name:      "connected_clients",
			Help:      "Total number of currently connected WebSocket clients (any role).",
		}),
	}
	MustRegister(
		m.ActiveConnections,
		m.ConnectionsTotal,
		m.MessagesReceived,
		m.MessagesSent,
		m.ErrorsTotal,
		m.BufferedDroppedTotal,
		m.MessageDuration,
		m.ConnectedClients,
	)
	return m
}

// NormalizeRole maps arbitrary user_role strings from the WS query param into
// a bounded label set so we never explode cardinality with attacker-controlled
// values.
func NormalizeRole(role string) string {
	switch role {
	case "guest", "cskh", "admin", "leader", "owner":
		return role
	default:
		return "unknown"
	}
}

// OnConnect increments connection counters and tracks active gauge.
func (m *WSMetrics) OnConnect(role string) {
	if m == nil {
		return
	}
	r := NormalizeRole(role)
	m.ConnectionsTotal.WithLabelValues(r).Inc()
	m.ActiveConnections.WithLabelValues(r).Inc()
	m.ConnectedClients.Inc()
}

// OnDisconnect decrements active gauge.
func (m *WSMetrics) OnDisconnect(role string) {
	if m == nil {
		return
	}
	r := NormalizeRole(role)
	m.ActiveConnections.WithLabelValues(r).Dec()
	m.ConnectedClients.Dec()
}

// OnMessageReceived increments the receive counter for an event type.
func (m *WSMetrics) OnMessageReceived(eventType string) {
	if m == nil {
		return
	}
	m.MessagesReceived.WithLabelValues(eventType).Inc()
}

// OnMessageSent increments the send counter.
func (m *WSMetrics) OnMessageSent(eventType, channel string) {
	if m == nil {
		return
	}
	if channel == "" {
		channel = "unknown"
	}
	m.MessagesSent.WithLabelValues(eventType, channel).Inc()
}

// OnBufferedDrop counts a soft-drop in the hub (client buffer full).
func (m *WSMetrics) OnBufferedDrop(channel string) {
	if m == nil {
		return
	}
	if channel == "" {
		channel = "unknown"
	}
	m.BufferedDroppedTotal.WithLabelValues(channel).Inc()
}

// OnError classifies an error phase.
func (m *WSMetrics) OnError(phase string) {
	if m == nil {
		return
	}
	m.ErrorsTotal.WithLabelValues(phase).Inc()
}

// ObserveWriteDuration records the duration of a write batch.
func (m *WSMetrics) ObserveWriteDuration(eventType string, secs float64) {
	if m == nil {
		return
	}
	m.MessageDuration.WithLabelValues(eventType).Observe(secs)
}
