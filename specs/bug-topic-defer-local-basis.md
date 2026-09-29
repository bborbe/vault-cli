---
status: draft
kind: bug
tags:
    - dark-factory
    - spec
---

## topic defer: the spec and the implementation disagree on the date basis

## Summary

- The `topic defer` integration spec computes its expected `defer_date` in UTC; the CLI writes it from the local calendar date.
- The two disagree by exactly one day whenever the local date differs from the UTC date, and `make test` then goes red on a tree with no changes at all.
- Because `.dark-factory.yaml` sets `preflightCommand: "make precommit"` and preflight failure is terminal, that red suite makes dark-factory refuse every run for the duration of the window — no prompt can start.
- This spec makes the spec compute on the same local basis the CLI already uses, and reconciles the suite's `time.Local = time.UTC`, which today overrides `TZ` in the test process while the spawned CLI honours it.
- No user-visible behavior changes: the CLI's `defer_date` is already the local-basis date and stays that way.

## Problem

`integration/cli_test.go:4518` computes its expected relative date as `time.Now().UTC().AddDate(0, 0, 7)`, while the CLI resolves the same offset through `pkg/ops/defer_date_parser.go:36` — `libtime.ToDate(now.AddDate(0, 0, days))` — where `now` is a plain `time.Time` parameter, supplied by its caller at `pkg/ops/topic_defer.go:53` (`now := o.currentDateTime.Now().Time()`, from an injected `libtime.CurrentDateTime` whose `Now()` is `time.Now()`), and `libtime.ToDate` takes `value.Date()`, the calendar date in `value`'s own location. The implementation basis is therefore **local**; the test basis is **UTC**. They disagree by exactly one day whenever the local calendar date differs from the UTC calendar date.

Two things make this worse than an ordinary flaky test. First, the suite's own guard hides it: `integration/integration_suite_test.go:20` assigns `time.Local = time.UTC` in the **test process**, which makes the parent agree with the test's UTC expectation — but the spec spawns the real binary, and a child does not inherit an in-process `time.Local`, so the CLI still resolves the host zone. The parent-side assignment gives false confidence, and it also overrides `TZ` for the parent, which is why the obvious fix (pin `TZ`) moves only one side. Second, the failure is load-bearing for the whole pipeline: `preflightCommand: "make precommit"` runs on a clean tree before each prompt and a preflight failure is terminal, so while the window is open no dark-factory prompt can start on this repo at all. The cost is a repository that is unbuildable on a schedule.

## Reproduction

Smallest config: this repository at `origin/master` (`6be825f`, release v0.153.1), a Go toolchain, and a `TZ` set to a zone whose local date differs from the UTC date. `Pacific/Pago_Pago` (UTC-11) was used below and was confirmed divergent at the moment of the run; any zone currently across the date line from UTC works, and the suite fails identically in the natural `Europe/Berlin` window of 00:00-02:00 CEST. No dark-factory run is needed to reproduce — the failure is in `go test`.

Against `dark-factory v0.196.0`:

```
$ TZ=Pacific/Pago_Pago date +%F ; TZ=UTC date +%F
2026-09-28
2026-09-29

$ TZ=Pacific/Pago_Pago go test ./integration/ -count=1 -run TestIntegration \
    -ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"
Running Suite: Integration Test Suite - .../integration
Random Seed: 1790662157
Will run 1 of 232 specs

• [FAILED] [0.500 seconds]
vault-cli integration tests vault-cli topic command family [It] topic defer writes defer_date for a relative and an absolute date
/Users/bborbe/Documents/workspaces/vault-cli-topic-defer/integration/cli_test.go:4495

  Timeline >>
  📅 Topic deferred to 2026-10-05: Round Trip
  [FAILED] in [It] - .../integration/cli_test.go:4521 @ 09/29/26 06:09:22.068
  << Timeline

  [FAILED] Expected
      <string>: ---
      defer_date: "2026-10-05"
      status: in_progress
      ---
      # Round Trip

  to contain substring
      <string>: defer_date: "2026-10-06"

Summarizing 1 Failure:
  [FAIL] vault-cli integration tests vault-cli topic command family [It] topic defer writes defer_date for a relative and an absolute date
  .../integration/cli_test.go:4521

Ran 1 of 232 Specs in 5.015 seconds
FAIL! -- 0 Passed | 1 Failed | 0 Pending | 231 Skipped
--- FAIL: TestIntegration (5.02s)
FAIL	github.com/bborbe/vault-cli/integration	5.381s
```

