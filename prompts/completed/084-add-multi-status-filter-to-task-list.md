---
status: completed
summary: Changed --status flag on task list from single string to string slice, supporting repeated flags and comma-separated values with backward compatibility preserved.
container: vault-cli-084-add-multi-status-filter-to-task-list
dark-factory-version: v0.57.5
created: "2026-03-18T20:10:59Z"
queued: "2026-03-18T20:20:48Z"
started: "2026-03-18T20:50:56Z"
completed: "2026-03-18T20:54:14Z"
---

<summary>
- The --status flag on task list accepts multiple values via repeated flags or comma-separated values
- Users can run: vault-cli task list --status=in_progress --status=completed
- Users can run: vault-cli task list --status=in_progress,completed
- When no --status is given, default behavior is preserved (show todo and in_progress only)
- When --all is given, status filtering is still skipped entirely
- Case-insensitive matching continues to work for each status value
- All existing tests continue to pass
</summary>

<objective>
Change the --status flag on `vault-cli task list` from a single string to a string slice so users can filter by multiple statuses at once. Preserve backward compatibility: a single --status value still works, and no --status still defaults to todo+in_progress.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/cli/cli.go — function `createTaskListCommand` (~line 302) defines the --status flag as `StringVar`.
Read pkg/ops/list.go — `ListOperation` interface, `listOperation.Execute`, `filterTasks`, `shouldIncludeTask`, `matchesStatusFilter` functions.
Read pkg/ops/list_test.go — existing test structure using Ginkgo v2, `statusFilter string` variable in BeforeEach.
Read mocks/list-operation.go — counterfeiter-generated mock with `Execute` signature matching the interface.
</context>

<requirements>
1. In `pkg/cli/cli.go`, function `createTaskListCommand`:
   - Change `var statusFilter string` to `var statusFilters []string`
   - Change the flag registration from:
     ```go
     cmd.Flags().StringVar(&statusFilter, "status", "",
         "Filter by status (e.g. todo, in_progress, completed, done, deferred)")
     ```
     to:
     ```go
     cmd.Flags().StringSliceVar(&statusFilters, "status", nil,
         "Filter by status (e.g. --status=in_progress --status=completed). Cobra StringSliceVar natively supports both repeated flags and comma-separated values.")
     ```
   - In the `RunE` closure, change the `listOp.Execute(...)` call to pass `statusFilters` instead of `statusFilter`

2. In `pkg/ops/list.go`, update the `ListOperation` interface:
   - Change parameter `statusFilter string` to `statusFilters []string` in the `Execute` method signature:
     ```go
     Execute(
         ctx context.Context,
         vaultPath string,
         vaultName string,
         pagesDir string,
         statusFilters []string,
         showAll bool,
         assigneeFilter string,
         goalFilter string,
         outputFormat string,
     ) error
     ```

3. In `pkg/ops/list.go`, update the `listOperation.Execute` method:
   - Change parameter `statusFilter string` to `statusFilters []string`
   - Update the `filterTasks` call to pass `statusFilters` instead of `statusFilter`

4. In `pkg/ops/list.go`, update `filterTasks`:
   - Change parameter `statusFilter string` to `statusFilters []string`
   - Pass `statusFilters` to `shouldIncludeTask`

5. In `pkg/ops/list.go`, update `shouldIncludeTask`:
   - Change parameter `statusFilter string` to `statusFilters []string`
   - Pass `statusFilters` to `matchesStatusFilter`

6. In `pkg/ops/list.go`, update `matchesStatusFilter`:
   - Change signature from `matchesStatusFilter(status domain.TaskStatus, filter string) bool` to `matchesStatusFilter(status domain.TaskStatus, filters []string) bool`
   - New body:
     ```go
     func matchesStatusFilter(status domain.TaskStatus, filters []string) bool {
         if len(filters) > 0 {
             for _, f := range filters {
                 if strings.EqualFold(string(status), f) {
                     return true
                 }
             }
             return false
         }
         // Default: show only todo and in_progress
         return status == domain.TaskStatusTodo || status == domain.TaskStatusInProgress
     }
     ```

7. Regenerate the mock by running:
   ```
   go generate ./pkg/ops/...
   ```
   This will regenerate `mocks/list-operation.go` from the counterfeiter directive. If counterfeiter is not available, manually update the mock to change `arg5 string` to `arg5 []string` throughout the file (in the struct fields, Execute method signature, and all helper methods).

8. In `pkg/ops/list_test.go`, update the test variable and all usages:
   - Change `var statusFilter string` to `var statusFilters []string`
   - Change `statusFilter = ""` in BeforeEach to `statusFilters = nil`
   - In the `JustBeforeEach` block, change the `listOp.Execute(...)` call to pass `statusFilters` instead of `statusFilter`
   - Where tests set `statusFilter = "in_progress"`, change to `statusFilters = []string{"in_progress"}`
   - Where tests set `statusFilter = "In_Progress"`, change to `statusFilters = []string{"In_Progress"}`
   - Where tests set `statusFilter = "in_progress"` in the combined goal+status test, change to `statusFilters = []string{"in_progress"}`

9. In `pkg/ops/list_test.go`, add a new test context for multi-status filtering inside the "ListOperation JSON output" describe block. Follow the existing pattern: set up tasks in `BeforeEach`, capture stdout, unmarshal JSON in each `It`. Example:
   ```go
   Context("with multiple --status filters", func() {
       BeforeEach(func() {
           tasks := []*domain.Task{
               {Name: "IP Task", Status: domain.TaskStatusInProgress},
               {Name: "Done Task", Status: domain.TaskStatusCompleted},
               {Name: "Todo Task", Status: domain.TaskStatusTodo},
               {Name: "Hold Task", Status: domain.TaskStatusHold},
           }
           mockPageStorage.ListPagesReturns(tasks, nil)

           capturedOutput = captureStdout(func() {
               err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", []string{"in_progress", "completed"}, false, "", "", "json")
               Expect(err).To(BeNil())
           })
       })

       It("includes tasks matching requested statuses", func() {
           var items []ops.TaskListItem
           Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
           Expect(items).To(HaveLen(2))
       })

       It("excludes tasks not matching requested statuses", func() {
           var items []ops.TaskListItem
           Expect(json.Unmarshal(capturedOutput, &items)).To(Succeed())
           for _, item := range items {
               Expect(item.Status).NotTo(Equal("todo"))
               Expect(item.Status).NotTo(Equal("hold"))
           }
       })
   })
   ```

10. In `pkg/ops/list_test.go`, update ALL 7 direct `listOp.Execute(...)` calls that pass `""` as the 5th argument (statusFilter) to pass `nil` instead. There are 6 one-line calls (lines ~340, 400, 483, 520, 545, 569) and 1 multi-line call (line ~442). Change `""` to `nil` for the 5th argument in each:
    ```go
    // Before:
    err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", "", true, "", "", "json")
    // After:
    err := listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "", "json")
    ```
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass after updates
- Backward compatibility: single --status=in_progress must still work
- Default behavior (no --status flag) must still show only todo and in_progress
- --all flag must still bypass status filtering entirely
- Case-insensitive matching must work for each value in the slice
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
