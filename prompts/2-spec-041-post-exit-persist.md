---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-21T14:35:53Z"
---

# Persist the task session id only after the headless turn exits (spec 041, prompt 2 of 3)

<summary>
- Changes the task path of `work-on` so the headless turn is started FIRST and the session id plus its metrics entry are written to the task only AFTER the turn exits cleanly — matching the goal path and the topic path, which already behave this way.
- Removes the compensating-clear function that exists only to undo a pre-spawn write, and the tests that exercise it. With nothing pre-written there is nothing to undo: a failed turn simply leaves no id.
- Reworks the task-side sequencing tests: the "write precedes the spawn" assertions invert, the "write before spawning" test becomes "write after the child exits" and gains an explicit ordering assertion, and the clear-based failure tests are replaced by a "persists nothing" assertion.
- Rewords the stale pre-spawn and liveness-window comments in the write-back test file to the post-exit, no-clear semantics. Every on-disk assertion in that file stays byte-identical — the ids never land either way, so only the prose and titles change.
- Confirms the goal path, the topic path, and their tests are already in the target state and leaves them untouched.
- Runs the spec's AC7-9 evidence gate plus the full repository gate. Makes no git call anywhere — `.git` is masked in this container.
- IMPORTANT FLAG FOR THE HUMAN REVIEWER: the task-side half of this spec was deliberately REVERTED in the tree after the spec was approved, because persisting only after the turn left the field empty while the child ran and the child's own session-connect then bound the task to a live, unrelated session. This prompt implements the spec as approved and re-applies the reversion's opposite; the reviewer must adjudicate at audit time (details and the two options are in the comment at the top of `<requirements>`). The docs prompt is coupled to that decision.
</summary>

<objective>
Make the task path of `work-on` persist `claude_session_id` only after the detached headless turn has finished cleanly, so no failure path can leave a resumable-looking id behind — matching the goal path and the topic path, which already ship this behaviour. Covers spec 041 ACs 7-9 and depends on prompt 1 having shipped (its block-until-exit `StartSession` is what these tests exercise).
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file (404 lines). Focus on `handleClaudeSession`, `persistSessionAndMetrics`, `clearSessionAndMetrics`, and `sessionFailureResult`.
- `pkg/ops/goal_workon.go` — the whole file (240 lines). This is the structural TEMPLATE the reordered task path must match: its `handleClaudeSession` starts the turn first and persists only after it returns cleanly, on both branches, with no compensating clear.
- `pkg/ops/topic_workon.go` — the whole file (245 lines). A THIRD sibling that postdates the spec (spec 052). Its `handleClaudeSession` is already start-then-persist with no compensating clear, and `persistTopicSessionID` already returns an empty id on failure. Read it to confirm the target shape; do NOT modify it.
- `pkg/ops/workon_test.go` — the whole file (1096 lines). The contexts this prompt reworks are `"success"`, `"when the session id write precedes the spawn"`, `"when the pre-spawn persist re-read fails"`, `"when persisting the session id before spawning"`, and `"when the spawn fails"`.
- `pkg/ops/goal_workon_test.go` — read `Context("when persisting the goal session id after the child exits", ...)` (~line 356) and `Context("goal work-on persists nothing for a failed turn", ...)` (~line 644) to confirm the goal path is already in the target state. Do not modify this file.
- `pkg/ops/workon_session_writeback_test.go` — the whole file (524 lines). Note the shared `newStarter(detachRun)` helper, the `pinnedSessionID` constant, and that every success-path fake already writes a valid turn-JSON line to the caller-owned `stdout *os.File` and then feeds `done <- nil` with a blocking waiter.
- `docs/work-on-session-lifecycle.md` — the design record. Its task-path sections currently describe the pre-spawn ordering; prompt 3 rewords them. Do NOT edit the doc here.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, Counterfeiter mocks.

**Environment fact that shapes this prompt: `git` is UNAVAILABLE in this container.** `/workspace/.git` is a character device, so every git command fails with `fatal: not a git repository`, and the daemon does not check `<verification>` exit codes — a git command here reports a false pass. Do NOT run git anywhere in this prompt. This prompt issues no git commands; the AC10 `scenarios/005` guard was verified in prompt 1 as a sha256 content pin.
</context>

<requirements>
<!-- ⚠️ HUMAN REVIEWER — READ BEFORE APPROVING: THIS PROMPT CONTRADICTS A SHIPPED, LIVE-REPRODUCED FIX ⚠️

