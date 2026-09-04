package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
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
	logger       zerolog.Logger
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
		logger:       zerolog.New(nil).With().Timestamp().Str("usecase", "case").Logger(),
	}
}

func (uc *CaseUseCase) ListCustomers(ctx context.Context) ([]*domain.CustomerProfile, error) {
	customers, err := uc.guestRepo.List(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list customers")
		return nil, err
	}
	return customers, nil
}

func (uc *CaseUseCase) UpdateCustomer(ctx context.Context, guestIDStr, displayName, phone string) error {
	gID, err := uuid.Parse(guestIDStr)
	if err != nil {
		uc.logger.Warn().Str("guest_id", guestIDStr).Err(err).
			Msg("update customer failed: invalid guest ID")
		return fmt.Errorf("mã khách hàng không hợp lệ: %w", err)
	}

	if err := uc.guestRepo.Update(ctx, gID, displayName, phone); err != nil {
		uc.logger.Error().Err(err).Str("guest_id", gID.String()).
			Msg("failed to update customer")
		return err
	}
	return nil
}

func (uc *CaseUseCase) DeleteCustomer(ctx context.Context, guestIDStr string) error {
	gID, err := uuid.Parse(guestIDStr)
	if err != nil {
		uc.logger.Warn().Str("guest_id", guestIDStr).Err(err).
			Msg("delete customer failed: invalid guest ID")
		return fmt.Errorf("mã khách hàng không hợp lệ: %w", err)
	}

	if err := uc.guestRepo.Delete(ctx, gID); err != nil {
		uc.logger.Error().Err(err).Str("guest_id", gID.String()).
			Msg("failed to delete customer")
		return err
	}
	return nil
}

func (uc *CaseUseCase) InitCase(ctx context.Context, sessionID string, guestID *uuid.UUID, customerName, customerPhone string) (*domain.ChatCase, error) {
	// Create guest if needed
	if guestID == nil && customerName != "" && customerName != "Khách hàng" {
		gID := uuid.New()
		g, err := uc.guestRepo.Create(ctx, gID, customerName, customerPhone)
		if err == nil && g != nil {
			guestID = &g.GuestID
		} else if err != nil {
			uc.logger.Warn().Err(err).Str("session_id", sessionID).
				Msg("failed to create guest for case (continuing without guest)")
		}
	}

	newCase, err := uc.caseRepo.Upsert(ctx, sessionID, guestID, customerName, customerPhone, domain.StatusAIActive, "", "Bắt đầu phiên trò chuyện")
	if err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to create case")
		return nil, err
	}
	return newCase, nil
}

func (uc *CaseUseCase) UpdateCustomerInfo(ctx context.Context, sessionID, customerName, customerPhone string) error {
	existingCase, err := uc.caseRepo.Get(ctx, sessionID)
	if err != nil || existingCase == nil {
		uc.logger.Warn().Err(err).Str("session_id", sessionID).
			Msg("update customer info failed: case not found")
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

	if _, err = uc.caseRepo.Upsert(ctx, sessionID, existingCase.GuestID, customerName, customerPhone, existingCase.Status, existingCase.AssignedCS, existingCase.LastMessage); err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to update customer info in case")
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
	cases, err := uc.caseRepo.List(ctx, status)
	if err != nil {
		uc.logger.Error().Err(err).Str("status_filter", string(status)).
			Msg("failed to list cases")
		return nil, err
	}
	return cases, nil
}

func (uc *CaseUseCase) GetCase(ctx context.Context, sessionID string) (*domain.ChatCase, error) {
	chatCase, err := uc.caseRepo.Get(ctx, sessionID)
	if err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to fetch case")
		return nil, err
	}
	return chatCase, nil
}

// TakeCase assigns a case to a CSKH agent and posts an introduction message.
func (uc *CaseUseCase) TakeCase(ctx context.Context, sessionID, csUsername, csFullName string) error {
	agentName := csFullName
	if agentName == "" {
		agentName = csUsername
	}

	if err := uc.caseRepo.Assign(ctx, sessionID, agentName); err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to assign case")
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

	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventMessage, savedMsg, csUsername)
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id":  sessionID,
		"status":      domain.StatusHumanCSActive,
		"assigned_cs": agentName,
	}, csUsername)

	uc.logger.Info().Str("session_id", sessionID).Str("agent_name", agentName).
		Msg("case taken by CS agent")

	return nil
}

// ResolveCase marks a case as resolved and processes continuous learning Q&A pairs.
func (uc *CaseUseCase) ResolveCase(ctx context.Context, sessionID, csUsername, csFullName, resolutionNote string, qaPairs []domain.QAPair) (autoLearned bool, learnedCount int, err error) {
	agentName := csFullName
	if agentName == "" {
		agentName = csUsername
	}

	if err = uc.caseRepo.Resolve(ctx, sessionID, agentName, resolutionNote); err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to resolve case")
		return false, 0, fmt.Errorf("failed to resolve case: %w", err)
	}

	autoLearnSetting, _ := uc.settingRepo.Get(ctx, "auto_learning_enabled", "0")
	isAutoLearn := autoLearnSetting == "1"

	for _, pair := range qaPairs {
		if pair.Question == "" || pair.Answer == "" {
			continue
		}

		if isAutoLearn {
			// Auto-learn enabled - add as approved and embed to Vector DB
			item, err := uc.learningRepo.Add(ctx, sessionID, pair.Question, pair.Answer, domain.LearnApproved, agentName)
			if err == nil && item != nil {
				_ = uc.learningRepo.MarkStatus(ctx, item.ID, domain.LearnApproved, agentName)

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
					} else {
						uc.logger.Warn().Err(err).Str("session_id", sessionID).Int64("learning_item_id", item.ID).
							Msg("failed to embed QA pair")
					}
				}
				learnedCount++
			}
		} else {
			// Auto-learn disabled - add to pending approval queue
			if _, err := uc.learningRepo.Add(ctx, sessionID, pair.Question, pair.Answer, domain.LearnPending, agentName); err == nil {
				learnedCount++
			}
		}
	}

	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id": sessionID,
		"status":     domain.StatusResolved,
	}, csUsername)

	uc.logger.Info().Str("session_id", sessionID).Str("agent_name", agentName).
		Int("learned_count", learnedCount).Bool("auto_learned", isAutoLearn).
		Msg("case resolved")

	return isAutoLearn, learnedCount, nil
}

func (uc *CaseUseCase) DeleteCase(ctx context.Context, sessionID string) error {
	_ = uc.learningRepo.DeleteBySession(ctx, sessionID)
	_ = uc.messageRepo.DeleteBySession(ctx, sessionID)

	if err := uc.caseRepo.Delete(ctx, sessionID); err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to delete case")
		return err
	}
	return nil
}

func (uc *CaseUseCase) ClearAllCases(ctx context.Context) error {
	uc.logger.Warn().Msg("clearing all cases (destructive operation)")

	_ = uc.learningRepo.ClearAll(ctx)
	_ = uc.messageRepo.DeleteAll(ctx)

	if err := uc.caseRepo.DeleteAll(ctx); err != nil {
		uc.logger.Error().Err(err).Msg("failed to clear all cases")
		return err
	}
	uc.logger.Warn().Msg("all cases cleared")
	return nil
}

// Helper function to convert guestID to string safely
func guestIDStr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
