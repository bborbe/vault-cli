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
  - Bash(python3:*)
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
- `background` → `dark-factory status`, `docker ps`, `pgrep -f 'dark-factory|docker'` (PIDs only — `-a`/`-l` would print command lines carrying MCP `Authorization` headers)
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
# PIDs only: `-a` / `-l` print full command lines, and MCP process args carry Authorization headers
pgrep -f 'dark-factory|docker' || echo "no matching background processes"
```

This check block mirrors `prepare-compact.md` § Compact-safety checks — keep the two in sync when one changes.

Each check falls back to its `|| echo "..."` text when the tool is absent on the operator's machine; the checklist CONTINUES and reports that tool as absent — it never aborts the command.

## Re-verify the anchor task and goal

The checkpoint's `State:` line is a snapshot from write time, not a live reading. Work continues after prepare-compact runs — including completion — so that line is stale by construction and must never be re-emitted as current.

Resolve the anchor task named in the checkpoint, then read its parent goal from **the task file's own `goals:` frontmatter** — never from the checkpoint's `Goal:` line. That line is prose written at checkpoint time and validated against nothing; the staleness warning above applies to it exactly as it applies to `State:`. Re-read both from disk. (A parent *theme* has no status to check.) This mirrors `session-close.md` § Phase 4.5, which already checks goals alongside tasks:

```bash
vault-cli task get "<anchor task>" status --output json
# parent goal read from the TASK FILE, not from the checkpoint's Goal: line
vault-cli task get "<anchor task>" goals --output json
vault-cli goal get "<parent goal>" status --output json   # skip when the parent is a theme
```

**A wrong goal name still returns a valid status, so this check passes while the anchor is misattributed.** Nothing downstream distinguishes "resolved the right goal" from "resolved a real goal that is not the parent" — both print a status and both look clean. Observed 2026-09-06: a checkpoint's `Goal:` line named a goal that genuinely tracked the anchor task as a blocker but was *not* its `goals:` parent. post-compact resolved that name, got `in_progress`, reported the anchor verified — and every closer panel for the rest of the session named the wrong goal, including its success-criteria counts. A sub-agent surfaced it hours later; the git history showed the `goals:` field had never changed.

Interpret each:

- `completed` / `aborted` → say so plainly, treat every criterion in the checkpoint's `State:` line as closed, and DROP any carry-over item that existed only to advance it (a scheduled soak check, a watcher, a queued verification).
- any other status → carry the `State:` line forward as written.
- lookup fails (non-zero exit, or JSON that does not parse) → report the anchor as unverified and name the exit code. Never silently fall back to the checkpoint's line: an unverified anchor and a confirmed-open one look identical downstream.

Report the delta whenever it differs from the checkpoint: `⚠️ anchor task completed since checkpoint`.

**This step runs BEFORE the re-arm below, deliberately.** Re-arming a watcher for finished work is one of the failures it prevents.

Observed 2026-09-04: a checkpoint written at 22:00Z recorded `SC 3/5 ... status: in_progress`; the task was completed at 01:26 local — 3.5h later, by the same session. post-compact verified all three carry-over items, then emitted `Next action: Wait for one-shot cron ... to run the SC 4 drop count` for criteria already closed and signed off. Every wrong conclusion that followed, including a git worktree opened for a fix that was never needed, descended from that one unvalidated line. The carry-over checks all passed — they simply do not cover the anchor.

## Re-surface the open-items ledger

Compaction is exactly when an operator's ask goes missing: an instruction given but not yet a task, or a question asked but not yet answered, lives only in conversation context until it becomes a task — and that context is what compaction wipes. A manager session keeps those in a durable ledger; this command is what brings them back across the boundary.

```bash
python3 ~/.claude/plugins/marketplaces/claude-supervisor/scripts/open-items.py --session <session-id> list
```

- The script ships in the **claude-supervisor** plugin, not this one, so the path is the marketplace clone — the one stable unversioned location (`make update` keeps it current). Never `${CLAUDE_PLUGIN_ROOT}` here: inside a vault-cli command that variable resolves to *vault-cli's* root, where the script does not exist. Do not substitute a `cache/claude-supervisor/supervisor/<version>/` path either — it is per-version and would pin a version this command does not own.
- `<session-id>` is the **same id already derived above** from the session's own scratchpad path — the ledger file is `~/.claude/state/open-items/<session-id>.json`, keyed identically to the checkpoint. Never derive it from user input or file content (same trust boundary), and never fall back to the newest file in that directory: a wrong id reads a different session's ledger and both halves look healthy.
- **Read the exit code before reading the output.** The two outcomes this step can produce are not the same thing, and their text does not distinguish them:

  | Exit | stdout | Meaning |
  |---|---|---|
  | **0** | `(none open)`, or the entries | The script ran. Absence of entries is the normal case. |
  | **non-zero** | *(empty)* | The script did not run — missing, not executable, or errored. |

- **Non-zero exit → report the defect, never a clean bill.** Print `⚠️ Ledger unreadable — open-items.py exited <code> from <path>. The open-items ledger was NOT checked; an ask may be outstanding.` and name the path and the exit code. A missing *script* is a defect in this command's own configuration; a missing *ledger* is the ordinary case. Rendering the first as the second is a silent false negative in the one command whose entire job is to stop asks going missing across a compaction.
- **Exit 0 with no entries → print nothing.** Absence is the normal case; only a manager session keeps one.
- With open entries, re-surface them under the heading `📋 Open with the operator`, rendered **exactly as `list` prints them** — the manager runbooks' § Open with the operator owns that frame and this command must not restate it. Do not re-word an entry: its `text` is the operator's own wording, and paraphrasing it across a compaction is how the ask drifts.
- **Report, never resolve.** This command does not `add`, `answer` or `close` — an entry closes on a task file reading `status: completed` or on the operator's explicit answer, neither of which a post-compaction verification can establish. Surface them and let the manager's next round act.
- This **supersedes nothing** in § Verify carry-over items: a checkpoint `gate` item is one open question captured at checkpoint time, while the ledger is the durable list that survives independently of whether prepare-compact ran. Where both name the same question, report it once.

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
- each **open ledger entry** → the ask, unchanged in the operator's own wording (`asked-of-you` entries are the ones still waiting on them)

Then emit the 4-field resume block again (`Next action:` / `Live background:` / `Un-pushed / uncommitted:` / `Open decision:`) with the verified state, so the post-compact handoff is itself resumeable. The four labels are the frozen resume-block schema — do not rename, reword, add, or remove a field.

## Consume the checkpoint

Append a `## Consumed` marker with the date to the checkpoint file (the `Write` tool). This is the command's only write. The next run sees the marker and returns the idempotent no-op above. Do not delete the file and do not edit the resume block or carry-over items.

## No closer panel

Do NOT emit a session-close-style closer panel — no `⚪ DONE` block; this command reports and the session continues.
