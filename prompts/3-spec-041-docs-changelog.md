---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-14T20:23:00Z"
---

# Lifecycle doc reword, CHANGELOG bullet, and the batch's full gate (spec 041, prompt 3 of 3)

<summary>
- Rewords the drifted sections of the work-on session lifecycle design document back to the post-exit ordering: both the task path and the goal path persist the session id only after the headless turn finishes, and no failure path writes anything at all.
- Removes the reverted vocabulary the previous release left behind in that document — the pre-spawn write, the compensating clear, and the liveness-window wording — so the document describes one ordering, not two.
- Corrects one more stale sentence in the same document that still says the compensating clear "still fires on every error" — that sentence is in a section the earlier drafts missed.
- Confirms the task-lifecycle scenario already says the headless turn blocks until completion, with no "returns in ~10s" claim.
- Adds the changelog bullet describing the inversion, appending it to the `## Unreleased` section that is on disk at authoring time (another prompt created it), and creating that section only if a release has consumed it by execution time.
- Flags for the reviewer that this bullet and the document reword contradict the release note that shipped the reversion, and that both are wrong if prompt 2 is rejected at audit.
- Runs the batch's full gate, and re-verifies the resume scenario is untouched by content hash (this container's git is masked, so the spec's `git diff` guard cannot run).
- Documentation and changelog only — no code, no tests.
</summary>

<objective>
Bring `docs/work-on-session-lifecycle.md` in line with the post-exit ordering this batch re-applies, record the change under `## Unreleased` in `CHANGELOG.md`, and run the batch's full gate. Covers spec 041 Acceptance Criteria 11, 12 and 13.
</objective>

<context>
This prompt depends on prompts 1 and 2 having shipped: the document must describe what is actually on disk on this branch, not an intention.

Read `CLAUDE.md` and `docs/dod.md` first. `docs/dod.md` carries the changelog placement rule that is load-bearing here: `## Unreleased` goes **below** the preamble block (the `All notable changes…` line and the `* MAJOR / MINOR / PATCH` lines) and **above** the newest `## vX.Y.Z` section — never between the `# Changelog` title and the preamble. `make check-changelog` (a target inside `make precommit`) enforces exactly that shape.

Then read in full:

- `docs/work-on-session-lifecycle.md` — the whole file. This is the file under edit.
- `scenarios/002-task-lifecycle.md` — the whole file. Confirm it; do not edit it.
- `CHANGELOG.md` — read the top ~30 lines (the preamble and the newest sections). At authoring time a `## Unreleased` section **does** exist, holding another prompt's bullet; the section below it is `## v0.131.10`. `.maintainer.yaml` sets `release.autoRelease: true`, so the top of the file moves between now and execution — re-read it before editing rather than trusting this note, and handle both cases: section present (append the bullet) or absent (create the section, then add the bullet).
- `pkg/ops/workon.go` and `pkg/ops/goal_workon.go` as prompts 1 and 2 left them — the document must describe the real implementation, including the real predicate wording in `pkg/ops/claude_session.go`.
- `specs/in-progress/041-bug-resume-races-live-headless-turn.md` — the spec, for Acceptance Criteria 11-13 and the Failure Modes table.

