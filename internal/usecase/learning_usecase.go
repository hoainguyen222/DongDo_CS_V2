package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type LearningUseCase struct {
	learningRepo domain.LearningRepository
	settingRepo  domain.SettingRepository
	vectorStore  domain.VectorStore
	embedder     domain.Embedder
	eventBus     domain.EventBus
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
	return uc.learningRepo.ListByStatus(ctx, domain.LearnPending)
}

func (uc *LearningUseCase) Approve(ctx context.Context, itemID int64, approverName string) error {
	item, err := uc.learningRepo.Get(ctx, itemID)
	if err != nil {
		return err
	}
	if item == nil {
		return errors.New("không tìm thấy mẩu tri thức này")
	}

	// 1. Embed and upsert to Qdrant
	docContent := fmt.Sprintf("CÂU HỎI / CHỦ ĐỀ CỦA KHÁCH HÀNG: %s\nCÂU TRẢ LỜI / THÔNG TIN CHÍNH THỨC CỦA ĐÔNG ĐÔ PARTNERS: %s", item.Question, item.Answer)
	docID := fmt.Sprintf("learned_qa_%d", item.ID)

	if uc.embedder != nil && uc.vectorStore != nil {
		vec, err := uc.embedder.EmbedText(ctx, docContent)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}

		err = uc.vectorStore.Upsert(ctx, []*domain.KnowledgeDocument{
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
		}, [][]float32{vec})
		if err != nil {
			return fmt.Errorf("failed to upsert to vector store: %w", err)
		}
	}

	// 2. Mark status as APPROVED
	if err := uc.learningRepo.MarkStatus(ctx, itemID, domain.LearnApproved, approverName); err != nil {
		return err
	}
	uc.publishLearningUpdate(ctx, "approved", itemID)
	return nil
}

func (uc *LearningUseCase) UpdateContent(ctx context.Context, itemID int64, question, answer string) error {
	if err := uc.learningRepo.UpdateContent(ctx, itemID, question, answer); err != nil {
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
		return err
	}
	uc.publishLearningUpdate(ctx, "rejected", itemID)
	return nil
}

func (uc *LearningUseCase) GetSettings(ctx context.Context) (bool, error) {
	val, err := uc.settingRepo.Get(ctx, "auto_learning_enabled", "0")
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

func (uc *LearningUseCase) SetSettings(ctx context.Context, autoLearnEnabled bool) error {
	val := "0"
	if autoLearnEnabled {
		val = "1"
	}
	return uc.settingRepo.Set(ctx, "auto_learning_enabled", val)
}

func (uc *LearningUseCase) AddPending(ctx context.Context, sessionID, question, answer, createdBy string) (*domain.LearningItem, error) {
	autoEnabled, _ := uc.GetSettings(ctx)
	if autoEnabled {
		item, err := uc.learningRepo.Add(ctx, sessionID, question, answer, domain.LearnApproved, createdBy)
		if err != nil {
			return nil, err
		}
		_ = uc.Approve(ctx, item.ID, "AutoSystem")
		return item, nil
	}
	item, err := uc.learningRepo.Add(ctx, sessionID, question, answer, domain.LearnPending, createdBy)
	if err != nil {
		return nil, err
	}
	uc.publishLearningUpdate(ctx, "added", item.ID)
	return item, nil
}

func (uc *LearningUseCase) UpsertVoiceLearning(ctx context.Context, sessionID, question, answer string, durationSeconds int) (*domain.LearningItem, error) {
	// Check if there is already a pending item for this voice call session
	items, err := uc.learningRepo.ListByStatus(ctx, domain.LearnPending)
	if err == nil {
		for _, it := range items {
			if it.SessionID == sessionID && it.CreatedBy == "Cuộc gọi thoại (Voice Call)" {
				// If incoming has >0 duration or longer content, update the existing pending item
				if durationSeconds > 0 || len(answer) > len(it.Answer) {
					_ = uc.learningRepo.UpdateContent(ctx, it.ID, question, answer)
				}
				return it, nil
			}
		}
	}
	return uc.AddPending(ctx, sessionID, question, answer, "Cuộc gọi thoại (Voice Call)")
}

func (uc *LearningUseCase) ResetLearnedKnowledge(ctx context.Context) (int, error) {
	var deletedCount int
	if uc.vectorStore != nil {
		count, err := uc.vectorStore.DeleteBySource(ctx, "CSKH_Learning")
		if err == nil {
			deletedCount = count
		}
	}
	_ = uc.learningRepo.ClearAll(ctx)
	return deletedCount, nil
}
