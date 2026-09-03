---
status: completed
spec: [044-bug-resolve-stops-at-first-vault-on-found-false]
summary: 'Fixed vault-cli resolve to search every configured vault: a per-vault found:false miss no longer terminates the search (ErrNotFound continuation signal), the first vault that resolves the name wins, and exhausted searches are consumed into a single found:false JSON with exit 0; added a newResolveOp injection seam, unit tests, two-vault integration cases, and a CHANGELOG fix entry.'
execution_id: vault-cli-exec-207-spec-044-resolve-multi-vault-dispatch
dark-factory-version: dev
created: "2026-09-03T19:38:41Z"
queued: "2026-09-03T19:43:35Z"
started: "2026-09-03T19:45:02Z"
completed: "2026-09-03T19:53:12Z"
---

# Fix resolve stopping at first vault on found:false

<summary>
- `vault-cli resolve <name> --output json` run WITHOUT `--vault` currently reports `found:false` for names that exist in a configured vault — nondeterministically, because the search order is random and a per-vault miss wrongly terminates the search.
- After this change, a vault that does not contain the name no longer ends the search: `resolve` keeps trying every configured vault until the name is found.
- The first vault that resolves the name to `found:true` wins; its result is emitted as exactly one JSON document and later vaults are not searched.
- `found:false` is emitted only after every configured vault has been searched; the dispatcher's wrapped "not found in any vault" error is consumed internally and converted to the single not-found JSON (exit 0, no error text on stdout or stderr).
- `--vault <name>` and single-vault configurations behave exactly as before; plain mode stays silent in every outcome.
- The resolve command gains a test-injection seam for its per-vault operation constructor, using the same pattern as the existing task-backfill command, so the per-vault dispatch decision is unit-testable.
- New unit tests prove a not-found vault does not terminate the search (both vaults are tried) while a found vault terminates it (only the first is tried), and that the exhausted-search error is converted to the single `found:false` JSON.
- New two-vault integration cases cover the task, goal, and exhausted paths end-to-end through the real binary.
- A `fix:` entry is added to the CHANGELOG under `## Unreleased`.
</summary>

<objective>
Make `resolve` search all configured vaults — a per-vault miss no longer terminates the search, the first vault that has the name wins, and `found:false` is emitted only after every vault has been searched — so consumers (entity-type auto-detection, orchestrator scripts) stop misrouting existing entities into create flows. The JSON schema, `--vault` semantics, single-vault behavior, and plain-mode silence are all unchanged.
</objective>

<context>
Read CLAUDE.md for project conventions (Ginkgo/Gomega, bborbe/errors, libtime, DoD).

Read these files before changing anything:
- `pkg/cli/cli.go` — `createResolveCommand` (the defect site, ~line 1181) and its single call site in `rootCmd.AddCommand(...)` (~line 117); `getVaults` (~line 29) returns all vaults via `GetAllVaults` when `--vault` is unset; `createTaskBackfillIdentifiersCommand` is the injection-pattern exemplar (a `newBackfillOp func(cfg *storage.Config) ops.EnsureAllTaskIdentifiersOperation` parameter).
- `pkg/ops/vault_dispatcher.go` — `FirstSuccess` continuation contract: a `nil` return is terminal success; ONLY a `storage.ErrNotFound`-class error lets the loop continue; any other error propagates immediately; the last `ErrNotFound` is wrapped as `not found in any vault (tried: ...)`. Do not modify this file.
- `pkg/ops/resolve.go` — `ResolveOperation.Execute` returns a not-found `domain.ResolveResult` with nil error on a miss. Do not modify this file.
- `pkg/domain/resolve_result.go` — the JSON contract `{"type","name","found"}` with NO `omitempty` (empty `type` and `false` must serialize).
- `pkg/storage/base.go` — the `errors.Wrapf(ctx, storage.ErrNotFound, ...)` wrapping idiom for not-found errors.
- `pkg/cli/task_backfill_identifiers_test.go` and `pkg/cli/export_test.go` — the ForTest-export + `mocks.Loader` + fake-operation pattern to mirror in the new `pkg/cli/resolve_test.go`; the `os.Pipe` stdout-capture pattern is at `task_backfill_identifiers_test.go` (~lines 127-137).
- `integration/cli_test.go` — `createTempVaultWithGoals` helper (~lines 28-78) and the existing `Describe("vault-cli resolve", ...)` block (~lines 1393-1522) with its four single-vault cases (do not modify them).
- `mocks/resolve-operation.go` and `mocks/config-loader.go` — the counterfeiter fake APIs (`ExecuteReturns`, `ExecuteCallCount`, `GetAllVaultsReturns`) used by the unit test.

