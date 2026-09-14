---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-03T10:05:00Z"
---

<summary>
- Re-applies the spec-041 start→persist reorder to `workon.go`'s fresh-start path: the headless turn is started FIRST and `claude_session_id` + the metrics entry are persisted only AFTER it exits cleanly.
- Deletes the now-dead `clearSessionAndMetrics` compensating-clear function from `workon.go` and its doc references (AC9) — the current tree still carries the pre-spawn persist + compensating clear that spec-041's AC9 requires removing.
- Reworks `workon_test.go`: the "persisting the session id before spawning" test becomes "after the child exits" asserting `writeTaskAt.After(childExitAt)` (AC7), the "write precedes the spawn" sequencing test inverts, the clear-based failure tests are deleted and replaced with a persists-nothing assertion, and the "pre-spawn persist re-read fails" context becomes "post-exit".
- Rewords the stale pre-spawn / liveness-window comments and the clear-based child-exit context in `workon_session_writeback_test.go` to the post-exit no-clear semantics; the writeback fakes already write valid JSON to the stdout file and exit cleanly via `done <- nil` with a blocking waiter, so the AC8 invariant assertions are confirmed unchanged.
- Confirms `goal_workon.go` and its tests are already in the spec-041 target state (they were never reverted) and are left untouched.
- ⚠️ IMPORTANT TREE CONFLICT FLAGGED FOR THE HUMAN REVIEWER: the task-side half of this spec was REVERTED in the tree after approval (commit dae6563, released v0.118.3) because persist-after-exit left `claude_session_id` empty during the turn and the child's own session-connect scanned the transcript dir by mtime and bound the task to a live unrelated session (reproduced live 2026-09-01). This prompt implements the spec AS APPROVED — re-applying start→persist — and the reviewer must adjudicate the conflict at audit time (details in requirement 2's comment): (A) approve, spec-041 wins and the session-connect regression is owned as a follow-up; or (B) reject and re-scope the spec to treat the reversion as the target. Prompt 3 is coupled to this decision.
- Runs `make test` and the spec-041 AC7-9 grep gate.
</summary>

<objective>
Make the task path of `work-on` persist `claude_session_id` only after the detached headless turn has finished, so no failure leaves a resumable-looking id behind — matching the goal path, which already ships this behavior. This prompt covers spec-041 ACs 7-9 and depends on prompt 1 having shipped (its `StartSession` block-until-exit behavior is what these tests exercise).
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file; focus on `handleClaudeSession` (starts at line 278), `persistSessionAndMetrics` (line 218), and `clearSessionAndMetrics` (line 247).
- `pkg/ops/goal_workon.go` — the whole file; this is the structural TEMPLATE the reordered task path must match (`handleClaudeSession` at line 198, `persistGoalSessionID` at line 174).
- `pkg/ops/workon_test.go` — the whole file; the AC7 contexts are "when persisting the session id before spawning" (line 906), "when the session id write precedes the spawn" (line 160), "when the spawn fails" (line 982), "when the pre-spawn persist re-read fails" (line 877), and the comment-era specs at lines 98-157.
- `pkg/ops/goal_workon_test.go` — the whole file; the AC7 context "when persisting the goal session id after the child exits" starts at line 355 (already spec-041 — confirm, do not touch).
- `pkg/ops/workon_session_writeback_test.go` — the whole file.
- `docs/work-on-session-lifecycle.md` — the design record for the post-exit ordering (its task-path sections were reverted in v0.118.3; prompt 3 fixes the doc — do NOT edit it here).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega conventions.

NOTE: git IS available in this container (`workflow: direct`, no hideGit) — but this prompt has no git commands; AC10's `scenarios/005` guard is verified in prompt 1.
</context>

<requirements>
<!-- ⚠️ HUMAN REVIEWER — TREE CONFLICT WITH THIS SPEC (READ BEFORE APPROVING) ⚠️

The task-side half of spec-041 was REVERTED in the tree after this spec was approved:

- Commit 247a789 (released v0.117.1) applied spec-041 to workon.go: persist claude_session_id only AFTER the turn, no compensating clear.
- Commit dae6563 (released v0.118.3) REVERTED that task-side half — "fix(workon): persist the fresh session id before the headless turn". Its rationale (verbatim): persist-after-exit left claude_session_id empty during the turn, and the child's own /vault-cli:work-on-task session-connect scanned the transcript dir by mtime and wrote a LIVE UNRELATED session's id into the field (reproduced live 2026-09-01). It re-adopted spec-040's persist-before-spawn + a re-read-based compensating clear on the task path ONLY.
- What the reversion KEPT (not reverted): claude_session.go's block-until-exit + validateSessionTurn (spec-041), the spec-042 per-session lock, and the vault-ui-side "gate Resume on live cards" resolver. goal_workon.go was never reverted and still matches spec-041.

