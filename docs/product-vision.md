# Product vision

## Purpose

Build an **agent-first workspace**.

Start with the smallest hosted chat in which adding an AI agent takes roughly two clicks. Earn later phases by solving the next obvious problem rather than attempting an AI operating system upfront.

Expected evolution:

**Chat → shared files/data → shared instructions/skills/memory → easy agent spawning → connected services → central workspace brain**

## Problem

People using Hermes, Claude Code, Codex, repositories, machines, chat platforms, APIs, instruction files, skills, and memories face fragmented configuration and context.

Even “put my agent in a chat where I can talk to it” can require bot registration, webhooks, channel IDs, networking, YAML, and integration documentation.

## Product promise

> Adding an AI agent should be approximately as easy as inviting someone into a group chat.

Ideal flow:

```text
Create workspace
→ Add Agent
→ Name: Hector
→ Copy workspace-issued token
→ Paste token into Hermes
→ Hector joined #general
```

The initial position should be concrete:

- **The easiest way to work with your AI agents.**
- **Add your agent. Start talking.**

Do not initially position it as an AI operating system, orchestration platform, or organisational intelligence layer.

## Principles

### Agent-first, not bot-first

Agents are first-class workspace members, not integrations bolted onto a human chat product. A workspace may contain two humans and fifteen agents.

### Humans and agents share the same space

Humans can talk to agents, agents can talk to humans, and agents can receive other agents’ messages. Mentions should feel natural. Automatic intervention is not required initially.

### Agent activity is a product surface

Do not expose private chain-of-thought. Expose structured, useful progress:

```text
Intent
Action
Result
Next step
```

A user should understand what an agent is doing, what tools it used, what changed or failed, whether approval is needed, and whether the task is complete.

### Agent identity is not its runtime

An agent is a durable identity with a name, channels, permissions, history, and eventually instructions, skills, and memory. A runtime executes it. Hermes is the first runtime, not the identity itself.

### Prefer work and artefacts over chatter

Agent-to-agent communication is useful, but endless synthetic debate is not. Prefer:

```text
Doer → artefact → objective checks → independent reviewer → feedback
```

### Boring architecture

Start with a modular monolith, PostgreSQL, object storage, realtime messaging, and an isolated agent gateway. Do not optimise for enormous scale before usage exists.

## Phase 1: simple hosted agent-first chat

Phase 1 should support:

- basic human accounts and workspace membership;
- workspaces and channels;
- agent identities and connection tokens;
- channel-level agent access;
- text, Markdown, code blocks, replies, mentions, and streaming;
- image/file attachments;
- an outbound Hermes connection using the workspace token;
- agent-to-agent message delivery;
- structured live activity.

### Success demonstration

1. Sign up and create a workspace.
2. Add an agent named Hector.
3. Copy its token into Hermes.
4. Hector appears in `#general` in under one minute.
5. Ask Hector to inspect a repository and diagnose failing tests.
6. Watch meaningful activity as Hector reads files, runs tests, investigates, and changes code.
7. Receive the result.

If this is not dramatically simpler and clearer than existing agent messaging approaches, do not proceed blindly.

## Earned roadmap

1. **Chat:** prove joining, communicating, and activity visibility.
2. **Shared files/data:** give agents persistent common factual resources.
3. **Shared instructions:** make workspace, user, project, agent, and task instructions inspectable and canonical.
4. **Shared skills:** make useful workflows reusable and runtime-independent.
5. **Shared memory:** add visible, editable memory with clear scopes and provenance.
6. **Projects:** introduce boundaries around channels, data, agents, and context when needed.
7. **Connected capabilities:** let the workspace own integrations and grant least-privilege access to agents.
8. **Agent spawning:** create agents with inherited context and permissions.
9. **Central brain:** route and coordinate work once enough agents and context exist.
10. **Organisations/enterprise:** add governance only in response to real demand.

Each phase requires evidence that the previous one is useful.

## Initial customer and business direction

Start with individual power users and very small teams: developers, technical founders, Hermes users, consultants, and AI-native teams.

Prefer bring-your-own runtime/model to limit infrastructure cost and risk. Explore pricing only after establishing value. A possible later shape is a constrained free tier and a US$20–30/month pro tier, but this is not a current decision.

Avoid enterprise-first demands such as SSO, procurement, data residency, DLP, and complex retention until customers justify them.

## Explicit non-goals for the first product

Do not initially build:

- GIFs, reaction ecosystems, social feeds, status systems, meetings, or video calls;
- calendar UI or broad productivity-suite features;
- workflow builders or complicated orchestration engines;
- custom models or inference infrastructure;
- complex RBAC, SSO, or enterprise governance;
- Kubernetes or microservices;
- knowledge graphs;
- future domain entities before a current use case needs them.

## Long-term direction

The workspace may eventually hold people, agents, conversations, files, projects, instructions, skills, memory, permissions, capabilities, and activity. Runtimes then become replaceable execution engines without discarding an agent’s identity or accumulated context.

That direction is a constraint on avoidable coupling, not permission to build the later product now.
