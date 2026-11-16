---
status: draft
spec: [067-bug-parked-exit-requires-registered-watch]
created: "2026-11-16T23:19:24Z"
branch: dark-factory/bug-parked-exit-requires-registered-watch
---

# Refuse a parked exit unless a registered park watch names the session

<summary>
- A session may only exit "parked" when a registered park watch names it; a park nothing will wake is refused instead of exiting into silence.
- The refusal applies at both accept points: a parked task, and a parked goal or topic page.
- The refusal names the session id and states that no registered park watch exists for it, so it is distinguishable from the existing "defer_date already elapsed" refusal.
- A watch naming a different session does not satisfy the guard.
- Malformed records and `*.lock` files neither break the guard nor count as a watch; each malformed record produces exactly one warning naming the file.
- The real exit path is guarded, not only the `--dry-run` branch.
- On a host where the park mechanism is not installed, the guard fails open with one warning — keyed on the producer's presence, never on the store's readability.
- The `session-close` doctrine tells a parking session to register a watch first, scoped to hosts that have the mechanism.
- A new docs page documents the store contract, the record schema, and the fail-open/fail-closed rules.
- The changelog records the change under `## Unreleased` when it is not already recorded in a version section.
</summary>

<objective>
Make `scripts/session-self-exit.py` refuse to certify a "parked" exit unless a park watch registered by `claude-supervisor`'s `park-event.py` names the session, at both the task and the goal/topic accept points and on the real (non-`--dry-run`) decision path, while failing open on hosts where the producer is not installed. Document the contract and teach registration in `session-close` Phase 4.5.
</objective>

<context>
<!--
OPEN QUESTIONS FOR THE REVIEWER (observed in this checkout on 2026-11-17):

1. This checkout ALREADY contains a complete-looking implementation of every
   requirement below: `claude_root`, `park_producer`, `park_store_dir`,
   `watched_session`, `park_guard` in `scripts/session-self-exit.py`; the stub
   producer and fixtures in `scripts/session-self-exit-test.sh`;
   `docs/park-watches.md`; the Phase 4.5 registration paragraph; and
   `CHANGELOG.md` `## v0.194.2` describing this exact fix. The requirements are
   therefore written as the TARGET CONTRACT, not as "insert this new block".
   The executor MUST read each target file first and, where the code already
   matches the contract, leave it untouched and implement only the gaps. Do not
   duplicate a helper that already exists. Do not "re-implement" a matching
   function to a different shape.

2. `CHANGELOG.md` already carries this change as `## v0.194.2` and there is no
   `## Unreleased` section. If that versioned entry still describes this change
   verbatim, do NOT add a duplicate `## Unreleased` bullet and do NOT rename or
   renumber the version section. Only add an `## Unreleased` bullet when the
   change is genuinely unrecorded.

3. `commands/session-close.md` Phase 4.5 currently contains a sentence saying the
   exit guard "is **not on master yet**". Once this prompt ships the guard, that
   sentence is false and must be removed/replaced (requirement 7).
-->

Read `CLAUDE.md` first, then `docs/dod.md`.

Read these files fully before editing:

- `scripts/session-self-exit.py` — the decision order in the module docstring, `refuse()` (`EXIT_REFUSED = 10`), `defer_instant()`, `terminal_verdict()` (including its `headless` branch), `page_verdict()`, and `main()` (task accept point, goal/topic accept point, and the headless SDK refusal that must stay ahead of the new guard).
- `scripts/session-self-exit-test.sh` — the harness helpers `run`, `run_msg`, `run_live`, `task`, `deferred_task`, `page`, `cards`, the `PLUGROOT`/store-dir setup block near the top, and the existing parked fixtures (the positive parked task, the positive parked goal/topic, and the Z-suffixed parked task).
- `commands/session-close.md` lines 248–388 (`### Phase 4.5` through the line before `### Phase 4.6` at line 389) — especially the `status: in_progress` HARD-flag bullet, which is the park bullet.
- `docs/park-watches.md` if it exists; otherwise you are creating it.
- `CHANGELOG.md` — the preamble and the most recent version section.
- `Makefile` — the `test` target, which runs `bash scripts/session-self-exit-test.sh`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement, required conventional prefix, and the rule that an auto-release repository's feature branch adds an unreleased bullet and does NOT bump version strings or tag.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — verification and completion criteria.
- `/home/node/.claude/plugins/marketplaces/coding/docs/git-workflow.md` — branch/changelog conventions.
</context>

