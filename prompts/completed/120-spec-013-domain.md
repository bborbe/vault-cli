---
status: completed
spec: [013-rename-task-status-phase-taxonomy]
summary: Atomically flipped canonical task status from 'todo' to 'next' and phase from 'in_progress' to 'execution' across pkg/domain/, pkg/ops/, pkg/storage/, plus version-aligned v0.65.0 release; all tests pass and make precommit exits 0.
container: vault-cli-exec-120-spec-013-domain
dark-factory-version: v0.162.0
created: "2026-05-20T16:09:22Z"
queued: "2026-05-20T16:33:29Z"
started: "2026-05-20T16:45:10Z"
completed: "2026-05-20T16:57:34Z"
branch: dark-factory/rename-task-status-phase-taxonomy
---

<summary>
- **Atomic merge of original prompts 120 (domain) and 121 (ops + release).** Splitting them broke precommit because the canonical flip in `pkg/domain/` immediately fails dependent tests in `pkg/ops/` (23) and `pkg/storage/` (2). All changes must land in one commit to keep precommit green.
- `TaskStatusNext = "next"` becomes the canonical status; `TaskStatusTodo = "todo"` stays as an alias-only constant
- `AvailableTaskStatuses` drops `TaskStatusTodo` and adds `TaskStatusNext`; `TaskStatus("todo").Validate()` now returns an error; `TaskStatus("next").Validate()` returns nil
- `NormalizeTaskStatus` is updated so `"todo"` maps to `TaskStatusNext`; all other existing alias mappings are preserved
- `TaskPhaseExecution = "execution"` becomes the canonical phase; `TaskPhaseInProgress = "in_progress"` stays as an alias-only constant
- `AvailableTaskPhases` drops `TaskPhaseInProgress` and adds `TaskPhaseExecution`
- `NormalizeTaskPhase` and `IsValidTaskPhase` are added, mirroring the status helpers
- `vault-cli lint` accepts old canonical status/phase aliases silently (no longer flags `status: todo` or `phase: in_progress` as fixable issues); error message lists `next` not `todo`
- `update.go`'s `statusFromProgress` returns `TaskStatusNext` for "not started" tasks
- `list.go`'s default filter and priority sort, and `goal_complete.go`'s open-task guard, accept both `TaskStatusNext` and `TaskStatusTodo`
- Storage tests' `task.Status()` assertions updated to expect `TaskStatusNext` (the normalize-on-read result)
- All four version strings (`CHANGELOG.md`, `plugin.json`, two `marketplace.json` fields) bumped to `0.65.0` atomically
- All tests pass; new tests assert legacy `status: todo` and `phase: in_progress` produce zero lint issues
</summary>

<objective>
Atomically deliver the canonical task taxonomy flip (`status: todo` → `next`, `phase: in_progress` → `execution`) across `pkg/domain/`, `pkg/ops/`, and `pkg/storage/` test assertions, plus the version-aligned `v0.65.0` release. Old values remain accepted aliases via `NormalizeTaskStatus` / `NormalizeTaskPhase`; existing vault files on disk are untouched. `make precommit` must exit 0 at the end.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read `docs/releasing-vault-cli.md` before touching any version file — the four-way alignment check is enforced by `make precommit`.

