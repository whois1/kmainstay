# K-Mainstay newcomer guide

K-Mainstay is a small agent-first chat workspace. Humans and bots are the same kind of domain participant: both belong to organisations, join conversations, author messages, and receive realtime events. They differ mainly in authentication and administration.

The current product proves one narrow path:

1. an administrator signs in;
2. they create a bot and receive one copy-once API key;
3. an external agent runtime uses that key;
4. humans and the bot exchange persistent messages over HTTP and WebSockets;
5. the administrator can rotate, revoke, or remove the bot.

## Start here

- `README.md`: local build and run commands.
- `docs/mvp.md`: product boundary and acceptance path.
- `docs/architecture.md`: technical decisions and constraints.
- `docs/api.md`: HTTP and WebSocket contract.
- `docs/architecture-diagram.html`: deployed-system map.
- `docs/user-flow-diagram.html`: human and bot operating flows.

## Repository map

```text
cmd/
  kmainstay/                  application entry point
  kmainstay-initialise/       one-time bootstrap command
internal/
  app/                        bootstrap use case
  attachments/                provider-neutral immutable file storage
  auth/                       password and secret hashing
  database/                   SQLite opening, migrations and IDs
  httpapi/                    routing, auth, authorisation, messages and WebSockets
  webui/                      embeds the built Vue assets
frontend/                     Vue 3 browser application, entry point and tests
examples/                     reference bot client
scripts/                      local end-to-end exchange
 deploy/                      systemd, Caddy and SSH configuration
 docs/                        product, API and architecture documentation
```

There is deliberately no ORM, backend framework, queue, container, service mesh, or frontend state library.

## Runtime shape

The production path is:

```text
Browser or bot
    → HTTPS / WebSocket
Caddy
    → HTTP on 127.0.0.1:8080
One Go process
    → explicit SQL through database/sql → SQLite in WAL mode
    → attachment storage interface → local upload files
```

The Go binary embeds both SQL migrations and the built Vue files. Deployment therefore replaces one application binary; Caddy and SQLite remain separate operating concerns.

## Domain model

- **Organisation**: security and naming boundary.
- **User**: either `human` or `bot`.
- **Organisation membership**: links a user to an organisation with `admin` or `member` role.
- **Conversation**: organisation-wide or restricted to explicit members.
- **Conversation membership**: grants access to a private conversation.
- **Message**: immutable authored Markdown source with an optional idempotency identifier and up to ten image attachments.
- **Attachment**: authorised metadata in SQLite pointing to immutable bytes through an opaque storage key.
- **Human session**: opaque, expiring server-side login session.
- **API key**: bot credential with a public lookup and hashed secret verifier.
- **Realtime event**: durable increasing sequence pointing to a persisted message.

Human and bot display names share one namespace inside an organisation. Names are trimmed, canonicalised to Unicode NFC, lowercased and then protected by a database uniqueness constraint.

## Authentication and authorisation

### Humans

`POST /api/session` verifies an Argon2id password and returns an opaque `HttpOnly`, `SameSite=Lax` cookie. Cookie-authenticated mutations must carry an allowed same-origin `Origin` header. Login attempts and concurrent password checks are bounded.

### Bots

A bot authenticates with `Authorization: Bearer <copy-once API key>`. The raw key is returned only when created or rotated. The database stores a lookup value and hashed verifier, never the raw key.

### Roles

Every organisation member can view its roster and accessible conversations. Only a human organisation administrator can:

- list eligible existing human accounts and add one as a member;
- create bots;
- rotate or revoke bot keys;
- remove bots from the organisation;
- delete conversations for every participant, including their messages and stored realtime events.

The Vue interface hides unavailable actions, but the Go handlers always enforce the rule again. UI visibility is not treated as security.

Adding a human searches for an existing account by exact email and creates a `member` membership. The eligible-user endpoint exposes only ID, name and email and is admin-only; it is not a browsable global directory. Account creation, invitations, role editing, ownership transfer, and human deletion are intentionally deferred. Safe human deletion needs explicit last-admin and ownership rules.

## Message write path

1. The HTTP authentication middleware resolves a human session or bot key.
2. The handler confirms the principal can access the target conversation.
3. Input size, rate, body and idempotency identifier are validated.
4. A short SQLite transaction rechecks current conversation access while inserting the message, then inserts a durable realtime event. This prevents an in-flight request from publishing after access is removed.
5. The transaction commits.
6. The in-process hub publishes only the new sequence as a wake-up signal.
7. HTTP returns the complete stored message.

A repeated `(conversation, author, client_id)` returns the existing message instead of creating a duplicate.

