---
status: completed
spec: [014-bug-work-on-silent-failure-and-hardcoded-slash-command]
summary: Added ErrStarterUnavailable sentinel, hard/soft failure branching in Execute, updated tests, and updated CHANGELOG
container: vault-cli-exec-130-spec-014-workon-propagate-session-error
dark-factory-version: v0.171.1-3-gd94f1fa
created: "2026-05-24T17:15:00Z"
queued: "2026-05-24T16:22:05Z"
started: "2026-05-24T16:22:08Z"
completed: "2026-05-24T16:28:48Z"
branch: dark-factory/bug-work-on-silent-failure-and-hardcoded-slash-command
---

<summary>
- `vault-cli task work-on` exits non-zero when claude's headless session returns a real failure (zero turns, is_error: true) — surfacing the error to downstream consumers (task-orchestrator UI, scripts)
- The "claude binary missing" case (starter == nil) STAYS a warning — spec 014 Failure Modes table marks that row "Unchanged", preserving v0.66.9 behavior so the task is still assigned + in_progress even when claude isn't installed
- Discriminated via a sentinel error `ErrStarterUnavailable` in `pkg/ops` — handleClaudeSession returns the sentinel for the starter-nil branch and a regular wrapped error for actual session-start failures
- Task frontmatter mutations (`status: in_progress`, `assignee`) STILL happen on disk in both paths — the error only changes the CLI exit code / `MutationResult.Success`
- The interactive path (`--mode interactive`) is unaffected when no session error occurs
- Closes spec 014 AC8 — verifier confirmed exit 0 on the forced unknown-command repro, contradicting the spec's Failure Modes "non-zero exit from CLI" requirement for the `num_turns: 0` row
</summary>

<objective>
Propagate real claude-session failures from `handleClaudeSession` to the CLI exit code so `vault-cli task work-on` exits non-zero when claude was invoked and rejected the request (zero turns, `is_error`). Preserve the existing warning-only behavior for the "claude binary missing" case (starter == nil), which spec 014's Failure Modes table marks Unchanged. Discriminate the two cases via a sentinel error. The task's on-disk frontmatter changes are kept in both paths.
</objective>

<context>
Read CLAUDE.md for project conventions and `docs/development-patterns.md` for the library-vs-CLI boundary (operations return `MutationResult` + error; the CLI maps both into stdout + exit code).

Read these files before making changes — anchor by symbol, not line number:

- `pkg/ops/workon.go` — `workOnOperation.Execute`. The relevant block: after `handleClaudeSession` returns a non-nil `sessionErr`, the current code captures it as a warning and falls through to `return MutationResult{Success: true, ...}, nil`. This is the bug. Task frontmatter writes (`status: in_progress`, `assignee`) happen earlier in `Execute` via `w.taskStorage.WriteTask` — those stay.
- `pkg/ops/workon_test.go` — the existing Ginkgo suite for `Execute`. Look for the test cases that use a `fakeStarter` returning an error and assert the current "warning, but success" behavior — those need to flip.
- `pkg/cli/cli.go` — `createWorkOnCommand` closure. The Cobra `RunE` returns whatever `Execute` returns; non-nil error → Cobra exits non-zero. No change needed here unless the test for the CLI layer needs updating.
- Coding plugin: `~/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, msg)` from `github.com/bborbe/errors` is the project's error-wrapping convention (used throughout `workon.go`).

Note: `Execute` is now invoked with the extra `vault *config.Vault` parameter added in the prior spec-014 prompt — preserve that signature.
</context>

<requirements>
1. In `pkg/ops/` (file: a new `pkg/ops/errors.go` is acceptable, or add to `workon.go` near the top of the file alongside the existing `var` block if one exists), declare an exported sentinel error:
   ```go
   // ErrStarterUnavailable indicates the claude session starter is nil
   // (typically because the configured claude script is not on PATH).
   // This is intentionally a soft failure — work-on still marks the task
   // in_progress on disk; the CLI exits 0 with the error recorded as a warning.
   var ErrStarterUnavailable = errors.New(context.Background(), "claude session starter unavailable — claude script not found in PATH")
   ```
   - Use `errors.New(ctx, ...)` from `github.com/bborbe/errors`. Use `context.Background()` since this is a package-level sentinel.
   - Export it so `Execute` can detect it via `errors.Is` and external callers (tests) can match against it.

2. In `pkg/ops/workon.go` `handleClaudeSession`, replace the inline `errors.New(ctx, ...)` for the starter-nil branch with the sentinel:
   ```go
   if w.starter == nil {
       return "", ErrStarterUnavailable
   }
   ```
   The actual session-start failure path (`return "", errors.Wrap(ctx, err, "start claude session")`) stays unchanged — it remains a wrapped error distinct from the sentinel.