Read these files in full before making changes:
- `pkg/domain/task_status.go` — current constants, `AvailableTaskStatuses`, `NormalizeTaskStatus`. The existing Normalize map has `"next": TaskStatusTodo`; this must be reversed so `"todo"` maps to `TaskStatusNext`.
- `pkg/domain/task_phase.go` — current constants, `AvailableTaskPhases`. No `NormalizeTaskPhase` exists yet. Pattern it exactly after `NormalizeTaskStatus` in `task_status.go`.
- `pkg/domain/task_status_test.go` — full file; every assertion referencing `TaskStatusTodo` in a canonical context must be updated.
- `pkg/domain/task_phase_test.go` — full file; the `DescribeTable("valid phases")` entry for `"in_progress"` must be replaced; a new `NormalizeTaskPhase` describe block must be added.
- `pkg/ops/lint.go` — `detectInvalidStatus` (lines ~356–375), `detectStatusPhaseMismatch` (lines ~511–564), the error message at line ~229, and `fixInvalidStatus` (lines ~682–701)
- `pkg/ops/lint_test.go` — full file (~1700 lines); read in chunks. Key sections: "with different valid statuses" (~line 590), "with migrateable status values" (~line 889), "with old migrateable status values" (~line 946), "with edge cases in status values" (~line 1218)
- `pkg/ops/update.go` — `statusFromProgress` (~line 120)
- `pkg/ops/update_test.go` — the test at ~line 105 that asserts `writtenTask.Status() == domain.TaskStatusTodo`
- `pkg/ops/list.go` — `matchesStatusFilter` (line 184, default branch at line 194) and `statusPriority` (~line 198)
- `pkg/ops/goal_complete.go` — open-task guard at ~line 119
- `pkg/storage/markdown_test.go` — two assertions at lines ~120 and ~146 expecting `domain.TaskStatusTodo`
- `CHANGELOG.md` — top entry (currently `## v0.64.2`)
- `.claude-plugin/plugin.json` — `"version"` field (currently `"0.61.0"`)
- `.claude-plugin/marketplace.json` — both `metadata.version` and `plugins[0].version` (currently `"0.61.0"`)

Reference docs from the coding plugin:
- `go-enum-type-pattern.md` in `~/.claude/plugins/marketplaces/coding/docs/` — enum shape: `Available*`, `Validate()`, `Contains()`, plural type
- `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` — Ginkgo v2 / Gomega conventions
- `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` — which test types to write
</context>

<requirements>
### 1. Update `pkg/domain/task_status.go`

#### 1a. Add `TaskStatusNext` constant and demote `TaskStatusTodo`

In the `const` block, immediately BEFORE `TaskStatusTodo`, add:

```go
// TaskStatusNext means the task is queued for action but not yet started.
// This is the canonical value; "todo" is accepted as an alias via NormalizeTaskStatus.
TaskStatusNext TaskStatus = "next"
```

Change the doc comment on `TaskStatusTodo` to mark it as alias-only:

```go
// TaskStatusTodo is an alias for TaskStatusNext kept for backward compatibility.
// Existing vault files with status: "todo" continue to read and validate via NormalizeTaskStatus.
// Do not use TaskStatusTodo for new writes — use TaskStatusNext.
TaskStatusTodo TaskStatus = "todo"
```

#### 1b. Update `AvailableTaskStatuses`

Replace `TaskStatusTodo` with `TaskStatusNext` in the slice literal:

```go
var AvailableTaskStatuses = TaskStatuses{
    TaskStatusNext,
    TaskStatusInProgress,
    TaskStatusBacklog,
    TaskStatusCompleted,
    TaskStatusHold,
    TaskStatusAborted,
}
```

`TaskStatusTodo` must NOT appear in this slice. `TaskStatus("todo").Validate()` must return an error after this change.

#### 1c. Update `NormalizeTaskStatus`

The existing migration map has `"next": TaskStatusTodo`. Replace the entire map with the updated version:

```go
migrationMap := map[string]TaskStatus{
    "todo":     TaskStatusNext,    // old canonical is now an alias
    "current":  TaskStatusInProgress,
    "done":     TaskStatusCompleted,
    "deferred": TaskStatusHold,
}
```

Note: DO NOT add a `"next": TaskStatusNext` identity entry. The existing early-return `if IsValidTaskStatus(status) { return status, true }` at the top of the function handles `"next"` cleanly via `AvailableTaskStatuses` (per §1b). An identity entry in the map would be dead code.

### 2. Update `pkg/domain/task_phase.go`

#### 2a. Add `TaskPhaseExecution` constant and demote `TaskPhaseInProgress`

In the `const` block, immediately BEFORE `TaskPhaseInProgress`, add:

```go
// TaskPhaseExecution means active implementation is underway.
// This is the canonical value; "in_progress" is accepted as an alias via NormalizeTaskPhase.
TaskPhaseExecution TaskPhase = "execution"
```

