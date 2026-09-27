---
spec: ["041-bug-resume-races-live-headless-turn"]
status: draft
created: "2026-09-27T08:54:45Z"
---

# Reword the work-on lifecycle doc, confirm the scenario, and record the change (spec 041, prompt 3 of 3)

<summary>
- Rewords the task-path sections of the work-on session lifecycle document so they describe both paths persisting the session id only after the headless turn exits, with no compensating clear — removing the pre-spawn wording a later reversion left behind under an already-correct heading.
- Confirms the task-lifecycle scenario already states that the headless turn blocks until completion and no longer claims a fast return, so no edit is needed there.
- Creates the changelog's unreleased section (it does not exist today) and records the change under it with a conventional prefix.
- Does not touch the plugin version strings or any tag — this repository's release agent owns version bumps and tagging after merge, so a feature branch only adds an unreleased bullet.
- Runs the repository's full gate as the batch's final validation, and reports the exact exit code.
- Coupled to the task-persistence prompt: the document reword and the changelog bullet both describe the ordering that prompt re-applies. If that prompt is rejected at audit, this one must be rejected too, because it would then document behaviour the code does not have. A reviewer comment inside requirement 4 records the tension with the release note that documented the earlier reversion.
- Flags one thing this prompt does NOT fix: the work-on-task agent definition still describes the headless start path as pre-setting the session id before the turn. That text becomes wrong if the reorder ships; it is outside this spec's stated scope and is recorded as a reviewer-owned follow-up.
- Confirms the two open questions from the spec are already resolved and need no work: the turn bound stays a constant rather than a config field, and the Vault UI modal copy lives in a different repository and is out of scope.
</summary>

<objective>
Bring the durable documentation and the changelog in line with the re-applied post-exit session-id ordering, so spec 041's ACs 11, 12 and 13 hold and the batch ships with an accurate design record and a releasable changelog entry.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully (in this order):
- `docs/work-on-session-lifecycle.md` — the whole file (242 lines). This is the file under test.
- `scenarios/002-task-lifecycle.md` — the whole file (67 lines).
- `CHANGELOG.md` — read the top ~20 lines only (the frozen `# Changelog` preamble and the newest versioned section, `## v0.149.0`). That is where the new section and bullet land; the rest of the file is not needed.
- `pkg/ops/workon.go` and `pkg/ops/goal_workon.go` — read only to confirm the ordering the documentation must describe. Do not modify either.
- `prompts/2-spec-041-post-exit-persist.md` — read the reviewer comment at the top of its `<requirements>` block; it records the conflict this prompt is coupled to.
- `docs/dod.md` — the repository's Definition of Done; its Documentation section states the changelog placement rule this prompt must satisfy.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the unreleased-section placement rule, the frozen-preamble rule, the required conventional prefixes, and the rule that in an auto-release repository a feature branch adds bullets under the unreleased section and does NOT bump version strings.
- `/home/node/.claude/plugins/marketplaces/coding/docs/git-workflow.md` — commit/branch conventions (this prompt does not commit).

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), but this prompt issues no git commands; the scenario-005 guard was verified in prompt 1.
</context>

<requirements>
The documentation is in a DRIFTED state. A later change rewrote the BODIES of the "Post-exit write ordering" and "Failure path" sections, and the per-session lock's detached-child safety paragraph, to describe a pre-spawn write plus a compensating clear on the task path — while the section HEADINGS and the introduction still carry the post-exit framing. The scenario is already correct. The one genuinely missing artifact is the changelog's unreleased section.

1. **Fail-loud gate — prompts 1 and 2 must have shipped.** Confirm prompt 2's deliverables exist:
   ```
   grep -c 'After(childExitAt)' pkg/ops/workon_test.go      # must be >= 1
   grep -c 'After(childExitAt)' pkg/ops/goal_workon_test.go # must be >= 1
   grep -c 'clearSessionAndMetrics' pkg/ops/workon.go       # must be 0
   ```
   If ANY check fails, STOP and report `"status":"failed"` with message `"spec-041 prompt 3 precondition missing: prompt 2 not yet deployed"`. Do NOT proceed, and do NOT reword the documentation to describe an ordering the code does not implement — if prompt 2 was rejected at audit, this prompt is wrong and should be rejected too.

