---
status: completed
summary: 'Normalised task status and phase aliases at the SetField boundary: `todo` maps to `next` and `in_progress` maps to `execution` before validation, canonical form is written to disk, and Validate() canonical-only contract is preserved.'
container: vault-cli-exec-122-normalize-aliases-in-setfield
dark-factory-version: v0.162.0
created: "2026-05-20T19:35:00Z"
queued: "2026-05-20T19:36:58Z"
started: "2026-05-20T19:37:00Z"
completed: "2026-05-20T19:39:26Z"
---

<summary>
- `vault-cli task set <id> phase in_progress` currently fails with `unknown task phase 'in_progress': validation error` even though the documented strategy says old values stay accepted forever as aliases.
- Same bug for `vault-cli task set <id> status todo` — fails with `unknown task status 'todo'`.
- Root cause: `TaskFrontmatter.SetField` calls `Validate()` directly on the raw operator input. `Validate()` only accepts canonical (`next`, `execution`); `Normalize*()` is never consulted on the write path.
- The fix normalises the input at the `SetField` boundary: operator-supplied old canonical (`todo`, `in_progress`) is accepted, normalised to new canonical (`next`, `execution`), then written to disk as canonical.
- Existing tests asserting `TaskStatusTodo.Validate()` and `TaskPhaseInProgress.Validate()` FAIL stay green — the canonical-only `Validate` contract is preserved. The fix is at the write-path entry, not in `Validate`.
- task-orchestrator drag-and-drop (still emits `phase=in_progress` to the backend during transition) starts working again immediately after the patch ships.
</summary>

<objective>
Make `vault-cli task set <id> status todo` and `vault-cli task set <id> phase in_progress` succeed: aliases are normalised to canonical at the `SetField` boundary, the canonical form is written to disk, and `Validate()`'s canonical-only contract is preserved. Existing tests on `Validate()` (which assert aliases reject) stay unchanged.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read `~/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` for the `errors.Wrapf(ctx, validation.Error, ...)` pattern.

Read `~/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` for the changelog entry style.

Read the parent goal context in CHANGELOG.md and the comment block at the top of `pkg/domain/task_status.go` lines 18-31 — the file already documents the alias intent ("kept for backward compatibility" / "accepted as an alias via NormalizeTaskStatus") but the SetField write path doesn't honour it.

Read `pkg/domain/task_phase.go` in full. Note:
- `TaskPhaseExecution` is canonical; `TaskPhaseInProgress` is the alias constant (defined, but NOT in `AvailableTaskPhases`).
- `Validate()` (line 62) checks `AvailableTaskPhases.Contains(t)` — rejects alias by design.
- `NormalizeTaskPhase(raw string) (TaskPhase, bool)` (line 81) already exists and maps `in_progress` → `TaskPhaseExecution`.

Read `pkg/domain/task_status.go` in full. Same shape:
- `TaskStatusNext` is canonical; `TaskStatusTodo` is the alias constant.
- `Validate()` rejects alias.
- `NormalizeTaskStatus(raw string) (TaskStatus, bool)` already maps `todo` → `TaskStatusNext`.

Read `pkg/domain/task_frontmatter.go` from line 170 to line 460. Pay attention to:
- `SetStatus(s TaskStatus) error` (line 175) — calls `s.Validate(context.Background())`. Public method, may be called from elsewhere; keep its canonical-only contract.
- `setPhaseField(ctx, value string) error` (line 402) — receives string value from `SetField`, casts to `TaskPhase`, calls `Validate()`. Internal helper.
- `SetField(ctx, key, value string) error` (line 417) — operator-facing write entry. The `"status"` and `"phase"` cases are the ones that fail today.

