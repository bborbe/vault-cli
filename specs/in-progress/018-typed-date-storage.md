---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-06-20T13:16:04Z"
generating: "2026-06-20T13:16:05Z"
prompted: "2026-06-20T13:31:17Z"
verifying: "2026-06-20T14:05:39Z"
branch: dark-factory/typed-date-storage
---

## Summary

- Today vault-cli stringifies `*DateOrDateTime` to YAML through a local helper (`formatDateOrDateTime`) that re-implements format rules the type itself already exposes via `encoding.TextMarshaler`. The helper exists in TWO copies (one in `pkg/domain/`, one in `pkg/ops/`), each with its own callers. Two — soon three — definitions of "what does this date look like on disk?" with no test tying them together.
- Both local helpers are removed. Setters store the typed value directly; YAML emission goes through the type's own `MarshalText` via yaml.v3. JSON projection in `pkg/ops/` goes through the type's own `MarshalJSON` / `String()`.
- Reader path stays string-tolerant for legacy files (entire vault history is on-disk strings) but also tolerates the in-memory typed shape so that Set → Get without an intervening YAML round-trip returns the same value.
- On-disk YAML format does not change. CLI JSON output format does not change. Defensive `!!timestamp` quoting still applies. Existing vault files remain readable, byte-for-byte unchanged on rewrite.
- Forward value: a future decision to emit unquoted YAML dates becomes a one-place change in `bborbe/time` (add `MarshalYAML`) — every consumer updates together.

## Problem

vault-cli has TWO local helpers (both named `formatDateOrDateTime`) that convert `*libtime.DateOrDateTime` to a string:

- `pkg/domain/task_frontmatter.go` — used by every Task/Goal/Objective/Theme setter and by `getField`-style string projection arms (~9 call sites across 4 files).
- `pkg/ops/frontmatter.go` — used by `pkg/ops/show.go` and `pkg/ops/list.go` to populate JSON-output Detail / Item structs for the CLI.

The type itself already implements `encoding.TextMarshaler` AND `json.Marshaler` with the same format rules. We have two — effectively three — definitions of the disk-and-wire format, in two repos, with no test asserting they agree. If the upstream `MarshalText` / `MarshalJSON` changes (extra precision, different zero-time handling, normalization), vault-cli silently keeps emitting the old format and the divergence is invisible until a downstream consumer breaks. This is a small but durable footgun, and it blocks a cleaner forward path: any future change to YAML or JSON emission has to be done in three places.

## Goal

After this work:

