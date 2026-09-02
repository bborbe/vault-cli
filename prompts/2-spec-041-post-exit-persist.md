---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-02T12:05:00Z"
---

<summary>
- Confirms non-interactive task and goal work-on start the headless turn first and persist the session id only after it exits cleanly (the fix shipped in v0.117.1; the tree already matches the spec Design).
- Confirms every failure path (spawn error, exit error, invalid turn, timeout, cancel) leaves no session id on disk — no compensating clear exists.
- Confirms the tests assert the storage write happens strictly after the child exits and that the written id equals the id the child was spawned with.
- Confirms the writeback invariant survives: frontmatter the headless turn wrote (phase, session_note) is preserved by the post-exit re-read/write.
- Confirms the writeback test fakes write valid JSON to the stdout file and exit cleanly so the real starter's validation passes.
- Confirms the cached-session path is unchanged.
- Small cleanup: rewords the stale "liveness window" comments in the writeback test to the post-exit terminology the spec ships (the concept was removed; the comments reference it).
- No new behavior is expected in this prompt — it is a confirm pass over the two workon files and their tests, plus the comment cleanup. Correct only genuine mismatches, which are not expected.
</summary>

<objective>
Confirm that both `workon.go` and `goal_workon.go` persist the session id only AFTER the detached headless turn has finished, so no failure leaves a resumable-looking id behind. This prompt covers spec-041 ACs 7-9 and depends on prompt 1 having shipped (its `StartSession` block-until-exit behavior is what these tests exercise).
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file; focus on `handleClaudeSession` (starts ~line 248) and `persistSessionAndMetrics` (~line 217).
- `pkg/ops/goal_workon.go` — the whole file; focus on `handleClaudeSession` (~line 198) and `persistGoalSessionID` (~line 174).
- `pkg/ops/workon_test.go` — the whole file; the AC7 context "when persisting the session id after the child exits" starts at line 877.
- `pkg/ops/goal_workon_test.go` — the whole file; the AC7 context "when persisting the goal session id after the child exits" starts at line 355.
- `pkg/ops/workon_session_writeback_test.go` — the whole file.
- `docs/work-on-session-lifecycle.md` — the design record for the post-exit ordering.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` idiom.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

NOTE: git IS available in this container (`workflow: direct`, no hideGit) — but this prompt has no git commands; AC10's `scenarios/005` guard is verified in prompt 1.
</context>

<requirements>
The target state for this prompt already exists in the tree (shipped in v0.117.1). Your job is to CONFIRM each piece matches the spec-041 Design below and correct only genuine mismatches (not expected). The only edit this prompt is expected to make is the comment reword in requirement 7.

1. **Guard — prompt 1 must have shipped.** Before doing anything, confirm prompt 1's deliverables exist: `grep -c 'validateSessionTurn' pkg/ops/claude_session.go` >= 2, `grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go` >= 1, and `grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go` >= 1. If ANY of these is 0, STOP and report `"status":"failed"` with message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed"` — do not proceed.

2. **Confirm the start→persist reorder in `workon.go`.** In `handleClaudeSession`'s non-interactive branch, the order must be: (a) capture `startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())` BEFORE the spawn (the turn's true start, not the write time); (b) call `w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive)` FIRST; (c) on error return `errors.Wrap(ctx, err, "start claude session")` with NO compensating clear (the comment must say nothing was written for this id, so there is nothing to undo); (d) only after a clean return call `persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)`. This must be structurally identical to the interactive branch. The cached-session path (existing id → re-read + re-persist) must be unchanged. If the order is wrong (persist-before-spawn), fix it to the above.

3. **Confirm the same reorder in `goal_workon.go`.** `handleClaudeSession`'s non-interactive branch must: call `g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive)` first; on error return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear; then `persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage)`. The cached-session path (existing id → `return existing, nil`) must be unchanged. If wrong, fix it.

