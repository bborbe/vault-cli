---
status: completed
summary: Added subtask completion check to complete operation with plain/JSON mode support
container: vault-cli-030-add-complete-subtask-check
dark-factory-version: v0.14.5
created: "2026-03-03T22:17:16Z"
queued: "2026-03-03T22:17:16Z"
started: "2026-03-03T22:17:16Z"
completed: "2026-03-03T22:24:12Z"
---
<objective>
Add subtask completion check to complete operation. Before marking a task complete, verify all checkboxes are done. In plain mode, warn and proceed. In JSON mode, return incomplete status.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding/docs/go-testing-guide.md for testing patterns.
Read pkg/ops/complete.go — the file to modify.
Read pkg/ops/complete_test.go — tests to update.
Read pkg/ops/update.go — has parseCheckboxes method to reference.
Read pkg/domain/task.go — CheckboxItem struct.
</context>

<requirements>
1. In `pkg/ops/complete.go`, add a subtask check BEFORE setting `task.Status = domain.TaskStatusCompleted` (for non-recurring tasks only):

   a. Parse checkboxes from task.Content (reuse or inline the checkbox parsing logic from update.go)
   b. Count pending (`[ ]`) and in-progress (`[/]`) items
   c. If pending > 0 OR inprogress > 0:
      - If `outputFormat == "json"`: output JSON with `{"success": false, "reason": "incomplete_items", "pending": N, "inprogress": M, "completed": C, "total": T}` and return nil (no error — caller handles)
      - If `outputFormat == "plain"`: print warning `⚠️ Warning: N/T subtasks incomplete (N pending, M in-progress). Completing anyway.` and CONTINUE (don't block)
   d. If all complete → proceed normally

2. Extract checkbox counting into a shared helper (avoid code duplication with update.go):
   - Add to `pkg/ops/complete.go`:
     ```go
     func countCheckboxStates(content string) (completed, inProgress, pending int) {
         // parse checkboxes, count each state
     }
     ```
   - Or use the existing parseCheckboxes from update.go if accessible (both in same package)

3. The `IncompleteResult` struct for JSON output:
   ```go
   type IncompleteResult struct {
       Success    bool   `json:"success"`
       Reason     string `json:"reason"`
       Pending    int    `json:"pending"`
       InProgress int    `json:"inprogress"`
       Completed  int    `json:"completed"`
       Total      int    `json:"total"`
   }
   ```

4. Do NOT check subtasks for recurring tasks — recurring always resets checkboxes, so they're expected incomplete.

5. Update tests in `pkg/ops/complete_test.go`:
   - Add test: task with unchecked checkboxes in content + plain format → warning printed, task still completed
   - Add test: task with unchecked checkboxes + json format → IncompleteResult output, task NOT completed (no WriteTask call for status update)
   - Add test: task with all checked checkboxes → completed normally
   - Add test: task with no checkboxes → completed normally (no checkbox = nothing to check)
   - Add test: recurring task with unchecked checkboxes → still resets (no blocking)
</requirements>

<constraints>
- Plain mode: warn but still complete (user confirmed by running the command)
- JSON mode: return incomplete status WITHOUT completing (caller decides)
- Do NOT modify recurring task flow
- Do NOT modify goal or daily note update logic
- Use Ginkgo v2 + Gomega, follow existing test patterns in complete_test.go
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- Task with `- [ ] unchecked` in content + plain → warning + completed
- Task with `- [ ] unchecked` in content + json → IncompleteResult, NOT completed
- Task with all `- [x]` → completed normally
- Task with zero checkboxes → completed normally
- Recurring task → not affected
</verification>
