---
status: approved
tags:
    - dark-factory
    - spec
approved: "2026-11-22T14:38:01Z"
branch: dark-factory/bug-work-on-trailing-bytes
---

## Summary

- A headless turn is validated by parsing the **whole** captured stdout file as one JSON document. Any byte after the turn's JSON object makes the entire blob unparseable, so a turn that completed successfully is thrown away.
- Observed 2026-11-22: `vault-cli task work-on "<task>" --mode headless` ran a 182-line worker turn, then failed with `parse claude output: invalid character '-' after top-level value` and exited 1.
- The loss is silent: the compensating clear removes `claude_session_id`, the phase stays `planning`, and no worker progress exists — while the child had already run and exited. The task looks untouched.
- Fix: accept a turn whose captured stdout **begins** with a valid turn JSON object, whatever follows it. A blob with no valid JSON at its start still fails as unparseable, so the existing failure handling and the compensating clear are unchanged.

## Goal

After this work, a completed headless turn is accepted whenever its captured stdout begins with a valid turn JSON object, whatever follows it; the session id persists, and the compensating clear no longer fires on a merely-suffixed blob. A blob with no valid JSON at its start still fails as unparseable and still clears the id.

## Desired Behavior

1. A turn whose captured stdout is a valid turn JSON object followed by `\n---\n` validates; the session id persists and the caller reports success.
2. A blob with no valid JSON at its start still returns the unparseable sentinel, and `errors.Is` matches it, so `runDetachedTurn` still falls back to the child's exit status.
3. A truncated JSON prefix (valid start, no closing brace) still returns the unparseable sentinel.
4. The three predicates — empty `session_id`, `num_turns: 0`, `is_error: true` — still reject, with the child's own `result` text leading the message.
5. A genuinely failed turn still triggers the compensating clear.

## Problem

A headless turn's result is written by the child into a temp file, read back after the child exits, and validated once (`runDetachedTurn`). The validator is shared by the interactive and detached branches, so both inherit the same strictness.

Whole-buffer unmarshalling requires the entire input to be one JSON value. Claude Code's `--print --output-format json` normally emits exactly that (a trivial probe on 2026-11-22 produced a single 1,723-byte JSON object with no trailing bytes), but the output file is the child's stdout fd: anything else writing to that fd appends bytes after the JSON object. The reader then rejects the whole turn even though the turn JSON itself is complete and valid.

The result is a turn-level false negative with a destructive side effect: the compensating clear treats "unparseable" as "nothing happened", removes the session id, and hands the operator a task that looks like it was never started.

## Assumptions

- The trailing bytes come from something other than the turn JSON itself. Identifying that producer is **out of scope** — the reader must tolerate any suffix.
- The temp file is complete when it is read: the read happens after the child exited and its fd closed.
- Decoding one value from the start of the buffer gives the required prefix semantics without a manual offset scan.
- **Alternatives considered and rejected:** trimming at the last `}` (fails on a nested-object suffix and on trailing `{`), scanning for the first `}` (same class of failure, plus no structure awareness), and a byte-offset scan for the value end (re-implements the decoder). Decoding exactly one value is the smallest correct shape.

## Reproduction

### Observed (production, recorded)

2026-11-22T10:06, `vault-cli task work-on "Headless Work-On Keeps Its Session When the Turn Output Has Trailing Bytes" --mode headless`:

```
Error: start work-on session: start claude session: parse claude output: invalid character '-' after top-level value: claude output is not valid turn JSON
EXIT: 1
```

Child session `4d5366b7-fa71-4634-a433-0c7a2a66588a` ran one turn (182 transcript lines) and exited. Effects: `claude_session_id` cleared, `phase` unchanged, no worker progress. The transient `/private/tmp` capture is gone; the error string and the transcript are the durable record.

### Deterministic (unit)

`validateSessionTurn(ctx, []byte(`{"session_id":"s","num_turns":1,"is_error":false,"result":"done"}` + "\n---\n"))` returns `parse claude output: invalid character '-' after top-level value` today, and `nil` after the fix.

