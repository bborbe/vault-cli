---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-08-30T18:52:00Z"
---

<summary>
- Confirms the non-interactive session-start path already blocks until the detached headless turn exits, instead of returning after ~10s while the child keeps writing.
- Confirms the turn's JSON output is validated on both the interactive and non-interactive branches, so a zero-turn, errored, or unparseable result is an error.
- Confirms child exit error, 30-minute bound expiry, and context cancellation all return an error so the caller persists no session id.
- Confirms the detached child survives parent timeout and cancellation (a wait bound, never a kill).
- Confirms the interactive TTY branch, `defaultCommandRunner`, and the 5-minute cap are unchanged.
- Backfill 1: renames one test variable so the spec's acceptance-criterion grep (`Expect(capturedWindow)...`) matches; behavior is unchanged.
- Backfill 2: adds the one genuinely missing test — the temp output file is removed after a clean exit.
- This prompt is verify-plus-backfill: the target state is already in the tree (shipped in v0.117.1); the only new code is one test variable rename and one new test.
- NOTE: `pkg/ops` may not compile until spec 042's executor lands `session_lock.go` — see constraints; do not let that derail this prompt.
</summary>

<objective>
Confirm — and backfill where anything is missing — that the non-interactive branch of `StartSession` blocks until the detached headless turn exits and validates its JSON, so the Vault UI never advertises Resume against a live or failed transcript. This prompt covers spec-041 ACs 1-6 and 10, and it is the foundation for prompts 2 and 3.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/claude_session.go` — the whole file. This is the file under test.
- `pkg/ops/export_test.go` — exposes the unexported constant.
- `pkg/ops/claude_session_test.go` — the whole file; the non-interactive branch context starts around line 246.
- `pkg/ops/claude_session_detach_test.go` — the detachment integration test.
- `docs/work-on-session-lifecycle.md` — the durable design record this implementation realizes.

Coding-plugin docs (read the ones relevant to this file):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` idiom; never `fmt.Errorf`, never bare `return err`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — why the two raw `go func`s in this file are deliberate (documented inline).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

NOTE: this container has NO git (`.git` is not mounted; `git rev-parse` fails). Do NOT run `git diff`, `git log`, or any bare `git` command. AC10's `git diff --exit-code HEAD -- scenarios/005-...` check is operator-side; you verify only its non-git guards (see `<verification>`).
</context>

<requirements>
The target state for this prompt already exists in the tree (shipped in v0.117.1). Your job is to CONFIRM each piece matches the spec-041 Design below, and BACKFILL only the two specific gaps named in requirements 8 and 9. Do not rewrite what is already correct — "confirm" means read the actual source and verify it matches; correct only genuine mismatches, which are not expected.

1. **Confirm the constant rename.** In `pkg/ops/claude_session.go` the unexported constant must be:
   ```go
   const sessionTurnTimeout = 30 * libtime.Minute
   ```
   with a doc comment stating it bounds the wait for the detached turn's exit, is never a kill, and is a tunable constant with no config field. `livenessWindow` must not appear anywhere in the repo (`grep -rn 'livenessWindow' pkg/` returns nothing). If the constant is missing or mis-named, define it exactly as above.

