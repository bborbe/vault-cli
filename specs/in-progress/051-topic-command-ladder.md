---
status: prompted
approved: "2026-09-20T20:15:32Z"
generating: "2026-09-20T20:26:42Z"
prompted: "2026-09-20T20:50:27Z"
branch: dark-factory/topic-command-ladder
---

## Summary

- Topics — the vault's cross-cutting domain pages under `23 Topics/` — get a first-class command family in vault-cli, at exact parity with the goal command family.
- The binary deliverable is the twelve goal leaves and nothing else: `add`, `clear`, `complete`, `defer`, `get`, `lint`, `list`, `remove`, `search`, `set`, `show`, `work-on`.
- The eleven goal *slash commands* are a different ladder and are out of scope for the binary; the topic slash-command ladder is vault-local, not a plugin deliverable, and is out of scope here.
- A topic's `phase` frontmatter value becomes observable through `topic show --output json` at the same key path and value shape the goal command uses.
- This spec does not write `phase` onto any topic page — it only surfaces a `phase` that is already on disk.

## Problem

Topic pages exist in the vault and carry real frontmatter, but no vault-cli command can address them. Every other entity level — task, goal, theme, objective, vision — has a command family, so an agent that wants to list, inspect, or mutate a topic has to fall back to reading and writing markdown by hand, which is exactly the failure mode the command families were built to remove. The gap is worse than a missing convenience: because a topic page can carry a `phase` field, an agent that reads topics through generic file access sees a lifecycle value that no supported command can report, so the field's presence is invisible to the tooling that is supposed to enforce it. The vault directory for topics and its configuration override already exist, so the missing piece is the command surface itself.

## Goal

After this work, a vault-cli user can address topic pages through a `topic` command family whose binary surface is identical in shape to the `goal` command family — the same twelve leaves, each doing for topics what its goal counterpart does for goals. Listing a topics directory honors the configured topics directory and its default. Showing a single topic emits its full detail, and in JSON output the phase value sits at the same key path and in the same value shape as the goal command's. The goal command family, the task command family, and every existing test behave exactly as they do today. The release notes record the addition.

## Non-goals

- Do NOT add any binary leaf beyond the twelve listed. The goal binary surface is exactly twelve leaves and the topic surface mirrors it one for one.
- Do NOT turn the eleven goal *slash commands* into binary subcommands. `plan-goal`, `execute-goal`, `verify-goal`, `audit-goal`, `create-goal`, `update-goal`, `goal-status`, `launch-goal`, `work-on-goal`, `complete-goal`, and `defer-goal` are plugin-level prompt files, not `vault-cli goal` subcommands; none of those names appears in `vault-cli topic --help`.
- Do NOT write, backfill, default, or normalize a `phase` field onto any topic page. Writing phase onto topic pages belongs to the sibling task `Add a Topic Phase Field With a Gated Planning → Execution Transition`; this spec only requires that `show` surfaces a `phase` already present on disk. No page under `23 Topics/` carries a `phase:` line today, so the present-phase behavior is exercised against a scratch page, never against the live vault.
- Do NOT invent a default phase (for example falling back to `todo`) when a topic page has no `phase` line — a topic with no phase key surfaces no phase value, mirroring the goal and task behavior.
- Do NOT modify, extend, or reuse the goal phase type, the task phase type, or their normalizers for the topic entity. The topic entity gets its own wrapper.
- Do NOT change the goal command family's output, flags, exit codes, or the goal-side frontmatter allowlists.
- Do NOT extend the hardcoded entity-kind lists elsewhere in the binary. The watch command's accepted type list, its watch-directory set, and the resolve/type set each name task, goal, theme, and objective by hand; "mirror the goal family" is not license to add a topic kind to any of them. Adding topic to the watch surface is a separate spec.
- Do NOT add a `docs/topic-writing.md`. The repo carries one `<entity>-writing.md` per other entity; a topic writing guide is out of scope for this deliverable and is a separate doc-only change.
- Do NOT ship topic slash-command files. The topic slash-command ladder is vault-local (`Personal/.claude/commands/`), not a plugin deliverable, for three checkable reasons. (1) The vault-local file already ships: `Personal/.claude/commands/verify-topic.md` exists (10 checks, written 2026-09-20). (2) `topic-status` is a retired name: the task `Rename topic-manager to worker-manager and topic-status to worker-status` is `completed`, and only `worker-status.md` exists in the supervisor plugin, so mandating a `commands/topic-status.md` would resurrect a retired file. (3) The plugin route was already declined: the `completed` task `Every Hierarchy Level Has a Command Ladder Except Topics` records it as Out of Scope under the vault's `Minimal Tooling First` rule — the plugin route is the fallback only if the operator wants the verifier shared across vaults.
- Do NOT restore the slash-command deliverable from this spec's history. Provenance: the owning goal page's Impact line once stated *"vault-cli ships eleven goal commands"* as a measurement when it was recalled, not read. `vault-cli goal --help` lists **twelve** binary leaves, and eight of the eleven names in that recalled list (`audit`, `create`, `execute`, `goal-status`, `launch`, `plan`, `update`, `verify`) are `/vault-cli:*` **slash** commands, not binary subcommands. The count error and the scope error arrived together; the scope error is the one that would have shipped. The goal page carries the same correction, with the full provenance chain recorded on it.
- Do NOT migrate, rewrite, or reformat existing topic pages.

