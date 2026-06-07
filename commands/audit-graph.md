---
description: Audit Obsidian vault link-graph topology — broken wikilinks, orphan pages, and de facto MOC hubs. Topic-scoped or full-vault.
argument-hint: "[topic]"
allowed-tools: Task
---

## Context

- Active vault config: !`vault-cli config list --output json`

<objective>
Run the `graph-auditor` agent against the active Obsidian vault. Two modes:
- **Topic mode** — when `$ARGUMENTS` is non-empty, scope the audit to pages semantically related to that topic.
- **Full-vault mode** — when `$ARGUMENTS` is empty, audit the whole vault.
</objective>

<process>
1. Parse `$ARGUMENTS`:
   - Non-empty → topic mode; pass the full argument string as the topic
   - Empty → full-vault mode
2. Invoke the `graph-auditor` agent via Task tool:
   - `subagent_type: "graph-auditor"`
   - `prompt: "Audit graph topology. Topic: <topic-or-empty>"`
3. Present the agent's report verbatim.
</process>

<success_criteria>
- Agent invoked successfully and report presented to user
</success_criteria>
