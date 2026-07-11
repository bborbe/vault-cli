---
status: approved
spec: [024-goal-phase-field]
created: "2026-07-11T21:30:00Z"
queued: "2026-07-11T21:28:32Z"
branch: dark-factory/goal-phase-field
---

<summary>
- Goals can now carry a `phase` frontmatter field, set through the existing `goal set <name> phase <value>` command — no new command is added.
- Setting a canonical phase (`todo` / `planning` / `execution` / `done`) writes `phase: <value>` into the goal file and it survives read-write cycles.
- Setting a non-canonical phase (e.g. `bogus`) fails with a non-zero exit and an error naming the offending value; the goal file is left untouched.
- Reading a goal through `goal show` surfaces the phase in both plain and `--output json` output when present.
- Goals that predate this field keep parsing, showing, and accepting unrelated edits with no error and no injected phase value — nothing is backfilled.
- A hand-typed legacy value (e.g. `phase: in_progress`) is tolerated on read/display but rejected on an explicit re-set.
- Adds a CHANGELOG entry and a behavioral verification pass against a scratch goal file.
</summary>

<objective>
Wire the goal-phase field into the existing goal frontmatter so `goal set <name> phase <value>` validates and persists a canonical `GoalPhase`, and `goal show <name>` (plain and `--output json`) surfaces it. Add a typed `Phase()` getter and `SetPhase(*GoalPhase)` setter on `GoalFrontmatter`, plus `phase` cases in `GetField`/`SetField`. No new command, no new ops/CLI code — this rides the existing generic goal set/show wiring. Then add a CHANGELOG entry.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable`, external `_test` package.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` + `github.com/bborbe/validation` sentinel.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format and prefixes.

Read these files before implementing:
- `pkg/domain/goal_phase.go` — created in prompt 1: `GoalPhase` newtype, `GoalPhaseTodo/Planning/Execution/Done`, `AvailableGoalPhases`, `GoalPhases.Contains`, `GoalPhase.Validate(ctx)`, `GoalPhase.Ptr()`. If this file does NOT exist yet, STOP and report `status: failed` with message "goal phase type not yet deployed (prompt 1)" — do NOT create it here.
- `pkg/domain/goal_frontmatter.go` — the file you extend. Note the existing `GetField(key string) string` switch (adds a `case "phase"`) and `SetField(ctx, key, value string) error` switch (adds a `case "phase"`). Note the setter pattern: existing `Set(...)`/`Delete(...)` come from the embedded `FrontmatterMap`. Note `GetString(key)` returns `""` for a missing key.
- `pkg/domain/task_frontmatter.go` — the reference for phase getter/setter/field-case shape:
  - `Phase()` getter (lines ~88-96): reads `GetString("phase")`, returns `nil` on empty, else `&GoalPhase(raw)`.
  - `SetPhase(p *TaskPhase)` (lines ~287-294): `nil` → `Delete("phase")`, else `Set("phase", string(*p))`.
  - `GetField` `case "phase"` (lines ~346-351): returns `""` when the pointer is nil, else `string(*ph)` — RAW value, no validation on read.
  - `setPhaseField` / `SetField` `case "phase"` (lines ~412-449): the task version normalizes aliases via `NormalizeTaskPhase`. The GOAL version must NOT normalize — it validates against `AvailableGoalPhases` directly and rejects anything non-canonical (goal enum has no aliases).
