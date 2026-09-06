---
status: prompted
approved: "2026-09-06T13:07:28Z"
generating: "2026-09-06T13:43:00Z"
prompted: "2026-09-06T13:43:00Z"
branch: dark-factory/bug-exit-code-outranks-validated-turn
---

## Summary

- A headless `work-on` turn that completes successfully is discarded when its child process exits non-zero.
- `runDetachedTurn` already captures and validates the child's result JSON — but only on the clean-exit path; a non-zero exit returns before the read.
- The operator sees a ~40-line Go stack whose only content is `exit status 1`, and the task loses its `claude_session_id`.
- The completed work survives on disk as a transcript the Vault UI can no longer reach; recovery requires knowing the UUID by hand.
- The fix inverts the precedence: the validated turn result decides success, and the exit code is only consulted when there is no usable result.

## Problem

`vault-cli work-on` spawns a detached `claude --print` child, redirects its stdout to a temp file, and validates that blob through `validateSessionTurn` — a session id alone proves nothing, because `claude` reports one even for a turn that did no work. That validation is the thing that makes the persisted id trustworthy. But the exit-status branch in `runDetachedTurn` returns before the file is ever read, so a turn that produced a complete, valid result is thrown away on the strength of a number the child sets for unrelated reasons. The operator loses both the session and any explanation of why.

## Reproduction

Observed 2026-09-06 on `vault-cli v0.122.3`; the code is unchanged at `v0.124.1` (`5d2b1ed`).

1. In Vault UI, click Start/Resume on a task with no `claude_session_id` (observed: `Diagnose Early Exit on Overnight US100 Demo Trade`, Personal vault).
2. `vault-cli work-on` mints a UUID, pre-persists it, and spawns the detached child. The argv it builds (`pkg/ops/claude_session.go:193-202`) is:

   ```
   <claude_script> --print -n "<task name>" \
     -p '/vault-cli:work-on-task "<task file path>" --non-interactive' \
     --output-format json --session-id <uuid>
   ```

   with stdout redirected to a temp file and stderr to `os.DevNull`. `--max-turns` is omitted because `maxTurns` is `-1`.
3. The child runs a full turn — roughly 2 minutes — and ends cleanly.
4. The child process exits 1.

Observed evidence, verbatim from `~/Library/Logs/vault-ui.log`:

```
2026-09-06 12:59:23 INFO  [vault_ui.api.tasks:1095] Starting vault-cli session for task Diagnose Early Exit on Overnight US100 Demo Trade
2026-09-06 13:01:15 ERROR [vault_ui.api.tasks:1154] Error creating session: vault-cli work-on failed: … error="claude session exited with error: exit status 1
```

The child's own transcript, `~/.claude/projects/-Users-bborbe-Documents-Obsidian-Personal/5a1b9c38-076e-42e9-9222-adf03f5c08e4.jsonl` (87 lines):

- `"stop_reason":"end_turn"` on the final assistant message
- zero records with `"is_error":true`
- `"permission_denials":[]`
- final assistant message written in full, followed by the `last-prompt` record

Task frontmatter after the run: no `claude_session_id` key; `metrics_sessions` still lists only the prior day's session.

Ruled out during triage: claude-code-router reachable; `claude` 2.1.260 works standalone; `-n` is a valid flag; `--max-turns` inert (`maxTurns: -1`, `claude_session.go:59`); a baseline `claude --print -n … --session-id … --output-format json` from the same cwd exits 0; a session-id collision produces a different, instant failure (`Error: Session ID … is already in use.`).

### Deterministic repro without waiting for the wild failure

The cause of the non-zero exit is a Non-goal and is unreproduced, so verification must not depend on it recurring. The repo already exposes the seam: `claude_script` (`pkg/config/config.go:35`, read via `GetClaudeScript()` at `pkg/cli/cli.go:386`, resolved through `exec.LookPath` at `pkg/ops/claude_session.go:53`) lets a vault point at any executable.

Two stubs make both outcomes deterministic:

- `stub-valid-exit1` — prints a valid blob (`session_id` set, `num_turns: 3`, `is_error: false`, `result: "done"`) to stdout, then `exit 1`. This is the bug.
- `stub-error-exit1` — prints `{"session_id":"…","num_turns":0,"is_error":true,"result":"seeded failure text"}`, then `exit 1`. This is a genuine failure and must still clear the id.

Point a scratch vault's `claude_script` at each in turn and click Start.

## Expected vs Actual

**Expected** — the turn is validated by its result blob, the same contract `validateSessionTurn` enforces on the interactive branch and on the clean-exit path of the detached branch. `docs/work-on-session-lifecycle.md:80` states the reason validation exists: *"an unvalidated id would be handed to the operator as resumable when it is not."*

**Actual** — `runDetachedTurn` (`pkg/ops/claude_session.go:258-260`) returns on `exitErr != nil` before reaching the `os.ReadFile` + `validateSessionTurn` pair at lines 276-280. A valid blob sitting in the temp file is never read; the id is cleared and the operator gets `exit status 1`.

## Why this is a bug

`docs/work-on-session-lifecycle.md:100` documents the current behavior as intended: *"Expiry, ctx cancellation, and a non-zero child exit all return an error, so the caller persists nothing."* So this is a contract defect, not code drifting from its spec — the documented rule is itself wrong, and the doc must change with the code.

The doc's own justification is what condemns it. It defends validation on the grounds that offering a Resume that cannot work is a lie to the operator. Discarding a session that *can* be resumed is the mirror image of that lie, and it is the more expensive one: the false-positive costs a failed `claude --resume`, while this costs the entire turn.

The exit code is also the weaker signal by construction. `claude`'s stderr goes to `os.DevNull`, so the exit status arrives with no accompanying explanation, while the result blob is a structured document the code already knows how to validate. Trusting the opaque signal over the structured one inverts the precedence.

## Workaround

Until the fix lands, a stranded session is recoverable by hand:

```bash
ls -t ~/.claude/projects/<cwd-slug>/*.jsonl | head -1        # newest transcript = the stranded session
vault-cli task set "<task name>" claude_session_id <uuid>    # restore the id the clear removed
```

The `<cwd-slug>` is the vault path with `/` replaced by `-`, e.g. `-Users-bborbe-Documents-Obsidian-Personal`.

## Goal

A headless turn is judged by what it produced, not by how its process happened to exit. When the captured result JSON validates, the session id persists and the Vault UI offers Resume. When it does not, the error names the child's own reason instead of an exit status, and the compensating clear runs exactly as it does today.

## Non-goals

- Root-causing why a clean-`end_turn` child exits 1 at all. The leading suspect is a non-async `Stop` hook (`afplay`) in a detached process with no audio session, but it is untested and irrelevant to this fix: vault-cli must not depend on the exit code being trustworthy.
- Any change to the interactive TTY branch's **control flow** — `cmd.Output()`, the synchronous validation, the 5m cap, the blocking wait. Its error message *text* does change, because AC 2 requires the child's reason to lead and Constraints forbid forking the shared `validateSessionTurn` (called at `claude_session.go:221` interactive and `:281` detached). Rewording the shared validator is the only implementation reachable under both rules; two interactive-branch assertions are updated accordingly.
- Removing or weakening the compensating clear. This spec narrows *when* a turn counts as failed; a genuinely failed turn must still clear the id.
- Vault UI banner styling. Only the message content changes.

## Acceptance Criteria

