---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-08-30T18:52:00Z"
---

<summary>
- Confirms non-interactive task and goal work-on start the headless turn first and persist the session id only after it exits cleanly.
- Confirms every failure path (spawn error, exit error, invalid turn, timeout, cancel) leaves no session id on disk — no compensating clear exists.
- Confirms the tests assert the storage write happens strictly after the child exits and that the written id equals the id the child was spawned with.
- Confirms the writeback invariant survives: frontmatter the headless turn wrote (phase, session_note) is preserved by the post-exit re-read/write.
- Confirms the writeback test fakes write valid JSON to the stdout file and exit cleanly so the real starter's validation passes.
- Confirms the cached-session path is unchanged.
- No new code is expected in this prompt — it is a confirm-only pass over the two workon files and their tests. Correct only genuine mismatches, which are not expected.
- NOTE: `pkg/ops` may not compile until spec 042's executor lands `session_lock.go` — see constraints; do not let that derail this prompt.
</summary>

<objective>
Confirm that both `workon.go` and `goal_workon.go` persist the session id only AFTER the detached headless turn has finished, so no failure leaves a resumable-looking id behind. This prompt covers spec-041 ACs 7-9 and depends on prompt 1 having shipped (its `StartSession` block-until-exit behavior is what these tests exercise).
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file; focus on `handleClaudeSession` (starts ~line 248), `persistSessionAndMetrics` (~line 217).
- `pkg/ops/goal_workon.go` — the whole file; focus on `handleClaudeSession` (~line 198), `persistGoalSessionID` (~line 174).
- `pkg/ops/workon_test.go` — the whole file; the AC7 context "when persisting the session id after the child exits" is at line 877.
- `pkg/ops/goal_workon_test.go` — the whole file; the AC7 context "when persisting the goal session id after the child exits" is at line 355.
- `pkg/ops/workon_session_writeback_test.go` — the whole file.
- `docs/work-on-session-lifecycle.md` — the design record for the post-exit ordering.

Coding-plugin docs (read the ones relevant to these files):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` idiom.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

NOTE: this container has NO git (`.git` is not mounted). Do NOT run any `git` command.
</context>

<requirements>
The target state for this prompt already exists in the tree (shipped in v0.117.1). Your job is to CONFIRM each piece matches the spec-041 Design below and correct only genuine mismatches (not expected). Do not rewrite what is already correct.

1. **Guard — prompt 1 must have shipped.** Before doing anything, confirm prompt 1's deliverables exist: `grep -c 'validateSessionTurn' pkg/ops/claude_session.go` >= 2, `grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go` >= 1, and `grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go` >= 1. If ANY of these is 0, STOP and report `"status":"failed"` with message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed"` — do not proceed.

2. **Confirm the start→persist reorder in `workon.go`.** In `handleClaudeSession`'s non-interactive branch, the order must be: (a) capture `startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())` BEFORE the spawn (the turn's true start, not the write time); (b) call `w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive)` FIRST; (c) on error return `errors.Wrap(ctx, err, "start claude session")` with NO compensating clear (the comment must say nothing was written for this id, so there is nothing to undo); (d) only after a clean return call `persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)`. This must be structurally identical to the interactive branch. The cached-session path (existing id) must be unchanged. If the order is wrong (persist-before-spawn), fix it to the above.

3. **Confirm the same reorder in `goal_workon.go`.** `handleClaudeSession`'s non-interactive branch must: call `g.starter.StartSession(...)` first; on error return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear; then `persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage)`. The cached-session path (existing id → `return existing, nil`) must be unchanged. If wrong, fix it.

4. **Confirm the dead code is deleted.** `grep -rn 'clearSessionAndMetrics' pkg/` and `grep -rn 'clearGoalSession' pkg/` must both return NOTHING (AC9). If either symbol exists, delete the function and its call sites and any test that references it. The doc comments on `persistSessionAndMetrics` and `handleClaudeSession` (both files) must state that the re-read is load-bearing on every branch because the turn mutates the same file, and that nothing is written on any failure path so there is no compensating clear.

5. **Confirm AC7 in `workon_test.go`.** The context "when persisting the session id after the child exits" (line 877; drives `Execute` with `isInteractive=false`) must declare `writeTaskAt`, `childExitAt time.Time` and `writtenSessionID`, `spawnedSessionID string`, and its `BeforeEach` must use a real starter via `ops.NewClaudeSessionStarterWithRunner` whose fake `detachRun` captures `spawnedSessionID` from the `--session-id` argv, writes valid turn JSON to the stdout `*os.File` (else `StartSession` returns "parse claude output"), records `childExitAt = time.Now()`, then `done <- nil`; a BLOCKING waiter (`libtime.WaiterDurationFunc` that blocks on a spec-local channel closed in `DeferCleanup`) — a nil-returning waiter would race the select and flip nondeterministically between success and the timeout branch. The spec must assert, in this order:
   ```go
   Expect(err).To(BeNil())
   Expect(writeTaskAt).NotTo(BeZero())
   Expect(childExitAt).NotTo(BeZero())
   Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
   Expect(writtenSessionID).To(Equal(spawnedSessionID))
   ```
   If any of these assertions is absent, add it.

