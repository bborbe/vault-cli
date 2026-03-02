---
status: completed
---

<objective>
Make `vault-cli lint --fix` auto-fix `MISSING_FRONTMATTER` by inserting minimal frontmatter with `status: backlog` at the top of the file.

Currently MISSING_FRONTMATTER is detected but never fixed. When `--fix` is passed, insert a minimal frontmatter block so the file becomes a valid task.
</objective>

<context>
Go CLI project for managing Obsidian vault tasks.
Read CLAUDE.md for project conventions.

Key file: `./pkg/ops/lint.go`

Minimal frontmatter to insert:
```yaml
---
status: backlog
---
```

Insert at the very beginning of the file (before any existing content).

Existing pattern to follow: `fixInvalidPriority()` and `fixInvalidStatus()` show how fixes work.
`IssueTypeMissingFrontmatter` constant already exists.
</context>

<requirements>
1. Set `Fixable: true` on `MISSING_FRONTMATTER` issues
2. Add `fixMissingFrontmatter()` method that prepends minimal frontmatter to file content
3. Add `IssueTypeMissingFrontmatter` case to `fixIssues()` switch
4. Add test cases in `lint_test.go`:
   - File with no frontmatter → detected as MISSING_FRONTMATTER, Fixable=true
   - `--fix` → file now starts with `---\nstatus: backlog\n---\n`
   - Existing content preserved after frontmatter
</requirements>

<output>
Modify in place:
- `./pkg/ops/lint.go`
- `./pkg/ops/lint_test.go`
</output>

<verification>
```
make test
go run main.go lint --fix 2>&1 | grep "FIXED.*MISSING"
```

Confirm: files previously missing frontmatter now have `status: backlog` frontmatter.
</verification>

<success_criteria>
- `make test` passes
- MISSING_FRONTMATTER is fixable (WARN not ERROR)
- `lint --fix` prepends `---\nstatus: backlog\n---\n` to files without frontmatter
- Original file content preserved after frontmatter block
</success_criteria>
