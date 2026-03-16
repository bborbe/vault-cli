---
status: completed
summary: Added ctx parameter to storage base helpers and replaced all fmt.Errorf with errors.Wrap/errors.Errorf throughout storage and CLI layers for consistent error wrapping.
container: vault-cli-064-wrap-errors-consistently
dark-factory-version: v0.57.3
created: "2026-03-16T15:19:37Z"
queued: "2026-03-16T15:19:37Z"
started: "2026-03-16T15:27:13Z"
completed: "2026-03-16T15:34:08Z"
---

<summary>
- Add context parameter to shared storage helpers so errors carry call-chain context
- Wrap all bare error returns in Find* methods across task, goal, and decision storage
- Replace standard fmt.Errorf with errors.Wrap in CLI command closures for consistent error chains
</summary>

<objective>
Make error wrapping consistent across the codebase. All error returns in storage and CLI layers should use github.com/bborbe/errors with context wrapping, matching the pattern already used in the ops layer.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/storage/base.go` — helpers to modify: `parseFrontmatter`, `serializeWithFrontmatter`, `findFileByName`
- `pkg/storage/task.go` — caller of base helpers, `FindTaskByName` has bare `return nil, err`
- `pkg/storage/decision.go` — `FindDecisionByName` has bare `return nil, err`, also uses `fmt.Errorf`
- `pkg/storage/theme.go` — caller of `parseFrontmatter` and `serializeWithFrontmatter`
- `pkg/storage/goal.go` — caller of all 3 base helpers, `FindGoalByName` has bare `return nil, err`
- `pkg/cli/cli.go` — uses `fmt.Errorf` instead of `errors.Wrap`

Note: `pkg/storage/page.go` and `pkg/storage/daily_note.go` do NOT call base helpers directly — skip them.

Key rule: **Don't log + return error — do one or the other.**
</context>

<constraints>
- Use `github.com/bborbe/errors` for all wrapping — never `fmt.Errorf("...: %w", err)`
- Pattern: `errors.Wrap(ctx, err, "context message")`
- For errors without a cause: `errors.Errorf(ctx, "message %s", arg)`
- Every function that returns an error and has access to `ctx` should wrap with `errors.Wrap`
- Do NOT change function signatures of exported interface methods
- Do NOT change test assertions — if tests check for specific error messages, preserve them
- `parseCheckboxes` does not return error — leave unchanged
- `isSymlinkOutsideVault` is a package-level func without ctx — leave unchanged
</constraints>

<requirements>

## 1. `pkg/storage/base.go` — Add ctx to internal helpers

Change signatures of these unexported methods:

- `parseFrontmatter(content []byte, target interface{}) error` → add `ctx context.Context` as first param
- `serializeWithFrontmatter(frontmatter interface{}, originalContent string) (string, error)` → add `ctx context.Context` as first param
- `findFileByName(dir string, name string) (string, string, error)` → add `ctx context.Context` as first param

Replace error returns in `parseFrontmatter`:
- `return fmt.Errorf("no frontmatter found")` → `return errors.Errorf(ctx, "no frontmatter found")`
- `return fmt.Errorf("unmarshal yaml: %w", err)` → `return errors.Wrap(ctx, err, "unmarshal yaml")`

Replace error returns in `serializeWithFrontmatter`:
- `return "", fmt.Errorf("marshal yaml: %w", err)` → `return "", errors.Wrap(ctx, err, "marshal yaml")`

Replace error returns in `findFileByName`:
- `return "", "", fmt.Errorf("read directory %s: %w", dir, err)` → `return "", "", errors.Wrap(ctx, err, fmt.Sprintf("read directory %s", dir))`
- `return "", "", fmt.Errorf("file not found: %s", name)` → `return "", "", errors.Errorf(ctx, "file not found: %s", name)`

Also in `readTaskFromPath` — update call: `b.parseFrontmatter(content, task)` → `b.parseFrontmatter(ctx, content, task)`

Add `"context"` to imports if not present.

## 2. Update all callers of base helpers

Every call to `parseFrontmatter`, `serializeWithFrontmatter`, `findFileByName` must now pass `ctx`. All callers already have `ctx`:

In `pkg/storage/task.go`:
- `t.serializeWithFrontmatter(task, task.Content)` → `t.serializeWithFrontmatter(ctx, task, task.Content)`
- `t.findFileByName(tasksDir, name)` → `t.findFileByName(ctx, tasksDir, name)`

In `pkg/storage/decision.go`:
- `d.parseFrontmatter(content, decision)` → `d.parseFrontmatter(ctx, content, decision)`
- `d.serializeWithFrontmatter(decision, decision.Content)` → `d.serializeWithFrontmatter(ctx, decision, decision.Content)`

In `pkg/storage/theme.go`:
- `t.parseFrontmatter(content, theme)` → `t.parseFrontmatter(ctx, content, theme)`
- `t.serializeWithFrontmatter(theme, theme.Content)` → `t.serializeWithFrontmatter(ctx, theme, theme.Content)`

In `pkg/storage/goal.go`:
- `g.parseFrontmatter(content, goal)` → `g.parseFrontmatter(ctx, content, goal)`
- `g.serializeWithFrontmatter(goal, goal.Content)` → `g.serializeWithFrontmatter(ctx, goal, goal.Content)`
- `g.findFileByName(goalsDir, name)` → `g.findFileByName(ctx, goalsDir, name)`

## 3. Wrap bare returns in Find* methods

In `pkg/storage/task.go` `FindTaskByName`:
- `return nil, err` → `return nil, errors.Wrap(ctx, err, "find task file")`

In `pkg/storage/goal.go` `FindGoalByName`:
- `return nil, err` → `return nil, errors.Wrap(ctx, err, "find goal file")`

In `pkg/storage/decision.go` `FindDecisionByName`:
- `return nil, err` (after ListDecisions) → `return nil, errors.Wrap(ctx, err, "list decisions")`
- `return nil, fmt.Errorf("invalid decision name: %s", name)` → `return nil, errors.Errorf(ctx, "invalid decision name: %s", name)`
- `return nil, fmt.Errorf("decision not found: %s", name)` → `return nil, errors.Errorf(ctx, "decision not found: %s", name)`
- `return nil, fmt.Errorf("ambiguous match: %q matches %d decisions: %s", ...)` → `return nil, errors.Errorf(ctx, "ambiguous match: %q matches %d decisions: %s", ...)`

## 4. `pkg/cli/cli.go` — Replace fmt.Errorf with errors.Wrap

In all RunE closures, replace:
- `return fmt.Errorf("get vaults: %w", err)` → `return errors.Wrap(ctx, err, "get vaults")`
- `return fmt.Errorf("task not found in any vault: %w", lastErr)` → `return errors.Wrap(ctx, lastErr, "task not found in any vault")`
- `return fmt.Errorf("decision not found in any vault: %w", lastErr)` → `return errors.Wrap(ctx, lastErr, "decision not found in any vault")`
- Apply same pattern to ALL `fmt.Errorf("...: %w", err)` in RunE closures

Add `"github.com/bborbe/errors"` to imports.

Keep `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` in `Execute()` unchanged — that's the slog prompt's scope.

</requirements>

<verification>
make precommit
</verification>
