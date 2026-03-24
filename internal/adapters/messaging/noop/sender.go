package noop

import (
	"context"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
)

type Sender struct {
	logger *observability.Logger
}

func NewSender(logger *observability.Logger) *Sender {
	return &Sender{logger: logger}
}

func (s *Sender) SendTextMessage(_ context.Context, phoneNumber, body string) error {
	s.logger.Info("whatsapp send skipped", map[string]any{
		"reason":       "missing_configuration",
		"phone_number": phoneNumber,
		"reply":        body,
	})
	return nil
}
