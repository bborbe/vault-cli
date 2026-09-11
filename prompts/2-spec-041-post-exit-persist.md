---
spec: [041-bug-resume-races-live-headless-turn]
status: draft
created: "2026-09-11T12:05:00Z"
---

<summary>
- Re-applies the spec-041 start→persist reorder to `workon.go`'s fresh-start path: the headless turn is started FIRST (via `w.starter.StartSession`) and `claude_session_id` + the `metrics_sessions` entry are persisted only AFTER it exits cleanly — structurally identical to `goal_workon.go`'s non-interactive branch (which is already at the target and is not touched).
- Deletes the now-dead `clearSessionAndMetrics` function from `workon.go` and its compensating-clear call site (AC9: the `clearSessionAndMetrics` grep in workon.go flips 3 → 0). On any failure the task simply carries no id.
- Reworks `workon_test.go`: the "when persisting the session id before spawning" context becomes "after the child exits" asserting `writeTaskAt.After(childExitAt)` (AC7, flips 0 → >= 1), the "session id write precedes the spawn" sequencing test inverts (`writeSeq < startSeq` → `> startSeq`), the clear-based failure tests are deleted and replaced with a "persists no session id when the spawn fails" assertion, the "pre-spawn persist re-read fails" context becomes "post-exit", and the comment-era specs are reworded to post-exit terminology.
- Rewords the pre-spawn / compensating-clear comments and titles in `workon_session_writeback_test.go` to post-exit no-clear semantics. The fakes ALREADY write a valid JSON line to the stdout `*os.File` and exit cleanly via `done <- nil` with a blocking waiter, so the AC8 invariant assertions (phase `execution`, `session_note`, `ClaudeSessionID() == pinnedSessionID`, `MetricsSessions()` len 1) stay byte-identical and the pinned-count greps hold exactly (2/2/4/2/2).
- Confirms `goal_workon.go` and `goal_workon_test.go` are already at the spec-041 target (post-exit persist, no compensating clear) and are not modified.
- Runs `make test` and the spec-041 AC7-9 grep gate.
</summary>

<objective>
Make the task path of `work-on` persist `claude_session_id` only after the detached headless turn has finished and validated, so no failure leaves a resumable-looking id behind — matching the goal path, which already ships this behavior. This prompt covers spec-041 ACs 7-9 and depends on prompt 1 having shipped (its `StartSession` block-until-exit + validation behavior is what these tests exercise).
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file; focus on `handleClaudeSession` (starts at line 281), `persistSessionAndMetrics` (line 221), `clearSessionAndMetrics` (line 250), and the fresh-start block (lines 297-322).
- `pkg/ops/goal_workon.go` — the whole file; this is the structural TEMPLATE the reordered task path must match (`handleClaudeSession` at line 200, non-interactive branch at lines 229-240).
- `pkg/ops/workon_test.go` — the whole file; the AC7 context "when persisting the session id before spawning" (line 906), the sequencing context "when the session id write precedes the spawn" (line 160), the failure context "when the spawn fails" (line 982), "when the pre-spawn persist re-read fails" (line 877), and the comment-era specs at lines 98-157.
- `pkg/ops/goal_workon_test.go` — the whole file; the AC7 context "when persisting the goal session id after the child exits" starts at line 356 (already spec-041 — confirm, do not touch).
- `pkg/ops/workon_session_writeback_test.go` — the whole file; the AC8 writeback invariants and the pinned-count strings.
- `pkg/ops/claude_session.go` — read the `runDetachedTurn` method (lines 226-310) so you understand exactly when `StartSession` returns vs errors; the writeback fakes must satisfy it (a valid JSON blob on stdout + a `done <- nil` for a clean exit, an empty stdout for the unparseable exit-status path).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

NOTE: git IS available in this container (`workflow: direct`, no hideGit) — but this prompt has no git commands; AC10's `scenarios/005` guard is verified in prompt 1.
</context>

<requirements>
1. **Guard — prompt 1 must have shipped.** Before doing anything, confirm prompt 1's deliverables exist: `grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go` >= 1 AND `grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go` >= 1. If EITHER is 0, STOP — make no changes — and complete with `"status":"failed"` and message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed (session-turn-block)"`. Do not proceed.

