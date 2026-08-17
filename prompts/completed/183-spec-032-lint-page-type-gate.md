---
status: completed
spec: [032-bug-task-identifier-lint-fires-on-goals]
summary: 'Gated the two task-identifier lint checks behind the task page type: pageType now flows from the CLI lint entry point (and ops.PageTypeTask on the single-file validate path), so goal/theme/objective/vision lint no longer emits MISSING_TASK_IDENTIFIER or INVALID_TASK_IDENTIFIER while task lint/validate stay byte-identical.'
execution_id: vault-cli-exec-183-spec-032-lint-page-type-gate
dark-factory-version: dev
created: "2026-08-17T08:20:00Z"
queued: "2026-08-17T08:36:53Z"
started: "2026-08-17T08:37:28Z"
completed: "2026-08-17T08:49:33Z"
branch: dark-factory/bug-task-identifier-lint-fires-on-goals
---

# Scope the two task-identifier lint checks to the task page type

<summary>
- Linting goals, themes, objectives, and vision items stops reporting a missing or malformed task identifier — those page types have no identifier concept anywhere in the product.
- Today 268 of the 269 goals across all vaults are flagged by that check, which makes goal lint 99.6% noise and hides the real goal problems underneath it.
- Which page type is being linted is decided by the command the operator ran, not by what happens to be written in the file's frontmatter — frontmatter is missing on a third of goals and would leave them still flagged.
- Linting tasks is completely unchanged: a task with no identifier is still an error, and a task with a non-UUID identifier is still an error.
- Validating a single task by name also still reports a missing identifier — that path is told explicitly that it is a task path.
- A goal that happens to carry a stray identifier key becomes inert rather than an error.
- Every other lint check — duplicate keys, invalid priority, invalid status, status/phase, status/date, orphan goals, status/checkbox — keeps running for every page type exactly as before.
- If a future caller passes a page type nobody recognises, identifier checks simply do not run rather than firing wrongly; that is covered by a test.
- The error message text is deliberately left alone in this prompt; the follow-up prompt replaces it once the remedy it names actually exists.
</summary>

<objective>
`vault-cli goal lint` (and theme / objective / vision lint) must stop reporting `MISSING_TASK_IDENTIFIER` and `INVALID_TASK_IDENTIFIER`, because those two checks assert a task-only invariant that no goal code writes, reads, or documents. Make the lint operation take the page type from its entry point and run the two identifier checks only for the task page type, leaving `vault-cli task lint` and `vault-cli task validate` behavior byte-identical for task files.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/CLI layering rules.

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega suite style, coverage expectations
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter regeneration
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format

Read these files fully before making changes:

- `/workspace/pkg/ops/lint.go` — the file being changed. Verified current interface:

  ```go
  //counterfeiter:generate -o ../../mocks/lint-operation.go --fake-name LintOperation . LintOperation
  type LintOperation interface {
  	Execute(
  		ctx context.Context,
  		vaultPath string,
  		tasksDir string,
  		goalsDir string,
  		fix bool,
  	) ([]LintIssue, error)
  	ExecuteFile(
  		ctx context.Context,
  		filePath string,
  		taskName string,
  		vaultName string,
  	) ([]LintIssue, error)
  }
  ```

  Verified internal call chain (all methods on `*lintOperation`):
  `Execute` → `lintFile(ctx, vaultPath, goalsDir, path, fix)` → either `handleMissingFrontmatterCase(vaultPath, goalsDir, filePath, content, fix)` or `collectLintIssues(vaultPath, goalsDir, filePath, frontmatterYAML, content)`. `handleMissingFrontmatterCase` also calls `collectLintIssues`. `ExecuteFile` calls `lintFile(ctx, "", "", filePath, false)`.

  Verified — the two lines at the end of `collectLintIssues` that must become conditional:

  ```go
  	// Check for missing task_identifier
  	issues = append(issues, l.missingTaskIdentifierIssues(filePath, frontmatterYAML)...)

  	// Check for invalid (non-UUID) task_identifier values
  	issues = append(issues, l.invalidTaskIdentifierIssues(filePath, frontmatterYAML)...)
  ```

