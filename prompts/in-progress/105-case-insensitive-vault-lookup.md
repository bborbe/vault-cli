---
status: committing
summary: Implemented case-insensitive vault name lookup by normalizing config keys, Vault.Name, and DefaultVault to lowercase on load, and lowercasing vaultName before map lookup in GetVault; added corresponding Ginkgo tests for mixed-case scenarios.
container: vault-cli-105-case-insensitive-vault-lookup
dark-factory-version: v0.125.1
created: "2026-04-19T00:00:00Z"
queued: "2026-04-19T11:21:01Z"
started: "2026-04-19T11:43:38Z"
completed: "2026-04-19T11:23:46Z"
lastFailReason: 'validate completion report: completion report status: partial'
---

<summary>
- Users can pass vault names in any case (e.g. "Personal", "PERSONAL") and they resolve to the correct vault.
- Mixed-case keys in ~/.vault-cli/config.yaml are normalized to lowercase on load.
- Default vault resolution continues to work as today.
- Unit tests cover mixed-case lookup, config normalization, and default vault resolution.
- No behavior change when all-lowercase names are used.
</summary>

<objective>
Make vault name lookup case-insensitive in vault-cli by lowercasing vault map keys,
Vault.Name fields, and DefaultVault on config load, and by lowercasing the incoming
vaultName parameter in GetVault before map lookup. This fixes the "vault not found"
error when users pass a folder-cased name like "Personal" while the config keys are
lowercase by convention.
</objective>

<context>
Go CLI project. Read CLAUDE.md for project conventions.
Read docs/development-patterns.md for project patterns.

Files to read before changes:
- pkg/config/config.go — the Load and GetVault methods to modify
- pkg/config/config_test.go — existing Ginkgo v2/Gomega test patterns to follow
- pkg/config/config_suite_test.go — test suite bootstrap

No interface changes; no counterfeiter regeneration required.
</context>

<requirements>
1. Edit `pkg/config/config.go` `Load()` method (the `configLoader.Load` function).
   After `yaml.Unmarshal(data, &config)` succeeds and before `return &config, nil`,
   normalize the config in-place:
   - Lowercase `config.DefaultVault` using `strings.ToLower`.
   - Rebuild `config.Vaults` into a new `map[string]Vault` where every key is
     lowercased via `strings.ToLower(key)` and every entry's `Vault.Name` field
     is also lowercased via `strings.ToLower(vault.Name)`.
   - Preserve all other `Vault` fields unchanged (Path, TasksDir, GoalsDir,
     ThemesDir, ObjectivesDir, VisionDir, DailyDir, ClaudeScript,
     SessionProjectDir, Excludes).
   - Add `"strings"` to the existing import block.

   Example shape:
   ```go
   config.DefaultVault = strings.ToLower(config.DefaultVault)
   normalized := make(map[string]Vault, len(config.Vaults))
   for key, vault := range config.Vaults {
       vault.Name = strings.ToLower(vault.Name)
       normalized[strings.ToLower(key)] = vault
   }
   config.Vaults = normalized
   ```

2. Edit `pkg/config/config.go` `GetVault()` method (the `configLoader.GetVault`
   function). Lowercase the resolved vault name before the map lookup. Place the
   lowercasing AFTER the `if vaultName == ""` default fallback so that an empty
   input first picks up `config.DefaultVault` (which is itself already lowercased
   by Load).

   Change this block:
   ```go
   if vaultName == "" {
       vaultName = config.DefaultVault
   }

   // Look up vault
   vault, ok := config.Vaults[vaultName]
   ```
   To:
   ```go
   if vaultName == "" {
       vaultName = config.DefaultVault
   }
   vaultName = strings.ToLower(vaultName)

   // Look up vault
   vault, ok := config.Vaults[vaultName]
   ```

   Keep the existing error message format: `fmt.Errorf("vault not found: %s", vaultName)`.

3. Do NOT modify `getDefaultConfig()` — its keys are already lowercase.

4. Do NOT modify `GetAllVaults`, `GetVaultPath`, or `GetCurrentUser` — they route
   through `Load` / `GetVault` and therefore inherit the normalization.

