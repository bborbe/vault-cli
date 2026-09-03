---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-03T10:10:00Z"
---

<summary>
- Rewords the task-path sections of `docs/work-on-session-lifecycle.md` back to the spec-041 post-exit ordering: "Post-exit write ordering", "Failure path", and the per-session lock's detached-child safety paragraph describe BOTH paths persisting only after the turn, with no compensating clear — the v0.118.3 reversion rewrote the bodies to pre-spawn + compensating clear while leaving the spec-041 headings, and this prompt removes that stale content.
- Confirms `scenarios/002-task-lifecycle.md` already says the headless turn blocks until completion (no "~10s" return claim) — AC11.
- Appends the spec-041 bullet under `## Unreleased` in `CHANGELOG.md` (AC12). The section exists today (carrying the concurrent spec-044 resolve bullet) but holds no spec-041 bullet, and `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'` reads 0. The requirement is written to create the section if a concurrent change consumes it between now and execution.
- Runs the `make precommit` full gate (AC13) as the batch's final validation.
- Coupled to prompt 2: the doc reword describes the post-exit behavior prompt 2 re-applies to `workon.go`. The appended changelog bullet also contradicts the v0.118.3 release note that documented the task-side reversion — a reviewer comment inside requirement 4 flags this tension.
- Open questions surfaced for the reviewer: (1) spec Open Question 1 (configurable `sessionTurnTimeout`) is resolved as a tunable const — no config field; (2) spec Open Question 2 (the vault-ui "Creating session… up to 2 minutes" modal copy) is a separate repo and out of scope here — no vault-ui change is made by this prompt.
</summary>

