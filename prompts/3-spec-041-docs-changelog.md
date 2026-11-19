---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-11-19T12:00:00Z"
---

# Reword the work-on lifecycle doc, confirm the scenario, and record the change (spec 041, prompt 3 of 3)

<summary>
- Rewords the stale task-path sections of the work-on session lifecycle document so they describe both paths persisting the session id only after the headless turn exits, with no compensating clear — removing the pre-spawn wording a later reversion left behind under already-correct headings, while preserving the goal/topic ownership and metrics-append rules that are already accurate.
- Confirms the task-lifecycle scenario already states that the headless turn blocks until completion and no longer claims a fast return, so no edit is needed there.
- Creates the changelog's unreleased section (this repository has none: the newest heading is a released version) and puts one conventional bullet in it.
- Does not touch the plugin version strings or any tag — this repository's release agent owns version bumps and tagging after merge, so a feature branch only adds an unreleased bullet.
- Runs the repository's scoped documentation gate (`make check-changelog`) and reports the exact exit code. The full repository gate (`make precommit`, spec AC13) ran in prompt 2, the batch's last Go-changing prompt; repeating it here would duplicate a slow check that a markdown-only change cannot break beyond `check-changelog`.
- Coupled to the task-persistence prompt: the document reword and the changelog bullet both describe the ordering that prompt re-applies. If that prompt is rejected at audit, this one must be rejected too, because it would then document behaviour the code does not have. A reviewer comment inside requirement 5 records the tension with the release note that documented the earlier reversion.
- Flags a broken guard inherited from the earlier generation of this prompt: its "no compensating clear anywhere" grep was unsatisfiable, because a legitimate "no compensating clears" sentence in the per-session-lock section must survive the reword. The corrected gate pins the exact surviving count.
- Confirms the two open questions from the spec are already resolved and need no work: the turn bound stays a constant rather than a config field, and the Vault UI modal copy lives in a different repository and is out of scope.
- Carries no git commands: `.git` is masked in this container, so the batch's "scenario 005 unchanged" invariant is checked with a content checksum instead of the spec's `git diff`.
</summary>

<objective>
Bring the durable documentation and the changelog in line with the re-applied post-exit session-id ordering, so spec 041's ACs 11 and 12 hold and the batch ships with an accurate design record and a releasable changelog entry. AC13's full gate ran in prompt 2.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully (in this order):
- `docs/work-on-session-lifecycle.md` — the whole file (281 lines). This is the file under test. The stale bodies are at the `## Post-exit write ordering` section (lines 32-60), one sentence in `## What the turn timeout does and does not cover` (line 196), the `## Failure path` first paragraph (lines 213-219), and the detached-child paragraph in `## The per-session lock` (lines 273-281).
- `scenarios/002-task-lifecycle.md` — the whole file (67 lines). Already correct; you are confirming, not editing.
- `CHANGELOG.md` — read the top ~15 lines only. The frozen preamble is lines 1-9 (title, "All notable changes…" line, the SemVer link, and the MAJOR/MINOR/PATCH bullets); the newest heading is `## v0.198.2` at line 11 and there is NO `## Unreleased` section. That is the whole edit surface — the rest of the 2226-line file is not needed and must not be touched.
- `pkg/ops/workon.go` and `pkg/ops/goal_workon.go` — read only to confirm the ordering the documentation must describe. Do not modify either.
- `prompts/2-spec-041-post-exit-persist.md` — read the reviewer comment at the top of its `<requirements>` block; it records the conflict this prompt is coupled to.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the unreleased-section placement rule, the frozen-preamble rule, the required conventional prefixes, and the rule that in an auto-release repository a feature branch adds bullets under the unreleased section and does NOT bump version strings.
- `/home/node/.claude/plugins/marketplaces/coding/docs/git-workflow.md` — commit/branch conventions (this prompt does not commit).

**`.git` is MASKED in this container.** This repo uses `.dark-factory.yaml` `workflow: worktree`, which masks `.git` (a character device), so every `git` invocation fails with `fatal: not a git repository`. This prompt issues no git commands and must not add any. The spec's AC10 `git diff --exit-code HEAD -- scenarios/005-...` guard cannot run here; the batch checks it with the content pin in `<verification>` instead (prompt 1 owns that check; it is repeated here as the final guard).
</context>

<requirements>
The documentation is in a DRIFTED state. A later change rewrote the BODIES of the `## Post-exit write ordering` and `## Failure path` sections, one sentence in `## What the turn timeout does and does not cover`, and the detached-child paragraph in `## The per-session lock`, to describe a pre-spawn write plus a compensating clear on the task path — while the section HEADINGS and the introduction still carry the post-exit framing. The scenario is already correct. The changelog needs a new unreleased section with one bullet.