The task-side half of spec 041 was REVERTED in the tree AFTER the spec was approved:

- Commit 247a789 (released v0.117.1) applied spec 041 to workon.go: persist claude_session_id only AFTER the turn, no compensating clear.
- Commit dae6563 (released v0.118.3), "fix(workon): persist the fresh session id before the headless turn", REVERTED that task-side half. Its stated rationale: with the field left empty for the whole turn, the child's own `/vault-cli:work-on-task` session-connect did not find an id, fell back to scanning the transcript directory by mtime, and wrote a LIVE UNRELATED session's id into the field. Reproduced live 2026-09-01 on a probe task: it spawned session 892ab117 and ended up bound to 34d27423, a live session on a different task. The post-turn persist then no-op'd because the field was no longer empty. The reversion re-adopted spec 040's persist-before-spawn plus a re-read-based compensating clear, on the task path only.
- The reversion did NOT touch claude_session.go's block-until-exit + validation (spec 041 prompt 1), the spec 042 per-session lock, goal_workon.go (which still persists post-exit), or topic_workon.go (spec 052, which was written post-exit from the start).

The current tree therefore FAILS spec 041 AC7 (`grep -c 'After(childExitAt)' pkg/ops/workon_test.go` == 0) and AC9 (`grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 3).

WHY THIS IS NOT MERELY A STYLE REVERSAL — the exact regression mechanism: `persistSessionAndMetrics` only writes the id when the refreshed task's field is EMPTY (`if refreshed.ClaudeSessionID() == ""`), then returns the MINTED id unconditionally. Under post-exit ordering, if the child's session-connect already wrote a foreign id during the turn, the guard skips the write, the function still returns the minted id, and `Execute` reports that id to the Vault UI — which then offers Resume for the FOREIGN session. That is dae6563's bug, unchanged.

The spec's Constraints section requires this reorder ("No compensating clear ... delete them"), and AC7/AC9 pin it, so this prompt implements the spec AS APPROVED. It does so on the assumption that the reviewer has weighed the conflict, because nothing in this spec supplies the missing mechanism (a non-frontmatter source of truth for the child's session-connect, or vault-ui writing the id earlier).

The reviewer must decide at audit time:
  (A) Approve — spec 041 wins; the reorder is re-applied and the session-connect regression is accepted and owned as a follow-up (e.g. a new spec that makes the child read the id from somewhere other than the task frontmatter).
  (B) Reject and re-scope spec 041 — treat dae6563 as the target. Then AC7/AC9 must be reworded to assert pre-spawn persist + compensating clear (which the current tree already satisfies), and prompt 3 must be rejected too, because its doc reword and its CHANGELOG bullet both describe the post-exit ordering.

For the executing agent: implement the requirements below as written. The approved spec is the source of truth for this batch; the reviewer adjudicates the conflict at audit time. Do NOT "fix" the requirements to preserve the reversion, and do NOT add a compensating clear or any other new mechanism that the spec does not name. -->

1. **Fail-loud gate — prompt 1 must have shipped.** Before changing anything, confirm prompt 1's deliverables exist:
   ```
   grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                                  # must be >= 1
   grep -c 'validateSessionTurn' pkg/ops/claude_session.go                                 # must be >= 2
   grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go   # must be >= 1
   ```
   If ANY of these is 0, STOP and report `"status":"failed"` with message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed"`. Do NOT proceed and do NOT re-implement prompt 1's work.

2. **Reorder the fresh-start path of `workon.go`'s `handleClaudeSession` to start-then-persist.** Replace the block that begins at the `// Persist id + metrics BEFORE the child exists, on both branches, ...` comment and ends at the function's closing `return sessionID, nil`, with exactly:
   ```go
   	// Captured BEFORE the spawn — the turn's true start, not the write time.
   	startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
   	if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
   		// No compensating clear needed: nothing was written for this id, so there is
   		// nothing to undo. Frontmatter the child wrote before failing stays untouched.
   		return "", errors.Wrap(ctx, err, "start claude session")
   	}
   	sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
   	return sessionID, err
   ```
   Keep the preceding lines (`prompt := fmt.Sprintf(...)`, `sessionID := w.uuidGenerator()`, `slog.Info("starting claude session", ...)`) and the bootstrap comment above `prompt` exactly as they are. DELETE: the `// Persist id + metrics BEFORE the child exists ...` comment, the pre-spawn `persistSessionAndMetrics` call and its `"persist claude session before spawn"` wrap, and the whole `if clearErr := w.clearSessionAndMetrics(...)` compensating-clear block.

   Signature note: `handleClaudeSession` returns `(string, error)` — two values. The spec's Design snippet shows `return "", nil, errors.Wrap(...)` (three values); that is a spec typo and does NOT compile here. Use the two-value form above, which is exactly `goal_workon.go`'s and `topic_workon.go`'s pattern. The `sessionID, err := ...` line compiles because `sessionID` is already declared in the scope and `err` is newly introduced by it.

   The cached-session path at the top of `handleClaudeSession` (the `if existing := task.ClaudeSessionID(); existing != ""` branch) must be UNCHANGED. Note that `persistSessionAndMetrics` is called from two places in this file — the cached path (keep) and the fresh path (this reorder). It has no other callers anywhere in the repo.

3. **Delete the dead `clearSessionAndMetrics` method from `workon.go`.** Remove the whole method — its doc comment and body. Its only call site was the block deleted in requirement 2. Do not add a replacement, and do not add any other clearing mechanism (the spec's Non-goals forbid it). After this, `grep -rn 'clearSessionAndMetrics' pkg/` must return nothing. `ClearClaudeSessionID()` on the domain type stays — `pkg/ops/complete.go` still uses it (and so do the goal/task frontmatter tests).

4. **Reword the `workon.go` doc comments that describe the pre-spawn design.**
   - `persistSessionAndMetrics`'s comment: replace `Used pre-spawn on the fresh-start path (the session id is new and must be on disk before the child exists) and on the cached-session path (the id already exists and is preserved).` with `Used post-exit on the fresh-start path (the session id is new and is persisted only after the headless turn completes cleanly) and on the cached-session path (the id already exists and is preserved).` Leave the surrounding sentences about the load-bearing re-read and the empty-id rule as they are.
   - `handleClaudeSession`'s comment: replace it with the wording `goal_workon.go` / `topic_workon.go` already use for the same method — on both branches the session id is persisted only AFTER the headless turn has finished cleanly, so an id on disk means the session is resumable rather than merely that one was started; nothing is written on any failure path, so there is no compensating clear, and frontmatter the child wrote before failing stays untouched so the Vault UI correctly keeps offering Start. Keep the cached-session sentence.
   - `sessionFailureResult`'s comment says the warnings are `the accumulated warnings (including any compensating-clear warning)`. Reword that clause — there is no clear, so the warnings are only the assignee and daily-note ones. Do not change the function's behaviour.

5. **Confirm `goal_workon.go` and `topic_workon.go` are already in the target state — do NOT change them.**
   - `goal_workon.go`: its `handleClaudeSession` must already start the turn first and persist after on both branches, return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear, and keep the cached path as `return existing, nil`. `persistGoalSessionID` must already return an empty id on failure. `clearGoalSession` must not exist (`grep -c 'clearGoalSession' pkg/ops/goal_workon.go` == 0).
   - `topic_workon.go`: same shape via `persistTopicSessionID`; no compensating clear exists.
   If both match, leave both files untouched.

6. **Rework `"when persisting the session id before spawning"` in `workon_test.go` to post-exit (AC7).** Rename the context to `"when persisting the session id after the child exits"`. In the variable block rename `spawnAt time.Time` to `childExitAt time.Time`, and in the fake `detachRun` rename the `spawnAt = time.Now()` assignment to `childExitAt = time.Now()` — keep it in the same position (immediately before the buffered `done` channel is fed, which is the child's exit point). The `BeforeEach` already captures `spawnedSessionID` from the `--session-id` argv, writes valid turn JSON to the `stdout *os.File`, and uses a blocking waiter; keep all of that unchanged. Rename the `It` to `"writes the session id to storage only after the child exits"` and replace its body with exactly:
   ```go
   Expect(err).To(BeNil())
   Expect(writeTaskAt).NotTo(BeZero())
   Expect(childExitAt).NotTo(BeZero())
   Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
   // AC5's "id equals the value in task frontmatter" — capture the id written to
   // storage and the id handed to detachRun and assert they are the same value.
   // Both derive from the pinned generator today, so this holds implicitly; assert
   // it explicitly so a future refactor that mints a second id cannot pass silently.
   Expect(writtenSessionID).To(Equal(spawnedSessionID))
   ```
   The old assertion `Expect(writeTaskAt.Before(spawnAt)).To(BeTrue())` must be gone. This produces AC7's `After(childExitAt)` evidence.

7. **Invert `"when the session id write precedes the spawn"` in `workon_test.go`.** Rename the context to `"when the session id write follows the spawn"`, rename the `It` to `"writes the session id to storage after StartSession returns"`, and flip the final assertion from `Expect(writeSeq).To(BeNumerically("<", startSeq))` to `Expect(writeSeq).To(BeNumerically(">", startSeq))`. Leave the `WriteTaskStub` / `StartSessionStub` sequencing setup exactly as it is. The ordering is still deterministic: `Execute` writes first (empty id, so `writeSeq` is not set), then `StartSession` runs (`startSeq`), then the post-exit persist writes the id (`writeSeq`).

8. **Replace the clear-based failure tests in `"when the spawn fails"` with a persists-nothing assertion (AC9).** In that context:
   - Keep `"returns the wrapped spawn error"` and `"returns Success=false"` unchanged.
   - DELETE the spec `"clears the pre-persisted session id and the metrics entry for the failed run"` and the whole nested `Context("when the compensating clear itself fails", ...)`.
   - ADD one spec:
     ```go
     It("persists no session id when the spawn fails", func() {
         // Execute's write only — nothing is pre-persisted and there is no
         // compensating clear, so a failed turn leaves no id and no metrics entry.
         Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
         Expect(writtenIDs).To(Equal([]string{""}))
         Expect(writtenMetricsSessions[0]).To(BeEmpty())
     })
     ```
   - Reword the `BeforeEach` comment that explains the compensating-clear pointer mutation so it reads as a snapshot of what each write lands: with no pre-persist and no clear there is exactly one write, and it carries an empty id because the id is minted inside `handleClaudeSession` after `Execute` has already written the task. Keep the stub itself — it is what makes `writtenIDs` observable.

9. **Rework `"when the pre-spawn persist re-read fails"` in `workon_test.go` to post-exit.** Rename the context to `"when the post-exit persist re-read fails"` and the `It` `"does not append a metrics entry when the pre-spawn re-read fails"` to `"does not append a metrics entry when the post-exit re-read fails"`. Reword the `BeforeEach` comment from `The failing call is persistSessionAndMetrics' PRE-SPAWN re-read, which runs before StartSession is ever called.` to `The failing call is persistSessionAndMetrics' POST-EXIT re-read, which runs after StartSession returns (the mock starter returns nil immediately).`, and the comment on the write-count `It` from `pre-spawn persist failed before writing` to `post-exit persist failed before writing`. The mock call indexes do NOT change: call 0 loads the task, `mockStarter.StartSessionReturns(nil)` makes no storage call, call 1 is the post-exit re-read that fails. All four `It` bodies stay byte-identical.

10. **Reword the remaining pre-spawn wording in `workon_test.go`'s `"success"` context.** Assertions and counts are unchanged; only comments and two titles change:
    - `"calls FindTaskByName"` comment: `Twice: once to load the task, once to re-read it before the child is spawned so the fresh session id lands on disk before the child exists.` → `Twice: once to load the task, once to re-read it after the child exits so the post-exit persist lands the fresh session id without reverting the child's frontmatter writes.`
    - `"re-reads the task from the vault path before spawning the session"`: rename to `"re-reads the task from the vault path after the child exits"` and reword its comment to `The second FindTaskByName is persistSessionAndMetrics' post-exit re-read: the session id is written to disk only after the child has exited.`
    - `"Fresh run records one entry"` comment: `The metrics entry lands in the pre-spawn persist write: write 0 is Execute's status/assignee/phase write, write 1 is the pre-spawn persist.` → `The metrics entry lands in the post-exit persist write: write 0 is Execute's status/assignee/phase write, write 1 is the post-exit persist.`
    The `Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))` assertion stays as it is — Execute's write plus the post-exit persist is still two.

11. **Reword `pkg/ops/workon_session_writeback_test.go` to post-exit semantics — comments and titles ONLY (AC8).** The fakes already write a valid turn-JSON line to the `stdout *os.File` and exit cleanly via `done <- nil` with a blocking waiter; confirm that and do NOT change it. Every on-disk assertion in this file holds unchanged under the new ordering — the ids simply never land — so DO NOT touch any assertion. The full list of prose to change (this is every stale occurrence in the file; the drift-guard grep in `<verification>` requires all of them):
    - The task context's `BeforeEach` comment (currently `Simulate the real headless turn: work-on persists the fresh session id and its metrics entry to the file BEFORE spawning (pre-spawn persist), then ... Nothing writes to the file after the child's own write, so the child's frontmatter survives.`) → `Simulate the real headless turn: work-on spawns the child first, then the Claude session runs plan-task -> execute-task and writes its own frontmatter on top of that file inside the detached child (the detachRun fake). Only after the child exits does work-on re-read and persist the session id, so the child's frontmatter survives.`
    - The task `It`'s comment `The metrics entry lands in the pre-spawn persist write (real storage round-trip) and survives because nothing writes to the file after the child's own write.` → `The metrics entry lands in the post-exit persist write (real storage round-trip), which re-reads the file the child already wrote.`
    - The goal context's `BeforeEach` comment clause `while the parent has already returned within the liveness window` → `while the parent blocks waiting for the detached turn`.
    - Rename `Context("when the child exits non-zero inside the liveness window", ...)` → `Context("when the child exits non-zero within the turn wait", ...)`.
    - Reword the comment `The liveness window has NOT elapsed when the child exits, so the starter must treat the exit as inside-the-window. A nil-returning waiter would race the select against the child's buffered exit; ...` → `The turn wait has NOT elapsed when the child exits, so the child-exit branch of the select wins. A nil-returning waiter would race the select against the child's buffered exit; ...` (keep the rest of the sentence about the blocking waiter).
    - Rename the `It` `"clears the pre-persisted session id and preserves the child's frontmatter write when the child exited non-zero inside the window"` → `"persists no session id and preserves the child's frontmatter write when the child exited non-zero within the turn wait"`.
    - Reword the body comment (the one describing the pre-spawn persist + compensating clear re-read) so it describes the post-exit, no-clear ordering: the turn failed, so nothing was ever written for this id, and the child's own `phase: planning` write is what survives.
    - Reword the `On-disk shape:` comment (which currently says the pre-spawn persist wrote the id and the compensating clear removed it) to say no id was ever persisted on this path, so its absence from the raw file proves the invariant directly. Keep the explanatory note about the pinned accessor-call counts and the raw-file assertion — that reasoning is unchanged.
    - Reword the comment above `Context("when the child exits non-zero after writing a valid turn result", ...)`: the pre-persisted id is no longer the mechanism — the post-exit persist runs because the validated result outranks the non-zero exit.
    - Rename the `It` `"retains the pre-persisted session id when the turn result validated despite the non-zero exit"` → `"persists the session id when the turn result validated despite the non-zero exit"`.
    - Reword that `It`'s comment so the retain is attributed to the post-exit persist (the validated result makes `StartSession` return nil, so the persist runs) rather than to a pre-spawn write surviving a clear.
    - Reword the `BeforeEach` comment clause `but the blob reports the turn's own failure, so the compensating clear still fires` → `but the blob reports the turn's own failure, so the turn is rejected and nothing is persisted`.
    - Rename the `It` `"clears the pre-persisted session id when the turn result reports its own failure"` → `"persists no session id when the turn result reports its own failure"`.
    - Reword the trailing comment (the one about the compensating clear removing the id) to say the turn failed so nothing was ever persisted, while the child's `phase: planning` write survives.
    - Do NOT touch the pinned-count strings anywhere in this file: `TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()`. The AC8 greps must keep returning their exact counts, so your reword must not add or remove any occurrence of those tokens.

12. **Confirm the goal AC7 test and the goal assertions are already correct — do not touch them.** `goal_workon_test.go`'s `"when persisting the goal session id after the child exits"` already asserts `writeGoalAt.After(childExitAt)` with both non-zero, and its `"goal work-on persists nothing for a failed turn"` context already proves the no-persist half. Leave the file unmodified.

13. **Confirm the spec's Failure Modes rows that this prompt owns.**
    - Row 6 (claude binary missing): the `ErrStarterUnavailable` soft path is unchanged — `handleClaudeSession` returns `"", ErrStarterUnavailable` when `w.starter == nil` and `Execute` downgrades it to a warning. Confirm `workon.go`, `goal_workon.go` and `topic_workon.go` all still reference it.
    - Row 7 (post-exit persist fails): covered by requirement 9 — `persistSessionAndMetrics` returns an error AND an empty id when the re-read or the write fails, so the task keeps whatever the turn wrote and no id lands.
    - Row 9 (two Start clicks): confirm NO double-start guard was added. It is a documented residual risk and a spec Non-goal. The spec 042 per-session lock already exists and is out of this prompt's scope.

14. **Self-check before finishing.** Re-read every changed hunk and walk spec 041 ACs 7, 8 and 9 against them, naming which artifact satisfies each. Run every command in `<verification>` and confirm each holds, including the two flips: `clearSessionAndMetrics` in `workon.go` 3 → 0, and `After(childExitAt)` in `workon_test.go` 0 → ≥ 1.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Do NOT run any `git` command: `.git` is a character device in this container, so every git command fails with `fatal: not a git repository` and the daemon does not check `<verification>` exit codes — a git command here reports a false pass.
- Interactive branch behaviour unchanged. `defaultCommandRunner`, the 5-minute TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched.
- No compensating clear: the id is never pre-written, so `clearSessionAndMetrics` must stay deleted and no replacement may be added. On failure the task simply carries no id. Do not add a double-Start guard and do not add any config knob (both are spec Non-goals / Open Question 1).
- Persist-after-exit is race-free within the process: the child has already exited before the post-exit persist runs, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- `ClaudeSessionStarter.StartSession`'s signature is UNCHANGED — six parameters, `mocks/claude-session-starter.go` untouched. `handleClaudeSession`'s `(string, error)` signature is UNCHANGED; the spec's three-value `return "", nil, errors.Wrap(...)` snippet is a typo and must NOT be used.
- The pinned-count tokens in `workon_session_writeback_test.go` (`TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()`) must remain byte-identical in count — requirement 11's reword is prose-only and must not touch an assertion.
- `goal_workon.go`, `goal_workon_test.go` and `topic_workon.go` are already in the target state — do not modify them except to read and confirm.
- Do NOT touch `pkg/ops/claude_session.go`, `pkg/ops/claude_session_test.go`, `pkg/ops/claude_session_detach_test.go`, or `pkg/ops/export_test.go` (prompt 1 owns them), and do NOT touch `docs/work-on-session-lifecycle.md`, `scenarios/002-task-lifecycle.md`, or `CHANGELOG.md` (prompt 3 owns them).
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'clearSessionAndMetrics' pkg/ops/workon.go                      # == 0 (AC9) — was 3, must flip to 0
grep -c 'clearGoalSession' pkg/ops/goal_workon.go                       # == 0 (AC9)
grep -c 'After(childExitAt)' pkg/ops/workon_test.go                     # >= 1 (AC7) — was 0, must flip to >= 1
grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go                # >= 1 (AC7)
grep -c 'writtenSessionID' pkg/ops/workon_test.go                       # >= 1 (AC7)
grep -c 'spawnedSessionID' pkg/ops/workon_test.go                       # >= 1 (AC7)
grep -c 'ErrStarterUnavailable' pkg/ops/workon.go                       # >= 1 (Failure Modes row 6)
grep -c 'ErrStarterUnavailable' pkg/ops/goal_workon.go                  # >= 1 (Failure Modes row 6)
grep -c 'ErrStarterUnavailable' pkg/ops/topic_workon.go                 # >= 1 (Failure Modes row 6, third sibling)
! grep -q 'persist claude session before spawn' pkg/ops/workon.go       # reverted vocabulary gone
! grep -qE 'pre-spawn|pre-persisted|pre-persist|compensating' pkg/ops/workon.go
! grep -qE 'pre-spawn|pre-persisted|pre-persist|compensating' pkg/ops/workon_test.go
! grep -qE 'pre-spawn|pre-persisted|pre-persist|compensating|liveness' pkg/ops/workon_session_writeback_test.go
```

AC8 writeback invariant counts — must hold exactly; this reword is prose-only and must not change them:

```
grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go   # == 2
grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go   # == 2
grep -c 'session_note' pkg/ops/workon_session_writeback_test.go         # == 4
grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go    # == 2
grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go    # == 2
```

AC9 — the failure path asserts nothing was persisted:

```
grep -c 'persists no session id when the spawn fails' pkg/ops/workon_test.go   # >= 1
```

SYNTAX + TESTS:
```
gofmt -e -l pkg/ops/workon.go pkg/ops/workon_test.go pkg/ops/workon_session_writeback_test.go   # must list NO files
make test                                                                                       # must exit 0
```

FULL GATE — `make precommit` at the repo root must exit 0. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make gosec`, `make errcheck`, ...), then `make precommit` once more. This is the batch's authoritative full-gate check (AC13), which prompt 3 runs last; running it here as well keeps the tree green for prompt 3, which builds on it.
</verification>
