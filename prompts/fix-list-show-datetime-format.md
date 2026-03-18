---
status: created
---

<summary>
- The task list and task show JSON outputs now preserve the full datetime when defer_date, planned_date, or due_date have a time component
- Date-only values continue to format as YYYY-MM-DD in JSON output
- Datetime values (e.g. 2026-03-18T16:00:00+01:00) format as RFC3339 in JSON output
- A shared helper `formatDateOrDateTime` centralizes the smart format logic, consistent with `DateOrDateTime.MarshalText`
- Plain-text (non-JSON) output is unchanged
- All existing tests continue to pass
</summary>

<objective>
Fix `pkg/ops/list.go` and `pkg/ops/show.go` which hardcode `Format("2006-01-02")` for date fields, discarding any time component. Use smart formatting: DateOnly when time is midnight UTC, RFC3339 otherwise — consistent with how `DateOrDateTime.MarshalText` works.
</objective>

<context>
Read CLAUDE.md for project conventions.

`pkg/domain/date_or_datetime.go` — `DateOrDateTime` type; `MarshalText` uses UTC zero-time check to pick DateOnly vs RFC3339.

In `pkg/ops/list.go` (~line 115) and `pkg/ops/show.go` (~line 98), the JSON output structs populate date fields with `task.DeferDate.Format("2006-01-02")` — this discards the time component.
</context>

<requirements>
1. In `pkg/ops/list.go`, replace the three date format calls:
   - Old: `items[i].DeferDate = task.DeferDate.Format("2006-01-02")`
   - New: `items[i].DeferDate = formatDateOrDateTime(task.DeferDate)`
   - Old: `items[i].PlannedDate = task.PlannedDate.Format("2006-01-02")`
   - New: `items[i].PlannedDate = formatDateOrDateTime(task.PlannedDate)`
   - Old: `items[i].DueDate = task.DueDate.Format("2006-01-02")`
   - New: `items[i].DueDate = formatDateOrDateTime(task.DueDate)`

2. In `pkg/ops/show.go`, replace the three date format calls:
   - Old: `detail.DeferDate = task.DeferDate.Format("2006-01-02")`
   - New: `detail.DeferDate = formatDateOrDateTime(task.DeferDate)`
   - Old: `detail.PlannedDate = task.PlannedDate.Format("2006-01-02")`
   - New: `detail.PlannedDate = formatDateOrDateTime(task.PlannedDate)`
   - Old: `detail.DueDate = task.DueDate.Format("2006-01-02")`
   - New: `detail.DueDate = formatDateOrDateTime(task.DueDate)`
   - Note: `show.go` may not import `domain` yet — add the import if needed for the `*domain.DateOrDateTime` parameter type

3. Add the helper function `formatDateOrDateTime` in `pkg/ops/list.go` (or a shared file if one exists for ops helpers):
   ```go
   func formatDateOrDateTime(d *domain.DateOrDateTime) string {
       if d == nil {
           return ""
       }
       t := d.Time().UTC()
       if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
           return d.Format(time.DateOnly)
       }
       return d.Format(time.RFC3339)
   }
   ```
   If `show.go` is in the same package, it can reuse the same function — no duplication needed.

4. Add a unit test for `formatDateOrDateTime` in `pkg/ops/list_test.go` (or appropriate test file):
   - Test with date-only `DateOrDateTime` (midnight UTC) → returns `YYYY-MM-DD`
   - Test with datetime `DateOrDateTime` (non-zero time) → returns RFC3339
   - Test with `nil` → returns empty string

5. Run `make test` to verify all tests pass.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Date-only values must still output as YYYY-MM-DD (no regression)
- All paths are repo-relative
</constraints>

<verification>
Run `make test` — must pass with no failures.
</verification>