2. **Reorder the fresh-start path in `workon.go`'s `handleClaudeSession` to start→persist.** The current fresh-start block runs from the `prompt := fmt.Sprintf(...)` line (line 300) through `return sessionID, nil` (line 321) and persists BEFORE the spawn, then compensates on failure. Replace that whole block with this exact code (structurally identical to `goal_workon.go`'s non-interactive branch at lines 229-240, and to the interactive branch — same checks, same error strings):
   ```go
   	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
   	sessionID := w.uuidGenerator()
   	slog.Info("starting claude session", "task", task.Name)
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
   The lines being deleted entirely: the `// Persist id + metrics BEFORE the child exists...` comment block (lines 303-307), the `if _, err := persistSessionAndMetrics(...); err != nil { return "", errors.Wrap(ctx, err, "persist claude session before spawn") }` call (lines 309-311), and the `if err := w.starter.StartSession(...); err != nil { ... clearSessionAndMetrics ... }` compensating-clear block (lines 312-320). The cached-session path (the `if existing := task.ClaudeSessionID(); existing != ""` branch, lines 289-293) must be UNCHANGED. Note the function returns `(string, error)` — the new `return "", errors.Wrap(...)` is 2 values; `sessionID, err := ...` compiles because `sessionID` is already declared above and `err` is newly introduced in that scope (this is exactly `goal_workon.go` line 238's pattern). Do NOT use the spec Design's `return "", nil, errors.Wrap(...)` snippet — that 3-value form is a spec typo and does not compile against the real signature.

3. **Delete the dead `clearSessionAndMetrics` function from `workon.go`.** Remove the entire function including its doc comment (currently lines 246-272). After this, `grep -rn 'clearSessionAndMetrics' pkg/` must return NOTHING (AC9: currently 3 occurrences in workon.go — doc comment line 246, definition line 250, call site line 316 — must become 0). Its only call site was the block deleted in requirement 2. Do not add any replacement.

4. **Reword the doc comments in `workon.go` that describe the pre-spawn design.**
   - `persistSessionAndMetrics`'s comment (lines 210-220): replace the sentence `Used pre-spawn on the fresh-start path (the session id is new and must be on disk before the child exists) and on the cached-session path (the id already exists and is preserved).` with `Used post-exit on the fresh-start path (the session id is new and is persisted only after the headless turn completes cleanly) and on the cached-session path (the id already exists and is preserved).` Keep the rest of the comment (the load-bearing re-read sentence) intact.
   - `handleClaudeSession`'s comment (lines 274-280): replace the sentences about pre-spawn persistence + compensating clear with text mirroring `goal_workon.go`'s (lines 194-199): `On both branches the session id is persisted only AFTER the headless turn has finished cleanly, so an id on disk means the session is resumable rather than merely that one was started. Nothing is written on any failure path, so there is no compensating clear: frontmatter the child wrote before failing stays untouched, and the Vault UI correctly keeps offering Start.` Keep the cached-session sentence.

