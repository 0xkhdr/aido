<!-- specd:managed:steering/tech.md:v3 begin -->
# Steering: Tech

> Stack is being decided as aido develops. This records intent and constraints the
> harness reads before proposing changes. Concrete choices get promoted to memory.md
> once verified.

## What aido must do technically
- **Project knowledge base** — ingest uploaded docs (workflow, use cases, tech stack,
  modules) into per-project foundational knowledge aido can retrieve when preparing a
  task. Implies document ingestion + retrieval (RAG-style grounding over project docs).
- **AI structuring** — take a raw ticket + project knowledge, output an organized,
  EARS-shaped, domain-aware, enriched task. LLM-backed.
- **Tasks / sub-tasks** — persist projects, tasks, sub-tasks, and their state
  (pick up → in progress → complete).
- **Grounding discipline** — enrichment is drawn from the project's docs and
  accumulated knowledge, not free-form generation. Cite/trace the source of enrichment
  where possible so it can be audited.

## Stack (intent — fill concrete choices as decided)
- **LLM:** default to the latest, most capable Claude models for ticket structuring
  and enrichment.
- **Language / runtime:** <TBD>
- **Storage:** needs project docs, knowledge index (embeddings/vector), and relational
  data for projects/tasks/sub-tasks. <TBD engine>
- **Build / test:** <TBD>

## Invariants (do not break without a recorded decision)
- Original ticket intent is preserved through restructuring.
- Output requirements are valid EARS syntax and testable.
- Enrichment is grounded in project knowledge, never hallucinated project facts.
<!-- specd:managed:steering/tech.md:v3 end -->
