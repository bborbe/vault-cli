---
status: completed
spec: ["002"]
summary: Created Objective and Vision domain structs and storage layer, added ThemeStorage/ObjectiveStorage/VisionStorage narrow interfaces with FindByName methods, generated counterfeiter mocks, and added test coverage above 80%.
container: vault-cli-068-spec-002-domain-storage
dark-factory-version: v0.57.5
created: "2026-03-17T10:00:00Z"
queued: "2026-03-17T10:30:14Z"
started: "2026-03-17T10:30:16Z"
completed: "2026-03-17T10:38:38Z"
branch: dark-factory/generic-frontmatter-ops
---

<summary>
- Objective and Vision become first-class entity types alongside task, goal, and theme
- Objectives and visions can be found by name, read, and written back to disk
- Themes gain a find-by-name capability (previously only read-by-ID was supported)
- All five entity types now have consistent storage patterns
- Mocks are generated for all new storage interfaces to support testing
- New storage code has test coverage above 80%
</summary>

<objective>
Create the domain structs and storage layer for Objective and Vision entities, and extract ThemeStorage as a proper narrow interface with FindThemeByName. This is the foundation that the generic frontmatter ops (prompt 2) and CLI wiring (prompt 3) depend on.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read these files before making changes (read ALL of them first):

- `pkg/domain/goal.go` — pattern for entity domain struct (Goal)
- `pkg/domain/theme.go` — pattern for entity domain struct (Theme)
- `pkg/domain/task.go` — pattern for entity domain struct (Task)
- `pkg/storage/storage.go` — existing Storage composite, TaskStorage, GoalStorage interfaces
- `pkg/storage/goal.go` — concrete goalStorage pattern to follow for objective/vision
- `pkg/storage/theme.go` — concrete themeStorage pattern; has ReadTheme/WriteTheme but no FindThemeByName
- `pkg/storage/base.go` — shared baseStorage with parseFrontmatter, findFileByName, serializeWithFrontmatter
</context>

<requirements>

## 1. Create `pkg/domain/objective.go`

New file with:
- `Objective` struct with YAML-tagged frontmatter fields:
  - `Status ObjectiveStatus yaml:"status"`
  - `PageType string yaml:"page_type"`
  - `Priority Priority yaml:"priority,omitempty"`
  - `Assignee string yaml:"assignee,omitempty"`
  - `StartDate *time.Time yaml:"start_date,omitempty"`
  - `TargetDate *time.Time yaml:"target_date,omitempty"`
  - `Tags []string yaml:"tags,omitempty"`
  - Metadata: `Name string yaml:"-"`, `Content string yaml:"-"`, `FilePath string yaml:"-"`
- `ObjectiveStatus string` type with constants `ObjectiveStatusActive`, `ObjectiveStatusCompleted`, `ObjectiveStatusOnHold`
- `ObjectiveID string` type with `.String() string` method

## 2. Create `pkg/domain/vision.go`

New file with:
- `Vision` struct with YAML-tagged frontmatter fields:
  - `Status VisionStatus yaml:"status"`
  - `PageType string yaml:"page_type"`
  - `Priority Priority yaml:"priority,omitempty"`
  - `Assignee string yaml:"assignee,omitempty"`
  - `Tags []string yaml:"tags,omitempty"`
  - Metadata: `Name string yaml:"-"`, `Content string yaml:"-"`, `FilePath string yaml:"-"`
- `VisionStatus string` type with constants `VisionStatusActive`, `VisionStatusCompleted`, `VisionStatusArchived`
- `VisionID string` type with `.String() string` method

## 3. Update `pkg/storage/storage.go` — add ThemeStorage interface

Add a new narrow interface:
```go
//counterfeiter:generate -o ../../mocks/theme-storage.go --fake-name ThemeStorage . ThemeStorage
type ThemeStorage interface {
    WriteTheme(ctx context.Context, theme *domain.Theme) error
    FindThemeByName(ctx context.Context, vaultPath string, name string) (*domain.Theme, error)
}
```

Update the `Storage` composite interface to embed `ThemeStorage` (replacing the existing `ReadTheme`/`WriteTheme` legacy methods). Keep `ReadTheme` on the Storage interface as a legacy method only — do NOT move it to ThemeStorage.

Add two new narrow interfaces:
```go
//counterfeiter:generate -o ../../mocks/objective-storage.go --fake-name ObjectiveStorage . ObjectiveStorage
type ObjectiveStorage interface {
    WriteObjective(ctx context.Context, objective *domain.Objective) error
    FindObjectiveByName(ctx context.Context, vaultPath string, name string) (*domain.Objective, error)
}

//counterfeiter:generate -o ../../mocks/vision-storage.go --fake-name VisionStorage . VisionStorage
type VisionStorage interface {
    WriteVision(ctx context.Context, vision *domain.Vision) error
    FindVisionByName(ctx context.Context, vaultPath string, name string) (*domain.Vision, error)
}
```

Embed `ThemeStorage`, `ObjectiveStorage`, and `VisionStorage` in the `Storage` composite interface.

## 4. Implement `FindThemeByName` in `pkg/storage/theme.go`