The current tree therefore FAILS spec-041 AC7 (grep 'After(childExitAt)' pkg/ops/workon_test.go = 0) and AC9 (grep 'clearSessionAndMetrics' pkg/ops/workon.go = 3).

This prompt implements the spec AS APPROVED: it re-applies the start→persist reorder to workon.go and deletes clearSessionAndMetrics. That re-introduces the session-connect bug v0.118.3 fixed UNLESS a separate mechanism ships in the same batch (e.g. making the child's session-connect read the id from a non-frontmatter source, or vault-ui writing the id earlier) — none exists in this spec.

The reviewer must decide at audit time between:
  (A) Approve this prompt — spec-041 wins; the reorder is re-applied and the session-connect regression is accepted/owned as a follow-up.
  (B) Reject and re-scope spec-041 — treat dae6563 as the new target; AC7/AC9 would need rewording to assert pre-spawn + compensating-clear behavior (which the current tree already satisfies).
If you approve, also approve prompt 3 — it rewrites the doc back to post-exit and is coupled to this decision.

For the executing agent: implement the requirements below as written — the spec is the source of truth for this batch; the reviewer adjudicates the conflict at audit time. Do not "fix" the requirements to preserve the reversion. -->

1. **Guard — prompt 1 must have shipped.** Before doing anything, confirm prompt 1's deliverables exist: `grep -c 'validateSessionTurn' pkg/ops/claude_session.go` >= 2, `grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go` >= 1, and `grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go` >= 1. If ANY of these is 0, STOP and report `"status":"failed"` with message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed"` — do not proceed.

2. **Reorder the fresh-start path in `workon.go`'s `handleClaudeSession` to start→persist.** The current code (the fresh-start block starting at the `prompt := fmt.Sprintf(...)` line and ending at `return sessionID, nil`) persists BEFORE the spawn and compensates on failure. Replace the block from `prompt := fmt.Sprintf(...)` through `return sessionID, nil` with this exact code (structurally identical to `goal_workon.go`'s non-interactive branch, and to the interactive branch — same checks, same error strings):
   ```go
   	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
   	sessionID := w.uuidGenerator()
   	slog.Info("starting claude session", "task", task.Name)
   	// Captured BEFORE the spawn — the turn's true start, not the write time.
   	startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
   	if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
   		// No compensating clear needed: nothing was written for this id, so there is nothing to undo.
   		return "", errors.Wrap(ctx, err, "start claude session")
   	}
   	sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
   	return sessionID, err
   ```
   The old code being replaced (delete these lines entirely): the `// Persist id + metrics BEFORE the child exists...` comment block, the `if _, err := persistSessionAndMetrics(...); err != nil { return "", errors.Wrap(ctx, err, "persist claude session before spawn") }` call, and the `if err := w.starter.StartSession(...); err != nil { ... clearSessionAndMetrics ... }` compensating-clear block. The cached-session path (the `if existing := task.ClaudeSessionID(); existing != ""` branch, which re-reads and re-persists via `persistSessionAndMetrics`) must be UNCHANGED. Note the function returns `(string, error)` — the new `return "", errors.Wrap(...)` is 2 values; `sessionID, err := ...` compiles because `sessionID` is already declared above and `err` is newly introduced in that scope (this is exactly `goal_workon.go` line 224's pattern). Do NOT copy the spec Design's `return "", nil, errors.Wrap(...)` snippet — that 3-value form is a spec typo and does not compile against the real signature.

3. **Delete the dead `clearSessionAndMetrics` function from `workon.go`.** Remove the entire function (its doc comment plus body, currently at lines 243-269). After this, `grep -rn 'clearSessionAndMetrics' pkg/` must return NOTHING (AC9). Its only call site was the compensating-clear block deleted in requirement 2. Do not add any replacement.

4. **Reword the doc comments in `workon.go` that describe the pre-spawn design.** Update `persistSessionAndMetrics`'s comment (currently lines 207-217) — replace the sentence `Used pre-spawn on the fresh-start path (the session id is new and must be on disk before the child exists) and on the cached-session path (the id already exists and is preserved).` with `Used post-exit on the fresh-start path (the session id is new and is persisted only after the headless turn completes cleanly) and on the cached-session path (the id already exists and is preserved).` Update `handleClaudeSession`'s comment (currently lines 271-277) to mirror `goal_workon.go`'s (lines 192-197): `On both branches the session id is persisted only AFTER the headless turn has finished cleanly, so an id on disk means the session is resumable rather than merely that one was started. Nothing is written on any failure path, so there is no compensating clear: frontmatter the child wrote before failing stays untouched, and the Vault UI correctly keeps offering Start.` Keep the cached-session sentence.

5. **Confirm `goal_workon.go` is already in the target state — do NOT change it.** `handleClaudeSession` must already persist AFTER `StartSession` on both branches, return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear, and keep the cached path as `return existing, nil`. `persistGoalSessionID` must already return an empty id on failure. If it matches, leave it untouched (AC9's `clearGoalSession` grep already passes — `clearGoalSession` does not exist in the repo).

6. **Rework AC7 in `workon_test.go` — "when persisting the session id before spawning" (lines 906-980).** Rename the context to `"when persisting the session id after the child exits"`. In the variable block, rename `spawnAt time.Time` to `childExitAt time.Time` and update the fake `detachRun` so the timestamp is captured as `childExitAt = time.Now()` at the point the child's `done` channel is fed `nil` (currently `spawnAt = time.Now()` before `done <- nil`). The BeforeEach already captures `spawnedSessionID` from the `--session-id` argv, writes valid turn JSON to the stdout `*os.File`, and uses a blocking waiter — keep all of that. Rename the `It` to `"writes the session id to storage only after the child exits"` and replace its body with exactly:
   ```go
   Expect(err).To(BeNil())
   Expect(writeTaskAt).NotTo(BeZero())
   Expect(childExitAt).NotTo(BeZero())
   Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
   Expect(writtenSessionID).To(Equal(spawnedSessionID))
   ```
   Keep the existing comment about AC5's "id equals the value in task frontmatter". This yields the AC7 evidence `After(childExitAt)` (currently absent — the old assertion is `Expect(writeTaskAt.Before(spawnAt)).To(BeTrue())`).

7. **Invert "when the session id write precedes the spawn" in `workon_test.go` (lines 160-185).** Rename the context to `"when the session id write follows the spawn"`, rename the `It` to `"writes the session id to storage after StartSession returns"`, and change the final assertion from `Expect(writeSeq).To(BeNumerically("<", startSeq))` to `Expect(writeSeq).To(BeNumerically(">", startSeq))`. The `WriteTaskStub`/`StartSessionStub` sequencing setup stays as-is.

8. **Delete the clear-based failure tests in `workon_test.go` and add a persists-nothing assertion (AC9).** In the `"when the spawn fails"` context (lines 982-1041):
   - Keep `"returns the wrapped spawn error"` and `"returns Success=false"` unchanged.
   - DELETE `"clears the pre-persisted session id and the metrics entry for the failed run"` and the nested `Context("when the compensating clear itself fails", ...)` entirely — the clear no longer exists.
   - ADD one spec to the context:
     ```go
     It("persists no session id when the spawn fails", func() {
         // Execute's write only — nothing is pre-persisted and there is no
         // compensating clear, so a failed turn leaves no id or metrics entry.
         Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
         Expect(writtenIDs).To(Equal([]string{""}))
         Expect(writtenMetricsSessions[0]).To(BeEmpty())
     })
     ```
   - Reword the BeforeEach comment that currently explains the compensating-clear pointer mutation: it now reads as a snapshot of what each write lands (Execute's write carries an empty id because the id is minted inside `handleClaudeSession` after `Execute` writes the task).

9. **Rework "when the pre-spawn persist re-read fails" in `workon_test.go` (lines 877-904) to post-exit.** Rename the context to `"when the post-exit persist re-read fails"` and the `It` `"does not append a metrics entry when the pre-spawn re-read fails"` to `"does not append a metrics entry when the post-exit re-read fails"`. Reword the BeforeEach comment from `The failing call is persistSessionAndMetrics' PRE-SPAWN re-read, which runs before StartSession is ever called.` to `The failing call is persistSessionAndMetrics' POST-EXIT re-read, which runs after StartSession returns (the mock starter returns nil immediately).` and the comment on the write-count `It` from `pre-spawn persist failed before writing` to `post-exit persist failed before writing`. The mock call indexes do NOT change: call 0 loads the task, `mockStarter.StartSessionReturns(nil)` makes no storage call, call 1 is the post-exit re-read that fails. All four `It` bodies stay byte-identical.

10. **Reword the comment-era specs in `workon_test.go` (lines 98-157) to post-exit terminology.** These all drive the mock starter and their assertions/counts are unchanged; only comments and one title change:
    - `"calls FindTaskByName"` comment: `Twice: once to load the task, once to re-read it before the child is spawned so the fresh session id lands on disk before the child exists.` → `Twice: once to load the task, once to re-read it after the child exits so the post-exit persist lands the fresh session id without reverting the child's frontmatter writes.`
    - `"re-reads the task from the vault path before spawning the session"`: rename title to `"re-reads the task from the vault path after the child exits"` and its comment to `The second FindTaskByName is persistSessionAndMetrics' post-exit re-read: the session id is written to disk only after the child has exited.`
    - `"Fresh run records one entry"` comment: `The metrics entry lands in the pre-spawn persist write: write 0 is Execute's status/assignee/phase write, write 1 is the pre-spawn persist.` → `The metrics entry lands in the post-exit persist write: write 0 is Execute's status/assignee/phase write, write 1 is the post-exit persist.`

11. **Reword `pkg/ops/workon_session_writeback_test.go` to post-exit semantics; confirm the fakes (AC8); keep the pinned counts byte-identical.** The fakes ALREADY write a valid JSON line to the stdout `*os.File` (`{"session_id":"<pinnedSessionID>","num_turns":3,"is_error":false,"result":"done"}` in both the task and goal success contexts) and exit cleanly via `done <- nil` with a blocking waiter — confirm this, do NOT change it. Reword only comments and titles:
    - The task-context BeforeEach comment from the pre-spawn description to: `Simulate the real headless turn: work-on spawns the child first, then the Claude session runs plan-task -> execute-task and writes its own frontmatter on top of that file inside the detached child (the detachRun fake). Only after the child exits does work-on re-read and persist the session id, so the child's frontmatter survives.`
    - The goal-context BeforeEach comment's `...while the parent has already returned within the liveness window.` → `...while the parent blocks waiting for the detached turn.`
    - Rename the context `"when the child exits non-zero inside the liveness window"` to `"when the child exits non-zero within the turn wait"` and the `It` title from `clears the pre-persisted session id and preserves the child's frontmatter write when the child exited non-zero inside the window` to `persists no session id and preserves the child's frontmatter write when the child exited non-zero within the turn wait`.
    - Reword the comment `The liveness window has NOT elapsed when the child exits, so the starter must treat the exit as inside-the-window.` to `The turn wait has NOT elapsed when the child exits, so the child-exit branch of the select wins.`
    - Reword the mechanism comments that describe the pre-spawn persist + compensating clear so they describe the post-exit no-clear ordering. The on-disk assertions they annotate (phase survives, raw file has no `claude_session_id:`, no `pinnedSessionID`) are byte-identical under the new ordering — a failed turn simply never persisted anything — so DO NOT touch the assertions.
    - Do NOT touch the pinned-count strings anywhere in this file: `TaskPhaseExecution` (==2), `GoalPhaseExecution` (==2), `session_note` (==4), `MetricsSessions()` (==2), `ClaudeSessionID()` (==2). The AC8 greps must stay byte-identical.

12. **Confirm the goal AC7 test and AC8 assertions are already correct — do not touch them.** `goal_workon_test.go` "when persisting the goal session id after the child exits" already asserts `writeGoalAt.After(childExitAt)` with both non-zero. `workon_session_writeback_test.go`'s task and goal `It`s already assert the child's phase + `session_note` survive, `ClaudeSessionID() == pinnedSessionID`, and `MetricsSessions()` length 1. Confirm and leave unchanged.

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
grep -c 'start work-on session' pkg/ops/workon_session_writeback_test.go  # >= 1
# Reversion vocabulary must be gone from workon.go:
grep -c 'persist claude session before spawn' pkg/ops/workon.go        # == 0
grep -c 'pre-spawn\|pre-persisted' pkg/ops/workon.go                   # == 0
```

SECONDARY — syntax validity:
```
gofmt -e -l pkg/ops/workon.go pkg/ops/workon_test.go pkg/ops/workon_session_writeback_test.go   # must list NO files
```

TESTS:
```
make test   # must exit 0
```

`make precommit` is NOT run in this prompt — it is the batch's full-gate check (AC13) in prompt 3. Running only `make test` + the grep gate here is correct.
</verification>
