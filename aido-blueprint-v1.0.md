# Aido Blueprint: The Documentary Agent

**Version:** 1.0  
**Status:** Foundational blueprint covering core concepts, architecture, and the specd bridge.  
**Companion:** See `docs/bridge-contract.md` for the formal aido ↔ specd integration contract.

---

## 1. Philosophy

Aido is a **documentary agent** for software projects. It does not write code, manage sprints, or replace human judgment. Its sole purpose is to **observe, structure, and narrate** what a project is, what was requested, and how it was understood — in a form both humans and coding agents can consume.

Aido sits **beside** the coding agent, not above it. Both serve the human. The human is the only required bridge.

> **Tagline:** *Aido writes the story of your project — so humans understand it and agents implement it correctly.*

### Core Principles

1. **Repo is truth.** `.aido/` lives inside the project, versioned with code.
2. **Documents are ground truth.** No embedding layer between AI and knowledge. Deterministic retrieval via structured links, explicit references, and document headers — not similarity search.
3. **OKF interoperability.** Knowledge conforms to the **Open Knowledge Format (OKF)** — an open, human- and agent-friendly format for representing knowledge as markdown files with YAML frontmatter.
4. **Any input, one pipeline.** Jira, Slack, voice, email, freeform text — all normalize to the same pipeline: parse intent → read context → produce spec → store in `.aido/requests/`.
5. **Human decides.** Aido suggests, never blocks. Flags are informational.
6. **Agent is optional assistant.** The coding agent helps Aido explore, then implements. Aido helps the coding agent understand, then witnesses.
7. **Communication is file-based.** No APIs, no protocols, no tokens wasted on plumbing. The project repository is the single source of truth.
8. **Witness, don't enforce.** Aido observes and flags. Action is human.
9. **Secrets never in repo.** API keys live in environment or git-ignored files.
10. **Aido owns its LLM.** Aido manages prompts, model selection, and instructions independently.
11. **Pluggable providers.** User chooses model and provider based on cost, privacy, and quality needs.
12. **Minimal stack.** Go + files + OKF. Complexity is in prompts and document logic, not infrastructure.
13. **Workspace depends on Core, never the reverse.** Core is headless; Workspace is optional.
14. **Aider as primary coding agent.** Git-native, model-agnostic, cost-optimized via Architect/Editor mode.

---

## 2. Project Split: Two Separate Projects

Aido is split into two independent but cooperating projects:

| Project | Role | Target User |
|---------|------|-------------|
| **Aido Core** (`aido-core`) | The documentary agent engine — headless, library + CLI | Coding agents, CI, power users |
| **Aido Workspace** (`aido-workspace`) | Native desktop application for managing Aido projects | Human developers |

**Dependency rule:** Workspace always depends on Core. Core never depends on Workspace. Coding agents depend on Core for knowledge retrieval.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AIDO WORKSPACE                               │
│              Native Desktop Application (Tauri + Svelte)             │
│  • Rich GUI for managing multiple Aido projects                      │
│  • OKF visual explorer (graph view, document tree)                   │
│  • Request CRUD + EARS editor with live preview                      │
│  • Request progress tracking & history timeline                      │
│  • LLM chat interface (context-aware: general, spec-building, OKF)   │
│  • Provider API key management UI                                    │
│  • Coding agent configuration (Aider support)                        │
│  • Project onboarding wizard                                         │
└─────────────────────────────────────────────────────────────────────┘
                              │ gRPC / file-based
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         AIDO CORE                                    │
│                    Documentary Agent Engine (Go)                       │
│  • Reads/writes `.aido/` directory                                   │
│  • OKF bundle management                                             │
│  • LLM provider routing & API key resolution                           │
│  • Spec generation (EARS)                                          │
│  • Cheap coding agent inquiries (MCP/CLI)                              │
│  • Witness / sync logic                                              │
│  • File-based communication interface                                │
│  • NO UI — headless, library + CLI binary                            │
└─────────────────────────────────────────────────────────────────────┘
                              │ file-based protocol (.aido/ directory)
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      CODING AGENT (Aider / Claude Code / etc.)       │
│  • Reads `.aido/requests/{id}.md` for specs                          │
│  • Reads linked OKF documents                                        │
│  • Implements code                                                   │
│  • Updates OKF docs if architecture/domain/APIs changed              │
│  • Appends to witness logs                                           │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. Core Concepts

### 3.1 Document-Native Knowledge

Aido does not embed documents into vectors. It reads them directly. Retrieval is deterministic — structured links, explicit references, and document headers — not similarity search. This eliminates vector storage cost, re-embedding overhead, and opaque "magic" retrieval.

### 3.2 OKF-Aligned Knowledge Bundle

Aido's `.aido/okf/` directory is an **OKF v0.1 bundle** — a directory tree of markdown files with YAML frontmatter, cross-linked via standard markdown links. This makes Aido's knowledge:
- Readable by humans without tooling
- Parseable by agents without bespoke SDKs
- Diffable in version control
- Portable across tools, organizations, and time

### 3.3 The `.aido/` Directory

Aido stores all its state inside the target project under `.aido/`. This includes:
- Configuration
- Processed requests and their specifications
- Links between requests and OKF concepts
- Witness logs of observed changes
- Templates for specifications
- The project's OKF knowledge bundle

Everything is plain text, human-readable, and git-tracked.

### 3.4 Any Input, One Pipeline

A request can arrive as a Jira ticket, a freeform Slack message, a voice transcript, an email, or a pasted description. All inputs normalize to the same pipeline: parse intent → read context → produce spec → store in `.aido/requests/`.

### 3.5 Human-in-the-Loop

AI suggests. The human confirms, edits, or overrides. Aido never blocks. Flags are informational.

### 3.6 Coding Agent as Assistant

