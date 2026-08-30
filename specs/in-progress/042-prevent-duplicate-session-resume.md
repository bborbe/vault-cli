---
status: prompted
approved: "2026-08-30T16:57:47Z"
generating: "2026-08-30T18:02:13Z"
prompted: "2026-08-30T18:02:13Z"
branch: dark-factory/prevent-duplicate-session-resume
---

## Summary

- Two `claude` processes can currently work on the same session at the same time (two Resume clicks, or a Resume racing a second `work-on`). Both append to the single transcript file `~/.claude/projects/<cwd>/<session-id>.jsonl`, and interleaved appends corrupt the transcript — this is corruption, not just a duplicate window.
- This spec adds a per-session lock in the vault-cli launch path: before any claude process is started or resumed, vault-cli takes an exclusive, non-blocking file lock keyed by the session id.
- A second `work-on` on a session whose lock is live is refused with a clear error (`ErrSessionBusy`) — no second writer is ever spawned, nothing blocks, nothing retries.
- The lock is injected into the launch path like the session-id generator and clock (never a package global), lives under the user's home on a real local filesystem, and the operating system frees it automatically when the holding process exits — even on SIGKILL — so a crashed session never leaves a stale lock.
- The Vault UI's own Resume-button gating (live vs quiet classification) is a separate follow-on spec. This spec is only the vault-cli prevention half.

## Problem

Two claude processes can work on the same session id at the same time, and both append to the one transcript file. Nothing guards against it. The persist-after-exit ordering from spec 040/041 fixed "Resume offered while the bootstrap turn is still writing", but once `claude_session_id` is on disk, two Resume clicks — or a Resume racing a second `vault-cli task work-on` / `goal work-on` — start two claude processes against the same transcript with no mutual exclusion. Interleaved appends corrupt the jsonl; nothing fails, so the corruption is silent. The Vault UI makes the second click easy: it renders a Start/Resume button for any task or goal that has a `claude_session_id` and shells out to `claude --resume <id>`.