3. In `pkg/ops/workon.go` `Execute`, replace the existing `sessionErr` handling block:
   ```go
   sessionID, sessionErr := w.handleClaudeSession(ctx, task, sessionDir, vault)
   if sessionErr != nil {
       warning := fmt.Sprintf("claude session: %v", sessionErr)
       warnings = append(warnings, warning)
       slog.Warn("workon warning", "warning", warning)
   }
   ```
   with:
   ```go
   sessionID, sessionErr := w.handleClaudeSession(ctx, task, sessionDir, vault)
   if sessionErr != nil {
       if errors.Is(sessionErr, ErrStarterUnavailable) {
           // Soft failure — claude binary missing. Spec 014 Failure Modes table:
           // "Unchanged". Keep as warning, continue, CLI exits 0.
           warning := fmt.Sprintf("claude session: %v", sessionErr)
           warnings = append(warnings, warning)
           slog.Warn("workon warning", "warning", warning)
       } else {
           // Hard failure — claude was invoked and rejected the request
           // (zero turns, is_error). Surface to caller as non-zero CLI exit.
           slog.Warn("workon session error", "error", sessionErr)
           return MutationResult{
               Success:   false,
               Name:      task.Name,
               Vault:     vaultName,
               Warnings:  warnings,
               SessionID: sessionID,
               Error:     sessionErr.Error(),
           }, errors.Wrap(ctx, sessionErr, "start work-on session")
       }
   }
   ```
   - Use `errors.Is` and `errors.Wrap` from `github.com/bborbe/errors` (already imported in this file).
   - Do NOT append the session error to `warnings` on the hard-failure path — it is the primary error.
   - Populate `MutationResult.Error` on the hard-failure path to match the existing error-return convention elsewhere in `Execute` (the `WriteTask` error block sets `Error: err.Error()`).
   - Task frontmatter mutations earlier in `Execute` are unaffected on both paths: the task remains `status: in_progress` and `assignee: <user>` on disk.

4. The two later `return MutationResult{Success: true, ...}` blocks (interactive resume + non-interactive) are reached only when `sessionErr == nil` OR the soft-fail branch took the warning path. They stay unchanged.

