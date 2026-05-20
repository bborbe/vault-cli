---
status: committing
summary: 'Added phase-advance logic to WorkOnOperation.Execute: advances phase to planning when nil/empty/todo, preserves mid-flight phases; added four Ginkgo test cases covering all four input states; updated godoc and CHANGELOG.'
container: vault-cli-exec-119-workon-advance-phase-to-planning
dark-factory-version: v0.162.0
created: "2026-05-20T00:00:00Z"
queued: "2026-05-20T12:16:12Z"
started: "2026-05-20T12:16:13Z"
---

<summary>
- `vault-cli task work-on` currently sets `status: in_progress` but does not touch `phase`
- Tasks that have never been started (missing/empty `phase`) and tasks parked as `phase: todo` do not advance into the workflow when work begins
- Resuming a mid-flight task (e.g. `phase: in_progress`, `ai_review`, `human_review`, `done`) currently relies on `phase` being untouched — that contract must be preserved
- After this change, `task work-on` advances `phase` to `planning` only when the current phase is missing, empty, or `todo`
- Any other phase value is left untouched — `work-on` never resets progress backward
- Behavior is single-concern: only `phase` semantics change; `status`, `assignee`, daily-note updates, and Claude session start/resume are unchanged
- Phase semantics follow the canonical chain `todo → planning → in_progress → ai_review → human_review → done` (see Phase System Guide in vault `50 Knowledge Base/`)
</summary>

<objective>
Fix `WorkOnOperation.Execute` in `pkg/ops/workon.go` so it advances `phase` to `planning` when the current phase is nil/empty/`todo`, and leaves it untouched otherwise. Add Ginkgo cases covering the four phase-input states. This restores symmetry with `task complete` (which sets `phase: done`) and supports the canonical phase chain documented in `~/Documents/Obsidian/Personal/50 Knowledge Base/TaskOrchestrator Phase System Guide.md`.
</objective>

<context>
Read `CLAUDE.md` for project conventions (no manual git, `make precommit` is the gate, `pkg/ops/` is a library layer with no stdout writes).

Read these files in full before making changes:

- `pkg/ops/workon.go` — the operation under change. The current `Execute` method does `_ = task.SetStatus(domain.TaskStatusInProgress)` and `task.SetAssignee(assignee)` then writes. The new phase-advance logic goes immediately after `SetAssignee`, before `WriteTask`.
- `pkg/ops/workon_test.go` — Ginkgo suite. Existing test scaffolding constructs the task with `map[string]any{"status": "todo"}` (no phase) via `domain.NewTask(...)`. The new test cases follow the existing `Context("...", func() { BeforeEach(...); It(...) })` shape.
- `pkg/ops/complete.go` — reference for the symmetric pattern. Lines around `task.SetPhase(domain.TaskPhaseDone.Ptr())` (~line 105) show the exact idiom used for setting phase from an ops operation. Mirror that style.
- `pkg/domain/task_phase.go` — phase enum. Constants `TaskPhaseTodo` (`"todo"`), `TaskPhasePlanning` (`"planning"`), `TaskPhaseInProgress` (`"in_progress"`), `TaskPhaseAIReview` (`"ai_review"`), `TaskPhaseHumanReview` (`"human_review"`), `TaskPhaseDone` (`"done"`). The `.Ptr()` helper returns a `*TaskPhase` and is the canonical way to pass a phase into `SetPhase`.
- `pkg/domain/task_frontmatter.go` — getter and setter shape. `Phase()` returns `*TaskPhase` and yields `nil` when the key is missing OR the raw string is `""` (see lines 87–95). `SetPhase(nil)` deletes the key; `SetPhase(&p)` writes `string(p)`. This means a single `task.Phase() == nil` check covers both "missing" and "empty string" — you only need a separate branch for `phase: "todo"`.
- `CHANGELOG.md` — recent prompts (e.g. `prompts/completed/117-task-list-expose-goals-field.md`) added a `## Unreleased` block at the top or appended to it. Follow the same convention. Do NOT bump version strings — the dark-factory daemon's `autoRelease` handles version bumps separately (see CLAUDE.md "Version Alignment").

Reference docs from the coding plugin:
- `~/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions used by this repo
- `~/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — context on the `TaskPhase` enum shape
- `~/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format
- `~/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter mocks (the test file uses `mocks.TaskStorage` and friends, no new mocks needed)

