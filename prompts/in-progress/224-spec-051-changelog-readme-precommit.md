---
status: approved
spec: [051-topic-command-ladder]
created: "2026-09-20T20:29:34Z"
queued: "2026-09-20T21:28:57Z"
branch: dark-factory/topic-command-ladder
---

# Release notes and the full gate: the `## Unreleased` bullet, the README's `topic` section, and the complete acceptance walk

<summary>
- The changelog records the new topic command family under an unreleased heading.
- The README's usage section documents the `topic` command family with all twelve of its leaves.
- No version string is bumped anywhere and no tag is created — the post-merge releaser owns that.
- The full precommit gate passes at the repo root: formatting, code generation, the whole test suite, lint, vet, vulnerability scanning and the changelog structure check.
- The shipped binary is rebuilt and driven directly to confirm the topic command family and the goal command family expose the same twelve leaves, member for member.
- None of the eleven goal slash-command names appears anywhere in the topic help output.
- Every one of the spec's fifteen acceptance criteria is walked and reported against the change.
</summary>

<objective>
Package the shipped surface: record the topic command family in the changelog under an unreleased heading, document it in the README's usage section, and run the full gate plus the spec's container-executable verification rung so the deliverable is provably complete. This is spec 051's prompt 4 of 4: it covers Desired Behavior 7 and Acceptance Criteria 14 and 15, and it depends on prompts 1, 2 and 3 having shipped the entity, the storage and the command family.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `CHANGELOG.md` — read the first ~30 lines. The structure is `# Changelog` → the `All notable changes to this project will be documented in this file.` line → the `Please choose versions by [Semantic Versioning]…` line → the three `* MAJOR / MINOR / PATCH` lines → the newest `## vX.Y.Z` section. **There is no `## Unreleased` section today** — the top section is a released version. You create `## Unreleased` between the preamble block and the newest versioned section.
- `scripts/check-changelog.sh` — the structural check `make precommit` runs. It fails if any `## ` section appears *above* the preamble line. Read it before placing the section.
- `scripts/check-versions.sh` — the alignment check (not part of `make precommit`; run it explicitly). It reads the top `## vX.Y.Z` heading and the three `.claude-plugin/` JSON fields and reports `✅ all four versions equal: <version>`. It uses `jq`, which the container has.
- `docs/dod.md` — this repo's `validationPrompt`. Its Documentation section carries the placement rule: "CHANGELOG.md has an entry under `## Unreleased`. If that section does not exist yet, create it **below** the preamble block … and **above** the newest `## vX.Y.Z` section — never between the `# Changelog` title and the preamble. The final order is always: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z` (newest first)." It also says "README.md is updated if the change affects usage, configuration, or setup" — a new command family does.
- `README.md` — read `## Usage` and its per-entity subsections in order: `### task`, `### goal`, `### theme`, `### objective`, `### vision`, `### decision`, `### search`, `### config`. Your `### topic` subsection goes between `### vision` and `### decision`, keeping the entity subsections contiguous.
- `.maintainer.yaml` — `release.autoRelease: true`. The `github-releaser` owns version bumps and tags: it converts `## Unreleased` into a versioned section, bumps all four version strings and tags **post-merge**. A hand-bump or a hand-tag here races it.
- `specs/in-progress/051-topic-command-ladder.md` — the spec. Read its `# Verification` § "Container-executable" rung (the commands this prompt reproduces) and § "Operator-executable" (which you do **not** run), its fifteen Acceptance Criteria, its Failure Modes table, and its `## Scenario coverage` decision.
- `prompts/1-spec-051-topic-domain-entity.md`, `prompts/2-spec-051-topic-storage-topics-dir.md`, `prompts/3-spec-051-topic-ops-and-command-group.md` — the three prompts whose work you are packaging. Read their `<verification>` blocks so you know which checks are already asserted and which the full gate re-runs.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format (`- <prefix>: <what> [context]`), the required prefix, the `## Unreleased` placement rule, and the anti-patterns.
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md` — README conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/git-workflow.md` — why dark-factory owns the commit and the release.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

