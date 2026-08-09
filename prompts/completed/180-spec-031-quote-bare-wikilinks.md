---
status: completed
spec: [031-bug-unquoted-wikilink-mangled-on-frontmatter-write]
summary: Add raw-text wikilink quoting pass to parseToFrontmatterMap that wraps bare [[X]] values in single quotes before yaml.Unmarshal, preventing the nested-list destruction on write
execution_id: vault-cli-exec-180-spec-031-quote-bare-wikilinks
dark-factory-version: v0.192.9
created: "2026-08-09T15:45:00Z"
queued: "2026-08-09T14:33:20Z"
started: "2026-08-09T14:34:10Z"
completed: "2026-08-09T14:37:33Z"
---

# Quote bare wikilinks before YAML parses the frontmatter

<summary>
- A frontmatter wikilink written without quotes — `related_task: [[Some Task]]` — survives a write instead of being silently destroyed.
- Today that line is valid YAML for a list-inside-a-list, so a write re-emits it as `- - Some Task` and the link and its backlink vanish with no error.
- After this change the same line comes back as `related_task: '[[Some Task]]'`, which Obsidian renders as a working link.
- The same fix applies to a bare wikilink written as a list entry (`- [[A Theme]]`).
- A wikilink that is already quoted is left exactly as it was.
- A value that merely mentions a wikilink inside a longer sentence keeps its original meaning and is not rewritten.
- A wikilink whose title contains an apostrophe still reads back with the apostrophe intact.
- Aliases (`[[X|alias]]`) and heading anchors (`[[X#Section]]`) are preserved character for character.
- Running the same write twice produces no further change — the fix is stable.
- A wikilink carrying a trailing YAML comment is knowingly left alone, and that limitation is recorded.
- All six entity types inherit the fix from one shared parse point, with no per-entity code.
- The developer-patterns doc gains the invariant so the next author does not reintroduce the bug.
</summary>

<objective>
Add a raw-text quoting pass to `parseToFrontmatterMap` in `/workspace/pkg/storage/base.go` that wraps a value which is exactly a bare Obsidian wikilink in single quotes before `yaml.Unmarshal` ever sees it, so the in-memory map holds a plain `string` and the existing serializer writes it back as a working, quoted wikilink instead of a nested block sequence.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

Read these files fully before making changes:

- `/workspace/pkg/storage/base.go` (273 lines) — the only non-test source file this prompt changes. Verified current shape of the two functions at the centre of this bug:

  ```go
  var (
      frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
      // ... CheckboxRegex, CheckboxCompleteRegex, CheckboxUncompleteRegex ...
  )

  // parseToFrontmatterMap parses the YAML frontmatter block from content into a
  // map[string]any, preserving all fields including unknown ones.
  // Returns an error if no frontmatter block is found or YAML is invalid.
  func (b *baseStorage) parseToFrontmatterMap(
      ctx context.Context,
      content []byte,
  ) (map[string]any, error) {
      matches := frontmatterRegex.FindSubmatch(content)
      if len(matches) < 2 {
          return nil, errors.Errorf(ctx, "no frontmatter found")
      }

      var m map[string]any
      if err := yaml.Unmarshal(matches[1], &m); err != nil {
          return nil, errors.Wrap(ctx, err, "unmarshal yaml frontmatter")
      }
      if m == nil {
          m = make(map[string]any)
      }
      return m, nil
  }
  ```

  `matches[1]` is the frontmatter block **between** the `---` fences; `matches[2]` is the markdown body. The quoting pass therefore never sees the body, which is why a `Tags: [[Task]]` line below the fence stays byte-identical for free.

  The file already imports `bytes`, `context`, `fmt`, `os`, `path/filepath`, `regexp`, `strings`, `time`, `github.com/bborbe/errors`, `gopkg.in/yaml.v3`, and `github.com/bborbe/vault-cli/pkg/domain`. **The new code needs no new import.**

- `/workspace/pkg/storage/export_test.go` — the test-only bridge you use from `storage_test`. Verified:

  ```go
  type BaseStorageForTest = baseStorage
  func NewBaseStorageForTest() *BaseStorageForTest
  func ParseToFrontmatterMapForTest(ctx context.Context, b *BaseStorageForTest, content []byte) (map[string]any, error)
  func SerializeMapAsFrontmatterForTest(ctx context.Context, b *BaseStorageForTest, data map[string]any, orig string) (string, error)
  func FindFileByNameForTest(ctx context.Context, b *BaseStorageForTest, dir string, name string) (string, string, error)
  ```

  Do **not** add a new exported test helper — `ParseToFrontmatterMapForTest` plus `SerializeMapAsFrontmatterForTest` are sufficient, and the quoting pass is deliberately unexported and untested directly.

- `/workspace/pkg/storage/base_test.go` (101 lines) — read fully. `package storage_test`, dot-imports ginkgo/gomega, `BeforeEach` sets `ctx = context.Background()` and `b = storage.NewBaseStorageForTest()`. It already has `Describe("baseStorage map methods")` containing `Describe("parseToFrontmatterMap")`, `Describe("serializeMapAsFrontmatter")` and a `Context("round-trip")`. You add a **new top-level** `Describe` beside it; do not modify any existing block.

