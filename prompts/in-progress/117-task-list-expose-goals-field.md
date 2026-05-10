---
status: executing
container: vault-cli-117-task-list-expose-goals-field
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-11T00:00:00Z"
queued: "2026-05-10T22:15:37Z"
started: "2026-05-10T22:15:38Z"
---

<summary>
- `vault-cli task list --output json` currently omits the `goals:` frontmatter array entirely
- Downstream consumers (e.g. task-orchestrator's `?goal=` filter) cannot match tasks to goals without re-parsing the markdown
- After this change, every task object emitted by `task list --output json` includes a `goals` array when the source frontmatter has one
- Goals are emitted verbatim — `[[Goal Name]]` brackets preserved; bracket-strip is the consumer's responsibility
- Tasks with no `goals:` frontmatter (or an empty list) emit no `goals` field at all (JSON `omitempty`)
- All existing JSON consumers continue to work — additive field, no removals or renames
- Single-concern change: scoped to `task list`, not extended to `task show` or any other command
</summary>

<objective>
Add a `Goals []string` field to `pkg/ops.TaskListItem` and populate it from each task's parsed frontmatter inside `listOperation.Execute`, so `vault-cli task list --output json` exposes the `goals` array for downstream filtering. Verbatim emission with brackets preserved. Use `omitempty` to keep the JSON output backwards-compatible for tasks without goals.
</objective>

<context>
Read CLAUDE.md for project conventions and the dark-factory workflow notes (no manual git, `make precommit` is the gate).

Read these files in full before making changes:

- `pkg/ops/list.go` — defines `TaskListItem` (struct currently has 14 fields) and `listOperation.Execute` (constructs each `TaskListItem` from a `*domain.Task`). The accessor for goals on the task is already wired: `task.Goals()` is used at line ~158 inside `shouldIncludeTask`.
- `pkg/domain/task_frontmatter.go` — confirm the existing `Goals()` accessor: `func (f TaskFrontmatter) Goals() []string { return f.GetStringSlice("goals") }`. No domain or parser changes are needed; the goals slice is already surfaced through `task.Goals()`.
- `pkg/domain/frontmatter_map.go` (or wherever `GetStringSlice` is defined) — `GetStringSlice` returns `nil` for a missing key. Confirmed by `pkg/domain/frontmatter_map_test.go` ("returns nil for missing key" case). This means `task.Goals()` returns `nil` for tasks with no `goals:` frontmatter, which is exactly what `json:",omitempty"` needs to drop the field.
- `pkg/ops/list_test.go` — Ginkgo/Gomega suite. New test cases go inside the existing `Describe("ListOperation", ...)` block. The pattern for constructing tasks with goals already exists (search for `taskWithGoal.SetGoals([]string{...})` around line 216 — the `--goal filter` context). Use the same pattern.
- `pkg/domain/task_frontmatter_test.go` (~line 124) — confirms `SetGoals` is the canonical helper for building test tasks with a goals array.
- `CHANGELOG.md` — entries are organized under released `## vX.Y.Z` headings. Recent prompts (e.g. `prompts/completed/109-list-tolerates-missing-pages-dir.md`) created or appended to a `## Unreleased` block at the top; follow that convention. The release pipeline (dark-factory `autoRelease: true`) handles version bumps separately — do NOT bump version strings in this prompt.

Reference docs from the coding plugin:
- `~/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions
- `~/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format
- `~/.claude/plugins/marketplaces/coding/docs/test-pyramid-triggers.md` — when to add integration vs unit tests
- `~/Documents/workspaces/vault-cli/docs/development-patterns.md` — `pkg/ops` is a library layer; operations return structured results. Do NOT add stdout printing.

Background (do NOT inline in code or tests, kept here for reviewer context only): the immediate consumer is task-orchestrator's `GET /tasks?goal=...` filter (its spec 006). It receives the goals from `vault-cli task list --output json`, currently sees `null`, and excludes everything → empty board. This prompt is the minimal additive fix to unblock that filter today. A separate spec will redesign `task list` to expose the full raw frontmatter map; that is explicitly out of scope here.
</context>

<requirements>
### 1. Add the `Goals` field to `TaskListItem` in `pkg/ops/list.go`

Locate the `TaskListItem` struct (currently the 14 fields starting with `Name string` and ending with `CompletedDate string`). Add ONE new field at the end of the struct, after `CompletedDate`:

```go
Goals []string `json:"goals,omitempty"`
```

Use the exact tag `json:"goals,omitempty"` — lowercase `goals`, with `omitempty` so `nil` or empty slices are dropped from JSON output.

Do NOT change any other field. Do NOT reorder. Do NOT add `Title`, `BlockedBy`, `Description`, or any other field — single-concern change.

### 2. Populate `Goals` inside `listOperation.Execute` in `pkg/ops/list.go`

Locate the construction loop (the `for i, task := range filteredTasks { items[i] = TaskListItem{...} ... }` block). After the existing assignments to `items[i].DeferDate`, `items[i].PlannedDate`, `items[i].DueDate`, `items[i].ModifiedDate`, `items[i].CompletedDate`, add ONE new assignment:

```go
items[i].Goals = task.Goals()
```

`task.Goals()` is already imported transitively via `task *domain.Task`. It returns `[]string` directly from the YAML frontmatter via `GetStringSlice("goals")`, which returns `nil` for missing keys — that's the desired `omitempty` behavior, no nil-check needed.

Do NOT strip `[[ ]]` brackets. Do NOT trim whitespace. Do NOT deduplicate. Emit verbatim — bracket-strip is the consumer's responsibility per task-orchestrator spec 006.

Do NOT change the `Execute` signature. Do NOT change the `ListOperation` interface. Do NOT touch the filter path (`shouldIncludeTask`, `taskHasGoal`).

### 3. Add Ginkgo test cases in `pkg/ops/list_test.go`

Add a new `Context("Goals field in TaskListItem", ...)` block **inside the existing `Describe("ListOperation JSON output", ...)` block at `pkg/ops/list_test.go:369`** (NOT the line-20 `Describe("ListOperation", ...)`). The JSON-output Describe already declares `var items []ops.TaskListItem` and already calls `items, execErr = listOp.Execute(...)` per-Context — exactly the scaffold these test cases need. Mirror the existing per-Context pattern: a local `BeforeEach` builds tasks via `mockPageStorage.ListPagesReturns(...)` and the existing per-Describe `JustBeforeEach` runs `Execute`. **Do NOT modify the line-20 Describe at all** — that would require widening its `JustBeforeEach` from `_, err = ...` to `items, err = ...` and inflate the diff across every existing test in the suite.

Add the following `It(...)` cases inside a new `Context("Goals field in TaskListItem", ...)` block:

a. **Goals from frontmatter are emitted verbatim** — build a single task via:
```go
taskWithGoals := domain.NewTask(
    map[string]any{"status": "todo"},
    domain.FileMetadata{Name: "Task With Goals"},
    domain.Content(""),
)
taskWithGoals.SetGoals([]string{"[[Goal A]]", "[[Goal B]]"})
mockPageStorage.ListPagesReturns([]*domain.Task{taskWithGoals}, nil)
```
Assert: `Expect(err).To(BeNil())`, `Expect(items).To(HaveLen(1))`, `Expect(items[0].Goals).To(Equal([]string{"[[Goal A]]", "[[Goal B]]"}))`. The brackets MUST be preserved character-for-character — no stripping, no trimming.

b. **Missing `goals:` frontmatter yields nil Goals** — build a task with no goals at all:
```go
taskNoGoals := domain.NewTask(
    map[string]any{"status": "todo"},
    domain.FileMetadata{Name: "Task No Goals"},
    domain.Content(""),
)
mockPageStorage.ListPagesReturns([]*domain.Task{taskNoGoals}, nil)
```
Assert: `Expect(items[0].Goals).To(BeNil())`. This is what produces `omitempty` dropping the key from JSON output.

c. **Empty `goals: []` frontmatter yields nil or empty Goals (document the behavior)** — build a task whose frontmatter explicitly has an empty slice:
```go
taskEmptyGoals := domain.NewTask(
    map[string]any{"status": "todo", "goals": []any{}},
    domain.FileMetadata{Name: "Task Empty Goals"},
    domain.Content(""),
)
mockPageStorage.ListPagesReturns([]*domain.Task{taskEmptyGoals}, nil)
```
First, observe what `task.Goals()` returns for this input by reading `pkg/domain/frontmatter_map.go`'s `GetStringSlice` implementation. Assert whichever is correct — either `Expect(items[0].Goals).To(BeNil())` OR `Expect(items[0].Goals).To(BeEmpty())`. Pick the one matching the actual implementation. Add a brief comment in the test explaining which case applies and why (e.g. `// GetStringSlice returns nil for empty slices because ...` or `// preserves the empty slice; omitempty still drops it from JSON`).

d. **JSON marshalling round-trip respects `omitempty`** — this is the boundary test (per the coding plugin's `test-pyramid-triggers.md`: a new struct tag is a serialization boundary). Marshal a `TaskListItem` with no Goals set and assert the JSON output does NOT contain the substring `"goals"`:
```go
item := ops.TaskListItem{Name: "X", Status: "todo", Vault: "v"}
data, err := json.Marshal(item)
Expect(err).To(BeNil())
Expect(string(data)).NotTo(ContainSubstring(`"goals"`))
```
Then marshal a `TaskListItem` WITH Goals set to `[]string{"[[Goal A]]"}` and assert the JSON DOES contain `"goals":["[[Goal A]]"]`. Add `"encoding/json"` to the test file imports if not already present.

Do NOT delete or modify any existing test case. Do NOT change the assertions in the existing `Context("with --goal filter", ...)` block — that block tests filtering, not field emission.

### 4. CHANGELOG entry

Open `CHANGELOG.md`. Recent prompts have placed new entries under a `## Unreleased` heading at the top of the file (above the topmost released version, e.g. above `## v0.61.0`). If `## Unreleased` already exists, append to it. If not, create it.

Add this entry exactly:

```markdown
- feat: Expose `goals` frontmatter array in `task list --output json` — enables consumers to filter tasks by goal without re-parsing the markdown source. Verbatim emission (brackets preserved); consumer strips `[[ ]]` if needed.
```

Do NOT bump any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json`. The dark-factory daemon's `autoRelease` pipeline handles release versioning separately; precommit no longer enforces version alignment per `.dark-factory.yaml` and the v0.60.0 changelog note ("Drop check-versions from `make precommit`").
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT strip `[[ ]]` brackets from goal names — emit verbatim from `task.Goals()`. Bracket-strip is consumer-side (task-orchestrator spec 006).
- Do NOT modify any other field in `TaskListItem` (`Name`, `Status`, `Assignee`, `Priority`, `Vault`, `Category`, `Recurring`, `DeferDate`, `PlannedDate`, `DueDate`, `ClaudeSessionID`, `Phase`, `ModifiedDate`, `CompletedDate`)
- Do NOT change the CLI flag surface or `task list` argument parsing
- Do NOT extend the change to `task show`, `goal list`, or any other command — single concern
- Do NOT add `Title`, `BlockedBy`, `Description`, or any other absent field — separate prompts handle those
- Do NOT change the `ListOperation` interface or the `Execute` method signature
- Do NOT modify the `shouldIncludeTask` or `taskHasGoal` filter logic — they already use `task.Goals()` and are unaffected
- Do NOT introduce a new package or rename existing files
- Do NOT add stdout writes from `pkg/ops` — operations return structured results, the CLI layer owns formatting (per `docs/development-patterns.md`)
- Existing tests must remain green without modification — the line-20 `Describe("ListOperation", ...)` block and its `JustBeforeEach` MUST NOT be touched (the line-369 `Describe("ListOperation JSON output", ...)` already has the right scaffold; reuse it)
- Use the `bborbe/errors` wrapping convention if any new error site is introduced (none expected — this change has no new error paths)
- Follow Ginkgo v2 / Gomega style (`Describe`, `Context`, `It`, `Expect(...).To(...)`) per `~/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- Do NOT bump versions in `CHANGELOG.md`, `.claude-plugin/plugin.json`, or `.claude-plugin/marketplace.json` — release pipeline handles it
</constraints>

<verification>
Run `make precommit` — must exit 0. This runs lint, format, generate, vet, and the full test suite.

Run `make test` independently to confirm the new Ginkgo cases pass:
```bash
make test
```

Manual smoke test (after the daemon ships a new binary; not required to pass during prompt execution since `make install` is gated by the release procedure):
```bash
vault-cli task list --vault personal --output json | jq '.[] | select(.goals != null) | {name, goals}'
```
Expect: at least one task object whose `goals` array contains values like `"[[Eliminate Agent Task Rot]]"` (verbatim, with brackets).

Verify backwards compatibility: tasks without `goals:` frontmatter must produce JSON objects that do NOT contain a `"goals"` key:
```bash
vault-cli task list --vault personal --output json | jq '.[0] | keys'
```
Output keys must be a subset of: `name, status, assignee, priority, vault, category, recurring, defer_date, planned_date, due_date, claude_session_id, phase, modified_date, completed_date, goals` — and `goals` appears only when the source has it.
</verification>
