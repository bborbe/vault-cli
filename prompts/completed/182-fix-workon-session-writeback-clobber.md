---
status: completed
summary: Fixed session write-back race in task/goal work-on by re-reading entity from disk after blocking StartSession call
execution_id: vault-cli-exec-182-fix-workon-session-writeback-clobber
dark-factory-version: dev
created: "2026-08-16T10:00:00Z"
queued: "2026-08-16T09:45:21Z"
started: "2026-08-16T09:45:58Z"
completed: "2026-08-16T09:53:27Z"
---

<summary>
- `vault-cli task work-on` stops throwing away the work its own Claude session just did.
- Today the headless startup turn runs for minutes, updates the task file (for example moving it into the execution phase), and then work-on overwrites that file with the copy it had loaded before the turn started — reverting everything except the session id it just added.
- After this change work-on re-reads the task from disk once the session returns, so whatever the session wrote survives and the session id is added on top.
- `goal work-on` carries the identical defect and gets the identical fix.
- A stale code comment claiming the headless turn "prints the next-step signal and STOPs" is corrected — since v0.109.0 that turn auto-chains planning into execution.
- A regression test reproduces the real race: a fake Claude session writes to the actual task file on disk mid-call, and the test then proves both the session's write and the session id are present afterwards.
- No new flag, config key, or behavior toggle; the auto-chaining behavior itself is untouched.
</summary>

<objective>
`task work-on` and `goal work-on` must stop clobbering frontmatter that the headless Claude session writes while `StartSession` is blocking. Both operations currently persist a stale in-memory copy of the entity after the session returns; make them re-read the entity from disk and apply `claude_session_id` to that fresh copy instead. Also correct the now-false comment in `Execute` describing the turn-1 contract.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/storage layering rules.

Read these files fully before making changes:

- `/workspace/pkg/ops/workon.go` — primary file. Two verified, load-bearing places:

  The bug, in `handleClaudeSession`:
  ```go
  sessionID, err := w.starter.StartSession(ctx, prompt, vaultPath, task.Name)
  if err != nil {
      return "", errors.Wrap(ctx, err, "start claude session")
  }
  task.SetClaudeSessionID(sessionID)
  if err := w.taskStorage.WriteTask(ctx, task); err != nil {
      return sessionID, errors.Wrap(ctx, err, "save session id to task")
  }
  return sessionID, nil
  ```
  `StartSession` blocks for the entire headless `claude --print` turn (measured ~2m49s live on 2026-08-16). During that turn the spawned session mutates the task file on disk — since v0.109.0 the `--non-interactive` work-on chain runs `plan-task` then `execute-task`, and `execute-task` writes `phase: execution`. `task` here is the value loaded by `Execute` **before** the session ran, so `WriteTask` reverts `phase` and discards every other frontmatter change the turn made. The only surviving change is `claude_session_id`, which this stale write itself adds. Live evidence: the transcript showed `vault-cli task set "Start Day - 2026-08-16" phase execution` → `✅ Set phase=execution`, an independent watcher observed `phase: execution` on disk, and the final file read `phase: planning` carrying the new `claude_session_id`.

  **Verified parameter-naming trap:** `handleClaudeSession`'s third parameter is *declared* `vaultPath string` but `Execute` calls it as `w.handleClaudeSession(ctx, task, sessionDir, vault)`. The value inside is the session working directory, not the vault path — it is passed straight through as `StartSession`'s `cwd` argument. `handleClaudeSession` therefore has no access to the real vault path today; the fix must plumb it in.

  The stale comment, in `Execute`'s interactive branch:
  ```go
  if isInteractive && w.resumer != nil && sessionID != "" {
      // Turn 1 ran headless with --non-interactive, which told the work-on
      // command to print the next-step signal and STOP. Turn 2 is interactive,
      // so re-invoke the same command WITHOUT the flag — otherwise the operator
      // lands on the tail of a turn that was instructed to stop.
      continuation := fmt.Sprintf(`%s "%s"`, vault.GetWorkOnCommand(), task.FilePath)
  ```

- `/workspace/pkg/ops/goal_workon.go` — the goal-side `handleClaudeSession` has the identical defect with `goal.SetClaudeSessionID(sessionID)` + `g.goalStorage.WriteGoal(ctx, goal)`, and the identical `vaultPath`-is-really-`sessionDir` parameter trap (`Execute` calls `g.handleClaudeSession(ctx, goal, sessionDir, vault)`).

