#!/usr/bin/env python3
"""Close a finished worker's own pane, so its spawn-cap slot is released.

`session-close` verifies a session is safe to end and then stops, leaving the
process alive until a human types `/exit`. A reaped worker that follows the
message exactly therefore keeps its registry entry and its slot. This script is
the step that actually ends the process — and it refuses unless every condition
below holds, because a wrong exit kills work in flight.

The decision, in order (first failure wins, and every failure prints a named
line so the refusal is diagnosable):

  1. A session id is known (`$CLAUDE_CODE_SESSION_ID`, or `--session`).
  2. The session owns a WezTerm pane. Proven by a session-registry entry
     `$SESSIONS_DIR/<pid>.json` whose `sessionId` matches. A headless worker has
     no entry — it is an in-process SDK query with no pid — and it INHERITS its
     spawner's `$WEZTERM_PANE`, so killing that pane would close the manager's
     tab. This is the guard that makes the exit safe.
  3. The session's anchor task exists — resolved by `claude_session_id`, or, when
     that id has moved to another session, by the session appearing in the task's
     `metrics_sessions` (a recorded handover).
  4. The task is terminal on disk: `status: aborted`, or `status: completed`
     with `phase: done` and zero `[ ]` / `[/]` boxes. `hold` does not qualify —
     it is parked, not finished. A handed-off session skips this guard: its own
     work is on the record, so the successor's progress is not its proof.
  5. The attention store holds no open card for this session. An unreachable
     store refuses: a failed read is not evidence that no gate is open.

Only then does it run `wezterm cli kill-pane --pane-id <own pane>`, which ends
this process. `--dry-run` prints the same decision without killing.

    exit 0   exit taken (or, under --dry-run, would be taken)
    exit 10  refused — the named reason is on stdout
    exit 2   usage / unreadable input

⚠️ Never recommend or take a change to `spawn.maxConcurrent` here: this frees a
slot nobody is using, it does not open the limit.

Testability: every input is an env var (`SESSION_ID`, `SESSIONS_DIR`,
`TASKS_ROOTS`, `WEZTERM_PANE`, `ATTENTION_STORE_URL`) so `scripts/session-self-exit-test.sh`
can drive each branch against fixtures without a live fleet.
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request

EXIT_TAKEN = 0
EXIT_REFUSED = 10

STORE_TIMEOUT = float(os.environ.get("ATTENTION_STORE_TIMEOUT", "5"))

# Any `- [ ]` or `- [/]` checkbox, at any indent. Same rule the vault's own
# session-close gate uses; `- [x]` and `- [-]` are not open.
OPEN_BOX = re.compile(r"^[ \t]*-[ \t]*\[[ /]\]", re.MULTILINE)


def refuse(reason: str) -> int:
    print(f"SELF-EXIT: stay — {reason}")
    return EXIT_REFUSED


def session_id(arg: str | None) -> str:
    return (arg or os.environ.get("SESSION_ID") or os.environ.get("CLAUDE_CODE_SESSION_ID") or "").strip()


def sessions_dir() -> str:
    return os.environ.get("SESSIONS_DIR") or os.path.expanduser("~/.claude/sessions")


def owns_pane(sid: str) -> tuple[bool, str]:
    """True when a registry entry names this session, i.e. it is a tab session."""
    for path in glob.glob(os.path.join(sessions_dir(), "*.json")):
        try:
            with open(path, encoding="utf-8") as fh:
                record = json.load(fh)
        except (OSError, ValueError):
            continue
        if record.get("sessionId") == sid:
            return True, ""
    return False, (
        "no session-registry entry for this session — a headless worker has no pane of its own "
        "and inherits its spawner's $WEZTERM_PANE; killing it would close the manager's tab"
    )


def tasks_roots() -> list[str]:
    override = os.environ.get("TASKS_ROOTS", "").strip()
    if override:
        return [p for p in override.split(os.pathsep) if p]
    try:
        out = subprocess.run(
            ["vault-cli", "config", "list", "--output", "json"],
            capture_output=True, text=True, timeout=20, check=True,
        ).stdout
        configs = json.loads(out)
    except (OSError, subprocess.SubprocessError, ValueError):
        return []
    roots = []
    for entry in configs:
        path, tasks_dir = entry.get("path"), entry.get("tasks_dir")
        if path and tasks_dir:
            roots.append(os.path.join(path, tasks_dir))
    return roots


def read_frontmatter(path: str) -> tuple[dict[str, str], str, str]:
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    if not text.startswith("---\n"):
        return {}, text, ""
    end = text.find("\n---\n", 4)
    if end == -1:
        return {}, text, ""
    block = text[4:end]
    meta: dict[str, str] = {}
    for line in block.splitlines():
        key, sep, value = line.partition(":")
        if sep:
            meta[key.strip()] = value.strip().strip("'\"")
    return meta, text[end + 5:], block


def metrics_session_ids(block: str) -> list[str]:
    """Session ids under `metrics_sessions` — every session that ever worked the task.

    The block is a list of maps (`session_id` + `started_at`), which the flat
    frontmatter reader above cannot see: it keeps the empty `metrics_sessions:`
    line and reads each nested key as a top-level one. The ids are read here.
    """
    ids: list[str] = []
    in_block = False
    for line in block.splitlines():
        if not line.strip():
            continue
        if not line[0].isspace():
            in_block = line.partition(":")[0].strip() == "metrics_sessions"
            continue
        if in_block:
            key, sep, value = line.strip().lstrip("- ").partition(":")
            if sep and key.strip() == "session_id":
                ids.append(value.strip().strip("'\""))
    return ids


def anchor_task(sid: str) -> tuple[str, dict[str, str], str, bool] | None:
    """Resolve the task this session may exit against.

    A session's own anchor (`claude_session_id`) wins. Failing that, a task the
    session was handed off from still counts: the id appears in its
    `metrics_sessions` while `claude_session_id` names the successor. The fourth
    element says which branch matched, because a handed-off session has already
    put its work on the record and does not wait for the task to go terminal.
    """
    handed_off: tuple[str, dict[str, str], str, bool] | None = None
    for root in tasks_roots():
        for path in glob.glob(os.path.join(root, "*.md")):
            try:
                meta, body, block = read_frontmatter(path)
            except OSError:
                continue
            if meta.get("claude_session_id") == sid:
                return path, meta, body, False
            if handed_off is None and sid in metrics_session_ids(block):
                handed_off = (path, meta, body, True)
    return handed_off


def terminal_verdict(meta: dict[str, str], body: str) -> tuple[bool, str]:
    status = meta.get("status", "")
    if status == "aborted":
        return True, "aborted"
    if status != "completed":
        return False, f"task status is {status or '(none)'}, not completed or aborted"
    if meta.get("phase", "") != "done":
        return False, f"task is completed but phase is {meta.get('phase') or '(none)'}, not done"
    open_boxes = len(OPEN_BOX.findall(body))
    if open_boxes:
        return False, f"task is completed but carries {open_boxes} open box(es)"
    return True, "completed"


def store_url() -> str:
    return (os.environ.get("ATTENTION_STORE_URL") or "http://localhost:18080").rstrip("/")


def gate_state(sid: str) -> tuple[bool, str]:
    """True when the gate is provably clear. An unreadable store is NOT clear."""
    url = f"{store_url()}/api/1.0/attention"
    try:
        with urllib.request.urlopen(url, timeout=STORE_TIMEOUT) as resp:
            items = json.load(resp)
    except (urllib.error.URLError, OSError, ValueError) as exc:
        return False, f"attention store unreadable ({exc}) — a failed read is not a clear gate"
    if not isinstance(items, list):
        return False, "attention store returned an unexpected shape — not evidence of a clear gate"
    for item in items:
        if item.get("producer_id") == sid and item.get("state") == "open":
            return False, f"open attention card {item.get('item_id')}"
    return True, ""


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--session", help="session id (default $CLAUDE_CODE_SESSION_ID)")
    parser.add_argument("--dry-run", action="store_true", help="print the decision without killing the pane")
    args = parser.parse_args()

    sid = session_id(args.session)
    if not sid:
        return refuse("no session id — pass --session or set $CLAUDE_CODE_SESSION_ID")

    owned, why = owns_pane(sid)
    if not owned:
        return refuse(why)

    pane = (os.environ.get("WEZTERM_PANE") or "").strip()
    if not pane:
        return refuse("$WEZTERM_PANE is unset — this session is not inside a WezTerm pane")
    if not pane.isdigit():
        # The value reaches `wezterm cli kill-pane` as an argument; a non-numeric
        # one could be read as a flag rather than a pane id.
        return refuse(f"$WEZTERM_PANE is not a pane id ({pane!r})")

    found = anchor_task(sid)
    if found is None:
        return refuse(f"no task carries claude_session_id {sid} — nothing proves the work is finished")
    path, meta, body, handed_off = found

    if handed_off:
        reason = f"handed off to {meta.get('claude_session_id') or '(none)'}"
    else:
        terminal, verdict = terminal_verdict(meta, body)
        if not terminal:
            return refuse(f"{os.path.basename(path)}: {verdict}")
        reason = f"{os.path.basename(path)} is {verdict}"

    clear, why = gate_state(sid)
    if not clear:
        return refuse(why)

    print(f"SELF-EXIT: exit — {reason}, no open card, pane {pane}")
    if args.dry_run:
        return EXIT_TAKEN

    try:
        subprocess.run(["wezterm", "cli", "kill-pane", "--pane-id", pane], check=True, timeout=10)
    except (OSError, subprocess.SubprocessError) as exc:
        return refuse(f"kill-pane failed ({exc})")
    return EXIT_TAKEN


if __name__ == "__main__":
    sys.exit(main())
