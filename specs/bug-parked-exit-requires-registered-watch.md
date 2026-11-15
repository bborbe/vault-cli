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

**Reported incident — 2026-11-13, operator-observed, no transcript retained.** Worker `b652848f` held `defer_date: 2026-11-13T05:56:00+01:00` (04:56Z). A fleet round ran at 05:45Z and the worker was still idle at 06:15Z. `park-event.py list` and `park-event.py due` both returned nothing for it. The earlier watch had been consumed by a 00:57Z hand-resume, and the replacement `defer_date` was written without `park-event.py register`.

**Synthetic reproduction — deterministic, no fleet required.** Run against `vault-cli@b76d76b` (`scripts/session-self-exit.py` as shipped in plugin `0.194.0`):

1. Create a scratch vault whose task file carries `status: in_progress`, `claude_session_id: <S>`, and `defer_date` set to one hour in the future.
2. Point the watch store at an empty directory so no watch can exist:
   ```bash
   export PARK_EVENT_STATE_DIR=$(mktemp -d)
   ```
3. Confirm the store is empty:
   ```bash
   python3 ~/.claude/plugins/cache/claude-supervisor/supervisor/*/scripts/park-event.py list
   ```
   Observed: `no park watches`
4. Ask the exit path for its decision:
   ```bash
   python3 scripts/session-self-exit.py --session <S> --dry-run; echo "exit=$?"
   ```
   Observed: stdout `parked until <defer_date>`, `exit=0`
5. Confirm the wake half cannot see the session:
   ```bash
   python3 ~/.claude/plugins/cache/claude-supervisor/supervisor/*/scripts/park-event.py due
   ```
   Observed: `no park watches due at <now>`

Steps 4 and 5 together are the defect: the exit path says the session is parked and safe to end, and the only component that could wake it cannot see it.

## Expected vs Actual

**Expected** — per `commands/session-close.md:783` (*"parked — `status: in_progress` carrying a `defer_date` that parses to a future instant"*) and `:698` (*"A parked task is the exception … reaches the clean verdicts"*), parking is granted *because* a re-check time is recorded, so the session resumes without the operator. A park that nothing will wake does not satisfy that justification, so the exit path must not certify it.

**Actual** — `scripts/session-self-exit.py` accepts the park on the `defer_date` alone. Neither accept point reads the watch store, and the string `park-event` does not occur in the file.

## Why this is a bug

The two halves of the park contract are owned by separate components, and only one of them can observe whether the other half exists. The park-due step is correct to read registered watches only — it has no business scanning task files for stray `defer_date`s. That leaves the exit path as the only place the asymmetry is visible, and it does not look. The documented behaviour (a park frees the slot *and* the session resumes) is therefore not what the system does.

## Assumptions

- Each park record carries a `sessions` list of session ids (`park-event.py:328`, verified). The guard joins on that field.
- **"The supervisor plugin is absent" is observable as "no `park-event.py` under the plugin cache" — NOT as "the store directory is unreadable".** This assumption is load-bearing and the obvious inverse is false: `~/.claude/state/park-events/` survives an uninstall, and on the authoring host it holds 31 records and 398 lock files with the plugin present, so it would remain readable and non-empty after removal. Keying fail-open on store readability would therefore refuse every park on a de-plugin'd host.
- The refusal exit code is a frozen external contract at `EXIT_REFUSED = 10` (`scripts/session-self-exit.py:94`), asserted by 52 call sites in `scripts/session-self-exit-test.sh`. It must not be renumbered.

## Goal

No session exits "parked" unless a registered watch names it. When the watch is missing, the exit is refused with a message naming the session and the missing watch, so the session either registers a watch and exits, or stays alive and reports — never exits into silence.

## Acceptance Criteria

