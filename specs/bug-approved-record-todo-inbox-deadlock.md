---
status: draft
kind: bug
created: 2026-11-08
---

## Summary

- A task carrying `approved_by` / `approved_at` while its `phase` still reads `todo` cannot leave the approval inbox by any documented verb.
- `task approve` refuses it as already approved. `task set phase planning` refuses it as the approval inbox, and `--force` does not bypass that refusal. `task work-on` refuses it and tells the operator to run `task approve` — the one verb that also refuses.
- The row is approved but unopenable: it renders as waiting-approval forever, and nothing reports a problem.
- The repair: `task approve` becomes idempotent on a row that already carries an approval record — it advances `todo → planning` and preserves the existing record verbatim, writing no new approval.
- `task lint` gains a fixable `APPROVED_RECORD_IN_TODO_INBOX` rule whose fix is that same phase advance, so the state is both reported and repairable.

## Problem

`vault-cli` has two guards that each assume the other owns the row. `refuseExistingApprovalRecord` refuses `approve` when an approval record is present; `todoPlanningSetRefusal` refuses `set phase planning` when the phase is `todo`. Nothing in the `set` path clears the approval record, and `checkPhaseRegression` returns early on `--force`, so a forced phase reset to `todo` leaves the record behind. The result is a state each guard reads as the other's responsibility, and no documented verb moves it. It is silent and durable: the task looks normal in every listing and simply never advances.

A partial record — only one of the two keys — produces the same deadlock. It can only arise from a manual frontmatter edit, since `task approve` writes both keys atomically, but it is the same state and the same repair covers it.

## Goal

A task that is approved but sitting in the approval inbox is advanced to `phase: planning` by `vault-cli task approve`, with the approval record it already carries left exactly as it was. `vault-cli task lint` reports that state as a fixable defect, and `task lint --fix` repairs it the same way.

## Non-goals

- Changing `phase: todo` as the operator's approval boundary, or weakening `todoPlanningSetRefusal` for a row that carries no approval record.
- Clearing the approval record when a reset regresses a task to `todo`. See Constraints for why this candidate was rejected.
- Adding a new CLI verb or flag to move an already-approved row.
- Adding `issue_type` to the lint JSON output. The new rule is verified through plain output; widening the JSON contract is separate work.
- Goal and topic rows. `goal approve` / `topic approve` carry the identical guard pair and therefore the same deadlock, but they have no observed victim and this spec's regression guard is task-scoped; they get a sibling spec.
- Any change to how a manager loop or the vault UI renders a `phase: todo` row.

## Assumptions

- The deadlock is reached through a forced phase reset (`task set phase todo --force`) or a phase write that did not land. A partial record can otherwise arise only from a manual frontmatter edit, because `task approve` writes both keys atomically.
- `task approve` is the documented sole verb that advances a `todo` row (`docs/task-writing.md:519`).
- Goal and topic rows exhibit the same deadlock but are out of scope (see Non-goals).
- The lint fix may rewrite `phase` in the same way the `approve` verb does; no other lint rule in the codebase mutates `phase`, so this is the first such rule and needs no precedence logic against a competing fixer.

## Acceptance Criteria

- [ ] `vault-cli task approve <name>` on a task at `phase: todo` carrying both `approved_by` and `approved_at` exits 0 and leaves the file at `phase: planning`. Evidence: `vault-cli task show <name>` prints `Phase: planning` and the exit code is `0`.
- [ ] The same run leaves `approved_by` and `approved_at` byte-identical to their pre-run values. Evidence: `grep -E '^approved_(by|at):' <file>` before and after the run produces identical output.
- [ ] `vault-cli task approve <name>` on a task at `phase: todo` carrying only one of the two keys exits 0, moves `phase` to `planning`, and does **not** add the missing key. Evidence: `grep -c '^approved_at:' <file>` returns `0` when the row carried only `approved_by`, and `grep -c '^approved_by:' <file>` returns `1` after the run.
- [ ] The negative arm holds: `vault-cli task approve <name>` on a task at `phase: todo` carrying **no** approval record still records an approval and advances the phase. Evidence: `grep -E '^approved_(by|at):' <file>` returns two lines and `vault-cli task show <name>` prints `Phase: planning`.
- [ ] The guard's own purpose survives: `vault-cli task set <name> phase planning` on a task at `phase: todo` carrying no approval record still exits non-zero with the approval-inbox refusal. Evidence: non-zero exit and stderr containing `the task is at phase "todo"`.
- [ ] `vault-cli task lint` reports `APPROVED_RECORD_IN_TODO_INBOX` for a task carrying `approved_by` at `phase: todo`. Evidence: `vault-cli task lint | grep -c 'APPROVED_RECORD_IN_TODO_INBOX'` returns `1`, and the matching line begins with `WARN` (fixable, not yet fixed).
- [ ] `vault-cli task lint --fix` repairs that row and preserves its record. Evidence: after the run `vault-cli task show <name>` prints `Phase: planning`, `grep -E '^approved_(by|at):' <file>` is unchanged, and a second `vault-cli task lint | grep -c 'APPROVED_RECORD_IN_TODO_INBOX'` returns `0`.
- [ ] `make precommit` exits 0. Evidence: exit code.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — format, generate, lint, tests, version alignment all clean
- `make test` — the Ginkgo suite passes, including the new specs
- `grep -n 'APPROVED_RECORD_IN_TODO_INBOX' pkg/ops/lint.go` — the new issue type exists

