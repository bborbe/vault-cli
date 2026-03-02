---
status: queued
created: "2026-03-02T11:56:19Z"
---










<objective>
Increase `pkg/storage` test coverage from 79.1% to ≥80%.
Targets: `WriteTask` (66.7%), `WriteGoal` (66.7%), `WriteTheme` (66.7%),
`ListTasks` (72.2%), `parseFrontmatter` (71.4%), `WriteDailyNote` (71.4%).
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Read CLAUDE.md for project conventions.
Read ~/.claude/docs/go-testing.md for testing patterns.

Existing tests in `pkg/storage/markdown_test.go` use real temp directories.
Follow that exact pattern — NO mocks for storage tests, use os.MkdirTemp.
</context>

<requirements>
Add tests in `./pkg/storage/markdown_test.go`:

1. `WriteTask` error paths:
   - Write to read-only directory → returns error
   - Round-trip: WriteTask then FindTaskByName → verify all fields preserved

2. `WriteGoal` error paths:
   - Write to read-only directory → returns error
   - Round-trip: WriteGoal then FindGoalByName → fields preserved

3. `WriteTheme` error paths:
   - Write to read-only directory → returns error

4. `ListTasks`:
   - Empty directory → returns empty slice, no error
   - Directory with non-.md files → skipped, only .md processed
   - File with invalid frontmatter → error returned or file skipped (check existing behavior)

5. `parseFrontmatter`:
   - File with no `---` markers → graceful handling (no error, empty frontmatter)
   - File with malformed YAML in frontmatter → error returned
   - File with valid frontmatter → parsed correctly

6. `WriteDailyNote`:
   - Write then ReadDailyNote → content preserved
   - Write to invalid path → error returned
</requirements>

<constraints>
- Use real temp directories: `os.MkdirTemp("", "vault-test-*")`
- Clean up with `AfterEach(func() { os.RemoveAll(tempDir) })`
- Do NOT use mocks — storage tests use real filesystem
- Check if suite file exists: `pkg/storage/storage_suite_test.go`
- Do NOT run make precommit iteratively — use make test; run make precommit once at the end
</constraints>

<verification>
Run: `make test`
Run: `go test -mod=mod -cover ./pkg/storage/...`

Target: `pkg/storage` coverage ≥80%.
</verification>

<success_criteria>
- make test passes
- pkg/storage coverage ≥80%
- WriteTask/WriteGoal/WriteTheme error paths tested
- ListTasks edge cases (empty dir, non-.md files) tested
- parseFrontmatter edge cases tested
- WriteDailyNote round-trip tested
</success_criteria>
