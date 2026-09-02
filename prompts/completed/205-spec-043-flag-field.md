---
status: completed
spec: [043-flag-field]
summary: Added boolean flag field to task frontmatter with Flag/SetFlag/ClearFlag accessors, validated setFlagField string-boundary helper, SetField/GetField dispatch routing, domain + ops tests, and CHANGELOG feat entry under Unreleased
execution_id: vault-cli-exec-205-spec-043-flag-field
dark-factory-version: dev
created: "2026-09-02T20:08:50Z"
queued: "2026-09-02T20:29:47Z"
started: "2026-09-02T20:29:49Z"
completed: "2026-09-02T20:33:31Z"
---

# Add the boolean `flag` field to task frontmatter

<summary>
- Task frontmatter gains a boolean `flag` field read through the existing `GetBool` coercion.
- `task set "T" flag true|yes` writes a real YAML bool `flag: true`; `false|no` writes `flag: false` — all case-insensitive, matching the read-path coercion.
- Any other value (e.g. `banana`) is rejected with an error naming the value and the accepted set, and no write occurs.
- `task get "T" flag` prints `true` / `false` when set, empty when absent — absent and explicit-false stay distinct.
- `task clear "T" flag` removes the `flag` key entirely; a task that was never flagged never gains a `flag:` line on any write.
- The flag is orthogonal to `status`, `phase`, `priority` and the date fields — setting it changes no other frontmatter key.
- CLI `get` / `set` / `clear` work through the existing generic dispatch — no new commands and no ops-layer changes.
- Ops-level tests prove the invalid-value error surfaces and the file is not written on failure.
- An existing task carrying an unparseable `flag: banana` reads as un-flagged and keeps the raw value until explicitly set or cleared.
- The changelog gains a `feat:` entry under `## Unreleased`.
</summary>

<objective>
Give a task a boolean `flag` in frontmatter that is writable, readable and clearable through the vault-cli typed surface with validation, so a focus pick can be expressed without corrupting `planned_date` or `priority` — the field the Vault UI will later sort to the top. This is a single-layer change: the domain frontmatter type plus its tests; the CLI and ops layers need no changes because the existing generic `SetField` / `GetField` / `ClearField` dispatch routes `flag` through the new validated setter.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` (the `## Entity Structure` section, `**Frontmatter** (pkg/domain/<entity>_frontmatter.go)` bullet list) for project conventions.

Read these files fully before making changes:

- `/workspace/pkg/domain/task_frontmatter.go` — the file you change. It is a `FrontmatterMap`-backed typed wrapper. Verified current shape of the patterns you mirror:
  - `Priority()` value-receiver getter (reads via `f.Get`, coerces, returns zero value on absence) and `SetPriority` — the validated-setter pattern at `task_frontmatter.go:295`:
    ```go
    func (f *TaskFrontmatter) SetPriority(ctx context.Context, p Priority) error {
        if err := p.Validate(ctx); err != nil {
            return errors.Wrap(ctx, err, "invalid priority")
        }
        f.Set("priority", int(p))
        return nil
    }
    ```
  - `ClearClaudeSessionID` — the delete-the-key pattern (pointer receiver, `f.Delete`).
  - `setPriorityField` — the string-boundary helper that parses, validates and clears-on-empty; `setPhaseField` — the validated helper that returns `errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", value)` for an invalid value. `strings`, `strconv`, `errors` (github.com/bborbe/errors) and `validation` are already imported — do not add or remove imports.
  - `GetField` (line ~342) and `SetField` (line ~444) — the dispatch switch you extend with a `flag` case. `strconv` is already imported and used by the `priority` GetField case.
