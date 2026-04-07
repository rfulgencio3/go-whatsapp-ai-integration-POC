package agro

import (
	"context"
	"strings"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

func (s *CaptureService) handleContextSwitchRequest(ctx context.Context, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil || s.farmMemberships == nil || s.phoneContexts == nil {
		return false, chatbot.ProcessResult{}, nil
	}
	if !isContextSwitchCommand(message.Text) {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	replyText := ""
	switch len(memberships) {
	case 0:
		replyText = buildUnregisteredNumberReply()
	case 1:
		replyText = buildSingleContextReply(memberships[0].FarmName)
	default:
		options := make([]domain.PhoneContextOption, 0, len(memberships))
		for _, membership := range memberships {
			options = append(options, domain.PhoneContextOption{
				FarmID:   membership.FarmID,
				FarmName: membership.FarmName,
			})
		}
		if err := s.phoneContexts.Upsert(ctx, &domain.PhoneContextState{
			PhoneNumber:    normalizedPhone,
			PendingOptions: options,
			UpdatedAt:      time.Now().UTC(),
		}); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = buildAmbiguousContextSelectionReply(options)
	}

	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := buildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  providerOrDefault(message.Provider),
	}
	if err := s.persistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) resolveMembership(ctx context.Context, phoneNumber string) (domain.FarmMembership, membershipResolution, error) {
	if s.farmMemberships == nil {
		return domain.FarmMembership{}, membershipResolutionUnavailable, nil
	}

	normalized := domain.NormalizePhoneNumber(phoneNumber)
	if normalized == "" {
		s.logger.Info("agro membership resolution skipped for empty phone", map[string]any{
			"raw_phone_number": phoneNumber,
		})
		return domain.FarmMembership{}, membershipResolutionUnavailable, nil
	}

	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalized)
	if err != nil {
		return domain.FarmMembership{}, membershipResolutionUnavailable, err
	}
	s.logger.Info("agro membership lookup completed", map[string]any{
		"raw_phone_number":        phoneNumber,
		"normalized_phone_number": normalized,
		"matches":                 len(memberships),
	})
	if len(memberships) > 1 && s.phoneContexts != nil {
		state, found, err := s.phoneContexts.GetByPhoneNumber(ctx, normalized)
		if err != nil {
			return domain.FarmMembership{}, membershipResolutionUnavailable, err
		}
		if found && strings.TrimSpace(state.ActiveFarmID) != "" {
			for _, membership := range memberships {
				if membership.FarmID == state.ActiveFarmID {
					return membership, membershipResolutionResolved, nil
				}
			}
		}
	}
	switch len(memberships) {
	case 0:
		return domain.FarmMembership{}, membershipResolutionNotFound, nil
	case 1:
		s.logger.Info("agro membership resolved", map[string]any{
			"normalized_phone_number": normalized,
			"farm_id":                 memberships[0].FarmID,
			"farm_name":               memberships[0].FarmName,
		})
		return memberships[0], membershipResolutionResolved, nil
	default:
		s.logger.Info("agro context is ambiguous for inbound phone", map[string]any{
			"phone_number":      normalized,
			"matching_contexts": len(memberships),
		})
		return domain.FarmMembership{}, membershipResolutionAmbiguous, nil
	}
}

func (s *CaptureService) handleUnresolvedMembership(ctx context.Context, resolution membershipResolution, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	replyText := ""
	switch resolution {
	case membershipResolutionNotFound:
		replyText = buildUnregisteredNumberReply()
	case membershipResolutionAmbiguous:
		handled, responseText, err := s.handleAmbiguousMembershipSelection(ctx, normalizedPhone, message.Text)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if !handled {
			return false, chatbot.ProcessResult{}, nil
		}
		replyText = responseText
	default:
		return false, chatbot.ProcessResult{}, nil
	}

	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := buildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  providerOrDefault(message.Provider),
	}
	if err := s.persistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) handleAmbiguousMembershipSelection(ctx context.Context, phoneNumber, text string) (bool, string, error) {
	if s.farmMemberships == nil || s.phoneContexts == nil {
		return true, buildAmbiguousContextReply(), nil
	}

	state, found, err := s.phoneContexts.GetByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return false, "", err
	}
	if found && len(state.PendingOptions) > 0 {
		selection := parseContextSelection(text)
		if selection >= 1 && selection <= len(state.PendingOptions) {
			option := state.PendingOptions[selection-1]
			state.ActiveFarmID = option.FarmID
			state.PendingOptions = nil
			state.UpdatedAt = time.Now().UTC()
			if err := s.phoneContexts.Upsert(ctx, &state); err != nil {
				return false, "", err
			}

			return true, buildSelectedContextReply(option.FarmName), nil
		}
	}

	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return false, "", err
	}
	if len(memberships) == 0 {
		return true, buildUnregisteredNumberReply(), nil
	}

	options := make([]domain.PhoneContextOption, 0, len(memberships))
	for _, membership := range memberships {
		options = append(options, domain.PhoneContextOption{
			FarmID:   membership.FarmID,
			FarmName: membership.FarmName,
		})
	}
	state = domain.PhoneContextState{
		PhoneNumber:    phoneNumber,
		PendingOptions: options,
		UpdatedAt:      time.Now().UTC(),
	}
	if err := s.phoneContexts.Upsert(ctx, &state); err != nil {
		return false, "", err
	}

	return true, buildAmbiguousContextSelectionReply(options), nil
}