Aido may ask the coding agent cheap exploratory questions (file trees, function signatures, grep results) to clarify a request. The coding agent implements the final spec. After implementation, the coding agent may optionally update documents and report back to Aido via simple file-based communication.

---

## 4. Architecture

### 4.1 Directory Structure

```
.aido/                              # Aido project root
├── config.yaml                      # Project configuration (repo-safe)
├── .secrets.yaml                    # API keys (git-ignored)
├── .gitignore                       # Aido-managed ignore rules
│
├── okf/                             # ✅ OKF BUNDLE — project knowledge
│   ├── index.md                     # ✅ OKF: bundle root index
│   ├── log.md                       # ✅ OKF: bundle change history
│   ├── architecture.md              # ✅ OKF concept
│   ├── domain-model.md              # ✅ OKF concept
│   ├── glossary.md                  # ✅ OKF concept
│   ├── api-contracts.md             # ✅ OKF concept
│   ├── operations.md                # ✅ OKF concept
│   └── adr/                         # ✅ OKF subdirectory
│       ├── index.md                 # ✅ OKF
│       ├── log.md                   # ✅ OKF
│       └── 001-kafka.md             # ✅ OKF concept
│
├── requests/                        # Aido specs (OKF-inspired frontmatter)
│   ├── req-001.md
│   └── req-002.md
│
├── links.yaml                       # Request → OKF concept mappings
├── witness/                         # Aido observation logs
│   └── 2026-07-20.log
├── templates/                       # Spec generation templates
│   └── ears.md
└── mcp-server/                      # MCP server for coding agent bridge
```

### 4.2 OKF Conformance

Aido's `.aido/okf/` conforms to OKF v0.1:

1. Every non-reserved `.md` file contains parseable YAML frontmatter
2. Every frontmatter contains a non-empty `type` field
3. Reserved filenames (`index.md`, `log.md`) follow OKF structure when present

Aido extends OKF with additional frontmatter keys (e.g., `resource` for repo links) while remaining fully consumable by any OKF-compatible tool.

### 4.3 `config.yaml`

Repo-safe. No secrets. Only references to external key sources.

```yaml
project: taxi
tracked_branch: main
last_sync_commit: abc123

required_docs:
  - okf/architecture.md
  - okf/domain-model.md
  - okf/glossary.md
  - okf/api-contracts.md
  - okf/operations.md

llm:
  default_provider: openrouter
  default_model: anthropic/claude-sonnet-4-20250514

  providers:
    openai:
      api_key_source: env:OPENAI_API_KEY
      base_url: https://api.openai.com/v1

    anthropic:
      api_key_source: env:ANTHROPIC_API_KEY

    mistral:
      api_key_source: env:MISTRAL_API_KEY
      base_url: https://api.mistral.ai/v1

    nvidia_nim:
      api_key_source: env:NVIDIA_API_KEY
      base_url: https://integrate.api.nvidia.com/v1

    openrouter:
      api_key_source: env:OPENROUTER_API_KEY
      base_url: https://openrouter.ai/api/v1

    ollama:
      base_url: http://localhost:11434
      api_key_source: none

  tasks:
    spec_generation: anthropic/claude-sonnet-4-20250514
    cheap_exploration: mistral/mistral-small-latest
    summary: ollama/llama3.1:8b

coding_agent:
  active: aider

  agents:
    aider:
      type: cli
      command: aider
      architect_mode: true
      model: claude-sonnet-4-20250514
      editor_model: claude-sonnet-4-20250514
    cursor:
      type: mcp
      mcp_server_path: .aido/mcp-server/
    claude_code:
      type: cli
      command: claude
    windsurf:
      type: mcp
      mcp_server_path: .aido/mcp-server/
    custom:
      type: mcp
      mcp_server_path: /path/to/custom/mcp

auto_sync: false
```

### 4.4 `.secrets.yaml` (Git-Ignored)

```yaml
# NEVER COMMIT THIS FILE
# Add to .gitignore: .aido/.secrets.yaml

openai_api_key: sk-...
anthropic_api_key: sk-ant-...
mistral_api_key: ...
nvidia_api_key: nvapi-...
openrouter_api_key: sk-or-...
```

### 4.5 API Key Resolution Order

Aido resolves API keys in this priority:

1. **Environment variable** (highest priority)
   - Example: `export OPENAI_API_KEY=sk-...`
2. **`.aido/.secrets.yaml`** (fallback)
   - Read from git-ignored file
3. **OS keyring** (optional)
   - `api_key_source: keyring:aido:openai`
4. **Interactive prompt** (last resort)
   - Aido asks user to input key, stores in `.secrets.yaml`

### 4.6 OKF Concept Document Format

Every document in `.aido/okf/` follows OKF v0.1:

```markdown
---
type: Architecture
title: Taxi Backend Architecture
description: Service boundaries, data flow, and tech stack for the taxi platform.
resource: https://github.com/acme/taxi
tags: [backend, microservices, event-driven]
timestamp: 2026-07-20T10:00:00Z
---

# Overview
...
```

### 4.7 OKF Index File

```markdown
<!-- .aido/okf/index.md -->
# Taxi Project Knowledge

## Systems
- [Architecture](architecture.md) — Service boundaries and data flow
- [Domain Model](domain-model.md) — Entities, states, invariants
- [API Contracts](api-contracts.md) — Interface definitions

## Operations
- [Operations](operations.md) — Deployment and runbooks

## Reference
- [Glossary](glossary.md) — Ubiquitous language
- [Architecture Decisions](adr/) — ADR records
```

### 4.8 OKF Log File

```markdown
<!-- .aido/okf/log.md -->
# Knowledge Update Log

## 2026-07-20
* **Update**: Refined ride matching flow in [architecture](architecture.md).
* **Creation**: Added [ADR-007](adr/007-redis-sharding.md) on Redis sharding strategy.

## 2026-07-15
* **Initialization**: Established foundational knowledge structure.
```