- `/workspace/pkg/ops/lint.go` around `detectDuplicateKeys` (line 336) — the in-repo precedent for a raw-frontmatter-text pass that runs instead of routing through `yaml.Unmarshal`:

  ```go
  func (l *lintOperation) detectDuplicateKeys(frontmatterYAML string) []string {
      lines := strings.Split(frontmatterYAML, "\n")
      ...
  }
  ```

  Same reason applies here: unmarshal is the operation that mangles the input, so the fix has to run before it.

- `/workspace/docs/development-patterns.md` — read the `## Entity Structure` section. The `**Storage** (pkg/storage/)` bullet group currently reads:

  ```
  **Storage** (`pkg/storage/`)
  - `parseToFrontmatterMap` parses the YAML frontmatter block into `map[string]any`
  - `serializeMapAsFrontmatter` marshals the map back to YAML; unknown fields are preserved
  - Entity-specific read helpers call `NewXxx(data, meta, content)` constructors
  - **Rendering caveat**: a bare YAML date (`review_date: 2026-08-15`) parses to `time.Time` and re-serializes as RFC3339 ...
  ```

  That is the list you extend.

- `/workspace/CHANGELOG.md` — there is currently **no** `## Unreleased` section. The first `## ` heading after the preamble is `## v0.104.1`. You create `## Unreleased` above it.

- `/workspace/.golangci.yml` — `run.tests: true`; `funlen` is 80 lines / 50 statements, `gocognit` min-complexity 20, `nestif` min-complexity 4, `forcetypeassert` and `prealloc` are on. `dupl`, `unparam`, and `gosec` are excluded for `_test.go` files. The implementation below is written to sit well inside every one of those budgets — keep it that way; do not merge the three helpers into one function.

**Measured YAML round-trip facts** — every row below was executed against `gopkg.in/yaml.v3` at the version in `/workspace/go.mod`. Use these as ground truth; do not re-derive them from intuition.

| Source frontmatter line(s) | `yaml.Unmarshal` yields today | `yaml.Marshal` re-emits today |
|---|---|---|
| `related_task: [[Some Other Task]]` | `[]any{[]any{"Some Other Task"}}` | `related_task:\n    - - Some Other Task` |
| `themes:` + `    - [[A Theme]]` | `[]any{[]any{[]any{"A Theme"}}}` | `themes:\n    - - - A Theme` |
| `related_task: '[[Some Task]]'` | `string("[[Some Task]]")` | `related_task: '[[Some Task]]'` |
| `related_task: "[[Some Task]]"` | `string("[[Some Task]]")` | `related_task: '[[Some Task]]'` (single quotes — serializer normalization, pre-existing) |
| `title: see [[X]] for details` | `string("see [[X]] for details")` | `title: see [[X]] for details` (unquoted plain scalar) |
| `related: [[A]] and [[B]]` | **error** `yaml: did not find expected key` | n/a |
| `related_task: [[X]]  # note` | `[]any{[]any{"X"}}` | `related_task:\n    - - X` |
| `related_task: [[Foo [bar]]]` | **error** `yaml: did not find expected ',' or ']'` | n/a |
| `description: \|` + `    related_task: [[X]]` | `string("related_task: [[X]]\n")` | byte-identical block scalar |
| `related_task:` + `    - - Some Other Task` | `[]any{[]any{"Some Other Task"}}` | byte-identical nested list |

And, measured on the Go side, `yaml.Marshal` of a `map[string]any` value:

| Go value | Emitted YAML |
|---|---|
| `"[[Some Other Task]]"` | `k: '[[Some Other Task]]'` |
| `"[[Ben's Task]]"` | `k: '[[Ben''s Task]]'` |
| `"[[X\|alias]]"` | `k: '[[X\|alias]]'` |
| `"[[X#Section]]"` | `k: '[[X#Section]]'` |
| `"[[Foo [bar]]]"` | `k: '[[Foo [bar]]]'` |
| `"see [[X]] for details"` | `k: see [[X]] for details` |
| `[]any{"[[A Theme]]"}` | `k:\n    - '[[A Theme]]'` |

Two consequences worth internalising before writing assertions: `yaml.Marshal` already produces exactly the quoting this spec wants once the value is a `string`, so **`serializeMapAsFrontmatter` needs no change at all**; and a double-quoted source line comes back single-quoted, so "already-quoted is byte-identical" holds for the single-quoted form only — the double-quoted form is normalized by the pre-existing serializer, not by this pass.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` API.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>

## 1. Add the three regexes

In `/workspace/pkg/storage/base.go`, extend the existing package-level `var (...)` block. Append these three declarations **after** `CheckboxUncompleteRegex`, inside the same block. Do not reorder or reword the four regexes already there.

```go
	// bareWikilinkMappingRegex matches a frontmatter mapping line whose value is
	// exactly a bare Obsidian wikilink, at any indentation: `related_task: [[X]]`.
	// Capture groups: 1=indentation plus the `key: ` prefix, 2=the wikilink value.
	// A trailing YAML comment defeats the match by design — see quoteBareWikilinks.
	bareWikilinkMappingRegex = regexp.MustCompile(
		`^( *[A-Za-z0-9_][A-Za-z0-9_.-]*:[ \t]+)(\[\[.+\]\])[ \t]*$`,
	)

	// bareWikilinkSequenceRegex matches a block-sequence entry whose value is
	// exactly a bare Obsidian wikilink, at any indentation: `    - [[A Theme]]`.
	// Capture groups: 1=indentation plus the `- ` marker, 2=the wikilink value.
	bareWikilinkSequenceRegex = regexp.MustCompile(
		`^( *-[ \t]+)(\[\[.+\]\])[ \t]*$`,
	)

	// blockScalarStartRegex matches a mapping or sequence line that opens a block
	// scalar (`|` or `>`, with optional chomping and indentation indicators).
	// Capture group 1 is the introducing line's indentation, used to find where the
	// scalar's body ends. Lines inside a block scalar are literal text, never YAML,
	// so the quoting pass must leave them alone.
	blockScalarStartRegex = regexp.MustCompile(
		`^( *)(?:[A-Za-z0-9_][A-Za-z0-9_.-]*:|-)[ \t]+[|>][+-]?[0-9]*[ \t]*$`,
	)
