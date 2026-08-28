---
status: draft
---

## Summary

- The v0.116.3/0.116.4 fix (spec 040) made the **non-interactive** branch of `StartSession` persist `claude_session_id` **before** spawning the detached headless `claude --print` turn, then return after a 10s `livenessWindow` while the turn keeps running (2–5+ min).
- The Vault UI watcher sees the id on the task at ~10s and flips the button to **▶ Resume** while the turn is still running. Clicking Resume runs `claude --resume <uuid>` against a transcript that is **mid-write by another process**: "session not found" first, then progressively more of the initial work on each reopen, and two writers on one jsonl (a corruption risk, not just a UX nuisance).
- Fix: the non-interactive branch **blocks until the detached child exits** (its headless turn completes), captures the turn's JSON output to a temp file and validates it, and only **then** does the caller persist `claude_session_id`. The id is never on disk while a child exists, so the button shows `⏳ Starting` for the whole turn and flips to `▶ Resume` exactly when the transcript is complete and resumable.
- This **reverses spec 040's non-interactive design** (persist-before-spawn + 10s liveness + compensated-failure path + "`--output-format json` is dead weight on the detached branch"). The interactive TTY branch, `defaultCommandRunner`, and `scenarios/005` are untouched. Spec 040 stays as the historical record.

## Goal

After this work, a **non-TTY** `task work-on` (the Vault UI Start button) persists `claude_session_id` only once the headless turn has completed, so the button holds `⏳ Starting` for the turn and offers `▶ Resume` only against a complete, single-writer transcript. A turn that fails — exit non-zero, `is_error:true`, 0 turns, or the bound expiring — leaves **no id**, so the button reverts to `▶ Start` rather than offering a broken Resume. The **TTY** start-then-resume flow behaves exactly as it does today.

## Problem

The shipped fix's contract was "return a session id in ~10s". That contract is what breaks Resume: an id on the frontmatter means "a session exists", but it does **not** mean "a session is resumable". The turn writes the transcript for minutes after the id appears. The Vault UI's button keys off `claude_session_id` presence (`sessionButtonHtml`, `hasSession`), so it advertises Resume while `claude --resume` cannot succeed.

Two consequences:
- **The operator cannot trust the button.** Resume fails, and on re-open the conversation replays the turn's tail as the writer advances — the same "live transcript" confusion the headless path was built to avoid.
- **Two writers on one transcript.** The resumed TTY and the detached child both hold the same session file concurrently. Even a "successful" resume mid-turn risks interleaved writes on the jsonl.

The verification that shipped the previous fix marked "card flips to Resume" as success without ever running `claude --resume` — the gap that let this through.

## Reproduction

