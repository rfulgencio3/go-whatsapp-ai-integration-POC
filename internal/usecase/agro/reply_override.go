package agro

import (
	"context"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type ReplyOverrideResolver struct {
	interpreter Interpreter
}

func NewReplyOverrideResolver(interpreter Interpreter) *ReplyOverrideResolver {
	return &ReplyOverrideResolver{interpreter: interpreter}
}

func (r *ReplyOverrideResolver) ResolveReply(ctx context.Context, message chat.IncomingMessage) (chatbot.ReplyOverride, bool, error) {
	if r == nil || r.interpreter == nil {
		return chatbot.ReplyOverride{}, false, nil
	}

	interpretation, err := r.interpreter.Interpret(ctx, InterpretationInput{
		MessageType: toDomainMessageType(message.Type),
		Text:        strings.TrimSpace(message.Text),
	})
	if err != nil {
		return chatbot.ReplyOverride{}, false, err
	}
	if !interpretation.RequiresConfirmation || strings.TrimSpace(interpretation.NormalizedIntent) == "" {
		return chatbot.ReplyOverride{}, false, nil
	}

	return chatbot.ReplyOverride{
		Text: buildDraftConfirmationPromptFromInterpretation(interpretation),
		Kind: chatbot.ReplyKindConfirmation,
	}, true, nil
}
