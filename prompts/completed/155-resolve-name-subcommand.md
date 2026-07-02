---
status: completed
spec: [021-resolve-name-subcommand]
summary: Added ResolveOperation in pkg/ops/resolve.go with task-first name resolution and full unit test coverage
execution_id: vault-cli-resolve-exec-155-resolve-name-subcommand
dark-factory-version: dev
created: "2026-07-02T10:00:00Z"
queued: "2026-07-02T09:46:53Z"
started: "2026-07-02T09:48:28Z"
completed: "2026-07-02T09:50:22Z"
---

<summary>
- Adds the read-only "resolve" operation: given a name, it probes task storage first, then goal storage, and reports which type matched (or neither)
- Task-first priority — a name that matches both a task and a goal resolves to "task"
- A miss is a normal outcome, not an error: when the name matches nothing, it returns a not-found result and NO error, so slash commands can call it without special-casing failures
- Purely composed from the two existing storage interfaces — no new storage methods, no file I/O in the operation
- Fully unit-tested with Counterfeiter mocks, including task-first priority and the not-found path
</summary>

<objective>
Add a `ResolveOperation` in `pkg/ops/resolve.go` that resolves a name to a task or goal for a single vault, probing task storage first then goal storage, and returning a `domain.ResolveResult` with a not-found result (never an error) when neither matches.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — `New*`/`Create*` factory, zero business logic in factories, constructors return interfaces.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface + private struct + constructor, error wrapping, Counterfeiter.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, Counterfeiter mocks, coverage ≥80%, error-path testing.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` API, never `fmt.Errorf`, never `context.Background()` in pkg/.

Read these files before implementing:
- `pkg/ops/show.go` — closest existing operation: interface + `New*` constructor + private struct injecting `storage.TaskStorage`, `Execute(ctx, vaultPath, vaultName, taskName)`. Mirror this structure.
- `pkg/ops/show_test.go` — Ginkgo test structure to mirror: `package ops_test`, `mocks.TaskStorage{}`, `FindTaskByNameReturns(...)`, BeforeEach/JustBeforeEach/Context layout.
- `pkg/storage/storage.go` lines 50-61 — the two interfaces you inject:
  ```go
  type TaskStorage interface {
      WriteTask(ctx context.Context, task *domain.Task) error
      FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)
      ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)
  }
  type GoalStorage interface {
      WriteGoal(ctx context.Context, goal *domain.Goal) error
      FindGoalByName(ctx context.Context, vaultPath string, name string) (*domain.Goal, error)
  }
  ```
- `pkg/storage/base.go` lines 96-141 — `findFileByName`: IMPORTANT semantics. On a miss it returns a plain string error `errors.Errorf(ctx, "file not found: %s", name)`. There is NO typed sentinel error and NO `errors.Is` target. `FindTaskByName` (`pkg/storage/task.go:65`) and `FindGoalByName` (`pkg/storage/goal.go:81`) wrap this error. Therefore the resolve operation treats ANY non-nil error from these two methods as a MISS and continues probing — it does not attempt to classify the error.
- `pkg/domain/resolve_result.go` — the `ResolveResult{Type, Name, Found}` struct created in prompt 01. If this file does not exist yet, STOP and report `Status: failed` with message `"ResolveResult type not yet deployed (prompt 01)"` — do not create it here.
- `mocks/task-storage.go`, `mocks/goal-storage.go` — the Counterfeiter fakes (`mocks.TaskStorage`, `mocks.GoalStorage`) already exist; use `FindTaskByNameReturns` / `FindGoalByNameReturns` to stub.

OPEN QUESTION (resolved for this prompt, surfaced for reviewer): The spec's Failure Modes table says an "unexpected storage error (not 'not found')" should be propagated. But the storage layer exposes NO typed sentinel distinguishing "file not found" from a genuine I/O error — both arrive as opaque wrapped string errors. Distinguishing them would require a new storage-layer sentinel, which the spec's Constraints forbid ("no new storage methods, no new interfaces"). Decision for this prompt: treat every find error as a miss (return `found:false`, no error). This satisfies AC3 and keeps resolve a never-failing read-only probe. If the reviewer wants true error propagation, that requires a follow-up spec adding a typed `ErrNotFound` sentinel to the storage layer.
</context>

<requirements>
1. Create `pkg/ops/resolve.go` in `package ops` with the standard 3-line BSD license header (copy verbatim from the top of `pkg/ops/show.go`).

2. Define the interface with a Counterfeiter annotation, exactly mirroring the `ShowOperation` style:
   ```go
   // ResolveOperation resolves a name to a task or goal for a single vault.
   //
   //counterfeiter:generate -o ../../mocks/resolve-operation.go --fake-name ResolveOperation . ResolveOperation
   type ResolveOperation interface {
       Execute(ctx context.Context, vaultPath string, name string) (domain.ResolveResult, error)
   }
   ```

3. Define the constructor as pure composition (no conditionals, no I/O, no `context.Background()`), injecting BOTH storage interfaces:
   ```go
   func NewResolveOperation(
       taskStorage storage.TaskStorage,
       goalStorage storage.GoalStorage,
   ) ResolveOperation {
       return &resolveOperation{
           taskStorage: taskStorage,
           goalStorage: goalStorage,
       }
   }

   type resolveOperation struct {
       taskStorage storage.TaskStorage
       goalStorage storage.GoalStorage
   }
   ```

4. Implement `Execute` with task-first priority and miss-is-not-an-error semantics:
   ```go
   func (o *resolveOperation) Execute(
       ctx context.Context,
       vaultPath string,
       name string,
   ) (domain.ResolveResult, error) {
       // Task-first: a name matching both a task and a goal resolves to "task".
       if _, err := o.taskStorage.FindTaskByName(ctx, vaultPath, name); err == nil {
           return domain.ResolveResult{Type: "task", Name: name, Found: true}, nil
       }
       if _, err := o.goalStorage.FindGoalByName(ctx, vaultPath, name); err == nil {
           return domain.ResolveResult{Type: "goal", Name: name, Found: true}, nil
       }
       return domain.ResolveResult{Type: "", Name: name, Found: false}, nil
   }
   ```
   - The `Execute` signature returns an `error` (matching the operation-interface convention across `pkg/ops`), but under the current storage contract it never returns a non-nil error — a find error means "not found", which yields `found:false`. Do NOT wrap or return the find errors.
   - Do NOT call `FindGoalByName` if `FindTaskByName` succeeded (short-circuit — task-first priority, AC4).

5. Add `pkg/ops/resolve_test.go` in `package ops_test`, mirroring `pkg/ops/show_test.go`. Use `mocks.TaskStorage{}` and `mocks.GoalStorage{}`. Import `"errors"` (stdlib) for stubbing miss errors, Ginkgo/Gomega dot-imports, `"github.com/bborbe/vault-cli/mocks"`, `"github.com/bborbe/vault-cli/pkg/domain"`, `"github.com/bborbe/vault-cli/pkg/ops"`. This test crosses the operation's dispatch boundary — it must call the real `Execute` and assert the returned `domain.ResolveResult`, not just construct structs. Cover these cases:
   - **Task match:** `FindTaskByNameReturns(&domain.Task{}, nil)` → result `{Type:"task", Name:<input>, Found:true}`, err nil. Assert `mockGoalStorage.FindGoalByNameCallCount()` is 0 (short-circuit).
   - **Goal match:** `FindTaskByNameReturns(nil, errors.New("file not found"))` and `FindGoalByNameReturns(&domain.Goal{}, nil)` → result `{Type:"goal", Name:<input>, Found:true}`, err nil.
   - **Task-first priority (AC4):** both `FindTaskByNameReturns(&domain.Task{}, nil)` and `FindGoalByNameReturns(&domain.Goal{}, nil)` → result Type is `"task"` (goal never consulted).
   - **Not found (AC3):** both return `errors.New("file not found")` → result `{Type:"", Name:<input>, Found:false}`, err nil. Assert `result.Type` is exactly `""` and `result.Found` is `false`.
   - **Name is echoed:** in every case, `result.Name` equals the exact input name passed to `Execute` (e.g. `"Does Not Exist"`).

6. Regenerate the Counterfeiter mock for the new interface:
   ```bash
   go generate ./...
   ```
   This creates `mocks/resolve-operation.go`. Confirm the file exists after generation.

7. Run `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep resolve` and confirm `resolve.go` is ≥80% covered.
</requirements>

<constraints>
- Existing interfaces only: inject `storage.TaskStorage` + `storage.GoalStorage`. Do NOT add storage methods or new storage interfaces (spec Constraint).
- Factory purity: `NewResolveOperation` is pure composition — no conditionals, no I/O, no `context.Background()` (spec Constraint).
- Error handling: use `github.com/bborbe/errors` wrapping WITH `ctx` anywhere you do wrap — never `fmt.Errorf`, never `context.Background()` in pkg/. (In this operation you do not wrap find errors — you treat them as misses.)
- Miss is NOT an error: not-found returns `found:false` with a nil error (spec Desired Behavior step 6). Exit 0 on miss is enforced at the CLI layer in prompt 03.
- Ginkgo v2 / Gomega with Counterfeiter mocks (spec Constraint: `mocks/task-storage.go`, `mocks/goal-storage.go`).
- Do NOT wire the CLI here — that is prompt 03. Do NOT create `pkg/domain/resolve_result.go` here — that is prompt 01.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

Acceptance criteria advanced by this prompt (from spec 021):
- [ ] AC4 — Task-first priority: name matching both returns task (unit-tested here; end-to-end in prompt 03)
- [ ] AC3 — Not-found path returns `found:false` with no error
- [ ] AC8 — No regression: `task get/goal get/task show/goal show` unaffected (this prompt adds a new file + new interface only)

Failure modes covered (from spec 021 Failure Modes table):
- Task/goal storage returns an error → treated as a miss (documented decision above); resolve never errors on a probe. Name-with-special-characters and missing-dir cases are delegated to existing `findFileByName` behavior — no new failure surface.
</constraints>

<verification>
Run `make test` — must pass.
Run `grep -rn "ResolveOperation" pkg/ops/` — operation implemented (spec Verification).
Run `go generate ./...` then `ls mocks/resolve-operation.go` — mock generated.
Run `go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep resolve` — ≥80% coverage on `resolve.go`.
</verification>
