package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// Voice Call UseCase
// ============================================================

type VoiceUseCase struct {
	voiceRepo domain.VoiceCallRepository
	caseRepo  domain.CaseRepository
	eventBus  domain.EventBus
}

func NewVoiceUseCase(voiceRepo domain.VoiceCallRepository, caseRepo domain.CaseRepository, eventBus domain.EventBus) *VoiceUseCase {
	return &VoiceUseCase{
		voiceRepo: voiceRepo,
		caseRepo:  caseRepo,
		eventBus:  eventBus,
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
		return nil, fmt.Errorf("failed to create voice call record: %w", err)
	}

	// Update case status to indicate incoming call without overwriting customer name if CS is caller
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

	// Broadcast ringing event to callee
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

	return createdCall, nil
}

func (uc *VoiceUseCase) EndCall(ctx context.Context, callID int64, sessionID string, durationSeconds int, recordingURL string) error {
	err := uc.voiceRepo.End(ctx, callID, durationSeconds, recordingURL)
	if err != nil {
		return err
	}

	// Update case status & last message when call ends
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

	return nil
}

func (uc *VoiceUseCase) GetCallsBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	return uc.voiceRepo.GetBySession(ctx, sessionID)
}

func (uc *VoiceUseCase) ListAllCalls(ctx context.Context) ([]*domain.VoiceCall, error) {
	return uc.voiceRepo.ListAll(ctx)
}

func (uc *VoiceUseCase) SetTranscript(ctx context.Context, id int64, transcript string) error {
	return uc.voiceRepo.SetTranscript(ctx, id, transcript)
}

func (uc *VoiceUseCase) DeleteCall(ctx context.Context, id int64) error {
	return uc.voiceRepo.Delete(ctx, id)
}

// ============================================================
// Analytics & Config UseCase
// ============================================================

type AnalyticsUseCase struct {
	analyticsRepo domain.AnalyticsRepository
	settingRepo   domain.SettingRepository
}

func NewAnalyticsUseCase(analyticsRepo domain.AnalyticsRepository, settingRepo domain.SettingRepository) *AnalyticsUseCase {
	return &AnalyticsUseCase{
		analyticsRepo: analyticsRepo,
		settingRepo:   settingRepo,
	}
}

func (uc *AnalyticsUseCase) GetDashboardStats(ctx context.Context) (*domain.AnalyticsStats, error) {
	return uc.analyticsRepo.GetStats(ctx)
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
