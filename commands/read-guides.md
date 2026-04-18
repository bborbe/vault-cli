---
description: Read all vault-cli + vault writing guides for context before working
allowed-tools: [Read, Glob, Bash]
---

Read all vault-cli docs and vault writing guides to build full context. Use this before creating/editing tasks, goals, objectives, themes, or visions.

## Step 1: Read vault-cli docs

Glob `~/Documents/workspaces/vault-cli/docs/*.md` and Read every file returned. These cover development patterns and Definition of Done.

Also Read `~/Documents/workspaces/vault-cli/CLAUDE.md` if present — project-specific instructions.

## Step 2: Read vault hierarchy writing guides

Read these from the Personal vault (`~/Documents/Obsidian/Personal/50 Knowledge Base/`):

- `Vision Writing Guide.md`
- `Theme Writing Guide.md`
- `Objective Writing Guide.md`
- `Goal Writing Guide.md`
- `Task Writing Guide.md`
- `Guide Writing Guide.md` (meta — how to write guides)

These define structure, frontmatter, and quality criteria for each level of the hierarchy.

## Step 3: List vault-cli commands

Glob `~/Documents/workspaces/vault-cli/commands/*.md` — list filenames only. These are the available slash commands (audit-goal, verify-task, defer-task, etc.).

Run `vault-cli --help` and `vault-cli task --help` to confirm current CLI surface.

## Step 4: Summarize

Report:
- **vault-cli workflow rules** — key patterns from docs + CLAUDE.md
- **Hierarchy guides loaded** — one-line per level (vision/theme/objective/goal/task)
- **Available commands** — filenames grouped by function (audit/verify/manage)
- **Confirm readiness to work**
