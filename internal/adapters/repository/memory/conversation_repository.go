package memory

import (
	"sync"

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

func (r *ConversationRepository) AppendMessage(phoneNumber string, message chat.Message) []chat.Message {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.store[phoneNumber] = append(r.store[phoneNumber], message)
	if len(r.store[phoneNumber]) > r.historyLimit {
		r.store[phoneNumber] = append([]chat.Message(nil), r.store[phoneNumber][len(r.store[phoneNumber])-r.historyLimit:]...)
	}

	return append([]chat.Message(nil), r.store[phoneNumber]...)
}
