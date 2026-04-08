package agro

import (
	"context"
	"strings"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type CaptureService struct {
	logger             *observability.Logger
	downstream         chatbot.MessageProcessor
	messageSender      chatbot.MessageSender
	replyFormatter     ReplyFormatter
	workflowRouter     WorkflowRouter
	persistence        CapturePersistence
	healthFlow         HealthTreatmentFlow
	correlatedExpenses CorrelatedExpenseFlow
	businessQueries    BusinessQueryFlow
	chatHistory        chatbot.ConversationRepository
	messageArchive     chatbot.MessageArchive
	interpreter        Interpreter
	farmMemberships    FarmMembershipRepository
	farmRegistrations  FarmRegistrationRepository
	phoneContexts      PhoneContextStateRepository
	onboardingStates   OnboardingStateRepository
	healthStates       HealthTreatmentStateRepository
	correlatedStates   CorrelatedExpenseStateRepository
	onboardingMessages OnboardingMessageRepository
	conversations      ConversationRepository
	sourceMessages     SourceMessageRepository
	transcriptions     TranscriptionRepository
	interpretationRuns InterpretationRunRepository
	businessEvents     BusinessEventRepository
	assistantMessages  AssistantMessageRepository
}

type membershipResolution string

const (
	membershipResolutionUnavailable membershipResolution = "unavailable"
	membershipResolutionNotFound    membershipResolution = "not_found"
	membershipResolutionAmbiguous   membershipResolution = "ambiguous"
	membershipResolutionResolved    membershipResolution = "resolved"
	membershipResolutionSelected    membershipResolution = "selected"
)

func NewCaptureService(
	logger *observability.Logger,
	downstream chatbot.MessageProcessor,
	messageSender chatbot.MessageSender,
	chatHistory chatbot.ConversationRepository,
	messageArchive chatbot.MessageArchive,
	interpreter Interpreter,
	farmMemberships FarmMembershipRepository,
	farmRegistrations FarmRegistrationRepository,
	phoneContexts PhoneContextStateRepository,
	onboardingStates OnboardingStateRepository,
	conversations ConversationRepository,
	sourceMessages SourceMessageRepository,
	transcriptions TranscriptionRepository,
	interpretationRuns InterpretationRunRepository,
	businessEvents BusinessEventRepository,
	assistantMessages AssistantMessageRepository,
	onboardingMessages ...OnboardingMessageRepository,
) *CaptureService {
	if logger == nil {
		logger = observability.NewLogger()
	}

	var onboardingMessageRepository OnboardingMessageRepository
	if len(onboardingMessages) > 0 {
		onboardingMessageRepository = onboardingMessages[0]
	}

	return &CaptureService{
		logger:         logger,
		downstream:     downstream,
		messageSender:  messageSender,
		replyFormatter: defaultReplyFormatter{},
		workflowRouter: defaultWorkflowRouter{},
		persistence: newDefaultCapturePersistence(
			chatHistory,
			messageArchive,
			interpreter,
			transcriptions,
			interpretationRuns,
			businessEvents,
			assistantMessages,
		),
		chatHistory:        chatHistory,
		messageArchive:     messageArchive,
		interpreter:        interpreter,
		farmMemberships:    farmMemberships,
		farmRegistrations:  farmRegistrations,
		phoneContexts:      phoneContexts,
		onboardingStates:   onboardingStates,
		onboardingMessages: onboardingMessageRepository,
		conversations:      conversations,
		sourceMessages:     sourceMessages,
		transcriptions:     transcriptions,
		interpretationRuns: interpretationRuns,
		businessEvents:     businessEvents,
		assistantMessages:  assistantMessages,
	}
}

func (s *CaptureService) SetHealthTreatmentStateRepository(repository HealthTreatmentStateRepository) {
	s.healthStates = repository
	s.healthFlow = newDefaultHealthTreatmentFlow(s)
}

func (s *CaptureService) SetCorrelatedExpenseStateRepository(repository CorrelatedExpenseStateRepository) {
	s.correlatedStates = repository
	s.correlatedExpenses = newDefaultCorrelatedExpenseFlow(s)
}

func (s *CaptureService) EnableBusinessQueryFlow() {
	s.businessQueries = newDefaultBusinessQueryFlow(s)
}

func (s *CaptureService) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) (chatbot.ProcessResult, error) {
	if s.downstream == nil {
		return chatbot.ProcessResult{}, nil
	}

	handled, result, err := s.handleOnboarding(ctx, message)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if handled {
		return result, nil
	}

	handled, result, err = s.handleContextSwitchRequest(ctx, message)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if handled {
		return result, nil
	}

	membership, resolution, err := s.resolveMembership(ctx, message.PhoneNumber)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}

	resolved := resolution == membershipResolutionResolved
	if resolved {
		handled, result, err := s.handleConfirmationMessage(ctx, membership, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
		if s.healthFlow != nil {
			handled, result, err = s.healthFlow.HandleIncomingMessage(ctx, membership, message)
			if err != nil {
				return chatbot.ProcessResult{}, err
			}
			if handled {
				return result, nil
			}
		}
		if s.correlatedExpenses != nil {
			handled, result, err = s.correlatedExpenses.HandleIncomingMessage(ctx, membership, message)
			if err != nil {
				return chatbot.ProcessResult{}, err
			}
			if handled {
				return result, nil
			}
		}
		if s.businessQueries != nil {
			handled, result, err = s.businessQueries.HandleIncomingMessage(ctx, membership, message)
			if err != nil {
				return chatbot.ProcessResult{}, err
			}
			if handled {
				return result, nil
			}
		}
	}

	if !resolved {
		handled, result, err := s.handleUnresolvedMembership(ctx, resolution, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
	}

	result, err = s.downstream.ProcessIncomingMessage(ctx, message)
	if err != nil || result.Duplicate {
		return result, err
	}

	if err := s.captureProcessedInteraction(ctx, membership, resolved, result); err != nil {
		s.logger.Error("agro inbound capture failed", map[string]any{
			"phone_number": domain.NormalizePhoneNumber(result.PhoneNumber),
			"message_id":   strings.TrimSpace(result.IncomingMessage.MessageID),
			"provider":     strings.TrimSpace(result.IncomingMessage.Provider),
			"error":        err.Error(),
		})
	}

	return result, nil
}
