---
status: completed
spec: [027-bug-daily-note-substring-shadowing]
summary: Replaced last substring-containment matcher in findAndUpdateTaskCheckbox with IsOwnDailyNoteEntry, wiring task work-on through the shared own-entry matcher shared with complete and defer; added three regression test contexts covering mention-only notes, mention above pending own entry, and mention above already-in-progress own entry; consolidated changelog bullet to name all three fixed commands
execution_id: vault-cli-exec-173-spec-027-workon-own-entry
dark-factory-version: v0.192.9
created: "2026-08-08T11:20:00Z"
queued: "2026-08-08T11:39:42Z"
started: "2026-08-08T11:45:27Z"
completed: "2026-08-08T11:47:34Z"
branch: dark-factory/bug-daily-note-substring-shadowing
---

# Wire task work-on to the own-entry matcher and close out spec 027

<summary>
- `vault-cli task work-on` now adds the task to today's daily note when the note only mentions the task in a narrative line, instead of silently concluding the task is already tracked.
- A chain-summary or rollup line that references the task is never mistaken for the task's own entry and is left byte-identical.
- When the task does have its own entry, the existing promotion rule is unchanged: a pending entry becomes in-progress, and an entry that is already in-progress or completed is left alone.
- All three daily-note commands — complete, defer, and work-on — now resolve a task's entry through the same shared rule, so they cannot drift apart again.
- The old substring-containment matching is fully gone from the operations package.
- The changelog carries one entry describing the fix across all three commands.
</summary>

<objective>
Replace the last case-insensitive substring containment in `findAndUpdateTaskCheckbox` (`pkg/ops/workon.go`) with the shared `ops.IsOwnDailyNoteEntry` matcher — so `task work-on` stops treating a narrative line that merely mentions the task as proof the task is already tracked, and actually adds the task's own entry to today's daily note when none exists. This is the last of the three call sites; after it, no daily-note path in `pkg/ops` uses containment.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done (test coverage rules, CHANGELOG placement rules).

**Precondition — prompts 1 and 2 of this spec must have shipped.** Verify before starting:

```
grep -n "func IsOwnDailyNoteEntry" pkg/ops/daily_note_entry.go
grep -n "IsOwnDailyNoteEntry" pkg/ops/complete.go pkg/ops/defer.go
```

If either returns nothing, STOP: do not invent the helper and do not convert `complete` or `defer` here. Report `status: failed` with the message `"IsOwnDailyNoteEntry / complete+defer wiring not yet deployed (prompts 1-2 of spec 027)"`.

Read these files fully before making changes:

- `/workspace/pkg/ops/daily_note_entry.go` — the shared matcher. Verified signature:
  ```go
  func IsOwnDailyNoteEntry(checkboxText string, taskName string) bool
  ```
  It takes capture group 3 of `storage.CheckboxRegex` (the text after the `- [x] ` marker), not the whole line.
- `/workspace/pkg/ops/workon.go` — read fully. The function to change is `findAndUpdateTaskCheckbox`; its current body is:
  ```go
  func findAndUpdateTaskCheckbox(lines []string, taskName string) (found, modified bool) {
      for i, line := range lines {
          if matches := storage.CheckboxRegex.FindStringSubmatch(line); len(
              matches,
          ) == 4 { //nolint:nestif
              taskText := matches[3]
              if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
                  found = true
                  state := matches[2]
                  // Only update if currently [ ] (pending)
                  if state == " " {
                      marker := matches[1]
                      lines[i] = strings.Replace(line, marker+" [ ]", marker+" [/]", 1)
                      modified = true
                  }
                  // If already [/] or [x], skip (already in-progress or completed)
                  break
              }
          }
      }
      return found, modified
  }
  ```
  Its caller `updateDailyNote` uses `found` to decide whether to call `appendTaskToDaily`, which writes `fmt.Sprintf("- [/] [[%s]]", taskName)` after the `## Must` header (or at end of file when there is none).
