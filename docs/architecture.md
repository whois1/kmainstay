# MVP architecture

## Status

This is the agreed architecture for the first local and single-VPS MVP. It is intentionally narrow and may change with evidence.

## Shape

```text
Browser or bot runtime
        │ HTTPS / WebSocket
        ▼
      Caddy                 VPS only
        │ localhost HTTP
        ▼
One Go process
  ├─ standard net/http API
  ├─ WebSocket event feed
  ├─ embedded Vue application
  ├─ embedded SQL migrations
  ├─ database/sql ───────────────► SQLite database in WAL mode
  └─ attachment storage interface ► local immutable files
```

Local development does not require Caddy. The Go process and Vite development server are sufficient.

## Technology decisions

- Go standard `net/http`; no Chi or backend web framework.
- Vue 3, TypeScript, and Vite.
- `database/sql` with a pure-Go SQLite driver.
- No ORM initially. Database access stays behind focused Go packages so an ORM or generated query layer can be introduced later if handwritten SQL becomes a demonstrated maintenance cost.
- WebSocket for realtime event delivery; HTTP for history and mutations.
- Safe Markdown source stored unchanged and rendered client-side with raw HTML disabled.
- JPEG and PNG attachment metadata in SQLite, with bytes stored as immutable local files behind a provider-neutral storage key.
- One systemd-managed binary behind Caddy on one VPS.
- No Docker.

## Domain

```text
Organisation
User                  kind: human | bot
OrganisationMembership role: admin | member; unique trimmed, NFC-canonicalised and lowercased user name per organisation
Conversation          visibility: organisation | members
ConversationMember
Message
Attachment
HumanSession
APIKey
RealtimeEvent
```

Organisation-visible conversations are available to every organisation user. Member-visible conversations are private to their selected users. Nested message threads are deferred.

There is no separate Phase 1 `Agent` table. A bot is a user. Runtime-specific state remains outside the chat domain.

## Authentication

### Humans

- email/password;
- Argon2id password hashes;
- opaque server-side sessions;
- secure HTTP-only SameSite cookies;
- origin/CSRF checks for cookie-authenticated mutations;
- bounded in-memory login throttling: five attempts per minute for each normalized email address, independent of source address;
- a conservative semaphore around Argon2id verification to cap concurrent password-hashing memory use.

Public registration, invitations, password reset, and email delivery are deferred.

### Bots

1. Michael creates a bot user.
2. The server generates a high-entropy key with a recognisable prefix.
3. The raw key is shown once.
4. Only a lookup identifier and SHA-256 verifier are stored; comparison is constant-time (Argon2id remains exclusive to human passwords).
5. The key authenticates that bot user through a Bearer header.
6. Rotation revokes the previous active key; explicit revocation is supported.

Fine-grained scopes are deferred. A key inherits its bot user’s current conversation access.

Organisation administration deliberately uses only two membership roles. The bootstrap human is an admin; bots and newly added humans are members. Every member may view the organisation settings page and roster. Human admins may add an existing human account through an exact-email lookup, create bots, rotate or revoke bot keys, remove bots, and delete conversations. Conversation deletion is global and cascades through its members, messages and stored message events; connected clients receive `conversation.deleted`, and reconnecting clients refresh their authorised conversation list. Bot removal revokes organisation access but preserves any user identity needed by historical messages. Invitations, account creation, role editing and human deletion are deferred until their security rules are justified.

## Messaging

- Every message has exactly one user author.
- Humans and bots use the same create-message path.
- Messages are persisted before realtime fan-out.
- Complete messages only; no response deltas. Bot working activity is an ephemeral, expiring signal rather than message history.
- Bot-authored events are not delivered to other bots.
- The bot runtime decides whether to respond to ordinary messages or mentions.
- Messages accept bounded Markdown source text.
- A message may include up to ten JPEG or PNG images, each up to 10 MB and together up to 20 MB; an image-only message may have an empty body.
- Uploads are signature-checked, fully decoded under bounded concurrency, and dimension-bounded. Filenames are display metadata only and never become storage paths.
- History and WebSocket events contain attachment metadata and an authorised content URL, never image bytes or Base64.
- Client-generated idempotency identifiers prevent duplicate sends on retry.
- An author may transactionally replace only their message body/caption. The update preserves creation identity and routing, recomputes display mentions, and records nullable `edited_at`.
- Message edits do not mutate bot deliveries, reads, latest-message/activity ordering, archives, or automatic titles.

## Attachment storage

