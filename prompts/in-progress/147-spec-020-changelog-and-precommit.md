---
status: approved
spec: [020-unify-checkbox-regex-accept-asterisk-prefix]
created: "2026-06-28T10:51:17Z"
queued: "2026-06-28T11:05:59Z"
branch: dark-factory/unify-checkbox-regex-accept-asterisk-prefix
---
<summary>
- `CHANGELOG.md` gains a single `## Unreleased` section containing one `fix(parser):` bullet that names the lint-vs-runtime checkbox-marker mismatch in plain language.
- The section is created at the top of the changelog (no existing `## Unreleased` is present), placed immediately above the current `## v0.91.0` heading.
- The bullet calls out both the silent-skip root cause and the dual-marker acceptance as the fix, so the changelog reads as a real release-note, not a bare commit message.
- No version bump is performed in this prompt — dark-factory handles release versioning from the prefix.
- The four plugin manifest version strings are NOT touched (still aligned with `v0.91.0` from the previous release).
- After the edit, `make precommit` is run once at the repo root as the final acceptance gate.

</summary>

<objective>
Record the spec 020 fix in `CHANGELOG.md` under a freshly-created `## Unreleased` section so users hitting the lint-vs-runtime mismatch can find the resolution in the release notes. No version bump, no plugin-manifest change.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read /home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md for changelog entry format rules.

Read these files fully before editing:
- CHANGELOG.md — top entry is currently `## v0.91.0` at line 11; there is NO `## Unreleased` section above it.
- .claude-plugin/plugin.json — verify the `"version"` field still reads `0.91.0` (do NOT change).
- .claude-plugin/marketplace.json — verify both `metadata.version` and `plugins[0].version` still read `0.91.0` (do NOT change).

The previous prompt (`spec-020-checkbox-regex-asterisk.md`) landed the regex unification + new Ginkgo tests across `pkg/storage/base.go`, `pkg/ops/update.go`, `pkg/ops/complete.go`, `pkg/ops/defer.go`, and `pkg/ops/workon.go`. This prompt runs after that one and assumes the code changes are already committed on the branch `feat/checkbox-accept-asterisk`.

Changelog style from the most recent entries (CHANGELOG.md lines 13–17): one bullet per logical change, prefixed with `<area>(<scope>):` then a verb-led sentence. Examples to mirror:
```
- feat(work-on-task): add Phase 5 auto-sharpen + auto-gate chain — after the assistant returns `Ready to work on this task.`, automatically invoke ...
- feat(goal): align `AvailableGoalStatuses` with task statuses — `next, in_progress, backlog, hold, aborted` accepted alongside legacy `active, completed, on_hold` ...
```
Note the structure: `prefix(scope): verb — what changed; why it changed` (em-dash separates the change from the rationale when rationale is given).
</context>

<requirements>

## Changelog entry

1. In `CHANGELOG.md`, insert a new `## Unreleased` section immediately above the line `## v0.91.0` (currently line 11). The section must contain exactly ONE bullet, formatted as:

   ```
   ## Unreleased

   - fix(parser): accept `* [...]` as well as `- [...]` Markdown checkboxes in storage and ops parsers — `complete-goal`, `complete-task`, `update-task`, `defer-task`, and `work-on-task` now see asterisk-prefixed lines that the linter already accepted, fixing the silent-skip bug where a goal could be marked complete with no Success Criteria actually checked (lint-vs-runtime mismatch; 8 vault files affected)
   ```

   Use a real em-dash (`—`, U+2014) between the change statement and the rationale, matching the style of `v0.91.0` and `v0.90.0`. Do NOT use two ASCII hyphens or an en-dash.

2. The bullet MUST begin with the literal prefix `fix(parser):` (the parenthesized area is `parser`; the verb follows the colon). This prefix is required so dark-factory's prefix-based version bump classifies the change correctly.

3. After the bullet, leave one blank line, then the existing `## v0.91.0` heading and all following content must remain unchanged.

## Verification

4. Run the spec's AC6 grep to confirm the bullet lands correctly:

   ```bash
   grep -nE '^- fix\(parser\):.*asterisk|^- fix\(parser\):.*list marker' CHANGELOG.md
   ```

   Expected: at least one matching line.

5. Confirm the bullet is positioned UNDER an `## Unreleased` heading:

   ```bash
   awk '/^## /{section=$0} /^- fix\(parser\):/{print section, NR, $0}' CHANGELOG.md
   ```

   Expected output line begins with `## Unreleased`.

## Final acceptance gate

6. From the repo root run `make precommit`. It MUST exit 0. If it fails:
   - If the failure is `check-versions`: STOP — this prompt must NOT bump the four version strings. Investigate whether the previous prompt's branch state left versions mis-aligned (unlikely; the previous prompt's `<verification>` did not touch them). Report the divergence in the completion report under `## Improvements` and DO NOT amend version strings to "fix" it.
   - If the failure is any other target: fix, run only the failing target until green, then re-run `make precommit` once.

7. Confirm the four version strings are still aligned with `v0.91.0` (they MUST be — this prompt touches only `CHANGELOG.md`):

   ```bash
   grep -n '"version"' .claude-plugin/plugin.json .claude-plugin/marketplace.json
   head -1 CHANGELOG.md    # intro
   grep -nE '^## v[0-9]+\.[0-9]+\.[0-9]+' CHANGELOG.md | head -1
   ```

   Expected: three `0.91.0` matches in the JSON files plus `## v0.91.0` as the top versioned section.

</requirements>

<constraints>
- Copied from spec 020:
  - This prompt MUST NOT bump any version string. Dark-factory handles release versioning from the `fix(parser):` prefix.
  - This prompt MUST NOT touch `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`. Version alignment with `v0.91.0` must be preserved exactly.
  - This prompt MUST NOT add additional bullets to the `## Unreleased` section. One logical change, one bullet.
  - This prompt MUST NOT modify any code in `pkg/`. The previous prompt (`spec-020-checkbox-regex-asterisk.md`) owns all code edits.
  - Must not amend prior releases or rearrange CHANGELOG.md sections.
- Do NOT use emoji in the changelog bullet.
- Do NOT include code paths or struct names in the bullet (changelog entries are user-facing release notes, not implementation logs).
- Do NOT commit — dark-factory handles git.
- Run `make precommit` once at the very end. Do not re-run individual targets preemptively.

</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. AC6: fix(parser) bullet exists under an Unreleased heading
awk '/^## /{section=$0} /^- fix\(parser\):/{print section, NR, $0}' CHANGELOG.md
# Expected: line begins with "## Unreleased"

# 2. Bullet uses an em-dash, not two hyphens
grep -nE '^- fix\(parser\):' CHANGELOG.md
# Expected: one line; em-dash present in the displayed output (verify with `cat -v` or visual inspection of the diff)

# 3. Four version strings still aligned with v0.91.0
grep -n '"version"' .claude-plugin/plugin.json .claude-plugin/marketplace.json
grep -m1 '^## v' CHANGELOG.md
# Expected: three JSON matches reading "0.91.0"; the CHANGELOG top versioned section is "## v0.91.0"

# 4. No code files touched
git diff --stat pkg/ cmd/
# Expected: no output

# 5. Only CHANGELOG.md modified
git diff --stat
# Expected: exactly one file, CHANGELOG.md, with a single insertion block (the new ## Unreleased section)
```
</verification>

<!-- DARK-FACTORY-REPORT -->