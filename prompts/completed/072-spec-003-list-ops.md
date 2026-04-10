---
status: completed
spec: [003-list-field-add-remove]
summary: Added EntityListAddOperation and EntityListRemoveOperation to the generic entity frontmatter ops layer, with isListField/appendToList/removeFromList reflection helpers and constructors for all five entity types (task, goal, theme, objective, vision)
container: vault-cli-072-spec-003-list-ops
dark-factory-version: v0.57.5
created: "2026-03-17T10:34:50Z"
queued: "2026-03-17T10:44:29Z"
started: "2026-03-17T11:00:53Z"
completed: "2026-03-17T11:09:04Z"
branch: dark-factory/list-field-add-remove
---

<summary>
- List fields (e.g. goals, tags) on all five entity types can be detected automatically via reflection
- Appending an item to a list field is a safe, idempotent-free operation — duplicates are rejected with a clear error
- Removing an item from a list field fails explicitly when the value is not present — no silent no-ops
- Attempting add or remove on a scalar field returns a "not a list field" error with no file write
- Task, goal, theme, objective, and vision entities all gain EntityListAdd and EntityListRemove operations
- New operations reuse the existing reflection helpers from frontmatter_reflect.go — no new field switch statements
- All new code has test coverage above 80%
</summary>

<objective>
Add `EntityListAddOperation` and `EntityListRemoveOperation` to the generic entity frontmatter ops layer, backed by new reflection helpers (`isListField`, `appendToList`, `removeFromList`) in `frontmatter_reflect.go`. Provide constructors for all five entity types (task, goal, theme, objective, vision). This is the foundation that CLI wiring (prompt 2) depends on.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files before making changes (read ALL of them first):

- `pkg/ops/frontmatter_reflect.go` — existing reflection helpers (`fieldByYAMLTag`, `getFieldAsString`, `setFieldFromString`, `clearField`, `isReadOnlyTag`); new helpers go here
- `pkg/ops/frontmatter_entity.go` — existing `EntityGetOperation`, `EntitySetOperation`, `EntityClearOperation`, `EntityShowOperation` interfaces and implementations; new add/remove interfaces go here
- `pkg/ops/frontmatter_reflect_test.go` — test patterns for reflection helpers; extend with new helper tests
- `pkg/ops/frontmatter_entity_test.go` — test patterns for entity ops; extend with add/remove tests
- `pkg/storage/storage.go` — `TaskStorage`, `GoalStorage`, `ThemeStorage`, `ObjectiveStorage`, `VisionStorage` interfaces
- `pkg/domain/task.go` — Task struct with list fields `Goals []string yaml:"goals"`, `Tags []string yaml:"tags"`
- `pkg/domain/goal.go` — Goal struct with list fields
- `mocks/` directory — counterfeiter-generated mocks for storage interfaces used in tests
</context>

<requirements>

## 1. Add list helpers to `pkg/ops/frontmatter_reflect.go`

Add three new unexported helpers immediately after the existing `clearField` function:

### `isListField`

```go
// isListField returns true if the struct field is a slice type.
func isListField(fieldVal reflect.Value) bool {
    return fieldVal.Kind() == reflect.Slice
}
```

### `appendToList`

```go
// appendToList appends value to a []string slice field.
// Returns an error if the value already exists in the list.
func appendToList(ctx context.Context, fieldVal reflect.Value, value string) error {
    if fieldVal.Kind() != reflect.Slice {
        return fmt.Errorf("field is not a list field")
    }
    for i := 0; i < fieldVal.Len(); i++ {
        if fieldVal.Index(i).String() == value {
            return fmt.Errorf("value %q already exists in list", value)
        }
    }
    newSlice := reflect.Append(fieldVal, reflect.ValueOf(value))
    fieldVal.Set(newSlice)
    return nil
}
```

### `removeFromList`

