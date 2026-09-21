---
status: completed
spec: [051-topic-command-ladder]
execution_id: vault-cli-topic-ladder-exec-221-spec-051-topic-domain-entity
dark-factory-version: v0.196.0
created: "2026-09-20T20:29:34Z"
queued: "2026-09-20T21:28:57Z"
started: "2026-09-20T21:28:59Z"
completed: "2026-09-20T21:34:01Z"
branch: dark-factory/topic-command-ladder
---

# Topic domain entity: `domain.Topic`, its own `TopicFrontmatter` wrapper, and the generic per-field view

<summary>
- Topic pages become a first-class vault entity type inside the binary, alongside tasks, goals, themes, objectives and vision.
- Reading a topic page yields its frontmatter, its file path and name, and its full markdown content — nothing is dropped.
- A frontmatter key the entity does not recognise survives a read-and-write cycle untouched.
- A topic page's `phase` value, when the page carries one, is readable as the exact string that is on disk — no validation, no rewriting, no default.
- A topic page that carries no `phase` line reports no phase at all: the key is simply absent, and reading it yields an empty result.
- The topic entity gets its own frontmatter wrapper; the goal phase type, the task phase type and their normalizers are not touched, extended or reused.
- The goal, task, theme, objective and vision entities and every existing test behave exactly as they do today.
- Nothing in this step writes, backfills, defaults or normalises a `phase` field onto any page.
- No command, no flag and no file path in the binary changes yet — this step adds the type the later steps build on.
</summary>

<objective>
Add the `Topic` domain entity and its own `TopicFrontmatter` wrapper so a topic page can be read into a typed value that preserves every frontmatter key and exposes a generic per-field read — which is what makes an already-present `phase` value observable, and an absent one genuinely absent, without inventing a phase type. This is spec 051's prompt 1 of 4: it covers Desired Behaviors 1 and 2 and Acceptance Criteria 4, 5 and 13. It is the foundation the storage layer (prompt 2) and the command family (prompt 3) compile against.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/domain/goal.go` — the entity shape you copy. `Goal` embeds `GoalFrontmatter`, `FileMetadata` and `Content`, and `NewGoal(data map[string]any, meta FileMetadata, content Content) *Goal` is its constructor. Note the doc comment on `Goal` explaining the three-concern split. Your `Topic` mirrors it exactly, minus the `Tasks []CheckboxItem` field (topics have no checkbox parsing in this spec).
- `pkg/domain/goal_frontmatter.go` — the wrapper you copy. Read `NewGoalFrontmatter`, `Phase()`, `Tags()`, `SetTags`, `DeferDate()`, `SetDeferDate`, `GetField`, `SetField`, `ClearField` and the unexported helpers it calls (`setDeferDateFromString`). This is the shape reference for `TopicFrontmatter`, with one deliberate difference: the topic wrapper has **no** typed phase accessor and **no** phase validation — see `<requirements>` § 3.
- `pkg/domain/frontmatter_map.go` — the backing store. `FrontmatterMap` is the raw `map[string]any` wrapper; `Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`, `Keys`, `RawMap` are the accessor family. Getters **coerce** rather than type-assert. `Keys()` and `RawMap()` are what the later show/list steps read.
- `pkg/domain/task_frontmatter.go` — the package-private helpers you reuse rather than reimplement: `setDateField(ctx context.Context, setter func(*libtime.DateOrDateTime), value string) error` (line ~442), `dateFieldString(d *libtime.DateOrDateTime) string` (line ~560), `stringSliceToAny(ss []string) []any` (line ~568). They live in this file but are package-scoped, so `topic_frontmatter.go` calls them unqualified.
- `pkg/domain/goal_phase.go` — READ ONLY. `GoalPhase`, `AvailableGoalPhases`, `GoalPhase.Validate`. You do **not** import, alias, extend or reference any of it from the topic wrapper.
- `pkg/domain/file_metadata.go` — `FileMetadata{Name, FilePath, ModifiedDate}`. Embedded, never stored in YAML.
- `pkg/domain/content.go` — `Content string` with a `String()` method. Embedded.
- `pkg/domain/page.go` — the generic entity used by the list path. READ ONLY; the topic entity does not extend or reuse it.
- `pkg/domain/goal_frontmatter_test.go` — the test shape you copy (Ginkgo v2 `Describe`/`It`, external `package domain_test`, `domain.NewGoalFrontmatter(...)` fixtures). In particular read the `Describe("SetField / GetField - unknown field round-trip")` block and the `Describe("GetField phase")` block — your topic specs mirror both.
- `pkg/domain/domain_suite_test.go` — the suite bootstrap; no change needed.
- `docs/development-patterns.md` § "Adding a New Command" and § "Entity Structure" — the Domain → Storage → Ops → CLI recipe and the three-concern split (frontmatter wrapper / filesystem metadata / content) this entity must follow. The `## Key Design Decisions` bullet "Map-based frontmatter — all entity frontmatter is stored in `map[string]any`; unknown fields survive read-write cycles; known fields have typed accessors" is the invariant you are extending.
- `docs/dod.md` — this repo's `validationPrompt`.
- `specs/in-progress/051-topic-command-ladder.md` — the spec. Read its Goal, Non-goals, Acceptance Criteria 4/5/13, Constraints and Failure Modes; every requirement below comes from them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct → Method, error wrapping.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` API; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comments start with the name and describe behaviour.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

**A source document that is NOT reachable, and what to do about it.** The spec's Constraints name `Personal/50 Knowledge Base/Topic Writing Guide.md` as "the documented source for the topic conventions the entity must preserve". That file lives in the Obsidian vault, which is mounted outside this repository — it is **not** present in this container and there is no mount for it. Do not search the filesystem for it, do not guess its contents, and do not invent topic-specific frontmatter conventions from it. The entity you build is deliberately convention-agnostic: it preserves *every* key it does not recognise, which is exactly what makes an unreadable convention source harmless. State this in your completion report.

**Environment facts that shape this prompt:**

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. The spec's Acceptance Criterion 13 is evidenced by `git diff <baseline>..HEAD -- pkg/domain/goal.go pkg/domain/goal_frontmatter.go pkg/domain/goal_phase.go pkg/ops/goal_workon.go` being empty; that check is **operator-side**, on the spec's `# Verification` § "Operator-executable" rung, and is not reproduced here. Section 6 below gives the mechanical non-git equivalents you must satisfy instead.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — `go build`/`go test` otherwise try to read a masked `.git`.
3. **`pkg/domain` has no `mocks/` dependency.** The domain package's tests use real values only; there is nothing to regenerate for this prompt.
</context>

