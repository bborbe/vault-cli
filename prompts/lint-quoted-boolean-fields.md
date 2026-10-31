---
status: draft
---

# Lint flags a boolean frontmatter field written as a quoted string and `--fix` unquotes it

<summary>
- Every `lint` subcommand reports a known boolean field whose value is a quoted string instead of a bare YAML boolean.
- The report names the file and the offending field, so a reader knows exactly which line to look at.
- `--fix` rewrites that one line to the bare boolean and leaves every other line byte-identical.
- A quoted value on a field that is not in the boolean set is not reported — `title: "true"` stays silent.
- A known boolean field carrying something that is not a boolean literal (`flag: "maybe"`) is not reported and not rewritten.
- All lint commands share one detection path, so a fix in one is a fix in all of them.
- A reader that only greps plain text now sees the same truth the CLI already computed, instead of missing the field.
- Existing lint behaviour is unchanged: every other issue type, its message and its fix stay exactly as they are.
- A new test fails on today's code and passes after the change, so it pins the fix rather than describing it.
</summary>

<objective>
Add a `QUOTED_BOOLEAN` lint issue that fires when a known boolean frontmatter field is stored as a quoted string (`flag: "true"`), and a `--fix` that unquotes it in place. Any plain-text consumer is blind to the quoted form: on 2026-10-31 a fleet manager grepped `^flag: true` over a vault, found 1 flagged goal where 11 carried `flag: "true"`, and acted on one goal instead of eleven. The stored shape is the defect, not the reader: `flag`, `reviewed` and `needs_review` all parse fine because each is read through `GetBool`, which coerces `"true"` and `"yes"`, while the other eight fields in the set are consumed only by fleet tooling that greps the raw text — which is precisely why their stored shape matters.
</objective>

<context>
Read `CLAUDE.md` for project conventions. Then read these files fully, in this order:

- `pkg/ops/lint.go` — the whole file. The anchors are: the `IssueType` const block (`IssueTypeMissingFrontmatter` … `IssueTypeBlockedByScalar`), the `LintIssue` struct, `collectLintIssues` (the `add` closure and the sequence of `detect…` calls), `fixIssues` (the `apply` closure and the per-issue-type `switch`), `fixInvalidPriority` (the rewrite exemplar — note it operates on the raw string, never on a parsed map), and the package-level regex vars at the top of the file.
  **Two regex shapes live at the top of this file and they are not interchangeable.** `frontmatterRegex` is `(?s)^---\n(.*?)\n---\n` — ONE capture group, the block, anchored at the start; it does **not** capture the body. `fixDuplicateKeysRegex` is `(?s)^(---\n)(.*?)(\n---\n)(.*)$` — four groups: opening delimiter, block, closing delimiter, body. Requirement 4 needs the four-group form; use `fixDuplicateKeysRegex` there rather than hand-slicing byte offsets.
