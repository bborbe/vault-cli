---
status: completed
summary: Excluded attention-board answers, ! shell input and rendered prompt echoes from the operator-prompt count via a new always-applied prefix list, extended the fixture, updated docs and CHANGELOG
execution_id: vault-cli-operator-prompt-tighten-exec-260-tighten-operator-prompt-predicate
dark-factory-version: v0.198.7
created: "2026-10-19T19:45:16Z"
queued: "2026-10-19T20:04:10Z"
started: "2026-10-19T20:06:06Z"
completed: "2026-10-19T20:09:57Z"
---

# Exclude attention-board answers, shell input and rendered prompt echoes from the operator-prompt count

<summary>
- The operator-prompt count stops counting three classes of transcript row that are not prompts the operator typed
- Attention-board answers — text relayed from the board — no longer count as typed prompts
- Shell input entered with the `!` prefix no longer counts as a typed prompt
- Rendered prompt echoes — the redrawn prompt block written back into the transcript — no longer count as typed prompts
- The exclusion works for both old-style transcript rows and current ones, not only the old-style rows the existing noise filter covers
- Slash-command prompts keep counting exactly as before; scheduled wakeups, peer messages and tool results stay excluded exactly as before
- The all-interaction count is untouched and still counts every user turn
- A regression fixture covers each new excluded class in both transcript shapes, and a sabotage check proves the fixture fails if the filter is loosened
- The weekly operator-prompt figure falls back inside its ±5% acceptance bound
</summary>

<objective>
Tighten the operator-typed prompt predicate so it no longer counts three measured classes of row that pass every existing signal check but are not prompts the operator typed: attention-board answers, `!` shell input, and rendered prompt echoes. A one-day validation measured 5,020 operator prompts against 89,711 interactions, and a stratified manual sample found 611 of the counted entries (12.2%) were these three classes — against an acceptance bound of ±5%. Excluding them brings the figure back inside its bound without changing the all-interaction count or the slash-command behaviour.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` for the Definition of Done (doc comments, Ginkgo v2 / Gomega, no output from `pkg/ops/`, CHANGELOG placement).

Read these files fully before editing:

- `pkg/ops/interaction_count.go` — the operator-prompt predicate. `CountOperatorPrompts` (delegates to `sum` with `countOperatorPromptsInSessionLog`), `isOperatorPrompt`, `operatorPromptText`, `operatorPromptSources`, `operatorPromptNoisePrefixes`, `hasOperatorPromptNoisePrefix`. Note that `operatorPromptNoisePrefixes` is applied **only** when the row is legacy (`entry.TurnOrigin == "" && entry.PromptSource == ""`).
- `pkg/ops/interaction_count_test.go` — the existing `Describe("InteractionCounter", …)` suite, its `writeFixture(id string, lines ...string)` helper, and the spec `Counts only operator prompts while the interaction count counts every user turn`, whose fixture currently asserts **7** operator prompts against **21** interactions.
- `commands/complete-task.md` § Passive metrics — the canonical description of what completion writes, which enumerates the predicate's exclusions.
- `README.md` — the `rollup weekly` paragraph that describes the operator-prompt figure and its exclusions.
- `CHANGELOG.md` — there is **no** `## Unreleased` section today; the newest heading is `## v0.172.0`, which shipped `metrics_operator_prompt_count`. `scripts/check-changelog.sh` fails the build when a `## ` section appears above the preamble.

### Measured evidence — read from real transcripts on 2026-10-19, do not re-derive

Validation day 2026-10-18: 158 sessions, `metrics_operator_prompt_count: 5020`, `metrics_interaction_count: 89711`. Stratified manual sample of the 5,020 counted entries: 611 (12.2%) are not operator-typed prompts.

| Class | Count in sample | Recognised by |
|---|---|---|
| Attention-board answers | 382 | text starts with `Operator answer, via attention board:` |
| `!` shell input | 145 | text starts with `<bash-input>` |
| Rendered prompt echo | 84 | text starts with `\u276f ` (U+276F followed by a space) |

