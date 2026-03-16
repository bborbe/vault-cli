# Scenario: Decision list and ack workflow

## Setup

```bash
WORK_DIR=$(mktemp -d)
cp -r example/ "$WORK_DIR/"
sed -i '' "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml"
CONFIG="$WORK_DIR/config.yaml"
```

- [ ] `vault-cli --config $CONFIG decision list` runs without error
- [ ] Output shows 1 decision (Review Architecture)

## Action

### List unreviewed decisions
- [ ] `vault-cli --config $CONFIG decision list --output json` returns JSON array with 1 entry
- [ ] Entry has `needs_review: true`

### Acknowledge a decision
- [ ] `vault-cli --config $CONFIG decision ack "Review Architecture"`
- [ ] Verify: `grep "needs_review: false" "$WORK_DIR/vault/25 Decisions/Review Architecture.md"` succeeds
- [ ] Verify: `grep "reviewed: true" "$WORK_DIR/vault/25 Decisions/Review Architecture.md"` succeeds
- [ ] Verify: `grep "reviewed_date:" "$WORK_DIR/vault/25 Decisions/Review Architecture.md"` shows today's date

### List again — should be empty
- [ ] `vault-cli --config $CONFIG decision list` returns empty output
- [ ] `vault-cli --config $CONFIG decision list --reviewed` shows Review Architecture

## Expected

- [ ] Review Architecture: needs_review=false, reviewed=true, reviewed_date=2026-03-16
- [ ] Markdown body content preserved unchanged
- [ ] No other files modified

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
