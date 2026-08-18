# Company Brain thesis

**Status:** long-term product thesis and idea bank. This is not an approved roadmap, MVP scope, or architecture commitment.

## Thesis

K-Mainstay could grow from an agent-first chat into:

> A private, version-controlled company brain where humans and AI agents share organisational knowledge while having explicit permissions over what each identity can read, modify and do.

Chat would be one replaceable interface. The durable product would be the organisation's maintained, permissioned and versioned knowledge.

The distinctive premise is not “put a chatbot over company documents”. It is:

> Design organisational information from the beginning for humans, agents and services as first-class security principals.

## Why this may matter

Existing products generally add AI to systems designed for human users. Capable agents introduce harder requirements:

- a model must receive only knowledge the current human-agent combination may access;
- agents need explicit data, tool and action authority rather than a prompt saying what not to do;
- AI-generated changes need provenance, review, approval, history and rollback;
- company knowledge must outlast conversations, agents, interfaces and model providers;
- private or zero-egress operation may be necessary for sensitive organisations.

The possible product moat is the control layer joining:

```text
ingestion
+ normalisation
+ provenance
+ permissions
+ version history
+ agent execution
+ knowledge maintenance
```

Chat, embeddings, transcription, document parsing and local model hosting are likely to become commodities on their own.

## Core product model

### Identities

Humans, agents and services use the same authorisation system. An agent is a real security principal with its own:

- role and instructions;
- knowledge scope;
- tool permissions;
- action authority;
- memory and skills;
- model and inference policy.

A user's broad access does not automatically pass to an agent. Effective knowledge access is the intersection of the requesting user's permissions and the invoked agent's permissions.

### Permissioned knowledge

A coherent organisational tree could contain company, team, project and personal spaces. Folder permissions inherit by default and may be overridden.

Possible authorities include read, create, write, delete, propose, approve, execute and administer. `Propose` is especially important: an agent can prepare a change without silently modifying authoritative knowledge.

Permissions must apply before retrieval, including to full-text search, semantic search, structured queries, citations, filenames, embeddings, diffs and historical versions. “Search everything, then tell the model not to reveal secrets” is not an acceptable security model.

### Versioned change

Every knowledge change should preserve:

- human or agent identity;
- reason and supporting evidence;
- affected sources;
- prior version and diff;
- required and recorded approval;
- rollback path.

Business users should see familiar history, compare, approve and restore interactions. Git-like mechanics may exist underneath without exposing Git terminology.

### Original and derived representations

Original files remain authoritative. The system creates synchronised, permission-preserving AI representations suited to each medium:

- prose: structured text and source mapping;
- PDF: extracted text, page mapping and visual fallback;
- spreadsheets: schemas, tables, formulas and queryable data;
- images: original, optical character recognition, description and metadata;
- audio/video: original, transcript, speakers, timestamps and keyframes;
- meetings: transcript, decisions, actions and proposed knowledge changes.

Derived content must retain source, location, version, ownership, timestamp and security scope. Changed content should be reprocessed incrementally rather than rebuilding everything.

### Retrieval and inference

Retrieval should combine exact lookup, full-text search, metadata, semantic search, structured queries, relationship traversal and reranking. The query determines the mechanism.

Inference policies could range from strict local processing to local-by-default, hybrid or cloud operation. A credible zero-egress mode would require local document extraction, optical character recognition, transcription, embeddings, search, reranking, model inference and agent execution, plus operational controls that prevent accidental outbound transfer.

Software deployment should come before hardware. A managed appliance or certified server may become useful later, but hardware would add support and supply-chain burdens before product value is proven.

### Knowledge maintenance

The stronger outcome is not merely answering questions. The system could help maintain organisational truth by finding:

- stale or contradictory policies;
- undocumented meeting decisions;
- knowledge trapped in tickets or individual employees;
- procedures affected by a new decision;
- proposed changes requiring an authorised reviewer.

Agents should bundle related edits into reviewable change sets with provenance rather than silently changing many files.

## Interfaces built on the substrate

Potential interfaces include:

- small agent-first chat;
- search with permission-safe citations;
- a business-friendly document and wiki editor;
- voice instructions that produce proposed changes;
- meeting capture that proposes decisions, actions and knowledge updates;
- APIs and connectors to existing business systems;
- visible delegation and agent activity.

K-Mainstay should not attempt to recreate Slack, Teams, SharePoint, Notion, GitHub and Microsoft 365. Existing systems may remain sources and interfaces. The central knowledge and control layer should not depend on one chat frontend, model provider, editor, storage engine or inference runtime.

## Major unresolved questions

These ideas introduce a different customer and problem from the current MVP. They remain hypotheses:

1. Do 20–200 person organisations have an urgent enough problem to adopt another knowledge control layer?
2. Which first workflow creates enough value to justify ingestion, permission mapping and trust work?
3. Is private inference a buying reason, a procurement requirement, or expensive infrastructure customers will not value?
4. Can permission-safe retrieval be made understandable and administrable without reproducing enterprise identity-management complexity?
5. Will organisations accept a new authoritative repository, or must K-Mainstay initially index existing systems without owning the source?
6. Can agents maintain knowledge accurately enough that review saves time rather than creating more work?
7. Is the initial customer still an individual agent power user, or an organisation buying private knowledge infrastructure?

The seventh question is especially important. It affects product, distribution, deployment, security and pricing. Do not silently treat both customer groups as one roadmap.

## Smallest useful validation

Do not build the full Company Brain next.

After the current chat proposition shows repeated use, test one narrow knowledge workflow end to end. A suitable experiment would use a few synthetic documents across three security folders and two agents with different access.

The demonstration should prove:

1. originals are ingested into useful derived representations;
2. exact and semantic retrieval both exclude unauthorised content before inference;
3. answers cite permitted source locations and versions;
4. one agent can propose a documented change;
5. an authorised human can review, approve and restore it;
6. a denied search and denied delegation are visible in the activity history;
7. the path can run locally, with no content egress.

This would prove architectural feasibility, not customer demand. Before or alongside it, interview target organisations around one concrete knowledge-maintenance problem and reconstruct their current workflow.

## Guardrails

Until evidence justifies expansion:

- keep the current agent-first chat MVP unchanged;
- do not add a general permissions engine, document platform, local inference stack, connectors, meetings or hardware;
- do not call a vector database plus chat a Company Brain;
- do not make private inference claims without verifiable egress controls;
- do not make K-Mainstay authoritative for company knowledge before rollback, provenance and permission history work;
- do not expose inaccessible information through names, search counts, citations, logs, embeddings or history;
- preserve the simplest current architecture and introduce abstractions only for a tested slice.

The thesis should guide avoidable coupling and future experiments. It is not permission to optimise today for an unvalidated enterprise platform.
