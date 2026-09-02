---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-02T12:10:00Z"
---

<summary>
- Confirms the session-lifecycle doc no longer contains the liveness-window concept in any form — the wording is "post-exit write ordering", "turn timeout", and "failure path" with no compensating clear.
- Confirms the task-lifecycle scenario note says the headless turn blocks until it completes, with no "~10s" return-time claim.
- Backfills the genuinely-missing CHANGELOG piece: the existing `## Unreleased` section carries no spec-041 bullet (its current bullet is about a session-rename suggestion), so a spec-041 bullet describing the non-interactive work-on inversion is appended (AC12).
- Runs the `make precommit` full gate (AC13) as the batch's final validation.
- The appended bullet is a near-duplicate of the already-shipped v0.117.1 entry — an open question for the reviewer is left as a comment inside requirement 4.
</summary>

<objective>
Confirm — and backfill where anything is missing — that the docs and scenario describe the inverted (post-exit) ordering, and that the CHANGELOG `## Unreleased` carries the spec-041 bullet, so the whole spec-041 batch satisfies ACs 11, 12, and 13.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `docs/work-on-session-lifecycle.md` — the whole file.
- `scenarios/002-task-lifecycle.md` — the whole file.
- `CHANGELOG.md` — read the top ~45 lines fully (the `# Changelog` header block, `## Unreleased`, and the newest version sections `## v0.118.1`, `## v0.118.0`). That is where the AC12 bullet lands; the rest of the file is not needed.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and style rules, prefix requirement (`feat:` / `fix:` / ...), one bullet per logical change. Note: this repo is `autoRelease: false` and the daemon's releaser owns version bumps/tags — write `## Unreleased` bullets only, never a version number.

NOTE: git IS available in this container (`workflow: direct`, no hideGit) — but AC10's `scenarios/005` guard is verified in prompt 1; this prompt has no git commands.
</context>

<requirements>
The docs and scenario for this prompt are ALREADY reworded in the tree (they shipped with the v0.117.1 fix). Your job is to CONFIRM they match the spec-041 target wording, correct any leftover liveness-window phrasing, and append the one genuinely-missing item: the spec-041 bullet under the existing `## Unreleased` CHANGELOG section (AC12).

1. **Guard — prompts 1 and 2 must have shipped.** Before doing anything, confirm prompt 2's deliverables exist: `grep -c 'After(childExitAt)' pkg/ops/workon_test.go` >= 1 AND `grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go` >= 1 AND `grep -c 'clearSessionAndMetrics' pkg/ops/workon.go` == 0. If ANY is absent, STOP and report `"status":"failed"` with message `"spec-041 prompt 3 precondition missing: prompt 2 not yet deployed"` — do not proceed.

2. **Confirm `docs/work-on-session-lifecycle.md` is fully reworded.** Read the file and confirm the following sections are present and say what the spec requires:
   - Intro: states spec 040's ~10s return shipped a worse bug and spec 041 inverted it — "An id on disk now means the session is resumable".
   - "Session id ownership": unchanged content (caller mints the id).
   - "Post-exit write ordering" (NOT "Pre-spawn write ordering"): the id and `metrics_sessions` are persisted only after the child exits; the re-read is load-bearing on every branch; nothing is persisted on any failure; there is no compensating clear.
   - "Why stream-json was rejected": present.
   - "Why the TTY branch is untouched": present.
   - "The fate of --output-format json": says the non-interactive branch captures the detached child's stdout to a caller-owned temp file and validates the same blob after the child exits through the shared `validateSessionTurn` helper (NOT "dead weight on the detached branch").
   - "What the turn timeout does and does not cover": describes `sessionTurnTimeout` (30m, tunable), a wait bound never a kill.
   - "Failure path" (NOT "Compensated failure path"): no-clear, frontmatter survives, empty-id rule.
   If any of these is missing or still describes the pre-041 design, reword it to the target above. THEN verify the two greps hold:
   - `grep -c 'livenessWindow' docs/work-on-session-lifecycle.md` == 0
   - `grep -ci 'liveness window' docs/work-on-session-lifecycle.md` == 0 (prose form too — the reword must not leave the concept behind under different casing)
   If either returns non-zero, remove/reword the offending phrasing. The historical narrative about spec 040's behavior in the intro is allowed and does not contain the phrase "liveness window". Note: the doc has a "The per-session lock" section (spec 042) whose "liveness gating" phrasing about the vault-ui follow-on is NOT the removed liveness-window concept — leave it.

