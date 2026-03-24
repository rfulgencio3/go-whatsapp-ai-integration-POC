package memory

import (
	"context"
	"sync"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

const defaultConversationHistoryLimit = 12

type ConversationRepository struct {
	mutex        sync.Mutex
	store        map[string][]chat.Message
	historyLimit int
}

func NewConversationRepository(historyLimit int) *ConversationRepository {
	if historyLimit <= 0 {
		historyLimit = defaultConversationHistoryLimit
	}

	return &ConversationRepository{
		store:        make(map[string][]chat.Message),
		historyLimit: historyLimit,
	}
}

func (r *ConversationRepository) GetMessages(_ context.Context, phoneNumber string) ([]chat.Message, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return append([]chat.Message(nil), r.store[phoneNumber]...), nil
}

func (r *ConversationRepository) AppendMessage(_ context.Context, phoneNumber string, message chat.Message) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}

	r.store[phoneNumber] = append(r.store[phoneNumber], message)
	if len(r.store[phoneNumber]) > r.historyLimit {
		r.store[phoneNumber] = append([]chat.Message(nil), r.store[phoneNumber][len(r.store[phoneNumber])-r.historyLimit:]...)
	}

	return nil
}
