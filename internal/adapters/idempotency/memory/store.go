package memory

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mutex          sync.Mutex
	items          map[string]storedState
	idempotencyTTL time.Duration
	processingTTL  time.Duration
}

type storedState struct {
	state     string
	expiresAt time.Time
}

const (
	stateProcessing = "processing"
	stateProcessed  = "processed"
)

func NewStore(idempotencyTTL, processingTTL time.Duration) *Store {
	return &Store{
		items:          make(map[string]storedState),
		idempotencyTTL: idempotencyTTL,
		processingTTL:  processingTTL,
	}
}

func (s *Store) Acquire(_ context.Context, messageID string) (bool, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return true, nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cleanupExpired(time.Now().UTC())
	if _, exists := s.items[messageID]; exists {
		return false, nil
	}

	s.items[messageID] = storedState{
		state:     stateProcessing,
		expiresAt: time.Now().UTC().Add(s.processingTTL),
	}
	return true, nil
}

func (s *Store) MarkProcessed(_ context.Context, messageID string) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.items[messageID] = storedState{
		state:     stateProcessed,
		expiresAt: time.Now().UTC().Add(s.idempotencyTTL),
	}
	return nil
}

func (s *Store) Release(_ context.Context, messageID string) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	state, exists := s.items[messageID]
	if !exists {
		return nil
	}

	if state.state == stateProcessing {
		delete(s.items, messageID)
	}

	return nil
}

func (s *Store) cleanupExpired(now time.Time) {
	for messageID, state := range s.items {
		if !state.expiresAt.IsZero() && !state.expiresAt.After(now) {
			delete(s.items, messageID)
		}
	}
}
