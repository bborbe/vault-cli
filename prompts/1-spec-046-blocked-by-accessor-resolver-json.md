---
spec: ["046-blocked-by-frontmatter-model"]
status: draft
created: "2026-09-11T07:53:52Z"
---

# Blocked-by dependency model: typed accessors, blocked-state resolver, list JSON emission

<summary>
- A task or goal file can list what it depends on in a `blocked_by` frontmatter list.
- Task and goal list output reports both the raw dependency list and a computed "is blocked" boolean, so consumers stop re-parsing frontmatter.
- A dependency counts as satisfied only when the named item is `status: completed`; any other status, a missing file, an unreadable file or a file with no readable status all leave the dependent blocked — "cannot verify it is done" never means "go ahead".
- Dependency names match by name, ignoring case and `[[wikilink]]` brackets, against files in the same directory (a task's dependencies are tasks, a goal's are goals).
- An item with no dependency list reports neither new field, and every pre-existing JSON field keeps its name and value.
- Blocked state is derived only — no `status` is written, no item is moved to `hold`, and no existing vault file is migrated.
- A dependency cycle leaves both items blocked without recursion or hang, because only a single status read is ever performed.
- A malformed dependency field (a scalar instead of a list) reads as empty and never blocks.
- Unit tests cover the resolver edge cases; an end-to-end test drives the real binary against a throwaway vault, including the blocked → unblocked transition, and the changelog records the new JSON surface.

</summary>

<objective>
Give tasks and goals a typed dependency list plus a computed blocked flag in `list` JSON output, so slash commands and the Vault UI read one authoritative derived value instead of each re-implementing blocked-state from raw frontmatter. This covers spec 046 Desired Behaviors 1-5 and Acceptance Criteria 1, 2, 3, 5 and 6 — the field contract and the derived state; the command that consumes the flag and the doc rewrite are prompts 2 and 3.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` (sections `## Entity Structure`, `## Output Format`, `## Testability`) for project conventions, and `docs/dod.md` for the definition of done.

Read these files fully before making changes:

- `pkg/domain/frontmatter_map.go` — the backing store for every frontmatter type. Note `GetStringSlice` (used by `Goals()` / `Tags()`): it accepts `[]string`, `[]any` and — importantly — a **scalar `string`, which it comma-splits into a list**. That comma-split is why the new accessor needs its own read helper (requirement 1). `fmt` and `strings` are already imported.
- `pkg/domain/task_frontmatter.go` — `TaskFrontmatter` embeds `FrontmatterMap`. The accessor to mirror is `Goals()` (`return f.GetStringSlice("goals")`, a value receiver, one line, doc comment above). `GetField` / `SetField` dispatch switches are NOT extended by this prompt — `blocked_by` stays an unknown key there, which is what keeps `task set <name> blocked_by ""` from failing.
- `pkg/domain/goal_frontmatter.go` — `GoalFrontmatter`, same shape. `Tags()` is the neighbouring accessor to place next to.
- `pkg/domain/page.go` — `Page` is what the list operation actually iterates. It embeds `FrontmatterMap`, `FileMetadata` (carrying `Name`) and `Content`; it already re-declares the task-shaped accessors it needs (`Goals()`, `Flag()`, `DeferDate()`, …). The new accessor belongs there too, otherwise the list path cannot reach it.
- `pkg/domain/task_status.go` — `TaskStatusCompleted` (`"completed"`) and `NormalizeTaskStatus`, which maps the legacy alias `done` → `completed`. `Page.Status()` applies that normalization, so `status: done` reads as completed.
- `pkg/ops/list.go` — `ListOperation.Execute` and `TaskListItem`. Verified current shape:
  - `Execute` reads all pages first: `tasks, err := l.pageStorage.ListPages(ctx, vaultPath, pagesDir)`, then filters (`filteredTasks := filterTasks(tasks, …)`) and sorts. The unfiltered `tasks` slice is still in scope when the output items are built — that is the slice the resolver must receive (requirement 3).
  - `TaskListItem` is built in a single loop; `items[i].Goals = task.Goals()` is the last assignment.
  - `pkg/cli/cli.go` reuses this one operation and this one struct for `task list` AND for `createGenericListCommand` (goal / theme / objective / vision list), so one emission point covers both AC1 and AC2.