vault-cli `v0.116.4` installed (`v0.116.4-dirty` on the operator's PATH at the time); observed 2026-08-28 on the Brogrammers vault (task `BRO-21734 Check Alerts`).

Setup — any task whose `/vault-cli:work-on-task` bootstrap turn takes longer than ~10s (all real tasks; a trivial task may not reproduce).

Action — click **Start** on the task in the Vault UI Kanban, wait for the card to flip to **▶ Resume** (~10s), copy the offered command and run it:

```bash
/Users/bborbe/Documents/workspaces/scripts/cc-brogrammers-deepseek --resume 96bc2eda-acf4-4402-bca7-9f93c5bcb02a
```

Observed — the resume session replays the still-running headless turn:

```
❯ /vault-cli:work-on-task "/Users/bborbe/Documents/Obsidian/Brogrammers/24 Tasks/BRO-21734 Check Alerts.md"
--non-interactive
  Press Ctrl-C again to exit
Resume this session with:
claude --resume "BRO-21734 Check Alerts"
```

On the first attempt the operator reports "session not found"; re-opening a few seconds later shows the first ~10s of the turn, and each reopen shows more — the transcript is being written by the detached child concurrently.

Root-cause evidence, `pkg/ops/claude_session.go` (v0.116.4):

```go
// non-interactive branch
done, err := c.detachRun(args, cwd)          // spawn detached
go func() { waitCh <- c.waiter.Wait(ctx, c.livenessWindow) }()  // 10s
select {
case exitErr := <-done:                       // only consumed if child dies in-window
    return errors.Errorf(ctx, "claude session exited during startup: %v", exitErr)
case err := <-waitCh:
    return nil                                 // returns after 10s, child abandoned
}
```

and `pkg/ops/workon.go` `handleClaudeSession`:

```go
sessionID := w.uuidGenerator()
persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)  // BEFORE spawn
w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive)
```

## Expected vs Actual

| | Expected (this spec) | Actual (v0.116.3/0.116.4) |
|---|---|---|
| Start click | `⏳ Starting` for the whole headless turn | flips to `▶ Resume` at ~10s |
| During the turn | no `claude_session_id` on the task | id present (persist-before-spawn) |
| Resume click (when offered) | `claude --resume <uuid>` opens the **complete** bootstrap conversation | "session not found" / partial replay / two writers |
| Turn fails (exit ≠ 0, `is_error`, 0 turns, bound) | no id persisted; button back to `▶ Start` | id persisted at ~10s regardless → broken Resume offered |

## Why this is a bug

`work-on` exists so the Start button hands the operator a session they can actually resume. Shipping an id at 10s hands them an id whose session is still being written — the button lies, the resume fails, and the transcript can be corrupted by the second writer. The prior spec's own acceptance criterion ("the generated UUID is the id the session actually uses — `claude --resume <uuid>` opens the bootstrap conversation") was never exercised on the non-interactive path; the verification stopped at "the jsonl exists".

## Non-goals

- No change to the interactive TTY branch, `defaultCommandRunner`, or the 5m TTY cap — turn 2 `syscall.Exec`s `claude --resume` against turn 1's on-disk result, so the blocking wait stays correct there.
- No change to how the turn works (guide loading, plan→execute chain) — the fix is to stop advertising Resume before the turn is done, not to make the turn faster.
- No vault-ui frontend/backend changes in this spec — the button already renders `⏳ Starting` when no id is present; only the id's timing changes. The "Creating session… up to 2 minutes" modal copy is a separate cosmetic change (Open Question 2).
- No config field for `sessionTurnTimeout` — a tunable const, per Open Question 1.
- No double-Start guard — two concurrent starts are a documented residual risk (Failure Modes row 7), not fixed here.

## Do-Nothing Option

Doing nothing keeps the status quo: the button flips to `▶ Resume` at ~10s, clicking it fails ("session not found", partial replay) and risks a second writer on the transcript. That is the exact failure this spec exists to remove — it is not "safe current behavior", it is a shipped bug (Problem). The alternative of reverting spec 040 wholesale (back to the 5m kill) is strictly worse — it reintroduces the kill-mid-write and the "no id persisted on timeout" failure. The 30-min bound is the cost that buys "Starting until done" without either erroring on normal turns or hanging forever on a pathological one.

## Constraints

- **Interactive branch behavior unchanged.** `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched. The only edit to the interactive branch is extracting its inline JSON validation into the shared `validateSessionTurn` helper — behavior-preserving, same checks, byte-identical error strings. Evidence: `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` (HEAD-relative, so a prompt committing a change before verifying cannot pass).
- **Detachment preserved.** `exec.Command` (NOT `CommandContext`), `Setpgid`, stdout/stderr handling that lets the child survive the parent — never SIGKILL the child on timeout; `--max-turns` is inert (`-1`), so the 30-min bound is a **wait-channel select**, not a context kill. Do NOT resurrect `"claude session start timed out"`.
- **Never offer a broken Resume.** On any failure (exit error, `is_error`, 0 turns, bound expiry, ctx cancel) `StartSession` returns an **error** so the caller persists nothing. Returning `nil` on ctx-cancel is now WRONG (it would persist an id for a still-running child) — the previous spec's cancel-returns-nil only worked because the id was already pre-persisted.
- **JSON validation.** `num_turns > 0` AND `is_error == false`. `session_id` is always present even for a dead session, so checking the id alone never catches a failure. Lowercase UUIDs; keep `-n "<task name>"` at mint so resume inherits the title.
- **Error idiom.** `errors.Wrapf(ctx, err, …)`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- **No compensating clear.** The id is never pre-written, so `clearSessionAndMetrics` / `clearGoalSession` become dead code — delete them. On failure the task simply carries no id.
- **Persist-before-spawn → persist-after-exit is still race-free.** The child has already exited before the post-exit persist, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes (the [[Fix vault-cli work-on Clobbering Task Frontmatter After Headless Turn]] invariant holds — it is the interactive branch's proven ordering).

## Design

### `pkg/ops/claude_session.go`

1. **Constant rename:** `livenessWindow = 10 * libtime.Second` → `sessionTurnTimeout = 30 * libtime.Minute`. Comment: bounds the wait for the detached turn's exit — never a kill; the child is detached and survives expiry.
2. **`defaultDetachedRunner`:** signature → `func(args []string, dir string, stdout *os.File) (<-chan error, error)`; `cmd.Stdout = stdout` (caller-owned temp file), `cmd.Stderr = devNull` (stderr still discarded — a crash surfaces via exit code; keeps the `os.DevNull` reference). Do NOT close the caller-owned stdout file. Keep `exec.Command`, `Setpgid`, the buffered `done` reaper, and the spawn audit log.
3. **Non-interactive branch** (replaces lines 177–204):
   - `outFile, err := os.CreateTemp("", "vault-claude-session-*.json")`; `defer` remove+close (covers spawn failure and the cancel/timeout early returns; unlink-before-close is safe on POSIX while the child may still hold the fd).
   - `done, err := c.detachRun(args, cwd, outFile)` (spawn error → wrapped error, unchanged).
   - Waiter goroutine: `waitCh <- c.waiter.Wait(ctx, c.sessionTurnTimeout)`.
   - `select`:
     - `case exitErr := <-done` → non-nil → `errors.Errorf(ctx, "claude session exited with error: %v", exitErr)`; nil → read the file, validate.
     - `case err := <-waitCh` → `err != nil` means ctx cancelled → `errors.Wrap(ctx, err, "claude session wait cancelled")`; `err == nil` means bound expired → `errors.Errorf(ctx, "claude session turn did not complete within %v", c.sessionTurnTimeout)`. **Both are errors.** The child survives detached either way.
   - After a clean exit: `output, err := os.ReadFile(outFile.Name())` then `validateSessionTurn(output)`.
4. **Extract `validateSessionTurn(output []byte) error`** from the interactive branch's inline block (lines 219–239) — same struct, same checks, **byte-identical error strings** ("parse claude output", "claude returned empty session_id", "claude returned 0 turns: %s", "claude reported error: %s"). Call it from both branches. `defaultCommandRunner` itself is untouched.

### `pkg/ops/workon.go`

`handleClaudeSession` non-interactive branch: reorder to **start → persist** (structurally identical to the interactive branch):

```go
startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time()) // captured BEFORE spawn — the turn's true start
if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
    // Nothing was persisted for this id (no pre-spawn write) — nothing to compensate.
    return "", nil, errors.Wrap(ctx, err, "start claude session")
}
sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
return sessionID, nil, err
```

Delete `clearSessionAndMetrics` (dead code). Cached-session path unchanged. Update the doc comments on `persistSessionAndMetrics` and `handleClaudeSession` (the re-read is now load-bearing on both branches).

### `pkg/ops/goal_workon.go`

Same reorder — `StartSession` first, then `persistGoalSessionID`. Delete `clearGoalSession`. Cached path unchanged. Doc comments updated.

### `pkg/ops/export_test.go`

`const LivenessWindow = livenessWindow` → `const SessionTurnTimeout = sessionTurnTimeout` (comment updated).

### Interface / mocks

`ClaudeSessionStarter.StartSession` signature is unchanged — `mocks/claude-session-starter.go` is untouched.

## Acceptance Criteria

1. **`StartSession` blocks until the detached child exits (non-interactive).** Unit test: with a blocking waiter, `StartSession` must not return until the test's fake `done` fires; assert the waiter receives `ops.SessionTurnTimeout` (wiring) and that equals `30 * libtime.Minute` (value). Evidence: `grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go` ≥ 1, `grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go` ≥ 1 (lowercase — the source identifier is unexported; the capital form exists only in `export_test.go`), `grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go` ≥ 1.
2. **A clean child exit proceeds to validation and succeeds.** Unit test: fake writes valid JSON to the stdout file, `done <- nil`; `StartSession` returns nil and the temp file is removed. Evidence: `grep -c 'validateSessionTurn' pkg/ops/claude_session.go` ≥ 2 (both branches), temp-file cleanup covered by the test.
3. **Turn JSON is validated on the non-interactive branch** (`is_error`, 0 turns, unparseable). Unit tests: `{"num_turns":0,...}` → "0 turns" error; `{"is_error":true,...}` → "reported error"; empty file → "parse claude output". Same error strings as the interactive branch (lock: existing interactive tests unchanged). Evidence: `grep -c '"0 turns"' pkg/ops/claude_session_test.go` ≥ 1, `grep -c 'validateSessionTurn' pkg/ops/claude_session.go` ≥ 2.
4. **Child exit with error → error, nothing persisted.** Unit test: `done <- errors.New("exit status 1")`, blocking waiter → error wrapping "exit status 1" under the NEW wording "claude session exited with error" — the existing assertion `ContainSubstring("exited during startup")` (claude_session_test.go:317) must be updated to the new string. The "nothing persisted" half is observable only through prompt 2's reworked `workon_test.go` (AC7) — state it there, not here. Evidence: `grep -c '"claude session exited with error"' pkg/ops/claude_session.go` ≥ 1, `grep -c 'exited during startup' pkg/ops/claude_session.go` == 0.
5. **Bound expiry → error, nothing persisted.** Unit test: waiter returns nil immediately, `done` never fires → "did not complete within" error. Evidence: `grep -c '"did not complete within"' pkg/ops/claude_session.go` ≥ 1.
6. **Ctx cancellation mid-wait → error, child survives.** Unit test: blocking waiter, cancel ctx → error (NOT nil). Integration test: real `sleep 12; touch sentinel` script, cancel at ~1s → StartSession returns <12s with an error, sentinel still appears later (detachment invariant preserved). Evidence: the integration test asserts the sentinel appears after the cancelled return.
7. **`writeTaskAt` is AFTER `childExitAt` (post-exit persist).** `pkg/ops/workon_test.go` / `goal_workon_test.go`: reworked "persisting the session id" tests assert `writeTaskAt.After(childExitAt)` and `writtenSessionID == spawnedSessionID`. Evidence: `grep -c 'After(childExitAt)' pkg/ops/workon_test.go` ≥ 1, `grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go` ≥ 1.
8. **The writeback invariant survives.** `workon_session_writeback_test.go`: frontmatter the child wrote (phase `execution`, `session_note`) survives the post-exit persist; `ClaudeSessionID() == pinnedSessionID`; `MetricsSessions()` len 1. Assertions byte-identical to today; the fake shape changes in TWO ways: the fakes must write a **valid JSON line to the stdout `*os.File`** (else `StartSession` returns "parse claude output" and the test's `Expect(err).To(BeNil())` fails) AND exit cleanly via `done <- nil` with a blocking waiter. Evidence: the existing assertion greps (`TaskPhaseExecution` 2, `GoalPhaseExecution` 2, `session_note` 4, `MetricsSessions()` 2, `ClaudeSessionID()` 2) still return their pinned counts.
9. **No compensating clear remains.** `grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 0, `grep -c 'clearGoalSession' pkg/ops/goal_workon.go` == 0. The old clear-failure tests are deleted.
10. **Interactive branch + scenario 005 untouched.** `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` empty (HEAD-relative); `grep -c 'defaultCommandRunner' pkg/ops/claude_session.go` == 3 (current count — the two constructors assign `runCmd: defaultCommandRunner` and the func is defined once; pinned so a rework cannot silently drop it); the 5m TTY cap still present (`grep -c 'context.WithTimeout' pkg/ops/claude_session.go` == 1, on the interactive branch).
11. **Docs/scenario updated.** `docs/work-on-session-lifecycle.md` rewords the liveness-window sections (intro, session-id ownership, "Pre-spawn write ordering" → "Post-exit write ordering", "`--output-format json` fate" → captured+validated, "liveness window" → "turn timeout", "Compensated failure path" → "Failure path" with no-clear). `scenarios/002-task-lifecycle.md` note: "returns within ~10s (the liveness window)" → "blocks until the headless turn completes (typically minutes)". Evidence: `grep -c 'livenessWindow' docs/work-on-session-lifecycle.md` == 0, `grep -ci 'liveness window' docs/work-on-session-lifecycle.md` == 0 (prose form too — the reword must not leave the concept behind under different casing), `grep -c '~10s' scenarios/002-task-lifecycle.md` == 0.
12. **CHANGELOG.** A `## Unreleased` bullet describes the inversion (non-interactive `task work-on`/`goal work-on` wait for the detached turn to exit before persisting `claude_session_id`, bounded by a 30-min turn timeout; TTY branch unchanged). Evidence — `## Unreleased` must exist (v0.116.6 consumed it, so the prompt creates it) and carry the bullet: `grep -c '^## Unreleased' CHANGELOG.md` ≥ 1 AND `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'` ≥ 1.
13. **Full gate.** `make precommit` exits 0.