## Acceptance Criteria

- [ ] `vault-cli topic --help` lists exactly the twelve leaves `add`, `clear`, `complete`, `defer`, `get`, `lint`, `list`, `remove`, `search`, `set`, `show`, `work-on`, and nothing else — evidence: stdout match plus set comparison, the topic leaf block and the goal leaf block each contain twelve lines AND `diff <(goal leaves) <(topic leaves)` is empty, so the two sets are equal member for member (equal counts alone are not sufficient).
- [ ] The goal ladder is unchanged and still reports exactly twelve leaves — evidence: `vault-cli goal --help` leaf block contains exactly twelve lines naming the twelve leaves above.
- [ ] `vault-cli topic --help` contains none of the eleven goal slash-command names (`plan-goal`, `execute-goal`, `verify-goal`, `audit-goal`, `create-goal`, `update-goal`, `goal-status`, `launch-goal`, `work-on-goal`, `complete-goal`, `defer-goal`) — evidence: negative evidence, a case-insensitive match of each of the eleven names against the full help output returns zero lines.
- [ ] `vault-cli topic show "<page-with-phase>" --vault <vault> --output json` emits the phase value at the JSON key path `.fields.phase` — evidence: stdout match, the JSON object has a `fields` member whose `phase` key is present and non-empty, and the value equals the same on-disk phase string that `vault-cli goal show "<goal-with-phase>" --vault <vault> --output json` reports at `.fields.phase`.
- [ ] A topic page with no `phase` line emits NO `phase` key in the JSON `fields` object, and reading that field directly prints an empty line and exits 0 — evidence: negative evidence, `vault-cli topic show "<page-without-phase>" --vault <vault> --output json | jq -e '.fields | has("phase")'` exits non-zero (key absent, not empty), and `vault-cli topic get "<page-without-phase>" phase` prints an empty line with exit code 0. The positive counterpart is required in the same pass: `vault-cli topic get "<page-with-phase>" phase` prints the page's on-disk phase value and exits 0, so the clause cannot pass vacuously on a page that does carry a phase. The argument order is name then key, mirroring the goal command's `get <name> <key>` contract. No default value is substituted. (The goal mirror is the reference: `vault-cli goal show "<phase-less-goal>" --output json` likewise emits no `phase` member.)
- [ ] `topic set` / `topic get` / `topic clear` / `topic add` / `topic remove` round-trip real frontmatter on a topic page — evidence: state transition, after `topic set <page> <key> <value>` the page's frontmatter on disk contains that key with that value, `topic get <page> <key>` prints exactly that value, and after `topic clear <page> <key>` the key is gone from the file; `topic add` appends one entry to a list field and `topic remove` drops it, with the file showing one fewer entry than after the add.
- [ ] `topic complete` and `topic defer` produce observable frontmatter state transitions — evidence: state transition with before/after framing, `topic complete <page>` moves the page's status from its prior value to the completed value and `topic complete <already-completed-page>` is refused or is a no-op with the file unchanged; `topic defer <page> +7d` writes a `defer_date` frontmatter value exactly seven days after the run date, `topic defer <page> 2027-03-19` writes that date, and `topic defer <page> 2000-01-01` is refused with a non-zero exit and leaves the file byte-identical.
- [ ] `topic lint` reports a seeded defect on a topic page and exits non-zero for it, while a clean page passes — evidence: stdout match plus exit code, a topic page seeded with a duplicate frontmatter key produces at least one lint issue naming that file, and an unseeded page produces zero issues for that file.
- [ ] `topic search "<query>"` returns a topic page that matches a seeded query — evidence: stdout match, a topic page seeded with a distinctive marker appears in the result set for a query naming that marker.
- [ ] `topic work-on <page>` moves the page into its in-progress state — evidence: state transition, the page's status frontmatter reads the in-progress value after the command, and the command reports a session outcome rather than a usage error.
- [ ] `vault-cli topic list --vault <vault>` returns exactly the topic pages on disk in the configured topics directory, and a topic written through the command family lands under that directory — evidence: stdout match plus file artifact, the listed count equals the file count on disk in that directory, and when the vault config names a non-default topics directory the written file appears under the configured directory and not under the default. The configured path is used verbatim — a configured-but-absent directory yields an empty result and exit 0, never a silent fallback to the default.
- [ ] A topic name that attempts to escape the topics directory reads and writes nothing outside it — evidence: negative evidence, `vault-cli topic show "../<outside-file>"` and `vault-cli topic set "../<outside-file>" k v` each exit non-zero, and `git status` plus a byte-compare show the outside file unmodified. This AC proves non-resolution — the name fails to resolve to a page inside the topics directory, which is what makes the command exit non-zero — and does not assert an explicit traversal guard, matching the Security section's position that this work introduces no new trust boundary.
- [ ] The four goal-side files — `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go`, `pkg/ops/goal_workon.go` — are unchanged, and the goal command's observable output is unchanged — evidence: negative evidence, `git diff <baseline>..HEAD -- pkg/domain/goal.go pkg/domain/goal_frontmatter.go pkg/domain/goal_phase.go pkg/ops/goal_workon.go` is empty, and `vault-cli goal show "<goal>" --output json` output is identical before and after. This AC claims only those four files; the shared CLI and ops files legitimately gain a topic case and are covered by the behavioral clause.
- [ ] `CHANGELOG.md` carries a topic-ladder bullet under an `## Unreleased` heading — evidence: file content grep, the heading is present and the bullet sits under it.
- [ ] `make precommit` exits 0 at the repo root — evidence: exit code.