- [ ] **Task holder refused.** A session whose task is `in_progress` with a future `defer_date` and no watch naming it is refused. Evidence: `python3 scripts/session-self-exit.py --session <S> --dry-run; echo "exit=$?"` prints `SELF-EXIT: stay — ` followed by a reason naming `<S>` and the missing park watch, and prints `exit=10`.
- [ ] **Goal/topic holder refused.** The same refusal reproduces for a page whose `claude_session_id` is `<S>` with `status: in_progress` and a future `defer_date`. Evidence: `exit=10` on that fixture.
- [ ] **Positive control — a watched park still exits.** Evidence: after `park-event.py register --event <key> --recheck <ISO> --session <S>`, the same `--dry-run` prints `parked until <defer_date>` with `exit=0`, and `park-event.py due` returns the record once `<recheck>` passes.
- [ ] **Negative control — an unrelated watch does not count.** Evidence: with the store holding a valid watch for a different session `<T>` only, the `<S>` `--dry-run` still prints `exit=10`.
- [ ] **Malformed records and lock files are tolerated.** Evidence: with one malformed `.json`, three `*.lock` files, and a valid watch for `<S>` in the store, `--dry-run` prints `parked until …` and `exit=0`, and stderr carries exactly one warning naming the malformed file.
- [ ] **The real exit path shares the guard.** Evidence: using the existing `run_live` harness with a fake `wezterm` on PATH, the non-`--dry-run` run for an unwatched park prints `SELF-EXIT: stay — …` and exits 10 — a guard placed only in the `--dry-run` branch fails this.
- [ ] **Fail-open is keyed on producer presence.** Evidence: with the store directory present and readable but no `park-event.py` anywhere under the plugin cache, `--dry-run` prints `exit=0` plus exactly one warning line naming the absent producer — never a refusal.
- [ ] **Doctrine names registration.** Evidence: `grep -n 'park-event.py register' commands/session-close.md` returns ≥ 1 line inside Phase 4.5.
- [ ] **No regression.** Evidence: `make precommit` exits 0; `git diff --stat` on `scripts/session-self-exit-test.sh` shows additions only.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0 (shellcheck, lint, version alignment)
- `make test` — exits 0
- `bash scripts/session-self-exit-test.sh` — exits 0
- `grep -n 'PARK_EVENT_STATE_DIR\|park-event' scripts/session-self-exit.py` — returns ≥ 1 line per guarded accept point
- `python3 scripts/session-self-exit.py --session <fixture> --dry-run; echo "exit=$?"` against the repo's own fixtures — the controls above

### Operator-executable (runs on the host after PR merge and plugin reload)

- `claude plugin update vault-cli@vault-cli`, then the operator runs `/reload-plugins`
- Re-run the synthetic reproduction against the **loaded** plugin copy under `~/.claude/plugins/cache/vault-cli/vault-cli/<version>/scripts/` — step 4 must print `exit=10` where it printed `exit=0` before

## Desired Behavior

1. A pane-owning session whose task is `in_progress` with a future instant `defer_date`, and whose session id appears in no park watch, is refused exit with `EXIT_REFUSED` (10).
2. The same refusal applies when the session holds a goal or topic page rather than a task — both holder kinds are guarded.
3. The refusal message names the session id and states that no registered park watch exists for it, and is distinguishable from the existing "defer_date already elapsed" refusal.
4. A session whose id appears in at least one park watch exits exactly as it does today: `parked until <defer_date>`, status 0.
5. A watch belonging to a different session does not satisfy the guard.
6. A malformed record or a `*.lock` file neither breaks the guard nor counts as a watch; a malformed record produces exactly one warning naming the file.
7. The guard is on the decision path, not the `--dry-run` branch — the real exit is guarded too.
8. Fail-open is keyed on the producer's presence in the plugin cache, not on the store's readability: with no `park-event.py` installed the guard exits 0 with one warning, even when a stale store directory exists and is readable.
9. `commands/session-close.md` Phase 4.5 names `park-event.py register` as a precondition of parking, so a session following the doctrine does not hit the refusal blind.
10. The definition of "parked" is unchanged: `in_progress` plus a `defer_date` parsing to a future instant. The guard adds a second condition to the exit decision; it does not narrow the definition.
11. A headless SDK session's existing behaviour is unchanged — it is already refused before reaching a park accept point, and that refusal keeps its current wording.

