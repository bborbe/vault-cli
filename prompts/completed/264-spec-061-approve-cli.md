---
status: completed
spec: [061-goal-topic-approve]
summary: Added `vault-cli goal approve` and `vault-cli topic approve` leaf commands with --by/JSON output and binary-level end-to-end coverage, plus changelog and command-registration updates
execution_id: vault-cli-goal-topic-approve-exec-264-spec-061-approve-cli
dark-factory-version: v0.198.7
created: "2026-10-20T10:03:18Z"
queued: "2026-10-20T10:16:58Z"
started: "2026-10-20T10:27:13Z"
completed: "2026-10-20T10:31:14Z"
---

# Wire `vault-cli goal approve` and `vault-cli topic approve` and prove them end to end (spec 061, prompt 4 of 5)

<summary>
- Operators get `vault-cli goal approve <goal>` and `vault-cli topic approve <topic>`.
- The approver defaults to `operator` and can be named with `--by`; an explicitly empty `--by` is refused.
- Plain output names the row and the phase it landed in; `--output json` emits the name and the new phase.
- A refusal exits non-zero in both output modes, including JSON.
- The new verbs are registered in the command-registration table the project's Definition of Done requires.
- Binary-level tests prove the recorded transition, key preservation, and every refusal against a real vault and a real binary.
</summary>

<objective>
Expose the two approval operations from prompts 2 and 3 as Cobra leaf commands, so the operator can perform the `todo → planning` transition from the command line, and prove the whole transition end to end. This closes acceptance criteria 1, 2 and 4 of spec 061 at the binary level.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully, in this order:
- `pkg/cli/cli.go` — `createTaskApproveCommand` (search for it). This is the exemplar: `getVaults` → `ops.NewVaultDispatcher().FirstSuccess` → build the operation per vault → plain/JSON output → the JSON error branch that prints the error object **and then returns the error**, so a refusal exits non-zero. Also read `createGoalCommands`, `createTopicCommands`, `createGoalCompleteCommand`, and `createTopicCompleteCommand` for where the new factories and registrations belong.
- `pkg/cli/output.go` — `OutputFormat.IsJSON()` and `PrintJSON`.
- `pkg/ops/vault_dispatcher.go` — `FirstSuccess` semantics: a single vault calls the callback directly and propagates its error; with several vaults only a not-found-class error lets the loop continue. A refusal stops the loop, which is the intent.
- `pkg/ops/goal_approve.go` and `pkg/ops/topic_approve.go` — the operations shipped by prompts 2 and 3. Read their `Execute` signatures before writing the call sites.
- `pkg/domain/goal_phase.go` and `pkg/domain/topic_phase.go` — `GoalPhasePlanning`, `TopicPhasePlanning`, so the output does not use bare `"planning"` literals.
- `integration/cli_test.go` — the existing `Describe("task approve", …)` block in full: its locally declared `runEntityCommand` / `sha256OfFile` / `readFile` / `valueOf` / `approvedKeys` helpers and its fixture constants. Also read `createTempVaultWithGoals` and `createTempVaultWithTopics`, and the `Describe("command registration", …)` table.
- `docs/dod.md` — "New CLI commands/subcommands have an entry in the integration test command registration table".

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — Cobra leaf-command shape.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, "get vaults")` from `github.com/bborbe/errors`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `gexec`, `Eventually(session).Should(gexec.Exit(N))`.
</context>

<requirements>
1. **Add `createGoalApproveCommand`** in `pkg/cli/cli.go`, immediately after `createGoalCompleteCommand`:

   ```go
   func createGoalApproveCommand(
       ctx context.Context,
       configLoader *config.Loader,
       vaultName *string,
       outputFormat *string,
   ) *cobra.Command
   ```

   - `Use: "approve <goal-name>"`, `Args: cobra.ExactArgs(1)`.
   - One flag: `cmd.Flags().StringVar(&approvedBy, "by", "operator", "Who is approving; recorded as approved_by")`. The default **must** be `"operator"` in the flag, so an explicit `--by ""` is a changed flag carrying an empty value and reaches the operation's refusal; a `""` default would make the empty case indistinguishable from "flag not given".
   - `RunE` mirrors `createTaskApproveCommand` minus the current-user lookup and the `--assignee` flag (spec Non-goal: no owner assignment):
     ```go
     goalName := args[0]
     vaults, err := getVaults(ctx, configLoader, vaultName)
     if err != nil {
         return errors.Wrap(ctx, err, "get vaults")
     }
     currentDateTime := libtime.NewCurrentDateTime()
     dispatcher := ops.NewVaultDispatcher()
     err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
         storageConfig := storage.NewConfigFromVault(vault)
         goalStore := storage.NewGoalStorage(storageConfig)
         approveOp := ops.NewGoalApproveOperation(goalStore, currentDateTime)
         result, err := approveOp.Execute(ctx, vault.Path, goalName, vault.Name, approvedBy)
         if err != nil {
             return err
         }
         if OutputFormat(*outputFormat).IsJSON() {
             return PrintJSON(map[string]any{
                 "success": true,
                 "name":    result.Name,
                 "phase":   string(domain.GoalPhasePlanning),
             })
         }
         fmt.Printf("✅ Goal approved: %s (phase %s)\n", result.Name, domain.GoalPhasePlanning)
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
   - The plain success line must contain the goal name — acceptance criterion 1 asserts it on stdout.

