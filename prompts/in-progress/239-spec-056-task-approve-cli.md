---
status: approved
spec: [056-task-approve-command]
created: "2026-09-28T07:54:18Z"
queued: "2026-09-28T09:12:46Z"
---

# Wire `vault-cli task approve` and prove the transition end to end (spec 056, prompt 2 of 5)

<summary>
- Exposes the approval operation as `vault-cli task approve <task-name>`, so an operator can perform the `todo → planning` transition from the command line instead of hand-editing frontmatter.
- The approver defaults to `operator` and can be named with `--by`; an explicitly empty `--by` is refused rather than recorded as a blank.
- Plain output names the task and the phase it landed in; `--output json` emits the task name and the new phase.
- A refusal exits non-zero in both output modes — including `--output json`, where the error object is printed and the command still fails.
- Proves the whole transition against a real binary and a real temp vault: the four recorded facts, byte-for-byte preservation of every unrelated key, and the refusals for a task past the inbox and for a task that already carries an approval record.
- Registers the new verb in the integration command-registration table so the project's Definition of Done is met.
- Covers spec 056 ACs 1–6 and 9.
- Does not touch `task set`, the phase-regression guard, or any reader — this spec adds the writer only.
</summary>

<objective>
Wire the `pkg/ops` approve operation into the Cobra command tree as `vault-cli task approve <task-name> [--by <approver>]`, with plain and JSON output, and add the integration specs that assert the recorded transition, the byte-for-byte key preservation, and every refusal against a real binary built from HEAD. This prompt covers spec 056 ACs 1–6 and 9. It depends on prompt 1 (the operation it calls) and is the precondition for prompt 3 (the docs and CHANGELOG that describe the surface it creates).
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**AC-mapping note, read before you plan.** Spec 056's `## Suggested Decomposition` table lists ACs 1–8 under prompt 1, but ACs 1–6 are binary-level assertions (`<bin> --config <cfg> task approve Alpha …`) that cannot run before the CLI exists. The operation-level substance of those ACs is covered by prompt 1's unit specs; the binary-level ACs 1–6 are delivered **here**, together with the CLI they invoke. AC 9 (the output paths) is also delivered here.

