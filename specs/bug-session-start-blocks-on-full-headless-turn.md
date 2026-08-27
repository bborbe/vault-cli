---
status: draft
kind: bug
---

## Summary

- `StartSession` is named and documented as "starts a headless Claude session, returns session_id", but it blocks for the **entire** agentic turn — `--output-format json` emits its single blob only at process exit, and `maxTurns` is `-1`.
- The 5m `context.WithTimeout` in `StartSession` therefore budgets not a session handshake but a full `/vault-cli:work-on-task` run: guide loading, semantic search, Jira MCP, daily-note write, task frontmatter write, and the `plan-task → execute-task` chain.
- Real tasks exceed it. On expiry `exec.CommandContext` kills the child mid-write and no `session_id` is persisted, so the next Start re-runs from zero against a task file the killed session already partially mutated.
- Fix: `claude` accepts `--session-id <uuid>`. Generate the id caller-side, persist it *before* the child exists, spawn the child **detached from the request context**, and wait only long enough to catch an immediate crash.
- Both the task path (`pkg/ops/workon.go`) and the goal path (`pkg/ops/goal_workon.go`) carry the defect and must change together.

## Goal

After this work, a **non-TTY** `task work-on` (the Vault UI Start button) returns a persisted session id within `livenessWindow`, the bootstrap turn runs to completion detached from the CLI's lifetime, and no timeout path can kill a turn mid-write **on that branch**. The **TTY** start-then-resume flow behaves exactly as it does today.

That leaves the 5m cap — and its kill-mid-write hazard — alive on the TTY branch. This is deliberate: turn 2 `syscall.Exec`s `claude --resume` against turn 1's on-disk result, so the blocking wait is what makes the handoff correct, and `scenarios/005` regression-locks it. The observed reproduction is non-TTY (the Vault UI banner, and the sub-agent that shelled `vault-cli task work-on` from a non-terminal context), so the fix does cover the bug as reported.

## Problem

`task work-on` exists so the Vault UI Start button can hand the operator a resumable session. Today the button's whole value — "give me a session id" — is gated behind work that has nothing to do with minting a session. The operator waits five minutes and then gets an error, not a session.

The failure is also silently destructive. The killed child has usually already written frontmatter (status, phase), so the task is left in a state no command produced deliberately. Retrying does not resume that work; it starts a second full turn against the mutated file.

## Reproduction

vault-cli `v0.116.2` (`git describe` → `v0.116.2-1-gf16ebcf`); observed 2026-08-27.

Setup — any task whose `/vault-cli:work-on-task` bootstrap exceeds 5 minutes (a task with guides to load and a `plan-task → execute-task` chain to run). A trivial task will NOT reproduce: it returns in seconds.

Action — click **Start** on that task in the Vault UI Kanban, or equivalently:

```bash
vault-cli task work-on "<some non-trivial task>"
```

Observed, from the Vault UI error banner:

```
vault-cli work-on failed: time=2026-08-27T09:54:46.972+02:00 level=WARN
msg="workon session error" error="claude session start timed out after 5m
    github.com/bborbe/vault-cli/pkg/ops.(*claudeSessionStarter).StartSession
    github.com/bborbe/vault-cli/pkg/ops.(*workOnOperation).handleClaudeSession
    ...
Error: start work-on session: start claude session: claude session start timed out after 5m
```

Root-cause evidence, in `claudeSessionStarter.StartSession`:

```go
timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
...
args = append(args, "-p", prompt, "--output-format", "json")
```

with `maxTurns: -1` set in `NewClaudeSessionStarter`. `--output-format json` is documented by `claude --help` as `"json" (single result)` — nothing is emitted until the turn ends.

**Independent live reproduction of the destructive half**, same day: a sub-agent preparing the vault task that tracks this bug ran `vault-cli task work-on`, which spawned the nested headless `claude --print`. It was killed at ~1 minute — but the child had already written `phase: planning` into that task's frontmatter. That is the kill-mid-write path, observed end to end.

## Expected vs Actual

