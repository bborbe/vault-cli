---
spec: [041-bug-resume-races-live-headless-turn]
status: draft
created: "2026-09-11T12:00:00Z"
---

<summary>
- Confirms the non-interactive branch of `StartSession` already blocks until the detached headless turn exits and validates its `--output-format json` blob through the shared `validateSessionTurn` helper, so an id is never handed over while the transcript is still being written — the spec-041 design is present in the tree (plus the spec-042 per-session lock and spec-045 result-outranks-exit-code precedence), and this prompt corrects nothing that is already correct.
- Confirms the 30-minute `sessionTurnTimeout` bound (a wait bound, never a kill: the child is detached and survives expiry) is wired through `runDetachedTurn`'s select, and that the old `livenessWindow` identifier no longer appears anywhere in `pkg/`.
- Confirms the interactive TTY branch, `defaultCommandRunner` (== 3), the 5m `context.WithTimeout` cap (== 1), `mocks/claude-session-starter.go`, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched (AC10).
- Confirms the AC3/AC4/AC5/AC6 unit-test matrix and the `claude_session_detach_test.go` integration test already exist and assert the spec's error strings ("did not complete within", "wait cancelled", "exited with error", "parse claude output", "num_turns", "is_error").
- Backfill 1: renames the AC1 test's bare `window` variable to `capturedWindow` so the spec's AC1 evidence grep (`Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))`) matches — a pure rename with no behavior change; the assertion already exists under the other name.
- Backfill 2: adds the one genuinely missing test — the temp output file is removed after a clean exit (AC2's "the temp file is removed" half).
- Flags spec artifacts for the human reviewer: AC3's evidence grep `'"0 turns"'` and AC4/AC5's trailing-quote greps can never match the real (post-spec-045) error strings in the tree; this prompt verifies the real, unquoted forms instead and forbids editing error strings to force the quoted greps.
- Runs `make test` and the AC10 `git diff --exit-code HEAD` guard for `scenarios/005` (git IS available in this container — `.dark-factory.yaml` is `workflow: direct`, no hideGit).
</summary>

<objective>
Confirm — and backfill the two genuinely missing pieces — that the non-interactive branch of `StartSession` blocks until the detached headless turn exits, captures and validates its JSON, and returns an error on every failure path, so the Vault UI never offers Resume against a live or failed transcript. This prompt covers spec-041 ACs 1-6 and 10, and is the foundation prompts 2 and 3 build on.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/claude_session.go` — the whole file. This is the file under test. The spec-041 target state (plus spec-042 lock, spec-045 precedence) is already implemented here; your job is to CONFIRM each piece against the requirements below, not to rewrite it.
- `pkg/ops/export_test.go` — exposes the unexported constant as `SessionTurnTimeout` (plus spec-042's `DefaultSessionLockDir`).
- `pkg/ops/claude_session_test.go` — the whole file. The "non-interactive branch" context starts at line 256; the "session lock lifecycle" context (spec 042) starts at line 625.
- `pkg/ops/claude_session_detach_test.go` — the detachment integration test.
- `docs/work-on-session-lifecycle.md` — the durable design record this implementation realizes (its task-path prose is reworded by prompt 3; do NOT edit the doc in this prompt).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`; never `fmt.Errorf`, never bare `return err`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — why the raw `go func`s in this file are deliberate (documented inline).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no hideGit). The AC10 `git diff --exit-code HEAD -- scenarios/005-...` guard runs here — it is NOT operator-side.
</context>

<requirements>
The target state for this prompt already exists in the tree (it shipped with spec-041 and was refined by specs 042 and 045). Your job is to CONFIRM each piece matches the spec-041 Design below, and to BACKFILL the two specific gaps named in requirements 9 and 10. Do not rewrite what is already correct — "confirm" means read the actual source and verify it matches; correct only genuine mismatches, which are not expected.

