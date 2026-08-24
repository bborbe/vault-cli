---
status: completed
spec: [036-passive-per-task-metrics]
summary: 'Documented the passive per-task metrics fields (metrics_sessions, metrics_completed_at, metrics_interaction_count, metrics_cycles) in the work-on-task and complete-task command docs as written-by-vault-cli and not hand-editable, and extended the existing ## Unreleased changelog section with the metrics feat bullet; version strings and tags left untouched.'
execution_id: vault-cli-exec-191-spec-036-plugin-docs-release
dark-factory-version: dev
created: "2026-08-24T18:30:00Z"
queued: "2026-08-24T18:31:32Z"
started: "2026-08-24T20:07:13Z"
completed: "2026-08-24T20:08:58Z"
branch: dark-factory/passive-per-task-metrics
---

<summary>
- The work-on and complete command docs each gain a short note stating that the passive metrics fields are written by vault-cli and must not be hand-edited.
- The changelog gains a single `## Unreleased` `feat:` bullet describing the passive per-task metrics feature.
- No code, no version-string, and no tag changes — this prompt is documentation plus the release bullet only.
- The version strings stay at their current values; the autoRelease releaser owns the bump and tag post-merge.
- The note text describes only fields that prompts 2 and 3 already ship — nothing references code that does not exist yet.
</summary>

<objective>
Document the metrics fields that prompts 2 and 3 made vault-cli write into task frontmatter, so operators know the fields are passive (not hand-edited), and add the single `## Unreleased` changelog bullet the feature needs before the autoRelease releaser converts it to a version.
</objective>

<context>
This prompt depends on prompts `2-spec-036-workon-start-hook.md` and `3-spec-036-complete-end-hook.md` having shipped: the docs describe behavior that already exists in the code. If the metrics hooks are absent from `pkg/ops/`, STOP and report `Status: failed` with "metrics hooks not yet deployed (prompts 2/3)".

Read `/workspace/CLAUDE.md` for project conventions (including the version-alignment and autoRelease rules in the Plugin Release Checklist section) and `/workspace/docs/dod.md`.

Read these files fully before making changes:

- `/workspace/commands/work-on-task.md` — the command doc. It currently has a `## Notes` section near the end. Append the new section after the last section of the file.
- `/workspace/commands/complete-task.md` — the command doc. It currently ends with a `<success_criteria>` block. Append the new section after it.
- `/workspace/CHANGELOG.md` — the top versioned section is currently `## v0.114.6` and there is NO `## Unreleased` section (verified). Determine the insertion point from the file itself, not from memory: `grep -n '^## ' CHANGELOG.md | head -3` shows the first release heading; `## Unreleased` goes immediately above it.
- `/workspace/pkg/ops/workon.go` and `/workspace/pkg/ops/complete.go` — to confirm the exact field names the docs will reference (`metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count`, `metrics_cycles`) — grep for them rather than trusting this list.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format, prefix rules, `## Unreleased` placement.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — prose style (the command-doc notes are plain English, not Go, but the "describe behavior not implementation" principle applies).
</context>

<requirements>

## 1. Document the passive metrics in `commands/work-on-task.md`