### 4.9 Request Specification Format

Stored in `.aido/requests/{id}.md`:

```markdown
---
aido_version: "0.1"
id: req-001
source: jira://TAXI-2847
title: "Driver app keeps showing wrong ETA"
raw_request: "..."
status: implemented
implemented_by: agent-aider
commits: [def456, ghi789]
---

# Request: Driver ETA Stale After Acceptance

## Context
...

## Domain Analysis
...

## Related Documents
- okf/architecture.md#event-flow
- okf/redis-caching-strategy.md#ttl-policy

## Specification (EARS)

### RQ-1: Event Publication
When the driver accepts a ride, the system shall publish a `RideAccepted` event.

### RQ-2: Cache Invalidation
When the `RideAccepted` event is handled, the system shall invalidate the cached ETA for that ride.

## Open Questions
- [ ] Should we warm the cache with new ETA or let next read trigger it?

## Implementation Notes
- Expected code changes: `internal/ride/eta.go`, `internal/ride/events.go`
- Expected doc updates: `okf/redis-caching-strategy.md#cache-invalidation`
```

### 4.10 `links.yaml`

Uses OKF concept IDs (file paths without `.md`):

```yaml
req-001:
  - okf/architecture#event-flow
  - okf/redis-caching-strategy#ttl-policy

req-002:
  - okf/domain-model#driver-onboarding
  - okf/api-contracts#driver-registration
```

### 4.11 Witness Log Format

Plain text, append-only:

```
2026-07-20T14:32:01Z SYNC main abc123->def456 okf/architecture.md changed
2026-07-20T15:10:33Z REQUEST req-001 implemented commits=[def456,ghi789] docs_changed=[okf/redis-caching-strategy.md]
2026-07-20T15:10:33Z WITNESS req-001 linked docs: all updated OK
2026-07-20T16:45:12Z REQUEST req-002 implemented commits=[jkl012] docs_changed=[] FLAG: no doc updates for linked docs
```

---

## 5. Tech Stack

### 5.1 Aido Core

| Layer | Technology | Status |
|-------|-----------|--------|
| Core Engine | Go 1.22+ | Primary — single binary, fast, excellent file operations |
| CLI Interface | Go stdlib / cobra | Primary human interface |
| File Storage | Plain files (YAML, Markdown) | All state in `.aido/` — no database |
| Git Operations | go-git (pure Go) | No C dependencies, no git binary required |
| Config Parsing | gopkg.in/yaml.v3 | Config and links parsing |
| LLM Routing | Go HTTP client + provider adapters | Multi-provider support |
| Agent Bridge | MCP server (Go SDK) | Standard protocol for agent communication |
| Service Interface | gRPC (Connect / bufbuild) | For Workspace integration |
| Knowledge Format | OKF v0.1 | Interoperable, human and agent readable |

**Removed from original plan:**
- **SQLite** — All state is file-based. No query complexity yet. Re-add only if performance demands it.
- **HTMX UI** — Replaced by Aido Workspace native desktop app.

### 5.2 Aido Workspace

| Layer | Technology | Status |
|-------|-----------|--------|
| Desktop Shell | Tauri 2.0 | Native Rust backend, OS webview, ~10MB binary |
| Frontend Framework | Svelte 5 | Compiled, no VDOM, extremely fast |
| Language | TypeScript | Type safety across frontend |
| Styling | Tailwind CSS | Utility-first, rapid UI development |
| State Management | Svelte Runes | Built-in, no extra library |
| Icons | Lucide / Phosphor | Clean, modern icon set |
| Charts / Timeline | LayerChart (Svelte) | Native Svelte charting |
| Markdown Rendering | Marked + custom OKF components | For OKF docs and specs |
| Code Editor | CodeMirror 6 | Lightweight, embeddable, EARS editing |
| Graph Visualization | D3.js / Cytoscape.js | Force-directed OKF concept graph |
| Communication | gRPC client (Rust) | Talks to Aido Core |
| Packaging | Tauri bundler | `.deb` for Debian Linux |

---

## 6. LLM Configuration

Aido manages its own LLM configuration. The user provides API keys and selects models. Aido handles prompts, instructions, and model routing independently.

### 6.1 Supported Providers

| Provider | Type | Use Case |
|----------|------|----------|
| OpenAI | Paid | Reliable, fast spec generation |
| Anthropic | Paid | Long context, good at structured output |
| Mistral | Free/Cheap | Cost-conscious users |
| NVIDIA NIM | Free/Self-hosted | Privacy-sensitive, local inference |
| OpenRouter | Aggregator | Access to many models, fallback routing |
| Ollama | Local | Fully offline, no API calls |

### 6.2 Task-Based Model Routing

Aido routes prompts to the appropriate model based on task type:

| Task | Typical Model | Why |
|------|---------------|-----|
| Spec generation | Claude Sonnet / GPT-4 | Complex reasoning, structured output |
| Cheap exploration | Mistral Small / local 8B | Low cost, fast, factual |
| Document summary | Ollama Llama 3.1 8B | Free, local, sufficient |
| Quick classification | Mistral Small | Fast, cheap |

---

## 7. Coding Agent Integration

User chooses their coding agent from supported options. This agent is used for cheap knowledge exploration only — never for spec generation or document synthesis.

### 7.1 Supported Agents

| Agent | Bridge Type | Status |
|-------|-------------|--------|
| Aider | CLI (Architect/Editor mode) | **Primary** — git-first, model-agnostic, cost-optimized |
| Cursor | MCP server | Planned |
| Claude Code | CLI / Tool use | Planned |
| Windsurf | MCP server | Planned |
| GitHub Copilot | VS Code extension API | Future |
| Custom | Generic MCP server | Supported |

### 7.2 Aider Integration (Primary)

Aider is the default coding agent. It is terminal-first, git-native, and supports Architect/Editor mode for cost optimization.

**Configuration in `config.yaml`:**

```yaml
coding_agent:
  active: aider
  agents:
    aider:
      type: cli
      command: aider
      architect_mode: true
      model: claude-sonnet-4-20250514
      editor_model: claude-sonnet-4-20250514
      weak_model: mistral-small-latest
      auto_commits: true
      dirty_commits: false
