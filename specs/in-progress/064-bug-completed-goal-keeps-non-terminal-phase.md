---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-10-27T22:44:53Z"
generating: "2026-10-27T23:02:26Z"
prompted: "2026-10-27T23:13:29Z"
verifying: "2026-10-27T23:32:04Z"
branch: dark-factory/bug-completed-goal-keeps-non-terminal-phase
---

## A completed goal keeps a non-terminal phase

## Summary

- `vault-cli task complete` lands `phase: done` on every task; `vault-cli goal complete` lands nothing — a completed goal keeps whatever phase it was at when it closed, so `status: completed` / `phase: execution` is a state the CLI produces and no reader can trust.
- The same gap exists on the abort path: `vault-cli goal set <name> status aborted` writes the terminal status and leaves the phase alone.
- This spec makes both terminal transitions write `phase: done` in the **same** `WriteGoal` that writes the status, so the ordinary path cannot create the inconsistency again.
- It also widens lint Rule 2, which today rejects `phase: done` on any status other than `completed` — without that, repairing an aborted goal to `done` fails the CLI's own check.
- Nothing changes for the goals that carry no `phase` key: absent phase stays legal, as both the lint and `docs/goal-writing.md` already assume.

## Problem

`status` and `phase` are orthogonal on a goal exactly as they are on a task — `status` says whether the goal is in play, `phase` says what stage of work is happening inside `in_progress`. For a terminal goal the pair is not orthogonal any more: the only sane terminal phase is `done`, which is what `docs/goal-writing.md` § Phase transitions describes (`todo → planning → execution → done`).

Tasks honour it — `pkg/ops/complete.go:126` calls `task.SetPhase(domain.TaskPhaseDone.Ptr())` on every completion — and goals do not: `pkg/ops/goal_complete.go` contains no `phase` reference at all, and `writeGoalCloseOutFieldsIfCloseOut` (`pkg/ops/frontmatter_entity.go:164`), the shared close-out hook for the `goal set` route, writes only `aborted_reason` and `gate_successor`.

The measured cost, over the Personal vault on 2026-10-27: of 109 completed goals, 6 read `phase: execution`, 4 read `phase: planning`, and 5 read `done` — so `done` is not a convention, it is an accident of which goals happened to be closed through a path that set it. Two aborted goals carry a non-terminal phase as well. A stale phase is what the operator reads on every goal/topic status surface when deciding whether a subject is still moving, so the field is currently informative on tasks and misleading on goals.

## Reproduction

Smallest config: any vault configured in `~/.config/vault-cli/config.yaml` with a `goals_dir`. Against `vault-cli version v0.179.0`:

1. Write a goal file at `<goals_dir>/ZZ Scratch Repro Phase.md` with `page_type: goal`, `status: in_progress`, `phase: execution`.
2. Run `vault-cli goal complete "ZZ Scratch Repro Phase"`.
3. Read the phase back.

```
$ vault-cli goal complete "ZZ Scratch Repro Phase"
✅ Goal completed: ZZ Scratch Repro Phase
exit=0

$ vault-cli goal get "ZZ Scratch Repro Phase" phase
execution
exit=0
```

The file after the successful completion — the status moved, the phase did not:

```
---
page_type: goal
phase: execution
status: completed
---
```

The abort route behaves identically:

```
$ vault-cli goal set "ZZ Scratch Repro Abort" status aborted --reason "probe" --gate-successor none
exit=0

$ vault-cli goal get "ZZ Scratch Repro Abort" phase
execution
exit=0
```

The population-level evidence, over the Personal vault on 2026-10-27:

```
$ vault-cli goal lint --vault private-personal | grep STATUS_PHASE_MISMATCH \
    | grep -E 'status is (completed|aborted)' | wc -l
12
```

12 terminal goals carry a non-terminal phase (10 `completed`, 2 `aborted`). The command reports **71** error lines in total and **13** `STATUS_PHASE_MISMATCH` lines; the 13th is an out-of-scope `status: backlog` / `phase: execution` row (Rule 3), which this spec does not touch.

## Expected vs Actual

