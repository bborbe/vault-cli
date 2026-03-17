---
status: inbox
created: "2026-03-17T12:00:00Z"
---

<summary>
- Goal lookup in findFileByName strips Obsidian wiki-link brackets `[[` and `]]` before matching
- `vault-cli task complete` successfully finds and updates goal files when goals frontmatter contains `[[Goal Name]]` format
- All existing callers of findFileByName benefit from the fix (complete, update, any future entity lookups)
- Test coverage added for bracket-wrapped names
</summary>

<objective>
Fix bug where `vault-cli task complete` fails to find goal files because `task.Goals` contains `[[Goal Name]]` (Obsidian wiki-link format) but `findFileByName` compares against bare filenames without brackets. Strip `[[` and `]]` from the name before matching.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/storage/base.go` — `findFileByName` function (~line 82). This is where the fix goes. The function receives `name` and compares `strings.ToLower(fileName)` against `strings.ToLower(name)`. When `name` is `[[Reusable Task Core]]`, it never matches `Reusable Task Core`.
- `pkg/ops/complete.go` — `markGoalCheckbox` (~line 337) calls `FindGoalByName` with raw `task.Goals[i]` which contains `[[...]]` brackets from YAML frontmatter.
- `pkg/domain/task.go` — `Task` struct, `Goals []string` field — values come from YAML like `goals: ["[[Reusable Task Core]]"]`.
- `pkg/storage/base_test.go` — existing tests for `findFileByName` (if any). Add bracket-stripping tests here.
</context>

<constraints>
- Do NOT commit — dark-factory handles git
- Fix in `findFileByName` (single place), NOT in every caller
- Strip brackets at the start of `findFileByName`, before the comparison loop
- Existing tests must pass unchanged
- Use `github.com/bborbe/errors` for error wrapping
- Tests must use Ginkgo/Gomega (follow existing patterns)
</constraints>

<requirements>

## 1. `pkg/storage/base.go` — Strip brackets in findFileByName

At the start of `findFileByName`, strip Obsidian wiki-link brackets from the name:

```go
func (b *baseStorage) findFileByName(ctx context.Context, dir string, name string) (string, string, error) {
	// Strip Obsidian wiki-link brackets [[...]]
	name = strings.TrimPrefix(name, "[[")
	name = strings.TrimSuffix(name, "]]")

	// ... existing code unchanged
```

This ensures all callers (complete, update, any future entity lookups) benefit from the fix.

## 2. Tests — Add bracket-stripping test cases

Add test cases in the appropriate test file (`pkg/storage/base_test.go` or integration tests) that verify:

1. `findFileByName` with `[[Goal Name]]` finds `Goal Name.md`
2. `findFileByName` with `Goal Name` still finds `Goal Name.md` (no regression)
3. `findFileByName` with `[[Nonexistent]]` returns file-not-found error

</requirements>

<verification>
Run `make precommit` — must pass.

Additionally verify:
- `go build ./...` compiles without errors
- All existing tests pass unchanged
- New bracket-stripping tests pass
</verification>
