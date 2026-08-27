# Work-on Session Lifecycle

This document records the design decisions behind the `work-on` session-start fix
(spec 040): why `task work-on` / `goal work-on` return a session id within ~10
seconds on a non-TTY caller instead of blocking for the whole headless bootstrap
turn, and what each branch of `StartSession` does and does not guarantee.

It is decisions only — implementation details live in the code and its doc
comments. The spec's Design section is the source; this file is its durable form.

## Session id ownership

The session id is minted by the **caller**, not by `claude`. `handleClaudeSession`
in `pkg/ops/workon.go` and `pkg/ops/goal_workon.go` generates it with
`github.com/google/uuid` through the injected `uuidGenerator func() string`
dependency (the same injection pattern as `libtime.CurrentDateTime` for the clock).
`StartSession` takes the id as a parameter and returns only an error; it never
invents or substitutes an id.

`claude --session-id <uuid>` is documented by the client itself as *"Use a specific
session ID for the conversation (must be a valid UUID)"* (verified 2026-08-27). The
caller mints the id so it can be persisted to the task/goal file **before the child
process exists** — the ordering that makes the fix a guarantee rather than a race.

## Pre-spawn write ordering

The id — and, on the task path, its `metrics_sessions` entry — are persisted while
**no child exists**. `persistSessionAndMetrics` (task) and `persistGoalSessionID`
(goal) run before `StartSession` is called on the non-interactive branch, so the
session's own read-modify-write always reads a file that already contains the id.

This is the load-bearing part of the fix. Before it, the id was written *after* the
headless turn returned, and the re-read existed because the turn mutated the very
file being written. With persist-before-spawn there is no post-turn id write to
revert on the fresh-start path; the re-read remains load-bearing on the interactive
branch and the cached-session path, where the turn may still mutate the file before
the post-return persist.

## Why stream-json was rejected

`--output-format stream-json --verbose` would deliver the session id in an init
event within ~1 second, which looks like a faster answer. It was rejected: returning
early from a stream would reintroduce the exact hazard this fix removes, from the
opposite direction — the parent would stop waiting on a live child that the request
context could still kill mid-write. The liveness window waits on the child's exit,
not on a message the child emits.

## Why the TTY branch is untouched

The interactive branch keeps its blocking behaviour and its 5m cap unchanged. On a
TTY caller, turn 2 `syscall.Exec`s `claude --resume` against turn 1's on-disk
result, so the blocking wait is precisely what makes the handoff correct — turn 2
never races a live turn-1 child on the same session transcript.
`scenarios/005-work-on-resume-auto-invokes-subtask.md` regression-locks this path.
The branch is chosen by the `--mode` flag (`auto | interactive | headless`), not by
the tty: `--mode headless` from a real terminal takes the fast path, `--mode
interactive` from a pipe takes the blocking path.

## The fate of --output-format json

Spec Open Question 4 is decided: `--output-format json` is **kept on both
branches**. The interactive branch still validates the JSON blob, so the flag is
required there. On the non-interactive branch it is harmless dead weight — the
detached child's stdout goes to `os.DevNull` and nothing parses the blob — and
dropping it would add risk for no gain: it would create a second argv difference
between the branches and one more way for the two paths to diverge.

## What the liveness window does and does not cover

On the non-interactive branch `StartSession` waits for `livenessWindow` (10s,
tunable) on a channel fed by the child's `Wait`. An exit inside the window is a real
failure and returns an error naming the child's exit status. The window covers the
failure mode that actually bites: a session that dies on startup (bad flag, auth
failure). It is deliberately **not** an inactivity watchdog — a session that hangs
*after* starting is left to the Vault UI's existing `claude_session_started` cleanup
sweep, which is out of scope here.

## Compensated failure path

When the spawn fails inside the liveness window, the pre-persisted state is rolled
back: the caller re-reads the task (or goal) from disk and clears only the
`claude_session_id` and, on the task path, this run's `metrics_sessions` entry. The
clear is itself a re-read-modify-write, so any frontmatter the child wrote before
dying (for example `phase: planning` at 8s) survives. A failed clear is surfaced as
a warning rather than masking the spawn error.