2. **Reword the stale bodies in `docs/work-on-session-lifecycle.md` to the post-exit, no-clear ordering.** The introduction (the paragraph mentioning "spec 040, revised by spec 041", "An id on disk now means the session is resumable") is already correct — keep it. Fix these four bodies, and nothing else:
   - **`## Post-exit write ordering`** (line 32) — the heading is correct; the body is stale. Replace the whole body (the paragraph starting `On the **task path** the fresh id and its metrics_sessions entry are now persisted **before the child is spawned**...`, the sentence `The **goal path** (pkg/ops/goal_workon.go) keeps its post-exit ordering unchanged: persistGoalSessionID runs only after StartSession returns cleanly.`, and the paragraph starting `On the task path the pre-spawn re-read before writing is load-bearing: ...`) with:
     ```
     On both paths — task (`pkg/ops/workon.go`) and goal (`pkg/ops/goal_workon.go`) — the
     fresh id and its `metrics_sessions` entry are persisted **only after the detached
     headless turn has finished cleanly**: `StartSession` runs first and blocks through
     the turn, then `persistSessionAndMetrics` / `persistGoalSessionID` re-reads and
     writes.

     The re-read is load-bearing on every branch: the task/goal file is a shared,
     concurrently-written vault file (the headless turn mutates it too), so writing the
     stale in-memory copy would revert the session's own frontmatter changes.

     Nothing is persisted on any failure path — child exit error, invalid turn result,
     bound expiry, or cancellation — so there is no clear to run, and frontmatter the
     child wrote before failing stays untouched.
     ```
     The goal-path sentence is subsumed by the replacement — delete it rather than leaving it as a dangling contrast.
   - **`## What the turn timeout does and does not cover`** (line 131) — one sentence in the last-but-one paragraph is stale: `The compensating clear is unchanged: it still fires on every error the detached turn returns. Only the definition of "failed" moved.` Replace it with `No compensating clear runs on any error the detached turn returns — nothing was written before the turn, so there is nothing to undo. Only the definition of "failed" moved.` Leave the rest of that section, including the "read is never hoisted above the select" paragraph, exactly as it is.
   - **`## Failure path`** (line 172) — replace the body paragraph starting `On the **task path** the id is pre-persisted, so a failed spawn runs a compensating clear: ...` (lines 174-180) with:
     ```
     On both paths the id is never written before the turn finishes, so there is no clear
     to run. A failed turn — child exit error, invalid turn result, bound expiry, or
     cancellation — leaves the task/goal exactly as the child left it: any frontmatter the
     child wrote before failing (for example `phase: planning`) survives, and no
     `claude_session_id` lands. The UI correctly keeps offering **Start**.
     ```
     Keep the following paragraph about the empty-id rule (starting `The persist step itself can also fail ...`) verbatim — it is already correct.
   - **`## The per-session lock`** (line 187) — the `**The detached-child safety property.**` paragraph (lines 234-242) is stale. Replace it with:
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
   - Do NOT touch `## Session id ownership`, `## The second writer: the session-connect append` (line 50), `## Why stream-json was rejected`, `## Why the TTY branch is untouched`, `## The fate of --output-format json`, or the rest of `## The per-session lock`. In particular, the phrase "liveness gating" in the lock's "Lock scope" paragraph is a spec-042 Vault UI follow-on concept, NOT the removed liveness-window concept — leave it. `scenarios/005-work-on-resume-auto-invokes-subtask.md` is never edited.
   - After the reword the whole file must contain none of the reverted vocabulary — see the drift-guard greps in `<verification>`.

3. **Confirm `scenarios/002-task-lifecycle.md` — no edit expected (AC11).** Its work-on action note must already state that the headless turn blocks until completion (it does: `**Both branches block until the turn completes**`, `bounded by a 30m turn timeout`, and `A fast return is a FAIL, not a pass`). Confirm `grep -c '~10s' scenarios/002-task-lifecycle.md` is 0. If it is non-zero, replace the fast-return wording with the blocking wording. Make no other change to this file.