### Control

A trivial turn (`claude --print -p "Reply with OK" --output-format json --max-turns 1`, 2026-11-22) produced clean JSON with no suffix. A spawn that only ever sees clean JSON therefore cannot demonstrate the fix — the acceptance test must inject the suffix.

## Why this is a bug

The validator's contract, as documented in `docs/work-on-session-lifecycle.md` § The fate of `--output-format json`, is "did the turn produce a usable result?" — not "is stdout byte-exactly one JSON document". The turn's result is fully readable in the failing case; rejecting it discards work the system already paid for and reports the task as untouched. The same defect hits every task and every repo.

## Non-goals

- Changing what the child prints, or finding and fixing the trailing-byte producer. The reader must be tolerant regardless.
- Changing the 30-minute turn bound or the detached-child lifecycle.
- Changing the supervisor wrapper's spawn path or its `answer-path` reach record.
- Relaxing the three predicates or the compensating clear.
- Hardening against a JSON object written **before** the child's output (see Failure Modes — first document wins, unchanged from today's "whole blob must parse" rule).

## Do-Nothing Option

Leave it. Every affected spawn loses its turn silently and leaves the task looking untouched; the manager cannot start the work it is responsible for driving. The workaround is to re-run the spawn until a clean-JSON turn happens, which is neither observable nor bounded.

## Constraints

- The unparseable sentinel must remain the wrapped cause, because `runDetachedTurn` branches on `errors.Is(validateErr, errClaudeOutputUnparseable)` to decide whether the child's exit status may lead the message. Wrapping the raw decode error instead silently restores the old behaviour of reporting a bare exit status for output that parsed.
- The output file must not be read before the child exits — the existing ordering comment in `runDetachedTurn` is load-bearing and stays.
- `make precommit` must pass; it is the repo's preflight.

## Design

Behavioural requirement: decode exactly one JSON value from the start of the buffer into the turn-result struct, ignore any suffix, and route a decode failure through the unparseable sentinel exactly as today. Tests: the trailing-bytes fixture validates; the existing empty-file fixture still fails with the sentinel; a truncated prefix still fails with the sentinel; the three predicate cases stay green. No code is prescribed beyond that.

## Acceptance Criteria

