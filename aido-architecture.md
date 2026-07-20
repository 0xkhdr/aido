# Aido: The Documentary Agent

## Philosophy

Aido is a **documentary agent** for software projects. It does not write code, manage sprints, or replace human judgment. Its sole purpose is to **observe, structure, and narrate** what a project is, what was requested, and how it was understood — in a form both humans and coding agents can consume.

Aido sits beside the coding agent, not above it. Both serve the human. The human is the only required bridge.

The project repository is the single source of truth. Aido's brain lives inside it, in the `.aido/` directory, versioned with code, readable by anyone.

---

## Core Concepts

### Document-Native Knowledge

Aido does not embed documents into vectors. It reads them directly. Retrieval is deterministic — structured links, explicit references, and document headers — not similarity search. This eliminates vector storage cost, re-embedding overhead, and opaque "magic" retrieval.

### The `.aido/` Directory

Aido stores all its state inside the target project under `.aido/`. This includes:
- Configuration
- Processed requests and their specifications
- Links between requests and documents
- Witness logs of observed changes
- Templates for specifications
- The project's foundational documents

Everything is plain text, human-readable, and git-tracked.

### Any Input, One Pipeline

A request can arrive as a Jira ticket, a freeform Slack message, a voice transcript, an email, or a pasted description. All inputs normalize to the same pipeline: parse intent, read context, produce spec, store in `.aido/requests/`.

### Human-in-the-Loop

AI suggests. The human confirms, edits, or overrides. Aido never blocks. Flags are informational.

### Coding Agent as Assistant

Aido may ask the coding agent cheap exploratory questions (file trees, function signatures, grep results) to clarify a request. The coding agent implements the final spec. After implementation, the coding agent may optionally update documents and report back to Aido via simple file-based communication.

---

## Use Case

A backend developer on the **taxi** project lands on Aido. They describe a bug: "Driver ETA doesn't update after acceptance, probably caching, happens in rush hour."

Aido:
1. Reads `.aido/config.yaml` to know the project's required documents
2. Reads `docs/architecture.md` to understand the system map
3. Reads `docs/domain-model.md#ride-lifecycle` to understand state transitions
4. Reads `docs/redis-caching-strategy.md` because caching was mentioned
5. If unclear, asks the coding agent: "List files in `internal/ride/` that handle ETA"
6. Synthesizes understanding
7. Produces a structured EARS specification in `.aido/requests/req-003.md`
8. Stores explicit links in `.aido/links.yaml`

The developer reviews the spec, edits it, and saves.

Later, a coding agent reads `.aido/requests/req-003.md`, implements the code, updates `docs/redis-caching-strategy.md` to reflect new invalidation logic, and appends a line to `.aido/witness/2026-07-20.log`.

Aido witnesses the commit, sees the linked doc was updated, and marks the request as fully documented.

---

## Architecture

### Directory Structure

```
.aido/
├── config.yaml              # Project configuration
├── requests/                # All processed requests and their specs
│   ├── req-001.md
│   └── req-002.md
├── links.yaml               # Explicit request-to-document mappings
├── witness/                 # Machine-readable observation logs
│   └── 2026-07-20.log
├── templates/               # Templates for spec generation
│   └── ears.md
└── docs/                    # Foundational project documents
    ├── architecture.md
    ├── domain-model.md
    ├── glossary.md
    ├── api-contracts.md
    ├── operations.md
    └── adr/
        └── 001-kafka.md
```

### `config.yaml`

```yaml
project: taxi
tracked_branch: main
last_sync_commit: abc123

required_docs:
  - docs/architecture.md
  - docs/domain-model.md
  - docs/glossary.md
  - docs/api-contracts.md
  - docs/operations.md

auto_sync: false
```

### Request Specification Format

Stored in `.aido/requests/{id}.md`:

```markdown
---
id: req-001
source: jira://TAXI-2847
title: "Driver app keeps showing wrong ETA"
raw_request: "..."
status: implemented
implemented_by: agent-cursor
commits: [def456, ghi789]
---

# Request: Driver ETA Stale After Acceptance

## Context
...

## Domain Analysis
...

## Related Documents
- docs/architecture.md#event-flow
- docs/redis-caching-strategy.md#ttl-policy

## Specification (EARS)

### RQ-1: Event Publication
When the driver accepts a ride, the system shall publish a `RideAccepted` event.

### RQ-2: Cache Invalidation
When the `RideAccepted` event is handled, the system shall invalidate the cached ETA for that ride.

## Open Questions
- [ ] Should we warm the cache with new ETA or let next read trigger it?

## Implementation Notes
- Expected code changes: `internal/ride/eta.go`, `internal/ride/events.go`
- Expected doc updates: `docs/redis-caching-strategy.md#cache-invalidation`
```

### `links.yaml`

```yaml
req-001:
  - docs/architecture.md#event-flow
  - docs/redis-caching-strategy.md#ttl-policy

