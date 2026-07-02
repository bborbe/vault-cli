---
status: draft
created: "2026-07-02T21:10:00Z"
---

<summary>
- A recent automated change produced a malformed CHANGELOG — a new section was inserted above the file's introductory preamble — and it shipped unnoticed because the pre-commit build never checks CHANGELOG structure.
- This adds a guard so a misplaced section fails the pre-commit build with a clear, actionable message instead of silently releasing.
- The correct, well-formed CHANGELOG keeps passing with no friction.
- The guard runs on every commit and in CI, closing the gap that let the broken file through.
- It enforces the ordering the project documentation already describes: title, then preamble, then the change sections.
</summary>

<objective>
Add a CHANGELOG-structure guard: a new `scripts/check-changelog.sh` that verifies the `# Changelog` title and the "All notable changes…" preamble precede every `##` section, wired into `make precommit` via the `check` target. No application code changes.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read these files before implementing:
- `scripts/check-versions.sh` — the STYLE TEMPLATE for check scripts: `#!/usr/bin/env bash`, `set -euo pipefail`, `ROOT=$(cd "$(dirname "$0")/.." && pwd); cd "$ROOT"`, a small `report()`/`echo` reporting style, exit non-zero on failure. Mirror this shape and tone.
- `Makefile`:
  - `check: lint vet vulncheck osv-scanner trivy` (around line 67) — the aggregate target that `precommit` depends on. Add `check-changelog` to this list.
  - `precommit: ensure format generate test check addlicense` (around line 23) — it depends on `check`, so wiring into `check` is sufficient (no need to edit the precommit line).
  - `check-versions` (around lines 26-28) — shows the `.PHONY:` + `@bash scripts/<name>.sh` target pattern to copy for the new target.
- `CHANGELOG.md` — the invariant to enforce: line 1 is `# Changelog`; then a preamble block (the line `All notable changes to this project will be documented in this file.`, the `Please choose versions…` line, and three `* MAJOR/MINOR/PATCH` lines); THEN the `##` sections (`## Unreleased` optional, then `## vX.Y.Z` newest-first). No `##` heading may appear before the `All notable changes` preamble line.

The specific bug this guards against: a `## Unreleased` section created ABOVE the preamble (between `# Changelog` and `All notable changes`).
</context>

<requirements>
1. Add `scripts/check-changelog.sh` (mirror `scripts/check-versions.sh`: `#!/usr/bin/env bash`, `set -euo pipefail`, resolve `ROOT` and `cd` to it):
   - `PREAMBLE_LINE=$(grep -n -m1 '^All notable changes to this project' CHANGELOG.md | cut -d: -f1 || true)` — the trailing `|| true` is REQUIRED so a no-match yields an empty string instead of aborting the script under `set -euo pipefail` (a no-match `grep` in a command-substitution pipeline otherwise exits non-zero and kills the script before the emptiness guard runs).
   - `FIRST_SECTION_LINE=$(grep -n -m1 '^## ' CHANGELOG.md | cut -d: -f1 || true)` — same `|| true` requirement.
   - FAIL (exit 1, message naming the offending line number/heading) if `FIRST_SECTION_LINE` is non-empty AND `FIRST_SECTION_LINE` < `PREAMBLE_LINE` — a `##` section precedes the preamble.
   - FAIL (exit 1, clear message) if `PREAMBLE_LINE` is empty (preamble missing).
   - On success print a one-line OK message (e.g. `CHANGELOG structure OK`) and exit 0.
   - Guard all arithmetic against empty strings so `set -u`/`set -e` don't misfire on a missing match.
2. Add a Makefile target and wire it into `check`:
   ```
   .PHONY: check-changelog
   check-changelog:
   	@bash scripts/check-changelog.sh
   ```
   and append `check-changelog` to the `check:` aggregate target so `precommit` runs it.
3. The current CHANGELOG has NO `## Unreleased` heading (its newest section is `## v0.96.3`). CREATE the `## Unreleased` section below the preamble and above `## v0.96.3`, and add the entry there — per `docs/dod.md`. (This also dog-foods the new check against a correctly-structured file.)
4. Run `make precommit` — must pass (the current CHANGELOG is well-formed, so the new check passes).
</requirements>

<constraints>
- Shell: `set -euo pipefail`; quote all variable expansions; POSIX-friendly enough for `/usr/bin/env bash` on macOS + Linux CI.
- Do NOT modify application Go code.
- Do NOT commit — dark-factory handles git.
- The check MUST pass on the current (correct) CHANGELOG and MUST fail on a `## Unreleased`/version section placed above the preamble.
- Do NOT bump the four version strings — the autoRelease bot versions `## Unreleased` on merge.
</constraints>

<verification>
Run `make precommit` — passes (the new check runs via `check`).
Run `bash scripts/check-changelog.sh` — exits 0 on the current CHANGELOG, prints the OK line.
Negative check: on a scratch copy (e.g. `cp CHANGELOG.md /tmp/cl.md` then move a `## Unreleased` line above the preamble in the copy, and point the script at it or reproduce its logic) confirm it would exit non-zero — do NOT leave `CHANGELOG.md` modified.
Run `grep -n "check-changelog" Makefile` — target defined AND listed in the `check:` aggregate.
</verification>
