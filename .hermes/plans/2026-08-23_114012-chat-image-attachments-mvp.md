# Chat Image Attachments MVP Implementation Plan

> **For Hermes:** Implement this plan test-first in small, independently verified changes.

**Goal:** Let humans and bots send and receive one authorised image with a chat message while storing bytes locally today and preserving a clean migration path to S3-compatible object storage later.

**Architecture:** Add structured attachment metadata to SQLite and immutable image bytes behind a narrow storage interface. The first implementation writes opaque storage keys beneath `/var/lib/kmainstay/uploads`; messages and realtime events expose attachment metadata and an authorised API endpoint, never filesystem paths. Keep the existing JSON text-message API intact and accept multipart form data on the same message endpoint when an image is attached.

**Technology:** Go standard library, SQLite, the existing HTTP/WebSocket API, Vue 3 and TypeScript. Add no cloud SDK, image-processing service, queue or frontend state library.

---

## Product boundary

### Included

- One image per message.
- JPEG and PNG only for the first slice.
- Maximum encoded file size: 10 MB.
- Maximum decoded dimensions/pixel count, selected conservatively to reject decompression bombs.
- Optional Markdown caption, including image-only messages.
- Human browser upload through a file picker with preview and removal before sending.
- Bot upload through the same authenticated message endpoint.
- Inline, bounded display in message history and realtime arrivals.
- Authorised image retrieval for humans and bots.
- Image deletion when its conversation is deleted.
- Deployment configuration, backup documentation and restore verification covering both SQLite and uploaded images.

### Explicitly excluded

- Multiple attachments per message in the user interface.
- Video, audio, arbitrary files, SVG, animated GIF and WebP.
- Drag-and-drop, clipboard paste, editing or image annotations.
- Thumbnails, transcoding, EXIF removal, optical character recognition or content moderation.
- Direct browser-to-object-storage uploads, signed object URLs, CDN delivery or an S3 SDK.
- Per-user storage billing, quotas or automatic deletion of valid chat history.

The schema may use a one-to-many message-to-attachment relationship because that costs little and avoids a later destructive migration, but the API must enforce one image for this MVP.

## Retention decision

Do **not** automatically delete conversations after 30 days of inactivity.

Thirty days is too aggressive for organisational knowledge, and “unused” is ambiguous: a conversation can be valuable precisely because nobody needs to revisit it often. Storage currently costs much less than accidental loss. Hector's Hermes session history is not a safe replacement for K-Mainstay because:

- Hector is only one participant and is not the authority for every organisation conversation;
- not every message necessarily wakes or is processed into a Hermes session;
- attachments and access-control context may not be retained with the transcript;
- Hermes sessions can be archived, pruned, reconfigured or deleted independently;
- restoring one bot's local transcript would not reconstruct K-Mainstay's memberships, read positions, event sequences or complete shared history.

K-Mainstay remains the system of record for K-Mainstay messages. Hermes may retain a useful secondary transcript, not a guaranteed backup.

Automatic cleanup in this feature should be limited to:

- failed multipart request temporary files;
- incomplete upload staging files older than 24 hours;
- attachment files belonging to explicitly deleted conversations;
- test artefacts.

If real storage pressure appears later, first add storage reporting and administrator-visible warnings. Only then consider an explicit organisation retention policy such as `never`, `90 days`, `365 days` or a user-selected date, with export, warning and a recovery grace period. Do not build that policy now.

---

## Proposed data model

Create migration `internal/database/migrations/006_message_attachments.sql` with an `attachments` table containing:

- `id`: application-generated `att_...` identifier;
- `message_id`: foreign key to `messages`, cascading on message deletion;
- `storage_key`: unique opaque key, not a local path or public URL;
- `media_type`: constrained to `image/jpeg` or `image/png`;
- `byte_size`: positive integer bounded by the application limit;
- `width` and `height`: validated positive dimensions;
- `original_filename`: display/download metadata only;
- `sha256`: integrity value for backup checks and future storage migration;
- `created_at`: portable UTC timestamp.

Add an index on `attachments(message_id, created_at, id)`.

Permit an empty `messages.body` only when the application inserts an attachment in the same transaction. Because SQLite cannot express that cross-table rule as a simple `CHECK`, the API must reject a message with neither trimmed text nor an attachment. Migration 6 must safely rebuild the populated `messages` table to relax its existing `length(body) BETWEEN 1 AND 20000` constraint while preserving IDs, client idempotency, realtime-event references and mention references.

Migration tests must exercise a populated version-5 database, run migration 6, execute `PRAGMA foreign_key_check`, verify existing messages/events/mentions remain intact and prove empty unattached messages are still rejected by the API.

