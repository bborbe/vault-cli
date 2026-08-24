---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-08-24T18:35:29Z"
generating: "2026-08-24T18:35:29Z"
prompted: "2026-08-24T18:56:37Z"
verifying: "2026-08-24T19:33:39Z"
completed: "2026-08-24T19:59:26Z"
branch: dark-factory/mandatory-abort-reason-and-gate-successor
---

## Summary

- Closing out a task or goal (`status: aborted` **or** `status: completed`) currently persists with no required explanation, so whatever the task owned — a risk trigger, gate, threshold, or recurring check — silently retires with it.
- A real July 2026 failure cost ~€412 of avoidable losses: a P1 task was aborted at `phase: todo` with no reason, and the ORB GBPJPY V6 rolling-20 `PF<0.8` risk trigger it owned fired unread for ~3 weeks.
- `aborted_reason` already exists as a vault convention but is used in only 3 of 113 aborted tasks (2.7%). This spec enforces the existing convention — it invents nothing.
- The close-out transition becomes impossible without two fields: a free-text `aborted_reason` **and** an explicit `gate_successor` — either the name of the task/alert/owner that inherits the risk, or the literal `none`.
- The CLI error must name the risk, not just the field: it asks what the task owns (trigger / gate / threshold / recurring check) and where the risk moves.

## Problem

A `status: aborted` with no recorded reason silently destroys the reasoning behind the decision and can silently retire whatever the task owned. In July 2026, task `Review TDR 2026-05-15 GBPJPY V6 at Q2 Gate` was `priority: 1`, aborted at `phase: todo` with no reason recorded anywhere. That task owned the ORB GBPJPY V6 rolling-20 `PF<0.8` risk trigger; the trigger fired, nobody read it, and V6 traded past its own stop for roughly three weeks — about €412 of avoidable losses. The same failure mode applies to `completed`: a completed task that owned a live gate retires it just as silently. `aborted_reason` already exists as a vault convention (3 of 113 aborted tasks use it), so the fix enforces an existing field rather than inventing one. A non-empty string alone is not enough either — a bare "no longer needed" passes validation while a live risk gate still dies; the disposition must be recorded explicitly.

## Goal

After this work, no code path in vault-cli can persist `status: aborted` or `status: completed` on a task or goal without an explicit close-out record: a non-empty free-text `aborted_reason` plus a `gate_successor` field whose value names where the risk moves, or records the literal `none`. When either is missing, the command fails with an actionable error that names the missing field(s), asks what the task owns (trigger / gate / threshold / recurring check) and where it moves, and shows the command form that succeeds. The transition is not persisted. Non-close-out transitions (`in_progress`, `next`, `backlog`, `hold`) keep working exactly as today, and no existing aborted/completed task is touched.

## Non-goals

- Do NOT backfill `aborted_reason` / `gate_successor` on the ~110 existing reason-less aborted tasks — historical, not worth the churn.
- Do NOT introduce a separate risk-trigger registry — requiring a reason at close-out time is the cheaper fix that generalises to all tasks, not just gate-owners.
- Do NOT add an `abort` subcommand — the existing routes (`task set`, `task complete`, `goal set`, `goal complete`) are the complete surface.
- Do NOT change any transition other than `aborted` / `completed` (e.g. `hold`, `defer` are untouched).
- Do NOT change vault-ui (separate repo, separate task).
- Do NOT add a bypass/opt-out (flag, env var, or config) that disables the reason guard — a close-out that skips the guard is the exact regression this spec exists to prevent. `task complete --force` bypasses only the incomplete-subtask check, never the reason guard.

## Acceptance Criteria

**Scenario coverage: none.** Unit tests at the domain guard plus integration tests through the ops layer reach this behavior with real task files; no Docker, cluster, or external service is involved. The four conditions for a scenario do not hold.

