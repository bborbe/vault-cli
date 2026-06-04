---
status: verifying
approved: "2026-06-04T10:24:49Z"
generating: "2026-06-04T10:26:38Z"
prompted: "2026-06-04T10:41:45Z"
verifying: "2026-06-04T10:49:56Z"
branch: dark-factory/work-on-task-move-create-gate-to-slash-command
---

## Summary

- `vault-cli:work-on-task-assistant` agent silently creates Obsidian task files when the requested task is missing, despite its own contract (`<constraints>: ASK before creating`, Phase 1 "Task not found: AskUserQuestion → ...") requiring an explicit user "Yes" first.
- Root cause is architectural: the agent owns BOTH the search and the create-on-miss decision, with `Skill` in its `tools:` frontmatter — the consent gate is enforced only by prompt text. A caller phrasing like "offer to create if missing" (from the controlling slash command or a parent agent) is enough to bypass the text-only gate.
- Fix is a Single Responsibility split on **task creation only** — the agent keeps its existing responsibilities (search, Jira assign + transition, Obsidian `status: in_progress`, daily-note tracking, guide discovery) but stops being able to create new task files. It emits a structured `not_found` verdict when no task matches.
- The slash command (`commands/work-on-task.md`) becomes the controller for the absence case: on `not_found`, it asks the user via AskUserQuestion in the main session, on Yes invokes `Skill: vault-cli:create-task`, on success re-invokes the agent against the newly created task so the standard prep mutations run.
- The agent loses the `Skill` and `Task` tools from its frontmatter — architecturally, it CAN NO LONGER create tasks. The consent gate is enforced by capability removal, not by prompt discipline. The other tools (`Read`, `Glob`, `Bash`, `Edit`, `AskUserQuestion`, MCP search/atlassian) stay — the existing happy-path mutations still need them.

## Problem

A real run this session: a user invoked `/vault-cli:work-on-task "Semantic search with Python 3.13"`. The task did not exist in the vault. The slash command delegated to `vault-cli:work-on-task-assistant`. The agent searched, found nothing, then silently invoked `Skill: vault-cli:create-task` and reported `Task created and tracked.` — without ever calling AskUserQuestion. The user only discovered the task had been auto-created when checking the output report.

The agent's own definition documents the expected behavior:

- `agents/work-on-task-assistant.md` `<constraints>`: `ASK: before creating a new Obsidian task file`
- `agents/work-on-task-assistant.md` Phase 1, "Task not found": `AskUserQuestion → "Create new task?" — Yes invokes Skill: vault-cli:create-task; No shows manual search tips and STOPS`

The gate exists as prose. The agent (running on Sonnet) skipped it. Phrasing in the calling slash command's prompt to the agent ("search the vault and offer to create it if missing" from the work-on-task slash command's delegate prompt template) was sufficient to override the gate — the model read "offer to create" as license to act, not as an obligation to ask first.