```

**Aider workflow with Aido:**
1. Aido Core produces spec in `.aido/requests/{id}.md`
2. Developer (or Workspace) spawns Aider with the spec as context
3. Aider reads the spec, adds relevant files to context via `/add`
4. Aider implements changes, auto-commits with descriptive messages
5. Aider updates OKF docs if architecture, domain model, or APIs changed
6. Aider appends to `.aido/witness/{date}.log`

**Aider-specific features:**
- **Architect mode:** Aider plans with a reasoning model, then executes with a cheaper model — 30-60% cost savings
- **Repository map:** Tree-sitter parses code into AST, builds dependency graph for context-aware editing
- **Git-native:** Every change is a commit; undo with `git revert` or `/undo`
- **Multi-model:** Switch models mid-session; use local models via Ollama

### 7.3 Cheap Inquiry Examples

Aido asks coding agent for facts, not reasoning:

| Inquiry | Agent Action | Aido Uses Result For |
|---------|-----------|----------------------|
| `list_files("internal/ride/")` | Returns file list | Knowing where to look |
| `grep("RideStatus", "internal/ride/*.go")` | Returns matches | Understanding state machine |
| `read_signatures("internal/ride/cache.go")` | Returns func signatures | Knowing cache interface |
| `git_log("--oneline", "internal/ride/", "-10")` | Returns recent commits | Understanding recent changes |

The coding agent does **no reasoning** — just returns raw text. Aido's LLM does the synthesis.

---

## 8. Aido Workspace: Native Desktop Application

### 8.1 Architecture

```
┌─────────────────────────────────────────┐
│           Tauri 2.0 (Rust)              │
│  ┌─────────────────────────────────┐    │
│  │  Core Bridge (Rust)             │    │
│  │  • Spawns aido-core process     │    │
│  │  • gRPC client to Core          │    │
│  │  • File watcher (notify crate)  │    │
│  │  • Native dialogs, menus, tray  │    │
│  │  • System notifications         │    │
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
                    │
                    ▼ IPC (Tauri commands)
┌─────────────────────────────────────────┐
│         Svelte 5 Frontend               │
│  ┌─────────────────────────────────┐    │
│  │  App Shell (sidebar + main)     │    │
│  │  • Project switcher             │    │
│  │  • Global search                │    │
│  │  • Settings / providers         │    │
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │  Views:                         │    │
│  │  • Dashboard (project health)   │    │
│  │  • OKF Explorer (graph + tree)  │    │
│  │  • Request Manager (list + CRUD)│    │
│  │  • Spec Editor (EARS + preview) │    │
│  │  • LLM Chat (thread-based)      │    │
│  │  • Witness Log (timeline)       │    │
│  │  • Config / Secrets UI          │    │
│  └─────────────────────────────────┘    │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │  Shared Components:             │    │
│  │  • OKFDocumentRenderer          │    │
│  │  • EARSParser/Editor            │    │
│  │  • ChatBubble / ThreadView      │    │
│  │  • ProviderConfigCard           │    │
│  │  • CommitTimeline               │    │
│  │  • StatusBadge                  │    │
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

### 8.2 Core ↔ Workspace Communication

**Protocol:** gRPC (Connect / bufbuild) over local TCP

**Core exposes:**

```protobuf
service AidoCore {
  // Project management
  rpc InitProject(InitProjectRequest) returns (Project);
  rpc SyncProject(SyncProjectRequest) returns (SyncResult);
  rpc ListProjects(ListProjectsRequest) returns (ListProjectsResponse);

  // OKF
  rpc ReadOKFDocument(ReadOKFDocumentRequest) returns (OKFDocument);
  rpc WriteOKFDocument(WriteOKFDocumentRequest) returns (OKFDocument);
  rpc QueryOKF(QueryOKFRequest) returns (QueryOKFResponse);
  rpc ListOKFDocuments(ListOKFDocumentsRequest) returns (ListOKFDocumentsResponse);

  // Requests
  rpc CreateRequest(CreateRequestRequest) returns (Request);
  rpc GetRequest(GetRequestRequest) returns (Request);
  rpc ListRequests(ListRequestsRequest) returns (ListRequestsResponse);
  rpc UpdateRequest(UpdateRequestRequest) returns (Request);
  rpc DeleteRequest(DeleteRequestRequest) returns (Empty);

  // LLM Chat (streaming)
  rpc Chat(stream ChatMessage) returns (stream ChatMessage);
  rpc ListChatThreads(ListChatThreadsRequest) returns (ListChatThreadsResponse);

  // Config
  rpc GetConfig(GetConfigRequest) returns (Config);
  rpc UpdateConfig(UpdateRequestRequest) returns (Config);
  rpc SetSecret(SetSecretRequest) returns (Empty);
  rpc TestProvider(TestProviderRequest) returns (TestProviderResponse);

  // Witness
  rpc GetWitnessLogs(GetWitnessLogsRequest) returns (GetWitnessLogsResponse);

  // Coding Agent
  rpc SpawnAgent(SpawnAgentRequest) returns (SpawnAgentResponse);
  rpc AgentStatus(AgentStatusRequest) returns (AgentStatusResponse);
}
```

**Why gRPC over file polling:**
- Real-time streaming for LLM chat responses
- Strongly typed, efficient binary protocol
- Bidirectional streaming support
- Auto-generated client stubs in Rust and TypeScript
- File-based ground truth remains in `.aido/`; gRPC is the access layer

### 8.3 Workspace Features