Vault-side semantics (do NOT inline, kept here for reviewer context only): the canonical phase chain is `todo → planning → in_progress → ai_review → human_review → done`. `work-on` represents *entering* the workflow — reading guides, gathering context. That maps to `planning`. Once a task has advanced beyond `todo`, resuming it (e.g. picking up after AI review) must NOT reset progress; hence the no-op branch for any non-`todo` value. Reference: `~/Documents/Obsidian/Personal/50 Knowledge Base/TaskOrchestrator Phase System Guide.md`.
</context>

<requirements>
### 1. Add phase-advance logic to `WorkOnOperation.Execute` in `pkg/ops/workon.go`

Locate the `Execute` method. Find the block:

```go
_ = task.SetStatus(domain.TaskStatusInProgress)
task.SetAssignee(assignee)
```

Immediately AFTER `task.SetAssignee(assignee)` and BEFORE `if err := w.taskStorage.WriteTask(ctx, task); err != nil {`, insert this block:

```go
// Advance phase to planning only when entering the workflow.
// Resuming a mid-flight task (in_progress, ai_review, human_review, done, ...)
// must not reset progress backward.
if currentPhase := task.Phase(); currentPhase == nil || *currentPhase == domain.TaskPhaseTodo {
    task.SetPhase(domain.TaskPhasePlanning.Ptr())
}
```

Notes on the condition:
- `task.Phase()` returns `*TaskPhase`. Per `pkg/domain/task_frontmatter.go` lines 87–95, it returns `nil` for both "key missing" AND "key present but empty string" — so `currentPhase == nil` covers two of the four trigger cases (nil, empty).
- `*currentPhase == domain.TaskPhaseTodo` covers the `"todo"` case.
- Any other value (`TaskPhasePlanning`, `TaskPhaseInProgress`, `TaskPhaseAIReview`, `TaskPhaseHumanReview`, `TaskPhaseDone`, or any non-canonical string round-tripped from disk) falls through and `phase` is left untouched.

Do NOT change anything else in `Execute`. Do NOT change the `WorkOnOperation` interface. Do NOT change the constructor signature. Do NOT touch `handleClaudeSession`, `updateDailyNote`, or the daily-note helpers.

### 2. Add Ginkgo test cases in `pkg/ops/workon_test.go`

Add a new top-level `Context("phase advancement", func() { ... })` block inside the existing `Describe("WorkOnOperation", ...)`, after the existing `Context("daily note updates", ...)` block (i.e. just before the closing `})` of the `Describe`). The block contains four sibling sub-contexts:

a. **`Context("when phase is missing (nil)", ...)`** — the default existing `BeforeEach` already constructs the task with `map[string]any{"status": "todo"}` (no `phase` key). No extra setup needed beyond the existing top-level `BeforeEach`. Assert:
```go
It("sets phase to planning", func() {
    Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
    _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    Expect(writtenTask.Phase()).NotTo(BeNil())
    Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
})
```

b. **`Context("when phase is empty string", ...)`** — override the task in this Context's `BeforeEach`:
```go
BeforeEach(func() {
    task = domain.NewTask(
        map[string]any{"status": "todo", "phase": ""},
        domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"},
        domain.Content(""),
    )
    mockTaskStorage.FindTaskByNameReturns(task, nil)
})
```
Assert the same as (a): phase becomes `planning`.

c. **`Context("when phase is todo", ...)`** — override the task to start with `phase: "todo"`:
```go
BeforeEach(func() {
    task = domain.NewTask(
        map[string]any{"status": "todo", "phase": "todo"},
        domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"},
        domain.Content(""),
    )
    mockTaskStorage.FindTaskByNameReturns(task, nil)
})
```
Assert the same as (a): phase becomes `planning`.

d. **`Context("when phase is in_progress (resume case)", ...)`** — override the task to start mid-flight:
```go
BeforeEach(func() {
    task = domain.NewTask(
        map[string]any{"status": "in_progress", "phase": "in_progress"},
        domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"},
        domain.Content(""),
    )
    mockTaskStorage.FindTaskByNameReturns(task, nil)
})

It("leaves phase unchanged", func() {
    Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
    _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    Expect(writtenTask.Phase()).NotTo(BeNil())
    Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhaseInProgress))
})
```

