# Organisation Settings Page Implementation Plan

> **For Hermes:** Implement this plan task-by-task using strict RED → GREEN TDD.

**Goal:** Replace the organisation modal with a clear full-content settings page that separates people and bots, centralises bot administration, and lets admins safely remove bots without deleting message history.

**Architecture:** Keep the dependency-free Vue SPA and switch between chat and settings with local view state; do not add Vue Router. Add one organisation-scoped bot-removal HTTP endpoint. Removal revokes keys and access in a transaction, preserves authored messages, and deletes the underlying bot identity only when no historical or cross-organisation references remain.

**Tech stack:** Vue 3, TypeScript, Vitest, Go `net/http`, `database/sql`, SQLite.

## Scope

- Settings cog opens a full content page, not a modal.
- Page has separate People and Bots sections.
- Existing humans are visible with roles.
- Admins can add bots, rotate/revoke keys, and remove bots.
- Members can view settings but not mutation controls.
- Bot removal preserves historical messages.
- Admins can add an eligible existing human as a `member`; account creation, invitations, role editing, ownership transfer, and human deletion are deferred.
- No router, state library, schema migration, or new dependency.

## Task 1: Bot removal API

**Files:**
- Modify: `internal/httpapi/integration_test.go`
- Modify: `internal/httpapi/router.go`
- Modify: `docs/api.md`

1. Add failing integration coverage for an admin removing a bot, a member and bot being denied, old keys becoming unauthorized, membership/conversation access disappearing, and authored messages retaining attribution.
2. Run the focused test and confirm RED.
3. Add `DELETE /api/organisations/{organisation}/bots/{bot}`.
4. In one transaction, prove the requesting human is an admin and the target is a bot in that organisation; delete API keys, organisation conversation memberships, and the organisation membership. Delete the user only when no memberships and no messages remain.
5. Return 204; use 403 for unauthorized/out-of-scope targets.
6. Run focused and full Go tests.

## Task 2: Full settings page

**Files:**
- Modify: `src/App.test.ts`
- Modify: `src/App.vue`
- Modify: `src/style.css`
- Modify: `src/types.ts` only if the API shape requires it

1. Add failing tests proving the cog switches the content area to settings, People and Bots are separate, Back returns to chat, admin bot actions exist, members lack them, and successful removal updates the bot list.
2. Run focused Vitest and confirm RED.
3. Replace modal state with `chat | settings` view state.
4. Render a settings header, People section, Bots section, Add bot action, key actions, and Remove bot action.
5. Keep the copy-once key modal for creation/rotation.
6. Use explicit confirmation text in the page rather than a native browser dialog; removal is one reversible-at-data-history level action but access revocation is immediate.
7. Add responsive styles without changing the sidebar or chat architecture.
8. Run frontend tests, type checking, build, and audit.

## Task 3: Documentation and diagrams

**Files:**
- Modify: `docs/architecture.md`
- Modify: `docs/api.md`
- Create: `docs/newcomer-guide.md`
- Create: `docs/architecture-diagram.html`
- Create: `docs/user-flow-diagram.html`

1. Update the architecture contract for settings and safe bot removal.
2. Write a newcomer guide covering startup, request flow, persistence, auth, realtime delivery, bot gateway, deployment, testing, and intentional exclusions.
3. Create self-contained architecture and user-flow HTML/SVG diagrams with no JavaScript or runtime dependencies.
4. Verify HTML structure and links.

## Task 4: Validation, review and deployment

1. Run `gofmt`, `go vet ./...`, `go test -race ./...`.
2. Run `npm test`, `npm run typecheck`, `npm run build`, and `npm audit --audit-level=high`.
3. Run `git diff --check`.
4. Review authorization, transactional deletion semantics, UI access controls, and documentation accuracy.
5. Commit and push.
6. Build a stripped static binary, deploy atomically with rollback, and verify service health and public asset hashes.
7. Verify the running API and settings page without deleting the connected production Hector bot.

## Material risks

- Removing the currently connected Hermes bot would intentionally disconnect the dev gateway. Production verification must use tests and non-destructive public checks, not remove Hector.
- Existing authenticated WebSockets are not forcibly closed, but subsequent event replay and HTTP access are denied after membership removal. This matches current revocation semantics.
- Human deletion requires last-admin and ownership-transfer rules and is explicitly not included.
