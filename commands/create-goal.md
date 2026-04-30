---
description: Create a new goal in the configured vault with guided prompts
argument-hint: "[goal description] [--tool] [--vault NAME] [--objective NAME]"
allowed-tools: [Task, Read, Write, Glob, Grep, Bash, AskUserQuestion]
---

Invoke the goal-creator agent to create a goal file in the vault.

Parse `$ARGUMENTS`:
- `--tool` → MODE=tool (orchestration mode, JSON output, no prompts)
- `--vault NAME` → target a specific vault (otherwise use the default vault from `~/.vault-cli/config.yaml`)
- `--objective NAME` → link to parent objective by name (interactive mode resolves via Glob/AskUserQuestion if ambiguous)
- Remaining text → goal description / title

Pass the parsed arguments to the goal-creator agent.
