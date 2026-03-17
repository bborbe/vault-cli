---
status: completed
spec: ["004"]
summary: Created GoalCompleteOperation and ObjectiveCompleteOperation with Ginkgo/Gomega test suites and counterfeiter mocks
container: vault-cli-075-spec-004-complete-ops
dark-factory-version: v0.57.5
created: "2026-03-17T00:00:00Z"
queued: "2026-03-17T10:44:30Z"
started: "2026-03-17T11:19:59Z"
completed: "2026-03-17T11:29:17Z"
branch: dark-factory/entity-complete-commands
---

<summary>
- A new GoalCompleteOperation marks a goal as completed with today's date
- Before completing, the operation scans all tasks and blocks if any task linked to this goal is still open (todo or in_progress)
- A --force flag can be passed through to bypass the open-task check
- Goals that are already completed return an error immediately
- A new ObjectiveCompleteOperation marks an objective as completed with today's date
- Objective complete has no linked-entity check (no --force needed)
- Both operations return a JSON result with entity name, new status, and completion date
- Counterfeiter mocks are generated for both operation interfaces
</summary>

<objective>
Create `GoalCompleteOperation` in `pkg/ops/goal_complete.go` and `ObjectiveCompleteOperation` in `pkg/ops/objective_complete.go`, including full Ginkgo/Gomega test suites and counterfeiter mocks. These operations implement the complete-entity semantics defined in spec 004.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/ops/complete.go` — existing task complete op; follow its patterns for JSON output (`MutationResult`), error wrapping, and libtime injection
- `pkg/domain/goal.go` — `Goal` struct with `Status GoalStatus`, `Completed *libtime.Date`; `GoalStatusCompleted` constant
- `pkg/domain/objective.go` — `Objective` struct with `Status ObjectiveStatus`, `Completed *libtime.Date`; `ObjectiveStatusCompleted` constant
- `pkg/domain/task.go` — `Task.Goals []string`, `TaskStatus` constants (`TaskStatusTodo`, `TaskStatusInProgress`, `TaskStatusCompleted`, `TaskStatusAborted`, `TaskStatusHold`)
- `pkg/storage/storage.go` — `GoalStorage`, `ObjectiveStorage`, `TaskStorage` (now includes `ListTasks`)
- `pkg/ops/vault_dispatcher.go` — `VaultDispatcher` for reference
- `mocks/goal-storage.go`, `mocks/objective-storage.go`, `mocks/task-storage.go` — counterfeiter mocks to use in tests
</context>

<requirements>

## 1. `pkg/ops/goal_complete.go` — GoalCompleteOperation

Create a new file with the following structure:

**Interface and constructor:**

```go
//counterfeiter:generate -o ../../mocks/goal-complete-operation.go --fake-name GoalCompleteOperation . GoalCompleteOperation
type GoalCompleteOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        goalName string,
        vaultName string,
        outputFormat string,
        force bool,
    ) error
}

