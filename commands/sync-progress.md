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
  - Bash(jq:*)
  - Bash(head:*)
  - Bash(git -c core.quotepath=off log:*)
  - Bash(git -c core.quotepath=off diff:*)
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

**Probe the disk before concluding "no completion".** Phrase detection above reads the *recent* conversation. After a `/compact` or a `/branch` the completion can sit outside what was re-read, and only the disk remembers it — so this probe runs **first**, ahead of the PR arm below, and a task it establishes takes the completion path even when a PR was also created. The terminal-non-completion arm further down already asks the disk (*"Resolve the status by probe, never from the conversation"*); the completion arm reading phrases only was the asymmetry. Observed 2026-10-08: a manager session completed `Diagnose the Silent Merge-Candidate Stream on Dev` at ~11:45, ran `/vault-cli:sync-progress` at ~11:58, and the run took the no-completion path and wrote nothing — the miss surfaced only when `session-close` Phase 7 found no daily-note entry.

**Candidate set** — the union of two sets.

- **(a) Tasks this session touched** — this command's own read of the conversation, including any compaction summary's mentions of task files. `session-close` Phase 1 defines touched as *"edits this session"* (CLI mutations included) and carries **no** compaction clause, so reading the summary is this command's own addition, not a reuse of that definition.
- **(b) Tasks this session owns on disk** — every task whose `claude_session_id`, or any `metrics_sessions[].session_id`, names this session's id. This half is disk-resolvable, so it survives a compaction that drops (a). Set (b) does **not** catch a task the session flipped without owning — a manager's status flip on a row with an empty id; set (a) is the source for that case.

Enumerate (b) in two steps, because the list JSON carries only the first field:

```bash
vault-cli --vault <v> task list --all --output json          # filter the output on claude_session_id
grep -l "session_id: <this-session-id>" "<tasks_dir>"/*.md   # both halves: matches claude_session_id too
```

`--all` is required: plain `task list` defaults to todo/in_progress and would miss the `completed` row this probe exists to find.

**Session start** — the first record carrying a top-level `timestamp` in this session's transcript. The earliest records (`agent-color`, `custom-title`) carry none, so a bare `head -1` is wrong — and `head` reports *its own* exit status, masking a `jq` failure upstream:

```bash
jq -r 'select(.timestamp) | .timestamp' ~/.claude/projects/*/<session-id>.jsonl 2>/dev/null | head -1
```

**A non-zero exit, an empty result, or an unparseable value means *not established*** — the rule the terminal-non-completion arm below already states. Do **not** carry on with an empty session start: every candidate then fails test 2 and the probe reports "no completion" as though it had run, which is the silent no-op this block exists to remove. Report the lookup as unverified and **continue to the PR check below**. This block sits *above* that check, so the STOP gate is not the fall-through here — jumping to it would drop a PR record the session legitimately made. The terminal-non-completion arm's identical wording is sound only because that arm is reached *after* the PR check has already declined.

The glob can match more than one file — a resumed session appears under each project dir it ran in — and `head -1` then takes whichever the shell expanded first. When it matches more than one, read each and take the earliest first-timestamped record.

A `/branch` session keeps its parent's history, so its first record is the original start — correct, since the completion lives in that history.

**Flipped this session** — a candidate counts as a completion only when **both** hold:

1. It reads `completed` now:
   ```bash
   vault-cli task get "<T>" status --output json
   ```
2. The flip is visible to the vault's git — the committed history shows `status: completed` arriving after this session started, **or** the working tree carries that change uncommitted. Both halves are needed: `git log --since` filters on **commit** time, and obsidian-git autocommits on a schedule, so a completion made minutes ago may not be in history yet. Without the second half the probe silently no-ops on the very case it targets. Pass the task file's **absolute** path (`<vault.path>/<tasks_dir>/<T>.md`) — git resolves the repository from cwd, so this needs no `cd` and the command is granted none:
   ```bash
   { git -c core.quotepath=off log --since="<session-start>" -p -- "<vault.path>/<tasks_dir>/<T>.md"
     git -c core.quotepath=off diff HEAD -- "<vault.path>/<tasks_dir>/<T>.md"; } | grep -q '^+status: completed$'
   ```
   `diff HEAD` covers staged and unstaged changes alike, so a `status: completed` sitting in either is matched. If cwd is outside the vault repo, git errors — that is the fail-closed case above, so continue to the PR check rather than treating it as "no completion".

A candidate passing both **takes the completion path**: treat it as the detected completion and continue with Phases 3–5. If phrase detection above already established a completion for that same task, the probe adds nothing — record it once, not twice. A candidate reading `completed` but failing test 2 was already finished when this session began — leave it alone. Without that test every side-reference to a finished task becomes a duplicate "Done" entry.

