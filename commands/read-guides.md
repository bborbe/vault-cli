---
description: Read all vault-cli + vault writing guides for context before working
allowed-tools: Read, Glob, Bash(vault-cli:*), Bash(ls:*)
---

Read all vault-cli docs and vault writing guides to build full context. Use this before creating/editing tasks, goals, objectives, themes, or visions.

## Step 1: Read vault-cli docs

Use Bash to list docs (Glob does NOT expand `~`):
```
ls ~/Documents/workspaces/vault-cli/docs/*.md
```

Read every file listed. These include:

- `goal-writing.md` — canonical goal structure, Non-goals convention, Goal Scope Fit smells
- `task-writing.md` — canonical task structure, Out-of-Scope convention, scope-check smells
- `development-patterns.md`, `dod.md`, `releasing-vault-cli.md` — tooling internals

Also Read `~/Documents/workspaces/vault-cli/CLAUDE.md` if present — project-specific instructions.

## Step 2: Read vault hierarchy writing guides

Read these from the Personal vault (`~/Documents/Obsidian/Personal/50 Knowledge Base/`) for vault-specific examples:

- `Vision Writing Guide.md`
- `Theme Writing Guide.md`
- `Objective Writing Guide.md`
- `Goal Writing Guide.md` (vault examples; structural rules in `vault-cli/docs/goal-writing.md`)
- `Task Writing Guide.md` (vault examples; structural rules in `vault-cli/docs/task-writing.md`)
- `Guide Writing Guide.md` (meta — how to write guides)

These provide example-rich context; the vault-cli docs in step 1 are the canonical structural contracts.

## Step 3: List vault-cli commands

Use Bash to list commands (Glob does NOT expand `~`):
```
ls ~/Documents/workspaces/vault-cli/commands/*.md
```

These are the available slash commands (audit-goal, verify-task, defer-task, etc.).

Run `vault-cli --help` and `vault-cli task --help` to confirm current CLI surface.

## Step 4: Summarize

Report:
- **vault-cli workflow rules** — key patterns from docs + CLAUDE.md
- **Hierarchy guides loaded** — one-line per level (vision/theme/objective/goal/task)
- **Available commands** — filenames grouped by function (audit/verify/manage)
- **Confirm readiness to work**
