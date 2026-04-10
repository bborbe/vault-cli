---
status: completed
spec: [008-flexible-frontmatter-refactor]
container: vault-cli-101-spec-008-task-migration
dark-factory-version: v0.108.0-dirty
created: "2026-04-10T00:00:00Z"
queued: "2026-04-10T21:45:56Z"
started: "2026-04-10T21:51:37Z"
completed: "2026-04-10T22:26:25Z"
---

<summary>
- `domain.Task` is restructured into three separate concerns: `TaskFrontmatter` (typed map wrapper), `FileMetadata` (filesystem info), and `Content` (named markdown body type)
- `TaskFrontmatter` exposes typed getter methods for all 15 known fields (Status, Priority, Goals, etc.) and a generic `GetField`/`SetField`/`ClearField` API for arbitrary keys
- A new `Priority.Validate(ctx) error` method rejects negative values and is called by `SetPriority` to enforce spec AC #6 ("Validate() rejects invalid known field values: negative priority")
- Unknown frontmatter fields set via `SetField("custom_key", "value")` round-trip correctly through read-write cycles without data loss
- `WriteTask` uses the map-based serializer — all fields in the map (known and unknown) are written to YAML frontmatter
- `ReadTask` / `ListTasks` / `FindTaskByName` use the map-based parser — all frontmatter fields land in the task's internal map
- Task status normalization (legacy aliases → canonical values) moves from `UnmarshalYAML` to the `Status()` getter
- UUID auto-generation on write is preserved (TaskIdentifier is set before serialization)
- The hardcoded switch in `pkg/ops/frontmatter.go` for task get/set is replaced with calls to `task.GetField` / `task.SetField`
- All ops files that read task struct fields are updated to use method calls (`task.Status()`, etc.)
- All existing task tests are updated to the new accessor pattern; all tests pass
</summary>

<objective>
Migrate `domain.Task` from a struct with YAML-tagged fields to a type that embeds `TaskFrontmatter` (a `FrontmatterMap`-backed typed wrapper) and `FileMetadata`. Update `pkg/storage/task.go` to parse and serialize via the map. Update all ops and CLI code that accesses Task fields. After this prompt, `vault-cli task get/set` work for both known and unknown fields, and unknown fields survive round-trips.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read the relevant coding guides surfaced by the `coding` plugin: `go-error-wrapping-guide.md`, `go-testing-guide.md`, `go-composition.md`, `go-enum-type-pattern.md`.

**Prompt 1 must be completed first.** This prompt depends on:
- `domain.FrontmatterMap` type existing in `pkg/domain/frontmatter_map.go`
- `domain.FileMetadata` type existing in `pkg/domain/file_metadata.go`
- `domain.Content` named string type existing in `pkg/domain/content.go`
- `baseStorage.parseToFrontmatterMap` and `baseStorage.serializeMapAsFrontmatter` existing in `pkg/storage/base.go`

Key files to read in full before making changes:

**Domain layer**
- `pkg/domain/task.go` — current Task struct with all YAML-tagged fields (will be replaced); also contains `TaskStatus` type, constants, `NormalizeTaskStatus`, and `(*TaskStatus).UnmarshalYAML` (line 132)
- `pkg/domain/task_status_test.go` — line 179 has `Describe("UnmarshalYAML", …)` for TaskStatus (must be deleted; see requirement 8)
- `pkg/domain/priority.go` — `Priority.UnmarshalYAML` stays; a new `Priority.Validate(ctx) error` method is added (requirement 1a)
- `pkg/domain/priority_test.go` — tests for `Priority.UnmarshalYAML` stay; add tests for new `Priority.Validate`
- `pkg/domain/frontmatter_map.go` — FrontmatterMap type (from Prompt 1)
- `pkg/domain/file_metadata.go` — FileMetadata type (from Prompt 1)
- `pkg/domain/content.go` — Content named string type (from Prompt 1)
- `pkg/domain/domain_suite_test.go` — already exists, uses `TestSuite` / `"domain Test Suite"`

**Storage layer**
- `pkg/storage/task.go` — ReadTask, WriteTask, ListTasks, FindTaskByName (will be updated)
- `pkg/storage/base.go` — `parseToFrontmatterMap`, `serializeMapAsFrontmatter` (from Prompt 1); also `readTaskFromPath` at line 142 (**shared helper, called from `page.go` and `task.go`**)
- `pkg/storage/page.go` — uses `readTaskFromPath`; must keep compiling
- `pkg/storage/task_test.go` — uses `task.Status`/etc directly; must be migrated
- `pkg/storage/markdown_test.go` — uses `task.Status`/`task.Phase`/etc directly; must be migrated

