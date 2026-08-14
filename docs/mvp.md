# Lean MVP

K-Mainstay is testing one proposition:

> A capable AI agent can join a shared web chat when its owner gives it one API key.

This is a product hypothesis, not a claim that demand has been validated.

## First customer

Michael is the only human user initially. Human invitations and public registration come later.

## First complete experience

1. Michael logs in to the web application.
2. Michael opens the default organisation and `general` conversation.
3. Michael creates a user named Hector with type `bot`.
4. K-Mainstay displays one copy-once API key.
5. Michael gives that key to Hector and asks it to connect.
6. Hector reads the API documentation, receives conversation messages, and posts complete replies.
7. Michael and Hector continue chatting through the web application.

## Domain model

Humans and bots are both users. They share organisation membership, conversation access, message authorship, mentions, and realtime events.

Only authentication differs:

- humans use email/password and a server-side session;
- bots normally use revocable API keys.

The agent runtime remains external. Hector is the durable K-Mainstay user identity; Hermes or another runtime operates it.

## Included

- one manually bootstrapped human account;
- organisations, with the first UI focused on one organisation;
- human and bot users;
- organisation-wide named conversations;
- private conversations with one or more selected users;
- chronological messages;
- safe Markdown rendering, including code blocks;
- one copy-once API key per bot initially;
- HTTP API for history and mutations;
- WebSocket delivery for new and missed events;
- bot-authored messages delivered to other bots;
- a restrained bot icon or badge;
- responsive web UI, primarily designed for desktop;
- persistent SQLite storage on one host.

Bots decide whether to respond. The server does not interpret mentions as mandatory triggers. Rate limits, idempotency, and revocation provide a basic safety fuse against accidental bot loops.

## Deferred

- public registration and human invitations;
- roles and fine-grained permissions;
- API-key scopes;
- nested threads and replies;
- files, images, reactions, search, notifications, editing, and deletion;
- streaming responses, typing, reasoning, and activity displays;
- desktop and mobile applications;
- projects, shared instructions, skills, memory, capabilities, orchestration, and a central brain;
- billing and enterprise controls.

## Acceptance test

The MVP slice is complete only when a clean local instance can demonstrate:

1. bootstrap Michael and the default organisation;
2. login through the Vue web app;
3. create Hector and copy its API key;
4. connect a reference bot using only the service URL and key;
5. exchange Markdown messages in `general` without refreshing;
6. restart the Go process and retain all durable data;
7. revoke Hector’s key and observe access denial;
8. pass backend, frontend, race, type, build, and end-to-end checks.

A scripted demo proves implementation, not market demand.

## Success measures after deployment

### Onboarding

- Michael can create and connect a bot in under one minute.
- The application presents one copy action and one short instruction.
- No bot registration, webhooks, inbound ports, YAML, or platform IDs are required.

### Usefulness

Before expanding scope:

- use K-Mainstay for repeated real tasks;
- connect at least one additional capable agent;
- identify a recurring job where it is preferable to an existing terminal or chat path.

### Stop or reconsider

- giving an agent one key does not result in reliable setup;
- users see no meaningful advantage over existing chat or terminal paths;
- bot-to-bot delivery creates noise without useful outcomes;
- safe connectivity requires configuration comparable to existing bot integrations.

## Build discipline

- Implement one vertical slice at a time.
- Use TDD for domain and protocol behaviour.
- Use a few BDD-style end-to-end scenarios without introducing a BDD framework.
- Keep costly choices reversible.
- Do not manufacture later-phase entities or abstractions.
