# POC Domain Model and Postgres Strategy

## Objective

This document defines the initial business model for the WhatsApp agro POC with these constraints:

- Postgres is the single source of truth.
- `whatsmeow` is the temporary channel adapter for POC validation.
- the core domain must remain independent from `whatsmeow`.
- the future MVP must be able to replace `whatsmeow` with the official Meta API without changing business rules.

The product goal is not "chatbot reply quality".
The goal is to capture operational and financial facts through WhatsApp and persist them as useful business context for the producer.

## Architectural Direction

### Short-term POC

- inbound and outbound channel via `whatsmeow`
- persistence only in Postgres
- support text and audio first
- audio goes through a transcription service
- AI converts message content into a structured interpretation
- the system asks for confirmation before finalizing business facts when confidence is not high enough

### Future MVP

- replace only the channel adapter:
  - `whatsmeow` -> Meta official WhatsApp API
- preserve:
  - domain model
  - use cases
  - Postgres schema
  - interpretation pipeline

## Core Business Principles

1. Every inbound message must be traceable.
2. Every transcription must be tied to the original message.
3. Every AI interpretation must be persisted as a separate artifact.
4. Every structured business fact must point back to the message and interpretation that originated it.
5. A user phone number is not the business entity itself.
   It is an identity key used to resolve the producer and farm context.
6. Channel adapters must never contain business rules about agro categories.

## Initial Domain Scope

The first POC should support only a small set of business categories.

- `finance.expense`
- `finance.revenue`
- `reproduction.insemination`
- `operations.note`

Optional near-next categories:

- `feeding.feed_expense`
- `health.vet_expense`
- `sanitary.calendar`
- `operations.task`

This is intentionally narrow.
Trying to cover the full agro domain in the first model will slow the POC and lower interpretation quality.

## Identity and Context Resolution

The POC uses one central WhatsApp number.
The sender phone number determines which producer or farm context applies.

### Rules

1. If the sender phone is linked to exactly one active farm, the message enters that context directly.
2. If the sender phone is linked to multiple farms, the system must ask the user to choose the target farm.
3. If the sender phone is unknown, the system must enter onboarding or reject processing.
4. A phone number can belong to:
   - producer owner
   - manager
   - employee
   - accountant

### Result

Conversation context is resolved as:

`sender_phone_number -> farm_membership -> producer + farm + role`

## Canonical Flow

1. Inbound message arrives from the channel adapter.
2. The adapter converts provider-specific payload into a provider-agnostic `InboundMessage`.
3. The application resolves sender context by phone number.
4. The raw message is persisted as `source_messages`.
5. If the message contains audio:
   - it is transcribed
   - the transcript is persisted in `transcriptions`
6. The normalized content is sent to the interpretation service.
7. The interpretation result is persisted in `interpretation_runs`.
8. The application decides:
   - create a draft `business_events`
   - ask for confirmation
   - or store only as operational note
9. The reply is persisted as `assistant_messages`.

## Postgres Schema

## Table: `producers`

Represents the business owner or customer account.

```sql
create table if not exists producers (
    id uuid primary key,
    name text not null,
    status text not null default 'active',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
```

## Table: `farms`

Represents the operational unit where events happen.

```sql
create table if not exists farms (
    id uuid primary key,
    producer_id uuid not null references producers(id),
    name text not null,
    activity_type text not null,
    timezone text not null default 'America/Sao_Paulo',
    status text not null default 'active',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_farms_producer_id on farms(producer_id);
```

Recommended `activity_type` values:

- `milk`
- `beef`
- `mixed`

## Table: `farm_memberships`

Maps sender phone numbers to business context.

```sql
create table if not exists farm_memberships (
    id uuid primary key,
    farm_id uuid not null references farms(id),
    person_name text,
    phone_number text not null,
    role text not null,
    is_primary boolean not null default false,
    status text not null default 'active',
    verified_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists idx_farm_memberships_farm_phone
    on farm_memberships(farm_id, phone_number);

create index if not exists idx_farm_memberships_phone_number
    on farm_memberships(phone_number);
```

Recommended `role` values:

- `owner`
- `manager`
- `worker`
- `veterinarian`
- `accountant`

## Table: `conversations`

Represents the hot logical thread for a phone number inside a farm context.

