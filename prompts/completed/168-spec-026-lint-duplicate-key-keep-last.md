---
status: completed
spec: [026-bug-duplicate-frontmatter-key-silent-task-loss]
summary: Inverted duplicate-frontmatter-key repair from keep-first to keep-last in pkg/ops/lint.go, with updated tests covering git-rest prepend race, three-or-more occurrences, and nested-key/list non-flagging
execution_id: vault-cli-dup-frontmatter-lint-exec-168-spec-026-lint-duplicate-key-keep-last
dark-factory-version: v0.192.9
created: "2026-08-06T17:05:00Z"
queued: "2026-08-06T17:44:56Z"
started: "2026-08-06T17:44:58Z"
completed: "2026-08-06T17:51:15Z"
branch: dark-factory/bug-duplicate-frontmatter-key-silent-task-loss
---

<summary>
- `vault-cli task lint --fix` repairing a frontmatter key that is defined more than once now keeps the **last** occurrence instead of the first.
- For the real-world case — `task_identifier` written twice, once prepended at line 1 by the server-side git-rest service and once at its alphabetically sorted position by vault-cli — the surviving value is now the one vault-cli itself wrote, which is the identifier every local reference uses.
- A repaired file is left with exactly one occurrence of the duplicated key; every other frontmatter key, value, and its formatting are byte-identical to before the repair.
- A key repeated three or more times collapses to its last occurrence in one pass.
- Detection behavior is unchanged: `DUPLICATE_KEY` still fires on the same files, with the same wording, and still refuses to fire on indented nested keys or YAML list entries that happen to reuse a top-level key's name.
- The existing safety guard is kept: if removing the surplus lines would produce invalid YAML, the file is left untouched and the issue is reported as unfixed.
- The unit test and the two code comments that described the old keep-first rule are updated, so nothing in the repository still claims keep-first.
- New regression tests lock in duplicate `task_identifier` repair, an arbitrary duplicated key, a clean single-key file, and the nested-key/list non-flagging case.
</summary>

<objective>
Invert the duplicate-frontmatter-key repair in `pkg/ops/lint.go` from keep-first to keep-last, so `vault-cli task lint --fix` on a file whose frontmatter defines a key twice retains the value at the **last** occurrence and removes every earlier one — leaving the file parseable and pointing at the identifier vault-cli wrote, not the one git-rest prepended at line 1.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

Read these files before making changes:

- `/workspace/pkg/ops/lint.go` (888 lines) — read fully. The relevant parts:
  - Package-level regexes at lines 24-38, notably:
    ```go
    fixDuplicateKeysRegex = regexp.MustCompile(`(?s)^(---\n)(.*?)(\n---\n)(.*)$`)
    keyRegex              = regexp.MustCompile(`^([a-z_]+):\s*`)
    ```
    `keyRegex` is applied per-line with `FindStringSubmatch(line)`, so `^` anchors at the start of each line — indented nested keys (`    status: x`) and list entries (`    - status`) do NOT match. That is exactly why nested/list names are never counted as duplicates of a top-level key. Do NOT change `keyRegex`.
  - `detectDuplicateKeys` (line ~337) — raw line scan over the frontmatter text, appends a key the first time its count reaches 2. This runs BEFORE any `yaml.Unmarshal`. Leave it alone.
  - `collectLintIssues` (line ~225) — emits `LintIssue{IssueType: IssueTypeDuplicateKey, Description: fmt.Sprintf("key %q defined multiple times", key), Fixable: true}`. Leave the wording and the `Fixable: true` flag alone.
  - `fixIssues` (line ~670) — dispatches `IssueTypeDuplicateKey` to `l.fixDuplicateKeys` via the `apply` helper and writes the file once at the end. Leave alone.
  - `fixDuplicateKeys` (line ~841) — the function to change.
- `/workspace/pkg/ops/lint_test.go` (2208 lines) — read at least the `DUPLICATE_KEY` `Context` (lines ~170-216) and the surrounding suite setup (lines 18-45). The suite creates a fresh temp vault per spec in `BeforeEach` with `tasksDir = "Tasks"`, and each `Context` writes its fixture file in its own `BeforeEach`. The helper `indexOf` is defined at line ~476 and is used ONLY by the duplicate-key test you are rewriting.
- `/workspace/CHANGELOG.md` — top versioned section is `## v0.102.3`; there is currently no `## Unreleased` section.

Verified signatures (do not re-derive):

```go
// pkg/ops/lint.go
func (l *lintOperation) fixDuplicateKeys(content string) (string, bool)
func (l *lintOperation) detectDuplicateKeys(frontmatterYAML string) []string

type LintIssue struct {
    FilePath    string
    IssueType   IssueType
    Description string
    Fixable     bool
    Fixed       bool
}

type LintOperation interface {
    Execute(ctx context.Context, vaultPath string, tasksDir string, goalsDir string, fix bool) ([]LintIssue, error)
    ExecuteFile(ctx context.Context, filePath string, taskName string, vaultName string) ([]LintIssue, error)
}

func NewLintOperation() LintOperation
```

