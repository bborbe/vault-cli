---
status: prompted
approved: "2026-09-30T07:05:44Z"
generating: "2026-09-30T07:20:25Z"
prompted: "2026-09-30T07:20:25Z"
branch: dark-factory/bug-approve-status-removes-row-from-spawn-offer
---

## task approve writes `in_progress`, so an approved row never reaches the spawn offer

## Summary

- `vault-cli task approve` writes **`status: in_progress`** together with `phase: planning` and the two approval keys (spec 056 AC 1).
- The manager sweep's **`ready-to-start`** bucket requires **`status: next`** — in both renderers, the vault-local model-free gate and the plugin's sweep reader.
- So the act of approving a row **removes it from the spawn offer**: it classifies as `🔄 progressing`, which reads as *a session is on it*, and no manager ever offers it.
- Measured 2026-09-30 on two operator-approved rows: a live manager returned **no `To open`** for them, and the only remaining route is the orphan/auto-resume gate, which fails terminally for a `mode: interactive` row.
- No user-visible CLI output changes; the fix is one frontmatter value, and the row's own frontmatter is the observable.

## Problem

`task approve` is the command that turns an operator's decision into work the fleet may start. It records the decision correctly — `phase: planning`, `approved_by`, `approved_at` — and then writes `status: in_progress`, which is the value the sweep reads as *someone already owns this*. Both renderers gate their spawn offer on `status: next`: the vault-local gate returns `🚀 ready-to-start` only inside `if status == "next":` and returns `🔄 progressing` for `status == "in_progress"`, and the plugin's reader states the same rule in words (*"the task is `next`, no session owns it…"*). The two writes therefore cancel: the approval that is supposed to release a row for spawn is the same write that disqualifies it. The row is never offered at any point in its life — before approval it sits at `phase: todo` and is held as waiting on the operator, and after approval it sits at `status: in_progress` and is read as already in flight. This spec was written from a live measurement, not from inspection: two rows the operator approved were swept for hours by two different managers, and the verdict returned for one of them named the dead end exactly — no open action, and a terminal refusal on the resume path.

## Reproduction

Smallest config: this repository at `origin/master` (`e7ab72d`, release v0.155.1), the installed `vault-cli v0.155.0`, the Personal vault holding two task files at `phase: planning` with `approved_by: operator` already recorded, and a manager session sweeping the topic those tasks belong to. Measured against `dark-factory v0.196.0`. The bug is not in `go test`; it is the classification a manager computes from the frontmatter the CLI wrote. The core of it — the value `task approve` writes — reproduces in a container against a temp vault; the classification consequence needs the vault's own sweep gate.

**Part 1 — the write, reproducible in a container.** A task at `phase: todo` approved with the shipped command:

```
$ vault-cli task approve <name> --vault <vault>
✅ Approved <name>: phase planning

$ grep -nE '^(status|phase|approved_by|approved_at):' "<task>.md"
13:phase: planning
16:status: in_progress
2:approved_at: 2026-09-29T22:33:24.568922+02:00
3:approved_by: operator
```

`status: in_progress` — and this is the documented, intended behavior of the shipped command, pinned by spec 056's own acceptance criterion, not an accident of one run.

**Part 2 — the consequence, measured live on 2026-09-30.** Two rows approved by the operator on 2026-09-29 (`22:33:20` and `22:33:24`), each carrying a dead `claude_session_id` and `mode: interactive`, were swept by two live manager sessions. The `Work Approval` manager returned this verbatim for `The Per-Manager Spawn Cap Becomes One Fleet-Wide Concurrent Limit, and an Unready Task Asks the Operator Instead of Being Held`:

```
Not resumed — The Per-Manager Spawn Cap Becomes One Fleet-Wide Concurrent Limit, and an Unready
Task Asks the Operator Instead of Being Held — gate fails on clause 11 (resumable by the path
this gate hands over): mode=interactive
```

and added, in its own words: *"There is no `To open` for this row: it is classified `problem`, not `ready-to-start`, so the drive leg has no open action for it."* Clauses 1–10 of the resume gate all held on the row; only clause 11 failed, and it fails on the **route**, not on the row.

**The classifying reads, quoted from source at the version in force:**

