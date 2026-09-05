package main

import (
	"context"
	"errors"
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
	infraAsterisk "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/asterisk"
	infraClaude "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/claude"
	infraEmbedding "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/embedding"
	infraQdrant "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/qdrant"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	repoPostgres "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/postgres"
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

	// 1. Initialize Database (PostgreSQL)
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

	logger.Info().
		Str("type", "postgresql").
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

	// Voice UC needs the queue manager + call-event repo. We always
	// provide a working QueueManager — when Redis is unavailable we
	// fall back to an in-process implementation (single-instance only).
	var queueMgr domain.QueueManager
	callEventRepo := repoPostgres.NewCallEventRepo(pgDB)
	if redisClient != nil {
		queueMgr = infraRedis.NewQueueManager(redisClient)
	} else {
		queueMgr = infraRedis.NewInMemoryQueueManager()
		logger.Warn().Msg("Redis unavailable; voice queue running with InMemory fallback")
	}
	voiceUC := usecase.NewVoiceUseCase(
		voiceRepo,
		callEventRepo,
		queueMgr,
		caseRepo,
		eventBus,
		usecase.VoiceUseCaseConfig{
			ReservationTTL: 30 * time.Second,
			KnownAgents:    strings.Split(os.Getenv("AGENT_EXTENSIONS"), ","),
		},
	)
	analyticsUC := usecase.NewAnalyticsUseCase(analyticsRepo, settingRepo)
	partnerUC := usecase.NewPartnerUseCase(partnerRepo, settingRepo)

	// 6. Initialize Asterisk AMI client (optional - graceful fallback when
	//    Asterisk is unreachable or disabled).
	if cfg.Asterisk.Enabled {
		astDomainCfg := domain.AsteriskConfig{
			Enabled:  cfg.Asterisk.Enabled,
			Host:     cfg.Asterisk.Host,
			Port:     cfg.Asterisk.Port,
			Username: cfg.Asterisk.Username,
			Password: cfg.Asterisk.Password,
			Context:  cfg.Asterisk.Context,
			Trunk:    cfg.Asterisk.Trunk,
			Queue:    cfg.Asterisk.Queue,
		}
		astClient, astErr := infraAsterisk.Factory(ctx, astDomainCfg)
		if astErr != nil {
			logger.Warn().
				Err(astErr).
				Msg("Asterisk AMI client init failed; falling back to NoOp")
		}
		if astClient != nil {
			// The AMI client implements domain.AsteriskClient which is
			// structurally compatible with the gateway surface we need
			// (Enabled/IsConnected + Originate/Hangup). Wire it through
			// the voice usecase so the gateway interface is honored.
			voiceUC.WithAsterisk(amiAsGateway(astClient))
			sm.Register("Asterisk AMI Connection", func(ctx context.Context) error {
				return astClient.Disconnect(ctx)
			})
			if astClient.IsConnected() {
				logger.Info().
					Str("host", cfg.Asterisk.Host).
					Int("port", cfg.Asterisk.Port).
					Msg("Asterisk AMI connected")
			} else {
				logger.Warn().
					Str("host", cfg.Asterisk.Host).
					Int("port", cfg.Asterisk.Port).
					Msg("Asterisk AMI client started but not yet connected; supervisor will retry")
			}
		}
	} else {
		logger.Info().Msg("Asterisk integration disabled via ASTERISK_ENABLED=false")
	}

	// 6b. Initialize Asterisk ARI (WebRTC / Stasis app) — runs alongside
	//     AMI.  AMI still owns originate-to-trunk while ARI owns the
	//     WebRTC guest leg + bridging once an agent accepts.
	var ariService *infraAsterisk.ARIService
	if cfg.AsteriskARI.Enabled {
		ariCfg, ariCfgErr := infraAsterisk.ARIConfigFromConfig(cfg.AsteriskARI)
		if ariCfgErr != nil {
			logger.Warn().Err(ariCfgErr).Msg("Asterisk ARI config invalid; skipping ARI")
		} else {
			ariClient, ariErr := infraAsterisk.NewARIClient(ariCfg)
			if ariErr != nil {
				logger.Warn().Err(ariErr).Msg("Asterisk ARI client init failed")
			} else {
				ariService = infraAsterisk.NewARIService(ariClient)
				// Hook ARI callbacks into the voice use-case so admin
				// dashboards receive the same WS notifications they
				// got from AMI before.
				ariService.OnGuestRinging = func(callID int64, sessionID, callerName, callerID string) {
					if voiceUC == nil {
						return
					}
					_ = voiceUC.HandleARIGuestRing(ctx, callID, sessionID, callerName, callerID)
				}
				ariService.OnCallActive = func(callID int64, sessionID, agentExt string) {
					if voiceUC == nil {
						return
					}
					_ = voiceUC.HandleARICallActive(ctx, callID, sessionID, agentExt)
				}
				ariService.OnCallEnded = func(callID int64, sessionID, cause string) {
					if voiceUC == nil {
						return
					}
					_ = voiceUC.HandleARICallEnded(ctx, callID, sessionID, cause)
				}
				voiceUC.WithARI(ariService)
				// Also expose the ARI service as a gateway so the voice
				// usecase can call OriginateGuestCall / OriginateAgentCall
				// without depending on the infra package directly.
				resolveSession := func(ctx context.Context, callID int64) (string, error) {
					call, err := voiceUC.GetCall(ctx, callID)
					if err != nil {
						return "", err
					}
					if call == nil {
						return "", fmt.Errorf("call %d not found", callID)
					}
					return call.SessionID, nil
				}
				voiceUC.WithAsterisk(infraAsterisk.NewARIGatewayAdapter(ariService, resolveSession))
				if err := ariService.Start(ctx); err != nil {
					logger.Warn().Err(err).Msg("Asterisk ARI service start failed")
				} else {
					sm.Register("Asterisk ARI Connection", func(ctx context.Context) error {
						ariService.Stop()
						return nil
					})
					logger.Info().
						Str("app", ariCfg.AppName).
						Str("base_url", ariCfg.BaseURL).
						Msg("Asterisk ARI service started")
				}
			}
		}
	} else {
		logger.Info().Msg("Asterisk ARI disabled via ASTERISK_ARI_ENABLED=false")
	}
	_ = ariService

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

	// Attach ARI service to handler (if wired up earlier).
	if ariService != nil {
		handler.SetARIService(ariService)
	}

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

// amiAsGateway adapts the AMI client (domain.AsteriskClient) into the
// refactored domain.AsteriskGateway interface used by the voice
// usecase. We keep the AMI client intact because it exposes the full
// event stream via Events() — the gateway interface only needs the
// control primitives.
func amiAsGateway(c domain.AsteriskClient) domain.AsteriskGateway {
	return &amiGatewayAdapter{client: c}
}

type amiGatewayAdapter struct{ client domain.AsteriskClient }

func (a *amiGatewayAdapter) Enabled() bool    { return a.client.Enabled() }
func (a *amiGatewayAdapter) Connected() bool { return a.client.IsConnected() }

func (a *amiGatewayAdapter) OriginateGuestCall(ctx context.Context, callID int64, sessionID, endpoint string) error {
	_, err := a.client.Originate(ctx, domain.OriginateRequest{
		Channel: "PJSIP/" + endpoint,
		Exten:   endpoint,
		Context: "from-internal",
		Async:   true,
		Variables: map[string]string{
			"DD_CALL_ID":    fmt.Sprintf("%d", callID),
			"DD_SESSION_ID": sessionID,
			"DD_LEG":        "guest",
		},
	})
	return err
}

func (a *amiGatewayAdapter) OriginateAgentCall(ctx context.Context, callID int64, sessionID, agentExt string) error {
	_, err := a.client.Originate(ctx, domain.OriginateRequest{
		Channel: "PJSIP/" + agentExt,
		Exten:   agentExt,
		Context: "from-internal",
		Async:   true,
		Variables: map[string]string{
			"DD_CALL_ID":    fmt.Sprintf("%d", callID),
			"DD_SESSION_ID": sessionID,
			"DD_LEG":        "agent",
			"DD_AGENT_EXT":  agentExt,
		},
	})
	return err
}

func (a *amiGatewayAdapter) HangupCall(ctx context.Context, callID int64) error {
	// The AMI client knows nothing about our callID. Real hangup
	// goes through the channel ID stored on the call row. The voice
	// usecase keeps that mapping in callMap.
	return errors.New("amiGatewayAdapter.HangupCall: requires channel id (use ARI for full hangup)")
}

func (a *amiGatewayAdapter) StartRecording(ctx context.Context, callID int64, filename string) error {
	// Recording is started by the voice usecase with a channel id.
	return errors.New("amiGatewayAdapter.StartRecording: requires channel id")
}