- `/workspace/pkg/storage/storage.go` — verified narrow interfaces; do NOT change them. Both already expose everything the fix needs:
  ```go
  type TaskStorage interface {
      WriteTask(ctx context.Context, task *domain.Task) error
      FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)
      ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)
  }

  type GoalStorage interface {
      WriteGoal(ctx context.Context, goal *domain.Goal) error
      FindGoalByName(ctx context.Context, vaultPath string, name string) (*domain.Goal, error)
  }
  ```
  Also verified constructors used by the new test: `func NewTaskStorage(storageConfig *Config) TaskStorage` and `func NewGoalStorage(storageConfig *Config) GoalStorage`, plus `type Config struct { TasksDir, GoalsDir, ThemesDir, ObjectivesDir, VisionDir, DailyDir string; Excludes []string }`.

- `/workspace/pkg/ops/claude_session.go` — verified interface, do NOT change:
  ```go
  type ClaudeSessionStarter interface {
      StartSession(ctx context.Context, prompt string, cwd string, name string) (string, error)
  }
  ```

- `/workspace/pkg/ops/wikilink_roundtrip_test.go` — **the model for the new regression test.** It is the existing precedent in `pkg/ops` for driving a real `storage.NewTaskStorage(cfg)` / `storage.NewGoalStorage(cfg)` against an `os.MkdirTemp` vault with `os.MkdirAll` for the entity subdirectories and `os.RemoveAll` in `AfterEach`. Copy its shape.

- `/workspace/pkg/ops/workon_test.go` — the mocked suite you must repair. Verified fixture values in `BeforeEach`: `mockStarter.StartSessionReturns("session-123", nil)`, `mockTaskStorage.FindTaskByNameReturns(task, nil)`, `testVault = config.Vault{Path: vaultPath, Name: "test-vault", WorkOnCommand: "/vault-cli:work-on-task"}`, task frontmatter `map[string]any{"status": "todo"}`, `FilePath: "/path/to/vault/tasks/my-task.md"`. The `JustBeforeEach` passes `vaultPath` for BOTH the `vaultPath` and `sessionDir` arguments.

- `/workspace/pkg/ops/goal_workon_test.go` — same shape for goals; `testVault` sets `WorkOnGoalCommand: "/vault-cli:work-on-goal"`, goal frontmatter `map[string]any{"status": "next"}`.

- `/workspace/pkg/ops/ops_suite_test.go` — defines the shared `var ErrTest`.

