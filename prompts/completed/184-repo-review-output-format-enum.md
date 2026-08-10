---
status: completed
summary: Typed OutputFormat with Validate/IsJSON/IsPlain methods, added validation to PersistentPreRunE, replaced all 48 constant comparisons with helper method calls
execution_id: repo-exec-184-repo-review-output-format-enum
dark-factory-version: v0.193.0
created: "2026-08-10T12:36:04Z"
queued: "2026-08-10T12:36:04Z"
started: "2026-08-10T12:43:11Z"
completed: "2026-08-10T12:47:15Z"
---

<summary>
- Passing a misspelled `--output` value currently prints plain output and says nothing; it now fails with a clear error listing the valid values.
- The two format constants become a proper type with a list of valid values and a validation method, matching the pattern the domain package already uses for statuses and phases.
- Comparisons throughout the CLI switch to a small helper method, so the intent reads the same everywhere.
</summary>

<objective>
Finding M5: `OutputFormatPlain` / `OutputFormatJSON` are untyped string constants compared 47× across the CLI (42 against JSON, 5 against Plain) with no validation, so an invalid `--output` value silently falls through to plain output. Give them a type, a collection, validation, and wire the validation to the flag.
</objective>

<context>
Read these files IN FULL before editing:
- `/workspace/pkg/cli/output.go` — declares the two constants.
- `/workspace/pkg/domain/task_status.go` — **the pattern to copy.** It has `type TaskStatus string` (line 16), `AvailableTaskStatuses` (line 39), `type TaskStatuses []TaskStatus` (line 49) and `func (s TaskStatus) Validate(ctx context.Context) error` (line 62). Mirror this shape exactly.
- `/workspace/pkg/cli/cli.go` — `var outputFormat string` (~line 81) bound by `StringVar(&outputFormat, "output", OutputFormatPlain, …)` (~line 107), threaded to subcommands as `*string`. It is compared against `OutputFormatJSON` at 42 sites **and against `OutputFormatPlain` at 5 sites** — both must be converted.
</context>

<requirements>
1. In `/workspace/pkg/cli/output.go`, mirroring `pkg/domain/task_status.go`:
   - `type OutputFormat string`
   - retype both constants to `OutputFormat`
   - `type OutputFormats []OutputFormat` with a `Contains(OutputFormat) bool` method
   - `var AvailableOutputFormats = OutputFormats{OutputFormatPlain, OutputFormatJSON}`
   - `func (f OutputFormat) Validate(ctx context.Context) error` — returns an error naming the invalid value and listing the valid ones when not in `AvailableOutputFormats`. Use `github.com/bborbe/errors`, matching how `TaskStatus.Validate` builds its error.
   - `func (f OutputFormat) IsJSON() bool { return f == OutputFormatJSON }`
   - `func (f OutputFormat) IsPlain() bool { return f == OutputFormatPlain }`
   - GoDoc on every exported identifier, starting with the identifier name.
2. In `/workspace/pkg/cli/cli.go`:
   - keep `var outputFormat string` and the existing `StringVar` binding — cobra needs a `*string`. Change the default argument to `string(OutputFormatPlain)`.
   - **validate once after flag parsing**: in the root command's `PersistentPreRunE` (add one if absent), call `OutputFormat(outputFormat).Validate(ctx)` and return the error. This is the behaviour change that makes an invalid `--output` fail loudly.
   - replace **every** comparison against **either** constant. `outputFormat` stays a plain `string`, so once the constants are typed these will not compile unless all are converted. There are exactly 48 occurrences: **35** `== OutputFormatJSON`, **7** `!= OutputFormatJSON`, **5** `== OutputFormatPlain`, and 1 `StringVar` default.
     - `outputFormat == OutputFormatJSON` → `OutputFormat(outputFormat).IsJSON()`
     - `*outputFormat == OutputFormatJSON` → `OutputFormat(*outputFormat).IsJSON()`
     - `*outputFormat != OutputFormatJSON` → `!OutputFormat(*outputFormat).IsJSON()`
     - `*outputFormat == OutputFormatPlain` → `OutputFormat(*outputFormat).IsPlain()` — **5 sites, around lines 450, 594, 764, 1885 and 1932**, each of the shape `if len(vaults) > 1 && *outputFormat == OutputFormatPlain`. Missing these is a compile error, not a style lapse.
3. Add a test to `/workspace/pkg/cli/` covering `Validate`: one valid value and one invalid value that returns an error. Follow the existing Ginkgo v2 / Gomega style in that package.
4. Do NOT change any other function signature. Findings M4 and S4 are OUT OF SCOPE.
5. Do NOT touch any file marked `// Code generated ... DO NOT EDIT.`
6. Add a bullet to the `## Unreleased` section of `/workspace/CHANGELOG.md` (create it at the top if absent), noting that an invalid `--output` value now errors instead of silently printing plain.
</requirements>

<verification>
- `cd /workspace && make precommit` exits 0.
- `grep -n 'type OutputFormat string' /workspace/pkg/cli/output.go` returns one match.
- `grep -n 'AvailableOutputFormats' /workspace/pkg/cli/output.go` returns at least one match.
- `grep -n 'func (f OutputFormat) Validate' /workspace/pkg/cli/output.go` returns one match.
- `grep -c '== OutputFormatJSON' /workspace/pkg/cli/cli.go` returns 0.
- `grep -c '== OutputFormatPlain' /workspace/pkg/cli/cli.go` returns 0 — the `StringVar` default uses `string(OutputFormatPlain)`, which is not a comparison.
- `grep -c 'IsPlain()' /workspace/pkg/cli/cli.go` is at least 5.
- `grep -c 'IsJSON()' /workspace/pkg/cli/cli.go` is at least 40.
- `grep -n 'PersistentPreRunE' /workspace/pkg/cli/cli.go` returns at least one match.
- `cd /workspace && git diff --name-only` lists ONLY files under `pkg/cli/` plus `CHANGELOG.md`.
</verification>

<allowed_files>
- /workspace/pkg/cli/output.go
- /workspace/pkg/cli/cli.go
- /workspace/pkg/cli/output_test.go
- /workspace/CHANGELOG.md
</allowed_files>
