---
description: Pre-close checklist — verify the session is safe to end (synced progress, committed work, no orphaned state)
allowed-tools:
  - Read
  - Glob
  - Grep
  - AskUserQuestion
  - Skill
  - Bash(vault-cli:*)
  - Bash(git status:*)
  - Bash(git log:*)
  - Bash(git rev-parse:*)
  - Bash(git worktree:*)
  - Bash(git ls-remote:*)
  - Bash(git branch:*)
  - Bash(jq:*)
  - Bash(ls:*)
  - Bash(jobs:*)
  - Bash(ps:*)
  - Bash(dark-factory status:*)
  - Bash(grep:*)
  - Bash(find:*)
  - Bash(lsof:*)
  - Bash(command -v:*)
---

End-of-session safety check. Verifies progress is documented, working trees are clean, and nothing important is still in flight. **Output is terse:** either `✅ Nothing to do — safe to close Claude :)` or a short numbered list of open items. Never prints a checklist table. Never asks yes/no. Never auto-closes anything.

This command **must stay inline** — it analyzes the parent conversation (touched files, completion signals, reflect + self-improve signals); a sub-agent cannot see the conversation.

Use before:

- Closing a long Claude Code session
- `/clear` or `/compact` when you want to keep history elsewhere
- Switching to a different project / context

## Runtime detection

```
VAULT_CONFIG     = `vault-cli config list --output json` (parsed once, cached for the session)
GH_AVAILABLE     = `command -v gh` exits 0
SEMSEARCH_MCP    = `mcp__semantic-search__search_related` present in session tools
DARK_FACTORY     = `command -v dark-factory` exits 0
TASK_LIST        = TaskList tool present in session
```

If any integration is absent, skip its phase silently — never error. Folder paths come from `VAULT_CONFIG` per vault; never hardcode `24 Tasks/`, `23 Goals/`, etc.

## Workflow

Run each phase in order **silently** — collect findings in memory, do NOT report inline. Only the Phase 9 final output is shown to the user. If a phase passes, no output. If a phase finds something open, capture it for the final list.

### Phase 1: Detect session scope (silent)

Collect — do NOT print yet. The summary goes in Phase 9 output.

Find what the session actually touched:

- **Goals**: edits this session under any vault's `<vault.goals_dir>` (from `VAULT_CONFIG`). Resolve each to its title + obsidian:// link.
- **Tasks**: edits this session under any vault's `<vault.tasks_dir>` (from `VAULT_CONFIG`). Resolve each to its title + obsidian:// link.
- **Repos**: walk every file path edited this session, find the nearest ancestor with a `.git` dir, dedupe. Express as `~/...` relative path.
- **Background tasks**: any still-running shell tasks from this session.

If multiple vaults touched, group goals/tasks per vault. Cap each list at 5 (show "+N more" if longer).

### Phase 2: Sync progress to vault (delegate to skill)

Invoke `/vault-cli:sync-progress` to flush conversation-tracked work into the daily note + task pages. If the skill aborts ("No vault context detected", "No completion or PR detected"), report that briefly and continue — not all sessions produce vault-tracked work.

Do NOT skip this step. Even sessions that "just talked" can include decisions worth recording.

### Phase 3: Check git state for each touched repo

For every repo the session touched (cwd + any other repos with edits in this conversation), run:

```bash
cd <repo> && git status --short
# Check upstream first — without this, `git log @{u}..` errors AND a fallback echo
# would be miscounted as an unpushed commit by any line-counting consumer.
cd <repo> && git rev-parse --abbrev-ref @{u} >/dev/null 2>&1 && git log --oneline @{u}.. || true
```

Interpret per repo:

- `git status --short` output → uncommitted changes (count + first few file paths)
- `git log @{u}..` output → unpushed commits; **empty output means either caught-up-with-remote OR no upstream configured** — both are silent-OK states
- Untracked files matching sensitive patterns (`.env`, `*.key`, `credentials.json`) → from `git status --short` (the `??` lines)

