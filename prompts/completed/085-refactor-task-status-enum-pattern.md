---
status: completed
summary: Refactored TaskStatus to canonical Go enum pattern with String(), Validate(), Ptr() methods, AvailableTaskStatuses collection, and simplified IsValidTaskStatus and parseTaskStatus to use collection lookup.
container: vault-cli-085-refactor-task-status-enum-pattern
dark-factory-version: v0.57.5
created: "2026-03-19T10:17:42Z"
queued: "2026-03-19T10:32:07Z"
started: "2026-03-19T10:32:16Z"
completed: "2026-03-19T10:36:52Z"
---

<summary>
- Task status validation uses a single source-of-truth list instead of duplicated switch statements
- Status values become self-validating with built-in validation support
- A collection type enables type-safe filtering and membership checks
- The frontmatter status parser is simplified to use the shared validation
- All existing tests continue to pass with no behavior changes
</summary>

<objective>
Refactor the existing TaskStatus type in pkg/domain/task.go to follow the canonical Go enum type pattern used across bborbe projects (constants, Available* collection, String(), Validate(), plural type with Contains()). This is a pure refactoring — no behavior changes, no new statuses, no API changes.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `~/.claude-yolo/docs/go-enum-pattern.md` — the canonical Go enum type pattern to follow (naming, methods, collection type).
Read `pkg/domain/task.go` — find the `TaskStatus` type, constants, `IsValidTaskStatus`, `NormalizeTaskStatus`, and `UnmarshalYAML`.
Read `pkg/domain/task_status_test.go` — existing tests for status validation and normalization.
Read `pkg/ops/frontmatter.go` — find `parseTaskStatus` function (~line 172) which manually lists valid statuses.
Read `pkg/ops/lint.go` — find uses of `IsValidTaskStatus` and `NormalizeTaskStatus`.

Dependencies: `github.com/bborbe/collection` and `github.com/bborbe/validation` (both already in go.mod).
</context>

<requirements>
1. In `pkg/domain/task.go`, add a `String() string` method to `TaskStatus` that returns `string(s)`.

1a. Add a `Ptr() *TaskStatus` method on `TaskStatus` that returns a pointer to a copy (see go-enum-pattern.md).

2. In `pkg/domain/task.go`, add a `Validate(ctx context.Context) error` method to `TaskStatus` that checks `AvailableTaskStatuses.Contains(s)` and returns a validation error if false. Import `github.com/bborbe/validation` for `validation.Error`.

3. In `pkg/domain/task.go`, define the plural collection type:
```go
type TaskStatuses []TaskStatus
```

4. In `pkg/domain/task.go`, add a `Contains(status TaskStatus) bool` method on `TaskStatuses` using `collection.Contains`. Import `github.com/bborbe/collection`.

5. In `pkg/domain/task.go`, define `AvailableTaskStatuses`:
```go
var AvailableTaskStatuses = TaskStatuses{
    TaskStatusTodo,
    TaskStatusInProgress,
    TaskStatusBacklog,
    TaskStatusCompleted,
    TaskStatusHold,
    TaskStatusAborted,
}
```

6. Rewrite `IsValidTaskStatus` to delegate to `AvailableTaskStatuses.Contains(status)`.

7. In `pkg/ops/frontmatter.go`, simplify `parseTaskStatus` to use `domain.AvailableTaskStatuses.Contains()` instead of manually listing all statuses. Remove the local `validStatuses` slice.

8. In `pkg/domain/task_status_test.go`, add tests for the new `String()` and `Validate()` methods:
   - `String()` returns the string value
   - `Validate()` returns nil for valid statuses
   - `Validate()` returns error for invalid statuses

9. Run `go generate ./pkg/ops/...` to regenerate mocks if any interface changed (none should have).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass — this is a pure refactoring
- Keep `NormalizeTaskStatus` and `UnmarshalYAML` as-is (they depend on `IsValidTaskStatus` which now delegates)
- Keep the migration map in `NormalizeTaskStatus` unchanged
- All paths are repo-relative
</constraints>

<verification>
Run `make precommit` -- must pass.
</verification>
