---
description: Mark a goal as complete — the vault-cli verb is the single gate
argument-hint: "<goal-name-or-path> [--non-interactive]"
---

<objective>
Mark a goal complete by calling `vault-cli goal complete`, which is the single closure gate: it reads the goal's `# Success Criteria` and `# Definition of Done` boxes plus its linked tasks and refuses on any open item. This command never counts boxes itself and offers no bypass.
</objective>

<process>
1. Parse arguments:
   - If contains `--non-interactive` (or deprecated `--tool`) → MODE=non_interactive, remove the flag from args
   - Otherwise → MODE=interactive
   - Extract the goal name from the remaining args

2. Run the verb — it is the gate:
   ```bash
   vault-cli goal complete "{goal_name}" --vault "<vault>"
   ```
   - **exit 0** → the goal is completed (or was already terminal — the verb is idempotent on a `completed`/`aborted` page).
   - **non-zero exit** → the verb refused. Report its printed line verbatim (`cannot complete goal: N closure item(s) open: …`) and STOP. Do not count boxes yourself.

3. **MODE=non_interactive** (`--non-interactive`): run the verb with `--output json` and return its own JSON verbatim — `{"success": true, …}` on exit 0, or `{"success": false, "error": "…", "open_items": [ … ]}` on a refusal. Never ask a question.

4. **MODE=interactive**: on exit 0 report `✅ Goal completed: [[{goal_name}]]`; on a refusal, show the verb's open-item list and stop. There is no bypass path — a close that would need one is a close this command does not make.
</process>

<success_criteria>
- `vault-cli goal complete` is invoked — the verb is the only gate; this command never counts boxes itself
- A refusal is reported verbatim (the verb's own open-item line) and leaves the goal open
- No bypass flag and no box-counting logic exist in this command
- MODE=non_interactive returns the verb's JSON and never asks a question
</success_criteria>
