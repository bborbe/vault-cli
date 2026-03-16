---
status: completed
summary: Added planned_date, recurring, last_completed, page_type, goals, and tags fields to frontmatter get/set/clear operations with full test coverage, extracting helper functions to keep cognitive complexity within linter limits.
container: vault-cli-061-add-missing-frontmatter-fields
dark-factory-version: v0.55.1
created: "2026-03-16T13:00:00Z"
queued: "2026-03-16T13:34:12Z"
started: "2026-03-16T13:34:23Z"
completed: "2026-03-16T13:39:53Z"
---

<summary>
- Frontmatter get/set/clear commands support planned_date, recurring, last_completed, page_type, goals, and tags fields
- planned_date works identically to defer_date (pointer to libtime.Date, YYYY-MM-DD format)
- goals and tags accept comma-separated strings on set, return comma-joined output on get, and clear to nil slice
- recurring, last_completed, and page_type use simple string get/set/clear
- All new fields have full test coverage in the existing test file
</summary>

<objective>
All Task domain fields are accessible through the frontmatter get/set/clear CLI commands, with full test coverage for each new field.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes:
- `pkg/ops/frontmatter.go` — the three switch statements to extend
- `pkg/domain/task.go` — the Task struct with all field types
- `pkg/ops/frontmatter_test.go` — existing Ginkgo test patterns to follow
</context>

<requirements>
1. Add `"strings"` to the import block in `pkg/ops/frontmatter.go` (needed for comma-separated parsing of goals/tags).

2. In the `frontmatterGetOperation.Execute` method, add these cases to the switch statement (before the `default` case):

   ```go
   case "planned_date":
       if task.PlannedDate != nil {
           return task.PlannedDate.Format("2006-01-02"), nil
       }
       return "", nil
   case "recurring":
       return task.Recurring, nil
   case "last_completed":
       return task.LastCompleted, nil
   case "page_type":
       return task.PageType, nil
   case "goals":
       return strings.Join(task.Goals, ","), nil
   case "tags":
       return strings.Join(task.Tags, ","), nil
   ```

3. In the `frontmatterSetOperation.Execute` method, add these cases to the switch statement (before the `default` case):

   ```go
   case "planned_date":
       if value == "" {
           task.PlannedDate = nil
       } else {
           t, err := time.Parse("2006-01-02", value)
           if err != nil {
               return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
           }
           d := libtime.ToDate(t)
           task.PlannedDate = d.Ptr()
       }
   case "recurring":
       task.Recurring = value
   case "last_completed":
       task.LastCompleted = value
   case "page_type":
       task.PageType = value
   case "goals":
       if value == "" {
           task.Goals = nil
       } else {
           task.Goals = strings.Split(value, ",")
       }
   case "tags":
       if value == "" {
           task.Tags = nil
       } else {
           task.Tags = strings.Split(value, ",")
       }
   ```

4. In the `frontmatterClearOperation.Execute` method, add these cases to the switch statement (before the `default` case):

   ```go
   case "planned_date":
       task.PlannedDate = nil
   case "recurring":
       task.Recurring = ""
   case "last_completed":
       task.LastCompleted = ""
   case "page_type":
       task.PageType = ""
   case "goals":
       task.Goals = nil
   case "tags":
       task.Tags = nil
   ```

5. Update the test file `pkg/ops/frontmatter_test.go`. Add the new fields to the default task in the `FrontmatterGetOperation` `BeforeEach` block:

   ```go
   plannedDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
   task = &domain.Task{
       Name:            taskName,
       Phase:           "implementation",
       ClaudeSessionID: "session-123",
       Assignee:        "alice",
       Status:          domain.TaskStatusInProgress,
       Priority:        domain.Priority(3),
       DeferDate:       libtime.ToDate(deferDate).Ptr(),
       PlannedDate:     libtime.ToDate(plannedDate).Ptr(),
       Recurring:       "weekly",
       LastCompleted:   "2025-03-10",
       PageType:        "task",
       Goals:           []string{"goal-1", "goal-2"},
       Tags:            []string{"urgent", "backend"},
   }
   ```

