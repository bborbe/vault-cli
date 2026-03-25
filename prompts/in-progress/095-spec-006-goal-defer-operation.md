---
status: approved
spec: ["006"]
created: "2026-03-25T09:30:00Z"
queued: "2026-03-25T09:29:41Z"
---

<summary>
- Date parsing logic is extracted into a shared helper so goals and tasks reuse the same rules
- A new goal defer operation sets the defer date on goals
- Deferring a goal by relative days, weekday name, or ISO date all work
- Past dates are rejected with a clear error; invalid formats produce a format error
- Goal defer does not update daily notes (unlike task defer)
- JSON output is supported for the new command
- Test infrastructure is set up for the new operation
- All existing task defer tests pass unchanged
</summary>

<objective>
Create `GoalDeferOperation` in `pkg/ops/goal_defer.go` that sets `defer_date` on a goal using the same date-parsing rules as task defer, wire it into `vault-cli goal defer`, and cover it with tests. Prompt 1 (adding `defer_date` to `domain.Goal`) must be completed first.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/ops/defer.go` — full file; `parseDate`, `isInPast`, `nextWeekday` are methods on `deferOperation`; study them to extract as shared helpers
- `pkg/ops/defer_test.go` — test patterns: Ginkgo, counterfeiter mocks, `libtimetest.ParseDateTime`
- `pkg/ops/goal_complete.go` — GoalCompleteOperation as structural template for GoalDeferOperation
- `pkg/ops/complete.go` — `MutationResult` type definition
- `pkg/domain/goal.go` — Goal struct with `DeferDate *DateOrDateTime` added in Prompt 1
- `pkg/storage/storage.go` — GoalStorage interface: `FindGoalByName`, `WriteGoal`
- `pkg/cli/cli.go` — `createGoalCommands` (line ~1129) and `createDeferCommand` (line ~160) as pattern for CLI wiring
- `mocks/` — directory for counterfeiter-generated mocks
</context>

<requirements>
### 1. Extract shared date parsing to `pkg/ops/defer_date_parser.go`

Create a new file `pkg/ops/defer_date_parser.go` with package-level helpers extracted from `deferOperation`:

```go
// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
    "fmt"
    "regexp"
    "strings"
    "time"

    libtime "github.com/bborbe/time"

    "github.com/bborbe/vault-cli/pkg/domain"
)

// parseDeferDate parses a date string using the same rules as task defer:
// +Nd (relative days), weekday names, YYYY-MM-DD (ISO date), RFC3339 datetime.
func parseDeferDate(dateStr string, now time.Time) (domain.DateOrDateTime, error) {
    // Handle relative dates: +1d, +7d, etc.
    if matched, _ := regexp.MatchString(`^\+\d+d$`, dateStr); matched {
        var days int
        if _, err := fmt.Sscanf(dateStr, "+%dd", &days); err != nil {
            return domain.DateOrDateTime{}, fmt.Errorf("parse relative date: %w", err)
        }
        t := libtime.ToDate(now.AddDate(0, 0, days)).Time()
        return domain.DateOrDateTime(t), nil
    }

    // Handle weekday names
    weekdayMap := map[string]time.Weekday{
        "monday":    time.Monday,
        "tuesday":   time.Tuesday,
        "wednesday": time.Wednesday,
        "thursday":  time.Thursday,
        "friday":    time.Friday,
        "saturday":  time.Saturday,
        "sunday":    time.Sunday,
    }
    if weekday, ok := weekdayMap[strings.ToLower(dateStr)]; ok {
        t := libtime.ToDate(nextWeekday(now, weekday)).Time()
        return domain.DateOrDateTime(t), nil
    }

    // Handle ISO date: 2024-12-31
    if t, err := time.Parse("2006-01-02", dateStr); err == nil {
        return domain.DateOrDateTime(t), nil
    }

    // Handle RFC3339 datetime: 2026-03-19T16:00:00+01:00
    if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
        return domain.DateOrDateTime(t), nil
    }

    return domain.DateOrDateTime{}, fmt.Errorf(
        "invalid date format: %s (use +Nd, weekday, YYYY-MM-DD, or RFC3339)",
        dateStr,
    )
}

// isDeferDateInPast reports whether targetDate is in the past relative to now.
// Date-only values (midnight UTC) are compared at day granularity so "today" is never past.
func isDeferDateInPast(targetDate domain.DateOrDateTime, now time.Time) bool {
    targetT := targetDate.Time()
    targetUTC := targetT.UTC()
    if targetUTC.Hour() == 0 && targetUTC.Minute() == 0 && targetUTC.Second() == 0 &&
        targetUTC.Nanosecond() == 0 {
        todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
        return targetT.Before(todayMidnight)
    }
    return targetT.Before(now)
}