**Environment facts that shape this prompt:**

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. The spec's operator rung (`make build && cp bin/vault-cli /tmp/new-vault-cli`, `git diff … --name-only`, walking `scenarios/*.md`, `make release-check`, `make install`) is host-only and is not run here.
2. **`make build` is not used here.** It writes to `bin/` (a build artifact) and the container-executable substitute is a direct toolchain build to a temp path: `go build -mod=mod -o /tmp/vault-cli-topic .`. That gives the same binary the spec's rung wants to drive, without touching the repo tree. Say so in your completion report.
3. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it — `go build` otherwise tries to read a masked `.git`.
4. **The version at HEAD may not be `0.141.1`.** Prompts 1-3 may have merged and the releaser may have converted `## Unreleased` into a versioned section in the meantime. Do NOT assert a literal version anywhere; assert only that the four strings are equal to each other, and create `## Unreleased` if it is absent.
</context>

<requirements>

## 0. Scope — two files

- `CHANGELOG.md` — one new `## Unreleased` section with one bullet.
- `README.md` — one new `### topic` subsection.

Nothing else. No Go file changes; no test changes; no `scenarios/*.md` change; no `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` change; no `docs/**` change; no `go.mod`/`go.sum` change. If a check in `<verification>` fails, the fix belongs in a *file* — do not paper over it by editing a script, a Makefile target, a linter config, or the acceptance evidence. If a failure is genuinely in prompts 1-3's output, fix that file and say so in the completion report.

## 1. `CHANGELOG.md` — one bullet under a new `## Unreleased`

Insert this block between the preamble block and the newest `## vX.Y.Z` section — that is, immediately after the `* PATCH version when you make backwards-compatible bug fixes.` line and immediately above the `## v…` heading that currently follows it:

```
## Unreleased

- feat: add the `topic` command family, at exact parity with the `goal` family: the same twelve leaves (`add`, `clear`, `complete`, `defer`, `get`, `lint`, `list`, `remove`, `search`, `set`, `show`, `work-on`), each addressing a topic page in the vault's configured topics directory. A topic page's `phase` value is observable through `topic show --output json` at `.fields.phase` — the same key path and value shape `goal show` uses — and a page with no `phase` line reports no `phase` key at all; this change writes no phase onto any page. Listing honours the configured `topics_dir` verbatim with no fallback to `23 Topics`, and a name that would resolve outside that directory is refused. The goal, task, theme, objective and vision command families are unchanged.
```

- The bullet must start with `- feat:` on a single line and must name `topic`. The remaining wording is yours to adjust, but keep the prefixes-and-scope shape: name what was added, not what you verified.
- **Do NOT bump any version.** Not `CHANGELOG.md`'s newest version heading, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. `.maintainer.yaml` sets `release.autoRelease: true` and the post-merge releaser converts `## Unreleased` into a versioned section and tags it — a hand-bump races it.
- Do NOT create a second `## Unreleased` section, do NOT add a second bullet for the same change, and do NOT reorder or reword any existing released section.
- Do NOT copy a `<verification>` comment into the changelog, do NOT use the prompt filename as the entry, and do NOT describe what you verified. Describe what was implemented.

## 2. `README.md` — the `### topic` subsection

Insert this block between the `### vision` subsection and the `### decision` subsection:

```markdown
### topic

```bash
vault-cli topic list                                  # List topics
vault-cli topic lint                                  # Detect frontmatter issues
vault-cli topic search "attention routing"            # Semantic search in topics
vault-cli topic show "Attention Routing"              # Show full topic detail
vault-cli topic get "Attention Routing" phase         # Get a frontmatter field
vault-cli topic set "Attention Routing" owner alice   # Set a frontmatter field
vault-cli topic clear "Attention Routing" owner       # Clear a frontmatter field
vault-cli topic add "Attention Routing" tags focus    # Add a value to a list field
vault-cli topic remove "Attention Routing" tags focus # Remove a value from a list field
vault-cli topic complete "Attention Routing"          # Mark a topic as complete
vault-cli topic defer "Attention Routing" +7d         # Defer a topic to a specific date
vault-cli topic work-on "Attention Routing"           # Mark in_progress and start a Claude session
```
```

