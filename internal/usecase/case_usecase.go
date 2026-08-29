package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type CaseUseCase struct {
	guestRepo    domain.GuestRepository
	caseRepo     domain.CaseRepository
	messageRepo  domain.MessageRepository
	learningRepo domain.LearningRepository
	settingRepo  domain.SettingRepository
	vectorStore  domain.VectorStore
	embedder     domain.Embedder
	eventBus     domain.EventBus
}

func NewCaseUseCase(
	guestRepo domain.GuestRepository,
	caseRepo domain.CaseRepository,
	messageRepo domain.MessageRepository,
	learningRepo domain.LearningRepository,
	settingRepo domain.SettingRepository,
	vectorStore domain.VectorStore,
	embedder domain.Embedder,
	eventBus domain.EventBus,
) *CaseUseCase {
	return &CaseUseCase{
		guestRepo:    guestRepo,
		caseRepo:     caseRepo,
		messageRepo:  messageRepo,
		learningRepo: learningRepo,
		settingRepo:  settingRepo,
		vectorStore:  vectorStore,
		embedder:     embedder,
		eventBus:     eventBus,
	}
}

func (uc *CaseUseCase) ListCustomers(ctx context.Context) ([]*domain.CustomerProfile, error) {
	return uc.guestRepo.List(ctx)
}

func (uc *CaseUseCase) UpdateCustomer(ctx context.Context, guestIDStr, displayName, phone string) error {
	gID, err := uuid.Parse(guestIDStr)
	if err != nil {
		return fmt.Errorf("mã khách hàng không hợp lệ: %w", err)
	}
	return uc.guestRepo.Update(ctx, gID, displayName, phone)
}

func (uc *CaseUseCase) DeleteCustomer(ctx context.Context, guestIDStr string) error {
	gID, err := uuid.Parse(guestIDStr)
	if err != nil {
		return fmt.Errorf("mã khách hàng không hợp lệ: %w", err)
	}
	return uc.guestRepo.Delete(ctx, gID)
}

func (uc *CaseUseCase) InitCase(ctx context.Context, sessionID string, guestID *uuid.UUID, customerName, customerPhone string) (*domain.ChatCase, error) {
	if guestID == nil && customerName != "" && customerName != "Khách hàng" {
		gID := uuid.New()
		g, err := uc.guestRepo.Create(ctx, gID, customerName, customerPhone)
		if err == nil && g != nil {
			guestID = &g.GuestID
		}
	}
	return uc.caseRepo.Upsert(ctx, sessionID, guestID, customerName, customerPhone, domain.StatusAIActive, "", "Bắt đầu phiên trò chuyện")
}

func (uc *CaseUseCase) UpdateCustomerInfo(ctx context.Context, sessionID, customerName, customerPhone string) error {
	existingCase, err := uc.caseRepo.Get(ctx, sessionID)
	if err != nil || existingCase == nil {
		return fmt.Errorf("không tìm thấy case với phiên: %s", sessionID)
	}

	if existingCase.GuestID != nil {
		_ = uc.guestRepo.Update(ctx, *existingCase.GuestID, customerName, customerPhone)
	} else if customerName != "" && customerName != "Khách hàng" {
		gID := uuid.New()
		g, err := uc.guestRepo.Create(ctx, gID, customerName, customerPhone)
		if err == nil && g != nil {
			existingCase.GuestID = &g.GuestID
		}
	}

	_, err = uc.caseRepo.Upsert(ctx, sessionID, existingCase.GuestID, customerName, customerPhone, existingCase.Status, existingCase.AssignedCS, existingCase.LastMessage)
	if err != nil {
		return err
	}

	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id":     sessionID,
		"customer_name":  customerName,
		"customer_phone": customerPhone,
		"status":         existingCase.Status,
	}, "admin")

	return nil
}

func (uc *CaseUseCase) ListCases(ctx context.Context, status domain.CaseStatus) ([]*domain.ChatCase, error) {
	return uc.caseRepo.List(ctx, status)
}

func (uc *CaseUseCase) GetCase(ctx context.Context, sessionID string) (*domain.ChatCase, error) {
	return uc.caseRepo.Get(ctx, sessionID)
}

