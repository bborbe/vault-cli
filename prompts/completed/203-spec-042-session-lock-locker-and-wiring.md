---
status: completed
spec: [042-prevent-duplicate-session-resume]
summary: Added a per-session flock locker (session_lock.go) with ErrSessionBusy refusal, wired it into the claude start/resume constructors and both CLI work-on sites, and added lock tests across all layers; make precommit and go test -race pass
execution_id: vault-cli-session-lock-exec-203-spec-042-session-lock-locker-and-wiring
dark-factory-version: dev
created: "2026-08-30T19:00:00Z"
queued: "2026-08-30T18:18:03Z"
started: "2026-08-30T18:40:55Z"
completed: "2026-08-30T19:01:05Z"
---

# Per-session flock locker, ErrSessionBusy refusal, and launch-path wiring (spec 042)

<summary>
- A new per-session file lock (flock) is taken before any claude process is started or resumed, keyed by the session id; on contention it refuses immediately with a clear `ErrSessionBusy` error instead of spawning a second writer.
- Starting a session now holds the lock for the whole start call and releases it on every return path — clean turn, child exit error, context cancellation, and the 30m turn bound — so a live session can never leave the lock held.
- The interactive resume's lock fd survives the exec into `claude --resume`, so the resumed claude process itself holds the lock until it exits; the fd is deliberately not close-on-exec.
- The lock directory is configurable through injection and defaults under the user's home on the real local filesystem, is created on demand, and lock files land at `<dir>/<session-id>.lock`; the kernel frees the flock on process death, so a crashed session never leaves a stale lock.
- The locker is injected through the same constructor seam as the session-id generator and clock — no package globals — and both task and goal work-on surface a busy session as a hard failure (Success:false), never a downgraded warning.
- All existing interface signatures, the interactive TTY branch, the 5m cap, `validateSessionTurn` and its error strings, and `scenarios/005` stay byte-identical; the lock tests run against the real flock syscall, not a mock.
</summary>

<objective>
Make the vault-cli launch path refuse to start or resume a second claude process against a session whose per-session flock is already held, surfacing the refusal as a hard `ErrSessionBusy` failure in task and goal work-on, while the lock is held for the whole time the first process writes the transcript and is kernel-released on process death so a stale lock is impossible. This removes the silent double-writer transcript corruption that two concurrent Resume clicks or a Resume racing a second work-on currently cause.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/storage layering rules.

Read these files fully before making changes:

- `/workspace/pkg/ops/errors.go` — the sentinel home. `ErrStarterUnavailable` is `stderrors.New("claude session starter unavailable — claude script not found in PATH")` (imports `stderrors "errors"`). The new `ErrSessionBusy` goes beside it.
- `/workspace/pkg/ops/claude_session.go` — the spawn path. `ClaudeSessionStarter.StartSession(ctx, sessionID, prompt, cwd, name, isInteractive) error` (line 38). The concrete struct `claudeSessionStarter` (line 155) currently has fields `claudePath`, `maxTurns`, `runCmd`, `detachRun`, `waiter libtime.WaiterDuration`, `sessionTurnTimeout libtime.Duration`. Constructors:
  - `func NewClaudeSessionStarter(claudeScript string) ClaudeSessionStarter` (line 52) — returns nil when `exec.LookPath` fails; sets `runCmd: defaultCommandRunner`, `detachRun: defaultDetachedRunner`, `waiter: libtime.NewWaiterDuration()`, `sessionTurnTimeout: sessionTurnTimeout`.
  - `func NewClaudeSessionStarterWithRunner(claudePath string, runCmd func(ctx context.Context, args []string, dir string) ([]byte, error), detachRun func(args []string, dir string, stdout *os.File) (<-chan error, error), waiter libtime.WaiterDuration) ClaudeSessionStarter` (line 69).
  `StartSession` builds `args` then branches on `isInteractive` into `runDetachedTurn` (non-interactive) or the inline interactive block (5m `context.WithTimeout`, `defaultCommandRunner`, `validateSessionTurn`).