- `pkg/ops/daily_note_entry.go` — the in-repo precedent for a plain exported boolean helper in `pkg/ops` (`IsOwnDailyNoteEntry(checkboxText string, taskName string) bool`) with a doc comment stating its rules. The resolver follows that shape: no interface, no mock, no I/O.
- `pkg/ops/list_test.go` — the `Describe("ListOperation", …)` suite. Fixtures are built with `domain.NewPage(map[string]any{…}, domain.FileMetadata{Name: …}, domain.Content(""))` and injected with `mockPageStorage.ListPagesReturns(tasks, nil)`; the existing `Context("Goals field in TaskListItem", …)` and `Context("Flag field in TaskListItem", …)` show the emission-assertion style (`json.Marshal(item)` + substring checks).
- `pkg/ops/daily_note_entry_test.go` — the `DescribeTable` style for a pure helper (`package ops_test`).
- `pkg/domain/page_test.go` — `Describe("Page Flag", …)`, the `DescribeTable` style for a `Page` accessor.
- `pkg/domain/task_frontmatter_metrics_test.go` — the in-repo frontmatter serialization round-trip pattern: `yaml.Marshal(fm.RawMap())`, assert on the serialized text, then `yaml.Unmarshal` back into `map[string]any` and rebuild with `domain.NewTaskFrontmatter(raw)`.
- `integration/cli_test.go` — the end-to-end harness. `createTempVault(tasks map[string]string)` and `createTempVaultWithGoals(tasks, goals)` write real task/goal files into a temp vault plus a config file and return `(vaultPath, configPath, cleanup)`. Tests run the built binary with `gexec.Start(exec.Command(binPath, "--config", configPath, "--vault", "test", …))` and assert with `Eventually(session).Should(gexec.Exit(0))`. `Describe("vault-cli task JSON schema", …)` is the closest existing model — it already decodes `task list --output json` into `[]map[string]any`.
- `CHANGELOG.md` — read the top ~15 lines. At authoring time there is **no** `## Unreleased` section and the newest versioned section is `## v0.130.2`.

