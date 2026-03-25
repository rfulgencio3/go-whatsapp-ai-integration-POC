# go-whatsapp-ai-integration-POC

`go-whatsapp-ai-integration-POC` exposes a Go HTTP API that receives WhatsApp webhook notifications, generates answers with Gemini, and sends the reply back to the user through the WhatsApp Cloud API.

The project is organized with a clean architecture style:

- `internal/domain`: core business entities and rules.
- `internal/usecase`: application orchestration and business flow.
- `internal/adapters`: external providers and repositories.
- `internal/transport`: HTTP controllers, DTOs, and Swagger/OpenAPI assets.
- `internal/app`: dependency wiring and server bootstrap.

## Endpoints

- `GET /privacy-policy`: public privacy policy page for Meta configuration.
- `GET /healthz`: application health and loaded integration status.
- `GET /metrics`: in-memory application counters for webhook and simulation flows.
- `GET /webhook`: Meta webhook verification.
- `POST /webhook`: receives WhatsApp notifications and enqueues them for async processing.
- `POST /simulate`: tests the conversation flow without WhatsApp.
- `GET /swagger`: Swagger UI.
- `GET /swagger/openapi.json`: OpenAPI document.

## Privacy Policy URL

After deployment, use this URL in Meta:

```text
https://<your-domain>/privacy-policy
```

For Railway, this will usually look like:

```text
https://<your-app>.up.railway.app/privacy-policy
```

A Markdown source version of the same policy is stored in `PRIVACY_POLICY.md`.

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `HTTP_ADDRESS` | no | HTTP bind address, overrides `PORT` when both exist |
| `PORT` | no | container-friendly fallback port, useful for Railway |
| `REQUEST_TIMEOUT` | no | outbound HTTP timeout, default `20s` |
| `CONVERSATION_HISTORY_LIMIT` | no | messages kept in the hot conversation store, default `12` |
| `WHATSAPP_VERIFY_TOKEN` | for real webhook | token used in Meta webhook verification |
| `WHATSAPP_APP_SECRET` | recommended for real webhook | Meta app secret used to validate `X-Hub-Signature-256` on incoming webhook notifications |
| `WHATSAPP_ACCESS_TOKEN` | for real replies | WhatsApp Cloud API bearer token |
| `WHATSAPP_PHONE_NUMBER_ID` | for real replies | WhatsApp Cloud API phone number id |
| `GEMINI_API_KEY` | for real AI replies | Gemini API key |
| `GEMINI_MODEL` | no | Gemini model, default `gemini-2.0-flash` |
| `SYSTEM_PROMPT` | no | base system instruction for the assistant |
| `ALLOWED_PHONE_NUMBER` | no | optional allowlist for a single phone number |
| `REDIS_URL` | recommended | Redis connection string used for recent conversation history and webhook idempotency |
| `REDIS_CONVERSATION_TTL` | no | TTL for Redis conversation keys, default `24h` |
| `REDIS_KEY_PREFIX` | no | Redis key prefix for history, default `chat:history` |
| `WEBHOOK_IDEMPOTENCY_TTL` | no | TTL for processed WhatsApp `message_id` keys, default `72h` |
| `WEBHOOK_PROCESSING_TTL` | no | TTL for in-flight webhook processing locks, default `2m` |
| `REDIS_IDEMPOTENCY_PREFIX` | no | Redis key prefix for processed webhook ids, default `webhook:idempotency` |
| `REDIS_PROCESSING_PREFIX` | no | Redis key prefix for in-flight webhook locks, default `webhook:processing` |
| `WEBHOOK_QUEUE_WORKERS` | no | worker count for async webhook processing, default `4` |
| `WEBHOOK_QUEUE_BUFFER_SIZE` | no | in-memory queue capacity, default `128` |
| `WEBHOOK_QUEUE_MAX_RETRIES` | no | retry attempts after the first failed processing attempt, default `3` |
| `WEBHOOK_QUEUE_RETRY_DELAY` | no | base retry delay for async processing, default `2s` |
| `DATABASE_URL` | recommended | Postgres connection string used to archive all chat messages |

## Run

```powershell
$env:HTTP_ADDRESS=":8081"
$env:WHATSAPP_VERIFY_TOKEN="your-verify-token"
$env:WHATSAPP_APP_SECRET="your-meta-app-secret"
$env:WHATSAPP_ACCESS_TOKEN="your-meta-token"
$env:WHATSAPP_PHONE_NUMBER_ID="1234567890"
$env:GEMINI_API_KEY="your-gemini-key"
$env:REDIS_URL="redis://localhost:6379/0"
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/whatsapp_ai?sslmode=disable"
go run .
```

Behavior by configuration:

- without `REDIS_URL`, the application falls back to in-memory conversation history and in-memory webhook idempotency;
- with `REDIS_URL`, recent conversation context and webhook message deduplication are stored in Redis;
- webhook requests are acknowledged after enqueue, and the actual processing happens in background workers;
- failed webhook jobs are retried asynchronously using the configured worker queue policy;
- with `DATABASE_URL`, every user and assistant message is archived in Postgres;
- without `GEMINI_API_KEY`, the application falls back to a deterministic mock reply;
- without WhatsApp sender credentials, outbound replies are logged instead of sent.

## Metrics

`GET /metrics` returns JSON counters for:

- received webhook requests
- extracted webhook messages
- successfully enqueued webhook messages
- enqueue failures
- processed webhook messages
- duplicate webhook messages skipped by idempotency
- async retry attempts
- webhook processing failures after retries are exhausted
- simulation requests
- simulation failures

## Local simulation

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8081/simulate `
  -ContentType "application/json" `
  -Body '{"phone_number":"5511999999999","message":"Which documents are required to renew my registration?"}'
```

## Swagger UI

Open `http://localhost:8081` in the browser. The root path redirects to `/swagger`.

The page loads Swagger UI assets from `unpkg.com`, while the OpenAPI document is served by this application at `/swagger/openapi.json`.

## Real WhatsApp test

1. Create a Meta app with WhatsApp Cloud API enabled.
2. Configure the webhook URL to point to `GET/POST /webhook`.
3. Use the same value in Meta and `WHATSAPP_VERIFY_TOKEN`.
4. Set `WHATSAPP_APP_SECRET`, `WHATSAPP_ACCESS_TOKEN`, and `WHATSAPP_PHONE_NUMBER_ID`.
5. Set `GEMINI_API_KEY`.
6. Configure `REDIS_URL` for hot conversation state and webhook idempotency.
7. Configure `DATABASE_URL` for durable message history.
8. Publish and use `https://<your-domain>/privacy-policy` in the Meta Privacy Policy URL field.
9. Expose the local server through HTTPS with a tunnel such as ngrok or Cloudflare Tunnel when needed.

## Current limitations

- The async queue is in-memory only and is not durable across process restarts.
- Postgres currently stores message history but not higher-level conversation/session entities.
- Audio, image, and document messages are not supported.
- The webhook still processes only text messages.
- Metrics are in-memory only and reset on restart.