- `/workspace/pkg/ops/claude_resume.go` — the resume path. `ClaudeResumer.ResumeSession(ctx, sessionID, cwd, prompt) error` (line 29). Concrete struct `claudeResumer` (line 60) has fields `claudePath`, `chdir func(dir string) error`, `execFn func(argv0 string, argv []string, envv []string) error`. Constructors:
  - `func NewClaudeResumer(claudeScript string) ClaudeResumer` (line 34).
  - `func NewClaudeResumerForTesting(claudePath string, chdir func(string) error, execFn func(string, []string, []string) error) ClaudeResumer` (line 48).
  `ResumeSession` currently: `chdir(cwd)` → build `args := []string{"claude", "--resume", sessionID}` (+ prompt when non-blank) → `c.execFn(c.claudePath, args, os.Environ())`.
- `/workspace/pkg/cli/cli.go` — the two CLI wiring sites that construct the starter + resumer: task work-on (lines ~376-388, `ops.NewClaudeSessionStarter(vault.GetClaudeScript())` / `ops.NewClaudeResumer(vault.GetClaudeScript())` then `ops.NewWorkOnOperation(...)`) and goal work-on (lines ~1505-1509, then `ops.NewGoalWorkOnOperation(goalStore, uuid.NewString, starter, resumer)`). `github.com/google/uuid` is already imported (`uuid.NewString`).
- `/workspace/pkg/ops/claude_session_test.go` — the `NewClaudeSessionStarterWithRunner` call sites (every `StartSession` unit test uses it; ~13 in this file). The suite-level `Describe` has a `JustBeforeEach` that rebuilds the starter per spec.
- `/workspace/pkg/ops/claude_resume_test.go` — the two `NewClaudeResumerForTesting` call sites (lines 38, 140).
- `/workspace/pkg/ops/claude_session_detach_test.go` — one `ops.NewClaudeSessionStarter(script)` call (the real-binary integration test).
- `/workspace/pkg/ops/workon_test.go`, `/workspace/pkg/ops/goal_workon_test.go`, `/workspace/pkg/ops/workon_session_writeback_test.go` — the `NewClaudeSessionStarterWithRunner` call sites that build a `realStarter` (workon_test.go line 896; goal_workon_test.go lines 368, 414, 479; workon_session_writeback_test.go lines 57 inside `newStarter` and 294). Each file already declares `const pinnedSessionID = "123e4567-e89b-12d3-a456-426614174000"`.
- `/workspace/pkg/ops/export_test.go` — the export-seam precedent: `const SessionTurnTimeout = sessionTurnTimeout` (a test-only alias into an unexported symbol). Mirror this for the default lock dir.
- `/workspace/pkg/ops/ops_suite_test.go` — `ErrTest = errors.New("test error")` (package `ops_test`, Ginkgo v2 + Gomega). Do not add another shared sentinel here.

