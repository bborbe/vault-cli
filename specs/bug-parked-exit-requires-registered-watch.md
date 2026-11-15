---
status: draft
kind: bug
---

## Summary

- A session can exit "parked" carrying only a `defer_date`, with no park watch registered to wake it.
- The park-due step reads registered watches only, so such a session is invisible to it rather than refused by it.
- The exit path accepts a park at two independent points, and neither consults the watch store.
- The result is a session that nothing **wakes** — it sits idle indefinitely past its own `defer_date`. (A fleet sweep does see the row as deferred, but deferral alone never brings it back.)

## Problem

Parking is the contract that lets a session free its slot and be woken later: `session-close` Phase 4.5 and `scripts/session-self-exit.py` both define "parked" as `in_progress` carrying a `defer_date` that parses to a future instant, and treat that as sufficient to exit clean. But the wake half of that contract is owned by a different component — the park-due step, which reads a store of registered watches and never looks at `defer_date` at all. So the exit path can certify a park that nothing will ever wake, and both halves are individually correct. The failure is silent: no error is raised, no line is logged, and the only symptom is a session sitting idle long after the time it said it would come back.

**Cost of not fixing:** parks continue to exit silently, and the operator discovers an idle session hours late — on 2026-11-13, roughly 80 minutes late and only because a fleet round happened to look.

## Reproduction

> **Correction (2026-11-15).** An earlier revision of this spec asserted `Observed: parked until <defer_date>, exit=0` for a two-line setup that was never run. **That claim is retracted.** Two later revisions also carried unrun reproductions: the `task` helper has no `defer_date` parameter, so the fixture shape exited 10 on `task status is in_progress, not completed or aborted`. Everything asserted below as observed was executed, and the exact environment is given.

**Reported incident — 2026-11-13, operator-observed, no transcript retained.** Worker `b652848f` held `defer_date: 2026-11-13T05:56:00+01:00` (04:56Z). A fleet round ran at 05:45Z and the worker was still idle at 06:15Z. `park-event.py list` and `park-event.py due` both returned nothing for it. The earlier watch had been consumed by a 00:57Z hand-resume, and the replacement `defer_date` was written without `park-event.py register`.

### Produced observation

`main()` refuses on pane ownership at `:662-666` before any park accept point, so a bare task file is not enough — the harness env is required. Running the script with a parked task, a live pane, and an **empty** `PARK_EVENT_STATE_DIR`:

```
SELF-EXIT: exit — Anchor.md is parked until 2026-11-15T13:28:04.424486+00:00, no open card, pane 777
exit=0
```

That is the defect: the exit is **taken** with no watch lookup. The environment that produced it: `SESSION_ID`, `SESSIONS_DIR` (registry pid 4242 → tty `ttys777`), `TASKS_ROOTS` holding a task with `status: in_progress`, `phase: execution`, `claude_session_id: <SID>` and a future `defer_date`; `WEZTERM_PANE=777` with `WEZTERM_PANES_FILE` / `PID_TTY_FILE` agreeing; `ATTENTION_STORE_URL` on a stub returning `[]`; `ATTENTION_STATE_DIR`; and `PARK_EVENT_STATE_DIR` pointing at an empty directory.

**Harness form.** The fixture must use the existing helpers, and must pass the store *per invocation* — `export` does not reach the child, because `run`/`run_msg`/`run_live` run under `env -i` (verified: an exported variable reads as `None` inside `env -i`):

```bash
deferred_task in_progress execution "<future-instant>"     # :103 — the park shape; `task` has no defer_date
run_msg 0 "unwatched park is accepted today (the bug)" "parked until" \
    PARK_EVENT_STATE_DIR="$EMPTY_STORE"
```

The wake half cannot see it. Resolve `park-event.py` to exactly one path — the glob over versions expands to **188+** files on the authoring host and python3 then treats the rest as argv, failing with `invalid choice`:

```bash
PARK_EVENT=$(ls -d "${CLAUDE_CONFIG_DIR:-$HOME/.claude}"/plugins/cache/claude-supervisor/supervisor/*/ | sort -V | tail -1)scripts/park-event.py
PARK_EVENT_STATE_DIR="$EMPTY_STORE" python3 "$PARK_EVENT" list    # → no park watches
PARK_EVENT_STATE_DIR="$EMPTY_STORE" python3 "$PARK_EVENT" due     # → no park watches due at <now>
```

Against the **live** store (no `PARK_EVENT_STATE_DIR`), `list` prints 31 records and `due` prints several — the empty-store prefix is what makes the above meaningful.

## Expected vs Actual