The CLI wrote `defer_date: "2026-10-05"` — the local date `2026-09-28` plus seven days. The spec demanded `"2026-10-06"` — the UTC date `2026-09-29` plus seven. The same failure was measured in the natural window on 2026-09-29 at 00:26 CEST, where the roles mirror: the CLI wrote `"2026-10-06"` (local 09-29 +7) and the spec demanded `"2026-10-05"` (UTC 09-28 +7).

## Expected vs Actual

Expected: a date the CLI writes for a vault file is resolved once, on one basis, and the test that asserts it uses the same basis. `defer_date` is a calendar date the user reads in the vault, so "seven days from today" means seven days from the user's today — the local calendar date. The implementation already does this. Nothing in the repository documents a UTC basis for vault dates; the only UTC reference is the test's own expectation.

Actual: the test computes its expectation from the UTC calendar date while the CLI writes the local one. On a tree with no changes, `make test` is red whenever the two calendar dates differ, and because that suite is dark-factory's `preflightCommand`, the pipeline refuses to start any prompt for the duration.

## Goal

The `topic defer` integration spec and the CLI agree on one date basis — the local calendar date — and the suite no longer contains a parent-process timezone assignment that silently disagrees with the subprocess it spawns. The spec fails on the unfixed tree and passes on the fixed tree under a zone pin, at any hour, with no dependence on wall-clock. The CLI's user-visible output is unchanged.

## Non-goals

- Changing the CLI's date semantics. `defer_date` is already local-basis and stays local-basis; this spec changes the test and the suite, not the resolver.
- Adding a subprocess-visible clock override (a `VAULT_CLI_NOW`-style env var or a `--now` flag) so the run date can be faked. The zone pin below already gives a deterministic discriminator; a clock seam is a larger change with its own contract to design.
- Auditing other specs in the suite for the same parent-versus-child timezone divergence. This spec owns the one that goes red.
- Making the suite's timezone handling a general convention beyond the line that causes this failure.
- A new scenario. See the scenario-coverage note under Acceptance Criteria.

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| Change the CLI to compute from the UTC calendar date | Inverts the product decision: `defer_date` is a date the user reads, and `+7d` means seven days from the user's today. It also changes user-visible output — a deferral made between 00:00 and 02:00 CEST would land a day earlier than the user asked. The implementation is the correct side. |
| Pin `TZ` for the test run and leave `time.Local = time.UTC` in place | Already attempted and recorded as failed on the sibling task: `time.Local = time.UTC` is an assignment, not a default, so it overrides `TZ` in the parent process. Only the child moves. The two sides stay disagreeing. |
| Pin `time.Local` to `time.UTC` in the parent *and* pass `TZ=UTC` to the child | Makes both sides agree, and makes the whole class of failure invisible: with both sides at UTC, local date and UTC date can never differ, so any test asserting the divergence passes vacuously. It also contradicts the local-basis decision. |
| Add a subprocess-visible clock override and fake the run date | Solves a different problem (deterministic run dates) at the cost of a new env-var or flag contract, and is out of scope per Non-goals. The zone pin is enough. |
| Mark the spec as skipped in the failing window | Hides the disagreement rather than settling it, and leaves dark-factory preflight blocked whenever the skip is not in force. |
| Do nothing | See the Do-Nothing Option below. |

## Acceptance Criteria

Fixture and command convention for the ACs below: `<pin>` is a `TZ` value whose local calendar date differs from the UTC calendar date at the moment of the run — the ACs use `Pacific/Pago_Pago` — and every AC that runs the suite prints both dates in the same invocation so the divergence is on the record. The focused spec is selected with `-ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"`. The suite runs as `go test ./integration/ -count=1 -run TestIntegration`.

