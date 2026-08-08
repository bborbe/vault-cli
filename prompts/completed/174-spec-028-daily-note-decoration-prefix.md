---
status: completed
spec: [028-bug-daily-note-decoration-prefix]
summary: Added trimLeadingDecoration helper to skip leading decoration (emoji, symbols, markdown emphasis) before wikilink test in IsOwnDailyNoteEntry; extended table tests with 22 new entries; added decorated-entry regression tests for complete/defer/work-on; updated docs and CHANGELOG
execution_id: vault-cli-exec-174-spec-028-daily-note-decoration-prefix
dark-factory-version: v0.192.9
created: "2026-08-08T12:30:00Z"
queued: "2026-08-08T12:52:37Z"
started: "2026-08-08T12:52:39Z"
completed: "2026-08-08T12:57:24Z"
branch: dark-factory/bug-daily-note-decoration-prefix
---

# Recognise decorated daily-note entries as a task's own entry

<summary>
- A daily-note entry that starts with a category emoji before the task link is now recognised as that task's own entry.
- `task complete` flips such an entry to done instead of silently doing nothing.
- `task defer` removes such an entry from the source day instead of leaving it behind.
- `task work-on` promotes such an entry in place instead of appending a duplicate line beside it.
- The decoration itself is preserved when the line is rewritten — the emoji stays.
- Decoration is recognised by character class, not by a list of known emoji, so uncommon prefixes and markdown emphasis work too.
- Prose before the link still makes the line a mention, in any script — a Japanese or Russian word before the link is prose, not decoration.
- A decorated line whose leading link points at a different task is still only a mention for this task.
- Nothing that already counted as an own entry stops counting — this widens recognition and narrows nothing.
- The daily-note contract doc is updated to describe the decoration rule.
</summary>

<objective>
Make the shared daily-note own-entry matcher skip leading decoration (category emoji, symbols, markdown emphasis) before it looks for the task's wikilink, so that `task complete`, `task defer`, and `task work-on` stop silently ignoring the ~11% of real daily-note entries that prefix the wikilink with an emoji — and so `work-on` stops appending a duplicate entry beside the one it failed to recognise.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement).

Read these files fully before making changes:

- `/workspace/pkg/ops/daily_note_entry.go` — the only file whose behavior changes. Current implementation, verified:
  ```go
  func IsOwnDailyNoteEntry(checkboxText string, taskName string) bool {
      // Step 1: trim leading whitespace
      trimmed := strings.TrimLeft(checkboxText, " \t")

      // Step 2: must start with [[
      if !strings.HasPrefix(trimmed, "[[") {
          return false
      }
      ...
  }
  ```
  Steps 2–7 (`[[` prefix, find `]]`, cut alias `|`, cut heading `#`, `strings.EqualFold` full-string compare) stay exactly as they are. Only step 1 changes.

- `/workspace/pkg/ops/daily_note_entry_test.go` — the existing `Describe("IsOwnDailyNoteEntry")` with one `DescribeTable` (13 entries) and one writer-contract `Describe`. You append entries to the existing table; you remove nothing.

- `/workspace/pkg/storage/base.go` — the checkbox parser, verified, do not change:
  ```go
  CheckboxRegex          = regexp.MustCompile(`^(\s*)[-*] \[([ x/])\] (.+)$`)   // 1=leading ws, 2=state, 3=text
  CheckboxCompleteRegex  = regexp.MustCompile(`([-*]) \[([ /])\]`)              // 1=list marker, 2=state
  CheckboxUncompleteRegex = regexp.MustCompile(`([-*]) \[x\]`)
  ```
  `IsOwnDailyNoteEntry` receives capture group 3. `CheckboxCompleteRegex` only rewrites the `- [ ]` / `- [/]` marker, which is why decoration in the text survives a rewrite for free.

- The three call sites, all already sharing the matcher — **do not modify these three `.go` files**, they inherit the fix:
  - `/workspace/pkg/ops/complete.go` → `updateDailyNote`, line with `if IsOwnDailyNoteEntry(taskText, taskName) {`
  - `/workspace/pkg/ops/defer.go` → `removeFromDailyNote`, same condition
  - `/workspace/pkg/ops/workon.go` → `findAndUpdateTaskCheckbox`, same condition