- Verified domain accessors used below (do NOT invent others):
  - `func (f TaskFrontmatter) Phase() *TaskPhase` / `func (f *TaskFrontmatter) SetPhase(p *TaskPhase)`
  - `func (f GoalFrontmatter) Phase() *GoalPhase` / `func (f *GoalFrontmatter) SetPhase(p *GoalPhase)`
  - `domain.TaskPhaseExecution TaskPhase = "execution"`, `func (t TaskPhase) Ptr() *TaskPhase`
  - `domain.GoalPhaseExecution GoalPhase = "execution"`, `func (g GoalPhase) Ptr() *GoalPhase`
  - `func (f TaskFrontmatter) ClaudeSessionID() string` / `func (f *TaskFrontmatter) SetClaudeSessionID(v string)` (same pair on `GoalFrontmatter`)
  - `func (f TaskFrontmatter) GetField(key string) string` / `func (f *TaskFrontmatter) SetField(ctx context.Context, key, value string) error` (same pair on `GoalFrontmatter`; unknown keys fall through to the raw map unvalidated)
  - `domain.TaskStatusInProgress`, `domain.GoalStatusInProgress`

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega with Counterfeiter fakes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` context wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>
1. In `/workspace/pkg/ops/workon.go`, change `handleClaudeSession` to take the real vault path in addition to the session directory, and rename the misleading existing parameter. New signature:

   ```go
   func (w *workOnOperation) handleClaudeSession(
       ctx context.Context,
       task *domain.Task,
       vaultPath string,
       sessionDir string,
       vault *config.Vault,
   ) (string, error) {
   ```

   Inside the body, the `StartSession` call must pass `sessionDir` as the `cwd` argument (this is a pure rename of the value that is already passed there — no behavior change):

   ```go
   sessionID, err := w.starter.StartSession(ctx, prompt, sessionDir, task.Name)
   ```

2. In the same function, replace the stale write-back with a re-read. The post-`StartSession` tail must read exactly:

   ```go
   sessionID, err := w.starter.StartSession(ctx, prompt, sessionDir, task.Name)
   if err != nil {
       return "", errors.Wrap(ctx, err, "start claude session")
   }
   // StartSession blocks for the whole headless turn, and that turn writes to this
   // very task file (since v0.109.0 the --non-interactive chain runs plan-task then
   // execute-task, which sets phase: execution). Writing back the copy loaded before
   // the session would revert every field the session changed, so re-read from disk
   // and apply only the session id on top.
   refreshed, err := w.taskStorage.FindTaskByName(ctx, vaultPath, task.Name)
   if err != nil {
       return sessionID, errors.Wrap(ctx, err, "re-read task after claude session")
   }
   refreshed.SetClaudeSessionID(sessionID)
   if err := w.taskStorage.WriteTask(ctx, refreshed); err != nil {
       return sessionID, errors.Wrap(ctx, err, "save session id to task")
   }
   return sessionID, nil
   ```

   Do NOT fall back to writing the stale `task` when the re-read fails — that reintroduces the clobber. A failed re-read is a hard error, exactly like the existing failed write is today: `Execute` already turns any non-`ErrStarterUnavailable` session error into `MutationResult{Success: false}` plus a wrapped `"start work-on session"` error.

   `task` itself stays in scope in `Execute` unchanged — its `Name` and `FilePath` are filesystem metadata that the session does not alter, so the continuation prompt and `MutationResult` keep using it.

3. Update the doc comment above `handleClaudeSession` in `/workspace/pkg/ops/workon.go` to say what it now does:

   ```go
   // handleClaudeSession starts or returns an existing Claude session for the task.
   // On a fresh start it re-reads the task from disk after the session returns, so
   // frontmatter the session itself wrote during the blocking turn survives.
   ```

4. Update the single call site in `/workspace/pkg/ops/workon.go` `Execute`:

   ```go
   sessionID, sessionErr := w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault)
   ```

   `vaultPath` is already a parameter of `Execute`; no other plumbing is needed.

5. Replace the stale comment in `Execute`'s interactive branch in `/workspace/pkg/ops/workon.go`. It currently claims turn 1 "print[s] the next-step signal and STOP[s]", which has been untrue since v0.109.0. Write it as:

   ```go
   if isInteractive && w.resumer != nil && sessionID != "" {
       // Turn 1 ran headless with --non-interactive. Since v0.109.0 that turn
       // auto-chains plan-task -> execute-task under a NO-ASK contract, so it can
       // end anywhere from phase: planning (a gate needed an answer it could not
       // ask for) to phase: execution. Turn 2 is interactive, so re-invoke the same
       // command WITHOUT the flag: it resumes the chain from whatever phase turn 1
       // left on disk and can ask the questions turn 1 had to skip.
       continuation := fmt.Sprintf(`%s "%s"`, vault.GetWorkOnCommand(), task.FilePath)
   ```

   Everything else in that branch stays byte-identical.

6. Apply the same three changes to `/workspace/pkg/ops/goal_workon.go`:

   - New signature `func (g *goalWorkOnOperation) handleClaudeSession(ctx context.Context, goal *domain.Goal, vaultPath string, sessionDir string, vault *config.Vault) (string, error)`.
   - `StartSession` receives `sessionDir` as `cwd`.
   - Post-session tail:
     ```go
     refreshed, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goal.Name)
     if err != nil {
         return sessionID, errors.Wrap(ctx, err, "re-read goal after claude session")
     }
     refreshed.SetClaudeSessionID(sessionID)
     if err := g.goalStorage.WriteGoal(ctx, refreshed); err != nil {
         return sessionID, errors.Wrap(ctx, err, "save session id to goal")
     }
     return sessionID, nil
     ```
     with the same explanatory comment above the re-read, and the same updated doc comment on the function.
   - Call site in `Execute`: `sessionID, sessionErr := g.handleClaudeSession(ctx, goal, vaultPath, sessionDir, vault)`.
   - Do NOT touch the goal-side interactive branch's `ResumeSession(ctx, sessionID, sessionDir, "")` call or its comment about spec 029 — the goal continuation prompt is out of scope.

7. Create `/workspace/pkg/ops/workon_session_writeback_test.go` — the regression test that actually reproduces the race. It must drive the **real** storage implementation against a temp vault so the fake session's write really lands on disk. A test that only asserts `result.SessionID == "session-123"` against mocked storage does not cover this bug and is not acceptable.

   Required shape (package `ops_test`, Ginkgo v2 / Gomega, following `/workspace/pkg/ops/wikilink_roundtrip_test.go`):

   ```go
   var _ = Describe("work-on session write-back", func() {
       var (
           ctx           context.Context
           vaultPath     string
           sessionDir    string
           storageConfig *storage.Config
           mockStarter   *mocks.ClaudeSessionStarter
           mockDailyNote *mocks.DailyNoteStorage
       )

       BeforeEach(func() {
           ctx = context.Background()

           var err error
           vaultPath, err = os.MkdirTemp("", "vault-workon-writeback-*")
           Expect(err).To(BeNil())
           // Deliberately NOT the vault path: the re-read must use vaultPath, not the
           // session working directory. If the fix plumbs the wrong value through,
           // FindTaskByName looks in an empty directory and this test fails.
           sessionDir, err = os.MkdirTemp("", "vault-workon-cwd-*")
           Expect(err).To(BeNil())

           storageConfig = &storage.Config{TasksDir: "24 Tasks", GoalsDir: "23 Goals"}
           for _, dir := range []string{"24 Tasks", "23 Goals"} {
               Expect(os.MkdirAll(filepath.Join(vaultPath, dir), 0755)).To(Succeed())
           }

           mockStarter = &mocks.ClaudeSessionStarter{}
           mockDailyNote = &mocks.DailyNoteStorage{}
           mockDailyNote.ReadDailyNoteReturns("", nil)
       })

       AfterEach(func() {
           if vaultPath != "" {
               _ = os.RemoveAll(vaultPath)
           }
           if sessionDir != "" {
               _ = os.RemoveAll(sessionDir)
           }
       })
       ...
   })
   ```

8. In that file, add the task-side case. The `StartSessionStub` is what makes this a real reproduction — it must mutate the task file on disk *inside* the blocking call, exactly as the real headless turn does:

   ```go
   Context("task work-on", func() {
       const taskFixture = `---
   assignee: user@example.com
   phase: planning
   status: in_progress
   ---
   body
   `
       var taskStore storage.TaskStorage

       BeforeEach(func() {
           taskStore = storage.NewTaskStorage(storageConfig)
           Expect(os.WriteFile(
               filepath.Join(vaultPath, "24 Tasks", "Repro Task.md"),
               []byte(taskFixture), 0600,
           )).To(Succeed())

           // Simulate the real headless turn: while StartSession blocks, the spawned
           // Claude session runs plan-task -> execute-task and writes to the very file
           // work-on loaded before the call.
           mockStarter.StartSessionStub = func(
               ctx context.Context, _ string, _ string, _ string,
           ) (string, error) {
               fresh, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
               if err != nil {
                   return "", err
               }
               fresh.SetPhase(domain.TaskPhaseExecution.Ptr())
               if err := fresh.SetField(ctx, "session_note", "written by the headless turn"); err != nil {
                   return "", err
               }
               if err := taskStore.WriteTask(ctx, fresh); err != nil {
                   return "", err
               }
               return "session-123", nil
           }
       })

       It("keeps the frontmatter the session wrote and adds claude_session_id", func() {
           currentDateTime := libtime.NewCurrentDateTime()
           currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
           testVault := config.Vault{
               Path:          vaultPath,
               Name:          "test-vault",
               WorkOnCommand: "/vault-cli:work-on-task",
           }
           workOnOp := ops.NewWorkOnOperation(
               taskStore, mockDailyNote, currentDateTime, mockStarter, nil,
           )

           result, err := workOnOp.Execute(
               ctx, vaultPath, "Repro Task", "user@example.com", "test-vault",
               false, sessionDir, &testVault,
           )
           Expect(err).To(BeNil())
           Expect(result.Success).To(BeTrue())
           Expect(result.SessionID).To(Equal("session-123"))

           written, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
           Expect(err).To(BeNil())
           // The bug: this was reverted to "planning" by the stale write-back.
           Expect(written.Phase()).NotTo(BeNil())
           Expect(*written.Phase()).To(Equal(domain.TaskPhaseExecution))
           // An unrelated field the session touched must survive too.
           Expect(written.GetField("session_note")).To(Equal("written by the headless turn"))
           // ...and the session id must still be persisted.
           Expect(written.ClaudeSessionID()).To(Equal("session-123"))
           Expect(written.Status()).To(Equal(domain.TaskStatusInProgress))
       })
   })
   ```

   The fixture sets `phase: planning` on purpose: `advancePhaseIfEntering` leaves a non-nil, non-`todo` phase alone, so the only thing that can move it to `execution` is the stub, and the only thing that can move it back is the bug.

9. In the same file, add the goal-side case with the same structure: fixture written at `filepath.Join(vaultPath, "23 Goals", "Repro Goal.md")` — i.e. inside the `os.MkdirTemp` vault, NOT under `/workspace` — with frontmatter `phase: planning` and `status: in_progress`; `goalStore := storage.NewGoalStorage(storageConfig)`; the stub calls `goalStore.FindGoalByName(ctx, vaultPath, "Repro Goal")`, `fresh.SetPhase(domain.GoalPhaseExecution.Ptr())`, `fresh.SetField(ctx, "session_note", "written by the headless turn")`, `goalStore.WriteGoal(ctx, fresh)`, returns `"session-123"`; the operation is `ops.NewGoalWorkOnOperation(goalStore, mockStarter, nil)` invoked as `Execute(ctx, vaultPath, "Repro Goal", "user@example.com", "test-vault", false, sessionDir, &testVault)` with `testVault = config.Vault{Path: vaultPath, Name: "test-vault", WorkOnGoalCommand: "/vault-cli:work-on-goal"}`; assertions mirror the task case using `domain.GoalPhaseExecution` and `domain.GoalStatusInProgress`.

10. Repair the two existing mocked assertions that the extra read breaks. In `/workspace/pkg/ops/workon_test.go`, `Context("success")`, replace `It("calls FindTaskByName", ...)` with:

    ```go
    It("calls FindTaskByName", func() {
        // Twice: once to load the task, once to re-read it after the blocking
        // session returns so the session's own frontmatter writes survive.
        Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(2))
        actualCtx, actualVaultPath, actualTaskName := mockTaskStorage.FindTaskByNameArgsForCall(0)
        Expect(actualCtx).To(Equal(ctx))
        Expect(actualVaultPath).To(Equal(vaultPath))
        Expect(actualTaskName).To(Equal(taskName))
    })

    It("re-reads the task from the vault path after the session", func() {
        Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(2))
        _, reReadVaultPath, reReadTaskName := mockTaskStorage.FindTaskByNameArgsForCall(1)
        Expect(reReadVaultPath).To(Equal(vaultPath))
        Expect(reReadTaskName).To(Equal(taskName))
    })
    ```

    In `/workspace/pkg/ops/goal_workon_test.go`, `Context("success")`, apply the mirror change to `It("calls FindGoalByName", ...)` using `mockGoalStorage.FindGoalByNameCallCount()` / `FindGoalByNameArgsForCall`, `goalName`, and a second `It("re-reads the goal from the vault path after the session", ...)`.

    These are the only two call-count assertions in the repo affected by the extra read — verified by grepping `FindTaskByNameCallCount` / `FindGoalByNameCallCount` across `pkg/` and `integration/`. The contexts where the starter is nil, the entity already has a cached session id, or the lookup/first write fails all still make exactly one lookup call; leave them alone.

11. Add a regression case to `/workspace/pkg/ops/workon_test.go` proving a failed re-read does not silently persist the stale copy. Add a new top-level `Context` inside the existing `Describe("WorkOnOperation")`:

    ```go
    Context("when the post-session re-read fails", func() {
        BeforeEach(func() {
            mockTaskStorage.FindTaskByNameReturnsOnCall(0, task, nil)
            mockTaskStorage.FindTaskByNameReturnsOnCall(1, nil, ErrTest)
        })

        It("returns a wrapped error and Success=false", func() {
            Expect(err).To(HaveOccurred())
            Expect(err.Error()).To(ContainSubstring("re-read task after claude session"))
            Expect(result.Success).To(BeFalse())
        })

        It("still reports the session id so the session is not orphaned silently", func() {
            Expect(result.SessionID).To(Equal("session-123"))
        })

        It("does not write a second time with the stale in-memory task", func() {
            Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
        })
    })
    ```

12. Add a `## Unreleased` section to `/workspace/CHANGELOG.md` with one `fix:` bullet. The file currently has **no** `## Unreleased` section — create it **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` lines) and **above** the existing `## v0.109.1` heading. Final order: `# Changelog` → preamble → `## Unreleased` → `## v0.109.1` → … Bullet text:

    ```
    - fix: `task work-on` and `goal work-on` no longer revert frontmatter written by their own headless bootstrap session. `StartSession` blocks for the entire `claude --print` turn (~3 min measured), and since v0.109.0 that turn auto-chains `plan-task` → `execute-task` and writes `phase: execution` to the entity file. Both operations then wrote back the in-memory copy loaded *before* the session ran, silently reverting `phase` and any other field the turn changed while adding only `claude_session_id`. Both now re-read the entity from disk once the session returns and apply `claude_session_id` to that fresh copy. Also corrects the stale comment in `pkg/ops/workon.go` claiming the non-interactive turn prints the next-step signal and stops
    ```