Read this coding-plugin doc (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, table tests.

<!-- OPEN QUESTIONS (resolved by the prompt writer; flagged for the human reviewer):

1. Spec Desired Behavior 1 says the accessor mirrors `Goals()` with "(missing or non-list value → empty slice)", but the spec's Failure Modes table says a malformed scalar `blocked_by` must be "treated as empty → unblocked". Those two cannot both hold if the accessor calls `GetStringSlice`: that helper comma-splits a scalar string, so `blocked_by: A` would become the one-element list `["A"]` and could block. The Failure Modes table wins (it is the testable statement), so the accessor reads through a new list-only helper (requirement 1). Consequence for the reviewer: `blocked_by: "A"` and `blocked_by: "A,B"` (scalar forms) both read as empty, whereas `blocked_by: [A]` and `blocked_by: [A, B]` (list forms) read as lists. `GetStringSlice` itself is NOT modified — `Goals()` / `Tags()` keep their current scalar behaviour.

2. `Page` needs the accessor too, even though the spec names only `TaskFrontmatter` and `GoalFrontmatter`: `ListOperation` iterates `[]*domain.Page`, not `*domain.Task` / `*domain.Goal`. `Page` already re-declares `Goals()` for exactly this reason. The spec's `grep -n 'blocked_by' pkg/domain/task_frontmatter.go pkg/domain/goal_frontmatter.go` smoke check still passes unchanged.

3. `Blocked` is a `*bool`, not a `bool` (requirement 3). A plain `bool` with `omitempty` would drop `"blocked": false` — which AC5 requires to be present once a task's blockers are all completed; a plain `bool` without `omitempty` would add `"blocked": false` to every item and break AC3. The pointer is the only shape that satisfies both.

4. `theme list` / `objective list` / `vision list` share `ops.ListOperation` and `ops.TaskListItem`, so they will emit the two fields as well whenever those files carry the key. That is an accepted consequence of one shared emission point, not an added feature — do not add per-page-type branching. The spec documents the surface for tasks and goals only.
-->

</context>

<requirements>

## 1. Domain: read `blocked_by` as a list, strictly

### 1a. The list-only read helper

In `pkg/domain/frontmatter_map.go`, add an unexported package-level helper immediately **after** the existing `GetStringSlice` method:

```go
// blockedByList reads the "blocked_by" frontmatter value as a list of blocker
// names. Only YAML list shapes are accepted: []any as produced by the YAML
// parser, []string as produced by in-memory setters. A scalar value — including
// a plain string — is malformed data rather than a one-element list, so it reads
// as an empty list and the entity stays unblocked. GetStringSlice is deliberately
// NOT used here: it comma-splits a scalar string, which would turn a malformed
// `blocked_by: A` into a real blocker.
func blockedByList(f FrontmatterMap) []string {
	switch v := f.Get("blocked_by").(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	default:
		return nil
	}
}
```

Do NOT change `GetStringSlice`, `GetBool`, `GetTime`, `Get`, `Set`, `Delete`, `Keys` or `RawMap`.

### 1b. The three accessors

Each is a one-line value-receiver method with a doc comment, placed next to the accessor it mirrors:

- `pkg/domain/task_frontmatter.go` — immediately after `Goals()`:
  ```go
  // BlockedBy reads the "blocked_by" key as a list of blocker names.
  // Only a YAML list is accepted (plain names or [[wikilinks]]); a scalar value
  // is malformed and reads as an empty list, so malformed data never blocks.
  func (f TaskFrontmatter) BlockedBy() []string { return blockedByList(f.FrontmatterMap) }
  ```
- `pkg/domain/goal_frontmatter.go` — immediately after `Tags()`:
  ```go
  // BlockedBy reads the "blocked_by" key as a list of blocker names.
  // Only a YAML list is accepted (plain names or [[wikilinks]]); a scalar value
  // is malformed and reads as an empty list, so malformed data never blocks.
  func (f GoalFrontmatter) BlockedBy() []string { return blockedByList(f.FrontmatterMap) }
  ```
- `pkg/domain/page.go` — immediately after `Goals()`:
  ```go
  // BlockedBy reads the "blocked_by" key as a list of blocker names.
  // Only a YAML list is accepted (plain names or [[wikilinks]]); a scalar value
  // is malformed and reads as an empty list, so malformed data never blocks.
  func (p Page) BlockedBy() []string { return blockedByList(p.FrontmatterMap) }
  ```

Do NOT add a `SetBlockedBy`, do NOT add a `blocked_by` case to `TaskFrontmatter.SetField` / `GetField` or to `GoalFrontmatter.SetField` / `GetField`. `blocked_by` stays an unknown key in those dispatch switches, so `task set <name> blocked_by ""` stores an empty string and the entity reads back unblocked through `blockedByList`'s `default` branch. `task clear <name> blocked_by` already deletes the key through `ClearField`.

## 2. Ops: the blocked-state resolver

Create `pkg/ops/blocked_by.go` (BSD license header, `package ops`, matching the header year convention of its neighbours) exporting exactly one function:

```go
// IsBlocked reports whether the entity that declares blockedBy is blocked.
//
// blockedBy is the raw blocked_by frontmatter list: plain names or [[wikilinks]].
// entities is every page of the same kind — the same directory the listing came
// from, BEFORE any status filter is applied. An empty blockedBy is unblocked.
// Otherwise the entity is blocked iff at least one named blocker cannot be
// confirmed completed: a blocker whose page is absent, unreadable, or carries no
// parseable status counts as not completed, so "cannot verify it is done" reads
// as blocked rather than as permission to start. Resolution is a single status
// read per blocker — a blocker's own blockedBy is never followed, so a dependency
// cycle terminates immediately.
func IsBlocked(blockedBy []string, entities []*domain.Page) bool
```

Rules the implementation must satisfy:

1. `len(blockedBy) == 0` → `false`.
2. Each entry is stripped of wikilink brackets exactly as `pkg/storage/base.go`'s `findFileByName` does it: `strings.TrimPrefix(name, "[[")` then `strings.TrimSuffix(name, "]]")`. Do not strip `|alias` or `#section` suffixes — the spec's contract is brackets only.
3. An entry matches an entity when the names are equal ignoring case: `strings.EqualFold(entity.Name, stripped)`. Use exact equality, NOT the substring fallback that `findFileByName` uses — the resolver already holds every page, so a fuzzy match would resolve `Alpha` against an unrelated `Alpha Two`.
4. No matching entity → the entity is blocked (`return true`).
5. Matching entity whose `entity.Status() != domain.TaskStatusCompleted` → blocked (`return true`). Compare the normalized status, never the raw string, so the `done` alias counts as completed.
6. Every entry matched and completed → `false`.
7. Entries are used verbatim and none is skipped: an empty or unresolvable entry leaves the entity blocked (fail-closed, same rule as a missing file).

Imports: `strings` and `github.com/bborbe/vault-cli/pkg/domain`. No `context`, no storage, no I/O, no error return — the resolver is pure.

## 3. Ops: emit `blocked_by` and `blocked` on list output

In `pkg/ops/list.go`:

### 3a. Extend `TaskListItem`

Add these two fields to the `TaskListItem` struct, immediately after the existing `Flag` field:

```go
	BlockedBy       []string `json:"blocked_by,omitempty"`
	Blocked         *bool    `json:"blocked,omitempty"`
```

`Blocked` MUST be a `*bool`. A plain `bool` with `omitempty` drops `"blocked": false`, which AC5 requires to be present for an item whose blockers are all completed; a plain `bool` without `omitempty` emits `"blocked": false` on every item and breaks AC3's "no key without a `blocked_by`" rule. The pointer distinguishes "no dependency list, emit nothing" (nil) from "dependency list present, not blocked" (pointer to false).

### 3b. Compute the fields in `Execute`

In `listOperation.Execute`, immediately after the existing `items[i].Goals = task.Goals()` line, add:

```go
		blockedBy := task.BlockedBy()
		items[i].BlockedBy = blockedBy
		if len(blockedBy) > 0 {
			blocked := IsBlocked(blockedBy, tasks)
			items[i].Blocked = &blocked
		}
```

`tasks` here is the **unfiltered** slice returned by `l.pageStorage.ListPages` — pass it, not `filteredTasks`. A completed blocker is excluded by the default status filter, so passing `filteredTasks` would make every dependency look absent and mark every dependent item blocked. Do NOT add an `IsBlocked` call with any other page slice.

Do NOT change `filterTasks`, `shouldIncludeTask`, `matchesStatusFilter`, `statusPriority`, the sort, or the `ListOperation` interface signature. Block state is derived output only: a blocked item is still listed.

Keep the addition to five statements — `.golangci.yml` enables `funlen` at 80 lines / 50 statements, and `Execute` is already ~71 lines, so an expanded helper inline in the loop can tip it over. If you extract a small helper instead, keep it a separate function rather than growing `Execute`.

### 3c. CLI layer

No change. `pkg/cli/cli.go` already prints whatever `ListOperation` returns via `PrintJSON` for `--output json`, for `task list` and for the generic `goal list`. Do not add `encoding/json` to any command file.

## 4. Domain tests

### 4a. `pkg/domain/task_frontmatter_test.go`

Add a `Describe("BlockedBy", …)` block next to the existing `Describe("Goals", …)` block, using the enclosing fixture (`fm`, `ctx`):

```go
	Describe("BlockedBy", func() {
		DescribeTable("reads a list and rejects scalars",
			func(stored any, expected []string) {
				fm = domain.NewTaskFrontmatter(map[string]any{"blocked_by": stored})
				Expect(fm.BlockedBy()).To(Equal(expected))
			},
			Entry("plain names", []any{"A", "B"}, []string{"A", "B"}),
			Entry("wikilink names", []any{"[[A]]", "[[B]]"}, []string{"[[A]]", "[[B]]"}),
			Entry("string slice", []string{"A"}, []string{"A"}),
			Entry("mixed types coerce to strings", []any{"A", 1}, []string{"A", "1"}),
			Entry("empty list", []any{}, []string{}),
			Entry("nil value", nil, nil),
			Entry("scalar string is malformed", "A", nil),
			Entry("scalar comma string is malformed", "A,B", nil),
		)

		It("returns an empty list for a missing key", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
			Expect(fm.BlockedBy()).To(BeEmpty())
		})

		It("reads back empty after the documented clear path", func() {
			fm = domain.NewTaskFrontmatter(map[string]any{"blocked_by": []any{"A"}})
			Expect(fm.SetField(ctx, "blocked_by", "")).To(Succeed())
			Expect(fm.BlockedBy()).To(BeEmpty())
		})
	})
```

Use those `Entry` descriptions verbatim — `<verification>` greps two of them by name. The `scalar string is malformed` and `scalar comma string is malformed` entries are load-bearing: they are the spec's Failure Mode row for a malformed `blocked_by`, and they are what distinguishes `blockedByList` from `GetStringSlice`.

Add a YAML round-trip `Describe` at the end of the file, following the `pkg/domain/task_frontmatter_metrics_test.go` pattern — this is the serialization boundary every frontmatter write crosses:

```go
var _ = Describe("TaskFrontmatter blocked_by YAML round-trip", func() {
	var fm domain.TaskFrontmatter

	BeforeEach(func() {
		fm = domain.NewTaskFrontmatter(map[string]any{
			"status":     "todo",
			"blocked_by": []any{"[[Blocker Task]]", "Plain Blocker"},
		})
	})

	It("writes blocked_by as a YAML list", func() {
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("blocked_by:"))
		Expect(string(data)).To(ContainSubstring("Blocker Task"))
		Expect(string(data)).To(ContainSubstring("Plain Blocker"))
	})

	It("round-trips blocked_by through marshal and unmarshal", func() {
		data, err := yaml.Marshal(fm.RawMap())
		Expect(err).NotTo(HaveOccurred())

		var raw map[string]any
		Expect(yaml.Unmarshal(data, &raw)).To(Succeed())
		re := domain.NewTaskFrontmatter(raw)
		Expect(re.BlockedBy()).To(Equal([]string{"[[Blocker Task]]", "Plain Blocker"}))
	})

	It("emits no blocked_by key when the field is absent", func() {
		plain := domain.NewTaskFrontmatter(map[string]any{"status": "todo"})
		data, err := yaml.Marshal(plain.RawMap())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("blocked_by"))
	})
})
```

### 4b. `pkg/domain/goal_frontmatter_test.go`

Add the same `Describe("BlockedBy", …)` block for `GoalFrontmatter`, with at least: `plain names`, `wikilink names`, `empty list`, `nil value`, `scalar string is malformed`, and an `It("returns an empty list for a missing key")`. Use `domain.NewGoalFrontmatter(...)` for the fixture.

### 4c. `pkg/domain/page_test.go`

Add a `Describe("Page BlockedBy", …)` block mirroring the existing `Describe("Page Flag", …)`: a `DescribeTable` building pages with `domain.NewPage(map[string]any{"status": "todo", "blocked_by": stored}, domain.FileMetadata{Name: "Task"}, domain.Content(""))` and asserting `page.BlockedBy()`, with entries `plain names`, `wikilink names`, `nil value`, `scalar string is malformed`.

## 5. Resolver tests: new `pkg/ops/blocked_by_test.go`

`package ops_test`, Ginkgo v2 / Gomega, no mocks. Build entities with `domain.NewPage(map[string]any{"status": …}, domain.FileMetadata{Name: …}, domain.Content(""))` and drive `ops.IsBlocked(blockedBy, entities)`.

A `DescribeTable` named so its intent is legible, with these entries (descriptions verbatim — `<verification>` greps several by name):

- `"empty list is unblocked"` — `nil` list, entities non-empty → false
- `"single completed blocker is unblocked"` — `["A"]`, A completed → false
- `"all completed blockers are unblocked"` — `["A","B"]`, both completed → false
- `"one of two blockers not completed is blocked"` — `["A","B"]`, A completed, B in_progress → true
- `"blocker with status next is blocked"`
- `"blocker with status in_progress is blocked"`
- `"blocker with status hold is blocked"`
- `"blocker with status backlog is blocked"`
- `"blocker with status aborted is blocked"`
- `"blocker with status done alias is unblocked"` — B has `status: done` → false (proves the normalized comparison)
- `"missing blocker file is blocked"` — `["Ghost"]`, no such page → true
- `"blocker without a status key is blocked"` — B has no `status` key → true
- `"blocker with an unparseable status is blocked"` — B has `status: banana` → true
- `"wikilink blocker name resolves to the plain-named file"` — `["[[A]]"]`, A completed → false
- `"wikilink blocker with a case mismatch resolves"` — `["[[alpha]]"]`, page `Alpha` completed → false
- `"plain blocker name with a case mismatch resolves"` — `["ALPHA"]`, page `Alpha` completed → false
- `"substring name does not match a longer file name"` — `["Alpha"]`, only page `Alpha Two` completed → true
- `"a completed entity blocked by itself is unblocked"` — `["A"]` on the same entity set where A is completed → false
- `"a blocked blocker is not followed transitively"` — `["B"]`, B `in_progress` and itself carrying `blocked_by: ["C"]` where C is completed → true (B's own dependency is never consulted)
- `"a cycle terminates and leaves both blocked"` — A `in_progress` blocked by `["B"]`, B `in_progress` blocked by `["A"]` → `IsBlocked` returns true for both, in one call each

Then add a dedicated `It` block for the spec's end-to-end reading of AC6: a vault-shaped fixture where one dependent task names a completed blocker in wikilink form and a case-mismatched plain name, asserting `IsBlocked` is false for the dependent and true for a second dependent whose single blocker is missing. Name it `It("resolves wikilinks and case-insensitive names against the same page set")`.

## 6. Ops emission tests in `pkg/ops/list_test.go`

Add a `Context("BlockedBy and Blocked fields in TaskListItem", …)` inside `Describe("ListOperation", …)`, following the existing `Context("Goals field in TaskListItem", …)` style (`mockPageStorage.ListPagesReturns(...)` then `listOp.Execute(ctx, "/vault", "my-vault", "Tasks", nil, true, "", "")`). Required cases:

1. Blocked task: page `Dependent` with `status: todo`, `blocked_by: []any{"A","B"}`, no A/B pages → `items[0].Blocked` is non-nil and `*items[0].Blocked` is true; `items[0].BlockedBy` equals `[]string{"A","B"}`; `json.Marshal(items[0])` contains `"blocked":true` and `"blocked_by":["A","B"]`.
2. Completed blockers: page `Dependent` with `blocked_by: []any{"A"}` plus page `A` with `status: completed` → `items[0].Blocked` is non-nil, `*items[0].Blocked` is false, and `json.Marshal(items[0])` contains `"blocked":false` — the key must be PRESENT with value false.
3. No dependency list: page with only `status: todo` → `BlockedBy` nil, `Blocked` nil, and `json.Marshal(item)` contains neither `"blocked_by"` nor `"blocked"`.
4. Explicit empty list: page with `blocked_by: []any{}` → `BlockedBy` nil-or-empty, `Blocked` nil, JSON contains neither key.
5. Malformed scalar: page with `blocked_by: "A"` → `Blocked` nil, JSON contains neither key.
6. Unfiltered page set: page `Dependent` (`status: todo`, `blocked_by: []any{"A"}`) plus page `A` (`status: completed`), called with `showAll=false` and default status filters → the returned items contain `Dependent` only (A is filtered out of the listing) AND `*items[0].Blocked` is false. This is the case that fails if `filteredTasks` is passed to `IsBlocked` instead of `tasks`.
7. Blocked items are still listed: page `Dependent` (`status: todo`, `blocked_by: []any{"Ghost"}`) → one item returned with `*items[0].Blocked` true.

Assert the exact JSON key names, not only the Go field values — the JSON tag is the contract.

## 7. Integration test in `integration/cli_test.go`

Add a top-level `Describe("vault-cli blocked_by JSON surface", …)` using `createTempVault` / `createTempVaultWithGoals` and the existing `gexec.Start(exec.Command(binPath, "--config", configPath, "--vault", "test", …))` pattern. This is the real production path for AC1, AC2, AC3, AC5 and the end-to-end half of AC6. Required specs:

1. **AC1 — task list emits the raw list and the computed flag.** Fixture task `dep-a` with `status: todo` and `blocked_by:\n  - "A"\n  - "B"` and no A/B files. Run `task list --output json`, decode into `[]map[string]any`, find the `dep-a` item, assert `item["blocked_by"]` equals `[]any{"A","B"}` (the decoded JSON shape — an `Equal([]string{...})` comparison fails against `[]any`) and `item["blocked"]` equals `true`.
2. **AC2 — goal list emits both fields.** Fixture goal `dep-goal` with `status: next` and a `blocked_by` list naming an absent goal, via `createTempVaultWithGoals`. Run `goal list --output json` and assert the same two fields on the item (`blocked_by` as `[]any{...}`, `blocked` as `true`).
3. **AC3 — no dependency list means no new keys.** Fixture with a single task carrying only `status: todo`. Run `task list --output json`, decode into `[]map[string]any`, and assert the item has exactly the four keys a bare on-disk task emits today (`name`, `status`, `vault`, `modified_date`) — `Expect(item).To(HaveLen(4))` plus `HaveKey` for each. `modified_date` is part of the pre-change key set, not a new key: `ListOperation.Execute` fills it from the page's file mtime, which the storage layer populates for every on-disk page. If the key set is anything other than those four, the change added or removed a key — fix the emission, not the test. Also assert the raw stdout contains no `blocked` substring at all: `Expect(string(session.Out.Contents())).NotTo(ContainSubstring("blocked"))`.
4. **AC5 — the blocked → unblocked transition.** Fixture task `dependent` with `status: todo` and `blocked_by:\n  - "Blocker"`, no `Blocker` file → `blocked` is `true`. Then write `Blocker.md` into the vault's `Tasks` directory with frontmatter `status: completed` (the harness returns `vaultPath`; use `os.WriteFile(filepath.Join(vaultPath, "Tasks", "Blocker.md"), …)`) and run the same command again → the decoded item has key `blocked` present with value `false`. Assert presence explicitly (`HaveKey("blocked")` plus `HaveKeyWithValue("blocked", false)`), not just the value — an omitted key would otherwise pass a naive check.
5. **AC6 end-to-end — wikilink and case-insensitive resolution.** Fixture task `dependent` with `status: todo` and `blocked_by:\n  - "[[blocker-task]]"`, plus a file `Blocker-Task.md` with `status: completed` → decoded `blocked` is `false`. This proves bracket stripping and case-insensitive matching through the real binary, not just in the resolver unit test.
6. **Malformed field is inert.** Fixture task `scalar-dep` with `status: todo` and a scalar `blocked_by: A` → decoded item has neither `blocked` nor `blocked_by` keys.

Keep the YAML fixtures as Go raw string literals in the existing style of `Describe("vault-cli task JSON schema", …)`.

## 8. Changelog

Read the top of `CHANGELOG.md` first. At authoring time there is **no** `## Unreleased` section — verify with `grep -n '^## ' CHANGELOG.md | head -3`. If it is still absent, create it directly **below** the preamble block (the `All notable changes…` line and the `* MAJOR / MINOR / PATCH` lines) and **above** the newest `## vX.Y.Z` section. If it already exists, append to it — never create a second one. Add:

```
- feat: `blocked_by` frontmatter on tasks and goals is now a typed dependency list — `vault-cli task list --output json` and `vault-cli goal list --output json` emit the raw `blocked_by` array plus a computed `blocked` boolean (blocked iff any named blocker is not `completed`; a missing, unreadable or status-less blocker counts as blocked). Additive only: an entity without `blocked_by` emits neither key and every existing field is unchanged.
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — the release agent owns those. Do NOT create a git tag.

</requirements>

<constraints>
- `blocked_by` frontmatter key is a YAML list of strings (plain names or `[[wikilinks]]`); both forms parse to the same list.
- Additive-only JSON change: existing task/goal output fields keep their names and values; no field is removed or re-typed.
- No changes to `status` / `phase` transitions, `task set`, `task defer`, or `task complete` semantics. In particular no `blocked_by` case is added to `SetField` / `GetField`.
- Follow vault-cli conventions (`docs/development-patterns.md`): accessors read through `FrontmatterMap`, Ginkgo/Gomega tests, `pkg/ops` returns structured results — no `fmt.Print*` and no `os.Stdout` in `pkg/ops/`.
- No cross-kind blockers: a task's blockers resolve in the tasks directory, a goal's in the goals directory. Same-kind only.
- No auto-setting `status: hold` from `blocked_by` — block state stays derived and orthogonal to status.
- No migration of existing vault tasks/goals and no bulk rewrite of any file under a vault directory.
- No new vault-cli subcommand or flag — blocked state is a computed list-output field.
- No `time.Duration`, no new config key, no environment variable, no opt-out flag, no Prometheus metric, no caching layer.
- Per `docs/development-patterns.md`: operations inject the storage interface (no direct file I/O in ops), counterfeiter mocks live in `mocks/`, and command files never import `encoding/json`.
- Existing tests must still pass, unmodified.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
</constraints>

<verification>
Run everything from the repository root.

**1. Full gate first pass:**

```
make precommit
```

Must exit 0. `make precommit` runs `ensure format generate test check addlicense`. If a step fails, fix it and re-run the failing target, then run `make precommit` one final time.

**2. The accessors, the resolver and the emission point landed** — each command must print at least one line:

```
grep -n 'func (f TaskFrontmatter) BlockedBy' pkg/domain/task_frontmatter.go
grep -n 'func (f GoalFrontmatter) BlockedBy' pkg/domain/goal_frontmatter.go
grep -n 'func (p Page) BlockedBy' pkg/domain/page.go
grep -n 'func blockedByList' pkg/domain/frontmatter_map.go
grep -n 'func IsBlocked' pkg/ops/blocked_by.go
```

**3. The JSON tags are exactly right** — this command must print exactly `2`:

```
grep -c 'json:"blocked' pkg/ops/list.go
```

And this must print at least one line, showing the resolver receives the unfiltered page set:

```
grep -n 'IsBlocked(blockedBy, tasks)' pkg/ops/list.go
```

**4. The unfiltered page set is used** — absence check (must print nothing and the shell must report success):

```
! grep -q 'IsBlocked(blockedBy, filteredTasks)' pkg/ops/list.go
```

**5. Named test entries are present** — each command must print a number `>= 1` (the `-ginkgo.v` flag is required, otherwise entry descriptions print only on failure):

```
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "scalar comma string is malformed"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "round-trips blocked_by through marshal and unmarshal"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "a cycle terminates and leaves both blocked"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "substring name does not match a longer file name"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "blocker with status done alias is unblocked"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "BlockedBy and Blocked fields in TaskListItem"
```

**6. The end-to-end JSON surface is proven against the real binary:**

```
go test -count=1 ./integration/... -v -ginkgo.v 2>&1 | grep -c "vault-cli blocked_by JSON surface"
```

Must print `>= 1`.

**7. The resolver is fully covered:**

```
go test -coverprofile=/tmp/cover-blocked.out ./pkg/ops/... && go tool cover -func=/tmp/cover-blocked.out | grep 'IsBlocked'
```

`IsBlocked` must report `100.0%`. If it reports below 100.0%, add the missing table entry in `pkg/ops/blocked_by_test.go` — do not add unrelated retroactive coverage.

**8. Changelog:**

```
grep -n '^## ' CHANGELOG.md | head -1
grep -c 'blocked_by' CHANGELOG.md
```

The first must print `## Unreleased`; if it prints a version heading instead, the bullet was placed between released sections — move it into the existing `## Unreleased` section. The second must print a number `>= 1`.

Before you finish, re-run the commands above and confirm each passes, then walk spec 046's Acceptance Criteria 1, 2, 3, 5 and 6 against the change and state in your final message which requirement satisfies each one.
</verification>
