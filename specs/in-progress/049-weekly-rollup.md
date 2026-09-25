---
status: verifying
approved: "2026-09-16T12:50:32Z"
generating: "2026-09-16T13:08:22Z"
prompted: "2026-09-16T13:08:22Z"
verifying: "2026-09-25T19:14:51Z"
branch: dark-factory/weekly-rollup
---

## Summary

- A new `vault-cli rollup weekly` command answers "how much work shipped without me this week?" with three numbers instead of a feeling.
- The three numbers are human interactions in the week, unattended deliveries in the week, and the per-family median interactions.
- "Unattended delivery" is not re-invented here — the command applies the rule the vault already wrote down.
- "Family" is not re-invented either — the command applies the filename-stem grouping rule the vault already uses.
- The command reads task frontmatter and prints a report. It never writes to the vault.

## Problem

The vault has recorded per-task interaction counts since v0.116.0 — several hundred task files carry one — and the agent side now records them too, through the controller's write-back. Nothing reads either half. The question the whole instrument exists to answer, *how much work shipped without me this week*, is answerable today only by hand, and the objective's north star — human time per recurring task going down — cannot be computed at all. Offload could be working brilliantly or not at all and the data would look identical. Every graduation decision so far has been made on human-side data plus judgment.

## Goal

A `vault-cli rollup weekly` command that, given a vault and a week, prints the three figures with the rule and the grouping stated alongside them, reproducibly — so the headline question has an answer that is a number rather than a feeling.

## Non-goals

- **Recording the 2026-09-12 baseline** — a separate task seeds the rollup with it; this spec builds the machine.
- **Authoring the unattended-delivery definition** — the rule is consumed, not authored here. It lives in the vault's `Unattended Execution` topic page.
- **Producing the agent-side count** — upstream work; the rollup reads whatever the frontmatter carries.
- **Acting on the numbers** — choosing what to graduate next is the topic's job.
- **Dashboard or chart work** — the deliverable is a rollup report, not a UI.
- **Backfilling history** — tasks completed before the metrics existed stay as they are.
- **A cross-vault aggregate** — see Constraints; the figures are meaningful per task population, and summing unrelated populations answers no question anyone asked.
- **A new E2E scenario** — the computation is reachable by the Ginkgo suite with fixtures, and the real-vault behaviour is carried by the operator-executable ACs. The four-condition scenario test fails on the first condition, matching the sibling `036-passive-per-task-metrics` decision.

## Assumptions

- The vault's task frontmatter is the only source. No session logs, no external system, no second data store.
- `metrics_completed_at` is written for every task that completes through the lifecycle, and carries an RFC3339 timestamp with a local offset.
- The agent-side count, where it exists, is already merged into the same frontmatter by the controller's write-back — the rollup does not know or care which writer produced a count.

## Acceptance Criteria

All commands below are run with `--vault personal`, which is the vault that carries the metrics, **except AC 4, AC 6 and AC 7, which run against the scratch vault `rollup-scratch`** (see Constraints and the setup line in Verification). `$NEW` denotes a freshly built binary (see Verification).