1. **Confirm the constant.** In `pkg/ops/claude_session.go` the unexported constant must be:
   ```go
   const sessionTurnTimeout = 30 * libtime.Minute
   ```
   with a doc comment stating it bounds the wait for the detached turn's exit, is never a kill (the child is detached and survives expiry), and is a tunable constant with no config field (spec Open Question 1: resolved as a const — do NOT add a config field). `livenessWindow` must not appear anywhere in `pkg/` (`grep -rn 'livenessWindow' pkg/` returns nothing). The constant is present today — confirm it; do not redefine it.

2. **Confirm `defaultDetachedRunner`.** Its signature must be:
   ```go
   func defaultDetachedRunner(args []string, dir string, stdout *os.File) (<-chan error, error)
   ```
   It must use `exec.Command` (NOT `exec.CommandContext`), set `cmd.Stdout = stdout` (the caller-owned temp file — the function must NOT close it), set `cmd.Stderr` to an `os.OpenFile(os.DevNull, os.O_WRONLY, 0)` handle (closed only after the child exits, inside the reaper goroutine), set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`, log the spawn audit line (`slog.Info("claude detached spawn started", ...)` with pid), and return a buffered `done` channel (capacity 1) that receives `cmd.Wait()`'s error. Confirm all of this; never close the caller-owned stdout file.

3. **Confirm the non-interactive branch.** `StartSession`'s non-interactive path (the `if !isInteractive` branch) must delegate to the `runDetachedTurn` method, which must do, in order:
   - `outFile, err := os.CreateTemp("", "vault-claude-session-*.json")`; on error wrap with `"create claude output file"`.
   - Eager unlink + close via `defer` (`_ = os.Remove(outFile.Name())`, `_ = outFile.Close()`) so no temp file survives any return path, including cancel/timeout while the child still holds the fd.
   - `done, err := c.detachRun(args, cwd, outFile)`; on error wrap with `"start detached claude session"`.
   - A waiter goroutine feeding a buffered channel: `waitCh <- c.waiter.Wait(ctx, c.sessionTurnTimeout)`.
   - A `select` with exactly these outcomes, ALL of which except a clean exit are errors:
     - `case exitErr := <-done:` — child exited. Read `os.ReadFile(outFile.Name())` (wrap with `"read claude output"`) then `validateSessionTurn(ctx, output)`. When the output parses and validates, a non-zero `exitErr` is overridden by the validated result (spec-045 precedence — log it via `slog.Warn`), and only an unparseable output falls back to `"claude session exited with error: %v"`. Confirm this exact branch, including the `errors.Is(validateErr, errClaudeOutputUnparseable)` sentinel check (`var errClaudeOutputUnparseable = stderrors.New("claude output is not valid turn JSON")`).
     - `case err := <-waitCh:` — `err != nil` (ctx cancelled) → `errors.Wrap(ctx, err, "claude session wait cancelled")`; `err == nil` (bound expired) → `errors.Errorf(ctx, "claude session turn did not complete within %v", c.sessionTurnTimeout)`. The child is detached and survives either way.
   - The old strings `"claude session start timed out"` and `"exited during startup"` must not exist anywhere in the repo (`grep -rn 'exited during startup' pkg/` returns nothing). Do NOT use `exec.CommandContext` — the child must survive the parent.

4. **Confirm `validateSessionTurn` extraction.** The helper
   ```go
   func validateSessionTurn(ctx context.Context, output []byte) error
   ```
   must exist and be called from BOTH branches (the interactive branch via `c.runCmd` output at line ~223, the non-interactive branch from the read temp file inside `runDetachedTurn`). Its checks and the error strings in the CURRENT tree (post-spec-045, via the `rejectTurn` helper) are authoritative:
   - `json.Unmarshal` failure → `errors.Wrapf(ctx, errClaudeOutputUnparseable, "parse claude output: %v", err)`
   - empty `session_id` → `rejectTurn(ctx, result.Result, "claude returned empty session_id")`
   - `num_turns == 0` → `rejectTurn(ctx, result.Result, "claude returned num_turns: 0")`
   - `is_error == true` → `rejectTurn(ctx, result.Result, "claude reported is_error: true")`
   - otherwise nil. It must not return nil on a dead session: a `session_id` alone proves nothing.
   The interactive branch must otherwise be byte-identical to today: `defaultCommandRunner` unchanged, the 5m `context.WithTimeout(ctx, 5*time.Minute)` cap, `"claude bootstrap turn timed out after 5m"`, and `"run claude"` wrap. Do NOT "simplify" `validateSessionTurn` back to the spec Design's older inline strings (`"claude returned 0 turns: %s"`, `"claude reported error: %s"`) — the `rejectTurn` forms in the tree are the shipped behavior and are locked by the existing tests.

5. **Confirm `export_test.go`.** It must contain
   ```go
   const SessionTurnTimeout = sessionTurnTimeout
   ```
   with a comment noting it is a test-only alias (locks wiring, not value — tests must also assert the literal `30 * libtime.Minute`). The file also carries `var DefaultSessionLockDir = defaultSessionLockDir` (spec 042's export) — that is expected and must be left untouched.

6. **Confirm the unit-test matrix exists** in `pkg/ops/claude_session_test.go`'s "non-interactive branch" context (starts at line 256):
   - AC1 — "blocks until the detached child exits": blocking waiter, `doneCh` only fires after a `Consistently(returned, "100ms").ShouldNot(Receive())`, then `Eventually(returned).Should(Receive(BeNil()))`; the waiter receives the bound via `windowCh` and asserts it equals `ops.SessionTurnTimeout` AND `30 * libtime.Minute`.
   - AC2 — a clean exit (`doneCh <- nil` with valid JSON written to stdout) returns nil ("passes the session id and name to the detached runner").
   - AC3 — "validates the turn and rejects a zero-turn result", "validates the turn and rejects an is_error result", "rejects an unparseable turn result".
   - AC4 — "treats a child exit error as an error": error containing `"exit status 1"` AND `"exited with error"`; no assertion anywhere still uses the old `"exited during startup"` string.
   - AC5 — "treats the turn timeout as an error so no id is persisted" and "fails on timeout even when a valid blob is already on disk" (the regression lock that the read must never be hoisted above the select).
   - AC6 — "treats context cancellation as an error so no id is persisted" (error containing `"wait cancelled"`, NOT nil) plus "wraps a spawn failure" (`"start detached claude session"`).
   The spec-045 precedence specs ("returns nil when the child writes a valid blob and exits non-zero", "leads with the child's reason when a parsed blob reports is_error", "names the exit status when the output is non-empty but unparseable", etc.) and the "session lock lifecycle" context (spec 042, line 625) are expected to be present — leave them untouched. The existing interactive-branch tests (lines 54-254) must be UNCHANGED — they lock the shared validation behavior.

7. **Confirm the detachment integration test.** `pkg/ops/claude_session_detach_test.go` must contain "child outlives a cancelled parent wait": it spawns a real script (`#!/bin/sh\nsleep 6\ntouch <sentinel>`), cancels the context after ~500ms, asserts `StartSession` returns an error, asserts the sentinel does NOT exist yet, and then `Eventually(..., "20s", "200ms")` asserts the sentinel appears — proving the detached child survived the parent's cancelled wait. The file constructs the starter via `ops.NewClaudeSessionStarter(script, ops.NewSessionLockerWithDir(lockDir))` (the two-arg form is spec 042's; keep it). Confirm, do not change.

