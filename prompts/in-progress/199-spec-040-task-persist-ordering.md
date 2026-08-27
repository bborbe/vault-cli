---
status: executing
spec: [040-bug-session-start-blocks-on-full-headless-turn]
execution_id: vault-cli-session-fast-return-exec-199-spec-040-task-persist-ordering
dark-factory-version: dev
created: "2026-08-27T12:00:00Z"
queued: "2026-08-27T10:51:51Z"
started: "2026-08-27T11:08:54Z"
branch: dark-factory/bug-session-start-blocks-on-full-headless-turn
---

<summary>
- The task work-on fresh path persists the session id (and its metrics entry) to the task file BEFORE the child process starts, so the session's own read-modify-write always reads a file that already contains the id.
- A spawn failure inside the liveness window triggers a compensating re-read-modify-write that clears the pre-persisted id and that run's metrics entry while preserving every frontmatter field the child wrote before dying — the task stays retryable.
- If the compensating clear also fails, the spawn error is returned and the clear failure is surfaced as a warning on the result, never masking the original error.
- The interactive (TTY) branch keeps its exact current blocking flow: block through the turn, then persist via the post-return re-read — unchanged behaviour.
- Tests lock the ordering (storage write timestamp precedes spawn timestamp), the early-exit rollback (child writes `phase: planning`, exits non-zero in-window; id and metrics cleared, phase survives), and the clear-failure warning path.
</summary>

<objective>
On the task path only, move the session-id persist ahead of the spawn on the non-interactive branch, add the compensating re-read-clear on spawn failure, surface a failed clear as a warning, and restructure the writeback tests around the runner fake so the persist-before-spawn and rollback behaviours are locked (spec 040 AC4, AC6, and Failure Modes rows 3, 5, 6, 13).
</objective>

<context>
Read `/workspace/CLAUDE.md` and `/workspace/docs/dod.md` first. Then read fully:

- `/workspace/pkg/ops/workon.go` — `workOnOperation.handleClaudeSession` (now has the `isInteractive bool` param and `w.uuidGenerator()` from prompt 1; the fresh path still calls `persistSessionAndMetrics` AFTER `StartSession` returns), `Execute` (call site `w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault, isInteractive)`), and `persistSessionAndMetrics`. Current post-return tail:
  ```go
  sessionID := w.uuidGenerator()
  slog.Info("starting claude session", "task", task.Name)
  if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
      return "", errors.Wrap(ctx, err, "start claude session")
  }
  startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
  return persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
  ```
- `/workspace/pkg/ops/workon_session_writeback_test.go` — restructured in this prompt (see requirement 4). It currently uses `mockStarter *mocks.ClaudeSessionStarter` and its two `StartSessionStub`s do the "headless turn" file write inside the call; the stub arity and pinned `pinnedSessionID` were fixed in prompt 1.
- `/workspace/pkg/ops/workon_test.go` — mock-storage suite; its `when the post-session re-read fails` context and the `re-reads the task from the vault path after the session` test need renaming under the new ordering.
- `/workspace/pkg/domain/task_frontmatter.go` and `/workspace/pkg/domain/task_frontmatter_metrics.go` — verified APIs used below:
  - `func (f *TaskFrontmatter) ClearClaudeSessionID()` — DELETES the key (distinct from `SetClaudeSessionID("")` which keeps an empty value).
  - `func (f TaskFrontmatter) MetricsSessions() []MetricsSession`; `func (f *TaskFrontmatter) AppendMetricsSession(entry MetricsSession)`; `f.Set("metrics_sessions", v)` is promoted from the embedded frontmatter map.
  - `domain.MetricsSession{SessionID string; StartedAt libtime.DateOrDateTime}`.
  - `func (f *TaskFrontmatter) SetPhase(p *TaskPhase)`; `domain.TaskPhasePlanning` / `domain.TaskPhaseExecution` and their `.Ptr()`.
