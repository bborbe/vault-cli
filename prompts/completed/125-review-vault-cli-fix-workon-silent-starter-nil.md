---
status: completed
summary: Fixed silent failure in handleClaudeSession when starter is nil — now returns an error that Execute wraps as a warning instead of empty session_id with success:true
container: vault-cli-exec-125-review-vault-cli-fix-workon-silent-starter-nil
dark-factory-version: v0.171.1-3-gd94f1fa
created: "2026-05-24T00:00:00Z"
queued: "2026-05-24T12:31:43Z"
started: "2026-05-24T12:31:44Z"
completed: "2026-05-24T12:34:43Z"
---

<summary>
- Surface claude session start failure when `ClaudeSessionStarter` is nil (script not found in PATH)
- Currently silently returns empty session_id with `success: true`, breaking callers like task-orchestrator
- Add warning to `MutationResult.Warnings` so callers can detect the failure
</summary>

<objective>
Fix silent failure in `pkg/ops/workon.go` `handleClaudeSession` when `w.starter == nil` (which happens when `exec.LookPath(claudeScript)` fails in `NewClaudeSessionStarter`). Today the function returns `("", nil)` if the task has no existing `claude_session_id`, and `workOnOperation.Execute` then reports `success: true` with an empty `session_id` and no warnings. Callers like task-orchestrator interpret an empty session_id as a hard error but have no diagnostic to act on.

After this fix, when starter is nil AND the task has no cached session_id, return an error explaining the script was not found. `Execute` already wraps `handleClaudeSession` errors as warnings, so the JSON output will surface the cause.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes:
- `pkg/ops/workon.go` (lines 113-162) — see `handleClaudeSession` and how `Execute` consumes its error
- `pkg/ops/claude_session.go` (lines 26-37) — `NewClaudeSessionStarter` returns nil when `LookPath` fails
- `pkg/ops/workon_test.go` — existing tests for starter behavior

Real-world failure trace:
1. task-orchestrator subprocess has restricted PATH that excludes the dir containing the user's claude wrapper script
2. vault-cli's `NewClaudeSessionStarter("claude-obsidian-personal.sh")` → `exec.LookPath` fails → returns nil
3. `workOnOperation.Execute` calls `handleClaudeSession`; starter is nil and task has no cached session_id → returns `("", nil)`
4. JSON output: `{"success": true, "session_id": ""}` — no warning, no error
5. task-orchestrator throws `RuntimeError("vault-cli work-on returned no session_id")` with no way to diagnose
</context>

<requirements>
### 1. Update `handleClaudeSession` in `pkg/ops/workon.go`

Current code (lines ~140-150):
```go
func (w *workOnOperation) handleClaudeSession(
    ctx context.Context,
    task *domain.Task,
    vaultPath string,
) (string, error) {
    if w.starter == nil {
        return task.ClaudeSessionID(), nil
    }
    if task.ClaudeSessionID() != "" {
        return task.ClaudeSessionID(), nil
    }
    ...
}
```

Change to:
```go
func (w *workOnOperation) handleClaudeSession(
    ctx context.Context,
    task *domain.Task,
    vaultPath string,
) (string, error) {
    if existing := task.ClaudeSessionID(); existing != "" {
        return existing, nil
    }
    if w.starter == nil {
        return "", errors.New(ctx, "claude session starter unavailable — claude script not found in PATH")
    }
    ...
}
```

Use `errors.New` from `github.com/bborbe/errors` (already imported in the file).

### 2. Update tests in `pkg/ops/workon_test.go`

The existing `Context("when starter is nil")` (around line 111) must be split into two sub-contexts with distinct assertions:

**Sub-context A: starter nil AND task has no cached session_id**
- `Execute` returns nil error (warnings, not hard error)
- `MutationResult.Success` is `true`
- `MutationResult.SessionID` is `""`
- `MutationResult.Warnings` contains an entry whose substring is `"claude session: claude session starter unavailable"` — assert via Gomega `ContainSubstring`, not full-string equality (the `errors.New` output may carry extra context). The `"claude session: "` prefix is added by `Execute` at lines ~113-118 when wrapping `handleClaudeSession`'s error.

**Sub-context B: starter nil BUT task already has cached `claude_session_id`**
- `Execute` returns nil error
- `MutationResult.Success` is `true`
- `MutationResult.SessionID` equals the cached value
- `MutationResult.Warnings` is empty (no warning emitted because the cached id short-circuits before the starter-nil check)

Remove or replace the old combined `Context("when starter is nil")` — do not leave it stale alongside the new sub-contexts.

### 3. No other behavior changes

The `Execute` method already converts `handleClaudeSession` errors into warnings (lines ~113-118). No changes needed there.
</requirements>

<constraints>
- Only change files in vault-cli repo
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Use `github.com/bborbe/errors` — never `fmt.Errorf` or bare error construction
- Do not change the `NewClaudeSessionStarter` contract (still returns nil on LookPath fail) — fix is at the call site
- Do NOT touch unrelated `fmt.Errorf` calls in `pkg/ops/claude_session.go` (lines 89, 102) — those are out of scope for this prompt
</constraints>

<verification>
```
make precommit
```
</verification>