8. **Confirm the interface and mock are unchanged.** `ClaudeSessionStarter.StartSession` keeps its 6-arg signature `(ctx context.Context, sessionID string, prompt string, cwd string, name string, isInteractive bool) error` and `mocks/claude-session-starter.go` is untouched (spec Constraint: "Interface / mocks — signature unchanged"). Confirm `grep -c 'StartSession' mocks/claude-session-starter.go` — the mock defines `StartSession` with the 6-arg signature; do not regenerate it.

9. **BACKFILL — rename the test variable to satisfy AC1's evidence grep.** In `pkg/ops/claude_session_test.go`, inside the "blocks until the detached child exits" spec (lines 340-347), the local variable is currently named `window`. Rename it to `capturedWindow` so the assertion line reads exactly `Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))`. The block changes old → new as follows (keep `windowCh` — the channel — as-is; only the bare `window` variable is renamed):
   ```go
   // OLD
   			var window libtime.Duration
   			Expect(windowCh).To(Receive(&window))
   			// Locks the wiring: StartSession hands the constant, not a stray literal.
   			Expect(window).To(Equal(ops.SessionTurnTimeout))
   			// Locks the value: SessionTurnTimeout is an alias, so the line above moves
   			// with the constant and would survive any retune. This line is the one that
   			// fails when the bound is changed.
   			Expect(window).To(Equal(30 * libtime.Minute))
   ```
   ```go
   // NEW
   			var capturedWindow libtime.Duration
   			Expect(windowCh).To(Receive(&capturedWindow))
   			// Locks the wiring: StartSession hands the constant, not a stray literal.
   			Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))
   			// Locks the value: SessionTurnTimeout is an alias, so the line above moves
   			// with the constant and would survive any retune. This line is the one that
   			// fails when the bound is changed.
   			Expect(capturedWindow).To(Equal(30 * libtime.Minute))
   ```
   Do not rename the `windowCh` channel or any other identifier. This is a pure rename; no behavior changes.

