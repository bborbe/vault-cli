---
description: Find task details, transition Jira, set status, track on daily note, discover guides, then auto-chain planning → execution (interactive) or signal the next steps (non-interactive)
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

   **`MODE=non_interactive`** exists for headless callers — e.g. `vault-cli work-on`'s Claude bootstrap runs `claude --print`, which cannot answer `AskUserQuestion`, so an interactive gate would block until the session-start timeout. In this mode the command NEVER calls `AskUserQuestion` and Phase 4's auto-create is skipped (the interactive create-task skill cannot run headlessly). Phase 5 forks on mode: non-interactive prints the next-step signal only — it does NOT chain, because `plan-task` / `execute-task` may call `AskUserQuestion` and would hang a headless caller; interactive auto-chains through planning → execution.

2. **Invoke work-on-task-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-task-assistant'
     prompt: 'Find details and guides for: {stripped arguments}'
   ```

3. **Drive to execution (Phase 5).** If the assistant's report ends with `Ready to work on this task.` (the `found` case), continue to Phase 5 below (auto-chain in interactive mode, signal only in non-interactive). If the report contains the `not_found:` marker, skip Phase 5 and run Phase 4 (Handle not_found) instead.

4. **Done**

The assistant handles all the work, detecting available integrations at runtime:

- **Jira (if `mcp__atlassian` MCP is available)**: fetch issue, auto-assign to current user, auto-transition to "In Progress". Cloud ID auto-detected via `getAccessibleAtlassianResources` — no hardcoded host.
- **Jira (if MCP absent)**: fall back to free-text search on the ID string in vault files. No error.
- **Obsidian task**: find by name or by `jira:` frontmatter; set status to `in_progress`; offer to create local file if missing
- **Daily note**: track with `[/]` checkbox in the Must section; report gracefully if note missing
- **Code tasks**: run `/coding:check-guides` and read project Development Guide if present
- **Guides (semantic search if available)**: search runbooks, operational guides, related docs; fall back to `Glob` if semantic search MCP absent

Output ends with `Ready to work on this task.`

## Phase 4 — Handle not_found (always create)

The agent (dispatched in `## Process` step 2) emits a structured `not_found` verdict from its own Phase 1 (`Find task`) when the requested task cannot be found in any source. This phase parses that verdict and **always creates the local task file** (via the interactive create-task skill) before continuing. There is no "create it?" consent prompt — a `work-on-task` invocation is an intent to work on a task, so a missing local file is created, not queried. (The create-task skill's own interactive flow is still where the operator can back out.)

**Non-interactive gate (checked first):** If `MODE=non_interactive`, do NOT create anything — the interactive create-task skill cannot run under headless `claude --print`. Print the `not_found:` report — the `Searched:` block from the verdict, then `❌ Task not found: "<input>"` — followed by `ℹ️ Non-interactive mode: no task created. Re-run in a terminal to create one.` and STOP. Skip steps 1–4 below.

1. **Parse the agent's report** for the `not_found:` marker and capture `SUGGESTED_NAME`. The agent's `<output_format>` defines two separate fenced markdown blocks — one for the `found` case (ends with `Ready to work on this task.`) and one for the `not_found:` case (literal `not_found:` header on its own line). Look for the `not_found:` block specifically; if the report ends with `Ready to work on this task.` and contains no `not_found:` block, Phase 4 is a no-op and you are done. When the `not_found:` block IS present, match on the `not_found:` token, then extract `SUGGESTED_NAME` — the value after `Suggested task name:` (verbatim, trimmed).
2. **Use `SUGGESTED_NAME` as the seed.** (If the input was a Jira ID and the Jira lookup returned a summary, the agent supplied that summary; otherwise the agent supplied the input string verbatim. Either way, `SUGGESTED_NAME` is what you pass on.)
3. **Always create the task** — invoke `Skill: vault-cli:create-task "<SUGGESTED_NAME>"` (use the same argument form as `commands/create-task.md` — pass the captured suggested name as a quoted argument). No `AskUserQuestion` gate, and no task-vs-goal prompt: `work-on-task` is unambiguously a task path. The create-task skill has its own interactive flow that asks for parent goal, priority, category, defer date, etc. — do not duplicate those asks.
4. **On create success** (create-task skill returns the new task file path or reports success): re-invoke `Task tool with subagent_type: 'vault-cli:work-on-task-assistant' prompt: 'Find details and guides for: <new task title>'` — same form as the Phase 2 invocation, but with the new task title. The agent's standard Phase 2–8 prep mutations then run against the just-created task.
   **On create failure or user cancel inside `vault-cli:create-task`** (the skill returns a non-success status, errors out, or the user aborts midway through its interactive prompts): print `❌ Task creation failed or was cancelled. No task created; no follow-up invocation.` and STOP — do NOT re-invoke `vault-cli:work-on-task-assistant`, do NOT retry the create.

## Phase 5 — Auto-chain plan → execute (interactive) / signal (non-interactive)

