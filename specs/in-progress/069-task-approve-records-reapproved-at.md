---
status: verifying
approved: "2026-11-17T23:45:39Z"
verifying: "2026-11-18T19:33:38Z"
branch: dark-factory/task-approve-records-reapproved-at
---

## Summary

- A re-approval currently moves a task's phase but leaves no trace of *when* it happened.
- `approved_at` is deliberately preserved, so a re-approval is invisible to anything reading the row.
- The decision-park resume leg keys on "the approval landed after the worker last spoke" and therefore never fires for a re-approved row.
- This spec adds one key — `reapproved_at` — written only on the re-approval branch, at the moment of the event.
- The first approval is unchanged: it remains observable through `approved_at`.

## Problem

`vault-cli task approve` refuses any row that is not at `phase: todo`. A row that already carries an approval record is advanced to `planning` with `approved_by` and `approved_at` preserved exactly as they are — a deliberate, documented contract, because the first approval already happened and re-stamping it would make a first approval indistinguishable from a re-approval.

That leaves a re-approval with exactly one observable trace: the phase string. It carries no time. A consumer that must react to the *act* of re-approval — "the operator cleared this wait just now" — cannot tell a re-approval that happened a minute ago from one that happened three weeks ago, and cannot tell either of them from a row that was never re-approved at all.

The concrete consumer is `claude-supervisor`'s decision-park resume leg (`scripts/decision-park-resume.py`, `approval_cleared()`: "the approval must be later than the session's last turn"). That claim is about an external repository and is **not verifiable from this repo**; it is cited so the reader can check it, not asserted as this spec's evidence. A re-approved row's `approved_at` predates that turn by construction, so the leg never fires — the operator's most intuitive recovery action produces silence, and the failure reads as a broken leg rather than an approval that was never recorded.

## Goal

A re-approval is an event with a timestamp. Any consumer reading a task row can answer "was this row re-approved, and when?" from the frontmatter alone, without git history, without cross-round memory, and without inferring it from the phase string.

## Non-goals

- Changing the preservation of `approved_by` / `approved_at` on re-approval. That contract is correct and is what makes the ambiguity resolvable: refreshing `approved_at` would make a first approval indistinguishable from a re-approval, which is why this spec adds a second key instead.
- Making `task approve` runnable from any phase. The `phase: todo` entry contract is unchanged.
- Giving re-approval its own verb. A marker field on the existing verb is the chosen mechanism; a separate verb would be a second way to do one thing.
- Consuming the new key anywhere in this repository.
- **The consuming change in `claude-supervisor`'s decision-park leg — tracked separately.** Until it lands this spec only makes the event observable; the leg stays silent.

## Acceptance Criteria

- [ ] A re-approval writes `reapproved_at` as a timestamp value, not a formatted string. Evidence: after `vault-cli task approve "<task>"` on a row that already carries `approved_at` and sits at `phase: todo`, `grep -cE '^reapproved_at: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}' "<task file>"` returns `1`, **and** the unit test asserts `written.Get("reapproved_at").(time.Time)` succeeds — mirroring the existing `approved_at` assertion at `pkg/ops/task_approve_test.go:88`. ⚠️ The grep alone cannot separate a `time.Time` from an RFC3339 string; the type assertion is what pins DB2's type parity.
- [ ] The key is readable through the existing generic field reader. Evidence: `vault-cli task get "<task>" reapproved_at` prints a non-empty value. ⚠️ **Shape only, never "RFC3339"** — the generic reader returns Go's `%v` form for a timestamp-typed value (`2026-11-18 09:00:00 +0100 CET`), so an RFC3339 assertion against it cannot pass without adding a read path this spec forbids.
- [ ] The first approval writes **no** `reapproved_at`, and the branch is presence-based rather than value-based. Evidence: on a freshly created row at `phase: todo` with no approval record, `vault-cli task approve "<task>"` then `grep -c '^reapproved_at:' "<task file>"` returns `0`, while `grep -c '^approved_at:'` returns `1`; **and** a second fixture carrying only `approved_by` (no `approved_at`) still receives `reapproved_at`, asserted in the unit test. A value-based predicate (`task.Get("approved_at") != nil`) would pass the first clause and fail the second.
- [ ] `approved_at` is still preserved on re-approval. Evidence: `grep -m1 '^approved_at:' "<task file>"` returns the identical string before and after the re-approval.
- [ ] A second re-approval **refreshes** `reapproved_at` rather than preserving it. Evidence: two re-approvals separated by at least one second produce two different `grep -m1 '^reapproved_at:'` values, and the second sorts later than the first. ⚠️ **Precondition:** `task set <name> phase todo` is refused only when the row is `in_progress` **and** already past planning (`execution` / `ai_review` / `human_review`, `pkg/ops/frontmatter.go:380-395`). A `planning` or `next` row accepts the staging step, so the probe row must not be in a post-planning phase.
- [ ] A terminal row is still refused and nothing is written. Evidence: on a row carrying an approval record at `phase: todo` with `status: completed`, `vault-cli task approve "<task>"` exits non-zero and `shasum -a 256 "<task file>"` returns the same digest before and after; the same behaviour is pinned by `pkg/ops/task_approve_test.go:318` ("refuses a terminal row that is still at phase todo, with zero writes"). ⚠️ **Not `git diff`** — the disposable probe vault is not a git repository, so `git diff` exits 129 there.
- [ ] `vault-cli task approve --help` names the new key **and** says it is written only on a re-approval. Evidence: `vault-cli task approve --help | grep -qE 'reapproved_at.*re-approval|re-approval.*reapproved_at'` exits `0`. ⚠️ A bare `grep -c reapproved_at` is not enough — the existing help already says "never refreshes `approved_at`", so an author could add "never writes `reapproved_at`" and satisfy a count while contradicting DB6.
- [ ] `make precommit` exits 0. ⚠️ `precommit` includes `test`; it also runs `check`, which shells out to `trivy` and an `addlicense` network fetch, so it can go red for environment reasons unrelated to this change. If it does, run `make test lint` and say which step failed and why.

