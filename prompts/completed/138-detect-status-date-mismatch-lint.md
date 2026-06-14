---
status: completed
spec: [017-enforce-status-in-progress-on-calendar-date]
summary: Added STATUS_DATE_MISMATCH lint check in pkg/ops/lint.go (detector + fix function) wired into collectLintIssues, plus 17 Ginkgo tests covering all 6 (status×date) combinations, 6 no-op cases, 4 fix cases, and lint/validate consistency; refactored collectLintIssues and fixIssues to stay under funlen/gocognit limits
container: vault-cli-status-date-exec-138-detect-status-date-mismatch-lint
dark-factory-version: v0.177.1
created: "2026-06-14T14:30:00Z"
queued: "2026-06-14T15:39:24Z"
started: "2026-06-14T15:39:25Z"
completed: "2026-06-14T15:48:26Z"
branch: dark-factory/enforce-status-in-progress-on-calendar-date
---

<summary>
- New `IssueTypeStatusDateMismatch` issue type surfaces when a task has `status: next` or `status: backlog` AND any of `planned_date`, `defer_date`, or `due_date` is set
- One detector function (`detectStatusDateMismatch`) powers both `vault-cli task lint` and `vault-cli task validate` — single source of truth
- `vault-cli task lint --fix` auto-rewrites the status field to `in_progress`; the date field that triggered the issue is left byte-identical
- Terminal status (`completed`, `aborted`) takes precedence — a stale `defer_date` on a `completed` task is out of scope and not flagged
- Empty date fields (e.g. `defer_date:` with no value) are treated as "no date set" and never trigger the rule
- Ginkgo tests cover the 4 inactive-status × 3 date-field combinations, terminal-status no-op, fix direction, and fix idempotence

</summary>

<objective>
Add a new lint check that detects tasks whose status is `next` or `backlog` while any of `planned_date`, `defer_date`, or `due_date` is set. The detector is wired into `collectLintIssues` (called from both `Execute` and `ExecuteFile`) and an auto-fix promotes the status to `in_progress`. The auto-fix direction is fixed: promote status, never strip date.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Read these files in full before making changes:
- `/workspace/pkg/ops/lint.go` — `IssueType` constants (around line 47-56), `collectLintIssues` (around line 196-280), `fixIssues` switch (around line 590-651), `detectStatusPhaseMismatch` (lines 509-563 — pattern to mirror), `fixStatusCheckboxMismatch` (line 703-757 — fix-function pattern to mirror)
- `/workspace/pkg/ops/lint_test.go` — test patterns; `MISSING_FRONTMATTER` and `INVALID_PRIORITY` Contexts (lines 69-167) show fixture + fix-test style
- `/workspace/pkg/domain/task_frontmatter.go` lines 175-182 (`SetStatus` exists; takes `*TaskFrontmatter` receiver and returns `error` from `Validate`)
- `/workspace/pkg/domain/task_status.go` — `TaskStatusNext`, `TaskStatusBacklog`, `TaskStatusInProgress`, `TaskStatusCompleted`, `TaskStatusAborted` constants
- `/workspace/pkg/domain/task_frontmatter.go` lines 73-78 (`DeferDate()`), 302-318 (`SetDeferDate`/`SetPlannedDate`/`SetDueDate` and their getter pattern)

`ExecuteFile` (line 109) already calls `lintFile` with `fix=false` and the same `collectLintIssues` is invoked once, so wiring the new check into `collectLintIssues` automatically makes `task validate` use the same detector (no separate wiring needed — `ExecuteFile` reuses the same `collectLintIssues` path). Verify this by reading `lintFile` (lines 137-167) and `ExecuteFile` (lines 109-120).

Note: The spec references `pkg/ops/lint.go` lines 509-563 for the `detectStatusPhaseMismatch` pattern. In the actual file as of HEAD, that function is at lines 511-563 (close enough; the line range will drift — use the function name as the anchor, not the line numbers).
</context>

<requirements>

### 1. Add new `IssueType` constant in `pkg/ops/lint.go`

In the const block at lines 47-56, add:

```go
IssueTypeStatusDateMismatch IssueType = "STATUS_DATE_MISMATCH"
```

Additive only — do not change any existing `IssueType` wire-format strings.

### 2. Add `detectStatusDateMismatch` method to `lintOperation` in `pkg/ops/lint.go`

Add a new method (placement: directly after `detectStatusPhaseMismatch` at line 563, before `missingTaskIdentifierIssues` at line 566). Signature and behavior:

```go
// detectStatusDateMismatch detects tasks whose status is next or backlog
// while any of planned_date, defer_date, or due_date is set.
// Per spec 017: calendar dates are commitments; only in_progress and terminal
// statuses are compatible with a date on an unstarted task.
// Returns: (issueFound, description)
func (l *lintOperation) detectStatusDateMismatch(frontmatterYAML string) (bool, string) {
    // Parse status
    statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
    statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
    if len(statusMatches) < 2 {
        return false, ""
    }
    status := domain.TaskStatus(statusMatches[1])

    // Only flag inactive statuses; completed/aborted/hold/in_progress are out of scope
    if status != domain.TaskStatusNext && status != domain.TaskStatusBacklog {
        return false, ""
    }

    // Check for any date field with a non-empty value
    // Match: `field: <value>` where value is non-empty (not just whitespace, not empty)
    dateRegex := regexp.MustCompile(`(?m)^(planned_date|defer_date|due_date):\s*['"]?([^\s'"]+)?['"]?\s*$`)
    matches := dateRegex.FindAllStringSubmatch(frontmatterYAML, -1)
    for _, m := range matches {
        if len(m) >= 3 && m[2] != "" {
            return true, fmt.Sprintf(
                "status is %s but %s is set (calendar dates are commitments; expected in_progress)",
                status, m[1],
            )
        }
    }
    return false, ""
}
```

Notes on the regex:
- The capture group `([^\s'"]+)?` makes the value optional but anchored to at least one non-whitespace/non-quote char. An empty `defer_date:` line (no value) will NOT match this branch and is correctly treated as "no date set".
- Only one matching date field is needed to trigger — the loop returns on the first hit.

### 3. Wire `detectStatusDateMismatch` into `collectLintIssues` in `pkg/ops/lint.go`

In `collectLintIssues` (lines 196-280), add a new check block directly after the `IssueTypeStatusPhaseMismatch` block (around line 249) and before the `IssueTypeOrphanGoal` block (around line 251). Use the same emit pattern as the phase-mismatch block. Set `Fixable: true` (this issue is fixable — promoting `next`/`backlog` to `in_progress` resolves it).

```go
// Check for status/date mismatch (calendar-as-commitment rule)
if mismatchIssue, mismatchDesc := l.detectStatusDateMismatch(frontmatterYAML); mismatchIssue {
    issues = append(issues, LintIssue{
        FilePath:    filePath,
        IssueType:   IssueTypeStatusDateMismatch,
        Description: mismatchDesc,
        Fixable:     true,
        Fixed:       false,
    })
}
```

This automatically makes `task validate` (which goes through `ExecuteFile` → `lintFile` → `collectLintIssues`) surface the same issue with the same description string. Do NOT add a separate wiring path.

### 4. Add `fixStatusDateMismatch` method to `lintOperation` in `pkg/ops/lint.go`

Add a new method (placement: directly after `fixStatusCheckboxMismatch` at line 757, before `fixMissingFrontmatter` at line 760). Behavior: replace the `status:` value with `in_progress`. ONLY when current status is `next` or `backlog`. Leave the date fields byte-identical. Returns `(newContent, fixed)`.

```go
// fixStatusDateMismatch promotes status from next/backlog to in_progress
// when a date field is set. Per spec 017: calendar-as-commitment rule auto-fixes
// the status, never strips the date. Idempotent on in_progress (no rewrite).
func (l *lintOperation) fixStatusDateMismatch(content string) (string, bool) {
    statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
    matches := statusRegex.FindStringSubmatch(content)
    if len(matches) < 2 {
        return content, false
    }
    current := matches[1]
    if current != "next" && current != "backlog" {
        return content, false
    }
    newContent := statusRegex.ReplaceAllString(content, "status: in_progress")
    return newContent, true
}
```

The fix is intentionally narrow: only the `status:` line changes. The date field that triggered the issue is in a different line of the frontmatter and is not touched.

### 5. Add the new case to the `fixIssues` switch in `pkg/ops/lint.go`

In the switch at line 605-641, add a new case after `IssueTypeStatusCheckboxMismatch` (which ends at line 640):

```go
case IssueTypeStatusDateMismatch:
    // Fix status/date mismatch by promoting status to in_progress
    newContent, fixed := l.fixStatusDateMismatch(updatedContent)
    if fixed {
        updatedContent = newContent
        issues[i].Fixed = true
        modified = true
    }
```

