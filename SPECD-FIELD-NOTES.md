# specd Field Notes — Driving the CLI Accurately

Record of one agent session implementing the `home-page` spec through specd
`v1.0.0` (commit `4b70b1d`). Documents what worked, where the tooling misled me,
and concrete recommendations so the next operator/agent drives specd without
guessing.

---

## 1. Context of the experiment

- **Goal:** implement the `home-page` spec (Go + htmx + SQLite task app: projects,
  sidebar, Claude-style composer, per-project task list).
- **Spec state at start:** `status=tasks`, `phase=plan`, `revision=3`,
  `mode=orchestrated`, orchestration enabled. Tasks `T1..T4` already authored.
- **Outcome:** all code implemented and verified green
  (`go build ./... && go vet ./... && go test ./...` → 10 tests pass; live HTTP
  smoke of every route/acceptance path passed). **specd never let me formally
  close a single task** — blocked on gates described below.

---

## 2. What worked as documented

- `specd handshake bootstrap <slug> --json` — clean identity packet (binary
  commit, schemas, digests, tool contracts, next commands). Reliable entry point.
- `specd status <slug> --guide --json` — phase, required artifact, legal vs
  human-only commands, blockers. Accurate.
- `specd context <slug> <task> --json` — bounded context manifest (declared
  files, acceptance IDs, role, skills, digests, token budget). Excellent; this is
  the real contract for a task.
- `specd next <slug> --json` / `--dispatch` — frontier selection returned the
  right eligible task.
- `specd verify <slug> <task>` — ran the task's verify command and appended to
  `evidence.jsonl` + `runs.jsonl`. Worked, but see §3.2 for the trap.

---

## 3. Failures, misguidance, and wasted cycles

### 3.1 `evidence` column format is undocumented and fails late
tasks.md shipped with `evidence: tests` (and `manual`) in every row. That parses
fine and survives `verify`. It only explodes at `complete-task`:

```
QUALITY_DECLARATION_INVALID: "tests" must be class/check-id
```

- **Misguidance:** nothing at authoring/verify time flags the bad value. The
  requirement (`class/check-id`, e.g. `test/db`) is discoverable only by hitting
  the error or reading Go source strings. The valid class set
  (`test|build|lint|integration|negative|coverage|static|manual|...`) is not
  surfaced by any `--help`.
- **Cost:** several edit→verify→complete round-trips to reverse-engineer.

### 3.2 `verify` records evidence that `complete-task` then rejects as "missing"
After fixing the declaration to `test/db`, tests genuinely pass, `verify` says
`evidence recorded`, yet:

```
EVIDENCE_MISSING: task T1 lacks passing evidence for test/db
```

Root cause found only by inspecting the binary: evidence records carry an
`evidence_class` field (`json:"evidence_class"`). **`specd verify <task>` never
sets it** — the rows in `evidence.jsonl` have only
`task_id/command/exit_code/git_head/timestamp/actor`. The completion gate matches
declared `class/check-id` against `evidence_class`, so plain task verify can
**never** satisfy a declared quality check. The only producers of class-tagged
evidence are the worker-report path (`specd brain report …`) and
`verify --criterion`.

- **Misguidance (severe):** the whole documented loop is
  `verify → complete-task`, but in a spec that declares quality checks, `verify`
  cannot produce the evidence `complete-task` demands. The two commands appear
  paired but are not, and nothing says so.

### 3.3 Orchestration ("The Brain") is fail-closed with no actionable hint
`mode=orchestrated`, so completion is meant to flow through the Brain →
worker → `brain report`. But:

```
specd brain run   → brain step: wait  (no dispatch authority or no frontier)
specd brain claim → usage: specd brain claim <spec> <mission-id> <worker-id> <role>
```

- Dispatch authority is `Grant dispatch authority (fail-closed by default)` —
  **human/operator only**; an agent cannot self-grant.
- `claim` needs a `mission-id` that only a dispatched mission produces →
  chicken-and-egg without authority.
- **Misguidance:** the error `"no dispatch authority or no frontier"` conflates
  two very different states and offers no next command. There is no
  `specd brain --help` (`usage: unknown operation for command "brain"`); the
  subcommand list (`start|step|run|status|cancel|resume|claim|heartbeat|report`)
  exists only as a binary string.

### 3.4 The "Pinky" worker agents are for a different harness and absent
specd references `.codex/agents/pinky-craftsman.toml`,
`pinky-validator.toml`, `pinky-scout.toml` (prompt: `"You are the specd Pinky"`).
- The handshake reports `"agent": "codex"` — specd assumes the **codex** harness,
  not the running one. Those `.toml` files are **not on disk**.
- Result: even with authority, the Brain has no worker to dispatch to in this
  environment. No `pinky` agent is registered in the running harness either.

### 3.5 `approve` blocked by a coverage gate that the authored tasks don't clear
```
approve refused: coverage: approved requirement(s) without an implementing
task or deferred disposition: R1, R2, R3, R4, R5
```
- Tasks `T1..T4` *do* reference every criterion (`R1.1 … R5.2`) in their
  `acceptance`/`refs` columns, yet the coverage gate still counts R1–R5 as
  uncovered. The mapping it wants (requirement-level linkage) is not the same as
  what tasks.md expresses, and the gate never says which field it reads.
- This predates the session — a latent authoring defect the spec was allowed to
  reach `tasks` phase with.

