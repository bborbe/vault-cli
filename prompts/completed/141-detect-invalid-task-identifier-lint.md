---
status: completed
summary: Added INVALID_TASK_IDENTIFIER lint rule that fires when task_identifier is present but not a valid UUID, with 5 Ginkgo test cases, bumped version to 0.79.0, and updated all test fixtures from non-UUID placeholder values to real UUIDs.
container: vault-cli-exec-141-detect-invalid-task-identifier-lint
dark-factory-version: v0.182.0
created: "2026-06-20T11:30:00Z"
queued: "2026-06-20T10:23:00Z"
started: "2026-06-20T10:23:39Z"
completed: "2026-06-20T10:31:20Z"
---

<summary>
- Adds a new lint rule that catches `task_identifier` values that are present but not valid UUIDs (e.g. literal `<uuid>` placeholder from the task template, typos like `abc-123`, truncated strings).
- Complements the existing `MISSING_TASK_IDENTIFIER` rule which only fires on empty/absent values — together they close the gap that lets the template placeholder ship as a real value.
- The rule is non-fixable on purpose: auto-fixing would silently mint a fresh UUID, which is the same hidden creation site that causes the concurrent-write merge-conflict race we are trying to prevent. Operator must replace with a real UUIDv4.
- Tests cover blank value (only MISSING fires), valid UUID (no issues), literal `<uuid>` (INVALID fires), malformed `abc-123` (INVALID fires), and missing key (only MISSING fires) — explicitly asserting no double-fire when one rule applies.
- Adds CHANGELOG entry under a new `## v0.79.0` section (minor bump per repo version-alignment rules); bumps `.claude-plugin/plugin.json` and both entries in `.claude-plugin/marketplace.json` from `0.78.1` to `0.79.0`.
- Out of scope: removing the `WriteTask` UUID fallback (kept as defensive net), wiring `EnsureAllTaskIdentifiers` to a CLI command, and any auto-fix behavior.
</summary>

<objective>
Add an `INVALID_TASK_IDENTIFIER` lint rule to `pkg/ops/lint.go` that flags non-empty `task_identifier` values which do not parse as a UUID. The detector is a sibling of the existing `missingTaskIdentifierIssues` function and is wired into `collectLintIssues` next to it. The rule is non-fixable; operator must replace the value with a real UUIDv4.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.

Read these files in full before making changes:
- `/workspace/pkg/ops/lint.go` — the file under modification. Pattern exemplars to mirror (do NOT inline their bodies):
  - `IssueType` const block (around lines 47-57 in current HEAD; anchor by name, not line number) — sibling constant placement
  - `missingTaskIdentifierIssues` function (around line 584-596) — sibling-function shape, return type `[]LintIssue`
  - `detectMissingTaskIdentifier` (around lines 598-607) — YAML-parse pattern using `gopkg.in/yaml.v3` and an anonymous struct with `yaml:"task_identifier"` tag
  - `collectLintIssues` call site for `missingTaskIdentifierIssues` (around line 260) — the exact wiring style to mirror
- `/workspace/pkg/ops/lint_test.go` — `Context("MISSING_TASK_IDENTIFIER", ...)` block (around lines 1731-1772) — Ginkgo style template for the new tests: fixture content built as a single string, `os.WriteFile` to `tasksDirPath`, `lintOp.Execute(ctx, vaultPath, tasksDir, false)`, then iterate over returned issues asserting on `IssueType`. Mirror this shape exactly for the new tests.
- `/workspace/pkg/storage/task.go` — has the existing `github.com/google/uuid` import for reference (this is the same package the new code will import).
- `/workspace/CHANGELOG.md` — note the `## v0.78.1` heading at top; new entry goes under a new `## v0.79.0` heading inserted ABOVE it (minor bump per repo version-alignment rules).
- `/workspace/.claude-plugin/plugin.json` — `"version": "0.78.1"` at line 4.
- `/workspace/.claude-plugin/marketplace.json` — two `"version": "0.78.1"` entries (one at line 8, one at line 15). BOTH must be bumped.

