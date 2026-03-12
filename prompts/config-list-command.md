---
spec: ["044"]
status: created
created: "2026-03-12T17:00:00Z"
---
<summary>
- Add `vault-cli config list` command that outputs all vault configurations
- Vault paths are always absolute in output (tilde resolved)
- Supports `--output plain` (default) and `--output json` formats
- Enables external tools (task-orchestrator) to discover vault configs without reading the YAML directly
- Respects existing `--vault` flag to filter output to a single vault
- Adds JSON struct tags to the Vault type for proper serialization
</summary>

<objective>
Add a `config list` subcommand that prints all configured vaults so external tools can programmatically discover vault names, paths, and directory settings. Supports `--output plain` (default) and `--output json`.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes:

- `pkg/cli/cli.go` — CLI structure with cobra commands; note how `taskCmd` and `goalCmd` are created and added to `rootCmd`; note the `getVaults` helper and `configLoader` pattern
- `pkg/config/config.go` — `Config`, `Vault` structs; `Loader` interface with `GetAllVaults(ctx)` method; `GetVault(ctx, name)` for single vault lookup
- `prompts/completed/043-version-flag.md` — recent prompt example for reference on style

The output JSON format should match the `config.Vault` struct fields with expanded paths:
```json
[
  {
    "name": "personal",
    "path": "/Users/bborbe/Documents/Obsidian/Personal",
    "tasks_dir": "24 Tasks",
    "goals_dir": "23 Goals",
    "daily_dir": "60 Periodic Notes/Daily"
  }
]
```

The `--vault` flag should work to filter to a single vault (consistent with other commands).
The `--output` flag already exists on the root command — reuse it for json/plain formatting.
</context>

<requirements>
1. In `pkg/cli/cli.go`, create a new function `createConfigListCommand` following the same pattern as other command creators (receives `ctx`, `configLoader`, `vaultName`, `outputFormat` pointers):

   ```go
   func createConfigListCommand(
       ctx context.Context,
       configLoader *config.Loader,
       vaultName *string,
       outputFormat *string,
   ) *cobra.Command {
   ```

   The command:
   - Use: `"list"`, Short: `"List configured vaults"`
   - Args: `cobra.NoArgs`
   - RunE function:
     a. Call `getVaults(ctx, configLoader, vaultName)` to get vaults (respects `--vault` flag)
     b. If `*outputFormat == "json"`: use `PrintJSON(vaults)` from `output.go` (same package, no extra import needed)
     c. If `*outputFormat == "plain"`: print one vault per line as `name<tab>path`, e.g. `personal	/Users/bborbe/Documents/Obsidian/Personal`

2. Add a parent `configCmd` cobra.Command in `Run()`:
   ```go
   configCmd := &cobra.Command{
       Use:   "config",
       Short: "Configuration management",
   }
   configCmd.AddCommand(createConfigListCommand(ctx, &configLoader, &vaultName, &outputFormat))
   rootCmd.AddCommand(configCmd)
   ```

   Place this after `rootCmd.AddCommand(createVisionCommands(...))` (~line 99), before `rootCmd.SetArgs(args)`.

3. Do NOT add `"encoding/json"` to `cli.go` — use the existing `PrintJSON()` helper from `output.go` (same package) for JSON output. No new import needed.

4. The `config.Vault` struct in `pkg/config/config.go` needs JSON tags added alongside existing YAML tags. Add `json:"..."` tags matching the yaml tag names:
   ```go
   type Vault struct {
       Path          string `yaml:"path" json:"path"`
       Name          string `yaml:"name" json:"name"`
       TasksDir      string `yaml:"tasks_dir,omitempty" json:"tasks_dir,omitempty"`
       GoalsDir      string `yaml:"goals_dir,omitempty" json:"goals_dir,omitempty"`
       ThemesDir     string `yaml:"themes_dir,omitempty" json:"themes_dir,omitempty"`
       ObjectivesDir string `yaml:"objectives_dir,omitempty" json:"objectives_dir,omitempty"`
       VisionDir     string `yaml:"vision_dir,omitempty" json:"vision_dir,omitempty"`
       DailyDir      string `yaml:"daily_dir,omitempty" json:"daily_dir,omitempty"`
   }
   ```
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Follow existing command creation patterns exactly (same function signature style)
- Reuse `getVaults` for vault resolution — do not duplicate vault lookup logic
- Paths must be expanded (no `~`) in output — `GetAllVaults` already handles this
- Existing tests must still pass
</constraints>

<verification>
Run `make precommit` — must pass.
Smoke test:
- `go run main.go config list` — plain output, one line per vault
- `go run main.go config list --output json` — JSON array output
- `go run main.go config list --vault personal` — single vault only
</verification>
