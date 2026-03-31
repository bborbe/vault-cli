---
status: completed
summary: Updated lint operation to use domain.NormalizeTaskStatus and domain.IsValidTaskStatus, added orphan goal detection and status/checkbox mismatch checks
container: vault-cli-037-fix-lint-use-normalize
dark-factory-version: v0.14.5
created: "2026-03-03T22:55:26Z"
queued: "2026-03-03T22:55:26Z"
started: "2026-03-03T22:55:26Z"
completed: "2026-03-03T23:05:14Z"
---
<objective>
Update lint operation to use the new NormalizeTaskStatus and IsValidTaskStatus from domain package instead of hardcoded status lists. Also add parent goal existence validation and status/checkbox consistency check.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding/docs/go-testing-guide.md for testing patterns.
Read pkg/ops/lint.go — the file to modify.
Read pkg/ops/lint_validate_exit_test.go — existing lint tests.
Read pkg/domain/task.go — NormalizeTaskStatus, IsValidTaskStatus (added by earlier prompt).
</context>

<requirements>
1. Update `detectInvalidStatus` in `pkg/ops/lint.go`:
   - Replace hardcoded `validStatuses` list with call to `domain.IsValidTaskStatus(domain.TaskStatus(statusValue))`
   - Replace hardcoded `statusMigrationMap` with call to `domain.NormalizeTaskStatus(statusValue)` — if it returns a different canonical value, it's fixable

2. Update `fixInvalidStatus` in `pkg/ops/lint.go`:
   - Replace hardcoded migration map with `domain.NormalizeTaskStatus(oldValue)` — use the returned canonical value

3. Add new lint check: `IssueTypeOrphanGoal` ("ORPHAN_GOAL")
   - For files that have `goals:` in frontmatter
   - Parse `goals:` field (list of strings, may contain `[[Goal Name]]` wikilinks)
   - Strip `[[` and `]]` from goal names
   - Check if goal file exists: try goalsDir + goalName + ".md"
   - If goal file not found → issue with description "goal not found: {goalName}"
   - This check requires the vaultPath, so add it to `lintFile` if not already available
   - Fixable: false (can't auto-create goals)

4. Add new lint check: `IssueTypeStatusCheckboxMismatch` ("STATUS_CHECKBOX_MISMATCH")
   - Parse checkboxes from content (reuse checkbox regex)
   - If status=completed AND checkboxes exist AND not all checked → issue "status is completed but N/M checkboxes unchecked"
   - If all checkboxes checked AND status is not completed → issue "all checkboxes checked but status is {status}"
   - Fixable: true for the second case (can set status to completed)
   - Skip this check for tasks with `recurring:` field (recurring tasks have unchecked boxes by design)

5. Add fix for STATUS_CHECKBOX_MISMATCH when fixable:
   - If all checkboxes checked and status != completed → set status to completed

6. Update tests:
   - Update existing status validation tests to work with NormalizeTaskStatus
   - Add test: task with `goals: ["[[Missing Goal]]"]` and goal file doesn't exist → ORPHAN_GOAL issue
   - Add test: task with `status: completed` and unchecked checkboxes → STATUS_CHECKBOX_MISMATCH
   - Add test: task with all checked checkboxes and `status: in_progress` → STATUS_CHECKBOX_MISMATCH (fixable)
   - Add test: recurring task with unchecked checkboxes → no mismatch (skipped)
</requirements>

<constraints>
- Depend on domain.NormalizeTaskStatus and domain.IsValidTaskStatus from the first prompt
- For goal file checking, lintFile may need the vaultPath parameter — adjust signature if needed, propagate from Execute
- Do NOT change the existing MISSING_FRONTMATTER, INVALID_PRIORITY, or DUPLICATE_KEY checks
- Use Ginkgo v2 + Gomega, follow existing test patterns
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- Lint uses NormalizeTaskStatus for status validation
- `status: done` → fixable to `completed`
- `status: current` → fixable to `in_progress`
- Orphan goal detection works
- Status/checkbox mismatch detected
- Recurring tasks exempt from mismatch check
</verification>