Two halves prevent this: the vault-cli launch path must refuse to start a second process against a live session, and the UI must not offer Resume against one. This spec is the vault-cli half. The UI half (live/quiet classification, Resume-button gating) is a separate follow-on; its liveness signal already exists (`src/vault_ui/activity.py`, `transcript_mtime()`, PR #39).

## Goal

After this work, a second `vault-cli task work-on <task>` or `goal work-on <goal>` that would start another claude process against a session id whose lock is already held by a live process is refused with a clear error (sentinel `ErrSessionBusy`), while the first process's lock stays held for the whole time it is writing the transcript. The lock releases automatically when the claude process exits — normal exit, crash, or SIGKILL — and on ctx cancellation and the 30m turn timeout; a stale lock or a poisoned session is impossible. The interactive TTY branch and scenario 005 behave byte-identically.

## Non-goals

- No vault-ui changes in this spec: the Resume button, live/quiet classification, and activity.py-driven gating are a separate follow-on spec. The UI's direct `claude --resume <id>` is guarded by that spec, not by this one.
- No change to the post-exit persist ordering (an id on disk means resumable), `validateSessionTurn` or its error strings, the 5m TTY cap, `--max-turns` (stays inert), or the detached child process group.
- No guard for two concurrent FRESH starts of a task with no session id: they mint different ids, write different transcripts, and cannot corrupt each other. This is the residual risk spec 041's Failure Modes row 7 recorded; it stays unchanged here.
- No watchdog for sessions that hang after starting — the Vault UI's `claude_session_started` cleanup sweep owns that.
- No config surface beyond the lock-directory default; no new tunables, no opt-out flag for the lock. The lock is the invariant, not a feature that can be switched off.

## Acceptance Criteria

1. **Locker contract.** A session lock held by one open fd refuses a second acquire on the same session id (returns `ErrSessionBusy`, `errors.Is` true), accepts a re-acquire once released, and is freed by closing the fd without an explicit unlock (process-death proxy). Mechanism: a locker unit test against the REAL flock syscall on a temp dir (two `open()` fds on the same lock file). Evidence: test asserts `MatchError(ops.ErrSessionBusy)` on the second acquire and `BeNil()` on both re-acquires; `grep -c 'ErrSessionBusy' pkg/ops/errors.go` returns ≥ 1 (the sentinel exists).

2. **StartSession lock lifecycle.** Starting a session whose id lock is already held refuses (`ErrSessionBusy`) and spawns NO child; after every `StartSession` return path (clean turn, child exit error, ctx cancel, 30m bound expiry) a fresh acquire of the same id succeeds — the lock was released, never leaked. Mechanism: unit test with the REAL flock locker on a temp dir (no mock locker). Evidence: test asserts the spawn counter stays 0 on the busy path and asserts successful re-acquire after each path; `grep -rn 'ErrSessionBusy' pkg/ops/claude_session_test.go` returns ≥ 1.

3. **ResumeSession lock + exec survival.** Resuming a session whose id lock is held refuses (`ErrSessionBusy`) without invoking the exec; on the normal path the lock fd survives the process replacement into `claude --resume` (its close-on-exec flag is clear), so the resumed claude process itself holds the lock until it exits. Mechanism: unit test with the REAL flock locker on a temp dir (no mock locker). Evidence: test asserts the exec function is not called on the busy path and asserts `FD_CLOEXEC` is not set on the lock fd; `grep -rn 'ErrSessionBusy' pkg/ops/claude_resume_test.go` returns ≥ 1.

4. **Busy refusal is a hard work-on failure (task and goal).** A task work-on and a goal work-on on a busy session fail HARD: Success:false with a message naming the session, never downgraded to a warning; the `ErrStarterUnavailable` soft path (warning, exit 0) is unchanged. Mechanism: unit tests on both paths. Evidence: `grep -rn 'ErrSessionBusy' pkg/ops/workon_test.go` returns ≥ 1 and `grep -rn 'ErrSessionBusy' pkg/ops/goal_workon_test.go` returns ≥ 1; the existing `ErrStarterUnavailable` test assertions are unchanged.

5. **Interactive TTY branch and scenario 005 byte-identical.** Negative evidence: `git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md` returns empty; `grep -c 'defaultCommandRunner' pkg/ops/claude_session.go` ≥ 1 (interactive branch still uses it); `grep -c 'context.WithTimeout' pkg/ops/claude_session.go` ≥ 1 (the 5m TTY cap still present); `validateSessionTurn` error strings and the detached `Setpgid` child are unchanged.

6. **Lock directory: configurable, created on demand, real filesystem.** Unit test: a configured lock dir is created (parent dirs included) if missing, and the lock file lands at `<dir>/<session-id>.lock`; the default dir resolves under the user's home on a real local persistent filesystem — not a tmpfs cleared on reboot. Evidence: the unit test asserts the lock file exists after acquire; the default path is a documented constant under the user's home.

7. **Docs and CHANGELOG.** `docs/work-on-session-lifecycle.md` gains a section documenting the lock: why (two writers on one transcript corrupt the jsonl), the invariant (lock held while a process writes the transcript; kernel-released on process death, so no stale lock), and the `ErrSessionBusy` refusal. `CHANGELOG.md` `## Unreleased` carries a bullet describing the duplicate-resume guard. Evidence: `grep -ci 'lock' docs/work-on-session-lifecycle.md` ≥ 1; `grep -c '^## Unreleased' CHANGELOG.md` ≥ 1 AND `grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume\|busy\|lock'` ≥ 1.

8. **Full gate.** `make precommit` exits 0; `make test` exits 0; `go test ./... -race` exits 0. Evidence: exit codes.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit                                                       # exit 0
make test                                                            # exit 0
go test ./... -race                                                  # exit 0 (concurrency AC)
grep -c 'ErrSessionBusy' pkg/ops/errors.go                           # >= 1 (sentinel)
grep -rn 'ErrSessionBusy' pkg/ops/claude_session_test.go             # >= 1
grep -rn 'ErrSessionBusy' pkg/ops/claude_resume_test.go              # >= 1
grep -rn 'ErrSessionBusy' pkg/ops/workon_test.go                     # >= 1
grep -rn 'ErrSessionBusy' pkg/ops/goal_workon_test.go                # >= 1
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md  # empty
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go             # >= 1
grep -c 'context.WithTimeout' pkg/ops/claude_session.go              # >= 1 (5m TTY cap)
grep -ci 'lock' docs/work-on-session-lifecycle.md                    # >= 1
grep -c '^## Unreleased' CHANGELOG.md                                # >= 1
grep -A15 '^## Unreleased' CHANGELOG.md | grep -ci 'resume\|busy\|lock'  # >= 1
```

### Operator-executable (host, after PR merge + release + `make install`; spec verification ladder)

1. `vault-cli --version` — new version, not `-dirty`.
2. **Live resume refused.** On a real TTY, run `vault-cli task work-on "<fixture task>"` (fresh, no live session) so it execs into `claude --resume <id>` — the resumed claude now holds the session lock. In a second real TTY, run `vault-cli task work-on "<same task>"` → refused: non-zero exit, message names the session (`ErrSessionBusy`). Use a throwaway fixture task; note today's daily-note entry for cleanup.
3. **SIGKILL → no stale lock.** SIGKILL the resumed claude process from step 2 (its PID, or `pkill -9 -f 'claude --resume <id>'`), wait for it to die, then re-run `vault-cli task work-on "<same task>"` → succeeds immediately (the kernel freed the lock on process death).
4. Clean up the fixture task and any daily-note checkbox.

## Desired Behavior

1. Before any claude process is started or resumed by the launch path — the headless bootstrap spawn (`StartSession`, both interactive and non-interactive branches) and the interactive resume (`ResumeSession`) — vault-cli acquires an exclusive, non-blocking per-session lock keyed by the session id. The acquire is atomic: on contention it fails immediately, never waits, and never spawns a second writer.

2. The lock is held for the claude process's lifetime. On the spawn path the lock is released by a deferred unlock that fires on every return: clean child exit, child exit error, ctx cancellation, and the 30m turn bound. On the interactive resume path the lock survives the process replacement into `claude --resume`, so the resumed claude process itself holds the lock until it exits.

3. A contended acquire returns the sentinel `ErrSessionBusy`, wrapped with a message naming the session id. `task work-on` and `goal work-on` surface it as a hard failure (Success:false, clear error), distinct from the `ErrStarterUnavailable` soft-warning path; nothing extra is persisted and no session is started.

4. The lock never goes stale. The operating system releases the flock when the holding process exits — normal exit, crash, or SIGKILL — so a re-work-on on the same session id succeeds immediately afterwards. No cleanup sweeps, no compensating clears, no lock TTL.

5. The lock directory is configurable and defaults under the user's home on a real local persistent filesystem (not a tmpfs cleared on reboot); the directory is created on demand. The locker is injected into the launch path through the same seam as the session-id generator and clock — never a package global — so tests can inject a temp directory.

6. Everything else is unchanged: the post-exit persist ordering (an id on disk means resumable), `validateSessionTurn` and its error strings, the 5m TTY cap, `--max-turns` inert, the detached child process group, and the interactive TTY branch — scenario 005 is byte-identical.

## Constraints

- **Id-on-disk invariants unchanged.** `claude_session_id` is still persisted only after the headless turn's child exits; an id on disk means resumable. The lock changes nothing about the persist ordering, the re-read-modify-write, or the metrics entry.
- **Turn bound is a wait, never a kill.** `sessionTurnTimeout` (30m) stays a wait bound; the child stays detached in its own process group. The lock is released by the PARENT's deferred unlock when it stops waiting (child exit, child error, ctx cancel, or bound expiry). The detached child that survives a cancel/timeout keeps running WITHOUT the parent's lock — safe because its id is never persisted on those paths, so no second engager can target it. This safety depends on the persist-after-exit ordering; do not pre-persist the id.
- **Injection, never a package global.** The locker is injected into the launch path exactly like `uuidGenerator` / `libtime.CurrentDateTime`; no package-level lock state. The lock dir is configurable and defaults under the user's home on a real local persistent filesystem (not tmpfs); created on demand. Exact default subpath (e.g. `~/.claude/session-locks/`) — agent decides at impl time.
- **Lock mode.** `LOCK_EX|LOCK_NB` via the syscall (no `flock(1)` binary; the flock syscall works on macOS/Go). Contention → `ErrSessionBusy`, a new sentinel beside `ErrStarterUnavailable` in `pkg/ops/errors.go`, wrapped via `github.com/bborbe/errors` so `errors.Is(err, ErrSessionBusy)` is true and the message names the session id. No blocking waits, no retry loops.
- **Error idiom.** `stderrors.New` sentinels + `errors.Wrap`/`errors.Wrapf(ctx, …)`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in pkg/.
- **Interactive branch behavior unchanged.** The 5m TTY cap, `validateSessionTurn` and its byte-identical error strings, and `defaultCommandRunner` are untouched. The interactive resume still execs into `claude --resume`; the lock must SURVIVE that exec (the lock fd must not be close-on-exec) so the resumed claude holds it until exit — load-bearing, not optional.
- **Interface signatures unchanged.** `ClaudeSessionStarter` and `ClaudeResumer` method signatures stay as-is (counterfeiter mocks untouched); only the concrete constructors and the CLI wiring gain the locker. `scenarios/005` file byte-identical.
- **Lock scope = launch path only.** The cached-id non-interactive re-persist (e.g. vault-ui Start on a task that already has an id) spawns no writer and takes no lock here; liveness gating there belongs to the vault-ui follow-on spec.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery | Concurrency |
|---|---|---|---|---|
| Lock dir missing or unwritable (permissions, read-only home, disk full) | work-on fails HARD with a clear wrapped error — fail-closed, never spawn unguarded (an unguarded spawn is the exact double-writer hazard this spec removes) | error message on work-on; non-zero exit | operator fixes dir permissions / frees space, re-runs | two racers both fail closed — no corruption either way; reversible |
| Parent vault-cli killed (SIGKILL) while holding a spawn lock; detached child still running | kernel releases the flock on parent death; the child's id was never persisted (persist happens only after a clean `StartSession` return), so no second engager can target that id; next work-on mints a fresh id | none needed — self-healing | none | mid-crash on the fresh-start path is safe because the id is not on disk |
| Exec'd claude (resume) killed — SIGKILL or crash | fd closes on process death → flock released → a re-work-on on the same id succeeds immediately | none needed | none | exactly the "never a stale lock" property |
| Two concurrent work-ons race on the same live session id | `LOCK_NB` is atomic: exactly one acquires, the other gets `ErrSessionBusy` | busy message on the loser | user waits for the live session or kills it, then re-runs | designed behavior — one winner, one refusal, zero corruption |
| Headless turn exceeds the 30m bound while the lock is held | `StartSession` returns "did not complete within" error; the deferred unlock releases the lock; the detached child keeps running; its id is not persisted | same error as today (spec 041) | same as spec 041 — button stays on Start; re-click mints a fresh id | child runs lockless but its id is unreachable (never on disk) |
| ctx cancelled mid-wait | same as the bound row — lock released on return, child keeps running detached | cancel error | same as the bound row | same as the bound row |
| Lock dir on a tmpfs or network mount (cleared on reboot / not host-local) | flock still kernel-released, but mutual exclusion can break across hosts or after the dir is cleared — two processes could both acquire | not auto-detectable | constraint forbids it: default is a real local filesystem under the user's home; do not relocate to /tmp or a shared mount | cross-host double-acquire would reopen the corruption window — this is why the "real local filesystem" default is a constraint, not a preference |

Category coverage: external unavailability (row 1), partial-progress crash (rows 2-3), resource exhaustion (row 1 disk-full), concurrency (rows 4-7). Rate limiting — N/A: the design never blocks (LOCK_NB), so there is no backpressure queue to exhaust; many concurrent work-ons collapse to one winner plus N busy refusals (row 4). Clock skew — N/A: flock is timestamp-free; the lock carries no time comparisons. Schema drift — N/A: no new on-disk schema; the transcript format and task/goal frontmatter are untouched.

## Security / Abuse Cases

The change touches files (lock files in a directory under the user's home) and a configurable path (the lock dir).

- The lock file name derives from the session id, which vault-cli mints as a UUID — not user-controlled. If any config surface ever feeds the lock dir or key, validate it (reject path separators / traversal) before opening. At impl time the key is a UUID by construction.
- The lock dir is operator-controlled config. A malicious config pointing it at a world-writable directory lets another local user pre-create and hold a lock file → DoS on work-on (`ErrSessionBusy`) for a session id they know. Mitigate: default the dir under the user's home with owner-only permissions; document that the dir must not be world-writable. The lock is advisory — it guards accidental double-resume, not a hostile actor.
- The lock file is empty (existence + flock state only); the session id is already in the task/goal frontmatter, so the lock file adds no secret material.
- No network I/O. Nothing here can hang (LOCK_NB) or retry forever (no retry loop).

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Locker + wiring + tests: new flock locker + `ErrSessionBusy` sentinel; `StartSession` and `ResumeSession` acquire/release (incl. exec-surviving fd on resume); constructor + CLI-wiring injection; unit + `-race` tests; export seam | 1, 2, 3, 4, 5 | 1, 2, 3, 4, 5, 6, 8 | — |
| 2 | Docs + CHANGELOG: `docs/work-on-session-lifecycle.md` lock section; CHANGELOG `## Unreleased` bullet | 6 | 7 | prompt 1 |

Rationale: prompt 1 is the entire code change — the locker, both launch entry points, and the two constructor call sites in the CLI wiring — and carries all behavior and unit-test ACs plus the full gate (AC8). Prompt 2 is doc-only and depends on prompt 1 because the doc describes behavior that only exists after it. If the daemon's prompt 1 naturally includes the CHANGELOG entry (dod.md requires one per change anyway), prompt 2 collapses into prompt 1 — keep the split only to give the docs its own audit surface.

## Do-Nothing Option

Keep the status quo: nothing stops a second `claude --resume <id>` (from the UI or a second `work-on`) while the first claude still holds the transcript. Two writers interleave on one jsonl — transcript corruption, not merely a duplicate window. The persist-after-exit ordering (spec 040/041) fixed the resume-offered-mid-bootstrap half; the same-id concurrent-resume half stays open, and spec 041's Failure Modes row 7 explicitly recorded "no double-Start guard" as a residual risk. The UI-gating alternative is complementary but cannot close the corruption risk alone: the UI classifies live/quiet from transcript mtime, and a second writer can still land in the window between a session going quiet and the UI's classification updating. Doing nothing keeps a real, named corruption risk open.