```
# vault-local model-free gate, sweep-gate.py
873:    if status == "next":
...
877:            return "\U0001f504 progressing", "progressing"
...
888:    if status == "in_progress":
889:        return "\U0001f504 progressing", "progressing"

# plugin sweep reader, agents/manager-sweep-reader.md:102
- **ready-to-start** — the task is `next`, **no session owns it**, and **every
  `blocked_by` it declares has shipped (`status: completed`)**
```

A row approved by the shipped command can never satisfy the first clause. Both approved rows rendered `🔄 progressing` in the manager's own sweep at 07:43.

## Expected vs Actual

**Expected.** `task approve` records the operator's decision and leaves the row in the state the fleet is designed to act on. An approved row with no live session is *queued to be started*: `phase: planning`, `approved_by` / `approved_at` present, `status: next`, no owner. The manager's `ready-to-start` bucket is exactly that set, and the drive leg's own clauses — the readiness audit, the prose-blocker check, the sweep-global concurrency cap — then decide whether it is opened or held with its clause named. That is the documented intent of the approval boundary: agents file at `todo` and stop, the operator's flip to `planning` is the approval, and managers open approved rows.

**Actual.** `task approve` writes `status: in_progress`, so the row leaves the offer set at the moment it enters the approved state. It is swept, rendered `🔄 progressing`, and no action line is emitted for it. Its only remaining route is the orphan/auto-resume gate — a mechanism for *resuming a session that died mid-work*, which is not what an approval asks for — and that route terminates for any row carrying `mode: interactive`. The net effect is that the fleet's no-push open path has never once fired for an approved row.

## Why this is a bug

Three sources agree that an approved row is meant to be offered, and the shipped write contradicts all three.

1. **The vault's own status semantics.** `CLAUDE.md` § Task Status Semantics defines `next` as *queued* and `in_progress` as *active*. A row nobody has picked up is queued, not active. Writing `in_progress` asserts an owner that does not exist.
2. **The plugin's own documentation names this exact failure.** `agents/manager-sweep-reader.md:83` warns that a task which *"is `status: in_progress` with no real session … reads as owned and **silently drops out of the ready-to-start set**, which is a missed dispatch rather than a false alarm."* The approval write creates precisely that shape — deliberately, and for every approved row.
3. **Spec 056's own criteria do not require the status write.** SC2 of the parent task requires only that the flip *"also writes `approved_by` and `approved_at`"*. The `status: in_progress` key is an extra write in the same transition, and it is the one that breaks the path the same task's later criterion depends on.

## Workaround

Available to the operator today, until the fix lands: after approving a row, restore it to the offer with

```
vault-cli task set "<task-name>" status next --vault <vault>
```

The row then classifies `🚀 ready-to-start` on the next sweep and the fleet can open it. This is a user-side mitigation, not the fix — it repairs one row at a time and must be repeated for every approval, which is why the writer is the thing that changes.

## Goal

`task approve` records the approval and leaves the row in the state the fleet acts on: after it runs, the file reads `phase: planning`, `status: next`, `approved_by`, and an RFC3339 `approved_at`, and nothing else in the approval transition changes. An approved row therefore classifies `🚀 ready-to-start` in the vault's sweep gate, and a manager tick decides it on its merits — opened if it clears the readiness bar, held with its clause named if it does not. The CLI's stdout is unchanged, the refusal behavior for rows past the inbox is unchanged, and no other key the command touches moves.

## Non-goals

- **Changing the `ready-to-start` predicate.** The bucket's requirement of `status: next` is correct and is shared by both renderers; the write is what is wrong. Widening the predicate would leave approved rows asserting an owner they do not have.
- **Repairing rows that are already approved.** This spec changes what the command writes from now on. Rows already carrying `status: in_progress` are a data condition, not a code path, and are handled outside this spec.
- **Changing the resume gate or clause 11.** That gate's refusal is correct — it declines to hand over a resume its own mechanism cannot execute. This spec does not touch it.
- **Any change to `phase` semantics, the approval keys, or the refusal conditions.** All are frozen by spec 056 and stay as they are.
- **A new scenario.** See the scenario-coverage note under Acceptance Criteria.

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| Widen `ready-to-start` to admit an approved `planning` row regardless of `status` | Leaves the row claiming `in_progress` with no owner — the exact shape `manager-sweep-reader.md:83` documents as a missed dispatch — and requires the same edit in three surfaces (the vault-local gate, the plugin reader, the runbook clause) where the write fix is one value in one place. |
| Clear `claude_session_id` at approval instead | Does not touch the actual cause: the row is excluded by its `status`, not by the presence of a session id. Measured on both approved rows, which were classified `progressing` on `status` alone. |
| Amend spec 056's AC 1 in place | Completed specs are immutable (`rules/spec-writing.md:395`). This spec supersedes that one acceptance criterion and says so. |
| Have the manager act on `status: in_progress` rows with no live session | Makes the manager responsible for detecting a state the writer should not have produced, and puts a second definition of "unowned" next to the sweep's own. |
| Do nothing | See the Do-Nothing Option below. |

