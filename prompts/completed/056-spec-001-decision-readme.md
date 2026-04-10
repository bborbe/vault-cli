---
status: completed
spec: [001-decision-list-ack]
summary: Rewrote README Usage section to comprehensively document all commands across task, goal, theme, objective, vision, decision, search, and config groups.
container: vault-cli-056-spec-001-decision-readme
dark-factory-version: v0.54.0
created: "2026-03-16T00:00:00Z"
queued: "2026-03-16T10:36:41Z"
started: "2026-03-16T10:57:21Z"
completed: "2026-03-16T11:01:23Z"
branch: dark-factory/decision-list-ack
---

<summary>
- README.md is rewritten to document ALL available commands, not just the original task shortcuts
- Commands are organized by noun (task, goal, theme, objective, vision, decision, search, config)
- Each command group shows the most useful subcommands with example usage
- The overview and description are updated to reflect the full scope of the tool
</summary>

<objective>
Rewrite the README.md Usage section to comprehensively document all available commands. The current README only shows `complete`, `defer`, and `update` — it is missing task list/show/get/set/clear/watch/work-on/lint/validate/search, all goal/theme/objective/vision subcommands, global search, config commands, and the new decision commands.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `README.md` — current state only documents 3 task commands.
Read `pkg/cli/cli.go` — scan all `Use:` fields to find every command and subcommand. Note the `Short:` descriptions for each.
</context>

<requirements>
1. Update description (line 7) to: "Go CLI tool for managing Obsidian vault tasks, goals, themes, objectives, visions, and decisions."

2. Update overview (line 11) to: "Fast CRUD operations for Obsidian markdown files (tasks, goals, themes, objectives, visions, decisions) without spawning full Claude Code sessions."

3. Replace the Usage section with a comprehensive command reference. Organize by noun. For each noun, show the most useful subcommands with brief examples. Use `vault-cli <noun> <verb>` format consistently.

   Cover ALL command groups discovered in `pkg/cli/cli.go`:
   - `task` (complete, defer, update, list, show, get, set, clear, work-on, watch, lint, validate, search)
   - `goal` (list, lint, search)
   - `theme` (list, lint, search)
   - `objective` (list, lint, search)
   - `vision` (list, lint, search)
   - `decision` (list, ack) — the new commands from this spec
   - `search` (global vault search)
   - `config` (list, current-user)

4. Keep examples concise — 1-2 examples per subcommand is enough. Group related commands together.

5. Keep the global flags section showing `--vault`, `--output`, `--config`.

6. Preserve the existing Installation, Configuration, Development, and License sections unchanged.
</requirements>

<constraints>
- Only modify README.md — no code changes
- Keep the existing badges, Installation, Configuration, Development, and License sections as-is
- Do NOT invent commands that don't exist — only document commands found in `pkg/cli/cli.go`
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
Run `make precommit` — must pass (README changes should not affect tests).
</verification>
