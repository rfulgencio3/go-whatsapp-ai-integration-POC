package agro

import (
	"strings"
	"time"
)

type FarmRole string
type ActivityType string
type ConversationStatus string
type EventStatus string
type MessageType string
type ReplyType string

const (
	RoleOwner        FarmRole = "owner"
	RoleManager      FarmRole = "manager"
	RoleWorker       FarmRole = "worker"
	RoleVeterinarian FarmRole = "veterinarian"
	RoleAccountant   FarmRole = "accountant"

	ActivityMilk  ActivityType = "milk"
	ActivityBeef  ActivityType = "beef"
	ActivityMixed ActivityType = "mixed"

	ConversationStatusOpen   ConversationStatus = "open"
	ConversationStatusClosed ConversationStatus = "closed"

	EventStatusDraft     EventStatus = "draft"
	EventStatusConfirmed EventStatus = "confirmed"
	EventStatusRejected  EventStatus = "rejected"
	EventStatusCorrected EventStatus = "corrected"

	MessageTypeText        MessageType = "text"
	MessageTypeAudio       MessageType = "audio"
	MessageTypeImage       MessageType = "image"
	MessageTypeDocument    MessageType = "document"
	MessageTypeInteractive MessageType = "interactive"
	MessageTypeUnsupported MessageType = "unsupported"

	ReplyTypeText         ReplyType = "text"
	ReplyTypeConfirmation ReplyType = "confirmation"
)

type Producer struct {
	ID        string
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Farm struct {
	ID           string
	ProducerID   string
	Name         string
	ActivityType ActivityType
	Timezone     string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type FarmMembership struct {
	ID          string
	FarmID      string
	FarmName    string
	PersonName  string
	PhoneNumber string
	Role        FarmRole
	IsPrimary   bool
	Status      string
	VerifiedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type FarmContext struct {
	ProducerID   string
	FarmID       string
	PhoneNumber  string
	Role         FarmRole
	ActivityType ActivityType
}

type Conversation struct {
	ID                         string
	FarmID                     string
	Channel                    string
	SenderPhoneNumber          string
	PendingConfirmationEventID string
	PendingCorrectionEventID   string
	Status                     ConversationStatus
	LastMessageAt              time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type PhoneContextOption struct {
	FarmID   string
	FarmName string
}

type PhoneContextState struct {
	PhoneNumber    string
	ActiveFarmID   string
	PendingOptions []PhoneContextOption
	UpdatedAt      time.Time
}

type SourceMessage struct {
	ID                string
	ConversationID    string
	Provider          string
	ProviderMessageID string
	SenderPhoneNumber string
	MessageType       MessageType
	RawText           string
	MediaURL          string
	MediaContentType  string
	MediaFilename     string
	ReceivedAt        time.Time
	CreatedAt         time.Time
}

type Transcription struct {
	ID              string
	SourceMessageID string
	Provider        string
	ProviderRef     string
	TranscriptText  string
	Language        string
	DurationSeconds float64
	CreatedAt       time.Time
}

type InterpretationRun struct {
	ID                   string
	SourceMessageID      string
	TranscriptionID      string
	ModelProvider        string
	ModelName            string
	PromptVersion        string
	NormalizedIntent     string
	Confidence           float64
	RequiresConfirmation bool
	RawOutputJSON        string
	CreatedAt            time.Time
}

type BusinessEvent struct {
	ID                  string
	FarmID              string
	SourceMessageID     string
	InterpretationRunID string
	Category            string
	Subcategory         string
	OccurredAt          *time.Time
	Description         string
	Amount              *float64
	Currency            string
	Quantity            *float64
	Unit                string
	AnimalCode          string
	LotCode             string
	PaddockCode         string
	CounterpartyName    string
	Status              EventStatus
	ConfirmedByUser     bool
	ConfirmedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type EventAttribute struct {
	ID              string
	BusinessEventID string
	Key             string
	Value           string
	CreatedAt       time.Time
}

type AssistantMessage struct {
	ID                string
	ConversationID    string
	SourceMessageID   string
	Provider          string
	ProviderMessageID string
	ReplyType         ReplyType
	Body              string
	CreatedAt         time.Time
}

func NormalizePhoneNumber(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(" ", "", "+", "", "-", "", "(", "", ")", "")
	return replacer.Replace(value)
}
