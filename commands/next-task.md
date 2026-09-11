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

Blocked state comes from `vault-cli task list --all --output json` (`blocked` / `blocked_by`), never from task content.

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
- **In Progress**: `status == in_progress` OR daily-note `[/]`, and not filtered by Step 5
- **Blocked**: `status == hold` (parked — still listed by name)
- **Pending**: `status in (next, todo)` and not filtered by Step 5

A task whose Step 5 `blocked` flag is true is removed from the candidate set entirely: it is not recommended, not listed by name, and not counted in any of the three groups above. Report it only as a count (Step 7).

### Step 5: Detect blocked tasks (JSON)

```bash
vault-cli --vault <name> task list --all --output json
```

Run once per vault in scope (the active vault first, then sibling vaults). Build a lookup from each item's `name` to its `blocked` and `blocked_by` fields.

- `"blocked": true` → the task is blocked; its `blocked_by` array names the unmet dependencies.
- `"blocked": false`, or the `blocked` key absent → the task is not blocked. The key is absent when the task declares no `blocked_by` list.
- A task that appears in no vault's output → treat it as not blocked (no declared dependency was found).

The JSON field is the single source of truth for blocked state. Never derive blocked state from task content — the body-marker heuristic this section replaced is gone and must not be reintroduced. Do not restate the removed patterns here: naming them invites a future reader to re-implement them.

### Step 6: Pick recommended task

Priority cascade (stop at first match):
1. Single in-progress task → that one
2. Multiple in-progress → first by `priority` then alphabetical
3. Pending with `priority: 1` → first one
4. Any pending → first by daily-note order
5. Every candidate was filtered as blocked (candidate set empty) → recommend nothing: state that every candidate is blocked and name the unmet dependency (from `blocked_by`) to resolve. Do not name the blocked task itself.

### Step 7: Present worker-mode output

```markdown
📋 Today's Tasks: <date>

In Progress (n):
→ [[Task]] (priority p, ~est) — recommended | continue

Pending (n):
○ [[Task]]
○ [[Task]]

Blocked (n):
⚠️ [[Task]] — status: hold, waiting
⛔ <N> hidden — unmet blocked_by (see `vault-cli task list --all --output json`)

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

Group as in Step 4-5 (in-progress / blocked / pending); the Step 5 filter applies to task children and to goal children alike. For goal children (a THEME argument), read blocked state from `vault-cli --vault <name> goal list --all --output json` using the same `blocked` / `blocked_by` fields. For task children, use the Step 5 task listing. Subtask checkbox lines parsed from a TASK's content have no JSON entity behind them and are listed unchanged.

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
⚠️ <child> — status: hold, waiting
⛔ <N> hidden — unmet blocked_by (see `vault-cli task list --all --output json`)

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
- Never name a task or child whose `blocked` flag is true — report the count only. `status: hold` items without a `blocked_by` list keep their named listing.
- Sort by priority then alphabetical
- Max 5 items per group; if more, append `... and N more`
