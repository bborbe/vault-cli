---
status: created
---

<summary>
- All domain types (Task, Goal, Objective, Theme, Vision) gain a ModifiedDate field populated from the file's mtime when loaded
- task list --output json includes modified_date for every task
- The existing file-stat pattern from the show operation is reused for consistency
- No frontmatter changes — ModifiedDate is metadata-only, not persisted
- All existing tests continue to pass
</summary>

<objective>
Add file modification time to all domain types so consumers (e.g. task-orchestrator) can determine how recently a file was changed. Expose modified_date in task list JSON output.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/domain/task.go — Task struct with FilePath field (yaml:"-").
Read pkg/domain/goal.go — Goal struct with FilePath field.
Read pkg/domain/objective.go — Objective struct with FilePath field.
Read pkg/domain/theme.go — Theme struct with FilePath field.
Read pkg/domain/vision.go — Vision struct with FilePath field.
Read pkg/storage/base.go — readTaskFromPath function that sets FilePath.
Read pkg/storage/goal.go — readGoalFromPath function that sets FilePath.
Read pkg/storage/objective.go — readObjectiveFromPath function that sets FilePath.
Read pkg/storage/theme.go — readThemeFromPath function that sets FilePath (if exists).
Read pkg/storage/vision.go — readVisionFromPath function that sets FilePath (if exists).
Read pkg/ops/list.go — TaskListItem struct and Execute method for JSON output.
Read pkg/ops/show.go — already uses os.Stat(task.FilePath).ModTime() at ~line 115 — use the same pattern.
</context>

<requirements>
1. In pkg/domain/task.go, add field to Task struct after FilePath:
   ```go
   ModifiedDate *time.Time `yaml:"-"` // File modification time, populated by storage layer
   ```
   Add import "time" if not present.

2. Repeat step 1 for pkg/domain/goal.go (Goal struct), pkg/domain/objective.go (Objective struct), pkg/domain/theme.go (Theme struct), pkg/domain/vision.go (Vision struct).

3. In pkg/storage/base.go, in readTaskFromPath, after setting FilePath on the task struct, add:
   ```go
   if info, err := os.Stat(filePath); err == nil {
       t := info.ModTime().UTC()
       task.ModifiedDate = &t
   }
   ```

4. In pkg/storage/goal.go, in readGoalFromPath, after setting FilePath on the goal struct, add the same os.Stat block setting goal.ModifiedDate.

5. In pkg/storage/objective.go, in readObjectiveFromPath, after setting FilePath, add the same os.Stat block setting objective.ModifiedDate.

6. In pkg/storage/theme.go, in readThemeFromPath, after setting FilePath on the theme struct, add the same os.Stat block setting theme.ModifiedDate.

7. In pkg/storage/vision.go, in readVisionFromPath, after setting FilePath on the vision struct, add the same os.Stat block setting vision.ModifiedDate.

8. In pkg/ops/list.go, add ModifiedDate field to TaskListItem:
   ```go
   ModifiedDate string `json:"modified_date,omitempty"`
   ```

9. In pkg/ops/list.go, in the Execute method's JSON output loop, populate ModifiedDate:
   ```go
   if task.ModifiedDate != nil {
       items[i].ModifiedDate = task.ModifiedDate.UTC().Format("2006-01-02T15:04:05Z")
   }
   ```

10. Add a test in pkg/ops/list_test.go that verifies modified_date appears in JSON output when outputFormat is "json". The test should create a task file on disk, call the list Execute method, capture stdout, unmarshal the JSON, and assert the first item has a non-empty modified_date field.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT add modified_date to frontmatter — it is metadata only (yaml:"-")
- Do NOT change the plain text output of task list — only JSON output
- Existing tests must still pass
- Follow the existing os.Stat pattern from pkg/ops/show.go
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