- [ ] **The command reports all three figures for a named week** — evidence: `$NEW rollup weekly --vault personal --week 2026-W37` exits 0 and stdout carries a line labelled `Human interactions`, a line labelled `Unattended deliveries`, and a line labelled `Per-family median` carrying the headline median of per-family medians; the per-family lines are listed beneath it, one per family.
- [ ] **A week with no data is reported as no data, never as zero** — evidence: `$NEW rollup weekly --vault personal --week 2026-W20` (before the metrics existed) prints an explicit no-data statement, and `$NEW rollup weekly --vault personal --week 2026-W20 | grep -cE 'Human interactions:[[:space:]]*0$'` returns 0; **and** `make test` runs green on a suite case building a non-empty fixture week in which no member carries a recorded count, asserting `no recorded counts` for the two scalar figures and `undefined` for the headline — the one state no real week exercises today.
- [ ] **Unattended deliveries follow the written rule, and an absent count is not a delivery** — evidence: stdout contains the vault's rule sentence — `A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0` — so a literal substring check is possible; and `make test` runs green on a suite case that builds a fixture week containing a completed task with an **absent** `metrics_interaction_count` key and asserts that task is excluded from `Unattended deliveries` — a body-level assertion, not a case name.
- [ ] **The grouping rule is stated and reproducible** — evidence: stdout states the filename-stem grouping rule in one sentence; `diff <($NEW rollup weekly --vault rollup-scratch --week 2026-W37) <($NEW rollup weekly --vault rollup-scratch --week 2026-W37)` returns empty, so the group list is byte-identical across runs.
- [ ] **A family with no recorded count reports `undefined`, not zero** — evidence: for `2026-W37`, stdout marks at least one family `undefined` and `grep -c 'undefined'` returns ≥1; and `$NEW rollup weekly --vault personal --week 2026-W37 | grep -cE '^[[:space:]]+[^:]+: 0$'` returns **0** — no W37 family's recorded values median to 0 as of 2026-09-16, so a non-zero count means the implementation is printing `0` where it should print `undefined`.
- [ ] **Re-running reproduces the JSON output exactly** — evidence: `diff <($NEW rollup weekly --vault rollup-scratch --week 2026-W37 --output json) <($NEW rollup weekly --vault rollup-scratch --week 2026-W37 --output json)` returns empty.
- [ ] **Each figure responds to a vault edit by exactly the expected amount** — evidence: state transition, three probes run one at a time **against the scratch vault `rollup-scratch`** (see Constraints), with `shasum -a 256 $NEW` recorded before each probe and identical across all three — proving every probe ran the same binary. (a) one task's `metrics_interaction_count` raised by a known delta → `Human interactions` rises by exactly that delta. (b) one completed task's `metrics_interaction_count` set to `0` → `Unattended deliveries` rises by exactly 1 and `Human interactions` falls by exactly that task's prior count. (c) one task added to a family → that family's median moves to the value hand-computed over the enlarged member set, and the headline `Per-family median` moves to the median recomputed over the families' new medians. Each probe states its predicted delta for all three figures — including the headline — before it runs. Probe (b)'s task is drawn from W37's set and carries a prior recorded count greater than 0; probe (c)'s added task carries a recorded count, and the headline recomputes to the hand-computed value whether or not it moves.
- [ ] **JSON output carries the same figures** — evidence: `diff <($NEW rollup weekly --vault personal --week 2026-W37 | grep -E '^(Human interactions|Unattended deliveries|Per-family median)') <($NEW rollup weekly --vault personal --week 2026-W37 --output json | jq -r '"Human interactions: \(.human_interactions)", "Unattended deliveries: \(.unattended_deliveries)", "Per-family median: \(.per_family_median)"')` returns empty.

## Verification

`$NEW` = a binary built in this worktree. The installed `vault-cli` predates this change and has no `rollup` verb, so every operator command below uses the fresh build.

```bash
go build -o /tmp/new-vault-cli .
NEW=/tmp/new-vault-cli
VAULT=$(vault-cli config list --output json | jq -r '.[] | select(.name=="personal") | .path')
```

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint / vet / vuln / format / generate / test / changelog checks clean
- `make test` — unit and integration suite passes
- `grep -rn 'rollup' pkg/cli/*.go` — the command is registered (wiring may land in `cli.go` or a sibling `pkg/cli/rollup.go`; the repo has precedent for both, so do not pin the file)
- `grep -nE 'Describe|It\(' pkg/ops/rollup_weekly_test.go` — the computation's Ginkgo suite exists (this package has one `func TestSuite` entry point; a second `func Test*` calling `RunSpecs` panics, so do not add one)

### Operator-executable (runs on the host against the real vault, after the change lands)