<requirements>

## 0. Scope — exactly four files, all new or new-content

- `pkg/domain/topic.go` — NEW.
- `pkg/domain/topic_frontmatter.go` — NEW.
- `pkg/domain/topic_test.go` — NEW.
- `pkg/domain/topic_frontmatter_test.go` — NEW.

Nothing else. Specifically: `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go`, `pkg/domain/task_phase.go`, `pkg/domain/task_frontmatter.go`, `pkg/domain/page.go`, `pkg/domain/frontmatter_map.go`, `pkg/domain/file_metadata.go`, `pkg/domain/content.go`, `pkg/storage/**`, `pkg/ops/**`, `pkg/cli/**`, `main.go`, `go.mod`, `go.sum`, `CHANGELOG.md`, `README.md`, `docs/**` and `scenarios/**` are all untouched. Four of those are named explicitly by the spec's Acceptance Criterion 13 and must carry an empty diff against the baseline: `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go`, `pkg/ops/goal_workon.go`. No new file beyond the four above. No new dependency.

## 1. `pkg/domain/topic.go` — the entity, the constructor, the two status literals

The file carries the standard BSD license header used by every file in this package (copy the header from `pkg/domain/goal.go` verbatim, including the year line as it appears there) and declares `package domain`.

It contains exactly four things, in this order:

### 1a. The `Topic` struct

```go
// Topic represents a topic page in the Obsidian vault.
// Frontmatter is stored in TopicFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Topic struct {
	TopicFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	Content Content
}
```

- Three embedded/declared fields, in that order, no others.
- No `Tasks []CheckboxItem` field and no checkbox parsing — topics have none in this spec.
- No `yaml:` or `json:` struct tags anywhere on this struct. Frontmatter lives in the map; the struct is never marshalled.
- `FileMetadata` and `Content` are embedded without a field name, exactly as `Goal` embeds them.

### 1b. The constructor

```go
// NewTopic creates a Topic from a parsed frontmatter map and metadata.
func NewTopic(data map[string]any, meta FileMetadata, content Content) *Topic {
	return &Topic{
		TopicFrontmatter: NewTopicFrontmatter(data),
		FileMetadata:     meta,
		Content:          content,
	}
}
```

- Signature frozen: `NewTopic(data map[string]any, meta FileMetadata, content Content) *Topic`. Three parameters, that order, pointer return. It mirrors `NewGoal`.
- No validation, no normalisation, no default filling, no error return. A `nil` map is legal and yields an empty frontmatter (that is what `NewFrontmatterMap` does with `nil`).

### 1c. The two status literals

```go
// TopicStatusInProgress is the status value `vault-cli topic work-on` writes.
// It is the same literal the goal family uses for its in-progress status.
const TopicStatusInProgress = "in_progress"

// TopicStatusCompleted is the status value `vault-cli topic complete` writes.
// It is the same literal the goal family uses for its completed status.
const TopicStatusCompleted = "completed"
```

- Both are **plain untyped string constants**. Do NOT declare a `TopicStatus` named type, an `AvailableTopicStatuses` collection, a `Validate` method, a `Contains` method, a `String` method or a normalizer. The spec's Non-goals forbid building a topic phase/status type family here, and the goal and task enums are explicitly not to be reused or extended.
- Do NOT import `github.com/bborbe/collection`, `github.com/bborbe/validation` or `github.com/bborbe/errors` into this file for these constants.
- Do NOT add any other constant. No `TopicStatusNext`, no `TopicStatusHold`, no default status.

If you are tempted to give `Topic` any other field or any typed accessor of its own: don't. The accessors live on the wrapper (§ 2).