13. Do NOT change the auto-chain behavior itself. `/workspace/commands/work-on-task.md`, `/workspace/commands/work-on-goal.md`, `/workspace/commands/plan-task.md`, `/workspace/commands/execute-task.md`, `/workspace/pkg/ops/claude_session.go`, `/workspace/pkg/ops/claude_resume.go`, `/workspace/pkg/storage/`, and `/workspace/mocks/` are all out of scope. The `--non-interactive` bootstrap prompt string in both `handleClaudeSession` functions keeps its trailing ` --non-interactive` and its existing comment verbatim.

14. Do NOT add a config field, flag, or environment variable to opt out of the re-read. An escape hatch on the very behavior this fix ships would reintroduce the bug.
</requirements>

<constraints>
- Scope is strictly the post-session write-back race plus the stale comment. The v0.109.0 auto-chain behavior is correct and verified — do not change it.
- Do NOT change any interface in `/workspace/pkg/storage/storage.go`. Both `FindTaskByName` and `FindGoalByName` already exist on the narrow interfaces; no new storage method and no new persistence layer.
- Do NOT regenerate or hand-edit `/workspace/mocks/` — no interface changed, so `go generate` must produce no diff there.
- `pkg/ops/` is a library layer — no `fmt.Print*`, no `os.Stdout` writes (see `/workspace/CLAUDE.md` § Key Design Decisions). `fmt.Sprintf` and `slog` are fine.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Exported types and functions keep doc comments; unexported ones that already have them keep them updated (see `/workspace/docs/dod.md`).
- This repo is `autoRelease: true` (`.maintainer.yaml`). Add the `## Unreleased` bullet only. Do NOT rename `## Unreleased` to a version, do NOT bump `.claude-plugin/plugin.json` or either version field in `.claude-plugin/marketplace.json`, and do NOT `git tag` — the `github-releaser-agent` owns all of that post-merge.
- Do NOT commit — dark-factory handles git.
- Do not use git in this rung. Commits, branches, and tags are the operator's and the releaser's job, not this prompt's. (Note: `.git` *is* visible in this container — `.dark-factory.yaml` sets `workflow: direct` with no `hideGit`, so `EffectiveHideGit()` is false. The prohibition is a scope rule, not a technical impossibility; do not treat a working `git` command as permission to use it.)
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

