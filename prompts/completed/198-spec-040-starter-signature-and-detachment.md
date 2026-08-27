---
status: completed
spec: [040-bug-session-start-blocks-on-full-headless-turn]
summary: Reworked ClaudeSessionStarter.StartSession to take a caller-minted session id and return only an error, added a detached non-interactive runner (exec.Command + os.DevNull + Setpgid) behind an injectable seam returning within a 10s liveness window, plumbed isInteractive and an injected uuid generator through the task/goal work-on operations and cli.go, regenerated the mock, rewrote/extended the unit and integration tests, and added the CHANGELOG entry
execution_id: vault-cli-session-fast-return-exec-198-spec-040-starter-signature-and-detachment
dark-factory-version: dev
created: "2026-08-27T12:00:00Z"
queued: "2026-08-27T10:51:51Z"
started: "2026-08-27T10:52:19Z"
completed: "2026-08-27T11:08:52Z"
branch: dark-factory/bug-session-start-blocks-on-full-headless-turn
---

<summary>
- `StartSession` stops promising to return a session id and instead takes the caller-minted session id as a parameter and returns only an error — the session id is generated caller-side so it can be persisted before the child process exists.
- The starter gains a second, detached runner: on the non-interactive branch the child is spawned with `exec.Command` (not `CommandContext`), stdout/stderr go to `os.DevNull`, `Setpgid` puts it in its own process group, and `StartSession` returns once the child has outlived a 10s liveness window — the bootstrap turn then continues after the CLI exits.
- The TTY/interactive branch is behaviourally unchanged: same blocking `runCmd`, same 5m `context.WithTimeout` cap, same JSON validation. Only the timeout error string changes (from the misleading "session start timed out" to a bootstrap-turn-named message).
- `isInteractive` is plumbed from `Execute` through `handleClaudeSession` to `StartSession` in both the task and goal paths; the uuid generator is injected on both operations (mirroring the injected clock), so tests can pin the session id.
- The counterfeiter mock is regenerated; `claude_session_test.go`, `workon_test.go`, `goal_workon_test.go`, and `workon_session_writeback_test.go` are mechanically updated for the new signature (stub arity, error-only return, pinned uuid).
- A new integration test spawns a real throwaway script and proves the child outlives the parent: the sentinel file appears after `StartSession` returns and the enclosing context is cancelled.
- `scenarios/005` is not touched; the misleading error string is gone repo-wide; the `## Unreleased` changelog bullet is added.
</summary>

<objective>
Change `ClaudeSessionStarter.StartSession` to take a caller-minted `sessionID` and return only an error, introduce a second detached runner behind a new injectable seam on `claudeSessionStarter` so the non-interactive branch returns within a 10s liveness window while the interactive branch keeps its exact current blocking behaviour, plumb `isInteractive` through the task and goal work-on call sites, inject the uuid generator on both operations, regenerate the mock, and mechanically update every affected test so the tree compiles and the AC1/AC2/AC3/AC5/AC7/AC10/AC11 acceptance criteria of spec 040 are met.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/storage layering rules.

Read these files fully before making changes:

- `/workspace/pkg/ops/claude_session.go` — the primary file. Current interface:
  ```go
  type ClaudeSessionStarter interface {
      // StartSession runs claude in headless mode to create a session, returns session_id.
      // When name is non-empty, the session is created with -n <name> so its
      // custom-title and agent-name are set from turn 1.
      StartSession(ctx context.Context, prompt string, cwd string, name string) (string, error)
  }
  ```
  `defaultCommandRunner` uses `exec.CommandContext(...)` + `cmd.Output()`; `StartSession` wraps everything in `context.WithTimeout(ctx, 5*time.Minute)`, builds `args := []string{c.claudePath, "--print"}` (plus `-n <name>`, `-p <prompt>`, `--output-format json`), and returns the `session_id` parsed from the JSON blob after validating `empty session_id` / `0 turns` / `is_error`. The timeout error string is `"claude session start timed out after 5m"` (the misleading string AC11 removes).

