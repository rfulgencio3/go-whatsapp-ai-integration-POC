# go-whatsapp-ai-integration-POC

`go-whatsapp-ai-integration-POC` exposes a Go HTTP API that receives WhatsApp webhook notifications, generates answers with Gemini, and sends the reply back to the user through the WhatsApp Cloud API.

The project is organized with a clean architecture style:

- `internal/domain`: core business entities and rules.
- `internal/usecase`: application orchestration and business flow.
- `internal/adapters`: external providers and repositories.
- `internal/transport`: HTTP controllers, DTOs, and Swagger/OpenAPI assets.
- `internal/app`: dependency wiring and server bootstrap.

## Endpoints

- `GET /healthz`: application health and loaded integration status.
- `GET /webhook`: Meta webhook verification.
- `POST /webhook`: receives WhatsApp notifications.
- `POST /simulate`: tests the conversation flow without WhatsApp.
- `GET /swagger`: Swagger UI.
- `GET /swagger/openapi.json`: OpenAPI document.

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `HTTP_ADDRESS` | no | HTTP bind address, default `:8081` |
| `REQUEST_TIMEOUT` | no | outbound HTTP timeout, default `20s` |
| `CONVERSATION_HISTORY_LIMIT` | no | messages kept in memory per phone number, default `12` |
| `WHATSAPP_VERIFY_TOKEN` | for real webhook | token used in Meta webhook verification |
| `WHATSAPP_ACCESS_TOKEN` | for real replies | WhatsApp Cloud API bearer token |
| `WHATSAPP_PHONE_NUMBER_ID` | for real replies | WhatsApp Cloud API phone number id |
| `GEMINI_API_KEY` | for real AI replies | Gemini API key |
| `GEMINI_MODEL` | no | Gemini model, default `gemini-2.0-flash` |
| `SYSTEM_PROMPT` | no | base system instruction for the assistant |
| `ALLOWED_PHONE_NUMBER` | no | optional allowlist for a single phone number |

## Run

```powershell
$env:HTTP_ADDRESS=":8081"
$env:WHATSAPP_VERIFY_TOKEN="your-verify-token"
$env:WHATSAPP_ACCESS_TOKEN="your-meta-token"
$env:WHATSAPP_PHONE_NUMBER_ID="1234567890"
$env:GEMINI_API_KEY="your-gemini-key"
go run .
```

If `GEMINI_API_KEY` is not configured, the application falls back to a deterministic mock reply so the end-to-end pipeline can still be exercised locally.

If WhatsApp sender credentials are not configured, the reply is logged instead of sent.

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
4. Set `WHATSAPP_ACCESS_TOKEN` and `WHATSAPP_PHONE_NUMBER_ID`.
5. Set `GEMINI_API_KEY`.
6. Expose the local server through HTTPS with a tunnel such as ngrok or Cloudflare Tunnel.

## Current limitations

- Conversation history is in memory only.
- Only text messages are processed.
- There is no retry, queue, or deduplication strategy.
- There is no webhook signature validation yet.
- Audio, image, and document messages are not supported.