<requirements>
1. **Add the producer/store resolution and the guard to `scripts/session-self-exit.py`.** Place these five functions after `page_verdict()` and before `store_url()`. `glob`, `json`, `os` and `sys` are already imported at the top of the file — do not add imports.

   ```python
   def claude_root() -> str:
       """The resolved `~/.claude` root — the same rule this repo's path helpers use.

       The park *producer* lives under it; the park *store* does not (see
       `park_store_dir`). Resolving the producer through `CLAUDE_CONFIG_DIR` is what
       makes a verify or fleet session read the plugin cache it actually loads.
       """
       return os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")


   def park_producer() -> str | None:
       """`claude-supervisor`'s `park-event.py` under the resolved plugin cache, or None.

       Its presence is the fail-open key: on a host where the producer is not
       installed no park watch can be registered, so refusing there would block
       every park on a mechanism the host does not have. Readability of the store is
       deliberately NOT the test — a stale store directory outlives its plugin, and
       keying on it would silently disable the guard.
       """
       cache = os.path.join(claude_root(), "plugins", "cache", "claude-supervisor")
       hits = sorted(glob.glob(os.path.join(cache, "*", "*", "scripts", "park-event.py")))
       return hits[0] if hits else None


   def park_store_dir() -> str:
       """The park-watch store, fixed by its producer.

       Deliberately NOT derived from `CLAUDE_CONFIG_DIR`, unlike `claude_root`:
       `park-event.py` honours only `$PARK_EVENT_STATE_DIR` and `Path.home()`, so a
       store derived from the config dir would read an empty directory under every
       verify session and refuse parks that are in fact registered.
       """
       raw = (os.environ.get("PARK_EVENT_STATE_DIR") or "").strip()
       if raw:
           return raw
       return os.path.join(os.path.expanduser("~"), ".claude", "state", "park-events")


   def watched_session(sid: str) -> tuple[bool, str]:
       """Whether a registered park watch names this session — and the note to print.

       Read-only: the store is read as files, never written, and `park-event.py` is
       never imported or shelled out to. vault-cli and claude-supervisor are
       separately installed plugins, and this script must keep working when the
       other is absent. Record schema and its failure modes: `docs/park-watches.md`.
       """
       if park_producer() is None:
           return True, (
               f"claude-supervisor's park-event.py is not installed under {claude_root()}"
               "/plugins/cache/claude-supervisor — no park watch can be registered on this host,"
               " so the park guard fails open"
           )
       store = park_store_dir()
       try:
           names = sorted(os.listdir(store))
       except OSError:
           # An absent store is "no watch", never "fail open": a fresh install
           # creates the directory on the first `register`, and conflating the two
           # would let every host whose store was never created bypass the guard.
           names = []
       watched = False
       warnings: list[str] = []
       for name in names:
           if not name.endswith(".json"):
               # `*.lock` files are normal during a concurrent `register`; ignoring
               # them silently is the documented behaviour, not an oversight.
               continue
           path = os.path.join(store, name)
           try:
               with open(path, encoding="utf-8") as handle:
                   record = json.load(handle)
           except (OSError, ValueError):
               warnings.append(f"park watch {path} is unreadable or malformed — skipped")
               continue
           if not isinstance(record, dict) or "sessions" not in record:
               warnings.append(f"park watch {path} carries no `sessions` key — not a watch")
               continue
           members = record.get("sessions")
           if not isinstance(members, list):
               warnings.append(f"park watch {path} has a non-list `sessions` value — not a watch")
               continue
           if sid in members:
               watched = True
       for warning in warnings:
           print(f"WARN: {warning}", file=sys.stderr)
       if watched:
           return True, ""
       return False, (
           f"no registered park watch names session {sid} — a park with no watch never wakes;"
           " register one with `park-event.py register` (see docs/park-watches.md)"
       )


   def park_guard(sid: str, label: str, verdict: str) -> str | None:
       """None when the park may proceed, else the refusal reason.

       Placed on the decision path in `main`, so the real exit is guarded exactly as
       the `--dry-run` is — a guard in the dry-run branch alone would let the live
       path park silently.
       """
       allowed, note = watched_session(sid)
       if not allowed:
           return f"{label}: {verdict}, but {note}"
       if note:
           print(f"WARN: {note}", file=sys.stderr)
       return None
   ```

   If any of these functions already exists, read it and keep it when it matches this contract; change only what diverges.