10. **BACKFILL — assert the temp output file is removed after a clean exit.** No existing test asserts AC2's "the temp file is removed" half. Add ONE dedicated spec as the LAST `It` in the "non-interactive branch" context of `pkg/ops/claude_session_test.go` (immediately after "wraps a spawn failure", before the context closes at line 623). This requires adding `path/filepath` to the file's imports (current import block: `context`, `errors`, `os`, `time`, `libtime`, `uuid`, ginkgo, gomega, `ops`). Add this spec verbatim:
    ```go
    It("removes the temp output file after a clean exit", func() {
        matches := func() []string {
            files, _ := filepath.Glob(filepath.Join(os.TempDir(), "vault-claude-session-*.json"))
            return files
        }
        before := matches()
        bw := blockWaiter
        starter = ops.NewClaudeSessionStarterWithRunner(
            "/usr/local/bin/claude",
            nil,
            func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
                if stdout != nil {
                    _, _ = stdout.WriteString(validTurnJSON)
                }
                done := make(chan error, 1)
                done <- nil
                return done, nil
            },
            libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
                <-bw
                return nil
            }),
            locker,
        )
        Expect(starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)).To(BeNil())
        Expect(matches()).To(Equal(before))
    })
    ```
    `validTurnJSON`, `blockWaiter`, `starter`, `ctx`, and `locker` are all in scope in that context. The eager unlink in `runDetachedTurn` runs before `StartSession` returns, so the before/after glob counts must be equal. The spec overrides `starter` with its own fake (writing `validTurnJSON` then `done <- nil`) so the child-exit branch wins and the blocking waiter goroutine stays parked on `<-bw` until `DeferCleanup` closes `blockWaiter` — the established pattern in this context.

11. **Self-check against AC1-6 and AC10.** Before finishing, re-read the changed hunks and walk each AC: the constant, the wait-select, the validation helper, the rename, and the new cleanup test. Run the `<verification>` block and confirm every grep that is expected to pass does pass. The three greps flagged in `<verification>` as spec artifacts (`"0 turns"`, the trailing-quote forms of the AC4/AC5 strings) must NOT be "fixed" by editing error strings or tests — they read 0 against correct, already-shipped code.