- `/workspace/pkg/ops/workon.go` — `workOnOperation.handleClaudeSession` (the fresh-start path calls `w.starter.StartSession(ctx, prompt, sessionDir, task.Name)` then `persistSessionAndMetrics`), `Execute` (the single call site `w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault)` at line 113), and `NewWorkOnOperation` (constructor at line 36).

- `/workspace/pkg/ops/goal_workon.go` — the goal-side mirror: `goalWorkOnOperation.handleClaudeSession`, `Execute` (call site at line 104), `NewGoalWorkOnOperation` (constructor at line 34).

- `/workspace/pkg/ops/claude_session_test.go` — the three `NewClaudeSessionStarterWithRunner` call sites (lines 34, 151, 184) you must update, and all the `StartSession` call sites whose arity changes.

- `/workspace/pkg/ops/workon_session_writeback_test.go` — bounded mechanical edit only (see requirements): the two `StartSessionStub` assignments (lines 85, 160), the five `Equal("session-123")` assertions (lines 121, 131, 136, 192, 202), and the two `NewWorkOnOperation` / `NewGoalWorkOnOperation` constructor calls (lines 111, 184). Every invariant assertion must survive.

- `/workspace/pkg/ops/workon_test.go` and `/workspace/pkg/ops/goal_workon_test.go` — constructor calls, `mockStarter.StartSessionReturns(...)` arity, `StartSessionArgsForCall(0)` destructuring, and the `Equal("session-123")` assertions on the resumed/generated session id.

- `/workspace/pkg/cli/cli.go` — the two operation-constructor call sites (lines 380 and 1507) that must pass the uuid generator. It does not currently import `github.com/google/uuid`.

- `/workspace/mocks/claude-session-starter.go` — regenerated by `make generate`; do not hand-edit.

- `/workspace/scenarios/005-work-on-resume-auto-invokes-subtask.md` — do NOT touch; AC3 requires it byte-identical.

Verified library APIs (grep-verified in module source — do not invent):
- `libtime.WaiterDuration` (`github.com/bborbe/time`, in repo go.mod): `type WaiterDuration interface { Wait(ctx context.Context, duration Duration) error }`; `func NewWaiterDuration() WaiterDuration`; `type WaiterDurationFunc func(ctx context.Context, duration Duration) error` implements `Wait`. `Duration` is `type Duration stdtime.Duration` with `const Second Duration = 1000 * Millisecond`.
- `uuid.NewString` (`github.com/google/uuid`, direct dep): `func NewString() string` — a valid `func() string` value for the injected generator. `func Parse(s string) (UUID, error)`.
- `syscall.SysProcAttr{Setpgid: true}` and `os.DevNull` are stdlib.
- `github.com/bborbe/errors`: `errors.Wrap(ctx, err, "...")` and `errors.Errorf(ctx, "...", args...)` are already used in `claude_session.go` — mirror them.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega with Counterfeiter fakes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` context wrapping, never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-concurrency-patterns.md` — the two bounded `go func()` uses in this prompt are deliberate: a detached-child reaper feeding a buffered `cmd.Wait()` channel, and an injectable-timer channel. They are not orchestrated concurrent work, so `run.CancelOnFirstErrorWait` does not apply; both goroutines are bounded by the liveness window and write to buffered channels (no leak, no blocking on send).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` and `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>
1. In `/workspace/pkg/ops/claude_session.go`, change the `ClaudeSessionStarter` interface to the new contract:
   ```go
   // ClaudeSessionStarter starts a new headless Claude session.
   type ClaudeSessionStarter interface {
       // StartSession runs claude in headless mode to create a session with the given
       // session id and returns only an error. The session id is minted by the caller
       // (not by claude) so it can be persisted before the child process exists.
       // When name is non-empty, the session is created with -n <name> so its
       // custom-title and agent-name are set from turn 1.
       // On the interactive branch it blocks until the headless turn completes (bounded
       // by a 5m timeout) and validates the JSON result. On the non-interactive branch
       // it spawns the child detached from the request context and returns within the
       // liveness window once the child has proven it survives startup. See
       // docs/work-on-session-lifecycle.md.
       StartSession(ctx context.Context, sessionID string, prompt string, cwd string, name string, isInteractive bool) error
   }
   ```