4. **Create the unreleased section in `CHANGELOG.md` and record the change under it (AC12).** Today `CHANGELOG.md` has NO unreleased section — the newest section is `## v0.149.0` (line 11). Insert, immediately after the frozen preamble (the `* MAJOR version...` / `* MINOR version...` / `* PATCH version...` bullet block ending at line 9) and immediately above `## v0.149.0`:
   ```
   ## Unreleased

   - fix: non-interactive `task work-on` persists `claude_session_id` only after the detached headless turn exits, matching `goal work-on`, so the Vault UI offers Resume only against a complete, single-writer transcript; a failed, zero-turn, or timed-out turn persists no id and the button reverts to Start. Replaces the task path's pre-spawn persist and its compensating clear; the turn wait stays bounded by the 30m turn timeout and the interactive TTY branch is unchanged.
   ```
   Rules for this edit:
   - If an unreleased section already exists when you run (a concurrent change may have created it), do NOT create a second one — append this bullet as the last bullet inside the existing section.
   - Never move, delete, or edit the frozen preamble (`# Changelog`, the "All notable changes…" line, the SemVer link, the MAJOR/MINOR/PATCH bullets). `scripts/check-changelog.sh` fails the build if any `## ` section precedes the preamble line.
   - Do NOT delete, reorder, or reword any existing bullet or any `## vX.Y.Z` section. In particular, do NOT touch the `## v0.117.1` section (which recorded the original spec-041 change) or the `## v0.118.3` section (which recorded the reversion this prompt reverses) — the changelog is append-only history and both entries stay as they are.
   - Do NOT bump the plugin version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag. This repository's release agent owns version bumps and tagging after merge (`.maintainer.yaml` sets `release.autoRelease: true`; see `CLAUDE.md` § Plugin Release Checklist and the changelog guide's "Version Alignment Is Release-Time"). Write `## Unreleased`, never `## vX.Y.Z`.
   - The bullet must be within the first 15 lines after the `## Unreleased` heading, and must contain the substring "Resume" (case-insensitive), because AC12's evidence grep reads exactly those 15 lines.

   <!-- OPEN QUESTION FOR THE HUMAN REVIEWER: this bullet re-describes the post-exit ordering that v0.117.1 shipped, while the v0.118.3 release note documented the task-side REVERSION back to persist-before-spawn (commit dae6563, "fix(workon): persist the fresh session id before the headless turn", whose rationale was a live-reproduced session-connect mis-binding). The bullet and the documentation reword in requirement 2 are correct only if prompt 2 is approved. If prompt 2 is rejected at audit, reject this prompt as well and re-scope spec 041 instead. Separately, spec Open Question 2 — the Vault UI "Creating session… up to 2 minutes" modal copy — lives in the vault-ui repository and is out of scope here; no Vault UI change is made by this prompt. -->

5. **Note, do not fix, the agent-definition drift.** `agents/work-on-task-assistant.md` § Session connect (line 158) still states `A miss is safe **only** on the headless Start path, which pre-sets this field via vault-cli before the turn`, and line 124 describes the status-flip routing that depends on the same pre-set. If prompt 2 ships, that text describes a pre-spawn pre-set the code no longer performs. Updating `agents/work-on-task-assistant.md` is OUTSIDE this spec's stated file scope (the spec's Design and Suggested Decomposition name only `pkg/ops` and `docs/scenarios`), so make NO edit to it in this prompt. Record it in the completion report's `## Improvements` section as a reviewer-owned follow-up (category: PROMPT) so it is not lost.

6. **Confirm the spec's two open questions need no code change.** Open Question 1: the turn bound stays an unexported tunable constant with no config field — do not add one (spec Non-goals). Open Question 2: the Vault UI modal copy is a different repository — no change here.

