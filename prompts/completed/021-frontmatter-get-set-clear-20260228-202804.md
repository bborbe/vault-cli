---
status: completed
---

<objective>
Add generic frontmatter field operations to vault-cli:
`vault-cli task get <name> <key>`, `vault-cli task set <name> <key> <value>`, `vault-cli task clear <name> <key>`.
Enables task-orchestrator to update any frontmatter field (phase, claude_session_id, assignee, etc.)
without spawning a Claude session.
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Read CLAUDE.md for project conventions.
Read ~/.claude/docs/go-patterns.md for interface/constructor patterns.
Read ~/.claude/docs/go-testing.md for testing patterns.

Existing pattern to follow: `pkg/ops/workon.go` — finds task by name, updates frontmatter fields, writes back.
Storage already supports `FindTaskByName` and `WriteTask` via the Storage interface.
The Task domain type in `pkg/domain/` has typed fields for known keys.
</context>

<requirements>
1. Add three subcommands under `vault-cli task` in `./pkg/cli/cli.go`:
   - `vault-cli task get <task-name> <key>` — print value of frontmatter field
   - `vault-cli task set <task-name> <key> <value>` — set frontmatter field to value
   - `vault-cli task clear <task-name> <key>` — remove/empty frontmatter field

2. Implement in `./pkg/ops/frontmatter.go` (new file):
   - `FrontmatterGetOperation` interface + implementation
   - `FrontmatterSetOperation` interface + implementation
   - `FrontmatterClearOperation` interface + implementation
   - Follow Interface → Constructor → Struct → Method pattern from go-patterns.md

3. Support these keys (map to Task struct fields):
   - `phase` → Task.Phase (string)
   - `claude_session_id` → Task.ClaudeSessionID (string)
   - `assignee` → Task.Assignee (string)
   - `status` → Task.Status (domain.Status)
   - `priority` → Task.Priority (domain.Priority)
   - `defer_date` → Task.DeferDate (string, YYYY-MM-DD)
   - Unknown keys → return error "unknown field: <key>"

4. `get` output:
   - plain: print value as string, one line
   - json: `{"key": "phase", "value": "in_progress", "name": "Task Name"}`
   - If field is empty/zero: print empty string (no error)

5. `set` output:
   - plain: `✅ Set phase=in_progress on: Task Name`
   - json: `{"success": true, "key": "phase", "value": "in_progress", "name": "Task Name"}`

6. `clear` output:
   - plain: `✅ Cleared phase on: Task Name`
   - json: `{"success": true, "key": "phase", "name": "Task Name"}`

7. Respect `--vault` flag (existing pattern) and `--output` flag (added in prompt 020).
   Multi-vault: if --vault not set, search all vaults for the task by name.

8. Add counterfeiter mock comments to all new interfaces.

9. Write tests in `./pkg/ops/frontmatter_test.go`:
   - get: existing field → returns value; empty field → returns empty; unknown key → error
   - set: updates correct field, writes task; unknown key → error; task not found → error
   - clear: zeros correct field, writes task; unknown key → error
   Use mockStorage pattern from complete_test.go.
</requirements>

<implementation>
Follow workon.go pattern exactly:

```go
//counterfeiter:generate -o ../../mocks/frontmatter-get-operation.go --fake-name FrontmatterGetOperation . FrontmatterGetOperation
type FrontmatterGetOperation interface {
    Execute(ctx context.Context, vaultPath, taskName, key string) (string, error)
}

func NewFrontmatterGetOperation(storage storage.Storage) FrontmatterGetOperation {
    return &frontmatterGetOperation{storage: storage}
}

type frontmatterGetOperation struct {
    storage storage.Storage
}

func (o *frontmatterGetOperation) Execute(ctx context.Context, vaultPath, taskName, key string) (string, error) {
    task, err := o.storage.FindTaskByName(ctx, vaultPath, taskName)
    if err != nil {
        return "", errors.Wrap(ctx, err, "find task")
    }
    switch key {
    case "phase":
        return task.Phase, nil
    case "claude_session_id":
        return task.ClaudeSessionID, nil
    // ... etc
    default:
        return "", fmt.Errorf("unknown field: %s", key)
    }
}
```

CLI command pattern (follow createCompleteCommand):
```go
func createTaskGetCommand(...) *cobra.Command {
    return &cobra.Command{
        Use:   "get <task-name> <key>",
        Short: "Get a frontmatter field value",
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            taskName, key := args[0], args[1]
            // ... iterate vaults, call op.Execute
        },
    }
}
```
</implementation>

<constraints>
- Unknown keys must return an error (not silently ignore)
- `clear` sets string fields to "" and numeric fields to zero value
- Do NOT add arbitrary string map support — only the listed known fields
- Do NOT run make precommit iteratively — use make test; run make precommit once at the end
- Check if suite file exists before creating: `pkg/ops/ops_suite_test.go`
</constraints>

<verification>
Run: `make test`
Run: `go generate -mod=mod ./...` (regenerate mocks after adding counterfeiter comments)

Manual checks:
- `vault-cli task get "My Task" phase` → prints current phase
- `vault-cli task set "My Task" phase in_progress` → updates frontmatter
- `vault-cli task clear "My Task" claude_session_id` → clears field
- `vault-cli task get "My Task" unknown_key` → error: unknown field
- `vault-cli task set "My Task" phase done --output json` → JSON response
</verification>

<success_criteria>
- make test passes
- get/set/clear commands work for all listed fields
- Unknown fields return error
- --output json works for all three commands
- Tests cover success and error paths for all three ops
</success_criteria>