If any repo has uncommitted/unpushed work, ASK whether to commit/push before closing. Do NOT auto-commit — surface the choice.

**Exception: vaults with obsidian-git autocommit.** If the repo is a vault path (matches any `VAULT_CONFIG[].path`), pending edits are the steady state — obsidian-git handles them. Don't flag.

### Phase 3.5: Detect uncleaned feature worktrees

For each touched repo (from Phase 1), list worktrees:

```bash
cd <repo> && git worktree list
```

Filter to **non-durable worktrees** — paths whose final component does NOT match `master`, `main`, `dev`, `prod` (these are the deployment-locked worktrees and stay forever).

For each non-durable worktree, check if its branch still exists on the remote:

```bash
cd <worktree> && git ls-remote --exit-code --heads origin "$(git branch --show-current)" >/dev/null 2>&1
```

- Exit-code **non-zero** → remote branch deleted (typical after `gh pr merge --delete-branch`). The worktree is **orphaned** — kept work is committed and merged; the worktree itself is now garbage.
- Exit-code **zero** → branch still on remote → work in flight, **leave alone** (could be a parked session).

Cross-check against other still-active Claude sessions: a worktree from a sibling session (different cwd, different conversation) may be active — don't flag those. Use a conservative test: if any process under the worktree path is running (`lsof +D <worktree>` shows hits, or any `cwd` in `/proc` or via `ps -o pid,cwd` matches), assume it's actively used.

Surface in Phase 9 as outstanding:

```
N. Orphan worktree: ~/Documents/workspaces/<name> (branch <feat>, deleted from remote) — `cd <parent> && git worktree remove ../<name>`
```

Don't auto-remove — the user may have local changes they want to inspect first.

### Phase 4: Check active task list (TaskCreate)

If `TASK_LIST` is available, list tasks with status `in_progress`. Each one represents work the session started but didn't finish.

Report: "X tasks still in_progress: …". Ask whether to mark complete, defer, or leave for next session.

### Phase 4.5: Check session anchor task is complete

The one-task-per-session contract: each Claude session anchors on a single vault task; closing the session is the routine bookend between two task sessions. If that anchor task is still `in_progress`, closing now means abandoning it mid-flight — exactly the failure mode `/vault-cli:complete-task` and `/vault-cli:sync-progress` closer panels are designed to prevent.

