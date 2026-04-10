---
status: completed
spec: [008-flexible-frontmatter-refactor]
container: vault-cli-102-spec-008-entity-migration
dark-factory-version: v0.108.0-dirty
created: "2026-04-10T00:00:00Z"
queued: "2026-04-10T21:45:58Z"
started: "2026-04-10T22:26:29Z"
completed: "2026-04-10T22:50:47Z"
---

<summary>
- Goal, Theme, Objective, and Vision are each restructured into a typed frontmatter wrapper (embeds `FrontmatterMap`) plus shared `FileMetadata` and a `Content` named string type
- Unknown YAML fields in goal, theme, objective, and vision files survive read-write cycles without data loss
- `vault-cli goal/theme/objective/vision set <name> custom_key value` succeeds for any key; `get` returns the stored value
- The reflection-based `fieldByYAMLTag` in `pkg/ops/frontmatter_entity.go` is replaced with per-entity `GetField`/`SetField`/`ClearField` methods for all four entity types
- Goal checkbox parsing (`Tasks []CheckboxItem`) is preserved — it is metadata derived from content, not frontmatter
- Goal field `Completed` and `DeferDate` round-trip through the map; `StartDate`/`TargetDate` are stored as `"YYYY-MM-DD"` strings
- `NewGoalShowOperation`, `NewThemeShowOperation` etc. are updated to use the new accessor methods
- `SetPriority` on all four entities calls `Priority.Validate(ctx)` (added in Prompt 2) to reject negative values, satisfying spec AC #6
- All existing goal/theme/objective/vision tests are updated to the new accessor pattern; all pass
- All previous task migration changes (Prompt 2) are unaffected
</summary>

<objective>
Migrate `domain.Goal`, `domain.Theme`, `domain.Objective`, and `domain.Vision` from struct-based YAML frontmatter to typed `FrontmatterMap`-backed wrappers. Update storage implementations and the generic `frontmatter_entity.go` operations to use per-entity method calls instead of reflection. After this prompt, all five entity types (including Task from Prompt 2) use the map-based pattern.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read the relevant coding guides surfaced by the `coding` plugin: `go-error-wrapping-guide.md`, `go-testing-guide.md`, `go-composition.md`, `go-enum-type-pattern.md`.

**Prompts 1 and 2 must be completed first.** This prompt depends on:
- `domain.FrontmatterMap`, `domain.FileMetadata`, `domain.Content` (from Prompt 1)
- `domain.TaskFrontmatter` and the refactored `domain.Task` (from Prompt 2)
- `Priority.Validate(ctx) error` method on `pkg/domain/priority.go` (from Prompt 2, requirement 0)
- `baseStorage.parseToFrontmatterMap` and `baseStorage.serializeMapAsFrontmatter` (from Prompt 1)

Key files to read in full before making changes:
- `pkg/domain/goal.go` — Goal struct and GoalStatus
- `pkg/domain/theme.go` — Theme struct and ThemeStatus
- `pkg/domain/objective.go` — Objective struct and ObjectiveStatus
- `pkg/domain/vision.go` — Vision struct and VisionStatus
- `pkg/domain/task_frontmatter.go` — **use this as a template** for the pattern; follow it closely
- `pkg/storage/goal.go` — ReadGoal, WriteGoal, FindGoalByName, ListGoals
- `pkg/storage/theme.go` — ReadTheme, WriteTheme, FindThemeByName, ListThemes
- `pkg/storage/objective.go` — ReadObjective, WriteObjective, FindObjectiveByName, ListObjectives
- `pkg/storage/vision.go` — ReadVision, WriteVision, FindVisionByName, ListVisions
- `pkg/ops/frontmatter_entity.go` — ALL entity factory functions (full file); the goal is to replace `fieldByYAMLTag` usage for these four types
- `pkg/ops/frontmatter_reflect.go` — understand what helpers are being replaced
- `pkg/ops/goal_complete.go`, `pkg/ops/goal_defer.go` — may access goal struct fields directly
- `pkg/ops/objective_complete.go` — may access objective struct fields directly
- `pkg/cli/cli.go` — find goal/theme/objective/vision get and set command handlers; check if they use hardcoded switches like the task ones did
</context>