## Acceptance Criteria

Fixture convention: each AC that exercises the write runs against a temp vault holding one task file, via the repository's existing `integration/` harness, which builds the real binary and asserts against a real file on disk. **The write-exercising ACs (1, 2 and 5) build their vault with `createTempVaultWithCurrentUser` (`integration/cli_test.go:94`), not `createTempVault`.** The shipped command refuses an unowned row — `resolveOwner` (`pkg/ops/task_approve.go:187-209`) errors with *"assignee must not be empty … refusing to approve an unowned task"* when the fixture has no `assignee`, no `--assignee` is passed, and the vault config carries no `current_user` — and that refusal is pinned by an existing integration test (`integration/cli_test.go:3595`). Against the bare `createTempVault` all three ACs would exit 1 on a correct implementation. AC 3 and AC 4 are unaffected: AC 3 seeds `assignee` among its unrelated keys, and AC 4's fixtures are refused by the phase guard before owner resolution is reached.

- [ ] **The approval transition writes `status: next`.** A `createTempVaultWithCurrentUser` fixture at `status: next`, `phase: todo`, with no approval keys: `vault-cli task approve <name> --vault <vault>` exits 0 and the file's four key lines read exactly `status: next`, `phase: planning`, `approved_by: operator`, and an `approved_at` whose value parses as RFC3339 (strip an optional surrounding pair of double quotes, then `time.Parse(time.RFC3339, value)` succeeds; assert the value is not the zero time) — evidence: the four lines read from the **file**, not from `task get`, plus exit code 0. This supersedes spec 056's AC 1, whose assertion on `status: in_progress` is the behavior being corrected.
- [ ] **The write does not depend on the pre-approval status.** A `createTempVaultWithCurrentUser` fixture at `status: in_progress`, `phase: todo`, with no approval keys: after `task approve` exits 0, the file's `status` line reads `status: next` — evidence: the single `status:` line, read from the file. The command must not preserve a pre-existing `in_progress`, because that is the value that removes the row from the offer.
- [ ] **No other key moves.** On the AC 1 fixture, carrying unrelated keys (`assignee`, `priority`, `page_type`, `task_identifier`, and an unknown `custom_key`): after the command exits 0, every line not matching `^(status|phase|approved_by|approved_at):` is byte-identical to its pre-state, and the whole-slice comparison of the two files with those four prefixes dropped is empty — evidence: negative evidence, the two filtered slices compared, not a count of deletions. This is spec 056's AC 3, retained because the fix edits the same write path.
- [ ] **The refusals are unchanged — all four of them, over six fixtures.** The five that exercise the existing guards: `phase: planning` (already past the inbox), `phase: execution`, and `phase: todo` carrying only `approved_by`, only `approved_at`, and both. The sixth is the owner guard: a `createTempVault` fixture (no `current_user`) at `phase: todo` with no `assignee`. Each `task approve` invocation exits non-zero and that fixture's `sha256` taken immediately before and after is unchanged — evidence: **six exit codes plus six unchanged hashes**, asserted per fixture. The `in_progress` write is not a reason for any of these refusals, and fixing it must not relax them — in particular the owner guard must survive, since it is the one this spec's own fixtures must satisfy rather than trip.
- [ ] **The stdout contract is unchanged.** On a fresh `createTempVaultWithCurrentUser` `phase: todo` fixture, plain output exits 0 and contains the task name and the literal `planning`; `--output json` exits 0 and parses into an object with `name` and `phase == "planning"` — evidence: exit codes plus the parsed object. This is spec 056's AC 9, retained.
- [ ] **`make precommit` exits 0** — evidence: exit code 0, with the suite's own summary line quoted. This is the container's full gate: lint, format, generate, tests, version alignment.
- [ ] **`CHANGELOG.md` carries an `## Unreleased` bullet prefixed `fix:` describing the corrected status write** — evidence: file content, asserted with the section-walking form (`awk '/^## /{sec=$0} /<bullet-pattern>/{print "sits under: " sec}' CHANGELOG.md` returning `## Unreleased`), never a line-window `grep -A`, which swallows a neighbouring section. On a branch where the releaser has already cut a release, assert against the newest `## vX.Y.Z` section instead — same bullet, same prefix.
- [ ] **The repository's own phase-lifecycle doc no longer states the superseded value.** `docs/task-writing.md:467` today reads *"The approval writes `status: in_progress`, `phase: planning`, `approved_by` and `approved_at` together in one write"*; after this change that sentence names the behavior being removed, and the next reader's grep lands on it first. The sentence names `status: next` in place of `status: in_progress`, and the doc carries no remaining approval-related `in_progress` claim — evidence: file content plus **negative evidence** — `grep -n 'status: next' docs/task-writing.md` returns the corrected line, and `grep -n 'status: in_progress' docs/task-writing.md | grep -i approval` returns 0 lines. A line-window `grep -A` is not sufficient here: the section spans several sentences and the assertion must be on the approval sentence specifically.
- [ ] **Post-Deploy (Rung-2): the released binary's approved rows classify `🚀 ready-to-start` in the vault's own sweep gate.** On the released tag, with the gate installed in the vault: approve a fresh `phase: todo` row with the released binary, run the vault's model-free gate once, and confirm the row's Status cell reads `🚀 ready-to-start` and **not** `🔄 progressing`; then confirm a manager tick emits an action line for it — evidence: the row's rendered Status cell plus the manager's action line, quoted. Before/after is the strongest form: the same sequence on the pre-fix binary renders `🔄 progressing` and produces no action line.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0 origin/master)`

**Scenario coverage: no new scenario.** The corrected behavior is reachable end-to-end by the repository's existing `integration/` harness, which builds the real binary and asserts against a real task file — the fix's observable is the file's own frontmatter. The classification consequence named in the Post-Deploy AC lives in the vault's sweep gate, which is not in this repository and cannot be exercised by a scenario here; it is verified on the host against the released binary. None of the four conditions in `docs/rules/scenario-writing.md` holds, so the default applies.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test

# the change landed — the four keys, read from the file
grep -nE '^(status|phase|approved_by|approved_at):' <fixture>.md
# expected: status: next · phase: planning · approved_by: operator · approved_at: <RFC3339>

# the old value is gone (negative form, so a zero match is a pass)
! grep -q '^status: in_progress' <fixture>.md

# the write path — it lives in pkg/ops/task_approve.go:136, NOT pkg/cli/cli.go.
# cli.go contains neither string, so probing it makes both directions vacuous:
# the positive probe can never pass and the negative one passes on the unfixed tree.
grep -n 'SetStatus(domain.TaskStatusNext)' pkg/ops/task_approve.go   # >= 1 line
! grep -q 'SetStatus(domain.TaskStatusInProgress)' pkg/ops/task_approve.go   # zero match = pass
# scoped to the call site, not the whole file: a comment naming the old constant is a
# natural thing to leave behind on a correct fix, and a whole-file grep would fail on it.

# the unit assertion that pins the old value must have moved
grep -n 'TaskStatusNext' pkg/ops/task_approve_test.go                # >= 1 line

# the doc that would otherwise still state the superseded value
grep -n 'status: next' docs/task-writing.md
! grep -n 'status: in_progress' docs/task-writing.md | grep -qi approval

# the changelog bullet sits under Unreleased, not a released section
awk '/^## /{sec=$0} /^- fix:/{print "sits under: " sec}' CHANGELOG.md
```

