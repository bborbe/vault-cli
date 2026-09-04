---
description: Post-compaction verification — read the session checkpoint, verify carry-over items, re-arm watchers, surface next actions
allowed-tools:
  - Read
  - Write
  - Glob
  - Monitor
  - Bash(git status:*)
  - Bash(git log:*)
  - Bash(git rev-parse:*)
  - Bash(pgrep:*)
  - Bash(dark-factory status:*)
  - Bash(docker ps:*)
---

Post-`/compact` verification. Reads the checkpoint that `/vault-cli:prepare-compact` wrote before compaction, verifies each carry-over item against live state, re-arms watchers the fresh context lost track of, and surfaces the next actions. Run after `/compact` in the same session when prepare-compact returned "Compact-safe — N carry-over items". Also safe to run after a bare `/compact` with no checkpoint — it reports nothing pending.

This command **must stay inline** — it derives the session id from the parent conversation's own scratchpad path; a sub-agent cannot see the conversation. It is read-and-report-only: its only write is marking the checkpoint consumed. Never auto-commit, auto-push, kill a daemon.

## Find the session checkpoint

The checkpoint path template is `~/.claude/compact-checkpoints/<session-id>.md`.

- `<session-id>` is derived from the session's own scratchpad path — the same derivation `/vault-cli:prepare-compact` uses, so both commands resolve the same file. Never derive it from user input, conversation text, or file content (trust boundary).
- If the file does not exist, or already carries a `## Consumed` marker, print `Nothing pending — no active checkpoint.` and stop. This is the idempotent no-op: a second run after consumption never re-surfaces work.

## Verify carry-over items

Read the `## Carry-over items` section. For each item, re-run the live check prepare-compact recorded and confirm the state still matches:

- `uncommitted` / `un-pushed` → `git status --short` / `git log @{u}..` (upstream via `git rev-parse`)
- `background` → `dark-factory status`, `docker ps`, `pgrep -af 'dark-factory|docker'`
- `gate` → no live check; confirm the open question is still unanswered and re-surface it

Report each as `✅ verified` (state matches the checkpoint) or `⚠️ changed` (state differs — name the delta). A changed item is not an error; it means work progressed during compaction. Note it and move on.

```bash
# Git state — reports gracefully outside a git repo
git status --short || echo "not a git repository"

# Un-pushed commits against the upstream (computed via git rev-parse)
git rev-parse --abbrev-ref @{u} >/dev/null 2>&1 && git log --oneline @{u}.. || echo "no upstream / nothing un-pushed"

# Daemon and containers — each falls back rather than aborting
dark-factory status || echo "no daemon"
docker ps || echo "no containers"

# Live background state — background shells, sub-agents, watchers
pgrep -af 'dark-factory|docker' || echo "no matching background processes"
```

This check block mirrors `prepare-compact.md` § Compact-safety checks — keep the two in sync when one changes.

Each check falls back to its `|| echo "..."` text when the tool is absent on the operator's machine; the checklist CONTINUES and reports that tool as absent — it never aborts the command.

## Re-verify the anchor task and goal

The checkpoint's `State:` line is a snapshot from write time, not a live reading. Work continues after prepare-compact runs — including completion — so that line is stale by construction and must never be re-emitted as current.

Resolve the anchor task named in the checkpoint, plus its parent goal when it has one (a parent *theme* has no status to check), and re-read both from disk. This mirrors `session-close.md` § Phase 4.5, which already checks goals alongside tasks:

```bash
vault-cli task get "<anchor task>" status --output json
vault-cli goal get "<parent goal>" status --output json   # skip when the parent is a theme
```

Interpret each:

- `completed` / `aborted` → say so plainly, treat every criterion in the checkpoint's `State:` line as closed, and DROP any carry-over item that existed only to advance it (a scheduled soak check, a watcher, a queued verification).
- any other status → carry the `State:` line forward as written.
- lookup fails (non-zero exit, or JSON that does not parse) → report the anchor as unverified and name the exit code. Never silently fall back to the checkpoint's line: an unverified anchor and a confirmed-open one look identical downstream.

Report the delta whenever it differs from the checkpoint: `⚠️ anchor task completed since checkpoint`.

**This step runs BEFORE the re-arm below, deliberately.** Re-arming a watcher for finished work is one of the failures it prevents.

Observed 2026-09-04: a checkpoint written at 22:00Z recorded `SC 3/5 ... status: in_progress`; the task was completed at 01:26 local — 3.5h later, by the same session. post-compact verified all three carry-over items, then emitted `Next action: Wait for one-shot cron ... to run the SC 4 drop count` for criteria already closed and signed off. Every wrong conclusion that followed, including a git worktree opened for a fix that was never needed, descended from that one unvalidated line. The carry-over checks all passed — they simply do not cover the anchor.

## Re-arm watchers and monitors

The fresh post-compact context lost track of background watchers / monitors that were running before compaction. From the resume block's `Live background:` line and the carry-over `background` items, re-establish anything still alive — restart the Monitor / background watch / watcher so completion and failure signals reach this session again.

## Re-anchor on conventions (optional, one line)

Compaction restores *state* (carry-over items, watchers, next actions) but not *rule-awareness* — the summarized context is exactly the drift trigger `/recall` exists for. After the re-arm step, emit a single pointer line — never the full doc re-read (the harness already re-injects `CLAUDE.md` into context each turn, so a full recall mostly re-emphasizes rather than recovers):

```
📌 Conventions: run /recall if you feel drifted — re-reads global + project CLAUDE.md.
```

Skip this step entirely when the session's drift risk is low (nothing touched git, worktrees, or repo conventions during the resumed task).

## Surface next actions

Print the concrete next steps, sourced from the verified items:

- the resume block's `Next action:` — the task where prepare-compact paused
- `uncommitted` / `un-pushed` work → the commit / push to run
- each `gate` → the open decision, phrased so the operator can answer it

Then emit the 4-field resume block again (`Next action:` / `Live background:` / `Un-pushed / uncommitted:` / `Open decision:`) with the verified state, so the post-compact handoff is itself resumeable. The four labels are the frozen resume-block schema — do not rename, reword, add, or remove a field.

## Consume the checkpoint

Append a `## Consumed` marker with the date to the checkpoint file (the `Write` tool). This is the command's only write. The next run sees the marker and returns the idempotent no-op above. Do not delete the file and do not edit the resume block or carry-over items.

## No closer panel

Do NOT emit a session-close-style closer panel — no `⚪ DONE` block; this command reports and the session continues.
