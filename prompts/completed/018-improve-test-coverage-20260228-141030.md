<objective>
Improve test coverage across vault-cli packages.
Target: pkg/ops ≥85%, pkg/storage ≥75%, pkg/config ≥80%, pkg/domain ≥80%.
Follow go-testing.md patterns from ~/.claude/docs/go-testing.md exactly.
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Testing framework: Ginkgo v2 + Gomega + counterfeiter mocks.
Mocks live in ./mocks/, generated via `go generate -mod=mod ./...`
Read CLAUDE.md for project conventions.
Read ~/.claude/docs/go-testing.md for testing patterns (counterfeiter, temp dirs, suite files).
Read ~/.claude/docs/go-patterns.md for code patterns.
</context>

<current_coverage>
Run `go test -cover ./...` to get current numbers. Known gaps:

pkg/ops (72%):
- workon.go — Execute 0% (new, no tests)
- complete.go — markGoalCheckbox 0%, updateDailyNote 17%
- update.go — syncGoalCheckboxes 0%

pkg/storage (49%):
- ListPages 0%
- WriteGoal 0%, FindGoalByName 0%
- ReadTheme 0%, WriteTheme 0%
- NewConfigFromVault 0%

pkg/config (0%):
- Load, GetVault, GetAllVaults, GetCurrentUser — all untested

pkg/domain (0%):
- Priority.UnmarshalYAML — untested (valid int, string→-1, missing field)
- Task/Goal/Theme String() methods — untested
</current_coverage>

<requirements>
1. Add tests for pkg/ops/workon.go — use mockStorage pattern from complete_test.go:
   - success: status set to in_progress, assignee set correctly
   - task not found: returns error, no write
   - write error: returns error

2. Add tests for pkg/domain/priority.go — Priority.UnmarshalYAML:
   - valid int (e.g. 1) → Priority(1)
   - string value (e.g. "medium") → Priority(-1)
   - missing field → Priority(0) or default

3. Add tests for pkg/config — use real temp config files (os.MkdirTemp):
   - Load: valid config file, missing file (returns default), malformed YAML
   - GetVault: existing vault, unknown vault returns error
   - GetAllVaults: returns all vaults with expanded paths
   - GetCurrentUser: configured user, missing current_user returns error

4. Add tests for pkg/storage gaps — use real temp dirs (os.MkdirTemp + AfterEach cleanup):
   - WriteGoal + FindGoalByName round-trip
   - ReadTheme + WriteTheme round-trip
   - ListPages returns all pages in directory

5. Check suite files exist for each package before creating — never duplicate.
6. Regenerate mocks if needed: `go generate -mod=mod ./...`
</requirements>

<constraints>
- Package name must be `mypackage_test` (external test package)
- Use ErrTest from suite file for error cases (check if already defined)
- Storage tests: always os.MkdirTemp + AfterEach os.RemoveAll
- Never hand-write mocks — use counterfeiter only
- Do NOT modify existing passing tests
</constraints>

<verification>
Run: `make test`
Run: `go test -cover ./pkg/ops/ ./pkg/storage/ ./pkg/config/ ./pkg/domain/`
Confirm coverage improvements:
- pkg/ops ≥ 85%
- pkg/storage ≥ 75%
- pkg/config ≥ 80%
- pkg/domain ≥ 80%
</verification>

<success_criteria>
- `make test` passes
- Coverage targets met for all four packages
- No existing tests broken
</success_criteria>
