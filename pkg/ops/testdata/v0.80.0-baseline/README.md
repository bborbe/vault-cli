# v0.80.0 Baseline

## Baseline Source

commit: unavailable

**Note:** Git history is masked in the YOLO container (`.git` appears as a character device).
The v0.80.0 tag could not be resolved via `git rev-list` and no baseline binary could be
built from the tagged commit. The outputs captured here were produced by the feature-branch
binary built at capture time (`/tmp/new-vault-cli`).

Capture date: 2026-06-20

## Purpose

These files record the expected on-disk and JSON output shapes for scenarios 002, 003, and 004
after the typed-date migration (prompts 1-3). They serve as a regression baseline for future
changes: any mutation to date formatting, frontmatter encoding, or JSON projection that changes
these outputs requires a deliberate update to the baseline files.

## Replay Commands

The following commands were used to capture the baseline. Replace `/tmp/new-vault-cli` with
the binary under test.

### Scenario 002 — Task lifecycle

```bash
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
cp -R /path/to/vault-cli/example/. "$WORK_DIR/"
sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml" && rm "$WORK_DIR/config.yaml.bak"
CONFIG="$WORK_DIR/config.yaml"

# Capture pre-action task list JSON
$VAULT_CLI --config $CONFIG task list --output json > scenario-002/task-list.json

# Run scenario actions
$VAULT_CLI --config $CONFIG task defer "Simple Task" +1d
$VAULT_CLI --config $CONFIG task complete "Simple Task"

# Capture resulting markdown and show JSON
cp "$WORK_DIR/vault/24 Tasks/Simple Task.md" scenario-002/Simple-Task.md
$VAULT_CLI --config $CONFIG task show "Simple Task" --output json > scenario-002/task-show.json

rm -rf "$WORK_DIR"
```

### Scenario 003 — Recurring task completion

```bash
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
cp -R /path/to/vault-cli/example/. "$WORK_DIR/"
sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml" && rm "$WORK_DIR/config.yaml.bak"
CONFIG="$WORK_DIR/config.yaml"

$VAULT_CLI --config $CONFIG task complete "Weekly Review"
cp "$WORK_DIR/vault/24 Tasks/Weekly Review.md" scenario-003/Weekly-Review.md

rm -rf "$WORK_DIR"
```

### Scenario 004 — Decision ack

```bash
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
cp -R /path/to/vault-cli/example/. "$WORK_DIR/"
sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml" && rm "$WORK_DIR/config.yaml.bak"
CONFIG="$WORK_DIR/config.yaml"

$VAULT_CLI --config $CONFIG decision ack "Review Architecture"
cp "$WORK_DIR/vault/25 Decisions/Review Architecture.md" scenario-004/Review-Architecture.md

rm -rf "$WORK_DIR"
```

## Captured Files

- `scenario-002/task-list.json` — `task list --output json` before any mutations
- `scenario-002/Simple-Task.md` — on-disk markdown after defer + complete
- `scenario-002/task-show.json` — `task show --output json` after complete
- `scenario-003/Weekly-Review.md` — on-disk markdown after completing a recurring task
- `scenario-004/Review-Architecture.md` — on-disk markdown after `decision ack`