| | Behaviour |
|---|---|
| **Expected** | `StartSession` does what its doc comment says — *"runs claude in headless mode to create a session, returns session_id"* — returning the id once the session exists. The bootstrap turn continues independently. |
| **Actual** | Returns only when the entire agentic turn has finished, or fails at 5m having killed the turn mid-write and persisted nothing. |

## Why this is a bug

Three separate contracts are violated:

1. **The function's own doc comment** on `ClaudeSessionStarter.StartSession` promises session creation, not turn completion.
2. **The error message misattributes the cause.** `"claude session start timed out"` names session start; what timed out is a `work-on-task` run. An operator reading it looks for a session/auth problem that does not exist.
3. **The persisted-state invariant.** The v0.109.2 writeback fix (see `CHANGELOG.md`, and the doc comments at `workon.go:185-190` / `goal_workon.go:162-164`) established that frontmatter written by the headless turn must survive. The timeout path breaks the same invariant from the other side: it kills a turn mid-write and leaves partial state no command intended.

*(`## Desired Behavior` is deliberately omitted: `Expected vs Actual` plus the Acceptance Criteria below carry its work for a single-behaviour bug.)*

## Constraints

- **Two runners, not one.** The detachment mechanics below apply to a **new, non-TTY-only runner**. `defaultCommandRunner` (`exec.CommandContext` + `cmd.Output()`, plus the JSON-blob validation it feeds at `claude_session.go:107-117`) stays **exactly as it is** and continues to serve the interactive branch. A single shared detached runner would silently change three TTY behaviours — the wait becomes unbounded, `Setpgid` stops `Ctrl-C` reaching the child (orphaning a live turn that keeps writing frontmatter), and the `empty session_id` / `0 turns` / `reported error` diagnostics are lost to `os.DevNull`. All three contradict "TTY unchanged".
- **The detached child must outlive the parent.** In the new runner, `exec.CommandContext(ctx, …)` and `cmd.Output()` are both fatal to a fast return: the context is cancelled when the CLI command finishes, sending the child `SIGKILL`; and `cmd.Output()`'s stdout pipe closes on parent exit, so the child dies on `EPIPE` at its terminal write (or blocks earlier once the 64KB pipe buffer fills with nobody draining). The detached runner MUST spawn with **`exec.Command`, not `exec.CommandContext`**, redirect stdout and stderr to `os.DevNull` (never a pipe the parent reads), set `SysProcAttr{Setpgid: true}`, and never cancel or wait on the child past the liveness window. Deliberate detachment, not merely absence of a `Wait`.
- **The runner seam changes shape.** The subprocess sits behind the plain `runCmd func(ctx, args, dir) ([]byte, error)` field on `claudeSessionStarter`, not behind the Counterfeiter mock (which covers `ClaudeSessionStarter`, one level up). That signature cannot express a detached spawn — there is no output to return and nothing to wait on. Prompt 1 owns introducing the second seam and updating the three `NewClaudeSessionStarterWithRunner` call sites in `pkg/ops/claude_session_test.go`.
- **`isInteractive` must be plumbed to `StartSession`.** It currently stops at `Execute` (`workon.go:69`, `goal_workon.go:60`) and `handleClaudeSession` is called without it (`workon.go:113`, `goal_workon.go:104`). Selecting the runner requires it at the starter.
- **The TTY start-then-resume flow does not change.** `workOnOperation.Execute` `syscall.Exec`s `claude --resume <sessionID> "<continuation>"` immediately after `handleClaudeSession` returns whenever `isInteractive`. Note `isInteractive` is the resolved `--mode` flag (`auto | interactive | headless`, `pkg/cli/cli.go:440-450`) — `term.IsTerminal` is only the `auto` arm. So `--mode headless` from a real terminal takes the fast path and `--mode interactive` from a pipe takes the blocking path; the branch is chosen by the flag, not by the tty. The existing comment states the contract: turn 2 *"resumes the chain from whatever phase turn 1 left on disk"* — which holds only because `StartSession` blocks until turn 1 finishes. **Therefore the fast return is scoped to the non-interactive branch only.** On the interactive branch nothing changes except the wording of its deadline error (AC11 requires the misleading `claude session start timed out` string gone repo-wide, and that string lives on this branch at `claude_session.go:92`; the replacement wording is agent-decides, but must name a bootstrap-turn timeout rather than a session-start one) — same runner, same blocking wait, same 5m cap, same JSON validation — so turn 2 never races a live turn-1 child on the same session transcript. The 5m cap is removed on the non-interactive branch only, where the liveness window replaces it.
- **`scenarios/005-work-on-resume-auto-invokes-subtask.md` must remain valid, unmodified.** It is `status: active` and walks the TTY start→resume path, asserting on the replayed turn-1 transcript. It is the regression lock on the previous constraint.
- **`scenarios/002-task-lifecycle.md` MUST be updated — it is invalidated by this fix.** It is `status: active` and drives the non-TTY branch. Its note at line 33 tells the operator to *"expect no output at all for ~2-3 minutes (measured 2m49s) … Allow ≥300s"* and warns that a fast return looks like a false FAIL. After this fix the correct behaviour is the inverse, so an operator replaying 002 unchanged would read a 10s return as a regression. Updating it also converts Failure Modes row 7 from manual-only detection into an automated lock: 002 already runs a real `claude` on the right branch, so the `<uuid>.jsonl` existence check belongs in its Expected block.
- **Write ordering is load-bearing.** `persistSessionAndMetrics` re-reads the task after `StartSession` returns *precisely because* the session mutates that same file. The fix must persist the id **before the child process starts**, so the session's own read-modify-write always reads a file that already contains it — not merely narrow the window.
- **`pkg/ops/workon_session_writeback_test.go` may be edited only mechanically.** It cannot stay byte-identical: its `StartSessionStub` assignments (`:85`, `:160`) hard-code today's arity and return `("session-123", nil)`, both of which the signature change and caller-side uuid generation invalidate. Permitted edits: stub arity, and the id constant becoming a `uuid.Parse`-able value. **Every invariant assertion must survive** — phase reads `execution`, `session_note` survives, `claude_session_id` is persisted, `MetricsSessions()` has len 1. The stub's write-to-the-task-file moves into the runner fake, since the turn is no longer bracketed by the call.
  Note the invariant has *moved* rather than vanished: with persist-before-spawn there is no post-turn id write to revert on the fresh-start path, so that context now guards a path that no longer exists. Its live home is the compensating-clear assertions in AC6/AC8 and the cached path at `workon.go:228`.