Coding plugin docs (read before writing tests): `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`, `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`, `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`.
</context>

<requirements>
1. **Fix the dispatch closure and add error consumption in `pkg/cli/cli.go` (`createResolveCommand`).**
   - Change the signature to accept a per-vault resolve-operation constructor as a new 5th parameter, mirroring `createTaskBackfillIdentifiersCommand`'s `newBackfillOp` parameter:
     `func createResolveCommand(ctx context.Context, configLoader *config.Loader, vaultName *string, outputFormat *string, newResolveOp func(cfg *storage.Config) ops.ResolveOperation) *cobra.Command`
   - Update the single call site (`rootCmd.AddCommand(createResolveCommand(...))`, ~line 117) to pass the real constructor:
     `func(cfg *storage.Config) ops.ResolveOperation { return ops.NewResolveOperation(storage.NewTaskStorage(cfg), storage.NewGoalStorage(cfg)) }`
   - Inside the `FirstSuccess` closure, replace the inline `storage.NewConfigFromVault` / `storage.NewTaskStorage` / `storage.NewGoalStorage` / `ops.NewResolveOperation` construction with `resolveOp := newResolveOp(storage.NewConfigFromVault(vault))`.
   - After `result, err := resolveOp.Execute(ctx, vault.Path, name)`, the closure's return contract becomes:
     - `err != nil` → return `err` unchanged (a hard vault error must propagate, not be masked as not-found).
     - `!result.Found` → return a `storage.ErrNotFound`-class error so `FirstSuccess` continues to the next vault. Follow the `pkg/storage/base.go` idiom: `errors.Wrapf(ctx, storage.ErrNotFound, "not found in vault %s", vault.Name)`.
     - `result.Found` → in JSON mode `return PrintJSON(result)`; in plain mode return `nil` (silent). This is what makes "first found vault wins" work — the found result is printed exactly once and `FirstSuccess` terminates.
   - Add `"github.com/bborbe/vault-cli/pkg/domain"` to the cli.go import block.
   - Capture the `FirstSuccess` return value instead of returning it directly. When `err != nil` AND `errors.Is(err, storage.ErrNotFound)` (bborbe/errors `Is` — same call as `pkg/ops/vault_dispatcher.go:65`), the search is exhausted (every vault missed, or a single-vault miss): in JSON mode `return PrintJSON(domain.ResolveResult{Type: "", Name: name, Found: false})`, in plain mode `return nil`. Never surface the `not found in any vault (tried: ...)` text — it stays inside the error object, which is consumed here. Any other error (hard vault error, context cancellation, "no vaults configured") is returned unchanged → non-zero exit.
   - Net effect: a per-vault miss prints nothing; exactly one JSON document is emitted per invocation; exit 0 for both found and not-found.

2. **Expose the command for unit testing in `pkg/cli/export_test.go`.**
   - Add `CreateResolveCommandForTest` mirroring `CreateTaskBackfillIdentifiersCommandForTest`: same signature as the new `createResolveCommand` (including the `newResolveOp` parameter), returning the command.

