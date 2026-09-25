---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-17T15:58:28Z"
---

# Confirm the non-interactive session start blocks until the headless turn exits (spec 041, prompt 1 of 3)

<summary>
- Confirms the non-interactive session start already blocks until the detached headless turn exits, instead of returning after a short window while the child keeps writing — so the Vault UI never offers Resume against a transcript another process is still writing. This half of the spec shipped and was never reverted.
- Confirms the turn's JSON result is validated on both the interactive and non-interactive branches through one shared helper, so a zero-turn, errored, or unparseable turn is an error and persists nothing.
- Confirms a child exit error, the 30-minute bound expiring, and context cancellation all return an error, and that the detached child survives the parent giving up — the bound is a wait, never a kill.
- Confirms the interactive terminal branch, its 5-minute cap, and the resume scenario are unchanged.
- Backfill 1: renames one test-local variable so the spec's pinned evidence grep for the turn bound matches. The assertion already exists under the old name, so this is a rename with no behaviour change.
- Backfill 2: adds the one genuinely missing test — that the turn's temporary output file is deleted after a clean exit.
- Flags spec evidence greps that can never match the real source: three of them carry a literal double quote the source does not contain, and one is stale. This prompt verifies the correct unquoted forms instead, and forbids editing error strings to force a broken grep to pass.
- Flags that the spec's error-string list for the shared validation helper is pre-spec-045 and must NOT be applied — the shipped strings are the current contract, and "restoring" the spec's forms would revert a later fix.
- Makes no production-code change: verification plus two test-only backfills.
</summary>

<objective>
Prove — and backfill the two gaps in — the already-shipped half of spec 041: a non-TTY `task work-on` blocks until the detached headless turn exits and validates its JSON result before the caller persists a session id, so the Vault UI advertises Resume only against a complete, single-writer transcript. This prompt covers spec 041 ACs 1-6 and 10, and is the precondition for prompts 2 and 3.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**Read this first — the spec's Design section is stale on two points.** `specs/in-progress/041-bug-resume-races-live-headless-turn.md` was written against v0.116.4. The `claude_session.go` half of it shipped as commit `247a789` and was then refined by spec 042 (per-session flock locker) and spec 045 (a validated turn result outranks a non-zero child exit). The spec's Design still quotes the pre-045 shapes — in particular its `validateSessionTurn` error strings and its bare `case exitErr := <-done:` handler. Those are superseded; see requirements 4 and 5. Do NOT "restore" them.

Read fully (in this order):
- `pkg/ops/claude_session.go` — the whole file (364 lines). This is the file under test.
- `pkg/ops/export_test.go` — the whole file; it exposes the unexported constant.
- `pkg/ops/claude_session_test.go` — the whole file (764 lines); the `Context("non-interactive branch", ...)` starts at line 256.
- `pkg/ops/claude_session_detach_test.go` — the whole file; the detachment integration test.
- `docs/work-on-session-lifecycle.md` — the durable design record this implementation realizes. Its task-path sections were rewritten by the v0.118.3 reversion; fixing that is prompt 3's job — do NOT edit the doc here.
- `pkg/ops/workon_session_writeback_test.go` — read the task and goal `BeforeEach` blocks only (lines ~110-240), to see how a fake `detachRun` writes a valid turn JSON line to the caller-owned `stdout *os.File` before feeding `done`. That is the established fake shape.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`, never `context.Background()` inside `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — why the raw `go func`s in this file are deliberate (documented inline in the source).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions used by this repo.

NOTE: git IS available in this container — `.dark-factory.yaml` is `workflow: direct` with no `hideGit`, so the AC10 `git diff --exit-code HEAD` guard in `<verification>` runs here and is NOT operator-side.
</context>

<requirements>
The target state for this prompt ALREADY EXISTS in the tree. Your job is to read the actual source, confirm each piece matches the contract below, and BACKFILL the two specific gaps named in requirements 10 and 11. "Confirm" means read the real code and verify it — correct only genuine mismatches, which are not expected. Do NOT rewrite what is already correct, and do NOT re-derive anything from the spec's Design section (see `<context>`).

