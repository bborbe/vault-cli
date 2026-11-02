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
  2. The session owns the WezTerm pane in `$WEZTERM_PANE`. A registry entry is
     NOT proof: a headless worker is a real `claude --print` process (registry
     `entrypoint: sdk-cli`) with a pid and an entry, and it INHERITS its
     spawner's `$WEZTERM_PANE`, so killing that pane would close the manager's
     tab. Ownership is proven by the tty: the pane's `tty_name` from
     `wezterm cli list --format json` must equal the controlling tty of the
     registry entry's `pid`. A headless worker has no controlling tty, so it
     never matches. This is the guard that makes the exit safe.
  3. The session's anchor task exists — resolved by `claude_session_id`, or, when
     that id has moved to another session, by the session appearing in the task's
     `metrics_sessions` (a recorded handover).
  4. The task is terminal on disk: `status: aborted`, or `status: completed`
     with `phase: done` and zero `[ ]` / `[/]` boxes. `hold` does not qualify —
     it is parked, not finished. A handed-off session skips this guard: its own
     work is on the record, so the successor's progress is not its proof.
     An `in_progress` task qualifies too, but only when it carries a
     `defer_date` that parses to a **future instant** — that is the park shape,
     where the session has nothing left to do this turn and a re-check time is
     recorded. A past or date-only `defer_date` is not an instant and refuses:
     a stale row must not free its slot, and the time-of-day reading of a
     date-only value belongs to its own task.
  5. The attention store holds no open card for this session. An unreachable
     store refuses: a failed read is not evidence that no gate is open. A card
     the session minted at its own `Stop` is not a card: it is the turn-end
     closer report, and its `cleared_by` is `SessionEnd` — the very event this
     gate is delaying. Refusing on it is a deadlock by construction, so it is
     skipped. Every other open card still refuses.

Only then does it run `wezterm cli kill-pane --pane-id <own pane>`, which ends
this process. `--dry-run` prints the same decision without killing.

    exit 0   exit taken (or, under --dry-run, would be taken)
    exit 10  refused — the named reason is on stdout
    exit 2   usage / unreadable input

⚠️ Never recommend or take a change to `spawn.maxConcurrent` here: this frees a
slot nobody is using, it does not open the limit.

Testability: every input is an env var (`SESSION_ID`, `SESSIONS_DIR`,
`TASKS_ROOTS`, `WEZTERM_PANE`, `WEZTERM_PANES_FILE`, `PID_TTY_FILE`,
`ATTENTION_STORE_URL`, `ATTENTION_STATE_DIR`) so
`scripts/session-self-exit-test.sh` can drive each branch against fixtures
without a live fleet.
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
from datetime import datetime, timezone

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


def registry_record(sid: str) -> dict | None:
    """The session-registry record whose `sessionId` is `sid`, or None."""
    for path in glob.glob(os.path.join(sessions_dir(), "*.json")):
        try:
            with open(path, encoding="utf-8") as fh:
                record = json.load(fh)
        except (OSError, ValueError):
            continue
        if isinstance(record, dict) and record.get("sessionId") == sid:
            return record
    return None


def _norm_tty(value: str) -> str:
    """`ttys002` / `/dev/ttys002` → `/dev/ttys002`; no tty (`??`, `-`, empty) → ``."""
    value = (value or "").strip()
    if not value or value in ("??", "?", "-"):
        return ""
    return value if value.startswith("/dev/") else "/dev/" + value


def pane_tty(pane: str) -> str:
    """The tty WezTerm reports for `pane`, or `` when it cannot be read.

    Source: `$WEZTERM_PANES_FILE` (a saved `wezterm cli list --format json`) when
    set, else the live command. Any failure reads as no tty, which refuses.
    """
    try:
        listing_file = os.environ.get("WEZTERM_PANES_FILE")
        if listing_file:
            with open(listing_file, encoding="utf-8") as fh:
                panes = json.load(fh)
        else:
            out = subprocess.run(
                ["wezterm", "cli", "list", "--format", "json"],
                capture_output=True, text=True, check=True, timeout=10,
            ).stdout
            panes = json.loads(out)
    except (OSError, ValueError, subprocess.SubprocessError):
        return ""
    for row in panes if isinstance(panes, list) else []:
        if isinstance(row, dict) and str(row.get("pane_id")) == pane:
            return _norm_tty(str(row.get("tty_name") or ""))
    return ""


