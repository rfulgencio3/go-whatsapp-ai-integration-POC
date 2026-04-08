package agro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

func (s *CaptureService) handleOnboarding(ctx context.Context, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil || s.onboardingStates == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	state, found, err := s.onboardingStates.GetByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	replyText := ""
	traceStep := domain.OnboardingStep("")
	switch {
	case s.workflowRouter.IsOnboardingStartCommand(message.Text):
		replyText, traceStep, err = s.startOnboarding(ctx, normalizedPhone)
	case found && state.Step == domain.OnboardingStepAwaitingProducerName:
		replyText, traceStep, err = s.completeOnboardingProducerStep(ctx, state, message.Text)
	case found && state.Step == domain.OnboardingStepAwaitingFarmName:
		replyText, traceStep, err = s.completeOnboardingFarmStep(ctx, state, message.Text)
	default:
		return false, chatbot.ProcessResult{}, nil
	}
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := s.persistence.BuildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  s.persistence.ProviderOrDefault(message.Provider),
	}
	if err := s.persistence.PersistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.persistOnboardingInteraction(ctx, normalizedPhone, traceStep, message, assistantMessage, now); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) startOnboarding(ctx context.Context, phoneNumber string) (string, domain.OnboardingStep, error) {
	if s.farmMemberships != nil {
		memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, phoneNumber)
		if err != nil {
			return "", "", err
		}
		if len(memberships) > 0 {
			return s.replyFormatter.BuildAlreadyRegisteredReply(), "", nil
		}
	}

	state := domain.OnboardingState{
		PhoneNumber: phoneNumber,
		Step:        domain.OnboardingStepAwaitingProducerName,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.onboardingStates.Upsert(ctx, &state); err != nil {
		return "", "", err
	}

	return "Vamos fazer seu cadastro inicial. Qual o nome do produtor ou responsavel?", domain.OnboardingStepAwaitingProducerName, nil
}

func (s *CaptureService) completeOnboardingProducerStep(ctx context.Context, state domain.OnboardingState, text string) (string, domain.OnboardingStep, error) {
	producerName := strings.TrimSpace(text)
	if producerName == "" {
		return "Envie o nome do produtor ou responsavel para continuar o cadastro.", domain.OnboardingStepAwaitingProducerName, nil
	}

	state.Step = domain.OnboardingStepAwaitingFarmName
	state.ProducerName = producerName
	state.UpdatedAt = time.Now().UTC()
	if err := s.onboardingStates.Upsert(ctx, &state); err != nil {
		return "", "", err
	}

	return "Agora envie o nome da fazenda ou negocio.", domain.OnboardingStepAwaitingFarmName, nil
}

func (s *CaptureService) completeOnboardingFarmStep(ctx context.Context, state domain.OnboardingState, text string) (string, domain.OnboardingStep, error) {
	farmName := strings.TrimSpace(text)
	if farmName == "" {
		return "Envie o nome da fazenda ou negocio para concluir o cadastro.", domain.OnboardingStepAwaitingFarmName, nil
	}
	if s.farmRegistrations == nil {
		return "", "", nil
	}

	membership, err := s.farmRegistrations.CreateInitialRegistration(ctx, state.PhoneNumber, state.ProducerName, farmName)
	if err != nil {
		return "", "", err
	}
	if s.onboardingStates != nil {
		if err := s.onboardingStates.DeleteByPhoneNumber(ctx, state.PhoneNumber); err != nil {
			return "", "", err
		}
	}

	return fmt.Sprintf("Cadastro concluido. Seu numero foi vinculado a %s. Agora voce ja pode enviar registros.", fallbackFarmName(domain.PhoneContextOption{FarmName: membership.FarmName}, 1)), "", nil
}

func (s *CaptureService) persistOnboardingInteraction(ctx context.Context, phoneNumber string, step domain.OnboardingStep, incomingMessage chat.IncomingMessage, assistantMessage chat.Message, createdAt time.Time) error {
	if s.onboardingMessages == nil {
		return nil
	}

	userBody := strings.TrimSpace(incomingMessage.Text)
	if userBody != "" {
		userMessage := domain.OnboardingMessage{
			ID:                uuid.NewString(),
			PhoneNumber:       phoneNumber,
			Step:              step,
			Direction:         domain.OnboardingMessageDirectionInbound,
			Provider:          s.persistence.ProviderOrDefault(incomingMessage.Provider),
			ProviderMessageID: strings.TrimSpace(incomingMessage.MessageID),
			MessageType:       s.persistence.ToDomainMessageType(incomingMessage.Type),
			Body:              userBody,
			CreatedAt:         createdAt,
		}
		if err := s.onboardingMessages.Create(ctx, &userMessage); err != nil {
			return err
		}
	}

	replyBody := strings.TrimSpace(assistantMessage.Text)
	if replyBody == "" {
		return nil
	}

	reply := domain.OnboardingMessage{
		ID:          uuid.NewString(),
		PhoneNumber: phoneNumber,
		Step:        step,
		Direction:   domain.OnboardingMessageDirectionOutbound,
		Provider:    s.persistence.ProviderOrDefault(assistantMessage.Provider),
		MessageType: domain.MessageTypeText,
		Body:        replyBody,
		CreatedAt:   createdAt,
	}
	if err := s.onboardingMessages.Create(ctx, &reply); err != nil {
		return err
	}

	return nil
}
