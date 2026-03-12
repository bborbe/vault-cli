---
status: completed
summary: Added `vault-cli task show <name>` command with ShowOperation interface, TaskDetail struct, JSON/plain output, multi-vault search, and tests.
container: vault-cli-049-b-task-show-command
dark-factory-version: v0.54.0
created: "2026-03-12T22:00:00Z"
queued: "2026-03-12T21:27:57Z"
started: "2026-03-12T21:32:00Z"
completed: "2026-03-12T21:35:55Z"
---

<summary>
- A new command shows full detail for a single task by name
- JSON output includes all frontmatter fields, description, content, and file modification time
- Plain output shows a human-readable summary
- The command searches across vaults if no vault is specified
- Non-existent task returns a clear error with non-zero exit code
</summary>

<objective>
Add `vault-cli task show <name>` command that returns complete task detail including content and metadata, enabling external tools to fetch full task information without reading files directly.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/domain/task.go` — find `Task` struct with all fields.
Read `pkg/cli/cli.go` — find `createTaskGetCommand` as a pattern for single-task commands that search across vaults.
Read `pkg/storage/markdown.go` — find `FindTaskByName` which resolves a task name to a `*domain.Task`.
Read `pkg/ops/frontmatter.go` — find the existing `DeferDate.Format("2006-01-02")` pattern (~line 58) for date conversion.
</context>

<requirements>
1. Create `pkg/ops/show.go` with a `ShowOperation` interface and implementation:

```go
type ShowOperation interface {
    Execute(ctx context.Context, vaultPath string, vaultName string, taskName string, outputFormat string) error
}
```

2. Create a `TaskDetail` struct for JSON output with all fields:

```go
type TaskDetail struct {
    Name            string   `json:"name"`
    Status          string   `json:"status"`
    Phase           string   `json:"phase,omitempty"`
    Assignee        string   `json:"assignee,omitempty"`
    Priority        int      `json:"priority,omitempty"` // domain.Priority is `type Priority int`
    Category        string   `json:"category,omitempty"` // from task.PageType
    Recurring       string   `json:"recurring,omitempty"`
    DeferDate       string   `json:"defer_date,omitempty"`
    PlannedDate     string   `json:"planned_date,omitempty"`
    ClaudeSessionID string   `json:"claude_session_id,omitempty"`
    Goals           []string `json:"goals,omitempty"`
    Description     string   `json:"description,omitempty"`
    Content         string   `json:"content"`
    ModifiedDate    string   `json:"modified_date,omitempty"`
    FilePath        string   `json:"file_path"`
    Vault           string   `json:"vault"`
}
```

Note: `domain.Priority` has base type `int`, so cast with `int(task.Priority)`.
`DueDate` and `BlockedBy` do not exist in `domain.Task` — do not include them.

3. The `Execute` method finds the task via `storage.FindTaskByName`, populates `TaskDetail` from the domain task:
   - `Description`: first 200 chars of content after frontmatter, stripped of markdown formatting
   - `ModifiedDate`: file modification time formatted as ISO 8601
   - `Content`: full markdown content (including frontmatter)
   - `FilePath`: absolute path to the task file
   - Date fields: format `*libtime.Date` as `YYYY-MM-DD` string when non-nil
   - `Category`: from `task.PageType`
   - `Priority`: cast with `int(task.Priority)` — `domain.Priority` base type is `int`

4. For plain output, print a readable summary:
```
Task: <name>
Status: <status>
Assignee: <assignee>
Priority: <priority>
Phase: <phase>
```

5. In `pkg/cli/cli.go`, create `createTaskShowCommand` following the same multi-vault pattern as `createTaskGetCommand` (try each vault, return first match).

6. Register the command: `taskCmd.AddCommand(createTaskShowCommand(...))`.

7. Add tests in `pkg/ops/show_test.go` for JSON output with all fields populated and with optional fields omitted.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- All file paths are repo-relative
- `domain.Task` does NOT have `DueDate` or `BlockedBy` — do not include these fields
- File modification time requires `os.Stat` on the task file path — handle errors gracefully (omit field on error)
- `Content` field includes the full file content as-is (frontmatter + body)
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