Expected, per `docs/goal-writing.md` § Phase transitions (the phase walk ends at `done`): a goal that reaches a terminal status reads `phase: done`, and the same command writes both fields. This is the contract tasks already hold — `pkg/ops/complete.go:126`.

Actual: `goal complete` writes `status: completed` and leaves the phase untouched; `goal set … status aborted` writes `status: aborted` and leaves the phase untouched; and lint Rule 2 rejects `phase: done` on `aborted`, so an operator who repairs the field by hand trades a Rule-1 error for a Rule-2 error. The CLI both produces the bad state and refuses the good one.

## Why this is a bug

A lifecycle field documented as a linear walk that ends at `done` is not being written at the end of the walk, on the one entity kind where the walk terminates through a CLI command. The observable consequence is a field that reports `execution` for work that finished months ago. It is not a feature request: the write path exists, the field exists, the terminal value exists, and tasks already do it.

## Goal

A goal that reaches a terminal status through the CLI reads `phase: done`, written by the same `WriteGoal` that writes the status, on both the completion and the abort route — and the lint accepts that state instead of rejecting it, so the invariant is self-consistent end to end.

## Non-goals

- Repairing the existing offenders in any vault. This fixes the write path and makes the lint able to accept the repair; converting those files is an operator action against a vault, not a code change.
- Populating `phase` on completed goals that carry none. Absent phase is a legitimate state that both the lint and `docs/goal-writing.md` treat as exempt; migrating them has no behavioural gain.
- Reconciling the vault's Goal Lifecycle Guide, whose grid still lists `aborted` with phase `(any)`. That page lives in a vault, not in this repo; it is updated separately once this ships.
- Changing anything about topic phase. Topics have no `complete` verb today; if one appears, the same rule applies and belongs in its own spec.
- Changing Rule 1 or the absent-phase early return. Both are load-bearing for the phase-less exemption.
- Do NOT add a `--force` / `--skip-phase` flag or a config key that suppresses the phase write — the write is the fix, and a bypass re-opens the hole this spec closes. Invariant.
- Do NOT widen Rule 2 beyond `aborted` — a blanket "phase done ⇒ no issue" short-circuit would stop flagging `phase: done` on an `in_progress` goal, which is a real inconsistency.

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| Repair the offender files by hand and stop there | Fixes the symptom for one vault on one day; the next `goal complete` re-creates it, and the reproduction is a single CLI invocation anyone can repeat. |
| Clear `phase` on terminal goals instead of setting `done` | Makes the census look clean by deleting the evidence. It also cannot be distinguished from a correct repair by any count of non-terminal phases, which is why the acceptance criteria assert the `done` delta rather than a zero count. |
| Set the phase in a second `WriteGoal` after the status | Two writes are two failure points and a torn state in between; the existing code already batches reason + successor + status into one write for exactly this reason. |
| Make lint Rule 2 accept `done` on every status | Would stop flagging `phase: done` on an `in_progress` goal, which is a real inconsistency. The widening is scoped to `aborted`, and AC 10 locks that scope. |
| Do nothing | See the Do-Nothing Option below. |

## Acceptance Criteria

Fixture vault: a temp vault with `Goals/Alpha.md` and a config naming `goals_dir: Goals` (the shape the existing `integration/` harness builds). `<bin>` is the binary built from HEAD — `gexec.Build` inside `integration/`, or `/tmp/new-vault-cli` on the host. Unless an AC says otherwise, `Alpha`'s frontmatter holds only `page_type: goal`, `status: in_progress`, `phase: execution` — no close-out fields, so nothing but the phase can move.