2. Add a package constant and new fields to the struct. At file scope:
   ```go
   // livenessWindow is how long the non-interactive branch waits for the detached child
   // to prove it survived startup (auth failure, bad flag). Tunable constant; no config
   // field unless a second caller needs one.
   const livenessWindow = 10 * libtime.Second
   ```
   The struct becomes:
   ```go
   type claudeSessionStarter struct {
       claudePath     string
       maxTurns       int // -1 = no limit, >0 = limit. Keep the field and keep the
                          // `if c.maxTurns > 0 { args = append(args, "--max-turns", …) }`
                          // branch in the rewritten StartSession. It is already inert
                          // (both constructors hardcode -1, no test sets it positive),
                          // but dropping it silently changes the struct's contract —
                          // out of scope for this bug fix.
       runCmd         func(ctx context.Context, args []string, dir string) ([]byte, error)
       detachRun      func(args []string, dir string) (<-chan error, error)
       waiter         libtime.WaiterDuration
       livenessWindow libtime.Duration
   }
   ```
   Add imports `"os"`, `"syscall"`, and `libtime "github.com/bborbe/time"`. The existing `"time"` import stays (used by `5*time.Minute`).

3. Add `defaultDetachedRunner`. Its body must contain neither `CommandContext` nor `cmd.Output()` (AC1 negative evidence), and must contain `Setpgid` and `os.DevNull` (AC1 positive evidence):
   ```go
   // defaultDetachedRunner is the non-interactive runner. It spawns the child detached
   // from the request context: exec.Command (not CommandContext), stdout/stderr
   // redirected to os.DevNull so the child never dies on EPIPE when the parent exits,
   // and Setpgid so it lives in its own process group and survives the parent. It
   // returns a buffered channel that receives cmd.Wait()'s error, plus a spawn error
   // when Start fails. The child may outlive this process by minutes; that is the
   // point of the detachment, not an accident.
   func defaultDetachedRunner(args []string, dir string) (<-chan error, error) {
       cmd := exec.Command(args[0], args[1:]...) //#nosec G204 -- args[0] is the claude binary path from LookPath
       cmd.Dir = dir
       // os.DevNull is a string constant ("/dev/null"), NOT an io.Writer — assigning it
       // directly to cmd.Stdout does not compile. Open it as a file. Leaving Stdout/Stderr
       // nil would also route to /dev/null, but AC1 requires an explicit os.DevNull
       // reference so the detachment is deliberate rather than accidental.
       devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
       if err != nil {
           return nil, err // caller wraps with ctx; never context.Background() here
       }
       cmd.Stdout = devNull
       cmd.Stderr = devNull
       cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
       if err := cmd.Start(); err != nil {
           _ = devNull.Close()
           return nil, err
       }
       done := make(chan error, 1)
       go func() {
           err := cmd.Wait()
           _ = devNull.Close() // close only after the child exits — closing earlier would
                               // hand the still-running child a dead fd
           done <- err
       }()
       return done, nil
   }
   ```

   Note the fd lifetime: the parent may return from `StartSession` long before this goroutine runs. That is intended — the goroutine outlives the call and closes the fd whenever the child finally exits. If the parent process exits first, the OS reclaims it.

4. Update the comment above `defaultCommandRunner` so the surrounding comment names it the interactive runner (AC1 requires `exec.CommandContext` appear on a line whose comment names the interactive runner):
   ```go
   // defaultCommandRunner is the interactive (blocking) runner. It runs the command
   // under the request context and returns combined stdout, so StartSession blocks
   // through the whole headless turn before any output is available.
   ```

5. Change both constructors:
   ```go
   func NewClaudeSessionStarter(claudeScript string) ClaudeSessionStarter {
       claudePath, err := exec.LookPath(claudeScript)
       if err != nil {
           return nil
       }
       return &claudeSessionStarter{
           claudePath:     claudePath,
           maxTurns:       -1,
           runCmd:         defaultCommandRunner,
           detachRun:      defaultDetachedRunner,
           waiter:         libtime.NewWaiterDuration(),
           livenessWindow: livenessWindow,
       }
   }

   func NewClaudeSessionStarterWithRunner(
       claudePath string,
       runCmd func(ctx context.Context, args []string, dir string) ([]byte, error),
       detachRun func(args []string, dir string) (<-chan error, error),
       waiter libtime.WaiterDuration,
   ) ClaudeSessionStarter {
       return &claudeSessionStarter{
           claudePath:     claudePath,
           maxTurns:       -1,
           runCmd:         runCmd,
           detachRun:      detachRun,
           waiter:         waiter,
           livenessWindow: livenessWindow,
       }
   }
   ```
   Note: `detachRun` may be nil only on starters whose interactive branch is the only one exercised in tests. Do not add a nil check for it — the non-interactive branch requires it.

