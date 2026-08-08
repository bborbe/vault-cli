---
status: verifying
approved: "2026-08-08T12:25:23Z"
generating: "2026-08-08T12:25:58Z"
prompted: "2026-08-08T12:41:14Z"
verifying: "2026-08-08T12:57:24Z"
branch: dark-factory/bug-daily-note-decoration-prefix
---

## Summary

- Spec 027 defined a task's own daily-note entry as a checkbox line whose text **begins with** `[[<taskName>]]`.
- Real daily notes routinely prefix the wikilink with a category emoji — `- [ ] 🐟 [[Feed Worms]]`, `- [/] 🔄 [[IBKR Swing Trading - 2026W29-tue]]`.
- Those lines do not begin with `[[`, so the matcher classifies them as mentions and every daily-note operation silently skips them.
- `complete` no-ops, `defer` leaves the entry behind, and `work-on` appends a **duplicate** entry beside the one it failed to recognise.
- 552 entries in the Personal vault's daily notes use this shape, across 24 distinct decoration prefixes (vs 4,649 plain ones) — roughly 11% of all entries.

## Problem

Spec 027 fixed a real bug: daily-note lines were matched by substring containment, so a chain-summary line mentioning a task shadowed or destroyed that task's own entry. The replacement rule — own entry iff the checkbox text *begins with* the task's wikilink — was derived from a reproduction whose entry happened to be a bare `- [/] [[Turn on hell - 2026W32-sat]]`. That fixture was unrepresentative. The vault's actual convention decorates entries with a leading category emoji, and the new rule rejects all of them.

The regression is silent in the same way the original bug was: every command still reports success. `complete` prints `✅ Task completed`, `defer` prints `📅 Task deferred`, and neither touches the note. `work-on` is worse than a no-op — it concludes the task is untracked and appends a second entry, so the note accumulates duplicates.

## Goal

A task's own daily-note entry is recognised whether or not the wikilink is preceded by decoration such as a category emoji, while a wikilink that appears after prose remains a mention. This **widens** what counts as an own entry and narrows nothing: every line spec 027 classified as an own entry stays one, and the prose-before-wikilink mention rule is untouched. 552 live lines change classification, all from mention to own entry.

## Reproduction

Reproduced 2026-08-08 against `vault-cli` at commit `3feb43d` (master, spec 027 completed), built to `/tmp/vc-emoji`, run against a throwaway vault seeded from `example/`. No live vault data involved.

Task file `24 Tasks/Feed Worms.md` exists with `status: in_progress`. Daily note:

```markdown
## Must
- [/] 🐟 [[Feed Worms]]
```

### Bug A — `complete` silently no-ops

```bash
vault-cli --config "$W/config.yaml" task complete "Feed Worms"
```

```
✅ Task completed: Feed Worms
```

`diff` of the note before vs after: **empty**. The entry is still `[/]`.

### Bug B — `defer` leaves the entry behind

Same fixture:

```bash
vault-cli --config "$W/config.yaml" task defer "Feed Worms" 2026-08-09
```

```
📅 Task deferred to 2026-08-09: Feed Worms
```

`diff` before vs after: **empty**. The entry remains on the source day's note.

### Bug C — `work-on` appends a duplicate

Fixture with the entry pending:

```markdown
## Must
- [ ] 🐟 [[Feed Worms]]
```

```bash
vault-cli --config "$W/config.yaml" task work-on "Feed Worms"
```

`diff` before vs after, verbatim:

```
5a6
> - [/] [[Feed Worms]]
```

Resulting note — the real entry is untouched and a second one now sits above it:

```
## Must
- [/] [[Feed Worms]]
- [ ] 🐟 [[Feed Worms]]
```

`findAndUpdateTaskCheckbox` returned `found = false`, so `updateDailyNote` called `appendTaskToDaily`.

### Scale on real data

Counted across `60 Periodic Notes/Daily/*.md` in the Personal vault:

```
entries where a wikilink immediately follows decoration:  552
plain leading-wikilink entries:                          4649
```

Full census — **24 distinct prefixes**, not the handful a casual sample suggests:

```
 148 '🔄'    133 '📈'    105 '🔧'     74 '🐟'     37 '🏠'     12 '🔒'
   6 '💼'      4 '🚀'      4 '🚨'      4 '🎯 **'   4 '🪴'      3 '🐛'
   3 '🚗'      2 '📊'      2 '🍵'      2 '📱'      2 '📚'      1 '⚠️'
   1 '🛡️'      1 '🚲'      1 '**'      1 '🎮'      1 '🌳'      1 '🦐'
```

The six most-frequent cover 509/552 = 92%; a fix validated only against those still leaves 43 lines broken. `**` and `🎯 **` are the non-emoji decoration class.

### Widening measured in the destructive direction

This fix makes previously-unmatched lines *deletable* by `defer`, which is spec 027's Bug B shape in reverse. Measured across the same corpus: **5 lines** move from mention to own entry while carrying a second wikilink in trailing prose, e.g.

```
- [x] 🚨 [[Fix Sun Boot Emergency Mode and Slow Boot]] — analysis done; root cause = aging [[Samsung SSD 840 Pro 256GB]]
```

All 5 are genuinely the leading task's own entry, so deleting the line on `defer` is correct behavior — but the trailing reference goes with it. Recorded in Failure Modes rather than special-cased.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| `complete` on `- [/] 🐟 [[Feed Worms]]` | entry flips to `[x]` | note byte-identical, exit 0 |
| `defer` on the same | entry removed from source day | note byte-identical, exit 0 |
| `work-on` on `- [ ] 🐟 [[Feed Worms]]` | entry promoted to `[/]` in place | duplicate `- [/] [[Feed Worms]]` appended |
| `complete` on `- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s]] → [[Feed Worms]].` | untouched (mention) | untouched (mention) — correct today, must stay correct |

Expected behavior per `docs/daily-notes.md`, which documents the own-entry contract but describes only the undecorated shape.

## Why this is a bug

The rule spec 027 introduced is *stricter than the format the vault actually uses*. It was validated against fixtures drawn from a single reproduction, and every downstream gate — two prompt audits and the spec verification — checked faithfully against those same fixtures. Nothing in that chain sampled real daily-note content, so an unrepresentative fixture propagated unchallenged into a shipped identity rule.

The failure is silent and, for `work-on`, actively corrupting: each invocation on a decorated entry adds another duplicate line, so the damage accumulates with use rather than staying constant.

This is filed as a new spec rather than a reopen of 027 per `bug-workflow.md`: completed specs are immutable, and a bug that appears in a new shape after a fix gets its own spec.

## Workaround

Until the fix ships, on the current (post-027) binary:

- `complete` / `defer` on a decorated entry are silent no-ops — not destructive, but the note must be corrected by hand.
- Avoid `work-on` on a decorated entry; it appends a duplicate line that must be deleted manually, and the pollution accumulates with each invocation.
- Removing the decoration from an entry restores correct behavior for all three commands.

## Acceptance Criteria

Fixtures below are the verbatim reproduction lines:

```
- [/] 🐟 [[Feed Worms]]
- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].
```