All three currently pass `isOperatorPrompt` because they carry no `toolUseResult`, no `tool_result` block, `isMeta` is not true, `turnOrigin` is `human` (or empty) and `promptSource` is `typed` (or empty).

The structural search was performed on the authoring host and is recorded here so the implementing agent does not repeat it (that transcript directory is not mounted in the container and is not re-read). Field distributions observed for the rows of each class:

| Class | `turnOrigin` | `promptSource` | `origin.kind` | `entrypoint` | `permissionMode` |
|---|---|---|---|---|---|
| Attention-board answers | `human` | `typed` (7 of 9) / `queued` (2 of 9) | `human` | `cli` | `auto` |
| `!` shell input | absent | absent | absent | `cli` | absent |
| Rendered prompt echo | `human` | `typed` | `human` | `cli` | `auto` |

**Conclusion the implementing agent must carry forward:** no structural field distinguishes attention-board answers or rendered echoes from a genuine typed prompt — `origin.kind` is `human`, `promptSource` is `typed`/`queued`, `entrypoint` and `permissionMode` match. The text prefix is the only discriminator for those two classes. `!` shell input rows are legacy rows in the sample (every signal field absent), but its prefix is treated the same way as the other two so a modern row carrying it is excluded as well. Record this finding in the new prefix list's doc comment.

Settled decision, do not re-litigate: attention-board answers are operator *input* but not operator *typed prompts*. The metric's criterion says typed, so they are excluded.

Slash commands are already correct and must not change: 1,197 `turnOrigin: human` plus 526 legacy slash-command rows counted, 2,311 `scheduled` rows excluded. `<command-name>` and `<command-message>` rows must keep counting.