#### 1. OKF Explorer
- **Tree View:** Recursive file tree of `.aido/okf/` with frontmatter preview on hover
- **Graph View:** Force-directed graph of OKF concept links (D3.js / Cytoscape.js)
- **Document Viewer:** Custom Svelte component rendering OKF markdown with YAML frontmatter as styled cards
- **Search:** Full-text search across all OKF documents (powered by Core)

#### 2. Request Manager & EARS Editor
- **List View:** Table with filters (status, date, linked OKF docs)
- **Detail View:** Split pane — EARS markdown editor (CodeMirror 6) on left, rendered preview on right
- **EARS Validation:** Real-time parsing of EARS syntax (`When... the system shall...`)
- **Progress Tracking:** Visual pipeline (Ingested → Spec Generated → Implemented → Witnessed → Documented)

#### 3. LLM Chat Interface
- **Context Modes:**
  - **General:** Free chat, uses project OKF as RAG context
  - **Spec Building:** Chat history feeds into `CreateRequest()`, structured output
  - **OKF Q&A:** Questions about architecture/domain model, grounded in documents
- **Features:** Streaming responses, code blocks with copy, citation of OKF sources, thread persistence

#### 4. Provider & Coding Agent Configuration
- **Provider Cards:** Visual config for each LLM provider with masked API key input, model selection dropdown, test connection button
- **Aider Integration:**
  - Configure Aider path, model, architect mode, editor model
  - Spawn Aider in integrated terminal panel
  - Monitor Aider session status and output
  - Auto-pass spec context to Aider on spawn

