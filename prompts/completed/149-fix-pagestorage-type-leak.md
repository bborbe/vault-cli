---
status: completed
summary: Introduced domain.Page type; PageStorage.ListPages now returns []*domain.Page instead of []*domain.Task, eliminating type contract violation for non-task entities
execution_id: vault-cli-exec-149-fix-pagestorage-type-leak
dark-factory-version: v0.188.1
created: "2026-06-28T22:00:00Z"
queued: "2026-06-28T20:59:31Z"
started: "2026-06-28T21:00:23Z"
completed: "2026-06-28T21:07:04Z"
---

<summary>
- `PageStorage.ListPages` returns `[]*domain.Task` but is used for all entity types (goals, themes, objectives, visions)
- `ops/list.go` calls `task.Status()` on non-task entities — returns empty/zero values silently
- Create a new shared `Page` type in `pkg/domain/` that wraps `FrontmatterMap` + `FileMetadata`
- `Page` exposes `Status()`, `Name()`, `Assignee()`, `Priority()`, `Phase()` etc. via generic frontmatter accessors
- `ListPages` returns `[]*domain.Page` instead of `[]*domain.Task`
- `ops/list.go` uses `*domain.Page` — no type contract violation for non-task entities
- All entity types (Task, Goal, Theme, Objective, Vision) can convert to `Page` without losing fields
- Existing task-specific list behavior (date fields, goals, claude_session_id) is preserved
</summary>

<objective>
Eliminate the type contract violation where `PageStorage.ListPages` returns `[]*domain.Task` for non-task entities, by introducing a shared `domain.Page` type.
</objective>

<context>
Read:
- `pkg/storage/storage.go` — `PageStorage` interface at line 92 returns `[]*domain.Task`
- `pkg/storage/page.go` — `ListPages` implementation at line 30
- `pkg/domain/task.go` — `Task` struct embeds `TaskFrontmatter` + `FileMetadata`
- `pkg/domain/frontmatter_map.go` — generic map accessors (`GetField`, `SetField`, `Keys`)
- `pkg/domain/task_frontmatter.go` — typed field accessors on `TaskFrontmatter`
- `pkg/domain/goal.go`, `theme.go`, `objective.go`, `vision.go` — entity structs
- `pkg/domain/file_metadata.go` — `FileMetadata` with `Name`, `FilePath`, `ModifiedDate`
- `pkg/ops/list.go` — `Execute` iterates `[]*domain.Task` and accesses typed fields
- `pkg/ops/frontmatter_entity.go` — `entityShowOperation` switches on concrete types (line ~650)
- Existing tests: `pkg/ops/list_test.go`, `pkg/storage/export_test.go`
</context>

<requirements>
1. **Create `pkg/domain/page.go`** with a `Page` struct that embeds `FrontmatterMap` + `FileMetadata` + has a `Content Content` field (matching the pattern in `Task` at `pkg/domain/task.go:10-16`):
   - `Page` type with constructor `NewPage(data map[string]any, meta FileMetadata, content Content)`
   - Methods on `Page` mirroring the common accessors used by `list.go`: `Status()`, `Name()`, `Assignee()`, `Priority()`, `Phase()`, `PageType()`, `Recurring()`, `Goals()` — each reads from the frontmatter map via `GetField` + typed conversion
   - Date accessors: `DeferDate()`, `PlannedDate()`, `DueDate()`, `CompletedDate()` — parse from frontmatter map
   - `ClaudeSessionID()` accessor
   - `ModifiedDate` field (from `FileMetadata`)

2. **Update `pkg/storage/storage.go`** `PageStorage` interface:
   - Change `ListPages` return type from `[]*domain.Task` to `[]*domain.Page`

3. **Update `pkg/storage/page.go`** `ListPages` implementation:
   - Add a `readPageFromPath` method (parallel to `baseStorage.readTaskFromPath` at `pkg/storage/base.go:164`) that returns `*domain.Page` instead of `*domain.Task`
   - `readPageFromPath` calls `domain.NewPage(data, meta, content)` instead of `domain.NewTask(data, meta, content)` — same frontmatter parsing, same symlink check
   - `ListPages` calls `p.readPageFromPath` instead of `p.readTaskFromPath`

4. **Update `pkg/ops/list.go`**:
   - Change `Execute` parameter types from `[]*domain.Task` to `[]*domain.Page`
   - `TaskListItem` fields remain the same
   - Access `name` from `page.Name` (FileMetadata field), not `task.Name`
   - Update `filterTasks`, `shouldIncludeTask`, `taskHasGoal`, `matchesStatusFilter` signatures to accept `*domain.Page`
   - Import `libtime` for date parsing from `github.com/bborbe/time`

5. **Update `ops/frontmatter_entity.go`**:
   - `entityShowOperation` at ~line 650 switches on concrete types — if it uses `ListPages`, update to `*domain.Page`

6. **Check and update all callers** of `PageStorage.ListPages`:
   - Grep for `ListPages` across the codebase
   - Update any type assertions or casts expecting `*domain.Task` from list results

7. **Update test files** — `pkg/ops/list_test.go` and `pkg/storage/markdown_test.go`:
   - `list_test.go` uses `[]*domain.Task` created via `domain.NewTask(...)` throughout (~40 call sites)
   - Change to `[]*domain.Page` created via `domain.NewPage(...)` with the same frontmatter data
   - The mock `PageStorage` returns `[]*domain.Page` — update `ListPagesReturns` calls accordingly
   - `markdown_test.go` (~line 627) accesses `pages[0].Name` and `.ModifiedDate` — these work on both types but test variable types must change

8. **Regenerate mocks** after the interface change:
   - `make generate` (part of `make precommit`) regenerates `mocks/page-storage.go` and `mocks/storage.go`
   - Verify the mock signatures accept/return `*domain.Page` not `*domain.Task`

9. **Existing tests must still pass** — run `make precommit`

8. **Do NOT change** the entity-specific storage types (`TaskStorage`, `GoalStorage`, etc. — they keep returning their concrete types). Only `PageStorage` changes.
</requirements>

<constraints>
- `Page` must not lose any fields that `ops/list.go` currently reads from `Task`
- Date fields on `Page` use the same parsing logic as `TaskFrontmatter` — reference `pkg/domain/task_frontmatter.go` for `DeferDate()`, `PlannedDate()`, `DueDate()`, `CompletedDate()`, `Goals()`
- Phase accessor matches `TaskFrontmatter.Phase()` — returns `*domain.TaskPhase`
- Preserve `TaskListItem.Goals` field — goals are set via `task.Goals()` on the `Page` type
- No changes to `TaskStorage`, `GoalStorage`, `ThemeStorage`, `ObjectiveStorage`, `VisionStorage` interfaces
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