```go
// removeFromList removes value from a []string slice field.
// Returns an error if the value is not found in the list.
func removeFromList(ctx context.Context, fieldVal reflect.Value, value string) error {
    if fieldVal.Kind() != reflect.Slice {
        return fmt.Errorf("field is not a list field")
    }
    for i := 0; i < fieldVal.Len(); i++ {
        if fieldVal.Index(i).String() == value {
            // Remove element at index i by appending the two slices around it
            newSlice := reflect.AppendSlice(fieldVal.Slice(0, i), fieldVal.Slice(i+1, fieldVal.Len()))
            fieldVal.Set(newSlice)
            return nil
        }
    }
    return fmt.Errorf("value %q not found in list", value)
}
```

NOTE: `appendToList` and `removeFromList` accept `context.Context` as the first parameter to match project conventions for error wrapping, even though they currently don't wrap external errors.

## 2. Add `EntityListAddOperation` and `EntityListRemoveOperation` to `pkg/ops/frontmatter_entity.go`

### Interface definitions

Add after the existing `EntityClearOperation` interface:

```go
//counterfeiter:generate -o ../../mocks/entity-list-add-operation.go --fake-name EntityListAddOperation . EntityListAddOperation
type EntityListAddOperation interface {
    Execute(ctx context.Context, vaultPath, entityName, field, value string) error
}

//counterfeiter:generate -o ../../mocks/entity-list-remove-operation.go --fake-name EntityListRemoveOperation . EntityListRemoveOperation
type EntityListRemoveOperation interface {
    Execute(ctx context.Context, vaultPath, entityName, field, value string) error
}
```

### Implementation structs

Both operations share the same struct shape as `entitySetOperation`:

```go
type entityListAddOperation struct {
    findFn     func(ctx context.Context, vaultPath, name string) (any, error)
    writeFn    func(ctx context.Context, entity any) error
    entityType string
}

func (o *entityListAddOperation) Execute(ctx context.Context, vaultPath, entityName, field, value string) error {
    entity, err := o.findFn(ctx, vaultPath, entityName)
    if err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
    }
    sf, fieldVal, found := fieldByYAMLTag(entity, field)
    if !found {
        return fmt.Errorf("unknown field %q for %s", field, o.entityType)
    }
    if isReadOnlyTag(sf) {
        return fmt.Errorf("field %q is read-only", field)
    }
    if !isListField(fieldVal) {
        return fmt.Errorf("field %q is not a list field", field)
    }
    if err := appendToList(ctx, fieldVal, value); err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("append to field %q", field))
    }
    if err := o.writeFn(ctx, entity); err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("write %s", o.entityType))
    }
    return nil
}
```

The `entityListRemoveOperation` follows the identical pattern but calls `removeFromList` instead of `appendToList`.

### Constructor functions

Add constructor functions for all five entity types. Follow the exact same closure pattern as the existing `NewGoalSetOperation`, `NewGoalClearOperation` constructors.

**For goal:**
```go
func NewGoalListAddOperation(goalStorage storage.GoalStorage) EntityListAddOperation {
    return &entityListAddOperation{
        findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
            return goalStorage.FindGoalByName(ctx, vaultPath, name)
        },
        writeFn: func(ctx context.Context, entity any) error {
            return goalStorage.WriteGoal(ctx, entity.(*domain.Goal))
        },
        entityType: "goal",
    }
}

func NewGoalListRemoveOperation(goalStorage storage.GoalStorage) EntityListRemoveOperation { ... }
```

Add identical pairs for:
- `NewThemeListAddOperation` / `NewThemeListRemoveOperation` (using `storage.ThemeStorage`, `domain.Theme`)
- `NewObjectiveListAddOperation` / `NewObjectiveListRemoveOperation` (using `storage.ObjectiveStorage`, `domain.Objective`)
- `NewVisionListAddOperation` / `NewVisionListRemoveOperation` (using `storage.VisionStorage`, `domain.Vision`)
- `NewTaskListAddOperation` / `NewTaskListRemoveOperation` (using `storage.TaskStorage`, `domain.Task`)

**Task constructors:**
```go
func NewTaskListAddOperation(taskStorage storage.TaskStorage) EntityListAddOperation {
    return &entityListAddOperation{
        findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
            return taskStorage.FindTaskByName(ctx, vaultPath, name)
        },
        writeFn: func(ctx context.Context, entity any) error {
            return taskStorage.WriteTask(ctx, entity.(*domain.Task))
        },
        entityType: "task",
    }
}

func NewTaskListRemoveOperation(taskStorage storage.TaskStorage) EntityListRemoveOperation { ... }
```

