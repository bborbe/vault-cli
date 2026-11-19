---
status: completed
spec: [069-bug-goal-topic-closure-gate]
summary: Added a shared pkg/ops closure gate, wired it into goal/topic complete with open_items reporting and terminal no-ops, and covered it with unit and integration tests
execution_id: vault-cli-closure-gate-exec-284-spec-069-goal-topic-closure-gate
dark-factory-version: v0.198.17
created: "2026-11-18T22:55:00Z"
queued: "2026-11-18T23:06:57Z"
started: "2026-11-18T23:06:58Z"
completed: "2026-11-18T23:19:37Z"
---

# Gate `goal complete` and `topic complete` on their own closure evidence

<summary>
- `vault-cli goal complete` refuses to close a goal while any `# Success Criteria` or `# Definition of Done` box is open, and names every open box.
- `vault-cli topic complete` refuses to close a topic while any `# Success Criteria` or `# Completion Gate` box is open, or while any goal listed in the topic's Goals section is not `completed`/`aborted`.
- An open task linked to a goal as `[[Some Goal]]` now blocks the close exactly as the bare name `Some Goal` already did.
- A refusal exits 1, prints one line per open item on stdout, and reports the same items as `open_items` in `--output json`.
- A page that is already `completed` or `aborted` is a no-op: exit 0, no write, file digest unchanged.
- Goals created before 2026-06-26 with no `# Definition of Done` section still close, but only when that section is genuinely absent — an empty or oddly-spelled DoD heading refuses.
- A checkbox inside a fenced code block is an example, never a box; an unterminated fence refuses the close.
- Both commands share one implementation, so the rule cannot drift between them.
- `task complete`, `objective complete`, and the existing `--force` semantics are unchanged.
</summary>

<objective>
Implement the vault's closure rule in one shared `pkg/ops` check and wire it into `goal complete` and `topic complete`, so a goal or topic can only close on its own recorded evidence. This is spec 069, deliberately one prompt: the spec's § Suggested Decomposition fixes a single prompt because a multi-prompt split cannot be sorted into dependency order by dark-factory's unprefixed-filename numbering.
</objective>

<context>
Read `CLAUDE.md` and `docs/dod.md` for project conventions and the Definition of Done.

Read these files fully before changing anything:

- `specs/in-progress/069-bug-goal-topic-closure-gate.md` — the whole spec. Desired Behavior 1-12, Constraints, Failure Modes, and the last Acceptance Criterion (the table-test case list) are all binding.
- `pkg/ops/complete.go` — `MutationResult` (line 66) is where the new `OpenItems` field belongs. `checkSubtaskCompletion` (line 234) is the existing whole-page box gate for tasks; it is the model, not a target. `countCheckboxStates` uses `storage.CheckboxRegex`, which accepts only `-`/`*` bullets and only ` `/`x`/`/` markers — it is **not** reusable here (see requirement 1.5).
- `pkg/ops/goal_complete.go` — the whole file. `Execute` currently refuses an already-`completed` goal (lines 73-76), then runs `checkOpenTasks` unless `force` (78-82), then writes. `taskLinkedToGoal` (185-192) compares `g == goalName` over raw `goals:` entries. `joinTaskNames` (194-200) formats `name (status)`.
- `pkg/ops/topic_complete.go` — the whole file. `Execute` (41-66) finds, refuses an already-`completed` topic (52-55), sets `status`, writes. No gate, no `--force`, no goal storage dependency.
- `pkg/ops/goal_complete_test.go` — the `Context("goal already completed")` block (lines 88-101) must be **inverted, not deleted**. Every other context that expects a successful close needs a closure-complete fixture body (requirement 6.3).
- `pkg/ops/topic_complete_test.go` — the `Context("topic already completed")` block (lines 121-144) must be **inverted, not deleted**. The shared `BeforeEach` fixture body (line 42) and the two alternate fixtures (lines 90, 108) need closure-complete bodies (requirement 6.4).
- `pkg/ops/export_test.go` — the established mechanism for exposing unexported `pkg/ops` symbols to the external `ops_test` package. Add the closure-gate aliases here (requirement 6.1).
- `pkg/ops/ops_suite_test.go` — `TestSuite`; Ginkgo v2/Gomega, external test package. `DescribeTable` is already used in `pkg/ops/escalation_test.go`, `task_approve_test.go`, `lint_test.go`.
- `pkg/domain/goal.go` / `pkg/domain/topic.go` / `pkg/domain/task_status.go` — `Goal` and `Topic` embed their frontmatter and carry `Content domain.Content` (the full raw markdown file, frontmatter included). Status constants: `domain.GoalStatusCompleted`, `domain.GoalStatusAborted`, `domain.TaskStatusNext`, `domain.TaskStatusTodo`, `domain.TaskStatusInProgress`, `domain.TopicStatusCompleted`.
- `pkg/domain/frontmatter_map.go` — `GetString(key)` / `Get(key)` on the embedded `FrontmatterMap`; `GetField("created")` on both `GoalFrontmatter` and `TopicFrontmatter` falls through to `GetString`.
- `pkg/storage/base.go` — `storage.ParseFrontmatterMap(ctx, content []byte) (map[string]any, error)` is exported. Its `frontmatterRegex` is `(?s)^---\n(.*?)\n---\n(.*)$`: it requires a leading `---\n` with **no BOM and no CRLF**, which is exactly why the gate normalises the document first. `storage.CheckboxRegex` is `^(\s*)[-*] \[([ x/])\] (.+)$` — do not reuse it (requirement 1.5).
- `pkg/storage/errors.go` — `storage.ErrNotFound`; test with `errors.Is` from `github.com/bborbe/errors`.
- `scripts/update-topic-goal-entries.awk` — the shipped, tested level-relative `Goals` reader. Port its three properties: level-relative bound, `^- \[\[<Name>\]\]` declaration only, and no silent skip.
- `agents/goal-manager-agent.md` line 152 — the in-repo closure-box table by kind: goal = `# Success Criteria` + `# Definition of Done`; topic = `# Success Criteria` + `# Completion Gate`. The section sets in this prompt must agree with it.
- `agents/topic-manager-agent.md` line 67 — the level-relative `## Goals` bound statement: `## Goals` runs past `###` sub-headings and ends at the next `#` or `##`; `# Goals` ends at the next `#`.
- `pkg/cli/cli.go` — `runMutation` (line 115) prints the `MutationResult` as JSON when `--output json` and otherwise returns the error; `Execute` (line 3301) prints `Error: <err>` to stderr and exits 1. `createGoalCompleteCommand` (1471-1532) and `createTopicCompleteCommand` (1797-1835) are the two call sites; `NewGoalCompleteOperation` is constructed at 1501, `NewTopicCompleteOperation` at 1822.
- `pkg/cli/output.go` — `OutputFormat`, `PrintJSON`.
- `integration/cli_test.go` — `TestIntegration` entry point is in `integration/integration_suite_test.go`; `binPath` is built by `gexec.Build`. Helpers: `createTempVaultWithGoals(tasks, goals)` (line 47) and `createTempVaultWithTopicPages(topicsDir, topics, goals)` (line 188). Existing complete cases that will break without a fixture update: the `Describe("vault-cli goal close-out gating")` fixture (line ~2085), the `Describe("vault-cli goal terminal phase")` fixture (line ~2197), and `It("topic complete moves the status to completed and refuses a second complete")` (line 6161). `sha256OfFile` is declared as a local closure in several Describe blocks — declare your own in each new block.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, external `_test` packages, `DescribeTable`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors`, never `fmt.Errorf`, never `context.Background()`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — exported doc comments, Interface → Constructor → Struct.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80 lines / 50 statements, nestif 4, gocognit 20, maintidx 20, dupl.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — coverage and verification.
</context>

<requirements>
## 1. New file `pkg/ops/closure_gate.go`

BSD license header `// Copyright (c) 2026 Benjamin Borbe All rights reserved.` and the standard two following lines, `package ops`. Imports: `context`, `fmt`, `regexp`, `strings`, `github.com/bborbe/errors`, `github.com/bborbe/vault-cli/pkg/domain`, `github.com/bborbe/vault-cli/pkg/storage`. It must not import `pkg/cli`, must not write to stdout/stderr, and must never write, create or delete a file.

