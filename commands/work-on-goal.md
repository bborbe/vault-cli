---
description: Work on a goal — see context, pick task, get guides via work-on-goal-assistant, then auto-chain the selected task plan → execute; auto-creates the goal when it doesn't exist
argument-hint: <goal-name-or-jira-id> [--non-interactive]
allowed-tools: [Task, AskUserQuestion, Skill, Bash(vault-cli *)]
---

Start working on a goal by seeing domain guides, progress, and task options.

## Usage

```bash
/vault-cli:work-on-goal "Goal Name"
/vault-cli:work-on-goal BRO-20702         # Jira key — resolves to the goal's jira: frontmatter, or auto-creates a goal for it
```

The goal name is **required** — pass it as a quoted string. (Focus-page auto-detection is not part of this command; if you want a default-goal workflow, build a vault-side wrapper that resolves the name then calls this command.)

## Process

1. **Parse input**
   - Parse `$ARGUMENTS`: if it contains `--non-interactive` → set `MODE=non_interactive` and strip that flag token from the arguments; otherwise `MODE=interactive`. Use the stripped arguments as the goal identifier everywhere below — NEVER pass the flag token into the assistant prompt or goal search.
   - If no argument remains after stripping: `❌ Pass a goal name: /vault-cli:work-on-goal "Goal Name"` and STOP

   **`MODE=non_interactive`** exists for headless callers (e.g. the Vault UI Start button / `vault-cli work-on-goal` CLI bootstrap runs `claude --print`, which cannot answer `AskUserQuestion`). In this mode the command NEVER calls `AskUserQuestion`, and that ban propagates to every command it chains into. Phase 4's auto-create is still skipped (the interactive create-goal skill cannot run headlessly).

2. **Invoke work-on-goal-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-goal-assistant'
     prompt: 'Find goal: {goal_name} and prepare work context'
   ```

3. **Branch on the assistant's report**
   - If the report ends with `Ready to work on this task.` (the `found` case): continue to Phase 3 (Drive to execution) below.
   - If the report contains the `not_found:` marker: skip Phase 3 and run Phase 4 (Handle not_found) instead.

## Phase 3 — Drive to execution

   After the assistant returns a `found` goal, resolve the selected task name from its `📋 Task: <name>` line and follow `commands/work-on-task.md` Phase 5 exactly.

   **Auto-chain the selected task (both modes):** invoke `Skill: vault-cli:plan-task "<name>"`, then on either success line — `✅ Plan ready` (phase still `planning`) or `✅ Task sharpened` (task already past planning) — invoke `Skill: vault-cli:execute-task "<name>"` (idempotent: flips `planning → execution` and prints first subtask + DoD, or re-surfaces the work block if already in execution, or refuses if done). If plan-task reports unresolved gaps (`⚠ …` / score < 8), stop at planning and print what remains — never force-execute. On a plan-task `❌ …` hard error, relay it verbatim.

   **Non-interactive / headless mode** chains identically, under NO-ASK: append ` --non-interactive` to both skill arguments. No `AskUserQuestion` anywhere in the chain — a gate that would ask prints the gap and stops at `planning` instead, so a headless caller can never hang.

   **Session-connect surfaces through the chain.** The goal assistant (Phase 1) sets the goal's `claude_session_id` frontmatter to the current session when empty (real UUID detected from the transcript dir, falling back to the goal name) and reports it in its context block. `commands/work-on-task.md` Phase 5 step 5 then echoes the `/rename` hint for the selected task — both the goal and the task are connected to the session by name.

## Phase 4 — Handle not_found (always create)

The agent (dispatched in `## Process` step 2) emits a structured `not_found` verdict from its own Phase 1 (`Find goal`) when the requested goal cannot be found in any source. This phase parses that verdict and **always creates the goal page** (via the interactive create-goal skill) before continuing. There is no "create it?" consent prompt — a `work-on-goal` invocation is an intent to work on a goal, so a missing goal page is created, not queried. (The create-goal skill's own interactive flow is still where the operator can back out.)

**Non-interactive gate (checked first):** If `MODE=non_interactive`, do NOT create anything — the interactive create-goal skill cannot run under headless `claude --print`. Print the `not_found:` report — the `Searched:` block from the verdict, then `❌ Goal not found: "<input>"` — followed by `ℹ️ Non-interactive mode: no goal created. Re-run in a terminal to create one.` and STOP. Skip steps 1–4 below.

1. **Parse the agent's report** for the `not_found:` marker and capture `SUGGESTED_NAME`. The agent's `<output_format>` defines two separate fenced markdown blocks — one for the `found` case (ends with `Ready to work on this task.`) and one for the `not_found:` case (literal `not_found:` header on its own line). Look for the `not_found:` block specifically; if the report ends with `Ready to work on this task.` and contains no `not_found:` block, Phase 4 is a no-op and you are done. When the `not_found:` block IS present, match on the `not_found:` token, then extract `SUGGESTED_NAME` — the value after `Suggested goal name:` (verbatim, trimmed).
2. **Use `SUGGESTED_NAME` as the seed.** (If the input was a Jira ID and the Jira lookup returned a summary, the agent supplied that summary; otherwise the agent supplied the input string verbatim. Either way, `SUGGESTED_NAME` is what you pass on.)
3. **Always create the goal** — invoke `Skill: vault-cli:create-goal "<SUGGESTED_NAME>"` (use the same argument form as `commands/create-goal.md` — pass the captured suggested name as a quoted argument). No `AskUserQuestion` gate: `work-on-goal` is unambiguously a goal path. The create-goal skill has its own interactive flow that asks for summary, success criteria, non-goals, etc. — do not duplicate those asks.
4. **On create success** (create-goal skill returns the new goal file path or reports success): re-invoke `Task tool with subagent_type: 'vault-cli:work-on-goal-assistant' prompt: 'Find goal: <new goal title> and prepare work context'` — same form as the Process step 2 invocation, but with the new goal title. The agent's standard Phase 1–8 prep then runs against the just-created goal.
   **On create failure or user cancel inside `vault-cli:create-goal`** (the skill returns a non-success status, errors out, or the user aborts midway through its interactive prompts): print `❌ Goal creation failed or was cancelled. No goal created; no follow-up invocation.` and STOP — do NOT re-invoke `vault-cli:work-on-goal-assistant`, do NOT retry the create.

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
2. `/vault-cli:work-on-goal "<name>"` → find-or-create: uses the existing goal, or auto-creates it when missing (Phase 4), then context + task selection, then auto-chain the selected task plan → execute (both modes)
3. Start work with full context

Sibling commands:
- `/vault-cli:next-task` — task-first workflow
- `/vault-cli:work-on-task <id>` — direct task prep (auto-creates the task on not_found)
- `/vault-cli:work-on <name-or-jira-id>` — auto-detect task vs goal, dispatch to the matching work-on command
- `/vault-cli:goal-status` — goal progress only (no task delegation)