- `/workspace/pkg/domain/frontmatter_map.go` — `GetBool(key string) bool` (coercing bool accessor) and `Get(key string) any`; verify the coercion rules you rely on (case-insensitive `true`/`yes` → true, everything else → false).
- `/workspace/pkg/domain/task_frontmatter_test.go` — the existing Ginkgo suite for `TaskFrontmatter` (`package domain_test`). Fixture: `BeforeEach` sets `fm = domain.NewTaskFrontmatter(nil)` and `ctx = context.Background()`. `errors`, `validation` and `yaml` are already imported. Existing blocks to extend: `Describe("Priority", ...)` / `Describe("SetPriority", ...)` (Setter/accessor pattern), `Describe("GetField", ...)` (ends at the `It("returns raw value for unknown key"...)`), `Describe("SetField", ...)` (ends at `It("stores unknown field without error"...)`).
- `/workspace/pkg/domain/task_frontmatter_metrics_test.go` — the in-repo serializer round-trip pattern to copy: `yaml.Marshal(fm.RawMap())`, assert on the serialized text, then `yaml.Unmarshal` back into `map[string]any` and rebuild with `domain.NewTaskFrontmatter(raw)`.
- `/workspace/pkg/ops/frontmatter.go` — confirm the ops layer is generic: `FrontmatterSetOperation.Execute` calls `task.SetField(ctx, key, value)` then `WriteTask`; `FrontmatterGetOperation.Execute` calls `task.GetField(key)`; `FrontmatterClearOperation.Execute` calls `task.ClearField(key)`. No ops-layer changes are needed.
- `/workspace/pkg/ops/frontmatter_test.go` — the ops test pattern to extend: `Describe("FrontmatterSetOperation", ...)` uses `mockTaskStorage`, checks `mockTaskStorage.WriteTaskCallCount()` and `_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)`; the invalid-field context asserts the error substring and `WriteTaskCallCount() == 0`. `Describe("FrontmatterGetOperation", ...)` uses `result, err = getOp.Execute(...)`.
- `/workspace/CHANGELOG.md` — a `## Unreleased` section already exists with one `fix:` bullet; add a `feat:` bullet below it.

Read these coding-plugin docs (present in the container):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, table tests.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.

<!-- OPEN QUESTIONS (resolved by the prompt writer; flagged for the human reviewer):
1. SetFlag takes `_ context.Context` (not a named `ctx`) because `.golangci.yml` enables `unparam`, which rejects an unused named parameter — and a bool has no invalid state to validate, so the validation lives at the string boundary in `setFlagField`. The signature stays `SetFlag(_ context.Context, v bool) error`, matching the spec's arity/types/error-return and the spec's own `grep 'func (f \*TaskFrontmatter) SetFlag'` verification.
2. `GetField("flag")` distinguishes absent (raw value nil → "") from explicit-false (key present → "false"), which AC3 requires. A present-but-unparseable value (e.g. `flag: banana`) reads as "false", consistent with `Flag()`.
-->
</context>

<requirements>

## 1. Add the typed accessors to `pkg/domain/task_frontmatter.go`

### 1a. `Flag()` getter

Insert this method immediately **after** the existing `Priority()` method (the getter that ends with `return 0` inside the type switch) and before `Assignee()`:

```go
// Flag reads the "flag" key via GetBool coercion.
// Returns true for a YAML bool true or the strings "true"/"yes" (case-insensitive,
// surrounding whitespace trimmed); false for a missing key, a false value, or an
// unrecognised value.
func (f TaskFrontmatter) Flag() bool { return f.GetBool("flag") }
```

### 1b. `SetFlag(ctx, bool) error` and `ClearFlag()`

Insert these two methods immediately **after** the existing `SetPriority` method (which ends with `return nil` after `f.Set("priority", int(p))`) and before `SetPhase`:

```go
// SetFlag stores the flag in the map. A bool has no invalid state, so this
// setter mirrors the SetPriority signature (ctx + error) for the validated
// field family and always returns nil; value validation happens at the string
// boundary in setFlagField. The parameter is named `_` because the unparam
// linter (enabled in .golangci.yml) rejects an unused named parameter.
func (f *TaskFrontmatter) SetFlag(_ context.Context, v bool) error {
	f.Set("flag", v)
	return nil
}

// ClearFlag removes the flag key entirely, so the task reads back as un-flagged
// and the key is never emitted on the next write.
func (f *TaskFrontmatter) ClearFlag() { f.Delete("flag") }
```

