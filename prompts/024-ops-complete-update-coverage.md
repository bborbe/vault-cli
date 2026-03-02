---
status: queued
created: "2026-03-02T11:56:19Z"
---









<objective>
Increase test coverage for `complete.go` and `update.go` output/helper methods.
Targets: `complete.go:Execute` (55.6%), `update.go:outputUpdateResult` (42.9%),
`update.go:outputErrorJSON` (33.3%), `update.go:handleNoCheckboxes` (50.0%).
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Read CLAUDE.md for project conventions.
Read ~/.claude/docs/go-testing.go for testing patterns.

Existing test files: `complete_test.go`, `update_test.go` — add tests there.
Use mock storage pattern (counterfeiter mocks from `mocks/` package).

Current coverage: `pkg/ops` = 71%. Target: ≥80%.
</context>

<requirements>
Add tests in `./pkg/ops/complete_test.go`:

1. `Execute` more paths:
   - Task with GoalLink set → triggers markGoalCheckbox path; mock storage.FindGoalByName + WriteGoal
   - updateDailyNote path: mock storage.ReadDailyNote + WriteDailyNote; verify checkbox removed
   - ReadDailyNote returns error → error propagated
   - WriteDailyNote returns error → error propagated
   - FindGoalByName returns error → error propagated
   - WriteGoal returns error → error propagated

Add tests in `./pkg/ops/update_test.go`:

2. `Execute` more paths covering output methods:
   - Task with no checkboxes in content → `handleNoCheckboxes` path: verify appropriate output/no-error
   - Task with all checked → progress=100
   - Task with none checked → progress=0
   - json output format → `outputUpdateResult` JSON path: verify JSON response structure
   - Storage WriteTask returns error → `outputErrorJSON` path triggered; verify error returned

3. `outputErrorJSON` path:
   - Trigger via WriteTask failure with outputFormat="json"
   - Verify error is returned
</requirements>

<constraints>
- Do NOT modify complete.go or update.go — tests only
- Use `mocks.Storage` counterfeiter mock
- Check existing test structure before adding (don't duplicate BeforeEach setup)
- Do NOT run make precommit iteratively — use make test; run make precommit once at the end
</constraints>

<verification>
Run: `make test`
Run: `go test -mod=mod -cover ./pkg/ops/...`

Target: `pkg/ops` coverage ≥80%.
</verification>

<success_criteria>
- make test passes
- pkg/ops coverage ≥80%
- complete.Execute goal path tested
- complete.Execute daily note paths (read + write errors) tested
- update.handleNoCheckboxes tested
- update.outputUpdateResult JSON path tested
- update.outputErrorJSON triggered and tested
</success_criteria>
