# Review Report — aido-config

<!--
Filled by the AUDITOR role, not the craftsman who wrote the code. The harness
cannot verify reviewer identity; a craftsman reviewing its own work is an
anti-pattern (see docs/validation-gates.md). Edit the three fields below, then
run `specd approve <spec> complete` with review.required enabled.
-->

- **Git HEAD:** b3b3c78940c80d1e5ae8695f6c98af4562a62038
- **Reviewer:** pinky-auditor (subagent, unattended run 2026-07-21)
- **Verdict:** approve

> The HEAD field above was set by the reviewer to the commit actually verified,
> replacing the `5cffbab` the scaffold stamped at T6. See the seventh pass at the
> end of this document.

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


---

## Re-audit at ddf549b (2026-07-21)

> The current call is the **Sixth pass at f08e624** subsection at the end of
> this document, not the original findings list above it.

Second pass by the same auditor, against `ddf549b`, on the three commits that
claim to close the findings above (`aa3dd69`, `ddf549b`, and the two `chore(specd)`
records between them). Everything below was checked by reading the code and by
running it, not by reading the commit messages.

Baseline at `ddf549b`: `go build ./...`, `go vet ./...` clean;
`CGO_ENABLED=0 go test ./...` passes; `internal/config` 90.0% statement coverage
(was 87.1%), `cmd/aido` 84.8% (unchanged); `CGO_ENABLED=0 go build ./cmd/aido`
produces a statically linked ELF, so R1.1's cgo half still holds.

**Provenance note.** The `**Git HEAD:**` field above still reads `5cffbab`, which
is now stale by five commits (`5168581`, `f1c5413`, `aa3dd69`, `ad896ea`,
`ddf549b`). It is left as the scaffold pinned it; correcting it by hand would
falsify what the harness recorded. The tree audited in this section is `ddf549b`.
This is the same defect F12 named, one round worse.

**Standing note on authority.** All three fix commits are out of band: they edit
files owned by T1, T3, T4, T6, and T7, all of which were already complete, and no
task marker or evidence record claims them. The code is better than it was; the
evidence chain is weaker than it was, because a passing suite at `ddf549b` is now
attested by no task's completion transaction.

### Disposition of F1–F12

| # | severity (orig) | disposition | note |
|---|---|---|---|
| F1 | BLOCKER | **resolved** — with new problems | `os/exec` is gone from `secrets.go`, banned by name in `imports_test.go:29`, and no test can skip any more. But see N1, N2, N3: the replacement is not behaviour-equivalent to `git check-ignore`, and the import closure grew a network stack. |
| F2 | MAJOR | **resolved** | `validate.go:52-55` implements the edge rule verbatim; `validate_test.go:55-57` asserts it; the two tests that pinned the violation were inverted correctly, and the `deepthought` case was given a key source so it still isolates R3.3. |
| F3 | MAJOR | **partially resolved** | The `abort` closure body now executes and is asserted. Write/chmod/close branches remain dead. See N8 on the seam. |
| F4 | MAJOR | **resolved** | `secrets.go:80` is now covered by `TestResolveKeyPresentFileMissingProviderKey`. The `.aido/`-does-not-exist crash is gone because `WriteSecrets` now discovers from the project directory (`secrets.go:92`). Residue: `secrets.go:68` (non-`ErrNotExist` I/O error) is still uncovered. |
| F5 | MAJOR | **partially resolved** | The three violating lines in `config_show_test.go` are gone and now route through `config.NewRoot`. The check that was supposed to make this a control is not one — see N5. |
| F6 | MODERATE | **still open** | `secrets.go:54` still only understands `none` and `env:`. F2's fix makes this marginally worse in effect: `Validate` now confirms `api_key_source` is *present* without checking its *form*, so `api_key_source: OPENAI_API_KEY` passes validation and is then silently ignored at resolution time. |
| F7 | MODERATE | **still open** | `design.md` is untouched since `5cffbab` (verified: `git log 5168581..HEAD -- design.md` is empty). `design.md:53` still declares `ResolveKey(provider string)`; the code still takes `(r Root, provider string)`. `WriteSecrets`, `ErrNotGitIgnored`, `DirName`, `SupportedProviders`, `Root.String()` are still exported and still absent from the Interfaces section. |
| F8 | MODERATE | **substance improved, finding still open** | I re-probed the two unsoundnesses: `cmd.Dir` pointing at a possibly-absent `.aido/` is fixed, and nested repositories now resolve correctly — `PlainOpenWithOptions(DetectDotGit)` finds the innermost enclosing repo, which is git's own answer (probed: outer repo ignoring `.aido/`, inner repo silent → `false`, matching `git check-ignore` run inside the inner repo). Neither is covered by a test. The design gap the finding is actually about is untouched: `design.md` still specifies no interface, failure mode, or verification for R4.6. |
| F9 | MODERATE | **still open** | Nothing added for I6. `design.md:167` still claims `go build` covers it. |
| F10 | MINOR | **partially resolved** | `TestWriteFileGoesThroughATempFile` (`write_test.go:154`) is a real fix for the `write_test.go:44` half — it observes two directory entries mid-write, so `os.WriteFile` now fails the suite, and I2 no longer rests on one test. The other three are unchanged: `secrets_test.go:152` still reads `if err != nil && strings.Contains(...)` and still asserts nothing when `err` is nil; `config_show_test.go` `TestConfigShowPrintsKeySourceNotKey` and the `"project"` substring assertion are as they were. |
| F11 | MINOR | **still open, and now demonstrably load-bearing** | Nothing asserts the `go` directive. `go.mod:3` moved from `go 1.22` to `go 1.25.0` in `aa3dd69` and no check in this repository noticed. See N4. The `net`/`net/rpc`/`crypto/tls` half of the finding got materially worse — see N3. |
| F12 | PROCESS | **still open, worse** | Three more out-of-band code commits, none claimed by a task, plus a HEAD field now five commits stale. |

Superseded: none. F1 is the only finding whose severity class changed.

### New findings

Ranked by severity. All were verified by execution against `ddf549b`, not by
inspection alone; the probe harness was a scratch copy of the module in `/tmp`,
so nothing in this repository was modified to obtain these results.

#### N1 — MAJOR. `.git/info/exclude` is silently not honoured; the go-git replacement is a functional regression on R4.6

`internal/config/secrets.go:143` — `gitignore.ReadPatterns(tree.Filesystem, nil)`

`ReadPatterns`'s own doc comment says it "reads the .git/info/exclude and then
the gitignore patterns". In go-git v5.19.1 it does not, because the worktree
billy filesystem refuses any path component named `.git`. Probed directly:

```
tree.Filesystem.Open(".git/info/exclude")
  → open: invalid path component: ".git/info/exclude"
gitignore.ReadPatterns(tree.Filesystem, nil) → 0 patterns, nil error
```

`ReadPatterns` discards that error (`ps, _ = readIgnoreFile(...)`), so the
failure is invisible: no error is returned, no pattern is loaded, and `gitIgnores`
answers `false`.

Consequence: a user who ignores `.aido/` in `.git/info/exclude` — the canonical
place for a *local, uncommitted* ignore of a local tool's state directory, and
arguably the most likely place for this exact rule — gets `WriteSecrets` refused
with `ErrNotGitIgnored`, on a path git genuinely ignores. The `git check-ignore`
implementation that F1 removed handled this correctly.

The direction is safe (a false refusal, not a false permit), but R4.6 is "write
only when git ignores it", and the implementation now disagrees with git on a
mainstream configuration. No test covers `.git/info/exclude` in either
implementation, which is why the regression landed silently.

Probe results for the full matrix at `ddf549b` (`gitIgnores` return for
`.aido/.secrets.yaml`):

| case | result | `git check-ignore` | verdict |
|---|---|---|---|
| `.gitignore` with `.aido/` | `true` | ignored | correct |
| `.gitignore` with `.aido/.secrets.yaml` | `true` | ignored | correct (existing test) |
| `.git/info/exclude` with `.aido/` | **`false`** | **ignored** | **N1 — wrong** |
| `core.excludesFile` with `.aido/` | **`false`** | **ignored** | **N2 — wrong** |
| tracked in the index | `false` | not ignored | correct, and now tested |
| not a repository | `false` | n/a | correct |
| nested repo, outer ignores | `false` | not ignored | correct (F8 concern closed) |
| project dir reached via symlink | **`false`** | ignored | **N7 — wrong** |
| `.aido/` then `!.aido/.secrets.yaml` | **`false`** | **ignored** | **N6 — wrong** |
| inside a bare repo | `false`, no error | n/a | correct, does not panic |

#### N2 — MODERATE. Global and system excludes are never consulted

`internal/config/secrets.go:143`

`gitignore.LoadGlobalPatterns` and `LoadSystemPatterns` exist in the same package
and are not called. A `core.excludesFile` naming `.aido/` — probed, returns
`false` — is ignored by git and refused by aido. Same class as N1, same safe
direction, and the same absence of any test. If the go-git port is kept, both
loaders belong in `gitIgnores` alongside `ReadPatterns`, in git's precedence
order (system, global, `info/exclude`, then `.gitignore` files).

#### N3 — MODERATE. `internal/config` now transitively links `net/http`, `crypto/tls`, `net`, and `golang.org/x/crypto/ssh`; the I5 check no longer means what it claims

`internal/config/secrets.go:11` — `import "github.com/go-git/go-git/v5"`

Verified with `go list -deps ./internal/config`: the closure now contains `net`,
`net/http`, `crypto/tls`, and `golang.org/x/crypto/ssh`. The whole dependency
graph went from ~30 packages to 337; the binary is 9.1 MB.

