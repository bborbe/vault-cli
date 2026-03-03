---
status: completed
summary: Added recurring task support to vault-cli complete command with checkbox reset, defer_date bumping, and comprehensive test coverage
container: vault-cli-029-recurring-task-complete
dark-factory-version: v0.13.2
created: "2026-03-03T16:36:18Z"
queued: "2026-03-03T16:36:18Z"
started: "2026-03-03T16:36:18Z"
completed: "2026-03-03T16:39:39Z"
---
<objective>
Add recurring task support to the complete command. Currently `vault-cli task complete` sets status=done for all tasks. For recurring tasks (frontmatter `recurring: daily|weekly|monthly`), it should reset checkboxes, bump defer_date, update last_completed, and keep status=in_progress.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/domain/task.go — Task struct (needs new fields).
Read pkg/ops/complete.go — Execute method (needs recurring branch).
Read pkg/ops/complete_test.go — existing tests (follow this pattern).
</context>

<requirements>
1. Add fields to `Task` struct in `pkg/domain/task.go`:
   - `Recurring string \`yaml:"recurring,omitempty"\`` — values: "daily", "weekly", "monthly", or empty
   - `LastCompleted string \`yaml:"last_completed,omitempty"\`` — ISO date string (YYYY-MM-DD)
   - `PlannedDate *time.Time \`yaml:"planned_date,omitempty"\`` — if present

2. In `pkg/ops/complete.go` Execute method, after finding the task, add a branch:

   ```
   if task.Recurring != "" {
       // Recurring task: don't mark done
       // 1. Reset all checkboxes in content: "- [x]" → "- [ ]"
       // 2. Set last_completed to today (YYYY-MM-DD)
       // 3. Bump defer_date based on recurring interval:
       //    - "daily": tomorrow
       //    - "weekly": +7 days
       //    - "monthly": +1 month
       // 4. If planned_date exists and < new defer_date, clear planned_date
       // 5. Keep status as-is (do NOT set to done)
       // Write task, update daily note, print message, return
   } else {
       // Existing logic: set status=done
   }
   ```

3. The checkbox reset must operate on `task.Content` — replace all `- [x]` with `- [ ]` in the body (after frontmatter).

4. The output message for recurring tasks should be:
   - plain: `🔄 Recurring task reset: <name> (next: <defer_date>)`
   - json: `{"success": true, "name": "...", "recurring": true, "next_date": "..."}`

5. Add tests in `pkg/ops/complete_test.go`:
   - Recurring daily task: checkboxes reset, defer_date = tomorrow, last_completed = today, status unchanged
   - Recurring weekly task: defer_date = +7 days
   - Recurring monthly task: defer_date = +1 month
   - Non-recurring task: existing behavior unchanged (status=done)
   - Recurring task with planned_date < new defer_date: planned_date cleared
</requirements>

<constraints>
- Do NOT change existing non-recurring completion behavior
- Do NOT modify the Storage interface
- Do NOT break existing tests
- Use time.Now() for "today" — the complete operation already imports time
- Checkbox reset: only reset in the body content, not in frontmatter
</constraints>

<verification>
Run: `make test`
Confirm: all tests pass including the new recurring task tests.
</verification>

<success_criteria>
`vault-cli task complete "Cleanup OmniFocus Inbox"` on a task with `recurring: daily`:
- Resets all `- [x]` to `- [ ]` in the task body
- Sets `last_completed` to today
- Bumps `defer_date` to tomorrow
- Keeps `status: in_progress`
- Prints `🔄 Recurring task reset: Cleanup OmniFocus Inbox (next: 2026-03-04)`
</success_criteria>
