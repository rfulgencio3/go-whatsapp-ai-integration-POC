package noop

import (
	"context"
	"log"
)

type Sender struct {
	logger *log.Logger
}

func NewSender(logger *log.Logger) *Sender {
	return &Sender{logger: logger}
}

func (s *Sender) SendTextMessage(_ context.Context, phoneNumber, body string) error {
	s.logger.Printf("whatsapp send skipped: missing configuration phone_number=%s reply=%q", phoneNumber, body)
	return nil
}
