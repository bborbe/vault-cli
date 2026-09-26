---
spec: [054-rollup-weekly-baseline-delta]
status: draft
created: "2026-09-26T15:48:55Z"
---

# The baseline file, its frozen keys, the analogue mapping and the delta arithmetic (spec 054, prompt 2 of 3)

<summary>
- The rollup learns to read a vault's baseline file and to carry its five stored figures in the result.
- The five figures are echoed exactly as the file stores them — never recomputed, rounded or normalised.
- Each figure the rollup computes that has an analogue in the baseline gets a delta: the computed value, the stored value, and the movement between them.
- The per-family median's delta is marked as a definitional mismatch, because the stored figure is a per-task median and the computed one is a median of per-family medians.
- A figure with no analogue — the unattended-delivery count, the stored total, the stored agent coverage — produces no delta at all.
- A week the baseline does not carry produces no human-interactions delta, never a row reading zero.
- A misconfigured baseline fails loudly and prints nothing: a missing file names the vault and the resolved path, a missing key names the key, a non-integer figure names the key.
- An absolute baseline path, or one escaping the vault root, is refused by the same rule the config verb uses at write time.
- The JSON result carries the stored figures and the deltas under one key, and carries no such key when the vault has no baseline.
- Nothing user-visible changes yet: this prompt lands the computation, and prompt 3 renders it.
</summary>

<objective>
Implement the baseline side of the weekly rollup: parse the hand-authored baseline file's five frozen frontmatter keys, map each computed figure to its stored analogue, compute the delta rows, and carry all of it in `RollupWeeklyResult` so the CLI can render it and the JSON printer can serialise it. This is spec 054's second prompt: it covers Desired Behaviors 2, 4 and 7 and Acceptance Criterion 2, and it depends on nothing at compile time — it must not import `pkg/config`. Prompts 1 and 2 run in sequence on the same branch under `workflow: direct`, so prompt 1's `baseline` key is already present in `pkg/config/config.go` when this prompt runs; that is expected, and is not a reason to read or edit `pkg/config/`.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/development-patterns.md` and `docs/dod.md` (whose Code Quality section carries the rule that `pkg/ops/` is a library layer whose operations return structured results and NEVER write to stdout — the CLI layer owns all formatting).

Read these files fully before making changes:

