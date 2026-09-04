package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// ============================================================
// Voice Call UseCase
// ============================================================

type VoiceUseCase struct {
	voiceRepo domain.VoiceCallRepository
	caseRepo  domain.CaseRepository
	eventBus  domain.EventBus
	logger    zerolog.Logger
}

func NewVoiceUseCase(voiceRepo domain.VoiceCallRepository, caseRepo domain.CaseRepository, eventBus domain.EventBus) *VoiceUseCase {
	return &VoiceUseCase{
		voiceRepo: voiceRepo,
		caseRepo:  caseRepo,
		eventBus:  eventBus,
		logger:    zerolog.New(nil).With().Timestamp().Str("usecase", "voice").Logger(),
	}
}

// InitiateCall initiates a WebRTC audio call between customer and CSKH.
func (uc *VoiceUseCase) InitiateCall(ctx context.Context, sessionID string, callerType domain.CallerType, callerID string, calleeType domain.CallerType, calleeID string) (*domain.VoiceCall, error) {
	call := &domain.VoiceCall{
		SessionID:  sessionID,
		CallerType: callerType,
		CallerID:   callerID,
		CalleeType: calleeType,
		CalleeID:   calleeID,
		Status:     domain.CallRinging,
	}

	createdCall, err := uc.voiceRepo.Create(ctx, call)
	if err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to create voice call record")
		return nil, fmt.Errorf("failed to create voice call record: %w", err)
	}

	targetCustomerName := ""
	targetCustomerPhone := ""
	existingCase, _ := uc.caseRepo.Get(ctx, sessionID)
	if existingCase != nil {
		targetCustomerName = existingCase.CustomerName
		targetCustomerPhone = existingCase.CustomerPhone
	} else if callerType == domain.CallerGuest {
		targetCustomerName = callerID
	}

	_, _ = uc.caseRepo.Upsert(ctx, sessionID, nil, targetCustomerName, targetCustomerPhone, domain.StatusNeedsHumanCS, "", "📞 Đang yêu cầu cuộc gọi thoại...")

	targetChannel := sessionID
	if callerType == domain.CallerGuest {
		targetChannel = "admin_inbox"
	}

	_ = uc.eventBus.PublishWS(ctx, targetChannel, domain.WSEventCallRing, map[string]interface{}{
		"call_id":     createdCall.ID,
		"session_id":  sessionID,
		"caller_type": callerType,
		"caller_id":   callerID,
	}, callerID)

	uc.logger.Info().Int64("call_id", createdCall.ID).Str("session_id", sessionID).
		Str("caller_type", string(callerType)).Str("caller_id", callerID).
		Msg("voice call initiated")

	return createdCall, nil
}

func (uc *VoiceUseCase) EndCall(ctx context.Context, callID int64, sessionID string, durationSeconds int, recordingURL string) error {
	if err := uc.voiceRepo.End(ctx, callID, durationSeconds, recordingURL); err != nil {
		uc.logger.Error().Err(err).Int64("call_id", callID).
			Msg("failed to end voice call")
		return err
	}

	if sessionID != "" {
		lastMsg := fmt.Sprintf("📞 Cuộc gọi thoại đã kết thúc (%d giây)", durationSeconds)
		if durationSeconds <= 0 {
			lastMsg = "📞 Cuộc gọi thoại đã kết thúc"
		}

		existingCase, _ := uc.caseRepo.Get(ctx, sessionID)
		if existingCase != nil {
			status := existingCase.Status
			if status == domain.StatusNeedsHumanCS {
				status = domain.StatusHumanCSActive
			}

			_, _ = uc.caseRepo.Upsert(ctx, sessionID, existingCase.GuestID, existingCase.CustomerName, existingCase.CustomerPhone, status, existingCase.AssignedCS, lastMsg)
		}
	}

	payload := map[string]interface{}{
		"call_id":          callID,
		"session_id":       sessionID,
		"duration_seconds": durationSeconds,
		"recording_url":    recordingURL,
	}

	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventCallEnd, payload, "system")
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallEnd, payload, "system")
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"type":       "call_ended",
		"session_id": sessionID,
	}, "system")

	uc.logger.Info().Int64("call_id", callID).Str("session_id", sessionID).
		Int("duration_seconds", durationSeconds).
		Msg("voice call ended")

	return nil
}

