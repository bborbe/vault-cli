---
status: verifying
approved: "2026-09-26T15:04:34Z"
generating: "2026-09-26T15:38:46Z"
prompted: "2026-09-26T16:21:47Z"
verifying: "2026-09-26T17:06:32Z"
branch: dark-factory/rollup-weekly-baseline-delta
---

## Summary

- `vault-cli rollup weekly` gains a second input: a baseline captured before the rollup existed.
- The baseline is a vault markdown file; its path comes from a new `baseline` key on the vault's config entry, set by a new `vault-cli config set-baseline <vault> <path>` verb.
- With a baseline configured, the report prints the five stored baseline figures verbatim, then one delta row per figure the rollup computes that has an analogue in the baseline.
- The baseline is never recomputed and never rewritten — it is stored text echoed back, so the five figures stay exactly as captured.
- With no baseline configured, the output is byte-identical to today's — no baseline block, no delta row.

## Problem

Spec 049 built the measuring machine: `rollup weekly` computes a week's human interactions, unattended deliveries, and per-family median from task frontmatter. A number with nothing to compare it against answers "how much" but not "is that better". The 2026-09-12 baseline — human total 62,485, median 64, W36 25,141, W37 26,476, agent coverage 1 of 420 — was captured precisely so that later weeks could be read as a movement from a fixed point, and spec 049 explicitly assigned seeding it to a separate task. Without that task, every rollup run produces a figure the reader must remember the baseline for, and the objective's north star — human time per recurring task going down — still has no fixed point to fall from.

## Goal

A `vault-cli rollup weekly` run against a vault whose config names a baseline prints that baseline's five stored figures and capture date, followed by the delta between each figure the rollup computes and its analogue in the baseline — so a week's number reads as a movement from a recorded fixed point rather than as a bare figure, and a vault that names no baseline keeps printing exactly what it prints today.

## Non-goals

- **Recomputing, normalising or adjusting the baseline.** The five figures are stored text echoed verbatim. The recorded definitional mismatches (below) are surfaced, not reconciled.
- **Producing the baseline itself.** Seeding the file with the 2026-09-12 figures is an operator act performed on the operator rung of Verification — the operator (Benjamin) authors the file and runs `config set-baseline`; this spec's code reads whatever the file carries. Until that file exists, the operator-rung ACs are blocked, not failing.
- **More than one baseline per vault.** One vault names one baseline file.
- **Refreshing the baseline.** The rollup never writes to it; a new baseline is a new operator-authored file plus a `config set-baseline` call.
- **A delta for figures with no analogue** — `Unattended deliveries`, the baseline's `total` (no per-week analogue), and `agent coverage` (a coverage figure the rollup does not emit).
- **A `config unset-baseline` / `--unset` verb.** Removing a baseline is a config-file edit. Do NOT add an unset verb — if a consumer demands one, that is a separate spec.
- **A cross-vault baseline** — a baseline is a property of one task population, on the same reasoning as spec 049's one-vault constraint.
- **A new E2E scenario** — the baseline parse, the analogue mapping and the delta arithmetic are all reachable by the Ginkgo suite with fixture vaults, and the real-vault behaviour is carried by the operator-executable ACs. The four-condition scenario test fails on the first condition, matching spec 049's decision.

## Assumptions

- Spec 049's `## Assumptions` pin — "The vault's task frontmatter is the only source. No session logs, no external system, no second data store" — is **narrowed, not repealed**. It governs the *computation*: the three rollup figures remain computed exclusively from task frontmatter. The baseline file is a fourth thing the command reads, and it is deliberately **not a computational source** — it is passthrough text that contributes no datum to any computed figure. A baseline file that is deleted changes the output's presentation and never changes a computed number.
- The five baseline figures are a historical capture and will disagree with a freshly computed week. The disagreements are expected and are recorded below rather than corrected.
- The operator authors the baseline file and points config at it; the rollup only reads it.
- The vault's config file is resolved by the loader in this order: the `--config` value when given; otherwise `~/.config/vault-cli/config.yaml` when the `~/.config/vault-cli/` directory exists; otherwise the legacy `~/.vault-cli/config.yaml` when that directory exists; otherwise the XDG path as the default. It is operator-owned: a config edit is reversible by editing the file back.

