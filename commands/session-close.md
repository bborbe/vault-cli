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
CLOSER_SWEEP     = a task-sweep skill/command is present in session (name ends in
                   `close-obsolete-tasks`); absent → Phase 4.6 names files only
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

Invoke the skill. Literally this call, not an inlined equivalent:

```
Skill: vault-cli:sync-progress
```

If the skill aborts ("No vault context detected", "No completion or PR detected"), report that briefly and continue — not all sessions produce vault-tracked work.

**Do NOT skip this step, and do NOT reimplement it inline.** "Run the sync-progress logic yourself" is not a valid reading — observed 2026-08-10, where the agent hand-checked the daily note instead of invoking the skill, skipped writing the session's entry, and then reported the session clean. Phase 7 is the backstop for exactly this, but it only catches the omission if it is written as specified below. Even sessions that "just talked" can include decisions worth recording.

**Exception — the record already exists.** If this session's work is ALREADY represented under "What happened today", do NOT invoke the skill: a second run appends duplicate entries. Two routes get you there, and both count:

1. **`/vault-cli:sync-progress` already ran in THIS conversation** and no vault-tracked work happened since. This ordering is normal, not exotic — the Integration section below lists `sync-progress` as the mid-session checkpoint, so an operator running it and then closing is the expected path. Observed 2026-08-16.
2. **The entry was written during task execution.** Recurring operational tasks routinely carry a `Document findings in today's daily note` subtask, so the record lands as a task deliverable, before close. Observed 2026-08-29 on a Prometheus alert-triage task: the entry was written during execution, this clause did not recognise that route, and the operator had to invoke `/vault-cli:sync-progress` by hand — which then wrote nothing, because the record was already there.

**Decide from the file, not from memory of what ran.** Run Phase 7's representation check (defined there — do not restate its matching rules here, they will drift) and treat a match as the exception firing. That test is route-agnostic: it answers "is the record there", which is the thing that actually matters, whereas "did the skill run" is a proxy that misses route 2.

Either way, state the skip explicitly in the Phase 9 output ("Phase 2 satisfied by the earlier sync-progress run" / "Phase 2 satisfied — entry written during task execution") rather than passing over it silently, so Phase 7's representation check still has something to verify against.

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

**A follow-up task this session CREATED is not an anchor — exclude it.** "Touched" is implemented as "edited a file under `tasks_dir`", and creating a file counts as editing it. But a task spun out mid-session is queued work for a *future* session, and the vault convention defaults new tasks to `in_progress` — so every session that files a follow-up trips this gate. That is a whole legitimate category, and the only lever available at close is a status flip, which silences the gate without changing anything real. `sync-progress.md` Phase 4a already draws this line: *"Follow-up items filed AS SEPARATE specs/tasks/ideas do NOT count as blockers — they explicitly off-scope themselves."*

A touched task is **excluded** from this gate when BOTH hold:

1. its file was created during this session (it did not exist when the session began), AND
2. its `claude_session_id` does not name this session — i.e. `work-on-task` never anchored on it.

Everything else touched and `in_progress` still hard-flags. Condition 2 is load-bearing: a session that creates a task *and then works it* has made it the anchor, and abandoning that is exactly what this phase exists to catch.

Observed 2026-09-02: a session completed its anchor, filed two follow-up tasks, and close flagged both — the operator reported it as near-daily (*"You do it every day nearly"*), and the proposed resolution was to flip both to `next` purely to clear the flag. The originating task [[Make Session-Close Refuse to Close While the Anchor Task Is In_progress]] weighed *mine vs sibling-session* and never considered *created-this-session*; this is that unconsidered third category, not a reversal of its Out of Scope.

For each touched task `T`, capture status AND error state:

```bash
STATUS_OUT="$(vault-cli task get "$T" status --output json 2>&1)"
STATUS_EXIT=$?
```

Interpret:

- `STATUS_EXIT == 0` and parsed `value` field:
  - `status: completed` → ✅ silent OK
  - `status: in_progress` → ⚠ **HARD flag** — the session anchored on this task but never completed it. This is a blocker on the clean verdict, not a soft warning: it forces Phase 9 into mode 3 (outstanding) and must be the item named in the closer's `approve:` line. There is no "no action needed" / "correct, standing trigger" / "deliberate" annotation that downgrades an `in_progress` anchor task to clean — a task that is designed never to complete still blocks the clean close until the operator explicitly resolves it (complete / defer / hold / abort). Observed 2026-08-30: a standing-trigger anchor task stayed `in_progress` by design, session-close flagged it as outstanding but named a worktree-cleanup item as the `approve:` item instead, and the session closed `⚪ DONE` with the anchor unfinished. The task is the gate; no other item stands in for it.
  - created this session AND not anchored (see the exclusion above) → ✅ silent OK (follow-up filed for a later session, never this session's anchor)
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

**Goals get the same check.** Phase 1 already detects touched goals, but until 2026-08-10 nothing verified their status — a goal left `in_progress` with every subtask done sailed through close unflagged, while the equivalent task was caught. Run the identical check against Phase 1's `Goals` list:

```bash
STATUS_OUT="$(vault-cli goal get "$G" status --output json 2>&1)"
STATUS_EXIT=$?
```

Interpretation is identical to the task branch above — `completed` / `hold` / `aborted` / `next` / `backlog` are silent OK, `in_progress` flags, a non-zero exit or parse failure surfaces as its own outstanding line rather than being skipped. Same scoping rule too: **touched goals only**. A goal this session never edited belongs to another session.

For each `in_progress` goal, surface in Phase 9 as outstanding:

```
N. Goal [[<title>]] still in_progress — `/vault-cli:complete-goal "<title>"` to close, `/vault-cli:defer-goal "<title>" <date>` to push out, or set status hold/aborted if abandoning
```

For each goal whose status lookup FAILED:

```
N. Goal check unverified for [[<title>]] — `vault-cli goal get` failed (exit <code>, stderr: <first-line>). Investigate before close.
```

A goal legitimately outliving the session is common — goals span 1–4 weeks, sessions do not. The flag is a prompt to confirm that's deliberate, not an assertion the goal should be closed.

### Phase 4.6: Check tasks this session CAUSED (not just touched)

Every phase above asks *"what did this session edit?"* — Phase 4.5 says so explicitly ("Scope this check to TOUCHED tasks only"), and that scoping is correct: it stops the command nagging about work belonging to sibling sessions. But it leaves a gap.

A session can also **cause** tasks it never edits. It fires a trigger, or merges a PR, and an external producer writes a task file into some vault. That task is neither touched-by-this-session nor another session's business — so nothing looks at it, and the session that set the work in motion reports clean while the work sits open.

Observed 2026-08-12: a session triggered a PR review, the producer wrote a review task, the PR then merged (making the review moot), and close reported `nothing queued`. The user had to ask "what about this task?" — it was still `in_progress`. Phase 4.5 could not have caught it: the session never opened that file.

**Skip silently** if Phase 1 recorded no repos AND no PRs — with no session artifacts to match against, there is nothing this phase can attribute.

For each vault in `VAULT_CONFIG`, scan `<vault.path>/<vault.tasks_dir>` for files meeting ALL of:

- non-terminal `status` (anything except `completed` / `aborted`), AND
- references a repo or PR URL from Phase 1's `Repos` / PR list, AND
- NOT touched this session (if touched, Phase 4.5 already owns it — never double-flag), AND
- NOT created by this session as a follow-up — Phase 4.5 excludes those deliberately, and they are not moot work an external producer left behind; they are queued work the operator just filed. Re-flagging them here would reinstate the daily false positive Phase 4.5's exclusion exists to remove.

Each match is work this session set in motion and walked away from. Surface in Phase 9 as outstanding, one line per task:

```
N. Task caused by this session still open: <title> (<vault>) — <why it may be moot>
```

If `CLOSER_SWEEP` is available, append its invocation as the suggested fix. If not, name the file path and let the operator decide.

**Never auto-close.** The producing pipeline may still legitimately own the task — a merged PR makes a *review* moot, but says nothing about, say, a follow-up task the same producer created. Flag and suggest; the close decision is the operator's.

**Stay producer-agnostic.** Do not hardcode task types, producer names, vault names, or sweep-script paths — this command ships to installs with one vault, no pipelines, and no sweep skill, where the phase must skip in silence rather than error.

### Phase 5: Check for orphaned background processes

**Scope: THIS session's processes only.** Build the candidate list from the conversation, not from the process table — a machine-wide `ps` scan cannot tell a sibling session's daemon from your own, and flagging someone else's is a false positive that makes the verdict untrustworthy.

Enumerate background work **this session started**:

- Bash calls made with `run_in_background: true` in this conversation
- `Monitor` watches armed in this conversation
- Long-running foreground commands this session backgrounded explicitly

If that list is empty → **skip this phase silently. Do NOT run `ps aux`.**

Only for PIDs already on that list, confirm liveness:

```bash
ps -p <pid> -o pid=,etime=,command= 2>/dev/null
```

Report only those that are still alive. Don't kill anything without confirmation.

**Never flag a process this session didn't spawn** — including `dark-factory daemon`, watchers, or dev servers belonging to sibling Claude sessions. Each session cleans up its own; another session's daemon is that session's business, and its `session-close` will handle it.

### Phase 6: Check for in-flight dark-factory work

If `DARK_FACTORY` is absent OR no project in scope has a `.dark-factory.lock` file → skip silently.

"In scope" means a repo **this session touched** (Phase 1's `Repos` list). A dark-factory project this session never edited belongs to another session — skip it, even if its daemon is running.

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

For each vault matching cwd (or the unambiguous vault if all session work was in one), read its `daily_dir` from `VAULT_CONFIG` and check that today's daily note exists **and that THIS session's work is represented in it**.

```bash
TODAY="$(date +%Y-%m-%d)"
DAILY_DIR="$(vault-cli config list --output json | jq -r --arg p "$(pwd)" '.[] | select($p | startswith(.path)) | .path + "/" + .daily_dir')"
DAILY="$DAILY_DIR/$TODAY.md"
ls "$DAILY"
```

**"Populated" is not the test — representation is.** Checking only that *some* `###` heading exists under "What happened today" passes trivially on any day earlier sessions already wrote entries, which is most days after the first. Observed failing 2026-08-10: the section held four entries from prior sessions, so the check went green while the current session's work was entirely absent, and the entry had to be written by hand afterwards.

Instead, take Phase 1's touched `Tasks` + `Goals` list and require that at least one `###` entry under the "What happened today" section references at least one of them by `[[wikilink]]`:

```bash
# For each touched task/goal title T, does any entry under the section link to it?
# T_RE = T with regex metacharacters escaped — see below, do not skip this.
T_RE=$(printf '%s' "$T" | sed 's/[][\.^$*+?(){}|\/]/\\&/g')
awk '/^#+ What happened today/,0' "$DAILY" \
  | grep -E "\[\[([^]|#]*/)?${T_RE}([|#][^]]*)?\]\]"
```

**Escaping `T` is mandatory, not a nicety.** Task titles routinely contain regex metacharacters — this vault alone has `Cleanup Email Inbox (Personal) - <date>`, `(Work)`, `(Recurrence)`. Unescaped, `(Personal)` is parsed as a capture group, so `grep -E` looks for the title *without* the literal parentheses and never matches. The entry is present, the check returns 0, and Phase 7 false-flags — the same always-flag failure class as the heading-level bug below, reached by a different route. Verified both ways: unescaped → 0, escaped → 1, same file and same entry.

**Do not hardcode the heading level.** Daily-note templates differ across vaults — the Personal vault uses `# What happened today` (h1), while this command's own prose and `sync-progress.md` both say `##`. An `awk` range anchored on `^## ` never matches there, so the range stays empty, the grep receives no input, and the check returns 0 **unconditionally** — it flags on every run regardless of content. The direction is fail-safe but the check is useless: crying wolf every time trains the reader to ignore it, which is precisely the failure Phase 7 exists to prevent. Caught by the v0.106.3 end-to-end run — which is the argument for exercising a check rather than reasoning about it.

**Match every wikilink form, not just the bare one.** A naive `grep -F "[[$T]]"` misses three variants this vault uses routinely, and each miss is a false flag:

| Form | Example |
|---|---|
| aliased | `[[Trading - IBKR Swing Trading Daily\|runbook]]` |
| heading | `[[Rebuild Dev and Prod#Fallback when sun is off]]` |
| path-prefixed | `[[22 Goals/90 Completed/Become profitable…\|Grid Trading Strategy Goal]]` |

The path-prefixed form is the nastiest: the link does not even *begin* with the title, so anchoring on `[[$T` fails too. The pattern above allows an optional folder path before the title and an optional `|alias` / `#heading` after it.

Erring toward over-matching is correct here. A false pass silently reinstates the original bug (close reports clean, record is missing); a false flag costs one glance.

- Match found → ✅ silent OK.
- No touched task/goal appears → ⚠ flag as outstanding.
- Phase 1 touched no tasks or goals at all (pure repo work, talk-only session) → fall back to the old weaker test (section exists and is non-empty), and do not flag on absence — there is nothing to match against.

**Report the symptom, not a guessed cause.** A missing entry has at least three causes that look identical from here: Phase 2 was skipped or inlined, the skill ran but aborted ("No vault context detected" / "No completion or PR detected"), or it ran and legitimately found nothing worth writing. Naming only the first sends the reader to debug the wrong thing.

Flag text for Phase 9:

```
N. Daily note has no entry for this session's work ([[<touched task/goal>]] not referenced) — Phase 2 was skipped, aborted, or found nothing to write. Check its output above, then run `/vault-cli:sync-progress` before closing
```

This is deliberately a flag, not an auto-fix: writing the entry is `sync-progress`'s job, and silently generating one here would hide that Phase 2 produced nothing.

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
# Search EVERY vault path in VAULT_CONFIG, not just the one owning the file.
# Read into an ARRAY — the shell here is often zsh, which does NOT word-split an
# unquoted "$paths" string. A bare `grep ... $ALL_VAULT_PATHS` passes all paths as
# ONE argument, grep fails, stderr is swallowed, and every link reads UNRESOLVED —
# a false flag indistinguishable from a real one. Verified 2026-08-16.
ALL_VAULT_PATHS=("${(@f)$(vault-cli config list --output json | jq -r '.[].path')}")  # zsh
# bash equivalent: mapfile -t ALL_VAULT_PATHS < <(vault-cli config list --output json | jq -r '.[].path')
# basename without .md, matched as a [[wikilink]] (with or without alias/heading)
grep -rlF "[[$BASENAME" "${ALL_VAULT_PATHS[@]}" --include='*.md' | grep -vF "$FILE" | head -1
```

**Sanity-check the search before trusting a negative.** `echo "${#ALL_VAULT_PATHS[@]}"` must be ≥1, and a known-good link must resolve. An empty result from a broken search looks exactly like a clean result from a working one — see [[Checks That Report False Green]].

Zero inbound links on a **newly created** page = orphan. Flag HIGH — it won't be found again.

**2. Broken outbound links (HIGH)** — extract `[[Target]]` targets from the page; verify each resolves to a file in **any** vault in `VAULT_CONFIG` (`find/glob` by basename). Unresolved target = broken link or typo. Flag with the target name.

**Tag-line wikilinks are not page links — exclude them from check #2.** Many vaults write the `Tags:` line as wikilinks (`Tags: [[Task]] [[Inbox]] [[OmniFocus]]`) for tags that intentionally have no page. Resolving those the same way as body links reports the vault's own tagging convention as broken. Observed 2026-08-20: `[[OmniFocus]]` was flagged on a generated recurring task, and it turned out to be one of **14** unresolved tag names across that vault's 74 generated tasks (`[[Backup]]` ×7, `[[Planning]]` ×5, `[[Review]]` ×4, `[[Finance]]` ×4, …) — not one broken link but the house style. Worse, the flag sent the operator to "fix" a source template, where the one-file change would have created drift against 73 siblings. When extracting targets, skip any wikilink on a line matching `^Tags:` — check body links only.

**Cross-vault links are normal — search all vaults for both checks.** Scoping resolution to the owning vault reports every legitimate cross-vault wikilink as broken. Observed 2026-08-16: a task in `Personal` linking `[[Boss Memory]]` was flagged unresolved because the check searched only `Personal` and `Trading`; the page lives in the `Boss` vault and is referenced by 20+ files across two others. A false "broken link" costs the operator a needless investigation and erodes trust in the whole verdict — search wide, and treat a hit in any vault as resolved.

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

**Mode gate (Phase 4.5 is a hard block, not a suggestion):** if any touched task is `in_progress`, modes 1 and 2 (the clean verdicts) are **forbidden** — the verdict MUST be mode 3 (outstanding), with that task as an outstanding item. An `in_progress` anchor task can never be annotated away ("no action needed", "standing trigger", "deliberate") into a clean verdict; the operator must resolve it (complete / defer / hold / abort) before the session can be suggested as closeable. Modes 1 and 2 are reachable only when every touched task is `completed`, `hold`, `aborted`, `next`, or `backlog`.

**1. Clean + no reflect signals** (all phases ✅ — including every touched task resolved, score < 3):

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
4. dark-factory daemon (pid X) — spawned by THIS session, still running in <project>
5. Link hygiene: [[<new page>]] orphaned — add backlink from [[<hub>]]
6. Consider /vault-cli:reflect — N knowledge file(s) created, org-level decisions captured
7. Consider /coding:self-improve — N friction signal(s): general correction, command misfire
```

Append the reflect and/or self-improve suggestions as the last numbered item(s) only if their signals fired. One line per item. No table, no tree, no asking. The user reads the list and decides what to do next.

Never auto-close, never auto-commit, never auto-kill, never auto-reflect, never auto-self-improve. The command's only job is the one-line verdict or the numbered open list.

### Closer panel — verbatim, no rewording

Append below the verdict. This command is terminal; without a fixed closer the trailing "name a concrete next action" convention gets improvised, and improvisation reliably produces a next-task recommendation — the exact thing the one-task-per-session contract forbids.

**Clean verdict (mode 1 or 2):**

```
⚪ DONE
👤 You: nothing — session closed
⏰ Next: you open a new session; the orchestrator picks the next anchor
```

**Outstanding items (mode 3):** `🔵 READY`, with `👤 You: approve:` naming exactly ONE item from the numbered list. **When an `in_progress` touched task is on the list, that task IS the item to name** — the `approve:` line must offer its resolution (`/vault-cli:complete-task "<title>"`, `/vault-cli:defer-task "<title>" <date>`, or set status hold/aborted), never a different item (a worktree cleanup, uncommitted files, a daemon). Naming any other item while the anchor task sits `in_progress` repeats the 2026-08-30 defect: the session closed `⚪ DONE` with the anchor unfinished. The task is the gate; no other item stands in for it.

**Never name a specific next task. Never recommend `/vault-cli:next-task`.** Next-session anchor selection belongs to the orchestrator (or to the user opening a fresh session), not to this command. Same rationale as `sync-progress.md` Phase 6 — and note that closing one task's session is the routine bookend between two task sessions, so the global "no end-of-day suggestions" rule does not apply here.

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