<requirements>
### 0. Add `Validate(ctx) error` methods to GoalStatus, ThemeStatus, ObjectiveStatus, VisionStatus

**Note**: These methods do NOT currently exist — verified via `grep -n 'func.*(Goal\|Theme\|Objective\|Vision)Status) Validate' pkg/domain/`. Only `TaskStatus.Validate` exists (at `pkg/domain/task.go:84`). The entity frontmatter setters below depend on `Validate` methods for each status type. Add them now before the setters can call them.

For each of `GoalStatus`, `ThemeStatus`, `ObjectiveStatus`, `VisionStatus`, add a `Validate(ctx context.Context) error` method that matches the `TaskStatus.Validate` pattern:

```go
// TaskStatus.Validate (at pkg/domain/task.go:84) — use as template:
func (s TaskStatus) Validate(ctx context.Context) error {
    if !AvailableTaskStatuses.Contains(s) {
        return fmt.Errorf("%w: unknown task status '%s'", validation.Error, s)
    }
    return nil
}
```

You will need a `AvailableXxxStatuses` set (or equivalent list) for each entity. If one does not exist, create a `var AvailableGoalStatuses = XxxStatuses{GoalStatusActive, GoalStatusCompleted, GoalStatusOnHold}` (using whatever constant names are defined in the existing file — read `pkg/domain/goal.go`, `theme.go`, `objective.go`, `vision.go` to find them) and an `XxxStatuses` slice type with a `.Contains(XxxStatus) bool` method. Follow the exact pattern used for `TaskStatuses` / `AvailableTaskStatuses` (see `pkg/domain/task.go` around the `TaskStatus` definitions).

Add unit tests for each new `Validate` method in the corresponding `pkg/domain/<entity>_test.go` file:
- Valid status returns nil
- Empty string returns an error
- Unknown status like `"banana"` returns an error

### 1. Create per-entity frontmatter types

Create one file per entity in `pkg/domain/`. Use `pkg/domain/task_frontmatter.go` as the exact template for structure and style.

**Each file must define:**
1. The type (e.g., `type GoalFrontmatter struct { FrontmatterMap }`)
2. A constructor (e.g., `func NewGoalFrontmatter(data map[string]any) GoalFrontmatter`)
3. Typed getters for every known field (list below)
4. Typed setters for every known field (with validation for status/priority)
5. `GetField(key string) string`, `SetField(ctx context.Context, key, value string) error`, `ClearField(key string)` matching `TaskFrontmatter` exactly

**Date serialization — use these exact formats (do NOT invent):**
- `*time.Time` (start_date, target_date) — stored as `"YYYY-MM-DD"` string. Parse with `time.Parse(time.DateOnly, raw)`. Format with `t.UTC().Format(time.DateOnly)`.
- `*libtime.Date` (completed) — stored as `"YYYY-MM-DD"` string. Parse with `libtime.ParseDate(ctx, raw)` (returns `*libtime.Date, error`). Format with `d.String()` — `libtime.Date.String()` returns `YYYY-MM-DD`. Do NOT call `d.Format()` without a layout argument — `libtime.Date.Format(layout string)` takes a layout and will not compile without one.
- `*DateOrDateTime` (defer_date) — reuse `formatDateOrDateTime` helper from `task_frontmatter.go` (move it to `frontmatter_map.go` as an unexported package helper if shared across entities, OR duplicate the 6-line helper per file).

#### `pkg/domain/goal_frontmatter.go`

Known Goal frontmatter fields (from `domain.Goal`):
- `status` → `GoalStatus` (string type, no normalization needed; use `GoalStatus(f.GetString("status"))`); setter validates via `GoalStatus.Validate(ctx)`
- `page_type` → `string`
- `theme` → `string`
- `priority` → `Priority` (int, same pattern as TaskFrontmatter); `SetPriority(ctx, p Priority) error` must call `p.Validate(ctx)` (from Prompt 2 requirement 0) and return the wrapped error on failure. Negative values are rejected per spec AC #6.
- `assignee` → `string`
- `start_date` → `*time.Time` (format: `time.DateOnly`)
- `target_date` → `*time.Time` (format: `time.DateOnly`)
- `tags` → `[]string` (via `GetStringSlice`)
- `completed` → `*libtime.Date` (format: `"YYYY-MM-DD"`; parse with `libtime.ParseDate(ctx, raw)`; format with `d.String()` — NOT `d.Format()`. `libtime.Date.Format(layout string)` takes a layout argument; the no-argument formatter is `String()`)
- `defer_date` → `*DateOrDateTime` (reuse `formatDateOrDateTime`)