Read fully (in this order):
- `pkg/ops/task_approve.go` — the file prompt 1 added. Read its `Execute` signature before you write the call site.
- `pkg/cli/cli.go` — `createTaskSetCommand` (definition near line 2511) and `createTaskClearCommand` immediately below it. These are the shape to copy: `getVaults` → `ops.NewVaultDispatcher().FirstSuccess` → build the operation per vault → format output. Also read `createTaskCommands` (definition near line 1305) for where the new command is registered, and `createTaskShowCommand` for a second example of the JSON/plain split.
- `pkg/cli/cli.go` — `runMutation` (near line 113) and `createCompleteCommand` (near line 233) only if you want the alternative shape. **Do not use `runMutation` here**: it prints the `MutationResult` struct as JSON, and `MutationResult` has no phase field, so AC 9's "stdout parses as JSON carrying the task name and the new `phase`" cannot be met through it.
- `pkg/cli/output.go` — `OutputFormat.IsJSON()` and `PrintJSON`. Note `PrintJSON` encodes with a two-space indent, so the whole stdout is multi-line JSON.
- `pkg/ops/vault_dispatcher.go` — `FirstSuccess` semantics: a single vault calls the callback directly and propagates its error; with several vaults only a `storage.ErrNotFound`-class error lets the loop continue. The approve refusal is not `ErrNotFound`-class, so it stops the loop — which is the intent.
- `pkg/domain/task_phase.go` — `domain.TaskPhasePlanning`, so the phase string in the output is not a bare literal.
- `integration/cli_test.go` — the `Describe("task append-metrics-session", ...)` block (near line 2589) in full, and the `Describe("task remove-metrics-session", ...)` block (near line 2874). Their `runEntityCommand`, `sha256OfFile`, `readFile`, `frontmatterOf` helpers and their fixture constants are the template to copy. ⚠️ these helpers are declared **inside each Describe's own closure**, so they are *not* visible to a sibling Describe — declare your own copies inside the new one, exactly as those blocks do. Note the comment above `baseFrontmatter`: the storage writer re-serializes the whole frontmatter in alphabetical key order on every write, so **every fixture's keys must be authored in alphabetical order** or the byte comparisons are meaningless.
- `integration/cli_test.go` — `createTempVault` (line 27) and `createTempVaultWithGoals` (line 34). `createTempVault(map[string]string{"Alpha": …})` is the fixture the spec's ACs describe: a temp vault with `Tasks/Alpha.md` and a config naming `tasks_dir: Tasks`.
- `integration/cli_test.go` — `Describe("command registration", ...)` (line 449) and its `DescribeTable`; add your `Entry` beside the other `task` entries.
- `docs/dod.md` — "New CLI commands/subcommands have an entry in the integration test command registration table".
- `docs/development-patterns.md` § Output Format and § Multi-Vault Pattern.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct; the Cobra leaf-command shape used across `pkg/cli`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, "get vaults")` and friends from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `gexec` session assertions, `Eventually(session).Should(gexec.Exit(N))`.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), so `git diff --exit-code` guards run here.
</context>

<requirements>
1. **Add the Cobra leaf.** In `pkg/cli/cli.go`, immediately after `createTaskSetCommand`, add:
   ```go
   func createTaskApproveCommand(
       ctx context.Context,
       configLoader *config.Loader,
       vaultName *string,
       outputFormat *string,
   ) *cobra.Command
   ```
   with:
   ```go
   var approvedBy string

   cmd := &cobra.Command{
       Use:   "approve <task-name>",
       Short: "Approve a task in the operator's inbox (todo -> planning)",
       Args:  cobra.ExactArgs(1),
       RunE:  func(cmd *cobra.Command, args []string) error { … },
   }
   cmd.Flags().StringVar(&approvedBy, "by", "operator", "Who is approving; recorded as approved_by")
   return cmd
   ```
   ⚠️ **The flag default must be `"operator"`, not `""`.** The default lives in the flag so that an explicit `--by ""` is a *changed* flag carrying an empty value and can be refused; if the default were `""` the empty case would be indistinguishable from "flag not given" and spec 056 AC 2's refusal could never fire. The operation refuses an empty approver on its own (prompt 1, requirement 2 step 1), so the default must be supplied here.

2. **`RunE` does exactly this**, mirroring `createTaskSetCommand`'s dispatch shape:
   ```go
   taskName := args[0]

   vaults, err := getVaults(ctx, configLoader, vaultName)
   if err != nil {
       return errors.Wrap(ctx, err, "get vaults")
   }

   currentDateTime := libtime.NewCurrentDateTime()

   dispatcher := ops.NewVaultDispatcher()
   err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
       storageConfig := storage.NewConfigFromVault(vault)
       taskStore := storage.NewTaskStorage(storageConfig)
       approveOp := ops.NewTaskApproveOperation(taskStore, currentDateTime)
       result, err := approveOp.Execute(ctx, vault.Path, taskName, vault.Name, approvedBy)
       if err != nil {
           return err
       }
       if OutputFormat(*outputFormat).IsJSON() {
           return PrintJSON(map[string]any{
               "success": true,
               "name":    result.Name,
               "phase":   string(domain.TaskPhasePlanning),
           })
       }
       fmt.Printf("✅ Approved %s: phase %s\n", result.Name, domain.TaskPhasePlanning)
       return nil
   })
   if err != nil {
       if OutputFormat(*outputFormat).IsJSON() {
           _ = PrintJSON(map[string]any{"success": false, "error": err.Error()})
       }
       return err
   }
   return nil
   ```
   Three properties of this shape are load-bearing:
   - **Plain success output must name the task and the new phase.** Spec 056 AC 9 asserts stdout contains both the task name and the literal string `planning`.
   - **JSON success output must carry the task name and the phase**, so a caller can parse `name` and `phase` out of stdout. `domain.TaskPhasePlanning` is used rather than a bare `"planning"` literal so the phase string has one definition.
   - ⚠️ **The JSON error branch prints the error object and then RETURNS the error.** This deliberately differs from `createTaskSetCommand`, whose JSON error branch returns `PrintJSON(...)` (which is normally nil, so `task set --output json` exits 0 on a refusal). Spec 056 AC 9 requires the refusal with `--output json` to exit **non-zero**, so the error must be propagated after the JSON is written. Do NOT copy `task set`'s branch, and do NOT "fix" `task set` to match — `task set`'s behaviour is frozen by spec 056's constraints.
   - **Do not add a new direct `encoding/json` use for this command** — output goes through `PrintJSON`. (`pkg/cli/cli.go` already uses `json.NewEncoder(os.Stdout)` for `task watch`'s streaming output; leave that alone and do not extend the pattern.)

3. **Register the command.** In `createTaskCommands`, beside the `createTaskSetCommand` registration:
   ```go
   cmd.AddCommand(createTaskApproveCommand(ctx, configLoader, vaultName, outputFormat))
   ```
   Add `//nolint:dupl,…` with the same linter list the neighbouring mutation commands carry if `make lint` demands it.

