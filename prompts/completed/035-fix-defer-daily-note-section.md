---
status: completed
summary: Fixed defer's addToDailyNote to insert tasks into Should section instead of appending to file bottom
container: vault-cli-035-fix-defer-daily-note-section
dark-factory-version: v0.14.5
created: "2026-03-03T22:44:14Z"
queued: "2026-03-03T22:44:14Z"
started: "2026-03-03T22:44:14Z"
completed: "2026-03-03T22:51:00Z"
---
<objective>
Fix defer's addToDailyNote to insert task into the "Should" section instead of appending to the bottom of the file. Match how the slash command /defer-task adds deferred tasks.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding-guidelines/go-testing-guide.md for testing patterns.
Read pkg/ops/defer.go — addToDailyNote method.
Read pkg/ops/defer_test.go — tests to update.
</context>

<requirements>
1. Modify `addToDailyNote` in `pkg/ops/defer.go`:

   Current behavior: appends `- [ ] [[taskName]]` to end of file.

   New behavior:
   a. Look for a "Should" section heading: `## Should` or `### Should`
   b. If found: insert `- [ ] [[taskName]]` at the end of that section (before next heading or end of file)
   c. If NOT found: look for a "Must" section heading: `## Must` or `### Must`
   d. If Must found: insert after Must section's last item (deferred task goes to Should, but if no Should exists, put after Must)
   e. If neither found: fall back to current behavior (append to end)

2. Section detection algorithm:
   ```
   - Split content into lines
   - Find line index of "## Should" or "### Should" (case-insensitive trim)
   - Find end of Should section: next heading (line starting with ## or ###) or EOF
   - Insert task line before the end-of-section line
   ```

3. Handle edge case: daily note doesn't exist yet
   - Keep current behavior: create basic structure with the task
   - But use section format: `## Should\n\n- [ ] [[taskName]]\n`

4. Update tests in `pkg/ops/defer_test.go`:
   - Add test: daily note with `## Should` section → task inserted in Should section
   - Add test: daily note with `## Must` but no Should → task inserted after Must items
   - Add test: daily note with no sections → task appended to end (fallback)
   - Add test: empty daily note → creates note with Should section
   - Add test: task already exists in Should → no duplicate added
</requirements>

<constraints>
- Do NOT modify removeFromDailyNote — it works correctly
- Do NOT change the task line format — keep `- [ ] [[taskName]]`
- Preserve all existing content in the daily note — only insert, never delete
- Use Ginkgo v2 + Gomega, follow existing test patterns
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- Deferred task appears in Should section (not at bottom)
- Existing daily note content preserved
- Fallback works when no sections exist
- No duplicate entries
</verification>