1. **Fail-loud gate — prompts 1 and 2 must have shipped.** Confirm prompt 2's deliverables exist:
   ```
   grep -c 'After(childExitAt)' pkg/ops/workon_test.go      # must be >= 1
   grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go # must be >= 1
   grep -c 'clearSessionAndMetrics' pkg/ops/workon.go       # must be 0
   ```
   If ANY check fails, STOP and report `"status":"failed"` with message `"spec-041 prompt 3 precondition missing: prompt 2 not yet deployed"`. Do NOT proceed, and do NOT reword the documentation to describe an ordering the code does not implement — if prompt 2 was rejected at audit, this prompt is wrong and should be rejected too.

2. **Reword the stale bodies in `docs/work-on-session-lifecycle.md` to the post-exit, no-clear ordering.** The introduction (the paragraph mentioning "spec 040, revised by spec 041", "An id on disk now means the session is resumable") is already correct — keep it. Fix these four spots, and nothing else:

   - **`## Post-exit write ordering`** — the heading is correct; three pieces of its body are stale, and the long middle paragraph is NOT. Make exactly three edits:
     1. Replace the FIRST paragraph (lines 34-41, starting `On the **task path** the fresh id and its `metrics_sessions` entry are now persisted **before the child is spawned**` and ending `metrics entry (see `## Failure path`).`) with:
        ```
        On both paths the fresh id and its `metrics_sessions` entry are persisted
        **only after the detached headless turn has finished cleanly**: `StartSession`
        runs first and blocks through the turn, then `persistSessionAndMetrics` /
        `persistGoalSessionID` re-reads and writes. The id is never on disk while a
        child runs, so an id on disk always names a completed, single-writer
        transcript. Nothing is persisted on any failure path — child exit error,
        invalid turn result, bound expiry, or cancellation — so there is no clear to
        run (see `## Failure path`).
        ```
     2. In the SECOND paragraph (lines 43-56), change only its opening clause. `The **goal and topic paths** (pkg/ops/goal_workon.go, pkg/ops/topic_workon.go) keep their post-exit ordering unchanged: `persistGoalSessionID` / `persistTopicSessionID` run only after `StartSession` returns cleanly.` becomes `The **goal and topic paths** (pkg/ops/goal_workon.go, pkg/ops/topic_workon.go) share that ordering: `persistGoalSessionID` / `persistTopicSessionID` run only after `StartSession` returns cleanly.` **Keep every remaining sentence of that paragraph byte-identical** — it carries the claim-window rule, the empty-field ownership rule, the `metrics_sessions` append rule, and the accepted last-write-wins race. Do NOT delete it; it is accurate and load-bearing.
     3. Replace the THIRD paragraph (lines 58-60, starting `On the task path the pre-spawn re-read before writing is load-bearing:`) with:
        ```
        The re-read before writing is load-bearing on every branch: the task/goal file
        is a shared, concurrently-written vault file (the headless turn mutates it too),
        so writing the stale in-memory copy would revert the child's frontmatter
        changes.
        ```

   - **`## What the turn timeout does and does not cover`** — one sentence (line 196) is stale: `The compensating clear is unchanged: it still fires on every error the detached turn returns. Only the definition of "failed" moved.` Replace it with `Nothing is cleared on any error the detached turn returns — nothing was written before the turn, so there is nothing to undo. Only the definition of "failed" moved.` Leave the rest of that section, including the "read is never hoisted above the select" paragraph, exactly as it is. (Note: the replacement deliberately avoids the phrase `compensating clear` — the drift gate in `<verification>` pins exactly one surviving occurrence of it in the file.)

   - **`## Failure path`** — replace the first paragraph (lines 213-219, starting `On the **task path** the id is pre-persisted, so a failed spawn runs a compensating clear:`) with:
     ```
     On both paths the id is never written before the turn finishes, so there is no
     clear to run. A failed turn — child exit error, invalid turn result, bound expiry,
     or cancellation — leaves the task/goal exactly as the child left it: any
     frontmatter the child wrote before failing (for example `phase: planning`) survives,
     and no `claude_session_id` lands. The UI correctly keeps offering **Start**.
     ```
     Keep the following paragraph (lines 221-224) about the persist step's own failure and the empty-id rule verbatim — it is already correct.

   - **`## The per-session lock`** — the `**The detached-child safety property.**` paragraph (lines 273-281) is stale. Replace it with:
     ```
     **The detached-child safety property.** On the spawn path, when the parent stops
     waiting — a failed turn, ctx cancel, or the 30m bound — the detached child keeps
     running *without* the parent's lock. The id is never on disk while the child runs:
     both paths persist only after exit, so during the running window Resume is not
     offered for a live turn (the Vault UI resolver fix, shipped separately) and the
     per-session lock (spec 042) refuses a second writer on the same id — the child
     running unlocked is not targetable. On any failure nothing was persisted, so the id
     cannot stay resumable-looking.
     ```

   - Do NOT touch `## Session id ownership`, `## The second writer: the session-connect append`, `## Why stream-json was rejected`, `## Why the TTY branch is untouched`, `## The fate of --output-format json`, or the rest of `## The per-session lock`. In particular:
     - the phrase "liveness gating" in the lock's `**Lock scope.**` paragraph (line 270) is a spec-042 Vault UI follow-on concept, NOT the removed liveness-window concept — leave it;
     - the sentence `There are no cleanup sweeps, no compensating clears, and no lock TTL` in the `**No stale lock.**` paragraph (line 247) is a legitimate statement about the LOCK and must survive verbatim. It is the one `compensating clear` occurrence the drift gate in `<verification>` expects to remain.
   - `scenarios/005-work-on-resume-auto-invokes-subtask.md` is never edited.

3. **Confirm `scenarios/002-task-lifecycle.md` — no edit expected (AC11).** Its work-on action note must already state that the headless turn blocks until completion (it does: `**Both branches block until the turn completes**`, `bounded by a 30m turn timeout`, and `A fast return is a FAIL, not a pass`). Confirm `grep -c '~10s' scenarios/002-task-lifecycle.md` is 0. If it is non-zero, replace the fast-return wording with the blocking wording. Make no other change to this file.

4. **Create the unreleased section in `CHANGELOG.md` and add one bullet (AC12).** The file currently has NO `## Unreleased` section: the preamble ends at line 9 and `## v0.198.2` is the first heading (line 11). Insert, immediately after the preamble's PATCH bullet line and before `## v0.198.2`:
   ```markdown
   ## Unreleased

   - fix: non-interactive `task work-on` persists `claude_session_id` only after the detached headless turn exits, matching `goal work-on`, so the Vault UI offers Resume only against a complete, single-writer transcript; a failed, zero-turn, or timed-out turn persists no id and the button reverts to Start. Replaces the task path's pre-spawn persist and its compensating clear; the turn wait stays bounded by the 30m turn timeout and the interactive TTY branch is unchanged.

   ```
   Rules for this edit:
   - The section heading must be exactly `## Unreleased` (never `## vX.Y.Z`), placed AFTER the frozen preamble and ABOVE the newest released heading. `scripts/check-changelog.sh` fails if any `## ` section precedes the preamble line.
   - The bullet must stay within the first 15 lines after the `## Unreleased` heading and must contain the substring "Resume" (case-insensitive), because AC12's evidence grep reads exactly those 15 lines.
   - Never move, delete, or edit the frozen preamble (`# Changelog`, the "All notable changes…" line, the SemVer link, the MAJOR/MINOR/PATCH bullets), and never delete, reorder, or reword an existing bullet or any `## vX.Y.Z` section.
   - Do NOT bump the plugin version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag. This repository's release agent owns version bumps and tagging after merge (see `CLAUDE.md` § Plugin Release Checklist and the changelog guide's "Version Alignment Is Release-Time"). The four version strings stay aligned at the last released version.

   <!-- OPEN QUESTION FOR THE HUMAN REVIEWER: this bullet re-describes the post-exit ordering that an earlier release shipped, while a later release note documented the task-side REVERSION back to persist-before-spawn (the CHANGELOG bullet beginning "persists the fresh `claude_session_id` ... before the headless Claude session is spawned"), whose rationale was a live-reproduced session-connect mis-binding. The bullet and the documentation reword in requirement 2 are correct only if prompt 2 is approved. If prompt 2 is rejected at audit, reject this prompt as well and re-scope spec 041 instead. Separately, spec Open Question 2 — the Vault UI "Creating session… up to 2 minutes" modal copy — lives in the vault-ui repository and is out of scope here; no Vault UI change is made by this prompt. -->