Do not add overloads, defaults, sentinels, or an error return beyond the signature shown. Do not change `Priority`, `SetPriority`, `SetPhase` or any other existing method.

## 2. Add the validated string-boundary helper `setFlagField`

Insert this method immediately **after** the existing `setPriorityField` method (which ends with `return f.SetPriority(ctx, Priority(n))`) and before `setPhaseField`:

```go
// setFlagField parses a boolean string and stores the flag, or deletes on empty.
// Accepted values match GetBool's coercion: "true"/"yes" and "false"/"no",
// case-insensitive with surrounding whitespace trimmed. The canonical form
// stored is a real YAML bool (true or false).
func (f *TaskFrontmatter) setFlagField(ctx context.Context, value string) error {
	if value == "" {
		f.ClearFlag()
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes":
		return f.SetFlag(ctx, true)
	case "false", "no":
		return f.SetFlag(ctx, false)
	default:
		return errors.Wrapf(ctx, validation.Error, "invalid flag value '%s' (accepted values: true/yes/false/no)", value)
	}
}
```

The error message MUST contain the offending value and the accepted set `true`/`yes`/`false`/`no` — AC2 asserts stderr names both.

## 3. Route `flag` through the `SetField` dispatch

In `pkg/domain/task_frontmatter.go`, in `func (f *TaskFrontmatter) SetField(ctx context.Context, key, value string) error`, insert a new case **immediately after** the existing `priority` case (which reads `case "priority": return f.setPriorityField(ctx, value)`) and before the `assignee` case:

```go
	case "flag":
		return f.setFlagField(ctx, value)
```

This is what makes the generic `task set "T" flag <value>` path validate instead of storing `flag` as a free-form string. The `default` case must still handle unknown keys exactly as before.

## 4. Route `flag` through the `GetField` dispatch

In `pkg/domain/task_frontmatter.go`, in `func (f TaskFrontmatter) GetField(key string) string`, insert a new case **immediately after** the existing `priority` case (the one that ends with `return strconv.Itoa(int(p))`) and before the `assignee` case:

```go
	case "flag":
		if f.Get("flag") == nil {
			return ""
		}
		return strconv.FormatBool(f.GetBool("flag"))
```

This makes `task get "T" flag` print `true` / `false` when the key is present and empty when absent (AC3). Absent and explicit-false are distinct: absent → `""`, `flag: false` → `"false"`. A present-but-unparseable value (e.g. `flag: banana`) prints `"false"`, consistent with `Flag()`.

## 5. Domain tests in `pkg/domain/task_frontmatter_test.go`

All new tests follow the existing Ginkgo v2 / Gomega style in `package domain_test`, using the `fm` / `ctx` fixture from the enclosing `Describe("TaskFrontmatter", ...)` `BeforeEach`.

### 5a. Accessor, setter and clearer blocks

Insert three new `Describe` blocks immediately **after** the existing `Describe("SetPriority", ...)` block (the one that ends with the `It("returns error for negative priority"...)`) and before `Describe("Goals", ...)`:

```go
	Describe("Flag", func() {
		DescribeTable("coerces the stored value",
			func(stored any, expected bool) {
				fm = domain.NewTaskFrontmatter(map[string]any{"flag": stored})
				Expect(fm.Flag()).To(Equal(expected))
			},
			Entry("bool true", true, true),
			Entry("bool false", false, false),
			Entry("string true", "true", true),
			Entry("string yes", "yes", true),
			Entry("string TRUE uppercase", "TRUE", true),
			Entry("string no", "no", false),
			Entry("string FALSE uppercase", "FALSE", false),
			Entry("string with surrounding whitespace", "  yes  ", true),
			Entry("unparseable string", "banana", false),
			Entry("nil value", nil, false),
		)

		It("returns false for a missing key", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
			Expect(fm.Flag()).To(BeFalse())
		})
	})

	Describe("SetFlag", func() {
		It("stores true", func() {
			Expect(fm.SetFlag(ctx, true)).To(Succeed())
			Expect(fm.Flag()).To(BeTrue())
		})

		It("stores false", func() {
			Expect(fm.SetFlag(ctx, false)).To(Succeed())
			Expect(fm.Flag()).To(BeFalse())
		})

		It("stores a real YAML bool, not the string", func() {
			Expect(fm.SetFlag(ctx, true)).To(Succeed())
			Expect(fm.Get("flag")).To(Equal(true))
		})
	})

	Describe("ClearFlag", func() {
		It("removes the key entirely", func() {
			Expect(fm.SetFlag(ctx, true)).To(Succeed())
			fm.ClearFlag()
			Expect(fm.Flag()).To(BeFalse())
			Expect(fm.Get("flag")).To(BeNil())
		})

		It("is a no-op when the key is absent", func() {
			fm.ClearFlag()
			Expect(fm.Get("flag")).To(BeNil())
		})
	})
```

Use those `Entry` descriptions verbatim — `<verification>` greps `string TRUE uppercase` and `string with surrounding whitespace` by name. The `unparseable string` entry is load-bearing: it proves `flag: banana` reads as false without erroring (spec Failure Mode row 2).

### 5b. Dispatch tests in the existing `Describe("GetField", ...)`

Append these `It` blocks at the end of the existing `Describe("GetField", ...)` block, after the `It("returns raw value for unknown key"...)`:

```go
		It("returns empty for an absent flag", func() {
			Expect(fm.GetField("flag")).To(Equal(""))
		})

		It("returns true for flag true", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"flag": true})
			Expect(fm.GetField("flag")).To(Equal("true"))
		})

		It("returns false for flag false", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"flag": false})
			Expect(fm.GetField("flag")).To(Equal("false"))
		})
```

### 5c. Dispatch tests in the existing `Describe("SetField", ...)`

Append these `It` blocks at the end of the existing `Describe("SetField", ...)` block, after the `It("stores unknown field without error"...)`:

```go
		It("sets flag true from the string 'yes'", func() {
			Expect(fm.SetField(ctx, "flag", "yes")).To(Succeed())
			Expect(fm.Flag()).To(BeTrue())
		})

		It("sets flag false from the string 'FALSE'", func() {
			Expect(fm.SetField(ctx, "flag", "FALSE")).To(Succeed())
			Expect(fm.Flag()).To(BeFalse())
		})

		It("returns a validation error for an invalid flag value", func() {
			err := fm.SetField(ctx, "flag", "banana")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, validation.Error)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("banana"))
			Expect(err.Error()).To(ContainSubstring("true"))
			Expect(err.Error()).To(ContainSubstring("false"))
		})

		It("clears flag on empty string", func() {
			Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())
			Expect(fm.SetField(ctx, "flag", "")).To(Succeed())
			Expect(fm.Get("flag")).To(BeNil())
		})
```

### 5d. YAML round-trip block (new top-level `Describe` at end of file)

Append a new top-level block at the end of `pkg/domain/task_frontmatter_test.go`, after the existing `var _ = Describe("TaskFrontmatter SetField alias normalization", ...)` block:

```go
var _ = Describe("TaskFrontmatter flag YAML round-trip", func() {
	var ctx context.Context
	var fm domain.TaskFrontmatter

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewTaskFrontmatter(nil)
	})

	It("writes flag true alongside pre-existing keys without touching them", func() {
		fm = domain.NewTaskFrontmatter(map[string]any{
			"status":       "in_progress",
			"priority":     3,
			"planned_date": "2025-03-15",
			"themes":       []any{"t1"},
			"custom_key":   "custom-value",
		})

		Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())

		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		yamlText := string(data)

		Expect(yamlText).To(ContainSubstring("flag: true"))
		for _, preexisting := range []string{"status:", "priority:", "planned_date:", "themes:", "custom_key:"} {
			Expect(yamlText).To(ContainSubstring(preexisting))
		}
	})

	It("never emits a flag key for a task that was never flagged", func() {
		fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("flag:"))
	})

	It("emits no flag key after clear", func() {
		Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())
		fm.ClearFlag()
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("flag:"))
	})

	It("round-trips flag true through unmarshal", func() {
		Expect(fm.SetField(ctx, "flag", "true")).To(Succeed())
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())

		var raw map[string]any
		Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
		re := domain.NewTaskFrontmatter(raw)
		Expect(re.Flag()).To(BeTrue())
	})

	It("round-trips explicit false as flag: false", func() {
		Expect(fm.SetFlag(ctx, false)).To(Succeed())
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("flag: false"))

		var raw map[string]any
		Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
		Expect(domain.NewTaskFrontmatter(raw).Flag()).To(BeFalse())
	})

	It("preserves an unparseable flag value on write until explicitly changed", func() {
		fm = domain.NewTaskFrontmatter(map[string]any{"flag": "banana", "status": "todo"})
		Expect(fm.Flag()).To(BeFalse())

		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("flag: banana"))
	})
})
```

These tests traverse the real serialization boundary production traffic uses (`yaml.Marshal` of `RawMap()` in `serializeMapAsFrontmatter`, read back via `parseToFrontmatterMap`) and cover AC5 (pre-existing keys byte-preserved), Desired Behavior #5 (omitempty round-trip), and the Failure Mode row for `flag: banana`.

## 6. Ops-level tests in `pkg/ops/frontmatter_test.go`

The ops layer itself is unchanged (generic), but add tests so the validation and no-write guarantees are exercised through the real `FrontmatterSetOperation.Execute` dispatch path.

### 6a. In `Describe("FrontmatterSetOperation", ...)`

Append these `Context` blocks after the existing `Context("setting invalid phase field", ...)` block:

```go
	Context("setting flag field", func() {
		BeforeEach(func() {
			key = "flag"
			value = "true"
		})

		It("updates the flag field", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Flag()).To(BeTrue())
		})
	})

	Context("setting flag field to false", func() {
		BeforeEach(func() {
			key = "flag"
			value = "no"
		})

		It("writes flag false", func() {
			Expect(err).To(BeNil())
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
			Expect(writtenTask.Flag()).To(BeFalse())
		})
	})

	Context("setting invalid flag field", func() {
		BeforeEach(func() {
			key = "flag"
			value = "banana"
		})

		It("returns an error naming the value and the accepted set", func() {
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("banana"))
			Expect(err.Error()).To(ContainSubstring("true"))
			Expect(err.Error()).To(ContainSubstring("false"))
		})

		It("does not write the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})
	})
```

The last `It` is the AC2 / Failure Mode 1 guarantee: an invalid value exits the dispatch before `WriteTask` is ever called.

### 6b. In `Describe("FrontmatterGetOperation", ...)`

Append these `Context` blocks after the existing `Context("unknown key", ...)` block:

```go
	Context("getting flag field when absent", func() {
		BeforeEach(func() {
			key = "flag"
		})

		It("returns empty string with no error", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("getting flag field when set", func() {
		BeforeEach(func() {
			key = "flag"
			_ = task.SetFlag(context.Background(), true)
		})

		It("returns true", func() {
			Expect(err).To(BeNil())
			Expect(result).To(Equal("true"))
		})
	})
```

Note the get-operation test fixture task (built in `BeforeEach` at the top of `Describe("FrontmatterGetOperation", ...)`) does not carry a `flag` key, so the first `Context` exercises the absent case of AC3.

## 7. Changelog

In `/workspace/CHANGELOG.md`, inside the existing `## Unreleased` section, add this bullet **below** the existing `- fix:` bullet and above the first `## vX.Y.Z` heading:

```
- feat: task frontmatter gains a boolean `flag` field — `vault-cli task set "T" flag true|yes|false|no` writes it (case-insensitive; invalid values error and write nothing), `vault-cli task clear "T" flag` removes it, and `vault-cli task get "T" flag` reads `true`/`false`/empty. The flag round-trips with the same omitempty semantics as the rest of the map and is orthogonal to status, phase, priority and the date fields.
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json` — the release agent owns those. Do NOT create a git tag. `## Unreleased` already exists; do not create a second one.

</requirements>

<constraints>
- New typed accessors are exactly: `Flag() bool` (GetBool-backed), `SetFlag(ctx, bool) error` (mirrors the `SetPriority` pattern), `ClearFlag()`.
- `SetField` dispatch gains a `flag` case routed to the validated `setFlagField` helper, so the generic `task set` path validates instead of passing through as a free-form key.
- `task get` and `task clear` work on `flag` through the existing generic infrastructure — no ops or CLI code changes.
- Missing `flag` must not cause an empty/false key to be written — keep the omitempty round-trip invariant (a never-flagged task gains no `flag:` line on any write).
- Existing tests must pass; the change must not break serialization of any existing field.
- Per `docs/development-patterns.md`: operations inject the storage interface, no direct file I/O in ops, `libtime` for dates, counterfeiter mocks, no `encoding/json` in command files.
- One boolean flag only — no colours/categories, no `flagged_on` date, no config key, no opt-out flag, no environment variable. The spec Non-goals forbid a dated shape and multiple categories.
- Flagging goals/themes/objectives is out of scope; the Vault UI rendering/sorting half is a separate repo and out of scope.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
- Existing tests must still pass, unmodified.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Full gate first pass:**

```
make precommit
```

Must exit 0. `make precommit` runs `format generate test check addlicense` (and `make test` is part of it, see the Makefile `precommit` target). If a step fails, fix and re-run the failing target, then run `make precommit` one final time.

**2. The accessors and dispatch landed** — each command must print at least one line:

```
grep -n 'func (f \*TaskFrontmatter) SetFlag' pkg/domain/task_frontmatter.go
grep -n 'ClearFlag' pkg/domain/task_frontmatter.go
grep -n 'func (f \*TaskFrontmatter) setFlagField' pkg/domain/task_frontmatter.go
grep -n 'setFlagField' pkg/domain/task_frontmatter.go
grep -n 'case "flag":' pkg/domain/task_frontmatter.go
```

`setFlagField` must appear at least twice (definition + `SetField` dispatch case); `case "flag":` must appear exactly twice (`GetField` + `SetField`).

**3. Named table entries are present** — each command must print a number `>= 1` (the `-ginkgo.v` flag is required, otherwise entry descriptions print only on failure):

```
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "string TRUE uppercase"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "string with surrounding whitespace"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "unparseable string"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "preserves an unparseable flag value on write until explicitly changed"
```

**4. Ops-level coverage of the validation path:**

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "does not write the task"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "setting invalid flag field"
```

Both must print `>= 1`.

**5. Coverage — the new methods at 100%:**

```
go test -coverprofile=/tmp/cover-flag.out ./pkg/domain/... && go tool cover -func=/tmp/cover-flag.out | grep -E 'Flag|setFlagField'
```

`Flag`, `SetFlag`, `ClearFlag` and `setFlagField` must each report `100.0%`. If any reports below 100.0%, add the missing `Entry` / `It` to the domain tests from requirement 5 (do not add unrelated retroactive coverage).

**6. Changelog:**

```
grep -n 'flag' CHANGELOG.md
grep -n '^## ' CHANGELOG.md | head -1
```

The first must print the `- feat:` flag bullet. The second must print `## Unreleased` — if it prints a version heading instead, the bullet was placed between released sections; move it into the existing `## Unreleased` section.
</verification>
