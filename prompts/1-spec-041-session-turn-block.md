---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-14T20:23:00Z"
---

# Session layer: confirm block-until-exit, backfill the two missing test pieces (spec 041, prompt 1 of 3)

<summary>
- The non-interactive session start already waits for the detached headless turn's child process to exit — this prompt confirms that behaviour is intact on this branch and closes the two remaining test gaps around it.
- The wait bound is confirmed as a 30-minute bound that never kills the child: the child runs in its own process group and survives the parent giving up on it.
- A finished turn's captured result is validated on both the interactive and the non-interactive path through one shared validator, so a zero-turn, errored, or unparseable turn is an error and persists nothing.
- A child exit error, an expired bound, and a cancelled wait all return an error — so the caller never persists a session id for a turn that did not genuinely finish.
- The interactive terminal path, its five-minute cap, and the resume scenario that protects it are confirmed untouched.
- One test variable is renamed so the spec's acceptance evidence for the 30-minute bound matches the assertion that already exists — a pure rename, no behaviour change.
- One genuinely missing test is added: the temporary capture file is removed after a clean turn.
- Three of the spec's evidence greps are broken as written (two quote a trailing double-quote that can never match, one was overtaken by a later spec's wording); the prompt verifies the real substrings instead and forbids editing source to satisfy a broken grep.
- The spec's `git diff` guard for the resume scenario cannot run — this container's `.git` is masked — so that scenario's content is pinned by hash instead.
- No production behaviour changes in this prompt.
</summary>

<objective>
Confirm — and backfill the two missing test pieces — that the non-interactive branch of `StartSession` blocks until the detached headless turn exits and validates its captured JSON, so the Vault UI never offers Resume against a live or failed transcript. This prompt covers spec 041 Acceptance Criteria 1-6 and 10, and is the foundation for prompts 2 and 3.
</objective>

<context>
Read `CLAUDE.md` for project conventions, then read these files in full, in this order:

- `pkg/ops/claude_session.go` — the whole file. This is the file under confirmation.
- `pkg/ops/export_test.go` — exposes the unexported constant.
- `pkg/ops/claude_session_test.go` — the whole file. The `Context("non-interactive branch", …)` block and the `Context("session lock lifecycle", …)` block are the ones that matter.
- `pkg/ops/claude_session_detach_test.go` — the detachment integration test.
- `docs/work-on-session-lifecycle.md` — the durable design record this implementation realizes. Read it; do NOT edit it (prompt 3 owns it).
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` — read it; do NOT edit it. Its content hash is pinned in `<verification>`.
- `specs/in-progress/041-bug-resume-races-live-headless-turn.md` — the spec. Read its Design section for `pkg/ops/claude_session.go`, and read the whole Acceptance Criteria + Verification sections so you can see the three artifact defects named in requirement 11.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `errors.Wrapf(ctx, …)` / `errors.Wrap(ctx, …)` / `errors.Errorf(ctx, …)` idiom from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, counterfeiter mocks, coverage expectations.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — why the raw `go func`s in this file are deliberate and documented inline (do not "fix" them into `run.CancelOnFirstErrorWait`).

Two environment facts that change how this prompt is written:

1. **`git` is unavailable in this container.** `/workspace/.git` is a character device, so every `git` command fails with `fatal: not a git repository`. Do NOT run `git` — the spec's AC10 `git diff --exit-code HEAD -- scenarios/005-…` guard is replaced by a content-hash pin (see `<verification>`), which is at least as strong for the purpose the guard serves: proving this batch did not touch that scenario.
2. **A later spec has already moved two things this spec's Design sketch describes.** Spec 045 (`specs/in-progress/045-bug-exit-code-outranks-validated-turn.md`; its prompts 208 and 209 are completed) shipped after spec 041 and (a) changed the validator's predicate wording, and (b) inverted the verdict precedence inside the child-exit branch. The spec-041 Design sketch predates both. Where the sketch and the tree disagree on those two points, **the tree wins and must not be "corrected" back** — see requirements 4 and 5.
</context>

<requirements>

## 0. This is a confirm-and-backfill prompt, not an implement-from-scratch prompt

`pkg/ops/claude_session.go`, `pkg/ops/export_test.go`, `pkg/ops/claude_session_test.go` and `pkg/ops/claude_session_detach_test.go` are ALREADY in the spec-041 target state on this branch. The v0.118.3 reversion that pulled the task-side half back (`pkg/ops/workon.go` + `docs/work-on-session-lifecycle.md`) never touched these four files.

Your job: read each piece below against the real source, confirm it matches, and change only what requirements 8 and 9 name. "Confirm" means read the actual code and check it — not rewrite it in your own style, not rename identifiers, not reorder, not "improve". If you find a genuine mismatch with a requirement below, correct that one thing and say so in the completion report.

## 1. Confirm the constant

In `pkg/ops/claude_session.go` the unexported constant must be:

```go
const sessionTurnTimeout = 30 * libtime.Minute
```

Its doc comment must state that it bounds the wait for the detached turn's exit, that it is never a kill (the child is detached in its own process group and survives expiry), that `--max-turns` is inert (`maxTurns` is -1) so a legitimate agentic chain can run for minutes, and that it is a tunable constant with no config field. Spec Open Question 1 resolved this as a const — do **not** add a config field, flag, or env override.

`livenessWindow` must not appear anywhere under `pkg/` (it must not survive as a stale name).

## 2. Confirm `defaultDetachedRunner`

Its signature must be exactly:

```go
func defaultDetachedRunner(args []string, dir string, stdout *os.File) (<-chan error, error)
```

and it must: use `exec.Command` (NOT `exec.CommandContext`), set `cmd.Stdout = stdout` (the caller-owned temp file — this function must NEVER close it), open `os.DevNull` with `os.OpenFile` and assign that handle to `cmd.Stderr`, set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`, close the devnull handle only after the child exits (inside the reaper goroutine), log the spawn audit line (`slog.Info("claude detached spawn started", …)` including the pid), and return a buffered `done` channel of capacity 1 carrying `cmd.Wait()`'s error. If any piece differs, correct it to match. Never close the caller-owned stdout file.

