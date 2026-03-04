---
status: completed
summary: Injected libtime.CurrentDateTime to replace all time.Now() calls in pkg/ops/
container: vault-cli-039-inject-libtime-current-datetime
dark-factory-version: v0.14.5
created: "2026-03-04T07:36:36Z"
queued: "2026-03-04T07:36:36Z"
started: "2026-03-04T07:36:36Z"
completed: "2026-03-04T07:46:19Z"
---
# Inject libtime.CurrentDateTime to Replace time.Now()

## Goal

Replace all direct `time.Now()` calls in `pkg/ops/` with an injected `libtime.CurrentDateTime` dependency from `github.com/bborbe/time`. This makes all time-dependent code testable with deterministic time.

## Why

Tests currently hardcode dates and break when the calendar date changes. The `libtime.CurrentDateTime` pattern provides `SetNow()` for tests while using real time in production — single instance shared across all ops.

## Constraints

- Do NOT change any interfaces (`CompleteOperation`, `DeferOperation`, `WorkOnOperation`, `UpdateOperation`)
- Do NOT change `pkg/cli/cli.go` command signatures — the `CurrentDateTime` is created once in the factory and passed to all ops
- Do NOT change any behavior — only inject time source
- All existing tests must pass
- Use `import libtime "github.com/bborbe/time"` and `import libtimetest "github.com/bborbe/time/test"`
- `make precommit` must pass

## Library Reference

```go
import libtime "github.com/bborbe/time"
import libtimetest "github.com/bborbe/time/test"

// Production: create once, pass to all consumers
currentDateTime := libtime.NewCurrentDateTime()

// Usage in code: replace time.Now()
now := s.currentDateTime.Now()            // returns libtime.DateTime
today := now.Format("2006-01-02")         // same as time.Time
stdTime := now.Time()                     // convert to time.Time if needed

// Tests: control time deterministically
currentDateTime := libtime.NewCurrentDateTime()
currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
```

`libtime.DateTime` wraps `time.Time` — has same methods (`Format`, `AddDate`, `Truncate`, `Before`, `After`, `Weekday`). Use `.Time()` only where a `time.Time` is explicitly needed (e.g., assigning to `*time.Time` fields).

## Steps

### 1. Add dependency

```bash
go get github.com/bborbe/time@latest
```

### 2. Update operation structs

Add `currentDateTime libtime.CurrentDateTime` field to:
- `completeOperation` in `pkg/ops/complete.go`
- `deferOperation` in `pkg/ops/defer.go`
- `workOnOperation` in `pkg/ops/workon.go`

(`updateOperation` in `pkg/ops/update.go` does NOT use `time.Now()` — skip it.)

### 3. Update factory functions

Add `currentDateTime libtime.CurrentDateTime` parameter to:
- `NewCompleteOperation(storage, currentDateTime)`
- `NewDeferOperation(storage, currentDateTime)`
- `NewWorkOnOperation(storage, currentDateTime)`

### 4. Replace all time.Now() calls

In `pkg/ops/complete.go`:
- Line ~129: `today := time.Now().Format("2006-01-02")` → `today := c.currentDateTime.Now().Format("2006-01-02")`
- Line ~216: `now := time.Now()` → `now := c.currentDateTime.Now().Time()`

In `pkg/ops/defer.go`:
- Line ~63: `today := time.Now().Truncate(...)` → `today := c.currentDateTime.Now().Time().Truncate(...)`
- Line ~132: `today := time.Now().Format(...)` → `today := d.currentDateTime.Now().Format(...)`
- Line ~173: `now := time.Now()` → `now := d.currentDateTime.Now().Time()`

In `pkg/ops/workon.go`:
- Line ~92: `today := time.Now().Format(...)` → `today := w.currentDateTime.Now().Format(...)`

### 5. Update pkg/cli/cli.go

Create ONE `currentDateTime` instance and pass to all ops:

```go
import libtime "github.com/bborbe/time"

// In the command creation area (near the top of Run or wherever ops are created):
currentDateTime := libtime.NewCurrentDateTime()

// Then pass to each op:
completeOp := ops.NewCompleteOperation(store, currentDateTime)
deferOp := ops.NewDeferOperation(store, currentDateTime)
// etc.
```

There are multiple call sites in cli.go (single vault + loop). ALL must pass `currentDateTime`.

### 6. Update ALL tests

In every test file that creates ops, inject a fixed-time `CurrentDateTime`:

```go
import libtime "github.com/bborbe/time"
import libtimetest "github.com/bborbe/time/test"

// In BeforeEach:
currentDateTime := libtime.NewCurrentDateTime()
currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
completeOp = ops.NewCompleteOperation(mockStorage, currentDateTime)
```

Fix ALL hardcoded date strings in tests to match the fixed time (`2026-03-03`). The tests already use `2026-03-03` — just ensure they stay consistent with the `SetNow` value.

For `defer_test.go`:
- All `todayContent` headers use `# 2026-03-03` — matches `SetNow("2026-03-03T12:00:00Z")`
- All `Expect(date).To(ContainSubstring("2026-03-03"))` — will now pass regardless of real date

For `complete_test.go`:
- Lines using `time.Now()` for planned_date calculations should use the same fixed time

### 7. Run go mod tidy and vendor (if applicable)

```bash
go mod tidy
```

## Verification

```bash
make precommit
```

All 271+ tests must pass. No test should depend on the real system clock.

## Files to modify

- `go.mod` (add dependency)
- `pkg/ops/complete.go` (inject + use)
- `pkg/ops/defer.go` (inject + use)
- `pkg/ops/workon.go` (inject + use)
- `pkg/ops/complete_test.go` (fixed time)
- `pkg/ops/defer_test.go` (fixed time)
- `pkg/ops/workon_test.go` (fixed time, if exists)
- `pkg/cli/cli.go` (create + pass)