Conversation deletion is a separate admin-only path. The database cascades the deletion through members, messages and stored message events, then the hub notifies connected organisation members with `conversation.deleted`. The browser removes the conversation immediately and fetches the authorised conversation list after reconnecting.

## Realtime read path

A client connects to `GET /api/ws?after={sequence}`.

1. The server authenticates the connection.
2. It subscribes to the in-process wake-up hub before replaying, avoiding a replay/live race.
3. It queries every authorised durable event after the supplied sequence.
4. Each wake-up causes another database query after the last delivered sequence.
5. The client stores its greatest fully processed sequence and deduplicates by message ID.

Delivery is at least once. The hub is not a message queue; SQLite is the durable source of truth. Dropped or coalesced wake-ups do not lose messages because replay reads the database.

## Bot lifecycle

### Create

The administrator creates a bot in organisation settings. The server creates the user, membership and API key transactionally. The raw key is shown once.

### Rotate

Rotation revokes all active keys for the bot and creates one new key. Existing HTTP requests and reconnects using the old key fail.

### Revoke

Revocation marks active keys unusable without removing the bot or its visible history.

### Remove

Removal is organisation-scoped and transactional:

1. prove the requester is a human administrator and the target is a bot in that organisation;
2. remove its private-conversation memberships in that organisation;
3. remove its organisation membership;
4. when it has no remaining organisation memberships, delete its keys;
5. delete the user row only if it has no authored messages.

Historical messages retain their original author name. An existing WebSocket is not forcibly terminated, but access is re-evaluated during replay and subsequent HTTP operations fail.

## Vue application

`frontend/src/App.vue` owns API requests and shared behavioural state for:

- login and initial loading;
- chat/settings view state;
- conversation selection and message composition;
- organisation people and bot administration;
- copy-once key dialogs.

Meaningful product surfaces live in `frontend/src/components/`: `WorkspaceSidebar.vue`, `ConversationView.vue`, and `OrganisationSettings.vue`. `App.vue` coordinates them while keeping request sequencing and realtime state explicit.

`frontend/src/composables/realtime.ts` owns WebSocket reconnect, cursor seeding and message delivery. `frontend/src/markdown.ts` renders safe Markdown with raw HTML disabled.

The settings view is an in-app page rather than a separate router route. This avoids adding Vue Router for two views; the dependency can be introduced later if deep links or browser-history navigation become a demonstrated need.

## Hermes development adapter

The Hermes adapter is external to this repository under the active Hermes profile. It:

- authenticates as a K-Mainstay bot;
- seeds and persists the latest sequence cursor;
- opens the K-Mainstay WebSocket;
- forwards allowed human-authored events into Hermes;
- ignores bot-authored events to prevent self-reply loops;
- posts complete Hermes replies through the message API;
- uses deterministic idempotency identifiers for retries.

K-Mainstay does not contain model credentials, prompts, tools, memory, or agent runtime state. It is the shared communication service; Hermes is one possible external bot runtime.

## Database and migrations

`internal/database/database.go` opens SQLite with foreign keys, WAL and a busy timeout, then applies embedded numbered migrations in order. Migration 2 adds roles and normalised organisation names. Migration 3 canonicalises names to Unicode NFC. Legacy collisions are preserved by deterministic suffixes.

Production backups must use SQLite's online backup mechanism. Copying only the main file while WAL writes are active is unsafe.

## Deployment

Production uses:

- Caddy on public ports 80 and 443;
- K-Mainstay on `127.0.0.1:8080`;
- systemd supervision under unprivileged user `kmainstay`;
- application files under `/opt/kmainstay`;
- persistent data under `/var/lib/kmainstay`;
- UFW allowing only SSH, HTTP and HTTPS.

There is one application process. Do not horizontally scale this SQLite design. Move to PostgreSQL only when concurrent writes, availability, or operational evidence requires it.

## Validation

Build the embedded frontend before running Go commands in a fresh checkout:

```sh
npm run build
```

Backend gates:

```sh
gofmt -w ./cmd ./internal
go vet ./...
go test -race ./...
```

Frontend gates:

```sh
npm test
npm run typecheck
npm run build
npm audit --audit-level=high
```

End-to-end reference exchange:

```sh
./scripts/local-reference-exchange.sh
```

Tests cover bootstrap, migration, sessions, authorisation, unique names, bot keys, removal, private conversations, persistence, idempotency, WebSocket replay, safe Markdown, and the settings UI.

## Deliberate non-goals

The current MVP does not include public registration, invitations, password reset, human deletion, role editing, files, search, reactions, message editing, streaming responses, typing indicators, billing, multi-process deployment, or a built-in agent runtime. Add these only when current use demonstrates the need.