- `$NEW rollup weekly --vault personal --week 2026-W37` — the three figures print with the rule and the grouping
- `$NEW rollup weekly --vault personal --week 2026-W20` — no-data week reports no data
- `diff <($NEW rollup weekly --vault personal --week 2026-W37) <($NEW rollup weekly --vault personal --week 2026-W37)` — empty
- `$NEW rollup weekly --vault personal --week 2026-W37 --output json | jq .` — parses, figures match the plain run
- `shasum -a 256 $NEW` recorded before each probe — identical across all three, proving every probe ran the same binary
- `git -C "$VAULT" diff --stat` after a read-only run — empty (the command never writes to the vault)
- Scratch-vault setup for AC 4, AC 6 and AC 7: `cp -R "$VAULT" /tmp/rollup-scratch`, then register it in `~/.config/vault-cli/config.yaml` under the name `rollup-scratch` — the exact name those ACs pass to `--vault` — with the same `tasks_dir`. Registration is config-file only; there is no env-var or `--config` override. Teardown: remove the config entry, then delete `/tmp/rollup-scratch`.

## Desired Behavior

1. `vault-cli rollup weekly` accepts `--week YYYY-Wnn` and operates on exactly one vault, selected by the standard `--vault` flag. With no `--vault` it resolves the config's `default_vault` key, exactly as `GetVault("")` does — never the `getVaults` all-vaults default. With no `--week` it reports the last complete ISO week. It prints the week and its date range.
2. A task belongs to a week by the **date component of its `metrics_completed_at`, in that timestamp's own offset** — so a task completing Monday 00:30 local is in that local week, not the previous UTC one. It belongs whatever its `status`. The week's task set is every task file under the resolved vault's configured `tasks_dir` carrying a `metrics_completed_at` inside that week.
3. `Human interactions` is the sum of `metrics_interaction_count` over that week's task set.
4. `Unattended deliveries` counts the tasks in that set that are `status: completed` **and** carry a recorded `metrics_interaction_count` of exactly `0`. A task whose count key is absent is not a delivery — it is indeterminate, and is counted in neither direction.
5. `Per-family median` is reported at two levels. **Per family:** the median of that family's recorded counts in the week's set; a family with no recorded count among its members reports `undefined` and contributes no datum to any aggregate. **Headline:** the median of those per-family medians, taken over the families whose median is defined — so the headline aggregates family medians, never raw per-task values, and a week whose families are all `undefined` is the no-counts week of DB 6.
6. A week in which no task carries a `metrics_completed_at` reports no data explicitly. A week whose set is non-empty but in which **no member carries a recorded count** reports `no recorded counts` for `Human interactions` and `Unattended deliveries`, and `undefined` for `Per-family median` — matching that week's per-family lines, which are all `undefined`. Neither case prints `0`.
7. The output states the unattended-delivery rule and the grouping rule, so a reader can check the figures without leaving the terminal. Each headline figure prints as `Label: value` — the label, a colon, one space, the value, and nothing else on the line; per-family lines are indented beneath the headline. An undefined figure renders as the string `undefined` in **both** plain and JSON output — never `null`, never `0` — so the two formats stay comparable line for line.

## Constraints

