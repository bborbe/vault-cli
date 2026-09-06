---
description: Work on a goal — see context, pick task, get guides via work-on-goal-assistant, then hand the selected task off (Start / Resume / leave-alone) instead of executing it; auto-creates the goal when it doesn't exist
argument-hint: <goal-name-or-jira-id> [--non-interactive]
allowed-tools: [Task, AskUserQuestion, Skill, Bash(vault-cli *)]
---

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

   **`MODE=non_interactive`** is for headless callers (`claude --print` — Vault UI Start / `vault-cli work-on-goal`). The command NEVER calls `AskUserQuestion`; the ban propagates to everything it chains into, and Phase 4's auto-create is skipped (create-goal cannot run headlessly).

2. **Invoke work-on-goal-assistant**
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-goal-assistant'
     prompt: 'Find goal: {goal_name}, prepare work context, and classify the recommended task's session state per your session-state contract'
   ```

3. **Branch on the assistant's report** (present it verbatim — its output format lives in `agents/work-on-goal-assistant.md`):
   - If the report ends with `Ready to work on this task.` (the `found` case): continue to Phase 3 (Hand off) below.
   - If the report contains the `not_found:` marker: skip Phase 3 and run Phase 4 (Handle not_found) instead.

## Phase 3 — Hand off (management session, not deep work)

A goal session decides *what* to work on and watches progress across a 1–4 week goal — it never does the work. This phase recommends ONE task, checks the live fleet for it, and hands off (Start / Resume / leave-alone). No `plan-task`, no `execute-task`; **this command never flips any task to `phase: execution`.**

### 3.1 Recommend the next open task

Resolve the recommended task from the assistant's `🎯 Recommended:` line. The assistant walked the goal's `# Tasks` per the `execute-goal.md` step-7 walk and recommends the first non-terminal task. If every task under the goal is terminal, print `✅ All tasks under the goal are terminal — run /vault-cli:verify-goal, then /vault-cli:complete-goal` and stop — never invent work.

### 3.2 Fleet check — session state, not status

Branch on the recommended task's session state (classified by the assistant per its session-state contract — `agents/work-on-goal-assistant.md` Phase 5.5 — keyed on the task's `claude_session_id`):

- `live` → a session is actively working the task → report it and stop for this task; do NOT offer Start or Resume (a duplicate session is what this check prevents)
- `quiet` → a session exists but is idle → offer Resume (attach to the existing session)
- `indeterminate` → ambiguous signals → report the ambiguity, let the operator decide
- none → no session exists → offer Start (3.3)

**Never read `status: in_progress` as "someone is working this"** — it is a queue state, not a liveness signal. `ListAgents` is a secondary display signal only, never the decision input.

### 3.3 Start — approval-gated background job

When the task has no session, offer to start one as a background job:

```
vault-cli task work-on "<task>" --mode headless
```

Same call vault-ui's ▶ Start button makes: spawns a real headless session (runs `<claude_script> --print -p "/vault-cli:work-on-task \"<task-path>\""`), writes `claude_session_id` into the task, returns a resume command (`<claude_script> --resume <session_id>`) the operator attaches to later. The started session arrives already planned and in execution — the goal session never does the work itself.

**Approval-gated, always:** starting a background session is a real, costed action. ASK mode gets explicit confirmation before running; NO-ASK mode prints the command + resume workflow for the operator (a headless caller cannot approve, and an unapproved start violates the gate).

### 3.4 Untracked work → create and link, then hand off

When goal analysis surfaces work that is not yet a task under the goal, do NOT do it inline: `Skill: vault-cli:create-task "<name>"` (NO-ASK: print the name instead), link the new task into the goal's `# Tasks` as a wikilink (via the assistant — it has Edit/Write), then hand off per 3.2 / 3.3.

### 3.5 Headless / NO-ASK — survey, record, stop

`vault-cli goal work-on --mode headless` (vault-ui ▶ Start on a goal) never flips any task to `phase: execution`. NO-ASK mode prints the recommended task + its session state as the record of this run, prints the Start / Resume command for the operator, and STOPS — no `AskUserQuestion`, no `plan-task`, no `execute-task`, no auto-start.

**Session-connect.** The goal assistant (Phase 1) sets the goal's `claude_session_id` frontmatter when empty (unchanged). The recommended task connects to a session only when one is started for it — outside this command.

## Phase 4 — Handle not_found (always create)

The agent emits a structured `not_found` verdict (its Phase 1) when the goal cannot be found. This phase **always creates the goal page** (via the interactive create-goal skill) — no consent prompt: a `work-on-goal` invocation is an intent to work on a goal. (The skill's own interactive flow is where the operator can back out.)

**Non-interactive gate (checked first):** If `MODE=non_interactive`, do NOT create anything — the interactive create-goal skill cannot run under headless `claude --print`. Print the `not_found:` report — the `Searched:` block from the verdict, then `❌ Goal not found: "<input>"` — followed by `ℹ️ Non-interactive mode: no goal created. Re-run in a terminal to create one.` and STOP. Skip steps 1–4 below.

1. **Parse the agent's report** for the `not_found:` marker (see the agent's `<output_format>` for the exact form) and capture `SUGGESTED_NAME` — the value after `Suggested goal name:` (verbatim, trimmed). If the report ends with `Ready to work on this task.` and contains no `not_found:` block, Phase 4 is a no-op and you are done.
2. **Use `SUGGESTED_NAME` as the seed** (Jira summary if the input was a Jira ID and the lookup returned one, else the input string verbatim).
3. **Always create the goal** — `Skill: vault-cli:create-goal "<SUGGESTED_NAME>"`. No `AskUserQuestion` gate; the skill's own interactive flow asks the rest — do not duplicate.
4. **On create success**: re-invoke the assistant (Process step 2 form) with the new goal title — its standard prep runs against the just-created goal.
   **On create failure or user cancel inside `vault-cli:create-goal`** (the skill returns a non-success status, errors out, or the user aborts midway through its interactive prompts): print `❌ Goal creation failed or was cancelled. No goal created; no follow-up invocation.` and STOP — do NOT re-invoke `vault-cli:work-on-goal-assistant`, do NOT retry the create.

## Integration

Goal-first workflow:
1. Pick goal name (from your notes, focus page, etc.)
2. `/vault-cli:work-on-goal "<name>"` → find-or-create: uses the existing goal, or auto-creates it when missing (Phase 4), then context + task selection + session-state check, then hand off (Start / Resume / leave-alone) — never executes the task itself
3. The task runs in its own session; re-run `/vault-cli:work-on-goal` to survey the goal and pick the next one

Sibling commands:
- `/vault-cli:next-task` — task-first workflow
- `/vault-cli:work-on-task <id>` — direct task prep (auto-creates the task on not_found)
- `/vault-cli:work-on <name-or-jira-id>` — auto-detect task vs goal, dispatch to the matching work-on command
- `/vault-cli:goal-status` — goal progress only (no task delegation)
