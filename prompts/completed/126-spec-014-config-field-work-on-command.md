---
status: completed
spec: [014-bug-work-on-silent-failure-and-hardcoded-slash-command]
summary: Added configurable work_on_command field to Vault config with default /vault-cli:work-on-task
container: vault-cli-exec-126-spec-014-config-field-work-on-command
dark-factory-version: v0.171.1-3-gd94f1fa
created: "2026-05-24T14:30:00Z"
queued: "2026-05-24T14:24:43Z"
started: "2026-05-24T14:24:45Z"
completed: "2026-05-24T14:26:48Z"
branch: dark-factory/bug-work-on-silent-failure-and-hardcoded-slash-command
---

<summary>
- `Vault` struct gains `WorkOnCommand string` field with yaml/json tags
- `GetWorkOnCommand()` method returns the field value or default `/vault-cli:work-on-task`
- Follows existing pattern for optional vault fields (e.g., `GetClaudeScript`)
</summary>

<objective>
Add a configurable `work_on_command` field to the vault configuration so the slash command sent to Claude is customizable per vault, defaulting to `/vault-cli:work-on-task`.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes:
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface/struct patterns, getter naming
- `pkg/config/config.go` — `Vault` struct (lines 25-43), `GetClaudeScript()` pattern (lines 137-142), existing field tagging conventions
- `pkg/config/vault_test.go` — test pattern for Vault getter methods (lines 101-111 for `GetClaudeScript`)
</context>

<requirements>
1. In `pkg/config/config.go`, in the `Vault` struct, add a new field after `SessionProjectDir` (line 36):
   ```go
   WorkOnCommand string `yaml:"work_on_command,omitempty" json:"work_on_command,omitempty"`
   ```

2. In `pkg/config/config.go`, add a getter method on `*Vault` after `GetSessionProjectDir()` (around line 109):
   ```go
   // GetWorkOnCommand returns the Claude slash command for starting work-on sessions,
   // defaulting to /vault-cli:work-on-task if not configured.
   func (v *Vault) GetWorkOnCommand() string {
       if v.WorkOnCommand != "" {
           return v.WorkOnCommand
       }
       return "/vault-cli:work-on-task"
   }
   ```

3. Follow the same pattern as `GetClaudeScript()` (lines 137-142):
   - Check if field is non-empty, return it
   - Otherwise return the default string

4. Add `omitempty` to both yaml and json tags so existing configs without the field are unaffected.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- The field must use `omitempty` on both yaml and json tags
- The default must be `/vault-cli:work-on-task` (not the old hardcoded value `/work-on-task`)
</constraints>

<verification>
Run `make precommit` — must pass.
Grep verification:
- `grep -n 'WorkOnCommand' pkg/config/config.go` returns the field and method
- `grep -n '"work_on_command"' pkg/config/config.go` returns the yaml and json tags
- `go test ./pkg/config/...` exits 0
</verification>