- [ ] `make precommit` exits 0 — evidence: exit code
- [ ] `make test` exits 0 AND a domain unit test asserts that `TaskFrontmatter.SetStatus(aborted)` without `aborted_reason` / `gate_successor` returns a validation error and leaves the frontmatter unchanged, and that the same call succeeds when both fields are present (mirror test for `GoalFrontmatter.SetStatus`) — evidence: exit code + test assertion
- [ ] Task aborted transition is gated: `vault-cli task set "<name>" status aborted` (no reason) exits non-zero, stderr contains `aborted_reason`, and `grep -n '^status:' "<taskfile>"` does not show `aborted`; with `--reason "<text>" --gate-successor none` the command exits 0 and the task file frontmatter contains `status: aborted`, `aborted_reason: <text>`, `gate_successor: none` — evidence: exit code + stderr match + file content
- [ ] Task completed transition is gated: `vault-cli task set "<name>" status completed` (no reason) exits non-zero and `vault-cli task complete "<name>"` (no reason) exits non-zero with stderr naming `aborted_reason`, and the task file `status` is not `completed`; `vault-cli task complete "<name>" --reason "<text>" --gate-successor "<successor>"` exits 0 and the task file frontmatter contains `status: completed`, `aborted_reason: <text>`, `gate_successor: <successor>` — evidence: exit code + stderr match + file content
- [ ] `vault-cli task update "<name>"` on a task with all checkboxes ticked and no `aborted_reason` exits non-zero and the task file `status` is not `completed` (no silent completion via checkbox sync) — evidence: exit code + file content (negative)
- [ ] Checkbox-sync completion still works when the fields are present: on a task with all checkboxes ticked AND `aborted_reason` + `gate_successor` already set, `vault-cli task update "<name>"` exits 0 and the task file frontmatter contains `status: completed` — evidence: exit code + file content
- [ ] Goal close-outs are gated the same way: `vault-cli goal set "<name>" status aborted` exits non-zero; `vault-cli goal set "<name>" status aborted --reason "<text>" --gate-successor none` and `vault-cli goal complete "<name>" --reason "<text>" --gate-successor none` exit 0 with `aborted_reason` and `gate_successor` present in the goal file frontmatter — evidence: exit code + file content
- [ ] Non-close-out transitions are unaffected: `vault-cli task set "<name>" status in_progress` and `status hold` exit 0 with no reason flags and persist the requested status — evidence: exit code + file content (regression)
- [ ] No backfill: exercising the new binary against a fixture vault that already contains a reason-less `status: aborted` task and a reason-less `status: completed` task leaves both files byte-identical — `git status --porcelain` shows no modifications — evidence: file diff (negative)

## Verification