## Constraints

- **The watch store location is fixed by its producer**, `claude-supervisor`'s `park-event.py`: `$PARK_EVENT_STATE_DIR`, defaulting to `~/.claude/state/park-events/`, holding one JSON file per watch plus `*.lock` files that must be ignored. Each record carries a `sessions` list of session ids. This schema is `claude-supervisor`'s contract, not vault-cli's — document it in `docs/session-liveness.md` so it outlives this spec, and reference it from the code.
- **Read the store as files; do not shell out to `park-event.py` and do not import it.** vault-cli and claude-supervisor are separately installed plugins, and vault-cli must keep working when the other is absent.
- **The guard is read-only.** It must never write to, create, or delete anything under the store.
- **`EXIT_REFUSED = 10` is frozen.** Do not renumber it; 52 assertions in `scripts/session-self-exit-test.sh` depend on it.
- **Do not change the park-due step.** It is correct to read registered watches only; that behaviour is out of scope.
- **Do not change what counts as a park.** A past, date-only, offset-less or unparseable `defer_date` keeps its current refusal wording, and the existing `no session id` refusal at `main()` (`:650`, exit 10) is unchanged — do not relax it into a fail-open.
- Add a `## Unreleased` CHANGELOG bullet; do not hand-bump version manifests or tag (`autoRelease: true`).

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| No `park-event.py` under the plugin cache (producer not installed) | Exit 0, one warning line naming the absent producer | None needed — the park mechanism does not exist on that machine |
| Store directory present and readable but producer absent | Exit 0, one warning (same as above — readability is not the test) | None; the stale directory is inert |
| A single record is malformed JSON | That record is skipped, the remaining records are still consulted, one warning names the bad file | Operator inspects the named file |
| A `*.lock` file is present | Ignored entirely, no warning (locks are normal during concurrent `register`) | None |
| Concurrent `register` writes while the guard reads | Read-only and tolerant: a partially written record is skipped, never raises | None — the next exit re-reads |
| Session id absent from the environment | Unchanged: `main()` refuses at `:650` with `no session id`, exit 10 | Caller passes `--session` |
| Clock skew between the `defer_date` offset and the guard's `now` | The existing aware-instant parser decides; a park that reads as elapsed keeps its current refusal | None new — behaviour unchanged |

## Security / Abuse Cases

`PARK_EVENT_STATE_DIR` is caller-controlled, so a session that wants to exit can point it at an empty directory and pass the guard. **This bypass is accepted, not closed:** the variable is an operator escape hatch for the same reason `--session` exists, and anyone who can set it already controls the machine. It is documented here so a future reader does not mistake the guard for an enforcement boundary against a hostile session — it closes the *accidental* silent park, which is the observed failure. The synthetic reproduction itself relies on this override, so removing it would make the bug unreproducible.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Guard the decision path (both accept points + real exit) in `scripts/session-self-exit.py`, with shell tests for ACs 1-7 | 1-8, 10-11 | 1-7, 9 | — |
| 2 | Doctrine + docs: `commands/session-close.md` Phase 4.5 registration precondition, `docs/session-liveness.md` store schema, `## Unreleased` bullet | 9 | 8 | prompt 1 |

Rationale: prompt 1 carries the behavioural change and its regression lock. Prompt 2 is doc-only and depends on prompt 1's final refusal wording.

*Scope note: DBs × ACs = 11 × 9 = 99, over the 50 split-threshold. Not split deliberately — this is one behavioural rule tested from nine angles plus its doctrine half, not several features, and splitting it would produce two specs that cannot be verified independently.*
