---
status: prompted
tags:
    - dark-factory
    - spec
approved: "2026-08-19T09:43:16Z"
generating: "2026-08-19T09:45:56Z"
prompted: "2026-08-19T10:02:31Z"
branch: dark-factory/prepare-compact-plugin-command
---

## Summary

- `/prepare-compact` (pre-`/compact` checkpoint: sync vault progress, surface live background state, write a resume block) currently lives only in the per-machine `~/.claude/commands/` directory — colleagues get it only by hand-copying the file.
- Promote it into the vault-cli plugin as `commands/prepare-compact.md` (`/vault-cli:prepare-compact`), so it ships via normal plugin install/update like its siblings `session-close`, `sync-progress`, `update-task`.
- All 6 steps of the source command are carried over faithfully: sync-progress call, goal/task sweep, compact-safety checks (git + live background state + unanswered gates), resume-block emission, per-session checkpoint file, verdict.
- `allowed-tools` moves from the source's bare `Bash` to a granular per-capability list — mirrors `session-close.md`'s house style and makes the command's own "read-and-report only, never auto-commit/push/kill" invariant enforced by capability, not just prose.
- The command's only writes remain vault progress updates (steps 1–2, delegated to `vault-cli:sync-progress`) plus the per-session checkpoint file (step 5) — no git/docker mutation, ever.

## Problem

`/prepare-compact` is a compaction-safety checklist: it flushes progress to the vault, reports what compaction would otherwise silently drop (uncommitted work, live background processes, unanswered decisions), and writes a per-session resume file. It is genuinely useful to anyone running Claude Code sessions against a vault-cli-tracked vault — but today it exists only as a file under one machine's `~/.claude/commands/`, invisible to every other install. Its two closest siblings, `session-close` and `sync-progress`, already ship as `/vault-cli:*` plugin commands; leaving `prepare-compact` outside is both an inconsistent product surface and a single point of loss (the file lives on one machine only).

## Goal

`/vault-cli:prepare-compact` exists as a first-class vault-cli plugin command, installed and updated the same way as `/vault-cli:session-close` and `/vault-cli:sync-progress`. Its behavior is unchanged from the source command in `~/.claude/commands/prepare-compact.md` except for namespace-fit adjustments: the `vault-cli:sync-progress` call becomes an in-plugin sibling reference, `allowed-tools` becomes a granular per-capability list, and any self-reference to `/vault-cli:session-close` reads correctly from inside the plugin.

## Non-goals

- Rewriting `prepare-compact`'s internal logic (step content, checks, thresholds, wording of the resume block, the "stopped daemon does not mean stopped work" warning) beyond what the namespace move requires. This is a port, not a redesign.
- Any cross-plugin graceful-degradation shim for the `sync-progress` call — the call becomes an ordinary same-plugin sibling reference, no fallback machinery needed.
- Migrating any other command out of `~/.claude/commands/`. Scope is `prepare-compact` only.
- Deciding the fate of the local `~/.claude/commands/prepare-compact.md` duplicate on the operator's machine — tracked separately in the vault task, not by this spec. The implementation must not touch any path outside this repository.
- Adding a configurable opt-out (e.g. a flag to skip the checkpoint write, or to widen `allowed-tools` back to bare `Bash`) — no consumer has asked for one, and it would puncture the exact invariant (read-and-report-only, capability-enforced) this spec exists to preserve. If a future consumer needs a variant, that is a separate spec.

## Acceptance Criteria

