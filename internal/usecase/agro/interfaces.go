package agro

import (
	"context"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

type Interpreter interface {
	Interpret(ctx context.Context, input InterpretationInput) (InterpretationResult, error)
}

type FarmMembershipRepository interface {
	FindActiveByPhoneNumber(ctx context.Context, phoneNumber string) ([]domain.FarmMembership, error)
}

type FarmRegistrationRepository interface {
	CreateInitialRegistration(ctx context.Context, phoneNumber, producerName, farmName string) (domain.FarmMembership, error)
}

type PhoneContextStateRepository interface {
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (domain.PhoneContextState, bool, error)
	Upsert(ctx context.Context, state *domain.PhoneContextState) error
}

type OnboardingStateRepository interface {
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (domain.OnboardingState, bool, error)
	Upsert(ctx context.Context, state *domain.OnboardingState) error
	DeleteByPhoneNumber(ctx context.Context, phoneNumber string) error
}

type OnboardingMessageRepository interface {
	Create(ctx context.Context, message *domain.OnboardingMessage) error
}

type SourceMessageRepository interface {
	Create(ctx context.Context, message *domain.SourceMessage) error
}

type ConversationRepository interface {
	GetOrCreateOpen(ctx context.Context, farmID, channel, senderPhoneNumber string, lastMessageAt time.Time) (domain.Conversation, error)
	SetPendingConfirmationEvent(ctx context.Context, conversationID, eventID string) error
	SetPendingCorrectionEvent(ctx context.Context, conversationID, eventID string) error
}

type TranscriptionRepository interface {
	Create(ctx context.Context, transcription *domain.Transcription) error
}

type InterpretationRunRepository interface {
	Create(ctx context.Context, run *domain.InterpretationRun) error
}

type BusinessEventRepository interface {
	Create(ctx context.Context, event *domain.BusinessEvent) error
	FindByID(ctx context.Context, eventID string) (domain.BusinessEvent, bool, error)
	CreateCorrectionLink(ctx context.Context, eventID, correctedEventID string) error
	UpdateStatus(ctx context.Context, eventID string, status domain.EventStatus, confirmedByUser bool, confirmedAt *time.Time) error
}

type AssistantMessageRepository interface {
	Create(ctx context.Context, message *domain.AssistantMessage) error
}

type InterpretationInput struct {
	MessageType domain.MessageType
	Text        string
	OccurredAt  time.Time
}

type InterpretationResult struct {
	NormalizedIntent     string
	Category             string
	Subcategory          string
	Description          string
	Confidence           float64
	RequiresConfirmation bool
	Amount               *float64
	Currency             string
	Quantity             *float64
	Unit                 string
	OccurredAt           *time.Time
	RawOutputJSON        string
}
