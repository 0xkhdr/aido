# Review Report — aido-config

<!--
Filled by the AUDITOR role, not the craftsman who wrote the code. The harness
cannot verify reviewer identity; a craftsman reviewing its own work is an
anti-pattern (see docs/validation-gates.md). Edit the three fields below, then
run `specd approve <spec> complete` with review.required enabled.
-->

- **Git HEAD:** 5cffbab12c21770a0f7a8f9cccc87e02fa4da958
- **Reviewer:** pinky-auditor (subagent, unattended run 2026-07-21)
- **Verdict:** needs-changes

> Note: the scaffold recorded HEAD as `5cffbab` (T6). The tree actually audited is
> `5168581` (T7, `test(config): enforce the tech.md T1 import allowlist`). The
> scaffold's HEAD field is stale by one code commit — see F12.

## Tasks under review

### T1

- files: go.mod, internal/config/paths.go, internal/config/paths_test.go
- acceptance: R1.1, R1.2, R1.3

### T2

- files: internal/config/config.go, internal/config/config_test.go
- acceptance: R2.1, R2.2, R2.3, R2.4

### T3

- files: internal/config/validate.go, internal/config/validate_test.go
- acceptance: R3.1, R3.2, R3.3, R3.4, R3.5

### T4

- files: internal/config/write.go, internal/config/write_test.go
- acceptance: R5.1, R5.2, R5.3, R5.4

### T5

- files: internal/config/secrets.go, internal/config/secrets_test.go
- acceptance: R4.1, R4.2, R4.3, R4.4, R4.5, R4.6

### T6

- files: cmd/aido/main.go, cmd/aido/config_show.go, cmd/aido/config_show_test.go
- acceptance: R6.1, R6.2, R6.3

### T7

- files: internal/config/imports_test.go
- acceptance: R1.1

### T8

- files: .specd/specs/aido-config/review.md
- acceptance: R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R3.5, R4.1, R4.2, R4.3, R4.4, R4.5, R4.6, R5.1, R5.2, R5.3, R5.4, R6.1, R6.2, R6.3

## Findings

Verdict is **needs-changes**. Build, vet, and the whole suite pass at `5168581`
(`internal/config` 87.1% statement coverage, `cmd/aido` 84.8%), and the bulk of
R1–R6 is genuinely demonstrated. But one steering rule is violated outright
(F1), one requirement stated in "Edge and failure behavior" is not implemented
and is actively locked in by a test asserting the opposite (F2), and several
criteria are implemented with the failure branch never executed by any test
(F3, F4). This would not ship as production code.

Findings are ranked by severity. Every claim below was checked against the code
at `5168581`, not against the implementer's self-report.

---

### F1 — BLOCKER. `secrets.go` shells out to the `git` binary, which `tech.md` T3 forbids outright

`internal/config/secrets.go:107`

```go
cmd := exec.Command("git", "check-ignore", "-q", "--", path)
```

`tech.md` T3 reads, verbatim: *"**T3 — No cgo, no shelling out to `git`.** Git
operations go through go-git. A build that requires a C toolchain, **or a
runtime that requires the `git` binary on PATH, is refused.**"* `go-git` is on
the T1 allowlist (`tech.md` T1) and was available; it was not used and no ADR
exists. This is not a judgement call — it is the named prohibited construct.

Aggravating, three ways:

1. `internal/config/imports_test.go:20-23` was written (in T7) specifically to
   enforce T3, and enforces only its cgo half:
   `"C": "the build must stay CGO_ENABLED=0 (R1.1)"`. The author read T3, encoded
   one clause, and left the clause that the sibling file in the same spec breaks.
   The allowlist test cannot catch this anyway, because `os/exec` is stdlib — so
   the check that exists gives false assurance on exactly the rule it cites.
2. `internal/config/secrets_test.go:162-164` skips when git is absent
   (`t.Skip("git is not available")`). R4.6's *only* three tests
   (`TestWriteSecretsWritesWhenIgnored`, `TestWriteSecretsRefusesTrackedPath`,
   `TestWriteSecretsRefusesOutsideRepository`) are therefore silently skippable,
   and the suite still reports `ok`. A criterion whose entire coverage vanishes
   on a machine without git is not demonstrated on any machine.
