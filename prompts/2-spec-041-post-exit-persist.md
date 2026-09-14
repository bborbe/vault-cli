---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-14T20:23:00Z"
---

# Task path: persist the session id only after the headless turn exits (spec 041, prompt 2 of 3)

<summary>
- Re-applies the spec-041 start-then-persist ordering to the task path: the headless turn is started first, and `claude_session_id` plus its metrics entry are written only after that turn exits cleanly.
- Deletes the now-dead compensating-clear helper from the task path — with nothing pre-written, a failure has nothing to undo, so the task simply carries no id.
- Reworks the task-path tests: the "persisted before the spawn" test becomes "persisted after the child exits" and asserts the write timestamp is later than the child's exit, the write-vs-spawn sequencing assertion inverts, and the clear-based failure tests are replaced by one test proving a failed turn persists nothing.
- Rewords the stale pre-spawn comments and section titles in the task tests and in the session write-back tests so they describe the post-exit, no-clear ordering; no assertion in the write-back file changes.
- Confirms the goal path and its tests are already in the target state and leaves them untouched.
- WARNING for the human reviewer: the task-side half of this spec was deliberately REVERTED in the tree after approval, because persisting after the exit caused a live regression. This prompt implements the spec AS APPROVED and re-introduces that ordering. The conflict, the evidence, the later change that may neutralise it, and the two adjudication options are in the reviewer block at the top of `<requirements>`. Prompt 3 is coupled to that decision.
- A later spec (045) was written against the reverted ordering and explicitly preserved the compensating clear on the task path; the reviewer block records that too, because it is strong evidence that the reversion is the live intent.
- Runs the spec-041 AC7-9 evidence gate plus `make precommit`.
</summary>

<objective>
Make the task path of `work-on` persist `claude_session_id` only after the detached headless turn has finished, so no failure leaves a resumable-looking id behind — matching the goal path, which already ships this behaviour. This prompt covers spec 041 Acceptance Criteria 7-9 and depends on prompt 1 having shipped (its block-until-exit `StartSession` is what these tests exercise).
</objective>

<context>
Read `CLAUDE.md` for project conventions, then read these files in full, in this order:

- `pkg/ops/workon.go` — the whole file. The parts this prompt rewrites are `handleClaudeSession` (the fresh-start block), `persistSessionAndMetrics`'s doc comment, `clearSessionAndMetrics` (to be deleted), and `sessionFailureResult`'s doc comment.
- `pkg/ops/goal_workon.go` — the whole file. This is the structural TEMPLATE the reordered task path must match; its `handleClaudeSession` already starts-then-persists on both branches and carries the doc comments to mirror.
- `pkg/ops/workon_test.go` — the whole file. The contexts this prompt reworks: the `"success"` block's comment-era specs, `"when the session id write precedes the spawn"`, `"when the pre-spawn persist re-read fails"`, `"when persisting the session id before spawning"`, and `"when the spawn fails"` (with its nested compensating-clear context).
- `pkg/ops/goal_workon_test.go` — the whole file. The `"when persisting the goal session id after the child exits"` context is already the target shape; confirm it, do not touch it.
- `pkg/ops/workon_session_writeback_test.go` — the whole file. Comment and title rewords only; its assertions are load-bearing and must not change.
- `docs/work-on-session-lifecycle.md` — read it for the design intent. Do NOT edit it; prompt 3 owns it.
- `specs/in-progress/041-bug-resume-races-live-headless-turn.md` — the spec. Read the whole Design section for `pkg/ops/workon.go`, plus Constraints, Acceptance Criteria 7-9, and Failure Modes.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `github.com/bborbe/errors` idiom (`errors.Wrap(ctx, err, …)`); no `fmt.Errorf`, no bare `return err`, no `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions and counterfeiter mock usage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage rules for changed code.

Environment notes:
- `git` is unavailable in this container (`/workspace/.git` is a character device; every git call fails with `fatal: not a git repository`). Do not run git. This prompt needs no git command.
- Prompt 1 must have shipped before this one runs — its guard is requirement 1.
</context>

<requirements>

<!-- ⚠️ HUMAN REVIEWER — TREE CONFLICT WITH THIS SPEC. READ BEFORE APPROVING. ⚠️

The task-side half of spec 041 was REVERTED in the tree AFTER this spec was approved.

1. What the spec asks. Spec 041's Goal is "no claude_session_id on disk while a detached child exists".
   Its Design applies that to BOTH paths: workon.go reorders to start-then-persist and deletes
   clearSessionAndMetrics.

2. What the tree says. `pkg/ops/goal_workon.go` already ships start-then-persist (never reverted).
   `pkg/ops/workon.go` does NOT: it pre-persists the id before the spawn and runs a compensating clear
   on failure. That is not drift — it is a deliberate, released reversion. The CHANGELOG's v0.118.3
   entry documents it, verbatim: "`task work-on` now persists the fresh `claude_session_id` and its
   `metrics_sessions` entry to the task file before the headless Claude session is spawned, so the
   session's own `/vault-cli:work-on-task` session-connect reads the field already set and keeps the
   fresh session instead of scanning the transcript directory and attaching whichever transcript was
   most recently modified ... a failed spawn now triggers a re-read-based compensating clear".
   What the reversion KEPT: claude_session.go's block-until-exit + shared validator (spec 041,
   confirmed by prompt 1), the spec-042 per-session lock, and the vault-ui-side "gate Resume on live
   cards" resolver.

3. A LATER spec was authored against the reverted design. Spec 045
   (`specs/in-progress/045-bug-exit-code-outranks-validated-turn.md`; its prompts 208/209 are
   completed) states in its Constraints, on the task path: "Removing or weakening the compensating
   clear. This spec narrows *when* a turn counts as failed; a genuinely failed turn must still clear
   the id." and "`handleClaudeSession`'s compensating clear continues to fire on every error
   `runDetachedTurn` returns". Spec 045's acceptance criteria require the clear to fire on both paths.

4. BUT the mechanism the reversion cited may no longer exist. The v0.118.3 rationale names the mtime
   scan: the child's session-connect used to attach "whichever transcript was most recently
   modified". That scan is now explicitly forbidden. `agents/work-on-task-assistant.md` (step 2 of
   session-connect) reads: "detect the current session's UUID by title-match, never by
   newest-transcript. The `ls -t ... | head -1` mtime scan is forbidden — in a fleet of concurrent
   sessions the newest transcript is almost never the current session (observed: a fresh headless
   Start session got bound to a live unrelated session this way)." `commands/work-on-task.md` agrees:
   title-match only, and "A title-match miss leaves the field empty ... that is safe only on the
   headless Start path, which pre-sets the field via vault-cli".
   The reviewer must weigh this: the unrelated-session-attach regression that forced the reversion may
   no longer be reachable. Note the residual either way — under start-then-persist the field IS empty
   during the turn, so the child's session-connect will run; on a title-match hit it writes its OWN
   session id mid-turn (the correct id, but on disk before the turn ends, which is the window spec 041
   exists to close — gated today by the vault-ui resolver, shipped separately, and by spec 042's lock,
   which the Vault UI's direct `claude --resume` does not take).

5. The doc currently carries BOTH orderings: the heading `## Post-exit write ordering` (spec 041) over
   a body describing the pre-spawn persist and the compensating clear (the reversion), and
   `## What the turn timeout does and does not cover` still says "The compensating clear is
   unchanged: it still fires on every error the detached turn returns." Prompt 3 rewrites those
   bodies.

