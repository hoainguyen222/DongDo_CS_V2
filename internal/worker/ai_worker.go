package worker

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/rs/zerolog"
)

// AIWorker consumes AI processing jobs from Redis Stream and generates RAG responses.
// It processes queries sequentially with configurable timeouts.
type AIWorker struct {
	eventBus    *infraRedis.EventBusService
	stateMgr    domain.StateManager
	ragUC       *usecase.RAGUseCase
	messageRepo domain.MessageRepository
	caseRepo    domain.CaseRepository
	consumer    string
	cfg         config.WorkerAIConfig
	logger      zerolog.Logger
}

// NewAIWorker creates a new AI worker with the provided configuration.
func NewAIWorker(
	eventBus *infraRedis.EventBusService,
	stateMgr domain.StateManager,
	ragUC *usecase.RAGUseCase,
	messageRepo domain.MessageRepository,
	caseRepo domain.CaseRepository,
	consumerName string,
	cfg config.WorkerAIConfig,
) *AIWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "ai_worker").Str("consumer", consumerName).Logger()

	// Apply defaults if not set
	if cfg.ReadCount <= 0 {
		cfg.ReadCount = 1 // Sequential processing by default
	}
	if cfg.BlockTimeout <= 0 {
		cfg.BlockTimeout = 2 * time.Second
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 500 * time.Millisecond
	}

	return &AIWorker{
		eventBus:    eventBus,
		stateMgr:    stateMgr,
		ragUC:       ragUC,
		messageRepo: messageRepo,
		caseRepo:    caseRepo,
		consumer:    consumerName,
		cfg:         cfg,
		logger:      logger,
	}
}

// Start runs the worker loop consuming from stream:ai with consumer group ai_group.
func (w *AIWorker) Start(ctx context.Context) {
	w.logger.Info().
		Int64("read_count", w.cfg.ReadCount).
		Dur("block_timeout", w.cfg.BlockTimeout).
		Msg("AI RAG Worker started")

	defer w.logger.Info().Msg("AI RAG Worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			messages, err := w.eventBus.ReadStreamGroup(
				ctx,
				infraRedis.StreamAI,
				infraRedis.GroupAI,
				w.consumer,
				w.cfg.ReadCount,
				w.cfg.BlockTimeout,
			)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error reading from AI stream")
				time.Sleep(w.cfg.RetryDelay)
				continue
			}

			for _, xmsg := range messages {
				sessionID, _ := xmsg.Values["session_id"].(string)
				query, _ := xmsg.Values["query"].(string)

				if sessionID == "" || query == "" {
					_ = w.eventBus.AckMessage(ctx, infraRedis.StreamAI, infraRedis.GroupAI, xmsg.ID)
					continue
				}

				_ = w.stateMgr.SetAIExecution(ctx, sessionID, true)

				if err := w.eventBus.PublishWS(ctx, sessionID, domain.WSEventTyping, map[string]interface{}{
					"sender_id": "Đông Đô AI",
					"typing":    true,
				}, "ai"); err != nil {
					w.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to publish typing indicator")
				}

				reply, sources, isFallback, err := w.ragUC.GenerateResponse(ctx, sessionID, query)
				if err != nil {
					w.logger.Error().Err(err).Str("session_id", sessionID).Msg("AI Worker processing error")
					reply = "Dạ xin lỗi anh/chị, hệ thống đang gặp gián đoạn tạm thời. Chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay."
					isFallback = true
				}

				if strings.TrimSpace(reply) == "" {
					w.logger.Warn().Str("session_id", sessionID).Msg("All LLM providers returned empty - using fallback")
					reply = "Dạ xin lỗi anh/chị, hiện tại em chưa nhận được phản hồi từ hệ thống AI. Chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay."
					isFallback = true
				}

				if err := w.eventBus.PublishWS(ctx, sessionID, domain.WSEventTyping, map[string]interface{}{
					"sender_id": "Đông Đô AI",
					"typing":    false,
				}, "ai"); err != nil {
					w.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to publish typing stop indicator")
				}

				aiMsg := &domain.Message{
					SessionID:  sessionID,
					SenderType: domain.SenderAI,
					SenderID:   "Đông Đô AI",
					Content:    reply,
					CreatedAt:  time.Now(),
				}
				savedMsg, err := w.messageRepo.Insert(ctx, aiMsg)
				if err != nil || savedMsg == nil {
					savedMsg = aiMsg
				}

				newStatus := domain.StatusAIActive
				if isFallback || strings.Contains(strings.ToLower(reply), "chuyên viên cskh") {
					newStatus = domain.StatusNeedsHumanCS
				}
				existingCase, _ := w.caseRepo.Get(ctx, sessionID)
				custName := ""
				custPhone := ""
				var guestID *uuid.UUID
				assignedCS := ""
				if existingCase != nil {
					custName = existingCase.CustomerName
					custPhone = existingCase.CustomerPhone
					guestID = existingCase.GuestID
					assignedCS = existingCase.AssignedCS
				}
				_, _ = w.caseRepo.Upsert(ctx, sessionID, guestID, custName, custPhone, newStatus, assignedCS, reply)

				if err := w.stateMgr.SetAIExecution(ctx, sessionID, false); err != nil {
					w.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to clear AI execution state")
				}

				if err := w.eventBus.PublishWS(ctx, sessionID, domain.WSEventMessage, map[string]interface{}{
					"message":     savedMsg,
					"sources":     sources,
					"is_fallback": isFallback,
				}, "ai"); err != nil {
					w.logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to publish AI reply")
				}

				if err := w.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
					"session_id":     sessionID,
					"customer_name":  custName,
					"customer_phone": custPhone,
					"status":         newStatus,
					"last_message":   reply,
					"assigned_cs":    assignedCS,
				}, "ai"); err != nil {
					w.logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to publish case update")
				}

				if err := w.eventBus.AckMessage(ctx, infraRedis.StreamAI, infraRedis.GroupAI, xmsg.ID); err != nil {
					w.logger.Error().Err(err).Msg("Failed to acknowledge AI message")
				}
			}
		}
	}
}