**Ops layer — all files reading/writing task struct fields (verified via grep)**
- `pkg/ops/frontmatter.go` — **CONTAINS THE HARDCODED SWITCH** targeted by requirement 6 (see `frontmatterGetOperation.Execute` line 47, `frontmatterSetOperation.Execute`, `frontmatterClearOperation.Execute`)
- `pkg/ops/frontmatter_test.go` — tests for the above
- `pkg/ops/frontmatter_entity.go` — `NewTaskListAddOperation` (line 430), `NewTaskListRemoveOperation` (line 527); both currently route through `entityListOperation` → `fieldByYAMLTag` (breaks once Task loses YAML tags — see requirement 5)
- `pkg/ops/frontmatter_reflect.go` — `fieldByYAMLTag` helper (stays — used by non-task entities; DO NOT delete, Prompt 4 handles it)
- `pkg/ops/complete.go` + `complete_test.go` — `task.Status`, `task.CompletedDate`
- `pkg/ops/defer.go` + `defer_test.go` — `task.DeferDate`, `task.Status`
- `pkg/ops/workon.go` + `workon_test.go` — `task.Assignee`, `task.ClaudeSessionID`, `task.Status`
- `pkg/ops/list.go` — task fields for filtering
- `pkg/ops/show.go` + `show_test.go` — task fields for display
- `pkg/ops/update.go` + `update_test.go` — `task.Status`, `task.Goals`
- `pkg/ops/ensure_task_identifiers.go` — `task.TaskIdentifier`
- `pkg/ops/goal_complete.go` — reads `task.Status`, `task.Goals` (walks linked tasks)

**CLI layer**
- `pkg/cli/cli.go` — task `get`/`set` commands already delegate to `ops.NewFrontmatterGetOperation`/etc. NO hardcoded switch lives here (the switch is in `pkg/ops/frontmatter.go`). CLI only needs changes if a currently-failing known field check is found.
</context>

<requirements>
### 0. Add `Priority.Validate` to `pkg/domain/priority.go`

Spec acceptance criterion #6 requires: *"`Validate()` rejects invalid known field values (bad status, **negative priority**)."* The current `Priority` type has no `Validate` method. Add one:

```go
// Validate returns an error if the priority value is invalid.
// Valid priorities are non-negative integers (0 and up).
// The sentinel value -1 (used by UnmarshalYAML for unparseable YAML values)
// is treated as invalid here because any explicit SetPriority call with a
// negative value is a user error.
func (p Priority) Validate(ctx context.Context) error {
    if p < 0 {
        return errors.Errorf(ctx, "priority must be >= 0, got %d", int(p))
    }
    return nil
}
```

Add the required imports to `pkg/domain/priority.go`:
```go
import (
    "context"

    "github.com/bborbe/errors"
    "gopkg.in/yaml.v3"
)
```

This method is used by `TaskFrontmatter.SetPriority` (requirement 1) and by the Goal/Theme/Objective/Vision setters in Prompt 3. The `UnmarshalYAML` sentinel path (`-1`) is unchanged — read side remains non-fatal; validation fires only on the write side via `SetPriority`.

Add a test in `pkg/domain/priority_test.go` covering:
- `Priority(0).Validate(ctx)` returns nil
- `Priority(5).Validate(ctx)` returns nil
- `Priority(-1).Validate(ctx)` returns an error
- `Priority(-42).Validate(ctx)` returns an error

### 1. Create `pkg/domain/task_frontmatter.go`

This file defines `TaskFrontmatter` — the per-task typed map wrapper with accessors for all known frontmatter fields.

The type embeds `FrontmatterMap` and exposes:
- Typed getters that read from the map and coerce to the appropriate Go type
- Typed setters that validate then write to the map
- Generic `GetField(key string) string` and `SetField(ctx, key, value string) error` for the CLI get/set commands
- `ClearField(key string)` to remove any field

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/bborbe/errors"
    libtime "github.com/bborbe/time"
)

// TaskFrontmatter holds the YAML frontmatter for a Task.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type TaskFrontmatter struct {
    FrontmatterMap
}

// NewTaskFrontmatter constructs a TaskFrontmatter from a raw map.
func NewTaskFrontmatter(data map[string]any) TaskFrontmatter {
    return TaskFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}