- Twelve `vault-cli topic …` lines, one per leaf, in the same commented style as the sibling subsections. The leaf names must match the binary's exactly.
- Do NOT reorder or reword any existing subsection, do NOT add a `### topic` block anywhere else in the file, and do NOT document the topics-directory config key here — `### Configuration` already documents `topics_dir` (added under spec 048) and this change adds no config key.
- Do NOT document the eleven goal slash-command names. The spec's Non-goals place the topic slash-command ladder out of scope, and the README's `## Claude Code Plugin` section is not the place to imply one exists.

## 3. The full gate

Run `make precommit` at the repo root and confirm it exits 0. It runs `ensure`, `format`, `generate`, the whole test suite (unit + integration, including the twelve topic command-registration entries and every behavioural topic spec), `check` (`lint`, `vet`, `vulncheck`, `osv-scanner`, `trivy`, `check-changelog`) and `addlicense`.

If it fails: fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until that target passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success, even if the failure looks pre-existing or unrelated.

Note that `make generate` does `rm -rf mocks` then `go generate ./...`. It must regenerate `mocks/topic-storage.go`, `mocks/topic-complete-operation.go`, `mocks/topic-defer-operation.go` and `mocks/topic-workon-operation.go` without error. If `make generate` produces a diff in any *other* mock, that is a signal a counterfeiter directive moved — report it rather than committing the diff.

## 4. The spec's container-executable rung, in a container-safe form

The spec's `# Verification` § "Container-executable" block lists nine commands. Reproduce all of them here, with two substitutions and one omission:

- `make build` → `go build -mod=mod -o /tmp/vault-cli-topic .` (the container rule against `make build`, and `bin/` is a build artifact).
- `./bin/vault-cli` → `/tmp/vault-cli-topic`.
- The `./bin/vault-cli goal set <goal> priority 3 && ./bin/vault-cli goal get <goal> priority` line is **omitted**: it needs a real goal in a real vault, and the goal family's scalar coercion is already pinned by the existing goal suite that `make test` runs. State that omission in your completion report.

The remaining eight checks, which `<verification>` carries:

1. `make precommit` exits 0.
2. `make test` passes.
3. The goal help leaf block contains exactly twelve lines.
4. The topic help leaf block contains exactly twelve lines.
5. The topic help output contains zero occurrences of the eleven goal slash-command names.
6. The goal leaf set and the topic leaf set are equal member for member.
7. `go build -mod=mod -o /tmp/vault-cli-topic .` exits 0 and the binary is executable — the substituted `make build`.
8. `grep -c '^## Unreleased' CHANGELOG.md` is at least 1, and the topic bullet sits under that heading — the two facts the spec states as its Acceptance Criterion 14 evidence.

## 5. The full acceptance walk

Walk all fifteen of spec 051's Acceptance Criteria against the shipped change and report each one in your completion report, naming the file and the check that satisfies it. The mapping across the four prompts is:

| AC | what it asserts | where it is satisfied |
|---|---|---|
| 1 | `topic --help` lists exactly the twelve leaves, set-equal to the goal leaves | prompt 3 § 6c; re-run in § 4 |
| 2 | the goal ladder is unchanged and still reports twelve leaves | prompt 3 § 6b/§ 6c; re-run in § 4 |
| 3 | `topic --help` contains none of the eleven goal slash-command names | prompt 3 § 6c; re-run in § 4 |
| 4 | `topic show … --output json` emits the phase at `.fields.phase`, equal to `goal show`'s | prompt 1 § 5a; prompt 3 § 1d/§ 6c |
| 5 | a page with no `phase` line emits no `phase` key, and `topic get … phase` prints an empty line and exits 0 | prompt 1 § 5a/§ 5b; prompt 3 § 6c |
| 6 | `topic set`/`get`/`clear`/`add`/`remove` round-trip real frontmatter | prompt 3 § 1b/§ 1c/§ 1e and § 6c |
| 7 | `topic complete` and `topic defer` produce observable state transitions, including both refusals | prompt 3 § 2/§ 3 and § 6c |
| 8 | `topic lint` reports a seeded defect and exits non-zero, and passes a clean page | prompt 3 § 6c |
| 9 | `topic search "<query>"` returns a matching page | prompt 3 § 6c (dispatch half); the semantic-result half is operator-side |
| 10 | `topic work-on` moves the page into its in-progress state | prompt 3 § 4 and § 6c |
| 11 | `topic list` returns exactly the pages in the configured directory, used verbatim | prompt 2 § 3d/§ 3e; prompt 3 § 6c |
| 12 | a traversal name reads and writes nothing outside the topics directory | prompt 2 § 2b/§ 2c/§ 3b; prompt 3 § 6c |
| 13 | the four goal-side files are unchanged and the goal output is unchanged | prompt 1 § 6 and its non-git equivalents; the `git diff` half is operator-side |
| 14 | `CHANGELOG.md` carries a topic bullet under `## Unreleased` | this prompt § 1 |
| 15 | `make precommit` exits 0 at the repo root | this prompt § 3 |

