package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

type LearningUseCase struct {
	learningRepo domain.LearningRepository
	settingRepo  domain.SettingRepository
	vectorStore  domain.VectorStore
	embedder     domain.Embedder
	eventBus     domain.EventBus
	logger       zerolog.Logger
}

func NewLearningUseCase(
	learningRepo domain.LearningRepository,
	settingRepo domain.SettingRepository,
	vectorStore domain.VectorStore,
	embedder domain.Embedder,
	eventBus domain.EventBus,
) *LearningUseCase {
	return &LearningUseCase{
		learningRepo: learningRepo,
		settingRepo:  settingRepo,
		vectorStore:  vectorStore,
		embedder:     embedder,
		eventBus:     eventBus,
		logger:       zerolog.New(nil).With().Timestamp().Str("usecase", "learning").Logger(),
	}
}

// publishLearningUpdate broadcasts a learning_update WS event so the admin inbox
// can refresh /api/admin/learning/pending in real-time instead of polling.
func (uc *LearningUseCase) publishLearningUpdate(ctx context.Context, action string, itemID int64) {
	if uc.eventBus == nil {
		return
	}
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventLearningUpdate, map[string]interface{}{
		"action":   action, // "added" | "approved" | "rejected" | "updated"
		"item_id":  itemID,
		"channels": []string{"learning_queue"},
	}, "system")
}

func (uc *LearningUseCase) ListPending(ctx context.Context) ([]*domain.LearningItem, error) {
	items, err := uc.learningRepo.ListByStatus(ctx, domain.LearnPending)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list pending learning items")
		return nil, err
	}
	return items, nil
}

func (uc *LearningUseCase) Approve(ctx context.Context, itemID int64, approverName string) error {
	item, err := uc.learningRepo.Get(ctx, itemID)
	if err != nil {
		uc.logger.Error().Err(err).Int64("item_id", itemID).
			Msg("failed to fetch learning item")
		return err
	}
	if item == nil {
		uc.logger.Warn().Int64("item_id", itemID).
			Msg("learning item not found")
		return errors.New("không tìm thấy mẩu tri thức này")
	}

	docContent := fmt.Sprintf("CÂU HỎI / CHỦ ĐỀ CỦA KHÁCH HÀNG: %s\nCÂU TRẢ LỜI / THÔNG TIN CHÍNH THỨC CỦA ĐÔNG ĐÔ PARTNERS: %s", item.Question, item.Answer)
	docID := fmt.Sprintf("learned_qa_%d", item.ID)

	if uc.embedder != nil && uc.vectorStore != nil {
		vec, err := uc.embedder.EmbedText(ctx, docContent)
		if err != nil {
			uc.logger.Error().Err(err).Int64("item_id", itemID).
				Msg("failed to generate embedding")
			return fmt.Errorf("failed to generate embedding: %w", err)
		}

		if err = uc.vectorStore.Upsert(ctx, []*domain.KnowledgeDocument{
			{
				ID:      docID,
				Content: docContent,
				Source:  "CSKH_Learning",
				Metadata: map[string]interface{}{
					"source":     "CSKH_Learning",
					"session_id": item.SessionID,
					"type":       "learned_qa",
					"question":   item.Question,
				},
			},
		}, [][]float32{vec}); err != nil {
			uc.logger.Error().Err(err).Int64("item_id", itemID).
				Msg("failed to upsert to vector store")
			return fmt.Errorf("failed to upsert to vector store: %w", err)
		}
	}

	if err := uc.learningRepo.MarkStatus(ctx, itemID, domain.LearnApproved, approverName); err != nil {
		uc.logger.Error().Err(err).Int64("item_id", itemID).
			Msg("failed to mark learning item approved")
		return err
	}

	uc.publishLearningUpdate(ctx, "approved", itemID)

	uc.logger.Info().Int64("item_id", itemID).Str("approver_name", approverName).
		Msg("learning item approved")

	return nil
}

func (uc *LearningUseCase) UpdateContent(ctx context.Context, itemID int64, question, answer string) error {
	if err := uc.learningRepo.UpdateContent(ctx, itemID, question, answer); err != nil {
		uc.logger.Error().Err(err).Int64("item_id", itemID).
			Msg("failed to update learning item content")
		return err
	}
	uc.publishLearningUpdate(ctx, "updated", itemID)
	return nil
}

