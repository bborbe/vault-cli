---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-11-17T00:20:00Z"
---

# Persist the task session id only after the headless turn exits (spec 041, prompt 2 of 3)

<summary>
- Changes the task path of `work-on` so the headless turn is started FIRST and the session id plus its metrics entry are written to the task only AFTER the turn exits cleanly — matching the goal and topic paths, which already behave this way.
- Removes the compensating-clear function that exists only to undo a write made before the turn, and the tests that exercise it. With nothing written early there is nothing to undo: a failed turn simply leaves no id.
- Reworks the task-side sequencing tests: the "write precedes the spawn" assertions invert, the "write before spawning" test becomes "write after the child exits" and gains an explicit ordering assertion, and the clear-based failure tests are replaced by a "persists nothing" assertion.
- Rewords the stale write-before-turn comments and titles in the write-back test file to the post-exit, no-clear semantics. Every on-disk assertion in that file stays byte-identical — the ids never land either way, so only the prose and titles change.
- Confirms the goal and topic paths and their tests are already in the target state and leaves them untouched.
- Runs the spec's AC7-9 evidence gate plus the full repository gate.
- IMPORTANT FLAG FOR THE HUMAN REVIEWER: the task-side half of this spec was deliberately REVERTED in the tree after the spec was approved (see `CHANGELOG.md` `## v0.118.3`), and the released note's stated rationale was the child's session-connect attaching an unrelated live transcript. That rationale has since been superseded at the agent layer — the session-connect now resolves its own uuid deterministically and forbids the newest-transcript scan — so re-applying the ordering no longer reproduces the mechanism the reversion named. A narrower residual risk remains (details and the two adjudication options are in the comment at the top of `<requirements>`). The docs prompt is coupled to that decision.
- Carries no git commands: `git` is masked in this container, and the scenario-005 guard is prompt 1's content checksum.
</summary>

<objective>
Make the task path of `work-on` persist `claude_session_id` only after the detached headless turn has finished cleanly, so no failure path can leave a resumable-looking id behind — matching the goal and topic paths, which already ship this behaviour. Covers spec 041 ACs 7-9 and depends on prompt 1 having shipped (its block-until-exit `StartSession` is what these tests exercise).
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file (586 lines). Focus on `handleClaudeSession` (line 438), `persistSessionAndMetrics` (line 378), `clearSessionAndMetrics` (line 407), `sessionFailureResult` (line 278), and the re-read/write helpers. Note `handleClaudeSession` receives the starter as its LAST PARAMETER named `starter` (line 445) — the struct also has a `starter` field, but the parameter is the one Execute passes and the one nil-checked at line 452, so the reordered code must keep using `starter`, not `w.starter`.
- `pkg/ops/goal_workon.go` — the whole file (267 lines). This is the structural TEMPLATE the reordered task path must match: `handleClaudeSession` (line 226) starts the turn first and persists only after it returns cleanly, with no compensating clear, and `persistGoalSessionID` (line 191) returns an empty id on failure. Its non-interactive branch comment ("Non-interactive branch: persist the id only AFTER the turn has finished.") is the wording to mirror.
- `pkg/ops/topic_workon.go` — read `handleClaudeSession` (line 228) only. It is already in the same target state; it is confirmation material, not an edit target.
- `pkg/ops/workon_test.go` — the whole file (1292 lines). The contexts this prompt reworks are `Context("success", ...)` (line 95), `Context("when the session id write precedes the spawn", ...)` (line 162), `Context("when the pre-spawn persist re-read fails", ...)` (line 1073), `Context("when persisting the session id before spawning", ...)` (line 1102), and `Context("when the spawn fails", ...)` (line 1178).
- `pkg/ops/goal_workon_test.go` — read `Context("when persisting the goal session id after the child exits", ...)` (line 358) and `Context("goal work-on persists nothing for a failed turn", ...)` (line 650) to confirm the goal path is already in the target state. Do not modify this file.
- `pkg/ops/workon_session_writeback_test.go` — the whole file (524 lines). Note the local `newStarter(detachRun)` closure (line 57), the `pinnedSessionID` constant (line 28), and that every success-path fake already writes a valid turn-JSON line to the caller-owned `stdout *os.File` and then feeds `done <- nil` with a blocking waiter.
- `docs/work-on-session-lifecycle.md` — the design record. Its task-path sections currently describe the write-before-the-turn ordering; prompt 3 rewords them. Do NOT edit the doc here.
- `CHANGELOG.md` — read `## v0.118.3` (line 957, the entry documenting the reversion this prompt reverses) AND `agents/work-on-task-assistant.md` § Session connect (line 127). Read both before writing the reviewer comment's claims; do not paraphrase beyond what they say.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `errors.Wrapf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Errorf(ctx, ...)` idiom from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, Counterfeiter mocks.

