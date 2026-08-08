---
status: completed
approved: "2026-08-08T11:09:51Z"
generating: "2026-08-08T11:10:57Z"
prompted: "2026-08-08T11:26:37Z"
verifying: "2026-08-08T11:47:34Z"
completed: "2026-08-08T12:03:23Z"
branch: dark-factory/bug-daily-note-substring-shadowing
---

## Summary

- Daily-note checkbox lines are matched by case-insensitive **substring containment** of the task name, not by the task's own wikilink.
- A prose/summary checkbox line that merely *mentions* a task is therefore indistinguishable from that task's own entry.
- All three daily-note operations are affected, each failing differently: `complete` leaves the real entry stale, `defer` **deletes** the mentioning line and everything on it, `work-on` silently never adds an entry at all.
- Every one of the three reports success. The damage is only visible by reading the note afterwards.
- One root cause, three call sites, one shared identity rule to fix it.

## Problem

Daily notes routinely contain narrative checkbox lines that reference several tasks at once — chain summaries, rollups, "blocked by" notes. This is ordinary vault usage, not an edge case. Because the matcher only asks "does this line's text contain the task name?", such a line is indistinguishable from the task's own entry. Worse, every affected path stops at the first match or acts on all matches indiscriminately, so a single mentioning line positioned earlier in the note is enough to break the operation. The operator gets a green checkmark either way and discovers the wrong state — or the missing prose — days later.

## Goal

Daily-note operations act on a task's **own** checkbox entry and never on a line that merely mentions the task. `complete` flips the right entry, `defer` removes the right entry and leaves narrative lines intact, `work-on` adds an entry when none exists. All three agree on what "the task's own entry" means because they share one matcher.

## Reproduction

All three bugs reproduced 2026-08-08 against `vault-cli` at commit `aa43847` (master), built to `/tmp/vc-repro`, run against a throwaway vault seeded from `example/` — no live vault data involved.

Shared fixture — daily note (`Daily Notes/<today>.md`), a mention line followed by the task's own entry:

```markdown
## Must
- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].
- [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today
```

### Bug A — `complete` leaves the real entry stale

```bash
vault-cli --config "$W/config.yaml" task complete "Turn on hell - 2026W32-sat"
```

Output, verbatim:

```
✅ Task completed: Turn on hell - 2026W32-sat
```

`diff` of the note before vs after: **empty**. The note is byte-identical — the own entry is still `[/]`. The task file itself was updated correctly (`status: completed`), isolating the defect to the daily-note path.

### Bug B — `defer` deletes the mentioning line

Same fixture, reset between runs:

```bash
vault-cli --config "$W/config.yaml" task defer "Turn on hell - 2026W32-sat" 2026-08-09
```

Output, verbatim:

```
📅 Task deferred to 2026-08-09: Turn on hell - 2026W32-sat
```

`diff` before vs after, verbatim:

```
6,7d5
< - [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].
< - [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today
```

**Both** lines are gone. The summary line took `[[Shutdown K3s - 2026W32-sat]]`'s `[x]` completion state and its prose with it. Exit 0, no warning.

### Bug C — `work-on` never adds an entry

Fixture reduced to the mention line only, with **no** own entry present:

```markdown
## Must
- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].
```

```bash
vault-cli --config "$W/config.yaml" task work-on "Turn on hell - 2026W32-sat"
```

Output, verbatim:

```
✅ Now working on: Turn on hell - 2026W32-sat (assigned to alice)
```

`diff` before vs after: **empty**. No entry was added. `findAndUpdateTaskCheckbox` matched the mention line, returned `found = true`, and broke — so the caller concluded the task was already tracked. The task never appears on the daily note at all.

## Expected vs Actual

| | `complete` (A) | `defer` (B) | `work-on` (C) |
|---|---|---|---|
| **Expected** | Own entry flips `[/]` → `[x]`; mention untouched | Own entry removed; mention untouched | Own entry added (or an existing one set `[/]`); mention untouched |
| **Actual** | First *containing* line is rewritten — usually an already-`[x]` mention, so a no-op — then `break`; own entry stays `[/]` | Every *containing* line is deleted, mention included, with no `break` | Mention matches, `found = true`, `break`; no entry is ever added |

Expected behavior derives from `pkg/ops/workon.go:278`, which writes a task's entry as exactly `- [/] [[<taskName>]]`. The wikilink is the identity of a task's own entry, so that is what all three paths must match on.

