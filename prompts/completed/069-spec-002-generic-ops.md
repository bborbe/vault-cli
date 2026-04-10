---
status: completed
spec: [002-generic-frontmatter-ops]
summary: Implemented reflection-based generic frontmatter get/set/clear/show operations for goal, theme, objective, and vision entities with counterfeiter mocks and ≥80% test coverage
container: vault-cli-069-spec-002-generic-ops
dark-factory-version: v0.57.5
created: "2026-03-17T10:00:00Z"
queued: "2026-03-17T10:30:14Z"
started: "2026-03-17T10:38:42Z"
completed: "2026-03-17T10:49:02Z"
branch: dark-factory/generic-frontmatter-ops
---

<summary>
- Get, set, and clear any frontmatter field on goals, themes, objectives, and visions
- Adding a new frontmatter field to an entity automatically makes it available via CLI
- Unknown field names return a clear error instead of silently failing
- Read-only metadata fields (name, file path, content) cannot be modified
- Type coercion handles strings, dates, integers, and string lists automatically
- Show command displays all frontmatter fields in plain text and JSON format
- All new code has test coverage above 80%
</summary>

<objective>
Build generic reflection-based frontmatter operations for goal, theme, objective, and vision entities so that adding a new field to a domain struct automatically makes it available via CLI — with no hardcoded field switch statements per entity type.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read these files before making changes (read ALL of them first):

- `pkg/ops/frontmatter.go` — existing task-specific frontmatter ops (switch-based); the new code must NOT use this pattern for other entities
- `pkg/ops/show.go` — existing task ShowOperation pattern to follow for entity show ops
- `pkg/domain/goal.go` — Goal struct with yaml tags; reflect on these
- `pkg/domain/theme.go` — Theme struct with yaml tags
- `pkg/domain/objective.go` — Objective struct (created in prompt 1)
- `pkg/domain/vision.go` — Vision struct (created in prompt 1)
- `pkg/storage/storage.go` — GoalStorage, ThemeStorage, ObjectiveStorage, VisionStorage interfaces
- `pkg/ops/frontmatter_test.go` — existing test patterns (Ginkgo/Gomega, counterfeiter mocks)
- `pkg/ops/show_test.go` — existing ShowOperation test pattern
- `mocks/` directory — counterfeiter-generated mocks to use in tests
</context>

<requirements>

## 1. Create `pkg/ops/frontmatter_reflect.go` — generic reflection helpers

This file contains unexported helpers used by all entity frontmatter ops.

