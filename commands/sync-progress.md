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
JIRA_MCP_AVAILABLE    = mcp__atlassian__getJiraIssue tool present
JIRA_CLOUD_ID         = first id returned from mcp__atlassian__getAccessibleAtlassianResources (cached for session)
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
1. `mcp__atlassian__getJiraIssue(cloudId=JIRA_CLOUD_ID, issueIdOrKey=<key>)` → current status
2. If conversation indicates completion AND ticket status != Done:
   - `getTransitionsForJiraIssue(...)` → find "Done" (case-insensitive)
   - `transitionJiraIssue(...)` → transition
   - Optionally `addCommentToJiraIssue(...)` with the summary

If JIRA_MCP_AVAILABLE is false: skip silently.

### 3.5 Track updated files

Store `UPDATED_FILES = [paths]` for Phase 4.

## Phase 4: Mark tasks complete

Skip the user-confirmation prompt when the evidence is unambiguous; only ask when something is fuzzy.

### 4a. Auto-complete (no AskUserQuestion) — strict objective criteria

Auto-complete by calling `vault-cli task complete "{name}"` directly if AND ONLY IF ALL of the following hold:

1. **Success Criteria present and fully ticked.** Task file contains a `# Success Criteria` (or `## Success Criteria`) heading AND every checkbox between it and the next `^#` heading is `[x]`. Zero `[ ]` and zero `[/]` in that section.
2. **No incomplete checkboxes anywhere in the file.** `grep -E '^\s*-\s+\[[ /]\]' <task-file>` returns zero lines.
3. **Verification evidence documented in the file.** At least ONE of:
   - A `# Results` (or `## Results (YYYY-MM-DD)`) section exists with non-empty content
   - A `# Pull Requests` section exists with at least one PR link
   - This `/vault-cli:sync-progress` run is itself about to add such a section (see Phase 3) AND the conversation explicitly cites a shipped artifact: a released version (`vX.Y.Z`), a merged/closed PR URL, a successful scenario replay, a successful integration test run, or equivalent objective shipping signal
4. **No unresolved blockers in conversation.** The conversation does NOT contain phrases like "still need to", "TODO before complete", "blocked on", "follow-up required for this task", "not yet done", "skip for now", or a deferred AC. Follow-up items filed AS SEPARATE specs/tasks/ideas do NOT count as blockers — they explicitly off-scope themselves.

If all 4 hold, call `vault-cli task complete` directly. Report it in Phase 5. Do NOT ask.

### 4b. Confirmed-complete (AskUserQuestion required)

If criteria 1–4 do NOT all hold but the conversation still signals completion (e.g. all checkboxes ticked but no Success Criteria section; or verification was discussed but not documented), use `AskUserQuestion`:

```
Question: "All N/N checkboxes ticked. Mark <task> as completed?"
Options: "Yes — mark completed" | "Hold — keep as in_progress"
```

If "Yes" → invoke `Skill: vault-cli:complete-task`.

### When NOT to mark complete

- Task is not 100% checked → never mark complete, never ask. (Phase 3 still updates progress.)
- Conversation contains an unresolved blocker for this specific task → never auto-complete; ask the user how to proceed.
- The user explicitly said "update progress" or "sync" (not "complete") AND the file has no Success Criteria block → skip the completion phase entirely.

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
