# K-Mainstay HTTP and realtime API

This is the machine-oriented version 1 contract. HTTP bodies are JSON except for image message creation, which uses `multipart/form-data`. Timestamps are RFC 3339 strings and identifiers are opaque, typed strings. Successful lists are JSON arrays, never `null`.

Humans use the `kmainstay_session` HttpOnly, SameSite=Lax cookie returned by `POST /api/session`. Cookie mutations require an `Origin` matching the request host. Bots use `Authorization: Bearer km_live_<lookup>_<secret>`. Errors are `{"error":"human-readable message"}`.

## Endpoints

| Method | Path | Auth | Request | Success |
| --- | --- | --- | --- | --- |
| `GET` | `/healthz` | none | — | `200 {"status":"ok"}` |
| `POST` | `/api/session` | Origin | `LoginRequest` | `200 Principal` + cookie |
| `DELETE` | `/api/session` | human | — | `204` |
| `GET` | `/api/me` | either | — | `200 Principal` |
| `GET` | `/api/organisations` | either | — | `200 Organisation[]` |
| `GET` | `/api/organisations/{organisation}/conversations?include_archived=true` | member | — | `200 Conversation[]`; archived conversations are omitted unless requested |
| `POST` | `/api/organisations/{organisation}/conversations` | member | `CreateConversationRequest` | `201 Conversation` |
| `PUT` | `/api/conversations/{conversation}/title` | participant | `UpdateConversationTitleRequest` | `200 ConversationTitle`; `Everyone` returns `409` |
| `PUT/DELETE` | `/api/conversations/{conversation}/archive` | human participant | — | archive/restore for that user; `204`; `Everyone` returns `409` |
| `DELETE` | `/api/organisations/{organisation}/conversations/{conversation}` | human organisation admin | — | `204` for non-`Everyone` organisation conversations and groups of at least three members; `Everyone` and direct chats return `409` |
| `GET` | `/api/organisations/{organisation}/users` | member | — | `200 User[]` |
| `GET` | `/api/organisations/{organisation}/eligible-users?email={exact_email}` | organisation admin | — | `200 EligibleUser[]` |
| `POST` | `/api/organisations/{organisation}/users` | organisation admin | `AddOrganisationUserRequest` | `201 User`; duplicate membership or name `409` |
| `POST` | `/api/organisations/{organisation}/bots` | organisation admin | `CreateBotRequest` | `201 BotCreated`; duplicate name `409` |
| `DELETE` | `/api/organisations/{organisation}/bots/{bot}` | organisation admin | — | `204` |
| `POST/DELETE` | `/api/bots/{bot}/key` | organisation admin | — | `201 {"api_key":"…"}` / `204` |
| `GET` | `/api/conversations/{conversation}/messages?limit=50&before={message_id}` or `?limit=100&after_sequence={sequence}` | participant | — | `200 Message[]`, oldest-first |
| `POST` | `/api/conversations/{conversation}/messages` | participant | JSON `CreateMessageRequest`, or multipart `body`, `client_id`, `reply_to_message_id`, repeated `image` | `201 Message`; retry `200 Message` |
| `PUT` | `/api/conversations/{conversation}/messages/{message}` | message author and participant | `UpdateMessageRequest` | `200 Message` |
| `PUT` | `/api/conversations/{conversation}/activity` | bot participant | `ConversationActivityRequest` | `204` |
| `GET` | `/api/attachments/{attachment}/content` | participant | — | authorised JPEG or PNG bytes |
| `PUT` | `/api/conversations/{conversation}/read` | participant | `ReadPositionRequest` | `200 ReadPosition` |
| `GET` | `/api/ws?after={sequence}` | either | WebSocket | durable `MessageCreatedEvent` and human-only `MessageUpdatedEvent`, plus ephemeral `ConversationDeletedEvent` and `ConversationActivityEvent` |

## JSON Schema

