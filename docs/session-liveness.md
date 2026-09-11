# Session liveness

The single definition of whether a task's `claude_session_id` names a session that is **running right now**. Every command, agent, and doc that needs this answer reads it here — do not restate the rule inline, and do not invent a second definition.

## The four states

| State | Meaning |
|---|---|
| `live` | A session is running now. Do not treat the task as free. |
| `quiet` | A transcript exists and the session has ended. Take-over is safe. |
| `indeterminate` | Cannot be proven dead. Surface it; do not assume either way. |
| `none` | No `claude_session_id`. A human task — nothing to classify. |

## The rule

Keyed on the task's `claude_session_id` frontmatter. Two signals, both required.

1. **Transcript recency.** The transcript at `~/.claude/projects/<encoded-project-dir>/<uuid>.jsonl` (use `session_project_dir` from vault-cli config when set, else the vault path).
   - Written within `LIVE_WINDOW` (5 minutes — matched to vault-cli's per-session flock) → **fresh**.
   - Exists and older → **stale**.
   - Not found → **absent**.
2. **Process cross-check.** A live `claude --resume <uuid>` or `claude --session-id <uuid>` process on this host. Closes the open-but-idle gap: a resumed session's transcript goes quiet while its process lives.

| Transcript | Process | State |
|---|---|---|
| fresh | alive | `live` |
| stale | alive | `live` — open but idle |
| stale | none | `quiet` — ended |
| **fresh** | **none** | **`indeterminate`** |
| absent | any | `indeterminate` — cannot be proven dead |
| no `claude_session_id` | — | `none` |

## Why fresh-transcript-with-no-process is `indeterminate`, not `live`

A detached one-turn spawn writes its transcript and exits. For five minutes afterwards the transcript is fresh while no process exists. Reading that as `live` **hard-locks the task against the session that is actually working it** — the spawn's own caller.

Observed 2026-09-11: `vault-cli task work-on` spawned a one-turn session onto a task the calling session was already working; the spawn exited, and its fresh transcript made the task read `live` for five minutes against its real owner.

`indeterminate` is the honest answer — the signal genuinely conflicts — and it routes to the operator instead of a false block.

This is also why the caller must not *create* the ambiguity: a command that spawns a session onto a task another session is working is the defect, not the classifier.

## Never use these

- **Task-file mtime.** It moves when a human edits the file and says nothing about sessions. A file touched by a sibling is evidence of *collision*, not of ownership.
- **Session name** (`ListAgents`, `customTitle`). The name→task join is fuzzy and goes stale on rename. `claude_session_id` is the exact key; the name is a display convenience.
- **`status`.** `in_progress` is a queue state. Measured 2026-09-06 in the Personal vault: of 171 `in_progress` tasks, **13** had a live session, 73 a quiet one, and 85 no session at all. Reading `in_progress` as "someone is on it" is wrong for 158 of 171.

## Consumers

Each of these reads the rule from here rather than restating it:

- `agents/work-on-task-assistant.md` — Phase 5.5 classification, and the session-connect guard
- `commands/plan-task.md` — the ownership gate at step 2 and the pre-`Edit` re-check at step 6
- `commands/task-status.md` — the `⚠️ also claimed by peer` anchor-pair suffix
- `vault-ui/src/vault_ui/activity.py:classify_session_state` — the display-side implementation

## Reference implementation

`vault-ui/src/vault_ui/activity.py:classify_session_state()` is the executable form of the table above. Keep it in step with this file: a divergence between the two is the bug this document exists to prevent.