- [ ] A child that writes a valid result blob (`session_id` non-empty, `num_turns > 0`, `is_error: false`) and exits non-zero causes `runDetachedTurn` to return `nil` — evidence: unit test asserts `err == nil` for that input pair; `make precommit` exits 0.
- [ ] A child that writes a **non-empty blob that parses but fails a predicate** (`is_error: true`, `num_turns: 0`, or empty `session_id`) returns an error that leads with the blob's `result` text and names the failed predicate — evidence: unit test asserts `err.Error()` has the seeded `result` string as a prefix AND contains one of `is_error` / `num_turns` / `session_id`. An exit-status mention may follow as a trailing clause, but must not precede the child's own reason.
- [ ] A child that exits non-zero AND leaves a **zero-length, unreadable, or non-empty-but-unparseable** output file returns an error naming the exit status — evidence: unit test asserts the message contains `exit status`. This is the only path where the exit code is authoritative; unparseable bytes land here rather than under the predicate case, because there is no `result` text to surface.
- [ ] A turn that hits `sessionTurnTimeout` while a valid blob is already present in the output file still returns an error — evidence: unit test seeds a valid blob, fires the timeout path, asserts `err != nil` and the message contains `did not complete within`. This locks DB 5 against the hoist-the-read refactor.
- [ ] Negative — the compensating clear still fires for a genuinely failed turn, on **both** the task path and the goal path: after `Execute` against a child seeded with `is_error: true`, `grep -c '^claude_session_id:' <task file>` returns 0 — evidence: `workon_session_writeback_test.go` and `goal_workon_test.go` each extended, asserting zero matches.
- [ ] Negative — a task or goal whose turn validates retains its id: after `Execute` against a valid-blob/non-zero-exit child, `grep -c '^claude_session_id:' <file>` returns 1 — evidence: same two test files, opposite assertion. `goal_workon.go:198` has its own `handleClaudeSession` and must not be left behind.
- [ ] The stale contract sentence is gone from the lifecycle doc AND replaced by one stating result-over-exit-code precedence — evidence: `grep -c 'a non-zero child exit all return an error' docs/work-on-session-lifecycle.md` returns 0, AND `grep -c 'validated result outranks the exit code' docs/work-on-session-lifecycle.md` returns ≥1.
- [ ] `CHANGELOG.md`'s topmost section carries a bullet naming the precedence change — evidence: `awk '/^## /{n++} n==1' CHANGELOG.md | grep -c 'validated result outranks the exit code'` returns ≥1. Heading-independent on purpose: `.maintainer.yaml` sets `autoRelease: true` and `.dark-factory.yaml` sets `pr: false`, so the release bot can rename `## Unreleased` to `## vX.Y.Z` between prompts and a heading-pinned grep would fail a correct implementation.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint, format, generate, test, checks; exits 0
- `grep -c 'a non-zero child exit all return an error' docs/work-on-session-lifecycle.md` — returns 0 (stale contract sentence deleted)
- `grep -c 'validated result outranks the exit code' docs/work-on-session-lifecycle.md` — returns ≥1 (replacement sentence present)
- `grep -n 'claude session exited with error' pkg/ops/claude_session.go` — returns exactly one line
- `awk '/^## /{n++} n==1' CHANGELOG.md | grep -c 'validated result outranks the exit code'` — returns ≥1

### Operator-executable (runs on the host after PR merge)

These two bullets are deliberately **not** Acceptance Criteria: they observe the Vault UI, which no container-executable check can reach, and the AC set is intentionally all container-executable so `spec-verifier` gates on evidence the pipeline can produce. They remain the operator's proof of the user-visible payoff. Both use the `claude_script` stubs from the Reproduction section, so neither waits on the wild failure:

- Scratch vault pointed at `stub-valid-exit1`, Vault UI Start: card reaches `▶ Resume`, no red banner, and `grep -c '^claude_session_id:' "<task file>"` returns 1
- Scratch vault pointed at `stub-error-exit1`, Vault UI Start: banner's first line contains `seeded failure text` and not `exit status`, and `grep -c '^claude_session_id:' "<task file>"` returns 0

## Desired Behavior

1. `runDetachedTurn` reads the captured output file on every path where the child has actually exited, before the file is unlinked.
2. When the blob validates, the turn is a success — the non-zero exit is not surfaced as an error.
3. When the blob is present but invalid, the returned error carries the blob's `result` text and names the failed predicate, so the operator learns the child's own reason.
4. When the blob is absent or unreadable and the child exited non-zero, the error names the exit status — the sole remaining case where the exit code decides.
5. The timeout and ctx-cancellation paths are unchanged and must NOT read the blob: the child is still running, so any bytes present are partial by definition and must never be validated as success.
6. `handleClaudeSession`'s compensating clear continues to fire on every error `runDetachedTurn` returns — the behavior change lives entirely in what counts as an error.

