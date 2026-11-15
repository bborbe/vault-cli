---
status: draft
kind: bug
---

## Summary

- A session can exit "parked" carrying only a `defer_date`, with no park watch registered to wake it.
- The park-due step reads registered watches only, so such a session is invisible to it rather than refused by it.
- The exit path accepts a park at two independent points, and neither consults the watch store.
- The result is a session that sits idle indefinitely past its own `defer_date`, with nothing reporting it.

## Problem

Parking is the contract that lets a session free its slot and be woken later: `session-close` Phase 4.5 and `scripts/session-self-exit.py` both define "parked" as `in_progress` carrying a `defer_date` that parses to a future instant, and treat that as sufficient to exit clean. But the wake half of that contract is owned by a different component — the park-due step, which reads a store of registered watches and never looks at `defer_date` at all. So the exit path can certify a park that nothing will ever wake, and both halves are individually correct. The failure is silent: no error is raised, no line is logged, and the only symptom is a session sitting idle long after the time it said it would come back.

**Cost of not fixing:** parks continue to exit silently, and the operator discovers an idle session hours late — on 2026-11-13, roughly 80 minutes late and only because a fleet round happened to look.

## Reproduction

> **Correction (2026-11-15).** An earlier revision of this spec carried `Observed: parked until <defer_date>, exit=0` for a bare two-line setup (a task file plus `PARK_EVENT_STATE_DIR`). **That observation was never produced — it is retracted.** `main()` refuses on pane ownership at `:662-666` long before any park accept point, so the bare setup returns `SELF-EXIT: stay — $WEZTERM_PANE is unset and no pane carries this session's controlling tty`, `exit=10`. Verified by running it. The reproduction below uses the repo's own harness, which supplies the preconditions `main()` requires.

**Reported incident — 2026-11-13, operator-observed, no transcript retained.** Worker `b652848f` held `defer_date: 2026-11-13T05:56:00+01:00` (04:56Z). A fleet round ran at 05:45Z and the worker was still idle at 06:15Z. `park-event.py list` and `park-event.py due` both returned nothing for it. The earlier watch had been consumed by a 00:57Z hand-resume, and the replacement `defer_date` was written without `park-event.py register`.

**Deterministic reproduction — a fixture in `scripts/session-self-exit-test.sh`.** The harness's `run` / `run_msg` / `run_live` helpers (`:190-235`) supply everything `main()` needs before it reaches a park accept point: `SESSION_ID`, `SESSIONS_DIR`, `TASKS_ROOTS`, `GOALS_ROOTS`, `TOPICS_ROOTS`, `WEZTERM_PANE=777`, `WEZTERM_PANES_FILE`, `PID_TTY_FILE`, and a stub attention store on `ATTENTION_STORE_URL`. A bare invocation without those refuses on pane ownership, as recorded above.

Add one fixture and run it:

```bash
# a task held by this session, in_progress, defer_date one hour ahead
task in_progress planning 0 "<future-instant>"      # existing helper shape
export PARK_EVENT_STATE_DIR="$(mktemp -d)"          # empty store: no watch can exist
run_msg 0 "unwatched park is accepted today (the bug)" "parked until"
```

**Current behaviour, derived from source** (not observed — stated as such deliberately):

1. `main()` acquires a pane, then calls `anchor_task(sid)` (`:676`).
2. For a task holder it calls `terminal_verdict(...)` (`:680-682`).
3. `terminal_verdict` returns `(True, f"parked until {defer}")` at `:466` — no watch lookup anywhere in the function.
4. `main()` prints `SELF-EXIT: exit — …` and returns `EXIT_TAKEN` (0) at `:732-734`.
5. The only component that could wake the session, `park-event.py cmd_due`, iterates `all_watches()` and cannot see a bare `defer_date`.

Resolve `park-event.py` to exactly one path — the glob over versions expands to **188** files on the authoring host and python3 then treats 187 of them as argv, failing with `invalid choice`:

```bash
PARK_EVENT=$(ls -d "${CLAUDE_CONFIG_DIR:-$HOME/.claude}"/plugins/cache/claude-supervisor/supervisor/*/ | sort -V | tail -1)scripts/park-event.py
python3 "$PARK_EVENT" list      # → no park watches
python3 "$PARK_EVENT" due       # → no park watches due at <now>
```

## Expected vs Actual

**Expected** — per `commands/session-close.md:783` (*"parked — `status: in_progress` carrying a `defer_date` that parses to a future instant"*) and `:698` (*"A parked task is the exception … reaches the clean verdicts"*), parking is granted *because* a re-check time is recorded, so the session resumes without the operator. A park that nothing will wake does not satisfy that justification, so the exit path must not certify it.

**Actual** — `scripts/session-self-exit.py` accepts the park on the `defer_date` alone. Neither accept point reads the watch store, and the string `park-event` does not occur in the file.

