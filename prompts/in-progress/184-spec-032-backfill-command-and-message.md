---
status: approved
spec: [032-bug-task-identifier-lint-fires-on-goals]
created: "2026-08-17T08:20:00Z"
queued: "2026-08-17T08:36:53Z"
branch: dark-factory/bug-task-identifier-lint-fires-on-goals
---

# Wire the identifier backfill command and make the lint message name it

<summary>
- Adds a real command that assigns the missing identifier to every task that lacks one, across the selected vault or across all vaults.
- The command reports how many task files it changed and how many it skipped, and running it twice is safe — tasks that already have an identifier are left untouched.
- The lint error that fires on a task with no identifier stops telling the operator to "run backfill", a phrase that pointed at nothing, and instead quotes the exact command they can copy into a shell.
- This matters right now: 248 real tasks across the five vaults are correctly flagged today and have had no available remedy.
- The command is proved to be genuinely wired — a test drives the command's own run function with a stand-in operation and checks it was invoked once for each selected vault, rather than only checking that a help entry exists.
- A task file that cannot be written is skipped and counted rather than aborting the run; a vault whose task directory is missing fails loudly with a non-zero exit and writes nothing.
- Interrupting the run keeps the identifiers already written and still reports the partial count, so re-running finishes the job.
- The command takes only the vault-selection flags every other multi-vault command takes; no dry-run, limit, or tuning knobs.
- The README task section and the task frontmatter schema doc both name the new command.
</summary>

