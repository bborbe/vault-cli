---
spec: [041-bug-resume-races-live-headless-turn]
status: draft
created: "2026-09-11T12:10:00Z"
---

<summary>
- Rewords `docs/work-on-session-lifecycle.md` so the TASK path is described as post-exit, no-clear — the durable design record currently has the new section headers but its task-path prose still describes pre-spawn persistence + a compensating clear (the spec-040 design this spec reverses). All four task-path passages (Post-exit write ordering, The fate of --output-format json, Failure path, detached-child safety) are corrected to match the shipped code.
- Confirms the two AC11 liveness greps already read 0 (`livenessWindow` and "liveness window" prose are already gone from the doc) and stay 0, and that no `pre-spawn` / `compensating clear` mechanism wording remains in the doc's current-state descriptions.
- Confirms `scenarios/002-task-lifecycle.md` already carries the target wording ("Both branches block until the turn completes … the session id is written only once the turn has finished … `claude --resume <id>` works") and that `grep -c '~10s' scenarios/002-task-lifecycle.md` stays 0 — no change is needed there.
- Creates a `## Unreleased` section at the top of `CHANGELOG.md` with a `fix:` bullet describing the inversion (non-interactive task/goal work-on wait for the detached turn to exit before persisting `claude_session_id`, bounded by a 30-min turn timeout; TTY branch unchanged), satisfying AC12 (section exists + the bullet mentions Resume).
- Runs `make precommit` as the batch's full-gate check (AC13) — it must exit 0.
</summary>

<objective>
Make the durable documentation and changelog consistent with the spec-041 behavior shipped by prompts 1 and 2: the non-interactive task path waits for the detached turn before persisting `claude_session_id`, there is no compensating clear, and the "liveness window" concept is fully gone. This prompt covers spec-041 ACs 11-12 and the AC13 full gate, and depends on prompts 1 and 2 having shipped.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `docs/work-on-session-lifecycle.md` — the durable design record. It already has the NEW section headers (`## Post-exit write ordering` line 32, `## The fate of --output-format json` line 71, `## Failure path` line 135) but its TASK-PATH prose still describes pre-spawn persist + compensating clear (the spec-040 design this spec reverses). The sections you will reword: lines 32-48, 120-122, 135-148, and 199-205.
- `scenarios/002-task-lifecycle.md` — read the session-lifecycle note (line 33) to confirm it already matches the target wording.
- `CHANGELOG.md` — the top of the file (currently no `## Unreleased`; the newest versioned section is `## v0.130.3`).
- `pkg/ops/workon.go` and `pkg/ops/goal_workon.go` — the code the doc describes (after prompt 2, both persist post-exit with no clear). This is the source of truth for the doc's wording.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` rules, entry format (`- <prefix>: <what> [context]`), prefix semantics (`fix:` → patch bump).
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — completion and verification rules.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs.

NOTE: git IS available in this container (`workflow: direct`, no hideGit). The AC10 `git diff --exit-code HEAD -- scenarios/005-...` guard is re-verified here (it is also checked in prompt 1) as part of the final gate.
</context>

<requirements>
1. **Guard — prompt 2 must have shipped.** Before doing anything, confirm prompt 2's deliverables exist: `grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 0 AND `grep -c 'After(childExitAt)' pkg/ops/workon_test.go` >= 1. If EITHER fails, STOP — make no changes — and complete with `"status":"failed"` and message `"spec-041 prompt 3 precondition missing: prompt 2 not yet deployed (post-exit-persist)"`. Do not proceed.

2. **Reword the "Post-exit write ordering" section of `docs/work-on-session-lifecycle.md` (lines 32-48) so BOTH paths persist after the turn.** The current prose (lines 34-41) says the task path persists `before the child is spawned` with a compensating clear, and (line 43) that only the goal path uses post-exit ordering. Rewrite lines 34-48 so it reads, in substance: on the **task path** the fresh id and its `metrics_sessions` entry are persisted only **after the child exits** — `persistSessionAndMetrics` runs only once `StartSession` has returned cleanly, which happens only after the detached headless turn completes and its JSON validates; the **goal path** uses the same post-exit ordering. Replace the paragraph at lines 46-48 (`On the task path the pre-spawn re-read before writing is load-bearing: ...`) with: `On both paths the re-read before writing is load-bearing: the task file is a shared, concurrently-written vault file (the headless turn mutates it too), so writing the stale in-memory copy would revert those changes.` Delete the sentence at line 40 about "a re-read-based compensating clear" (the clear no longer exists) and the sentence at lines 37-39 about the child's session-connect needing the field pre-set (it no longer does — the id is written only after the child exits). Keep the section's title `## Post-exit write ordering` unchanged.

3. **Reword the "The fate of --output-format json" section's tail (lines 120-122).** The sentence `The compensating clear is unchanged: it still fires on every error the detached turn returns. Only the definition of "failed" moved.` is now wrong — there is no compensating clear anywhere. Replace it with, in substance: `On any error — exit non-zero with no usable result, is_error, zero turns, the turn bound, or ctx cancellation — StartSession returns an error and the caller persists nothing; there is no compensating clear because the id was never written. Only the definition of "failed" moved.` Keep the rest of the section (lines 71-119, 124-133) intact.