<objective>
Confirm — and correct where the tree drifted — that the docs and scenario describe the spec-041 inverted (post-exit, no-clear) ordering, and that the CHANGELOG `## Unreleased` carries the spec-041 bullet, so the whole spec-041 batch satisfies ACs 11, 12, and 13.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `docs/work-on-session-lifecycle.md` — the whole file. This is the file under test.
- `scenarios/002-task-lifecycle.md` — the whole file.
- `CHANGELOG.md` — read the top ~50 lines fully (the `# Changelog` header block, `## Unreleased` if present, and the newest version sections `## v0.121.1`, `## v0.121.0`, `## v0.120.0`). That is where the AC12 bullet lands; the rest of the file is not needed.
- `pkg/ops/goal_workon.go` — only to confirm the post-exit wording the doc must match (lines 192-238). Do not modify it.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and style rules, prefix requirement (`feat:` / `fix:` / ...), one bullet per logical change. Write `## Unreleased` bullets only, never a version number and never a manifest/tag bump (this repo's release model: the github-releaser owns version bumps/tags post-merge; `make precommit` runs `check-versions` which requires the four version strings aligned, but this prompt does not hand-bump them).

NOTE: git IS available in this container (`workflow: direct`, no hideGit) — but AC10's `scenarios/005` guard is verified in prompt 1; this prompt has no git commands.
</context>

<requirements>
The docs for this prompt are in a DRIFTED state: the v0.118.3 task-side reversion rewrote the BODIES of the "Post-exit write ordering" and "Failure path" sections (and the per-session lock's detached-child safety paragraph) to describe pre-spawn persist + compensating clear on the task path, while the section HEADINGS and the intro still carry the spec-041 post-exit framing. This prompt rewrites those bodies back to the spec-041 target. The scenario is already correct; the one genuinely missing item is the spec-041 bullet under CHANGELOG `## Unreleased` (AC12).

1. **Guard — prompts 1 and 2 must have shipped.** Before doing anything, confirm prompt 2's deliverables exist: `grep -c 'After(childExitAt)' pkg/ops/workon_test.go` >= 1 AND `grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go` >= 1 AND `grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 0. If ANY is absent, STOP and report `"status":"failed"` with message `"spec-041 prompt 3 precondition missing: prompt 2 not yet deployed"` — do not proceed.

2. **Reword `docs/work-on-session-lifecycle.md`'s task-path sections to the spec-041 post-exit, no-clear ordering.** The intro (lines 3-13, "spec 040, revised by spec 041 ... An id on disk now means the session is resumable") is already correct — keep it. Fix these three stale bodies:
   - **"## Post-exit write ordering" (heading is correct; BODY is stale).** The paragraph currently says `On the **task path** the fresh id and its metrics_sessions entry are now persisted **before the child is spawned**: persistSessionAndMetrics runs first, then StartSession... A spawn failure triggers a re-read-based compensating clear...` and `On the task path the pre-spawn re-read before writing is load-bearing`. Reword the body to:
     ```
     On both paths — task (`pkg/ops/workon.go`) and goal (`pkg/ops/goal_workon.go`) — the
     fresh id and its `metrics_sessions` entry are persisted **only after the detached
     headless turn has finished cleanly**: `StartSession` runs first and blocks through
     the turn, then `persistSessionAndMetrics` / `persistGoalSessionID` re-reads and
     writes.

     The re-read is load-bearing on every branch: the task/goal file is a shared,
     concurrently-written vault file (the headless turn mutates it too), so writing the
     stale in-memory copy would revert the session's own frontmatter changes.

     Nothing is persisted on any failure path (child exit error, invalid turn, timeout,
     cancellation), so there is no clear to run — frontmatter the child wrote
     before failing stays untouched.
     ```
   - **"## Failure path" (heading is correct; BODY is stale).** The paragraph currently says `On the **task path** the id is pre-persisted, so a failed spawn runs a compensating clear...` and `The **goal path** persists nothing on failure and needs no clear.` Reword to:
     ```
     On both paths the id is never written before the turn finishes, so there is no clear to run. A
     failed turn — child exit error, invalid JSON, timeout, or cancellation — leaves the
     task/goal exactly as the child left it: any frontmatter the child wrote before
     failing (for example `phase: planning`) survives, and no `claude_session_id`
     lands. The UI correctly keeps offering **Start**.
     ```
     Keep the following paragraph about the empty-id rule verbatim — it is already correct.
   - **"The per-session lock" section's "**The detached-child safety property.**" paragraph.** It currently says `On the task path the id is pre-persisted, and the safety argument is layered: ... on any failure the compensating clear removes the id, so it cannot stay resumable-looking. The goal path keeps its post-exit ordering, so there the id is only on disk once the child has exited.` Reword to:
     ```
     **The detached-child safety property.** On the spawn path, when the parent stops
     waiting — child exit error, ctx cancel, or the 30m bound — the detached child
     keeps running *without* the parent's lock. The id is never on disk while the child
     runs: both paths persist only after exit, so during the running window Resume is
     not offered for a live turn (the Vault UI resolver fix, shipped separately) and the
     per-session lock (spec 042) refuses a second writer on the same id — the child
     running unlocked is not targetable. On any failure nothing was persisted, so the id
     cannot stay resumable-looking.
     ```
   - Do NOT touch the other sections (`## Session id ownership`, `## Why stream-json was rejected`, `## Why the TTY branch is untouched`, `## The fate of --output-format json`, `## What the turn timeout does and does not cover`, and the rest of `## The per-session lock`) — they are already spec-041. In particular the "liveness gating" phrase in the lock's "Lock scope" paragraph is the spec-042 vault-ui follow-on concept, NOT the removed liveness-window concept — leave it.
   - After the reword, the whole file must contain NO occurrence of the reverted vocabulary: `grep -c 'pre-spawn\|pre-persisted\|pre-persist\|before the child is spawned\|compensating clear' docs/work-on-session-lifecycle.md` must be 0.

3. **Confirm `scenarios/002-task-lifecycle.md` (AC11).** The work-on action note must say the headless turn blocks until completion — it already does (`**Both branches block until the turn completes** ... bounded by a 30m turn timeout` and `A fast return is a FAIL, not a pass`). Verify `grep -c '~10s' scenarios/002-task-lifecycle.md` == 0. If it returns non-zero, replace the "~10s" wording. No other edit to this file.

4. **ADD the spec-041 bullet under the `## Unreleased` section of `CHANGELOG.md` (AC12 — the one genuinely missing item).** Today `## Unreleased` exists (it carries the concurrent spec-044 `resolve` bullet) — do NOT create a second `## Unreleased` section and do NOT touch any existing bullet. Append ONE new bullet directly after the existing bullet(s) within the same section, so `## Unreleased` reads (existing bullet(s) then) the spec-041 bullet:
   ```
   - fix: non-interactive `task work-on` / `goal work-on` now wait for the detached headless turn to exit before persisting `claude_session_id`, bounded by a 30m turn timeout (a wait bound, never a kill), so the Vault UI offers Resume only against a complete, single-writer transcript; a failed or zero-turn session persists no id. The interactive TTY branch is unchanged.
   ```
   If `## Unreleased` does NOT exist when you run (a concurrent change may have consumed it into a `## vX.Y.Z` section), CREATE it: insert `## Unreleased` immediately after the `# Changelog` preamble and before the newest `## vX.Y.Z` section, with the spec-041 bullet as its only bullet. Either way, the spec-041 bullet must appear within the first 15 lines of the `## Unreleased` section so `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'` reads >= 1 from THIS bullet. The bullet must contain the substrings "Resume" (case-insensitive) and "wait for the detached headless turn". Today `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'` reads 0; after this edit it must read >= 1.

   <!-- OPEN QUESTION FOR THE HUMAN REVIEWER: this bullet re-describes the post-exit inversion that v0.117.1 shipped, but v0.118.3's release note then documented the task-side REVERSION back to persist-before-spawn (commit dae6563, "fix(workon): persist the fresh session id before the headless turn"). This bullet is appended to satisfy AC12 and to describe what prompt 2 re-applies; if prompt 2 is rejected at audit (see its requirements comment), this bullet and the doc reword in requirement 2 are both wrong and the spec should be re-scoped instead. Also note spec Open Question 2 (the vault-ui "Creating session… up to 2 minutes" modal copy) is a separate repo and out of scope here — no vault-ui change is made by this prompt. -->

5. **Full gate (AC13).** Run `make precommit` at the repo root. This is the batch's final validation. If it fails on any spec-041-related lint/test, fix it (re-run only the failing target — `make lint`, `make gosec`, `make errcheck`, etc. — then `make precommit` once more). Note the repo's version-alignment rule: `make precommit` runs `check-versions`; do NOT hand-bump the plugin manifests or `git tag` — the github-releaser owns version bumps post-merge. If `check-versions` fails because `## Unreleased` now sits above an un-released `## vX.Y.Z`, that is a pre-existing release-sequencing condition, not a spec-041 defect — report it in `## Improvements` and continue.

6. **Self-check.** Re-read the changed doc/scenario/changelog hunks and walk ACs 11-13. Run the `<verification>` block and confirm every grep holds, including the content-level reverted-vocabulary grep in requirement 2 (the spec's own AC11 greps — `livenessWindow`, `liveness window`, `~10s` — already pass in the current tree; the added grep is what catches the v0.118.3 body drift).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` is untouched — do not edit it (verified in prompt 1).
- Interactive TTY branch unchanged; do not reword any doc text into claiming otherwise.
- This prompt does NOT hand-bump the plugin manifests or `git tag` — only the `## Unreleased` bullet is in scope. Write `## Unreleased`, never `## vX.Y.Z`.
- Do NOT delete or reword the existing `## Unreleased` bullets (e.g. the concurrent spec-044 `resolve` bullet) or any `## vX.Y.Z` section — the spec-041 bullet is APPENDED (or, only if `## Unreleased` is absent, creates the section).
- The "liveness gating" phrase in the per-session lock section is spec-042's vault-ui follow-on concept — leave it; it is not the removed liveness-window concept.
- Do NOT touch `pkg/ops/workon.go`, `pkg/ops/goal_workon.go`, or any `_test.go` file in this prompt — code changes belong to prompts 1 and 2.
- Existing tests must still pass.
</constraints>

<verification>
Grep gates — run each and record the count:

```
# AC11 docs/scenario reword:
grep -c 'livenessWindow' docs/work-on-session-lifecycle.md               # == 0
grep -ci 'liveness window' docs/work-on-session-lifecycle.md             # == 0
grep -c '~10s' scenarios/002-task-lifecycle.md                           # == 0
# Content-level drift guard (catches the v0.118.3 body reversion):
grep -c 'pre-spawn\|pre-persisted\|pre-persist\|before the child is spawned\|compensating clear' docs/work-on-session-lifecycle.md   # == 0
# AC12 CHANGELOG — Unreleased exists AND carries the spec-041 bullet:
grep -c '^## Unreleased' CHANGELOG.md                                    # >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'              # >= 1 — must now match the spec-041 bullet itself (today 0)
grep -c 'wait for the detached headless turn' CHANGELOG.md               # >= 1 — the spec-041 bullet is present
```

FULL GATE — `make precommit` (AC13):
Run `make precommit` at the repo root. It must exit 0. If it fails on a spec-041-related lint/test, fix it (re-run only the failing target, then `make precommit` once more). If `check-versions` fails on a release-sequencing condition unrelated to spec-041, report it in `## Improvements` and re-run after confirming the four version strings are aligned.

AC10's `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` was verified in prompt 1 (this container has git; the guard passed there).
</verification>