5. **Confirm `goal_workon.go` is already in the target state — do NOT change it.** `handleClaudeSession` must already persist AFTER `StartSession` on both branches, return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear, and keep the cached path as `return existing, nil` (line 209). `persistGoalSessionID` must already return an empty id on failure. If it matches, leave it untouched (AC9's `clearGoalSession` grep already passes — `clearGoalSession` does not exist anywhere in the repo).

6. **Rework AC7 in `workon_test.go` — "when persisting the session id before spawning" (lines 906-980).** Rename the context to `"when persisting the session id after the child exits"`. In the variable block (line 908), rename `writeTaskAt, spawnAt time.Time` to `writeTaskAt, childExitAt time.Time`. In the `detachRun` fake, rename the `spawnAt = time.Now()` line (line 945) to `childExitAt = time.Now()` (it already sits immediately before `done <- nil`). The BeforeEach already captures `spawnedSessionID` from the `--session-id` argv, writes valid turn JSON to the stdout `*os.File`, and uses a blocking waiter — keep all of that. Rename the `It` (line 969) from `"writes the session id to storage before the runner spawns the child"` to `"writes the session id to storage only after the child exits"` and replace the assertion block (lines 970-978) with exactly:
   ```go
   	Expect(err).To(BeNil())
   	Expect(writeTaskAt).NotTo(BeZero())
   	Expect(childExitAt).NotTo(BeZero())
   	Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
   	Expect(writtenSessionID).To(Equal(spawnedSessionID))
   ```
   Keep the existing comment about AC5's "id equals the value in task frontmatter" (lines 974-977). This yields the AC7 evidence `After(childExitAt)` (currently absent — the old assertion is `Expect(writeTaskAt.Before(spawnAt)).To(BeTrue())` at line 973).

7. **Invert "when the session id write precedes the spawn" in `workon_test.go` (lines 160-185).** Rename the context to `"when the session id write follows the spawn"`, rename the `It` (line 179) to `"writes the session id to storage after StartSession is called"`, and change the final assertion (line 183) from `Expect(writeSeq).To(BeNumerically("<", startSeq))` to `Expect(writeSeq).To(BeNumerically(">", startSeq))`. The `WriteTaskStub`/`StartSessionStub` sequencing setup stays as-is.

8. **Delete the clear-based failure tests in `workon_test.go` and add a persists-nothing assertion (AC9).** In the `"when the spawn fails"` context (lines 982-1041):
   - Keep `"returns the wrapped spawn error"` (line 1005) and `"returns Success=false"` (line 1010) unchanged.
   - DELETE `"clears the pre-persisted session id and the metrics entry for the failed run"` (lines 1014-1023) and the nested `Context("when the compensating clear itself fails", ...)` (lines 1025-1040) entirely — the clear no longer exists.
   - ADD one spec to the context (after `"returns Success=false"`):
     ```go
     It("persists no session id when the spawn fails", func() {
         // Execute's write only — nothing is pre-persisted and there is no
         // compensating clear, so a failed turn leaves no id or metrics entry.
         Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
         Expect(writtenIDs).To(Equal([]string{""}))
         Expect(writtenMetricsSessions[0]).To(BeEmpty())
     })
     ```
   - Reword the BeforeEach comment (lines 989-992) that currently explains the compensating-clear pointer mutation: it now reads as a snapshot of what each write lands, with the note that on the failure path the only write is Execute's own (the id is minted inside `handleClaudeSession` after `Execute` writes the task), so it carries an empty id and no metrics entry.

9. **Rework "when the pre-spawn persist re-read fails" in `workon_test.go` (lines 877-904) to post-exit.** Rename the context to `"when the post-exit persist re-read fails"` and the `It` (line 900) `"does not append a metrics entry when the pre-spawn re-read fails"` to `"does not append a metrics entry when the post-exit re-read fails"`. Reword the BeforeEach comment (lines 880-881) from `The failing call is persistSessionAndMetrics' PRE-SPAWN re-read, which runs before StartSession is ever called.` to `The failing call is persistSessionAndMetrics' POST-EXIT re-read, which runs after StartSession returns (the mock starter returns nil immediately).` and the comment on the write-count `It` (line 896) from `Execute's write only — the pre-spawn persist failed before writing.` to `Execute's write only — the post-exit persist failed before writing.` The mock call indexes do NOT change: call 0 loads the task, `mockStarter.StartSessionReturns(nil)` makes no storage call, call 1 is the post-exit re-read that fails. All four `It` bodies stay byte-identical.

10. **Reword the comment-era specs in `workon_test.go` (lines 98-157) to post-exit terminology.** These all drive the mock starter and their assertions/counts are unchanged (`FindTaskByNameCallCount()` == 2, `WriteTaskCallCount()` == 2); only comments and one title change:
    - `"calls FindTaskByName"` comment (lines 99-100): `Twice: once to load the task, once to re-read it before the child is spawned so the fresh session id lands on disk before the child exists.` → `Twice: once to load the task, once to re-read it after the child exits so the post-exit persist lands the fresh session id without reverting the child's frontmatter writes.`
    - `"re-reads the task from the vault path before spawning the session"` (line 110): rename title to `"re-reads the task from the vault path after the child exits"` and its comment (lines 112-113) to `The second FindTaskByName is persistSessionAndMetrics' post-exit re-read: the session id is written to disk only after the child has exited.`
    - `"Fresh run records one entry"` comment (lines 149-150): `The metrics entry lands in the pre-spawn persist write: write 0 is Execute's status/assignee/phase write, write 1 is the pre-spawn persist.` → `The metrics entry lands in the post-exit persist write: write 0 is Execute's status/assignee/phase write, write 1 is the post-exit persist.`

11. **Reword `pkg/ops/workon_session_writeback_test.go` to post-exit semantics; confirm the fakes (AC8); keep the pinned counts byte-identical.** The fakes ALREADY write a valid JSON line to the stdout `*os.File` (`{"session_id":"<pinnedSessionID>","num_turns":3,"is_error":false,"result":"done"}` in the task and goal success contexts and the valid-result-retain context) and exit via `done <- nil` (success paths) or `done <- errors.New("exit status 1")` (failure paths) with a blocking waiter — confirm this, do NOT change the fakes' behavior. Reword only comments and titles, and change NOTHING about the assertions:
    - The task-context BeforeEach comment (lines 127-132) from the pre-spawn description to: `Simulate the real headless turn: work-on spawns the child first, then the Claude session runs plan-task -> execute-task and writes its own frontmatter on top of that file inside the detached child (the detachRun fake). Only after the child exits does work-on re-read and persist the session id, so the child's frontmatter survives.`
    - The task-context success-`It` comment (lines 187-189) `The metrics entry lands in the pre-spawn persist write (real storage round-trip) and survives because nothing writes to the file after the child's own write.` → `The metrics entry lands in the post-exit persist write (real storage round-trip) and survives because it is written after the child's own write.`
    - The goal-context BeforeEach comment's `...while the parent has already returned within the liveness window.` (lines 215-216) → `...while the parent blocks waiting for the detached turn.`
    - Rename the context `"when the child exits non-zero inside the liveness window"` (line 270) to `"when the child exits non-zero within the turn wait"` and the `It` title (line 318) from `clears the pre-persisted session id and preserves the child's frontmatter write when the child exited non-zero inside the window` to `persists no session id and preserves the child's frontmatter write when the child exited non-zero within the turn wait`. Reword the mechanism comments (lines 286-290, 342-347, 353-359) so they describe the post-exit no-clear ordering: the child's exit error makes `StartSession` return `"claude session exited with error"` (the stdout file is left empty by the fake, so the unparseable fallback names the exit status), `handleClaudeSession` persists nothing, and the child's `phase: planning` write survives on disk. Do NOT touch the assertions (`raw` has no `claude_session_id:`, no `pinnedSessionID`, has `phase: planning`; error contains `"exit status 1"` and `"start work-on session"`) — they are byte-identical under the new ordering.
    - The valid-result-retain context (line 368, spec-045 "result outranks exit code"): reword the context comment (lines 369-371) and the `It` title (line 412) from `retains the pre-persisted session id when the turn result validated despite the non-zero exit` to `persists the session id when the turn result validated despite the non-zero exit`, and reword the mechanism comment (lines 424-427) from the pre-spawn-retain story to: `The validated result is authoritative over the child's non-zero exit (spec 045), so StartSession returns nil and handleClaudeSession proceeds to persist the id post-exit. The phase change the child wrote survives because the persist re-reads first.` Do NOT touch the assertions (`claude_session_id:` count == 1, `pinnedSessionID` present, `phase: planning` present).
    - The own-failure context (line 447, spec-045): reword the context comment (lines 463-465) and the `It` title (line 487) from `clears the pre-persisted session id when the turn result reports its own failure` to `persists no session id when the turn result reports its own failure`, and reword the final mechanism comment (lines 514-515) from the compensating-clear story to: `The turn result failed its own predicate (is_error/0 turns), so StartSession returned an error and nothing was persisted; the child's phase: planning write survived.` Do NOT touch the assertions (`claude_session_id:` count == 0, no `pinnedSessionID`, `phase: planning` present, error leads with `seeded failure text`).
    - Do NOT touch the pinned-count strings anywhere in this file: `TaskPhaseExecution` (==2), `GoalPhaseExecution` (==2), `session_note` (==4), `MetricsSessions()` (==2), `ClaudeSessionID()` (==2). The AC8 greps must stay byte-identical.

12. **Confirm the goal AC7 test and the AC8 assertions are already correct — do not touch them.** `goal_workon_test.go` "when persisting the goal session id after the child exits" (lines 356-407) already asserts `writeGoalAt.After(childExitAt)` with both non-zero (line 406). `workon_session_writeback_test.go`'s task and goal success `It`s already assert the child's phase + `session_note` survive, `ClaudeSessionID() == pinnedSessionID`, and `MetricsSessions()` length 1. Confirm and leave unchanged.

13. **Confirm failure-mode rows 6, 7, 9 from the spec table.**
    - Row 6 (claude binary missing): `ErrStarterUnavailable` soft path unchanged — `workon.go` `handleClaudeSession` returns `"", ErrStarterUnavailable` when `w.starter == nil`, surfaced as a warning by `Execute`. Confirm `grep -c 'ErrStarterUnavailable' pkg/ops/workon.go` >= 1 and `grep -c 'ErrStarterUnavailable' pkg/ops/goal_workon.go` >= 1.
    - Row 7 (post-exit persist fails): covered by requirement 9's reworked context — `persistSessionAndMetrics` / `persistGoalSessionID` return an error AND an empty id when the re-read or write fails; the task keeps whatever the turn wrote; no id lands.
    - Row 9 (two Start clicks): confirm NO double-start guard was added by this spec (documented residual risk, Non-goals). The spec-042 per-session lock is already landed and is out of this prompt's scope. Do not add a guard.

14. **Self-check.** Re-read the changed hunks and walk ACs 7-9 against them. Run the `<verification>` block and confirm every expected-to-pass grep holds, including the flips: `clearSessionAndMetrics` in workon.go 3 → 0, `After(childExitAt)` in workon_test.go 0 → >= 1.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Interactive branch behavior unchanged. `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched.
- No compensating clear: the id is never pre-written, so `clearSessionAndMetrics` must stay deleted. On failure the task simply carries no id.
- Persist-after-exit is race-free: the child has already exited before the post-exit persist, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- The `ClaudeSessionStarter` interface signature is UNCHANGED — `mocks/claude-session-starter.go` is untouched. `handleClaudeSession`'s `(string, error)` signature is UNCHANGED — the spec Design's `return "", nil, errors.Wrap(...)` snippet is a typo and must NOT be used (it does not compile).
- Do NOT add a double-Start guard and do NOT add any config knob (both are spec Non-goals / Open Question 1).
- The AC8 pinned-count strings (`TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()`) in `workon_session_writeback_test.go` must remain byte-identical — requirement 11's comment reword must not touch any assertion.
- `goal_workon.go` and `goal_workon_test.go` are already in the spec-041 target state — do not modify them except to confirm.
- Do NOT touch `docs/work-on-session-lifecycle.md` in this prompt (prompt 3 rewords it) or `pkg/ops/claude_session.go` (prompt 1 owns it).
- Existing tests must still pass.
</constraints>

<verification>
PRIMARY GATE — spec evidence greps. Run each and record the count:

```
grep -c 'clearSessionAndMetrics' pkg/ops/workon.go                     # == 0 (AC9) — currently 3, must flip to 0
grep -c 'clearSessionAndMetrics' -r pkg/                               # == 0 (AC9, whole package)
grep -c 'clearGoalSession' pkg/ops/goal_workon.go                      # == 0 (AC9)
grep -c 'After(childExitAt)' pkg/ops/workon_test.go                    # >= 1 (AC7) — currently 0, must flip to >= 1
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
grep -c 'persists no session id when the spawn fails' pkg/ops/workon_test.go  # >= 1
# Reversion vocabulary must be gone from workon.go:
grep -c 'persist claude session before spawn' pkg/ops/workon.go        # == 0
grep -c 'pre-spawn\|pre-persisted' pkg/ops/workon.go                   # == 0
grep -c 'livenessWindow' pkg/ops/workon.go                             # == 0
```

SECONDARY — syntax validity:
```
gofmt -e -l pkg/ops/workon.go pkg/ops/workon_test.go pkg/ops/workon_session_writeback_test.go   # must list NO files
```

TESTS:
```
make test   # must exit 0
```

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running `make test` + the grep gate here is correct; the daemon's preflight already runs `make precommit` before execution.
</verification>