## 3. Confirm the non-interactive branch delegates to `runDetachedTurn`

`StartSession`'s non-interactive path must be a single delegation to `c.runDetachedTurn(ctx, args, cwd)`; a method

```go
func (c *claudeSessionStarter) runDetachedTurn(ctx context.Context, args []string, cwd string) error
```

must exist and must do, in this order:

- `outFile, err := os.CreateTemp("", "vault-claude-session-*.json")`; on error wrap with `"create claude output file"`.
- Eager cleanup via `defer`: `_ = os.Remove(outFile.Name())` then `_ = outFile.Close()`, so no temp file survives any return path — including cancel and timeout, where the still-running child holds the fd.
- `done, err := c.detachRun(args, cwd, outFile)`; on error wrap with `"start detached claude session"`.
- A waiter goroutine sending `c.waiter.Wait(ctx, c.sessionTurnTimeout)` into a buffered channel of capacity 1.
- A `select` whose two cases are described in requirement 4.

If the branch differs structurally from that, rewrite it to match. Do NOT use `exec.CommandContext` here — the child must survive the parent.

## 4. Confirm the `select` — and do NOT simplify it back to the spec's Design sketch

The `select` in `runDetachedTurn` must have the **spec 045** shape, not the shape printed in spec 041's Design section. Spec 041's sketch (`case exitErr := <-done:` → immediately return `errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`) predates spec 045, which shipped later and inverted the verdict precedence. Reverting to the sketch would break spec 045's tests and silently discard a session that can be resumed.

The required shape:

- `case exitErr := <-done:` — the child has exited, so its fd is closed and the file is complete. Read it (`os.ReadFile(outFile.Name())`, wrap a read failure with `"read claude output"`) and call `validateSessionTurn(ctx, output)`. Then:
  - validated (nil) → return nil; when `exitErr != nil`, log the existing warning that a validated result overrides a non-zero child exit.
  - not validated AND `errors.Is(validateErr, errClaudeOutputUnparseable)` AND `exitErr != nil` → `errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`. This is the one surviving path where the exit status is the reason.
  - otherwise → return `validateErr` **unwrapped**, so the child's own `result` text leads the message.
  - The read must stay inside this case branch. Hoisting it above the `select` is the specific refactor this design forbids: on the timeout and cancellation paths the child is still running, so the bytes present are partial by definition and must never be validated as success.
