package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	deliveryHTTP "github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/http"
	deliveryWS "github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraClaude "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/claude"
	infraEmbedding "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/embedding"
	infraQdrant "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/qdrant"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/observability"
	repoPostgres "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/postgres"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/worker"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/graceful"

	redisGo "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	serviceName    = "dongdo-cs-server"
	serviceVersion = "2.0.0"
)

func init() {
	// Configure zerolog for production
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	// Initialize logger with structured fields
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Str("service", serviceName).
		Str("version", serviceVersion).
		Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()
	serverAddr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)

	logger.Info().
		Str("address", serverAddr).
		Str("service", serviceName).
		Str("version", serviceVersion).
		Str("go_version", runtime.Version()).
		Msg("Server starting")

	sm := graceful.NewShutdownManager(15 * time.Second)

	// 1. Initialize Database (PostgreSQL only)
	var (
		userRepo      domain.UserRepository
		sessionRepo   domain.SessionRepository
		guestRepo     domain.GuestRepository
		messageRepo   domain.MessageRepository
		caseRepo      domain.CaseRepository
		learningRepo  domain.LearningRepository
		settingRepo   domain.SettingRepository
		voiceRepo     domain.VoiceCallRepository
		analyticsRepo domain.AnalyticsRepository
		partnerRepo   domain.PartnerRepository
		tagRepo       domain.ChatTagRepository
	)

	var pgDB *repoPostgres.DB
	for attempt := 1; attempt <= 15; attempt++ {
		var err error
		pgDB, err = repoPostgres.NewDB(ctx, cfg.DatabaseURL)
		if err == nil {
			break
		}
		if attempt == 1 {
			logger.Warn().
				Str("type", "postgresql").
				Err(err).
				Msg("Waiting for PostgreSQL")
		}
		if attempt == 15 {
			logger.Fatal().
				Str("type", "postgresql").
				Err(err).
				Msg("PostgreSQL connection failed after all retries")
		}
		time.Sleep(1 * time.Second)
	}

	sm.Register("PostgreSQL Connection Pool", func(ctx context.Context) error {
		pgDB.Close()
		return nil
	})
	userRepo = repoPostgres.NewUserRepo(pgDB)
	sessionRepo = repoPostgres.NewSessionRepo(pgDB)
	guestRepo = repoPostgres.NewGuestRepo(pgDB)
	messageRepo = repoPostgres.NewMessageRepo(pgDB)
	caseRepo = repoPostgres.NewCaseRepo(pgDB)
	learningRepo = repoPostgres.NewLearningRepo(pgDB)
	settingRepo = repoPostgres.NewSettingRepo(pgDB)
	voiceRepo = repoPostgres.NewVoiceCallRepo(pgDB)
	analyticsRepo = repoPostgres.NewAnalyticsRepo(pgDB)
	partnerRepo = repoPostgres.NewPartnerRepo(pgDB)
	tagRepo = repoPostgres.NewChatTagRepo(pgDB)

	logger.Info().
		Str("type", "postgresql").
		Msg("Database connected")

	// 2. Initialize Redis (Event Bus & State) with config-based connection pool
	var eventBus domain.EventBus = infraRedis.NewNoOpEventBus()
	var stateMgr domain.StateManager = infraRedis.NewNoOpStateManager()
	var redisClient *infraRedis.Client
	var streamEventBus *infraRedis.EventBusService

	if cfg.RedisURL != "" {
		var err error
		for attempt := 1; attempt <= 15; attempt++ {
			redisClient, err = infraRedis.NewClientWithConfig(cfg.RedisURL, cfg.Worker.RedisPool)
			if err == nil {
				break
			}
			if attempt == 1 {
				logger.Warn().
					Err(err).
					Msg("Waiting for Redis")
			}
			if attempt == 15 {
				logger.Error().
					Err(err).
					Msg("Redis connection failed after all retries; running with NoOp fallback")
			}
			time.Sleep(1 * time.Second)
		}

		if err != nil {
			eventBus = infraRedis.NewNoOpEventBus()
			stateMgr = infraRedis.NewNoOpStateManager()
		} else {
			// Create EventBus with configured stream max length
			streamEventBus = infraRedis.NewEventBusWithConfig(redisClient, cfg.Worker.Streams.MaxLen)
			eventBus = streamEventBus
			stateMgr = infraRedis.NewStateManager(redisClient)
			sm.Register("Redis Connection", func(ctx context.Context) error {
				return redisClient.Close()
			})
			logger.Info().
				Str("url", cfg.RedisURL).
				Int("pool_size", cfg.Worker.RedisPool.PoolSize).
				Int64("stream_max_len", cfg.Worker.Streams.MaxLen).
				Msg("Redis connected")
		}
	} else {
		logger.Warn().
			Msg("Redis URL not configured; using NoOp event bus and state manager")
	}

	// 3. Initialize Qdrant Vector DB
	qdrantClient, err := infraQdrant.NewClient(ctx, cfg.QdrantHost, cfg.QdrantPort, 384)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("Qdrant unavailable; RAG will run in fallback mode")
	} else {
		sm.Register("Qdrant gRPC Connection", func(ctx context.Context) error {
			qdrantClient.Close()
			return nil
		})
		logger.Info().
			Str("host", cfg.QdrantHost).
			Int("port", cfg.QdrantPort).
			Msg("Qdrant connected")
	}

	// 4. Initialize Embedder & Claude LLM Client
	embedder := infraEmbedding.NewEmbedder(cfg.EmbeddingModel)
	claudeClient := infraClaude.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicWorkspaceID, cfg.OpenAIAPIKey, cfg.GeminiAPIKey, cfg.LLMModel, cfg.LLMTemperature)

	// 5. Initialize Use Cases
	authUC := usecase.NewAuthUseCase(userRepo, sessionRepo, guestRepo)
	ragUC := usecase.NewRAGUseCase(qdrantClient, embedder, claudeClient, messageRepo, settingRepo, cfg.SystemPrompt, cfg.MemoryWindow, cfg.RetrieverK)
	caseUC := usecase.NewCaseUseCase(guestRepo, caseRepo, messageRepo, learningRepo, settingRepo, qdrantClient, embedder, eventBus)
	chatUC := usecase.NewChatUseCase(messageRepo, caseRepo, eventBus, stateMgr)
	learningUC := usecase.NewLearningUseCase(learningRepo, settingRepo, qdrantClient, embedder, eventBus)
	voiceUC := usecase.NewVoiceUseCase(voiceRepo, caseRepo, eventBus)
	analyticsUC := usecase.NewAnalyticsUseCase(analyticsRepo, settingRepo)
	partnerUC := usecase.NewPartnerUseCase(partnerRepo, settingRepo)
	tagUC := usecase.NewChatTagUseCase(tagRepo)

	// 7. Initialize WebSocket Hub
	hub := deliveryWS.NewHub()
	go hub.Run()
	eventBus.SetHub(hub)

	// 8. Start Background Workers if Redis Streams is available
	// All workers now use centralized config from cfg.Worker
	startedWorkers := []string{}
	if streamEventBus != nil {
		// WS Worker - broadcasts WebSocket events
		wsWorker := worker.NewWSWorker(
			streamEventBus,
			hub,
			"ws_worker_1",
			cfg.Worker.WS,
		)
		go wsWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "ws_worker_1")

		// AI Worker - processes RAG queries
		aiWorker := worker.NewAIWorker(
			streamEventBus,
			stateMgr,
			ragUC,
			messageRepo,
			caseRepo,
			"ai_worker_1",
			cfg.Worker.AI,
		)
		go aiWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "ai_worker_1")

		// DB Worker - batch writes to PostgreSQL
		dbWorker := worker.NewDBWorker(
			streamEventBus,
			messageRepo,
			"db_worker_1",
			cfg.Worker.DB,
		)
		go dbWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "db_worker_1")

		// Retry Worker - handles dead letter queue
		retryWorker := worker.NewRetryWorker(
			streamEventBus,
			"retry_worker_1",
			cfg.Worker.Retry,
		)
		go retryWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "retry_worker_1")

		logger.Info().
			Strs("workers", startedWorkers).
			Int("batch_size", cfg.Worker.DB.BatchSize).
			Dur("flush_interval", cfg.Worker.DB.FlushInterval).
			Int("ws_read_count", int(cfg.Worker.WS.ReadCount)).
			Int64("ai_read_count", cfg.Worker.AI.ReadCount).
			Int("max_retries", cfg.Worker.Retry.MaxRetries).
			Msg("Workers started with config")
	} else {
		logger.Warn().
			Msg("Redis not available; background workers not started")
	}

	// 9. Initialize observability (Prometheus + pprof + business poller)
	httpMetrics := observability.NewHTTPMetrics()
	wsMetrics := observability.NewWSMetrics()
	businessMetrics := observability.NewBusinessMetrics()

	var metricsServer *observability.MetricsServer
	var pprofServer *observability.PprofServer
	if cfg.Observability.MetricsEnabled {
		metricsServer = observability.NewMetricsServer(cfg.Observability.MetricsAddr)
		sm.Register("Prometheus Metrics Server", func(ctx context.Context) error {
			// Use a fresh context bounded by the parent's lifetime for
			// shutdown. MetricsServer.Start already handles ctx.Done, so we
			// just cancel a dedicated ctx here.
			return nil
		})
		go func() {
			if err := metricsServer.Start(ctx); err != nil {
				logger.Warn().Err(err).Msg("Metrics server exited with error")
			}
		}()
		logger.Info().
			Str("address", cfg.Observability.MetricsAddr).
			Msg("Prometheus /metrics endpoint listening")
	}

	if cfg.Observability.PprofEnabled {
		pprofServer = observability.NewPprofServer(cfg.Observability.PprofAddr)
		go func() {
			if err := pprofServer.Start(ctx); err != nil {
				logger.Warn().Err(err).Msg("pprof server exited with error")
			}
		}()
		logger.Info().
			Str("address", cfg.Observability.PprofAddr).
			Msg("pprof debug server listening")
	}

	if cfg.Observability.BusinessEnabled {
		var rdb = redisClientForMetrics(redisClient)
		hubRef := hub
		go businessMetrics.StartBusinessPoller(
			ctx,
			cfg.Observability.BusinessPollInterval,
			pgDB.Pool,
			rdb,
			func() int { return hubRef.OnlineStaffCount() },
			nil,
		)
		logger.Info().
			Dur("interval", cfg.Observability.BusinessPollInterval).
			Msg("Business metrics poller started")
	}

	// 10. Initialize HTTP Router
	handler := deliveryHTTP.NewHandler(
		authUC,
		chatUC,
		caseUC,
		learningUC,
		voiceUC,
		analyticsUC,
		partnerUC,
		ragUC,
		tagUC,
		qdrantClient,
		embedder,
		cfg.DocumentsDir,
		eventBus,
	)

	router := deliveryHTTP.SetupRouter(handler, hub, chatUC, voiceUC, stateMgr, eventBus, authUC, httpMetrics, wsMetrics)

	// 10. Start HTTP Server with configured timeouts
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.Worker.HTTP.ReadTimeout,
		WriteTimeout: cfg.Worker.HTTP.WriteTimeout,
		IdleTimeout:  cfg.Worker.HTTP.IdleTimeout,
	}

	sm.Register("HTTP Server", func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().
				Err(err).
				Msg("HTTP server error")
		}
	}()

	logger.Info().
		Str("address", serverAddr).
		Dur("read_timeout", cfg.Worker.HTTP.ReadTimeout).
		Dur("write_timeout", cfg.Worker.HTTP.WriteTimeout).
		Msg("HTTP server listening")

	// 11. Wait for shutdown signal
	sm.WaitForSignal(ctx)

	logger.Info().
		Msg("Server shutdown complete")
}

// redisClientForMetrics exposes the underlying *redis.Client from the
// infraRedis.Client wrapper. Returns nil if Redis was not configured (NoOp
// mode), so the business poller can still run with DB-only gauges.
func redisClientForMetrics(c *infraRedis.Client) *redisGo.Client {
	if c == nil {
		return nil
	}
	return c.RDB()
}
