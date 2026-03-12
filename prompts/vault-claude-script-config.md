---
status: created
created: "2026-03-12T21:30:00Z"
---

<summary>
- Each vault can optionally specify a custom AI script/wrapper in the config file
- If not configured, the default "claude" binary is used automatically
- The work-on command uses the vault's configured script instead of always using the default
- When listing vaults as JSON, the custom script is shown only when configured
- The new accessor and updated constructors are covered by tests
</summary>

<objective>
Add optional `claude_script` field to vault config so each vault can specify which Claude wrapper script to use for sessions, defaulting to "claude" when not set.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/config/config.go` — find the `Vault` struct with its YAML/JSON tags and the `Get*Dir()` accessor pattern.
Read `pkg/ops/claude_session.go` — find `NewClaudeSessionStarter()` (zero params, uses `exec.LookPath("claude")`) and `NewClaudeSessionStarterWithRunner` (testing seam).
Read `pkg/ops/claude_resume.go` — find `NewClaudeResumer()` (zero params, uses `exec.LookPath("claude")`) and `NewClaudeResumerForTesting` (testing seam).
Read `pkg/ops/workon.go` — find where `ClaudeSessionStarter` and `ClaudeResumer` are used.
Read `pkg/cli/cli.go` — find `createWorkOnCommand` where starter and resumer are created at ~line 304-305, currently outside the vault loop.
</context>

<requirements>
1. Add `ClaudeScript` field to the `Vault` struct in `pkg/config/config.go`:

```go
ClaudeScript string `yaml:"claude_script,omitempty" json:"claude_script,omitempty"`
```

2. Add a `GetClaudeScript()` accessor following the existing `Get*Dir()` pattern:

```go
func (v *Vault) GetClaudeScript() string {
    if v.ClaudeScript != "" {
        return v.ClaudeScript
    }
    return "claude"
}
```

3. Update `NewClaudeSessionStarter` in `pkg/ops/claude_session.go` to accept a `claudeScript string` parameter. Use `exec.LookPath(claudeScript)` instead of hardcoded `"claude"`:

```go
// Old:
func NewClaudeSessionStarter() ClaudeSessionStarter

// New:
func NewClaudeSessionStarter(claudeScript string) ClaudeSessionStarter
```

4. Update `NewClaudeResumer` in `pkg/ops/claude_resume.go` to accept a `claudeScript string` parameter. Use `exec.LookPath(claudeScript)` instead of hardcoded `"claude"`:

```go
// Old:
func NewClaudeResumer() ClaudeResumer

// New:
func NewClaudeResumer(claudeScript string) ClaudeResumer
```

5. In `createWorkOnCommand` in `pkg/cli/cli.go`, the `starter` and `resumer` are currently created once outside the vault loop (~line 304-305). Move their creation inside the per-vault branches so each vault uses its own `vault.GetClaudeScript()`:

```go
// Old (outside loop):
starter := ops.NewClaudeSessionStarter()
resumer := ops.NewClaudeResumer()

// New (inside per-vault code, after vault is resolved):
starter := ops.NewClaudeSessionStarter(vault.GetClaudeScript())
resumer := ops.NewClaudeResumer(vault.GetClaudeScript())
```

6. Update tests for `ClaudeSessionStarter` and `ClaudeResumer` to verify the custom script name is passed through. Follow the existing `NewClaudeSessionStarterWithRunner` and `NewClaudeResumerForTesting` patterns as testing seams.

7. Add test for `GetClaudeScript()` — returns custom value when set, "claude" when empty.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- All file paths are repo-relative
- Agent has no memory between prompts — all context must be in this prompt
- `claude_script` is optional — omitting it from config YAML must work and default to "claude"
- The JSON tag must include `omitempty` so `config list --output json` omits it when not set
- Use `os/exec` for `exec.LookPath` (standard library)
- Use `golang.org/x/term` for TTY detection (already in go.mod)
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
