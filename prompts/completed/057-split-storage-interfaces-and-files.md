---
status: completed
summary: Split monolithic pkg/storage/markdown.go into per-domain files (base, task, goal, theme, daily_note, page, decision) with narrow interfaces, a composed Storage interface preserving backward compatibility, and a shared baseStorage embedded struct.
container: vault-cli-057-split-storage-interfaces-and-files
dark-factory-version: v0.54.0
created: "2026-03-16T11:00:00Z"
queued: "2026-03-16T12:09:47Z"
started: "2026-03-16T12:09:49Z"
completed: "2026-03-16T12:16:17Z"
---

<summary>
- The single large storage file is split into one file per domain (tasks, goals, daily notes, pages, decisions)
- Each domain gets its own narrow interface with only the methods it exposes
- A composed interface embeds all domain interfaces so existing callers keep working
- Shared parsing and serialization helpers move to a common embedded struct reused by all domains
- Methods not called by any operation (ReadTask, ListTasks, ReadGoal, ReadTheme, WriteTheme) are removed from interfaces but kept as private helpers where needed internally
- Storage tests are updated to call private helpers or use the composed interface for removed methods
</summary>

<objective>
Split `pkg/storage/markdown.go` into per-domain files with narrow interfaces and a composed `Storage` interface, extracting shared helpers into a `baseStorage` embedded struct. This is the foundational refactoring step -- no consumers are changed yet.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/storage/markdown.go` -- the monolithic file to split (~660 lines).
Read `pkg/storage/markdown_test.go` -- understand existing test patterns.
Read `docs/development-patterns.md` -- naming and architecture conventions.
Read `mocks/storage.go` -- the current monolithic mock (will be regenerated in a later prompt).
</context>

<requirements>
1. Create `pkg/storage/storage.go` containing:
   - The `Config` struct and its constructors (`NewConfigFromVault`, `DefaultConfig`) -- move from markdown.go
   - Per-domain interfaces:

   ```go
   //counterfeiter:generate -o ../../mocks/task-storage.go --fake-name TaskStorage . TaskStorage
   type TaskStorage interface {
       WriteTask(ctx context.Context, task *domain.Task) error
       FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)
   }

   //counterfeiter:generate -o ../../mocks/goal-storage.go --fake-name GoalStorage . GoalStorage
   type GoalStorage interface {
       WriteGoal(ctx context.Context, goal *domain.Goal) error
       FindGoalByName(ctx context.Context, vaultPath string, name string) (*domain.Goal, error)
   }

   //counterfeiter:generate -o ../../mocks/daily-note-storage.go --fake-name DailyNoteStorage . DailyNoteStorage
   type DailyNoteStorage interface {
       ReadDailyNote(ctx context.Context, vaultPath string, date string) (string, error)
       WriteDailyNote(ctx context.Context, vaultPath string, date string, content string) error
   }

   //counterfeiter:generate -o ../../mocks/page-storage.go --fake-name PageStorage . PageStorage
   type PageStorage interface {
       ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Task, error)
   }

   //counterfeiter:generate -o ../../mocks/decision-storage.go --fake-name DecisionStorage . DecisionStorage
   type DecisionStorage interface {
       ListDecisions(ctx context.Context, vaultPath string) ([]*domain.Decision, error)
       FindDecisionByName(ctx context.Context, vaultPath string, name string) (*domain.Decision, error)
       WriteDecision(ctx context.Context, decision *domain.Decision) error
   }
   ```

   - The composed Storage interface embedding all domain interfaces:

   ```go
   //counterfeiter:generate -o ../../mocks/storage.go --fake-name Storage . Storage
   type Storage interface {
       TaskStorage
       GoalStorage
       DailyNoteStorage
       PageStorage
       DecisionStorage
   }
   ```

   - NOTE: The narrow interfaces do NOT include the unused methods: `ReadTask`, `ListTasks`, `ReadGoal`, `ReadTheme`, `WriteTheme`. These are removed from the public API. The `readTaskFromPath`, `readGoalFromPath`, `readThemeFromPath` private methods remain where needed internally.

   - However, the composed `Storage` interface DOES need to keep `ReadTask`, `ListTasks`, `ReadGoal`, `ReadTheme`, and `WriteTheme` so that `pkg/storage/markdown_test.go` compiles (it calls these methods through the `Storage` interface). Add them to the composed interface only:

   ```go
   type Storage interface {
       TaskStorage
       GoalStorage
       DailyNoteStorage
       PageStorage
       DecisionStorage
       // Legacy methods — used by storage tests, not by ops.
       // Keep on composed interface for backward compat; not on narrow interfaces.
       ReadTask(ctx context.Context, vaultPath string, taskID domain.TaskID) (*domain.Task, error)
       ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)
       ReadGoal(ctx context.Context, vaultPath string, goalID domain.GoalID) (*domain.Goal, error)
       ReadTheme(ctx context.Context, vaultPath string, themeID domain.ThemeID) (*domain.Theme, error)
       WriteTheme(ctx context.Context, theme *domain.Theme) error
   }
   ```

   - The `taskStorage` struct implements `ReadTask` and `ListTasks` as exported methods (so they satisfy the composed interface). Same for `goalStorage.ReadGoal`, `themeStorage.ReadTheme`, `themeStorage.WriteTheme`.

   - The `NewStorage` constructor function that returns `Storage`:

   ```go
   func NewStorage(storageConfig *Config) Storage {
       if storageConfig == nil {
           storageConfig = DefaultConfig()
       }
       base := &baseStorage{config: storageConfig}
       return &markdownStorage{
           taskStorage:      &taskStorage{baseStorage: base},
           goalStorage:      &goalStorage{baseStorage: base},
           dailyNoteStorage: &dailyNoteStorage{baseStorage: base},
           pageStorage:      &pageStorage{baseStorage: base},
           decisionStorage:  &decisionStorage{baseStorage: base},
       }
   }
   ```

   - The `markdownStorage` struct now composes all domain storages:

   ```go
   type markdownStorage struct {
       *taskStorage
       *goalStorage
       *dailyNoteStorage
       *pageStorage
       *decisionStorage
   }
   ```

   - Per-domain constructor functions for narrow usage (used by ops in the next prompt):

   ```go
   func NewTaskStorage(storageConfig *Config) TaskStorage {
       if storageConfig == nil { storageConfig = DefaultConfig() }
       return &taskStorage{baseStorage: &baseStorage{config: storageConfig}}
   }
   // Same pattern for NewGoalStorage, NewDailyNoteStorage, NewPageStorage, NewDecisionStorage
   ```

2. Create `pkg/storage/base.go` containing:
   - The `baseStorage` struct:

   ```go
   type baseStorage struct {
       config *Config
   }
   ```

   - The `frontmatterRegex` and `checkboxRegex` package-level vars (move from markdown.go)
   - Methods on `*baseStorage`:
     - `parseFrontmatter(content []byte, target interface{}) error`
     - `serializeWithFrontmatter(frontmatter interface{}, originalContent string) (string, error)`
     - `findFileByName(dir string, name string) (string, string, error)`
     - `parseCheckboxes(content string) []domain.CheckboxItem`
   - The standalone `isSymlinkOutsideVault(path, vaultPath string) bool` function (stays package-level, not a method)

3. Create `pkg/storage/task.go` containing:
   - `type taskStorage struct { *baseStorage }`
   - Methods: `WriteTask`, `FindTaskByName`, `readTaskFromPath` (private helper)
   - Move the implementations verbatim from markdown.go, changing receiver from `*markdownStorage` to `*taskStorage` and calling `t.baseStorage.parseFrontmatter(...)` etc. (or just `t.parseFrontmatter(...)` since baseStorage is embedded)

4. Create `pkg/storage/goal.go` containing:
   - `type goalStorage struct { *baseStorage }`
   - Methods: `WriteGoal`, `FindGoalByName`, `readGoalFromPath` (private helper)
   - Same pattern as task.go

5. Create `pkg/storage/daily_note.go` containing:
   - `type dailyNoteStorage struct { *baseStorage }`
   - Methods: `ReadDailyNote`, `WriteDailyNote`

6. Create `pkg/storage/page.go` containing:
   - `type pageStorage struct { *baseStorage }`
   - Methods: `ListPages`
   - NOTE: `ListPages` currently calls `readTaskFromPath` -- since that's on `taskStorage`, the `pageStorage` needs its own copy or the helper should live on `baseStorage`. The cleanest solution: move the `readTaskFromPath` logic into `pageStorage` as a private method, since `ListPages` is the only consumer outside of `taskStorage`. Alternatively, put a `readTaskFromPath` on `baseStorage` since it uses only `parseFrontmatter` (a baseStorage method). Choose the `baseStorage` approach since both `taskStorage` and `pageStorage` need it.

7. Create `pkg/storage/decision.go` containing:
   - `type decisionStorage struct { *baseStorage }`
   - Methods: `ListDecisions`, `FindDecisionByName`, `WriteDecision`, `readDecisionFromPath` (private helper)

8. Delete `pkg/storage/markdown.go` -- all its content has been moved to the new files.

9. Place the `readTaskFromPath` helper on `*baseStorage` in `base.go`. Both `taskStorage.FindTaskByName` and `pageStorage.ListPages` use it, and it only depends on `parseFrontmatter` (a baseStorage method).

10. Verify: Run `make precommit` to confirm compilation and all tests pass.
</requirements>

<constraints>
- All new files must have the same copyright/license header as the original markdown.go
- All new files must be in `package storage`
- The composed `Storage` interface must be fully backward compatible -- any code that uses `storage.Storage` today must continue to compile and work
- Do NOT change any files outside `pkg/storage/` in this prompt -- ops, cli, mocks, and tests are updated in subsequent prompts
- Do NOT remove private helper methods that are used internally (e.g., `readTaskFromPath`, `readGoalFromPath`, `readDecisionFromPath`)
- DO remove from the narrow domain interfaces: `ReadTask`, `ListTasks`, `ReadGoal`, `ReadTheme`, `WriteTheme` -- these are not called by any ops code. BUT keep them on the composed `Storage` interface so `markdown_test.go` compiles
- The `isSymlinkOutsideVault` function stays package-level (not a method) as it has no receiver dependency
- Do NOT run `go generate` yet -- mock regeneration happens in prompt 3
- Do NOT commit -- dark-factory handles git
- Existing tests must still pass -- `markdown_test.go` uses `storage.NewStorage()` which returns the composed `Storage` interface including legacy methods. Ops tests use `mocks.Storage` which still implements everything.
</constraints>

<verification>
Run `make precommit` -- must pass.

Specifically verify:
- `go build ./...` compiles without errors
- `go test ./pkg/storage/...` passes
- `go test ./pkg/ops/...` passes (ops still use `storage.Storage` which embeds all narrow interfaces)
- `go vet ./...` passes
</verification>
