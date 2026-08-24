---
status: completed
spec: [036-passive-per-task-metrics]
summary: 'Wired the passive complete-end metrics hook: new InteractionCounter streams Claude session JSONL logs (unsafe ids rejected, missing/malformed files contribute 0), task complete writes metrics_completed_at and metrics_interaction_count in the same write, and recurring completion archives one metrics_cycles entry then clears the accumulator; unit + integration tests green, make precommit exits 0.'
execution_id: vault-cli-exec-190-spec-036-complete-end-hook
dark-factory-version: dev
created: "2026-08-24T18:30:00Z"
queued: "2026-08-24T18:31:32Z"
started: "2026-08-24T19:56:15Z"
completed: "2026-08-24T20:07:11Z"
branch: dark-factory/passive-per-task-metrics
---

<summary>
- `vault-cli task complete` writes `metrics_completed_at` (the injected-clock completion timestamp) into the task file in the same write as the status and completed-date update.
- For a task that was worked on, complete also derives `metrics_interaction_count` — the total number of `type: "user"` entries across the task's recorded Claude session JSONL logs — and writes it in the same write.
- A task completed without ever running work-on gets `metrics_completed_at` but NO `metrics_interaction_count`: unknown is never forged as zero, and completion never fails for lack of logs.
- For a recurring task that was worked on, complete archives the finished cycle as one `metrics_cycles` entry (earliest started, completed, interaction count) and then clears the active accumulator so the next cycle measures fresh; status stays `in_progress` and `claude_session_id` is cleared as today.
- Missing, unreadable, or malformed session log files contribute 0 each and never block completion; session ids that could escape the project directory are rejected and contribute 0.
- Session logs are read streamed with bounded memory — never loaded whole — and `~/.claude` is read-only to vault-cli.
- The counter is a new small reader injected into the complete operation; no new service or datastore.
</summary>

<objective>
Give `task complete` the passive end signal: write the completion timestamp, derive and store the interaction count from the task's Claude session logs (best-effort, never failing), and archive/reset the metrics accumulator on recurring completion — all inside the existing single task write per completion.
</objective>

<context>
This prompt depends on prompts `1-spec-036-metrics-frontmatter.md` and `2-spec-036-workon-start-hook.md` having shipped: it consumes `domain.MetricsSession`, `TaskFrontmatter.AppendMetricsSession`/`MetricsSessions`/`SetMetricsCompletedAt`/`SetMetricsInteractionCount`/`MetricsCycles`/`AppendMetricsCycle`/`ClearMetricsSessions`/`ClearMetricsCompletedAt`/`ClearMetricsInteractionCount` and the `metrics_sessions` entries written by work-on. If those symbols are absent, STOP and report `Status: failed` with "metrics frontmatter not yet deployed (prompt 1)".

Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/storage layering.

Read these files fully before making changes:

- `/workspace/pkg/ops/complete.go` — the file you change. Verified shapes:
  - Constructor + struct:
    ```go
    func NewCompleteOperation(
        taskStorage storage.TaskStorage,
        goalStorage storage.GoalStorage,
        dailyNoteStorage storage.DailyNoteStorage,
        currentDateTime libtime.CurrentDateTime,
    ) CompleteOperation {
        return &completeOperation{
            taskStorage:      taskStorage,
            goalStorage:      goalStorage,
            dailyNoteStorage: dailyNoteStorage,
            currentDateTime:  currentDateTime,
        }
    }
    type completeOperation struct {
        taskStorage      storage.TaskStorage
        goalStorage      storage.GoalStorage
        dailyNoteStorage storage.DailyNoteStorage
        currentDateTime  libtime.CurrentDateTime
    }
    ```
  - Non-recurring path in `Execute` (the frozen hook site, before `WriteTask` at ~line 116):
    ```go
    _ = task.SetStatus(domain.TaskStatusCompleted)
    task.SetPhase(domain.TaskPhaseDone.Ptr())
    nowTime := c.currentDateTime.Now().Time()
    completedD := libtime.DateOrDateTime(nowTime)
    task.SetCompletedDate(&completedD)

    // Write updated task
    if err := c.taskStorage.WriteTask(ctx, task); err != nil {
    ```
  - Recurring path `handleRecurringTask` (frozen hook site before `WriteTask` at ~line 228): it already calls `task.ClearClaudeSessionID()` (spec 015 behavior — do not remove), then `WriteTask`. `now := c.currentDateTime.Now().Time()` is available at the top.