## Recorded definitional mismatches (never adjusted)

Captured 2026-09-12. These are stored as-is; the rollup surfaces them and does not reconcile them.

| Baseline figure | Rollup analogue | Mismatch | Handling |
|---|---|---|---|
| `W37: 26,476` | `rollup weekly --week 2026-W37` `Human interactions` = 39,055 | 12,579 apart — the baseline was captured mid-W37, so the two cover different windows | Both values print; the delta is computed and shown as a movement, not a defect |
| `median: 64` | headline `Per-family median` (W37 = 113) | `64` is a per-task median; the headline is a median of per-family medians | Both values print; the delta row carries the frozen marker `[definitional mismatch]` |
| `total: 62,485` | none | no per-week analogue | echoed verbatim; no delta row |
| `agent: 1 of 420` | none | a coverage figure the rollup does not emit | echoed verbatim; no delta row |

## Acceptance Criteria

`$NEW` = a binary built in this worktree (`go build -o /tmp/new-vault-cli .`). `$PRE` = a binary built at the branch point before this change. `$CONFIG` = the config file path resolved by the loader's own rule (see Verification). `rollup-scratch` = the scratch vault described in Verification, carrying only the **distinct fixture figures** (`111`, `7`, `222`, `333`, `2 of 9`, captured `2026-01-02`). The real 2026-09-12 figures live only in the `personal` vault's baseline file.

