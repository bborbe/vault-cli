---
status: approved
spec: [017-enforce-status-in-progress-on-calendar-date]
created: "2026-06-14T14:30:00Z"
queued: "2026-06-14T15:39:27Z"
branch: dark-factory/enforce-status-in-progress-on-calendar-date
---

<summary>
- `vault-cli task defer` on a `next` or `backlog` task now also writes `status: in_progress` in the same file write — closing the create-side leak at write-time
- Auto-promote is gated to `next` and `backlog` only — `in_progress`, `completed`, `aborted`, and `hold` are left untouched
- `defer` on an already-`in_progress` task is idempotent (status line is NOT re-written — only `defer_date` is set)
- `defer` on `completed` / `aborted` / `hold` tasks leaves the status line byte-identical — terminal and held statuses are preserved
- All existing defer semantics (past-date validation, planned_date clearing when before target, daily-note updates) continue to work unchanged
- Ginkgo tests in `defer_test.go` cover promote, no-op, and idempotent paths

</summary>

<objective>
Inject a status auto-promote in `findAndDeferTask` (`pkg/ops/defer.go`): when the current status is `next` or `backlog`, the same file write that sets `defer_date` also sets `status: in_progress`. For any other status, the status line is left untouched. This closes the create-side leak at write-time so the Kanban board cannot miss a deferred task again.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Read these files in full before making changes:
- `/workspace/pkg/ops/defer.go` — full file. The injection point is `findAndDeferTask` (lines 134-151). The current code:
  ```go
  func (d *deferOperation) findAndDeferTask(
      ctx context.Context,
      task *domain.Task,
      targetDate libtime.DateOrDateTime,
  ) (*domain.Task, error) {
      task.SetDeferDate(targetDate.Ptr())

      // Clear planned_date if it's before the defer target date
      if task.PlannedDate() != nil && task.PlannedDate().Before(targetDate) {
          task.SetPlannedDate(nil)
      }

      if err := d.taskStorage.WriteTask(ctx, task); err != nil {
          return nil, errors.Wrap(ctx, err, "write task")
      }
      return task, nil
  }
  ```
- `/workspace/pkg/ops/defer_test.go` — full file. The default `BeforeEach` (line 35) creates a `domain.Task` with `{"status": "todo"}`. Existing Contexts (line 60+) reuse this default; new Contexts override the status via a per-Context `BeforeEach`.
- `/workspace/pkg/domain/task_frontmatter.go` — `SetStatus` (lines 175-182) signature: `func (f *TaskFrontmatter) SetStatus(s TaskStatus) error`. Note: `SetStatus` is on `*TaskFrontmatter`, NOT on `*Task` — and there is NO named `Frontmatter` field on `Task`. The `Task` struct EMBEDS `TaskFrontmatter`, so `task.Status()`, `task.SetStatus(...)`, and `task.GetString("status")` are promoted method calls (call them on the `*Task` value directly — verify by reading `pkg/domain/task.go`).
- `/workspace/pkg/domain/task_status.go` — `TaskStatusNext`, `TaskStatusBacklog`, `TaskStatusInProgress`, `TaskStatusCompleted`, `TaskStatusHold`, `TaskStatusAborted` constants and their `Validate` methods.

The `findAndDeferTask` function is the single injection point. Do NOT touch `Execute`, `updateDailyNotes`, or any other function in `defer.go`. The single file write to `taskStorage.WriteTask` must remain a single write — the auto-promote happens on the in-memory `task` object BEFORE that single `WriteTask` call.

Prompt 1 must be completed first. The detector and fix are prerequisites for the lint-driven safety net, but defer's auto-promote is independently correct without Prompt 1 — both prompts are testing their own behavior. The dependency noted in the spec ("shared test fixture conventions") is real: defer_test.go's `BeforeEach` line 47-51 uses `{"status": "todo"}` as the default, and existing tests assert `Status() == domain.TaskStatusNext` (line 73) because `todo` is normalized to `next` via `NormalizeTaskStatus`. The new tests for auto-promote will override the status to `next` / `backlog` / `in_progress` / `completed` / etc. per-Context. The pre-existing `+7d` Context (line 61) asserts "does not change task status" (line 70) — this test should be updated because under the new spec, `+7d` on the default `todo` task (normalized to `next`) WILL now promote to `in_progress`. The existing assertion must be flipped from `Status() == domain.TaskStatusNext` to `Status() == domain.TaskStatusInProgress` AND a new test must be added for the case where status is already `in_progress` (idempotent: no flip). The spec AC 7 says "leaves the status line byte-identical" for `in_progress` — verify by reading the line byte-value, not by calling `Status()` (which normalizes).

**Verify the spec's claim about idempotence by reading the in-memory behavior**: when `task.Status()` is already `in_progress` and the auto-promote block runs, calling `SetStatus(domain.TaskStatusInProgress)` is a no-op for the in-memory state (it just rewrites the same string to the same key) and the byte-serialized form is also unchanged. The "byte-identical" property holds trivially.
</context>