- **Do not read the id from a `--output-format stream-json --verbose` init event.** It arrives in ~1s, but returning early from a stream reintroduces exactly the hazard above from the opposite direction. Rejected deliberately.
- **The uuid generator is injected, not called inline.** `handleClaudeSession` takes a generator dependency on `workOnOperation` / `goalWorkOnOperation`, mirroring the `libtime.CurrentDateTime` pattern already used for the clock. Without it the minted id is nondeterministic and the five `Equal("session-123")` assertions in `workon_session_writeback_test.go` (`:121`, `:131`, `:136`, `:192`, `:202`) have no path to a pinned value — the bounded-diff whitelist below would be unsatisfiable. Tests pin a fixed uuid; every equality assertion survives verbatim. Note `session-123` also appears in `workon_test.go`, `frontmatter_test.go`, and `goal_workon_test.go` — the injection keeps all of them working.
- `pkg/ops/` stays a pure library — structured returns, no stdout.
- Unit tests fake the subprocess and must not sleep through the liveness window. **The liveness wait goes behind `libtime.WaiterDuration`** (`github.com/bborbe/time` exports `NewWaiterDuration`), injected on `claudeSessionStarter` — `libtime.CurrentDateTime` supplies only `Now()` and cannot shorten a wait. Detached branch only; the interactive branch's `context.WithTimeout` is not refactored (see AC3). **Exception:** the detachment behaviour (AC1) is unobservable through that seam and requires the integration test named in AC1, which spawns a real throwaway script — never a real `claude` binary.
- The cached-session path (`task.ClaudeSessionID() != ""`) is unchanged.
- The bootstrap prompt, `--non-interactive`, and everything the turn does are unchanged. This spec changes *when the parent stops waiting*, nothing about the work itself.