3. `design.md:10` enumerates the external boundaries as *"the filesystem under
   `.aido/`, the process environment, and the `aido` CLI's
   stdout/stderr/exit code."* A `git` subprocess is a fourth boundary that
   design.md never declares. `design.md:83-84` (I5) further asserts *"Every
   exported function is a pure function of its arguments plus on-disk state and
   environment"* — `WriteSecrets` is not.

Required: replace `gitIgnores` with a `go-git` ignore-matcher check, or land an
ADR amending T3. Either way `design.md` must gain the boundary it currently
omits.

### F2 — MAJOR. `Validate` does not implement the "provider with neither `api_key_source` nor `base_url`" rule, and a test asserts the opposite

`requirements.md:88-89` requires: *"A provider entry with neither
`api_key_source` nor `base_url`: validation error naming the provider."*

`internal/config/validate.go:25-59` contains no such check. `Validate` inspects
`Project`, `TrackedBranch`, `RequiredDocs` prefixes, provider names, and
`default_provider` membership — and nothing about a provider entry's fields.
An entry `openai: {}` validates clean.

Worse, `internal/config/validate_test.go:106-115`
(`TestValidateAcceptsEverySupportedProvider`) constructs
`c.LLM.Providers[name] = Provider{}` for all six supported providers — every one
of them has neither `api_key_source` nor `base_url` — and asserts
`Validate() == nil`. The test suite does not merely fail to cover the rule; it
*pins the violation in place*, so implementing R-edge correctly now breaks a
green test. `validate_test.go:52` does the same with `Provider{}`.

This is the failure mode the task instructions warned about: requirements stated
outside the numbered R-list were dropped, and the numbered criteria were treated
as the whole spec. T3's `checks` column in `tasks.md:9` also omits it, so the
gap was baked in at task-decomposition time.

### F3 — MAJOR. R5.3's pre-rename cleanup path is dead code under test; the `abort` closure never executes

`internal/config/write.go:31-48`

Coverage profile at `5168581` shows zero coverage on `write.go:31.48,35.3` (the
`abort` closure body), `36.41,38.3` (write failure), `39.38,41.3` (chmod
failure), `42.33,44.3` (fsync failure), and `45.34,48.3` (close failure).

The one test that claims this path,
`TestWriteFileFailureLeavesDestinationIntact` (`write_test.go:69`), induces
failure by `os.Chmod(dir, 0o500)` — which makes `os.CreateTemp` fail at
`write.go:25`, returning at line 27 **before `abort` is ever defined or called**.
The test's own comment ("a failure before the rename leaves the destination
byte-for-byte unchanged and removes nothing but its own temp file") describes
behaviour the test does not reach: no temp file was ever created, so "removes its
own temp file" is asserted vacuously.

`TestWriteFileRenameFailureRemovesTemp` (`write_test.go:108`) covers only the
separate inline cleanup at `write.go:50`, not `abort`.

Net: R5.3 is demonstrated for the rename branch only. Delete the `os.Remove`
from inside `abort` and the entire suite still passes. Needs a test with a
short-write or chmod-failure injection (e.g. a destination on a full tmpfs, or
closing `f` out from under the write).

### F4 — MAJOR. R4.3's main not-found path — secrets file present, provider key absent — is never executed

`internal/config/secrets.go:78`

Coverage shows `secrets.go:78.2,78.117` uncovered. That line is the return for
*"the secrets file parsed fine, and it has no key for this provider"* — the
single most likely real-world not-found case.

The tests that appear to cover R4.3 do not reach it:

- `TestResolveKeyMissingFileIsNotFound` (`secrets_test.go:80`) hits the
  `fs.ErrNotExist` branch at `secrets.go:65`, a different return.
- `TestResolveKeyNeverLeaksKeyIntoErrors` (`secrets_test.go:151`) resolves
  `"anthropic"` and `"mistral"`, neither of which is in the config map, so both
  return at `secrets.go:45` — the *unknown-provider* error, not the not-found
  one. That loop also uses `if err != nil && strings.Contains(...)`, so it
  asserts nothing at all if `err` were nil.

Consequence: the R4.3 requirement that the error *"name the provider and the
sources consulted"* is asserted only for the missing-file path. If line 78's
message regressed to a bare `ErrKeyNotFound`, nothing would fail.

