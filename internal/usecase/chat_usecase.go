package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type ChatUseCase struct {
	messageRepo domain.MessageRepository
	caseRepo    domain.CaseRepository
	eventBus    domain.EventBus
	stateMgr    domain.StateManager
}

func NewChatUseCase(
	messageRepo domain.MessageRepository,
	caseRepo domain.CaseRepository,
	eventBus domain.EventBus,
	stateMgr domain.StateManager,
) *ChatUseCase {
	return &ChatUseCase{
		messageRepo: messageRepo,
		caseRepo:    caseRepo,
		eventBus:    eventBus,
		stateMgr:    stateMgr,
	}
}

// SendGuestMessage handles an incoming message from a guest.
func (uc *ChatUseCase) SendGuestMessage(ctx context.Context, sessionID, customerName, content string, clientMsgID *uuid.UUID) (*domain.Message, error) {
	if content == "" {
		return nil, errors.New("tin nhắn không được để trống")
	}

	msg := &domain.Message{
		SessionID:   sessionID,
		SenderType:  domain.SenderGuest,
		SenderID:    customerName,
		Content:     content,
		ClientMsgID: clientMsgID,
		CreatedAt:   time.Now(),
	}

	// 1. Direct or batch save message
	savedMsg, err := uc.messageRepo.Insert(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// 2. Fetch existing case
	existingCase, err := uc.caseRepo.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	caseStatus := domain.StatusAIActive
	var assignedCS string
	targetCustomerName := customerName
	var targetCustomerPhone string

	if existingCase != nil {
		caseStatus = existingCase.Status
		assignedCS = existingCase.AssignedCS
		targetCustomerPhone = existingCase.CustomerPhone
		if existingCase.CustomerName != "" && existingCase.CustomerName != "Khách hàng" {
			// If incoming customerName is a UUID or empty, keep the human friendly name from registration
			if len(customerName) == 36 || customerName == "" || customerName == "Khách hàng" {
				targetCustomerName = existingCase.CustomerName
			}
		}
	}

	// 3. Upsert case
	_, err = uc.caseRepo.Upsert(ctx, sessionID, nil, targetCustomerName, targetCustomerPhone, caseStatus, assignedCS, content)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert case: %w", err)
	}

	// 4. Publish to WebSocket stream to update UI in real-time
	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventMessage, savedMsg, targetCustomerName)
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id":     sessionID,
		"customer_name":  targetCustomerName,
		"customer_phone": targetCustomerPhone,
		"last_message":   content,
		"status":         caseStatus,
	}, targetCustomerName)

	// 5. If Human CS is actively chatting, increment unread and DO NOT trigger AI
	if caseStatus == domain.StatusHumanCSActive && assignedCS != "" {
		_, _ = uc.stateMgr.IncrementUnread(ctx, sessionID, assignedCS)
		_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventUnread, map[string]interface{}{
			"session_id": sessionID,
			"assigned":   assignedCS,
		}, customerName)
		return savedMsg, nil
	}

	// 6. Otherwise, publish AI Job to stream:ai for asynchronous processing
	_ = uc.eventBus.PublishAIJob(ctx, sessionID, content, customerName, clientMsgID)

	return savedMsg, nil
}

// SendCSReply handles a reply from a CSKH specialist.
func (uc *ChatUseCase) SendCSReply(ctx context.Context, sessionID, csUsername, csFullName, content string) (*domain.Message, error) {
	if content == "" {
		return nil, errors.New("tin nhắn không được để trống")
	}

	senderName := csFullName
	if senderName == "" {
		senderName = csUsername
	}

	msg := &domain.Message{
		SessionID:  sessionID,
		SenderType: domain.SenderHumanCS,
		SenderID:   senderName,
		Content:    content,
		CreatedAt:  time.Now(),
	}

	savedMsg, err := uc.messageRepo.Insert(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to save CS message: %w", err)
	}

	// Update case to HUMAN_CS_ACTIVE preserving customer details
	existingCase, _ := uc.caseRepo.Get(ctx, sessionID)
	targetCustomerName := ""
	targetCustomerPhone := ""
	var targetGuestID *uuid.UUID
	if existingCase != nil {
		targetCustomerName = existingCase.CustomerName
		targetCustomerPhone = existingCase.CustomerPhone
		targetGuestID = existingCase.GuestID
	}
	_, err = uc.caseRepo.Upsert(ctx, sessionID, targetGuestID, targetCustomerName, targetCustomerPhone, domain.StatusHumanCSActive, senderName, content)
	if err != nil {
		return nil, err
	}

	// Clear unread count
	_ = uc.stateMgr.ClearUnread(ctx, sessionID, csUsername)

	// Broadcast via WebSocket
	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventMessage, savedMsg, csUsername)
	_ = uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
		"session_id":     sessionID,
		"customer_name":  targetCustomerName,
		"customer_phone": targetCustomerPhone,
		"status":         domain.StatusHumanCSActive,
		"assigned_cs":    senderName,
		"last_message":   content,
	}, csUsername)

	return savedMsg, nil
}

// GetHistory returns the message history for a given session.
func (uc *ChatUseCase) GetHistory(ctx context.Context, sessionID string) ([]*domain.Message, *domain.ChatCase, error) {
	messages, err := uc.messageRepo.GetHistory(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	chatCase, err := uc.caseRepo.Get(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	return messages, chatCase, nil
}
