---
spec: ["004"]
status: created
created: "2026-03-17T00:00:00Z"
---

<summary>
- Goal and Objective domain structs gain a completion date field for recording when they were completed
- The narrow TaskStorage interface gains a ListTasks method so ops can scan all tasks without importing the full Storage interface
- The TaskStorage counterfeiter mock is regenerated to reflect the new method
- No behavioral change to existing commands — purely additive model layer changes
</summary>

<objective>
Extend the domain structs and storage interfaces needed by the goal/objective complete operations: add a `Completed` date field to `Goal` and `Objective`, and add `ListTasks` to the narrow `TaskStorage` interface so the goal-complete op can scan linked tasks.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/domain/goal.go` — `Goal` struct; add `Completed` field
- `pkg/domain/objective.go` — `Objective` struct; add `Completed` field
- `pkg/domain/task.go` — `Task.Goals []string` field — this is the source of truth for goal linkage
- `pkg/storage/storage.go` — `TaskStorage` interface; also shows how other fields are wired via `NewConfigFromVault`
- `pkg/storage/task.go` — `ListTasks` implementation already exists on `*taskStorage`; verify signature matches
- `mocks/task-storage.go` — existing counterfeiter mock to be regenerated
</context>

<requirements>

## 1. `pkg/domain/goal.go` — Add `Completed` date field

Add a `Completed` date pointer to the `Goal` struct using `libtime.Date`:

```go
import (
    libtime "github.com/bborbe/time"
    // keep existing time import if still needed, else remove
)

type Goal struct {
    // ... existing fields ...
    Completed  *libtime.Date `yaml:"completed,omitempty"`
}
```

Remove the bare `"time"` import if it is only used by `StartDate`/`TargetDate` as `*time.Time` — check whether those fields remain `*time.Time` and keep `"time"` if so.

## 2. `pkg/domain/objective.go` — Add `Completed` date field

Same as above for `Objective`:

```go
type Objective struct {
    // ... existing fields ...
    Completed  *libtime.Date `yaml:"completed,omitempty"`
}
```

## 3. `pkg/storage/storage.go` — Add `ListTasks` to `TaskStorage` interface

The method already exists on the concrete `taskStorage` struct in `pkg/storage/task.go`.
Add it to the narrow interface so ops can depend on it without pulling the full `Storage`:

```go
//counterfeiter:generate -o ../../mocks/task-storage.go --fake-name TaskStorage . TaskStorage
type TaskStorage interface {
    WriteTask(ctx context.Context, task *domain.Task) error
    FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)
    ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)
}
```

The existing `Storage` composite interface already includes `ListTasks` via the legacy section — do NOT add a duplicate there.

## 4. Regenerate mocks

Run counterfeiter to regenerate the `TaskStorage` mock with the new method:

```bash
go generate ./pkg/storage/...
```

Verify `mocks/task-storage.go` now has a `ListTasksStub` and `ListTasksArgsForCall` etc.

Also regenerate `mocks/storage.go` if the composite `Storage` interface mock needs updating (it embeds `TaskStorage`):

```bash
go generate ./pkg/storage/...
```

</requirements>

<constraints>
- Use `libtime.Date` (not `time.Time`) for the `Completed` field — consistent with `DeferDate`, `PlannedDate`, `DueDate` in `Task`
- `omitempty` tag required so nil dates are not serialized
- Do NOT change any op, CLI, or test behaviour — this prompt is model-layer only
- Existing tests must still pass after `ListTasks` is added to the interface
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
make test

# Confirm new field serializes correctly:
# Create a temp goal file with `completed: 2026-03-17` frontmatter and verify it parses without error
</verification>
