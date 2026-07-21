# Design — aido-config

- references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2, R2.3, R2.4, R3, R3.1, R3.2, R3.3, R3.4, R3.5, R4, R4.1, R4.2, R4.3, R4.4, R4.5, R4.6, R5, R5.1, R5.2, R5.3, R5.4, R6, R6.1, R6.2, R6.3
- disposition: accepted
- owner: unassigned (unattended run; see APPROVALS.md)
- boundaries: `internal/config` owns `.aido/` path construction, config and secrets parsing, validation, key resolution, and atomic writes; `cmd/aido` owns wiring and the `config show` output only. Creating `.aido/`, OKF parsing, query handling, LLM calls, keyring, and interactive prompts are excluded. See Boundaries.
- interfaces: Go `package config` exports `Root` with pure path constructors, `DirName`, `Load`, `Config.Validate`, `Config.ResolveKey`, `WriteSecrets`, `SupportedProviders`, `ErrKeyNotFound`, `ErrUnsupportedKeySource`, `ErrNotGitIgnored`, `ValidationError`, and `WriteFile`; the CLI exposes `aido config show`. Consumed on-disk contracts are `.aido/config.yaml` and `.aido/.secrets.yaml` per blueprint §4.3–§4.4. See Interfaces.
- invariants: I1 a resolved key never enters an error, log, or printed line; I2 no `.aido/` file is truncated in place, every write is temp-plus-rename; I3 path construction performs no I/O; I4 validation is total and reports every problem at once; I5 no network or LLM call in this package; I6 `cmd/aido` holds no validity decision. See Invariants.
- failure: missing config yields a wrapped `fs.ErrNotExist`; malformed YAML yields a parse error naming file and position; malformed `.secrets.yaml` yields an error naming the path with contents never quoted; validation yields one aggregate error; an unresolvable key yields `ErrKeyNotFound` naming provider and sources consulted; a failed write removes its temp file and leaves the destination unchanged. See Failure.
- integration: depends on the Go standard library, `gopkg.in/yaml.v3`, `github.com/go-git/go-git`, and `github.com/go-git/go-billy` (go-git's filesystem interface, reached only through it), builds with `CGO_ENABLED=0`, and is depended on by every later aido-core package through `Root`; no gRPC, MCP, or Workspace reference (P9, T12); no prior on-disk state exists to migrate. The external boundaries are the filesystem under `.aido/`, the process environment, and the `aido` CLI's stdout/stderr/exit code. Integration evidence: task T6 is an integration task carrying evidence `test/integration-cli-config-show`, which drives `aido config show` end to end against a real temp `.aido/` tree and asserts the CLI contract; error-path negative checks at the same boundary are planned in T6 (missing config, invalid config, key never printed) and in T4 (write failure leaves destination and directory unchanged). See Integration and Verification.
- alternatives: rejected a config library (T1/T2), eager key resolution at load (R6.3), `os.WriteFile` (T10), and path-based file-mode inference; deferred OS keyring and interactive prompt (dependency outside T1, so `reasoning.md` R3 makes it blocked rather than a default) and a shared provider-set constant with `internal/llm` (R4, R5). See Alternatives.

## Boundaries

- Owned: `internal/config/` — the only package in this spec. It owns `.aido/`
  path construction, `config.yaml` parse and validation, `.secrets.yaml` parse,
  key resolution, and the atomic write primitive.
- Owned: `cmd/aido/` — CLI entrypoint and the `config show` subcommand. Wiring
  and output formatting only; no validation or resolution logic lives here
  (`structure.md` S5).
- Owned: `go.mod` — module declaration, Go version, and the single dependency
  `gopkg.in/yaml.v3` (allowed by `tech.md` T1).
- Excluded: creating or scaffolding `.aido/` (no `aido init` here), OKF parsing,
  query handling, LLM calls, keyring, interactive prompts. `internal/config`
  hands parsed values to later packages and never grows a second responsibility.
- `internal/config` is the sole owner of `.aido/` path strings for this spec;
  `internal/okf`, `internal/query`, and `internal/witness` will each own their
  own subtree later (`structure.md` S6) but obtain the root from here.

## Interfaces

Go API, `package config`:

- `type Root string` — an `.aido/` directory. `func NewRoot(projectDir string) Root`
  returns `<projectDir>/.aido` without touching the filesystem (R1.3).
- `func (r Root) ConfigPath() string`, `SecretsPath() string`, `OKFDir() string`,
  `QueriesDir() string`, `LinksPath() string`, `WitnessDir() string`,
  `TemplatesDir() string` — the path constructors of R1.2, one per entry in the
  `structure.md` S1 tree. Pure string functions.
- `type Config struct` — `Project`, `TrackedBranch`, `LastSyncCommit`,
  `RequiredDocs []string`, `LLM LLMConfig`, `CodingAgent CodingAgentConfig`,
  `AutoSync bool`. YAML tags match blueprint §4.3 keys exactly; `CodingAgent` is
  parsed and preserved but unused (R6 non-goal).
- `func Load(r Root) (*Config, error)` — R2. Returns a wrapped `fs.ErrNotExist`
  when the file is absent, so callers use `errors.Is(err, fs.ErrNotExist)`
  (R2.2). Returns a parse error naming file and position on malformed YAML
  (R2.3). Does not validate.
- `func (c *Config) Validate() error` — R3. Returns `nil` or a `ValidationError`
  aggregating every failure (R3.5).
- `type ValidationError struct { Problems []string }` implementing `error`;
  `Unwrap() []error` is not provided — the aggregate is flat and its `Error()`
  joins problems with `"; "`.
- `func (c *Config) ResolveKey(r Root, provider string) (string, error)` — R4.
  Returns a wrapped `ErrKeyNotFound` naming provider and sources consulted
  (R4.3), or `ErrUnsupportedKeySource` when `api_key_source` is neither `none`
  nor an `env:NAME` reference — a distinct condition, because the key was never
  looked for rather than looked for and missing. The `Root` parameter is
  required: R4.2 falls back to `.aido/.secrets.yaml`, and `Root` is the sole
  owner of that path (I3, `structure.md` S6), so a method with only a provider
  name cannot reach it without violating this design's own boundary.
  *(Amended 2026-07-21 by operator ruling. The originally approved signature,
  `ResolveKey(provider string)`, could not satisfy R4.2 and was implemented with
  the `Root` parameter in T5; this records the deviation rather than leaving the
  approved artifact contradicting the code. See `APPROVALS.md`.)*
- `var ErrKeyNotFound = errors.New("api key not found")`.
- `var ErrUnsupportedKeySource = errors.New("unsupported api_key_source")`.
- `func WriteSecrets(r Root, secrets map[string]string) error` — **R4.6**, the
  only function in the package that writes a key. Refuses unless the target is
  `Root.SecretsPath()` — satisfied by construction, since the target is not a
  parameter — and unless that path is confirmed git-ignored first, returning
  `ErrNotGitIgnored` otherwise. Writes at mode `0600` (R5.4) through `WriteFile`.
- `var ErrNotGitIgnored = errors.New(...)` — the R4.6 refusal. Its message names
  the repository scope, because a refusal a user reads as wrong gets filed as a
  bug.
- `const DirName = ".aido"` and `func (r Root) String() string` — the directory
  name and the root path, exported so callers never spell either (R1.2).
- `var SupportedProviders []string` — the closed provider set R3.3 validates
  against, exported so a caller can present it.
- `func WriteFile(path string, data []byte, perm fs.FileMode) error` — R5. Temp
  file in the destination directory, `fsync`, `rename`, `os.Remove` of the temp
  on any failure before rename.

On-disk contracts consumed (not defined) here: `.aido/config.yaml` and
`.aido/.secrets.yaml` per blueprint §4.3/§4.4. Both are versioned public surfaces
under `tech.md` T13 — this spec reads them; it does not change their shape.

CLI: `aido config show` prints `project`, `tracked_branch`, `last_sync_commit`,
`required_docs`, `llm.default_provider`, `llm.default_model`, `auto_sync`, and
one line per provider giving name, `base_url`, and `api_key_source` verbatim
(R6.1, R6.3). Exit code is zero on every path (R6.2).

## Invariants

- **I1 (R4.5, R4.6, P7, T9).** A resolved key value never leaves `ResolveKey`'s
  return value. It is never placed in an error, never logged, never printed by
  the CLI. `ResolveKey` errors name the provider and the source *names*, never
  source contents.
- **I2 (R5.1, R5.2, T10).** No file under `.aido/` is ever opened for truncating
  write at its final path. A crash at any point leaves the destination either
  fully old or fully new.
- **I3 (R1.3).** Path construction is pure. No function returning a path performs
  I/O, and no `.aido/` directory is created as a side effect of a read path.
- **I4 (R3.5).** `Validate` is total — it inspects every rule against the whole
  config before returning, so the problem list is complete on the first call.
- **I5 (T7, P5).** No LLM call and no network call exists in this package. Every
  exported function is a pure function of its arguments plus on-disk state and
  environment.
- **I6 (S5).** `cmd/aido` contains no branch that decides validity; it calls
  `Load`, `Validate`, and prints. Moving `cmd/aido` to another main package must
  not change behaviour.

## Failure

- Missing `.aido/config.yaml` → wrapped `fs.ErrNotExist` (R2.2). Contained: the
  caller decides. `aido config show` prints "no .aido/config.yaml found" to
  stderr and exits zero (R6.2). Recovery is user-side; a later `aido init` spec
  covers creation.
- Malformed `config.yaml` → parse error naming file and `yaml.v3`'s reported
  line/column (R2.3). Contained to `Load`; no partial config is returned.
- Malformed `.secrets.yaml` → error naming the path only. The file's contents are
  never quoted into the error, including the parse excerpt `yaml.v3` would
  normally attach — the message is rebuilt, not wrapped (I1, R4.5).
- Validation failures → one `ValidationError` listing all problems (R3.5).
  Non-fatal to the process; `product.md` P5 means aido reports and continues.
- `api_key_source` neither `none` nor `env:NAME` → `ErrUnsupportedKeySource`
  naming the provider and the expected forms. The offending value is never
  quoted: a user who pasted a literal key where a reference belongs is exactly
  the person the message must not echo (I1, R4.5).
- Target of a key write not ignored by any rule *inside the repository* →
  `ErrNotGitIgnored` (R4.6). Machine-global ignore sources — `/etc/gitconfig`,
  `~/.gitconfig`, `~/.config/git/config`, `$XDG_CONFIG_HOME/git/ignore` — are
  deliberately not consulted: a `.secrets.yaml` protected only by the current
  machine is unprotected in every other clone, which is not what R4.6 asks. A
  file already tracked in the index is likewise never treated as ignored,
  matching git's own precedence.
- Key absent from every source → `ErrKeyNotFound` wrapped with provider and the
  list of source names consulted (R4.3). Distinct from an I/O error so a caller
  can offer the user a fix.
- Atomic write failure → temp file removed (R5.3), destination untouched (R5.2),
  error returned naming the destination. A leaked temp file after a hard kill is
  acceptable and is not cleaned up on next run; it is inert (`.tmp` suffix, not a
  path any reader constructs).
- `.secrets.yaml` written with mode `0600` (R5.4); `WriteFile` takes the mode
  from its caller rather than inferring it from the path, so the guarantee is at
  the one call site that writes secrets, not in a filename heuristic.

## Integration

- Depends on: Go stdlib, `gopkg.in/yaml.v3`, and `github.com/go-git/go-git` —
  all on the `tech.md` T1 allowlist. go-git is used for R4.6's git-ignore check
  only, and only its `plumbing/format/{gitignore,index}` packages plus
  `go-billy/osfs`: its root package pulls `net/http` and `crypto/tls` into this
  package's dependency graph, which invariant I5 forbids. Two build-time checks
  hold that line — an import allowlist in `internal/config` and an assertion in
  `cmd/aido` that `go list -deps ./internal/config` contains no `net`,
  `net/http`, or `crypto/tls`. go-git declares `go 1.25.0`, which is why the
  module floor moved (R1.1, `tech.md`). `go-billy` is go-git's filesystem
  interface and arrives only through it, so it rides on the same T1 entry rather
  than being a separate dependency decision.

  *(Amended 2026-07-21 by operator ruling. This bullet previously read "Go stdlib
  and `gopkg.in/yaml.v3` only", which stopped being true at `aa3dd69`, when the
  `git` subprocess was replaced to satisfy `tech.md` T3. See `APPROVALS.md`.)* `CGO_ENABLED=0` holds; nothing imports cgo (R1.1, T3).
- Depended on by: every later aido-core package. `Root` and its path
  constructors are the compatibility surface — later specs (`okf-bundle`,
  `query-links`) consume `Root` and must not construct `.aido/` paths themselves
  (S6).
- Backward compatibility: none required. This is the first code in the repo;
  there is no prior on-disk state to migrate. `config.yaml` and `.secrets.yaml`
  shapes are taken as given from the blueprint, so a future change to either is
  the T13 event, not this spec.
- No gRPC, no MCP, no Workspace reference (P9, T12).

## Alternatives

- **Rejected — `viper` or another config library.** `tech.md` T1 allowlist and
  the "may not be added" list both exclude it; `yaml.v3` plus a struct covers
  §4.3 entirely. T2 applies: nothing here is a named stdlib edge-case failure.
- **Rejected — resolving keys eagerly at `Load` time.** Would put every provider
  key in memory whenever config is read, including for `aido config show`, which
  must never hold one (R6.3). `ResolveKey` is called on demand by the provider
  that needs it.
- **Rejected — `os.WriteFile` for `.aido/` writes.** Truncates in place; violates
  I2 and T10 directly.
- **Rejected — inferring file mode from the target path inside `WriteFile`.**
  A heuristic ("path ends in `.secrets.yaml` → 0600") silently fails when a
  future secret lands at a different path. Mode is an explicit parameter.
- **Deferred — OS keyring and interactive prompt** (blueprint §4.5 steps 3–4).
  Keyring needs a dependency outside T1, so R3 of `reasoning.md` makes it
  `blocked`, not a default. Recorded as a non-goal in requirements; its own spec
  and ADR later.
- **Deferred — a `Config.Provider(name)` accessor and provider-set constant
  shared with `internal/llm`.** The supported-provider list is validated here
  (R3.3) and will be needed again by the routing spec. Duplicating one string
  slice later is cheaper to undo than inventing a shared package now
  (`reasoning.md` R4, R5).

## Verification

- R4.6: tests assert the write succeeds when `.gitignore` or `.git/info/exclude`
  covers the path, and is refused when the file is tracked, when the project is
  outside a repository, and when the only covering rule lives outside the
  repository (five configurations: the XDG default, `core.excludesFile` in
  `~/.gitconfig`, in `~/.config/git/config`, in the repository's `.git/config`,
  and via an `[include]` directive). A linked worktree is covered with its own
  index. The suite pins `HOME` and `XDG_CONFIG_HOME` to a temp directory, with a
  negative control that fails if any ignore rule reaches it from outside.
- I1: a test resolves a key from env and from `.secrets.yaml`, then asserts the
  key value appears in neither the returned error of a subsequent failed
  resolution nor the full stdout+stderr of `aido config show` run against the
  same config. Asserted by substring search for the key literal.
- I2, R5.1–R5.3: a test writes over an existing file with a deliberately failing
  writer, then asserts the destination bytes are unchanged and no `.tmp` file
  remains in the directory.
- I3: a test calls every path constructor against a `Root` under an empty temp
  dir, then asserts the temp dir is still empty.
- I4, R3.5: a test loads a config violating R3.1, R3.2, and R3.4 at once and
  asserts all three problems appear in the single returned error.
- I5: `go vet ./...` plus the absence of `net/http` and any provider import in
  `internal/config` — asserted by a test reading the package's own imports via
  `go/parser`.
- I6: covered by `go build ./...` and by R6 tests exercising `config show`
  through its exported behaviour.
- R1.1: `CGO_ENABLED=0 go build ./...` in the task verify command.
- Baseline every task carries (`workflow.md` W7): `go build ./...`,
  `go vet ./...`, `go test ./...`.

## Deployment

No deployment. aido-core is a library plus a CLI binary; this spec ships no
environment, no service, and no artifact beyond the built binary. `project.yml`
declares no environments. Ownership of the built binary is out of scope until a
release spec exists.

## Rollback

Trigger: any task's verify fails at HEAD, or a completed task is found to violate
I1 or I2. Path: `git revert` of the task's single commit (`workflow.md` W4 makes
each task exactly one commit, so a revert is exact). No on-disk state migration
is needed — this spec creates no `.aido/` state, only reads it, so reverting the
code leaves nothing behind to undo.