## Verification

**Both rungs are run by the verifier.** The probe below needs only `go build` and a temp directory, so it is container-executable too — it is separated only because it reads more clearly on its own.

### Rung 1 — container-executable

```bash
make precommit
make test      # faster inner loop while iterating; precommit already includes it
```

### Rung 2 — the behavioural probe, asserting rather than printing

Build the binary first — never the installed `vault-cli`, which is a different version:

```bash
go build -o /tmp/vc-reapproved-bin . && echo built
```

```bash
rm -rf /tmp/vc-reapproved-probe && mkdir -p "/tmp/vc-reapproved-probe/25 Tasks"
cat > /tmp/vc-reapproved-probe/config.yaml <<'EOF'
current_user: bborbe
default_vault: probe
vaults:
    probe:
        path: /tmp/vc-reapproved-probe
        name: probe
        tasks_dir: 25 Tasks
        goals_dir: 24 Goals
        topics_dir: 23 Topics
EOF
cat > "/tmp/vc-reapproved-probe/25 Tasks/Probe Re-Approval.md" <<'EOF'
---
page_type: task
status: next
phase: todo
assignee: bborbe
---
EOF
F="/tmp/vc-reapproved-probe/25 Tasks/Probe Re-Approval.md"
VC="/tmp/vc-reapproved-bin --config /tmp/vc-reapproved-probe/config.yaml"
fail() { echo "FAIL $1"; exit 1; }

# AC3 — the first approval writes no reapproved_at
$VC task approve "Probe Re-Approval" >/dev/null
[ "$(grep -c '^reapproved_at:' "$F")" = 0 ] || fail "AC3: first approval wrote reapproved_at"
[ "$(grep -c '^approved_at:' "$F")" = 1 ]   || fail "AC3: approved_at missing"
BEFORE_AT=$(grep -m1 '^approved_at:' "$F")

# AC1 + AC2 + AC4 — revert, re-approve, the key appears and approved_at did not move
$VC task set "Probe Re-Approval" phase todo >/dev/null
sleep 1
$VC task approve "Probe Re-Approval" >/dev/null
[ "$(grep -cE '^reapproved_at: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}' "$F")" = 1 ] \
  || fail "AC1: reapproved_at missing or not a timestamp"
[ -n "$($VC task get 'Probe Re-Approval' reapproved_at)" ] || fail "AC2: reader returned empty"
[ "$(grep -m1 '^approved_at:' "$F")" = "$BEFORE_AT" ] || fail "AC4: approved_at moved"

# AC5 — a second re-approval refreshes, and the new instant is later
FIRST=$(grep -m1 '^reapproved_at:' "$F")
$VC task set "Probe Re-Approval" phase todo >/dev/null
sleep 1
$VC task approve "Probe Re-Approval" >/dev/null
SECOND=$(grep -m1 '^reapproved_at:' "$F")
[ "$FIRST" != "$SECOND" ] || fail "AC5: reapproved_at was preserved, not refreshed"
[ "$SECOND" \> "$FIRST" ] || fail "AC5: the refreshed instant is not later"

# AC6 — a terminal row is refused and nothing is written
cat > "/tmp/vc-reapproved-probe/25 Tasks/Probe Terminal.md" <<'EOF'
---
page_type: task
status: completed
phase: todo
approved_by: operator
approved_at: 2026-01-01T00:00:00Z
assignee: bborbe
---
EOF
T="/tmp/vc-reapproved-probe/25 Tasks/Probe Terminal.md"
BEFORE_SUM=$(shasum -a 256 "$T" | awk '{print $1}')
$VC task approve "Probe Terminal" >/dev/null 2>&1 && fail "AC6: a terminal row was approved"
[ "$(shasum -a 256 "$T" | awk '{print $1}')" = "$BEFORE_SUM" ] || fail "AC6: the file changed"

# AC7 — the help names the key and its condition
$VC task approve --help | grep -qE 'reapproved_at.*re-approval|re-approval.*reapproved_at' \
  || fail "AC7: help does not name reapproved_at as a re-approval key"

echo "probe: all assertions passed"
```