- `/workspace/pkg/ops/complete_test.go` — read `Context("updateDailyNote with a mention line above the own entry", ...)` near the end of the file. That block is the exact shape to copy for the new decorated-entry block: nested `BeforeEach` reassigns `taskName`, rebuilds `task` with `domain.NewTask(map[string]any{"status": "todo"}, domain.FileMetadata{Name: taskName}, domain.Content(""))`, calls `mockTaskStorage.FindTaskByNameReturns(task, nil)`, sets `mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)` and `WriteDailyNoteReturns(nil)`. The outer `JustBeforeEach` runs `completeOp.Execute`. Assertions read `_, _, _, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)`.

- `/workspace/pkg/ops/defer_test.go` — read `Context("removeFromDailyNote with a mention line and duplicate own entries", ...)`. Note it uses `ReadDailyNoteReturnsOnCall(0, todayContent, nil)` / `ReadDailyNoteReturnsOnCall(1, targetContent, nil)` and sets `dateStr = "2026-12-31"`; `WriteDailyNoteArgsForCall(0)` is the source day.

- `/workspace/pkg/ops/workon_test.go` — read `Context("daily note updates", ...)` and its nested `Context("updateDailyNote with a mention line above a pending own entry", ...)`. Note the task in this file needs `domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/..."}`, and the outer `BeforeEach` already wires `mockStarter.StartSessionReturns("session-123", nil)` so no real Claude session is spawned.

- `/workspace/docs/daily-notes.md` — exists; you edit the "Task Entry Contract" and "Own Entry vs Mention" sections.
- `/workspace/CHANGELOG.md` — a `## Unreleased` section already exists with two bullets; append to it.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>

## 1. Skip leading decoration in the matcher

In `/workspace/pkg/ops/daily_note_entry.go`, add the `unicode` import and one unexported helper above `IsOwnDailyNoteEntry`:

```go
// trimLeadingDecoration returns s with its leading run of decoration runes
// removed. A decoration rune is any rune that is neither a letter nor a digit
// and is not the wikilink opener. Iteration is over runes, not bytes, so a
// multi-rune emoji with a variation selector is consumed whole; and because
// the test is Unicode-aware, a leading letter in any script — Latin, CJK,
// Cyrillic — ends the run and leaves the text classified as prose.
func trimLeadingDecoration(s string) string {
	for i, r := range s {
		if r == '[' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return s[i:]
		}
	}
	return ""
}
```

Then replace step 1 of `IsOwnDailyNoteEntry`:

```go
// old
trimmed := strings.TrimLeft(checkboxText, " \t")

// new
trimmed := trimLeadingDecoration(checkboxText)
```

Leave steps 2–7 byte-for-byte unchanged. Whitespace is already covered by the new rule (a space is neither letter nor digit nor `[`), so the old `TrimLeft` is subsumed, not merely reordered. `strings` stays imported — steps 2–7 still use `HasPrefix`, `Index`, `EqualFold`, `TrimSpace`.

Update the `IsOwnDailyNoteEntry` GoDoc to state the widened rule: any leading run of non-letter, non-digit runes is skipped as decoration before the wikilink test; prose before the wikilink still makes the line a mention; the classification is by character class, not by an enumerated set of prefixes.

**No emoji character may appear anywhere in `/workspace/pkg/ops/daily_note_entry.go` — not in code, not in comments, not in examples.** Write "an emoji" or "a category emoji" as words. An emoji literal in the doc comment fails the generality check in `<verification>`. Do not add a cutset, a rune slice, a package-level `var`, or any list of known prefixes; the character-class test above is the whole rule.

## 2. Extend the matcher's table test

In `/workspace/pkg/ops/daily_note_entry_test.go`, append `Entry` cases to the **existing** `DescribeTable`. Do not modify, reorder, or remove any existing entry. Use exactly these descriptions — `<verification>` greps them verbatim.

Fixtures use task name `Feed Worms` throughout, matching the spec's reproduction.