The stale write-back is gone from both files:

```
grep -c 'task.SetClaudeSessionID(sessionID)' pkg/ops/workon.go
grep -c 'goal.SetClaudeSessionID(sessionID)' pkg/ops/goal_workon.go
```

Each must print exactly `0`.

The re-read is in place in both files:

```
grep -c 'refreshed, err := w.taskStorage.FindTaskByName(ctx, vaultPath, task.Name)' pkg/ops/workon.go
grep -c 'refreshed.SetClaudeSessionID(sessionID)' pkg/ops/workon.go
grep -c 'refreshed, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goal.Name)' pkg/ops/goal_workon.go
grep -c 'refreshed.SetClaudeSessionID(sessionID)' pkg/ops/goal_workon.go
```

Each must print exactly `1`.

The vault path is plumbed to both call sites:

```
grep -c 'w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault)' pkg/ops/workon.go
grep -c 'g.handleClaudeSession(ctx, goal, vaultPath, sessionDir, vault)' pkg/ops/goal_workon.go
```

Each must print exactly `1`.

The stale comment is gone and the turn-1 bootstrap is untouched:

```
grep -c 'print the next-step signal and STOP' pkg/ops/workon.go
grep -c -- '--non-interactive`,' pkg/ops/workon.go
grep -c -- '--non-interactive`,' pkg/ops/goal_workon.go
```

The first must print `0`; the second and third must each print `1`.

The trailing `` `, `` in the pattern is load-bearing: it matches only the bootstrap prompt line inside `handleClaudeSession`. A bare `--non-interactive` pattern also matches the surrounding comments (3 occurrences in `workon.go`, 2 in `goal_workon.go`), so a correct implementation would fail the gate.