## Why this is a bug

Writing and matching disagree. `workon` *writes* entries in a known shape (`- [/] [[<taskName>]]`) but all three paths *read* them back with substring containment — a strictly weaker test that the written shape does not require. Any line satisfying the weaker test is treated as the entry. The module does not recognise its own output format, which is a contract violation independent of any vault convention.

Three properties turn that mismatch into silent damage rather than a visible error:

- `complete` and `work-on` break on first match, so a false positive doesn't add work — it *prevents* the correct line from ever being examined.
- `defer` deletes on match with no `break`, so a false positive destroys content that is unrecoverable from vault state.
- All three report success regardless, so nothing surfaces the failure at the call site.

Root-cause hypothesis (triage, not the fix) — one expression, three call sites, all consuming `storage.CheckboxRegex` (`pkg/storage/base.go:30`) whose capture group 3 is the full post-checkbox text including surrounding prose:

```
pkg/ops/complete.go:360   updateDailyNote          — containment, break on first
pkg/ops/defer.go:210      removeFromDailyNote      — containment, drops line, no break
pkg/ops/workon.go:251     findAndUpdateTaskCheckbox — containment, sets found, break on first
```

## Workaround

Until the fix ships:

- Avoid `vault-cli task defer` on any day whose note contains a chain-summary or rollup line — it will delete that line. This is the only path that destroys data.
- After any `complete` / `work-on`, re-read the daily note rather than trusting the success message.
- Deleted prose is recoverable only from the vault's git history (`git log -p "60 Periodic Notes/Daily/<date>.md"`), which obsidian-git commits on a schedule — recovery is possible but not guaranteed to be immediate.

## Acceptance Criteria

Every fixture below uses these two verbatim lines from the Reproduction — the own entry carries trailing prose and the mention line carries a second wikilink, because that is the shape that actually breaks:

```
- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Turn on hell - 2026W32-sat]].
- [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today
```

- [ ] `complete` flips the own entry and leaves the mention byte-identical — evidence: a Ginkgo test using the fixture above asserts, after `Complete`, that the note contains `- [x] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today` and still contains the mention line unchanged, character for character.
- [ ] `complete` updates *every* own entry, not just the first — evidence: a second Ginkgo test whose fixture carries the mention line plus the own entry **twice** asserts, after `Complete`, that both own-entry lines read `- [x] …` and the mention line is unchanged. This is what gates DB5's removal of break-on-first; without it a retained `break` passes every other AC.
- [ ] `defer` removes the own entry and leaves the mention byte-identical — evidence: a Ginkgo test using the same fixture asserts, after `Defer`, that the mention line is still present unchanged and that no line beginning `- [/] [[Turn on hell` remains.
- [ ] `work-on` adds an own entry when only a mention is present — evidence: a Ginkgo test whose fixture is the mention line alone asserts, after `WorkOn`, that the note gained a line matching `- [/] [[Turn on hell - 2026W32-sat]]` and that the mention line is unchanged.
- [ ] All three call sites resolve the own entry through one shared helper — evidence: `grep -rn 'strings.Contains(strings.ToLower(taskText)' pkg/ops/` returns 0 lines, and the helper's identifier appears in `pkg/ops/complete.go`, `pkg/ops/defer.go`, and `pkg/ops/workon.go` (one `grep -n` per file, each ≥1 hit). The identifier is fixed by prompt 1 and substituted into this command at verification time — it is deliberately unbound at spec-write time.
- [ ] The shared helper has direct unit tests covering the discriminating cases — evidence: `go test ./pkg/ops/... -v -ginkgo.v` output contains a named assertion for each of: bare `[[X]]` → own entry; `[[X]]` + trailing prose → own entry; prose + `[[X]]` → mention; `[[Y]] → [[X]].` → mention; `[[X|Label]]` → own entry; `[[X#Heading]]` → own entry; task name `Plan Week` against line `[[Plan Weekend - 2026W32-sat]]` → no match; task name containing `(` and `+` → matched literally, no panic.
- [ ] Matching stays case-insensitive — evidence: a unit test asserts task name `turn on hell - 2026w32-sat` matches entry `[[Turn on hell - 2026W32-sat]]`, preserving today's behavior and staying consistent with case-insensitive task-file resolution at `pkg/storage/base.go:127`.
- [ ] No existing daily-note behavior regresses — evidence: `make test` exits 0, and `git diff --stat pkg/ops/complete_test.go pkg/ops/defer_test.go pkg/ops/workon_test.go` shows insertions only, with no existing assertion deleted or weakened.
- [ ] `make precommit` exits 0.
- [ ] The daily-note entry contract is documented in `docs/daily-notes.md` — evidence, all four must hold: `grep -c '\[\[<taskName>\]\]' docs/daily-notes.md` ≥1 (entry shape); `grep -n -i 'own entry' docs/daily-notes.md` ≥1 **and** `grep -n -i 'mention' docs/daily-notes.md` ≥1 (both polarities of the rule); `grep -c 'complete\|defer\|work-on' docs/daily-notes.md` ≥3 (which commands read it back).
- [ ] `CHANGELOG.md` has a bullet under `## Unreleased` naming all three fixed paths — evidence: `grep -n -A20 '^## Unreleased' CHANGELOG.md` shows a line mentioning daily-note matching.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint + format + generate + test + checks, exits 0
- `make test` — unit suite passes
- `grep -rn 'strings.Contains(strings.ToLower(taskText)' pkg/ops/` — returns 0 lines
- `grep -rn '<helper identifier from prompt 1>' pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go` — ≥1 hit in each file
- `grep -n -i 'own entry\|mention' docs/daily-notes.md` — returns ≥2 lines
- `grep -n -A20 '^## Unreleased' CHANGELOG.md` — shows the new bullet