### Operator-executable (runs on the host after PR merge, spec verification ladder)

- `git tag --sort=-creatordate | head -1` — the merge cut a release tag
- `vault-cli --version` — the installed binary reports that tag after the plugin update
- Replay the reproduction against a throwaway vault (`--config` pointed at a scratch vault) and confirm all four verbs now behave as Expected vs Actual states. Per `docs/releasing-vault-cli.md`, the scenario gate runs against a freshly built binary, never the installed one.

## Desired Behavior

1. `task approve` on a `phase: todo` task carrying neither key behaves exactly as today: one write setting `phase: planning`, `approved_by`, `approved_at`, and the owner when it differs from the task's current assignee.
2. `task approve` on a `phase: todo` task carrying **both** keys advances `phase` to `planning`, leaves `approved_by` and `approved_at` byte-identical, and exits 0.
3. `task approve` on a `phase: todo` task carrying **one** key advances `phase` to `planning` and leaves that key byte-identical. It does not fabricate the missing key, and it does not refuse.
4. Every existing refusal is unchanged: a task not at `phase: todo` is refused with the phase-naming message, an empty approver is refused before the task is read, and an unowned task with no `--assignee` and no configured `current_user` is refused.
5. `vault-cli task lint` reports one `APPROVED_RECORD_IN_TODO_INBOX` issue, marked fixable, for a task carrying `approved_by` or `approved_at` at `phase: todo`.
6. `vault-cli task lint --fix` advances that task's `phase` to `planning`, preserves the approval record verbatim, and a second lint run reports zero such issues.
7. `task work-on` is unchanged. Its refusal — *"task is at phase `todo`, not yet approved; run `vault-cli task approve …` first"* — becomes an actionable instruction rather than a dead end, because the verb it names now succeeds. The loop closes without a second code path.

## Constraints

- `todoPlanningSetRefusal` and `checkPhaseRegression` are unchanged. `--force` still does not bypass the approval-inbox refusal for a row carrying no approval record.
- The repair path writes no `approved_by` / `approved_at` value — not even to complete a partial record. An approval nobody gave must never be recorded.
- `MutationResult`'s shape and the CLI's printed success line are unchanged.
- `task set` semantics are unchanged.
- No new CLI verb, subcommand or flag.
- No `go.mod` / `go.sum` change.
- **Rejected candidate — `--force` clears the approval record on a reset.** It silently discards an approval that was genuinely given, and it cannot repair a row already stuck in the field, which is the state this spec exists to make recoverable.
- **Rejected candidate — a new explicit flag on `task set`.** It adds a third verb for a state that should be self-healing, and leaves the documented path broken: every operator-facing refusal message already tells the reader to run `task approve`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| A task sits at `phase: todo` with an approval record and `task approve` runs | Phase advances to `planning`, record preserved, exit 0 | none needed — this is the repair |
| A task carries a partial record (one key only) | Phase advances, the partial record is preserved as-is | `task clear <name> approved_by` (or `approved_at`) if the stray key is wrong |
| Two sessions run `task approve` on the same row concurrently | Both read the same pre-state; the last write wins; the phase is `planning` and the record is unchanged in either order | none — the operation is idempotent |
| `task lint --fix` runs over a vault with many such rows | Each row advances to `planning`, each record preserved | `task set <name> phase todo --force` re-inboxes a row whose advance was wrong (note: it does not clear the record, so the row returns to the same repairable state) |
| A row carries `approved_by` with an empty value | Treated as present, so the repair advances the phase and preserves the empty value | `task clear <name> approved_by` |

## Reproduction

Fixture: a throwaway vault created for the reproduction, configured through `--config` so no real vault is touched.