Add this method to `themeStorage`, following the exact same pattern as `goalStorage.FindGoalByName`:

```go
// FindThemeByName searches for a theme by name in the vault.
func (t *themeStorage) FindThemeByName(
    ctx context.Context,
    vaultPath string,
    name string,
) (*domain.Theme, error) {
    themesDir := filepath.Join(vaultPath, t.config.ThemesDir)
    matchedPath, matchedName, err := t.findFileByName(ctx, themesDir, name)
    if err != nil {
        return nil, errors.Wrap(ctx, err, "find theme file")
    }
    return t.readThemeFromPath(ctx, matchedPath, matchedName)
}
```

## 5. Create `pkg/storage/objective.go`

Mirror `pkg/storage/goal.go` exactly, adapting for Objective:
- `objectiveStorage struct` embedding `*baseStorage`
- `ReadObjective(ctx, vaultPath, objectiveID ObjectiveID) (*domain.Objective, error)` — reads from `config.ObjectivesDir`
- `readObjectiveFromPath(ctx, filePath, name) (*domain.Objective, error)` — private helper
- `WriteObjective(ctx, objective *domain.Objective) error`
- `FindObjectiveByName(ctx, vaultPath, name) (*domain.Objective, error)` — uses `t.findFileByName`

## 6. Create `pkg/storage/vision.go`

Mirror `pkg/storage/theme.go` exactly, adapting for Vision:
- `visionStorage struct` embedding `*baseStorage`
- `ReadVision(ctx, vaultPath, visionID VisionID) (*domain.Vision, error)` — reads from `config.VisionDir`
- `readVisionFromPath(ctx, filePath, name) (*domain.Vision, error)` — private helper
- `WriteVision(ctx, vision *domain.Vision) error`
- `FindVisionByName(ctx, vaultPath, name) (*domain.Vision, error)` — uses `t.findFileByName`

## 7. Update `pkg/storage/storage.go` — update Storage composite and markdownStorage

Update `markdownStorage` struct to embed `*objectiveStorage` and `*visionStorage`.

Update `NewStorage` to construct and wire in the new sub-storages:
```go
func NewStorage(storageConfig *Config) Storage {
    // ...existing...
    base := &baseStorage{config: storageConfig}
    return &markdownStorage{
        taskStorage:      &taskStorage{baseStorage: base},
        goalStorage:      &goalStorage{baseStorage: base},
        dailyNoteStorage: &dailyNoteStorage{baseStorage: base},
        pageStorage:      &pageStorage{baseStorage: base},
        decisionStorage:  &decisionStorage{baseStorage: base},
        themeStorage:     &themeStorage{baseStorage: base},
        objectiveStorage: &objectiveStorage{baseStorage: base},
        visionStorage:    &visionStorage{baseStorage: base},
    }
}
```

Add constructor functions:
```go
func NewThemeStorage(storageConfig *Config) ThemeStorage { ... }
func NewObjectiveStorage(storageConfig *Config) ObjectiveStorage { ... }
func NewVisionStorage(storageConfig *Config) VisionStorage { ... }
```

Also update the Storage composite interface to include `ReadObjective` and `ReadVision` as legacy methods (same pattern as `ReadTheme`/`ReadGoal` on the interface).

## 8. Generate mocks

After adding all counterfeiter directives, run:
```
go generate ./pkg/storage/...
```

This will create:
- `mocks/theme-storage.go`
- `mocks/objective-storage.go`
- `mocks/vision-storage.go`

## 9. Write tests

Create tests in new files (or add to existing `pkg/storage/` test files if appropriate):
- `pkg/storage/objective_test.go` — at minimum test `FindObjectiveByName` finds a file and returns correctly parsed Objective
- `pkg/storage/vision_test.go` — same for Vision

For `FindThemeByName`, if the existing `pkg/storage/markdown_test.go` doesn't cover it, add a test there.

Domain struct tests (optional, only if they have non-trivial logic): if `ObjectiveStatus` or `VisionStatus` have custom YAML unmarshal, test them.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing task, goal, and decision tests must still pass unchanged
- `WriteTheme` and `ReadTheme` on the `Storage` interface must remain for backward compatibility — do NOT remove them from the composite
- `markdownStorage` embeds each sub-storage by pointer, so it satisfies both the narrow interface and the composite interface simultaneously
- Follow the exact same pattern as `goalStorage` for objective/vision — no deviations
- All new domain structs use `*time.Time` (not `*libtime.Date`) for date fields, matching the pattern in `goal.go` and `theme.go`
- File permissions for WriteObjective/WriteVision: `0600` (matching existing WriteGoal/WriteTheme)
- JSON output format supported via YAML serializer (no changes needed here — `serializeWithFrontmatter` handles it)
- Use `github.com/bborbe/errors` for error wrapping — never `fmt.Errorf` for wrapping
- Multi-vault dispatch works automatically since storage constructors accept config from vault
</constraints>

<verification>
Run `make precommit` — must pass.

Additional verification:
- `go build ./...` compiles with no errors
- `go test ./pkg/domain/... ./pkg/storage/...` passes with all new tests
- `go generate ./pkg/storage/...` produces mock files without errors
</verification>