- [ ] `ls commands/prepare-compact.md` succeeds — evidence: exit code 0.
- [ ] Frontmatter declares a granular `allowed-tools` list, not bare `Bash` — evidence: `grep -c '^  - Bash$' commands/prepare-compact.md` returns `0` (no unscoped `Bash` entry anywhere in the frontmatter list).
- [ ] `allowed-tools` grants every Bash capability the body actually invokes — evidence: `grep -cE 'Bash\((git status|git log|git rev-parse|pgrep|dark-factory status|docker ps)' commands/prepare-compact.md` returns `6` (one scoped entry per capability: `git status`, `git log`, `git rev-parse`, `pgrep`, `dark-factory status`, `docker ps`).
- [ ] `allowed-tools` grants the non-Bash capabilities the body uses — evidence: `grep -cE '^  - (Skill|Read|Write|Glob)$' commands/prepare-compact.md` returns `4`.
- [ ] `allowed-tools` grants directory-creation for the checkpoint dir — evidence: `grep -c 'Bash(mkdir:' commands/prepare-compact.md` returns `1`.
- [ ] Step 1 (sync-progress) is present and phrased as a same-plugin sibling call — evidence: `grep -n 'vault-cli:sync-progress' commands/prepare-compact.md` returns ≥1 match.
- [ ] Step 2 (goal/task sweep) is present — evidence: `grep -n 'Sweep every goal' commands/prepare-compact.md` returns ≥1 match.
- [ ] Step 3 (compact-safety checks) is present in full, including the mechanical-check commands and the stopped-daemon warning — evidence: `grep -n 'Compact-safety checks' commands/prepare-compact.md` returns ≥1 match AND `grep -n 'A stopped daemon does not mean stopped work' commands/prepare-compact.md` returns ≥1 match.
- [ ] Step 3's graceful-degradation fallback for absent tooling is preserved — evidence: `grep -c '|| echo "no daemon"' commands/prepare-compact.md` returns ≥1 AND `grep -c '|| echo "no containers"' commands/prepare-compact.md` returns ≥1 — the literal shell fallback wiring, not merely the words appearing in prose, proving a missing `dark-factory`/`docker` is reported rather than fatal.
- [ ] Step 4 (resume block) is present with its 4 fields — evidence: `grep -n 'RESUME AFTER COMPACT' commands/prepare-compact.md` returns ≥1 match AND `grep -cE 'Next action:|Live background:|Un-pushed / uncommitted:|Open decision:' commands/prepare-compact.md` returns `4`.
- [ ] Step 5 (per-session checkpoint) preserves the per-session (not shared-fixed-path) invariant — evidence: `grep -n 'compact-checkpoints/<session-id>.md' commands/prepare-compact.md` returns ≥1 match AND `grep -n 'Per session, not a single fixed path' commands/prepare-compact.md` returns ≥1 match.
- [ ] Step 6 (verdict) preserves both verdict strings — evidence: `grep -c 'Ready to compact' commands/prepare-compact.md` returns ≥1 AND `grep -c 'Not compact-safe yet' commands/prepare-compact.md` returns ≥1.
- [ ] The read-and-report-only invariant statement is preserved verbatim in intent — evidence: `grep -n 'Never auto-commit, auto-push, kill a daemon' commands/prepare-compact.md` returns ≥1 match.
- [ ] The command does not emit a session-close-style closer panel, and the instruction forbidding it survives the port verbatim — evidence: `grep -c '⚪ DONE' commands/prepare-compact.md` returns exactly `1`, and that sole occurrence is the prohibition sentence itself (`grep -c 'Do NOT emit a' commands/prepare-compact.md` returns ≥1). Negative check that no actual closer panel is present: `grep -c '👤 You:' commands/prepare-compact.md` returns `0` (every closer panel carries that line; prepare-compact must never emit one). NOTE: the source file contains `⚪ DONE` once, on its line 82, inside the prohibition. An AC demanding `0` is unsatisfiable by a faithful port and would push the implementer to reword the safety instruction to dodge the grep — do not reintroduce that form.
- [ ] The contrast with `/vault-cli:session-close` is explicit and correctly namespaced — evidence: `grep -n '/vault-cli:session-close' commands/prepare-compact.md` returns ≥1 match (no bare, un-namespaced `/session-close` reference — `grep -c '[^:]/session-close' commands/prepare-compact.md` returns `0`).
- [ ] Step 3's remaining narrative elements survive the port — evidence: `grep -c 'Unanswered gates' commands/prepare-compact.md` returns ≥1 AND `grep -c 'A running dark-factory daemon is worth pausing for' commands/prepare-compact.md` returns ≥1.
- [ ] Step 5's load-bearing instruction to surface the checkpoint path to the operator survives — evidence: `grep -c 'state the full path in your final message' commands/prepare-compact.md` returns ≥1 (without this the checkpoint file is written but never announced, defeating its purpose).
- [ ] Frontmatter carries a discoverability `description` — evidence: `grep -c '^description: Pre-compaction checkpoint' commands/prepare-compact.md` returns `1`.
- [ ] `CHANGELOG.md` records the new command under `## Unreleased` — evidence: `sed -n '/## Unreleased/,/## v/p' CHANGELOG.md | grep -c prepare-compact` returns ≥1.
- [ ] `make precommit` exits 0 — evidence: exit code.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — repo-wide format/generate/test/check gate stays green.
- Every `grep` / `sed` command listed in Acceptance Criteria, run from the repo root.

