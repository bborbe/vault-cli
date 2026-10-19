---
status: completed
spec: [060-bug-rollup-aborts-on-frontmatterless-task-file]
summary: Weekly rollup now reads tasks tolerantly, reports skipped task files in the result's `skipped` JSON array and a final `skipped:` plain-report line, with ops and CLI specs covering the reproduction fixture, JSON array-not-null, missing tasks_dir, and report ordering.
execution_id: vault-cli-rollup-skip-frontmatter-exec-258-rollup-skip-result-and-renderers
dark-factory-version: v0.198.7
created: "2026-10-19T17:56:00Z"
queued: "2026-10-19T18:10:28Z"
started: "2026-10-19T18:16:55Z"
completed: "2026-10-19T18:20:44Z"
branch: dark-factory/bug-rollup-aborts-on-frontmatterless-task-file
---

# Surface skipped task files in the rollup result and both renderers

<!--
Decomposition note (for the auditor): the spec's row 2 owns AC1, AC3 and AC4.
AC6's evidence in the spec is an `Execute`-level test, so this prompt adds the
rollup-level AC6 assertion; prompt 1 already pins the same invariant at the
storage boundary. The spec's row 1 listed AC6 under prompt 1 — the storage-level
test there is kept, and this prompt adds the operation-level proof the AC text
actually names.
-->

<summary>
- The weekly rollup no longer aborts when a single task file cannot be parsed.
- It reports the week's figures from every file it could read.
- The result carries the list of files it skipped, serialized as a JSON array.
- Plain output prints exactly one final line beginning `skipped:` naming them.
- A run that skipped nothing prints no skip line and emits an empty array, never `null`.
- Skipped names are shown as paths relative to the configured tasks directory.
- A missing tasks directory is still an error — it is never reported as an empty week.
- The existing report layout is unchanged when nothing was skipped.
- The rollup reads the tolerant listing, so one unreadable file no longer aborts the week.
</summary>

<objective>
Make the weekly rollup consume the tolerant task listing and surface the files it skipped in both the plain report and the JSON result, so one malformed task file costs one named omission instead of the whole week's measurement.
</objective>

<context>
Read `CLAUDE.md` for project conventions and `docs/development-patterns.md` for the output-format (`PrintJSON`) and testing conventions. This repo is a single Go module at the repo root; run `make test` and `make precommit` at the repo root.

Read these files before editing:

- `pkg/ops/rollup_weekly.go` — `RollupWeeklyResult`, `Execute` (the `ListTasksStrict` call), the `rollupNoData` / `rollupNoRecordedCounts` constants, and `rollupFigures`.
- `pkg/ops/rollup_weekly_test.go` — the Ginkgo suite, the `writeRollupTask` and `rollupTaskFrontmatter` helpers, and the existing spec `returns an error naming an unreadable task file`.
- `pkg/cli/rollup.go` — `formatRollupWeeklyPlain` and its `writeRollupWeekly*` helpers; `writeRollupWeeklyDeltas` currently writes the final block.
- `pkg/cli/rollup_test.go` — the exact-output specs over `cli.FormatRollupWeeklyPlainForTest`.
- `pkg/cli/export_test.go` — `FormatRollupWeeklyPlainForTest` already exposes the renderer; do not add a new export.
- `pkg/cli/output.go` — `PrintJSON` (writes to stdout; the JSON test must use `json.Marshal` instead).
- `pkg/storage/storage.go` — the `TaskStorage` interface entry added by the previous prompt.

Coding guides (in-container paths):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md`

Load-bearing contracts added by prompt 1 of this spec (run it first):

```go
ListTasksTolerant(ctx context.Context, vaultPath string) ([]*domain.Task, []string, error)
```

The returned `skipped` slice is sorted, `tasks_dir`-relative, control-character-sanitised, and non-nil when empty.

The result struct as it stands (add the field, keep every existing tag):

```go
type RollupWeeklyResult struct {
	Year                 int             `json:"year"`
	Week                 int             `json:"week"`
	WeekStart            string          `json:"week_start"`
	WeekEnd              string          `json:"week_end"`
	HumanInteractions    string          `json:"human_interactions"`
	UnattendedDeliveries string          `json:"unattended_deliveries"`
	PerFamilyMedian      string          `json:"per_family_median"`
	Families             []RollupFamily  `json:"families"`
	Baseline             *RollupBaseline `json:"baseline,omitempty"`
}
```

The reproduction fixture (from the spec — reproduce it verbatim in the AC1 spec):

```bash
VAULT=/tmp/rollup-skip-fixture
mkdir -p "$VAULT/Tasks"
# One healthy completed task with a recorded zero: a recorded zero is a measurement,
# so it contributes 0 human interactions and exactly 1 unattended delivery.
printf -- '---\nstatus: completed\nmetrics_completed_at: "2026-10-14T10:00:00Z"\nmetrics_interaction_count: 0\n---\n# Healthy\n' \
  > "$VAULT/Tasks/Healthy.md"