def pid_tty(pid: int) -> str:
    """The controlling tty of `pid`, or `` when it has none or cannot be read.

    Source: `$PID_TTY_FILE` (a JSON map of pid → tty) when set, else `ps`.
    """
    try:
        map_file = os.environ.get("PID_TTY_FILE")
        if map_file:
            with open(map_file, encoding="utf-8") as fh:
                return _norm_tty(str(json.load(fh).get(str(pid)) or ""))
        out = subprocess.run(
            ["ps", "-o", "tty=", "-p", str(pid)], capture_output=True, text=True, timeout=10
        ).stdout
    except (OSError, ValueError, AttributeError, subprocess.SubprocessError):
        return ""
    return _norm_tty(out)


def owns_pane(sid: str, pane: str) -> tuple[bool, str]:
    """True only when `pane`'s tty is the controlling tty of this session's process.

    A registry entry alone proves nothing: a headless SDK-CLI worker has a pid and
    an entry too, and it inherits its spawner's `$WEZTERM_PANE`. Ownership is the
    tty match — the pane's `tty_name` from `wezterm cli list` against the tty of
    the registry `pid`. A headless worker has no controlling tty, so it never
    matches. Every unreadable input refuses.
    """
    record = registry_record(sid)
    if record is None:
        return False, "no session-registry entry for this session — nothing ties it to a process"
    pid = record.get("pid")
    if not isinstance(pid, int) or isinstance(pid, bool) or pid <= 0:
        return False, f"the registry entry carries no usable pid ({pid!r})"
    ptty = pane_tty(pane)
    if not ptty:
        return False, f"pane {pane} is not owned by this session — wezterm lists no tty for it"
    own = pid_tty(pid)
    if not own:
        return False, (
            f"pane {pane} is not owned by this session — pid {pid} has no controlling tty "
            "(a headless worker inherits its spawner's $WEZTERM_PANE; killing it would close that tab)"
        )
    if own != ptty:
        return False, f"pane {pane} is not owned by this session — pane tty {ptty}, pid {pid} tty {own}"
    return True, f"pane {pane} on {ptty}"


def entrypoint_of(sid: str) -> str:
    """The registry entry's `entrypoint`; `sdk-cli` marks a headless SDK query.

    Read separately from `owns_pane`, which never consults it: pane ownership is
    the tty match alone. The park branch keeps this as a second, independent
    refusal for headless workers.
    """
    for path in glob.glob(os.path.join(sessions_dir(), "*.json")):
        try:
            with open(path, encoding="utf-8") as fh:
                record = json.load(fh)
        except (OSError, ValueError):
            continue
        if record.get("sessionId") == sid:
            return str(record.get("entrypoint") or "").strip()
    return ""


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


# A bare `YYYY-MM-DD` is a date, not an instant. `datetime.fromisoformat` accepts
# it and silently returns midnight, which would read a same-day park as "already
# elapsed" — so it is rejected before parsing.
DATE_ONLY = re.compile(r"^\d{4}-\d{2}-\d{2}$")


def defer_instant(value: str) -> datetime | None:
    """The aware instant a `defer_date` names, or None when it names none.

    None covers absent, date-only, unparseable, and offset-less values. An
    offset-less value is ambiguous — it could be any timezone — and guessing one
    would let a stale row free its slot, so it is not an instant either.
    """
    text = (value or "").strip()
    if not text or DATE_ONLY.match(text):
        return None
    # Only the trailing designator is rewritten; a bare `.replace` would rewrite
    # any `Z` anywhere in the string.
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(text)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return None
    return parsed