### 1.1 Types and constants

```go
// closureKind selects the section set a closure gate reads.
type closureKind string

const (
	// closureKindGoal gates a goal on Success Criteria + Definition of Done.
	closureKindGoal closureKind = "goal"
	// closureKindTopic gates a topic on Success Criteria + Completion Gate.
	closureKindTopic closureKind = "topic"
)

// closureOutcome is the verdict of a closure gate.
type closureOutcome string

const (
	// closureOutcomeOpen means the page is measurable and at least one box or member is open.
	closureOutcomeOpen closureOutcome = "OPEN"
	// closureOutcomeUnmeasurable means the page's evidence cannot be read at all.
	closureOutcomeUnmeasurable closureOutcome = "UNMEASURABLE"
	// closureOutcomeClose means every box is ticked and every member is terminal.
	closureOutcomeClose closureOutcome = "CLOSE"
	// closureOutcomeTerminal means the page is already completed or aborted.
	closureOutcomeTerminal closureOutcome = "TERMINAL"
)

// closureMember is a linked task (goal kind) or a member goal (topic kind).
// Fields are exported so the external ops_test package can build values.
type closureMember struct {
	Name   string
	Status string
}

// closureVerdict is the result of checkClosureGate.
type closureVerdict struct {
	Outcome   closureOutcome
	OpenItems []string
}

// dodRequiredAsOf is the grandfathering cutoff. It is the fourth copy of the
// value: claude-supervisor/scripts/goal-closure-read.py, agents/goal-auditor.md
// and agents/task-auditor.md carry the other three. They move together.
const dodRequiredAsOf = "2026-06-26"
```

### 1.2 The shared check

```go
// checkClosureGate reads the page's own closure evidence and returns a verdict.
// It is the single implementation behind both `goal complete` and
// `topic complete`; neither verb may carry a second copy of the rule.
//
// The section set is fixed by kind, never guessed from the page:
// goal -> "# Success Criteria" + "# Definition of Done";
// topic -> "# Success Criteria" + "# Completion Gate".
//
// The check is read-only. A page already at completed/aborted returns TERMINAL
// with no items. On any refusal OpenItems holds one human-readable string per
// blocking item, in document order (sections first, then members).
func checkClosureGate(
	ctx context.Context,
	kind closureKind,
	content string,
	members []closureMember,
) (closureVerdict, error)
```

Evaluation order, exactly:

1. `doc := normaliseDocument(content)`.
2. `fields, err := storage.ParseFrontmatterMap(ctx, []byte(doc))`; on error return `closureVerdict{}` and `errors.Wrap(ctx, err, "parse frontmatter")`. A page whose frontmatter does not parse is never grandfathered.
3. Read `status` from `fields` with a `string` type switch (a non-string value reads as `""`). If it is `completed` or `aborted`, return `{Outcome: closureOutcomeTerminal}` immediately, with no items and no section scan.
4. `body, err := stripFrontmatter(ctx, doc)`; on error return `closureVerdict{}` and the error.
5. `body, fenceOK := stripFencedBlocks(body)`. If `!fenceOK`, return `{Outcome: closureOutcomeUnmeasurable, OpenItems: []string{"unterminated code fence"}}` — the fence makes every later section unreadable, so nothing else is evaluated.
6. `created` is read from `fields` the same way as `status`.
7. Walk the kind's section set in the fixed order `["# Success Criteria", "# Definition of Done"]` for goals and `["# Success Criteria", "# Completion Gate"]` for topics:
   - `lines, found := findSection(body, heading)`.
   - **Absent:** add the item `"<heading>: section absent"`, except the goal-only grandfathering case in 1.4.
   - **Present:** run the section's own check (1.5 / 1.6).