```sql
create table if not exists conversations (
    id uuid primary key,
    farm_id uuid not null references farms(id),
    channel text not null,
    sender_phone_number text not null,
    status text not null default 'open',
    last_message_at timestamptz not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_conversations_farm_phone
    on conversations(farm_id, sender_phone_number);
```

## Table: `source_messages`

Stores the original inbound message before interpretation.

```sql
create table if not exists source_messages (
    id uuid primary key,
    conversation_id uuid not null references conversations(id),
    provider text not null,
    provider_message_id text,
    sender_phone_number text not null,
    message_type text not null,
    raw_text text,
    media_url text,
    media_content_type text,
    media_filename text,
    received_at timestamptz not null,
    created_at timestamptz not null default now()
);

create unique index if not exists idx_source_messages_provider_message_id
    on source_messages(provider, provider_message_id)
    where provider_message_id is not null;

create index if not exists idx_source_messages_conversation_id
    on source_messages(conversation_id, received_at desc);
```

Recommended `message_type` values:

- `text`
- `audio`
- `image`
- `document`
- `interactive`
- `unsupported`

## Table: `transcriptions`

Stores transcript output for audio inputs.

```sql
create table if not exists transcriptions (
    id uuid primary key,
    source_message_id uuid not null unique references source_messages(id),
    provider text not null,
    provider_ref text,
    transcript_text text not null,
    language text,
    duration_seconds double precision,
    created_at timestamptz not null default now()
);
```

`provider_ref` can store the previous external transcription id if needed.

## Table: `interpretation_runs`

Stores the raw machine interpretation and normalized decision fields.

```sql
create table if not exists interpretation_runs (
    id uuid primary key,
    source_message_id uuid not null references source_messages(id),
    transcription_id uuid references transcriptions(id),
    model_provider text not null,
    model_name text not null,
    prompt_version text not null,
    normalized_intent text not null,
    confidence numeric(5,4),
    requires_confirmation boolean not null default true,
    raw_output_json jsonb not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_interpretation_runs_source_message_id
    on interpretation_runs(source_message_id);
```

Example `normalized_intent` values:

- `finance.expense`
- `finance.revenue`
- `reproduction.insemination`
- `operations.note`

## Table: `business_events`

Stores the structured business fact extracted from a message.

```sql
create table if not exists business_events (
    id uuid primary key,
    farm_id uuid not null references farms(id),
    source_message_id uuid not null references source_messages(id),
    interpretation_run_id uuid not null references interpretation_runs(id),
    category text not null,
    subcategory text not null,
    occurred_at timestamptz,
    description text not null,
    amount numeric(14,2),
    currency text,
    quantity numeric(14,3),
    unit text,
    animal_code text,
    lot_code text,
    paddock_code text,
    counterparty_name text,
    status text not null default 'draft',
    confirmed_by_user boolean not null default false,
    confirmed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_business_events_farm_id_occurred_at
    on business_events(farm_id, occurred_at desc);

create index if not exists idx_business_events_category
    on business_events(farm_id, category, subcategory);
```

Recommended `status` values:

- `draft`
- `confirmed`
- `rejected`
- `corrected`

## Table: `event_attributes`

Flexible extension table for category-specific fields.

```sql
create table if not exists event_attributes (
    id uuid primary key,
    business_event_id uuid not null references business_events(id) on delete cascade,
    attr_key text not null,
    attr_value text not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_event_attributes_event_id
    on event_attributes(business_event_id);
```

Examples:

- `inseminator_name`
- `vaccine_name`
- `payment_method`
- `invoice_number`

## Table: `assistant_messages`

Stores outbound replies sent to the user.

```sql
create table if not exists assistant_messages (
    id uuid primary key,
    conversation_id uuid not null references conversations(id),
    source_message_id uuid references source_messages(id),
    provider text not null,
    provider_message_id text,
    reply_type text not null,
    body text not null,
    created_at timestamptz not null default now()
);

create index if not exists idx_assistant_messages_conversation_id
    on assistant_messages(conversation_id, created_at desc);
```

## Interpretation Contract

The AI layer should not return free-form prose as the main output.
It should return a strict JSON contract.

Example:

```json
{
  "intent": "finance.expense",
  "confidence": 0.94,
  "requires_confirmation": true,
  "event": {
    "category": "finance",
    "subcategory": "feeding.feed_expense",
    "occurred_at": "2026-04-06",
    "description": "Compra de 10 sacos de racao",
    "amount": 850.00,
    "currency": "BRL",
    "quantity": 10,
    "unit": "saco",
    "animal_code": null,
    "lot_code": null,
    "paddock_code": null,
    "counterparty_name": null
  }
}
```