# One task file with no frontmatter at all.
printf 'Failed to authenticate: OAuth session expired and could not be refreshed\n' \
  > "$VAULT/Tasks/Broken.md"
```

Expected figures: `Human interactions: 0`, `Unattended deliveries: 1`, one skipped entry `Broken.md`. The week token is `2026-W42`.
</context>

<requirements>
1. In `pkg/ops/rollup_weekly.go`, add the field to `RollupWeeklyResult`, after `Families` and before `Baseline`:

   ```go
   Skipped []string `json:"skipped"`
   ```

   No `omitempty`: the key must always be present. Never emit `null` — the zero value produced by `Execute` must marshal as `[]`.

2. In `Execute`, switch the listing and record the skipped names. The existing result literal already sets `Families: []RollupFamily{}`; add `Skipped: []string{}` beside it. Replace the strict listing block with:

   ```go
   tasks, skipped, err := o.taskStorage.ListTasksTolerant(ctx, vaultPath)
   if err != nil {
       return RollupWeeklyResult{}, errors.Wrapf(ctx, err, "list tasks of vault %s", vaultName)
   }
   if len(skipped) != 0 {
       result.Skipped = skipped
   }
   ```

   `Skipped` is set before the `len(weekTasks) == 0` early return, so a week whose every task was unparseable still returns `no data` figures plus the skipped names. Keep the error text `list tasks of vault %s` unchanged. Update `Execute`'s doc comment: it no longer reads the task files strictly; it reads them tolerantly and names the files it skipped.

3. In `pkg/cli/rollup.go`, make the skipped line the report's final line. Add `writeRollupWeeklySkipped(&builder, result.Skipped)` to `formatRollupWeeklyPlain` after the existing `writeRollupWeeklyDeltas(&builder, result)` call. Add the helper:

   ```go
   // writeRollupWeeklySkipped writes the report's final line: one `skipped:` line
   // naming every task file the run could not read, as a tasks_dir-relative path.
   // A run that skipped nothing writes nothing, so a clean report carries no skip
   // noise and keeps its pre-skip bytes.
   func writeRollupWeeklySkipped(builder *strings.Builder, skipped []string) {
       if len(skipped) == 0 {
           return
       }
       fmt.Fprintf(builder, "skipped: %s\n", strings.Join(skipped, ", "))
   }
   ```

   Format is exactly `skipped: <path>` for one entry and `skipped: <path1>, <path2>` for several. `strings` and `fmt` are already imported in this file. Do not change any other renderer or the ordering of the existing blocks.

4. Update the now-stale interface comment in `pkg/storage/storage.go`. Replace the `ListTasksStrict` doc comment

   ```go
   // ListTasksStrict returns every task in the vault's configured tasks_dir and
   // fails on an unreadable task file instead of skipping it. A caller whose
   // result must not silently describe a smaller set (the weekly rollup) uses
   // this; the error names the offending file.
   ```

   with

   ```go
   // ListTasksStrict returns every task in the vault's configured tasks_dir and
   // fails on an unreadable task file instead of skipping it. A caller that must
   // not silently describe a smaller set uses this; the error names the offending
   // file. A caller that wants to report the readable set and name the skipped
   // files uses ListTasksTolerant.
   ```

   This is a comment-only change: do NOT touch the `ListTasksStrict` method body in `pkg/storage/task.go`, and do not change the interface signature.

5. Update the existing spec in `pkg/ops/rollup_weekly_test.go` named `returns an error naming an unreadable task file`. Its current assertions are the pre-fix behaviour and will fail after this change. Keep the root-user skip guard and the `chmod 0000` setup, then assert the new behaviour: `err` is nil, `result.Skipped` equals `[]string{"Unreadable Task.md"}`, and the figures render as `no data` because the only task in the week was skipped (`result.HumanInteractions == "no data"`, `result.UnattendedDeliveries == "no data"`, `result.PerFamilyMedian == "no data"`).

6. Add the AC1 spec to `pkg/ops/rollup_weekly_test.go`, reusing `writeRollupTask` and `rollupTaskFrontmatter`:

   ```go
   writeRollupTask(vaultPath, "Healthy", rollupTaskFrontmatter(
       "completed", "2026-10-14T10:00:00Z", "0",
   ))
   Expect(os.WriteFile(
       filepath.Join(vaultPath, "Tasks", "Broken.md"),
       []byte("Failed to authenticate: OAuth session expired and could not be refreshed\n"),
       0600,
   )).To(Succeed())

   result, err := rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W42")
   Expect(err).To(BeNil())
   Expect(result.HumanInteractions).To(Equal("0"))
   Expect(result.UnattendedDeliveries).To(Equal("1"))
   Expect(result.Skipped).To(Equal([]string{"Broken.md"}))
   ```

   This is the spec's `# Reproduction` fixture. Do not assert `no data` for the figures — the readable task must still be measured.