<objective>
`vault-cli task backfill-identifiers` must exist and actually assign `task_identifier` to tasks that lack one, and the `MISSING_TASK_IDENTIFIER` lint message must name that command verbatim instead of the word "backfill". Today the error tells 248 legitimately-flagged tasks' owner to run an operation that exists in the codebase but is wired to no CLI command, which makes the error unactionable.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/CLI layering rules.

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — cobra command construction
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega suite style
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter fakes
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)`, never `fmt.Errorf`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format

**Precondition — prompt 1 (`spec-032` page-type gate) must have shipped.** Verify before writing anything:

```
grep -c 'const PageTypeTask = "task"' pkg/ops/lint.go
```

If that prints `0`, prompt 1 has not landed. STOP, report `"status":"failed"` with the message `page-type gate not yet deployed (prompt 1)`, and do NOT work around it by adding the gate yourself.

Read these files fully before making changes:

- `/workspace/pkg/ops/ensure_task_identifiers.go` — the operation being wired. Verified, complete and already tested:

  ```go
  // BackfillResult holds the outcome of an EnsureAllTaskIdentifiers run.
  type BackfillResult struct {
  	// ModifiedFiles is the list of absolute file paths that were written during backfill.
  	ModifiedFiles []string
  	// SkippedFiles is the count of files skipped due to errors.
  	SkippedFiles int
  }

  //counterfeiter:generate -o ../../mocks/ensure-all-task-identifiers-operation.go --fake-name EnsureAllTaskIdentifiersOperation . EnsureAllTaskIdentifiersOperation
  type EnsureAllTaskIdentifiersOperation interface {
  	Execute(ctx context.Context, vaultPath string) (BackfillResult, error)
  }

  func NewEnsureAllTaskIdentifiersOperation(
  	taskStorage storage.TaskStorage,
  ) EnsureAllTaskIdentifiersOperation
  ```

  Its `Execute` already implements every failure mode this prompt needs: it checks `ctx.Done()` per task and returns the partial result on cancellation; it skips tasks that already carry an identifier; it increments `SkippedFiles` and continues when `WriteTask` fails; and it returns `errors.Wrap(ctx, err, "list tasks")` when the walk fails (`ListTasks` wraps `filepath.WalkDir`'s error, which names the missing directory). **Do not modify this file** — it needs no changes.

- `/workspace/mocks/ensure-all-task-identifiers-operation.go` — existing counterfeiter fake. Verified: `type EnsureAllTaskIdentifiersOperation struct` with `ExecuteReturns(ops.BackfillResult, error)`, `ExecuteCallCount() int`, and `ExecuteArgsForCall(i int) (context.Context, string)`.

- `/workspace/pkg/storage/storage.go` — verified constructors used by the production call site:

  ```go
  func NewConfigFromVault(vault *config.Vault) *Config
  func NewTaskStorage(storageConfig *Config) TaskStorage
  ```

  `Config` carries per-vault `TasksDir` and `Excludes`, so the task storage — and therefore the operation — must be built **per vault**, not once for the whole command. That is why the injection seam in requirement 1 is a factory function and not a bare operation value.

- `/workspace/pkg/cli/cli.go` — verified helpers the new command reuses:

  ```go
  func getVaults(
  	ctx context.Context,
  	configLoader *config.Loader,
  	vaultName *string,
  ) ([]*config.Vault, error)
  ```

  `getVaults` returns just the named vault when `*vaultName != ""`, otherwise every configured vault. Verified multi-vault plain-output header convention, from `createGenericLintCommand`'s `RunE`:

  ```go
  			for _, vault := range vaults {
  				if len(vaults) > 1 && OutputFormat(*outputFormat).IsPlain() {
  					fmt.Printf("=== %s ===\n", vault.Name)
  				}
  ```

  Verified JSON helpers in `/workspace/pkg/cli/output.go`: `OutputFormat(s).IsJSON()`, `OutputFormat(s).IsPlain()`, `PrintJSON(v any) error`. `--vault`, `--config`, `--output` and `--verbose` are persistent root flags — the new command declares no flags of its own.

  Verified task command tree, in `createTaskCommands`:

  ```go
  	cmd.AddCommand(createTaskListCommand(ctx, configLoader, vaultName, outputFormat))
  	cmd.AddCommand(createLintCommand(ctx, configLoader, vaultName, outputFormat))
  	cmd.AddCommand(createValidateCommand(ctx, configLoader, vaultName, outputFormat))
  ```

  `cli.go` is 2379 lines — over the 2000-line smell threshold — so the new command goes in its own file in the same package, alongside the existing `pkg/cli/output.go`.

- `/workspace/pkg/storage/export_test.go` — the in-repo precedent for reaching an unexported symbol from an external `_test` package. Verified shape:

  ```go
  package storage

  // Test-only exports of unexported baseStorage methods.
  // These functions are only visible to _test.go files in package storage_test.

  type BaseStorageForTest = baseStorage

  func NewBaseStorageForTest() *BaseStorageForTest { … }
  ```

  Every test file in `pkg/cli` is `package cli_test` (`cli_suite_test.go`, `output_test.go`, `watch_test.go`), so the new command factory is reached the same way.

- `/workspace/mocks/config-loader.go` — verified: `type Loader struct` implementing `config.Loader`, with `GetVaultReturns(*config.Vault, error)` and `GetAllVaultsReturns([]*config.Vault, error)`. `config.Vault` has `Name` and `Path` string fields.

- `/workspace/pkg/ops/lint.go` — the message to reword, in `missingTaskIdentifierIssues`. Verified current value:

  ```go
  		Description: "task_identifier is missing; run backfill to assign one",
  ```

- `/workspace/README.md` — verified the task command block ends with these lines:

  ```
  vault-cli task lint                                # Detect frontmatter issues
  vault-cli task lint --fix                          # Auto-fix frontmatter issues
  vault-cli task validate "Build vault-cli Go Tool"  # Validate a single task
  ```

- `/workspace/docs/task-writing.md` — verified the schema line to update:

  ```
  task_identifier: <uuid>                          # generated by tooling
  ```
</context>

<requirements>
1. Create `/workspace/pkg/cli/task_backfill_identifiers.go` (`package cli`, BSD license header with year 2026 matching the other 2026 files in this package). It contains the injection seam and the command factory:

   ```go
   func createTaskBackfillIdentifiersCommand(
   	ctx context.Context,
   	configLoader *config.Loader,
   	vaultName *string,
   	outputFormat *string,
   	newBackfillOp func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation,
   ) *cobra.Command
   ```

   This factory shape is **not new** — it matches the existing convention used by `createEntityGetCommand` / `createEntitySetCommand` / `createEntityClearCommand` / `createEntityShowCommand` / `createEntityListAddCommand` / `createEntityListRemoveCommand` in the same file (e.g. `newAddOp func(cfg *storage.Config) ops.EntityListAddOperation` at `cli.go:1043`). The operation depends on a vault's task storage config, so the closure is built per vault inside `RunE` — exactly as `createGenericLintCommand` already builds `storage.NewConfigFromVault(vault)` per vault. No named type alias; declare the parameter inline as the six entity commands do.

   The returned command:
   - `Use: "backfill-identifiers"`, `Args: cobra.NoArgs`, `Short: "Assign task_identifier to tasks that are missing one"`.
   - Declares **no** flags of its own.
   - `RunE`: call `getVaults(ctx, configLoader, vaultName)` and wrap a failure with `errors.Wrap(ctx, err, "get vaults")` (same as `createGenericLintCommand`). Then loop over the vaults in order; for each, build `storageConfig := storage.NewConfigFromVault(vault)` and call `newBackfillOp(storageConfig).Execute(ctx, vault.Path)`. On error: **first print the partial counts for that vault (plain output) exactly as the success path does, then** wrap with `errors.Wrapf(ctx, err, "backfill identifiers in vault %s", vault.Name)` and return it immediately. Printing before returning is required: `Execute` returns a *populated* `BackfillResult` alongside the wrapped error when the run is interrupted, and the spec's failure-mode row requires already-written identifiers to be reported rather than discarded. A vault whose task directory is missing still fails loudly with a non-zero exit (its result is the zero value, so it prints `modified: 0` / `skipped: 0` and then errors) rather than being silently counted as a success.
   - Plain output: print the multi-vault `=== %s ===` header exactly as `createGenericLintCommand` does when `len(vaults) > 1`, then per vault print exactly two lines with `fmt.Printf`:

     ```
     modified: <len(result.ModifiedFiles)>
     skipped: <result.SkippedFiles>
     ```

   - JSON output: accumulate one entry per vault and emit a single `PrintJSON` call after the loop. The entry type is private to this file and has exactly three fields:

     ```go
     type backfillResultJSON struct {
     	Vault    string `json:"vault"`
     	Modified int    `json:"modified"`
     	Skipped  int    `json:"skipped"`
     }
     ```

     JSON support is not new configurability — `--output` is a persistent root flag that every sibling command already honors, and the `/vault-cli:*` slash commands parse it.

2. Register the command in `createTaskCommands` in `/workspace/pkg/cli/cli.go`, immediately after the `createValidateCommand` line, passing the production factory:

   ```go
   	cmd.AddCommand(createTaskBackfillIdentifiersCommand(
   		ctx, configLoader, vaultName, outputFormat,
   		func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation {
   			return ops.NewEnsureAllTaskIdentifiersOperation(
   				storage.NewTaskStorage(cfg),
   			)
   		},
   	))
   ```

   This follows the existing factory-injection convention already used by the six entity commands in the same file — read the `createEntityListAddCommand` registration in `cli.go` (~9 lines above your insertion point) and match its shape. Do **not** refactor any of the remaining inline `ops.New*` constructions in `cli.go`.

3. Create `/workspace/pkg/cli/export_test.go` (`package cli`, license header) following the `pkg/storage/export_test.go` pattern, exposing exactly the one symbol the test needs and nothing else:

   ```go
   // Test-only exports for package cli.
   // These are only visible to _test.go files in package cli_test.

   // CreateTaskBackfillIdentifiersCommandForTest exposes
   // createTaskBackfillIdentifiersCommand for testing.
   func CreateTaskBackfillIdentifiersCommandForTest(
   	ctx context.Context,
   	configLoader *config.Loader,
   	vaultName *string,
   	outputFormat *string,
   	newBackfillOp func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation,
   ) *cobra.Command {
   	return createTaskBackfillIdentifiersCommand(
   		ctx, configLoader, vaultName, outputFormat, newBackfillOp,
   	)
   }
   ```

4. Create `/workspace/pkg/cli/task_backfill_identifiers_test.go` (`package cli_test`, Ginkgo, license header). This is the first mock-based test in `pkg/cli`; it uses `mocks.Loader` for the config loader and `mocks.EnsureAllTaskIdentifiersOperation` for the operation. Because `configLoader` is a `*config.Loader` (pointer to interface), assign the fake into an interface variable first and pass its address. Required specs, each with the exact `It` text given:

   - `"invokes the backfill operation once for each vault when no vault is selected"` — `fakeLoader.GetAllVaultsReturns([]*config.Vault{{Name: "alpha", Path: "/tmp/alpha"}, {Name: "beta", Path: "/tmp/beta"}}, nil)`; `vaultName := ""`; factory closure returns one shared `fakeOp`; run `cmd.RunE(cmd, nil)`; assert no error, `fakeOp.ExecuteCallCount() == 2`, and that the vault-path argument of call 0 is `/tmp/alpha` and of call 1 is `/tmp/beta` (via `ExecuteArgsForCall`). (Spec AC 6.)
   - `"invokes the backfill operation exactly once for the selected vault"` — `fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)`; `vaultName := "alpha"`; assert `fakeOp.ExecuteCallCount() == 1` and the vault-path argument is `/tmp/alpha`. (Spec AC 6.)
   - `"builds the operation from the vault it is about to process"` — assert the factory closure was called once per vault and received a `*storage.Config` whose `TasksDir` matches the vault's `tasks_dir` (record the `cfg.TasksDir` values the closure receives in a slice and compare). This is what stops the command from being wired to a single mis-scoped operation.
   - `"returns the error when the backfill operation fails"` — `fakeOp.ExecuteReturns(ops.BackfillResult{}, errors.Errorf(ctx, "list tasks: no such directory"))` using `github.com/bborbe/errors` (`New`/`Errorf` there take `ctx` as the first argument); assert `cmd.RunE(cmd, nil)` returns an error whose message contains the vault name.
   - `"prints the partial counts and still returns the error when the run is cancelled"` — `fakeOp.ExecuteReturns(ops.BackfillResult{ModifiedFiles: []string{"/tmp/alpha/Tasks/A.md"}, SkippedFiles: 0}, context.Canceled)`; single vault; capture stdout; assert `cmd.RunE(cmd, nil)` returns an error **and** the captured output contains `modified: 1`. (Spec failure mode "Backfill interrupted mid-run".)
   - `"reports the modified and skipped counts it received"` — `fakeOp.ExecuteReturns(ops.BackfillResult{ModifiedFiles: []string{"/tmp/alpha/Tasks/A.md"}, SkippedFiles: 2}, nil)`; single vault; assert no error. Capturing stdout is not required here — the end-to-end count assertion is done against the real binary in `<verification>`.

5. In `/workspace/pkg/ops/lint.go`, change the `Description` in `missingTaskIdentifierIssues` to name the command verbatim:

   ```go
   		Description: "task_identifier is missing; run: vault-cli task backfill-identifiers",
   ```

   The literal `vault-cli task backfill-identifiers` must appear in the string, because an operator copies the message text straight into a shell. Change nothing else about the issue: it stays `Fixable: false`, `IssueTypeMissingTaskIdentifier`, and is still emitted only for the task page type via the gate added in prompt 1.

6. Add a spec to the existing `Describe("LintOperation - Missing Task Identifier", …)` block in `/workspace/pkg/ops/lint_test.go`, named exactly `"names a runnable command in the MISSING_TASK_IDENTIFIER description"`: task fixture with no `task_identifier`; call `Execute` with `ops.PageTypeTask`; find the `ops.IssueTypeMissingTaskIdentifier` issue and assert its `Description` contains the substring `vault-cli task backfill-identifiers`. This pins the message contract that the operator relies on.

7. Add one line to the task command block in `/workspace/README.md`, immediately after the `vault-cli task validate …` line, keeping the existing comment-column alignment:

   ```
   vault-cli task backfill-identifiers                # Assign task_identifier to tasks missing one
   ```

8. In `/workspace/docs/task-writing.md`, change the `task_identifier` schema line so it names the command that assigns one:

   ```
   task_identifier: <uuid>                          # generated by tooling; backfill with `vault-cli task backfill-identifiers`
   ```

9. Append bullets to the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (it already exists at `CHANGELOG.md:25`, positioned below the `## v0.111.2` section — leave it where it is; do not replace it, do not create a second one). Add exactly two bullets, one per half of this change, for example:

   ```
   - feat: `vault-cli task backfill-identifiers` assigns `task_identifier` to every task missing one in the selected vault (or all vaults), skipping tasks that already have one and reporting modified/skipped counts. Wires the existing `EnsureAllTaskIdentifiersOperation`, which had no CLI entry point.
   - fix: the `MISSING_TASK_IDENTIFIER` lint message named "backfill", an operation no command exposed, so the 248 tasks across five vaults that legitimately lack `task_identifier` pointed at a remedy nobody could run. It now names `vault-cli task backfill-identifiers` verbatim.
   ```

   Do NOT create a `## vX.Y.Z` section, do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT tag. The `github-releaser` owns version bumps for this repo (`.maintainer.yaml` sets `autoRelease: true`).
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- **The command name `task backfill-identifiers` is pinned.** It is written into a persistent user-visible error message and into the spec's reproduction. Do not rename it, do not alias it, do not add a shorter form.
- Do NOT add `--dry-run`, `--limit`, `--force`, or any other tuning flag to the new command. It takes only the vault-selection flags every other multi-vault command already takes (`--vault`, `--config`), which are persistent root flags it inherits for free.
- **The injection seam is authorised for this one command only.** Every other command in `pkg/cli/cli.go` constructs its operation inline inside `RunE` (67 `ops.New*` sites). Do not refactor any of them.
- Do NOT modify `/workspace/pkg/ops/ensure_task_identifiers.go` or its test. The operation is complete and already handles cancellation, skip-on-write-error, and idempotency.
- **Known deviation from the spec's failure-modes table, do NOT "fix" it here.** `storage.ListTasks` drops files it cannot parse with a `slog.Debug` and no counter (`pkg/storage/task.go`, inside the `WalkDir` callback), so an *unparseable* file is invisible to the operation and does not appear in `SkippedFiles`. `SkippedFiles` counts `WriteTask` failures only. Changing that requires reworking `ListTasks`, which is out of scope for this prompt — file a follow-up if the count is wanted.
- **Another spec-row deviation, also frozen:** the spec says *"Disk full / read-only vault during backfill → Command exits non-zero naming the file that failed"*, but `Execute` swallows `WriteTask` errors into `SkippedFiles` and returns `nil`. That is the operation's designed behavior — do not make the command fail on write errors to satisfy the spec text.
- Do NOT change `WriteTask`'s existing UUID auto-generation in `pkg/storage/task.go`. The new command reaches identifier generation through the existing task storage write path and must not mint UUIDs itself.
- `pkg/ops/` stays a library layer: the operation returns a structured `BackfillResult` and the CLI layer owns all output formatting. Do not add printing to `pkg/ops/`.
- `vault-cli task lint` and `vault-cli task validate` behavior for task files must not change **except** for the `MISSING_TASK_IDENTIFIER` description string — same issue types, same set of flagged files, same exit codes. That one string is the entire permitted user-visible delta.
- Do NOT make `MISSING_TASK_IDENTIFIER` fixable via `--fix`. It stays `Fixable: false`.
- Do NOT change the lint JSON output shape: `LintIssueJSON` and `ValidateResult` field sets are a public contract for the `/vault-cli:*` slash commands.
- Do NOT introduce `goal_identifier`, any goal identifier, or any new frontmatter key, and do NOT migrate or remove the 6 existing inert `goal_identifier:` keys or the 1 stray `task_identifier:` on a goal.
- Do NOT change anything under `agents/` or `commands/`.
- Do NOT undo prompt 1's page-type gate. `grep -c 'const PageTypeTask = "task"' pkg/ops/lint.go` must still print `1` at the end of this prompt.
- Do NOT run `go mod vendor` and do NOT pass `-mod=vendor` to any command; this repo does not commit `vendor/`.
</constraints>

