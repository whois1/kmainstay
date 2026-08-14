# Lean MVP

This is a hypothesis set and experiment plan, not a feature contract.

## Problem hypothesis

AI power users find it too difficult to place their local agents in a shared chat, and existing chat integrations do not clearly show what agents are doing.

## Customer hypothesis

Start with people who already run at least one local agent, especially Hermes users, developers, technical founders, consultants, and very small AI-native teams.

Do not target people who merely want a general AI chatbot. They have many existing options and do not validate agent connection or supervision.

## Value hypothesis

A hosted workspace is meaningfully better if a user can:

1. add an existing agent in under one minute with one copied token;
2. talk to it alongside another human or agent;
3. understand its live work without seeing chain-of-thought.

## Riskiest assumptions

Ordered roughly by existential risk:

1. **Users care enough to leave Telegram, Discord, or terminals for this workspace.**
2. **Hermes can provide useful structured activity events without invasive runtime changes.**
3. **A single workspace token can make connection genuinely simple while remaining safe enough.**
4. **Agent-to-agent delivery is useful in real work rather than merely impressive in a demo.**
5. **A hosted control surface is trusted with message and activity metadata.**
6. **Users will pay for the workspace while bringing their own runtime/model.**

## MVP boundary

### Must prove

- one user can create one workspace;
- the workspace has `#general`;
- the user can create an agent and receive a one-time-visible token;
- Hermes can connect outbound with that token;
- online/offline state is visible;
- humans and agents can exchange text messages;
- one agent can receive a second agent’s message;
- the UI can stream a response;
- the agent can publish structured activity states;
- a small file or image can be attached to a message;
- channel access is allow/deny per agent.

### Defer unless a test requires it

- multiple workspace roles;
- rich channel administration;
- search, reactions, editing, presence beyond agent connectivity, notifications, and mobile apps;
- projects, shared instructions, shared skills, shared memory, capabilities, and credentials;
- runtime switching;
- billing;
- enterprise controls.

## Success measures

Use behavioural evidence rather than feature completion.

### Onboarding test

For at least five target users:

- median time from “Add Agent” to visible online agent: **under 60 seconds**;
- no more than **one instruction page** and **one copied token**;
- at least **four of five** complete without live help.

### Comprehension test

Show users an agent performing a multi-step task. Without opening raw logs, at least four of five should correctly answer:

- what the agent is doing now;
- what has succeeded or failed;
- whether it is waiting for approval;
- whether it has finished.

### Value test

Before expanding scope, seek evidence of repeated use:

- target users return for a second real task;
- at least three ask to keep using it or connect another agent;
- users prefer it over their current agent-chat path for at least one recurring job.

These thresholds are starting points and may be revised with evidence.

## Smallest build slices

Each slice should be deployable or demonstrable and should test a risk.

### Slice 0: clickable onboarding prototype

A disposable UI prototype for workspace creation, Add Agent, token copy, connection state, channel chat, and activity display.

**Learn:** whether the flow is understandable before building infrastructure.

### Slice 1: protocol spike

A throwaway local gateway plus tiny fake agent using an outbound connection. Exchange a user message, streamed agent output, and activity events.

**Learn:** whether the proposed protocol and activity model are sufficient.

### Slice 2: hosted vertical path

One hosted workspace, `#general`, one human, one agent, persistence, token redemption, and realtime messaging. Manual account setup is acceptable.

**Learn:** whether connection remains simple outside a local demo.

### Slice 3: Hermes connector

The minimum Hermes integration needed to connect, receive messages, send streamed output, and emit activity. Avoid general runtime abstractions beyond a narrow connector boundary.

**Learn:** whether the real runtime supports the intended experience.

### Slice 4: second participant and files

Add a second human or agent, agent-to-agent delivery, channel allow/deny, and minimal attachment handling.

**Learn:** whether shared work creates value beyond a direct agent chat.

### Slice 5: observed pilot

Run the complete demo with target users, measure onboarding and comprehension, and record friction. Fix only demonstrated blockers.

**Learn:** whether to continue, change direction, or stop.

## Build discipline

- One modular monolith until evidence demands otherwise.
- Prefer managed services for a pilot, but avoid provider-specific design before stack selection.
- Implement only the next slice.
- Keep costly choices reversible.
- Instrument onboarding time, connection failures, activity states, and repeat use from the start.
- Record discoveries and decisions, including reasons and evidence.
- Do not confuse a successful scripted demo with validated demand.

## Stop or reconsider if

- target users do not see enough benefit over current chat/terminal paths;
- reliable structured activity requires exposing private reasoning or extensive Hermes changes;
- secure connection needs configuration comparable to existing bot integrations;
- users like the demo but do not return with real work;
- agent-to-agent messaging adds noise without improving outcomes.

## Next decision

Before selecting the application stack, complete Slice 0 and define the smallest Hermes-side integration surface needed for Slice 1. Stack choice should follow those findings rather than lead them.