Verified library APIs (grep-verified in module source — do not invent):
- `github.com/bborbe/errors` (direct dep, v1.5.19): `func Wrap(ctx context.Context, err error, message string) error`, `func Wrapf(ctx context.Context, err error, format string, args ...interface{}) error`, `func Errorf(ctx context.Context, format string, args ...interface{}) error`, `func Is(err, target error) bool` (delegates to stdlib `errors.Is`, so it walks Unwrap chains — `errors.Is(wrappedErrSessionBusy, ErrSessionBusy)` is true through the work-on double-wrap). `Wrap`/`Wrapf` return nil when `err == nil`.
- stdlib `syscall` on both macOS and Linux: `func Flock(fd int, how int) error`, constants `LOCK_EX`, `LOCK_NB`, `EWOULDBLOCK`, `F_GETFD`, `F_SETFD`, `FD_CLOEXEC`. All compile on both platforms — no build tags needed.
- `FcntlInt` does NOT exist in stdlib `syscall` — it lives in `golang.org/x/sys/unix` (`unix.FcntlInt(fd, cmd, arg)`). Import `unix "golang.org/x/sys/unix"` where needed; it is already an indirect dep in `go.mod`, so the import promotes it to a direct dep via `go mod tidy` (run by `make precommit`'s ensure).
- Gomega `MatchError(err)` matches wrapped errors via `errors.Is` (used for the AC1 second-acquire assertion).

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` context wrapping; never `fmt.Errorf`, never `context.Background()` in `pkg/`, sentinels via `stderrors.New`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega with Counterfeiter mocks, `ops_test` external test package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — Interface → Constructor → Struct → Method composition (the locker follows this shape).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` and `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — verification and changelog conventions (the CHANGELOG bullet itself belongs to the NEXT prompt in this spec; do not add one here).
</context>

<requirements>
1. In `/workspace/pkg/ops/errors.go`, add the new sentinel beside `ErrStarterUnavailable`, same `stderrors.New` idiom (message wording is yours, but it must read as a hard busy refusal, not an availability warning):
   ```go
   // ErrSessionBusy indicates another claude process already holds the per-session
   // lock for the requested session id (a second work-on or resume raced a live
   // session). Hard failure — the caller spawns no second writer.
   var ErrSessionBusy = stderrors.New(
       "claude session is busy — another process is already working on this session",
   )
   ```

2. Create `/workspace/pkg/ops/session_lock.go` — the flock locker. This is a new pattern with no in-tree exemplar, so the shape is specified; implement bodies to the contract below.
   - `SessionLock` interface:
     ```go
     type SessionLock interface {
         // Release releases the lock by closing the fd. Idempotent; the kernel also
         // releases it on process death, so Release is never strictly required.
         Release() error
         // File returns the underlying open file. FD_CLOEXEC is cleared on it so the
         // fd survives syscall.Exec on the interactive resume path (the resumed
         // claude holds the lock until it exits). Exposed for that exec-survival proof.
         File() *os.File
     }
     ```
   - `SessionLocker` interface:
     ```go
     type SessionLocker interface {
         // Acquire takes the exclusive, non-blocking per-session lock for sessionID,
         // creating the lock directory on demand. On contention it fails immediately
         // and returns ErrSessionBusy (errors.Is true) wrapped with a message naming
         // the session id. The lock is held until Release or process death.
         Acquire(ctx context.Context, sessionID string) (SessionLock, error)
     }
     ```
   - Constructors:
     ```go
     // NewSessionLocker returns a SessionLocker using the default lock directory
     // under the user's home.
     func NewSessionLocker() SessionLocker

     // NewSessionLockerWithDir returns a SessionLocker using the given lock
     // directory. Tests inject a temp dir.
     func NewSessionLockerWithDir(dir string) SessionLocker
     ```
   - Concrete `sessionLocker struct { dir string }` and `sessionLock struct { file *os.File }`. `Release()` closes `file` and swallows the already-closed error (returns nil). `File()` returns `file`.
   - `Acquire` (real flock, fail-closed, no `context.Background()`):
     1. `os.MkdirAll(l.dir, 0700)` — created on demand, parent dirs included. On error: `errors.Wrap(ctx, err, "create session lock dir")`.
     2. Open `filepath.Join(l.dir, sessionID+".lock")` with `os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)`. On error: `errors.Wrap(ctx, err, "open session lock file")`.
     3. `syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)`. If the error `errors.Is(err, syscall.EWOULDBLOCK)` → close the fd and return `errors.Wrapf(ctx, ErrSessionBusy, "session %s already has a live claude process", sessionID)`. Any other flock error → close the fd and `errors.Wrap(ctx, err, "lock session file")`.
     4. Clear close-on-exec so the fd survives exec on the resume path: `unix.FcntlInt(int(f.Fd()), unix.F_SETFD, 0)` — import `unix "golang.org/x/sys/unix"` in this file; FcntlInt is NOT in stdlib syscall. On error → close and `errors.Wrap(ctx, err, "clear close-on-exec on session lock fd")`. This is load-bearing, not optional (AC3).
     5. Return `&sessionLock{file: f}`.
   - Default dir (documented, under the user's home on a real local persistent filesystem — never tmpfs):
     ```go
     // defaultSessionLockDir returns the default per-session lock directory under the
     // user's home (real local filesystem, never tmpfs — a lock on a mount cleared on
     // reboot would reopen the double-writer window). Subpath chosen by you, e.g.
     // ~/.claude/session-locks/. Fail closed if the home dir cannot be resolved.
     func defaultSessionLockDir() string
     ```
     `NewSessionLocker()` returns `NewSessionLockerWithDir(defaultSessionLockDir())`. Keep the factory pure: no I/O in the constructor.

3. Wire the locker into the spawn path in `/workspace/pkg/ops/claude_session.go`:
   - Add field `locker SessionLocker` to `claudeSessionStarter` (line 155).
   - Change both constructors to require it (Go has no default parameters — every caller is updated in this prompt):
     ```go
     func NewClaudeSessionStarter(claudeScript string, locker SessionLocker) ClaudeSessionStarter
     func NewClaudeSessionStarterWithRunner(
         claudePath string,
         runCmd func(ctx context.Context, args []string, dir string) ([]byte, error),
         detachRun func(args []string, dir string, stdout *os.File) (<-chan error, error),
         waiter libtime.WaiterDuration,
         locker SessionLocker,
     ) ClaudeSessionStarter
     ```
     Both assign the new field.
   - In `StartSession` (line 169), acquire the lock FIRST, before building `args` and before either branch, and release on every return path:
     ```go
     lock, err := c.locker.Acquire(ctx, sessionID)
     if err != nil {
         return err
     }
     defer func() { _ = lock.Release() }()
     ```
     The deferred release covers clean child exit, child exit error, ctx cancel, the 30m bound expiry, the interactive 5m timeout, and validation errors — all return paths. Do NOT pre-persist the id; nothing about the persist ordering changes.
   - Interface `ClaudeSessionStarter` and `StartSession`'s signature stay EXACTLY as-is. Do not touch `defaultCommandRunner`, `defaultDetachedRunner`, `validateSessionTurn` or its error strings, the 5m `context.WithTimeout`, `sessionTurnTimeout`, or the `Setpgid` child.

4. Wire the locker into the resume path in `/workspace/pkg/ops/claude_resume.go`:
   - Add field `locker SessionLocker` to `claudeResumer` (line 60).
   - Change both constructors to require it:
     ```go
     func NewClaudeResumer(claudeScript string, locker SessionLocker) ClaudeResumer
     func NewClaudeResumerForTesting(
         claudePath string,
         chdir func(string) error,
         execFn func(string, []string, []string) error,
         locker SessionLocker,
     ) ClaudeResumer
     ```
   - In `ResumeSession` (line 66), acquire the lock FIRST (before `chdir`, so a busy session is refused before any state change), then `chdir`, then exec:
     ```go
     lock, err := c.locker.Acquire(ctx, sessionID)
     if err != nil {
         return err // ErrSessionBusy — hard failure, exec never invoked
     }
     if err := c.chdir(cwd); err != nil {
         _ = lock.Release()
         return errors.Wrapf(ctx, err, "change directory to %s", cwd)
     }
     args := []string{"claude", "--resume", sessionID}
     if strings.TrimSpace(prompt) != "" {
         args = append(args, prompt)
     }
     if err := c.execFn(c.claudePath, args, os.Environ()); err != nil {
         _ = lock.Release()
         return errors.Wrap(ctx, err, "exec claude resume")
     }
     return nil
     ```
     Do NOT release on the success path: `syscall.Exec` never returns on success, and the whole point is that the lock fd survives the replacement so the resumed claude holds it until it exits. In tests the fake `execFn` may return nil — that leaves the lock held in the test process, which is expected and contained by the per-spec temp dir (see requirement 9). Interface `ClaudeResumer` and `ResumeSession`'s signature stay EXACTLY as-is.

5. Wire the CLI in `/workspace/pkg/cli/cli.go` at BOTH work-on command sites:
   - Task work-on (~line 374): create `locker := ops.NewSessionLocker()` once before the `dispatcher.FirstSuccess` call, then pass it to both constructors inside the closure: `ops.NewClaudeSessionStarter(vault.GetClaudeScript(), locker)` and `ops.NewClaudeResumer(vault.GetClaudeScript(), locker)`.
   - Goal work-on (~line 1503): identical — one `ops.NewSessionLocker()`, both constructors gain `locker`.
   - No other CLI call sites construct a starter/resumer (verified: only these two).

6. In `/workspace/pkg/ops/export_test.go`, add the test-only export seam for the default lock dir (mirrors the `SessionTurnTimeout` precedent) so the AC6 test can assert the default resolves under the user's home:
   ```go
   // DefaultSessionLockDir exposes the unexported defaultSessionLockDir so tests can
   // assert the default lock directory resolves under the user's home. Test-only.
   var DefaultSessionLockDir = defaultSessionLockDir
   ```

7. Create `/workspace/pkg/ops/session_lock_test.go` (package `ops_test`, Ginkgo v2 + Gomega) with a real flock locker on a temp dir (`ops.NewSessionLockerWithDir`), never a fake. Create a fresh temp dir per spec in `BeforeEach` (e.g. `os.MkdirTemp("", "vault-session-lock-*")`) and remove it in `DeferCleanup`. Tests:
   - **AC1 locker contract**: acquire for a session id → `BeNil()`. Second acquire on the same id → `Expect(err).To(MatchError(ops.ErrSessionBusy))`. After `Release()`, re-acquire → `BeNil()`. Process-death proxy: acquire again, then close the fd WITHOUT calling `Release` (`lock.File().Close()`), then re-acquire → `BeNil()` (flock is freed by the fd closing — the same mechanism that makes SIGKILL release it).
   - **AC6 lock dir**: acquire with `ops.NewSessionLockerWithDir(filepath.Join(tempDir, "nested", "locks"))` (a non-existent nested path) → succeeds and the lock file exists at exactly `<dir>/<session-id>.lock` (assert with `os.Stat` / `BeAnExistingFile`). The default dir resolves under the user's home: `Expect(ops.DefaultSessionLockDir()).To(HavePrefix(os.UserHomeDir()))`.
   - **Failure Modes row 1 (fail-closed)**: a lock dir that cannot be created (e.g. a path whose parent is a regular file) → `Acquire` returns a non-nil error wrapping the create failure (never a nil "success"); the wrapped error message contains "create session lock dir". A second call on a different session id must fail the same way — an unguarded spawn is never possible.

8. Update `/workspace/pkg/ops/claude_session_test.go`:
   - Mechanical: add the `locker` argument (a real `ops.NewSessionLockerWithDir(tempDir)` from a new shared `BeforeEach` temp dir, removed via `DeferCleanup`) as the final argument to EVERY `NewClaudeSessionStarterWithRunner(...)` call in this file. Keep all existing assertions intact.
   - Add a `Context("session lock lifecycle", ...)` using the real temp-dir locker and a `detachRun` fake that records whether it was invoked (spawn counter). Each spec creates its own fresh temp dir so a held/leaked lock from a prior spec can never bleed into the next.
     - **AC2 busy path**: pre-acquire the lock for the session id via `locker.Acquire`, then `starter.StartSession(ctx, sessionID, "prompt", "/vault", "", false)` → `errors.Is(err, ops.ErrSessionBusy)` is true AND the spawn counter stays 0 (the `detachRun` fake was never called — no child is ever spawned). Release the pre-acquired lock after.
     - **AC2 release on every return path**: for each of the four paths below, run `StartSession`, assert its existing error/result, then assert a FRESH `locker.Acquire` of the same session id succeeds (`BeNil()`) and release it — proving the lock was released, never leaked:
       1. clean non-interactive turn (fake `detachRun` writes valid turn JSON, returns a nil-error channel; blocking waiter) → `err` nil.
       2. child exit error (fake `detachRun` returns a channel carrying `ErrTest`) → error contains "exited with error".
       3. 30m bound expiry (fake waiter returns nil immediately) → error contains "did not complete within".
       4. ctx cancel (fake waiter returns `context.Canceled`) → error contains "wait cancelled".
     These four `It`s reuse the existing fake shapes in this file (the non-interactive branch Context at line ~246); the only addition is the fresh-acquire assertion after each call.

9. Update `/workspace/pkg/ops/claude_resume_test.go`:
   - Mechanical: add the `locker` argument (real temp-dir locker, fresh `BeforeEach` temp dir per spec) as the final argument to both `NewClaudeResumerForTesting(...)` calls (lines 38, 140). Keep all existing assertions intact — in particular the existing "chdir fails" test still asserts the error contains "change directory" and `capturedArgv0` is empty (the acquire succeeds on an unlocked session, then chdir fails and the lock is released).
   - Add a `Context("session lock", ...)` with the real temp-dir locker:
     - **AC3 busy path**: pre-acquire the lock for `"session-abc"`, then `resumer.ResumeSession(ctx, "session-abc", "/vault/path", "")` → `errors.Is(err, ops.ErrSessionBusy)` is true, `capturedArgv0` is empty (the exec function was NOT invoked), and `capturedChdirDir` is empty (the refusal happens before chdir). Release the pre-acquired lock.
     - **AC3 exec survival (FD_CLOEXEC)**: acquire a lock via the real locker and assert its fd is NOT close-on-exec:
       ```go
       lock, err := locker.Acquire(ctx, "session-abc")
       Expect(err).To(BeNil())
       flags, ferr := unix.FcntlInt(int(lock.File().Fd()), unix.F_GETFD, 0)
       Expect(ferr).To(BeNil())
       Expect(flags & unix.FD_CLOEXEC).To(BeZero())
       Expect(lock.Release()).To(Succeed())
       ```
       Add the `unix "golang.org/x/sys/unix"` import to this file.
     - **AC3 normal path still execs**: with the session unlocked, `ResumeSession` calls the exec function (existing "successful resume" tests already cover this — leave them unchanged).

10. Update `/workspace/pkg/ops/workon_test.go`, `/workspace/pkg/ops/goal_workon_test.go`, `/workspace/pkg/ops/workon_session_writeback_test.go`:
    - Mechanical: add a real temp-dir locker (`ops.NewSessionLockerWithDir(os.MkdirTemp(...))`, per-spec cleanup) as the final argument to every `NewClaudeSessionStarterWithRunner(...)` call in these files (workon_test.go line 896; goal_workon_test.go lines 368, 414, 479; workon_session_writeback_test.go inside `newStarter` at line 57 and at line 294). Keep every existing assertion intact.
    - **AC4 task work-on busy refusal** (in workon_test.go): a Context where `workOnOp` is rebuilt (mirror the `realStarter` pattern at line ~926) with the real temp-dir locker, a real starter whose `runCmd`/`detachRun` fakes would record a spawn, and the mock `taskStorage`. Pre-acquire the lock for `pinnedSessionID` via the locker, then run `Execute(...)` (isInteractive=false). Assert: `errors.Is(err, ops.ErrSessionBusy)` true, `result.Success` false, and `result.Error` names the session (contains `pinnedSessionID`). Release the pre-acquired lock after. This drives the refusal through the real work-on dispatch path with the real flock syscall (level-2 boundary test).
    - **AC4 goal work-on busy refusal** (in goal_workon_test.go): the same shape with `NewGoalWorkOnOperation` and the real temp-dir locker.
    - Do NOT change the existing `ErrStarterUnavailable` tests in either file — the soft-warning path (warning, exit 0) is unchanged; the new busy tests assert the HARD-failure branch.

11. Update `/workspace/pkg/ops/claude_session_detach_test.go`: the single `ops.NewClaudeSessionStarter(script)` call gains a real temp-dir locker (`ops.NewSessionLockerWithDir(...)`). Keep the test's semantics (real throwaway script, detached-child-outlives-parent) intact.

12. Self-check before finishing: re-run every command in `<verification>` and confirm it passes; then walk spec 042 acceptance criteria 1-6 and 8 against the actual change and state which edit satisfies each. Do NOT add a CHANGELOG entry or docs in this prompt — AC7 (docs + CHANGELOG) is the next prompt in this spec.
</requirements>

<constraints>
- **Injection, never a package global.** The locker is injected into the launch path exactly like `uuidGenerator` / `libtime.CurrentDateTime`; no package-level lock state. The lock dir is configurable and defaults under the user's home on a real local persistent filesystem (not tmpfs); created on demand.
- **Lock mode.** `LOCK_EX|LOCK_NB` via the syscall (no `flock(1)` binary; the flock syscall works on macOS and Linux). Contention → `ErrSessionBusy`, a new sentinel beside `ErrStarterUnavailable` in `pkg/ops/errors.go`, wrapped via `github.com/bborbe/errors` so `errors.Is(err, ErrSessionBusy)` is true and the message names the session id. No blocking waits, no retry loops.
- **Error idiom.** `stderrors.New` sentinels + `errors.Wrap`/`errors.Wrapf(ctx, …)`; no `fmt.Errorf`; no bare `return err`; no `context.Background()` in `pkg/`.
- **Fail-closed.** A lock dir that is missing or unwritable fails work-on HARD with a clear wrapped error — never spawn unguarded (an unguarded spawn is the exact double-writer hazard this spec removes). Never let a nil locker silently disable locking.
- **Interface signatures unchanged.** `ClaudeSessionStarter` and `ClaudeResumer` method signatures stay as-is, so the counterfeiter mocks in `mocks/` need no regeneration — do not hand-edit them (the `make generate` step inside `make precommit` regenerates them to identical content, which is fine). Only the concrete constructors and the CLI wiring gain the locker.
- **Interactive branch behavior unchanged.** The 5m TTY cap, `validateSessionTurn` and its byte-identical error strings, `defaultCommandRunner`, and the detached `Setpgid` child are untouched. The interactive resume still execs into `claude --resume`; the lock fd MUST survive that exec (FD_CLOEXEC cleared) so the resumed claude holds it until exit — load-bearing, not optional.
- **`scenarios/005-work-on-resume-auto-invokes-subtask.md` stays byte-identical** (AC5): `git diff --exit-code HEAD -- scenarios/005-...` must be empty.
- **Turn bound is a wait, never a kill.** `sessionTurnTimeout` (30m) stays a wait bound; the child stays detached. The lock is released by the parent's deferred unlock when it stops waiting (child exit, child error, ctx cancel, or bound expiry). The detached child that survives keeps running WITHOUT the parent's lock — safe because its id is never persisted on those paths; do NOT pre-persist the id.
- **Lock scope = launch path only.** The cached-id non-interactive re-persist (existing `task.ClaudeSessionID() != ""` branch) spawns no writer and takes no lock here; liveness gating there belongs to the vault-ui follow-on spec.
- **No new config surface, no opt-out, no tunables.** The lock is the invariant, not a feature that can be switched off. The only lock-directory surface is the default + the test constructor's dir param.
- **No stale-lock machinery.** No cleanup sweeps, no compensating clears, no lock TTL — the kernel releases the flock when the holding process exits (normal exit, crash, SIGKILL).
- **Lock file hygiene.** Lock files are empty (existence + flock state only) with owner-only perms (dir 0700, file 0600); the default dir lives under the user's home and must not be world-writable.
- **Real flock in tests, no mock locker.** Every lock test uses `ops.NewSessionLockerWithDir(tempDir)` against the real `syscall.Flock`; a fresh temp dir per spec so a held/leaked fd can never bleed across specs.
- `pkg/ops/` stays a pure library — structured returns, no stdout. Tests use Ginkgo v2 / Gomega, external `ops_test` package, new code ≥80% coverage.
- Do NOT commit — dark-factory handles git. `.git` is visible in this container (`workflow: direct`, no `hideGit`), so git-based verification works; the prohibition is a scope rule.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

The sentinel exists and the refusal path is exercised in every layer (AC1-AC4):

```
grep -c 'ErrSessionBusy' pkg/ops/errors.go                  # must be >= 1
grep -rn 'ErrSessionBusy' pkg/ops/claude_session_test.go    # must be >= 1
grep -rn 'ErrSessionBusy' pkg/ops/claude_resume_test.go     # must be >= 1
grep -rn 'ErrSessionBusy' pkg/ops/workon_test.go            # must be >= 1
grep -rn 'ErrSessionBusy' pkg/ops/goal_workon_test.go       # must be >= 1
grep -rn 'ErrSessionBusy' pkg/ops/session_lock_test.go      # must be >= 1
```

The interactive branch and scenario 005 are byte-identical (AC5):

```
grep -c 'defaultCommandRunner' pkg/ops/claude_session.go    # must be >= 1
grep -c 'context.WithTimeout' pkg/ops/claude_session.go     # must be >= 1 (5m TTY cap)
git diff --exit-code HEAD -- scenarios/005-work-on-resume-auto-invokes-subtask.md  # must exit 0 (empty)
```

The lock fd is not close-on-exec (AC3) and the locker is wired into the launch path (no package global):

```
grep -n 'F_SETFD' pkg/ops/session_lock.go                   # must be >= 1 line (FD_CLOEXEC cleared)
grep -n 'locker SessionLocker' pkg/ops/claude_session.go    # must be >= 1 line (starter struct)
grep -n 'locker SessionLocker' pkg/ops/claude_resume.go     # must be >= 1 line (resumer struct)
grep -rn 'NewSessionLocker' pkg/cli/cli.go                  # must be >= 1 (wiring)
```

The default lock dir is under the user's home (AC6):

```
grep -n 'UserHomeDir' pkg/ops/session_lock.go               # must be >= 1 line
```

Then run the full gate once (AC8):

```
make precommit
go test ./... -race
```

Both must exit 0. If anything fails, fix and re-run only the failing target until green, then re-run `make precommit` once and `go test ./... -race` once.

Before finishing, re-run every command in this block and confirm each passes, then walk spec 042's AC1, AC2, AC3, AC4, AC5, AC6 and AC8 one at a time against the actual change and state which edit satisfies each. Do not report success on any criterion whose evidence you have not run.
</verification>