**Fail-loud gate.** If `grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go` is 0, or `validateSessionTurn` does not exist in that file, or the non-interactive branch does not call a method named `runDetachedTurn`, STOP immediately and report `"status":"failed"` with message `"spec-041 prompt 1 target state absent: claude_session.go does not carry the block-until-exit design"`. Do NOT reconstruct it from the spec's Design section — those strings are pre-spec-045 and reconstructing them would revert spec 045. Report and stop.

1. **Confirm the turn bound constant.** In `pkg/ops/claude_session.go` the unexported constant must be exactly:
   ```go
   const sessionTurnTimeout = 30 * libtime.Minute
   ```
   (type `libtime.Duration` from `github.com/bborbe/time`, NOT stdlib `time.Duration`). Its doc comment must state that it bounds the wait for the detached turn's exit, that it is never a kill (the child is detached and survives expiry), and that it is a tunable constant with no config field. `livenessWindow` must not appear anywhere under `pkg/`. This resolves spec Open Question 1 as a constant — do NOT add a config field.

2. **Confirm `defaultDetachedRunner`.** Its signature must be:
   ```go
   func defaultDetachedRunner(args []string, dir string, stdout *os.File) (<-chan error, error)
   ```
   It must use `exec.Command` (NOT `exec.CommandContext`), set `cmd.Stdout = stdout` and never close that caller-owned file, set `cmd.Stderr` to a handle opened with `os.OpenFile(os.DevNull, os.O_WRONLY, 0)` (closed inside the reaper goroutine only after the child exits), set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`, emit the spawn audit log line (`slog.Info("claude detached spawn started", ...)` with the pid), and return a `done` channel buffered with capacity 1 that receives `cmd.Wait()`'s error from a reaper goroutine. `exec.CommandContext` must not appear in this function.

3. **Confirm the non-interactive branch delegates to `runDetachedTurn`.** `StartSession`'s `if !isInteractive` path must return `c.runDetachedTurn(ctx, args, cwd)`, and `runDetachedTurn` must do, in order:
   - `outFile, err := os.CreateTemp("", "vault-claude-session-*.json")`; on error wrap with `"create claude output file"`.
   - A `defer` that both unlinks and closes the file (`_ = os.Remove(outFile.Name())` then `_ = outFile.Close()`), so no temp file survives ANY return path — including the cancel and timeout paths, where the still-running child holds the fd.
   - `done, err := c.detachRun(args, cwd, outFile)`; on error wrap with `"start detached claude session"`.
   - A waiter goroutine whose only send is `waitCh <- c.waiter.Wait(ctx, c.sessionTurnTimeout)`, on a `waitCh` buffered with capacity 1.
   - A `select` over `done` and `waitCh`. The read of the output file MUST live inside the `done` branch and must NOT be hoisted above the `select` — on the timeout and cancellation paths the child is still running, so any bytes present are partial by definition and must never be validated as success. There is a unit test locking this ("fails on timeout even when a valid blob is already on disk"); keep it.

4. **Confirm the `done` branch's current (post-spec-045) shape — do NOT replace it with the spec's simpler form.** The spec's Design says a non-nil `exitErr` should immediately return `errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`. That is NOT the shipped contract; spec 045 refined it so a validated turn result outranks a non-zero child exit. The shipped order is:
   - Read the file (`errors.Wrap(ctx, readErr, "read claude output")` on failure), then call `validateSessionTurn(ctx, output)`.
   - If validation returns nil → return nil. If `exitErr` is non-nil, log it (`slog.Warn("validated turn result overrides non-zero child exit", ...)`) rather than swallowing it silently.
   - If validation failed AND `exitErr != nil` AND `errors.Is(validateErr, errClaudeOutputUnparseable)` → return `errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`. There is no usable result, so the child's exit status is the only reason that can be named.
   - Otherwise → if `exitErr != nil`, log it at `slog.Debug` (NOT `Warn` — spec 045 SC5: at Warn it became the first line of the Vault UI banner, ahead of the error itself), then return `validateErr` unwrapped so the child's own result text leads the message.
   The sentinel `errClaudeOutputUnparseable` (`stderrors.New("claude output is not valid turn JSON")`) must exist and must remain the wrapped cause in the unparseable case, so `errors.Is` matches. Do not "simplify" this back to the spec's form.

5. **Confirm the `waitCh` branch — both outcomes are errors.** `err != nil` (context cancelled) → `errors.Wrap(ctx, err, "claude session wait cancelled")`. `err == nil` (the bound expired) → `errors.Errorf(ctx, "claude session turn did not complete within %v", c.sessionTurnTimeout)`. Returning nil on cancellation is WRONG: it would let the caller persist an id for a still-running child. The child survives detached either way — never SIGKILL it. The strings `"claude session start timed out"` and `"exited during startup"` must not exist anywhere in the repo.

6. **Confirm `validateSessionTurn` and its CURRENT error contract.** A helper
   ```go
   func validateSessionTurn(ctx context.Context, output []byte) error
   ```
   must exist and be called from BOTH branches — the interactive branch on `c.runCmd`'s output, the non-interactive branch on the read temp file. It unmarshals `session_id`, `num_turns`, `is_error`, `result`. Its checks, in order, and their CURRENT strings (these are the spec-045 forms — the spec's Design section lists older ones; do NOT apply those):
   - `json.Unmarshal` failure → `errors.Wrapf(ctx, errClaudeOutputUnparseable, "parse claude output: %v", err)`.
   - empty `session_id` → `rejectTurn(ctx, result.Result, "claude returned empty session_id")`.
   - `num_turns == 0` → `rejectTurn(ctx, result.Result, "claude returned num_turns: 0")`.
   - `is_error == true` → `rejectTurn(ctx, result.Result, "claude reported is_error: true")`.
   - otherwise nil.
   `rejectTurn(ctx, resultText, reason)` returns `errors.New(ctx, reason)` when `resultText` is empty, else `errors.Errorf(ctx, "%s (%s)", resultText, reason)` — the child's own result text leads because it is the only part an operator can act on. `validateSessionTurn` must never return nil for a dead session: claude reports a `session_id` even for a turn that did no work, so the id alone proves nothing.

7. **Confirm the interactive branch is byte-identical to today.** `defaultCommandRunner` unchanged; the cap is `context.WithTimeout(ctx, 5*time.Minute)`; the timeout error is `"claude bootstrap turn timed out after 5m"`; the runner error wraps with `"run claude"`; the branch ends with `validateSessionTurn(ctx, output)`. The only permitted difference from the pre-041 form is that the inline JSON validation now calls the shared helper.

8. **Confirm `pkg/ops/export_test.go`.** It must contain
   ```go
   const SessionTurnTimeout = sessionTurnTimeout
   ```
   with a comment noting it is a test-only alias: asserting against it locks the WIRING (StartSession hands the constant, not a stray literal) but NOT the value, because a retune moves both sides — so tests must also assert the literal `30 * libtime.Minute`. The file must also carry `var DefaultSessionLockDir = defaultSessionLockDir` (spec 042's export) — that is expected; leave it untouched.

9. **Confirm the existing test matrix in `pkg/ops/claude_session_test.go`.** In `Context("non-interactive branch", ...)` confirm these specs exist and match:
   - "blocks until the detached child exits" — a blocking waiter, `Consistently(returned, "100ms").ShouldNot(Receive())` before `doneCh <- nil`, then `Eventually(returned).Should(Receive(BeNil()))`; the waiter receives the bound through `windowCh` and it is asserted equal to BOTH `ops.SessionTurnTimeout` and `30 * libtime.Minute`.
   - "passes the session id and name to the detached runner" — clean exit with valid JSON returns nil, and argv carries `--session-id`, `--print`, `-n`, the name, and the cwd.
   - "validates the turn and rejects a zero-turn result", "validates the turn and rejects an is_error result", "rejects an unparseable turn result".
   - "treats a child exit error as an error" — the error contains `"exit status 1"` AND `"exited with error"`. No assertion anywhere may still use `"exited during startup"`.
   - "returns nil when the child writes a valid blob and exits non-zero", "leads with the child's reason when a parsed blob reports is_error" / "... has zero turns" / "... has an empty session_id", "names the exit status when the output is non-empty but unparseable" — these are spec 045's precedence locks; leave them alone.
   - "fails on timeout even when a valid blob is already on disk", "treats the turn timeout as an error so no id is persisted", "treats context cancellation as an error so no id is persisted" (asserting `"wait cancelled"`), "wraps a spawn failure" (asserting `"start detached claude session"`).
   The interactive-branch tests above that context and the `Context("session lock lifecycle", ...)` below it (spec 042) must be left UNTOUCHED — they lock byte-identical strings and the lock contract.

10. **BACKFILL — rename the test-local variable so AC1's pinned evidence grep matches.** In the "blocks until the detached child exits" spec, the local variable holding the received bound is named `window`. Rename ONLY that variable to `capturedWindow`, so the assertion line becomes exactly `Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))`:
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
   Do NOT rename the `windowCh` CHANNEL, and do not touch any other identifier. Pure rename, zero behaviour change. This backfill exists only so the spec's AC1 evidence grep (`Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))`) resolves against the real file.

11. **BACKFILL — assert the temp output file is removed after a clean exit.** No existing test covers AC2's "the temp file is removed" half. Add ONE spec at the end of `Context("non-interactive branch", ...)` (after "wraps a spawn failure"). Capture the file's path from the `stdout *os.File` the fake receives — do NOT glob `os.TempDir()`, which is shared state and makes the assertion racy against any concurrent or leaked file. `validTurnJSON`, `blockWaiter`, `starter`, `ctx` and `locker` are all in scope there. Add exactly:
   ```go
   It("removes the temp output file after a clean exit", func() {
       var outPath string
       bw := blockWaiter
       starter = ops.NewClaudeSessionStarterWithRunner(
           "/usr/local/bin/claude",
           nil,
           func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
               outPath = stdout.Name()
               if _, err := stdout.WriteString(validTurnJSON); err != nil {
                   return nil, err
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
       Expect(outPath).NotTo(BeEmpty())
       _, statErr := os.Stat(outPath)
       Expect(statErr).To(HaveOccurred())
   })
   ```
   Notes: capture `bw := blockWaiter` spec-locally BEFORE the waiter closure reads it — `StartSession` can return via the child-exit branch while the waiter goroutine is still parked, so that goroutine outlives the spec and must not read a variable the next spec reassigns. The fake writes valid JSON and then feeds `done`, so the child-exit branch wins and the waiter goroutine stays parked until `DeferCleanup` closes `blockWaiter`. The eager unlink in `runDetachedTurn` runs before `StartSession` returns, so the `os.Stat` after the call must fail. No new import is needed — `os` is already imported.

12. **Confirm the detachment integration test.** `pkg/ops/claude_session_detach_test.go` must contain a spec ("child outlives a cancelled parent wait") that writes a real shell script (`#!/bin/sh\nsleep 6\ntouch <sentinel>`), cancels the context after ~500ms, asserts `StartSession` returns an error, asserts the sentinel does NOT exist yet, and then `Eventually(..., "20s", "200ms")` asserts the sentinel appears — proving the detached child survived the parent's cancelled wait. It constructs the starter with the two-argument form `ops.NewClaudeSessionStarter(script, ops.NewSessionLockerWithDir(lockDir))` (the locker is spec 042's; keep it). If the file or spec is missing, report `"status":"failed"` — do not re-implement from the spec, whose snippet uses a 12s script and a 1s cancel, neither of which matters to the invariant.

13. **Confirm the AC10 guards by reading, then by grep.** `defaultCommandRunner` is defined once and referenced by both constructors — the grep count in `pkg/ops/claude_session.go` must be exactly 3. `context.WithTimeout` must appear exactly once, on the interactive branch. `scenarios/005-work-on-resume-auto-invokes-subtask.md` must be byte-identical to `HEAD` (see `<verification>`). `mocks/claude-session-starter.go` must be untouched: `ClaudeSessionStarter.StartSession`'s signature is `StartSession(context.Context, string, string, string, string, bool) error` — six parameters, unchanged.

14. **Self-check before finishing.** Re-read the two changed hunks and walk spec 041 ACs 1-6 and 10 against them, stating which artifact satisfies each. Run every command in `<verification>` and confirm each holds. The three spec evidence greps flagged in `<verification>` as quoting artifacts must NOT be "fixed" by editing source strings.

Failure-mode coverage carried by this prompt (spec's Failure Modes table): row 1 bound expiry (the timeout spec), row 2 ctx cancel with child survival (the cancellation spec plus the detach integration spec), row 3 non-zero child exit (the exit-error spec), row 4 `is_error` / zero turns (the validation specs), row 5 unreadable or empty temp file (the unparseable spec), row 8 UI request timeout shorter than the turn (the cancellation path).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff --exit-code HEAD` only reads; do not stage or commit anything.
- Interactive branch behaviour unchanged. `defaultCommandRunner`, the 5-minute TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched. The only interactive-branch change already in place is the shared-helper extraction — behaviour-preserving, same checks, same strings. Do NOT re-extract or change it.
- Detachment preserved: `exec.Command` (NOT `CommandContext`), `Setpgid`, and stdout/stderr handling that lets the child survive the parent. NEVER SIGKILL the child on timeout — `--max-turns` is inert (`maxTurns` is -1), so the 30-minute bound is a wait-channel select, not a context kill. Do NOT resurrect `"claude session start timed out"`.
- Never offer a broken Resume: on any failure (child exit error, `is_error`, zero turns, bound expiry, context cancel) `StartSession` returns an error so the caller persists nothing. Returning nil on cancellation is wrong.
- JSON validation: `num_turns > 0` AND `is_error == false`. Session ids are lowercase UUIDs; keep `-n "<name>"` at mint so a resume inherits the title.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- `sessionTurnTimeout` stays a tunable constant — do NOT add a config field (spec Non-goals and Open Question 1 both forbid it).
- Do NOT alter error strings to satisfy a grep pattern. Three of the spec's evidence greps carry a literal double quote the source does not contain (see `<verification>`); the source strings are correct as written.
- Do NOT touch `pkg/ops/workon.go`, `pkg/ops/goal_workon.go`, `pkg/ops/workon_test.go`, `pkg/ops/goal_workon_test.go`, `pkg/ops/workon_session_writeback_test.go`, or `docs/work-on-session-lifecycle.md` in this prompt — the caller-side reorder is prompt 2 and the doc reword is prompt 3.
- Do NOT add a double-Start guard and do NOT add any config knob — both are spec Non-goals.
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                          # >= 1 (source identifier, lowercase)
grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go                   # >= 1 (value pin)
grep -c 'validateSessionTurn' pkg/ops/claude_session.go                         # >= 2 (both branches call it)
grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go  # >= 1 — BACKFILL REQ 10; must flip 0 -> 1
grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go           # >= 1 — BACKFILL REQ 11; must flip 0 -> 1
grep -c 'claude session exited with error' pkg/ops/claude_session.go            # >= 1 (AC4, real check — unquoted form)
grep -c 'did not complete within' pkg/ops/claude_session.go                     # >= 1 (AC5, real check — unquoted form)
grep -c 'num_turns: 0' pkg/ops/claude_session.go                                # >= 1 (AC3, real check)
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go                        # == 3 (AC10)
grep -c 'context.WithTimeout' pkg/ops/claude_session.go                         # == 1 (AC10, interactive branch)
! grep -q 'exited during startup' pkg/ops/claude_session.go                     # AC4: absent
! grep -q 'claude session start timed out' pkg/ops/claude_session.go            # AC4: absent
! grep -rq 'livenessWindow' pkg/                                              # AC1: absent everywhere
```

Spec-quoting artifacts — do NOT try to make these pass. The spec's AC1/AC3/AC4/AC5 evidence greps use `'"claude session exited with error"'`, `'"did not complete within"'` and `'"0 turns"'` (a literal double quote inside the pattern). None of those three can match the real source strings (`"claude session exited with error: %v"`, `"claude session turn did not complete within %v"`, `"claude returned num_turns: 0"`), so they read 0 against CORRECT code. The unquoted forms above are the real checks. Never edit a source string to force a broken grep to pass.

SECONDARY — AC10 git guard (git IS available: `workflow: direct`, no `hideGit`):
```
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md   # must exit 0 with empty output
```

SYNTAX + TESTS:
```
gofmt -e -l pkg/ops/claude_session.go pkg/ops/claude_session_test.go pkg/ops/export_test.go   # must list NO files
make test                                                                                    # must exit 0
```

FULL GATE — `make precommit` at the repo root must exit 0. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make gosec`, `make errcheck`, ...), then `make precommit` once more. This is the batch's authoritative full-gate check (AC13), which prompt 3 runs last; running it here as well keeps the tree green for prompt 2, which builds on it.
</verification>
