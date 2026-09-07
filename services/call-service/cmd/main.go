package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/config"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/application"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/asterisk"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/postgres"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/redis"
	callhttp "github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/interfaces/http"
	callws "github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/interfaces/websocket"
	_ "github.com/lib/pq"
	godredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Starting DongDo Standalone WebRTC Call Service...")

	cfg := config.LoadConfig()

	// 1. Initialize Postgres DB
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect Postgres DB")
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// 2. Initialize Redis
	opt, err := godredis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse Redis URL")
	}
	redisClient := godredis.NewClient(opt)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn().Err(err).Msg("Redis ping warning - retrying in background")
	}

	// 3. Infrastructure Stores
	callRepo := postgres.NewCallRepository(db)
	agentStore := redis.NewAgentStateStore(redisClient)
	streamMgr := redis.NewCallStreamManager(redisClient)
	ariClient := asterisk.NewARIClient(cfg.AsteriskURL, cfg.AsteriskUser, cfg.AsteriskPass, cfg.AsteriskApp)

	// 4. WebSocket Hub
	wsHub := callws.NewWSHub()
	go wsHub.Run()

	// 5. Application Call Service
	callSvc := application.NewCallService(callRepo, agentStore, streamMgr, ariClient, wsHub)

	// 6. Background Workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Worker A: Redis Stream Call Router Worker
	go streamMgr.ConsumeCallRequests(ctx, "router_worker_1", func(callID, customerID string) error {
		return callSvc.HandleRouteCall(ctx, callID, customerID)
	})

	// Worker B: Asterisk ARI Event Consumer WebSocket
	ariConsumer := asterisk.NewARIEventConsumer(cfg.AsteriskURL, cfg.AsteriskUser, cfg.AsteriskPass, cfg.AsteriskApp, func(event *asterisk.ARIEvent) {
		callSvc.HandleARIEvent(event)
	})
	go ariConsumer.Start(ctx)

	// 7. Gin HTTP Engine
	router := gin.Default()
	// CORS Middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	handler := callhttp.NewCallHandler(callSvc, agentStore, wsHub)
	handler.RegisterRoutes(router)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Info().Str("addr", server.Addr).Msg("Standalone Call Service HTTP Listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Call Service HTTP server failed")
		}
	}()

	// Graceful Shutdown Handler
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Call Service gracefully...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server forced shutdown")
	}

	log.Info().Msg("Call Service stopped cleanly")
}
