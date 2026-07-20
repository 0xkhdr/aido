# Requirements — aido-documentary-agent

## R1 — Project onboarding

owner: product owner
priority: must
risk: medium

- R1.1: When a developer initializes Aido in a Git repository, the system shall create a repository-readable project workspace without overwriting existing project knowledge.
- R1.2: When initialization detects the repository's default branch, the system shall propose it as the tracked branch and require human confirmation before saving it.
- R1.3: When required foundational documents are missing, the system shall identify every missing document, provide a usable template for each, and report the project as not ready without blocking document creation.
- R1.4: When all required foundational documents exist, the system shall record the tracked revision and report the project as ready.
- R1.5: When a developer selects language-model and coding-agent preferences, the system shall persist the non-secret configuration for subsequent sessions.

## R2 — Repository-native knowledge

owner: product owner
priority: must
risk: high

- R2.1: When Aido stores project knowledge or operational state, the system shall use human-readable, Git-trackable documents inside the project workspace, except for secrets explicitly excluded from version control.
- R2.2: When Aido retrieves project context, the system shall resolve explicit document paths, references, and section headings deterministically.
- R2.3: When a referenced document or section does not exist, the system shall identify the unresolved reference and continue with the remaining available context.
- R2.4: While retrieving project context, the system shall not require embeddings, similarity search, or vector storage.

## R3 — Request ingestion and normalization

owner: product owner
priority: must
risk: high

- R3.1: When a developer submits a supported request source, the system shall preserve the source identity and raw request while normalizing its content into one common processing flow.
- R3.2: When a request lacks enough accessible content to determine intent, the system shall present the missing information as an open question rather than inventing it.
- R3.3: When multiple requests are stored, the system shall assign each a stable, unique identifier and shall not overwrite an existing request.

## R4 — Context selection

owner: product owner
priority: must
risk: high

- R4.1: When processing a request, the system shall read the configured foundational documents and select additional context only through explicit request terms, links, references, or headings.
- R4.2: When context is selected for a request, the system shall record explicit request-to-document references that a human or coding agent can resolve without Aido.
- R4.3: When no relevant project context is available, the system shall disclose that limitation in the generated specification.

## R5 — Specification generation and review

owner: product owner
priority: must
risk: high

- R5.1: When sufficient request and project context is available, the system shall produce a human-readable specification containing context, domain analysis, related documents, uniquely identified testable EARS requirements, open questions, and expected implementation and documentation impacts.
- R5.2: When generation completes, the system shall store the specification under its stable request identifier and await human confirmation, editing, or override.
- R5.3: While a specification has unresolved open questions, the system shall preserve them visibly and shall not represent assumptions as confirmed requirements.
- R5.4: When a human edits or overrides a generated specification, the system shall treat the saved human-authored content as authoritative.

## R6 — Model-provider configuration and routing

owner: product owner
priority: must
risk: high

- R6.1: When a configured AI task is invoked, the system shall route it to the model selected for that task, or to the configured default when no task-specific selection exists.
- R6.2: When a selected provider or model is unavailable, the system shall report the failed provider and task without silently substituting an unconfigured model.
- R6.3: When provider credentials are required, the system shall resolve them in the documented precedence order and shall never write credential values to repository-tracked configuration, requests, links, or witness logs.
- R6.4: When no credential can be resolved, the system shall request one interactively and store it only in an explicitly version-control-excluded secret store after user consent.
- R6.5: When a local provider requiring no credential is selected, the system shall operate without prompting for an API key.

## R7 — Coding-agent exploration bridge

owner: product owner
priority: should
risk: high

- R7.1: When request context remains unclear and a coding agent is configured, the system shall ask only bounded factual repository questions such as file listing, text matching, signature reading, or scoped history inspection.
- R7.2: When a coding agent returns exploration results, Aido shall perform all interpretation and specification synthesis itself.
- R7.3: When no coding agent is configured or the configured agent is unavailable, the system shall continue with accessible documents and expose the resulting knowledge gap as an open question.
- R7.4: When communicating implementation guidance to a coding agent, the system shall make the confirmed request specification and its explicit document links consumable through repository files without requiring a proprietary protocol.

## R8 — Implementation witnessing and synchronization

owner: product owner
priority: must
risk: high

- R8.1: When the tracked branch advances, the system shall record the previous and current revisions and the observed relevant document changes in an append-only, timestamped witness record.
- R8.2: When an implementation is reported for a request, the system shall associate its revisions and declared document changes with that request without claiming to have authored the implementation.
- R8.3: When implementation revisions change every document linked to the request that requires an update, the system shall record the documentation as current.
- R8.4: When implementation revisions do not change a linked document, the system shall issue an informational review flag and shall not block or alter the implementation.
- R8.5: When synchronization is disabled, the system shall not monitor or mutate branch state automatically.

## R9 — Human authority and product boundaries

owner: product owner
priority: must
risk: critical

- R9.1: When Aido proposes a specification, readiness assessment, or documentation flag, the system shall leave confirmation, editing, override, and follow-up action to the human.
- R9.2: While operating as Aido, the system shall not write application implementation code, manage sprint state, approve its own output, or enforce documentation changes.
- R9.3: When a recoverable document, provider, agent, or synchronization failure occurs, the system shall preserve existing repository knowledge and report an actionable error without corrupting prior requests, links, configuration, or witness history.

## Edge and failure behavior

- Initialization outside a Git repository shall fail before creating project state and shall explain the required precondition.
- Invalid configuration, duplicate request identifiers, malformed references, and unsupported request sources shall be rejected or surfaced before existing data is mutated.
- Interrupted writes shall leave the last valid repository-readable state recoverable; append-only witness history shall not be rewritten.
- Content received from tickets, messages, transcripts, documents, coding agents, or model providers shall be treated as untrusted project data and shall not override human authority or secret-handling rules.
- Empty requests and inaccessible external sources shall produce an actionable input error or open question, not an invented specification.
- Concurrent processing shall not allocate the same request identifier or lose an existing request-to-document link.

## Non-goals

- Writing or modifying application implementation code.
- Managing sprints, assignments, estimates, approvals, or enforcement workflows.
- Replacing human review or judgment.
- Semantic or vector-based retrieval, embeddings, or vector-database operation.
- Requiring a coding agent for normal request processing or specification synthesis.
- Automatically modifying foundational project documents in response to an implementation.
- Guaranteeing that an implementation is correct; witnessing reports repository evidence and documentation freshness only.