**Scenario coverage — decision: NO new scenario.** Justification against the four-condition test: (a) unit tests in the domain package reach the frontmatter wrapper and phase reading directly, and the repository's existing CLI integration harness builds the binary and drives real command invocations against a temporary vault on disk, so the dispatch path, the JSON serialization, the directory resolution, and the traversal refusal are all reachable without a sandbox or external service; (b) the behavior is a command surface, not a multi-service journey; (c) no existing scenario covers a new entity command family; (d) there is no named regression risk that only a scenario could catch — the topic family touches no network, no cluster, and no shared external state, only the filesystem the integration harness already exercises. The repo's `scenarios/*.md` walk remains a release-gate obligation and is listed on the operator rung below; it is not a new scenario file written by this spec.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```bash
make precommit                                                                                      # format, generate, test, check, addlicense — must exit 0
make test                                                                                           # unit + integration suites — must pass
make build                                                                                          # builds to bin/vault-cli (Makefile target: `go build -o bin/vault-cli main.go`)
./bin/vault-cli goal --help | sed -n '/Available Commands:/,/^$/p' | grep -c '^  [a-z]'              # expect 12
./bin/vault-cli topic --help | sed -n '/Available Commands:/,/^$/p' | grep -c '^  [a-z]'            # expect 12
./bin/vault-cli topic --help | grep -ciE 'plan-goal|execute-goal|verify-goal|audit-goal|create-goal|update-goal|goal-status|launch-goal|work-on-goal|complete-goal|defer-goal'   # expect 0
grep -c '^## Unreleased' CHANGELOG.md                                                               # expect >= 1
diff <(./bin/vault-cli goal --help | sed -n '/Available Commands:/,/^$/p' | grep '^  [a-z]' | awk '{print $1}' | sort) \
     <(./bin/vault-cli topic --help | sed -n '/Available Commands:/,/^$/p' | grep '^  [a-z]' | awk '{print $1}' | sort)   # expect empty (sets equal)
./bin/vault-cli goal set <goal> priority 3 && ./bin/vault-cli goal get <goal> priority                # expect the literal "3" (goal scalar coercion unchanged)
```

### Operator-executable (runs on the host after PR merge, spec verification ladder)