- `/workspace/pkg/cli/cli.go` — the complete command wiring at ~line 169-180 (verified):
  ```go
  func(ctx context.Context, vault *config.Vault) (ops.MutationResult, error) {
      storageConfig := storage.NewConfigFromVault(vault)
      taskStore := storage.NewTaskStorage(storageConfig)
      goalStore := storage.NewGoalStorage(storageConfig)
      dailyStore := storage.NewDailyNoteStorage(storageConfig)
      completeOp := ops.NewCompleteOperation(
          taskStore,
          goalStore,
          dailyStore,
          currentDateTime,
      )
      result, err := completeOp.Execute(ctx, vault.Path, taskName, vault.Name, force)
  ```
  The work-on wiring two functions above it computes the session directory the same way every session cwd is chosen:
  ```go
  sessionDir := vault.Path
  if dir := vault.GetSessionProjectDir(); dir != "" {
      sessionDir = dir
  }
  ```
  `cli.go` already imports `os` and `path/filepath`.

- `/workspace/pkg/ops/complete_test.go` — the mocked suite. Verified: `BeforeEach` calls `ops.NewCompleteOperation(mockTaskStorage, mockGoalStorage, mockDailyNoteStorage, currentDateTime)` with the clock set via `currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))`; the recurring-daily context (line ~467) sets `task.SetRecurring("daily")`, `task.SetClaudeSessionID("test-session-uuid")`, status in_progress, and asserts on `mockTaskStorage.WriteTaskArgsForCall(0)`.

- `/workspace/pkg/ops/workon_test.go` — for the `MetricsSession` fixture shape used by the mocked complete tests (set the task's sessions via `task.AppendMetricsSession(domain.MetricsSession{...})` exactly as prompt 2's tests do).

- `/workspace/pkg/ops/ops_suite_test.go` — defines `var ErrTest`.

- `/workspace/pkg/storage/storage.go` — verified narrow interfaces; do NOT change them.

Claude Code session log layout (external contract, do not re-derive): each session's JSONL log lives at `~/.claude/projects/<encoded-project-dir>/<session-id>.jsonl`, where `<encoded-project-dir>` is the session's cwd encoded by Claude Code (leading `/` → `-`, every `/` → `-`, every other character outside `[A-Za-z0-9-]` → `-`; e.g. `/Users/bborbe/Documents/vault` → `-Users-bborbe-Documents-vault`). Each JSONL line is an object with a `type` field; `type: "user"` entries are the operator's turns. The session cwd is the same `sessionDir` the work-on wiring passes to `StartSession` — i.e. `vault.Path` or `vault.GetSessionProjectDir()`.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega with Counterfeiter fakes, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md` — gosec (`#nosec` with reasons for `os.Open` on a user-controlled path).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — non-blocking context check in the per-line scan loop.
</context>

<requirements>

## 1. New interaction counter

Create `/workspace/pkg/ops/interaction_count.go`:

```go
//counterfeiter:generate -o ../../mocks/interaction-counter.go --fake-name InteractionCounter . InteractionCounter

// InteractionCounter counts user interactions from a task's recorded Claude session logs.
type InteractionCounter interface {
	// Count returns the total number of type:"user" entries across the session JSONL
	// logs for the given session ids. Missing, unreadable, malformed, or unsafe session
	// ids contribute 0. It never returns an error and never blocks completion.
	Count(ctx context.Context, sessionIDs []string) int
}

// NewInteractionCounter creates an InteractionCounter that reads session logs under
// projectsDir/<encoded sessionDir>/. projectsDir is the Claude Code projects base
// (typically <home>/.claude/projects); sessionDir is the cwd the sessions were started
// in (the vault path or its session_project_dir override).
func NewInteractionCounter(projectsDir string, sessionDir string) InteractionCounter {
	return &interactionCounter{projectsDir: projectsDir, sessionDir: sessionDir}
}
```