5. **Confirm the spec's two open questions need no code change.** Open Question 1: the turn bound stays an unexported tunable constant with no config field — do not add one (spec Non-goals). Open Question 2: the Vault UI modal copy is a different repository — no change here.

6. **Scoped gate (AC12/AC11 structural).** Run `make check-changelog` at the repo root; it must print `CHANGELOG structure OK` and exit 0. Do NOT run `make precommit` here: spec AC13's full gate ran in prompt 2, the last prompt that changed Go code, and this prompt changes only markdown. If `make check-changelog` fails, fix the section placement (requirement 4) and re-run it.

7. **Self-check before finishing.** Re-read the changed doc and changelog hunks and walk spec 041 ACs 11 and 12 against them. Run every command in `<verification>` and confirm each holds — including the content-level drift guard, which is what actually catches the stale body wording. The spec's own AC11 greps (`livenessWindow`, "liveness window", `~10s`) already pass in the current tree and are therefore NOT evidence that the reword was done; the drift guard is.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git (and `.git` is masked in this container, so no git command can run anyway). Do not write a `git` command anywhere in this prompt's execution: every invocation fails with `fatal: not a git repository`, and the daemon records a failed verification command as a pass.
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` is untouched — do not edit it (verify with the content pin in `<verification>`).
- The interactive TTY branch is unchanged; do not reword any documentation text into claiming otherwise.
- Do NOT bump the plugin manifests and do NOT create a tag — only the unreleased section is in scope. Never write `## vX.Y.Z`; the heading is `## Unreleased`.
- Do NOT delete or reword existing changelog bullets or any `## vX.Y.Z` section.
- The "liveness gating" phrase in the per-session lock section is spec 042's Vault UI follow-on concept — leave it; it is not the removed liveness-window concept. The "no compensating clears" sentence in the `**No stale lock.**` paragraph must survive verbatim.
- The goal/topic paragraph in `## Post-exit write ordering` must keep every sentence except its opening clause — it documents the claim-window, ownership and metrics-append rules, none of which this spec changes.
- Do NOT touch `pkg/ops/*.go` or any `_test.go` file in this prompt — code changes belong to prompts 1 and 2.
- Do NOT add a config field for the turn bound (spec Non-goals / Open Question 1).
- Existing tests must still pass (no Go change is expected, so this is a sanity check only; do not run `make precommit`).
</constraints>

