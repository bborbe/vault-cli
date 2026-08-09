---
status: completed
spec: [030-bug-decision-ack-destroys-frontmatter]
summary: Added GetBool coercing accessor to FrontmatterMap with 100% coverage and all entry points documented
execution_id: vault-cli-decision-frontmatter-exec-178-spec-030-frontmatter-getbool
dark-factory-version: v0.192.9
created: "2026-08-09T11:15:00Z"
queued: "2026-08-09T11:51:18Z"
started: "2026-08-09T11:51:59Z"
completed: "2026-08-09T11:53:49Z"
---

# Add a coercing GetBool accessor to FrontmatterMap

<summary>
- `FrontmatterMap` gains a bool accessor, joining the string, time, and string-slice accessors it already has.
- A frontmatter value written as a real YAML boolean reads back as that boolean.
- A frontmatter value quoted by hand — `"true"`, `"yes"`, `"TRUE"` — also reads back as `true` instead of silently reading as `false`.
- `"false"` and `"no"` read back as `false`, in any letter case.
- A missing key, or a value that is neither a bool nor a recognised word, reads back as `false`.
- This closes the silent-wrong-value hole where a hand-quoted `reviewed: "true"` would have been read as "not reviewed" and resurfaced an already-acknowledged decision.
- Nothing calls the new accessor yet — the decision read path is wired to it in the next prompt.
- The developer-patterns doc gains the accessor list so it stops drifting from the code.
</summary>

<objective>
Add `GetBool` to `domain.FrontmatterMap` so bool-valued frontmatter keys are read by coercion (like the existing `GetString` / `GetTime` / `GetStringSlice` accessors) rather than by a bare `.(bool)` type assertion at the call site, which silently yields `false` for a hand-quoted `"true"`.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

Read these files fully before making changes:

- `/workspace/pkg/domain/frontmatter_map.go` (146 lines) — the file you change. It currently exposes exactly `Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`, `Keys`, `RawMap`. There is **no** bool accessor. Verified current shape:

  ```go
  type FrontmatterMap struct {
      data map[string]any
  }

  func NewFrontmatterMap(data map[string]any) FrontmatterMap {
      if data == nil {
          data = make(map[string]any)
      }
      return FrontmatterMap{data: data}
  }

  // GetString returns the string representation of the value stored for key.
  // Returns "" if the key is absent or the value cannot be stringified.
  func (f FrontmatterMap) GetString(key string) string {
      v := f.data[key]
      if v == nil {
          return ""
      }
      switch s := v.(type) {
      case string:
          return s
      default:
          return fmt.Sprintf("%v", v)
      }
  }
  ```

  Note the shape every accessor follows: value receiver, `v := f.data[key]`, nil guard, type switch, safe zero-value default. `GetBool` follows it exactly. `strings` is already imported (used by `GetStringSlice`); `fmt`, `context`, `time`, and `libtime` are also already imported — do not add or remove imports.

- `/workspace/pkg/domain/frontmatter_map_test.go` — the existing Ginkgo suite for this type, `package domain_test`, dot-importing `ginkgo/v2` and `gomega`. It uses `Describe` / `It` blocks named after the accessor under test.

- `/workspace/pkg/domain/goal_frontmatter_test.go` around line 329 — the in-repo `DescribeTable` shape to copy:

  ```go
  DescribeTable("accepts canonical values",
      func(value string) {
          fm := domain.NewGoalFrontmatter(nil)
          Expect(fm.SetField(ctx, "phase", value)).To(Succeed())
          Expect(fm.GetField("phase")).To(Equal(value))
      },
      Entry("todo", "todo"),
      ...
  )
  ```

- `/workspace/docs/development-patterns.md` — read the `## Entity Structure` section, specifically the `**Frontmatter** (pkg/domain/<entity>_frontmatter.go)` bullet list that begins with `- Embeds `FrontmatterMap` (a `map[string]any` wrapper)`. That is the list you extend.

- `/workspace/CHANGELOG.md` — the newest versioned section is `## v0.103.0`. There is currently **no** `## Unreleased` section; you create it. Do not trust this version string blindly: `## Unreleased` goes above whichever `## vX.Y.Z` heading appears **first** in the file, whatever its number.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>