Change the doc comment on `TaskPhaseInProgress` to mark it as alias-only:

```go
// TaskPhaseInProgress is an alias for TaskPhaseExecution kept for backward compatibility.
// Existing vault files with phase: "in_progress" continue to read and validate via NormalizeTaskPhase.
// Do not use TaskPhaseInProgress for new writes — use TaskPhaseExecution.
TaskPhaseInProgress TaskPhase = "in_progress"
```

#### 2b. Update `AvailableTaskPhases`

Replace `TaskPhaseInProgress` with `TaskPhaseExecution`:

```go
var AvailableTaskPhases = TaskPhases{
    TaskPhaseTodo,
    TaskPhasePlanning,
    TaskPhaseExecution,
    TaskPhaseAIReview,
    TaskPhaseHumanReview,
    TaskPhaseDone,
}
```

`TaskPhaseInProgress` must NOT appear in this slice.

#### 2c. Add `IsValidTaskPhase` helper

In `pkg/domain/task_phase.go`, immediately after the existing `Ptr()` method on `TaskPhase`, add:

```go
// IsValidTaskPhase returns true if the phase is a valid canonical phase value.
func IsValidTaskPhase(phase TaskPhase) bool {
    return AvailableTaskPhases.Contains(phase)
}
```

#### 2d. Add `NormalizeTaskPhase` function

In `pkg/domain/task_phase.go`, immediately after the new `IsValidTaskPhase` function from §2c, add:

```go
// NormalizeTaskPhase converts alias phase values to their canonical form.
// Returns the canonical phase and true if valid, or empty and false if unknown.
func NormalizeTaskPhase(raw string) (TaskPhase, bool) {
    // Check if already valid canonical phase
    phase := TaskPhase(raw)
    if IsValidTaskPhase(phase) {
        return phase, true
    }

    // Migration map for legacy/alias phase values
    migrationMap := map[string]TaskPhase{
        "in_progress": TaskPhaseExecution,
    }

    if canonical, ok := migrationMap[raw]; ok {
        return canonical, true
    }

    return "", false
}
```

### 3. Update `pkg/domain/task_status_test.go`

The following existing test cases must be changed. Do NOT add new describe blocks — edit the existing ones in place.

#### 3a. `Describe("String")` — no changes needed (TaskStatusTodo.String() still returns "todo")

#### 3b. `Describe("Validate")` — update two cases, add one

- Change `It("returns nil for todo")` to:
```go
It("rejects 'todo' as canonical", func() {
    Expect(domain.TaskStatusTodo.Validate(ctx)).NotTo(BeNil())
})
```

- Add a new `It` immediately after:
```go
It("accepts 'next' as canonical", func() {
    Expect(domain.TaskStatusNext.Validate(ctx)).To(BeNil())
})
```

Note: these exact `It` names are required — spec AC #2 evidence greps for `It("rejects 'todo' as canonical")` and `It("accepts 'next' as canonical")`.

- Leave all other `It` cases untouched.

#### 3c. `Describe("NormalizeTaskStatus")` — restructure canonical/alias buckets

`"next"` is now canonical (in `AvailableTaskStatuses`), `"todo"` is now alias. Restructure the two existing tests by moving + rewriting them; do NOT keep duplicates.

**Step 1 — Delete** the existing `It("returns todo unchanged")` block from `Context("canonical values")`.

**Step 2 — Delete** the existing `It("normalizes next to todo")` block from `Context("alias values")`.

**Step 3 — In `Context("canonical values")`, add** a single new test for the new canonical:
```go
It("returns next unchanged", func() {
    status, ok := domain.NormalizeTaskStatus("next")
    Expect(ok).To(BeTrue())
    Expect(status).To(Equal(domain.TaskStatusNext))
})
```

**Step 4 — In `Context("alias values")`, add** a single new test for the demoted alias:
```go
It("normalizes todo to next", func() {
    status, ok := domain.NormalizeTaskStatus("todo")
    Expect(ok).To(BeTrue())
    Expect(status).To(Equal(domain.TaskStatusNext))
})
```