2. **Confirm `defaultDetachedRunner`.** Its signature must be:
   ```go
   func defaultDetachedRunner(args []string, dir string, stdout *os.File) (<-chan error, error)
   ```
   It must use `exec.Command` (NOT `exec.CommandContext`), set `cmd.Stdout = stdout` (the caller-owned temp file — the function must NOT close it), set `cmd.Stderr` to an `os.OpenFile(os.DevNull, os.O_WRONLY, 0)` handle (closed only after the child exits), set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`, log the spawn audit line (`slog.Info("claude detached spawn started", ...)` with pid), and return a buffered `done` channel (capacity 1) that receives `cmd.Wait()`'s error. `exec.CommandContext` must not appear in this function. If any piece differs, correct it to match; never close the caller-owned stdout file.

3. **Confirm the non-interactive branch.** `StartSession`'s non-interactive path must delegate to a `runDetachedTurn(ctx, args, cwd)` method that does, in order:
   - `outFile, err := os.CreateTemp("", "vault-claude-session-*.json")`; on error wrap with `"create claude output file"`.
   - Eager unlink + close via `defer` (`_ = os.Remove(outFile.Name())`, `_ = outFile.Close()`) so no temp file survives any return path, including cancel/timeout while the child still holds the fd.
   - `done, err := c.detachRun(args, cwd, outFile)`; on error wrap with `"start detached claude session"`.
   - A waiter goroutine: `waitCh <- c.waiter.Wait(ctx, c.sessionTurnTimeout)`.
   - A `select` with exactly these outcomes, ALL of which except a clean exit are errors:
     - `case exitErr := <-done:` — non-nil → `errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`; nil → fall through to read + validate.
     - `case err := <-waitCh:` — `err != nil` (ctx cancelled) → `errors.Wrap(ctx, err, "claude session wait cancelled")`; `err == nil` (bound expired) → `errors.Errorf(ctx, "claude session turn did not complete within %v", c.sessionTurnTimeout)`. The child is detached and survives either way.
   - After a clean exit: `os.ReadFile(outFile.Name())` (wrap with `"read claude output"`) then `validateSessionTurn(ctx, output)`.
   - The old strings `"claude session start timed out"` and `"exited during startup"` must not exist anywhere in the repo.
   If the branch differs, rewrite it to the contract above. Do NOT use `exec.CommandContext` here — the child must survive the parent.

4. **Confirm `validateSessionTurn` extraction.** A helper
   ```go
   func validateSessionTurn(ctx context.Context, output []byte) error
   ```
   must exist and be called from BOTH branches (the interactive branch via `c.runCmd` output at the end of the interactive block, the non-interactive branch from the read temp file). Its checks and error strings must be byte-identical to these:
   - `json.Unmarshal` failure → `errors.Wrap(ctx, err, "parse claude output")`
   - empty `session_id` → `errors.Errorf(ctx, "claude returned empty session_id")`
   - `num_turns == 0` → `errors.Errorf(ctx, "claude returned 0 turns: %s", result.Result)`
   - `is_error == true` → `errors.Errorf(ctx, "claude reported error: %s", result.Result)`
   - otherwise nil. It must not return nil on a dead session: a `session_id` alone proves nothing.
   The interactive branch must otherwise be byte-identical to today: `defaultCommandRunner` unchanged, the 5m `context.WithTimeout(ctx, 5*time.Minute)` cap, `"claude bootstrap turn timed out after 5m"`, and `"run claude"` wrap.

5. **Confirm `export_test.go`.** It must contain
   ```go
   const SessionTurnTimeout = sessionTurnTimeout
   ```
   with the existing comment noting it is an alias (locks wiring, not value — tests must also assert the literal `30 * libtime.Minute`).

6. **Confirm the test matrix exists.** In `pkg/ops/claude_session_test.go` the "non-interactive branch" context (starts ~line 246) must contain specs that cover:
   - AC1 — "blocks until the detached child exits" (~line 316): blocking waiter, `doneCh` only fires after a `Consistently(returned, "100ms").ShouldNot(Receive())`, then `Eventually(returned).Should(Receive(BeNil()))`; the waiter receives the bound and it equals `ops.SessionTurnTimeout` AND `30 * libtime.Minute`.
   - AC2 — a clean exit (`done <- nil` with valid JSON written to stdout) returns nil ("passes the session id and name to the detached runner").
   - AC3 — "validates the turn and rejects a zero-turn result", "validates the turn and rejects an is_error result", "rejects an unparseable turn result".
   - AC4 — "treats a child exit error as an error": error containing `"exit status 1"` AND `"exited with error"`; no assertion anywhere still uses the old `"exited during startup"` string.
   - AC5 — "treats the turn timeout as an error so no id is persisted": error containing `"did not complete within"`.
   - AC6 — "treats context cancellation as an error so no id is persisted": error containing `"wait cancelled"` (NOT nil); plus "wraps a spawn failure" for `"start detached claude session"`.
   The existing interactive-branch tests (lines ~48-244) must be UNCHANGED — they lock the byte-identical validation strings.

7. **Confirm the detachment integration test.** `pkg/ops/claude_session_detach_test.go` must contain a test ("child outlives a cancelled parent wait") that spawns a real script (`sleep 6; touch <sentinel>`), cancels the context after ~500ms, asserts `StartSession` returns an error, asserts the sentinel does NOT exist yet, and then `Eventually(..., "20s", "200ms")` asserts the sentinel appears — proving the detached child survived the parent's cancelled wait. If the file or test is missing, implement it.

8. **BACKFILL — rename the test variable to satisfy AC1's evidence grep.** In `pkg/ops/claude_session_test.go`, inside the "blocks until the detached child exits" spec (~lines 329-336), the local variable is currently named `window`. Rename it to `capturedWindow` so the assertion line reads exactly:
   ```go
   Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))
   ```
   The block changes old → new as follows (keep `windowCh` — the channel — as-is; only the bare `window` variable is renamed):
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

9. **BACKFILL — assert the temp output file is removed after a clean exit.** No existing test asserts AC2's "the temp file is removed" half. Add ONE dedicated spec in the "non-interactive branch" context of `pkg/ops/claude_session_test.go` (after "wraps a spawn failure", the last spec in that context). `validTurnJSON`, `blockWaiter`, `starter`, and `ctx` are all in scope there. Add this spec verbatim:
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
       )
       Expect(starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)).To(BeNil())
       Expect(matches()).To(Equal(before))
   })
   ```
   This requires adding `path/filepath` to the file's imports (currently imports `context`, `errors`, `os`, `time`, `libtime`, `uuid`, ginkgo, gomega, `ops`). The eager unlink in `runDetachedTurn` runs before `StartSession` returns, so the before/after glob counts must be equal. Note the selector picks the child-exit branch (`done <- nil`) so the blocking waiter goroutine stays parked on `<-bw` until `DeferCleanup` closes `blockWaiter` — the established pattern in this context.

