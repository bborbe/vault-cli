---
status: completed
spec: [036-passive-per-task-metrics]
summary: 'work-on now records one metrics_sessions entry per run via a single re-read/write: persistSessionAndMetrics replaces persistTaskSessionID, routes both fresh-start and cached-session paths through it with the injected clock timestamp, preserves cached session ids and prior entries, and keeps the prompt-182 write-back fix; four new mocked tests plus a real-storage writeback assertion cover the behavior'
execution_id: vault-cli-exec-189-spec-036-workon-start-hook
dark-factory-version: dev
created: "2026-08-24T18:30:00Z"
queued: "2026-08-24T18:31:32Z"
started: "2026-08-24T19:49:46Z"
completed: "2026-08-24T19:56:13Z"
branch: dark-factory/passive-per-task-metrics
---

<summary>
- Every `vault-cli task work-on` run that starts or reuses a Claude session appends exactly one entry to the task's `metrics_sessions` frontmatter field.
- The appended entry carries the session id of the run and a start timestamp from the injected clock.
- Running work-on a second time on the same task appends a second entry; the first run's values are untouched.
- A run where the Claude binary is unavailable (no session at all) writes no metrics entry — no anchor, no metric.
- The session-id write-back fix from prompt 182 (re-read the task after the blocking session so the session's own frontmatter writes survive) is preserved and the metrics append lands in that same re-read/write.
- A task whose cached session id already exists still gets a metrics entry on every run.
- The existing work-on behavior (status, phase, daily note, warnings, session resume) is otherwise unchanged.
</summary>

<objective>
Make the work-on path record the passive start signal: after a Claude session id is known, append a `{session_id, started_at}` entry to the task's `metrics_sessions` frontmatter field — on every run, fresh or cached session — without adding flags, extra operator steps, or a second metrics write on the common fresh-start path.
</objective>

<context>
This prompt depends on prompt `1-spec-036-metrics-frontmatter.md` having shipped: it uses `domain.MetricsSession` and `TaskFrontmatter.AppendMetricsSession`, which that prompt defines. If those symbols are absent, STOP and report `Status: failed` with "metrics frontmatter not yet deployed (prompt 1)".

Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/storage layering.

Read these files fully before making changes:

- `/workspace/pkg/ops/workon.go` — the file you change. The frozen hook site is the work-on path after the task write. Two verified functions:
  ```go
  func persistTaskSessionID(
      ctx context.Context,
      vaultPath string,
      taskName string,
      sessionID string,
      taskStorage storage.TaskStorage,
  ) (string, error) {
      refreshed, err := taskStorage.FindTaskByName(ctx, vaultPath, taskName)
      if err != nil {
          return sessionID, errors.Wrap(ctx, err, "re-read task after claude session")
      }
      refreshed.SetClaudeSessionID(sessionID)
      if err := taskStorage.WriteTask(ctx, refreshed); err != nil {
          return sessionID, errors.Wrap(ctx, err, "save session id to task")
      }
      return sessionID, nil
  }
  ```
  and its sole caller in `handleClaudeSession` (fresh-start path only; the cached-session path returns early with no write):
  ```go
  func (w *workOnOperation) handleClaudeSession(
      ctx context.Context,
      task *domain.Task,
      vaultPath string,
      sessionDir string,
      vault *config.Vault,
  ) (string, error) {
      if existing := task.ClaudeSessionID(); existing != "" {
          return existing, nil
      }
      if w.starter == nil {
          return "", ErrStarterUnavailable
      }
      prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
      slog.Info("starting claude session", "task", task.Name)
      sessionID, err := w.starter.StartSession(ctx, prompt, sessionDir, task.Name)
      if err != nil {
          return "", errors.Wrap(ctx, err, "start claude session")
      }
      return persistTaskSessionID(ctx, vaultPath, task.Name, sessionID, w.taskStorage)
  }
  ```
  The receiver has `currentDateTime libtime.CurrentDateTime`, whose `Now()` returns `libtime.DateTime` with `.Time()` (verified: `w.currentDateTime.Now().Format("2006-01-02")` and `c.currentDateTime.Now().Time()` both appear in this package). The injectable clock is the only time source.

- `/workspace/pkg/domain/task_frontmatter_metrics.go` — the accessor this prompt consumes: `AppendMetricsSession(entry domain.MetricsSession)` (appends, preserving prior entries; ignores an empty `SessionID`) and `MetricsSessions() []domain.MetricsSession`. `domain.MetricsSession` is `{SessionID string; StartedAt libtime.DateOrDateTime}`.

- `/workspace/pkg/ops/workon_test.go` — the mocked suite. Verified fixtures: `mockStarter.StartSessionReturns("session-123", nil)`, `mockTaskStorage.FindTaskByNameReturns(task, nil)`, current time set via `libtime.NewCurrentDateTime()` + `currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))`. The fresh-start path already performs TWO `FindTaskByName` calls (initial load + post-session re-read) and TWO `WriteTask` calls (initial status/phase write + the persist write) — this prompt does NOT change those counts. The cached-session contexts ("when starter is nil but task has cached session ID" at ~line 248, "when task already has a session ID" at ~line 279) currently assert NO write and make ONE lookup; this prompt changes that to one extra lookup + one write, and those contexts contain no call-count assertions today, so they keep passing — verify by running `make test`.

- `/workspace/pkg/ops/workon_session_writeback_test.go` — the prompt-182 regression test that drives real storage + a `StartSessionStub` mutating the task file on disk. After this change its task-side and goal-side cases must still pass and gain one metrics assertion each.

- `/workspace/pkg/ops/ops_suite_test.go` — defines the shared `var ErrTest`.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega with Counterfeiter fakes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` context wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — context checks in loops (the metrics write itself is a single I/O; no new loop needs one).
</context>

<requirements>

## 1. Rework the persist helper into the single metrics-recording write

In `/workspace/pkg/ops/workon.go`, replace `persistTaskSessionID` with `persistSessionAndMetrics`:

```go
// persistSessionAndMetrics re-reads the task from disk and writes back the session id
// and one metrics_sessions entry in a single write. The re-read is load-bearing: the
// StartSession call blocks for the entire headless turn, and that turn writes to this
// very task file, so writing the stale in-memory copy would revert the session's own
// frontmatter changes. Used on both the fresh-start path (the session id is new) and
// the cached-session path (the id already exists and is preserved).
func persistSessionAndMetrics(
	ctx context.Context,
	vaultPath string,
	taskName string,
	sessionID string,
	startedAt libtime.DateOrDateTime,
	taskStorage storage.TaskStorage,
) (string, error) {
	refreshed, err := taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return sessionID, errors.Wrap(ctx, err, "re-read task after claude session")
	}
	if refreshed.ClaudeSessionID() == "" {
		refreshed.SetClaudeSessionID(sessionID)
	}
	refreshed.AppendMetricsSession(domain.MetricsSession{
		SessionID: sessionID,
		StartedAt: startedAt,
	})
	if err := taskStorage.WriteTask(ctx, refreshed); err != nil {
		return sessionID, errors.Wrap(ctx, err, "save session id to task")
	}
	return sessionID, nil
}
```

Behavioral contract (this is the spec's "accumulate, never overwrite" + "one frontmatter write per operation" + the prompt-182 write-back fix, all in one):
- The re-read must never be bypassed and the stale `task` must never be written instead — a failed re-read is a hard error exactly as today.
- `SetClaudeSessionID` runs only when the re-read task has no cached session id, so a cached-session run never overwrites the id.
- Exactly one `AppendMetricsSession` call per invocation — one run, one entry (fires spec AC1/AC2).
- `startedAt` is the injected clock value of THIS run, passed in by the caller.

## 2. Route both session paths through the single write

In `/workspace/pkg/ops/workon.go`, `handleClaudeSession`, replace both terminal paths so EVERY run that yields a non-empty session id records a metrics entry:

- Cached path — replace `return existing, nil` with:
  ```go
  startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
  return persistSessionAndMetrics(ctx, vaultPath, task.Name, existing, startedAt, w.taskStorage)
  ```
- Fresh path — replace the `return persistTaskSessionID(...)` tail with:
  ```go
  startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
  return persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
  ```

Do NOT touch the `ErrStarterUnavailable` early return (`if w.starter == nil`): a run with no starter and no cached id produces no session id and therefore no metrics entry. Do NOT touch the `prompt := ...` line, the `slog.Info` line, or the `StartSession` error wrap. Do NOT touch `Execute` itself — `handleClaudeSession` is already called from the single place in `Execute`, and the interactive-resume branch consumes the returned session id unchanged.

## 3. Tests

In `/workspace/pkg/ops/workon_test.go` (the mocked suite), add new `It` blocks in the existing `Describe("WorkOnOperation")`:

1. **Fresh run records one entry.** In the default success context (starter returns `"session-123"`), assert on the persist write — the SECOND `WriteTask` call (`mockTaskStorage.WriteTaskArgsForCall(1)`):
   - `writtenTask.MetricsSessions()` has length 1,
   - entry `[0].SessionID == "session-123"`,
   - entry `[0].StartedAt` equals `libtime.DateOrDateTime(libtimetest.ParseDateTime("2026-03-03T12:00:00Z").Time())` (the injected clock; mirror the existing `SetNow` usage),
   - and `mockTaskStorage.WriteTaskCallCount()` is 2 (the initial status/phase write plus the persist write). Note: in this mocked suite `FindTaskByNameReturns(task, nil)` returns the same `*domain.Task` on every call, so `WriteTaskArgsForCall(0)` and `WriteTaskArgsForCall(1)` alias the same mutated pointer — do NOT inspect index 0 after the append (it will already carry the entry). If per-write inspection is wanted, give the re-read a distinct task via `FindTaskByNameReturnsOnCall(1, fresh, nil)`; otherwise assert call counts only.
2. **Cached run appends and preserves.** Build the second-run state per spec AC2: before `Execute`, set `task.SetClaudeSessionID("cached-session-456")` and append one existing entry `domain.MetricsSession{SessionID: "first-session", StartedAt: <some time>}` to the task that `FindTaskByName` returns. After `Execute`:
   - `mockTaskStorage.WriteTaskCallCount()` is 2 (the initial write + the persist write),
   - the persist write's task has `MetricsSessions()` length 2,
   - entry 0 is byte-identical to the pre-existing first entry (SessionID `"first-session"` and the same StartedAt — untouched),
   - entry 1 has `SessionID == "cached-session-456"`,
   - `writtenTask.ClaudeSessionID() == "cached-session-456"` (not overwritten).
3. **No-anchor run records nothing.** In the `"when starter is nil and task has no cached session ID"` context, add: `mockTaskStorage.WriteTaskCallCount()` is 1 (only the initial status/phase write) and the written task has `MetricsSessions()` nil.
4. **Failed re-read still fails hard.** In the existing post-session re-read failure context (from prompt 182), keep its assertions and add one: no metrics entry is appended (the error path returns before append).

In `/workspace/pkg/ops/workon_session_writeback_test.go` (the real-storage regression test), extend the task-side case: after the existing assertions, add
```go
Expect(written.MetricsSessions()).To(HaveLen(1))
Expect(written.MetricsSessions()[0].SessionID).To(Equal("session-123"))
```
This proves the metrics entry lands in the same re-read/write that preserves the session's own frontmatter writes (real storage round-trip — the serialization boundary).

## 4. Changelog — NOT this prompt

Do NOT add a CHANGELOG entry. Prompt `4-spec-036-plugin-docs-release.md` owns the single `## Unreleased` bullet. Do NOT touch `/workspace/pkg/domain/`, `/workspace/pkg/cli/`, `/workspace/pkg/storage/`, `/workspace/mocks/`, `/workspace/commands/`, or `/workspace/CHANGELOG.md`.

