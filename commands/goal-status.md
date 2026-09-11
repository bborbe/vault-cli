---
description: Show status of current goal with progress and next task, always leading with the Async State Closer anchor pair (clickable goal + next-task links).
argument-hint: (detects from conversation)
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash(vault-cli:*)
  - Bash(grep:*)
  - Bash(command -v:*)
  - Task
---

Quick "where am I?" recovery tool. Detects the active goal from the parent conversation, emits the Async State Closer anchor pair (clickable `🎯 Goal:` + `📌 Task:` lines), then delegates the aggregate status body to `goal-manager-agent`.

**This command must stay inline** — Phase 1 (goal detection) and Phase 2 (anchor pair) analyze the parent conversation and the current session; a sub-agent cannot see either. Only the status body (Phase 3) delegates.

## Phase 1: Detect active goal

Inline. If the command was invoked with a goal argument, use it directly. Otherwise scan the parent conversation in priority order:

1. Most recent `/vault-cli:work-on-goal` / `/vault-cli:execute-goal` / `/vault-cli:launch-goal` output → use that goal name.
2. Most recent `[[Goal Name]]` wikilink referenced as a goal subject (not generic prose mention).
3. Daily note's first `[/]` checkbox's linked goal.

Resolve the detected name via `Glob` `<goals_dir>/*<arg>*.md` (fallback `<goals_dir>` = `23 Goals/`, then `22 Goals/` for compatibility). Multiple matches → list candidates, ask via `AskUserQuestion`. Zero → `❌ No active goal detected. Pass a goal name or path.` STOP.

Print `Detected goal: <name>` on first line so the owner can interrupt if wrong before Phase 3 runs, followed by the always-shown Async State Closer anchor pair:

```
🎯 Goal: [<goal name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <n>/<m> SC · <n>/<m> subtasks · binding: <value>
📌 Task: [<next open task>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <phase>, session <id8> (this one) · ⚠️ also claimed by peer <id8>
```

**The anchor pair is emitted on EVERY run** — on the `🔜 Next:` / complete / `❌` branches alike — so the operator can always open the goal and its next task from the status output. It sits above the agent's body, one blank line after.

## Phase 2: Build the anchor pair

Build it inline per `docs/output-formatting.md` § Anchor pair (link rule, session suffix, peer claim, counts — same recipe as `task-status`). Goal-specific pieces:

**Vault identity** — obsidian vault name = basename of the matching vault's `path` from `vault-cli config list --output json` (NOT the lowercase config `name`):

```bash
read -r VAULT_NAME VAULT_PATH GOALS_DIR TASKS_DIR <<< "$(vault-cli config list --output json | python3 -c "
import sys, json, os
vs = json.load(sys.stdin); cwd = os.getcwd()
v = next((x for x in vs if cwd.startswith(x['path'])), vs[0])
print(v['path'].rstrip('/').split('/')[-1], v['path'], v.get('goals_dir','23 Goals'), v.get('tasks_dir','24 Tasks'))")"
```

**Next-task resolution** — walk the goal's `# Tasks` list items **in listed order** (same rule as `execute-goal.md` step 7): each item's task is its **leading `[[...]]` only** (`|alias` stripped). For each task resolve to `<TASKS_DIR>/<Task Title>.md` and read its status via `vault-cli task get "<title>" status --output json`. **Next open task** = the first *resolving* task whose status is NOT `completed` and NOT `aborted`:

- No task wikilinks at all under `# Tasks` → `📌 Task: none — no tasks under # Tasks`.
- No open task (all complete / only aborted remain) → `📌 Task: none — all tasks complete`.
- Otherwise the `📌` line uses that task's file: link, its `phase` (`vault-cli task get "<title>" phase --output json`), and its session/peer suffix (against the TASK's own `claude_session_id` / `metrics_sessions` — see `docs/output-formatting.md` § Anchor pair).

**Counts + binding** — goal SC + subtask counts and the optional `binding:` frontmatter segment per `docs/output-formatting.md` § Anchor pair.

## Phase 3: Generate the status body

Delegate to `goal-manager-agent` — pass the resolved goal explicitly (sub-agents cannot detect from conversation):

```
Task tool with:
  subagent_type: 'vault-cli:goal-manager-agent'
  prompt: 'ACTION: status
           ARGS: <resolved-goal-name-from-phase-1>
           MODE: interactive'
```

The agent renders its aggregate body unchanged (`Status:` / `Criteria:` / `Subtasks:` / `🔜 Next:`).

## Output shape

The final output ALWAYS leads with the anchor pair from Phase 2, then the agent's body:

```
🎯 Goal: [<name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <n>/<m> SC · <n>/<m> subtasks · binding: <value>
📌 Task: [<name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <phase>, session <id8> (this one) · ⚠️ also claimed by peer <id8>

🎯 Goal: <goal_name>
Status: <status>
Criteria: <completed>/<total> (<pct>%)
Subtasks: <completed>/<total> (<pct>%)
🔜 Next: <next_step>
```

## Output ends with one of

- `🔜 Next: <next step>` (work remaining)
- `🎉 Ready to complete!` (agent completion branch)
- `❌ No active goal detected. Pass a goal name or path.` (Phase 1 zero-match)