**Expected** — per `commands/session-close.md:783` (*"parked — `status: in_progress` carrying a `defer_date` that parses to a future instant"*) and `:698` (*"A parked task is the exception … reaches the clean verdicts"*), parking is granted *because* a re-check time is recorded, so the session resumes without the operator. A park that nothing will wake does not satisfy that justification, so the exit path must not certify it.

**Actual** — `scripts/session-self-exit.py` accepts the park on the `defer_date` alone. Neither accept point reads the watch store, and the string `park-event` does not occur in the file.

## Why this is a bug

The two halves of the park contract are owned by separate components, and only one of them can observe whether the other half exists. The park-due step is correct to read registered watches only — it has no business scanning task files for stray `defer_date`s. That leaves the exit path as the only place the asymmetry is visible, and it does not look. The documented behaviour (a park frees the slot *and* the session resumes) is therefore not what the system does.

## Assumptions

- Each park record carries a `sessions` list of session ids. Verified against `claude-supervisor` 0.315.0: `sessions` is **written** at `scripts/park-event.py:533` and **read** at `:508`, with further reads at `:562`, `:598`, `:626`, `:643`, `:653`. Cite the symbol, not a line — this file moves between versions.
- **"The supervisor plugin is absent" is observable as "no `park-event.py` under the plugin cache" — NOT as "the store directory is unreadable".** The obvious inverse is false: `~/.claude/state/park-events/` survives an uninstall, and on the authoring host it holds 31 records and 398 lock files with the plugin present, so it would remain readable and non-empty after removal.
- **The producer root must be resolved explicitly, and it differs by environment.** Use `os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")` — **not** the shell idiom `${CLAUDE_CONFIG_DIR:-$HOME/.claude}`, which under `env -i` yields `/.claude` because `HOME` is empty, failing open in every test. On the authoring host the producer is present under `/Users/bborbe/.claude`. **In the YOLO container it is absent**: `~/.claude-yolo/plugins/cache/` contains only `coding` and is mounted as `/home/node/.claude`, and `env -i` clears `CLAUDE_CONFIG_DIR`, so `Path.home()` resolves to `/home/node`. This is why the harness must materialise a stub producer (see Constraints) — otherwise every refusal fixture fails open in the container, where `make test` actually runs.
- The refusal exit code is a frozen external contract at `EXIT_REFUSED = 10` (`scripts/session-self-exit.py:94`), asserted by **53** call sites in `scripts/session-self-exit-test.sh` (27 `run 10` + 25 `run_msg 10` + 1 `run_live 10`). It must not be renumbered.

## Goal

No session exits "parked" unless a registered watch names it — on any host where the park mechanism exists. When the watch is missing, the exit is refused with a message naming the session and the missing watch, so the session either registers a watch and exits, or stays alive and reports — never exits into silence. (The one host class outside this Goal is the one where the producer is not installed, handled by DB 8.)

## Acceptance Criteria

- [ ] **Task holder refused.** A session whose task is `in_progress` with a future `defer_date` and no watch naming it is refused. Evidence: a `run_msg 10` fixture on `deferred_task in_progress execution "<future>"` prints `SELF-EXIT: stay — ` plus a reason naming `<SID>` and the missing park watch.
- [ ] **Goal/topic holder refused.** The same refusal reproduces for a page whose `claude_session_id` is `<SID>` with `status: in_progress` and a future `defer_date`. Evidence: a `run_msg 10` fixture on the goal/topic roots.
- [ ] **Positive control — a watched park still exits.** Evidence: with a record for `<SID>` registered under `PARK_EVENT_STATE_DIR`, a `run_msg 0` fixture prints `parked until <defer_date>`; and `park-event.py due` returns that record once its recheck instant passes.
- [ ] **Negative control — an unrelated watch does not count.** Evidence: with the store holding a valid record for a different session only, the `<SID>` fixture still exits 10.
- [ ] **Malformed records and lock files are tolerated.** Evidence: with one malformed `.json`, three `*.lock` files, and a valid record for `<SID>`, the fixture exits 0 with `parked until …`, and the captured stderr carries exactly one warning naming the malformed file.
- [ ] **The real exit path shares the guard.** Evidence: a `run_live 10` fixture (`:221-235`, no `--dry-run`) for an unwatched park prints `SELF-EXIT: stay — …` — a guard placed only in the `--dry-run` branch fails this.
- [ ] **Fail-open is keyed on producer presence.** Evidence: with `CLAUDE_CONFIG_DIR` pointing at a temp root whose plugin cache contains no `park-event.py`, the fixture exits 0 plus exactly one warning naming the absent producer — never a refusal, even when the store directory exists and is readable.
- [ ] **Doctrine names registration.** Evidence: `grep -n 'park-event.py register' commands/session-close.md` returns ≥ 1 line inside Phase 4.5.
- [ ] **No regression, no deleted assertions, producer stubbed for every refusal fixture.** Evidence: `make precommit` exits 0; `git diff --numstat scripts/session-self-exit-test.sh` shows **0 deletions**; and `grep -c 'CLAUDE_CONFIG_DIR=' scripts/session-self-exit-test.sh` returns ≥ 4 (the stub is wired into each fixture that must see the producer present, so the four existing positive parked fixtures at `:314`, `:338`, `:568` ×2 pass via a real store read rather than via fail-open).

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0. Target is `ensure format generate test check addlicense`; there is no shellcheck in it, and `check-versions` runs only under `release-check`. `test` invokes `bash scripts/session-self-exit-test.sh` (`Makefile:80`), which is why the stub producer is mandatory here.
- `make test` — exits 0
- `bash scripts/session-self-exit-test.sh` — exits 0
- `grep -n 'PARK_EVENT_STATE_DIR\|park-event' scripts/session-self-exit.py` — returns ≥ 1 line per guarded accept point
- `git diff --numstat scripts/session-self-exit-test.sh` — 0 deletions

