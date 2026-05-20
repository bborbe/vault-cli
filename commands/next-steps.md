---
description: Show next actionable steps for current task; offer to defer if nothing left today
allowed-tools:
  - Read
  - Grep
  - Glob
  - Bash
  - AskUserQuestion
argument-hint: (detects from conversation, or pass task name)
---

Show next actionable steps for the current task. If nothing remains for today, offer to defer.

## Step 0: Anchor on real date

```bash
date "+%Y-%m-%d %H:%M:%S %Z"
```

Use the `YYYY-MM-DD` field for every "today" / "tomorrow" reference. Long sessions can cross midnight; the session-start date goes stale.

## Step 1: Detect current task

Find task from:
1. `$ARGUMENTS`
2. Conversation context — `[[Task Name]]` wikilinks or file paths
3. Recent `/vault-cli:work-on-task` / `/vault-cli:task-status` mentions

Search vault: read `vault-cli config list --output json`, get `tasks_dir` and `goals_dir` for the active vault, then:
- `{tasks_dir}/{name}.md`
- `{goals_dir}/{name}.md` (for task-like goals)

If not found: `❌ No task detected. Pass a task name or work on a task first.`

## Step 2: Extract remaining steps

Read the task file. Collect:
1. `- [ ]` unchecked items
2. `- [/]` in-progress items
3. Items under `## Next Steps` / `## Remaining`

Skip:
- Items with future defer dates
- Items marked blocked
- Subtask headers (contain other checkboxes)

## Step 3: Filter to today's scope

Keep items actionable today. Take the first 3-5.

## Step 4: Present results

**All steps complete (no unchecked items):**

Call `Skill: vault-cli:complete-task` with the task name. The complete command handles normal vs recurring tasks and updates goals/daily notes.

**Steps remain for today:**

```markdown
## Next Steps for [[{Task Name}]]

1. {First step} (recommended)
2. {Second step}
3. {Third step}
```

Then AskUserQuestion:
- header: `Next step`
- question: `Continue with the recommended step?`
- options: `Yes - Start step 1` | `Pick different step` | `Defer entire task`

**Unchecked items exist but none actionable today:**

```markdown
## [[{Task Name}]] — Nothing actionable today

Remaining items scheduled for future or blocked.
```

Compute tomorrow's date (YYYY-MM-DD).

AskUserQuestion:
- header: `Defer task`
- question: `Defer [[{Task Name}]] until {tomorrow}?`
- options: `Yes - Defer to {date}` | `No - Keep on today's list`

## Step 5: Handle response

- `Yes - Start step 1` → display step details, suggest how to proceed
- `Pick different step` → show numbered list, ask which number
- `Defer entire task` or `Yes - Defer` → `Skill: vault-cli:defer-task` with task + date
- `Keep on today's list` → acknowledge, no action

## Output format

Keep concise. Task name as header with wikilink. Numbered step list. First marked `(recommended)`. Clear AskUserQuestion.
