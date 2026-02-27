<objective>
Extend `vault-cli lint --fix` to auto-correct invalid status values using a migration map.

Currently INVALID_STATUS issues are detected but never fixed. The vault has stale tasks with `status: next` (should be `todo`) and `status: current` (should be `in_progress`) from a migration that wasn't fully completed. The lint command should fix these automatically when `--fix` is passed.
</objective>

<context>
Go CLI project for managing Obsidian vault tasks.
Read CLAUDE.md for project conventions.

Valid status values: `todo`, `in_progress`, `backlog`, `completed`, `hold`, `aborted`

Migration map (fixable invalid values):
- `next` → `todo`
- `current` → `in_progress`

Any other invalid status value is NOT fixable (reported as ERROR, not auto-fixed).

Key file: `./pkg/ops/lint.go`

The existing pattern in this file:
- `detectInvalidStatus()` returns `(bool, string)` — needs to also return whether it's fixable
- `LintIssue.Fixable` field controls whether `--fix` attempts a fix
- `fixIssues()` switch handles fixable issue types
- `IssueTypeInvalidStatus` constant already exists
</context>

<requirements>
1. Update `detectInvalidStatus()` to return `(bool, string, bool)` — issue found, invalid value, is fixable
2. Set `Fixable: true` on INVALID_STATUS issues when the value is in the migration map
3. Add `fixInvalidStatus()` method that replaces the invalid status value with the correct one using regex on frontmatter
4. Add `IssueTypeInvalidStatus` case to the `fixIssues()` switch statement
5. Update the `lintFile()` call site to handle the new return value
6. Update `lint_test.go` to add tests for:
   - `next` status → detected as INVALID_STATUS, Fixable=true
   - `current` status → detected as INVALID_STATUS, Fixable=true
   - `--fix` on `next` → replaces with `todo` in file
   - `--fix` on `current` → replaces with `in_progress` in file
   - Unknown invalid status (e.g. `foo`) → Fixable=false
</requirements>

<implementation>
Follow the same pattern as `fixInvalidPriority()`:
- Use regex to find and replace the status field in frontmatter
- Only replace within the frontmatter block (between `---` delimiters)
- Return `(string, bool)` — new content, was fixed

Status regex pattern: `(?m)^status:\s*['"]?(next|current)['"]?\s*$`

Migration map:
```go
statusMigrationMap := map[string]string{
    "next":    "todo",
    "current": "in_progress",
}
```
</implementation>

<output>
Modify in place:
- `./pkg/ops/lint.go` — add fixable status detection and fix logic
- `./pkg/ops/lint_test.go` — add tests for new behavior
</output>

<verification>
Run after changes:
```
make test
go run main.go lint 2>&1 | grep "next\|current" | head -5
```

Confirm:
- `make test` passes
- Tasks with `status: next` show as WARN (fixable) not ERROR
- `go run main.go lint --fix` changes `next` → `todo` and `current` → `in_progress` in actual files
</verification>

<success_criteria>
- `make test` passes with no failures
- `detectInvalidStatus` returns fixable=true for `next` and `current`
- `lint --fix` writes corrected status values to files
- Unknown invalid statuses remain non-fixable (ERROR not WARN)
- Tests cover all migration cases
</success_criteria>