After the assistant returns a `found` task, `work-on-task` **drives the task toward execution** rather than stopping at a signal. It orients, then plans, then — when the plan is clean — enters execution and surfaces the first subtask. It never forces execution past an unready plan: the planning gate is still real, just auto-invoked.

Runs only after Phase 2 returned a `found` task — never on `not_found` (Phase 4 handles that branch).

1. **Resolve the task name** from the assistant's `📋 Task: <name>` line (verbatim).

2. **Non-interactive mode (`MODE=non_interactive`) — signal only, no chain.** `plan-task` and `execute-task` may call `AskUserQuestion`; a headless `claude --print` caller cannot answer, so chaining would hang. Print the signal and STOP:
   ```
   ✅ Oriented: <name>. Next:
   → /vault-cli:plan-task "<name>"     — validate the plan (Success Criteria + subtasks)
   → /vault-cli:execute-task "<name>"  — begin executing (flips planning → execution)
   → /vault-cli:complete-task "<name>" — close when done
   ```

3. **Interactive mode — auto-chain through the phases.**

   a. **Plan.** Invoke `Skill: vault-cli:plan-task "<name>"`. It runs the planning gates itself — passes clean with no questions when the task already has Success Criteria + goal-reaching subtasks (e.g. recurring / runbook tasks), or asks its normal targeted questions when there are real gaps. Let it run its own fix loop.

   b. **Branch on plan-task's terminal line:**
      - `✅ Plan ready` → invoke `Skill: vault-cli:execute-task "<name>"`. That re-checks the hard gates, flips `planning → execution`, and prints the first subtask + DoD reminder. The combined plan-task + execute-task output IS the final output — do NOT re-print the signal.
      - `⚠ Task improved …` / score < 8 / gaps the operator left unresolved → do NOT execute. Print:
        ```
        ⚠ Stopped at planning — plan not ready. Remaining: <bullets from plan-task>.
        → Re-run /vault-cli:plan-task "<name>" when ready, then /vault-cli:execute-task "<name>".
        ```
      - Any other terminal line (task closed, phase already past planning, `❌ …`) → relay plan-task's output verbatim; do NOT force execute.

`work-on-task` orients, then drives. In interactive mode a task with an already-complete plan lands in `phase: execution` with its first subtask surfaced, in one command. A task with real planning gaps stops at `planning` after plan-task's questions — the gate is enforced, not skipped. Non-interactive callers keep the deliberate signal (no chaining).

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. **`/vault-cli:work-on-task`** — orient (status + guides + daily note), then auto-chain plan → execute (interactive) or signal (non-interactive) — this command
3. `/vault-cli:plan-task` — sharpen (5 hard gates); never flips phase; auto-invoked by work-on-task (interactive), or run directly
4. `/vault-cli:execute-task` — gate planning → execution; flips phase + prints first subtask + DoD reminder; auto-invoked by work-on-task when the plan is clean, or run directly
5. Start work — while working, use any of:
   - `/vault-cli:update-task` — log completed work, sync to daily note / parent goal
   - `/vault-cli:task-status` — grouped-checkbox status (Success Criteria / Tasks / DoD) + next step
   - `/vault-cli:next-steps` — next actionable steps; offer defer if nothing left today
6. `/vault-cli:sync-progress` — flush conversation to daily note + task pages
7. `/vault-cli:complete-task` — close task
8. `/vault-cli:session-close` — verify session is safe to end (synced, committed, no orphaned state)

In interactive mode `work-on-task` orients, then auto-chains: it runs `/plan-task` and, when the plan is clean, `/execute-task` — so the end state is `phase: execution` with the first subtask surfaced (or `phase: planning` if plan-task found real gaps). In non-interactive mode it orients and stops at `phase: planning`, printing the signal. `/complete-task` is always a deliberate operator step.

## Notes

- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- Works in Personal, Brogrammers, Trading, or any future vault registered with `vault-cli config`
- Each vault session loads a single Atlassian MCP under the canonical name `atlassian` (see vault-specific `mcp-*.json` configs); the agent uses `mcp__atlassian__*` regardless of which Jira instance is active
- The agent searches; the slash command auto-creates the task file on `not_found` (interactive mode).
- **Phase 5 auto-chains in interactive mode, signals in non-interactive.** Phase 2 → Phase 5 covers the "I want to work on this task" intent by orienting (status, guides, daily note), then driving: interactive invocations run `plan-task` and — when it reports `✅ Plan ready` — `execute-task`, landing the task in `phase: execution` with its first subtask surfaced. The planning gate stays enforced: if `plan-task` finds real gaps, the chain stops at `planning` (it never force-executes an unready plan). Non-interactive invocations print the plan → execute → complete signal instead of chaining, because `plan-task` / `execute-task` may call `AskUserQuestion` and would hang a headless `claude --print` caller. Phase 5 is skipped on the `not_found` branch (Phase 4 handles that).
