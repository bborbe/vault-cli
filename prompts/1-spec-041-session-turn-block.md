---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-06T16:20:00Z"
---

<summary>
- Confirms the non-interactive branch of `StartSession` already blocks until the detached headless turn exits (a wait-channel select bounded by `sessionTurnTimeout`, never a kill) and validates the turn's JSON on BOTH branches through the shared `validateSessionTurn` helper — the spec-041 session-side is shipped in the tree and NOT reverted. Do not rewrite what is already correct.
- Notes that spec 045's precedence inversion already landed in the same file: a validated turn result outranks a non-zero child exit, `validateSessionTurn`'s rejection messages were reworded (they now lead with the child's `result` text via the `rejectTurn` helper, plus the `errClaudeOutputUnparseable` sentinel), and the detached branch reads + validates the blob before consulting the exit status. This prompt confirms the 041 invariants that SURVIVE that shape and does NOT re-touch the exit-status-vs-result routing (spec 045 owns it).
- Confirms the 30-minute `sessionTurnTimeout` constant, the caller-owned temp-file capture with eager unlink, the `defaultDetachedRunner` signature, `export_test.go`'s `SessionTurnTimeout` accessor, and the AC10 interactive-branch guards (`defaultCommandRunner` == 3, `context.WithTimeout` == 1).
- Backfill 1: renames the turn-bound test variable `window` to `capturedWindow` in `pkg/ops/claude_session_test.go` so the spec's AC1 evidence grep (`Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))`) matches. The assertion already exists under the other name — this is a pure rename, no behavior change.
- Backfill 2: adds the one genuinely missing test — the temp output file is removed after a clean exit (the AC2 "temp file is removed" half). No test currently asserts it.
- Flags a spec artifact: the spec's AC3 evidence grep `'"0 turns"'` cannot match the post-045 validator string (`"claude returned num_turns: 0"`). This prompt verifies the post-045 strings and forbids editing the validator to force the stale grep.
- Runs `make test` + the grep gate. `make precommit` is the batch's full-gate check (AC13) in prompt 3.
</summary>

<objective>
Confirm — and backfill where anything is missing — that the non-interactive branch of `StartSession` blocks until the detached headless turn exits and validates its JSON, so the Vault UI never advertises Resume against a live or failed transcript. This prompt covers spec-041 ACs 1-6 and 10, and is the foundation for prompts 2 and 3.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/claude_session.go` — the whole file. This is the file under test. It already contains BOTH the spec-041 session-side (block-until-exit + temp-file capture + shared validation) AND spec 045's precedence inversion (the read+validate happens before the exit-status check inside the child-exit select branch). Do not undo either.
- `pkg/ops/export_test.go` — exposes the unexported constant.
- `pkg/ops/claude_session_test.go` — the whole file. The "non-interactive branch" context starts at line 256. The "session lock lifecycle" context (spec 042) starts at line 625 and must stay untouched.
- `pkg/ops/claude_session_detach_test.go` — the detachment integration test.
- `docs/work-on-session-lifecycle.md` — the durable design record this implementation realizes. Note its task-path sections still describe the reverted pre-spawn design — that is prompt 3's job to fix; do NOT edit the doc in this prompt.
- `pkg/ops/session_lock.go` — `SessionLocker` / `NewSessionLockerWithDir` / `ErrSessionBusy` / `SessionLock.Release()` (spec 042 wiring already in the file).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`; never `fmt.Errorf`, never bare `return err`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — why the raw `go func`s in this file are deliberate (documented inline).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

IMPORTANT — git is NOT usable in this container: the daemon runs with `hideGit=true` (`.git` is masked, a character device). Do NOT run any `git` command — not to diff, not to check scenario 005. The AC10 `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` guard is an OPERATOR-side check (spec Verification ladder); the container-side proxies are the `defaultCommandRunner` / `context.WithTimeout` greps below.
</context>

<requirements>
The target state for this prompt already exists in the tree (the spec-041 session-side shipped; the v0.118.3 task-side reversion did NOT touch `claude_session.go`, `export_test.go`, or the session tests, and spec 045 has since refined the detached branch). Your job is to CONFIRM each piece matches the spec-041 invariants below, and BACKFILL the two specific gaps named in requirements 8 and 9. Do not rewrite what is already correct — "confirm" means read the actual source and verify it matches; correct only genuine mismatches, which are not expected.

1. **Confirm the constant.** In `pkg/ops/claude_session.go` the unexported constant must be:
   ```go
   const sessionTurnTimeout = 30 * libtime.Minute
   ```
   with a doc comment stating it bounds the wait for the detached turn's exit, is never a kill, and is a tunable constant with no config field (spec Open Question 1: resolved as a const — do NOT add a config field). `livenessWindow` must not appear anywhere in `pkg/` (`grep -rn 'livenessWindow' pkg/` returns nothing). If the constant is missing or mis-named, define it exactly as above.

2. **Confirm `defaultDetachedRunner`.** Its signature must be:
   ```go
   func defaultDetachedRunner(args []string, dir string, stdout *os.File) (<-chan error, error)
   ```
   It must use `exec.Command` (NOT `exec.CommandContext`), set `cmd.Stdout = stdout` (the caller-owned temp file — the function must NOT close it), set `cmd.Stderr` to an `os.OpenFile(os.DevNull, os.O_WRONLY, 0)` handle (closed only after the child exits, inside the reaper goroutine), set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`, log the spawn audit line (`slog.Info("claude detached spawn started", ...)` with pid), and return a buffered `done` channel (capacity 1) that receives `cmd.Wait()`'s error. If any piece differs, correct it to match; never close the caller-owned stdout file.

3. **Confirm `runDetachedTurn`'s spec-041 invariants (do NOT re-touch spec-045's precedence routing).** The method `func (c *claudeSessionStarter) runDetachedTurn(ctx context.Context, args []string, cwd string) error` must have ALL of these, in order:
   - `outFile, err := os.CreateTemp("", "vault-claude-session-*.json")`; on error wrap with `"create claude output file"`.
   - Eager unlink + close via `defer` (`_ = os.Remove(outFile.Name())`, `_ = outFile.Close()`) so no temp file survives any return path, including cancel/timeout while the child still holds the fd.
   - `done, err := c.detachRun(args, cwd, outFile)`; on error wrap with `"start detached claude session"`.
   - A waiter goroutine: `waitCh <- c.waiter.Wait(ctx, c.sessionTurnTimeout)`.
   - A `select` with these outcomes:
     - `case exitErr := <-done:` — the child has exited. It MUST read `outFile` and run `validateSessionTurn` before deciding success. The exit-status-vs-result precedence inside this branch is spec 045's — do NOT change it (the current shape reads + validates, treats a validated result as success even on a non-zero exit, falls back to the exit status only for unparseable output, and logs a `slog.Warn("validated turn result overrides non-zero child exit", ...)` when a validated result wins over a non-zero exit).
     - `case err := <-waitCh:` — both outcomes are errors so the caller persists no session id. `err != nil` (ctx cancelled) → `errors.Wrap(ctx, err, "claude session wait cancelled")`; `err == nil` (bound expired) → `errors.Errorf(ctx, "claude session turn did not complete within %v", c.sessionTurnTimeout)`. The child is detached and survives either way.
   - The old string `"claude session start timed out"` must not exist anywhere in the repo. `"exited during startup"` must not exist either.
   The 041-invariant properties this confirms: StartSession never returns early with a still-running child, every failure path returns an error (never nil on ctx-cancel or bound expiry), and the bound is a wait, never a kill (`exec.Command`, not `CommandContext`).

4. **Confirm `validateSessionTurn` extraction and both-branch use.** A helper `func validateSessionTurn(ctx context.Context, output []byte) error` must exist and be called from BOTH branches (the interactive branch via `c.runCmd` output at the end of `StartSession`, the non-interactive branch from the read temp file in `runDetachedTurn`). Its checks must be `num_turns > 0` AND `is_error == false` AND `session_id` non-empty. NOTE: spec 045 already reworded its error strings (the current implementation uses `errors.Wrapf(ctx, errClaudeOutputUnparseable, "parse claude output: %v", err)` for unparseable output and a `rejectTurn(ctx, resultText, reason)` helper that produces `<result text> (<predicate>)` messages like `claude returned num_turns: 0`, `claude reported is_error: true`, `claude returned empty session_id`). These post-045 strings are CORRECT — do NOT reword them back to the pre-045 strings the spec text quotes, and do NOT force the spec's stale `"0 turns"` evidence grep (see `<verification>`). The interactive branch must otherwise be byte-identical to today: `defaultCommandRunner` unchanged, the 5m `context.WithTimeout(ctx, 5*time.Minute)` cap, `"claude bootstrap turn timed out after 5m"`, and `"run claude"` wrap.

5. **Confirm `export_test.go`.** It must contain
   ```go
   const SessionTurnTimeout = sessionTurnTimeout
   ```
   with a comment noting it is a test-only alias (locks wiring, not value — tests must also assert the literal `30 * libtime.Minute`). The file must also carry `var DefaultSessionLockDir = defaultSessionLockDir` (spec 042's export) — leave it untouched.

6. **Confirm the AC10 guards.** `grep -c 'defaultCommandRunner' pkg/ops/claude_session.go` must be 3 (the two constructors assign `runCmd: defaultCommandRunner` and the func is defined once — pinned so a rework cannot silently drop it) and `grep -c 'context.WithTimeout' pkg/ops/claude_session.go` must be 1 (on the interactive branch only). `mocks/claude-session-starter.go` must be untouched (the `StartSession` signature is unchanged).

7. **Confirm the test matrix covers AC1-6 and the detachment integration test.** In `pkg/ops/claude_session_test.go` the "non-interactive branch" context (starts at line 256) must contain specs that cover:
   - AC1 — "blocks until the detached child exits": blocking waiter, `doneCh` only fires after a `Consistently(returned, "100ms").ShouldNot(Receive())`, then `Eventually(returned).Should(Receive(BeNil()))`; the waiter receives the bound via `windowCh` and it is asserted to equal `ops.SessionTurnTimeout` AND `30 * libtime.Minute`.
   - AC2 — a clean exit (`doneCh <- nil` with valid JSON written to stdout) returns nil ("passes the session id and name to the detached runner").
   - AC3 — "validates the turn and rejects a zero-turn result", "validates the turn and rejects an is_error result", "rejects an unparseable turn result".
   - AC4 — "treats a child exit error as an error": error containing `"exit status 1"` AND `"exited with error"`; no assertion anywhere still uses `"exited during startup"`.
   - AC5 — "treats the turn timeout as an error so no id is persisted": error containing `"did not complete within"`.
   - AC6 — "treats context cancellation as an error so no id is persisted": error containing `"wait cancelled"` (NOT nil); plus "wraps a spawn failure" for `"start detached claude session"`.
   The spec-045 test specs already present in the same context (valid blob + non-zero exit → nil; parsed-but-rejected blob leads with the child's reason; unparseable output names the exit status; timeout-with-valid-blob-on-disk still errors) are spec 045's — confirm they exist, do NOT touch them. The interactive-branch tests (lines 54-254) must be UNCHANGED by you (spec 045 already updated the two that lock its reworded validator strings). The "session lock lifecycle" context (line 625) must also be left untouched.
   `pkg/ops/claude_session_detach_test.go` must contain "child outlives a cancelled parent wait": spawns a real script (`#!/bin/sh\nsleep 6\ntouch <sentinel>`), cancels the context after ~500ms, asserts `StartSession` returns an error, asserts the sentinel does NOT exist yet, then `Eventually(..., "20s", "200ms")` asserts the sentinel appears — proving the detached child survived the parent's cancelled wait. It constructs the starter via `ops.NewClaudeSessionStarter(script, ops.NewSessionLockerWithDir(lockDir))` (the two-arg form is spec 042's; keep it). If any of these is missing, implement it.

8. **BACKFILL — rename the test variable to satisfy AC1's evidence grep.** In `pkg/ops/claude_session_test.go`, inside the "blocks until the detached child exits" spec (starts at line 327), the local variable is currently named `window`. Rename it to `capturedWindow` so the assertion line reads exactly `Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))`. The block changes old → new as follows (keep `windowCh` — the channel — as-is; only the bare `window` variable is renamed):
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

9. **BACKFILL — assert the temp output file is removed after a clean exit.** No existing test asserts AC2's "the temp file is removed" half. Add ONE dedicated spec in the "non-interactive branch" context of `pkg/ops/claude_session_test.go` (after "wraps a spawn failure", the last spec in that context, which ends around line 622). `validTurnJSON`, `blockWaiter`, `starter`, `ctx`, and `locker` are all in scope there. This requires adding `path/filepath` to the file's imports (the current import block is: `context`, `errors`, `os`, `time`, `libtime`, `uuid`, ginkgo, gomega, `ops`). Add this spec verbatim:
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
   The eager unlink in `runDetachedTurn` runs before `StartSession` returns, so the before/after glob counts must be equal. Note the spec overrides `starter` with its own fake (writing `validTurnJSON` then `done <- nil`) so the child-exit branch wins and the blocking waiter goroutine stays parked on `<-bw` until `DeferCleanup` closes `blockWaiter` — the established pattern in this context.

10. **Self-check against AC1-6 and AC10.** Before finishing, re-read the changed hunks and walk each AC: the constant, the wait-select, the validation helper, the rename, and the new cleanup test. Run the `<verification>` block and confirm every grep that is expected to pass does pass; the two greps flagged in `<verification>` as spec artifacts must NOT be "fixed" by editing error strings.

Failure-mode coverage from the spec's table: bound expiry (row 1, AC5 test), ctx cancel mid-wait with child survival (row 2, AC6 unit + detach integration), child exits non-zero (row 3, AC4 test — with the post-045 refinement, the unparseable-output case is what this assertion covers), turn JSON is_error / 0 turns (row 4, AC3 tests), temp file unreadable/empty (row 5, AC3 unparseable test), UI request timeout < turn (row 8, AC6 cancel path). Each is covered by the corresponding test in this prompt.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Interactive branch behavior unchanged. `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched. The only permitted interactive-branch edit is none — the shared `validateSessionTurn` is already extracted and called from both branches; do NOT re-extract or reword it.
- Detachment preserved: `exec.Command` (NOT `CommandContext`), `Setpgid`, stdout/stderr handling that lets the child survive the parent. NEVER SIGKILL the child on timeout; `--max-turns` is inert (`-1`), so the 30-min bound is a wait-channel select, not a context kill. Do NOT resurrect `"claude session start timed out"`.
- Never offer a broken Resume: on any failure (exit error, `is_error`, 0 turns, bound expiry, ctx cancel) `StartSession` returns an error. Returning nil on ctx-cancel is wrong — it would persist an id for a still-running child.
- JSON validation: `num_turns > 0` AND `is_error == false`. Lowercase UUIDs; keep `-n "<task name>"` at mint so resume inherits the title.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- `sessionTurnTimeout` stays a tunable const — do NOT add a config field (spec Open Question 1 recommends const; no second caller exists).
- Do NOT alter the spec-045 error strings or the exit-status-vs-result precedence to satisfy a grep pattern. The spec's AC3/AC4/AC5 evidence greps are stale against the post-045 validator (see `<verification>`); the source strings are correct as written.
- `ClaudeSessionStarter.StartSession` signature is UNCHANGED (6 args: ctx, sessionID, prompt, cwd, name, isInteractive) — `mocks/claude-session-starter.go` is untouched. The `SessionLocker` constructor parameter (spec 042) is already wired in and must stay.
- Do NOT touch `pkg/ops/workon.go`, `pkg/ops/goal_workon.go`, or `docs/work-on-session-lifecycle.md` in this prompt — the workon reorder is prompt 2, the doc reword is prompt 3.
- Do NOT run `git` — `.git` is masked in this container (`hideGit=true`). The scenario-005-untouched check is operator-side.
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — spec evidence greps. Run each and record the count. Rows expected >= 1 must hold; rows expected == 0 must be 0:

```
grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                        # >= 1 (source identifier, lowercase)
grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go                 # >= 1 (value pin)
grep -c 'validateSessionTurn' pkg/ops/claude_session.go                       # >= 2 (both branches call it)
grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go  # >= 1 — THIS IS BACKFILL REQ 8; must flip 0 -> 1
grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go          # >= 1 — THIS IS BACKFILL REQ 9; must flip 0 -> 1
grep -c 'claude session exited with error' pkg/ops/claude_session.go          # >= 1 (AC4, real check — unquoted form; the string survives on the unparseable-output path)
grep -c 'exited during startup' pkg/ops/claude_session.go                     # == 0 (AC4)
grep -c 'did not complete within' pkg/ops/claude_session.go                   # >= 1 (AC5, real check — unquoted form)
grep -c 'livenessWindow' -r pkg/                                              # == 0
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go                      # == 3 (AC10)
grep -c 'context.WithTimeout' pkg/ops/claude_session.go                       # == 1 (AC10, interactive branch)
```

Note on spec artifacts you must NOT "fix":
- The spec's AC3 evidence `grep -c '"0 turns"' pkg/ops/claude_session_test.go >= 1` CANNOT match the post-045 validator (`claude returned num_turns: 0`); the tests assert `num_turns` / `claude reported is_error` instead. Do NOT reword the validator to force "0 turns".
- The spec's AC4/AC5 evidence greps carry a literal trailing double-quote (`'"claude session exited with error"'`, `'"did not complete within"'`) that can never match the real source strings (`"claude session exited with error: %v"` / `"claude session turn did not complete within %v"`). The unquoted forms above are the real checks.

SECONDARY — AC10 scenario-005 guard is OPERATOR-SIDE (git is masked in this container). The container-side proxies are the `defaultCommandRunner` == 3 and `context.WithTimeout` == 1 greps above.

SYNTAX + TESTS:
```
gofmt -e -l pkg/ops/claude_session.go pkg/ops/claude_session_test.go pkg/ops/export_test.go   # must list NO files
make test                                                                                    # must exit 0
```

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running only `make test` + the grep gate here is correct.
</verification>