## Design

Decisions only — the implementation belongs to the prompts, and the durable version of this reasoning belongs in `docs/work-on-session-lifecycle.md` (AC13).

1. **Caller-side id.** `claude --help` documents `--session-id <uuid>` as *"Use a specific session ID for the conversation (must be a valid UUID)"* (verified 2026-08-27). `handleClaudeSession` generates it via `github.com/google/uuid` (already a direct dependency) so the persist can precede the spawn; `StartSession` therefore takes the id as a parameter and returns only an error.
2. **Persist before spawn.** The write happens while no child exists, which is what makes the ordering a guarantee rather than a race.
3. **Detached spawn.** Per the first Constraint above.
4. **Liveness window, not a timeout.** On the non-interactive branch `StartSession` waits on a `Wait`-fed channel for `livenessWindow` (10s). An exit inside the window is a real failure; otherwise return nil and abandon the child. The 5m `context.WithTimeout` is removed **on that branch only** — the interactive branch keeps it, unchanged (see Constraints).
5. **Compensating write on spawn failure.** A failed spawn clears both the pre-written `claude_session_id` and the `metrics_sessions` entry `persistSessionAndMetrics` appended, leaving the task retryable. **The clear is itself a re-read-modify-write:** it must re-read the task from disk and clear only those two fields, preserving any frontmatter the child wrote before dying. A child that starts, writes `phase: planning`, then exits at 8s is squarely inside the window — clearing from the stale in-memory copy would revert that write and reproduce the exact "partial state no command intended" this spec exists to eliminate.
6. **Scope: non-interactive only.** Per the Constraints, the fast return applies to the non-TTY branch. The interactive branch keeps blocking through turn 1 so its `--resume` handoff is unchanged.

**Accepted cost:** no event stream, therefore no true inactivity watchdog. The window covers the failure mode that actually bites (session dies on startup: bad flag, auth failure). A session that hangs *after* starting is left to the Vault UI's existing `claude_session_started` cleanup sweep — out of scope here.

## Acceptance Criteria

