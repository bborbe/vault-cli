---
status: approved
spec: [030-bug-decision-ack-destroys-frontmatter]
created: "2026-08-09T11:15:00Z"
queued: "2026-08-09T11:51:18Z"
---

# Stop decision writes from destroying non-managed frontmatter

<summary>
- Writing a decision no longer discards every frontmatter field the tool does not itself manage.
- A Trading Decision Record keeps `selected_option`, `decision_status`, `review_date`, `related_task`, `related`, `supersedes`, and anything else it carries.
- The six fields an acknowledgement is meant to change — `needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type` — still change exactly as before.
- A list-valued field stays a list; an integer stays an integer; a boolean stays a boolean.
- The markdown body below the frontmatter is still never touched by a write.
- Where a managed field and a pre-existing field share a name, the managed value wins — that precedence is now pinned by a test.
- A decision read from a file now carries its complete frontmatter, so preservation holds for any field a future Trading Decision Record introduces without a code change.
- A hand-quoted `reviewed: "true"` is now read as reviewed instead of silently re-surfacing the decision.
- A file with unparseable frontmatter is still skipped and left byte-identical on disk.
- One rendering caveat is documented: a bare date such as `review_date: 2026-08-15` is rewritten as `2026-08-15T00:00:00Z` — same instant, normalized form — which is already how every other entity in the tool writes dates.
</summary>

<objective>
Make `domain.Decision` carry the complete parsed frontmatter map and make `WriteDecision` serialize that map with the six managed keys overlaid on top, so `vault-cli decision ack` mutates only the fields it is defined to mutate and stops silently deleting every other key on the page.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

**Precondition — prompt 1 must have shipped.** This prompt calls `domain.FrontmatterMap.GetBool`. Verify it exists before writing any code:

```
grep -n 'func (f FrontmatterMap) GetBool(key string) bool' pkg/domain/frontmatter_map.go
```

If that prints nothing, prompt 1 has not shipped. STOP, report `"status":"failed"` with the message `GetBool not yet deployed (prompt 1)`, and do NOT work around it by writing a local bool-coercion helper or a `.(bool)` assertion.

Read these files fully before making changes:

- `/workspace/pkg/domain/decision.go` (31 lines) — read fully. Current shape, verified:

  ```go
  // Decision represents a markdown file in the vault that has needs_review frontmatter.
  type Decision struct {
      // Frontmatter fields
      NeedsReview  bool                    `yaml:"needs_review"`
      Reviewed     bool                    `yaml:"reviewed,omitempty"`
      ReviewedDate *libtime.DateOrDateTime `yaml:"-"` // managed by storage layer
      Status       string                  `yaml:"status,omitempty"`
      Type         string                  `yaml:"type,omitempty"`
      PageType     string                  `yaml:"page_type,omitempty"`

      // Metadata — excluded from YAML serialization
      Name     string `yaml:"-"`
      Content  string `yaml:"-"`
      FilePath string `yaml:"-"`
  }
  ```

  It imports only `libtime "github.com/bborbe/time"`. There is no constructor.

- `/workspace/pkg/domain/frontmatter_map.go` — the type being embedded. Verified accessors (all value receiver except `Set` / `Delete`):

  ```go
  func NewFrontmatterMap(data map[string]any) FrontmatterMap   // nil data → empty map
  func (f FrontmatterMap) Get(key string) any
  func (f FrontmatterMap) GetString(key string) string          // "" when absent
  func (f FrontmatterMap) GetBool(key string) bool              // added by prompt 1
  func (f FrontmatterMap) GetTime(key string) *time.Time        // handles time.Time, libtime.DateOrDateTime, string
  func (f FrontmatterMap) GetStringSlice(key string) []string
  func (f *FrontmatterMap) Set(key string, value any)
  func (f *FrontmatterMap) Delete(key string)
  func (f FrontmatterMap) Keys() []string
  func (f FrontmatterMap) RawMap() map[string]any               // returns the underlying map; nil on a zero-value FrontmatterMap
  ```

- `/workspace/pkg/domain/goal_frontmatter.go` — read `StartDate()` around line 85 for the verified date-projection idiom you reuse:

  ```go
  func (f GoalFrontmatter) StartDate() *libtime.DateOrDateTime {
      t := f.GetTime("start_date")
      if t == nil {
          return nil
      }
      d := libtime.DateOrDateTime(*t)
      return &d
  }
  ```

