---
description: Create a new task in the configured vault with guided prompts
argument-hint: "[task description] [--docs-only|--code] [--non-interactive] [--vault NAME]"
allowed-tools: [Task, AskUserQuestion]
---

Create one task file in the vault. Which agent performs the write depends on whether the task is a docs-only change, so resolve that first.

## Parse arguments

- `--docs-only` → `ROUTE=docs` (explicit)
- `--code` → `ROUTE=code` (explicit)
- `--non-interactive` (deprecated alias: `--tool`) → `MODE=non_interactive` (JSON output, no prompts)
- `--vault NAME` → target a specific vault (otherwise the default vault from `~/.vault-cli/config.yaml`)
- Remaining text → task description / title

## Resolve the route

If **both** `--docs-only` and `--code` were passed, print
`❌ Pass only one of --docs-only / --code — they are mutually exclusive.`
and STOP. Neither branch below runs for that input, so without this guard `ROUTE` is left unassigned and the command dispatches nothing at all.

If neither `--docs-only` nor `--code` was passed:

- **`MODE=interactive`** → ask exactly one `AskUserQuestion`:
  *"Docs-only change (guide / KB page / runbook / note)?"*
  1. **Yes — write the file directly**
  2. **No — full task-creator pipeline**

  Take the answer as `ROUTE`. Do not ask again, and do not ask anything else.

- **`MODE=non_interactive`** → do **not** infer the route from the title. A headless caller cannot answer, and a wrong guess silently takes the wrong path. Print
  `❌ Pass --docs-only or --code — the route cannot be inferred headlessly.`
  and STOP.

## Dispatch

- **`ROUTE=docs`** → dispatch the **`task-writer`** agent.
- **`ROUTE=code`** → dispatch the **`task-creator`** agent.

Pass the parsed arguments — the description, `--vault`, `--non-interactive` — to the chosen agent. Strip `--docs-only` / `--code`; they are routing flags and mean nothing to the agents.

Do NOT chain `/vault-cli:plan-task` — leave the task at `phase: todo` for the operator to approve.