```bash
# 1. Release gate — mandatory before make install (docs/releasing-vault-cli.md)
#    Unit tests + make precommit alone are NOT sufficient.
make build && cp bin/vault-cli /tmp/new-vault-cli     # freshly built binary, never the installed one
git diff "$(vault-cli --version | awk '{print $NF}')"..HEAD --name-only | grep -E '\.(go|mod|sum)$|^Makefile$'   # non-empty → scenarios MUST run
#    then walk every scenarios/*.md against /tmp/new-vault-cli per repo CLAUDE.md
make release-check                                    # precommit + check-versions

# 2. Install and verify
make install
vault-cli topic --help                                # twelve leaves, matching `vault-cli goal --help` leaf for leaf
vault-cli topic list --vault Personal                 # returns the eight pages under "23 Topics/"
vault-cli topic show "Attention Routing" --vault Personal --output json   # no .fields.phase member today — the field is not yet written on topic pages
vault-cli topic show "Attention Routing" --vault Personal                 # plain output renders without error
vault-cli goal --help                                 # still twelve leaves, unchanged
```

No `phase` value is expected on any current topic page — this spec does not write one. The operator check that a phase *value* round-trips uses a scratch page the operator adds a `phase:` line to under the topics directory, and must be removed afterwards; the ACs that assert a non-empty `.fields.phase` are exercised by the integration harness against a temporary vault, not against the real vault.

## Desired Behavior

1. A topic is a first-class vault entity: a topic page reads into an entity that carries its frontmatter, its filesystem metadata, and its markdown content, and every frontmatter key it does not recognise survives a read-write cycle untouched.
2. A topic page's `phase` value is observable. When the page carries a `phase` line, showing the topic reports that value in the same place and the same shape the goal command reports a goal's phase; when the page carries no `phase` line, no phase value is reported at all — the field is simply absent, and reading it directly yields an empty result.
3. A topic page can be read and written by name inside a vault's configured topics directory, and the configured directory is used verbatim with no fallback: when the configuration names a directory that does not exist, the result is empty and the command succeeds; when the configuration is silent, the topics default applies.
4. A `topic` command family is registered at the root of the binary with exactly the twelve leaves named in the Acceptance Criteria, each mirroring its goal counterpart's purpose, argument shape, and flags, and each performing the real operation its name promises rather than a placeholder.
5. Showing a single topic publishes its full detail — name, file path, vault, ordered fields, and content — with the phase value placed exactly as the goal command places it.
6. Marking a topic to be worked on moves the topic into its in-progress state and starts or resumes a Claude session, mirroring the goal command's behavior.
7. The release notes gain a topic bullet under the unreleased heading.

## Constraints

- The goal binary surface stays at exactly twelve leaves with unchanged names, flags, argument counts, output, and exit codes.
- The goal-side frontmatter allowlists and the goal work-on operation are not modified. Four goal-side files carry an empty diff against the baseline: `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go`, `pkg/ops/goal_workon.go`. The shared CLI and ops files legitimately gain a topic case.
- Existing task, goal, theme, objective, and vision commands and their tests pass unchanged.
- The topics directory default is the vault's existing topics default; a configuration override wins over the default and is used verbatim.
- Topic pages with no `phase` line parse and show without error, and no file is backfilled.
- Unknown frontmatter keys on a topic page survive a read-write cycle.
- The twelve leaf names are fixed; no alias, no additional leaf, and no goal-slash-command name appears in the topic binary surface.
- The layered recipe in `docs/development-patterns.md` § "Adding a New Command" (Domain → Storage → Ops → CLI) governs the shape of this work; the topic entity follows the same three-concern split (frontmatter wrapper, filesystem metadata, content) as the existing entities.
- `Personal/50 Knowledge Base/Topic Writing Guide.md` is the documented source for the topic conventions the entity must preserve; read it before writing the entity so the wrapper round-trips what the guide declares.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection |
|---------|-------------------|----------|-----------|
| Topics directory absent from the vault | `vault-cli topic list` prints 0 rows and exits 0 (evidence: exit code 0, row count 0); a write creates the directory path it needs | Operator populates the directory, then `vault-cli topic list` prints N rows where N equals the file count in that directory | Exit code 0 in both states; row count 0 before population, N after |
| Topic page named by `show` does not exist | Command exits non-zero with an error naming the topic and the directory searched; no file is written | Operator corrects the name and re-runs; evidence: the command exits 0 and `.name` in JSON output equals the corrected name | Non-zero exit; stderr names the topic; `git status` shows no new file |
| Configuration names a topics directory that does not exist | The configured path is used verbatim with no fallback: list returns an empty result set and exits 0; a write creates the configured path | Operator fixes the configuration or creates the directory; evidence: after the write, `ls <configured-dir>` lists the file and the default directory is still absent | Exit code 0 with empty stdout; the default topics directory is not created |
| Topic page carries a `phase` value that is not one of the canonical lifecycle values | `show` surfaces the raw on-disk string without rejecting the page; the page stays readable | Operator corrects the value on disk; evidence: `vault-cli topic show <page> --output json \| jq -r '.fields.phase'` prints the corrected value | The raw value appears at `.fields.phase`; no error is emitted |
| Topic page carries a duplicate frontmatter key | The page reads; `topic lint` reports the duplicate as an issue naming the file, and plain `topic lint` exits non-zero | Operator removes the duplicate; evidence: re-running `topic lint` reports zero issues for that file | Lint issue count for the file is ≥ 1 before, 0 after |
| `defer` is given a relative date or a date in a non-UTC timezone | The relative form resolves against the run date in the operator's local timezone and the absolute form is stored as the date written, with no off-by-one from timezone conversion | Operator re-runs with the intended absolute date; evidence: `vault-cli topic get defer_date <page>` prints the date the operator intended, not one day earlier or later | The stored `defer_date` differs from the intended date by one day — visible by reading the field back |
| `defer` is given a date in the past | Command exits non-zero naming the date and the past-date refusal; the page is byte-identical afterwards | Operator supplies a future date; evidence: the command exits 0 and the page's `defer_date` equals the supplied date | Non-zero exit; `git diff` of the page is empty |
| Two writers mutate the same topic page at once | Last write wins on the file; no partial or interleaved frontmatter is written | Operator compares the file against the intended value and re-applies the losing change; evidence: `vault-cli topic get <page> <key>` prints the re-applied value and `topic lint` reports zero issues | The file parses cleanly and `topic lint` reports zero issues for it |
| Crash mid-write of a topic page | The page is left in its prior or fully written state, never truncated to a partial frontmatter block | Operator re-runs the mutation; evidence: `vault-cli topic show <page>` exits 0 and the page's frontmatter parses | The page reads successfully after the re-run; a truncated block would fail to parse |