```

#### Typed getters — implement each of the following:

**Status** — reads `"status"` key, applies `NormalizeTaskStatus`. Returns `""` (empty) if value is absent or unrecognized.
```go
func (f TaskFrontmatter) Status() TaskStatus {
    raw := f.GetString("status")
    normalized, ok := NormalizeTaskStatus(raw)
    if !ok {
        return ""
    }
    return normalized
}
```

**PageType** — reads `"page_type"` key, returns string.
```go
func (f TaskFrontmatter) PageType() string { return f.GetString("page_type") }
```

**Goals** — reads `"goals"` key via `GetStringSlice`.
```go
func (f TaskFrontmatter) Goals() []string { return f.GetStringSlice("goals") }
```

**Priority** — reads `"priority"` key as int. Returns 0 on missing or parse failure.
```go
func (f TaskFrontmatter) Priority() Priority {
    v := f.Get("priority")
    if v == nil {
        return 0
    }
    switch p := v.(type) {
    case int:
        return Priority(p)
    case int64:
        return Priority(p)
    case float64:
        return Priority(int(p))
    case string:
        n, err := strconv.Atoi(p)
        if err != nil {
            return 0
        }
        return Priority(n)
    default:
        return 0
    }
}
```

**Assignee** — reads `"assignee"` key as string.
```go
func (f TaskFrontmatter) Assignee() string { return f.GetString("assignee") }
```

**DeferDate** — reads `"defer_date"` key, parses via `libtime.ParseTime`. Returns nil on missing or parse failure.
```go
func (f TaskFrontmatter) DeferDate() *DateOrDateTime {
    raw := f.GetString("defer_date")
    if raw == "" {
        return nil
    }
    t, err := libtime.ParseTime(context.Background(), raw)
    if err != nil {
        return nil
    }
    d := DateOrDateTime(*t)
    return &d
}
```

**Tags** — reads `"tags"` key via `GetStringSlice`.
```go
func (f TaskFrontmatter) Tags() []string { return f.GetStringSlice("tags") }
```

**Phase** — reads `"phase"` key as string, returns `*TaskPhase`.
```go
func (f TaskFrontmatter) Phase() *TaskPhase {
    raw := f.GetString("phase")
    if raw == "" {
        return nil
    }
    p := TaskPhase(raw)
    return &p
}
```

**ClaudeSessionID** — reads `"claude_session_id"` key as string.
```go
func (f TaskFrontmatter) ClaudeSessionID() string { return f.GetString("claude_session_id") }
```

**Recurring** — reads `"recurring"` key as string.
```go
func (f TaskFrontmatter) Recurring() string { return f.GetString("recurring") }
```

**LastCompleted** — reads `"last_completed"` key as string.
```go
func (f TaskFrontmatter) LastCompleted() string { return f.GetString("last_completed") }
```

**CompletedDate** — reads `"completed_date"` key as string.
```go
func (f TaskFrontmatter) CompletedDate() string { return f.GetString("completed_date") }
```

**PlannedDate** — reads `"planned_date"` key, parses via `libtime.ParseTime`. Same pattern as DeferDate.
```go
func (f TaskFrontmatter) PlannedDate() *DateOrDateTime { /* same as DeferDate but key "planned_date" */ }
```

**DueDate** — reads `"due_date"` key, same pattern.
```go
func (f TaskFrontmatter) DueDate() *DateOrDateTime { /* same as DeferDate but key "due_date" */ }
```

**TaskIdentifier** — reads `"task_identifier"` key as string.
```go
func (f TaskFrontmatter) TaskIdentifier() string { return f.GetString("task_identifier") }
```

#### Typed setters:

```go
func (f *TaskFrontmatter) SetStatus(s TaskStatus) error {
    if err := s.Validate(context.Background()); err != nil {
        return err
    }
    f.Set("status", string(s))
    return nil
}

func (f *TaskFrontmatter) SetPageType(v string)       { f.Set("page_type", v) }
func (f *TaskFrontmatter) SetGoals(v []string)         { f.Set("goals", stringSliceToAny(v)) }
func (f *TaskFrontmatter) SetAssignee(v string)        { f.Set("assignee", v) }
func (f *TaskFrontmatter) SetClaudeSessionID(v string) { f.Set("claude_session_id", v) }
func (f *TaskFrontmatter) SetRecurring(v string)       { f.Set("recurring", v) }
func (f *TaskFrontmatter) SetLastCompleted(v string)   { f.Set("last_completed", v) }
func (f *TaskFrontmatter) SetCompletedDate(v string)   { f.Set("completed_date", v) }
func (f *TaskFrontmatter) SetTaskIdentifier(v string)  { f.Set("task_identifier", v) }
func (f *TaskFrontmatter) SetTags(v []string)          { f.Set("tags", stringSliceToAny(v)) }