- [ ] **The child outlives the parent.** An integration test spawns a throwaway script (not `claude`) that sleeps past the liveness window and then writes a sentinel file; after `StartSession` returns AND the enclosing operation's context is cancelled, the sentinel appears. Negative evidence, scoped to the detached runner only (the interactive runner keeps `CommandContext` + `cmd.Output()` — see Constraints): the detached runner's function body contains neither `CommandContext` nor `cmd.Output()`, and `grep -c 'exec.CommandContext' pkg/ops/claude_session.go` returns exactly **1**, on a line whose surrounding comment names it the interactive runner. Positive evidence that the detachment is deliberate rather than accidental: `grep -n 'Setpgid' pkg/ops/claude_session.go` returns ≥1 line, and `grep -n 'os.DevNull' pkg/ops/claude_session.go` returns ≥1 line (a `nil` `cmd.Stdout` satisfies "not a pipe" by accident and must not be how this passes).
- [ ] **`StartSession` returns within `livenessWindow`** on a runner whose child outlives it — asserted against the constant with an injected clock, not a wall-clock tolerance.
- [ ] **TTY behaviour is unchanged.** With `isInteractive` true, `StartSession` routes to the untouched interactive runner: it does not return until the child has exited (test with a slow fake child); **the 5m cap still applies** — negative evidence (this count is 1 both before and after; it guards against deletion, it does not prove work): `grep -c 'context.WithTimeout' pkg/ops/claude_session.go` returns exactly 1, on the interactive path, and a unit test driven by a parent context whose deadline has already expired (the 5m `context.WithTimeout` inherits it, so `timeoutCtx.Err()` is `DeadlineExceeded` immediately) asserts the interactive branch returns a deadline-derived error — **the interactive branch's deadline source is not refactored**, since `context.WithTimeout` reads the runtime monotonic clock and no injected `libtime` value can move it; and the `empty session_id` / `0 turns` / `reported error` validation branches still fire (`grep -c 'claude returned 0 turns' pkg/ops/claude_session.go` returns 1). `git diff --exit-code scenarios/005-work-on-resume-auto-invokes-subtask.md` is empty.
- [ ] **Persist precedes spawn.** The storage fake and the runner fake each record a call timestamp; the test asserts `writeTaskAt < spawnAt`.
- [ ] **The full argv survives the signature change.** The fake runner's captured argv contains `--session-id <id>`, `--print`, and `-n <task name>` (the flag that sets the session's custom title from turn 1); `uuid.Parse(id)` returns no error; `id` equals the value in task frontmatter.
- [ ] **Early exit is an error, and rolls back without clobbering.** Driven through `workOnOperation.Execute` / `handleClaudeSession` (the rollback is caller-side per Design §5 — a test calling `StartSession` directly would never observe it). A runner whose child writes `phase: planning` and then exits non-zero inside the window makes the call return a non-nil error wrapping the child's exit status; afterwards `grep -c '^claude_session_id:' <task>` returns 0, the `metrics_sessions` entry for that id is gone, **and `grep '^phase:' <task>` still reads `planning`** — the child's write survived the compensating clear.
- [ ] **The writeback invariant holds under a bounded diff.** `go test ./pkg/ops/ -run Writeback` exits 0, AND every invariant assertion survives the mechanical edit: in the post-fix file `grep -c 'TaskPhaseExecution'` returns 2, `grep -c 'GoalPhaseExecution'` returns 2, `grep -c 'session_note'` returns 4, `grep -c 'MetricsSessions()'` returns 2, **and `grep -c 'ClaudeSessionID()'` returns 2** — the id-persistence invariant is the one caller-side generation actually threatens, so it is locked explicitly. Counts are pinned to the current tree; the permitted edits (stub arity, pinned uuid constant) do not touch those lines.
- [ ] **The goal path matches.** `pkg/ops/goal_workon.go` receives the same treatment. `goal_workon_test.go` asserts: `writeGoalAt < spawnAt`; captured argv contains `--session-id <id>`, `--print`, `-n <goal name>`; and on early exit `grep -c '^claude_session_id:' <goal>` returns 0 with the child's frontmatter write preserved. Note `persistGoalSessionID` writes the id only — the goal path has no `metrics_sessions`, so there is no metrics rollback to assert.
- [ ] **`scenarios/002-task-lifecycle.md` reflects the new timing.** Its `task work-on` step expects a return within ≲`livenessWindow` with `session_id:` present, the ≥300s / 2m49s note is replaced by one stating the turn continues after the CLI exits, and its Expected block gains the `~/.claude/projects/<encoded-cwd>/<uuid>.jsonl` existence check (the automated lock for Failure Modes row 7). Evidence: `grep -c '300s' scenarios/002-task-lifecycle.md` returns 0, `grep -c '2m49s' scenarios/002-task-lifecycle.md` returns 0 (the misleading note carries both, plus `2-3 minutes` and `exit 124`, all on one line — a partial rewrite that drops only "300s" must not pass), and `grep -c 'jsonl' scenarios/002-task-lifecycle.md` returns ≥1.
- [ ] **The counterfeiter fake is regenerated.** `mocks/claude-session-starter.go` matches the new `ClaudeSessionStarter` signature — `git diff --stat mocks/claude-session-starter.go` against the pre-change commit is non-empty, and `make precommit` (which runs generate) leaves the tree clean.
- [ ] **The misleading error is gone.** `grep -rn 'claude session start timed out' pkg/` returns 0 lines; the startup-failure error names the child's exit status instead.
- [ ] **Stale comments corrected.** `grep -rniE 'blocks? for the entire|after StartSession blocks|blocking turn' pkg/ops/` returns 0 lines (4 lines in 2 files today: `goal_workon.go:164`, `goal_workon.go:186`, `workon.go:187`, `workon.go:218` — two of them the `persistSessionAndMetrics` / `persistGoalSessionID` doc comments, whose stated rationale "the re-read is load-bearing because StartSession blocks" genuinely changes under persist-before-spawn), and the `StartSession` doc comment describes the new contract.
- [ ] **`docs/work-on-session-lifecycle.md` exists with real content**, not just headings: `grep -c '^## ' docs/work-on-session-lifecycle.md` ≥4, and each of `grep -n 'os.DevNull'`, `grep -n 'stream-json'`, `grep -n 'output-format json'`, `grep -n 'TTY'` returns ≥1 line — covering who owns the session id, why the pre-spawn write ordering exists, why `stream-json` was rejected, why the TTY branch is untouched, the fate of `--output-format json` (Open Q4), and what the liveness window does and does not cover. `grep -n 'work-on-session-lifecycle' pkg/ops/claude_session.go` returns ≥1 line.
- [ ] `## Unreleased` CHANGELOG entry added: `grep -A5 '^## Unreleased' CHANGELOG.md` returns ≥1 non-empty bullet.
- [ ] `make precommit` green.