<verification>
Run all of the following from `/workspace`.

The old, unrunnable remedy is gone and the new one is named verbatim:

```
grep -c 'run backfill to assign one' pkg/ops/lint.go
grep -c 'vault-cli task backfill-identifiers' pkg/ops/lint.go
```

The first must print exactly `0`. The second must print exactly `1`.

Prompt 1's gate is still intact:

```
grep -c 'const PageTypeTask = "task"' pkg/ops/lint.go
grep -c 'l\.taskIdentifierIssues(' pkg/ops/lint.go
```

Each must print exactly `1`.

The command is wired into the task tree behind the injected factory, and the seam was not spread anywhere else:

```
grep -c 'createTaskBackfillIdentifiersCommand' pkg/cli/cli.go
grep -c 'ops.NewEnsureAllTaskIdentifiersOperation' pkg/cli/cli.go
grep -c 'newBackfillOp' pkg/cli/task_backfill_identifiers.go
```

The first two must each print exactly `1`. The third must print `>= 2` (the factory parameter declaration and its uses).

The mock-based CLI test exists and asserts real invocation:

```
grep -c 'mocks.EnsureAllTaskIdentifiersOperation' pkg/cli/task_backfill_identifiers_test.go
grep -c 'ExecuteCallCount()' pkg/cli/task_backfill_identifiers_test.go
go test -mod=mod -count=1 ./pkg/cli/ -v -ginkgo.v 2>&1 | grep -c "invokes the backfill operation once for each vault when no vault is selected"
go test -mod=mod -count=1 ./pkg/cli/ -v -ginkgo.v 2>&1 | grep -c "invokes the backfill operation exactly once for the selected vault"
go test -mod=mod -count=1 ./pkg/cli/ -v -ginkgo.v 2>&1 | grep -c "builds the operation from the vault it is about to process"
go test -mod=mod -count=1 ./pkg/cli/ -v -ginkgo.v 2>&1 | grep -c "returns the error when the backfill operation fails"
go test -mod=mod -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "names a runnable command in the MISSING_TASK_IDENTIFIER description"
```