Append a new section at the very end of `/workspace/commands/work-on-task.md` (after the existing `## Notes` section's last bullet):

```markdown
## Passive metrics

Each work-on run appends one entry to the task's `metrics_sessions` frontmatter field
(session id + start timestamp). These metrics fields are written passively by vault-cli
and must not be hand-edited.
```

Do not reword, reorder, or delete any existing content.

## 2. Document the passive metrics in `commands/complete-task.md`

Append a new section at the very end of `/workspace/commands/complete-task.md` (after the existing `<success_criteria>` block):

```markdown
## Passive metrics

Completion writes `metrics_completed_at` and, when the task was worked on,
`metrics_interaction_count`. Recurring completion archives the finished cycle into
`metrics_cycles` and resets the accumulator. These metrics fields are written passively
by vault-cli and must not be hand-edited.
```

Do not reword, reorder, or delete any existing content.

## 3. Changelog bullet

Create a `## Unreleased` section in `/workspace/CHANGELOG.md`, placed **below** the preamble block (the `All notable changes…` line and the three `* MAJOR / MINOR / PATCH` lines) and **above the first `## vX.Y.Z` heading in the file** (currently `## v0.114.6`). Never between the `# Changelog` title and the preamble, and never between two released sections.

Determine the insertion point from the file itself: `grep -n '^## ' CHANGELOG.md | head -3` shows the first release heading; `## Unreleased` goes immediately above it. (`scripts/check-changelog.sh` only verifies the preamble precedes the first `## ` section, so a misplacement between two released sections would NOT be caught by `make precommit`.)

```
## Unreleased

- feat: Passive per-task time and interaction metrics — `task work-on` appends one `{session_id, started_at}` entry per run to the task's `metrics_sessions` frontmatter; `task complete` writes `metrics_completed_at` and (when worked on) `metrics_interaction_count` derived from the task's Claude session logs, archiving finished recurring cycles into `metrics_cycles` and resetting the accumulator; no new flags or operator steps
```

Exactly one bullet, exactly one `## Unreleased` section.

## 4. Version strings — do NOT touch

Do NOT rename `## Unreleased` to a version, do NOT bump `.claude-plugin/plugin.json` or either version field in `.claude-plugin/marketplace.json`, and do NOT `git tag`. This repo is `autoRelease: true` (`.maintainer.yaml`); the `github-releaser` owns the version bump across all four strings and the tag, post-merge.

## 5. Scope

Do NOT touch any Go file, any other file under `commands/`, `pkg/`, `docs/`, or `mocks/`. This prompt is the two command-doc sections and the changelog bullet only.

</requirements>

<constraints>
- Metrics land only in the task frontmatter fields `metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count`, and `metrics_cycles`. The docs must state these are written passively by vault-cli and must not be hand-edited (spec AC7).
- No new flags, no opt-out — passive recording is the invariant; the docs must not suggest one.
- Do NOT add a new E2E scenario.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string, do NOT create a git tag (this repo is `autoRelease: true`; the releaser owns version bumps).
- Existing tests must still pass.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Both command docs mention the metrics fields** — each must print `>= 1`:
```
grep -c 'metrics_sessions\|metrics_completed_at\|metrics_interaction_count\|metrics_cycles' commands/work-on-task.md
grep -c 'metrics_sessions\|metrics_completed_at\|metrics_interaction_count\|metrics_cycles' commands/complete-task.md
```

**2. Both docs say "must not be hand-edited"** — each must print `>= 1`:
```
grep -c 'must not be hand-edited' commands/work-on-task.md
grep -c 'must not be hand-edited' commands/complete-task.md
```

**3. Changelog has exactly one `## Unreleased` with a metrics bullet** — each must print exactly `1`:
```
grep -c '^## Unreleased$' CHANGELOG.md
grep -c 'metrics' CHANGELOG.md
```

**4. `## Unreleased` is the FIRST `## ` heading** — must print `## Unreleased`:
```
grep -n '^## ' CHANGELOG.md | head -1
```
If it prints a version heading instead, the section was inserted between two released sections — move it above the first release heading.

**5. Version strings have NOT moved** — must still print the current values:
```
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```
First must still print `## v0.114.6`; second must still show `0.114.6`. (If a prior unrelated release bumped them, report the actual values and proceed — do NOT change them.)

**6. No Go file references metrics fields it did not before** — the metrics hooks live only in `pkg/ops/` and `pkg/domain/` (from prompts 1–3); nothing under `pkg/cli/` or `pkg/storage/` may reference them. Must print `0`:
```
grep -rc 'metrics_' pkg/cli/ pkg/storage/ | grep -v ':0' | wc -l
```

**7. Full gate, once, at the end:**
```
make precommit
```
Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
