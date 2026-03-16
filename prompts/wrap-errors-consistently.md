---
status: queued
---

<summary>
- Add `ctx context.Context` parameter to `baseStorage` helpers (`parseFrontmatter`, `serializeWithFrontmatter`, `findFileByName`)
- Replace `fmt.Errorf` with `errors.Wrap(ctx, ...)` in those helpers
- Wrap bare `return nil, err` in `FindTaskByName` and `FindDecisionByName`
- Replace `fmt.Errorf` with `errors.Wrap` in `cli.go` RunE closures
- Update all callers of changed helpers to pass ctx
</summary>

<objective>
Make error wrapping consistent across the codebase. All error returns in storage and CLI layers should use `github.com/bborbe/errors` with context wrapping, matching the pattern already used in ops layer.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/storage/base.go` — helpers to modify: `parseFrontmatter`, `serializeWithFrontmatter`, `findFileByName`
- `pkg/storage/task.go` — caller of base helpers, `FindTaskByName` has bare `return nil, err`
- `pkg/storage/decision.go` — `FindDecisionByName` has bare `return nil, err`, also uses `fmt.Errorf`
- `pkg/storage/theme.go` — caller of base helpers
- `pkg/storage/goal.go` — caller of base helpers
- `pkg/storage/page.go` — caller of base helpers
- `pkg/storage/daily_note.go` — caller of base helpers
- `pkg/cli/cli.go` — uses `fmt.Errorf` instead of `errors.Wrap`
- `pkg/storage/markdown_test.go` — tests for storage layer
- `pkg/storage/decision_test.go` — tests for decision storage

Key rule: **Don't log + return error — do one or the other.**
</context>

<rules>
- Use `github.com/bborbe/errors` for all wrapping — never `fmt.Errorf("...: %w", err)`
- Pattern: `errors.Wrap(ctx, err, "context message")`
- For errors without a cause: `errors.Errorf(ctx, "message %s", arg)` or keep `fmt.Errorf` for simple value errors without ctx
- Every function that returns an error and has access to `ctx` should wrap with `errors.Wrap`
- Do NOT change function signatures of exported interface methods
- Do NOT change test assertions — if tests check for specific error messages, preserve them
- `parseCheckboxes` does not return error — leave unchanged
- `isSymlinkOutsideVault` is a package-level func without ctx — leave unchanged
</rules>

<changes>

## 1. `pkg/storage/base.go` — Add ctx to helpers

Change signatures:
```go
// Before:
func (b *baseStorage) parseFrontmatter(content []byte, target interface{}) error
func (b *baseStorage) serializeWithFrontmatter(frontmatter interface{}, originalContent string) (string, error)
func (b *baseStorage) findFileByName(dir string, name string) (string, string, error)

// After:
func (b *baseStorage) parseFrontmatter(ctx context.Context, content []byte, target interface{}) error
func (b *baseStorage) serializeWithFrontmatter(ctx context.Context, frontmatter interface{}, originalContent string) (string, error)
func (b *baseStorage) findFileByName(ctx context.Context, dir string, name string) (string, string, error)
```

Replace error returns:
- `return fmt.Errorf("no frontmatter found")` → `return errors.Errorf(ctx, "no frontmatter found")`
- `return fmt.Errorf("unmarshal yaml: %w", err)` → `return errors.Wrap(ctx, err, "unmarshal yaml")`
- `return "", fmt.Errorf("marshal yaml: %w", err)` → `return "", errors.Wrap(ctx, err, "marshal yaml")`
- `return "", "", fmt.Errorf("read directory %s: %w", dir, err)` → `return "", "", errors.Wrap(ctx, err, fmt.Sprintf("read directory %s", dir))`
- `return "", "", fmt.Errorf("file not found: %s", name)` → `return "", "", errors.Errorf(ctx, "file not found: %s", name)`

Add `"context"` to imports.

## 2. Update all callers of base helpers

Every call to `parseFrontmatter`, `serializeWithFrontmatter`, `findFileByName` must now pass `ctx`. These callers already have `ctx`:

- `pkg/storage/base.go:134` — `b.parseFrontmatter(content, task)` → `b.parseFrontmatter(ctx, content, task)`
- `pkg/storage/task.go:35` — `t.serializeWithFrontmatter(task, task.Content)` → `t.serializeWithFrontmatter(ctx, task, task.Content)`
- `pkg/storage/task.go:54` — `t.findFileByName(tasksDir, name)` → `t.findFileByName(ctx, tasksDir, name)`
- `pkg/storage/decision.go:41` — `d.parseFrontmatter(content, decision)` → `d.parseFrontmatter(ctx, content, decision)`
- `pkg/storage/decision.go:164` — `d.serializeWithFrontmatter(decision, decision.Content)` → `d.serializeWithFrontmatter(ctx, decision, decision.Content)`
- `pkg/storage/theme.go:48` — `t.parseFrontmatter(content, theme)` → `t.parseFrontmatter(ctx, content, theme)`
- `pkg/storage/theme.go:57` — `t.serializeWithFrontmatter(theme, theme.Content)` → `t.serializeWithFrontmatter(ctx, theme, theme.Content)`
- `pkg/storage/goal.go` — same pattern for parseFrontmatter and serializeWithFrontmatter calls
- `pkg/storage/page.go` — same pattern for parseFrontmatter calls
- `pkg/storage/daily_note.go` — same pattern for all base helper calls

## 3. Wrap bare returns in Find* methods

`pkg/storage/task.go:55-56`:
```go
// Before:
return nil, err
// After:
return nil, errors.Wrap(ctx, err, "find task file")
```

`pkg/storage/decision.go:116-117`:
```go
// Before:
return nil, err
// After:
return nil, errors.Wrap(ctx, err, "list decisions")
```

`pkg/storage/decision.go:111`:
```go
// Before:
return nil, fmt.Errorf("invalid decision name: %s", name)
// After:
return nil, errors.Errorf(ctx, "invalid decision name: %s", name)
```

## 4. `pkg/cli/cli.go` — Replace fmt.Errorf with errors.Wrap

In all `RunE` closures, replace:
- `return fmt.Errorf("get vaults: %w", err)` → `return errors.Wrap(ctx, err, "get vaults")`
- `return fmt.Errorf("task not found in any vault: %w", lastErr)` → `return errors.Wrap(ctx, lastErr, "task not found in any vault")`
- Apply same pattern to ALL `fmt.Errorf("...: %w", err)` in RunE closures

Add `"github.com/bborbe/errors"` to imports, remove `"fmt"` if no longer needed.

Keep `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` in `Execute()` unchanged — that's the slog prompt's scope.

</changes>

<verification>
make precommit
</verification>