Each must print a number `>= 1`. Do **not** pass `-run` — the Ginkgo entry points are `func TestSuite(t *testing.T)` in `pkg/cli/cli_suite_test.go` and `pkg/ops/ops_suite_test.go`; a wrong `-run` value matches nothing, prints `[no tests to run]`, and still exits 0.

Both affected packages are green:

```
go test -mod=mod -count=1 ./pkg/ops/... ./pkg/cli/...
```

Must exit 0.

End-to-end against a real binary built from this tree — this is the container-side replay of the spec's scratch-vault acceptance criterion. Run each block and check the stated expectation:

```
go build -mod=mod -o /tmp/vault-cli-verify .
/tmp/vault-cli-verify task backfill-identifiers --help
```

The build must exit 0 and `--help` must exit 0 and print the `backfill-identifiers` usage line.

```
rm -rf /tmp/scratch-vault && mkdir -p /tmp/scratch-vault/Tasks
printf -- '---\nstatus: in_progress\npage_type: task\npriority: 1\n---\n# Scratch Task\n' > '/tmp/scratch-vault/Tasks/Scratch Task.md'
printf -- 'vaults:\n  scratch:\n    name: scratch\n    path: /tmp/scratch-vault\n' > /tmp/scratch-config.yaml
/tmp/vault-cli-verify task lint --config /tmp/scratch-config.yaml --vault scratch | grep -c MISSING_TASK_IDENTIFIER
```