4. **Reword the "## Failure path" section (lines 135-148) so both paths persist nothing on failure.** The current first paragraph (lines 137-143) describes the task path's pre-persist + compensating clear. Replace it with, in substance: `On **both paths** nothing is persisted for a failed turn: StartSession returns an error, so the id and this run's metrics entry never land on disk. Frontmatter the child wrote before failing stays untouched (for example phase: planning), and the Vault UI correctly keeps offering Start.` Keep the second paragraph (lines 145-148, the persist-step failure → empty id rule) unchanged — it is still accurate. Also reword the intro sentence if it references the clear.

5. **Reword the detached-child safety paragraph (lines 197-205) so it no longer references the task path's pre-persisted id or compensating clear.** The sentences `On the task path the id is pre-persisted, and the safety argument is layered: ... on any failure the compensating clear removes the id, so it cannot stay resumable-looking. The goal path keeps its post-exit ordering, so there the id is only on disk once the child has exited.` must become, in substance: `On both paths the id is only on disk once the child has exited, so a running child never has a resumable-looking id on the task. During the running window Resume is not offered for a live turn (the Vault UI resolver fix, shipped separately) and the per-session lock (spec 042) refuses a second writer on the same id, so the child running unlocked is not targetable.` Keep the rest of the paragraph and the `## The per-session lock` section (lines 150-195) unchanged — in particular the lock-section sentence at line 171 about "no cleanup sweeps, no compensating clears" refers to the LOCK and is accurate; do not touch it.

6. **Confirm the liveness reword is complete.** After requirements 2-5, verify: `grep -c 'livenessWindow' docs/work-on-session-lifecycle.md` == 0, `grep -ci 'liveness window' docs/work-on-session-lifecycle.md` == 0, and `grep -ci 'pre-spawn' docs/work-on-session-lifecycle.md` == 0. These first two greps already read 0 before your edits (the reword happened in an earlier pass); your job is to keep them 0 and remove the remaining `pre-spawn` / mechanism references to the compensating clear in the current-state descriptions. The historical sentence in the intro (line 8: "Spec 040 originally had the non-interactive branch return within ~10 seconds") describes spec 040 as history and stays.

7. **Confirm `scenarios/002-task-lifecycle.md` is already at the target — do NOT change it.** The session-lifecycle note (line 33) already reads "Both branches block until the turn completes — expect no output for the whole bootstrap (typically 2-5 minutes; bounded by a 30m turn timeout) ... A fast return is a FAIL, not a pass: the session id is written only once the turn has finished, which is what makes `claude --resume <id>` work." This is the spec-041 target wording, and `grep -c '~10s' scenarios/002-task-lifecycle.md` already reads 0. Confirm, make no edits to the file.

8. **Create `## Unreleased` in `CHANGELOG.md` with the inversion bullet (AC12).** The file currently has no `## Unreleased` (the newest versioned section is `## v0.130.3`). Insert a new section at the top of the versioned list — immediately after the intro block (which ends at the blank line before `## v0.130.3`) — reading:
   ```markdown
   ## Unreleased

   - fix: non-interactive `task work-on` / `goal work-on` now wait for the detached headless turn to exit before persisting `claude_session_id` (bounded by a 30-min turn timeout), so the Vault UI offers Resume only against a complete, single-writer transcript; the TTY branch is unchanged
   ```
   Follow `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` for the entry format. The bullet must mention Resume (case-insensitive) so the AC12 evidence grep matches. Do NOT bump the four version strings and do NOT hand-edit `.claude-plugin/` manifests — this repo is `autoRelease: true` and the github-releaser owns version bumps + tags; `## Unreleased` is exactly the input it converts.

9. **Run the final gate.** After the doc + changelog edits, run the `<verification>` block. The `make precommit` step is the batch's full-gate check (AC13) and must exit 0. If it fails, fix the failure (re-run only the failing target first), then re-run `make precommit` once everything passes.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff --exit-code HEAD` reads only; do not stage or commit anything.
- No Go code changes in this prompt. `pkg/`, `mocks/`, `scenarios/005-work-on-resume-auto-invokes-subtask.md`, and the `## v0.130.3` and older CHANGELOG entries are all untouched.
- The doc reword must match the shipped code: post-exit persist on both paths, no compensating clear, `sessionTurnTimeout` (30m, a wait bound, never a kill). Do not reintroduce "liveness window", "pre-spawn", or a compensating-clear mechanism in any current-state description. Historical references to spec 040's ~10s behavior in the intro are fine.
- Do NOT change `scenarios/002-task-lifecycle.md` — it is already at the target wording.
- Do NOT bump version strings or edit `.claude-plugin/` manifests (autoRelease: true — the github-releaser owns releases; `## Unreleased` is its input).
- CHANGELOG entry format per the changelog-guide: `- <prefix>: <what> [context]`, prefix required, `fix:` prefix for this bug fix.
- Existing tests must still pass (`make precommit` exit 0).
</constraints>

<verification>
PRIMARY GATE — spec evidence greps. Run each and record the count:

```
# AC11 docs/scenario reword:
grep -c 'livenessWindow' docs/work-on-session-lifecycle.md                    # == 0
grep -ci 'liveness window' docs/work-on-session-lifecycle.md                  # == 0 (prose form too)
grep -ci 'pre-spawn' docs/work-on-session-lifecycle.md                        # == 0 (reworded current-state sections)
grep -c '~10s' scenarios/002-task-lifecycle.md                                # == 0
# AC12 CHANGELOG (Unreleased must exist — v0.116.6 consumed it — and carry the bullet):
grep -c '^## Unreleased' CHANGELOG.md                                         # >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'                   # >= 1
```

SECONDARY — AC10 git guard (git IS available — workflow `direct`, no hideGit):
```
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md   # must exit 0 with empty output
```

FULL GATE — AC13:
```
make precommit    # must exit 0
```

`make precommit` runs the full lint + test + check suite and is the batch's final gate. If it fails, fix and re-run only the failing target first (`make lint`, `make gosec`, `make errcheck`, ...), then re-run `make precommit` once all individual targets pass.
</verification>