Coding-plugin doc (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — bullet format, the required prefix, one bullet per logical change, and the `## Unreleased` rules. Write `## Unreleased` bullets only: never a version number, never a plugin-manifest bump, never a tag (this repo's github-releaser owns those post-merge).

Environment note: `git` is unavailable in this container (`/workspace/.git` is a character device; every git call fails with `fatal: not a git repository`). Do not run git. The spec's AC10 `git diff --exit-code HEAD -- scenarios/005-…` guard is replaced by a content-hash pin in `<verification>`.
</context>

<requirements>

## 1. Guard — prompts 1 and 2 must have shipped

Before editing anything, confirm both predecessors' deliverables exist:

```
test "$(grep -c 'validateSessionTurn' pkg/ops/claude_session.go)" -ge 2
test "$(grep -c 'sessionTurnTimeout' pkg/ops/claude_session.go)" -ge 1
test "$(grep -c 'After(childExitAt)' pkg/ops/workon_test.go)" -ge 1
test "$(grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go)" -ge 1
test "$(grep -c 'clearSessionAndMetrics' pkg/ops/workon.go)" = "0"
```

If any fails, STOP and report `"status":"failed"` with the message `"spec-041 prompt 3 precondition missing: prompt 1 or 2 not yet deployed"`. Do not proceed, and do not do prompts 1 or 2's work here.

## 2. Reword the drifted sections of `docs/work-on-session-lifecycle.md`

The document is currently in a mixed state: the headings and the intro carry the spec-041 post-exit framing while three bodies still describe the reverted pre-spawn ordering, and a fourth sentence further down still asserts a compensating clear. Rewrite those four spots to the post-exit, no-clear ordering. Use the text below (it is the target content; you may adjust line wrapping, but not the claims, and not the wording the greps in `<verification>` pin).

**2a. `## Post-exit write ordering` — the body is stale, the heading is correct.** The current body opens `On the **task path** the fresh id and its \`metrics_sessions\` entry are now persisted **before the child is spawned**…` and ends with the sentence about the re-read being load-bearing on the task path. Replace that whole body with:

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
cancellation), so there is no clear to run — frontmatter the child wrote before
failing stays untouched.
```

**2b. `## What the turn timeout does and does not cover` — one stale closing sentence.** The paragraph that lists the rejection messages ends with `The compensating clear is unchanged: it still fires on every error the detached turn returns. Only the definition of "failed" moved.` The first of those two sentences is now false. Replace it so the pair reads:

```
Nothing is persisted on any failure path, so there is no clear to run — frontmatter
the child wrote before failing stays untouched. Only the definition of "failed" moved.
```

Leave every other paragraph of that section untouched, including the predicate strings it quotes (`claude reported is_error: true`, `claude returned num_turns: 0`, `claude returned empty session_id`) and the "validated result outranks the exit code" paragraph — those describe spec 045's shipped behaviour and are correct.

**2c. `## Failure path` — the first paragraph is stale, the heading is correct.** It currently opens `On the **task path** the id is pre-persisted, so a failed spawn runs a compensating clear: …` and ends `The **goal path** persists nothing on failure and needs no clear.` Replace that paragraph with:

```
On both paths the id is never written before the turn finishes, so there is no clear
to run. A failed turn — child exit error, invalid JSON, timeout, or cancellation —
leaves the task/goal exactly as the child left it: any frontmatter the child wrote
before failing (for example `phase: planning`) survives, and no `claude_session_id`
lands. The UI correctly keeps offering **Start**.
```

Keep the following paragraph (the one about the persist step itself failing and the empty-id rule) verbatim — it is already correct.

**2d. `## The per-session lock` — the "detached-child safety property" paragraph is stale.** It currently argues the safety of the pre-persisted id in layers and ends `The goal path keeps its post-exit ordering, so there the id is only on disk once the child has exited.` Replace that paragraph with:

```
**The detached-child safety property.** On the spawn path, when the parent stops
waiting — child exit error, ctx cancel, or the 30m bound — the detached child keeps
running *without* the parent's lock. The id is never on disk while the child runs:
both paths persist only after exit, so during the running window Resume is not offered
for a live turn (the Vault UI resolver fix, shipped separately) and the per-session
lock (spec 042) refuses a second writer on the same id — the child running unlocked is
not targetable. On any failure nothing was persisted, so the id cannot stay
resumable-looking.
```

Leave the rest of `## The per-session lock` alone. In particular the phrase `liveness gating` in its "Lock scope" paragraph is spec 042's vault-ui follow-on concept, NOT the removed liveness-window concept — do not touch it and do not reword it.

**Do NOT touch** `## Session id ownership`, `## Why stream-json was rejected`, `## Why the TTY branch is untouched`, `## The fate of --output-format json`, or the intro — all already describe the target state.

After 2a-2d, the file must contain none of: `livenessWindow`, the prose form `liveness window` (any casing), `pre-spawn`, `pre-persisted`, `before the child is spawned`, or the phrase `compensating clear`. Say what the code does now — "no clear to run", "nothing is persisted on any failure path" — rather than naming what it no longer does; the verification greps are written against the phrase itself, so a sentence like "there is no compensating clear" would trip them.

## 3. Confirm `scenarios/002-task-lifecycle.md`

Its work-on note must already say the headless turn blocks until completion — it does (`**Both branches block until the turn completes** … bounded by a 30m turn timeout`, plus `A fast return is a FAIL, not a pass`). Verify `~10s` does not appear in the file. If it does, replace that wording with the blocking statement; otherwise make no edit. No other change to this file.

## 4. Add the spec-041 bullet to `## Unreleased` in `CHANGELOG.md`

Add exactly one bullet:

```
- fix: non-interactive `task work-on` / `goal work-on` now wait for the detached headless turn to exit before persisting `claude_session_id`, bounded by a 30m turn timeout (a wait bound, never a kill), so the Vault UI offers Resume only against a complete, single-writer transcript; a failed or zero-turn session persists no id. The interactive TTY branch is unchanged.
```

Placement, in this order:

1. Re-read the top of `CHANGELOG.md` at execution time. If `## Unreleased` exists, APPEND the bullet as the last bullet of that section — do not delete, reword, or reorder any existing bullet. If it does not exist (a release consumed it), CREATE the section immediately after the preamble block (after the `* PATCH version when you make backwards-compatible bug fixes.` line) and immediately before the newest `## vX.Y.Z` section, with the bullet as its first bullet.
2. Then run the AC12 check: `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'` must be ≥ 1. If the appended bullet falls outside that 15-line window because the section already carried other bullets, move the spec-041 bullet to be the FIRST bullet under the heading (a move, not a rewrite — no existing bullet is deleted, reworded, or reordered relative to its siblings) and re-run the check. Say in the completion report which placement you used.

Rules that bind this edit: write `## Unreleased` only — never a `## vX.Y.Z` heading, never a plugin-manifest version bump, never a git tag. `make check-changelog` (inside `make precommit`) fails the build if a `## ` section appears above the preamble.

<!-- OPEN QUESTION FOR THE HUMAN REVIEWER: this bullet re-describes the post-exit inversion that v0.117.1 shipped, but the v0.118.3 release note in this same file documents the task-side REVERSION back to persist-before-spawn. The bullet is written to satisfy AC12 and to describe what prompt 2 re-applies; if prompt 2 is rejected at audit (see the reviewer block in prompt 2), then this bullet and requirement 2's reword are both wrong and the spec should be re-scoped instead — the doc's current bodies and the v0.118.3 note already describe the reverted behaviour. Also note spec Open Question 2 (the vault-ui "Creating session… up to 2 minutes" modal copy) lives in a different repo and is out of scope here — this prompt makes no vault-ui change. -->

## 5. Full gate (AC13)

Run `make precommit` at the repo root; it must exit 0. This is the batch's final validation. If it fails on a spec-041-related check, fix it, then re-run ONLY the failing target (`make lint`, `make gosec`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. Note that `check-versions` is NOT part of `make precommit` (it is in `release-check`), so this prompt must not hand-bump any plugin manifest.

## 6. Self-check

Re-read every changed hunk and walk ACs 11, 12 and 13 against it. Run the whole `<verification>` block and confirm every check passes, including the content-level drift greps in requirement 2 (the spec's own AC11 greps — `livenessWindow`, `liveness window`, `~10s` — already pass in the current tree; the added reverted-vocabulary greps are what catch the body drift). In the completion report, name the file and line that satisfies each of AC11, AC12 and AC13.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. (This container has no usable git; make no git calls.)
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` is untouched — do not edit it (its content hash is pinned in `<verification>`).
- Interactive TTY branch unchanged; do not reword any doc text into claiming otherwise.
- This prompt does NOT hand-bump the plugin manifests, write a `## vX.Y.Z` heading, or tag anything — only the `## Unreleased` bullet is in scope.
- Do NOT delete or reword any existing `## Unreleased` bullet or any `## vX.Y.Z` section — the spec-041 bullet is appended (or the section is created if absent).
- The `liveness gating` phrase in the per-session lock section is spec 042's vault-ui follow-on concept — leave it.
- Do NOT touch `pkg/ops/*.go` or any `_test.go` file in this prompt — the code changes belong to prompts 1 and 2.
- Documentation and changelog only: no code, no tests, no scenarios.
- Existing tests must still pass.
</constraints>

<verification>
Run everything from the repo root.

**Full gate — `make precommit` must exit 0** (spec AC13). It must be run unpiped — never pipe it through `tee` or `tail`, the pipeline would report the last stage's status. If it fails, fix the cause, re-run only the failing target, then run `make precommit` once more; if it still fails, report `"status":"failed"` naming the failing target.

AC11 — the document describes one ordering, the post-exit one:

```
! grep -q 'livenessWindow' docs/work-on-session-lifecycle.md
! grep -qi 'liveness window' docs/work-on-session-lifecycle.md
! grep -Eq 'pre-spawn|pre-persisted|before the child is spawned|compensating clear' docs/work-on-session-lifecycle.md
test "$(grep -c 'Post-exit write ordering' docs/work-on-session-lifecycle.md)" = "1"
test "$(grep -c 'no clear to run' docs/work-on-session-lifecycle.md)" -ge 1
test "$(grep -c 'On both paths' docs/work-on-session-lifecycle.md)" -ge 1
test "$(grep -c 'detached-child safety property' docs/work-on-session-lifecycle.md)" = "1"
```

AC11 — the scenario:

```
! grep -q '~10s' scenarios/002-task-lifecycle.md
```

AC12 — the changelog section exists and carries the bullet:

```
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "1"
test "$(grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume')" -ge 1
test "$(grep -c 'wait for the detached headless turn' CHANGELOG.md)" -ge 1
make check-changelog
```

AC10 — `scenarios/005` untouched. The spec's guard is `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md`, which cannot run here: this container's `.git` is a character device, so every git command dies with `fatal: not a git repository`. The hash and byte count below are the substitute and pin the file exactly as it stands at authoring time. If a check fails you edited the scenario — revert your edit; do NOT update the expected value and do NOT add a git command.

```
test "$(sha256sum scenarios/005-work-on-resume-auto-invokes-subtask.md | cut -d' ' -f1)" = "973840d5a8c6a55cb84c6db10c9c24ab2ff269b1ba0fa82e31ab1ac630ea0153"
test "$(wc -c < scenarios/005-work-on-resume-auto-invokes-subtask.md)" = "7533"
test "$(grep -c 'claude_session_id' scenarios/005-work-on-resume-auto-invokes-subtask.md)" = "3"
```

If `sha256sum` is not present in this image, the byte count plus the `claude_session_id` line count are the substitute and both must still hold; note the missing tool in `## Improvements`.

Prompt 2's code invariants must still hold (this prompt touches no code, so a failure here means an earlier prompt regressed):

```
test "$(grep -c 'clearSessionAndMetrics' pkg/ops/workon.go)" = "0"
test "$(grep -c 'After(childExitAt)' pkg/ops/workon_test.go)" -ge 1
test "$(grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go)" -ge 1
```

Suite:

```
go test ./pkg/ops/...
```
must pass (unpiped).
</verification>

<!-- DARK-FACTORY-REPORT -->
