---
status: completed
spec: [049-weekly-rollup]
summary: Wired `vault-cli rollup weekly` (single-vault resolution via GetVault, --week flag, plain report stating both rule sentences, and JSON via PrintJSON) with unit and real-binary integration specs, README, changelog and docs updates
execution_id: vault-cli-weekly-rollup-exec-218-spec-049-rollup-cli-surface
dark-factory-version: v0.196.0
created: "2026-09-16T12:57:14Z"
queued: "2026-09-25T18:16:58Z"
started: "2026-09-25T19:10:26Z"
completed: "2026-09-25T19:14:49Z"
---

# `vault-cli rollup weekly`: the command, its plain and JSON reports, docs (spec 049, prompt 2 of 2)

<summary>
- The headline question now has a command: one line each for the week's human interactions, its unattended deliveries, and the median interactions of a recurring task family.
- The command covers exactly one vault — the one named by `--vault`, or the configured default when the flag is omitted — never every configured vault at once.
- A week is named with `--week YYYY-Wnn`; omitting it reports the last complete week. A malformed week fails loudly with the expected format in the message and no figures.
- The output states both rules it applied — what counts as an unattended delivery, and how filenames are grouped into families — so a reader can check the numbers without leaving the terminal.
- The per-family breakdown is printed beneath the headline figure, one indented line per family.
- A figure with no measurement behind it reads "undefined" or "no data" in both plain and JSON output — never a zero, never a null.
- The JSON output carries the same three figures under stable keys, and two runs over the same vault produce byte-identical output in either format.
- A week with no recorded counts says so, and a week with no completions reports no data — the two are distinguishable.
- The README documents the command, the changelog records it under Unreleased, and the command is registered in the integration test's command table.
- Nothing else changes: every existing command, flag and output format behaves exactly as before.
</summary>

<objective>
Wire the weekly rollup computation into the CLI as `vault-cli rollup weekly` and give it the two output formats the repo already uses — a plain report that states the rules it applied, and the same figures as JSON — so the spec's headline question ("how much work shipped without me this week?") has an answer that is a number rather than a feeling. This is spec 049's second and final prompt: it covers Desired Behaviors 1 and 7 and Acceptance Criteria 1, 2 (the operator-observable half), 4 and 8, and it depends on prompt 1's computation.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/development-patterns.md` (in particular `## Output Format` and `## Multi-Vault Pattern`) and `docs/dod.md` (the Definition of Done — it names the changelog, the README and the integration command-registration table explicitly).

Read these files fully before making changes:

- `pkg/ops/rollup_weekly.go` — **written by prompt 1 of this spec, which runs before this prompt.** If it is absent when you start, stop and report `"status":"failed"` naming the missing file — do not create it and do not edit `pkg/ops/` (requirement 0). The operation you call, and the result you render. Its exact surface:
  ```go
  type RollupWeeklyOperation interface {
      Execute(ctx context.Context, vaultPath string, vaultName string, week string) (RollupWeeklyResult, error)
  }
  func NewRollupWeeklyOperation(taskStorage storage.TaskStorage, currentDateTime libtime.CurrentDateTime) RollupWeeklyOperation

  type RollupWeeklyResult struct {
      Year                 int            `json:"year"`
      Week                 int            `json:"week"`
      WeekStart            string         `json:"week_start"`
      WeekEnd              string         `json:"week_end"`
      HumanInteractions    string         `json:"human_interactions"`
      UnattendedDeliveries string         `json:"unattended_deliveries"`
      PerFamilyMedian      string         `json:"per_family_median"`
      Families             []RollupFamily `json:"families"`
  }
  type RollupFamily struct {
      Name   string `json:"name"`
      Median string `json:"median"`
  }
  ```
  The three figure fields and every `Median` are **already rendered strings** — the decimal value, or `undefined`, or `no data`, or `no recorded counts`. Do not parse them, re-format them, or convert them to numbers. `Families` is already sorted ascending by `Name`; do not re-sort it, and do not turn it into a map.
