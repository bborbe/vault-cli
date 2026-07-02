---
description: Auto-detect task or goal from argument and dispatch to the correct assistant — one command, no type-memorization required
argument-hint: <name-or-jira-id>
allowed-tools: [Task, AskUserQuestion, Skill, Bash(vault-cli resolve *)]
---

Auto-detects whether `<name-or-jira-id>` is a task or goal, then dispatches to the appropriate assistant agent. Eliminates the need to remember `/vault-cli:work-on-task` vs `/vault-cli:work-on-goal`.

## Usage

```bash
/vault-cli:work-on BRO-12345                          # Jira-ID → task (regex match, zero CLI calls)
/vault-cli:work-on "Existing Task Name"               # probe vault → task → work-on-task-assistant
/vault-cli:work-on "Existing Goal Name"               # probe vault → goal → work-on-goal-assistant
/vault-cli:work-on "Not Found"                        # not found → AskUserQuestion (create task or goal)
```

## Process

### Phase 1 — Classify

1. **Validate input**: if no argument or empty string → print `❌ Pass a task/goal name or Jira ID: /vault-cli:work-on "<name-or-jira-id>"` and STOP

2. **Jira-ID fast path**: if argument matches `^[A-Z][A-Z0-9]+-\d+$` (e.g. `BRO-20996`, `TRADE-1234`), classification is `task` — goals never have Jira IDs. Skip to Phase 2 dispatch. Zero CLI calls, zero vault probes.

3. **Probe via resolve**: run `vault-cli resolve "<argument>" --output json`. Parse the output:
   - `found: true, type: "task"` → classification = `task`
   - `found: true, type: "goal"` → classification = `goal`
   - `found: false` → classification = `not_found`

### Phase 2 — Dispatch

4. **If `task`**: invoke the `vault-cli:work-on-task-assistant` agent with the argument as the prompt:
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-task-assistant'
     prompt: 'Find details and guides for: {argument}'
   ```
   Then continue with Phase 3 (auto-sharpen + auto-gate), identical to `commands/work-on-task.md` Phase 5.

5. **If `goal`**: invoke the `vault-cli:work-on-goal-assistant` agent:
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-goal-assistant'
     prompt: 'Find goal: {argument} and prepare work context'
   ```
   The assistant handles task selection and delegation — exactly as `commands/work-on-goal.md` Process step 2.

6. **If `not_found`**: run Phase 4 (Handle not_found).

### Phase 3 — Auto-sharpen + auto-gate (task route only)

When the `work-on-task-assistant` report ends with `Ready to work on this task.`, chain into `vault-cli:plan-task` → `vault-cli:execute-task` exactly as `commands/work-on-task.md` Phase 5. The goal route delegates this to the goal assistant, which handles it internally.

### Phase 4 — Handle not_found

Follow `commands/work-on-task.md` Phase 4 (Handle not_found) verbatim, with one extension:

- The `AskUserQuestion` gains a third option: **`Goal`** — description: `Run vault-cli:create-goal with "<SUGGESTED_NAME>" as the seed title`. The two existing options (`Yes, create it` for task, `No, stop here`) remain.
- The options are now:
  1. `Yes, create task` — invoke `Skill: vault-cli:create-task "<SUGGESTED_NAME>"`, then re-invoke work-on-task-assistant
  2. `Create goal` — invoke `Skill: vault-cli:create-goal "<SUGGESTED_NAME>"`, then re-invoke work-on-goal-assistant
  3. `No, stop here` — print manual search tips and STOP

## Integration

Task lifecycle (extends `commands/work-on-task.md` Integration section):

1. `/vault-cli:create-task` / `/vault-cli:create-goal` — capture
2. **`/vault-cli:work-on`** — orient + auto-detect type + dispatch — this command
3. `/vault-cli:plan-task` — sharpen (auto-chained for task route)
4. `/vault-cli:execute-task` — gate planning → execution (auto-chained when plan passes)
5. Work → `/vault-cli:update-task` / `/vault-cli:task-status`
6. `/vault-cli:sync-progress` → `/vault-cli:complete-task`
7. `/vault-cli:session-close`

## Notes

- Keeps `work-on-task.md` and `work-on-goal.md` as functional aliases (no changes to either file)
- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- `vault-cli resolve` dependency: must be installed (binary ≥ v0.95.0). If `resolve` exits non-zero or outputs invalid JSON, print a clear error: `❌ vault-cli resolve failed — ensure vault-cli binary is ≥ v0.95.0` and fall back to `AskUserQuestion` asking the user to pick task vs goal manually.
- Works in any vault registered with `vault-cli config`
