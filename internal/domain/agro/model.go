package agro

import (
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/common"
)

type FarmRole string
type ActivityType string
type ConversationStatus string
type EventStatus string
type MessageType string
type ReplyType string
type OnboardingMessageDirection string

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

	OnboardingMessageDirectionInbound  OnboardingMessageDirection = "inbound"
	OnboardingMessageDirectionOutbound OnboardingMessageDirection = "outbound"
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

type OnboardingStep string

const (
	OnboardingStepAwaitingProducerName OnboardingStep = "awaiting_producer_name"
	OnboardingStepAwaitingFarmName     OnboardingStep = "awaiting_farm_name"
)

type OnboardingState struct {
	PhoneNumber  string
	Step         OnboardingStep
	ProducerName string
	UpdatedAt    time.Time
}

type HealthTreatmentStep string

const (
	HealthTreatmentStepAwaitingDiagnosisDate HealthTreatmentStep = "awaiting_diagnosis_date"
	HealthTreatmentStepAwaitingMedicine      HealthTreatmentStep = "awaiting_medicine"
	HealthTreatmentStepAwaitingTreatmentDays HealthTreatmentStep = "awaiting_treatment_days"
)

type HealthTreatmentState struct {
	PhoneNumber         string
	FarmID              string
	Category            string
	Subcategory         string
	AnimalCode          string
	Description         string
	Attributes          map[string]string
	DiagnosisDateText   string
	DiagnosisOccurredAt *time.Time
	Medicine            string
	TreatmentDays       int
	Step                HealthTreatmentStep
	UpdatedAt           time.Time
}

type CorrelatedExpenseStep string

const (
	CorrelatedExpenseStepAwaitingDecision       CorrelatedExpenseStep = "awaiting_decision"
	CorrelatedExpenseStepAwaitingMedicineAmount CorrelatedExpenseStep = "awaiting_medicine_amount"
	CorrelatedExpenseStepAwaitingVetAmount      CorrelatedExpenseStep = "awaiting_vet_amount"
	CorrelatedExpenseStepAwaitingExamAmount     CorrelatedExpenseStep = "awaiting_exam_amount"
)

type CorrelatedExpenseState struct {
	PhoneNumber     string
	FarmID          string
	RootEventID     string
	RootCategory    string
	RootSubcategory string
	AnimalCode      string
	Description     string
	OccurredAt      *time.Time
	MedicineAmount  *float64
	VetAmount       *float64
	ExamAmount      *float64
	Step            CorrelatedExpenseStep
	UpdatedAt       time.Time
}

type OnboardingMessage struct {
	ID                string
	PhoneNumber       string
	Step              OnboardingStep
	Direction         OnboardingMessageDirection
	Provider          string
	ProviderMessageID string
	MessageType       MessageType
	Body              string
	CreatedAt         time.Time
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

type MilkWithdrawalAnimal struct {
	AnimalCode    string
	Subcategory   string
	Description   string
	AffectedTeats []string
	OccurredAt    *time.Time
	ActiveUntil   *time.Time
}

func NormalizePhoneNumber(value string) string {
	return common.NormalizePhoneNumber(value)
}

func PhoneNumberLookupCandidates(value string) []string {
	return common.PhoneNumberLookupCandidates(value)
}