```go
package ops

import (
    "context"
    "fmt"
    "reflect"
    "strconv"
    "strings"
    "time"

    "github.com/bborbe/errors"
)

// fieldByYAMLTag finds a struct field by its yaml tag name.
// Returns the field, a pointer to its value, and whether it was found.
// If the yaml tag is "-", the field is metadata (read-only).
func fieldByYAMLTag(entityPtr any, tagName string) (reflect.StructField, reflect.Value, bool) {
    v := reflect.ValueOf(entityPtr).Elem()
    t := v.Type()
    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        yamlTag := field.Tag.Get("yaml")
        // Strip options like ",omitempty"
        name := strings.Split(yamlTag, ",")[0]
        if name == tagName {
            return field, v.Field(i), true
        }
    }
    return reflect.StructField{}, reflect.Value{}, false
}

// getFieldAsString reads a struct field value as a string.
// Handles: string, string-alias, int-alias (Priority), *time.Time, []string.
func getFieldAsString(fieldVal reflect.Value) (string, error) {
    if !fieldVal.IsValid() {
        return "", nil
    }
    switch fieldVal.Kind() {
    case reflect.String:
        return fieldVal.String(), nil
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        return strconv.FormatInt(fieldVal.Int(), 10), nil
    case reflect.Slice:
        if fieldVal.IsNil() {
            return "", nil
        }
        strs := make([]string, fieldVal.Len())
        for i := 0; i < fieldVal.Len(); i++ {
            strs[i] = fieldVal.Index(i).String()
        }
        return strings.Join(strs, ","), nil
    case reflect.Ptr:
        if fieldVal.IsNil() {
            return "", nil
        }
        // Handle *time.Time
        if t, ok := fieldVal.Interface().(*time.Time); ok {
            return t.Format("2006-01-02"), nil
        }
        return "", fmt.Errorf("unsupported pointer type: %s", fieldVal.Type())
    default:
        return "", fmt.Errorf("unsupported field type: %s", fieldVal.Kind())
    }
}

// setFieldFromString sets a struct field from a string value.
// Type coercion is based on the field's reflect.Kind and type.
func setFieldFromString(ctx context.Context, fieldVal reflect.Value, fieldType reflect.Type, value string) error {
    switch fieldVal.Kind() {
    case reflect.String:
        fieldVal.SetString(value)
        return nil
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        n, err := strconv.ParseInt(value, 10, 64)
        if err != nil {
            return errors.Wrap(ctx, err, "invalid integer value")
        }
        fieldVal.SetInt(n)
        return nil
    case reflect.Slice:
        if value == "" {
            fieldVal.Set(reflect.Zero(fieldType))
            return nil
        }
        parts := strings.Split(value, ",")
        slice := reflect.MakeSlice(fieldType, len(parts), len(parts))
        for i, p := range parts {
            slice.Index(i).SetString(p)
        }
        fieldVal.Set(slice)
        return nil
    case reflect.Ptr:
        if value == "" {
            fieldVal.Set(reflect.Zero(fieldType))
            return nil
        }
        // Handle *time.Time
        if fieldType == reflect.TypeOf((*time.Time)(nil)) {
            t, err := time.Parse("2006-01-02", value)
            if err != nil {
                return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
            }
            fieldVal.Set(reflect.ValueOf(&t))
            return nil
        }
        return fmt.Errorf("unsupported pointer type: %s", fieldType)
    default:
        return fmt.Errorf("unsupported field type: %s", fieldVal.Kind())
    }
}

// clearField zeros a struct field.
func clearField(fieldVal reflect.Value, fieldType reflect.Type) {
    fieldVal.Set(reflect.Zero(fieldType))
}

// isReadOnlyTag returns true if the yaml tag marks the field as metadata (yaml:"-").
func isReadOnlyTag(field reflect.StructField) bool {
    return field.Tag.Get("yaml") == "-"
}
```

## 2. Create `pkg/ops/frontmatter_entity.go` — entity frontmatter operations

This file defines four generic operation interfaces (EntityGet, EntitySet, EntityClear, EntityShow) plus entity-specific constructors for goal, theme, objective, and vision.

### EntityGetOperation

```go
//counterfeiter:generate -o ../../mocks/entity-get-operation.go --fake-name EntityGetOperation . EntityGetOperation
type EntityGetOperation interface {
    Execute(ctx context.Context, vaultPath, entityName, key string) (string, error)
}
```

Implementation uses the reflection helpers from `frontmatter_reflect.go`:
```go
type entityGetOperation struct {
    findFn     func(ctx context.Context, vaultPath, name string) (any, error)
    entityType string // e.g. "goal", used in error messages
}

func (o *entityGetOperation) Execute(ctx context.Context, vaultPath, entityName, key string) (string, error) {
    entity, err := o.findFn(ctx, vaultPath, entityName)
    if err != nil {
        return "", errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
    }
    field, fieldVal, found := fieldByYAMLTag(entity, key)
    if !found {
        return "", fmt.Errorf("unknown field %q for %s", key, o.entityType)
    }
    if isReadOnlyTag(field) {
        return "", fmt.Errorf("field %q is read-only", key)
    }
    return getFieldAsString(fieldVal)
}
```

Constructor functions for each entity:
```go
func NewGoalGetOperation(goalStorage storage.GoalStorage) EntityGetOperation {
    return &entityGetOperation{
        findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
            return goalStorage.FindGoalByName(ctx, vaultPath, name)
        },
        entityType: "goal",
    }
}

func NewThemeGetOperation(themeStorage storage.ThemeStorage) EntityGetOperation { ... }
func NewObjectiveGetOperation(objectiveStorage storage.ObjectiveStorage) EntityGetOperation { ... }
func NewVisionGetOperation(visionStorage storage.VisionStorage) EntityGetOperation { ... }
```