- `/workspace/pkg/cli/cli.go` — six lint entry points. Verified:

  `createGenericLintCommand` already takes `pageType string` and today uses it only for the `Short` help text:

  ```go
  func createGenericLintCommand(
  	ctx context.Context,
  	configLoader *config.Loader,
  	vaultName *string,
  	pageType string,
  	getDirFunc func(*storage.Config) string,
  	getGoalsDirFunc func(*storage.Config) string,
  	outputFormat *string,
  ) *cobra.Command {
  ```

  Its single `Execute` call, inside `RunE`:

  ```go
  			lintOp := ops.NewLintOperation()
  			issues, err := lintOp.Execute(
  				ctx,
  				vault.Path,
  				getDirFunc(storageConfig),
  				getGoalsDirFunc(storageConfig),
  				fix,
  			)
  ```

  The five registration sites already supply the correct literal and need **no change**: `createLintCommand` passes `"task"`; `createGoalCommands` passes `"goal"`; `createThemeCommands` passes `"theme"`; `createObjectiveCommands` passes `"objective"`; `createVisionCommands` passes `"vision"`.

  The sixth entry point is `createValidateCommand`, whose single `ExecuteFile` call is:

  ```go
  			lintOp := ops.NewLintOperation()
  			issues, err := lintOp.ExecuteFile(ctx, taskFilePath, taskName, foundInVault.Name)
  ```

  `ops.NewLintOperation()` is constructed inline at both sites (`createGenericLintCommand` and `createValidateCommand`). Keep it that way — do NOT introduce an injection seam here.

- `/workspace/pkg/ops/lint_test.go` — 2432 lines, Ginkgo, `package ops_test`, already imports `github.com/bborbe/vault-cli/pkg/ops`. Verified: **99** occurrences of the literal prefix `lintOp.Execute(ctx, vaultPath` and **4** occurrences of `lintOp.ExecuteFile(`. Every one of the 99 is a task-directory fixture.

- `/workspace/pkg/ops/lint_validate_exit_test.go` — 93 lines, plain `testing` + `NewWithT(t)` gomega, `package ops_test`. Verified: **4** occurrences of `lintOp.ExecuteFile(`. This is the in-repo precedent for a non-Ginkgo test file in `pkg/ops`.

- `/workspace/mocks/lint-operation.go` — counterfeiter fake, regenerated by `make generate` (which runs `rm -rf mocks avro`, recreates `mocks/mocks.go`, then `go generate -mod=mod ./...`). Note `make precommit` already runs `generate`, so requirement 7 is a fast-fail, not the only path. It is not referenced by any test today, but it must still compile.

Linter facts that constrain the shape (from `/workspace/.golangci.yml`): `funlen` is 80 lines / 50 statements, `gocognit` min-complexity 20, `nestif` min-complexity 4, `golines --max-len=100` runs in `make format`. `collectLintIssues` is currently 71 lines — hence requirement 4 below extracts a helper rather than inlining an `if` block.
</context>

<requirements>
1. In `/workspace/pkg/ops/lint.go`, add an exported constant just below the `IssueType` const block:

   ```go
   // PageTypeTask is the lint page type that carries the task-identifier invariants.
   // The two identifier checks (MISSING_TASK_IDENTIFIER, INVALID_TASK_IDENTIFIER) run
   // only when the caller passes this value. Any other page type — including an
   // unrecognised one — skips them. Identifier checks fail open on purpose: a caller
   // that forgets its page type gets no identifier checking rather than a false error
   // on every file it lints.
   const PageTypeTask = "task"
   ```

2. Add `pageType string` as the **second** parameter (immediately after `ctx`) to both methods of the `LintOperation` interface and to both implementations on `*lintOperation`:

   ```go
   type LintOperation interface {
   	Execute(
   		ctx context.Context,
   		pageType string,
   		vaultPath string,
   		tasksDir string,
   		goalsDir string,
   		fix bool,
   	) ([]LintIssue, error)
   	ExecuteFile(
   		ctx context.Context,
   		pageType string,
   		filePath string,
   		taskName string,
   		vaultName string,
   	) ([]LintIssue, error)
   }
   ```

   Keep the `//counterfeiter:generate` directive and `NewLintOperation()`'s signature unchanged.

3. Thread `pageType` down the private call chain, in each case as the parameter immediately following `ctx` (or as the first parameter where the method takes no `ctx`): `lintFile`, `handleMissingFrontmatterCase`, `collectLintIssues`. `Execute` passes its own `pageType` through; `ExecuteFile` passes its own `pageType` through (it must NOT hardcode `PageTypeTask` — the call site decides, see requirement 6).