---

## Storage boundary

Create `internal/attachments/` containing a small storage contract and filesystem implementation. It should support only the operations needed now: save immutable bytes under a caller-supplied opaque key, open bytes for authorised delivery and delete bytes. Do not expose filesystem paths through the interface or database.

Filesystem rules:

- Root supplied through `ATTACHMENT_PATH`.
- Production value: `/var/lib/kmainstay/uploads`.
- Local default derived predictably beside the configured database, documented explicitly.
- Create files with restrictive permissions and directories inaccessible to unrelated users.
- Write to a staging file in the same filesystem, sync/close it, then atomically rename it to the final storage key.
- Never use the original filename as a path.
- Refuse overwrite when a generated storage key unexpectedly exists.
- Remove request staging files on every error path.
- Clean staging files older than 24 hours at application startup; do not sweep valid final files merely because metadata lookup temporarily fails.

Inject the storage implementation through `httpapi.Dependencies`, alongside the database and embedded assets. This is the only abstraction added for future object storage.

A future R2/S3 implementation should preserve the same storage keys and metadata. Migration would copy each object, verify `byte_size` and `sha256`, then switch configuration. No message rows, attachment IDs, clients or URLs need to change.

---

## API contract

### Message creation

Keep the existing JSON request unchanged:

- `Content-Type: application/json`
- `{ "body": "...", "client_id": "..." }`

Add multipart handling on the same endpoint:

- `POST /api/conversations/{conversation}/messages`
- `Content-Type: multipart/form-data`
- text fields: `body`, `client_id`
- one file field: `image`

Validation order:

1. Authenticate and authorise conversation access.
2. Apply the existing message rate limit.
3. Bound the complete request before parsing multipart data.
4. Accept one file only.
5. Read no more than 10 MB plus a small multipart allowance.
6. Inspect the byte signature rather than trusting the submitted content type or extension.
7. Decode image configuration with Go's JPEG/PNG decoders and reject invalid or excessive dimensions/pixel count.
8. Bound and sanitise the original filename as metadata.
9. Require non-empty body or a valid image.
10. Preserve existing client-ID idempotency.

The storage/database sequence must avoid an event that references unavailable bytes:

1. validate and hash the upload;
2. persist the immutable object atomically;
3. insert the message, attachment and realtime event in one SQLite transaction;
4. if the transaction fails or resolves to an existing idempotent message, delete the newly written unused object;
5. publish the realtime wake-up only after commit.

### Message representation

Extend history, create-message responses and `message.created` events with:

- `attachments: []` for text-only messages;
- one attachment object for image messages;
- attachment fields: ID, media type, byte size, width, height, original filename and stable API content URL.

Do not include image bytes, Base64, filesystem paths or storage keys.

### Image retrieval

Add:

- `GET /api/attachments/{attachment}/content`

The handler must:

- authenticate cookies or bot bearer keys through the existing middleware;
- join attachment → message → conversation and re-evaluate current conversation access;
- return `404` or the project's established non-disclosing response when access is absent;
- stream bytes rather than load the complete image into memory;
- set the validated `Content-Type`, `Content-Length`, `ETag` from the checksum, `X-Content-Type-Options: nosniff`, private cache controls and a safe `Content-Disposition`;
- never redirect to or reveal a local storage path.

This endpoint remains stable after moving bytes to R2/S3. A future implementation may stream from object storage or return short-lived signed delivery URLs behind the same authorisation boundary.

---

## Implementation tasks

### Task 1: Add the attachment schema through a tested migration

**Files:**

- Create `internal/database/migrations/006_message_attachments.sql`
- Modify `internal/database/database.go`
- Modify `internal/database/database_test.go`

**Steps:**

1. Write failing fresh-database and populated-version-5 migration tests.
2. Verify the tests fail before migration 6 exists.
3. Add the embedded migration and version registration.
4. Rebuild `messages` safely, create `attachments`, restore indexes and preserve dependent rows.
5. Run the focused database tests.
6. Run `go test ./internal/database` and verify `PRAGMA foreign_key_check` returns no rows.

### Task 2: Add and verify local attachment storage

**Files:**

- Create `internal/attachments/store.go`
- Create `internal/attachments/filesystem.go`
- Create `internal/attachments/filesystem_test.go`

**Steps:**

1. Test atomic save/open/delete, restrictive permissions, overwrite refusal, staging cleanup and cancellation/error cleanup.
2. Implement the smallest interface satisfying those tests.
3. Use temporary directories only in tests.
4. Run `go test ./internal/attachments`.