The MVP stores attachment metadata, dimensions, byte size and SHA-256 checksum in SQLite. Opaque storage keys resolve to immutable files under `ATTACHMENT_PATH`, which defaults to a directory beside the database. Every content request is authorised through conversation membership; filesystem paths are never public.

The storage package supports save, open and delete by opaque key. Moving to S3-compatible storage later requires copying objects under the same keys and replacing that implementation, not changing message IDs, API URLs, permissions or frontend data shapes. Local deployment uses `/var/lib/kmainstay/uploads` with mode `0700` and object files with mode `0600`.

Deleting a conversation cascades attachment metadata and then removes its objects. A failed object deletion can leave an unreachable orphan and is logged; it must not resurrect or block deletion of the authoritative chat record. Crash-safe writes use a staging file, atomic link and directory sync. On startup, local storage removes only staging files older than 24 hours. Final objects are never inferred to be abandoned merely because they are absent from one database snapshot; that preserves recovery options after restoring either side of the backup pair.

## Realtime events

A versioned envelope uses `message.created` for creation and `message.updated` for human-client replacement. Both are durable. Creation rows remain alone in `realtime_events` so a rolled-back binary cannot mistake an update for a creation. Update rows live in `message_update_events`; current servers allocate from a shared SQLite integer sequence before inserting either kind of event, ordering both tables in one gapless replay cursor. The update envelope receives a new replay sequence while its payload retains the original creation sequence.

Migration 11 is deliberately fail-closed across rollback: its `realtime_events` trigger rejects inserts without an explicit sequence with a current-binary schema error. A rolled-back binary therefore cannot receive a stale `LastInsertId` or write a creation event whose response and read cursor refer to a different sequence.

Bots are excluded from update replay. Each creation row also stores the completed creation payload so delayed bot replay preserves the body, mentions, attachments, and reply preview as they existed at creation. Human history and update envelopes render current message state.

Creation example:

```json
{
  "version": 1,
  "type": "message.created",
  "sequence": 42,
  "payload": {
    "id": "msg_...",
    "conversation_id": "conv_...",
    "author_id": "usr_...",
    "author_name": "Hector",
    "author_kind": "bot",
    "body": "Complete message body",
    "client_id": "optional-idempotency-id",
    "created_at": "2026-08-14T10:00:00Z",
    "sequence": 42
  }
}
```

Message events are durable. The server subscribes before replay. A reconnecting client supplies its last fully processed sequence and receives every currently accessible later event. Live hub notifications are wake-up signals only: after any wake, the server queries all authorized durable events beyond the last delivered sequence, so notification coalescing or drops cannot make delivery gaps. Delivery is at least once; clients deduplicate by event/message ID.

Bot participants may also publish `conversation.activity` start, refresh and stop signals. These have no sequence and are never stored. An active signal carries a six-second expiry, while the bot runtime refreshes it every two seconds; the browser removes it on explicit stop, complete bot output, or expiry. This bounds stale “working” state after crashes without adding a database lifecycle.

The connection context ends when the peer closes, and every server write has a short deadline. API-key revocation prevents new requests and reconnects but does not forcibly terminate a connection that completed its authentication handshake. Conversation authorization is re-evaluated for each replay batch, so removed access stops subsequent delivery; clients reconnect after any socket close using their last fully processed sequence.

Organisation events reach all organisation members. Private-conversation events reach only participants.

## SQLite operating constraints

- exactly one application process;
- WAL mode;
- foreign keys enabled;
- bounded busy timeout;
- short transactions;
- application-generated IDs and portable UTC timestamps;
- tested online backup and restore procedure before production reliance.
- backups and restores must include both the SQLite database and the attachment directory; neither is a complete backup alone.

Do not horizontally scale this design. Move to PostgreSQL if concurrent writes, operations, or availability requirements justify it.

## Security floor

- TLS for public traffic;
- parameterised SQL;
- passwords and API keys never stored in plaintext;
- server-side authorisation on every read, write, replay, and WebSocket event;
- bounded payloads and rates;
- safe Markdown rendering with no raw HTML or unsafe URL schemes;
- secrets excluded from logs;
- revocable bot keys;
- no runtime/model credentials stored by K-Mainstay.

This is a pilot security floor, not an enterprise model.

## Deployment

```text
/usr/local/bin/kmainstay
/etc/kmainstay/kmainstay.env
/var/lib/kmainstay/kmainstay.db
```

- Vue production assets and migrations are embedded in the Go binary.
- systemd runs one unprivileged process.
- Caddy provides automatic HTTPS and WebSocket reverse proxying.
- Database backups use SQLite’s online backup mechanism rather than copying a live file blindly.

The VPS, domain, and backup destination are selected only after the local acceptance path passes.
