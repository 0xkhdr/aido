# Design — aido-documentary-agent-v2

- references: R1, R1.1, R1.2, R1.3, R1.4, R1.5, R2, R2.1, R2.2, R2.3, R2.4, R3, R3.1, R3.2, R3.3, R4, R4.1, R4.2, R4.3, R5, R5.1, R5.2, R5.3, R5.4, R6, R6.1, R6.2, R6.3, R6.4, R6.5, R7, R7.1, R7.2, R7.3, R7.4, R8, R8.1, R8.2, R8.3, R8.4, R8.5, R9, R9.1, R9.2, R9.3
- disposition: accepted
- owner: product owner

## Boundaries

- **Command boundary:** exposes project initialization, request ingestion, explicit synchronization, and status inspection. It gathers human choices but does not approve specifications or implementation work. References: R1, R3, R8.5, R9.
- **Workspace boundary:** owns `.aido/` creation, validated document reads, atomic mutable-document writes, append-only witness writes, and stable request-ID allocation. It does not modify application source. References: R1.1, R2, R3.3, R8, R9.2, R9.3.
- **Context boundary:** resolves configured documents and heading fragments, records unresolved references, and returns source-labelled text. It performs no semantic or vector retrieval. References: R2.2–R2.4, R4.
- **Ingestion boundary:** adapters convert a source into `{source, raw_request}`; all subsequent analysis uses the same normalized request path. External content remains untrusted data. References: R3.1, R3.2, R9.3.
- **Synthesis boundary:** Aido constructs the model input from normalized request data, deterministic context, and optional factual exploration results; it validates the returned specification shape before storage. References: R4, R5, R7.2.
- **Provider boundary:** selects configured task models, resolves credentials, invokes providers, and returns explicit failures. It owns no project truth. References: R6.
- **Agent boundary:** exposes only allow-listed, bounded factual inquiries and returns raw results to synthesis. Coding-agent implementation remains outside Aido. References: R7, R9.2.
- **Witness boundary:** compares declared implementation revisions and linked documents, then appends observations or informational flags. It never gates or changes implementation. References: R8, R9.
- **Deferred implementation boundary:** programming language, CLI framework, YAML library, provider SDKs, filesystem locking mechanism, and agent transports remain implementation choices because the approved architecture does not select a runtime.

## Interfaces

- Aido's public contracts are repository files plus four bounded operations: initialize, ingest/generate, inquire, and synchronize. References: R1–R8.

### Repository contracts

- `.aido/config.yaml` contains `project`, `tracked_branch`, `last_sync_commit`, `required_docs`, `llm.default_provider`, `llm.default_model`, provider connection metadata and credential-source references, task-to-model routes, configured coding agents, the active agent, and `auto_sync`. Credential values are forbidden. References: R1.2, R1.4, R1.5, R6.1, R6.3, R8.5.
- `.aido/.gitignore` includes `.secrets.yaml`; `.aido/.secrets.yaml` is an optional local fallback credential map and is never required for credential-free providers. References: R6.3–R6.5.
- `.aido/docs/` contains required foundational documents configured by relative path. Initial defaults are `architecture.md`, `domain-model.md`, `glossary.md`, `api-contracts.md`, and `operations.md`; optional decisions live under `docs/adr/`. References: R1.3, R1.4, R2.1.
- `.aido/requests/req-NNN.md` contains YAML front matter with stable identity, source, title, raw request, review/implementation status, implementer, and revisions; its body contains Context, Domain Analysis, Related Documents, Specification (EARS), Open Questions, and Implementation Notes. References: R3, R5, R7.4, R8.2.
- `.aido/links.yaml` maps each request ID to a deduplicated list of repository-relative document paths with optional normalized heading fragments. References: R4.2, R8.3, R8.4.
- `.aido/witness/YYYY-MM-DD.log` contains newline-delimited UTC observations. Each record starts with an RFC 3339 timestamp and one event type: `SYNC`, `IMPLEMENTATION`, or `WITNESS`; payloads contain stable request IDs, revisions, and repository-relative document paths. References: R8.
- `.aido/templates/ears.md` supplies the request-specification structure without embedding project facts. References: R1.1, R5.1.

### Behavioral contracts