#### `pkg/domain/theme_frontmatter.go`

Known Theme frontmatter fields:
- `status` → `ThemeStatus`
- `page_type` → `string`
- `priority` → `Priority`
- `assignee` → `string`
- `start_date` → `*time.Time`
- `target_date` → `*time.Time`
- `tags` → `[]string`

Same `GetField`/`SetField`/`ClearField` pattern.

#### `pkg/domain/objective_frontmatter.go`

Known Objective frontmatter fields:
- `status` → `ObjectiveStatus`
- `page_type` → `string`
- `priority` → `Priority`
- `assignee` → `string`
- `start_date` → `*time.Time`
- `target_date` → `*time.Time`
- `tags` → `[]string`
- `completed` → `*libtime.Date`

#### `pkg/domain/vision_frontmatter.go`

Known Vision frontmatter fields:
- `status` → `VisionStatus`
- `page_type` → `string`
- `priority` → `Priority`
- `assignee` → `string`
- `tags` → `[]string`

### 2. Refactor entity domain structs

For each of Goal, Theme, Objective, Vision, update the struct in its existing `pkg/domain/<entity>.go` file. Keep the status types, constants, and ID types. Remove the YAML-tagged frontmatter fields and the `yaml:"-"` metadata fields from the struct. Replace with embedding:

```go
// Goal example (apply same pattern to Theme, Objective, Vision):
type Goal struct {
    GoalFrontmatter
    FileMetadata
    Content Content

    // Tasks holds checkbox items parsed from content.
    // It is populated by the storage layer and is NOT stored in frontmatter.
    Tasks []CheckboxItem
}

func NewGoal(data map[string]any, meta FileMetadata, content Content) *Goal {
    return &Goal{
        GoalFrontmatter: NewGoalFrontmatter(data),
        FileMetadata:    meta,
        Content:         content,
    }
}
```

Note: `Goal.Tasks` is metadata derived from content (checkbox parsing). It stays as a direct field, not in the frontmatter map.

Theme, Objective, Vision do NOT have a `Tasks []CheckboxItem` field — omit it.

The `Content` field on all four entities is `domain.Content` (from Prompt 1), NOT raw `string`. Call sites passing content to storage/WriteFile must convert with `string(entity.Content)` or `entity.Content.String()`; call sites constructing entities from file bytes must wrap with `domain.Content(...)`.

### 3. Update storage implementations

For each entity's storage file (`goal.go`, `theme.go`, `objective.go`, `vision.go`), update the private `read<Entity>FromPath` helper and the `Write<Entity>` method to use map-based parse/serialize.

Pattern (same as what was done for Task in Prompt 2):

```go
// readGoalFromPath — updated version
func (g *goalStorage) readGoalFromPath(ctx context.Context, filePath, name string) (*domain.Goal, error) {
    content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
    if err != nil {
        return nil, errors.Wrapf(ctx, err, "read file %s", filePath)
    }

    var modTime *time.Time
    if info, err := os.Stat(filePath); err == nil {
        t := info.ModTime().UTC()
        modTime = &t
    }

    data, err := g.parseToFrontmatterMap(ctx, content)
    if err != nil {
        return nil, errors.Wrap(ctx, err, "parse frontmatter")
    }

    meta := domain.FileMetadata{Name: name, FilePath: filePath, ModifiedDate: modTime}
    goal := domain.NewGoal(data, meta, domain.Content(content))
    goal.Tasks = g.parseCheckboxes(string(content))
    return goal, nil
}

// WriteGoal — updated version
func (g *goalStorage) WriteGoal(ctx context.Context, goal *domain.Goal) error {
    content, err := g.serializeMapAsFrontmatter(ctx, goal.RawMap(), string(goal.Content))
    if err != nil {
        return errors.Wrap(ctx, err, "serialize frontmatter")
    }
    if err := os.WriteFile(goal.FilePath, []byte(content), 0600); err != nil {
        return errors.Wrapf(ctx, err, "write file %s", goal.FilePath)
    }
    return nil
}
```

