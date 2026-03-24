package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

const (
	startupTimeout  = 5 * time.Second
	stateProcessing = "processing"
	stateProcessed  = "processed"
)

var acquireScript = redisv9.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 1 or redis.call("EXISTS", KEYS[2]) == 1 then
	return 0
end
redis.call("SET", KEYS[2], ARGV[1], "EX", ARGV[2])
return 1
`)

var markProcessedScript = redisv9.NewScript(`
redis.call("DEL", KEYS[2])
redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
return 1
`)

type Store struct {
	client               *redisv9.Client
	idempotencyTTL       time.Duration
	processingTTL        time.Duration
	idempotencyKeyPrefix string
	processingKeyPrefix  string
}

func NewStore(ctx context.Context, redisURL string, idempotencyTTL, processingTTL time.Duration, idempotencyKeyPrefix, processingKeyPrefix string) (*Store, error) {
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

	return &Store{
		client:               client,
		idempotencyTTL:       idempotencyTTL,
		processingTTL:        processingTTL,
		idempotencyKeyPrefix: idempotencyKeyPrefix,
		processingKeyPrefix:  processingKeyPrefix,
	}, nil
}

func (s *Store) Acquire(ctx context.Context, messageID string) (bool, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return true, nil
	}

	result, err := acquireScript.Run(ctx, s.client, []string{s.idempotencyKey(messageID), s.processingKey(messageID)}, stateProcessing, int(s.processingTTL.Seconds())).Int()
	if err != nil {
		return false, fmt.Errorf("acquire message state: %w", err)
	}

	return result == 1, nil
}

func (s *Store) MarkProcessed(ctx context.Context, messageID string) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil
	}

	if _, err := markProcessedScript.Run(ctx, s.client, []string{s.idempotencyKey(messageID), s.processingKey(messageID)}, stateProcessed, int(s.idempotencyTTL.Seconds())).Result(); err != nil {
		return fmt.Errorf("mark message as processed: %w", err)
	}

	return nil
}

func (s *Store) Release(ctx context.Context, messageID string) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil
	}

	if err := s.client.Del(ctx, s.processingKey(messageID)).Err(); err != nil {
		return fmt.Errorf("release message state: %w", err)
	}

	return nil
}

func (s *Store) idempotencyKey(messageID string) string {
	return buildKey(s.idempotencyKeyPrefix, messageID, "webhook:idempotency")
}

func (s *Store) processingKey(messageID string) string {
	return buildKey(s.processingKeyPrefix, messageID, "webhook:processing")
}

func buildKey(prefix, messageID, fallback string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = fallback
	}

	if strings.HasSuffix(prefix, ":") {
		return prefix + messageID
	}

	return prefix + ":" + messageID
}
