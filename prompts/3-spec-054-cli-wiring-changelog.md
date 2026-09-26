---
spec: [054-rollup-weekly-baseline-delta]
status: draft
created: "2026-09-26T15:48:55Z"
---

# The baseline and delta blocks in `rollup weekly`, and the shipped changelog entry (spec 054, prompt 3 of 3)

<summary>
- `rollup weekly` prints the vault's baseline figures directly under the week header, before the figures it computes.
- The delta block prints last, after the rule lines, with one row per computed figure that has a baseline analogue.
- The per-family median's delta row carries the frozen `[definitional mismatch]` marker, because the stored and computed medians measure different things.
- The unattended-delivery count never gets a delta row, and a week the baseline does not carry leaves the human-interactions row out entirely — never a row reading zero.
- A vault whose config names no baseline prints exactly the report it printed before this feature: no extra line, byte for byte.
- A vault whose baseline is configured but broken fails loudly and prints nothing at all — no figures, no partial block.
- The JSON output carries the same stored figures and the same deltas under one `baseline` key, and carries no such key when the vault has no baseline.
- The README's `### rollup` section describes the two blocks and points at the baseline contract document.
- One `## Unreleased` changelog bullet describes the shipped feature, and no version string is bumped.
</summary>

<objective>
Render the baseline and the deltas in `rollup weekly`: wire the config key prompt 1 landed into the rollup operation prompt 2 built, print the two blocks in the frozen order, and keep the no-baseline report byte-identical to the pre-change output. This is spec 054's third and final prompt: it covers Desired Behaviors 3, 5 and 6 and Acceptance Criteria 1, 2, 3, 4 and 5, and it depends on prompts 1 and 2 — both must already be in the tree.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/development-patterns.md` (`## Output Format`, `## Multi-Vault Pattern`) and `docs/dod.md` (the Definition of Done — it names the changelog, the README and the integration command-registration table explicitly).

Read these files fully before making changes:

- `pkg/cli/rollup.go` (139 lines) — the file you extend. `createRollupCommands` and `createRollupWeeklyCommand`, whose `RunE` resolves exactly one vault via `(*configLoader).GetVault(ctx, *vaultName)`, builds `storage.NewConfigFromVault(vault)`, `storage.NewTaskStorage(...)`, `ops.NewRollupWeeklyOperation(taskStore, libtime.NewCurrentDateTime())`, calls `rollupOp.Execute(ctx, vault.Path, vault.Name, week)` and prints `PrintJSON(result)` or `fmt.Print(formatRollupWeeklyPlain(result))`; `formatRollupWeeklyPlain(result ops.RollupWeeklyResult) string`, which is pure and calls `writeRollupWeeklyHeader`, `writeRollupWeeklyFigures`, `writeRollupWeeklyFamilies`, `writeRollupWeeklyRules` in that order; the frozen constants `unattendedRuleSentence` and `groupingRuleSentence`. The file imports `context`, `fmt`, `strings`, `github.com/bborbe/errors`, `libtime "github.com/bborbe/time"`, `github.com/spf13/cobra`, `pkg/config`, `pkg/ops`, `pkg/storage`.
- `pkg/cli/rollup_test.go` — the formatter's existing specs. Its first spec asserts the WHOLE rendered string with a single `Equal`, which is the pre-change byte-identity guard; keep it passing unchanged.
- `pkg/cli/export_test.go` — `FormatRollupWeeklyPlainForTest(result ops.RollupWeeklyResult) string` already exists; do not change it.
- `pkg/cli/cli_suite_test.go` — the `cli_test` suite entry point. **One `func TestSuite`; do not add a second `func Test*` calling `RunSpecs`.**
- `pkg/cli/output.go` — `OutputFormat`, `IsJSON()`, `PrintJSON(v any) error`.
- `pkg/cli/cli.go` `createRollupCommands`'s registration line and `NewRootCommand` — read only, do not change.
- `pkg/config/config.go` — **prompt 1's change.** `type Vault struct` now ends with `Baseline string \`yaml:"baseline,omitempty" json:"baseline,omitempty"\``. `GetVault` returns the vault with `Baseline` populated. If the field is absent when you start, stop and report `"status":"failed"` naming the missing field — do not add it here.
- `pkg/config/baseline.go` — **prompt 1's change.** `func ValidateBaselinePath(ctx context.Context, baselinePath string) error` rejects an empty path, an absolute path, and any path whose cleaned form escapes the vault root, naming the rejected path. This is the one rule that guards both sides of the feature: prompt 1's `config set-baseline` calls it at write time, and this prompt calls it at read time before the rollup is constructed. Call it — do not re-implement it. If the file is absent when you start, stop and report `"status":"failed"` naming it.
- `pkg/ops/baseline.go` — **prompt 2's change.** `RollupBaseline` (`Captured string`, `HumanTotal int`, `Median int`, `Weeks map[string]int`, `AgentCoverage string`, `Deltas RollupBaselineDeltas`), `RollupBaselineDeltas` (`HumanInteractions *RollupBaselineDelta`, `PerFamilyMedian *RollupBaselineDelta`), `RollupBaselineDelta` (`Computed float64`, `Baseline float64`, `Delta float64`, `Mismatch bool`). If the file is absent when you start, stop and report `"status":"failed"` naming it.
- `pkg/ops/rollup_weekly.go` — **prompt 2's change.** `RollupWeeklyResult` now ends with `Baseline *RollupBaseline \`json:"baseline,omitempty"\``, and `NewRollupWeeklyOperationWithBaseline(taskStorage storage.TaskStorage, currentDateTime libtime.CurrentDateTime, baselinePath string) RollupWeeklyOperation` exists alongside the unchanged `NewRollupWeeklyOperation`. The `Execute` signature is still `(ctx, vaultPath, vaultName, week string)`.
- `integration/integration_suite_test.go` — `package integration_test`, `func TestIntegration`, and `var binPath string` built once in `BeforeSuite` with `gexec.Build("github.com/bborbe/vault-cli")`.
- `integration/cli_test.go` — read `createTempVaultWithTopics(topicsDir string) (vaultPath string, configPath string, cleanup func())` (lines ~135-166) as the model for a config fixture that sets one extra per-vault key, and read the `gexec.Start` + `Eventually(session).Should(gexec.Exit(0))` + `gbytes` spec style. Do NOT modify this file.
- `docs/baseline-file.md` — the frozen report layout and the frozen header literals you implement: `Baseline (captured <YYYY-MM-DD>)` at column 0, `Delta (vs baseline captured <YYYY-MM-DD>)` at column 0, the block order, and the "no delta row" rules.
- `specs/in-progress/054-rollup-weekly-baseline-delta.md` — the spec. Read Desired Behaviors 3, 5, 6 and 7, all of the Constraints, the Failure Modes table and the Security section.
- `README.md` § Usage `### rollup` — the fenced `bash` block and the two paragraphs after it.
- `CHANGELOG.md` — the `# Changelog` title, the `All notable changes…` preamble, the `* MAJOR / MINOR / PATCH` lines, then `## Unreleased` (which already holds one `- docs:` bullet) and `## v0.148.0`.
- `scripts/check-changelog.sh` — the structural check `make check-changelog` runs: the preamble must precede every `## ` section.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format and the `## Unreleased` placement rule.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits (`funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20) and license headers.

Three environment facts:

1. **Make no git calls anywhere, including in `<verification>`.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. Every check below is git-free.
2. **`integration/` is part of `make test`.** `go test ./...` builds the real binary through `gexec.Build` and runs it as a subprocess, so the stdout assertions below are the container-executable form of the spec's operator-rung evidence. `GOFLAGS=-buildvcs=false` is already set in `.dark-factory.yaml` — keep it, or `gexec.Build` can fail reading VCS build info.
3. **`integration/cli_test.go` is already 4007 lines.** Do NOT grow it and do NOT refactor it. Put your fixture helper and your specs in a NEW file `integration/rollup_baseline_test.go` in the same `package integration_test`; `binPath` and the `TestIntegration` entry point are package-scoped and reachable from it. Flag the existing file's size in your completion report's `## Improvements` section.
</context>

<requirements>

## 0. Scope — exactly five files, no others

- `pkg/cli/rollup.go` — modify: the constructor swap, two new writers, one number formatter, one frozen marker constant.
- `pkg/cli/rollup_test.go` — modify: the new formatter specs.
- `integration/rollup_baseline_test.go` — new: the fixture helper and the real-binary specs.
- `README.md` — modify: the `### rollup` section.
- `CHANGELOG.md` — modify: one bullet appended under the existing `## Unreleased`.

Nothing else. Do NOT touch `pkg/cli/cli.go`, `pkg/cli/export_test.go`, `pkg/config/`, `pkg/ops/`, `pkg/storage/`, `integration/cli_test.go`, or `docs/baseline-file.md`. Do NOT add a scenario file (`ls scenarios/*.md | wc -l` must still print `6`) — the spec's Non-goals rule one out explicitly.

## 1. `pkg/cli/rollup.go` — wire the baseline path in

In `createRollupWeeklyCommand`'s `RunE`, insert the read-time path validation and replace exactly one line:

```go
if vault.Baseline != "" {
	if err := config.ValidateBaselinePath(ctx, vault.Baseline); err != nil {
		return errors.Wrapf(ctx, err, "baseline of vault %s", vault.Name)
	}
}
// old
rollupOp := ops.NewRollupWeeklyOperation(taskStore, libtime.NewCurrentDateTime())
// new
rollupOp := ops.NewRollupWeeklyOperationWithBaseline(
	taskStore,
	libtime.NewCurrentDateTime(),
	vault.Baseline,
)
```

- `config.ValidateBaselinePath` is the shared rule prompt 1 wrote; calling it here is what makes a hand-edited `baseline` of `/etc/passwd` or `../../outside.md` exit non-zero with no figures and no partial block, exactly as `config set-baseline` refuses the same path at write time. The spec's Constraints require the same validation in both places. `pkg/cli/rollup.go` already imports `github.com/bborbe/vault-cli/pkg/config`, so no import line changes.
- The validation runs BEFORE the operation is constructed, so a rejected path never reaches a file read. Do not re-implement the rule, do not clean or normalise the path, and do not add a second validation inside the operation.
- `vault.Baseline` is the raw config value: the empty string when the vault names no baseline, in which case neither the validation nor the operation's baseline read runs and the report is byte-identical to the pre-change output.
- Do not add a flag, and do not change the `PrintJSON` / `formatRollupWeeklyPlain` split or the error propagation: the returned error already names the vault, and the root command's `Execute` prints it to stderr and exits 1.

## 2. `pkg/cli/rollup.go` — the two blocks, in the frozen order

`formatRollupWeeklyPlain` becomes:

```go
func formatRollupWeeklyPlain(result ops.RollupWeeklyResult) string {
	var builder strings.Builder
	writeRollupWeeklyHeader(&builder, result)
	writeRollupWeeklyBaseline(&builder, result)
	writeRollupWeeklyFigures(&builder, result)
	writeRollupWeeklyFamilies(&builder, result.Families)
	writeRollupWeeklyRules(&builder)
	writeRollupWeeklyDeltas(&builder, result)
	return builder.String()
}
```

The function stays **pure**: it builds and returns text, and the caller prints. No `fmt.Print*`, no `os.Stdout`, no `cmd.OutOrStdout()` inside it or inside the writers.

### 2a. The baseline block

```go
// writeRollupWeeklyBaseline writes the stored baseline figures directly beneath
// the week header and above the computed figures. A result with no baseline
// writes nothing at all, which is what keeps the no-baseline report identical to
// the pre-baseline report. Every value is echoed exactly as the file stores it:
// nothing here recomputes, rounds or reformats a baseline figure.
func writeRollupWeeklyBaseline(builder *strings.Builder, result ops.RollupWeeklyResult)
```

- `if result.Baseline == nil { return }` first.
- Header at column 0: `fmt.Fprintf(builder, "Baseline (captured %s)\n", result.Baseline.Captured)`.
- Then, each indented by **exactly two spaces**, in this fixed order, each `Label: value` with nothing else on the line:
  - `fmt.Fprintf(builder, "  Human interactions (total): %d\n", result.Baseline.HumanTotal)`
  - `fmt.Fprintf(builder, "  Median: %d\n", result.Baseline.Median)`
  - one line per entry of `result.Baseline.Weeks`, rendered `fmt.Fprintf(builder, "  Week %s: %d\n", token, value)`, **sorted ascending by the week token**. `Weeks` is a Go map and iterating it directly makes the report non-deterministic; copy the keys into a `[]string`, `sort.Strings` them, then iterate. The `YYYY-Wnn` keys are zero-padded, so a lexicographic sort is also a chronological one.
  - `fmt.Fprintf(builder, "  Agent coverage: %s\n", result.Baseline.AgentCoverage)` — a string, echoed verbatim, never parsed.

### 2b. The delta block

```go
// writeRollupWeeklyDeltas writes the delta block as the report's final block: one
// row per computed figure that has a baseline analogue. A figure with no analogue
// has no row, and a row whose pointer is nil is omitted entirely rather than
// rendered as a zero. A result with no baseline writes nothing.
func writeRollupWeeklyDeltas(builder *strings.Builder, result ops.RollupWeeklyResult)
```

- `if result.Baseline == nil { return }` first.
- Header at column 0: `fmt.Fprintf(builder, "Delta (vs baseline captured %s)\n", result.Baseline.Captured)`.
- Then, in this fixed order and each indented by exactly two spaces:
  - `if delta := result.Baseline.Deltas.HumanInteractions; delta != nil { fmt.Fprintf(builder, "  Human interactions: %s - %s = %s\n", formatRollupNumber(delta.Computed), formatRollupNumber(delta.Baseline), formatRollupNumber(delta.Delta)) }`
  - `if delta := result.Baseline.Deltas.PerFamilyMedian; delta != nil { … }` with the same `Label: <computed> - <baseline> = <delta>` shape, and when `delta.Mismatch` is true, the frozen marker appended to the same line after one space:
    `fmt.Fprintf(builder, "  Per-family median: %s - %s = %s %s\n", formatRollupNumber(delta.Computed), formatRollupNumber(delta.Baseline), formatRollupNumber(delta.Delta), rollupDefinitionalMismatchMarker)`
- The marker is a frozen literal, single-sourced from a constant in this file, next to `unattendedRuleSentence` and `groupingRuleSentence`:
<!-- OPEN QUESTION: the spec and docs/baseline-file.md pin the marker's text and that the median delta row carries it, but not its position on the line. This prompt commits to appending it after one space at the end of the row, so the row's `Per-family median: <computed> - <baseline> = <delta>` core stays a prefix of the line — which is what keeps Acceptance Criterion 4's substring assertion on that core true. A reviewer who prefers the marker elsewhere (before the colon, or in its own column) should say so at audit time. -->

  ```go
  // rollupDefinitionalMismatchMarker is the frozen marker the per-family median's
  // delta row carries: the stored figure is a per-task median and the computed one
  // is a median of per-family medians. It is a frozen literal — rewording it is a
  // behaviour change, not a style choice — and it is the plain-report form of the
  // JSON's `mismatch: true`.
  const rollupDefinitionalMismatchMarker = "[definitional mismatch]"
  ```
- `Unattended deliveries` has no analogue: do not add a row for it, and do not iterate anything to produce rows. The two rows are written by name, in the order above.
- Nothing is written in place of an omitted row — no placeholder, no `0`, no blank line.
<!-- OPEN QUESTION: when a baseline is configured but no delta row is emitted at all (every computed figure is a sentinel), this prompt still prints the delta header line, on the reading that DB 4 makes the delta block unconditional once a baseline is configured. A reviewer who prefers suppressing the header too should say so at audit time; the acceptance criteria never exercise that state. -->

### 2c. The number formatter

```go
// formatRollupNumber renders a delta value the same way the computed figures are
// rendered elsewhere: the shortest decimal form, so an integer prints without a
// decimal point and a fractional median prints as it was computed. Using one
// formatter for the computed value, the stored value and the delta is what keeps
// the row's `<computed>` equal to the figure the same run prints above it.
func formatRollupNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
```

Add `strconv` and `sort` to the file's imports as needed.

### 2d. The report's exact shape

For a result whose week is `2026-W37`, whose figures are `39055` / `12` / `113`, whose families are `check prometheus alerts: 52` and `start day: undefined`, and whose baseline is the real 2026-09-12 capture, `formatRollupWeeklyPlain` returns exactly:

```
Week: 2026-W37 (2026-09-07 to 2026-09-13)
Baseline (captured 2026-09-12)
  Human interactions (total): 62485
  Median: 64
  Week 2026-W36: 25141
  Week 2026-W37: 26476
  Agent coverage: 1 of 420
Human interactions: 39055
Unattended deliveries: 12
Per-family median: 113
  check prometheus alerts: 52
  start day: undefined
Unattended rule: A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0
Grouping rule: A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively.
Delta (vs baseline captured 2026-09-12)
  Human interactions: 39055 - 26476 = 12579
  Per-family median: 113 - 64 = 49 [definitional mismatch]
```

Every line ends with `\n`, including the last. `Baseline (captured …)` and `Delta (vs baseline captured …)` are the ONLY lines in the whole report that begin with `Baseline` and `Delta` respectively — the acceptance criteria scope the two blocks by grepping `^Baseline` and `^Delta`, so no other line may start with either word, at any indentation, in any output. The delta block runs from its header to the end of the output, which is what makes "no delta row for `Unattended deliveries`" a scoped assertion rather than a substring accident.

## 3. `pkg/cli/rollup_test.go` — the formatter's new specs

Extend the existing file. **One `func TestSuite` in `pkg/cli/cli_suite_test.go`; do not add a second `func Test*` calling `RunSpecs`.** Do not change or weaken the three existing specs — the first one asserts the whole pre-change string with a single `Equal` and is the no-baseline byte-identity guard.

Use these spec names verbatim; they are grep targets in `<verification>`:

1. `"renders the baseline block between the week header and the computed figures"` — a fully-populated result with `Baseline` set, asserted with a single `Equal` against the whole expected string from section 2d (a multi-line raw string literal). This one spec pins the block order, the two-space indent, the sorted week lines, the column-0 header literals and the trailing newline. Do not weaken it to a set of `ContainSubstring` calls.
2. `"renders the delta block last with the frozen marker on the median row"` — same result; assert the delta header line, `  Human interactions: 39055 - 26476 = 12579`, and `  Per-family median: 113 - 64 = 49 [definitional mismatch]` with the marker literal written out in the expected string (not read from the constant — a reworded constant must fail this spec).
3. `"renders no baseline and no delta block when the result carries no baseline"` — the same result with `Baseline: nil`; assert the rendered text matches neither `(?m)^Baseline` nor `(?m)^Delta` (Gomega's `MatchNot` with a regexp), and assert it ends with the grouping-rule line followed by `\n`.
4. `"omits the human-interactions row when the week has no stored figure"` — `Baseline` set with `Deltas.HumanInteractions == nil` and `Deltas.PerFamilyMedian` non-nil; assert the rendered text contains `Delta (vs baseline captured` and the median row, and does NOT contain a line matching `(?m)^  Human interactions: ` — the delta row's label, which is distinct from the computed figure's `^Human interactions: ` line and from the baseline block's `^  Human interactions (total): ` line.

## 4. `integration/rollup_baseline_test.go` — the real-binary specs

New file, `package integration_test`, BSD license header matching its siblings. Do NOT declare a second `TestSuite`/`TestIntegration` — `integration/integration_suite_test.go` owns the only one.

Two helpers:

```go
// createTempVaultWithBaseline creates a temporary vault whose config names
// baselineFile as the vault's baseline, writes baselineContent at that
// vault-relative path, and returns the vault path, the config path and a cleanup
// func. Modelled on createTempVaultWithTopics in integration/cli_test.go.
func createTempVaultWithBaseline(
	tasks map[string]string,
	baselineFile string,
	baselineContent string,
) (vaultPath string, configPath string, cleanup func())

// baselineDocument renders a complete baseline markdown file: the five frozen
// frontmatter keys and a markdown body. weeks is the pre-rendered body of
// baseline_weeks, e.g. "  2026-W36: 25141\n  2026-W37: 26476\n".
func baselineDocument(
	captured string, humanTotal string, median string, weeks string, agentCoverage string,
) string
```

`createTempVaultWithBaseline` writes `Tasks/` task files, then a config of the shape `createTempVaultWithTopics` writes with one extra line — `    baseline: %s` — under the `test` vault, with `baselineFile` interpolated. When `baselineFile` is empty, omit the `baseline:` line entirely (that is the no-baseline fixture). When `baselineContent` is empty, write no file at all — the config still names `baselineFile`, which is the missing-file fixture integration spec 5 needs.

Fixtures:

```go
const (
	realBaselineWeeks = "  2026-W36: 25141\n  2026-W37: 26476\n"
	scratchBaselineWeeks = "  2026-W36: 222\n  2026-W37: 333\n"
)
```
- real figures: `baselineDocument("2026-09-12", "62485", "64", realBaselineWeeks, "1 of 420")`
- distinct figures: `baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9")`

Use these spec names verbatim; they are grep targets in `<verification>`. Every spec starts the real binary through `gexec.Start` with `binPath` and calls the fixture's `cleanup` via `defer`; assert exit 0 with `Eventually(session).Should(gexec.Exit(0))` and read stdout with `string(session.Out.Contents())`.

1. `"prints the baseline block from the configured file"` — the real-figures fixture with a `2026-W37` task carrying a recorded count. Assert stdout contains `Baseline (captured 2026-09-12)`, `  Human interactions (total): 62485`, `  Median: 64`, `  Week 2026-W36: 25141`, `  Week 2026-W37: 26476` and `  Agent coverage: 1 of 420`. This is Acceptance Criterion 2's first half: the block is rendered from the configured file, not compiled in.
2. `"prints the fixture's own figures and not the real ones"` — the distinct-figures fixture, same week. Assert stdout contains `Baseline (captured 2026-01-02)`, `  Human interactions (total): 111`, `  Median: 7`, `  Week 2026-W36: 222`, `  Week 2026-W37: 333`, `  Agent coverage: 2 of 9`, and assert it contains none of `62485`, `25141`, `26476`. A hardcoded Go constant fails this spec.
3. `"prints the delta block last with one row per analogue"` — the distinct-figures fixture (`2026-W37` = `333`, median `7`). Assert stdout contains `Delta (vs baseline captured 2026-01-02)`, a line matching `^  Human interactions: <computed> - 333 = <computed minus 333>` where `<computed>` is read from the run's own `^Human interactions: ` line, and `  Per-family median: <computed> - 7 = <computed minus 7> [definitional mismatch]` built the same way. Then assert the block order with `strings.Index`: the baseline header's index is less than the index of `"\nHuman interactions: "`, which is less than the delta header's index. Finally, scope the no-analogue assertion: take `stdout[strings.Index(stdout, "Delta (vs baseline captured"):]` and assert that tail contains no `Unattended deliveries`. Asserting over the tail, not the whole output, is the point — the whole output legitimately carries the unattended figure above the delta block.
4. `"prints no baseline and no delta block when no baseline is configured"` — a fixture built with an empty `baselineFile`. Assert exit 0, that stdout contains the three computed figure labels, and that stdout contains neither `Baseline (` nor `Delta (` anywhere.
5. `"fails loudly when the baseline file is missing"` — a fixture whose `baseline` names a vault-relative path with no file at it. Assert the process exits non-zero, stderr contains the vault name and the resolved absolute path, and stdout contains no `Baseline`.
6. `"fails loudly when a baseline key is missing"` — a baseline file whose `baseline_median` line is removed. Assert the process exits non-zero, stderr contains `baseline_median`, and stdout contains neither `Baseline` nor `Median`.
7. `"rejects a baseline path outside the vault"` — two cases in one spec: `baseline` set to `/etc/passwd`, then to `../../outside.md`. Each exits non-zero, and each run's stdout contains none of `Human interactions`, `Baseline`, `Delta` — no figures and no partial block.
8. `"carries the baseline figures and the deltas in JSON"` — the real-figures fixture run with `--output json`. Unmarshal stdout into a small local struct and assert `baseline.captured == "2026-09-12"`, `human_total == 62485`, `median == 64`, `agent_coverage == "1 of 420"`, `weeks["2026-W37"] == 26476`, `deltas.human_interactions.baseline == 26476` and `deltas.human_interactions.delta == deltas.human_interactions.computed - deltas.human_interactions.baseline`, and `deltas.per_family_median.mismatch == true`. Then run the no-baseline fixture with `--output json`, unmarshal into a `map[string]any`, and assert `_, ok := m["baseline"]; Expect(ok).To(BeFalse())`.
9. `"prints the edited baseline figure after the file is rewritten"` — Acceptance Criterion 3's state transition: the block tracks the file's content across an edit. Record `sha256OfFile(binPath)` before the first run. Using the distinct-figures fixture, run the binary and assert stdout contains `  Median: 7`. Then rewrite that fixture's baseline file in place — same path, regenerated with `baselineDocument("2026-01-02", "111", "8", scratchBaselineWeeks, "2 of 9")` written to `filepath.Join(vaultPath, baselineFile)` — and run the *same* binary again. Assert the second run's stdout contains `  Median: 8`, does NOT contain `  Median: 7`, and that its median delta row matches `  Per-family median: <computed> - 8 = <computed minus 8> [definitional mismatch]`, where `<computed>` is read from the second run's own `^Per-family median: ` line. Then restore the file (write the original `baselineDocument("2026-01-02", "111", "7", scratchBaselineWeeks, "2 of 9")` back) and run the binary a third time; assert stdout contains `  Median: 7` again. Record `sha256OfFile(binPath)` again after the third run and assert the two digests are equal — three runs, one binary. Declare your own local `sha256OfFile(path string) string` closure, of the same shape the existing integration suite uses inline (`os.ReadFile` + `sha256.Sum256` + `fmt.Sprintf("%x", sum)`, e.g. `integration/cli_test.go:2319`) — there it is a per-spec closure, not a package-level helper, so a new file cannot call it. Do not shell out to `shasum`, which the container image may not carry.

`encoding/json` in a `_test.go` file is fine; the "never import `encoding/json` in a command file" rule applies to `pkg/cli/*.go` non-test files.

## 5. `README.md` — the `### rollup` section

After the existing two paragraphs, add one short paragraph stating: that a vault whose config entry names a `baseline` file has that file's stored figures printed above the computed ones under a `Baseline (captured <date>)` header, and the movement from each computed figure to its baseline analogue printed last under a `Delta (vs baseline captured <date>)` header; that the figures are echoed exactly as the file stores them and are never recomputed or rewritten; and that a vault with no baseline configured prints exactly what it printed before. Point at `docs/baseline-file.md` for the frontmatter contract and the `vault-cli config set-baseline <vault> <path>` verb that records the path. Do not reword the existing paragraphs, do not document a flag the command does not have, and do not document a baseline-writing or baseline-refreshing verb — the spec's Non-goals forbid both.

## 6. `CHANGELOG.md` — one bullet under the existing `## Unreleased`

`## Unreleased` already exists directly below the preamble and already holds one `- docs:` bullet for `docs/baseline-file.md`. **Append** one `- feat:` bullet to that section; do not create a second section and do not move the existing bullet.

```
- feat: `vault-cli rollup weekly` prints a vault's recorded baseline figures above the figures it computes and the delta from each computed figure to its baseline analogue below them, and `vault-cli config set-baseline <vault> <path>` records the vault-relative path of the baseline file the rollup reads. The five stored figures are echoed verbatim — never recomputed, normalised or rewritten — the per-family median's delta row carries `[definitional mismatch]` because the stored figure is a per-task median while the computed one is a median of per-family medians, and a vault whose config names no baseline prints exactly what it printed before.
```

- One bullet, starting `- feat:`, on a single line, naming `rollup weekly` and `config set-baseline`.
- Do NOT bump any version: not `CHANGELOG.md`'s newest version heading, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. `.maintainer.yaml` sets `release.autoRelease: true`, and the post-merge releaser converts `## Unreleased` into a versioned section and tags it — a hand-bump races it, and the spec's Constraints say so.
- `make precommit` runs `scripts/check-changelog.sh`, which fails if any `## ` section lands above the preamble. Keep the section below the preamble.

## 7. Failure modes and security — what each row carries

Map the spec's Failure Modes table onto this change and state the mapping in your completion report:

- **`baseline` key set, file missing or unreadable** → `Execute` returns the error before any figure exists, the `RunE` propagates it, `Execute` prints `Error: …` to stderr and exits 1, and stdout carries no `Baseline`. Covered by integration spec 5.
- **`baseline` key set, file present, a required frontmatter key absent** → same path, with the key name in stderr. Covered by integration spec 6.
- **A baseline figure is non-numeric or malformed** → prompt 2's reader errors naming the key; nothing is coerced and nothing is rendered. Nothing to build here; do not add a defensive branch in the formatter.
- **Requested week absent from `baseline_weeks`** → the baseline block prints and the human-interactions delta row is omitted, with nothing in its place; exit 0. Covered by unit spec 4 and by prompt 2's spec 5.
- **`config set-baseline` names a vault absent from the config, or is given an absolute or escaping path** → prompt 1's verb at write time, and `config.ValidateBaselinePath` at read time here, before the operation is constructed; the run exits non-zero with no figures. Covered by integration spec 7.
- **A vault with no `baseline` key** → neither block prints and the report is byte-identical to the pre-change output. Covered by the existing whole-string unit spec plus unit spec 3 and integration spec 4.
- **Clock skew / timezone** → no effect; the capture date is stored text echoed verbatim and no baseline figure is date-arithmetic. The formatter reads no clock.
- **Two `config set-baseline` calls race** → operator-side, prompt 1's concern.
- **A probe mutates a live vault's baseline** → impossible by construction; the specs run against throwaway temp vaults. The spec's operator-rung probes run against a scratch vault copy, not the real one.

Security: the formatter prints values that prompt 2's parser already validated, and it builds no path and reads no file. The command reads two files under one vault — the configured `tasks_dir` and the config-named baseline file — and writes nothing; after a run the vault tree must be byte-identical to what it was before. Do not add a shell invocation, a network call, an `os/exec`, or a path built from user input. Do not add escaping or sanitisation to the formatter: the values it prints are frontmatter the storage layer parsed and a path the config validated.

## 8. Self-check before finishing

- Re-read the changed hunks and confirm: `formatRollupWeeklyPlain` calls the five writers in the frozen order; both new writers early-return on a nil `Baseline`; the week lines are sorted; no branch substitutes a `0` for a missing figure; `pkg/cli/rollup.go` contains no `encoding/json` and no `fmt.Print*` inside the formatter; `integration/cli_test.go`, `pkg/ops/`, `pkg/config/` and `pkg/storage/` are untouched.
- Walk the spec's Acceptance Criteria 1, 2, 3, 4 and 5 and state in your completion report which requirement and which spec satisfies each one, and which spec covers each row of the Failure Modes table (section 7 is the mapping). Note explicitly that AC 5's byte-identity comparison against a pre-change binary is operator-only, and that its container-executable half is the nil-baseline unit spec plus integration spec 4.
- Walk `docs/dod.md`: exported identifiers have doc comments, no `fmt.Print*` was added to `pkg/ops/`, tests use Ginkgo v2 / Gomega, the README documents the new behaviour, the changelog entry sits under `## Unreleased` below the preamble, and the integration command-registration table still carries `config set-baseline` (prompt 1's row).
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 054 — frozen block header strings.** The baseline block's header is the literal `Baseline (captured <YYYY-MM-DD>)` at column 0, where `<YYYY-MM-DD>` is the file's `baseline_captured`. The delta block's header is the literal `Delta (vs baseline captured <YYYY-MM-DD>)` at column 0. Both literals are frozen and are the only lines in the report that begin with `Baseline` and `Delta` respectively.
- **Copied from spec 054 — frozen report order when a baseline is configured:** week header, baseline block, computed figures (with the per-family lines), rule lines, delta block last. The delta block is the report's final block and runs from its header line to the end of the output. With no baseline configured, neither block is printed and the report is unchanged.
- **Copied from spec 054 — frozen marker text.** `[definitional mismatch]` is the literal string the median delta row carries.
- **Copied from spec 054 — every baseline value is echoed exactly as the file stores it.** The rollup never recomputes, rounds, reformats or normalises a baseline figure. This is what keeps the recorded definitional mismatches legible: the report shows `26,476` next to the computed figure and lets the reader see the capture-window gap.
- **Copied from spec 054 — no delta for a figure with no analogue.** `Unattended deliveries`, the baseline's `total` and its `agent coverage` print no delta row. Never add one for symmetry.
- **Copied from spec 054 — never a row reading `0`.** A week absent from `baseline_weeks` omits the row and prints nothing in its place.
- **Copied from spec 054 — an unconfigured baseline and a misconfigured one are different states, and only the first is silent.** A vault with no `baseline` key prints neither block; a vault whose `baseline` is set but broken exits non-zero naming the vault and the resolved path (or the missing key) and prints no figures and no partial baseline block.
- **Copied from spec 054 — backward compatibility is exact, not approximate.** With no `baseline` key the output is byte-identical to the pre-change output — not "equivalent", not "the same figures". The existing Ginkgo suite and the existing operator commands keep passing unchanged.
- **Copied from spec 054 — the rollup never writes.** No vault file, no baseline file, no config file is written by `rollup weekly`.
- **Copied from spec 054 — `pkg/ops/` never writes to stdout.** The operation returns a structured result; the CLI layer owns formatting. Do not move formatting into the operation.
- **Copied from spec 054 — `--output plain` is the default; `--output json` uses the repo's existing JSON printer.** No `encoding/json` import in a command file. Use `PrintJSON`.
- **Copied from spec 054 — dates come from the injected clock**, never `time.Now()`. The capture date is read from the file, never computed.
- **Copied from spec 054 — no scenario file.** The spec's Non-goals rule one out. `ls scenarios/*.md | wc -l` must still print `6`.
- **Copied from spec 054 — Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- **Exactly one changelog bullet, in this prompt only.** Prompts 1 and 2 wrote none; do not add a second here, and do not create a second `## Unreleased` section.
- **Do NOT bump any version string** and do not create a tag: `.maintainer.yaml` sets `release.autoRelease: true` and the post-merge releaser owns version bumps.
- **Do NOT add a knob.** No `--baseline` flag, no `--no-baseline` opt-out, no config key beyond `baseline`, no environment variable, no threshold, no baseline-refreshing verb. An escape hatch on the very behaviour this feature ships is itself a regression.
- **Do NOT add a dependency** and do NOT run `go mod vendor`; never write `-mod=vendor` in a verification command.
- **Do NOT modify `integration/cli_test.go`** (it is 4007 lines and must not grow) and do NOT modify `pkg/cli/export_test.go`. Flag the file's size in your completion report's `## Improvements` section.
- **Tests.** Ginkgo v2 + Gomega in external `_test` packages; one suite entry point per package, never a second `RunSpecs`. Every new Go file keeps its BSD license header.
- **Linter limits** the new code must respect: `funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20, `gocyclo` default. Split the formatter into the small named writers above rather than growing one function past the limit.
- **Do NOT run** `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. The daemon does not check `<verification>` exit codes — read each line's exit status yourself and report `"status":"failed"` if any is non-zero. The checks below are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, so a check that silently stops matching still surfaces as a non-zero exit.

**The formatter's suite, with its frozen spec names.** `-args -ginkgo.v -ginkgo.no-color` is mandatory: Ginkgo v2's default reporter prints only dots on a green run, so without `-ginkgo.v` every name grep below returns zero matches against a correct suite, and with colour on Ginkgo injects ANSI escapes between the container and It texts. Never pipe a test command.

```
go test ./pkg/cli/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/spec054-cli.log 2>&1; test "$?" = "0"
grep -F -q -- 'renders the baseline block between the week header and the computed figures' /tmp/spec054-cli.log
grep -F -q -- 'renders the delta block last with the frozen marker on the median row' /tmp/spec054-cli.log
grep -F -q -- 'renders no baseline and no delta block when the result carries no baseline' /tmp/spec054-cli.log
grep -F -q -- 'omits the human-interactions row when the week has no stored figure' /tmp/spec054-cli.log
```

**The real-binary suite, with its frozen spec names.** `make test` runs `./integration/...` too, but run it explicitly here so a failure is attributable:

```
go test ./integration/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/spec054-int.log 2>&1; test "$?" = "0"
grep -F -q -- 'prints the baseline block from the configured file' /tmp/spec054-int.log
grep -F -q -- "prints the fixture's own figures and not the real ones" /tmp/spec054-int.log
grep -F -q -- 'prints the delta block last with one row per analogue' /tmp/spec054-int.log
grep -F -q -- 'prints no baseline and no delta block when no baseline is configured' /tmp/spec054-int.log
grep -F -q -- 'fails loudly when the baseline file is missing' /tmp/spec054-int.log
grep -F -q -- 'fails loudly when a baseline key is missing' /tmp/spec054-int.log
grep -F -q -- 'rejects a baseline path outside the vault' /tmp/spec054-int.log
grep -F -q -- 'carries the baseline figures and the deltas in JSON' /tmp/spec054-int.log
grep -F -q -- 'prints the edited baseline figure after the file is rewritten' /tmp/spec054-int.log
```

If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment; export it rather than touching `.git`.

**The wiring, the frozen literals and the block order are in the file:**

```
grep -F -q 'ops.NewRollupWeeklyOperationWithBaseline' pkg/cli/rollup.go
grep -F -q 'vault.Baseline' pkg/cli/rollup.go
grep -F -q 'config.ValidateBaselinePath(ctx, vault.Baseline)' pkg/cli/rollup.go
grep -F -q 'Baseline (captured %s)' pkg/cli/rollup.go
grep -F -q 'Delta (vs baseline captured %s)' pkg/cli/rollup.go
grep -F -q '"[definitional mismatch]"' pkg/cli/rollup.go
grep -F -q 'func writeRollupWeeklyBaseline' pkg/cli/rollup.go
grep -F -q 'func writeRollupWeeklyDeltas' pkg/cli/rollup.go
grep -F -q 'func formatRollupNumber' pkg/cli/rollup.go
grep -F -q 'sort.Strings' pkg/cli/rollup.go
test "$(grep -c 'ops.NewRollupWeeklyOperation(' pkg/cli/rollup.go)" = "0"
```

The last line is the constructor swap: the two-argument constructor must no longer be called from the CLI, or the baseline path never reaches the operation and every baseline integration spec fails.

**The formatter is still pure and the block order is the frozen one:**

```
test "$(grep -c '"encoding/json"' pkg/cli/rollup.go)" = "0"
test "$(grep -c 'fmt.Print' pkg/cli/rollup.go)" = "1"
grep -A 8 'func formatRollupWeeklyPlain' pkg/cli/rollup.go | grep -F -q 'writeRollupWeeklyBaseline(&builder, result)'
grep -A 8 'func formatRollupWeeklyPlain' pkg/cli/rollup.go | grep -F -q 'writeRollupWeeklyDeltas(&builder, result)'
```

The `fmt.Print` count is `1` because `formatRollupWeeklyPlain`'s caller prints exactly once; a second occurrence means a writer prints instead of building, which breaks the pure-function contract the existing specs depend on. Read the order with your own eyes as well: the two `grep -A 8` lines only prove the calls exist inside the function.

**README, changelog and the scenario count:**

```
grep -F -q 'docs/baseline-file.md' README.md
grep -F -q 'Delta (vs baseline captured' README.md
test "$(grep '^## ' CHANGELOG.md | head -1)" = "## Unreleased"
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "1"
test "$(grep -cE '^- feat:.*baseline' CHANGELOG.md)" = "1"
test "$(grep -cE '^- feat:.*config set-baseline' CHANGELOG.md)" = "1"
test "$(grep -cE '^- docs:.*baseline-file.md' CHANGELOG.md)" = "1"
make check-changelog
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "6"
```

The three changelog counts pin "exactly one bullet of each kind". Do NOT anchor on `^- feat:.*rollup weekly`: the released `## v0.148.0` section already carries one such bullet, so that pattern counts 2 once yours lands. `- feat:.*baseline` counts 0 today — prompt 1 and prompt 2 write no changelog bullet, which is what makes it 1 after this prompt — and prompt 3 must not remove the existing `- docs:.*baseline-file.md` bullet.

**Nothing out of scope was touched:**

```
test "$(grep -c 'Baseline' integration/cli_test.go)" = "0"
test "$(grep -c 'baseline' pkg/ops/rollup_weekly.go)" -ge 1
test "$(grep -c 'RollupBaseline' pkg/ops/baseline.go)" -ge 1
test "$(grep -c 'set-baseline' pkg/cli/config.go)" -ge 1
test "$(grep -cE '^[[:space:]]+Baseline[[:space:]]+string' pkg/config/config.go)" = "1"
test "$(grep -l 'func TestSuite' integration/*_test.go | wc -l | tr -d ' ')" = "0"
test "$(grep -l 'func TestIntegration' integration/*_test.go | wc -l | tr -d ' ')" = "1"
```

The first line proves `integration/cli_test.go` was not grown. The last two prove the new integration file added no second suite entry point — a second `RunSpecs` panics the whole integration suite.

**The whole suite, the full gate, then formatting:**

```
make test
make precommit
test -z "$(gofmt -e -l pkg/cli/rollup.go pkg/cli/rollup_test.go integration/rollup_baseline_test.go 2>&1)"
```

`make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Finally, walk the spec's Acceptance Criteria 1, 2, 3, 4 and 5 and state in your completion report which requirement and which spec satisfies each one, plus which evidence covers each row of the Failure Modes table (section 7 of `<requirements>` is the mapping). AC 5's comparison against a pre-change binary is operator-only; say so explicitly and name the container-executable evidence that stands in for it.
</verification>