- `pkg/domain/goal_frontmatter_test.go` — the suite you extend. Mirror the existing `Describe("Status")` / `Describe("SetStatus")` / `GetField` / `SetField` block structure for the new phase behavior. External `package domain_test`.
- `pkg/ops/frontmatter_entity.go` — DO NOT MODIFY. Context only: `EntitySetOperation` for goals (`goalSetOperation.Execute`, ~line 100) calls `goal.SetField(ctx, key, value)` directly with no allowlist gate, so the new `SetField` `case "phase"` is automatically reachable via `goal set`. `EntityShowOperation.Execute` (~line 656) iterates over the goal's actual `Keys()` and calls `GetField(k)` per key, so a present `phase:` key is automatically surfaced in `--output json` (`Fields` map) and plain output once `GetField` handles it. `knownGoalScalarFields` (~line 326) only guards the tags list-mutation path; it does NOT need `phase` for set/show/JSON and MUST NOT be edited (spec Constraints: Domain-only footprint).
- `CHANGELOG.md` — top section is `## v0.99.2`. There is no `## Unreleased` yet; add one directly under the intro block (above `## v0.99.2`).

Depends on prompt 1 (`GoalPhase` type). That prompt lands first.
</context>

<requirements>
1. In `pkg/domain/goal_frontmatter.go`, add a typed getter `Phase()` mirroring `TaskFrontmatter.Phase()`. Place it near the other getters (e.g. after `Assignee()`):
   ```go
   // Phase reads "phase" key as string, returns *GoalPhase.
   // Returns nil when the key is absent. The raw value is returned as-is
   // (no validation, no default substitution) so legacy/hand-typed values survive display.
   func (f GoalFrontmatter) Phase() *GoalPhase {
       raw := f.GetString("phase")
       if raw == "" {
           return nil
       }
       p := GoalPhase(raw)
       return &p
   }
   ```

2. In `pkg/domain/goal_frontmatter.go`, add a setter `SetPhase(p *GoalPhase)` mirroring `TaskFrontmatter.SetPhase`. Place it near the other setters (e.g. after `SetAssignee`):
   ```go
   // SetPhase stores the phase pointer in the map. Deletes the key if p is nil.
   func (f *GoalFrontmatter) SetPhase(p *GoalPhase) {
       if p == nil {
           f.Delete("phase")
           return
       }
       f.Set("phase", string(*p))
   }
   ```

3. Add a private field-parse helper `setPhaseField(ctx, value string) error` to `pkg/domain/goal_frontmatter.go` that validates against the goal enum directly (NO alias normalization):
   ```go
   // setPhaseField validates the value against the goal phase enum and stores it,
   // or clears the key on empty. Goal phases have no aliases — a non-canonical value is rejected.
   func (f *GoalFrontmatter) setPhaseField(ctx context.Context, value string) error {
       if value == "" {
           f.SetPhase(nil)
           return nil
       }
       phase := GoalPhase(value)
       if err := phase.Validate(ctx); err != nil {
           return errors.Wrapf(ctx, validation.Error, "unknown goal phase '%s'", value)
       }
       f.SetPhase(&phase)
       return nil
   }
   ```
   This requires importing `"github.com/bborbe/validation"` in `goal_frontmatter.go` (the file already imports `"github.com/bborbe/errors"` and `"context"`; add the validation import to the existing import block).

4. In the existing `GoalFrontmatter.GetField(key string) string` switch, add a `case "phase"` returning the raw value (no validation on read — mirrors task):
   ```go
   case "phase":
       ph := f.Phase()
       if ph == nil {
           return ""
       }
       return string(*ph)
   ```
   Place it among the existing cases (order does not affect behavior; group logically near `assignee`).

5. In the existing `GoalFrontmatter.SetField(ctx context.Context, key, value string) error` switch, add a `case "phase"`:
   ```go
   case "phase":
       return f.setPhaseField(ctx, value)
   ```

6. Do NOT modify `pkg/ops/frontmatter_entity.go`, `pkg/ops/*goal*.go`, `pkg/cli/`, or any storage file. The generic `goal set` / `goal show` wiring already routes through `SetField` / `GetField` / `Keys()`. Do NOT add `phase` to `knownGoalScalarFields` — that map only guards the tags list-mutation path and editing it is outside the Domain-only footprint (spec Constraints).

