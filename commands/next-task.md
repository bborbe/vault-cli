---
description: Suggest next task — from daily note (worker mode) OR next items under a goal/task/theme (boss mode)
allowed-tools:
  - Read
  - Bash
  - Edit
  - Grep
  - Glob
  - AskUserQuestion
argument-hint: "[goal|task|theme name]"
---

Two modes:
- **Worker mode** (no args): next task from today's daily note
- **Boss mode** (with arg): next items under a goal, task, or theme

## Runtime context

```bash
vault-cli config list --output json
```

Match cwd to a vault entry, then use `tasks_dir`, `goals_dir`, `themes_dir`, `daily_dir` from that entry. For cross-vault discovery, iterate every entry under `~/Documents/Obsidian/`.

Date anchor:
```bash
date "+%Y-%m-%d"
```

## Step 1: Detect mode

- No args / empty → **Worker mode** (Step 2 onwards)
- Args present → **Boss mode** (Step 10 onwards)

---

## Worker mode

### Step 2: Read daily note

`{daily_dir}/YYYY-MM-DD.md`. If missing: `❌ Daily note missing. Run /start-day.`

### Step 3: Collect candidate tasks from daily note

Parse `## Must` / `## Should` / `## Could` sections. Collect lines:
- `- [/] [[Task]]` — in progress
- `- [ ] [[Task]]` — pending
- Skip `- [x]` (completed)

For each `[[Task]]`, resolve to a task file (active vault first, then sibling vaults).

### Step 4: Filter and group

For each resolved task:
- Read frontmatter: `status`, `defer_date`, `priority`
- Skip if `defer_date > today`
- Skip if `status == completed` or `status == aborted`

Accept any of: `status == in_progress`, `status == next`, `status == todo` (legacy alias), `status == hold`.

Group:
- **In Progress**: `status == in_progress` OR daily-note `[/]`
- **Blocked**: `status == hold` OR file content contains blocker refs (see Step 5)
- **Pending**: `status in (next, todo)` and not blocked

### Step 5: Detect blockers

Scan each task's content for:
- `**Blocker:** [[Task]]`
- `Blocked by: [[Task]]`
- `⚠️ Blocked by: [[Task]]`
- `- [ ] [[Task]]` lines under a `Prerequisites` section

For each blocker, resolve to a task file. If found and `status != completed`, it's an active blocker.

### Step 6: Pick recommended task

Priority cascade (stop at first match):
1. Single in-progress task → that one
2. Multiple in-progress → first by `priority` then alphabetical
3. Pending with `priority: 1` → first one
4. Any unblocked pending → first by daily-note order
5. Only blocked tasks → first blocker to resolve

### Step 7: Present worker-mode output

```markdown
📋 Today's Tasks: <date>

In Progress (n):
→ [[Task]] (priority p, ~est) — recommended | continue

Pending (n):
○ [[Task]]
○ [[Task]]

Blocked (n):
⚠️ [[Task]] — blocked by [[Blocker]] (<status>)

🎯 Recommended: [[Task]]
Why: <rationale>
```

### Step 8: AskUserQuestion

- header: `Start work`
- question: `Start on [[Task]]?`
- options: `Yes` | `Pick different task` | `Defer task`

### Step 9: Handle response

- `Yes` → run `Skill: vault-cli:work-on-task` with the task name
- `Pick different task` → ask for selection (1-N) then `vault-cli:work-on-task`
- `Defer task` → `Skill: vault-cli:defer-task` with task + tomorrow's date

---

## Boss mode

### Step 10: Resolve item

Argument is a name. Search active + sibling vaults in order:
1. `{themes_dir}/*{name}*.md` → THEME
2. `{goals_dir}/*{name}*.md` → GOAL
3. `{tasks_dir}/*{name}*.md` → TASK

If not found: `❌ Not found: "<name>". Searched themes/goals/tasks across vaults.` and STOP.

### Step 11: List children

**THEME** → find goals: `Grep: 'themes:.*\[\[<theme>\]\]'` in `{goals_dir}` and sibling vaults

**GOAL** → find tasks: `Grep: 'goals:.*\[\[<goal>\]\]'` in `{tasks_dir}` and sibling vaults. Also parse the goal file's `Active Tasks` / `Sub-Tasks` sections for explicit links.

**TASK** → list subtasks: parse `[ ]` / `[/]` / `[x]` lines from content.

### Step 12: Filter children

For goals/tasks (not subtasks):
- Skip `status in (completed, aborted)`
- Skip `defer_date > today`
- Accept `status in (in_progress, next, todo, hold)`

Group as in Step 4-5 (in-progress / blocked / pending).

### Step 13: Present boss-mode output

```markdown
📋 NEXT: <Item Name> (<type>)
Status: <status>

[If children:]
In Progress (n):
→ <child>
Pending (n):
○ <child>
Blocked (n):
⚠️ <child> — blocked by ...

🎯 Recommended: <child>
Why: <rationale>
```

For TASK with subtasks: show next 3 pending subtask lines.

### Step 14: AskUserQuestion

- header: `Work on child`
- question: `Work on <child>?`
- options: `Yes` | `Pick different` | `Show another level deeper`

---

## Jira detection (graceful)

If task content references a Jira-style ID matching `[A-Z]+-\d+` (e.g. `TRADE-123`, `BRO-456`), and `mcp__atlassian__getJiraIssue` is available in the session:
- Decorate the task in the output with the Jira status (single getJiraIssue call)
- Otherwise display the bare ID without decoration

The recommended-task path then routes through `/vault-cli:work-on-task` which handles full Jira lookup gracefully.

## Output rules

- Use vault-relative paths in display; absolute paths only when crossing vaults
- Wikilinks preferred over filenames
- Hide deferred tasks but show count
- Sort by priority then alphabetical
- Max 5 items per group; if more, append `... and N more`
