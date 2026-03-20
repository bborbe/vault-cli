---
status: created
spec: ["005"]
created: "2026-03-20T00:00:00Z"
---

<summary>
- Seven mutation operations stop writing to stdout and return structured results instead
- All seven operations return a structured result with success/error/warning fields
- The output format parameter is removed from all seven operation interfaces
- Subtask-blocking and recurring-task result types are consolidated into one result type
- The CLI layer receives results and formats plain or JSON output
- All mocks are regenerated to match the new interfaces
- All existing tests pass with assertions updated from stdout capture to direct result checks
</summary>

<objective>
Refactor the seven mutation operations in `pkg/ops/` so they return `(MutationResult, error)` and never write to stdout. The CLI layer receives the result and formats output. This is the second of three prompts for spec 005. Prompt 1 must be completed first (query ops).
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/ops/complete.go` — CompleteOperation, MutationResult, IncompleteResult types (lines 1–170)
- `pkg/ops/defer.go` — DeferOperation interface + returnError/formatResult helpers
- `pkg/ops/workon.go` — WorkOnOperation interface (full file, ~240 lines)
- `pkg/ops/update.go` — UpdateOperation interface (full file, ~250 lines)
- `pkg/ops/decision_ack.go` — DecisionAckOperation interface
- `pkg/ops/goal_complete.go` — GoalCompleteOperation interface (full file, ~180 lines)
- `pkg/ops/objective_complete.go` — ObjectiveCompleteOperation interface
- `pkg/cli/cli.go` — call sites:
  - createCompleteCommand (~line 101): `completeOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)`
  - createDeferCommand (~line 139): `deferOp.Execute(ctx, vault.Path, taskName, dateStr, vault.Name, *outputFormat)`
  - createUpdateCommand (~line 192): `updateOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)`
  - createWorkOnCommand (~line 221): `workOnOp.Execute(ctx, ..., *outputFormat, isInteractive, sessionDir)`
  - createDecisionAckCommand (~line 1306): `ackOp.Execute(ctx, vault.Path, vault.Name, decisionName, statusOverride, *outputFormat)`
  - createGoalCompleteCommand (~line 960): `completeOp.Execute(ctx, vault.Path, goalName, vault.Name, *outputFormat, force)`
  - createObjectiveCompleteCommand (~line 1165): `completeOp.Execute(ctx, vault.Path, objectiveName, vault.Name, *outputFormat)`
- `pkg/cli/output.go` — PrintJSON helper
- `mocks/` — counterfeiter-generated mocks to regenerate
</context>

<requirements>
### 1. Extend MutationResult in `pkg/ops/complete.go`

The existing `MutationResult` type needs extra fields to absorb the `IncompleteResult` case:

```go
// MutationResult represents the result of a mutation operation.
type MutationResult struct {
    Success    bool     `json:"success"`
    Name       string   `json:"name,omitempty"`
    Vault      string   `json:"vault,omitempty"`
    Error      string   `json:"error,omitempty"`
    Warnings   []string `json:"warnings,omitempty"`
    SessionID  string   `json:"session_id,omitempty"`
    // Subtask blocking fields (used when a task cannot be completed due to incomplete subtasks)
    Reason     string   `json:"reason,omitempty"`
    Pending    int      `json:"pending,omitempty"`
    InProgress int      `json:"inprogress,omitempty"`
    Completed  int      `json:"completed,omitempty"`
    Total      int      `json:"total,omitempty"`
}
```

Remove the `IncompleteResult` type entirely — its fields are now part of `MutationResult`.
Remove the `RecurringMutationResult` type — `handleRecurringTask` should return `MutationResult` instead. The `NextDate` field from `RecurringMutationResult` can be encoded in the `Message` field.

### 2. `pkg/ops/complete.go` — CompleteOperation

Change the interface:
```go
type CompleteOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        taskName string,
        vaultName string,
    ) (MutationResult, error)
}
```

In `completeOperation.Execute`:
- Remove `outputFormat string` parameter throughout (including helpers `checkSubtaskCompletion`, `handleRecurringTask`)
- On error from `FindTaskByName`: return `(MutationResult{Success: false, Error: err.Error()}, wrappedErr)`
- `checkSubtaskCompletion` should return `(MutationResult, bool, error)`:
  - `bool` = shouldBlock; if true, return the MutationResult with Reason/Pending/etc fields set and a non-nil error (e.g. `errors.Errorf(ctx, "incomplete subtasks: %d pending", pending)`)
- `handleRecurringTask` returns `(MutationResult, error)`
- On success: return `(MutationResult{Success: true, Name: task.Name, Vault: vaultName, Warnings: warnings}, nil)`
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports (`encoding/json`, `fmt`, `os`)

### 3. `pkg/ops/defer.go` — DeferOperation

Change the interface:
```go
type DeferOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        taskName string,
        dateStr string,
        vaultName string,
    ) (MutationResult, error)
}
```

In `deferOperation.Execute`:
- Remove `outputFormat string` parameter
- Remove `returnError` helper method (it wrote JSON to stdout) — replace calls to `returnError(ctx, err, msg, outputFormat)` with direct `return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, msg)`
- Remove `formatResult` helper method — replace with direct `return MutationResult{Success: true, Name: name, Vault: vaultName, Warnings: warnings}, nil`
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports

### 4. `pkg/ops/workon.go` — WorkOnOperation

Change the interface:
```go
type WorkOnOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        taskName string,
        assignee string,
        vaultName string,
        isInteractive bool,
        sessionDir string,
    ) (MutationResult, error)
}
```

In `workOnOperation.Execute`:
- Remove `outputFormat string` parameter
- On success, the SessionID field of MutationResult should be populated from the sessionID variable
- Plain-text equivalent in CLI: `"✅ Now working on: %s (assigned to %s)\n"` + optional `"session_id: %s\n"` line
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports

### 5. `pkg/ops/update.go` — UpdateOperation

Change the interface:
```go
type UpdateOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        taskName string,
        vaultName string,
    ) (MutationResult, error)
}
```

In `updateOperation.Execute`:
- Remove `outputFormat string` parameter and all helper methods that accepted it (`outputErrorJSON`, `handleNoCheckboxes`)
- For the "no checkboxes" case: return `(MutationResult{Success: true, Name: taskName, Vault: vaultName, Warnings: []string{warning + ": " + taskName}}, nil)` — it's not an error
- On success: return `(MutationResult{Success: true, Name: taskName, Vault: vaultName, Warnings: warnings}, nil)` where the plain-text equivalent is `"✅ Updated %s/%s: %d/%d checkboxes complete\n"`
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports

### 6. `pkg/ops/decision_ack.go` — DecisionAckOperation

Change the interface:
```go
type DecisionAckOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        vaultName string,
        decisionName string,
        statusOverride string,
    ) (MutationResult, error)
}
```

In `decisionAckOperation.Execute`:
- Remove `outputFormat string` parameter
- On success: return `(MutationResult{Success: true, Name: decision.Name, Vault: vaultName}, nil)`
- Plain-text equivalent in CLI: `"Acknowledged: %s\n"`
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports

### 7. `pkg/ops/goal_complete.go` — GoalCompleteOperation

Change the interface:
```go
type GoalCompleteOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        goalName string,
        vaultName string,
        force bool,
    ) (MutationResult, error)
}
```

In `goalCompleteOperation.Execute`:
- Remove `outputFormat string` parameter throughout (including `checkOpenTasks`)
- Remove `outputGoalCompleteError` helper function
- On success: return `(MutationResult{Success: true, Name: goal.Name, Vault: vaultName}, nil)`
- Plain-text equivalent in CLI: `"✅ Goal completed: %s\n"`
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports

### 8. `pkg/ops/objective_complete.go` — ObjectiveCompleteOperation

Change the interface:
```go
type ObjectiveCompleteOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        objectiveName string,
        vaultName string,
    ) (MutationResult, error)
}
```

Remove the `ObjectiveCompleteResult` type — use `MutationResult` instead.

In `objectiveCompleteOperation.Execute`:
- Remove `outputFormat string` parameter throughout
- Remove `outputObjectiveCompleteError` helper function
- Replace all `ObjectiveCompleteResult` usage with `MutationResult`
- On success: return `(MutationResult{Success: true, Name: objective.Name, Vault: vaultName}, nil)`
- Plain-text equivalent in CLI: `"✅ Objective completed: %s\n"`
- Remove ALL `json.NewEncoder(os.Stdout)`, `fmt.Printf` output calls
- Remove unused imports

### 9. Regenerate mocks

Run:
```
go generate ./pkg/ops/...
```
This regenerates:
- `mocks/complete-operation.go`
- `mocks/defer-operation.go`
- `mocks/workon-operation.go`
- `mocks/update-operation.go`
- `mocks/decision-ack-operation.go`
- `mocks/goal-complete-operation.go`
- `mocks/objective-complete-operation.go`

### 10. Update `pkg/cli/cli.go` — CLI call sites

For each command, receive the `(result, err)` from Execute, then format output.

**General pattern for all mutation commands:**
```go
result, err := op.Execute(ctx, ...)
if err != nil {
    if *outputFormat == cli.OutputFormatJSON {
        _ = cli.PrintJSON(result) // print error result before returning error
    }
    return err
}
if *outputFormat == cli.OutputFormatJSON {
    return cli.PrintJSON(result)
}
// Plain text output (specific to each command, see below)
```

**createCompleteCommand** plain text:
- If result has Reason set (subtask blocking): print `"⚠️  Cannot complete: %s\n"` and the reason
- Otherwise: `"✅ Task completed: %s\n"`, task name
- Print any warnings: `"⚠️  %s\n"` per warning

**createDeferCommand** plain text:
- `"📅 Task deferred to %s: %s\n"` (date, task name)
- Print any warnings

**createWorkOnCommand** plain text:
- `"✅ Now working on: %s (assigned to %s)\n"` (task name, assignee)
- If result.SessionID != "": `"session_id: %s\n"` (result.SessionID)

**createUpdateCommand** plain text:
- For no-checkboxes case (warning present): `"⚠️  %s\n"` for each warning
- Otherwise: `"✅ Updated %s/%s: %d/%d checkboxes complete\n"` — preserve original format
  - Note: the original format string uses `taskName`, `vaultName`, and counts from the task
  - Since MutationResult doesn't carry checkbox counts, add optional fields or encode in a Warning string
  - Simplest approach: add `Message string` to MutationResult and populate it in updateOperation with the formatted count string; CLI prints it in plain mode

**createDecisionAckCommand** plain text:
- `"Acknowledged: %s\n"` (result.Name)

**createGoalCompleteCommand** plain text:
- `"✅ Goal completed: %s\n"` (result.Name)

**createObjectiveCompleteCommand** plain text:
- `"✅ Objective completed: %s\n"` (result.Name)

### 11. Add Message field to MutationResult

Add `Message string` to `MutationResult`:
```go
Message string `json:"message,omitempty"`
```
Use this in `updateOperation.Execute` to carry the checkbox count string for plain-text display.

### 12. Update tests

In test files for each changed operation:
- Remove stdout capture / `os.Stdout` redirect setup
- Assert on the returned `MutationResult` values directly
- Keep all test cases; only change assertion style

Files to update:
- `pkg/ops/complete_test.go`
- `pkg/ops/defer_test.go`
- `pkg/ops/workon_test.go`
- `pkg/ops/update_test.go`
- `pkg/ops/decision_ack_test.go` (if exists)
- `pkg/ops/goal_complete_test.go`
- `pkg/ops/objective_complete_test.go` (if exists)
</requirements>

<constraints>
- CLI output format must not change — same text, same JSON structure, same field names
- Operation naming convention is preserved (no renames)
- Mock generation comments (`//counterfeiter:generate`) are preserved; mocks are regenerated
- Factory function pattern (pure composition, no I/O) is preserved
- Do NOT commit — dark-factory handles git
- Existing tests must still pass after assertion updates
- No operation in pkg/ops/ may write to os.Stdout after this prompt (for the seven operations changed here)
- MutationResult type remains in `pkg/ops/complete.go` — do not move it
</constraints>

<verification>
```
make precommit
```

```
grep -r 'os\.Stdout' pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go pkg/ops/update.go pkg/ops/decision_ack.go pkg/ops/goal_complete.go pkg/ops/objective_complete.go
# expected: no output
```

```
grep -r 'fmt\.Print' pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go pkg/ops/update.go pkg/ops/decision_ack.go pkg/ops/goal_complete.go pkg/ops/objective_complete.go
# expected: no output
```
</verification>
