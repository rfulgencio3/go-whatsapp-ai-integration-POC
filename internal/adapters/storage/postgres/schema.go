package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS producers (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS farms (
		id UUID PRIMARY KEY,
		producer_id UUID NOT NULL REFERENCES producers(id),
		name TEXT NOT NULL,
		activity_type TEXT NOT NULL,
		timezone TEXT NOT NULL DEFAULT 'America/Sao_Paulo',
		status TEXT NOT NULL DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_farms_producer_id ON farms(producer_id)`,
	`CREATE TABLE IF NOT EXISTS farm_memberships (
		id UUID PRIMARY KEY,
		farm_id UUID NOT NULL REFERENCES farms(id),
		person_name TEXT,
		phone_number TEXT NOT NULL,
		role TEXT NOT NULL,
		is_primary BOOLEAN NOT NULL DEFAULT FALSE,
		status TEXT NOT NULL DEFAULT 'active',
		verified_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_farm_memberships_farm_phone
		ON farm_memberships(farm_id, phone_number)`,
	`CREATE INDEX IF NOT EXISTS idx_farm_memberships_phone_number
		ON farm_memberships(phone_number)`,
	`CREATE TABLE IF NOT EXISTS conversations (
		id UUID PRIMARY KEY,
		farm_id UUID NOT NULL REFERENCES farms(id),
		channel TEXT NOT NULL,
		sender_phone_number TEXT NOT NULL,
		pending_confirmation_event_id TEXT,
		pending_correction_event_id TEXT,
		status TEXT NOT NULL DEFAULT 'open',
		last_message_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`ALTER TABLE conversations ADD COLUMN IF NOT EXISTS pending_confirmation_event_id TEXT`,
	`ALTER TABLE conversations ADD COLUMN IF NOT EXISTS pending_correction_event_id TEXT`,
	`CREATE INDEX IF NOT EXISTS idx_conversations_farm_phone
		ON conversations(farm_id, sender_phone_number)`,
	`CREATE TABLE IF NOT EXISTS source_messages (
		id UUID PRIMARY KEY,
		conversation_id UUID NOT NULL REFERENCES conversations(id),
		provider TEXT NOT NULL,
		provider_message_id TEXT,
		sender_phone_number TEXT NOT NULL,
		message_type TEXT NOT NULL,
		raw_text TEXT,
		media_url TEXT,
		media_content_type TEXT,
		media_filename TEXT,
		received_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_source_messages_provider_message_id
		ON source_messages(provider, provider_message_id)
		WHERE provider_message_id IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_source_messages_conversation_id
		ON source_messages(conversation_id, received_at DESC)`,
	`CREATE TABLE IF NOT EXISTS transcriptions (
		id UUID PRIMARY KEY,
		source_message_id UUID NOT NULL UNIQUE REFERENCES source_messages(id),
		provider TEXT NOT NULL,
		provider_ref TEXT,
		transcript_text TEXT NOT NULL,
		language TEXT,
		duration_seconds DOUBLE PRECISION,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS interpretation_runs (
		id UUID PRIMARY KEY,
		source_message_id UUID NOT NULL REFERENCES source_messages(id),
		transcription_id UUID REFERENCES transcriptions(id),
		model_provider TEXT NOT NULL,
		model_name TEXT NOT NULL,
		prompt_version TEXT NOT NULL,
		normalized_intent TEXT NOT NULL,
		confidence NUMERIC(5,4),
		requires_confirmation BOOLEAN NOT NULL DEFAULT TRUE,
		raw_output_json JSONB NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_interpretation_runs_source_message_id
		ON interpretation_runs(source_message_id)`,
	`CREATE TABLE IF NOT EXISTS business_events (
		id UUID PRIMARY KEY,
		farm_id UUID NOT NULL REFERENCES farms(id),
		source_message_id UUID NOT NULL REFERENCES source_messages(id),
		interpretation_run_id UUID NOT NULL REFERENCES interpretation_runs(id),
		category TEXT NOT NULL,
		subcategory TEXT NOT NULL,
		occurred_at TIMESTAMPTZ,
		description TEXT NOT NULL,
		amount NUMERIC(14,2),
		currency TEXT,
		quantity NUMERIC(14,3),
		unit TEXT,
		animal_code TEXT,
		lot_code TEXT,
		paddock_code TEXT,
		counterparty_name TEXT,
		status TEXT NOT NULL DEFAULT 'draft',
		confirmed_by_user BOOLEAN NOT NULL DEFAULT FALSE,
		confirmed_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_business_events_farm_id_occurred_at
		ON business_events(farm_id, occurred_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_business_events_category
		ON business_events(farm_id, category, subcategory)`,
	`CREATE TABLE IF NOT EXISTS event_attributes (
		id UUID PRIMARY KEY,
		business_event_id UUID NOT NULL REFERENCES business_events(id) ON DELETE CASCADE,
		attr_key TEXT NOT NULL,
		attr_value TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_event_attributes_event_id
		ON event_attributes(business_event_id)`,
	`CREATE TABLE IF NOT EXISTS assistant_messages (
		id UUID PRIMARY KEY,
		conversation_id UUID NOT NULL REFERENCES conversations(id),
		source_message_id UUID REFERENCES source_messages(id),
		provider TEXT NOT NULL,
		provider_message_id TEXT,
		reply_type TEXT NOT NULL,
		body TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_assistant_messages_conversation_id
		ON assistant_messages(conversation_id, created_at DESC)`,
}

func EnsureSchema(ctx context.Context, database *sql.DB) error {
	for _, statement := range schemaStatements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure poc schema: %w", err)
		}
	}

	return nil
}