## 3. Generate mocks

After adding the counterfeiter directives, run:
```
go generate ./pkg/ops/...
```

This creates:
- `mocks/entity-list-add-operation.go`
- `mocks/entity-list-remove-operation.go`

## 4. Tests in `pkg/ops/frontmatter_reflect_test.go` — extend existing suite

Add a new `Describe("isListField")` block:
- Returns true for a `[]string` field
- Returns false for a `string` field
- Returns false for a `*time.Time` field

Add a new `Describe("appendToList")` block:
- Appends value to empty list → list has one element
- Appends value to non-empty list → list has N+1 elements
- Appending duplicate value → returns error containing "already exists"
- Calling on non-slice field → returns error containing "not a list field"

Add a new `Describe("removeFromList")` block:
- Removes existing value from list → list has N-1 elements, value absent
- Removing non-existent value → returns error containing "not found"
- Removing last element → list is empty (not nil — it becomes an empty slice from reflect.AppendSlice)
- Calling on non-slice field → returns error containing "not a list field"

## 5. Tests in `pkg/ops/frontmatter_entity_test.go` — extend existing suite

Use counterfeiter mocks for `GoalStorage` (from `mocks/goal-storage.go`) and `TaskStorage` (from `mocks/task-storage.go`).

Add `Describe("NewGoalListAddOperation")`:
- Successfully adds value to tags field → `FindGoalByName` called, `WriteGoal` called with updated tags
- Returns error containing "already exists" when value already in list — `WriteGoal` NOT called
- Returns error containing "not a list field" for scalar field (e.g. `status`) — `WriteGoal` NOT called
- Returns error containing "unknown field" for nonexistent field — `WriteGoal` NOT called
- Returns error when `FindGoalByName` fails — error propagated
- Returns error when `WriteGoal` fails — error propagated

Add `Describe("NewGoalListRemoveOperation")`:
- Successfully removes value from tags field → `WriteGoal` called with updated tags
- Returns error containing "not found" when value absent from list — `WriteGoal` NOT called
- Returns error containing "not a list field" for scalar field — `WriteGoal` NOT called
- Returns error containing "unknown field" for nonexistent field — `WriteGoal` NOT called
- Returns error when `FindGoalByName` fails
- Returns error when `WriteGoal` fails

Add `Describe("NewTaskListAddOperation")` with equivalent coverage for task (use `goals` field).
Add `Describe("NewTaskListRemoveOperation")` with equivalent coverage for task.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing `get`, `set`, `clear` commands must not change behavior — all current tests must pass
- `set` on a list field continues to replace the whole list — do NOT change `setFieldFromString`
- The `appendToList` / `removeFromList` helpers operate directly on the reflected slice field in-place (via `fieldVal.Set(...)`) — they do NOT return a new value
- Error messages must match exactly:
  - Scalar field: `"field %q is not a list field"` (e.g. `field "status" is not a list field`)
  - Duplicate add: `"value %q already exists in list"` (e.g. `value "foo" already exists in list`)
  - Missing remove: `"value %q not found in list"` (e.g. `value "foo" not found in list`)
  - Unknown field: `"unknown field %q for %s"` (reusing existing pattern)
- Use `github.com/bborbe/errors` for error wrapping — never `fmt.Errorf` for wrapping
- Use `fmt.Errorf` only for leaf errors that don't wrap other errors
- Multi-vault dispatch is handled by the CLI layer — these ops receive a single `vaultPath`
- Follow `go-testing.md` patterns: external test package (`package ops_test`), Ginkgo/Gomega, counterfeiter mocks from `mocks/`
- Test coverage for new code must be ≥80%
</constraints>

<verification>
Run `make precommit` — must pass.

Additional verification:
```bash
go build ./...
go test ./pkg/ops/...
go generate ./pkg/ops/...
```
</verification>