All four sub-contexts piggyback on the top-level `JustBeforeEach` (`_, err = workOnOp.Execute(...)`) — do NOT add a new `JustBeforeEach` inside the new block. Do NOT modify the top-level `BeforeEach` or any existing `Context` — those rely on the default no-phase task and must keep their current assertions (the `status: in_progress` assertion in the `success` Context continues to hold).

Do NOT add an `It` asserting `Phase()` in the existing `success` Context — the new `phase advancement` block owns those assertions. Keep the existing `success` block focused on status/assignee/session.

### 3. Update `pkg/ops/workon.go` doc comment

The existing godoc on `Execute` reads:

```go
// Execute marks a task as in_progress, assigns it, and starts or resumes a Claude session.
```

Replace with:

```go
// Execute marks a task as in_progress, advances phase to planning when entering the
// workflow (current phase nil/empty/"todo"), assigns it, and starts or resumes a Claude session.
// A mid-flight phase (in_progress, ai_review, human_review, done, ...) is preserved.
```

### 4. CHANGELOG entry

Open `CHANGELOG.md`. If a `## Unreleased` section exists at the top, append to it. If not, create one above the topmost released `## vX.Y.Z` heading. Add:

```markdown
- fix: `vault-cli task work-on` advances `phase` from `todo`/missing/empty to `planning` when entering the workflow; mid-flight phases (`in_progress`, `ai_review`, `human_review`, `done`, ...) are left unchanged so resuming a task does not reset progress
```

Do NOT bump any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json`. The release pipeline handles that.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT modify the `WorkOnOperation` interface or the `NewWorkOnOperation` constructor signature
- Do NOT change `task.SetStatus(domain.TaskStatusInProgress)`, `task.SetAssignee(assignee)`, or the order in which they are called
- Do NOT touch `handleClaudeSession`, `updateDailyNote`, `findAndUpdateTaskCheckbox`, or `appendTaskToDaily`
- Do NOT introduce a new helper function, package, or file — the change is a single `if` block inside `Execute`
- Do NOT extend the phase-advance logic to any other operation (`task create`, `task set`, `task complete`, etc.) — single-concern change
- Do NOT modify the existing `Context("success", ...)`, `Context("when starter is nil", ...)`, `Context("when task already has a session ID", ...)`, `Context("when session start fails", ...)`, `Context("interactive mode", ...)`, `Context("task not found", ...)`, `Context("write error", ...)`, or `Context("daily note updates", ...)` blocks — add a new sibling `Context("phase advancement", ...)` only
- Do NOT add stdout writes from `pkg/ops` — operations return structured results (per `docs/development-patterns.md`)
- Existing tests must remain green without modification
- Use the `bborbe/errors` wrapping convention if any new error site is introduced (none expected — `SetPhase` does not return an error)
- Follow Ginkgo v2 / Gomega style (`Describe`, `Context`, `It`, `Expect(...).To(...)`)
- Do NOT bump versions in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json`
- The phase advancement is unconditional within its branch — there is no flag, no opt-out, no env var
</constraints>

<verification>
Run `make precommit` — must exit 0. This runs lint, format, generate, vet, and the full test suite.

Run `make test` independently to confirm the new four Ginkgo cases pass alongside the existing suite:
```bash
make test
```

Spot-check the diff is minimal:
```bash
git diff --stat pkg/ops/workon.go pkg/ops/workon_test.go CHANGELOG.md
```
Expect: three files changed, ~5–8 lines added in `workon.go`, ~50–70 lines added in `workon_test.go` (four new sub-contexts), one line added in `CHANGELOG.md`.

Manual smoke (after daemon ships a new binary; not gating prompt execution):

1. Create a task with no phase, then `task work-on`, then confirm `phase: planning` appears:
   ```bash
   vault-cli task create --vault personal --name "phase-test-1"
   vault-cli task work-on --vault personal phase-test-1
   vault-cli task show --vault personal phase-test-1 --output json | jq '.phase'
   # Expect: "planning"
   ```

2. Create a task, manually set `phase: in_progress`, then `task work-on`, then confirm phase is unchanged:
   ```bash
   vault-cli task create --vault personal --name "phase-test-2"
   vault-cli task set --vault personal phase-test-2 phase in_progress
   vault-cli task work-on --vault personal phase-test-2
   vault-cli task show --vault personal phase-test-2 --output json | jq '.phase'
   # Expect: "in_progress"
   ```
</verification>