- [ ] A turn whose captured stdout is a valid turn JSON object followed by `\n---\n` validates — evidence: `pkg/ops/claude_session_test.go` gains a fixture built from a **nested** turn object (`{"session_id":"s","num_turns":1,"is_error":false,"result":"done","usage":{"input_tokens":5}}`) plus `"\n---\n"`, asserting `StartSession` returns nil. The nested shape is required: a flat fixture lets a cut-at-the-first-`}` hack pass while truncating real nested claude output.
- [ ] A suffix that does **not** start with a newline is also tolerated — evidence: a second fixture appending `\n---\n{}` to a valid turn object returns nil. The brace-bearing suffix is required: it defeats a last-`}` trim, which would otherwise pass every other fixture.
- [ ] A blob with no valid JSON at its start still returns the unparseable sentinel — evidence: `pkg/ops/export_test.go` exports the sentinel (`var ErrClaudeOutputUnparseable = errClaudeOutputUnparseable`); a fixture whose child **exits 0** writing an empty blob asserts `errors.Is(err, ops.ErrClaudeOutputUnparseable)` and that `err.Error()` contains `parse claude output`. The exit-0 child is required: a non-zero exit returns the exit-status error instead of the sentinel.
- [ ] A truncated JSON prefix still returns the unparseable sentinel — evidence: the same exported sentinel, with a child exiting 0 writing `{"session_id":"s","num_turns":1` (no closing brace), asserts `errors.Is` matches.
- [ ] Empty `session_id`, `num_turns: 0`, and `is_error: true` each still reject, with the child's own `result` text leading the message — evidence: the three detached-branch predicate cases stay green and each asserts `HavePrefix` on the fixture's `result` text.
- [ ] A genuinely failed turn still triggers the compensating clear — evidence: `pkg/ops/workon_session_writeback_test.go`'s existing compensating-clear case stays green, asserting `claude_session_id` is empty after the failed spawn.
- [ ] `CHANGELOG.md` carries a bullet starting `- fix:` that names trailing bytes — evidence: `awk '/^## /{sec=$0} /trailing/{print sec}' CHANGELOG.md` prints `## Unreleased`, placed per `docs/dod.md` (below the preamble, above the newest `## vX.Y.Z`); create `## Unreleased` above the newest version section if absent — the releaser renames it on master. On a branch where the release already cut, the same awk prints the newest `## vX.Y.Z`.
- [ ] `make precommit` exits 0 at the fix commit — evidence: exit code.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery |
|---|---|---|---|
| Trailing bytes after the turn JSON (this bug) | Turn validates; id persists | Unit fixture `validTurnJSON + "\n---\n"` returns nil | None needed — the fix |
| No valid JSON at the start of the buffer | Unparseable sentinel returned; `errors.Is` matches; compensating clear fires | Unit fixture with an empty/garbage blob | `vault-cli task work-on "<task>" --mode headless` exits 0 and `claude_session_id` is populated, after the underlying cause is fixed |
| Truncated document (child died mid-write) | Unparseable sentinel returned; id cleared | Unit fixture with an unterminated object | `vault-cli task work-on "<task>" --mode headless` exits 0 and `claude_session_id` is populated on a re-run |
| JSON object written **before** claude's output | First document wins; a forged `session_id` would be accepted and persisted | Not detectable from the buffer alone; no known producer writes to the child's stdout before claude | Out of scope; a future change can assert the decoded id equals the minted id |
| Buffer is the JSON literal `null` | Decodes without error into a zero struct, then rejects via the empty-`session_id` predicate | Predicate test | None needed — still a rejection |
| Buffer is a JSON scalar (`true`, a number, a string) | Type error from the decode; unparseable sentinel returned | Unit fixture | None needed |
| Claude changes its output shape (leading non-JSON line, renamed fields) | Still rejected — sentinel or a predicate — and the id is cleared. No leniency is added for unknown shapes | The first real spawn after an upgrade fails with `parse claude output` | Investigate the shape change; do not widen the parser to accept anything |
| Decode error wrapped with the raw error instead of the sentinel | `runDetachedTurn` reports a bare exit status for output that parsed | `errors.Is` assertion in the sentinel test | Restore the sentinel as the wrapped cause |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Accept a suffixed turn result and lock it with tests | DB 1–5 | AC 1–8 | — |

## Verification

1. Unit: `make precommit` in the prompt worktree — the new fixtures pass, the sentinel cases still fail as expected, predicates green.
2. End to end: with `claude_script` unset (so the bare name resolves on `PATH`), put a stub `claude` earlier on `PATH` that writes a valid turn JSON plus a trailing `\n---\n` and exits 0, then run `vault-cli task work-on "<a scratch approved task>" --mode headless`; expect exit 0 and `claude_session_id` populated in the task frontmatter. Run the same command with a stub writing clean JSON as the control — a clean-JSON spawn does not count as proof. (No real `claude` invocation is needed: the producer of the suffix is out of scope, so the stub is the only reproducible way to inject it.)
3. Deployed: `vault-cli --version` on the installed binary equals the released tag produced by the merge, so the binary exercised in step 2 is the one that shipped.
4. Docs: `docs/work-on-session-lifecycle.md` § The fate of `--output-format json` states the new "begins with a valid turn JSON object" rule.

## Security / Abuse Cases

The only abuse surface is the child's stdout buffer. A forged JSON prefix — an object written to the fd before claude's own output — would win the decode and could persist a chosen `session_id`. That risk is accepted and recorded in Failure Modes and Non-goals: nothing in the current spawn path writes to the child's stdout before claude, and asserting the decoded id equals the minted id is a separate change. The fix introduces no credential, network, or caller-supplied path.

## Open Questions

- None blocking. The trailing-byte producer stays unidentified by design.
