---
status: completed
summary: Move regex compilation to package-level vars and thread goalsDir through lint pipeline
execution_id: vault-cli-exec-151-fix-lint-hardcoded-dir-and-regex-compile
dark-factory-version: v0.188.1
created: "2026-06-28T22:00:00Z"
queued: "2026-06-28T20:59:31Z"
started: "2026-06-28T21:09:10Z"
completed: "2026-06-28T21:17:24Z"
---

<summary>
- `lint.go` hardcodes `"Goals"` directory in `goalFileExists` instead of using vault config
- All `detect*` methods compile regex patterns at call time via `regexp.MustCompile` — should be package-level `var` declarations
- Fix measurable regex recompilation overhead when linting hundreds of files
- `goalFileExists` gets the goals directory from caller via `collectLintIssues` signature expansion
- No behavior change — same lint rules, same output
</summary>

<objective>
Move regex compilation to package-init time and thread the goals directory through the lint pipeline instead of hardcoding it.
</objective>

<context>
Read:
- `pkg/ops/lint.go` — full file, especially:
  - `Execute` at line 79 — receives `tasksDir` from caller
  - `collectLintIssues` at line 199 — receives `vaultPath` but NOT a goals dir
  - `detectOrphanGoals` at line 365 — calls `goalFileExists` at line 397/417
  - `goalFileExists` at line 432 — hardcodes `filepath.Join(vaultPath, "Goals")`
  - All `detect*` methods using `regexp.MustCompile` inline: lines 311, 332, 350, 442, 454, 460, 497, 509, 574, 694, 704, 723, 759, 771, 801
- `pkg/storage/storage.go` — `Config` struct has `GoalsDir` field
- `pkg/cli/cli.go` — `createLintCommand` at line 469 and `createGenericLintCommand` at line 543
</context>

<requirements>
1. **Extract regex patterns to package-level vars**:
   - Move every `regexp.MustCompile` call in `lint.go` to package-level `var` declarations
   - Use the exact same regex strings — no changes to patterns
   - Example: `var priorityRegex = regexp.MustCompile(...)` at the top of the file, replace all `priorityRegex := regexp.MustCompile(...)` with `priorityRegex`

2. **Thread goals directory through `collectLintIssues`**:
   - Add `goalsDir string` parameter to `collectLintIssues` signature
   - Pass it from `lintFile` callers (lines 159, 194)
   - Pass it from `Execute`/`ExecuteFile` into `lintFile` → `collectLintIssues`
   - `Execute` receives `tasksDir` at line 79 — `goalsDir` needs parallel handling

3. **Update `LintOperation.Execute` interface**:
   - Add `goalsDir string` parameter to `Execute` method at line 24
   - Update all callers — `createGenericLintCommand` in `cli.go` at line 570 already has `getDirFunc(storageConfig)` — parallel that for goals dir

4. **Update `goalFileExists`**:
   - Add `goalsDir string` parameter
   - Join `vaultPath` with `goalsDir` instead of hardcoded `"Goals"`
   - Update call sites in `parseInlineGoalsList` and `parseMultilineGoalsList`

5. **Update `detectOrphanGoals`**:
   - Add `goalsDir string` parameter
   - Pass it to `goalFileExists`, `parseInlineGoalsList`, `parseMultilineGoalsList`

6. **Update all callers in `cli.go`**:
   - `createGenericLintCommand` at line 570 passes both tasks dir and goals dir
   - `createLintCommand` at line 469 — needs goals dir too

7. **Update test callers in `lint_test.go`**:
   - Every `lintOp.Execute(ctx, vaultPath, tasksDir, false)` call site (~30+) needs the new `goalsDir` parameter
   - Pass `""` for goalsDir as a placeholder — orphan-goal detection is not tested, so hardcoded vs parameterized doesn't matter for existing assertions
   - The test vault may not have a `Goals/` subdirectory; passing `""` makes `detectOrphanGoals` skip (returns nil) which matches current behavior when no goals dir exists

8. **Regenerate counterfeiter mocks** after the interface change:
   - Run `go generate ./...` to regenerate `mocks/lint-operation.go` with the new `Execute` signature

9. **Existing tests must still pass** — run `make precommit`
</requirements>

<constraints>
- Do NOT change regex patterns — only move them to package level
- Do NOT change lint output format or JSON structure
- The goals directory comes from vault config via `storage.Config.GoalsDir` — same pattern as tasks dir
- Update `counterfeiter:generate` annotation if `LintOperation` interface signature changes
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
