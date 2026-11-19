---
status: completed
summary: 'Scoped ORPHAN_GOAL detection to the goals: block with line-bounded regexes, unescaped YAML single-quote doubling in goal names, added four regression contexts, and a CHANGELOG entry.'
execution_id: vault-cli-orphan-goal-exec-283-lint-orphan-goal-block-scope
dark-factory-version: v0.199.1
created: "2026-11-17T18:45:00Z"
queued: "2026-11-17T18:18:01Z"
started: "2026-11-17T20:44:20Z"
completed: "2026-11-19T04:06:32Z"
---

# Scope ORPHAN_GOAL detection to the `goals:` block

<summary>
- Fixes false ORPHAN_GOAL reports: a task that lists goals and then themes (or any other list) was reported as referencing goals that do not exist, because the goal reader ran past the end of the goals list into every later frontmatter key.
- Measured on the private-personal vault 2026-11-17: 4662 false `ORPHAN_GOAL` lines across 1450 distinct values (top: `Leverage Autonomous Agents` 1515, `Reduce Manual Work` 211, `Advance AI Autonomy` 180 — all themes).
- Fix: read only the goals list itself, stopping at the next frontmatter key.
- Also unescapes YAML single-quote doubling (`''` → `'`) in goal names, so a goal whose filename contains an apostrophe is matched rather than reported missing (132 such lines on private-personal, e.g. `attention-controller Is the Fleet''s Single Source of Session State`).
- Adds a regression test that asserts BOTH directions: 0 findings when `goals:` is followed by `themes:`/`metrics_sessions:` lists, and exactly 1 finding for a real missing goal (block-list and flow-list shapes).
- Adds an `## Unreleased` CHANGELOG bullet.
- `make precommit` passes.
</summary>

<objective>
`ORPHAN_GOAL` in `pkg/ops/lint.go` reads only the `goals:` key's values — block list or flow list — and stops at the next top-level key, so no non-`goals` list item is ever reported as a missing goal, while a genuine orphan `goals:` entry is still reported.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read before implementing:
- `pkg/ops/lint.go`:
  - Lines ~33-35: the package-level regexes `goalsInlineRegex`, `goalsMultilineRegex`, `goalItemRegex`.
  - `detectOrphanGoals` (~line 496): tries `goalsInlineRegex`, then `goalsMultilineRegex`, then parses.
  - `parseInlineGoalsList` (~line 519): the flow-list parser, exercised by the new flow-list context.
  - `parseMultilineGoalsList` (~line 542): splits the captured block by `\n` and applies `goalItemRegex` per line.
  - `extractGoalName` (~line 565), `goalFileExists` (~line 572).
  - `LintIssue` struct (~line 149): fields are `FilePath`, `IssueType`, `Description`, `Fixable`, `Fixed`. `IssueTypeOrphanGoal` is the exported constant.
  - `lintFile` (~line 230) passes `string(matches[1])` — the frontmatter block between the `---` fences — to `collectLintIssues` → `detectOrphanGoals`.
- `pkg/ops/lint_test.go`:
  - `Describe("LintOperation - Orphan Goal Detection")` (line ~1551) — extend it; do not create a new suite.
  - Existing contexts: "when goal file exists", "when goal file does not exist", "with multi-line goals format". Each uses the suite-level `BeforeEach` that creates a temp vault with `Tasks/` and `Goals/`.

Root cause: `goalsMultilineRegex` is `(?ms)^goals:\s*\n((?:\s*-\s*.+\n?)+)`. The `s` flag makes `.` match `\n`, so the first `.+` consumes the rest of the frontmatter and the `+` then has nothing left to match. The capture is "everything from the first `- ` to end of frontmatter". `parseMultilineGoalsList` then picks every `- ` line in that span, including `themes:` and `metrics_sessions:` items. The existing "with multi-line goals format" test does not catch this because the key it puts after `goals:` (`task_identifier:`) carries no `- ` list items — the fixture needs a following key that has its own list.
</context>

<requirements>
1. Replace the two goals regexes in `pkg/ops/lint.go` with line-bounded, non-dotall forms:
   ```go
   goalsInlineRegex    = regexp.MustCompile(`(?m)^goals:[ \t]*\[(.*?)\]`)
   goalsMultilineRegex = regexp.MustCompile(`(?m)^goals:[ \t]*\n((?:[ \t]*-[ \t]*[^\n]*\n?)+)`)
   ```
   Rationale (record it in a code comment above the var block):
   - No `s` flag → `.`/`[^\n]` cannot span lines, so the block stops at the next top-level key.
   - `[ \t]` instead of `\s` → `\s` matches `\n` and would still let the separator cross a line boundary.
   - `[^\n]*` instead of `.+` → a list item cannot swallow following lines.
   Keep the variable names and the `(?m)` flag. Leave `goalItemRegex`, `goalFileExists` and `detectOrphanGoals`'s signature unchanged.
   Add a short comment above `goalsMultilineRegex` recording the bug: the old dotall `.+` swallowed later keys and produced 4662 false findings on private-personal (2026-11-17).

