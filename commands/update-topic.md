---
description: Update a topic page from disk — propose missing `## Goals` members, refresh listed goal statuses, re-date Current Reading, append Progress — delegates to topic-manager-agent
allowed-tools: [Read, Glob, Bash, AskUserQuestion, Task]
argument-hint: (detects from conversation)
---

Reconcile a topic page with disk: detect the topic, propose any goal whose `topics:` names it but which is absent from `## Goals`, refresh each listed goal's status, re-date `# Current Reading`, and append a dated `# Progress` entry. Every `## Goals` addition is confirmation-gated — never silent.

Topic detection **must stay inline** — a sub-agent cannot see the parent conversation (the same constraint `commands/execute-topic.md` documents). The resolved topic name and page path are passed to the agent as `ARGS`.

## Process

### 1. Resolve `topics_dir`

Never hardcode the folder — the per-vault folder is declared, not fixed:

```bash
TOPICS_DIR=$(vault-cli config list --output json 2>/dev/null | python3 -c "import json,sys,os;cwd=os.path.realpath(os.getcwd());d=json.load(sys.stdin);print(next((v.get('topics_dir') or '23 Topics' for v in d if os.path.realpath(os.path.expanduser(v['path']))==cwd),'23 Topics'))" 2>/dev/null || echo "23 Topics")
```

The `|| echo` is a last-resort fallback for a vault with no config entry, not a default to prefer.

### 2. Detect the topic (inline)

**With argument:** exact path if path-like, else `Glob` `<topics_dir>/*<arg>*.md`. Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (same priority order as `/vault-cli:execute-topic`):

1. Most recent `/verify-topic`, `/vault-cli:execute-topic` or `/worker-verify` output — scan for the resolved topic name.
2. Most recent `[[Topic Name]]` wikilink referenced as a topic subject.
3. Most recently modified file in `<topics_dir>/`.

Multiple matches → ask via `AskUserQuestion`. Zero → `❌ No topic detected. Pass a topic identifier or name.` STOP.

Print `Detected topic: <name>` on the first line, before any state mutation.

### 3. Invoke topic-manager-agent

`MODE` is `interactive` unless the caller passed `--non-interactive`, in which case it is `tool` — the headless contract: the agent prints the proposal and writes nothing.

```
Task tool with:
  subagent_type: 'vault-cli:topic-manager-agent'
  prompt: 'ACTION: update\nARGS: <absolute page path>\nMODE: <interactive|tool>'
```

The agent computes the membership diff, asks for confirmation, applies the `## Goals` additions, refreshes the status tokens, re-dates `# Current Reading` and appends the `# Progress` entry.

### 4. Done

The command prints the agent's report. Under `MODE: tool` the agent prints the proposed additions and writes nothing.

## Notes

- **Confirmation lives in the agent** — it holds `AskUserQuestion` and asks before writing; declining leaves `## Goals` unchanged while the proposal is still printed.
- **Never removes a listed goal.** Cross-topic goals already listed are left alone.
- **Writes no counters** — `# Status Summary` stays hand-written per the vault's Topic Writing Guide.
- Vault detection via `vault-cli config list` + `$PWD`; `topics_dir` / `goals_dir` come from the matched vault config.
- Sibling commands: `/vault-cli:update-goal`, `/vault-cli:update-task`, `/vault-cli:execute-topic`