**`git` is MASKED in this container.** `.git` is a character device (`crw-rw-rw-`), so every `git` invocation fails with `fatal: not a git repository`. This prompt issues no git commands; do not add any. The spec's AC10 `git diff --exit-code HEAD -- scenarios/005-...` guard cannot run here and is replaced by a content checksum in prompt 1 — do not repeat or reinvent it here. Nothing in this prompt modifies `scenarios/005-work-on-resume-auto-invokes-subtask.md`.

**The container's HTTP proxy breaks the suite's localhost probes.** A bare `make test` fails with `❌ stub attention store never answered on http://127.0.0.1:<port>` — the loopback probe is routed through the session proxy. That is an environment artifact, NOT a code failure: the identical tree passes when loopback is excluded. Every `make test` / `make precommit` invocation in `<verification>` is prefixed with `env NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost`. Use that prefix; do not "fix" the probe script or the Makefile.
</context>

<requirements>
<!-- ⚠️ HUMAN REVIEWER — READ BEFORE APPROVING: THIS PROMPT CONTRADICTS A SHIPPED, DOCUMENTED FIX ⚠️

The task-side half of spec 041 was REVERTED in the tree AFTER the spec was approved. The
reversion is recorded in `CHANGELOG.md` under `## v0.118.3` (line 957):

  "`task work-on` now persists the fresh `claude_session_id` and its `metrics_sessions`
   entry to the task file before the headless Claude session is spawned, so the session's
   own `/vault-cli:work-on-task` session-connect reads the field already set and keeps the
   fresh session instead of scanning the transcript directory and attaching whichever
   transcript was most recently modified (in a fleet of concurrent sessions, rarely the
   fresh one); a failed spawn now triggers a re-read-based compensating clear ..."

That entry is exactly the ordering this prompt removes. It did not touch
`claude_session.go`'s block-until-exit + validation (prompt 1), the spec 042 per-session
lock, or `goal_workon.go` / `topic_workon.go` (which persist post-exit).