- `pkg/cli/cli.go` — the file you add one registration line to. Read `NewRootCommand` (the root `--vault` / `--config` / `--output` / `--verbose` persistent flags and the `rootCmd.AddCommand(...)` block), `getVaults` (the shared resolver: `--vault NAME` → that vault, **no flag → every configured vault** — which is exactly what the rollup must not use), `createWatchCommand` (the sibling that already diverges from `getVaults`, with a doc comment recording why), and `createConfigListCommand` (the `PrintJSON` / `fmt.Printf` split). `Execute` prints `Error: %v` to stderr and exits 1 when `Run` returns an error; the root command sets `SilenceUsage: true`.
- `pkg/cli/output.go` — `PrintJSON(v any) error` and `OutputFormat` / `OutputFormatPlain` / `OutputFormatJSON` / `IsJSON()` / `IsPlain()`.
- `pkg/cli/export_test.go` — the test-only export file (`package cli`, visible to `cli_test`); `CreateResolveCommandForTest`, `CreateTaskBackfillIdentifiersCommandForTest` and `GetWatchVaultsForTest` are its existing entries.
- `pkg/cli/cli_suite_test.go` — the `cli_test` suite entry point.
- `pkg/cli/watch_test.go` and `pkg/cli/resolve_test.go` — the existing `pkg/cli` unit-test style (Ginkgo v2, `cli.Run(ctx, args)` against a temp config, `mocks.Loader` where a loader is injected).
- `pkg/config/config.go` — `GetVault` (lowercases the name; **`vaultName == ""` resolves `config.DefaultVault`**) and `GetAllVaults`. `GetVault("")` is the single-vault resolution the rollup needs.
- `pkg/storage/storage.go` — `NewConfigFromVault(vault)` (carries the vault's `tasks_dir`) and `NewTaskStorage(storageConfig)`.
- `integration/cli_test.go` — the harness you extend. Read `createTempVault(tasks map[string]string)` (one temp vault, `Tasks/`, config with `default_vault: test`), `createTwoTempVaults(...)` (vaults `alpha` and `beta`, config with `default_vault: alpha`), the `gexec.Start` + `Eventually(session).Should(gexec.Exit(0))` + `gbytes` style, the outer `Describe("vault-cli integration tests", …)`, and the `DescribeTable("exits 0 for --help", …)` command-registration table with its `Entry(...)` rows.
- `integration/integration_suite_test.go` — `binPath` is built once in `BeforeSuite` with `gexec.Build("github.com/bborbe/vault-cli")`; the specs run the real binary as a subprocess.
- `README.md` § Usage — the per-noun `### task` … `### config` sections you add one to, each a fenced `bash` block with inline `#` comments.
- `CHANGELOG.md` — the `# Changelog` title, the `All notable changes…` preamble, the `* MAJOR / MINOR / PATCH` lines, then `## v0.133.0`. There is no `## Unreleased` section yet.
- `scripts/check-changelog.sh` — the structural check `make check-changelog` runs: the preamble must precede every `## ` section.
- `specs/in-progress/049-weekly-rollup.md` — the spec. Read Desired Behavior, Acceptance Criteria, Constraints, Failure Modes and Non-goals.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — cobra command construction, `Use` / `Short` / `Long` / `Args`, error propagation.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err` for a newly constructed error.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format and the `## Unreleased` placement rule.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits (`funlen` 80/50, `gocognit` 20, `nestif` 4) and license headers.

Two environment facts:

1. **Make no git calls anywhere, including in `<verification>`.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. Every check below is git-free.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it: `gexec.Build` runs `go build`, which otherwise tries to read a masked `.git` and fails with a VCS status error.

The two rule sentences this command prints are **consumed, not authored**. The unattended rule lives in the vault's `Unattended Execution` topic page and the grouping rule in `Recurring Task Automation Ranking` § Method. Print them; do not reword them, and do not add a config key, flag or environment variable that changes either.
</context>

<requirements>

## 0. Scope — exactly eight files, no others

- `pkg/cli/rollup.go` — new: the command builders and the plain-report formatter.
- `pkg/cli/rollup_test.go` — new: the formatter's unit specs.
- `pkg/cli/export_test.go` — one test-only alias added.
- `pkg/cli/cli.go` — one registration line added to `NewRootCommand`.
- `integration/cli_test.go` — one table entry plus one `Describe` block.
- `README.md` — one `### rollup` usage section.
- `CHANGELOG.md` — one `## Unreleased` section with one bullet.
- `docs/development-patterns.md` — the lines naming `watch` as the only command diverging from `getVaults` (in `## Multi-Vault Pattern` and step 4 of `## Adding a New Command`). Name `rollup` as the second exception and state its reason in one clause, so `## Multi-Vault Pattern` reads `All commands except `watch` and `rollup` use `getVaults()` to resolve vaults:`.

Nothing else. `pkg/ops/`, `pkg/storage/`, `pkg/domain/` and `pkg/config/` are read-only for this prompt — prompt 1 already landed the computation, and any change there is a scope violation rather than a fix. No scenario file is added or changed (`ls scenarios/*.md | wc -l` must still print `5`), and `go.mod` is untouched.

## 1. `pkg/cli/rollup.go` — the command

New file, `package cli`, BSD license header matching its siblings. Three declarations, names frozen because they are grep targets:

```go
// createRollupCommands returns the parent "rollup" command.
func createRollupCommands(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command

// createRollupWeeklyCommand returns the "rollup weekly" leaf command.
func createRollupWeeklyCommand(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
	outputFormat *string,
) *cobra.Command

// formatRollupWeeklyPlain renders a rollup result as the plain-text report.
func formatRollupWeeklyPlain(result ops.RollupWeeklyResult) string
```

- `createRollupCommands` builds `&cobra.Command{Use: "rollup", Short: "Weekly rollup of vault task metrics"}` and adds exactly one child, `createRollupWeeklyCommand(...)`. No other verb: not `rollup monthly`, not `rollup daily`, not a bare `rollup` that does something. The spec's Goal is one command.
- `createRollupWeeklyCommand` builds the leaf: `Use: "weekly"`, `Args: cobra.NoArgs`, and one flag registered on the leaf:
  ```go
  cmd.Flags().StringVar(&week, "week", "", "ISO week to report, e.g. 2026-W37 (default: the last complete ISO week)")
  ```
  Do not add a second flag. `--vault`, `--output` and `--config` are inherited persistent flags and must not be re-declared.
- Register it in `NewRootCommand` in `pkg/cli/cli.go`, alongside the other `rootCmd.AddCommand(...)` calls:
  ```go
  rootCmd.AddCommand(createRollupCommands(ctx, &configLoader, &vaultName, &outputFormat))
  ```

### The RunE body — one vault, never every vault

```go
vault, err := (*configLoader).GetVault(ctx, *vaultName)
if err != nil {
    return errors.Wrap(ctx, err, "get vault")
}
storageConfig := storage.NewConfigFromVault(vault)
taskStore := storage.NewTaskStorage(storageConfig)
rollupOp := ops.NewRollupWeeklyOperation(taskStore, libtime.NewCurrentDateTime())
result, err := rollupOp.Execute(ctx, vault.Path, vault.Name, week)
if err != nil {
    return err
}
if OutputFormat(*outputFormat).IsJSON() {
    return PrintJSON(result)
}
fmt.Print(formatRollupWeeklyPlain(result))
return nil
```

- **`GetVault`, never `getVaults`.** `GetVault(ctx, "")` resolves the config's `default_vault` key; `getVaults` with no `--vault` returns *every* configured vault, and the spec's Constraints deliberately diverge from that: the three figures are properties of one task population, and summing Personal with Brogrammers answers no question. Put a doc comment on `createRollupWeeklyCommand` recording the divergence and the reason, in the style `getWatchVaults` already uses — naming `getVaults` and `GetVault("")` in prose is expected and fine. Only the call form is forbidden: `grep -cE 'getVaults\(ctx' pkg/cli/rollup.go` must be `0`. Write the prose form without the call parentheses (`` `getVaults` ``, or `` `getVaults` with no `--vault` ``) so the comment never matches the check.
- `libtime.NewCurrentDateTime()` is constructed at the call site, exactly as `createCompleteCommand` and its siblings do — this command reads the clock only to resolve the default week.
- The error from `Execute` propagates unwrapped: it already names the vault and the failing path, and the root command's `Execute` prints it and exits 1. A malformed `--week` therefore exits non-zero with `YYYY-Wnn` in the message and prints no figures.
- `PrintJSON(result)` is the only JSON path — no `encoding/json` import in this file, and no hand-rolled marshalling.
- `formatRollupWeeklyPlain` must be **pure**: it takes the result and returns the text. No `fmt.Print*` inside it, no `os.Stdout`, no writing to `cmd.OutOrStdout()`. The `RunE` body is the only place that prints.

## 2. The plain report — exact shape

`formatRollupWeeklyPlain` returns this, for a result with two families, where `2026-W37` is `2026-09-07` to `2026-09-13`:

```
Week: 2026-W37 (2026-09-07 to 2026-09-13)
Human interactions: 26476
Unattended deliveries: 12
Per-family median: 52
  check prometheus alerts: 52
  start day: undefined
Unattended rule: A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0
Grouping rule: A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively.
```

Line by line, all of these are non-negotiable:

1. **Header.** `Week: <year>-W<nn> (<start> to <end>)`, where the week token is `fmt.Sprintf("%d-W%02d", result.Year, result.Week)` — zero-padded to two digits, so week 5 renders `W05` — and the range uses `result.WeekStart` and `result.WeekEnd` verbatim.
2. **The three headline lines start at column 0** and each is `Label: value` — the label, a colon, exactly one space, the value, and nothing else on the line. Labels are exactly `Human interactions`, `Unattended deliveries`, `Per-family median`. The values are `result.HumanInteractions`, `result.UnattendedDeliveries` and `result.PerFamilyMedian` verbatim — never re-formatted, never prefixed with `0`, never replaced by `0` when they read `undefined` or `no data`.
3. **The family block sits directly beneath the `Per-family median` line**, one line per entry in `result.Families`, each indented with **exactly two spaces**, each `<name>: <median>`, nothing else on the line. When `result.Families` is empty the block is empty — no placeholder line, no `(none)`.
4. **The two rule lines come after the family block.** Their labels are `Unattended rule:` and `Grouping rule:`, each followed by a colon and exactly one space and then the sentence. The sentences are frozen string constants in this file:
   ```go
   const unattendedRuleSentence = "A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0"
   const groupingRuleSentence = "A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively."
   ```
   The unattended sentence must be that exact substring — no backticks, no markdown emphasis, no line wrapping, and no rewording of `status is completed` or `metrics_interaction_count is exactly 0`. The grouping sentence must name the filename stem and all four strip classes (dates, week numbers, versions, month names) and the case-insensitive comparison. Both sentences are single-sourced from these constants — never retyped at the call site.
5. **Every line ends with `\n`, including the last**, and there is no trailing blank line, no `===` banner, no colour, no box drawing.
6. The report renders all three degenerate states without special-casing: `no data` (an empty week), `no recorded counts` (a non-empty week with no recorded counts) and `undefined` (a family, or a headline, with no measurement behind it) arrive as ordinary values and print as ordinary values. **No branch in this function may substitute a `0` for any of them**, and no branch may print an empty value.

## 3. `pkg/cli/export_test.go` — one alias

Add to the existing file:

```go
// FormatRollupWeeklyPlainForTest exposes formatRollupWeeklyPlain for testing.
func FormatRollupWeeklyPlainForTest(result ops.RollupWeeklyResult) string {
	return formatRollupWeeklyPlain(result)
}
```

Keep the file's existing doc-comment style (`// Test-only exports for package cli.`). Do not change the three existing exports.

## 4. `pkg/cli/rollup_test.go` — the formatter's unit specs

New file, `package cli_test`, Ginkgo v2 + Gomega. Specs 1–5 call `FormatRollupWeeklyPlainForTest` with hand-built `ops.RollupWeeklyResult` values — no temp vault, no storage, no clock. They pin the exact line shapes; the integration block in section 5 pins the wiring. Spec 6 is a coverage spec and does need a temp vault — see below.

Use these spec names verbatim; they are grep targets in `<verification>`:

1. `"renders the three figures, the family block and both rules"` — one fully-populated result, asserted with a single `Equal` against the whole expected string (a multi-line raw string literal). This is the spec that pins column-0 labels, the two-space indent, the trailing newline and the absence of any extra line. Do not weaken it to a set of `ContainSubstring` calls.
2. `"renders undefined and no-data values without substituting zero"` — a result whose `PerFamilyMedian` is `undefined` and whose families mix `undefined` with real values; assert the rendered text contains `  <name>: undefined`, and assert the rendered text does **not** contain `  <name>: 0` for that family and does not contain `Per-family median: 0`.
3. `"renders a no-data week as three no-data lines and no family block"` — all three figures `no data`, `Families` empty; assert the three `no data` lines are present, that the text contains no line matching an indented `: 0` shape, and that the two rule lines are still printed.
4. `"prints the unattended rule sentence verbatim"` — assert the rendered text contains `A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0` with no backticks in the constant; the literal is a frozen substring of the spec's Acceptance Criterion 3, so a reworded sentence is a failure, not a style choice.
5. `"prints the grouping rule sentence naming the four strip classes"` — assert the rendered text contains `Grouping rule:` and that the sentence names the filename stem, dates, week numbers, versions and month names, and the case-insensitive comparison.
6. `"wires the rollup weekly command in-process"` — invoke the command through `cli.Run(ctx, []string{"--config", configPath, "rollup", "weekly", "--week", "2026-W37"})` against a temp vault that carries a `Tasks/` directory, in the style `pkg/cli/watch_test.go` already uses, and assert it returns no error. **This is a coverage spec, not a behaviour spec:** the section-5 integration specs run the binary as a subprocess, so they contribute no statement coverage to `pkg/cli` — without this spec the two builders and their `RunE` body are uncovered, and `docs/dod.md` asks ≥80% on new code. The behaviour itself is proven in section 5; do not duplicate the integration assertions here.

## 5. `integration/cli_test.go` — the real-binary specs

Two additions to the existing file, inside the outer `Describe("vault-cli integration tests", …)`.

### 5a. The registration table entry

Add one `Entry` to the existing `DescribeTable("exits 0 for --help", …)`, in a new `// Rollup subcommands` group placed after the `// Root-level commands` group:

```go
Entry("rollup weekly", "rollup", "weekly"),
```

### 5b. The `Describe` block

Add a new block after `Describe("vault-cli config list --output json topics_dir", …)`. Its fixture is `createTempVault(map[string]string{...})` — vault name `test`, `tasks_dir: Tasks`, `default_vault: test` — with task files written as raw `---`-delimited frontmatter. A fixture task in `2026-W37` looks like:

```go
"Rollup Fixture A - 2026-09-08": `---
status: completed
page_type: task
metrics_completed_at: "2026-09-08T10:00:00+02:00"
metrics_interaction_count: 7
---
Fixture body.
`,
```

Use these spec names verbatim; they are grep targets in `<verification>`. Every spec starts the real binary through `gexec.Start` and calls the fixture's `cleanup` via `defer`.

1. `"rollup weekly prints the three figures with the rule and the grouping"` — a fixture week of `2026-W37` containing at least one completed task with a recorded zero, one completed task with a recorded positive count, and one task whose count key is absent. Assert exit 0 and that stdout contains a line starting `Human interactions:`, a line starting `Unattended deliveries:`, a line starting `Per-family median:`, the exact substring `A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0`, a `Grouping rule:` line, and at least one line indented by two spaces carrying a family name and a value. Assert the `Unattended deliveries:` value is the count you hand-computed from the fixture — `1`, from the recorded zero alone — so the absent-count task is proven excluded through the real binary and not only in the unit suite.
2. `"rollup weekly reports no data for a week with no metrics"` — a fixture whose tasks carry no `metrics_completed_at`, run with `--week 2026-W20`. Assert exit 0, that stdout contains `no data`, and that stdout matches neither `Human interactions: 0` nor `Human interactions: no recorded counts`. Also assert stdout does not match an indented `: 0` family line.
3. `"rollup weekly prints the same bytes on two runs"` — run the same command twice against the same fixture and assert `string(first.Out.Contents()) == string(second.Out.Contents())`. Do the same for `--output json`. This is the determinism property the spec's Acceptance Criteria 4 and 6 assert by `diff`, reproduced as an assertion.
4. `"rollup weekly --output json carries the same three figures as plain"` — run plain, run `--output json`, unmarshal the JSON into a small local struct with `HumanInteractions`, `UnattendedDeliveries` and `PerFamilyMedian` string fields tagged `human_interactions`, `unattended_deliveries` and `per_family_median`, and assert the plain stdout contains `Human interactions: <v>`, `Unattended deliveries: <v>` and `Per-family median: <v>` for the JSON's values. Assert the JSON parses and the three keys are present. This is Acceptance Criterion 8 as an assertion: the JSON values are **strings**, so `jq`'s `\(.human_interactions)` and the plain line carry the same characters.
5. `"rollup weekly without --vault reads only the default vault"` — the divergence test, and the one that proves the single-vault constraint. Use `createTwoTempVaults` (vaults `alpha` and `beta`, `default_vault: alpha`): give `alpha` a `2026-W37` task with `metrics_interaction_count: 5` and `beta` a `2026-W37` task with `metrics_interaction_count: 999`, then run `rollup weekly --week 2026-W37` with **no** `--vault` flag. Assert exit 0 and that stdout contains `Human interactions: 5`. Assert stdout does **not** contain `Human interactions: 1004` and does not contain `999` — either would mean the all-vaults path was taken. Then run with `--vault beta` and assert `Human interactions: 999`, so the flag is proven to select.
6. `"rollup weekly rejects a malformed --week"` — `--week 2026-W5`. Assert the process exits non-zero and that stderr contains `YYYY-Wnn`. Assert stdout carries none of the three figure labels. Add a second case in the same spec running `--week 2026-W53` and asserting exit 0 — 2026 has 53 ISO weeks, so a naive `<= 52` guard would fail here and the spec's failure-modes table calls this case out by name.

`encoding/json` in a `_test.go` file is fine; the "never import `encoding/json` in a command file" rule applies to `pkg/cli/*.go` non-test files.

## 6. `README.md` — one usage section

Add a `### rollup` section inside § Usage, after the `### config` block and before `## Claude Code Plugin`, matching the neighbouring sections' shape (a fenced `bash` block with inline `#` comments):

```bash
vault-cli rollup weekly                        # Last complete ISO week, plain report
vault-cli rollup weekly --week 2026-W37        # A named ISO week
vault-cli rollup weekly --vault personal       # A named vault (default_vault when omitted)
vault-cli rollup weekly --output json          # The same figures as JSON
```

Follow it with two short paragraphs, in prose: what the three figures mean (human interactions in the week, unattended deliveries, and the median interactions of a recurring task family), that an unattended delivery is a completed task whose recorded interaction count is exactly zero, and that the command reads only task frontmatter from one vault and writes nothing. Name the exact text `rollup weekly` somewhere in the section. Do not document a flag the command does not have, and do not reorder or reword any other line in the file.

## 7. `CHANGELOG.md` — one bullet under a new `## Unreleased`

There is no `## Unreleased` section yet. Create it directly below the preamble — after the `* PATCH version when you make backwards-compatible bug fixes.` line and above `## v0.133.0` — and put one bullet in it:

```
## Unreleased

- feat: `vault-cli rollup weekly` reports a week's human interactions, unattended deliveries and per-family median interactions from task frontmatter, stating the unattended-delivery rule and the filename-stem grouping rule in its output. It reads one vault — the `--vault` flag, or the config's `default_vault` when the flag is omitted, never every configured vault — accepts `--week YYYY-Wnn` and defaults to the last complete ISO week, supports `--output plain|json`, and writes nothing to the vault.
```

- The bullet must start with `- feat:` on a single line and must name `rollup weekly`; the remaining wording is yours to adjust.
- Do NOT bump any version: not `CHANGELOG.md`'s newest version heading, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. `.maintainer.yaml` sets `release.autoRelease: true`, and the post-merge releaser converts `## Unreleased` into a versioned section and tags it — a hand-bump would race it. `make check-versions` must still report the version it reports today.
- `make precommit` runs `scripts/check-changelog.sh`, which fails if any `## ` section lands above the preamble. Place the section below the preamble, as described.

## 8. Failure modes and security — what each carries

Map the spec's Failure Modes table onto the change and state the mapping in your completion report:

- **`--week` malformed, or a week number outside the year's ISO week count** → the operation returns a usage error naming `YYYY-Wnn`, the command exits non-zero, no figures print. Covered by integration spec 6, which also pins `2026-W53` as valid.
- **Week contains no task with `metrics_completed_at`** → three `no data` lines, exit 0. Covered by integration spec 2.
- **Week's set non-empty but no member carries a recorded count** → `no recorded counts` for the two scalars, `undefined` for the median. The values arrive from the operation; the formatter must print them unchanged. Covered by the unit specs in section 4 (spec 2) and by the operation's own suite from prompt 1.
- **A `metrics_*` value is malformed (non-numeric)** → treated as absent by the accessor, before this prompt's code is reached. Nothing to build here; do not add a defensive branch.
- **A task file is unreadable mid-scan** → the operation returns an error naming the file and the vault; the command propagates it and exits non-zero with no figures. Do not catch, log-and-continue, or fall back to a partial report.
- **The resolved vault's task directory is missing** → same path: the operation's error names the vault and the resolved directory, the command exits non-zero. `tasks_dir` defaults to `Tasks` when unset, so an unset key is not itself an error — do not add a config check or an existence check.
- **Two tasks completing either side of a local midnight in the same UTC week** → each lands in its own local week; that rule lives in prompt 1's computation and is unaffected by formatting.
- **A probe mutates a task count and is not restored** → impossible by construction; probes run against a scratch vault copy. Operator-side; nothing to build.

Security: the command reads only the resolved vault's own task files and writes nothing — after a read-only run the vault tree must be byte-identical to what it was before, a property the spec's operator rung checks on the real vault and which nothing in this change may weaken. It takes no free-form input beyond a week token and a vault name; the week token is validated before use and the vault name is looked up in the operator's own config. Do not add a shell invocation, a network call, a path built from user input, or an `os.Exec`. Do not add escaping or sanitisation to the formatter: the values it prints come from frontmatter that the storage layer already parsed, and the family names are already reduced to lowercase alphanumerics and single spaces by prompt 1's normalization.

## 9. Self-check before finishing

- Re-read the changed hunks and confirm: `pkg/cli/rollup.go` contains no `getVaults`, no `encoding/json`, no `fmt.Print*` inside `formatRollupWeeklyPlain`, and no second flag; the registration line is present in `NewRootCommand`; `pkg/ops/`, `pkg/storage/`, `pkg/domain/` and `pkg/config/` are untouched.
- Walk the spec's Acceptance Criteria 1, 2, 4 and 8 and state in your completion report which requirement and which spec satisfies each one, and which spec covers each row of the Failure Modes table (section 8 is the mapping).
- Walk `docs/dod.md`: the exported identifiers have doc comments, no `fmt.Print*` was added to `pkg/ops/`, tests use Ginkgo v2 / Gomega, the README documents the new command, the changelog entry sits under `## Unreleased` below the preamble, and the integration command-registration table gained its entry.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 049 — one vault, never an aggregate.** This deliberately diverges from `getVaults`'s "no flag → all configured vaults" default: the three figures are properties of one task population, and summing Personal with Brogrammers would produce a number that answers no question. `--vault` selects it; with the flag absent the command resolves the config's `default_vault` key via `GetVault("")`, never the all-vaults path.
- **Copied from spec 049 — read set is the resolved vault's configured `tasks_dir`, frontmatter only.** No data file, no cache, no sidecar. The directory comes from the vault config via `storage.NewConfigFromVault`, never hardcoded — configured vaults use `tasks`, `24 Tasks` and `25 Tasks` variously. The command reads nothing else and writes nothing.
- **Copied from spec 049 — an absent count is never coerced to zero.** The operation already encodes this through the typed accessor; the formatter must not reintroduce it by substituting `0` for `undefined`, `no data` or `no recorded counts`.
- **Copied from spec 049 — the unattended rule is the vault's written rule verbatim**: `status` completed and `metrics_interaction_count` exactly `0`. Zero means a *recorded* zero. The printed sentence must be the frozen literal, unbackticked and unbroken.
- **Copied from spec 049 — the grouping rule is the filename-stem rule already documented in the vault**, and the ≥3-instance threshold governs whether a family counts as recurring evidence, never membership. The command only states the rule; it does not apply a threshold.
- **Copied from spec 049 — `--output plain` is the default; `--output json` uses the repo's existing JSON printer.** No `encoding/json` import in a command file. Use `PrintJSON`.
- **Copied from spec 049 — `pkg/ops/` never writes to stdout.** The operation returns a structured result; the CLI layer owns formatting. Do not move formatting into the operation, and do not change the operation.
- **Copied from spec 049 — Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- **Copied from spec 049 — existing task/goal/theme commands and their output are unchanged**, and `make test` / `make precommit` stay green. No existing command gains or loses a line, no flag changes meaning, and `getVaults` keeps its all-vaults default for every other caller.
- **Do NOT add a scenario file** — the spec's Non-goals rule it out explicitly (the four-condition scenario test fails on its first condition, matching the sibling `036-passive-per-task-metrics` decision). `ls scenarios/*.md | wc -l` must still print `5`.
- **Do NOT add a knob.** No opt-out flag, no config key, no environment variable, no tunable threshold, no `--include-*` filter, no second verb under `rollup`. The rule, the grouping and the three figures are invariants of this feature.
- **Do NOT bump any version string** and do not create a tag.
- **Do NOT run `go mod vendor`** and never write `-mod=vendor` in a verification command. Do not add a dependency.
- **Tests.** Ginkgo v2 + Gomega in external `_test` packages; counterfeiter mocks only where the repo already generates them. Every new Go file keeps its BSD license header.
- **Linter limits** the new file must respect: `funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20. Split the formatter into small named helpers rather than growing one function past the limit.
- **Do NOT run** `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

The checks below are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, so a check that silently stops matching still surfaces as a non-zero exit. The daemon does not check `<verification>` exit codes — you must read each line's exit status yourself and report `"status":"failed"` if any of them is non-zero.

**The two suites, with their frozen spec names.** Capture each run, check its exit status separately from the name greps, and never pipe a test command. `-args -ginkgo.v -ginkgo.no-color` is mandatory, not cosmetic: Ginkgo v2's default reporter prints only dots on a green run, so without **`-ginkgo.v`** every spec-name grep below returns zero matches against a correct suite. `-ginkgo.no-color` is the second half of the guarantee: with colour on, Ginkgo injects ANSI escapes *between* the container and It texts, so keep both flags to ensure every target below matches as a contiguous substring:

```
go test ./pkg/cli/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/rollup-cli.log 2>&1; test "$?" = "0"
grep -F -q -- 'renders the three figures, the family block and both rules' /tmp/rollup-cli.log
grep -F -q -- 'renders undefined and no-data values without substituting zero' /tmp/rollup-cli.log
grep -F -q -- 'renders a no-data week as three no-data lines and no family block' /tmp/rollup-cli.log
grep -F -q -- 'prints the unattended rule sentence verbatim' /tmp/rollup-cli.log
grep -F -q -- 'prints the grouping rule sentence naming the four strip classes' /tmp/rollup-cli.log
grep -F -q -- 'wires the rollup weekly command in-process' /tmp/rollup-cli.log
```

```
go test ./integration/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/rollup-int.log 2>&1; test "$?" = "0"
grep -F -q -- 'rollup weekly prints the three figures with the rule and the grouping' /tmp/rollup-int.log
grep -F -q -- 'rollup weekly reports no data for a week with no metrics' /tmp/rollup-int.log
grep -F -q -- 'rollup weekly prints the same bytes on two runs' /tmp/rollup-int.log
grep -F -q -- 'rollup weekly --output json carries the same three figures as plain' /tmp/rollup-int.log
grep -F -q -- 'rollup weekly without --vault reads only the default vault' /tmp/rollup-int.log
grep -F -q -- 'rollup weekly rejects a malformed --week' /tmp/rollup-int.log
```

`go test ./integration/...` builds the binary with `gexec.Build` and runs it as a subprocess against temp vaults — it is the real-binary proof of the three figures, the rule sentences, the determinism and the JSON surface, and the repeatable form of the spec's operator rung. If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment; export it rather than touching `.git`.

**The command is registered and the divergence is real:**

```
grep -F -q 'createRollupCommands' pkg/cli/cli.go
test "$(grep -c 'rootCmd.AddCommand(createRollupCommands' pkg/cli/cli.go)" = "1"
test "$(grep -cE 'getVaults\(ctx' pkg/cli/rollup.go)" = "0"
test "$(grep -cE '\.GetVault\(ctx' pkg/cli/rollup.go)" = "1"
grep -F -q 'Entry("rollup weekly", "rollup", "weekly")' integration/cli_test.go
```

The third line is the one that matters most: a single call to `getVaults` in this file would silently make the command aggregate every configured vault, which is the constraint the spec calls out as a deliberate divergence.

**The formatter's shape — pure, hand-rolled JSON-free, and carrying the frozen rule literals:**

```
test "$(grep -c '"encoding/json"' pkg/cli/rollup.go)" = "0"
grep -F -q 'PrintJSON(result)' pkg/cli/rollup.go
grep -F -q 'A task is an unattended delivery when its status is completed and its metrics_interaction_count is exactly 0' pkg/cli/rollup.go
grep -F -q 'A task family is the filename stem with dates, week numbers, versions and month names stripped, compared case-insensitively.' pkg/cli/rollup.go
test "$(grep -c 'StringVar(&week, "week"' pkg/cli/rollup.go)" = "1"
test "$(grep -c 'cmd.Flags()' pkg/cli/rollup.go)" = "1"
```

The last two pin "exactly one new flag": `--vault`, `--output` and `--config` are inherited persistent flags, so a second `cmd.Flags()` registration is a scope violation. The second-to-last pins the flag's variable name, which the frozen `RunE` shape depends on.

**The formatter is exercised by a unit spec, not only by the subprocess suite:**

```
grep -F -q 'FormatRollupWeeklyPlainForTest' pkg/cli/export_test.go
grep -F -q 'FormatRollupWeeklyPlainForTest' pkg/cli/rollup_test.go
grep -F -q 'Per-family median: 0' pkg/cli/rollup_test.go
```

The third line is the absence assertion, written positively so it survives `grep -c` exiting 1 on a zero count: the spec must state the string it forbids.

**README, changelog, doc, scenario count:**

```
grep -F -q 'rollup weekly' README.md
grep -F -q '### rollup' README.md
test "$(grep '^## ' CHANGELOG.md | head -1)" = "## Unreleased"
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "1"
test "$(grep -cE '^- feat:.*rollup weekly' CHANGELOG.md)" -ge 1
make check-changelog
make check-versions
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "5"
grep -qE 'except .watch. and .rollup. use .getVaults' docs/development-patterns.md
```

If `grep '^## ' CHANGELOG.md | head -1` prints a version heading, the bullet was placed between released sections — move it into the `## Unreleased` section directly below the preamble.

**Formatting:**

```
test -z "$(gofmt -e -l pkg/cli/rollup.go pkg/cli/rollup_test.go pkg/cli/export_test.go pkg/cli/cli.go integration/cli_test.go 2>&1)"
```

**The computation was not touched by this prompt:**

```
test "$(grep -c 'ListTasksStrict' pkg/ops/rollup_weekly.go)" -ge 1
grep -F -q 'json:"human_interactions"' pkg/ops/rollup_weekly.go
```

These two lines fail if `pkg/ops/rollup_weekly.go` was renamed, rewritten or had its JSON tags changed to make the formatter's life easier — a change that belongs to prompt 1 and would invalidate its suite.

Finally, run `make precommit` once more and confirm it exits 0, then walk spec 049's Acceptance Criteria 1, 2, 4 and 8 against the change and state in your completion report which requirement and which spec satisfies each one, plus which evidence covers each row of the spec's Failure Modes table (section 8 of `<requirements>` is the mapping).
</verification>
