---
description: Update a topic page from disk — propose missing `## Goals` members, refresh listed goal statuses, re-date Current Reading, append Progress — delegates to topic-manager-agent
allowed-tools: [Read, Glob, Bash, AskUserQuestion, Task]
argument-hint: [topic name] [--non-interactive]
---

Reconcile a topic page with disk: detect the topic, propose any goal whose `topics:` names it but which is absent from `## Goals`, refresh each listed goal's status, re-date `# Current Reading`, and append a dated `# Progress` entry. Every `## Goals` addition is confirmation-gated — never silent.

Topic detection **must stay inline** — a sub-agent cannot see the parent conversation (the same constraint `commands/execute-topic.md` documents). The resolved topic name and page path are passed to the agent as `ARGS`.

## Non-interactive contract

If the arguments contain `--non-interactive`, run under **NO-ASK**: never call `AskUserQuestion`, not for the ambiguous-match case, not anywhere.

**Strip the flag before anything else.** Leaving it in the argument list makes step 3 glob for the literal text `--non-interactive` and stop with `❌ No topic detected`, so the headless path never runs as written.

- **Ambiguous topic** (multiple `Glob` matches) → print the candidates and `❌ Ambiguous topic identifier — pass an exact path.` STOP. Never ask.
- The agent receives `MODE: tool` and prints the proposal, writing nothing.

## Process

### 1. Parse arguments

- `--non-interactive` present → `MODE=tool`, and remove the token from the argument list.
- Otherwise → `MODE=interactive`.
- Whatever remains is the topic identifier (empty when the caller passed none).

### 2. Resolve `topics_dir`

Never hardcode the folder — the per-vault folder is declared, not fixed:

```bash
TOPICS_DIR=$(vault-cli config list --output json 2>/dev/null | python3 -c "import json,sys,os;cwd=os.path.realpath(os.getcwd());d=json.load(sys.stdin);print(next((v.get('topics_dir') or '23 Topics' for v in d if os.path.realpath(os.path.expanduser(v['path']))==cwd),'23 Topics'))" 2>/dev/null || echo "23 Topics")
```

The `|| echo` is a last-resort fallback for a vault with no config entry, not a default to prefer. A failed lookup yields no `Glob` match, so the command STOPs rather than writing to the wrong folder.

### 3. Detect the topic (inline)

**With an argument:** exact path if path-like, else `Glob` `<topics_dir>/*<arg>*.md`. Multiple matches → list and STOP (under NO-ASK) or `AskUserQuestion` (interactive). Zero → STOP.

**Without an argument — detect from conversation** (same priority order as `/vault-cli:execute-topic`):

1. Most recent `/verify-topic`, `/vault-cli:execute-topic` or `/worker-verify` output — scan for the resolved topic name.
2. Most recent `[[Topic Name]]` wikilink referenced as a topic subject.
3. Most recently modified file in `<topics_dir>/`.

Multiple matches → same branch as above. Zero → `❌ No topic detected. Pass a topic identifier or name.` STOP.

Print `Detected topic: <name>` on the first line, before any state mutation. The name is the page basename without `.md` — that is the value the agent matches against `topics:`.

### 4. Invoke topic-manager-agent

```
Task tool with:
  subagent_type: 'vault-cli:topic-manager-agent'
  prompt: 'ACTION: update\nARGS: <absolute page path>\nMODE: <interactive|tool>'
```

The agent computes the membership diff, asks for confirmation, applies the `## Goals` additions, refreshes the status tokens, re-dates `# Current Reading` and appends the `# Progress` entry.

### 5. Done

The command prints the agent's report. Under `MODE: tool` the agent prints the proposed additions and writes nothing.

## Notes

- **Confirmation lives in the agent** — it holds `AskUserQuestion` and asks before writing; declining leaves `## Goals` unchanged while the proposal is still printed.
- **Never removes a listed goal.** Cross-topic goals already listed are left alone.
- **Writes no counters** — `# Status Summary` stays hand-written per the vault's Topic Writing Guide.
- Membership reads the frontmatter `topics:` field via `scripts/update-topic-taggers.sh`, which `scripts/update-topic-taggers-test.sh` pins — never a whole-file grep, which would report goals that merely mention the topic.
- Vault detection via `vault-cli config list` + `$PWD`; `topics_dir` / `goals_dir` come from the matched vault config.
- Sibling commands: `/vault-cli:update-goal`, `/vault-cli:update-task`, `/vault-cli:execute-topic`