## Constraints

- `validateSessionTurn` stays the single shared validator for both branches; do not fork its logic.
- The read must stay inside the child-exited branch. Hoisting it above the `select` is the specific refactor DB 5 and its AC forbid.
- The temp file must still be unlinked on every return path, including cancel and timeout where the child holds the fd.
- Stderr still goes to `os.DevNull`.
- The interactive branch's `cmd.Output()` + `validateSessionTurn` sequence must not change.
- Tests use Ginkgo v2 + Gomega with Counterfeiter mocks; no stdlib `t.Run` table tests.
- Errors wrap via `github.com/bborbe/errors` with a real `ctx` — no `fmt.Errorf`, no bare `return err`.
- The existing `exit status 1` assertions must keep passing unchanged: `claude_session_test.go:433`, `goal_workon_test.go:537`, `workon_session_writeback_test.go:339`. Their `detachRun` stubs ignore the `*os.File` parameter, so they exercise the zero-length-output path and encode the AC-3 contract. If a change makes one of them fail, the empty-file case has been misrouted to the predicate branch.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery |
|---|---|---|---|
| Child exits non-zero, blob valid | Success; id persists | Vault UI shows `▶ Resume` | None needed |
| Child exits non-zero, blob non-empty and parses but fails a predicate | Error leading with the blob's `result` and failed predicate; id cleared | Banner first line is the child's reason | Operator fixes the named cause, re-runs Start, confirms `▶ Resume` |
| Child exits non-zero, output file zero-length, unreadable, or unparseable | Error naming the exit status; id cleared | Banner says `exit status N` | Run the Workaround commands: newest transcript under `~/.claude/projects/<slug>/`, then `vault-cli task set … claude_session_id <uuid>`; confirm `grep -c '^claude_session_id:'` returns 1 |
| Output file unreadable (fd/permission failure) after child exit | Error wrapping the read failure; id cleared | Banner names the read error, not the exit status | Re-run Start; if it recurs, `ls -ld $TMPDIR` and confirm the temp dir is writable |
| `claude` changes its `--output-format json` shape | `validateSessionTurn` rejects every turn; no id ever persists | Every Start banner reports a parse or predicate failure with the same text | Pin the parser to the fields it needs and add the new shape; under the new precedence this validator is the only remaining gate, so a drift here fails closed, not silently |
| Turn timeout (30m) expires with child still running | Unchanged — error, id cleared, blob NOT read | Banner names the timeout | Unchanged; the detached child keeps running and its transcript is recoverable via the Workaround |
| ctx cancelled mid-turn | Unchanged — error, id cleared, blob NOT read | Banner names the cancellation | Unchanged |
| Two Start clicks race the same task | Unchanged — the per-session lock refuses the second before any child spawns | Second click errors immediately | Wait for the first turn to finish |

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `runDetachedTurn` precedence inversion + unit tests for the four exit/blob combinations | 1, 2, 3, 4, 5 | 1, 2, 3, 4 | — |
| 2 | `workon` / `goal_workon` writeback tests for clear-vs-retain | 6 | 5, 6 | prompt 1 |
| 3 | Lifecycle doc contract rewrite + CHANGELOG bullet | — | 7, 8 | prompt 1 |

Rationale: prompt 1 carries the whole behavior change and its direct tests, including the timeout guard that prevents the plausible regression; prompt 2 proves the caller-side consequence on real task files; prompt 3 is text-only and depends on prompt 1 only so the doc describes what actually shipped.

## Do-Nothing Option

Every Vault UI Start whose child trips this exit-code path keeps costing a complete agentic turn plus the operator time to work out that nothing is actually wrong. The error surface stays a Go stack with no diagnostic content, so each occurrence is re-investigated from scratch — this one cost roughly forty minutes. The stranded transcripts are recoverable only by an operator who knows the Workaround exists, which means in practice they are lost. Doing nothing also leaves the exit code trusted over a validated result, so the next unrelated cause of a non-zero exit produces the same silent loss.