## 2. `pkg/domain/topic_frontmatter.go` — the wrapper

Standard BSD license header (as in `pkg/domain/goal_frontmatter.go`), `package domain`. Imports: `context`, `strings`, `github.com/bborbe/errors`, `libtime "github.com/bborbe/time"`. Add nothing else — in particular no `strconv`, no `time`, no `validation`, no `collection`.

### 2a. The type and constructor

```go
// TopicFrontmatter holds the YAML frontmatter for a Topic.
// It uses FrontmatterMap as its backing store so unknown fields survive round-trips.
type TopicFrontmatter struct {
	FrontmatterMap
}

// NewTopicFrontmatter constructs a TopicFrontmatter from a raw map.
func NewTopicFrontmatter(data map[string]any) TopicFrontmatter {
	return TopicFrontmatter{FrontmatterMap: NewFrontmatterMap(data)}
}
```

Both frozen. Value receiver on the constructor, matching `NewGoalFrontmatter`.

### 2b. `GetField` — the generic per-field read, and the phase contract

```go
// GetField returns the string representation of any frontmatter field by key.
func (f TopicFrontmatter) GetField(key string) string {
	switch key {
	case "phase":
		// Raw on-disk read: no validation, no default, no normalisation.
		// A topic page with no `phase` line has no key in the map, so this
		// returns "" and the key stays absent from Keys().
		return f.GetString("phase")
	case "tags":
		return strings.Join(f.Tags(), ",")
	case "defer_date":
		return dateFieldString(f.DeferDate())
	default:
		return f.GetString(key)
	}
}
```

Non-negotiable properties of the `phase` branch:

- It returns the value exactly as the YAML frontmatter holds it. It must NOT validate against any enum, must NOT lowercase, must NOT trim, must NOT map aliases, must NOT substitute a default, and must NOT consult `GoalPhase`, `TaskPhase`, `AvailableGoalPhases`, `AvailableTaskPhases` or `NormalizeTaskStatus`.
- It must NOT be implemented in terms of a typed `Phase()` method on this wrapper. There is no `Phase()` method on `TopicFrontmatter` and you must not add one — the spec's Non-goals place the typed phase accessor and its validation in a sibling task.
- An unrecognised or non-canonical on-disk value (for example `phase: whatever` or `phase: execution`) is returned verbatim and the page stays readable. There is no rejection path.
- Absent key → `f.GetString("phase")` returns `""`. Nothing is written back and `Keys()` does not gain `phase`.

The `tags` and `defer_date` branches exist because those keys hold a `[]any` and a `libtime.DateOrDateTime` respectively in the raw map, so the bare `GetString` default would stringify them wrongly. Every other key — including `status`, `assignee`, `claude_session_id` and any unknown key — goes through `default`.

- Do NOT add a `case` for any key other than those three. In particular do NOT add `status`, `assignee`, `page_type`, `completed` or `claude_session_id` cases — the default already handles them correctly and an extra case is dead code.

### 2c. `SetField` — the generic per-field write

```go
// SetField sets a frontmatter field by key from a string value.
func (f *TopicFrontmatter) SetField(ctx context.Context, key, value string) error {
	switch key {
	case "defer_date":
		return f.setDeferDateFromString(ctx, value)
	default:
		f.Set(key, value)
	}
	return nil
}
```

- Pointer receiver. Returns `error` — the `FrontmatterEntity` interface in `pkg/ops/frontmatter_entity.go` requires `SetField(ctx context.Context, key, value string) error`, and `pkg/ops` calls it through that interface in prompt 3.
- The `defer_date` branch exists because `setDeferDateFromString` parses the string into a `libtime.DateOrDateTime`; the default branch stores the raw string.
- `Set(key, value)` with an empty `value` stores `""`, which is the existing `FrontmatterMap` behaviour — do NOT special-case empty to delete, except inside `setDeferDateFromString` (see 2f), which mirrors the goal wrapper.
- There is **no** phase branch here and you must not add one: no phase validation, no phase enum, no "clear on empty" special case for phase. `topic set <page> phase <value>` goes through the default branch and stores the string verbatim, exactly as it would for any other key.
- Do NOT call `context.Background()` in this file. The `ctx` you are handed is the one you use.

### 2d. `ClearField`

```go
// ClearField removes a frontmatter field by key.
func (f *TopicFrontmatter) ClearField(key string) {
	f.Delete(key)
}
```

Pointer receiver, no return, no special cases. Mirrors `GoalFrontmatter.ClearField`.

### 2e. `Tags` / `SetTags`

```go
// Tags reads "tags" key via GetStringSlice.
func (f TopicFrontmatter) Tags() []string { return f.GetStringSlice("tags") }

// SetTags stores tags in the map. Deletes key if v is nil or empty.
func (f *TopicFrontmatter) SetTags(v []string) {
	if len(v) == 0 {
		f.Delete("tags")
		return
	}
	f.Set("tags", stringSliceToAny(v))
}
```

- `Tags` is a value receiver; `SetTags` is a pointer receiver. Both mirror `GoalFrontmatter` exactly.
- These two exist because the `add` and `remove` leaves in prompt 3 append to and drop from a list field, and every sibling entity's ops layer reads its list field through a typed accessor. Do NOT add `BlockedBy`/`SetBlockedBy` or any other list accessor — the topic list-field allowlist is `tags` only (see prompt 3).