```

Use ` *` (space-only) for indentation, **not** `\s*`: YAML forbids tabs as indentation, and `\s` in Go RE2 also matches `\n`, which would be wrong if the expression were ever applied to more than one line.

## 2. Add the quoting pass

In `/workspace/pkg/storage/base.go`, add these three functions immediately **after** `parseToFrontmatterMap` and **before** `serializeMapAsFrontmatter`. Write them exactly as shown.

```go
// quoteBareWikilinks rewrites frontmatter lines whose value is exactly a bare
// Obsidian wikilink into a single-quoted YAML scalar, and returns the rewritten
// frontmatter block.
//
// A bare wikilink is well-formed YAML flow-sequence syntax: `k: [[X]]` unmarshals
// to []any{[]any{"X"}} and marshals back as the nested block sequence `k:\n - - X`,
// silently destroying the link and its backlink. Neither yaml.Unmarshal nor
// yaml.Marshal is wrong on its own terms — the corruption emerges only from the
// round-trip, which is why nothing detects it. Quoting the value before
// yaml.Unmarshal ever sees it keeps the in-memory value a plain string, so the
// existing serializer writes it back as a working wikilink with no change to
// serializeMapAsFrontmatter.
//
// Only quoting changes; YAML shape never does. A bare wikilink under a
// conventionally-list key (`themes: [[A Theme]]`) becomes the quoted *scalar*
// `themes: '[[A Theme]]'`, not a one-element sequence — the authored form is
// preserved as authored.
//
// Left byte-identical: an already-quoted value, a value that merely contains a
// wikilink among other text, a value carrying a trailing YAML comment, a value
// spanning multiple lines, the body of a block scalar, and an already-destroyed
// nested list. The pass is a pure text transform and cannot introduce a parse
// error it did not receive; it is also idempotent, because a quoted value no
// longer starts with `[[`.
func quoteBareWikilinks(frontmatter []byte) []byte {
	if !bytes.Contains(frontmatter, []byte("[[")) {
		return frontmatter
	}
	lines := strings.Split(string(frontmatter), "\n")
	changed := false
	blockScalarIndent := -1
	for i, line := range lines {
		if blockScalarIndent >= 0 && isBlockScalarBody(line, blockScalarIndent) {
			continue
		}
		blockScalarIndent = -1
		if m := blockScalarStartRegex.FindStringSubmatch(line); len(m) == 2 {
			blockScalarIndent = len(m[1])
			continue
		}
		rewritten, ok := quoteBareWikilinkLine(line)
		if !ok {
			continue
		}
		lines[i] = rewritten
		changed = true
	}
	if !changed {
		return frontmatter
	}
	return []byte(strings.Join(lines, "\n"))
}

// isBlockScalarBody reports whether line belongs to the body of a block scalar
// whose introducing key is indented by keyIndent spaces. A blank line and any
// line indented deeper than the key are part of the body; the first line at or
// left of the key's indentation ends it.
func isBlockScalarBody(line string, keyIndent int) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	return len(line)-len(strings.TrimLeft(line, " ")) > keyIndent
}