| Entry description | checkboxText | taskName | expected |
|---|---|---|---|
| `decorated wikilink is the own entry` | `🐟 [[Feed Worms]]` | `Feed Worms` | `true` |
| `decoration before an alias wikilink is the own entry` | `🐟 [[Feed Worms\|worms]]` | `Feed Worms` | `true` |
| `decorated own entry with a second wikilink in trailing prose is the own entry` | `🚨 [[Feed Worms]] — analysis done; root cause = aging [[Samsung SSD 840 Pro 256GB]]` | `Feed Worms` | `true` |
| `decoration followed by prose is a mention` | `🔧 Nuke-reboot chain — [[Feed Worms]].` | `Feed Worms` | `false` |
| `decoration then a wikilink to another task is a mention` | `🔧 [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].` | `Feed Worms` | `false` |
| `CJK leading prose is a mention` | `作業 [[Feed Worms]]` | `Feed Worms` | `false` |
| `Cyrillic leading prose is a mention` | `Задача по [[Feed Worms]]` | `Feed Worms` | `false` |
| `decoration with no wikilink is not an own entry` | `🐟 ` (emoji then one space) | `Feed Worms` | `false` |
| `arrows-counterclockwise emoji is decoration` | `🔄 [[Feed Worms]]` | `Feed Worms` | `true` |
| `chart-increasing emoji is decoration` | `📈 [[Feed Worms]]` | `Feed Worms` | `true` |
| `wrench emoji is decoration` | `🔧 [[Feed Worms]]` | `Feed Worms` | `true` |
| `fish emoji is decoration` | `🐟 [[Feed Worms]] — feed the worms` | `Feed Worms` | `true` |
| `house emoji is decoration` | `🏠 [[Feed Worms]]` | `Feed Worms` | `true` |
| `lock emoji is decoration` | `🔒 [[Feed Worms]]` | `Feed Worms` | `true` |
| `variation-selector warning emoji is decoration` | `⚠️ [[Feed Worms]]` | `Feed Worms` | `true` |
| `variation-selector shield emoji is decoration` | `🛡️ [[Feed Worms]]` | `Feed Worms` | `true` |
| `markdown emphasis is decoration` | `**[[Feed Worms]]**` | `Feed Worms` | `true` |
| `emoji plus markdown emphasis is decoration` | `🎯 **[[Feed Worms]]**` | `Feed Worms` | `true` |
| `section-sign decoration is skipped by character class` | `§ [[Feed Worms]]` | `Feed Worms` | `true` |
| `diamond decoration is skipped by character class` | `❖ [[Feed Worms]]` | `Feed Worms` | `true` |
| `double-dagger decoration is skipped by character class` | `‡ [[Feed Worms]]` | `Feed Worms` | `true` |
| `keycap digit prefix is not decoration` | `1️⃣ [[Feed Worms]]` | `Feed Worms` | `false` |

Notes on specific rows:

- The `\|` in the alias row is table-escaping in this prompt only; in Go it is a plain `|` inside a normal string literal.
- `⚠️` is `U+26A0 U+FE0F` and `🛡️` is `U+1F6E1 U+FE0F` — copy them verbatim including the trailing variation selector; the point of these rows is that the trailing `U+FE0F` rune is also skipped.
- `1️⃣` is `U+0031 U+FE0F U+20E3` — it leads with the ASCII digit `1`, so the skip run terminates immediately and the line is not an own entry. This is a deliberate, documented limitation with zero live occurrences, not a bug to work around.
- The three `skipped by character class` rows are load-bearing: they use decorations that appear in no other fixture and in no census, and they are what prove the implementation is not an enumerated list.

## 3. `complete` flips a decorated own entry

In `/workspace/pkg/ops/complete_test.go`, add a new top-level `Context("updateDailyNote with a decorated own entry", ...)` beside the existing `updateDailyNote` contexts, following their shape exactly (nested `BeforeEach` sets `taskName = "Feed Worms"`, rebuilds `task`, wires the mocks).

Daily-note fixture:

```go
dailyContent := "# 2026-03-03\n\n## Must\n" +
	"- [/] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].\n" +
	"- [/] 🐟 [[Feed Worms]]\n"
```