</requirements>

<constraints>
- Metrics land only in the task frontmatter field `metrics_sessions` (this prompt). `metrics_completed_at`, `metrics_interaction_count`, and `metrics_cycles` are later prompts.
- All timestamps come from the injected `w.currentDateTime` clock, serialized as `libtime.DateOrDateTime` (ISO 8601 with timezone offset).
- Accumulate, never overwrite: work-on must not truncate or replace prior `metrics_sessions` entries; the only path that clears the active accumulator is recurring-task completion, which is a later prompt.
- Existing behavior must not regress: spec 015 recurring completion clears `claude_session_id` (untouched, complete is a later prompt), the incomplete-checkbox guard, daily-note and goal roll-up, status/phase semantics. All existing tests in `pkg/ops` continue to pass. The `NumTurns` value read at session start is unrelated to the interaction count and unchanged.
- The prompt-182 write-back fix is load-bearing: `handleClaudeSession` must keep re-reading the task from `vaultPath` after `StartSession` and must never write the stale in-memory `task` copy.
- Do NOT add a config field, flag, or environment variable to opt out of metrics recording. Passive recording is the invariant.
- `goal work-on` (`/workspace/pkg/ops/goal_workon.go`) is deliberately OUT OF SCOPE — spec 036's metrics are task-only (every AC names `vault-cli task work-on` / `task complete`). Its `handleClaudeSession` is a sibling implementation; do NOT extend the metrics append to goals and do NOT modify `/workspace/pkg/ops/goal_workon.go`.
- Do NOT add a new E2E scenario.
- `pkg/ops/` is a library layer — no `fmt.Print*`, no `os.Stdout` writes. `fmt.Sprintf` and `slog` are fine.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT change any interface in `/workspace/pkg/storage/storage.go`; no new storage method, no new persistence layer.
- Do NOT regenerate or hand-edit `/workspace/mocks/` — no interface changed, so `go generate` must produce no diff there.
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