Notes:
- `serializeMapAsFrontmatter` signature is `(ctx, data map[string]any, originalContent string)` — convert `goal.Content` to `string` with `string(goal.Content)`.
- `NewGoal` takes `domain.Content`, so wrap the `[]byte` with `domain.Content(content)`.
- Use `errors.Wrapf` per `go-error-wrapping-guide.md` instead of `errors.Wrap` + `fmt.Sprintf`.

Apply the same pattern for Theme, Objective, and Vision. Remove calls to the old `parseFrontmatter` in these files.

### 4. Update `pkg/ops/frontmatter_entity.go` — replace all reflection with method calls

The current `entityGetOperation.Execute`, `entitySetOperation.Execute`, `entityClearOperation.Execute`, `entityListAddOperation.Execute`, and `entityListRemoveOperation.Execute` all call `fieldByYAMLTag` and related reflection helpers.

Replace the reflection approach with a per-entity method dispatch. The cleanest approach is to define a new interface that all entity types implement:

```go
// FrontmatterEntity is implemented by all refactored entity types (Goal, Theme, Objective, Vision, Task).
type FrontmatterEntity interface {
    GetField(key string) string
    SetField(ctx context.Context, key, value string) error
    ClearField(key string)
}
```

Then update the operation structs to use this interface instead of reflection.

For **Get** operations: instead of `fieldByYAMLTag` + `getFieldAsString`, call `entity.GetField(key)`.

For **Set** operations: instead of `fieldByYAMLTag` + `setFieldFromString`, call `entity.SetField(ctx, key, value)`. Remove the `isReadOnlyTag` check — in the map-based design, metadata fields (Name, FilePath) are not in the YAML map at all, so they cannot be targeted by `SetField`.

For **Clear** operations: call `entity.ClearField(key)`.

For **ListAdd/ListRemove** operations: instead of `fieldByYAMLTag` + `appendToList`/`removeFromList`, use the entity's typed setters. The only list field on Goal/Theme/Objective/Vision is `tags` — use `entity.Tags()` and `entity.SetTags(updated)`. (`Goal.Goals` does NOT exist — `Goals []string` is a Task field, not a Goal field.) Update the factory functions (`NewGoalListAddOperation`, `NewThemeListAddOperation`, etc.) to call these tag methods directly. Since only `tags` is supported, the operation can either accept only `key == "tags"` or validate upfront and error on any other key.

For **Show** operations: replace `fieldByYAMLTag` iteration with reading the `FrontmatterMap.Keys()` and calling `entity.GetField(k)` for each key, plus the metadata fields directly.

The current `entityShowOperation.Execute` (around `pkg/ops/frontmatter_entity.go:577-598`) uses `reflect.Value.FieldByName("Name")` / `FieldByName("Content")` for metadata access. After this migration, replace the reflection block with a type switch on the concrete entity types — each type exposes `.Name` / `.FilePath` / `.Content` via the embedded `FileMetadata` and direct `Content` field, and the frontmatter map via `.Keys()` / `.GetField(k)`. Example shape:

```go
func (o *entityShowOperation) Execute(ctx context.Context, vaultPath, name string) (map[string]string, error) {
    result := make(map[string]string)

    switch e := entity.(type) {
    case *domain.Goal:
        result["name"] = e.Name
        result["file_path"] = e.FilePath
        result["content"] = string(e.Content)
        for _, k := range e.Keys() {
            result[k] = e.GetField(k)
        }
    case *domain.Theme:
        // same pattern
    case *domain.Objective:
        // same pattern
    case *domain.Vision:
        // same pattern
    default:
        return nil, errors.Errorf(ctx, "unsupported entity type %T", entity)
    }
    return result, nil
}
```

If the refactored code needs a unifying interface, define one in the same file:
```go
// FrontmatterEntity is implemented by Goal, Theme, Objective, Vision, and Task.
type FrontmatterEntity interface {
    GetField(key string) string
    SetField(ctx context.Context, key, value string) error
    ClearField(key string)
    Keys() []string
}
```
Note that `Name`/`FilePath`/`Content` are NOT on the interface because different entities may diverge; use type assertions for those fields. Do NOT use reflection.