## Failure Modes

| Trigger | Detection | Expected behaviour | Recovery |
|---|---|---|---|
| Bootstrap turn runs 20 minutes | n/a — nominal | `StartSession` returned at ~10s; turn completes independently; frontmatter writes land normally | none needed |
| `claude` binary missing | `NewClaudeSessionStarter` returns nil at construction | Unchanged — `ErrStarterUnavailable`, CLI warns, exits 0 | install `claude`; re-run `task work-on` |
| Child exits non-zero within window (auth failure, bad flag) | `Wait` fires inside `livenessWindow` | Error returned naming the exit status; pre-written id AND metrics entry cleared | fix the cause, re-run `task work-on` — task is already retryable |
| Child exits non-zero *after* window | **Undetected by vault-cli** — surfaces only when the operator runs `--resume` and gets an empty/broken session | Accepted. Id points at a dead session | `vault-cli task clear "<name>" claude_session_id`, then re-run `task work-on` — the cached path short-circuits otherwise, so the clear is mandatory, not optional |
| Persist succeeds, spawn fails, compensating clear also fails | spawn error returned + a separate warning in `MutationResult.Warnings` | Return the spawn error wrapped; surface the clear failure as a warning — never mask the original | same manual clear as the row above, then re-run |
| Installed `claude` rejects `--session-id` | Child exits non-zero immediately → caught by the liveness window as a bad-flag failure | Error returned; id and metrics rolled back | upgrade `claude`; the flag was verified present 2026-08-27 |
| Installed `claude` silently ignores `--session-id` and mints its own | **Not detectable at runtime.** Caught only by operator-executable step 3 — the `<uuid>.jsonl` file is absent under `~/.claude/projects/<encoded-cwd>/` | Frontmatter id points at a session that does not exist | treat as the dead-session row: clear the id and re-run. If reproducible, the caller-side-id design is invalid and the spec must be reopened |
| Task already has `claude_session_id` | n/a | Cached path, unchanged — no spawn, no new id | to force a fresh session, clear the field first |
| Two concurrent `work-on` calls on one task | n/a | Unchanged from today: last writer wins on the id | not made worse by this change; out of scope |
| Session writes frontmatter 1s after spawn | n/a | Reads a file that already contains the id — the ordering guarantee, not a race | none needed |
| TTY caller (`term.IsTerminal`) runs `task work-on` | n/a — the branch is chosen before the spawn | Unchanged from today: the parent blocks through turn 1, then `syscall.Exec`s `claude --resume`. Turn 2 reads turn 1's completed on-disk result | none needed. If a fast return ever leaks into this branch, turn 2 races the live child on one session transcript — that is the regression `scenarios/005-*.md` locks |
| TTY caller's bootstrap exceeds 5m | 5m deadline error at the CLI | Unchanged from today: child killed mid-write, partial frontmatter persists. **Deliberately out of scope** — regression-locked by `scenarios/005` | `vault-cli task clear "<name>" claude_session_id`, then re-run from a terminal |
| Child writes `phase`, then exits non-zero at 8s | `Wait` fires inside the window | Error returned; id + metrics cleared **via re-read**; the child's `phase` write survives | fix the cause, re-run — the task carries the child's partial progress, not a reverted copy |

