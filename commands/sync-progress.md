---
description: Synchronize task progress documentation from completed work in this conversation — daily note, task pages, PR + Jira links (gracefully)
allowed-tools:
  - Read
  - Edit
  - Grep
  - Glob
  - Bash(vault-cli:*)
  - Bash(grep:*)
  - Bash(command -v:*)
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
1. `mcp__atlassian__getJiraIssue(cloudId=JIRA_CLOUD_ID, issueIdOrKey=<key>)` → current status. If the ticket does not exist (404 / not accessible) → skip silently.
2. **Always** post a progress comment via `addCommentToJiraIssue(...)`. Same content as Phase 3.1's daily-note section (summary + key results + decisions + PR links), as Jira markdown. Deduplicate: if the last comment on the ticket already contains the same headline summary and a timestamp within the last hour, skip — avoids double-posting on re-runs of `/vault-cli:sync-progress`.
3. If conversation indicates completion AND ticket status != Done:
   - `getTransitionsForJiraIssue(...)` → find "Done" (case-insensitive)
   - `transitionJiraIssue(...)` → transition
   - The comment from step 2 stands as the completion record — no second comment needed.

If JIRA_MCP_AVAILABLE is false: skip silently.

### 3.5 Track updated files

For each file written in Phase 3.1–3.4, record a structured record (in memory) for Phase 5:

- `path` — absolute file path
- `vault` — vault name (basename of the matching `vault.path` from `vault-cli config list`)
- `relpath` — file path minus the vault path, no leading slash, no `.md` suffix
- `link` — `obsidian://open?vault=<vault>&file=<percent-encoded relpath>`. Percent-encode every character in `relpath` that is NOT in the unreserved set `[A-Za-z0-9-_.~]`. Common cases: space → `%20`, `/` → `%2F`, em-dash `—` → `%E2%80%94`, `+` → `%2B`, `%` → `%25`, `:` → `%3A`, `&` → `%26`, `?` → `%3F`, `#` → `%23`. NEVER encode the literal `?` or `=` separators between query-string keys. The `vault` value follows the same rule.
- `title` — basename of the file without `.md`
- `category` — one of `daily` | `task` | `goal` | `runbook` | `doc`, classified by ancestor directory:
  - matches `vault.daily_dir` → `daily`
  - matches `vault.tasks_dir` → `task`
  - matches `vault.goals_dir` → `goal`
  - path contains `/65 Runbooks/` or `/70 Runbooks/` → `runbook`
  - else → `doc`
- `section` — the h2 section name where content landed (e.g. `What happened today`, `Pull Requests`, `Results`); empty if the whole file is new

Phase 5 reads these structured records to emit clickable links — do not skip the schema and feed Phase 5 raw paths.

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

Output a concise summary. **Every updated file is rendered as a clickable `obsidian://` link** built from the Phase 3.5 records — wikilinks aren't clickable in chat, raw paths aren't openable.

Grouping order: `Daily` → `Task` → `Goal` → `Runbook` → `Doc`. Omit any group with zero entries. One bullet per file.

```markdown
🔄 Synced progress for {Task / PR-only / multiple}

Updated:
- Daily: [{title}]({link})
- Task: [{title}]({link}) — {section}
- Goal: [{title}]({link}) — {section}
- Runbook: [{title}]({link}) — {section}
- Doc: [{title}]({link}) — {section}

PRs: [<org>/<repo>#<N>](<url>)            ← only if any
Jira: <KEY> → Done                         ← only if any
Decisions: {if any}                        ← only if any
Completed: [{title}]({link})               ← only if Phase 4 auto-completed or user said Yes
```

Rules:
- Use the structured `link` from Phase 3.5 — do NOT hand-roll `obsidian://` URLs in Phase 5
- Drop the trailing `— {section}` if `section` is empty
- Never invent links — only emit links for files actually written this run

Worked example:

```markdown
🔄 Synced progress for Reclaim Disk Space on nuke-k3s-dev-0

Updated:
- Daily: [2026-05-24](obsidian://open?vault=Personal&file=60%20Periodic%20Notes%2FDaily%2F2026-05-24)
- Task: [Reclaim Disk Space on nuke-k3s-dev-0 — MT5 Bases Cache + BoltDB Growth 2026-05](obsidian://open?vault=Personal&file=24%20Tasks%2FReclaim%20Disk%20Space%20on%20nuke-k3s-dev-0%20%E2%80%94%20MT5%20Bases%20Cache%20%2B%20BoltDB%20Growth%202026-05) — Verification
- Goal: [Reduce Trading BoltDB Disk Footprint by 40%](obsidian://open?vault=Personal&file=23%20Goals%2FReduce%20Trading%20BoltDB%20Disk%20Footprint%20by%2040%25) — Tasks
- Runbook: [DiskOutOfSpace Nuke Host Volume Expansion](obsidian://open?vault=Personal&file=65%20Runbooks%2FDiskOutOfSpace%20Nuke%20Host%20Volume%20Expansion) — Expansion History

Completed: [Reclaim Disk Space on nuke-k3s-dev-0 — MT5 Bases Cache + BoltDB Growth 2026-05](obsidian://open?vault=Personal&file=24%20Tasks%2FReclaim%20Disk%20Space%20on%20nuke-k3s-dev-0%20%E2%80%94%20MT5%20Bases%20Cache%20%2B%20BoltDB%20Growth%202026-05)
```

If the `Completed:` task is already listed under `Task:` above, omit the `Completed:` line to avoid duplicate links — the report is for at-a-glance; the auto-complete is implied by Phase 4's separate console output.