// nextWeekday returns the next occurrence of the specified weekday after from.
func nextWeekday(from time.Time, targetWeekday time.Weekday) time.Time {
    daysUntil := (int(targetWeekday) - int(from.Weekday()) + 7) % 7
    if daysUntil == 0 {
        daysUntil = 7 // Next week's occurrence
    }
    return from.AddDate(0, 0, daysUntil)
}
```

### 2. Refactor `pkg/ops/defer.go` to use the shared helpers

In `deferOperation`, replace the three methods `parseDate`, `isInPast`, and `nextWeekday` with calls to the package-level helpers:

- Replace `d.parseDate(dateStr)` → `parseDeferDate(dateStr, d.currentDateTime.Now().Time())`
- Remove the `parseDate` method from `deferOperation`
- Replace `d.isInPast(targetDate, now)` → `isDeferDateInPast(targetDate, now)`
- Remove the `isInPast` method from `deferOperation`
- Remove the `nextWeekday` method from `deferOperation` (now package-level)

The existing `deferOperation.Execute` logic (preserving time component for `+Nd` when existing `DeferDate` has a time, updating daily notes) must remain unchanged. Only the helper methods move to the new file.

After this refactor, `pkg/ops/defer.go` must compile and all existing defer tests must pass without modification.

### 3. Create `pkg/ops/goal_defer.go`

```go
// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
    "context"
    "fmt"

    "github.com/bborbe/errors"
    libtime "github.com/bborbe/time"

    "github.com/bborbe/vault-cli/pkg/domain"
    "github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/goal-defer-operation.go --fake-name GoalDeferOperation . GoalDeferOperation
type GoalDeferOperation interface {
    Execute(
        ctx context.Context,
        vaultPath string,
        goalName string,
        dateStr string,
        vaultName string,
    ) (MutationResult, error)
}

// NewGoalDeferOperation creates a new goal defer operation.
func NewGoalDeferOperation(
    goalStorage storage.GoalStorage,
    currentDateTime libtime.CurrentDateTime,
) GoalDeferOperation {
    return &goalDeferOperation{
        goalStorage:     goalStorage,
        currentDateTime: currentDateTime,
    }
}

type goalDeferOperation struct {
    goalStorage     storage.GoalStorage
    currentDateTime libtime.CurrentDateTime
}