### Operator-executable (runs on the host after the fix lands)

- Rebuild to `/tmp`, seed a throwaway vault from `example/` with the Reproduction fixture, and replay all three repros: A flips the own entry and preserves the mention; B removes only the own entry; C adds an entry. Compare `diff` before/after in each case.

## Desired Behavior

1. A checkbox line is a task's **own entry** iff its checkbox text, after trimming leading whitespace, **begins with** a wikilink resolving to that task. Trailing prose after the wikilink does not disqualify it: `- [/] [[X]] — due today` is X's own entry.
2. A wikilink to the task appearing anywhere else in the checkbox text is a **mention**, never an own entry: `- [x] Chain — [[Y]] → [[X]].` is not X's own entry.
3. Alias (`[[X|Label]]`) and heading (`[[X#Section]]`) link forms resolve to the same task and are recognised as own entries when they lead the line.
4. Mentions are never rewritten, never deleted, and never counted as "the task is already tracked".
5. `complete` updates every own entry it finds rather than stopping at the first, so a note carrying a duplicate entry ends fully consistent. Removing the break is part of the fix, not scope creep: the break is precisely what makes a shadowing match fatal rather than merely wasteful.
6. `defer` removes every own entry and no mention.
7. `complete`, `defer`, and `work-on` obtain this decision from one shared helper, so the three paths cannot drift apart again.
8. When an operation matches no own entry, existing behavior is preserved: `complete` and `defer` leave the note untouched and still succeed; `work-on` adds a new entry.

## Constraints

