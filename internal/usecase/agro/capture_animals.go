package agro

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

func (s *CaptureService) handleAnimalRegistration(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.farmAnimals == nil || s.messageSender == nil || s.workflowRouter == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	command, ok := s.workflowRouter.ParseAnimalRegistrationCommand(message.Text)
	if !ok {
		return false, chatbot.ProcessResult{}, nil
	}

	now := time.Now().UTC()
	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	birthDate, _ := parseAnimalLifecycleDate(command.BirthDate)
	firstCalvingDate, _ := parseAnimalLifecycleDate(command.FirstCalvingDate)
	animal := domain.FarmAnimal{
		ID:               uuid.NewString(),
		FarmID:           membership.FarmID,
		AnimalCode:       strings.TrimSpace(strings.ToUpper(command.AnimalCode)),
		DisplayName:      buildAnimalDisplayName(command),
		AnimalType:       strings.TrimSpace(command.AnimalType),
		Sex:              strings.TrimSpace(command.Sex),
		BirthDate:        birthDate,
		MotherAnimalCode: strings.TrimSpace(strings.ToUpper(command.MotherAnimalCode)),
		FirstCalvingDate: firstCalvingDate,
		Status:           "active",
		LastSeenAt:       &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.farmAnimals.Upsert(ctx, &animal); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if s.animalCache != nil {
		s.animalCache.Set(membership.FarmID, animal.AnimalCode, true)
	}

	replyText := s.replyFormatter.BuildAnimalRegisteredReply(animal)
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

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func buildAnimalDisplayName(command animalRegistrationCommand) string {
	animalCode := strings.TrimSpace(strings.ToUpper(command.AnimalCode))
	animalType := strings.TrimSpace(command.AnimalType)
	if animalType == "" {
		return "Animal " + animalCode
	}
	return strings.ToUpper(animalType[:1]) + animalType[1:] + " " + animalCode
}

func parseAnimalLifecycleDate(value string) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	location := time.FixedZone("BRT", -3*60*60)
	for _, layout := range []string{"02/01/2006", "2/1/2006", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			result := parsed.UTC()
			return &result, true
		}
	}
	return nil, false
}

func (s *CaptureService) validateAnimalExists(ctx context.Context, farmID, animalCode string) (bool, error) {
	animalCode = strings.TrimSpace(strings.ToUpper(animalCode))
	if farmID == "" || animalCode == "" || s.farmAnimals == nil {
		return true, nil
	}
	if exists, ok := s.animalCache.Get(farmID, animalCode); ok {
		return exists, nil
	}

	animal, found, err := s.farmAnimals.FindByAnimalCode(ctx, farmID, animalCode)
	if err != nil {
		return false, err
	}
	exists := found && strings.EqualFold(strings.TrimSpace(animal.Status), "active")
	if s.animalCache != nil {
		s.animalCache.Set(farmID, animalCode, exists)
	}
	return exists, nil
}
