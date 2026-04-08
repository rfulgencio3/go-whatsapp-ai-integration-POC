package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

type FarmMembershipRepository struct {
	database *sql.DB
}

type FarmRegistrationRepository struct {
	database *sql.DB
}

type ConversationRepository struct {
	database *sql.DB
}

type PhoneContextStateRepository struct {
	database *sql.DB
}

type OnboardingStateRepository struct {
	database *sql.DB
}

type HealthTreatmentStateRepository struct {
	database *sql.DB
}

type CorrelatedExpenseStateRepository struct {
	database *sql.DB
}

type OnboardingMessageRepository struct {
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

func NewFarmRegistrationRepository(database *sql.DB) *FarmRegistrationRepository {
	return &FarmRegistrationRepository{database: database}
}

func NewConversationRepository(database *sql.DB) *ConversationRepository {
	return &ConversationRepository{database: database}
}

func NewPhoneContextStateRepository(database *sql.DB) *PhoneContextStateRepository {
	return &PhoneContextStateRepository{database: database}
}

func NewOnboardingStateRepository(database *sql.DB) *OnboardingStateRepository {
	return &OnboardingStateRepository{database: database}
}

func NewHealthTreatmentStateRepository(database *sql.DB) *HealthTreatmentStateRepository {
	return &HealthTreatmentStateRepository{database: database}
}

func NewCorrelatedExpenseStateRepository(database *sql.DB) *CorrelatedExpenseStateRepository {
	return &CorrelatedExpenseStateRepository{database: database}
}

func NewOnboardingMessageRepository(database *sql.DB) *OnboardingMessageRepository {
	return &OnboardingMessageRepository{database: database}
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
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)
	rows, err := r.database.QueryContext(
		ctx,
		`SELECT fm.id, fm.farm_id, f.name, fm.person_name, fm.phone_number, fm.role, fm.is_primary, fm.status, fm.verified_at, fm.created_at, fm.updated_at
		FROM farm_memberships fm
		INNER JOIN farms f ON f.id = fm.farm_id
		WHERE fm.phone_number IN ($1, $2) AND fm.status = 'active'
		ORDER BY fm.is_primary DESC, fm.created_at ASC`,
		primaryPhone,
		secondaryPhone,
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

func (r *FarmRegistrationRepository) CreateInitialRegistration(ctx context.Context, phoneNumber, producerName, farmName string) (agro.FarmMembership, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return agro.FarmMembership{}, fmt.Errorf("begin registration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now().UTC()
	producerID := uuid.NewString()
	farmID := uuid.NewString()
	membershipID := uuid.NewString()
	normalizedPhone := agro.NormalizePhoneNumber(phoneNumber)

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO producers (id, name, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5)`,
		producerID,
		strings.TrimSpace(producerName),
		"active",
		now,
		now,
	); err != nil {
		return agro.FarmMembership{}, fmt.Errorf("insert producer: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO farms (id, producer_id, name, activity_type, timezone, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		farmID,
		producerID,
		strings.TrimSpace(farmName),
		string(agro.ActivityMixed),
		"America/Sao_Paulo",
		"active",
		now,
		now,
	); err != nil {
		return agro.FarmMembership{}, fmt.Errorf("insert farm: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO farm_memberships (id, farm_id, person_name, phone_number, role, is_primary, status, verified_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		membershipID,
		farmID,
		strings.TrimSpace(producerName),
		normalizedPhone,
		string(agro.RoleOwner),
		true,
		"active",
		now,
		now,
		now,
	); err != nil {
		return agro.FarmMembership{}, fmt.Errorf("insert farm membership: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO phone_context_states (phone_number, active_farm_id, pending_options, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (phone_number) DO UPDATE
		SET active_farm_id = EXCLUDED.active_farm_id,
			pending_options = EXCLUDED.pending_options,
			updated_at = EXCLUDED.updated_at`,
		normalizedPhone,
		farmID,
		[]byte("[]"),
		now,
	); err != nil {
		return agro.FarmMembership{}, fmt.Errorf("upsert phone context after registration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return agro.FarmMembership{}, fmt.Errorf("commit registration transaction: %w", err)
	}

	verifiedAt := now
	return agro.FarmMembership{
		ID:          membershipID,
		FarmID:      farmID,
		FarmName:    strings.TrimSpace(farmName),
		PersonName:  strings.TrimSpace(producerName),
		PhoneNumber: normalizedPhone,
		Role:        agro.RoleOwner,
		IsPrimary:   true,
		Status:      "active",
		VerifiedAt:  &verifiedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (r *PhoneContextStateRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (agro.PhoneContextState, bool, error) {
	var state agro.PhoneContextState
	var activeFarmID sql.NullString
	var pendingOptionsRaw []byte
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)

	err := r.database.QueryRowContext(
		ctx,
		`SELECT phone_number, active_farm_id, pending_options, updated_at
		FROM phone_context_states
		WHERE phone_number IN ($1, $2)
		ORDER BY CASE WHEN phone_number = $1 THEN 0 ELSE 1 END
		LIMIT 1`,
		primaryPhone,
		secondaryPhone,
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

func (r *OnboardingStateRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (agro.OnboardingState, bool, error) {
	var state agro.OnboardingState
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)
	err := r.database.QueryRowContext(
		ctx,
		`SELECT phone_number, step, producer_name, updated_at
		FROM onboarding_states
		WHERE phone_number IN ($1, $2)
		ORDER BY CASE WHEN phone_number = $1 THEN 0 ELSE 1 END
		LIMIT 1`,
		primaryPhone,
		secondaryPhone,
	).Scan(
		&state.PhoneNumber,
		&state.Step,
		&state.ProducerName,
		&state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return agro.OnboardingState{}, false, nil
	}
	if err != nil {
		return agro.OnboardingState{}, false, fmt.Errorf("query onboarding state: %w", err)
	}

	return state, true, nil
}

func (r *OnboardingStateRepository) Upsert(ctx context.Context, state *agro.OnboardingState) error {
	if state == nil {
		return fmt.Errorf("upsert onboarding state: nil state")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO onboarding_states (phone_number, step, producer_name, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (phone_number) DO UPDATE
		SET step = EXCLUDED.step,
			producer_name = EXCLUDED.producer_name,
			updated_at = EXCLUDED.updated_at`,
		agro.NormalizePhoneNumber(state.PhoneNumber),
		string(state.Step),
		nullIfEmpty(strings.TrimSpace(state.ProducerName)),
		state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert onboarding state: %w", err)
	}

	return nil
}

func (r *OnboardingStateRepository) DeleteByPhoneNumber(ctx context.Context, phoneNumber string) error {
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)
	_, err := r.database.ExecContext(
		ctx,
		`DELETE FROM onboarding_states WHERE phone_number IN ($1, $2)`,
		primaryPhone,
		secondaryPhone,
	)
	if err != nil {
		return fmt.Errorf("delete onboarding state: %w", err)
	}

	return nil
}

func (r *HealthTreatmentStateRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (agro.HealthTreatmentState, bool, error) {
	var state agro.HealthTreatmentState
	var attributesRaw []byte
	var animalCode sql.NullString
	var diagnosisDateText sql.NullString
	var diagnosisOccurredAt sql.NullTime
	var medicine sql.NullString
	var treatmentDays sql.NullInt64
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)

	err := r.database.QueryRowContext(
		ctx,
		`SELECT
			phone_number,
			farm_id,
			category,
			subcategory,
			animal_code,
			description,
			attributes,
			diagnosis_date_text,
			diagnosis_occurred_at,
			medicine,
			treatment_days,
			step,
			updated_at
		FROM health_treatment_states
		WHERE phone_number IN ($1, $2)
		ORDER BY CASE WHEN phone_number = $1 THEN 0 ELSE 1 END
		LIMIT 1`,
		primaryPhone,
		secondaryPhone,
	).Scan(
		&state.PhoneNumber,
		&state.FarmID,
		&state.Category,
		&state.Subcategory,
		&animalCode,
		&state.Description,
		&attributesRaw,
		&diagnosisDateText,
		&diagnosisOccurredAt,
		&medicine,
		&treatmentDays,
		&state.Step,
		&state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return agro.HealthTreatmentState{}, false, nil
	}
	if err != nil {
		return agro.HealthTreatmentState{}, false, fmt.Errorf("query health treatment state: %w", err)
	}
	if animalCode.Valid {
		state.AnimalCode = animalCode.String
	}
	if len(attributesRaw) > 0 {
		if err := json.Unmarshal(attributesRaw, &state.Attributes); err != nil {
			return agro.HealthTreatmentState{}, false, fmt.Errorf("decode health treatment attributes: %w", err)
		}
	}
	if diagnosisDateText.Valid {
		state.DiagnosisDateText = diagnosisDateText.String
	}
	if diagnosisOccurredAt.Valid {
		timestamp := diagnosisOccurredAt.Time
		state.DiagnosisOccurredAt = &timestamp
	}
	if medicine.Valid {
		state.Medicine = medicine.String
	}
	if treatmentDays.Valid {
		state.TreatmentDays = int(treatmentDays.Int64)
	}

	return state, true, nil
}

func (r *HealthTreatmentStateRepository) Upsert(ctx context.Context, state *agro.HealthTreatmentState) error {
	if state == nil {
		return fmt.Errorf("upsert health treatment state: nil state")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}

	attributesRaw, err := json.Marshal(state.Attributes)
	if err != nil {
		return fmt.Errorf("encode health treatment attributes: %w", err)
	}

	_, err = r.database.ExecContext(
		ctx,
		`INSERT INTO health_treatment_states (
			phone_number,
			farm_id,
			category,
			subcategory,
			animal_code,
			description,
			attributes,
			diagnosis_date_text,
			diagnosis_occurred_at,
			medicine,
			treatment_days,
			step,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (phone_number) DO UPDATE
		SET farm_id = EXCLUDED.farm_id,
			category = EXCLUDED.category,
			subcategory = EXCLUDED.subcategory,
			animal_code = EXCLUDED.animal_code,
			description = EXCLUDED.description,
			attributes = EXCLUDED.attributes,
			diagnosis_date_text = EXCLUDED.diagnosis_date_text,
			diagnosis_occurred_at = EXCLUDED.diagnosis_occurred_at,
			medicine = EXCLUDED.medicine,
			treatment_days = EXCLUDED.treatment_days,
			step = EXCLUDED.step,
			updated_at = EXCLUDED.updated_at`,
		agro.NormalizePhoneNumber(state.PhoneNumber),
		state.FarmID,
		state.Category,
		state.Subcategory,
		nullIfEmpty(strings.TrimSpace(state.AnimalCode)),
		state.Description,
		attributesRaw,
		nullIfEmpty(strings.TrimSpace(state.DiagnosisDateText)),
		nullTime(state.DiagnosisOccurredAt),
		nullIfEmpty(strings.TrimSpace(state.Medicine)),
		nullInt(state.TreatmentDays),
		string(state.Step),
		state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert health treatment state: %w", err)
	}

	return nil
}

func (r *HealthTreatmentStateRepository) DeleteByPhoneNumber(ctx context.Context, phoneNumber string) error {
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)
	_, err := r.database.ExecContext(
		ctx,
		`DELETE FROM health_treatment_states WHERE phone_number IN ($1, $2)`,
		primaryPhone,
		secondaryPhone,
	)
	if err != nil {
		return fmt.Errorf("delete health treatment state: %w", err)
	}

	return nil
}

func (r *CorrelatedExpenseStateRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (agro.CorrelatedExpenseState, bool, error) {
	var state agro.CorrelatedExpenseState
	var animalCode sql.NullString
	var occurredAt sql.NullTime
	var medicineAmount sql.NullFloat64
	var vetAmount sql.NullFloat64
	var examAmount sql.NullFloat64
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)

	err := r.database.QueryRowContext(
		ctx,
		`SELECT
			phone_number,
			farm_id,
			root_event_id,
			root_category,
			root_subcategory,
			animal_code,
			description,
			occurred_at,
			medicine_amount,
			vet_amount,
			exam_amount,
			step,
			updated_at
		FROM correlated_expense_states
		WHERE phone_number IN ($1, $2)
		ORDER BY CASE WHEN phone_number = $1 THEN 0 ELSE 1 END
		LIMIT 1`,
		primaryPhone,
		secondaryPhone,
	).Scan(
		&state.PhoneNumber,
		&state.FarmID,
		&state.RootEventID,
		&state.RootCategory,
		&state.RootSubcategory,
		&animalCode,
		&state.Description,
		&occurredAt,
		&medicineAmount,
		&vetAmount,
		&examAmount,
		&state.Step,
		&state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return agro.CorrelatedExpenseState{}, false, nil
	}
	if err != nil {
		return agro.CorrelatedExpenseState{}, false, fmt.Errorf("query correlated expense state: %w", err)
	}
	if animalCode.Valid {
		state.AnimalCode = animalCode.String
	}
	if occurredAt.Valid {
		timestamp := occurredAt.Time
		state.OccurredAt = &timestamp
	}
	if medicineAmount.Valid {
		value := medicineAmount.Float64
		state.MedicineAmount = &value
	}
	if vetAmount.Valid {
		value := vetAmount.Float64
		state.VetAmount = &value
	}
	if examAmount.Valid {
		value := examAmount.Float64
		state.ExamAmount = &value
	}

	return state, true, nil
}

func (r *CorrelatedExpenseStateRepository) Upsert(ctx context.Context, state *agro.CorrelatedExpenseState) error {
	if state == nil {
		return fmt.Errorf("upsert correlated expense state: nil state")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO correlated_expense_states (
			phone_number,
			farm_id,
			root_event_id,
			root_category,
			root_subcategory,
			animal_code,
			description,
			occurred_at,
			medicine_amount,
			vet_amount,
			exam_amount,
			step,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (phone_number) DO UPDATE
		SET farm_id = EXCLUDED.farm_id,
			root_event_id = EXCLUDED.root_event_id,
			root_category = EXCLUDED.root_category,
			root_subcategory = EXCLUDED.root_subcategory,
			animal_code = EXCLUDED.animal_code,
			description = EXCLUDED.description,
			occurred_at = EXCLUDED.occurred_at,
			medicine_amount = EXCLUDED.medicine_amount,
			vet_amount = EXCLUDED.vet_amount,
			exam_amount = EXCLUDED.exam_amount,
			step = EXCLUDED.step,
			updated_at = EXCLUDED.updated_at`,
		agro.NormalizePhoneNumber(state.PhoneNumber),
		state.FarmID,
		state.RootEventID,
		state.RootCategory,
		state.RootSubcategory,
		nullIfEmpty(strings.TrimSpace(state.AnimalCode)),
		state.Description,
		nullTime(state.OccurredAt),
		nullFloat64(state.MedicineAmount),
		nullFloat64(state.VetAmount),
		nullFloat64(state.ExamAmount),
		string(state.Step),
		state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert correlated expense state: %w", err)
	}

	return nil
}

func (r *CorrelatedExpenseStateRepository) DeleteByPhoneNumber(ctx context.Context, phoneNumber string) error {
	primaryPhone, secondaryPhone := phoneLookupVariants(phoneNumber)
	_, err := r.database.ExecContext(
		ctx,
		`DELETE FROM correlated_expense_states WHERE phone_number IN ($1, $2)`,
		primaryPhone,
		secondaryPhone,
	)
	if err != nil {
		return fmt.Errorf("delete correlated expense state: %w", err)
	}

	return nil
}

func (r *OnboardingMessageRepository) Create(ctx context.Context, message *agro.OnboardingMessage) error {
	if message == nil {
		return fmt.Errorf("create onboarding message: nil message")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}

	_, err := r.database.ExecContext(
		ctx,
		`INSERT INTO onboarding_messages (
			id,
			phone_number,
			step,
			direction,
			provider,
			provider_message_id,
			message_type,
			body,
			created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		message.ID,
		agro.NormalizePhoneNumber(message.PhoneNumber),
		nullIfEmpty(string(message.Step)),
		string(message.Direction),
		message.Provider,
		nullIfEmpty(message.ProviderMessageID),
		string(message.MessageType),
		message.Body,
		message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert onboarding message: %w", err)
	}

	return nil
}

func (r *ConversationRepository) GetOrCreateOpen(ctx context.Context, farmID, channel, senderPhoneNumber string, lastMessageAt time.Time) (agro.Conversation, error) {
	var conversation agro.Conversation
	var pendingConfirmationEventID sql.NullString
	var pendingCorrectionEventID sql.NullString
	primaryPhone, secondaryPhone := phoneLookupVariants(senderPhoneNumber)
	row := r.database.QueryRowContext(
		ctx,
		`SELECT id, farm_id, channel, sender_phone_number, pending_confirmation_event_id, pending_correction_event_id, status, last_message_at, created_at, updated_at
		FROM conversations
		WHERE farm_id = $1 AND channel = $2 AND sender_phone_number IN ($3, $4) AND status = 'open'
		ORDER BY CASE WHEN sender_phone_number = $3 THEN 0 ELSE 1 END, updated_at DESC
		LIMIT 1`,
		farmID,
		channel,
		primaryPhone,
		secondaryPhone,
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

func (r *BusinessEventRepository) CreateAttributes(ctx context.Context, eventID string, attributes map[string]string) error {
	if len(attributes) == 0 {
		return nil
	}

	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := strings.TrimSpace(attributes[key])
		if strings.TrimSpace(key) == "" || value == "" {
			continue
		}
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
			key,
			value,
			time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("insert event attribute: %w", err)
		}
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

func nullInt(value int) any {
	if value == 0 {
		return nil
	}

	return value
}

func phoneLookupVariants(phoneNumber string) (string, string) {
	candidates := agro.PhoneNumberLookupCandidates(phoneNumber)
	switch len(candidates) {
	case 0:
		return "", ""
	case 1:
		return candidates[0], candidates[0]
	default:
		return candidates[0], candidates[1]
	}
}
