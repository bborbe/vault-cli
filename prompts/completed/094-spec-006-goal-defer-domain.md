---
status: completed
spec: ["006"]
summary: Added DeferDate *DateOrDateTime field to domain.Goal struct and extended frontmatter_reflect to handle *DateOrDateTime pointer type for generic set/get/clear operations, with full test coverage.
container: vault-cli-094-spec-006-goal-defer-domain
dark-factory-version: v0.67.3-dirty
created: "2026-03-25T09:30:00Z"
queued: "2026-03-25T09:29:37Z"
started: "2026-03-25T09:29:39Z"
completed: "2026-03-25T09:35:06Z"
---

<summary>
- Goals can now have a defer date, matching the capability tasks already have
- Setting a defer date on a goal works through the existing generic set command
- Reading the defer date back returns the stored value; empty if unset
- Clearing the defer date removes it from the goal
- All existing goal functionality remains unchanged
</summary>

<objective>
Add `defer_date` to the `domain.Goal` struct so the existing generic frontmatter set/get/clear infrastructure (spec 002) automatically supports `vault-cli goal set/get/clear … defer_date` without any additional ops or CLI changes. This is the foundation for the `goal defer` subcommand added in the next prompt.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/domain/goal.go` — Goal struct (add the field here)
- `pkg/domain/task.go` — shows how `DeferDate *DateOrDateTime` is declared on Task (line 24)
- `pkg/domain/date_or_datetime.go` — DateOrDateTime type
- `pkg/ops/frontmatter_entity.go` — GoalGetOperation, GoalSetOperation, GoalClearOperation (lines 50–300 approx); understand how the generic set/get/clear iterates struct fields via reflection/yaml tags
- `pkg/storage/goal.go` — how Goal is read/written from YAML frontmatter
</context>

<requirements>
### 1. Add `defer_date` to `pkg/domain/goal.go`

Add the field to the `Goal` struct immediately after `Completed`:

```go
// Frontmatter fields
Status     GoalStatus      `yaml:"status"`
PageType   string          `yaml:"page_type"`
Theme      string          `yaml:"theme,omitempty"`
Priority   Priority        `yaml:"priority,omitempty"`
Assignee   string          `yaml:"assignee,omitempty"`
StartDate  *time.Time      `yaml:"start_date,omitempty"`
TargetDate *time.Time      `yaml:"target_date,omitempty"`
Tags       []string        `yaml:"tags,omitempty"`
Completed  *libtime.Date   `yaml:"completed,omitempty"`
DeferDate  *DateOrDateTime `yaml:"defer_date,omitempty"`
```

The `DateOrDateTime` type is in the same package (`domain`), so no new import is needed.

### 2. Verify that existing generic operations pick up the new field

The generic set/get/clear operations use `fieldByYAMLTag` (reflection on YAML tags) to discover struct fields. Once `DeferDate` is present with the correct `yaml:"defer_date"` tag, the operations auto-discover it — no registration or additional code changes needed.

Read `pkg/ops/frontmatter_entity.go` to confirm this is still the case.

### 3. Update tests

Add test cases in `pkg/ops/frontmatter_entity_test.go` (or the appropriate goal-ops test file) to verify:

- `entitySetOperation.Execute(ctx, vaultPath, goalName, "defer_date", "2026-04-01")` writes `defer_date: 2026-04-01` to frontmatter (no error)
- `entityGetOperation.Execute(ctx, vaultPath, goalName, "defer_date")` returns `"2026-04-01"` after it has been set
- `entityClearOperation.Execute(ctx, vaultPath, goalName, "defer_date")` removes the field (no error; subsequent get returns empty)

Use the existing Ginkgo/Gomega style (`Describe`/`Context`/`It`/`BeforeEach`) and counterfeiter mocks (`mocks.GoalStorage`) matching the patterns in the same test file.

Test all three operations (set, get, clear) with the `defer_date` key to ensure the field round-trips correctly.
</requirements>

<constraints>
- Do NOT add a `goal defer` subcommand here — that is in the next prompt
- Do NOT modify task defer behavior
- All existing tests must pass
- The `DeferDate` field type must be `*DateOrDateTime` (pointer, omitempty) — identical to how Task declares it
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
grep 'defer_date' pkg/domain/goal.go
# expected: one line with DeferDate *DateOrDateTime yaml:"defer_date,omitempty"
```
</verification>
