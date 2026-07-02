---
status: draft
spec: [021-resolve-name-subcommand]
created: "2026-07-02T10:00:00Z"
branch: dark-factory/resolve-name-subcommand
---

<summary>
- Wires a new top-level `vault-cli resolve <name>` command that prints typed JSON identifying whether a name is a task, a goal, or neither
- Multi-vault aware: probes each configured vault in order and returns the first match; honors the existing `--vault` flag for single-vault scoping
- JSON is the only useful mode — in plain (default) mode the command prints nothing and exits 0 (it is a machine contract for slash commands)
- A total miss across all vaults is a normal result (`found:false`, exit 0), never an error
- Adds an integration test that runs the built binary against a temp vault and asserts the JSON contract for the task-match, goal-match, and not-found cases
- After this prompt, `vault-cli resolve "Some Name" --output json` works end-to-end and `make precommit` passes
</summary>

<objective>
Add a top-level `vault-cli resolve <name>` Cobra command in `pkg/cli/cli.go` (via `createResolveCommand`) that resolves the name to a task/goal/neither across vaults using the `ResolveOperation`, prints the `domain.ResolveResult` as JSON in `--output json` mode, prints nothing in plain mode, and always exits 0 on a clean probe (including not-found). Add an integration test in `integration/cli_test.go`.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-http-handler-refactoring-guide.md` is NOT relevant. Read instead:
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping with `ctx`, never `fmt.Errorf`.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, integration test conventions.

Read these files before implementing:
- `pkg/cli/cli.go`:
  - `createSearchCommand` (function starts near line 1764) — the closest template: a TOP-LEVEL command (not under a noun), `Use: "search <query>"`, `Args: cobra.ExactArgs(1)`, resolves `getVaults`, iterates vaults, honors `*outputFormat` with `PrintJSON`. Mirror its shape.
  - `createWorkOnCommand` (line 306) — shows the `getVaults` + `ops.NewVaultDispatcher()` + per-vault store creation pattern with `storage.NewConfigFromVault(vault)`, `storage.NewTaskStorage(...)`. Use this for building the two stores per vault.
  - `getVaults` (line 31) — `getVaults(ctx, configLoader, vaultName)` returns `[]*config.Vault`.
  - `NewRootCommand` (line 77) — where top-level commands are registered via `rootCmd.AddCommand(...)`. Line 111 registers `createSearchCommand`. Add the resolve command registration right after it, following the same argument list `(ctx, &configLoader, &vaultName, &outputFormat)`.
  - `OutputFormatJSON` / `OutputFormatPlain` constants (defined in `pkg/cli/output.go` lines 13-14) and `PrintJSON` helper (`pkg/cli/output.go` line 18).
- `pkg/storage/storage.go`:
  - `NewConfigFromVault(vault)` (line 26), `NewTaskStorage(cfg)` (line 161), `NewGoalStorage(cfg)` (line 169).
- `pkg/ops/resolve.go` — `NewResolveOperation(taskStorage, goalStorage)` and `Execute(ctx, vaultPath, name) (domain.ResolveResult, error)` from prompt 02. If this file does not exist yet, STOP and report `Status: failed` with message `"ResolveOperation not yet deployed (prompt 02)"`.
- `integration/cli_test.go`:
  - `createTempVault(tasks map[string]string)` (lines 20-57) — creates a temp vault with a `Tasks/` dir and a config (`tasks_dir: Tasks`, default_vault `test`). NOTE: it does NOT create a `Goals/` dir. For the goal-match integration test you must additionally create a `Goals/` dir under the returned `vaultPath` and write a goal file there (the vault's `GetGoalsDir()` defaults to `"Goals"`, so no config change is needed — see `pkg/config/config.go:55`).
  - `command registration` DescribeTable (lines 70-146) — add an `Entry("resolve", "resolve")` so `resolve --help` exits 0.
  - The JSON-schema Describe block (lines 728-803) — template for running `binPath ... --output json` and asserting parsed JSON with `json.Unmarshal` + `HaveKeyWithValue`.
  - `binPath` is the built binary path (set up in the suite bootstrap file in `integration/`).

Multi-vault / not-found decision (resolved for this prompt): Do NOT use `ops.NewVaultDispatcher().FirstSuccess` here. `FirstSuccess` returns an error ("not found in any vault") when no vault yields success, but resolve's not-found is NOT an error — it must print `{"type":"","name":...,"found":false}` and exit 0. Instead iterate vaults manually: for each vault, call `resolveOp.Execute`; on the FIRST vault whose result has `Found == true`, print that result (JSON mode) and return nil. If no vault matches, print the not-found result (JSON mode) for the input name and return nil. This is the "first success wins, miss is exit-0" semantics the spec requires (AC5 + AC3).
</context>

<requirements>
1. Add `createResolveCommand` to `pkg/cli/cli.go`, modeled on `createSearchCommand`, with the standard function signature `(ctx context.Context, configLoader *config.Loader, vaultName *string, outputFormat *string) *cobra.Command`:
   ```go
   func createResolveCommand(
       ctx context.Context,
       configLoader *config.Loader,
       vaultName *string,
       outputFormat *string,
   ) *cobra.Command {
       return &cobra.Command{
           Use:   "resolve <name>",
           Short: "Resolve a name to a task or goal (JSON contract for slash commands)",
           Args:  cobra.ExactArgs(1),
           RunE: func(cmd *cobra.Command, args []string) error {
               name := args[0]
               vaults, err := getVaults(ctx, configLoader, vaultName)
               if err != nil {
                   return errors.Wrap(ctx, err, "get vaults")
               }
               for _, vault := range vaults {
                   storageConfig := storage.NewConfigFromVault(vault)
                   taskStore := storage.NewTaskStorage(storageConfig)
                   goalStore := storage.NewGoalStorage(storageConfig)
                   resolveOp := ops.NewResolveOperation(taskStore, goalStore)
                   result, err := resolveOp.Execute(ctx, vault.Path, name)
                   if err != nil {
                       return errors.Wrap(ctx, err, "resolve name")
                   }
                   if result.Found {
                       if *outputFormat == OutputFormatJSON {
                           return PrintJSON(result)
                       }
                       return nil
                   }
               }
               // No vault matched: not-found is a valid result, exit 0.
               if *outputFormat == OutputFormatJSON {
                   return PrintJSON(domain.ResolveResult{Type: "", Name: name, Found: false})
               }
               return nil
           },
       }
   }
   ```
   - Confirm `"github.com/bborbe/vault-cli/pkg/domain"` is imported in `cli.go` (grep it; add to the import block if missing).
   - Plain mode prints NOTHING and returns nil (exit 0) — both on match and on miss (spec: plain-text is a quiet no-op).

2. Register the command in `NewRootCommand` immediately after the `createSearchCommand` registration (line 111):
   ```go
   rootCmd.AddCommand(createResolveCommand(ctx, &configLoader, &vaultName, &outputFormat))
   ```

3. Add an `Entry("resolve", "resolve")` to the `command registration` DescribeTable in `integration/cli_test.go` (in the "Root-level commands" group near `Entry("search", "search")`), so `vault-cli resolve --help` exits 0.

4. Add a new `Describe("vault-cli resolve", ...)` block in `integration/cli_test.go` with three integration cases that run the built `binPath` binary against a temp vault and assert the JSON contract (use `json.Unmarshal` into `map[string]any` and `HaveKeyWithValue`, mirroring the JSON-schema block at lines 728-803):
   - **Task match (AC1):** create a temp vault via `createTempVault(map[string]string{"Existing Task Name": "---\nstatus: todo\n---\nbody\n"})`. Run `binPath --config <cfg> --vault test resolve "Existing Task Name" --output json`. Expect exit 0 and parsed JSON `HaveKeyWithValue("type","task")`, `HaveKeyWithValue("found", true)`, `HaveKeyWithValue("name","Existing Task Name")`.
   - **Goal match (AC2):** create a temp vault via `createTempVault(map[string]string{})` (empty tasks), then create `filepath.Join(vaultPath, "Goals")` with `os.MkdirAll(..., 0755)` and write `filepath.Join(vaultPath, "Goals", "Existing Goal Name.md")` with content `"---\nstatus: todo\n---\nbody\n"` (perm 0600). Run `binPath --config <cfg> --vault test resolve "Existing Goal Name" --output json`. Expect exit 0 and `HaveKeyWithValue("type","goal")`, `HaveKeyWithValue("found", true)`.
   - **Not found (AC3):** using the task-match vault, run `binPath --config <cfg> --vault test resolve "Does Not Exist" --output json`. Expect exit 0 and `HaveKeyWithValue("found", false)`, `HaveKeyWithValue("type","")`, `HaveKeyWithValue("name","Does Not Exist")`.
   - Each case must `AfterEach(cleanup)` to remove the temp vault, matching the pattern in existing Describe blocks.
   - NOTE on JSON number/bool types: `json.Unmarshal` into `map[string]any` decodes `true`/`false` as Go `bool`, so `HaveKeyWithValue("found", true)` works directly.

5. Run `make test` iteratively until green, then `make precommit` once at the end (AC6).
</requirements>

<constraints>
- Output contract: `PrintJSON` for JSON; plain mode is a silent no-op that ALWAYS exits 0 (not-found is not an error) — spec Constraint + Desired Behavior step 7.
- Multi-vault: iterate vaults, first `Found == true` wins; total miss returns `found:false` exit 0. Do NOT use `VaultDispatcher.FirstSuccess` (it errors on total miss — wrong for resolve). See context decision above.
- Error handling: `github.com/bborbe/errors` wrapping with `ctx`; never `fmt.Errorf`, never `context.Background()` in pkg/ (spec Constraint).
- No new dependencies, no new storage methods, no new interfaces (spec Constraint).
- Never import `encoding/json` in `pkg/cli/cli.go` — use `PrintJSON` (project convention, `docs/development-patterns.md`). The `encoding/json` import in the INTEGRATION test is fine (that file already imports it).
- Do NOT modify `pkg/domain/resolve_result.go` (prompt 01) or `pkg/ops/resolve.go` (prompt 02) — consume them as-is.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass (AC8).

Acceptance criteria satisfied end-to-end by this prompt (from spec 021):
- [ ] AC1 — Task match returns `{"type":"task",...,"found":true}`
- [ ] AC2 — Goal match returns `{"type":"goal",...,"found":true}`
- [ ] AC3 — Not found returns `{"type":"","name":...,"found":false}`, exit 0
- [ ] AC4 — Task-first priority (operation-level, exercised through the wired command)
- [ ] AC5 — Vault scoping via `--vault` (same JSON shape, single-vault probe)
- [ ] AC6 — `make precommit` passes with resolve wired in
- [ ] AC7 — `grep -n "resolve" integration/cli_test.go` returns ≥1 line
- [ ] AC8 — No regression: `task get/goal get/task show/goal show` unaffected

Failure modes covered (from spec 021 Failure Modes table): total miss across vaults → `found:false` exit 0 (not an error). Storage errors, special-character names, and missing dirs are handled at the operation/storage layer (prompt 02) with no new failure surface at the CLI.
</constraints>

<verification>
Run `make precommit` — must pass (AC6).
Run `make test` — unit + integration suite passes (AC8).
Run `grep -n "resolve" integration/cli_test.go` — ≥1 line (AC7).
Run `grep -rn "createResolveCommand" pkg/cli/cli.go` — CLI command wired (spec Verification).
Manual smoke (optional, inside container): build the binary and run
`./vault-cli --config <tmp-config> --vault test resolve "Existing Task Name" --output json | jq -e '.type == "task"'`.
</verification>
