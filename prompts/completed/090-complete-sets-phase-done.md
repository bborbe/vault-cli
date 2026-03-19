---
status: completed
summary: Set task.Phase to domain.TaskPhaseDone.Ptr() on non-recurring task completion and added test asserting both status and phase are correct after complete
container: vault-cli-090-complete-sets-phase-done
dark-factory-version: v0.57.5
created: "2026-03-19T19:40:27Z"
queued: "2026-03-19T19:40:27Z"
started: "2026-03-19T19:40:34Z"
completed: "2026-03-19T19:45:51Z"
---

<summary>
- Completing a task leaves its phase stuck in whatever state it was in (e.g. review)
- After the fix, completing a non-recurring task also transitions phase to done
- Recurring tasks continue to clear phase on completion (no change)
- Test added to verify both status and phase are set correctly after completion
- All existing tests must still pass
</summary>

<objective>
`vault-cli task complete` should set `phase: done` in addition to `status: completed` when marking a non-recurring task as complete.

Currently `task.Phase` is not updated, so tasks end up with `status: completed` but `phase: human_review` (or whatever phase they were in), which creates inconsistent state visible on the TaskOrchestrator board.
</objective>

<context>
Go CLI project for managing Obsidian vault tasks.
Read CLAUDE.md for project conventions.

Key file: `./pkg/ops/complete.go`

The fix location is in the `Execute` method around line 112:

```go
// Update task status to completed
task.Status = domain.TaskStatusCompleted
task.CompletedDate = c.currentDateTime.Now().Time().UTC().Format("2006-01-02T15:04:05Z")
```

The domain constant exists: `domain.TaskPhaseDone` in `./pkg/domain/task_phase.go`.

Note: `task.Phase` is a `*TaskPhase` (pointer). Set it the same way as other phase assignments in the codebase.

Do NOT change the recurring task path (`handleRecurringTask`) — recurring tasks intentionally clear phase with `task.Phase = nil`.
</context>

<requirements>
1. In `Execute()` in `./pkg/ops/complete.go`, after setting `task.Status`, also set `task.Phase` to `domain.TaskPhaseDone` using the pointer pattern: `task.Phase = domain.TaskPhaseDone.Ptr()`
2. Add/update test in `./pkg/ops/complete_test.go` to assert that after `task complete`, the task has both `status: completed` and `phase: done`
3. Ensure recurring task path is NOT affected (keep `task.Phase = nil` in `handleRecurringTask`)
4. Files to modify: `./pkg/ops/complete.go`, `./pkg/ops/complete_test.go`
</requirements>

<constraints>
- Do NOT modify `handleRecurringTask` — recurring tasks must keep `task.Phase = nil`
- Use `domain.TaskPhaseDone.Ptr()` for pointer assignment (consistent with codebase pattern)
- Do NOT commit — leave changes staged for review
- Follow project conventions in CLAUDE.md
</constraints>

<verification>
```
make test
```

Confirm:
- `make test` passes
- A task with `phase: human_review` that is completed ends up with `phase: done` and `status: completed`
- A recurring task completed still clears phase (`phase` field absent)
</verification>