2. **Guard the task accept point in `main()`.** In the branch where `anchor_task(sid)` resolved a task this session holds, the code currently reads:

   ```python
           if not terminal:
               return refuse(f"{os.path.basename(path)}: {verdict}")
           reason = f"{os.path.basename(path)} is {verdict}"
   ```

   Insert the park guard between those two statements, so the task branch reads:

   ```python
           if not terminal:
               return refuse(f"{os.path.basename(path)}: {verdict}")
           if verdict.startswith("parked until"):
               refused = park_guard(sid, os.path.basename(path), verdict)
               if refused:
                   return refuse(refused)
           reason = f"{os.path.basename(path)} is {verdict}"
   ```

   This placement is load-bearing: it must be **after** `terminal_verdict()` has returned, because the headless-SDK refusal lives inside `terminal_verdict()` (`:449-457`) and moving the guard ahead of it would change the headless wording and break the existing headless test. Do not touch `terminal_verdict()`.

3. **Guard the goal/topic accept point in `main()`.** In the loop over `held` pages, the code currently reads:

   ```python
           for kind, path, meta in held:
               closable, verdict = page_verdict(kind, meta, now)
               if not closable:
                   return refuse(f"{kind} {path}: {verdict}")
               verdicts.append(f"{kind} {path} is {verdict}")
   ```

   Make it read:

   ```python
           for kind, path, meta in held:
               closable, verdict = page_verdict(kind, meta, now)
               if not closable:
                   return refuse(f"{kind} {path}: {verdict}")
               if verdict.startswith("parked until"):
                   refused = park_guard(sid, f"{kind} {path}", verdict)
                   if refused:
                       return refuse(refused)
               verdicts.append(f"{kind} {path} is {verdict}")
   ```

   The headless SDK refusal for goal/topic holders happens earlier in `main()` (before this loop); leave it where it is.

4. **Materialise a stub producer and store fixtures in `scripts/session-self-exit-test.sh`.** The YOLO container has no `claude-supervisor`, so every fixture that must be *refused* needs a stub producer under `CLAUDE_CONFIG_DIR` — otherwise it passes by fail-open. Near the existing `TMP`/`TASKS`/`SESSIONS` setup, add:

   ```bash
   # The park-watch producer and its store. The guard fails OPEN when
   # `park-event.py` is absent, so every fixture that must be refused needs a stub
   # producer under `CLAUDE_CONFIG_DIR` — the YOLO container has none, and a fixture
   # that relied on the host's copy would pass there by fail-open while refusing here.
   PLUGROOT="$TMP/plugroot"
   PARKSTORE="$TMP/park-events"
   NOWATCH="$TMP/park-events-other"
   EMPTYSTORE="$TMP/park-events-empty"
   BADSTORE="$TMP/park-events-bad"
   NOPRODUCER="$TMP/noproducer"
   mkdir -p "$PLUGROOT/plugins/cache/claude-supervisor/supervisor/0.0.0/scripts" \
       "$PARKSTORE" "$NOWATCH" "$EMPTYSTORE" "$BADSTORE" "$NOPRODUCER" "$TMP/home"
   : > "$PLUGROOT/plugins/cache/claude-supervisor/supervisor/0.0.0/scripts/park-event.py"

   park_watch() { # park_watch <file> <session...> — one store record naming these sessions
       local file=$1
       shift
       python3 - "$file" "$@" <<'PY'
   import json, sys
   path, sessions = sys.argv[1], sys.argv[2:]
   with open(path, "w", encoding="utf-8") as handle:
       json.dump({"event": "test-event", "sessions": sessions}, handle)
   PY
   }
   ```

   Then pass the producer root and store to every harness runner. In `run`, `run_msg` and `run_live`, the `env -i` line must include `HOME="$TMP/home" CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$PARKSTORE"` (per-invocation extras passed through `"$@"` override these, because `env -i` runs the whole assignment list and a later duplicate wins). `CLAUDE_CONFIG_DIR=` must appear at least four times in the file in total.

   Create the store records before the parked fixtures:

   ```bash
   OTHER=99999999-8888-7777-6666-555555555555
   park_watch "$PARKSTORE/watched.json" "$SID"
   park_watch "$NOWATCH/other.json" "$OTHER"
   printf '{"sessions": [\n' > "$BADSTORE/broken.json"
   printf '{"event": "drifted"}\n' > "$BADSTORE/drifted.json"
   : > "$BADSTORE/watched.json.lock"
   : > "$BADSTORE/broken.json.lock"
   : > "$BADSTORE/other.json.lock"
   park_watch "$BADSTORE/watched.json" "$SID"
   ```

   (`OTHER` may already be defined for the handover fixtures; if so, reuse it and do not redeclare it.)

