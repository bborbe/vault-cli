---
status: cancelled
spec: [013-rename-task-status-phase-taxonomy]
created: "2026-05-20T16:09:22Z"
queued: "2026-05-20T16:33:29Z"
branch: dark-factory/rename-task-status-phase-taxonomy
cancelled: "2026-05-20T16:45:05Z"
---

<summary>
- `vault-cli lint` no longer reports `status: todo` or `phase: in_progress` as invalid issues — both are silently accepted as known aliases via the updated `detectInvalidStatus` logic
- The lint error message for truly unknown status values lists `next` (new canonical) instead of `todo`
- `status: backlog` or `status: hold` combined with `phase: execution` is now correctly flagged as a status/phase mismatch (execution is an active phase)
- `update.go`'s `statusFromProgress` now returns `TaskStatusNext` for "not started" tasks instead of `TaskStatusTodo`, so newly-computed statuses use the new canonical
- `list.go`'s default filter and priority sort accept both `TaskStatusNext` and `TaskStatusTodo`, so vault files with either value appear in `vault-cli task list`
- `goal_complete.go`'s open-task guard checks both `TaskStatusNext` and `TaskStatusTodo`, so tasks with either value block a goal from completing
- All four version strings (`CHANGELOG.md`, `plugin.json`, two `marketplace.json` fields) are bumped to `0.65.0` in the same commit
- All existing tests pass; new tests assert that legacy `status: todo` and `phase: in_progress` produce zero lint issues
</summary>

<objective>
Update the ops layer to work correctly with the domain taxonomy flip from prompt 1 (spec 013). This prompt assumes `TaskStatusNext`, `TaskPhaseExecution`, and `NormalizeTaskPhase` already exist in `pkg/domain/`. It updates `lint.go`, `update.go`, `list.go`, and `goal_complete.go` to use the new canonical constants, and updates their tests accordingly. It also delivers the version-aligned release bump to `v0.65.0`.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read `docs/releasing-vault-cli.md` before touching any version file — the four-way alignment check is enforced by `make precommit`.

Read these files in full before making changes:
- `pkg/ops/lint.go` — `detectInvalidStatus` (lines ~356–375), `detectStatusPhaseMismatch` (lines ~511–564), the error message at line ~229, and `fixInvalidStatus` (lines ~682–701)
- `pkg/ops/lint_test.go` — full file (~1700 lines); read in chunks if needed (offset/limit). Key sections: "with different valid statuses" (~line 590), "with migrateable status values" (~line 889), "with old migrateable status values" (~line 946), "with edge cases in status values" (~line 1218)
- `pkg/ops/update.go` — `statusFromProgress` (~line 120)
- `pkg/ops/update_test.go` — the test at ~line 105 that asserts `writtenTask.Status() == domain.TaskStatusTodo`
- `pkg/ops/list.go` — `matchesStatusFilter` (line 184, default branch at line 194) and `statusPriority` (~line 198)
- `pkg/ops/goal_complete.go` — open-task guard at ~line 119
- `CHANGELOG.md` — top entry (currently `## v0.64.2`)
- `.claude-plugin/plugin.json` — `"version"` field (currently `"0.61.0"`)
- `.claude-plugin/marketplace.json` — both `metadata.version` and `plugins[0].version` (currently `"0.61.0"`)

Reference docs from the coding plugin:
- `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` — Ginkgo v2 / Gomega patterns
- `changelog-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` — changelog entry format
- `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` — which test types to write
</context>

<requirements>
### 1. Update `detectInvalidStatus` in `pkg/ops/lint.go`

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

This means `NormalizeTaskStatus` is the sole gate: canonical values and all known aliases (including `"todo"`, `"current"`, `"done"`, `"deferred"`) produce no issue. Only values that `NormalizeTaskStatus` cannot map (e.g. `"garbage"`) are reported as invalid — and they are not fixable, since there is no known target.

**Implication for `fixInvalidStatus`**: Since `detectInvalidStatus` now always returns `isFixable: false`, `fixInvalidStatus` will never be called from the fix loop. Do NOT remove `fixInvalidStatus` — leave it in place for now to avoid unrelated deletion churn.

### 2. Update the error message string in `pkg/ops/lint.go`

At line ~229, the `Description` in the `IssueTypeInvalidStatus` block currently reads:

```go
"status is %q, expected one of: todo, in_progress, backlog, completed, hold, aborted",
```

Change to:

```go
"status is %q, expected one of: next, in_progress, backlog, completed, hold, aborted",
```

