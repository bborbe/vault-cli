<objective>
Add daily note update to workon operation. When starting work on a task, mark it as in-progress `[/]` in today's daily note. Matches how /work-on-task slash command behaves.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding-guidelines/go-testing-guide.md for testing patterns.
Read pkg/ops/workon.go — the file to modify.
Read pkg/ops/workon_test.go — tests to update.
Read pkg/ops/complete.go — updateDailyNote method for reference on daily note manipulation.
Read pkg/storage/markdown.go — Storage interface with ReadDailyNote/WriteDailyNote.
</context>

<requirements>
1. Add a `updateDailyNote` method to `workOnOperation` in `pkg/ops/workon.go`:
   - Read today's daily note via `w.storage.ReadDailyNote(ctx, vaultPath, today)`
   - If daily note is empty → skip (no daily note exists)
   - Search for a checkbox line containing the task name (case-insensitive)
   - If found with `[ ]` → replace with `[/]` (mark in-progress)
   - If found with `[/]` → already in-progress, skip
   - If found with `[x]` → already completed, skip
   - If not found → append `- [/] [[taskName]]` to the Must section (or end of file if no Must section)
   - Write updated daily note

2. Call this method in Execute, after WriteTask succeeds:
   ```go
   today := time.Now().Format("2006-01-02")
   if err := w.updateDailyNote(ctx, vaultPath, today, task.Name); err != nil {
       warning := fmt.Sprintf("failed to update daily note: %v", err)
       warnings = append(warnings, warning)
   }
   ```

3. Add `warnings` tracking to Execute (similar to complete.go pattern):
   - Collect warnings
   - Include in JSON output
   - Print to stderr in plain output

4. Update `MutationResult` usage in workon: add Warnings field (already exists on MutationResult struct in complete.go — same package, reuse it)

5. Update tests in `pkg/ops/workon_test.go`:
   - Add test: daily note exists with `- [ ] [[my-task]]` → changed to `- [/] [[my-task]]`
   - Add test: daily note exists with `- [/] [[my-task]]` → unchanged
   - Add test: daily note doesn't exist → no error, task still marked in_progress
   - Add test: daily note exists without task → task appended as `- [/] [[my-task]]`

Note: the regex for checkbox matching must support `[/]` — use `^(\s*)- \[([ x/])\] (.+)$`
</requirements>

<constraints>
- Do NOT modify the complete or defer operations
- Do NOT change the core workon logic (status + assignee)
- Daily note update failures are warnings, not errors — never block the main operation
- Use Ginkgo v2 + Gomega, follow existing test patterns in workon_test.go
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- Workon updates daily note checkbox to `[/]`
- Missing daily note doesn't cause error
- Already in-progress task not modified
- Warnings reported but don't block operation
</verification>