**A set-(a) candidate must have been *edited* this session to pass test 2 — a mention is not enough.** Test 2 filters on commit time, and the scheduled-autocommit rationale works in reverse as well: an earlier session completes a task and leaves it uncommitted, this session starts, obsidian-git then flushes that change, and a session that merely *mentions* the task sees `completed` arriving after its own start. Set (a) admits mentions for **enumeration**; only an edit qualifies it for test 2. Set (b) candidates are owned by this session (`claude_session_id` / `metrics_sessions`), so they need no such check.

**When the probe, not the conversation, established the completion, Phases 3.1 and 4a have no conversation to read.** Phase 3.1's entry shape asks for a summary, *Key results* and *Files updated*; Phase 4a's criteria 3 and 4 ask for verification evidence and the absence of blockers. In the compaction-lost case that motivates this block, none of it is in view. Do **not** invent it, and do **not** write a bare heading: read what the disk does hold — the task's `# Results` / `# Progress` sections and its ticked Success Criteria — and say plainly in the entry that the completion was established by probe after a compaction, so a reader knows the summary's provenance. If the disk holds nothing either, write the heading and a one-line statement of that fact; an honest thin entry beats a fabricated full one.

**Known limit, stated rather than hidden:** obsidian-git commits do not record *which* session made a change, so a different session flipping the same file after this session started passes test 2. Set (a)/(b) membership is what keeps that case out.

If no candidate passes both, continue to the PR check below.

If NO completion detected, check whether a PR was created (Phase 3.3 detection rules):
- PR present, no completion → proceed but only run Phase 3.3 (PR-only sync). Report as "PR-only sync." This arm deliberately wins over the terminal-non-completion arm below: a session that both opened a PR and was aborted records the PR, and the outcome is not duplicated.
- Anchor ended in a **terminal non-completion** (`status: aborted` or `hold`) → proceed with Phase 3.1 (daily note) only, heading the entry with the outcome — `### [[Task Name]] — Aborted (superseded)`, wikilink mandatory per Phase 3.1's entry shape — and skipping Phase 4, since there is no completion to mark. **Resolve the status by probe, never from the conversation:** run `vault-cli task get "<anchor task>" status --output json` and branch on the parsed `value`, exactly as `session-close` Phase 4.5 does. A non-zero exit or an unparseable result means *not established* — fall through to the STOP gate below rather than guessing. The conversation is not a sufficient source: the case this arm exists for is an anchor that already ended `aborted` in an earlier session or on another branch, where the transcript carries no signal at all.
- Neither PR nor completion → `No task completion or PR detected. Use /update instead for in-progress work.` and STOP.

## Phase 3: Update progress notes

### 3.1 Daily note

File: `{daily_dir}/YYYY-MM-DD.md`. Add to the daily note's "What happened today" section.

**Resolve the date with `date +%Y-%m-%d` — never from conversation context.** A session can span midnight or be resumed days later, so the date you remember is stale by construction, and the entry lands in a past day's note where the work is invisible on the day it happened. Observed 2026-09-02: an entry was written to `2026-08-30.md` three days late, caught only incidentally because `session-close` Phase 7 resolves the date properly (`TODAY="$(date +%Y-%m-%d)"`) while this command shipped only the `YYYY-MM-DD` placeholder. Same defect class as the stale-copy guard below — stale input, different field.

**Do not assume the heading level.** Templates differ across vaults — the Personal vault uses `# What happened today` (h1). Locate the section with `grep -nE '^#+ What happened today'` before concluding it is absent. A `^## ` grep returns nothing there, and reading that as "this vault's template has no such section" silently skips the entry — the daily note then reports the session as unrecorded and `session-close` Phase 7 has to catch it. Observed 2026-08-20. Same defect class `session-close.md` § Phase 7 already fixed on its own side.

**Re-read the file immediately before writing — never write from a stale copy.** This command reads the daily note early (to locate the section) and may compose the entry over several turns. In that gap a sibling session can rewrite the same file, and writing over it from the earlier read silently destroys their entry — the clobbered text is lost for good, because obsidian-git autocommits whatever is on disk and the earlier write never enters history. Observed 2026-08-23 on the Personal vault: a `sync-progress` entry vanished this way, provable via `git log -S "<entry-text>"` showing a single commit (the later restore) instead of add+remove.

