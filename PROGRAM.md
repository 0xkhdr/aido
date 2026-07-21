# Aido Program

Ordered spec list for aido-core. Derived from `aido-blueprint-v1.0.md` (source
material) and constrained by `.specd/steering/*` (authority).

**Provenance:** this file did not exist at the start of the 2026-07-21
unattended run. It was drafted by the agent from the blueprint on the operator's
instruction, and then executed under delegated approval. It is *not* a
human-authored program. Treat its scope as provisional until a human reviews it.

Scope rule applied (`reasoning.md` R5, R3): the smallest spine that makes
aido-core a real, buildable, testable Go program. Everything the blueprint
describes beyond that spine is deferred, not forgotten — see *Deferred* below.

## Dependency order

Linear. Each spec `follows` the one above it.

| # | Slug | What it lands | Follows |
|---|------|---------------|---------|
| 1 | `aido-config` | Go module + `internal/config`: `.aido/config.yaml` load, validation, API-key resolution order (P7), atomic write helper (T10) | — |
| 2 | `okf-bundle` | `internal/okf`: frontmatter parse, OKF v0.1 conformance check (P3), concept-id derivation (S3), reserved-name handling (S2) | `aido-config` |
| 3 | `query-links` | `internal/query`: slug validation (S4), `links.yaml` read/write as query→concept priors (C2) | `okf-bundle` |

`cmd/aido` grows one subcommand per spec as the packages land; it stays wiring
only (S5).

## Deferred (named, not built)

Not in this program. Each needs its own spec and a human-set priority:

- LLM provider adapters and routing (`internal/llm`, T6/T8)
- Coding-agent bridge and cheap inquiries (`internal/agent`, P8)
- Witness/sync, drift inference, witness logs (`internal/witness`, P6, S9)
- MCP server (`.aido/mcp-server/`, OQ-2 unresolved)
- gRPC service definitions (`proto/`, C1)
- Query ingestion pipeline and normalizers (P4)
- Aido Workspace in its entirety (separate repository, OQ-1)
