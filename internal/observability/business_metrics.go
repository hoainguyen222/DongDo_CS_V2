package observability

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// BusinessMetrics holds the gauges that describe live business state.
//
// All collectors are polled periodically (BusinessPollInterval) by a goroutine
// started from cmd/server/main.go. They are not updated synchronously inside
// the hot path — that would add latency to chat messages and call signaling.
type BusinessMetrics struct {
	// Chat / CSKH
	ChatActiveConversations *prometheus.GaugeVec // by status (AI_ACTIVE, HUMAN_CS_ACTIVE, ...)
	CustomerWaiting         prometheus.Gauge     // unassigned cases needing human
	AgentOnline             prometheus.Gauge     // WS-connected staff (any role)
	AgentAvailable          prometheus.Gauge     // call-service: agents:available set
	AgentBusy               prometheus.Gauge     // call-service: agent:<id>:state=RESERVED/IN_CALL

	// Redis Streams
	StreamLength    *prometheus.GaugeVec // per-stream length (XLEN)
	StreamPending   *prometheus.GaugeVec // per-stream pending count (XPENDING)
	StreamConsumers *prometheus.GaugeVec // per-stream consumer count (XINFO CONSUMERS)

	// Postgres pool
	DBPoolAcquired *prometheus.GaugeVec
	DBPoolIdle     *prometheus.GaugeVec
	DBPoolTotal    *prometheus.GaugeVec

	// Redis client pool
	RedisPoolHits   prometheus.Gauge
	RedisPoolMisses prometheus.Gauge
	RedisPoolTotal  prometheus.Gauge
	RedisPoolIdle   prometheus.Gauge
	RedisPoolStale  prometheus.Gauge
}

// NewBusinessMetrics registers and returns the business collectors.
func NewBusinessMetrics() *BusinessMetrics {
	m := &BusinessMetrics{
		ChatActiveConversations: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "chat",
				Name:      "active_conversations",
				Help:      "Number of chat cases currently in each status.",
			},
			[]string{"status"},
		),
		CustomerWaiting: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chat",
			Name:      "customer_waiting",
			Help:      "Number of customers waiting to be picked up by a CSKH (NEEDS_HUMAN_CS and unassigned).",
		}),
		AgentOnline: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "agent",
			Name:      "online",
			Help:      "Number of CSKH staff currently holding an open WebSocket connection.",
		}),
		AgentAvailable: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "agent",
			Name:      "available",
			Help:      "Number of CSKH agents currently available for new calls (call-service Redis set).",
		}),
		AgentBusy: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "agent",
			Name:      "busy",
			Help:      "Number of CSKH agents currently in a call or otherwise unavailable.",
		}),
		StreamLength: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "redis",
				Subsystem: "stream",
				Name:      "length",
				Help:      "Approximate number of entries in each Redis stream.",
			},
			[]string{"stream"},
		),
		StreamPending: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "redis",
				Subsystem: "stream",
				Name:      "pending",
				Help:      "Number of pending (un-acked) entries per Redis stream, per consumer group.",
			},
			[]string{"stream", "group"},
		),
		StreamConsumers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "redis",
				Subsystem: "stream",
				Name:      "consumers",
				Help:      "Number of consumers attached to each Redis stream's consumer group.",
			},
			[]string{"stream", "group"},
		),
		DBPoolAcquired: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "db",
				Subsystem: "pool",
				Name:      "acquired",
				Help:      "Number of currently acquired (in-use) PostgreSQL connections.",
			},
			[]string{"db"},
		),
		DBPoolIdle: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "db",
				Subsystem: "pool",
				Name:      "idle",
				Help:      "Number of currently idle PostgreSQL connections.",
			},
			[]string{"db"},
		),
		DBPoolTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "db",
				Subsystem: "pool",
				Name:      "total",
				Help:      "Total number of PostgreSQL connections in the pool.",
			},
			[]string{"db"},
		),
		RedisPoolHits: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "redis",
			Subsystem: "pool",
			Name:      "hits",
			Help:      "Cumulative number of times a free connection was found in the pool.",
		}),
		RedisPoolMisses: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "redis",
			Subsystem: "pool",
			Name:      "misses",
			Help:      "Cumulative number of times a free connection was NOT found in the pool.",
		}),
		RedisPoolTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "redis",
			Subsystem: "pool",
			Name:      "total_conns",
			Help:      "Number of total connections in the pool.",
		}),
		RedisPoolIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "redis",
			Subsystem: "pool",
			Name:      "idle_conns",
			Help:      "Number of idle connections in the pool.",
		}),
		RedisPoolStale: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "redis",
			Subsystem: "pool",
			Name:      "stale_conns",
			Help:      "Number of stale connections removed from the pool.",
		}),
	}
	MustRegister(
		m.ChatActiveConversations,
		m.CustomerWaiting,
		m.AgentOnline,
		m.AgentAvailable,
		m.AgentBusy,
		m.StreamLength,
		m.StreamPending,
		m.StreamConsumers,
		m.DBPoolAcquired,
		m.DBPoolIdle,
		m.DBPoolTotal,
		m.RedisPoolHits,
		m.RedisPoolMisses,
		m.RedisPoolTotal,
		m.RedisPoolIdle,
		m.RedisPoolStale,
	)
	return m
}

// BusinessSource is the narrow contract the polling goroutine needs. It is
// satisfied by *pgxpool.Pool and *redis.Client; tests can supply a fake.
type BusinessSource interface {
	Ping(ctx context.Context) error
}