### Operator-executable (runs on the host after PR merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Version alignment, then install
make release-check
make install
vault-cli --version

# Install the Claude Code plugin manifests (separate artifact)
claude plugin update vault-cli@vault-cli   # then restart Claude Code
```

## Desired Behavior

1. `vault-cli task approve <name>` leaves the task file reading `phase: planning`, `status: next`, `approved_by: <by>`, and an RFC3339 `approved_at`, and changes no other key. The pre-approval `status` is not preserved: a fixture that was `next` and a fixture that was `in_progress` both read `status: next` afterwards.
2. An approved row with no live session classifies `🚀 ready-to-start` in the vault's model-free sweep gate and reaches the manager's drive leg as an open candidate, where the leg's own clauses decide it — opened, or held with its clause named. Neither outcome is this spec's concern; that the row is *offered at all* is.
3. The command's stdout is byte-identical to today's: plain output names the task and the phase, `--output json` carries `name` and `phase`, and both exit codes are unchanged.
4. Every refusal is unchanged — **all four**: a row past the inbox, a row already carrying an approval key, an empty `--by`, and a row with no resolvable owner all still exit non-zero with the same stderr, and their files' hashes are unchanged across the invocation.
5. `make precommit` and `make test` are green, and the four version strings stay aligned — no version bump is hand-written, because the releaser owns it.
6. `CHANGELOG.md` carries an `## Unreleased` bullet prefixed `fix:` describing the corrected status write, naming the consequence it removes (an approved row never reaching the spawn offer).
7. The repository's own phase-lifecycle documentation states the value the command now writes: `docs/task-writing.md`'s approval sentence names `status: next`, and the doc carries no approval-related `in_progress` claim. A behavior change that leaves the repo's own doc contradicting it is not finished.