// SetPriority validates the priority and stores it in the map.
// Returns an error from Priority.Validate (added in requirement 0)
// when the value is negative, per spec AC #6.
func (f *TaskFrontmatter) SetPriority(ctx context.Context, p Priority) error {
    if err := p.Validate(ctx); err != nil {
        return errors.Wrap(ctx, err, "invalid priority")
    }
    f.Set("priority", int(p))
    return nil
}

func (f *TaskFrontmatter) SetPhase(p *TaskPhase) {
    if p == nil {
        f.Delete("phase")
        return
    }
    f.Set("phase", string(*p))
}

func (f *TaskFrontmatter) SetDeferDate(d *DateOrDateTime) {
    if d == nil {
        f.Delete("defer_date")
        return
    }
    f.Set("defer_date", formatDateOrDateTime(d))
}

func (f *TaskFrontmatter) SetPlannedDate(d *DateOrDateTime) {
    if d == nil {
        f.Delete("planned_date")
        return
    }
    f.Set("planned_date", formatDateOrDateTime(d))
}

func (f *TaskFrontmatter) SetDueDate(d *DateOrDateTime) {
    if d == nil {
        f.Delete("due_date")
        return
    }
    f.Set("due_date", formatDateOrDateTime(d))
}
```

Add these package-level helpers at the bottom of `task_frontmatter.go`:

```go
// formatDateOrDateTime serializes a DateOrDateTime to YYYY-MM-DD for date-only values
// and RFC3339 for values with a time component.
func formatDateOrDateTime(d *DateOrDateTime) string {
    if d == nil {
        return ""
    }
    t := d.Time().UTC()
    if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
        return t.Format(time.DateOnly)
    }
    return t.Format(time.RFC3339)
}

// stringSliceToAny converts []string to []any for map storage.
func stringSliceToAny(ss []string) []any {
    if ss == nil {
        return nil
    }
    result := make([]any, len(ss))
    for i, s := range ss {
        result[i] = s
    }
    return result
}
```

#### Generic access methods (used by CLI get/set):

```go
// GetField returns the string representation of any frontmatter field by key.
// Known fields return formatted values (status string, priority decimal, dates as YYYY-MM-DD etc.).
// Unknown fields return fmt.Sprintf("%v", rawValue).
// Returns "" if the key is absent.
func (f TaskFrontmatter) GetField(key string) string {
    switch key {
    case "status":
        return string(f.Status())
    case "page_type":
        return f.PageType()
    case "goals":
        return strings.Join(f.Goals(), ",")
    case "priority":
        p := f.Priority()
        if p == 0 {
            return ""
        }
        return strconv.Itoa(int(p))
    case "assignee":
        return f.Assignee()
    case "defer_date":
        return formatDateOrDateTime(f.DeferDate())
    case "tags":
        return strings.Join(f.Tags(), ",")
    case "phase":
        ph := f.Phase()
        if ph == nil {
            return ""
        }
        return string(*ph)
    case "claude_session_id":
        return f.ClaudeSessionID()
    case "recurring":
        return f.Recurring()
    case "last_completed":
        return f.LastCompleted()
    case "completed_date":
        return f.CompletedDate()
    case "planned_date":
        return formatDateOrDateTime(f.PlannedDate())
    case "due_date":
        return formatDateOrDateTime(f.DueDate())
    case "task_identifier":
        return f.TaskIdentifier()
    default:
        return f.GetString(key)
    }
}

