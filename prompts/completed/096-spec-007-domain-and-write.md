---
status: completed
spec: [007-task-identifier-field]
container: vault-cli-096-spec-007-domain-and-write
dark-factory-version: v0.68.1-dirty
created: "2026-03-27T00:00:00Z"
queued: "2026-03-27T18:40:26Z"
started: "2026-03-27T19:53:14Z"
completed: "2026-03-27T18:53:23Z"
branch: dark-factory/task-identifier-field
---

<summary>
- Tasks carry a stable `task_identifier` field that round-trips through frontmatter
- Reading a task file with `task_identifier: abc-123` in frontmatter populates the new field
- Writing a task automatically generates a UUIDv4 when `task_identifier` is not yet set
- Subsequent writes preserve the existing identifier — generation is idempotent
- Tasks without `task_identifier` in frontmatter read back with an empty field (no UUID injected at read time)
- The existing generic `vault-cli task get/set/clear task_identifier` commands work out of the box — no new CLI code needed
- Lint reports a non-fixable error for tasks that have no `task_identifier`
- Empty `task_identifier` is never written to frontmatter (`omitempty`)
- The UUID dependency is promoted from indirect to direct
- All existing tests pass unchanged
</summary>

<objective>
Add `TaskIdentifier string` to `domain.Task`, auto-generate a UUIDv4 in `WriteTask` when the field is absent, and add a lint check that flags tasks missing `task_identifier`. This enables stable task identity that survives file renames, satisfies the generic get/set/clear without any CLI changes (the reflection-based infrastructure discovers the new YAML tag automatically), and surfaces unidentified tasks via the linter.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/domain/task.go` — Task struct (add field here); note how other optional string fields like `ClaudeSessionID` and `Recurring` use `omitempty`
- `pkg/storage/task.go` — `WriteTask` (inject UUID here before `serializeWithFrontmatter`)
- `pkg/storage/base.go` — `serializeWithFrontmatter` (understand how YAML is written)
- `pkg/ops/lint.go` — `IssueType` constants, `collectLintIssues`, `lintFile` (add new check here)
- `pkg/ops/lint_test.go` — existing test patterns (Ginkgo/Gomega)
- `pkg/storage/task_test.go` — storage test patterns (temp dirs, WriteTask/ReadTask flow)
- `go.mod` — confirm `github.com/google/uuid` is present as indirect; promote to direct
- `vendor/github.com/google/uuid/` — check if vendored; if not, run `go mod vendor` after promoting the dep
</context>

<requirements>
### 1. Add `TaskIdentifier` to `pkg/domain/task.go`

Add the field to the `Task` struct after `DueDate` and before the `// Metadata` comment block:

```go
DueDate         *DateOrDateTime `yaml:"due_date,omitempty"`
TaskIdentifier  string          `yaml:"task_identifier,omitempty"`
```

No new imports are needed — `string` is a primitive type.

The `omitempty` tag ensures that tasks with no identifier write no `task_identifier:` line to frontmatter.

### 2. Promote `github.com/google/uuid` to a direct dependency in `go.mod`

In `go.mod`, move `github.com/google/uuid v1.6.0` from the `// indirect` block to the direct `require` block (the one at the top that contains `github.com/bborbe/errors`, etc.). Remove the `// indirect` comment.

After editing `go.mod`, check if `vendor/github.com/google/uuid/` exists. If not, run `go mod vendor` to vendor it.

### 3. Auto-generate UUID in `pkg/storage/task.go` — `WriteTask`

At the top of `WriteTask`, before the call to `serializeWithFrontmatter`, add:

```go
if task.TaskIdentifier == "" {
    task.TaskIdentifier = uuid.New().String()
}
```

Add the import `"github.com/google/uuid"` to the import block in `pkg/storage/task.go`.

The full updated `WriteTask` should look like:

