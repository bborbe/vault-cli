---
status: approved
created: "2026-05-24T00:00:00Z"
queued: "2026-05-25T12:34:04Z"
---

<summary>
- Replace all fmt.Errorf with errors.Errorf/errors.Wrap in pkg/config/config.go
- Fix goerr113-suppressed errors with proper context wrapping
- Ensure all error wrapping includes context about what operation was being performed
</summary>

<objective>
Fix fmt.Errorf usage in pkg/config/config.go. All errors in business logic must use github.com/bborbe/errors with proper context wrapping. This file has ~13 fmt.Errorf violations that need fixing.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read `go-error-wrapping-guide.md` for proper error wrapping patterns.

Files to read before making changes:
- `pkg/config/config.go` — all fmt.Errorf usages (lines ~171, 184, 208, 220, 227, 235, 250, 262, 272, 279, 293, 316, 319, 336, 350)
</context>

<requirements>
### 1. Fix pkg/config/config.go

Read the file to find all fmt.Errorf usages. For each occurrence:

- If it's wrapping an existing error (has %w): use `errors.Wrapf(ctx, err, "operation description %s", param)`
- If it's creating a new error (no %w): use `errors.Errorf(ctx, "error message")`

Common patterns to fix:
- `fmt.Errorf("read config file %s: %w", configPath, err)` → `errors.Wrapf(ctx, err, "read config file %s", configPath)`
- `fmt.Errorf("load config: %w", err)` → `errors.Wrap(ctx, err, "load config")`
- `fmt.Errorf("current_user not configured")` → `errors.Errorf(ctx, "current_user not configured")`
- `fmt.Errorf("vault not found: %s", vaultName)` → `errors.Errorf(ctx, "vault not found: %s", vaultName)`

### 2. Add errors import

Ensure `github.com/bborbe/errors` is imported.

### 3. Remove fmt import if no longer needed

After fixing all fmt.Errorf usages, check if fmt is still used elsewhere in the file. If fmt is only used for Errorf, remove the import.
</requirements>

<constraints>
- Only change files in this repo
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Use `errors.Wrap`/`errors.Errorf` from `github.com/bborbe/errors` — never `fmt.Errorf`
</constraints>

<verification>
```
make precommit
```
</verification>
