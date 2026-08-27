---
status: completed
spec: [040-bug-session-start-blocks-on-full-headless-turn]
summary: 'Created docs/work-on-session-lifecycle.md (7 sections: session-id ownership, pre-spawn ordering, stream-json rejection, TTY branch, --output-format json fate, liveness window scope, compensated failure path), corrected the stale blocking-behaviour doc comments in pkg/ops (AC12 grep now 0 lines), updated scenarios/002 for the ~10s non-TTY return with a ~/.claude/projects jsonl transcript check, and appended the docs: CHANGELOG bullet'
execution_id: vault-cli-session-fast-return-exec-201-spec-040-docs-and-scenario
dark-factory-version: dev
created: "2026-08-27T12:00:00Z"
queued: "2026-08-27T10:51:51Z"
started: "2026-08-27T12:01:33Z"
completed: "2026-08-27T12:07:28Z"
branch: dark-factory/bug-session-start-blocks-on-full-headless-turn
---

<summary>
- A new design document explains why the session-start fix works the way it does: who is responsible for minting the session id, the write ordering that prevents losing work already done, why an obvious-looking alternative approach was rejected, why the terminal-based workflow was deliberately left alone, and what the new startup safety check does and does not catch.
- Code comments that still describe the old behaviour — where starting a session waited for the whole background job to finish — are corrected to describe the new fast return.
- The task-lifecycle test scenario is updated to expect the new fast return instead of the old multi-minute wait, and gains an automated check that catches a session id which does not correspond to a real session.
- The scenario covering the unaffected terminal-based workflow is left completely untouched, so it keeps guarding that path.
- A changelog entry records the documentation and scenario work for the next release.
</summary>

<objective>
After this prompt, the session-start fix has a durable written record: the design rationale lives in a project document rather than only in the spec, every code comment that still describes the old blocking behaviour matches the new contract, and the operator-facing task-lifecycle scenario reflects the new timing instead of a misleading multi-minute wait. Covers spec 040 AC9 / AC12 / AC13 / AC14.
</objective>

<context>
Read `/workspace/CLAUDE.md` (the Scenario-skip rule and the release checklist sections) and `/workspace/docs/dod.md` first. Then read fully:

- `/workspace/pkg/ops/claude_session.go` — the new `StartSession` from prompt 1 (non-interactive detached branch, interactive blocking branch, `livenessWindow`, `--session-id <id>`, `--output-format json` kept on both branches). AC13 requires a comment here referencing `docs/work-on-session-lifecycle.md`.
- `/workspace/pkg/ops/workon.go` and `/workspace/pkg/ops/goal_workon.go` — the four stale comments AC12 removes. Verified current lines:
  - `workon.go` line 187 (inside the `persistSessionAndMetrics` doc comment): "the StartSession call blocks for the entire headless turn, and that turn writes to this very task file..."
  - `workon.go` line 218 (above `handleClaudeSession`): "On a fresh start it re-reads the task from disk after the session returns, so frontmatter the session itself wrote during the blocking turn survives."
  - `goal_workon.go` line 164 (inside the `persistGoalSessionID` doc comment): "Used after StartSession blocks so that frontmatter the headless turn wrote is not reverted..."
  - `goal_workon.go` line 186 (above `handleClaudeSession`): "On a fresh start it re-reads the goal from disk after the session returns, so frontmatter the session itself wrote during the blocking turn survives."
