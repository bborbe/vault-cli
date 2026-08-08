---
status: completed
spec: [027-bug-daily-note-substring-shadowing]
summary: Added IsOwnDailyNoteEntry helper in pkg/ops/daily_note_entry.go with full test coverage, daily-notes.md contract doc, and Unreleased changelog entry
execution_id: vault-cli-exec-171-spec-027-daily-note-own-entry-matcher
dark-factory-version: v0.192.9
created: "2026-08-08T11:20:00Z"
queued: "2026-08-08T11:39:42Z"
started: "2026-08-08T11:39:43Z"
completed: "2026-08-08T11:42:29Z"
branch: dark-factory/bug-daily-note-substring-shadowing
---

# Add the shared daily-note own-entry matcher

<summary>
- vault-cli gains one shared rule for deciding whether a daily-note checkbox line is a given task's own entry.
- A line counts as the task's own entry only when it starts with a link to that task; prose after the link is allowed.
- A line that merely mentions the task somewhere in the middle — a chain summary or a rollup — is never treated as that task's entry.
- Alias and heading link forms still resolve to the same task when they lead the line.
- Matching stays case-insensitive, so a lowercase task name still finds its entry.
- A task name that is a prefix of a longer task's name no longer matches the longer task's entry.
- Task names containing punctuation are compared literally, so nothing is ever treated as a search pattern.
- The daily-note entry contract is written down so vault authors know what shape a task entry must have and which commands read it back.
- No command behavior changes in this prompt — the three affected commands are wired to the new rule in the two follow-up prompts.
</summary>

<objective>
Add one shared, directly-tested helper that decides whether a daily-note checkbox line is a given task's own entry, and document the contract it enforces — so that `task complete`, `task defer`, and `task work-on` can stop using case-insensitive substring containment (which cannot tell a task's own entry apart from a line that merely mentions it) and instead share a single identity rule that cannot drift apart again.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

Read these files fully before making changes:

- `/workspace/pkg/storage/base.go` — the checkbox parser this helper consumes. Verified, do not change:
  ```go
  // CheckboxRegex matches a Markdown checkbox line with either `-` or `*` as
  // the list marker. Capture groups: 1=leading whitespace, 2=state (` `, `x`,
  // or `/`), 3=task text.
  CheckboxRegex = regexp.MustCompile(`^(\s*)[-*] \[([ x/])\] (.+)$`)
  ```
  The helper you write takes **capture group 3** (the text after the `- [x] ` marker), not the whole line.
- `/workspace/pkg/ops/workon.go` — `appendTaskToDaily` writes a task's entry as exactly:
  ```go
  newLine := fmt.Sprintf("- [/] [[%s]]", taskName)
  ```
  and `/workspace/pkg/ops/defer.go` — `addToDailyNote` writes:
  ```go
  taskLine := fmt.Sprintf("- [ ] [[%s]]", taskName)
  ```
  The wikilink is the identity of a task's own entry. The new helper must accept exactly what these two write.
- `/workspace/pkg/ops/complete.go` (`updateDailyNote`), `/workspace/pkg/ops/defer.go` (`removeFromDailyNote`), `/workspace/pkg/ops/workon.go` (`findAndUpdateTaskCheckbox`) — the three current call sites, all using the same broken condition:
  ```go
  if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
  ```
  Read them to understand the shape the helper must slot into, but **do not modify them in this prompt** — prompts 2 and 3 own those edits.
- `/workspace/pkg/domain/task_phase_test.go` — the `DescribeTable` / `Entry` style this project uses for table tests (see the `DescribeTable("valid phases", ...)` block).
- `/workspace/pkg/ops/ops_suite_test.go` — every test file in `pkg/ops` is `package ops_test` (external test package). There is no internal test file, so the helper must be **exported** to be directly testable.
- `/workspace/docs/task-writing.md` — an existing `docs/` page, for tone and heading style. `/workspace/docs/daily-notes.md` does **not** exist yet; you are creating it.
- `/workspace/CHANGELOG.md` — top versioned section is `## v0.102.4`; there is currently no `## Unreleased` section.