7. Add the AC6 operation-level spec: remove `Tasks` from the vault (`os.RemoveAll(filepath.Join(vaultPath, "Tasks"))`) and call `Execute`. Assert `err` is not nil and the result equals `ops.RollupWeeklyResult{}` (no report is produced for a misconfigured vault).

8. Add the AC4 JSON specs. Both must marshal a result returned by `Execute` (not `PrintJSON`, which writes to stdout):

   a. Fixture vault — recreate the AC1 fixture inside this `It` (Ginkgo `It` blocks do not share state): `json.Marshal(result)` then unmarshal into a struct with `Skipped []string \`json:"skipped"\``. Assert the decoded slice equals `[]string{"Broken.md"}` and the raw bytes contain `"skipped":["Broken.md"]`.

   b. Clean vault: one completed task with `metrics_interaction_count: 2` in the week (`writeRollupTask(vaultPath, "Attended", rollupTaskFrontmatter("completed", "2026-10-14T10:00:00Z", "2"))`). Assert the raw JSON contains `"skipped":[]` and does NOT contain `"skipped":null`, and the decoded slice has length 0. This is the array-not-null lock.

   Also add `"skipped"` to the frozen-key list in the existing spec `renders the result under the frozen JSON keys`, set `Skipped: []string{}` in that spec's literal, and assert the raw bytes contain `"skipped":[]` — the key must not disappear, and a zero-value literal must not marshal `null` unnoticed.

9. Add the AC3 specs to `pkg/cli/rollup_test.go` over `cli.FormatRollupWeeklyPlainForTest`:

   a. With `Skipped: []string{"Broken.md"}` (plus `Families: []ops.RollupFamily{}`), split the output on newlines after `strings.TrimRight(rendered, "\n")`. Assert the final line equals `skipped: Broken.md`; count lines beginning `skipped: ` and assert the count is exactly 1; assert the output contains `Broken.md`.

   b. With several names, `Skipped: []string{"sub-a/Broken.md", "sub-b/Broken.md"}`, assert the final line equals `skipped: sub-a/Broken.md, sub-b/Broken.md` and there is still exactly one `skipped: ` line.

   c. With `Skipped: []string{}` and with `Skipped: nil`, assert `strings.Contains(rendered, "skipped:")` is false in both cases. A clean report must be byte-identical to the pre-skip report, so the existing exact-output specs must keep passing unchanged.

10. Do not touch `pkg/storage/task.go`, the mocks, the README, or `CHANGELOG.md` in this prompt. Do not add a metrics counter, a config knob, or a new command flag — the spec asks for none.
</requirements>

<constraints>
- Copied from the spec: the result field is `Skipped []string` with JSON tag `skipped`; it is always an array, empty when nothing was skipped, never `null`.
- `pkg/ops/` stays output-free. The operation returns structured data; all formatting stays in `pkg/cli/`.
- The skipped line is the last line of the report, after the delta block, so the figures block's layout is unchanged. When none was skipped, the line is absent — a clean run prints no skip noise.
- The skipped files contribute nothing to any figure, and the figures are not rendered as `no data` merely because something was skipped.
- A walk-level failure (missing or unreadable `tasks_dir`) is still fatal. Do not swallow it.
- Do not weaken `ListTasksStrict`; do not change its method body or its interface signature.
- Do not hand-bump any version string or tag (`.maintainer.yaml: release.autoRelease: true`).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass, except the one spec this prompt explicitly rewrites to the new behaviour.
</constraints>

<verification>
Run at the repo root:

```bash
make test
make precommit
```

Both must exit 0.

Then confirm the new surface:

```bash
grep -n 'json:"skipped"' pkg/ops/rollup_weekly.go
grep -n 'Skipped' pkg/ops/rollup_weekly.go
grep -n 'ListTasksTolerant' pkg/ops/rollup_weekly.go
grep -n 'writeRollupWeeklySkipped(&builder, result.Skipped)' pkg/cli/rollup.go
grep -n 'skipped: %s' pkg/cli/rollup.go
! grep -q 'ListTasksStrict' pkg/ops/rollup_weekly.go
```

The five `grep -n` commands must print a line each. The final `! grep -q` must exit 0: the rollup must no longer call the strict listing.

Before finishing, re-run the verification block and confirm it passes. Walk AC1, AC3, AC4 and AC6 from the spec against the new specs and confirm each is exercised by a named spec with the pinned values.
</verification>