## 1. Add `GetBool` to `FrontmatterMap`

In `/workspace/pkg/domain/frontmatter_map.go`, insert this method immediately **after** `GetString` and **before** `GetTime`, so the accessor family stays grouped:

```go
// GetBool returns the bool value stored for key.
//
// A bool value passes through unchanged. A string value is matched
// case-insensitively after trimming surrounding whitespace: "true" and "yes"
// yield true, everything else yields false. A missing key, a nil value, or any
// other type also yields false.
//
// Coercion rather than a bare type assertion is deliberate: YAML 1.2 leaves a
// hand-quoted reviewed: "true" as a string, and a .(bool) assertion would read
// that as false — "not reviewed" — silently resurfacing an already-acknowledged
// page.
func (f FrontmatterMap) GetBool(key string) bool {
	v := f.data[key]
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "yes":
			return true
		default:
			return false
		}
	default:
		return false
	}
}
```

Write it exactly as shown. Do not add a second signature (no `GetBoolDefault`, no `GetBoolOK`, no pointer-returning variant), do not add a package-level lookup table, and do not change `Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`, `Keys`, or `RawMap` in any way.

## 2. Table test for the coercion rules

In `/workspace/pkg/domain/frontmatter_map_test.go`, add one new top-level `Describe("GetBool", ...)` block beside the existing accessor blocks. Do not modify, reorder, or remove any existing block.

The block contains one `DescribeTable` plus two standalone `It`s:

```go
Describe("GetBool", func() {
	DescribeTable("coerces the stored value",
		func(stored any, expected bool) {
			fm := domain.NewFrontmatterMap(map[string]any{"reviewed": stored})
			Expect(fm.GetBool("reviewed")).To(Equal(expected))
		},
		Entry("bool true", true, true),
		Entry("bool false", false, false),
		Entry("string true", "true", true),
		Entry("string yes", "yes", true),
		Entry("string TRUE uppercase", "TRUE", true),
		Entry("string Yes mixed case", "Yes", true),
		Entry("string false", "false", false),
		Entry("string no", "no", false),
		Entry("string FALSE uppercase", "FALSE", false),
		Entry("string with surrounding whitespace", "  true  ", true),
		Entry("empty string", "", false),
		Entry("unparseable string", "maybe", false),
		Entry("int is not coerced", 1, false),
		Entry("nil value", nil, false),
	)

	It("returns false for a missing key", func() {
		fm := domain.NewFrontmatterMap(map[string]any{"status": "proposed"})
		Expect(fm.GetBool("reviewed")).To(BeFalse())
	})

	It("returns false on a nil-constructed map", func() {
		fm := domain.NewFrontmatterMap(nil)
		Expect(fm.GetBool("reviewed")).To(BeFalse())
	})
})
```

Use those `Entry` descriptions verbatim — `<verification>` greps several of them by name. The `int is not coerced` entry is load-bearing: it proves the accessor does not fall through to a truthiness rule the way `GetString` falls through to `fmt.Sprintf`.

## 3. Document the accessor family

In `/workspace/docs/development-patterns.md`, inside the `**Frontmatter** (pkg/domain/<entity>_frontmatter.go)` bullet list in the `## Entity Structure` section, add one bullet directly after the existing `- Embeds `FrontmatterMap` (a `map[string]any` wrapper)` bullet:

```
- `FrontmatterMap` provides the raw accessor family: `Get`, `GetString`, `GetBool`, `GetTime`,
  `GetStringSlice`, `Set`, `Delete`, `Keys`, `RawMap`. Getters **coerce** rather than type-assert —
  `GetBool` accepts a YAML bool and the strings `true` / `yes` / `false` / `no` (case-insensitive),
  returning `false` for a missing key or an unrecognised value.
```

Do not remove or reword any other bullet in that list.

## 4. Changelog

Create a `## Unreleased` section in `/workspace/CHANGELOG.md`, placed **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` bullets) and **above the first `## vX.Y.Z` heading in the file** (currently `## v0.103.0`). Never between the `# Changelog` title and the preamble, and never between two released sections.