// SetField sets a frontmatter field by key from a string value.
// Known fields apply type coercion and validation; unknown fields are stored as-is.
func (f *TaskFrontmatter) SetField(ctx context.Context, key, value string) error {
    switch key {
    case "status":
        return f.SetStatus(TaskStatus(value))
    case "page_type":
        f.SetPageType(value)
        return nil
    case "goals":
        if value == "" {
            f.SetGoals(nil)
        } else {
            f.SetGoals(strings.Split(value, ","))
        }
        return nil
    case "priority":
        if value == "" {
            f.Delete("priority")
            return nil
        }
        n, err := strconv.Atoi(value)
        if err != nil {
            return errors.Wrap(ctx, err, "priority must be an integer")
        }
        return f.SetPriority(ctx, Priority(n))
    case "assignee":
        f.SetAssignee(value)
        return nil
    case "defer_date":
        if value == "" {
            f.Delete("defer_date")
            return nil
        }
        t, err := libtime.ParseTime(ctx, value)
        if err != nil {
            return errors.Wrap(ctx, err, "invalid date format")
        }
        d := DateOrDateTime(*t)
        f.SetDeferDate(&d)
        return nil
    case "tags":
        if value == "" {
            f.SetTags(nil)
        } else {
            f.SetTags(strings.Split(value, ","))
        }
        return nil
    case "phase":
        if value == "" {
            f.SetPhase(nil)
            return nil
        }
        p := TaskPhase(value)
        f.SetPhase(&p)
        return nil
    case "claude_session_id":
        f.SetClaudeSessionID(value)
        return nil
    case "recurring":
        f.SetRecurring(value)
        return nil
    case "last_completed":
        f.SetLastCompleted(value)
        return nil
    case "completed_date":
        f.SetCompletedDate(value)
        return nil
    case "planned_date":
        if value == "" {
            f.Delete("planned_date")
            return nil
        }
        t, err := libtime.ParseTime(ctx, value)
        if err != nil {
            return errors.Wrap(ctx, err, "invalid date format")
        }
        d := DateOrDateTime(*t)
        f.SetPlannedDate(&d)
        return nil
    case "due_date":
        if value == "" {
            f.Delete("due_date")
            return nil
        }
        t, err := libtime.ParseTime(ctx, value)
        if err != nil {
            return errors.Wrap(ctx, err, "invalid date format")
        }
        d := DateOrDateTime(*t)
        f.SetDueDate(&d)
        return nil
    case "task_identifier":
        f.SetTaskIdentifier(value)
        return nil
    default:
        // Unknown field — store as string without validation
        f.Set(key, value)
        return nil
    }
}

