---
status: completed
summary: Extended task date fields to support datetime-with-timezone via new DateOrDateTime type, updated defer command to accept RFC3339 strings and preserve time on +Nd offsets
container: vault-cli-079-defer-datetime-support
dark-factory-version: v0.57.5
created: "2026-03-18T14:01:12Z"
queued: "2026-03-18T14:01:12Z"
started: "2026-03-18T14:01:17Z"
completed: "2026-03-18T14:18:10Z"
---

<summary>
- All task date fields (defer_date, planned_date, due_date) support both date-only and datetime-with-timezone values
- A new local type handles smart serialization: date-only values stay as YYYY-MM-DD, datetime values serialize as RFC3339
- The defer command accepts RFC3339 datetime strings directly (e.g. 2026-03-19T16:00:00+01:00)
- When deferring with a relative offset (+Nd) and the task already has a time component, the time and timezone are preserved
- Backward compatible: existing YYYY-MM-DD frontmatter values continue to work unchanged
</summary>

<objective>
Extend all task date fields to support full datetime-with-timezone values in frontmatter, without breaking existing date-only usage. The defer command gains the ability to set a specific time-of-day, and relative offsets (+Nd) preserve the existing time component.
</objective>

<context>
Read CLAUDE.md for project conventions and dark-factory workflow.

Key files:
- `pkg/domain/task.go` — Task struct with DeferDate, PlannedDate, DueDate as `*libtime.Date`
- `pkg/ops/defer.go` — defer operation; parseDate returns libtime.Date; Execute validates past dates
- `pkg/ops/frontmatter.go` — get/set operations for task fields; parseDatePtr helper parses YYYY-MM-DD only
- `pkg/storage/base.go` — parseFrontmatter uses yaml.Unmarshal into domain.Task; serializeWithFrontmatter uses yaml.Marshal
- `github.com/bborbe/time` — libtime package; DateTime type with MarshalText→RFC3339Nano; ParseTime handles RFC3339, DateOnly, and other formats via UnmarshalText

The yaml library calls MarshalText/UnmarshalText for types that implement encoding.TextMarshaler/TextUnmarshaler.
</context>

<requirements>
1. Create `pkg/domain/date_or_datetime.go` with a new type `DateOrDateTime` (wraps `time.Time`):
   - Implements `encoding.TextMarshaler`: if the underlying `time.Time` has zero hour/minute/second/nanosecond (checked in UTC via `t.UTC()`), format as `time.DateOnly` ("2006-01-02"); otherwise format as `time.RFC3339`. Note: `2026-03-19T00:00:00+01:00` has UTC value `2026-03-18T23:00:00Z` so its UTC hour is non-zero and it serializes as RFC3339 — only pure `2026-03-19` (→ `00:00:00Z`) gets DateOnly format.
   - Implements `encoding.TextUnmarshaler`: delegates to `libtime.ParseTime(ctx context.Context, value interface{}) (*time.Time, error)` which handles both formats
   - Add `Time() time.Time` method
   - Add `Ptr() *DateOrDateTime` method
   - Add `IsZero() bool` method
   - Add `Before(other time.Time) bool` method (note: existing code uses `(*task.PlannedDate).Before(targetDate)` with `libtime.Date` — update callers to pass `.Time()` since `Before` now takes `time.Time`)

2. In `pkg/domain/task.go`, change the three date fields:
   - `DeferDate   *libtime.Date` → `DeferDate   *DateOrDateTime`
   - `PlannedDate *libtime.Date` → `PlannedDate *DateOrDateTime`
   - `DueDate     *libtime.Date` → `DueDate     *DateOrDateTime`
   - Update imports: remove `libtime` import if no longer used elsewhere in that file

3. In `pkg/ops/frontmatter.go`:
   - Replace `parseDatePtr` function: change return type from `*libtime.Date` to `*domain.DateOrDateTime`; use `libtime.ParseTime` for parsing (already handles both formats); construct `DateOrDateTime` from the result
   - In the get operation (`frontmatterGetOperation.Execute`), update the three date cases to call `task.DeferDate.Time().Format(...)` with the same smart logic as MarshalText (DateOnly if zero time, RFC3339 otherwise)
   - In the set operation (`frontmatterSetOperation.Execute`), update the three date cases to use the updated `parseDatePtr` return type
   - In the clear operation, `task.DeferDate = nil` etc. — no change needed

