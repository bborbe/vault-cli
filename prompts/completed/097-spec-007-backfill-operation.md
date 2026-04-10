---
status: completed
spec: [007-task-identifier-field]
container: vault-cli-097-spec-007-backfill-operation
dark-factory-version: v0.68.1-dirty
created: "2026-03-27T00:00:00Z"
queued: "2026-03-27T18:55:23Z"
started: "2026-03-27T18:55:28Z"
completed: "2026-03-27T18:59:21Z"
branch: dark-factory/task-identifier-field
---

<summary>
- All tasks in a vault can be backfilled with `task_identifier` in a single operation
- Tasks that already have a `task_identifier` are skipped (no overwrite)
- Tasks missing `task_identifier` get a generated UUID written back to their file
- Unparseable task files are skipped with a warning log; the operation continues
- The caller receives the list of file paths that were modified (for batch-commit workflows)
- The operation returns structured results, never writes to stdout
- A counterfeiter mock is generated for the new interface
- The new operation composes `TaskStorage` (list + write) — no direct file I/O in the operation itself
- Tests cover all branches: already-identified tasks, missing-identifier tasks, parse errors, write errors
- All existing tests from Prompt 1 continue to pass
</summary>

<objective>
Create `EnsureAllTaskIdentifiersOperation` in `pkg/ops/` that walks all tasks in a vault and writes back any task missing `task_identifier`. Because `WriteTask` already auto-generates UUIDs (added in Prompt 1), the backfill logic is: list all tasks, call `WriteTask` for any task with an empty `TaskIdentifier`, collect modified file paths, skip and warn on errors. This is the foundation for external consumers who need to batch-assign identifiers to existing vaults.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

**Prompt 1 must be completed first.** This prompt depends on:
- `domain.Task.TaskIdentifier` field existing
- `WriteTask` auto-generating a UUID when `TaskIdentifier == ""`

Key files to read before making changes:
- `pkg/domain/task.go` — Task struct with `TaskIdentifier` field (added in Prompt 1)
- `pkg/storage/storage.go` — `TaskStorage` interface: `ListTasks`, `WriteTask`
- `pkg/storage/task.go` — `WriteTask` implementation (auto-generates UUID)
- `pkg/ops/goal_complete.go` — structural template for a new ops file
- `pkg/ops/complete.go` — `MutationResult` type and error-wrapping patterns
- `pkg/ops/goal_defer.go` — another template showing constructor + interface + counterfeiter annotation
- `mocks/` — directory where counterfeiter writes generated mocks
- `pkg/ops/ops_suite_test.go` — test suite bootstrap for the ops package
</context>

<requirements>
### 1. Create `pkg/ops/ensure_task_identifiers.go`

New file with the following content:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
    "context"
    "log/slog"

    "github.com/bborbe/errors"

    "github.com/bborbe/vault-cli/pkg/storage"
)

// BackfillResult holds the outcome of an EnsureAllTaskIdentifiers run.
type BackfillResult struct {
    // ModifiedFiles is the list of absolute file paths that were written during backfill.
    ModifiedFiles []string
    // SkippedFiles is the count of files skipped due to errors.
    SkippedFiles int
}

//counterfeiter:generate -o ../../mocks/ensure-all-task-identifiers-operation.go --fake-name EnsureAllTaskIdentifiersOperation . EnsureAllTaskIdentifiersOperation
type EnsureAllTaskIdentifiersOperation interface {
    Execute(ctx context.Context, vaultPath string) (BackfillResult, error)
}

// NewEnsureAllTaskIdentifiersOperation creates a new backfill operation.
func NewEnsureAllTaskIdentifiersOperation(
    taskStorage storage.TaskStorage,
) EnsureAllTaskIdentifiersOperation {
    return &ensureAllTaskIdentifiersOperation{
        taskStorage: taskStorage,
    }
}

type ensureAllTaskIdentifiersOperation struct {
    taskStorage storage.TaskStorage
}

