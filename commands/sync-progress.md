---
description: Synchronize task progress documentation from completed work in this conversation — daily note, task pages, PR + Jira links (gracefully)
allowed-tools:
  - Read
  - Edit
  - Grep
  - Glob
  - Bash
---

Synchronize progress documentation based on completed work in the conversation. Updates the daily note, task/goal pages, and (if integrations are available) records the matching PR and transitions Jira.

This command **must stay inline** — it analyzes the parent conversation for completion signals; a sub-agent cannot see the conversation.

## Core principle

From the Document-Driven Workflow: *update documents during work, not after*. This command automates that "update during work" step to prevent context loss across compactions.

## Runtime detection

```
GH_AVAILABLE          = `command -v gh` exits 0
JIRA_MCP_AVAILABLE    = any mcp__atlassian-*__getJiraIssue tool present
JIRA_CLOUD_ID         = first id returned from mcp__atlassian-*__getAccessibleAtlassianResources (cached)
JIRA_NAMESPACE        = matched atlassian-mcp suffix (e.g. personal, seibert)
```

If any integration is absent, skip its section silently — never error.

## Phase 1: Detect context

Find the active vault:
```bash
vault-cli config list --output json
```

Match cwd against each `path`. If cwd is inside a vault → strong signal. Else scan the conversation for vault-tracked work (`[[Task]]` / `[[Goal]]` wikilinks, daily-note-shaped completions). If neither: `❌ No vault context detected. Run from a vault dir or describe vault-tracked work.` and STOP.

Use `daily_dir`, `tasks_dir`, `goals_dir` from the matched vault.

## Phase 2: Analyze conversation for completion

Detect completion phrases:
- "that's done" / "verification passed" / "completed successfully"
- "finished with X" / "all tests pass"
- "deployed to production" / "shipped" / "released vX.Y.Z"

Implicit indicators:
- User provided final results/metrics
- User said "let's move to next task"
- User confirmed acceptance criteria met

Extract from the conversation:
- What was completed (task name, subtask, verification)
- Key results (metrics, findings, outcomes)
- Timestamp (today, YYYY-MM-DD)
- Blockers / deferred items

If NO completion detected, check whether a PR was created (Phase 3.3 detection rules):
- PR present, no completion → proceed but only run Phase 3.3 (PR-only sync). Report as "PR-only sync."
- Neither PR nor completion → `No task completion or PR detected. Use /update instead for in-progress work.` and STOP.

## Phase 3: Update progress notes

### 3.1 Daily note

File: `{daily_dir}/YYYY-MM-DD.md`. Add to `## What happened today`:

```markdown
### {Task Name} — Done ✅

**{1-2 sentence summary}**

**Key results:**
- {result 1}
- {result 2}

**Files updated:**
- [[File 1]] — {what changed}

**Decisions:**
- {key decision if any}
```

Rules:
- New section at the top of "What happened today" (newest first)
- Use `###` (h3); `##` is reserved for the day's top-level structure
- Quote exact numbers/versions/metrics from the conversation
- 2-3 sentence summary max; link to content pages for full context

### 3.2 Task / goal pages

Only update if the conversation explicitly references a `[[Task]]` or `[[Goal]]`. Find or create `## Results` / `## Progress`:

```markdown
### Results (YYYY-MM-DD)
{summary of findings/metrics}
```

### 3.3 Pull Requests (always record if detected)

Detect PRs:
- `gh pr create` output containing `https://github.com/<org>/<repo>/pull/<N>`
- Any `https://github.com/.../pull/\d+` URL referenced as "the PR" / "opened PR" / "created PR"
- `gh pr view` / `gh pr list` output the user acted on

For each detected PR:
1. Resolve task page (unambiguous match required; otherwise fall back to daily note only)
2. Find or create `## Pull Requests` section on the task page (above `## Results` if present, else near top)
3. Append (do not duplicate): `- [<org>/<repo>#<N>](<url>) — <title> (YYYY-MM-DD)`
4. Also add to daily note's "What happened today": `**PR:** [<org>/<repo>#<N>](<url>)`

Never invent PR URLs — only record ones that appear verbatim in conversation/tool output.

### 3.4 Jira sync (if JIRA_MCP_AVAILABLE)

Detect Jira ticket refs in conversation: `[A-Z]+-\d+`.

For each detected ticket:
1. `mcp__atlassian-{JIRA_NAMESPACE}__getJiraIssue(cloudId={JIRA_CLOUD_ID}, issueIdOrKey=<key>)` → current status
2. If conversation indicates completion AND ticket status != Done:
   - `getTransitionsForJiraIssue(...)` → find "Done" (case-insensitive)
   - `transitionJiraIssue(...)` → transition
   - Optionally `addCommentToJiraIssue(...)` with the summary

If JIRA_MCP_AVAILABLE is false: skip silently.

### 3.5 Track updated files

Store `UPDATED_FILES = [paths]` for Phase 4.

## Phase 4: Mark tasks complete (gated)

If conversation indicates a task is fully complete (all checkboxes done):
- AskUserQuestion → `Mark <task> as completed?`
- If yes: `Skill: vault-cli:complete-task` with the task name

Never auto-complete without confirmation.

## Phase 5: Report

Output a concise summary:

```markdown
🔄 Synced progress for {Task / PR-only / multiple}

Updated:
- {daily_dir}/YYYY-MM-DD.md
- [[Task Page]] — {section}
- {PR recorded on task page if any}
- {Jira ticket transitioned if any}

Decisions: {if any}
```