### 3.6 `--help` output is thin and sometimes wrong
- `specd brain --help` → `usage: unknown operation for command "brain"`.
- `specd agents inspect --json` → `usage: unknown operation for command "agents"`
  (yet `agents.inspect` is a listed tool contract).
- Command help lists flags but not the *shape/enum* of critical values
  (`evidence` class/check-id, quality classes, mission-id origin).

### 3.7 MCP `status` tool arg mismatch
`status` with `["home-page","--guide"]` → `usage: status slug [--json]`. The
`--guide` had to be passed as a structured flag, not a positional. Positional vs
flag handling differs between the CLI and the MCP surface and is easy to trip on.

---

## 4. Root-cause summary

| Symptom | Real cause | Who can fix |
|---|---|---|
| `QUALITY_DECLARATION_INVALID` | `evidence` col must be `class/check-id`; not validated early | spec author |
| `EVIDENCE_MISSING` after passing verify | `verify` doesn't stamp `evidence_class`; only worker-report/criterion do | specd design / operator |
| `brain run: wait` | dispatch authority fail-closed, human-only | operator |
| Brain has no worker | `pinky-*.toml` target codex harness, absent on disk | operator/setup |
| `approve refused: coverage` | task↔requirement coverage mapping unsatisfied | spec author |

Net: a spec can pass `requirements`/`design` approval and sit in `tasks` phase
while carrying **three independent latent blockers** (bad evidence declarations,
unsatisfiable-by-verify quality checks, broken coverage mapping), none surfaced
until you try to close a task.

---

## 5. Recommendations to drive specd accurately

### For the operator (before handing a spec to an agent)
1. **Wire the harness first.** Confirm `specd handshake` reports the *correct*
   agent harness. If it says `codex` but you run something else, orchestration
   (`brain`) cannot dispatch. Generate/verify the worker configs
   (`.codex/agents/pinky-*.toml` or the equivalent for your harness) on disk.
2. **Grant dispatch authority explicitly** if `mode=orchestrated`. Otherwise the
   agent hits `wait (no dispatch authority)` and cannot close anything. If you
   want a single-actor agent loop instead, set the spec to a non-orchestrated
   mode so `verify → complete-task` is the sanctioned path.
3. **Validate tasks.md before approving into `tasks` phase:**
   - every `evidence` cell is `class/check-id` (not a bare word);
   - each declared quality check is actually producible by the task's `verify`
     (or by a criterion/worker report you intend to run);
   - `verify` commands are non-interactive (see §5, agent rule 6);
   - requirement coverage the `approve` gate wants is satisfied — run
     `specd approve <slug>` as a dry check and read the coverage blocker.

### For the agent driving the CLI
1. **Bootstrap → guide → context, every task.** Never act outside
   `status --guide` legal commands. Treat `context --json` as the authority.
2. **Read the gate model before writing code:** run `specd complete-task` (or
   `approve`) *early, expecting it to fail*, to surface evidence/coverage
   requirements up front instead of after implementation.
3. **Know that `verify` ≠ closable evidence** when quality checks are declared.
   If a spec declares `class/check-id`, plan for the worker-report/criterion path,
   not plain `verify`.
4. **Do not self-grant or fake authority/evidence.** On any authority, digest,
   scope, or gate mismatch: stop and report the exact blocker (this is the
   specd contract). Never hand-edit `state.json`, `evidence.jsonl`, `runs.jsonl`,
   or task markers.
5. **tasks.md is editable** (it is not a protected ledger); fixing a malformed
   `evidence`/`files` cell is legitimate craftsman work, but note it as a
   deviation.
6. **Never author a self-killing verify command.** `go run . & sleep 1; …;
   kill %1` relies on interactive job control and hangs in a non-interactive
   shell (it did — timed out at 120s and orphaned a server). Use an explicit PID:
   `go run . & P=$!; sleep 1; curl -sf …; kill $P`.
7. **Distrust thin `--help`.** For undocumented shapes (evidence classes,
   mission-id origin, brain subcommands) the binary strings are the real spec;
   but prefer asking the operator over reverse-engineering in a loop.

### For specd itself (product feedback)
- Validate `evidence` `class/check-id` at authoring/`check` time, not at
  `complete-task`. List the valid class enum in the error and in `--help`.
- Make `EVIDENCE_MISSING` state *why* — e.g. "verify evidence has no
  `evidence_class`; declared checks require a worker report or `verify
  --criterion`." Or let `verify` stamp `evidence_class` from the task's
  declaration.
- Split `"no dispatch authority or no frontier"` into two distinct messages, each
  with the exact next command to run.
- Provide real `specd brain --help` and `specd agents --help`; align the MCP tool
  names (`agents.inspect`) with working CLI subcommands.
- Fail the `tasks`-phase approval if coverage/evidence declarations are already
  unsatisfiable, so latent blockers can't accumulate silently.

---

## 6. Session deliverable status (for the record)

- **Code:** `internal/db/db.go` (+`db_test.go`, 10 tests), `internal/handlers/handlers.go`,
  templates `index/sidebar/main/composer/list.html`. Build/vet/test green; live
  smoke of all routes and every R1–R5 acceptance path passed.
- **specd ledger:** `verify` evidence recorded for T1–T3; **no task formally
  completed**, spec **not approved** — blocked on operator dispatch authority and
  the coverage gate (§3.3, §3.5).
- **Authoring fixes applied during the session:** corrected all `evidence` cells
  to `class/check-id`; added `internal/db/db_test.go` to T1's declared `files`.