5. **Add the guard fixtures to `scripts/session-self-exit-test.sh`.** Keep every existing assertion. Add, in the task section after the positive parked fixture:

   ```bash
   # --- the guard: a park with no registered watch is refused, at both accept points
   deferred_task in_progress execution "$FUTURE"
   run_msg 10 "an unwatched task park refuses" "no registered park watch names session $SID" \
       CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$EMPTYSTORE"

   deferred_task in_progress execution "$FUTURE"
   run_msg 10 "a missing store directory refuses, never fails open" "no registered park watch names session $SID" \
       CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$TMP/no-such-store"

   deferred_task in_progress execution "$FUTURE"
   run_msg 10 "a watch for another session does not count" "no registered park watch names session $SID" \
       CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$NOWATCH"

   deferred_task in_progress execution "$FUTURE"
   run_msg 0 "a watch naming the session survives malformed records and lock files" "parked until $FUTURE" \
       CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$BADSTORE"
   for bad in broken.json drifted.json; do
       if [ "$(grep -c "$bad" "$TMP/out")" -eq 1 ]; then
           pass=$((pass + 1))
       else
           fail=$((fail + 1))
           echo "❌ a malformed store file is warned about exactly once: $bad — $(cat "$TMP/out")" >&2
       fi
   done
   if grep -q 'lock' "$TMP/out"; then
       fail=$((fail + 1))
       echo "❌ lock files are ignored silently — $(cat "$TMP/out")" >&2
   else
       pass=$((pass + 1))
   fi

   deferred_task in_progress execution "$FUTURE"
   run_live 10 "the live exit path is guarded too" \
       CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$EMPTYSTORE"
   if grep -qF 'SELF-EXIT: stay' "$TMP/out"; then
       pass=$((pass + 1))
   else
       fail=$((fail + 1))
       echo "❌ the live exit path prints the refusal — $(cat "$TMP/out")" >&2
   fi

   # --- fail-open is keyed on the PRODUCER's presence, never the store's readability
   deferred_task in_progress execution "$FUTURE"
   run_msg 0 "no producer under CLAUDE_CONFIG_DIR fails open" "park guard fails open" \
       CLAUDE_CONFIG_DIR="$NOPRODUCER" PARK_EVENT_STATE_DIR="$PARKSTORE"
   mkdir -p "$TMP/emptyhome"
   deferred_task in_progress execution "$FUTURE"
   run_msg 0 "no producer under a HOME-only root fails open" "park guard fails open" \
       HOME="$TMP/emptyhome" CLAUDE_CONFIG_DIR=
   ```

   The "no producer under a HOME-only root" fixture is the container's own shape: `env -i` clears `CLAUDE_CONFIG_DIR`, and both `os.path.expanduser("~")` and `Path.home()` follow `HOME`. Do not delete the `mkdir -p "$TMP/emptyhome"`.

   Add the goal/topic guard inside the existing `for kind in goal topic` loop, immediately after the positive parked-page fixture:

   ```bash
       page "$dir" in_progress "$SID" "$FUTURE"
       run_msg 10 "$kind: an unwatched park refuses" "no registered park watch names session $SID" \
           CLAUDE_CONFIG_DIR="$PLUGROOT" PARK_EVENT_STATE_DIR="$EMPTYSTORE"
   ```

   The existing positive parked fixtures (the task one, the goal/topic one, and the Z-suffixed task one) must keep passing. They pass because the runner's default `PARK_EVENT_STATE_DIR` is `$PARKSTORE`, which holds `watched.json` naming `$SID` — a real store read, not fail-open. Do not weaken them by pointing them at an empty store or at a missing producer.