4. Replace the two unconditional identifier appends at the end of `collectLintIssues` with a single call to a new private helper, and add that helper next to `missingTaskIdentifierIssues`:

   ```go
   	// Identifier checks are task-only — see PageTypeTask.
   	issues = append(issues, l.taskIdentifierIssues(pageType, filePath, frontmatterYAML)...)
   ```

   ```go
   // taskIdentifierIssues returns the identifier issues for a file, but only when the
   // lint entry point declared the task page type. Goals, themes, objectives and vision
   // items have no identifier concept, so a missing or non-UUID task_identifier on them
   // is inert rather than an error.
   func (l *lintOperation) taskIdentifierIssues(
   	pageType string,
   	filePath string,
   	frontmatterYAML string,
   ) []LintIssue {
   	if pageType != PageTypeTask {
   		return nil
   	}
   	issues := l.missingTaskIdentifierIssues(filePath, frontmatterYAML)
   	return append(issues, l.invalidTaskIdentifierIssues(filePath, frontmatterYAML)...)
   }
   ```

   Do NOT change `missingTaskIdentifierIssues`, `detectMissingTaskIdentifier`, or `invalidTaskIdentifierIssues` — including the `Description` string `"task_identifier is missing; run backfill to assign one"`, which the next prompt rewrites. Do NOT change the order of any other check inside `collectLintIssues`.

5. In `/workspace/pkg/cli/cli.go`, inside `createGenericLintCommand`'s `RunE`, pass the factory's existing `pageType` parameter as the second argument to `lintOp.Execute`. Do not touch the five registration sites, the `Short` help text, or `getDirFunc`/`getGoalsDirFunc`.

6. In `/workspace/pkg/cli/cli.go`, inside `createValidateCommand`'s `RunE`, pass `ops.PageTypeTask` as the second argument to `lintOp.ExecuteFile`. This is the one place the task-only nature of the single-file path is made explicit; do not default it inside `pkg/ops`.

7. Run `make generate` to regenerate `/workspace/mocks/lint-operation.go` against the new interface. Do not hand-edit that file.

8. Update the existing test call sites mechanically. This is arity-only — **do not weaken, delete, retarget, or re-fixture a single existing assertion**, and do not change any existing fixture content:

   - In `/workspace/pkg/ops/lint_test.go`: `lintOp.Execute(ctx, vaultPath` → `lintOp.Execute(ctx, ops.PageTypeTask, vaultPath` (99 sites), and `lintOp.ExecuteFile(ctx, ` → `lintOp.ExecuteFile(ctx, ops.PageTypeTask, ` (4 sites).
   - In `/workspace/pkg/ops/lint_validate_exit_test.go`: `lintOp.ExecuteFile(ctx, ` → `lintOp.ExecuteFile(ctx, ops.PageTypeTask, ` (4 sites).

   Every existing fixture in both files is a task file, so `ops.PageTypeTask` preserves current behavior everywhere. Run `make format` afterwards so `golines` re-wraps any line that now exceeds 100 characters.

9. Add a new Ginkgo `Describe("LintOperation - Page Type Scoping", ...)` block at the end of `/workspace/pkg/ops/lint_test.go`. Follow the existing `Describe("LintOperation - Missing Task Identifier", ...)` block for setup style (temp vault via `os.MkdirTemp`, `os.MkdirAll` the collection dir with `0755`, `os.WriteFile` fixtures with `0600`, `AfterEach` cleanup). Create the fixture files under a `goalsDir := "23 Goals"` subdirectory for the goal cases, and pass that same directory as the walk directory argument. Required specs, each with the exact `It` text given so the verification gates can find them:

   - `"reports no identifier issues for a goal file with no task_identifier"` — goal fixture with frontmatter `status: in_progress` and no identifier key; `Execute(ctx, "goal", vaultPath, goalsDir, goalsDir, false)`; assert no returned issue has `ops.IssueTypeMissingTaskIdentifier` and none has `ops.IssueTypeInvalidTaskIdentifier`. (Spec AC 2.)
   - `"reports no identifier issues for a goal file carrying a non-UUID task_identifier"` — goal fixture with `task_identifier: not-a-uuid`; same call; assert zero `ops.IssueTypeInvalidTaskIdentifier` and zero `ops.IssueTypeMissingTaskIdentifier`. (Spec AC 5.)
   - `"still reports exactly one MISSING_TASK_IDENTIFIER for a task file with no task_identifier"` — task fixture in a `Tasks` dir; `Execute(ctx, ops.PageTypeTask, vaultPath, tasksDir, "", false)`; count issues of that type and assert the count is exactly 1. (Spec AC 3.)
   - `"skips identifier checks for an unrecognised page type but still runs the other checks"` — fixture with no `task_identifier` **and** a genuine non-identifier defect (`priority: high`, which `detectInvalidPriority` flags as `INVALID_PRIORITY`); call `Execute` with the literal page type `"widget"`; assert zero identifier issues of either type and at least one `ops.IssueTypeInvalidPriority`. (Failure Modes row 1.)
   - `"keeps running the non-identifier checks for the goal page type"` — goal fixture with a duplicate frontmatter key (e.g. `status:` twice); `Execute(ctx, "goal", ...)`; assert at least one `ops.IssueTypeDuplicateKey`. (Desired Behavior 4.)