4. **Confirm the dead code is deleted.** `grep -rn 'clearSessionAndMetrics' pkg/` and `grep -rn 'clearGoalSession' pkg/` must both return NOTHING (AC9). If either symbol exists, delete the function and its call sites and any test that references it. The doc comments on `persistSessionAndMetrics`, `persistGoalSessionID`, and both `handleClaudeSession` methods must state that the re-read is load-bearing on every branch because the turn mutates the same file, and that nothing is written on any failure path so there is no compensating clear. On persist failure both helpers return an EMPTY id (never the one they were handed) — confirm that (`persistSessionAndMetrics` returns `""` on re-read/write error; `persistGoalSessionID` the same).

5. **Confirm AC7 in `workon_test.go`.** The context "when persisting the session id after the child exits" (line 877; drives `Execute` with `isInteractive=false`) must declare `writeTaskAt`, `childExitAt time.Time` and `writtenSessionID`, `spawnedSessionID string`, and its `BeforeEach` must use a real starter via `ops.NewClaudeSessionStarterWithRunner` whose fake `detachRun` captures `spawnedSessionID` from the `--session-id` argv, writes valid turn JSON to the stdout `*os.File` (else `StartSession` returns "parse claude output"), records `childExitAt = time.Now()`, then `done <- nil`; a BLOCKING waiter (`libtime.WaiterDurationFunc` that blocks on a spec-local `blockWaiter` channel closed in `DeferCleanup`) — a nil-returning waiter would race the select and flip nondeterministically between success and the timeout branch. The spec "writes the session id to storage only after the child exits" must assert, in this order:
   ```go
   Expect(err).To(BeNil())
   Expect(writeTaskAt).NotTo(BeZero())
   Expect(childExitAt).NotTo(BeZero())
   Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
   Expect(writtenSessionID).To(Equal(spawnedSessionID))
   ```
   If any of these assertions is absent, add it.

6. **Confirm AC7 in `goal_workon_test.go`.** The context "when persisting the goal session id after the child exits" (line 355) must drive `Execute` with `isInteractive=false`, use a real starter with a blocking waiter, write valid JSON to stdout and `done <- nil`, and assert `writeGoalAt.After(childExitAt)` with both non-zero. If absent, add it.

7. **REWORD the stale "liveness window" comments in `workon_session_writeback_test.go`.** This test file shipped with spec 040's terminology, which spec 041 removed. The concept is gone from the design record (`docs/work-on-session-lifecycle.md`); these comments must not keep referencing it. Reword, with no behavior change and no assertion change:
   - The `Context("when the child exits non-zero inside the liveness window", ...)` declaration (~line 268) → rename the context string to `"when the child exits non-zero within the turn wait"` (update the corresponding `It` title at ~line 316 similarly — "inside the window" → "within the turn wait").
   - The two BeforeEach comments "while the parent has already returned within the liveness window" (task ~line 130 and goal ~line 213) → "while the parent blocks waiting for the detached turn" (matching the shipped post-exit ordering).
   - The comment "The liveness window has NOT elapsed when the child exits..." (~line 284) → "The turn wait has NOT elapsed when the child exits...".
   Do NOT touch the pinned-count strings `TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()` in this file — the AC8 greps must stay byte-identical.