The mention line is seeded `- [/]`, **not** `- [x]`, and this matters. `CheckboxCompleteRegex` is `([-*]) \[([ /])\]` — it does not match `[x]`, so an `- [x]` mention line comes back byte-identical no matter how the matcher classifies it, and the assertion below would hold even for a wrong implementation. Seeded `- [/]`, a misclassification flips it to `- [x]` and the test actually fires.

Two `It` nodes with exactly these names:

- `flips the decorated own entry and preserves the decoration` — asserts `err` is nil, `WriteDailyNoteCallCount()` is 1, and `updatedContent` contains `- [x] 🐟 [[Feed Worms]]`.
- `leaves the decorated mention line byte-identical` — asserts `updatedContent` still contains `- [/] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].` with the `[/]` state intact.

## 4. `defer` removes a decorated own entry

In `/workspace/pkg/ops/defer_test.go`, add a new top-level `Context("removeFromDailyNote with a decorated own entry", ...)` following the shape of the existing `removeFromDailyNote with a mention line and duplicate own entries` block: set `taskName = "Feed Worms"`, `dateStr = "2026-12-31"`, use `ReadDailyNoteReturnsOnCall(0, todayContent, nil)` and `ReadDailyNoteReturnsOnCall(1, targetContent, nil)`.

```go
todayContent := "# 2026-03-03\n\n## Must\n" +
	"- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].\n" +
	"- [/] 🐟 [[Feed Worms]]\n"
targetContent := "# 2026-12-31\n"
```

Three `It` nodes with exactly these names, all reading `_, _, date, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)` and asserting `date` contains `2026-03-03`:

- `removes the decorated own entry from the source day` — asserts `updatedContent` does not contain `🐟 [[Feed Worms]]`, and `strings.Count(updatedContent, "[[Feed Worms]]")` equals `1` (the surviving mention's trailing link).
- `leaves the decorated mention line byte-identical` — asserts `updatedContent` contains `- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].`
- `still succeeds` — asserts `err` is nil and `WriteDailyNoteCallCount()` equals `2` (defer writes both the source day and the target day; the sibling block at `defer_test.go:872` asserts the same). Without this, a regression that skips the target-day write passes unnoticed.

## 5. `work-on` promotes a decorated entry in place

In `/workspace/pkg/ops/workon_test.go`, add a nested `Context("updateDailyNote with a decorated pending own entry", ...)` inside the existing `Context("daily note updates", ...)`, following the shape of its sibling `updateDailyNote with a mention line above a pending own entry` (task built with both `Name` and `FilePath` in `domain.FileMetadata`).

```go
dailyContent := "## Must\n- [ ] 🐟 [[Feed Worms]]\n"
```

Use the unindented list form above verbatim — `findAndUpdateTaskCheckbox` rewrites the marker with a `strings.Replace` keyed on capture group 1, which is leading whitespace, so an indented fixture would exercise a pre-existing quirk unrelated to this spec.

Three `It` nodes with exactly these names:

- `promotes the decorated pending entry in place` — asserts `WriteDailyNoteCallCount()` is 1 and `content` contains `- [/] 🐟 [[Feed Worms]]`.
- `appends no duplicate entry` — asserts `strings.Count(content, "[[Feed Worms]]")` equals `1`.
- `writes no undecorated duplicate` — asserts `content` does not contain `- [/] [[Feed Worms]]` (the exact line the bug used to append).

## 6. Document the decoration rule

Edit `/workspace/docs/daily-notes.md`:

- In **Task Entry Contract**, widen the opening sentence: a task's own entry is a checkbox line whose text begins with `[[<taskName>]]`, **optionally preceded by decoration** — a category emoji, a symbol, or markdown emphasis such as `**`. Use the word **decoration** and the word **emoji** in prose.
- State the rule as a character class, not a list: everything before the wikilink is skipped as decoration as long as it contains no letter and no digit. The first letter or digit ends the decoration run and makes the rest prose.
- State both polarities explicitly: `- [/] 🐟 [[Feed Worms]]` is an own entry; `- [x] 🔧 Nuke-reboot chain — [[Feed Worms]].` is a mention, because `Nuke-reboot` is a word, not decoration.
- Note that letters in **any script** end the decoration run, so `作業 [[Feed Worms]]` and `Задача по [[Feed Worms]]` are mentions.
- Note that a digit ends the run too, so a keycap prefix such as `1️⃣ [[Feed Worms]]` is not recognised.
- Note that decoration is **preserved** when a command rewrites the line: `task complete` on `- [/] 🐟 [[Feed Worms]]` produces `- [x] 🐟 [[Feed Worms]]`.
- Update the existing **Own Entry vs Mention** explanation so the mention example is explained by "prose before the wikilink", not by "the wikilink is not at the very start".

Do not remove the existing Wikilink Forms, Literal Comparison, or Commands That Read the Contract sections.

## 7. Changelog

Append one bullet to the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (do not create a second `## Unreleased`, do not touch the `## v0.102.4` section):

```
- fix: `task complete`, `task defer`, and `task work-on` now recognise a daily-note entry whose wikilink is preceded by decoration — a category emoji such as `- [/] 🐟 [[Feed Worms]]`, or markdown emphasis — instead of skipping it as a mention. `complete` and `defer` were silent no-ops on those lines and `work-on` appended a duplicate entry beside the one it failed to recognise. Decoration is matched by character class, not by a list of known prefixes, and is preserved when the line is rewritten
```

## 8. Coverage

`pkg/ops` is the changed package. `IsOwnDailyNoteEntry` and `trimLeadingDecoration` must both reach 100% statement coverage from the table in requirement 2 — the `return ""` fall-through in `trimLeadingDecoration` is covered by the `decoration with no wikilink is not an own entry` entry. Do not add retroactive coverage to unrelated untested `pkg/ops` code.

</requirements>

<constraints>
- `IsOwnDailyNoteEntry` keeps its exact signature `func IsOwnDailyNoteEntry(checkboxText string, taskName string) bool` and stays the single shared matcher. Do not add a second matcher, a per-call-site variant, or a second exported symbol in `pkg/ops/daily_note_entry.go`.
- Do NOT modify `/workspace/pkg/ops/complete.go`, `/workspace/pkg/ops/defer.go`, or `/workspace/pkg/ops/workon.go`. All three inherit the fix through the shared matcher; only their `_test.go` files gain new blocks.
- Every test added by spec 027 must keep passing **unmodified**. This change widens what counts as an own entry and must narrow nothing. No existing `Entry` or `It` may be removed, renamed, or weakened in any of the four test files.
- The mention rule is the invariant that must not regress: a wikilink appearing after prose is never an own entry, decorated or not.
- `storage.CheckboxRegex`, `storage.CheckboxCompleteRegex`, and `storage.CheckboxUncompleteRegex` keep their current semantics. Do not edit `/workspace/pkg/storage/base.go` — this changes identification, not parsing.
- Decoration preservation falls out of the existing implementation (`CheckboxCompleteRegex` only replaces the checkbox marker). Do not "improve" the rewrite path to normalise, strip, or re-order decoration.
- List-marker preservation stays intact: `-` stays `-`, `*` stays `*`.
- Do NOT add a flag, config key, environment variable, threshold, or opt-out to select the old behavior. The identity rule is invariant.
- Do NOT hardcode an emoji list, cutset, rune slice, or allow-list of prefixes. The rule is `unicode.IsLetter` / `unicode.IsDigit` / `[` and nothing else. No emoji literal may appear in `/workspace/pkg/ops/daily_note_entry.go`, including comments.
- Out of scope, do not change: goal-file checkbox matching in `markGoalCheckbox` (`/workspace/pkg/ops/complete.go`) and in `/workspace/pkg/ops/update.go`.
- Out of scope, do not change: `findAndUpdateTaskCheckbox`'s `strings.Replace(line, marker+" [ ]", marker+" [/]", 1)` in `/workspace/pkg/ops/workon.go`, where `marker` is capture group 1 (leading whitespace, not the list marker). Its behavior on indented checkbox lines predates this spec and is not in scope; requirement 5 uses an unindented fixture to keep the two concerns separate.
- Notes already polluted with duplicate entries by the old `work-on` behavior are not repaired — this fix is forward-only.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` in `pkg/ops/`.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`. (The matcher returns a plain `bool` and produces no errors.)
- Do NOT hand-bump or tag any version. Do NOT touch `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`. Add the `## Unreleased` bullet only.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**

```
make test
```

Must exit 0.

**2. Every new table entry is present and named as specified** — each command must print a number `>= 1`:

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "decorated wikilink is the own entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "decoration followed by prose is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "decoration then a wikilink to another task is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "CJK leading prose is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Cyrillic leading prose is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "decoration with no wikilink is not an own entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "variation-selector warning emoji is decoration"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "markdown emphasis is decoration"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "section-sign decoration is skipped by character class"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "diamond decoration is skipped by character class"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "double-dagger decoration is skipped by character class"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "keycap digit prefix is not decoration"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "variation-selector shield emoji is decoration"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "emoji plus markdown emphasis is decoration"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "CJK leading prose is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "Cyrillic leading prose is a mention"
```

The last four are load-bearing and must not be dropped: `🛡️` and `🎯 **` are the tail-prefix rows a census-derived cutset cannot survive, and the CJK/Cyrillic rows are the **only** fixtures that distinguish the Unicode-aware rule from an ASCII-only one — an ASCII-only implementation passes every other check in this prompt while wrongly promoting non-Latin prose lines to own entries.

The `-ginkgo.v` flag is required — plain `go test -v` does not enable Ginkgo's verbose reporter, so entry descriptions print only when a spec fails.

**3. Call-site regression tests present** — each must print `>= 1`:

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "flips the decorated own entry and preserves the decoration"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "removes the decorated own entry from the source day"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "promotes the decorated pending entry in place"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "appends no duplicate entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "writes no undecorated duplicate"
```

**4. Spec 027's named entries still exist** — each must print `>= 1`:

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "second wikilink in a chain summary is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "prefix task name does not match a longer task"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "prose before the wikilink is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "bare wikilink is the own entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "task name with regex metacharacters matches literally"
```

No new `Entry` description may contain an existing description as a substring — otherwise these greps pass on the new entry alone and stop proving the spec-027 entry survives.

**4b. No existing test was deleted or weakened** (spec 028 AC10) — the deletions column must be `0` for every file:

```
git diff -w --numstat pkg/ops/daily_note_entry_test.go pkg/ops/complete_test.go pkg/ops/defer_test.go pkg/ops/workon_test.go
```

`-w` ignores whitespace-only reflow from `make format`. A non-zero deletions count means an existing `Entry` or `It` was removed or rewritten — restore it. The named greps above pin only 5 of roughly 40 existing nodes, so this is the check that covers the rest.

**5. Generality — no emoji literal in the implementation:**

```
grep -cE '🐟|🔄|🔧|🏠|📈|🔒|⚠️' pkg/ops/daily_note_entry.go
```

Must print `0`. (`grep -c` exits 1 when the count is 0 — that non-zero exit is the expected, passing result here. If it prints anything above `0`, an emoji leaked into the code or a comment; remove it.)

Also confirm no cutset crept in — this must print nothing:

```
grep -n 'TrimLeft\|Cutset\|cutset' pkg/ops/daily_note_entry.go
```

**6. Docs and changelog:**

```
grep -n -i 'decoration\|emoji' docs/daily-notes.md
grep -n -A25 '^## Unreleased' CHANGELOG.md
```

The first must print at least one line. The second must show a new `- fix:` bullet mentioning decorated / emoji-prefixed daily-note entries, and must still show the two pre-existing bullets.

**7. Coverage — both functions at 100%:**

```
go test -coverprofile=/tmp/cover.out ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep -E 'IsOwnDailyNoteEntry|trimLeadingDecoration'
```

Both lines must report `100.0%`.

**8. Full gate, once, at the end:**

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make format`, etc.) until it passes, then run `make precommit` one final time. `make format` runs golines at a 100-column limit and will reflow the long emoji fixtures — let it.
</verification>