6. Rewrite `StartSession`:
   ```go
   func (c *claudeSessionStarter) StartSession(
       ctx context.Context,
       sessionID string,
       prompt string,
       cwd string,
       name string,
       isInteractive bool,
   ) error {
       args := []string{
           c.claudePath,
           "--print",
       }
       if name != "" {
           args = append(args, "-n", name)
       }
       args = append(args, "-p", prompt, "--output-format", "json", "--session-id", sessionID)

       if !isInteractive {
           // Non-interactive branch: spawn detached and return once the child has
           // outlived the liveness window. The child keeps running after this process
           // exits (the Vault UI Start button gets its session id back in ~10s).
           done, err := c.detachRun(args, cwd)
           if err != nil {
               return errors.Wrap(ctx, err, "start detached claude session")
           }
           waitCh := make(chan error, 1)
           go func() {
               waitCh <- c.waiter.Wait(ctx, c.livenessWindow)
           }()
           select {
           case exitErr := <-done:
               return errors.Errorf(ctx, "claude session exited during startup: %v", exitErr)
           case err := <-waitCh:
               if err != nil {
                   // The request context was cancelled mid-window. The child is detached
                   // and survives on its own; the parent is exiting anyway.
                   return nil
               }
               return nil
           }
       }

       // Interactive branch, unchanged behaviour: block through the headless turn so
       // turn 2's `claude --resume` reads turn 1's completed on-disk result.
       timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
       defer cancel()

       output, err := c.runCmd(timeoutCtx, args, cwd)
       if err != nil {
           if timeoutCtx.Err() == context.DeadlineExceeded {
               return errors.Errorf(ctx, "claude bootstrap turn timed out after 5m")
           }
           return errors.Wrap(ctx, err, "run claude")
       }

       var result struct {
           SessionID string `json:"session_id"`
           NumTurns  int    `json:"num_turns"`
           IsError   bool   `json:"is_error"`
           Result    string `json:"result"`
       }
       if err := json.Unmarshal(output, &result); err != nil {
           return errors.Wrap(ctx, err, "parse claude output")
       }

       if result.SessionID == "" {
           return errors.Errorf(ctx, "claude returned empty session_id")
       }

       if result.NumTurns == 0 {
           return errors.Errorf(ctx, "claude returned 0 turns: %s", result.Result)
       }

       if result.IsError {
           return errors.Errorf(ctx, "claude reported error: %s", result.Result)
       }

       return nil
   }
   ```
   The interactive branch's deadline source is `context.WithTimeout` exactly once in this file (AC3), and `claude returned 0 turns` appears exactly once (AC3). The old string `"claude session start timed out after 5m"` is gone from the whole repo (AC11). The liveness-wait goroutine is bounded by `livenessWindow`; it writes to a buffered channel so it never blocks.

7. Add the injected uuid generator to both operations. In `/workspace/pkg/ops/workon.go`:
   - Add field `uuidGenerator func() string` to `workOnOperation`.
   - New constructor:
     ```go
     func NewWorkOnOperation(
         taskStorage storage.TaskStorage,
         dailyNoteStorage storage.DailyNoteStorage,
         currentDateTime libtime.CurrentDateTime,
         uuidGenerator func() string,
         starter ClaudeSessionStarter,
         resumer ClaudeResumer,
     ) WorkOnOperation {
         return &workOnOperation{
             taskStorage:      taskStorage,
             dailyNoteStorage: dailyNoteStorage,
             currentDateTime:  currentDateTime,
             uuidGenerator:    uuidGenerator,
             starter:          starter,
             resumer:          resumer,
         }
     }
     ```
   In `/workspace/pkg/ops/goal_workon.go`:
   - Add field `uuidGenerator func() string` to `goalWorkOnOperation`.
   - New constructor:
     ```go
     func NewGoalWorkOnOperation(
         goalStorage storage.GoalStorage,
         uuidGenerator func() string,
         starter ClaudeSessionStarter,
         resumer ClaudeResumer,
     ) GoalWorkOnOperation {
         return &goalWorkOnOperation{
             goalStorage:   goalStorage,
             uuidGenerator: uuidGenerator,
             starter:       starter,
             resumer:       resumer,
         }
     }
     ```

