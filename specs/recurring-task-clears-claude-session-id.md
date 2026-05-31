---
tags:
  - dark-factory
  - spec
status: draft
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
- [ ] A unit test in `pkg/ops/complete_test.go`, in the existing "recurring daily task" context, asserts that the completed file's frontmatter does not contain the `claude_session_id` key after completion — evidence: `go test ./pkg/ops/ -run <NewTestName>` exits 0; the test fails if the field clearing is removed from `handleRecurringTask`.
- [ ] `make precommit` exits 0 — evidence: exit code.

## Verification

```
make precommit
go test ./pkg/ops/ -run TestComplete -v
```

## Do-Nothing Option

Leaving this bug in place means every recurring task that ever had a `workon` invocation will keep reporting that stale session ID forever (or until the next `workon` overwrites it). Any consumer that reads `claude_session_id` to attribute work to a session gets wrong data. The cost to fix is tiny (one method, one call, one test); the cost to leave is ongoing data corruption. Not acceptable.