func NewGoalCompleteOperation(
    goalStorage storage.GoalStorage,
    taskStorage storage.TaskStorage,
    currentDateTime libtime.CurrentDateTime,
) GoalCompleteOperation {
    return &goalCompleteOperation{
        goalStorage:     goalStorage,
        taskStorage:     taskStorage,
        currentDateTime: currentDateTime,
    }
}
```

**Execute logic (in order):**

1. Call `goalStorage.FindGoalByName(ctx, vaultPath, goalName)`. On error, output JSON error if `outputFormat == "json"` and return wrapped error.

2. If `goal.Status == domain.GoalStatusCompleted`, return error: `fmt.Errorf("goal %q is already completed", goalName)`. In JSON mode output `MutationResult{Success: false, Error: ...}` first.

3. If `!force`: call `taskStorage.ListTasks(ctx, vaultPath)`. Filter for tasks where `task.Goals` slice contains `goalName` (case-sensitive exact match). From that filtered set, collect tasks where `task.Status == domain.TaskStatusTodo || task.Status == domain.TaskStatusInProgress`. If any open tasks found, build error message: `fmt.Sprintf("cannot complete goal: %d task(s) still open: %s", count, joinTaskNames(openTasks))` where `joinTaskNames` joins `task.Name` with `", "`. In JSON mode output `MutationResult{Success: false, Error: ...}` and return.

4. Set `goal.Status = domain.GoalStatusCompleted`. Set `goal.Completed = libtime.ToDate(currentDateTime.Now().Time()).Ptr()` (using `libtime.ToDate(...).Ptr()` per project patterns).

5. Call `goalStorage.WriteGoal(ctx, goal)`. On error, output JSON error and return.

6. Output result. In JSON mode:
```go
type GoalCompleteResult struct {
    Success   bool   `json:"success"`
    Name      string `json:"name,omitempty"`
    Status    string `json:"status,omitempty"`
    Completed string `json:"completed,omitempty"`
    Vault     string `json:"vault,omitempty"`
    Error     string `json:"error,omitempty"`
}
```
Populate `Completed` as `goal.Completed.Format("2006-01-02")`.
In plain mode: `fmt.Printf("✅ Goal completed: %s\n", goal.Name)`.

**Helper:**

```go
func joinTaskNames(tasks []*domain.Task) string {
    names := make([]string, len(tasks))
    for i, t := range tasks {
        names[i] = fmt.Sprintf("%s (%s)", t.Name, t.Status)
    }
    return strings.Join(names, ", ")
}
```

## 2. `pkg/ops/goal_complete_test.go` — Tests

Create a Ginkgo test file in package `ops_test`. Use counterfeiter mocks from `mocks/` package.

Test cases (use `Describe`/`It`/`BeforeEach` pattern from other test files):

- **Goal not found** — `FindGoalByName` returns error → Execute returns error
- **Goal already completed** — `goal.Status = GoalStatusCompleted` → returns "already completed" error
- **Open tasks block completion** — goal found, `ListTasks` returns tasks with goal in Goals, one is `todo` → returns "cannot complete goal" error listing the task name
- **In-progress task blocks completion** — same as above with `in_progress` status
- **Completed/aborted/hold tasks do NOT block** — `ListTasks` returns tasks with statuses `completed`, `aborted`, `hold` → Execute succeeds
- **Zero linked tasks** — `ListTasks` returns tasks but none have this goal → Execute succeeds
- **force=true bypasses check** — open tasks present but `force=true` → Execute succeeds
- **Success plain mode** — sets status=completed, writes goal, no error
- **Success JSON mode** — output contains `"success":true`, `"status":"completed"`, `"completed":"<date>"`
- **WriteGoal error** — write fails → returns error

## 3. `pkg/ops/objective_complete.go` — ObjectiveCompleteOperation

Create a new file following the same structure as goal_complete.go but simpler (no task check, no force):

```go
//counterfeiter:generate -o ../../mocks/objective-complete-operation.go --fake-name ObjectiveCompleteOperation . ObjectiveCompleteOperation
type ObjectiveCompleteOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        objectiveName string,
        vaultName string,
        outputFormat string,
    ) error
}

func NewObjectiveCompleteOperation(
    objectiveStorage storage.ObjectiveStorage,
    currentDateTime libtime.CurrentDateTime,
) ObjectiveCompleteOperation {
    return &objectiveCompleteOperation{
        objectiveStorage: objectiveStorage,
        currentDateTime:  currentDateTime,
    }
}
```

**Execute logic:**

1. Find objective via `objectiveStorage.FindObjectiveByName`. On error → JSON + return.
2. If `objective.Status == domain.ObjectiveStatusCompleted` → error "objective %q is already completed". JSON + return.
3. Set `objective.Status = domain.ObjectiveStatusCompleted`. Set `objective.Completed = libtime.ToDate(currentDateTime.Now().Time()).Ptr()`.
4. Write via `objectiveStorage.WriteObjective`. On error → JSON + return.
5. JSON mode: `GoalCompleteResult`-style struct with name/status/completed/vault. Plain: `fmt.Printf("✅ Objective completed: %s\n", ...)`.

## 4. `pkg/ops/objective_complete_test.go` — Tests

Same pattern as goal complete tests but without force/task-check cases:

- Objective not found → error
- Already completed → error
- Success plain mode
- Success JSON mode
- WriteObjective error

## 5. Generate mocks

```bash
go generate ./pkg/ops/...
```

Verify `mocks/goal-complete-operation.go` and `mocks/objective-complete-operation.go` are created.

</requirements>

<constraints>
- Use `github.com/bborbe/errors` for all error wrapping — NEVER `fmt.Errorf` for wrapped errors
- Use `libtime.CurrentDateTime` injection for today's date — NEVER `time.Now()` directly
- Use `libtime.ToDate(...).Ptr()` to get a `*libtime.Date` pointer
- Task linkage check: tasks link to goals via `Task.Goals []string` — check for exact match of `goalName` in the slice
- Blocking statuses: `TaskStatusTodo` and `TaskStatusInProgress` block completion
- Non-blocking: `TaskStatusCompleted`, `TaskStatusAborted`, `TaskStatusHold`, `TaskStatusBacklog`
- A goal with zero linked tasks (none have it in Goals) is completable
- `force=true` skips the open-task check only — the entity must still exist
- Existing `task complete` behavior must not change — do not edit `pkg/ops/complete.go`
- Do NOT commit — dark-factory handles git
- Tests must use external test package (`package ops_test`)
- Tests must use counterfeiter mocks, not manual mocks
- Coverage ≥80% for new files
</constraints>

<verification>
make test

# Confirm goal complete blocks on open tasks:
# (tests cover this — verify test names in output)

# Confirm JSON output shape:
# grep for GoalCompleteResult in test output or run:
# go test -v ./pkg/ops/... -run GoalComplete
</verification>