8. Plumb `isInteractive` and the generated id in `/workspace/pkg/ops/workon.go`:
   - New `handleClaudeSession` signature: `func (w *workOnOperation) handleClaudeSession(ctx context.Context, task *domain.Task, vaultPath string, sessionDir string, vault *config.Vault, isInteractive bool) (string, error)`.
   - Inside, on the fresh-start path, mint the id and pass it (this prompt does NOT change the persist ordering — `persistSessionAndMetrics` still runs after `StartSession` returns, exactly as today):
     ```go
     sessionID := w.uuidGenerator()
     slog.Info("starting claude session", "task", task.Name)
     if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
         return "", errors.Wrap(ctx, err, "start claude session")
     }
     startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
     return persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
     ```
   - Update the single call site in `Execute` (line 113): `sessionID, sessionErr := w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault, isInteractive)`.
   - The cached path (`task.ClaudeSessionID() != ""`) is unchanged.

9. Mirror requirement 8 in `/workspace/pkg/ops/goal_workon.go`: new signature `func (g *goalWorkOnOperation) handleClaudeSession(ctx context.Context, goal *domain.Goal, vaultPath string, sessionDir string, vault *config.Vault, isInteractive bool) (string, error)`; mint via `sessionID := g.uuidGenerator()`; call `g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive)`; update the `Execute` call site (line 104). The cached path (`goal.ClaudeSessionID() != ""`) is unchanged.

10. Update `/workspace/pkg/cli/cli.go`: add `"github.com/google/uuid"` to the imports, then pass `uuid.NewString` (the `func() string` value) to both constructors:
    - Line ~380: `workOnOp := ops.NewWorkOnOperation(taskStore, dailyStore, currentDateTime, uuid.NewString, starter, resumer)`.
    - Line ~1507: `workOnOp := ops.NewGoalWorkOnOperation(goalStore, uuid.NewString, starter, resumer)`.

11. Regenerate the mock by running `make generate` (or `go generate -mod=mod ./...`). Do not hand-edit `/workspace/mocks/claude-session-starter.go`. After regeneration its `StartSession` must have signature `func (fake *ClaudeSessionStarter) StartSession(arg1 context.Context, arg2 string, arg3 string, arg4 string, arg5 string, arg6 bool) error` (AC10).

