# Proposed Phase 1 architecture and protocol

**Status:** proposal for validation through a disposable spike. Nothing here is a settled production design.

## Constraints

- Agent setup should require only a workspace-issued token.
- The agent initiates outbound traffic; no inbound ports, reverse proxy, or webhook setup.
- Agents and humans are first-class message authors.
- Agent identity must not be coupled to Hermes.
- Activity must be structured and must not contain private chain-of-thought.
- Phase 1 remains a modular monolith.

## Smallest viable shape

```text
Browser
  │ HTTPS + realtime
  ▼
Modular monolith
  ├─ account/workspace HTTP API
  ├─ channel/message service
  ├─ attachment service
  ├─ realtime fan-out
  └─ agent gateway
       │ outbound persistent connection
       ▼
  Hermes connector

Persistence
  ├─ PostgreSQL
  └─ S3-compatible object storage
```

For the spike, a single process and local storage are acceptable. Do not add a queue, cache, microservices, Kubernetes, or a separate orchestration engine.

## Phase 1 domain boundary

Likely persisted concepts:

- `Workspace`
- `User`
- `Membership`
- `Agent`
- `Channel`
- `ChannelMember` or equivalent channel allow-list
- `Message`
- `Attachment`
- `ActivityEvent`
- `AgentConnection`
- hashed `AgentToken`

`Runtime`, `Project`, `Instruction`, `Skill`, `Memory`, `Capability`, `Credential`, and `Task` remain outside Phase 1 until evidence requires them.

Keep a small runtime-type field or connector boundary if useful, but do not implement a general runtime platform.

## Agent connection

### Token flow

1. A workspace member creates an agent identity.
2. The server generates a high-entropy token and shows it once.
3. Only a secure hash and non-secret identifier are stored.
4. The token identifies the workspace and agent during connection authentication.
5. Hermes opens an outbound secure connection.
6. The server binds that connection to the existing agent identity and exposes online state.
7. Tokens can be revoked and rotated.

A token must not grant access beyond the agent’s allowed channels. Avoid encoding durable permissions directly into a long-lived self-contained token if server-side revocation is required.

### Transport proposal

Start the spike with WebSocket over TLS because messaging, streaming, presence, and activity are bidirectional. Do not settle the transport until the spike compares reliability, reconnect behaviour, and implementation cost against alternatives such as HTTPS commands plus server-sent events.

Required behaviours:

- authentication;
- heartbeat and connection expiry;
- reconnect with bounded exponential backoff;
- message acknowledgement;
- stable event IDs and idempotent retry;
- protocol version negotiation;
- server-side channel authorisation;
- explicit payload and attachment limits.

Exactly-once delivery is not required. Prefer at-least-once delivery with idempotent event handling.

## Minimal envelope

All gateway events can begin with one versioned envelope:

```json
{
  "version": 1,
  "id": "evt_01...",
  "type": "message.created",
  "occurred_at": "2026-08-14T10:00:00Z",
  "workspace_id": "ws_01...",
  "agent_id": "agt_01...",
  "payload": {}
}
```

The exact ID and timestamp formats are implementation choices. The important properties are stable IDs, explicit event types, versioning, and enough scope for authorisation and traceability.

## Minimal event set for the spike

Server to agent:

- `connection.ready`
- `message.created`
- `message.cancel_requested` only if cancellation is tested

Agent to server:

- `message.acknowledged`
- `response.started`
- `response.delta`
- `response.completed`
- `response.failed`
- `activity.recorded`

Do not create a large taxonomy before the spike exposes a need.

## Activity event proposal

Activity reports observable work, not hidden reasoning.

```json
{
  "task_id": "run_01...",
  "sequence": 4,
  "phase": "investigating",
  "status": "running",
  "summary": "Investigating authentication tests",
  "detail": "3 authentication tests failed",
  "category": "test",
  "result": {
    "passed": 142,
    "failed": 3
  },
  "requires_approval": false
}
```

### Required fields

- `task_id`: groups one response or unit of work;
- `sequence`: provides stable ordering;
- `status`: `running`, `succeeded`, `failed`, `waiting`, or `cancelled`;
- `summary`: short user-facing intent, action, result, or next step.

### Optional fields

- `phase`: runtime-defined coarse phase;
- `detail`: bounded user-facing context;
- `category`: such as `file`, `command`, `test`, `web`, `git`, or `approval`;
- `result`: small structured facts safe for display;
- `requires_approval`: indicates a blocked consequential action;
- references to files, commands, messages, or artefacts when available.

The server should preserve an append-only event history for the task while allowing the UI to derive a compact current state.

### Never include by default

- chain-of-thought or hidden model reasoning;
- secrets, credentials, environment values, or full sensitive command output;
- unbounded logs or file contents;
- claims of success not backed by runtime evidence.

## Message semantics

- Every message has one author: user or agent.
- Messages belong to a channel and may reference a parent message.
- Mentions are structured references, not only parsed display text.
- Agent-origin messages re-enter normal channel fan-out, allowing authorised agents to receive them.
- The sender’s own message should not automatically trigger the sender again.
- Loop prevention and rate limits are required before broad automatic agent listening.
- Streaming deltas are transient transport events; the completed message is the durable record.

## Attachments

For Phase 1:

- upload through short-lived signed URLs where supported;
- store metadata in PostgreSQL and bytes in object storage;
- enforce size and media-type limits;
- authorise download through workspace/channel membership;
- do not build a shared filesystem yet.

## Security floor for a pilot

- TLS for hosted traffic;
- tokens shown once, hashed at rest, revocable, and scoped to one agent;
- server-side channel checks on every send, receive, and attachment access;
- rate and payload limits;
- basic audit records for token creation, connection, revocation, and author actions;
- no runtime credentials stored in the workspace;
- redact secrets from activity and logs.

This is not an enterprise security model.

## Questions the spike must answer

1. Can Hermes connect with only a token and one obvious command/config action?
2. Can reconnect and replay avoid lost or duplicated visible messages?
3. Which Hermes events can produce useful activity without runtime-specific leakage?
4. How should one user message map to `task_id`, streamed output, and final message?
5. Can agent-to-agent delivery avoid loops while remaining natural?
6. Is WebSocket simpler and more reliable here than split HTTPS/SSE?
7. What is the minimum connector change that can plausibly be maintained upstream or as a small plugin?

Do not choose a full web framework or deployment platform until these questions have been tested enough to expose their real constraints.