Also uncovered on the same path: `secrets.go:66-67` (I/O error other than
not-exist) and `secrets.go:120` (a `git check-ignore` failure that is not an
`ExitError` — e.g. git missing from PATH, or `.aido/` not yet existing, since
`cmd.Dir` is set to `r.String()`). Verified: with no `.aido/` directory,
`exec` fails with a chdir `*fs.PathError`, so `WriteSecrets` returns a raw
`git check-ignore ...` error rather than `ErrNotGitIgnored` or anything a
caller can classify.

### F5 — MAJOR. R1.2 has no check of any kind, and is already violated in the tree

`requirements.md:18` requires that any package needing an `.aido/` path obtain it
from the exported constructors *"rather than by string concatenation at the call
site."*

Nothing tests this. `paths_test.go` verifies the constructors return the right
strings (`TestConstructorsMatchOnDiskContract`) and create nothing
(`TestConstructorsCreateNothing`) — both are about the constructors, neither is
about call sites. R1.2 holds today only by inspection, and inspection already
fails:

`cmd/aido/config_show_test.go:37` — `aido := filepath.Join(dir, ".aido")`
`cmd/aido/config_show_test.go:42` — `filepath.Join(aido, "config.yaml")`
`cmd/aido/config_show_test.go:46` — `filepath.Join(aido, ".secrets.yaml")`

That is a package outside `internal/config` reconstructing three `.aido/` paths
by concatenation, in the exact form R1.2 names. It is test code, so the leak is
contained — but it is the first call site written after the rule, and it broke
the rule immediately, which is precisely the evidence that inspection is not a
control. `config_show_test.go` should use `config.NewRoot(dir).ConfigPath()` /
`.SecretsPath()`, and a grep- or AST-based check (the machinery in
`imports_test.go` already does AST walking) should fail the build on a
`".aido"` literal outside `internal/config/paths.go`.

Note also `imports_test.go` scans only `"."` (`internal/config`). `cmd/aido` is
under no allowlist enforcement at all, so R1.1's dependency half is unguarded
there.

### F6 — MODERATE. Unrecognised `api_key_source` values fail open and silently

`internal/config/secrets.go:52` — `strings.CutPrefix(p.APIKeySource, "env:")`

