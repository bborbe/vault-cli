---
status: idea
kind: bug
created: 2026-08-08
---

## Summary

- `task work-on` never promotes an **indented** pending daily-note entry from `[ ]` to `[/]`.
- The line is recognised — `found` is true, so no duplicate is appended — but the rewrite silently produces no change and nothing is written.
- Cause: `findAndUpdateTaskCheckbox` treats `CheckboxRegex` capture group 1 as the list marker when it is actually the **leading whitespace**.
- Pre-existing and independent of specs 027 and 028: it predates both and reproduces on a plain undecorated entry.
- 2,079 indented task entries exist in the Personal vault's daily notes, 759 of them pending.

## Problem

`pkg/ops/workon.go:255-257`:

```go
marker := matches[1]
lines[i] = strings.Replace(line, marker+" [ ]", marker+" [/]", 1)
```

`storage.CheckboxRegex` is `^(\s*)[-*] \[([ x/])\] (.+)$` — group 1 is leading whitespace, group 2 is the state, group 3 is the text. **The list marker is not captured at all.**

For an unindented line, group 1 is `""`, so the search string is `" [ ]"`, which occurs in `- [ ] [[X]]` and the replace works by coincidence. For an indented line, group 1 is `"  "`, so the search string becomes `"   [ ]"` (three spaces) — which does not occur in `"  - [ ] [[X]]"`. No replacement, `modified` stays false, and `updateDailyNote` returns without writing.

Spec 027 noticed the misnaming and explicitly scoped it out as unrelated to its identity fix. That was correct for 027, but the underlying defect is real.

## Reproduction

Verified 2026-08-08 by running the exact expression from `workon.go:255-257` against `storage.CheckboxRegex`:

```
promoted=true   in="- [ ] [[Feed Worms]]"      out="- [/] [[Feed Worms]]"
promoted=false  in="  - [ ] [[Feed Worms]]"    out="  - [ ] [[Feed Worms]]"
promoted=false  in="\t- [ ] [[Feed Worms]]"    out="\t- [ ] [[Feed Worms]]"
```

Both space- and tab-indented entries fail. Decoration is irrelevant — this reproduces on a bare wikilink entry.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| `work-on` on `  - [ ] [[X]]` | entry promoted to `  - [/] [[X]]` | no change, no write, command reports success |
| `work-on` on `- [ ] [[X]]` | entry promoted | works (by coincidence — group 1 is empty) |

## Why this is a bug

The function's stated contract is "find a task checkbox and update it to in-progress if pending". It silently does nothing for an entire class of well-formed input that `CheckboxRegex` itself accepts. The parser admits indented lines; the rewriter cannot handle them. And because `found` is true, the caller's fallback (`appendTaskToDaily`) does not fire either — so the user gets neither a promotion nor a new entry, and a success message regardless.

## Scale

Measured across `60 Periodic Notes/Daily/*.md` in the Personal vault:

```
indented lines that are task entries : 2079
  of those, pending [ ] (work-on path):  759
```

Many are review-checklist children (`\t\t- [ ] [[23 Goals]]`) rather than vault-cli-managed tasks, so the practical impact is smaller than the raw count — but the affected population is not empty.

## Open Questions

1. Should the fix rebuild the line from capture groups (`matches[1] + marker + " [/] " + matches[3]`), which requires capturing the list marker that `CheckboxRegex` currently discards — and therefore touches a regex three other call sites depend on? Or should it operate on the trimmed line and re-attach the indent, leaving the regex alone?
2. Do `complete` and `defer` have the same indentation blind spot? They use `CheckboxCompleteRegex` / whole-line removal rather than this marker-splice, so probably not — but that needs checking before scoping.
3. Is promoting indented entries actually wanted? An indented entry may be a sub-item of a parent task rather than a task's own entry, in which case the correct fix might be to leave it alone deliberately and document that.

Question 3 needs an answer before this leaves `idea` — the fix direction depends on it.