Read `pkg/domain/task_status_test.go` and `pkg/domain/task_phase_test.go` in full before adding tests. Note especially:
- `task_status_test.go:33` asserts `TaskStatusTodo.Validate(ctx)` returns non-nil — stays green (we don't touch Validate).
- `task_phase_test.go:83` asserts `AvailableTaskPhases.Contains(TaskPhaseInProgress)` is `false` — stays green.

Read `pkg/domain/task_frontmatter_test.go` to find existing `SetField` test patterns to mirror.
</context>

<requirements>

### 1. Normalise status alias in `SetField` (pkg/domain/task_frontmatter.go, `case "status"` ~line 419)

Find:

```go
	case "status":
		return f.SetStatus(TaskStatus(value))
```

Replace with:

```go
	case "status":
		canonical, ok := NormalizeTaskStatus(value)
		if !ok {
			return errors.Wrapf(ctx, validation.Error, "unknown task status '%s'", value)
		}
		return f.SetStatus(canonical)
```

**Why:** `SetStatus` then receives only canonical values; its existing `Validate` call continues to accept them. Operator-supplied alias (`todo`) is mapped to `next` before validation, and the canonical form (`next`) is what lands in frontmatter on disk.

### 2. Normalise phase alias in `setPhaseField` (pkg/domain/task_frontmatter.go ~line 402)

Find:

```go
func (f *TaskFrontmatter) setPhaseField(ctx context.Context, value string) error {
	if value == "" {
		f.SetPhase(nil)
		return nil
	}
	p := TaskPhase(value)
	if err := p.Validate(ctx); err != nil {
		return err
	}
	f.SetPhase(&p)
	return nil
}
```

Replace with:

```go
func (f *TaskFrontmatter) setPhaseField(ctx context.Context, value string) error {
	if value == "" {
		f.SetPhase(nil)
		return nil
	}
	canonical, ok := NormalizeTaskPhase(value)
	if !ok {
		return errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", value)
	}
	f.SetPhase(&canonical)
	return nil
}
```

**Why:** Operator-supplied alias (`in_progress`) is normalised to `execution` before being stored. Unknown values continue to be rejected via the same `validation.Error` class. `SetPhase` receives only canonical pointers.

### 3. Add the `validation` import

After steps 1 and 2, `pkg/domain/task_frontmatter.go` uses `errors.Wrapf` and `validation.Error`. The `github.com/bborbe/errors` import is already present, but `github.com/bborbe/validation` is NOT. Add it to the import block — `goimports` (run by `make precommit`) may also add it automatically; either path is fine, but the resulting file must compile and pass lint.

### 4. New tests in `pkg/domain/task_frontmatter_test.go`

**IMPORTANT — match the existing test conventions in this file before writing.** Mirror the construction pattern at the top of `task_frontmatter_test.go` (around line 26): `fm` is declared as `var fm domain.TaskFrontmatter` (value, NOT pointer) and constructed via `fm = domain.NewTaskFrontmatter(nil)` (takes a `map[string]any`, nil for empty). Do NOT invent a new no-arg constructor.

Add a new top-level `Describe` block at the end of the file (there is no existing `SetField`-specific Describe to extend — the existing blocks are per-field accessor):

```go
var _ = Describe("TaskFrontmatter SetField alias normalization", func() {
	var ctx context.Context
	var fm domain.TaskFrontmatter

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewTaskFrontmatter(nil)
	})

	Context("status field", func() {
		It("normalises alias 'todo' to canonical 'next' on disk", func() {
			Expect(fm.SetField(ctx, "status", "todo")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusNext))
		})

		It("accepts canonical 'next' verbatim", func() {
			Expect(fm.SetField(ctx, "status", "next")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.TaskStatusNext))
		})

		It("rejects an unknown status value with validation.Error", func() {
			err := fm.SetField(ctx, "status", "banana")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
		})
	})

	Context("phase field", func() {
		It("normalises alias 'in_progress' to canonical 'execution' on disk", func() {
			Expect(fm.SetField(ctx, "phase", "in_progress")).To(Succeed())
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhaseExecution))
		})

		It("accepts canonical 'execution' verbatim", func() {
			Expect(fm.SetField(ctx, "phase", "execution")).To(Succeed())
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TaskPhaseExecution))
		})

		It("clears the phase on empty value", func() {
			Expect(fm.SetField(ctx, "phase", "execution")).To(Succeed())
			Expect(fm.SetField(ctx, "phase", "")).To(Succeed())
			Expect(fm.Phase()).To(BeNil())
		})

		It("rejects an unknown phase value with validation.Error", func() {
			err := fm.SetField(ctx, "phase", "banana")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
		})
	})
})
```

Verify imports in `task_frontmatter_test.go` already include `errors "github.com/bborbe/errors"` and `"github.com/bborbe/validation"`. If `validation` is missing, add it. `errors.Is` should resolve via the bborbe-errors import (it re-exports the stdlib symbol). Do not duplicate or rename imports — match the existing alias convention in the file.

### 5. CHANGELOG entry

Open `CHANGELOG.md`. Check whether a `## Unreleased` section already exists.

- If `## Unreleased` exists: append the bullet below under it.
- If `## Unreleased` does NOT exist: create a new `## Unreleased` section above the topmost version section, then add the bullet under it.

```
- fix: `vault-cli task set <id> {status|phase}` accepts the legacy aliases `todo` and `in_progress` again — both are normalised to canonical (`next`, `execution`) before validation, and the canonical form is written to disk. Restores the alias acceptance documented in the rename strategy ([[Rename Task Status and Phase Taxonomy]]) that was missing on the write path.
```

### 6. Sanity-check greps

After editing, run:

```bash
grep -n "NormalizeTaskStatus(value)" pkg/domain/task_frontmatter.go
```
Expected: one match inside the `"status"` case of `SetField`.

```bash
grep -n "NormalizeTaskPhase(value)" pkg/domain/task_frontmatter.go
```
Expected: one match inside `setPhaseField`.

```bash
grep -n "TaskStatus(value)" pkg/domain/task_frontmatter.go
```
Expected: zero matches inside `SetField` (the old direct cast is gone).

```bash
grep -n "p := TaskPhase(value)" pkg/domain/task_frontmatter.go
```
Expected: zero matches.

</requirements>

<constraints>
- DO NOT modify `Validate()` on either `TaskStatus` or `TaskPhase`. The canonical-only `Validate` contract is intentional and tested.
- DO NOT add `TaskStatusTodo` to `AvailableTaskStatuses` or `TaskPhaseInProgress` to `AvailableTaskPhases`. Existing tests assert these are absent; respect that design.
- DO NOT change `SetStatus(s TaskStatus) error` or `SetPhase(*TaskPhase)`. Both stay canonical-only. The normalization happens at the `SetField` boundary only.
- DO NOT add a new public function. Reuse the existing `NormalizeTaskStatus` and `NormalizeTaskPhase`.
- DO NOT bulk-migrate vault frontmatter files. Old values on disk remain valid for reads via the same normalize path; this prompt does not touch any vault file.
- Use `github.com/bborbe/errors`'s `errors.Wrapf` with `validation.Error` for the unknown-value error class — matches the style used elsewhere in the package.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass unmodified. The new tests are added at the end of the existing test file (or as a new `_test.go` file in the same package if structurally cleaner — follow the file conventions).
</constraints>

<verification>
Run `make precommit` — must exit 0.

Confirm the alias path:
```bash
grep -n "NormalizeTaskStatus(value)" pkg/domain/task_frontmatter.go
grep -n "NormalizeTaskPhase(value)" pkg/domain/task_frontmatter.go
```

Confirm Validate contract is untouched:
```bash
git diff pkg/domain/task_status.go pkg/domain/task_phase.go
```
Expected: no diff for these two files (they are intentionally NOT modified).

Run the new tests specifically:
```bash
go test ./pkg/domain/ -run TaskFrontmatter -v
```
Expected: all new alias-normalization specs pass.

Run the full suite:
```bash
make test
```
Expected: green, no skipped tests, no regressions on the existing `Validate` tests.
</verification>
