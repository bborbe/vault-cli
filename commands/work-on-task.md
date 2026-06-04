---
description: Find task details, transition Jira, set status, track on daily note, discover guides — delegates to work-on-task-assistant
argument-hint: <jira-id-or-text>
allowed-tools: [Task, AskUserQuestion, Skill]
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

## Phase 4 — Handle not_found

The agent (Phase 2 of this command) emits a structured `not_found` verdict when the requested task cannot be found in any source. This phase parses that verdict and asks the user before any file is created.

1. **Parse the agent's report** for the `not_found:` marker. The agent's `<output_format>` produces a fenced markdown block with the literal `not_found:` header on its own line — match on that token.
2. **Derive a suggested task name.** The agent's verdict includes a `Suggested task name:` line — use that value verbatim as the seed for the create step. (If the input was a Jira ID and the Jira lookup returned a summary, the agent supplies that summary; otherwise the agent supplies the input string verbatim.)
3. **Ask the user via `AskUserQuestion`** with the `vault-cli` main-session UX channel:
   - `header`: `Create new task?`
   - `question`: `Create new Obsidian task "<suggested name>"?`
   - `options`: two entries — `Yes, create it` (description: `Run vault-cli:create-task with "<suggested name>" as the seed title, then re-invoke work-on-task-assistant`) and `No, stop here` (description: `Print manual search tips and stop — no task is created`)
4. **On `Yes, create it`**: invoke `Skill: vault-cli:create-task "<suggested name>"` (use the same argument form as `commands/create-task.md` — pass the suggested name as a quoted argument). The create-task skill has its own interactive flow that asks for parent goal, priority, category, defer date, etc. — do not duplicate those asks.
5. **On create success** (create-task skill returns the new task file path or reports success): re-invoke `Task tool with subagent_type: 'vault-cli:work-on-task-assistant' prompt: 'Find details and guides for: <new task title>'` — same form as the Phase 2 invocation, but with the new task title. The agent's standard Phase 2–8 prep mutations then run against the just-created task.
6. **On `No, stop here`**: print the manual search tips and STOP. The manual search tips are:
   ```
   ❌ Task not found: "<input>"

   Searched:
   - <echo the Searched: section from the agent's verdict>
   - <echo the Glob paths tried>

   Manual search tips:
   - Check the active vault's tasks dir (`vault-cli config list` → `tasks_dir`)
   - Grep across vaults: `grep -rln "<keyword>" ~/Documents/Obsidian/`
   - Check today's daily note (`{daily_dir}/YYYY-MM-DD.md`)
   - If input looked like a Jira ID, confirm the issue exists in the Atlassian project

   No task was created.
   ```

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
- The agent searches; the slash command asks before creating.