10. **Self-check against AC1-6 and AC10.** Before finishing, re-read the changed hunks and walk each AC: the constant, the wait-select, the validation helper, the rename, and the new cleanup test. Run the `<verification>` block and confirm every grep that is expected to pass does pass; the two greps flagged in `<verification>` as spec-quoting artifacts must NOT be "fixed" by editing error strings.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Do NOT run any `git` command (no `.git` in this container; `git` commands fail).
- Interactive branch behavior unchanged. `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched. The only permitted interactive-branch edit is the extraction of `validateSessionTurn` (already done — do not re-extract or change it) — behavior-preserving, same checks, byte-identical error strings.
- Detachment preserved: `exec.Command` (NOT `CommandContext`), `Setpgid`, stdout/stderr handling that lets the child survive the parent. NEVER SIGKILL the child on timeout; `--max-turns` is inert (`-1`), so the 30-min bound is a wait-channel select, not a context kill. Do NOT resurrect `"claude session start timed out"`.
- Never offer a broken Resume: on any failure (exit error, `is_error`, 0 turns, bound expiry, ctx cancel) `StartSession` returns an error. Returning nil on ctx-cancel is wrong — it would persist an id for a still-running child.
- JSON validation: `num_turns > 0` AND `is_error == false`. Lowercase UUIDs; keep `-n "<task name>"` at mint so resume inherits the title.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- Do NOT alter the error strings to satisfy a grep pattern. The AC4/AC5 evidence greps in the spec's Verification carry a trailing-quote artifact (see `<verification>`); the source strings are correct as written.
- KNOWN PRE-EXISTING BREAK — NOT YOURS: `pkg/ops/session_lock.go:85` currently fails to compile (`cannot use int(f.Fd()) (value of type int) as uintptr value in argument to unix.FcntlInt`). That is in-flight spec-042 work (`specs/in-progress/042-prevent-duplicate-session-resume.md`), owned by another executor. Do NOT fix it, do NOT delete it, do NOT touch `session_lock.go`/`session_lock_test.go`. Because of it, `make test` / `go test ./pkg/ops/...` may fail at BUILD time for reasons unrelated to this prompt. Your verification therefore rests on the grep matrix + `gofmt` syntax check; `make test` is a diagnostic (see `<verification>`).
- Existing tests must still pass (modulo the 042 build break above).
</constraints>

<verification>
PRIMARY GATE — spec evidence greps (all grep-based; none need the package to compile). Run each and record the count. Rows expected >= 1 must hold; rows expected == 0 must be 0:

```
grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                        # >= 1 (source identifier, lowercase)
grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go                 # >= 1 (value pin)
grep -c 'validateSessionTurn' pkg/ops/claude_session.go                       # >= 2 (both branches call it)
grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go  # >= 1 — THIS IS BACKFILL REQ 8; must flip 0 -> 1
grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go          # >= 1 — THIS IS BACKFILL REQ 9; must flip 0 -> 1
grep -c '"0 turns"' pkg/ops/claude_session_test.go                            # >= 1 (AC3)
grep -c 'claude session exited with error' pkg/ops/claude_session.go          # >= 1 (AC4, real check)
grep -c 'exited during startup' pkg/ops/claude_session.go                     # == 0 (AC4)
grep -c 'did not complete within' pkg/ops/claude_session.go                   # >= 1 (AC5, real check)
grep -c 'livenessWindow' -r pkg/                                              # == 0
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go                      # == 3 (AC10)
grep -c 'context.WithTimeout' pkg/ops/claude_session.go                       # == 1 (AC10, interactive branch)
```

Note on spec-quoting artifacts: the spec's AC4/AC5 evidence greps use the literal `'"claude session exited with error"'` and `'"did not complete within"'` (trailing double-quote inside the pattern). Those two forms CANNOT match the real source strings (`"claude session exited with error: %v"` / `"claude session turn did not complete within %v"`), so they read 0 against CORRECT code. Do NOT force them to 1 by editing error strings — the non-quoted forms above are the real checks and must pass.

SECONDARY — syntax validity (no compile of the package):
```
gofmt -e -l pkg/ops/claude_session.go pkg/ops/claude_session_test.go pkg/ops/export_test.go   # must list NO files
```

DIAGNOSTIC — `make test`:
Run `make test` at the repo root. Because of the pre-existing spec-042 `session_lock.go` build break (see constraints), `go test` may fail at BUILD time before any test runs. If the ONLY failure is that 042 compile error (and the spec-041 backfills are present per the grep matrix), report `"status":"partial"` with the 042 build break explicitly named in the completion report — do NOT "fix" session_lock.go and do NOT mark the prompt failed on it. If the package DOES compile (042 already landed), `make test` must exit 0.

AC10's `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` CANNOT run here (no `.git`). Do not attempt it; it is verified by the operator ladder in the spec's Verification section.

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running only `make test` (diagnostic) + the grep gate here is correct.
</verification>
