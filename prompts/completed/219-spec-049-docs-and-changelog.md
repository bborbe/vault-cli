---
status: completed
spec: [049-publish-escalation-on-assignee-clear]
summary: Amended the README's standalone and pipeline promises to name the opt-in broker gate, documented the optional notification config, reconciled the two readings of an empty assignee in docs/task-writing.md, and added the changelog bullet under the existing Unreleased section
execution_id: vault-cli-exec-219-spec-049-docs-and-changelog
dark-factory-version: v0.193.0
created: "2026-09-17T13:30:00Z"
queued: "2026-09-17T15:50:49Z"
started: "2026-09-17T17:20:18Z"
completed: "2026-09-17T17:21:57Z"
---

# Documentation: the amended standalone promise, the opt-in gate, and the park semantics

<summary>
- The README stops claiming vault-cli is untouched by Kafka and states the gate that decides whether it ever connects to a broker.
- The README documents the optional configuration that turns the publish on, and what each of its two keys means.
- The task-writing doc explains that an empty assignee means two different things depending on how it became empty — an unclaimed inbox at creation, a park at clear.
- The changelog records the change under Unreleased.
- No version string is bumped and no tag is created; the post-merge releaser owns that.
- No code, no test, and no scenario file changes.
</summary>

<objective>
Make the documentation match the behaviour the two previous prompts shipped: a park performed by hand is now published, and that publish is opt-in. The README's standalone promise currently contradicts the new behaviour in two places, and `docs/task-writing.md` documents an empty assignee as the unclaimed inbox without saying when the same value means an escalation instead. This is spec 049's prompt 3 of 3: it covers Desired Behavior 6 and Acceptance Criterion 7.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `README.md` — the three places you edit. `## Overview` opens with the standalone sentence that contains the string `No Kafka`; `## Where this fits in the bigger picture` contains the paragraph that contains the string `never produces or consumes Kafka events`; `## Configuration` holds the fenced YAML example and is where the new subsection goes. The file's other headings (`## Usage`, `## Claude Code Plugin`, `## Shell Completion`, `## Development`, `## License`) are untouched.
- `docs/task-writing.md` — `### Frontmatter`. The `assignee` semantics paragraph is the one that reads "`assignee` semantics: empty (`\"\"`) means **unclaimed inbox** …". The `vault-cli task work-on` matrix follows it; your new paragraph goes between the two.
- `CHANGELOG.md` — the `# Changelog` title, the `All notable changes…` preamble, the `* MAJOR / MINOR / PATCH` lines, then `## v0.133.0`. There is no `## Unreleased` section yet.
- `scripts/check-changelog.sh` — the structural check `make precommit` runs: the preamble must precede every `## ` section.
- `docs/dod.md` — this repository's `validationPrompt`, in particular its changelog-placement rule.
- `specs/in-progress/049-publish-escalation-on-assignee-clear.md` — the spec. Read Desired Behavior 6, Acceptance Criterion 7, Constraints (the "Documentation tension to reconcile, not leave standing" bullet) and Non-goals.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format and the `## Unreleased` placement rule.
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md` — documentation conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/markdown-todo-guide.md` — checkbox syntax, if you touch a checklist.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

The exact current text you replace, quoted verbatim:

`README.md`, `## Overview` (the second paragraph):

```
Standalone-usable — pure local filesystem I/O against an Obsidian vault. No Kafka, no Kubernetes, no cluster. Works against any Obsidian vault that follows the bborbe frontmatter conventions.
```

`README.md`, `## Where this fits in the bigger picture` (the paragraph after the four bullets):

```
vault-cli itself is **not** a pipeline participant: it never produces or consumes Kafka events. It reads and mutates the vault that the pipeline materializes.
```

