---
status: completed
summary: Replaced all fmt.Errorf with errors.Wrapf/errors.Errorf in pkg/ops/ files to enable context-enriched error tracing
container: vault-cli-exec-134-review-vault-cli-fix-fmt-errorf-in-ops
dark-factory-version: v0.171.1-3-gd94f1fa
created: "2026-05-24T00:00:00Z"
queued: "2026-05-25T12:34:04Z"
started: "2026-05-25T12:42:46Z"
completed: "2026-05-25T12:48:03Z"
---

<summary>
- Replace all fmt.Errorf with errors.Errorf/errors.Wrap in pkg/ops files
- Fix goerr113-suppressed errors in goal_complete.go, objective_complete.go, defer.go
- Ensure all error wrapping includes context about what operation was being performed
</summary>

<objective>
Fix fmt.Errorf usage in pkg/ops/ files. All errors in business logic must use github.com/bborbe/errors with proper context wrapping. This is a code quality fix to ensure consistent error handling and meaningful stack traces.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read `go-error-wrapping-guide.md` for proper error wrapping patterns.

Files to read before making changes:
- `pkg/ops/watch.go` (~line 71)
- `pkg/ops/claude_session.go` (~lines 89, 102)
- `pkg/ops/claude_resume.go` (~line 58)
- `pkg/ops/lint.go` (~lines 140, 157, 641)
- `pkg/ops/complete.go` (~line 315)
- `pkg/ops/objective_complete.go` (~line 65)
- `pkg/ops/goal_complete.go` (~lines 71, 133)
- `pkg/ops/defer.go` (~line 101)
- `pkg/ops/goal_defer.go` (~line 63)
- `pkg/ops/defer_date_parser.go` (~lines 23, 54)
- `pkg/ops/frontmatter_entity.go` (~lines 346, 349, 393, 396, 440, 443, 490, 493, 566, 569, 606, 621)
</context>

<requirements>
### 1. Fix pkg/ops/watch.go

At line 71, change:
```go
return fmt.Errorf("create watcher: %w", err)
```
to:
```go
return errors.Wrap(ctx, err, "create watcher")
```

### 2. Fix pkg/ops/claude_session.go

At line 89, change:
```go
return "", fmt.Errorf("claude session start timed out after 5m")
```
to:
```go
return "", errors.Errorf(ctx, "claude session start timed out after 5m")
```

At line 102, change:
```go
return "", fmt.Errorf("claude returned empty session_id")
```
to:
```go
return "", errors.Errorf(ctx, "claude returned empty session_id")
```

### 3. Fix pkg/ops/claude_resume.go

At line 58, change:
```go
return fmt.Errorf("change directory to %s: %w", cwd, err)
```
to:
```go
return errors.Wrapf(ctx, err, "change directory to %s", cwd)
```

### 4. Fix pkg/ops/lint.go

At line 140, change:
```go
return nil, fmt.Errorf("read file: %w", err)
```
to:
```go
return nil, errors.Wrap(ctx, err, "read file")
```

At line 157, change:
```go
return nil, fmt.Errorf("fix issues: %w", err)
```
to:
```go
return nil, errors.Wrap(ctx, err, "fix issues")
```

At line 641, change:
```go
return issues, fmt.Errorf("write file: %w", err)
```
to:
```go
return issues, errors.Wrap(ctx, err, "write file")
```

### 5. Fix pkg/ops/complete.go

At line 315, change:
```go
return fmt.Errorf(
```
to use errors.Errorf with context wrapping.

### 6. Fix pkg/ops/objective_complete.go

At line 65, change:
```go
return MutationResult{Success: false, Error: msg}, fmt.Errorf("%s", msg) //nolint:goerr113
```
to:
```go
return MutationResult{Success: false, Error: msg}, errors.Errorf(ctx, "%s", msg)
```
Also remove the //nolint:goerr113 comment.

### 7. Fix pkg/ops/goal_complete.go

At lines 71 and 133, change:
```go
return MutationResult{Success: false, Error: msg}, fmt.Errorf("%s", msg) //nolint:goerr113
```
to:
```go
return MutationResult{Success: false, Error: msg}, errors.Errorf(ctx, "%s", msg)
```
Remove the //nolint:goerr113 comments.

### 8. Fix pkg/ops/defer.go

At line 101, change the fmt.Errorf to use errors.Errorf with ctx.

### 9. Fix pkg/ops/goal_defer.go

At line 63, change the fmt.Errorf to use errors.Errorf with ctx.

### 10. Fix pkg/ops/defer_date_parser.go

This file needs ctx propagation. Read the file to understand the call chain and thread ctx through from the callers. Alternatively, change fmt.Errorf to errors.Errorf where ctx is available in the call chain.

### 11. Fix pkg/ops/frontmatter_entity.go

At lines 346, 349, 393, 396, 440, 443, 490, 493, 566, 569, 606, 621:
- Change fmt.Errorf to errors.Errorf with ctx where ctx is available in the method
- Read the file to confirm which methods have ctx in scope

For each location, if the enclosing method has ctx in scope, use errors.Errorf(ctx, ...). If not, thread ctx through the call chain.

### 12. Remove unused fmt import if no longer needed

After making changes, check if fmt is still used. If fmt is only used for Errorf, remove the import.

### 13. Ensure errors import is present

Add `github.com/bborbe/errors` to imports in each file that needs it.
</requirements>

<constraints>
- Only change files in this repo
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Use `errors.Wrap`/`errors.Errorf` from `github.com/bborbe/errors` — never `fmt.Errorf` or bare `return err`
</constraints>

<verification>
```
make precommit
```
</verification>