Implementation (`interactionCounter`) requirements:

- `Count`: for each session id, validate it with `isSafeSessionID` (skip invalid → contribute 0), build `filepath.Join(c.projectsDir, encodeProjectDir(c.sessionDir), id+".jsonl")`, and add `countUserTurnsInSessionLog(ctx, path)`.
- `isSafeSessionID(id string) bool`: returns false for empty, `"."`, `".."`, any id containing `/` or `\`, and any id containing the substring `..`. This is the spec's path-traversal guard: a session id from the task must never escape the encoded project directory. (Security section of spec 036 — load-bearing.)
- `encodeProjectDir(path string) string`: the Claude Code encoding — every `/` (including a leading one) becomes `-`; every other character outside `[A-Za-z0-9-]` becomes `-`; everything else is kept. `/Users/bborbe/Documents/vault` → `-Users-bborbe-Documents-vault`.
- `countUserTurnsInSessionLog(ctx, path) int`: `os.Open` (read-only) — on open failure return 0 (spec failure mode: "Session JSONL missing, pruned, or unreadable → contributes 0"). Add `//#nosec G304 -- session ids are validated by isSafeSessionID; read-only best-effort` to the `os.Open` line. Stream with a `bufio.Scanner` (set `scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)` — Claude Code lines are large) and `json.Unmarshal` each line into a `struct { Type string \`json:"type"\` }`; a line that fails to parse is skipped (spec security: "session log lines that fail to parse are skipped, never panic"); count `Type == "user"`. Never `os.ReadFile` the whole file — bounded memory (spec failure mode: "Very large session JSONL → streamed parse"). In the scan loop, include the standard non-blocking `select { case <-ctx.Done(): return count; default: }` check per `go-context-cancellation-in-loops.md`.
- Do NOT create a `NewInteractionCounterForTesting` and do NOT export `encodeProjectDir` — tests exercise the encoding through the fixture-dir behavior (requirement 4).

## 2. Complete operation wiring

In `/workspace/pkg/ops/complete.go`:

- Add `interactionCounter InteractionCounter` to the `completeOperation` struct and to `NewCompleteOperation` as a new fifth parameter. There are exactly two call sites of `NewCompleteOperation`: `/workspace/pkg/cli/cli.go` (~line 174) and `/workspace/pkg/ops/complete_test.go` (~line 44) — update BOTH. No other caller exists (verified by grep).

- Non-recurring path — in `Execute`, immediately before the `// Write updated task` block, add:
  ```go
  task.SetMetricsCompletedAt(&completedD)
  if sessions := task.MetricsSessions(); len(sessions) > 0 {
      task.SetMetricsInteractionCount(c.interactionCounter.Count(ctx, metricsSessionIDs(sessions)))
  }
  ```
  This lands `metrics_completed_at` and (when worked) `metrics_interaction_count` in the SAME write as the status and completed-date update (fires spec AC3). A task with no `metrics_sessions` gets `metrics_completed_at` but no `metrics_interaction_count` — unknown, never zero, and completion never fails (fires AC6).

- Recurring path — in `handleRecurringTask`, immediately before the `// Write updated task` block, add:
  ```go
  if sessions := task.MetricsSessions(); len(sessions) > 0 {
      task.AppendMetricsCycle(domain.MetricsCycle{
          StartedAt:        earliestStartedAt(sessions),
          CompletedAt:      libtime.DateOrDateTime(now),
          InteractionCount: c.interactionCounter.Count(ctx, metricsSessionIDs(sessions)),
      })
  }
  task.ClearMetricsSessions()
  task.ClearMetricsCompletedAt()
  task.ClearMetricsInteractionCount()
  ```
  Behavior (fires AC5): a recurring task that was worked on archives one aggregate cycle and clears the active accumulator so the next cycle measures fresh; status stays `in_progress` (the existing code never sets completed) and `claude_session_id` is cleared by the existing `task.ClearClaudeSessionID()` call — leave that line untouched. A recurring task with no `metrics_sessions` archives nothing (no fabricated cycle with a zero count) but still clears any stale accumulator fields. `now` is already in scope (`now := c.currentDateTime.Now().Time()`).