6. **Confirm AC7 in `goal_workon_test.go`.** The context "when persisting the goal session id after the child exits" (line 355) must drive `Execute` with `isInteractive=false`, use a real starter with a blocking waiter, write valid JSON to stdout and `done <- nil`, and assert `writeGoalAt.After(childExitAt)` with both non-zero. If absent, add it.

7. **Confirm AC8 — the writeback invariant.** `pkg/ops/workon_session_writeback_test.go` must contain (task and goal contexts): a fake `detachRun` that, while the parent is blocked in `StartSession`, (a) re-reads the task/goal from the real storage, sets phase to `execution`, writes a `session_note` field, and writes it back (simulating the real headless turn's own frontmatter write), (b) writes valid JSON to the stdout `*os.File`, and (c) exits cleanly via `done <- nil` with a blocking waiter. The specs must assert the phase and `session_note` the child wrote SURVIVE, `ClaudeSessionID() == pinnedSessionID`, and `MetricsSessions()` has length 1. These assertions must remain byte-identical to today's (the pinned counts are in `<verification>`). If the fakes do NOT write JSON / exit cleanly, the test would currently be failing at `StartSession` validation — fix the fakes, not the assertions. The valid JSON line the fakes write is:
   ```go
   _, _ = stdout.WriteString(
       `{"session_id":"` + pinnedSessionID + `","num_turns":3,"is_error":false,"result":"done"}`,
   )
   ```

8. **Confirm AC9 — no clear-failure tests remain.** No test may reference `clearSessionAndMetrics` / `clearGoalSession` or assert a compensating clear. The "when the child exits non-zero..." context in `workon_session_writeback_test.go` must instead assert that `Execute` returns an error containing `"start work-on session"` and `"exit status 1"`, `result.Success == false`, and the raw task file contains NO `claude_session_id:` and does NOT contain `pinnedSessionID` — nothing was ever persisted. If an old clear-based test exists anywhere, delete it.

9. **Self-check.** Re-read the changed hunks and walk ACs 7-9 against them. Run the `<verification>` block and confirm every expected-to-pass grep holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Do NOT run any `git` command (no `.git` in this container).
- Interactive branch behavior unchanged. `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched.
- No compensating clear: the id is never pre-written, so `clearSessionAndMetrics` / `clearGoalSession` are dead code — they must stay deleted. On failure the task simply carries no id.
- Persist-after-exit is race-free: the child has already exited before the post-exit persist, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- The `ClaudeSessionStarter` interface signature is UNCHANGED — `mocks/claude-session-starter.go` is untouched.
- KNOWN PRE-EXISTING BREAK — NOT YOURS: `pkg/ops/session_lock.go:85` currently fails to compile (`cannot use int(f.Fd()) (value of type int) as uintptr value in argument to unix.FcntlInt`). That is in-flight spec-042 work (`specs/in-progress/042-prevent-duplicate-session-resume.md`), owned by another executor. Do NOT fix it, do NOT delete it, do NOT touch `session_lock.go`/`session_lock_test.go`. Because of it, `make test` / `go test ./pkg/ops/...` may fail at BUILD time for reasons unrelated to this prompt. Your verification therefore rests on the grep matrix + `gofmt` syntax check; `make test` is a diagnostic (see `<verification>`).
- Existing tests must still pass (modulo the 042 build break above).
</constraints>

<verification>
PRIMARY GATE — spec evidence greps (all grep-based; none need the package to compile). Run each and record the count:

```
grep -c 'clearSessionAndMetrics' pkg/ops/workon.go                     # == 0 (AC9)
grep -c 'clearGoalSession' pkg/ops/goal_workon.go                      # == 0 (AC9)
grep -c 'After(childExitAt)' pkg/ops/workon_test.go                    # >= 1 (AC7)
grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go               # >= 1 (AC7)
grep -c 'writtenSessionID' pkg/ops/workon_test.go                      # >= 1 (AC7)
grep -c 'spawnedSessionID' pkg/ops/workon_test.go                      # >= 1 (AC7)
# AC8 writeback invariant counts — must hold exactly, deletion-safe:
grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go  # == 2
grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go  # == 2
grep -c 'session_note' pkg/ops/workon_session_writeback_test.go        # == 4
grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go   # == 2
grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go   # == 2
# AC9 — failure path asserts nothing persisted:
grep -c 'start work-on session' pkg/ops/workon_session_writeback_test.go  # >= 1
```

SECONDARY — syntax validity (no compile of the package):
```
gofmt -e -l pkg/ops/workon.go pkg/ops/goal_workon.go pkg/ops/workon_test.go pkg/ops/goal_workon_test.go pkg/ops/workon_session_writeback_test.go   # must list NO files
```

DIAGNOSTIC — `make test`:
Run `make test` at the repo root. Because of the pre-existing spec-042 `session_lock.go` build break (see constraints), `go test` may fail at BUILD time before any test runs. If the ONLY failure is that 042 compile error (and the grep matrix above holds), report `"status":"partial"` with the 042 build break explicitly named in the completion report — do NOT "fix" session_lock.go and do NOT mark the prompt failed on it. If the package DOES compile (042 already landed), `make test` must exit 0.

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running only `make test` (diagnostic) + the grep gate here is correct.
</verification>