- [ ] A decorated own entry is recognised by the matcher — evidence: a Ginkgo entry in the `IsOwnDailyNoteEntry` table asserts `🐟 [[Feed Worms]]` against task name `Feed Worms` returns `true`.
- [ ] A decorated line whose wikilink follows prose is still a mention — evidence: a Ginkgo entry asserts `🔧 Nuke-reboot chain — [[Feed Worms]].` against `Feed Worms` returns `false`. The fixture deliberately uses a **single** wikilink naming the target task, so the entry isolates "prose before the wikilink" rather than passing because the leading link names a different task.
- [ ] Non-Latin leading prose is a mention, not decoration — evidence: Ginkgo entries assert `作業 [[Feed Worms]]` and `Задача по [[Feed Worms]]` against `Feed Worms` each return `false`. This is what distinguishes the Unicode-aware rule from an ASCII-only one — an implementation testing `r < utf8.RuneSelf && isASCIIAlnum(r)` passes every other AC in this spec while wrongly promoting CJK and Cyrillic prose lines to own entries.
- [ ] The decoration classes present in the live vault are handled — evidence: Ginkgo entries each returning `true` when the wikilink immediately follows, covering all three classes: (a) the six most-frequent emoji `🔄` (148 live lines), `📈` (133), `🔧` (105), `🐟` (74), `🏠` (37), `🔒` (12); (b) variation-selector emoji `⚠️` and `🛡️` (both `U+FE0F`-suffixed); (c) markdown emphasis `**[[Feed Worms]]**` and `🎯 **[[Feed Worms]]**` — the only non-emoji decoration class in the vault.
- [ ] **Decoration is skipped by character class, not by an enumerated list** — evidence, both halves required: Ginkgo entries assert `true` for at least three decorations that appear in no fixture, no other AC, and no Failure-Modes row (e.g. `§ [[Feed Worms]]`, `❖ [[Feed Worms]]`, `‡ [[Feed Worms]]`); **and** `grep -cE '🐟|🔄|🔧|🏠|📈|🔒|⚠️' pkg/ops/daily_note_entry.go` returns `0` — no emoji literal in the implementation. Without this AC a hardcoded emoji cutset passes every other criterion while leaving 43 of 552 live decorated entries (8%) broken — the tail beyond the six most-frequent prefixes.
- [ ] A decorated line whose leading wikilink is a *different* task is a mention for this task — evidence: a Ginkgo entry asserts `🔧 [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].` against task name `Feed Worms` returns `false`.
- [ ] `complete` flips a decorated own entry — evidence: a Ginkgo test in `complete_test.go` using the decorated fixture asserts the note contains `- [x] 🐟 [[Feed Worms]]` after `Complete`, with the decoration preserved.
- [ ] `defer` removes a decorated own entry — evidence: a Ginkgo test in `defer_test.go` asserts no line containing `[[Feed Worms]]` as a leading link remains after `Defer`, while a decorated *mention* line in the same fixture survives byte-identical.
- [ ] `work-on` promotes a decorated pending entry in place and appends nothing — evidence: a Ginkgo test in `workon_test.go` asserts the note contains `- [/] 🐟 [[Feed Worms]]` and that `strings.Count(updatedContent, "[[Feed Worms]]")` equals `1`.
- [ ] Spec 027's behavior is preserved — evidence: `make test` exits 0; every named Ginkgo entry that spec 027 added still appears in `go test -count=1 ./pkg/ops/... -v -ginkgo.v` output (notably `second wikilink in a chain summary is a mention`, `prefix task name does not match a longer task`, and `prose before the wikilink is a mention`); and `git diff -w --numstat pkg/ops/daily_note_entry_test.go pkg/ops/complete_test.go pkg/ops/defer_test.go pkg/ops/workon_test.go` shows deletions `0` ignoring whitespace-only reflow. No existing `Entry` or `It` may be removed or weakened.
- [ ] `docs/daily-notes.md` documents the decoration rule — evidence: `grep -n -i 'decoration\|emoji' docs/daily-notes.md` returns ≥1 line, and the file states that leading decoration before the wikilink is permitted while prose before it makes the line a mention.
- [ ] `CHANGELOG.md` has a bullet under `## Unreleased` describing the fix — evidence: `grep -n -A20 '^## Unreleased' CHANGELOG.md` shows a line mentioning decorated or emoji-prefixed daily-note entries.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0
- `make test` — unit suite passes
- `go test -count=1 ./pkg/ops/... -v -ginkgo.v 2>&1 | grep -c "<entry name>"` — each new named entry ≥1. The `-ginkgo.v` flag is required: plain `go test -v` does not enable Ginkgo's verbose reporter, so descriptions print only when a spec fails.
- `grep -n -i 'decoration\|emoji' docs/daily-notes.md` — ≥1 line
- `grep -n -A20 '^## Unreleased' CHANGELOG.md` — shows the new bullet

### Operator-executable (runs on the host after the fix lands)

- Rebuild to `/tmp`, seed a throwaway vault from `example/` with a `Daily Notes/` dir and a stub `claude_script`, and replay all three reproductions: A flips the decorated entry preserving the emoji; B removes it; C promotes in place with no duplicate. **Use a stub `claude_script`** — `task work-on` spawns a real `claude --print` session otherwise (`pkg/ops/claude_session.go:69`, 5-minute timeout).

