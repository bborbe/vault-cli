---
description: Sync conversation progress to disk, then show grouped-checkbox task status (Success Criteria / Tasks / Definition of Done) with a phase/plan assessment, verbatim state, and next step.
argument-hint: (detects from conversation)
allowed-tools:
  - Read
  - Edit
  - Grep
  - Glob
  - Bash(vault-cli:*)
  - Bash(grep:*)
  - Bash(command -v:*)
  - Task
---

Quick "where was I?" recovery tool. Detects active task from the parent conversation, syncs any in-flight progress to disk first (via `/vault-cli:sync-progress` logic, inline), re-evaluates the task's phase and plan state against its `# Success Criteria` / `# Tasks` sections, then emits a grouped-checkbox status report.

**Important side-effect:** this command mutates the vault (daily note + task page) before reporting. The mutation reflects work the conversation has already done — not new content. If you want a pure read without disk writes, use `/vault-cli:verify-task` instead.

**This command must stay inline** — Phase 1 (sync), Phase 2 (task detection), and Phase 2.5 (phase/plan re-evaluation) all analyze the parent conversation; a sub-agent cannot see the conversation. Only the final output formatting (Phase 3) delegates to `task-manager-agent`.

## Phase 1: Sync progress from conversation

Invoke the skill. Literally this call, not an inlined equivalent:

```
Skill: vault-cli:sync-progress
```

**"Run the sync-progress logic" is not a licence to reimplement it here.** Observed 2026-08-10: the agent hand-rolled the steps, checked the daily note directly, and skipped writing the session entry — the same misread that hit `/vault-cli:session-close` Phase 2 minutes earlier.

**`commands/sync-progress.md` is the single source of truth for what this does.** Deliberately not restated here: a copy of its phase list would drift the first time that command changes, and a stale copy is exactly what invites hand-rolling it again. Read the command if you need the detail.

Log its outcome in one line before continuing — `(sync: no-op — disk already fresh)` or `(sync: wrote {N} sections; {task} ticked {M} checkboxes)`.

This phase MUST run before Phase 3 — the status report reads disk, so disk must be fresh.

## Phase 2: Detect active task

Inline. Scan the parent conversation in priority order:

1. Most recent `/vault-cli:create-task` output → use that name.
2. Most recent `[[Task Name]]` wikilink referenced as a task subject (not generic prose mention).
3. Daily note's first `[/]` checkbox.
4. Most recently modified file in `<tasks_dir>/`.

Resolve the detected name via `Glob` `<tasks_dir>/*<arg>*.md`. Multiple matches → list candidates, ask via `AskUserQuestion`. Zero → `❌ No active task detected. Pass a task identifier or name.` STOP.

Print `Detected task: <name>` on first line so the owner can interrupt if wrong before Phase 3 runs, followed by the always-shown Async State Closer anchor pair:

```
🎯 Goal: [<goal name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <n>/<m> SC · <n>/<m> subtasks · binding: <value>
📌 Task: [<task name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <phase>, session <id8> (this one) · ⚠️ also claimed by peer <id8>
```

**The anchor pair is emitted on EVERY run** — on the `Next:` / `✅ Task complete` / `❌` branches alike — so the operator can always open the task and its parent goal from the status output. It sits above the assessment block, one blank line after. The `📌` line supersedes the former standalone `📎` task link; the always-emit guarantee is preserved by it.

### Building the anchor pair

Build it inline per `docs/output-formatting.md` § Anchor pair (link rule, session suffix, peer claim, counts — same recipe for `goal-status`). Task-specific pieces:

**Vault identity** — obsidian vault name = basename of the matching vault's `path` from `vault-cli config list --output json` (NOT the lowercase config `name`):

```bash
read -r VAULT_NAME VAULT_PATH GOALS_DIR <<< "$(vault-cli config list --output json | python3 -c "
import sys, json, os
vs = json.load(sys.stdin); cwd = os.getcwd()
v = next((x for x in vs if cwd.startswith(x['path'])), vs[0])
print(v['path'].rstrip('/').split('/')[-1], v['path'], v.get('goals_dir','23 Goals'))")"
```

