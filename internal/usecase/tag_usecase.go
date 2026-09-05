package usecase

import (
	"context"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// ChatTagUseCase handles business logic for Chat Tags and Alert Config.
type ChatTagUseCase struct {
	tagRepo domain.ChatTagRepository
	logger  zerolog.Logger
}

// NewChatTagUseCase constructs a ChatTagUseCase.
func NewChatTagUseCase(tagRepo domain.ChatTagRepository) *ChatTagUseCase {
	return &ChatTagUseCase{
		tagRepo: tagRepo,
		logger:  zerolog.New(nil).With().Timestamp().Str("usecase", "chat_tag").Logger(),
	}
}

// ============================================================
// Tag CRUD
// ============================================================

func (uc *ChatTagUseCase) ListTags(ctx context.Context) ([]*domain.ChatTag, error) {
	tags, err := uc.tagRepo.ListTags(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list tags")
		return nil, err
	}
	return tags, nil
}

func (uc *ChatTagUseCase) CreateTag(ctx context.Context, name, description, color, createdBy string) (*domain.ChatTag, error) {
	if name == "" {
		return nil, fmt.Errorf("tên tag không được để trống")
	}
	if color == "" {
		color = "#6366f1"
	}
	tag := &domain.ChatTag{
		Name:        name,
		Description: description,
		Color:       color,
		CreatedBy:   createdBy,
		IsActive:    true,
	}
	created, err := uc.tagRepo.CreateTag(ctx, tag)
	if err != nil {
		uc.logger.Error().Err(err).Str("name", name).Msg("failed to create tag")
		return nil, err
	}
	return created, nil
}

func (uc *ChatTagUseCase) UpdateTag(ctx context.Context, id int64, name, description, color string) error {
	if name == "" {
		return fmt.Errorf("tên tag không được để trống")
	}
	return uc.tagRepo.UpdateTag(ctx, id, name, description, color)
}

func (uc *ChatTagUseCase) DeleteTag(ctx context.Context, id int64) error {
	return uc.tagRepo.DeleteTag(ctx, id)
}

// ============================================================
// Case Tag operations
// ============================================================

func (uc *ChatTagUseCase) GetCaseTags(ctx context.Context, sessionID string) ([]*domain.CaseTag, error) {
	return uc.tagRepo.GetCaseTags(ctx, sessionID)
}

func (uc *ChatTagUseCase) AttachTag(ctx context.Context, sessionID string, tagID int64, assignedBy string) error {
	if sessionID == "" {
		return fmt.Errorf("session_id không được để trống")
	}
	return uc.tagRepo.AttachTag(ctx, sessionID, tagID, assignedBy)
}

func (uc *ChatTagUseCase) DetachTag(ctx context.Context, sessionID string, tagID int64, performedBy string) error {
	if sessionID == "" {
		return fmt.Errorf("session_id không được để trống")
	}
	return uc.tagRepo.DetachTag(ctx, sessionID, tagID, performedBy)
}

// ============================================================
// Alert Config
// ============================================================

func (uc *ChatTagUseCase) GetAlertConfig(ctx context.Context) (*domain.AlertConfig, error) {
	return uc.tagRepo.GetAlertConfig(ctx)
}

func (uc *ChatTagUseCase) SaveAlertConfig(ctx context.Context, isEnabled bool, timeoutSeconds int, alertContent, updatedBy string) (*domain.AlertConfig, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	cfg := &domain.AlertConfig{
		IsEnabled:      isEnabled,
		TimeoutSeconds: timeoutSeconds,
		AlertContent:   alertContent,
		UpdatedBy:      updatedBy,
	}
	if err := uc.tagRepo.UpsertAlertConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return uc.tagRepo.GetAlertConfig(ctx)
}

// ============================================================
// Alert Events
// ============================================================

func (uc *ChatTagUseCase) CreateAlertEvent(ctx context.Context, sessionID string, timeoutSeconds int) (*domain.AlertEvent, error) {
	return uc.tagRepo.CreateAlertEvent(ctx, sessionID, timeoutSeconds)
}

func (uc *ChatTagUseCase) ResolveAlertEvent(ctx context.Context, sessionID string) error {
	return uc.tagRepo.ResolveAlertEvent(ctx, sessionID)
}

func (uc *ChatTagUseCase) ListUnresolvedAlertEvents(ctx context.Context) ([]*domain.AlertEvent, error) {
	return uc.tagRepo.ListUnresolvedAlertEvents(ctx)
}