4. In `pkg/ops/defer.go`:
   - Change `parseDate` to return `domain.DateOrDateTime` instead of `libtime.Date`
   - Accept RFC3339 datetime strings by using `libtime.ParseTime` directly (it already handles RFC3339)
   - Weekday and `+Nd` parsing: keep existing logic but return `DateOrDateTime`
   - In `Execute`, after finding the task: if dateStr matches `+Nd` pattern AND `task.DeferDate != nil` AND the existing defer has a non-zero time component, preserve the time and timezone when adding days (use `existingTime.AddDate(0, 0, N)` — NOT `Add(N*24h)` to handle DST transitions correctly)
   - Update past-date validation: compare `targetDateTime.Time().Before(now)` using full datetime instead of date-only. Edge case: deferring to "today at 4pm" when it's 2pm must be allowed (the current date-only comparison would reject same-day future times)
   - Update `findAndDeferTask`: `task.DeferDate = targetDate.Ptr()` — no signature change needed since type changes
   - Update PlannedDate comparison: `(*task.PlannedDate).Before(targetDate)` → `task.PlannedDate.Before(targetDate.Time())` since `Before` now takes `time.Time`
   - Update `updateDailyNotes` and `formatResult`: format date as `DateOnly` for display/daily-note key (use `targetDate.Time().Format("2006-01-02")`)

5. In `pkg/cli/cli.go`, update the defer command:
   - Update `Long` description to add the RFC3339 format: `2026-03-19T16:00:00+01:00 - Full datetime with timezone`
   - No changes needed to args parsing — the single `[date]` argument already accepts any string

6. In `pkg/ops/defer.go`, also update the private method signatures that accept `libtime.Date`:
   - `findAndDeferTask(ctx, vaultPath, taskName, targetDate libtime.Date, format)` → change `targetDate` param to `domain.DateOrDateTime`
   - `updateDailyNotes(ctx, vaultPath, taskName, targetDate libtime.Date)` → change `targetDate` param to `domain.DateOrDateTime`; extract date string via `targetDate.Time().Format("2006-01-02")`
   - `formatResult(name, vault, targetDate libtime.Date, warnings, format)` → change `targetDate` param to `domain.DateOrDateTime`; extract display date via `targetDate.Time().Format("2006-01-02")`

7. Create `pkg/domain/date_or_datetime_test.go` with unit tests for `DateOrDateTime`:
   - `MarshalText` for a date-only value (zero time) → `YYYY-MM-DD`
   - `MarshalText` for a datetime with timezone → RFC3339
   - `UnmarshalText` round-trip for `YYYY-MM-DD` → MarshalText produces `YYYY-MM-DD`
   - `UnmarshalText` round-trip for RFC3339 → MarshalText produces RFC3339
   - `UnmarshalText` for empty string → nil/zero

8. Update `pkg/ops/defer_test.go` and `pkg/ops/frontmatter_test.go` (if it exists) for changed types:
   - Update test assertions to use `DateOrDateTime` instead of `libtime.Date`
   - Add a test case: defer with RFC3339 datetime string sets time component correctly
   - Add a test case: `+1d` on a task with existing DeferDate with time → preserves time, adds 1 day
   - Add a test case: `+1d` on a task with date-only DeferDate → stays date-only

9. Run `make test` to verify all tests pass before completing.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- YAML frontmatter with `defer_date: 2026-03-19` must continue to round-trip correctly as `defer_date: 2026-03-19`
- YAML frontmatter with `defer_date: 2026-03-19T16:00:00+01:00` must round-trip correctly
- `planned_date` and `due_date` behavior is unchanged for date-only values
- All paths are repo-relative
</constraints>

<verification>
Run `make test` — must pass with no failures.
</verification>
