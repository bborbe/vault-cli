---
description: Find task details, transition Jira, set status, track on daily note, discover guides, then auto-chain planning → execution (non-interactive chains too, printing gaps instead of asking)
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

   **`MODE=non_interactive`** exists for headless callers — e.g. `vault-cli work-on`'s Claude bootstrap runs `claude --print`, which cannot answer `AskUserQuestion`, so an interactive gate would block until the session-start timeout. In this mode the command NEVER calls `AskUserQuestion`, and that ban propagates to every command it chains into. Phase 4's auto-create is still skipped (the interactive create-task skill cannot run headlessly).

   **Both modes chain through planning → execution.** They differ only in what happens when a gate would ask a question: interactive asks it; non-interactive stops at `phase: planning` and prints the unresolved gaps for the operator to resolve on resume. Non-interactive is not "orient and stop" — a headless caller that produces a clean plan lands in `phase: execution` with its first subtask surfaced, same as interactive.

2. **Invoke work-on-task-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-task-assistant'
     prompt: 'Find details and guides for: {stripped arguments}'
   ```

3. **Drive to execution (Phase 5).** If the assistant's report ends with `Ready to work on this task.` (the `found` case), continue to Phase 5 below (auto-chains in both modes). If the report contains the `not_found:` marker, skip Phase 5 and run Phase 4 (Handle not_found) instead.

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

## Phase 5 — Auto-chain plan → execute

After the assistant returns a `found` task, `work-on-task` **drives the task toward execution** rather than stopping at a signal. It orients, then plans, then — when the plan is clean — enters execution and surfaces the first subtask. It never forces execution past an unready plan: the planning gate is still real, just auto-invoked.

**Both modes chain.** Mode changes only how an unresolved gap is handled, never whether the chain runs.

Runs only after Phase 2 returned a `found` task — never on `not_found` (Phase 4 handles that branch).

1. **Resolve the task name** from the assistant's `📋 Task: <name>` line (verbatim).

2. **Set the question policy from `MODE`, and state it to every command you chain into.**
   - `MODE=interactive` → **ASK**: chained commands may use `AskUserQuestion` as they normally would.
   - `MODE=non_interactive` → **NO-ASK**: neither this command nor anything it invokes may call `AskUserQuestion`. A gate that would ask instead reports the gap and stops. Pass this explicitly when invoking the skills below — append ` --non-interactive` to the skill argument so `plan-task` / `execute-task` apply their own NO-ASK contract (see `commands/plan-task.md` § Non-interactive contract).

   Rationale: a headless `claude --print` caller cannot answer a prompt, so an ask is not a question — it is a hang. Stopping with the gap printed gives the operator the same information without the deadlock, and the Vault UI's Start button (which runs exactly this headless turn and then hands the operator a bare `--resume`, no follow-up prompt) still lands a clean task in `execution`.

3. **Plan.** Invoke `Skill: vault-cli:plan-task "<name>"` (NO-ASK mode: `Skill: vault-cli:plan-task "<name>" --non-interactive`). It runs the planning gates itself — passes clean with no questions when the task already has Success Criteria + goal-reaching subtasks (e.g. recurring / runbook tasks), or surfaces real gaps. In ASK mode let it run its own fix loop; in NO-ASK mode it reports gaps instead of asking.

4. **Branch on plan-task's terminal line** (identical in both modes):
   - **Plan is good → proceed to execute-task.** Two success lines both qualify: `✅ Plan ready` (plan just passed, phase still `planning`) OR `✅ Task sharpened` (the task was already past planning — plan-task validated but didn't move phase). In both cases invoke `Skill: vault-cli:execute-task "<name>"` (appending ` --non-interactive` in NO-ASK mode). execute-task owns the phase logic and is idempotent: it flips `planning → execution` and prints the first subtask + DoD; or, when the task is already in `execution` / `ai_review` / `human_review`, re-surfaces the first unchecked subtask + DoD ("where was I?"); or, when the task is `done` / closed, prints its own refusal. The combined plan-task + execute-task output IS the final output — do NOT re-print the signal.
   - `⚠ Task improved …` / score < 8 / unresolved gaps → do NOT execute (planning gate not cleared). Print:
     ```
     ⚠ Stopped at planning — plan not ready. Remaining: <bullets from plan-task>.
     → Re-run /vault-cli:plan-task "<name>" when ready, then /vault-cli:execute-task "<name>".
     ```
     In NO-ASK mode this is the expected landing spot for an under-specified task: the operator resumes the session, answers the listed gaps, and the chain continues interactively from there.
   - `❌ …` (plan-task hard error — task not found, input error) → relay plan-task's output verbatim; do NOT invoke execute-task.

5. **Surface the session-connect suggestion.** The assistant (Phase 3 session-connect) sets the task's `claude_session_id` frontmatter to the current session when empty (real UUID detected from the transcript dir, falling back to the task name) and reports it in its output (`✅ Session: connected …` / `ℹ️ Session: already connected …`). After the chain lands (or stops at planning), echo the rename hint once:
   ```
   💡 Suggest: run /rename <name> to name this session after the task — no quotes: /rename takes the rest of the line verbatim, so a quoted suggestion names the session with literal quote characters
   ```
   This connects the task with the session — the session appears under the task name in the session list / Vault UI.

`work-on-task` orients, then drives. A task with an already-complete plan lands in `phase: execution` with its first subtask surfaced, in one command, headless or not. A task with real planning gaps stops at `planning` — after plan-task's questions in ASK mode, or with the gaps printed in NO-ASK mode. The gate is enforced in both, and never at the cost of a hang.

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. **`/vault-cli:work-on-task`** — orient (status + guides + daily note), then auto-chain plan → execute — this command
3. `/vault-cli:plan-task` — sharpen (5 hard gates); never flips phase; auto-invoked by work-on-task, or run directly
4. `/vault-cli:execute-task` — gate planning → execution; flips phase + prints first subtask + DoD reminder; auto-invoked by work-on-task when the plan is clean, or run directly
5. Start work — while working, use any of:
   - `/vault-cli:update-task` — log completed work, sync to daily note / parent goal
   - `/vault-cli:task-status` — grouped-checkbox status (Success Criteria / Tasks / DoD) + next step
   - `/vault-cli:next-steps` — next actionable steps; offer defer if nothing left today
6. `/vault-cli:sync-progress` — flush conversation to daily note + task pages
7. `/vault-cli:complete-task` — close task
8. `/vault-cli:session-close` — verify session is safe to end (synced, committed, no orphaned state)

`work-on-task` orients, then auto-chains in both modes: it runs `/plan-task` and, when the plan is clean, `/execute-task` — so the end state is `phase: execution` with the first subtask surfaced (or `phase: planning` if plan-task found real gaps). Non-interactive differs only in that a gap is printed rather than asked about. `/complete-task` is always a deliberate operator step.

## Notes

- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- Works in Personal, Brogrammers, Trading, or any future vault registered with `vault-cli config`
- Each vault session loads a single Atlassian MCP under the canonical name `atlassian` (see vault-specific `mcp-*.json` configs); the agent uses `mcp__atlassian__*` regardless of which Jira instance is active
- The agent searches; the slash command auto-creates the task file on `not_found` (interactive mode).
- **Phase 5 auto-chains in both modes.** Phase 2 → Phase 5 covers the "I want to work on this task" intent by orienting (status, guides, daily note), then driving: it runs `plan-task` and — when it reports `✅ Plan ready` — `execute-task`, landing the task in `phase: execution` with its first subtask surfaced. The planning gate stays enforced: if `plan-task` finds real gaps, the chain stops at `planning` (it never force-executes an unready plan). Non-interactive invocations chain under a NO-ASK contract — no `AskUserQuestion` anywhere in the chain; a gate that would ask prints the gap and stops instead, so a headless `claude --print` caller can never hang. Phase 5 is skipped on the `not_found` branch (Phase 4 handles that).
- **Non-interactive is the normal path, not an edge case.** The Vault UI "Start" button runs the headless turn and then hands the operator a bare resume command (`vault-ui/src/vault_ui/api/tasks.py` `_build_resume_command` emits `<script> --resume <id>[ -n <title>]`, no prompt argument). vault-cli's own turn-2 continuation (`pkg/ops/workon.go`) only fires when vault-cli resumes the session itself, which that path never does. So whatever the headless turn accomplishes is all the operator gets before they type — which is why it must chain.

## Passive metrics

Each work-on run appends one entry to the task's `metrics_sessions` frontmatter field
(session id + start timestamp). These metrics fields are written passively by vault-cli
and must not be hand-edited.
