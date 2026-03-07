---
status: completed
summary: Changed DeferDate and PlannedDate in Task domain model from *time.Time to *libtime.Date so YAML serialization produces date-only values, updated all callers and tests accordingly, and upgraded github.com/bborbe/time to v1.25.0
container: vault-cli-041-fix-defer-date-type
dark-factory-version: v0.25.1
created: "2026-03-07T21:45:00Z"
queued: "2026-03-07T21:30:28Z"
started: "2026-03-07T21:30:33Z"
completed: "2026-03-07T21:39:50Z"
---

<summary>
- Date fields in task files will serialize as `2026-03-08` instead of `2026-03-08T21:35:32+01:00`
- No user-visible behavior change — only the stored format improves
- All internal date comparisons updated to use the new type's built-in methods
- Weekday comparisons use the library's own weekday type throughout
- Adds test coverage for both the actual and expected sides of assertions
</summary>

<objective>
Change `DeferDate` and `PlannedDate` in the Task domain model from `*time.Time` to `*libtime.Date` so YAML serialization produces date-only values (`2026-03-08`) instead of full timestamps (`2026-03-08T21:35:32.742132+01:00`). The `libtime.Date` type from `github.com/bborbe/time` already implements `encoding.TextMarshaler` returning `2006-01-02` format.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/domain/task.go` — the `Task` struct with `DeferDate` and `PlannedDate` fields.
Read `pkg/ops/defer.go` — `Execute`, `parseDate`, `findAndDeferTask`, `updateDailyNotes`, `formatResult`, `nextWeekday` methods.
Read `pkg/ops/complete.go` — `calculateNextDeferDate` function and recurring completion logic.
Read `pkg/ops/frontmatter.go` — `FrontmatterGetOperation.Execute` and `FrontmatterSetOperation.Execute` for DeferDate get/set.
Read `pkg/ops/defer_test.go` — tests for DeferDate and PlannedDate assertions.
Read `pkg/ops/complete_test.go` — tests for recurring completion DeferDate and PlannedDate.
Read `pkg/ops/frontmatter_test.go` — tests for frontmatter get/set/clear of DeferDate.
Read `~/Documents/workspaces/time/time_date.go` for the `Date` type API. Key methods: `ToDate(time.Time) Date`, `.Time() time.Time`, `.Ptr() *Date`, `.Weekday() libtime.Weekday`, `.Format()`, `.Before(HasTime)`, `.After(HasTime)`, `.Truncate(HasDuration)`. Note: `Before`/`After` take `HasTime` interface — `time.Time` does NOT implement `HasTime`, so wrap with `libtime.ToDate()` for comparisons against raw `time.Time` values.
Read `~/Documents/workspaces/coding-guidelines/go-testing-guide.md` for test patterns.
</context>

<requirements>
1. First run `go get github.com/bborbe/time@v1.25.0 && go mod vendor` to pull in the latest `libtime.Date` which has `Before`, `After`, and `Truncate` methods.

2. In `pkg/domain/task.go`, in the `Task` struct:
   - Add import: `libtime "github.com/bborbe/time"`
   - Change `DeferDate *time.Time` → `DeferDate *libtime.Date`
   - Change `PlannedDate *time.Time` → `PlannedDate *libtime.Date`
   - Remove `"time"` import if no longer needed (check other usages first)

3. In `pkg/ops/defer.go`:

   In `parseDate` method — change return type from `(time.Time, error)` to `(libtime.Date, error)`:
   - Relative date branch: `return now.AddDate(0, 0, days), nil` → `return libtime.ToDate(now.AddDate(0, 0, days)), nil`
   - Weekday branch: `return d.nextWeekday(now, weekday), nil` → `return libtime.ToDate(d.nextWeekday(now, weekday)), nil`
   - ISO date branch: `return t, nil` → `return libtime.ToDate(t), nil`
   - Error return: `return time.Time{}, ...` → `return libtime.Date{}, ...`

   In `Execute` method — fix date validation:
   - `today := d.currentDateTime.Now().Time().Truncate(24 * time.Hour)` → `today := libtime.ToDate(d.currentDateTime.Now().Time())`
   - Remove `targetDateTruncated := targetDate.Truncate(24 * time.Hour)` entirely
   - `if targetDateTruncated.Before(today)` → `if targetDate.Before(today)` (Date.Before takes HasTime; libtime.Date implements HasTime)

   In `findAndDeferTask` method:
   - Change `targetDate time.Time` parameter → `targetDate libtime.Date`
   - `task.DeferDate = &targetDate` → `task.DeferDate = targetDate.Ptr()`
   - `task.PlannedDate.Before(targetDate)` — PlannedDate is now `*libtime.Date` (pointer). Dereference: `(*task.PlannedDate).Before(targetDate)` or restructure the nil-check + comparison

   In `updateDailyNotes` method:
   - Change `targetDate time.Time` parameter → `targetDate libtime.Date`

   In `formatResult` method:
   - Change `targetDate time.Time` parameter → `targetDate libtime.Date`

   Keep `nextWeekday` returning `time.Time` — internal helper, converted to `Date` at call site.

4. In `pkg/ops/complete.go`:

   In `calculateNextDeferDate` function — change return type from `time.Time` to `libtime.Date`. Wrap all return values with `libtime.ToDate()`. There are 7 return points:
   - `"daily"` case: `return now.AddDate(0, 0, 1)` → `return libtime.ToDate(now.AddDate(0, 0, 1))`
   - `"weekly"` case: `return now.AddDate(0, 0, 7)` → `return libtime.ToDate(now.AddDate(0, 0, 7))`
   - `"monthly"` case: `return now.AddDate(0, 1, 0)` → `return libtime.ToDate(now.AddDate(0, 1, 0))`
   - `"weekdays"` Saturday: `return now.AddDate(0, 0, 3)` → `return libtime.ToDate(now.AddDate(0, 0, 3))`
   - `"weekdays"` Sunday: `return now.AddDate(0, 0, 2)` → `return libtime.ToDate(now.AddDate(0, 0, 2))`
   - `"weekdays"` default: `return next` → `return libtime.ToDate(next)`
   - `default` case: `return now.AddDate(0, 0, 1)` → `return libtime.ToDate(now.AddDate(0, 0, 1))`

   In recurring completion logic:
   - `task.DeferDate = &newDeferDate` → `task.DeferDate = newDeferDate.Ptr()`
   - `task.PlannedDate.Before(newDeferDate)` — same pointer dereference as defer.go: `(*task.PlannedDate).Before(newDeferDate)`
   - `newDeferDate.Format("2006-01-02")` — no change needed (Date has Format)

5. In `pkg/ops/frontmatter.go`:

   In `FrontmatterGetOperation.Execute`:
   - `task.DeferDate.Format("2006-01-02")` — no change needed (Date has Format)

   In `FrontmatterSetOperation.Execute`:
   - `task.DeferDate = &t` where `t` is `time.Time` → `d := libtime.ToDate(t); task.DeferDate = d.Ptr()`

   In `FrontmatterClearOperation.Execute`:
   - `task.DeferDate = nil` — no change needed

6. In `pkg/ops/defer_test.go`:

   **Remove Truncate calls on actual side** (Date is already date-only, use `.Time()` instead):
   - All `writtenTask.DeferDate.Truncate(24 * time.Hour)` → `writtenTask.DeferDate.Time()`
   - All `writtenTask.PlannedDate.Truncate(24 * time.Hour)` → `writtenTask.PlannedDate.Time()`
   - The expected side (e.g. `libtimetest.ParseDateTime(...).Time().AddDate(...).Truncate(24 * time.Hour)`) produces `time.Time` at midnight — keep as-is, it matches `Date.Time()` output

   **Weekday comparison** in the "next Monday" test — use `libtime.Weekday`:
   - `Expect(writtenTask.DeferDate.Weekday()).To(Equal(time.Monday))` → `Expect(writtenTask.DeferDate.Weekday()).To(Equal(libtime.Weekday(time.Monday)))`

   **After comparison** in the "next Monday" test — wrap `time.Time` because `After` takes `HasTime`:
   - `writtenTask.DeferDate.After(libtimetest.ParseDateTime(...).Time())` → `writtenTask.DeferDate.After(libtime.ToDate(libtimetest.ParseDateTime(...).Time()))`

   **PlannedDate assignments** — convert from `time.Time` to `libtime.Date`:
   - All `task.PlannedDate = &plannedDate` (where `plannedDate` is `time.Time`) → `pd := libtime.ToDate(plannedDate); task.PlannedDate = pd.Ptr()`
   - There are two such assignments: in the "before target date" and "after target date" contexts

7. In `pkg/ops/complete_test.go`:

   **Weekday comparisons** — use `libtime.Weekday`:
   - All `Expect(writtenTask.DeferDate.Weekday()).To(Equal(time.Saturday))` → `Equal(libtime.Weekday(time.Saturday))`
   - Same for `time.Sunday` → `libtime.Weekday(time.Sunday)`

   **After comparison**:
   - `Expect(writtenTask.DeferDate.After(now)).To(BeTrue())` → `Expect(writtenTask.DeferDate.After(libtime.ToDate(now))).To(BeTrue())`

   **PlannedDate assignment**:
   - `task.PlannedDate = &oldPlannedDate` → `pd := libtime.ToDate(oldPlannedDate); task.PlannedDate = pd.Ptr()`

8. In `pkg/ops/frontmatter_test.go`:

   **DeferDate setup** — convert `time.Time` to `libtime.Date` in struct literals and assignments:
   - In get-test BeforeEach: `DeferDate: &deferDate` → `DeferDate: libtime.ToDate(deferDate).Ptr()`
   - In set-test: `task.DeferDate = &deferDate` → `task.DeferDate = libtime.ToDate(deferDate).Ptr()`
   - In clear-test BeforeEach: `DeferDate: &deferDate` → `DeferDate: libtime.ToDate(deferDate).Ptr()`
</requirements>

<constraints>
- Do NOT change any interfaces (`DeferOperation`, `Storage`, etc.)
- Do NOT change CLI layer (`pkg/cli/cli.go`) — it passes strings, not dates
- Do NOT change `pkg/storage/markdown.go` — YAML marshaling will work automatically via `Date.MarshalText`
- Do NOT change daily note logic — it receives formatted date strings
- Existing tests must still pass after type changes
- Use `import libtime "github.com/bborbe/time"` consistently
- Use `import libtimetest "github.com/bborbe/time/test"` in test files if needed
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
