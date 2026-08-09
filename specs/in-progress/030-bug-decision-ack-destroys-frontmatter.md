---
status: verifying
approved: "2026-08-09T11:40:53Z"
generating: "2026-08-09T11:51:26Z"
prompted: "2026-08-09T11:51:26Z"
verifying: "2026-08-09T12:00:49Z"
branch: dark-factory/bug-decision-ack-destroys-frontmatter
---

## Summary

- Acknowledging a decision silently deletes every frontmatter field the tool does not itself manage.
- Only six fields survive; everything else on the page is discarded without warning or error.
- Trading Decision Records are the worst-affected class — the deleted fields ARE the decision record.
- One deleted field is a scheduled review date, so the loss silently retires a governance gate.
- Decision is the only entity in the tool that loses data this way; the other five preserve unknown fields.
- After the fix, acking a decision changes only the fields the ack is meant to change.

## Problem

`vault-cli decision ack` rebuilds a decision's frontmatter from scratch on every write, emitting only a
six-key whitelist. Any other key present on the page is dropped. For Trading Decision Records this is
data destruction, not cosmetic loss: `selected_option` and `decision_status` *are* the decision, and
`review_date` is the scheduled governance gate. A silently retired review gate is the exact failure that
let one strategy trade roughly three weeks past its own fired kill-switch in July 2026. The command that
is supposed to *record* a decision is the command that erases it.

The blast radius is every document acked through this path that carries frontmatter beyond the whitelist.
Periodic notes happen to be safe — everything they carry is inside the whitelist — which is why the bug
went unnoticed until a TDR was acked.

## Reproduction

Observed 2026-08-09. Tool version `v0.102.5` (`vault-cli --version`).

Smallest reproducing input — a decision file with any key outside the managed six:

```yaml
---
date: 2026-08-09
decision_confidence: high
decision_status: proposed
needs_review: true
page_type: decision
related:
    - '[[Some Page]]'
    - '[[Another Page]]'
related_task: '[[Some Task]]'
review_date: 2026-08-15
selected_option: B
type: Trading Decision Record
---
Body text here.
```

Command sequence:

```bash
vault-cli decision ack "TDR 2026-08-09 - GBPJPY V6 Pause Continuation" --vault trading
git diff
```

Observed evidence — `git diff` showed these lines removed and never re-emitted:

```diff
-date: 2026-08-09
-decision_confidence: high
-review_date: 2026-08-15
-related_task: '[[Some Task]]'
-related:
-    - '[[Some Page]]'
-    - '[[Another Page]]'
```

The body content below the frontmatter was preserved. The six managed keys were written correctly.
The file was restored by hand from `git`.

## Expected vs Actual

**Expected** — acking a decision mutates only the fields the ack is defined to mutate
(`needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type`) and leaves every other
frontmatter key byte-identical. This is the documented contract every other entity in the tool honours:
`pkg/storage/task.go`, `goal.go`, `objective.go`, `theme.go`, and `vision.go` all serialize
`RawMap()`, the full preserved map, so unknown keys round-trip untouched.

**Actual** — `WriteDecision` (`pkg/storage/decision.go:218`) constructs a brand-new `map[string]any`
containing only the six managed keys and passes that to `serializeMapAsFrontmatter`. The write is a
whitelist REPLACE, not a merge. Every non-managed key is dropped.

## Why this is a bug

Parsing is not at fault. `readDecisionFromPath` (`pkg/storage/decision.go:28`) already calls
`parseToFrontmatterMap` and reads the complete map into `data` correctly — it then projects that map
onto a six-field struct and discards the rest. `domain.Decision` (`pkg/domain/decision.go:10`) is a
plain struct and is the **only** domain type that does not embed `FrontmatterMap`, the type that provides
`RawMap()` (`pkg/domain/frontmatter_map.go:144`). The write path rebuilds from the lossy projection.

This is a deliberate deferral that was never closed, not an oversight: `specs/completed/008-flexible-frontmatter-refactor.md`
introduced `FrontmatterMap` for Task/Goal/Theme/Objective/Vision, and its Non-goals section explicitly
excluded Decision as "a different pattern [that] can be done separately." This spec closes that deferral.