- `/workspace/pkg/storage/decision.go` (262 lines) — read fully. The two functions you change are `readDecisionFromPath` (line 28, lines 43-78 are the lossy six-field projection) and `WriteDecision` (line 218, which builds a fresh six-key map — the bug). `ListDecisions`, `findByPathMatch`, `FindDecisionByName`, and `formatReviewedDate` stay as they are. Verified helper you must keep calling:

  ```go
  // formatReviewedDate serializes a *libtime.DateOrDateTime to string for YAML storage.
  // Midnight-UTC values format as YYYY-MM-DD; others as RFC3339 with timezone.
  func formatReviewedDate(d *libtime.DateOrDateTime) string
  ```

- `/workspace/pkg/storage/task.go` `WriteTask` (line 44) — the write shape to copy:

  ```go
  content, err := t.serializeMapAsFrontmatter(ctx, task.RawMap(), string(task.Content))
  ```

- `/workspace/pkg/storage/base.go` lines 24 and 46-94 — read `frontmatterRegex`, `parseToFrontmatterMap`, and `serializeMapAsFrontmatter`. Both helpers already behave correctly and are **not** modified. Verified:

  ```go
  frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
  func (b *baseStorage) parseToFrontmatterMap(ctx context.Context, content []byte) (map[string]any, error)
  func (b *baseStorage) serializeMapAsFrontmatter(ctx context.Context, data map[string]any, originalContent string) (string, error)
  ```

  `serializeMapAsFrontmatter` takes the markdown body from `originalContent` (capture group 2) and emits `yaml.Marshal(data)` between `---` fences. Keys come out alphabetically sorted.

- `/workspace/pkg/ops/decision_ack.go` — the caller. It sets `decision.Reviewed`, `decision.ReviewedDate`, and optionally `decision.Status` as plain struct fields, then calls `WriteDecision`. **This file does not change** — the fields it writes stay struct fields.

- `/workspace/pkg/ops/decision_list.go` — reads `dec.Reviewed`, `dec.ReviewedDate`, `dec.Status`, `dec.Type`, `dec.PageType` as struct fields. **This file does not change** either.

- `/workspace/pkg/storage/decision_test.go` (444 lines) — read fully. `package storage_test`, `BeforeEach` builds `store = storage.NewStorage(nil)` and a temp `vaultPath`. Existing imports: `context`, `os`, `path/filepath`, `time`, `libtime`, ginkgo, gomega, `storage`. Every existing spec assigns the six managed fields directly (`d.NeedsReview = false`, `d.ReviewedDate = &reviewedDate`) — that is why the six stay struct fields. These specs must keep compiling and passing unmodified.

- `/workspace/pkg/domain/decision_test.go` (110 lines) — read fully. It calls `yaml.Marshal(decision)` on a `domain.Decision` value and asserts the output contains `needs_review: true` and does **not** contain `reviewed_date:` or the metadata values. This is why the embedded field must carry a `yaml:"-"` tag: `gopkg.in/yaml.v3` does **not** inline anonymous struct fields unless tagged `,inline` (verified in `yaml.go` `getStructInfo`, which skips a field whose yaml tag is `-` at line 552), so an untagged embed would emit a stray `frontmattermap: {}` key.

- `/workspace/docs/development-patterns.md` — the `## Entity Structure` and `**Storage** (pkg/storage/)` sections.
- `/workspace/CHANGELOG.md` — prompt 1 created a `## Unreleased` section with one `- feat:` bullet. Append to it.

**Verified YAML round-trip behavior** (measured against `gopkg.in/yaml.v3 v3.0.1`, the version in `go.mod`, and confirmed end-to-end through `storage.WriteGoal`):

| Source line | `parseToFrontmatterMap` yields | `serializeMapAsFrontmatter` re-emits |
|---|---|---|
| `selected_option: B` | `string("B")` | `selected_option: B` |
| `unknown_count: 7` | `int(7)` | `unknown_count: 7` |
| `unknown_flag: true` | `bool(true)` | `unknown_flag: true` |
| `related:` + two `- '[[X]]'` items | `[]any{"[[X]]", ...}` | same sequence, same quoting |
| `review_date: 2026-08-15` | `time.Time` | `review_date: 2026-08-15T00:00:00Z` |

The last row is a **rendering normalization, not a data loss** — the instant is preserved — and it is pre-existing, tool-wide behavior: `storage.WriteGoal` already rewrites `start_date: 2026-08-15` as `start_date: 2026-08-15T00:00:00Z` today. Do NOT add date-reformatting logic to fix it: converting a `time.Time` back to a `YYYY-MM-DD` string would make yaml quote it (`review_date: "2026-08-15"`), which changes a non-managed key's YAML **type** from timestamp to string — exactly what this spec forbids.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `bborbe/errors` API.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>

## 1. Embed the preserved map in `domain.Decision`

In `/workspace/pkg/domain/decision.go`, replace the `Decision` struct declaration with:

