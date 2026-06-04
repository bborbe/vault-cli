---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-05-31T11:46:15Z"
generating: "2026-05-31T11:46:53Z"
prompted: "2026-05-31T11:49:02Z"
completed: "2026-06-04T14:45:45Z"
branch: dark-factory/recurring-task-clears-claude-session-id
---

## Summary

- Recurring task completion currently leaves the previous run's `claude_session_id` in the file, so every following iteration inherits a stale session ID.
- The next iteration of a recurring task should start with no Claude session attached — it's a fresh run.
- Fix scope: when a recurring task is completed and rolled to its next occurrence, the `claude_session_id` frontmatter field must be removed (not blanked).
- Non-recurring task completion stays untouched — the session ID there is part of the historical record.
- Small bug fix: one new clearer method, one call site, one unit test.

## Problem

When `vault-cli task complete` runs on a recurring task, the task file is rewritten in place for the next occurrence — checkboxes are reset, the phase is cleared, `last_completed_date` and `defer_date` are advanced. But the `claude_session_id` field set by `vault-cli task workon` during the previous run is left in place. Each subsequent iteration of the recurring task inherits that stale ID until another `workon` happens to overwrite it. A real recurring task in the vault has carried the same session ID for ~5 weeks of daily completions, which corrupts any tooling that trusts `claude_session_id` to identify the session currently working on the task.

## Goal

After completing a recurring task, the resulting file has no `claude_session_id` frontmatter key. The next time `workon` runs on that task, it sets a fresh ID; until then, the field is absent.

## Non-goals

- Do NOT backfill or clean up `claude_session_id` on already-completed recurring task files in the vault — out of scope for this code change.
- Do NOT modify non-recurring task completion behavior in any way.
- Do NOT clear any other frontmatter field as part of this change — `claude_session_id` only.
- Do NOT change `SetClaudeSessionID` semantics (empty string handling stays as-is) — invariant; if a future caller needs delete-on-empty, that's a separate spec.

## Desired Behavior

1. When `vault-cli task complete` is invoked on a recurring task, the rewritten file contains no `claude_session_id` key in its frontmatter, regardless of whether the field was present beforehand.
2. When `vault-cli task complete` is invoked on a non-recurring task, the `claude_session_id` field is preserved exactly as it was.
3. The clearing operation removes the key entirely; it does not leave `claude_session_id: ""` or `claude_session_id: null` in the file.

## Assumptions

- `Frontmatter` (or its underlying map) already exposes a way to delete a key — either `Frontmatter.Delete(key)` (confirmed: `pkg/domain/frontmatter_map.go:122`) or a setter that maps to it. If a new `ClearClaudeSessionID()` method is added on `TaskFrontmatter`, it routes through that existing delete primitive — no new storage layer required.
- Delete-of-absent-key is a no-op in the underlying map and cannot fail (`pkg/domain/frontmatter_map.go:122` — plain `delete(map, key)`).

## Constraints

- The fix must live inside the existing recurring-task completion path in `pkg/ops/complete.go` (`handleRecurringTask`). No restructuring of the completion flow.
- `SetClaudeSessionID(v string)` keeps its current behavior — empty string still stores `""`. The delete must go through a separate, explicit path (a new `ClearClaudeSessionID` method or a direct `Frontmatter.Delete` call).
- All existing tests in `pkg/ops/complete_test.go` continue to pass.
- See `docs/development-patterns.md` for repo conventions.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Recurring task has no `claude_session_id` field to begin with | Completion proceeds normally; file has no `claude_session_id` after | None — no-op |
| Frontmatter map's `Delete` of an absent key | No error; field stays absent | None |
| Task is recurring but completion fails for an unrelated reason (e.g. file write error) | Error surfaces as before; no partial frontmatter change persists | Re-run after fixing root cause |

## Acceptance Criteria

- [ ] After `vault-cli task complete` runs on a recurring task whose frontmatter contained `claude_session_id: <uuid>`, reading the file back shows no `claude_session_id` key — evidence: `grep -c '^claude_session_id:' <task-file>` returns `0`.
- [ ] After `vault-cli task complete` runs on a non-recurring task whose frontmatter contained `claude_session_id: <uuid>`, the same value is still present — evidence: `grep '^claude_session_id:' <task-file>` returns the original line unchanged.
- [ ] Given a recurring task whose frontmatter contains `claude_session_id`, when `handleRecurringTask` completes the file, the resulting frontmatter does not contain that key — evidence: a unit test in `pkg/ops/complete_test.go` (under the existing "recurring daily task" context) exits 0 with `go test ./pkg/ops/ -run <NewTestName>`, AND mutation-test check: removing the clearing call from `handleRecurringTask` makes that test fail.
- [ ] `make precommit` exits 0 — evidence: exit code.

## Verification

```
make precommit
go test ./pkg/ops/ -run TestComplete -v
```

## Do-Nothing Option

Leaving this bug in place means every recurring task that ever had a `workon` invocation will keep reporting that stale session ID forever (or until the next `workon` overwrites it). Any consumer that reads `claude_session_id` to attribute work to a session gets wrong data. The cost to fix is tiny (one method, one call, one test); the cost to leave is ongoing data corruption. Not acceptable.

## Verification Result

**Verified:** 2026-06-04T14:42:40Z (HEAD f7ee1c6)
**Binary:** /tmp/vault-cli-015 (built from HEAD, `vault-cli version dev`)
**Scenario:** Built CLI from master, created isolated test vault with two tasks each carrying `claude_session_id`, ran `vault-cli task complete` on both, inspected resulting frontmatter; ran focused Ginkgo test + mutation check; ran `make precommit`.
**Evidence:**
- AC1: `grep -c '^claude_session_id:' "$VAULT/24 Tasks/Recurring Test Task.md"` → `0`; rewritten frontmatter shows no key (not `""`, not `null`).
- AC2: `grep '^claude_session_id:' "$VAULT/24 Tasks/NonRecurring Test Task.md"` → `claude_session_id: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee` (unchanged).
- AC3: `go test ./pkg/ops/ -ginkgo.focus="clears claude_session_id"` → `1 Passed | 0 Failed`. Mutation: commenting out `task.ClearClaudeSessionID()` at `pkg/ops/complete.go:200` → `1 Failed` (call is load-bearing).
- AC4: `make precommit` → `ready to commit`, exit 0.
**Verdict:** PASS