6. **Create `docs/park-watches.md`.** It must document: the definition of a park and a park watch; the store location `$PARK_EVENT_STATE_DIR` defaulting to `~/.claude/state/park-events/`; that the store path is never derived from `CLAUDE_CONFIG_DIR` while the producer root is; the record schema (`event`, `slug`, `recheck`, `sessions`); that `sessions` is the field the guard reads; that the guard is read-only and never imports or shells out to `park-event.py`; a failure-mode table covering producer absent (fail open, one warning), producer present but store dir absent (refuse), producer absent but store readable (fail open), a watch naming this session (allow), watches naming only other sessions (refuse), malformed record (skip, warn once, keep consulting the rest), missing/non-list `sessions` (not a watch, warn once), and `*.lock` (ignored silently); that the refusal names the session id and is distinguishable from the "defer_date already elapsed" refusal; how to register and inspect (`park-event.py register --task … --event … --recheck …`, `list`, `due`); and that the guard is not an enforcement boundary against a hostile session (a session that can write the store can write a record naming itself; pointing the store at an empty directory makes the guard refuse, not bypass).

   `scripts/session-self-exit.py`'s `watched_session()` docstring already references this page; keep that reference.

7. **Teach registration in `commands/session-close.md` Phase 4.5.** Inside the `status: in_progress` HARD-flag bullet (line 290 — it must stay within `### Phase 4.5` lines 248–388, before `### Phase 4.6` at line 389), ensure the paragraph contains the literal `park-event.py register` **and** the word `defer_date` on the same line, and states the precondition: a park is not complete until a watch names the session; on a host that has the park mechanism the session must run `park-event.py register --task "<task>" --event "<what is being waited on>" --recheck "<the same defer_date>"` before parking.

   Scope the instruction to hosts that have the mechanism: "on a host where the mechanism is not installed — no `park-event.py` under the resolved plugin cache — this step does not apply, the same host class the exit guard fails open on."

   Remove the now-false sentence in that bullet that says the exit guard "is **not on master yet**" and that "until it ships, the exit is accepted silently". Replace it with a statement that the exit guard refuses such a park, so a session that parks without a watch is refused with a message naming the session and the missing watch. Do not append a new bullet elsewhere in the file; the instruction must sit in the park bullet itself.

8. **Record the change in `CHANGELOG.md`.** If the change is not already recorded, insert an `## Unreleased` section immediately after the preamble's SemVer bullet block and before the first `## vX.Y.Z` section:

   ```
   ## Unreleased

   - fix: `session-self-exit.py` refuses to exit a parked session unless a registered park watch names it — a park with no watch never wakes, so the exit is refused with a message naming the session and the missing watch. Fail-open is keyed on the park producer's presence, never on the store's readability; see `docs/park-watches.md`
   ```

   Exactly one `## Unreleased` heading may exist after the edit. If a recent versioned section already records this exact change (this checkout has `## v0.194.2`), do NOT duplicate it, do NOT rename the section, and do NOT bump any version string.
</requirements>