## Desired Behavior

1. Before testing for a leading wikilink, the matcher skips any leading run of **runes** for which neither `unicode.IsLetter` nor `unicode.IsDigit` holds and which are not `[`, together with surrounding whitespace. Iteration is over runes, not bytes, and the classification is Unicode-aware, not ASCII-only — an ASCII-only test would skip CJK and Cyrillic leading text and wrongly promote a prose line such as `作業 [[X]]` to an own entry. Emoji, symbols, punctuation, and markdown emphasis are all skipped by this one rule; there is no emoji-specific list to maintain.
2. After that skip, the existing spec-027 rule applies unchanged: the text must begin with a wikilink resolving to the task, trailing prose is allowed, and alias/heading link forms resolve to the same task.
3. A line whose decoration is followed by prose rather than a wikilink remains a mention. `🔧 Nuke-reboot chain — [[X]] → [[Y]].` skips `🔧 `, finds `Nuke-reboot` — alphanumeric, not `[[` — and is a mention.
4. A line whose decoration is followed by a wikilink to a *different* task remains a mention for this task: `🔧 [[Other]] → [[X]].` resolves the leading link to `Other`.
5. Decoration is preserved on write — `complete` on `- [/] 🐟 [[Feed Worms]]` produces `- [x] 🐟 [[Feed Worms]]`, not a stripped line.
6. All three call sites inherit the change automatically, because they share the single matcher introduced by spec 027.

## Assumptions

- Decoration is always a **leading contiguous run**. A wikilink preceded by decoration mid-text is prose, not an entry; the vault contains no counterexample.
- No operator writes a punctuation-led continuation line (`→ [[X]]`) that is meant to be a mention. Such a line would now be read as an own entry. Zero occurrences in the corpus measured; recorded because it is the shape that would break the assumption.
- The 24-prefix census is a snapshot of the Personal vault's daily notes. The fix must generalise by character class rather than track this list — that is what the generality AC enforces.

## Constraints

- `IsOwnDailyNoteEntry` keeps its signature and stays the single shared matcher — do not add a second matcher or a per-call-site variant.
- Every test added by spec 027 must keep passing unmodified. This spec widens what counts as an own entry; it must not narrow anything.
- The mention rule is the invariant that must not regress: a wikilink appearing after prose is never an own entry, decorated or not.
- `storage.CheckboxRegex` and the two rewrite regexes keep their current semantics — this changes identification, not parsing.
- Decoration must be preserved in the rewritten line; `CheckboxCompleteRegex` only replaces the checkbox marker, so this falls out of the existing implementation and must not be "improved".
- List-marker preservation stays intact (`-` stays `-`, `*` stays `*`).
- `pkg/ops/` remains a library layer — no stdout writes.
- Out of scope, unchanged: goal-file checkbox matching (`pkg/ops/complete.go` `markGoalCheckbox`, `pkg/ops/update.go`).
- Do not hand-bump versions or tag — add the `## Unreleased` bullet only.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery |
|---|---|---|---|
| Multi-rune emoji with variation selector (`⚠️`) | Skipped as decoration, entry recognised | Unit entry for `⚠️` | None needed |
| Decoration followed by prose, then a wikilink | Mention — not an own entry | Unit entry with the chain-summary fixture | None needed |
| Decoration followed by a wikilink to another task | Mention for this task | Unit entry asserting `🔧 [[Other]] → [[X]].` is false for `X` | None needed |
| Line is decoration only, no wikilink | Not an own entry, no panic | Unit entry with `🐟 ` alone | None needed |
| Notes already polluted with duplicates by `work-on` | Not repaired — forward-only | Operator reads the note | Delete the undecorated duplicate by hand; `git log -p` on the daily note if already committed |
| Entry decorated with markdown emphasis (`**[[X]]**`) | Recognised — `*` is skipped as decoration | Unit entry | For `defer`, the line is now deletable — recover from obsidian-git history |
| Decorated line whose leading wikilink is this task **and** which carries a second wikilink in trailing prose | Own entry — `defer` deletes the whole line including the trailing reference | Unit entry with the two-link fixture | Recover the trailing reference from obsidian-git history |
| Keycap emoji prefix (`1️⃣ [[X]]`) | Not recognised — leads with ASCII digit `1`, so the skip run terminates immediately | Known limitation, 0 live occurrences | Remove the keycap or use a non-digit prefix |
| Non-Latin leading prose (`作業 [[X]]`, `Задача [[X]]`) | Mention — `unicode.IsLetter` is true for CJK/Cyrillic, so the skip run terminates | Unit entry asserting `false` | None needed |