Split into container-executable (runs inside the YOLO container at prompt time) and operator-executable (runs on the host after PR merge, spec verification ladder). Operator commands mirror the vault task's own end-to-end verification.

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
grep -rn "gate_successor" pkg/domain/*_test.go
grep -rn "vault-cli task set" pkg/cli/*.go
```

### Operator-executable (runs on the host after PR merge, spec verification ladder)

Build a fresh binary and drive a scratch vault (copy `example/` to a temp dir so the real vault is never touched; substitute the `__VAULT_PATH__` placeholder in the copied config with the temp vault path):

```
make build
cp -r example /tmp/vault-cli-e2e
sed -i '' "s|__VAULT_PATH__|/tmp/vault-cli-e2e/vault|" /tmp/vault-cli-e2e/config.yaml
# aborted without reason → rejected
./bin/vault-cli --config /tmp/vault-cli-e2e/config.yaml task set "Simple Task" status aborted ; echo "exit=$?"   # exit≠0, stderr names aborted_reason
grep -n '^status:' "/tmp/vault-cli-e2e/vault/24 Tasks/Simple Task.md"                                        # not aborted
# aborted with disposition → accepted
./bin/vault-cli --config /tmp/vault-cli-e2e/config.yaml task set "Simple Task" status aborted --reason "gate moved to X" --gate-successor none ; echo "exit=$?"   # exit=0
grep -n -e '^status: aborted' -e '^aborted_reason:' -e '^gate_successor:' "/tmp/vault-cli-e2e/vault/24 Tasks/Simple Task.md"
# completed without reason → rejected; with reason → accepted (same pattern for `task complete` and both goal commands)
```

Assert each `exit=$?` and each `grep` result against the Acceptance Criteria.

## Desired Behavior

1. **Domain guard (task).** The `SetStatus` chokepoint on the task frontmatter rejects a target status of `aborted` or `completed` unless the frontmatter at that moment holds a non-empty `aborted_reason` AND a non-empty `gate_successor` (a successor name or the literal `none`). The rejection is a validation error in the existing style and is raised before any write; the task file is not modified. Because the guard sits at the chokepoint, every route that sets the status goes through it: `task set <name> status aborted|completed`, `task complete`, and the `task update` checkbox sync.

2. **Domain guard (goal).** The same rule applies to the goal frontmatter's `SetStatus` chokepoint for target statuses `aborted` and `completed`, covering `goal set <name> status aborted|completed` and `goal complete`.

3. **Error message content.** A rejected close-out prints an error that names the missing field(s) (`aborted_reason`, `gate_successor`), lists the owned-risk categories the operator must consider (trigger / gate / threshold / recurring check), asks where the risk moves or to record `none`, and shows the command form that succeeds. The same message text appears in human output and in the `error` field of `--output json`.

4. **One-step close-out surface.** `task set <name> status aborted|completed`, `task complete`, `goal set <name> status aborted|completed`, and `goal complete` accept `--reason <text>` and `--gate-successor <name|none>` flags. When supplied, the command persists `aborted_reason` (and `gate_successor`) and then the status in one invocation. Setting `aborted_reason` / `gate_successor` directly via `task set <name> aborted_reason "<text>"` remains valid — the guard reads the fields at transition time, so a two-step close-out (set fields, then set status) still works.

5. **Checkbox-sync completion is gated.** When `task update` would set status to `completed` and the guard fails (fields absent), the operation aborts with the same actionable error and persists no status change — the task file's `status` stays as it was. No silent completion through the sync path.

6. **Invariants on the edges.** Transitions to `in_progress`, `next`, `backlog`, and `hold` never require the fields; `task complete --force` bypasses only the incomplete-subtask check, never the reason guard; pre-existing aborted/completed tasks are left byte-identical (no backfill, no migration); no opt-out disables the guard.

## Constraints

- Frozen field names: `aborted_reason` (free text) and `gate_successor` (successor name or literal `none`). The reason field is named `aborted_reason` for BOTH `aborted` and `completed` transitions — this is the existing vault convention and the boss decision recorded in the vault task; do NOT invent a separate `completion_reason` field. Convention documented in `docs/task-writing.md` § Lifecycle (Close-out fields) — the prompt should keep the two in sync.
- `gate_successor` is required at close-out time with `none` allowed as the explicit no-gate disposition. "Optional" in the design note means the VALUE may be `none` (no gate owned) — it does NOT mean the operator may skip the choice. A missing `gate_successor` is indistinguishable from "did not think about the gate", which is the exact silent-gate-death the control prevents.
- Validation style is frozen: errors raised via `errors.Wrapf(ctx, validation.Error, ...)`, matching the existing `TaskStatus.Validate` / `GoalStatus.Validate` pattern. Do not introduce a new error-raise mechanism.
- The `SetStatus` chokepoints are frozen enforcement points: `TaskFrontmatter.SetStatus` and `GoalFrontmatter.SetStatus`. Every route that can set the status must pass through them — no route may special-case around them.
- `task set <name> <key> <value>` remains a valid 3-argument command for every non-close-out key and for setting `aborted_reason` / `gate_successor` directly; only the close-out transitions gain the guard.
- Existing behavior for `in_progress`, `next`, `backlog`, `hold`, phases, and all non-status fields must not change.
- `aborted_reason` and `gate_successor` values are user-supplied strings persisted into YAML frontmatter. They must be stored as Go strings through the existing YAML serializer so special characters (newlines, colons) are quoted — never string-concatenated into the file text.
- Breaking change for existing automation: scripts that call `task complete` or `task set ... status completed|aborted` without the fields will now fail. This is intended; the CHANGELOG entry must call it out.

## Failure Modes

This change writes persistent frontmatter state and changes an existing CLI contract, so Detection, Reversibility, and Concurrency columns apply. External system unavailability and rate limiting do not apply (no network I/O). Clock skew does not apply (no new timestamp logic — `completed_date` behaviour is unchanged).

| Trigger | Expected behavior | Recovery | Detection | Reversibility | Concurrency |
|---------|-------------------|----------|-----------|---------------|-------------|
| Operator runs a close-out without `aborted_reason` / `gate_successor` (the intended gate) | Command exits non-zero with the actionable error; nothing is written | Operator supplies `--reason` and `--gate-successor <name\|none>` and re-runs | exit code + stderr | Reversible — no write occurred | n/a |
| Existing script calls `task complete` or `task set ... status completed|aborted` without the fields | Command now fails (intended breaking change); no data lost | Update the script to pass `--reason` / `--gate-successor`; CHANGELOG documents the break | exit code | Reversible — no write occurred | n/a |
| Crash mid-close-out after the task write but before goal/daily-note updates | Task persists as `completed` with the reason; some goal checkboxes may be stale (pre-existing multi-write behaviour, unchanged by this spec) | Operator re-runs `task complete` (task path is not blocked on already-completed; goal path reports already-completed and is left as-is) | task file + goal file diff | Partial | Two instances completing the same task race on the file write; both carry the reason, last write wins — no new race introduced by the guard |
| External writer (hand edit, obsidian-git sync, vault-ui) sets `aborted`/`completed` directly in the file, bypassing the CLI guard | The transition persists without the fields — the guard only covers vault-cli writes; vault-ui is a separate task/repo | Not addressed by this spec; the vault-ui task enforces the same rule on its own surface | task file shows `aborted`/`completed` without the fields | Irreversible from the CLI's perspective | n/a |
| `--reason` / `--gate-successor` text contains YAML-special characters | Serializer quotes the value correctly; file remains valid YAML (must be covered by a test) | If a malformed file ever appears, existing `task lint` surfaces it | `task lint` / file parse | Reversible | n/a |

## Security / Abuse Cases

This change accepts user-controlled input (`--reason`, `--gate-successor`) and writes it into vault frontmatter files, so input-validation rules apply.

- **Input into YAML:** both values must be persisted as Go strings through the YAML serializer (which quotes special characters). A raw string-concatenated value containing a newline could inject a second frontmatter key or break file parsing. The spec's constraint and tests pin this; no other file format is touched.
- **`gate_successor` is not cross-validated:** there is no task/alert registry (out of scope), so a typo'd successor name is not detected. The control's verifiability comes from the field being present and queryable, not from reference integrity — an accepted limitation.
- **No new trust boundary:** the change adds no HTTP, no network, no path resolution, and no new file types. The only files written are the task/goal markdown files vault-cli already owns.
- **Error output:** the error message is static CLI text (human and JSON); it embeds the command form, not user input, so it adds no injection surface.

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Domain guard at the `SetStatus` chokepoints (task + goal): reject `aborted`/`completed` without non-empty `aborted_reason` + `gate_successor`, in the existing validation style, plus domain unit tests | 1, 2 | 1, 2, 7 | — |
| 2 | Ops layer: `complete.go` / `goal_complete.go` propagate the `SetStatus` error instead of ignoring it (no partial write); `update.go` aborts without a status change when the guard fails; integration tests for each ops path | 4 (ops), 5 | 3, 4, 5 | prompt 1 |
| 3 | CLI surface: `--reason` / `--gate-successor` flags on `task set status aborted\|completed`, `task complete`, `goal set status aborted\|completed`, `goal complete`; the risk-naming error message (owned-risk categories + command form) in human and JSON output; CLI/integration tests | 3, 4 (CLI), 6 | 3, 4, 5, 6, 7, 8 | prompt 2 |

Rationale: prompt 1 establishes the hard invariant at the single domain chokepoint and can be verified by unit tests alone. Prompt 2 makes the ops paths honor it (the error today is swallowed with `_ = task.SetStatus(...)` — the silent hole). Prompt 3 builds the operator-facing surface — flags, error text, JSON parity — on top; it depends on prompt 2 because the CLI cannot surface an error the ops layer discards. The no-backfill AC (8) is verified at the operator rung after all three prompts land.

## Do-Nothing Option

Without this change, `aborted`/`completed` continue to persist with no reason and no disposition. The July failure — a P1 task aborted at `phase: todo` that owned a live risk trigger, ~€412 of avoidable losses across W27–W29 — repeats whenever a gate-owning task is closed out without thought. The cost of the change is small (an enforced existing field plus one structured field, at a single chokepoint) against a demonstrated, named loss. The control's weak half (`aborted_reason` free text) is already adopted in the vault; this spec makes the mechanical half actually enforce the behavioural half. Not doing it leaves a known, priced failure mode open.

## Verification Result

**Verified:** 2026-08-24T19:57:28Z (binary source 39bb36c; 037 commits b8f61f2/e4aa305/12a50a3 present)
**Binary:** /Users/bborbe/Documents/workspaces/vault-cli/bin/vault-cli — fresh `make build`, source 39bb36c
**Scenario:** none (spec declares "Scenario coverage: none"); ACs driven via operator rungs against fresh scratch vaults
**Evidence:**
- `make precommit` exit 0; `make test` exit 0; domain tests: SetStatus(aborted|completed) without fields → validation error, frontmatter unchanged; with both fields → success (task + goal mirror)
- aborted/completed reject without reason: exit 1, stderr "missing close-out field(s) aborted_reason, gate_successor", file `status` unchanged (task set/complete, goal set/complete, task update)
- close-outs accept with `--reason` + `--gate-successor`: exit 0, frontmatter gains `status`, `aborted_reason`, `gate_successor` (task + goal)
- non-close-out transitions (in_progress/hold/next/backlog): exit 0, persisted without reason flags
- no backfill: git-initialized fixture with reason-less aborted+completed tasks — `git diff HEAD` empty after exercising, only intentional write in porcelain
**Verdict:** PASS