### EntitySetOperation

```go
//counterfeiter:generate -o ../../mocks/entity-set-operation.go --fake-name EntitySetOperation . EntitySetOperation
type EntitySetOperation interface {
    Execute(ctx context.Context, vaultPath, entityName, key, value string) error
}
```

Implementation:
```go
type entitySetOperation struct {
    findFn     func(ctx context.Context, vaultPath, name string) (any, error)
    writeFn    func(ctx context.Context, entity any) error
    entityType string
}

func (o *entitySetOperation) Execute(ctx context.Context, vaultPath, entityName, key, value string) error {
    entity, err := o.findFn(ctx, vaultPath, entityName)
    if err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
    }
    field, fieldVal, found := fieldByYAMLTag(entity, key)
    if !found {
        return fmt.Errorf("unknown field %q for %s", key, o.entityType)
    }
    if isReadOnlyTag(field) {
        return fmt.Errorf("field %q is read-only", key)
    }
    if err := setFieldFromString(ctx, fieldVal, field.Type, value); err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
    }
    if err := o.writeFn(ctx, entity); err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("write %s", o.entityType))
    }
    return nil
}
```

Constructor functions for each entity:
```go
func NewGoalSetOperation(goalStorage storage.GoalStorage) EntitySetOperation {
    return &entitySetOperation{
        findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
            return goalStorage.FindGoalByName(ctx, vaultPath, name)
        },
        writeFn: func(ctx context.Context, entity any) error {
            return goalStorage.WriteGoal(ctx, entity.(*domain.Goal))
        },
        entityType: "goal",
    }
}

func NewThemeSetOperation(themeStorage storage.ThemeStorage) EntitySetOperation { ... }
func NewObjectiveSetOperation(objectiveStorage storage.ObjectiveStorage) EntitySetOperation { ... }
func NewVisionSetOperation(visionStorage storage.VisionStorage) EntitySetOperation { ... }
```

### EntityClearOperation

```go
//counterfeiter:generate -o ../../mocks/entity-clear-operation.go --fake-name EntityClearOperation . EntityClearOperation
type EntityClearOperation interface {
    Execute(ctx context.Context, vaultPath, entityName, key string) error
}
```

Same pattern as Set but calls `clearField` instead of `setFieldFromString`. Constructor functions for goal, theme, objective, vision.

### EntityShowOperation

```go
//counterfeiter:generate -o ../../mocks/entity-show-operation.go --fake-name EntityShowOperation . EntityShowOperation
type EntityShowOperation interface {
    Execute(ctx context.Context, vaultPath, vaultName, entityName, outputFormat string) error
}
```

Implementation uses reflection to enumerate all non-metadata fields:
```go
type entityShowOperation struct {
    findFn     func(ctx context.Context, vaultPath, name string) (any, error)
    entityType string
}

func (o *entityShowOperation) Execute(ctx context.Context, vaultPath, vaultName, entityName, outputFormat string) error {
    entity, err := o.findFn(ctx, vaultPath, entityName)
    if err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
    }

    // Build field map from struct using reflection
    v := reflect.ValueOf(entity).Elem()
    t := v.Type()
    fields := make(map[string]string)
    var fieldOrder []string
    for i := 0; i < t.NumField(); i++ {
        sf := t.Field(i)
        yamlTag := sf.Tag.Get("yaml")
        name := strings.Split(yamlTag, ",")[0]
        if name == "" || name == "-" {
            continue // skip metadata fields
        }
        val, _ := getFieldAsString(v.Field(i))
        fields[name] = val
        fieldOrder = append(fieldOrder, name)
    }

    // Get metadata via separate struct fields (Name, FilePath, Content)
    nameVal := v.FieldByName("Name").String()
    filePathVal := v.FieldByName("FilePath").String()
    contentVal := v.FieldByName("Content").String()

    if outputFormat == "json" {
        result := map[string]any{
            "name":      nameVal,
            "file_path": filePathVal,
            "vault":     vaultName,
            "fields":    fields,
            "content":   contentVal,
        }
        data, err := json.Marshal(result)
        if err != nil {
            return errors.Wrap(ctx, err, "marshal json")
        }
        fmt.Println(string(data))
        return nil
    }

    // Plain output
    fmt.Printf("%s: %s\n", o.entityType, nameVal)
    for _, name := range fieldOrder {
        if fields[name] != "" {
            fmt.Printf("%s: %s\n", name, fields[name])
        }
    }
    return nil
}
```

