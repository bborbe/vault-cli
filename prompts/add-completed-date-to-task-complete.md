---
status: created
---

<summary>
- task complete sets a completed_date frontmatter field to the current UTC datetime (ISO 8601)
- completed_date is exposed in task list --output json and task show --output json
- Recurring task completion does NOT set completed_date (it uses last_completed instead)
- The field is optional — existing tasks without it work unchanged
- All existing tests continue to pass
</summary>

<objective>
When a non-recurring task is marked complete, record the completion date in frontmatter as completed_date. Expose this date in task list and task show JSON output so consumers can determine exactly when a task was completed.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/domain/task.go — Task struct with existing LastCompleted field pattern.
Read pkg/ops/complete.go — Execute method; non-recurring path sets task.Status = domain.TaskStatusCompleted before calling taskStorage.WriteTask.
Read pkg/ops/list.go — TaskListItem struct and JSON output loop.
Read pkg/ops/show.go — TaskDetail struct with existing fields for JSON output.
</context>

<requirements>
1. In pkg/domain/task.go, add field to Task struct after LastCompleted:
   ```go
   CompletedDate string `yaml:"completed_date,omitempty"`
   ```

2. In pkg/ops/complete.go, in the Execute method, on the non-recurring path, after setting task.Status = domain.TaskStatusCompleted and before calling taskStorage.WriteTask, add:
   ```go
   task.CompletedDate = c.currentDateTime.Now().Time().UTC().Format("2006-01-02T15:04:05Z")
   ```

3. In pkg/ops/list.go, add CompletedDate field to TaskListItem:
   ```go
   CompletedDate string `json:"completed_date,omitempty"`
   ```
   In the JSON output loop, populate it:
   ```go
   items[i].CompletedDate = task.CompletedDate
   ```

4. In pkg/ops/show.go, add CompletedDate field to TaskDetail struct:
   ```go
   CompletedDate string `json:"completed_date,omitempty"`
   ```
   In the Execute method where TaskDetail is populated, add:
   ```go
   detail.CompletedDate = task.CompletedDate
   ```

5. Add tests in pkg/ops/complete_test.go:
   - Verify that after Execute on a non-recurring task, the task written to storage has CompletedDate set to a non-empty ISO 8601 datetime string (format "2006-01-02T15:04:05Z")
   - Verify that after Execute on a recurring task (task.Recurring != ""), CompletedDate is NOT set

6. Add a test in pkg/ops/list_test.go verifying that a completed task with CompletedDate set appears with completed_date in the JSON output.

7. Add a test in pkg/ops/show_test.go verifying that a task with CompletedDate set appears with completed_date in the JSON output.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT set CompletedDate in handleRecurringTask — recurring tasks use LastCompleted instead
- Do NOT change the plain text output of task list or task complete
- Existing tests must still pass
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