// ClearField removes a frontmatter field by key.
// Works for both known and unknown fields.
func (f *TaskFrontmatter) ClearField(key string) {
    f.Delete(key)
}
```

### 2. Refactor `pkg/domain/task.go`

**⚠️ Shadowing danger zone**: Remove all YAML-tagged fields from `Task` in a single atomic edit. Do NOT leave the struct in a half-migrated state where both field `Phase` (from the old struct) and method `Phase()` (promoted from `TaskFrontmatter`) coexist — Go will report a compile error ("field and method with the same name"). Apply the full struct replacement in one Edit/Write operation.

Replace the entire `Task` struct definition (keep the `TaskStatus` type, constants, `AvailableTaskStatuses`, `NormalizeTaskStatus`, `IsValidTaskStatus`, `TaskStatuses`, `TaskID`, and their methods — do NOT remove those). Only change the `Task` struct itself:

**Remove** all YAML-tagged frontmatter fields from `Task`.
**Remove** the `Name`, `Content`, `FilePath`, `ModifiedDate` metadata fields from `Task`.
**Keep** `TaskStatus`, its constants, `NormalizeTaskStatus`, etc. (they are now used by `TaskFrontmatter.Status()`).
**Remove** the `UnmarshalYAML` method on `TaskStatus` — normalization now happens in `TaskFrontmatter.Status()` instead.

The new `Task` struct:

```go
// Task represents a task in the Obsidian vault.
// Frontmatter is stored in TaskFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Task struct {
    TaskFrontmatter
    FileMetadata
    // Content is the full markdown content including the frontmatter block.
    // It is used by the storage layer to extract the markdown body on write.
    Content Content
}
```

**Also add a convenience constructor** that is used by the storage layer:

```go
// NewTask creates a Task from a parsed frontmatter map and metadata.
func NewTask(data map[string]any, meta FileMetadata, content Content) *Task {
    return &Task{
        TaskFrontmatter: NewTaskFrontmatter(data),
        FileMetadata:    meta,
        Content:         content,
    }
}
```

Note: `Task.Content` is the `domain.Content` named string type (from Prompt 1), NOT raw `string`. Call sites that previously passed a `string` must convert with `domain.Content(...)`. Call sites that passed `task.Content` to APIs expecting raw strings must convert with `string(task.Content)` or call `task.Content.String()`.

### 3. Update `pkg/storage/base.go` — replace `readTaskFromPath`

The `readTaskFromPath(ctx, filePath, name string) (*domain.Task, error)` helper lives in `pkg/storage/base.go` at line 142. It is shared — called from 4 sites in `pkg/storage/task.go` (lines 33, 40, 72, 95) and 1 site in `pkg/storage/page.go` (line 49). **Do NOT move it** — rewrite it in place so all 5 callers continue to work.

The new implementation must:

1. Read the file bytes with `os.ReadFile(filePath)`
2. Populate `FileMetadata` (`Name`, `FilePath`, `ModifiedDate` from `os.Stat`)
3. Call `b.parseToFrontmatterMap(ctx, content)` to get `map[string]any`
4. Call `domain.NewTask(data, meta, domain.Content(content))` to construct the task (wrap the `[]byte` as `domain.Content` via `string` conversion)
5. Return the task or wrapped error

Delete any lingering references in this helper to the old struct-based parse path. The helper signature stays exactly the same — all 4 call sites compile unchanged.

### 3a. Update `pkg/storage/task.go` — `WriteTask`

`WriteTask` must:

1. Ensure `TaskIdentifier` is set before serialization: `if task.TaskIdentifier() == "" { task.SetTaskIdentifier(uuid.New().String()) }`
2. Call `t.serializeMapAsFrontmatter(ctx, task.RawMap(), task.Content)` — `RawMap()` is promoted through `TaskFrontmatter → FrontmatterMap`, so `task.RawMap()` resolves at compile time as long as neither `Task` nor `TaskFrontmatter` shadows the name (they don't in this design)
3. Write the result to `task.FilePath` with mode `0600`

The full updated `WriteTask`:
```go
func (t *taskStorage) WriteTask(ctx context.Context, task *domain.Task) error {
    if task.TaskIdentifier() == "" {
        task.SetTaskIdentifier(uuid.New().String())
    }

    content, err := t.serializeMapAsFrontmatter(ctx, task.RawMap(), string(task.Content))
    if err != nil {
        return errors.Wrap(ctx, err, "serialize frontmatter")
    }

    if err := os.WriteFile(task.FilePath, []byte(content), 0600); err != nil {
        return errors.Wrapf(ctx, err, "write file %s", task.FilePath)
    }

    return nil
}
```

Notes:
- `serializeMapAsFrontmatter` takes `originalContent string` (Prompt 1 signature), so convert `task.Content` via `string(task.Content)`.
- Use `errors.Wrapf` (not `errors.Wrap` + `fmt.Sprintf`) per `go-error-wrapping-guide.md`.

### 4. Update all ops files that access Task struct fields

Read each file listed in the `<context>` section. Find every reference to old field-access patterns like `task.Status`, `task.Priority`, `task.DeferDate`, etc. Replace with method calls: `task.Status()`, `task.Priority()`, `task.DeferDate()`, etc.

**Assignment patterns** (`task.Status = x`) must change to setter calls (`task.SetStatus(x)` or for the generic path `task.SetField(ctx, "status", "...")`).

Common patterns to find and fix:
- `task.Status == TaskStatusXxx` → `task.Status() == TaskStatusXxx`
- `task.Status = TaskStatusXxx` → `err := task.SetStatus(domain.TaskStatusXxx)`
- `task.DeferDate = &d` → `task.SetDeferDate(&d)`
- `task.Phase = &p` → `task.SetPhase(&p)`
- `task.Phase == nil` → `task.Phase() == nil`
- `task.Phase.String()` → `task.Phase().String()` (dereference via the getter)
- `task.CompletedDate = "..."` → `task.SetCompletedDate("...")`
- `task.ClaudeSessionID = "..."` → `task.SetClaudeSessionID("...")`
- `task.TaskIdentifier` → `task.TaskIdentifier()`
- `task.Goals` → `task.Goals()`
- `task.Recurring` → `task.Recurring()`
- `task.LastCompleted` → `task.LastCompleted()`
- `task.Assignee` → `task.Assignee()`

Pay special attention to `pkg/ops/goal_complete.go`: it walks linked tasks via `task.Status` and `task.Goals` (lines around 119, 138) — both must become method calls.

Metadata fields (`task.Name`, `task.FilePath`, `task.ModifiedDate`) continue to work unchanged via embedding. `task.Content` also continues to work as a direct field access but its type changed from `string` to `domain.Content` — any site that passes `task.Content` to an API expecting `string` must add an explicit `string(task.Content)` conversion.

### 5. Replace task list operations in `pkg/ops/frontmatter_entity.go`

**Critical**: `NewTaskListAddOperation` (line 430) and `NewTaskListRemoveOperation` (line 527) currently return `&entityListOperation{...}`, whose `Execute` method (line 324) calls `fieldByYAMLTag(task, key)` via reflection. Once Task loses its YAML tags, `fieldByYAMLTag` returns `(_, _, false)` for `goals`/`tags` and the operations silently stop working (no compile error, just runtime failure).

Create a new task-specific list operation struct in `pkg/ops/frontmatter_entity.go` (or a new file `pkg/ops/task_list_operation.go` — one type per file preferred):

```go
type taskListOperation struct {
    taskStorage storage.TaskStorage
    mode        string // "add" or "remove"
}

