package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	deliveryHTTP "github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/http"
	deliveryWS "github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraClaude "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/claude"
	infraEmbedding "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/embedding"
	infraQdrant "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/qdrant"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	repoPostgres "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/postgres"
	repoSqlite "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlite"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/worker"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/graceful"

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

	// 1. Initialize Database (PostgreSQL with auto SQLite fallback)
	var userRepo domain.UserRepository
	var sessionRepo domain.SessionRepository
	var guestRepo domain.GuestRepository
	var messageRepo domain.MessageRepository
	var caseRepo domain.CaseRepository
	var learningRepo domain.LearningRepository
	var settingRepo domain.SettingRepository
	var voiceRepo domain.VoiceCallRepository
	var analyticsRepo domain.AnalyticsRepository
	var partnerRepo domain.PartnerRepository

	usePostgres := cfg.DatabaseURL != "" && strings.HasPrefix(cfg.DatabaseURL, "postgres://")
	dbLabel := "sqlite"

	if usePostgres {
		var pgDB *repoPostgres.DB
		var err error
		for attempt := 1; attempt <= 15; attempt++ {
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
				logger.Error().
					Str("type", "postgresql").
					Err(err).
					Msg("PostgreSQL connection failed after all retries; falling back to SQLite")
			}
			time.Sleep(1 * time.Second)
		}

		if err != nil {
			usePostgres = false
		} else {
			dbLabel = "postgresql"
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
		}
	}

	if !usePostgres {
		sqliteDB, err := repoSqlite.NewDB("chat_history.db")
		if err != nil {
			logger.Fatal().
				Str("type", "sqlite").
				Err(err).
				Msg("Failed to initialize SQLite")
		}

		sm.Register("SQLite Database", func(ctx context.Context) error {
			return sqliteDB.Close()
		})
		userRepo = repoSqlite.NewUserRepo(sqliteDB)
		sessionRepo = repoSqlite.NewSessionRepo(sqliteDB)
		guestRepo = repoSqlite.NewGuestRepo(sqliteDB)
		messageRepo = repoSqlite.NewMessageRepo(sqliteDB)
		caseRepo = repoSqlite.NewCaseRepo(sqliteDB)
		learningRepo = repoSqlite.NewLearningRepo(sqliteDB)
		settingRepo = repoSqlite.NewSettingRepo(sqliteDB)
		voiceRepo = repoSqlite.NewVoiceCallRepo(sqliteDB)
		analyticsRepo = repoSqlite.NewAnalyticsRepo(sqliteDB)
		partnerRepo = repoSqlite.NewPartnerRepo(sqliteDB)
	}

	logger.Info().
		Str("type", dbLabel).
		Msg("Database connected")

	// 2. Initialize Redis (Event Bus & State)
	var eventBus domain.EventBus = infraRedis.NewNoOpEventBus()
	var stateMgr domain.StateManager = infraRedis.NewNoOpStateManager()
	var redisClient *infraRedis.Client
	var streamEventBus *infraRedis.EventBusService

	if cfg.RedisURL != "" {
		var err error
		for attempt := 1; attempt <= 15; attempt++ {
			redisClient, err = infraRedis.NewClient(cfg.RedisURL)
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
			streamEventBus = infraRedis.NewEventBus(redisClient)
			eventBus = streamEventBus
			stateMgr = infraRedis.NewStateManager(redisClient)
			sm.Register("Redis Connection", func(ctx context.Context) error {
				return redisClient.Close()
			})
			logger.Info().
				Str("url", cfg.RedisURL).
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

	// 7. Initialize WebSocket Hub
	hub := deliveryWS.NewHub()
	go hub.Run()
	eventBus.SetHub(hub)

	// 8. Start Background Workers if Redis Streams is available
	startedWorkers := []string{}
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

		logger.Info().
			Strs("workers", startedWorkers).
			Msg("Workers started")
	} else {
		logger.Warn().
			Msg("Redis not available; background workers not started")
	}

	// 9. Initialize HTTP Router
	handler := deliveryHTTP.NewHandler(
		authUC,
		chatUC,
		caseUC,
		learningUC,
		voiceUC,
		analyticsUC,
		partnerUC,
		ragUC,
		qdrantClient,
		embedder,
		cfg.DocumentsDir,
		eventBus,
	)

	router := deliveryHTTP.SetupRouter(handler, hub, chatUC, voiceUC, stateMgr, eventBus, authUC)

	// 10. Start HTTP Server
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
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
		Msg("HTTP server listening")

	// 11. Wait for shutdown signal
	sm.WaitForSignal(ctx)

	logger.Info().
		Msg("Server shutdown complete")
}