6. Add GET test contexts in the `FrontmatterGetOperation` Describe block, following the exact same pattern as the existing "getting defer_date field" context:

   - "getting planned_date field" — expects `"2025-03-15"`
   - "getting planned_date when nil" — set `task.PlannedDate = nil`, expects `""`
   - "getting recurring field" — expects `"weekly"`
   - "getting last_completed field" — expects `"2025-03-10"`
   - "getting page_type field" — expects `"task"`
   - "getting goals field" — expects `"goal-1,goal-2"`
   - "getting goals when empty" — set `task.Goals = nil`, expects `""`
   - "getting tags field" — expects `"urgent,backend"`
   - "getting tags when empty" — set `task.Tags = nil`, expects `""`

7. Add SET test contexts in the `FrontmatterSetOperation` Describe block, following the same pattern as "setting defer_date field":

   - "setting planned_date field" — value `"2025-06-15"`, verify `writtenTask.PlannedDate` is not nil and formats to `"2025-06-15"`
   - "clearing planned_date with empty string" — value `""`, set task.PlannedDate to a date first, verify `writtenTask.PlannedDate` is nil
   - "invalid planned_date format" — value `"2025-13-45"`, expect error containing `"invalid date format"`
   - "setting recurring field" — value `"monthly"`, verify `writtenTask.Recurring` equals `"monthly"`
   - "setting last_completed field" — value `"2025-03-15"`, verify `writtenTask.LastCompleted` equals `"2025-03-15"`
   - "setting page_type field" — value `"task"`, verify `writtenTask.PageType` equals `"task"`
   - "setting goals field" — value `"goal-a,goal-b"`, verify `writtenTask.Goals` equals `[]string{"goal-a", "goal-b"}`
   - "clearing goals with empty string" — value `""`, set `task.Goals = []string{"old"}` first, verify `writtenTask.Goals` is nil
   - "setting tags field" — value `"tag-a,tag-b"`, verify `writtenTask.Tags` equals `[]string{"tag-a", "tag-b"}`
   - "clearing tags with empty string" — value `""`, set `task.Tags = []string{"old"}` first, verify `writtenTask.Tags` is nil

8. Add CLEAR test contexts in the `FrontmatterClearOperation` Describe block. First, update the default task in the `BeforeEach` block to include the new fields (same as step 5). Then add contexts following the same pattern as "clearing defer_date field":

   - "clearing planned_date field" — verify `writtenTask.PlannedDate` is nil
   - "clearing recurring field" — verify `writtenTask.Recurring` equals `""`
   - "clearing last_completed field" — verify `writtenTask.LastCompleted` equals `""`
   - "clearing page_type field" — verify `writtenTask.PageType` equals `""`
   - "clearing goals field" — verify `writtenTask.Goals` is nil
   - "clearing tags field" — verify `writtenTask.Tags` is nil
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass — do not modify existing test assertions, only add new ones
- Use `strings.Join` / `strings.Split` for slice fields (goals, tags) — no custom parsing
- For `planned_date`, follow the exact same pattern as `defer_date` (nil check on get, `time.Parse` + `libtime.ToDate` + `.Ptr()` on set, nil on clear)
- For `goals` and `tags` set: empty string produces nil slice, non-empty string splits on comma
- For `goals` and `tags` get: nil or empty slice produces empty string via `strings.Join`
- Do NOT change any files other than `pkg/ops/frontmatter.go` and `pkg/ops/frontmatter_test.go`
</constraints>

<verification>
Run `make precommit` — must pass.

Specifically verify:
- `go build ./...` compiles
- `go test ./pkg/ops/...` passes with all new test cases
- `go vet ./...` passes
</verification>