Failure-mode coverage from the spec's table, all already covered by existing tests in this file: bound expiry (row 1, AC5 tests), ctx cancel mid-wait with child survival (row 2, AC6 unit + detach integration), child exits non-zero (row 3, AC4 test), turn JSON is_error / 0 turns (row 4, AC3 tests), temp file unreadable/empty (row 5, AC3 unparseable test + the new cleanup test's counterpart), UI request timeout < turn (row 8, AC6 cancel path).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff --exit-code HEAD` reads only; do not stage or commit anything.
- Interactive branch behavior unchanged. `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched. Do NOT re-extract or change `validateSessionTurn`'s call sites beyond confirming them.
- Detachment preserved: `exec.Command` (NOT `CommandContext`), `Setpgid`, stdout/stderr handling that lets the child survive the parent. NEVER SIGKILL the child on timeout; `--max-turns` is inert (`-1`), so the 30-min bound is a wait-channel select, not a context kill. Do NOT resurrect `"claude session start timed out"`.
- Never offer a broken Resume: on any failure (exit error, `is_error`, 0 turns, bound expiry, ctx cancel) `StartSession` returns an error. Returning nil on ctx-cancel is wrong — it would persist an id for a still-running child.
- JSON validation: `num_turns > 0` AND `is_error == false`. Lowercase UUIDs; keep `-n "<task name>"` at mint so resume inherits the title.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- `sessionTurnTimeout` stays a tunable const — do NOT add a config field (spec Open Question 1 recommends const; no second caller exists).
- Do NOT alter error strings or test assertions to satisfy a grep pattern. The AC3 `"0 turns"` evidence grep and the AC4/AC5 trailing-quote greps in the spec's Verification are artifacts of the pre-spec-045 error strings; the real strings in the tree are correct as written (see `<verification>`).
- `ClaudeSessionStarter.StartSession` signature is UNCHANGED — `mocks/claude-session-starter.go` is untouched. The `SessionLocker` constructor parameter (spec 042) is already wired in and must stay.
- Do NOT touch `pkg/ops/workon.go`, `pkg/ops/goal_workon.go`, or `docs/work-on-session-lifecycle.md` in this prompt — the workon reorder is prompt 2, the doc reword is prompt 3.
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — spec evidence greps. Run each and record the count. Rows expected >= 1 must hold; rows expected == 0 must be 0:

```
grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                        # >= 1 (source identifier, lowercase)
grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go                 # >= 1 (value pin)
grep -c 'validateSessionTurn' pkg/ops/claude_session.go                       # >= 2 (both branches call it)
grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go  # >= 1 — BACKFILL REQ 9; must flip 0 -> 1
grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go          # >= 1 — BACKFILL REQ 10; must flip 0 -> 1
grep -c 'num_turns' pkg/ops/claude_session_test.go                            # >= 1 (AC3 real check)
grep -c 'claude session exited with error' pkg/ops/claude_session.go          # >= 1 (AC4 real check — unquoted form)
grep -c 'exited during startup' pkg/ops/claude_session.go                     # == 0 (AC4)
grep -c 'did not complete within' pkg/ops/claude_session.go                   # >= 1 (AC5 real check — unquoted form)
grep -c 'wait cancelled' pkg/ops/claude_session.go                            # >= 1 (AC6)
grep -c 'livenessWindow' -r pkg/                                              # == 0
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go                      # == 3 (AC10)
grep -c 'context.WithTimeout' pkg/ops/claude_session.go                       # == 1 (AC10, interactive branch)
```

Note on spec-quoting artifacts (do NOT "fix" these): the spec's AC3 evidence grep uses the literal `'"0 turns"'` and AC4/AC5 use `'"claude session exited with error"'` / `'"did not complete within"'` (a trailing double-quote inside the pattern). Against the CURRENT tree these read 0 even on CORRECT code: the shipped `rejectTurn` strings are `"claude returned num_turns: 0"` / `"claude reported is_error: true"`, and the exit-error / bound strings are `"claude session exited with error: %v"` / `"claude session turn did not complete within %v"`. The unquoted forms above are the real checks and must pass. Do NOT edit error strings or test bodies to force the quoted greps.

SECONDARY — AC10 git guard (git IS available — workflow `direct`, no hideGit):
```
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md   # must exit 0 with empty output
```

SYNTAX + TESTS:
```
gofmt -e -l pkg/ops/claude_session.go pkg/ops/claude_session_test.go pkg/ops/export_test.go   # must list NO files
make test                                                                                    # must exit 0
```

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running `make test` + the grep gate here is correct; the daemon's preflight already runs `make precommit` before execution.
</verification>
