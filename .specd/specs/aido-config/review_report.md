# Review Report — aido-config

<!--
Filled by the AUDITOR role, not the craftsman who wrote the code. The harness
cannot verify reviewer identity; a craftsman reviewing its own work is an
anti-pattern (see docs/validation-gates.md). Edit the three fields below, then
run `specd approve <spec> complete` with review.required enabled.
-->

- **Git HEAD:** 5cffbab12c21770a0f7a8f9cccc87e02fa4da958
- **Reviewer:** pinky-auditor (subagent, unattended run 2026-07-21)
- **Verdict:** needs-changes — see "Third pass at 6d1fa4b" at the end of this document; that subsection, not the original findings list, is the current call.

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


---

## Re-audit at ddf549b (2026-07-21)

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
