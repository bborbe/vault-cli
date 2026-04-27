---
status: active
---

# Scenario 001: Config list — all vault fields surface correctly

Validates that `vault-cli config list` exposes every documented `Vault` field with correct path resolution and `omitempty` behavior. Pins the public JSON surface that downstream consumers (slash commands, agents, scripts) depend on for vault discovery.

## Setup

```bash
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
VAULT_CLI=/tmp/new-vault-cli
WORK_DIR=$(mktemp -d)
mkdir -p "$WORK_DIR/vault" "$WORK_DIR/templates"
touch "$WORK_DIR/templates/Task Template.md"

cat > "$WORK_DIR/config.yaml" <<EOF
current_user: alice
default_vault: full
vaults:
  full:
    path: $WORK_DIR/vault
    name: full
    tasks_dir: "24 Tasks"
    goals_dir: "23 Goals"
    themes_dir: "21 Themes"
    objectives_dir: "22 Objectives"
    vision_dir: "20 Vision"
    daily_dir: "60 Periodic Notes/Daily"
    session_project_dir: $WORK_DIR
    claude_script: claude.sh
    task_template: "templates/Task Template.md"
    goal_template: /abs/templates/goal.md
    theme_template: ~/templates/theme.md
    objective_template: "22 Templates/Objective Template.md"
    vision_template: "20 Templates/Vision Template.md"
    excludes:
      - "90 Templates"
      - ".obsidian"
  minimal:
    path: $WORK_DIR/vault
    name: minimal
EOF

CONFIG="$WORK_DIR/config.yaml"
```

## Action

### Plain output
- [ ] `$VAULT_CLI --config $CONFIG config list` runs without error
- [ ] Output mentions both vaults: `full` and `minimal`

### JSON output — full vault has all fields
- [ ] `$VAULT_CLI --config $CONFIG config list --output json` returns valid JSON
- [ ] Output for vault `full` contains: `path`, `name`, `tasks_dir`, `goals_dir`, `themes_dir`, `objectives_dir`, `vision_dir`, `daily_dir`, `session_project_dir`, `claude_script`, `excludes`, `task_template`, `goal_template`, `theme_template`, `objective_template`, `vision_template`

### Path resolution — template fields
- [ ] `task_template` (relative `templates/Task Template.md`) → resolves to `$WORK_DIR/vault/templates/Task Template.md`
- [ ] `goal_template` (absolute `/abs/templates/goal.md`) → returned unchanged
- [ ] `theme_template` (`~/templates/theme.md`) → tilde expanded; output starts with `$HOME` and contains no `~`
- [ ] `objective_template` (relative) → resolves under `$WORK_DIR/vault/`
- [ ] `vision_template` (relative) → resolves under `$WORK_DIR/vault/`

### Path resolution — vault path and session_project_dir
- [ ] `path` returned as the absolute filesystem path (no `~`)
- [ ] `session_project_dir` returned as absolute path (no `~`)

### Omitempty — minimal vault
- [ ] JSON entry for `minimal` does NOT contain keys: `tasks_dir`, `goals_dir`, `themes_dir`, `objectives_dir`, `vision_dir`, `daily_dir`, `session_project_dir`, `claude_script`, `excludes`, `task_template`, `goal_template`, `theme_template`, `objective_template`, `vision_template`
- [ ] JSON entry for `minimal` contains only: `path`, `name`

### Single-vault filter
- [ ] `$VAULT_CLI --config $CONFIG config list --vault full --output json` returns only the `full` vault entry
- [ ] `$VAULT_CLI --config $CONFIG config list --vault minimal --output json` returns only the `minimal` vault entry

## Expected

- [ ] All 16 documented `Vault` fields appear in JSON output for `full` vault
- [ ] All omitempty fields absent from JSON output for `minimal` vault
- [ ] All template paths resolved (relative → vault-relative, ~ → home, absolute → unchanged)
- [ ] Single-vault filter returns matching vault only

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
