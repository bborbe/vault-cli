---
description: Find task details, transition Jira, set status, track on daily note, discover guides — delegates to work-on-task-assistant
argument-hint: <jira-id-or-text>
allowed-tools: Task
---

Find task details and relevant operational guides before starting work. Delegates to the `vault-cli:work-on-task-assistant` agent (which is the heavy lifter).

## Usage

```bash
/vault-cli:work-on-task TRADE-1234           # any Jira-style ID
/vault-cli:work-on-task BRO-456              # works with any project key
/vault-cli:work-on-task "check kafka backups" # free text
```

## Process

1. **Validate input**
   - If no argument: `❌ Pass a task identifier or description.` and STOP

2. **Invoke work-on-task-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-task-assistant'
     prompt: 'Find details and guides for: {arguments}'
   ```

3. **Done**

The assistant handles all the work, detecting available integrations at runtime:

- **Jira (if `mcp__atlassian` MCP is available)**: fetch issue, auto-assign to current user, auto-transition to "In Progress". Cloud ID auto-detected via `getAccessibleAtlassianResources` — no hardcoded host.
- **Jira (if MCP absent)**: fall back to free-text search on the ID string in vault files. No error.
- **Obsidian task**: find by name or by `jira:` frontmatter; set status to `in_progress`; offer to create local file if missing
- **Daily note**: track with `[/]` checkbox in the Must section; report gracefully if note missing
- **Code tasks**: run `/coding:check-guides` and read project Development Guide if present
- **Guides (semantic search if available)**: search runbooks, operational guides, related docs; fall back to `Glob` if semantic search MCP absent

Output ends with `Ready to work on this task.`

## Integration

Task-first workflow:
1. `/vault-cli:next-task` → pick task
2. `/vault-cli:work-on-task <id>` → guides + context (this command)
3. Start work
4. `/vault-cli:sync-progress` → log progress when done

**If the task isn't ready** (unclear DoD, vague subtasks, scope creep, goal-orphan): run `/vault-cli:refine-task <id>` first. `work-on-task` is content-agnostic by design — it sets status and finds guides, it does not question your task content. `refine-task` is the dedicated tool for sharpening substance.

## Notes

- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- Works in Personal, Brogrammers, Trading, or any future vault registered with `vault-cli config`
- Each vault session loads a single Atlassian MCP under the canonical name `atlassian` (see vault-specific `mcp-*.json` configs); the agent uses `mcp__atlassian__*` regardless of which Jira instance is active