- **One vault, never an aggregate.** This deliberately diverges from `getVaults`'s "no flag → all configured vaults" default: the three figures are properties of one task population, and summing Personal with Brogrammers would produce a number that answers no question. `--vault` selects it; with the flag absent the command resolves the config's `default_vault` key via `GetVault("")`, never the all-vaults path.
- **Read set is the resolved vault's configured `tasks_dir`, frontmatter only.** No data file, no cache, no sidecar. The directory is read from vault config (`tasks_dir`), never hardcoded — configured vaults use `tasks`, `24 Tasks`, and `25 Tasks` variously. The command reads nothing else and writes nothing.
- **Parse frontmatter, never grep lines.** A line-grep for `metrics_interaction_count: 0` over the task directory over-counts — sample-YAML lines inside task bodies match too. Frontmatter-scoped extraction is the only correct read.
- **An absent count is never coerced to zero.** The existing typed accessor already encodes this contract (its doc comment: an absent key or a non-numeric value yields nil, never 0); reuse it rather than reading the key directly.
- **The grouping rule is the filename-stem rule already documented in the vault** — `Recurring Task Automation Ranking` § Method: strip dates, week numbers, versions and month names from each filename, then group. Normalization is case-insensitive on the stripped stem. The ≥3-instance threshold governs whether a family counts as recurring evidence, never membership — a 1–2 member family stays in the median.
- **The unattended rule is the vault's written rule verbatim** — `status` completed and `metrics_interaction_count` exactly `0`. Zero means a *recorded* zero.
- **Probes never touch live metrics history.** The vault autocommits on a schedule, so a probe that mutates a real task's count would become permanent history. Probes run against a copy of the vault registered as a scratch vault.
- **`pkg/ops/` never writes to stdout.** The operation returns a structured result; the CLI layer owns formatting. See `docs/development-patterns.md`.
- **`--output plain` is the default; `--output json` uses the repo's existing JSON printer.** No `encoding/json` import in a command file.
- **Dates come from the injected clock**, never `time.Now()`.
- **Why a CLI verb and not a vault-local script** — recorded here because the reasoning would otherwise live only in a vault task page: the verb reuses the storage layer's frontmatter parse (a script reading files directly re-opens the bare-wikilink corruption class the read path guards against), reuses the typed metrics accessor whose contract encodes the absent-key rule, and is exercisable by `make test`. The stored counts were themselves produced by the existing distinct-session-id counting, so the rollup inherits that semantics without re-deriving it — it reads the stored numbers, never the session logs that produced them.
- **Do NOT commit** — dark-factory handles git.
- Existing task/goal/theme commands and their output are unchanged, and `make test` / `make precommit` stay green.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| `--week` malformed (not `YYYY-Wnn`), or a week number outside the resolved year's ISO week count (2026 has 53, so `W53` is valid and must not be rejected) | Exit non-zero with a usage error naming the expected format; no figures printed | Operator re-runs with a valid week; exit 0 and the three labelled lines print |
| Week contains no task with `metrics_completed_at` | Explicit no-data statement; exit 0 | None required — a legitimate week |
| Week's set is non-empty but no member carries a recorded count | The two scalar figures report `no recorded counts`, the third `undefined` (DB 6); exit 0 | None required |
| `metrics_*` value is malformed (non-numeric) | Treated as absent, per the existing accessor's contract; no crash, no coercion to 0 | None required |
| A task file is unreadable mid-scan | Exit non-zero naming the file; the scan never reports a silently smaller week | Operator checks file permissions, re-runs |
| The resolved vault's task directory is missing | Exit non-zero naming the vault and the resolved directory (`tasks_dir` defaults to `Tasks` when unset, so an unset key is not itself an error) | Operator checks `vault-cli config list --output json` |
| Two tasks complete either side of a local midnight within the same UTC week | Each lands in its own local week, per the offset in its own timestamp | None required — the rule is stated in Desired Behavior 2 |
| A probe mutates a task count and is not restored | Impossible by construction — probes run against a scratch copy | Delete the scratch copy |

## Security / Abuse

The command reads only the resolved vault's own task files and writes nothing. It takes no free-form input beyond a week token and a vault name, both validated before use. No network, no credentials, no external system. The one untrusted-input surface is the task frontmatter itself: a malformed or hostile `metrics_*` value must be treated as absent rather than parsed into a number, and must never panic the scan.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | The computation: week set, three figures, family grouping, undefined-median, no-data and no-counts states, plus its Ginkgo suite including the reproducibility cases | 2, 3, 4, 5, 6 | 2, 3, 5, 6 | — |
| 2 | CLI wiring + report formatting (plain and JSON), rule and grouping stated | 1, 7 | 1, 4, 8 | prompt 1 |

Rationale: prompt 1 establishes the computation and its unit contract, which is where the absent-key, undefined-median, and no-counts rules live; its suite also carries the determinism property that AC 6 asserts. Prompt 2 is the user-visible surface and depends on it. **AC 7 is assigned to no prompt — it is verified by the operator rung only:** the container has no vault mount, so the probes cannot run at prompt time, and a test-only prompt could lock determinism but could not tick the criterion.

## Do-Nothing Option

The vault keeps recording interaction counts that nothing reads. The goal's headline question stays unanswerable, the agent-side counts shipped by two upstream tasks remain machinery nobody consumes, and every future graduation decision continues to be made on human-side data plus judgment. The cost is not the missing report — it is that the measurement cannot distinguish offload working from offload not working.
