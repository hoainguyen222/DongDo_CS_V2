package worker

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
)

type AIWorker struct {
	eventBus    *infraRedis.EventBusService
	stateMgr    domain.StateManager
	ragUC       *usecase.RAGUseCase
	messageRepo domain.MessageRepository
	caseRepo    domain.CaseRepository
	consumer    string
}

func NewAIWorker(
	eventBus *infraRedis.EventBusService,
	stateMgr domain.StateManager,
	ragUC *usecase.RAGUseCase,
	messageRepo domain.MessageRepository,
	caseRepo domain.CaseRepository,
	consumerName string,
) *AIWorker {
	return &AIWorker{
		eventBus:    eventBus,
		stateMgr:    stateMgr,
		ragUC:       ragUC,
		messageRepo: messageRepo,
		caseRepo:    caseRepo,
		consumer:    consumerName,
	}
}

// Start runs the worker loop consuming from stream:ai with consumer group ai_group.
func (w *AIWorker) Start(ctx context.Context) {
	log.Println("🤖 Started AI RAG Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 AI Worker stopped")
			return
		default:
			messages, err := w.eventBus.ReadStreamGroup(ctx, infraRedis.StreamAI, infraRedis.GroupAI, w.consumer, 1, 2*time.Second)
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			for _, xmsg := range messages {
				sessionID, _ := xmsg.Values["session_id"].(string)
				query, _ := xmsg.Values["query"].(string)

				if sessionID == "" || query == "" {
					_ = w.eventBus.AckMessage(ctx, infraRedis.StreamAI, infraRedis.GroupAI, xmsg.ID)
					continue
				}

				// Set AI execution state in Redis
				_ = w.stateMgr.SetAIExecution(ctx, sessionID, true)

				// Broadcast "AI is typing..." indicator to client
				_ = w.eventBus.PublishWS(ctx, sessionID, domain.WSEventTyping, map[string]interface{}{
					"sender_id": "Đông Đô AI",
					"typing":    true,
				}, "ai")

				// Process RAG & Claude API call
				reply, sources, isFallback, err := w.ragUC.GenerateResponse(ctx, sessionID, query)
				if err != nil {
					log.Printf("❌ AI Worker error for session %s: %v", sessionID, err)
					reply = "Dạ xin lỗi anh/chị, hệ thống đang gặp gián đoạn tạm thời. Chuyên viên CSKH của Đông Đô sẽ trực tiếp tham gia cuộc trò chuyện để hỗ trợ bạn ngay."
					isFallback = true
				}

				// Stop typing indicator
				_ = w.eventBus.PublishWS(ctx, sessionID, domain.WSEventTyping, map[string]interface{}{
					"sender_id": "Đông Đô AI",
					"typing":    false,
				}, "ai")

				// Save AI reply message
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

				// Update case status preserving customer info
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

				// Clear AI execution state
				_ = w.stateMgr.SetAIExecution(ctx, sessionID, false)

				// Broadcast AI reply to user and CS Studio via WebSocket stream
				_ = w.eventBus.PublishWS(ctx, sessionID, domain.WSEventMessage, map[string]interface{}{
					"message":     savedMsg,
					"sources":     sources,
					"is_fallback": isFallback,
				}, "ai")

				_ = w.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
					"session_id":     sessionID,
					"customer_name":  custName,
					"customer_phone": custPhone,
					"status":         newStatus,
					"last_message":   reply,
					"assigned_cs":    assignedCS,
				}, "ai")

				// XACK after successful completion
				_ = w.eventBus.AckMessage(ctx, infraRedis.StreamAI, infraRedis.GroupAI, xmsg.ID)
			}
		}
	}
}