def terminal_verdict(
    meta: dict[str, str], body: str, now: datetime, headless: bool = False
) -> tuple[bool, str]:
    status = meta.get("status", "")
    if status == "aborted":
        return True, "aborted"
    defer = meta.get("defer_date", "").strip()
    if status == "in_progress" and defer:
        if headless:
            # A headless SDK query has no pane of its own and inherits its
            # spawner's $WEZTERM_PANE, so "exiting" here would close the
            # spawner's tab. It does not need this script at all: its process
            # ends when its turn ends.
            return False, (
                "task is parked, but this session is a headless SDK query with no pane of its own "
                "— it inherits its spawner's $WEZTERM_PANE, so exiting would close that tab"
            )
        instant = defer_instant(defer)
        if instant is None:
            return False, (
                f"task is in_progress and defer_date {defer!r} is not a future instant "
                "— a date-only, offset-less or unparseable value is not a park"
            )
        if instant <= now:
            return False, f"task is in_progress and defer_date {defer!r} has already elapsed"
        return True, f"parked until {defer}"
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


def state_dir() -> str:
    """The directory the attention hook writes its per-session logs to.

    Deliberately byte-identical to `attention-log.py`'s own line — `ATTENTION_STATE_DIR`,
    else `~/.claude/state/attention` — and deliberately NOT derived from
    `CLAUDE_CONFIG_DIR`, which this repo's other path helpers do honour. Mirroring
    the producer is the whole requirement: if the hook does not follow
    `CLAUDE_CONFIG_DIR`, following it here would point the join at a directory the
    hook never writes, and the carve-out would silently stop firing. If the hook
    ever moves, both move together.
    """
    return os.environ.get("ATTENTION_STATE_DIR") or os.path.expanduser("~/.claude/state/attention")


def mint_events(sid: str) -> dict[str, bool] | None:
    """`item_id` → True when every record under it is a closer; None when the log is unreadable.

    The store cannot say which event minted an item: `attention-watcher.py` forwards
    neither `event` nor `cleared_by`, so a `Stop` closer report and a blocking
    `AskUserQuestion` reach it byte-identically (`interrupt_class: pick`,
    `provenance_class: hook`, `answer_mechanism: message`). The hook's per-session
    log is where that provenance still lives, joined on the store's `dedup_key` —
    which the watcher sets to the hook's own `item_id` (`attention-watcher.py`
    `"dedup_key": dedup_key or item["item_id"]`).

    Returns None rather than `{}` when the log cannot be read, because the caller
    must not read an absent log as "nothing was a closer".
    """
    # The sid reaches a filesystem path; a charset guard keeps a `../` or an
    # absolute value from resolving outside the state directory. Not a trust
    # boundary — the caller is the session being gated — but free.
    if not re.fullmatch(r"[A-Za-z0-9-]+", sid):
        return None
    path = os.path.join(state_dir(), f"{sid}.events.jsonl")
    records: dict[str, bool] = {}
    # `errors="replace"` rather than a `UnicodeDecodeError` catch: a hook killed
    # mid-write leaves a truncated multi-byte sequence, and the decode happens
    # during iteration — outside the per-line guard. Replacing makes that line
    # fail `json.loads` like any other corruption, so it is skipped rather than
    # turning a documented named refusal into a traceback.
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            for line in fh:
                try:
                    rec = json.loads(line)
                except ValueError:
                    continue
                # Every value here is parsed JSON, so nothing about its shape is
                # guaranteed. `null`, `123` and `[1,2]` parse and then raise
                # AttributeError on `.get`; an `item_id` that parses as an object
                # or array is unhashable and raises TypeError as a dict key. Both
                # are tracebacks where the contract promises a named refusal.
                if not isinstance(rec, dict):
                    continue
                item_id = rec.get("item_id")
                if not isinstance(item_id, str) or not item_id:
                    continue
                if rec.get("type") != "open":
                    continue
                # Fail-closed on duplicates: an id counts as a closer only when
                # EVERY record under it is one. Last-wins would let a later `Stop`
                # mint that happens to share an id with a real ask resolve to the
                # closer and skip a live card. This cannot, in either order —
                # which matters because `item_id` hashes the ask's own text, so a
                # closer whose line matches a question would collide.
                records[item_id] = records.get(item_id, True) and is_closer_report(rec)
    except OSError:
        return None
    return records