## Failure Modes

| Mode | Expected behavior / Detection | Recovery |
|---|---|---|
| Turn exceeds 30-min bound | `StartSession` returns "did not complete within 30m0s" error; no id persisted | Child survives detached and eventually completes; button back to `▶ Start`; re-click starts a fresh turn (the orphaned turn's transcript is unreferenced) |
| Ctx cancelled mid-wait (vault-ui request dies before the turn ends) | `StartSession` returns cancel error; no id persisted | Child survives detached and completes; button shows `▶ Start`; the completed transcript is orphaned unless the user re-runs work-on (which mints a new id) — strictly safer than today's broken Resume |
| Child exits non-zero (bad flag, auth failure, skill crash) | `StartSession` returns "exited with error"; no id persisted | Error surfaces to vault-ui, which clears its own `claude_session_started` flag (vault-ui's sweep — out of this spec's scope), button back to `▶ Start` |
| Turn completes but JSON says `is_error:true` / 0 turns | `validateSessionTurn` returns the interactive branch's error strings; no id persisted | Same recovery as the previous row |
| Temp file unreadable / empty | `validateSessionTurn` → "parse claude output"; no id persisted | No false success; same recovery |
| `claude` binary missing (`ErrStarterUnavailable`) | Unchanged: `StartSession` never constructed (workon.go:119-121 / goal_workon.go:110-112) | vault-ui surfaces the existing "claude binary missing" soft-failure path; task stays `in_progress` with no id — same as today |
| Post-exit persist fails (storage error on the re-read/write) | `persistSessionAndMetrics` error after a clean turn | Turn's frontmatter survives (it wrote it); the id is not persisted → button shows `▶ Start`; user re-clicks, which mints a fresh id and a fresh turn (the completed transcript is orphaned) |
| UI request timeout < turn duration | vault-ui kills the subprocess → ctx cancel path (row 2) | Document as a vault-ui-side consideration (client timeout ≥ the 30-min bound, or treat the error as "still starting"); out of scope for vault-cli |
| Two Start clicks on one task | Second `work-on` short-circuits on the first's id if present; if the first hasn't persisted yet, both spawn | Two detached children can both write the task (last-writer-wins on frontmatter); NOT jsonl corruption — ids differ → different transcripts. No mitigation in this spec (Non-goals); the window widens from ~10s to the whole turn — accepted, documented residual risk |

## Suggested Decomposition

Three prompts, driven by the daemon (`autoGeneratePrompts: true`):

| # | Prompt focus | Covers ACs | Depends on |
|---|---|---|---|
| 1 | `claude_session.go` + `export_test.go` + `claude_session_test.go` + `claude_session_detach_test.go` — constant rename, temp-file capture, `validateSessionTurn` extraction, the select rework, unit/integration test matrix | 1–6, 10 (the `defaultCommandRunner`==3 guard) | — |
| 2 | `workon.go` + `goal_workon.go` + `workon_test.go` + `goal_workon_test.go` + `workon_session_writeback_test.go` — start→persist reorder, delete compensating clears + their tests, writeback fake rework (must write valid JSON to the stdout `*os.File`) | 7–9 | 1 |
| 3 | `docs/work-on-session-lifecycle.md` + `scenarios/002-task-lifecycle.md` + `CHANGELOG.md` — reword + create `## Unreleased` + bullet | 11–12 | 1, 2 |

AC13 (`make precommit`) is the full-gate check across all three. Spec 040 (`specs/completed/040-…`) is the historical record — do NOT edit it; reference it from this spec.

## Workaround

Until this ships: do not click Resume in the Vault UI until the initial work has visibly finished (the card shows `⏳ Starting` then `▶ Resume`; wait for the flip to have settled, or wait several minutes after clicking Start). Prefer the TTY `vault-cli task work-on` path for critical tasks — it blocks through the turn before handing over.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make test        # exit 0; the new block-until-exit + validation tests run
```

```
grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                        # >= 1 (lowercase — source identifier)
grep -c '30 \* libtime.Minute' pkg/ops/claude_session_test.go                 # >= 1 (value pin)
grep -c 'validateSessionTurn' pkg/ops/claude_session.go                       # >= 2 (both branches)
grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go  # >= 1
grep -c 'clearSessionAndMetrics' pkg/ops/workon.go                            # == 0
grep -c 'clearGoalSession' pkg/ops/goal_workon.go                             # == 0
grep -c 'After(childExitAt)' pkg/ops/workon_test.go                           # >= 1
grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go                      # >= 1
# writeback invariant counts (AC8) — must hold exactly, deletion-safe:
grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go         # == 2
grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go         # == 2
grep -c 'session_note' pkg/ops/workon_session_writeback_test.go               # == 4
grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go          # == 2
grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go          # == 2
# AC10 guards:
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go                      # == 3
grep -c 'context.WithTimeout' pkg/ops/claude_session.go                       # == 1 (interactive branch)
# AC11 docs/scenario reword:
grep -c 'livenessWindow' docs/work-on-session-lifecycle.md                    # == 0
grep -ci 'liveness window' docs/work-on-session-lifecycle.md                  # == 0 (prose form too)
grep -c '~10s' scenarios/002-task-lifecycle.md                                # == 0
# AC12 CHANGELOG (Unreleased must exist — v0.116.6 consumed it — and carry the bullet):
grep -c '^## Unreleased' CHANGELOG.md                                         # >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'                   # >= 1
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md  # empty
make precommit                                                               # exit 0
```

### Operator-executable (host, after PR merge + release + `make install`; spec verification ladder)

1. `vault-cli --version` — new version, not `-dirty`.
2. In the Vault UI, click **Start** on a real (non-trivial) task. Confirm the card shows `⏳ Starting` for the whole turn and flips to `▶ Resume` only once it finishes.
3. **This time actually run resume:** click Resume, take the offered `claude --resume <uuid>` command, run it in a terminal. Confirm it opens the **completed** bootstrap conversation — no "session not found", no partial replay.
4. Failure path: force a turn failure (e.g. a task whose work-on command errors) and confirm no `claude_session_id` lands on the task and the button reverts to `▶ Start`.

## Open Questions

1. Should `sessionTurnTimeout` (30 min) be configurable per-vault, or is a tunable const enough? (Recommend const for now — no second caller exists.)
2. Should the vault-ui "Creating session… up to 2 minutes" modal copy be updated in the same release (cosmetic; separate repo)?