// Execute walks all tasks in vaultPath and writes back any task missing task_identifier.
// Tasks that already have task_identifier are skipped. Unparseable files are skipped
// with a warning. Returns the list of file paths that were modified.
func (e *ensureAllTaskIdentifiersOperation) Execute(
    ctx context.Context,
    vaultPath string,
) (BackfillResult, error) {
    tasks, err := e.taskStorage.ListTasks(ctx, vaultPath)
    if err != nil {
        return BackfillResult{}, errors.Wrap(ctx, err, "list tasks")
    }

    var result BackfillResult
    for _, task := range tasks {
        if task.TaskIdentifier != "" {
            continue // Already has an identifier, skip
        }

        // WriteTask auto-generates the UUID when TaskIdentifier is empty.
        if writeErr := e.taskStorage.WriteTask(ctx, task); writeErr != nil {
            slog.Warn("backfill: skipping task write error",
                "file", task.FilePath,
                "error", writeErr,
            )
            result.SkippedFiles++
            continue
        }

        result.ModifiedFiles = append(result.ModifiedFiles, task.FilePath)
    }

    return result, nil
}
```

### 2. Generate the mock

Run:
```
go generate ./pkg/ops/...
```

This creates `mocks/ensure-all-task-identifiers-operation.go`.

### 3. Write tests in `pkg/ops/ensure_task_identifiers_test.go`

Use Ginkgo/Gomega with counterfeiter mocks (`mocks.TaskStorage`). Cover all branches:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
    "context"
    "errors"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    "github.com/bborbe/vault-cli/mocks"
    "github.com/bborbe/vault-cli/pkg/domain"
    "github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("EnsureAllTaskIdentifiersOperation", func() {
    var (
        ctx          context.Context
        op           ops.EnsureAllTaskIdentifiersOperation
        mockStorage  *mocks.TaskStorage
        vaultPath    string
        result       ops.BackfillResult
        err          error
    )

    BeforeEach(func() {
        ctx = context.Background()
        mockStorage = &mocks.TaskStorage{}
        op = ops.NewEnsureAllTaskIdentifiersOperation(mockStorage)
        vaultPath = "/vault"
    })

    JustBeforeEach(func() {
        result, err = op.Execute(ctx, vaultPath)
    })

    Context("when ListTasks fails", func() {
        BeforeEach(func() {
            mockStorage.ListTasksReturns(nil, errors.New("disk error"))
        })

        It("returns an error", func() {
            Expect(err).To(HaveOccurred())
            Expect(err.Error()).To(ContainSubstring("list tasks"))
        })

        It("returns an empty result", func() {
            Expect(result.ModifiedFiles).To(BeEmpty())
        })
    })

    Context("when all tasks already have task_identifier", func() {
        BeforeEach(func() {
            mockStorage.ListTasksReturns([]*domain.Task{
                {Name: "Task A", FilePath: "/vault/Tasks/Task A.md", TaskIdentifier: "uuid-a"},
                {Name: "Task B", FilePath: "/vault/Tasks/Task B.md", TaskIdentifier: "uuid-b"},
            }, nil)
        })

        It("does not call WriteTask", func() {
            Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
        })

        It("returns empty ModifiedFiles", func() {
            Expect(result.ModifiedFiles).To(BeEmpty())
        })

        It("returns no error", func() {
            Expect(err).NotTo(HaveOccurred())
        })
    })

    Context("when some tasks are missing task_identifier", func() {
        BeforeEach(func() {
            mockStorage.ListTasksReturns([]*domain.Task{
                {Name: "Task A", FilePath: "/vault/Tasks/Task A.md", TaskIdentifier: "uuid-existing"},
                {Name: "Task B", FilePath: "/vault/Tasks/Task B.md", TaskIdentifier: ""},
                {Name: "Task C", FilePath: "/vault/Tasks/Task C.md", TaskIdentifier: ""},
            }, nil)
            mockStorage.WriteTaskReturns(nil)
        })

        It("calls WriteTask only for tasks without identifier", func() {
            Expect(mockStorage.WriteTaskCallCount()).To(Equal(2))
        })

        It("returns modified file paths", func() {
            Expect(result.ModifiedFiles).To(ConsistOf(
                "/vault/Tasks/Task B.md",
                "/vault/Tasks/Task C.md",
            ))
        })

        It("returns no error", func() {
            Expect(err).NotTo(HaveOccurred())
        })

        It("has zero skipped files", func() {
            Expect(result.SkippedFiles).To(Equal(0))
        })
    })

    Context("when WriteTask fails for one task", func() {
        BeforeEach(func() {
            mockStorage.ListTasksReturns([]*domain.Task{
                {Name: "Task A", FilePath: "/vault/Tasks/Task A.md", TaskIdentifier: ""},
                {Name: "Task B", FilePath: "/vault/Tasks/Task B.md", TaskIdentifier: ""},
            }, nil)
            // First call fails, second succeeds
            mockStorage.WriteTaskReturnsOnCall(0, errors.New("permission denied"))
            mockStorage.WriteTaskReturnsOnCall(1, nil)
        })

        It("skips the failing task and continues", func() {
            Expect(err).NotTo(HaveOccurred())
        })

        It("records the successful write in ModifiedFiles", func() {
            Expect(result.ModifiedFiles).To(ConsistOf("/vault/Tasks/Task B.md"))
        })

        It("increments SkippedFiles for the failed write", func() {
            Expect(result.SkippedFiles).To(Equal(1))
        })
    })

    Context("when vault has no tasks", func() {
        BeforeEach(func() {
            mockStorage.ListTasksReturns([]*domain.Task{}, nil)
        })

        It("returns no error", func() {
            Expect(err).NotTo(HaveOccurred())
        })

        It("returns empty result", func() {
            Expect(result.ModifiedFiles).To(BeEmpty())
            Expect(result.SkippedFiles).To(Equal(0))
        })
    })
})
```

### 4. Verify test coverage

After writing the tests, check coverage for the new file:

```
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep ensure_task_identifiers
```

Target: ≥80% statement coverage for `pkg/ops/ensure_task_identifiers.go`.
</requirements>

<constraints>
- The operation must NOT perform direct file I/O — all reads and writes go through `TaskStorage`
- `ListTasks` errors are fatal (return early); individual `WriteTask` errors are non-fatal (log warning, increment SkippedFiles, continue)
- Tasks with a non-empty `TaskIdentifier` must be skipped — never overwrite an existing identifier
- `ModifiedFiles` contains the file paths of tasks successfully written during this run; it does NOT include tasks that were already identified
- The `BackfillResult.ModifiedFiles` slice may be nil when empty (not `[]string{}`) — the caller must handle both
- `//counterfeiter:generate` annotation must be present on the interface
- The operation returns structured results, never writes to stdout
- Do NOT add a CLI subcommand for this operation — the spec says no new CLI subcommands are needed
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
</constraints>

<verification>
```
make precommit
```

```
# Confirm mock was generated
ls mocks/ensure-all-task-identifiers-operation.go
```

```
# Confirm no direct file I/O in the new op file
grep -n 'os\.' pkg/ops/ensure_task_identifiers.go
# expected: no output
```

```
# Confirm counterfeiter annotation is present
grep 'counterfeiter:generate' pkg/ops/ensure_task_identifiers.go
# expected: one line referencing EnsureAllTaskIdentifiersOperation
```

```
# Coverage check
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep ensure_task_identifiers
# expected: ≥80% coverage on the new file
```
</verification>
