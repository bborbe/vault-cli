---
description: Pre-compaction checkpoint — sync vault progress, surface live background state, write a resume block
allowed-tools:
  - Skill
  - Read
  - Write
  - Glob
  - Bash(git status:*)
  - Bash(git log:*)
  - Bash(git rev-parse:*)
  - Bash(pgrep:*)
  - Bash(dark-factory status:*)
  - Bash(docker ps:*)
  - Bash(mkdir:*)
---

Pre-`/compact` checkpoint. Syncs vault progress, surfaces live background state, and writes a resume block so compaction never silently drops work. Run before `/compact` when you want to keep working in this session afterward.

This command **must stay inline** — it reads the parent conversation's state (touched goals and tasks, live background work, the session's own id); a sub-agent cannot see the conversation. It is read-and-report-only: its only writes are vault progress updates (steps 1–2) and the per-session checkpoint file (step 5). Never auto-commit, auto-push, kill a daemon.

## Sync vault progress

Invoke the sibling command exactly once:

```
Skill: vault-cli:sync-progress
```

This flushes the session's completed work into the daily note and task/goal pages before anything is compacted. No fallback or graceful-degradation logic needed: `vault-cli:sync-progress` ships in the same plugin bundle and is guaranteed present — this is an ordinary same-plugin sibling reference. If it reports no vault-tracked work, note that and continue.

## Goal and task sweep

Sweep every goal and task touched in this session. For each, confirm the page's Status Summary / Progress / Next line and its checkboxes reflect reality — tick what is actually done, note what is not. This is the second and last write step: progress updates only, via the step-1 sync-progress call and these page edits. No new scope, no new work.

## Compact-safety checks

Compaction silently drops whatever is not written down. Check what would be lost, in this order:

**Git state** — uncommitted work (`git status`) and un-pushed commits (`git log` against the upstream computed via `git rev-parse`), each guarded so a non-git directory reports gracefully instead of aborting.

**Live background state** — background shells / sub-agents / watchers via `pgrep`; the dark-factory daemon via `dark-factory status`; running containers via `docker ps`.

```bash
# Git state — reports gracefully outside a git repo
git status --short || echo "not a git repository"

# Un-pushed commits against the upstream (computed via git rev-parse)
git rev-parse --abbrev-ref @{u} >/dev/null 2>&1 \
  && git log --oneline @{u}.. || echo "no upstream / nothing un-pushed"

# Live background state — background shells, sub-agents, watchers
pgrep -af 'dark-factory|docker' || echo "no matching background processes"

# Daemon and containers — each falls back rather than aborting
dark-factory status || echo "no daemon"
docker ps || echo "no containers"
```

Each check falls back to its `|| echo "..."` text when the tool is absent on the operator's machine; the checklist CONTINUES and reports that tool as absent — it never aborts the command.

Interpretations:

- `git status --short` output → uncommitted work. `git log @{u}..` output → un-pushed commits; empty means caught up with the remote OR no upstream configured — both silent-OK states.
- `pgrep` hits → background shells / sub-agents / watchers still alive.
- A stopped daemon does not mean stopped work — the processes it spawned can still be running.
- A running dark-factory daemon is worth pausing for before compaction.
- `docker ps` hits → running containers that would outlive the compact.

**Unanswered gates.** Inventory the decisions this session raised but never answered — a deferred question, a skipped confirmation, a choice left open. Name each one so it survives the compact; an unanswered gate is open state.

## RESUME AFTER COMPACT

Emit the 4-field resume block below, populated from the findings of steps 2–3 — this is what the next session reads to pick up exactly where this one paused:

```
Next action:
Live background:
Un-pushed / uncommitted:
Open decision:
```

Fill the fields from the sweep and checks: the concrete next step on the touched goal or task; anything still running in the background; anything git reported; each unanswered gate. The four labels are the frozen resume-block schema — do not rename, reword, add, or remove a field.

## Per-session checkpoint file

Write the resume state to a checkpoint file whose path template is `~/.claude/compact-checkpoints/<session-id>.md`.

- `<session-id>` is derived from the session's own scratchpad path — never from user input, conversation text, or file content. This is the trust boundary: a user- or file-controlled session-id would allow writing to an arbitrary filename. If the checkpoint directory does not exist, create it with the granted mkdir capability.
- Per session, not a single fixed path — two concurrent sessions in the same project each write their own `<session-id>.md` and never clobber each other.
- The file lives under `~/.claude/` — outside every repo on purpose — so it survives compacts and is never committed to any worktree.
- Perform the write with the `Write` tool (the granted `Write` scope); this is one of only two write paths in the command, the other being the step 1–2 vault progress updates.

After writing, state the full path in your final message — a checkpoint that is written but never announced defeats its purpose.

## Verdict

- `✅ Ready to compact` — when all checks pass: nothing uncommitted or un-pushed, nothing live in the background, no unanswered gates, and the resume block + checkpoint file are in place.
- `⚠️ Not compact-safe yet` — when any open item exists (uncommitted work, live background process, unanswered gate). Name the open items so they are resolved or deliberately carried before compaction.

## No closer panel

Do NOT emit a session-close-style closer panel — no `⚪ DONE` block; this command reports and writes, and the session continues. Unlike `/vault-cli:session-close`, which ends the session, `/vault-cli:prepare-compact` only pauses it for compaction — the session continues afterward.