```go
func (t *taskStorage) WriteTask(ctx context.Context, task *domain.Task) error {
    if task.TaskIdentifier == "" {
        task.TaskIdentifier = uuid.New().String()
    }

    content, err := t.serializeWithFrontmatter(ctx, task, task.Content)
    if err != nil {
        return errors.Wrap(ctx, err, "serialize frontmatter")
    }

    if err := os.WriteFile(task.FilePath, []byte(content), 0600); err != nil {
        return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", task.FilePath))
    }

    return nil
}
```

### 4. Add `IssueTypeMissingTaskIdentifier` lint check to `pkg/ops/lint.go`

#### 4a. Add the constant

In the `const` block with the other `IssueType` values:

```go
IssueTypeMissingTaskIdentifier IssueType = "MISSING_TASK_IDENTIFIER"
```

#### 4b. Add the check to `collectLintIssues`

Append a new check at the end of `collectLintIssues`, after the status/checkbox mismatch check:

```go
// Check for missing task_identifier
if missingID := l.detectMissingTaskIdentifier(frontmatterYAML); missingID {
    issues = append(issues, LintIssue{
        FilePath:    filePath,
        IssueType:   IssueTypeMissingTaskIdentifier,
        Description: "task_identifier is missing; run backfill to assign one",
        Fixable:     false,
        Fixed:       false,
    })
}
```

#### 4c. Add `detectMissingTaskIdentifier` helper

Add this private method to `lintOperation`:

```go
// detectMissingTaskIdentifier returns true if task_identifier is absent or empty.
func (l *lintOperation) detectMissingTaskIdentifier(frontmatterYAML string) bool {
    var fm struct {
        TaskIdentifier string `yaml:"task_identifier"`
    }
    if err := yaml.Unmarshal([]byte(frontmatterYAML), &fm); err != nil {
        return false // Cannot parse; other checks will surface the error
    }
    return fm.TaskIdentifier == ""
}
```

`yaml` is already imported in `lint.go`.

### 5. Write tests

#### 5a. Storage tests — `pkg/storage/task_test.go`

Add a `Describe("WriteTask")` block (or extend the existing one if it already exists) with these cases:

- **Auto-generates UUID when TaskIdentifier is empty**: create a temp vault, write a task with `TaskIdentifier: ""`, read the file back and confirm frontmatter contains a non-empty `task_identifier` value.
- **Preserves existing TaskIdentifier**: write a task with `TaskIdentifier: "existing-uuid"`, read the file back and confirm `task_identifier: existing-uuid` is preserved.
- **Round-trip**: write then read via `ReadTask`; confirm `task.TaskIdentifier` matches what was written.

Use the existing `Ginkgo/Gomega` style (`Describe`/`Context`/`It`/`BeforeEach`) and temp directory setup already in the file.

Example:

```go
Describe("WriteTask UUID generation", func() {
    var (
        taskPath string
        task     *domain.Task
    )

    BeforeEach(func() {
        taskPath = filepath.Join(tasksDir, "My Task.md")
        task = &domain.Task{
            Name:     "My Task",
            FilePath: taskPath,
            Status:   domain.TaskStatusTodo,
            Content:  "---\nstatus: todo\npage_type: task\n---\n# My Task\n",
        }
    })

    It("generates a UUID when TaskIdentifier is empty", func() {
        Expect(store.WriteTask(ctx, task)).To(Succeed())
        content, err := os.ReadFile(taskPath)
        Expect(err).NotTo(HaveOccurred())
        Expect(string(content)).To(ContainSubstring("task_identifier:"))
    })

    It("preserves an existing TaskIdentifier", func() {
        task.TaskIdentifier = "my-stable-uuid"
        Expect(store.WriteTask(ctx, task)).To(Succeed())
        content, err := os.ReadFile(taskPath)
        Expect(err).NotTo(HaveOccurred())
        Expect(string(content)).To(ContainSubstring("task_identifier: my-stable-uuid"))
    })

    It("round-trips TaskIdentifier through read", func() {
        task.TaskIdentifier = "round-trip-uuid"
        Expect(store.WriteTask(ctx, task)).To(Succeed())
        read, err := store.ReadTask(ctx, vaultDir, domain.TaskID("My Task"))
        Expect(err).NotTo(HaveOccurred())
        Expect(read.TaskIdentifier).To(Equal("round-trip-uuid"))
    })
})
```