## Suggested Decomposition

Single-layer change: one matcher function plus tests at four sites and two docs. One prompt.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Decoration-skip in `IsOwnDailyNoteEntry` + unit entries + call-site regression tests + docs + CHANGELOG | 1–6 | 1–12 | — |

Kept as one prompt: a single function in a single package, whose three consumers already share it via spec 027's matcher. Splitting would force a second prompt to re-derive the same identity rule — the drift 027 exists to prevent.

The DB × AC product (6 × 12 = 72) is above the usual 50 budget. Recorded as a knowing exception, same as spec 027: the change touches one function plus its tests and two docs, the decomposition is a single prompt, and the AC count is inflated by enumerating decoration classes rather than by conceptual surface. The load-bearing AC is the generality one — it is what stops a hardcoded emoji cutset from passing.

## Verification Result

**Verified:** 2026-08-08T13:07:00Z (HEAD 2c860d7)
**Binary:** `vc-new-2c860d7` built from 2c860d7, compared against `vc-old-3feb43d` (pre-fix 3feb43d)
**Scenario:** All three Reproductions replayed on a throwaway vault seeded from `example/` with a `Daily Notes/` dir and a stub `claude_script` — each run twice, old binary then new, same fixture, `diff` before/after; then all three of spec 027's Reproductions replayed against the new binary.
**Evidence:**
- A (`complete` on `- [/] 🐟 [[Feed Worms]]`) — old: empty diff; new: `2c2 - [/] 🐟 [[Feed Worms]] → - [x] 🐟 [[Feed Worms]]`, emoji preserved, mention line byte-identical
- B (`defer`) — old: empty diff; new: `2d1` (own entry only; decorated chain-summary mention survives)
- C (`work-on` on `- [ ] 🐟 …`) — old: `1a2 > - [/] [[Feed Worms]]` (duplicate, 2 links); new: `2c2 → - [/] 🐟 [[Feed Worms]]`, `grep -c '[[Feed Worms]]'` = 1
- 027 regression, new binary: A `3c3` own entry flips, decorated mention left `[/]`; B `3d2` own entry only; C `1a2 > - [/] [[Turn on hell - 2026W32-sat]]` added — all three still pass
- Runtime matcher matrix via `complete` (23 lines, one pass): all 6 top emoji + `⚠️`/`🛡️` + `**`/`🎯 **` + `§`/`❖`/`‡` + alias/heading flipped to `[x]`; `作業`, `Задача по`, `Working on`, `🔧 Nuke-reboot chain —`, `🔧 [[Shutdown K3s]] → `, `1️⃣`, bare `🐟 ` all stayed `[/]`
- Mutation test: ASCII-only variant (`r < utf8.RuneSelf && isASCIIAlnum(r)`) in a throwaway worktree fails **exactly** `CJK leading prose is a mention` and `Cyrillic leading prose is a mention` (730 passed / 2 failed) — the AC-3 fixtures are load-bearing
- `grep -cE '🐟|🔄|🔧|🏠|📈|🔒|⚠️' pkg/ops/daily_note_entry.go` → `0`; `git diff -w --numstat 3feb43d HEAD` on the four test files → `34/0`, `132/0`, `44/0`, `37/0` (insertions only)
- `make test` exit 0; `make precommit` exit 0 ("ready to commit"); `go test -count=1 ./pkg/ops/... -v -ginkgo.v` exit 0, 732/734, all 23 required named entries present; scenario 002 passes on the new binary
**Verdict:** PASS