**Scope this check to TOUCHED tasks only** (Phase 1's `Tasks` list). Vault tasks not touched in this session belong to OTHER sessions (running in sibling tabs OR queued for the orchestrator to pick up next) — they are NOT this session's responsibility and MUST NOT be flagged here.

For each touched task `T`, capture status AND error state:

```bash
STATUS_OUT="$(vault-cli task get "$T" status --output json 2>&1)"
STATUS_EXIT=$?
```

Interpret:

- `STATUS_EXIT == 0` and parsed `value` field:
  - `status: completed` → ✅ silent OK
  - `status: in_progress` → ⚠ flag — the session anchored on this task but never completed it
  - `status: hold` / `status: aborted` → ✅ silent OK (deliberate non-completion, owner already decided)
  - `status: next` / `status: backlog` → ✅ silent OK (touched as a side-reference, not as an active anchor)
- `STATUS_EXIT != 0` OR JSON parse failure → ⚠ surface as outstanding (do NOT silently skip — a failed check means the anchor-task gate is unverified, which is exactly the failure mode this phase guards against)

For each `in_progress` task, surface in Phase 9 as outstanding:

```
N. Task [[<title>]] still in_progress — `/vault-cli:complete-task "<title>"` to finish, `/vault-cli:defer-task "<title>" <date>` to push out, or set status hold/aborted if abandoning
```

For each task whose status lookup FAILED, surface in Phase 9 as outstanding:

```
N. Anchor-task check unverified for [[<title>]] — `vault-cli task get` failed (exit <code>, stderr: <first-line>). Investigate before close: `vault-cli task get "<title>" status` or open the file directly. Closing now skips the in_progress safety gate for this task.
```

Do not collapse failures into a generic warning — each unverified task is its own outstanding line so the user sees scope explicitly.

**Do NOT** check `[/]` items on the daily note here. Those represent the day's overall queue; items not touched by this session belong to other sessions and the orchestrator. Flagging them would force the user to clear unrelated work before closing — exactly the rule one-task-per-session is meant to avoid.

**MIT exception:** if today's daily-note "Most important task" checkbox `- [ ] [[Task]]` references a task that IS in this session's touched list AND that task is still `in_progress`, the warning above already covers it — no extra rule needed. If the MIT was not touched, it's a separate session's concern.

### Phase 5: Check for orphaned background processes

```bash
jobs -l 2>/dev/null
ps aux | grep -E '(dark-factory|daemon|watch)' | grep -v grep | head
```

If anything is still running that the user spawned this session, call it out. Don't kill anything without confirmation.

### Phase 6: Check for in-flight dark-factory work

If `DARK_FACTORY` is absent OR no project in scope has a `.dark-factory.lock` file → skip silently.

Otherwise, check daemon status:

```bash
cd <project> && dark-factory status 2>&1 | grep -E 'Current:|Queue:|verifying'
```

Report:

- Active container (still executing): name + duration
- Queued prompts: count
- Specs in `verifying`: count + names (these need `dark-factory spec complete` after AC verification)

If a spec is in `verifying`, ASK whether to verify+complete it now or leave for later.

### Phase 7: Check daily note

For each vault matching cwd (or the unambiguous vault if all session work was in one), read its `daily_dir` from `VAULT_CONFIG` and check that today's daily note exists with a populated "What happened today" section.

```bash
TODAY="$(date +%Y-%m-%d)"
DAILY_DIR="$(vault-cli config list --output json | jq -r --arg p "$(pwd)" '.[] | select($p | startswith(.path)) | .path + "/" + .daily_dir')"
ls "$DAILY_DIR/$TODAY.md"
```

If the file exists but has no `###` heading under `## What happened today`, flag and continue — likely `/vault-cli:sync-progress` was skipped.

### Phase 8: Detect reflect-worthy signals

Decide whether the session produced enough durable learning to warrant `/vault-cli:reflect`. Reflect is expensive (extracts → writes KB entries) and noisy if run on trivial sessions; auto-invoking it always erodes KB quality. Instead, **detect signals** and surface a suggestion only when they fire.

Resolve the knowledge dir(s) and runbooks dir(s) from `VAULT_CONFIG`:

- `KNOWLEDGE_DIRS` = list of `<vault.path>/<vault.knowledge_dir>` for each vault in scope
- `RUNBOOK_DIRS` = vault subdirs whose basename matches the regex `^[0-9]+ [Rr]unbooks$` (auto-discover; no config field today). Common cases: `65 Runbooks`, `70 Runbooks`.

Score the session silently:

| Signal | Detection | Weight |
|---|---|---|
| New or major edit to a knowledge/runbook file | Files under `KNOWLEDGE_DIRS` or `RUNBOOK_DIRS` created or with > 30 lines changed this session | +2 each (cap +4) |
| Org-/infra-level config decision | Conversation mentions `gh api`, secrets/variables, rulesets, branch protection, GitHub App, IAM, org policy | +1 |
| Tradeoff discussion captured | ≥ 3 `AskUserQuestion` calls this session, OR explicit "option A vs B" framing in assistant output | +1 |
| New reusable artifact created | Workflow templates, scripts, runbook procedures added to a vault or repo | +1 |
| Substantive session | > 50 tool calls total (rough proxy) | +1 |

If **total score ≥ 3** → flag as reflect candidate. Otherwise skip silently.

Do NOT run `/vault-cli:reflect` from here. Only surface the suggestion in Phase 9 output. The user decides whether to invoke it.

### Phase 8.5: Detect runbook improvements

If a runbook was executed this session, surface gaps so the runbook can be updated. **Detect, rate, suggest — never auto-edit.**

**Detect runbook usage:** scan conversation for `Read` of files under any vault's `RUNBOOK_DIRS` (see Phase 8). If none, skip silently.

**Extract gaps for each runbook used:** compare what the session actually did vs what the runbook documents:

- Procedures executed that runbook doesn't mention (e.g. discovered endpoint via source grep)
- Troubleshooting scenarios encountered but not covered (e.g. multi-day stale vs only "yesterday")
- Tools/commands the session needed but runbook omits
- Outdated paths/commands corrected mid-run

**Significance filter** (mirrors `/vault-cli:reflect` — all three must be YES):

| Question | Must be YES |
|---|---|
| Would future runs benefit? | Yes |
| Non-obvious from current runbook text? | Yes |
| Not documented elsewhere in the vault? | Yes |

**Rate each gap:**

- **HIGH** — procedure missing; caused investigation time (grep'd source, asked user, trial-and-error)
- **MEDIUM** — scenario/variant not covered; would speed diagnosis next time
- **LOW** — cosmetic, nice-to-have, minor wording

Cap at 3 gaps per runbook. Skip if none pass the filter.

**Surface in Phase 9 output** as a numbered "outstanding" item (never auto-edit the runbook):

```
N. Runbook improvements: 1 HIGH, 1 MEDIUM in [[<runbook>]] — review + edit
```

### Phase 8.6: Link hygiene for session-touched vault pages

Vault pages created/edited this session can end up orphaned or one-way-linked — discoverable only if something links *to* them. **Detect, surface, never auto-link.** Scope strictly to `.md` files the session touched under any `VAULT_CONFIG[].path` (skip repos / non-vault dirs). Skip silently if none.

For each touched vault page (cap 5):

**1. Orphan check (HIGH)** — does any *other* vault page link to it?

```bash
# basename without .md, matched as a [[wikilink]] (with or without alias/heading)
grep -rlF "[[$BASENAME" "$VAULT_PATH" --include='*.md' | grep -vF "$FILE" | head -1
```

Zero inbound links on a **newly created** page = orphan. Flag HIGH — it won't be found again.

**2. Broken outbound links (HIGH)** — extract `[[Target]]` targets from the page; verify each resolves to a file somewhere in the vault (`find/glob` by basename). Unresolved target = broken link or typo. Flag with the target name.

**3. One-way link to a hub/canonical page (LOW)** — if the page links to a hub/index/concept page (`page_type: hub`, or a `*Hub*`/`*Concept*`/`*Pipeline*` page) that does **not** link back, the new page is invisible from the hub. Suggest a reciprocal backlink. Soft signal — suggest, don't insist.

**Significance filter** — only flag #3 when the target is a genuine hub/canonical page a reader would navigate *from*. Don't flag reciprocity for every incidental mention.

**Surface in Phase 9 output** (never auto-edit):

```
N. Link hygiene: [[<page>]] orphaned (no inbound links) — add a backlink from [[<likely hub>]]
N. Link hygiene: [[<page>]] → [[<target>]] unresolved (broken wikilink / typo)
N. Link hygiene: [[<hub>]] doesn't link back to [[<new page>]] — one-way (consider reciprocal link)
```

### Phase 8.7: Detect self-improve-worthy signals

Decide whether the session revealed enough **tooling friction** to warrant `/coding:self-improve` (reviews the session, proposes ≤2 durable improvements to commands / agents / rules). Like reflect, this is **suggest-only** — never auto-run; auto-invoking always erodes signal. Reflect captures durable *knowledge*; self-improve captures *friction in the tooling* — a different signal set, so score it separately.

Score the session silently:

| Signal | Detection | Weight |
|---|---|---|
| General correction to assistant behavior | User corrected a non-one-off behavior that generalizes beyond this task ("did u read…", "always X", "don't Y") | +2 |
| Repeated instruction | Same instruction given 2+ times this session | +2 |
| Command / agent / skill misfired | A slash command or agent gave wrong output, needed a retry, or was abandoned mid-run | +1 |
| Documented rule violated | Assistant broke a rule in a `CLAUDE.md` it should have followed | +1 |
| Manual multi-step workflow with no command | ≥3-step procedure reinvented by hand that no existing command/skill covers | +1 |

If **total score ≥ 3** → flag as self-improve candidate. Otherwise skip silently.

Do NOT run `/coding:self-improve` from here. Only surface the suggestion in Phase 9. The user decides whether to invoke it.

### Phase 9: Final status line — one of three modes

**Do NOT print a checklist table. Do NOT ask a yes/no question.** The point of the command is a one-glance answer.

Always start with a **Session summary** block (from Phase 1 scope), then the verdict.

**Summary block** (always shown, even when nothing else is):

```
Session worked on:
- Goals: [Goal Title](obsidian://open?vault=V&file=PATH), [Other Goal](...)
- Tasks: [Task Title](obsidian://open?vault=V&file=PATH)
- Repos: ~/Documents/workspaces/sm-octopus, ~/Documents/workspaces/run
```

Omit any line with zero entries. If nothing was touched (e.g. talk-only session), omit the block entirely.

**Verdict — three modes:**

**1. Clean + no reflect signals** (all phases ✅, score < 3):

```
<summary block, if any>

✅ Nothing to do — safe to close Claude :)
```

**2. Clean + reflect and/or self-improve signals fired** (all phases ✅, reflect score ≥ 3 and/or self-improve score ≥ 3):

```
<summary block>

✅ Nothing outstanding — but the session has follow-up-worthy moments. Append whichever fired:
- reflect (new knowledge files, decisions captured) → Consider `/vault-cli:reflect` before closing.
- self-improve (tooling friction: corrections, retries, missing command) → Consider `/coding:self-improve` before closing.
```

**3. Outstanding items** (any phase ⚠):

```
<summary block>

⚠ Outstanding before close:

1. <repo>: N uncommitted file(s) — <first path>
2. ~/.claude: untracked <file>
3. Orphan worktree: ~/Documents/workspaces/<name> (branch <feat>, deleted from remote) — `git worktree remove ../<name>`
4. dark-factory daemon (pid X) running in <project>
5. Link hygiene: [[<new page>]] orphaned — add backlink from [[<hub>]]
6. Consider /vault-cli:reflect — N knowledge file(s) created, org-level decisions captured
7. Consider /coding:self-improve — N friction signal(s): general correction, command misfire
```

Append the reflect and/or self-improve suggestions as the last numbered item(s) only if their signals fired. One line per item. No table, no tree, no asking. The user reads the list and decides what to do next.

Never auto-close, never auto-commit, never auto-kill, never auto-reflect, never auto-self-improve. The command's only job is the one-line verdict or the numbered open list.

## Integration

End-of-session bookend of the per-session lifecycle:

```
session start → (work, tracked via per-task + per-day lifecycles) → /vault-cli:session-close
```

Pairs with:

- `/vault-cli:work-on-task` — anchor each session on a task
- `/vault-cli:sync-progress` — mid-session checkpoint
- `/vault-cli:complete-day` — per-day end bookend (day-level analog)

## Notes

- This command is **read + report + ask**, not write+act. Only the embedded `/vault-cli:sync-progress` skill writes files; everything else is observation + questions.
- If the user has multiple Claude Code sessions running concurrently, this command only sees state of the current session's cwd and conversation — it cannot inspect sibling sessions.
- Respect global preferences: terse output, numbered options not either/or, no Claude attribution.
- Works in any vault registered with `vault-cli config`; gracefully skips integrations (dark-factory, gh, semantic-search, TaskList) not present in the session.
