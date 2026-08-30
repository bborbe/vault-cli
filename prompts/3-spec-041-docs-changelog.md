---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-08-30T18:52:00Z"
---

<summary>
- Confirms the session-lifecycle doc no longer contains the liveness-window concept in any form — the wording is "post-exit write ordering", "turn timeout", and "failure path" with no compensating clear.
- Confirms the task-lifecycle scenario note says the headless turn blocks until it completes, with no "~10s" return-time claim.
- Creates the genuinely-missing `## Unreleased` CHANGELOG section with a bullet describing the non-interactive work-on inversion (AC12).
- Runs the `make precommit` full gate (AC13) as the batch's final validation.
- The changelog bullet is a near-duplicate of the already-shipped v0.117.1 entry — an open question for the reviewer is left as a comment inside requirement 4.
- NOTE: `make precommit` may fail at BUILD time until spec 042's executor lands `session_lock.go` — see constraints; do not let that derail the doc/changelog work.
</summary>

<objective>
Confirm — and backfill where anything is missing — that the docs and scenario describe the inverted (post-exit) ordering, and create the missing `## Unreleased` CHANGELOG entry, so the whole spec-041 batch satisfies ACs 11, 12, and 13.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read fully (in this order):
- `docs/work-on-session-lifecycle.md` — the whole file.
- `scenarios/002-task-lifecycle.md` — the whole file.
- `CHANGELOG.md` — read the top ~45 lines fully (the `# Changelog` header block plus the newest version sections `## v0.118.0`, `## v0.117.3`, `## v0.117.2`, `## v0.117.1`). That is where `## Unreleased` goes and where the AC12 bullet lands; the rest of the 1234-line file is not needed.

Coding-plugin docs:
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and style rules, prefix requirement (`feat:` / `fix:` / ...), one bullet per logical change.

NOTE: this container has NO git (`.git` is not mounted). Do NOT run any `git` command. The spec's AC10 `git diff --exit-code HEAD -- scenarios/005-...` check is operator-side and is NOT part of this prompt.
</context>

<requirements>
The docs and scenario for this prompt are ALREADY reworded in the tree (they shipped with the v0.117.1 fix). Your job is to CONFIRM they match the spec-041 target wording, correct any leftover liveness-window phrasing, and create the genuinely-missing `## Unreleased` CHANGELOG entry (AC12).

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
   If either returns non-zero, remove/reword the offending phrasing. The historical narrative about spec 040's behavior in the intro is allowed and does not contain the phrase "liveness window".

3. **Confirm `scenarios/002-task-lifecycle.md` note.** The work-on action note must say the headless turn blocks until completion (it already does: "Both branches block until the turn completes ... typically 2-5 minutes; bounded by a 30m turn timeout"). Verify `grep -c '~10s' scenarios/002-task-lifecycle.md` == 0. If it returns non-zero, replace the "~10s" wording.

4. **CREATE `## Unreleased` in `CHANGELOG.md` (AC12 — the one genuinely missing item).** Today the changelog top is `## v0.118.0` and there is NO `## Unreleased` section. Insert `## Unreleased` directly above `## v0.118.0` — i.e. immediately after the last header line (the `* PATCH version when you make backwards-compatible bug fixes.` line) and its blank line — per the changelog-guide (never insert above or inside the header block). Use this bullet:
   ```
   - fix: non-interactive `task work-on` / `goal work-on` now block until the detached headless turn exits before persisting `claude_session_id` — bounded by a 30m turn timeout (a wait bound, never a kill) — so the Vault UI offers Resume only against a complete, single-writer transcript; a failed or zero-turn session persists no id. The interactive TTY branch is unchanged.
   ```
   The bullet must contain the substring "resume" (case-insensitive) so AC12's grep passes. If `## Unreleased` already exists (a later prompt or release added one), append this bullet to it instead of creating a duplicate section.

   <!-- OPEN QUESTION FOR THE HUMAN REVIEWER: The spec (AC12) requires this Unreleased bullet, and it was written assuming the fix had NOT yet shipped. In reality the fix already shipped and is documented under ## v0.117.1 — so this bullet is a near-duplicate of that entry. It is created here only to satisfy the spec's acceptance-criterion grep. After audit you may drop or reword it (e.g. to a `chore:` note) if you judge the duplication undesirable; if you drop it, the AC12 greps in the spec's Verification will fail and the spec's evidence should be adjusted accordingly. -->

5. **Full gate (AC13).** Run `make precommit` at the repo root. This is the batch's final validation. Because of the pre-existing spec-042 `session_lock.go` build break (see constraints), `make precommit` may fail at the BUILD/`make test` stage before lint runs. If the ONLY failure is that 042 compile error (and all grep gates in `<verification>` hold), report `"status":"partial"` with the 042 build break explicitly named — do NOT "fix" session_lock.go and do NOT mark the prompt failed on it. If the package compiles (042 already landed), `make precommit` must exit 0.

6. **Self-check.** Re-read the changed doc/scenario/changelog hunks and walk ACs 11-13. Run the `<verification>` block and confirm every grep holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Do NOT run any `git` command (no `.git` in this container).
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` is untouched — do not edit it.
- Interactive TTY branch unchanged; do not reword any doc text into claiming otherwise.
- This repo's `.dark-factory.yaml` is `autoRelease: false`; still, this prompt does NOT hand-bump the plugin manifests or `git tag` — only the `## Unreleased` bullet is in scope here. (The changelog-guide's "never a version number on a feature branch" rule applies: write `## Unreleased`, not `## vX.Y.Z`.)
- KNOWN PRE-EXISTING BREAK — NOT YOURS: `pkg/ops/session_lock.go:85` currently fails to compile (`cannot use int(f.Fd()) (value of type int) as uintptr value in argument to unix.FcntlInt`). That is in-flight spec-042 work (`specs/in-progress/042-prevent-duplicate-session-resume.md`), owned by another executor. Do NOT fix it, do NOT delete it, do NOT touch `session_lock.go`/`session_lock_test.go`. Because of it, `make precommit` may fail at BUILD time for reasons unrelated to this prompt.
- Existing tests must still pass (modulo the 042 build break above).
</constraints>

<verification>
Grep gates (none need the package to compile) — run each and record the count:

```
# AC11 docs/scenario reword:
grep -c 'livenessWindow' docs/work-on-session-lifecycle.md               # == 0
grep -ci 'liveness window' docs/work-on-session-lifecycle.md             # == 0
grep -c '~10s' scenarios/002-task-lifecycle.md                           # == 0
# AC12 CHANGELOG (Unreleased must exist and carry the bullet):
grep -c '^## Unreleased' CHANGELOG.md                                    # >= 1 — must flip 0 -> 1 in requirement 4
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'              # >= 1 — must flip 0 -> 1 in requirement 4
```

FULL GATE — `make precommit` (AC13):
Run `make precommit` at the repo root. If it fails ONLY on the pre-existing spec-042 `session_lock.go` build break (see constraints), report `"status":"partial"` with that named in the completion report — do NOT "fix" session_lock.go. If the package compiles (042 already landed), `make precommit` must exit 0; if it fails on any spec-041-related lint/test, fix it (re-run only the failing target, then `make precommit` once more).

The spec's AC10 `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` CANNOT run here (no `.git`). It is verified by the operator ladder in the spec's Verification section.
</verification>