def is_closer_report(rec: dict | None) -> bool:
    """True when this record is the session's own turn-end closer, not an ask.

    Only the `Stop` hook mints a closer, and it mints it as a `question`. That
    card is a report of what the session would do next; nothing is waiting on it,
    and only the session's own end clears it — so a gate that refuses on it can
    never be satisfied by the session it gates.

    The predicate keys on `event`, never on `cleared_by`: current mints declare
    `{"event": "SessionEnd"}`, but older ones in the same append-only log declare
    `{"event": "UserPromptSubmit"}` (measured 2026-11-02 — both shapes sit in one
    session's log). `event: Stop` is stable across every mint this hook has made;
    `cleared_by` is not.
    """
    if not rec:
        return False
    return rec.get("kind") == "question" and rec.get("event") == "Stop"


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
    # `isinstance(i, dict)` for the same reason as in `mint_events`: the store
    # returns parsed JSON, and a `null` element is a legal JSON value that has no
    # `.get`.
    mine = [
        i for i in items
        if isinstance(i, dict) and i.get("producer_id") == sid and i.get("state") == "open"
    ]
    if not mine:
        return True, "no open card"
    events = mint_events(sid)
    closers = 0
    for item in mine:
        # An unreadable log leaves every item's provenance unproven, so the
        # closer branch simply never fires and the gate refuses as it always did.
        # The store is the other unguarded input. `isinstance` rather than `str()`:
        # a non-string key cannot be a hook `item_id` at all, so treating it as one
        # is a coercion that could in principle be crafted to match. A non-string
        # (or an unhashable object/array, which `str()` would have to survive) now
        # simply fails to join, and the gate refuses by name.
        key = item.get("dedup_key")
        is_closer = events.get(key) if events is not None and isinstance(key, str) else None
        if is_closer is True:
            closers += 1
            continue
        return False, f"open attention card {item.get('item_id')}"
    return True, f"no open card ({closers} closer report(s) ignored)"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--session", help="session id (default $CLAUDE_CODE_SESSION_ID)")
    parser.add_argument("--dry-run", action="store_true", help="print the decision without killing the pane")
    args = parser.parse_args()

    sid = session_id(args.session)
    if not sid:
        return refuse("no session id — pass --session or set $CLAUDE_CODE_SESSION_ID")

    pane = (os.environ.get("WEZTERM_PANE") or "").strip()
    if not pane:
        return refuse("$WEZTERM_PANE is unset — this session is not inside a WezTerm pane")
    if not pane.isdigit():
        # The value reaches `wezterm cli kill-pane` as an argument; a non-numeric
        # one could be read as a flag rather than a pane id.
        return refuse(f"$WEZTERM_PANE is not a pane id ({pane!r})")

    owned, why = owns_pane(sid, pane)
    if not owned:
        return refuse(why)

    found = anchor_task(sid)
    if found is None:
        return refuse(f"no task carries claude_session_id {sid} — nothing proves the work is finished")
    path, meta, body, handed_off = found

    if handed_off:
        reason = f"handed off to {meta.get('claude_session_id') or '(none)'}"
    else:
        terminal, verdict = terminal_verdict(
            meta, body, datetime.now(timezone.utc), entrypoint_of(sid) == "sdk-cli"
        )
        if not terminal:
            return refuse(f"{os.path.basename(path)}: {verdict}")
        reason = f"{os.path.basename(path)} is {verdict}"

    clear, note = gate_state(sid)
    if not clear:
        return refuse(note)

    print(f"SELF-EXIT: exit — {reason}, {note}, pane {pane}")
    if args.dry_run:
        return EXIT_TAKEN

    try:
        subprocess.run(["wezterm", "cli", "kill-pane", "--pane-id", pane], check=True, timeout=10)
    except (OSError, subprocess.SubprocessError) as exc:
        return refuse(f"kill-pane failed ({exc})")
    return EXIT_TAKEN


if __name__ == "__main__":
    sys.exit(main())
