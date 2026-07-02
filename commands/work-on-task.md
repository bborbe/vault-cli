---
description: Find task details, transition Jira, set status, track on daily note, discover guides, then auto-sharpen and gate the planning → execution transition
argument-hint: "<jira-id-or-text> [--non-interactive]"
allowed-tools: [Task, AskUserQuestion, Skill, Bash(vault-cli *)]
---

Find task details and relevant operational guides before starting work. Delegates to the `vault-cli:work-on-task-assistant` agent (which is the heavy lifter).

## Usage

```bash
/vault-cli:work-on-task TRADE-1234           # any Jira-style ID
/vault-cli:work-on-task BRO-456              # works with any project key
/vault-cli:work-on-task "check kafka backups" # free text
```

## Process

1. **Parse input**
   - Parse `$ARGUMENTS`: if it contains `--non-interactive` → set `MODE=non_interactive` and strip that flag token from the arguments; otherwise `MODE=interactive`. Parsing is self-contained here — it does not depend on any other command. Use the stripped arguments as the task identifier everywhere below — NEVER pass the flag token into the assistant prompt or task search.
   - If no argument remains after stripping: `❌ Pass a task identifier or description.` and STOP

   **`MODE=non_interactive`** exists for headless callers — e.g. `vault-cli work-on`'s Claude bootstrap runs `claude --print`, which cannot answer `AskUserQuestion`, so an interactive gate would block until the session-start timeout. In this mode the command NEVER calls `AskUserQuestion` and NEVER runs the interactive sharpening chain (Phase 5): it orients (via the assistant) and stops. Interactive sharpening resumes later when the session is opened in a real terminal.

2. **Invoke work-on-task-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-task-assistant'
     prompt: 'Find details and guides for: {stripped arguments}'
   ```

3. **Auto-sharpen + auto-gate (Phase 5).** If the assistant's report ends with `Ready to work on this task.` (the `found` case), continue to Phase 5 below. If the report contains the `not_found:` marker, skip Phase 5 and run Phase 4 (Handle not_found) instead.

4. **Done**

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

**Non-interactive gate (checked first):** If `MODE=non_interactive`, do NOT call `AskUserQuestion` and do NOT create anything. Print the `not_found:` report — the `Searched:` block from the verdict, then `❌ Task not found: "<input>"` — followed by `ℹ️ Non-interactive mode: no task created. Re-run in a terminal to create one.` and STOP. Skip steps 1–7 below (they are the interactive create-gate).

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

## Phase 5 — Auto-sharpen + auto-gate

Goal: by the time work-on-task returns, the resolved task is in `phase: planning` if its plan still has gaps, or `phase: execution` (with kickoff printed) if the plan already passes the gates. The owner sees an interactive sharpening loop only when the plan actually has gaps.

Runs only after Phase 2 returned a `found` task — never on `not_found` (Phase 4 handles that branch).

**Non-interactive gate (checked first):** If `MODE=non_interactive`, SKIP Phase 5 entirely — do NOT invoke `plan-task` or `execute-task`. Both own `AskUserQuestion` flows and would hang a headless caller. The assistant's orient (status, daily note, guides) already ran in `## Process` step 2 and IS the complete non-interactive result. Print `✅ Oriented (non-interactive). Run /vault-cli:plan-task in a terminal to sharpen and gate execution.` and STOP.

1. **Resolve the task name from the assistant's report.** The assistant prints `📋 Task: <name>` near the top of its `found` block; that line is the canonical identifier. Capture it verbatim.

2. **Invoke `Skill: vault-cli:plan-task` with the captured name as a quoted argument.** plan-task owns its own entry contract, gate logic, and `AskUserQuestion` flow — do not intercept. Wait for it to return.

3. **Read resulting phase** (after plan-task returns):
   ```bash
   vault-cli task get "<name>" phase --output json
   ```

4. **Conditional kickoff:**
   - If `phase: execution` → invoke `Skill: vault-cli:execute-task "<name>"`. execute-task is idempotent on `execution` (re-runs the 4 hard gates as a safety check, then prints `🎯 Start with: <first unchecked subtask>` + `📋 When done, verify: <DoD>`). This is the work-block kickoff the owner needs.
   - If `phase: planning` → STOP after plan-task. Print: `⏸️ Plan not yet ready — phase remains: planning. Re-run /vault-cli:plan-task when you have answers, or /vault-cli:execute-task to re-check the gate.` The owner is mid-conversation with plan-task or has deferred; never force execute-task on a task whose gates haven't passed.
   - If `phase: ai_review` / `human_review` / `done` → STOP. Print: `ℹ️ Phase already past planning (phase: <value>). No kickoff needed.\n→ If the work is genuinely done: run /vault-cli:sync-progress to flush conversation state, then /vault-cli:session-close.` The task is being shipped or has shipped; surfacing a "Start with" line here would be misleading, and the operator's most likely next step is closing the lifecycle (the sync-progress + session-close pair the lifecycle was designed around).

The Phase 5 chain is purely additive — it never overrides the assistant's report or plan-task's owner-question flow. Owner can always interrupt before any phase mutation.

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. **`/vault-cli:work-on-task`** — orient (status + guides + daily note) + auto-chain to plan-task and (when gates pass cleanly) execute-task — this command
3. `/vault-cli:plan-task` — sharpen (5 hard gates; may flip phase if entry contract permits); invoked automatically by Phase 5, or directly when re-entering sharpen mode
4. `/vault-cli:execute-task` — gate planning → execution; flips phase + prints first subtask + DoD reminder; invoked automatically by Phase 5 on a clean plan, or directly to re-surface the kickoff line
5. Start work — while working, use any of:
   - `/vault-cli:update-task` — log completed work, sync to daily note / parent goal
   - `/vault-cli:task-status` — grouped-checkbox status (Success Criteria / Tasks / DoD) + next step
   - `/vault-cli:next-steps` — next actionable steps; offer defer if nothing left today
6. `/vault-cli:sync-progress` — flush conversation to daily note + task pages
7. `/vault-cli:complete-task` — close task
8. `/vault-cli:session-close` — verify session is safe to end (synced, committed, no orphaned state)

`work-on-task` chains `plan-task` (always) and `execute-task` (only when plan-task left phase = `execution`) — the operator gets one command for the full orient → sharpen → kickoff path when the plan is already clean, and an interactive sharpen loop (driven by plan-task's own owner-questions) when there are real gaps. The end state after `/work-on-task` is always either `phase: planning` (gaps remain) or `phase: execution` (kickoff printed) — the only two states the operator needs to reason about.

## Notes

- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- Works in Personal, Brogrammers, Trading, or any future vault registered with `vault-cli config`
- Each vault session loads a single Atlassian MCP under the canonical name `atlassian` (see vault-specific `mcp-*.json` configs); the agent uses `mcp__atlassian__*` regardless of which Jira instance is active
- The agent searches; the slash command asks before creating.
- **Phase 5 is the auto-sharpen + auto-gate chain.** Phase 2 → Phase 5 covers the full "I want to work on this task" intent: orient (status, guides, daily note), then sharpen (via plan-task), then kick off execution (via execute-task) when the plan passes the 4 hard gates. Owner interaction is forced only when plan-task surfaces real gaps; a clean plan goes silently from `work-on-task` to a kickoff line. Phase 5 is skipped on the `not_found` branch (Phase 4 handles that).