```go
// Decision represents a markdown file in the vault that has needs_review frontmatter.
//
// The embedded FrontmatterMap holds the complete frontmatter parsed from the
// file, including keys this type has no field for. WriteDecision serializes that
// map and overlays only the six managed keys, so a Trading Decision Record's
// selected_option, review_date, and related survive a read-write cycle untouched.
//
// Unlike Task/Goal/Theme/Objective/Vision, Decision keeps typed struct fields for
// its six managed keys instead of an XxxFrontmatter wrapper: those fields are the
// mutation surface pkg/ops/decision_ack.go writes and pkg/ops/decision_list.go
// reads directly.
type Decision struct {
	// FrontmatterMap holds every frontmatter key parsed from the file.
	// The yaml:"-" tag keeps it out of yaml.Marshal(Decision); gopkg.in/yaml.v3
	// does not inline anonymous struct fields, so without the tag it would emit
	// a stray "frontmattermap: {}" key.
	FrontmatterMap `yaml:"-"`

	// Managed frontmatter fields — the only six keys WriteDecision overlays.
	NeedsReview  bool                    `yaml:"needs_review"`
	Reviewed     bool                    `yaml:"reviewed,omitempty"`
	ReviewedDate *libtime.DateOrDateTime `yaml:"-"` // managed by storage layer
	Status       string                  `yaml:"status,omitempty"`
	Type         string                  `yaml:"type,omitempty"`
	PageType     string                  `yaml:"page_type,omitempty"`

	// Metadata — excluded from YAML serialization
	Name     string `yaml:"-"` // Relative path from vault root without .md extension
	Content  string `yaml:"-"` // Full markdown content including frontmatter
	FilePath string `yaml:"-"` // Absolute path to file
}
```

Then add the constructor directly below the struct, above `DecisionID`:

```go
// NewDecision creates a Decision from a parsed frontmatter map and file metadata.
//
// The complete map is retained so unknown keys survive a read-write cycle; the
// six managed keys are additionally projected onto typed struct fields. Values
// are read through the coercing FrontmatterMap accessors, so a hand-quoted
// reviewed: "true" reads as true rather than silently as false.
func NewDecision(data map[string]any, name string, content string, filePath string) *Decision {
	fm := NewFrontmatterMap(data)
	decision := &Decision{
		FrontmatterMap: fm,
		NeedsReview:    fm.GetBool("needs_review"),
		Reviewed:       fm.GetBool("reviewed"),
		Status:         fm.GetString("status"),
		Type:           fm.GetString("type"),
		PageType:       fm.GetString("page_type"),
		Name:           name,
		Content:        content,
		FilePath:       filePath,
	}
	if t := fm.GetTime("reviewed_date"); t != nil {
		d := libtime.DateOrDateTime(*t)
		decision.ReviewedDate = &d
	}
	return decision
}
```

Do not add or remove imports — `libtime` is already imported and is still the only one needed. Do not add setter methods, do not add a `DecisionFrontmatter` wrapper type, and do not touch `DecisionID`.

## 2. Read the complete map in `readDecisionFromPath`

In `/workspace/pkg/storage/decision.go`, replace the whole projection block in `readDecisionFromPath` — everything from `decision := &domain.Decision{` (line 43) through `return decision, nil` (line 78) — with a single call:

```go
	return domain.NewDecision(data, name, string(content), filePath), nil
```

The `os.ReadFile` guard and the `parseToFrontmatterMap` guard above it stay byte-for-byte unchanged, including their `errors.Wrap` messages `read file %s` and `parse frontmatter`.

After this edit `readDecisionFromPath` contains no `.(bool)`, `.(string)`, or `.(time.Time)` type assertion and no `libtime.ParseTime` call — that logic now lives in the `FrontmatterMap` accessors.

## 3. Serialize the preserved map with the managed overlay in `WriteDecision`

In `/workspace/pkg/storage/decision.go`, replace the body of `WriteDecision` with:

```go
// WriteDecision writes a decision to its markdown file, preserving both the body
// content and every frontmatter key that was parsed from the file.
//
// The preserved map is written first and the six managed keys are overlaid last,
// so a managed value always wins over a preserved key of the same name.
func (d *decisionStorage) WriteDecision(ctx context.Context, decision *domain.Decision) error {
	preserved := decision.RawMap()
	data := make(map[string]any, len(preserved)+6)
	maps.Copy(data, preserved)

	// Managed overlay — applied last so managed values win over preserved ones.
	data["needs_review"] = decision.NeedsReview
	if decision.Reviewed {
		data["reviewed"] = decision.Reviewed
	}
	if decision.ReviewedDate != nil {
		data["reviewed_date"] = formatReviewedDate(decision.ReviewedDate)
	}
	if decision.Status != "" {
		data["status"] = decision.Status
	}
	if decision.Type != "" {
		data["type"] = decision.Type
	}
	if decision.PageType != "" {
		data["page_type"] = decision.PageType
	}

	content, err := d.serializeMapAsFrontmatter(ctx, data, decision.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(decision.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", decision.FilePath))
	}

	return nil
}
```