- [ ] **The focused spec passes under `<pin>`.** `TZ=<pin> go test ./integration/ -count=1 -run TestIntegration -ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"` exits 0, with `TZ=<pin> date +%F` and `TZ=UTC date +%F` printed in the same invocation and shown to differ — evidence: exit code 0 plus the two printed dates plus the Ginkgo summary line (`1 Passed | 0 Failed`). The printed-date pair is the discriminator: an exit-0 run in a zone where the two dates are equal is green on the broken tree too and proves nothing.
- [ ] **The CLI still resolves `defer_date` from the process's local calendar date, not UTC.** Under `TZ=<pin>`, a `topic defer "+7d"` run through the built binary writes the local-basis date (`2026-10-05` when `<pin>` is `Pacific/Pago_Pago` and UTC is `2026-09-29`), not the UTC-basis date (`2026-10-06`) — evidence: the written `defer_date` value, asserted against both candidate dates so the assertion distinguishes them. This is the regression lock: the fix must not "resolve" the disagreement by moving the CLI onto the UTC basis.
- [ ] **`integration/integration_suite_test.go` no longer assigns `time.Local`.** `grep -n 'time\.Local' integration/integration_suite_test.go` returns no assignment line — evidence: negative evidence, written as `! grep -q 'time\.Local' integration/integration_suite_test.go` so a zero-match result is an exit-0 pass rather than a `grep -c` exit-1 false failure. If the implementation instead reconciles the child's zone rather than removing the line, this AC is satisfied by a `TZ` set for the child *and* a parent zone that agree — but the parent assignment must not remain the only zone source.
- [ ] **The source reads on the local basis in both places — the test's expectation and the implementation's comment.** `integration/cli_test.go`'s `expectedRelative` expression contains no `.UTC()`, and a one-line comment directly above `libtime.ToDate(now.AddDate(0, 0, days))` in `pkg/ops/defer_date_parser.go` names the local calendar date as the basis — evidence: file content — `grep -n 'expectedRelative :=' integration/cli_test.go` returns a line whose text does not contain `UTC()`, and `grep -n -B1 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go` shows an adjacent comment whose text contains `local`. A call-site-only edit is not sufficient on its own: AC 1 is what proves the effective basis.
- [ ] **`make precommit` is green under `<pin>`.** `TZ=<pin> make precommit` exits 0 with the two dates printed in the same invocation and shown to differ — evidence: exit code 0 plus the printed dates. Run in a zone where the dates agree this is green on the broken tree as well, so the printed pair is part of the evidence.
- [ ] **The suite's non-`topic defer` behavior is unchanged.** The full integration suite under `<pin>` reports the same pass/fail set as it does under `TZ=UTC` apart from the spec this change repairs — evidence: the two Ginkgo summary lines quoted side by side. This is the regression lock against a suite-wide zone change that repairs one spec by breaking others.
- [ ] `CHANGELOG.md` carries an `## Unreleased` bullet prefixed `fix:` describing the test-basis repair — evidence: file content — the `## Unreleased` section contains a line starting with `- fix:`. Assert with the section-walking form (`awk '/^## /{sec=$0} /<bullet-pattern>/{print sec}' CHANGELOG.md` returning `## Unreleased`), not with a line-window `grep -A`, which swallows a neighbouring section and reports a false positive. On a branch where the releaser has already cut a release, assert against the newest `## vX.Y.Z` section instead — same bullet, same `fix:` prefix.
- [ ] **Post-Deploy (Rung-2):** the released binary carries the repaired spec — on the released tag, `TZ=<pin> go test ./integration/ -count=1 -run TestIntegration -ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"` exits 0 with both dates printed and shown to differ, and the pre-change reproduction from the Reproduction section no longer reproduces — evidence: exit code 0 plus the printed date pair plus the recorded replay transcript in the source task's `# Results` section. Before/after is the strongest form here: the same command fails on `6be825f` and passes on the released tag, both quoted.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`

