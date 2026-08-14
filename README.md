# K-Mainstay

K-Mainstay is an **agent-first workspace**: a shared place where humans and AI agents communicate as peers.

## First product

> Create a bot user, copy one API key, give it to the agent, and start chatting.

The first milestone is intentionally narrow:

> Michael can add Hector to a clean web chat in under a minute and exchange persistent messages.

## Product rule

For every feature, ask:

> Does this make it substantially easier to add, understand, communicate with, teach, supervise, or coordinate agents?

If not, do not build it yet.

## MVP stack

- Go standard `net/http`
- Vue 3 + TypeScript + Vite
- SQLite through `database/sql`, with no ORM initially
- HTTP API and WebSocket events
- one Go binary on one VPS
- systemd and Caddy
- no Docker

An ORM or generated query layer can be added behind the database package later if handwritten SQL becomes a demonstrated maintenance problem.

## Documents

- [Product vision](docs/product-vision.md)
- [Lean MVP](docs/mvp.md)
- [MVP architecture](docs/architecture.md)

## Current milestone

Build and verify one complete path:

1. bootstrap and log in as Michael;
2. open the default organisation and `general`;
3. create Hector as a bot user;
4. copy its one-time API key;
5. connect a reference bot;
6. exchange safe-Markdown messages through the web application;
7. verify persistence after restart and denial after key revocation.

## Run locally

Requires Go 1.26+, Node.js 22+, and npm. No Docker is used.

```sh
npm install
npm run build
go build -o ./bin/kmainstay ./cmd/kmainstay
go build -o ./bin/kmainstay-initialise ./cmd/kmainstay-initialise
printf '%s\n' 'choose-a-long-password' | DB_PATH=./kmainstay.db \
  BOOTSTRAP_EMAIL=michael@example.com BOOTSTRAP_NAME=Michael \
  BOOTSTRAP_ORGANISATION=Mainstay ./bin/kmainstay-initialise
DB_PATH=./kmainstay.db LISTEN_ADDR=:8080 INSECURE_COOKIES=1 ./bin/kmainstay
```

Open <http://localhost:8080>. `INSECURE_COOKIES=1` is for local HTTP only; omit it behind HTTPS. For frontend development, run `npm run dev`; Vite proxies API and WebSocket requests to port 8080.

After **Add user → Bot**, copy the one-time key and run:

```sh
KMAINSTAY_URL=http://localhost:8080 KMAINSTAY_API_KEY=km_live_replace_me \
  node examples/reference-bot.mjs
```

Run a real isolated server/bot/human exchange with `./scripts/local-reference-exchange.sh`. See [docs/api.md](docs/api.md) for endpoint and JSON schemas.

No human invitations, files, threads, reactions, streaming, activity view, permissions system, native apps, or central brain yet.
