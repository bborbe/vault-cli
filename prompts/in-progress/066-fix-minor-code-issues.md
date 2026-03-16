---
status: approved
created: "2026-03-16T15:19:37Z"
queued: "2026-03-16T15:19:37Z"
---

<summary>
- Fix theme reading to use the configured themes directory instead of a hardcoded path
- Fix counterfeiter directive placement to be directly above interface declarations
</summary>

<objective>
Fix two code quality issues: ReadTheme ignores the configured themes directory (breaking custom vault layouts), and two counterfeiter directives have a blank line before the interface they generate for (violating project convention).
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/storage/theme.go` — `ReadTheme` method, see how it constructs the file path
- `pkg/storage/storage.go` — `Config` struct with `ThemesDir` field, verify field name
- `pkg/ops/show.go` — counterfeiter directive at top of file, check for blank line before interface
- `pkg/ops/watch.go` — counterfeiter directive, check for blank line before interface
</context>

<constraints>
- Minimal changes only — fix exactly these two issues
- Do NOT change any test files unless they hardcode the "Themes" path and would break
- Counterfeiter directive must be on the line directly above the `type` declaration, no blank line between
</constraints>

<requirements>

## 1. `pkg/storage/theme.go` — Use configured themes directory

In `ReadTheme` method, replace the hardcoded `"Themes"` string with `t.config.ThemesDir`:

```go
// Before (in ReadTheme):
filePath := filepath.Join(vaultPath, "Themes", themeID.String()+".md")

// After:
filePath := filepath.Join(vaultPath, t.config.ThemesDir, themeID.String()+".md")
```

## 2. `pkg/ops/show.go` — Remove blank line before ShowOperation interface

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

## 3. `pkg/ops/watch.go` — Remove blank line before WatchOperation interface

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

</requirements>

<verification>
make precommit
</verification>