- `pkg/ops/lint_test.go` — the existing suite shape: a temp directory built with `os.MkdirTemp` plus `os.WriteFile`, an `ops.NewLintOperation()` value, and Ginkgo v2 / Gomega assertions on the returned `[]ops.LintIssue`. Follow that setup; do not invent a fixture framework.
- `pkg/cli/cli.go` — `createGenericLintCommand` is the single factory behind `task`, `goal`, `topic`, `theme`, `objective` and `vision` lint. Separately, `task validate` calls `lintOp.ExecuteFile`, which reaches the same `collectLintIssues`. This is why no CLI change is required, and why the change is visible on seven surfaces rather than one.
- `pkg/domain/task_frontmatter.go` — `Flag()`; and `pkg/domain/frontmatter_map.go` — `GetBool`, the coercion it delegates to. The reader is correct and must not change.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Wrapf` / `errors.Errorf` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and the required `fix:` prefix.
</context>

<requirements>
1. **Add the issue type and the boolean field set to `pkg/ops/lint.go`.** In the `IssueType` const block, append `IssueTypeQuotedBoolean IssueType = "QUOTED_BOOLEAN"`. **gofmt column-aligns that block** — the longest existing name is `IssueTypeStatusCheckboxMismatch`, so the emitted line carries several spaces between the name and `IssueType`. Let gofmt do it; do not hand-pad.

   Beside the package-level regex vars, add a package-level `map[string]struct{}` named `booleanFields` holding exactly these eleven keys:

   ```
   flag, manager_pin, goal_loop, auto_abort_prior, spawn_notification,
   shipping, reviewed, fixture, needs_review, fresh, awaiting_input
   ```

   A map, not a slice: membership is the only operation, and the set is looked up once per frontmatter line. The set is the boolean-valued frontmatter keys measured across the `private-personal` vaults on 2026-10-31. `flag`, `reviewed` and `needs_review` are read by vault-cli (all through `GetBool` — `pkg/domain/decision.go` projects the last two); the other eight are read only by fleet tooling. Do not narrow the set and do not extend it beyond these eleven.

2. **Add one shared classifier and one detector.** Both the report and the fix must classify a line identically, so the classification lives in exactly one place:

   ```go
   // quotedBooleanKey reports whether line is a top-level frontmatter line whose
   // key is a known boolean field and whose value is a quoted boolean literal.
   // It returns the key and the unquoted literal so the detector and the fixer
   // cannot drift apart.
   func quotedBooleanKey(line string) (key, literal string, ok bool)
   ```

   It returns `ok == false` unless **all** hold:
   - the line does not begin with a space or a tab (top-level key only — an indented `flag: "true"` inside a nested mapping is another key's child and must not fire);
   - splitting at the **first** `:` gives a key, trimmed, that is present in `booleanFields`;
   - the remainder, trimmed, is exactly one of `"true"`, `"false"`, `'true'`, `'false'` — two quote characters × two literals. Compare against those four strings; do **not** attempt a regex with a backreference, because RE2 has none. The returned `literal` is that remainder with the quotes stripped (`strings.Trim(remainder, "\"'")`), so the four-literal check and the returned value cannot disagree.

   Then add `detectQuotedBooleans(frontmatterYAML string) []string` — a method on `*lintOperation` returning the offending field names in the order they appear, built by walking `strings.Split(frontmatterYAML, "\n")` and calling `quotedBooleanKey` on each line.

3. **Call the detector from `collectLintIssues`.** After the `detectBlockedByScalar` block and before the `taskIdentifierIssues` line, add one loop:

   ```go
   for _, field := range l.detectQuotedBooleans(frontmatterYAML) {
       add(IssueTypeQuotedBoolean, fmt.Sprintf("field %q is a quoted string, expected a bare bool", field), true)
   }
   ```

   The description must name the field — a criterion asserts the report names both the file (`LintIssue.FilePath`, already set by `add`) and the field. Do not change any other `detect…` call, message or fixable flag.

4. **Add `func (l *lintOperation) fixQuotedBooleans(content string) (string, bool)`.** Behaviour:
   - Match `fixDuplicateKeysRegex` against `content`. Fewer than five submatch entries → return `content, false`; a file with no frontmatter block is not this rule's business. Group 1 is the opening `---\n`, group 2 the block, group 3 the closing `\n---\n`, group 4 the body.
   - Walk only group 2's lines, calling the same `quotedBooleanKey` from requirement 2. A matching line becomes `key + ": " + literal`. A line that does not match is kept **byte-identical**, including its own spacing after the colon — so `flag:   "true"` becomes `flag: true` (that one line changes) while every other line is untouched.
   - Reassemble as `group1 + newBlock + group3 + group4`. Take the delimiters from the match; do not rebuild them from `"---\n"` literals.
   - Return `content, false` when no line matched, so the caller can distinguish "fixed nothing" from "fixed".
   - Never `yaml.Unmarshal` / `yaml.Marshal` any part of the block. A re-serialise rewrites key order, indentation and the quoting of unrelated keys, which is precisely the defect this prompt removes — and it would make the `--fix` diff larger than the issue. **This is where you diverge from the `fixDuplicateKeys` exemplar:** it ends with a `yaml.Unmarshal` re-parse to validate its own output. Do not copy that step here; the byte-preservation contract is stronger than a re-parse, and a parse would accept a rewritten block that this rule must never produce.

5. **Wire the fix into `fixIssues`.** `QUOTED_BOOLEAN` is a whole-file fix: one rewrite clears every quoted boolean in the file, so it must not run once per issue. Before the existing `for i := range issues` loop, scan for a `Fixable` `IssueTypeQuotedBoolean` issue; when one exists, call `l.fixQuotedBooleans(updatedContent)` once, and on a `true` result assign the new content, set `modified = true`, and mark **every** `IssueTypeQuotedBoolean` issue `Fixed = true`. Do not add a `case` for it to the per-issue `switch`. The existing `apply` closure stays as it is for every other issue type.

6. **Add specs to `pkg/ops/lint_test.go`.** Build the fixtures with `os.WriteFile` in a temp directory, exactly as the existing tests do, and drive them through `ops.NewLintOperation()`. **Name every new `Describe` / `Context` / `It` so its text contains `QUOTED_BOOLEAN`** — the verification block focuses on that string, and a spec named "quoted boolean" matches nothing. Cover:
   - **Detection across all eleven fields:** a task fixture whose frontmatter carries all eleven keys in double-quoted form yields exactly eleven `IssueTypeQuotedBoolean` issues, and each issue's `Description` contains its own field name. This is the case that fails on today's code.
   - **Detection through the ops entry point for a second page type:** run the same frontmatter through `Execute` against a goal directory, so the shared-helper claim is exercised rather than asserted.
   - **The `task validate` surface:** drive one quoted-boolean file through `ExecuteFile` — the path `task validate` uses — and assert the `QUOTED_BOOLEAN` issue comes back. The constraint asserts this surface, so it is exercised rather than assumed.
   - **The fix, proven byte-preserving:** run `--fix` on the eleven-field fixture and assert the **whole file string** equals an expected string in which only the eleven value lines changed — every other line, including the blank line after the block and the body, byte-identical. Asserting a substring is not enough; the defect is collateral churn.
   - **A single-field fixture:** one `flag: "true"` file, `--fix`, assert exactly one line differs from the input.
   - **The negative cases:** `title: "true"` (unlisted field) and `flag: "maybe"` (listed field, not a boolean literal) each produce zero `IssueTypeQuotedBoolean` issues, and `--fix` leaves both files byte-identical.
   - **Single-quoted and false literals:** `flag: 'true'` and `flag: 'false'` are each detected and fixed to `flag: true` / `flag: false`, and a double-quoted `flag: "false"` is detected and fixed the same way — all four accepted literals are exercised.
   - **An unquoted value is silent:** `flag: true` yields no issue.
   - **The indented negative:** a nested mapping carrying an indented `flag: "true"` produces zero `IssueTypeQuotedBoolean` issues, and `--fix` leaves that file byte-identical — the top-level guard is exercised, not assumed.

   These cases must cover every branch the new code adds — this package runs under `-cover` and new code is held to ≥80%.

7. **Record the change in `CHANGELOG.md`.** There is no `## Unreleased` section today. Insert one directly below the preamble (the `* PATCH version…` bullet) and above the newest `## vX.Y.Z` section, with one bullet:

   ```markdown
   - fix: every `lint` subcommand now reports a boolean frontmatter field stored as a quoted string (`flag: "true"`) as `QUOTED_BOOLEAN`, and `--fix` unquotes it in place without re-serialising the block. vault-cli already read both forms as true; a plain-text consumer grepping `^flag: true` did not.
   ```

   Do **not** bump any version string — this repo's release automation owns the four version fields. Add the unreleased bullet only.