- Add the two small package helpers in `interaction_count.go` (or `complete.go`, your choice — same package):
  ```go
  // metricsSessionIDs extracts the session ids from metrics_sessions entries.
  func metricsSessionIDs(sessions []domain.MetricsSession) []string

  // earliestStartedAt returns the minimum StartedAt across the sessions.
  func earliestStartedAt(sessions []domain.MetricsSession) libtime.DateOrDateTime
  ```
  `earliestStartedAt` is used for "human time per recurring task" (completed minus earliest started) — the north star of spec 036's measure half. A non-empty slice is guaranteed by the caller (both call sites check `len(sessions) > 0`); still handle it defensively (return the zero value rather than panicking on an empty slice).

## 3. CLI wiring

In `/workspace/pkg/cli/cli.go`, inside the complete command's vault loop (the closure shown in `<context>`), replace the `completeOp := ops.NewCompleteOperation(...)` block with:

```go
sessionDir := vault.Path
if dir := vault.GetSessionProjectDir(); dir != "" {
    sessionDir = dir
}
homeDir, err := os.UserHomeDir()
if err != nil {
    return ops.MutationResult{}, errors.Wrap(ctx, err, "get home directory")
}
projectsDir := filepath.Join(homeDir, ".claude", "projects")
interactionCounter := ops.NewInteractionCounter(projectsDir, sessionDir)
completeOp := ops.NewCompleteOperation(
    taskStore,
    goalStore,
    dailyStore,
    currentDateTime,
    interactionCounter,
)
```

This mirrors the work-on wiring's session-dir selection exactly, so the counter reads the logs of the very sessions work-on started. `os`, `path/filepath`, `errors`, and `ops` are already imported. No other CLI command changes.

## 4. Tests

### 4a. `/workspace/pkg/ops/interaction_count_test.go` (new) — `package ops_test`

Drive the real counter against fixture JSONL files in temp dirs (`os.MkdirTemp`, `os.RemoveAll` in `AfterEach`, mirroring `/workspace/pkg/ops/wikilink_roundtrip_test.go`). A fixture line is `{"type":"user","message":{...}}` — the `Type` field is what matters; other fields are ignored. Cases:

1. **Sum across sessions**: session dir `/home/node/vault-cli`, fixture files at `<projectsDir>/-home-node-vault-cli/s1.jsonl` (N user turns) and `s2.jsonl` (M user turns) → `Count(ctx, []string{"s1","s2"}) == N+M`. This pins `encodeProjectDir` behavior through the fixture path — if the encoding were wrong, the files would not be found and the count would be 0.
2. **Missing file contributes 0**: `Count(ctx, []string{"missing"}) == 0`.
3. **Malformed lines skipped**: a fixture with one valid user line and two garbage lines → count 1, no panic.
4. **Non-user types ignored**: fixture with `assistant`, `system`, `summary`, and `user` lines → only `user` counted.
5. **Unsafe session ids rejected**: `"../x"`, `"a/b"`, `"a\\b"`, `".."`, `""` → each contributes 0. Place a decoy `user`-turn file at `<projectsDir>/-home-node-vault-cli/..-x.jsonl`... do NOT create a traversal-capable fixture — instead assert the count is 0 for all five ids and that the call never touches a path outside the project dir (the `isSafeSessionID` guard is unit-covered by these cases).
6. **Large line**: a fixture line of ~200KB is counted without error (exercises the scanner buffer).

### 4b. `/workspace/pkg/ops/complete_test.go` — update the constructor and add cases