- `storage.CheckboxRegex`, `CheckboxCompleteRegex`, and `CheckboxUncompleteRegex` (`pkg/storage/base.go`) keep their current semantics — this fix changes how a task is *identified* within a checkbox line, not how the line is *parsed*.
- `pkg/ops/workon.go` keeps writing entries as `- [/] [[<taskName>]]`; the new matcher must accept exactly what `workon` writes.
- List-marker preservation stays intact — an entry written with `*` stays `*`, one with `-` stays `-` (locked by existing tests around `pkg/ops/complete_test.go:244-269`).
- `work-on`'s existing state rule is unchanged: only a `[ ]` entry is promoted to `[/]`; entries already `[/]` or `[x]` are left alone.
- `pkg/ops/` remains a library layer — no stdout writes, structured results only (`CLAUDE.md` § Key Design Decisions).
- **Out of scope, do not change:** goal-file checkbox matching at `pkg/ops/complete.go:308` and `pkg/ops/update.go:184`. Those apply containment to a different surface with no wikilink contract; they are not covered by this spec's ACs and must be left as-is.
- Do not hand-bump versions or tag — add the `## Unreleased` bullet only.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery |
|---|---|---|---|
| Note has the task's own entry twice | All own entries updated (`complete`) or removed (`defer`) | Fixture with two own entries asserts both changed | None needed — end state is consistent |
| Note has only a mention, no own entry | `complete`/`defer` leave it byte-identical and succeed; `work-on` adds an entry | `git diff` on the note is empty for the first two; AC3 covers the third | None needed |
| Task name contains regex metacharacters (`(`, `)`, `+`, `[`) | Matched literally, never as a pattern | Unit test with a name containing `(` and `+`; a panic or silent non-match here is a new bug | None needed |
| Task name is a strict prefix of another (`Plan Week` vs `Plan Weekend`) | Only the exact task's entry matches | Unit test asserts `Plan Week` does not match `[[Plan Weekend - …]]` | None needed |
| Daily note missing or empty | Return nil, no write, as today | Existing tests cover the empty-content path | None needed |
| Note already corrupted by the old binary (deleted prose, stale `[/]`) | Not repaired by this fix — forward-only | Operator reads the note | `git log -p "<daily note path>"` in the vault repo recovers deleted prose from obsidian-git history |
| Note concurrently edited in Obsidian while a command runs | Unchanged from today — whole-file read-modify-write, last writer wins | Out of scope; recorded so the non-goal is durable | Re-apply the lost edit by hand |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Shared own-entry matcher + its unit tests + `docs/daily-notes.md` | 1, 2, 3, 4 | 6, 7, 10 | — |
| 2 | Wire `complete` to the matcher; drop break-on-first; regression tests (single + duplicate own entry) | 5, 7 | 1, 2 | prompt 1 |
| 3 | Wire `defer` to the matcher; regression test | 6, 7 | 3 | prompt 1 |
| 4 | Wire `work-on` to the matcher; regression test; containment grep clean; CHANGELOG bullet | 7, 8 | 4, 5, 8, 9, 11 | prompts 1, 2, 3 |

Rationale: prompt 1 establishes the single identity rule, locks it with direct unit tests, and writes the contract doc that outlives this spec. Prompts 2–4 are independent consumers, each carrying the regression test for its own path. AC5's zero-containment grep sits on prompt 4 because it cannot pass until the last call site is converted — putting it earlier would fail that prompt's own gate.

Kept as one spec rather than split: the whole point is that the three paths share one identity rule, and splitting the call sites across specs would recreate exactly the drift this fixes. The DB × AC product is above the usual budget, but the change touches a single package plus its tests, and each prompt stays small.

## Verification Result

**Verified:** 2026-08-08T12:03:18Z (HEAD 741f1ad)
**Binary:** `/tmp/vc-verify-741f1ad` (built from 741f1ad), compared against `/tmp/vc-old-aa43847` (pre-fix aa43847)
**Scenario:** All three Reproduction repros replayed against a throwaway vault seeded from `example/` with a `Daily Notes/` dir — each run twice, old binary then new, same fixture, `diff` before/after.
**Evidence:**
- A (`complete`), mention line seeded `[/]` so a false match is visible — old: `4c4 - [/] 🔧 Nuke-reboot chain … → - [x] 🔧 Nuke-reboot chain …` (mention rewritten, own entry left `[/]`); new: `5c5 - [/] [[Turn on hell - 2026W32-sat]] — nuke-reboot chain, due today → - [x] …` (own entry only, mention byte-identical)
- B (`defer`) — old: `4,5d3` (both lines deleted); new: `5d4` (own entry only, chain-summary line survives)
- C (`work-on`) — old: empty diff (no entry added); new: `3a4 > - [/] [[Turn on hell - 2026W32-sat]]`
- Duplicate own entry + `complete` — new flips both (`5,6c5,6`), old flips neither; `complete`/`defer` on a mention-only note are byte-identical; alias/heading/lowercase forms all flip; `*` marker preserved
- `go test ./pkg/ops/... -v -ginkgo.v`: 702/704 passed, all 8 AC6 named assertions present incl. `prefix task name does not match a longer task` and `regex metacharacters are not treated as a pattern`
- `grep -rn 'strings.Contains(strings.ToLower(taskText)' pkg/ops/` → 0 hits; `IsOwnDailyNoteEntry` at complete.go:360, defer.go:210, workon.go:251; `break` removed from `updateDailyNote`
- `make test` exit 0; `make precommit` exit 0 ("ready to commit"); scenario 002 passes identically on both binaries
**Verdict:** PASS