## Desired Behavior

1. On a row at `phase: todo` with **no** approval record, `task approve` behaves exactly as today: it sets `phase: planning`, writes `approved_by` and `approved_at`, and writes no `reapproved_at`.
2. On a row at `phase: todo` that **already carries** an approval record, `task approve` sets `phase: planning`, preserves `approved_by` and `approved_at` byte-for-byte, and writes `reapproved_at` with the current instant as a `time.Time`.
3. Every subsequent re-approval of the same row **refreshes** `reapproved_at` to the current instant. It is an event time, not a record of the first re-approval.
4. A row carrying an approval record whose status is `completed` or `aborted` is still refused, and the refusal writes nothing — including no `reapproved_at`.
5. The zero-clock guard still runs before any mutation: a zero instant refuses the approval rather than writing a placeholder into either key.
6. `vault-cli task approve --help` names `reapproved_at` and says it is written only on a re-approval, so a reader can discover the key without reading this spec.

## Constraints

- `approved_by` and `approved_at` preservation on re-approval is a frozen contract. This spec adds a key; it removes and reinterprets nothing.
- **`docs/task-writing.md` is not edited.** Its approval-record paragraph states that the re-approval branch "never overwrites either key, never fabricates a missing one" — a claim that stays true: `approved_by` and `approved_at` are neither overwritten nor fabricated. A third, separate key is added beside them. Editing that paragraph is forbidden by the consuming task's scope, and the paragraph's own assertion survives the change. The key's durable discovery surface is therefore the command help (DB6).
- The key is named exactly `reapproved_at` and holds an RFC3339 instant, matching `approved_at`'s type and formatting.
- `hasApprovalRecord` stays presence-based (`task.Keys()`), not value-based. A row whose `approved_at:` is present but blank — or which carries only `approved_by` — still takes the re-approval branch, so it still receives `reapproved_at`.
- The change stays inside the existing re-approval branch and its single write path. No new operation, no new interface method.
- `task approve` performs exactly **one** storage write per invocation, as it does today; the new key travels in that same write.
- `task get "<task>" reapproved_at` reads the value back through the existing generic field reader — no new subcommand, no new output path.
- Every existing test in `pkg/ops/task_approve_test.go` continues to pass unchanged.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Row carries an approval record but `approved_at` is unparseable | The branch is presence-based, so `reapproved_at` is still written; `approved_at` is left untouched | None needed — the new key is the signal, not the old one |
| Clock returns a zero instant | Refused before any mutation; neither key is written | Fix the clock source, then `vault-cli task approve "<task>"` again |
| Row is `completed` / `aborted` at `phase: todo` | Refused; nothing written, no `reapproved_at` | Run `vault-cli task get "<task>" phase` and decide deliberately whether the row is mis-phased; `vault-cli task set "<task>" phase <phase>` if it is |
| The storage write fails | Nothing reaches disk — no phase change and no `reapproved_at` | Re-run `vault-cli task approve "<task>"`; the guard order means a retry is safe |
| Two concurrent re-approvals | Last write wins; the earlier instant is lost, and both rows end at `phase: planning` | Not a supported workflow — an operator acts on one row at a time; the surviving instant is still later than any prior turn, so the consumer is unaffected |
| Two re-approvals in the same second | Both write the same instant; a consumer ordering by this key alone cannot separate them | Consumers that need strict ordering use the phase transition count or a distinct verb; out of scope here |
| A consumer treats `reapproved_at` as "the only approval" | It misses first approvals, which have no such key | Consumers read `approved_at` for the first approval and `reapproved_at` for re-approvals — documented in the consuming repo |

## Security / Abuse Cases

The change writes one additional key into a file the command already writes. It introduces no new input, no new path, and no new subprocess. The approver identity and the zero-clock guard are unchanged.

## Suggested Decomposition

Single layer, single prompt.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Write `reapproved_at` on the re-approval branch, pin it with tests, and name it in the command help | 1-6 | 1-8 | — |

## Do-Nothing Option

Without this, a re-approval stays unobservable and no consumer can distinguish "re-approved a minute ago" from "never re-approved". The consuming leg stays silent, so an operator who re-approves to clear a wait gets no reaction and cannot tell that from a broken leg; the only remaining recovery is a manual session resume. This spec makes the event observable — it does not by itself resume anything.
