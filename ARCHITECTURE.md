# Architecture

## Runtime View

```text
                           +------------------------------+
                           |         WhatsApp User        |
                           |   text, audio, image, docs   |
                           +---------------+--------------+
                                           |
                                           v
                           +------------------------------+
                           |    WhatsApp account host     |
                           |   linked by whatsmeow Web    |
                           +---------------+--------------+
                                           |
                                           v
        +----------------------------------------------------------------------------------+
        |                   go-whatsapp-ai-integration-POC                                  |
        |----------------------------------------------------------------------------------|
        | HTTP server                                                                       |
        | - health endpoints                                                                |
        | - optional webhook endpoints                                                      |
        |                                                                                  |
        | Messaging runtime                                                                 |
        | - whatsmeow client                                                                |
        | - outbound sender                                                                 |
        | - async queue                                                                     |
        +------------------------------+-------------------------------+-------------------+
                                       |                               |
                                       v                               v
                          +--------------------------+      +---------------------------+
                          | chatbot.Service          |      | agro.CaptureService       |
                          | - history-aware replies  |      | - membership resolution   |
                          | - Gemini fallback        |      | - onboarding/context      |
                          | - outbound send          |      | - confirmations           |
                          +------------+-------------+      | - event capture           |
                                       |                    +-------------+-------------+
                                       |                                  |
                                       v                                  v
                          +--------------------------+      +---------------------------+
                          | go-audio-transcription   |      | Postgres                  |
                          | POST /transcribe         |      | source_messages           |
                          | - stateless transcript   |      | transcriptions            |
                          | - Gemini audio process   |      | interpretation_runs       |
                          +--------------------------+      | business_events           |
                                                            | event_attributes          |
                                                            | assistant_messages        |
                                                            | onboarding / context      |
                                                            +---------------------------+
```

## Current Channel Strategy

- `whatsmeow` is the active WhatsApp channel for the POC.
- The host account is a real WhatsApp number linked as a web session.
- The app can still keep optional webhook plumbing, but the primary production-like path for the POC is `whatsmeow -> queue -> chatbot/agro`.

## Main Responsibilities

### `go-whatsapp-ai-integration-POC`

- Receives inbound messages from the configured channel runtime.
- Normalizes inbound messages into `chat.IncomingMessage`.
- Runs business capture in `agro.CaptureService`.
- Delegates generic conversational handling to `chatbot.Service`.
- Persists the operational trail in Postgres.
- Sends outbound replies through the configured sender.

### `go-audio-transcription`

- Receives multipart audio in `POST /transcribe`.
- Transcribes audio with Gemini.
- Returns transcript data to the WhatsApp service.
- Does not persist transcripts locally.

## Persistence Model

### Postgres

Postgres is the durable source of truth for the POC domain and operational trace.

Main tables and concerns:

- `farm_memberships`
- `phone_context_states`
- `onboarding_states`
- `onboarding_messages`
- `source_messages`
- `transcriptions`
- `interpretation_runs`
- `business_events`
- `event_attributes`
- `assistant_messages`

This model supports:

- onboarding by WhatsApp
- membership and farm context resolution
- traceability of inbound and outbound messages
- confirmation and correction flows
- extensible domain capture through `business_events` + `event_attributes`

### Redis

Redis is optional.

When enabled, it may support:

- short-lived chatbot history
- deduplication
- transient runtime concerns

It is not the durable source of truth of the agro domain.

## Application Structure

### Composition Root

`internal/app` builds the runtime from small builders:

- storage builders
- messaging/channel builders
- chatbot builders
- agro builders

The goal is to keep `app.New` as composition only, without business rules.

### Agro Capture

`internal/usecase/agro` is organized around focused collaborators:

- `capture_service.go`
  - main inbound orchestration
- `capture_onboarding.go`
  - onboarding flow
- `capture_membership.go`
  - membership and farm context flow
- `capture_confirmation.go`
  - draft confirmation and correction flow
- `capture_persistence.go`
  - persistence helpers
- `capture_workflow.go`
  - workflow routing helpers
- `reply_formatter.go`
  - agro reply formatting

This split reduces duplication and keeps the main use case cohesive.

### Chatbot

`internal/usecase/chatbot` remains responsible for generic conversation handling:

- load recent history
- call Gemini when appropriate
- degrade gracefully on quota errors
- send reply and persist legacy conversation trace

## Main Flows

### Text

```text
WhatsApp -> whatsmeow -> queue -> agro.CaptureService / chatbot.Service -> sender -> Postgres
```

### Audio

```text
WhatsApp -> whatsmeow -> media download -> go-audio-transcription /transcribe
         -> transcript injected into inbound message
         -> agro.CaptureService / chatbot.Service
         -> sender -> Postgres
```

### Confirmation

```text
User message -> interpreter -> draft business_event -> structured confirmation reply
             -> user answers SIM/NAO
             -> event confirmed or rejected/corrected
```

### Onboarding

```text
Unknown phone -> onboarding prompts -> producer/farm creation
              -> membership creation -> active phone context
```

## Event Model

The current event model is centered on:

- `finance.input_purchase`
- `finance.expense`
- `finance.revenue`
- `reproduction.insemination`
- `operations.note`
- `health.mastitis_treatment`
- `health.hoof_treatment`
- `health.bloat`

High-variance details should prefer `event_attributes` instead of rigid schema growth.

Examples:

- affected teats
- milk withdrawal flag
- medicine
- treatment days
- related expense type

## Design Rules

- Keep channel adapters thin.
- Keep shared phone normalization in a single domain module.
- Keep response formatting outside the main use case orchestration.
- Prefer extracting focused collaborators before adding more branches to `CaptureService`.
- Prefer deterministic/rule-based handling first for common agro flows, and use Gemini as a fallback where ambiguity justifies the token cost.

See also:

- [`docs/architecture-guidelines.md`](C:/repos/go-whatsapp-ai-integration-POC/docs/architecture-guidelines.md)