The current tree therefore FAILS spec 041 AC7 (`grep -c 'After(childExitAt)' pkg/ops/workon_test.go` == 0) and AC9 (`grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 3).

WHY THE v0.118.3 RATIONALE IS NOW SUPERSEDED — read
`agents/work-on-task-assistant.md` § Session connect (line 127) before judging this. The
regression the reversion named was a **newest-transcript mtime scan**: with the field
empty, the child's session-connect picked whichever transcript was most recently modified.
That scan is now FORBIDDEN in the assistant (step 2: "Never detect by newest-transcript:
the `ls -t ... | head -1` mtime scan is forbidden"), and detection is deterministic first
via `$CLAUDE_CODE_SESSION_ID`, which the harness sets to the session it launched. The
detached child is launched with `--session-id <minted uuid>`, so the deterministic branch
yields the minted uuid — the same id the post-exit persist would write. Re-applying the
post-exit ordering therefore does NOT reproduce "bound to a live unrelated session".

WHAT ACTUALLY REMAINS, and it is narrower: if the child's session-connect DOES write
during the turn (deterministic branch, so normally its own minted id), the post-exit
`persistSessionAndMetrics` finds the field non-empty, skips the id write, and appends a
SECOND `metrics_sessions` entry for the same id. That is a duplicate analytics row, which
`docs/work-on-session-lifecycle.md` says is handled on the read side (per-distinct-session
dedupe) and which spec 036/038 deliberately froze as an accumulating list. The genuinely
bad case survives only if the child's detection FAILS and something else stamped a foreign
non-empty id during the turn: `persistSessionAndMetrics` skips the write but returns the
minted id (`if refreshed.ClaudeSessionID() == "" { ... }` then `return sessionID, nil`), so
`Execute` would report the minted id while the task names the foreign one. Nothing in this
spec supplies a mechanism to close that, so it is accepted, documented residual risk.

The spec's Constraints section requires this reorder ("No compensating clear ... delete
them"), and AC7/AC9 pin it, so this prompt implements the spec AS APPROVED.

The reviewer must decide at audit time:
  (A) Approve — spec 041 wins; the reorder is re-applied, the duplicate metrics row is
      accepted, and the foreign-id residual risk is owned as a follow-up if it matters.
  (B) Reject and re-scope spec 041 — treat the v0.118.3 behaviour as the target. Then
      AC7/AC9 must be reworded to assert the write-before-the-turn ordering plus the
      compensating clear (which the current tree already satisfies), and prompt 3 must be
      rejected too, because its doc reword and its CHANGELOG bullet both describe the
      post-exit ordering.

For the executing agent: implement the requirements below as written. The approved spec is
the source of truth for this batch; the reviewer adjudicates the conflict at audit time.
Do NOT "fix" the requirements to preserve the reversion, and do NOT add a compensating
clear or any other new mechanism that the spec does not name. -->

1. **Fail-loud gate — prompt 1 must have shipped.** Before changing anything, confirm prompt 1's deliverables exist:
   ```
   grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go                                  # must be >= 1
   grep -c 'validateSessionTurn' pkg/ops/claude_session.go                                 # must be >= 2
   grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go   # must be >= 1
   ```
   If ANY of these is 0, STOP and report `"status":"failed"` with message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed"`. Do NOT proceed and do NOT re-implement prompt 1's work.

2. **Reorder the fresh-start path of `workon.go`'s `handleClaudeSession` to start-then-persist.** Replace the block that begins at the `// Persist id + metrics BEFORE the child exists, on both branches, ...` comment (line 461) and ends at the function's closing `return sessionID, nil` (line 479) with exactly:
   ```go
   	// Captured BEFORE the spawn — the turn's true start, not the write time.
   	startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
   	if err := starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
   		// No compensating clear needed: nothing was written for this id, so there is
   		// nothing to undo. Frontmatter the child wrote before failing stays untouched.
   		return "", errors.Wrap(ctx, err, "start claude session")
   	}
   	sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
   	return sessionID, err
   ```
   Keep the preceding lines (`prompt := fmt.Sprintf(...)`, `sessionID := w.uuidGenerator()`, `slog.Info("starting claude session", "task", task.Name)`) and the bootstrap comment above `prompt` exactly as they are. DELETE: the `// Persist id + metrics BEFORE the child exists ...` comment (and its four continuation lines), the pre-turn `persistSessionAndMetrics` call and its `"persist claude session before spawn"` wrap, and the whole `if clearErr := w.clearSessionAndMetrics(...)` block.

   Use the `starter` PARAMETER, never `w.starter`: `handleClaudeSession` takes the starter as its last parameter (line 445), nil-checks that parameter at line 452, and Execute passes the launcher-resolved starter into it (line 140). The struct field `w.starter` is the vault-default starter and is NOT the same value when a launcher was resolved, so `w.starter` would silently bypass launcher resolution and disagree with the nil check above.

   Signature note: `handleClaudeSession` returns `(string, error)` — two values. The spec's Design snippet shows `return "", nil, errors.Wrap(...)` (three values); that is a spec typo and does NOT compile here. Use the two-value form above, which is exactly `goal_workon.go`'s pattern. The `sessionID, err := ...` line compiles because `sessionID` is already declared in the function scope and `err` is newly introduced by it.

   The cached-session path at the top of `handleClaudeSession` (the `if existing := task.ClaudeSessionID(); existing != ""` branch, lines 447-451) must be UNCHANGED. Note that `persistSessionAndMetrics` is called from two places in this file — the cached path (keep) and the fresh path (this reorder). It has no other callers anywhere in the repo.

3. **Delete the dead `clearSessionAndMetrics` method from `workon.go`.** Remove the whole method — its doc comment and body (lines 403-429). Its only call site was the block deleted in requirement 2. Do not add a replacement, and do not add any other clearing mechanism (the spec's Non-goals forbid it). After this, `grep -rn 'clearSessionAndMetrics' pkg/` must return nothing. `ClearClaudeSessionID()` on the domain type stays — `pkg/ops/complete.go` still uses it at line 294.

4. **Reword the `workon.go` doc comments that describe the write-before-the-turn design.**
   - `persistSessionAndMetrics`'s comment (lines 367-377): replace the sentence `Used pre-spawn on the fresh-start path (the session id is new and must be on disk before the child exists) and on the cached-session path (the id already exists and is preserved).` with `Used post-exit on the fresh-start path (the session id is new and is persisted only after the headless turn completes cleanly) and on the cached-session path (the id already exists and is preserved).` Reflow to stay within the 100-column golines limit. Leave the surrounding sentences about the load-bearing re-read and the empty-id rule as they are.
   - `handleClaudeSession`'s comment (lines 431-437): replace it with the wording `goal_workon.go` already uses for the same method, adapted to the task path — on both branches the session id is persisted only AFTER the headless turn has finished cleanly, so an id on disk means the session is resumable rather than merely that one was started; nothing is written on any failure path, so there is no compensating clear, and frontmatter the child wrote before failing stays untouched so the Vault UI correctly keeps offering Start. Keep the cached-session sentence.
   - `sessionFailureResult`'s comment (lines 275-277) says the warnings are `the accumulated warnings (including any compensating-clear warning)`. Reword that clause — there is no clear, so the warnings are only the assignee and daily-note ones. Do not change the function's behaviour.

5. **Confirm `goal_workon.go` and `topic_workon.go` are already in the target state — do NOT change either.** Each `handleClaudeSession` must already start the turn first and persist after on both branches, return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear, and keep the cached path as `return existing, nil`. `persistGoalSessionID` / `persistTopicSessionID` must already return an empty id on failure. `clearGoalSession` must not exist (`grep -c 'clearGoalSession' pkg/ops/goal_workon.go` == 0), and there must be no `clearTopicSession`. If it all matches, leave both files untouched.

6. **Rework `"when persisting the session id before spawning"` in `workon_test.go` to post-exit (AC7).** Rename the context (line 1102) to `"when persisting the session id after the child exits"`. In the variable block (lines 1103-1109) rename `writeTaskAt, spawnAt time.Time` to `writeTaskAt, childExitAt time.Time`, and in the fake `detachRun` (line 1141) rename the `spawnAt = time.Now()` assignment to `childExitAt = time.Now()` — keep it in the same position (immediately before the buffered `done` channel is fed, which is the child's exit point). The `BeforeEach` already captures `spawnedSessionID` from the `--session-id` argv, writes valid turn JSON to the `stdout *os.File`, and uses a blocking waiter; keep all of that unchanged. Rename the `It` (line 1163) to `"writes the session id to storage only after the child exits"` and replace its body with exactly:
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

7. **Invert `"when the session id write precedes the spawn"` in `workon_test.go` (line 162).** Rename the context to `"when the session id write follows the spawn"`, rename the `It` (line 181) to `"writes the session id to storage after StartSession returns"`, and flip the final assertion from `Expect(writeSeq).To(BeNumerically("<", startSeq))` to `Expect(writeSeq).To(BeNumerically(">", startSeq))`. Leave the `WriteTaskStub` / `StartSessionStub` sequencing setup (lines 165-179) exactly as it is. The ordering is still deterministic: `Execute` writes first (empty id, so `writeSeq` is not set), then `StartSession` runs (`startSeq`), then the post-exit persist writes the id (`writeSeq`).

8. **Replace the clear-based failure tests in `"when the spawn fails"` (line 1178) with a persists-nothing assertion (AC9).** In that context:
   - Keep `"returns the wrapped spawn error"` (line 1201) and `"returns Success=false"` (line 1206) unchanged.
   - DELETE the spec `"clears the pre-persisted session id and the metrics entry for the failed run"` (line 1210) and the whole nested `Context("when the compensating clear itself fails", ...)` (lines 1221-1236).
   - ADD one spec:
     ```go
     It("persists no session id when the spawn fails", func() {
         // Execute's write only — nothing is written before the turn and there is no
         // compensating clear, so a failed turn leaves no id and no metrics entry.
         Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
         Expect(writtenIDs).To(Equal([]string{""}))
         Expect(writtenMetricsSessions[0]).To(BeEmpty())
     })
     ```
   - Reword the `BeforeEach` comment (lines 1184-1188, currently starting `Snapshot each write's state as it lands:`) so it no longer describes a clear mutating a pointer a pre-turn write wrote. Replace the sentence `the mock returns the same *Task pointer for every FindTaskByName, so the compensating clear mutates the very pointer the pre-spawn persist wrote — reading WriteTaskArgsForCall after Execute returns would see the already-cleared task.` with `the mock returns the same *Task pointer for every FindTaskByName. With nothing written before the spawn and no clear to run, there is exactly one write, and it carries an empty id because the id is minted inside handleClaudeSession after Execute has already written the task.` Keep the final sentence about the stub capturing ids at write time.

9. **Rework `"when the pre-spawn persist re-read fails"` in `workon_test.go` to post-exit (line 1073).** Rename the context to `"when the post-exit persist re-read fails"` and the `It` `"does not append a metrics entry when the pre-spawn re-read fails"` (line 1096) to `"does not append a metrics entry when the post-exit re-read fails"`. Reword the `BeforeEach` comment (lines 1076-1077) from `The failing call is persistSessionAndMetrics' PRE-SPAWN re-read, which runs before StartSession is ever called.` to `The failing call is persistSessionAndMetrics' POST-EXIT re-read, which runs after StartSession returns (the mock starter returns nil immediately).`, and the comment on the write-count `It` (line 1092) from `Execute's write only — the pre-spawn persist failed before writing.` to `Execute's write only — the post-exit persist failed before writing.` The mock call indexes do NOT change: call 0 loads the task, `mockStarter.StartSessionReturns(nil)` makes no storage call, call 1 is the post-exit re-read that fails. All four `It` bodies stay byte-identical.

10. **Reword the remaining write-before-the-turn wording in `workon_test.go`'s `"success"` context (line 95).** Assertions and counts are unchanged; only comments and two titles change:
    - `"calls FindTaskByName"` comment (lines 101-102): `Twice: once to load the task, once to re-read it before the child is spawned so the fresh session id lands on disk before the child exists.` → `Twice: once to load the task, once to re-read it after the child exits so the post-exit persist lands the fresh session id without reverting the child's frontmatter writes.`
    - `"re-reads the task from the vault path before spawning the session"` (line 112): rename to `"re-reads the task from the vault path after the child exits"` and reword its comment (line 114) to `The second FindTaskByName is persistSessionAndMetrics' post-exit re-read: the session id is written to disk only after the child has exited.`
    - `"Fresh run records one entry"` comment (lines 151-152): `The metrics entry lands in the pre-spawn persist write: write 0 is Execute's status/assignee/phase write, write 1 is the pre-spawn persist.` → `The metrics entry lands in the post-exit persist write: write 0 is Execute's status/assignee/phase write, write 1 is the post-exit persist.`
    The `Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(2))` assertion stays as it is — Execute's write plus the post-exit persist is still two.

11. **Reword `pkg/ops/workon_session_writeback_test.go` to post-exit semantics — comments and titles ONLY (AC8).** The fakes already write a valid turn-JSON line to the `stdout *os.File` and exit cleanly via `done <- nil` with a blocking waiter; confirm that and do NOT change it. Every on-disk assertion in this file holds unchanged under the new ordering — the ids simply never land — so DO NOT touch any assertion. Apply exactly these replacements (this is every stale occurrence in the file; the drift-guard grep in `<verification>` requires all of them):
    1. Task context `BeforeEach` comment (line 128): replace `Simulate the real headless turn: work-on persists the fresh session id and its metrics entry to the file BEFORE spawning (pre-spawn persist), then the spawned Claude session runs plan-task -> execute-task and writes its own frontmatter on top of that file inside the detached child (the detachRun fake). Nothing writes to the file after the child's own write, so the child's frontmatter survives.` with `Simulate the real headless turn: work-on spawns the child first, then the spawned Claude session runs plan-task -> execute-task and writes its own frontmatter to that file inside the detached child (the detachRun fake). Only after the child exits does work-on re-read and persist the session id, so the child's frontmatter survives.`
    2. Task `It` comment (line 187): `The metrics entry lands in the pre-spawn persist write (real storage round-trip) and survives because nothing writes to the file after the child's own write.` → `The metrics entry lands in the post-exit persist write (real storage round-trip), which re-reads the file the child already wrote.`
    3. Goal context `BeforeEach` comment (line 216): replace the clause `while the parent has already returned within the liveness window.` with `while the parent blocks waiting for the detached turn.`
    4. Rename `Context("when the child exits non-zero inside the liveness window", ...)` (line 270) → `Context("when the child exits non-zero within the turn wait", ...)`.
    5. Comment (line 286): replace `The liveness window has NOT elapsed when the child exits, so the starter must treat the exit as inside-the-window. A nil-returning waiter would race the select against the child's buffered exit;` with `The turn wait has NOT elapsed when the child exits, so the child-exit branch of the select wins. A nil-returning waiter would race the select against the child's buffered exit;` — keep the rest of that comment about the blocking waiter verbatim.
    6. Rename the `It` (line 318) `"clears the pre-persisted session id and preserves the child's frontmatter write when the child exited non-zero inside the window"` → `"persists no session id and preserves the child's frontmatter write when the child exited non-zero within the turn wait"`.
    7. Body comment (line 342): replace `The child's write survived: the pre-spawn persist wrote the id and this run's metrics entry, the child's \`phase: planning\` write landed on top, then the compensating clear re-read the file (so \`phase: planning\` was already on disk) and removed only the id and this run's metrics entry. A clear from the stale in-memory copy would have reverted phase back to execution.` with `The child's write survived: the turn failed, so nothing was ever written for this id, and the child's own \`phase: planning\` write is what remains on disk. No re-read-modify-write runs on this path, so nothing can revert phase back to execution.`
    8. `On-disk shape:` comment (line 353): replace `On-disk shape: the pre-spawn persist wrote the id and this run's metrics entry, and the compensating clear removed them again after the failed spawn (the AC6 grep pins keep the count of the id/metrics accessor calls in this file at 2, so the absence is asserted via the raw file). The id itself is the shared fingerprint of both claude_session_id and the metrics_sessions entry, so its absence proves neither survived the clear.` with `On-disk shape: no id was ever persisted on this path, so its absence from the raw file proves the invariant directly (the AC6 grep pins keep the count of the id/metrics accessor calls in this file at 2, so the absence is asserted via the raw file). The id is the shared fingerprint of both claude_session_id and the metrics_sessions entry, so its absence proves neither was written.`
    9. Comment above `Context("when the child exits non-zero after writing a valid turn result", ...)` (line 370): replace `under the spec 045 precedence a validated turn result is authoritative over the child's non-zero exit, so the pre-persisted session id survives what used to be the failed-spawn path and the Vault UI can offer Resume.` with `under the spec 045 precedence a validated turn result is authoritative over the child's non-zero exit, so the post-exit persist runs and the Vault UI can offer Resume.`
    10. Rename the `It` (line 412) `"retains the pre-persisted session id when the turn result validated despite the non-zero exit"` → `"persists the session id when the turn result validated despite the non-zero exit"`.
    11. Comment (line 424): replace `The retain is caller-side (handleClaudeSession/Execute): the pre-spawn persist wrote the id, the validated result stops the compensating clear from firing, so only Execute observes the id surviving — a direct StartSession call never sees it.` with `The persist is caller-side (handleClaudeSession/Execute): the validated result makes StartSession return nil, so the post-exit persist runs and only Execute observes the id — a direct StartSession call never sees it.`
    12. Comment (line 465): replace `Same re-read/set-phase/write-back shape as the rollback Context (the child's frontmatter write must survive the clear), but the blob reports the turn's own failure, so the compensating clear still fires.` with `Same re-read/set-phase/write-back shape as the rollback Context (the child's frontmatter write must survive), but the blob reports the turn's own failure, so the turn is rejected and nothing is persisted.`
    13. Rename the `It` (line 487) `"clears the pre-persisted session id when the turn result reports its own failure"` → `"persists no session id when the turn result reports its own failure"`.
    14. Comment (line 514): replace `The compensating clear removed the id and this run's metrics entry, but the child's \`phase: planning\` write survived (the fixture seeded \`execution\`).` with `The turn failed, so nothing was ever persisted, and the child's \`phase: planning\` write survived (the fixture seeded \`execution\`).`
    - Do NOT touch the pinned-count strings anywhere in this file: `TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()`. The AC8 greps must keep returning their exact counts, so your reword must not add or remove any occurrence of those tokens.

12. **Confirm the goal AC7 test and the goal assertions are already correct — do not touch them.** `goal_workon_test.go`'s `"when persisting the goal session id after the child exits"` (line 358) already asserts `writeGoalAt.After(childExitAt)` (line 409) with both non-zero, and its `"goal work-on persists nothing for a failed turn"` context (line 650) already proves the no-persist half. Leave the file unmodified.

13. **Confirm the spec's Failure Modes rows that this prompt owns.**
    - Row 6 (claude binary missing): the `ErrStarterUnavailable` soft path is unchanged — `handleClaudeSession` returns `"", ErrStarterUnavailable` when the passed starter is nil and `Execute` downgrades it to a warning. Confirm both `workon.go` and `goal_workon.go` still reference it.
    - Row 7 (post-exit persist fails): covered by requirement 9 — `persistSessionAndMetrics` returns an error AND an empty id when the re-read or the write fails, so the task keeps whatever the turn wrote and no id lands.
    - Row 9 (two Start clicks): confirm NO double-start guard was added. It is a documented residual risk and a spec Non-goal. The spec 042 per-session lock already exists and is out of this prompt's scope.

14. **Self-check before finishing.** Re-read every changed hunk and walk spec 041 ACs 7, 8 and 9 against them, naming which artifact satisfies each. Run every command in `<verification>` and confirm each holds, including the two flips: `clearSessionAndMetrics` in `workon.go` 3 → 0, and `After(childExitAt)` in `workon_test.go` 0 → ≥ 1.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git (and `.git` is masked in this container, so no git command can run anyway). Do not write a `git` command anywhere in this prompt's execution: every invocation fails with `fatal: not a git repository`, and the daemon records a failed verification command as a pass.
- Interactive branch behaviour unchanged. `defaultCommandRunner`, the 5-minute TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched.
- No compensating clear: the id is never written before the turn, so `clearSessionAndMetrics` must stay deleted and no replacement may be added. On failure the task simply carries no id. Do not add a double-Start guard and do not add any config knob (both are spec Non-goals / Open Question 1).
- Persist-after-exit is race-free within the process: the child has already exited before the post-exit persist runs, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes.
- Use the `starter` PARAMETER in `handleClaudeSession`, never `w.starter` — the parameter is the launcher-resolved starter Execute passed in, and it is the value nil-checked earlier in the same function.
- Error idiom: `errors.Wrapf(ctx, err, ...)` / `errors.Wrap(ctx, err, ...)` / `errors.Errorf(ctx, ...)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- `ClaudeSessionStarter.StartSession`'s signature is UNCHANGED — six parameters, `mocks/claude-session-starter.go` untouched. `handleClaudeSession`'s `(string, error)` signature is UNCHANGED; the spec's three-value `return "", nil, errors.Wrap(...)` snippet is a typo and must NOT be used.
- The pinned-count tokens in `workon_session_writeback_test.go` (`TaskPhaseExecution`, `GoalPhaseExecution`, `session_note`, `MetricsSessions()`, `ClaudeSessionID()`) must remain byte-identical in count — requirement 11's reword is prose-only and must not touch an assertion.
- `goal_workon.go`, `goal_workon_test.go` and `topic_workon.go` are already in the target state — do not modify them except to read and confirm.
- Do NOT touch `pkg/ops/claude_session.go`, `pkg/ops/claude_session_test.go`, `pkg/ops/claude_session_detach_test.go`, or `pkg/ops/export_test.go` (prompt 1 owns them), and do NOT touch `docs/work-on-session-lifecycle.md`, `scenarios/002-task-lifecycle.md`, or `CHANGELOG.md` (prompt 3 owns them).
- Do NOT touch `agents/work-on-task-assistant.md` — the session-connect behaviour is background evidence for the reviewer, not an edit target.
- Do NOT edit `Makefile`, `scripts/*.sh`, or any test harness to work around the container proxy. Use the `env NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost` prefix on `make` commands instead.
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

SCENARIO-005 GUARD — content pin (nothing in this prompt may touch the file; the pin is repeated so a stray edit cannot pass unnoticed):

```
printf '%s  %s\n' '973840d5a8c6a55cb84c6db10c9c24ab2ff269b1ba0fa82e31ab1ac630ea0153' 'scenarios/005-work-on-resume-auto-invokes-subtask.md' | sha256sum -c -
```
must print `scenarios/005-work-on-resume-auto-invokes-subtask.md: OK`. If it prints FAILED, restore the file to its pre-prompt content — do NOT "fix" the pin.

SYNTAX + TESTS:
```
gofmt -e -l pkg/ops/workon.go pkg/ops/workon_test.go pkg/ops/workon_session_writeback_test.go   # must list NO files
env NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost make test                          # must exit 0
```

FULL GATE — `env NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost make precommit` at the repo root must exit 0. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make gosec`, `make errcheck`, ...), then `make precommit` once more. This is the batch's authoritative full-gate check (AC13), which prompt 3 runs last; running it here as well keeps the tree green for prompt 3, which builds on it.

A bare `make test` (without the `NO_PROXY` prefix) fails on `stub attention store never answered on http://127.0.0.1:<port>` because the container proxy intercepts loopback. That failure is an environment artifact and must not be reported as a code failure; always use the prefixed form above.
</verification>

<success-criteria>
- `clearSessionAndMetrics` is gone from `pkg/ops/` and `After(childExitAt)` appears in `workon_test.go`.
- The task path starts the turn before writing the id, matching `goal_workon.go`'s structure, and no failure path writes an id.
- The writeback test's pinned assertion counts are unchanged and all of its stale prose is reworded.
- `env NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost make test` exits 0 and `make precommit` (same prefix) exits 0.
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` still matches its pinned sha256.
- The completion report records the actual `make precommit` exit code and names which artifact satisfies each of spec 041 ACs 7, 8 and 9.
</success-criteria>
