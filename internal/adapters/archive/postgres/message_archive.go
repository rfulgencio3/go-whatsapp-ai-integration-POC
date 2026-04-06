package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pocpostgres "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/storage/postgres"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type MessageArchive struct {
	database *sql.DB
}

func NewMessageArchive(ctx context.Context, databaseURL string) (*MessageArchive, error) {
	database, err := pocpostgres.OpenDatabase(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	archive := NewMessageArchiveWithDatabase(database)
	if err := archive.EnsureSchema(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}

	return archive, nil
}

func NewMessageArchiveWithDatabase(database *sql.DB) *MessageArchive {
	return &MessageArchive{database: database}
}

func (a *MessageArchive) RecordMessage(ctx context.Context, phoneNumber string, message chat.Message) error {
	createdAt := message.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err := a.database.ExecContext(
		ctx,
		`INSERT INTO chat_messages (
			phone_number,
			role,
			body,
			created_at,
			message_type,
			provider,
			provider_message_id,
			media_url,
			media_content_type,
			media_filename,
			transcription_id,
			transcription_language,
			audio_duration_seconds
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		phoneNumber,
		string(message.Role),
		message.Text,
		createdAt,
		string(message.Type),
		message.Provider,
		message.ProviderMessageID,
		message.MediaURL,
		message.MediaContentType,
		message.MediaFilename,
		message.TranscriptionID,
		message.TranscriptionLanguage,
		message.AudioDurationSeconds,
	)
	if err != nil {
		return fmt.Errorf("insert chat message: %w", err)
	}

	return nil
}

func (a *MessageArchive) EnsureSchema(ctx context.Context) error {
	if err := pocpostgres.EnsureSchema(ctx, a.database); err != nil {
		return err
	}

	if _, err := a.database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS chat_messages (
			id BIGSERIAL PRIMARY KEY,
			phone_number TEXT NOT NULL,
			role TEXT NOT NULL,
			body TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			message_type TEXT NOT NULL DEFAULT 'text',
			provider TEXT NOT NULL DEFAULT 'whatsapp',
			provider_message_id TEXT,
			media_url TEXT,
			media_content_type TEXT,
			media_filename TEXT,
			transcription_id TEXT,
			transcription_language TEXT,
			audio_duration_seconds DOUBLE PRECISION
		)
	`); err != nil {
		return fmt.Errorf("create chat_messages table: %w", err)
	}

	schemaUpdates := []string{
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS message_type TEXT NOT NULL DEFAULT 'text'`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'whatsapp'`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS provider_message_id TEXT`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS media_url TEXT`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS media_content_type TEXT`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS media_filename TEXT`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS transcription_id TEXT`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS transcription_language TEXT`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS audio_duration_seconds DOUBLE PRECISION`,
	}
	for _, statement := range schemaUpdates {
		if _, err := a.database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("update chat_messages table: %w", err)
		}
	}

	if _, err := a.database.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chat_messages_phone_number_created_at
		ON chat_messages (phone_number, created_at DESC)
	`); err != nil {
		return fmt.Errorf("create chat_messages index: %w", err)
	}

	return nil
}