### Task 3: Wire storage configuration into the application

**Files:**

- Modify `cmd/kmainstay/main.go`
- Modify `internal/httpapi/router.go`
- Modify `internal/httpapi/router_test.go`
- Modify `README.md`

**Steps:**

1. Add failing tests proving API construction requires usable attachment storage when attachment routes are enabled.
2. Read `ATTACHMENT_PATH`, create the filesystem store and inject it through `httpapi.Dependencies`.
3. Preserve existing tests that construct the router without a database or with test dependencies.
4. Document the local and production paths.
5. Run focused router and command build tests.

### Task 4: Add validated multipart message creation

**Files:**

- Create `internal/httpapi/attachments.go`
- Modify `internal/httpapi/router.go`
- Modify `internal/httpapi/integration_test.go`
- Modify `internal/httpapi/security_test.go`

**Test cases:**

- authenticated human uploads valid JPEG with caption;
- authenticated bot uploads valid PNG without caption;
- existing JSON text message behaviour and idempotency remain unchanged;
- empty body without image is rejected;
- second image is rejected;
- wrong signature, SVG, oversized bytes and excessive dimensions are rejected;
- malformed multipart request is bounded and cleaned up;
- inaccessible conversation is denied before persistence;
- duplicate `client_id` returns the existing message and leaves no duplicate file;
- database failure removes the new object;
- response contains metadata but no storage key or bytes.

Run each focused test red, implement minimally, then run `go test ./internal/httpapi`.

### Task 5: Return attachments in history and realtime events

**Files:**

- Modify `internal/httpapi/router.go`
- Modify `internal/httpapi/integration_test.go`
- Modify `internal/httpapi/bot_delivery_test.go`
- Modify `internal/httpapi/hub_test.go`

**Steps:**

1. Add attachment response types and a helper that loads attachments for returned messages.
2. Ensure text messages serialise `attachments: []`, not `null`.
3. Prove bounded history, after-sequence replay, initial create response, idempotent retry and live WebSocket delivery all return identical attachment metadata.
4. Confirm attachment bytes never enter WebSocket payloads.
5. Run all HTTP API tests.

### Task 6: Add authorised image delivery

**Files:**

- Modify `internal/httpapi/attachments.go`
- Modify `internal/httpapi/router.go`
- Modify `internal/httpapi/security_test.go`
- Modify `internal/httpapi/integration_test.go`

**Test cases:**

- participant cookie can retrieve bytes;
- authorised bot bearer key can retrieve bytes;
- non-member and removed bot cannot retrieve bytes;
- another organisation cannot infer or retrieve the attachment;
- revoked API key fails;
- response headers are safe and complete;
- nonexistent metadata and missing underlying object fail without path disclosure.

Run focused tests, then `go test ./internal/httpapi`.

### Task 7: Handle conversation deletion and file cleanup

**Files:**

- Modify `internal/httpapi/router.go`
- Modify `internal/httpapi/integration_test.go`

**Steps:**

1. Test that conversation deletion removes attachment metadata through cascade and removes associated files.
2. Collect authorised conversation storage keys before the database delete, commit the existing deletion, then delete the now-unreachable objects.
3. Treat post-commit file deletion failure as an operational cleanup fault, not a reason to resurrect the deleted conversation; log it without exposing paths to clients.
4. Verify unrelated conversation files remain.

Do not add automatic inactive-conversation deletion.

### Task 8: Add browser selection, preview, sending and rendering

**Files:**

- Modify `frontend/src/types.ts`
- Modify `frontend/src/App.vue`
- Modify `frontend/src/App.test.ts`
- Modify `frontend/src/components/ConversationView.vue`
- Modify `frontend/src/components/ConversationView.test.ts`
- Modify `frontend/src/style.css`

**Behaviour:**

- Add an accessible image-picker button beside the composer.
- Show selected filename, bounded preview and remove control.
- Enable Send when either trimmed text or a valid image is present.
- Submit JSON for text-only messages and multipart form data when an image exists.
- Preserve both caption and selected file after a failed send.
- Clear the draft and revoke preview object URLs after success, removal, conversation switch or component disposal.
- Render image-only and captioned messages without broken empty Markdown containers.
- Use lazy-loaded images, bounded dimensions and useful alternative text based on safe filename metadata.
- Preserve unread-divider, jump-to-latest and realtime merge behaviour.

Tests must cover selection, removal, invalid client-side size/type feedback, multipart fields, retry preservation, successful cleanup, image-only messages, realtime image arrival and history rendering.

