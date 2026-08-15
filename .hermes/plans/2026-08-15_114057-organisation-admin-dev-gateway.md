# Organisation Administration and Dev Gateway Recovery Plan

> **For Hermes:** Implement task-by-task with strict RED → GREEN → refactor cycles.

**Goal:** Add the minimum organisation administration and authorisation needed for K-Mainstay’s MVP, deploy it safely, and finish the separate K-Mainstay Dev Hermes gateway experiment.

**Architecture:** Keep users unified as human/bot principals. Add `role` and normalised membership name to `organisation_memberships`, making roles and name uniqueness organisation-scoped without adding a general permission framework. Keep the Hermes connection as a user-installed `kmainstay_dev` platform plugin using the existing HTTP/WebSocket API.

**Tech stack:** Go `net/http`, `database/sql`, SQLite migrations, Vue 3/TypeScript/Vitest, Hermes platform plugin, systemd/Caddy VPS deployment.

---

## Confirmed scope

- Membership roles are exactly `admin` and `member`.
- The bootstrapped human is an admin.
- Bots are members and cannot administer the organisation.
- All organisation members can view the organisation user list and roles.
- Only admins can create bots and rotate/revoke bot API keys.
- Display names are unique within an organisation after trimming and case-folding.
- The organisation view lists people/bots, roles, and admin-only bot key actions.
- Rotated keys remain copy-once.
- The existing dev API key connects Hermes only to the disposable K-Mainstay dev server.

## Explicit non-goals

- Human invitations or account creation.
- Custom roles or permission editor.
- Role mutation UI.
- Multiple-organisation administration UI.
- Attachments, typing indicators, reactions, threads, or general gateway routing.
- Treating the `sslip.io` deployment as durable production.

## Task 1: Schema migration and bootstrap role

**Files:**
- Create: `internal/database/migrations/002_membership_roles.sql`
- Modify: `internal/database/database.go`
- Modify: `internal/database/database_test.go`
- Modify: `internal/app/bootstrap.go`
- Test: `internal/app/bootstrap_test.go`

1. Write failing migration tests proving existing memberships gain roles and normalised names, and duplicate case-insensitive names are rejected within one organisation.
2. Run targeted tests and confirm RED.
3. Add migration 2 by rebuilding `organisation_memberships` with `role`, `name_normalized`, and a unique organisation/name constraint.
4. Make bootstrap insert Michael as `admin` with a normalised name.
5. Run targeted tests and full `go test ./...`.

## Task 2: Admin-only API behaviour

**Files:**
- Modify: `internal/httpapi/integration_test.go`
- Modify: `internal/httpapi/router.go`
- Modify: `docs/api.md`

1. Write failing integration tests proving:
   - users include role;
   - same-name bot creation returns conflict;
   - members and bots cannot create bots;
   - members cannot rotate/revoke bot keys;
   - admins retain those actions.
2. Run tests and confirm RED.
3. Add a narrow organisation-role lookup and admin guard.
4. Return role from the users endpoint and map uniqueness violations to `409`.
5. Run targeted and full Go tests, vet and race tests.

## Task 3: Organisation administration UI

**Files:**
- Modify: `src/types.ts`
- Modify: `src/App.test.ts`
- Modify: `src/App.vue`
- Modify: `src/style.css`

1. Write failing Vitest cases for opening the organisation user list, role badges, hidden admin actions for non-admins, and rotate/revoke copy-once behaviour.
2. Run the focused tests and confirm RED.
3. Add a single organisation modal/panel and minimal API-key action controls.
4. Keep bot creation visible only to admins; reuse the existing copy-once key presentation.
5. Run Vitest, TypeScript checking, production build and npm high-severity audit.

## Task 4: Full product verification and deployment

**Files:**
- Embedded assets under `internal/webui/dist/`
- Existing `deploy/` files unchanged unless verification exposes a real need.

1. Build the frontend and copy verified assets into the embedded distribution.
2. Run Go format, vet, race tests, frontend tests/typecheck/build/audit and the local reference exchange.
3. Build stripped Linux AMD64 binaries.
4. Upload binaries only, run the migration through normal application startup, restart `kmainstay`, and verify systemd/Caddy health.
5. Verify public login, organisation view, Michael admin role, duplicate-name rejection, bot-key rotate/revoke, realtime chat and persistence.

## Task 5: Finish K-Mainstay Dev Hermes gateway

**Files:**
- `~/.hermes/plugins/platforms/kmainstay_dev/{__init__.py,adapter.py,plugin.yaml,test_adapter.py}`
- `~/.hermes/config.yaml` via Hermes CLI only.
- Secret remains only in `~/.hermes/.env`.

1. Re-run adapter unit tests and `hermes plugins doctor`.
2. Restart the gateway from an external process boundary, not from inside the gateway turn.
3. Verify cursor creation, a live WebSocket and clean logs.
4. Send one uniquely tagged human message through the dev chat and verify Hermes posts a reply.
5. Restart once more and verify cursor/session recovery without replay loops.

## Task 6: Save reusable Hermes gateway procedure

**Files:**
- Create through `skill_manage`: internal skill covering user platform plugin layout, secret/config separation, cursor handling, test method, restart boundary, and verification.

Do not include the API key, live password, session cookie or other credentials.

## Task 7: Review, commit and push

1. Run an independent security/correctness review focused on migration safety, authorisation and gateway secret handling.
2. Fix substantive findings test-first.
3. Commit and push K-Mainstay changes to its repository.
4. Stage only the relevant Hermes plugin/config/skill changes, preserving unrelated dirty files; commit and push the Hermes configuration repository.
5. Verify both local heads equal their remotes and report exact live state.

## Material risks

- Rebuilding a SQLite membership table requires foreign-key-safe migration ordering and backup-aware deployment.
- Name normalisation uses SQLite/Python/Go case behaviour; the MVP guarantee is reliable for ordinary Latin names, not full Unicode confusable handling.
- Rotating the dev bot key will disconnect the adapter on its next reconnect; the runtime secret must be updated together.
- A gateway process cannot safely restart itself during an active gateway turn; restart must cross an external process boundary.