// MarkMissedCall marks a voice call as missed (no one answered)
func (uc *VoiceUseCase) MarkMissedCall(ctx context.Context, callID int64, sessionID string) error {
	if err := uc.voiceRepo.MarkMissed(ctx, callID); err != nil {
		uc.logger.Error().Err(err).Int64("call_id", callID).
			Msg("failed to mark voice call as missed")
		return err
	}

	// Update case with missed call message
	if sessionID != "" {
		lastMsg := "📞 Cuộc gọi thoại - Không ai nghe máy (Gọi nhỡ)"

		existingCase, _ := uc.caseRepo.Get(ctx, sessionID)
		if existingCase != nil {
			_, _ = uc.caseRepo.Upsert(ctx, sessionID, existingCase.GuestID, existingCase.CustomerName, existingCase.CustomerPhone, existingCase.Status, existingCase.AssignedCS, lastMsg)
		}
	}

	payload := map[string]interface{}{
		"call_id":    callID,
		"session_id": sessionID,
		"status":     "MISSED",
	}

	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventCallEnd, payload, "system")
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallEnd, payload, "system")
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"type":       "call_missed",
		"session_id": sessionID,
	}, "system")

	uc.logger.Info().Int64("call_id", callID).Str("session_id", sessionID).
		Msg("voice call marked as missed")

	return nil
}

func (uc *VoiceUseCase) GetCallsBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	calls, err := uc.voiceRepo.GetBySession(ctx, sessionID)
	if err != nil {
		uc.logger.Error().Err(err).Str("session_id", sessionID).
			Msg("failed to fetch voice calls")
		return nil, err
	}
	return calls, nil
}

func (uc *VoiceUseCase) ListAllCalls(ctx context.Context) ([]*domain.VoiceCall, error) {
	calls, err := uc.voiceRepo.ListAll(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list voice calls")
		return nil, err
	}
	return calls, nil
}

func (uc *VoiceUseCase) SetTranscript(ctx context.Context, id int64, transcript string) error {
	if err := uc.voiceRepo.SetTranscript(ctx, id, transcript); err != nil {
		uc.logger.Error().Err(err).Int64("call_id", id).
			Msg("failed to set transcript")
		return err
	}
	return nil
}

func (uc *VoiceUseCase) DeleteCall(ctx context.Context, id int64) error {
	if err := uc.voiceRepo.Delete(ctx, id); err != nil {
		uc.logger.Error().Err(err).Int64("call_id", id).
			Msg("failed to delete voice call")
		return err
	}
	return nil
}

// ============================================================
// Analytics & Config UseCase
// ============================================================

type AnalyticsUseCase struct {
	analyticsRepo domain.AnalyticsRepository
	settingRepo   domain.SettingRepository
	logger        zerolog.Logger
}

func NewAnalyticsUseCase(analyticsRepo domain.AnalyticsRepository, settingRepo domain.SettingRepository) *AnalyticsUseCase {
	return &AnalyticsUseCase{
		analyticsRepo: analyticsRepo,
		settingRepo:   settingRepo,
		logger:        zerolog.New(nil).With().Timestamp().Str("usecase", "analytics").Logger(),
	}
}

func (uc *AnalyticsUseCase) GetDashboardStats(ctx context.Context) (*domain.AnalyticsStats, error) {
	stats, err := uc.analyticsRepo.GetStats(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch dashboard stats")
		return nil, err
	}
	return stats, nil
}

func (uc *AnalyticsUseCase) GetSystemConfig(ctx context.Context, defaultPrompt, defaultModel string, defaultTemp float64) (prompt, model string, temp float64, err error) {
	prompt, _ = uc.settingRepo.Get(ctx, "system_prompt", defaultPrompt)
	model, _ = uc.settingRepo.Get(ctx, "llm_model", defaultModel)
	tempStr, _ := uc.settingRepo.Get(ctx, "temperature", fmt.Sprintf("%.1f", defaultTemp))

	var parsedTemp float64
	_, _ = fmt.Sscanf(tempStr, "%f", &parsedTemp)
	if parsedTemp == 0 {
		parsedTemp = defaultTemp
	}

	return prompt, model, parsedTemp, nil
}

func (uc *AnalyticsUseCase) SaveSystemConfig(ctx context.Context, prompt, model string, temp float64) error {
	if prompt == "" || model == "" {
		return errors.New("system prompt và model không được để trống")
	}

	_ = uc.settingRepo.Set(ctx, "system_prompt", prompt)
	_ = uc.settingRepo.Set(ctx, "llm_model", model)
	_ = uc.settingRepo.Set(ctx, "temperature", fmt.Sprintf("%.1f", temp))

	return nil
}
