---
status: completed
summary: Extract expandVaultPaths helper in pkg/config/config.go to deduplicate tilde expansion and template path resolution between GetVault and GetAllVaults
execution_id: vault-cli-exec-150-fix-config-path-expansion-duplication
dark-factory-version: v0.188.1
created: "2026-06-28T22:00:00Z"
queued: "2026-06-28T20:59:31Z"
started: "2026-06-28T21:07:06Z"
completed: "2026-06-28T21:09:09Z"
---

<summary>
- `config.go` has identical home-dir expansion + template path resolution duplicated in `GetVault` and `GetAllVaults`
- ~80 lines of identical code across two methods — extract into a shared `expandVaultPaths` helper
- Fix reduces duplication, eliminates the risk of one method drifting from the other
- No behavior change — same logic, one code path
</summary>

<objective>
Deduplicate the ~40 lines of identical path-expansion code that appears twice in `pkg/config/config.go` by extracting a single `expandVaultPaths` helper.
</objective>

<context>
Read:
- `pkg/config/config.go` — `GetVault` method starting at line 215, specifically lines 233-263 (tilde expansion + template resolution)
- Same file, `GetAllVaults` starting at line 269, specifically lines 278-307 (identical logic)
- `resolveTemplatePath` helper function at line 339 — used by both callers
</context>

<requirements>
1. **Extract helper function** in `pkg/config/config.go`:
   - Add `func expandVaultPaths(ctx context.Context, vault *Vault) (*Vault, error)` that contains the duplicate logic
   - Implementation: `result := *vault` (shallow copy on entry), mutate `result.Path`, `result.SessionProjectDir`, and each template field on the copy, return `&result`
   - Logic to extract: expand `~` in `result.Path` to home dir, expand `~` in `result.SessionProjectDir`, resolve all template fields (`TaskTemplate`, `GoalTemplate`, `ThemeTemplate`, `ObjectiveTemplate`, `VisionTemplate`) via `resolveTemplatePath` on the copy

2. **Update `GetVault`** (~line 258): replace lines 233-263 with `return expandVaultPaths(ctx, &vault)`

3. **Update `GetAllVaults`** (~line 307): replace lines 278-307 with a single call inside the existing loop — after `v := vault`, call `expanded, err := expandVaultPaths(ctx, &v)` and set `vaults[i] = expanded`

4. **Verify**: `GetVault` copies `vault` (not a pointer), modifies, returns `&vault`. The new helper must work with both callers' patterns.

5. **Existing tests must still pass** — run `make precommit`
</requirements>

<constraints>
- Preserve the `v := vault` copy pattern in `GetAllVaults` (avoids pointer aliasing bugs)
- The helper should NOT mutate the input vault — create a copy internally or accept `*Vault` and return a new one
- No behavior change for any config path
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