Out-of-scope files are byte-identical. `make precommit` runs `go generate`, so run this check **after** `make precommit`:

```
md5sum pkg/ops/claude_session.go pkg/ops/claude_resume.go mocks/task-storage.go mocks/goal-storage.go mocks/claude-session-starter.go
```

The five hashes must be **exactly**:

```
6fd7090f033d6c3156d74dd2cde041f0  pkg/ops/claude_session.go
3c1b6339d2473fd8c84973d690c0c780  pkg/ops/claude_resume.go
dccc5324c2ea66f3f4205eabd88421b0  mocks/task-storage.go
8c90e70ca20e47d09717b3ee0f3b9fdb  mocks/goal-storage.go
15f0a2a24711abf83e8c9364aef66dea  mocks/claude-session-starter.go
```

Nothing under `commands/` or `pkg/storage/` may be modified either.

The regression test exists and passes:

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "keeps the frontmatter the session wrote and adds claude_session_id"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "re-reads the task from the vault path after the session"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "re-reads the goal from the vault path after the session"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "does not write a second time with the stale in-memory task"
```

Each must print a number `>= 1`.

The regression test really reproduces the bug — it must FAIL against the old code, not merely pass against the new. Verify by hand:

1. Temporarily revert `pkg/ops/workon.go`'s re-read to `task.SetClaudeSessionID(sessionID)` + `w.taskStorage.WriteTask(ctx, task)`.
2. Run `go test -count=1 ./pkg/ops/ 2>&1 | tail -40`.
3. Confirm the failure names `TaskPhaseExecution` vs `planning` (i.e. the assertion `Expect(*written.Phase()).To(Equal(domain.TaskPhaseExecution))` fails).
4. Restore the fixed code.
5. **Restore gate — run this before anything else:**

```
grep -c 'refreshed, err := w.taskStorage.FindTaskByName(ctx, vaultPath, task.Name)' pkg/ops/workon.go
```

   It must print `1`. If it prints `0`, step 4 did not happen — restore the fixed code and re-check. Do **not** run `make precommit` or continue until this prints `1`.

6. Re-run `go test -count=1 ./pkg/ops/` and confirm green.

Do **not** pass `-run` here. The suite entrypoint is `func TestSuite(t *testing.T)` in `pkg/ops/ops_suite_test.go`; a wrong `-run` value (e.g. `-run TestOps`) matches nothing, prints `ok ... [no tests to run]`, and **exits 0** — which would make step 2 look like a pass and trigger the rewrite instruction below against a correct test.

If step 2 genuinely passes with the reverted code — i.e. tests actually ran and reported success — the test does not cover the bug; rewrite it before continuing. Confirm tests ran by checking the output reports a non-zero spec count, not `[no tests to run]`.

Exactly one changelog bullet for this fix, under `## Unreleased`:

```
grep -c 'no longer revert frontmatter written by their own headless bootstrap session' CHANGELOG.md
grep -c '^## Unreleased$' CHANGELOG.md
```

Each must print exactly `1`.

Version strings must NOT have moved:

```
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

The first must still print `## v0.109.1`; the second must still show `0.109.1`.

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