8. **Confirm AC8 — the writeback invariant.** `pkg/ops/workon_session_writeback_test.go` must contain (task and goal contexts): a fake `detachRun` that, while the parent is blocked in `StartSession`, (a) re-reads the task/goal from the real storage, sets phase to `execution`, writes a `session_note` field, and writes it back (simulating the real headless turn's own frontmatter write), (b) writes valid JSON to the stdout `*os.File`, and (c) exits cleanly via `done <- nil` with a blocking waiter. The specs must assert the phase and `session_note` the child wrote SURVIVE, `ClaudeSessionID() == pinnedSessionID`, and `MetricsSessions()` has length 1. These assertions must remain byte-identical to today's (the pinned counts are in `<verification>`). If the fakes do NOT write JSON / exit cleanly, the test would currently be failing at `StartSession` validation — fix the fakes, not the assertions. The valid JSON line the fakes write is:
   ```go
   _, _ = stdout.WriteString(
       `{"session_id":"` + pinnedSessionID + `","num_turns":3,"is_error":false,"result":"done"}`,
   )
   ```

9. **Confirm AC9 — no clear-failure tests remain.** No test may reference `clearSessionAndMetrics` / `clearGoalSession` or assert a compensating clear. The "when the child exits non-zero..." context in `workon_session_writeback_test.go` must instead assert that `Execute` returns an error containing `"start work-on session"` and `"exit status 1"`, `result.Success == false`, and the raw task file contains NO `claude_session_id:` and does NOT contain `pinnedSessionID` — nothing was ever persisted. If an old clear-based test exists anywhere, delete it.

10. **Confirm failure-mode rows 6, 7, 9 from the spec table.**
    - Row 6 (claude binary missing): `ErrStarterUnavailable` soft path unchanged — `workon.go` `handleClaudeSession` returns `"" , ErrStarterUnavailable` when `w.starter == nil`, surfaced as a warning (not a hard failure) by `Execute`. Confirm `grep -c 'ErrStarterUnavailable' pkg/ops/workon.go pkg/ops/goal_workon.go` >= 1 in each.
    - Row 7 (post-exit persist fails): `persistSessionAndMetrics` / `persistGoalSessionID` return an error AND an empty id when the re-read or write fails; the task keeps whatever the turn wrote; no id lands → button shows Start. Confirm the "never appends a metrics entry when the post-spawn re-read fails" spec in `workon_test.go` (~line 871) and its goal equivalent.
    - Row 9 (two Start clicks): confirm NO double-start guard was added by this spec (it is a documented residual risk, Non-goals) — the actual double-writer guard is spec 042's per-session lock, which is out of this prompt's scope and already landed. Do not add a guard here.

11. **Self-check.** Re-read the changed hunks and walk ACs 7-9 against them. Run the `<verification>` block and confirm every expected-to-pass grep holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Interactive branch behavior unchanged. `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched.
- No compensating clear: the id is never pre-written, so `clearSessionAndMetrics` / `clearGoalSession` are dead code — they must stay deleted. On failure the task simply carries no id.
- Persist-after-exit is race-free: the child has already exited before the post-exit persist, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- The `ClaudeSessionStarter` interface signature is UNCHANGED — `mocks/claude-session-starter.go` is untouched.
- Do NOT add a double-Start guard and do NOT add any config knob (both are spec Non-goals / Open Question 1).
- The AC8 pinned-count strings (`TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()`) in `workon_session_writeback_test.go` must remain byte-identical — requirement 7's comment reword must not touch any assertion.
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — spec evidence greps. Run each and record the count:

```
grep -c 'clearSessionAndMetrics' pkg/ops/workon.go                     # == 0 (AC9)
grep -c 'clearGoalSession' pkg/ops/goal_workon.go                      # == 0 (AC9)
grep -c 'After(childExitAt)' pkg/ops/workon_test.go                    # >= 1 (AC7)
grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go               # >= 1 (AC7)
grep -c 'writtenSessionID' pkg/ops/workon_test.go                      # >= 1 (AC7)
grep -c 'spawnedSessionID' pkg/ops/workon_test.go                      # >= 1 (AC7)
grep -c 'ErrStarterUnavailable' pkg/ops/workon.go                      # >= 1 (Failure Modes row 6)
grep -c 'ErrStarterUnavailable' pkg/ops/goal_workon.go                 # >= 1 (Failure Modes row 6)
# AC8 writeback invariant counts — must hold exactly, deletion-safe:
grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go  # == 2
grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go  # == 2
grep -c 'session_note' pkg/ops/workon_session_writeback_test.go        # == 4
grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go   # == 2
grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go   # == 2
# AC9 — failure path asserts nothing persisted:
grep -c 'start work-on session' pkg/ops/workon_session_writeback_test.go  # >= 1
```

SECONDARY — syntax validity:
```
gofmt -e -l pkg/ops/workon.go pkg/ops/goal_workon.go pkg/ops/workon_test.go pkg/ops/goal_workon_test.go pkg/ops/workon_session_writeback_test.go   # must list NO files
```

TESTS:
```
make test   # must exit 0 (spec 042's build break is resolved — the package compiles)
```

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running only `make test` + the grep gate here is correct.
</verification>