Run `npm run test`, `npm run typecheck` and `npm run build`.

### Task 9: Update the reference bot and API documentation

**Files:**

- Modify `examples/reference-bot.mjs`
- Modify `examples/reference-bot.test.mjs`
- Modify `docs/api.md`
- Modify `docs/architecture.md`
- Modify `docs/newcomer-guide.md`
- Modify `README.md`

**Steps:**

1. Document JSON and multipart variants without breaking existing bot clients.
2. Document attachment response fields and authorised download requirements.
3. Add a small reference-bot example/test for uploading and downloading an image using bearer authentication.
4. Record local filesystem storage as the MVP decision and S3-compatible storage as a later replacement behind the storage boundary.
5. State the retention decision: explicit conversation deletion only; temporary/staging cleanup is automatic.

### Task 10: Make deployment and backup honest

**Files:**

- Modify `deploy/kmainstay.service`
- Modify `deploy/install-release.sh` if its rollback snapshot must cover new mutable state
- Modify `deploy/README.md`
- Add or modify the smallest backup/restore script justified by the existing deployment workflow

**Steps:**

1. Provision `/var/lib/kmainstay/uploads` with `kmainstay:kmainstay` ownership and restrictive permissions.
2. Set `ATTACHMENT_PATH=/var/lib/kmainstay/uploads` in systemd.
3. Replace the now-false statement that SQLite is the only application state.
4. Ensure deployment rollback cannot discard or overwrite uploaded files.
5. Define a consistent pilot backup: briefly stop the single process, snapshot SQLite plus uploads together, restart, and verify health. Prefer a short honest maintenance window over an unproven online multi-resource snapshot.
6. Restore a test backup into an isolated directory and verify message metadata, checksum, image bytes and authorised retrieval.
7. Do not add a cloud backup service as part of this image slice, but retain the existing warning that no off-server backup destination exists.

### Task 11: End-to-end acceptance and full verification

**Acceptance path:**

1. Start from a clean database and empty upload directory.
2. Sign in as Michael.
3. Open a private Michael/Hector conversation.
4. Select a valid image, preview it and send it without a caption.
5. Observe it inline after the HTTP response and after page reload.
6. Confirm Hector receives attachment metadata over WebSocket and downloads the exact bytes with its bearer key.
7. Send a captioned image from the reference bot and observe it live in the browser.
8. Restart K-Mainstay and verify both images remain available.
9. Revoke the bot key and verify subsequent image retrieval is denied.
10. Delete the conversation and verify metadata and image files are removed while unrelated data remains.
11. Perform the documented backup/restore check.

**Commands:**

- `gofmt` on changed Go files
- `go test ./...`
- `npm run test`
- `npm run typecheck`
- `npm run build`
- `./scripts/local-reference-exchange.sh`, extended only as needed to cover attachment exchange
- `git status --short`
- inspect the final diff for secrets, generated temporary images and accidental unrelated changes

Do not deploy until all checks pass and the production upload directory plus backup behaviour are verified.

---

## Risks and trade-offs

- **Local disk failure:** uploads and SQLite share one VPS failure domain. This is acceptable only for the pilot and makes off-server backup the next operational priority, not object-storage integration itself.
- **Filesystem/database atomicity:** no transaction spans both. Immutable atomic writes, compensating deletion and explicit failure tests limit inconsistent states.
- **Deletion failure:** database deletion can succeed while a now-unreachable file remains. This wastes space but does not expose data through the authorised API. Log and reconcile operationally rather than failing the user's completed deletion.
- **Image payload abuse:** byte limits alone are insufficient. Decode dimensions before acceptance and prohibit active formats such as SVG.
- **No EXIF removal:** JPEG metadata may contain location/device information. State this pilot limitation; add server-side re-encoding only if users need privacy sanitisation.
- **Backup growth:** local uploads make SQLite-only backups incomplete. Production documentation and restore proof are part of this feature, not follow-up polish.
- **Premature scalability:** direct uploads, cloud SDKs, queues and CDNs add failure modes without current evidence. The opaque storage key and injected store preserve the migration seam at far lower cost.

## Stop/reconsider conditions

Reconsider local filesystem storage when any of these becomes true:

- attachment data materially lengthens or destabilises backup/restore;
- upload/download traffic competes with chat responsiveness;
- disk capacity approaches the operational warning threshold;
- more than one application host is required;
- off-site durability requirements cannot be met simply;
- image transformation or scanning needs justify asynchronous workers.

At that point, implement an R2/S3 store, migrate and checksum existing immutable objects, and leave the message/API model unchanged.