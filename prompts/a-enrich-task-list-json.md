---
status: created
created: "2026-03-12T22:00:00Z"
---

<summary>
- Task list JSON output includes additional fields needed by external tools
- New fields: category, recurring, defer date, planned date, claude session ID, phase
- Existing fields and plain-text output remain unchanged
- Fields that are empty or unset are omitted from JSON output via omitempty
- Tests verify new fields appear correctly in JSON output
</summary>

<objective>
Enrich the task list JSON output with fields that task-orchestrator needs to render the Kanban board, so external tools can use vault-cli as the single source of truth for task data.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/ops/list.go` — find `TaskListItem` struct and the JSON serialization loop in `Execute`.
Read `pkg/domain/task.go` — find `Task` struct with all available frontmatter fields.
Read `pkg/ops/frontmatter.go` — find the existing `DeferDate.Format("2006-01-02")` pattern (~line 58) for date conversion.
Read `pkg/ops/list_test.go` — note existing tests use `outputFormat: "plain"`; you will need to add a JSON-output test context.
</context>

<requirements>
1. Add the following fields to `TaskListItem` in `pkg/ops/list.go`:

```go
type TaskListItem struct {
    Name            string `json:"name"`
    Status          string `json:"status"`
    Assignee        string `json:"assignee,omitempty"`
    Priority        int    `json:"priority,omitempty"`
    Vault           string `json:"vault"`
    Category        string `json:"category,omitempty"`
    Recurring       string `json:"recurring,omitempty"`
    DeferDate       string `json:"defer_date,omitempty"`
    PlannedDate     string `json:"planned_date,omitempty"`
    ClaudeSessionID string `json:"claude_session_id,omitempty"`
    Phase           string `json:"phase,omitempty"`
}
```

2. In the JSON serialization loop in `Execute`, populate the new fields from the `domain.Task`:
   - `Category` from `task.PageType` (vault-cli uses `page_type` for category)
   - `Recurring` from `task.Recurring`
   - `DeferDate` from `task.DeferDate` — nil-guard then format:
     ```go
     if task.DeferDate != nil {
         items[i].DeferDate = task.DeferDate.Format("2006-01-02")
     }
     ```
   - `PlannedDate` from `task.PlannedDate` — same nil-guard pattern as DeferDate
   - `ClaudeSessionID` from `task.ClaudeSessionID`
   - `Phase` from `task.Phase`

3. Update tests in `pkg/ops/list_test.go` to verify the new fields appear in JSON output.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- All file paths are repo-relative
- `omitempty` on all new fields — empty values must not appear in JSON output
- `DeferDate` and `PlannedDate` are `*libtime.Date` in the domain model — convert to string with `.Format("2006-01-02")` only when non-nil
- Do NOT change the `domain.Task` struct — only change the JSON serialization layer
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