Add `"maps"` to the stdlib import group of `/workspace/pkg/storage/decision.go`. `maps.Copy(data, nil)` is a no-op, which is why a hand-constructed `&domain.Decision{}` with a zero-value `FrontmatterMap` (as `pkg/ops` tests build) writes the managed keys and nothing else instead of panicking.

Keep `formatReviewedDate` exactly as it is, in this file, and keep it as the only place `reviewed_date` is formatted. Do NOT reformat, re-type, or normalize any preserved value — copy each one through untouched.

## 4. Preservation regression tests

In `/workspace/pkg/storage/decision_test.go`, add a new top-level `Describe("frontmatter preservation", ...)` beside the existing `Describe` blocks. Do not modify, reorder, or remove any existing spec in this file.

Add `"strings"` and `"gopkg.in/yaml.v3"` to the test file's imports, plus `"github.com/bborbe/vault-cli/pkg/domain"` (needed by requirement 4f). Add this unexported helper at the bottom of the file — first run `grep -rn 'func splitDecisionFrontmatter' pkg/storage/` to confirm no collision:

```go
// splitDecisionFrontmatter returns the YAML frontmatter block and the markdown
// body of a decision file's content.
func splitDecisionFrontmatter(content string) (string, string) {
	parts := strings.SplitN(content, "---\n", 3)
	Expect(parts).To(HaveLen(3))
	return parts[1], parts[2]
}
```

The `Describe` uses this fixture — the spec's reproduction input, extended with a typed integer, a typed boolean, and `supersedes`. It is a raw backtick literal, so its lines start at column 0 inside the literal; do not indent them to match the surrounding Go code.

```go
originalContent := `---
date: 2026-08-09
decision_confidence: high
decision_status: proposed
needs_review: true
page_type: decision
related:
    - '[[Some Page]]'
    - '[[Another Page]]'
related_task: '[[Some Task]]'
review_date: 2026-08-15
selected_option: B
status: proposed
supersedes: '[[Older TDR]]'
type: Trading Decision Record
unknown_count: 7
unknown_flag: true
---
# TDR 2026-08-09 - GBPJPY V6 Pause Continuation

Body text here.

## Options

Option A, Option B.
`
```

**Structure — read this before writing any block.** Ginkgo runs every enclosing `BeforeEach` outermost-first, so a `Context` nested inside a `Describe` **does** inherit that `Describe`'s `BeforeEach`. Requirements 5 and 6 must therefore NOT be nested inside the fixture's `BeforeEach`; they must be **siblings** of it. Lay the `Describe` out like this:

```go
Describe("frontmatter preservation", func() {
    Context("after an ack-style write", func() {
        var writtenContent string
        var parsed map[string]any
        BeforeEach(func() { /* fixture write + read-back, below */ })
        // requirement 4's It blocks (4a-4h) live here
    })

    Context("managed-value precedence", func() { /* requirement 5 */ })
    Context("empty preserved map",   func() { /* requirement 6a */ })
    Context("unparseable frontmatter", func() { /* requirement 6b */ })
    Context("unwritable path",       func() { /* requirement 6c */ })
})
```

Declare `writtenContent string` and `parsed map[string]any` in the **inner** `Context`'s `var` block, not the `Describe`'s. Ginkgo closure vars are not reset between specs and `yaml.Unmarshal` **merges** into a non-nil map rather than replacing it, so set `parsed = nil` as the first statement of each `BeforeEach` that unmarshals. Without this, requirement 6a's "exactly two managed keys" assertion sees the 16-key fixture leak in and fails.

The `BeforeEach` in `Context("after an ack-style write")` writes the fixture to `filepath.Join(vaultPath, "TDR.md")`, reads it back with `store.ListDecisions(ctx, vaultPath)` (expect exactly one), applies the same mutations `decisionAckOperation.Execute` applies plus the `needs_review` flip, writes, and re-reads the raw file:

```go
d := decisions[0]
d.Reviewed = true
reviewedDate := libtime.DateOrDateTime(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC))
d.ReviewedDate = &reviewedDate
d.NeedsReview = false
Expect(store.WriteDecision(ctx, d)).To(Succeed())

rawBytes, err := os.ReadFile(filePath)
Expect(err).To(BeNil())
writtenContent = string(rawBytes)

fmYAML, _ := splitDecisionFrontmatter(writtenContent)
Expect(yaml.Unmarshal([]byte(fmYAML), &parsed)).To(Succeed())
```

