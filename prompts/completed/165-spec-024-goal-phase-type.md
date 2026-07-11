---
status: completed
spec: [024-goal-phase-field]
summary: Created GoalPhase string-enum type in pkg/domain with four canonical values (todo/planning/execution/done) plus tests, mirroring TaskPhase shape without touching it
execution_id: vault-cli-goal-phase-exec-165-spec-024-goal-phase-type
dark-factory-version: v0.191.4
created: "2026-07-11T21:30:00Z"
queued: "2026-07-11T21:28:32Z"
started: "2026-07-11T21:28:33Z"
completed: "2026-07-11T21:30:51Z"
branch: dark-factory/goal-phase-field
---

<summary>
- Introduces a dedicated goal-phase concept with exactly four allowed values: todo, planning, execution, done.
- Setting a goal phase to anything outside those four values will (in a later prompt) fail loudly; this prompt builds the validated type that enables it.
- The goal phase is a deliberate 4-value subset of the 7-value task phase — goals have no ai_review / human_review / in_progress.
- The new type mirrors the shape of the existing task-phase type (canonical constants, an available-set with a membership check, string conversion, validation, pointer helper) but is entirely separate — nothing about tasks changes.
- Ships a table-driven unit test proving each of the four canonical values validates and at least one non-canonical value is rejected.
- This is the data-layer foundation only; no goal command reads or writes the phase yet (that is prompt 2).
</summary>

<objective>
Create a `GoalPhase` string-enum type in `pkg/domain` with the four canonical values `todo` / `planning` / `execution` / `done`, mirroring the shape of the existing `TaskPhase` type (constants, `AvailableGoalPhases` collection with `Contains`, `String()`, `Validate(ctx)`, `Ptr()`), plus a Ginkgo `DescribeTable` unit test. Do NOT touch, reuse, or extend the task-phase type. This is the enum foundation only — no frontmatter wiring in this prompt.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — the canonical string-enum recipe (`Available*` collection, `Validate()`, plural collection type, `Contains()`).
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable`/`Entry`, external `_test` package.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping idiom and the `github.com/bborbe/validation` sentinel.

Read these files before implementing — the new type must mirror them exactly:
- `pkg/domain/task_phase.go` — the reference implementation to mirror in shape. Copy the structure of the newtype, the `const` block, the `Available…` var, the plural collection type with `Contains`, `String()`, `Validate(ctx)`, and `Ptr()`. Note the exact imports it uses: `"context"`, `"github.com/bborbe/collection"`, `"github.com/bborbe/errors"`, `"github.com/bborbe/validation"`. Note `Validate` wraps `validation.Error`: `return errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", t)`.
- `pkg/domain/task_phase_test.go` — the reference test suite. Mirror the `Describe`/`DescribeTable`/`Entry` structure for `Validate`, `String`, `Ptr`, and `Available….Contains`. DROP every alias/`NormalizeTaskPhase`/`in_progress` case — the goal enum has NO aliases.

Do NOT read the goal enum from training data — the `GoalPhase` type does not yet exist in the repo (`grep -rn "GoalPhase" pkg/` returns nothing). You are creating it.
</context>

<requirements>
1. Create `pkg/domain/goal_phase.go` (package `domain`) that declares a `GoalPhase` string newtype mirroring `pkg/domain/task_phase.go` in shape, with EXACTLY these four canonical values — no more, no fewer:
   ```go
   // GoalPhase represents a phase in a goal's lifecycle.
   type GoalPhase string

   const (
       // GoalPhaseTodo means the goal is ready to start but needs planning.
       GoalPhaseTodo GoalPhase = "todo"
       // GoalPhasePlanning means the approach is being designed.
       GoalPhasePlanning GoalPhase = "planning"
       // GoalPhaseExecution means active work is underway.
       GoalPhaseExecution GoalPhase = "execution"
       // GoalPhaseDone means the goal is ready to close.
       GoalPhaseDone GoalPhase = "done"
   )

   // AvailableGoalPhases lists all valid canonical goal phase values.
   var AvailableGoalPhases = GoalPhases{
       GoalPhaseTodo,
       GoalPhasePlanning,
       GoalPhaseExecution,
       GoalPhaseDone,
   }

   // GoalPhases is a collection of GoalPhase values.
   type GoalPhases []GoalPhase

   // Contains returns true if the collection contains the given phase.
   func (g GoalPhases) Contains(phase GoalPhase) bool {
       return collection.Contains(g, phase)
   }

   // String returns the string representation of the phase.
   func (g GoalPhase) String() string {
       return string(g)
   }

   // Validate returns an error if the phase is not a valid canonical value.
   func (g GoalPhase) Validate(ctx context.Context) error {
       if !AvailableGoalPhases.Contains(g) {
           return errors.Wrapf(ctx, validation.Error, "unknown goal phase '%s'", g)
       }
       return nil
   }

   // Ptr returns a pointer to a copy of the phase.
   func (g GoalPhase) Ptr() *GoalPhase {
       return &g
   }
   ```
   Use imports `"context"`, `"github.com/bborbe/collection"`, `"github.com/bborbe/errors"`, `"github.com/bborbe/validation"`. Prepend the standard BSD license header (copy the 3-line header verbatim from the top of `pkg/domain/task_phase.go`).