- `/workspace/pkg/ops/claude_session.go` — the new `StartSession(ctx, sessionID, prompt, cwd, name, isInteractive) error` from prompt 1; the non-interactive branch returns `errors.Errorf(ctx, "claude session exited during startup: %v", exitErr)` on an in-window exit.

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`
</context>

<requirements>
1. In `/workspace/pkg/ops/workon.go`, change `handleClaudeSession` to return the session id plus warnings so the compensating-clear failure can surface on `MutationResult.Warnings` (Failure Modes row 5 — never mask the original spawn error). New signature:
   ```go
   func (w *workOnOperation) handleClaudeSession(
       ctx context.Context,
       task *domain.Task,
       vaultPath string,
       sessionDir string,
       vault *config.Vault,
       isInteractive bool,
   ) (string, []string, error)
   ```
   Update the single call site in `Execute`:
   ```go
   sessionID, sessionWarnings, sessionErr := w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault, isInteractive)
   warnings = append(warnings, sessionWarnings...)
   ```
   (`warnings` is the slice declared at the top of `Execute` and already carried into every `MutationResult`; no other change to `Execute` is needed.)

2. Rewrite the fresh-start branch of `handleClaudeSession` so the interactive branch is byte-for-byte behaviourally unchanged and the non-interactive branch persists BEFORE the spawn:
   ```go
   sessionID := w.uuidGenerator()
   slog.Info("starting claude session", "task", task.Name)
   if isInteractive {
       // TTY branch, unchanged: block through the headless turn, then re-read and
       // persist so frontmatter the session itself wrote survives.
       if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
           return "", nil, errors.Wrap(ctx, err, "start claude session")
       }
       startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
       sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
       return sessionID, nil, err
   }
   // Non-interactive branch: persist id + metrics BEFORE the child exists, so the
   // session's own read-modify-write always reads a file that already contains it.
   startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
   if _, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage); err != nil {
       return "", nil, errors.Wrap(ctx, err, "persist claude session before spawn")
   }
   if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
       // Compensating clear: the child may have written frontmatter before dying
       // inside the window (e.g. phase: planning). Re-read and clear only the id and
       // this run's metrics entry, preserving every other field on disk.
       if clearErr := w.clearSessionAndMetrics(ctx, vaultPath, task.Name, sessionID); clearErr != nil {
           return "", []string{fmt.Sprintf("failed to clear claude session id after spawn failure: %v", clearErr)},
               errors.Wrap(ctx, err, "start claude session")
       }
       return "", nil, errors.Wrap(ctx, err, "start claude session")
   }
   return sessionID, nil, nil
   ```
   The cached path (`task.ClaudeSessionID() != ""`) at the top of the function is unchanged except for the new return arity.

3. Add `clearSessionAndMetrics` as a method on `workOnOperation` (the re-read is load-bearing — a stale in-memory clear would revert the child's writes and reproduce the exact "partial state no command intended" this spec eliminates):
   ```go
   // clearSessionAndMetrics re-reads the task after a spawn failure and clears only
   // the claude_session_id and the metrics_sessions entry for this run, preserving
   // any frontmatter the child wrote before dying. The re-read is load-bearing:
   // clearing from the stale in-memory copy would revert the child's writes.
   func (w *workOnOperation) clearSessionAndMetrics(
       ctx context.Context,
       vaultPath string,
       taskName string,
       sessionID string,
   ) error {
       refreshed, err := w.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
       if err != nil {
           return errors.Wrap(ctx, err, "re-read task after spawn failure")
       }
       refreshed.ClearClaudeSessionID()
       var kept []domain.MetricsSession
       for _, m := range refreshed.MetricsSessions() {
           if m.SessionID != sessionID {
               kept = append(kept, m)
           }
       }
       refreshed.Set("metrics_sessions", kept)
       if err := w.taskStorage.WriteTask(ctx, refreshed); err != nil {
           return errors.Wrap(ctx, err, "clear session id after spawn failure")
       }
       return nil
   }
   ```

4. Restructure `/workspace/pkg/ops/workon_session_writeback_test.go` so the "headless turn" file write moves from the starter mock into the runner fake (the turn is no longer bracketed by the call — the parent returns within the liveness window while the child keeps running):
   - Rename the var `mockStarter *mocks.ClaudeSessionStarter` to `starter ops.ClaudeSessionStarter` (the real starter now satisfies the operation's dependency) and drop its `&mocks.ClaudeSessionStarter{}` initializer. The `mocks` import stays (still used for `mocks.DailyNoteStorage`).
   - Add a helper inside the Describe:
     ```go
     newStarter := func(detachRun func(args []string, dir string) (<-chan error, error)) ops.ClaudeSessionStarter {
         return ops.NewClaudeSessionStarterWithRunner(
             "/usr/local/bin/claude",
             nil,
             detachRun,
             libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil }),
         )
     }
     ```
   - Task success case: fixture `phase: planning`, `status: in_progress` at `filepath.Join(vaultPath, "24 Tasks", "Repro Task.md")`; the fake `detachRun` performs the child's write synchronously (find, `SetPhase(domain.TaskPhaseExecution.Ptr())`, `SetField(ctx, "session_note", "written by the headless turn")`, `WriteTask`), then returns `(make(chan error), nil)` — the child is "still running" and never observed again. Build with `newStarter(detachRun)` and keep the full invariant assertion block from prompt 1 (phase execution, session_note, `ClaudeSessionID() == pinnedSessionID`, `MetricsSessions()` has len 1, status in_progress). The pre-spawn persist runs before `StartSession` is called, so the child's write lands on top and nothing later clobbers it.
   - Goal success case: mirror with `GoalPhaseExecution`, `session_note`, `ClaudeSessionID() == pinnedSessionID`, no metrics assertions.
   - The five pinned grep counts (`TaskPhaseExecution` 2, `GoalPhaseExecution` 2, `session_note` 4, `MetricsSessions()` 2, `ClaudeSessionID()` 2) must remain exactly as pinned (AC6).
   - Add an early-exit rollback case (AC6 / Failure Modes row 13). Task fixture WITHOUT a `phase` line (so only the child can put planning on disk in a way a stale clear would revert) or with `phase: execution` (a stale in-memory clear would revert to it):
     ```go
     const rollbackFixture = `---
     phase: execution
     status: in_progress
     ---
     body
     `
     ```
     The fake `detachRun` writes `phase: planning` (`fresh.SetPhase(domain.TaskPhasePlanning.Ptr())` + `WriteTask`), then returns `(done, nil)` where `done` immediately carries `errors.New("exit status 1")` — the child wrote, then exited non-zero inside the window. Drive through `workOnOperation.Execute` (the rollback is caller-side; a direct `StartSession` call would never observe it), and **name the case `It("clears the pre-persisted id and preserves the child's frontmatter write when the child exited non-zero inside the window", …)`** — `<verification>` greps for the literal phrase `exited non-zero inside the window`, so a differently-worded description reports 0 matches on an otherwise-correct implementation. Then assert:
     - `err` is non-nil and its message contains `"start work-on session"` AND `"exit status 1"` (the spawn error is never masked);
     - on disk `written.ClaudeSessionID()` is `""` and `written.MetricsSessions()` is nil/empty (the pre-persisted id and that run's metrics entry are cleared);
     - `written.Phase()` is non-nil and equals `domain.TaskPhasePlanning` — the child's write survived the compensating clear.
     Also assert the on-disk shape: `grep -c '^claude_session_id:' <task-file>` returns 0 and `grep '^phase:' <task-file>` reads `planning`.
   - Add the clear-failure warning case (Failure Modes row 5): `starter` returns a spawn error (use a fake `detachRun` returning an errored `done`), and the task store's `FindTaskByName` returns an error for the clear's re-read; assert the returned error is the spawn error (`"start work-on session"` present) AND `result.Warnings` contains a warning containing `"failed to clear claude session id after spawn failure"`.

5. Update `/workspace/pkg/ops/workon_test.go` for the new ordering and arity:
   - Rename the context `when the post-session re-read fails` to `when the pre-spawn persist re-read fails` and update its comments: under the new ordering the failing `FindTaskByName` call is `persistSessionAndMetrics`'s pre-spawn re-read.
   - In that context, `It("still reports the session id so the session is not orphaned silently", ...)` becomes an assertion that the session id is NOT reported (no spawn ever happened): `Expect(result.SessionID).To(Equal(""))`. Keep the wrapped-error assertion (`"re-read task after claude session"`) and the `WriteTaskCallCount() == 1` / `MetricsSessions() == nil` assertions — they still hold (Execute's write only, since the pre-spawn persist failed before writing).
   - Rename `It("re-reads the task from the vault path after the session", ...)` to `It("re-reads the task from the vault path before spawning the session", ...)` and update the comment (the second `FindTaskByName` is now the pre-spawn persist re-read; its call index 1 and arguments are unchanged).
   - Add an AC4 persist-before-spawn context. It needs a MOCK storage (to record the write timestamp) plus a REAL starter (to record the spawn timestamp), so build a local `ops.ClaudeSessionStarter` and a rebuilt `workOnOp` inside the context's `BeforeEach` (the describe-level `mockStarter` stays a counterfeiter mock and is not reused here). Add `"time"` to the file's imports if not already present:
     ```go
     Context("when persisting the session id before spawn", func() {
         var writeTaskAt, spawnAt time.Time
         BeforeEach(func() {
             mockTaskStorage.WriteTaskStub = func(_ context.Context, t *domain.Task) error {
                 if t.ClaudeSessionID() != "" {
                     writeTaskAt = time.Now()
                 }
                 return nil
             }
             realStarter := ops.NewClaudeSessionStarterWithRunner(
                 "/usr/local/bin/claude",
                 nil,
                 func(_ []string, _ string) (<-chan error, error) {
                     spawnAt = time.Now()
                     return make(chan error), nil
                 },
                 libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil }),
             )
             currentDateTime := libtime.NewCurrentDateTime()
             currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
             workOnOp = ops.NewWorkOnOperation(
                 mockTaskStorage, mockDailyNoteStorage, currentDateTime,
                 func() string { return pinnedSessionID },
                 realStarter, mockResumer,
             )
         })
         It("writes the session id to storage before the runner spawns the child", func() {
             Expect(err).To(BeNil())
             Expect(writeTaskAt).NotTo(BeZero())
             Expect(spawnAt).NotTo(BeZero())
             Expect(writeTaskAt.Before(spawnAt)).To(BeTrue())
             // AC5's "id equals the value in task frontmatter" — capture the id written to
             // storage and the id handed to detachRun and assert they are the same value.
             // Both derive from the pinned generator today, so this holds implicitly; assert
             // it explicitly so a future refactor that mints a second id cannot pass silently.
             Expect(writtenSessionID).To(Equal(spawnedSessionID))
         })
     })
     ```
     (The mocked suite drives `Execute` with `isInteractive=false`, so the non-interactive branch is the one under test.)
   - Add a spawn-failure rollback context (mock starter, mock storage): `mockStarter.StartSessionReturns(ErrTest)`; assert `err` contains `"start work-on session"`, `result.Success` is false, and the LAST `mockTaskStorage.WriteTask` call wrote a task whose `ClaudeSessionID()` is `""` (the compensating clear ran against the mock storage).
   - Add a clear-failure warning context: `mockStarter.StartSessionReturns(ErrTest)` plus `mockTaskStorage.FindTaskByNameReturnsOnCall(2, nil, ErrTest)` (call 0 = Execute load, call 1 = pre-spawn persist re-read, call 2 = the clear's re-read); assert the error still names `"start work-on session"` and `result.Warnings` contains a warning with `"failed to clear claude session id after spawn failure"`.

6. Append a `fix:` bullet under the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (created in prompt 1 — append, do not replace):
    ```
    - fix: on the non-interactive `task work-on` branch the session id and its metrics entry are persisted to the task file before the child is spawned, so the session's own read-modify-write always reads a file that already contains the id; a spawn failure inside the liveness window triggers a re-read-based compensating clear that removes the id and that run's metrics entry while preserving any frontmatter the child wrote before dying, and a failed clear is surfaced as a warning rather than masking the spawn error
    ```

7. Do NOT change: the interactive branch's persist flow (post-return re-read — keep it byte-for-byte), `pkg/ops/goal_workon.go` (prompt 3), `pkg/storage/`, `pkg/domain/`, `pkg/ops/claude_session.go` (beyond what prompt 1 already did), the cached-path behaviour, or the bootstrap prompt string.
</requirements>

<constraints>
- Persist-before-spawn is the guarantee: the write happens while no child exists, so the ordering is not a race. `persistSessionAndMetrics`'s re-read is load-bearing on the interactive branch (the turn mutates the file) and on the cached path; on the non-interactive branch the persist is pre-spawn.
- The compensating clear is a re-read-modify-write: it must re-read the task from disk and clear only the id and this run's metrics entry, preserving any frontmatter the child wrote before dying. Clearing from the stale in-memory copy would reproduce the exact "partial state no command intended" this spec eliminates.
- Rollback is caller-side (`handleClaudeSession` / `Execute`), never inside `StartSession` — a test calling `StartSession` directly would never observe it.
- A failed clear surfaces as a warning on `MutationResult.Warnings`; the original spawn error is always returned, never masked.
- The interactive branch's flow is byte-for-byte unchanged (same blocking wait, same 5m cap, same post-return persist).
- `pkg/ops/` stays a pure library — no stdout. Errors use `github.com/bborbe/errors` with the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Tests use Ginkgo v2 / Gomega with Counterfeiter mocks where mocking is the point; the writeback file deliberately uses real `storage.NewTaskStorage`/`NewGoalStorage` against an `os.MkdirTemp` vault (the session write must land on disk for the invariant to be meaningful).
- New code ≥80% statement coverage; test error paths (spawn failure, clear failure, re-read failure).
- Do NOT commit — dark-factory handles git. `.git` is visible in this container (`workflow: direct`, no `hideGit`), so git-based checks work; the prohibition is a scope rule.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

Persist-before-spawn is in place (AC4 test present and passing):

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'writes the session id to storage before the runner spawns the child'
```