2. **Add `createTopicApproveCommand`** the same way, immediately after `createTopicCompleteCommand`: `Use: "approve <topic-name>"`, the same `--by` flag and default, `storage.NewTopicStorage(storageConfig)`, `ops.NewTopicApproveOperation`, `domain.TopicPhasePlanning`, and the plain line `✅ Topic approved: %s (phase %s)\n`. Add `//nolint:dupl,…` with the same linter list the neighbouring parallel commands carry if `make lint` demands it.

3. **Register both commands** beside their siblings:
   ```go
   cmd.AddCommand(createGoalApproveCommand(ctx, configLoader, vaultName, outputFormat))   // in createGoalCommands
   cmd.AddCommand(createTopicApproveCommand(ctx, configLoader, vaultName, outputFormat))  // in createTopicCommands
   ```

4. **Add the binary-level specs** in `integration/cli_test.go`, immediately after the existing `Describe("task approve", …)` block: one `Describe("goal approve", …)` and one `Describe("topic approve", …)`. Declare their helpers locally (the sibling blocks' copies are closure-scoped). Author every fixture's keys alphabetically — the storage writer re-serializes the whole frontmatter in alphabetical key order, so a non-alphabetical fixture makes every byte assertion vacuous.

   Fixtures: goals via `createTempVaultWithGoals(nil, map[string]string{"Alpha": …})`; topics via `createTempVaultWithTopics("Topics")` plus `os.MkdirAll(filepath.Join(vaultPath, "Topics"), 0755)` and a written page. Name each `It` with its AC label, matching the file's style.

   ⚠️ **AC-rendering note — read before writing the `approved_at` assertion.** Acceptance criterion 1 says `vault-cli goal get <goal> approved_at` parses as RFC3339. It does not, and must not be made to: the operation writes a bare `time.Time` (spec Desired Behavior 3: "formatted the same way `task approve` writes it"), yaml.v3 renders that as an unquoted `approved_at: 2026-10-20T10:03:18Z`, and reading it back yields a `time.Time` whose `FrontmatterMap.GetString` rendering is Go's `%v` form (`2026-10-20 10:03:18 +0000 UTC`). Writing a formatted string instead would quote the line and diverge from the task record. So: run the RFC3339 parse **on the raw file line** (strip an optional surrounding pair of double quotes), exactly as the existing `task approve` spec does, and separately assert `goal get Alpha approved_at` exits 0 with a non-empty value. Do not change the write shape to satisfy the AC's literal rendering.

   Cover, for **both** entity types:
   - **AC 1 — the recorded transition.** Fixture at `status: next`, `phase: todo`, no approval keys. `goal approve Alpha` exits 0. Read the file: `phase: planning`, `approved_by: operator`, and an `approved_at` line whose value parses as RFC3339. Capture `before := time.Now().UTC()` immediately before the invocation and assert `parsed.Sub(before) >= 0 && parsed.Sub(before) <= 60*time.Second`. Also assert `goal get Alpha phase` prints `planning`, `goal get Alpha approved_by` prints `operator`, and `goal get Alpha approved_at` prints a non-empty value. Assert stdout contains `Alpha`.
   - **AC 1 — `--by`.** On a fresh `todo` fixture, `goal approve Alpha --by alice` exits 0 and the file's `approved_by` line reads `alice`.
   - **AC 2 — empty approver.** On a fresh `todo` fixture, `goal approve Alpha --by ""` exits non-zero, stderr contains `approved_by`, and the file's sha256 is unchanged.
   - **AC 4 — refusals, table-driven.** For each row, a fresh fixture, one invocation, non-zero exit, stderr contains the row name, and the sha256 taken immediately before and after that invocation is unchanged:
     - `phase: planning`, `phase: execution`, `phase: done`, and no `phase` key at all — stderr names both the phase found and the phase required (`todo`);
     - `phase: todo` carrying only `approved_by: someone`, only `approved_at: 2026-01-01T00:00:00Z`, and both — stderr names the present key;
     - an unknown row name (`Missing`) — stderr contains `Missing`.
   - **AC 4 — key preservation.** A `todo` fixture carrying `custom_key: keep`, `page_type: goal`, `priority: 1` (topics: `custom_key: keep`, `page_type: topic`). Keep every value in this fixture a scalar: the writer re-serializes the raw map with `yaml.Marshal`, which renders a flow list (`tags: [a]`) as a block list indented four spaces, so a list key would break the byte comparison for a reason unrelated to approval. After a successful approve, drop every line matching `^(phase|approved_by|approved_at):` from the pre and post content and assert the two slices are equal — whole-slice equality, not a count. Assert `custom_key` still reads `keep`.
   - **JSON output.** `--output json` on a fresh `todo` fixture exits 0 and stdout parses into an object with `name == "Alpha"` and `phase == "planning"`; `--output json` on a `phase: planning` fixture exits non-zero.

5. **Register both verbs** in the `Describe("command registration", …)` table, beside the other goal/topic entries:
   ```go
   Entry("goal approve", "goal", "approve"),
   Entry("topic approve", "topic", "approve"),
   ```

6. **Self-check before finishing.** Re-run the `<verification>` block and confirm every row; walk acceptance criteria 1, 2 and 4 of spec 061 against the change.
</requirements>

<constraints>
- `task approve` is frozen: its command body, its flags, and its JSON-error branch are unchanged. Do not add `--assignee`, `--force`, or a current-user lookup to the new verbs (spec Non-goal: no owner assignment at approval time).
- The operations shipped by prompts 2 and 3 are called as-is. Do not add fields to `MutationResult` and do not change `pkg/ops/goal_approve.go`, `pkg/ops/topic_approve.go`, or `pkg/ops/approval_record.go`.
- No new direct `encoding/json` use for these commands — output goes through `PrintJSON`.
- `docs/` and `CHANGELOG.md` are out of scope — prompt 5 owns them.
- `commands/` docs are out of scope here; a `commands/` change needs a plugin version bump, which the releaser handles in prompt 5. Do not hand-bump the manifests.
- Fixtures must be authored in alphabetical key order (requirement 4).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass. The integration suite's `topic defer writes defer_date for a relative and an absolute date` spec is a known pre-existing failure for the hours when the local date leads the UTC date; do not "fix" it and do not let it block this prompt.
</constraints>

<verification>
EVIDENCE GREPS — run each and confirm the count. Absence assertions use `! grep -q` because `grep -c` exits 1 when the count is 0.

```
grep -c 'createGoalApproveCommand' pkg/cli/cli.go                       # >= 2 (factory definition + registration)
grep -c 'createTopicApproveCommand' pkg/cli/cli.go                      # >= 2
grep -c 'NewGoalApproveOperation' pkg/cli/cli.go                        # 1
grep -c 'NewTopicApproveOperation' pkg/cli/cli.go                       # 1
grep -c 'approve <goal-name>' pkg/cli/cli.go                            # 1
grep -c 'approve <topic-name>' pkg/cli/cli.go                           # 1
grep -c 'StringVar(&approvedBy, "by"' pkg/cli/cli.go                    # 3 (task approve's existing one + one per new verb)
grep -c 'Goal approved' pkg/cli/cli.go                                  # 1
grep -c 'Topic approved' pkg/cli/cli.go                                 # 1
grep -c 'Describe("goal approve"' integration/cli_test.go               # 1
grep -c 'Describe("topic approve"' integration/cli_test.go              # 1
grep -c 'Entry("goal approve", "goal", "approve")' integration/cli_test.go    # 1
grep -c 'Entry("topic approve", "topic", "approve")' integration/cli_test.go  # 1
```

⚠️ Do not gate on `grep -cE 'AC[0-9]+: ' integration/cli_test.go` — the file already contains many AC labels, so that count proves nothing. The rows above are the non-vacuous ones.

FROZEN-NEIGHBOUR GUARD — the container has no usable `.git` (the daemon runs with `hideGit=true`), so this cannot be checked with git. The `<constraints>` forbid touching these files, and `make precommit` catches a broken neighbour. If a hard guard is wanted, record the `sha256sum` baseline of the three ops files at authoring time and compare; `pkg/cli/cli.go` is the fourth frozen file and is covered by the following `grep`:

```
sha256sum pkg/ops/goal_approve.go pkg/ops/topic_approve.go pkg/ops/approval_record.go > /tmp/frozen.sha256   # BEFORE editing
sha256sum -c /tmp/frozen.sha256                                                                              # after — every row must print OK
grep -c 'approve <task-name>' pkg/cli/cli.go    # 1 — task approve's Use line is untouched
```

TESTS:

```
go build ./...
go test ./integration/... -ginkgo.focus="goal approve|topic approve" -ginkgo.v
make precommit
```

⚠️ `-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focus run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

SYNTAX:

```
gofmt -e -l pkg/cli/cli.go integration/cli_test.go
```

`make precommit` must exit 0 apart from the named pre-existing `topic defer` integration failure. `make precommit` runs `ensure format generate test check addlicense`. If the known `topic defer` failure stops the chain at `test`, `addlicense` never runs — run `make addlicense` as the last action in that case. Do not use git to restore anything — `.git` is masked in the container.
</verification>