Verified behavior of `gopkg.in/yaml.v3` v3.0.1 (the version in `go.mod`) unmarshalling a duplicate top-level key into `map[string]any`:

```
yaml: unmarshal errors:
  line 4: mapping key "task_identifier" already defined at line 1
```

Read these coding-plugin docs (they are in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>
1. In `/workspace/pkg/ops/lint.go`, rewrite the line-filtering loop inside `fixDuplicateKeys` so it keeps the LAST occurrence of every duplicated key. Use a two-pass count-and-decrement over the frontmatter lines (this handles three-or-more occurrences correctly in a single pass over the output). Replace the current body between the `matches`/`body` extraction and the `if !modified` check with:

   ```go
   // Count occurrences per key so the last one can be kept.
   lines := strings.Split(frontmatterYAML, "\n")
   remaining := make(map[string]int)
   for _, line := range lines {
       if lineMatches := keyRegex.FindStringSubmatch(line); len(lineMatches) >= 2 {
           remaining[lineMatches[1]]++
       }
   }

   // Keep only the last occurrence of each key: drop a key line while more
   // occurrences of that key still follow.
   newLines := make([]string, 0, len(lines))
   modified := false
   for _, line := range lines {
       if lineMatches := keyRegex.FindStringSubmatch(line); len(lineMatches) >= 2 {
           key := lineMatches[1]
           if remaining[key] > 1 {
               remaining[key]--
               modified = true
               continue
           }
       }
       newLines = append(newLines, line)
   }
   ```

   Do NOT change the surrounding code: the `fixDuplicateKeysRegex.FindStringSubmatch(content)` extraction and its `len(matches) < 5` guard, the `frontmatterStart`/`frontmatterYAML`/`frontmatterEnd`/`body` variables, the `if !modified { return content, false }` early return, the `strings.Join(newLines, "\n")` reconstruction, and the trailing `yaml.Unmarshal` validity guard that returns `(content, false)` when the repaired frontmatter would not parse — all stay exactly as they are.

2. Remove only the surplus key LINE itself. Do NOT attempt to also remove indented continuation lines (nested mappings or list items) belonging to an earlier occurrence. If dropping the key line orphans indented children and the result no longer parses, the existing `yaml.Unmarshal` guard returns `(content, false)`, the file is left byte-identical, and the issue is reported as unfixed — that is the intended behavior for that case (spec Failure Modes row 1).

3. Update the two stale doc comments in `/workspace/pkg/ops/lint.go` so nothing describes keep-first:
   - Line ~840: `// fixDuplicateKeys removes duplicate YAML keys, keeping the first occurrence.` → `// fixDuplicateKeys removes duplicate top-level YAML keys from frontmatter,\n// keeping the last occurrence. The last occurrence is the value vault-cli\n// itself writes at the key's alphabetically sorted position; earlier\n// occurrences come from external writers that prepend at line 1.`
   - Line ~853: `// Parse frontmatter line by line, keeping only first occurrence of each key` → replace with the two comments shown in the code block in requirement 1.
   After editing, verify with `! grep -rniq "first occurrence" /workspace/pkg/` — this exits 0 when the phrase is gone. Note that a bare `grep` returning no matches exits 1; that is SUCCESS here, not a failure to chase.

4. In `/workspace/pkg/ops/lint_test.go`, rewrite the existing `It("fixes duplicate keys by keeping first occurrence", ...)` (line ~194, inside the `Context("DUPLICATE_KEY")` whose fixture has `assignee: bborbe` followed by `assignee: alice`). Rename it to `It("fixes duplicate keys by keeping the last occurrence", ...)` and invert the assertions. Replace the whole body (including the dead `lines`/byte-counting loop and the `indexOf` calls) with a direct count:

   ```go
   It("fixes duplicate keys by keeping the last occurrence", func() {
       _, err := lintOp.Execute(ctx, vaultPath, tasksDir, "", true)
       Expect(err).To(BeNil())

       taskPath := filepath.Join(vaultPath, tasksDir, "Duplicate Key.md")
       content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
       Expect(err).To(BeNil())
       contentStr := string(content)

       Expect(strings.Count(contentStr, "assignee:")).To(Equal(1))
       Expect(contentStr).To(ContainSubstring("assignee: alice"))
       Expect(contentStr).NotTo(ContainSubstring("assignee: bborbe"))
   })
   ```

   Add the `"strings"` import to the test file if it is not already present. After this rewrite, the `indexOf` helper at line ~476 has no remaining callers — delete the helper and its `// Helper function to find substring index` comment (an unused unexported function is a lint finding).

5. In `/workspace/pkg/ops/lint_test.go`, add a new `Context("DUPLICATE_KEY task_identifier (git-rest prepend race)")` that reproduces the real corruption and locks in both the keep-last value and the byte-identity of everything else. Write the fixture with the duplicated `task_identifier` first at line 1 of the frontmatter and again at its alphabetically sorted position:

   ```go
   Context("DUPLICATE_KEY task_identifier (git-rest prepend race)", func() {
       const prependedUUID = "e1bc4321-7570-41f9-bfc6-a783d7aa4371"
       const sortedUUID = "9fba815b-e1bb-442d-bc3e-87722f767a1f"

       var taskPath string
       var originalContent string

       BeforeEach(func() {
           originalContent = `---
   task_identifier: ` + prependedUUID + `
   assignee: bborbe
   page_type: task
   priority: 3
   status: completed
   task_identifier: ` + sortedUUID + `
   themes:
       - '[[Administration]]'
   ---
   # Order CR2450 Batteries for Kitchen Switch

   Body text.
   `
           taskPath = filepath.Join(vaultPath, tasksDir, "Order CR2450 Batteries for Kitchen Switch.md")
           Expect(os.WriteFile(taskPath, []byte(originalContent), 0600)).To(Succeed())
       })

       // Its:
       //  - detection reports DUPLICATE_KEY naming task_identifier
       //  - --fix keeps the sorted-position UUID
       //  - --fix removes exactly one line and nothing else
       //  - the repaired frontmatter parses as YAML
   })
   ```

   Note the fixture string is a raw backtick literal, so the frontmatter lines must start at column 0 inside it — do not indent them to match the surrounding Go code. Add these `It` blocks to that context:
   - **detects it**: call `lintOp.Execute(ctx, vaultPath, tasksDir, "", false)`, filter the returned `[]ops.LintIssue` to those with `IssueType == ops.IssueTypeDuplicateKey`, and assert exactly one such issue whose `Description` equals `key "task_identifier" defined multiple times` and whose `Fixable` is `true`. Filter by issue type rather than asserting `HaveLen` on all issues, so unrelated findings on the fixture never make this test brittle.
   - **keeps the sorted-position value**: call `Execute` with `fix = true`, re-read the file, and assert `strings.Count(contentStr, "task_identifier:") == 1`, `contentStr` contains `sortedUUID`, and `contentStr` does NOT contain `prependedUUID`.
   - **touches only the surplus line**: with the same fixed content, assert it equals the original with exactly the line-1 occurrence removed:
     ```go
     expected := strings.Replace(originalContent, "task_identifier: "+prependedUUID+"\n", "", 1)
     Expect(contentStr).To(Equal(expected))
     ```
     This is the container-executable form of the spec's "0 insertions, 1 deletion" acceptance criterion.
   - **repaired frontmatter parses**: this is the boundary the production read path (`pkg/storage/base.go`'s `parseToFrontmatterMap` → `yaml.Unmarshal` into `map[string]any`) crosses, and it is the boundary that fails today. Extract the frontmatter between the leading `---\n` and the following `\n---\n` from the fixed content and unmarshal it with `gopkg.in/yaml.v3`:
     ```go
     var m map[string]any
     Expect(yaml.Unmarshal([]byte(fm), &m)).To(Succeed())
     Expect(m["task_identifier"]).To(Equal(sortedUUID))
     ```
     Add the import `"gopkg.in/yaml.v3"` to the test file. Do NOT reach into `pkg/storage` from `pkg/ops` tests.

6. In `/workspace/pkg/ops/lint_test.go`, add a `Context("DUPLICATE_KEY with a non-identifier key")` whose fixture repeats an ordinary key (use `assignee`, appearing as `assignee: bborbe` then `assignee: alice`, with an otherwise healthy frontmatter carrying `status`, `page_type`, `priority`, and a single valid `task_identifier`). Assert that `Execute` with `fix = false` produces exactly one `IssueTypeDuplicateKey` issue whose `Description` equals `key "assignee" defined multiple times`.

7. In `/workspace/pkg/ops/lint_test.go`, add a `Context("no DUPLICATE_KEY when every top-level key is unique")` whose fixture has each top-level key exactly once. Assert that filtering `Execute`'s result (with `fix = false`) to `IssueTypeDuplicateKey` yields zero issues.

8. In `/workspace/pkg/ops/lint_test.go`, add a `Context("no DUPLICATE_KEY for nested mapping keys or list entries")` — the regression lock for the already-correct top-level-only detection. Fixture:

   ```
   ---
   status: completed
   page_type: task
   priority: 1
   assignee: bborbe
   metadata:
       status: nested
       assignee: nested_person
   tags:
       - status
       - assignee
       - page_type
   task_identifier: 22222222-2222-4222-a222-222222222222
   ---
   # Nested Keys Task

   Body text.
   ```

   Assert that filtering `Execute`'s result (with `fix = false`) to `IssueTypeDuplicateKey` yields zero issues. Then run `Execute` again with `fix = true` and assert the file content is byte-identical to the fixture (no repair was attempted on it).

9. Add a `## Unreleased` bullet to `/workspace/CHANGELOG.md` immediately after implementing, before running `make precommit`. If a `## Unreleased` section already exists (the sibling prompt for this spec may have created it), APPEND this bullet to it — do not replace the section or its existing bullets, and do not create a second `## Unreleased` heading. If it does not exist, create it **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` bullets) and **above** the `## v0.102.3` heading — never between the `# Changelog` title and the preamble. Add this bullet:

   ```
   ## Unreleased

   - fix: `task lint --fix` now keeps the last occurrence when a top-level frontmatter key is defined more than once, so a duplicated `task_identifier` resolves to the value vault-cli wrote at its sorted position instead of the one prepended at line 1 by an external writer
   ```

   Do NOT bump or hand-edit any version string in `CHANGELOG.md` or in `.claude-plugin/plugin.json` / `.claude-plugin/marketplace.json` — the release agent owns those.

10. Coverage: the changed package is `pkg/ops`. `fixDuplicateKeys` must be exercised by the new and updated tests along both its outcomes — the successful repair path (requirements 4, 5) and the "no duplicates, nothing to do" path (requirements 7, 8, where `modified` stays false and the function returns `(content, false)`). Do not add retroactive coverage to unrelated untested lint code.

11. Add an `It` covering three or more occurrences of the same key. This is the case where count-and-decrement differs from a naive "drop all but the last seen" implementation, so it must be locked in. Use a fixture whose frontmatter defines the same top-level key three times with distinct values, run `Execute` with `fix = true`, then assert the repaired content contains exactly one occurrence of that key (`strings.Count`) and that the surviving value is the **third** one.

12. Add an `It` covering the invalid-YAML guard. This guard is what makes the repair safe on files that are broken in more than one way, and it is currently untested. Use a fixture with a duplicate top-level key PLUS an unrelated YAML syntax error, run `Execute` with `fix = true`, then assert the file content is byte-identical to the fixture (no repair written) and that the `DUPLICATE_KEY` issue's `Fixed` field is `false`.
</requirements>

<constraints>
- Do NOT introduce a new issue type. `DUPLICATE_KEY` already exists and already fires on the real corrupted file; a second type would be a duplicate rule under a different name.
- The `DUPLICATE_KEY` issue type name, its `Fixable: true` flag, and its plain-output rendering (`WARN` / `FIXED` prefix, `key %q defined multiple times` wording) stay exactly as they are — the lint output format is consumed by scripts and agents.
- Detection MUST keep running on the raw frontmatter text before any `yaml.Unmarshal`. Any rewrite that routes detection through unmarshal is a regression, because unmarshal is precisely the operation that fails on this input. Do NOT touch `detectDuplicateKeys`.
- Detection MUST keep considering only top-level frontmatter keys. Do NOT change `keyRegex`; indented nested mapping keys and YAML list items are never duplicates of a top-level key.
- Preserve the existing "only write if the repaired YAML parses" guard at the end of `fixDuplicateKeys`. If the repair would produce invalid YAML, the file stays byte-identical and the issue is reported as unfixed.
- Unknown frontmatter fields must survive the repair. Only the surplus duplicate key lines are removed; every other key, value, and its formatting is untouched.
- Do NOT add a flag, config key, or environment variable to select keep-first vs keep-last. The rule is invariant.
- Do NOT modify `pkg/storage/page.go` or `pkg/storage/task.go` — the enumeration warning is a separate prompt, and `taskStorage.ListTasks` is explicitly out of scope for this spec.
- Do NOT change how any command creates task files, and do NOT remove the `WriteTask` identifier-stamping fallback.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` in `pkg/ops/`.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0, including the rewritten and new `pkg/ops` `DUPLICATE_KEY` contexts.

Confirm no keep-first language survives anywhere in the package (this exits 0 on success; a bare `grep` with no matches exits 1, which is NOT a failure):

```
! grep -rniq "first occurrence" pkg/
```

Confirm the changelog entry landed with the right prefix:

```
grep -A20 '^## Unreleased' CHANGELOG.md
```

Must show a line starting with `- fix:`.

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.

Coverage for the changed package:

```
go test -coverprofile=/tmp/cover.out ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep fixDuplicateKeys
```
</verification>