// TakeCase assigns a case to a CSKH agent and posts an introduction message.
func (uc *CaseUseCase) TakeCase(ctx context.Context, sessionID, csUsername, csFullName string) error {
	agentName := csFullName
	if agentName == "" {
		agentName = csUsername
	}

	err := uc.caseRepo.Assign(ctx, sessionID, agentName)
	if err != nil {
		return fmt.Errorf("failed to assign case: %w", err)
	}

	introMsg := fmt.Sprintf("Dạ em chào anh/chị, em là %s - Chuyên viên CSKH của Đông Đô Partners. Em đã tham gia cuộc trò chuyện và sẽ hỗ trợ anh/chị ngay đây ạ!", agentName)
	msg := &domain.Message{
		SessionID:  sessionID,
		SenderType: domain.SenderHumanCS,
		SenderID:   agentName,
		Content:    introMsg,
	}
	savedMsg, _ := uc.messageRepo.Insert(ctx, msg)

	// Broadcast via WebSocket
	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventMessage, savedMsg, csUsername)
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id":  sessionID,
		"status":      domain.StatusHumanCSActive,
		"assigned_cs": agentName,
	}, csUsername)

	return nil
}

// ResolveCase marks a case as resolved and processes continuous learning Q&A pairs.
func (uc *CaseUseCase) ResolveCase(ctx context.Context, sessionID, csUsername, csFullName, resolutionNote string, qaPairs []domain.QAPair) (autoLearned bool, learnedCount int, err error) {
	agentName := csFullName
	if agentName == "" {
		agentName = csUsername
	}

	err = uc.caseRepo.Resolve(ctx, sessionID, agentName, resolutionNote)
	if err != nil {
		return false, 0, fmt.Errorf("failed to resolve case: %w", err)
	}

	// Check if auto-learning is enabled
	autoLearnSetting, _ := uc.settingRepo.Get(ctx, "auto_learning_enabled", "0")
	isAutoLearn := autoLearnSetting == "1"

	for _, pair := range qaPairs {
		if pair.Question == "" || pair.Answer == "" {
			continue
		}

		if isAutoLearn {
			// Add as approved and embed directly to Vector DB
			item, err := uc.learningRepo.Add(ctx, sessionID, pair.Question, pair.Answer, domain.LearnApproved, agentName)
			if err == nil && item != nil {
				_ = uc.learningRepo.MarkStatus(ctx, item.ID, domain.LearnApproved, agentName)

				// Embed and upsert to Qdrant
				docContent := fmt.Sprintf("CÂU HỎI / CHỦ ĐỀ CỦA KHÁCH HÀNG: %s\nCÂU TRẢ LỜI / THÔNG TIN CHÍNH THỨC CỦA ĐÔNG ĐÔ PARTNERS: %s", pair.Question, pair.Answer)
				docID := fmt.Sprintf("learned_qa_%d", item.ID)
				if uc.embedder != nil && uc.vectorStore != nil {
					vec, err := uc.embedder.EmbedText(ctx, docContent)
					if err == nil {
						_ = uc.vectorStore.Upsert(ctx, []*domain.KnowledgeDocument{
							{
								ID:      docID,
								Content: docContent,
								Source:  "CSKH_Learning",
								Metadata: map[string]interface{}{
									"source":     "CSKH_Learning",
									"session_id": sessionID,
									"type":       "learned_qa",
									"question":   pair.Question,
								},
							},
						}, [][]float32{vec})
					}
				}
				learnedCount++
			}
		} else {
			// Add to pending approval queue
			_, err := uc.learningRepo.Add(ctx, sessionID, pair.Question, pair.Answer, domain.LearnPending, agentName)
			if err == nil {
				learnedCount++
			}
		}
	}

	// Broadcast case resolved
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id": sessionID,
		"status":     domain.StatusResolved,
	}, csUsername)

	return isAutoLearn, learnedCount, nil
}

func (uc *CaseUseCase) DeleteCase(ctx context.Context, sessionID string) error {
	_ = uc.learningRepo.DeleteBySession(ctx, sessionID)
	_ = uc.messageRepo.DeleteBySession(ctx, sessionID)
	return uc.caseRepo.Delete(ctx, sessionID)
}

func (uc *CaseUseCase) ClearAllCases(ctx context.Context) error {
	_ = uc.learningRepo.ClearAll(ctx)
	_ = uc.messageRepo.DeleteAll(ctx)
	return uc.caseRepo.DeleteAll(ctx)
}