8. Walk `members`. For each member the kind considers open, append `"<Name> (<Status>)"`; when `Status == ""` append just `"<Name>"`. Goal members are open when the status is one of `next`, `todo`, `in_progress` (unchanged from today); every other goal-member status is terminal. Topic members are open when the status is neither `completed` nor `aborted`. Empty member statuses are always open.
9. Outcome: if any item came from an unmeasurable condition (absent section, section with zero checkboxes, strict Completion Gate violation, unterminated fence) return `closureOutcomeUnmeasurable`; else if any item is an open box or open member return `closureOutcomeOpen`; else return `closureOutcomeClose`. `UNMEASURABLE` outranks `OPEN`. The returned `OpenItems` is the full list either way, in the order produced above.

### 1.3 Normalisation and section location

- `normaliseDocument(content string) string`: strip one leading UTF-8 BOM (the three-byte sequence written `"\ufeff"`), replace `"\r\n"` with `"\n"`, then replace any remaining `"\r"` with `"\n"`. This is what makes `frontmatterRegex`'s `^---\n` match a CRLF or BOM page.
- `stripFrontmatter(ctx context.Context, doc string) (string, error)`: `doc` is already normalised and its frontmatter already parsed, so it starts with `"---\n"`. Return the body after the closing `"\n---\n"`. When the closing delimiter is missing, return `errors.Errorf(ctx, "frontmatter block not found")` — never `fmt.Errorf`. Callers only reach this after a successful parse, so this is defensive.
- `stripFencedBlocks(body string) (string, bool)`: walk lines; outside a fence, a line whose left-trimmed form starts with three backticks or three tildes opens a fence (remember which character) and is dropped; inside a fence, a line whose left-trimmed form starts with three of the opening character closes it and is dropped; every other in-fence line is dropped. Return the remaining lines joined with `"\n"`, and `false` when the document ends inside a fence. A checkbox inside a fence is never a box.
- `findSection(body, heading string) ([]string, bool)`: `heading` is `"# Success Criteria"`, `"# Definition of Done"` or `"# Completion Gate"` — level 1 and exact text after trimming trailing spaces. The section body runs from the line after the heading to the next level-1 heading (exclusive) or end of document. Subsections (`##`, `###`) are part of the body and their boxes count.
- `headingLevel(line string) int`: 1-6 from a leading `#` run followed by a space or tab, else 0. Define the regex locally; `storage.parseMarkdownHeading` is unexported and must not be reached for.
- `hasDoDShapedHeading(body string) bool`: true when **any** line, after trimming leading whitespace, leading `#` characters and a leading `-`/`*`/`+` list marker (plus following whitespace), equals `Definition of Done`. This catches `# Definition of Done`, `## Definition of Done`, a setext heading's text line, and `- Definition of Done`. The grandfathering exemption is skipped entirely when this is true.

### 1.4 Grandfathering (goals only, absence only)

Only when `kind == closureKindGoal` and the `# Definition of Done` section is absent:

- Skip the exemption when `hasDoDShapedHeading(body)` is true — add `"# Definition of Done: section absent"` and treat it as unmeasurable.
- `created == ""` → grandfathered.
- `created` matching `^\d{4}-\d{2}-\d{2}$` and lexicographically earlier than `dodRequiredAsOf` → grandfathered.
- Any other `created` (non-ISO, e.g. `15.01.2026`, `2026`) → not grandfathered; add `"# Definition of Done: section absent (created \"<value>\" is not an ISO date)"` and treat it as unmeasurable.
- `created` on or after the cutoff → not grandfathered; add `"# Definition of Done: section absent (created <value> is not before <cutoff>)"`.
- A **present** DoD heading with zero boxes is never grandfathered: it is unmeasurable, always.
- Grandfathering never applies to `topic`.

### 1.5 Checkbox rules

Define package-level compiled regexes (package-level vars are the pattern `pkg/storage/base.go` already uses):

```go
// closureCheckboxRegex matches a checkbox item with any bullet marker and any
// single-character marker. Capture 1 is the marker, 2 the item text.
var closureCheckboxRegex = regexp.MustCompile(`^[ \t]*[-*+][ \t]+\[(.)\][ \t]*(.*)$`)

// closureListItemRegex matches any markdown list item, checkbox or not.
var closureListItemRegex = regexp.MustCompile(`^[ \t]*([-*+]|\d+\.)[ \t]+`)
```

- `sectionCheckboxItems(lines []string) (open []string, total int)`: for each line matching `closureCheckboxRegex`, `total++`; the marker `x` or `X` is done, **every** other marker (` `, `/`, `-`, `?`, anything) is open and contributes the trimmed capture 2 as an item, or the trimmed raw line when capture 2 is empty. Do **not** use `storage.CheckboxRegex` — it accepts only `-`/`*` and only three markers, and it would silently treat `- [+] item` as a non-checkbox.
- A section present with `total == 0` adds `"<heading>: section contains no checkbox"` and is unmeasurable. An empty section is never vacuously complete.

### 1.6 Strict Completion Gate

For the topic's `# Completion Gate` only: after collecting its checkboxes, scan the same lines with `closureListItemRegex`. Any matching line that is **not** a checkbox (does not match `closureCheckboxRegex`) is a violation: add `"# Completion Gate: non-checkbox list item: <trimmed line>"` and mark the section unmeasurable. This includes ordered-list items (`1. ...`). Do not apply this strictness to `# Success Criteria` or `# Definition of Done`.

### 1.7 Goals-section member reader

```go
// parseGoalSectionMembers returns the goal names declared in a topic page's
// Goals section, bounded level-relative.
func parseGoalSectionMembers(content string) []string
```