4. **Add the integration specs.** In `integration/cli_test.go`, add a new `Describe("task approve", func() { … })` **after** the `Describe("task remove-metrics-session", ...)` block (which ends before `Describe("vault-cli defer", ...)`, near line 3117) and **before** `Describe("vault-cli defer", ...)`. Declare `runEntityCommand`, `sha256OfFile` and `readFile` locally inside the new Describe — the sibling blocks' copies are closure-scoped and not visible. Use `createTempVault(map[string]string{"Alpha": …})` for the fixture and `AfterEach(cleanup)`.

   **Fixture authoring rule (a trap, not a style preference).** The storage writer re-serializes the entire frontmatter in alphabetical key order on every write, so a fixture authored out of order is reordered by the write and every byte comparison becomes meaningless. Author every fixture's keys alphabetically. The AC 3 fixture therefore reads, in this order: `assignee`, `custom_key`, `page_type`, `phase`, `priority`, `status`, `task_identifier`. The AC 1 fixture: `page_type`, `phase`, `priority`, `status`, `task_identifier`.

   Name each new `It` with its AC label, matching the file's existing style (`It("AC6c: …")`), so a reader can map spec to spec.

   Add a small local helper that returns the four recorded lines, so the AC 1 and AC 3 assertions read as one thing:
   ```go
   // approvedKeys returns the lines of content whose key is one of the four the
   // approve transition writes.
   approvedKeys := func(content string) []string { … }   // prefix match on status:/phase:/approved_by:/approved_at:
   ```
   Cover:
   - **AC 1 — the recorded transition.** Fixture at `status: next`, `phase: todo`, with no approval keys. `task approve Alpha` exits 0. The file's four key lines read exactly `status: in_progress`, `phase: planning`, `approved_by: operator`, and an `approved_at` whose value parses as RFC3339: strip an optional surrounding pair of double quotes, then `time.Parse(time.RFC3339, value)` must succeed (also assert the value is not the zero time). Read all four from the **file**, not from `task get` — the four-keys-in-one-read is the assertion that the transition is recorded, not merely performed.
   - **AC 2 — the approver.** (a) On a fresh `todo` fixture, `task approve Alpha --by "Manager Layer"` exits 0 and the file carries exactly one line starting with `approved_by:` reading `approved_by: Manager Layer`. (b) On a second fresh `todo` fixture, `task approve Alpha --by ""` exits non-zero, its stderr contains `approved_by`, and that file's `sha256` is unchanged.
   - **AC 3 — key preservation.** Fixture at `status: next`, `phase: todo`, carrying `assignee: someone`, `priority: 1`, `page_type: task`, `task_identifier: 22222222-2222-2222-2222-222222222222`, and an unknown key `custom_key: keep`. After `task approve Alpha` exits 0: the five unrelated lines are identical to the pre-state, and the **stripped diff is empty** — build the pre and post line slices, drop every line matching `^(status|phase|approved_by|approved_at):` from both, and assert the two slices are equal. Assert the whole-slice equality rather than counting deletions: a count proves only that nothing was removed, not that nothing stray was added. This is the AC that fails if the implementation rebuilds the frontmatter from a literal instead of composing onto the existing map, and `task_identifier` is a value other commands consume, so losing it would otherwise be silent.
   - **AC 4 — a task past the inbox.** Fixture at `phase: planning`, no approval keys. `task approve Alpha` exits non-zero, stderr contains **both** `planning` and `todo`, and the file's `sha256` taken immediately before and immediately after that one invocation is unchanged.
   - **AC 5 — a task further along.** Fixture at `phase: execution`. `task approve Alpha` exits non-zero and the file's `sha256` is unchanged. Together with AC 4 this pins the refusal to "not `todo`" rather than to one specific wrong phase.
   - **AC 6 — an existing approval record.** Three fixtures at `phase: todo`, each otherwise identical: one carrying only `approved_by: someone`, one carrying only `approved_at: 2026-01-01T00:00:00Z`, one carrying both. For **each** fixture, `task approve Alpha` exits non-zero and that fixture's own `sha256` is unchanged — three runs, three hashes, asserted per fixture. The one-key-each fixtures are load-bearing: a guard that checks only `approved_by` passes the both-keys fixture, so a single combined fixture cannot distinguish "refuses either key" from "refuses one key".
   - **AC 9 — the output paths.** (a) Plain (the default) on a fresh `todo` fixture: exits 0 and stdout contains the task name `Alpha` and the string `planning`. (b) `--output json` on a fresh `todo` fixture: exits 0 and stdout parses with `encoding/json` into an object carrying `name == "Alpha"` and `phase == "planning"`. (c) `--output json` on a `phase: planning` fixture: exits non-zero.
   - **Registration and help.** In the existing `Describe("command registration", ...)` `DescribeTable` (line 449), beside the other `task` entries, add `Entry("task approve", "task", "approve")`. Inside the new Describe, also assert `--help task approve` exits 0 and its output names the verb.
   - **Security boundary (spec 056 § Security / Abuse Cases — not an AC).** A `--by` value containing a newline (`"line1\nline2"`) is accepted (exit 0) and cannot introduce a sibling frontmatter key: the serialized file carries exactly one line starting with `approved_by:`, and parsing the frontmatter back yields `approved_by` as a single string containing both `line1` and `line2`, with no new top-level key. No newline-specific guard is added to the code — the YAML encoder's block-scalar handling is the containment, and this spec proves it holds. Keep this to one small `It`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` / `git diff --exit-code` only read, and `git checkout -- mocks/mocks.go` only restores the header the generator strips; never stage or commit anything else.
