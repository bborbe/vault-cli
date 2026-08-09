---
status: prompted
approved: "2026-08-09T14:01:08Z"
generating: "2026-08-09T14:31:07Z"
prompted: "2026-08-09T14:31:07Z"
branch: dark-factory/bug-unquoted-wikilink-mangled-on-frontmatter-write
---

## Summary

- An unquoted Obsidian wikilink in frontmatter (`related_task: [[Some Task]]`) is valid YAML *flow sequence* syntax. It parses as a list containing a list, not as a string.
- Any vault-cli write path re-emits that value as a nested block sequence (`- - Some Task`), silently destroying the wikilink. The key survives; the link does not.
- No error, no warning, no exit code change. Obsidian stops rendering the link and the backlink vanishes from the graph.
- All six entity types (Task, Goal, Theme, Objective, Vision, Decision) share one parse chokepoint and one serialize chokepoint, so all six are affected identically.
- After the fix, a bare wikilink is quoted before YAML ever sees it, so it round-trips as a working wikilink forever, and already-quoted wikilinks are untouched.

## Problem

`[[X]]` is well-formed YAML. `yaml.Unmarshal` accepts it without complaint and produces `[]any{[]any{"X"}}`. `yaml.Marshal` faithfully writes that structure back as a nested block sequence. Neither side is wrong on its own terms — the corruption emerges only from the round-trip, which is why nothing detects it.

The damage is silent and delayed: a page is hand-authored (or agent-authored) with a bare wikilink, sits correct on disk for weeks, and is then destroyed by an unrelated command that happened to touch the file. Because the *key* is preserved, audits that compare which frontmatter keys survived a write pass this cleanly — only the value's type changed. Exposure grows unnoticed with every bare wikilink authored.

## Goal

A frontmatter value that is a bare Obsidian wikilink survives every vault-cli read/write cycle as a working wikilink, for all six entity types, without the operator having to remember to quote it by hand.

## Non-goals / Out of Scope

- Do NOT rewrite `serializeMapAsFrontmatter`'s broader structure. Spec 030 froze that function; this spec does not touch it at all (the fix lands on the parse side).
- Do NOT rewrite a bare wikilink followed by a trailing YAML comment (`related_task: [[X]]  # migrated 2026-01`). The value portion is not exactly a wikilink, so the pass skips it and the bug persists on that line. Accepted: zero such lines exist across the measured vaults, and comment-aware parsing is a materially larger surface. See Failure Modes.
- Do NOT auto-repair wikilinks vault-wide. Exactly one affected file is known (see Reproduction); repairing it is an operator step, not a code feature. No sweep command, no `--fix` flag, no migration.
- Do NOT change how quoted wikilinks are authored or documented elsewhere. The quoted form is already correct and stays the recommended authoring style.
- Do NOT fix the decision-ack whitelist bug — separate, already tracked under spec 030.
- Do NOT add a config key, flag, or env var to opt out of the quoting. It is an invariant.
- Do NOT extend the quoting to multi-line YAML flow collections, block scalars, or wikilinks embedded in a longer string (`title: see [[X]] for details`). Only a value that is *exactly* a bare wikilink is rewritten. See Constraints.

## Acceptance Criteria