### Operator-executable (runs on the host after PR merge, per this repo's `autoRelease: true` plugin release flow)

- After the `## Unreleased` bullet is released and tagged, `claude plugin update vault-cli@vault-cli` picks up the new version.
- After a Claude Code restart, `/vault-cli:prepare-compact` is available as a slash command (manual smoke check — invoke it once in a vault-cli-tracked session and confirm the six-step output shape).

## Desired Behavior

1. **Step 1 — sync-progress as an in-plugin sibling.** The command's first step invokes `vault-cli:sync-progress` the same way it did from `~/.claude/commands/`, except the call is now a same-plugin sibling reference rather than a cross-install one — no new fallback or degradation logic is needed because the skill is now guaranteed present (it ships in the same plugin bundle).
2. **Step 2 — goal/task sweep, unchanged.** The sweep-every-touched-goal-and-task step carries over verbatim: confirm each touched page's Status Summary / Progress / Next line and checkboxes reflect reality.
3. **Step 3 — compact-safety checks, unchanged in full.** Git status + un-pushed-commit check, live background state (the `pgrep` / `dark-factory status` / `docker ps` mechanical checks, the stopped-daemon-does-not-mean-stopped-work warning, background shells/subagents/watchers), and unanswered-gates detection all carry over verbatim, including the exact shell snippet and its `|| echo "..."` fallbacks for absent tooling.
4. **Step 4 — resume block, unchanged.** The 4-field `RESUME AFTER COMPACT` block (Next action / Live background / Un-pushed-or-uncommitted / Open decision) carries over verbatim.
5. **Step 5 — per-session checkpoint file, unchanged.** The write to `~/.claude/compact-checkpoints/<session-id>.md`, its per-session (never shared-fixed-path) rationale, the session-id derivation from the scratchpad path, and the "outside every repo on purpose" rationale all carry over verbatim.
6. **Step 6 — verdict, unchanged.** Both verdict strings (`✅ Ready to compact` / `⚠️ Not compact-safe yet`) and their trigger conditions carry over verbatim.
7. **`allowed-tools` becomes a granular per-capability list.** Replace the source's bare `Bash` entry with this exact set — the concrete answer to "which style," not a placeholder:
   ```yaml
   allowed-tools:
     - Skill
     - Read
     - Write
     - Glob
     - Bash(git status:*)
     - Bash(git log:*)
     - Bash(git rev-parse:*)
     - Bash(pgrep:*)
     - Bash(dark-factory status:*)
     - Bash(docker ps:*)
     - Bash(mkdir:*)
   ```
   `Write` covers the step-5 checkpoint file; `Bash(mkdir:*)` covers creating `~/.claude/compact-checkpoints/` if absent. No other Bash capability is granted — the command cannot commit, push, or kill anything even if its prose were misread, because the tool surface does not permit it.
8. **Namespace self-references and packaging.** Every place the source command refers to `/vault-cli:session-close` (the continuing-vs-ending contrast) is confirmed to already read correctly as a same-plugin sibling reference (no path changes needed — it was already namespaced in the source). `CHANGELOG.md` gets an `## Unreleased` bullet naming the new command, per this repo's rule that any change to `commands/` requires a plugin version bump.

## Constraints