- `/workspace/scenarios/002-task-lifecycle.md` — the `task work-on` step at line 31-33 whose note says "expect no output at all for ~2-3 minutes (measured 2m49s) ... Allow ≥300s. A short timeout kills it at exit 124..." and the Expected block that must gain the jsonl check. AC9's evidence greps: `grep -c '300s'` == 0, `grep -c '2m49s'` == 0 (the misleading note carries both, plus `2-3 minutes` and `exit 124`, all on one line — replace the whole note, a partial rewrite dropping only "300s" must not pass), `grep -c 'jsonl'` >= 1.
- `/workspace/scenarios/005-work-on-resume-auto-invokes-subtask.md` — do NOT touch.
- `/workspace/CHANGELOG.md` — has `## Unreleased` from prompts 1-3; append one more bullet.

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md` — structure for the design doc.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — bullet format.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comments start with the name, full sentences, behavior not implementation.
</context>

<requirements>
1. Create `/workspace/docs/work-on-session-lifecycle.md`. It is the durable version of the spec's Design section — decisions only. It must have at least 4 `## ` headings (AC13 `grep -c '^## '` >= 4) and cover, each with real content (not just headings):
   - Who owns the session id: the caller mints it (`github.com/google/uuid` via the injected generator on `workOnOperation`/`goalWorkOnOperation`); `StartSession` takes it as a parameter and returns only an error; `claude --session-id <uuid>` documents "Use a specific session ID for the conversation (must be a valid UUID)".
   - Why the pre-spawn write ordering exists: the id (and the task path's metrics entry) are persisted while no child exists so the session's own read-modify-write always reads a file that already contains the id; the write ordering is a guarantee, not a race.
   - Why `stream-json` was rejected: returning early from an init event would reintroduce the same hazard from the opposite direction.
   - Why the TTY branch is untouched: turn 2 `syscall.Exec`s `claude --resume` against turn 1's on-disk result, so the blocking wait is what makes the handoff correct; `scenarios/005` regression-locks it.
   - The fate of `--output-format json` on the non-interactive branch (spec Open Question 4, decided): it is KEPT on both branches — it is required by the interactive branch's JSON validation, and on the non-interactive branch stdout goes to `os.DevNull` so it is harmless dead weight; dropping it would add risk for no gain. State this explicitly, do not leave it implicit.
   - What the liveness window does and does not cover: it catches a session that dies on startup (bad flag, auth failure); a session that hangs after starting is left to the Vault UI's existing `claude_session_started` cleanup sweep (out of scope).
   - The compensated failure path: a spawn failure inside the window clears the pre-persisted id (and task metrics entry) via a re-read-modify-write that preserves the child's frontmatter writes.
   The doc must mention `os.DevNull`, `stream-json`, `output-format json`, and `TTY` (each with `grep -n` returning >= 1 line per AC13).

2. Correct the stale comments (AC12). The final state must satisfy `grep -rniE 'blocks? for the entire|after StartSession blocks|blocking turn' pkg/ops/` returning 0 lines, so rewrite the four lines listed in Context:
   - In `/workspace/pkg/ops/workon.go`, update the `persistSessionAndMetrics` doc comment: the re-read is now load-bearing on the interactive branch and the cached path (the turn may mutate the file before the post-return persist), while on the non-interactive branch the persist runs before the child exists.
   - Update the `handleClaudeSession` doc comment (workon.go): on a fresh non-interactive start the id is persisted before the child starts so the session's own read-modify-write reads a file that already contains it; on spawn failure a re-read-based compensating clear preserves the child's writes.
   - Mirror both in `/workspace/pkg/ops/goal_workon.go` (`persistGoalSessionID` and `handleClaudeSession`).
   - Update the `StartSession` interface doc comment in `/workspace/pkg/ops/claude_session.go` so it describes the new contract and ends by referencing the doc: the comment must contain the literal text `work-on-session-lifecycle` (AC13 `grep -n 'work-on-session-lifecycle' pkg/ops/claude_session.go` >= 1), e.g. "... See docs/work-on-session-lifecycle.md."
   - Do NOT change any behavior or signature in this prompt — comments and docs only.

3. Update `/workspace/scenarios/002-task-lifecycle.md` (AC9):
   - Replace the entire note under the `task work-on` step (the one carrying `2-3 minutes`, `2m49s`, `300s`, and `exit 124`) with one that reflects the new timing, e.g.:
     > Spawns a real headless `claude --print` turn. On a non-TTY caller (CI, an agent shell, a pipe) the CLI now returns within ~10s (the liveness window) with `✅ Now working on: …` and `session_id: …`; the bootstrap turn continues after the CLI exits. Run the session-lifecycle check in Expected below. TTY callers (a real terminal) still block through the turn and hand you the interactive session.
   - In the Expected block, add a check for the session transcript on disk, which is the automated lock for Failure Modes row 7 (a `claude` that silently ignores `--session-id` and mints its own leaves no matching transcript):
     ```
     - [ ] `claude_session_id` in the task file is non-empty, and a transcript file named `<that id>.jsonl` exists under `~/.claude/projects/<encoded-cwd>/`. Resolve `<encoded-cwd>` the way the installed `claude` client does for the directory `work-on` was invoked in; the exact shell is agent-decided. This is the check that catches a `claude` build which accepts `--session-id` but silently mints its own (spec 040 Failure Modes row 7).
     ```
     (Make the check concrete and runnable; the exact shell is agent-decided but must reference the `<uuid>.jsonl` file under `~/.claude/projects/`.)
   - Evidence after the edit: `grep -c '300s' scenarios/002-task-lifecycle.md` == 0, `grep -c '2m49s' scenarios/002-task-lifecycle.md` == 0, `grep -c 'jsonl' scenarios/002-task-lifecycle.md` >= 1.

4. Append a `docs:` bullet under the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (append, do not replace):
    ```
    - docs: document the work-on session lifecycle (`docs/work-on-session-lifecycle.md`) — caller-minted session id, pre-spawn persist ordering, why `stream-json` was rejected, why the TTY branch blocks, the fate of `--output-format json` on the detached branch, and the liveness window's scope — and update `scenarios/002` for the fast non-TTY return (returns within ~10s with `session_id:`, turn continues after CLI exit, `~/.claude/projects/<encoded-cwd>/<uuid>.jsonl` existence check added)
    ```

5. Do NOT touch: `scenarios/005-work-on-resume-auto-invokes-subtask.md` (AC3 — must stay byte-identical), `pkg/ops/workon.go` / `pkg/ops/goal_workon.go` behavior (comments only), any test file, or the version fields in `.claude-plugin/` (the `github-releaser` owns bumps and tags).
</requirements>

<constraints>
- Comments and docs only in this prompt: no behavior, signature, or test changes. If you find a doc comment that still describes removed behavior beyond the four pinned lines, fix it only if it falls under the AC12 grep patterns — otherwise leave it (scope).
- `scenarios/002` is `status: active` and must remain runnable; the new expected timing is a ~10s return with `session_id:` present, and the jsonl check must be executable by an operator (not a unit test).
- `scenarios/005` must remain valid, unmodified — it is the regression lock on the TTY branch.
- This repo is `autoRelease: true`. Append the `## Unreleased` bullet only; do NOT bump version fields or tag.
- Do NOT commit — dark-factory handles git. `.git` is visible in this container (`workflow: direct`, no `hideGit`).
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

The design doc exists with real content (AC13):

```
grep -c '^## ' docs/work-on-session-lifecycle.md                        # must be >= 4
grep -n 'os.DevNull' docs/work-on-session-lifecycle.md                  # >= 1 line
grep -n 'stream-json' docs/work-on-session-lifecycle.md                 # >= 1 line
grep -n 'output-format json' docs/work-on-session-lifecycle.md          # >= 1 line
grep -n 'TTY' docs/work-on-session-lifecycle.md                         # >= 1 line
grep -n 'work-on-session-lifecycle' pkg/ops/claude_session.go           # >= 1 line (comment reference)
```

The stale comments are gone (AC12):

```
grep -rniE 'blocks? for the entire|after StartSession blocks|blocking turn' pkg/ops/   # must print nothing (exit 1)
```

Scenario 002 reflects the new timing (AC9):

```
grep -c '300s' scenarios/002-task-lifecycle.md     # must be 0
grep -c '2m49s' scenarios/002-task-lifecycle.md    # must be 0
grep -c 'jsonl' scenarios/002-task-lifecycle.md    # must be >= 1
```

Scenario 005 is untouched (AC3):

```
git diff --exit-code scenarios/005-work-on-resume-auto-invokes-subtask.md   # must exit 0
```

Changelog (AC14):

```
grep -A5 '^## Unreleased' CHANGELOG.md    # must show >= 1 non-empty bullet
grep -c 'document the work-on session lifecycle' CHANGELOG.md   # must be 1
```

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.

Before finishing, re-run every command in this block and confirm each passes, then walk spec 040's AC9, AC12, AC13 and AC14 one at a time against the actual change and state which edit satisfies each. Do not report success on any criterion whose evidence you have not run.
</verification>