Re-read via `os.ReadFile` + `yaml.Unmarshal`, **not** via `store.ListDecisions` — after the `needs_review` flip `ListDecisions` filters the file out, so a storage round-trip cannot see it.

`forcetypeassert` is enabled in `.golangci.yml` with `run.tests: true`, so every type assertion below must use the two-value form.

Add these `It` blocks with exactly these names — `<verification>` greps them:

**4a. `preserves every non-managed frontmatter key`**
```go
Expect(parsed).To(HaveKeyWithValue("decision_confidence", "high"))
Expect(parsed).To(HaveKeyWithValue("decision_status", "proposed"))
Expect(parsed).To(HaveKeyWithValue("selected_option", "B"))
Expect(parsed).To(HaveKeyWithValue("related_task", "[[Some Task]]"))
Expect(parsed).To(HaveKeyWithValue("supersedes", "[[Older TDR]]"))
Expect(parsed).To(HaveKey("date"))
Expect(parsed).To(HaveKey("review_date"))
Expect(parsed).To(HaveKey("related"))
```

**4b. `preserves date-valued keys as the same instant`**
```go
reviewVal, ok := parsed["review_date"].(time.Time)
Expect(ok).To(BeTrue(), "review_date must stay a YAML timestamp")
Expect(reviewVal.UTC()).To(Equal(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)))

dateVal, ok := parsed["date"].(time.Time)
Expect(ok).To(BeTrue(), "date must stay a YAML timestamp")
Expect(dateVal.UTC()).To(Equal(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)))
```

**4c. `re-renders a bare date in RFC3339 form without changing the instant`** — pins the documented normalization so it can never regress silently into a quoted string:
```go
Expect(writtenContent).To(ContainSubstring("\nreview_date: 2026-08-15T00:00:00Z\n"))
Expect(writtenContent).NotTo(ContainSubstring("review_date: \"2026-08-15\""))
```
Match on the full line including the leading `\n` — a bare `date:` substring also matches `review_date:` and `reviewed_date:`.

**4d. `round-trips a list-valued key as a list`**
```go
relatedVal, ok := parsed["related"].([]any)
Expect(ok).To(BeTrue(), "related must round-trip as a sequence, not a flattened string")
Expect(relatedVal).To(HaveLen(2))
Expect(relatedVal[0]).To(Equal("[[Some Page]]"))
Expect(relatedVal[1]).To(Equal("[[Another Page]]"))
```

**4e. `does not coerce an unknown key to a different YAML type`**
```go
countVal, ok := parsed["unknown_count"].(int)
Expect(ok).To(BeTrue(), "unknown_count must round-trip as int, not string")
Expect(countVal).To(Equal(7))

flagVal, ok := parsed["unknown_flag"].(bool)
Expect(ok).To(BeTrue(), "unknown_flag must round-trip as bool, not string")
Expect(flagVal).To(BeTrue())
```

**4f. `updates the six managed fields`**
```go
Expect(parsed).To(HaveKeyWithValue("needs_review", false))
Expect(parsed).To(HaveKeyWithValue("reviewed", true))
Expect(parsed).To(HaveKeyWithValue("reviewed_date", "2026-08-09"))
Expect(parsed).To(HaveKeyWithValue("status", "proposed"))
Expect(parsed).To(HaveKeyWithValue("type", "Trading Decision Record"))
Expect(parsed).To(HaveKeyWithValue("page_type", "decision"))
```
`reviewed_date` re-parses as the **string** `"2026-08-09"`, not a `time.Time`: `formatReviewedDate` emits a `YYYY-MM-DD` string for a midnight-UTC value and yaml quotes it. The existing spec at `decision_test.go:391` already asserts that raw form.

**4g. `leaves the markdown body byte-identical`**
```go
_, originalBody := splitDecisionFrontmatter(originalContent)
_, writtenBody := splitDecisionFrontmatter(writtenContent)
Expect(writtenBody).To(Equal(originalBody))
```

**4h. `loses no frontmatter key`** — the count guard that catches a future key being dropped without anyone adding an assertion for it:
```go
Expect(parsed).To(HaveLen(16))
```
Sixteen is the exact expected key count: the fixture declares 14 keys — `date`, `decision_confidence`, `decision_status`, `needs_review`, `page_type`, `related`, `related_task`, `review_date`, `selected_option`, `status`, `supersedes`, `type`, `unknown_count`, `unknown_flag` — and the write adds exactly two more, `reviewed` and `reviewed_date`. If this assertion fails, a key was dropped or an extra key was invented; do not adjust the number to match the output.

## 5. Managed-value precedence