**Mandatory write protocol for the daily note:**
1. **Immediately before writing**, read the file again from disk (`Read` tool — not memory, not the earlier grep).
2. **Merge, never overwrite** — splice the new entry into the *current* content. If the file grew or changed since your earlier read (a sibling session's entry landed), keep their content and add yours alongside.
3. If the file is missing or the section vanished since your read, re-locate it and re-merge rather than recreating from scratch.

The same guard applies to any other shared vault file this command writes (task/goal pages, PR sections).

Entry shape:

```markdown
### [[{Task Name}]] — Done ✅

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
- **Wikilink the task/goal name in the heading.** `session-close` Phase 7 verifies this session's work is represented under "What happened today" by matching `[[wikilink]]` against the touched task/goal titles — a plain-text heading fails that check even though the entry is present, and the operator has to hand-patch it before close passes
- Quote exact numbers/versions/metrics from the conversation
- 2-3 sentence summary max; link to content pages for full context
- **Never wikilink a file from another vault.** `[[Name]]` resolves within the *current* vault only, so a cross-vault reference is at best dead and at worst silently wrong: a dated filename like `[[2026-09-18]]` resolves to THIS vault's own daily note rather than the file you meant, producing a self-link that looks correct in the rendered note and in the graph. Write cross-vault files as a plain path — `` `Trading/IBKR Scans/2026-09-18.md` `` — and reserve `[[wikilinks]]` for pages in the vault being written to. Observed 2026-09-18, on an entry generated from the `Files updated:` block above.

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
1. **Project allowlist gate — runs before any lookup.** The allowlist is `allowed = ["BRO"]`. Reduce the key to its project (the part before the `-`) and compare it against `allowed`. Only `BRO` keys proceed; a key whose project is not listed is **skipped silently** — no lookup, no comment, no transition, no error, no warning.

   This is a deliberately short allowlist, not a special case. `[A-Z]+-\d+` is a heuristic over free text, and an ID from another system can collide with a real Jira project key. Observed 2026-10-06: the decision-register ID `DEC-79` (from decisions.seibert.group) resolved against a real `DEC` project to an unrelated 2022 closed ticket, so the phase would have commented on a stranger's ticket. `IT-` — the IT / helpdesk service desk — is one instance of the same class: a progress comment landed on `IT-47383` on 2026-10-01 and the operator deleted it. Extend the allowlist by adding one key.

   Detection itself is unaffected — a skipped ticket may still belong in the daily note.
2. `mcp__atlassian__getJiraIssue(cloudId=JIRA_CLOUD_ID, issueIdOrKey=<key>)` → current status. If the ticket does not exist (404 / not accessible) → skip silently.
3. **Always** post a progress comment via `addCommentToJiraIssue(...)`. Same content as Phase 3.1's daily-note section (summary + key results + decisions + PR links), as Jira markdown. Deduplicate: if the last comment on the ticket already contains the same headline summary and a timestamp within the last hour, skip — avoids double-posting on re-runs of `/vault-cli:sync-progress`.
4. If conversation indicates completion AND ticket status != Done:
   - `getTransitionsForJiraIssue(...)` → find "Done" (case-insensitive)
   - `transitionJiraIssue(...)` → transition
   - The comment from step 3 stands as the completion record — no second comment needed.

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
- `section` — the section name where content landed (e.g. `What happened today`, `Pull Requests`, `Results`), at whatever heading level the vault uses; empty if the whole file is new

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

## Phase 6: Closer panel (task-complete only)

If a task was completed in Phase 4 (auto via 4a OR confirmed via 4b), append the state-closer panel below the Phase 5 report — exactly this shape, verbatim, no rewording:

```
⚪ DONE
👤 You: approve: /vault-cli:session-close
⏰ Next: your reply
```

Why this closer is the only correct one here:

- **One task per session.** Completing a task = THIS session is done. Queued items on today's daily note are NOT "queued in this session" — they are picked up by the orchestrator in fresh Claude sessions, never by appending more tasks to the current one.
- **Never recommend `/vault-cli:next-task` here.** That command exists for the orchestrator (or the user opening a new session); it is not a follow-up to `/vault-cli:sync-progress`.
- **Never recommend a specific next task by name.** Same reason — the next session's anchor selection belongs to the orchestrator, not to this command.
- **The "no end-of-day suggestions" global rule does NOT override this.** Session-close ≠ day-close. The rule forbids unsolicited *stop for the day* nudges; closing one task's session is the routine step between two task sessions, not a wind-down.

**Skip the closer entirely** when Phase 4 did NOT complete a task — i.e. PR-only sync, progress-only sync, or any path where the active task is still `in_progress`. In those cases the session continues on the same task; emitting `⚪ DONE` would be wrong.