// StartBusinessPoller runs until ctx is cancelled. It refreshes business
// gauges every interval. Errors are non-fatal — collectors stay at their last
// value so dashboards don't flap.
//
// pgPool may be nil (DB-independent gauges will still be updated).
// rdb may be nil (Redis-independent gauges will still be updated).
// staffOnline may be nil; if non-nil it must return the number of staff
// currently connected via WebSocket — used for the agent_online gauge.
//
// The streams argument is preserved for caller convenience; the poller
// already knows the canonical stream list, but callers may pass additional
// streams to track. Pass nil for default behaviour.
func (m *BusinessMetrics) StartBusinessPoller(
	ctx context.Context,
	interval time.Duration,
	pgPool *pgxpool.Pool,
	rdb *redis.Client,
	staffOnline func() int,
	streams []string,
) {
	if m == nil || interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	// Run once immediately so /metrics has values right after boot.
	_ = streams // reserved for future extension
	m.refresh(ctx, pgPool, rdb, staffOnline)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.refresh(ctx, pgPool, rdb, staffOnline)
		}
	}
}

func (m *BusinessMetrics) refresh(
	ctx context.Context,
	pgPool *pgxpool.Pool,
	rdb *redis.Client,
	staffOnline func() int,
) {
	if staffOnline != nil {
		m.AgentOnline.Set(float64(staffOnline()))
	}
	if pgPool != nil {
		m.refreshDB(ctx, pgPool)
	}
	if rdb != nil {
		m.refreshRedis(ctx, rdb, nil)
	}
}

func (m *BusinessMetrics) refreshDB(ctx context.Context, pgPool *pgxpool.Pool) {
	stat := pgPool.Stat()
	m.DBPoolAcquired.WithLabelValues("postgres").Set(float64(stat.AcquiredConns()))
	m.DBPoolIdle.WithLabelValues("postgres").Set(float64(stat.IdleConns()))
	m.DBPoolTotal.WithLabelValues("postgres").Set(float64(stat.TotalConns()))

	// Active conversation counts grouped by status. We use the existing
	// chat_cases table. The query is small and bounded (only 4 statuses).
	rows, err := pgPool.Query(ctx, `
		SELECT status::text, COUNT(*)
		FROM chat_cases
		WHERE status IN ('AI_ACTIVE','NEEDS_HUMAN_CS','HUMAN_CS_ACTIVE','RESOLVED')
		GROUP BY status
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	type kv struct {
		status string
		count  int64
	}
	seen := make(map[string]float64, 4)
	for rows.Next() {
		var k kv
		if err := rows.Scan(&k.status, &k.count); err == nil {
			seen[k.status] = float64(k.count)
		}
	}
	for _, s := range []string{"AI_ACTIVE", "NEEDS_HUMAN_CS", "HUMAN_CS_ACTIVE", "RESOLVED"} {
		m.ChatActiveConversations.WithLabelValues(s).Set(seen[s])
	}

	// Waiting = NEEDS_HUMAN_CS AND not assigned to any CSKH yet.
	var waiting int64
	_ = pgPool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM chat_cases
		WHERE status = 'NEEDS_HUMAN_CS'
		  AND (assigned_cs IS NULL OR assigned_cs = '')
	`).Scan(&waiting)
	m.CustomerWaiting.Set(float64(waiting))
}

func (m *BusinessMetrics) refreshRedis(ctx context.Context, rdb *redis.Client, _ []string) {
	// Pool stats: PoolStats() is a snapshot.
	ps := rdb.PoolStats()
	m.RedisPoolHits.Set(float64(ps.Hits))
	m.RedisPoolMisses.Set(float64(ps.Misses))
	m.RedisPoolTotal.Set(float64(ps.TotalConns))
	m.RedisPoolIdle.Set(float64(ps.IdleConns))
	m.RedisPoolStale.Set(float64(ps.StaleConns))

	// Stream gauges. Each stream has one or more consumer groups (ws_group,
	// ai_group, db_group). We report pending count and consumer count per
	// (stream, group) pair so a stuck consumer shows up immediately.
	streamGroups := map[string][]string{
		"stream:ws":  {"ws_group"},
		"stream:ai":  {"ai_group"},
		"stream:db":  {"db_group"},
		"stream:dlq": {""},
	}
	for s, groups := range streamGroups {
		if s == "" {
			continue
		}
		if n, err := rdb.XLen(ctx, s).Result(); err == nil {
			m.StreamLength.WithLabelValues(s).Set(float64(n))
		}
		for _, g := range groups {
			label := g
			if label == "" {
				label = "default"
			}
			res, err := rdb.XPending(ctx, s, g).Result()
			if err == nil {
				m.StreamPending.WithLabelValues(s, label).Set(float64(res.Count))
				m.StreamConsumers.WithLabelValues(s, label).Set(float64(len(res.Consumers)))
			}
		}
	}

	// Agent gauges (call-service Redis state)
	if n, err := rdb.SCard(ctx, "agents:available").Result(); err == nil {
		m.AgentAvailable.Set(float64(n))
	}
	// Busy = active reservations. We approximate by counting pending call
	// stream messages: every RINGING/ACTIVE call corresponds to one in-flight
	// reservation. This is good enough for SRE-level dashboards.
	if n, err := rdb.XLen(ctx, "stream:calls").Result(); err == nil {
		m.AgentBusy.Set(float64(n))
	}
}
