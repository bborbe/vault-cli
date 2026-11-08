---
status: approved
spec: [066-bug-approved-record-todo-inbox-deadlock]
created: "2026-11-08T18:04:30Z"
queued: "2026-11-08T18:15:49Z"
branch: dark-factory/bug-approved-record-todo-inbox-deadlock
---

# Record the contract change and the changelog entry (spec 066, prompt 3 of 3)

<summary>
- The task-writing guide's approval paragraph no longer says the command refuses a row that already carries an approval record; it states that such a row is advanced, not re-approved.
- The amended text names the three guarantees the repair makes: the record is never overwritten, a missing key is never fabricated, and no owner is required or written.
- The changelog gains an unreleased section above the newest released version, with one bullet describing the approve repair and the new lint finding.
- No version string is bumped and no tag is created: this repository's release automation owns both after merge.
- No Go code changes in this prompt; the batch's earlier two prompts own the behaviour and its tests.
- The documentation is only amended after confirming the code it describes has actually shipped, so the guide can never describe behaviour the binary lacks.
</summary>

<objective>
Bring the durable contract in line with the shipped behaviour: amend the `docs/task-writing.md` approval sentence that documents a refusal which no longer exists, and record the change in `CHANGELOG.md` under a new `## Unreleased` section. This is spec 066 prompt 3 of 3: it covers Acceptance Criterion 10 and depends on prompts 1 and 2.
</objective>

<context>
Read `CLAUDE.md` first, then `docs/dod.md`.

Read these files before editing:

- `docs/task-writing.md` lines 500–535 — the `### Phase transitions` section. The sentence to replace is in the paragraph beginning "**The approval is `vault-cli task approve`.**".
- `CHANGELOG.md` lines 1–20 only — the frozen preamble ends at the SemVer bullet block; the first section is `## v0.188.3` and there is NO `## Unreleased` section today. That is the whole edit surface.
- `scripts/check-changelog.sh` — the structural rule: the `# Changelog` title and the "All notable changes…" preamble must precede every `## ` section; `## Unreleased` before `## vX.Y.Z` is the expected order.
- `scripts/check-versions.sh` — it reads the top `^## v` line with `grep -m1`, so a `## Unreleased` heading does not disturb it. This is why adding the section needs no version bump.
- `pkg/ops/task_approve.go` and `pkg/ops/lint.go` — read only, to confirm the behaviour you are documenting exists. Do not modify either.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the unreleased-section placement rule, the frozen-preamble rule, the required conventional prefixes, and the rule that in an auto-release repository a feature branch adds bullets under the unreleased section and does NOT bump version strings or tag.

`git` may be masked in this container, so this prompt issues no git commands; the version-alignment invariant is checked with greps instead.
</context>

<requirements>
1. **Fail-loud gate — prompts 1 and 2 must have shipped.** Confirm before editing:

   ```
   grep -c 'hasApprovalRecord' pkg/ops/task_approve.go                    # must be >= 2
   grep -c 'IssueTypeApprovedRecordInTodoInbox' pkg/ops/lint.go           # must be >= 4
   grep -c 'fixApprovedRecordInTodoInbox' pkg/ops/lint.go                 # must be >= 2
   ```

   If ANY check fails, STOP and report `"status":"failed"` with message `"spec-066 prompt 3 precondition missing: prompt 1 or prompt 2 not yet deployed"`. Do not document behaviour the code does not have.

2. **Amend the approval sentence in `docs/task-writing.md`.** Find this exact sentence inside the paragraph that begins `**The approval is \`vault-cli task approve\`.**`:

   ```
   The command refuses any row that is not at `phase: todo`, and any row that already carries `approved_by` or `approved_at`, writing nothing in either case.
   ```

   Replace it with:

   ```
   The command refuses any row that is not at `phase: todo`, writing nothing. A row that already carries `approved_by` or `approved_at` is already approved, so the command does not re-approve it: it completes the transition by advancing `phase` to `planning` and leaves the existing record exactly as it is — it never overwrites either key, never fabricates a missing one, and does not require or set an owner.
   ```

   Replace only that sentence. Leave the rest of the paragraph — the `--by` default sentence, the delegation paragraph, the table row for `todo`, and every other section — byte-identical. Do not add a lint-rule section: the spec's decomposition scopes this prompt to the one sentence.

3. **Ensure exactly one `## Unreleased` section carries this bullet in `CHANGELOG.md`.** Prompt 1 may already have created the section and written its own bullet for the `task approve` change — check first with `grep -n '^## Unreleased' CHANGELOG.md`.

   - **If `## Unreleased` already exists**, add your lint bullet to that section (under its existing bullets) and leave the existing bullet untouched. Do not create a second `## Unreleased` heading.
   - **If it does not exist**, insert it immediately after the preamble's SemVer bullet block and immediately before `## v0.188.3`, in this shape:

   ```
   ## Unreleased

   - fix: `task approve` is now idempotent on a `phase: todo` row that already carries `approved_by` / `approved_at`: instead of refusing, it advances the phase to `planning` and preserves the record verbatim — no second approval, no fabricated key, no owner required. `task lint` reports the same state as a new fixable `APPROVED_RECORD_IN_TODO_INBOX` finding whose fix performs the same advance, and reports it non-fixable on a `completed` / `aborted` row so a terminal row is never silently re-opened.

   ```

   Exactly one bullet **from this prompt**, as the spec's decomposition states. Use the `fix:` prefix — the change repairs a defect, not a new capability. Prompt 1's own bullet may sit in the same section; leave it alone.

4. **Do not bump any version string and do not create a tag.** This repository is `autoRelease: true` for the releaser path described in `CLAUDE.md`; the release automation converts `## Unreleased` into a versioned section, bumps `.claude-plugin/plugin.json` and both `.claude-plugin/marketplace.json` versions, and tags after merge. The four version strings stay at `0.188.3` in this branch. No code file changes in this prompt.
</requirements>

<constraints>
- Copied from the spec: no new CLI verb, subcommand or flag; no `go.mod` / `go.sum` change.
- Copied from the spec: the changelog carries a `## Unreleased` heading above `## v0.188.3` with one bullet. Exactly one such heading may exist — if prompt 1 already created it, add to it rather than duplicating it.
- Do NOT bump `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` (`metadata.version` and `plugins[0].version`), or add a versioned changelog section. All four stay at `0.188.3`.
- Do NOT create or move a git tag.
- Do NOT modify any Go file, test file, or scenario file.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` once — it must exit 0. It runs the changelog structure check, so a section inserted above the preamble or a malformed section fails here.

Then confirm the edit surface:

```
grep -c '^## Unreleased' CHANGELOG.md                       # expect 1
awk '/^## /{print NR": "$0}' CHANGELOG.md | head -2          # the Unreleased line must print first
grep -m1 '^## v' CHANGELOG.md                                # still ## v0.188.3
grep -c 'does not re-approve it' docs/task-writing.md        # expect 1
! grep -q 'and any row that already carries' docs/task-writing.md  # must find nothing
jq -r .version .claude-plugin/plugin.json                    # still 0.188.3
jq -r '.metadata.version' .claude-plugin/marketplace.json    # still 0.188.3
jq -r '.plugins[0].version' .claude-plugin/marketplace.json  # still 0.188.3
```

Do not emit any `git` command; this container's `.git` may be masked and the daemon does not grade verification exit codes.
</verification>
