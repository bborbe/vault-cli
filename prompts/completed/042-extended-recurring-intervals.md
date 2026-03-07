---
status: completed
summary: Added RecurringInterval type with ParseRecurringInterval supporting named aliases (quarterly, yearly) and numeric shorthand (3d, 2w, etc.), updated calculateNextDeferDate to use it, and added corresponding tests in both domain and ops packages.
container: vault-cli-042-extended-recurring-intervals
dark-factory-version: v0.26.0
created: "2026-03-07T23:03:38Z"
queued: "2026-03-07T23:03:38Z"
started: "2026-03-07T23:03:45Z"
completed: "2026-03-07T23:09:24Z"
---
<summary>
- Recurring tasks support numeric shorthand intervals (e.g. `3d`, `2w`, `2m`, `1q`, `2y`)
- New named aliases `quarterly` and `yearly` added alongside existing daily/weekly/monthly
- Parsing centralised in `RecurringInterval` type with unit tests
- Existing daily/weekly/monthly/weekdays behavior unchanged
- Invalid recurring values fall back to daily with warning (same as today)
</summary>

<objective>
Recurring tasks support a richer interval syntax — named aliases like `quarterly` and `yearly`, plus numeric shorthand like `3d` or `2w`. The parsing is centralised in a typed domain value so future interval formats can be added in one place.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/ops/complete.go — `calculateNextDeferDate` function (lines 286-313).
Read pkg/ops/complete_test.go — existing recurring task tests (follow this pattern).
Read pkg/domain/task.go — Task struct with `Recurring string` field (line 25).
</context>

<requirements>
1. Create `pkg/domain/recurring_interval.go` with a `RecurringInterval` type:

   ```go
   type RecurringInterval struct {
       Years  int
       Months int
       Days   int
   }

   func (r RecurringInterval) AddTo(t time.Time) time.Time {
       return t.AddDate(r.Years, r.Months, r.Days)
   }
   ```

2. Add `ParseRecurringInterval(s string) (RecurringInterval, error)` in the same file:
   - Named aliases:
     - `daily` → `{0, 0, 1}`
     - `weekly` → `{0, 0, 7}`
     - `monthly` → `{0, 1, 0}`
     - `quarterly` → `{0, 3, 0}`
     - `yearly` → `{1, 0, 0}`
   - Numeric shorthand `<N><unit>` parsed via regex `^([1-9]\d*)([dwmqy])$` (N must be >= 1):
     - `d` → `{0, 0, N}`
     - `w` → `{0, 0, N*7}`
     - `m` → `{0, N, 0}`
     - `q` → `{0, N*3, 0}`
     - `y` → `{N, 0, 0}`
   - `weekdays` is NOT handled here — it is checked before calling this function (see requirement 4)
   - Unknown or empty input → return error

3. Create `pkg/domain/recurring_interval_test.go` with table-driven tests (Ginkgo v2 + Gomega):
   - All named aliases parse correctly
   - Numeric: `1d`, `3d`, `2w`, `2m`, `1q`, `2q`, `1y`, `2y`
   - `AddTo` correctness: `2m` from Jan 31 → Mar 31, `1m` from Jan 31 → Feb 28 (Go's AddDate behavior)
   - Invalid input returns error: `""`, `"foo"`, `"0d"`, `"weekdays"`

4. Update `calculateNextDeferDate` in `pkg/ops/complete.go`:
   - Keep `weekdays` as the FIRST check — before calling `ParseRecurringInterval`. The weekend-skipping logic stays inline exactly as-is.
   - For all other values, call `domain.ParseRecurringInterval(recurring)`
   - If parse succeeds, return `libtime.ToDate(interval.AddTo(now))`
   - If parse fails, log warning to stderr and fall back to daily (preserve current behavior)
   - Remove the hardcoded `daily`, `weekly`, `monthly` switch cases (they are now handled by `ParseRecurringInterval`)

5. Update existing tests in `pkg/ops/complete_test.go`:
   - Existing daily/weekly/monthly/weekdays tests must still pass (no behavior change)
   - Add test: `recurring: "3d"` → defer_date = now + 3 days
   - Add test: `recurring: "quarterly"` → defer_date = now + 3 months
   - Add test: `recurring: "2w"` → defer_date = now + 14 days
   - Add test: `recurring: "yearly"` → defer_date = now + 1 year
</requirements>

<constraints>
- Do NOT change the `Task.Recurring` field type (stays `string`)
- Do NOT change the `handleRecurringTask` method signature: `func (c *completeOperation) handleRecurringTask(ctx context.Context, task *domain.Task, vaultPath string, vaultName string, outputFormat string, warnings []string) error`
- Do NOT modify weekdays logic — keep it as a special case in `calculateNextDeferDate`, checked BEFORE `ParseRecurringInterval`
- `weekdays` is NOT a valid `RecurringInterval` — it must return an error from `ParseRecurringInterval`
- Numeric shorthand requires N >= 1 (reject `0d`, `0w`, etc.)
- Use Ginkgo v2 + Gomega for tests, follow existing patterns in `complete_test.go`
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- `recurring: "3d"` → defer_date = today + 3 days
- `recurring: "quarterly"` → defer_date = today + 3 months
- `recurring: "2w"` → defer_date = today + 14 days
- `recurring: "yearly"` → defer_date = today + 1 year
- `recurring: "weekdays"` → still skips weekends correctly
- All existing recurring tests still pass
- No lint errors
</verification>