Final state: `Context("canonical values")` has `It("returns next unchanged")`; `Context("alias values")` has `It("normalizes todo to next")`. No `It("returns todo unchanged")` or `It("normalizes next to todo")` should remain anywhere in the file.

#### 3d. `Describe("IsValidTaskStatus")` — update one case, add one

- Change `It("returns true for todo")` to:
```go
It("returns false for todo (alias-only)", func() {
    Expect(domain.IsValidTaskStatus(domain.TaskStatusTodo)).To(BeFalse())
})
```

- Add immediately after:
```go
It("returns true for next", func() {
    Expect(domain.IsValidTaskStatus(domain.TaskStatusNext)).To(BeTrue())
})
```

### 4. Update `pkg/domain/task_phase_test.go`

#### 4a. `DescribeTable("valid phases")` — replace `in_progress` entry with `execution`

Change:
```go
Entry("in_progress", domain.TaskPhaseInProgress),
```
to:
```go
Entry("execution", domain.TaskPhaseExecution),
```

`domain.TaskPhaseInProgress` must NOT appear in the valid-phases table — it is now alias-only.

#### 4b. `Describe("String")` — no changes needed (TaskPhaseInProgress.String() still returns "in_progress")

#### 4c. `Describe("AvailableTaskPhases.Contains")` — add two assertions

In `It("returns true for valid phases")`, add:
```go
Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhaseExecution)).To(BeTrue())
```

Add a NEW `It` immediately after the existing valid/invalid blocks (do NOT fold this into the invalid-phases test — `TaskPhaseInProgress` is not invalid, it is alias-only):
```go
It("excludes alias phases from canonical set", func() {
    Expect(domain.AvailableTaskPhases.Contains(domain.TaskPhaseInProgress)).To(BeFalse())
})
```

#### 4d. Add `Describe("NormalizeTaskPhase")` block

Add a new top-level `Describe("NormalizeTaskPhase", ...)` block after the existing `Describe("TaskPhase", ...)` block (before the closing `)`):

```go
var _ = Describe("NormalizeTaskPhase", func() {
    var ctx context.Context
    BeforeEach(func() {
        ctx = context.Background()
        _ = ctx
    })

    Context("canonical values round-trip", func() {
        DescribeTable("returns the canonical value unchanged",
            func(raw string, expected domain.TaskPhase) {
                phase, ok := domain.NormalizeTaskPhase(raw)
                Expect(ok).To(BeTrue())
                Expect(phase).To(Equal(expected))
            },
            Entry("todo", "todo", domain.TaskPhaseTodo),
            Entry("planning", "planning", domain.TaskPhasePlanning),
            Entry("execution", "execution", domain.TaskPhaseExecution),
            Entry("ai_review", "ai_review", domain.TaskPhaseAIReview),
            Entry("human_review", "human_review", domain.TaskPhaseHumanReview),
            Entry("done", "done", domain.TaskPhaseDone),
        )
    })

    Context("alias values", func() {
        It("normalizes in_progress to execution", func() {
            phase, ok := domain.NormalizeTaskPhase("in_progress")
            Expect(ok).To(BeTrue())
            Expect(phase).To(Equal(domain.TaskPhaseExecution))
        })
    })

    Context("invalid values", func() {
        It("returns false for garbage", func() {
            phase, ok := domain.NormalizeTaskPhase("garbage")
            Expect(ok).To(BeFalse())
            Expect(phase).To(Equal(domain.TaskPhase("")))
        })

        It("returns false for empty string", func() {
            phase, ok := domain.NormalizeTaskPhase("")
            Expect(ok).To(BeFalse())
            Expect(phase).To(Equal(domain.TaskPhase("")))
        })
    })
})
```

#### 4e. YAML marshal test — keep using `TaskPhaseInProgress`

