---
status: completed
spec: [040-bug-session-start-blocks-on-full-headless-turn]
summary: Goal work-on now persists the session id before the child spawn on the non-interactive branch and compensates with a re-read-based id-only clear on spawn failure, mirroring the task path, with AC8 tests and changelog entry
execution_id: vault-cli-session-fast-return-exec-200-spec-040-goal-persist-ordering
dark-factory-version: dev
created: "2026-08-27T12:00:00Z"
queued: "2026-08-27T10:51:51Z"
started: "2026-08-27T11:54:40Z"
completed: "2026-08-27T12:01:31Z"
branch: dark-factory/bug-session-start-blocks-on-full-headless-turn
---

<summary>
- The goal work-on fresh path receives the same persist-before-spawn treatment as the task path: the goal id is persisted before the child is spawned on the non-interactive branch, so the session's own read-modify-write always reads a file that already contains it.
- A spawn failure inside the liveness window triggers a re-read-based compensating clear of the goal's session id that preserves any frontmatter the child wrote before dying. The goal path has no `metrics_sessions`, so there is no metrics rollback.
- The interactive (TTY) goal branch keeps its exact current blocking flow (block through the turn, then post-return persist) — unchanged behaviour.
- A failed clear surfaces as a warning on the result, never masking the original spawn error.
- Tests lock the ordering (goal write timestamp precedes spawn timestamp), the captured argv (`--session-id`, `--print`, `-n <goal name>`), and the early-exit rollback (id cleared, child's frontmatter write preserved).
</summary>

<objective>
On the goal path, mirror the task-path persist-before-spawn and compensating-clear work from prompt 2: move `persistGoalSessionID` ahead of the spawn on the non-interactive branch, add the re-read-based clear of the goal session id (id only — goals carry no metrics), surface a failed clear as a warning, and lock it with tests in `goal_workon_test.go` (spec 040 AC8, AC4-goal, and Failure Modes row 13 applied to goals).
</objective>

<context>
Read `/workspace/CLAUDE.md` and `/workspace/docs/dod.md` first. Then read fully:

- `/workspace/pkg/ops/goal_workon.go` — `goalWorkOnOperation.handleClaudeSession` (now has the `isInteractive bool` param and `g.uuidGenerator()` from prompt 1; the fresh path still calls `persistGoalSessionID` AFTER `StartSession` returns), `Execute` (call site `g.handleClaudeSession(ctx, goal, vaultPath, sessionDir, vault, isInteractive)`), and `persistGoalSessionID`. Current post-return tail:
  ```go
  sessionID := g.uuidGenerator()
  slog.Info("starting claude session", "goal", goal.Name)
  if err := g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive); err != nil {
      return "", errors.Wrap(ctx, err, "start claude session")
  }
  return persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage)
  ```
- `/workspace/pkg/ops/workon.go` — the already-shipped task-path pattern from prompt 2 (the `isInteractive` branch, `clearSessionAndMetrics`, the `[]string` warnings return). Mirror its shape exactly; do NOT extract a shared helper (the codebase duplicates these deliberately — see spec Open Question 2).
- `/workspace/pkg/domain/goal_frontmatter.go` — verified: `func (f *GoalFrontmatter) ClearClaudeSessionID()` DELETES the key; `func (f GoalFrontmatter) ClaudeSessionID() string`; `func (f *GoalFrontmatter) SetClaudeSessionID(v string)`; `func (f *GoalFrontmatter) SetPhase(p *GoalPhase)`; `domain.GoalPhaseExecution`, `domain.GoalPhasePlanning` and their `.Ptr()`.
- `/workspace/pkg/ops/goal_workon_test.go` — mock-storage suite. Note `FindGoalByNameCallCount()` is 2 in the success context (Execute load + the persist re-read — unchanged count under the new ordering), and `re-reads the goal from the vault path after the session` (line 100) needs renaming. Add `"time"` and `libtime "github.com/bborbe/time"` to its imports if not already present.

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`
</context>

<requirements>
1. In `/workspace/pkg/ops/goal_workon.go`, change `handleClaudeSession` to return the session id plus warnings (mirroring the task path):
   ```go
   func (g *goalWorkOnOperation) handleClaudeSession(
       ctx context.Context,
       goal *domain.Goal,
       vaultPath string,
       sessionDir string,
       vault *config.Vault,
       isInteractive bool,
   ) (string, []string, error)
   ```
   Update the single call site in `Execute`:
   ```go
   sessionID, sessionWarnings, sessionErr := g.handleClaudeSession(ctx, goal, vaultPath, sessionDir, vault, isInteractive)
   warnings = append(warnings, sessionWarnings...)
   ```

2. Rewrite the fresh-start branch so the interactive branch is byte-for-byte behaviourally unchanged and the non-interactive branch persists BEFORE the spawn:
   ```go
   sessionID := g.uuidGenerator()
   slog.Info("starting claude session", "goal", goal.Name)
   if isInteractive {
       // TTY branch, unchanged: block through the headless turn, then re-read and
       // persist so frontmatter the session itself wrote survives.
       if err := g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive); err != nil {
           return "", nil, errors.Wrap(ctx, err, "start claude session")
       }
       sessionID, err := persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage)
       return sessionID, nil, err
   }
   // Non-interactive branch: persist the id BEFORE the child exists, so the session's
   // own read-modify-write always reads a file that already contains it.
   if _, err := persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage); err != nil {
       return "", nil, errors.Wrap(ctx, err, "persist claude session id before spawn")
   }
   if err := g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive); err != nil {
       if clearErr := g.clearGoalSession(ctx, vaultPath, goal.Name); clearErr != nil {
           return "", []string{fmt.Sprintf("failed to clear claude session id after spawn failure: %v", clearErr)},
               errors.Wrap(ctx, err, "start claude session")
       }
       return "", nil, errors.Wrap(ctx, err, "start claude session")
   }
   return sessionID, nil, nil
   ```
   The cached path (`goal.ClaudeSessionID() != ""`) at the top of the function is unchanged except for the new return arity.

3. Add `clearGoalSession` as a method on `goalWorkOnOperation` (id-only clear; goals carry no `metrics_sessions`, so there is no metrics rollback to assert):
   ```go
   // clearGoalSession re-reads the goal after a spawn failure and clears only the
   // claude_session_id, preserving any frontmatter the child wrote before dying.
   // The re-read is load-bearing: clearing from the stale in-memory copy would
   // revert the child's writes.
   func (g *goalWorkOnOperation) clearGoalSession(
       ctx context.Context,
       vaultPath string,
       goalName string,
   ) error {
       refreshed, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
       if err != nil {
           return errors.Wrap(ctx, err, "re-read goal after spawn failure")
       }
       refreshed.ClearClaudeSessionID()
       if err := g.goalStorage.WriteGoal(ctx, refreshed); err != nil {
           return errors.Wrap(ctx, err, "clear goal session id after spawn failure")
       }
       return nil
   }
   ```

4. In `/workspace/pkg/ops/goal_workon_test.go`:
   - Rename `It("re-reads the goal from the vault path after the session", ...)` (line 100) to `It("re-reads the goal from the vault path before spawning the session", ...)` and update its comment (the second `FindGoalByName` is now the pre-spawn persist re-read; call index 1 and arguments are unchanged).
   - Add an AC8 persist-before-spawn context (mock goal storage records the write timestamp, a REAL starter records the spawn timestamp — build the real starter and a rebuilt `goalWorkOnOp` inside the context's `BeforeEach`; the describe-level `mockStarter` counterfeiter mock is not reused here). Add `"time"` to the imports if not present:
     ```go
     Context("when persisting the goal session id before spawn", func() {
         var writeGoalAt, spawnAt time.Time
         BeforeEach(func() {
             mockGoalStorage.WriteGoalStub = func(_ context.Context, g *domain.Goal) error {
                 if g.ClaudeSessionID() != "" {
                     writeGoalAt = time.Now()
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
             goalWorkOnOp = ops.NewGoalWorkOnOperation(
                 mockGoalStorage,
                 func() string { return pinnedSessionID },
                 realStarter,
                 mockResumer,
             )
         })
         It("writes the goal session id to storage before the runner spawns the child", func() {
             Expect(err).To(BeNil())
             Expect(writeGoalAt).NotTo(BeZero())
             Expect(spawnAt).NotTo(BeZero())
             Expect(writeGoalAt.Before(spawnAt)).To(BeTrue())
         })
     })
     ```
     (The mocked suite drives `Execute` with `isInteractive=false`, so the non-interactive branch is under test.)
   - Add an argv-capture context (AC8 goal-path argv): a real starter whose fake `detachRun` records `args` and returns `(make(chan error), nil)`; assert `args` contains `"--session-id"`, `pinnedSessionID`, `"--print"`, and `"-n"` with `goalName`.
   - Add an early-exit rollback context (AC8 / Failure Modes row 13 for goals): fixture at `filepath.Join(vaultPath, "23 Goals", "Rollback Goal.md")` with `phase: planning` and `status: in_progress`; the fake `detachRun` writes `phase: execution` (`fresh.SetPhase(domain.GoalPhaseExecution.Ptr())` + `WriteGoal`), then returns `(done, nil)` where `done` immediately carries `errors.New("exit status 1")`. **Name the `Context`/`It` block so its description contains the literal phrase `goal work-on early exit rollback`** (e.g. `It("goal work-on early exit rollback preserves the child's frontmatter write", func() { … })`) — `<verification>` greps for that exact string, so a differently-worded description reports 0 matches on an otherwise-correct implementation. Drive through `Execute` and assert:
     - `err` is non-nil containing `"start work-on session"` and `"exit status 1"`;
     - on disk `written.ClaudeSessionID()` is `""` — `grep -c '^claude_session_id:' <goal-file>` returns 0;
     - `written.Phase()` is non-nil and equals `domain.GoalPhaseExecution` — the child's write survived the compensating clear.
   - Add a clear-failure warning context: `mockStarter.StartSessionReturns(ErrTest)` plus `mockGoalStorage.FindGoalByNameReturnsOnCall(2, nil, ErrTest)` (call 0 = Execute load, call 1 = pre-spawn persist re-read, call 2 = the clear's re-read); assert the error still names `"start work-on session"` and `result.Warnings` contains a warning with `"failed to clear claude session id after spawn failure"`.

5. Append a `fix:` bullet under the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (append, do not replace):
    ```
    - fix: the non-interactive `goal work-on` branch now persists the session id to the goal file before the child is spawned, and a spawn failure inside the liveness window triggers a re-read-based compensating clear of the id that preserves any frontmatter the child wrote before dying (goals carry no metrics_sessions, so there is no metrics rollback)
    ```

6. Do NOT change: the interactive goal branch's persist flow (post-return re-read — keep byte-for-byte), `pkg/ops/workon.go` (already shipped in prompt 2), `pkg/storage/`, `pkg/domain/`, `pkg/ops/claude_session.go`, the cached-path behaviour, or the bootstrap prompt string. Do NOT extract a shared helper between the task and goal paths — the codebase deliberately duplicates `persistSessionAndMetrics` / `persistGoalSessionID` (spec Open Question 2 resolves to keeping the duplication).
</requirements>

<constraints>
- Persist-before-spawn is the guarantee: the goal id is written while no child exists, so the ordering is not a race.
- The compensating clear is a re-read-modify-write preserving any frontmatter the child wrote before dying; clearing from the stale in-memory copy would reproduce the "partial state no command intended" this spec eliminates.
- Rollback is caller-side (`handleClaudeSession` / `Execute`), never inside `StartSession`.
- A failed clear surfaces as a warning on `MutationResult.Warnings`; the original spawn error is always returned, never masked.
- The interactive goal branch's flow is byte-for-byte unchanged.
- The goal path has no `metrics_sessions` — there is no metrics rollback to assert (AC8).
- `pkg/ops/` stays a pure library — no stdout. Errors use `github.com/bborbe/errors` with the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- New code ≥80% statement coverage; test error paths (spawn failure, clear failure, re-read failure).
- Do NOT commit — dark-factory handles git. `.git` is visible in this container (`workflow: direct`, no `hideGit`).
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

The goal-path tests from AC8 are present and passing:

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'writes the goal session id to storage before the runner spawns the child'   # >= 1
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c 'goal work-on early exit rollback'   # >= 1
```

The goal-path clear exists and is id-only:

```
grep -c 'clearGoalSession' pkg/ops/goal_workon.go                    # must be >= 1 (definition + call)
grep -c 'failed to clear claude session id after spawn failure' pkg/ops/goal_workon.go   # must be 1
grep -c 'MetricsSessions\|ClearMetricsSessions' pkg/ops/goal_workon.go   # must be 0 (goals have no metrics rollback)
```

The rename is in place:

```
grep -c 're-reads the goal from the vault path before spawning the session' pkg/ops/goal_workon_test.go   # must be 1
```

Changelog:

```
grep -c 'non-interactive `goal work-on` branch now persists the session id' CHANGELOG.md   # must be 1
```

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