Determine the insertion point from the file itself rather than from a hardcoded version — `grep -n '^## ' CHANGELOG.md | head -3` shows the first release heading; `## Unreleased` goes immediately above it. `scripts/check-changelog.sh` only verifies the preamble precedes the first `## ` section, so a misplacement between two released sections would NOT be caught by `make precommit`.

```
## Unreleased

- feat: Add `FrontmatterMap.GetBool` — a coercing bool accessor that reads a YAML bool as-is and the case-insensitive strings `true` / `yes` / `false` / `no`, returning `false` for a missing key or an unrecognised value
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json` — the release agent owns those.

## 5. Coverage

`pkg/domain` is the changed package. `GetBool` must reach 100% statement coverage from the table in requirement 2 — every branch (nil, bool, string-true, string-false, default) has a dedicated `Entry`. Do not add retroactive coverage to unrelated untested `pkg/domain` code.

</requirements>

<constraints>
- `GetBool` is the ONLY behavior added by this prompt. Do NOT wire it into any call site. `pkg/storage/decision.go` still reads `data["needs_review"].(bool)` after this prompt lands; replacing that is the next prompt's job, and doing it here would split one behavior change across two prompts.
- Do NOT touch `/workspace/pkg/domain/decision.go`, `/workspace/pkg/storage/decision.go`, `/workspace/pkg/ops/decision_ack.go`, or `/workspace/pkg/ops/decision_list.go` in this prompt.
- Do NOT modify `Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`, `Keys`, or `RawMap` — they already behave correctly and five entity types depend on their exact semantics.
- Do NOT add, remove, or reorder imports in `frontmatter_map.go`. `strings` is already imported; `GetBool` needs nothing else.
- Do NOT add a config key, flag, environment variable, or opt-out to select assertion-vs-coercion. The coercion rule is invariant.
- A missing key and an unrecognised value are deliberately indistinguishable — both are `false`. Do NOT add an error return, a second return value, or a sentinel.
- No other domain type is touched. This prompt adds a method; it changes no struct.
- Tests use Ginkgo v2 / Gomega in the external `domain_test` package. `pkg/domain/domain_suite_test.go` already exists, so new specs run.
- Errors are wrapped with `github.com/bborbe/errors`; never `fmt.Errorf`, never `context.Background()` in `pkg/`. (`GetBool` returns a plain `bool` and produces no errors.)
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
- Existing tests must still pass, unmodified.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**

```
make test
```

Must exit 0.

**2. The accessor exists with the expected signature** — must print one line:

```
grep -n 'func (f FrontmatterMap) GetBool(key string) bool' pkg/domain/frontmatter_map.go
```

**3. Named table entries are present** — each command must print a number `>= 1`:

```
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "string TRUE uppercase"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "string with surrounding whitespace"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "unparseable string"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "int is not coerced"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "returns false for a missing key"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "returns false on a nil-constructed map"
```

The `-ginkgo.v` flag is required — plain `go test -v` does not enable Ginkgo's verbose reporter, so entry descriptions print only when a spec fails.

**4. No call site was wired up in this prompt** — this must still print the pre-existing assertion line, proving the decision read path was left alone:

```
grep -n 'data\["needs_review"\]' pkg/storage/decision.go
```

**5. Coverage — `GetBool` at 100%:**

```
go test -coverprofile=/tmp/cover.out ./pkg/domain/... && go tool cover -func=/tmp/cover.out | grep GetBool
```

Must report `100.0%`.

**6. Docs and changelog:**

```
grep -n 'GetBool' docs/development-patterns.md
grep -n -A6 '^## Unreleased' CHANGELOG.md
```

The first must print at least one line. The second must show a `- feat:` bullet naming `FrontmatterMap.GetBool`.

Then confirm `## Unreleased` is the **first** `## ` heading in the file — not merely present somewhere:

```
grep -n '^## ' CHANGELOG.md | head -1
```

Must print `## Unreleased`. If it prints a version heading instead, `## Unreleased` was inserted between two released sections — move it above the first release heading.

**7. Full gate, once, at the end:**

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make format`, etc.) until it passes, then run `make precommit` one final time.
</verification>