Add an `It("lets a managed value win over the preserved key of the same name")` in `Context("managed-value precedence")` — a **sibling** of `Context("after an ack-style write")`, per the structure diagram in requirement 4, so it does not inherit that fixture's `BeforeEach`. Write the fixture (which carries `status: proposed`), read it, set `d.Status = "accepted"`, write, and assert the re-parsed frontmatter has `status` equal to `"accepted"` and exactly one `status:` line (`strings.Count(writtenContent, "\nstatus:")` equals `1`).

This is the container-executable proof of the spec's Failure Modes row "A non-managed key collides with a managed key name → managed value wins — the overlay is applied last".

## 6. Empty-preserved-map and error-path tests

Add these three `It` blocks, each in its own `Context` that is a **sibling** of `Context("after an ack-style write")` — never nested inside it (see the structure diagram in requirement 4):

**6a. `writes the managed keys when the preserved map is empty`** — spec Failure Modes row "Decision file has no frontmatter at all". Build the decision by hand so `RawMap()` is nil, exactly as `pkg/ops` tests do:
```go
d := &domain.Decision{
	NeedsReview: true,
	Status:      "proposed",
	Name:        "Empty",
	Content:     "---\nneeds_review: true\n---\n# Empty\n\nBody.\n",
	FilePath:    filepath.Join(vaultPath, "Empty.md"),
}
Expect(d.RawMap()).To(BeNil())
Expect(store.WriteDecision(ctx, d)).To(Succeed())
```
Then re-read the file, unmarshal the frontmatter, and assert it has exactly the two managed keys (`needs_review: true`, `status: proposed`) and that the body is still `# Empty\n\nBody.\n`.

**6b. `skips a file with unparseable frontmatter and leaves it byte-identical`** — spec Failure Modes row "Decision file has malformed YAML frontmatter". Write a file whose frontmatter block contains invalid YAML (for example `needs_review: true\n  bad: [unclosed\n`), call `store.ListDecisions`, assert no error and that the malformed file is not among the results, then re-read the file and assert its content equals the fixture byte-for-byte.

**6c. `returns a wrapped error when the file cannot be written`** — spec Failure Modes row "Write fails partway". Build a `&domain.Decision{}` whose `FilePath` points inside a directory that does not exist (`filepath.Join(vaultPath, "no-such-dir", "X.md")`) and whose `Content` is a valid frontmatter document, call `store.WriteDecision`, and assert the returned error is non-nil and its `Error()` contains `write file`.

## 7. Domain-level constructor test

In `/workspace/pkg/domain/decision_test.go`, add a new top-level `Describe("NewDecision", ...)` beside the existing `Describe("YAML marshaling")`. Do not modify the existing blocks — they are the regression lock for the `yaml:"-"` tag on the embedded field.

Add these `It` blocks with exactly these names:

- `projects the six managed keys onto struct fields` — call `domain.NewDecision(map[string]any{"needs_review": true, "reviewed": false, "status": "proposed", "type": "Trading Decision Record", "page_type": "decision"}, "TDR", "content", "/vault/TDR.md")` and assert each field plus `Name`, `Content`, `FilePath`.
- `retains keys it has no field for` — pass a map containing `selected_option: "B"` and assert `d.Get("selected_option")` equals `"B"` and `d.RawMap()` has the same length as the input map.
- `reads a hand-quoted reviewed value as true` — pass `map[string]any{"reviewed": "true"}` and assert `d.Reviewed` is `true`. This is the coercion hole `GetBool` closes; a `.(bool)` assertion would yield `false` here.
- `parses reviewed_date from a YAML date value` — pass `map[string]any{"reviewed_date": time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)}` and assert `d.ReviewedDate` is non-nil and `d.ReviewedDate.Time().UTC()` equals that instant.
- `leaves ReviewedDate nil when the key is absent` — pass an empty map and assert `d.ReviewedDate` is nil.
- `does not emit the embedded map when the struct is marshaled` — call `yaml.Marshal(*domain.NewDecision(map[string]any{"needs_review": true, "selected_option": "B"}, "n", "c", "p"))` and assert the output does not contain `frontmattermap` and does not contain `selected_option`. This pins the `yaml:"-"` tag.

Add `"time"` to the test file's imports; `yaml` and `domain` are already imported.

## 8. Documentation

Edit `/workspace/docs/development-patterns.md`:

- In `## Entity Structure`, after the `**Markdown content**` bullet group and before the `**Storage** (pkg/storage/)` group, add a short `**Decision — the one hybrid**` paragraph: `domain.Decision` embeds `FrontmatterMap` directly (tagged `yaml:"-"`) rather than a `DecisionFrontmatter` wrapper, and keeps typed struct fields for its six managed keys — `needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type` — because those fields are the mutation surface `pkg/ops/decision_ack.go` writes. `WriteDecision` copies the preserved map and overlays those six last, so a managed value wins on a name collision and every other key round-trips untouched.
- In the `**Storage** (pkg/storage/)` bullet group, add one bullet recording the rendering caveat: a bare YAML date (`review_date: 2026-08-15`) parses to `time.Time` and re-serializes as RFC3339 (`review_date: 2026-08-15T00:00:00Z`). The instant is preserved; only the rendering is normalized. This applies to every entity, not just decisions — `WriteGoal` behaves the same way today.