That last count must print exactly `1` (the pre-state — the task is correctly flagged), and the printed line must contain `vault-cli task backfill-identifiers`. Confirm:

```
/tmp/vault-cli-verify task lint --config /tmp/scratch-config.yaml --vault scratch | grep -c 'vault-cli task backfill-identifiers'
```

Must print exactly `1`.

Now run the remedy the message names and prove it works:

```
/tmp/vault-cli-verify task backfill-identifiers --config /tmp/scratch-config.yaml --vault scratch
```

Must exit 0 and its stdout must contain the lines `modified: 1` and `skipped: 0`. Then:

```
grep -c '^task_identifier: ' '/tmp/scratch-vault/Tasks/Scratch Task.md'
/tmp/vault-cli-verify task lint --config /tmp/scratch-config.yaml --vault scratch | grep -c MISSING_TASK_IDENTIFIER
```

The first must print exactly `1`. The second must print exactly `0`.

Re-running is idempotent — nothing is modified the second time:

```
/tmp/vault-cli-verify task backfill-identifiers --config /tmp/scratch-config.yaml --vault scratch
```

Must exit 0 and print `modified: 0` and `skipped: 0`.

A vault whose task directory does not exist fails loudly and writes nothing:

```
rm -rf /tmp/missing-vault && mkdir -p /tmp/missing-vault
printf -- 'vaults:\n  missing:\n    name: missing\n    path: /tmp/missing-vault\n' > /tmp/missing-config.yaml
/tmp/vault-cli-verify task backfill-identifiers --config /tmp/missing-config.yaml --vault missing; echo "exit=$?"
ls -A /tmp/missing-vault | wc -l
```