#### 5b. Lint tests — `pkg/ops/lint_test.go`

Add test cases for `IssueTypeMissingTaskIdentifier`:

- **Task without `task_identifier`**: lint a file with valid frontmatter but no `task_identifier` field → issues contains one entry with `IssueType == IssueTypeMissingTaskIdentifier`.
- **Task with `task_identifier`**: lint a file with `task_identifier: some-uuid` → no `IssueTypeMissingTaskIdentifier` in issues.
- **Task with missing frontmatter**: confirm `IssueTypeMissingTaskIdentifier` is NOT reported (missing frontmatter already reported; don't pile on).

Use temp files and `lintOp.ExecuteFile` (or `lintOp.Execute` for the directory-walk variant). Follow the existing test style.

#### 5c. Domain struct test — verify `omitempty` behavior

In `pkg/storage/task_test.go`, add a case: write a task with `TaskIdentifier: ""` and confirm the written file does NOT contain the string `task_identifier: ""` or `task_identifier:""` (omitempty suppresses the key entirely when empty).

Note: the UUID auto-generation in `WriteTask` means an empty identifier will get replaced before serialization. To test `omitempty` isolation, test the serialization directly on the domain struct:

```go
It("omits task_identifier from frontmatter when empty", func() {
    // After WriteTask, task.TaskIdentifier is set (UUID generated).
    // Verify the file has task_identifier: <something-non-empty>
    // (not the empty omitempty case — that's covered at the yaml.Marshal level)
    Expect(store.WriteTask(ctx, task)).To(Succeed())
    content, err := os.ReadFile(taskPath)
    Expect(err).NotTo(HaveOccurred())
    // Should have a non-empty task_identifier (auto-generated)
    Expect(string(content)).To(MatchRegexp(`task_identifier: \S+`))
})
```
</requirements>

<constraints>
- Existing tests must pass — adding the field must not break existing serialization (the `omitempty` tag ensures backward compat)
- Tasks without `task_identifier` must NOT get an empty field written (enforced by `omitempty` + UUID generation in WriteTask)
- Auto-generation must be idempotent — once set, subsequent writes preserve the existing identifier (the `if task.TaskIdentifier == ""` guard ensures this)
- Auto-generation happens at the write layer, not the read layer — `ReadTask` returns empty `TaskIdentifier` for old tasks without the field
- `IssueTypeMissingTaskIdentifier` is NOT fixable (Fixable: false) — fixing requires the backfill operation, not the lint fixer
- Field name follows existing snake_case convention: `task_identifier` (matching `page_type`, `planned_date`, `defer_date`)
- UUID generation uses `github.com/google/uuid` (crypto/rand-backed); never `math/rand`
- The lint check must NOT fire for files with missing frontmatter (those already produce `IssueTypeMissingFrontmatter`; `detectMissingTaskIdentifier` returns false on parse error)
- The operation layer returns structured results, never writes to stdout
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
# Confirm field is present in domain struct
grep 'task_identifier' pkg/domain/task.go
# expected: one line with TaskIdentifier string yaml:"task_identifier,omitempty"
```

```
# Confirm UUID import in storage
grep 'google/uuid' pkg/storage/task.go
# expected: one line
```

```
# Confirm lint constant exists
grep 'MissingTaskIdentifier' pkg/ops/lint.go
# expected: at least two lines (constant + usage)
```

```
# Confirm google/uuid is a direct dep
grep 'google/uuid' go.mod
# expected: one line WITHOUT "// indirect"
```
</verification>