7. Extend `pkg/domain/goal_frontmatter_test.go` (package `domain_test`) with these cases. Mirror the existing block style:
   - `Describe("Phase")`:
     - missing key → `Expect(domain.NewGoalFrontmatter(nil).Phase()).To(BeNil())`.
     - present canonical value → `fm := domain.NewGoalFrontmatter(map[string]any{"phase": "execution"})`; `Expect(fm.Phase()).NotTo(BeNil())`; `Expect(*fm.Phase()).To(Equal(domain.GoalPhaseExecution))`.
     - present legacy/hand-typed value → `fm := domain.NewGoalFrontmatter(map[string]any{"phase": "in_progress"})`; `Expect(*fm.Phase()).To(Equal(domain.GoalPhase("in_progress")))` (read tolerates the raw value; no validation on read).
   - `Describe("SetPhase")`:
     - non-nil pointer stores the string: `SetPhase(domain.GoalPhaseDone.Ptr())` then `Get("phase")` equals `"done"` (assert via `fm.GetField("phase")` == `"done"`).
     - nil pointer deletes the key: seed `{"phase":"todo"}`, call `SetPhase(nil)`, then `fm.GetField("phase")` == `""` and `phase` is absent from `fm.Keys()`.
   - `Describe("SetField phase")` — the write-path validation (this is the load-bearing integration test crossing the validator boundary via the real `SetField` entry point that `goal set` calls):
     - `DescribeTable` over the four canonical values: `fm.SetField(ctx, "phase", <value>)` returns nil AND `fm.GetField("phase")` round-trips the same value.
     - invalid value: `err := fm.SetField(ctx, "phase", "bogus")`; `Expect(err).NotTo(BeNil())`; `Expect(err.Error()).To(ContainSubstring("unknown goal phase"))`; AND `Expect(err.Error()).To(ContainSubstring("bogus"))` (error names the offending phase); AND the key was NOT written (`fm.GetField("phase")` == `""` when the fm started empty).
     - legacy alias on explicit re-set is rejected: `fm.SetField(ctx, "phase", "in_progress")` returns a non-nil error containing `"unknown goal phase"` (goal enum has no aliases).
     - empty value clears: seed `{"phase":"execution"}`, `fm.SetField(ctx, "phase", "")` returns nil, `fm.GetField("phase")` == `""`.
   - `Describe("GetField phase")`:
     - present → `fm := domain.NewGoalFrontmatter(map[string]any{"phase":"planning"})`; `Expect(fm.GetField("phase")).To(Equal("planning"))`.
     - absent → `Expect(domain.NewGoalFrontmatter(nil).GetField("phase")).To(Equal(""))`.
   - `Describe("legacy goal round-trip")` — a goal with unrelated fields and NO phase: seed `{"status":"active","theme":"x"}`, assert `GetField("phase")` == `""`, then `fm.SetField(ctx, "theme", "y")` (unrelated mutation) returns nil, and `"phase"` is still absent from `fm.Keys()` (no phase injected).

8. Add a CHANGELOG entry. In `CHANGELOG.md`, add a `## Unreleased` section directly above `## v0.99.2` (below the intro/semver block) with a single bullet:
   ```
   ## Unreleased

   - feat(goal): goals carry a validated `phase` frontmatter field (`todo` / `planning` / `execution` / `done`); set via `goal set <name> phase <value>`, surfaced by `goal show` (plain + `--output json`). Legacy goals without a phase are untouched; non-canonical values are rejected on write. Mirrors the task-phase shape without touching the task-phase type.
   ```
   If a `## Unreleased` section already exists, append the bullet to it instead of creating a second one.

