---
status: active
---

# Scenario 004: Decision list and ack workflow

Validates that `decision list` reports unreviewed decisions, `decision ack` marks them reviewed (updating `needs_review`, `reviewed`, `reviewed_date`), and the post-ack list filters reflect the new state.

## Setup

```bash
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
cp -r ~/Documents/workspaces/vault-cli/example/ "$WORK_DIR/"
sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml" && rm "$WORK_DIR/config.yaml.bak"
CONFIG="$WORK_DIR/config.yaml"
TODAY=$(date +%Y-%m-%d)
DECISION_FILE="$WORK_DIR/vault/25 Decisions/Review Architecture.md"
```

- [ ] `$VAULT_CLI --config $CONFIG decision list` runs without error
- [ ] Output shows 1 decision (Review Architecture)

## Action

### List unreviewed decisions
- [ ] `$VAULT_CLI --config $CONFIG decision list --output json` returns JSON array with 1 entry
- [ ] Entry has `needs_review: true`

### Acknowledge the decision
- [ ] `$VAULT_CLI --config $CONFIG decision ack "Review Architecture"` exits 0

### List filters reflect new state
- [ ] `$VAULT_CLI --config $CONFIG decision list` returns empty output
- [ ] `$VAULT_CLI --config $CONFIG decision list --reviewed` shows Review Architecture

## Expected

- [ ] `grep "needs_review: false" "$DECISION_FILE"` succeeds
- [ ] `grep "reviewed: true" "$DECISION_FILE"` succeeds
- [ ] `grep "reviewed_date: $TODAY" "$DECISION_FILE"` succeeds
- [ ] Markdown body content preserved unchanged

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