The current tree therefore FAILS spec 041 AC7 (`grep -c 'After(childExitAt)' pkg/ops/workon_test.go`
== 0) and AC9 (`grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 3).

This prompt implements the spec AS APPROVED: it re-applies start-then-persist to workon.go and deletes
clearSessionAndMetrics, making the task path match the goal path. No separate mechanism that would
neutralise the v0.118.3 regression is named by this spec.

The reviewer must decide at audit time between:

  (A) Approve this prompt — spec 041 wins; the reorder is re-applied, spec 045's "compensating clear is
      not weakened" constraint is superseded on the task path, and any residual session-connect risk
      is accepted and owned as a follow-up. Prompt 3 must be approved with it (prompt 3 rewrites the
      doc and the changelog to post-exit).
  (B) Reject and re-scope spec 041 — treat the v0.118.3 ordering as the new target. AC7 and AC9 would
      need rewording to assert the pre-spawn persist plus the compensating clear, which the current
      tree already satisfies; this prompt would be dropped, and prompt 3's doc reword and changelog
      bullet would be dropped with it (the doc's current bodies and the v0.118.3 release note already
      describe the reverted behaviour).

Second-order effect either way: re-applying the reorder makes the changelog history self-contradictory
(v0.118.3 documents the opposite ordering), and it invalidates the prose — though not the assertions —
of the two spec-045 write-back specs, which requirement 11 reworks.

For the executing agent: implement the requirements below as written. The spec is the source of truth
for this batch; the reviewer adjudicates the conflict at audit time. Do not "fix" the requirements to
preserve the reversion, and do not add any mechanism the spec does not name.
-->

## 1. Guard — prompt 1 must have shipped

Before changing anything, confirm prompt 1's deliverables exist:

```
test "$(grep -c 'validateSessionTurn' pkg/ops/claude_session.go)" -ge 2
test "$(grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go)" -ge 1
test "$(grep -c 'Expect(capturedWindow).To(Equal(ops.SessionTurnTimeout))' pkg/ops/claude_session_test.go)" = "1"
test "$(grep -c 'removes the temp output file after a clean exit' pkg/ops/claude_session_test.go)" = "1"
```

If any of these fails, STOP and report `"status":"failed"` with the message `"spec-041 prompt 2 precondition missing: prompt 1 not yet deployed"`. Do not proceed and do not re-implement prompt 1's work here.

## 2. Reorder the fresh-start path in `handleClaudeSession` to start-then-persist

In `pkg/ops/workon.go`, the fresh-start block of `handleClaudeSession` currently persists BEFORE the spawn and compensates on failure. Replace the block that begins at the `prompt := fmt.Sprintf(...)` line and ends at `return sessionID, nil` with exactly:

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

Delete entirely: the `// Persist id + metrics BEFORE the child exists, …` comment block, the `if _, err := persistSessionAndMetrics(…); err != nil { return "", errors.Wrap(ctx, err, "persist claude session before spawn") }` call, and the compensating-clear body inside the `if err := w.starter.StartSession(…)` block (including its `slog.Warn("workon warning", …)` line).

Two notes that matter for correctness:

- The function's signature stays `(string, error)`. Spec 041's Design section prints `return "", nil, errors.Wrap(ctx, err, "start claude session")` — that three-value form is a typo in the spec and does not compile against the real signature. The two-value form above is the one to write; it is exactly `goal_workon.go`'s shape.
- `sessionID, err := persistSessionAndMetrics(…)` compiles because `sessionID` is already declared above in the same scope and `err` is newly introduced by that short declaration. This mirrors `goal_workon.go`'s non-interactive branch line for line. Do not restructure it into two statements.

The cached-session path (the `if existing := task.ClaudeSessionID(); existing != ""` branch) is UNCHANGED. The `if w.starter == nil { return "", ErrStarterUnavailable }` soft path is UNCHANGED.

## 3. Delete the dead `clearSessionAndMetrics`

Remove the whole function — its doc comment and body — from `pkg/ops/workon.go`. Its only call site was the compensating-clear block deleted in requirement 2. Add no replacement and no deprecation stub. After this, `clearSessionAndMetrics` must not appear anywhere under `pkg/`.

## 4. Reword the now-stale doc comments

Three comments in `pkg/ops/workon.go` and one in `pkg/ops/claude_session.go` assert the ordering this prompt inverts. Reword them to the post-exit, no-clear semantics — mirror `pkg/ops/goal_workon.go`'s wording where it covers the same point.

- `persistSessionAndMetrics`'s comment: replace the sentence
  `Used pre-spawn on the fresh-start path (the session id is new and must be on disk before the child exists) and on the cached-session path (the id already exists and is preserved).`
  with
  `Used post-exit on the fresh-start path (the session id is new and is persisted only after the headless turn completes cleanly) and on the cached-session path (the id already exists and is preserved).`
  Keep the rest of the comment — the load-bearing re-read rationale and the "returns an empty id on failure" paragraph — byte-identical.
- `handleClaudeSession`'s comment: replace its second sentence (the `On the fresh-start path … an id on disk means a resumable session.` run) with the goal path's formulation, adapted to the task path:
  `On both branches the session id is persisted only AFTER the headless turn has finished cleanly, so an id on disk means the session is resumable rather than merely that one was started. Nothing is written on any failure path, so there is no compensating clear: frontmatter the child wrote before failing stays untouched, and the Vault UI correctly keeps offering Start.`
  Keep the cached-session sentence that follows it.
- `sessionFailureResult`'s comment: `the accumulated warnings (including any compensating-clear warning)` → `the accumulated warnings`. There is no clear warning any more.
- `pkg/ops/claude_session.go` — the ONE edit this prompt may make to that file, and it is a comment only. `ClaudeSessionStarter.StartSession`'s doc comment currently ends its first paragraph with `(not by claude) so it can be persisted before the child process exists.` That reason is now wrong. Replace those three lines with:
  `// StartSession runs claude in headless mode to create a session with the given`
  `// session id and returns only an error. The session id is minted by the caller`
  `// (not by claude) so it can be passed to the child with --session-id and`
  `// correlated with the transcript the child writes.`
  Change nothing else in `claude_session.go` — prompt 1 owns that file and its verification greps are pinned against it.

## 5. Confirm the goal path is already in the target state — do NOT change it

`pkg/ops/goal_workon.go`'s `handleClaudeSession` must already start-then-persist on both branches (its interactive branch and its non-interactive branch), return `errors.Wrap(ctx, err, "start claude session")` with no compensating clear, and keep the cached path as `return existing, nil`. `persistGoalSessionID` must already return an empty id on failure. Confirm by reading it; if it matches, leave it untouched. `clearGoalSession` must not exist anywhere (`grep -c 'clearGoalSession' pkg/ops/goal_workon.go` == 0 already).

`pkg/ops/goal_workon_test.go`'s `"when persisting the goal session id after the child exits"` context already asserts `writeGoalAt.After(childExitAt)`. Confirm and leave it alone.

## 6. Rework AC7 in `workon_test.go`

In the `Context("when persisting the session id before spawning", …)` block:

- Rename the context to `"when persisting the session id after the child exits"`.
- In the variable block, rename `spawnAt time.Time` to `childExitAt time.Time`.
- In the `detachRun` fake, rename the assignment `spawnAt = time.Now()` to `childExitAt = time.Now()` and keep it immediately before `done <- nil` — the timestamp must mark the moment the child's exit is signalled, not the moment the child was spawned. Keep the existing argv capture of `spawnedSessionID`, the valid-JSON write to `stdout`, the `Expect(err).To(BeNil())` on that write, and the blocking waiter exactly as they are.
- Rename the `It` to `"writes the session id to storage only after the child exits"` and replace its body with:

```go
			Expect(err).To(BeNil())
			Expect(writeTaskAt).NotTo(BeZero())
			Expect(childExitAt).NotTo(BeZero())
			Expect(writeTaskAt.After(childExitAt)).To(BeTrue())
			Expect(writtenSessionID).To(Equal(spawnedSessionID))
```

Keep the existing AC5 comment above the last assertion ("capture the id written to storage and the id handed to detachRun and assert they are the same value…"). This yields the AC7 evidence `After(childExitAt)` in `workon_test.go`, which is absent today.

## 7. Invert the write-vs-spawn sequencing test

In the `Context("when the session id write precedes the spawn", …)` block:

- Rename the context to `"when the session id write follows the spawn"`.
- Rename the `It` to `"writes the session id to storage after StartSession returns"`.
- Change the final assertion from `Expect(writeSeq).To(BeNumerically("<", startSeq))` to `Expect(writeSeq).To(BeNumerically(">", startSeq))`.

The `WriteTaskStub` / `StartSessionStub` sequencing setup and the two `NotTo(Equal(0))` guards stay as they are.

## 8. Replace the clear-based failure tests with a persists-nothing test

In the `Context("when the spawn fails", …)` block:

- Keep `"returns the wrapped spawn error"` and `"returns Success=false"` unchanged.
- DELETE the `It("clears the pre-persisted session id and the metrics entry for the failed run", …)` spec and the nested `Context("when the compensating clear itself fails", …)` entirely — the clear no longer exists, so neither spec describes anything.
- ADD this spec to the context:

```go
		It("persists no session id when the spawn fails", func() {
			// Execute's write only — nothing is written for this id before the turn
			// runs and nothing is written after it fails, so a failed turn leaves no
			// id and no metrics entry.
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			Expect(writtenIDs).To(Equal([]string{""}))
			Expect(writtenMetricsSessions[0]).To(BeEmpty())
		})
```

- Reword the `BeforeEach` comment that currently explains the compensating-clear pointer mutation. The `WriteTaskStub` stays (it snapshots each write's state as it lands, because the mock returns the same `*Task` pointer for every `FindTaskByName`); the comment must now read as a snapshot of what each write lands — Execute's write carries an empty id because the id is minted inside `handleClaudeSession` after `Execute` writes the task, and no later write happens on this path.

## 9. Rework the pre-spawn re-read failure context

In the `Context("when the pre-spawn persist re-read fails", …)` block:

- Rename the context to `"when the post-exit persist re-read fails"`.
- Rename the `It` `"does not append a metrics entry when the pre-spawn re-read fails"` to `"does not append a metrics entry when the post-exit re-read fails"`.
- Reword the `BeforeEach` comment from `The failing call is persistSessionAndMetrics' PRE-SPAWN re-read, which runs before StartSession is ever called.` to `The failing call is persistSessionAndMetrics' POST-EXIT re-read, which runs after StartSession returns (the mock starter returns nil immediately).`
- Reword the comment on the write-count `It` from `pre-spawn persist failed before writing` to `post-exit persist failed before writing`.

All four `It` bodies stay byte-identical. The mock call indexes do not change: call 0 loads the task, `mockStarter.StartSessionReturns(nil)` makes no storage call, and call 1 is the post-exit re-read that fails.

## 10. Reword the comment-era specs in the `"success"` block

These specs drive the mock starter; their assertions and call counts are unchanged. Only comments and one title change:

- `It("calls FindTaskByName")`'s comment `Twice: once to load the task, once to re-read it before the child is spawned so the fresh session id lands on disk before the child exists.` → `Twice: once to load the task, once to re-read it after the child exits so the post-exit persist lands the fresh session id without reverting the child's frontmatter writes.`
- `It("re-reads the task from the vault path before spawning the session")` → rename to `It("re-reads the task from the vault path after the child exits")` and reword its comment to `The second FindTaskByName is persistSessionAndMetrics' post-exit re-read: the session id is written to disk only after the child has exited.`
- `It("Fresh run records one entry")`'s comment `The metrics entry lands in the pre-spawn persist write: write 0 is Execute's status/assignee/phase write, write 1 is the pre-spawn persist.` → `The metrics entry lands in the post-exit persist write: write 0 is Execute's status/assignee/phase write, write 1 is the post-exit persist.`

After requirements 6-10, `pkg/ops/workon_test.go` must contain none of the words `pre-spawn`, `pre-persisted`, `compensating`, or the phrase `before the child is spawned` — including inside a negation. Say what the code does now instead of naming what it no longer does.

## 11. Reword `workon_session_writeback_test.go` to post-exit semantics

The fakes in this file ALREADY have the shape the reordered code needs: they write a valid JSON line to the stdout `*os.File` and exit cleanly via `done <- nil` with a blocking waiter. Confirm that; do NOT change it. Only comments and titles change, and **no assertion may change**:

- Task context `BeforeEach` comment → `Simulate the real headless turn: work-on spawns the child first, then the Claude session runs plan-task -> execute-task and writes its own frontmatter on top of that file inside the detached child (the detachRun fake). Only after the child exits does work-on re-read and persist the session id, so the child's frontmatter survives.`
- Task context, the metrics comment inside the `It` (`The metrics entry lands in the pre-spawn persist write (real storage round-trip) and survives because nothing writes to the file after the child's own write.`) → `The metrics entry lands in the post-exit persist write (real storage round-trip) and survives because nothing writes to the file after the child's own write.`
- Goal context `BeforeEach` comment: the tail `…while the parent has already returned within the liveness window.` → `…while the parent blocks waiting for the detached turn.`
- Context `"when the child exits non-zero inside the liveness window"` → `"when the child exits non-zero within the turn wait"`; its `It` title `"clears the pre-persisted session id and preserves the child's frontmatter write when the child exited non-zero inside the window"` → `"persists no session id and preserves the child's frontmatter write when the child exited non-zero within the turn wait"`; the comment `The liveness window has NOT elapsed when the child exits, so the starter must treat the exit as inside-the-window.` → `The turn wait has NOT elapsed when the child exits, so the child-exit branch of the select wins.`
- Same context, the `It`'s lead comment (`The rollback is caller-side (handleClaudeSession/Execute), so it must be driven through Execute — a direct StartSession call never observes it.`) → `The failure path is caller-side (handleClaudeSession/Execute), so it must be driven through Execute — a direct StartSession call never observes it.`
- Same context, the mechanism comment (`The child's write survived: the pre-spawn persist wrote the id and this run's metrics entry, … then the compensating clear re-read the file …`) → a post-exit description: the turn failed, so nothing was persisted, and the child's `phase: planning` write is simply what remains on disk.
- Spec-045 context `"when the child exits non-zero after writing a valid turn result"`: its `It` title `"retains the pre-persisted session id when the turn result validated despite the non-zero exit"` → `"persists the session id when the turn result validated despite the non-zero exit"`; reword the `The retain is caller-side …` comment to describe the post-exit persist (the validated result makes `StartSession` return nil, then the persist runs) instead of a write that a clear would have undone.
- Spec-045 context `"when the child reports its own failure"`: its `It` title `"clears the pre-persisted session id when the turn result reports its own failure"` → `"persists no session id when the turn result reports its own failure"`; reword its two mechanism comments (the `BeforeEach` one — `Same re-read/set-phase/write-back shape as the rollback Context (the child's frontmatter write must survive the clear), but the blob reports the turn's own failure, so the compensating clear still fires.` — and the one before the raw-file assertions) to the post-exit no-clear description: the failure path writes nothing at all, so the child's frontmatter survives simply because nothing overwrote it.
- Do NOT touch the pinned-count strings anywhere in this file: `TaskPhaseExecution` (== 2), `GoalPhaseExecution` (== 2), `session_note` (== 4), `MetricsSessions()` (== 2), `ClaudeSessionID()` (== 2). Those greps are spec 041's AC8 evidence and must stay byte-identical. After this reword the file must contain none of the words `liveness`, `pre-spawn`, `pre-persisted` or `compensating` — say what the code does now, not what it no longer does.

## 12. Confirm the failure-mode rows this prompt owns

- Row 6 (claude binary missing): the `ErrStarterUnavailable` soft path is unchanged — `handleClaudeSession` returns `"", ErrStarterUnavailable` when `w.starter == nil`, and `Execute` downgrades it to a warning. Confirm the sentinel is still referenced in both `pkg/ops/workon.go` and `pkg/ops/goal_workon.go`.
- Row 7 (post-exit persist fails): covered by requirement 9's reworked context — `persistSessionAndMetrics` returns an error AND an empty id when the re-read or write fails; the task keeps whatever the turn wrote; no id lands.
- Row 9 (two Start clicks on one task): confirm this prompt added NO double-start guard (spec Non-goal, documented residual risk). The spec-042 per-session lock is already landed and is out of scope. Do not add a guard, a flag, or a config knob.

## 13. Self-check

Re-read every changed hunk and walk ACs 7, 8 and 9 against it. Then run the whole `<verification>` block and confirm every check passes, including the two flips: `clearSessionAndMetrics` in `workon.go` 3 → 0, and `After(childExitAt)` in `workon_test.go` 0 → 1. In the completion report, name which spec or assertion satisfies each of AC7, AC8 and AC9.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. (This container has no usable git; make no git calls.)
- **Interactive branch behaviour unchanged.** `defaultCommandRunner`, the 5m TTY cap, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` are untouched.
- **No compensating clear.** The id is never pre-written, so `clearSessionAndMetrics` must stay deleted. On failure the task simply carries no id.
- **Persist-after-exit is race-free.** The child has already exited before the post-exit persist, so there is no concurrent writer; the re-read-modify-write preserves the child's frontmatter writes (the write-back invariant). That ordering is what `goal_workon.go` already ships.
- **Error idiom.** `errors.Wrapf(ctx, err, …)` / `errors.Wrap(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- **The `ClaudeSessionStarter` interface signature is UNCHANGED** — `mocks/claude-session-starter.go` is untouched. `handleClaudeSession`'s `(string, error)` signature is UNCHANGED — spec 041's Design prints a three-value `return "", nil, errors.Wrap(…)` and that is a spec typo; do not use it.
- **Do NOT revert spec 045.** The child-exit verdict precedence and the current predicate wording in `pkg/ops/claude_session.go` belong to spec 045, which shipped later. The only edit permitted in that file is the interface doc comment named in requirement 4.
- Do NOT add a double-Start guard, and do NOT add any config knob (both are spec Non-goals / Open Question 1).
- The AC8 pinned-count strings in `workon_session_writeback_test.go` must remain byte-identical — requirement 11's reword must not touch any assertion.
- `goal_workon.go` and `goal_workon_test.go` are already in the spec-041 target state — do not modify them.
- Do NOT touch `docs/work-on-session-lifecycle.md` (prompt 3 owns it), `CHANGELOG.md`, or anything under `scenarios/`.
- Tests use Ginkgo v2 + Gomega with counterfeiter mocks — no stdlib `t.Run` table tests.
- Existing tests must still pass.
</constraints>

<verification>
Run everything from the repo root.

**Full gate — `make precommit` must exit 0.** If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make gosec`, `make test`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each of these must exit 0. They are self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0 and the daemon does not check `<verification>` exit codes, so the comment-only form reports success on unchanged code.

```
test "$(grep -c 'clearSessionAndMetrics' pkg/ops/workon.go)" = "0"
test "$(grep -c 'clearGoalSession' pkg/ops/goal_workon.go)" = "0"
! grep -rq 'clearSessionAndMetrics' pkg/
! grep -rq 'clearGoalSession' pkg/
test "$(grep -c 'After(childExitAt)' pkg/ops/workon_test.go)" -ge 1
test "$(grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go)" -ge 1
test "$(grep -c 'writtenSessionID' pkg/ops/workon_test.go)" -ge 1
test "$(grep -c 'spawnedSessionID' pkg/ops/workon_test.go)" -ge 1
test "$(grep -c 'persists no session id when the spawn fails' pkg/ops/workon_test.go)" = "1"
test "$(grep -c 'when persisting the session id after the child exits' pkg/ops/workon_test.go)" = "1"
test "$(grep -c 'persist claude session before spawn' pkg/ops/workon.go)" = "0"
! grep -Eqi 'pre-spawn|pre-persisted|before the child is spawned' pkg/ops/workon.go
! grep -q 'compensating-clear warning' pkg/ops/workon.go
! grep -Eq 'pre-spawn|pre-persisted|compensating|before the child is spawned' pkg/ops/workon_test.go
! grep -Eq 'pre-spawn|pre-persisted|compensating|liveness' pkg/ops/workon_session_writeback_test.go
test "$(grep -ci 'no compensating clear' pkg/ops/workon.go)" -ge 1
test "$(grep -c 'ErrStarterUnavailable' pkg/ops/workon.go)" -ge 1
test "$(grep -c 'ErrStarterUnavailable' pkg/ops/goal_workon.go)" -ge 1
```

AC8 write-back invariant counts — these must hold EXACTLY, so a reword that deletes or duplicates an assertion fails here:

```
test "$(grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go)" = "2"
test "$(grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go)" = "2"
test "$(grep -c 'session_note' pkg/ops/workon_session_writeback_test.go)" = "4"
test "$(grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go)" = "2"
test "$(grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go)" = "2"
```

Prompt 1's session-layer invariants must still hold after your edit to `claude_session.go`'s interface comment:

```
test "$(grep -c 'defaultCommandRunner' pkg/ops/claude_session.go)" = "3"
test "$(grep -c 'context.WithTimeout' pkg/ops/claude_session.go)" = "1"
test "$(grep -c 'claude session exited with error' pkg/ops/claude_session.go)" = "1"
test "$(grep -c 'validateSessionTurn' pkg/ops/claude_session.go)" -ge 2
! grep -q 'fmt.Errorf' pkg/ops/workon.go
```

Syntax and suite:

```
gofmt -e -l pkg/ops/workon.go pkg/ops/workon_test.go pkg/ops/workon_session_writeback_test.go pkg/ops/claude_session.go
```
must list NO files.

```
go test ./pkg/ops/...
```
must pass (unpiped — never pipe a test command, the pipeline would report the last stage's status).
</verification>

<!-- DARK-FACTORY-REPORT -->