func (uc *LearningUseCase) ApproveWithContent(ctx context.Context, itemID int64, approverName, question, answer string) error {
	if question != "" && answer != "" {
		_ = uc.learningRepo.UpdateContent(ctx, itemID, question, answer)
	}
	return uc.Approve(ctx, itemID, approverName)
}

func (uc *LearningUseCase) Reject(ctx context.Context, itemID int64, approverName string) error {
	if err := uc.learningRepo.MarkStatus(ctx, itemID, domain.LearnRejected, approverName); err != nil {
		uc.logger.Error().Err(err).Int64("item_id", itemID).
			Msg("failed to mark learning item rejected")
		return err
	}

	uc.publishLearningUpdate(ctx, "rejected", itemID)

	uc.logger.Info().Int64("item_id", itemID).Str("approver_name", approverName).
		Msg("learning item rejected")

	return nil
}

func (uc *LearningUseCase) GetSettings(ctx context.Context) (bool, error) {
	val, err := uc.settingRepo.Get(ctx, "auto_learning_enabled", "0")
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to get auto-learning setting")
		return false, err
	}
	return val == "1", nil
}

func (uc *LearningUseCase) SetSettings(ctx context.Context, autoLearnEnabled bool) error {
	val := "0"
	if autoLearnEnabled {
		val = "1"
	}

	if err := uc.settingRepo.Set(ctx, "auto_learning_enabled", val); err != nil {
		uc.logger.Error().Err(err).Msg("failed to set auto-learning setting")
		return err
	}
	return nil
}

func (uc *LearningUseCase) AddPending(ctx context.Context, sessionID, question, answer, createdBy string) (*domain.LearningItem, error) {
	autoEnabled, _ := uc.GetSettings(ctx)

	if autoEnabled {
		// Auto-learning enabled - add as approved and embed to vector store
		item, err := uc.learningRepo.Add(ctx, sessionID, question, answer, domain.LearnApproved, createdBy)
		if err != nil {
			uc.logger.Error().Err(err).Str("session_id", sessionID).
				Msg("failed to add learning item with auto-approval")
			return nil, err
		}

		_ = uc.Approve(ctx, item.ID, "AutoSystem")

		uc.logger.Info().Int64("item_id", item.ID).Str("session_id", sessionID).
			Msg("learning item auto-approved")

		return item, nil
	}

	item, err := uc.learningRepo.Add(ctx, sessionID, question, answer, domain.LearnPending, createdBy)
	if err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to add learning item to pending queue")
		return nil, err
	}

	uc.publishLearningUpdate(ctx, "added", item.ID)
	return item, nil
}

func (uc *LearningUseCase) UpsertVoiceLearning(ctx context.Context, sessionID, question, answer string, durationSeconds int) (*domain.LearningItem, error) {
	items, err := uc.learningRepo.ListByStatus(ctx, domain.LearnPending)
	if err == nil {
		for _, it := range items {
			if it.SessionID == sessionID && it.CreatedBy == "Cuộc gọi thoại (Voice Call)" {
				if durationSeconds > 0 || len(answer) > len(it.Answer) {
					_ = uc.learningRepo.UpdateContent(ctx, it.ID, question, answer)
					return it, nil
				}
			}
		}
	}

	return uc.AddPending(ctx, sessionID, question, answer, "Cuộc gọi thoại (Voice Call)")
}

func (uc *LearningUseCase) ResetLearnedKnowledge(ctx context.Context) (int, error) {
	uc.logger.Warn().Msg("resetting learned knowledge (destructive)")

	var deletedCount int

	if uc.vectorStore != nil {
		count, err := uc.vectorStore.DeleteBySource(ctx, "CSKH_Learning")
		if err == nil {
			deletedCount = count
		} else {
			uc.logger.Warn().Err(err).Msg("failed to delete documents from vector store (continuing)")
		}
	}

	_ = uc.learningRepo.ClearAll(ctx)

	uc.logger.Warn().Int("deleted_count", deletedCount).
		Msg("learned knowledge reset completed")

	return deletedCount, nil
}
