<objective>
Write Ginkgo/Gomega tests for the untested ops packages in vault-cli.

Currently `complete`, `defer`, and `update` operations have 0% test coverage. Add comprehensive tests using the existing Ginkgo suite pattern from `list_test.go` and `lint_test.go`.
</objective>

<context>
Go CLI project for managing Obsidian vault tasks.
Read CLAUDE.md for project conventions.

Existing test files to follow as patterns:
- `./pkg/ops/list_test.go` — uses counterfeiter mocks, table-driven Ginkgo specs
- `./pkg/ops/lint_test.go` — uses temp files, tests file mutations
- `./pkg/ops/ops_suite_test.go` — suite bootstrap (add nothing here)

Mocks already available in `./mocks/`:
- `mocks/storage.go` — fake Storage with ReadTask, WriteTask, ListTasks stubs

Key ops to test:
- `./pkg/ops/complete.go` — marks task completed, updates daily note
- `./pkg/ops/defer.go` — parses dates (+7d, monday, 2025-12-31), updates defer_date
- `./pkg/ops/update.go` — counts checkboxes, updates progress field
</context>

<requirements>
Write test files:
1. `./pkg/ops/complete_test.go` — test CompleteOperation.Execute:
   - Happy path: task found, status set to "completed", completed_at set to today
   - Task not found → error propagated
   - Storage write called with updated task

2. `./pkg/ops/defer_test.go` — test DeferOperation.Execute and parseDate:
   - `+7d` → today + 7 days
   - `+1d` → today + 1 day
   - `monday` → next Monday date
   - ISO date `2025-12-31` → parsed correctly
   - Invalid date string → error returned
   - Task not found → error propagated
   - Storage write called with updated defer_date

3. `./pkg/ops/update_test.go` — test UpdateOperation.Execute and parseCheckboxes:
   - All checkboxes checked → progress = 100
   - No checkboxes checked → progress = 0
   - Mixed checkboxes → correct percentage
   - No checkboxes in content → progress field unchanged or 0
   - Task not found → error propagated
</requirements>

<implementation>
Follow this pattern from list_test.go:

```go
package ops_test

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/bborbe/vault-cli/mocks"
    "github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("CompleteOperation", func() {
    var (
        fakeStorage *mocks.Storage
        op          ops.CompleteOperation
        ctx         context.Context
    )

    BeforeEach(func() {
        ctx = context.Background()
        fakeStorage = &mocks.Storage{}
        op = ops.NewCompleteOperation(fakeStorage)
    })

    Context("Execute", func() {
        It("marks task as completed", func() {
            // Arrange
            // Act
            // Assert
        })
    })
})
```

Use `fakeStorage.ReadTaskReturns(...)` and `fakeStorage.WriteTaskCallCount()` for assertions.
Use `time.Now()` from `ops_suite_test.go` which sets `time.Local = time.UTC`.
</implementation>

<output>
Create:
- `./pkg/ops/complete_test.go`
- `./pkg/ops/defer_test.go`
- `./pkg/ops/update_test.go`
</output>

<verification>
```
make test
go test -mod=mod -cover ./pkg/ops/...
```

Target: coverage > 70% in pkg/ops after adding tests.
</verification>

<success_criteria>
- `make test` passes
- `complete`, `defer`, `update` each have meaningful test coverage
- Tests use Ginkgo/Gomega, counterfeiter mocks, AAA pattern
- No real filesystem access in tests (use mocks)
</success_criteria>
