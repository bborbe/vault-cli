---
description: Update goal with completed work, subtask checkboxes, management summary, and next-step suggestion — delegates to goal-manager-agent
allowed-tools: Task
argument-hint: (detects from conversation)
---

Update goal progress: auto-detect goal from the current conversation, refresh subtask checkbox state, update success criteria, refresh the management summary, calculate metrics, and suggest the next step.

## Process

1. **Validate input** (none expected — agent auto-detects from conversation context).

2. **Invoke goal-manager-agent**:
   ```
   Task tool with:
     subagent_type: 'vault-cli:goal-manager-agent'
     prompt: 'ACTION: update\nARGS: (none — detect goal from conversation context)'
   ```

3. **Done.**

The agent handles:
- Detect goal from conversation context (wiki-links, file paths, mentions)
- Read goal structure + subtask statuses (resolved against `tasks_dir` for the active vault)
- Analyze conversation for completed work
- Update subtask checkboxes
- Update success-criteria checkboxes when matched
- Determine if the change is noteworthy
- If noteworthy: append to `## Recent Progress` and refresh `## Current Status` / management summary
- If noteworthy: append a goal-level entry to today's daily note
- Calculate metrics (tasks remaining / completed, success-criteria coverage, time horizon)
- Suggest the next concrete step

## Notes

- Vault detection via `vault-cli config list` + `$PWD`; folders (`goals_dir`, `tasks_dir`, `daily_dir`) come from the matched vault config
- No hardcoded paths or vault assumptions
- Sibling commands: `/vault-cli:update-task`, `/vault-cli:complete-goal`, `/vault-cli:goal-status`, `/vault-cli:work-on-goal`