- Normalise and strip frontmatter as above (tolerate a missing frontmatter block by using the whole normalised document — this reader must not fail a page the gate will judge separately). Strip fenced blocks; on an unterminated fence return `nil`.
- Find a heading whose trimmed text is exactly `Goals` at level 1 or 2 (`# Goals` or `## Goals`). Ignore `### Goals` and deeper. If absent, return `nil`.
- The section ends at the next heading whose level is **less than or equal to the Goals heading's own level**. `## Goals` therefore runs past `###` sub-headings; `# Goals` ends at the next `#`. This bound is load-bearing (spec DB12): bounding at any heading, or at `##` only, under-reads members and lets an incomplete topic close.
- An entry is a line matching `^- \[\[([^\]|#]+)\]\]` at column 0 — a declaration, not a mention. Strip a trailing `|alias` and `#heading` from the captured name (do this in the capture or immediately after; `storage.findFileByName` does not strip them). Numbered list items, bold prose lines and mid-sentence wikilinks are not entries.
- Return names in document order, de-duplicated on first occurrence.

### 1.8 Refusal message

```go
// closureRefusalMessage builds the single-line error text for a refusal.
func closureRefusalMessage(subject string, items []string) string
```

Returns `fmt.Sprintf("cannot complete %s: %d closure item(s) open: %s", subject, len(items), strings.Join(items, "; "))`. `subject` is `"goal"` or `"topic"`. Existing unit tests assert `"cannot complete goal"` and the member name appear in `err.Error()`, so the item text must be embedded.

Keep every function in this file under 80 lines / 50 statements with `nestif` below 4. Split the section walk into a helper if needed (e.g. `closureSectionItems`); do not reach for `//nolint:funlen`.

## 2. `MutationResult` gains `OpenItems`

In `pkg/ops/complete.go`, add to `MutationResult` after `Total`:

```go
	// OpenItems lists every closure item that blocks a goal or topic close, in
	// document order. Populated only on a closure-gate refusal.
	OpenItems []string `json:"open_items,omitempty"`
```

`checkSubtaskCompletion`, `countCheckboxStates`, `resetCheckboxes` and every task path stay untouched.

## 3. Wire `goal complete`

In `pkg/ops/goal_complete.go`:

1. Replace the `already completed` branch (lines 73-76) with a terminal no-op. Use the already-parsed page status, so a terminal page does not list tasks:
   ```go
   if goal.Status() == domain.GoalStatusCompleted || goal.Status() == domain.GoalStatusAborted {
       return MutationResult{Success: true, Name: goal.Name, Vault: vaultName}, nil
   }
   ```
   After this rewrite nothing else in `goal_complete.go` uses `fmt` or `strings` (they were used only by the removed refusal message and `joinTaskNames`); remove both imports if the compiler reports them unused.
2. Replace `checkOpenTasks` with a member collector that keeps the same filtering and formatting:
   ```go
   // linkedTaskMembers returns every task linked to the goal, as closure members.
   // It is the only reader of the task relation; the gate decides which statuses
   // are open.
   func (g *goalCompleteOperation) linkedTaskMembers(
       ctx context.Context,
       vaultPath string,
       goalName string,
   ) ([]closureMember, error)
   ```
   `ListTasks`; for each task with `taskLinkedToGoal(task, goalName)` append `closureMember{Name: task.Name, Status: string(task.Status())}`. Do not filter by status here — the gate applies the open-status set. On `ListTasks` error return `errors.Wrap(ctx, err, "list tasks")`. Delete `joinTaskNames` if nothing else uses it (grep first); keep `checkOpenTasks`'s existing message assertions satisfied through `closureRefusalMessage` (requirement 1.8).
3. In `Execute`, after the terminal no-op and before any status mutation:
   ```go
   var members []closureMember
   if !force {
       members, err = g.linkedTaskMembers(ctx, vaultPath, goalName)
       if err != nil {
           return MutationResult{Success: false, Error: err.Error()}, err
       }
   }
   verdict, err := checkClosureGate(ctx, closureKindGoal, string(goal.Content), members)
   if err != nil {
       return MutationResult{Success: false, Error: err.Error()}, err
   }
   if verdict.Outcome == closureOutcomeTerminal {
       return MutationResult{Success: true, Name: goal.Name, Vault: vaultName}, nil
   }
   if verdict.Outcome != closureOutcomeClose {
       msg := closureRefusalMessage("goal", verdict.OpenItems)
       return MutationResult{Success: false, Error: msg, OpenItems: verdict.OpenItems},
           errors.Errorf(ctx, "%s", msg)
   }
   ```
   `force` therefore bypasses **only** the member list, never the section gate: a forced close still refuses on an open criterion.
4. `taskLinkedToGoal` normalises both link forms:
   ```go
   func taskLinkedToGoal(task *domain.Task, goalName string) bool {
       for _, g := range task.Goals() {
           if normaliseGoalLink(g) == goalName {
               return true
           }
       }
       return false
   }

   // normaliseGoalLink strips the wikilink wrapper and any alias/heading suffix
   // so '[[Some Goal]]', '[[Some Goal|label]]' and 'Some Goal' compare equal.
   func normaliseGoalLink(link string) string
   ```
   Trim surrounding whitespace; strip one leading `[[` and one trailing `]]`; strip everything from the first `|`; strip everything from the first `#`. Leave the open-status set (`next`, `todo`, `in_progress`) unchanged — it now lives in the gate, not in this function.

`checkClosureGate` must appear **exactly once** in this file (the call above). Do not mention it in a comment here.

## 4. Wire `topic complete`

In `pkg/ops/topic_complete.go`:

1. Constructor gains goal storage:
   ```go
   func NewTopicCompleteOperation(
       topicStorage storage.TopicStorage,
       goalStorage storage.GoalStorage,
   ) TopicCompleteOperation
   ```
   Add the field and doc-comment the reason: the member-goal terminality check needs each member's status, and topics carry no task relation. Update the `Execute` doc comment, which currently says "there is no open-task gate" — it now has a closure gate.