// Execute sets defer_date on a goal without updating daily notes.
func (g *goalDeferOperation) Execute(
    ctx context.Context,
    vaultPath string,
    goalName string,
    dateStr string,
    vaultName string,
) (MutationResult, error) {
    now := g.currentDateTime.Now().Time()

    targetDate, err := parseDeferDate(dateStr, now)
    if err != nil {
        return MutationResult{
            Success: false,
            Error:   err.Error(),
        }, errors.Wrap(ctx, err, "parse date")
    }

    if isDeferDateInPast(targetDate, now) {
        baseErr := fmt.Errorf(
            "cannot defer to past date: %s",
            targetDate.Time().Format("2006-01-02"),
        ) //nolint:goerr113
        return MutationResult{
            Success: false,
            Error:   baseErr.Error(),
        }, errors.Wrap(ctx, baseErr, "validate date")
    }

    goal, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
    if err != nil {
        return MutationResult{
            Success: false,
            Error:   err.Error(),
        }, errors.Wrap(ctx, err, "find goal")
    }

    goal.DeferDate = targetDate.Ptr()

    if err := g.goalStorage.WriteGoal(ctx, goal); err != nil {
        return MutationResult{
            Success: false,
            Error:   err.Error(),
        }, errors.Wrap(ctx, err, "write goal")
    }

    formattedDate := targetDate.Time().Format("2006-01-02")
    return MutationResult{
        Success: true,
        Name:    goal.Name,
        Vault:   vaultName,
        Message: formattedDate,
    }, nil
}
```

Note: `domain.DateOrDateTime` has a `Ptr()` method if it follows the same pattern as the `Task.DeferDate`. Verify by reading `pkg/domain/date_or_datetime.go`. If `Ptr()` does not exist, use `&targetDate` directly (after converting: `dod := domain.DateOrDateTime(targetDate); goal.DeferDate = &dod`).

### 4. Generate the mock

Run:
```
go generate ./pkg/ops/...
```

This creates `mocks/goal-defer-operation.go`.

### 5. Wire `vault-cli goal defer` in `pkg/cli/cli.go`

Add a new command `createGoalDeferCommand` and register it in `createGoalCommands`.

**Add the constructor** (place it after `createGoalCompleteCommand`):

```go
func createGoalDeferCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    return &cobra.Command{
        Use:   "defer <goal-name> [date]",
        Short: "Defer a goal to a specific date",
        Long: `Defer a goal to a specific date.

If no date is provided, defaults to +1d (tomorrow).

Date formats:
  +Nd                        - Relative days (e.g., +7d for 7 days from now)
  monday                     - Next occurrence of weekday
  2024-12-31                 - ISO date format (YYYY-MM-DD)
  2026-03-19T16:00:00+01:00  - Full datetime with timezone`,
        Args: cobra.RangeArgs(1, 2),
        RunE: func(cmd *cobra.Command, args []string) error {
            goalName := args[0]
            dateStr := "+1d"
            if len(args) > 1 {
                dateStr = args[1]
            }

            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return errors.Wrap(ctx, err, "get vaults")
            }

            currentDateTime := libtime.NewCurrentDateTime()

            dispatcher := ops.NewVaultDispatcher()
            return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                storageConfig := storage.NewConfigFromVault(vault)
                goalStore := storage.NewGoalStorage(storageConfig)
                deferOp := ops.NewGoalDeferOperation(goalStore, currentDateTime)
                result, err := deferOp.Execute(
                    ctx,
                    vault.Path,
                    goalName,
                    dateStr,
                    vault.Name,
                )
                if err != nil {
                    if *outputFormat == OutputFormatJSON {
                        _ = PrintJSON(result)
                    }
                    return err
                }
                if *outputFormat == OutputFormatJSON {
                    return PrintJSON(result)
                }
                fmt.Printf("📅 Goal deferred to %s: %s\n", result.Message, result.Name)
                return nil
            })
        },
    }
}
```

**Register in `createGoalCommands`** — add after `createGoalCompleteCommand`:

```go
cmd.AddCommand(createGoalDeferCommand(ctx, configLoader, vaultName, outputFormat))
```

### 6. Write tests for `GoalDeferOperation` in `pkg/ops/goal_defer_test.go`

Use Ginkgo/Gomega with counterfeiter mocks. Cover all acceptance criteria:

- `+7d` sets `defer_date` 7 days from now (verify `WriteGoal` called with correct `DeferDate`)
- weekday name (e.g., `"monday"`) sets `defer_date` to next Monday
- ISO date (`"2026-12-31"`) sets `defer_date` to that date
- Past date (`"2025-01-01"`) returns error containing `"cannot defer to past date"`
- Invalid format (`"invalid"`) returns error containing `"invalid date format"`
- Goal not found: `FindGoalByName` returns error → `Execute` returns error
- `WriteGoal` fails → `Execute` returns error
- Successful result has `Success: true`, `Name: goalName`, `Vault: vaultName`, `Message: "YYYY-MM-DD"`

Use `mocks.GoalStorage` (counterfeiter mock). Set `currentDateTime` to a fixed time (`libtimetest.ParseDateTime("2026-03-25T12:00:00Z")`). Follow the `defer_test.go` pattern exactly (Describe/Context/It/BeforeEach, `JustBeforeEach` calling Execute).

Coverage target: ≥80% statement coverage for `pkg/ops/goal_defer.go`.
</requirements>

<constraints>
- Task defer behavior must not change — the refactor of `deferOperation` to use package-level helpers must be purely mechanical (no logic change)
- Daily notes must NOT be updated by `goal defer` — goals are not tracked in daily notes
- Date parsing logic is shared via package-level helpers, not duplicated
- The `DeferDate` field on `domain.Goal` must use the same `*domain.DateOrDateTime` type as `domain.Task.DeferDate`
- JSON output must work for `goal defer` (`--output json` returns `MutationResult` JSON)
- `//counterfeiter:generate` annotation must be present on the `GoalDeferOperation` interface
- Do NOT commit — dark-factory handles git
- All existing tests must pass after the `defer.go` refactor
</constraints>

<verification>
```
make precommit
```

```
# Confirm GoalDeferOperation mock was generated
ls mocks/goal-defer-operation.go
```

```
# Confirm date helpers are in new file, not duplicated
grep -n 'func parseDeferDate\|func isDeferDateInPast\|func nextWeekday' pkg/ops/defer_date_parser.go
# expected: 3 lines
```

```
# Confirm parseDate/isInPast/nextWeekday methods removed from deferOperation
grep -n 'func (d \*deferOperation) parseDate\|func (d \*deferOperation) isInPast\|func (d \*deferOperation) nextWeekday' pkg/ops/defer.go
# expected: no output
```

```
# Confirm goal defer command is registered
grep -n 'createGoalDeferCommand' pkg/cli/cli.go
# expected: two lines (definition + registration)
```
</verification>