The system may still generate a human reply after this.
But the structured JSON must be the canonical AI output for persistence.

## Confirmation Policy

The POC should not blindly finalize all events.

### Confirm directly when all are true

- confidence is high
- category is low risk
- required fields are present

### Ask confirmation when any is true

- confidence is below threshold
- amount was extracted from audio
- date is inferred rather than explicit
- category is financially relevant
- the same phone number has multiple farms

### Example outbound confirmation

```text
Registrei uma despesa de R$ 850,00 com racao, 10 sacos. Confirmar?
```

## Package Boundary Proposal

This package split is designed to let the POC use `whatsmeow` now and Meta later.

### `internal/domain`

Pure business types and rules.

Suggested packages:

- `internal/domain/identity`
- `internal/domain/conversation`
- `internal/domain/message`
- `internal/domain/transcription`
- `internal/domain/interpretation`
- `internal/domain/event`

### `internal/usecase`

Application orchestration.

Suggested packages:

- `internal/usecase/inbound`
- `internal/usecase/contextresolver`
- `internal/usecase/transcribe`
- `internal/usecase/interpret`
- `internal/usecase/confirm`
- `internal/usecase/reply`

### `internal/adapters/channel/whatsmeow`

POC-only provider adapter.

Responsibilities:

- connect session
- receive inbound provider events
- map provider payload -> `InboundMessage`
- send outbound text replies

Must not:

- classify agro domain
- resolve farm context
- persist business events

### `internal/adapters/channel/meta`

Future adapter with the same outward contract as `whatsmeow`.

### `internal/adapters/storage/postgres`

Postgres repositories for all domain aggregates.

Suggested repositories:

- `ProducerRepository`
- `FarmMembershipRepository`
- `ConversationRepository`
- `SourceMessageRepository`
- `TranscriptionRepository`
- `InterpretationRunRepository`
- `BusinessEventRepository`
- `AssistantMessageRepository`

### `internal/adapters/ai`

Model-specific clients.

Suggested adapters:

- `internal/adapters/ai/gemini`
- `internal/adapters/ai/disabled`

### `internal/adapters/transcription`

Audio transcription providers.

Short term:

- keep the existing transcription service integration if it accelerates the POC

Medium term:

- either move transcription inside this repo
- or keep a provider interface and call an external transcription service

## Provider-Agnostic Contracts

The key migration rule is:

`whatsmeow` and Meta must implement the same application-facing contracts.

Example interfaces:

```go
type InboundMessage struct {
    Provider          string
    ProviderMessageID string
    SenderPhoneNumber string
    MessageType       string
    Text              string
    MediaURL          string
    MediaContentType  string
    MediaFilename     string
    ReceivedAt        time.Time
}

type ChannelInboundConsumer interface {
    HandleInbound(ctx context.Context, msg InboundMessage) error
}

type ChannelSender interface {
    SendText(ctx context.Context, phoneNumber string, body string) (providerMessageID string, err error)
}
```

The core use case must depend on these contracts only.

## POC Implementation Sequence

### Phase 1

- add Postgres schema
- add phone number -> farm resolution
- persist `source_messages`
- persist `assistant_messages`
- keep plain text response flow

### Phase 2

- persist audio transcriptions in Postgres
- add `interpretation_runs`
- convert free-text AI reply into strict JSON extraction + user reply

### Phase 3

- create `business_events`
- add confirmation flow
- support the initial 4 categories only

### Phase 4

- add image/document ingestion
- keep the same persistence chain:
  - `source_messages`
  - `interpretation_runs`
  - `business_events`

## Things To Avoid

- do not store only the final event and discard the original message
- do not keep the business model inside WhatsApp-specific structs
- do not let the provider determine business category names
- do not optimize for all agro subdomains before validating the first 4 categories
- do not make Redis a required source of truth for the POC

## Final Recommendation

For the POC:

- use `whatsmeow`
- use Postgres only
- isolate provider adapters
- validate text and audio first
- confirm extracted facts before marking them as final

For the MVP:

- replace only the channel adapter with Meta official API
- keep the same Postgres schema and domain flow
