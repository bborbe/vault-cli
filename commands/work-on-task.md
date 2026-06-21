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

The agent (dispatched in `## Process` step 2) emits a structured `not_found` verdict from its own Phase 1 (`Find task`) when the requested task cannot be found in any source. This phase parses that verdict and asks the user before any file is created.

1. **Parse the agent's report** for the `not_found:` marker AND capture the verdict body into variables. The agent's `<output_format>` defines two separate fenced markdown blocks — one for the `found` case (ends with `Ready to work on this task.`) and one for the `not_found:` case (literal `not_found:` header on its own line). Look for the `not_found:` block specifically; if the report ends with `Ready to work on this task.` and contains no `not_found:` block, Phase 4 is a no-op and you are done. When the `not_found:` block IS present, match on the `not_found:` token, then extract:
   - `SEARCHED_BLOCK` — the bullet list under the `Searched:` line (verbatim, line-by-line, until the next blank line or `Suggested task name:` line)
   - `SUGGESTED_NAME` — the value after `Suggested task name:` (verbatim, trimmed)
2. **Use `SUGGESTED_NAME` as the seed.** (If the input was a Jira ID and the Jira lookup returned a summary, the agent supplied that summary; otherwise the agent supplied the input string verbatim. Either way, `SUGGESTED_NAME` is what you pass on.)
3. **Ask the user via `AskUserQuestion`** with the `vault-cli` main-session UX channel:
   - `header`: `Create new task?`
   - `question`: `Create new Obsidian task "<SUGGESTED_NAME>"?` (substitute the captured value)
   - `options`: two entries — `Yes, create it` (description: `Run vault-cli:create-task with "<SUGGESTED_NAME>" as the seed title, then re-invoke work-on-task-assistant`) and `No, stop here` (description: `Print manual search tips and stop — no task is created`)
4. **On `Yes, create it`**: invoke `Skill: vault-cli:create-task "<SUGGESTED_NAME>"` (use the same argument form as `commands/create-task.md` — pass the captured suggested name as a quoted argument). The create-task skill has its own interactive flow that asks for parent goal, priority, category, defer date, etc. — do not duplicate those asks.
5. **On create success** (create-task skill returns the new task file path or reports success): re-invoke `Task tool with subagent_type: 'vault-cli:work-on-task-assistant' prompt: 'Find details and guides for: <new task title>'` — same form as the Phase 2 invocation, but with the new task title. The agent's standard Phase 2–8 prep mutations then run against the just-created task.
6. **On create failure or user cancel inside `vault-cli:create-task`** (the skill returns a non-success status, errors out, or the user aborts midway through its interactive prompts): print `❌ Task creation failed or was cancelled. No task created; no follow-up invocation.` and STOP — do NOT re-invoke `vault-cli:work-on-task-assistant`, do NOT retry the create.
7. **On `No, stop here`**: print the manual search tips and STOP. Substitute `SEARCHED_BLOCK` (captured in step 1) where indicated; resolve `{daily_dir}` from `vault-cli config list --output json` for the active vault before printing:
   ```
   ❌ Task not found: "<input>"

   Searched:
   <SEARCHED_BLOCK>

   Manual search tips:
   - Check the active vault's tasks dir (`vault-cli config list` → `tasks_dir`)
   - Grep across vaults: `grep -rln "<keyword>" ~/Documents/Obsidian/`
   - Check today's daily note (`<resolved daily_dir>/YYYY-MM-DD.md`)
   - If input looked like a Jira ID, confirm the issue exists in the Atlassian project

   No task was created.
   ```

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. **`/vault-cli:work-on-task`** — orient (status + guides + daily note) — this command
3. `/vault-cli:plan-task` — sharpen (5 hard gates; may flip phase if entry contract permits)
4. `/vault-cli:execute-task` — gate planning → execution; flips phase + prints first subtask + DoD reminder
5. Start work — while working, use any of:
   - `/vault-cli:update-task` — log completed work, sync to daily note / parent goal
   - `/vault-cli:task-status` — grouped-checkbox status (Success Criteria / Tasks / DoD) + next step
   - `/vault-cli:next-steps` — next actionable steps; offer defer if nothing left today
6. `/vault-cli:sync-progress` — flush conversation to daily note + task pages
7. `/vault-cli:complete-task` — close task
8. `/vault-cli:session-close` — verify session is safe to end (synced, committed, no orphaned state)

`work-on-task` is content-agnostic by design — it sets status and finds guides, it does not question your task content. `plan-task` is the dedicated tool for sharpening substance, and `execute-task` is the gate that flips the phase transition to `execution`.

## Notes

- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- Works in Personal, Brogrammers, Trading, or any future vault registered with `vault-cli config`
- Each vault session loads a single Atlassian MCP under the canonical name `atlassian` (see vault-specific `mcp-*.json` configs); the agent uses `mcp__atlassian__*` regardless of which Jira instance is active
- The agent searches; the slash command asks before creating.