<requirements>

### 1. Add status auto-promote block in `findAndDeferTask` in `pkg/ops/defer.go`

In `findAndDeferTask` (lines 134-151), add a new block BETWEEN the `task.SetDeferDate(targetDate.Ptr())` call and the `planned_date` clearing block. The block must:

1. Read the current status via `task.Status()` (returns `domain.TaskStatus`, already normalized via `NormalizeTaskStatus`).
2. If the current status is `domain.TaskStatusNext` or `domain.TaskStatusBacklog`, call `task.SetStatus(domain.TaskStatusInProgress)`. `SetStatus` returns an error — handle it by returning `(nil, errors.Wrap(ctx, err, "set status to in_progress"))` from `findAndDeferTask` (mirrors the existing error-wrap style at line 148).
3. For any other status (`in_progress`, `completed`, `aborted`, `hold`, or empty), do nothing — the status line is left untouched.

```go
// findAndDeferTask updates task defer status and writes it.
func (d *deferOperation) findAndDeferTask(
    ctx context.Context,
    task *domain.Task,
    targetDate libtime.DateOrDateTime,
) (*domain.Task, error) {
    task.SetDeferDate(targetDate.Ptr())

    // Calendar-as-commitment: promote status when deferring a non-active task.
    // Per spec 017: deferring to a future date is a commitment to work the task;
    // next/backlog tasks are invisible to the Kanban board and miss cadence.
    // Promote to in_progress so the board surfaces the task on its target day.
    // Idempotent on in_progress; no-op on completed/aborted/hold (out of scope).
    if status := task.Status(); status == domain.TaskStatusNext || status == domain.TaskStatusBacklog {
        if err := task.SetStatus(domain.TaskStatusInProgress); err != nil {
            return nil, errors.Wrap(ctx, err, "set status to in_progress")
        }
    }

    // Clear planned_date if it's before the defer target date
    if task.PlannedDate() != nil && task.PlannedDate().Before(targetDate) {
        task.SetPlannedDate(nil)
    }

    if err := d.taskStorage.WriteTask(ctx, task); err != nil {
        return nil, errors.Wrap(ctx, err, "write task")
    }
    return task, nil
}
```

The block MUST NOT add a second `WriteTask` call. The existing single `WriteTask` call at line 147 still writes the file with both `defer_date` and (if applicable) `status` set in a single atomic write.

### 2. Update the existing default-status test in `pkg/ops/defer_test.go`

The existing `Context("with relative date +7d", ...)` (around line 61) has an `It("does not change task status", ...)` block at line 70 that asserts `Status() == domain.TaskStatusNext` (because the default `BeforeEach` creates a `{"status": "todo"}` task which normalizes to `next`). Under the new behavior, deferring this task WILL change its status to `in_progress`. Update this test:

- Rename the `It` to `"promotes status from next (todo alias) to in_progress"`.
- Replace the assertion `Expect(writtenTask.Status()).To(Equal(domain.TaskStatusNext))` with `Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))`.

All other existing tests (defer_date set, past-date validation, daily-note updates, planned_date clearing) are unaffected — they assert on different fields and continue to pass.

### 3. Add new tests in `pkg/ops/defer_test.go`

Add a new top-level `Context("status auto-promote on defer", func() { ... })` block (placement: after the existing success Contexts, before the `Context("calls FindTaskByName", ...)` block — verify by reading the file to find a good insertion point). Inside it, add the following sub-Contexts and `It(...)` blocks:

**Promote from `next` / `backlog`:**

- `Context("when current status is next", ...)` — override the default task in `BeforeEach` to `map[string]any{"status": "next"}` (or rely on the default `{"status": "todo"}` which normalizes to `next`). Add:
  - `It("writes status: in_progress", ...)` — assert `writtenTask.Status() == domain.TaskStatusInProgress` after `Execute`
  - `It("still writes defer_date", ...)` — assert `writtenTask.DeferDate() != nil` and matches the expected date (parallel to the existing `+7d` defer_date test)
  - `It("does not call WriteTask twice", ...)` — assert `mockTaskStorage.WriteTaskCallCount() == 1` (single atomic write — both fields land in the same write)

- `Context("when current status is backlog", ...)` — override the default task to `map[string]any{"status": "backlog"}`. Add:
  - `It("writes status: in_progress", ...)` — assert `writtenTask.Status() == domain.TaskStatusInProgress`
  - `It("still writes defer_date", ...)` — parallel to above
  - `It("does not call WriteTask twice", ...)` — same as above

**No-op on `in_progress` (idempotent — AC 7):**