10. Add two new top-level test functions to `/workspace/pkg/ops/lint_validate_exit_test.go`, matching that file's existing plain-`testing` + `NewWithT(t)` style and its `os.CreateTemp` / `defer os.Remove` fixture pattern:

    - `func TestValidateExecuteFileTaskMissingIdentifier(t *testing.T)` — temp file with `"---\nstatus: in_progress\npriority: 1\n---\n# Task\n"`; call `lintOp.ExecuteFile(ctx, ops.PageTypeTask, f.Name(), "Test Task", "test")`; assert exactly one issue of type `ops.IssueTypeMissingTaskIdentifier`. (Spec AC 4 — this is the single-file `vault-cli task validate` path.)
    - `func TestValidateExecuteFileNonTaskPageTypeSkipsIdentifier(t *testing.T)` — same fixture; call with page type `"goal"`; assert zero issues of type `ops.IssueTypeMissingTaskIdentifier`. (Failure Modes row 3 detection.)

11. Add a `## Unreleased` section to `/workspace/CHANGELOG.md`, immediately below the `All notable changes…` preamble block and immediately above `## v0.111.4`, containing exactly one bullet for this change. Use the `fix:` prefix and name the behavior, for example:

    ```
    ## Unreleased

    - fix: `goal`, `theme`, `objective` and `vision` lint no longer report `MISSING_TASK_IDENTIFIER` / `INVALID_TASK_IDENTIFIER`. `task_identifier` is a task-only invariant, but the lint walker applied it to whatever collection it was pointed at, flagging 268 of 269 goals across all vaults. The page type now comes from the lint entry point — not from frontmatter, which only 178 of those 269 goals carry — and the two identifier checks run only for the task page type. `task lint` and `task validate` are unchanged.
    ```

    Do NOT create a `## vX.Y.Z` section, do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT tag. The `github-releaser` owns version bumps for this repo (`.maintainer.yaml` sets `autoRelease: true`).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- `vault-cli task lint` and `vault-cli task validate` behavior for task files must not change at all in this prompt: same issue types, same set of flagged files, same descriptions, same exit codes. The `MISSING_TASK_IDENTIFIER` description string is rewritten by the **next** prompt, not this one — leave `"task_identifier is missing; run backfill to assign one"` exactly as it is.