Do not remove or reword any other bullet.

## 9. Changelog

Append **one** bullet to the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (prompt 1 created it, directly above the first release heading — currently `## v0.103.0`). Do not create a second `## Unreleased` heading, do not replace the existing bullet, and do not touch any released `## vX.Y.Z` section:

```
- fix: `vault-cli decision ack` no longer deletes frontmatter fields it does not manage. `domain.Decision` now carries the full parsed frontmatter map and `WriteDecision` overlays only `needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, and `page_type` onto it, so a Trading Decision Record keeps `selected_option`, `decision_status`, `review_date`, `related_task`, `related`, and `supersedes` — with list, integer, and boolean values keeping their YAML types
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json` — the release agent owns those.

## 10. Coverage

The changed packages are `pkg/domain` and `pkg/storage`. `domain.NewDecision` must reach 100% statement coverage from requirement 7 (the `reviewed_date` present and absent branches are both covered). `decisionStorage.WriteDecision` must be exercised along the preserved-map path (requirement 4), the empty-map path (6a), the managed-overlay path (5), and the write-error path (6c). Do not add retroactive coverage to unrelated untested code.

</requirements>

<constraints>
- The managed key set stays exactly `needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type`. Do NOT add, remove, or rename a managed field, and do NOT change when each one is emitted (`needs_review` always; the other five only when non-zero).
- Do NOT modify `serializeMapAsFrontmatter` or `parseToFrontmatterMap` in `/workspace/pkg/storage/base.go`. They already behave correctly, and five other entity types depend on their exact semantics.
- Do NOT modify `/workspace/pkg/ops/decision_ack.go` or `/workspace/pkg/ops/decision_list.go`. They read and write the six managed keys as struct fields and inherit the fix unchanged. If you find yourself converting `decision.Reviewed` to `decision.Reviewed()`, stop — that is the wrong design for this spec.
- Do NOT touch any other domain type. `decision.go` is the sole outlier; `task.go`, `goal.go`, `objective.go`, `theme.go`, and `vision.go` are already correct.
- Do NOT add a `DecisionFrontmatter` wrapper type, typed getter methods, or typed setter methods on `Decision`. The six stay plain struct fields; the embedded `FrontmatterMap` is for preservation only.
- Do NOT reformat, re-type, or normalize a preserved value on write. In particular, do NOT convert a `time.Time` back to a `YYYY-MM-DD` string to make the diff smaller — yaml would quote it, changing a non-managed key's YAML type from timestamp to string.
- `reviewed_date` formatting keeps using `formatReviewedDate` in `/workspace/pkg/storage/decision.go`: midnight-UTC values format as `YYYY-MM-DD`, everything else as RFC3339. Do not move, rename, or reimplement it.
- Do NOT use a bare `.(bool)` type assertion to read a frontmatter bool — use `FrontmatterMap.GetBool`. `forcetypeassert` is enabled and `run.tests: true`, so every assertion in code and tests uses the two-value form.
- Do NOT add a flag, config key, environment variable, or opt-out to select whitelist-replace vs preserve. Preservation is invariant.
- Existing specs in `/workspace/pkg/storage/decision_test.go`, `/workspace/pkg/domain/decision_test.go`, `/workspace/pkg/ops/decision_ack_test.go`, and `/workspace/pkg/ops/decision_list_test.go` must keep passing **unmodified**. They assign and read the six managed fields as struct fields; if any of them stops compiling, the struct change is wrong — fix the struct, not the test.
- Out of scope, do not attempt: reconstructing fields lost by decisions acked before this fix (restore from git history by hand), and any form of write locking for the vault-cli / git-rest concurrent-writer race documented in `specs/completed/026-bug-duplicate-frontmatter-key-silent-task-loss.md`. Preserving the map neither improves nor worsens that race.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` in `pkg/`.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never a bare `return err`, never `context.Background()` in `pkg/`.
- Tests use Ginkgo v2 / Gomega in external `_test` packages. `pkg/storage/storage_suite_test.go` and `pkg/domain/domain_suite_test.go` already exist, so new specs run.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**

```
make test
```

Must exit 0, including every pre-existing decision spec.

**2. Spec acceptance greps** — each must print at least one line:

```
grep -n 'FrontmatterMap' pkg/domain/decision.go
grep -n 'RawMap()' pkg/storage/decision.go
grep -n 'GetBool' pkg/domain/decision.go
grep -n 'maps.Copy' pkg/storage/decision.go
```

**3. The lossy projection is gone** — each of these must print nothing:

```
grep -n 'data\["needs_review"\]' pkg/storage/decision.go
grep -n 'data\["status"\]\.(string)' pkg/storage/decision.go
grep -n 'libtime.ParseTime' pkg/storage/decision.go
```

A bare `grep` with no matches exits 1 — that non-zero exit is the expected, passing result here.

**4. The ops layer was not rewritten** — each must still print at least one line, proving `pkg/ops` was left alone:

```
grep -n 'decision.Reviewed = true' pkg/ops/decision_ack.go
grep -n 'decision.ReviewedDate = &reviewedDate' pkg/ops/decision_ack.go
grep -n 'dec.PageType' pkg/ops/decision_list.go
```

**5. Named regression specs are present** — each command must print a number `>= 1`:

```
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "preserves every non-managed frontmatter key"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "preserves date-valued keys as the same instant"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "re-renders a bare date in RFC3339 form without changing the instant"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "round-trips a list-valued key as a list"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "does not coerce an unknown key to a different YAML type"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "updates the six managed fields"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "leaves the markdown body byte-identical"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "loses no frontmatter key"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "lets a managed value win over the preserved key of the same name"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "writes the managed keys when the preserved map is empty"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "skips a file with unparseable frontmatter and leaves it byte-identical"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "returns a wrapped error when the file cannot be written"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "reads a hand-quoted reviewed value as true"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "retains keys it has no field for"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "does not emit the embedded map when the struct is marshaled"
```

The `-ginkgo.v` flag is required — plain `go test -v` does not enable Ginkgo's verbose reporter.

**6. Pre-existing specs survived** — each must print `>= 1`, proving the struct change did not force a test rewrite:

```
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "preserves markdown body content and only changes frontmatter"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "writes midnight-UTC DateOrDateTime as YYYY-MM-DD in frontmatter"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "reads YAML date literal as date-only DateOrDateTime"
go test -count=1 ./pkg/storage/... -v -ginkgo.v 2>&1 | grep -c "leaves ReviewedDate nil when reviewed_date is absent"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "does not marshal metadata fields"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "round-trips correctly for YAML-managed fields"
```

**7. No existing spec was deleted or weakened** — spec-count floors, not a diff.

`.git` is masked inside this container (`hideGit=true`), so **no `git` command works here** — `git diff` would die with `fatal: not a git repository`, and the daemon does not check verification exit codes, so it would report a pass that never ran. Count the spec-declaring lines directly instead. Each command must print a number **greater than or equal to** the floor shown:

```
grep -c 'It(\|DescribeTable(' pkg/ops/decision_ack_test.go      # must equal 11
grep -c 'It(\|DescribeTable(' pkg/ops/decision_list_test.go     # must equal 12
grep -c 'It(\|DescribeTable(' pkg/storage/decision_test.go      # must be > 23
grep -c 'It(\|DescribeTable(' pkg/domain/decision_test.go       # must be > 9
```

Those four numbers are the pre-change counts, measured on this branch at prompt-authoring time. If a starting count does not match, re-measure before editing and use your measured value as the floor. `pkg/ops/*` must come back **exactly equal** (this prompt does not touch them); `pkg/storage/decision_test.go` and `pkg/domain/decision_test.go` must come back **strictly greater** (they gain specs). A count that dropped means an existing spec was deleted or weakened — restore it rather than adjusting the floor.

**8. Coverage:**

```
go test -coverprofile=/tmp/cover.out ./pkg/domain/... && go tool cover -func=/tmp/cover.out | grep NewDecision
go test -coverprofile=/tmp/cover2.out ./pkg/storage/... && go tool cover -func=/tmp/cover2.out | grep -E 'WriteDecision|readDecisionFromPath'
```

`NewDecision` must report `100.0%`; `WriteDecision` and `readDecisionFromPath` must both be at or above `80.0%`.

**9. Docs and changelog:**

```
grep -n -i 'decision' docs/development-patterns.md
grep -n 'RFC3339' docs/development-patterns.md
grep -n -A12 '^## Unreleased' CHANGELOG.md
```

The first two must each print at least one line. The third must show **both** the `- feat:` bullet from prompt 1 and the new `- fix:` bullet naming `vault-cli decision ack`, under a single `## Unreleased` heading.

**10. Full gate, once, at the end:**

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make format`, etc.) until it passes, then run `make precommit` one final time. `make format` runs golines at a 100-column limit and will reflow the long fixture literals — let it, but note that golines does not reflow raw backtick strings, so the fixture stays intact.
</verification>