12. Rewrite `/workspace/pkg/ops/claude_session_test.go`:
    - Update all `StartSession` calls to the 6-argument form `StartSession(ctx, "session-abc", "my prompt", "/my/vault", "My Task Title", true)`.
    - Update the three `NewClaudeSessionStarterWithRunner` call sites (lines 34, 151, 184) to the 4-argument constructor; interactive tests pass `nil` for `detachRun` and `libtime.NewWaiterDuration()` for the waiter.
    - Replace the `returns the session_id` assertions (they asserted on the JSON blob's `session_id`, which is no longer returned) with `Expect(err).To(BeNil())`.
    - Update the two argv assertions to include `--session-id` at the end. Expected with no name: `["/bin/claude", "--print", "-p", "my prompt", "--output-format", "json", "--session-id", "session-abc"]`; with name: `["/bin/claude", "--print", "-n", "My Task Title", "-p", "my prompt", "--output-format", "json", "--session-id", "session-abc"]`.
    - Keep the existing interactive-branch tests for `command fails`, `invalid JSON output`, `empty session_id`, `missing session_id field`, `num_turns is zero`, `is_error is true` — they now assert on the returned `error` instead of `(string, error)`. The `num_turns is zero` error must still contain `"0 turns"` and the result text.
    - Add an interactive-branch deadline test (AC3): drive `StartSession(ctx, "id", "prompt", "/vault", "", true)` with a parent context whose deadline is already in the past (e.g. `context.WithDeadline(ctx, time.Now().Add(-time.Minute))`) and a `runCmd` fake that returns an error; assert the returned error contains `"bootstrap turn timed out"`. This proves the interactive branch's deadline source is `context.WithTimeout` (it cannot be moved by an injected clock).
    - Add an interactive-branch blocking test (AC3): a `runCmd` fake that blocks on a channel; call `StartSession` in a goroutine, assert `Consistently(done).ShouldNot(BeClosed())` before releasing the channel, then `Eventually(done).Should(BeClosed())` — proving the interactive branch does not return until the child has exited.
    - Add non-interactive-branch tests (each with `isInteractive=false` and a fake `detachRun`; the fake waiter is `libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil })`):
      - argv capture (AC5): a fake `detachRun` that records `args` and returns `(make(chan error), nil)`; assert `args` contains `"--session-id"`, the id, `"--print"`, and `"-n"` with the name; assert `uuid.Parse("123e4567-e89b-12d3-a456-426614174000")` succeeds.
      - returns within the liveness window (AC2): a never-firing `done` channel; record `start := time.Now()` before the call; assert the call returns nil and `time.Since(start)` is less than `10*time.Second` (mirrors the package constant `livenessWindow` — not a wall-clock tolerance).
      - early exit is an error (AC11): fake `detachRun` returns `(done, nil)` where `done` immediately carries `errors.New("exit status 1")`; assert the returned error contains `"exit status 1"` and `"exited during startup"`.
      - spawn failure: fake `detachRun` returns `(nil, ErrTest)`; assert the returned error is non-nil and wraps `"start detached claude session"` (`ErrTest` is defined in `/workspace/pkg/ops/ops_suite_test.go`).

13. Create `/workspace/pkg/ops/claude_session_detach_test.go` — the AC1 integration test (package `ops_test`, Ginkgo v2). It must use the REAL production starter (via `ops.NewClaudeSessionStarter(scriptPath)`, which wires `defaultDetachedRunner` + the real waiter), never a fake. Shape:
    ```go
    var _ = Describe("ClaudeSessionStarter detachment (integration)", func() {
        It("child outlives the parent and survives context cancellation", func() {
            dir, err := os.MkdirTemp("", "vault-claude-detach-*")
            Expect(err).To(BeNil())
            defer os.RemoveAll(dir)
            sentinel := filepath.Join(dir, "sentinel.txt")
            script := filepath.Join(dir, "worker.sh")
            Expect(os.WriteFile(script, []byte("#!/bin/sh\nsleep 12\ntouch "+sentinel+"\n"), 0755)).To(Succeed())
            starter := ops.NewClaudeSessionStarter(script)
            Expect(starter).NotTo(BeNil())
            ctx, cancel := context.WithCancel(context.Background())
            // The child sleeps past the liveness window (12s > 10s), so StartSession
            // waits out the real window and returns nil while the child still runs.
            err = starter.StartSession(ctx, "123e4567-e89b-12d3-a456-426614174000", "prompt", dir, "worker", false)
            Expect(err).To(BeNil())
            cancel()
            // The sentinel appears only because the detached child survived the
            // parent's context cancellation and process exit.
            Eventually(func() bool {
                _, statErr := os.Stat(sentinel)
                return statErr == nil
            }, "20s", "200ms").Should(BeTrue())
        })
    })
    ```
    This test takes ~10-12s by design (the real liveness window). Note the test's context is cancelled AFTER `StartSession` returns, proving the child is not killed by cancellation.

14. Make the bounded mechanical edit to `/workspace/pkg/ops/workon_session_writeback_test.go` (this file may be edited only mechanically — stub arity, error-only return, and the id constant becoming a `uuid.Parse`-able value; every invariant assertion must survive):
    - Add at file scope: `const pinnedSessionID = "123e4567-e89b-12d3-a456-426614174000"`.
    - The two `StartSessionStub` assignments (lines 85 and 160) become `func(ctx context.Context, _ string, _ string, _ string, _ string, _ bool) error { ... return nil }` — they keep doing the file write (that simulation moves to the runner fake in prompt 2), but no longer return a session id.
    - The five `Equal("session-123")` assertions (lines 121, 131, 136, 192, 202) become `Equal(pinnedSessionID)`.
    - The two constructor calls gain the pinned generator: `ops.NewWorkOnOperation(taskStore, mockDailyNote, currentDateTime, func() string { return pinnedSessionID }, mockStarter, nil)` and `ops.NewGoalWorkOnOperation(goalStore, func() string { return pinnedSessionID }, mockStarter, nil)`.
    - Do not change any other line. The grep counts `TaskPhaseExecution` (2), `GoalPhaseExecution` (2), `session_note` (4), `MetricsSessions()` (2), and `ClaudeSessionID()` (2) must all remain exactly as they are (AC7, verified again in prompt 2).

15. Mechanically update `/workspace/pkg/ops/workon_test.go` and `/workspace/pkg/ops/goal_workon_test.go`:
    - Add the `pinnedSessionID` const to each file.
    - Add the pinned generator as the new constructor argument at every `ops.NewWorkOnOperation(...)` / `ops.NewGoalWorkOnOperation(...)` call (workon_test.go lines 49, 230, 269; goal_workon_test.go lines 42, 209), including the starter-nil contexts.
    - `mockStarter.StartSessionReturns("session-123", nil)` → `mockStarter.StartSessionReturns(nil)`; `mockStarter.StartSessionReturns("", ErrTest)` → `mockStarter.StartSessionReturns(ErrTest)`; the `claude returned 0 turns` returns become error-only too.
    - Update every `StartSessionArgsForCall(0)` destructuring from 4 to 6 values: `_, prompt, _, _` → `_, _, prompt, _, _, _`; `_, _, _, name` → `_, _, _, _, name, _`.
    - The `Equal("session-123")` assertions on the generated/resumed id become `Equal(pinnedSessionID)`: workon_test.go lines 140, 382, 843; goal_workon_test.go line 298.
    - Add plumbing assertions: in the interactive-mode context assert `StartSession` was called with `isInteractive=true`; in a non-interactive context assert it was called with `isInteractive=false`.
    - Do NOT touch the `calls FindTaskByName` / `re-reads the task from the vault path after the session` call-count tests — their counts are unchanged by this prompt (prompt 2 renames the "after the session" one).

16. Add the `## Unreleased` section and one bullet to `/workspace/CHANGELOG.md`, below the preamble and above `## v0.116.2` (the file has no `## Unreleased` yet). Bullet:
    ```
    - fix: `task work-on` / `goal work-on` no longer wait for the entire headless bootstrap turn before returning a session id. `StartSession` now takes a caller-minted session id and, on the non-interactive branch, spawns the child detached (`exec.Command` + `Setpgid` + `os.DevNull`) and returns within a 10s liveness window while the turn continues independently; the TTY branch keeps its blocking 5m behaviour unchanged. The misleading "claude session start timed out" error is renamed to name the bootstrap turn
    ```
    Do NOT bump the version fields in `.claude-plugin/plugin.json` / `.claude-plugin/marketplace.json` and do NOT `git tag` — the `github-releaser` owns that post-merge.

17. Do NOT modify: `pkg/ops/claude_resume.go`, `pkg/storage/`, `scenarios/005-work-on-resume-auto-invokes-subtask.md` (must stay byte-identical — AC3), `pkg/ops/frontmatter_test.go` (its `"claude_session_id": "session-123"` fixtures are arbitrary storage values that do not pass through the generator — leave them alone), the interactive branch's blocking/JSON semantics, or the bootstrap prompt string (which keeps its `--non-interactive` suffix verbatim).
</requirements>

<constraints>
- Two runners, not one: `defaultCommandRunner` (exec.CommandContext + cmd.Output + the JSON-blob validation) stays exactly as it is and serves the interactive branch. A single shared detached runner would silently change three TTY behaviours (unbounded wait, Setpgid stopping Ctrl-C, diagnostics lost to os.DevNull) — all contradict "TTY unchanged".
- The detached child must outlive the parent: `exec.Command`, not `exec.CommandContext`; stdout/stderr to `os.DevNull`; `Setpgid: true`; never cancel or wait on the child past the liveness window. Deliberate detachment, not merely absence of a `Wait`.
- `isInteractive` is the resolved `--mode` flag (`auto | interactive | headless`, pkg/cli/cli.go) — `term.IsTerminal` is only the `auto` arm. The branch is chosen by the flag, not by the tty.
- The TTY start-then-resume flow does not change; `scenarios/005` must remain valid, unmodified (`git diff --exit-code scenarios/005-work-on-resume-auto-invokes-subtask.md` empty).
- Do NOT read the id from a `--output-format stream-json --verbose` init event — rejected deliberately.
- The uuid generator is injected, not called inline; it mirrors the `libtime.CurrentDateTime` pattern. Tests pin a fixed uuid (`pinnedSessionID`), so every equality assertion survives verbatim.
- `pkg/ops/` stays a pure library — structured returns, no stdout.
- Unit tests fake the subprocess and must not sleep through the liveness window (the fake `libtime.WaiterDurationFunc` makes the window instant). The one sanctioned exception is the AC1 integration test, which spawns a real throwaway script — never a real `claude` binary.
- The cached-session path (`task.ClaudeSessionID() != ""`) is unchanged.
- The bootstrap prompt, `--non-interactive`, and everything the turn does are unchanged. This change only alters when the parent stops waiting.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- New code ≥80% statement coverage; tests use Ginkgo v2 / Gomega with Counterfeiter mocks; external test packages (`ops_test`).
- This repo is `autoRelease: true`. Add the `## Unreleased` bullet only; do NOT rename it, do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, do NOT `git tag`.
- Do NOT commit — dark-factory handles git. (Note: `.git` is visible in this container — `.dark-factory.yaml` sets `workflow: direct` with no `hideGit` — so git-based verification works; the prohibition is a scope rule, not a technical impossibility.)
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

The detached runner is in place and the interactive runner is untouched (AC1):

```
grep -c 'exec.CommandContext' pkg/ops/claude_session.go    # must be 1, in defaultCommandRunner (whose comment names the interactive runner)
grep -n 'Setpgid' pkg/ops/claude_session.go                # must be >= 1 line
grep -n 'os.DevNull' pkg/ops/claude_session.go             # must be >= 1 line (a nil cmd.Stdout must NOT be how this passes)
grep -c 'cmd.Output()' pkg/ops/claude_session.go           # must be 1, in defaultCommandRunner only
```

The interactive branch keeps its cap and diagnostics (AC3):

```
grep -c 'context.WithTimeout' pkg/ops/claude_session.go    # must be 1, on the interactive path
grep -c 'claude returned 0 turns' pkg/ops/claude_session.go # must be 1
git diff --exit-code scenarios/005-work-on-resume-auto-invokes-subtask.md   # must exit 0 (file unchanged)
```

The misleading error is gone repo-wide (AC11):

```
grep -rn 'claude session start timed out' pkg/             # must print nothing (exit 1)
```

The mock was regenerated (AC10):

```
grep -c 'func (fake \*ClaudeSessionStarter) StartSession(arg1 context.Context, arg2 string, arg3 string, arg4 string, arg5 string, arg6 bool) error' mocks/claude-session-starter.go
# must be >= 1
git diff --stat mocks/claude-session-starter.go            # must be non-empty (the mock changed vs HEAD)
```

The new tests exist and pass (AC1, AC2, AC5):

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'child outlives the parent and survives context cancellation'
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'returns within the liveness window'
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'bootstrap turn timed out'
```

Each must print a number >= 1.

The writeback test kept its pinned invariants (AC6 counts, held here and re-verified in prompt 2):

```
grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go   # must be 2
grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go   # must be 2
grep -c 'session_note' pkg/ops/workon_session_writeback_test.go         # must be 4
grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go    # must be 2
grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go    # must be 2
```

Changelog:

```
grep -c '^## Unreleased$' CHANGELOG.md                     # must be 1
grep -c 'no longer wait for the entire headless bootstrap turn' CHANGELOG.md   # must be 1
grep -m1 '^## v' CHANGELOG.md                              # must still print ## v0.116.2 (do not bump)
```

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
