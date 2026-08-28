# Work-on Session Lifecycle

This document records the design decisions behind the `work-on` session-start fix
(spec 040, revised by spec 041): why `task work-on` / `goal work-on` persist a
session id only once the headless bootstrap turn has finished on a non-TTY caller,
and what each branch of `StartSession` does and does not guarantee.

Spec 040 originally had the non-interactive branch return within ~10 seconds while
the turn kept running. That shipped a worse bug: the Vault UI offers **Resume** as
soon as `claude_session_id` appears, so it advertised a session whose transcript was
still being written — `claude --resume` failed, and a second writer could land on the
same jsonl. Spec 041 inverted it. **An id on disk now means the session is
resumable**, not merely that one exists.

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
caller mints the id so it can be passed to the child and correlated with the
transcript the child writes.

## Post-exit write ordering

The id — and, on the task path, its `metrics_sessions` entry — are persisted only
**after the child has exited**. On the non-interactive branch `StartSession` is
called first and `persistSessionAndMetrics` (task) / `persistGoalSessionID` (goal)
run only when it returns cleanly.

This is the load-bearing part of the fix, and it is what makes the id trustworthy:
the id is the UI's signal that Resume will work, so it must not appear while a child
still holds the transcript. Writing after the child exits is also race-free — there
is no concurrent writer left — and the re-read before writing is load-bearing on
**every** branch, because the turn mutates the same file it is being written to.

On any failure — child exit error, invalid turn JSON, bound expiry, ctx cancel —
nothing is persisted. There is no compensating clear because there is nothing to
undo, and the task keeps whatever frontmatter the child wrote.

## Why stream-json was rejected

`--output-format stream-json --verbose` would deliver the session id in an init
event within ~1 second, which looks like a faster answer. It was rejected: returning
early from a stream would reintroduce the exact hazard this fix removes, from the
opposite direction — the parent would stop waiting on a live child that the request
context could still kill mid-write. The non-interactive branch waits on the child's
exit, not on a message the child emits — and an init event says only that a session
started, which is exactly the claim that proved untrustworthy.

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

`--output-format json` is **kept on both branches**, and on both it is now
load-bearing. The interactive branch validates the blob from `cmd.Output()`. The
non-interactive branch redirects the detached child's stdout to a caller-owned temp
file and validates the same blob after the child exits, through the shared
`validateSessionTurn` helper.

Validation is not optional. `claude` reports a `session_id` even for a turn that did
no work or failed outright, so an unvalidated id would be handed to the operator as
resumable when it is not — the same class of lie this fix exists to remove. A turn
whose result is `num_turns: 0`, `is_error: true`, or unparseable is an error, and no
id is persisted.

A temp **file** rather than a pipe is deliberate: the child writes to an inherited fd
with no reader, so there is no pipe-buffer deadlock and no EPIPE if the parent goes
away, and the file is complete once `cmd.Wait()` returns. It is unlinked eagerly, so
no path — including cancel and timeout, where the child still holds the fd — leaves
anything behind. Stderr still goes to `os.DevNull`; a crash surfaces via exit code.

## What the turn timeout does and does not cover

On the non-interactive branch `StartSession` blocks on a channel fed by the child's
`Wait`, bounded by `sessionTurnTimeout` (30m, tunable). The bound is a **wait bound,
never a kill**: the child is detached in its own process group and survives expiry —
the parent only stops waiting. `--max-turns` is inert (`maxTurns` is -1), so a
legitimate agentic chain can run for minutes; 30m is roughly 6-10x the observed turn
length, chosen to bound a pathological hang without cutting off normal work.

Expiry, ctx cancellation, and a non-zero child exit all return an error, so the
caller persists nothing and the UI keeps showing **Start** rather than offering a
Resume that cannot work. This is deliberately **not** an inactivity watchdog — a
session that hangs after starting is left to the Vault UI's existing
`claude_session_started` cleanup sweep, which is out of scope here.

## Failure path

Nothing is persisted on any failure, so there is nothing to compensate for. The
previous design pre-wrote the id and needed a re-read-modify-write to clear it after
a failed spawn; with post-exit ordering that path is gone entirely, along with its
"failed to clear" warning. Frontmatter the child wrote before failing (for example
`phase: planning`) is untouched, because the caller never writes on the failure path.

The persist step itself can also fail — the re-read or the write. When it does, the
caller is handed an **empty** id, never the one it minted. Nothing landed on disk, so
reporting the id would advertise a session the UI cannot resume, which is the same lie
in a different place. The rule is uniform: the id is returned only when it is on disk.