- `/workspace/pkg/ops/complete.go` and `/workspace/pkg/ops/defer.go` — already converted by prompt 2. Read the converted call sites so the `work-on` conversion matches their shape. Do not change them again.
- `/workspace/pkg/ops/workon_test.go` — read fully. Its `BeforeEach` sets `taskName = "my-task"` and builds `task` via `domain.NewTask(map[string]any{"status": "todo"}, domain.FileMetadata{Name: taskName, FilePath: "/path/to/vault/tasks/my-task.md"}, domain.Content(""))`; `JustBeforeEach` calls `workOnOp.Execute(...)`. Existing daily-note contexts stub with `mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)` and read the write via `mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)`. Every existing daily-note fixture already uses wikilink entries (`- [ ] [[my-task]]`, `* [ ] [[my-task]]`, `- [/] [[my-task]]`, `- [x] [[my-task]]`) so none of them need fixture correction. The file does **not** currently import `"strings"`.
- `/workspace/docs/daily-notes.md` — the entry contract written by prompt 1.
- `/workspace/CHANGELOG.md` — the `## Unreleased` section created by prompt 1 and appended to by prompt 2.

Read these coding-plugin docs (they are in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.
</context>

<requirements>
1. In `/workspace/pkg/ops/workon.go`, inside `findAndUpdateTaskCheckbox`, replace the containment condition with the shared matcher:

   old:
   ```go
   if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
   ```
   new:
   ```go
   if IsOwnDailyNoteEntry(taskText, taskName) {
   ```

   Change nothing else in the function. In particular:
   - **Keep the `break`.** `work-on` promotes at most one entry; a task's own entry is expected to appear once, and stopping at the first own entry is correct here. Only `complete` needed its break removed (it must update every own entry), and that was done in prompt 2.
   - Keep the `state == " "` guard and the `marker := matches[1]` / `strings.Replace` promotion exactly as written. `work-on`'s existing state rule is unchanged: only a `[ ]` entry is promoted to `[/]`; entries already `[/]` or `[x]` are left alone, and `found` is still set to `true` for them so no duplicate entry is appended. The `marker` variable holds capture group 1 (leading whitespace) rather than the list marker; that is pre-existing and out of scope for this spec — do not "fix" it here.
   - Keep the `//nolint:nestif` directive and the `(found, modified bool)` named-return signature.

2. If `strings` becomes unused in `/workspace/pkg/ops/workon.go` after step 1, do NOT remove the import blindly — `findAndUpdateTaskCheckbox` still uses `strings.Replace` and `appendTaskToDaily` uses `strings.Contains`, so it stays. Confirm with `grep -n 'strings\.' pkg/ops/workon.go` rather than guessing.

3. In `/workspace/pkg/ops/workon_test.go`, add `Context("updateDailyNote when the note holds only a mention")` — this is the direct reproduction of spec 027's Bug C. Its `BeforeEach` must override the task identity and stub the daily note:

   ```go
   taskName = "Turn on hell - 2026W32-sat"
   task = domain.NewTask(
       map[string]any{"status": "todo"},
       domain.FileMetadata{
           Name:     taskName,
           FilePath: "/path/to/vault/tasks/Turn on hell - 2026W32-sat.md",
       },
       domain.Content(""),
   )
   mockTaskStorage.FindTaskByNameReturns(task, nil)

   dailyContent := "## Must\n" +
       "- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].\n"
   mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
   mockDailyNoteStorage.WriteDailyNoteReturns(nil)
   ```

   Add these `It` blocks, each asserting `mockDailyNoteStorage.WriteDailyNoteCallCount()` is `1` and reading `_, _, _, content := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)`:
   - `It("adds the task's own entry")` — assert `content` contains `- [/] [[Turn on hell - 2026W32-sat]]`.
   - `It("leaves the mention line byte-identical")` — assert `content` contains `- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].` character for character.
   - `It("adds exactly one entry")` — assert `strings.Count(content, "[[Turn on hell - 2026W32-sat]]")` equals `2` (one occurrence inside the surviving mention line, one in the newly added own entry).

   Add the `"strings"` import to `/workspace/pkg/ops/workon_test.go` (it is not currently imported).

4. In `/workspace/pkg/ops/workon_test.go`, add `Context("updateDailyNote with a mention line above a pending own entry")`. Same identity override as requirement 3, with:

   ```go
   dailyContent := "## Must\n" +
       "- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].\n" +
       "- [ ] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today\n"
   ```

   Add:
   - `It("promotes the own entry to in-progress")` — assert `content` contains `- [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today`.
   - `It("leaves the mention line byte-identical")` — same assertion as requirement 3.
   - `It("does not append a second entry")` — assert `strings.Count(content, "[[Turn on hell - 2026W32-sat]]")` equals `2` (the mention plus the single promoted own entry).

   Without the fix this context fails on the first `It`: the mention line matches first, `found` becomes `true`, the loop breaks, and the pending own entry is never promoted.

5. In `/workspace/pkg/ops/workon_test.go`, add `Context("updateDailyNote with a mention line above an already in-progress own entry")`. Same identity override, with:

   ```go
   dailyContent := "## Must\n" +
       "- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].\n" +
       "- [/] [[Turn on hell - 2026W32-sat]]\n"
   ```

   Add `It("leaves the note untouched")` asserting `err` is `nil` and `mockDailyNoteStorage.WriteDailyNoteCallCount()` equals `0`. This locks the constraint that `work-on`'s state rule is unchanged: an entry already `[/]` is neither re-promoted nor duplicated.

6. Confirm the containment matcher is fully gone from the operations package. After the edit in requirement 1:

   ```
   grep -rn 'strings.Contains(strings.ToLower(taskText)' pkg/ops/
   ```

   must return zero lines, and `IsOwnDailyNoteEntry` must appear in `pkg/ops/complete.go`, `pkg/ops/defer.go`, and `pkg/ops/workon.go`. If any of the three is missing, a prior prompt did not land — do not patch it here; report `status: failed` naming the missing file.

7. Consolidate the `## Unreleased` fix bullet in `/workspace/CHANGELOG.md` so a single entry names all three fixed paths, per spec 027's acceptance criterion. Prompt 2 added exactly this bullet:

   ```
   - fix: `task complete` and `task defer` now act on a task's own daily-note entry — a checkbox line beginning with `[[<taskName>]]` — instead of any line whose text merely contains the task name; `complete` updates every own entry rather than stopping at the first, and `defer` no longer deletes a chain-summary line that only mentions the task
   ```

   **Replace that one bullet verbatim** with the consolidated bullet below. Leave prompt 1's `- docs:` bullet untouched, keep exactly one `## Unreleased` heading, and do not touch any `## vX.Y.Z` section. If prompt 2's bullet is not found verbatim, do NOT add the consolidated bullet alongside it — that would leave two overlapping `- fix:` entries; report `status: failed` instead.

   ```
   - fix: `task complete`, `task defer`, and `task work-on` now act on a task's own daily-note entry — a checkbox line whose text begins with `[[<taskName>]]` — instead of any line whose text merely contains the task name. `complete` updates every own entry rather than stopping at the first, `defer` no longer deletes a chain-summary line that only mentions the task, and `work-on` now adds an entry when the note only mentions the task
   ```

8. Coverage: the changed package is `pkg/ops`. `findAndUpdateTaskCheckbox` must be exercised along the own-entry-found-and-promoted path (requirement 4), the own-entry-found-but-not-pending path (requirement 5), and the not-found path that triggers `appendTaskToDaily` (requirement 3). The existing contexts covering asterisk markers, `[x]` entries, empty notes, and read/write errors must keep passing untouched. Do not add retroactive coverage to unrelated untested `pkg/ops` code.
</requirements>

<constraints>
- Do NOT modify `/workspace/pkg/ops/daily_note_entry.go`, `/workspace/pkg/ops/complete.go`, or `/workspace/pkg/ops/defer.go` — prompts 1 and 2 own those. If a test here appears to need a matcher change, that is a signal the test fixture is wrong, not the matcher.
- Do NOT modify `/workspace/docs/daily-notes.md` — prompt 1 owns it. The verification block greps that file; if a grep fails, report `status: failed` naming the failing check. Do not edit the doc to make your own verification pass.
- Keep the `break` in `findAndUpdateTaskCheckbox`. Only `complete` needed its break removed.
- `work-on`'s existing state rule is unchanged: only a `[ ]` entry is promoted to `[/]`; entries already `[/]` or `[x]` are left alone.
- `/workspace/pkg/ops/workon.go` keeps writing entries as `- [/] [[<taskName>]]` in `appendTaskToDaily`. Do not change that format, the `## Must` insertion point, or the end-of-file fallback.
- `storage.CheckboxRegex`, `storage.CheckboxCompleteRegex`, and `storage.CheckboxUncompleteRegex` (`/workspace/pkg/storage/base.go`) keep their current semantics. This fix changes how a task is *identified* within a checkbox line, not how the line is *parsed*. Do not edit `/workspace/pkg/storage/base.go`.
- List-marker preservation stays intact — an entry written with `*` stays `*`, one with `-` stays `-`. The existing asterisk-marker contexts in `workon_test.go` must keep passing untouched.
- Out of scope, do not change: goal-file checkbox matching in `markGoalCheckbox` (`/workspace/pkg/ops/complete.go`) and in `/workspace/pkg/ops/update.go`. Those apply containment to a different surface with no wikilink contract and are explicitly excluded by the spec.
- Out of scope, do not change: the pre-existing `marker := matches[1]` naming in `findAndUpdateTaskCheckbox` (capture group 1 is leading whitespace, not the list marker). It is unrelated to this spec's identity fix.
- Do NOT add a flag, config key, or environment variable to select the old containment behavior. The identity rule is invariant.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` in `pkg/ops/`.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT hand-bump or tag any version. Do NOT touch `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — the release agent owns those. Edit the `## Unreleased` section only.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`:

```
make test
```

Must exit 0, including every existing `pkg/ops` daily-note context and the new ones.

Confirm containment is gone from the whole operations package (this exits 0 on success; a bare `grep` with no matches exits 1, which is NOT a failure to chase):

```
! grep -rq 'strings.Contains(strings.ToLower(taskText)' pkg/ops/
```

Confirm all three call sites resolve through the one shared helper (each must print at least one line):

```
grep -n "IsOwnDailyNoteEntry" pkg/ops/complete.go
grep -n "IsOwnDailyNoteEntry" pkg/ops/defer.go
grep -n "IsOwnDailyNoteEntry" pkg/ops/workon.go
```

Confirm `work-on` kept its break:

```
sed -n '/^func findAndUpdateTaskCheckbox/,/^}/p' pkg/ops/workon.go | grep -c "break"
```

Must print `1`.

Confirm the new regression contexts ran (each must print a number `>= 1`):

```
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "adds the task's own entry"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "promotes the own entry to in-progress"
go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "does not append a second entry"
```

Confirm the daily-note contract doc is still in place:

```
grep -c '\[\[<taskName>\]\]' docs/daily-notes.md
grep -c 'complete\|defer\|work-on' docs/daily-notes.md
```

First must be `>= 1`, second `>= 3`.

Confirm the consolidated changelog bullet landed under a single `## Unreleased` heading:

```
grep -c '^## Unreleased' CHANGELOG.md
grep -A20 '^## Unreleased' CHANGELOG.md
```

The first must print `1`; the second must show one `- fix:` line naming `task complete`, `task defer`, and `task work-on`. Mechanically, exactly one consolidated bullet must exist:

```
grep -c '^- fix:.*daily-note' CHANGELOG.md
```

Must print `1` — a `2` means prompt 2's bullet was appended to rather than replaced.

Confirm no existing assertion was deleted or weakened (spec AC8) — the deletions column must be `0`:

```
git diff --numstat pkg/ops/workon_test.go
```

If the deletions count is non-zero, an existing test was removed or rewritten; restore it before finishing.

Coverage for the changed package:

```
go test -coverprofile=/tmp/cover.out ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep -E "findAndUpdateTaskCheckbox|appendTaskToDaily"
```

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