2. Replace the `already completed` branch (lines 52-55) with:
   ```go
   if status := topic.GetField("status"); status == domain.TopicStatusCompleted || status == "aborted" {
       return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
   }
   ```
3. Before `SetField`, build members and run the gate:
   ```go
   members, err := o.goalMembers(ctx, vaultPath, string(topic.Content))
   if err != nil {
       return MutationResult{Success: false, Error: err.Error()}, err
   }
   verdict, err := checkClosureGate(ctx, closureKindTopic, string(topic.Content), members)
   if err != nil {
       return MutationResult{Success: false, Error: err.Error()}, err
   }
   if verdict.Outcome == closureOutcomeTerminal {
       return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
   }
   if verdict.Outcome != closureOutcomeClose {
       msg := closureRefusalMessage("topic", verdict.OpenItems)
       return MutationResult{Success: false, Error: msg, OpenItems: verdict.OpenItems},
           errors.Errorf(ctx, "%s", msg)
   }
   ```
   ```go
   // goalMembers resolves each goal named in the topic's Goals section to its
   // status. A member whose goal file does not exist reads as open (Status ""):
   // an unresolvable member is never treated as terminal.
   func (o *topicCompleteOperation) goalMembers(
       ctx context.Context,
       vaultPath string,
       content string,
   ) ([]closureMember, error)
   ```
   For each name from `parseGoalSectionMembers(content)`: `goalStorage.FindGoalByName(ctx, vaultPath, name)`; on `errors.Is(err, storage.ErrNotFound)` append `closureMember{Name: name}` and continue; on any other error return `errors.Wrapf(ctx, err, "find goal %s", name)`. Otherwise append `closureMember{Name: name, Status: string(goal.Status())}`.
4. Do **not** add a `--force` flag, a `reason` or a `gateSuccessor` parameter.

Update the one production call site in `pkg/cli/cli.go` (line 1822):

```go
goalStore := storage.NewGoalStorage(storageConfig)
completeOp := ops.NewTopicCompleteOperation(topicStore, goalStore)
```

`grep -rn 'NewTopicCompleteOperation' .` afterwards must show only `pkg/cli/cli.go`, `pkg/ops/topic_complete.go` and `pkg/ops/topic_complete_test.go`.

## 5. CLI: print one line per open item

In `pkg/cli/cli.go`, add next to `runMutation`:

```go
// printOpenItems writes one stdout line per closure-gate item. A refusal must
// name what is open on stdout; the error itself goes to stderr via Execute.
func printOpenItems(items []string) {
	for _, item := range items {
		fmt.Println(item)
	}
}
```

In `createGoalCompleteCommand`'s runner and `createTopicCompleteCommand`'s runner, replace the plain `if err != nil { return result, err }` with:

```go
if err != nil {
    if !OutputFormat(*outputFormat).IsJSON() {
        printOpenItems(result.OpenItems)
    }
    return result, err
}
```

The success lines (`✅ Goal completed: …`, `✅ Topic completed: …`) stay as they are and are printed only on success. `runMutation` already prints the result as JSON on error, so `--output json` surfaces `open_items` with no further change. A refusal exits 1 via `cli.Execute`; nothing else is written to stdout.

If `dupl` reports the two runners as duplicates, add `//nolint:dupl // Mutation commands have similar structure but different operations` above the function, matching the existing comments in this file. Do not refactor unrelated commands.

## 6. Tests

### 6.1 Expose the gate to `ops_test`

Append to `pkg/ops/export_test.go` (same style as the existing aliases):

```go
// CheckClosureGate exposes checkClosureGate for the external ops_test package.
var CheckClosureGate = checkClosureGate

// ParseGoalSectionMembers exposes parseGoalSectionMembers for ops_test.
var ParseGoalSectionMembers = parseGoalSectionMembers

// Closure kinds and outcomes for ops_test.
const (
	ClosureKindGoal             = closureKindGoal
	ClosureKindTopic            = closureKindTopic
	ClosureOutcomeOpen          = closureOutcomeOpen
	ClosureOutcomeUnmeasurable  = closureOutcomeUnmeasurable
	ClosureOutcomeClose         = closureOutcomeClose
	ClosureOutcomeTerminal      = closureOutcomeTerminal
)

// ClosureMember and ClosureVerdict expose the gate's value types to ops_test.
type ClosureMember = closureMember

type ClosureVerdict = closureVerdict
```

The `closureMember` / `closureVerdict` fields are already exported, so the aliases are constructible from `ops_test`.

### 6.2 New file `pkg/ops/closure_gate_test.go` (package `ops_test`)

One `Describe("Closure gate", …)` containing a single `DescribeTable("verdict", …)` (or one table per kind — the focus must still match every entry). Each entry calls `ops.CheckClosureGate(ctx, kind, content, members)` and asserts **both** `verdict.Outcome` and the expected `OpenItems` count/content. A case that only runs is not evidence.

Required entries, each asserting its own expected verdict:

| Entry name | Kind | Fixture | Expected |
|---|---|---|---|
| `SC box open` | goal | full frontmatter, `# Success Criteria` with `- [ ] an open criterion`, `# Definition of Done` all `[x]` | `OPEN`, 1 item containing `an open criterion` |
| `DoD section present but empty` | goal | SC all `[x]`, `# Definition of Done` heading with no box | `UNMEASURABLE`, 1 item naming `# Definition of Done` |
| `DoD absent before cutoff is grandfathered` | goal | `created: "2026-06-01"`, no DoD heading, SC all `[x]` | `CLOSE`, 0 items |
| `DoD absent with no created is grandfathered` | goal | no `created`, no DoD heading, SC all `[x]` | `CLOSE`, 0 items |
| `DoD heading in a non-# spelling is unmeasurable` | goal | `created: "2026-06-01"`, `## Definition of Done` with no box | `UNMEASURABLE`, not grandfathered |
| `DoD heading as a bullet is unmeasurable` | goal | `created: "2026-06-01"`, `- Definition of Done` with no box | `UNMEASURABLE`, not grandfathered |
| `DoD heading as setext is unmeasurable` | goal | `created: "2026-06-01"`, `Definition of Done` followed by `===` and no box | `UNMEASURABLE`, not grandfathered |
| `DoD absent on or after cutoff` | goal | `created: "2026-06-26"`, no DoD heading, SC all `[x]` | `UNMEASURABLE`, 1 item naming `# Definition of Done` |
| `DoD absent with non-ISO created` | goal | `created: "15.01.2026"`, no DoD heading, SC all `[x]` | `UNMEASURABLE`, 1 item naming `# Definition of Done` |
| `star bullet open box` | goal | `# Success Criteria` with `* [ ] open`, DoD all `[x]` | `OPEN` |
| `unrecognised dash marker` | goal | `# Success Criteria` with `- [-] open`, DoD all `[x]` | `OPEN` |
| `unrecognised question marker` | goal | `# Success Criteria` with `- [?] open`, DoD all `[x]` | `OPEN` |
| `box inside fenced block is not a box` | goal | SC holding only a fenced `- [ ] example`, DoD all `[x]` | `UNMEASURABLE`, 1 item naming `# Success Criteria` |
| `unterminated fence` | goal | document ending inside a fence | `UNMEASURABLE`, exactly 1 item `unterminated code fence` |
| `BOM prefixed CRLF page` | goal | content prefixed with `"\ufeff"` and using `"\r\n"` line endings, `created` before cutoff, SC all `[x]` | `CLOSE` (frontmatter read) |
| `Completion Gate box open` | topic | SC all `[x]`, `# Completion Gate` with `- [ ] gate` | `OPEN`, 1 item `gate` |
| `Completion Gate mixes a checkbox with an ordered item` | topic | SC all `[x]`, `# Completion Gate` with `- [x] done` and `1. an ordered item` | `UNMEASURABLE` |
| `member goal not terminal` | topic | complete SC + Gate, member `{Name: "member", Status: "in_progress"}` | `OPEN`, item naming `member` |
| `member goal terminal` | topic | complete SC + Gate, member `{Name: "member", Status: "completed"}` | `CLOSE` |
| `topic with no Goals section` | topic | complete SC + Gate, no Goals heading, no members | `CLOSE` |
| `already completed page` | goal | `status: completed` | `TERMINAL`, 0 items |
| `already aborted page` | goal | `status: aborted` | `TERMINAL`, 0 items |
| `missing frontmatter` | goal | content with no frontmatter | an error (assert `err` is not nil) |

Add a second `Describe("Closure gate Goals section", …)` with a `DescribeTable("members", …)` covering `parseGoalSectionMembers`:
- `## Goals` with `- [[A]]` and `- [[B]]` → `[A B]`;
- `## Goals` with a `### Lever A` sub-heading holding `- [[C]]` → `[A B C]` (bound is level-relative);
- `# Goals` with a `## Sub` sub-heading holding `- [[D]]` → includes `D` (bound is not fixed at `##`);
- `# Goals` followed by a second `# Other` heading whose body holds `- [[E]]` → `E` excluded;
- a numbered `1. [[X]]`, a bold `**[[Y]]**` line and a mid-sentence wikilink → none included;
- `- [[Name|label]]` → `Name`;
- no Goals heading → empty.

### 6.3 Repair `pkg/ops/goal_complete_test.go`

- Add a package-level helper in the test file (or a `BeforeEach` default) producing a closure-complete goal body:
  ```go
  func completeGoalContent(status string) domain.Content {
      return domain.Content("---\nstatus: " + status + "\n---\n# Success Criteria\n- [x] done\n")
  }
  ```
  The absent `# Definition of Done` is grandfathered (no `created`), so this body closes.
- Replace every fixture that expects a successful close (`Content("")` on a goal expected to complete: the default `BeforeEach`, "goal completes without close-out fields", "one-step close-out…", both phase contexts, "success", "completed tasks do not block", "tasks linked to other goals do not block", "zero linked tasks", "force bypasses open task check") with `completeGoalContent(<its status>)`. Do not weaken any assertion.
- **Invert** `Context("goal already completed")` (lines 88-101): keep the context, keep the two `It`s but change them to assert `err` is nil, `result.Success` is true, `result.Name` equals `goalName`, and `WriteGoalCallCount()` is 0. Add a third `It` asserting `result.OpenItems` is empty. Do not delete the context.
- Add `Context("goal already aborted")` mirroring the inverted completed context (status `aborted`).
- Add `Context("force does not bypass an open criterion")`: `force = true`, content with an open SC box and a DoD all ticked; assert error, `WriteGoalCallCount() == 0`, and `result.OpenItems` contains the criterion text.
- Add `Context("bracketed goal link blocks completion")`: a task named `linked-task` at `status: todo` whose `SetGoals([]string{"[[" + goalName + "]]"})`; assert the error contains `linked-task` and no write. Add the bare-name twin (`SetGoals([]string{goalName})`) asserting the same, so the asymmetry is pinned. Also add two suffix cases, each asserting the refusal still fires and no write happens: `SetGoals([]string{"[[" + goalName + "|label]]"})` and `SetGoals([]string{"[[" + goalName + "#heading]]"})` — these pin `normaliseGoalLink`'s alias and heading stripping, which nothing else exercises.
- Add `Context("open criterion blocks completion")`: content with one open SC box and a ticked DoD; assert error containing the criterion text, `WriteGoalCallCount() == 0`, and `result.OpenItems` length 1.
- Keep the existing "open todo task blocks completion" / "open in_progress task blocks completion" contexts working: their error text now comes from `closureRefusalMessage`, and it still contains `cannot complete goal` and the task name.