- **`task set` is frozen.** Its command body, its JSON-error branch (which returns `PrintJSON`'s nil result and exits 0), and its `--force` flag are unchanged. The approve command's JSON-error branch exits non-zero **on purpose** (AC 9); the asymmetry between the two verbs is the specified behaviour, not a bug to unify.
- **The read side is frozen.** `pkg/ops/frontmatter.go` (including `checkPhaseRegression`), `pkg/ops/frontmatter_entity.go`, and every existing integration spec are unmodified. In particular, do not modify the `Describe("task append-metrics-session", ...)` or `Describe("task remove-metrics-session", ...)` blocks.
- **No `--force` on approve.** There is no bypass flag on this verb. Re-opening a closed transition is `task set --force phase <earlier>`, which already exists.
- **No new field on `MutationResult`** and no change to `pkg/ops/task_approve.go` — the operation shipped in prompt 1 is called as-is.
- **No new direct `encoding/json` use in the approve command** — output goes through `PrintJSON`.
- **`docs/` and `CHANGELOG.md` are out of scope** — prompt 3 owns them. Do not add the `## Unreleased` bullet here.
- **README.md's `### task` list is a non-exhaustive illustration** (it already omits `task add`, `task remove` and `task remove-metrics-session`). Spec 056 DB 8 and AC 10 name `docs/task-writing.md` and `CHANGELOG.md` as the documentation deliverables, so do not touch README.md here or in prompt 3.
- Existing tests must still pass, and the integration suite's pre-existing `topic defer writes defer_date for a relative and an absolute date` spec is a **known pre-existing failure** for the ~2 h each day when the local date leads the UTC date (it computes its expectation from `time.Now().UTC().AddDate(0,0,7)` while the CLI writes the local date). Do not "fix" it opportunistically and do not let it block this prompt — `make test` passing apart from exactly that named failure is a pass.
- Fixtures must be authored in alphabetical key order (requirement 4). A non-alphabetical fixture reorders on write and makes every byte assertion in AC 3 vacuous.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'createTaskApproveCommand' pkg/cli/cli.go        # >= 2 (the factory definition and the AddCommand registration)
grep -n 'approve <task-name>' pkg/cli/cli.go             # 1 (the Use: field)
grep -c 'NewTaskApproveOperation' pkg/cli/cli.go         # 1 (the call site)
grep -c 'StringVar(&approvedBy, "by"' pkg/cli/cli.go     # 1 (the --by flag registration)
grep -c 'Approved %s' pkg/cli/cli.go                     # 1 (the plain success line, which names the phase)
grep -c 'TaskApprove\|task approve' pkg/ops/ pkg/cli/    # >= 1 (the spec's own smoke grep over the two conventional locations)
grep -c 'Describe("task approve"' integration/cli_test.go          # 1
grep -c 'Entry("task approve", "task", "approve")' integration/cli_test.go   # 1
grep -c 'sha256OfFile' integration/cli_test.go           # strictly greater than the pre-change count (21 at HEAD), i.e. the new Describe declares its own copies — the absolute floor is not the check
grep -c 'Manager Layer' integration/cli_test.go           # >= 1 (the AC 2 named-approver fixture; 0 at HEAD)
grep -c 'custom_key' integration/cli_test.go             # >= 1 (the AC 3 unknown-key preservation fixture; 0 at HEAD)
grep -c 'approved_by: someone' integration/cli_test.go   # >= 1 (the AC 6 existing-record fixture; 0 at HEAD)
```

⚠️ **Do not gate on the AC label count.** `grep -cE 'AC[0-9]+: ' integration/cli_test.go` already reads 25 at HEAD, so an AC-count row proves nothing — the three rows above are non-vacuous (each string is absent at HEAD) and are the ones to check.

FROZEN-NEIGHBOUR GUARD — these must exit 0 with empty output:
```
git diff --exit-code -- pkg/ops/frontmatter.go
git diff --exit-code -- pkg/ops/frontmatter_entity.go
git diff --exit-code -- pkg/ops/task_approve.go        # the operation shipped in prompt 1 is called as-is
git diff -U0 -- integration/cli_test.go | grep -c 'Describe("task append-metrics-session"'    # must print 0
git diff -U0 -- integration/cli_test.go | grep -c 'Describe("task remove-metrics-session"'   # must print 0
```
(The last two are line-level guards, not whole-file ones: the file legitimately gains the new Describe, so a whole-file `--exit-code` check would fail spuriously.)

TESTS:
```
go build ./...                                                          # exits 0
go test ./integration/... -ginkgo.focus="task approve" -ginkgo.v        # exits 0
go test ./pkg/ops/...                                                   # exits 0 (prompt 1's specs still pass)
make test                                                               # exits 0 apart from the named `topic defer` failure
```

⚠️ **Guard the focus run against a false pass.** `-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focus run, confirm the new spec names actually appear in the output (grep the log for `AC 3` and for the approval-refusal spec names) and treat a run whose log does not contain them as a failure.

SYNTAX:
```
gofmt -e -l pkg/cli/cli.go integration/cli_test.go    # must list NO files
```

FULL GATE — `make precommit` at the repo root must exit 0, apart from the single named pre-existing `topic defer writes defer_date for a relative and an absolute date` integration failure permitted by <constraints>; every other target (lint, vet, check-changelog, addlicense) and every other spec must pass. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make vet`, `make check-changelog`, ...), then `make precommit` once more. `make precommit` does **not** run `check-versions` (release-time only), and this prompt adds no release — the `## Unreleased` bullet is prompt 3's job.

⚠️ **`make precommit` ends in `generate`, which re-runs `rm -rf mocks` + `echo "package mocks" > mocks/mocks.go` and re-strips the copyright header from `mocks/mocks.go`.** If the header shows up as a diff, restore it as the **last** action, after the final `make precommit`:
```
git checkout -- mocks/mocks.go
git diff --exit-code -- mocks/mocks.go
```
</verification>