Coding-plugin docs (in-container paths — never use host paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and the conventional prefixes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the DoD checklist.
</context>

<requirements>
1. **Extend the predicate in `pkg/ops/interaction_count.go`.** Keep the existing legacy noise list's semantics unchanged; add a second list for prefixes that must fire on modern rows too.

   a. Add a new package-level slice immediately below `operatorPromptNoisePrefixes`, with the measured evidence in its doc comment:

   ```go
   // operatorPromptAlwaysNoisePrefixes are transcript rows that pass every signal
   // check — no toolUseResult, no tool_result block, isMeta not set, turnOrigin
   // "human", promptSource "typed"/"queued" — but are not prompts the operator
   // typed. Unlike operatorPromptNoisePrefixes these apply to modern rows as well
   // as legacy ones: measured on the 2026-10-18 sample, attention-board answers and
   // rendered prompt echoes carry turnOrigin "human" and promptSource
   // "typed"/"queued", with origin.kind "human", and no structural field
   // distinguishes them from a genuine prompt, so the text prefix is the only
   // discriminator. Shell input rows were legacy in the same sample; the prefix is
   // listed here so a modern row carrying it is excluded too.
   var operatorPromptAlwaysNoisePrefixes = []string{
   	"Operator answer, via attention board:",
   	"<bash-input>",
   	"\u276f ",
   }
   ```

   `"\u276f "` is the rendered prompt glyph followed by one space. Keep it as the escape, not a literal character, so the file stays ASCII-safe.

   b. Widen the existing helper to take the prefix list, and update its one call site:

   ```go
   // hasOperatorPromptNoisePrefix reports whether text starts with one of the given
   // non-operator prefixes, after trimming surrounding whitespace.
   func hasOperatorPromptNoisePrefix(text string, prefixes []string) bool {
   	trimmed := strings.TrimSpace(text)
   	for _, prefix := range prefixes {
   		if strings.HasPrefix(trimmed, prefix) {
   			return true
   		}
   	}
   	return false
   }
   ```

   c. In `isOperatorPrompt`, apply the always list to every row before the legacy gate, and keep the legacy list gated exactly as today:

   ```go
   	if hasOperatorPromptNoisePrefix(text, operatorPromptAlwaysNoisePrefixes) {
   		return false
   	}
   	legacy := entry.TurnOrigin == "" && entry.PromptSource == ""
   	if legacy && hasOperatorPromptNoisePrefix(text, operatorPromptNoisePrefixes) {
   		return false
   	}
   ```

   d. Update the doc comments so they match the code: on `operatorPromptNoisePrefixes`, state that it applies to legacy rows only; on the `InteractionCounter` interface's `CountOperatorPrompts` method, replace "and not one of the known legacy noise prefixes" with a phrase naming all four exclusion families — tool results, meta injections, non-human origins / non-operator prompt sources, and the known non-prompt text prefixes (legacy noise, attention-board answers, shell input, rendered prompt echoes).

   e. Change nothing else. Do not add fields to `operatorPromptEntry`, do not decode `origin`, do not touch `countUserTurnsInSessionLog`, `isSafeSessionID`, `encodeProjectDir`, `sum`, `metricsSessionIDs` or `earliestStartedAt`, and do not change `Count`. The interface is unchanged, so no mock regeneration is needed.

2. **Extend the regression fixture** in `pkg/ops/interaction_count_test.go`, inside the existing spec `Counts only operator prompts while the interaction count counts every user turn`, so it reuses `ctx`, `counter` and `writeFixture`.

   a. Append exactly these six lines to the excluded block of `operatorPromptFixture`, grouped and commented as shown — one line per new class in each transcript shape — and change that block's `// excluded — 14 type:"user" lines` comment to `// excluded — 20 type:"user" lines`:

   ```go
   	// excluded — attention-board answers, both shapes (2 lines)
   	`{"type":"user","turnOrigin":"human","promptSource":"typed","message":{"role":"user","content":"Operator answer, via attention board: approve: /x"}}`,
   	`{"type":"user","message":{"role":"user","content":"Operator answer, via attention board: approve: /x"}}`,
   	// excluded — ! shell input, both shapes (2 lines)
   	`{"type":"user","turnOrigin":"human","promptSource":"typed","message":{"role":"user","content":"<bash-input>ls</bash-input>"}}`,
   	`{"type":"user","message":{"role":"user","content":"<bash-input>ls</bash-input>"}}`,
   	// excluded — rendered prompt echo, both shapes (2 lines)
   	`{"type":"user","turnOrigin":"human","promptSource":"typed","message":{"role":"user","content":"\u276f /reload-plugins\n  \u23bf  Reloaded: 12 plugins"}}`,
   	`{"type":"user","message":{"role":"user","content":"\u276f /reload-plugins\n  \u23bf  Reloaded: 12 plugins"}}`,
   ```

   The `\u276f` and `\u23bf` escapes are JSON string escapes inside Go raw strings, so `json.Unmarshal` decodes them to the glyphs — the fixture file stays ASCII.

   b. Update the assertions explicitly, leaving every existing line of the fixture unchanged: the seven operator-typed lines still count, so `CountOperatorPrompts` still equals **7**; the excluded `type:"user"` lines are now 20 (14 existing + 6 new), so `Count` now equals **27**. Update the comment beside the interaction assertion from `(7 + 14)` to `(7 + 20)`. The missing-file and unsafe-id assertions stay as they are.

   c. Add a second, small spec named exactly `Counts a modern row carrying a legacy-only noise prefix` — the lowercase phrase `legacy-only` is the focus string `<verification>` uses, and Ginkgo's focus is case-sensitive — that writes a fixture with one modern row carrying a legacy-list prefix and asserts it still counts, proving the legacy list was not blanket-widened onto modern rows:

   ```go
   	writeFixture("s1", `{"type":"user","turnOrigin":"human","promptSource":"typed","message":{"role":"user","content":"<local-command-stdout>Compacted</local-command-stdout>"}}`)
   	Expect(counter.CountOperatorPrompts(ctx, []string{"s1"})).To(Equal(1))
   ```

   Add a short comment on that spec stating it locks the deliberate split between the two prefix lists.

3. **Keep the falsification property.** With `CountOperatorPrompts` delegating to `Count`, the fixture must fail. Perform it with filesystem copies, never `git` (a dead `git` command still leaves a false pass):

   a. `cp pkg/ops/interaction_count.go /tmp/interaction_count.go.fixed`
   b. Edit `CountOperatorPrompts` so its body is exactly `return c.Count(ctx, sessionIDs)`, leaving the method signature and doc comment in place.
   c. Run the focused spec and confirm it **FAILS** (non-zero exit, got 27, want 7), capturing the output:
      ```
      go test ./pkg/ops/... -count=1 -ginkgo.focus='operator prompts' > /tmp/falsify.log 2>&1
      grep -nE '<int>: 27' /tmp/falsify.log
      grep -nE '<int>: 7' /tmp/falsify.log
      ```
      The grep must print the 27-vs-7 mismatch. If it prints nothing, the spec did not run or did not discriminate — do not proceed. Do not weaken the assertion to make it pass.
   d. Restore: `cp /tmp/interaction_count.go.fixed pkg/ops/interaction_count.go`
   e. Confirm the restore: `grep -n 'return c.sum(ctx, sessionIDs, countOperatorPromptsInSessionLog)' pkg/ops/interaction_count.go` matches and `grep -n 'return c.Count(ctx, sessionIDs)' pkg/ops/interaction_count.go` prints nothing.
   f. Re-run the focused spec and confirm it now **PASSES**.

   The restored file is what gets committed — a tree left in the sabotaged state is a failed prompt.

4. **Update the two places that describe the predicate.**

   a. `commands/complete-task.md` § Passive metrics: extend the `metrics_operator_prompt_count` sentence so its exclusion list also names attention-board answers, `!` shell input and rendered prompt echoes, and keep the existing clause that `<command-name>` and `<command-message>` slash commands still count.

   b. `README.md`, the `rollup weekly` paragraph: extend the operator-prompts parenthetical's exclusion list the same way. Change nothing else in the paragraph.

5. **Add one `## Unreleased` `fix:` bullet to `CHANGELOG.md`.** The section does not exist today: create it below the preamble's last line (`* PATCH version when you make backwards-compatible bug fixes.`) and above `## v0.172.0` (or above whatever the newest `## vX.Y.Z` heading is if a release landed meanwhile). One flat bullet, no category heading, no date suffix, ending with a `Change set:` clause. It must carry: the three excluded classes and how each is recognised; that attention-board answers are operator input but not typed prompts, which is why they are excluded; that the exclusion applies to modern and legacy transcript rows; that slash-command rows, the interaction count and the stored metric's meaning are unchanged; and that the 2026-10-18 sample measured 611 over-counted entries (12.2%) against a ±5% acceptance bound. Begin the bullet `- fix: operator-prompt count …` so the verification's `awk` matches it.

6. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation. Walk the change: the predicate excludes all three classes in both transcript shapes; the legacy list still fires only on legacy rows; the seven operator-typed lines and the slash-command lines still count; `Count` still counts every user turn (27); the falsification failed against the loosened filter and passed once restored; both docs name the new exclusions; the CHANGELOG bullet is under `## Unreleased`; `make precommit` and `make test` are green. State in your final report which signal was found for each class (structural field versus text prefix) and why the structural search came up empty, so the measurement decision is recorded with the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage, commit, tag or push.
- Do NOT bump any version string. `.dark-factory.yaml` sets `autoRelease: false` and `.maintainer.yaml`'s github-releaser owns the bump and tag post-merge. Do not edit `CHANGELOG.md`'s newest `## vX.Y.Z` section, `.claude-plugin/plugin.json`, or either version field in `.claude-plugin/marketplace.json`, and do not run `git tag`, `make release-check` or `make check-versions`.
- `metrics_interaction_count` behaviour is frozen: every `type:"user"` line still counts, the same session-id dedup and unsafe-id guard apply. Do not touch `rollupFigures`, `RollupWeeklyResult`, `pkg/cli/rollup.go`, the baseline file or the delta block.
- Slash-command counting is frozen: `<command-name>` and `<command-message>` rows keep counting, and the `scheduled` / `task_notification` / `peer` / `system` exclusions keep working. Do not add or remove any exclusion beyond the three named classes.
- Attention-board answers are excluded because the metric's criterion is *typed* prompts. Do not add a separate "operator input" metric, a config knob, a CLI flag or a new frontmatter field.
- Do not add fields to `operatorPromptEntry` and do not start decoding `origin`. The measured `origin.kind` is `human` for both over-counted modern classes, so it cannot discriminate.
- `pkg/ops/` stays output-free: no `fmt.Print*`, no `os.Stdout`.
- The sabotage revert in requirement 3 is temporary and the restore is mandatory. `pkg/ops/interaction_count.go` must be back in its fixed form before the prompt ends; `<verification>` greps for it and runs `make precommit` last.
- Tests follow repository convention: Ginkgo v2 / Gomega, external `_test` package. New exported functions and interface methods carry doc comments.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…` and the `/tmp` scratch copy used by the falsification step; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
- Existing tests must still pass, including every other spec in `pkg/ops/interaction_count_test.go`, `pkg/ops/complete_test.go`, `pkg/ops/rollup_weekly_test.go`, `pkg/cli/rollup_test.go` and `integration/`.
</constraints>

<verification>
Run each of these and confirm the printed result against its expectation. The daemon does not gate on exit codes, so read the output.

**PRIMARY GATE — the new prefix list exists and is wired into the predicate, and the file is restored after the falsification revert.**

```
grep -n 'operatorPromptAlwaysNoisePrefixes' pkg/ops/interaction_count.go
grep -n 'hasOperatorPromptNoisePrefix(text, operatorPromptAlwaysNoisePrefixes)' pkg/ops/interaction_count.go
grep -n 'hasOperatorPromptNoisePrefix(text, operatorPromptNoisePrefixes)' pkg/ops/interaction_count.go
grep -n 'Operator answer, via attention board:' pkg/ops/interaction_count.go
grep -n '<bash-input>' pkg/ops/interaction_count.go
grep -n 'u276f' pkg/ops/interaction_count.go
grep -n 'return c.sum(ctx, sessionIDs, countOperatorPromptsInSessionLog)' pkg/ops/interaction_count.go
grep -c 'return c.Count(ctx, sessionIDs)' pkg/ops/interaction_count.go
```

Expect: the declaration and both call sites; all three prefixes; the delegation restored; and the absence check printing `0` (no sabotage left behind).

**TESTS — focused first, then the full suite.**

```
go test -v ./pkg/ops/... -count=1 -ginkgo.focus='operator prompts' -ginkgo.v
go test -v ./pkg/ops/... -count=1 -ginkgo.focus='legacy-only' -ginkgo.v
go test ./pkg/ops/... -count=1
make test
make precommit
```

The first command must exit 0 and its output must name the spec `Counts only operator prompts while the interaction count counts every user turn` — a focus string that matches nothing also exits 0 with no spec lines, so read the output for the spec name. The second must name the `Counts a modern row carrying a legacy-only noise prefix` spec. Every other command must exit 0.

**DOCS — both predicate descriptions name the new exclusions.**

```
grep -n 'attention.board' commands/complete-task.md README.md
grep -n 'bash-input\|shell input' commands/complete-task.md README.md
grep -n 'prompt echo\|rendered' commands/complete-task.md README.md
```

Expect a hit for each of the three classes in both files.

**CHANGELOG — structure and placement.**

```
bash scripts/check-changelog.sh
grep -n '^## ' CHANGELOG.md | head -2
awk '/^## /{sec=$0} /^- fix:/ && /operator/ {print sec}' CHANGELOG.md | head -1
```

Expect `CHANGELOG structure OK`; then `## Unreleased` followed by the newest `## vX.Y.Z`; then `## Unreleased` from the `awk`. If the `awk` prints nothing, the bullet is missing or does not name the predicate.

**SYNTAX.**

```
gofmt -e -l pkg/ops/*.go
```

Must list no files.

**SELF-CHECK before finishing:** re-run the PRIMARY GATE, the TESTS and the CHANGELOG checks. Confirm the falsification spec failed against `return c.Count(ctx, sessionIDs)` with a 27-vs-7 mismatch and passed after `cp /tmp/interaction_count.go.fixed pkg/ops/interaction_count.go`; confirm `pkg/ops/interaction_count.go` is in its fixed form — the sabotaged body must not be left in the tree.
</verification>