3. **Confirm `scenarios/002-task-lifecycle.md` note.** The work-on action note must say the headless turn blocks until completion (it already does: "Both branches block until the turn completes ... typically 2-5 minutes; bounded by a 30m turn timeout"). Verify `grep -c '~10s' scenarios/002-task-lifecycle.md` == 0. If it returns non-zero, replace the "~10s" wording.

4. **APPEND the spec-041 bullet under the existing `## Unreleased` section of `CHANGELOG.md` (AC12 — the one genuinely missing item).** Today `## Unreleased` already exists (it carries a `fix:` bullet about the `/vault-cli:work-on-task` session-rename suggestion) — do NOT create a second `## Unreleased` section and do NOT touch the existing bullet. Append ONE new bullet directly after it (same section, before the blank line that precedes `## v0.118.1`), so the section reads `## Unreleased` followed by BOTH bullets:
   ```
   - fix: non-interactive `task work-on` / `goal work-on` now wait for the detached headless turn to exit before persisting `claude_session_id` — bounded by a 30m turn timeout (a wait bound, never a kill) — so the Vault UI offers Resume only against a complete, single-writer transcript; a failed or zero-turn session persists no id. The interactive TTY branch is unchanged.
   ```
   The bullet must contain the substring "resume" (case-insensitive) so AC12's grep passes for the right reason. Today `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'` reads 1 only because the `## v0.118.1` bullet bleeds into the -A15 window; after this edit it must match within the Unreleased section itself (count >= 1, and the match should now come from the spec-041 bullet). Run `grep -A15 '^## Unreleased' CHANGELOG.md` after editing and confirm the "resume" match is inside the Unreleased section, not only the v0.118.1 bleed.

   <!-- OPEN QUESTION FOR THE HUMAN REVIEWER: The spec (AC12) requires a ## Unreleased bullet describing the non-interactive inversion. The fix already shipped and is documented under ## v0.117.1, so this bullet is a near-duplicate of that entry — it is appended only to satisfy the acceptance-criterion grep and to keep the Unreleased section self-describing. After audit you may drop or reword it (e.g. to a chore: note) if you judge the duplication undesirable; if you drop it, the AC12 greps in the spec's Verification will fail and the spec's evidence should be adjusted accordingly. Also note spec Open Question 2 (the vault-ui "Creating session… up to 2 minutes" modal copy) is a separate repo and out of scope here — no vault-ui change is made by this prompt. -->

5. **Full gate (AC13).** Run `make precommit` at the repo root. This is the batch's final validation. The spec-042 `session_lock.go` build break is resolved (the package compiles; `make test` passed in prompts 1 and 2), so `make precommit` must exit 0. If it fails on any spec-041-related lint/test, fix it (re-run only the failing target — `make lint`, `make gosec`, `make errcheck`, etc. — then `make precommit` once more).

6. **Self-check.** Re-read the changed doc/scenario/changelog hunks and walk ACs 11-13. Run the `<verification>` block and confirm every grep holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` is untouched — do not edit it (verified in prompt 1).
- Interactive TTY branch unchanged; do not reword any doc text into claiming otherwise.
- This repo is `autoRelease: false`; still, this prompt does NOT hand-bump the plugin manifests or `git tag` — only the `## Unreleased` bullet is in scope here. Write `## Unreleased`, never `## vX.Y.Z` (the changelog-guide's "never a version number on a feature branch" rule).
- Do NOT delete or reword the existing `## Unreleased` session-rename bullet or the `## v0.118.1` / `## v0.118.0` sections — the spec-041 bullet is APPENDED, not merged into anything.
- Existing tests must still pass.
</constraints>

<verification>
Grep gates — run each and record the count:

```
# AC11 docs/scenario reword:
grep -c 'livenessWindow' docs/work-on-session-lifecycle.md               # == 0
grep -ci 'liveness window' docs/work-on-session-lifecycle.md             # == 0
grep -c '~10s' scenarios/002-task-lifecycle.md                           # == 0
# AC12 CHANGELOG — Unreleased exists AND carries the spec-041 bullet:
grep -c '^## Unreleased' CHANGELOG.md                                    # >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'              # >= 1 — must now match the spec-041 bullet itself, not just the v0.118.1 bleed
grep -c 'wait for the detached headless turn' CHANGELOG.md               # >= 1 — the spec-041 bullet is present
```

FULL GATE — `make precommit` (AC13):
Run `make precommit` at the repo root. It must exit 0. If it fails on a spec-041-related lint/test, fix it (re-run only the failing target, then `make precommit` once more).

AC10's `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` was verified in prompt 1 (this container has git; the guard passed there).
</verification>
