package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

const startupTimeout = 5 * time.Second

type MessageArchive struct {
	database *sql.DB
}

func NewMessageArchive(ctx context.Context, databaseURL string) (*MessageArchive, error) {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	startupContext, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	if err := database.PingContext(startupContext); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	archive := &MessageArchive{database: database}
	if err := archive.ensureSchema(startupContext); err != nil {
		_ = database.Close()
		return nil, err
	}

	return archive, nil
}

func (a *MessageArchive) RecordMessage(ctx context.Context, phoneNumber string, message chat.Message) error {
	createdAt := message.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err := a.database.ExecContext(
		ctx,
		`INSERT INTO chat_messages (phone_number, role, body, created_at) VALUES ($1, $2, $3, $4)`,
		phoneNumber,
		string(message.Role),
		message.Text,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("insert chat message: %w", err)
	}

	return nil
}

func (a *MessageArchive) ensureSchema(ctx context.Context) error {
	if _, err := a.database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS chat_messages (
			id BIGSERIAL PRIMARY KEY,
			phone_number TEXT NOT NULL,
			role TEXT NOT NULL,
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create chat_messages table: %w", err)
	}

	if _, err := a.database.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chat_messages_phone_number_created_at
		ON chat_messages (phone_number, created_at DESC)
	`); err != nil {
		return fmt.Errorf("create chat_messages index: %w", err)
	}

	return nil
}