Only two forms are understood: `none`, and `env:NAME`. Anything else — a bare
`OPENAI_API_KEY`, a typo `en:OPENAI_API_KEY`, `keyring`, or the empty string
(which F2's missing validation permits) — takes neither branch. Resolution
silently skips the environment entirely, consults only `.secrets.yaml`, and the
`consulted` list in the R4.3 error names only the secrets path, so the resulting
message *tells the user the environment was never checked without saying why*.

R4.1 says *"its `api_key_source` names an environment variable"*; a config that
names one in an unrecognised form is treated as naming none. No test exercises a
non-`env:`, non-`none` source. Either `Validate` should reject unknown source
forms (which fits R3's "validation is total" posture and would fold into F2's
fix), or `ResolveKey` should return a distinct error.

### F7 — MODERATE. `ResolveKey`'s signature deviates from the approved design, and design.md was not amended

`design.md:53` specifies `func (c *Config) ResolveKey(provider string) (string, error)`.
`internal/config/secrets.go:42` implements
`func (c *Config) ResolveKey(r Root, provider string) (string, error)`.

Confirmed. The implementation is the *better* shape — `Config` has no way to
reach `.aido/` otherwise, so the design as written was not implementable — but
the approved artifact still carries the wrong signature, and `design.md`
Interfaces is explicitly the compatibility surface later specs are told to
consume (`design.md:117-120`). The correct handling was a design amendment
before or with T5, not a note in the feedback log after it. `WriteSecrets`,
`ErrNotGitIgnored`, `DirName`, `SupportedProviders`, and `Root.String()` are
likewise exported and absent from `design.md:34-58`.

### F8 — MODERATE. R4.6 is cited in design.md but has no interface, failure mode, or verification there

`design.md:3` lists R4.6 in `references:`; `design.md:71-74` names it in I1.
Neither the Interfaces section (`design.md:32-58`), the Failure section
(`design.md:89-111`), nor the Verification section (`design.md:151-171`)
specifies anything for it — no `WriteSecrets`, no `ErrNotGitIgnored`, no
git-ignore confirmation, no test to hold it. The entire R4.6 mechanism was
invented at implementation time inside T5 with no design gate.

Independently judging the implementation that resulted: the *direction* of the
check is right — refuse rather than warn — and two edge cases I probed behave
correctly:

- `.gitignore` ignoring the directory (`.aido/`) rather than the file: verified
  `git check-ignore -q -- <path>` still exits 0. Correctly treated as protected.
- a file already committed *before* a matching `.gitignore` rule was added:
  verified `check-ignore` exits 1 (git does not apply ignore rules to tracked
  paths). This fails **safe** — the write is refused on a tracked file. Good.

But the check is unsound in two directions the tests do not probe:

- `cmd.Dir = r.String()` (`secrets.go:88`, `secrets.go:108`) points at `.aido/`,
  which may not exist — see F4. It should be the project directory.
- Nested repositories: `check-ignore` resolves against whichever repo encloses
  `cmd.Dir`. If `.aido/` sits inside a nested inner repo whose `.gitignore` is
  silent while the outer repo ignores it (or vice versa), the answer given is
  for the wrong repository. Untested in either direction.

R4.6 is *partially* demonstrated — and only when git is on PATH (F1).

### F9 — MODERATE. Invariant I6 has no check

`design.md:167-168` states I6 is *"covered by `go build ./...` and by R6 tests
exercising `config show` through its exported behaviour."* A successful compile
is not a check that `cmd/aido` holds no validity decision — it would pass
identically if the whole validator were inlined into `config_show.go`.

The code currently honours I6 (`cmd/aido/config_show.go:35` delegates to
`c.Validate()`; `main.go` only dispatches subcommands), so I record I6 as
**held by inspection, unheld by any check**. `config_show.go:28`'s
`errors.Is(err, fs.ErrNotExist)` and `:36`'s `errors.As(..., &ve)` do duplicate
knowledge of the error taxonomy into the CLI — presentation branches, not
validity branches, so I do not call them violations, but they are the seam where
I6 will erode first.

### F10 — MINOR. Weak and near-vacuous assertions

- `cmd/aido/config_show_test.go:85` (`TestConfigShowPrintsKeySourceNotKey`)
  claims to demonstrate I1/R6.3 at the CLI boundary. `configShow` never calls
  `ResolveKey` at all, so the "no key in output" assertion would pass against
  essentially any implementation of `writeConfig` that does not go out of its way
  to read `.secrets.yaml`. It is a useful regression tripwire, not a
  demonstration.
- `secrets_test.go:152` — `if _, err := ...; err != nil && strings.Contains(...)`
  asserts nothing when `err` is nil. See F4.
- `write_test.go:44` (`TestWriteFileOverwrites`) would pass verbatim against
  `os.WriteFile`, i.e. against a direct I2 violation. Only
  `TestWriteFileFailureLeavesDestinationIntact` actually discriminates atomic
  from non-atomic (a plain `os.WriteFile` would clobber a 0644 file in a 0500
  directory) — so I2 rests on exactly one test.
- `config_show_test.go:118` asserts stderr mentions `"project"` — a substring so
  common it would match many wrong messages.

### F11 — MINOR. R1.1's Go-version and CGO clauses rest on the verify command only

No test asserts `go.mod`'s `go` directive is ≥ 1.22, and nothing in-repo fails if
someone raises or lowers it. `CGO_ENABLED=0` is enforced by the task verify
string in `tasks.md:3` and by `imports_test.go`'s ban on `import "C"` — the
latter is a real check, so R1.1 is partially demonstrated, but the "Go 1.22 or
newer" half is demonstrated by nothing that runs.

Relatedly, `imports_test.go:20-23` forbids `net/http` but not `net`, `net/rpc`,
or `crypto/tls`, so I5's "no network call" is enforced against one import path
rather than the capability.

### F12 — PROCESS. The spec was approved `executing` → `verifying` with T7 and T8 still pending, and the review scaffold is stale

Recorded here because it bears on the weight of every approval in this spec.
`review_report.md:10` pins HEAD at `5cffbab` (T6), but T7 landed afterwards at
`5168581`. The scaffold was therefore generated against a tree that did not yet
contain `imports_test.go` — the very file T7 added to satisfy R1.1. A reviewer
following the scaffold's own HEAD field would have audited a tree missing a
declared task's output.

Combined with the fact that all of T1–T7 was written and self-approved by one
agent under delegated approval, the standing evidence for this spec is: one
author, one approver, one reviewer — and the reviewer's baseline was wrong.
That is the context in which F1 (a steering rule broken while a test citing that
same rule was written) and F2 (a requirement dropped and then pinned shut by a
test) both survived to HEAD.

---

## Criterion-by-criterion trace

`D` = demonstrated by a test that would fail if the behaviour regressed.
`P` = partially demonstrated. `N` = not demonstrated.

| # | status | test that demonstrates it | note |
|---|---|---|---|
| R1.1 | P | `TestPackageImportsStayInAllowlist`, `TestDisallowedImportIsCaught`, `TestParserCatchesBannedImportInSource` (`imports_test.go:76,85,108`) | cgo/allowlist half checked for `internal/config` only; Go-1.22 clause and `cmd/aido` unchecked (F11, F5) |
| R1.2 | **N** | none | no call-site check exists; already violated at `cmd/aido/config_show_test.go:37` (F5) |
| R1.3 | D | `TestConstructorsCreateNothing` (`paths_test.go:52`), `TestLoadCreatesNothing` (`config_test.go:114`) | also `TestConstructorsMatchOnDiskContract` for the S1 tree |
| R2.1 | D | `TestLoadPopulatesAndIgnoresUnknown` (`config_test.go:65`) | |
| R2.2 | D | `TestLoadMissingIsNotExist` (`config_test.go:27`) | asserts `errors.Is(err, fs.ErrNotExist)` and nil config |
| R2.3 | D | `TestLoadMalformedNamesFileAndPosition` (`config_test.go:49`) | asserts both file and `"line "` |
| R2.4 | D | `TestLoadEmptyFile` (`config_test.go:38`), `TestLoadPopulatesAndIgnoresUnknown:101,108` | |
| R3.1 | D | `TestValidateSingleFailures/missing project`, `/missing tracked_branch`, `TestValidateBothRequiredKeysMissing` (`validate_test.go:49,50,75`) | |
| R3.2 | D | `TestValidateSingleFailures/default_provider not in providers` (`validate_test.go:51`) | |
| R3.3 | D | `TestValidateSingleFailures/unsupported provider` (`validate_test.go:52`), `TestValidateAcceptsEverySupportedProvider` (`:106`) | the latter is also the vehicle for F2 |
| R3.4 | D | `TestValidateSingleFailures/required_docs outside okf/` (`validate_test.go:53`) | |
| R3.5 | D | `TestValidateReportsEveryFailureAtOnce` (`validate_test.go:87`), `TestConfigShowInvalidConfigReportsEveryProblem` (`config_show_test.go:118`) | |
| R4.1 | D | `TestResolveKeyEnvWins` (`secrets_test.go:36`) | only for the `env:` form (F6) |
| R4.2 | D | `TestResolveKeyEmptyEnvFallsThroughToFile` (`secrets_test.go:50`), `TestResolveKeyNvidiaKeyName` (`:65`) | |
| R4.3 | **P** | `TestResolveKeyMissingFileIsNotFound` (`secrets_test.go:80`) | file-absent branch only; the file-present/key-absent return at `secrets.go:78` is uncovered (F4) |
| R4.4 | D | `TestResolveKeyNoneSource` (`secrets_test.go:99`) | |
| R4.5 | D | `TestResolveKeyMalformedSecretsQuotesNothing` (`secrets_test.go:124`), `TestResolveKeyNeverLeaksKeyIntoErrors` (`:142`) | the second is weak (F10); the first is genuinely good — it verifies the rebuilt, unwrapped yaml message |
| R4.6 | **P** | `TestWriteSecretsWritesWhenIgnored`, `TestWriteSecretsRefusesTrackedPath`, `TestWriteSecretsRefusesOutsideRepository` (`secrets_test.go:182,204,220`) | all three `t.Skip` without git on PATH (F1); nested-repo and missing-`.aido/` cases untested (F8) |
| R5.1 | D | `TestWriteFileCreates` (`write_test.go:25`), `TestWriteFileTempStaysInDestinationDirectory` (`:124`) | the same-directory temp is the load-bearing assertion |
| R5.2 | D | `TestWriteFileFailureLeavesDestinationIntact` (`write_test.go:69`) | one test carries this |
| R5.3 | **P** | `TestWriteFileRenameFailureRemovesTemp` (`write_test.go:108`) | rename branch only; `abort` at `write.go:31-48` is never executed (F3) |
| R5.4 | D | `TestWriteFileHonoursMode` (`write_test.go:137`), `TestWriteSecretsWritesWhenIgnored` (`secrets_test.go:191`) | the second is git-skippable |
| R6.1 | D | `TestConfigShowValid` (`config_show_test.go:61`), `TestConfigShowBinaryIntegration` (`:141`) | `llm.tasks` and `coding_agent` are not printed — matches `design.md:64`, so not a finding |
| R6.2 | D | `TestConfigShowMissingConfigExitsZero` (`:107`), `TestConfigShowInvalidConfigReportsEveryProblem` (`:118`), binary integration negative path (`:167`) | |
| R6.3 | D | `TestConfigShowPrintsKeySourceNotKey` (`:85`) | weak — `configShow` never calls `ResolveKey`, so it cannot regress (F10) |

Edge/failure clauses from `requirements.md:78-89`, which the numbered list does
not cover:

| clause | status | evidence |
|---|---|---|
| `.aido/` absent entirely → same not-found as R2.2 | D | `TestLoadMissingIsNotExist` (`config_test.go:27`) uses a bare temp dir with no `.aido/` |
| `.secrets.yaml` absent → not an error, falls to R4.3 | D | `TestResolveKeyMissingFileIsNotFound` (`secrets_test.go:80`) |
| `.secrets.yaml` unparseable → error names file, quotes no content | D | `TestResolveKeyMalformedSecretsQuotesNothing` (`secrets_test.go:124`) |
| empty `config.yaml` → zero config, then R3.1 with both keys | D | `TestLoadEmptyFile` + `TestValidateBothRequiredKeysMissing` |
| provider with neither `api_key_source` nor `base_url` → error naming it | **N — not implemented** | no code in `validate.go`; `validate_test.go:106` asserts the opposite (F2) |

Non-goals (`requirements.md:91-101`) — all honoured: no keyring or prompt code
exists (`imports_test.go:96` even bans `go-keyring` by name); nothing creates
`.aido/` (`write.go:23`, `TestWriteFileMissingDirectoryIsAnError`); no LLM or
network call; `coding_agent` is parsed and preserved but unused
(`config.go:38-51`, asserted at `config_test.go:104`).

## Invariants I1–I6

| id | verdict | check that holds it |
|---|---|---|
| I1 — a resolved key never enters an error, log, or printed line | **held, checks partial** | `TestResolveKeyMalformedSecretsQuotesNothing` (`secrets_test.go:124`) is the strong one: it proves the yaml error is rebuilt, not wrapped. `TestResolveKeyNeverLeaksKeyIntoErrors` (`:142`) and `TestConfigShowPrintsKeySourceNotKey` (`config_show_test.go:85`) are weak (F10). I read every error-construction site in `secrets.go`, `write.go`, `config.go`, and `config_show.go` and found **no path that can carry a key value** — `write.go:34` and `:47` interpolate only `path` and the OS error; `secrets.go:67` names the path only. I1 is substantively sound. |
| I2 — no `.aido/` file truncated in place | **held, one check** | Only `TestWriteFileFailureLeavesDestinationIntact` (`write_test.go:69`) discriminates temp-plus-rename from `os.WriteFile`. `TestWriteFileTempStaysInDestinationDirectory` (`:124`) holds the same-filesystem half. Durability (`f.Sync`, dir fsync at `write.go:56-63`) is unverifiable in-process and uncovered — acceptable. |
| I3 — path construction is pure | **held** | `TestConstructorsCreateNothing` (`paths_test.go:52`), plus the `constructors()` helper (`paths_test.go:21`) which routes every constructor through one map so a newly added one inherits the purity check. This is the best-designed test in the spec. |
| I4 — `Validate` is total | **held for the rules it has** | `TestValidateReportsEveryFailureAtOnce` (`validate_test.go:87`) asserts three unrelated problems in one error; `validate.go:38-44` sorts provider names for stable messages. Totality is real — but the rule set itself is incomplete (F2), so "complete on the first call" is true of a short list. |
| I5 — no LLM or network call | **held, check narrow** | `imports_test.go:20` bans `net/http` by AST inspection, and `TestNoAliasedImportsHideOrigin` (`:127`) closes the aliasing escape. But `net`, `net/rpc`, and `crypto/tls` are unbanned, and the "pure function of arguments plus on-disk state and environment" clause of `design.md:83` is **broken** by the `git` subprocess (F1). |
| I6 — `cmd/aido` holds no validity decision | **held by inspection, no check** | `config_show.go:35` delegates to `c.Validate()`; `main.go:17-34` only dispatches. `design.md:167` claims `go build` covers this; it does not (F9). |