### Operator-executable (runs on the host after PR merge and plugin reload)

- `claude plugin update vault-cli@vault-cli`, then the operator runs `/reload-plugins`
- Re-run the produced observation against the **loaded** plugin copy under `~/.claude/plugins/cache/vault-cli/vault-cli/<version>/scripts/`. The discriminating signal is the **message**: the unwatched fixture must print `SELF-EXIT: stay — … no registered park watch`, where it previously printed `SELF-EXIT: exit — … parked until …`. An exit-code-only comparison is a false pass, because the pane-ownership refusal also exits 10.

## Desired Behavior

1. A pane-owning session whose task is `in_progress` with a future instant `defer_date`, and whose session id appears in no park watch, is refused exit with `EXIT_REFUSED` (10).
2. The same refusal applies when the session holds a goal or topic page rather than a task — both holder kinds are guarded.
3. The refusal message names the session id and states that no registered park watch exists for it, and is distinguishable from the existing "defer_date already elapsed" refusal.
4. A session whose id appears in at least one park watch exits exactly as it does today: `parked until <defer_date>`, status 0.
5. A watch belonging to a different session does not satisfy the guard.
6. A malformed record or a `*.lock` file neither breaks the guard nor counts as a watch; a malformed record produces exactly one warning naming the file.
7. The guard is on the decision path, not the `--dry-run` branch — the real exit is guarded too.
8. Fail-open is keyed on the producer's presence under the resolved plugin-cache root, not on the store's readability: with no `park-event.py` installed the guard exits 0 with one warning, even when a stale store directory exists and is readable.
9. `commands/session-close.md` Phase 4.5 names `park-event.py register` as a precondition of parking, so a session following the doctrine does not hit the refusal blind.
10. The definition of "parked" is unchanged: `in_progress` plus a `defer_date` parsing to a future instant. The guard adds a second condition to the exit decision; it does not narrow the definition.
11. A headless SDK session's behaviour is unchanged. For a **task** holder the existing headless refusal sits *inside* `terminal_verdict` at `:449-457`, so the new guard must be placed **after** that branch — putting it before would change headless wording and break the test at `:321`. For a **goal/topic** holder the refusal happens earlier in `main()` at `:703-708`.

## Constraints