`"todo"` is removed; `"next"` replaces it. All other values are unchanged.

### 3. Add `TaskPhaseExecution` to `activePhases` in `detectStatusPhaseMismatch`

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

### 4. Update `pkg/ops/lint_test.go`

Read the full file before editing. Key changes:

#### 4a. Update `validStatuses` literals (~lines 591, 922)

Both occurrences of:
```go
validStatuses := []string{"todo", "in_progress", "backlog", "completed", "hold", "aborted"}
```
Must change to:
```go
validStatuses := []string{"next", "in_progress", "backlog", "completed", "hold", "aborted"}
```

`"todo"` is removed from the canonical-set literal; `"next"` replaces it. These lists enumerate the canonical set for tests that say "valid statuses produce no error".

#### 4b. Update "with migrateable status values" block (~line 889)

This block currently creates a file with `status: next` and asserts it is reported as a fixable issue. After the change, `"next"` is the new canonical — it must produce zero issues.

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

#### 4c. Update "with old migrateable status values" block (~line 946)

This block has `migrateMap := map[string]string{"next": "todo", "current": "in_progress", "completed": "completed"}` and tests auto-fix behavior. After the change, all known aliases are silently accepted (no auto-fix). The `"next"` entry is now wrong (next is canonical, not migrateable).

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

#### 4d. Update "with edge cases in status values" block (~line 1218)

The `statusEdgeCases` map currently marks `"next"`, `"current"`, and `"done"` as `false` (expect issues). After the change, all known values are accepted. Change the map to:

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

Since all entries are now `true`, the `else` branch (`It("detects fixable invalid status in json mode")` and `It("fixes invalid status in json mode")`) inside the surrounding loop never fires. Delete the entire `else { ... }` block (including both `It` blocks it contained) — leaving dead test code is worse than the brief mid-flight diff.

After the edit, the loop body should contain only the `if isValid { It("accepts valid status in json mode", func() { ... }) }` branch, with no `else`.

#### 4e. Add legacy-value lint acceptance tests

Add a new `Context` block immediately before the closing of the main lint test `Describe` (before the last `})`) to satisfy the spec AC. Place it after the "with edge cases" block:

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

### 5. Update `pkg/ops/update.go`

In `statusFromProgress` (~line 127), change the final return:

```go
return domain.TaskStatusTodo
```
to:
```go
return domain.TaskStatusNext
```

This ensures newly-computed default statuses use the canonical `"next"` value.

### 6. Update `pkg/ops/update_test.go`

Find the test at ~line 105 that says:

```go
It("sets status to todo", func() {
    Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
    _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    Expect(writtenTask.Status()).To(Equal(domain.TaskStatusTodo))
})
```

Change it to:

```go
It("sets status to next", func() {
    Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
    _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    Expect(writtenTask.Status()).To(Equal(domain.TaskStatusNext))
})
```

### 7. Update `pkg/ops/list.go`

#### 7a. `matchesStatusFilter` default branch (~line 194)

In `pkg/ops/list.go`, the function `matchesStatusFilter` (defined at line 184) has a default-branch return at line 194, immediately after the comment `// Default: show only todo and in_progress`.

Currently:
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

This ensures tasks with either `status: todo` (old files) or `status: next` (new files) appear in the default `task list` output. Anchor the edit to the `// Default:` comment, not the line number — line numbers may drift.

#### 7b. `statusPriority` (~line 202)

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

Both the new canonical and the old alias get the same sort priority.

### 8. Update `pkg/ops/goal_complete.go`

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

This ensures tasks with `status: next` (new files) are treated as open when checking whether a goal can be completed.

### 8b. Update `pkg/storage/markdown_test.go`

Two assertions at lines ~120 and ~146 read a file authored with `status: todo` on disk and expect `task.Status()` to equal `domain.TaskStatusTodo`. After prompt 120's canonical flip, the read path runs `NormalizeTaskStatus("todo")` and returns `TaskStatusNext`. The assertions must change to match.

At ~line 120 (`It("reads a task successfully")`):
```go
Expect(task.Status()).To(Equal(domain.TaskStatusTodo))
```
Change to:
```go
Expect(task.Status()).To(Equal(domain.TaskStatusNext))
```

At ~line 146 (`It("reads a task with string priority as 0 (resilient parsing)")`, same file):
```go
Expect(task.Status()).To(Equal(domain.TaskStatusTodo))
```
Change to:
```go
Expect(task.Status()).To(Equal(domain.TaskStatusNext))
```

Do NOT change the on-disk `status: todo` YAML in either test. That's the alias path being exercised — the test now correctly asserts the normalize-on-read behavior.