**2. The old helper is gone, the new one is in place** — each command must print exactly the stated count:
```
grep -c 'func persistTaskSessionID(' pkg/ops/workon.go
grep -c 'func persistSessionAndMetrics(' pkg/ops/workon.go
grep -c 'persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)' pkg/ops/workon.go
grep -c 'persistSessionAndMetrics(ctx, vaultPath, task.Name, existing, startedAt, w.taskStorage)' pkg/ops/workon.go
```
First must print `0`; the others `1` each.

**3. Exactly one AppendMetricsSession call in the persist path** — must print `1`:
```
grep -c 'refreshed.AppendMetricsSession(domain.MetricsSession{' pkg/ops/workon.go
```
The other (non-fresh, nil-starter) paths must not append — must print `0`:
```
grep -c 'AppendMetricsSession' pkg/ops/workon.go
```
Wait: the previous grep counts the single call site and this grep counts all mentions; the second must be exactly `1` too (one call site, plus the two `persistSessionAndMetrics` call sites which pass the entry by value and do NOT mention `AppendMetricsSession`). If it prints more than `1`, an extra append exists — fix it.

**4. Cached-session run now writes** — the default mocked success path still has exactly two `FindTaskByName` calls (load + re-read):
```
grep -c 'Expect(mockTaskStorage.FindTaskByNameCallCount()).To(Equal(2))' pkg/ops/workon_test.go
```
Must print `>= 1`.