1b. In `extractGoalName` (~line 565), after stripping the `[[` / `]]` brackets and trimming, unescape YAML single-quote doubling so a goal whose name contains an apostrophe resolves to its real filename:
   ```go
   goalName = strings.ReplaceAll(goalName, "''", "'")
   ```
   Rationale (record in the function's doc comment): a YAML single-quoted scalar escapes `'` as `''`, so `- '[[attention-controller Is the Fleet''s Single Source of Session State]]'` must resolve to the goal file `attention-controller Is the Fleet's Single Source of Session State.md`. Today it resolves to a name with a doubled apostrophe, the file lookup misses, and `ORPHAN_GOAL` fires falsely. `strings` is already imported. Do this after `TrimPrefix`/`TrimSuffix` and before the final `TrimSpace`, so the replace cannot affect bracket stripping.

2. Extend `Describe("LintOperation - Orphan Goal Detection")` in `pkg/ops/lint_test.go` with four new `Context` blocks (reuse the existing `BeforeEach`/`AfterEach` temp-vault setup). Add one small helper in the test file:
   ```go
   func orphanGoalIssues(issues []ops.LintIssue) []ops.LintIssue {
       var out []ops.LintIssue
       for _, i := range issues {
           if i.IssueType == ops.IssueTypeOrphanGoal {
               out = append(out, i)
           }
       }
       return out
   }
   ```
   - **Context "with goals followed by themes and metrics_sessions lists"** — write a task file whose frontmatter is:
     ```yaml
     status: todo
     page_type: task
     goals:
       - "[[Existing Goal]]"
     themes:
       - "[[Leverage Autonomous Agents]]"
     metrics_sessions:
       - session_id: 76a00500-0000-4000-8000-000000000000
         started_at: "2026-11-17T17:30:33Z"
     task_identifier: 12341234-1234-4234-8234-123412341234
     ```
     Create `Goals/Existing Goal.md`. Run `issues, err := lintOp.Execute(ctx, ops.PageTypeTask, vaultPath, tasksDir, goalsDir, false)`, assert `Expect(err).To(BeNil())`, then assert `Expect(orphanGoalIssues(issues)).To(BeEmpty())` — no `ORPHAN_GOAL` for the theme or the metrics_sessions item.
   - **Context "with a genuine orphan goal in a block list"** — same shape but `goals:` lists `"[[Missing Goal]]"` (and keep the trailing `themes:` list). Assert `Expect(orphanGoalIssues(issues)).To(HaveLen(1))` and `Expect(orphanGoalIssues(issues)[0].Description).To(ContainSubstring("Missing Goal"))`.
   - **Context "with a flow-list goals followed by a theme list"** — frontmatter `goals: ["[[Missing Goal]]"]` on one line, then `themes: ["[[Leverage Autonomous Agents]]"]` on the next. Assert exactly 1 `ORPHAN_GOAL` (the flow-list missing goal), and that its description names `Missing Goal`, not the theme.
   - **Context "with an escaped apostrophe in a goal name"** — write a task file whose `goals:` block has one item `- '[[attention-controller Is the Fleet''s Single Source of Session State]]'`, and create the goal file `Goals/attention-controller Is the Fleet's Single Source of Session State.md` (single apostrophe). Assert `Expect(err).To(BeNil())` and `Expect(orphanGoalIssues(issues)).To(BeEmpty())` — the doubled apostrophe must resolve to the real filename, not report a missing goal.

3. Add a `## Unreleased` bullet to `CHANGELOG.md`. Resolve the newest versioned heading at run time and insert `## Unreleased` directly above it; if an `## Unreleased` already exists, append the bullet inside it (never create a second). Bullet:
   ```
   - fix: scope ORPHAN_GOAL detection to the goals: block so later frontmatter lists are not reported as missing goals
   ```
   Do NOT bump version strings or create a tag.

4. Run `make test` iteratively until green, then `make precommit` once.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do not touch any lint rule other than `ORPHAN_GOAL` and the three goal regexes.
- Do not change `detectOrphanGoals`'s signature or the `LintIssue` type.
- Do not introduce a YAML unmarshal for the goals field — the block-extraction regex is deliberate: lint must stay resilient to malformed frontmatter and must not start failing on files it currently lints. (Frontmatter with bare `[[wikilinks]]` is not valid YAML without a quoting pass; the regex path avoids that dependency.)
- Error handling: `github.com/bborbe/errors` wrapping with `ctx`; never `fmt.Errorf`.
- Ginkgo v2 / Gomega only.
</constraints>

<verification>
- `make test` → green, including the four new contexts.
- `make precommit` → exit 0.
- `grep -n 'MustCompile' pkg/ops/lint.go | grep -cF '(?ms)'` → 0 (no compiled regex carries the dotall flag; a comment may mention it).
- `grep -n 'with goals followed by themes and metrics_sessions lists' pkg/ops/lint_test.go` → the new context is present.
- `grep -n "ReplaceAll(goalName, \"''\", \"'\")" pkg/ops/lint.go` → 1 (the apostrophe unescape is present).
</verification>