Silent data loss with no error and no warning also fails the tool's own baseline expectation — the same
class of defect as `specs/completed/026-bug-duplicate-frontmatter-key-silent-task-loss.md`.

## Workaround

Do not run `vault-cli decision ack` on any Trading Decision Record until this ships. Ack TDRs by
hand-editing the `reviewed` / `reviewed_date` / `status` frontmatter fields instead. Periodic notes are
unaffected and may be acked normally.

## Goal

`vault-cli decision ack` preserves every pre-existing frontmatter field on any decision it writes,
mutating only the six managed keys. `domain.Decision` follows the same `FrontmatterMap` pattern as the
other five domain entities, so the preservation property holds by construction rather than by a
hand-maintained field list that drifts as new TDR fields are introduced.

## Acceptance Criteria

- [ ] Acking a decision whose frontmatter carries keys outside the managed six leaves every such key and
      its value intact — evidence: a test in `pkg/storage/decision_test.go` writes a decision carrying
      `date`, `decision_confidence`, `decision_status`, `selected_option`, `review_date`, `related_task`,
      `supersedes`, re-parses the written file, and asserts each key equals its pre-write value; `make test` exits 0.
- [ ] A list-valued frontmatter key round-trips as a list, not a flattened string — evidence: the same
      test asserts the re-parsed `related` key is a 2-element sequence with both original entries; `make test` exits 0.
- [ ] The six managed fields still update correctly on ack — evidence: the same test asserts the written
      file parses to `reviewed: true`, `needs_review: false`, and a non-empty `reviewed_date`, and that
      `status` / `type` / `page_type` retain their set values; `make test` exits 0.
- [ ] Negative — the markdown body below the frontmatter is not modified by a write — evidence: the test
      asserts the body substring after the closing `---` is string-equal to the pre-write body; `make test` exits 0.
- [ ] Negative — an unknown key is not silently coerced to a different YAML type — evidence: the test
      asserts an integer-valued and a boolean-valued unknown key re-parse as `int` and `bool`, not `string`; `make test` exits 0.
- [ ] `FrontmatterMap.GetBool` coerces rather than asserts — evidence: a table test in
      `pkg/domain/frontmatter_map_test.go` asserts `true`/`false` (bool), `"true"`/`"yes"`/`"TRUE"` → `true`,
      `"false"`/`"no"` → `false`, and missing key / unparseable value → `false`; `make test` exits 0.
- [ ] `domain.Decision` carries the preserved map rather than a six-field projection — evidence:
      `grep -n 'FrontmatterMap' pkg/domain/decision.go` returns ≥1 line, and
      `grep -n 'RawMap()' pkg/storage/decision.go` returns ≥1 line inside `WriteDecision`.
- [ ] **Operator-executable:** the original reproduction no longer reproduces — evidence: copy a real TDR
      into a scratch git-tracked dir, run the freshly-built `vault-cli decision ack` against it, and confirm
      `git diff` shows zero deleted lines for `date`, `decision_confidence`, `review_date`, `related_task`,
      `related`, `selected_option`, `decision_status`, `supersedes` — only managed-field lines change.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint, vet, and full test suite pass
- `make test` — unit tests pass, including the new preservation tests
- `grep -n 'FrontmatterMap' pkg/domain/decision.go` — struct embeds the preserved map
- `grep -n 'RawMap()' pkg/storage/decision.go` — write path serializes the preserved map

### Operator-executable (runs on the host after merge)

- Build the binary, copy a real TDR into a scratch git dir, run `vault-cli decision ack` against it, and
  inspect `git diff` — the reproduction replay described in the final Acceptance Criterion.

## Desired Behavior

1. Reading a decision retains the complete parsed frontmatter map, not a six-field projection.
2. Writing a decision emits the retained map with the managed keys overlaid onto it.
3. Unknown keys round-trip with their YAML types intact — lists stay lists, integers stay integers, booleans stay booleans.
4. The six managed keys continue to behave exactly as they do today, including `reviewed_date`'s
   date-vs-datetime formatting rule.