- [ ] `<bin> --config <cfg> goal complete Alpha` exits 0, and `Goals/Alpha.md` afterwards contains `status: completed` **and** `phase: done` — evidence: exit code 0 plus file content, read back through `<bin> --config <cfg> goal get Alpha phase` printing `done`. ⚠️ The CLI's own printed output is **not** this AC's evidence: `goal complete` prints only `✅ Goal completed: <name>` today, and a `MutationResult` field could be populated with a hardcoded literal while the file stays at `execution` — the read-back is what discriminates.
- [ ] `<bin> --config <cfg> goal complete Alpha --reason "probe" --gate-successor none` exits 0, and one write carries `status: completed`, `phase: done`, `aborted_reason: probe` and `gate_successor: none` — evidence: file content plus a behavioral case asserting `WriteGoalCallCount()` equals 1 with all four fields on the written goal. The completion route has its own close-out block (`setCompletedStatus`), so the aborted-route evidence does not transfer to it.
- [ ] `go test ./pkg/ops/...` exits 0, and the suite contains a behavioral case asserting the goal-completion operation performs **one** `WriteGoal` carrying both the terminal status and `phase: done` — evidence: exit code 0 plus the case asserting `WriteGoalCallCount()` equals 1 (precedent: `pkg/ops/complete_test.go` asserts the same count for the task path) and asserting the written goal's phase is `done`. A case that only asserts the returned `MutationResult` does not satisfy this AC — atomicity is not observable from the result.
- [ ] `<bin> --config <cfg> goal set Alpha status aborted --reason "probe" --gate-successor none` exits 0, and `Goals/Alpha.md` afterwards contains `status: aborted` **and** `phase: done` — evidence: exit code 0 plus file content plus `goal get Alpha phase` printing `done`.
- [ ] `<bin> --config <cfg> goal set Alpha status completed` (Alpha at `phase: execution`) exits 0, and `Goals/Alpha.md` afterwards contains `status: completed` **and** `phase: done` — evidence: exit code 0 plus file content plus `goal get Alpha phase` printing `done`. This is the `goal set` completion route, a different function from the `goal complete` route ACs 1–3 cover.
- [ ] `<bin> --config <cfg> goal set Alpha status in_progress` on a goal at `phase: execution` exits 0 and `goal get Alpha phase` still prints `execution`; the same holds for `<bin> --config <cfg> goal defer Alpha --until <date>` and `<bin> --config <cfg> goal approve Alpha` — evidence: negative evidence (phase unchanged after each command, read back from the file). A hook that sets the phase on **every** status write fails here.
- [ ] `<bin> --config <cfg> goal set Alpha status aborted --reason "probe" --gate-successor none` performs one write — evidence: a behavioral case in `go test ./pkg/ops/...` asserting `WriteGoalCallCount()` equals 1 for the `goal set` close-out path with the written goal's phase at `done`. This AC exists because the two terminal routes are two separate functions; a fix to one leaves the other broken.
- [ ] `<bin> --config <cfg> goal lint` on a goal at `status: aborted` / `phase: done` exits 0 with no line naming it, and on the same goal at `status: aborted` / `phase: execution` exits non-zero with exactly one line matching `status is aborted but phase is execution` — evidence: exit codes plus negative evidence (zero matching lines on the first) plus stdout line count and content on the second. The first half is the Rule-2 widening; the second is the Rule-1 regression lock, so the widening cannot disable the detector.
- [ ] `<bin> --config <cfg> goal lint` on a goal at `status: completed` with **no** `phase` key exits 0 with no line naming it, and on the same goal at `status: completed` / `phase: execution` exits non-zero with exactly one line matching `status is completed but phase is execution` — evidence: exit codes plus negative evidence plus stdout line content. Absent phase must stay exempt, and the detector that reports the historical offenders must survive the change.
- [ ] `<bin> --config <cfg> goal lint` on a goal at `status: in_progress` / `phase: done` exits non-zero and its output contains exactly one line naming it, matching `phase is done but status is in_progress` — evidence: exit code non-zero plus stdout line content. This is the scoping lock: a blanket `phase == done ⇒ no issue` short-circuit satisfies ACs 8 and 9 and fails here.
- [ ] `docs/goal-writing.md` § Phase transitions states that a terminal status (`completed` or `aborted`) carries `phase: done` and that the CLI writes it in the same mutation, and `CHANGELOG.md` carries a `fix:` bullet in the newest unreleased position — evidence: file content — `grep -nE 'aborted.*done|done.*aborted' docs/goal-writing.md` returns ≥ 1 line, and the bullet appears under `## Unreleased` if that section exists, otherwise under the newest `## vX.X.X` section (the repo's releaser renames `## Unreleased` when it cuts a release, and HEAD currently has no `## Unreleased` section at all).
- [ ] **Post-Deploy (Rung-2):** the released binary is installed and the reproduction replay runs against it — on a scratch goal at `phase: execution` in a temp vault, `vault-cli goal complete "<scratch>"` then `vault-cli goal get "<scratch>" phase` prints `done`, and the pre-existing reproduction from the Problem section no longer reproduces — evidence: exit code plus the read-back output plus the recorded replay transcript in the source task's `# Results` section.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`

**Scenario coverage: no new scenario.** Both routes are reachable by unit tests over the operations with Counterfeiter mocks (the `WriteGoalCallCount` assertion is exactly the layer that can see atomicity), and the lint contract is reachable through `goal lint` against a temp vault. No external service, no cluster, no Docker. None of the four conditions in `docs/rules/scenario-writing.md` holds, so the default applies.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
grep -n 'SetPhase' pkg/ops/goal_complete.go pkg/ops/frontmatter_entity.go   # ≥1 line each
grep -nE 'aborted.*done|done.*aborted' docs/goal-writing.md                 # ≥1 line
grep -c '^- fix:' <(sed -n '/^## /,/^## /p' CHANGELOG.md)                   # ≥1 in the newest unreleased section
```

### Operator-executable (runs on the host after PR merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Reproduction replay against the fresh binary (see Acceptance Criteria fixtures)
/tmp/new-vault-cli --config <fixture-config> goal complete Alpha
/tmp/new-vault-cli --config <fixture-config> goal get Alpha phase     # prints done

# Version alignment, then install
make release-check
make install
vault-cli --version

# The vault repair is a separate operator action, recorded not asserted
vault-cli goal lint --vault private-personal | grep STATUS_PHASE_MISMATCH \
  | grep -E 'status is (completed|aborted)' | wc -l     # 12 before the repair, 0 after
```

## Desired Behavior

1. `vault-cli goal complete <name>` writes `phase: done` in the same `WriteGoal` that writes `status: completed`, on every completion — including one that carries `--reason` / `--gate-successor`, whose close-out fields land in that same write.
2. `vault-cli goal set <name> status aborted --reason … --gate-successor …` writes `phase: done` in the same `WriteGoal` that writes `status: aborted`, and `vault-cli goal set <name> status completed` behaves identically to DB 1. Every **non-terminal** status transition — `goal set … status in_progress`, `goal approve`, `goal defer`, `goal clear`, and `task complete` — leaves a goal's phase untouched: the phase write is confined to the two terminal transitions in DB 1 and this one.
3. Lint Rule 2 accepts `phase: done` on `status: aborted` as well as on `status: completed`, and on nothing else — `status: in_progress` / `phase: done` stays an issue. Its message for a genuine mismatch keeps naming both values, and Rule 1 keeps reporting `aborted`/`completed` with a non-terminal phase. A goal whose `phase` key is absent remains exempt — the early return is unchanged.
4. The two operations carry behavioral Ginkgo v2 coverage asserting `WriteGoalCallCount() == 1` and the written goal's phase, plus `DescribeTable`/`Entry` rows for the lint rule covering the five status × phase combinations in the acceptance criteria. `docs/goal-writing.md` § Phase transitions states the terminal pair, and `CHANGELOG.md` carries a `fix:` bullet.

## Assumptions

- `domain.GoalPhaseDone` and the goal entity's phase setter already exist and are unchanged — the field and its enum landed in v0.100.0 and `goal set <name> phase <value>` already writes it.
- `WriteGoal` is the single persistence call on both routes, so setting the phase on the in-memory goal before that call is what makes the write atomic; no new storage method is needed.
- A goal at `phase: done` that is later reopened (`status: in_progress`) is a manual operator action and out of scope — this spec does not add a reopen path.

## Constraints

- `pkg/ops/` stays free of stdout and of `time.Now()`: the phase is set on the in-memory entity, and the operations keep returning `MutationResult`. The CLI layer formats output; no new print statement is added to the operations.
- Both operations keep their current `MutationResult` shape. This spec does not add `Status` or `Phase` fields to `MutationResult` — the acceptance criteria deliberately read the phase back from the file rather than from the CLI's own output.
- The single-`WriteGoal` contract is frozen for both routes. A change that writes the status and the phase in two calls does not satisfy this spec even if the end state is correct.
- Rule 1 and the absent-phase early return are unchanged; only Rule 2's status test widens, and only for `aborted`.
- `STATUS_PHASE_MISMATCH` keeps its existing issue type and message vocabulary — no second issue type for the same condition.
- The `goal set` refusal for `todo → planning` and the `goal approve` approval record are untouched.
- Tests follow repository convention: Ginkgo v2 / Gomega, Counterfeiter mocks, `DescribeTable`/`Entry` for the table-driven lint rows.
- Entity-name resolution, file permissions (0600), and the symlink refusal in the write path are unchanged.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---|---|---|---|---|
| Goal already at `status: completed` when `goal complete` is invoked | Refused with `goal %q is already completed`; nothing written, phase untouched | None needed — the goal is already terminal | Exit code non-zero plus the message | Reversible (no write) |
| Two CLI invocations against the same goal at once (or one racing a vault autocommit) | Last writer wins; the file is never torn, because the phase rides the same single `WriteGoal` as the status | Re-run `goal complete` — the second call refuses with `goal %q is already completed`, so the state is idempotent | File content shows both terminal fields | Reversible |
| `goal complete --force` on a goal with open tasks | Completes anyway and writes `phase: done`, exactly as the non-forced path does | Reopen by hand if the completion was wrong | File content shows both terminal fields | Reversible — the file can be edited back |
| `goal set <name> status in_progress` on a goal at `phase: done` | Phase is **not** touched — a non-terminal status never writes a phase | Operator sets the phase explicitly if a reopen is intended | `goal get <name> phase` still prints `done` | Reversible |
| A pre-existing file at `status: completed` / `phase: execution` | Unchanged by this spec; reported by `goal lint` on the next run | Operator repairs it (set `phase: done`, or leave it and accept the lint line) | The lint line naming the file | Reversible |
| A pre-existing file at `status: aborted` / `phase: execution` | Unchanged; reported by Rule 1 | Same as above | Same | Reversible |
| Crash between the in-memory mutations and `WriteGoal` | No partial state — nothing is persisted until the single write | Re-run the command | The file is unchanged | Reversible |

## Security / Abuse Cases

- **Attacker-controllable input:** none new. The phase value written is a constant (`done`), not derived from user input; the entity name and the close-out strings are the existing attack surface and are unchanged by this spec.
- **Trust boundaries:** the write stays inside the configured goals directory through the existing entity-name lookup; the symlink refusal is unchanged.
- **What can hang or retry forever:** nothing — both routes remain find, mutate, single-write sequences with no retry loop and no network I/O.
- **What must be validated:** nothing new. The phase value is not user-supplied, so no new validation surface is introduced.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Both terminal write paths set `phase: done` in their existing single `WriteGoal` (completion operation + `writeGoalCloseOutFieldsIfCloseOut`), with behavioral `WriteGoalCallCount` coverage and the non-terminal confinement locked | 1, 2 | 1–7 | — |
| 2 | Lint Rule 2 widened to treat `aborted` as terminal **and nothing else**; `DescribeTable` rows over the five status × phase combinations; Rule 1 and the absent-phase exemption regression-locked | 3 | 8–10 | — |
| 3 | `docs/goal-writing.md` § Phase transitions + `CHANGELOG.md` `fix:` bullet | 4 | 11 | prompts 1, 2 |

Rationale: prompts 1 and 2 are independent — the writer and the detector touch different files and neither's tests depend on the other's. Prompt 3 documents the surface they create, so it lands last. AC 12 is operator-executed after merge and is not a prompt.

## Do-Nothing Option

`goal complete` keeps landing a terminal status with a stale phase, the existing offenders stay wrong, and any operator who repairs one by hand trades a Rule-1 lint error for a Rule-2 one — so the field stays unusable for goals at exactly the moment it is most needed, which is deciding whether a subject is still moving. Every future completion re-creates the inconsistency, and the lint cannot be made clean without the code change.
