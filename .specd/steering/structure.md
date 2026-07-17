<!-- specd:managed:steering/structure.md:v3 begin -->
# Steering: Structure

> Real code has landed (Go task-CRUD foundation). This maps the current tree plus the
> planned aido domains, so tasks can target files instead of scanning. Update as more
> of the AI/knowledge layers land; promote settled conventions to memory.md.

## Current layout (in the tree)
- **main.go** — entrypoint: reads `DB_PATH`/`ADDR` env, opens + migrates the store,
  wires handlers, runs the `net/http` server with graceful shutdown on SIGINT/SIGTERM.
- **internal/db/** — `db.go`: `Store` over `modernc.org/sqlite`, `Task` model,
  `Open` (pragmas, single conn), `Migrate`, and task queries (`ListTasks`, `AddTask`,
  `ToggleTask`, `DeleteTask`).
- **internal/handlers/** — `handlers.go`: `Handler` bundling store + parsed templates,
  `Routes` mux, and per-route handlers; `/healthz`. Renders full page or the
  `#task-list` HTMX fragment.
- **internal/handlers/templates/** — `index.html` (full page) and `list.html`
  (task-list fragment), embedded via `embed.FS`.
- **data/** — runtime SQLite database file (`app.db`, WAL). Not source.
- **Dockerfile / compose.yaml / .dockerignore** — container packaging.
- **.specd/** — specd harness: specs, roles, steering, skills.

## Planned layout (aido AI domains, not yet built — by domain concern)
- **projects/** — Project entity: create project, own uploaded docs, own accumulated
  knowledge base.
- **docs/ (ingestion)** — upload, parse, and index project docs (workflow, use cases,
  tech stack, modules) into the per-project knowledge base.
- **knowledge/** — foundational knowledge store + retrieval (grounding source for
  enrichment). Embeddings/index live here.
- **tasks/** — Task and Sub-task entities and state. Today this lives in
  `internal/db` + `internal/handlers`; promote/split as sub-tasks + project scoping land.
- **structuring/ (AI)** — the aido core: raw ticket + retrieved project knowledge →
  organized, EARS-shaped, domain-aware, enriched task. LLM calls isolated here.

## Naming & patterns
- Non-entrypoint code lives under `internal/` (unexported module surface).
- One domain concern per package; cross-concern logic goes through explicit
  interfaces, not shared globals.
- LLM/prompt code stays inside `structuring/` — no LLM calls scattered across entities.
- Enrichment always carries a trace to its knowledge source (auditable grounding).
- DB access is confined to the store package; handlers call the store, never SQL.
- Mutating routes return the `#task-list` fragment for HTMX swap; GET renders the page.
- Tests colocated with source (Go `_test.go` convention).

## Spec authoring format
- `design.md` decision contract: declare `references:` (the `R<n>` requirements it
  traces to), plus `boundaries:`, `interfaces:`, `invariants:`, `failure:`,
  `integration:`, `alternatives:`, `disposition:`, and `owner:`. An unknown
  reference is always refused; the full contract is required under the production
  profile.
- `tasks.md` optional trace/risk columns: `refs`, `kind`, `risk`, `complexity`,
  `capabilities`, `context`, `evidence`, `checks`. The six required columns alone are
  a valid table; the rest may be omitted — the production planning profile requires
  the full set.
<!-- specd:managed:steering/structure.md:v3 end -->