### 2f. `DeferDate` / `SetDeferDate` / `setDeferDateFromString`

Copy `GoalFrontmatter.DeferDate`, `GoalFrontmatter.SetDeferDate` and `GoalFrontmatter.setDeferDateFromString` verbatim, with `GoalFrontmatter` → `TopicFrontmatter`:

```go
// DeferDate reads "defer_date" key as *libtime.DateOrDateTime.
func (f TopicFrontmatter) DeferDate() *libtime.DateOrDateTime {
	t := f.GetTime("defer_date")
	if t == nil {
		return nil
	}
	d := libtime.DateOrDateTime(*t)
	return &d
}

// SetDeferDate stores the defer_date in the map. Deletes key if d is nil.
func (f *TopicFrontmatter) SetDeferDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("defer_date")
		return
	}
	f.Set("defer_date", *d)
}

func (f *TopicFrontmatter) setDeferDateFromString(ctx context.Context, value string) error {
	if value == "" {
		f.SetDeferDate(nil)
		return nil
	}
	t, err := libtime.ParseTime(ctx, value)
	if err != nil {
		return errors.Wrap(ctx, err, "invalid date format")
	}
	d := libtime.DateOrDateTime(*t)
	f.SetDeferDate(&d)
	return nil
}
```

- These three exist because prompt 3's `topic defer` writes `defer_date` and its Failure Modes row requires the stored value to round-trip without an off-by-one. They mirror the goal wrapper's trio exactly, including the `libtime.ParseTime` parse and the `errors.Wrap(ctx, err, "invalid date format")` message.
- `SetDeferDate` is what prompt 3 calls with a parsed `*libtime.DateOrDateTime`; `setDeferDateFromString` is what `SetField(ctx, "defer_date", v)` calls.
- Do NOT add `SetStartDate`, `SetTargetDate`, `SetCompleted`, `SetPriority`, `SetStatus`, `SetAssignee`, `SetPageType`, `SetTheme`, `SetClaudeSessionID`, `ClearClaudeSessionID` or `Priority()`. Every one of those is a typed accessor for a field this spec does not give the topic entity a contract for; the generic `GetField`/`SetField`/`ClearField` covers them. If prompt 3 needs to write `status`, `assignee` or `claude_session_id`, it does so through `SetField` — not through a new accessor here.

## 3. What must NOT appear in the topic wrapper — the "own wrapper" constraint

These are hard, greppable constraints. `pkg/domain/topic_frontmatter.go` and `pkg/domain/topic.go` must contain **zero** occurrences of each of:

- `GoalPhase`, `AvailableGoalPhases`, `GoalPhases`
- `TaskPhase`, `AvailableTaskPhases`, `TaskPhases`, `NormalizeTaskStatus`
- `GoalStatus`, `AvailableGoalStatuses`, `GoalStatuses`, `TaskStatus`, `AvailableTaskStatuses`
- `GoalFrontmatter`, `TaskFrontmatter`, `ThemeFrontmatter`, `ObjectiveFrontmatter`, `VisionFrontmatter`
- `Goal`, `Task`, `Theme`, `Objective`, `Vision` as type references
- `validation.` and `github.com/bborbe/validation`
- `collection.` and `github.com/bborbe/collection`

The topic entity gets its **own** wrapper. The goal phase type, the task phase type and their normalizers are neither modified, nor extended, nor reused.

Also: neither file may write a `phase` key. There is no code path in these two files that sets `phase`, defaults it, backfills it, or normalises it. The only mention of `phase` is the read branch in `GetField` (§ 2b).

## 4. `pkg/domain/topic_test.go` — the entity specs

External test package `package domain_test` (matching `goal_frontmatter_test.go`), standard BSD license header, Ginkgo v2 + Gomega dot-imports, `github.com/bborbe/vault-cli/pkg/domain` imported normally.

One `var _ = Describe("Topic", func() { ... })` with these specs, each carrying a real assertion:

1. **Constructor wiring.** `domain.NewTopic(map[string]any{"status": "in_progress"}, domain.FileMetadata{Name: "Attention Routing", FilePath: "/vault/23 Topics/Attention Routing.md"}, domain.Content("---\nstatus: in_progress\n---\nbody\n"))` returns a non-nil `*domain.Topic` whose `Name`, `FilePath` and `Content` equal the values passed in, and whose `GetField("status")` equals `"in_progress"`.
2. **Nil map.** `domain.NewTopic(nil, domain.FileMetadata{}, domain.Content(""))` returns a non-nil `*domain.Topic` whose `Keys()` is empty/nil and whose `GetField("anything")` is `""`. No panic.
3. **Unknown key survives.** Build a topic from a map containing an unknown key (`map[string]any{"status": "in_progress", "attention_routing_hint": "keep me"}`), assert `GetField("attention_routing_hint")` equals `"keep me"`, then assert the key is still present in `RawMap()` after a `SetField` on an unrelated key. The spec's Desired Behavior 1 and its Constraint "Unknown frontmatter keys on a topic page survive a read-write cycle" are what this pins.
4. **The two status literals.** `domain.TopicStatusInProgress` equals `"in_progress"` and `domain.TopicStatusCompleted` equals `"completed"`. Write this as two assertions on the constants (this is the one place a constant-value assertion is the point — it is the AC's frozen literal).
5. **`Keys()` is promoted from the embedded `FrontmatterMap`.** A topic built from `map[string]any{"a": 1, "b": 2}` reports both keys from `Keys()`. This pins the embedding, which is what makes `*domain.Topic` satisfy `ops.FrontmatterEntity` in prompt 3.

Do NOT assert on `ModifiedDate` (it is populated by the storage layer, not the constructor) and do NOT add a YAML-marshalling spec here — the wrapper specs in § 5 carry the round-trip.

## 5. `pkg/domain/topic_frontmatter_test.go` — the wrapper specs

External test package `package domain_test`, standard BSD license header, same imports as `goal_frontmatter_test.go` minus the ones you do not use (`context`, Ginkgo, Gomega, `domain`, and `libtime` only if you assert a date). Copy the file's structure: a `var _ = Describe("TopicFrontmatter", func() { ... })` with a `BeforeEach` that sets `ctx = context.Background()` and `fm = domain.NewTopicFrontmatter(nil)`.

The specs below are mandatory. Name them exactly as written; the names listed in `<verification>` are grep targets there and the rest are mandatory but asserted by inspection.

### 5a. Phase — Acceptance Criterion 4 (positive) and 5 (negative)

```
Describe("GetField phase", ...)
  It("returns the raw on-disk phase string verbatim")
      fm := domain.NewTopicFrontmatter(map[string]any{"phase": "planning"})
      Expect(fm.GetField("phase")).To(Equal("planning"))

  It("returns a non-canonical phase value without rejecting the page")
      fm := domain.NewTopicFrontmatter(map[string]any{"phase": "whatever-the-vault-holds"})
      Expect(fm.GetField("phase")).To(Equal("whatever-the-vault-holds"))

  It("returns empty and leaves the key absent when the page has no phase line")
      fm := domain.NewTopicFrontmatter(map[string]any{"status": "in_progress"})
      Expect(fm.GetField("phase")).To(Equal(""))
      Expect(fm.Keys()).NotTo(ContainElement("phase"))

  It("returns empty for an empty frontmatter map")
      Expect(domain.NewTopicFrontmatter(nil).GetField("phase")).To(Equal(""))
```

The third spec is the negative-evidence half of Acceptance Criterion 5: the key must be **absent**, not present-and-empty. `ContainElement("phase")` on `Keys()` is the assertion that distinguishes the two.

### 5b. No phase is ever written

```
Describe("phase is never written by the topic wrapper", ...)
  It("does not inject phase on an unrelated mutation")
      fm := domain.NewTopicFrontmatter(nil)
      Expect(fm.SetField(ctx, "status", "in_progress")).To(Succeed())
      Expect(fm.GetField("phase")).To(Equal(""))
      Expect(fm.Keys()).NotTo(ContainElement("phase"))

  It("does not default phase when another key is cleared")
      fm := domain.NewTopicFrontmatter(map[string]any{"assignee": "alice"})
      fm.ClearField("assignee")
      Expect(fm.GetField("phase")).To(Equal(""))
      Expect(fm.Keys()).NotTo(ContainElement("phase"))
```

These pin the spec's Non-goals "Do NOT invent a default phase (for example falling back to `todo`) when a topic page has no `phase` line" and "Do NOT write, backfill, default, or normalize a `phase` field onto any topic page".

### 5c. Generic round-trip — Acceptance Criterion 5's `get` contract and Desired Behavior 1

```
Describe("SetField / GetField - unknown field round-trip", ...)
  It("round-trips an unknown key")
      Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
      Expect(fm.GetField("custom_note")).To(Equal("hello"))

  It("returns empty for an absent key")
      Expect(fm.GetField("unknown_key")).To(Equal(""))

  It("clears a key with ClearField")
      Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
      fm.ClearField("custom_note")
      Expect(fm.GetField("custom_note")).To(Equal(""))
      Expect(fm.Keys()).NotTo(ContainElement("custom_note"))
```

### 5d. Status and assignee go through the generic path

```
Describe("SetField / GetField - status and assignee", ...)
  It("stores and reads status as a plain string with no validation")
      Expect(fm.SetField(ctx, "status", "completed")).To(Succeed())
      Expect(fm.GetField("status")).To(Equal("completed"))

  It("accepts a status value no enum would recognise")
      Expect(fm.SetField(ctx, "status", "some-vault-local-status")).To(Succeed())
      Expect(fm.GetField("status")).To(Equal("some-vault-local-status"))

  It("round-trips assignee")
      Expect(fm.SetField(ctx, "assignee", "alice")).To(Succeed())
      Expect(fm.GetField("assignee")).To(Equal("alice"))
```

The second spec is the one that proves no status enum was smuggled in: a validating `SetField` would reject that value.

### 5e. Tags

```
Describe("Tags", ...)
  It("returns nil for a missing key")
  It("returns the list for a []any value")
  It("returns the list for a []string value")

Describe("SetTags", ...)
  It("stores a list")
  It("deletes the key when set to nil")
  It("deletes the key when set to an empty slice")

Describe("SetField / GetField - tags", ...)
  It("joins tags with a comma on GetField and reads them back split")
      Expect(fm.SetField(ctx, "tags", "urgent,q1")).To(Succeed())
      Expect(fm.GetField("tags")).To(Equal("urgent,q1"))
      Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
```

Note: `GoalFrontmatter.setTagsFromString` splits a comma-separated string. The topic wrapper does **not** implement a comma-split in `SetField` (the goal wrapper's `setTagsFromString` is goal-specific); instead the `default` branch stores the raw string `"urgent,q1"`, and `GetStringSlice` splits it back on comma — which is why the spec above passes. If `Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))` does not hold with the plain default branch, **stop and re-read `FrontmatterMap.GetStringSlice`** — its `case string:` arm splits on comma — rather than adding a `tags` case to `SetField`.

### 5f. defer_date

```
Describe("DeferDate", ...)
  It("returns nil for a missing key")
  It("reads a date-only value")
      fm := domain.NewTopicFrontmatter(map[string]any{"defer_date": "2027-03-19"})
      Expect(fm.DeferDate()).NotTo(BeNil())

Describe("SetDeferDate", ...)
  It("stores a date and reads it back at day granularity")
      d := libtime.DateOrDateTime(time.Date(2027, 3, 19, 0, 0, 0, 0, time.UTC))
      fm.SetDeferDate(&d)
      Expect(fm.DeferDate().Time().Format("2006-01-02")).To(Equal("2027-03-19"))

  It("deletes the key when set to nil")

Describe("SetField / GetField - defer_date", ...)
  It("round-trips a date-only value via SetField/GetField")
      Expect(fm.SetField(ctx, "defer_date", "2027-03-19")).To(Succeed())
      Expect(fm.GetField("defer_date")).To(Equal("2027-03-19"))

  It("returns an error for an unparseable value")
      Expect(fm.SetField(ctx, "defer_date", "not-a-date")).NotTo(Succeed())

  It("clears the key on an empty value")
      Expect(fm.SetField(ctx, "defer_date", "2027-03-19")).To(Succeed())
      Expect(fm.SetField(ctx, "defer_date", "")).To(Succeed())
      Expect(fm.GetField("defer_date")).To(Equal(""))
```

The day-granularity assertion is deliberate: the spec's Failure Modes row for `defer` requires no off-by-one from timezone conversion, and `GetField` renders through `dateFieldString`. Do NOT assert the raw `defer_date` map value's Go type.

### 5g. Boundary: the frontmatter map survives a read-write round-trip

```
Describe("round-trip", ...)
  It("preserves every key including unknown ones through GetField and SetField")
      fm := domain.NewTopicFrontmatter(map[string]any{
          "status": "in_progress",
          "phase":  "execution",
          "tags":   []any{"a", "b"},
          "unknown_one": "x",
          "unknown_two": 7,
      })
      Expect(fm.SetField(ctx, "status", "completed")).To(Succeed())
      Expect(fm.Keys()).To(ContainElements("status", "phase", "tags", "unknown_one", "unknown_two"))
      Expect(fm.GetField("phase")).To(Equal("execution"))
      Expect(fm.GetField("unknown_one")).To(Equal("x"))
```

This is the boundary test for the new code: the value the wrapper holds is exactly what the storage layer will hand to `yaml.Marshal` in prompt 2, so a key dropped here is a key dropped on disk.

## 6. Acceptance Criterion 13 without git — the mechanical equivalents

Acceptance Criterion 13's own evidence (`git diff <baseline>..HEAD -- <four files>` is empty) is operator-side and is not run here. In this container you satisfy the same property by construction and by these checks, all of which are in `<verification>`:

1. **The four goal-side files are not in the changed set.** Your change adds exactly four files under `pkg/domain/` and modifies nothing. Confirm by re-reading your own diff conceptually: if `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go` or `pkg/ops/goal_workon.go` appears anywhere in your work, you have violated scope — revert it.
2. **The goal-side anchors are still present and unchanged in shape.** `<verification>` greps for the goal wrapper's `Phase()` signature, the goal phase enum's `Validate` signature, and the goal status set — an accidental edit to those files changes at least one of them.
3. **The topic files reference no goal/task type.** `<verification>` greps for zero occurrences of the forbidden identifiers listed in § 3.
4. **The whole existing suite still passes.** `make test` runs the goal, task, theme, objective and vision suites unchanged; `make precommit` runs the full gate.

## 7. Self-check before finishing

- Re-read the two new source files and confirm: the `Topic` struct has exactly three fields in the documented order; `NewTopic`'s signature matches § 1b; `GetField`'s `phase` branch returns `f.GetString("phase")` and nothing else; there is no `Phase()` method; there is no `TopicStatus` type; the forbidden-identifier list in § 3 greps clean.
- Walk spec 051's Acceptance Criteria 4, 5 and 13 and state in your completion report which requirement and which spec satisfies each, plus which evidence covers each relevant row of the spec's Failure Modes table:
  - "Topic page carries a `phase` value that is not one of the canonical lifecycle values" → § 5a's second spec.
  - "Topic pages with no `phase` line parse and show without error, and no file is backfilled" → § 5a's third and fourth specs and § 5b.
  - The remaining Failure Modes rows belong to the storage (prompt 2) and command (prompt 3) layers; name them as out of scope for this prompt.
- Walk `docs/dod.md`: every exported type, function and constant has a doc comment; no `fmt.Print*` and no `os.Stdout` anywhere; errors (there are none raised in this prompt) would use `github.com/bborbe/errors`; tests use Ginkgo v2 / Gomega in an external test package; the changelog rule does not apply to this prompt (prompt 4 owns it).
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 051 — non-goals.** Do NOT add any binary leaf. Do NOT turn the eleven goal *slash commands* into binary subcommands — `plan-goal`, `execute-goal`, `verify-goal`, `audit-goal`, `create-goal`, `update-goal`, `goal-status`, `launch-goal`, `work-on-goal`, `complete-goal` and `defer-goal` are plugin-level prompt files, not `vault-cli` subcommands. Do NOT write, backfill, default or normalize a `phase` field onto any topic page. Do NOT invent a default phase (for example falling back to `todo`) when a topic page has no `phase` line. Do NOT modify, extend or reuse the goal phase type, the task phase type, or their normalizers for the topic entity — the topic entity gets its own wrapper. Do NOT change the goal command family's output, flags, exit codes, or the goal-side frontmatter allowlists. Do NOT extend the hardcoded entity-kind lists elsewhere in the binary (the watch command's accepted type list, its watch-directory set, and the resolve/type set). Do NOT add a `docs/topic-writing.md`. Do NOT ship topic slash-command files. Do NOT migrate, rewrite or reformat existing topic pages.
- **Copied from spec 051 — constraints.** The goal binary surface stays at exactly twelve leaves with unchanged names, flags, argument counts, output and exit codes. Four goal-side files carry an empty diff against the baseline: `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go`, `pkg/ops/goal_workon.go`. Existing task, goal, theme, objective and vision commands and their tests pass unchanged. Unknown frontmatter keys on a topic page survive a read-write cycle. The layered recipe in `docs/development-patterns.md` § "Adding a New Command" (Domain → Storage → Ops → CLI) governs the shape of this work; the topic entity follows the same three-concern split (frontmatter wrapper, filesystem metadata, content) as the existing entities.
- **Frozen names.** Type `Topic`; wrapper `TopicFrontmatter`; constructors `NewTopic`, `NewTopicFrontmatter`; methods `GetField`, `SetField`, `ClearField`, `Tags`, `SetTags`, `DeferDate`, `SetDeferDate`; constants `TopicStatusInProgress` = `"in_progress"`, `TopicStatusCompleted` = `"completed"`; files `pkg/domain/topic.go`, `pkg/domain/topic_frontmatter.go`. All are grep targets in the acceptance criteria.
- **Do NOT add a typed phase accessor and do NOT add a topic status/phase enum type.** No `TopicPhase`, no `Phase()`, no `Validate()`, no `AvailableTopicPhases`, no `TopicStatus` named type, no normalizer. The spec places the typed phase accessor and its validation in the sibling task `Add a Topic Phase Field With a Gated Planning → Execution Transition`.
- **Mirror the goal wrapper's shape exactly, minus phase.** Same embedded `FrontmatterMap`, same value-receiver getters / pointer-receiver setters, same `default`-branch fallthrough for unknown keys, same helper reuse (`setDateField`-style parsing via `libtime.ParseTime`, `dateFieldString`, `stringSliceToAny`).
- **Do NOT modify any existing file.** This prompt adds four files and changes nothing else. If you find yourself editing `pkg/domain/goal_frontmatter.go` to share a helper, stop — the helpers you need are already package-scoped in `pkg/domain/task_frontmatter.go`.
- **`Personal/50 Knowledge Base/Topic Writing Guide.md` is not reachable from this container.** Do not search the filesystem for it, do not guess its contents, and do not invent topic conventions from it. The wrapper's convention-agnostic preservation of unknown keys is the mitigation; say so in the completion report.
- **Tests.** Ginkgo v2 + Gomega, external test packages (`package domain_test`), no stdlib `t.Run` table tests. Every named `It` must contain a real assertion — a spec that is only named does not satisfy the acceptance criteria. Every Go file keeps its BSD license header.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass. Acceptance Criterion 13's `git diff` evidence is operator-side on the spec's verification ladder.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
- No new dependency. `go.mod` and `go.sum` are untouched.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The domain test run, with its frozen spec names.** Capture the run's output, check its exit status separately from the name greps (a failing run still prints the names), and never pipe a test command:

```
go test ./pkg/domain/... -v -ginkgo.v -count=1 > /tmp/topic-domain.log 2>&1; test "$?" = "0"
grep -F -q -- 'returns the raw on-disk phase string verbatim' /tmp/topic-domain.log
grep -F -q -- 'returns a non-canonical phase value without rejecting the page' /tmp/topic-domain.log
grep -F -q -- 'returns empty and leaves the key absent when the page has no phase line' /tmp/topic-domain.log
grep -F -q -- 'does not inject phase on an unrelated mutation' /tmp/topic-domain.log
grep -F -q -- 'does not default phase when another key is cleared' /tmp/topic-domain.log
grep -F -q -- 'round-trips an unknown key' /tmp/topic-domain.log
grep -F -q -- 'stores and reads status as a plain string with no validation' /tmp/topic-domain.log
grep -F -q -- 'accepts a status value no enum would recognise' /tmp/topic-domain.log
grep -F -q -- 'round-trips a date-only value via SetField/GetField' /tmp/topic-domain.log
grep -F -q -- 'preserves every key including unknown ones through GetField and SetField' /tmp/topic-domain.log
```

**The four new files exist and the two source files carry the frozen names:**

```
test "$(ls pkg/domain/topic.go pkg/domain/topic_frontmatter.go pkg/domain/topic_test.go pkg/domain/topic_frontmatter_test.go | wc -l | tr -d ' ')" = "4"
test "$(grep -c 'func NewTopic(' pkg/domain/topic.go)" = "1"
test "$(grep -c 'func NewTopicFrontmatter(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f TopicFrontmatter) GetField(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TopicFrontmatter) SetField(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TopicFrontmatter) ClearField(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f TopicFrontmatter) Tags(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TopicFrontmatter) SetTags(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f TopicFrontmatter) DeferDate(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TopicFrontmatter) SetDeferDate(' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'TopicStatusInProgress = "in_progress"' pkg/domain/topic.go)" = "1"
test "$(grep -c 'TopicStatusCompleted = "completed"' pkg/domain/topic.go)" = "1"
```

**The phase branch is a raw read and nothing else — the spec's Non-goals, mechanically:**

```
test "$(grep -c 'f.GetString("phase")' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'case "phase":' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f TopicFrontmatter) Phase(' pkg/domain/topic_frontmatter.go)" = "0"
test "$(grep -c 'func (f \*TopicFrontmatter) SetPhase(' pkg/domain/topic_frontmatter.go)" = "0"
test "$(grep -cE 'GoalPhase|AvailableGoalPhases|GoalPhases' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
test "$(grep -cE 'TaskPhase|AvailableTaskPhases|TaskPhases|NormalizeTaskStatus' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
test "$(grep -cE 'GoalStatus|AvailableGoalStatuses|GoalStatuses|TaskStatus|AvailableTaskStatuses' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
test "$(grep -cE 'GoalFrontmatter|TaskFrontmatter|ThemeFrontmatter|ObjectiveFrontmatter|VisionFrontmatter' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
test "$(grep -c 'bborbe/validation' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
test "$(grep -c 'bborbe/collection' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
test "$(grep -c 'type TopicStatus' pkg/domain/topic.go)" = "0"
test "$(grep -c 'type TopicPhase' pkg/domain/topic.go pkg/domain/topic_frontmatter.go | grep -vc ':0$')" = "0"
```

`grep -c` on multiple files prints one `file:count` line per file, so the `grep -vc ':0$'` idiom counts the files whose count is non-zero — it must print `0`. This is the absence form; do not rewrite it as `grep -c`, which exits 1 on a zero count and would report a failure on a correct tree.

**Acceptance Criterion 13's non-git equivalents — the goal-side anchors are intact and unmodified:**

```
test "$(grep -c 'func (f GoalFrontmatter) Phase() \*GoalPhase' pkg/domain/goal_frontmatter.go)" = "1"
test "$(grep -c 'func (g GoalPhase) Validate(ctx context.Context) error' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'GoalPhaseTodo GoalPhase = "todo"' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'GoalPhasePlanning GoalPhase = "planning"' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'GoalPhaseExecution GoalPhase = "execution"' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'GoalPhaseDone GoalPhase = "done"' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'func NewGoal(data map\[string\]any, meta FileMetadata, content Content) \*Goal' pkg/domain/goal.go)" = "1"
test "$(grep -c 'func (f \*GoalFrontmatter) SetPhase(p \*GoalPhase)' pkg/domain/goal_frontmatter.go)" = "1"
test "$(grep -c 'func NewGoalWorkOnOperation(' pkg/ops/goal_workon.go)" = "1"
```

If any of these is not `1`, one of the four frozen goal-side files was edited — revert the edit rather than adjusting the assertion.

**Formatting and the full suite:**

```
test -z "$(gofmt -e -l pkg/domain/topic.go pkg/domain/topic_frontmatter.go pkg/domain/topic_test.go pkg/domain/topic_frontmatter_test.go)"
go test ./pkg/... -count=1 > /tmp/topic-domain-pkg.log 2>&1; test "$?" = "0"
```

Finally, walk spec 051's Acceptance Criteria 4, 5 and 13 against the change and state in your completion report which requirement and which spec satisfies each one, which evidence covers the `phase`-related rows of the spec's Failure Modes table, and confirm that `Personal/50 Knowledge Base/Topic Writing Guide.md` was not reachable from this container and was therefore not read.
</verification>
