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
	"github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis/callqueue"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/infrastructure/asterisk"
	repoPostgres "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/postgres"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	calluc "github.com/hoainguyen222/DongDo_CS_V2/internal/usecase/call"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/worker"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/graceful"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	serviceName    = "dongdo-cs-server"
	serviceVersion = "3.0.0"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
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

	// ============================================================
	// 1. PostgreSQL
	// ============================================================
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
		callRepo      domain.CallRepository // v2
	)

	var pgDB *repoPostgres.DB
	var err error
	for attempt := 1; attempt <= 15; attempt++ {
		pgDB, err = repoPostgres.NewDB(ctx, cfg.DatabaseURL)
		if err == nil {
			break
		}
		if attempt == 1 {
			logger.Warn().Str("type", "postgresql").Err(err).Msg("Waiting for PostgreSQL")
		}
		if attempt == 15 {
			logger.Fatal().Str("type", "postgresql").Err(err).Msg("PostgreSQL connection failed after all retries")
		}
		time.Sleep(1 * time.Second)
	}
	sm.Register("PostgreSQL Connection Pool", func(ctx context.Context) error { pgDB.Close(); return nil })

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
	callRepo = repoPostgres.NewCallRepo(pgDB)

	logger.Info().Str("type", "postgresql").Msg("Database connected")

	// ============================================================
	// 2. Redis (events, state, call queue)
	// ============================================================
	var eventBus domain.EventBus = infraRedis.NewNoOpEventBus()
	var stateMgr domain.StateManager = infraRedis.NewNoOpStateManager()
	var redisClient *infraRedis.Client
	var streamEventBus *infraRedis.EventBusService

	var callQueueMgr domain.CallQueueManager = callqueue.NewNoopManager()

	if cfg.RedisURL != "" {
		var rerr error
		for attempt := 1; attempt <= 15; attempt++ {
			redisClient, rerr = infraRedis.NewClient(cfg.RedisURL)
			if rerr == nil {
				break
			}
			if attempt == 1 {
				logger.Warn().Err(rerr).Msg("Waiting for Redis")
			}
			if attempt == 15 {
				logger.Error().Err(rerr).Msg("Redis connection failed after all retries; running with NoOp fallback")
			}
			time.Sleep(1 * time.Second)
		}

		if rerr != nil {
			eventBus = infraRedis.NewNoOpEventBus()
			stateMgr = infraRedis.NewNoOpStateManager()
		} else {
			streamEventBus = infraRedis.NewEventBus(redisClient)
			eventBus = streamEventBus
			stateMgr = infraRedis.NewStateManager(redisClient)
			callQueueMgr = callqueue.New(redisClient.RDB(), logger)

			sm.Register("Redis Connection", func(ctx context.Context) error { return redisClient.Close() })
			logger.Info().Str("url", cfg.RedisURL).Msg("Redis connected")
		}
	} else {
		logger.Warn().Msg("Redis URL not configured; using NoOp fallback for events + state + call queue")
	}

	// ============================================================
	// 3. Qdrant Vector DB
	// ============================================================
	qdrantClient, err := infraQdrant.NewClient(ctx, cfg.QdrantHost, cfg.QdrantPort, 384)
	if err != nil {
		logger.Warn().Err(err).Msg("Qdrant unavailable; RAG will run in fallback mode")
	} else {
		sm.Register("Qdrant gRPC Connection", func(ctx context.Context) error { qdrantClient.Close(); return nil })
		logger.Info().Str("host", cfg.QdrantHost).Int("port", cfg.QdrantPort).Msg("Qdrant connected")
	}

	// ============================================================
	// 4. Embedder & Claude LLM
	// ============================================================
	embedder := infraEmbedding.NewEmbedder(cfg.EmbeddingModel)
	claudeClient := infraClaude.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicWorkspaceID, cfg.OpenAIAPIKey, cfg.GeminiAPIKey, cfg.LLMModel, cfg.LLMTemperature)

	// ============================================================
	// 5. Use Cases
	// ============================================================
	authUC := usecase.NewAuthUseCase(userRepo, sessionRepo, guestRepo)
	ragUC := usecase.NewRAGUseCase(qdrantClient, embedder, claudeClient, messageRepo, settingRepo, cfg.SystemPrompt, cfg.MemoryWindow, cfg.RetrieverK)
	caseUC := usecase.NewCaseUseCase(guestRepo, caseRepo, messageRepo, learningRepo, settingRepo, qdrantClient, embedder, eventBus)
	chatUC := usecase.NewChatUseCase(messageRepo, caseRepo, eventBus, stateMgr)
	learningUC := usecase.NewLearningUseCase(learningRepo, settingRepo, qdrantClient, embedder, eventBus)
	voiceUC := usecase.NewVoiceUseCase(voiceRepo, caseRepo, eventBus)
	analyticsUC := usecase.NewAnalyticsUseCase(analyticsRepo, settingRepo)
	partnerUC := usecase.NewPartnerUseCase(partnerRepo, settingRepo)
	tagUC := usecase.NewChatTagUseCase(tagRepo)

	// ============================================================
	// 6. WebSocket Hub
	// ============================================================
	hub := deliveryWS.NewHub()
	go hub.Run()
	eventBus.SetHub(hub)

	// ============================================================
	// 7. Call v2 subsystem — AsteriskGateway + CallUseCase + workers
	// ============================================================
	ariCfg := asterisk.Config{
		BaseURL:            cfg.ARI.URL,
		Username:           cfg.ARI.User,
		Password:           cfg.ARI.Password,
		AppName:            cfg.ARI.App,
		RecordingEnabled:   cfg.ARI.RecordingEnabled,
		HTTPTimeout:        cfg.ARI.HTTPTimeout,
		WSReconnectBackoff: cfg.ARI.WSReconnectBackoff,
	}
	ariClient := asterisk.NewClient(ariCfg, logger)
	sm.Register("Asterisk ARI Client", func(ctx context.Context) error { return ariClient.Close() })

	// Pre-flight health check (non-fatal — log only).
	hcCtx, hcCancel := context.WithTimeout(ctx, 3*time.Second)
	if err := ariClient.HealthCheck(hcCtx); err != nil {
		logger.Warn().Err(err).Msg("Asterisk not reachable at startup; calls will fail fast")
	} else {
		logger.Info().Msg("Asterisk ARI reachable")
	}
	hcCancel()

	callUC := calluc.NewUseCase(callRepo, callQueueMgr, ariClient, hub, caseRepo, calluc.Config{
		RingTimeout:      cfg.Call.AgentRingTimeout,
		MaxDuration:      cfg.Call.MaxDuration,
		QueueTTL:         cfg.Call.QueueTTL,
		RecordingEnabled: cfg.ARI.RecordingEnabled,
	}, logger)
	sm.Register("Call UseCase", func(ctx context.Context) error { return callUC.Shutdown(ctx) })

	// ARI WS consumer
	ariConsumer := asterisk.NewWSConsumer(ariCfg, callUC.HandleARIEvent, logger)
	go func() {
		if err := ariConsumer.Start(ctx); err != nil && ctx.Err() == nil {
			logger.Error().Err(err).Msg("ARI WS consumer exited")
		}
	}()

	// Routing + reconciliation workers
	routingWorker := calluc.NewRoutingWorker(callUC, logger, 500*time.Millisecond)
	go routingWorker.Start(ctx)
	reconcileWorker := calluc.NewReconciliationWorker(callUC, logger, cfg.Call.ReconcileEvery)
	go reconcileWorker.Start(ctx)

	// Wire the WS hub's customer-disconnect hook so a customer whose browser
	// disconnects while in the queue has their pending call cancelled.
	hub.SetDisconnectHandler(func(sessionID, _, role string) {
		if role != "guest" {
			return
		}
		ctxDisconnect, cancelDisconnect := context.WithTimeout(ctx, 3*time.Second)
		defer cancelDisconnect()
		callUC.CustomerDisconnected(ctxDisconnect, sessionID)
	})

	// ============================================================
	// 8. Background workers (chat, AI, retry)
	// ============================================================
	startedWorkers := []string{"call_routing", "call_reconcile", "ari_consumer"}
	if streamEventBus != nil {
		wsWorker := worker.NewWSWorker(streamEventBus, hub, "ws_worker_1")
		go wsWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "ws_worker_1")

		aiWorker := worker.NewAIWorker(streamEventBus, stateMgr, ragUC, messageRepo, caseRepo, "ai_worker_1")
		go aiWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "ai_worker_1")

		dbWorker := worker.NewDBWorker(streamEventBus, messageRepo, "db_worker_1", cfg.DBBatchSize, time.Duration(cfg.DBBatchInterval)*time.Millisecond)
		go dbWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "db_worker_1")

		retryWorker := worker.NewRetryWorker(streamEventBus, "retry_worker_1", cfg.RetryMaxCount, cfg.RetryClaimAfter)
		go retryWorker.Start(ctx)
		startedWorkers = append(startedWorkers, "retry_worker_1")
	} else {
		logger.Warn().Msg("Redis not available; chat background workers not started")
	}

	logger.Info().Strs("workers", startedWorkers).Msg("Workers started")

	// ============================================================
	// 9. HTTP Server
	// ============================================================
	handler := deliveryHTTP.NewHandler(
		authUC, chatUC, caseUC, learningUC, voiceUC, analyticsUC, partnerUC, ragUC, tagUC,
		qdrantClient, embedder, cfg.DocumentsDir, eventBus,
		callUC,
	)
	callHandler := deliveryHTTP.NewCallHandler(callUC)

	router := deliveryHTTP.SetupRouter(handler, hub, chatUC, voiceUC, stateMgr, eventBus, authUC, callHandler)

	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	sm.Register("HTTP Server", func(ctx context.Context) error { return srv.Shutdown(ctx) })

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	logger.Info().Str("address", serverAddr).Msg("HTTP server listening")

	sm.WaitForSignal(ctx)

	logger.Info().Msg("Server shutdown complete")
}
