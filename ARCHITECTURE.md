# Architecture

## Runtime view

```text
                                   +------------------------------+
                                   |         WhatsApp User        |
                                   |   text, audio, image, docs   |
                                   +---------------+--------------+
                                                   |
                                                   v
                                   +------------------------------+
                                   |      Twilio WhatsApp         |
                                   | sandbox / inbound webhook    |
                                   +---------------+--------------+
                                                   |
                                                   v
        +----------------------------------------------------------------------------------+
        |                   go-whatsapp-ai-integration-POC                                  |
        |----------------------------------------------------------------------------------|
        | POST /webhook/twilio                                                             |
        | - validate X-Twilio-Signature                                                    |
        | - parse MessageSid / WaId / Body / MediaUrlN / MediaContentTypeN                |
        | - enqueue async processing                                                       |
        +------------------------------+-------------------------------+-------------------+
                                       |                               |
                                       v                               v
                          +--------------------------+      +---------------------------+
                          | In-memory worker queue   |      | Redis                     |
                          | retry + idempotency key  |      | chat history + dedup      |
                          +------------+-------------+      +---------------------------+
                                       |
                                       v
                          +--------------------------+
                          | chatbot.Service          |
                          | - Twilio preprocessor    |
                          | - Gemini reply builder   |
                          | - outbound sender        |
                          +------------+-------------+
                                       |
                    +------------------+------------------+
                    |                                     |
                    v                                     v
      +-------------------------------+      +-------------------------------+
      | go-audio-transcription        |      | Twilio Messages API          |
      | POST /transcribe              |      | send WhatsApp reply          |
      | - Gemini audio transcript     |      +-------------------------------+
      | - MongoDB persistence         |
      +---------------+---------------+
                      |
                      v
      +-------------------------------+
      | MongoDB                       |
      | full transcript record        |
      +-------------------------------+

                                       |
                                       v
                          +--------------------------+
                          | Postgres                 |
                          | chat archive             |
                          | media + transcript refs  |
                          +--------------------------+
```

## Repository responsibilities

### `go-whatsapp-ai-integration-POC`

- Recebe webhook do WhatsApp via Meta ou Twilio.
- Para Twilio, baixa áudio enviado pelo usuário.
- Chama o serviço de transcrição quando existe `TRANSCRIPTION_API_BASE_URL`.
- Gera resposta com Gemini.
- Responde ao usuário via Twilio ou Meta.
- Persiste:
  - contexto curto em Redis;
  - arquivo conversacional em Postgres.

### `go-audio-transcription`

- Recebe multipart `audio` em `POST /transcribe`.
- Transcreve o áudio com Gemini.
- Opcionalmente enriquece a transcrição.
- Persiste o registro completo no MongoDB.
- Retorna `Id`, `transcript`, `language` e `audioDuration` para o serviço de WhatsApp.

## Persistence split

### Redis

- histórico curto usado para contexto do chatbot;
- deduplicação/idempotência do webhook;
- não é fonte durável.

### Postgres

- trilha de auditoria do fluxo conversacional;
- cada linha guarda:
  - `phone_number`
  - `role`
  - `body`
  - `message_type`
  - `provider`
  - `provider_message_id`
  - `media_url`
  - `media_content_type`
  - `media_filename`
  - `transcription_id`
  - `transcription_language`
  - `audio_duration_seconds`
  - `created_at`

### MongoDB

- documento completo da transcrição;
- continua sendo a fonte principal do domínio de áudio.

## Main flows

### Text

```text
WhatsApp -> Twilio webhook -> queue -> chatbot.Service -> Gemini -> Twilio send -> Redis/Postgres
```

### Audio

```text
WhatsApp -> Twilio webhook -> queue -> download Twilio media
         -> go-audio-transcription /transcribe -> MongoDB
         -> chatbot.Service with transcript -> Gemini -> Twilio send
         -> Redis/Postgres with transcript reference
```

### Unsupported media

```text
WhatsApp image/document -> Twilio webhook -> queue -> deterministic unsupported-media reply
                        -> Redis/Postgres archive
```

## Operational notes

- A fila ainda é em memória. Se o processo reiniciar, jobs pendentes são perdidos.
- O webhook da Twilio depende da URL pública correta para validar assinatura; `TWILIO_WEBHOOK_BASE_URL` existe para reduzir erro em proxy reverso.
- O transcript completo fica no MongoDB do `go-audio-transcription`; o Postgres guarda a referência operacional para consulta cruzada.