- **Page type comes from the lint entry point, never from frontmatter.** Do NOT read `page_type` out of the file to decide whether identifier checks apply. Only 178 of 269 goals carry that key, so frontmatter-driven detection would leave 91 goals still flagged, and a "missing `page_type` ⇒ skip" default would silently stop checking tasks that lack the key too.
- Do NOT introduce `goal_identifier`, any goal identifier, or any new frontmatter key. The lint path must not write any key to any file.
- Do NOT add a flag, config key, or environment variable to re-enable identifier checks on non-task page types. This is an invariant, not a tunable.
- Do NOT make `MISSING_TASK_IDENTIFIER` fixable via `--fix`. It stays `Fixable: false`.
- Do NOT change `WriteTask`'s UUID auto-generation in `pkg/storage/task.go`, or anything else under `pkg/storage/`.
- Do NOT change the `agents/goal-creator.md` or `agents/task-creator.md` definitions, or anything under `agents/` or `commands/`. A creator-side fix was already tried and rejected (PR #92, closed unmerged) — it would fix zero existing files.
- Do NOT change the lint JSON output shape: `LintIssueJSON` and `ValidateResult` field sets are a public contract for the `/vault-cli:*` slash commands.
- Do NOT refactor `ops.NewLintOperation()` to be injected at either CLI call site. Every command in `pkg/cli/cli.go` constructs its operation inline; keep it that way here.
- All other lint checks (duplicate keys, invalid priority, invalid status, status/phase, status/date, orphan goals, status/checkbox) must continue to run for every page type exactly as today.
- Existing tests in `pkg/ops/lint_test.go` and `pkg/ops/lint_validate_exit_test.go` must still pass. The only permitted edit to an existing test body is the mechanical arity update in requirement 8.
- `pkg/ops/` stays a library layer: it returns structured results and never writes to stdout. The CLI layer owns all output formatting.
- Do NOT run `go mod vendor` and do NOT pass `-mod=vendor` to any command; this repo does not commit `vendor/`.
</constraints>

<verification>
Run all of the following from `/workspace`.

The gate is in place and correctly named:

```
grep -c 'const PageTypeTask = "task"' pkg/ops/lint.go
grep -c 'l\.taskIdentifierIssues(' pkg/ops/lint.go
grep -c 'if pageType != PageTypeTask {' pkg/ops/lint.go
```

Each must print exactly `1`.

The identifier checks are no longer reachable unconditionally — they are called only from the gated helper:

```
grep -c 'l\.missingTaskIdentifierIssues(' pkg/ops/lint.go
grep -c 'l\.invalidTaskIdentifierIssues(' pkg/ops/lint.go
```

Each must print exactly `1`.

The single-file path is told explicitly that it is a task path:

```
grep -c 'ops.PageTypeTask' pkg/cli/cli.go
```

Must print exactly `1`.

`pkg/ops` does not default or hardcode the page type anywhere in the call chain:

```
grep -c 'lintFile(ctx, PageTypeTask' pkg/ops/lint.go
grep -c 'ExecuteFile(ctx, PageTypeTask' pkg/ops/lint.go
grep -c 'collectLintIssues(PageTypeTask' pkg/ops/lint.go
```

Each must print exactly `0`.

The message text is untouched in this prompt:

```
grep -c 'run backfill to assign one' pkg/ops/lint.go
```

Must print exactly `1`.

The regenerated mock matches the new interface:

```
grep -c 'ExecuteStub .*func(context.Context, string, string, string, string, bool)' mocks/lint-operation.go
grep -c 'ExecuteFileStub .*func(context.Context, string, string, string, string)' mocks/lint-operation.go
```

Each must print exactly `1`.

The new specs exist and run:

```
go test -mod=mod -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "reports no identifier issues for a goal file with no task_identifier"
go test -mod=mod -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "reports no identifier issues for a goal file carrying a non-UUID task_identifier"
go test -mod=mod -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "still reports exactly one MISSING_TASK_IDENTIFIER for a task file with no task_identifier"
go test -mod=mod -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "skips identifier checks for an unrecognised page type but still runs the other checks"
go test -mod=mod -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "keeps running the non-identifier checks for the goal page type"
grep -c 'func TestValidateExecuteFileTaskMissingIdentifier' pkg/ops/lint_validate_exit_test.go
grep -c 'func TestValidateExecuteFileNonTaskPageTypeSkipsIdentifier' pkg/ops/lint_validate_exit_test.go
```

Each must print a number `>= 1`.

Do **not** pass `-run` to those commands. The Ginkgo entry point is `func TestSuite(t *testing.T)` in `pkg/ops/ops_suite_test.go`; a wrong `-run` value matches nothing, prints `[no tests to run]`, and still exits 0.

Both affected packages are green:

```
go test -mod=mod -count=1 ./pkg/ops/... ./pkg/cli/...
```

Must exit 0.

Nothing outside the intended surface moved. These files must NOT appear in the change:

```
grep -c 'SetTaskIdentifier(uuid.New().String())' pkg/storage/task.go
grep -c 'goal_identifier' pkg/domain/goal.go
```

The first must still print exactly `1` (the existing UUID auto-generation in `WriteTask` is untouched); the second must print `0`.

Changelog is an `## Unreleased` entry only, and no version string moved:

```
grep -c '^## Unreleased$' CHANGELOG.md
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

The first must print exactly `1`. The second must still print `## v0.111.4`. The third must still show `0.111.4`.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make vet`, `make test`, …) until it passes, then run `make precommit` once more. A non-zero exit code from `make precommit` means `"status":"failed"` in the completion report — no exceptions.
</verification>