- [ ] **`make precommit` exits 0 in the repo root** — evidence: exit code.
- [ ] **The baseline block is rendered from the configured file, not compiled in** — evidence: `make test` runs green on a suite case that points a fixture vault at a baseline file carrying the real figures and asserts stdout carries `Baseline (captured 2026-09-12)`, `Human interactions (total): 62485`, `Median: 64`, `Week 2026-W36: 25141`, `Week 2026-W37: 26476`, and `Agent coverage: 1 of 420`; **and** `make test` runs green on a second case whose fixture baseline file carries distinct figures (`111`, `7`, `222`, `333`, `2 of 9`, captured `2026-01-02`) and asserts those distinct figures print while `grep -cE '62485|25141|26476'` over that run returns **0** — a hardcoded Go constant fails this case.
- [ ] **Editing the configured baseline file changes the printed block** — evidence: state transition — against `rollup-scratch`, whose baseline file is at a known path, the rollup prints `Median: 7`; the file's median is rewritten to `8` and the same command re-run prints `Median: 8` and `Median: 7` is absent; the file is restored to `7` and the command re-run prints `Median: 7` again; `shasum -a 256 $NEW` is identical across all three runs, proving one binary produced all three outputs.
- [ ] **Each delta equals the rollup's own figure minus the baseline analogue, hand-computed, and a figure with no analogue produces no delta row** — evidence: against `rollup-scratch` (baseline week `2026-W37` = `222`, baseline median `7`), stdout asserts the delta rows read `Human interactions: <computed> - 222 = <computed minus 222>` and `Per-family median: <computed> - 7 = <computed minus 7>`, where `<computed>` is the figure the same run prints for that label; **and** the delta block is scoped by its frozen header — `$NEW rollup weekly --vault rollup-scratch --week 2026-W37 | awk '/^Delta/&&!f{f=1}f' | grep -c 'Unattended deliveries'` returns **0** — no delta row exists for a figure with no analogue; **and** the delta block's header line is the full frozen literal, not a prefix — `$NEW rollup weekly --vault rollup-scratch --week 2026-W37 | grep -cx 'Delta (vs baseline captured 2026-01-02)'` returns **1** (the scratch baseline's capture date); **and** the block order is fixed — `$NEW rollup weekly --vault rollup-scratch --week 2026-W37 | awk '/^Baseline/{b=NR} !h&&/^Human interactions: /{h=NR} /^Delta/{d=NR} END{exit !(b&&h&&d&&b<h&&h<d)}'` exits **0**, proving the baseline block precedes the computed figures and the delta block is last.
- [ ] **NEGATIVE — a vault with no baseline configured prints no baseline block and no delta row, byte-identically to the pre-change build** — evidence: `$NEW rollup weekly --vault <no-baseline-vault> --week 2026-W37 | grep -cE 'Baseline|Delta'` returns **0**; **and** `diff <($NEW rollup weekly --vault <no-baseline-vault> --week 2026-W37) <($PRE rollup weekly --vault <no-baseline-vault> --week 2026-W37)` returns empty.
- [ ] **Operator — the real baseline prints verbatim and the W37 delta is arithmetically consistent** — evidence: after the baseline file exists in the `personal` vault and `$NEW config set-baseline personal "<path>"` has run, `$NEW rollup weekly --vault personal --week 2026-W37` stdout carries `Baseline (captured 2026-09-12)`, `Human interactions (total): 62485`, `Median: 64`, `Week 2026-W36: 25141`, `Week 2026-W37: 26476`, `Agent coverage: 1 of 420`, and a delta line of the form `<computed> - 26476 = <delta>` in which `<computed>` equals the `Human interactions` figure the same run prints and `<delta>` equals `<computed>` minus `26476`; as of 2026-09-26 that line reads `39055 - 26476 = 12579`, and the AC holds at the arithmetic level if the vault has grown since.
- [ ] **Operator — the definitional mismatches are recorded, never adjusted, and the baseline file is not rewritten** — evidence: the `personal` run prints `Median: 64` and `Week 2026-W37: 26476` exactly as stored (`grep -c 'Median: 64'` returns **1**, `grep -c 'Week 2026-W37: 26476'` returns **1**) while the same run's headline `Per-family median` prints a different number — the stored `64` is never recomputed into a median-of-family-medians; the median delta row carries the frozen marker `[definitional mismatch]`; **and** `git -C "$VAULT" diff --stat -- "<baseline path>"` is empty after the run — the baseline file specifically was not rewritten.
- [ ] **Operator — NEGATIVE — the rollup still writes nothing to the vault** — evidence: `git -C "$VAULT" diff --stat` (whole vault, no pathspec) and `git -C "$VAULT" status --porcelain` are both empty after a run with a baseline configured.
- [ ] **JSON output carries the baseline figures and the deltas, preserving spec 049's plain/JSON parity; and NEGATIVE — `.baseline` is absent when no baseline is configured** — evidence: `$NEW rollup weekly --vault personal --week 2026-W37 --output json` yields `jq -e '.baseline.captured == "2026-09-12"'` exit 0, `jq -r '.baseline.human_total, .baseline.median, .baseline.agent_coverage'` printing `62485`, `64`, `1 of 420`, `jq -e '.baseline.weeks["2026-W37"] == 26476'` exit 0, and `jq -e '.baseline.deltas.human_interactions.delta == (.baseline.deltas.human_interactions.computed - .baseline.deltas.human_interactions.baseline)'` exit 0, plus `jq -e '.baseline.deltas.per_family_median.mismatch == true'` exit 0; **and** `$NEW rollup weekly --vault <no-baseline-vault> --week 2026-W37 --output json | jq 'has("baseline")'` prints `false`.
- [ ] **`vault-cli config set-baseline <vault> <path>` persists the key** — evidence: state transition — with the config file named explicitly, `$NEW config set-baseline personal "<path>" --config "$CONFIG"` exits 0 and `$NEW config list --config "$CONFIG" --output json | jq -r '.[] | select(.name=="personal") | .baseline'` prints the path; **and** NEGATIVE — a second call naming a vault absent from the config exits non-zero and `shasum -a 256 "$CONFIG"` is identical before and after.
- [ ] **NEGATIVE — `config set-baseline` rejects a path outside the vault** — evidence: `$NEW config set-baseline personal "/etc/passwd" --config "$CONFIG"` and `$NEW config set-baseline personal "../../outside.md" --config "$CONFIG"` each exit non-zero, each name the rejected path on stderr, and `shasum -a 256 "$CONFIG"` is identical before and after both calls.
- [ ] **NEGATIVE — a configured-but-misconfigured baseline fails loudly and prints nothing** — evidence: every probe runs against `rollup-scratch` and never against the `personal` vault, so no probe can leave live metrics history mutated. With `rollup-scratch`'s `baseline` set to a path that does not exist, `$NEW rollup weekly --vault rollup-scratch --week 2026-W37` exits non-zero with stderr naming the vault and the resolved path and `... | grep -c 'Baseline'` returns **0**; with the file present but its `baseline_median` key removed, the run exits non-zero with stderr naming `baseline_median` and `... | grep -cE 'Baseline|Median'` returns **0**; and with `rollup-scratch`'s `baseline` hand-edited to an absolute path (`/etc/passwd`) or to one escaping the vault root (`../../outside.md`), the run exits non-zero and `... | grep -cE 'Human interactions|Baseline|Delta'` returns **0** — no figures and no partial baseline block in any case. Each probe sets the scratch `baseline` key and the scratch baseline file to the state it needs and restores them before the next probe; the scratch config entry and `/tmp/rollup-scratch` are deleted in the Verification teardown, so no probe's mutation outlives the run and the `personal` vault is never touched.

## Verification

`$NEW` = `go build -o /tmp/new-vault-cli .` in this worktree. `$PRE` = a binary built at the branch point before this change (`git stash` the change, build, restore), used only for the byte-identity AC. The baseline file's location inside the vault is the operator's choice; the ACs below use the path recorded by `config set-baseline`. `$CONFIG` is the config file the loader resolves — `--config` value if given, else `~/.config/vault-cli/config.yaml` when `~/.config/vault-cli/` exists, else `~/.vault-cli/config.yaml` when that directory exists, else the XDG default. Every config-touching AC passes `--config "$CONFIG"` explicitly so it holds under all three resolutions:

```bash
if [ -d "$HOME/.config/vault-cli" ]; then CONFIG="$HOME/.config/vault-cli/config.yaml"
elif [ -d "$HOME/.vault-cli" ]; then CONFIG="$HOME/.vault-cli/config.yaml"
else CONFIG="$HOME/.config/vault-cli/config.yaml"; fi
```

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint / vet / vuln / format / generate / test / changelog checks clean
- `make test` — unit and integration suite passes, including the baseline fixture cases
- `grep -nE 'Describe|It\(' pkg/ops/rollup_weekly_test.go` — the baseline parse, analogue-mapping and delta cases exist in the rollup's Ginkgo suite (this package has one `func TestSuite` entry point; a second `func Test*` calling `RunSpecs` panics, so do not add one)
- `grep -rn 'set-baseline' pkg/cli/*.go` — the config verb is registered
- `grep -rn 'baseline' pkg/config/config.go` — the vault config entry carries the new key

### Operator-executable (runs on the host against the real vault, after the change lands)

- `go build -o /tmp/new-vault-cli .` then `NEW=/tmp/new-vault-cli`
- `VAULT=$(vault-cli config list --output json | jq -r '.[] | select(.name=="personal") | .path')`
- **Seeding step (owner: the operator, Benjamin) — required before AC 6, AC 7 and AC 9 can pass.** Author the baseline file inside the `personal` vault at a path of the operator's choosing, carrying exactly the five frozen frontmatter keys and the 2026-09-12 figures documented in `docs/baseline-file.md` (`baseline_captured: 2026-09-12`, `baseline_human_total: 62485`, `baseline_median: 64`, `baseline_weeks: {2026-W36: 25141, 2026-W37: 26476}`, `baseline_agent_coverage: "1 of 420"`), then run `$NEW config set-baseline personal "<vault-relative path>"`. This is the "separate task" spec 049's Non-goals assigned; it is an operator act, not a prompt, and the code under test cannot satisfy those three ACs without it.
- `$NEW config set-baseline personal "<vault-relative path to the baseline file>" --config "$CONFIG"` — the verb writes the key
- `$NEW config list --config "$CONFIG" --output json | jq -r '.[] | select(.name=="personal") | .baseline'` — the key reads back
- `$NEW rollup weekly --vault personal --week 2026-W37` — the baseline block and the delta rows print
- `$NEW rollup weekly --vault personal --week 2026-W37 --output json | jq '.baseline'` — the JSON carries the same figures and the same deltas
- `$NEW rollup weekly --vault personal --week 2026-W20` — a week absent from the baseline's week map prints the baseline block with no `Human interactions` delta row
- `diff <($NEW rollup weekly --vault <no-baseline-vault> --week 2026-W37) <($PRE rollup weekly --vault <no-baseline-vault> --week 2026-W37)` — empty
- `git -C "$VAULT" diff --stat` after every run — empty (the command never writes to the vault)
- Scratch-vault setup for AC 3, AC 4 and AC 12: `cp -R "$VAULT" /tmp/rollup-scratch`, register it in `$CONFIG` under the name `rollup-scratch` (the exact name those ACs pass to `--vault`) with the same `tasks_dir`, and write a baseline file inside it carrying **only the distinct fixture figures** (`111`, `7`, `222`, `333`, `2 of 9`, captured `2026-01-02`) plus fixture task data. The scratch vault never carries the real 2026-09-12 figures — those exist only in the `personal` vault's baseline file, and asserting them against the scratch vault is a contradiction. Registration is config-file only; there is no env-var override for the vault name. Teardown: remove the config entry, then delete `/tmp/rollup-scratch`.

## Desired Behavior

1. `vault-cli config set-baseline <vault> <path>` writes a `baseline` key onto the named vault's config entry, alongside the existing per-vault keys (`tasks_dir`, `goals_dir`, and the rest). The verb reads the config file before it writes anything, so a malformed or unparseable config exits non-zero and leaves the file untouched. The verb accepts a vault name and a vault-relative path, exits non-zero naming the vault when the vault is absent from the config, and exits non-zero rejecting an absolute path or one that resolves outside the vault root. A failed write leaves the config file exactly as it was: the file holds either its previous content or the new content, never a partial merge. `vault-cli config` today exposes only `list` and `current-user`; this adds the third verb.
2. The baseline file is a markdown file inside the vault whose frontmatter carries the five figures and the capture date, under these frozen keys: `baseline_captured` (the capture date), `baseline_human_total`, `baseline_median`, `baseline_weeks` (a map from `YYYY-Wnn` to a figure), and `baseline_agent_coverage`. The keys are frozen because the file is authored by hand outside the code and read by the rollup; `docs/baseline-file.md` carries the contract, the analogue mapping and the mismatch table, and the example below is the contract, not a sample.

   ```yaml
   ---
   baseline_captured: 2026-09-12
   baseline_human_total: 62485
   baseline_median: 64
   baseline_weeks:
     2026-W36: 25141
     2026-W37: 26476
   baseline_agent_coverage: "1 of 420"
   ---
   ```
3. With a baseline configured, the report prints a baseline block after the week header and before the computed figures: a header line carrying the capture date, then one line per stored figure, indented two spaces beneath the header and rendered as `Label: value` — the label, a colon, one space, the value, and nothing else on the line, matching the existing rollup convention. The labels are `Human interactions (total)`, `Median`, `Week <YYYY-Wnn>`, and `Agent coverage`. Every value is echoed exactly as the file stores it: the rollup never recomputes, rounds, reformats or normalises a baseline figure.
4. With a baseline configured, the report prints a delta block as its **final** block, after the computed figures and the rule lines: a header line carrying the capture date, then one row per figure the rollup computes that has a baseline analogue, indented two spaces beneath the header. A row renders as `Label: <computed> - <baseline> = <delta>`. The analogue mapping is fixed: `Human interactions` maps to the baseline's week figure for the requested week when that week is a key in `baseline_weeks`; `Per-family median` maps to `baseline_median`. The `Per-family median` row carries the frozen marker `[definitional mismatch]`, because the stored figure is a per-task median and the computed figure is a median of per-family medians.
5. `Unattended deliveries` has no baseline analogue and prints no delta row. When the requested week is absent from `baseline_weeks`, the `Human interactions` delta row is omitted and nothing is printed in its place — never a row reading `0`.
6. A vault whose config entry carries no `baseline` key prints no baseline block and no delta row, and its output is byte-identical to the pre-change output. A vault whose `baseline` key is set but whose file is missing, unreadable, or whose frontmatter lacks a required key, exits non-zero naming the vault and the resolved path (or the missing key) and prints no figures and no partial baseline block — an unconfigured baseline and a misconfigured one are different states, and only the first is silent.
7. `--output json` carries the same baseline figures and the same deltas under a `baseline` key, preserving the plain/JSON parity spec 049 established: `.baseline.captured`, `.baseline.human_total`, `.baseline.median`, `.baseline.weeks` (the `YYYY-Wnn` map), `.baseline.agent_coverage`, and `.baseline.deltas.<figure>` objects each carrying `computed`, `baseline` and `delta`, with the median object additionally carrying `mismatch: true` — the JSON form of the frozen marker. A vault with no baseline configured carries no `baseline` key in its JSON output.

## Constraints

- **Spec 049's relation — three amendments, stated so a reader who finds both documents can reconcile them.** Spec 049's `## Non-goals` assigned the 2026-09-12 baseline to a separate task (this spec is that task) and its `## Assumptions` pinned task frontmatter as the only source (narrowed, not repealed — see this spec's `## Assumptions`). Two further sentences in 049 are amended here:
  - 049's `## Constraints` says "Read set is the resolved vault's configured `tasks_dir`, frontmatter only. No data file, no cache, no sidecar. … The command reads nothing else and writes nothing." **Amended:** the read set becomes `tasks_dir` frontmatter (the *computation* read set, unchanged and still exclusive) **plus** one config-named baseline file that is a *presentation* input outside the computation read set. The baseline contributes no datum to any computed figure — it is echoed text, and deleting it changes the output's presentation and never a computed number. "Writes nothing" is unamended and still exact.
  - 049's `## Security / Abuse` says "It takes no free-form input beyond a week token and a vault name, both validated before use." **Amended:** the baseline path is a third input, but it is not free-form user input — it arrives from the operator's own config file. `rollup weekly` never accepts it on the command line; only `config set-baseline <vault> <path>` takes it, as a positional argument, and validates it (an absolute path, or one whose cleaned form escapes the vault root, is rejected with a non-zero exit and no write) before it is persisted. The value the rollup later reads back from config is validated the same way: an absolute or escaping path makes `rollup weekly` exit non-zero and print no figures.
  - The argument is the same one this spec's `## Assumptions` already makes: the pin governs the *computation*, and the baseline is not a computational source. 049's constraints were written before a presentation input existed; they are widened by one read, not repealed.
- **Frozen block header strings.** The baseline block's header is the literal `Baseline (captured <YYYY-MM-DD>)` at column 0, where `<YYYY-MM-DD>` is the file's `baseline_captured`. The delta block's header is the literal `Delta (vs baseline captured <YYYY-MM-DD>)` at column 0. Both literals are frozen and are the only lines in the report that begin with `Baseline` and `Delta` respectively — the ACs grep `^Baseline` and `^Delta` to scope the two blocks.
- **Frozen report order when a baseline is configured:** week header, baseline block, computed figures (with the per-family lines), rule lines, delta block last. The delta block is the report's final block and runs from its header line to the end of the output, so `awk '/^Delta/&&!f{f=1}f'` over stdout yields the delta block and nothing else — which is what keeps "no delta row for `Unattended deliveries`" a scoped assertion rather than a substring accident. With no baseline configured, neither block is printed and the report is unchanged.
- **The rollup never writes.** No vault file, no baseline file, no config file is written by `rollup weekly`. Only `config set-baseline` writes, and it writes only the config file, atomically.
- **Backward compatibility is exact, not approximate.** With no `baseline` key the output is byte-identical to the pre-change output — not "equivalent", not "the same figures". The existing Ginkgo suite and the existing operator commands keep passing unchanged.
- **Config compatibility.** The `baseline` key is additive: an existing `config.yaml` with no `baseline` key parses and behaves exactly as before, and no existing per-vault key changes meaning.
- **Carrier: a vault file named by config.** The baseline lives in a vault markdown file whose path comes from the vault's `baseline` config key. `docs/baseline-file.md` carries the frontmatter contract, the analogue mapping and the mismatch table; that doc is the authority for the key set.
- **Rejected carrier — a designated task file inside `tasks_dir`.** It would sit inside the very population `rollup weekly` counts, so the rollup would count its own baseline: a task file carries `metrics_completed_at` and `metrics_interaction_count` and would join a week's task set and a family's member set, changing the figures it is meant to be compared against.
- **Rejected carrier — a hardcoded vault-relative path.** Configured vaults use `tasks`, `24 Tasks` and `25 Tasks` variously, and a vault-agnostic CLI cannot know a path that differs per vault; a fixed path would silently find nothing in most vaults and fail to distinguish "no baseline" from "baseline at the wrong path".
- **The five figures are stored text.** None is derived or recomputed. This is what keeps the recorded definitional mismatches legible: the rollup shows `26,476` next to `39,055` and lets the reader see the capture-window gap, rather than quietly normalising one to the other.
- **Frozen marker text.** `[definitional mismatch]` is the literal string the median delta row carries; ACs grep it.
- **`pkg/ops/` never writes to stdout.** The operation returns a structured result; the CLI layer owns formatting. See `docs/development-patterns.md`.
- **`--output plain` is the default; `--output json` uses the repo's existing JSON printer.** No `encoding/json` import in a command file.
- **Dates come from the injected clock**, never `time.Now()`. The capture date is read from the file, never computed.
- **Do NOT commit** — dark-factory handles git.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---|---|---|---|---|
| The config file is malformed or unparseable when `config set-baseline` runs | Exit non-zero with the parse error; nothing is written | Operator fixes the YAML | Non-zero exit; `shasum` of the config file unchanged | Reversible — no write occurred |
| The config write fails partway (disk full, permission denied, interrupted) | Exit non-zero; the config file holds either its previous content or the new content, never a truncated or merged file | Operator frees space or fixes permissions and re-runs the verb | Non-zero exit; `config list` still parses; `shasum` matches the pre-call value | Reversible — no write landed |
| `baseline` key set, file missing or unreadable | Exit non-zero naming the vault and the resolved path; no figures, no partial block | Operator creates the file or runs `config set-baseline` with the right path | Non-zero exit and the path in stderr | Reversible — config edit |
| `baseline` key set, file present, a required frontmatter key absent | Exit non-zero naming the missing key; no partial block, never a rendered `0` | Operator adds the key to the file | Non-zero exit and the key name in stderr | Reversible — file edit |
| A baseline figure is non-numeric or malformed | Exit non-zero naming the file and key; never coerced into a number, never a panic | Operator fixes the value | Non-zero exit | Reversible — file edit |
| Requested week absent from `baseline_weeks` | Baseline block prints; `Human interactions` delta row omitted; exit 0 | None required — a legitimate week with no stored analogue | The absent row in stdout | Not applicable |
| `config set-baseline` names a vault absent from the config | Exit non-zero naming the vault; config file unchanged | Operator checks `config list --output json` | Non-zero exit; `shasum` of config unchanged | Reversible — no write occurred |
| `config set-baseline` given an absolute path or one escaping the vault root | Exit non-zero naming the rejected path; config file unchanged | Operator passes a vault-relative path | Non-zero exit; `shasum` of config unchanged | Reversible — no write occurred |
| Two `config set-baseline` calls race on the config file | One wins; the file is never interleaved or corrupt | Operator re-runs with the intended path | `config list` reads back a single coherent path | Reversible — re-run |
| Clock skew / timezone | No effect — the capture date is stored text echoed verbatim and no baseline figure is date-arithmetic | None required | Output matches the file byte for byte | Not applicable |

## Security / Abuse

- **Untrusted input is the config-supplied path and the baseline file's content.** The path arrives from the operator's own config file and is validated before use: an absolute path, or one whose cleaned form escapes the vault root, is rejected with a non-zero exit and no write. The rollup resolves the path against the vault root only.
- **The baseline file is read-only to the tool.** No code path in `rollup weekly` writes to it; the negative ACs assert this against the real vault.
- **Frontmatter parsing, never line-grepping.** A hostile or malformed value must be treated as an error naming the key, never parsed into a number and never able to panic the scan. This matches spec 049's existing constraint on the task read path.
- **No network, no credentials, no external system.** The command reads two files under one vault and prints; it opens no connection and needs no secret.
- **Nothing hangs or retries.** Both reads are local filesystem reads with no retry loop; a failure exits rather than spinning.

## Suggested Decomposition

**Size budget — the spec stays whole.** It carries 7 Desired Behaviors and 12 Acceptance Criteria, so DB × AC = 84, above the 50 threshold that normally signals a split. It stays whole deliberately: the config verb and the rollup read are one pipeline — the verb is useless unread and the read is untestable unset, so a split would produce two specs that each ship half a feature and neither of which could tick a single operator-rung AC on its own. The decomposition below is the split that matters, and it is a prompt-level split, not a spec-level one.

Generate the prompts in this order.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `config set-baseline` verb: config key, read-before-write, path validation, atomic persistence, plus its unit suite | 1 | — | — |
| 2 | Baseline file parse (frozen keys, error states), analogue mapping, delta computation, JSON shape — with its Ginkgo fixture suite | 2, 4, 7 | 2 | — |
| 3 | CLI wiring: baseline and delta block rendering in the frozen order, the no-baseline byte-identical path, error surfacing | 3, 5, 6 | 1 | prompts 1, 2 |

Rationale: prompt 1 is the config-side contract and prompt 2 the ops-side computation; neither depends on the other, so they can run in parallel, and prompt 2's fixture suite carries the anti-hardcode and arithmetic cases without needing a configured vault. Prompt 3 is the user-visible surface and needs both. **Every AC except AC 1 (`make precommit`) and AC 2 (`make test`) is operator-only and is assigned to no prompt** — per spec 049's convention. Those ACs need a configured vault, the scratch vault copy and the real seeded baseline file, none of which exist inside the container; a test-only prompt could lock fixtures but could not tick them.

## Do-Nothing Option

`rollup weekly` keeps printing a week's three figures with nothing to compare them against. The 2026-09-12 baseline stays in a task page, readable by a human who remembers it and invisible to the tool, and the objective's north star — human time per recurring task going down — stays uncomputable for the same reason it was before spec 049: not because the machine cannot measure, but because it has no fixed point to measure from. The cost is not the missing block; it is that every future rollup run is read as an absolute number and every graduation decision continues to be made on a figure whose direction is a matter of memory.
