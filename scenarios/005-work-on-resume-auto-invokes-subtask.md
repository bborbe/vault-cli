---
status: draft
---

# Scenario 005: work-on resume auto-invokes a bare slash-command subtask

Validates that `task work-on` resumes its headless session interactively and invokes a first subtask that is a bare allowlisted slash-command call, with no approval turn.

## Why a scenario

The argv half of this behavior *is* unit-covered (`pkg/ops/claude_resume_test.go`). What no test can reach is the other half: the classification rule lives in `commands/execute-task.md`, a Claude Code markdown command with no test harness, and it executes only behind `term.IsTerminal` → `syscall.Exec` → a live Claude session interpreting that markdown. Real TTY, real process replacement, real LLM.

Two consequences for whoever walks this:

- **Must be a real interactive terminal.** Under any non-TTY caller — CI, an agent shell, a pipe — `Execute` skips the resume branch and this scenario silently tests nothing while still exiting 0.
- **Re-walk after every plugin bump, not just every binary bump.** `commands/execute-task.md` ships on a channel `vault-cli --version` says nothing about.

**This scenario is not sandboxed, by necessity.** `StartSession` and `ResumeSession` pass only cwd and `os.Environ()`; the spawned session uses the **installed** `vault-cli` from PATH and the **default** config, not a `--config` flag. A temp-vault sandbox would be fiction — the session half would read the real vault anyway. So it runs against the live vault with a clearly-named throwaway fixture. It mutates: the fixture file (`status`, `phase`, `claude_session_id`) and today's daily note (adds a tracking checkbox). Both are cleaned up below.

## Setup

```bash
VAULT_NAME=personal   # pinned explicitly; config list exposes no default marker
VAULT=$(vault-cli config list --output json | python3 -c "import json,sys; print(next(v['path'] for v in json.load(sys.stdin) if v['name']=='$VAULT_NAME'))")
FIXTURE="$VAULT/24 Tasks/Scenario 005 Fixture.md"
PLUGIN_VER=$(claude plugin list | grep -A1 'vault-cli@vault-cli' | awk '/Version/{print $2}')

cat > "$FIXTURE" <<'EOF'
---
page_type: task
status: in_progress
phase: execution
priority: 2
---
Tags: [[Task]]

---
Throwaway fixture for scenario 005. Delete after the walk.

# Success Criteria

- [ ] The subtask below is invoked, not printed.
- [ ] No approval turn occurs between resume and invocation.

# Tasks

- [ ] Run `/vault-cli:next-task`

# Definition of Done

- [ ] Invoked with no approval turn.
EOF
echo "FIXTURE=$FIXTURE"   # note this path; you need it after the session exits
```

- [ ] `test -t 0 && echo TTY` prints `TTY`
- [ ] `command -v claude` resolves — otherwise `NewClaudeSessionStarter` returns nil, `work-on` downgrades to a warning and exits 0 with no session at all
- [ ] `grep -c 'claude_session_id' "$FIXTURE"` returns `0`
- [ ] `grep -c '^### Subtask classification' ~/.claude/plugins/cache/vault-cli/vault-cli/$PLUGIN_VER/commands/execute-task.md` returns `1` — resolve the path from the **installed** version, never from the latest git tag. `autoRelease` tags on every merge, so the tag routinely runs ahead of what is installed while `commands/execute-task.md` is unchanged; keying on the tag fails a walk that would have passed. Note the path is doubly nested (`cache/vault-cli/vault-cli/`) — a singly-nested guess silently returns 0 and reads as a stale plugin
- [ ] The vault directory is already trusted by Claude Code — otherwise the first-run "Do you trust the files in this folder?" gate is itself an approval turn and breaks the assertion below

## Action

- [ ] Run `vault-cli --vault "$VAULT_NAME" task work-on "Scenario 005 Fixture"` — `syscall.Exec` replaces the **vault-cli process** (the parent shell survives) with an interactive `claude` session
- [ ] Note where the replayed turn-1 transcript ends and **new** output begins — `claude --resume` reprints the prior turn, whose tail legitimately contains `✅ Oriented: … Next: →`. Every assertion below applies **only to output produced after the continuation prompt is submitted**
- [ ] Exit the session

## Expected

Scoped to post-continuation output only:

- [ ] Contains `🚀 Running: /vault-cli:next-task`
- [ ] Contains `📋 Today's Tasks:` — next-task's own worker-mode header, proving the command actually ran rather than just being announced
- [ ] Does **not** contain `🎯 Start with:` — negative evidence; its presence means classification fell through to print
- [ ] Does **not** contain a fresh `✅ Oriented:` / `Next: →` block — negative evidence; that is the original bug reproducing
- [ ] No `AskUserQuestion` option list and no permission prompt appears between the resume and the `🚀 Running:` line
- [ ] `grep -c 'claude_session_id' "$FIXTURE"` now returns `1` — durable on-disk proof the bootstrap ran, and the one assertion that survives losing the scrollback

## Cleanup

```bash
rm -f "$FIXTURE"
# remove the fixture's tracking checkbox from today's daily note
grep -n 'Scenario 005 Fixture' "$VAULT/60 Periodic Notes/Daily/$(date +%F).md"
```