For AC 13, state plainly that the `git diff <baseline>..HEAD -- pkg/domain/goal.go pkg/domain/goal_frontmatter.go pkg/domain/goal_phase.go pkg/ops/goal_workon.go` evidence is operator-side on the spec's verification ladder, that prompt 1's anchor greps are the container-executable equivalent, and that `make precommit`'s full suite passing on the unchanged goal tests is the behavioural corroboration.

For AC 9, state plainly that `semantic-search-mcp` is not guaranteed on `PATH` in this container, so only the dispatch half is container-executable and the "returns a topic page that matches a seeded query" half is operator-side.

## 6. What you must NOT do

- Do NOT run the spec's operator-executable rung: no `make install`, no `make release-check`, no walking `scenarios/*.md`, no `vault-cli topic list --vault Personal`, no `git diff`.
- Do NOT add, modify or delete a scenario file. The spec's `## Scenario coverage` section records the four-condition justification for shipping no new scenario; `ls scenarios/*.md | wc -l` must still print `5`.
- Do NOT add `docs/topic-writing.md`. The spec's Non-goals place it out of scope.
- Do NOT bump a version string, create a tag, or edit `.claude-plugin/**`.
- Do NOT "fix" a failing linter by adding a blanket `//nolint` or by editing `.golangci.yml`. If `dupl` fires on a new topic command builder, the annotation is `//nolint:dupl // Command groups are structurally similar but manage distinct entity types` on that one function — nothing wider.
- Do NOT weaken an existing test to make the gate pass. If an existing test fails, the cause is in prompts 1-3's output; fix that.

## 7. Self-check before finishing

- Re-read the two changed hunks and confirm: `## Unreleased` sits below the preamble and above the newest `## v` section; the bullet starts with `- feat:` and names `topic`; the README gained exactly one `### topic` subsection with twelve `vault-cli topic` lines; no version string moved; no other file changed.
- Walk all fifteen acceptance criteria per § 5 and report each one.
- Walk `docs/dod.md`: the changelog entry sits under `## Unreleased` in the documented position; the README documents the usage change; no debug output was added; no `pkg/ops/` file prints to stdout; no new dependency entered `go.mod`; the integration command-registration table carries the twelve topic entries.
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 051 — non-goals.** Do NOT add any binary leaf beyond the twelve. Do NOT turn the eleven goal *slash commands* into binary subcommands. Do NOT write, backfill, default or normalize a `phase` field onto any topic page. Do NOT invent a default phase when a topic page has no `phase` line. Do NOT modify, extend or reuse the goal phase type, the task phase type, or their normalizers for the topic entity. Do NOT change the goal command family's output, flags, exit codes, or the goal-side frontmatter allowlists. Do NOT extend the hardcoded entity-kind lists elsewhere in the binary — the watch command's accepted type list, its watch-directory set and the resolve/type set. Do NOT add a `docs/topic-writing.md`. Do NOT ship topic slash-command files. Do NOT migrate, rewrite or reformat existing topic pages.
- **Copied from spec 051 — constraints.** The goal binary surface stays at exactly twelve leaves with unchanged names, flags, argument counts, output and exit codes. `pkg/domain/goal.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/goal_phase.go` and `pkg/ops/goal_workon.go` carry an empty diff against the baseline; their `git diff` evidence is operator-side. Existing task, goal, theme, objective and vision commands and their tests pass unchanged. The twelve leaf names are fixed; no alias, no additional leaf, and no goal-slash-command name appears in the topic binary surface.
- **Depends on prompts 1, 2 and 3.** The entity, the storage and the command family must already be in the tree. If `vault-cli topic --help` cannot list twelve leaves, a prior prompt did not land — report `"status":"failed"` naming it rather than building any of the missing pieces here.
- **No version bump, no tag.** `.maintainer.yaml` sets `release.autoRelease: true`; the post-merge `github-releaser` converts `## Unreleased` into a versioned section, bumps all four version strings and tags. A hand-bump races it and `make check-versions` would then disagree with the releaser's next run.
- **Frozen strings.** The section heading `## Unreleased`; the bullet prefix `- feat:`; the subsection heading `### topic`; the twelve README lines each beginning `vault-cli topic `. All are grep targets in the acceptance criteria.
- **No new scenario file.** `ls scenarios/*.md | wc -l` must still print `5`. The spec's `## Scenario coverage` section records the justification.
- **No `docs/topic-writing.md`.** The repo carries one `<entity>-writing.md` per other entity; a topic writing guide is a separate doc-only change.
- **`make build` is not used.** The container-executable substitute is `go build -mod=mod -o /tmp/vault-cli-topic .`. Do NOT write to `bin/`.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- Do NOT run `docker`, `kubectl`, `gh`, or any `dark-factory` command. Do NOT run `make install`, `make release-check` or `make buca`.
- Do NOT run `go mod vendor`. No new dependency.
</constraints>

