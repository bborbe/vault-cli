---
status: active
---

# Scenario 003: Task recurring completion

Validates that completing a recurring task (one with a `recurring:` frontmatter field) does NOT mark it `completed`. Status stays `in_progress`, body checkboxes reset to unchecked, and `defer_date` is bumped forward by the recurrence interval.

## Setup

```bash
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
cp -r ~/Documents/workspaces/vault-cli/example/ "$WORK_DIR/"
sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml" && rm "$WORK_DIR/config.yaml.bak"
CONFIG="$WORK_DIR/config.yaml"
TASK_FILE="$WORK_DIR/vault/24 Tasks/Weekly Review.md"
NEXT_WEEK=$(date -v+7d +%Y-%m-%d 2>/dev/null || date -d '+7 days' +%Y-%m-%d)
```

- [ ] `$VAULT_CLI --config $CONFIG task list` shows Weekly Review
- [ ] `grep "recurring:" "$TASK_FILE"` succeeds (fixture is recurring)

## Action

### Complete the recurring task
- [ ] `$VAULT_CLI --config $CONFIG task complete "Weekly Review"` exits 0

## Expected

- [ ] `grep "status: in_progress" "$TASK_FILE"` succeeds (status NOT set to completed)
- [ ] `grep -E "^- \[ \]" "$TASK_FILE"` finds at least one unchecked checkbox (checkboxes were reset)
- [ ] `grep -E "^- \[x\]" "$TASK_FILE"` finds zero checked checkboxes
- [ ] `grep "defer_date: $NEXT_WEEK" "$TASK_FILE"` succeeds (deferred 1 week forward)

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
