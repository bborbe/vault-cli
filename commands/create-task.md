---
description: Create a new task in the configured vault with guided prompts
argument-hint: "[task description] [--tool] [--vault NAME]"
allowed-tools: [Task, Read, Write, Glob, Grep, Bash, AskUserQuestion]
---

Invoke the task-creator agent to create a task file in the vault.

Parse `$ARGUMENTS`:
- `--tool` → MODE=tool (orchestration mode, JSON output, no prompts)
- `--vault NAME` → target a specific vault (otherwise use the default vault from `~/.vault-cli/config.yaml`)
- Remaining text → task description / title

Pass the parsed arguments to the task-creator agent.
