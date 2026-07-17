<!-- specd:managed:steering/tech.md:v3 begin -->
# Steering: Tech

> Stack has started landing. Concrete choices below are real (in the tree today);
> the AI/knowledge layers are still intent until their code lands. Promote further
> settled choices to memory.md once verified.

## Stack (current — in the tree)
- **Language / runtime:** Go 1.26.
- **HTTP:** stdlib `net/http` with the 1.22+ method+pattern mux (e.g.
  `GET /`, `POST /tasks/{id}/toggle`). No web framework.
- **Storage:** SQLite via `modernc.org/sqlite` (pure-Go, cgo-free). WAL mode,
  `busy_timeout=5000`, `foreign_keys=1`, pool pinned to `SetMaxOpenConns(1)` to
  serialize writes.
- **Templates / assets:** `html/template` parsed from an `embed.FS` (`//go:embed
  all:templates`) — single static binary, no runtime asset dir.
- **Frontend:** server-rendered HTML + HTMX 2.0.3 (loaded from unpkg CDN). Mutations
  return the `#task-list` fragment for HTMX swap.
- **Config:** environment variables with defaults — `DB_PATH` (default
  `data/app.db`), `ADDR` (default `:8080`).
- **Packaging:** multi-stage Dockerfile (`golang:1.26-alpine` build →
  `alpine:3.20` runtime, non-root uid 10001, `/healthz` HEALTHCHECK). `compose.yaml`
  maps host `8090` → container `8080`.
- **Build / test:** `go build ./...`, `go test ./...`.

## Stack (intent — not yet in the tree)
- **LLM:** default to the latest, most capable Claude models for ticket structuring
  and enrichment.
- **Knowledge base:** document ingestion + retrieval (RAG-style grounding over
  per-project docs). Needs a knowledge/embedding index — engine TBD; SQLite is the
  current relational store for projects/tasks/sub-tasks.

## What aido must do technically
- **Project knowledge base** — ingest uploaded docs (workflow, use cases, tech stack,
  modules) into per-project foundational knowledge aido retrieves when preparing a
  task.
- **AI structuring** — take a raw ticket + project knowledge, output an organized,
  EARS-shaped, domain-aware, enriched task. LLM-backed.
- **Tasks / sub-tasks** — persist projects, tasks, sub-tasks, and their state
  (pick up → in progress → complete). Task CRUD exists today.
- **Grounding discipline** — enrichment drawn from the project's docs and accumulated
  knowledge, not free-form generation. Cite/trace the source where possible.

## Invariants (do not break without a recorded decision)
- Single static binary — templates and assets stay embedded, no runtime file deps
  beyond the SQLite database file.
- Writes stay serialized (single open conn); do not raise the pool without a
  concurrency-safety decision.
- Original ticket intent is preserved through restructuring.
- Output requirements are valid EARS syntax and testable.
- Enrichment is grounded in project knowledge, never hallucinated project facts.
<!-- specd:managed:steering/tech.md:v3 end -->