After this change, `pkg/ops/frontmatter_reflect.go` functions (`fieldByYAMLTag`, `getFieldAsString`, `setFieldFromString`, `clearField`, `isListField`, `appendToList`, `removeFromList`, `isReadOnlyTag`) are no longer called by any code in `frontmatter_entity.go`. Do NOT delete `frontmatter_reflect.go` yet — that happens in Prompt 4.

### 5. Update ops files that access entity struct fields

Read these files in full and migrate every struct-field read/write:

- `pkg/ops/goal_complete.go` — `goal.Status`, `goal.Completed`, etc.
- `pkg/ops/goal_defer.go` — `goal.DeferDate` (around line 81)
- `pkg/ops/objective_complete.go` — `objective.Status`, `objective.Completed` (lines 63, 68-69)
- **`pkg/ops/update.go`** — lines 180 (`strings.Split(goal.Content, "\n")`) and 206 (`goal.Content = strings.Join(lines, "\n")`). After Prompt 1, `goal.Content` is `domain.Content` (named string type), not raw `string`. These call sites will NOT compile without conversion. Replace as follows:
  ```go
  // Line 180:
  lines := strings.Split(string(goal.Content), "\n")
  // Line 206:
  goal.Content = domain.Content(strings.Join(lines, "\n"))
  ```
- **`pkg/ops/complete.go`** — lines 297 and 319, same pattern:
  ```go
  // Line 297:
  lines := strings.Split(string(goal.Content), "\n")
  // Line 319:
  goal.Content = domain.Content(strings.Join(lines, "\n"))
  ```

Find any references to struct fields like `goal.Status`, `goal.DeferDate`, etc. and replace with method calls: `goal.Status()`, `goal.DeferDate()`, `goal.SetStatus(...)`, etc.

When calling `SetPriority`, note it now takes `(ctx, Priority)` and returns `error` — propagate the error via `errors.Wrap(ctx, err, "set priority")`.

**Before declaring done**, run these sweeps to catch any remaining compile errors from the `Content` type change:
```bash
grep -rn 'strings\.Split(.*\.Content,' pkg/ integration/ --include='*.go'
grep -rn '\.Content = strings\.' pkg/ integration/ --include='*.go'
grep -rn '\.Content = ""' pkg/ integration/ --include='*.go'
```
Every match must either use `string(xxx.Content)` for reading or `domain.Content(...)` for writing.

### 6. Update CLI handlers for goal/theme/objective/vision

Search `pkg/cli/cli.go` for the goal/theme/objective/vision `get`, `set`, and `clear` command handlers. If they contain hardcoded switches on field names (similar to how task was handled before Prompt 2), replace with `entity.GetField(key)` / `entity.SetField(ctx, key, value)` / `entity.ClearField(key)`. If they already delegate to the ops layer (via `EntityGetOperation` etc.), no change is needed there — the ops layer update in step 4 handles it.

### 7. Write tests

- Update `pkg/storage/goal_test.go`, `pkg/storage/objective_test.go`, `pkg/storage/vision_test.go`: replace all `entity.Field` struct access with `entity.Field()` method calls. Note: `pkg/storage/theme_test.go` does NOT exist — theme storage tests live in `markdown_test.go` (listed below).
- Update `pkg/storage/markdown_test.go`: lines 345, 346, 473 access `goal.Status`, `goal.Theme`, `theme.Status` as struct fields. Migrate to method calls `goal.Status()`, `goal.Theme()`, `theme.Status()`. Any other `entity.FieldName` reads in this file must be migrated too — run `grep -n 'goal\.\(Status\|Theme\|Priority\|Tasks\)\|theme\.\(Status\|Priority\)' pkg/storage/markdown_test.go` first to enumerate.
- Update `pkg/ops/frontmatter_entity_test.go`: replace reflection-based assertions with method-call assertions.
- Update `pkg/ops/goal_complete_test.go`, `pkg/ops/goal_defer_test.go`, `pkg/ops/objective_complete_test.go`, `pkg/ops/update_test.go`, `pkg/ops/complete_test.go` (if they touch goal/objective struct fields).
- Update `pkg/ops/frontmatter_test.go` for any goal/theme/objective/vision test cases.

