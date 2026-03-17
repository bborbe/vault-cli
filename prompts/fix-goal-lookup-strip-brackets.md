---
status: inbox
created: "2026-03-17T12:00:00Z"
---

<summary>
- Entity name lookups accept Obsidian wiki-link format `[[Name]]` and resolve it the same as bare `Name`
- Bracket stripping happens once in the shared lookup function, benefiting all callers
- Goal completion works when goals frontmatter contains bracket-wrapped names
- No regression for bare name lookups without brackets
- New test file covers bracket-wrapped, bare, and nonexistent name lookups
</summary>

<objective>
Entity name lookups accept both Obsidian wiki-link format `[[Goal Name]]` and bare `Goal Name`, resolving both to the same file.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/storage/base.go` — `findFileByName` function (~line 74). Receives `name`, does exact path check then case-insensitive substring match against directory entries. Currently does not strip `[[`/`]]` brackets.
- `pkg/storage/base_test.go` — does NOT exist yet. Create this file for bracket-stripping tests. Follow patterns from `pkg/storage/objective_test.go` (Ginkgo Describe/It blocks, `storage_test` package).
- `pkg/ops/complete.go` — `markGoalCheckbox` calls `FindGoalByName` with values from `task.Goals` which may contain `[[...]]` brackets from YAML frontmatter.
</context>

<constraints>
- Do NOT commit — dark-factory handles git
- Fix in `findFileByName` (single place), NOT in every caller
- Existing tests must pass unchanged
- Use `github.com/bborbe/errors` for error wrapping
- Tests must use Ginkgo/Gomega (follow existing patterns)
</constraints>

<requirements>

## 1. Strip Obsidian brackets in `findFileByName`

At the start of `findFileByName` in `pkg/storage/base.go`, strip `[[` prefix and `]]` suffix from `name` before any comparison:

```go
name = strings.TrimPrefix(name, "[[")
name = strings.TrimSuffix(name, "]]")
```

## 2. Add test cases for bracket handling

Create `pkg/storage/base_test.go` (package `storage_test`, follow patterns from `objective_test.go`). Add cases verifying:

1. `findFileByName` with `[[Goal Name]]` resolves to `Goal Name.md`
2. `findFileByName` with `Goal Name` still resolves to `Goal Name.md` (no regression)
3. `findFileByName` with `[[Nonexistent]]` returns file-not-found error

</requirements>

<verification>
Run `make precommit` — must pass.
</verification>