#### 5. Project Onboarding Wizard
1. Select local repo path
2. Detect git repo, confirm branch
3. Initialize `.aido/` (calls Core's `InitProject`)
4. Check required OKF docs, generate templates if missing
5. Configure first LLM provider
6. Configure coding agent (Aider)

### 8.4 Packaging for Debian

Tauri 2.0 built-in `.deb` support:

```json
// tauri.conf.json
{
  "bundle": {
    "linux": {
      "deb": {
        "depends": ["aido-core"],
        "files": {
          "/usr/bin/aido-core": "../target/release/aido-core"
        }
      }
    }
  }
}
```

- `.deb` installs both `aido-core` (CLI binary) and `aido-workspace` (desktop app)
- `aido-core` spawned as child process or systemd user service
- Desktop entry + icon for app launcher

---

## 9. Workflow

### Phase 1: Project Onboarding

1. Developer runs `aido init taxi` (CLI) or clicks "New Project" in Workspace
2. Aido Core detects git repository, suggests tracking branch "main"
3. Developer confirms
4. Core initializes `.aido/` directory with `config.yaml`, `.gitignore`, `templates/`, `requests/`, `witness/`
5. Core asks for LLM provider and model preference
6. Core securely stores API key in `.aido/.secrets.yaml` (git-ignored)
7. Core asks for coding agent preference (default: Aider)
8. Core checks for required OKF documents in `.aido/okf/`
9. If missing, Core generates OKF templates and warns: "Project not ready — add required documents"
10. Once present, Core records `last_sync_commit` and is ready
11. Workspace displays project in dashboard with health indicators

### Phase 2: Request Ingestion (Any Input)

1. Developer describes request (Jira URL, freeform text, pasted message) — in Workspace or CLI
2. Core normalizes input to raw text
3. Core reads `.aido/config.yaml` for required documents
4. Core reads relevant `.aido/okf/` OKF files (whole or by section header)
5. If unclear, Core asks coding agent cheap exploratory questions
6. Core routes spec generation to configured LLM
7. Core produces structured spec in `.aido/requests/{id}.md`
8. Core stores explicit links in `.aido/links.yaml`
9. Workspace renders spec in editor; developer reviews, edits, confirms
10. Or: developer edits file directly with their editor (CLI workflow)

### Phase 3: Implementation

1. Coding agent (Aider) reads spec from `.aido/requests/{id}.md`
2. Aider reads linked OKF documents
3. Aider implements code (Architect mode for complex tasks)
4. If architecture, domain model, or APIs changed: Aider updates relevant `.aido/okf/` OKF files
5. Aider appends to `.aido/okf/log.md`
6. Aider appends implementation line to `.aido/witness/{date}.log`

### Phase 4: Witness

1. Aido Core monitors tracked branch for OKF document changes
2. On sync, Core checks if linked OKF concepts changed in implementation commits
3. If docs changed: silent, corpus is fresh
4. If docs unchanged: flag request as "consider reviewing linked documents"
5. Flag is informational, not blocking
6. Workspace displays updated status in request timeline

---

## 10. Required Documents (OKF Concepts)

A project is "healthy" when these OKF concepts exist in `.aido/okf/`:

| Document | OKF Type | Purpose |
|----------|----------|---------|
| `architecture.md` | Architecture | Service boundaries, data flow, tech stack |
| `domain-model.md` | Domain Model | Entities, states, invariants, business rules |
| `glossary.md` | Glossary | Ubiquitous language — terms both human and agent must use |
| `api-contracts.md` | API Contract | Interface definitions, DTOs, protocols |
| `operations.md` | Operations | Deployment, observability, runbooks |

Additional documents (ADRs, deep dives) live in `.aido/okf/adr/` or as sections within the above.

---

## 11. Coding Agent Skill (Minimal)

```markdown
# Aido Skill for Coding Agents

When starting work:
1. Read `.aido/requests/{id}.md` for the spec
2. Read linked OKF documents in the spec
3. Implement

When done:
1. If you changed architecture, domain model, or APIs:
   - Update the relevant `.aido/okf/` OKF file
   - Append change to `.aido/okf/log.md`
   - Note which docs changed
2. Append to `.aido/witness/{date}.log`:
   IMPLEMENTATION {request_id} commits=[{hashes}] docs_changed=[{paths}]
```

No API calls. No special protocol. Just file-based communication.

---

## 12. Cost Model

| Activity | Cost |
|----------|------|
| Document embedding | $0 (not used) |
| Vector storage | $0 (plain files) |
| Aido exploring code | $0.001-0.01 per cheap query |
| Spec generation | $0.10-0.30 (doc reading + LLM synthesis) |
| Witness / sync | $0 (file operations only) |
| Aider Architect mode | 30-60% savings vs single-model execution |

Deterministic retrieval eliminates embedding infrastructure and opaque retrieval costs.

---

## 13. OKF Interoperability

By conforming to OKF v0.1, Aido gains:
- **Any agent can read Aido docs** — Cursor, Claude, custom agents — all speak OKF
- **Aido can read any OKF bundle** — Existing team docs in OKF format work immediately
- **Visualizers work** — Google's OKF visualizer renders Aido's knowledge graph
- **Validation tools work** — Community validators check Aido doc conformance
- **Future-proof** — OKF v0.1 evolves; Aido rides the standard

Aido is:
- An **OKF producer** — generates conformant knowledge documents
- An **OKF consumer** — reads any OKF bundle
- An **OKF witness** — tracks changes, flags drift
- Plus **Aido-specific** request processing, EARS generation, agent bridge

---

## 14. The specd Bridge: Integration Architecture

### 14.1 Thesis: Two Planes, One Repository

specd and aido are **two planes that share one repository**, not two tools competing for the same job.

- **specd is the enforcement plane.** A deterministic, LLM-free harness. It ratchets a spec through `perceive → analyze → plan → execute → verify → reflect`, gates every state change on evidence (a `verify` exit `0` pinned to a real git HEAD), and requires a human `approve` at each boundary. Its tagline: *the agent reasons, the harness enforces.*
- **aido is the documentary + reasoning plane.** An LLM-owning agent. It holds the project's OKF knowledge base, answers questions grounded in it, structures your raw request into a clean EARS-shaped request, and witnesses commits so documentation stays a first-class citizen.

They are already compatible where it counts: **both speak EARS**, both are Go, both are file-native and git-tracked, both are plain-markdown-plus-a-structured-sidecar, and both speak MCP. So this is a **bridge, not a merge** — and the bridge is **pure convention**: a documented contract plus a skill the coding agent loads. No adapter code lives in either project.

### 14.2 The One Hard Rule

> Neither tool ever writes into the other's directory. `aido` owns `.aido/`. `specd` owns `.specd/`. The **coding agent is the only actor that reads both** — and it writes only into `.specd/` (via the specd CLI) and into code.

### 14.3 The Three Actors and Two Worlds

There are exactly **three actors** and **two owned directories**. The coding agent is the membrane between them.

```
        ┌──────────────────────────── aido world (.aido/) ────────────────────────────┐
        │  OKF knowledge base   ·   aido request (.aido/requests/{slug}.md)   ·        │
        │  links.yaml   ·   witness log                                                │
        │  Owner: aido (LLM-driven). Answers your questions. Structures your request.  │
        └───────────────▲───────────────────────────────────────────────┬─────────────┘
                        │ (reads OKF + aido request as grounding)         │ (reads git HEAD +
                        │                                                 │  specd report --json)
                ┌───────┴─────────────────────────────────────────────────▼───────┐
                │                     CODING AGENT (the membrane)                   │
                │   Loaded with: AGENTS.md (specd) + the Bridge Skill (§15)          │
                │   The ONLY actor that reads both worlds.                          │
                │   Writes: .specd/ (via specd CLI) and source code. Nothing in .aido/. │
                └───────┬───────────────────────────────────────────────────────────┘
                        │ (drives: new → approve → next → verify → complete → submit)
        ┌───────────────▼──────────────────────────── specd world (.specd/) ──────────┐
        │  requirements.md · design.md · tasks.md · state.json · roles · steering       │
        │  Owner: specd (deterministic harness). Gates, DAG, evidence, ratchet.         │
        └───────────────────────────────────────────────────────────────────────────────┘
```

### 14.4 The Circulation Loop

Every hop is a file or CLI read — never a code call:

1. **You ask aido a question or hand it a raw request.** aido answers / structures it from OKF, and writes `.aido/requests/{slug}.md` (aido's version, EARS-shaped, with the OKF concepts it leaned on recorded in `links.yaml`). **Nothing in `.specd/` touched.**
2. **Agent (analyze → reformat into specd's complete flow).** Loaded with the Bridge Skill, the agent reads the aido spec + the referenced OKF docs, runs `specd new <slug>`, and **re-derives** specd's own flow. Requirements come out as specd-native EARS. Then, through the gated loop, it re-derives `design.md` and `tasks.md`. **The aido spec is never fed to specd's pipeline** — only these agent-authored artifacts are.
3. **Human ratchet.** You review and `specd approve requirements`. Then design → tasks → waves → verify → complete, all gated. aido is not in this loop.
4. **Execute + evidence.** `specd next` → `specd verify` (exit 0, pinned to HEAD) → `specd complete-task`. specd will not mark done without the passing record. specd artifacts stay OKF-agnostic — no concept ids stamped anywhere.
5. **Breadcrumb.** Agent appends to `BRIDGE.log`: `HANDOFF slug=<slug> specd_status=complete head=<sha> okf_hints=[...]`.
6. **aido (witness + document).** aido takes its request-time `links.yaml` priors, reads `specd report <slug> --history --json` + the diff, runs a broad diff→OKF inference, notices which docs changed, and **flags / drafts** an update for your approval. It appends to `okf/log.md`. Docs stay first-class.

The loop closes — OKF → request → requirements → commits → witness → OKF — with **zero code coupling**.

---

## 15. Bridge Contract v0.1 (Summary)

> **Full contract:** See `docs/bridge-contract.md` for the complete, versioned convention.

### 15.1 Directory Boundary (Non-Crossing Rule)
- aido reads/writes only `.aido/`.
- specd reads/writes only `.specd/` (+ code, via its gated flow).
- The coding agent reads both, writes only `.specd/` (via CLI) and source. **No process writes across the boundary.**

### 15.2 Identifier & Trace Convention
- A single **slug** (kebab-case, human-meaningful, e.g. `driver-eta-stale`) is minted when aido creates the request.
- It is reused verbatim as: aido request id **and** specd slug (`specd new driver-eta-stale`).
- **Trace chain (Option B — loose coupling).** specd artifacts stay **OKF-agnostic**: no OKF ids are ever stamped into `requirements.md`, `design.md`, or `tasks.md`. The chain is anchored on the shared **slug**:
  `OKF concept ids  ←(recorded at request time)→  aido request + links.yaml  ←(shared slug)→  specd spec  →(git)→  commits`
- aido records the OKF concepts the request leaned on in `links.yaml` **at request time** — this is the primary witness anchor. At witness time aido joins `slug → concepts` (from `links.yaml`) with `slug → commits` (from git / `report --json`) and infers doc drift with its own reasoning over the diff.

### 15.3 Format Independence (No Dialect Coupling)
- The two tools keep **their own formats.** aido writes the aido spec in aido's style; specd's `ears` gate validates only the **agent-authored** specd `requirements.md`. aido is under no obligation to satisfy specd's grammar, and specd never sees aido's file.
- The agent **reformats, not translates.** It analyzes the aido spec and re-derives specd's complete flow (requirements → design → tasks) in specd's own grammar, grounded in OKF. Impedance between the two formats is the agent's job to absorb.
- Consequence: the human review at `specd approve requirements` is a **real** review of a re-derivation, not a rubber-stamp of aido's EARS. That's the correct place for that scrutiny — specd's ratchet is exactly where intent becomes enforceable.

### 15.4 Witness Inputs (specd → aido, read-only)
- **Preferred:** `specd report <slug> --history --json` and `--trace --json` (metadata-only run trace: records, git_heads, task status).
- **Fallback (specd absent or older):** raw `git log` over the slug's touched files.
- aido never parses `state.json` internals as a contract; it consumes the documented `--json` report surface, which is specd's stable machine output.

### 15.5 The Witness Breadcrumb
On the last completed task (or at submit), the agent appends one line to a **neutral repo-root `BRIDGE.log`**:
```
HANDOFF slug=driver-eta-stale specd_status=complete head=<sha> okf_hints=[architecture#event-flow, redis-caching#ttl]
```
Under Option B the `okf_hints` are exactly that — **hints, not authority.** They cost nothing (they live outside both trees, specd never sees them) and they let the agent pass forward any concept it discovered *during* implementation that aido couldn't have known at request time. aido is free to trust, verify, or ignore them; its own re-derivation over `links.yaml` + the diff remains the source of truth.

### 15.6 Versioning & Degradation
- The contract carries a `bridge_contract: "0.1"` marker (in the skill header and the aido request frontmatter).
- If a party sees an unknown version, it degrades to file-only, read-only behavior and surfaces a notice rather than guessing.

---

## 16. Bridge Skill v0.1 (The "Code" of a No-Code Bridge)

A single markdown skill the coding agent loads alongside specd's `AGENTS.md`. It can also be referenced from a specd `steering/*.md` file (specd-native) and from aido's coding-agent skill list — both by convention, neither authoring the other.

```markdown
---
name: aido-specd-bridge
bridge_contract: "0.1"
applies_to: coding-agent (Claude Code / Aider / Cursor / custom)
---

# aido ↔ specd Bridge Skill

You are the membrane between two worlds. You read both; you write only `.specd/`
(via the specd CLI) and source code. You never edit files under `.aido/`.

## When starting from an aido spec
1. Read `.aido/requests/<slug>.md` (the aido spec) — human-approved *intent* and source material, not a file to ingest or copy.
2. Read every OKF doc it references (spec body + `.aido/links.yaml`). These are your grounding; the knowledge base of record is `.aido/okf/`.
3. Run `specd new <slug>` (reuse the aido slug verbatim).
4. **Reformat, don't translate.** Analyze the aido spec against the OKF grounding and re-derive specd's *complete* flow in specd's own grammar — author `requirements.md`, then through the gated loop author `design.md` and `tasks.md`. The aido spec never enters specd's pipeline; only your specd-native artifacts do.
5. Drive the ratchet: stop at each boundary for the human `specd approve`.

## Through the specd lifecycle
- Drive the normal loop: `approve` → `next` → `context` → edit → `verify` → `complete-task`.
- Honor every gate. There is no bypass. Evidence is a passing verify pinned to git HEAD.
- Keep specd artifacts OKF-agnostic: do NOT stamp OKF concept ids into requirements/design/tasks.
  specd must never need to know aido exists.

## On completion (last task / before submit)
- Append the witness breadcrumb (Bridge Contract §15.5) to the repo-root `BRIDGE.log`:
  `HANDOFF slug=<slug> specd_status=<status> head=<sha> okf_hints=[...]`
- `okf_hints` are optional pass-forward hints for aido — include any OKF concept you noticed
  the change actually touched (especially ones the original request didn't anticipate). Hints, not authority.
- Do not touch `.aido/okf/` yourself. aido owns documentation. Your job ends at the breadcrumb.

## If aido is not present
- There is no aido request. Proceed with specd's normal flow from a human-written
  requirements.md. This skill simply doesn't fire.
```

That skill file **is** the integration. Everything else is the two tools behaving normally.

---

## 17. Three Independence Run-Modes (All First-Class)

| Mode | What runs | How it works |
|---|---|---|
| **specd only** | `.specd/` + coding agent | Human hand-authors `requirements.md`. The Bridge Skill never fires (no aido request). specd is unaware aido exists. |
| **aido only** | `.aido/` + any coding agent | aido answers questions, structures requests, and witnesses. The coding agent implements straight from `.aido/requests/` using aido's existing coding-agent integration (Aider, etc.). specd is absent. |
| **Both** | full loop (§14.4) | The Bridge Skill connects them. Still no code coupling — the agent is the only thing that knows both. |

Removing either tool degrades to the adjacent mode with **no migration** — because nothing was ever cross-written.

---

## 18. Knowledge-Sharing Model: Author-Time Transfer

Because the agent authors specd's *own* artifacts from OKF (rather than aido projecting a view into `.specd/`), **knowledge transfers at authoring time.** The path is:

```
OKF (canonical)  →  agent reads it  →  agent bakes derived decisions into specd requirements/design  →  specd gates the artifacts
```

specd therefore **never needs to ingest OKF at all.** Its bounded-context manifests (`specd context`, `next --dispatch`) read specd-native files that already encode the OKF-derived decisions. There is exactly one knowledge base (OKF); specd consumes *decisions derived from it*, not the base itself. That satisfies "same knowledge base" without duplication, without projection, and without violating "no cross-authoring." It also keeps specd's determinism spotless — no LLM, no OKF, in any gate.

### Witness Precision: Option B (Looser Coupling)

specd stays **100% OKF-agnostic**. No OKF ids in any specd artifact. aido re-derives which concepts a completed spec touched, using:
- `links.yaml` — the `slug → OKF concepts` mapping aido recorded **at request time** (the primary anchor / priors).
- The diff + commits for that slug (git, or `report --json`).
- Its own LLM reasoning over the diff to confirm which concept docs the change affected and whether they were updated.
- Optionally, the `okf_hints` the agent left in `BRIDGE.log` (§15.5) — pass-forward hints, never binding.

**Why this fits aido's identity.** Determinism lives in specd; inference lives in aido. Option B puts the concept-mapping *inference* exactly where the inference engine already is. specd literally cannot tell aido exists — the cleanest possible separation.

**The one consequence to design around.** With no specd-side backstop, witness precision now rests on two things: (1) how thoroughly aido records concepts in `links.yaml` at request time, and (2) aido's diff inference at witness time. If implementation drifts and pulls in a concept the original request never anticipated, links.yaml won't contain it — so aido must catch it by inference (helped by the optional `okf_hints`). Mitigation is cheap and on-brand: aido's witness pass should run a **broad diff→OKF inference** (not limited to links.yaml priors), since aido owns an LLM and reads OKF directly. Record `links.yaml` generously; treat it as priors, not a whitelist.

---

## 19. End-to-End Worked Example

**Bug:** "Driver ETA doesn't update after acceptance — probably caching, worse at rush hour."

1. **aido (reason + structure).** You ask aido. It reads `okf/architecture.md#event-flow` and `okf/redis-caching-strategy.md#ttl-policy`, explains the likely cause, and writes `.aido/requests/driver-eta-stale.md` with an EARS block and `links.yaml: driver-eta-stale → [architecture#event-flow, redis-caching#ttl-policy]`. **Nothing in `.specd/` touched.**
2. **Agent (analyze → reformat into specd's complete flow).** Loaded with the Bridge Skill, the agent reads the aido spec + the two OKF docs, runs `specd new driver-eta-stale`, and re-derives specd's own flow. Requirements come out as:
   - `R1` When the driver accepts a ride, the system shall publish a `RideAccepted` event.
   - `R2` When `RideAccepted` is handled, the system shall invalidate the cached ETA for that ride.
   Then, through the gated loop, it re-derives `design.md` and `tasks.md`. **The aido spec is never fed to specd's pipeline** — only these agent-authored artifacts are.
3. **Human ratchet.** You review and `specd approve requirements`. Then design → tasks (a small DAG: publish-event task, cache-invalidation task, integration-test task) → each gated.
4. **Execute + evidence.** `specd next` → `specd verify` (exit 0, pinned to HEAD) → `specd complete-task`. specd will not mark done without the passing record. specd artifacts stay OKF-agnostic — no concept ids stamped anywhere.
5. **Breadcrumb.** Agent appends to `BRIDGE.log`: `HANDOFF slug=driver-eta-stale specd_status=complete head=<sha> okf_hints=[architecture#event-flow, redis-caching#ttl-policy]`.
6. **aido (witness + document).** aido takes its request-time `links.yaml` priors (`[architecture#event-flow, redis-caching#ttl-policy]`), reads `specd report driver-eta-stale --history --json` + the diff, runs a broad diff→OKF inference (optionally aided by the `okf_hints`), notices `redis-caching-strategy.md` changed behavior but the doc wasn't updated, and **flags / drafts** an update to `#ttl-policy` for your approval. It appends to `okf/log.md`. Docs stay first-class.

Every hop was a file or `--json` CLI read. Neither binary called the other.

---

## 20. Open Decisions

1. **Knowledge witness precision.** Resolved: Option B (looser). specd stays OKF-agnostic; aido re-derives from `links.yaml` + diff inference. Record `links.yaml` generously; run a broad diff→OKF inference at witness time (§18).
2. **Breadcrumb location.** Resolved: neutral repo-root `BRIDGE.log`, carrying optional `okf_hints`. Keeps "the agent writes nothing under `.aido/`" absolutely clean (§15.5).
3. **Where the skill is canonically hosted:** a repo `skills/` file both reference, vs duplicated into each tool's convention dir. *(Single-source + reference avoids drift.)*
4. **EARS grammar authority:** confirm specd's `ears` gate is the canonical dialect both sides target. *(Recommended.)*
5. **Contract home:** `docs/bridge-contract.md` in which repo — a third neutral repo, or checked into whichever project you consider "primary"? *(A neutral home matches the peer, no-dependency spirit.)*

---

*Blueprint v1.0. Synthesizes aido architecture, philosophy, and the specd bridge contract into a single foundational document.*