Read these coding-plugin docs (they are in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style for the exported helper.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>
1. Create `/workspace/pkg/ops/daily_note_entry.go` (package `ops`) with the standard BSD license header used by every other file in the package. Add exactly one exported function:

   ```go
   // IsOwnDailyNoteEntry reports whether checkboxText is the daily-note entry
   // that belongs to taskName.
   //
   // checkboxText is capture group 3 of storage.CheckboxRegex — everything after
   // the `- [x] ` marker on a daily-note checkbox line.
   //
   // A checkbox line is a task's own entry iff its text, after trimming leading
   // whitespace, begins with a wikilink resolving to that task. Trailing prose
   // after the wikilink does not disqualify it, so
   // `[[Some Task]] — due today` is Some Task's own entry. A wikilink to the
   // task appearing anywhere else in the text is a mention, never an own entry,
   // so `Chain — [[Other]] → [[Some Task]].` is not Some Task's own entry.
   //
   // Alias (`[[Task|label]]`) and heading (`[[Task#Section]]`) link forms
   // resolve to the same task. Comparison is case-insensitive and literal —
   // the task name is never interpreted as a pattern.
   func IsOwnDailyNoteEntry(checkboxText string, taskName string) bool
   ```

2. Implement `IsOwnDailyNoteEntry` with plain string operations only — **no `regexp`**, so task names containing `(`, `)`, `+`, `[`, `*` are compared literally and can never panic or match as a pattern. Use exactly this sequence:
   1. Trim leading whitespace from `checkboxText` (`strings.TrimLeft(checkboxText, " \t")`).
   2. If the result does not have the prefix `"[["`, return `false`.
   3. Take the text after `"[["`; find the first `"]]"` with `strings.Index`. If there is none, return `false`.
   4. The link target is everything before that `"]]"`.
   5. Strip an alias suffix first: if the target contains `"|"`, keep only the part before the first `"|"`.
   6. Then strip a heading suffix: if the remaining target contains `"#"`, keep only the part before the first `"#"`.
      Order matters — `[[Task#Head|Label]]` is the canonical Obsidian form, and cutting `"|"` first also keeps `[[Task|La#bel]]` correct.
   7. Return `strings.EqualFold(strings.TrimSpace(target), strings.TrimSpace(taskName))`.

   Full-string equality (not prefix, not containment) is what makes task name `Plan Week` fail to match entry `[[Plan Weekend - 2026W32-sat]]`.

3. Do NOT add any other exported symbol, config field, flag, threshold, or option to this file. The rule is invariant — there is no opt-out.

4. Create `/workspace/pkg/ops/daily_note_entry_test.go` (package `ops_test`, standard license header) with a Ginkgo `Describe("IsOwnDailyNoteEntry")` containing a single `DescribeTable` whose function body is:

   ```go
   func(checkboxText string, taskName string, expected bool) {
       Expect(ops.IsOwnDailyNoteEntry(checkboxText, taskName)).To(Equal(expected))
   }
   ```

   Add these `Entry` cases with **exactly these descriptions** (the verification step greps for them verbatim):

   | Entry description | checkboxText | taskName | expected |
   |---|---|---|---|
   | `bare wikilink is the own entry` | `[[Turn on hell - 2026W32-sat]]` | `Turn on hell - 2026W32-sat` | `true` |
   | `wikilink with trailing prose is the own entry` | `[[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today` | `Turn on hell - 2026W32-sat` | `true` |
   | `prose before the wikilink is a mention` | `🔧 Nuke-reboot chain — [[Turn on hell - 2026W32-sat]].` | `Turn on hell - 2026W32-sat` | `false` |
   | `second wikilink in a chain summary is a mention` | `🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].` | `Turn on hell - 2026W32-sat` | `false` |
   | `alias wikilink is the own entry` | `[[Turn on hell - 2026W32-sat\|hell]] — due today` | `Turn on hell - 2026W32-sat` | `true` |
   | `heading wikilink is the own entry` | `[[Turn on hell - 2026W32-sat#Steps]]` | `Turn on hell - 2026W32-sat` | `true` |
   | `prefix task name does not match a longer task` | `[[Plan Weekend - 2026W32-sat]]` | `Plan Week` | `false` |
   | `task name with regex metacharacters matches literally` | `[[Fix (urgent) c++ build]]` | `Fix (urgent) c++ build` | `true` |
   | `regex metacharacters are not treated as a pattern` | `[[Fix XurgentX cXX build]]` | `Fix (urgent) c++ build` | `false` |
   | `lowercase task name matches a capitalised wikilink` | `[[Turn on hell - 2026W32-sat]]` | `turn on hell - 2026w32-sat` | `true` |
   | `empty checkbox text is not an own entry` | `` (empty string) | `Turn on hell - 2026W32-sat` | `false` |
   | `unterminated wikilink is not an own entry` | `[[Turn on hell - 2026W32-sat` | `Turn on hell - 2026W32-sat` | `false` |
   | `leading whitespace before the wikilink is tolerated` | `  [[Turn on hell - 2026W32-sat]]` | `Turn on hell - 2026W32-sat` | `true` |

   In the Go source the alias entry's `|` is a plain character inside a normal Go string literal — the `\|` above is only table-escaping in this prompt.

5. Add a second `Describe` block named `"IsOwnDailyNoteEntry accepts what the daily-note writers produce"` that closes the write/read contract the spec calls out. For each of the two literal formats the code writes today, build the line, parse it with `storage.CheckboxRegex`, and assert the helper accepts capture group 3.

   **The assertions MUST live inside an `It` node.** Gomega assertions placed directly in a `Describe`/`Context` body run during Ginkgo v2's tree-construction phase and fail the suite with "assertion in the Tree Construction Phase". Write it exactly this shape:

   ```go
   Describe("IsOwnDailyNoteEntry accepts what the daily-note writers produce", func() {
       It("accepts both writer formats", func() {
           for _, line := range []string{
               fmt.Sprintf("- [/] [[%s]]", "Turn on hell - 2026W32-sat"), // workon.appendTaskToDaily
               fmt.Sprintf("- [ ] [[%s]]", "Turn on hell - 2026W32-sat"), // defer.addToDailyNote
           } {
               matches := storage.CheckboxRegex.FindStringSubmatch(line)
               Expect(matches).To(HaveLen(4))
               Expect(ops.IsOwnDailyNoteEntry(matches[3], "Turn on hell - 2026W32-sat")).To(BeTrue())
           }
       })
   })
   ```

   This is the boundary test: it traverses the real parser (`storage.CheckboxRegex`) rather than asserting on a hand-written string, so a future change to the **parser** breaks here. Note it does NOT lock the writer format — the format strings above are copies, and `appendTaskToDaily` / `addToDailyNote` are unexported and unreachable from `package ops_test`. The writer-side contract is covered end-to-end by prompt 3's `WorkOn` regression test instead. Import `"github.com/bborbe/vault-cli/pkg/storage"` in the test file for this.

6. Create `/workspace/docs/daily-notes.md` documenting the daily-note task-entry contract. It must contain, at minimum:
   - The entry shape written verbatim as `[[<taskName>]]` — state that a task's own entry on a daily note is a checkbox line whose text begins with `[[<taskName>]]`, with either `-` or `*` as the list marker, and that trailing prose after the wikilink is allowed.
   - Both polarities of the rule, using the words **own entry** and **mention**: a leading wikilink is the task's *own entry*; a wikilink anywhere else on the line is a *mention* and is never rewritten, never deleted, and never counted as "the task is already tracked".
   - A worked example using the two lines from the spec's reproduction:
     ```markdown
     - [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].
     - [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today
     ```
     labelling the first as a mention and the second as the own entry.
   - Which commands read the contract back, naming `task complete`, `task defer`, and `task work-on` — on at least three separate lines, one per command, describing what each does with an own entry (`complete` flips it to `[x]`; `defer` removes it from today's note and adds one to the target day's note; `work-on` promotes a `[ ]` entry to `[/]`, or adds a new `[/]` entry when the task has none).
   - That alias (`[[<taskName>|label]]`) and heading (`[[<taskName>#Section]]`) forms resolve to the same task, and that matching is case-insensitive.

7. Add a `## Unreleased` bullet to `/workspace/CHANGELOG.md` immediately after implementing, before running `make precommit`. There is currently no `## Unreleased` section — create it **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` bullets) and **above** the `## v0.102.4` heading, never between the `# Changelog` title and the preamble. Add this bullet:

   ```
   ## Unreleased

   - docs: document the daily-note task-entry contract in `docs/daily-notes.md` — a task's own entry is a checkbox line beginning with `[[<taskName>]]`, while a wikilink elsewhere on the line is a mention
   ```

8. Coverage: `pkg/ops` is the changed package. `IsOwnDailyNoteEntry` must reach 100% statement coverage from the table in requirement 4 — every early return (`no [[` prefix, no closing `]]`) and every branch (alias cut, heading cut, equal, not equal) has a dedicated Entry above. Do not add retroactive coverage to unrelated untested `pkg/ops` code.
</requirements>

<constraints>
- Do NOT modify `/workspace/pkg/ops/complete.go`, `/workspace/pkg/ops/defer.go`, or `/workspace/pkg/ops/workon.go` in this prompt. Prompt 2 wires `complete` and `defer`; prompt 3 wires `work-on`. This prompt adds the helper, its tests, and the doc only.
- `storage.CheckboxRegex`, `storage.CheckboxCompleteRegex`, and `storage.CheckboxUncompleteRegex` in `/workspace/pkg/storage/base.go` keep their current semantics. This change is about how a task is *identified* within a checkbox line, not how the line is *parsed*. Do not edit `/workspace/pkg/storage/base.go`.
- `/workspace/pkg/ops/workon.go` keeps writing entries as `- [/] [[<taskName>]]`; the new matcher must accept exactly what `workon` writes (requirement 5 locks this).
- Out of scope, do not change: goal-file checkbox matching in `markGoalCheckbox` (`/workspace/pkg/ops/complete.go`) and in `/workspace/pkg/ops/update.go`. Those apply containment to a different surface with no wikilink contract and are explicitly excluded by the spec.
- Do NOT add a flag, config key, or environment variable to select the old containment behavior. The identity rule is invariant.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` in `pkg/ops/`.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`. (The new helper returns a plain `bool` and produces no errors.)
- Do NOT hand-bump or tag any version. Do NOT touch `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — the release agent owns those. Add the `## Unreleased` bullet only.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0.

Confirm every required table case is present and named as specified (each grep must print at least one line):

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "bare wikilink is the own entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "second wikilink in a chain summary is a mention"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "prefix task name does not match a longer task"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "task name with regex metacharacters matches literally"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "lowercase task name matches a capitalised wikilink"
```

Each must print a number `>= 1`.

Confirm the doc contract landed:

```
grep -c '\[\[<taskName>\]\]' docs/daily-notes.md
grep -n -i 'own entry' docs/daily-notes.md
grep -n -i 'mention' docs/daily-notes.md
grep -c 'complete\|defer\|work-on' docs/daily-notes.md
```

The first must be `>= 1`, the two middle greps must each print at least one line, the last must be `>= 3`.

Confirm the changelog entry landed:

```
grep -A20 '^## Unreleased' CHANGELOG.md
```

Must show a line starting with `- docs:`.

Coverage for the new helper:

```
go test -coverprofile=/tmp/cover.out ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep IsOwnDailyNoteEntry
```

Must report `100.0%`.

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