2. Do NOT add a `Normalize…`, alias map, `IsValid…`, `in_progress`, `ai_review`, or `human_review` value. The goal enum has no legacy/alias values (spec Non-goals — hard veto). Do NOT add a "default to todo" helper; a missing phase is empty, not `todo`.

3. Create `pkg/domain/goal_phase_test.go` (package `domain_test`) — a Ginkgo suite mirroring the applicable Contexts of `pkg/domain/task_phase_test.go`:
   - `Validate` → `DescribeTable` "valid phases" with one `Entry` per canonical value (`todo`, `planning`, `execution`, `done`), asserting `Expect(phase.Validate(ctx)).To(BeNil())`.
   - `Validate` invalid: a `Context` asserting `domain.GoalPhase("bogus").Validate(ctx)` returns a non-nil error whose `.Error()` contains `"unknown goal phase"`. Also assert `domain.GoalPhase("").Validate(ctx)` returns an error (empty is not canonical). Also assert `domain.GoalPhase("in_progress").Validate(ctx)` returns an error (proves goal enum has no task aliases).
   - `String` → asserts `domain.GoalPhaseExecution.String()` equals `"execution"`.
   - `Ptr` → asserts a non-nil pointer with the correct value, and that two `Ptr()` calls return independent pointers (`Expect(p1).NotTo(BeIdenticalTo(p2))`).
   - `AvailableGoalPhases.Contains` → true for each canonical value; false for `domain.GoalPhase("invalid")`, `domain.GoalPhase("")`, and `domain.GoalPhase("in_progress")`.
   Import the domain package as `"github.com/bborbe/vault-cli/pkg/domain"` and the Ginkgo/Gomega dot-imports as in `task_phase_test.go`. The suite bootstrap `pkg/domain/domain_suite_test.go` already exists — do NOT add a new `RunSpecs`.

4. Do NOT create, modify, or reference `pkg/domain/goal_frontmatter.go` in this prompt — the getter/setter/field-case wiring is prompt 2. If you find yourself editing `goal_frontmatter.go`, stop — that is out of scope here.
</requirements>

<constraints>
- The task-side phase type (`pkg/domain/task_phase.go`), its constants, `NormalizeTaskPhase`, and its test suite (`pkg/domain/task_phase_test.go`) must be byte-for-byte unchanged. Frozen (spec Constraints — hard veto). Verify with `git diff --stat pkg/domain/task_phase.go` showing no changes.
- The goal phase enum has EXACTLY four values. Do NOT add `ai_review`, `human_review`, or `in_progress` — that 4-of-7 subset is intentional (spec Assumptions — hard veto).
- Do NOT add alias/normalize handling — no `in_progress` synonym, no migration map (spec Non-goals — hard veto).
- Error wrapping: use `github.com/bborbe/errors` with `ctx` (`errors.Wrapf(ctx, validation.Error, ...)`); never `fmt.Errorf`; never `context.Background()` in non-test code.
- Tests: Ginkgo v2 / Gomega, external `_test` package, Counterfeiter for mocks (none needed here). Coverage ≥80% for `pkg/domain` — the enum is fully exercised by the table test.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make test` iteratively while developing (fast feedback).
Run `go test ./pkg/domain/...` — the new `GoalPhase` suite and the existing `TaskPhase` suite both pass (exit 0).
Run `grep -nE '"todo"|"planning"|"execution"|"done"' pkg/domain/goal_phase.go` — returns ≥4 lines (AC evidence).
Run `grep -nE 'in_progress|ai_review|human_review|Normalize' pkg/domain/goal_phase.go` — returns 0 lines (confirms the excluded values/aliases are absent).
Run `git diff --stat pkg/domain/task_phase.go` — shows no changes (task type frozen).
Run `make precommit` — must pass (lint + format + generate + test + version checks).
</verification>