<verification>
Evidence greps — run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c '^## Unreleased' CHANGELOG.md                                          # == 1 (AC12) — must flip 0 -> 1; a SECOND section is the failure mode here
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'                    # >= 1 (AC12) — must flip 0 -> >= 1
grep -c 'only after the detached headless turn exits' CHANGELOG.md             # >= 1 if you used the suggested wording
[ "$(grep -m1 '^## ' CHANGELOG.md)" = "## Unreleased" ]                        # the first heading is Unreleased
grep -m1 '^## v' CHANGELOG.md                                                  # still v0.198.2 (no version bump)
! grep -q 'livenessWindow' docs/work-on-session-lifecycle.md                   # AC11: absent (already true)
! grep -ci 'liveness window' docs/work-on-session-lifecycle.md                 # AC11: absent (already true)
! grep -q '~10s' scenarios/002-task-lifecycle.md                               # AC11: absent (already true)
! grep -qE 'pre-spawn|pre-persisted|pre-persist|before the child is spawned' docs/work-on-session-lifecycle.md   # drift guard — the real check for req 2
[ "$(grep -c 'compensating clear' docs/work-on-session-lifecycle.md)" -eq 1 ]  # exactly the surviving "no compensating clears" sentence in the lock section
grep -c 'share that ordering' docs/work-on-session-lifecycle.md                # >= 1 — proves the goal/topic paragraph survived (req 2.2)
grep -c 'persistGoalSessionID` / `persistTopicSessionID` run only after' docs/work-on-session-lifecycle.md   # >= 1 — same paragraph intact
grep -n -m1 '^All notable changes to this project' CHANGELOG.md                # preamble present
grep -n -m1 '^## ' CHANGELOG.md                                                # must be the Unreleased heading, AFTER the preamble line above
```

Why the compensating-clear gate is a count and not a zero: the earlier generation of this prompt demanded zero `compensating clear` matches, which is unsatisfiable — the `**No stale lock.**` paragraph legitimately says "There are no cleanup sweeps, no compensating clears, and no lock TTL", and requirement 2 forbids touching it. Before your reword the file has four occurrences (lines 40, 196, 247, 279); after a correct reword exactly one (line 247) remains. A count of 2+ means a stale body survived; a count of 0 means you edited the lock section, which is forbidden.

SCENARIO-005 GUARD — content pin (replaces the spec's `git diff --exit-code HEAD`, which cannot run: `git` is masked in this container):

```
printf '%s  %s\n' '973840d5a8c6a55cb84c6db10c9c24ab2ff269b1ba0fa82e31ab1ac630ea0153' 'scenarios/005-work-on-resume-auto-invokes-subtask.md' | sha256sum -c -
```
must print `scenarios/005-work-on-resume-auto-invokes-subtask.md: OK`. If it prints FAILED, you edited a file this prompt forbids — revert that edit; do NOT "fix" the pin.

SCOPED GATE — `make check-changelog` must print `CHANGELOG structure OK` and exit 0. This is the only repository gate this markdown-only prompt needs; spec AC13's `make precommit` ran in prompt 2, the batch's last Go-changing prompt.

The spec's AC10 guard (`git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md`) is the content pin above; it is also verified in prompt 1. There is no runnable git command in this batch.
</verification>