**Scenario coverage: no new scenario.** The repaired behavior is reachable by the existing `integration/` harness, which builds the real binary and asserts against a temp vault — no Docker, no cluster, no `gh`, no external service. `docs/rules/scenario-writing.md` names this case explicitly: a bug fix whose original failure would have been caught by a unit or integration test that simply did not assert the right thing needs that test fixed, not a scenario. None of the four conditions holds, so the default applies.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
# the discriminator — both dates printed beside the result
TZ=Pacific/Pago_Pago date +%F ; TZ=UTC date +%F
TZ=Pacific/Pago_Pago go test ./integration/ -count=1 -run TestIntegration \
  -ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"

# the change landed (negative forms, so a zero match is a pass)
! grep -q 'time\.Local' integration/integration_suite_test.go
grep -n 'expectedRelative :=' integration/cli_test.go        # no UTC() in the line
grep -n -B1 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go   # comment above, naming the local basis
awk '/^## /{sec=$0} /^- fix:/{print "sits under: " sec}' CHANGELOG.md
```

Note on the `TZ` pins above: they are zone selections for a single `go test` invocation, not a clock override and not a config change — the suite's own `time.Local = time.UTC` is what currently defeats them for the parent process, and removing that assignment is part of the fix. After the fix the parent honours `TZ` exactly as the child already does.

### Operator-executable (runs on the host after PR merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Before/after reproduction replay against the fresh binary
TZ=Pacific/Pago_Pago date +%F ; TZ=UTC date +%F
TZ=Pacific/Pago_Pago go test ./integration/ -count=1 -run TestIntegration \
  -ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"

# Version alignment, then install
make release-check
make install
vault-cli --version

# Install the Claude Code plugin manifests (separate artifact)
claude plugin update vault-cli@vault-cli   # then restart Claude Code
```

## Desired Behavior

1. The `topic defer` integration spec computes its expected `defer_date` from the same clock source and the same basis as the implementation — the process's local calendar date, offset by the requested number of days — and `integration/integration_suite_test.go` no longer assigns `time.Local = time.UTC`, so a `TZ` set for the run governs the test process and the CLI subprocess alike. Under a `TZ` whose local date differs from the UTC date, the expectation and the CLI's output agree, and the spec passes. If the implementation prefers to keep the parent at UTC and set the child's zone to match, that is acceptable provided both sides resolve the same zone and the divergence is still exercisable — but the parent assignment must not remain the only zone source, because that is what makes the child disagree.
2. The CLI's resolution is unchanged: `defer_date` for a relative offset remains the local calendar date plus that offset, resolved through `libtime.ToDate` from the injected `libtime.CurrentDateTime`. A user who runs `vault-cli topic defer "<name>" "+7d"` in `Europe/Berlin` gets the same date before and after this change.
3. A one-line comment directly above the `defer_date` computation in `pkg/ops/defer_date_parser.go` states that the basis is the local calendar date, so the next reader does not have to re-derive it from `libtime.ToDate`'s internals.
4. `make precommit` and `make test` are green under a `TZ` whose local date differs from the UTC date, at any hour. The failure is no longer a function of wall-clock, so dark-factory preflight no longer refuses runs during a daily window.
5. The suite's other specs behave identically to before under `TZ=UTC` and under a divergent pin, apart from the repaired spec.
6. `CHANGELOG.md` carries an `## Unreleased` `fix:` bullet describing the test-basis repair.

## Assumptions

- `defer_date` is a user-read vault date and the local calendar date is the correct basis for it. This is the product decision this spec records; the implementation already behaves this way.
- The suite's `time.Local = time.UTC` was added to reduce locale flakiness in the test process and carries no product intent. Nothing in the repository documents it as a basis statement.
- `TZ` is honoured by the Go runtime in both the test process and the spawned binary once the parent assignment is removed. The child already honours it — that is what makes the pinned-zone reproduction work today.
- No other spec in `integration/` depends on the parent process being at UTC in a way that removing the assignment would break. AC 7 is the check for this; if it fires, the assignment is narrowed rather than removed.

