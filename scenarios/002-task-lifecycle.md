---
status: active
---

# Scenario 002: Task lifecycle — list, work-on, defer, complete

Validates that the standard happy-path task lifecycle (list → work-on → defer → complete) updates the on-disk markdown frontmatter as expected for a non-recurring task.

## Setup

```bash
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
cp -r ~/Documents/workspaces/vault-cli/example/ "$WORK_DIR/"
sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml" && rm "$WORK_DIR/config.yaml.bak"
CONFIG="$WORK_DIR/config.yaml"
TASK_FILE="$WORK_DIR/vault/24 Tasks/Simple Task.md"
TOMORROW=$(date -v+1d +%Y-%m-%d 2>/dev/null || date -d '+1 day' +%Y-%m-%d)
```

- [ ] `$VAULT_CLI --config $CONFIG task list` runs without error
- [ ] Output shows at least Simple Task

## Action

### List tasks
- [ ] `$VAULT_CLI --config $CONFIG task list --output json` returns JSON array including Simple Task

### Work on a task
- [ ] `$VAULT_CLI --config $CONFIG task work-on "Simple Task"` exits 0

### Defer the task
- [ ] `$VAULT_CLI --config $CONFIG task defer "Simple Task" +1d` exits 0

### Complete the task
- [ ] `$VAULT_CLI --config $CONFIG task complete "Simple Task"` exits 0

## Expected

- [ ] `grep "status: completed" "$TASK_FILE"` succeeds
- [ ] `grep "assignee: alice" "$TASK_FILE"` succeeds
- [ ] `grep "defer_date: $TOMORROW" "$TASK_FILE"` succeeds

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
