package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
)

func main() {
	log.Println("============================================================")
	log.Println("🚀 Khởi động Đông Đô CS Core V2 (Golang Clean Architecture)")
	log.Println("============================================================")

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	if usePostgres {
		log.Printf("🐘 Connecting to PostgreSQL: %s", cfg.DatabaseURL)
		var pgDB *repoPostgres.DB
		var err error
		for attempt := 1; attempt <= 15; attempt++ {
			pgDB, err = repoPostgres.NewDB(ctx, cfg.DatabaseURL)
			if err == nil {
				break
			}
			log.Printf("⏳ Waiting for PostgreSQL (attempt %d/15): %v", attempt, err)
			time.Sleep(1 * time.Second)
		}

		if err != nil {
			log.Printf("⚠️ PostgreSQL connection failed (%v). Falling back to SQLite.", err)
			usePostgres = false
		} else {
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
		log.Println("📦 Khởi tạo cơ sở dữ liệu SQLite cục bộ (chat_history.db)...")
		sqliteDB, err := repoSqlite.NewDB("chat_history.db")
		if err != nil {
			log.Fatalf("❌ Failed to initialize SQLite: %v", err)
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
			log.Printf("⏳ Waiting for Redis (attempt %d/15): %v", attempt, err)
			time.Sleep(1 * time.Second)
		}

		if err != nil {
			log.Printf("⚠️ Redis connection failed (%v). Running with NoOp fallback.", err)
		} else {
			streamEventBus = infraRedis.NewEventBus(redisClient)
			eventBus = streamEventBus
			stateMgr = infraRedis.NewStateManager(redisClient)
			sm.Register("Redis Connection", func(ctx context.Context) error {
				return redisClient.Close()
			})
		}
	}

	// 3. Initialize Qdrant Vector DB
	var qdrantClient *infraQdrant.Client
	qdrantClient, err := infraQdrant.NewClient(ctx, cfg.QdrantHost, cfg.QdrantPort, 384)
	if err != nil {
		log.Printf("⚠️ Qdrant connection notice: %v (RAG will run in fallback mode until Qdrant is up)", err)
	} else {
		sm.Register("Qdrant gRPC Connection", func(ctx context.Context) error {
			return qdrantClient.Close()
		})
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
	if streamEventBus != nil {
		wsWorker := worker.NewWSWorker(streamEventBus, hub, "ws_worker_1")
		go wsWorker.Start(ctx)

		aiWorker := worker.NewAIWorker(streamEventBus, stateMgr, ragUC, messageRepo, caseRepo, "ai_worker_1")
		go aiWorker.Start(ctx)

		dbWorker := worker.NewDBWorker(streamEventBus, messageRepo, "db_worker_1", cfg.DBBatchSize, time.Duration(cfg.DBBatchInterval)*time.Millisecond)
		go dbWorker.Start(ctx)

		retryWorker := worker.NewRetryWorker(streamEventBus, "retry_worker_1", cfg.RetryMaxCount, cfg.RetryClaimAfter)
		go retryWorker.Start(ctx)
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
	serverAddr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
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
		log.Printf("🌐 Server listening on http://%s", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP server error: %v", err)
		}
	}()

	// 11. Wait for shutdown signal
	sm.WaitForSignal(ctx)
}
