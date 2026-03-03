<objective>
Code review after slash command alignment changes. Verify consistency across all operations, check for dead code, ensure tests cover new behavior.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding-guidelines/go-testing-guide.md for testing patterns.
Read ALL files in pkg/ops/ — review complete, defer, update, workon, list, lint, frontmatter.
Read ALL files in pkg/domain/ — verify model consistency.
Read ALL test files in pkg/ops/ — verify coverage.
</context>

<requirements>
Review and fix these categories:

1. **Dead code removal:**
   - Search for any remaining references to `TaskStatusDone` or `TaskStatusDeferred`
   - Search for unused variables, functions, or imports
   - Run `golangci-lint run ./...` and fix all findings

2. **Consistency check:**
   - All operations writing `status: completed` (never `done`)
   - All checkbox regex patterns include `[/]` support: `[ x/]` not `[ x]`
   - All daily note operations handle missing daily note gracefully
   - All JSON output uses consistent struct patterns (MutationResult vs custom)
   - Error wrapping uses `errors.Wrap(ctx, err, "msg")` pattern consistently

3. **Test coverage gaps:**
   - Run `go test -cover ./pkg/ops/...` — target ≥85%
   - If coverage < 85%, add tests for uncovered paths
   - Verify each operation has tests for: success, error, edge cases

4. **Frontmatter set operation:**
   - `pkg/ops/frontmatter.go` line 100: `task.Status = domain.TaskStatus(value)` — this bypasses validation
   - Add validation: `if _, ok := domain.NormalizeTaskStatus(value); !ok { return error }`
   - Or use NormalizeTaskStatus to accept aliases: `normalized, ok := domain.NormalizeTaskStatus(value); task.Status = normalized`

5. **List operation:**
   - Verify `statusPriority` covers all 6 canonical statuses
   - Verify `matchesStatusFilter` works with aliases (user might pass `--status done`)
   - Add NormalizeTaskStatus call in filter: `status, _ := domain.NormalizeTaskStatus(filter)` before comparing
</requirements>

<constraints>
- This is a review prompt — fix issues found, don't add new features
- Do NOT refactor working code for style — only fix correctness issues
- Do NOT change public interfaces unless needed for correctness
- Use Ginkgo v2 + Gomega for any new tests
- Run `make precommit` once at the very end
</constraints>

<verification>
Run: `make precommit`
Run: `go test -cover ./pkg/ops/...`
Confirm:
- Zero golangci-lint findings
- No references to deprecated status constants
- Coverage ≥ 85% for pkg/ops
- All operations consistent in behavior
</verification>