- `pkg/ops/rollup_weekly.go` (313 lines) — the file you extend. The `//counterfeiter:generate -o ../../mocks/rollup-weekly-operation.go --fake-name RollupWeeklyOperation . RollupWeeklyOperation` annotation and the `RollupWeeklyOperation` interface whose only method is `Execute(ctx context.Context, vaultPath string, vaultName string, week string) (RollupWeeklyResult, error)`; `NewRollupWeeklyOperation(taskStorage storage.TaskStorage, currentDateTime libtime.CurrentDateTime) RollupWeeklyOperation`; `type rollupWeeklyOperation struct` holding exactly those two deps; `RollupWeeklyResult` (`Year`, `Week`, `WeekStart`, `WeekEnd`, `HumanInteractions`, `UnattendedDeliveries`, `PerFamilyMedian`, `Families`, each with a snake_case `json:` tag) and `RollupFamily`; the sentinel constants `rollupNoData = "no data"`, `rollupNoRecordedCounts = "no recorded counts"`, `rollupUndefined = "undefined"`; and `Execute`'s body, which resolves the week, calls `o.taskStorage.ListTasksStrict(ctx, vaultPath)`, and derives the three figures. Note that `HumanInteractions` and `PerFamilyMedian` are **already rendered strings** — a decimal, or one of the three sentinels — and that `PerFamilyMedian` can be fractional (`strconv.FormatFloat(median, 'f', -1, 64)` over a mean of two middle values).
- `pkg/ops/ops_suite_test.go` — the `ops_test` suite entry point. **This package has exactly ONE `func TestSuite(t *testing.T)` calling `RunSpecs`. A second `func Test*` calling `RunSpecs` panics, so do not add one.**
- `pkg/ops/rollup_weekly_test.go` — the suite you extend. Its helpers `writeRollupTask(vaultPath, name, frontmatter string)` and `rollupTaskFrontmatter(status, completedAt, count string)`, its `BeforeEach` that makes a temp vault with a `Tasks/` directory and a `libtime.NewCurrentDateTime()` pinned with `currentDateTime.SetNow(libtimetest.ParseDateTime("2026-09-16T12:00:00Z"))`, and its call style `rollupOp.Execute(ctx, vaultPath, vaultName, "2026-W37")`.
- `pkg/storage/base.go` — the frontmatter parser you reuse. `frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)` and `func (b *baseStorage) parseToFrontmatterMap(ctx context.Context, content []byte) (map[string]any, error)`, whose body uses only the package-level `frontmatterRegex` and `quoteBareWikilinks` — the receiver is unused. The package comment on `CheckboxRegex` states the house rule: "Shared across storage and ops packages to keep the parser shape in one place." Follow it — do not write a second frontmatter regex in `pkg/ops`.
- `pkg/storage/export_test.go` — `ParseToFrontmatterMapForTest` calls `b.parseToFrontmatterMap`, and `pkg/storage/base_test.go` exercises it. Your change must keep both working.
- `pkg/config/baseline.go` — **written by prompt 1 of this spec, and NOT a dependency of this prompt.** You must not import `pkg/config` and must not call `config.ValidateBaselinePath`: this prompt is deliberately independent of prompt 1, so it runs alongside it. The CLI validates the config-supplied path before it constructs the operation (prompt 3), and hands this operation a path that has already been accepted. Do not add a validation call of your own here, and do not edit `pkg/config/`.
- `docs/baseline-file.md` — the authority for the frontmatter contract: the five frozen keys and their types, the analogue mapping table, the "no delta row" rows, and the frozen report layout. Your parse and your mapping implement this document.
- `specs/in-progress/054-rollup-weekly-baseline-delta.md` — the spec. Read Desired Behaviors 2, 4, 5 and 7, the Constraints, the Failure Modes table and the Security section.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` / `errors.Wrapf(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never `context.Background()` inside `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md` — gosec rules and `#nosec` with a reason.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits (`funlen` 80/50, `gocognit` 20, `nestif` 4, `maintidx` 20) and license headers.

Two verified facts about `gopkg.in/yaml.v3`, confirmed by running it against the contract's own example. They are the reason the parse is written the way section 2 describes:

1. Unmarshalling the frontmatter into a `map[string]any` yields `baseline_captured` as a **`time.Time`** (the contract's example writes the date unquoted, so YAML resolves it as a timestamp), `baseline_human_total` and `baseline_median` as `int`, `baseline_weeks` as `map[string]any` with `int` values, and `baseline_agent_coverage` as `string`. Unmarshalling the same block into a struct with a `string` field for `baseline_captured` gives the literal `2026-09-12` — so the `string` field form works, but the struct form cannot report a *missing* key (it silently leaves the zero value), which the acceptance criteria require. Parse into the map and extract each key explicitly.
2. A non-integer value fails a typed `int` decode with an error that names the YAML **line**, not the key (`cannot unmarshal !!str 'abc' into int`). The acceptance criteria require the error to name `baseline_median`, so the type checks must be explicit and must carry the key name.

Two environment facts:

1. **Make no git calls anywhere, including in `<verification>`.** The daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass. Every check below is git-free.
2. **`make generate` deletes and regenerates the whole `mocks/` tree via `go generate ./...`.** The `RollupWeeklyOperation` interface is unchanged by this prompt, so its mock is unchanged too — do not hand-edit or hand-write any file under `mocks/`.
</context>

<requirements>

## 0. Scope — exactly four files, no others

- `pkg/storage/base.go` — modify: export the frontmatter parser, keeping the existing method as a delegate.
- `pkg/ops/baseline.go` — new: the result types, the reader, the delta arithmetic.
- `pkg/ops/rollup_weekly.go` — modify: one result field, one constructor alongside the existing one, one struct field, one wiring block in `Execute`.
- `pkg/ops/rollup_weekly_test.go` — modify: the baseline fixture helper and the specs.

Nothing else. Do NOT touch `pkg/config/`, `pkg/cli/`, `integration/`, `pkg/storage/base_test.go`, `pkg/storage/export_test.go`, or `CHANGELOG.md`. Do NOT change the `RollupWeeklyOperation` interface or the `Execute` signature — prompt 3's CLI call site depends on the 4-argument form. Do NOT add a scenario file (`ls scenarios/*.md | wc -l` must still print `6`).

## 1. `pkg/storage/base.go` — export the frontmatter parser

Move the body of `func (b *baseStorage) parseToFrontmatterMap(ctx context.Context, content []byte) (map[string]any, error)` into a new exported package-level function and leave the method as a one-line delegate, so `pkg/storage/export_test.go`'s `ParseToFrontmatterMapForTest` and `pkg/storage/base_test.go` keep passing untouched:

```go
// ParseFrontmatterMap parses the YAML frontmatter block from content into a
// map[string]any, preserving all fields including unknown ones.
// Returns an error if no frontmatter block is found or YAML is invalid.
// A bare Obsidian wikilink value is quoted before unmarshal — see
// quoteBareWikilinks — so it is read as a string rather than a nested list.
func ParseFrontmatterMap(ctx context.Context, content []byte) (map[string]any, error) {
	// the existing body, verbatim
}

// parseToFrontmatterMap is the baseStorage method form of ParseFrontmatterMap.
func (b *baseStorage) parseToFrontmatterMap(
	ctx context.Context,
	content []byte,
) (map[string]any, error) {
	return ParseFrontmatterMap(ctx, content)
}
```

Keep the moved body byte for byte, including the `frontmatterRegex` match, the `quoteBareWikilinks` call, the `errors.Errorf(ctx, "no frontmatter found")` branch and the `m == nil` guard. Do not change `frontmatterRegex`. Do not delete the method — `pkg/storage/export_test.go` and `pkg/storage/base_test.go` reference it.

## 2. `pkg/ops/baseline.go` — the types, the reader and the delta arithmetic

New file, `package ops`, BSD license header matching its siblings. Imports: `context`, `fmt`, `os`, `strconv`, `time`, `github.com/bborbe/errors`, `github.com/bborbe/vault-cli/pkg/storage`.

### 2a. The frozen key names

```go
// The baseline file's frontmatter keys. They are frozen: the file is authored by
// hand outside the code and read here, so renaming one is a behaviour change, not
// a style choice. docs/baseline-file.md is the authority for the key set.
const (
	rollupBaselineKeyCaptured      = "baseline_captured"
	rollupBaselineKeyHumanTotal    = "baseline_human_total"
	rollupBaselineKeyMedian        = "baseline_median"
	rollupBaselineKeyWeeks         = "baseline_weeks"
	rollupBaselineKeyAgentCoverage = "baseline_agent_coverage"
)
```

### 2b. The result types

```go
// RollupBaseline is a vault's stored baseline: the five figures the file records,
// echoed verbatim, plus the delta from each computed figure to its analogue here.
// It is nil on a RollupWeeklyResult whose vault has no baseline configured.
type RollupBaseline struct {
	Captured      string               `json:"captured"`
	HumanTotal    int                  `json:"human_total"`
	Median        int                  `json:"median"`
	Weeks         map[string]int       `json:"weeks"`
	AgentCoverage string               `json:"agent_coverage"`
	Deltas        RollupBaselineDeltas `json:"deltas"`
}

// RollupBaselineDeltas holds one row per computed figure that has a baseline
// analogue. A figure with no analogue — the unattended-delivery count, the stored
// total, the stored agent coverage — has no field here, and a figure whose row is
// omitted (the requested week is absent from Weeks, or the computed figure is not
// a number) leaves its pointer nil.
type RollupBaselineDeltas struct {
	HumanInteractions *RollupBaselineDelta `json:"human_interactions,omitempty"`
	PerFamilyMedian   *RollupBaselineDelta `json:"per_family_median,omitempty"`
}

// RollupBaselineDelta is one computed-versus-stored movement. Computed is the
// rollup's own figure parsed from its rendered string, Baseline the stored
// analogue, Delta their difference, and Mismatch marks a row whose two figures
// measure different things — the JSON form of the plain report's frozen
// `[definitional mismatch]` marker.
type RollupBaselineDelta struct {
	Computed float64 `json:"computed"`
	Baseline float64 `json:"baseline"`
	Delta    float64 `json:"delta"`
	Mismatch bool    `json:"mismatch,omitempty"`
}
```

`float64` rather than `int` for the delta fields is deliberate: the computed per-family median can be fractional, and the stored week figures and median are integers that render identically either way.

### 2c. The reader

```go
// readRollupBaseline reads and validates the baseline file at resolvedPath for
// vaultName. Every figure is echoed as the file stores it: none is recomputed,
// rounded, reformatted or normalised. A missing or unreadable file, a frontmatter
// block that will not parse, a required key that is absent, and a value of the
// wrong shape are each an error naming the file and the key.
func readRollupBaseline(
	ctx context.Context,
	resolvedPath string,
	vaultName string,
) (*RollupBaseline, error)
```

Steps, in this order:

1. `content, err := os.ReadFile(resolvedPath)` with `//#nosec G304 -- user-controlled vault path` on that line, mirroring `pkg/storage/base.go` line 311. On error return `errors.Wrapf(ctx, err, "read baseline file %s of vault %s", resolvedPath, vaultName)`.
2. `m, err := storage.ParseFrontmatterMap(ctx, content)`; on error return `errors.Wrapf(ctx, err, "parse baseline file %s of vault %s", resolvedPath, vaultName)`.
3. Extract the five keys with small named helpers, each of which takes the map, the key and a context string carrying `resolvedPath` and `vaultName`, and returns the value or an error naming the key. Use the **comma-ok** form for every type assertion (`forcetypeassert` is enabled):
   - `rollupBaselineCaptured(ctx, m, where)` — `v, ok := m[key]`; `!ok` → `errors.Errorf(ctx, "baseline file %s is missing required key %s", where, key)`. Then a type switch: `time.Time` → `v.Format("2006-01-02")`; `string` → `v` verbatim when non-empty, an error naming the key when empty; anything else (including a nil value) → an error naming the key. The `time.Time` case is load-bearing: the contract's example writes the date unquoted, so YAML resolves it as a timestamp. Never `fmt.Sprintf("%v", v)` on the raw value — that renders `2026-09-12 00:00:00 +0000 UTC`.
   - `rollupBaselineInt(ctx, m, key, where)` — require `int`; `!ok` or any other type → an error naming the key. Never coerce a `float64` or a `string` into a number.
   - `rollupBaselineWeeks(ctx, m, where)` — require `map[string]any`; iterate it and require every value to be `int`, an error naming both `baseline_weeks` and the offending week token otherwise. Build the `map[string]int` result.
   - `rollupBaselineString(ctx, m, key, where)` — require `string`; an error naming the key otherwise.
4. Return the assembled `*RollupBaseline`. Leave `Deltas` at its zero value — the caller fills it after the figures are computed.

Every error message must contain the key name verbatim (`baseline_median`), the resolved path and the vault name; the acceptance criteria grep stderr for the key name. Nothing in this function may panic on a hostile or malformed value: a type switch, not an unchecked assertion, and never a `strconv` call on an unchecked input.

### 2d. The analogue mapping and the delta arithmetic

```go
// rollupBaselineDeltas maps the rollup's computed figures to their baseline
// analogues. The mapping is fixed: HumanInteractions maps to the baseline's figure
// for the requested week when that week is a key of Weeks; PerFamilyMedian maps to
// Median and its row is marked a definitional mismatch, because the stored figure
// is a per-task median while the computed one is a median of per-family medians.
// A computed figure that is not a number — `no data`, `no recorded counts`,
// `undefined` — produces no row at all, and never a row reading zero.
func rollupBaselineDeltas(
	baseline *RollupBaseline,
	weekToken string,
	computedHumanInteractions string,
	computedPerFamilyMedian string,
) RollupBaselineDeltas
```

Body:

```go
	deltas := RollupBaselineDeltas{}
	if computed, err := strconv.ParseFloat(computedHumanInteractions, 64); err == nil {
		if stored, ok := baseline.Weeks[weekToken]; ok {
			deltas.HumanInteractions = &RollupBaselineDelta{
				Computed: computed,
				Baseline: float64(stored),
				Delta:    computed - float64(stored),
			}
		}
	}
	if computed, err := strconv.ParseFloat(computedPerFamilyMedian, 64); err == nil {
		deltas.PerFamilyMedian = &RollupBaselineDelta{
			Computed: computed,
			Baseline: float64(baseline.Median),
			Delta:    computed - float64(baseline.Median),
			Mismatch: true,
		}
	}
	return deltas
```

- `Unattended deliveries` has no analogue: do not add a field for it and do not compute anything for it.
<!-- OPEN QUESTION: the spec pins the omission of a delta row for a week absent from `baseline_weeks` (DB 5) but does not pin what happens when the week IS present and the computed figure is a sentinel — `no data`, `no recorded counts`, `undefined`. A delta cannot be computed against a non-number, and the spec's "never a row reading 0" rule forbids substituting one. This prompt commits to omitting the row, extending DB 5's rule to the computed side. -->
- `baseline_human_total` and `baseline_agent_coverage` have no analogue: same.
- `Mismatch` is `true` on the median row only; `omitempty` keeps it out of the human-interactions object.
- A week absent from `Weeks` leaves `HumanInteractions` nil — never a delta of zero.

Also add:

```go
// rollupBaselineWeekToken renders a resolved ISO year and week as the YYYY-Wnn
// token used as a key of the baseline's week map. The week is zero-padded to two
// digits, matching the contract's keys.
func rollupBaselineWeekToken(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}
```

## 3. `pkg/ops/rollup_weekly.go` — carry the baseline

Four edits, and nothing else in the file.

### 3a. One field on the result

Add to `RollupWeeklyResult`, as the LAST field, after `Families`:

```go
	Baseline             *RollupBaseline `json:"baseline,omitempty"`
```

A pointer with `omitempty` is what makes a vault with no baseline serialise without a `baseline` key at all. Do not add the field to `RollupFamily`, and do not change any existing tag.

### 3b. One field on the operation

```go
type rollupWeeklyOperation struct {
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
	baselinePath    string
}
```

### 3c. One constructor alongside the existing one

Keep `NewRollupWeeklyOperation(taskStorage, currentDateTime) RollupWeeklyOperation` exactly as it is today in signature and doc comment, and make it delegate:

```go
func NewRollupWeeklyOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
) RollupWeeklyOperation {
	return NewRollupWeeklyOperationWithBaseline(taskStorage, currentDateTime, "")
}

// NewRollupWeeklyOperationWithBaseline creates a weekly rollup operation that also
// reads the baseline file at baselinePath. baselinePath is vault-relative and is
// resolved against the vault path passed to Execute; an empty baselinePath means
// no baseline, and the result then carries no Baseline and no delta rows, exactly
// as before this feature existed.
func NewRollupWeeklyOperationWithBaseline(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
	baselinePath string,
) RollupWeeklyOperation {
	return &rollupWeeklyOperation{
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
		baselinePath:    baselinePath,
	}
}
```

Neither constructor may validate, read or stat anything: a factory is pure composition. Validation and I/O belong in `Execute`.

### 3d. One wiring block in `Execute`

Leave the `RollupWeeklyOperation` interface, its `//counterfeiter:generate` annotation and the `Execute` signature untouched — the CLI's call site in prompt 3 depends on the 4-argument form, and `mocks/rollup-weekly-operation.go` must stay valid as generated.

In `Execute`, after `resolveWeek` succeeds and **before** `ListTasksStrict`, read the baseline so a misconfigured baseline fails before any figure is computed:

```go
	if o.baselinePath != "" {
		baseline, err := readRollupBaseline(
			ctx, filepath.Join(vaultPath, o.baselinePath), vaultName,
		)
		if err != nil {
			return RollupWeeklyResult{}, err
		}
		result.Baseline = baseline
	}
```

The path is joined against the vault root and nothing else: `filepath.Join(vaultPath, o.baselinePath)`. Validating it is NOT this prompt's job — the CLI rejects an absolute or escaping path before it constructs the operation (prompt 3), and this prompt must stay independent of prompt 1. Do not import `pkg/config`.

Then, after `result.PerFamilyMedian` has been assigned and just before the final `return result, nil`, fill the deltas:

```go
	if result.Baseline != nil {
		result.Baseline.Deltas = rollupBaselineDeltas(
			result.Baseline,
			rollupBaselineWeekToken(year, weekNumber),
			result.HumanInteractions,
			result.PerFamilyMedian,
		)
	}
```

- The empty-week `no data` branch returns early, so a block placed only before the final `return result, nil` is not reached by it. That is fine — `Deltas` then keeps its zero value, whose nil pointers produce no rows, which is the required `no data` outcome. Place the block once, immediately before the final `return result, nil`; do not duplicate it into the early-return branch.
- `Execute` must never write: no `os.WriteFile`, no `os.Create`, no `os.Rename` anywhere in `pkg/ops/baseline.go` or `pkg/ops/rollup_weekly.go`.
- Add the import `path/filepath` to `pkg/ops/rollup_weekly.go`, and `path/filepath` plus `os` to `pkg/ops/baseline.go`. Do NOT import `github.com/bborbe/vault-cli/pkg/config` in either file — this prompt is independent of prompt 1.
- If `Execute` grows past `funlen`'s 80 lines / 50 statements, extract the baseline block into a small unexported method on `*rollupWeeklyOperation` rather than raising a linter limit.

## 4. `pkg/ops/rollup_weekly_test.go` — the fixture suite

Extend the existing file. **One `func TestSuite` in `pkg/ops/ops_suite_test.go`; do not add a second `func Test*` calling `RunSpecs`.** Keep every existing spec and helper unchanged, and keep the existing 4-argument `Execute` call form.

Add two helpers next to `writeRollupTask`:

```go
// writeRollupBaseline writes a baseline file at a vault-relative path. The
// frontmatter argument is the raw YAML block between the --- delimiters.
func writeRollupBaseline(vaultPath string, relPath string, frontmatter string) {
	content := "---\n" + frontmatter + "---\n# Baseline\n\nCaptured figures.\n"
	full := filepath.Join(vaultPath, relPath)
	Expect(os.MkdirAll(filepath.Dir(full), 0755)).To(Succeed())
	Expect(os.WriteFile(full, []byte(content), 0600)).To(Succeed())
}

// baselineFrontmatter renders a baseline frontmatter block from its five figures.
// weeks is the pre-rendered body of baseline_weeks, e.g. "  2026-W36: 25141\n".
func baselineFrontmatter(
	captured string, humanTotal int, median int, weeks string, agentCoverage string,
) string {
	return "baseline_captured: " + captured + "\n" +
		"baseline_human_total: " + strconv.Itoa(humanTotal) + "\n" +
		"baseline_median: " + strconv.Itoa(median) + "\n" +
		"baseline_weeks:\n" + weeks +
		"baseline_agent_coverage: \"" + agentCoverage + "\"\n"
}
```

Use these spec names verbatim; they are grep targets in `<verification>`. Build the operation under test with `ops.NewRollupWeeklyOperationWithBaseline(taskStore, currentDateTime, relPath)` inside each baseline spec, and keep the existing `rollupOp` built with the two-argument constructor for the no-baseline spec.

1. `"reads the baseline figures from the configured file"` — a baseline file carrying the real 2026-09-12 figures (`baseline_captured: 2026-09-12`, `baseline_human_total: 62485`, `baseline_median: 64`, `baseline_weeks` with `2026-W36: 25141` and `2026-W37: 26476`, `baseline_agent_coverage: "1 of 420"`) and a W37 task. Assert `result.Baseline.Captured == "2026-09-12"`, `HumanTotal == 62485`, `Median == 64`, `Weeks["2026-W36"] == 25141`, `Weeks["2026-W37"] == 26476`, `AgentCoverage == "1 of 420"`. The unquoted date is the point: it proves the parse renders the YAML timestamp as `YYYY-MM-DD` rather than a `time.Time` string.
2. `"reads distinct figures and carries no compiled-in constant"` — a second fixture carrying `2026-01-02`, `111`, `7`, `2026-W36: 222` / `2026-W37: 333`, `2 of 9`. Assert those exact values, assert `HumanTotal != 62485`, and assert `Weeks` contains neither `25141` nor `26476`. A hardcoded constant fails this spec.
3. `"computes the human-interactions delta from the week's stored figure"` — baseline `2026-W37: 222`, and a W37 task set whose recorded counts sum to a value you hand-compute (for example two tasks with counts `300` and `200`). Assert `result.HumanInteractions == "500"`, `Deltas.HumanInteractions.Computed == 500`, `.Baseline == 222`, `.Delta == 278`.
4. `"computes the per-family median delta and marks it a definitional mismatch"` — baseline median `7`; assert `Deltas.PerFamilyMedian.Baseline == 7`, `.Computed` equals the parsed `result.PerFamilyMedian`, `.Delta == Computed - 7`, and `.Mismatch` is true.
5. `"omits the human-interactions delta when the requested week is absent from the baseline"` — baseline weeks carry `2026-W37` only; run `--week 2026-W20`. Assert `Deltas.HumanInteractions` is nil, `Deltas.PerFamilyMedian` is not nil, and `result.HumanInteractions` is still a real figure (the omission is scoped to the delta, not to the computed block).
6. `"omits a delta row when the computed figure is not a number"` — a W37 task set whose counts are all absent, so `result.HumanInteractions` reads `no recorded counts` and `result.PerFamilyMedian` reads `undefined`. Assert both delta pointers are nil, and assert neither is a `RollupBaselineDelta` with a zero `Delta`.
7. `"carries no baseline when the operation is built without one"` — the existing two-argument constructor and a vault whose baseline file exists at a path the operation was never told about; assert `result.Baseline` is nil.
8. `"fails naming the vault and the resolved path when the baseline file is missing"` — a baseline path inside the vault with no file at it. Assert `err` is non-nil, the message contains the vault name and the resolved absolute path, and `result` carries no figures.
9. `"fails naming the missing frontmatter key"` — a baseline file with `baseline_median` removed. Assert the error message contains `baseline_median`.
10. `"fails naming the key when a baseline figure is not an integer"` — `baseline_median: not-a-number`. Assert the error message contains `baseline_median`, and assert it does not panic.
11. `"fails naming the key when baseline_weeks carries a non-integer"` — `2026-W37: many`. Assert the error message contains `baseline_weeks` and the offending week token.
12. `"marshals the baseline figures and the deltas under a baseline key"` — `json.Marshal(result)` for the real-figures fixture, unmarshalled into a `map[string]any`; assert `baseline.captured == "2026-09-12"`, `baseline.human_total == float64(62485)`, `baseline.median == float64(64)`, `baseline.agent_coverage == "1 of 420"`, `baseline.weeks["2026-W37"] == float64(26476)`, and that the median delta object carries `mismatch == true` while the human-interactions delta object has no `mismatch` key. `encoding/json` in a `_test.go` file is fine — the "no `encoding/json` in a command file" rule applies to `pkg/cli/*.go` non-test files.
13. `"omits the baseline key from the JSON when no baseline is configured"` — marshal the result of the two-argument constructor and assert the unmarshalled map has no `baseline` key at all (`_, ok := m["baseline"]; Expect(ok).To(BeFalse())`).

## 5. Failure modes and security — what each row carries

Map the spec's Failure Modes table onto this change and state the mapping in your completion report:

- **`baseline` key set, file missing or unreadable** → `os.ReadFile` fails and the error names the vault and the resolved path; `Execute` returns the error before any figure exists, so nothing prints. Covered by spec 8.
- **`baseline` key set, file present, a required frontmatter key absent** → the extraction helper's `!ok` branch names the key. Covered by spec 9.
- **A baseline figure is non-numeric or malformed** → the explicit `int` type check names the key; nothing is coerced and nothing panics. Covered by specs 10 and 11.
- **Requested week absent from `baseline_weeks`** → the baseline block still carries the stored figures; `Deltas.HumanInteractions` is nil and no row reading `0` is produced. Covered by spec 5.
- **The configured path is absolute or escapes the vault root** → the CLI rejects it before this operation is even constructed, so no figures are produced and no file is opened. That row belongs to prompt 3 (its integration spec 7); this prompt never receives an unvalidated path, which is why it does not import `pkg/config`.
- **A computed figure is a sentinel rather than a number** (`no data`, `no recorded counts`, `undefined`) → no delta row. This is the spec's "never a row reading 0" rule extended to the computed side; covered by spec 6.
- **Clock skew / timezone** → no effect; the capture date is stored text echoed verbatim and no baseline figure is date-arithmetic. The injected `currentDateTime` is used only to resolve a default week, exactly as before.
- **Two `config set-baseline` calls race** → operator-side, prompt 1's concern; nothing to build here.

Security: the untrusted input is the baseline file's content. Parse frontmatter, never line-grep; a hostile or malformed value must be an error naming the key, never a coerced number and never a panic. `rollup weekly` never writes — assert this yourself by grepping for `os.WriteFile`, `os.Create` and `os.Rename` in `pkg/ops/`. The command takes no free-form input beyond a week token, a vault name and the config-supplied baseline path, all validated before use. No network, no credentials, no external system, no retry loop, no `os/exec`.

## 6. Self-check before finishing

- Re-read the changed hunks and confirm: the `RollupWeeklyOperation` interface and `Execute` signature are unchanged; `NewRollupWeeklyOperation` still has its original signature and delegates; the delta block runs once, before the single final `return`; `pkg/ops/baseline.go` and `pkg/ops/rollup_weekly.go` contain no write call; `pkg/config/`, `pkg/cli/`, `integration/` and `CHANGELOG.md` are untouched.
- Walk `docs/dod.md`: exported types have doc comments, errors use `github.com/bborbe/errors` with context wrapping, no `fmt.Print*` or `os.Stdout` was added to `pkg/ops/`, tests are Ginkgo v2 / Gomega in the external `ops_test` package.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 054 — the baseline is stored text, echoed verbatim.** None of the five figures is derived or recomputed. The recorded definitional mismatches are surfaced, not reconciled: the stored `26,476` prints next to the computed figure and the reader sees the capture-window gap. Never normalise one to the other.
- **Copied from spec 054 — the read set is `tasks_dir` frontmatter plus one config-named baseline file.** The baseline is a *presentation* input outside the computation read set: it contributes no datum to any computed figure. Deleting it changes the output's presentation and never a computed number. No data file, no cache, no sidecar.
- **Copied from spec 054 — the rollup never writes.** No vault file, no baseline file, no config file. Only `config set-baseline` writes, and it writes only the config file, atomically.
- **Copied from spec 054 — backward compatibility is exact, not approximate.** With no `baseline` key the output is byte-identical to the pre-change output. Every existing spec in `pkg/ops/rollup_weekly_test.go` must keep passing unchanged, and the existing 4-argument `Execute` call form must keep compiling.
- **Copied from spec 054 — no delta for a figure with no analogue.** `Unattended deliveries`, the baseline's `total` and its `agent coverage` produce no delta row. A figure with no analogue must not gain a field "for symmetry".
- **Copied from spec 054 — never a row reading `0`.** An absent stored week and a non-numeric computed figure both omit the row; nothing is printed or serialised in its place.
- **Copied from spec 054 — frontmatter parsing, never line-grepping.** A hostile or malformed value is an error naming the key, never parsed into a number and never able to panic the scan.
- **Copied from spec 054 — no scenario file.** The spec's Non-goals rule one out (the four-condition scenario test fails on its first condition, matching spec 049's decision). `ls scenarios/*.md | wc -l` must still print `6`.
- **Copied from spec 054 — Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- **Do NOT write the changelog.** `CHANGELOG.md` is prompt 3's alone. Do not bump any version string and do not create a tag: `.maintainer.yaml` sets `release.autoRelease: true` and the post-merge releaser owns version bumps.
- **Do NOT add a dependency** and do NOT run `go mod vendor`; never write `-mod=vendor` in a verification command. `gopkg.in/yaml.v3` is already a direct dependency. Do NOT import `github.com/bborbe/vault-cli/pkg/config` — this prompt must compile and pass on its own, before prompt 1 lands.
- **Do NOT add a knob.** No `--baseline` flag, no config key, no environment variable, no threshold, no `--refresh`, no baseline-writing verb. The spec's Non-goals forbid refreshing the baseline and forbid more than one baseline per vault.
- **Tests.** Ginkgo v2 + Gomega in the external `ops_test` package; one `func TestSuite` entry point, never a second `RunSpecs`. Every new Go file keeps its BSD license header.
- **Linter limits** the new files must respect: `funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20, `gocyclo` default, `forcetypeassert` (comma-ok form on every assertion). Split `readRollupBaseline` into the small named helpers section 2c describes rather than growing one function past the limit.
- **Do NOT run** `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. The daemon does not check `<verification>` exit codes — read each line's exit status yourself and report `"status":"failed"` if any is non-zero. The checks below are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, so a check that silently stops matching still surfaces as a non-zero exit.

**The focused suites, with their frozen spec names.** `-args -ginkgo.v -ginkgo.no-color` is mandatory: Ginkgo v2's default reporter prints only dots on a green run, so without `-ginkgo.v` every name grep below returns zero matches against a correct suite, and with colour on Ginkgo injects ANSI escapes between the container and It texts. Never pipe a test command.

```
go test ./pkg/ops/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/spec054-ops.log 2>&1; test "$?" = "0"
grep -F -q -- 'reads the baseline figures from the configured file' /tmp/spec054-ops.log
grep -F -q -- 'reads distinct figures and carries no compiled-in constant' /tmp/spec054-ops.log
grep -F -q -- "computes the human-interactions delta from the week's stored figure" /tmp/spec054-ops.log
grep -F -q -- 'computes the per-family median delta and marks it a definitional mismatch' /tmp/spec054-ops.log
grep -F -q -- 'omits the human-interactions delta when the requested week is absent from the baseline' /tmp/spec054-ops.log
grep -F -q -- 'omits a delta row when the computed figure is not a number' /tmp/spec054-ops.log
grep -F -q -- 'carries no baseline when the operation is built without one' /tmp/spec054-ops.log
grep -F -q -- 'fails naming the vault and the resolved path when the baseline file is missing' /tmp/spec054-ops.log
grep -F -q -- 'fails naming the missing frontmatter key' /tmp/spec054-ops.log
grep -F -q -- 'fails naming the key when a baseline figure is not an integer' /tmp/spec054-ops.log
grep -F -q -- 'fails naming the key when baseline_weeks carries a non-integer' /tmp/spec054-ops.log
grep -F -q -- 'marshals the baseline figures and the deltas under a baseline key' /tmp/spec054-ops.log
grep -F -q -- 'omits the baseline key from the JSON when no baseline is configured' /tmp/spec054-ops.log
```

**The storage parser still works after the export, and its suite is untouched:**

```
go test ./pkg/storage/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/spec054-storage.log 2>&1; test "$?" = "0"
grep -F -q 'func ParseFrontmatterMap' pkg/storage/base.go
grep -F -q 'return ParseFrontmatterMap(ctx, content)' pkg/storage/base.go
test "$(grep -c 'func ParseFrontmatterMap' pkg/storage/base.go)" = "1"
test "$(grep -c 'func (b \*baseStorage) parseToFrontmatterMap' pkg/storage/base.go)" = "1"
```

**The frozen keys, the analogue mapping and the JSON shape are present:**

```
grep -F -q 'baseline_captured' pkg/ops/baseline.go
grep -F -q 'baseline_human_total' pkg/ops/baseline.go
grep -F -q 'baseline_median' pkg/ops/baseline.go
grep -F -q 'baseline_weeks' pkg/ops/baseline.go
grep -F -q 'baseline_agent_coverage' pkg/ops/baseline.go
grep -F -q 'Mismatch bool' pkg/ops/baseline.go
grep -F -q 'json:"human_interactions,omitempty"' pkg/ops/baseline.go
grep -F -q 'json:"per_family_median,omitempty"' pkg/ops/baseline.go
grep -F -q 'func readRollupBaseline' pkg/ops/baseline.go
grep -F -q 'func rollupBaselineDeltas' pkg/ops/baseline.go
grep -F -q 'func rollupBaselineWeekToken' pkg/ops/baseline.go
grep -F -q 'json:"baseline,omitempty"' pkg/ops/rollup_weekly.go
grep -F -q 'func NewRollupWeeklyOperationWithBaseline' pkg/ops/rollup_weekly.go
```

**The interface and the mock are unchanged, and the operation still never writes:**

```
make generate
test -f mocks/rollup-weekly-operation.go
grep -F -q 'Execute(arg1 context.Context, arg2 string, arg3 string, arg4 string)' mocks/rollup-weekly-operation.go
test "$(grep -c 'counterfeiter:generate -o ../../mocks/rollup-weekly-operation.go' pkg/ops/rollup_weekly.go)" = "1"
test "$(grep -c 'vault-cli/pkg/config' pkg/ops/baseline.go)" = "0"
test "$(grep -c 'vault-cli/pkg/config' pkg/ops/rollup_weekly.go)" = "0"
test "$(grep -c 'os.WriteFile\|os.Create\|os.Rename' pkg/ops/baseline.go)" = "0"
test "$(grep -c 'os.WriteFile\|os.Create(\|os.Rename' pkg/ops/rollup_weekly.go)" = "0"
test "$(grep -c 'fmt.Errorf' pkg/ops/baseline.go)" = "0"
test "$(grep -c 'context.Background()' pkg/ops/baseline.go)" = "0"
```

The `os.WriteFile`/`os.Create`/`os.Rename` lines are the spec's "the rollup never writes" constraint expressed as a check. If any of them reports a non-zero count, the constraint is violated regardless of what the suite says.

The `mocks/` line is the interface guard: the generated mock still declares `Execute` with exactly four parameters, so a widened `Execute` signature — which would break prompt 3's call site — shows up here. The `pkg/config` lines are the independence guard: this prompt must compile before prompt 1 lands. 

**The ops package still has exactly one suite entry point:**

```
test "$(grep -l 'func TestSuite' pkg/ops/*_test.go | wc -l | tr -d ' ')" = "1"
test "$(grep -c 'RunSpecs' pkg/ops/rollup_weekly_test.go)" = "0"
```

A second `RunSpecs` in this package panics the whole suite, which is why this is checked rather than assumed.

**Nothing out of scope was touched:**

```
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "6"
test "$(grep -cE '^- feat:.*baseline' CHANGELOG.md)" = "0"
test "$(grep -c 'Baseline' pkg/cli/rollup.go)" = "0"
test "$(grep -c 'RollupBaseline\|readRollupBaseline' pkg/config/config.go)" = "0"
test "$(grep -c 'baseline' pkg/config/config.go)" -ge 1
```

The first added line fails if the *reader* was added here rather than in prompt 3. The second confirms prompt 1's `baseline` config key is present: prompts run in sequence on the same branch under `workflow: direct`, so it must be there by now, and a `0` here means prompt 1 did not land.

**The whole suite, then formatting:**

```
make test
test -z "$(gofmt -e -l pkg/storage/base.go pkg/ops/baseline.go pkg/ops/rollup_weekly.go pkg/ops/rollup_weekly_test.go 2>&1)"
```

If a check fails, fix the cause and re-run only that check. Do NOT run `make precommit` in this prompt — prompt 3 lands last and owns the full gate. In your completion report, walk the spec's Desired Behaviors 2, 4 and 7 and the Failure Modes mapping from section 5.
</verification>
