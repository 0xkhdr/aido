# Requirements — aido-config

Foundation of aido-core: the Go module, the `.aido/` path contract, loading and
validating `.aido/config.yaml`, resolving provider API keys without ever writing
one into a tracked file, and the atomic write primitive every later package
depends on.

Source material: `aido-blueprint-v1.0.md` §4.1, §4.3, §4.4, §4.5.
Authority: `.specd/steering/{product,tech,structure,workflow,reasoning}.md`.

## R1 — Module and path contract

owner: unassigned (unattended run; see APPROVALS.md)
priority: must
risk: medium

- R1.1: When the repository is built, the system shall compile as a single Go module targeting Go 1.22 or newer with `CGO_ENABLED=0` and no cgo dependency.
- R1.2: When any package needs a path under `.aido/`, the system shall obtain it from the config package's exported path constructors rather than by string concatenation at the call site.
- R1.3: When a caller requests a `.aido/` path, the system shall return the path defined by the on-disk contract in `structure.md` S1 without creating it as a side effect.

## R2 — Config loading

owner: unassigned
priority: must
risk: medium

- R2.1: When `.aido/config.yaml` exists and parses as YAML, the system shall return a config value populated from it.
- R2.2: When `.aido/config.yaml` is absent, the system shall report a distinct not-found condition that a caller can test for, and shall not substitute a default config.
- R2.3: When `.aido/config.yaml` contains YAML that does not parse, the system shall return an error naming the file and the parse position.
- R2.4: When `.aido/config.yaml` omits an optional key, the system shall leave the corresponding field at its zero value rather than failing.

## R3 — Config validation

owner: unassigned
priority: must
risk: high

- R3.1: When a loaded config omits `project` or `tracked_branch`, the system shall report a validation error naming the missing key.
- R3.2: When a loaded config names an `llm.default_provider` with no entry under `llm.providers`, the system shall report a validation error naming the unknown provider.
- R3.3: When a loaded config names a provider outside the set `openai`, `anthropic`, `mistral`, `nvidia_nim`, `openrouter`, `ollama`, the system shall report a validation error naming the unsupported provider.
- R3.4: When a loaded config declares a `required_docs` entry whose path does not start with `okf/`, the system shall report a validation error naming the offending entry.
- R3.5: When validation finds more than one failure, the system shall report every failure found rather than stopping at the first.

## R4 — API key resolution

owner: unassigned
priority: must
risk: critical

- R4.1: When a provider key is requested and its `api_key_source` names an environment variable that is set and non-empty, the system shall return that value.
- R4.2: When that environment variable is unset or empty and `.aido/.secrets.yaml` holds a key for the provider, the system shall return the value from `.aido/.secrets.yaml`.
- R4.3: When neither source yields a key and `api_key_source` is not `none`, the system shall report a distinct not-found condition naming the provider and the sources consulted.
- R4.4: When `api_key_source` is `none`, the system shall return an empty key and no error.
- R4.5: When the system emits any error or log line during key resolution, the system shall exclude the key value and every substring of it from that output.
- R4.6: When a code path would write a resolved key to disk, the system shall refuse unless the target is `.aido/.secrets.yaml` and that path is confirmed git-ignored first.

## R5 — Atomic writes

owner: unassigned
priority: must
risk: high

- R5.1: When the system writes any file under `.aido/`, the system shall write to a temporary file in the same directory and rename it into place.
- R5.2: When a write fails before the rename, the system shall leave any pre-existing file at the destination byte-for-byte unchanged.
- R5.3: When a write fails, the system shall remove the temporary file it created.
- R5.4: When the system writes `.aido/.secrets.yaml`, the system shall create it with file mode `0600`.

## R6 — CLI surface

owner: unassigned
priority: should
risk: low

- R6.1: When a user runs `aido config show`, the system shall print the loaded config's non-secret values and exit zero.
- R6.2: When `aido config show` hits a load or validation failure, the system shall print the failure to stderr and still exit zero, because aido reports and does not block (`product.md` P5).
- R6.3: When `aido config show` prints a provider entry, the system shall print that provider's `api_key_source` and shall never print a resolved key value.

## Edge and failure behavior

- `.aido/` absent entirely: reported as the same not-found condition as R2.2, not
  as an I/O error.
- `.aido/.secrets.yaml` absent: not an error; key resolution falls through to the
  not-found condition in R4.3.
- `.aido/.secrets.yaml` present but unparseable: an error, and the error text
  names the file without quoting its contents (R4.5).
- `.aido/config.yaml` present but empty: parses to a zero config, then fails
  validation under R3.1 with both missing keys reported.
- A provider entry with neither `api_key_source` nor `base_url`: validation error
  naming the provider.

## Non-goals

- OS keyring resolution (blueprint §4.5 step 3). Needs a dependency outside the
  `tech.md` T1 allowlist; deferred to its own spec with an ADR.
- Interactive key prompting (blueprint §4.5 step 4). Deferred with keyring.
- Writing or scaffolding `.aido/` (an `aido init` command). This spec reads and
  validates; creation is a later spec.
- Any LLM call. Routing, adapters, and model selection are deferred
  (`PROGRAM.md`).
- `coding_agent` config semantics. The keys are parsed and preserved; acting on
  them belongs to the agent-bridge spec.