- [ ] A task file whose frontmatter contains `related_task: [[Some Other Task]]` and `themes: [[A Theme]]`, after `vault-cli task set <name> priority 3`, contains `related_task: '[[Some Other Task]]'` and `themes: '[[A Theme]]'`. Evidence: file content — `grep -c '^related_task: .\[\[Some Other Task\]\].$' <file>` returns `1`, and `grep -c '^ *- - ' <file>` returns `0`.
- [ ] The same round-trip produces a working wikilink for Goal, Theme, Objective, and Vision, driven through each entity's set path — `ops.NewGoalSetOperation` / `NewThemeSetOperation` / `NewObjectiveSetOperation` / `NewVisionSetOperation`, the same composition `pkg/cli/cli.go` builds behind `vault-cli <entity> set`. Driving the ops layer rather than shelling the compiled binary is deliberate: the container has no installed binary, and the ops path still crosses find → set → write. Evidence: `go test -v ./pkg/ops/ -args -ginkgo.v -ginkgo.no-color -ginkgo.focus 'Wikilink'` exits 0 and, for each of the four entities, the written file contains `related_task: '[[Some Other Task]]'` and no `- - ` line.
- [ ] The same round-trip produces a working wikilink for Decision. Decision has no `set` or `get` subcommand — `vault-cli decision` exposes only `ack` and `list`, and `ack` is operator-gated — so this is driven by a Go test calling `decisionStorage.WriteDecision` directly against a temp vault. Evidence: `go test -v ./pkg/storage/ -args -ginkgo.v -ginkgo.no-color -ginkgo.focus 'Wikilink'` exits 0 and the Decision case asserts the written file contains `related_task: '[[Some Other Task]]'` and no `- - ` line.
- [ ] An already-quoted wikilink is byte-identical after a write. Evidence: negative — a file authored with `related_task: '[[Some Task]]'` shows `git diff --numstat <file>` reporting `0` insertions and `0` deletions for that line after `task set`.
- [ ] The quoted block-sequence list form (`themes:` with `    - '[[A Theme]]'` entries) is byte-identical after a write. Evidence: negative — `git diff <file>` shows no change to the `themes:` block.
- [ ] A bare wikilink appearing as a block-sequence *entry* (`themes:` followed by `    - [[A Theme]]`) round-trips as a quoted entry, not as a nested list. Evidence: file content — after `task set`, `grep -c "^ *- '\[\[A Theme\]\]'$" <file>` returns `1` and `grep -c '^ *- - ' <file>` returns `0`.
- [ ] A wikilink containing a literal single quote (`[[Ben's Task]]`) round-trips to a valid, re-parseable YAML scalar. Evidence: after `task set`, `vault-cli task get <name> related_task` exits 0 (proving the file still parses) and its stdout equals `[[Ben's Task]]`.
- [ ] Wikilinks with an alias (`[[X|alias]]`) and a heading anchor (`[[X#Section]]`) round-trip unchanged inside the quotes. Evidence: file content — the post-write line contains the verbatim substring `[[X|alias]]` / `[[X#Section]]`.
- [ ] A frontmatter value that merely *contains* a wikilink among other text (`title: see [[X]] for details`) is not rewritten into a quoted-wikilink scalar and keeps its original meaning. Evidence: after `task set`, `vault-cli task get <name> title` exits 0 and its stdout equals `see [[X]] for details`.
- [ ] The fix is idempotent: running `task set` twice in a row on a bare-wikilink file produces no diff on the second run. Evidence: negative — `git diff --numstat <file>` after the second `task set` reports `0` insertions and `0` deletions.
- [ ] Unit tests cover the bare-scalar, bare-list-entry, already-quoted, single-quote-in-title, alias, anchor, embedded-in-longer-string, and idempotence cases, and they exercise the shared parse path used by all six entity types. Evidence: `go test ./pkg/storage/...` exits 0 and the new test names appear in `go test -v ./pkg/storage/ -args -ginkgo.v -ginkgo.no-color -ginkgo.focus 'Wikilink'` output.
- [ ] `make precommit` exits 0 at the repository root. Evidence: exit code 0.
- [ ] `CHANGELOG.md` contains an `## Unreleased` bullet prefixed `fix:` describing the wikilink round-trip fix, and no version string in `.claude-plugin/` was hand-edited. Evidence: `grep -A20 '^## Unreleased' CHANGELOG.md` returns a line starting with `- fix:`; and `git diff origin/master -- .claude-plugin/` is empty.
- [ ] **Post-Deploy (Rung-2):** the released version is installed locally and reports the released tag. Evidence: `vault-cli --version` stdout ends with the released tag.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** the one known affected file — Trading vault `40 Decisions/TDR 2026-01-09 - USDJPY ORB V11 Volatility Filter Deployment.md` — has a working `related_task` wikilink after a hand-repair to the quoted form. Evidence: file content — `grep -c "^related_task: '\[\[Validate USDJPY priceRangeMaxPercent Filter Finding\]\]'$" <file>` returns `1`. No vault-cli write is fired against this file: `decision ack` is the only Decision write path and it is operator-gated, so write-survival for Decision is proven on a scratch copy instead (next AC).
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`
- [ ] **Post-Deploy (Rung-2):** a byte-copy of that TDR, placed in a scratch vault and written through `vault-cli decision ack`, still holds the quoted wikilink afterwards. Evidence: file content — `grep -c "^related_task: '\[\[Validate USDJPY priceRangeMaxPercent Filter Finding\]\]'$" <scratch-copy>` returns `1` after the `ack`, and `grep -c '^ *- - ' <scratch-copy>` returns `0`.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`