`README.md`, `## Configuration` (the existing example's shape):

````
Create `~/.config/vault-cli/config.yaml` (XDG-first location; the legacy path `~/.vault-cli/config.yaml` is still read as a fallback when the XDG file is absent):

```yaml
default_vault: personal
vaults:
  personal:
    name: personal
    path: ~/Documents/Obsidian/Personal
    …
  brogrammers:
    …
```
````

`docs/task-writing.md` (the paragraph your new text follows):

```
`assignee` semantics: empty (`""`) means **unclaimed inbox** (anyone with vault access can pick up); an agent name means the executor should spawn that agent; a human name means that human is currently doing the work.
```

Two environment facts that shape this prompt:

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
2. **`make check-changelog` fails if any `## ` section lands above the preamble.** Place the new section below the preamble block and above `## v0.133.0`.
</context>

<requirements>

## 0. Scope — three documentation files, nothing else

- `README.md` — one sentence replaced in `## Overview`, one paragraph replaced in `## Where this fits in the bigger picture`, one new subsection in `## Configuration`.
- `docs/task-writing.md` — one new paragraph after the `assignee` semantics paragraph.
- `CHANGELOG.md` — one `## Unreleased` section with one bullet.

Nothing else. No Go file, no test, no scenario file, no version string, no `.claude-plugin/` JSON, no `Makefile`, no config file. This prompt is documentation-only; if you find yourself needing a code change, stop and say so in the completion report instead.

## 1. `README.md` — amend the standalone promise

Replace the `## Overview` standalone sentence with exactly this text:

```
Standalone-usable — local filesystem I/O against an Obsidian vault. No Kubernetes, no cluster. Works against any Obsidian vault that follows the bborbe frontmatter conventions. It opens a Kafka broker connection only when the optional `notification` section of the config file names brokers; with no brokers configured it publishes nothing and connects nowhere.
```

Non-negotiable properties:

- The word `brokers` (and therefore `broker`) must appear in this sentence. It is the first `broker` occurrence in the file, and the acceptance criterion requires one within this amended standalone section.
- The exact string `No Kafka` must be gone from the file. Do NOT keep it as a parenthetical, a struck-through phrase, or a "previously" note.
- The word `pure` must be gone from this sentence: the I/O is no longer purely local, and leaving the claim while adding the gate would contradict itself in one sentence.
- Keep the sentence's role and position — it stays the standalone promise in `## Overview`, not a new section.

## 2. `README.md` — replace the pipeline-participation paragraph

Replace the paragraph in `## Where this fits in the bigger picture` with exactly this text:

```
vault-cli reads and mutates the vault that the pipeline materializes. Its only pipeline write is opt-in and narrow: when the config file names brokers, clearing a task's assignee publishes one `agent-escalation` notification into the shared notification core, so a park performed by hand tells the agent side what happened.
```

Non-negotiable properties:

- The exact string `never produces or consumes Kafka events` must be gone from the file. Deletion alone does not satisfy the acceptance criterion — the replacement must be present and must name the gate, which the phrase `when the config file names brokers` does.
- Do NOT add a second paragraph, a table, or a diagram. Do NOT touch the four bullets above this paragraph, and do NOT touch the `Full system map:` line below it.

## 3. `README.md` — document the optional configuration

Add this subsection at the end of `## Configuration`, after the existing YAML example and before the `## Usage` heading. Its heading level is `###`, matching the `###` subsections already used under `## Usage`:

````markdown
### Notification (optional)

Clearing a task's assignee — `vault-cli task set "<task>" assignee ""` or `vault-cli task clear "<task>" assignee` — publishes one `agent-escalation` notification into the shared notification core, so a park performed by hand reaches the operator's chat. This is opt-in and off by default: omit the section and vault-cli never opens a broker connection.

```yaml
notification:
  brokers: "broker-1:9092,broker-2:9092"
  topic_prefix: "master"
```

- `brokers` — comma-separated broker addresses. Empty or absent means no publish and no connection attempt.
- `topic_prefix` — the deployment's Kafka topic prefix, the same value the consuming services derive from their branch (`master` in prod, `develop` in dev). A producer that omits it publishes into a topic nothing consumes.

The publish is bounded to a single attempt of five seconds. A failure or an unreachable broker is logged, never fails the command, never changes its exit code or output, and never delays it past that bound.
````

Non-negotiable properties:

- The YAML keys are exactly `notification`, `brokers`, `topic_prefix`, and they sit at the TOP level of the config file — not inside a vault. The example block must show them at that level.
- Do NOT add the keys to the existing `personal` or `brogrammers` vault entries in the example above.
- Do NOT document a chat token, a chat id, a channel name, or any per-channel secret. Routing is the shared core's job, and no such value exists in this repository.
- Do NOT renumber, reword, or restructure anything else under `## Configuration`.

## 4. `docs/task-writing.md` — reconcile the two readings of an empty assignee

Insert this paragraph immediately after the existing `assignee` semantics paragraph and immediately before the paragraph that begins "`vault-cli task work-on` applies a three-case matrix":

```
An empty `assignee` means two different things depending on how it became empty. A task that has never been assigned is the **unclaimed inbox** above — anyone with vault access can pick it up. A task whose assignee was cleared was *parked*: the baton went back to the operator, and `vault-cli task set "<name>" assignee ""` (or `vault-cli task clear "<name>" assignee`) publishes one `agent-escalation` notification into the shared notification core when the config file names brokers, so the agent side learns the task was handed back. The unclaimed-inbox reading applies at creation; the park reading applies at clear.
```

Non-negotiable properties:

- The paragraph must state WHICH reading applies WHEN — that is the whole point of the reconciliation. The final sentence ("The unclaimed-inbox reading applies at creation; the park reading applies at clear.") is frozen and must be present verbatim.
- The phrase `agent-escalation` must appear. The phrase `unclaimed inbox` keeps its existing meaning and its existing wording in the paragraph above; do not edit that paragraph.
- Do NOT delete or rewrite the `work-on` matrix paragraph below, and do NOT change the table it introduces.
- Do NOT add a second paragraph, a warning callout, or a new heading.

## 5. `CHANGELOG.md` — one bullet under a new `## Unreleased`

There is no `## Unreleased` section yet. Create it directly below the preamble block — after the `* PATCH version when you make backwards-compatible bug fixes.` line and above `## v0.133.0` — with this bullet:

```
## Unreleased

- feat: clearing a task's assignee through `vault-cli task set "<task>" assignee ""` or `vault-cli task clear "<task>" assignee` now publishes one `agent-escalation` notification into the shared notification core, so a park performed by hand reaches the operator's chat instead of staying silent. The publish is opt-in — with no `notification.brokers` in the config file vault-cli publishes nothing and opens no connection — and bounded to a single five-second attempt whose failure is logged and never fails the command, never changes its exit code or output, and never delays it past the bound.
```

Non-negotiable properties:

- The bullet starts with `- feat:` on a single line and names the assignee-clear publish. The phrase `agent-escalation` must appear.
- Do NOT bump any version: not `CHANGELOG.md`'s newest version heading, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. `.maintainer.yaml` sets `release.autoRelease: true`, so the post-merge releaser converts `## Unreleased` into a versioned section and tags it; a hand-bump would race it. `make check-versions` must still print the same version it prints today.
- `make check-changelog` fails if any `## ` section lands above the preamble. Place the section below the preamble, as described.

## 6. What this prompt must NOT do

- Do NOT add or edit a scenario file. `scenarios/` keeps exactly its five existing `.md` files. The spec's post-deploy proof is an operator-side rung on the spec itself, not a committed scenario.
- Do NOT edit `docs/releasing-vault-cli.md`, `docs/development-patterns.md`, `docs/output-formatting.md`, or any other file under `docs/`.
- Do NOT edit the README's `## Usage` command list. `vault-cli task set … assignee ""` and `vault-cli task clear … assignee` are already documented there.
- Do NOT mention internal cluster names, hostnames, or the operator's broker addresses anywhere in the documentation — use placeholder broker addresses only.
- Do NOT add a `## Unreleased` bullet to a released section, and do NOT edit any existing changelog bullet.

## 7. Self-check before finishing

- Re-read the three edited regions and confirm: the `## Overview` sentence names the gate, the pipeline paragraph is replaced rather than annotated, the new subsection sits inside `## Configuration`, and the task-writing paragraph states which reading applies when.
- Confirm the tree contains no code, test, or scenario change from this prompt.
- Walk spec 049's Acceptance Criterion 7 and Desired Behavior 6 and state in the completion report which requirement satisfies each, naming the exact grep that proves it.
- Walk `docs/dod.md`'s documentation section: the README reflects the configuration change, and the changelog entry sits under `## Unreleased` below the preamble and above the newest `## vX.Y.Z`.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 049 — non-goals.** Do NOT change the controller-side publish path or its behaviour. Do NOT re-notify on later edits of an already-parked task. Do NOT backfill notifications for tasks parked before this ships. Do NOT cover a raw editor write of the assignee field. Do NOT implement Matrix delivery. Do NOT make the publish mandatory — a deployment with no broker configured must keep today's behaviour exactly.
- **Copied from spec 049 — the documentation tension.** `docs/task-writing.md` documents an empty assignee as the *unclaimed inbox*; this change treats the same transition as *escalate to the operator*. Both readings ship in this repository, so the doc update must state which applies when. That reconciliation is requirement 4 and is not optional.
- **Copied from spec 049 — constraints.** No chat credentials: no chat token, chat id, or per-channel secret enters this repository, and none may be documented as if it did. The topic prefix is required for the deployed topology: a producer that omits it publishes into a topic nothing consumes, which is why requirement 3 documents the key. The metadata the notification carries is `taskIdentifier`, `taskName`, `previousAssignee`.
- **Frozen strings.** The `## Overview` sentence must contain `brokers`; the pipeline paragraph must contain `when the config file names brokers`; the new README subsection's keys are `notification`, `brokers`, `topic_prefix`; the task-writing paragraph's last sentence is `The unclaimed-inbox reading applies at creation; the park reading applies at clear.`; the changelog bullet starts with `- feat:` and contains `agent-escalation`. All of these are grep targets.
- **Both contradicting README statements must be GONE.** `grep -c 'never produces or consumes Kafka events' README.md` must be 0 AND `grep -c 'No Kafka' README.md` must be 0. Deletion alone does not pass: the replacement must be present and must name the gate.
- **No version bumps.** Leave `CHANGELOG.md`'s newest version heading and both `.claude-plugin/` JSON files untouched.
- **No scenario file.** `scenarios/` keeps exactly five `.md` files.
- **Documentation only.** No Go file, no test, no `Makefile`, no config file, no code change of any kind.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make check-changelog`, `make check-versions`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. Absence is written as `! grep -q` on purpose: `grep -c` prints `0` and exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so an absence assertion written as a count would report the wrong status.

**The two contradicting README statements are gone, and the replacement names the gate:**

```
! grep -q 'never produces or consumes Kafka events' README.md
! grep -q 'No Kafka' README.md
! grep -q 'pure local filesystem I/O' README.md
test "$(grep -c 'when the config file names brokers' README.md)" = "1"
test "$(grep -c 'broker' README.md)" -ge 1
```

**The `broker` mention lands inside the amended standalone section, not merely somewhere in the file** — the first occurrence must sit in `## Overview`, above the `## Where this fits in the bigger picture` heading:

```
test "$(grep -n 'broker' README.md | head -1 | cut -d: -f1)" -gt "$(grep -n '^## Overview' README.md | cut -d: -f1)"
test "$(grep -n 'broker' README.md | head -1 | cut -d: -f1)" -lt "$(grep -n '^## Where this fits in the bigger picture' README.md | cut -d: -f1)"
```

**The optional configuration is documented, at the top level, with no credential:**

```
test "$(grep -c '^### Notification (optional)' README.md)" = "1"
test "$(grep -c '^notification:' README.md)" = "1"
test "$(grep -c '^  topic_prefix: "master"' README.md)" = "1"
test "$(grep -n '^notification:' README.md | cut -d: -f1)" -gt "$(grep -n '^## Configuration' README.md | cut -d: -f1)"
test "$(grep -n '^notification:' README.md | cut -d: -f1)" -lt "$(grep -n '^## Usage' README.md | cut -d: -f1)"
test "$(grep -rniE 'telegram|chat_id|chat_token' README.md docs/task-writing.md | wc -l | tr -d ' ')" = "0"
```

**The task-writing reconciliation states which reading applies when:**

```
test "$(grep -c 'The unclaimed-inbox reading applies at creation; the park reading applies at clear.' docs/task-writing.md)" = "1"
test "$(grep -c 'agent-escalation' docs/task-writing.md)" -ge 1
test "$(grep -c 'unclaimed inbox' docs/task-writing.md)" -ge 1
test "$(grep -n 'The unclaimed-inbox reading applies at creation' docs/task-writing.md | cut -d: -f1)" -gt "$(grep -n 'unclaimed inbox' docs/task-writing.md | head -1 | cut -d: -f1)"
test "$(grep -n 'The unclaimed-inbox reading applies at creation' docs/task-writing.md | cut -d: -f1)" -lt "$(grep -n 'applies a three-case matrix' docs/task-writing.md | cut -d: -f1)"
```

**The changelog section is in the right place and names the change:**

```
test "$(grep '^## ' CHANGELOG.md | head -1)" = "## Unreleased"
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "1"
test "$(grep -cE '^- feat:.*assignee' CHANGELOG.md)" -ge 1
test "$(grep -c 'agent-escalation' CHANGELOG.md)" -ge 1
make check-changelog
make check-versions
```

If `grep '^## ' CHANGELOG.md | head -1` prints a version heading, the bullet was placed between released sections — move it into the `## Unreleased` section directly below the preamble. `make check-versions` must print the same version it printed before this prompt; a different version means something bumped a version string that this change must leave alone.

**Nothing but documentation changed:**

```
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "5"
test -f specs/in-progress/049-publish-escalation-on-assignee-clear.md
```

**The full gate:**

```
make precommit
```

Finally, walk spec 049's Acceptance Criterion 7 and Desired Behavior 6 against the change and state in your completion report which requirement and which grep satisfies each one.
</verification>

<!--
OPEN QUESTIONS FOR THE AUDITOR — not instructions for the executing agent.

1. Acceptance Criterion 7's README evidence says `grep -n 'broker' README.md` must
   return >= 1 "within the amended standalone section" without naming which section
   that is. This prompt reads "the standalone section" as `## Overview` — where the
   standalone promise lives — and pins the FIRST `broker` occurrence to that section
   with two line-number checks. If the author meant the amended pipeline paragraph in
   `## Where this fits in the bigger picture` instead, the two line-number checks in
   <verification> must be re-pointed; the `>= 1` count check passes either way.

2. The spec requires the README's standalone promise to be "amended to match the new
   behaviour rather than left contradicting it" but does not say whether the words
   "pure local filesystem I/O" must go. This prompt removes them, because keeping a
   claim of pure local I/O while adding a broker gate contradicts itself inside one
   sentence. If the intent was a narrower edit, drop the `! grep -q 'pure local
   filesystem I/O'` check.

3. The spec names three documentation surfaces (README, `docs/task-writing.md`,
   CHANGELOG). This prompt adds a `### Notification (optional)` subsection to the
   README's existing `## Configuration` section so an operator can actually find the
   two config keys; the acceptance criterion does not require it. It is the one
   addition here that goes beyond the criterion's literal evidence, and it is
   justified by the feature being operator-configured.

4. `docs/releasing-vault-cli.md` and the spec's own `deploy_check` mention that
   `.dark-factory.yaml` sets `autoRelease: false` while `.maintainer.yaml` carries
   `release.autoRelease: true`, so the tag is cut post-merge by the releaser. This
   prompt therefore bumps nothing. If the operator intends to hand-cut the release
   instead, that is a separate, operator-side action.
-->

<!-- DARK-FACTORY-REPORT -->