- Must not modify `~/.claude/commands/prepare-compact.md` or any other path outside this repository — the fate of the local duplicate is explicitly out of scope (see Non-goals).
- Must not widen `allowed-tools` beyond the list in Desired Behavior 7 — no bare `Bash`, no `Edit`, no `AskUserQuestion` (the source command never asks; it reports and writes).
- Must not change the 4-field resume-block schema, the checkpoint path template, or the verdict wording — these are frozen by the source spec of the original command's behavior contract.
- `make precommit` must stay green; no other `commands/*.md` file changes.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| `allowed-tools` omits a Bash capability the body invokes (e.g. missing `Bash(pgrep:*)`) | Claude Code blocks that tool call mid-run; the compact-safety check for that item is skipped or errors | Fix the frontmatter entry (see Desired Behavior 7's exact list), re-run `make precommit`, ship a follow-up release |
| Colleague's machine has neither `dark-factory` nor `docker` installed | `pgrep`, `dark-factory status`, `docker ps` all fail; each falls back to its `\|\| echo "no daemon"` / `\|\| echo "no containers"` text — the checklist completes and reports those tools as absent, never aborts | None needed — this is the correct, designed behavior; if the checklist aborts instead, that is a regression to fix before merge |
| Colleague has not run `claude plugin update vault-cli@vault-cli` after this ships | `/vault-cli:prepare-compact` is "command not found" | Colleague runs `claude plugin update vault-cli@vault-cli`, restarts Claude Code |
| Two Claude Code sessions in the same project both run `/vault-cli:prepare-compact` concurrently | Each writes its own `<session-id>.md` — no collision, because session-id is derived per-session from the scratchpad path | None needed if the per-session path template (Desired Behavior 5) is intact; if a regression collapses it to a shared filename, the second write clobbers the first — fix by restoring the `<session-id>` path segment |
| Session crashes/compacts between step 1 (sync-progress) and step 5 (checkpoint write) | Vault progress from steps 1–2 is already durable (written before the crash point); only the ephemeral checkpoint file is missing for that run | Re-run `/vault-cli:prepare-compact` — step 5 overwrites the checkpoint file per session, so a re-run produces a complete file with no manual cleanup |
| `CHANGELOG.md`'s `## Unreleased` section already has entries when this ships | New bullet appends alongside existing ones, not a replacement | None needed — this is normal `autoRelease: true` behavior; the release folds all `## Unreleased` bullets into the next `## vX.Y.Z` together |

## Security / Abuse Cases

- **Prompt-injection-driven tool abuse.** The command reads git output, task/goal page content, and (via the sync-progress skill) conversation history — any of which could contain adversarial text if a page was edited by an untrusted source. The granular `allowed-tools` list (Desired Behavior 7) is the primary defense: even if hostile text in a read file tried to steer the model into running an arbitrary command, the tool surface has no unscoped `Bash` entry to exploit — only the 6 explicitly-scoped read-only/status subcommands plus `mkdir` and file writes to the fixed checkpoint path.
- **Session-id trust boundary.** The checkpoint file path (`~/.claude/compact-checkpoints/<session-id>.md`) derives `<session-id>` from the environment's own scratchpad path, never from conversation text or user input — this must not change, since a user- or file-controlled session-id would allow writing to an arbitrary filename.
- **No credential exposure.** None of the six steps read or transmit credentials; `git status`/`git log` output and `dark-factory status` output are the only external command outputs surfaced, and both are already local-repo metadata (branch names, commit subjects, queue counts) with no secret material by construction.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Port `commands/prepare-compact.md` verbatim (steps 1–6), granular `allowed-tools`, namespace fit, `CHANGELOG.md` bullet | 1–8 | all | — |

Rationale: this is a single-file, single-layer change (one new file under `commands/`, one `CHANGELOG.md` bullet) — content is a faithful port of an already-written, already-battle-tested command, not new design. Splitting into multiple prompts would fragment one coherent file-write with no independent-deployability benefit; a single prompt is the correct decomposition here even though the spec exceeds 5 Desired Behaviors.

## Do-Nothing Option

`/prepare-compact` keeps existing only on the one machine where it was authored. Colleagues who want it must hand-copy the file into their own `~/.claude/commands/`, with no update path when the source improves — silent drift is the default outcome, not an edge case. Given its two closest siblings (`session-close`, `sync-progress`) already ship as plugin commands, leaving this one behind is inconsistent and loses exactly the compaction-safety net that makes the sibling commands valuable to begin with. The cost of doing nothing is ongoing manual-copy toil plus drift risk; the cost of doing this spec is one small, low-risk, single-file port.