- Before touching `complete_test.go`, run `go generate ./...` so `mocks/interaction-counter.go` is created from requirement 1's counterfeiter annotation. This matters: `make precommit` (verification step 11) also runs generate, but the intermediate `make test` (step 1) and the step-6 `go test` compile `pkg/ops` and will fail with `undefined: mocks.InteractionCounter` unless the mock exists first.
- Then update the `BeforeEach` (line ~44): add `mockInteractionCounter := &mocks.InteractionCounter{}` to the suite vars and pass it as the fifth argument. The counterfeiter default returns 0, so existing tests that assert `metrics_completed_at`/absence keep their meaning; for count-specific tests set `mockInteractionCounter.CountReturns(7)` (or use `CountStub`).
- New cases (each inspects `mockTaskStorage.WriteTaskArgsForCall(0)`):
  1. **Non-recurring worked**: task with one `AppendMetricsSession(domain.MetricsSession{SessionID: "s1", StartedAt: <fixed time>})`; `mockInteractionCounter.CountReturns(7)`. After `Execute`: `writtenTask.MetricsCompletedAt()` equals the injected-clock completion time; `*writtenTask.MetricsInteractionCount() == 7`; `mockInteractionCounter.CountCallCount() == 1` and the args passed are `[]string{"s1"}`.
  2. **Non-recurring no-anchor (AC6)**: task with NO metrics sessions. After `Execute`: `writtenTask.MetricsCompletedAt()` non-nil; `writtenTask.MetricsInteractionCount()` nil; `mockInteractionCounter.CountCallCount() == 0`.
  3. **Recurring worked (AC5)**: task `SetRecurring("daily")`, status in_progress, `claude_session_id` set, and two `AppendMetricsSession` entries with different StartedAt (e.g. 10:00 and 11:00); `CountReturns(7)`. After `Execute`: `writtenTask.MetricsCycles()` has 1 entry with `StartedAt` = the earlier (10:00) entry, `CompletedAt` = injected-clock completion time, `InteractionCount == 7`; `writtenTask.MetricsSessions()` nil; `writtenTask.MetricsCompletedAt()` nil; `writtenTask.MetricsInteractionCount()` nil; `writtenTask.Status() == domain.TaskStatusInProgress`; `writtenTask.Get("claude_session_id")` nil.
  4. **Recurring not worked**: recurring task with NO metrics sessions. After `Execute`: `writtenTask.MetricsCycles()` nil; the accumulator keys absent.
  5. **Count never fails**: `mockInteractionCounter.CountStub = func(...) int { return 0 }` with a task that has one session — completion still returns `Success: true` and `err == nil`.

### 4c. Mutation check (AC4)

The AC4 fixture test is the interaction-count sum case (4a case 1) plus the complete-op case 4b.1: removing the `c.interactionCounter.Count(...)` call in `complete.go` must make case 4b.1 fail (the written count would be 0, not 7). Verify by hand once: comment the `SetMetricsInteractionCount` line, run `go test -count=1 ./pkg/ops/`, confirm a failure naming `MetricsInteractionCount`, restore, re-run green.

## 5. Changelog — NOT this prompt

Do NOT add a CHANGELOG entry. Prompt `4-spec-036-plugin-docs-release.md` owns the single `## Unreleased` bullet. Do NOT touch `/workspace/pkg/domain/`, `/workspace/mocks/` (the new `InteractionCounter` fake is produced by `go generate` from the counterfeiter annotation — `make precommit` runs generate), `/workspace/commands/`, or `/workspace/CHANGELOG.md`.

</requirements>

<constraints>
- Metrics land only in the task frontmatter fields `metrics_completed_at`, `metrics_interaction_count`, and `metrics_cycles` (this prompt; `metrics_sessions` was prompt 2). No new service, no new datastore, no Prometheus sink.
- All timestamps come from the injected `c.currentDateTime` clock as `libtime.DateOrDateTime` (ISO 8601 with timezone offset).
- The interaction count is the total of `type: "user"` entries across the task's recorded session logs; a missing, unreadable, or malformed session file contributes 0 and never fails completion; an unsafe session id contributes 0 and never reads outside the encoded project directory.
- A task completed with no `metrics_sessions` gets `metrics_completed_at` but NO `metrics_interaction_count` — unknown, never zero, no new error, no enforcement.
- Existing behavior must not regress: spec 015 (recurring completion clears `claude_session_id` — keep the existing `ClearClaudeSessionID` call), the incomplete-checkbox guard, daily-note and goal roll-up, status/phase semantics. All existing tests in `pkg/ops` continue to pass. The `NumTurns` value read at session start is unrelated and unchanged.
- vault-cli reads Claude Code session logs read-only; it never writes, deletes, or follows symlinks outside the operated vault's encoded project directory. `~/.claude` is never written by vault-cli.
- Do NOT add an opt-out flag, config key, or environment variable for metrics recording — passive recording is the invariant.
- `goal complete` (`/workspace/pkg/ops/goal_complete.go`) and `objective complete` (`/workspace/pkg/ops/objective_complete.go`) are deliberately OUT OF SCOPE — spec 036's metrics are task-only. Do NOT wire the interaction counter into those sibling operations and do NOT modify those files.
- Do NOT add a new E2E scenario.
- `pkg/ops/` is a library layer — no `fmt.Print*`, no `os.Stdout` writes. `fmt.Sprintf` and `slog` are fine.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT change any interface in `/workspace/pkg/storage/storage.go`; the `CompleteOperation` interface and `MutationResult` are unchanged (only the constructor and struct gain the counter).
- Do NOT commit — dark-factory handles git. Do NOT bump any version string, do NOT create a git tag (this repo is `autoRelease: true`; the releaser owns version bumps).
- Existing tests must still pass.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**
```
make test
```
Must exit 0.

