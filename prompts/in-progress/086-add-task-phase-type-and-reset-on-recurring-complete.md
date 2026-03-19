---
status: approved
created: "2026-03-19T10:17:42Z"
queued: "2026-03-19T10:32:10Z"
---

<summary>
- Phase becomes a validated type with six defined values: todo, planning, in_progress, ai_review, human_review, done
- Setting an invalid phase value now returns an error instead of silently accepting any string
- Phase field is optional — tasks without a phase continue to work unchanged
- Completing a recurring task clears the phase so each cycle starts fresh
- List and show commands continue displaying phase as a plain string in output
</summary>

<objective>
Introduce a strongly-typed TaskPhase enum following the Go enum type pattern, replace the free-form string Phase field in Task with *TaskPhase, and clear phase when completing a recurring task. This gives phase the same type safety as TaskStatus and ensures recurring tasks start each cycle with a clean slate.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `~/.claude-yolo/docs/go-enum-pattern.md` — the canonical Go enum type pattern to follow (naming, methods, collection type).
Read `pkg/domain/task.go` — find the `Task` struct with `Phase string` field. Note how `TaskStatus` is implemented (it follows the enum pattern with `AvailableTaskStatuses`, `String()`, `Validate()`, `Ptr()`, `Contains()`).
Read `pkg/ops/complete.go` — find `handleRecurringTask` method which resets checkboxes, bumps defer_date, but currently does NOT touch phase.
Read `pkg/ops/frontmatter.go` — find the phase cases in get/set/clear operations.
Read `pkg/ops/list.go` — find `TaskListItem` struct with `Phase string` field, and where `task.Phase` is assigned.
Read `pkg/ops/show.go` — find `TaskDetail` struct with `Phase string` field, and where `task.Phase` is used.
Read `pkg/ops/complete_test.go` — find recurring task tests to understand existing patterns.
Read `pkg/ops/list_test.go` — find tests that set `task.Phase`.
Read `pkg/ops/show_test.go` — find tests that reference `task.Phase`.
Read `pkg/ops/frontmatter_test.go` — find phase-related test contexts.

Dependencies: `github.com/bborbe/collection` and `github.com/bborbe/validation` (both already in go.mod).
</context>

<requirements>
1. Create `pkg/domain/task_phase.go` with the TaskPhase enum type following the canonical pattern:
```go
const (
    TaskPhaseTodo        TaskPhase = "todo"
    TaskPhasePlanning    TaskPhase = "planning"
    TaskPhaseInProgress  TaskPhase = "in_progress"
    TaskPhaseAIReview    TaskPhase = "ai_review"
    TaskPhaseHumanReview TaskPhase = "human_review"
    TaskPhaseDone        TaskPhase = "done"
)

var AvailableTaskPhases = TaskPhases{
    TaskPhaseTodo,
    TaskPhasePlanning,
    TaskPhaseInProgress,
    TaskPhaseAIReview,
    TaskPhaseHumanReview,
    TaskPhaseDone,
}

type TaskPhase string

func (t TaskPhase) String() string { return string(t) }

func (t TaskPhase) Validate(ctx context.Context) error {
    if !AvailableTaskPhases.Contains(t) {
        return errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", t)
    }
    return nil
}

type TaskPhases []TaskPhase

func (t TaskPhases) Contains(phase TaskPhase) bool {
    return collection.Contains(t, phase)
}
```

2. Add a `Ptr() *TaskPhase` method on `TaskPhase` that returns a pointer to a copy (same pattern as `DateOrDateTime.Ptr()`).

3. In `pkg/domain/task.go`, change the Task struct field from:
```go
Phase string `yaml:"phase,omitempty"`
```
to:
```go
Phase *TaskPhase `yaml:"phase,omitempty"`
```

4. In `pkg/ops/complete.go` `handleRecurringTask` method, after resetting checkboxes (step 1) and before writing the task, clear the phase:
```go
// Clear phase so next cycle starts fresh
task.Phase = nil
```

5. In `pkg/ops/frontmatter.go` frontmatter **get** operation, update the phase case:
```go
case "phase":
    if task.Phase != nil {
        return task.Phase.String(), nil
    }
    return "", nil
```

6. In `pkg/ops/frontmatter.go` frontmatter **set** operation, update the phase case to validate:
```go
case "phase":
    phase := domain.TaskPhase(value)
    if err := phase.Validate(ctx); err != nil {
        return err
    }
    task.Phase = phase.Ptr()
```

7. In `pkg/ops/frontmatter.go` frontmatter **clear** operation, update the phase case:
```go
case "phase":
    task.Phase = nil
```

8. In `pkg/ops/list.go`, update the `TaskListItem` struct — keep `Phase string` for JSON output. Update the assignment where `task.Phase` is mapped:
```go
Phase: func() string {
    if task.Phase != nil {
        return task.Phase.String()
    }
    return ""
}(),
```
Or use a helper. The JSON output field remains `string` — consumers should not break.

9. In `pkg/ops/show.go`, apply the same pointer-dereference pattern for the `TaskDetail` struct and the plain-text output.

10. Create `pkg/domain/task_phase_test.go` with Ginkgo tests:
    - Test all 6 phase constants are valid via `Validate()`
    - Test invalid phase returns error
    - Test `String()` returns expected value
    - Test `Ptr()` returns non-nil pointer with correct value
    - Test `AvailableTaskPhases.Contains()` for valid and invalid phases
    - Test YAML marshal/unmarshal with `*TaskPhase` (nil omitted, value round-trips)

11. In `pkg/ops/complete_test.go`, add a test in the recurring task context:
    - Task with `Phase: domain.TaskPhasePlanning.Ptr()` and `Recurring: "weekly"`
    - After complete, verify `writtenTask.Phase` is nil (cleared)
    - Also test: task with `Phase: nil` and `Recurring: "daily"` — verify phase remains nil after complete

12. Update all existing tests that set `task.Phase = "something"` to use `domain.TaskPhase("something").Ptr()` or the appropriate constant pointer. Check these files:
    - `pkg/ops/list_test.go` — Phase field assignments
    - `pkg/ops/show_test.go` — `task.Phase = ""`
    - `pkg/ops/frontmatter_test.go` — phase get/set/clear tests

13. Update `pkg/ops/frontmatter_test.go` phase set tests: add a test that setting an invalid phase value returns an error.

14. Run `go generate ./pkg/ops/...` to regenerate mocks if needed.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass after updates
- JSON output for list and show must remain `string` type for phase (not break consumers)
- Phase is optional — nil means "no phase set", not "todo"
- Only reset phase on recurring task completion, NOT on defer
- All paths are repo-relative
- This prompt depends on the refactor-task-status-enum-pattern prompt being completed first (TaskStatus follows enum pattern)
</constraints>

<verification>
Run `make precommit` -- must pass.
</verification>