## Assumptions

- The `ready-to-start` predicate's requirement of `status: next` is correct and stays. It is stated identically in the vault-local gate and in the plugin's reader, and the two must agree — this spec does not move either.
- Spec 056's `status: in_progress` assertion was an implementation choice recorded as an acceptance criterion, not a product requirement. The parent task's SC2 requires only `phase` + `approved_by` + `approved_at`, and the vault's own status vocabulary calls an unowned row `next`.
- No consumer depends on an approved row reading `in_progress`. This is the one assumption the fix could falsify, and the AC 3 key-preservation check plus the Post-Deploy classification check are what would catch it; if a consumer does depend on it, that consumer is the thing to change, because the value asserts an owner that does not exist.
- The releaser owns the version bump and the tag on this repository; this change adds only an `## Unreleased` bullet.
- An approved row does not disappear from `task list`'s default view: `pkg/ops/list.go:212` already admits `next`, `todo` **and** `in_progress`, so membership is unaffected. One side effect is real and harmless: `statusPriority` (`pkg/ops/list.go:218`) sorts `in_progress` at 1 and `next` / `todo` at 2, so an approved row drops one sort band — from just under in-progress work to just under other queued work. Verified, not assumed; no consumer was found that depends on the band.

## Constraints

- `phase: planning`, `approved_by` and `approved_at` keep their exact spellings and formats — `approved_at` stays RFC3339 and the vault's frontmatter re-serialization order is unchanged.
- The refusal conditions are frozen — **all four**: a row whose `phase` is not `todo`, a row already carrying `approved_by` or `approved_at`, an empty `--by`, and a row with **no resolvable owner** (`resolveOwner`, `pkg/ops/task_approve.go:187-209`; added in v0.155.0 and asserted at `integration/cli_test.go:3595`). The owner guard is listed explicitly because this spec's own fixtures must satisfy it rather than trip it, and because a prompt editing the write path could otherwise read it as free to remove.
- No new flag, no `--force`, no bypass. Re-opening a closed transition remains `task set --force phase <earlier>`.
- The command's stdout and its `--output json` shape are frozen.
- Tests follow repository convention: Ginkgo v2 / Gomega, against the `integration/` harness's temp vault. The fixture-authoring rule from spec 056 holds — the storage writer re-serializes frontmatter in alphabetical key order, so every fixture is authored alphabetically or byte comparisons become meaningless.
- `.dark-factory.yaml` is unchanged.
- Paths in this spec and in any generated prompt are repo-relative **for anything the prompts modify**. The spec also cites artifacts that are deliberately *not* in this repository and must not be hunted for in the workspace: `sweep-gate.py` (the Personal vault's model-free sweep gate, vault-local) and `agents/manager-sweep-reader.md` (the claude-supervisor Claude Code plugin) are evidence for the bug, not modification targets; `rules/spec-writing.md` and `docs/rules/scenario-writing.md` are dark-factory repo paths, not this repo's.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---|---|---|---|---|
| A future change re-introduces the `in_progress` write | AC 1 fails: the file's `status` line reads `in_progress`; the Post-Deploy AC then renders `🔄 progressing` | Restore `status: next` on the approval write | `grep -nE '^status:' <fixture>.md` plus the rendered Status cell | Reversible — one value |
| A consumer outside this repository depends on an approved row reading `in_progress` | The Post-Deploy AC renders an unexpected cell, or a dashboard or rollup changes shape | Decide per consumer: an unowned row is `next` by the vault's own vocabulary, so the consumer is the thing to change | The Post-Deploy classification check plus a diff of any dashboard that reads `status` | Reversible — the write can be restored, but a consumer change may not be |
| The refusal guards are loosened while editing the write path | AC 4 fails: a fixture past the inbox exits 0, or its hash changes | Restore the guards; they are independent of the status value | Six exit codes plus six unchanged hashes | Reversible |
| A fixture is authored out of alphabetical order | Byte comparisons fail on a correct implementation, because the writer reorders keys | Re-author the fixture alphabetically | The diff shows only reordering, with no value change | Reversible |
| Rows already approved before this fix keep `status: in_progress` and stay out of the offer | Expected: this spec changes the writer, not the data. Such rows remain classified `progressing` | Repair the data outside this spec — the affected set is identifiable by `phase: planning` plus a present `approved_by` plus `status: in_progress` | A sweep of the vault showing approved rows rendering `🔄 progressing` | Reversible — a frontmatter write |
| A manager sweep reads the row concurrently with the approval write | The sweep observes either the pre-approval file (`phase: todo`, held as waiting-approval) or the post-approval file (`phase: planning`, `status: next`), never a half-written one: all four keys travel in a single `WriteTask`, so there is no intermediate state in which the row is approved but unoffered — which is the state this bug produced persistently | n/a — no recovery needed; the next sweep reads the settled file | None expected. If a row is ever observed with `phase: planning` and `status: in_progress` after this fix, the writer regressed rather than the read racing | Reversible |

## Security / Abuse Cases

Not applicable, and deliberately so: this change adds no input surface and removes none. `task approve`'s existing inputs — `--by` (whose newline-injection containment is proved by spec 056's security boundary check) and `--assignee` — are untouched, as is the symlink refusal on the vault path. The only thing that moves is one value written through the existing `WriteTask` path, and that path's key-preservation guarantee is pinned by the retained AC 3. Recorded explicitly so the omission reads as a decision rather than an oversight.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Change the approval write's status value to `next`, move the assertions that pin the old value (`integration/cli_test.go`'s AC 1 fixture, `pkg/ops/task_approve_test.go:63`), and correct `docs/task-writing.md:467` — one atomic change, since the suite is red until the write and its assertions move together | 1, 2, 3, 4, 5, 7 | 1, 2, 3, 4, 5, 6, 8 | — |
| 2 | `CHANGELOG.md` `## Unreleased` `fix:` bullet | 6 | 7 | prompt 1 |

Rationale: this is a single-layer, single-value fix. The code edit, the two assertions that pin the old value (`integration/cli_test.go:3484`, `pkg/ops/task_approve_test.go:63`) and the docs correction are not independently verifiable — the suite is red until the write and its assertions move together, so splitting them would produce an intermediate tree that fails for a new reason. The Post-Deploy AC is operator-executed after merge and is not a prompt.

## Do-Nothing Option

Every row the operator approves is silently removed from the fleet's offer at the moment of approval. The fleet keeps running, keeps sweeping, keeps rendering those rows — as `🔄 progressing`, which reads as *someone is on it* — and opens none of them. The failure is invisible in exactly the way that makes it durable: the row is present in every sweep, the tally counts it, and the only thing missing is the action line nobody expects to see for work that appears to be in hand. Measured 2026-09-30, the two rows approved the previous evening had been swept for hours by two live managers and drawn no action; the no-push open path has never fired for an approved row. The operator's approval is the one lever this design gives them, and today it does nothing.