### 6. Add tests in `pkg/ops/lint_test.go`

Add a new `Context("STATUS_DATE_MISMATCH", func() { ... })` block (placement: after the existing `STATUS_CHECKBOX_MISMATCH` Context if present, or after the `MISSING_TASK_IDENTIFIER` Context — verify by reading the end of the test file to find a good insertion point). The block must contain the following `It(...)` blocks:

**Detection — each (status × date-field) combination must trigger:**

- `It("detects status: next + defer_date", ...)` — fixture with `status: next` + `defer_date: 2026-12-01`; expect `HaveLen(1)`, `IssueType == IssueTypeStatusDateMismatch`, `Fixable == true`, `Description` contains "status is next" and "defer_date"
- `It("detects status: next + planned_date", ...)` — same shape, with `planned_date: 2026-12-01`; description must contain "planned_date"
- `It("detects status: next + due_date", ...)` — description must contain "due_date"
- `It("detects status: backlog + defer_date", ...)` — description must contain "status is backlog" and "defer_date"
- `It("detects status: backlog + planned_date", ...)` — description must contain "planned_date"
- `It("detects status: backlog + due_date", ...)` — description must contain "due_date"

**No-op — terminal and active statuses must NOT trigger:**

- `It("does not flag status: in_progress + defer_date", ...)` — fixture `status: in_progress` + `defer_date: 2026-12-01`; expect 0 STATUS_DATE_MISMATCH issues
- `It("does not flag status: completed + defer_date", ...)` — `status: completed` + `defer_date: 2026-12-01`; expect 0 STATUS_DATE_MISMATCH issues
- `It("does not flag status: aborted + defer_date", ...)` — `status: aborted` + `defer_date: 2026-12-01`; expect 0 STATUS_DATE_MISMATCH issues
- `It("does not flag status: hold + defer_date", ...)` — `status: hold` + `defer_date: 2026-12-01`; expect 0 STATUS_DATE_MISMATCH issues (hold is out of scope per spec — the spec lists "next" and "backlog" as the only flagged statuses; hold is non-terminal but not the focus of this spec)
- `It("does not flag status: next + no date field", ...)` — `status: next` with no `planned_date` / `defer_date` / `due_date`; expect 0 STATUS_DATE_MISMATCH issues
- `It("does not flag status: next + empty defer_date", ...)` — `status: next` + `defer_date:` (no value); expect 0 STATUS_DATE_MISMATCH issues (empty value treated as "no date set")

**Auto-fix:**

- `It("fixes status: next + defer_date by promoting to in_progress", ...)` — write fixture, run `Execute(ctx, vaultPath, tasksDir, true)`, then read file back and assert `ContainSubstring("status: in_progress")` AND `ContainSubstring("defer_date: 2026-12-01")` (date unchanged)
- `It("fixes status: backlog + due_date by promoting to in_progress", ...)` — assert `ContainSubstring("status: in_progress")` AND `ContainSubstring("due_date: 2026-12-01")` (due_date unchanged)
- `It("leaves all other frontmatter fields byte-identical", ...)` — fixture with `priority: 1`, `assignee: bborbe`, `task_identifier: <uuid>`, plus `defer_date: 2026-12-01`; after fix, assert all four are still present byte-identical
- `It("does not touch terminal-status files", ...)` — `status: completed` + `defer_date: 2026-12-01`; run with `fix=true`; assert file is byte-identical to original (no rewrite)

**Lint/validate consistency (AC 3 — same detector powers both):**

- `It("ExecuteFile surfaces STATUS_DATE_MISMATCH for the same fixture", ...)` — write the same `status: next` + `defer_date: 2026-12-01` fixture; call `ExecuteFile(ctx, path, "task", "vault")` (read the actual signature at line 109); assert the returned issues contain one with `IssueType == IssueTypeStatusDateMismatch` AND `Description` matches the string from the `Execute` run for the same file (string equality on `Description`).

Use the `MISSING_FRONTMATTER` and `INVALID_PRIORITY` Contexts as the style template (lines 69-167). For the `ExecuteFile` test, use the actual signature from lines 109-120.

### 7. Iterative verification

After each section of changes, run `make test` (from the repo root) to catch issues early. Do NOT run `make precommit` iteratively — only run it once at the end per CLAUDE.md guidance.

</requirements>

