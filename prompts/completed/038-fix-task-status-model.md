---
status: completed
summary: Fixed TaskStatus constants, added UnmarshalYAML with alias normalization support
container: vault-cli-038-fix-task-status-model
dark-factory-version: v0.14.5
created: "2026-03-03T23:05:15Z"
queued: "2026-03-03T23:05:15Z"
started: "2026-03-03T23:05:15Z"
completed: "2026-03-03T23:10:29Z"
---
<objective>
Fix TaskStatus constants in pkg/domain/task.go. The canonical status values must match what Obsidian slash commands use. Current values `done` and `deferred` are wrong. Add missing statuses. Support aliases on read (unmarshal) but always write canonical values.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read ~/Documents/workspaces/coding/docs/go-testing-guide.md for testing patterns.
Read pkg/domain/task.go — the file to modify.
Read pkg/ops/complete.go, pkg/ops/defer.go, pkg/ops/update.go, pkg/ops/list.go — consumers of TaskStatus.
Read pkg/ops/lint.go — already validates against the correct canonical values.
Read pkg/storage/markdown.go — parseFrontmatter uses yaml.Unmarshal into Task struct.
</context>

<requirements>
1. Replace TaskStatus constants in `pkg/domain/task.go`:

   ```go
   const (
       TaskStatusTodo       TaskStatus = "todo"
       TaskStatusInProgress TaskStatus = "in_progress"
       TaskStatusCompleted  TaskStatus = "completed"
       TaskStatusBacklog    TaskStatus = "backlog"
       TaskStatusHold       TaskStatus = "hold"
       TaskStatusAborted    TaskStatus = "aborted"
   )
   ```

   Remove `TaskStatusDone` and `TaskStatusDeferred`. They no longer exist.

2. Add a `NormalizeTaskStatus` function to `pkg/domain/task.go` that maps aliases to canonical values:

   ```go
   // NormalizeTaskStatus converts alias status values to their canonical form.
   // Returns the canonical status and true if valid, or empty and false if unknown.
   func NormalizeTaskStatus(raw string) (TaskStatus, bool) {
       // canonical values
       // aliases: done→completed, current→in_progress, next→todo, deferred→hold
   }
   ```

   Alias mapping:
   - `done` → `completed`
   - `current` → `in_progress`
   - `next` → `todo`
   - `deferred` → `hold`

   All canonical values also return themselves.

3. Add `IsValidTaskStatus(status TaskStatus) bool` that returns true for all 6 canonical values.

4. Implement `UnmarshalYAML` on `TaskStatus` that normalizes on read:

   ```go
   func (s *TaskStatus) UnmarshalYAML(unmarshal func(interface{}) error) error {
       var raw string
       if err := unmarshal(&raw); err != nil {
           return err
       }
       normalized, ok := NormalizeTaskStatus(raw)
       if !ok {
           return fmt.Errorf("invalid task status: %q", raw)
       }
       *s = normalized
       return nil
   }
   ```

   This means: reading `status: done` from YAML → `TaskStatusCompleted` in memory. Writing always produces `completed`.

5. Update ALL references across the codebase:
   - `domain.TaskStatusDone` → `domain.TaskStatusCompleted` everywhere
   - Remove all references to `domain.TaskStatusDeferred`
   - `pkg/ops/list.go` statusPriority: remove `TaskStatusDeferred` case, add `TaskStatusBacklog`, `TaskStatusHold`, `TaskStatusAborted`
   - `pkg/ops/lint.go` statusMigrationMap: update to use `NormalizeTaskStatus` or align with same alias map

6. Update tests:
   - `pkg/ops/complete_test.go`: change `domain.TaskStatusDone` → `domain.TaskStatusCompleted`
   - `pkg/ops/defer_test.go`: change `domain.TaskStatusDeferred` references (see prompt 3 for defer behavior change)
   - `pkg/ops/update_test.go`: change `domain.TaskStatusDone` → `domain.TaskStatusCompleted`
   - Add unit tests for `NormalizeTaskStatus` covering all aliases and canonical values
   - Add unit tests for `IsValidTaskStatus`
   - Add unit test for `UnmarshalYAML` — verify `done` unmarshals to `completed`, `current` to `in_progress`, etc.
</requirements>

<constraints>
- Package name for tests: `domain_test` (external test package)
- Use Ginkgo v2 + Gomega for tests, follow existing test patterns in the project
- Use counterfeiter for mocks where needed (`go generate -mod=mod ./...`)
- Do NOT change the YAML tag on Task.Status — it stays `yaml:"status"`
- Do NOT modify storage layer (markdown.go) — UnmarshalYAML on TaskStatus handles normalization transparently
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Confirm:
- No references to `TaskStatusDone` or `TaskStatusDeferred` remain (except in test assertions proving alias support)
- `NormalizeTaskStatus("done")` returns `("completed", true)`
- `NormalizeTaskStatus("current")` returns `("in_progress", true)`
- `NormalizeTaskStatus("next")` returns `("todo", true)`
- `NormalizeTaskStatus("deferred")` returns `("hold", true)`
- `NormalizeTaskStatus("garbage")` returns `("", false)`
- All existing tests pass (updated for new constant names)
</verification>
