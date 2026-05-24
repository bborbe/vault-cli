---
status: draft
created: "2026-05-24T00:00:00Z"
---

<summary>
- Add symlink escape protection to 5 storage types that are missing it
- The baseStorage already has isSymlinkOutsideVault helper — use it in all storage read/write paths
- Also fix the isSymlinkOutsideVault function to return true (unsafe) on error rather than false
</summary>

<objective>
Fix missing symlink protection in storage implementations. A malicious symlink inside a vault could cause storage operations to read/write files outside the vault directory. The isSymlinkOutsideVault helper exists in baseStorage but is not used by 5 of 6 storage types.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes:
- `pkg/storage/base.go` (~lines 196-214) — existing isSymlinkOutsideVault helper
- `pkg/storage/decision.go` (~line 101) — example of symlink check already in use
- `pkg/storage/task.go` (~line 31) — missing symlink check
- `pkg/storage/goal.go` (~line 28) — missing symlink check
- `pkg/storage/theme.go` (~line 28) — missing symlink check
- `pkg/storage/objective.go` (~line 28) — missing symlink check
- `pkg/storage/vision.go` (~line 28) — missing symlink check
- `pkg/storage/daily_note.go` (~line 26) — missing symlink check
</context>

<requirements>
### 1. Read the existing isSymlinkOutsideVault implementation in base.go

Note: `isSymlinkOutsideVault` is a **package-level function** (NOT a method on `baseStorage`). Signature: `func isSymlinkOutsideVault(path, vaultPath string) bool`. Pass the vault path explicitly at each call site.

Understand how it works and its current error handling behavior (returns false on error, which silently allows broken symlinks).

### 2. Fix isSymlinkOutsideVault to return true on error

The current implementation returns `false` when `filepath.EvalSymlinks` fails, which silently allows broken symlinks. Change the error case to return `true` (treat as unsafe) or at minimum log the error.

```go
// Current: returns false on error
// Should: return true on error (fail safe) or log + return true
```

### 3. Add symlink check to task.go

Read task.go around line 31. Add a call to `isSymlinkOutsideVault(path, vaultPath)` before `os.ReadFile`/`os.WriteFile` operations. If the check returns true (symlink escapes vault), return an error.

### 4. Add symlink check to goal.go

Same pattern as task.go.

### 5. Add symlink check to theme.go

Same pattern.

### 6. Add symlink check to objective.go

Same pattern.

### 7. Add symlink check to vision.go

Same pattern.

### 8. Add symlink check to daily_note.go

Same pattern.

### 9. Verify decision.go already has the check

Read decision.go to confirm it already uses isSymlinkOutsideVault properly. If so, note it as the reference implementation.

### 10. Ensure consistent error messages

All storage implementations should return a similar error when a symlink escape is detected, e.g.:
```go
errors.Errorf(ctx, "symlink outside vault: %s", resolvedPath)
```
</requirements>

<constraints>
- Only change files in this repo
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- The fix must be applied consistently across all storage types
- Use `errors.Errorf` from `github.com/bborbe/errors` for error messages
</constraints>

<verification>
```
make precommit
```
</verification>