The existing YAML marshal/unmarshal test uses `domain.TaskPhaseInProgress` and asserts `ContainSubstring("in_progress")`. Do NOT change this test — the constant's string value is still `"in_progress"` and the test continues to pass. It documents that the alias constant remains functional for any caller that still uses it.

**Do NOT add a `phase.Validate(ctx)` call to this test.** `TaskPhaseInProgress` now fails `Validate` directly (it is alias-only). The marshal test exercises serialization only and must not be "fixed" by adding validation that would now fail.

### 5. Update `detectInvalidStatus` in `pkg/ops/lint.go`

The current implementation calls `domain.IsValidTaskStatus` first, then uses `NormalizeTaskStatus` only to determine `isFixable`. After the domain change, `"todo"` is no longer in `AvailableTaskStatuses`, so `IsValidTaskStatus("todo")` returns false, and the current code would report `"todo"` as a fixable issue. The spec requires that `"todo"` (and all known aliases) be silently accepted.

Replace the entire `detectInvalidStatus` body with the normalize-first approach:

```go
func (l *lintOperation) detectInvalidStatus(frontmatterYAML string) (bool, string, bool) {
    statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
    matches := statusRegex.FindStringSubmatch(frontmatterYAML)
    if len(matches) >= 2 {
        statusValue := matches[1]
        _, ok := domain.NormalizeTaskStatus(statusValue)
        if ok {
            return false, "", false // canonical or known alias — accepted silently
        }
        return true, statusValue, false // truly unknown, not fixable
    }
    return false, "", false
}
```

`NormalizeTaskStatus` is the sole gate. Only values it cannot map (e.g. `"garbage"`) are reported as invalid — and they are not fixable.

**Implication for `fixInvalidStatus`**: Since `detectInvalidStatus` now always returns `isFixable: false`, `fixInvalidStatus` will never be called from the fix loop. Do NOT remove `fixInvalidStatus` — leave it in place to avoid unrelated deletion churn.

### 6. Update the lint error message in `pkg/ops/lint.go`

At line ~229, the `Description` in the `IssueTypeInvalidStatus` block currently reads:

```go
"status is %q, expected one of: todo, in_progress, backlog, completed, hold, aborted",
```

Change to:

```go
"status is %q, expected one of: next, in_progress, backlog, completed, hold, aborted",
```

Canonical-only. Aliases are not surfaced in user-facing error text (they fail `Validate`).

### 7. Add `TaskPhaseExecution` to `activePhases` in `detectStatusPhaseMismatch`

In `detectStatusPhaseMismatch` (~line 546), the `activePhases` slice currently is:

```go
activePhases := []domain.TaskPhase{
    domain.TaskPhaseInProgress,
    domain.TaskPhaseAIReview,
    domain.TaskPhaseHumanReview,
}
```

Add `TaskPhaseExecution` as the first entry (new canonical before its alias):

```go
activePhases := []domain.TaskPhase{
    domain.TaskPhaseExecution,
    domain.TaskPhaseInProgress,
    domain.TaskPhaseAIReview,
    domain.TaskPhaseHumanReview,
}
```

This ensures `status: backlog` + `phase: execution` is correctly flagged as a mismatch.

### 8. Update `pkg/ops/lint_test.go`

#### 8a. Update `validStatuses` literals (~lines 591, 922)

Anchor by the literal:
```go
validStatuses := []string{"todo", "in_progress", "backlog", "completed", "hold", "aborted"}
```
Change BOTH occurrences to:
```go
validStatuses := []string{"next", "in_progress", "backlog", "completed", "hold", "aborted"}
```

#### 8b. Replace `Context("with migrateable status values"`

Replace the entire `Context("with migrateable status values", ...)` block with:

```go
Context("with 'next' status (new canonical)", func() {
    BeforeEach(func() {
        content := `---
status: next
priority: 1
task_identifier: test-uuid-next
---
# Next Status Task
`
        taskPath := filepath.Join(vaultPath, tasksDir, "Next.md")
        Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())
    })

    It("reports no IssueTypeInvalidStatus for status: next", func() {
        issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
        Expect(err).To(BeNil())
        Expect(issues).NotTo(BeNil(), "Execute must return a non-nil issues slice (guards against no-op)")
        for _, issue := range issues {
            Expect(issue.IssueType).NotTo(Equal(ops.IssueTypeInvalidStatus),
                "status: next must not produce an invalid status issue")
        }
    })
})
```