**2. The counter exists with the expected API** — each command must print exactly one line:
```
grep -n 'type InteractionCounter interface' pkg/ops/interaction_count.go
grep -n 'func NewInteractionCounter(projectsDir string, sessionDir string) InteractionCounter' pkg/ops/interaction_count.go
grep -n 'Count(ctx context.Context, sessionIDs \[\]string) int' pkg/ops/interaction_count.go
grep -n '//counterfeiter:generate -o ../../mocks/interaction-counter.go' pkg/ops/interaction_count.go
```

**3. The complete hooks are in place** — each must print `1`:
```
grep -c 'task.SetMetricsCompletedAt(&completedD)' pkg/ops/complete.go
grep -c 'task.SetMetricsInteractionCount(c.interactionCounter.Count(ctx, metricsSessionIDs(sessions)))' pkg/ops/complete.go
grep -c 'task.AppendMetricsCycle(domain.MetricsCycle{' pkg/ops/complete.go
grep -c 'task.ClearMetricsSessions()' pkg/ops/complete.go
grep -c 'task.ClearMetricsCompletedAt()' pkg/ops/complete.go
grep -c 'task.ClearMetricsInteractionCount()' pkg/ops/complete.go
```

**4. The constructor gained the counter at both call sites** — each must print `1`:
```
grep -c 'ops.NewInteractionCounter(projectsDir, sessionDir)' pkg/cli/cli.go
grep -c 'interactionCounter InteractionCounter' pkg/ops/complete.go
grep -c 'mockInteractionCounter' pkg/ops/complete_test.go
```

**5. The mock is generated** — the counterfeiter fake exists:
```
ls mocks/interaction-counter.go
```

**6. Named test specs are present and pass** — each must print a number `>= 1`:
```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Sum across sessions"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Non-recurring worked"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Recurring worked"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Unsafe session ids rejected"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Malformed lines skipped"
```
Use `It`/`Describe` descriptions matching these patterns (adjust the grep patterns to your chosen descriptions and keep them consistent).

**7. Path-traversal guard is present** — must print `1` each:
```
grep -c 'func isSafeSessionID(id string) bool' pkg/ops/interaction_count.go
grep -c 'isSafeSessionID(id)' pkg/ops/interaction_count.go
```

**8. Bounded-memory read** — no whole-file read in the counter; must print `0`:
```
grep -c 'os.ReadFile' pkg/ops/interaction_count.go
```

**9. Mutation check for AC4** — see requirement 4c. Verify by hand: comment the `SetMetricsInteractionCount` call, confirm the 4b.1 spec fails naming `MetricsInteractionCount`, restore, and confirm the **restore gate** prints `1` before continuing:
```
grep -c 'task.SetMetricsInteractionCount(c.interactionCounter.Count(ctx, metricsSessionIDs(sessions)))' pkg/ops/complete.go
```

**10. Changelog untouched by this prompt** — must still print `0`:
```
grep -c '^## Unreleased$' CHANGELOG.md
```
(It does not exist yet; prompt 4 creates it. If it prints `1` from a prior unrelated run, leave it alone.)

**11. Full gate, once, at the end:**
```
make precommit
```
Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
