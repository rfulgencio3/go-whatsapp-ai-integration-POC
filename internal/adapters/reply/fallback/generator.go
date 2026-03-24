package fallback

import (
	"context"
	"fmt"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) GenerateReply(_ context.Context, history []chat.Message) (string, error) {
	lastUserMessage := ""
	for index := len(history) - 1; index >= 0; index-- {
		if history[index].Role == chat.UserRole {
			lastUserMessage = history[index].Text
			break
		}
	}

	return fmt.Sprintf("Gemini is not configured. The last received topic was: %q", lastUserMessage), nil
}