**Goal resolution** — the task's `goals:` frontmatter, first entry, `|alias` stripped:

```bash
GOAL_TITLE="$(grep -m1 '^goals:' -A1 "$TASK_PATH" | grep -oE '\[\[[^]]*\]\]' | head -1 | sed 's/\[\[//; s/\]\]//; s/|.*//')"
GOAL_FILE="$VAULT_PATH/$GOALS_DIR/$GOAL_TITLE.md"
[ -f "$GOAL_FILE" ] || GOAL_FILE="$VAULT_PATH/22 Goals/$GOAL_TITLE.md"
```

- No `goals:` frontmatter → `🎯 Goal: (no goal linked)` (pair still emitted).
- Goal file missing → `🎯 Goal: <title> — (goal file missing)`.

**Session suffix + peer claim** — see `docs/output-formatting.md` § Anchor pair: `MINE` = `$CLAUDE_CODE_SESSION_ID` (kept only when the transcript `~/.claude/projects/<enc>/<MINE>.jsonl` exists), `IDS` = task's `claude_session_id` ∪ `metrics_sessions[].session_id`; `, session <MINE8> (this one)` always, ` · ⚠️ also claimed by peer <id8>` for each foreign id whose session is LIVE (<5-min transcript mtime or `claude --resume <id>` process).

## Phase 2.5: Re-evaluate phase & plan state

The phase/plan assessment is computed by `task-manager-agent` as part of its grouped report (see Output shape below) — this command does NOT inline the classification algorithm. The agent is the single parser for status, counts, and classification; the command only orchestrates detection and delegation.

**Recommend-only constraint.** The assessment classifies and recommends; it never mutates status, phase, or any checkbox. Per [[Task Lifecycle Guide]], manual phase-setting via `vault-cli task set` is a documented anti-pattern — `/vault-cli:execute-task` (planning → execution) and `/vault-cli:complete-task` (→ done) are the sole phase flippers, and `/plan-task` never flips itself. The agent's assessment step is read-only by contract.

## Phase 3: Generate grouped-checkbox status report

Delegate to `task-manager-agent`:

```
Task tool with:
  subagent_type: 'vault-cli:task-manager-agent'
  prompt: 'ACTION: status
           TASK_PATH: <resolved-path-from-phase-2>
           MODE: interactive
           OUTPUT: grouped-checkbox

           Read the task file (already disk-fresh after sync). Parse # Success Criteria,
           # Tasks, # Definition of Done sections. Compute the phase/plan assessment and
           emit grouped-checkbox output per the agent contract.'
```

The agent does NOT detect from conversation in this phase — Phase 2 already resolved the path. The agent only reads, parses, classifies, formats.

## Output shape (from task-manager-agent)

The final output ALWAYS leads with the Async State Closer anchor pair from Phase 2 (goal link + counts + binding; task link + phase + session/peer), then the agent's grouped report:

```
🎯 Goal: [<name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <n>/<m> SC · <n>/<m> subtasks · binding: <value>
📌 Task: [<name>](obsidian://open?vault=<vault>&file=<percent-encoded relpath>) — <phase>, session <id8> (this one) · ⚠️ also claimed by peer <id8>

Phase: <branch>
Plan: <validated · N/M subtasks · complete|not complete | not started (missing SC/Tasks)>
Recommend: <command | none — reason>

Task: <name>
Status: <status> · phase: <phase> · <completed>/<total> (<pct>%)
ℹ️ legend: ✅ done · ⏳ in-progress · ❌ pending   (only when ≥1 non-[x] item)

## Success Criteria
✅ <SC item text, truncated to ~80 chars>
❌ <SC item text>
⏳ <SC item text>

## Tasks
✅ <subtask>
❌ <subtask>

## Definition of Done
✅ <DoD item>
❌ <DoD item>

Next: <first unchecked item from SC, then Tasks, then DoD — one action>
```

If a section is absent in the task file, the agent omits the header (does NOT print an empty heading).

## Output ends with one of

- `Next: <first unchecked item>` (work remaining)
- `✅ Task complete. Run /complete-task to close.` (everything ticked)
- `❌ No active task detected. Pass a task identifier or name.` (Phase 2 zero-match)
