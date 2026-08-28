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
cp -R ~/Documents/workspaces/vault-cli/example/. "$WORK_DIR/"  # /. trailing-dot is portable across BSD + GNU cp
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

> Spawns a real headless `claude --print` turn. **Both branches block until the turn completes** — expect no output for the whole bootstrap (typically 2-5 minutes; bounded by a 30m turn timeout), then `✅ Now working on: …` and `session_id: …`. A fast return is a FAIL, not a pass: the session id is written only once the turn has finished, which is what makes `claude --resume <id>` work. Allow ≥300s. Run the session-lifecycle check in Expected below.

### Defer the task
- [ ] `$VAULT_CLI --config $CONFIG task defer "Simple Task" +1d` exits 0

### Complete the task
- [ ] `$VAULT_CLI --config $CONFIG task complete "Simple Task"` exits 0

## Expected

- [ ] `grep "status: completed" "$TASK_FILE"` succeeds
- [ ] `grep "assignee: alice" "$TASK_FILE"` succeeds
- [ ] `grep -E "^defer_date: \"?$TOMORROW\"?\$" "$TASK_FILE"` succeeds (YAML may quote the date)
- [ ] `claude_session_id` in the task file is non-empty, and a transcript file named `<that id>.jsonl` exists under `~/.claude/projects/<encoded-cwd>/`. Resolve `<encoded-cwd>` the way the installed `claude` client does for the directory `work-on` was invoked in; the exact shell is agent-decided. This is the check that catches a `claude` build which accepts `--session-id` but silently mints its own (spec 040 Failure Modes row 7).

```bash
# The bootstrap claude runs with the vault dir as its cwd (vault.Path; no
# session_project_dir is configured), so its transcript is keyed on that dir.
# claude encodes the cwd for ~/.claude/projects/<encoded-cwd>/ by replacing path
# separators and dots with '-'. The find fallback covers any encoding drift so the
# check stays runnable, and still looks for the <uuid>.jsonl under ~/.claude/projects.
SESSION_ID=$(awk -F': ' '/^claude_session_id:/{gsub(/"/, "", $2); print $2; exit}' "$TASK_FILE")
ENCODED_CWD=$(printf '%s' "$WORK_DIR/vault" | sed 's|^/||; s|/|-|g; s|\.|-|g' | tr -c 'A-Za-z0-9_-' '-')
TRANSCRIPT="$HOME/.claude/projects/-${ENCODED_CWD}/${SESSION_ID}.jsonl"
if [ ! -f "$TRANSCRIPT" ]; then
  TRANSCRIPT=$(find "$HOME/.claude/projects" -name "${SESSION_ID}.jsonl" 2>/dev/null | head -1)
fi
test -n "$SESSION_ID" && test -n "$TRANSCRIPT" && echo "transcript: $TRANSCRIPT"
```

## Cleanup

```bash
rm -rf "$WORK_DIR"
```