func (o *taskListOperation) Execute(
    ctx context.Context,
    vaultPath, taskName, key, value string,
) error {
    task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
    if err != nil {
        return errors.Wrap(ctx, err, "find task")
    }

    var current []string
    switch key {
    case "goals":
        current = task.Goals()
    case "tags":
        current = task.Tags()
    default:
        return errors.Errorf(ctx, "unsupported list field %q", key)
    }

    updated := applyListMutation(current, value, o.mode)

    switch key {
    case "goals":
        task.SetGoals(updated)
    case "tags":
        task.SetTags(updated)
    }

    if err := o.taskStorage.WriteTask(ctx, task); err != nil {
        return errors.Wrap(ctx, err, "write task")
    }
    return nil
}
```

Provide an `applyListMutation` helper with signature:
```go
func applyListMutation(current []string, value, mode string) []string
```
For `mode == "add"`: return `current` with `value` appended if and only if not already present.
For `mode == "remove"`: return `current` with all occurrences of `value` filtered out.
Do NOT reuse the existing `appendToList`/`removeFromList` in `frontmatter_reflect.go` — those take `reflect.Value` and are removed in Prompt 4.

Rewrite `NewTaskListAddOperation` and `NewTaskListRemoveOperation` to return `&taskListOperation{taskStorage: taskStorage, mode: "add"}` / `"remove"` respectively.

**Do NOT change** `NewGoalListAddOperation`, `NewThemeListAddOperation`, etc. — those still use `entityListOperation` with reflection and are migrated in Prompt 3. The shared `entityListOperation` struct and `fieldByYAMLTag` helper remain in place.

### 6. Replace the hardcoded switch in `pkg/ops/frontmatter.go`

**Correction**: the hardcoded switch for task `get`/`set`/`clear` is in `pkg/ops/frontmatter.go`, NOT `pkg/cli/cli.go`. The CLI already delegates to `ops.NewFrontmatterGetOperation` / `NewFrontmatterSetOperation` / `NewFrontmatterClearOperation`. Update the ops layer only.

Target file: `pkg/ops/frontmatter.go`
- `frontmatterGetOperation.Execute` (line 47): currently a `switch key { case "phase": ... case "status": ... }`. Replace the entire switch body with `return task.GetField(key), nil` (after the `FindTaskByName` call).
- `frontmatterSetOperation.Execute`: replace the switch with `if err := task.SetField(ctx, key, value); err != nil { return errors.Wrap(ctx, err, "set field") }` followed by `return o.taskStorage.WriteTask(ctx, task)`.
- `frontmatterClearOperation.Execute`: replace with `task.ClearField(key)` followed by `WriteTask`.

If any existing validation on the set path rejects unknown keys (returns "unknown field"), remove it — unknown fields must pass through to `SetField`'s default branch so `vault-cli task set <name> custom_key value` succeeds.

Update `pkg/ops/frontmatter_test.go` to:
- Add a test case proving unknown-field set/get round-trips (`custom_field: my-value`)
- Remove any test assertion that previously required the "unknown field" error

`pkg/cli/cli.go` requires NO changes for this requirement — it already delegates correctly.

### 7. Update tests

Every test file that constructs `domain.Task` literals or reads/writes task struct fields must be migrated. Replace `task.Field` (read) with `task.Field()` (method call), and direct assignments with setter methods. For test literals, use `domain.NewTask(map[string]any{...}, domain.FileMetadata{...}, domain.Content(content))` or construct a `TaskFrontmatter` directly. Note the `domain.Content(...)` conversion — the `Content` parameter is the named type, not raw `string`.

Files to migrate:
- `pkg/storage/task_test.go`
- `pkg/storage/markdown_test.go`
- `pkg/ops/complete_test.go`
- `pkg/ops/defer_test.go`
- `pkg/ops/workon_test.go`
- `pkg/ops/show_test.go`
- `pkg/ops/update_test.go`
- `pkg/ops/list_test.go`
- `pkg/ops/ensure_task_identifiers_test.go`
- `pkg/ops/goal_complete_test.go`
- `pkg/ops/frontmatter_test.go`
- `pkg/ops/frontmatter_entity_test.go` (task-related cases only; other entities stay until Prompt 3)

Before declaring done, run this sweep to catch any `domain.Task{}` struct literal that still sets removed fields:
```bash
grep -rn 'domain\.Task{' pkg/ integration/ --include='*.go'
```
Every match must use `domain.NewTask(map[string]any{...}, domain.FileMetadata{...}, domain.Content(...))` or explicit embedding of `domain.TaskFrontmatter`.

### 8. Delete the `TaskStatus.UnmarshalYAML` test block

`pkg/domain/task_status_test.go` line 179 contains `Describe("UnmarshalYAML", func() { ... })`. Since `TaskStatus.UnmarshalYAML` is removed (constraint), this describe block must also be deleted. Replace it with coverage for `NormalizeTaskStatus` directly if not already covered elsewhere.

**Do NOT touch** `pkg/domain/priority_test.go` — `Priority.UnmarshalYAML` stays (Priority is not refactored in this prompt).

### 9. Add tests in `pkg/domain/task_frontmatter_test.go`

New test file covering:
- `SetField("custom_key", "value")` → `GetField("custom_key") == "value"` (unknown field round-trip)
- `SetField("status", "banana")` returns an error
- `SetField("status", "todo")` succeeds, `Status() == TaskStatusTodo`
- `SetField("status", "next")` succeeds with normalization (legacy alias → `TaskStatusTodo`)
- `SetField("priority", "5")` succeeds, `Priority() == 5`
- `SetField("priority", "-1")` returns an error (spec AC #6: negative priority rejected)
- `SetField("priority", "-42")` returns an error
- `SetField("defer_date", "2026-04-15")` sets the defer date; `GetField("defer_date") == "2026-04-15"`
- Known field getters return zero values (not panic) when the underlying map value has the wrong type (e.g., `priority: "not-a-number"`)
- `ClearField("status")` then `Status()` returns `""`
- `NewTaskFrontmatter(nil)` is safe for reads and writes

### 10. Integration test — complex nested value round-trip

Add a scenario to `integration/cli_test.go` (or enable the pending `PIt` block if it already exists): write a task file with a custom frontmatter field containing a nested map, then run `vault-cli task set <name> status in_progress`, re-read the file, and assert the nested custom field byte-equals the original (modulo alphabetical key reordering). This covers the spec failure mode "YAML contains complex nested value".
</requirements>

<constraints>
- Storage interface signatures (`ReadTask`, `WriteTask`, `FindTaskByName`, `ListTasks`) must NOT change
- CLI command surface and flags must NOT change
- `NormalizeTaskStatus` logic must still run for every task read from disk — it now lives in `TaskFrontmatter.Status()` instead of `TaskStatus.UnmarshalYAML`
- UUID auto-generation on `WriteTask` must be preserved — check `TaskIdentifier()` before serializing
- `task.Name`, `task.FilePath`, `task.Content`, `task.ModifiedDate` must continue to work (via embedded `FileMetadata` and the `Content` field)
- `TaskStatus.UnmarshalYAML` must be REMOVED — YAML parsing now goes through `parseToFrontmatterMap` which stores raw strings; normalization happens in the getter
- `isReadOnlyTag` read-only detection for task fields (Name, FilePath, etc.) is enforced by the fact that those fields are no longer in the YAML map — `GetField`/`SetField` operate only on the frontmatter map; metadata fields cannot be targeted
- The `pkg/ops/frontmatter_reflect.go` helpers are NOT removed in this prompt — they are still used by non-task entity operations
- All existing tests must pass (update assertions but not remove test scenarios)
- Do NOT migrate Goal, Theme, Objective, or Vision — those are in Prompt 3
- Do NOT update `docs/development-patterns.md` in this prompt — spec AC #12 ("docs updated to reflect map-based pattern") is explicitly handled by Prompt 4 (cleanup) after all five entities are migrated, to avoid documenting a half-migrated state
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
# Confirm TaskFrontmatter type exists
grep -n 'type TaskFrontmatter' pkg/domain/task_frontmatter.go
# expected: one line

# Confirm Task struct uses embedding
grep -n 'TaskFrontmatter' pkg/domain/task.go
grep -n 'FileMetadata' pkg/domain/task.go
# expected: both embedded

# Confirm UnmarshalYAML removed from TaskStatus
grep -n 'UnmarshalYAML' pkg/domain/task.go
# expected: no output

# Confirm Priority.Validate exists and is called by SetPriority
grep -n 'func (p Priority) Validate' pkg/domain/priority.go
# expected: one line
grep -n 'p.Validate(ctx)' pkg/domain/task_frontmatter.go
# expected: one line (inside SetPriority)

# Confirm WriteTask uses map-based serialize
grep -n 'serializeMapAsFrontmatter\|RawMap' pkg/storage/task.go
# expected: at least one match

# Confirm unknown field round-trip works
# Create a temp task file with a custom frontmatter key and check vault-cli preserves it
```

```
# Coverage check for new files
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/domain/... ./pkg/storage/... && \
  go tool cover -func=/tmp/cover.out | grep 'task_frontmatter\|file_metadata'
# expected: ≥80% on new files
```
</verification>
