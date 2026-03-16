# Scenario: Task lifecycle — list, work-on, defer, complete

## Setup

```bash
WORK_DIR=$(mktemp -d)
cp -r example/ "$WORK_DIR/"
sed -i '' "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml"
CONFIG="$WORK_DIR/config.yaml"
```

- [ ] `vault-cli --config $CONFIG task list` runs without error
- [ ] Output shows 2 tasks (Simple Task, Weekly Review)

## Action

### List tasks
- [ ] `vault-cli --config $CONFIG task list --output json` returns JSON array with 2 entries

### Work on a task
- [ ] `vault-cli --config $CONFIG task work-on "Simple Task"`
- [ ] Verify: `grep "status: in_progress" "$WORK_DIR/vault/24 Tasks/Simple Task.md"` succeeds
- [ ] Verify: `grep "assignee: alice" "$WORK_DIR/vault/24 Tasks/Simple Task.md"` succeeds

### Defer a task
- [ ] `vault-cli --config $CONFIG task defer "Simple Task" +1d`
- [ ] Verify: `grep "defer_date:" "$WORK_DIR/vault/24 Tasks/Simple Task.md"` shows tomorrow's date

### Complete a non-recurring task
- [ ] `vault-cli --config $CONFIG task complete "Simple Task"`
- [ ] Verify: `grep "status: completed" "$WORK_DIR/vault/24 Tasks/Simple Task.md"` succeeds

### Complete a recurring task
- [ ] `vault-cli --config $CONFIG task complete "Weekly Review"`
- [ ] Verify: status stays `in_progress` (not completed)
- [ ] Verify: checkboxes reset to `- [ ]` (not `- [x]`)
- [ ] Verify: `defer_date` bumped forward by 1 week

## Expected

- [ ] Simple Task: status=completed, has defer_date, assignee=alice
- [ ] Weekly Review: status=in_progress, checkboxes unchecked, defer_date=2026-03-23
- [ ] No other files modified

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