**Scenario coverage: no new scenario.** The behavior is a pure content transformation on the parse path, fully reachable from unit tests against a temporary vault directory. The existing `scenarios/001`–`004` still run unchanged as the release gate. None of the four conditions in dark-factory `docs/rules/scenario-writing.md` holds here.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit
make test
go test -v ./pkg/storage/ -args -ginkgo.v -ginkgo.no-color -ginkgo.focus 'Wikilink'
go test -v ./pkg/ops/     -args -ginkgo.v -ginkgo.no-color -ginkgo.focus 'Wikilink'
grep -A20 '^## Unreleased' CHANGELOG.md          # must contain a "- fix:" bullet
```

### Operator-executable (runs on the host after merge)

```
# Release gate — mandatory before make install, per docs/releasing-vault-cli.md
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .
/tmp/new-vault-cli --version
ls scenarios/*.md        # walk each scenario's Action + Expected against /tmp/new-vault-cli

# Reproduction replay against the fresh binary — the mandatory bug-workflow step
D=/tmp/wlrepro && rm -rf $D && mkdir -p "$D/24 Tasks"
printf 'vaults:\n  repro:\n    path: %s\n    tasks_dir: 24 Tasks\n' "$D" > $D/config.yaml
printf -- '---\npriority: 2\nrelated_task: [[Some Other Task]]\nstatus: in_progress\nthemes: [[A Theme]]\n---\nTags: [[Task]]\n\n---\nbody\n' > "$D/24 Tasks/Repro Task.md"
/tmp/new-vault-cli --config $D/config.yaml --vault repro task set "Repro Task" priority 3
grep -c '^ *- - ' "$D/24 Tasks/Repro Task.md"                 # must be 0
grep -c "^related_task: '\[\[Some Other Task\]\]'$" "$D/24 Tasks/Repro Task.md"   # must be 1
# Idempotence: a second write must leave the line unchanged
/tmp/new-vault-cli --config $D/config.yaml --vault repro task set "Repro Task" priority 4
grep -c "^related_task: '\[\[Some Other Task\]\]'$" "$D/24 Tasks/Repro Task.md"   # must still be 1

# Version alignment before any plugin release
make release-check

# Install
make install
vault-cli --version

# Repair the one affected TDR by hand (quote the wikilink), then confirm
grep -c "^related_task: '\[\[Validate USDJPY priceRangeMaxPercent Filter Finding\]\]'$" \
  "$HOME/Documents/Obsidian/Trading/40 Decisions/TDR 2026-01-09 - USDJPY ORB V11 Volatility Filter Deployment.md"

# Write-survival for Decision: scratch copy only, never the live TDR
# (decision ack is the sole Decision write path and is operator-gated)
```

## Reproduction

Confirmed 2026-08-09 against installed `vault-cli version v0.104.0`.

**Setup** — scratch vault with a `24 Tasks/` directory and a config file:

```yaml
vaults:
  repro:
    path: /tmp/wlrepro
    tasks_dir: 24 Tasks
```

`24 Tasks/Repro Task.md`:

```
---
priority: 2
related_task: [[Some Other Task]]
status: in_progress
themes: [[A Theme]]
---
Tags: [[Task]]

---
body
```

**Action:**

```
$ vault-cli --config /tmp/wlrepro/config.yaml --vault repro task set "Repro Task" priority 3
✅ Set priority=3 on: Repro Task
```

**Observed evidence (verbatim, file content after the command):**

```
---
priority: 3
related_task:
    - - Some Other Task
status: in_progress
task_identifier: 22ba84c4-9e2a-474d-bad1-4f16386369f9
themes:
    - - A Theme
---
Tags: [[Task]]

---
body
```

Exit code 0. Nothing on stderr. Both wikilinks are destroyed.

**Read paths verified NOT affected** (same binary, same session): a file containing `themes: [[A Theme]]` was left byte-identical by both `vault-cli task get "Read Only" status` (stdout `in_progress`) and `vault-cli task list`. The corruption requires a write.

**Real-world instance:** Trading vault `40 Decisions/TDR 2026-01-09 - USDJPY ORB V11 Volatility Filter Deployment.md:7` currently holds `related_task: [[Validate USDJPY priceRangeMaxPercent Filter Finding]]` — the one bare-wikilink file found by `grep -rlE "^[a-z_]+: \[\[" "40 Decisions/"` on 2026-08-09 (1 of 25 TDRs; 0 of all Personal tasks and goals). It was destroyed once already and hand-repaired; with v0.104.0 installed, the next vault-cli write against it destroys it again.

## Expected vs Actual

| Behavior | Expected | Actual (verified 2026-08-09, v0.104.0) |
|---|---|---|
| Bare wikilink scalar (`k: [[X]]`) after a write | Quoted scalar `k: '[[X]]'`, link intact | Nested block sequence `k:\n    - - X`, link destroyed |
| Bare wikilink as a list entry (`- [[X]]`) after a write | Quoted entry `- '[[X]]'` | Triple-nested list `- - - X` (verified 2026-08-09 — the entry's own sequence level adds one on top of the flow sequence's two) |
| Already-quoted wikilink after a write | Unchanged | **Already correct** — unchanged |
| Quoted block-sequence list after a write | Unchanged | **Already correct** — unchanged |
| Pure read commands (`task get`, `task list`) | File untouched | **Already correct** — untouched |
| Operator notification when a value's type changes | Not applicable once the value is preserved | Silent — exit 0, no stderr |

The write contract is `pkg/storage/base.go`'s `serializeMapAsFrontmatter` (`:72`), whose doc comment promises only that "fields are written in YAML library key order… preserving the markdown body". It says nothing about value fidelity, and the parse side (`parseToFrontmatterMap`, `:49`) hands it a structure that is already wrong. This is a design gap at the parse boundary, not an implementation slip in the serializer.

## Why this is a bug

Silent data loss. The vault's link graph is load-bearing — backlinks are how goals find their tasks, how decisions find their evidence, and how `search_related` surfaces related pages. Destroying a wikilink removes a graph edge with no error, no warning, and no exit-code signal, so nothing prompts the operator to look. It is strictly worse than a loud failure: the operator's own audit technique for this class of corruption (compare which frontmatter keys survived a write) passes cleanly, because the key *is* preserved — only its type silently changed from string to nested list.

It also violates the round-trip expectation the storage layer already meets elsewhere: spec 008 (`008-flexible-frontmatter-refactor.md:16,28`) established map-based frontmatter precisely so "unknown YAML fields survive round-trip without code changes", and states at `:41` that unknown fields are stored as strings with no automatic type coercion. A bare wikilink is exactly such an unknown field — and it is silently type-coerced from string to nested list, which is the guarantee inverted.

## Desired Behavior

1. Before `yaml.Unmarshal` runs in `pkg/storage/base.go`'s `parseToFrontmatterMap` (`:49`), the raw frontmatter text is passed through a quoting pass that wraps bare wikilinks in single quotes. YAML therefore never sees a flow sequence, and the in-memory map holds a plain `string` for that key.
2. The quoting pass rewrites a line only when the value is *exactly* a bare wikilink — the value portion, after trimming surrounding whitespace, matches `[[...]]` from first character to last. Two line shapes qualify: a mapping value (`key: [[X]]`, at any indentation) and a block-sequence entry (`- [[X]]`, at any indentation).
3. The pass changes quoting only, never YAML shape. A bare wikilink under a conventionally-list key (`themes: [[A Theme]]`) becomes the quoted **scalar** `themes: '[[A Theme]]'` — it is not normalised into a one-element block sequence. The authored form is preserved as authored.
4. A value that is already quoted (single or double) is left byte-identical. A value that merely contains a wikilink among other text is left byte-identical. A value spanning multiple lines is left byte-identical.
5. Single quotes inside the wikilink title are escaped by doubling (`[[Ben's Task]]` → `'[[Ben''s Task]]'`), so the emitted scalar re-parses to the original title.
6. Because the map now holds a string, the existing `serializeMapAsFrontmatter` (`:72`) writes it back as a quoted scalar with no change to that function.
7. All six entity types are covered by the single shared parse chokepoint, with no per-entity code.
8. Consumers reading `RawMap()` receive the wikilink as a `string`, not as a nested list — so downstream code that renders or compares the value sees the authored form.

## Constraints

- `serializeMapAsFrontmatter` (`pkg/storage/base.go:72`) is not modified. The fix lands entirely on the parse side.
- **This spec knowingly reverses half of spec 030's constraint.** Spec 030 (`specs/in-progress/030-bug-decision-ack-destroys-frontmatter.md:171`) states "`serializeMapAsFrontmatter` and `parseToFrontmatterMap` are not modified — they already behave correctly." That was true for 030's scope; this spec's Reproduction proves the parse side does *not* behave correctly for bare wikilinks. 030's code is merged (PR #67, shipped in `v0.104.0`) and the spec is at `status: verifying` with an empty prompt queue, so there is no execution conflict — 030's constraint bounds 030's own work, not this one. Do not treat it as a block.
- The quoting pass operates on raw frontmatter text before `yaml.Unmarshal`, mirroring the precedent set by spec 026's duplicate-key detection (detection cannot route through unmarshal when unmarshal is what mangles the input).
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. This spec adds no output of any kind. See `docs/development-patterns.md`.
- Unknown frontmatter fields continue to survive read-write cycles. The pass adds quotes to bare wikilinks and changes nothing else — key order, indentation, and every other value stay as the existing serializer produces them.
- No new exported interface, no new return value, no signature change to `parseToFrontmatterMap` or any caller.
- No new config key, flag, or env var.
- Exit codes are unchanged for every command.
- All existing scenarios `scenarios/001`–`004` continue to pass against a freshly built binary — the mandatory release gate in `docs/releasing-vault-cli.md`, not optional.
- Version strings are not hand-edited. Add an `## Unreleased` bullet prefixed `fix:`; the release flow owns the version bump and tag. See `docs/releasing-vault-cli.md`.

## Failure Modes

| Trigger | Detection | Expected behavior | Recovery | Reversibility |
|---|---|---|---|---|
| A genuine one-element nested YAML list is authored inline (`foo: [[a]]`) | Post-write `grep` of the file shows a quoted scalar where a nested list was intended | Accepted collision — the value is quoted as a wikilink. Indistinguishable from a bare wikilink by construction, and no vault frontmatter uses inline nested lists | Operator authors the nested list in block form (`foo:` then `    - - a`), which the pass never touches | Reversible via `git` |
| A wikilink title contains `[` or `]` (`[[Foo [bar]]]`) | Unit test / post-write re-parse | The regex matches the outermost `[[`…`]]` pair on the line; the quoted result re-parses to the full title | Operator quotes it by hand | Reversible via `git` |
| Frontmatter contains no wikilinks at all | Negative — `git diff` after a write shows only the intended field change | Pass is a no-op; zero behavioral difference from today | None needed | N/A |
| A file's frontmatter is unparseable for an unrelated reason (duplicate key, syntax error) | Existing parse error surfaces as it does today | Unchanged — the quoting pass is a text transform that cannot introduce a parse error it did not receive; the pre-existing error still propagates | Operator runs `vault-cli task lint --fix` (spec 026) | Reversible — no write occurred |
| The quoting pass itself produces invalid YAML (escaping bug) | `yaml.Unmarshal` immediately after the pass returns an error naming the line | Parse fails loudly with the existing wrapped error; no write happens, file left byte-identical | Operator reports it; the file is untouched | Reversible — read-only path |
| A bare wikilink carries a trailing YAML comment (`k: [[X]]  # note`) | Post-write `grep` still shows `- - X` on that line | Not rewritten — the value portion is not exactly a wikilink, so the bug persists there. Declared in Non-goals | Operator quotes that line by hand | Reversible via `git` |
| obsidian-git autocommits a vault file mid-write | `git log` on the vault shows interleaved autocommits | Write is a whole-file replace; a concurrent commit captures either the pre- or post-write content, never a torn file | Re-run the command | Reversible via `git` |
| An already-destroyed file (`- - X`) is written again | Post-write `grep` still shows `- - X` | Unchanged — the pass fixes bare wikilinks, it does not resurrect already-destroyed ones. The nested list is now a faithfully preserved nested list | Operator hand-repairs the file to the quoted form; subsequent writes preserve it | Reversible via `git` |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Add the bare-wikilink quoting pass to `parseToFrontmatterMap` in `pkg/storage/base.go`, plus unit tests for bare scalar, bare list entry, already-quoted, quoted list, single-quote-in-title, alias, anchor, embedded-in-longer-string, multi-line, and idempotence | 1, 2, 3, 4, 5, 6 | 4, 5, 6, 7, 8, 9, 10, 11 | — |
| 2 | Round-trip tests driving all six entity types through their own storage write path (`Task`/`Goal`/`Theme`/`Objective`/`Vision` plus `decisionStorage.WriteDecision` for Decision, which has no `set` command), asserting the wikilink survives as a quoted scalar | 7, 8 | 1, 2, 3 | prompt 1 |
| 3 | `## Unreleased` changelog entry with a `fix:` bullet; add the YAML-flow-sequence invariant to `docs/development-patterns.md` next to the existing frontmatter-preservation note; repo-wide precommit | — | 12, 13 | prompts 1, 2 |

Rationale: prompt 1 is the whole behavioral change and carries the bulk of the coverage; prompt 2 proves the single chokepoint really does cover all six entities rather than assuming it; prompt 3 runs last so the changelog describes both landed changes and the invariant is documented against the final shape. ACs 14, 15, and 16 are operator-side — install verification, the Trading TDR repair, and the scratch-copy Decision write — and are not container-executable.

## Alternatives Considered

| Option | Rejected because |
|---|---|
| **Coerce the shape after unmarshal** — walk the map and convert `[]any{[]any{"X"}}` back to a `"[[X]]"` string | After parse, a mangled wikilink and a genuine one-element nested list are byte-identical structures. The information needed to tell them apart exists only in the raw text, and it is destroyed by the time this check would run. Raw-text quoting preserves that distinction for free. |
| **Coerce the shape at serialize time** | Same ambiguity as above, plus it leaves the in-memory value wrong: consumers reading `RawMap()` still see a nested list. Only the bytes on disk would be repaired. |
| **Decode via `yaml.Node` instead of `map[string]any`** | Preserves original scalar styles exactly, but requires changing `parseToFrontmatterMap`'s return type and every caller — a signature change across all six entities, contradicting the no-new-interface constraint. Disproportionate to a one-shape bug. |
| **Require quoting at authoring time (doc + lint rule only)** | Depends on perfect recall from human and agent authors who get no signal when they get it wrong — the exact failure that produced the known instance. Documented as the interim Workaround, not the fix. |

The chosen option also matches the precedent set by spec 026, whose duplicate-key detection runs on raw frontmatter text before any unmarshal for the same reason: unmarshal is the operation that mangles the input.

## Assumptions

- All six entity read paths continue to funnel through `parseToFrontmatterMap`. Verified 2026-08-09 by call-site grep; if a seventh entity or a bypassing read path is added later, it must route through the same chokepoint or it reintroduces the bug.
- Vault frontmatter never intentionally uses inline nested YAML lists. Verified by inspection across the Personal and Trading vaults; the accepted collision in Failure Modes depends on this.
- The quoted-scalar form (`'[[X]]'`) is what Obsidian renders as a working link. Verified — this is the form already used by every `themes:` block in both vaults.
- Exactly one bare-wikilink file exists across the measured vaults as of 2026-08-09. The repair AC covers that file; new bare wikilinks authored before the fix ships would need the same treatment.

## Do-Nothing Option

Not acceptable. Doing nothing leaves a silent link-destroying write path in the tool that every vault workflow runs dozens of times a day. The corruption is undetectable by the operator's existing audit technique, unbounded in duration, and grows with every bare wikilink authored — including by agents, which have no habit of quoting. The manual workaround (remember to quote every wikilink, forever, in every hand-authored and agent-authored page) depends on perfect recall from writers who have no signal when they get it wrong, which is exactly the failure that produced the one known instance.

## Workaround (until the fix ships)

Author every frontmatter wikilink in quoted form — `related_task: '[[Some Task]]'` — or in the quoted block-sequence list form. Both already round-trip cleanly. To find existing exposure in a vault:

```bash
grep -rlE "^[a-z_]+: \[\[" .
```

Hand-quote every file returned before running any vault-cli write against it.