## Constraints

- The CLI's `defer_date` semantics are frozen: relative offsets resolve from the local calendar date, exactly as today. This spec does not change what the user sees.
- `libtime.ToDate`'s behavior is not modified — it belongs to `github.com/bborbe/time`, and its "calendar date in `value`'s own location" semantics are what make the local basis work.
- No subprocess-visible clock override is introduced (no `VAULT_CLI_NOW` env var, no `--now` flag). The zone pin is the determinism mechanism.
- No `TZ` is hard-coded into the suite, the test, or the CLI. The zone is selected per invocation by whoever runs the test; the suite must not pin one of its own.
- Tests follow repository convention: Ginkgo v2 / Gomega. The repaired spec stays where it is; this change does not restructure the suite.
- `.dark-factory.yaml` is unchanged — in particular `preflightCommand: "make precommit"` stays as it is. The fix removes the reason preflight fails, rather than relaxing the gate.
- Paths in this spec and in any generated prompt are repo-relative.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---|---|---|---|---|
| A future change re-introduces a parent-only `time.Local` assignment | AC 3 fails: the grep finds the assignment, and AC 1 fails under the pin because the parent and child diverge again | Remove the assignment, or set the child's zone to match the parent's | `! grep -q 'time\.Local' integration/integration_suite_test.go` plus the pinned-zone run | Reversible — one line |
| The suite is run without a divergent `TZ` | The suite is green, but the run does not exercise the repaired path | Re-run with `<pin>`; the printed date pair makes the omission visible | The two printed dates are equal | Reversible |
| Removing `time.Local = time.UTC` changes another spec's result | AC 7 fires: the pass/fail sets under `TZ=UTC` and under `<pin>` differ by more than the repaired spec | Narrow the change — set the parent zone explicitly to the child's zone instead of removing the assignment | The two Ginkgo summary lines quoted side by side | Reversible — restore the line, take the narrower fix |
| A future spec in the suite assumes the test process is at UTC | Same as above; AC 7 catches it at the point the assignment is removed, not later | Same narrower fix | AC 7 | Reversible |
| Midnight passes between the parent's expectation and the child's run, so the two resolve different calendar dates for the same offset | The spec fails on a tree with no changes. This is the one residual flake the fix does not remove: parent and child share a zone, but they read the clock at two different instants, so a run straddling midnight still sees two dates | Re-run — the window is milliseconds wide and recurs once per day. Not widened by this change, and not newly introduced by it | The failure appears only on a run that straddles midnight, and both sides print the same zone, which distinguishes it from the bug being repaired | Reversible — re-run |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Reconcile the suite's zone source (`integration/integration_suite_test.go`), flip the spec's expectation to the local basis (`integration/cli_test.go:4518`), and add the basis comment in `pkg/ops/defer_date_parser.go` — one atomic change, since the spec only passes when all three land together | 1, 2, 3, 4 | 1, 2, 3, 4, 5, 6, 7 | — |
| 2 | `CHANGELOG.md` `## Unreleased` `fix:` bullet | 6 | 8 | prompt 1 |

Rationale: this is a single-layer, single-behavior fix. The three code edits are not independently verifiable — the spec stays red until the suite's zone source and the expectation both change, and the CLI-side comment is documentation of the decision the other two implement. Splitting them across prompts would produce an intermediate tree where the suite is red for a new reason, which is exactly the condition this spec exists to remove. AC 9 is operator-executed after merge and is not a prompt.

## Do-Nothing Option

`make test` and `make precommit` go red on a tree with no changes whenever the local calendar date differs from the UTC date — currently two hours a day in `Europe/Berlin`, and the same class of window in any zone. Because `preflightCommand: "make precommit"` is the daemon's gate and preflight failure is terminal, dark-factory cannot start a single prompt on this repository for the duration, so every queued spec and every standalone prompt stalls on a schedule. The cost recurs daily until fixed, and it is invisible for the other twenty-two hours — which is what makes it survive: a developer who runs `make test` at midday sees green and concludes the tree is fine.