req-002:
  - docs/domain-model.md#driver-onboarding
  - docs/api-contracts.md#driver-registration
```

### Witness Log Format

Plain text, append-only:

```
2026-07-20T14:32:01Z SYNC main abc123->def456 docs/architecture.md changed
2026-07-20T15:10:33Z REQUEST req-001 implemented commits=[def456,ghi789] docs_changed=[docs/redis-caching-strategy.md]
2026-07-20T15:10:33Z WITNESS req-001 linked docs: all updated OK
2026-07-20T16:45:12Z REQUEST req-002 implemented commits=[jkl012] docs_changed=[] FLAG: no doc updates for linked docs
```

---

## Workflow

### Phase 1: Project Onboarding

1. Developer creates project "taxi", points to local repo path
2. Aido detects git repository, suggests tracking branch "main"
3. Developer confirms
4. Aido initializes `.aido/` directory with `config.yaml`, `templates/`, `requests/`, `witness/`
5. Aido checks for required documents in `.aido/docs/`
6. If missing, Aido generates templates and warns: "Project not ready — add required documents"
7. Once present, Aido records `last_sync_commit` and is ready

### Phase 2: Request Ingestion (Any Input)

1. Developer describes request (Jira URL, freeform text, pasted message)
2. Aido normalizes input to raw text
3. Aido reads `.aido/config.yaml` for required documents
4. Aido reads relevant `.aido/docs/` files (whole or by section header)
5. If unclear, Aido asks coding agent cheap exploratory questions
6. Aido produces structured spec in `.aido/requests/{id}.md`
7. Aido stores explicit links in `.aido/links.yaml`
8. Developer reviews, edits, confirms

### Phase 3: Implementation

1. Coding agent reads spec from `.aido/requests/{id}.md`
2. Coding agent reads linked documents
3. Coding agent implements code
4. If architecture, domain model, or APIs changed: coding agent updates relevant `.aido/docs/` files
5. Coding agent appends implementation line to `.aido/witness/{date}.log`

### Phase 4: Witness

1. Aido monitors tracked branch for document changes
2. On sync, Aido checks if linked documents changed in implementation commits
3. If docs changed: silent, corpus is fresh
4. If docs unchanged: flag request as "consider reviewing linked documents"
5. Flag is informational, not blocking

---

## Required Documents

A project is "healthy" when these documents exist in `.aido/docs/`:

| Document | Purpose |
|----------|---------|
| `architecture.md` | Service boundaries, data flow, tech stack |
| `domain-model.md` | Entities, states, invariants, business rules |
| `glossary.md` | Ubiquitous language — terms both human and agent must use |
| `api-contracts.md` | Interface definitions, DTOs, protocols |
| `operations.md` | Deployment, observability, runbooks |

Additional documents (ADRs, deep dives) live in `.aido/docs/adr/` or as sections within the above.

---

## Coding Agent Skill (Minimal)

```markdown
# Aido Skill for Coding Agents

When starting work:
1. Read `.aido/requests/{id}.md` for the spec
2. Read linked documents in the spec
3. Implement

When done:
1. If you changed architecture, domain model, or APIs:
   - Update the relevant `.aido/docs/` file
   - Note which docs changed
2. Append to `.aido/witness/{date}.log`:
   IMPLEMENTATION {request_id} commits=[{hashes}] docs_changed=[{paths}]
```

No API calls. No special protocol. Just file-based communication.

---

## Cost Model

| Activity | Cost |
|----------|------|
| Document embedding | $0 (not used) |
| Vector storage | $0 (plain files) |
| Aido exploring code | $0.001-0.01 per cheap query |
| Spec generation | $0.10-0.30 (doc reading + LLM synthesis) |
| Witness / sync | $0 (file operations only) |

Deterministic retrieval eliminates embedding infrastructure and opaque retrieval costs.

---

## Principles

1. **Repo is truth.** `.aido/` lives inside the project, versioned with code.
2. **Documents are ground truth.** No embedding layer between AI and knowledge.
3. **Any input, one pipeline.** Jira is a source, not a special case.
4. **Human decides.** Aido suggests, never blocks.
5. **Agent is optional assistant.** Coding agent helps Aido explore, then implements.
6. **Communication is file-based.** No APIs, no protocols, no tokens wasted on plumbing.
7. **Witness, don't enforce.** Aido observes and flags. Action is human.

---

## Tagline

> **Aido writes the story of your project — so humans understand it and agents implement it correctly.**