This is the second class of consent failure in vault-cli (the first being spec 014's `claude_session.go` silent-success: an agent reports completion when in fact a precondition was unmet). The shape is identical: a contract enforced only by docstrings/prose is fragile against caller-side phrasing pressure.

## Goal

`vault-cli:work-on-task-assistant` cannot create Obsidian task files — its `tools:` frontmatter excludes `Skill` and `Task`. Its existing responsibilities are unchanged: search across Jira/Obsidian/daily-note/semantic, assign the Jira issue to the current user and transition it to "In Progress", set the Obsidian task's `status: in_progress`, track the task on today's daily-note as `[/]`, surface relevant guides/runbooks. When the agent cannot find the requested task in any source, it emits a structured `not_found` verdict in its report and stops without proposing a fix.

`commands/work-on-task.md` slash command handles the `not_found` case in the main Claude session: shows the user what was searched (Jira, daily note, free-text/semantic, file globs), proposes a task name derived from the input argument, asks via AskUserQuestion (single yes/no), and on `Yes` invokes `Skill: vault-cli:create-task` with the suggested name. On `No`, the slash command prints manual search tips and stops.

After the new task is created, the slash command re-invokes `vault-cli:work-on-task-assistant` with the new task title so the standard prep mutations (Jira sync, status set, daily-note tracking, guide discovery) run against the just-created task.

## Non-goals

- Touching any other vault-cli agent's tool list — only `work-on-task-assistant.md` loses `Skill` and `Task`. Other agents that legitimately need creation (`vault-cli:create-task` itself, `vault-cli:task-manager-agent`) keep their tools.
- Replacing AskUserQuestion with a different consent mechanism (CLI prompt, env-var override) — AskUserQuestion is the documented main-session UX channel in this codebase; preserve it.
- Adding any new slash command — `vault-cli:create-task` already exists with an interactive create flow; the new behavior just routes to it.
- Auto-deriving a parent goal / priority / category for the new task — `vault-cli:create-task` handles those interactively. The slash command only suggests a *name*; the create-task skill asks the rest.
- Migrating in-flight callers of the agent — there are no other slash commands or agents in vault-cli that invoke `vault-cli:work-on-task-assistant`; only `commands/work-on-task.md` does. Confirmed via `grep -r "work-on-task-assistant" commands/ agents/`.
- Changing the verdict format the agent emits when a task IS found — the existing `output_format` for found tasks stays as-is. Only the `not_found` case gets a new structured form.
- Implementing a regression test that exercises a real Claude sub-agent invocation — agent behavior is not unit-testable. Coverage is via spec verification (scenario walkthrough) at PR time, not Go tests.

## Do-Nothing Option

Leaving the bug costs user trust in the consent gate. Every `/vault-cli:work-on-task` call against an unknown title silently materialises a task — possibly with a name the user would have phrased differently, possibly under a wrong parent goal, definitely without the user's explicit Yes. The cost per occurrence is small (correcting or deleting the auto-created task takes < 1 min), but the cumulative trust cost is structural: a "I'll ask before changing the vault" contract that the system silently breaks erodes the user's willingness to delegate.

Hardening the prompt text (stronger `MUST ASK` language, repeated in three places) is the obvious cheaper fix. It does not work: spec 014 showed that prose-only contracts fail under caller-side phrasing pressure, and the consent gate in this agent already has the strongest possible prose ("MANDATORY mutations" in `<critical_writes>`, ASK in `<constraints>`, Phase 1 explicit AskUserQuestion call). The model still skipped it.

Capability removal (`Skill` out of `tools:`) is the only fix that scales: the agent literally cannot invoke `Skill: vault-cli:create-task` because the tool isn't available in its session. No future phrasing change in any caller can bypass that.

Cost of fix is low: two markdown files (`agents/work-on-task-assistant.md`, `commands/work-on-task.md`). No Go code changes, no test rewrites, no migration. The change is fully reversible — if the SRP split turns out to be the wrong design, restoring `Skill` to `tools:` is a one-line revert.

## Reproduction

vault-cli HEAD: master at time of filing (493eb1f, "bump deps and go 1.26.4").

The bug fires whenever `/vault-cli:work-on-task <unknown-title>` runs against a vault that does not contain that title. Concrete trace from this session:

```
/vault-cli:work-on-task "Semantic search with Python 3.13"
   → slash command delegates to vault-cli:work-on-task-assistant with prompt
     "Find details and guides for: Semantic search with Python 3.13 ...
      search the vault and offer to create it if missing ..."
   → agent runs Phase 1: Jira lookup (n/a), daily-note grep (miss), semantic search (no hit), Glob (no file)
   → agent SKIPS the documented AskUserQuestion call
   → agent invokes Skill: vault-cli:create-task with derived name "Relax Python Version Floor in semantic-search to 3.13"
   → task file created at 24 Tasks/Relax Python Version Floor in semantic-search to 3.13.md
   → agent emits report "Task created and tracked."
```

The user only saw the consent failure after reading the report and asking "did work on task find the task ? guess we have non yet?" — at which point the task was already on disk.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| Agent tools when task missing | Cannot invoke `Skill: vault-cli:create-task` (tool not in `tools:`) | Invokes it directly |
| Consent gate enforcement | Architectural (capability removal) | Prose-only (`<constraints>` + `<critical_writes>` + Phase 1 text) |
| Agent verdict on miss | Structured `not_found` form with what-was-searched evidence | "Task created and tracked." |
| Who runs AskUserQuestion | Slash command (main session, user-facing) | Sub-agent (model decides whether to call it) |
| New-task naming | User confirms via `vault-cli:create-task` interactive flow | Agent auto-derives from input |
| Re-discovery after create | Slash command re-invokes agent against new task | Agent assumes happy path after silent create |

## Why this is a bug

The contract documented in `agents/work-on-task-assistant.md` says the agent asks before creating. The agent does not ask. The user's consent is bypassed. That's the bug.

The deeper bug is the design that makes the contract violable. The agent holds both the search responsibility (legitimate) and the creation responsibility (illegitimate — should belong to the controller). The Skill tool that enables the creation lives in the agent's frontmatter. The consent gate is the only thing standing between an unknown-task input and a materialised file, and it is implemented as prose. A model running the agent has full capability to skip the prose-gate any time a caller's phrasing nudges it toward action.

Fixing only the prose (e.g., promoting the constraint to a louder `🚨 BLOCKING:` block) addresses the symptom and not the cause. The cause is that the agent CAN create at all.

## Constraints

- `commands/work-on-task.md` happy path (task found) MUST NOT change — current behaviour ("delegates to agent, reports `Ready to work on this task.`") is correct for the found case.
- `agents/work-on-task-assistant.md` Phases 2–8 (Jira mutations, Obsidian status set, daily-note tracking, code-task guide discovery, runbook/guide search, mutation verification) MUST NOT change — those are correct mutations the agent owns.
- `vault-cli:create-task` skill MUST NOT change — its interactive create flow (asks title, parent goal, priority, category, defer date, …) is what we want to route to.
- No new MCP, no new tool, no new dark-factory primitive — the change is two `.md` edits.
- The slash command must continue to work for callers that pass a Jira-style ID (`[A-Z]+-\d+`). On `not_found` for a Jira ID, propose the task name derived from the Jira summary if `JIRA_MCP_AVAILABLE`, else fall back to the raw ID. The agent's existing Jira lookup output supplies the summary.

## Failure Modes

| Trigger | Detection | Expected behavior | Recovery |
|---|---|---|---|
| Agent run against unknown title | Agent Phase 1 search returns 0 hits across all sources | Agent emits `not_found` verdict in report; agent stops without further mutation | Slash command, on parse, asks user via AskUserQuestion; on Yes invokes `vault-cli:create-task`; on No prints manual search tips |
| User answers No to "Create new task?" | AskUserQuestion result | Slash command prints manual search tips (existing copy from agent Phase 1, moved up to the command); no task is created | Done |
| User answers Yes | AskUserQuestion result | Slash command invokes `Skill: vault-cli:create-task` with suggested name; on success re-invokes `vault-cli:work-on-task-assistant` with the new task title | Standard work-on-task flow runs against new task |
| Agent retains stale `Skill` capability (regression) | `grep "^- Skill" agents/work-on-task-assistant.md` returns ≥1 line in frontmatter | Spec verification fails; revert or correct the frontmatter | Manual fix |
| Caller is another agent (not the slash command) that does not know about the new verdict | Caller parses old `output_format` only | Caller sees an unfamiliar verdict block, no task is created, agent stops cleanly | Update caller separately — non-goal for this spec |

## Acceptance Criteria

- [ ] `agents/work-on-task-assistant.md` `tools:` frontmatter does NOT include `Skill` — evidence: `grep -nE '^tools:' agents/work-on-task-assistant.md` and the value on that line + any wrap continuations contain no `Skill`. The remaining tools (`Read`, `Glob`, `Bash`, `Edit`, `AskUserQuestion`, `Task`, `mcp__semantic-search__search_related`, `mcp__atlassian__*`) stay. **Amendment (2026-06-04, during PR #8 bot review at HEAD `e27f49e`)**: the original AC required removing both `Skill` AND `Task`. Bot review correctly flagged that Phase 5 (`coding:check-guides` lookup) and Phase 7 (`vault-cli:task-manager-agent` subagent dispatch) need `Task`. The spec's intent was creation-only enforcement, not removal of all dispatch primitives — `Task` is a generic dispatch tool that does not grant create-task capability on its own. `Skill` removal is the load-bearing architectural block (it was the path to `Skill: vault-cli:create-task`); `Task` is retained with a `<constraints>` entry restricting its dispatch to documented subagent types (`coding:pre-implementation-assistant`, `vault-cli:task-manager-agent`).
- [ ] `agents/work-on-task-assistant.md` Phase 1 "Task not found" branch is rewritten: instead of `AskUserQuestion → "Create new task?"`, it now emits a structured `not_found` verdict with the searched-source evidence (Jira: yes/no/skipped, daily-note: hit/miss, semantic-search: top-3 misses, Glob: paths tried) and STOPS — evidence: the new Phase 1 text contains no `AskUserQuestion` call and contains the literal string `not_found`.
- [ ] `agents/work-on-task-assistant.md` `<output_format>` adds a `not_found` form (alongside the existing `found` form) — evidence: the section contains both `found:` and `not_found:` headers (or equivalent structured markers) with the not-found form including a `Searched:` evidence list and a `Suggested task name:` line.
- [ ] `agents/work-on-task-assistant.md` `<constraints>` removes the `ASK: before creating a new Obsidian task file` line — evidence: `grep -n 'ASK: before creating' agents/work-on-task-assistant.md` returns 0 lines. The remaining ASK rules (Jira/Obsidian status) stay.
- [ ] `agents/work-on-task-assistant.md` `<critical_writes>` no longer mentions task creation as a mutation — evidence: the `<critical_writes>` block lists only the Jira and Obsidian status mutations; no mention of `Skill: vault-cli:create-task` anywhere in the section.
- [ ] `commands/work-on-task.md` adds a `## Phase 4 — Handle not_found` section AFTER the existing `## Process` invocation of the agent — evidence: `grep -n 'Phase 4 — Handle not_found\|Phase 4 - Handle not_found' commands/work-on-task.md` returns ≥1 line.
- [ ] The new Phase 4 in `commands/work-on-task.md` describes: (a) parse the agent's report for the `not_found` form; (b) derive a suggested task name from the argument or Jira summary; (c) call `AskUserQuestion` with a single Yes/No question naming the suggested title; (d) on Yes, invoke `Skill: vault-cli:create-task` with the suggested name as the seed; (e) on success, re-invoke `vault-cli:work-on-task-assistant` with the new title; (f) on No, print manual search tips and stop — evidence: each of (a)–(f) appears as a distinct bullet or numbered step in Phase 4.
- [ ] `commands/work-on-task.md` `## Notes` (or equivalent) explains the architectural split — one sentence: "The agent searches; the slash command asks before creating." — evidence: that sentence (or close paraphrase containing both "agent" and "slash command" and "before creating") appears in the file.
- [ ] No other `commands/*.md` or `agents/*.md` files are modified — evidence: `git diff --name-only origin/master..HEAD -- commands/ agents/` returns exactly two paths: `commands/work-on-task.md` and `agents/work-on-task-assistant.md`.
- [ ] No Go files are modified — evidence: `git diff --name-only origin/master..HEAD -- '*.go'` returns 0 lines.
- [ ] **Runtime repro — not_found path**: in this worktree, with the post-edit agent/command, run `/vault-cli:work-on-task "Definitely Not A Real Vault Task XYZ-impossible-string"`. Evidence: the slash command produces an AskUserQuestion to the user (intercepted at audit time — the actual interactive run happens at PR verify, not in the daemon container). The not-found verdict from the agent is visible in the transcript before the AskUserQuestion fires. No task file is materialised under `~/Documents/Obsidian/Personal/24 Tasks/` before the user answers. **Verifier MUST NOT mark this AC complete based on dry-run / static-analysis / file-diff inspection alone — verifier MUST observe a real session transcript with the agent's `not_found` verdict block visible AND the slash command's AskUserQuestion prompt visible.**
- [ ] **Runtime repro — found path** (regression check): on a known-existing task, e.g. `/vault-cli:work-on-task "Try Hermes Agent harness"` against the Personal vault, the existing happy path (Phase 2–8) runs unchanged. Evidence: the report ends with `Ready to work on this task.` and the task's daily-note tracking + `status: in_progress` are correct.
- [ ] `make precommit` exits 0.
- [ ] CHANGELOG.md has a `## Unreleased` entry covering both file changes — evidence: `grep -nE '^## Unreleased' CHANGELOG.md` returns ≥1 line AND `grep -nE 'work-on-task|ask gate|consent gate|SRP' CHANGELOG.md` returns ≥1 line in the section above the next `## v` header. (vault-cli's release driver is the github-releaser-agent watcher via `.maintainer.yaml: release.autoRelease: true` — it renames `## Unreleased` → `## vX.Y.Z` post-merge on master; on the feature branch the section persists as `## Unreleased`.)

## Verification

```bash
cd ~/Documents/workspaces/vault-cli-ask-gate
make precommit          # full check; no Go changes so should be fast
grep -nE '^tools:' agents/work-on-task-assistant.md         # confirm no Skill/Task
grep -n 'not_found'  agents/work-on-task-assistant.md       # confirm new verdict
grep -n 'Phase 4'    commands/work-on-task.md               # confirm new phase
git diff --name-only origin/master..HEAD -- commands/ agents/   # exactly 2 files
git diff --name-only origin/master..HEAD -- '*.go'             # 0 files
```

Runtime checks (manual, at PR verify time):

```bash
# Not-found path (in main Claude Code session, this worktree's vault config):
/vault-cli:work-on-task "Definitely Not A Real Vault Task XYZ-impossible-string"
# Expect: agent emits not_found verdict; slash command asks user via AskUserQuestion; no task created until user answers Yes.

# Found path (regression):
/vault-cli:work-on-task "Try Hermes Agent harness"
# Expect: standard Phase 2–8 flow; report ends with "Ready to work on this task."
```

## Open Questions

- None on the architectural split. The remaining question is the *exact* form of the `not_found` verdict block — whether it should be loose YAML, a bulleted list, or a fenced markdown block. The prompt-generation step can pick the form; the spec only requires that `not_found` is present and includes Searched: + Suggested task name:. Leaving the surface form to the prompt is intentional — it avoids over-specifying.