5. The markdown body below the frontmatter is never modified by a decision write.

## Constraints

- The managed key set stays exactly `needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type`.
  This spec does not add, remove, or rename a managed field.
- `serializeMapAsFrontmatter` and `parseToFrontmatterMap` are not modified — they already behave correctly.
- No other domain type is touched. `decision.go` is the sole outlier; the other five are already correct.
- Follow the existing pattern verbatim: `pkg/storage/task.go` `WriteTask` for the write shape,
  `pkg/domain/decision.go` mirroring the struct shape documented in `docs/development-patterns.md`.
- `FrontmatterMap` currently exposes `Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`,
  `Keys`, `RawMap` — there is **no** bool accessor, because `needs_review` / `reviewed` are the first
  bool frontmatter fields in the codebase. Add `GetBool` to the accessor family; do NOT type-assert
  `.(bool)` at the call site. Like its siblings it must coerce rather than assume one YAML type:
  `bool` passes through, and the strings `true` / `false` / `yes` / `no` parse to their bool value
  (case-insensitive); anything else, and a missing key, return `false`. A bare assertion silently yields
  `false` for a quoted `reviewed: "true"`, which on this command means "not reviewed" and re-surfaces an
  already-acked decision — a silent-wrong-value failure in the same family as the bug being fixed.
- `reviewed_date` formatting must keep using `formatReviewedDate` — midnight-UTC values format as
  `YYYY-MM-DD`, everything else as RFC3339.
- Error handling follows `github.com/bborbe/errors` — `errors.Wrap` / `errors.Wrapf`, never `fmt.Errorf`,
  never a bare `return err`.
- Tests use Ginkgo/Gomega. `pkg/storage/storage_suite_test.go` already exists, so new specs in that
  package will actually run.
- Existing decision tests in `pkg/storage/decision_test.go` must continue to pass unchanged.
- `docs/development-patterns.md` documents the `FrontmatterMap` accessor list — add `GetBool` to it so the
  doc does not drift from the code.

## Failure Modes

| Trigger | Expected behavior | Detection | Recovery |
|---|---|---|---|
| Decision file has malformed YAML frontmatter | Parse fails, wrapped error returned, file left untouched | `decision ack` exits non-zero with a wrapped parse error | Operator fixes the YAML by hand and re-runs |
| Decision file has no frontmatter at all | Managed keys are written; no crash on the empty preserved map | Written file contains only the six managed keys | None needed — this is correct behavior |
| A non-managed key collides with a managed key name | Managed value wins — the overlay is applied last | Test asserts overlay ordering | None needed — defined precedence |
| Write fails partway (disk full, permissions) | Wrapped error returned; no partial frontmatter emitted | Non-zero exit with wrapped write error | Original file is intact on disk; re-run after fixing the cause |
| A previously-acked TDR already lost fields before this fix | Out of scope for the code fix — history is not reconstructed | Operator audit of the Trading vault git history | Restore from `git` history by hand |
| Two writers touch the same frontmatter file concurrently (vault-cli plus the git-rest service — the collision documented in `specs/completed/026-bug-duplicate-frontmatter-key-silent-task-loss.md`) | Unchanged from pre-fix behavior — last writer wins, no locking added | Conflicting content appears on the page or in `git` history | Out of scope for this fix; preserving the map neither improves nor worsens the race |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Add `FrontmatterMap.GetBool` with its coercion table test | — | 7 | — |
| 2 | Embed `FrontmatterMap` in `domain.Decision`, populate it in `readDecisionFromPath`, serialize `RawMap()` with the managed overlay in `WriteDecision` | 1, 2, 4, 5 | 6 | prompt 1 (uses `GetBool`) |
| 3 | Preservation regression tests — unknown-key survival, list-type round-trip, managed-field correctness, body-unchanged, type-coercion negatives | 3 | 1-5 | prompt 2 |

Rationale: prompt 1 is a self-contained accessor addition that prompt 2 depends on; prompt 2 is the
behavior change and is meaningless without prompt 3's proof; prompt 3 is test-only and depends on
prompt 2's struct shape existing. Three prompts, strict order.
