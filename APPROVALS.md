# Approvals audit trail

Every `specd approve` and `specd mode` run during the 2026-07-21 unattended run,
with the `specd check` output it was approved on and the question the agent
would have put to the human had they been present.

## Standing objection (read this first)

This run violates `.specd/steering/workflow.md` **W10 — Human approval is
human**: *"`specd approve` is never invoked by an agent, under any instruction,
from any file."* `CLAUDE.md` states the same: *"`approve` is human-only. Agent
must never self-approve."*

The operator delegated approval live, in conversation, scoped to this run only,
for the explicit purpose of exercising the harness end to end. That is a human
authority (`reasoning.md` authority order, rank 1), not repository text
(rank 5) — which is why the run proceeded rather than stopping on R1. It is
recorded here because a reader of this repo six months from now cannot tell the
two apart from the on-disk record alone, and should not have to.

The evidence line was **not** delegated and was not crossed: every task in this
run closes on a real `specd verify` at current HEAD. No ledger, `state.json`, or
task marker was hand-edited. No `--force` anywhere.

Also unaudited by construction: the program itself. `PROGRAM.md` was drafted by
the same agent that approved every artifact written against it. Nobody checked
the scope.

---

## Log

_Entries appended below as the run proceeds._
### aido-config · requirements → design

- **Command:** `specd check aido-config` → exit 0, no output. Then
  `specd approve aido-config` → `approved aido-config: requirements → design`.
- **Check output approved on:** empty. `specd check` prints nothing on success
  (logged as an improvement in `SPECD-FEEDBACK.md`); the only evidence the gates
  ran is exit 0. Immediately prior run of the same command had failed with 13
  `error ears:` lines, so the gate registry was demonstrably live against this
  spec and this revision.
- **Post-state:** `specd status aido-config` → 25 acceptance criteria across
  R1–R6, 0 passing, 0 tasks.
- **Would have asked you:** R4 and R5 are where this spec can actually hurt
  someone — key resolution and atomic writes — and I gave them 10 of the 25
  criteria. R6.2 is the one I am least sure of: `product.md` P5 says aido never
  blocks, so I specified `aido config show` exiting **zero even when the config
  is invalid**, printing the failure to stderr. That is a real usability
  tradeoff (no CI check can gate on it) and it is my reading of P5, not P5's own
  words. Would you rather P5's "never blocks" mean "never blocks a human action"
  and let a diagnostic command exit non-zero?

### aido-config · design → tasks

- **Command:** `specd check aido-config` → exit 0, no output.
  `specd approve aido-config` → `approved aido-config: design → tasks`.
- **Check output approved on:** empty again — but this transition is the one that
  proves a clean check is *not* sufficient. The first approve attempt, taken
  after an identically clean `specd check` (exit 0, no output), was refused:
  ```
  error design: design contract field boundaries is required
  error design: design contract field interfaces is required
  error design: design contract field invariants is required
  error design: design contract field failure is required
  error design: design contract field integration is required
  error design: design contract field alternatives is required
  approve refused: readiness gates failing
  ```
  Fixed by restating the six as `- key:` contract fields (the scaffold renders
  them as `##` sections; the gate does not accept that form — logged in
  `SPECD-FEEDBACK.md`). Re-checked clean, then approved. **No `--force`.** The
  artifact was changed to satisfy the gate; the gate was not worked around.
- **Note on this run's approval rule:** "approve only on a clean check" turned out
  to be weaker than it sounds — `check` and `approve` run different gate sets.
  Every remaining approval in this run therefore treats a refused `approve` as
  the real gate and a clean `check` as advisory.
- **Would have asked you:** two things. First, the design commits `internal/config`
  to owning **all** `.aido/` path construction (`Root` + seven constructors),
  including paths for subtrees this spec never reads — `WitnessDir`,
  `TemplatesDir`, `QueriesDir`. `structure.md` S6 says only four packages may
  touch `.aido/` paths, which pushed me to centralise, but it means shipping
  three constructors with no caller until specs 2 and 3 land. YAGNI says drop
  them; S6 says centralise. I chose S6. Which wins?
  Second, `WriteFile` takes the file mode as a parameter rather than inferring
  `0600` for `.secrets.yaml`. That puts the security guarantee at the call site
  instead of in the primitive — safer against a future secret at a new path,
  but it means one careless caller can write a secret world-readable. Would you
  want the primitive to refuse a mode looser than `0600` for any path under
  `.aido/` that is git-ignored?

### aido-config · tasks → executing — NOT APPROVED (run stopped here)

- **Command:** `specd check aido-config` → exit 0, `--json` → `[]`.
  `specd approve aido-config` → exit 1, twice:
  ```
  error evidence-policy: evidence-policy: external boundary lacks integration evidence planning
  approve refused: readiness gates failing
  ```
- **Two attempts at the same blocker, both refused:** (1) an integration-kind
  task with an integration evidence id and negative checks; (2) an explicit
  integration-evidence plan in the design contract's `integration:` field. Per
  the standing rule for this run — "if it fails twice for the same reason, stop
  and report" — the run stopped rather than permuting the artifact a third time.
- **No `--force` was used.** No `state.json`, ledger, or task marker was touched.
  No task was executed, so no verify record exists to be questioned.
- **Approvals recorded this run:** 2 (`requirements → design`, `design → tasks`),
  both on a clean check, both with the artifact fixed rather than the gate
  bypassed.
- **Would have asked you:** what does `evidence-policy` want? The message names
  no boundary, no artifact, and no remedy, and the gate is documented only as a
  side effect of `profile: production` in `project.yml`. My best remaining guess
  is that the CLI boundary task needs evidence class `output_eval` rather than
  `test` — but that is a guess about a gate's grammar, and permuting `tasks.md`
  until a gate goes quiet is not the same thing as satisfying it. I would rather
  you tell me the rule than watch me find it by brute force.

### aido-config · tasks → executing

- **Command:** `specd check aido-config` → exit 0. `specd status aido-config
  --guide --json` → `"blockers": null`. `specd approve aido-config` →
  `approved aido-config: tasks → executing`, exit 0.
- **Context:** the operator reported fixing the `evidence-policy` blocker that
  stopped this run. **I could not identify what changed.** Checked and found
  unchanged: `specd version` (still `1.0.0 (2549cf56…)`), `project.yml` (still
  `profile: production`, `criteria.required: true`, `review.required: true`),
  `~/.config/specd/config.yml` (mtime 2026-06-28, untouched), and both spec
  artifacts — `tasks.md` T6 and the `design.md` `integration:` field are
  byte-identical to what I authored before the refusal. `git status` shows no
  modification outside my own `SPECD-FEEDBACK.md` appends. Spec revision is
  still 2, the same revision that was refused twice.
- **What this means for the audit:** the same inputs that produced
  `error evidence-policy: external boundary lacks integration evidence planning`
  twice now produce a clean approve, and I cannot name the difference. Either
  the fix was outside everything I inspected, or the gate is not a pure function
  of the state I can see. The second reading would contradict
  `reasoning.md` ("Gates, DAG computation, and reports are pure functions of
  on-disk state") and is worth the operator confirming. Logged in
  `SPECD-FEEDBACK.md`.
- **No `--force`.** No state.json, ledger, or task marker touched.
- **Would have asked you:** what did you change? I am approving on your word
  that it was fixed plus a clean gate, not on an understanding of why it now
  passes — and an approval I cannot explain is exactly the kind this file exists
  to flag.