#### 8c. Replace `Context("with old migrateable status values"`

Replace the entire `Context("with old migrateable status values", ...)` block with:

```go
Context("with legacy alias status values (silently accepted)", func() {
    DescribeTable("produces no invalid status issue for known aliases",
        func(status string) {
            content := "---\nstatus: " + status + "\npriority: 1\n---\n# Task\n"
            taskPath := filepath.Join(vaultPath, tasksDir, "Alias.md")
            Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

            issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
            Expect(err).To(BeNil())
            for _, issue := range issues {
                Expect(issue.IssueType).NotTo(Equal(ops.IssueTypeInvalidStatus),
                    "alias status %q must not produce an invalid status issue", status)
            }
        },
        Entry("todo (old canonical, now alias)", "todo"),
        Entry("current (alias for in_progress)", "current"),
        Entry("done (alias for completed)", "done"),
        Entry("deferred (alias for hold)", "deferred"),
    )
})
```

#### 8d. Update `statusEdgeCases` block (~line 1218)

Anchor by the `statusEdgeCases := map[string]bool{` literal. Change the map to:

```go
statusEdgeCases := map[string]bool{
    "next":        true, // canonical
    "todo":        true, // accepted alias
    "in_progress": true, // canonical
    "backlog":     true, // canonical
    "completed":   true, // canonical
    "hold":        true, // canonical
    "aborted":     true, // canonical
    "current":     true, // accepted alias
    "done":        true, // accepted alias
}
```

Since all entries are now `true`, the `else { ... }` branch inside the surrounding loop (which contained `It("detects fixable invalid status in json mode")` and `It("fixes invalid status in json mode")`) never fires. **Delete the entire `else { ... }` block** including both `It` blocks it contained. After the edit, the loop body should contain only the `if isValid { It("accepts valid status in json mode", func() { ... }) }` branch, with no `else`.

#### 8e. Add legacy-value lint acceptance tests

Add a new `Context` block immediately before the closing of the main lint test `Describe` (before the last `})`):

```go
Context("with legacy status and phase values on disk", func() {
    It("accepts legacy 'todo' status with zero IssueTypeInvalidStatus issues", func() {
        content := `---
status: todo
priority: 1
task_identifier: test-uuid-legacy-status
---
# Legacy Status Task
`
        taskPath := filepath.Join(vaultPath, tasksDir, "LegacyStatus.md")
        Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

        issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
        Expect(err).To(BeNil())
        for _, issue := range issues {
            Expect(issue.IssueType).NotTo(Equal(ops.IssueTypeInvalidStatus),
                "status: todo must produce zero IssueTypeInvalidStatus issues")
        }
    })

    It("accepts legacy 'phase: in_progress' with zero IssueTypeStatusPhaseMismatch issues (when status is compatible)", func() {
        content := `---
status: in_progress
phase: in_progress
priority: 1
task_identifier: test-uuid-legacy-phase
---
# Legacy Phase Task
`
        taskPath := filepath.Join(vaultPath, tasksDir, "LegacyPhase.md")
        Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

        issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
        Expect(err).To(BeNil())
        for _, issue := range issues {
            Expect(issue.IssueType).NotTo(Equal(ops.IssueTypeStatusPhaseMismatch),
                "status: in_progress + phase: in_progress must produce zero mismatch issues")
        }
    })
})
```

### 9. Update `pkg/ops/update.go`

In `statusFromProgress` (~line 127), change the final return:

```go
return domain.TaskStatusTodo
```
to:
```go
return domain.TaskStatusNext
```

### 10. Update `pkg/ops/update_test.go`

Find the test at ~line 105:

```go
It("sets status to todo", func() {
    Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
    _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    Expect(writtenTask.Status()).To(Equal(domain.TaskStatusTodo))
})
```

