---
status: draft
created: "2026-05-24T00:00:00Z"
---

<summary>
- Fix bare `return err` in storage walk callbacks that lack context
- Add proper error wrapping with context about which vault/file was being processed
- Ensure all error paths include meaningful context for debugging
</summary>

<objective>
Fix bare `return err` statements in pkg/storage/ that don't wrap errors with context. Walk callbacks need to indicate which vault path was being processed when the error occurred.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read `go-error-wrapping-guide.md` for proper error wrapping patterns.

Files to read before making changes:
- `pkg/storage/decision.go` (~line 90)
- `pkg/storage/task.go` (~line 85)
- `pkg/storage/base.go` (~line 99)
- `pkg/ops/lint.go` (~line 88)
</context>

<requirements>
### 1. Fix pkg/storage/decision.go line 90

The `filepath.WalkDir` callback returns bare `err`. Change to:
```go
return errors.Wrapf(ctx, err, "walk vault %s", vaultPath)
```

### 2. Fix pkg/storage/task.go line 85

Fix the bare `return err` in the walk callback:
```go
return errors.Wrapf(ctx, err, "walk tasks dir")
```

### 3. Fix pkg/storage/base.go line 99

Fix the bare `return err` in the walk callback:
```go
return errors.Wrapf(ctx, err, "walk directory")
```

### 4. Fix pkg/ops/lint.go line 88

The `filepath.Walk` callback returns bare `err`:
```go
return errors.Wrapf(ctx, err, "walk %s", tasksDirPath)
```

### 5. Ensure errors import

Add `github.com/bborbe/errors` import where needed.
</requirements>

<constraints>
- Only change files in this repo
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Use `errors.Wrapf` from `github.com/bborbe/errors` — never bare `return err`
</constraints>

<verification>
```
make precommit
```
</verification>