`imports_test.go:26` bans `net/http` "because this package makes no network call
(invariant I5)". That ban now guards only the *direct* import statement while the
package's actual dependency closure carries a full HTTP client, a TLS stack, and
an SSH transport, linked in for the sake of a local ignore-file lookup. No
network call is made, so I5 *holds*; but the one check cited as holding it has
been reduced to a naming convention. This is F11's second paragraph, escalated by
the fix for F1.

Avoidable: `gitIgnores` needs repository discovery, the index, and the ignore
matcher. `plumbing/format/gitignore`, `plumbing/format/index` (or
`storage/filesystem`), and `go-billy/osfs` supply all three without pulling
`remote.go` and its transports. If the root package is kept for convenience, that
is a decision T2 requires be recorded, not a side effect.

#### N4 — MODERATE. `go.mod`'s Go floor moved from 1.22 to 1.25.0 with no ADR, and `tech.md` still says 1.22

`go.mod:3` — `go 1.25.0` (was `go 1.22`)

Forced, not gratuitous: go-git v5.19.1's own `go.mod` declares `go 1.25.0`. But
the consequence is that a toolchain on Go 1.22, 1.23, or 1.24 can no longer build
this module, while `tech.md` still states "Go 1.22 or newer" and `requirements.md:17`
still states R1.1 in those terms. R1.1 is arguably satisfied on a literal reading
(1.25 *is* "1.22 or newer"), and I do not call it violated — but the module's
supported-toolchain floor is a steering fact that moved three minor versions in a
commit whose subject line is about `git check-ignore`, with no record. Either pin
an older go-git v5 that keeps the 1.22 floor, or amend `tech.md` and R1.1 by ADR.
F11 is exactly the check that would have caught this.

#### N5 — MODERATE. `TestNoHandBuiltAidoPaths` is evadable by the same trick it uses on itself, and guards one package

`cmd/aido/config_show_test.go:64`

Two defects:

1. It matches string *literals* containing `.aido`. `filepath.Join(dir, "."+"aido")`,
   `fmt.Sprintf(".%s", "aido")`, or `"." + config.DirName[1:]` all evade it. The
   test knows this — `config_show_test.go:66` builds its own needle as
   `"." + config.DirName[1:]` precisely so it does not match itself. The evasion
   is demonstrated in the check's own source, two lines above the check.