Must print a number >= 1.

The writeback invariant holds under the bounded diff (AC6):

```
grep -c 'TaskPhaseExecution' pkg/ops/workon_session_writeback_test.go   # must be 2
grep -c 'GoalPhaseExecution' pkg/ops/workon_session_writeback_test.go   # must be 2
grep -c 'session_note' pkg/ops/workon_session_writeback_test.go         # must be 4
grep -c 'MetricsSessions()' pkg/ops/workon_session_writeback_test.go    # must be 2
grep -c 'ClaudeSessionID()' pkg/ops/workon_session_writeback_test.go    # must be 2
go test ./pkg/ops/ -run Writeback    # must exit 0 (per spec AC6 letter)
```

The rollback path is locked (AC6 / Failure Modes row 13):

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'exited non-zero inside the window'   # >= 1
grep -c 'failed to clear claude session id after spawn failure' pkg/ops/workon.go                # must be 1
```

The on-disk rollback shape is asserted (the child's `phase: planning` write survives, the id is gone):

```
go test -count=1 ./pkg/ops/ -run Writeback   # the rollback test asserts the on-disk shape itself
```

The pre-spawn re-read rename is in place:

```
grep -c 'when the pre-spawn persist re-read fails' pkg/ops/workon_test.go     # must be 1
grep -c 're-reads the task from the vault path before spawning the session' pkg/ops/workon_test.go  # must be 1
```

Changelog has exactly the expected bullets under `## Unreleased`:

```
grep -c 'persisted to the task file before the child is spawned' CHANGELOG.md   # must be 1
```

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