<verification>
Run everything from the repo root. Each check below is a self-failing assertion on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The full gate, first and once:**

```
make precommit
```

It must exit 0. If it fails, fix the cause, re-run ONLY the failing target until it passes, then run `make precommit` once more. Report the actual exit code in the completion report's `verification` block.

**The test suite and the changelog/version checks:**

```
make test
make check-changelog
make check-versions
```

`make check-versions` must print `✅ all four versions equal: <version>` — do NOT assert a literal version, because the post-merge releaser may have moved it since this prompt was written. A `❌ version mismatch` line means something bumped a version string that this change must leave alone.

**`CHANGELOG.md` — the section exists, is in the right place, and carries the bullet:**

```
test "$(grep -c '^## Unreleased' CHANGELOG.md)" -ge 1
test "$(awk '/^## /{sec=$0} /^- feat:.*[Tt]opic/{print sec}' CHANGELOG.md | grep -c '^## Unreleased')" -ge 1
```

The second line asserts both halves of the claim: the bullet exists **and** it sits under `## Unreleased`. A bare `grep -cE '^- feat:.*topic'` is vacuous — the unmodified tree already carries three such bullets under released headings (`## v0.134.0`, `## v0.133.0`, `## v0.71.0`), so an agent that edits nothing passes it, and it cannot distinguish a bullet under `## Unreleased` from one folded into the newest `## vX.Y.Z`. The `awk` tracks the most recent `## ` heading and prints it for every matching bullet, so the `grep -c '^## Unreleased'` counts only the bullets under the unreleased section.

```
PREAMBLE=$(grep -n -m1 '^All notable changes to this project' CHANGELOG.md | cut -d: -f1); \
UNREL=$(grep -n -m1 '^## Unreleased' CHANGELOG.md | cut -d: -f1); \
TOPVER=$(grep -n -m1 '^## v' CHANGELOG.md | cut -d: -f1); \
test -n "$PREAMBLE" && test -n "$UNREL" && test -n "$TOPVER" && \
test "$PREAMBLE" -lt "$UNREL" && test "$UNREL" -lt "$TOPVER"
```

The ordering test is the one that matters: `scripts/check-changelog.sh` only catches a section placed *above* the preamble, so a section placed *below* the newest versioned heading would pass `make check-changelog` and still be wrong. This is why the three-line-number comparison is here as well.

**The README subsection:**

```
test "$(grep -c '^### topic$' README.md)" = "1"
test "$(grep -c '^vault-cli topic ' README.md)" = "12"
test "$(for leaf in list lint search get set clear show add remove complete defer work-on; do grep -F -q "vault-cli topic $leaf" README.md || echo "$leaf"; done | wc -l | tr -d ' ')" = "0"
```