9. Behavioral verification against a scratch goal (run after `make precommit` passes, using the freshly built binary — never the installed `vault-cli`). Build with `make build` (or `go build -o /tmp/vault-cli-024 .` if `make build` produces a differently named artifact — check the Makefile). Create a scratch vault with a `Goals/` dir containing a minimal goal markdown file that has frontmatter (`status: next`) and NO `phase:` line, then:
   - `goal set <goal> phase execution` → exit 0; re-read the file and confirm it now contains a `phase: execution` line.
   - `goal show <goal> --output json` → output contains `"phase":"execution"` under the fields map.
   - `goal set <goal> phase bogus` → non-zero exit; stderr contains a message naming `bogus`; re-read the file and confirm the `phase:` line is UNCHANGED (still `execution`, not clobbered).
   - On a second goal file with NO `phase:` line: `goal show <legacy-goal> --output json` → exit 0 and output contains no `phase` value; then `goal set <legacy-goal> theme foo` (unrelated) → exit 0 and the file still has no `phase:` line.
   Capture the exit codes and the relevant output. If the CLI subcommand names or vault-path flags differ from `goal set` / `goal show`, discover them via `vault-cli goal --help` on the built binary and use the real flags — do NOT guess.
</requirements>

<constraints>
- Domain-only footprint. Do NOT add a new `goal update`, `goal status`, `plan-goal`, `execute-goal`, or any phase-transition/gating command (spec Non-goals — hard veto). Phase rides the existing `goal set` / `goal show` exactly as task phase rides `task set` / `task show`.
- Do NOT modify, extend, or reuse the task-side `TaskPhase` type, its constants, or `NormalizeTaskPhase` (spec Non-goals — hard veto).
- Do NOT add alias handling (no `in_progress` synonym) — the goal phase enum has no legacy values (spec Non-goals — hard veto).
- Do NOT invent a "default to todo" read behavior — a missing phase reads as empty/nil, never `todo` (spec Non-goals — hard veto).
- Do NOT backfill, rewrite, or migrate existing goal files that lack a phase (spec Non-goals — hard veto).
- Do NOT edit `pkg/ops/frontmatter_entity.go`, `knownGoalScalarFields`, storage, or CLI — the generic set/show wiring is reused unchanged (spec Constraints).
- Frontmatter remains map-based (`FrontmatterMap`); unknown keys must continue to survive read-write cycles — no separate migration code (spec Constraints).
- Do NOT add or extend goal-specific status/phase mismatch lint rules (spec Non-goals — hard veto). Legacy goals (no phase) must remain lint-clean.
- Error wrapping: `github.com/bborbe/errors` with `ctx` and the `github.com/bborbe/validation` sentinel; never `fmt.Errorf`; never `context.Background()` in non-test code.
- Tests: Ginkgo v2 / Gomega, external `_test` package. Coverage ≥80% for changed code in `pkg/domain`; test every added path including the invalid-value and empty-value branches.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass; every task command behaves identically to before.
</constraints>

<verification>
Run `make test` iteratively while developing.
Run `go test ./pkg/domain/...` — the extended goal-frontmatter suite passes (exit 0).
Run `go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/domain/... && go tool cover -func=/tmp/cover.out` — changed goal-frontmatter phase paths are covered (≥80%).
Run `grep -n "func (f GoalFrontmatter) Phase\|func (f \*GoalFrontmatter) SetPhase\|setPhaseField\|case \"phase\"" pkg/domain/goal_frontmatter.go` — returns the getter, setter, helper, and both switch cases.
Run `git diff --stat pkg/domain/task_phase.go pkg/domain/task_frontmatter.go pkg/ops/frontmatter_entity.go` — shows no changes (task type + ops frozen).
Run `grep -n 'goal phase\|phase.*frontmatter field' CHANGELOG.md` — returns ≥1 line (AC evidence).
Perform the scratch-goal behavioral checks from requirement 9 against the freshly built binary and confirm: valid phase writes `phase: execution`; JSON surfaces `"phase":"execution"`; invalid `bogus` exits non-zero and leaves the file unchanged; a no-phase goal shows/sets cleanly with no phase value.
Run `make precommit` — must pass in the repo root (lint + format + generate + test + version checks). Non-zero exit = report `status: failed`.
</verification>