```json
{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{
  "Principal":{"type":"object","required":["id","kind","name"],"properties":{"id":{"type":"string"},"kind":{"enum":["human","bot"]},"name":{"type":"string"}}},
  "Organisation":{"type":"object","required":["id","name","role"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"role":{"enum":["admin","member"]}}},
  "Conversation":{"type":"object","required":["id","name","visibility","member_ids"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"visibility":{"enum":["organisation","members"]},"member_ids":{"type":"array","items":{"type":"string"},"uniqueItems":true},"read_sequence":{"type":"integer","minimum":0},"latest_sequence":{"type":"integer","minimum":0},"activity_at":{"type":"string","format":"date-time"},"title_automatic":{"type":"boolean"},"archived":{"type":"boolean"},"is_everyone":{"type":"boolean"}}},
  "User":{"allOf":[{"$ref":"#/$defs/Principal"},{"type":"object","required":["role"],"properties":{"role":{"enum":["admin","member"]}}}]},
  "EligibleUser":{"type":"object","required":["id","name","email"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"email":{"type":"string"}}},
  "MentionedUser":{"type":"object","required":["id","name"],"properties":{"id":{"type":"string"},"name":{"type":"string"}}},
  "Attachment":{"type":"object","required":["id","media_type","byte_size","width","height","original_filename","created_at","content_url"],"properties":{"id":{"type":"string"},"media_type":{"enum":["image/jpeg","image/png"]},"byte_size":{"type":"integer","minimum":1,"maximum":10485760},"width":{"type":"integer","minimum":1},"height":{"type":"integer","minimum":1},"original_filename":{"type":"string"},"created_at":{"type":"string","format":"date-time"},"content_url":{"type":"string"}}},
  "MessageReply":{"type":"object","required":["id","author_name","body"],"properties":{"id":{"type":"string"},"author_name":{"type":"string"},"body":{"type":"string"}}},
  "Message":{"type":"object","required":["id","conversation_id","author_id","author_name","author_kind","body","created_at","edited_at","sequence","mentions","attachments"],"properties":{"id":{"type":"string"},"conversation_id":{"type":"string"},"author_id":{"type":"string"},"author_name":{"type":"string"},"author_kind":{"enum":["human","bot"]},"body":{"type":"string","maxLength":20000},"client_id":{"type":"string"},"reply_to":{"$ref":"#/$defs/MessageReply"},"created_at":{"type":"string","format":"date-time"},"edited_at":{"type":["string","null"],"format":"date-time"},"sequence":{"type":"integer","minimum":1},"mentions":{"type":"array","items":{"$ref":"#/$defs/MentionedUser"},"uniqueItems":true},"attachments":{"type":"array","maxItems":10,"items":{"$ref":"#/$defs/Attachment"}}}},
  "LoginRequest":{"type":"object","required":["email","password"],"properties":{"email":{"type":"string"},"password":{"type":"string"}}},
  "CreateConversationRequest":{"type":"object","required":["name","visibility","member_ids"],"properties":{"name":{"type":"string"},"visibility":{"enum":["organisation","members"]},"member_ids":{"type":"array","items":{"type":"string"},"uniqueItems":true},"automatic_title":{"type":"boolean"}}},
  "UpdateConversationTitleRequest":{"type":"object","required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":20000}}},
  "ConversationTitle":{"type":"object","required":["id","name","title_automatic"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"title_automatic":{"const":false}}},
  "CreateBotRequest":{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"conversation_ids":{"type":"array","items":{"type":"string"}}}},
  "AddOrganisationUserRequest":{"type":"object","required":["user_id"],"properties":{"user_id":{"type":"string"}}},
  "BotCreated":{"allOf":[{"$ref":"#/$defs/User"},{"type":"object","required":["api_key"],"properties":{"api_key":{"type":"string","pattern":"^km_live_"}}}]},
  "CreateMessageRequest":{"type":"object","required":["body"],"properties":{"body":{"type":"string","minLength":1,"maxLength":20000},"client_id":{"type":"string","maxLength":200},"reply_to_message_id":{"type":"string","maxLength":200}}},
  "UpdateMessageRequest":{"type":"object","required":["body"],"properties":{"body":{"type":"string","maxLength":20000}}},
  "ConversationActivityRequest":{"type":"object","required":["active"],"properties":{"active":{"type":"boolean"}}},
  "ConversationActivity":{"type":"object","required":["conversation_id","user_id","user_name","user_kind","active","expires_at"],"properties":{"conversation_id":{"type":"string"},"user_id":{"type":"string"},"user_name":{"type":"string"},"user_kind":{"const":"bot"},"active":{"type":"boolean"},"expires_at":{"type":"string","format":"date-time"}}},
  "ReadPositionRequest":{"type":"object","required":["sequence"],"properties":{"sequence":{"type":"integer","minimum":0}}},
  "ReadPosition":{"type":"object","required":["sequence"],"properties":{"sequence":{"type":"integer","minimum":0}}},
  "MessageCreatedEvent":{"type":"object","required":["version","type","sequence","payload"],"properties":{"version":{"const":1},"type":{"const":"message.created"},"sequence":{"type":"integer"},"payload":{"$ref":"#/$defs/Message"}}},
  "MessageUpdatedEvent":{"type":"object","required":["version","type","sequence","payload"],"properties":{"version":{"const":1},"type":{"const":"message.updated"},"sequence":{"type":"integer"},"payload":{"$ref":"#/$defs/Message"}}},
  "ConversationDeletedEvent":{"type":"object","required":["version","type","payload"],"properties":{"version":{"const":1},"type":{"const":"conversation.deleted"},"payload":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}},
  "ConversationActivityEvent":{"type":"object","required":["version","type","payload"],"properties":{"version":{"const":1},"type":{"const":"conversation.activity"},"payload":{"$ref":"#/$defs/ConversationActivity"}}}
}}
```

