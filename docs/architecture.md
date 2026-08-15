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
  └─ database/sql
        │
        ▼
SQLite database in WAL mode
```

Local development does not require Caddy. The Go process and Vite development server are sufficient.

## Technology decisions

- Go standard `net/http`; no Chi or backend web framework.
- Vue 3, TypeScript, and Vite.
- `database/sql` with a pure-Go SQLite driver.
- No ORM initially. Database access stays behind focused Go packages so an ORM or generated query layer can be introduced later if handwritten SQL becomes a demonstrated maintenance cost.
- WebSocket for realtime event delivery; HTTP for history and mutations.
- Safe Markdown source stored unchanged and rendered client-side with raw HTML disabled.
- One systemd-managed binary behind Caddy on one VPS.
- No Docker.

## Domain

```text
Organisation
User                  kind: human | bot
OrganisationMembership role: admin | member; unique normalised user name per organisation
Conversation          visibility: organisation | members
ConversationMember
Message
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

Organisation administration deliberately uses only two membership roles. The bootstrap human is an admin; bots are members. Every member may view the organisation roster, while only human admins may create bots or rotate and revoke bot keys.

## Messaging

- Every message has exactly one user author.
- Humans and bots use the same create-message path.
- Messages are persisted before realtime fan-out.
- Complete messages only; no response deltas or activity events.
- Bot-authored events are delivered to other bots.
- The bot runtime decides whether to respond to ordinary messages or mentions.
- Messages accept bounded Markdown source text.
- Client-generated idempotency identifiers prevent duplicate sends on retry.

## Realtime events

A versioned envelope begins with one event type:

```json
{
  "version": 1,
  "id": "evt_...",
  "sequence": 42,
  "type": "message.created",
  "occurred_at": "2026-08-14T10:00:00Z",
  "organisation_id": "org_...",
  "conversation_id": "conv_...",
  "data": {
    "message": {}
  }
}
```

Events are durable. The server subscribes before replay. A reconnecting client supplies its last fully processed sequence and receives every currently accessible later event. Live hub notifications are wake-up signals only: after any wake, the server queries all authorized durable events beyond the last delivered sequence, so notification coalescing or drops cannot make delivery gaps. Delivery is at least once; clients deduplicate by event/message ID.

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