2. `parser.ParseDir(fset, ".", nil, 0)` at `config_show_test.go:68` scans the
   current package only, i.e. `cmd/aido`. R1.2 is a repository-wide rule ("when
   *any* package needs a path under `.aido/`"). Today `cmd/aido` and
   `internal/config` are the only packages, so the gap is latent — but the first
   `internal/git` or `internal/okf` added by the next spec inherits no check at
   all, and nothing will say so. The same objection applies to `imports_test.go:140`,
   which still scans only `"."`.

It also has a false-positive mode: any literal containing `.aido` is flagged,
including a legitimate error message such as `"no .aido directory found"`. A
check that fires on correct code trains authors to route around it.

A `go:generate`-free repo-root test walking `go list ./...` and parsing every
package outside `internal/config` would cover R1.2 properly. As it stands R1.2 is
enforced in one package, against one syntactic form, by a test that documents its
own bypass. I record R1.2 as **still N** in the trace table above.

#### N6 — MINOR. Negation inside an excluded directory diverges from git

Probed: `.gitignore` containing `.aido/` followed by `!.aido/.secrets.yaml`
returns `false`. Git's rule is that a file cannot be re-included when a parent
directory is excluded, so real git reports the file *ignored*. Fails safe
(refusal). Low likelihood, but it is a second divergence in the same matcher and
belongs in whatever test finally covers N1/N2.

#### N7 — MINOR. A project directory reached through a symlink is refused

`internal/config/secrets.go:128-132`

`filepath.Rel(tree.Filesystem.Root(), path)` compares an evaluated worktree root
against an unevaluated caller path. Probed: with the project reached via a
symlink, `rel` starts with `..` and `gitIgnores` returns `false`, so a correctly
ignored `.secrets.yaml` is refused. Fails safe. Affects anyone whose project
lives behind a symlink, and every `t.TempDir()` on macOS (`/var` → `/private/var`),
which means the R4.6 tests would misreport there. One `filepath.EvalSymlinks` on
both sides closes it.

#### N8 — MINOR. The `fsync` seam is sound, with two caveats

`internal/config/write.go:16` — `var fsync = func(f *os.File) error { return f.Sync() }`

I judge the seam **acceptable**. It is package-private, the production value is
`f.Sync()` unchanged, `WriteFile`'s behaviour on the success path is identical,
both tests restore it via `t.Cleanup`, and it buys the one thing that could not be
bought otherwise: proof that the cleanup runs, with the temp file's existence at
failure time asserted (`write_test.go:126-134`) rather than assumed. The
alternative — leaving R5.3's cleanup permanently unexecuted and hoping — is worse.
Two caveats:

- It is an unsynchronised package-level variable. No test in `internal/config`
  calls `t.Parallel()` today, so there is no race now, but the first one added
  makes this a data race, and nothing in the file says so. A one-line comment on
  the var, or a `t.Setenv`-style helper that fails under `-race`, would fix it.
- Coverage after the fix: the `abort` closure (`write.go:39-43`) and the fsync
  branch (`write.go:50-52`) are now covered. The write (`:44`), chmod (`:47`), and
  close (`:53`) branches are still at zero. Since all three route through the same
  now-proven `abort`, I do not treat them as separately load-bearing — the close
  branch at `:53-56` duplicates `abort`'s cleanup inline rather than calling it,
  so that one is genuinely unexercised code, but it is three lines and correct by
  inspection. F3 is downgraded, not dismissed.

#### N9 — MINOR. Two comments in the new tests describe something the code does not do

- `internal/config/secrets_test.go:234` says "`--force`, because the path is
  ignored". `git.AddOptions` has no `Force` field, and none is set. The add
  succeeds anyway because `AddWithOptions` passes an empty ignore-pattern list to
  `doAdd` (`worktree_status.go:349`), so an explicit `Path` is never filtered.
  The test is *sound* — I confirmed the index entry is genuinely created, because
  if it were not, `gitIgnores` would return `true` and the test would fail — but
  the comment attributes the outcome to a flag that does not exist.
- `cmd/aido/config_show_test.go:78-80` — the `found` counter and its
  `testing.Verbose()` log are dead ceremony; `t.Errorf` already carries the
  signal.

### Verdict

**needs-changes.**

F1's blocker is genuinely closed: the `git` binary is out of the runtime path,
the ban is now enforced by a check that would have caught it, and the R4.6 tests
can no longer skip themselves into silence. F2, F4, and the substance of F3 and
F10 are real fixes, honestly done, and the coverage moved in the right direction
for the right reasons. I want to be clear that this pass is better work than the
one it corrects.

It does not ship, for four reasons:

1. **N1** is a functional regression on R4.6 introduced by the fix for F1, in the
   safe direction but silently, on a mainstream git configuration, uncovered by
   any test. Trading a steering violation for a correctness regression on the same
   requirement is not a closed finding.
2. **F7 and F8** are untouched. `design.md` still carries a signature the code
   does not implement and still specifies nothing whatsoever for R4.6 — the
   requirement that has now been implemented twice, both times without a design
   gate, and got it wrong the second time. That is the causal chain, not a
   coincidence.
3. **N5** means R1.2 is still enforced by nothing that would stop the next
   violation, which is the identical position F5 described; only the specific
   three lines were fixed.
4. **F12/N4** — every one of these changes sits outside any task's completion
   transaction, and a steering-relevant toolchain floor moved inside a commit
   about something else. The harness cannot currently attest the tree it is being
   asked to approve.

Minimum to clear: fix N1 and N2 (load `info/exclude` and global excludes, and
stop discarding `ReadPatterns`' error) with a test per ignore source; amend
`design.md` for R4.6 and for `ResolveKey`'s real signature plus the five
undesigned exports; record N3 and N4 as an ADR or reduce the import to the
subpackages that are actually needed; and either widen the R1.2 check to every
package or drop the claim that R1.2 is checked. F6, F9, F10's residue, and F11
remain open and are not blockers.

### Third pass at 6d1fa4b

Baseline at `6d1fa4b`: `go build ./...` and `go vet ./...` clean; `CGO_ENABLED=0
go test ./...` passes **on a machine with no global git ignore rule matching
`.aido/`** — see NN2, which is not a hypothetical. `internal/config` coverage
85.0% (down from 90.0% at `ddf549b`), `cmd/aido` 84.8%.

Method: I re-ran the probe matrix, this time differentially — every case runs
`gitIgnores` and a real `git check-ignore` against the same repository with the
same `HOME`/`XDG_CONFIG_HOME`, and disagreement is the finding. `!!` marks a
divergence.

```
   root .gitignore .aido/                  aido=true  git=ignored
   info/exclude only                       aido=true  git=ignored
   info/exclude CRLF                       aido=true  git=ignored
   subdir .aido/.gitignore                 aido=true  git=ignored
   XDG ignore, core.excludesFile unset     aido=true  git=ignored
   core.excludesFile in ~/.gitconfig       aido=true  git=ignored
   core.excludesFile = ~/mine (tilde)      aido=true  git=ignored
   tracked via `git add -f` despite ignore  aido=false git=not-ignored
   not a repository                        aido=false git=n/a
   dir inside a bare repo                  aido=false git=n/a
   symlinked project path                  aido=true  git=ignored
!! core.excludesFile set + XDG ignore too  aido=true  git=not-ignored   NN3
!! core.excludesFile in ~/.config/git/config aido=false git=ignored     NN4
!! linked worktree, main info/exclude      aido=false git=ignored       NN5
!! negation inside an excluded directory   aido=false git=ignored       N6
!! .aido/ does not exist yet               aido=false git=ignored       NN1
   stray empty .git directory              aido=true  git=fatal(128)    NN7
```

Eleven of the seventeen cases now agree with git, including all four that N1/N2
were about and the one N7 was about. That is real progress, and the narrowing to
plumbing packages (N3) is exactly right. Six disagree; three of those are new.

**Disclosure.** An earlier probe of mine had an environment-construction bug
(`append(os.Environ(), "HOME=...")` — glibc `getenv` returns the *first* match,
so the child git read the real `$HOME`) and consequently wrote
`~/.gitconfig`, `~/otherignore`, and `~/.config/git/ignore` in the operator's
home directory. I removed the two junk files and rewrote `~/.gitconfig` with the
`user.name`/`user.email` recovered from this repository's own commit authorship
(`Mohamed Khedr <0xkhdr@gmail.com>`). **If that file held anything else — signing
keys, aliases, credential helpers, includes — it is gone and must be restored
from backup.** The matrix above was re-run afterwards with a properly isolated
environment; nothing in it depends on the contaminated state. The accident did
independently reproduce NN2.

#### Disposition of N1–N9

| # | disposition | note |
|---|---|---|
| N1 — `.git/info/exclude` dropped | **resolved** | `secrets.go:253-255` reads it directly with `os.ReadFile`, bypassing the billy `.git` rejection. Verified against `git check-ignore`, including a CRLF file, which I expected to break and does not — `ParsePattern` tolerates the trailing `\r`. |
| N2 — global/system excludes | **partially resolved** | `LoadSystemPatterns`, `LoadGlobalPatterns`, and the XDG default are now consulted, and `~` expansion in `core.excludesFile` works. Two new divergences remain: NN3 (over-application, **unsafe direction**) and NN4. |
| N3 — network stack in the closure | **resolved** | Verified: `go list -deps ./internal/config` = 112 packages, and `net`, `net/http`, `crypto/tls` are all absent. Binary 9,125,483 → 4,242,547 bytes. The whole-module closure is 113, so the test-only go-git root import does not leak into production. The *check* protecting this was weakened — NN6. |
| N4 — Go floor | **resolved** | `tech.md:34-37` and `requirements.md:17` both state 1.25 with the reason and the deliberate digest invalidation recorded inline. This is the right shape for an operator ruling. (Cosmetic: the specd-managed block at `tech.md:16` still reads "e.g. Go 1.22" — that is scaffold placeholder text, not a claim.) F11 remains open: still nothing *fails* if the directive moves again. |
| N5 — R1.2 check | **substantially resolved** | Now walks the whole module from the repo root, folds literal concatenations via `stringValue`, and guards vacuity with `if scanned == 0 { t.Fatal }` — which answers my objection directly. The residual evasions are documented in-code as a named ceiling rather than hidden, which is the correct way to ship a partial check. One residual matters: NN9. |
| N6 — negation inside an excluded dir | **still open** | Unchanged and still untested. `.aido/` then `!.aido/.secrets.yaml` → aido `false`, git `ignored`. Safe direction. Now the *only* remaining pre-existing divergence, so it is cheap to close. |
| N7 — symlinked project path | **resolved, with a regression** | `relativeTo` (`secrets.go:193-209`) resolves both sides; probe and `TestWriteSecretsThroughSymlinkedProjectPath` both confirm. The same function introduced NN1. |
| N8 — `fsync` seam | **unchanged** | Still an unsynchronised package-level var with no note about `t.Parallel`; `write.go:44`, `:47`, `:53-56`, `:65-67`, `:69-71` still at zero coverage. Minor, as before. |
| N9 — misleading comments | **partially resolved** | The `found`/`testing.Verbose()` ceremony is gone. `secrets_test.go:233` still says "`--force`, because the path is ignored"; `git.AddOptions` still has no `Force` field. |

#### On the specific questions asked

**Is `TestWriteSecretsHonoursInfoExclude` non-vacuous?** Not reliably. It asserts
only the positive (`WriteSecrets` returns nil) and does not isolate `HOME`, so on
any machine whose global excludes already match `.aido/` it passes without
`.git/info/exclude` being consulted at all. That is not theoretical — it is the
machine this audit ran on, after my own accident, and it is the configuration a
user of this tool is most likely to have. It needs `t.Setenv("HOME", t.TempDir())`
plus a negative control (same repository, no `info/exclude`, expect
`ErrNotGitIgnored`). See NN2.

**Is `parseExcludeFile` faithful, and is the nil domain right?** The nil domain is
**correct**. `gitignore.ReadPatterns` passes its `path` slice as the domain, which
is `nil` for the worktree root, and git anchors `info/exclude` and global-exclude
patterns at the worktree root exactly as it anchors a root `.gitignore`. Nil is
the right answer for all three sources.

The parser is faithful enough. Two immaterial deviations from go-git's own
`readIgnoreFile`, neither worth changing: go-git tests `HasPrefix(s, "#")` on the
untrimmed line, so `"  # note"` is a *pattern* to go-git and to git but a comment
to `parseExcludeFile`; and `bufio.Scanner` strips a trailing `\r` where
`strings.Split` does not — I expected that to break CRLF exclude files and probed
it, and it does not, because `ParsePattern` trims trailing whitespace itself. One
genuine gap: git honours a backslash-escaped trailing space (`foo\ `), and neither
implementation does. Not worth code.

**Does `findRepository` handle anything worse than `PlainOpenWithOptions`?** Two
things. It does not validate that the `.git` it found is a repository, so a stray
empty `.git` directory is accepted where go-git refused (NN7). And its `.git`-file
branch — the linked-worktree and submodule support the commit message advertises —
resolves the git directory correctly but then looks for `info/exclude` inside it,
where for a linked worktree that file does not live (NN5). Everything else it
handles at parity: absolute and relative `gitdir:` targets, `.git` as a symlink,
per-worktree `index`, walking to the filesystem root, and bare repositories.

**Can the R1.2 check still be evaded in a way that matters?** Yes, once: see NN9.

#### New findings

##### NN1 — MAJOR. `WriteSecrets` now refuses when `.aido/` does not exist yet, with a false security error, contradicting its own comment

`internal/config/secrets.go:199-203`, reported at `secrets.go:97-98`

```
.aido/ absent -> WriteSecrets err = refusing to write a key to a path that is
                 not git-ignored: /tmp/.../.aido/.secrets.yaml
  gitIgnores = false, <nil>
  relativeTo = "", <nil>
```

`relativeTo` calls `filepath.EvalSymlinks(filepath.Dir(path))` and, on failure,
returns `"", nil` — "a missing parent directory cannot be inside anything". But
`filepath.Dir(path)` *is* `.aido/`, and `WriteSecrets`'s own comment two functions
up (`secrets.go:91-92`) says "The project directory, not `.aido/` itself: `.aido/`
need not exist yet". The N7 fix made that comment false.

Three things wrong with the result, in increasing order:

1. It is the **first-run case**. Every project starts without `.aido/`.
2. The error is `ErrNotGitIgnored` — a security refusal — for a condition that is
   nothing of the kind. A caller doing `errors.Is(err, ErrNotGitIgnored)` will
   tell the user their `.gitignore` is wrong when their directory is merely
   absent. At `ddf549b` this case correctly evaluated the ignore rule and then
   failed in `WriteFile` with an `fs.ErrNotExist`-wrapping error a caller could
   classify. That is a regression in error quality, introduced by a fix for an
   unrelated finding.
3. `secrets.go:200-203` has **zero test coverage**, which is why it landed.

Fix is small: resolve the nearest existing ancestor, or resolve `worktree` and the
project directory and join the known-relative remainder. Then assert the case.

##### NN2 — MAJOR. The suite is not hermetic: a global git ignore matching `.aido/` fails a test and hollows out another

`internal/config/secrets_test.go:162` (`gitProject`)

`gitIgnores` now reads `/etc/gitconfig`, `$HOME/.gitconfig`, and
`$XDG_CONFIG_HOME/git/ignore`. No test isolates any of them. Reproduced
deliberately:

```
$ printf '.aido/\n' > $H/.config/git/ignore
$ HOME=$H XDG_CONFIG_HOME=$H/.config go test ./internal/config
--- FAIL: TestWriteSecretsRefusesTrackedPath (0.00s)
    secrets_test.go:207: err = <nil>, want errors.Is(err, ErrNotGitIgnored)
FAIL
```

The developer configuration that breaks the build is *`.aido/` in your global git
ignore* — which is the single most sensible thing a user of this tool would do,
and is precisely the capability N1/N2 asked for. The suite now punishes its own
recommended setup.

The mirror image is worse for evidence: on that same machine
`TestWriteSecretsHonoursInfoExclude` and `TestWriteSecretsWritesWhenIgnored` pass
because of the *global* rule, whether or not `info/exclude` or the repository
`.gitignore` is consulted at all. The test written to prove N1 closed cannot
distinguish N1-closed from N1-open on the machines where it matters most.

One line in `gitProject` — `t.Setenv("HOME", t.TempDir())` and the same for
`XDG_CONFIG_HOME` — fixes both halves. `/etc/gitconfig` stays uncontrollable
(go-git does not honour `GIT_CONFIG_NOSYSTEM`); accept that or inject the pattern
sources.

##### NN3 — MODERATE, and the only divergence in the unsafe direction. XDG excludes are applied *in addition to* `core.excludesFile`, not as its default

`internal/config/secrets.go:245-252`

Git's rule: `$XDG_CONFIG_HOME/git/ignore` is the **default value** of
`core.excludesFile`. If `core.excludesFile` is set, the XDG file is not read.
This code appends it unconditionally, on top of whatever `LoadGlobalPatterns`
returned. Probed:

```
!! core.excludesFile set (to an irrelevant file) + ~/.config/git/ignore present
   aido=true   git=not-ignored
```

Every other divergence in this implementation errs toward refusing to write.
This one errs toward **writing a key to a path git will happily track** — the
exact leak R4.6 exists to prevent. Low likelihood, maximal consequence, and it is
the one case where "fails safe" no longer covers for the gap.

Fix: read the XDG file only when neither `LoadSystemPatterns` nor
`LoadGlobalPatterns` produced patterns from an explicit `core.excludesFile` —
which means the code needs to know *whether* the option was set, not just what it
returned. Reading `~/.gitconfig`'s `core.excludesfile` directly (go-git's
`plumbing/format/config` is already in the closure) is the honest way to get that.

##### NN4 — MODERATE. `core.excludesFile` declared in `~/.config/git/config` is not read

`internal/config/secrets.go:242`

`gitignore.LoadGlobalPatterns` reads `$HOME/.gitconfig` only. Git also reads the
XDG config file `$XDG_CONFIG_HOME/git/config`, which is where a lot of modern
setups put it. Probed: aido `false`, git `ignored`. Safe direction (false
refusal), but it is the same class of gap N2 was raised for, one level up: the
sources are now consulted, the *config files that name* the sources are not.

##### NN5 — MODERATE. In a linked worktree, `info/exclude` is read from the wrong directory, and the entire `.git`-file branch is untested

`internal/config/secrets.go:168-181` and `:255`

`findRepository` correctly follows a `.git` file to `<main>/.git/worktrees/<name>`,
and reading `index` from there is right — that index is per-worktree. But
`info/exclude` is **not** per-worktree: git reads `$GIT_COMMON_DIR/info/exclude`,
i.e. the main repository's `.git/info/exclude`. Probed with a real
`git worktree add`:

```
findRepository(linked) -> gitDir=<main>/.git/worktrees/linked
info/exclude at gitDir exists = false
pattern count = 0
!! aido=false  git=ignored
```

The `commondir` file sitting next to `gitdir` in that same directory names the
path to resolve. Two lines.

Compounding: coverage shows `secrets.go:168-181` — the whole `.git`-file branch,
i.e. every submodule and every linked worktree — at **zero**. The feature the
commit message advertises is exercised by nothing.

##### NN6 — MODERATE. The import allowlist now matches by module prefix, which re-opens the hole N3 was about

`internal/config/imports_test.go:18-32`

`allowedModule` accepts anything under `github.com/go-git/go-git/v5/`. That now
includes `plumbing/transport/http` and `plumbing/transport/ssh`, either of which
pulls `net/http` and `crypto/tls` straight back into this package's closure — and
the `forbidden` map would not fire, because it matches the literal import path
`net/http`, not what an allowed import drags in.

So the check that certifies N3 closed would not notice N3 re-opening. The
prefix widening was necessary (three subpackages are legitimately used); the
mistake is relying on an import-path check for a *dependency-closure* property.
The direct fix is one assertion that costs nothing:
`go list -deps ./internal/config` must not contain `net/http`. That tests the
thing I5 actually claims, for every future import, and would have caught the
original F1 fix too.

##### NN7 — MINOR. A stray `.git` directory is accepted as a repository

`internal/config/secrets.go:166-167`

`findRepository` returns on the first `.git` that stats as a directory, without
checking for `HEAD` or `objects/`. Probed: a bare `mkdir .git` plus a `.gitignore`
yields aido `true` where git exits 128 ("not a git repository"). `PlainOpen`
rejected this. Direction is a false permit, but the consequence is nil — a
directory that is not a repository cannot commit anything — so this is
housekeeping, not a leak. One `os.Stat(gitDir/"HEAD")` closes it if you care.

##### NN8 — MINOR. Coverage regression, concentrated in the new hand-rolled code

`internal/config` fell 90.0% → 85.0%. The new code added roughly 110 lines of
repository discovery, index decoding, and pattern assembly, and the following are
at zero: `secrets.go:138-140`, `:142-144`, `:149-151`, `:159-161`, `:168-181`
(NN5), `:195-197`, `:200-203` (NN1), `:205-207`, `:218-220`, `:223-225`, `:231`,
`:257-259`.

Two of those uncovered blocks are the two functional bugs in this pass. That is
not a coincidence and it is the same pattern F3 and F4 named: a branch nobody
executes is a branch nobody checked. Replacing a well-tested library function
with 110 lines of your own is defensible here — the operator asked for the
narrowing and the library was genuinely dropping `info/exclude` — but it moves
the burden of proof onto this repository's tests, and they have not taken it up.
The differential probe in this section is the shape the test should have: build a
repository, ask `gitIgnores` and `git check-ignore` the same question, require the
same answer. Keep it behind a `git`-on-PATH check so T3 is not reintroduced into
the runtime.

##### NN9 — MINOR. The one R1.2 evasion that matters, plus a prefix nit

`cmd/aido/config_show_test.go:66-92`

`stringValue` folds literal `+` chains, so `"." + "aido"` is caught, and the
in-code ceiling comment is honest about non-literal operands. The evasion that
matters is the one an author would reach for *by accident*:

```go
filepath.Join(dir, config.DirName, "config.yaml")   // caught by nothing
config.DirName + "/config.yaml"                     // caught by nothing
```

Neither contains a `.aido` literal, both are hand-built `.aido/` paths, and the
second is exactly what someone half-remembering R1.2 writes — reach for the
exported constant, skip the constructor. `config.DirName` is exported precisely
so this is easy. Adding `DirName` as a second needle — flag any `config.DirName`
selector outside `internal/config` — costs four lines and closes the realistic
case. The adversarial cases (`fmt.Sprintf`, byte slices) I agree are not worth
`go/types`.

Nit: `strings.HasPrefix(rel, filepath.Join("internal","config"))` at
`config_show_test.go:92` is a string-prefix test, so a future
`internal/configloader` would inherit the owner exemption silently. Use
`rel == owner || strings.HasPrefix(rel, owner+string(filepath.Separator))`. Also,
the exemption covers all of `internal/config` where only `paths.go` needs it.

#### Verdict

**needs-changes.**

The direction is right and the work is substantially better than either prior
pass. N3 is closed cleanly and verifiably — the network stack is out of the
production closure, the binary halved, and the narrowing to plumbing packages is
the correct reading of both T1 and T2. N1 is closed and I confirmed it against
real git. N4 is handled the way a steering change should be: an operator ruling,
recorded in both artifacts, with the digest invalidation stated rather than
hidden. N5's ceiling is documented in the code instead of being papered over.
This is the first pass where I would say the author is auditing their own work
honestly.

It still does not ship:

1. **NN1** — the first-run case, `.aido/` not yet created, now returns a security
   refusal for a missing directory. Introduced by this commit, uncovered by any
   test, and it contradicts a comment fifteen lines away in the same file.
2. **NN2** — the suite is non-hermetic against the very configuration the last two
   passes were about supporting, and the test that certifies N1 is vacuous on the
   machines where N1 mattered. I will not certify R4.6 on evidence that a
   `~/.config/git/ignore` line can produce.
3. **NN3** — the first divergence in this whole audit that fails *permissive* on a
   guard whose entire purpose is refusing to leak a key.
4. **NN5/NN8** — 110 lines of hand-rolled git internals replaced a well-tested
   library call, and the two blocks that turned out to be wrong are both blocks no
   test executes.

Minimum to clear, in order: NN1, NN2, NN3. Then NN5 (`commondir`, two lines) and
NN6 (one `go list -deps` assertion, which is worth more than the rest of the
import test put together). N6, NN4, NN7, NN9, F6, F7, F8, F9, F10-residue, F11
and F12 remain open and are not blockers — but F7 and F8 have now survived three
passes, and `design.md` still specifies nothing for the requirement this entire
re-audit has been about.

Standing note, unchanged from the second pass: `6d1fa4b`, `ddf549b`, and
`aa3dd69` are all out of band, no task marker claims them, and the `**Git HEAD:**`
field above still reads `5cffbab`.

### Fourth pass at d90efc4

Baseline: `go build ./...`, `go vet ./...`, and `CGO_ENABLED=0 go test ./...` all
pass. `internal/config` 86.5%, `cmd/aido` 84.8%. `go list -deps ./internal/config`
= 112 packages with no `net`, `net/http`, or `crypto/tls`.

Environment discipline this pass: every probe replaced `HOME` and
`XDG_CONFIG_HOME` by filtering the parent environment before appending, and ran
entirely under `/tmp`. Nothing was written under `$HOME`, and no git write
command was issued. For the record, the earlier damage was a probe writing
`~/.gitconfig` through the duplicate-`HOME` bug; I have not run `git commit`,
`git push`, or any history-rewriting command in any pass of this audit.

#### Probe matrix at d90efc4

`XX` marks a divergence in the **unsafe** direction — aido says ignored, git says
tracked.

```
   root .gitignore .aido/                       aido=true  git=ignored
   info/exclude only                            aido=true  git=ignored
   info/exclude CRLF                            aido=true  git=ignored
   subdir .aido/.gitignore                      aido=true  git=ignored
   XDG ignore, core.excludesFile unset          aido=true  git=ignored
   core.excludesFile in ~/.gitconfig (tilde)    aido=true  git=ignored
   NN3: excludesFile set + XDG ignore present   aido=false git=not-ignored   FIXED
   tracked via `git add -f` despite ignore      aido=false git=not-ignored
   NN1: .aido/ does not exist yet               aido=true  git=ignored       FIXED
   symlinked project path                       aido=true  git=ignored
   NN5: REAL `git worktree add`, info/exclude   aido=true  git=ignored       FIXED
   NN5b: linked worktree, tracked in ITS index  aido=false git=not-ignored   FIXED
   not a repository                             aido=false git=n/a
   dir inside a bare repo                       aido=false git=n/a
!! NN4: excludesFile in ~/.config/git/config    aido=false git=ignored
!! N6:  negation inside an excluded directory   aido=false git=ignored
   NN7: stray empty .git directory              aido=true  git=fatal
XX NEW: excludesFile in ~/.config/git/config
        + XDG ignore present                    aido=true  git=not-ignored   NEW1
XX NEW: excludesFile in the repo's .git/config  aido=true  git=not-ignored   NEW1
XX NEW: excludesFile via [include] path         aido=true  git=not-ignored   NEW1
```

Fourteen of twenty agree, up from eleven of seventeen. All four cases the
operator's rulings targeted are genuinely fixed and I verified NN5 against a real
`git worktree add`, not only against the hand-built fixture. But three cases now
diverge in the direction that matters, and they are the direct consequence of the
NN3 fix.

#### Disposition of NN1–NN9

| # | disposition | note |
|---|---|---|
| NN1 — `.aido/` absent → false refusal | **resolved** | `resolveExisting` (`secrets.go:218-239`) walks to the deepest existing ancestor and re-appends the missing tail. Probed: `.aido/` absent now returns `ignored=true` and `WriteSecrets` fails wrapping `fs.ErrNotExist`. `TestWriteSecretsMissingAidoDirIsNotARefusal` asserts both halves, including the negative (`!errors.Is(err, ErrNotGitIgnored)`). This is the right shape for a regression test. |
| NN2 — non-hermetic suite | **resolved** | `isolateGitEnvironment` (`secrets_test.go:173`) and `TestIgnoreSourcesAreIsolated` (`:335`). The negative control is the correct instrument: it fails if anything leaks in from outside, which is exactly the property that was missing. The `/etc/gitconfig` gap is real (go-git ignores `GIT_CONFIG_NOSYSTEM`) and correctly documented rather than papered over. |
| NN3 — XDG applied as an addition | **partially resolved — and it regressed into a wider unsafe class** | The specific case is fixed and asserted (`TestGlobalExcludesFileDefersToCoreExcludesFile`). But making the XDG file conditional means aido now has to answer "is `core.excludesFile` set?", and it answers from one config file out of five. See NEW1. |
| NN4 — `core.excludesFile` in `~/.config/git/config` | **still open, and promoted** | Last pass this was a benign false refusal. It is now one of three demonstrated **leaks**, because the unseen setting activates a fallback git is not using. |
| NN5 — linked-worktree `info/exclude` | **resolved** | `commonDir` (`secrets.go:328-340`) follows `commondir`, relative or absolute. Verified against a real `git worktree add`: `info/exclude` from the main checkout applies, and a file tracked in the *linked* worktree's own index is still correctly refused. |
| NN6 — import allowlist by prefix | **resolved** | `TestConfigPackageHasNoNetworkDependency` (`cmd/aido/config_show_test.go:185`) asserts the dependency closure, which is what I5 actually claims, and `forbiddenSubtrees` (`imports_test.go:30`) denies `plumbing/transport` at the import site. Belt and braces, and the belt is the right one. Placing it in `cmd/aido` to respect the `os/exec` ban is the correct call. |
| NN7 — stray `.git` accepted | **still open** | Unchanged, still nil consequence. |
| NN8 — coverage of hand-rolled code | **partially resolved** | 85.0% → 86.5%, and the two blocks that were wrong last pass are now covered. Twenty-five blocks in `secrets.go` remain at zero, and `globalExcludesFile` (77.8%) and `excludesFile` (76.9%) are the two lowest-covered new functions — which is where NEW1 lives. |
| NN9 — R1.2 evasion via `config.DirName` | **resolved** | Referencing `DirName` outside `internal/config` is now itself the finding (`config_show_test.go:110`), which catches `config.DirName + "/x"` without needing `go/types`. The owner exemption compares path components. Deriving the needle from `filepath.Base(string(config.NewRoot("x")))` to avoid self-flagging is neat. |

Numbering note: my third pass used `N6` for the negation divergence and `NN6` for
the import allowlist; the fourth-pass request called the negation finding `NN6`.
The negation divergence is **N6** and is still open. `NN6` (allowlist) is closed.

#### NEW1 — MAJOR, leaks a key. `globalExcludesFile` infers "is `core.excludesFile` set?" from one config file out of five

`internal/config/secrets.go:288-301`

`globalExcludesFile` reads `core.excludesFile` from `$HOME/.gitconfig` and, when
it finds nothing, returns `$XDG_CONFIG_HOME/git/ignore` as git's documented
default. Git resolves that key across **system config, `$XDG_CONFIG_HOME/git/config`,
`~/.gitconfig`, the repository's `.git/config`, the per-worktree config, and any
`[include]`/`[includeIf]` those pull in** — last writer wins. Any setting aido
cannot see leaves it believing the key is unset, so it applies a default file git
is not using.

Not a theoretical ordering argument. End-to-end, `WriteSecrets` returning `nil`
and the key landing on disk at a path `git status --untracked-files=all` lists —
i.e. one `git add .` from being committed:

```
core.excludesFile in ~/.config/git/config    WriteSecrets=<nil> keyOnDisk=true gitWouldAddIt=true
core.excludesFile in the repo's .git/config  WriteSecrets=<nil> keyOnDisk=true gitWouldAddIt=true
core.excludesFile via [include] path         WriteSecrets=<nil> keyOnDisk=true gitWouldAddIt=true
```

The middle one is the ordinary case: `git config core.excludesFile <path>` run
inside a project is a normal thing to do, and it silently disarms the guard.

This is worse than what it replaced. At `6d1fa4b` the XDG file was applied
unconditionally — one unsafe vector. Making it conditional on incomplete
information turned one vector into at least three, because now *failing to read a
config file* is itself the trigger. Every gap in aido's config resolution is now
a leak rather than a missed pattern.

Two ways out, and I would take the second.

1. **Resolve the key properly**: read system, XDG, `~/.gitconfig`, `.git/config`,
   and worktree config in git's order, following `[include]`. That is
   reimplementing `git config --get` and it will keep growing this exact class of
   bug. `[includeIf]` alone (gitdir, onbranch, hasconfig conditions) is more logic
   than the rest of this package.
2. **Delete the global sources entirely.** `ignorePatterns` should consult
   `.git/info/exclude` and the repository's `.gitignore` files, and nothing else.
   This is smaller code, it fails safe in every direction by construction, and it
   is the better *product* answer: a `.secrets.yaml` protected only by the current
   user's machine-global ignore rule is not protected for anyone who clones the
   repository. R4.6 exists so the key cannot be committed — by anyone, from any
   checkout. A protection that does not travel with the repository does not
   satisfy that, and refusing until the rule is written into `.gitignore` or
   `info/exclude` is the correct refusal, not an inconvenience.

Option 2 deletes `globalExcludesFile`, `excludesFile`, `systemGitConfig`, the
`/etc/gitconfig` gap NN2 had to document, NN4, NEW1, and NEW2 in one edit, and
shrinks `ignorePatterns` to four lines. If the operator wants global rules
honoured anyway, that is a product decision that needs recording — and it still
needs option 1 done properly to be safe.

#### NEW2 — MINOR. Residual `core.excludesFile` parsing gaps

`internal/config/secrets.go:305-323`

Probed. Case-insensitivity is fine — `excludesFile` matches `excludesFile`,
`excludesfile`, and `[CORE]`, because go-git's config lookup folds case. Three
gaps remain:

- `excludesFile(systemGitConfig, "")` passes `home = ""`, so a `~/…` value in
  `/etc/gitconfig` is returned unexpanded (`"~/mine"`) and silently opens nothing.
  Safe direction.
- git's `%(prefix)/…` form is unhandled. Safe direction.
- An explicitly empty `excludesfile =` is treated as unset, so the XDG default
  applies; git treats an empty value as "no excludes file". Same unsafe class as
  NEW1.

All three vanish under NEW1's option 2.

#### NEW3 — MINOR. The linked-worktree fixture is thinner than it looks

`internal/config/secrets_test.go:437-476`

The hand-built worktree writes `commondir`, `gitdir`, and `HEAD` but no `index`,
so `trackedInIndex` takes its "no index yet" early return and the per-worktree
index path — the half of NN5 that is genuinely subtle — is never exercised by the
test. The behaviour is nonetheless correct: I verified it separately against a
real `git worktree add` with the file tracked in the linked worktree's own index,
and aido refuses correctly. So the code is right and the fixture under-claims.
Faithful enough for the assertion it makes; worth one `os.WriteFile` of an empty
index or a `t.Skip`-guarded real-git variant if you want the test to carry it.

#### NEW4 — MINOR. The `DirName` rule matches on selector name only

`cmd/aido/config_show_test.go:110`

`sel.Sel.Name == "DirName"` flags any selector so named, from any package. A
future `flags.DirName` or `osutil.DirName` would be reported as an R1.2 violation
it has nothing to do with. Checking the receiver identifier resolves to the
config import is a two-line fix; false positives on a rule that fires at build
time train people to work around it.

#### Is criterion 4.6 demonstrably satisfied at d90efc4?

**No.** I am not able to certify it, and I would give the same answer if someone
else had written the code.

R4.6 requires that a key be written only to a path *confirmed git-ignored first*.
At `d90efc4` there are three reproducible configurations — one of them the
everyday `git config core.excludesFile <path>` inside a project — in which
`WriteSecrets` returns `nil`, writes the key at mode 0600, and leaves it at a path
`git status` reports as untracked-and-visible. The guard does not hold. The
mechanism is demonstrated above end to end, not inferred.

Everything else R4.6 needs is now in place, and I want that on the record because
the gap is narrow and specific:

- the refusal is a refusal, not a warning, and it is tested in both directions;
- tracking beats ignoring, verified in a normal repository and in a linked
  worktree's own index;
- the first-run case is no longer misreported as a security refusal;
- repository-local sources — `.gitignore` at any depth and `info/exclude`,
  including through `$GIT_COMMON_DIR` — agree with `git check-ignore` in every
  case I probed;
- the tests are hermetic, with a negative control that fails if they stop being;
- nothing requires the `git` binary at runtime, and the dependency closure is
  clean.

What is missing is exactly one thing: **the global-excludes path can report
`ignored` for a path git tracks.** Close NEW1 — by my reading, by deleting the
global sources rather than by resolving them — and add one test asserting that
`WriteSecrets` refuses when the only rule protecting the path comes from outside
the repository. At that point I will record 4.6 as demonstrated, and N6, NN4,
NN7, NEW2, NEW3, and NEW4 will all be non-blocking residue on a criterion I would
sign.

Two process facts unchanged and still standing: `d90efc4`, `6d1fa4b`, `ddf549b`,
and `aa3dd69` are all out of band with no task marker claiming them, and the
`**Git HEAD:**` field at the top of this document still reads `5cffbab`. F7 and
F8 have now survived four passes — `design.md` still specifies no interface, no
failure mode, and no verification for R4.6, which is the fourth consecutive pass
in which this requirement has been redesigned at implementation time.

### Fifth pass at 3a1e33b

Baseline: `go build ./...`, `go vet ./...`, `CGO_ENABLED=0 go test ./...` pass.
`internal/config` 87.3%, `cmd/aido` 84.8%. Closure still 112 packages, no `net`,
`net/http`, or `crypto/tls`. Nothing written under `$HOME`; no git write command
issued.

#### 1. Is NEW1 closed?

**Yes.** `grep` over `internal/config/*.go` finds no `UserHomeDir`, no
`XDG_CONFIG_HOME`, no `/etc/gitconfig`, no `LoadGlobalPatterns`/`LoadSystemPatterns`,
and no `excludesFile`. The only surviving `os.Getenv` is `secrets.go:58`, which is
R4.1's `env:NAME` lookup and belongs there. `ignorePatterns` is now
`info/exclude` from `commonDir(gitDir)` plus `gitignore.ReadPatterns` over the
worktree, and nothing else.

I probed the two escapes I could construct rather than taking the grep as proof:

- **A symlinked subdirectory inside the worktree pointing at a directory outside
  it, containing a matching `.gitignore`.** `ReadPatterns` does not follow it —
  billy's `ReadDir` reports the symlink as a non-directory, so the recursion skips
  it. aido `false`, git `not-ignored`: agreement, no escape.
- **`commondir` pointing at an arbitrary directory** whose `info/exclude` matches.
  aido does follow it, because `commonDir` returns the file's contents
  unvalidated. This is not a reachable weakness: writing `commondir` requires
  write access inside `.git`, and anyone holding that can write
  `.git/info/exclude` directly. Worth one sentence in the doc comment, not a
  finding.

All five configurations I demonstrated as leaks at `d90efc4` now refuse, verified
against real `git check-ignore` in an isolated `HOME`:

```
!! GLOBAL: XDG default ignore                    aido=false git=ignored
!! GLOBAL: core.excludesFile in ~/.gitconfig     aido=false git=ignored
!! GLOBAL: core.excludesFile in XDG config       aido=false git=ignored
!! GLOBAL: core.excludesFile in repo .git/config aido=false git=ignored
!! GLOBAL: core.excludesFile via [include]       aido=false git=ignored
```

Those five `!!` are deliberate, documented, and tested by
`TestWriteSecretsIgnoresProtectionOutsideTheRepository` (`secrets_test.go:396`),
which is table-driven over exactly these five and asserts `ErrNotGitIgnored` *and*
that nothing reached disk. **There are now zero divergences in the unsafe
direction anywhere in the matrix.** That is the first time in five passes.

#### 2. On deleting the global sources

I still think it is right, and more firmly having read the implementation than
when I proposed it. The doc comment at `secrets.go:263-281` states the reason
better than my finding did, and states it where the next person to touch this
will read it — including the concrete history of why reading `~/.gitconfig` alone
was worse than reading nothing. That is the correct place for that paragraph.

The reasoning, restated so it can be quoted: R4.6 exists so a resolved key cannot
be committed. "Committed" is not scoped to this machine. A `.secrets.yaml`
protected only by the current user's global ignore is unprotected in every other
clone and for every other collaborator, so treating that as satisfying the guard
answers a question R4.6 did not ask. Refusing until the rule is written into the
repository — `.gitignore` or `.git/info/exclude` — is the correct refusal, and it
is also the one that makes the protection durable rather than incidental.

**The message should say so.** Today the refusal is:

```
refusing to write a key to a path that is not git-ignored: <path>
```

For the user whose `~/.config/git/ignore` does cover `.aido/`, that sentence is
false as they will read it — their git *does* ignore it — and their first move
will be to check the rule they already have, find it, and file a bug. The
refusal is right; the explanation is missing. Something like "not ignored by any
rule in this repository (`.gitignore` or `.git/info/exclude`); a machine-global
ignore does not protect the file in other clones" costs one line and converts a
confusing refusal into an instruction. I record this as **NEW5 — MINOR**, and it
is explicitly **not** blocking for R4.6: R4.6 constrains the refusal, not its
wording, and no requirement in this spec specifies message text for it.

#### 3. Residual matrix

| case | status |
|---|---|
| root `.gitignore`, `info/exclude`, subdirectory `.gitignore` | agree with git |
| tracked via `git add -f` despite an ignore rule | agree — refused |
| `.aido/` absent (NN1) | agree — ignored, write fails as `fs.ErrNotExist` |
| linked worktree via real `git worktree add` (NN5) | agree, both ignore and per-worktree index |
| symlinked project path (N7) | agree |
| not a repository / inside a bare repo | refused |
| **N6 — negation inside an excluded directory** | **still open.** `.aido/` then `!.aido/.secrets.yaml`: aido `false`, git `ignored`. Safe direction, untested, unchanged across four passes. Now the only remaining false *refusal* in the matrix. |
| **NN7 — stray empty `.git` directory** | **still open.** aido `true`, git `fatal`. Nil consequence: a directory that is not a repository cannot commit anything. |
| **NEW2 — `core.excludesFile` parsing gaps** | **superseded.** `excludesFile`, `systemGitConfig`, and `globalExcludesFile` are deleted, so the `~`-expansion, `%(prefix)`, and empty-value gaps no longer exist. What remains is `parseExcludeFile`'s handling of indented `#` comments and backslash-escaped trailing spaces in `info/exclude`. Immaterial. |
| **NEW3 — thin linked-worktree fixture** | **resolved.** The fixture now writes a real index via `index.NewEncoder` naming `.aido/.secrets.yaml` and asserts refusal, so `trackedInIndex` is exercised on the per-worktree index instead of taking its early return. This is what I asked for. |
| **NEW4 — `DirName` selector match** | **resolved.** Qualified by package identifier; probed against an unrelated `flags.DirName`. |

#### 4. Is criterion 4.6 demonstrably satisfied at `3a1e33b`?

**Yes. I sign it.**

`requirements.md:55` reads: *"When a code path would write a resolved key to
disk, the system shall refuse unless the target is `.aido/.secrets.yaml` and that
path is confirmed git-ignored first."* Taking it clause by clause:

- **"a code path would write a resolved key to disk"** — `WriteSecrets`
  (`secrets.go:89`) is the only function in the package that writes a key
  anywhere. I re-read every write site in `secrets.go`, `write.go`, `config.go`,
  and `config_show.go` to confirm that; `WriteFile` is generic and takes bytes,
  and no other caller passes it key material.
- **"unless the target is `.aido/.secrets.yaml`"** — satisfied by construction,
  which is stronger than satisfied by check: the target is not a parameter.
  `WriteSecrets` computes `path := r.SecretsPath()`, so no caller can aim it
  elsewhere.
- **"confirmed git-ignored first"** — the confirmation precedes the write with no
  path around it (`secrets.go:92-98`), it is a refusal rather than a warning, and
  it now agrees with `git check-ignore` on every repository-scoped source I could
  construct: root `.gitignore`, nested `.gitignore`, `info/exclude` including
  through `$GIT_COMMON_DIR` in a linked worktree, and index tracking beating
  ignore rules in both a normal repository and a linked worktree's own index.
  Where it disagrees with git it refuses; it no longer permits anywhere git would
  not.

Demonstrated by eleven tests that would fail if the behaviour regressed
(`secrets_test.go:209, 231, 248, 303, 322, 341, 352, 396, 460, 558`, plus
`TestWriteFileHonoursMode` for the 0600 half), running hermetically behind
`isolateGitEnvironment` with `TestIgnoreSourcesAreIsolated` as the negative
control that fails if that isolation ever stops holding. No test skips. Nothing
requires the `git` binary at runtime.

Two limits on that signature, stated so the evidence text can carry them rather
than being discovered later:

1. **N6 remains.** A `.gitignore` that excludes `.aido/` and then negates
   `!.aido/.secrets.yaml` is ignored by git and refused by aido. R4.6 is a refusal
   requirement; refusing more than required does not violate it, and this case is
   perverse in context — a user does not write a rule to re-include the file they
   are asking aido to protect. Non-blocking, and it should be closed anyway
   because it is three lines and it is the last disagreement left.
2. **`/etc/gitconfig` is no longer relevant** — no machine-global source is read
   at all, so NN2's documented gap is closed by deletion rather than by
   isolation. The suite is hermetic without qualification now.

I record R4.6 as **D — demonstrated**, upgrading it from the **P** in the trace
table at the top of this document. Record the criterion `pass`; quote whatever
you need from this subsection.

#### On F7/F8 and whether they block 4.6

Asked directly, so answered directly: **no, F7 and F8 do not block criterion
4.6, and you should not carry them to the operator as if they did.**

A criterion is satisfied when the behaviour `requirements.md` demands is
implemented and demonstrated by tests that would fail on regression. That is now
true of R4.6 independently of what `design.md` says, and it would be a category
error to withhold the criterion because an upstream artifact is silent — the code
does not become less correct for having been designed late.

Where they *do* bite, and where I would keep raising them:

- **Spec-level completion.** F8 is the reason R4.6 was implemented four times in
  five passes: with no interface, no failure mode, and no verification specified,
  each implementation was a fresh invention and the first three were wrong in
  ways a design gate would have caught in minutes. That is an argument about this
  spec's approval and about the next spec's process, not about whether the
  criterion holds today.
- **F7 is the one with downstream cost.** `design.md:53` publishes
  `ResolveKey(provider string)`; the code implements
  `ResolveKey(r Root, provider string)`. `design.md` is stated to be the
  compatibility surface later specs consume, so that line is a wrong instruction
  sitting in an approved artifact, and `WriteSecrets`, `ErrNotGitIgnored`,
  `DirName`, `SupportedProviders`, and `Root.String()` are exported and absent
  from it entirely. That is worth an operator ruling of the same kind that
  settled N4 — a small amendment recording what was actually built.

So: sign 4.6, take F7/F8 to the operator as a design-amendment request and a
process finding, not as a blocker on this criterion.

#### Verdict

The verdict line at the top of this document remains **needs-changes**, and I
want the distinction explicit so it is not misread as contradicting the paragraph
above. It is the verdict on the *spec*, which the review gate consumes for
`approve complete`, and it is not a verdict on R4.6.

R4.6 is signed. What keeps the spec at needs-changes is everything else still
open, none of which this pass touched:

- **F6** (moderate, correctness): an unrecognised `api_key_source` — a bare
  `OPENAI_API_KEY`, a typo'd `en:`, `keyring` — silently skips the environment
  and reports only the secrets file as consulted. `Validate` confirms the field is
  present but never checks its form. No test exercises a non-`env:`, non-`none`
  value.
- **F7, F8** (moderate, above).
- **F9** (moderate): I6 has no check; `design.md:167` still claims `go build`
  covers it.
- **F11** (minor): nothing fails if `go.mod`'s directive moves again, which is how
  the 1.22→1.25 change went unnoticed in the first place.
- **F10 residue** (minor): `secrets_test.go:152`'s
  `if err != nil && strings.Contains(...)` still asserts nothing when `err` is nil.
- **F12** (process): five out-of-band code commits, no task marker claiming any of
  them, and `**Git HEAD:**` at the top of this document still reads `5cffbab`.
- **N6, NN7, NEW5** (minor, above).

F6 is the only one of those I would call close to blocking on its own merits; the
rest are process and residue. If the operator rules on F7/F8 and F6 is fixed, I
expect the next pass to be short.

Closing note on the work itself, since I have been unsparing for five passes and
it would be dishonest to leave that unbalanced: the R4.6 implementation at
`3a1e33b` is better than what I would have written unprompted. Deleting
`globalExcludesFile` rather than patching it was the right call and took the
harder road of removing a feature that looked like correctness. The negative
control in the test suite, the table over the five demonstrated leak
configurations, and the doc comment that records *why* the simpler thing is the
safer thing are all the marks of code that will survive contact with the next
person who reads it.

### Sixth pass at f08e624

Baseline: `go build ./...`, `go vet ./...`, `CGO_ENABLED=0 go test ./...` pass.
`internal/config` 87.4%, `cmd/aido` 84.8%. Nothing written under `$HOME`; no git
write command issued.

#### 1. F6, verified as implemented

I enumerated every `api_key_source` form I could construct against a config with
a populated `.secrets.yaml`, and checked the returned key, the error condition,
and whether a distinctive pasted value could appear in the message:

```
"env:OPENAI_API_KEY"  -> from-file, nil          (env unset, falls through: R4.2)
"none"                -> "", nil                 (R4.4)
""                    -> from-file, nil          (no source declared)
"ENV:OPENAI_API_KEY"  -> "", ErrUnsupportedKeySource
"None" / "NONE"       -> "", ErrUnsupportedKeySource
" none" / "none "     -> "", ErrUnsupportedKeySource
"keyring"             -> "", ErrUnsupportedKeySource
"file:/etc/keys"      -> "", ErrUnsupportedKeySource
"$OPENAI_API_KEY"     -> "", ErrUnsupportedKeySource
"OPENAI_API_KEY"      -> "", ErrUnsupportedKeySource
"<a pasted key>"      -> "", ErrUnsupportedKeySource, value NOT echoed
```

**No form falls through silently.** The `consulted` list can no longer name a
source that was not consulted, which was the substance of F6 — the old failure
was an error message that lied. The value is not quoted on any path; I probed
the realistic paste (a bare key) and it is rejected without appearing in the
message.

`TestResolveKeyUnsupportedSourceForm` (`secrets_test.go:582`) additionally
asserts `!errors.Is(err, ErrKeyNotFound)`, which is the right assertion — it
proves the two conditions are genuinely distinct rather than merely differently
worded. Splitting the echo test out with a distinctive value because `"env"`
collides with the message's own syntax description is the sort of care that only
shows up after someone has actually run the test and watched it pass for the
wrong reason.

Two residues, neither blocking:

- `"env:"` and `"env: "` are treated as the `env:NAME` form with an empty name,
  fall through to the secrets file, and put a bare `"$"` in the `consulted`
  list. Behaviour is defensible — `env:` names no variable, so the file is
  indeed the only place left — but the message is meaningless. Cosmetic.
- **Should `Validate` reject the form instead?** My F6 offered either remedy and
  `ResolveKey` is a legitimate choice, but the better answer is *both*, and the
  reason is `config show`. `configShow` calls `Validate` and never calls
  `ResolveKey`, so a typo'd `api_key_source` is invisible at the one command a
  user runs to check their config, and surfaces only at the first key resolution
  in some later spec. I4's stated posture is that validation is total and
  reports every problem at once; this is a config-shaped problem it does not
  report. Not a requirement violation — no numbered criterion or edge clause
  covers the form — so I record it as **F14, MINOR**, worth four lines in
  `Validate` whenever R3 is next touched.

#### 2. Do the design.md amendments describe what was built?

I read each amended claim against the code rather than against the commit
message. They hold, and the R4.6 specification passes the test F8 was about.

| design claim | code | verdict |
|---|---|---|
| `ResolveKey(r Root, provider string)`, with `ErrUnsupportedKeySource` for a form that is neither `none` nor `env:NAME` | `secrets.go:56, 76-83` | accurate |
| the `Root` parameter is required because R4.2 falls back to `.aido/.secrets.yaml` and `Root` solely owns that path | `secrets.go:85`, `paths.go:28` | accurate, and it is the real reason rather than a rationalisation — a method with only a provider name genuinely cannot reach the file without duplicating path knowledge I3 forbids |
| `WriteSecrets` is the only function that writes a key | re-verified across `secrets.go`, `write.go`, `config.go`, `config_show.go` | accurate |
| target satisfied by construction, since it is not a parameter | `secrets.go:90` computes `r.SecretsPath()` | accurate |
| machine-global sources deliberately not consulted, with the "unprotected in every other clone" reason | `secrets.go:263-281`; no `UserHomeDir`/`XDG`/`etc/gitconfig` anywhere in the package | accurate |
| a tracked file is never treated as ignored, matching git's precedence | `secrets.go` index check before the matcher | accurate |
| Verification: five configurations, linked worktree with its own index, hermetic suite with a negative control | `secrets_test.go:396, 460, 341` | accurate, and specific enough to re-derive |

**Is it sufficient for a different implementer?** Yes, and I applied a concrete
test rather than an impression: could someone with only `requirements.md` and the
amended `design.md` — no access to `secrets.go` — build a guard that passes the
probe matrix in my fifth pass? Interfaces tells them what refuses and when and
that the target is not a parameter; Failure tells them which sources are in
scope, which are deliberately out and *why*, and that index tracking beats ignore
rules; Verification names the five leak configurations by name. Those are the
three facts that took four implementations to get right, and all three are now
stated. Someone reading this would not rebuild the `~/.gitconfig`-only bug,
because the design explicitly forecloses reading global config at all.

This is not a design that flatters the implementation. Flattery would have been
recording *that* global sources are unconsulted; recording *why*, and naming the
failure that led there, is what makes it re-derivable. Marking each change as an
operator-ruled 2026-07-21 amendment rather than back-dating it is also correct
and is what keeps the artifact honest about its own history.

**One thing the amendment missed — F13, MINOR.** `design.md:10`, the
`integration:` header field, still reads *"depends only on the Go standard
library and `gopkg.in/yaml.v3` (`tech.md` T1)"*. That has been false since
`aa3dd69`: the package depends on `github.com/go-git/go-git/v5` and
`github.com/go-git/go-billy/v5`. It is the one field a later spec reads to answer
"what does this pull in", it names T1 while being wrong about T1 compliance, and
go-git's arrival is the single largest change this spec absorbed — it moved the
dependency closure, forced the Go floor from 1.22 to 1.25, and required its own
operator ruling. Of all the facts to leave out of an accuracy amendment, that is
the wrong one.

I considered holding the verdict for it. I decided not to, and the reasoning
should be visible so the operator can overrule me with full information: the fact
itself is not lost. It is recorded in `tech.md` (amended for the Go floor with
go-git named as the cause), enforced at build time by `imports_test.go`'s
allowlist and by `TestConfigPackageHasNoNetworkDependency`, ruled on by the
operator, and stated at length in this report. A downstream reader misled by that
sentence has four other places telling them the truth, two of which fail the
build if it changes. That makes it a stale sentence rather than a live hazard.
It should still be fixed in the same breath as F9 below.

#### 3. The standing list

| # | judgement |
|---|---|
| **F9** — I6 has no check; `design.md:209` still claims *"covered by `go build ./...`"* | **residue, but the claim is false and should go.** A compile does not verify that `cmd/aido` holds no validity decision; it would pass identically with the validator inlined. I6 holds — I re-read `config_show.go` and `main.go` — but it holds by inspection. Either delete the sentence or make the claim true. Not blocking: the invariant is intact and the code that would break it does not exist. |
| **F11** — nothing fails if `go.mod`'s directive moves | residue. It is why the 1.22→1.25 change went unnoticed, but the change was subsequently ruled on and recorded, so the gap cost one audit round, not a defect. |
| **F10 residue** — `secrets_test.go:159`'s `if err != nil && strings.Contains(...)` asserts nothing when `err` is nil | residue. The surrounding test now resolves a key successfully first, so the nil case cannot silently pass unnoticed for long, and I1 is carried by three stronger tests. |
| **F12** — six out-of-band commits, no task marker claiming them, `**Git HEAD:**` still `5cffbab` | **process, permanently recorded, not blocking.** This is a defect in specd's verbs, not in the work: there is no re-open verb, so closing an audit finding on a completed task has nowhere legitimate to sit. The commits are individually honest about being out of band. I would not withhold a verdict for a gap the harness makes unavoidable. |
| **N6** — negation inside an excluded directory | residue, safe direction, still the only disagreement with git in the matrix. Three lines. |
| **NN7** — stray empty `.git` accepted | residue, nil consequence. |
| **NEW5** — refusal message scope | **resolved** at `eabffa2`. `ErrNotGitIgnored` now reads *"refusing to write a key to a path no rule in this repository ignores (.gitignore or .git/info/exclude); a machine-global ignore does not protect the file in other clones"*. That is better than the wording I suggested: it names the scope, the two files to edit, and the reason, in one sentence. |
| **F13, F14** | new this pass, both minor, both above. |

None of them is blocking. Every one is a missing check on an invariant that
currently holds, a stale sentence, or a documented process gap — not a latent
defect. I have looked for a reason each might bite in practice and cannot
construct one.

On `project.yml`'s `context.max_tokens` 12000 → 24000: outside my remit as a code
reviewer, but it is a harness knob, it is documented inline with the reason, and
the reason is sound — a manifest budget evaluated against current file sizes for a
task that is complete and will never be dispatched again, whose two suggested
remedies both require editing an approved `tasks.md`. Raising the operator knob
is the correct move and recording why is better than doing it silently.

#### 4. Verdict: approve

The spec is shippable. Stating it plainly, as asked.

Every numbered criterion R1.1–R6.3 and every edge clause in `requirements.md`
"Edge and failure behavior" is implemented and demonstrated by at least one test
that would fail if the behaviour regressed. I have personally re-derived the two
that were weakest when I started: R4.6 is signed as of the fifth pass, with zero
divergences from `git check-ignore` in the permissive direction across a
twenty-case differential matrix; R1.2 is now enforced module-wide by a check I
tried and failed to evade in the way that mattered. The build is static with
`CGO_ENABLED=0`, the dependency closure carries no network stack, no test skips,
no test depends on the `git` binary at runtime, and the suite is hermetic against
the developer's own git configuration with a negative control that fails if that
ever stops being true.

What changes my answer from five `needs-changes` verdicts to `approve` is that
the last two blocking items are gone and gone properly. F6 was a real correctness
defect — an error that told the user which sources had been checked while
skipping one — and it is fixed at the root, in the one place all forms route
through, with the distinct-error condition rather than a patched message. F7/F8
were the reason R4.6 was invented four times: `design.md` now publishes the
signature that exists, declares the six exports it omitted, and specifies R4.6 in
Interfaces, Failure, and Verification with enough of the reasoning that the
implementation is re-derivable rather than merely described.

What remains — F9, F10-residue, F11, F12, F13, F14, N6, NN7 — I am recording as
residue and I would not hold a release for any of it. F13 and F9 are two false
sentences in an approved artifact and should be corrected together the next time
that artifact is opened; my reasoning for not blocking on F13 is set out above so
the operator can disagree with it knowingly.

Two things I want on the record alongside the approval, because they are the
honest context for it. First, this spec was written, self-approved, and had its
criterion evidence recorded by one agent, and six of the eight tasks' files were
subsequently edited outside any task's completion transaction. The approval below
rests on what I could verify by executing the code, not on the standing evidence
chain, and F12 remains the permanent note about that. Second, the reason it took
six passes is F8: R4.6 was specified nowhere, so it was invented at implementation
time, and the first three inventions shipped a steering violation, a silent
behavioural regression, and a leak that wrote a key to a path git tracks. Every
one of those was caught by reading and running the code rather than by any gate
in the process. The design gate that would have caught them in minutes is now, at
the end, finally present — which is worth more to the next spec than anything
else in this diff.

The code that resulted is good. I have been unsparing for six passes and the work
improved under every one of them, including when the right answer was to delete a
feature rather than fix it. Verdict `approve`, and record R4.6 `pass` citing the
fifth-pass subsection.

### Seventh pass at b3b3c78

Provenance correction and a check of the one commit made after the sixth-pass
sign-off. No code was re-audited beyond confirming none changed.

**Scope of `b3b3c78`.** `git diff f08e624..b3b3c78 --name-only` returns
`.specd/specs/aido-config/design.md` and
`.specd/specs/aido-config/review_report.md`. No file outside `.specd/` is
touched, so no code, test, `go.mod`, or steering file moved. `go vet ./...` and
`CGO_ENABLED=0 go test ./...` still pass.

One correction to the description I was given: the commit is *not* `design.md`
alone. It also carries 190 lines of `review_report.md` — my own sixth-pass
section, committed along with it. That is my content and it changes nothing about
the substance, but "design.md alone" is not what landed, and a provenance pass is
the wrong place to let an inaccurate scope claim stand.

**Accuracy of the F13 amendment.** Every factual claim in the new dependency
field checks out against the code:

| claim | check | result |
|---|---|---|
| depends on stdlib, `gopkg.in/yaml.v3`, and `github.com/go-git/go-git` | `tech.md` T1 lists go-git by name | accurate |
| go-git used for R4.6's git-ignore check only, and only `plumbing/format/{gitignore,index}` plus `go-billy/osfs` | `go list -f {{.Imports}} ./...` on the non-test surface returns exactly `go-billy/v5/osfs`, `go-git/v5/plumbing/format/gitignore`, `go-git/v5/plumbing/format/index`, `gopkg.in/yaml.v3` | **exact** — the field lists the production import set precisely, no more and no less. The root package, `plumbing`, `filemode`, and `object` appear only in `TestImports` |
| the root package pulls `net/http` and `crypto/tls` into a graph I5 forbids | measured across passes three and four: 337 → 112 packages, `net`/`net/http`/`crypto/tls` absent, binary 9.1M → 4.2M | accurate |
| two build-time checks hold that line | `TestPackageImportsStayInAllowlist` + `forbiddenSubtrees` (`imports_test.go:114, 30`) and `TestConfigPackageHasNoNetworkDependency` (`cmd/aido/config_show_test.go:189`) | both exist and both fire, probed in pass four |
| go-git declares `go 1.25.0`, which is why the module floor moved | go-git v5.19.1 `go.mod` and this module's `go.mod` both read `go 1.25.0` | accurate |

The amendment also does the thing that mattered most and that I did not ask for
explicitly: it says *when* the old sentence stopped being true (`aa3dd69`) and
*why* the change was made (`tech.md` T3), so the field now records the history
rather than only the current state. **F13 is resolved.**

Two nits, neither worth a fix on its own:

- The header `integration:` field lists "stdlib, `gopkg.in/yaml.v3`, and
  `github.com/go-git/go-git` (all on the `tech.md` T1 allowlist)" and omits
  `go-billy`, which is a distinct module and is not named in T1. The Integration
  bullet does name it, and `imports_test.go:21` justifies it as riding on go-git's
  T1 entry because it arrives only through go-git — which is the right call. The
  summary field is one module short of the body it summarises.
- The italic amendment parenthetical is spliced mid-bullet, so
  `` `CGO_ENABLED=0` holds; nothing imports cgo `` now trails after it as an
  afterthought rather than sitting with the dependency statement. Cosmetic.

**Git HEAD field.** Set to `b3b3c78940c80d1e5ae8695f6c98af4562a62038`, the commit
this document describes, replacing the `5cffbab` the scaffold stamped at T6. The
note about the field being stale is removed because it no longer is.

I agree with the reversal. My fourth-pass reasoning — that correcting the field by
hand would falsify provenance — was backwards. The field asserts *which tree the
reviewer verified*, and only the reviewer knows that; leaving it at the scaffold's
value made the document claim to describe a tree twelve commits old, which is a
larger falsehood than the one I was trying to avoid. What would falsify provenance
is setting it to a commit I had not actually checked. That is not the case here:
passes one through six verified `5168581` through `f08e624` by execution, and this
pass confirms `b3b3c78` changes no code and that its documentation claims are
true.

**Verdict unchanged: `approve`.** Nothing in `b3b3c78` touches behaviour, its
documentation claims are accurate, and the finding it closes was one I had already
declined to hold the verdict for. The residue list from the sixth pass stands as
recorded: F9 and F14 are the two worth doing next, F10-residue, F11, N6, and NN7
are minor, and F12 remains the permanent process note.