<constraints>
- **The guard is read-only.** It must never write to, create, or delete anything under the park store.
- **Read the store as files.** Do not shell out to `park-event.py` and do not import it. vault-cli and claude-supervisor are separately installed plugins, and vault-cli must keep working when the other is absent.
- **`EXIT_REFUSED = 10` is frozen.** Do not renumber it; 53 assertions in `scripts/session-self-exit-test.sh` depend on it.
- **The store path and the producer root resolve by different rules.** `claude_root()` uses `CLAUDE_CONFIG_DIR` else `~/.claude`; `park_store_dir()` uses only `PARK_EVENT_STATE_DIR` else `~/.claude/state/park-events`. Do not derive the store from `CLAUDE_CONFIG_DIR`.
- **Fail open only when the producer is absent.** A present producer with an absent store directory is "no watch" and must refuse. Never conflate the two.
- **Do not change the park-due step** in `claude-supervisor`; reading registered watches only is correct and out of scope.
- **Do not change what counts as a park.** The definition stays `in_progress` plus a `defer_date` parsing to a future instant. A past, date-only, offset-less or unparseable `defer_date` keeps its current refusal wording, and the existing "no session id" refusal in `main()` stays exit 10 — do not relax it into a fail-open.
- **Do not move the guard ahead of the headless-SDK refusals.** For a task holder the headless branch lives inside `terminal_verdict()`; for a goal/topic holder it is earlier in `main()`. Both must keep their current wording and test expectations.
- **Do not delete or weaken any existing assertion** in `scripts/session-self-exit-test.sh`. The change is additive to the harness apart from the three runner env lines.
- **Do not hand-bump version manifests or create a tag.** `.maintainer.yaml` has `release.autoRelease: true`; the releaser owns the version bump. `.claude-plugin/plugin.json` and both `.claude-plugin/marketplace.json` version fields stay as they are.
- Do not modify any file under `scenarios/`.
- Do not commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` once — it must exit 0 (its `test` target runs `bash scripts/session-self-exit-test.sh`).

Then run `make test` — it must exit 0.

Then confirm the edit surface by reading the counts (do not rely on exit codes; `grep -c` exits 1 on zero matches, so a broken expectation and a passing one can look alike):

```
grep -c 'def park_producer' scripts/session-self-exit.py          # expect 1
grep -c 'def park_store_dir' scripts/session-self-exit.py         # expect 1
grep -c 'def watched_session' scripts/session-self-exit.py        # expect 1
grep -c 'def park_guard' scripts/session-self-exit.py             # expect 1
grep -c 'park_guard(' scripts/session-self-exit.py                # expect 3 (definition + two accept points)
grep -c 'CLAUDE_CONFIG_DIR=' scripts/session-self-exit-test.sh    # expect >= 4
grep -c 'no registered park watch names session' scripts/session-self-exit-test.sh  # expect >= 3 (task + goal + topic)
grep -c 'park guard fails open' scripts/session-self-exit-test.sh # expect 2
grep -c 'the live exit path is guarded too' scripts/session-self-exit-test.sh       # expect 1
grep -n 'park-event.py register' commands/session-close.md        # >= 1 line inside Phase 4.5, carrying defer_date
grep -c '^## Unreleased' CHANGELOG.md                             # 1 if the change was unrecorded, 0 if already in a version section
```

Also confirm the existing positive parked fixtures still pass in the harness output (the runner defaults to `PARK_EVENT_STATE_DIR="$PARKSTORE"`, which holds a watch naming `$SID`).

**Do not run `git` commands.** This checkout masks `.git`; the zero-deletion baseline check (`git diff --numstat origin/master -- scripts/session-self-exit-test.sh`) is operator-side and is recorded in the spec's Verification section. Verify the no-deletion requirement structurally instead: every pre-existing `run`, `run_msg` and `run_live` call in the harness is still present.

If `bash scripts/session-self-exit-test.sh` fails at the stub-store readiness probe with a proxy `403 Filtered`, re-run it as `NO_PROXY=127.0.0.1,localhost bash scripts/session-self-exit-test.sh` — the probe uses a localhost URL and the container proxy intercepts it.
</verification>