- `init(project, repo)` validates Git context, proposes the default branch, collects human configuration choices, creates only absent workspace entries, checks required documents, and records readiness after confirmation. References: R1.
- `ingest(source)` returns preserved source identity and raw text or an actionable source error; it never allocates an ID or writes request state before validation succeeds. References: R3, R9.3.
- `resolve(reference)` returns resolved path, optional heading, source-labelled content, and resolution status. Missing paths/headings are non-fatal to other resolutions. References: R2.2, R2.3, R4.
- `generate(normalized_request, context, exploration?)` returns a structurally validated draft or an explicit provider/validation error. Drafts retain uncertainties as open questions. References: R3.2, R5, R6.2, R7.2.
- `save_draft(draft)` atomically allocates the next available `req-NNN`, writes the request, and updates links without overwriting existing IDs or losing links under concurrent calls. References: R3.3, R4.2, R5.2, R9.3.
- `inquire(operation, scope)` accepts only `list_files`, `grep`, `read_signatures`, and scoped `git_log` operations; rejects paths outside the target repository and returns raw bounded output. References: R7.1–R7.3.
- `sync(previous, current)` validates revisions, determines linked-document changes, appends witness records, and advances `last_sync_commit` only after the complete observation is durable. References: R8, R9.3.

## Invariants

- Existing project knowledge is never overwritten during initialization; mutable structured files are replaced atomically only after validation. References: R1.1, R9.3.
- Repository-tracked files contain no credential values; credential material is never included in prompts, errors, request files, links, or witness records. References: R6.3, R6.4.
- Every stored request has one immutable ID, preserved source identity and raw input, resolvable explicit links, and human-visible uncertainty. References: R3, R4.2, R5.
- Human-saved request documents are authoritative. Regeneration creates a proposed revision and never silently replaces confirmed human content. References: R5.4, R9.1.
- All model interpretation occurs inside Aido; coding-agent output is untrusted factual input only. References: R7.1, R7.2.
- Witness history is append-only, ordered by UTC timestamp, and informational. A missing documentation change cannot fail implementation or mutate application state. References: R8, R9.1, R9.2.
- All filesystem paths are repository-relative after normalization and containment checks; external inputs cannot select paths outside the project workspace. References: R2, R7.1, R9.3.
- No embeddings, vector index, sprint management, application-code writing, or self-approval path exists in an accepted Aido workflow. References: R2.4, R9.2.

## Failure

- **Invalid location/configuration:** validate Git repository, schema, path containment, provider/task references, and required fields before mutation; report every actionable validation error together. References: R1, R6, R9.3.
- **Missing document/heading:** retain an unresolved-reference result, continue other deterministic reads, and include the gap in the draft. References: R2.3, R4.3.
- **Unsupported, empty, or inaccessible source:** preserve no partial request, return a source-specific error, and allow the developer to supply text directly. References: R3.1, R3.2, R9.3.
- **Provider unavailable or malformed output:** identify provider, model, and task; do not silently reroute; store no invalid draft. Existing project files remain unchanged. References: R5.1, R6.2, R9.3.
- **Credential unavailable:** follow environment, ignored secret file, optional keyring, then consensual prompt. Cancellation returns a non-mutating error. References: R6.3–R6.5.
- **Agent unavailable/invalid inquiry:** skip exploration, record the knowledge gap, and continue document-grounded synthesis when possible. Reject non-allow-listed operations or escaping paths before invocation. References: R7.1, R7.3.
- **Concurrent or interrupted write:** use exclusive creation for request IDs and atomic replacement for structured mutable files; retry allocation after a collision. Append witness records as complete single lines under a writer lock. References: R3.3, R8.1, R9.3.
- **Invalid sync range/history:** append no observation and do not advance the sync revision; report the exact missing or non-ancestor revision. References: R8.1, R9.3.

## Integration

- **Git:** repository detection, default/tracked branch selection, revision validation, scoped history, and changed-path comparison. All Git operations are local and non-destructive. References: R1, R7.1, R8.
- **Model providers:** a small common request/response boundary supports configured remote or local providers. Provider-specific authentication and endpoints stay behind that boundary; automatic fallback is excluded unless later approved. References: R6.
- **Credential sources:** environment variables have highest precedence, then `.aido/.secrets.yaml`, optional OS keyring, then consensual interactive entry. References: R6.3–R6.5.
- **Request sources:** Jira, Slack, email, transcripts, and pasted text are adapters into the normalized ingestion contract; the first implementation need only include sources selected in tasks. References: R3.1.
- **Coding agents:** MCP, CLI, or another configured transport may implement the same factual inquiry contract. Repository request and link files remain the implementation handoff. References: R7.
- **Filesystem:** UTF-8 Markdown, YAML, and newline-delimited logs are the durable interoperability layer. References: R2.1, R5.2, R7.4, R8.