- Setter call sites for date-typed fields on Task, Goal, Objective, Theme, and any other frontmatter type that stores `*DateOrDateTime` pass the typed value directly into the underlying map. No local stringification of `*DateOrDateTime` remains in vault-cli.
- JSON-projection call sites in `pkg/ops/` (Detail/Item population in `show.go` / `list.go`) use the type's own `MarshalJSON` / `String()` directly — no `pkg/ops/`-local stringifier.
- Reader call sites tolerate three observable shapes for date-valued frontmatter keys: legacy string (from on-disk files written before this change), `time.Time` (yaml.v3's default decode for timestamp scalars), and the typed `libtime.DateOrDateTime` value (in-memory Set → Get round-trip without YAML). Missing key, empty string, unparseable string, and unsupported types continue to yield "no value" without panicking.
- Writing a task, goal, objective, or theme that was loaded from disk and rewritten without other edits produces byte-identical output to the pre-change implementation for every date field combination exercised by the existing scenarios.
- The lower-level `formatTimeAsDate(time.Time) string` helper continues to exist and is not modified by this work — only the `*DateOrDateTime`-typed wrappers are removed.

## Non-goals

- Do NOT add `MarshalYAML` / `UnmarshalYAML` to `bborbe/time`. That is a separate, opt-in future change; if it lands, it would remove the defensive quoting, which IS a disk-format change and needs its own spec.
- Do NOT change the on-disk YAML format of any date field. Date-only values stay `YYYY-MM-DD`; values with a time component stay RFC3339 / RFC3339Nano as the type itself emits them. Quoting stays as yaml.v3 currently emits it.
- Do NOT change the CLI JSON output format. The shape produced by `vault-cli task list --output json` and `vault-cli task show --output json` is byte-identical to v0.80.0 for the same inputs.
- Do NOT remove or modify `formatTimeAsDate(time.Time) string`. It is the lower-level helper used by other call sites and stays untouched.
- Do NOT change any exported setter or reader signature. Callers outside the changed package see no API difference.
- Do NOT touch the WriteTask UUID fallback or `INVALID_TASK_IDENTIFIER` lint behavior — separate concern, already closed in v0.79.0.
- Do NOT add a feature flag / opt-out for the typed-storage path. The whole point is that there is one definition of the format; a flag re-introduces the divergence the spec is removing. If a future consumer demands variation, that is a separate spec.

## Desired Behavior

1. **Typed storage at the setter boundary.** Every setter on the affected frontmatter types that accepts a `*libtime.DateOrDateTime` stores the dereferenced typed value into the underlying map when non-nil, and deletes the key when nil. No setter calls a `*DateOrDateTime`-to-string conversion helper in vault-cli.
2. **Reader-side multi-shape tolerance.** The reader path for date-valued keys returns a non-nil result when the stored value is any of: a non-empty parseable string (legacy on-disk), a `time.Time` (yaml.v3 decode), or a `libtime.DateOrDateTime` (in-memory). It returns nil for: missing key, empty string, unparseable string, or any other type. No panic on unexpected input.
3. **YAML round-trip stability.** A task / goal / objective / theme loaded from disk, with no field changes, and rewritten, produces byte-identical YAML to what the pre-change implementation produced for the same input. Verified by golden file or scenario replay.
4. **Stringifier deletion is complete in BOTH locations.** Both copies of `formatDateOrDateTime` are gone from the codebase: the one in `pkg/domain/` AND the one in `pkg/ops/`. The `case "<field>":` arms in any `getField`-style switch that previously called the domain helper now call the typed `DateOrDateTime`'s own `String()` method (or equivalent). The JSON-projection sites in `pkg/ops/show.go` and `pkg/ops/list.go` (Detail/Item struct population) call the type's own `MarshalJSON` / `String()` directly — no local stringifier wrapper.
5. **Setter audit coverage.** The migration covers Task, Goal, Objective, and Theme frontmatter setters that take `*libtime.DateOrDateTime`. Decision frontmatter has no `*DateOrDateTime` setters today and is in-scope only to confirm that absence — if any are found, they are migrated under the same rule.
6. **Single source of format truth.** After this work, the on-disk string form AND the CLI JSON string form of a `*libtime.DateOrDateTime` are determined exclusively by the type itself (`MarshalText` for YAML, `MarshalJSON` for JSON, both in `bborbe/time`). vault-cli contains no code that branches on `isMidnightUTC`-style logic for `*libtime.DateOrDateTime` values.

## Constraints

- Public setter signatures (`SetDeferDate(*libtime.DateOrDateTime)`, `SetStartDate(*libtime.DateOrDateTime)`, etc.) on Task / Goal / Objective / Theme frontmatter MUST NOT change.
- Public reader signatures (`DeferDate() *libtime.DateOrDateTime`, `GetTime(key) *time.Time`, etc.) MUST NOT change.
- The `last_completed_date` + `last_completed` dual-write window on Task frontmatter MUST be preserved — both keys still get the same value.
- The CLI surface (`vault-cli task list --output json`, `vault-cli task show --output json`, scenarios 001–004) MUST produce byte-identical output for the same input. The type's existing `MarshalJSON` is the authoritative JSON shape and is unchanged.
- BOTH `formatDateOrDateTime` helpers (the one in `pkg/domain/task_frontmatter.go` and the one in `pkg/ops/frontmatter.go`) are in scope for removal. Neither survives this work. Removing only one leaves the same duplicate-stringification problem the spec is solving.
- `pkg/domain/frontmatter_map.go::GetTime` already implements the string / `time.Time` switch documented in `[[vault-cli YAML Date Accessor Regression]]`. This work extends that switch with a `libtime.DateOrDateTime` arm; it does not replace or rewrite the existing arms.
- Parse helper additions follow `~/Documents/workspaces/coding/docs/go-parse-pattern.md` — if a new `ParseDateOrDateTime(ctx, any) (*libtime.DateOrDateTime, error)` is introduced, it ships paired with `ParseDateOrDateTimeDefault(ctx, any, default) libtime.DateOrDateTime`.
- `formatTimeAsDate(time.Time) string` MUST remain in place, unchanged in signature and behavior.
- `make precommit` MUST stay clean throughout.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Legacy on-disk file has `defer_date: "2025-07-31"` (string) | Reader returns the parsed date; rewrite emits identical bytes | None — automatic |
| Legacy on-disk file has `defer_date: 2025-07-31` (unquoted, yaml.v3 decodes as `time.Time`) | Reader returns the parsed time; rewrite emits the same scalar yaml.v3 produces for our typed value | None — automatic |
| Frontmatter value under a date key is an unparseable string (e.g. `"not-a-date"`) | Reader returns nil; setter that overwrites it succeeds; no panic | Caller treats as missing; can overwrite via setter |
| Frontmatter value under a date key is an unexpected type (e.g. `int`, `[]any`) | Reader returns nil; no panic | Caller treats as missing |
| Setter called with `nil *DateOrDateTime` | Key is deleted from the map (existing behavior) | None — automatic |
| Setter called with a `*DateOrDateTime` whose underlying time is the zero value | Stored as the type's zero representation; reader returns nil (matches type's `IsZero` contract) | None — automatic |
| `MarshalText` on the type panics or returns an error mid-write | yaml.v3 surfaces the error to the caller of the write; no partial file is written | Existing error path in the write layer; no new recovery needed |
| In-memory Set followed by Get without YAML round-trip | Get returns the same logical value Set was given | None — automatic |
| CLI JSON output projection (`pkg/ops/` Detail/Item) diverges from canonical type format — e.g. only the `pkg/domain/` helper is removed and the `pkg/ops/` helper silently keeps the old string form, OR vice versa | Caught by the `vault-cli task list --output json` and `vault-cli task show --output json` byte-identical baseline ACs (#13 below); CI fails before merge | Re-migrate the missed call sites in `pkg/ops/show.go` + `pkg/ops/list.go` to use `MarshalJSON` / `String()` directly; delete the `pkg/ops/` helper |

## Security / Abuse Cases

Not applicable — this work changes an internal storage encoding boundary. No new HTTP, file, or user-input surface; no new parsing of attacker-controlled data. Input shapes already exist in the vault and are already parsed by `libtime.ParseTime`.

## Acceptance Criteria

- [ ] `grep -rn 'formatDateOrDateTime' pkg/` returns zero matches across the entire `pkg/` tree (covers BOTH `pkg/domain/` and `pkg/ops/` copies) — evidence: shell exit code 1 / empty output
- [ ] `grep -rn 'formatTimeAsDate' pkg/domain/task_frontmatter.go` returns at least one match (helper still exists) — evidence: exit code 0, non-empty output
- [ ] `make precommit` exits 0 — evidence: exit code
- [ ] All existing tests under `pkg/domain/...` and `pkg/ops/...` pass unchanged — evidence: `go test ./pkg/domain/... ./pkg/ops/...` exit 0
- [ ] New unit test asserts: `Set<X>Date(d)` followed by `<X>Date()` (no YAML round-trip) returns a value equal to `d` for every migrated setter — evidence: test names listed in `go test -v ./pkg/domain/... -run TypedDateRoundTrip` output, all pass
- [ ] New unit test asserts: a frontmatter map whose date key holds a raw `libtime.DateOrDateTime` (struct, not pointer) is correctly read by `GetTime` / the date accessor — evidence: test pass
- [ ] New unit test asserts: a frontmatter map whose date key holds a legacy `string` value (e.g. `"2025-07-31"`) is correctly read — evidence: test pass (this is a regression guard for the existing v0.55.2 behavior)
- [ ] New unit test asserts: a frontmatter map whose date key holds a `time.Time` value is correctly read — evidence: test pass
- [ ] New unit test asserts: a frontmatter map whose date key holds an unparseable string returns nil from the reader — evidence: test pass
- [ ] Golden-file or roundtrip test: serializing a TaskFrontmatter with all date fields set, then comparing against a checked-in golden YAML, matches byte-for-byte — evidence: file diff empty
- [ ] Scenarios 002 (`task-lifecycle`), 003 (`task-recurring-completion`), 004 (`decision-list-ack`) pass — evidence: `make scenarios` exit 0, scenario report shows all pass
- [ ] Scenarios 002, 003, 004 produce task / goal / objective / theme files whose byte content matches what v0.80.0 produced for the same inputs — evidence: `diff -r` between scenario output and a v0.80.0 baseline checked into the repo returns no differences. Baseline location: `pkg/ops/testdata/v0.80.0-baseline/scenario-{002,003,004}/` (a new directory created during prompt 3 by replaying the scenarios against a clean v0.80.0 checkout and committing the produced files into the repo so the comparison is deterministic across CI runs and re-replays). No `git stash` / runtime-capture alternative — the baseline must be a checked-in artifact
- [ ] `vault-cli task list --output json` against a vault containing dates in all three shapes (string, time.Time-decoded, typed) returns the same JSON shape as v0.80.0 for the same inputs — evidence: `diff` of stdout against a v0.80.0 baseline file checked into `pkg/ops/testdata/v0.80.0-baseline/task-list.json` returns empty
- [ ] `vault-cli task show --output json` against the same vault returns the same JSON shape as v0.80.0 — evidence: `diff` of stdout against a v0.80.0 baseline file checked into `pkg/ops/testdata/v0.80.0-baseline/task-show.json` returns empty
- [ ] `pkg/ops/testdata/v0.80.0-baseline/README.md` exists and records the v0.80.0 commit SHA + the replay command used to capture the baseline files. Evidence: `grep -E '^commit: [a-f0-9]{40}$' pkg/ops/testdata/v0.80.0-baseline/README.md` returns exactly 1 line; the README also contains the replay command verbatim and the capture date.
- [ ] If a new `ParseDateOrDateTime` is added: the paired `ParseDateOrDateTimeDefault` exists in the same package — evidence: `grep -n 'func ParseDateOrDateTime\(Default\)\?' pkg/domain/` returns both names

Scenario coverage: NO new scenario added. The three existing scenarios (002, 003, 004) already exercise every setter and reader migrated here, against a real on-disk vault. Adding a fourth would duplicate coverage of behaviors already verified by unit + golden-file + existing-scenario tests.

## Verification

```
cd ~/Documents/workspaces/vault-cli-date-storage
make precommit
make scenarios
grep -rn 'formatDateOrDateTime' pkg/   # must print nothing, exit 1
grep -n 'formatTimeAsDate' pkg/domain/task_frontmatter.go   # must print the helper definition
diff -r pkg/ops/testdata/v0.80.0-baseline/scenario-002/ <scenario-002-output-dir>/
diff -r pkg/ops/testdata/v0.80.0-baseline/scenario-003/ <scenario-003-output-dir>/
diff -r pkg/ops/testdata/v0.80.0-baseline/scenario-004/ <scenario-004-output-dir>/
diff <(vault-cli task list --output json --vault <fixture-vault>) pkg/ops/testdata/v0.80.0-baseline/task-list.json
diff <(vault-cli task show <task-id> --output json --vault <fixture-vault>) pkg/ops/testdata/v0.80.0-baseline/task-show.json
```

Expected: `make precommit` exits 0, `make scenarios` exits 0 with 002 / 003 / 004 reported pass, first grep exits 1 with empty stdout, second grep exits 0 with the helper definition line, all `diff` commands produce empty output.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Reader-side multi-shape tolerance: extend the date-reader path (`GetTime` and any sibling) with the `libtime.DateOrDateTime` arm; add `ParseDateOrDateTime` + `ParseDateOrDateTimeDefault` if introduced; unit tests for all four shapes (string / `time.Time` / typed / unparseable). No setter changes yet. Neither stringifier is touched. | 2 | reader-shape ACs (#6–#9), parse-pair AC (#16), existing-tests-pass AC (#4) | — |
| 2 | Setter migration on Task frontmatter only: drop the `pkg/domain/` `formatDateOrDateTime` call from each `Set<X>Date` setter on Task; update any `getField`-style switch arms; add typed round-trip unit tests for Task setters; golden-file test for Task. Both stringifier helpers STAY in place because Goal/Objective/Theme still call the `pkg/domain/` one and `pkg/ops/` still calls its own. | 1, 4 (Task scope only — domain helper not yet removable) | typed-storage AC (#5 for Task), golden-file AC (#10) | 1 |
| 3 | Setter migration on Goal, Objective, Theme; audit Decision frontmatter (expected: no `*DateOrDateTime` setters present, confirm and note); delete the `pkg/domain/` `formatDateOrDateTime`; migrate `pkg/ops/show.go` + `pkg/ops/list.go` to use the type's own `MarshalJSON` / `String()` directly; delete the `pkg/ops/` `formatDateOrDateTime`; capture v0.80.0 baseline artifacts into `pkg/ops/testdata/v0.80.0-baseline/` (scenario output trees + `task-list.json` + `task-show.json`) by replaying against a clean v0.80.0 checkout — author `pkg/ops/testdata/v0.80.0-baseline/README.md` alongside the baseline files, capturing the v0.80.0 commit SHA at the moment of replay (so subsequent re-generations against `master` cannot pass off newer-master output as the v0.80.0 baseline), the verbatim replay command, and the capture date; scenarios 002 / 003 / 004 sweep + byte-identical baseline check + JSON-output baseline check. | 1, 4, 5, 6 | grep-zero AC (#1), helper-stays AC (#2), precommit AC (#3), scenario-pass AC (#11), scenario-byte-identical AC (#12), `task list` JSON AC (#13), `task show` JSON AC (#14), baseline-provenance README AC (#15) | 2 |

Rationale: reader first (prompt 1) so that once setters start writing typed values, the in-memory and YAML-decoded forms both already read correctly — no broken-intermediate state. Task next (prompt 2) because it has the most setters and the dual-write `last_completed` quirk; isolating it surfaces any unexpected coupling. Goal / Objective / Theme + the `pkg/ops/` JSON-projection migration last (prompt 3) because both helper deletions can only happen once every caller is migrated — deleting either earlier breaks the tree. The `pkg/ops/` migration rides in prompt 3 rather than its own prompt because (a) it shares the AC #1 grep gate with the domain helper, (b) it shares the v0.80.0 baseline capture work, and (c) splitting it would leave one prompt unable to satisfy AC #1. Each prompt leaves the tree green; no prompt depends on a future prompt's behavior.

## Do-Nothing Option

If we don't do this: the codebase keeps two `formatDateOrDateTime` helpers (domain + ops) plus the type's own `MarshalText` / `MarshalJSON` — three definitions of the wire form of `*DateOrDateTime`, in two repos, with no test linking them. Cost today is near-zero — the formats agree. Risk is durable: any change to `MarshalText` or `MarshalJSON` in `bborbe/time` silently diverges vault-cli's on-disk and CLI-JSON form from every other Go consumer of the type, and the divergence is only caught when a downstream reader breaks. Also blocks a clean single-place implementation of any future YAML- or JSON-emission change (e.g. unquoted dates, normalized precision).

The do-nothing case is defensible if `bborbe/time`'s `MarshalText` and `MarshalJSON` are considered frozen forever. They are not — recent additions to the package (e.g. `RFC3339Nano` precision) prove it evolves. So: do it once, here, cheaply, while the surface is small.