- **The watch store location is fixed by its producer**, `claude-supervisor`'s `park-event.py`: `$PARK_EVENT_STATE_DIR`, defaulting to `~/.claude/state/park-events/`, holding one JSON file per watch plus `*.lock` files that must be ignored. Each record carries a `sessions` list. This schema is `claude-supervisor`'s contract, not vault-cli's — document it in `docs/park-watches.md` (a new page; `docs/session-liveness.md` is scoped to liveness classification and carries no park content) and reference it from the code.
- **Resolve the producer root explicitly** as `os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")`, in the guard and in the harness.
- **The harness must materialise a stub producer** at `$TMP/plugroot/plugins/cache/claude-supervisor/supervisor/0.0.0/scripts/park-event.py` and pass `CLAUDE_CONFIG_DIR="$TMP/plugroot"` to every fixture that requires the producer present — the YOLO container has none, so without this the refusal fixtures fail open there. AC7 uses a second root that omits the stub.
- **Read the store as files; do not shell out to `park-event.py` and do not import it.** vault-cli and claude-supervisor are separately installed plugins, and vault-cli must keep working when the other is absent.
- **The guard is read-only.** It must never write to, create, or delete anything under the store.
- **`EXIT_REFUSED = 10` is frozen.** Do not renumber it; 53 assertions in `scripts/session-self-exit-test.sh` depend on it.
- **Do not change the park-due step.** It is correct to read registered watches only; that behaviour is out of scope.
- **Do not change what counts as a park.** A past, date-only, offset-less or unparseable `defer_date` keeps its current refusal wording, and the existing `no session id` refusal at `main()` (`:650`, exit 10) is unchanged — do not relax it into a fail-open.
- Add a `## Unreleased` CHANGELOG bullet. `autoRelease: true` is `.maintainer.yaml: release.autoRelease` (`.dark-factory.yaml` carries `autoRelease: false` for the container run) — do not hand-bump version manifests or tag; the releaser owns them.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| No `park-event.py` under the resolved plugin-cache root | Exit 0, one warning naming the absent producer | `ls "$(python3 -c 'import os;print(os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude"))')/plugins/cache/claude-supervisor"` confirms absence |
| **Store directory absent but producer present** (fresh install, no parks yet) | Treated as **no watch** → refuse. Must never be conflated with producer-absent → fail open, or any host whose store dir was never created would bypass the guard entirely | `ls -d "$PARK_EVENT_STATE_DIR"`; a missing dir is created by the first `register` |
| Store present and readable but producer absent | Exit 0, one warning (readability is not the test) | None; the stale directory is inert |
| A record is valid JSON but has no `sessions` key (schema drift) | Treated as **not a watch** for any session, and one warning names the file — never silently absorbed, since a schema change would otherwise refuse every park for ever with no signal | `grep -n '<file>' <stderr capture>` returns the warning line; the schema assumption is documented in `docs/park-watches.md` |
| A single record is malformed JSON | That record is skipped, the remaining records are still consulted, one warning names the bad file | `grep -c '<bad-file>' <stderr capture>` returns 1; operator deletes or repairs the named file, then `park-event.py list` no longer names it |
| A `*.lock` file is present | Ignored entirely, no warning (locks are normal during concurrent `register`) | None — `park-event.py list` is unaffected |
| Concurrent `register` writes while the guard reads | Read-only and tolerant: a partially written record is skipped, never raises | None — the next exit re-reads; `park-event.py due` still returns the intact records |
| Session id absent from the environment | Unchanged: `main()` refuses at `:650` with `no session id`, exit 10 | Caller passes `--session` |
| Clock skew between the `defer_date` offset and the guard's `now` | The existing aware-instant parser decides; a park that reads as elapsed keeps its current refusal | None new — `session-self-exit.py --dry-run` prints the elapsed refusal |

## Security / Abuse Cases

`PARK_EVENT_STATE_DIR` is caller-controlled. **Pointing it at an empty directory does not bypass the guard — it makes the guard refuse**, because an empty store yields no watch naming the session. The real bypass is the opposite: a session can *write* a record naming its own id and then pass. That is accepted rather than closed — anyone who can write the store already controls the machine, and the guard's job is to close the *accidental* silent park, which is the observed failure. Documented here so a future reader does not mistake the guard for an enforcement boundary against a hostile session. The reproduction relies on the empty-directory override, which is why it must stay supported.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Store reader + producer-presence check + fail-open, as its own function with unit-level shell coverage | 6, 8 | 5, 7 | — |
| 2 | Guard both accept points and the live path in `scripts/session-self-exit.py` | 1-5, 7, 10-11 | 1-4, 6 | prompt 1 |
| 3 | Harness: stub producer under a temp `CLAUDE_CONFIG_DIR`, fixtures for ACs 1-7 and 9, four parked fixtures gain a watch | — | 5, 9 | prompt 2 |
| 4 | Doctrine + docs: `commands/session-close.md` Phase 4.5 registration precondition, new `docs/park-watches.md`, `## Unreleased` bullet | 9 | 8 | prompt 2 |

Rationale: prompt 1 establishes the store contract and its failure semantics in isolation; prompt 2 consumes it at both accept points; prompt 3 locks the behaviour down in the harness; prompt 4 is doc-only and needs prompt 2's final refusal wording.

*Scope note: DBs × ACs = 11 × 9 = 99, over the 50 split-threshold, and the work spans four layers. Kept as one spec because the four prompts are strictly ordered and share one behavioural rule; the decomposition above is what bounds the surface, not a second spec.*