Important verification notes from upstream pattern-discovery (these will fail the auditor if ignored):
- `github.com/google/uuid` is NOT yet imported in `pkg/ops/lint.go` — the existing imports are `context`, `fmt`, `os`, `path/filepath`, `regexp`, `strings`, `github.com/bborbe/errors`, `gopkg.in/yaml.v3`, and `github.com/bborbe/vault-cli/pkg/domain`. You MUST add `"github.com/google/uuid"` to the import block in the same edit (correctly grouped with the other third-party imports).
- `IssueTypeStatusDateMismatch IssueType = "STATUS_DATE_MISMATCH"` is already present at line 56 — the new constant goes immediately after it (preserving the existing block-end formatting).
- The wiring style in `collectLintIssues` uses the `add(issueType, desc, fixable)` closure for single-issue checks, BUT `missingTaskIdentifierIssues` is the exception — it returns `[]LintIssue` and is appended via `issues = append(issues, l.missingTaskIdentifierIssues(...)...)` (line 260). The new `invalidTaskIdentifierIssues` MUST follow the SAME returning-slice convention so wiring is `issues = append(issues, l.invalidTaskIdentifierIssues(filePath, frontmatterYAML)...)` on the line immediately after line 260.
</context>

<requirements>

### 1. Add new `IssueType` constant in `pkg/ops/lint.go`

In the `const ( ... )` block, immediately AFTER the existing `IssueTypeStatusDateMismatch IssueType = "STATUS_DATE_MISMATCH"` line, add:

```go
IssueTypeInvalidTaskIdentifier  IssueType = "INVALID_TASK_IDENTIFIER"
```

Additive only — do NOT change or reorder any existing constant or its wire-format string. Match the existing alignment/spacing style of the surrounding lines (the block uses spaces to align the `IssueType` keyword across declarations).

### 2. Add `github.com/google/uuid` import to `pkg/ops/lint.go`

In the existing import block, add `"github.com/google/uuid"` grouped with the other third-party imports (`github.com/bborbe/errors`, `gopkg.in/yaml.v3`). Alphabetical order within the group keeps it after `github.com/bborbe/errors` and before `gopkg.in/yaml.v3`. The result of running `goimports` on the file MUST be a no-op after your edit.

### 3. Add `invalidTaskIdentifierIssues` method on `lintOperation` in `pkg/ops/lint.go`

Placement: directly AFTER the existing `detectMissingTaskIdentifier` function (which ends around line 607), BEFORE the `fixIssues` function (which starts around line 609-610). The new function is a sibling of `missingTaskIdentifierIssues`: same return type, same call shape, same overall structure.

Exact signature and body:

```go
// invalidTaskIdentifierIssues returns a lint issue if task_identifier is set
// to a value that does not parse as a UUID. Empty values are out of scope —
// they are covered by IssueTypeMissingTaskIdentifier (see missingTaskIdentifierIssues).
// Non-fixable on purpose: auto-fix would silently mint a fresh UUID, recreating
// the hidden creation site that causes concurrent-write merge conflicts on
// legacy tasks. Operator must replace the value with a real UUIDv4.
func (l *lintOperation) invalidTaskIdentifierIssues(filePath, frontmatterYAML string) []LintIssue {
	var fm struct {
		TaskIdentifier string `yaml:"task_identifier"`
	}
	if err := yaml.Unmarshal([]byte(frontmatterYAML), &fm); err != nil {
		return nil // Cannot parse; other checks will surface the error
	}
	if fm.TaskIdentifier == "" {
		return nil // Empty value is covered by MISSING_TASK_IDENTIFIER
	}
	if _, err := uuid.Parse(fm.TaskIdentifier); err == nil {
		return nil // Valid UUID — no issue
	}
	return []LintIssue{{
		FilePath:    filePath,
		IssueType:   IssueTypeInvalidTaskIdentifier,
		Description: fmt.Sprintf("task_identifier %q is not a valid UUID; replace with a fresh UUIDv4", fm.TaskIdentifier),
		Fixable:     false,
		Fixed:       false,
	}}
}
```

Notes:
- `uuid.Parse` is from `github.com/google/uuid`. It accepts any valid UUID variant; it returns a non-nil error for malformed values, literal placeholders like `<uuid>`, and truncated strings. That single check covers all three failure modes in the task description.
- The two `return nil` early exits (empty value, valid UUID) are what prevent double-fire with `MISSING_TASK_IDENTIFIER` and false-positive on valid values respectively.
- Do NOT add a `detectInvalidTaskIdentifier` helper — `missingTaskIdentifierIssues` uses a separate `detectMissingTaskIdentifier` only because the same detector is called from elsewhere. The invalid-identifier check has only one call site, so the parse + issue-construction live in one function for clarity.

### 4. Wire `invalidTaskIdentifierIssues` into `collectLintIssues` in `pkg/ops/lint.go`

Immediately after the existing line:

```go
issues = append(issues, l.missingTaskIdentifierIssues(filePath, frontmatterYAML)...)
```

add:

```go
// Check for invalid (non-UUID) task_identifier values
issues = append(issues, l.invalidTaskIdentifierIssues(filePath, frontmatterYAML)...)
```

The two checks are siblings and intentionally separate — `missing` fires only when empty/absent, `invalid` fires only when non-empty and unparseable. They are mutually exclusive by construction (see the early `return nil` on empty value in step 3), so no double-fire is possible on any single file.

### 5. Add Ginkgo tests in `pkg/ops/lint_test.go`

Add a new `Context("INVALID_TASK_IDENTIFIER", func() { ... })` block. Placement: immediately AFTER the existing `Context("MISSING_TASK_IDENTIFIER", func() { ... })` block closes (its closing `})` is currently near line 1772), INSIDE the same `Describe("LintOperation - Missing Task Identifier", ...)` block (the `Describe` opener is at line 1704; anchor by that opener, not the line number). Do NOT place the new `Context` inside the sibling `Describe("LintOperation - Status Date Mismatch", ...)` block that begins at line 1775.

The Context MUST contain these `It` blocks (exact assertion shape — mirror the existing `MISSING_TASK_IDENTIFIER` Context's fixture-write + `Execute` + iterate style):

**a. blank value → MISSING fires, INVALID does NOT (no double-fire):**

```go
It("does not report INVALID_TASK_IDENTIFIER when task_identifier is empty (MISSING covers it)", func() {
	content := "---\nstatus: in_progress\npage_type: task\ntask_identifier:\n---\n# Task With Empty Identifier\n"
	taskPath := filepath.Join(vaultPath, tasksDir, "Empty.md")
	Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

	issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
	Expect(err).To(BeNil())
	missingSeen, invalidSeen := false, false
	for _, i := range issues {
		if i.IssueType == ops.IssueTypeMissingTaskIdentifier {
			missingSeen = true
		}
		if i.IssueType == ops.IssueTypeInvalidTaskIdentifier {
			invalidSeen = true
		}
	}
	Expect(missingSeen).To(BeTrue(), "expected MISSING_TASK_IDENTIFIER to fire on empty value")
	Expect(invalidSeen).To(BeFalse(), "INVALID_TASK_IDENTIFIER must NOT fire on empty value")
})
```

**b. valid UUID → no issues of either kind:**

```go
It("does not report any task_identifier issue for a valid UUID", func() {
	content := "---\nstatus: in_progress\npage_type: task\ntask_identifier: 4b54eec9-0a55-4b10-8487-ce78818d831e\n---\n# Task With Valid UUID\n"
	taskPath := filepath.Join(vaultPath, tasksDir, "Valid.md")
	Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

	issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
	Expect(err).To(BeNil())
	for _, i := range issues {
		Expect(i.IssueType).NotTo(Equal(ops.IssueTypeMissingTaskIdentifier))
		Expect(i.IssueType).NotTo(Equal(ops.IssueTypeInvalidTaskIdentifier))
	}
})
```

**c. literal `<uuid>` placeholder → INVALID fires:**

```go
It("detects literal <uuid> placeholder from the task template", func() {
	content := "---\nstatus: in_progress\npage_type: task\ntask_identifier: <uuid>\n---\n# Task From Template\n"
	taskPath := filepath.Join(vaultPath, tasksDir, "Placeholder.md")
	Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

	issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
	Expect(err).To(BeNil())
	found := false
	for _, i := range issues {
		if i.IssueType == ops.IssueTypeInvalidTaskIdentifier {
			found = true
			Expect(i.Fixable).To(BeFalse())
			Expect(i.Description).To(ContainSubstring("<uuid>"))
			Expect(i.Description).To(ContainSubstring("not a valid UUID"))
		}
	}
	Expect(found).To(BeTrue(), "expected INVALID_TASK_IDENTIFIER issue for <uuid>")
})
```

**d. malformed `abc-123` → INVALID fires:**

```go
It("detects malformed task_identifier values", func() {
	content := "---\nstatus: in_progress\npage_type: task\ntask_identifier: abc-123\n---\n# Task With Bad Identifier\n"
	taskPath := filepath.Join(vaultPath, tasksDir, "Malformed.md")
	Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

	issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
	Expect(err).To(BeNil())
	found := false
	for _, i := range issues {
		if i.IssueType == ops.IssueTypeInvalidTaskIdentifier {
			found = true
			Expect(i.Fixable).To(BeFalse())
			Expect(i.Description).To(ContainSubstring("abc-123"))
		}
	}
	Expect(found).To(BeTrue(), "expected INVALID_TASK_IDENTIFIER issue for abc-123")
})
```

**e. missing key entirely → MISSING fires, INVALID does NOT (no double-fire on the absent-key path):**

```go
It("does not report INVALID_TASK_IDENTIFIER when task_identifier key is missing entirely", func() {
	content := "---\nstatus: in_progress\npage_type: task\n---\n# Task Without Identifier Key\n"
	taskPath := filepath.Join(vaultPath, tasksDir, "NoKey.md")
	Expect(os.WriteFile(taskPath, []byte(content), 0600)).To(Succeed())

	issues, err := lintOp.Execute(ctx, vaultPath, tasksDir, false)
	Expect(err).To(BeNil())
	missingSeen, invalidSeen := false, false
	for _, i := range issues {
		if i.IssueType == ops.IssueTypeMissingTaskIdentifier {
			missingSeen = true
		}
		if i.IssueType == ops.IssueTypeInvalidTaskIdentifier {
			invalidSeen = true
		}
	}
	Expect(missingSeen).To(BeTrue(), "expected MISSING_TASK_IDENTIFIER to fire on absent key")
	Expect(invalidSeen).To(BeFalse(), "INVALID_TASK_IDENTIFIER must NOT fire on absent key")
})
```

All five fixtures use `status: in_progress` (not `next`/`backlog`) deliberately, so the unrelated `STATUS_DATE_MISMATCH` rule never enters the picture and the assertions stay narrowly scoped.

### 6. Update `CHANGELOG.md`

Insert a new `## v0.79.0` heading immediately ABOVE the existing `## v0.78.1` heading at the top of the file (after the intro paragraph). The new section body:

```
## v0.79.0

- feat: Add `INVALID_TASK_IDENTIFIER` lint check in `pkg/ops/lint.go` — surfaces when `task_identifier` is present but does not parse as a UUID (catches the literal `<uuid>` placeholder from `90 Templates/Task Template.md`, typos, and truncated values). Closes the gap that let template placeholders ship as real values — `MISSING_TASK_IDENTIFIER` only fires on empty/absent values, so a forgotten `<uuid>` placeholder would otherwise pass lint and then get backfilled to a random UUID by the `WriteTask` fallback on the next write, reintroducing the concurrent-write merge-conflict race on legacy tasks. Non-fixable on purpose: operator must replace with a fresh UUIDv4 (auto-fix would itself become a hidden UUID creation site, defeating the rule's purpose).
```

Use the existing `## v0.78.1` and `## v0.78.0` entries as the style template for tone, length, and the leading `- feat:` / `- fix:` prefix. Do NOT alter any existing changelog entry.

### 7. Bump version to `0.79.0` in the three plugin manifest locations

- `/workspace/.claude-plugin/plugin.json` line 4: `"version": "0.78.1"` → `"version": "0.79.0"`
- `/workspace/.claude-plugin/marketplace.json` line 8: `"version": "0.78.1"` → `"version": "0.79.0"`
- `/workspace/.claude-plugin/marketplace.json` line 15: `"version": "0.78.1"` → `"version": "0.79.0"`

All three MUST be updated in the same prompt. After your edit, `grep -n '"version"' /workspace/.claude-plugin/plugin.json /workspace/.claude-plugin/marketplace.json` MUST show exactly three `0.79.0` lines and zero `0.78.1` lines in those two files.

### 8. Iterative verification

After each section, run `make test` (from the repo root) to catch issues early. Do NOT run `make precommit` iteratively — only run it once at the end per CLAUDE.md guidance.

</requirements>

<constraints>
- Do NOT modify the existing `WriteTask` UUID fallback or any other code path that mints UUIDs — out of scope per the task description.
- Do NOT add a `--fix` path or any auto-fix function for `IssueTypeInvalidTaskIdentifier`. `Fixable: false` is load-bearing: auto-fix would itself become a hidden UUID creation site (the exact bug class this rule is designed to catch). If you find yourself writing a `fixInvalidTaskIdentifier` function, STOP — that contradicts the spec.
- Do NOT wire `EnsureAllTaskIdentifiers` to a CLI command — out of scope.
- The new lint check MUST NOT double-fire with `MISSING_TASK_IDENTIFIER`. The early `return nil` on empty value in step 3 enforces this — do not weaken that guard.
- Existing `IssueType` constants and their wire-format strings MUST NOT change — additive only.
- Tests use Ginkgo v2 / Gomega per project convention. No new interface seams, no Counterfeiter mocks (the lint operation is a concrete `lintOperation`).
- `make precommit` MUST stay green from the repo root.
- Coverage for the new `invalidTaskIdentifierIssues` function MUST be ≥80% per `docs/definition-of-done.md` (the five test cases above will land 100%).
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. New constant present with the spec-pinned wire-format string
grep -n 'IssueTypeInvalidTaskIdentifier' /workspace/pkg/ops/lint.go
# Expected: exactly 2 matches — the const declaration AND the `IssueType:` field in the LintIssue returned by `invalidTaskIdentifierIssues`. The wiring in `collectLintIssues` calls the method `l.invalidTaskIdentifierIssues(...)`, not the constant, so it does NOT increase this count.

grep -n '"INVALID_TASK_IDENTIFIER"' /workspace/pkg/ops/lint.go
# Expected: exactly 1 match — the const declaration

# 2. uuid import added
grep -n '"github.com/google/uuid"' /workspace/pkg/ops/lint.go
# Expected: 1 match

# 3. Detector function exists with the exact name
grep -n 'func.*invalidTaskIdentifierIssues' /workspace/pkg/ops/lint.go
# Expected: 1 match

# 4. NO fix function was added (rule is intentionally non-fixable)
grep -c 'fixInvalidTaskIdentifier' /workspace/pkg/ops/lint.go
# Expected: 0

# 5. Wiring in collectLintIssues
grep -n 'l.invalidTaskIdentifierIssues' /workspace/pkg/ops/lint.go
# Expected: 1 match — the wiring line in collectLintIssues

# 6. Tests are in place
grep -c 'INVALID_TASK_IDENTIFIER' /workspace/pkg/ops/lint_test.go
# Expected: ≥5 (Context label + 4 explicit string assertions in the It blocks)

grep -c 'IssueTypeInvalidTaskIdentifier' /workspace/pkg/ops/lint_test.go
# Expected: ≥5 — one per It block

# 7. Version bumped in all three locations
grep -n '"version"' /workspace/.claude-plugin/plugin.json /workspace/.claude-plugin/marketplace.json
# Expected: 3 lines, all containing 0.79.0; zero 0.78.1 in these files

# 8. CHANGELOG entry under v0.79.0
grep -n '^## v0.79.0' /workspace/CHANGELOG.md
# Expected: 1 match

grep -A2 '^## v0.79.0' /workspace/CHANGELOG.md | grep 'INVALID_TASK_IDENTIFIER'
# Expected: ≥1 match

# 9. Targeted Go test passes
cd /workspace && go test -count=1 -run 'INVALID_TASK_IDENTIFIER|LintOperation' ./pkg/ops/...
# Expected: PASS

# 10. Final gate
cd /workspace && make precommit
# Expected: exit 0
```

AC evidence — synthetic fixture repro (run from `/workspace` AFTER `make install`):

```bash
# Setup: a task with literal <uuid> placeholder (the original bug)
mkdir -p /tmp/iti-fixture-dir
cat > /tmp/iti-fixture-dir/placeholder.md <<'EOF'
---
status: in_progress
page_type: task
task_identifier: <uuid>
---
# Test
EOF

# AC: vault-cli task lint reports INVALID_TASK_IDENTIFIER
vault-cli task lint /tmp/iti-fixture-dir 2>&1 | grep 'INVALID_TASK_IDENTIFIER' | head -1
# Expected: ≥1 line containing "INVALID_TASK_IDENTIFIER" and "<uuid>" and "not a valid UUID"

# AC: --fix does NOT mutate the file (rule is non-fixable)
ORIG_SHA=$(shasum /tmp/iti-fixture-dir/placeholder.md | awk '{print $1}')
vault-cli task lint /tmp/iti-fixture-dir --fix >/dev/null 2>&1 || true
NEW_SHA=$(shasum /tmp/iti-fixture-dir/placeholder.md | awk '{print $1}')
[ "$ORIG_SHA" = "$NEW_SHA" ] && echo "OK: file byte-identical after --fix" || echo "FAIL: file mutated by --fix (rule must be non-fixable)"
```

Coverage check:

```bash
cd /workspace && go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep 'invalidTaskIdentifierIssues' | awk '$NF+0 < 100 {print "FAIL: coverage " $NF " < 100%"; exit 1} {print}'
# Expected: exit 0 with the coverage line printed. Non-zero exit (FAIL) blocks the prompt.
```

</verification>