The `exit=` line must show a non-zero code and the error must name the missing path. The `ls` count must print `0`.

JSON output is a parsed contract for the `/vault-cli:*` slash commands — exercise it once:

```
/tmp/vault-cli-verify task backfill-identifiers --config /tmp/scratch-config.yaml --vault scratch --output json
```

Must exit 0 and emit a single JSON array whose one element has exactly the keys `vault`, `modified`, `skipped` — verify with:

```
/tmp/vault-cli-verify task backfill-identifiers --config /tmp/scratch-config.yaml --vault scratch --output json | grep -c '"vault":"scratch"'
```

Must print exactly `1`.

Non-task lint on the same binary reports no identifier issues (prompt 1's gate, re-checked end to end):

```
mkdir -p '/tmp/scratch-vault/23 Goals'
printf -- '---\nstatus: in_progress\n---\n# Scratch Goal\n' > '/tmp/scratch-vault/23 Goals/Scratch Goal.md'
printf -- 'vaults:\n  scratch:\n    name: scratch\n    path: /tmp/scratch-vault\n    goals_dir: 23 Goals\n' > /tmp/scratch-config.yaml
/tmp/vault-cli-verify goal lint --config /tmp/scratch-config.yaml --vault scratch | grep -c 'TASK_IDENTIFIER'
```

Must print exactly `0`.

Docs and changelog name the command:

```
grep -c 'backfill-identifiers' README.md
grep -c 'backfill-identifiers' docs/task-writing.md
sed -n '/^## Unreleased$/,/^## v/p' CHANGELOG.md | grep -c 'backfill-identifiers'
```

Each must print a number `>= 1`. The third proves the entry is **inside** the `## Unreleased` section, not just somewhere in the file.

Changelog is still a single `## Unreleased` section and no version string moved:

```
grep -c '^## Unreleased$' CHANGELOG.md
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

The first must print exactly `1`. The second must still print `## v0.111.4`. The third must still show `0.111.4`.

Out-of-scope files must not have moved:

```
grep -c 'func (e \*ensureAllTaskIdentifiersOperation) Execute' pkg/ops/ensure_task_identifiers.go
grep -c 'uuid.NewString\|uuid.New()' pkg/storage/task.go
```

Each must print exactly `1`. The first proves the operation itself is untouched; the second proves `WriteTask`'s `task.SetTaskIdentifier(uuid.New().String())` auto-generation is untouched.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make vet`, `make test`, …) until it passes, then run `make precommit` once more. A non-zero exit code from `make precommit` means `"status":"failed"` in the completion report — no exceptions.
</verification>
