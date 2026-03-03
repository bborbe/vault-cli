---
status: completed
summary: Fixed defer operation to not change task status, added planned_date clearing and past-date validation
container: vault-cli-036-fix-defer-no-status-change
dark-factory-version: v0.14.5
created: "2026-03-03T22:51:01Z"
queued: "2026-03-03T22:51:01Z"
started: "2026-03-03T22:51:01Z"
completed: "2026-03-03T22:55:25Z"
---
<objective>
Fix defer operation to NOT change task status. Defer should only set defer_date and update daily notes. Also add planned_date clearing and past-date validation. Align with how /defer-task slash command works.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding-guidelines/go-testing-guide.md for testing patterns.
Read pkg/ops/defer.go — the file to modify.
Read pkg/ops/defer_test.go — tests to update.
Read pkg/domain/task.go — Task struct with DeferDate and PlannedDate fields.
</context>

<requirements>
1. In `pkg/ops/defer.go` method `findAndDeferTask`:
   - REMOVE the line `task.Status = domain.TaskStatusDeferred`
   - Task status stays unchanged (whatever it was before defer)

2. Add planned_date clearing in `findAndDeferTask`:
   - After setting `task.DeferDate = &targetDate`
   - If `task.PlannedDate != nil && task.PlannedDate.Before(targetDate)`:
     - Set `task.PlannedDate = nil`

3. Add past-date validation in `Execute` method:
   - After `d.parseDate(dateStr)` succeeds
   - Compare `targetDate` with `time.Now()` (truncated to day)
   - If `targetDate` is before today → return error "cannot defer to past date: YYYY-MM-DD"
   - Allow same day (today) — only reject strictly before today

4. Update `pkg/ops/defer_test.go`:
   - Remove test assertion that checks `status == deferred`
   - Replace with assertion: status unchanged from original (stays whatever was set in BeforeEach)
   - Add test: task with planned_date before target → planned_date cleared
   - Add test: task with planned_date after target → planned_date preserved
   - Add test: task with no planned_date → no crash, works fine
   - Add test: defer to yesterday → returns error containing "cannot defer to past"
   - Add test: defer to today → succeeds (no error)
</requirements>

<constraints>
- Do NOT modify existing passing tests beyond what's needed for the behavior change
- Do NOT change the parseDate function — it works correctly
- Do NOT change daily note logic (removeFromDailyNote, addToDailyNote) — those are fine
- Use Ginkgo v2 + Gomega, follow existing test patterns in defer_test.go
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- Defer no longer changes task.Status
- PlannedDate cleared when before targetDate
- PlannedDate preserved when after targetDate
- Past date returns error
- Today's date succeeds
</verification>
