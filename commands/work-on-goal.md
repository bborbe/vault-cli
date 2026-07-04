---
description: Work on a goal — see context, pick task, get guides via work-on-goal-assistant, then signal the plan → execute → complete next steps
argument-hint: <goal-name>
allowed-tools: Task
---

Start working on a goal by seeing domain guides, progress, and task options.

## Usage

```bash
/vault-cli:work-on-goal "Goal Name"
```

The goal name is **required** — pass it as a quoted string. (Focus-page auto-detection is not part of this command; if you want a default-goal workflow, build a vault-side wrapper that resolves the name then calls this command.)

## Process

1. **Validate input**
   - If no argument or empty: `❌ Pass a goal name: /vault-cli:work-on-goal "Goal Name"` and STOP

2. **Invoke work-on-goal-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-goal-assistant'
     prompt: 'Find goal: {goal_name} and prepare work context'
   ```

3. **Signal next steps**

   After the assistant returns (ends with `Ready to work on this task.`), resolve the selected task name from its `📋 Task: <name>` line and print the next-step signal — identical to `commands/work-on-task.md` Phase 5:
   ```
   ✅ Oriented: <name>. Next:
   → /vault-cli:plan-task "<name>"     — validate the plan (Success Criteria + subtasks)
   → /vault-cli:execute-task "<name>"  — begin executing (flips planning → execution)
   → /vault-cli:complete-task "<name>" — close when done
   ```

   Do NOT auto-invoke `plan-task` or `execute-task` — the operator runs each deliberately.

The assistant returns:
- Goal summary and domain
- Domain-level operational guides (from semantic search or Glob fallback)
- Progress overview (`X/Y` completed, deferred count)
- In-progress / blocked / pending task lists
- Recommended task with rationale
- Task options to select
- After selection: delegates to `vault-cli:work-on-task-assistant`, returns combined context
- Ends with `Ready to work on this task.`

## Integration

Goal-first workflow:
1. Pick goal name (from your notes, focus page, etc.)
2. `/vault-cli:work-on-goal "<name>"` → context + task selection
3. Start work with full context

Sibling commands:
- `/vault-cli:next-task` — task-first workflow
- `/vault-cli:work-on-task <id>` — direct task prep
- `/vault-cli:goal-status` — goal progress only (no task delegation)
