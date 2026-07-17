<!-- specd:managed:steering/product.md:v3 begin -->
# Steering: Product

## Thesis
- **What this product is:** aido — an AI-assisted todo/ticket tool that turns raw,
  messy, or incomplete technical Jira tickets into structured, project-aware tasks
  written in EARS syntax.
- **Who it is for:** engineering teams (dev + tech lead) who write and consume Jira
  tickets. Job hired for: take a half-formed ticket and produce a well-organized,
  domain-aware, workflow-consistent task a developer can pick up and execute.

## Core value
- A ticket comes in unorganized, not in EARS syntax, missing project-related context.
  aido "aids" — it rewrites the ticket into EARS-shaped requirements, organizes it,
  and enriches it with project knowledge and the related workflow.
- The differentiator is **project-grounded understanding**: aido does not just reword
  text; it understands the task's domain inside the project and the workflow it
  touches, then enriches the ticket with accumulated project knowledge.

## Domain model
- **Project** — top-level container. Holds uploaded docs and accumulated foundational
  knowledge (workflow, use cases, tech stack, main modules, domain).
- **Task** — a ticket inside a project. Structured, EARS-shaped, enriched with project
  context.
- **Sub-task** — a task may decompose into sub-tasks; sub-tasks carry the same
  structure and inherit project context.
- **Project docs** — user-uploaded material describing the project's workflow, use
  cases, tech stack, and modules. aido ingests these to build the project's
  foundational knowledge base and domain understanding.

## Product workflow (runtime, user-facing)
1. User creates a **project** and uploads docs (workflow, use cases, tech stack,
   modules, any context aido may need).
2. aido ingests the docs into a **foundational knowledge base** for that project and
   its domain.
3. A raw/technical Jira ticket is fed in — possibly unorganized, non-EARS, missing
   context.
4. aido produces a **structured task**: EARS syntax, organized, domain-aware, enriched
   with related workflow and accumulated project knowledge. Decomposes into sub-tasks
   where useful.
5. Project knowledge accumulates over time; later tickets get richer enrichment.
6. **Developer** picks tasks to work on and moves them to complete.

## Principles
- Never lose the ticket's original intent while restructuring — enrich, do not invent
  requirements the ticket did not imply.
- Enrichment must be grounded in the project's uploaded docs and accumulated
  knowledge, not in generic assumptions.
- Output requirements adhere to EARS syntax and are testable and unambiguous.
- Boundary — aido is not a Jira replacement or a project-management suite. It is the
  intelligence layer that structures and enriches tickets against project context.

specd's own thesis, for reference: **Agent = Model + Harness.** The harness makes the
plan safely delegable; every harness decision is deterministic and evidence-backed.
<!-- specd:managed:steering/product.md:v3 end -->