### 9. Version-aligned release: bump to `v0.65.0`

**Read `docs/releasing-vault-cli.md` before making version changes.**

The four version strings must all equal `0.65.0`:

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

Do NOT create an `## Unreleased` section. This is a versioned release commit. The `check-versions` target in `make precommit` will fail if any of the four strings differ.
</requirements>

<constraints>
- Prompt 1 (domain changes: `TaskStatusNext`, `TaskPhaseExecution`, `NormalizeTaskPhase`) MUST be executed and merged before this prompt runs. This prompt references those symbols — they must exist in `pkg/domain/` before this code compiles.
- Do NOT touch `pkg/domain/` — all domain-layer changes are in prompt 1.
- Do NOT modify the `WorkOnOperation` interface, `NewWorkOnOperation` constructor, or any logic in `pkg/ops/workon.go` — the phase-advancement behavior was already shipped in a prior prompt (119); the existing code at `workon.go:91` compares to `domain.TaskPhaseTodo` (unchanged constant, still valid).
- `phase: todo` is unaffected by this rename — only the `status: todo` → `next` flip and `phase: in_progress` → `execution` flip are in scope. `TaskPhaseTodo` remains canonical and stays in `AvailableTaskPhases`. Do NOT "fix" any reference to `TaskPhaseTodo`.
- `TaskStatusTodo` and `TaskPhaseInProgress` constants MUST remain usable as comparison targets in `list.go`, `goal_complete.go`, and test files — they still equal `"todo"` and `"in_progress"` respectively.
- The `fixInvalidStatus` function in `lint.go` must NOT be removed (leave in place), but it will no longer be triggered from the fix loop since `detectInvalidStatus` always returns `isFixable: false`.
- All four version strings (`CHANGELOG.md`, `plugin.json`, `marketplace.json` ×2) must equal `0.65.0` — `make precommit` runs `check-versions` and fails on divergence.
- Do NOT commit — dark-factory handles git.
- Existing tests not listed above must remain green without modification.
- Follow Ginkgo v2 / Gomega style for all new test cases.
</constraints>

<verification>
```bash
# Precheck: prompt 1 must have landed (TaskStatusNext defined in domain).
grep -q 'TaskStatusNext' pkg/domain/task_status.go || { echo "ERROR: prompt 1 not landed — abort"; exit 1; }
grep -q 'TaskPhaseExecution' pkg/domain/task_phase.go || { echo "ERROR: prompt 1 not landed — abort"; exit 1; }
grep -q 'func NormalizeTaskPhase' pkg/domain/task_phase.go || { echo "ERROR: prompt 1 not landed — abort"; exit 1; }
```

```bash
make test
```

```bash
# Confirm lint error message uses next, not todo
grep -n 'expected one of:' pkg/ops/lint.go
# expected: one line containing "next" and NOT containing "todo"

# Confirm detectInvalidStatus uses NormalizeTaskStatus as the only gate
grep -n 'NormalizeTaskStatus\|IsValidTaskStatus' pkg/ops/lint.go
# expected: NormalizeTaskStatus appears in detectInvalidStatus; IsValidTaskStatus no longer used there

# Confirm activePhases includes TaskPhaseExecution
grep -n 'TaskPhaseExecution' pkg/ops/lint.go
# expected: ≥1 match in detectStatusPhaseMismatch

# Confirm statusFromProgress returns TaskStatusNext
grep -n 'TaskStatusNext\|TaskStatusTodo' pkg/ops/update.go
# expected: return domain.TaskStatusNext (not TaskStatusTodo)

# Confirm list.go handles both TaskStatusNext and TaskStatusTodo
grep -n 'TaskStatusNext' pkg/ops/list.go
# expected: ≥2 matches (shouldShowByStatus + statusPriority)

# Confirm goal_complete.go handles TaskStatusNext
grep -n 'TaskStatusNext' pkg/ops/goal_complete.go
# expected: ≥1 match

# Confirm validStatuses literals use "next" not "todo"
grep -n '"todo"' pkg/ops/lint_test.go | grep 'validStatuses'
# expected: 0 matches (todo must not appear in the canonical-set literals)

grep -n '"next"' pkg/ops/lint_test.go | grep 'validStatuses'
# expected: matches at both line 591 and line 922 contexts

# Version alignment check
grep -n 'v0.65.0\|0\.65\.0' CHANGELOG.md .claude-plugin/plugin.json .claude-plugin/marketplace.json
# expected: matches in all three files

make precommit
```
</verification>