**5. Named test specs are present and pass** — each must print a number `>= 1`:
```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Fresh run records one entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Cached run appends and preserves"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "No-anchor run records nothing"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "keeps the frontmatter the session wrote and adds claude_session_id"
```
Use the exact `It` descriptions from requirement 3 for the first three (adjust these grep patterns to match your chosen descriptions and keep them consistent). The fourth is the prompt-182 regression spec, which must still pass.

**6. Mutation check — removing the append fails the test.** Verify by hand once:
1. Temporarily comment out the `refreshed.AppendMetricsSession(...)` line in `persistSessionAndMetrics`.
2. Run `go test -count=1 ./pkg/ops/`.
3. Confirm a failure names `MetricsSessions` (e.g. expected length 1, got 0).
4. Restore the line.
5. **Restore gate — run before anything else:**
   ```
   grep -c 'refreshed.AppendMetricsSession(domain.MetricsSession{' pkg/ops/workon.go
   ```
   Must print `1`. If `0`, the restore did not happen — restore and re-check. Do not continue until it prints `1`.
6. Re-run `go test -count=1 ./pkg/ops/` and confirm green.

Do **not** pass `-run` here: the suite entrypoint is `func TestSuite(t *testing.T)` in `pkg/ops/ops_suite_test.go`; a wrong `-run` matches nothing and exits 0. Confirm tests ran by checking output reports a non-zero spec count.

**7. Out-of-scope files unchanged** — must print `0`:
```
grep -rc 'metrics_' pkg/cli/ pkg/storage/ | grep -v ':0' | wc -l
```
(`pkg/domain/` — prompt 1's `task_frontmatter_metrics.go` — and `pkg/ops/` are the only trees expected to mention `metrics_`; scan only the out-of-scope trees for a true 0.)

**8. Full gate, once, at the end:**
```
make precommit
```
Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