7. **Full gate (AC13).** Run `make precommit` at the repo root. It must exit 0. If it fails on something this prompt introduced (most likely `check-changelog`), fix it and re-run only the failing target (`make check-changelog`, `make lint`, ...), then `make precommit` once more. Note that `make precommit` runs `check` (which includes `check-changelog`) but does NOT run `check-versions`; the four version strings are already aligned at the last released version and must stay that way.

8. **Self-check before finishing.** Re-read the changed doc, scenario, and changelog hunks and walk spec 041 ACs 11, 12 and 13 against them. Run every command in `<verification>` and confirm each holds — including the content-level drift guard, which is what actually catches the stale body wording; the spec's own AC11 greps already pass in the current tree and are therefore NOT evidence that the reword was done.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- `scenarios/005-work-on-resume-auto-invokes-subtask.md` is untouched — do not edit it.
- The interactive TTY branch is unchanged; do not reword any documentation text into claiming otherwise.
- Do NOT bump the plugin manifests and do NOT create a tag — only the unreleased bullet is in scope. Write `## Unreleased`, never `## vX.Y.Z`.
- Do NOT delete or reword existing changelog bullets or any `## vX.Y.Z` section — the new bullet is appended inside the unreleased section (or the section is created if absent). The `## v0.117.1` and `## v0.118.3` sections in particular stay byte-identical.
- The "liveness gating" phrase in the per-session lock section is spec 042's Vault UI follow-on concept — leave it; it is not the removed liveness-window concept.
- Do NOT edit `agents/work-on-task-assistant.md` — its pre-spawn contract text is a reviewer-owned follow-up (requirement 5), not this prompt's work.
- Do NOT touch `pkg/ops/*.go` or any `_test.go` file in this prompt — code changes belong to prompts 1 and 2.
- Do NOT add a config field for the turn bound (spec Non-goals / Open Question 1).
- Existing tests must still pass.
</constraints>

<verification>
Evidence greps — run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c '^## Unreleased' CHANGELOG.md                                          # >= 1 (AC12) — must flip 0 -> >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume'                    # >= 1 (AC12) — must flip 0 -> >= 1
grep -c 'only after the detached headless turn exits' CHANGELOG.md             # >= 1 if you used the suggested wording
! grep -q 'livenessWindow' docs/work-on-session-lifecycle.md                   # AC11: absent
! grep -ci 'liveness window' docs/work-on-session-lifecycle.md                 # AC11: absent (prose form too)
! grep -q '~10s' scenarios/002-task-lifecycle.md                               # AC11: absent
grep -n -m1 '^All notable changes to this project' CHANGELOG.md                # preamble present
grep -n -m1 '^## ' CHANGELOG.md                                                # must be the Unreleased heading, AFTER the preamble line above
```

CONTENT DRIFT GUARD — the real check for requirement 2. The reverted vocabulary must be gone, and the new framing present. The reworded text legitimately contains the words "compensating clear" in a negated sentence ("No compensating clear runs on any error…") and the lock section legitimately keeps "no compensating clears" — so the guard targets the STALE CLAIMS by exact phrase, not the bare two words:

```
! grep -qiE 'pre-spawn|pre-persisted|pre-persist|before the child is spawned' docs/work-on-session-lifecycle.md
! grep -q 'runs a compensating clear' docs/work-on-session-lifecycle.md
! grep -q 'The compensating clear is' docs/work-on-session-lifecycle.md
! grep -q 'the compensating clear removes' docs/work-on-session-lifecycle.md
grep -c 'only after the detached headless turn has finished cleanly' docs/work-on-session-lifecycle.md   # >= 1 (req 2, Post-exit write ordering)
grep -c 'the id is never written before the turn finishes' docs/work-on-session-lifecycle.md             # >= 1 (req 2, Failure path)
```

FULL GATE — `make precommit` at the repo root must exit 0, and the completion report must carry its actual exit code. If it fails, fix the cause and re-run only the failing target, then `make precommit` once more.

The spec's AC10 guard (`git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md`) was verified in prompt 1 and is not repeated here.
</verification>
