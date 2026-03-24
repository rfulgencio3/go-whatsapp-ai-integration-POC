package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	redisv9 "github.com/redis/go-redis/v9"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

const startupTimeout = 5 * time.Second

type ConversationRepository struct {
	client       *redisv9.Client
	historyLimit int
	ttl          time.Duration
	keyPrefix    string
}

type storedMessage struct {
	Role      chat.MessageRole `json:"role"`
	Text      string           `json:"text"`
	CreatedAt time.Time        `json:"created_at"`
}

func NewConversationRepository(ctx context.Context, redisURL string, historyLimit int, ttl time.Duration, keyPrefix string) (*ConversationRepository, error) {
	options, err := redisv9.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redisv9.NewClient(options)
	startupContext, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	if err := client.Ping(startupContext).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &ConversationRepository{
		client:       client,
		historyLimit: historyLimit,
		ttl:          ttl,
		keyPrefix:    keyPrefix,
	}, nil
}

func (r *ConversationRepository) GetMessages(ctx context.Context, phoneNumber string) ([]chat.Message, error) {
	values, err := r.client.LRange(ctx, r.key(phoneNumber), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("load conversation history: %w", err)
	}

	messages := make([]chat.Message, 0, len(values))
	for _, value := range values {
		var stored storedMessage
		if err := json.Unmarshal([]byte(value), &stored); err != nil {
			return nil, fmt.Errorf("decode conversation message: %w", err)
		}

		messages = append(messages, chat.Message{
			Role:      stored.Role,
			Text:      stored.Text,
			CreatedAt: stored.CreatedAt,
		})
	}

	return messages, nil
}

func (r *ConversationRepository) AppendMessage(ctx context.Context, phoneNumber string, message chat.Message) error {
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}

	payload, err := json.Marshal(storedMessage{
		Role:      message.Role,
		Text:      message.Text,
		CreatedAt: message.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode conversation message: %w", err)
	}

	pipeline := r.client.TxPipeline()
	pipeline.RPush(ctx, r.key(phoneNumber), payload)
	pipeline.LTrim(ctx, r.key(phoneNumber), int64(-r.historyLimit), -1)
	if r.ttl > 0 {
		pipeline.Expire(ctx, r.key(phoneNumber), r.ttl)
	}

	if _, err := pipeline.Exec(ctx); err != nil {
		return fmt.Errorf("append conversation message: %w", err)
	}

	return nil
}

func (r *ConversationRepository) key(phoneNumber string) string {
	prefix := strings.TrimSpace(r.keyPrefix)
	if prefix == "" {
		prefix = "chat:history"
	}

	if strings.HasSuffix(prefix, ":") {
		return prefix + phoneNumber
	}

	return prefix + ":" + phoneNumber
}
