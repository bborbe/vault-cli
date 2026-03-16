---
status: completed
summary: Added due_date field to Task struct, frontmatter get/set/clear operations, list JSON output, show JSON output, and all corresponding tests mirroring the planned_date implementation.
container: vault-cli-062-add-due-date-field
dark-factory-version: v0.57.3
created: "2026-03-16T12:55:00Z"
queued: "2026-03-16T14:17:04Z"
started: "2026-03-16T14:17:17Z"
completed: "2026-03-16T14:24:52Z"
---

<summary>
- Tasks can have a due_date field in YAML frontmatter (type: *libtime.Date, yaml tag: due_date)
- The `frontmatter get due_date` command returns the due date in YYYY-MM-DD format
- The `frontmatter set due_date` command accepts YYYY-MM-DD and persists the value
- The `frontmatter clear due_date` command removes the due date from a task
- The `list --format json` output includes due_date when present
- The `show --format json` output includes due_date when present
- All existing tests continue to pass unchanged
- New tests cover get/set/clear/nil for due_date following the planned_date test pattern
</summary>

<objective>
Add `due_date` field support to vault-cli, mirroring the existing `planned_date` implementation exactly. Every place `PlannedDate` appears in the codebase, add a corresponding `DueDate` entry following the same pattern.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):
- `pkg/domain/task.go` — Task struct definition
- `pkg/ops/frontmatter.go` — get/set/clear switch statements
- `pkg/ops/list.go` — TaskListItem struct and JSON mapping
- `pkg/ops/show.go` — TaskDetail struct and JSON mapping
- `pkg/ops/frontmatter_test.go` — test pattern to follow
- `pkg/ops/show_test.go` — test pattern to follow
- `pkg/ops/list_test.go` — test pattern to follow
</context>

<requirements>
1. **Domain struct** (`pkg/domain/task.go`):
   Add a new field to the `Task` struct, placed immediately after `PlannedDate`:
   ```go
   DueDate     *libtime.Date `yaml:"due_date,omitempty"`
   ```

2. **Frontmatter get** (`pkg/ops/frontmatter.go`):
   In the `Execute` method of `frontmatterGetOperation`, add a new case in the switch statement right after the `"planned_date"` case:
   ```go
   case "due_date":
       if task.DueDate != nil {
           return task.DueDate.Format("2006-01-02"), nil
       }
       return "", nil
   ```

3. **Frontmatter set** (`pkg/ops/frontmatter.go`):
   In the `Execute` method of `frontmatterSetOperation`, add a new case in the switch statement right after the `"planned_date"` case:
   ```go
   case "due_date":
       d, err := parseDatePtr(ctx, value)
       if err != nil {
           return err
       }
       task.DueDate = d
   ```

4. **Frontmatter clear** (`pkg/ops/frontmatter.go`):
   In the `Execute` method of `frontmatterClearOperation`, add a new case in the switch statement right after the `"planned_date"` case:
   ```go
   case "due_date":
       task.DueDate = nil
   ```

5. **List output** (`pkg/ops/list.go`):
   5a. Add field to `TaskListItem` struct right after `PlannedDate`:
   ```go
   DueDate     string `json:"due_date,omitempty"`
   ```
   5b. In the `Execute` method of `listOperation`, inside the JSON output block (the `for i, task := range filteredTasks` loop), add right after the `PlannedDate` nil-check block:
   ```go
   if task.DueDate != nil {
       items[i].DueDate = task.DueDate.Format("2006-01-02")
   }
   ```

6. **Show output** (`pkg/ops/show.go`):
   6a. Add field to `TaskDetail` struct right after `PlannedDate`:
   ```go
   DueDate     string `json:"due_date,omitempty"`
   ```
   6b. In the `Execute` method of `showOperation`, add right after the `PlannedDate` nil-check block:
   ```go
   if task.DueDate != nil {
       detail.DueDate = task.DueDate.Format("2006-01-02")
   }
   ```

7. **Frontmatter tests** (`pkg/ops/frontmatter_test.go`):
   7a. In the `FrontmatterGetOperation` Describe block's `BeforeEach`, add to the task initialization (after `PlannedDate` line):
   ```go
   DueDate: libtime.ToDate(time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)).Ptr(),
   ```
   7b. Add two new test Contexts in `FrontmatterGetOperation` (after the `planned_date when nil` context):
   - `"getting due_date field"` — sets `key = "due_date"`, expects `"2025-06-30"`
   - `"getting due_date when nil"` — sets `key = "due_date"` and `task.DueDate = nil`, expects `""`

   7c. In the `FrontmatterSetOperation` Describe block, add three new test Contexts (after the `invalid planned_date format` context):
   - `"setting due_date field"` — sets `key = "due_date"`, `value = "2025-06-15"`, expects `writtenTask.DueDate` not nil and formatted as `"2025-06-15"`
   - `"clearing due_date with empty string"` — sets `key = "due_date"`, `value = ""`, pre-sets `task.DueDate` to a date, expects `writtenTask.DueDate` to be nil
   - `"invalid due_date format"` — sets `key = "due_date"`, `value = "2025-13-45"`, expects error containing `"invalid date format"`

   7d. In the `FrontmatterClearOperation` Describe block's `BeforeEach`, add to the task initialization (after `PlannedDate` line):
   ```go
   DueDate: libtime.ToDate(time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)).Ptr(),
   ```
   7e. Add a new test Context in `FrontmatterClearOperation` (after the `clearing planned_date field` context):
   - `"clearing due_date field"` — sets `key = "due_date"`, expects `writtenTask.DueDate` to be nil

   Follow the exact same Ginkgo v2 `Context/BeforeEach/It` pattern used by the existing `planned_date` tests.

8. **Show tests** (`pkg/ops/show_test.go`):
   In the `BeforeEach` that initializes the task, add `DueDate` field after `PlannedDate`:
   ```go
   DueDate: libtime.ToDate(time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)).Ptr(),
   ```
   If there are existing JSON output assertions that check for `planned_date`, add matching assertions for `due_date`.

9. **List tests** (`pkg/ops/list_test.go`):
   If any test tasks set `PlannedDate`, add corresponding `DueDate` fields. If there are JSON output assertions, add `due_date` assertions where appropriate.
</requirements>

<constraints>
- Do NOT commit -- dark-factory handles git
- Existing tests must still pass
- Use `*libtime.Date` type (import: `libtime "github.com/bborbe/time"`) -- same as PlannedDate
- YAML tag must be `due_date` (snake_case with omitempty)
- JSON tag must be `due_date` (snake_case with omitempty)
- Date format is always `"2006-01-02"` (Go reference time for YYYY-MM-DD)
- Place `DueDate` immediately after `PlannedDate` in every struct and switch statement for consistent ordering
- Do NOT modify any existing PlannedDate behavior
- Follow Ginkgo v2 / Gomega test patterns exactly as used in existing tests
</constraints>

<verification>
Run `make precommit` -- must pass with zero failures.

Additionally verify:
1. `grep -r "DueDate" pkg/` returns hits in domain/task.go, ops/frontmatter.go, ops/list.go, ops/show.go, and their test files
2. `grep -r "due_date" pkg/` returns hits in the same files (yaml/json tags)
3. No existing PlannedDate references were removed or modified (only new DueDate lines added)
</verification>
