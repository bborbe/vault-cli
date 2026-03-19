---
status: completed
summary: Added optional session_project_dir vault config field with tilde expansion, GetSessionProjectDir getter, sessionDir parameter to WorkOnOperation.Execute, updated mock, CLI wiring, and tests
container: vault-cli-087-add-session-project-dir-config
dark-factory-version: v0.57.5
created: "2026-03-19T10:39:35Z"
queued: "2026-03-19T10:51:00Z"
started: "2026-03-19T10:51:03Z"
completed: "2026-03-19T10:57:53Z"
---

<summary>
- Vaults can configure a session_project_dir that overrides the working directory for Claude sessions
- When session_project_dir is set, work-on starts Claude sessions in that directory instead of the vault path
- When session_project_dir is empty, behavior is unchanged (sessions start in vault path)
- config list --output json includes session_project_dir in its output for each vault
- Tilde (~) in session_project_dir is expanded to the user's home directory
- All existing tests continue to pass
</summary>

<objective>
Add an optional `session_project_dir` field to the vault configuration so that `work-on` can start Claude sessions in a directory different from the vault path. This is needed for vaults that use custom Claude scripts (e.g. claude-personal.sh) but need sessions created in a specific project directory.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes:
- `pkg/config/config.go` — `Vault` struct, `GetClaudeScript()` pattern, tilde expansion in `GetVault()` and `GetAllVaults()`
- `pkg/config/vault_test.go` — test pattern for Vault getter methods
- `pkg/ops/workon.go` — `handleClaudeSession` calls `w.starter.StartSession(ctx, prompt, vaultPath)` and `w.resumer.ResumeSession(sessionID, vaultPath)`
- `pkg/cli/cli.go` — `createWorkOnCommand` (~line 221) passes `vault.Path` to `workOnOp.Execute`; `createConfigListCommand` (~line 1655) uses `PrintJSON(vaults)` for JSON output
</context>

<requirements>
1. In `pkg/config/config.go`, add a new field to the `Vault` struct after `ClaudeScript`:
   ```go
   SessionProjectDir string `yaml:"session_project_dir,omitempty" json:"session_project_dir,omitempty"`
   ```

2. In `pkg/config/config.go`, add a getter method `GetSessionProjectDir` on `*Vault`. Unlike `GetClaudeScript` which returns a default value when empty, this returns empty string when not set (empty means "use vault path"):
   ```go
   // GetSessionProjectDir returns the session project directory override, or empty string if not set.
   func (v *Vault) GetSessionProjectDir() string {
       return v.SessionProjectDir
   }
   ```

3. In `pkg/config/config.go`, in the `GetVault` method, add tilde expansion for `SessionProjectDir` after the existing tilde expansion for `vault.Path` (~line 169-175). Follow the exact same pattern:
   ```go
   if len(vault.SessionProjectDir) > 0 && vault.SessionProjectDir[0] == '~' {
       homeDir, err := os.UserHomeDir()
       if err != nil {
           return nil, fmt.Errorf("get home directory: %w", err)
       }
       vault.SessionProjectDir = filepath.Join(homeDir, vault.SessionProjectDir[1:])
   }
   ```
   Note: reuse the existing `homeDir` variable if it's already in scope from the Path expansion, or redeclare if in a separate block.

4. In `pkg/config/config.go`, in the `GetAllVaults` method, add the same tilde expansion for `SessionProjectDir` after the existing tilde expansion for `v.Path` (~line 191-197). Same pattern as step 3.

5. In `pkg/cli/cli.go`, in `createWorkOnCommand`, in the closure where `workOnOp.Execute` is called (~line 267), determine the session working directory and pass it. Change:
   ```go
   return workOnOp.Execute(
       ctx,
       vault.Path,
       taskName,
       currentUser,
       vault.Name,
       *outputFormat,
       isInteractive,
   )
   ```
   to:
   ```go
   sessionDir := vault.Path
   if dir := vault.GetSessionProjectDir(); dir != "" {
       sessionDir = dir
   }
   return workOnOp.Execute(
       ctx,
       vault.Path,
       taskName,
       currentUser,
       vault.Name,
       *outputFormat,
       isInteractive,
       sessionDir,
   )
   ```

6. In `pkg/ops/workon.go`, update the `WorkOnOperation` interface `Execute` method to accept an additional `sessionDir string` parameter at the end:
   ```go
   Execute(
       ctx context.Context,
       vaultPath string,
       taskName string,
       assignee string,
       vaultName string,
       outputFormat string,
       isInteractive bool,
       sessionDir string,
   ) error
   ```

7. In `pkg/ops/workon.go`, update the `workOnOperation.Execute` method signature to match the interface (add `sessionDir string` parameter).

8. In `pkg/ops/workon.go`, in the `Execute` method, change the call to `w.handleClaudeSession` (~line 104) from:
   ```go
   sessionID, sessionErr := w.handleClaudeSession(ctx, task, vaultPath)
   ```
   to:
   ```go
   sessionID, sessionErr := w.handleClaudeSession(ctx, task, sessionDir)
   ```

9. In `pkg/ops/workon.go`, in the `Execute` method, change the call to `w.resumer.ResumeSession` (~line 112) from:
   ```go
   return w.resumer.ResumeSession(sessionID, vaultPath)
   ```
   to:
   ```go
   return w.resumer.ResumeSession(sessionID, sessionDir)
   ```

10. Regenerate the mock for `WorkOnOperation` by running:
    ```
    go generate ./pkg/ops/...
    ```
    This updates `mocks/workon-operation.go` from the counterfeiter directive. If counterfeiter is not available, manually update the mock to add `sessionDir string` as the 8th argument throughout (struct fields, Execute signature, all helper methods).

11. In `pkg/config/vault_test.go`, add a test for `GetSessionProjectDir` following the existing pattern (e.g. `GetClaudeScript` tests):
    ```go
    Describe("GetSessionProjectDir", func() {
        It("returns custom session project dir when set", func() {
            vault := &config.Vault{SessionProjectDir: "/custom/project/dir"}
            Expect(vault.GetSessionProjectDir()).To(Equal("/custom/project/dir"))
        })

        It("returns empty string when not set", func() {
            vault := &config.Vault{}
            Expect(vault.GetSessionProjectDir()).To(Equal(""))
        })
    })
    ```

12. The `config list --output json` command already works because it calls `PrintJSON(vaults)` which serializes the `Vault` struct. Since we added the `json:"session_project_dir,omitempty"` tag in step 1, the field will appear in JSON output when set and be omitted when empty. No additional code changes needed for config list.

13. Search for any other callers of `WorkOnOperation.Execute` by grepping for `\.Execute(` in `pkg/cli/cli.go` and all `*_test.go` files. Update each caller to pass the additional `sessionDir` parameter. For test files using the generated mock, the 8th arg should be the vault path (preserving existing behavior). Expected callers: `createWorkOnCommand` in `pkg/cli/cli.go` (already updated in step 5) and any workon integration tests in `pkg/ops/workon_test.go`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Do NOT change the plain text output of `config list` (only JSON is affected)
- Do NOT change `ClaudeSessionStarter` or `ClaudeResumer` interfaces — the override happens at the caller level
- The `vaultPath` parameter in `Execute` must remain unchanged — it is used for task storage operations. Only `sessionDir` controls where Claude sessions run.
- `session_project_dir` must use `omitempty` on both yaml and json tags so existing configs without it are unaffected
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