Change to:

```go
It("sets status to next", func() {
    Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
    _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    Expect(writtenTask.Status()).To(Equal(domain.TaskStatusNext))
})
```

### 11. Update `pkg/ops/list.go`

#### 11a. `matchesStatusFilter` default branch

Anchor to the comment `// Default: show only todo and in_progress` (line ~193). Currently:
```go
// Default: show only todo and in_progress
return status == domain.TaskStatusTodo || status == domain.TaskStatusInProgress
```

Change both the comment and the return:
```go
// Default: show only next/todo (alias) and in_progress
return status == domain.TaskStatusNext ||
    status == domain.TaskStatusTodo ||
    status == domain.TaskStatusInProgress
```

#### 11b. `statusPriority`

Currently:
```go
case domain.TaskStatusTodo:
    return 2
```

Change to:
```go
case domain.TaskStatusNext, domain.TaskStatusTodo:
    return 2
```

### 12. Update `pkg/ops/goal_complete.go`

At ~line 119:
```go
if task.Status() == domain.TaskStatusTodo || task.Status() == domain.TaskStatusInProgress {
```

Change to:
```go
if task.Status() == domain.TaskStatusNext ||
    task.Status() == domain.TaskStatusTodo ||
    task.Status() == domain.TaskStatusInProgress {
```

### 13. Update `pkg/storage/markdown_test.go`

Two assertions at lines ~120 and ~146 read a file authored with `status: todo` on disk and expect `task.Status()` to equal `domain.TaskStatusTodo`. After the canonical flip, the read path runs `NormalizeTaskStatus("todo")` and returns `TaskStatusNext`.

At ~line 120 (`It("reads a task successfully")`) and ~line 146 (`It("reads a task with string priority as 0 (resilient parsing)")`), change BOTH occurrences of:
```go
Expect(task.Status()).To(Equal(domain.TaskStatusTodo))
```
to:
```go
Expect(task.Status()).To(Equal(domain.TaskStatusNext))
```

Do NOT change the on-disk `status: todo` YAML in either test — that's the alias-on-read path being exercised.

### 14. Version-aligned release: bump to `v0.65.0`

**Read `docs/releasing-vault-cli.md` before making version changes.**

All four version strings must equal `0.65.0`:

1. `CHANGELOG.md`: Add a new section at the top, above `## v0.64.2`:

```markdown
## v0.65.0

- feat: Rename canonical task status `todo` → `next` and phase `in_progress` → `execution` to eliminate status/phase name collision. Old values (`todo`, `in_progress`) remain accepted aliases via `NormalizeTaskStatus` / `NormalizeTaskPhase` — existing vault files are untouched on disk.
- feat: Add `TaskStatusNext`, `TaskPhaseExecution`, `IsValidTaskPhase`, and `NormalizeTaskPhase` to `pkg/domain/`
- refactor: `vault-cli lint` accepts old canonical status/phase aliases silently (no longer flags `status: todo` or `phase: in_progress` as fixable issues)
- refactor: `statusFromProgress` emits `next` instead of `todo` for newly-computed default statuses
```

2. `.claude-plugin/plugin.json`: Change `"version": "0.61.0"` to `"version": "0.65.0"`.

3. `.claude-plugin/marketplace.json`: Change both `"version": "0.61.0"` occurrences to `"version": "0.65.0"` — one in `metadata` and one in `plugins[0]`.

Do NOT create an `## Unreleased` section. The `check-versions` target in `make precommit` fails if any of the four strings differ.
</requirements>

