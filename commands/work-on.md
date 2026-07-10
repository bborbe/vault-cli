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
/vault-cli:work-on "Not Found"                        # not found → always create (free text → pick task/goal; Jira ID → task)
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
   Then continue with Phase 3 (next-step signal), identical to `commands/work-on-task.md` Phase 5.

5. **If `goal`**: invoke the `vault-cli:work-on-goal-assistant` agent:
   ```
   Task tool with:
     subagent_type: 'vault-cli:work-on-goal-assistant'
     prompt: 'Find goal: {argument} and prepare work context'
   ```
   The assistant handles task selection and delegation — exactly as `commands/work-on-goal.md` Process step 2.

6. **If `not_found`**: run Phase 4 (Handle not_found).

### Phase 3 — Next-step signal

When the `work-on-task-assistant` (task route) or `work-on-goal-assistant` (goal route) report ends with `Ready to work on this task.`, print the plan → execute → complete signal exactly as `commands/work-on-task.md` Phase 5 — resolve `<name>` from the `📋 Task: <name>` line. Do NOT auto-invoke `plan-task` or `execute-task`; the operator runs each deliberately.

### Phase 4 — Handle not_found (always create)

`work-on` **always creates a file** on `not_found` — never a "create it?" consent prompt. The only question is *which type*, and that depends on how Phase 1 classified the input.

**Non-interactive gate (checked first):** if `MODE=non_interactive` (inherited from a headless caller), create nothing — the interactive create skills cannot run under `claude --print`. Print the `not_found:` report and STOP, exactly as `commands/work-on-task.md` Phase 4's non-interactive gate.

Branch on the Phase 1 classification path:

- **Jira-ID input** (Phase 1 step 2 fast-path → `task`): delegate to `commands/work-on-task.md` Phase 4 verbatim — it auto-creates the task (no prompt), then re-invokes `work-on-task-assistant`. A Jira ID is unambiguously a task.
- **Free-text input** (Phase 1 step 3 `resolve` returned `found: false` → type genuinely unknown): the file type cannot be inferred, so ask **only which type** via `AskUserQuestion` — two options, **no "stop / don't create" escape** (this is type disambiguation, not a consent gate):
  1. `Task` — invoke `Skill: vault-cli:create-task "<SUGGESTED_NAME>"`, then on success re-invoke `work-on-task-assistant` (`prompt: 'Find details and guides for: <new task title>'`)
  2. `Goal` — invoke `Skill: vault-cli:create-goal "<SUGGESTED_NAME>"`, then on success re-invoke `work-on-goal-assistant` (`prompt: 'Find goal: <new goal title> and prepare work context'`)

  `SUGGESTED_NAME` is the input string verbatim (free text has no Jira summary to derive from). On create failure or user cancel inside the create skill, print `❌ Create failed or was cancelled. No file created; no follow-up invocation.` and STOP.

## Integration

Task lifecycle (extends `commands/work-on-task.md` Integration section):

1. `/vault-cli:create-task` / `/vault-cli:create-goal` — capture
2. **`/vault-cli:work-on`** — orient + auto-detect type + dispatch + signal next steps — this command
3. `/vault-cli:plan-task` — sharpen (run directly after work-on)
4. `/vault-cli:execute-task` — gate planning → execution (run directly after plan-task)
5. Work → `/vault-cli:update-task` / `/vault-cli:task-status`
6. `/vault-cli:sync-progress` → `/vault-cli:complete-task`
7. `/vault-cli:session-close`

## Notes

- Keeps `work-on-task.md` and `work-on-goal.md` as functional aliases (no changes to either file)
- No hardcoded Jira hostname, project key, or vault path — everything detected at runtime
- `vault-cli resolve` dependency: must be installed (binary ≥ v0.95.0). If `resolve` exits non-zero or outputs invalid JSON, print a clear error: `❌ vault-cli resolve failed — ensure vault-cli binary is ≥ v0.95.0` and fall back to `AskUserQuestion` asking the user to pick task vs goal manually.
- Works in any vault registered with `vault-cli config`