## Image messages

Authors may edit only the body/caption of their own message with `PUT`. The body is limited to 20,000 characters and may be empty or whitespace only when the existing message has attachments. An edit preserves the message ID, author, creation timestamp and sequence, attachments, reply target, and client ID. It sets `edited_at`, recomputes display mentions, and does not change bot-delivery routing, reads, conversation activity/order, archive state, or automatic titles. Submitting the unchanged body is a no-op that preserves `edited_at` and emits no event. Message creation and edit requests share the per-principal message rate limit.

Send text-only messages as JSON. Send a message with images as multipart form data with these fields:

- `body`: optional caption, at most 20,000 characters;
- `client_id`: optional idempotency identifier, at most 200 characters;
- `reply_to_message_id`: optional message in the same conversation;
- `image`: one to ten repeated JPEG or PNG parts. Each image may be at most 10 MB, all images together may be at most 20 MB, and each decoded image may be at most 10,000 by 10,000 and 16 million pixels.

For example, a bot can upload an image through the same participant-authorised endpoint:

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $KMAINSTAY_API_KEY" \
  -F body='Optional caption' \
  -F client_id="bot-$(date +%s)" \
  -F image=@./photo.png \
  -F image=@./second-photo.jpg \
  "$KMAINSTAY_URL/api/conversations/$CONVERSATION_ID/messages"