### 6.4 Repair `pkg/ops/topic_complete_test.go`

- Add `mockGoalStorage *mocks.GoalStorage` and construct with `ops.NewTopicCompleteOperation(mockTopicStorage, mockGoalStorage)`. The file will need the imports `"github.com/bborbe/errors"` (for `errors.Is`) and `"github.com/bborbe/vault-cli/pkg/storage"` (for `storage.ErrNotFound`).
- Add a closure-complete topic body helper:
  ```go
  func completeTopicContent(status string) domain.Content {
      return domain.Content("---\nstatus: " + status + "\n---\n# Success Criteria\n- [x] done\n# Completion Gate\n- [x] gated\n")
  }
  ```
- Use it for the default `BeforeEach` (line 42) and both alternate fixtures (lines 90, 108).
- **Invert** `Context("topic already completed")` (lines 121-144): assert `err` is nil, `result.Success` is true, `WriteTopicCallCount() == 0`. Do not delete the context.
- Add `Context("topic already aborted")` mirroring it.
- Add `Context("open Completion Gate box blocks completion")` and `Context("non-terminal member goal blocks completion")` (member `my-member` at `in_progress`, `mockGoalStorage.FindGoalByNameReturns(goal, nil)` for that name; assert the error names the member and no write).
- Add `Context("member goal missing from the vault blocks completion")`: `FindGoalByNameReturns(nil, storage.ErrNotFound)`; assert error and no write.
- Existing "topic not found" and "write topic fails" contexts stay unchanged.

### 6.5 Integration tests in `integration/cli_test.go`

Add a new `Describe("vault-cli goal and topic closure gate", …)` block (or two sibling blocks — goal and topic) after the existing `Describe("vault-cli goal terminal phase")` block. Each `It` gets its own temp vault via `createTempVaultWithGoals` / `createTempVaultWithTopicPages`, a `runCommand(args ...string) *gexec.Session` closure, a `readFile` closure, and its own `sha256OfFile` closure (copy the existing local-closure style; do not add a package-level helper).

Case names and assertions (every name must appear in `go test ./integration -run TestIntegration -ginkgo.v -v`):

1. `It("goal complete refuses an open Success Criteria box and leaves status in_progress")` — goal fixture with one open SC box and a ticked DoD; `goal complete` exits non-zero, stdout contains the box text, and the on-disk file still reads `status: in_progress`.
2. `It("goal complete closes a fully ticked goal with terminal linked tasks")` — every SC and DoD box `[x]`, one linked task at `completed`; exit 0, file reads `status: completed`.
3. `It("goal complete refuses an open task linked as a wikilink")` — task `goals: ["[[<goal>]]"]` at `todo`; non-zero exit, stdout contains the task name, file unchanged.
4. `It("goal complete refuses an open task linked as a bare name")` — same task with the bare name; non-zero exit, stdout contains the task name.
5. `It("goal complete closes a pre-cutoff goal with no Definition of Done")` — `created: "2026-01-05"`, no DoD section, SC all `[x]`; exit 0, `status: completed`.
6. `It("goal complete refuses a goal whose Definition of Done section holds no box")` — DoD heading present, no box; non-zero exit, stdout names `# Definition of Done`.
7. `It("goal complete refuses a post-cutoff goal with no Definition of Done")` — `created: "2026-07-01"`, no DoD; non-zero exit, stdout names `# Definition of Done`.
8. `It("topic complete refuses an open Completion Gate box")` — SC all `[x]`, open Gate box; non-zero exit, stdout contains the gate text, file still `in_progress`.
9. `It("topic complete refuses a non-terminal member goal")` — complete SC + Gate, `## Goals` listing `- [[Member Goal]]`, member goal at `in_progress`; non-zero exit, stdout names `Member Goal`.
10. `It("topic complete reads a member under a ### sub-heading inside ## Goals")` — SC and Gate all `[x]`, member declared under `### Lever A` inside `## Goals`, member at `in_progress`; non-zero exit naming the member (proves the bound is not "any heading").
11. `It("topic complete reads a member under ## inside # Goals")` — SC and Gate all `[x]`, `# Goals` with a `## Sub` sub-heading holding the non-terminal member; non-zero exit naming the member (proves the bound is level-relative, not fixed at `##`).
12. `It("topic complete closes a fully ticked topic with terminal member goals")` — SC and Gate all `[x]`, member goal `completed`; exit 0, `status: completed`.
13. `It("goal complete prints one line per open item and reports them in open_items")` — goal with exactly two open SC boxes. Run the plain invocation: exit non-zero, and `strings.Split(strings.TrimSpace(string(session.Out.Contents())), "\n")` has length 2. Run the same command with `--output json`: exit non-zero, unmarshal stdout into `map[string]any`, assert `open_items` is a `[]any` of length 2. Do not depend on `jq` being installed; the JSON probe is the Go unmarshal.
14. `It("goal complete is a no-op on an already-completed goal")` — fixture `status: completed`; capture the digest, run, exit 0, digest unchanged.
15. `It("goal complete is a no-op on an aborted goal")` — fixture `status: aborted`; exit 0, digest unchanged.
16. `It("topic complete is a no-op on an aborted topic")` — topic `status: aborted`; exit 0, digest unchanged.

Additionally **rewrite** the existing `It("topic complete moves the status to completed and refuses a second complete")` (line 6161) into `It("topic complete moves the status to completed and is a no-op on a second complete")`: give the fixture a closure-complete body, keep the first-command assertions (`status: completed`), then assert the second command exits **0** and the file digest is unchanged. That is the fourth terminal digest case.