## Security / Abuse Cases

- The topic name is user-supplied and is used to resolve a file path. A name containing path separators or parent-directory segments must not escape the topics directory; resolution rejects or neutralizes traversal so a `show`, `set`, or `remove` cannot read or write outside the vault's topics directory. This is asserted by a negative-evidence Acceptance Criterion.
- Topic names come from the same trust boundary as existing entity names and follow the same resolution rules as the goal and task families; this work introduces no new trust boundary.
- Topic file content is parsed as YAML frontmatter and markdown. Malformed YAML must fail with an error naming the file, never a panic, and must leave the file untouched.
- No network, no subprocess, and no credential access is introduced. The `work-on` path reuses the existing session starter and locker, so it inherits that path's existing gating rather than adding a new one.

## Suggested Decomposition

Prompts are generated in this order — each row is a single prompt with a clear scope. The four code layers are the four prompts; the last row is the packaging and verification pass. Every remaining AC appears in exactly one row.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Topic domain entity + its own frontmatter wrapper, generic per-field view | 1, 2 | 4, 5, 13 | — |
| 2 | Topic storage type + topics-directory plumbing through both configuration sources | 3 | 11, 12 | prompt 1 (uses the entity) |
| 3 | CLI: `topic` command group with the twelve leaves, registered at root, over the ops layer | 4, 5, 6 | 1, 2, 3, 6, 7, 8, 9, 10 | prompt 2 (uses the storage) |
| 4 | CHANGELOG `## Unreleased` bullet + full precommit pass | 7 | 14, 15 | prompt 3 (packages the shipped surface) |

Rationale: prompt 1 is the foundation every later layer compiles against; prompt 2 needs the entity type; prompt 3 needs the storage interface and also builds the ops layer the command group calls; prompt 4 packages the surface prompt 3 shipped, so it can only be written once the leaf set is final. Prompts 1 and 2 have no cycle risk between them, but 2→3→4 is a strict chain and must run in filename order. AC mapping note: the ops layer has no standalone AC because an ops-layer operation has no user-observable surface until the command group calls it; its correctness is asserted by the ACs prompt 3 owns (6–10). The domain wrapper's own ACs (4, 5) assert the generic per-field read only — the typed phase accessor and its validation belong to the sibling task `Add a Topic Phase Field With a Gated Planning → Execution Transition`, and prompt 1 must not build them.

## Do-Nothing Option

Doing nothing leaves the topic entity unreachable by every vault-cli command. Agents that need topic data keep hand-editing markdown, the `phase` field stays invisible to the tooling, and the vault grows a level of pages that no command, no lint, and no list can address — while the directory and its configuration override sit unused. The cost of the gap is paid on every agent interaction that touches a topic, which is the reason the other five entity levels all have command families already.