## Alternatives

- **Accepted — direct document retrieval:** deterministic paths and headings are inspectable, cheap, and satisfy repository-native truth. Vector retrieval is rejected by R2.4.
- **Accepted — one normalized ingestion pipeline:** source adapters stop at preserved raw text, avoiding source-specific specification logic. References: R3.1.
- **Accepted — repository files for agent handoff:** works without a live bridge and keeps the human as the required coordination point. References: R7.4, R9.1.
- **Accepted — append-only witnessing:** preserves auditability while keeping flags non-blocking. An enforcement/status workflow is rejected by R8.4 and R9.
- **Deferred — implementation language and libraries:** choose during task planning after the repository establishes a runtime; adding a framework or dependency before that would be speculative.
- **Deferred — OS keyring:** optional behind the credential-source contract; environment and ignored-file sources satisfy the initial workflow. References: R6.3.
- **Deferred — automatic synchronization:** configuration retains the switch, but explicit sync is sufficient until a background lifecycle and ownership model are approved. References: R8.5.
- **Deferred — full set of external-source and coding-agent adapters:** implement pasted/freeform input and one useful agent bridge first; add adapters only when selected use cases require them.
- **Rejected — automatic provider fallback:** it could change cost, privacy, or model behavior without human configuration. References: R6.2, R9.1.

## Verification

- **Workspace contract:** initialize in a temporary Git repository containing sentinel `.aido/` content; assert required absent files are created, sentinels remain unchanged, missing-doc readiness is reported, and a second initialization is idempotent. Proves R1 and repository invariants.
- **Retrieval contract:** resolve complete files, valid headings, missing files, missing headings, traversal attempts, and similarly named headings; assert deterministic source-labelled results and no vector dependency. Proves R2 and R4.
- **Ingestion/spec contract:** process pasted text plus representative adapter fixtures; assert raw preservation, unique IDs under concurrent allocation, complete EARS sections, visible questions, explicit links, atomic failure, and preservation of human edits. Proves R3 and R5.
- **Provider/security contract:** use fake providers and isolated credential sources to assert routing, precedence, credential-free local operation, cancellation behavior, explicit provider failure, and absence of secret values from every tracked artifact and diagnostic. Proves R6 and R9.3.
- **Agent contract:** use a fake bridge to assert the allow-list, repository containment, bounded raw results, synthesis ownership, and graceful unavailable-agent behavior. Proves R7 and R9.2.
- **Witness contract:** use a temporary Git history to assert changed-path comparison, request association, all-current and missing-doc observations, disabled synchronization, invalid revision handling, atomic sync cursor advancement, and append-only history. Proves R8 and R9.
- **Boundary audit:** inspect commands and integration tests to establish that Aido exposes no application-writing, sprint-management, enforcement, or self-approval behavior. Proves R9.

## Deployment

- Deliver first as an explicitly invoked local tool against a developer-selected Git repository; default `auto_sync` to false.
- Create or migrate `.aido/` only after a dry validation pass and show the developer the files and branch that will be tracked.
- Introduce provider and agent integrations behind configuration, with credential-free fixture implementations used by verification.
- Observe initialization failures, generation failures, unresolved references, sync outcomes, and informational flags without recording request bodies, document bodies, or credentials in operational diagnostics.
- Product owner owns provider availability, required-document defaults, and supported adapter decisions; repository maintainers own project documents and confirmed specifications.

## Rollback

- Disable invocation or `auto_sync` to stop Aido activity; no resident service is required for the initial design.
- Restore mutable configuration, request, and link files through ordinary Git history. Never rewrite witness logs; append a corrective observation referencing the superseded entry.
- If an upgrade cannot read the existing workspace schema, fail before mutation and require the previous compatible version or an explicit future migration.
- Provider or agent integration failures roll back by disabling that configured integration; repository documents and confirmed request specifications remain usable directly.
