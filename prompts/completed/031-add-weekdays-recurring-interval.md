---
status: completed
summary: Added weekdays recurring interval to complete operation with weekend-skipping logic
container: vault-cli-031-add-weekdays-recurring-interval
dark-factory-version: v0.14.5
created: "2026-03-03T22:24:13Z"
queued: "2026-03-03T22:24:13Z"
started: "2026-03-03T22:24:13Z"
completed: "2026-03-03T22:28:41Z"
---
<objective>
Add `weekdays` recurring interval to complete operation. When a recurring task has `recurring: weekdays`, the next defer_date should skip weekends (Saturday/Sunday → next Monday).
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding-guidelines/go-testing-guide.md for testing patterns.
Read pkg/ops/complete.go — handleRecurringTask method, the switch on task.Recurring.
Read pkg/ops/complete_test.go — existing recurring task tests.
</context>

<requirements>
1. In `pkg/ops/complete.go` `handleRecurringTask`, add a `case "weekdays":` to the switch:
   ```go
   case "weekdays":
       next := now.AddDate(0, 0, 1) // tomorrow
       switch next.Weekday() {
       case time.Saturday:
           newDeferDate = now.AddDate(0, 0, 3) // Saturday → Monday
       case time.Sunday:
           newDeferDate = now.AddDate(0, 0, 2) // Sunday → Monday
       default:
           newDeferDate = next
       }
   ```

2. Update the default case to NOT silently treat unknown intervals as daily. Instead:
   - Log a warning: `fmt.Fprintf(os.Stderr, "Warning: unknown recurring interval %q, treating as daily\n", task.Recurring)`
   - Then set `newDeferDate = now.AddDate(0, 0, 1)`

3. Add tests in `pkg/ops/complete_test.go`:
   - Test: recurring=weekdays, today=Monday → next=Tuesday
   - Test: recurring=weekdays, today=Friday → next=Monday (+3 days)
   - Test: recurring=weekdays, today=Saturday → next=Monday (+2 days)
   - Test: recurring=weekdays, today=Sunday → next=Monday (+1 day)

   Note: These tests need to control "now". If the existing code uses `time.Now()` directly, you may need to either:
   a. Accept that the test can only verify the defer_date is set (not exact value), OR
   b. Introduce a `clock` interface or `nowFunc` field on completeOperation for testability

   Prefer option (a) for simplicity — just verify `task.DeferDate != nil` and that it's a weekday (Monday-Friday) and after today.
</requirements>

<constraints>
- Do NOT change existing daily/weekly/monthly behavior
- Do NOT refactor time.Now() usage unless absolutely needed for testing
- Keep the weekday calculation logic simple and inline
- Use Ginkgo v2 + Gomega, follow existing test patterns
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- Recurring task with `recurring: weekdays` → defer_date is always a weekday
- Existing daily/weekly/monthly tests still pass
- Unknown recurring interval logs warning
</verification>
