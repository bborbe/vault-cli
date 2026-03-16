---
status: queued
---

<summary>
- Fix `ReadTheme` to use `config.ThemesDir` instead of hardcoded `"Themes"`
- Remove blank lines between counterfeiter directives and interface declarations in `show.go` and `watch.go`
</summary>

<objective>
Fix two minor code quality issues found during code review.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/storage/theme.go` — `ReadTheme` hardcodes `"Themes"` directory
- `pkg/storage/storage.go` — `Config` struct with `ThemesDir` field
- `pkg/ops/show.go` — counterfeiter directive placement
- `pkg/ops/watch.go` — counterfeiter directive placement
- `pkg/storage/markdown_test.go` — existing theme tests (check for hardcoded "Themes" in tests too)
</context>

<rules>
- Minimal changes only — fix exactly these two issues
- Update tests if they rely on the hardcoded "Themes" path
- Counterfeiter directive must be on the line directly above the `type` declaration, no blank line
</rules>

<changes>

## 1. `pkg/storage/theme.go` — Use config.ThemesDir

Line 28:
```go
// Before:
filePath := filepath.Join(vaultPath, "Themes", themeID.String()+".md")

// After:
filePath := filepath.Join(vaultPath, t.config.ThemesDir, themeID.String()+".md")
```

## 2. `pkg/ops/show.go` — Remove blank line before interface

Lines 20-22:
```go
// Before:
//counterfeiter:generate -o ../../mocks/show-operation.go --fake-name ShowOperation . ShowOperation

// ShowOperation returns full detail for a single task.
type ShowOperation interface {

// After:
//counterfeiter:generate -o ../../mocks/show-operation.go --fake-name ShowOperation . ShowOperation
// ShowOperation returns full detail for a single task.
type ShowOperation interface {
```

## 3. `pkg/ops/watch.go` — Remove blank line before interface

Lines 35-37:
```go
// Before:
//counterfeiter:generate -o ../../mocks/watch-operation.go --fake-name WatchOperation . WatchOperation

// WatchOperation watches vault directories and streams change events.
type WatchOperation interface {

// After:
//counterfeiter:generate -o ../../mocks/watch-operation.go --fake-name WatchOperation . WatchOperation
// WatchOperation watches vault directories and streams change events.
type WatchOperation interface {
```

</changes>

<verification>
make precommit
</verification>
