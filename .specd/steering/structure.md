<!-- specd:managed:steering/structure.md:v3 begin -->
# Steering: Structure

> No aido application code exists yet — repo holds only the `.specd` skeleton. This is
> the **planned** layout by domain, so tasks can target files instead of scanning.
> Update to the real tree as code lands; promote settled conventions to memory.md.

## Planned layout (by domain concern)
- **projects/** — Project entity: create project, own uploaded docs, own accumulated
  knowledge base.
- **docs/ (ingestion)** — upload, parse, and index project docs (workflow, use cases,
  tech stack, modules) into the per-project knowledge base.
- **knowledge/** — foundational knowledge store + retrieval (grounding source for
  enrichment). Embeddings/index live here.
- **tasks/** — Task and Sub-task entities: persistence and state (pick up → in
  progress → complete), decomposition into sub-tasks.
- **structuring/ (AI)** — the aido core: take raw ticket + retrieved project knowledge,
  emit organized, EARS-shaped, domain-aware, enriched task. LLM calls isolated here.
- **api/** or **cli/** — entry surface a developer uses to feed tickets and pick tasks.

## Naming & patterns
- One domain concern per directory; cross-concern logic goes through explicit
  interfaces, not shared globals.
- LLM/prompt code stays inside `structuring/` — no LLM calls scattered across entities.
- Enrichment always carries a trace to its knowledge source (auditable grounding).
- Tests colocated with source per language convention (TBD once stack chosen).

## Spec authoring format

## Spec authoring format
- `design.md` decision contract: declare `references:` (the `R<n>` requirements it
  traces to), plus `boundaries:`, `interfaces:`, `invariants:`, `failure:`,
  `integration:`, `alternatives:`, `disposition:`, and `owner:`. An unknown
  reference is always refused; the full contract is required under the production
  profile.
- `tasks.md` optional trace/risk columns: `refs`, `kind`, `risk`, `complexity`,
  `capabilities`, `context`, `evidence`, `checks`. Legacy six-column tables keep working (backward compatible);
  the production planning profile requires the full set.
<!-- specd:managed:steering/structure.md:v3 end -->