Repair the two existing fixtures that feed a successful `goal complete`:
- `Describe("vault-cli goal close-out gating")` fixture (line ~2085): change the `my-goal` body to `---\nstatus: in_progress\n---\n# My Goal\n# Success Criteria\n- [x] done\n`.
- `Describe("vault-cli goal terminal phase")` fixture (line ~2197): same body, keeping `phase: execution`.
No other existing integration assertion may be relaxed or deleted.

## 7. CHANGELOG

`CHANGELOG.md` has no `## Unreleased` section. Create it **below** the preamble block (after the `* PATCH version …` line) and **above** `## v0.197.1`, with one bullet:

```
## Unreleased

- fix: `goal complete` and `topic complete` refuse to close a page whose own closure evidence is incomplete, name every open item on stdout and in `--output json` under `open_items`, and treat an already-completed or aborted page as a no-op instead of an error
```

Do not bump any version manifest and do not tag.
</requirements>

<constraints>
- **Do not change `task complete`'s behaviour.** `pkg/ops/complete.go`'s `checkSubtaskCompletion`, `countCheckboxStates` and `resetCheckboxes` are the model, not a target. The only edit to that file is the `OpenItems` field on `MutationResult`.
- **Do not shell out to, import, or depend on `claude-supervisor`.** The two plugins are installed independently; vault-cli must work when the other is absent. Do not read `goal-closure-read.py` at runtime.
- **The check is read-only.** `checkClosureGate`, `parseGoalSectionMembers` and their helpers never write, create or delete a file.
- **`pkg/ops` is a library layer** — it returns structured results and never writes to stdout. All output formatting lives in `pkg/cli`.
- **The JSON field is `open_items`**, a `[]string` on `MutationResult`, with tag `json:"open_items,omitempty"`. Do not invent another name.
- **Invert, do not delete, the two existing terminal tests** (`pkg/ops/goal_complete_test.go:88-101`, `pkg/ops/topic_complete_test.go:121-144`).
- **`--force` on `goal complete` bypasses only the open-task gate**, never the closure gate. `topic complete` gains no `--force`, no `--reason`, no `--gate-successor`.
- **`checkClosureGate` appears exactly once in `pkg/ops/goal_complete.go` and exactly once in `pkg/ops/topic_complete.go`** — the call site. The definition lives in `pkg/ops/closure_gate.go`. Do not mention the name in comments in those two files.
- **No checkbox regex in the two call-site files.** `grep -cE '\[[ xX/-]\]' pkg/ops/goal_complete.go pkg/ops/topic_complete.go` must return 0 for each; the regex lives only in `pkg/ops/closure_gate.go`.
- **Do not touch** `pkg/ops/objective_complete.go`, `theme complete`, `vision complete`, `goal set`, `topic set`, `task set`, or any lint rule.
- **Do not retrofit historical pages.** No migration, no bulk rewrite, no backfill of `created`.
- **Do not edit** `agents/*.md`, `docs/*.md`, `scripts/*.sh`, `commands/*.md`, or the vault's closure guide. The in-repo closure-box table (`agents/goal-manager-agent.md` line 152) is a reference to agree with, not a file to change.
- **Error wrapping** goes through `github.com/bborbe/errors` — `errors.Wrap`, `errors.Wrapf`, `errors.Errorf` with `ctx`. Never `fmt.Errorf`, never `context.Background()` inside `pkg/ops`.
- **Do not hand-bump version manifests or tag.** `.dark-factory.yaml` carries `autoRelease: false` for the container run; the releaser owns the tag post-merge.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make test` — must exit 0. Re-run it iteratively while implementing; it is the fast feedback loop.

Focused runs (confirm the new case names actually appear — a focus matching zero specs exits 0 while running nothing):

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus=Closure -ginkgo.v
go test -mod=mod ./integration -run TestIntegration -ginkgo.v -v
go test -mod=mod ./integration -run TestIntegration -v
```

`go test -mod=mod ./integration -run TestIntegration -v` alone may not list spec names under Ginkgo v2's default reporter; the `-ginkgo.v` run is the one that prints them. Both must exit 0, and the `-ginkgo.v` output must contain every case name listed in requirement 6.5 and every table entry name in requirement 6.2.

Evidence greps (each count is exact unless stated):

```
grep -n 'checkClosureGate' pkg/ops/goal_complete.go pkg/ops/topic_complete.go   # exactly one line per file
grep -cE '\[[ xX/-]\]' pkg/ops/goal_complete.go                                  # 0
grep -cE '\[[ xX/-]\]' pkg/ops/topic_complete.go                                 # 0
grep -c 'open_items' pkg/ops/complete.go                                         # 1
grep -c 'func checkClosureGate' pkg/ops/closure_gate.go                          # 1
grep -rn 'NewTopicCompleteOperation' pkg/ integration/                          # cli.go, topic_complete.go, topic_complete_test.go only
grep -n '^## Unreleased' CHANGELOG.md                                            # 1 line, above v0.197.1
```

```
go vet -mod=mod ./pkg/ops/... ./integration/... ./pkg/cli/...   # exits 0
```

Finally run `make precommit` once — must exit 0.

If `make precommit` exits non-zero: fix, re-run only the failing target (`make lint`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. A non-zero `make precommit` exit code must be reported as `"status":"failed"` in the completion report — never rationalised as success.
</verification>

<!--
Reviewer notes — two points the spec left open, resolved here rather than asked:

1. A topic member goal whose file cannot be found. The spec's Failure Modes table
   does not cover it. Chosen: read it as open (Status ""), never as terminal, so an
   unresolvable member refuses the close instead of silently passing. Any error
   other than storage.ErrNotFound still propagates.
2. Goals-section entry normalisation. The shipped reader
   (scripts/update-topic-goal-entries.awk) takes the raw wikilink inner text, which
   would make `- [[Name|label]]` unresolvable by FindGoalByName. Chosen: strip a
   trailing `|alias` and `#heading` so the entry resolves; the level-relative bound
   and the `^- \[\[...\]\]` declaration-only rule are ported unchanged.
-->