<constraints>
- **Atomic: all domain + ops + storage + release changes must land in one commit so `make precommit` exits 0 at the end.** This prompt is a merged version of the original 120 (domain) + 121 (ops + release) split — the split was unworkable because the canonical flip in `pkg/domain/` immediately fails dependent tests in `pkg/ops/` and `pkg/storage/`.
- `TaskStatusTodo` and `TaskPhaseInProgress` Go constants MUST remain exported with their original string values (`"todo"` and `"in_progress"`). Do NOT remove or rename them — external Go consumers and existing internal references must keep compiling.
- `AvailableTaskStatuses` must NOT contain `TaskStatusTodo`. `AvailableTaskPhases` must NOT contain `TaskPhaseInProgress`.
- `NormalizeTaskStatus("todo")` must return `(TaskStatusNext, true)`. `NormalizeTaskStatus("next")` must return `(TaskStatusNext, true)` via the canonical-set early return.
- `NormalizeTaskPhase("in_progress")` must return `(TaskPhaseExecution, true)`. `NormalizeTaskPhase("execution")` must return `(TaskPhaseExecution, true)`.
- Do NOT modify the `WorkOnOperation` interface, `NewWorkOnOperation` constructor, or any logic in `pkg/ops/workon.go` — the phase-advancement behavior was already shipped in prompt 119; the existing code at `workon.go:91` compares to `domain.TaskPhaseTodo` (unchanged constant, still valid).
- `phase: todo` is unaffected by this rename — only the `status: todo` → `next` flip and `phase: in_progress` → `execution` flip are in scope. `TaskPhaseTodo` remains canonical and stays in `AvailableTaskPhases`. Do NOT "fix" any reference to `TaskPhaseTodo`.
- `fixInvalidStatus` in `lint.go` must NOT be removed (leave in place), but it will no longer be triggered from the fix loop since `detectInvalidStatus` always returns `isFixable: false`.
- Existing tests that reference `TaskStatusTodo` or `TaskPhaseInProgress` in a NON-canonical context (the YAML marshal test, the alias-on-read paths in `markdown_test.go`) must NOT be "fixed" by adding `Validate()` calls — those constants now fail `Validate` directly. Update only the assertions noted in §13.
- All four version strings (`CHANGELOG.md`, `plugin.json`, `marketplace.json` ×2) must equal `0.65.0` — `make precommit` runs `check-versions` and fails on divergence.
- Do NOT commit — dark-factory handles git.
- Follow Ginkgo v2 / Gomega style for all new test cases.
- Error wrapping for `Validate` in `task_status.go` currently uses `fmt.Errorf`; leave it unchanged. The phase `Validate` uses `errors.Wrapf` — also leave unchanged.
</constraints>

<verification>
```bash
# Domain
grep -n 'TaskStatusNext' pkg/domain/task_status.go
# expected: ≥2 matches (constant declaration + AvailableTaskStatuses)

grep -n 'TaskPhaseExecution' pkg/domain/task_phase.go
# expected: ≥2 matches (constant declaration + AvailableTaskPhases)

grep -n 'TaskStatusTodo TaskStatus = "todo"' pkg/domain/task_status.go
# expected: 1 match (alias constant preserved)

grep -n 'TaskPhaseInProgress TaskPhase = "in_progress"' pkg/domain/task_phase.go
# expected: 1 match (alias constant preserved)

grep -n 'TaskStatusTodo,' pkg/domain/task_status.go
# expected: 0 matches (removed from canonical set)

grep -n 'TaskPhaseInProgress,' pkg/domain/task_phase.go
# expected: 0 matches (removed from canonical set)

grep -n 'func NormalizeTaskPhase' pkg/domain/task_phase.go
# expected: 1 match

# Ops layer
grep -n 'expected one of:' pkg/ops/lint.go
# expected: one line containing "next" and NOT containing "todo"

grep -n 'TaskPhaseExecution' pkg/ops/lint.go
# expected: ≥1 match in detectStatusPhaseMismatch

grep -n 'TaskStatusNext' pkg/ops/update.go pkg/ops/list.go pkg/ops/goal_complete.go
# expected: matches in all three files

# Lint test literals
grep -n '"todo"' pkg/ops/lint_test.go | grep 'validStatuses'
# expected: 0 matches

# Version alignment
grep -n '0\.65\.0' CHANGELOG.md .claude-plugin/plugin.json .claude-plugin/marketplace.json
# expected: matches in all three files; marketplace.json has 2 matches

# Full build + test gate
make precommit
# expected: exit 0
```
</verification>