3. **Add unit tests in a new file `pkg/cli/resolve_test.go`** (package `cli_test`, Ginkgo/Gomega — mirror `pkg/cli/task_backfill_identifiers_test.go`).
   - Setup: `fakeLoader := &mocks.Loader{}` wrapped as `var loader config.Loader = fakeLoader; configLoader = &loader`; `fakeOp := &mocks.ResolveOperation{}`; two vaults via `fakeLoader.GetAllVaultsReturns([]*config.Vault{{Name: "alpha", Path: "/tmp/alpha"}, {Name: "beta", Path: "/tmp/beta"}}, nil)`; `vaultName = ""` (exercises the `GetAllVaults` path); run via `cli.CreateResolveCommandForTest(ctx, configLoader, &vaultName, &outputFormat, func(cfg *storage.Config) ops.ResolveOperation { return fakeOp }).RunE(cmd, []string{"my-entity"})`.
   - **A vault resolving `found:false` does not terminate the search**: `fakeOp.ExecuteReturns(domain.ResolveResult{Type: "", Name: "my-entity", Found: false}, nil)`, outputFormat `"plain"` → RunE returns nil and `Expect(fakeOp.ExecuteCallCount()).To(Equal(2))` (both vaults tried).
   - **A vault resolving `found:true` terminates the search**: `fakeOp.ExecuteReturns(domain.ResolveResult{Type: "task", Name: "my-entity", Found: true}, nil)`, outputFormat `"plain"` → RunE returns nil and `Expect(fakeOp.ExecuteCallCount()).To(Equal(1))` (second vault never tried).
   - **Exhausted search converts to the single `found:false` JSON**: `fakeOp.ExecuteReturns(domain.ResolveResult{Type: "", Name: "my-entity", Found: false}, nil)`, outputFormat `"json"` → capture `os.Stdout` with an `os.Pipe` (pattern at `task_backfill_identifiers_test.go` ~lines 127-137) → RunE returns nil, the captured output contains `"found": false` and `"type": ""`, and `"found"` appears exactly once (exactly one JSON document).
   - **A hard vault error propagates**: `fakeOp.ExecuteReturns(domain.ResolveResult{}, errors.Errorf(ctx, "list tasks: no such directory"))`, outputFormat `"json"` → RunE returns a non-nil error and the captured stdout is empty (no partial JSON emitted). This pins the error-consumption branch: only `storage.ErrNotFound`-class errors are converted to `found:false`; context cancellation and "no vaults configured" errors take the same propagate path.

4. **Add two-vault integration coverage in `integration/cli_test.go`.**
   - Add a helper `createTwoTempVaults(tasksA, goalsA, tasksB, goalsB map[string]string) (vaultPathA, vaultPathB, configPath string, cleanup func())` modeled on `createTempVaultWithGoals` (~lines 28-78): two `os.MkdirTemp` vault dirs with `Tasks`/`Goals` subdirectories, and a config file with `default_vault: alpha` and two vaults `alpha` and `beta`, each with `name`, `path`, `tasks_dir: Tasks`, `goals_dir: Goals`.
   - Add a new `Context("multi-vault fall-through", ...)` inside the existing `Describe("vault-cli resolve", ...)` block, after the `"with task and goal present"` Context (it must be a distinct two-vault Context, not a duplicate single-vault case). Each case runs WITHOUT `--vault` (only `--config <configPath> resolve <name> --output json`), so `getVaults` returns both vaults via `GetAllVaults`. Note the iteration order is random (unsorted map) — each case must pass for both orders; with the name present in exactly one vault, both orders resolve `found:true` under the fix.
   - **Task case (AC 1):** task `my-task` only in vault alpha, beta empty → exit 0; `json.Unmarshal(session.Out.Contents(), &result)` succeeds; `HaveKeyWithValue("type", "task")`, `HaveKeyWithValue("name", "my-task")`, `HaveKeyWithValue("found", true)`; exactly one JSON document (see assertion below).
   - **Goal case (AC 2):** goal `my-goal` only in vault beta, alpha empty → exit 0; `HaveKeyWithValue("type", "goal")`, `HaveKeyWithValue("name", "my-goal")`, `HaveKeyWithValue("found", true)`; exactly one JSON document.
   - **Exhausted case (AC 3):** name in neither vault → exit 0; `HaveKeyWithValue("type", "")`, `HaveKeyWithValue("name", "<name>")`, `HaveKeyWithValue("found", false)`; raw output contains `"type": ""` and `"found": false` (no omitempty); exactly one JSON document; AND `Expect(string(session.Out.Contents())).NotTo(ContainSubstring("not found in any vault"))` AND `Expect(string(session.Err.Contents())).NotTo(ContainSubstring("not found in any vault"))`.
   - Assert "exactly one JSON document" as `Expect(strings.Count(string(session.Out.Contents()), "\"found\"")).To(Equal(1))` — add `"strings"` to the integration test imports.
   - Do NOT modify the four existing single-vault cases (`returns task match as JSON`, `returns goal match as JSON`, `returns not found for unknown name`, `task-first priority when name matches both`) — they must remain present and unchanged.

