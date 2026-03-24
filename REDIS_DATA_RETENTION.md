# Redis Data Retention

## Purpose

Redis is not treated as a single generic "message store" in this architecture.
Each Redis key serves a specific operational purpose and should use a TTL that matches that purpose.

In this project, Redis is used for volatile operational data on the hot path.
Postgres is the durable source of truth for archived chat data and long-term history.

## Redis Key Categories and TTLs

### `chat:buffer:{phone_number}`

- Purpose: short aggregation window before sending text to Gemini
- Default TTL: `30s`
- Rationale: this key is only meant to collect a burst of user messages sent within a short time window
- Note: this key is only needed when buffered or debounced AI processing is introduced

### `webhook:processing:{message_id}`

- Purpose: short-lived processing lock to avoid concurrent handling of the same inbound WhatsApp event
- TTL range: `90s` to `120s`
- Default TTL: `2m`
- Rationale: this key should live only slightly longer than the expected processing time for a single webhook event

### `webhook:idempotency:{message_id}`

- Purpose: prevent duplicate webhook deliveries from being processed more than once
- Default TTL: `72h`
- Rationale: this key should remain available long enough to absorb duplicate deliveries and delayed retries from the provider

### `chat:history:{phone_number}`

- Purpose: hot conversation history for contextual Gemini replies
- Default TTL: `24h`
- Rationale: this key stores recent context for the chatbot, but should remain short-lived because durable history is archived in Postgres

## Postgres Responsibility

Postgres stores durable chat history without TTL.
Redis keys in this design are operational and disposable, while Postgres is responsible for auditability, durable history, and future rehydration strategies.

This separation keeps Redis focused on low-latency state and keeps long-term data in the persistence layer designed for that purpose.

## Recommended Defaults for This Project

- Conversation history in Redis: `24h`
- Webhook idempotency keys: `72h`
- Webhook processing lock: `2m`
- AI message buffer: `30s` only when buffering is enabled
- Durable archive: Postgres, no expiration

## Official Redis Guidance Used

The following official Redis documentation informs these defaults:

- `EXPIRE` and TTL-based temporary state  
  https://redis.io/docs/latest/commands/expire/
- `SET ... NX EX|PX` for lock semantics  
  https://redis.io/docs/latest/commands/set/
- Distributed lock guidance  
  https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/
- Eviction and memory monitoring guidance  
  https://redis.io/docs/latest/develop/reference/eviction/

From these references, the project should follow these conclusions:

- Short TTLs fit buffering and processing locks
- Medium TTLs fit idempotency keys
- Hot chat history should stay short-lived because Postgres is the durable layer
- Every Redis key in this design should have an expiration

## Operational Notes

- Monitor `expired_keys` and `evicted_keys`
- Revisit TTLs if users commonly resume conversations after more than `24h`
- Keep Redis focused on hot-path state, not long-term archival data
- If Redis memory pressure becomes frequent, reduce conversation history TTL before reducing idempotency TTL

## Current Project Policy

For the current implementation and near-term roadmap:

- Use Redis hot history with a default TTL of `24h`
- Use Redis idempotency keys with a default TTL of `72h`
- Use Redis processing locks with a default TTL of `2m`
- Add a `30s` Redis message buffer only when buffered AI aggregation is implemented
- Keep Postgres as the durable archive with no expiration policy