0. Record the buggy binary: `vault-cli --version` → `v0.187.1`.
1. Create `/tmp/vc-fixture/config.yaml` with one vault pointing at `/tmp/vc-fixture/vault`, `tasks_dir: tasks`, `goals_dir: goals`.
2. Create `/tmp/vc-fixture/vault/tasks/Fixture Task.md` with frontmatter `status: next`, `phase: todo`, `priority: 2`.
3. Approve it:
   ```
   vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task approve "Fixture Task"
   ```
   Observed: `✅ Approved Fixture Task: phase planning` — the file now carries `approved_by: operator`, `approved_at: <ts>`, `phase: planning`.
4. Make it an active row and move it past planning, then force it back to the inbox:
   ```
   vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task set "Fixture Task" status in_progress
   vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task set "Fixture Task" phase execution
   vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task set "Fixture Task" phase todo --force
   ```
   Observed: `✅ Set phase=todo on: Fixture Task`. The file now reads `approved_by: operator`, `approved_at: <ts>`, `phase: todo`, `status: in_progress` — the deadlocked state.
5. Run all four verbs against it (verbatim output, exit code in brackets).

`vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task approve "Fixture Task"` → `[1]`
```
Error: refusing to approve "Fixture Task": task already carries an approval record (approved_by, approved_at); re-approving is refused
```

`vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task set "Fixture Task" phase planning` → `[1]`
```
Error: refusing to set phase "planning" on "Fixture Task": the task is at phase "todo", the operator's approval inbox. Leaving it is the operator's approval and is performed by `vault-cli task approve "Fixture Task"`, which records approved_by and approved_at in the same write
```

`vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task set "Fixture Task" phase planning --force` → `[1]` — byte-identical to the previous refusal.

`vault-cli --config /tmp/vc-fixture/config.yaml --vault fixture task work-on "Fixture Task"` → `[1]`
```
Error: refusing to work on "Fixture Task": task is at phase "todo", not yet approved; run `vault-cli task approve "Fixture Task"` first
```

## Expected vs Actual

**Expected**, per `docs/task-writing.md` § Phase transitions: the `todo → planning` flip "is the operator's approval, performed by `vault-cli task approve <task-name>`". A row that already carries the approval record has, by that contract, already been approved, so the verb whose whole job is to perform the approval must complete the remaining half of it — the phase move.

**Actual:** `task approve` refuses because the record is present, and every other documented route refuses because the phase is `todo`. The row is approved and unopenable at once.

`docs/task-writing.md:519` also documents the refusal verbatim — "The command refuses any row that is not at `phase: todo`, and any row that already carries `approved_by` or `approved_at`, writing nothing in either case." So the refusal is not an undocumented implementation addition. The defect is that the contract states two clauses that are jointly unsatisfiable for a row carrying the record: line 519 makes `task approve` the only verb that advances a `todo` row, and line 519 also makes it refuse that same row. The contract is incomplete, not merely the code — which is why prompt 3 amends it.

One correction to the impact report that opened this work: not *every* phase write refuses. Only the `planning` target is refused — `task set phase execution` (also `ai_review`, `human_review`, `done`) succeeds from `todo`, with or without `--force`. So an undocumented escape exists, but it skips the planning gate entirely, and `task work-on` — the documented entry path — is blocked.

## Workaround

Until the fix ships, clear the record and re-approve:

```
vault-cli task clear "<name>" approved_by
vault-cli task clear "<name>" approved_at
vault-cli task approve "<name>"
```

This is a hand repair that loses the original `approved_at` timestamp and the identity of the original approver.

## Why this is a bug

The contract documents a refusal that creates a state no verb can leave: `docs/task-writing.md:519` names `task approve` as the only verb that advances a `todo` row and also refuses any row carrying an approval record, so a `todo` row with a record is unreachable by every documented path. Neither guard is wrong in isolation; the defect is that the pair has no reachable repair path, and `task work-on`'s refusal message directs the operator to the one verb that also refuses.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `task approve` advances an already-approved `todo` row, preserving the record; unit specs for the full-record, partial-record and no-record arms plus the unchanged refusals | 1, 2, 3, 4 | 1, 2, 3, 4, 5 | — |
| 2 | `APPROVED_RECORD_IN_TODO_INBOX` lint rule with its fix, reusing the phase-advance behaviour established in prompt 1 | 5, 6 | 6, 7 | prompt 1 |
| 3 | Add a new `## Unreleased` heading above `## v0.188.3` in `CHANGELOG.md` with one bullet; amend the `docs/task-writing.md` § Phase transitions sentence that reads *"The command refuses any row that is not at `phase: todo`, and any row that already carries `approved_by` or `approved_at`, writing nothing in either case"* so it states that a row already carrying the record is **advanced**, not re-approved | — | 8 | prompts 1, 2 |

Rationale: prompt 1 establishes the behaviour; prompt 2 builds the lint fix on it; prompt 3 records the contract change and the changelog entry once the code is settled.