**Sweep before declaring done**, catch any missed test literals:
```bash
grep -rn 'domain\.Goal{\|domain\.Theme{\|domain\.Objective{\|domain\.Vision{' pkg/ integration/ --include='*.go'
```
Every match must either use `domain.NewGoal(...)`/`NewTheme(...)`/etc constructors or explicit embedding of the new frontmatter types.
- Add tests in `pkg/domain/goal_frontmatter_test.go` (and similar for other entities):
  - `SetField("custom_key", "value")` → `GetField("custom_key") == "value"` (unknown field round-trip)
  - `SetField("status", "active")` succeeds; `Status() == GoalStatusActive`
  - `GetField("tags")` returns comma-joined string after `SetField("tags", "a,b")`
  - Known date field (`start_date`) round-trips through `SetField`/`GetField` as `"YYYY-MM-DD"`
  - `SetField("priority", "-1")` returns an error (spec AC #6 — negative priority rejected). Repeat for theme, objective, vision.
</requirements>

<constraints>
- Storage interface signatures must NOT change
- CLI command surface and flags must NOT change
- Goal checkbox parsing (`goal.Tasks = parseCheckboxes(content)`) must be preserved — it is content-derived metadata, not frontmatter
- `pkg/ops/frontmatter_reflect.go` must NOT be deleted in this prompt — it is deleted in Prompt 4 after verification
- `docs/development-patterns.md` must NOT be edited in this prompt — Prompt 4 (cleanup) handles the doc update
- The Task migration from Prompt 2 must be untouched
- `domain.Decision` is explicitly excluded from this refactor — leave it unchanged
- Validation for known fields must work: `SetField("status", "banana")` on a Goal returns an error; `SetField("priority", "-1")` returns an error (via `Priority.Validate` from Prompt 2)
- Unknown fields set via `SetField` are stored as strings without validation
- All existing tests must pass (update assertions to new accessor pattern)
- One type per file convention: `GoalFrontmatter` in `goal_frontmatter.go`, etc.
- Before declaring done, run `grep -rn 'goal\.\(Status\|Priority\|Assignee\|DeferDate\|Completed\|StartDate\|TargetDate\|Tags\|Theme\|PageType\)' pkg/ | grep -v '()' ` to catch any missed struct-field access on Goal. Repeat for theme/objective/vision.
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
# Confirm all four new frontmatter types exist
grep -rn 'type GoalFrontmatter\|type ThemeFrontmatter\|type ObjectiveFrontmatter\|type VisionFrontmatter' pkg/domain/
# expected: one line each

# Confirm fieldByYAMLTag is no longer called in frontmatter_entity.go
grep -n 'fieldByYAMLTag' pkg/ops/frontmatter_entity.go
# expected: no output

# Confirm frontmatter_reflect.go still exists (not yet deleted)
ls pkg/ops/frontmatter_reflect.go
# expected: file found

# Confirm Goal, Theme, Objective, Vision structs embed their frontmatter types
grep -n 'GoalFrontmatter\|ThemeFrontmatter\|ObjectiveFrontmatter\|VisionFrontmatter' pkg/domain/goal.go pkg/domain/theme.go pkg/domain/objective.go pkg/domain/vision.go
# expected: one line per file

# Confirm all four entity SetPriority implementations call Priority.Validate
grep -n 'p.Validate(ctx)' pkg/domain/goal_frontmatter.go pkg/domain/theme_frontmatter.go pkg/domain/objective_frontmatter.go pkg/domain/vision_frontmatter.go
# expected: one line per file

# Confirm Content Content type on all entities
grep -n 'Content Content' pkg/domain/goal.go pkg/domain/theme.go pkg/domain/objective.go pkg/domain/vision.go
# expected: one line per file

# Coverage check
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/domain/... ./pkg/storage/... ./pkg/ops/... && \
  go tool cover -func=/tmp/cover.out | grep 'frontmatter\|_frontmatter'
# expected: ≥80% on new files
```
</verification>
