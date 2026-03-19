---
status: inbox
---

<summary>
- Linter catches completed/aborted tasks that still have an active phase (not done)
- Linter catches phase=done when status is not completed
- Linter catches backlog/hold tasks with active phases (in_progress, ai_review, human_review)
- New STATUS_PHASE_MISMATCH issue type added to lint output
- All mismatches are non-fixable errors (user decides which field to change)
- No phase = no validation (phase is optional)
</summary>

<objective>
Add cross-validation between task status and phase fields in the linter. Currently each field is validated independently, allowing inconsistent combinations like status=completed with phase=in_progress. Add a new detectStatusPhaseMismatch check with three rules.
</objective>

<context>
The linter (`pkg/ops/lint.go`) validates individual fields but does NOT cross-validate status against phase.

### Current types

- `TaskStatus`: `todo`, `in_progress`, `backlog`, `completed`, `hold`, `aborted`
- `TaskPhase` (`*TaskPhase`, nil = not used): `todo`, `planning`, `in_progress`, `ai_review`, `human_review`, `done`
- Phase is optional — many tasks have no phase (nil)
- Recurring tasks have phase cleared to nil on completion

### Existing lint check pattern (follow this)

In `collectLintIssues` (~line 326), checks are called sequentially. Each returns issues appended to the slice. Follow the pattern of `detectStatusCheckboxMismatch` (~line 575) which parses both status and content from frontmatter YAML.
</context>

<requirements>
1. Add new issue type constant in `pkg/ops/lint.go`:
   `IssueTypeStatusPhaseMismatch IssueType = "STATUS_PHASE_MISMATCH"`

2. Add method `func (l *lintOperation) detectStatusPhaseMismatch(frontmatterYAML string) (bool, string)` in `pkg/ops/lint.go`:
   - Parse `phase:` from frontmatterYAML using regex (same pattern as status parsing ~line 480)
   - If phase key is absent → return `(false, "")`
   - Rule 1: If status is `completed` or `aborted` and phase is NOT `done` → return `(true, "status is <status> but phase is <phase> (expected done or no phase)")`
   - Rule 2: If phase is `done` and status is NOT `completed` → return `(true, "phase is done but status is <status> (expected completed)")`
   - Rule 3: If status is `backlog` or `hold` and phase is `in_progress`, `ai_review`, or `human_review` → return `(true, "status is <status> but phase is <phase> (active phase incompatible with inactive status)")`
   - Otherwise → return `(false, "")`

3. Call `detectStatusPhaseMismatch` from `collectLintIssues` in `pkg/ops/lint.go`. Insert after the `IssueTypeInvalidStatus` append block and before the orphan goals check (`detectOrphanGoals`). Append issue with `Fixable: false`. Note: this method returns `(bool, string)` not `(bool, string, bool)` like `detectStatusCheckboxMismatch` — the third bool is unnecessary since all mismatches are non-fixable.

4. Add tests in `pkg/ops/lint_test.go` for all combinations listed below.
</requirements>

<test-cases>
### Should trigger STATUS_PHASE_MISMATCH

- `status: completed`, `phase: in_progress` → rule 1
- `status: completed`, `phase: todo` → rule 1
- `status: aborted`, `phase: planning` → rule 1
- `status: aborted`, `phase: human_review` → rule 1
- `status: todo`, `phase: done` → rule 2
- `status: in_progress`, `phase: done` → rule 2
- `status: backlog`, `phase: done` → rule 2
- `status: backlog`, `phase: in_progress` → rule 3
- `status: backlog`, `phase: ai_review` → rule 3
- `status: hold`, `phase: human_review` → rule 3
- `status: hold`, `phase: in_progress` → rule 3

### Should NOT trigger

- `status: completed`, `phase: done` → valid
- `status: completed`, no phase → valid (nil)
- `status: aborted`, no phase → valid (nil)
- `status: todo`, `phase: todo` → valid
- `status: in_progress`, `phase: in_progress` → valid
- `status: in_progress`, `phase: ai_review` → valid
- `status: hold`, `phase: todo` → valid
- `status: hold`, `phase: planning` → valid
- `status: backlog`, `phase: todo` → valid
- `status: backlog`, no phase → valid (nil)
</test-cases>

<constraints>
- Use domain constants (`domain.TaskPhaseDone`, `domain.TaskPhaseInProgress`, `domain.TaskPhaseAIReview`, `domain.TaskPhaseHumanReview`, `domain.TaskStatusCompleted`, `domain.TaskStatusAborted`, `domain.TaskStatusBacklog`, `domain.TaskStatusHold`) for comparisons
- Follow existing patterns in `collectLintIssues` for adding checks
- All new issues must be `Fixable: false` — user decides which field to change
- Do NOT commit
</constraints>

<verification>
make precommit
</verification>