## Why this is a bug

The two halves of the park contract are owned by separate components, and only one of them can observe whether the other half exists. The park-due step is correct to read registered watches only — it has no business scanning task files for stray `defer_date`s. That leaves the exit path as the only place the asymmetry is visible, and it does not look. The documented behaviour (a park frees the slot *and* the session resumes) is therefore not what the system does.

## Assumptions

- Each park record carries a `sessions` list of session ids. Verified against `claude-supervisor` 0.315.0, where the key is written at `scripts/park-event.py:508` and read at `:533`, `:562`, `:598`, `:626`, `:643`, `:653` — cite the symbol, not a line number, since this file moves between versions.
- **"The supervisor plugin is absent" is observable as "no `park-event.py` under the plugin cache" — NOT as "the store directory is unreadable".** This assumption is load-bearing and the obvious inverse is false: `~/.claude/state/park-events/` survives an uninstall, and on the authoring host it holds 31 records and 398 lock files with the plugin present, so it would remain readable and non-empty after removal. Keying fail-open on store readability would therefore refuse every park on a de-plugin'd host.
- **The producer resolution root must be explicit**, because the test harness runs `env -i` with no `HOME`: the guard and the tests must both resolve the cache as `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/plugins/cache/claude-supervisor`. Left implicit, `Path.home()` returns `/Users/bborbe` even under `env -i`, so the guard would see a present producer in every test and the producer-absent case would be unreachable.
- The refusal exit code is a frozen external contract at `EXIT_REFUSED = 10` (`scripts/session-self-exit.py:94`), asserted by **53** call sites in `scripts/session-self-exit-test.sh` (27 `run 10` + 25 `run_msg 10` + 1 `run_live 10`). It must not be renumbered.

## Goal

No session exits "parked" unless a registered watch names it — on any host where the park mechanism exists. When the watch is missing, the exit is refused with a message naming the session and the missing watch, so the session either registers a watch and exits, or stays alive and reports — never exits into silence. (The one host class outside this Goal is the one where the producer is not installed, handled by DB 8.)

## Acceptance Criteria

