---
description: Update current task with completed work and sync to daily note / parent goal — delegates to task-manager-agent
allowed-tools: Task
argument-hint: (detects from conversation)
---

Update task progress: detect task from the current conversation, mark completed checkboxes, sync changes to the daily note and parent goal when the progress is noteworthy.

## Process

1. **Validate input** (none expected — agent auto-detects from conversation context).

2. **Invoke task-manager-agent**:
   ```
   Task tool with:
     subagent_type: 'vault-cli:task-manager-agent'
     prompt: 'ACTION: update\nARGS: (none — detect task from conversation context)'
   ```

3. **Done.**

The agent handles:
- Detect task from conversation (file paths, wiki-links, mentions; ranked by confidence)
- Read current checkbox state from the task file (`[x]` / `[/]` / `[ ]`)
- Analyze the conversation for completed work (files created/modified, commands run, problems solved)
- Tick completed checkboxes via `vault-cli task update` (NOT direct Edit on frontmatter)
- Determine if the progress is noteworthy (100% complete, major milestone, >20% jump)
- If noteworthy: append a progress entry to today's daily note
- If 100% complete: invoke `vault-cli task complete` to finalize
- Return a structured report (progress %, items completed, next pending)

## Notes

- The vault is detected via `vault-cli config list` and `$PWD`; folder names (`tasks_dir`, `daily_dir`, `goals_dir`) come from the matched vault config
- No hardcoded paths or vault assumptions — works in any vault registered with `vault-cli config`
- Sibling commands: `/vault-cli:update-goal`, `/vault-cli:complete-task`, `/vault-cli:sync-progress`
