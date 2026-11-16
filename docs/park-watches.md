# Park watches

A **park** is a session that ends its own pane while its task stays `in_progress` with a future `defer_date`. A **park watch** is the record that says who will wake it. This page is the contract between the two plugins that share it: `claude-supervisor` produces and reads park watches, and `vault-cli`'s `session-self-exit.py` reads them to decide whether a park may proceed.

The rule this page exists for: **a park with no watch never wakes.** A session that exits into silence is indistinguishable from one that finished, so the exit is refused instead.

## Where the store lives

`$PARK_EVENT_STATE_DIR`, defaulting to `~/.claude/state/park-events/`. One JSON file per event key, plus `*.lock` files that are normal during a concurrent `register`.

The store path is fixed by its producer and is **never** derived from `CLAUDE_CONFIG_DIR`. `park-event.py` honours only `PARK_EVENT_STATE_DIR` and `Path.home()`, so a store derived from the config dir would read an empty directory under every verify or fleet session and refuse parks that are in fact registered. The producer's *location* and the store's *path* resolve by different rules — do not conflate them.

## Record schema

```json
{
  "event": "<event key — what the wait is on>",
  "slug": "<filename stem>",
  "recheck": "2026-11-16T02:00:00+01:00",
  "sessions": ["11111111-2222-3333-4444-555555555555"]
}
```

`sessions` is the field the guard reads: a session may park only when its id appears in at least one record's `sessions` list. A record naming a different session does not count.

## The guard

`scripts/session-self-exit.py` consults the store at both accept points — a parked **task** and a parked **goal/topic** page — and on the decision path, so the real exit is guarded exactly as `--dry-run` is. It is **read-only**: it never writes, creates or deletes anything under the store, and it never imports or shells out to `park-event.py`, because vault-cli must keep working when claude-supervisor is not installed.

| Condition | Behaviour |
|---|---|
| No `park-event.py` under the resolved plugin-cache root | **Fail open** — exit 0 with one warning. The host has no park mechanism, so a refusal would block every park on a mechanism it does not have. |
| Producer present, store directory absent | **Refuse.** A missing directory is "no watch", not "fail open" — a fresh install creates it on the first `register`, and conflating the two would let any host whose store was never created bypass the guard. |
| Producer absent, store present and readable | Fail open, one warning. Readability is not the test; a stale directory outlives its plugin and is inert. |
| A record naming this session | **Allow** — `parked until <defer_date>`, exit 0. |
| Records naming only other sessions | **Refuse**, naming the session and the missing watch. |
| A malformed record | Skipped, one warning naming the file; the remaining records are still consulted. |
| A record with no `sessions` key, or a non-list value | Treated as **not a watch**, one warning naming the file. Never absorbed silently: a schema change would otherwise refuse every park for ever with no signal. |
| A `*.lock` file | Ignored entirely, no warning. |

The refusal is distinguishable from the existing "`defer_date` already elapsed" refusal by its wording, which names the session id and states that no registered park watch exists for it.

## Registering and inspecting

```bash
python3 ~/.claude/plugins/cache/claude-supervisor/supervisor/*/scripts/park-event.py register \
  --task "<task name>" --event "<what it waits on>" --recheck "<ISO instant>"
python3 .../park-event.py list    # every watch
python3 .../park-event.py due     # watches whose recheck instant has passed
```

`commands/session-close.md` Phase 4.5 names registration as a precondition of parking, so a session following the doctrine does not hit the refusal blind.

## What this is not

The guard closes the **accidental** silent park. It is not an enforcement boundary against a hostile session: `PARK_EVENT_STATE_DIR` is caller-controlled, and a session that can write the store can write a record naming itself and then pass. That is accepted — anyone who can write the store already controls the machine. Note the direction: pointing the store at an empty directory does **not** bypass the guard, it makes the guard refuse.