- [ ] **Task holder refused.** A session whose task is `in_progress` with a future `defer_date` and no watch naming it is refused. Evidence: a `run_msg 10` fixture prints `SELF-EXIT: stay — ` plus a reason naming `<S>` and the missing park watch.
- [ ] **Goal/topic holder refused.** The same refusal reproduces for a page whose `claude_session_id` is `<S>` with `status: in_progress` and a future `defer_date`. Evidence: a `run_msg 10` fixture on the goal/topic roots.
- [ ] **Positive control — a watched park still exits.** Evidence: with `park-event.py register --event <key> --recheck <ISO> --session <S>` run against `PARK_EVENT_STATE_DIR`, a `run_msg 0` fixture prints `parked until <defer_date>`, and `park-event.py due` returns the record once `<recheck>` passes.
- [ ] **Negative control — an unrelated watch does not count.** Evidence: with the store holding a valid watch for a different session `<T>` only, the `<S>` fixture still exits 10.
- [ ] **Malformed records and lock files are tolerated.** Evidence: with one malformed `.json`, three `*.lock` files, and a valid watch for `<S>`, the fixture exits 0 with `parked until …`, and stderr carries exactly one warning naming the malformed file.
- [ ] **The real exit path shares the guard.** Evidence: a `run_live 10` fixture (the harness's `run_live` at `:221-235`, no `--dry-run`) for an unwatched park prints `SELF-EXIT: stay — …` — a guard placed only in the `--dry-run` branch fails this.
- [ ] **Fail-open is keyed on producer presence.** Evidence: with the store directory present and readable but `CLAUDE_CONFIG_DIR` pointing at a temp root whose plugin cache contains no `park-event.py`, the fixture exits 0 plus exactly one warning naming the absent producer — never a refusal.
- [ ] **Doctrine names registration.** Evidence: `grep -n 'park-event.py register' commands/session-close.md` returns ≥ 1 line inside Phase 4.5.
- [ ] **No regression, no deleted assertions.** Evidence: `make precommit` exits 0; `git diff scripts/session-self-exit-test.sh` contains **no deleted line** (`git diff --numstat` shows 0 deletions), and the four existing positive parked fixtures (`:314`, `:338`, `:568` ×2) each gain a registered watch rather than being relaxed to pass via fail-open.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0. Target is `ensure format generate test check addlicense`; there is no shellcheck in it, and `check-versions` runs only under `release-check`.
- `make test` — exits 0
- `bash scripts/session-self-exit-test.sh` — exits 0
- `grep -n 'PARK_EVENT_STATE_DIR\|park-event' scripts/session-self-exit.py` — returns ≥ 1 line per guarded accept point
- `git diff --numstat scripts/session-self-exit-test.sh` — 0 deletions

### Operator-executable (runs on the host after PR merge and plugin reload)

- `claude plugin update vault-cli@vault-cli`, then the operator runs `/reload-plugins`
- Re-run the reproduction fixture against the **loaded** plugin copy under `~/.claude/plugins/cache/vault-cli/vault-cli/<version>/scripts/`. The discriminating signal is the *message*, not the exit code: the unwatched fixture must print `SELF-EXIT: stay — … no registered park watch`, where it previously printed `SELF-EXIT: exit — … parked until …`. An exit-code-only comparison is a false pass, because the pane-ownership refusal also exits 10.

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

- **The watch store location is fixed by its producer**, `claude-supervisor`'s `park-event.py`: `$PARK_EVENT_STATE_DIR`, defaulting to `~/.claude/state/park-events/`, holding one JSON file per watch plus `*.lock` files that must be ignored. Each record carries a `sessions` list of session ids. This schema is `claude-supervisor`'s contract, not vault-cli's — document it in `docs/park-watches.md` (a new page; `docs/session-liveness.md` is scoped to liveness classification and currently carries no park content) and reference it from the code.
- **Resolve the producer root explicitly** as `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/plugins/cache/claude-supervisor`, in the guard and in the harness, so the producer-absent case is reachable under `env -i`.
- **Read the store as files; do not shell out to `park-event.py` and do not import it.** vault-cli and claude-supervisor are separately installed plugins, and vault-cli must keep working when the other is absent.
- **The guard is read-only.** It must never write to, create, or delete anything under the store.
- **`EXIT_REFUSED = 10` is frozen.** Do not renumber it; 53 assertions in `scripts/session-self-exit-test.sh` depend on it.
- **Do not change the park-due step.** It is correct to read registered watches only; that behaviour is out of scope.
- **Do not change what counts as a park.** A past, date-only, offset-less or unparseable `defer_date` keeps its current refusal wording, and the existing `no session id` refusal at `main()` (`:650`, exit 10) is unchanged — do not relax it into a fail-open.
- Add a `## Unreleased` CHANGELOG bullet. `autoRelease: true` is `.maintainer.yaml: release.autoRelease` (`.dark-factory.yaml` carries `autoRelease: false` for the container run) — do not hand-bump version manifests or tag; the releaser owns them.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| No `park-event.py` under the resolved plugin-cache root | Exit 0, one warning line naming the absent producer | None needed — the park mechanism does not exist on that machine |
| Store directory present and readable but producer absent | Exit 0, one warning (same as above — readability is not the test) | None; the stale directory is inert |
| A record is valid JSON but has no `sessions` key (schema drift) | Treated as **not a watch** for any session, and one warning names the file — never silently absorbed, since a schema change would otherwise refuse every park for ever with no signal | Operator checks the warning; the guard's schema assumption is documented in `docs/park-watches.md` |
| A single record is malformed JSON | That record is skipped, the remaining records are still consulted, one warning names the bad file | `grep -n '<bad-file>' <stderr capture>` locates it; operator deletes or repairs the named file |
| A `*.lock` file is present | Ignored entirely, no warning (locks are normal during concurrent `register`) | None |
| Concurrent `register` writes while the guard reads | Read-only and tolerant: a partially written record is skipped, never raises | None — the next exit re-reads |
| Session id absent from the environment | Unchanged: `main()` refuses at `:650` with `no session id`, exit 10 | Caller passes `--session` |
| Clock skew between the `defer_date` offset and the guard's `now` | The existing aware-instant parser decides; a park that reads as elapsed keeps its current refusal | None new — behaviour unchanged |

## Security / Abuse Cases

`PARK_EVENT_STATE_DIR` is caller-controlled. **Pointing it at an empty directory does not bypass the guard — it makes the guard refuse**, because an empty store yields no watch naming the session. The real bypass is the opposite: a session can *write* a record naming its own id and then pass. That is accepted rather than closed — anyone who can write the store already controls the machine, and the guard's job is to close the *accidental* silent park, which is the observed failure. Documented here so a future reader does not mistake the guard for an enforcement boundary against a hostile session. The reproduction relies on the empty-directory override, which is why it must stay supported.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Guard the decision path (both accept points + real exit) in `scripts/session-self-exit.py`, harness fixtures for ACs 1-7 and 9 | 1-8, 10-11 | 1-7, 9 | — |
| 2 | Doctrine + docs: `commands/session-close.md` Phase 4.5 registration precondition, new `docs/park-watches.md` store schema, `## Unreleased` bullet | 9 | 8 | prompt 1 |

Rationale: prompt 1 carries the behavioural change and its regression lock. Prompt 2 is doc-only and depends on prompt 1's final refusal wording.

*Scope note: DBs × ACs = 11 × 9 = 99, over the 50 split-threshold. Not split deliberately — this is one behavioural rule tested from nine angles plus its doctrine half, not several features, and splitting it would produce two specs that cannot be verified independently.*