Constructor functions for goal, theme, objective, vision.

## 3. Generate mocks

Run:
```
go generate ./pkg/ops/...
```

This creates:
- `mocks/entity-get-operation.go`
- `mocks/entity-set-operation.go`
- `mocks/entity-clear-operation.go`
- `mocks/entity-show-operation.go`

## 4. Write tests in `pkg/ops/frontmatter_entity_test.go`

Use Ginkgo/Gomega with counterfeiter mocks for GoalStorage (from `mocks/goal-storage.go`).

Test `NewGoalGetOperation`:
- Returns string value for known field (e.g. `status` → "active")
- Returns comma-joined string for `tags` field
- Returns empty string for unset optional field (no error)
- Returns error containing `"unknown field"` for unknown key (e.g. `"xyz"`)
- Returns error containing `"read-only"` for metadata key (e.g. `"name"`) — test this by calling get with `"name"` as key, BUT NOTE: `name` field has `yaml:"-"` so the tag is literally `"-"`, not `"name"`. The field lookup by tag name `"name"` will return not-found → "unknown field" error. Only if someone passes the literal tag value `"-"` would it hit `isReadOnlyTag`. For the error message "field is read-only", test with the set operation instead.
- Returns error when FindGoalByName fails

Test `NewGoalSetOperation`:
- Sets a string field (e.g. `status`) and calls WriteGoal with updated value
- Sets a `*time.Time` date field from YYYY-MM-DD string
- Returns error for invalid date format
- Sets `[]string` field from comma-separated value
- Sets nil slice for empty string value
- Returns error for unknown field
- Returns error when FindGoalByName fails
- Returns error when WriteGoal fails

Test `NewGoalClearOperation`:
- Clears string field (sets to "")
- Clears pointer field (sets to nil)
- Clears slice field (sets to nil)
- Returns error for unknown field

Test `EntityShowOperation` for goal:
- JSON output contains name, fields map, vault
- Plain output prints entity name and fields
- Returns error when find fails

Test the reflect helpers in `pkg/ops/frontmatter_reflect_test.go`:
- `fieldByYAMLTag`: finds field by name, returns not-found for missing name
- `getFieldAsString`: string, int, slice, nil pointer, non-nil pointer (*time.Time)
- `setFieldFromString`: string, int, slice, nil on empty string, *time.Time, error on bad date
- `clearField`: zeros value

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing task frontmatter.go and its tests must NOT be changed — task ops keep their switch-based approach
- All existing tests must pass unchanged
- The generic ops must use reflection on yaml struct tags — no hardcoded field maps or switch statements for the entity-specific ops
- Entity names (goal, theme, objective, vision) come from the `entityType` field in error messages; exact format: `"unknown field %q for %s"` and `"field %q is read-only"`
- The `findFn` closures use type assertions (`entity.(*domain.Goal)`) which is acceptable for internal implementation
- Do NOT use `strings.Title` if the linter flags it — use `cases.Title` from `golang.org/x/text/cases` or just lowercase entity type names in output
- Use `github.com/bborbe/errors` for error wrapping — never `fmt.Errorf` for wrapping
- Test coverage for new packages must be ≥80%
- Follow `go-testing.md` patterns: external test package (`package ops_test`), Ginkgo/Gomega, counterfeiter mocks from `mocks/`
</constraints>

<verification>
Run `make precommit` — must pass.

Additional verification:
- `go build ./...` compiles with no errors
- `go test ./pkg/ops/...` passes
- `go generate ./pkg/ops/...` succeeds (no errors)
</verification>