- `Context("when current status is in_progress", ...)` — override the default task to `map[string]any{"status": "in_progress"}`. Add:
  - `It("leaves status as in_progress (idempotent)", ...)` — assert `writtenTask.Status() == domain.TaskStatusInProgress` AND assert that `writtenTask`'s raw `GetString("status")` (promoted from embedded `TaskFrontmatter`) (or equivalent byte-identical read) is exactly `"in_progress"` — the status line is NOT re-written
  - `It("still writes defer_date", ...)` — defer_date is set normally

**No-op on `completed` / `aborted` / `hold` (AC 8, spec Non-goals):**

- `Context("when current status is completed", ...)` — override to `{"status": "completed"}`. Add:
  - `It("leaves status as completed (terminal preserved)", ...)` — assert `writtenTask.Status() == domain.TaskStatusCompleted` AND the raw frontmatter `status` key is exactly `"completed"` byte-identical

- `Context("when current status is aborted", ...)` — override to `{"status": "aborted"}`. Add:
  - `It("leaves status as aborted (terminal preserved)", ...)` — parallel to completed

- `Context("when current status is hold", ...)` — override to `{"status": "hold"}`. Add:
  - `It("leaves status as hold", ...)` — parallel to completed

For the "byte-identical status line" assertions, use the `task.GetString("status")` accessor — `GetString` is promoted from the embedded `TaskFrontmatter` → `FrontmatterMap` (verify by reading `pkg/domain/frontmatter_map.go`). It returns the raw stored string, not the normalized form. This is the byte-level check the spec AC 7/8 demand.

### 4. Iterative verification

After each section of changes, run `make test` from the repo root to catch issues early. Do NOT run `make precommit` iteratively.

</requirements>

<constraints>
- The auto-promote MUST be a single in-memory mutation on the `task` parameter, followed by the EXISTING single `WriteTask` call. Do NOT add a second `WriteTask` — both `defer_date` and the new `status` must land in the same atomic file write.
- The auto-promote direction is fixed: only `next` → `in_progress` and `backlog` → `in_progress`. The inverse (e.g. deferring should NOT downgrade `in_progress` to anything) is also fixed.
- `in_progress`, `completed`, `aborted`, `hold` statuses are NEVER promoted by defer — the auto-promote block is a no-op for them. The defer write still sets `defer_date` and updates daily notes as before.
- Past-date validation in `Execute` (around line 99-112) is unchanged — deferring to yesterday still returns an error. The auto-promote must NOT fire before the past-date check (it doesn't, because the past-date check happens earlier in `Execute` and the error path returns before `findAndDeferTask` is called).
- The existing planned_date clearing logic (lines 142-145) is unchanged.
- `SetStatus` returns an error from its `Validate` call. Wrap with `errors.Wrap(ctx, err, "set status to in_progress")` to match the existing error-wrap style. If `Validate` is unreachable for `TaskStatusInProgress` (a constant), this is defense-in-depth — never hit in practice — and a future caller that misuses `SetStatus` gets a clear error.
- Tests use Ginkgo v2 / Gomega per project convention.
- `make precommit` MUST stay green from the repo root.
- Do NOT commit — dark-factory handles git.
- Coverage for the modified function MUST be ≥80% per `docs/definition-of-done.md` — both promote paths (next, backlog) and all 4 no-op paths (in_progress, completed, aborted, hold) must be covered.

</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. The auto-promote block is in findAndDeferTask
grep -n 'Calendar-as-commitment\|status is next\|set status to in_progress' pkg/ops/defer.go
# Expected: matches inside findAndDeferTask

# 2. Only ONE WriteTask call in findAndDeferTask
awk '/^func.*findAndDeferTask/,/^}$/' pkg/ops/defer.go | grep -c 'WriteTask'
# Expected: 1 (the existing call — no second write added)

# 3. The two promoted statuses are listed
awk '/^func.*findAndDeferTask/,/^}$/' pkg/ops/defer.go | grep -E 'TaskStatusNext|TaskStatusBacklog|TaskStatusInProgress'
# Expected: all three referenced

# 4. AC 13: make precommit exits 0
make precommit
# Expected: "ready to commit"
```

**Note**: End-to-end CLI fixture verification for ACs 6-9 (status-by-status promote/no-op via `vault-cli task defer` against synthetic markdown files) belongs in the spec's manual verification ladder (`/dark-factory:verify-spec` rung), NOT in this prompt's `<verification>` block. The autonomous container has no `~/.vault-cli/config.yaml` declaring a vault, and the CLI shape `vault-cli task defer <task-name> <date>` selects vault by `--vault <name>` from configured vaults — not a positional path. ACs 6-9 are still covered at the unit level by the Ginkgo tests in requirement 3 (each status × promote/no-op `It` block); the operator confirms the same behavior end-to-end at PR-verify time.

Coverage check:

```bash
go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep -E "findAndDeferTask"
# Expected: findAndDeferTask at 100% coverage
```

</verification>