5. **Add a CHANGELOG entry.** In `CHANGELOG.md`, add a `fix:` bullet under the existing `## Unreleased` section (above the current `feat:` flag entry) describing that `vault-cli resolve <name> --output json` without `--vault` now searches every configured vault — a not-found vault no longer stops the search, and `found:false` is emitted only after all vaults have been searched.

6. **Self-check before finishing:** re-run `<verification>` and confirm it passes; walk each of the spec's Acceptance Criteria 1-5 against the change (multi-vault task fall-through, multi-vault goal fall-through, found:false only after exhaustion with no `not found in any vault` text, the four single-vault regression cases intact, and the resolve unit test call counts).
</requirements>

<constraints>
- JSON output schema of `resolve` unchanged — exactly `{"type","name","found"}`; `type` and `found` serialize even when empty/false (no `omitempty`).
- `ResolveOperation` contract unchanged — a miss is a not-found result with nil error from `Execute`; the continuation signal is produced by the resolve command's dispatch layer, not by changing the operation.
- `VaultDispatcher.FirstSuccess` semantics unchanged — only a `storage.ErrNotFound`-class error continues the loop; any other error propagates immediately. Do not edit `pkg/ops/vault_dispatcher.go` or `pkg/ops/resolve.go`.
- `--vault` flag semantics unchanged; single-vault dispatch path unchanged.
- Do NOT introduce deterministic vault ordering — explicit non-goal; the unsorted `GetAllVaults` iteration stays as-is.
- Error style: `github.com/bborbe/errors` with context wrapping; no `fmt.Errorf`.
- No `os.Stdout` / `fmt.Print*` in `pkg/ops/` — output stays in the CLI layer.
- Exactly one JSON document per invocation on stdout; exit 0 for both found and not-found outcomes; non-zero only for genuine errors (no vaults configured, context cancellation, hard vault error).
- Do NOT commit — dark-factory handles git (workflow: direct; git work happens host-side).
- Existing tests must still pass.
- No README change — this fixes a bug in a shipped command and restores its documented contract; README does not describe the buggy behavior.
</constraints>

<verification>
Run `make precommit` — must exit 0.
Run `make test` — must exit 0 (includes the new two-vault resolve integration cases and the new resolve unit test).

Grep side-effect checks:
- `grep -c 'multi-vault' integration/cli_test.go` — must be >= 1
- `grep -c 'HaveKeyWithValue("type", "task")' integration/cli_test.go` — must be >= 3
- `grep -c 'HaveKeyWithValue("type", "goal")' integration/cli_test.go` — must be >= 2
- `grep -c 'HaveKeyWithValue("found", false)' integration/cli_test.go` — must be >= 2
- `grep -c 'NotTo(ContainSubstring("not found in any vault"))' integration/cli_test.go` — must be >= 2
- `grep -c 'returns task match as JSON' integration/cli_test.go` — must be == 1
- `grep -c 'returns goal match as JSON' integration/cli_test.go` — must be == 1
- `grep -c 'task-first priority' integration/cli_test.go` — must be == 1
- `grep -c 'Equal(2)' pkg/cli/resolve_test.go` — must be >= 1
- `grep -c 'Equal(1)' pkg/cli/resolve_test.go` — must be >= 1
</verification>

<!--
Open questions for the reviewer (not instructions for the agent):
1. The fix requires an injection seam for the per-vault resolve-operation constructor so the dispatch decision is unit-testable. The chosen approach mirrors the existing `createTaskBackfillIdentifiersCommand` / `CreateTaskBackfillIdentifiersCommandForTest` pattern (constructor param + ForTest export). Alternative considered: test through real storage — rejected because AC 5 explicitly demands fakes with call counts.
2. The per-vault miss error message ("not found in vault <name>") is new diagnostic text. It never reaches stdout or stderr — it exists only inside the error chain that RunE consumes on exhaustion. Wording chosen to match the `errors.Wrapf(ctx, storage.ErrNotFound, ...)` idiom in pkg/storage/base.go.
-->