5. Add new unit tests to `pkg/config/config_test.go` following the existing
   Ginkgo v2/Gomega style (`Describe`/`Context`/`BeforeEach`/`It`, `Expect(...).To(...)`).
   Reuse the `ctx`, `tempDir`, `configPath`, and `loader` variables already declared
   at the top of the `Describe("Loader", ...)` block.

   Add the following new `Context` blocks / `It` cases:

   a) Inside `Describe("Load", ...)`, add a new `Context("mixed-case keys", ...)`:
      - Write a config yaml where keys and Name are mixed-case:
        ```yaml
        default_vault: Personal
        vaults:
          Personal:
            name: Personal
            path: /path/personal
          WORK:
            name: WORK
            path: /path/work
        ```
      - `It("normalizes vault map keys to lowercase")`: assert
        `cfg.Vaults` has keys `"personal"` and `"work"` and does NOT have
        `"Personal"` or `"WORK"`.
      - `It("normalizes Vault.Name to lowercase")`: assert
        `cfg.Vaults["personal"].Name == "personal"` and
        `cfg.Vaults["work"].Name == "work"`.
      - `It("normalizes DefaultVault to lowercase")`: assert
        `cfg.DefaultVault == "personal"`.

   b) Inside `Describe("GetVault", ...)`, add a new
      `Context("case-insensitive lookup", ...)`:
      - Write a config yaml with a lowercase key (the common case):
        ```yaml
        vaults:
          personal:
            name: personal
            path: /path/personal
        ```
      - `It("resolves mixed-case vault name")`: call
        `loader.GetVault(ctx, "Personal")`, expect no error, expect
        `vault.Name == "personal"` and `vault.Path == "/path/personal"`.
      - `It("resolves upper-case vault name")`: call
        `loader.GetVault(ctx, "PERSONAL")`, expect no error, expect
        `vault.Name == "personal"`.
      - `It("still returns error for unknown vault regardless of case")`:
        call `loader.GetVault(ctx, "Nonexistent")`, expect
        `err.Error()` to contain `"vault not found"`.

   c) Inside `Describe("GetVault", ...)`, add a new
      `Context("mixed-case default vault", ...)`:
      - Write a config yaml where `default_vault` is mixed-case but the map
        key is lowercase:
        ```yaml
        default_vault: Personal
        vaults:
          personal:
            name: personal
            path: /path/personal
        ```
      - `It("resolves default vault when called with empty name")`: call
        `loader.GetVault(ctx, "")`, expect no error, expect
        `vault.Name == "personal"`.

6. Do NOT change any existing tests.

7. Do NOT modify any other files (no cli/ops/lint changes). Callers who pass
   vault names from `--vault` flags will now be handled transparently.
</requirements>

<constraints>
- Go project using coding-guidelines (interface → constructor → struct → method).
- Ginkgo v2 + Gomega for tests; follow existing style in config_test.go.
- Counterfeiter mocks: no interface change, so do NOT regenerate mocks.
- Keep `strings.ToLower` as the only normalization — no trimming, no Unicode folding.
- Do NOT change the `Loader` interface signatures.
- Do NOT change behavior of `getDefaultConfig()`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from the repo root:

1. `make test` — all tests pass, including the new ones.
2. `make precommit` — must pass end-to-end (formatting, lint, vet, tests).

Spot-check the new tests are actually exercised:
- `go test ./pkg/config/... -run TestConfig -v` should show the new
  `Context` descriptions ("mixed-case keys", "case-insensitive lookup",
  "mixed-case default vault") in the output.
</verification>

<success_criteria>
- `make precommit` passes.
- `loader.GetVault(ctx, "Personal")` returns the vault keyed as `"personal"` in config.
- Config yaml with mixed-case keys loads with all keys/names lowercased.
- `default_vault: Personal` in yaml results in `cfg.DefaultVault == "personal"`.
- No changes to `Loader` interface; no mock regeneration.
- No behavior change for existing all-lowercase configs.
</success_criteria>