The `^### topic$` anchor matters: a bare `grep -c '### topic'` would also match a heading like `### topics`. The twelve-line count is the mechanical form of "all twelve leaves are documented".

**Nothing else moved — no version bump, no scenario, no new doc, no plugin manifest change:**

```
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "5"
test ! -f docs/topic-writing.md
test "$(grep -c 'topic' .claude-plugin/plugin.json .claude-plugin/marketplace.json | grep -vc ':0$')" = "0"
test "$(grep -c 'unreleased' .claude-plugin/plugin.json .claude-plugin/marketplace.json | grep -vc ':0$')" = "0"
```

**The built binary — the spec's container-executable rung, with its build step replaced by a direct toolchain build to a temp path (never a repo-tree build target):**

```
go build -mod=mod -o /tmp/vault-cli-topic .; test "$?" = "0"
test -x /tmp/vault-cli-topic
```

```
/tmp/vault-cli-topic goal --help  | sed -n '/Available Commands:/,/^$/p' | grep '^  [a-z]' | awk '{print $1}' | sort > /tmp/goal-leaves.txt
/tmp/vault-cli-topic topic --help | sed -n '/Available Commands:/,/^$/p' | grep '^  [a-z]' | awk '{print $1}' | sort > /tmp/topic-leaves.txt
test "$(wc -l < /tmp/goal-leaves.txt | tr -d ' ')" = "12"
test "$(wc -l < /tmp/topic-leaves.txt | tr -d ' ')" = "12"
diff /tmp/goal-leaves.txt /tmp/topic-leaves.txt
```

`diff` exits 0 when the two leaf sets are identical. **The count checks are not the assertion** — Acceptance Criterion 1 says so explicitly ("equal counts alone are not sufficient"); the `diff` is. If `diff` prints anything, the two ladders differ member for member.

```
test "$(/tmp/vault-cli-topic topic --help | grep -ciE 'plan-goal|execute-goal|verify-goal|audit-goal|create-goal|update-goal|goal-status|launch-goal|work-on-goal|complete-goal|defer-goal')" = "0"
```

The count is captured inside a command substitution, so `grep -c`'s exit-1-on-zero-count behaviour cannot fire the assertion spuriously — the outer `test` compares the printed `0`.

**The integration suite, which drives the same binary as a subprocess:**

```
go test ./integration/... -v -ginkgo.v -count=1 > /tmp/topic-final-integration.log 2>&1; test "$?" = "0"
grep -F -q -- 'topic --help lists exactly the twelve leaves' /tmp/topic-final-integration.log
grep -F -q -- 'topic --help leaf set equals the goal --help leaf set' /tmp/topic-final-integration.log
grep -F -q -- 'topic --help contains none of the eleven goal slash-command names' /tmp/topic-final-integration.log
grep -F -q -- 'topic show emits phase at .fields.phase, equal to what goal show emits' /tmp/topic-final-integration.log
grep -F -q -- 'topic work-on moves the page into its in-progress state and reports a session outcome' /tmp/topic-final-integration.log
```

Check the run's exit status separately from the name greps: a failing run still prints the spec names. If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment; export it rather than touching `.git`.

**This prompt changed no Go file — confirm the shipped surface is still intact:**

```
test "$(sed -n '/func createTopicCommands(/,/^}/p' pkg/cli/cli.go | grep -c 'cmd.AddCommand(')" = "12"
test "$(grep -c 'rootCmd.AddCommand(createTopicCommands(' pkg/cli/cli.go)" = "1"
test "$(grep -c 'func NewTopicStorage(' pkg/storage/storage.go)" = "1"
test "$(grep -c 'func NewTopic(' pkg/domain/topic.go)" = "1"
test -z "$(gofmt -e -l pkg/cli/cli.go pkg/storage/storage.go pkg/domain/topic.go)"
```

The four greps assert that the surface prompts 1-3 shipped is still present and unchanged; the `gofmt` line asserts the three most-edited Go files are still formatted, which a stray edit from this prompt would break.

Finally, walk all fifteen of spec 051's Acceptance Criteria against the shipped change and report each one in your completion report, per `<requirements>` § 5. For AC 13 and AC 9, state plainly which half is operator-side and why.
</verification>