5. In `pkg/ops/workon_test.go`, update existing test contexts to match the new semantics. Each requires explicit changes:

   a. `Context("when starter is nil and task has no cached session ID", ...)` (around `workon_test.go:132`):
      - `It("returns no error")` — assertion stays `Expect(err).To(BeNil())` (soft-fail path keeps exit 0).
      - `It("emits warning about missing starter")` — assertion stays valid; the warning string is unchanged (still `"claude session: claude session starter unavailable …"` from the sentinel's `.Error()` text).
      - No further change needed in this context.

   b. `Context("when session start fails", ...)` (around `workon_test.go:209`):
      - `It("still returns no error (session failure is a warning)")` — DELETE this `It` and replace its enclosing context with two `It` blocks that match the new hard-fail behavior:
        - `It("returns wrapped error")` — `Expect(err).To(HaveOccurred())` AND `Expect(err.Error()).To(ContainSubstring("start work-on session"))`.
        - `It("returns Success=false")` — `Expect(result.Success).To(BeFalse())`.
      - Rename the `Context` from `"when session start fails"` to `"when session start fails (hard failure)"` for clarity.

6. Add one new Ginkgo `Context` + `It` block in `pkg/ops/workon_test.go` named `Context("when claude returns zero turns")` containing `It("returns non-nil error wrapped with start work-on session and Success=false")`:
   - Configure the existing `mockStarter` (a counterfeiter `FakeClaudeSessionStarter` already used elsewhere in the file) so `StartSession` returns `("", errors.New(ctx, "claude returned 0 turns: Unknown command: /x"))`. Use `errors.New` from `github.com/bborbe/errors`.
   - Call `Execute` with a valid task and vault.
   - Asserts: returned error is non-nil AND `Error()` contains BOTH `"start work-on session"` AND `"Unknown command: /x"`.
   - Asserts: `result.Success == false`.
   - Asserts: the in-memory task passed to `mockTaskStorage.WriteTask` had `Status: "in_progress"` (lock down the "task frontmatter mutations persist even on session failure" invariant — use `mockTaskStorage.WriteTaskCallCount()` + `WriteTaskArgsForCall` if a counterfeiter mock is in use, or inspect the test's existing task fixture).

7. If any test in `pkg/cli/` (CLI layer) drives `task work-on` end-to-end and currently asserts exit 0 when claude fails, update it to assert non-zero. If no such test exists, no action needed. Use `grep -rn 'work-on' pkg/cli/ --include='*_test.go'` to check.

8. Update `CHANGELOG.md`. The top entry is currently `## v0.66.12` (no `## Unreleased` section exists — the autoRelease daemon renamed it on the previous prompt). Insert a new `## Unreleased` heading ABOVE `## v0.66.12` containing:
   ```
   ## Unreleased

   - fix(workon): `task work-on` now exits non-zero when claude's headless session returns an actual failure (zero turns, is_error). The "claude binary missing" case still exits 0 with a warning, preserving v0.66.9 behavior. Closes spec 014 AC8 — the verifier confirmed exit 0 on the forced unknown-command repro before this fix.
   ```
   Do NOT modify existing versioned sections.

9. Run `make precommit` — must pass cleanly. Address any lint findings (formatting, unused imports, unused locals).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Existing happy-path tests must continue to pass.
- The starter-nil (claude binary missing) path MUST stay a soft failure — exit 0 with `Warnings` populated. Spec 014 Failure Modes table marks this row "Unchanged"; this preserves v0.66.9 behavior so a task can still be assigned + in_progress when claude isn't installed.
- `Execute`'s Go signature (parameters + return types `(MutationResult, error)`) MUST NOT change. Only the conditions under which it returns a non-nil error tighten.
- The hard-failure error message must include both the wrap prefix (`start work-on session`) AND the underlying claude failure detail (e.g. `Unknown command: /...`) so downstream callers (task-orchestrator UI) can surface a meaningful message.
- `errors.Wrap(ctx, err, msg)`, `errors.New(ctx, ...)`, and `errors.Is(err, target)` from `github.com/bborbe/errors` — NOT `fmt.Errorf("%w", err)`, NOT `stderrors.New`.
- Task frontmatter (`status: in_progress`, `assignee`) MUST still be written to disk before the error return — `w.taskStorage.WriteTask` is already invoked earlier in `Execute`; do not move or skip it. Applies to BOTH the soft and hard failure paths.
- Do not change the interactive (`--mode interactive`) happy path return.
- Do not append the hard-failure session error to `warnings` — it is the primary error and goes in `MutationResult.Error` + the wrapped return.
- `ErrStarterUnavailable` MUST be exported (capitalized) so tests can use `errors.Is` against it.
</constraints>

<verification>
Run `make precommit` — must pass.

Targeted greps and tests:
- `grep -n 'ErrStarterUnavailable' pkg/ops/` returns ≥3 lines (declaration + use in handleClaudeSession + use in Execute via errors.Is).
- `grep -n 'errors.Wrap(ctx, sessionErr' pkg/ops/workon.go` returns ≥1 line.
- `grep -n '"start work-on session"' pkg/ops/workon.go` returns ≥1 line.
- `grep -n 'when claude returns zero turns' pkg/ops/workon_test.go` returns ≥1 line.
- `grep -n 'start work-on session' pkg/ops/workon_test.go` returns ≥1 line (the new hard-fail test asserts this substring).
- `grep -n '## Unreleased' CHANGELOG.md` returns exactly 1 line, ABOVE `## v0.66.12`.
- `go test ./pkg/ops/... -v` exits 0 and `-v` output lists the new "when claude returns zero turns" `Context` along with the existing starter-nil and session-start contexts.

Runtime repro (the AC8 failure case from spec 014):

```bash
WORK_DIR=$(mktemp -d)
mkdir -p "$WORK_DIR/Tasks"
cat > "$WORK_DIR/config.yaml" <<EOF
current_user: tester
default_vault: AC8Test
vaults:
  AC8Test:
    path: $WORK_DIR
    name: AC8Test
    tasks_dir: Tasks
    work_on_command: /definitely-not-a-real-command-xyz
EOF
cat > "$WORK_DIR/Tasks/test-task.md" <<'EOF'
---
status: next
page_type: task
---
Body.
EOF
vault-cli --config "$WORK_DIR/config.yaml" task work-on test-task --mode headless --vault AC8Test --output json
echo "exit=$?"
rm -rf "$WORK_DIR"
```

Expected output:
- `exit=` line shows a non-zero number (not 0).
- stderr contains `Unknown command: /definitely-not-a-real-command-xyz` or `claude returned 0 turns`.
- stdout JSON (if any) contains `"success": false`.

Soft-failure repro (must still exit 0 — preserves v0.66.9 behavior):

```bash
WORK_DIR=$(mktemp -d)
mkdir -p "$WORK_DIR/Tasks"
cat > "$WORK_DIR/config.yaml" <<EOF
current_user: tester
default_vault: SoftTest
vaults:
  SoftTest:
    path: $WORK_DIR
    name: SoftTest
    tasks_dir: Tasks
    claude_script: /nonexistent/path/to/claude.sh
EOF
cat > "$WORK_DIR/Tasks/test-task.md" <<'EOF'
---
status: next
page_type: task
---
Body.
EOF
vault-cli --config "$WORK_DIR/config.yaml" task work-on test-task --mode headless --vault SoftTest --output json
echo "exit=$?"
rm -rf "$WORK_DIR"
```

Expected output:
- `exit=0` (soft failure — claude binary missing is still tolerated).
- stdout JSON contains `"success": true` and a `warnings` array including `claude session starter unavailable`.
</verification>