// quoteBareWikilinkLine returns the single-quoted rewrite of line and true when
// line's value is exactly one bare wikilink; otherwise it returns line unchanged
// and false.
//
// Single quotes inside the title are escaped by doubling, so `[[Ben's Task]]`
// emits '[[Ben''s Task]]' and re-parses to the original title. A line holding
// more than one wikilink (`k: [[A]] and [[B]]`) is rejected: its value is not
// exactly a wikilink, and it is not valid YAML today either, so the pass leaves
// the pre-existing parse error intact rather than silently making it parse.
func quoteBareWikilinkLine(line string) (string, bool) {
	matches := bareWikilinkMappingRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		matches = bareWikilinkSequenceRegex.FindStringSubmatch(line)
	}
	if len(matches) != 3 {
		return line, false
	}
	value := matches[2]
	if strings.Contains(value[2:len(value)-2], "]]") {
		return line, false
	}
	return matches[1] + "'" + strings.ReplaceAll(value, "'", "''") + "'", true
}
```

The `value[2:len(value)-2]` slice is always safe: `\[\[.+\]\]` requires at least five characters, so the inner span is never out of range.

## 3. Call the pass from `parseToFrontmatterMap`

In `/workspace/pkg/storage/base.go`, change exactly one line of `parseToFrontmatterMap`:

```go
	if err := yaml.Unmarshal(matches[1], &m); err != nil {
```

becomes

```go
	if err := yaml.Unmarshal(quoteBareWikilinks(matches[1]), &m); err != nil {
```

Nothing else in the function changes: the `frontmatterRegex.FindSubmatch` call, the `len(matches) < 2` guard and its `no frontmatter found` message, the `unmarshal yaml frontmatter` wrap message, the nil-map guard, and the signature all stay byte-for-byte as they are.

Extend the function's doc comment with one sentence, appended after the existing two lines:

```go
// parseToFrontmatterMap parses the YAML frontmatter block from content into a
// map[string]any, preserving all fields including unknown ones.
// Returns an error if no frontmatter block is found or YAML is invalid.
// A bare Obsidian wikilink value is quoted before unmarshal — see
// quoteBareWikilinks — so it is read as a string rather than a nested list.
```

## 4. Unit tests for the quoting pass

In `/workspace/pkg/storage/base_test.go`, add one **new top-level** `Describe` beside the existing `Describe("baseStorage map methods", ...)`. Do not modify, reorder, or remove any existing block. Add no imports beyond `strings` (needed by requirement 4s); `context`, ginkgo, gomega, and `storage` are already imported.

The `Describe` text must contain the word `Wikilink` — `<verification>` focuses the suite on it:

```go
var _ = Describe("bare Wikilink quoting on the parse path", func() {
	var (
		ctx context.Context
		b   *storage.BaseStorageForTest
	)

	BeforeEach(func() {
		ctx = context.Background()
		b = storage.NewBaseStorageForTest()
	})

	// wrap builds a full markdown document from a frontmatter block. The body
	// deliberately contains a bare wikilink so every spec also proves the pass
	// never reaches past the closing fence.
	wrap := func(frontmatter string) string {
		return "---\n" + frontmatter + "\n---\nTags: [[Task]]\n\n---\nbody\n"
	}

	parse := func(frontmatter string) (map[string]any, error) {
		return storage.ParseToFrontmatterMapForTest(ctx, b, []byte(wrap(frontmatter)))
	}

	// roundTrip parses a frontmatter block and re-serializes it, returning the
	// full document as it would be written back to disk.
	roundTrip := func(frontmatter string) string {
		original := wrap(frontmatter)
		parsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(original))
		Expect(err).To(BeNil())
		serialized, err := storage.SerializeMapAsFrontmatterForTest(ctx, b, parsed, original)
		Expect(err).To(BeNil())
		return serialized
	}

	// ... It blocks 4a-4u below ...
})
```

Add these `It` blocks with **exactly** these names — `<verification>` greps several by name. Every "no nested list" assertion uses the multiline regexp `(?m)^ *- - ` so it matches the spec's own `grep -c '^ *- - '` evidence.

**4a. `reads a bare wikilink scalar as a string`**
```go
m, err := parse("related_task: [[Some Other Task]]")
Expect(err).To(BeNil())
Expect(m).To(HaveKeyWithValue("related_task", "[[Some Other Task]]"))
```

**4b. `writes a bare wikilink scalar back as a quoted scalar`**
```go
out := roundTrip("priority: 2\nrelated_task: [[Some Other Task]]\nstatus: in_progress")
Expect(out).To(ContainSubstring("\nrelated_task: '[[Some Other Task]]'\n"))
Expect(out).NotTo(MatchRegexp(`(?m)^ *- - `))
```

**4c. `keeps a bare wikilink under a list-style key as a scalar`** — pins Desired Behavior 3: the authored scalar is not normalised into a one-element sequence.
```go
out := roundTrip("themes: [[A Theme]]")
Expect(out).To(ContainSubstring("\nthemes: '[[A Theme]]'\n"))
Expect(out).NotTo(ContainSubstring("themes:\n"))
```

**4d. `reads a bare wikilink block-sequence entry as a string`**
```go
m, err := parse("themes:\n    - [[A Theme]]\n    - [[B Theme]]")
Expect(err).To(BeNil())
Expect(m).To(HaveKeyWithValue("themes", []any{"[[A Theme]]", "[[B Theme]]"}))
```

**4e. `writes a bare wikilink block-sequence entry back as a quoted entry`**
```go
out := roundTrip("themes:\n    - [[A Theme]]")
Expect(out).To(ContainSubstring("\n    - '[[A Theme]]'\n"))
Expect(out).NotTo(MatchRegexp(`(?m)^ *- - `))
```

**4f. `leaves an already single-quoted wikilink byte-identical`**
```go
out := roundTrip("related_task: '[[Some Task]]'")
Expect(out).To(ContainSubstring("\nrelated_task: '[[Some Task]]'\n"))
Expect(out).NotTo(ContainSubstring("''[["))
```

**4g. `does not re-quote an already double-quoted wikilink`** — the parsed value must be the bare title, not a string with literal quote characters baked in.
```go
m, err := parse(`related_task: "[[Some Task]]"`)
Expect(err).To(BeNil())
Expect(m).To(HaveKeyWithValue("related_task", "[[Some Task]]"))
```

**4h. `escapes a single quote in the wikilink title`**
```go
out := roundTrip("related_task: [[Ben's Task]]")
Expect(out).To(ContainSubstring("\nrelated_task: '[[Ben''s Task]]'\n"))

reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
Expect(err).To(BeNil())
Expect(reparsed).To(HaveKeyWithValue("related_task", "[[Ben's Task]]"))
```

**4i. `preserves an alias wikilink verbatim`**
```go
out := roundTrip("related_task: [[X|alias]]")
Expect(out).To(ContainSubstring("[[X|alias]]"))

reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
Expect(err).To(BeNil())
Expect(reparsed).To(HaveKeyWithValue("related_task", "[[X|alias]]"))
```

**4j. `preserves a heading-anchor wikilink verbatim`** — same shape as 4i with `[[X#Section]]`.

**4k. `leaves a value that merely contains a wikilink unchanged`**
```go
out := roundTrip("title: see [[X]] for details")
Expect(out).To(ContainSubstring("\ntitle: see [[X]] for details\n"))

reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
Expect(err).To(BeNil())
Expect(reparsed).To(HaveKeyWithValue("title", "see [[X]] for details"))
```

**4l. `does not quote a line holding two wikilinks`** — `related: [[A]] and [[B]]` is not valid YAML today (`yaml: did not find expected key`). The pass must leave that error intact rather than silently turning a broken line into a parseable string.
```go
_, err := parse("related: [[A]] and [[B]]")
Expect(err).NotTo(BeNil())
```

**4m. `leaves a wikilink carrying a trailing comment unchanged`** — pins the declared Non-goal. The bug deliberately persists on this line shape.
```go
out := roundTrip("related_task: [[X]]  # note")
Expect(out).To(MatchRegexp(`(?m)^ *- - X$`))
```

**4n. `leaves a wikilink inside a block scalar unchanged`** — the block-scalar body is literal text, not YAML. The trailing `other:` line proves the skip window closes at the right place.
```go
m, err := parse("description: |\n    related_task: [[X]]\n\n    second line\nother: [[Y]]")
Expect(err).To(BeNil())
Expect(m).To(HaveKeyWithValue("description", "related_task: [[X]]\n\nsecond line\n"))
Expect(m).To(HaveKeyWithValue("other", "[[Y]]"))
```

**4o. `quotes a bare wikilink nested under a parent key`** — "at any indentation".
```go
m, err := parse("nested:\n    inner: [[Deep Link]]")
Expect(err).To(BeNil())
Expect(m).To(HaveKeyWithValue("nested", map[string]any{"inner": "[[Deep Link]]"}))
```

**4p. `leaves an already-destroyed nested list unchanged`** — spec Failure Modes row: the pass fixes bare wikilinks, it does not resurrect already-destroyed ones.
```go
out := roundTrip("related_task:\n    - - Some Other Task")
Expect(out).To(MatchRegexp(`(?m)^ *- - Some Other Task$`))
```

**4q. `quotes a wikilink title containing square brackets`** — spec Failure Modes row: the outermost `[[`…`]]` pair wins and the quoted result re-parses to the full title. This line is a YAML syntax error today, so the pass strictly improves it.
```go
out := roundTrip("related_task: [[Foo [bar]]]")
Expect(out).To(ContainSubstring("\nrelated_task: '[[Foo [bar]]]'\n"))

reparsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(out))
Expect(err).To(BeNil())
Expect(reparsed).To(HaveKeyWithValue("related_task", "[[Foo [bar]]]"))
```

**4r. `does not touch wikilinks in the markdown body`**
```go
out := roundTrip("related_task: [[Some Other Task]]")
Expect(out).To(ContainSubstring("\nTags: [[Task]]\n"))
Expect(out).To(HaveSuffix("\n---\nbody\n"))
```

**4s. `is idempotent across two write cycles`**
```go
first := roundTrip("priority: 2\nrelated_task: [[Some Other Task]]\nthemes:\n    - [[A Theme]]")
parsed, err := storage.ParseToFrontmatterMapForTest(ctx, b, []byte(first))
Expect(err).To(BeNil())
second, err := storage.SerializeMapAsFrontmatterForTest(ctx, b, parsed, first)
Expect(err).To(BeNil())
Expect(second).To(Equal(first))
Expect(strings.Count(second, "related_task: '[[Some Other Task]]'")).To(Equal(1))
```

**4t. `leaves frontmatter without wikilinks unchanged`** — covers the early-return fast path.
```go
out := roundTrip("priority: 2\nstatus: in_progress")
Expect(out).To(ContainSubstring("\npriority: 2\n"))
Expect(out).To(ContainSubstring("\nstatus: in_progress\n"))
```

**4u. `still reports a pre-existing YAML syntax error`** — spec Failure Modes row: the pass cannot introduce a parse error it did not receive, and it must not swallow one either.
```go
_, err := parse("status: in_progress\n  bad: [unclosed")
Expect(err).NotTo(BeNil())
Expect(err.Error()).To(ContainSubstring("unmarshal yaml frontmatter"))
```

## 5. Document the invariant

Edit `/workspace/docs/development-patterns.md`. In the `## Entity Structure` section's `**Storage** (pkg/storage/)` bullet group, add one bullet immediately **after** the `- \`parseToFrontmatterMap\` parses the YAML frontmatter block into \`map[string]any\`` bullet and **before** the `serializeMapAsFrontmatter` bullet:

```
- **Bare-wikilink invariant**: `[[X]]` is valid YAML *flow sequence* syntax, so an unquoted
  frontmatter wikilink (`related_task: [[X]]`) unmarshals to a nested list and marshals back as
  `- - X`, silently destroying the link. `parseToFrontmatterMap` therefore runs a raw-text quoting
  pass (`quoteBareWikilinks`) before `yaml.Unmarshal`, rewriting a value that is *exactly* a bare
  wikilink — as a mapping value or as a block-sequence entry — into a single-quoted scalar. Only
  quoting changes, never YAML shape. Already-quoted values, values that merely contain a wikilink,
  values with a trailing YAML comment, and block-scalar bodies are left byte-identical. All six
  entity types inherit this from the one shared chokepoint; a new read path that bypasses
  `parseToFrontmatterMap` reintroduces the bug.
```

Do not remove or reword any other bullet in that list, and do not touch the `**Decision — the one hybrid**` group.

## 6. Changelog

Create a `## Unreleased` section in `/workspace/CHANGELOG.md`, placed **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` bullets) and **above the first `## vX.Y.Z` heading in the file** (currently `## v0.104.1`). Never between the `# Changelog` title and the preamble, and never between two released sections.

Determine the insertion point from the file itself rather than from a hardcoded version — `grep -n '^## ' CHANGELOG.md | head -3` shows the first release heading; `## Unreleased` goes immediately above it. `scripts/check-changelog.sh` only verifies that the preamble precedes the first `## ` section, so a misplacement between two released sections would NOT be caught by `make precommit`.

```
## Unreleased

- fix: an unquoted Obsidian wikilink in frontmatter (`related_task: [[Some Task]]`) is no longer destroyed by a write. `[[X]]` is valid YAML flow-sequence syntax, so every vault-cli write path silently re-emitted it as the nested block sequence `- - X`, removing the link and its backlink with no error and no exit-code change. `parseToFrontmatterMap` now quotes a value that is exactly a bare wikilink before `yaml.Unmarshal` sees it, so it round-trips as `related_task: '[[Some Task]]'` — a working link — for Task, Goal, Theme, Objective, Vision, and Decision alike. Already-quoted wikilinks, quoted list forms, values that merely contain a wikilink, and block-scalar bodies are untouched; aliases (`[[X|alias]]`), heading anchors (`[[X#Section]]`), and apostrophes in titles survive intact. A wikilink carrying a trailing YAML comment is knowingly not rewritten
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json` — the release agent owns those.

## 7. Coverage

`pkg/storage` is the changed package. `quoteBareWikilinks`, `quoteBareWikilinkLine`, and `isBlockScalarBody` must each reach **100%** statement coverage from requirement 4 — every branch has a dedicated spec (4t covers the no-`[[` early return, 4f covers the `changed == false` return, 4n covers both `isBlockScalarBody` branches and the skip-window close, 4l covers the two-wikilink rejection, 4d/4e cover the sequence regex, 4h covers the quote-doubling). Do not add retroactive coverage to unrelated untested `pkg/storage` code.

</requirements>

<constraints>
- Do NOT modify `serializeMapAsFrontmatter` in `/workspace/pkg/storage/base.go`. Spec 030 froze it and it already behaves correctly — once the map holds a `string`, `yaml.Marshal` emits exactly the quoted scalar this spec wants. If you find yourself editing that function, the fix is on the wrong side of the boundary.
- Do NOT change the signature of `parseToFrontmatterMap` or of any caller. No new exported interface, no new return value, no new parameter. Exactly one line inside the function body changes.
- Do NOT rewrite a bare wikilink that carries a trailing YAML comment (`related_task: [[X]]  # migrated`). The value portion is not exactly a wikilink; the bug persists on that line shape by design. Do not add comment-aware parsing.
- Do NOT extend the quoting to multi-line YAML flow collections, block scalars, or a wikilink embedded in a longer string (`title: see [[X]] for details`). Only a value that is *exactly* a bare wikilink from its first character to its last is rewritten.
- Do NOT normalise a bare wikilink under a conventionally-list key into a one-element sequence. `themes: [[A Theme]]` becomes the quoted **scalar** `themes: '[[A Theme]]'`. The authored form is preserved as authored.
- Do NOT auto-repair wikilinks vault-wide. No sweep command, no `--fix` flag, no migration, no lint rule. Repairing an already-destroyed `- - X` file is an operator step, not a code feature.
- Do NOT add a config key, flag, or environment variable to opt out of the quoting. It is an invariant.
- Do NOT add per-entity code. All six entity types are covered by the single shared parse chokepoint; `task.go`, `goal.go`, `theme.go`, `objective.go`, `vision.go`, and `decision.go` in `/workspace/pkg/storage/` are not touched by this prompt.
- Do NOT touch `/workspace/pkg/domain/` or `/workspace/pkg/ops/` in this prompt.
- Do NOT add any output. This spec adds no stdout, no stderr, no log line, no warning, and no exit-code change for any command. `pkg/ops/` is a library layer — operations return structured results and never write to stdout. See `/workspace/docs/development-patterns.md`.
- Do NOT add an import to `/workspace/pkg/storage/base.go`. `bytes`, `regexp`, and `strings` are already imported and are all the new code needs.
- Do NOT mutate `matches[1]` in place — it aliases the caller's `content` slice. `strings.Split` / `strings.Join` allocate fresh memory, which is why the implementation above is safe.
- Unknown frontmatter fields continue to survive read-write cycles. The pass adds quotes to bare wikilinks and changes nothing else — key order, indentation, and every other value stay exactly as the existing serializer produces them.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never a bare `return err`, never `context.Background()` in `pkg/`. (The three new functions take no ctx and return no error — they are pure text transforms.)
- Tests use Ginkgo v2 / Gomega in the external `storage_test` package. `/workspace/pkg/storage/storage_suite_test.go` already exists (suite func `TestSuite`), so new specs run automatically.
- Existing specs in `/workspace/pkg/storage/base_test.go`, `/workspace/pkg/storage/decision_test.go`, `/workspace/pkg/storage/task_test.go`, `/workspace/pkg/storage/goal_test.go`, `/workspace/pkg/storage/objective_test.go`, and `/workspace/pkg/storage/vision_test.go` must keep passing **unmodified**.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**

```
make test
```

Must exit 0, including every pre-existing storage spec.

**2. The pass exists and is wired in** — each must print at least one line:

```
grep -n 'func quoteBareWikilinks(frontmatter \[\]byte) \[\]byte' pkg/storage/base.go
grep -n 'func quoteBareWikilinkLine(line string) (string, bool)' pkg/storage/base.go
grep -n 'func isBlockScalarBody(line string, keyIndent int) bool' pkg/storage/base.go
grep -n 'bareWikilinkMappingRegex' pkg/storage/base.go
grep -n 'bareWikilinkSequenceRegex' pkg/storage/base.go
grep -n 'blockScalarStartRegex' pkg/storage/base.go
grep -n 'yaml.Unmarshal(quoteBareWikilinks(matches\[1\]), &m)' pkg/storage/base.go
```

**3. The serializer was not touched** — the frozen function must still read exactly as before:

```
grep -c 'yamlBytes, err := yaml.Marshal(data)' pkg/storage/base.go
grep -c 'buf.WriteString("---\\n")' pkg/storage/base.go
awk '/^func \(b \*baseStorage\) serializeMapAsFrontmatter\(/,/^}/' pkg/storage/base.go | grep -c 'quoteBareWikilinks'
```

The first must print `1`, the second `2`, the third `0`.

The third is the load-bearing one: it extracts the frozen function's body and proves the pass did not leak into it. Do NOT replace it with a file-wide `grep -c 'quoteBareWikilinks' pkg/storage/base.go` against a fixed expected number — that count legitimately varies with how many doc comments mention the helper, so a fixed number produces false failures and invites deleting correct comments to satisfy it.

**4. No per-entity code and no new signature** — each of these must print nothing:

```
grep -n 'quoteBareWikilinks' pkg/storage/task.go pkg/storage/goal.go pkg/storage/theme.go pkg/storage/objective.go pkg/storage/vision.go pkg/storage/decision.go
grep -rn 'quoteBareWikilinks' pkg/domain/ pkg/ops/ pkg/cli/
```

A bare `grep` with no matches exits 1 — that non-zero exit is the expected, passing result here.

**5. Named regression specs are present.**

Run the suite ONCE and capture the output, then grep the capture. Do not re-run `go test` per spec name.

```
go test -count=1 -v ./pkg/storage/ -args -ginkgo.v -ginkgo.no-color > /tmp/storage-specs.txt 2>&1
```

**Both flags are mandatory.** Without `-ginkgo.v` Ginkgo uses the dot reporter and prints no spec descriptions at all; without `-ginkgo.no-color` it injects ANSI escape sequences between the Describe and It descriptions, so a contiguous-substring grep returns 0 even though the spec ran. Either omission makes every grep below silently return 0.

Each of these must print `>= 1`:

```
grep -c "reads a bare wikilink scalar as a string" /tmp/storage-specs.txt
grep -c "writes a bare wikilink scalar back as a quoted scalar" /tmp/storage-specs.txt
grep -c "keeps a bare wikilink under a list-style key as a scalar" /tmp/storage-specs.txt
grep -c "writes a bare wikilink block-sequence entry back as a quoted entry" /tmp/storage-specs.txt
grep -c "leaves an already single-quoted wikilink byte-identical" /tmp/storage-specs.txt
grep -c "does not re-quote an already double-quoted wikilink" /tmp/storage-specs.txt
grep -c "escapes a single quote in the wikilink title" /tmp/storage-specs.txt
grep -c "preserves an alias wikilink verbatim" /tmp/storage-specs.txt
grep -c "preserves a heading-anchor wikilink verbatim" /tmp/storage-specs.txt
grep -c "leaves a value that merely contains a wikilink unchanged" /tmp/storage-specs.txt
grep -c "does not quote a line holding two wikilinks" /tmp/storage-specs.txt
grep -c "leaves a wikilink carrying a trailing comment unchanged" /tmp/storage-specs.txt
grep -c "leaves a wikilink inside a block scalar unchanged" /tmp/storage-specs.txt
grep -c "quotes a bare wikilink nested under a parent key" /tmp/storage-specs.txt
grep -c "leaves an already-destroyed nested list unchanged" /tmp/storage-specs.txt
grep -c "quotes a wikilink title containing square brackets" /tmp/storage-specs.txt
grep -c "does not touch wikilinks in the markdown body" /tmp/storage-specs.txt
grep -c "is idempotent across two write cycles" /tmp/storage-specs.txt
grep -c "leaves frontmatter without wikilinks unchanged" /tmp/storage-specs.txt
grep -c "still reports a pre-existing YAML syntax error" /tmp/storage-specs.txt
```

**6. Pre-existing specs survived** — grep the same capture from step 5; each must print `>= 1`:

```
grep -c "returns expected map entries" /tmp/storage-specs.txt
grep -c "preserves the unknown field in the map" /tmp/storage-specs.txt
grep -c "re-parses to the same map" /tmp/storage-specs.txt
grep -c "preserves every non-managed frontmatter key" /tmp/storage-specs.txt
grep -c "round-trips a list-valued key as a list" /tmp/storage-specs.txt
```

**7. No existing spec was deleted or weakened** — spec-count floors, not a diff.

`.git` is masked inside this container (`hideGit=true`), so **no `git` command works here** — `git diff` would die with `fatal: not a git repository`, and the daemon does not check verification exit codes, so it would report a pass that never ran. Count the spec-declaring lines directly instead:

```
grep -c 'It(\|DescribeTable(' pkg/storage/base_test.go        # must be > 6
grep -c 'It(\|DescribeTable(' pkg/storage/decision_test.go    # must equal 35
```

Those are the pre-change counts, measured on this branch at prompt-authoring time. If a starting count does not match, re-measure before editing and use your measured value as the floor. `base_test.go` must come back **strictly greater** (it gains 21 specs); `decision_test.go` must come back **exactly equal** (this prompt does not touch it). A count that dropped means an existing spec was deleted — restore it rather than adjusting the floor.

**8. Coverage — all three new functions at 100%:**

```
go test -coverprofile=/tmp/cover.out ./pkg/storage/... && go tool cover -func=/tmp/cover.out | grep -E 'quoteBareWikilinks|quoteBareWikilinkLine|isBlockScalarBody'
```

All three lines must report `100.0%`.

**9. Docs and changelog:**

```
grep -n 'quoteBareWikilinks' docs/development-patterns.md
grep -n 'flow sequence' docs/development-patterns.md
grep -n -A8 '^## Unreleased' CHANGELOG.md
```

The first two must each print at least one line. The third must show a `- fix:` bullet naming the wikilink round-trip.

Then confirm `## Unreleased` is the **first** `## ` heading in the file — not merely present somewhere:

```
grep -n '^## ' CHANGELOG.md | head -1
```

Must print `## Unreleased`. If it prints a version heading instead, `## Unreleased` was inserted between two released sections — move it above the first release heading.

**10. Reproduction replay, in-container** — the spec's own reproduction, driven through the library rather than the CLI. This is the mandatory bug-workflow step: it replays the exact repro from the spec and proves the bug no longer reproduces.

The program MUST live inside the module. A file under `/tmp` cannot resolve `import "github.com/bborbe/vault-cli/pkg/ops"` — it is outside the module root, so `go run /tmp/wlrepro.go` fails with a package-resolution error before it runs a single line.

Build the fixture (this part does live in `/tmp` — it is data, not code):

```
D=/tmp/wlrepro && rm -rf $D && mkdir -p "$D/24 Tasks"
printf -- '---\npriority: 2\nrelated_task: [[Some Other Task]]\nstatus: in_progress\nthemes: [[A Theme]]\n---\nTags: [[Task]]\n\n---\nbody\n' > "$D/24 Tasks/Repro Task.md"
```

Create `/workspace/zz_wlrepro/main.go` with exactly this content:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

func main() {
	ctx := context.Background()
	op := ops.NewFrontmatterSetOperation(storage.NewTaskStorage(&storage.Config{TasksDir: "24 Tasks"}))
	// Twice in a row — the second run proves idempotence.
	for _, v := range []string{"3", "4"} {
		if err := op.Execute(ctx, "/tmp/wlrepro", "Repro Task", "priority", v); err != nil {
			fmt.Fprintln(os.Stderr, "execute:", err)
			os.Exit(1)
		}
	}
}
```

Run it, then assert:

```
go run ./zz_wlrepro
grep -c '^ *- - ' "/tmp/wlrepro/24 Tasks/Repro Task.md"                                     # must be 0
grep -c "^related_task: '\[\[Some Other Task\]\]'$" "/tmp/wlrepro/24 Tasks/Repro Task.md"   # must be 1
grep -c "^themes: '\[\[A Theme\]\]'$" "/tmp/wlrepro/24 Tasks/Repro Task.md"                 # must be 1
grep -c '^Tags: \[\[Task\]\]$' "/tmp/wlrepro/24 Tasks/Repro Task.md"                        # must be 1
```

Then delete it — it must not be left in the repo:

```
rm -rf /workspace/zz_wlrepro
ls /workspace/zz_wlrepro 2>/dev/null   # must print nothing
```

**11. Full gate, once, at the end:**

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make format`, etc.) until it passes, then run `make precommit` one final time. `make format` runs golines at a 100-column limit; the regex declarations above are already split across lines to stay inside it.
</verification>