```

The server checks the declared media type, file signature and complete decoded image rather than trusting the filename. Responses, history and realtime events include an `attachments` array. Fetch `content_url` with the same session cookie or Bearer key; knowing an attachment ID does not grant access. The content response uses `no-store` and `nosniff`, and is not a public filesystem URL.

Eligible-user discovery is admin-only, requires an exact email search, and returns a matching existing human only when it is not already in the organisation and its normalised name does not conflict. Results contain only ID, name and email. Adding one always creates a `member` membership; it does not create an account or invitation. Bot keys are returned once. User display names are trimmed with Unicode whitespace rules, canonicalised to NFC, and Unicode-lowercased for organisation-scoped uniqueness across both humans and bots. When migration encounters legacy collisions, including canonically equivalent NFC/NFD spellings, it preserves every user and deterministically appends ` 2`, ` 3`, and so on to later display names. Membership roles are deliberately limited to `admin` and `member`; bots are members, and only human admins can add existing humans, create bots, rotate/revoke keys, remove bots, or permanently delete non-`Everyone` organisation conversations and user-created groups of at least three members. Direct chats can be archived personally but cannot be deleted for everyone. Deleting an eligible conversation removes it and all of its messages and realtime events for every participant, so no existing participant or API key can continue messaging it. Bot removal deletes the bot's membership and private-conversation access in that organisation. When no memberships remain, its keys are deleted; the user identity remains only when required to preserve authored-message attribution. Login attempts are limited to five per minute per normalized email address; the bounded in-memory limiter resets when the process restarts. Concurrent Argon2 password checks are also capped to protect server memory.

Conversation list and create responses include `member_ids` (`[]` for organisation-visible conversations), `activity_at`, `title_automatic`, and `is_everyone`, while list responses also include `read_sequence`, `latest_sequence`, and the authenticated principal's `archived` state. The organisation-wide `Everyone` conversation has `is_everyone: true` and cannot be renamed, archived, or deleted; these mutations return `409`. Other user-visible conversation names are unique within an organisation after Unicode whitespace trimming, NFC canonicalisation, and Unicode case folding; conflicting creation or rename requests return `409`. Archived conversations are omitted by default and included with `include_archived=true`. Archive/restore is per human user, preserves history and access, and new message activity restores the conversation for all users who archived it. Lists are newest-activity-first; an empty conversation uses its creation time. A create request may mark its supplied name as `automatic_title`. The first non-empty message body then becomes a unique title, with a numeric suffix when needed. Editing the title clears that flag, so later messages never overwrite it. `after_sequence` retrieves the next bounded page in event order so clients can open at the true unread boundary without loading an entire large history. Read positions are monotonic. A positive sequence submitted to the read endpoint must identify a message in that conversation, and successfully creating a message advances its author's position in the same transaction. A reply target must belong to the same conversation; message responses expose its ID, author name, and body as `reply_to` without nesting replies.

Message delivery over WebSocket is at-least-once: reconnect with the greatest fully processed `sequence` in `after` and deduplicate by `payload.id`. Each internal message notification is only a wake-up; the server reads all authorised durable message events after the last delivered sequence, so coalesced notifications do not create gaps. `conversation.deleted` is an immediate list-removal notification for connected organisation members and has no sequence because deletion removes the conversation's stored events. `conversation.activity` is non-persisted and bot-only: `active:true` expires after six seconds unless refreshed, while `active:false` clears immediately. Clients also clear a bot's activity when its complete message arrives. Clients must fetch the authorised conversation list after reconnecting, which reconciles deletion notifications missed while disconnected; periodic activity refreshes restore a working indicator after a brief connection loss. Revoking an API key rejects new HTTP requests and reconnects but does not forcibly close a WebSocket that already authenticated; clients must reconnect after a close, and conversation-access changes are applied during replay.

Human WebSocket clients receive every message they can access. Bots receive a human-authored message without an explicit mention only in a `members` conversation whose only two members are that human author and that bot. In organisation conversations and private conversations with additional members, a bot receives the message only when its organisation-scoped display name is written as `@Display Name` (case-insensitive). Name boundaries prevent email addresses and longer names from matching. The server derives mentions from the persisted body, stores them durably, and records bot delivery eligibility when the message is created so later membership changes do not rewrite routing history. `Message.mentions` returns the recognised IDs and name snapshots; client-supplied mention IDs are not accepted. Messages authored by bots are never delivered to another bot.