<constraints>
- The detector MUST be a single function invoked by both `Execute` (lint) and `ExecuteFile` (validate) — no re-implementation in the validate path. Adding the check to `collectLintIssues` (called from `lintFile`) satisfies this.
- Auto-fix direction is fixed: promote `next`/`backlog` to `in_progress`, never strip the date field. If a future caller needs the inverse fix, that requires a new spec.
- Empty date values (`defer_date:` with no value) MUST be treated as "no date set" and MUST NOT trigger the rule.
- Terminal status (`completed`, `aborted`) and active status (`in_progress`, `hold`) MUST NOT trigger the rule — out of scope per spec Non-goals.
- Existing `IssueType` constants and their wire-format strings MUST NOT change — additive only.
- Tests use Ginkgo v2 / Gomega per project convention. No new interface seams; no Counterfeiter mocks required (the lint operation is a concrete `lintOperation`).
- `make precommit` MUST stay green from the repo root (no per-package Makefile in this repo).
- Do NOT commit — dark-factory handles git.
- Coverage for the new detector function MUST be ≥80% per `docs/definition-of-done.md`.

</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. New issue type constant added (wire-format string)
grep -n 'IssueTypeStatusDateMismatch' pkg/ops/lint.go
# Expected: 3 matches — the const declaration, the collectLintIssues emit, the fixIssues case

# 2. Wire-format string is the spec-pinned value
grep -n '"STATUS_DATE_MISMATCH"' pkg/ops/lint.go
# Expected: 1 match — the const declaration

# 3. Detector function exists
grep -n 'func.*detectStatusDateMismatch' pkg/ops/lint.go
# Expected: 1 match

# 4. Fix function exists
grep -n 'func.*fixStatusDateMismatch' pkg/ops/lint.go
# Expected: 1 match

# 5. AC 1: lint surfaces status_date_mismatch for status:next + defer_date
echo "Run on synthetic fixture — see AC evidence section below"

# 6. AC 2: lint --fix rewrites next -> in_progress, leaves defer_date unchanged
echo "Run on synthetic fixture — see AC evidence section below"

# 7. AC 3: ExecuteFile (validate) surfaces the same issue with the same description
echo "Test in lint_test.go verifies string equality between lint and validate outputs"

# 8. AC 12: Ginkgo test count for new STATUS_DATE_MISMATCH Context
grep -c 'It(".*status.*date.*mismatch\|It(".*status.*in_progress.*date\|It(".*status.*backlog.*date\|It(".*fixes.*status.*date\|It(".*does not flag.*status.*in_progress.*date\|It(".*does not flag.*status.*completed.*date\|It(".*does not flag.*status.*aborted.*date\|It(".*does not flag.*status.*hold.*date\|It(".*does not flag.*status.*next.*no date\|It(".*does not flag.*status.*next.*empty defer_date\|It(".*leaves all other frontmatter\|It(".*does not touch terminal\|It(".*ExecuteFile surfaces' pkg/ops/lint_test.go
# Expected: ≥13 (all new It blocks present)

# 9. AC 13: make precommit exits 0
make precommit
# Expected: "ready to commit"
```

AC evidence — synthetic fixture repro (run these from the repo root AFTER `make install`):

```bash
# Setup: a task with status: next + defer_date
cat > /tmp/sdm-next-defer.md <<'EOF'
---
status: next
page_type: task
priority: 3
defer_date: 2026-12-01
task_identifier: sdm-test
---
# Test
EOF

# AC 1: vault-cli task lint reports STATUS_DATE_MISMATCH
vault-cli task lint /tmp/sdm-fixture-dir 2>&1 | grep 'STATUS_DATE_MISMATCH' | head -1
# Expected: ≥1 line containing "STATUS_DATE_MISMATCH" and "defer_date"

# AC 2: vault-cli task lint --fix promotes status to in_progress
cp /tmp/sdm-next-defer.md /tmp/sdm-fix-test.md
mkdir -p /tmp/sdm-fix-dir
cp /tmp/sdm-fix-test.md /tmp/sdm-fix-dir/
vault-cli task lint /tmp/sdm-fix-dir --fix
grep -c '^status: in_progress' /tmp/sdm-fix-dir/sdm-fix-test.md   # expected: 1
grep -c '^status: next' /tmp/sdm-fix-dir/sdm-fix-test.md          # expected: 0
grep '^defer_date:' /tmp/sdm-fix-dir/sdm-fix-test.md              # expected: defer_date: 2026-12-01 (byte-identical)
```

Coverage check:

```bash
go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep -E "detectStatusDateMismatch|fixStatusDateMismatch"
# Expected: both functions at 100% coverage
```

</verification>
