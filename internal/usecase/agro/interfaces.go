package agro

import (
	"context"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

type FarmMembershipRepository interface {
	FindActiveByPhoneNumber(ctx context.Context, phoneNumber string) ([]domain.FarmMembership, error)
}

type SourceMessageRepository interface {
	Create(ctx context.Context, message *domain.SourceMessage) error
}

type ConversationRepository interface {
	GetOrCreateOpen(ctx context.Context, farmID, channel, senderPhoneNumber string, lastMessageAt time.Time) (domain.Conversation, error)
}

type TranscriptionRepository interface {
	Create(ctx context.Context, transcription *domain.Transcription) error
}

type InterpretationRunRepository interface {
	Create(ctx context.Context, run *domain.InterpretationRun) error
}

type BusinessEventRepository interface {
	Create(ctx context.Context, event *domain.BusinessEvent) error
}

type AssistantMessageRepository interface {
	Create(ctx context.Context, message *domain.AssistantMessage) error
}
