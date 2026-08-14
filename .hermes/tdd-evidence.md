# TDD evidence

Commands are appended in execution order. Output descriptions report what the command actually showed.

## 2026-08-14 frontend behaviours — RED

Command: `npm install && npm test`

Result: `npm install` completed with 0 vulnerabilities. Vitest exited 1: all three suites failed because the production modules (`App.vue`, `markdown.ts`, and `composables/realtime.ts`) did not exist. This is the expected initial RED for login/workspace/composer/bot/private-conversation, safe Markdown, and realtime replay/deduplication behaviours.

## Frontend behaviours — first GREEN attempt

Command: `npm test`

Result: Vitest exited 1 with 3 passing and 3 failing tests. Realtime, login, and bot-key behaviours passed. The remaining failures exposed overly broad/brittle assertions: unsafe URL source remained inert text (no unsafe `href`), rendered message text also included avatars/times, and loading the newly created conversation legitimately followed its POST. Assertions were narrowed to the actual security and behavior boundaries before rerunning.

## Frontend behaviours — GREEN

Command: `npm test && npm run typecheck`

Result: exited 0. Vitest passed 6/6 tests across 3 files; `vue-tsc --noEmit` passed.

Command: `npm run build`

Result: exited 0. Vite produced `internal/webui/dist/index.html` and hashed CSS/JS assets.

## SPA serving — RED

Command: `go test ./internal/httpapi`

Result: exited 1 at compile time because `httpapi.Dependencies` had no `Assets` field. The new test therefore demonstrated the missing SPA packaging seam before implementation.

## SPA serving — GREEN

Command: `gofmt -w internal/httpapi/router.go internal/httpapi/router_test.go internal/webui/webui.go cmd/kmainstay/main.go && go test ./internal/httpapi ./internal/webui`

Result: exited 0. The HTTP API package passed and the embedded web UI package compiled.

## Reference bot — RED

Command: `node --test examples/reference-bot.test.mjs`

Result: exited 1 with `ERR_MODULE_NOT_FOUND` for `examples/reference-bot.mjs`, proving the receive-and-reply behavior was absent before implementation.

## Reference bot — GREEN

Command: `npm install && node --test examples/reference-bot.test.mjs`

Result: exited 0; npm reported 0 vulnerabilities and the Node test passed 1/1.

## Real local exchange

Command: `chmod +x scripts/local-reference-exchange.sh && ./scripts/local-reference-exchange.sh`

Result: exited 0 with `reference exchange passed: Michael -> Hector -> Michael`. The script built and ran the production Go server, bootstrapped a human, created a bot/key through HTTP, ran the Node WebSocket bot, posted a human message, and observed the complete bot reply in HTTP history.

## Final verification — first attempt

Command: `npm test && npm run typecheck && npm run build && gofmt -w $(rg --files -g '*.go') && go vet ./... && go test -race ./... && go build -o /tmp/kmainstay-production ./cmd/kmainstay && ./scripts/local-reference-exchange.sh`

Result: exited 1 at `npm test`. Vitest passed all 6 frontend tests but also discovered the Node `node:test` file and reported “No test suite found”; chained later checks did not run. `examples/**` was then excluded from Vitest discovery because the package script runs that suite with Node's test runner immediately afterward.

## Final verification — GREEN

Command: `npm test && npm run typecheck && npm run build && gofmt -w $(rg --files -g '*.go') && go vet ./... && go test -race ./... && go build -o /tmp/kmainstay-production ./cmd/kmainstay && ./scripts/local-reference-exchange.sh`

Result: exited 0. Vitest passed 6/6; Node passed 1/1; TypeScript passed; Vite emitted the production bundle; `gofmt` and `go vet ./...` passed; `go test -race ./...` passed; the production Go binary built; and the isolated reference exchange passed.
