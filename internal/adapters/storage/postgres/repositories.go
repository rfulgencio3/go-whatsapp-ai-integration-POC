package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
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

type PhoneContextStateRepository struct {
	database *sql.DB
}

type SourceMessageRepository struct {
	database *sql.DB
}

type TranscriptionRepository struct {
	database *sql.DB
}

type InterpretationRunRepository struct {
	database *sql.DB
}

type BusinessEventRepository struct {
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

func NewPhoneContextStateRepository(database *sql.DB) *PhoneContextStateRepository {
	return &PhoneContextStateRepository{database: database}
}

func NewSourceMessageRepository(database *sql.DB) *SourceMessageRepository {
	return &SourceMessageRepository{database: database}
}

func NewTranscriptionRepository(database *sql.DB) *TranscriptionRepository {
	return &TranscriptionRepository{database: database}
}

func NewInterpretationRunRepository(database *sql.DB) *InterpretationRunRepository {
	return &InterpretationRunRepository{database: database}
}

func NewBusinessEventRepository(database *sql.DB) *BusinessEventRepository {
	return &BusinessEventRepository{database: database}
}

func NewAssistantMessageRepository(database *sql.DB) *AssistantMessageRepository {
	return &AssistantMessageRepository{database: database}
}

func (r *FarmMembershipRepository) FindActiveByPhoneNumber(ctx context.Context, phoneNumber string) ([]agro.FarmMembership, error) {
	rows, err := r.database.QueryContext(
		ctx,
		`SELECT fm.id, fm.farm_id, f.name, fm.person_name, fm.phone_number, fm.role, fm.is_primary, fm.status, fm.verified_at, fm.created_at, fm.updated_at
		FROM farm_memberships fm
		INNER JOIN farms f ON f.id = fm.farm_id
		WHERE fm.phone_number = $1 AND fm.status = 'active'
		ORDER BY fm.is_primary DESC, fm.created_at ASC`,
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
			&membership.FarmName,
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

func (r *PhoneContextStateRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (agro.PhoneContextState, bool, error) {
	var state agro.PhoneContextState
	var activeFarmID sql.NullString
	var pendingOptionsRaw []byte

	err := r.database.QueryRowContext(
		ctx,
		`SELECT phone_number, active_farm_id, pending_options, updated_at
		FROM phone_context_states
		WHERE phone_number = $1`,
		agro.NormalizePhoneNumber(phoneNumber),
	).Scan(
		&state.PhoneNumber,
		&activeFarmID,
		&pendingOptionsRaw,
		&state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return agro.PhoneContextState{}, false, nil
	}
	if err != nil {
		return agro.PhoneContextState{}, false, fmt.Errorf("query phone context state: %w", err)
	}
	if activeFarmID.Valid {
		state.ActiveFarmID = activeFarmID.String
	}
	if len(pendingOptionsRaw) > 0 {
		if err := json.Unmarshal(pendingOptionsRaw, &state.PendingOptions); err != nil {
			return agro.PhoneContextState{}, false, fmt.Errorf("decode phone context pending options: %w", err)
		}
	}

	return state, true, nil
}

func (r *PhoneContextStateRepository) Upsert(ctx context.Context, state *agro.PhoneContextState) error {
	if state == nil {
		return fmt.Errorf("upsert phone context state: nil state")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}

	pendingOptionsRaw, err := json.Marshal(state.PendingOptions)
	if err != nil {
		return fmt.Errorf("encode phone context pending options: %w", err)
	}

	_, err = r.database.ExecContext(
		ctx,
		`INSERT INTO phone_context_states (
			phone_number,
			active_farm_id,
			pending_options,
			updated_at
		) VALUES ($1,$2,$3,$4)
		ON CONFLICT (phone_number) DO UPDATE
		SET active_farm_id = EXCLUDED.active_farm_id,
			pending_options = EXCLUDED.pending_options,
			updated_at = EXCLUDED.updated_at`,
		agro.NormalizePhoneNumber(state.PhoneNumber),
		nullIfEmpty(state.ActiveFarmID),
		pendingOptionsRaw,
		state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert phone context state: %w", err)
	}

	return nil
}

func (r *ConversationRepository) GetOrCreateOpen(ctx context.Context, farmID, channel, senderPhoneNumber string, lastMessageAt time.Time) (agro.Conversation, error) {
	var conversation agro.Conversation
	var pendingConfirmationEventID sql.NullString
	var pendingCorrectionEventID sql.NullString
	row := r.database.QueryRowContext(
		ctx,
		`SELECT id, farm_id, channel, sender_phone_number, pending_confirmation_event_id, pending_correction_event_id, status, last_message_at, created_at, updated_at
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
		&pendingConfirmationEventID,
		&pendingCorrectionEventID,
		&conversation.Status,
		&conversation.LastMessageAt,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err == nil {
		if pendingConfirmationEventID.Valid {
			conversation.PendingConfirmationEventID = pendingConfirmationEventID.String
		}
		if pendingCorrectionEventID.Valid {
			conversation.PendingCorrectionEventID = pendingCorrectionEventID.String
		}
		if !lastMessageAt.IsZero() {
			if _, updateErr := r.database.ExecContext(
				ctx,
				`UPDATE conversations
				SET last_message_at = $2, updated_at = $2
				WHERE id = $1`,
				conversation.ID,
				lastMessageAt,
			); updateErr != nil {
				return agro.Conversation{}, fmt.Errorf("update conversation timestamp: %w", updateErr)
			}
			conversation.LastMessageAt = lastMessageAt
			conversation.UpdatedAt = lastMessageAt
		}
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
			pending_confirmation_event_id,
			pending_correction_event_id,
			status,
			last_message_at,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		conversation.ID,
		conversation.FarmID,
		conversation.Channel,
		conversation.SenderPhoneNumber,
		nil,
		nil,
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

func (r *ConversationRepository) SetPendingConfirmationEvent(ctx context.Context, conversationID, eventID string) error {
	_, err := r.database.ExecContext(
		ctx,
		`UPDATE conversations
		SET pending_confirmation_event_id = $2,
			updated_at = NOW()
		WHERE id = $1`,
		conversationID,
		nullIfEmpty(eventID),
	)
	if err != nil {
		return fmt.Errorf("update conversation pending confirmation event: %w", err)
	}

	return nil
}

func (r *ConversationRepository) SetPendingCorrectionEvent(ctx context.Context, conversationID, eventID string) error {
	_, err := r.database.ExecContext(
		ctx,
		`UPDATE conversations
		SET pending_correction_event_id = $2,
			updated_at = NOW()
		WHERE id = $1`,
		conversationID,
		nullIfEmpty(eventID),
	)
	if err != nil {
		return fmt.Errorf("update conversation pending correction event: %w", err)
	}

	return nil
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

func (r *TranscriptionRepository) Create(ctx context.Context, transcription *agro.Transcription) error {
	if transcription == nil {
		return fmt.Errorf("create transcription: nil transcription")
	}
	if transcription.CreatedAt.IsZero() {
		transcription.CreatedAt = time.Now().UTC()
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO transcriptions (
			id,
			source_message_id,
			provider,
			provider_ref,
			transcript_text,
			language,
			duration_seconds,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		transcription.ID,
		transcription.SourceMessageID,
		transcription.Provider,
		nullIfEmpty(transcription.ProviderRef),
		transcription.TranscriptText,
		nullIfEmpty(transcription.Language),
		transcription.DurationSeconds,
		transcription.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transcription: %w", err)
	}

	return nil
}

func (r *InterpretationRunRepository) Create(ctx context.Context, run *agro.InterpretationRun) error {
	if run == nil {
		return fmt.Errorf("create interpretation run: nil run")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO interpretation_runs (
			id,
			source_message_id,
			transcription_id,
			model_provider,
			model_name,
			prompt_version,
			normalized_intent,
			confidence,
			requires_confirmation,
			raw_output_json,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		run.ID,
		run.SourceMessageID,
		nullIfEmpty(run.TranscriptionID),
		run.ModelProvider,
		run.ModelName,
		run.PromptVersion,
		run.NormalizedIntent,
		run.Confidence,
		run.RequiresConfirmation,
		run.RawOutputJSON,
		run.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert interpretation run: %w", err)
	}

	return nil
}

func (r *BusinessEventRepository) Create(ctx context.Context, event *agro.BusinessEvent) error {
	if event == nil {
		return fmt.Errorf("create business event: nil event")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = event.CreatedAt
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO business_events (
			id,
			farm_id,
			source_message_id,
			interpretation_run_id,
			category,
			subcategory,
			occurred_at,
			description,
			amount,
			currency,
			quantity,
			unit,
			animal_code,
			lot_code,
			paddock_code,
			counterparty_name,
			status,
			confirmed_by_user,
			confirmed_at,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		event.ID,
		event.FarmID,
		event.SourceMessageID,
		event.InterpretationRunID,
		event.Category,
		event.Subcategory,
		nullTime(event.OccurredAt),
		event.Description,
		nullFloat64(event.Amount),
		nullIfEmpty(event.Currency),
		nullFloat64(event.Quantity),
		nullIfEmpty(event.Unit),
		nullIfEmpty(event.AnimalCode),
		nullIfEmpty(event.LotCode),
		nullIfEmpty(event.PaddockCode),
		nullIfEmpty(event.CounterpartyName),
		string(event.Status),
		event.ConfirmedByUser,
		nullTime(event.ConfirmedAt),
		event.CreatedAt,
		event.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert business event: %w", err)
	}

	return nil
}

func (r *BusinessEventRepository) FindByID(ctx context.Context, eventID string) (agro.BusinessEvent, bool, error) {
	var event agro.BusinessEvent
	var occurredAt sql.NullTime
	var amount sql.NullFloat64
	var quantity sql.NullFloat64
	var confirmedAt sql.NullTime

	err := r.database.QueryRowContext(
		ctx,
		`SELECT
			id,
			farm_id,
			source_message_id,
			interpretation_run_id,
			category,
			subcategory,
			occurred_at,
			description,
			amount,
			currency,
			quantity,
			unit,
			animal_code,
			lot_code,
			paddock_code,
			counterparty_name,
			status,
			confirmed_by_user,
			confirmed_at,
			created_at,
			updated_at
		FROM business_events
		WHERE id = $1`,
		eventID,
	).Scan(
		&event.ID,
		&event.FarmID,
		&event.SourceMessageID,
		&event.InterpretationRunID,
		&event.Category,
		&event.Subcategory,
		&occurredAt,
		&event.Description,
		&amount,
		&event.Currency,
		&quantity,
		&event.Unit,
		&event.AnimalCode,
		&event.LotCode,
		&event.PaddockCode,
		&event.CounterpartyName,
		&event.Status,
		&event.ConfirmedByUser,
		&confirmedAt,
		&event.CreatedAt,
		&event.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return agro.BusinessEvent{}, false, nil
	}
	if err != nil {
		return agro.BusinessEvent{}, false, fmt.Errorf("query business event by id: %w", err)
	}
	if occurredAt.Valid {
		timestamp := occurredAt.Time
		event.OccurredAt = &timestamp
	}
	if amount.Valid {
		value := amount.Float64
		event.Amount = &value
	}
	if quantity.Valid {
		value := quantity.Float64
		event.Quantity = &value
	}
	if confirmedAt.Valid {
		timestamp := confirmedAt.Time
		event.ConfirmedAt = &timestamp
	}

	return event, true, nil
}

func (r *BusinessEventRepository) CreateCorrectionLink(ctx context.Context, eventID, correctedEventID string) error {
	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO event_attributes (
			id,
			business_event_id,
			attr_key,
			attr_value,
			created_at
		) VALUES ($1,$2,$3,$4,$5)`,
		uuid.NewString(),
		eventID,
		"corrects_event_id",
		correctedEventID,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert correction link attribute: %w", err)
	}

	return nil
}

func (r *BusinessEventRepository) UpdateStatus(ctx context.Context, eventID string, status agro.EventStatus, confirmedByUser bool, confirmedAt *time.Time) error {
	_, err := r.database.ExecContext(
		ctx,
		`UPDATE business_events
		SET status = $2,
			confirmed_by_user = $3,
			confirmed_at = $4,
			updated_at = NOW()
		WHERE id = $1`,
		eventID,
		string(status),
		confirmedByUser,
		nullTime(confirmedAt),
	)
	if err != nil {
		return fmt.Errorf("update business event status: %w", err)
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

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return *value
}

func nullFloat64(value *float64) any {
	if value == nil {
		return nil
	}

	return *value
}
