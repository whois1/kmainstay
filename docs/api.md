# K-Mainstay HTTP and realtime API

This is the machine-oriented version 1 contract. All HTTP bodies are JSON. Timestamps are RFC 3339 strings and identifiers are opaque, typed strings. Successful lists are JSON arrays, never `null`.

Humans use the `kmainstay_session` HttpOnly, SameSite=Lax cookie returned by `POST /api/session`. Cookie mutations require an `Origin` matching the request host. Bots use `Authorization: Bearer km_live_<lookup>_<secret>`. Errors are `{"error":"human-readable message"}`.

## Endpoints

| Method | Path | Auth | Request | Success |
| --- | --- | --- | --- | --- |
| `GET` | `/healthz` | none | — | `200 {"status":"ok"}` |
| `POST` | `/api/session` | Origin | `LoginRequest` | `200 Principal` + cookie |
| `DELETE` | `/api/session` | human | — | `204` |
| `GET` | `/api/me` | either | — | `200 Principal` |
| `GET` | `/api/organisations` | either | — | `200 Organisation[]` |
| `GET/POST` | `/api/organisations/{organisation}/conversations` | member | — / `CreateConversationRequest` | `200 Conversation[]` / `201 Conversation` |
| `GET` | `/api/organisations/{organisation}/users` | member | — | `200 User[]` |
| `GET` | `/api/organisations/{organisation}/eligible-users?email={exact_email}` | organisation admin | — | `200 EligibleUser[]` |
| `POST` | `/api/organisations/{organisation}/users` | organisation admin | `AddOrganisationUserRequest` | `201 User`; duplicate membership or name `409` |
| `POST` | `/api/organisations/{organisation}/bots` | organisation admin | `CreateBotRequest` | `201 BotCreated`; duplicate name `409` |
| `DELETE` | `/api/organisations/{organisation}/bots/{bot}` | organisation admin | — | `204` |
| `POST/DELETE` | `/api/bots/{bot}/key` | organisation admin | — | `201 {"api_key":"…"}` / `204` |
| `GET` | `/api/conversations/{conversation}/messages?limit=50&before={message_id}` | participant | — | `200 Message[]`, oldest-first |
| `POST` | `/api/conversations/{conversation}/messages` | participant | `CreateMessageRequest` | `201 Message`; retry `200 Message` |
| `GET` | `/api/ws?after={sequence}` | either | WebSocket | `MessageCreatedEvent` stream |

## JSON Schema

```json
{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{
  "Principal":{"type":"object","required":["id","kind","name"],"properties":{"id":{"type":"string"},"kind":{"enum":["human","bot"]},"name":{"type":"string"}}},
  "Organisation":{"type":"object","required":["id","name","role"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"role":{"enum":["admin","member"]}}},
  "Conversation":{"type":"object","required":["id","name","visibility"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"visibility":{"enum":["organisation","members"]}}},
  "User":{"allOf":[{"$ref":"#/$defs/Principal"},{"type":"object","required":["role"],"properties":{"role":{"enum":["admin","member"]}}}]},
  "EligibleUser":{"type":"object","required":["id","name","email"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"email":{"type":"string"}}},
  "Message":{"type":"object","required":["id","conversation_id","author_id","author_name","author_kind","body","created_at","sequence"],"properties":{"id":{"type":"string"},"conversation_id":{"type":"string"},"author_id":{"type":"string"},"author_name":{"type":"string"},"author_kind":{"enum":["human","bot"]},"body":{"type":"string","minLength":1,"maxLength":20000},"client_id":{"type":"string"},"created_at":{"type":"string","format":"date-time"},"sequence":{"type":"integer","minimum":1}}},
  "LoginRequest":{"type":"object","required":["email","password"],"properties":{"email":{"type":"string"},"password":{"type":"string"}}},
  "CreateConversationRequest":{"type":"object","required":["name","visibility","member_ids"],"properties":{"name":{"type":"string"},"visibility":{"enum":["organisation","members"]},"member_ids":{"type":"array","items":{"type":"string"},"uniqueItems":true}}},
  "CreateBotRequest":{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"conversation_ids":{"type":"array","items":{"type":"string"}}}},
  "AddOrganisationUserRequest":{"type":"object","required":["user_id"],"properties":{"user_id":{"type":"string"}}},
  "BotCreated":{"allOf":[{"$ref":"#/$defs/User"},{"type":"object","required":["api_key"],"properties":{"api_key":{"type":"string","pattern":"^km_live_"}}}]},
  "CreateMessageRequest":{"type":"object","required":["body","client_id"],"properties":{"body":{"type":"string","minLength":1,"maxLength":20000},"client_id":{"type":"string","maxLength":200}}},
  "MessageCreatedEvent":{"type":"object","required":["version","type","sequence","payload"],"properties":{"version":{"const":1},"type":{"const":"message.created"},"sequence":{"type":"integer"},"payload":{"$ref":"#/$defs/Message"}}}
}}
```

Eligible-user discovery is admin-only, requires an exact email search, and returns a matching existing human only when it is not already in the organisation and its normalised name does not conflict. Results contain only ID, name and email. Adding one always creates a `member` membership; it does not create an account or invitation. Bot keys are returned once. User display names are trimmed with Unicode whitespace rules and Unicode-lowercased for organisation-scoped uniqueness across both humans and bots. When migration encounters legacy collisions, it preserves every user and deterministically appends ` 2`, ` 3`, and so on to later display names. Membership roles are deliberately limited to `admin` and `member`; bots are members, and only human admins can add existing humans, create bots, rotate/revoke keys, or remove bots. Removal deletes the bot's membership and private-conversation access in that organisation. When no memberships remain, its keys are deleted; the user identity remains only when required to preserve authored-message attribution. Login attempts are limited to five per minute per normalized email address; the bounded in-memory limiter resets when the process restarts. Concurrent Argon2 password checks are also capped to protect server memory.

WebSocket delivery is at-least-once: reconnect with the greatest fully processed `sequence` in `after` and deduplicate by `payload.id`. Each internal notification is only a wake-up; the server reads all authorized durable events after the last delivered sequence, so coalesced notifications do not create gaps. Events contain complete messages only. Revoking an API key rejects new HTTP requests and reconnects but does not forcibly close a WebSocket that already authenticated; clients must reconnect after a close, and conversation-access changes are applied during replay.
