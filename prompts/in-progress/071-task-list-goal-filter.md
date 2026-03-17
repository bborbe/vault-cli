---
status: approved
created: "2026-03-17T10:00:00Z"
queued: "2026-03-17T10:44:29Z"
---

<summary>
- Tasks can be filtered by goal name using a new --goal flag on task list
- Only tasks whose goals frontmatter list contains the exact goal name are shown
- The goal filter composes with existing --status and --assignee filters using AND logic
- Both JSON and plain output formats work correctly with the goal filter
- Empty result when no tasks match returns empty output, not an error
- Internal test infrastructure is updated to support the new parameter
</summary>

<objective>
Add a `--goal` flag to `vault-cli task list` that filters tasks by goal name (exact, case-sensitive match against the task's `goals` frontmatter list). The flag composes with existing `--status` and `--assignee` filters using AND logic.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/cli/cli.go` — `createTaskListCommand` function where `--status` and `--assignee` flags are defined. This is the pattern to follow for adding `--goal`.
- `pkg/ops/list.go` — `ListOperation` interface, `listOperation.Execute`, `filterTasks`, and `shouldIncludeTask` functions. The assignee filter pattern in `shouldIncludeTask` is the exact pattern to replicate for the goal filter.
- `pkg/domain/task.go` — `Task` struct already has `Goals []string` field.
- `pkg/ops/list_test.go` — existing tests for list operation. New tests for goal filtering go here.
- `mocks/list-operation.go` — counterfeiter-generated mock. Must be regenerated after changing the `ListOperation` interface.
</context>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must pass unchanged (update call sites to pass empty string for new goalFilter parameter)
- JSON and plain output both work with the filter
- Empty result (no matching tasks) returns empty output, not an error
- Use `github.com/bborbe/errors` for error wrapping
- Goal matching is exact and case-sensitive (not case-insensitive like status)
- Tests must use Ginkgo/Gomega with Counterfeiter mocks (follow existing patterns in `pkg/ops/list_test.go`)
</constraints>

<requirements>

## 1. `pkg/cli/cli.go` — Add --goal flag in createTaskListCommand

Add a `goalFilter` variable alongside the existing `assigneeFlag`, and pass it through to `listOp.Execute`.

```go
// Before (existing variables):
var statusFilter string
var showAll bool
var assigneeFlag string

// After:
var statusFilter string
var showAll bool
var assigneeFlag string
var goalFilter string
```

Update the `Execute` call to pass `goalFilter` as a new parameter after `assigneeFlag`:

```go
// Before:
if err := listOp.Execute(ctx, vault.Path, vault.Name, storageConfig.TasksDir, statusFilter, showAll, assigneeFlag, *outputFormat); err != nil {

// After:
if err := listOp.Execute(ctx, vault.Path, vault.Name, storageConfig.TasksDir, statusFilter, showAll, assigneeFlag, goalFilter, *outputFormat); err != nil {
```

Add the flag registration after the existing `assigneeFlag` registration:

```go
cmd.Flags().StringVar(&goalFilter, "goal", "", "Filter by goal name (exact match)")
```

Update the `Long` description to mention the goal filter:

```go
Long: `List tasks from the vault, optionally filtered by status, assignee, and goal.

By default, shows only tasks with status "todo" or "in_progress".
Use --status to filter by specific status, or --all to show all tasks.
Use --assignee to filter by assignee.
Use --goal to filter by goal name.`,
```

## 2. `pkg/ops/list.go` — Add goalFilter parameter throughout

### Update the `ListOperation` interface

```go
// Before:
type ListOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		pagesDir string,
		statusFilter string,
		showAll bool,
		assigneeFilter string,
		outputFormat string,
	) error
}

// After:
type ListOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		pagesDir string,
		statusFilter string,
		showAll bool,
		assigneeFilter string,
		goalFilter string,
		outputFormat string,
	) error
}
```

### Update `listOperation.Execute` method signature

Add `goalFilter string` parameter between `assigneeFilter` and `outputFormat`:

```go
// Before:
func (l *listOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	pagesDir string,
	statusFilter string,
	showAll bool,
	assigneeFilter string,
	outputFormat string,
) error {

// After:
func (l *listOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	pagesDir string,
	statusFilter string,
	showAll bool,
	assigneeFilter string,
	goalFilter string,
	outputFormat string,
) error {
```

Update the `filterTasks` call inside `Execute`:

```go
// Before:
filteredTasks := filterTasks(tasks, statusFilter, showAll, assigneeFilter)

// After:
filteredTasks := filterTasks(tasks, statusFilter, showAll, assigneeFilter, goalFilter)
```

### Update `filterTasks` function

```go
// Before:
func filterTasks(
	tasks []*domain.Task,
	statusFilter string,
	showAll bool,
	assigneeFilter string,
) []*domain.Task {
	filteredTasks := make([]*domain.Task, 0, len(tasks))
	for _, task := range tasks {
		if !shouldIncludeTask(task, statusFilter, showAll, assigneeFilter) {
			continue
		}
		filteredTasks = append(filteredTasks, task)
	}
	return filteredTasks
}

// After:
func filterTasks(
	tasks []*domain.Task,
	statusFilter string,
	showAll bool,
	assigneeFilter string,
	goalFilter string,
) []*domain.Task {
	filteredTasks := make([]*domain.Task, 0, len(tasks))
	for _, task := range tasks {
		if !shouldIncludeTask(task, statusFilter, showAll, assigneeFilter, goalFilter) {
			continue
		}
		filteredTasks = append(filteredTasks, task)
	}
	return filteredTasks
}
```

### Update `shouldIncludeTask` function

Add goal filtering after the existing assignee check, following the same pattern:

```go
// Before:
func shouldIncludeTask(
	task *domain.Task,
	statusFilter string,
	showAll bool,
	assigneeFilter string,
) bool {
	// Filter by assignee if specified
	if assigneeFilter != "" && task.Assignee != assigneeFilter {
		return false
	}
	// ...
}

// After:
func shouldIncludeTask(
	task *domain.Task,
	statusFilter string,
	showAll bool,
	assigneeFilter string,
	goalFilter string,
) bool {
	// Filter by assignee if specified
	if assigneeFilter != "" && task.Assignee != assigneeFilter {
		return false
	}

	// Filter by goal if specified (exact, case-sensitive match)
	if goalFilter != "" && !taskHasGoal(task.Goals, goalFilter) {
		return false
	}

	// Skip status filtering if showAll is true
	if showAll {
		return true
	}

	// Apply status filter
	return matchesStatusFilter(task.Status, statusFilter)
}
```

### Add `taskHasGoal` helper function

Add this after `shouldIncludeTask`:

```go
// taskHasGoal returns true if the goals list contains the given goal name.
func taskHasGoal(goals []string, goal string) bool {
	for _, g := range goals {
		if g == goal {
			return true
		}
	}
	return false
}
```

## 3. `pkg/cli/cli.go` — Update createGenericListCommand

The `createGenericListCommand` function (~line 493) also calls `listOp.Execute(...)` with the same signature. Update it to pass `""` (empty string) for the new `goalFilter` parameter so it compiles. The generic list command does not need a `--goal` flag — only task list uses it.

```go
// Before (~line 524):
if err := listOp.Execute(ctx, vault.Path, vault.Name, pagesDir, statusFilter, showAll, assigneeFilter, *outputFormat); err != nil {

// After:
if err := listOp.Execute(ctx, vault.Path, vault.Name, pagesDir, statusFilter, showAll, assigneeFilter, "", *outputFormat); err != nil {
```

## 4. Regenerate counterfeiter mock

Run:
```bash
go generate ./pkg/ops/...
```

This regenerates `mocks/list-operation.go` from the updated `ListOperation` interface.

## 5. `pkg/ops/list_test.go` — Update existing tests and add goal filter tests

### Update existing test setup

All existing `Execute` calls need the new `goalFilter` parameter (empty string). In the `BeforeEach` block, add:

```go
// Add alongside existing variables:
var goalFilter string

// In BeforeEach:
goalFilter = ""
```

Update the `JustBeforeEach` Execute call:

```go
// Before:
err = listOp.Execute(
    ctx,
    vaultPath,
    "test-vault",
    pagesDir,
    statusFilter,
    showAll,
    assigneeFilter,
    "plain",
)

// After:
err = listOp.Execute(
    ctx,
    vaultPath,
    "test-vault",
    pagesDir,
    statusFilter,
    showAll,
    assigneeFilter,
    goalFilter,
    "plain",
)
```

Also update ALL `Execute` calls in the "JSON output" describe block to pass the empty goalFilter parameter. These calls currently have 8 positional args — add empty string `""` between the assignee filter (7th arg, currently `""`) and the output format (8th arg, currently `"json"`):

```go
// Before:
err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", "", true, "", "json")

// After:
err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", "", true, "", "", "json")
```

### Add new test contexts for goal filtering

Add these inside the main `Describe("ListOperation", ...)` block, after the existing contexts:

```go
Context("with --goal filter", func() {
    BeforeEach(func() {
        goalFilter = "Return to Live Trading"
        tasks = []*domain.Task{
            {
                Name:   "Task With Goal",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"Return to Live Trading"},
            },
            {
                Name:   "Task Without Goal",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"Other Goal"},
            },
            {
                Name:   "Task No Goals",
                Status: domain.TaskStatusTodo,
            },
        }
        mockPageStorage.ListPagesReturns(tasks, nil)
    })

    It("returns no error", func() {
        Expect(err).To(BeNil())
    })
})

Context("with --goal filter and no matching tasks", func() {
    BeforeEach(func() {
        goalFilter = "Nonexistent Goal"
        tasks = []*domain.Task{
            {
                Name:   "Task A",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"Some Goal"},
            },
        }
        mockPageStorage.ListPagesReturns(tasks, nil)
    })

    It("returns no error", func() {
        Expect(err).To(BeNil())
    })
})

Context("with --goal and --status filters combined", func() {
    BeforeEach(func() {
        goalFilter = "My Goal"
        statusFilter = "in_progress"
        tasks = []*domain.Task{
            {
                Name:   "Matching Both",
                Status: domain.TaskStatusInProgress,
                Goals:  []string{"My Goal"},
            },
            {
                Name:   "Goal Match Status Mismatch",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"My Goal"},
            },
            {
                Name:   "Status Match Goal Mismatch",
                Status: domain.TaskStatusInProgress,
                Goals:  []string{"Other Goal"},
            },
        }
        mockPageStorage.ListPagesReturns(tasks, nil)
    })

    It("returns no error", func() {
        Expect(err).To(BeNil())
    })
})

Context("with --goal filter case sensitivity", func() {
    BeforeEach(func() {
        goalFilter = "my goal"
        tasks = []*domain.Task{
            {
                Name:   "Task With Different Case",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"My Goal"},
            },
        }
        mockPageStorage.ListPagesReturns(tasks, nil)
    })

    It("returns no error (case-sensitive means no match)", func() {
        Expect(err).To(BeNil())
    })
})
```

### Add goal filter to TaskListItem JSON test

Add a test for goal filtering with JSON output. In the `Describe("ListOperation JSON output", ...)` block, add:

```go
Context("with --goal filter", func() {
    BeforeEach(func() {
        tasks := []*domain.Task{
            {
                Name:   "Matching Task",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"Target Goal", "Other Goal"},
            },
            {
                Name:   "Non-matching Task",
                Status: domain.TaskStatusTodo,
                Goals:  []string{"Different Goal"},
            },
        }
        mockPageStorage.ListPagesReturns(tasks, nil)

        capturedOutput = captureStdout(func() {
            err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", "", true, "", "Target Goal", "json")
            Expect(err).To(BeNil())
        })
    })

    It("returns only tasks matching the goal", func() {
        var items []ops.TaskListItem
        Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
        Expect(items).To(HaveLen(1))
        Expect(items[0].Name).To(Equal("Matching Task"))
    })
})
```

</requirements>

<verification>
Run `make precommit` — must pass.

Additionally verify:
- `go generate ./pkg/ops/...` succeeds and regenerates `mocks/list-operation.go`
- `go build ./...` compiles without errors
- All existing tests pass without modification beyond adding the empty goalFilter parameter
</verification>
