package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

type FarmMembershipRepository struct {
	database *sql.DB
}

type ConversationRepository struct {
	database *sql.DB
}

type SourceMessageRepository struct {
	database *sql.DB
}

type AssistantMessageRepository struct {
	database *sql.DB
}

func NewFarmMembershipRepository(database *sql.DB) *FarmMembershipRepository {
	return &FarmMembershipRepository{database: database}
}

func NewConversationRepository(database *sql.DB) *ConversationRepository {
	return &ConversationRepository{database: database}
}

func NewSourceMessageRepository(database *sql.DB) *SourceMessageRepository {
	return &SourceMessageRepository{database: database}
}

func NewAssistantMessageRepository(database *sql.DB) *AssistantMessageRepository {
	return &AssistantMessageRepository{database: database}
}

func (r *FarmMembershipRepository) FindActiveByPhoneNumber(ctx context.Context, phoneNumber string) ([]agro.FarmMembership, error) {
	rows, err := r.database.QueryContext(
		ctx,
		`SELECT id, farm_id, person_name, phone_number, role, is_primary, status, verified_at, created_at, updated_at
		FROM farm_memberships
		WHERE phone_number = $1 AND status = 'active'
		ORDER BY is_primary DESC, created_at ASC`,
		agro.NormalizePhoneNumber(phoneNumber),
	)
	if err != nil {
		return nil, fmt.Errorf("query farm memberships by phone: %w", err)
	}
	defer rows.Close()

	var memberships []agro.FarmMembership
	for rows.Next() {
		var membership agro.FarmMembership
		var verifiedAt sql.NullTime
		if err := rows.Scan(
			&membership.ID,
			&membership.FarmID,
			&membership.PersonName,
			&membership.PhoneNumber,
			&membership.Role,
			&membership.IsPrimary,
			&membership.Status,
			&verifiedAt,
			&membership.CreatedAt,
			&membership.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan farm membership: %w", err)
		}
		if verifiedAt.Valid {
			t := verifiedAt.Time
			membership.VerifiedAt = &t
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate farm memberships: %w", err)
	}

	return memberships, nil
}

func (r *ConversationRepository) GetOrCreateOpen(ctx context.Context, farmID, channel, senderPhoneNumber string, lastMessageAt time.Time) (agro.Conversation, error) {
	var conversation agro.Conversation
	row := r.database.QueryRowContext(
		ctx,
		`SELECT id, farm_id, channel, sender_phone_number, status, last_message_at, created_at, updated_at
		FROM conversations
		WHERE farm_id = $1 AND channel = $2 AND sender_phone_number = $3 AND status = 'open'
		ORDER BY updated_at DESC
		LIMIT 1`,
		farmID,
		channel,
		agro.NormalizePhoneNumber(senderPhoneNumber),
	)
	err := row.Scan(
		&conversation.ID,
		&conversation.FarmID,
		&conversation.Channel,
		&conversation.SenderPhoneNumber,
		&conversation.Status,
		&conversation.LastMessageAt,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err == nil {
		return conversation, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return agro.Conversation{}, fmt.Errorf("query open conversation: %w", err)
	}

	if lastMessageAt.IsZero() {
		lastMessageAt = time.Now().UTC()
	}
	conversation = agro.Conversation{
		ID:                uuid.NewString(),
		FarmID:            farmID,
		Channel:           channel,
		SenderPhoneNumber: agro.NormalizePhoneNumber(senderPhoneNumber),
		Status:            agro.ConversationStatusOpen,
		LastMessageAt:     lastMessageAt,
		CreatedAt:         lastMessageAt,
		UpdatedAt:         lastMessageAt,
	}

	_, err = r.database.ExecContext(
		ctx,
		`INSERT INTO conversations (
			id,
			farm_id,
			channel,
			sender_phone_number,
			status,
			last_message_at,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		conversation.ID,
		conversation.FarmID,
		conversation.Channel,
		conversation.SenderPhoneNumber,
		string(conversation.Status),
		conversation.LastMessageAt,
		conversation.CreatedAt,
		conversation.UpdatedAt,
	)
	if err != nil {
		return agro.Conversation{}, fmt.Errorf("insert conversation: %w", err)
	}

	return conversation, nil
}

func (r *SourceMessageRepository) Create(ctx context.Context, message *agro.SourceMessage) error {
	if message == nil {
		return fmt.Errorf("create source message: nil message")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	if message.ReceivedAt.IsZero() {
		message.ReceivedAt = message.CreatedAt
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO source_messages (
			id,
			conversation_id,
			provider,
			provider_message_id,
			sender_phone_number,
			message_type,
			raw_text,
			media_url,
			media_content_type,
			media_filename,
			received_at,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		message.ID,
		message.ConversationID,
		message.Provider,
		message.ProviderMessageID,
		agro.NormalizePhoneNumber(message.SenderPhoneNumber),
		string(message.MessageType),
		message.RawText,
		message.MediaURL,
		message.MediaContentType,
		message.MediaFilename,
		message.ReceivedAt,
		message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert source message: %w", err)
	}

	return nil
}

func (r *AssistantMessageRepository) Create(ctx context.Context, message *agro.AssistantMessage) error {
	if message == nil {
		return fmt.Errorf("create assistant message: nil message")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO assistant_messages (
			id,
			conversation_id,
			source_message_id,
			provider,
			provider_message_id,
			reply_type,
			body,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		message.ID,
		message.ConversationID,
		nullIfEmpty(message.SourceMessageID),
		message.Provider,
		nullIfEmpty(message.ProviderMessageID),
		string(message.ReplyType),
		message.Body,
		message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert assistant message: %w", err)
	}

	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}

	return value
}