8. **Self-check before finishing.** Re-read the diff against the summary bullets above, confirm each of the seven test cases is present and named, then run the full `<verification>` block.
</requirements>

<constraints>
- Do not change `pkg/domain/task_frontmatter.go`, `pkg/domain/frontmatter_map.go`, `GetBool`, or any reader. The reader is correct; the stored shape is not.
- Do not add a CLI flag or verb. `createGenericLintCommand` already serves `task`, `goal`, `topic`, `theme`, `objective` and `vision` lint; `pkg/cli/cli.go` needs no change.
- `vault-cli task validate` will start reporting `QUOTED_BOOLEAN` in its plain and JSON output, because it reaches `collectLintIssues` through `ExecuteFile`. That is expected — do not gate the rule on page type to keep it off that surface.
- Do not re-serialise the frontmatter block with `yaml.Marshal` anywhere in the new code.
- Do not report a quoted value on a field outside `booleanFields`, and do not report `"yes"` / `"no"` / `"maybe"` on a field inside it — only the four quoted boolean literals fire.
- Do not touch any existing issue type, its description, its `Fixable` flag or its fix.
- Do not mark an issue `Fixed` unless the rewrite actually changed the file.
- Existing tests must still pass.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make test` iteratively while implementing, then `make precommit` once at the end. `make precommit` must exit 0.

Targeted run:

```
go test -mod=mod ./pkg/ops/... -ginkgo.focus='QUOTED_BOOLEAN' -ginkgo.v
```

`-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focused run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

Evidence greps (run each and confirm the count). The const-block grep allows for gofmt's column alignment:

```
grep -cE 'IssueTypeQuotedBoolean +IssueType = "QUOTED_BOOLEAN"' pkg/ops/lint.go   # 1
grep -c 'func quotedBooleanKey' pkg/ops/lint.go                                   # 1
grep -c 'func (l \*lintOperation) detectQuotedBooleans' pkg/ops/lint.go           # 1
grep -c 'func (l \*lintOperation) fixQuotedBooleans' pkg/ops/lint.go              # 1
grep -c 'QUOTED_BOOLEAN' CHANGELOG.md                                             # >= 1
```

Then confirm the changelog bullet sits under the right heading — a bare count does not prove placement:

```
awk '/^## /{sec=$0} /QUOTED_BOOLEAN/{print "sits under: " sec}' CHANGELOG.md
```

The output must read `sits under: ## Unreleased`.

Syntax and formatting:

```
gofmt -e -l pkg/ops/lint.go pkg/ops/lint_test.go
```

The command must print nothing.
</verification>