- `case err := <-waitCh:` — **both** outcomes are errors, so the caller persists no session id:
  - `err != nil` (ctx cancelled) → `errors.Wrap(ctx, err, "claude session wait cancelled")`.
  - `err == nil` (bound expired) → `errors.Errorf(ctx, "claude session turn did not complete within %v", c.sessionTurnTimeout)`.
  - The child is detached and keeps running in either case; the parent only stops waiting. Never SIGKILL it.

`exec.CommandContext`, `"claude session start timed out"` and `"exited during startup"` must not appear anywhere under `pkg/`.

## 5. Confirm the shared validator — with the CURRENT predicate wording, not the spec's historical literals

A helper

```go
func validateSessionTurn(ctx context.Context, output []byte) error
```

must exist and be called from BOTH branches (the interactive branch on `c.runCmd`'s output, the non-interactive branch on the bytes read from the temp file). Its checks and its current error strings are:

- `json.Unmarshal` failure → `errors.Wrapf(ctx, errClaudeOutputUnparseable, "parse claude output: %v", err)`. The sentinel must stay the wrapped cause so `errors.Is` matches it; do not "fix" this back to `errors.Wrap`.
- empty `session_id` → `rejectTurn(ctx, result.Result, "claude returned empty session_id")`
- `num_turns == 0` → `rejectTurn(ctx, result.Result, "claude returned num_turns: 0")`
- `is_error == true` → `rejectTurn(ctx, result.Result, "claude reported is_error: true")`
- otherwise nil. A `session_id` alone proves nothing — claude reports one even for a turn that did no work.

**Do not rewrite these strings to spec 041's Design literals** (`"claude returned 0 turns: %s"`, `"claude reported error: %s"`). Spec 045 deliberately changed them, its tests pin the current wording, and `docs/work-on-session-lifecycle.md` documents them. Also leave `rejectTurn`'s "the child's own result text leads" property alone.

The interactive branch must otherwise be byte-identical to today: `defaultCommandRunner` untouched, the 5m `context.WithTimeout(ctx, 5*time.Minute)` cap present, and the `"claude bootstrap turn timed out after 5m"` / `"run claude"` strings unchanged. The only permitted interactive-branch edit is the already-extracted `validateSessionTurn` call.

## 6. Confirm `export_test.go`

It must contain `const SessionTurnTimeout = sessionTurnTimeout` with a comment noting it is a test-only alias that locks the wiring but NOT the value (so tests must also assert the literal `30 * libtime.Minute`). The file must also still carry `var DefaultSessionLockDir = defaultSessionLockDir` (spec 042's export) — leave that untouched.

## 7. Confirm the test matrix exists

In `pkg/ops/claude_session_test.go`, the `Context("non-interactive branch", …)` block must already contain specs covering:

- a blocking waiter: `StartSession` does not return until the fake child's exit channel fires (`Consistently(returned, "100ms").ShouldNot(Receive())` then `Eventually(returned).Should(Receive(BeNil()))`), and the waiter receives the bound, asserted both against `ops.SessionTurnTimeout` and against the literal `30 * libtime.Minute`;
- a clean exit with a valid JSON blob written to the stdout file → nil error;
- a zero-turn blob, an `is_error` blob, and an unparseable blob → errors naming `num_turns`, `is_error`, and `parse claude output` respectively;
- a child exit error → an error containing `exit status 1` and `exited with error`;
- an expired bound → an error containing `did not complete within`;
- a cancelled wait → an error containing `wait cancelled` (NOT nil);
- a spawn failure → an error containing `start detached claude session`.

And `pkg/ops/claude_session_detach_test.go` must contain the integration spec that spawns a real `#!/bin/sh` script (`sleep 6` then `touch <sentinel>`), cancels the context after ~500ms, asserts `StartSession` returned an error and the sentinel does not exist yet, then asserts with `Eventually` that the sentinel appears — proving the detached child survived the parent's cancelled wait. If the file or that spec is missing, implement it.

The interactive-branch specs earlier in `claude_session_test.go` must stay UNCHANGED — they lock the validation strings. The `Context("session lock lifecycle", …)` block (spec 042) must also stay untouched.

## 8. BACKFILL — rename the bound variable so AC1's evidence grep matches

Spec 041 AC1's evidence grep is `grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go`. The assertion already exists, but under a different variable name, so the grep reads 0 against correct code. In the `"blocks until the detached child exits"` spec, rename the local variable `window` to `capturedWindow`. Keep the channel `windowCh` named exactly as it is — only the bare `window` variable is renamed:

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

Do not rename any other identifier. This is a pure rename; no behaviour changes.

## 9. BACKFILL — assert the temp capture file is removed after a clean exit

No existing spec covers AC2's "the temp file is removed" half. Add ONE spec at the end of the `Context("non-interactive branch", …)` block, after `"wraps a spawn failure"`. `validTurnJSON`, `blockWaiter`, `starter`, `ctx` and `locker` are all in scope there. Add exactly this spec:

```go
		It("removes the temp output file after a clean exit", func() {
			var capturedPath string
			bw := blockWaiter
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
					capturedPath = stdout.Name()
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
			Expect(capturedPath).NotTo(BeEmpty())
			// runDetachedTurn's deferred cleanup unlinks the file before StartSession
			// returns; on POSIX the name stops resolving the moment it is removed.
			_, statErr := os.Stat(capturedPath)
			Expect(errors.Is(statErr, os.ErrNotExist)).To(BeTrue())
		})
```

Notes on why this shape and not a glob:

- Capture the name from the `stdout` parameter and stat THAT path. Do **not** glob `os.TempDir()` for `vault-claude-session-*.json` — that reads a shared directory another process could write to, and a false failure there costs a whole cycle. The captured-name form is exact.
- The fake writes `validTurnJSON` and returns `done <- nil` so the child-exit branch wins the select, and the blocking waiter parks on `<-bw` until the block's `DeferCleanup` closes it (the established pattern in this block).
- No new import is needed: `os` and stdlib `errors` are both already imported by this file. If you find yourself adding an import, you have changed the shape — go back to the spec above.

## 10. Self-check against ACs 1-6 and 10

Re-read the changed hunks and walk each acceptance criterion: the constant, the wait select, the shared validator, the rename, and the new cleanup spec. Then run the whole `<verification>` block and confirm every check passes.

## 11. Three spec-artifact defects — verify the real substrings, never edit source to satisfy a broken grep

Three evidence greps in spec 041's **Acceptance Criteria** text cannot match correct code. The spec's own Verification block already carries the corrected unquoted forms for the first two; the third has no replacement anywhere, so it is substituted here with the live assertion.

1. AC4's `grep -c '"claude session exited with error"' pkg/ops/claude_session.go` — the real source string is `"claude session exited with error: %v"`; a pattern ending in a double-quote can never match it.
2. AC5's `grep -c '"did not complete within"' pkg/ops/claude_session.go` — same trailing-quote defect against `"claude session turn did not complete within %v"`.
3. AC3's `grep -c '"0 turns"' pkg/ops/claude_session_test.go` — the quoted form matches nothing; the live assertion is `ContainSubstring("num_turns")` (spec 045 changed the wording).

The `<verification>` block below checks the unquoted substrings and the live assertion instead. **Do not edit any error string or test assertion to make a broken grep pass.** If you think a source string is wrong, say so in the completion report and leave it alone.

Failure-mode coverage from the spec's table that this prompt's tests carry: bound expiry (row 1), ctx cancel mid-wait with the child surviving (row 2, unit + detach integration), child exits non-zero (row 3), turn JSON is `is_error` / zero turns / unparseable (rows 4 and 5), and a UI request timeout shorter than the turn (row 8, the cancel path). Say in the completion report which spec covers each.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. (This container has no usable git anyway; make no git calls.)
- **Interactive branch behaviour unchanged.** `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched. The only permitted interactive-branch edit is the already-extracted `validateSessionTurn` call — do not re-extract it, do not change its strings.
- **Detachment preserved.** `exec.Command` (NOT `CommandContext`), `Setpgid`, stdout redirected to the caller-owned temp file, stderr to `os.DevNull`. NEVER SIGKILL the child on timeout — the 30m bound is a wait-channel select, not a context kill. Do NOT resurrect `"claude session start timed out"`.
- **Never offer a broken Resume.** On any failure (child exit error, `is_error`, 0 turns, unparseable output, bound expiry, ctx cancel) `StartSession` returns an error so the caller persists nothing. Returning nil on ctx-cancel is WRONG — it would persist an id for a still-running child.
- **Do not revert spec 045.** The child-exit branch's verdict precedence (validated result outranks the exit code) and the current predicate wording (`num_turns`, `is_error`) are spec 045's, shipped after this spec. Spec 041's Design sketch for those two points is historical.
- **JSON validation.** `num_turns > 0` AND `is_error == false`. Lowercase UUIDs; keep `-n "<task name>"` at mint so resume inherits the title.
- **Error idiom.** `errors.Wrapf(ctx, err, …)` / `errors.Wrap(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- `sessionTurnTimeout` stays a tunable const — do NOT add a config field (spec Open Question 1; no second caller exists).
- Do NOT alter any error string to satisfy a grep pattern (requirement 11).
- `ClaudeSessionStarter.StartSession`'s signature is UNCHANGED (ctx, sessionID, prompt, cwd, name, isInteractive) — `mocks/claude-session-starter.go` is untouched. The `SessionLocker` constructor parameter (spec 042) is already wired in and must stay.
- Do NOT touch `pkg/ops/workon.go`, `pkg/ops/goal_workon.go`, `pkg/ops/workon_test.go`, `pkg/ops/goal_workon_test.go`, `pkg/ops/workon_session_writeback_test.go`, `docs/work-on-session-lifecycle.md`, `CHANGELOG.md`, or anything under `scenarios/`. Prompts 2 and 3 own those.
- Tests use Ginkgo v2 + Gomega with counterfeiter mocks — no stdlib `t.Run` table tests.
- Existing tests must still pass.
</constraints>

<verification>
Run everything from the repo root.

**Full gate — `make precommit` must exit 0.** If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make gosec`, `make test`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each of these must exit 0. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0 and the daemon does not check `<verification>` exit codes, so the comment-only form reports success on unchanged code.

```
test "$(grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go)" -ge 1
test "$(grep -c 'validateSessionTurn' pkg/ops/claude_session.go)" -ge 2
test "$(grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go)" -ge 1
test "$(grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go)" = "1"
test "$(grep -c 'Expect(capturedWindow).To(Equal(30 \* libtime.Minute))' pkg/ops/claude_session_test.go)" = "1"
test "$(grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go)" = "1"
test "$(grep -c 'ContainSubstring("num_turns")' pkg/ops/claude_session_test.go)" -ge 3
test "$(grep -c 'claude session exited with error' pkg/ops/claude_session.go)" = "1"
test "$(grep -c 'did not complete within' pkg/ops/claude_session.go)" = "1"
test "$(grep -c 'defaultCommandRunner' pkg/ops/claude_session.go)" = "3"
test "$(grep -c 'context.WithTimeout' pkg/ops/claude_session.go)" = "1"
! grep -Eq 'livenessWindow|exited during startup|claude session start timed out' pkg/ops/claude_session.go
! grep -rq 'livenessWindow' pkg/
! grep -q 'fmt.Errorf' pkg/ops/claude_session.go
```

AC10's `scenarios/005` guard, substituted because this container's `.git` is masked (`git diff --exit-code HEAD` cannot run — every git call dies with `fatal: not a git repository`). The hash and byte count below pin the file as it stands today; both must hold. If a digest check fails, you edited the scenario — revert your edit. Do NOT update the expected value, and do NOT add a `git` command.

```
test "$(sha256sum scenarios/005-work-on-resume-auto-invokes-subtask.md | cut -d' ' -f1)" = "973840d5a8c6a55cb84c6db10c9c24ab2ff269b1ba0fa82e31ab1ac630ea0153"
test "$(wc -c < scenarios/005-work-on-resume-auto-invokes-subtask.md)" = "7533"
test "$(grep -c 'claude_session_id' scenarios/005-work-on-resume-auto-invokes-subtask.md)" = "3"
```

If `sha256sum` is not present in this image, the byte count plus the `claude_session_id` line count are the substitute and both must still hold; note the missing tool in `## Improvements`.

Syntax and suite:

```
gofmt -e -l pkg/ops/claude_session.go pkg/ops/claude_session_test.go pkg/ops/export_test.go
```
must list NO files.

```
go test ./pkg/ops/...
```
must pass (unpiped — never pipe a test command, the pipeline would report the last stage's status).
</verification>

<!-- DARK-FACTORY-REPORT -->