## Suggested Decomposition

| # | Prompt focus | Covers ACs | Depends on |
|---|---|---|---|
| 1 | `StartSession` signature change, second (detached) runner seam, liveness window, `isInteractive` plumbed through `workon.go` + `goal_workon.go` call sites, integration test for detachment, mock regeneration, bounded edit to `workon_session_writeback_test.go` so the tree compiles, new error string replacing the timeout message | 1, 2, 3, 5, 7, 10, 11 | — |
| 2 | Task path: pre-spawn persist + compensating re-read-clear of id and metrics | 4, 6 | 1 |
| 3 | Goal path: same treatment (id-only rollback) | 8 | 1, 2 |
| 4 | `docs/work-on-session-lifecycle.md`, doc comments, `scenarios/002` update, CHANGELOG | 9, 12, 13, 14 | 1–3 |

AC11 (the misleading error string) belongs to prompt 1, which owns removing the timeout — not to the docs prompt. AC15 (`make precommit` green) applies to every prompt, not one.

`scenarios/005-work-on-resume-auto-invokes-subtask.md` is **not** revised by any prompt — AC3 requires it unchanged, and prompt 1 owns keeping it that way.

The table omits the `Covers DBs` column because `## Desired Behavior` is deliberately absent (see the note under *Why this is a bug*); ACs are the traceability target instead.

## Workaround

Until the fix lands: run `vault-cli task work-on "<name>"` from a real terminal rather than clicking Start in the Vault UI — the TTY path blocks through turn 1 and hands you the interactive session directly. If a Start click has already timed out, `vault-cli task clear "<name>" claude_session_id` before retrying, or the cached path will short-circuit on a dead id.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- The unit and integration tests named in the Acceptance Criteria.
- The grep/diff assertions in ACs 1, 3, 6, 7, 8, 9, 11, 12, 13, 14.
- `make precommit`.

### Operator-executable (runs on the host after PR merge, spec verification ladder)

Needs a browser and a real `claude`; neither exists in the container.

1. Click **Start** in the Vault UI on a task whose bootstrap exceeds 5 minutes.
2. Card flips to **Resume** within ~10s, no error banner.
3. `~/.claude/projects/<encoded-cwd>/<uuid>.jsonl` exists, where `<uuid>` is the `claude_session_id` in frontmatter. **This is the step that catches a silently-ignored `--session-id`** (Failure Modes row 7).
4. `claude --resume <uuid>` opens the bootstrap conversation.
5. Once the turn lands, `grep '^phase:'` on the task file reads `execution` — the writeback invariant, live, with the parent long gone.

## Open Questions

1. Is 10s the right liveness window? Long enough to catch auth failures, short enough that Start feels instant. Tunable constant; no config field unless a second caller needs one.
2. Should the goal path share a helper with the task path, or stay duplicated as it is today? The codebase currently duplicates deliberately (`persistSessionAndMetrics` / `persistGoalSessionID`); this spec follows that convention rather than refactoring under a bug fix.
3. Resolved: the detached child's stdout/stderr go to `os.DevNull`. A per-session log file would make the "died after the liveness window" row diagnosable rather than merely recoverable, but that is a new artifact with its own lifecycle — deferred until that failure is actually observed.
4. `--output-format json` becomes dead weight on the non-interactive branch: nothing parses the blob any more, and stdout goes to `DevNull`. Keep it (harmless, and the interactive branch still exits on it) or drop it? — **agent decides at impl time**; either way the choice must be stated in `docs/work-on-session-lifecycle.md`, not left implicit.
5. Resolved: **no Windows target**, so no build-tag split. `.goreleaser.yaml` builds `darwin` and `linux` only, and `pkg/` contains zero `//go:build` lines. `SysProcAttr{Setpgid: true}` goes inline in `pkg/ops/claude_session.go` — which AC1's file-pinned `Setpgid` and `os.DevNull` greps depend on. A `claude_session_unix.go` split would make both return 0 and fail AC1.
