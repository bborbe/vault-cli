---
status: verifying
approved: "2026-09-21T07:18:35Z"
generating: "2026-09-21T14:21:05Z"
prompted: "2026-09-21T14:21:05Z"
verifying: "2026-09-21T14:41:48Z"
branch: dark-factory/topic-phase-field
---

## Summary

- Topics gain a validated `phase` frontmatter field with exactly four values: `todo`, `planning`, `execution`, `done`.
- The field mirrors the shape of the goal-side `GoalPhase` type without touching or reusing the goal or task phase types.
- Setting an invalid phase on a topic fails loudly; a valid phase is written to the topic page's frontmatter and survives read-write cycles.
- Topic pages that predate this field keep parsing and operating with no error — no file is backfilled.
- This is the data-layer foundation only. The `plan-topic` / `execute-topic` gate commands that will consume the phase are explicitly out of scope — they are a separate task.

## Problem

Goals already carry a validated lifecycle `phase` (`todo → planning → execution → done`) that the plan/execute gate commands read to enforce a plan-before-execute workflow, and tasks carry the same idea one level down. Topics — the vault's cross-cutting domain pages under `23 Topics/` — have no equivalent field, so "Phase-Gated Topic Flow" has nothing to read or write at the topic level. A topic's readiness is inferred from prose by whoever happens to read the page.

The command surface is a separate, prior deliverable: `vault-cli topic set` / `topic show` / `topic get` do not exist today, and building them is [[Build the Vault-Cli Topic Command Ladder]]. That family gives a topic page a generic frontmatter read and a generic write path, which is enough to *surface* a phase already on disk — but a generic write cannot *validate* one. `GoalFrontmatter.SetField` ends in `default: f.Set(key, value)`, so an unrecognised key is written raw with no validation; a topic frontmatter taking that default would accept `phase: banana` and exit 0. Making the field real — typed, enumerable, and rejected when invalid — is this work.

## Goal

After this work, a topic page can carry a `phase` frontmatter field constrained to `todo` / `planning` / `execution` / `done`. Setting the field through the topic field-set command validates the value against that enum and persists it; reading the topic through the topic show command surfaces the value in both plain and JSON output. Topic pages with no `phase` field continue to parse, show, and mutate exactly as they do today. The goal-side and task-side phase types and every existing command behave identically to before.

## Non-goals

- Do NOT add `plan-topic` / `execute-topic` / any phase-transition or gating command — those are separate work that consumes this field, carried by [[Build the Vault-Cli Execute-Topic Gate Command]].
- Do NOT build the `vault-cli topic` command family itself — that is [[Build the Vault-Cli Topic Command Ladder]], a prerequisite. Phase rides the `topic set` / `topic get` / `topic show` commands that family ships.
- Do NOT backfill, rewrite, or migrate existing topic pages that lack a phase.
- Do NOT modify, extend, or reuse the goal-side `GoalPhase` type, the task-side `TaskPhase` type, their constants, or `NormalizeTaskPhase`.
- **Do NOT add `in_progress`, `ai_review`, or `human_review`.** The enum is **four values by decision (2026-09-20)**, mirroring the goal side — which chose the same subset deliberately, and the parent goal asks for the goal-level implementation mirrored exactly. Those three task-level values are excluded *pending a consumer*: the gate task builds no review-phase gates, so they would be dead values nothing reads. This is a decision, not an oversight. If a consumer appears, extending the enum is a separate spec.
- Do NOT add alias handling (e.g. an `in_progress` synonym) — the topic phase enum has no legacy values.
- Do NOT invent a "default to todo" read behavior — a missing phase is empty, not `todo`.
- Do NOT add or extend topic-specific status/phase mismatch lint rules in this work.
- Do NOT add a `docs/topic-writing.md` or any other doc-only change. The lifecycle semantics this spec fixes (four values, no backfill, lazy migration) belong in the vault's `Topic Writing Guide.md` — the topic-convention source the ladder spec already names — recorded there by whoever owns that page, not by this spec.

## Acceptance Criteria

- [ ] A topic-phase enum type declares canonical constants for `todo`, `planning`, `execution`, `done` and a matching `Available…` collection — evidence: `grep -nE '"todo"|"planning"|"execution"|"done"' pkg/domain/topic_phase.go` returns ≥4 lines.
- [ ] The topic-phase type rejects a non-canonical value and accepts each canonical value — evidence: a `DescribeTable` unit test covering the 4 canonical values plus ≥1 invalid value passes under `go test ./pkg/domain/...` (exit 0).
- [ ] `topic set <page> phase execution` on a real topic page writes `phase: execution` to that page's frontmatter — evidence: `git diff` (or file read) of the topic page shows an added `phase: execution` line. **This criterion alone does not discriminate this work** — the generic `SetField` default writes any key raw, so it passes on the ladder task's branch before this enum exists. The discriminating clauses are the refusal and the empty-clear below.
- [ ] **The falsifier.** `topic set <page> phase bogus` exits non-zero, names the offending value, and leaves the page unchanged — evidence: shell exit code ≠ 0; stderr contains **the domain validator's own wording**, `unknown topic phase 'bogus'` (not merely "some error naming the phase" — a CLI that re-lists the four strings inline would satisfy a looser phrasing while bypassing the validator); `git status` shows the topic page unmodified. **Backed by a test asserting the CLI refusal path reaches the domain validator**, so the refusal cannot be re-implemented as a second inline list. This cannot pass on the generic default path, which accepts any string and exits 0.
- [ ] `topic set <page> phase ""` removes the `phase:` line from the page — evidence: state transition, the page carried a `phase:` line before the command and carries none after, and `topic show <page> --output json` reports no phase value. (Covers the "empty is not invalid" constraint below.)
- [ ] A topic page with no `phase` key runs `topic show` and an unrelated `topic set` cleanly — evidence: both commands exit 0; `topic show --output json` contains no `phase` value; the round-tripped file still has no `phase:` line. ⚠️ Non-discriminating on its own — the generic read path already tolerates a missing key. Retained because lazy migration is a real regression the enum could introduce.
- [ ] A topic carrying a canonical phase alongside a **consistent** status produces zero `STATUS_PHASE_MISMATCH` lint issues — evidence: negative evidence, `vault-cli topic lint <page>` reports zero issues of that kind for a page with `phase: execution` + `status: in_progress`; and a page with `phase: done` + `status: in_progress` reports at least one, so the check is shown to fire rather than merely to be silent. (Covers the lint constraint below — the generic mismatch rule keys off the presence of a `phase:` line and will begin evaluating topics.)
- [ ] The goal-side and task-side phase type files are unchanged — evidence: `git diff --stat <baseline>..HEAD` names no changes to `pkg/domain/goal_phase.go`, `pkg/domain/task_phase.go`, or `pkg/domain/goal_frontmatter.go`, with `<baseline>` being the merge-base with `master` at prompt time.
- [ ] `make precommit` exits 0 in the repo root — evidence: exit code.
- [ ] A CHANGELOG entry for the topic phase field exists **under the `## Unreleased` heading specifically** — evidence: `awk '/^## /{sec=$0} /topic phase/{print sec}' CHANGELOG.md` prints `## Unreleased`. A bare `grep` does not assert the section: the bullet could pass folded under a released `## vX.Y.Z` heading, which is the failure the repo's changelog-fold guard exists to catch.

*(Moved — not this spec's criterion.)* `topic show <page> --output json` surfacing `.fields.phase`, and `topic get <page> phase` printing the value, both belong to [[Build the Vault-Cli Topic Command Ladder]], whose own Acceptance Criteria require those exact commands and whose generic read path serves them. Moved 2026-09-20 under §5 Axis A. **The discriminating clauses for this work are the refusal and the empty-clear above.**

Scenario coverage: NO new scenario. Unit tests (domain enum + frontmatter getter/setter) plus integration-level exercise of the existing topic set/get/show commands reach every behavior; no real Docker / cluster / external tool is involved.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```bash
make precommit                                                       # must exit 0
make build                                                           # builds to bin/vault-cli
./bin/vault-cli topic set <topic> phase execution                    # exit 0; page gains `phase: execution`
./bin/vault-cli topic set <topic> phase bogus                        # exit non-zero; stderr names `unknown topic phase 'bogus'`; page unchanged
./bin/vault-cli topic set <topic> phase ""                           # the `phase:` line is removed
./bin/vault-cli topic show <legacy-topic> --output json              # exit 0; no phase key in output
awk '/^## /{sec=$0} /topic phase/{print sec}' CHANGELOG.md           # expect `## Unreleased`
```

**Use the freshly built `./bin/vault-cli`, never the installed one.** The installed binary answers `unknown command "topic"` until the ladder releases, so a bare `vault-cli topic …` here silently exercises a stale install and proves nothing — the same shipped-≠-deployed trap this spec's own subject matter is about.

### Operator-executable (runs on the host after merge, spec verification ladder)

```bash
cd ~/.claude && make update && claude plugin update vault-cli@vault-cli   # install the new version
vault-cli topic set <topic> phase execution     # exit 0
vault-cli topic get <topic> phase               # prints `execution` — the LADDER's criterion, listed here only to confirm the write is readable
vault-cli topic set <topic> phase bogus         # exit non-zero; stderr says `unknown topic phase 'bogus'`; page unchanged  <-- the discriminating check
```

Steps 2 and 3 alone are satisfiable by the generic write path; **step 4 is the one that requires the typed enum.** Run it, not just the happy path.

## Desired Behavior

1. A topic page's phase value is constrained to exactly `todo` / `planning` / `execution` / `done` and rejects anything else. The type is member-for-member identical in shape to the goal-side phase type — frozen template: `pkg/domain/goal_phase.go`.
2. The topic frontmatter exposes a typed phase read that returns the parsed value when the key is present and an empty result when it is absent — no default substitution.
3. Setting the topic `phase` field to a canonical value through the topic field-set command writes `phase: <value>` into the page's frontmatter and preserves it through a read-write cycle.
4. Setting the topic `phase` field to any non-canonical value fails with a non-zero exit and an error naming the offending value, **produced by the same validator that defines the enum** — not by a second list of valid values living in the command layer. The page is left unchanged.
5. Setting the topic `phase` field to an empty value clears the key from the frontmatter rather than writing an empty one; only non-empty input is validated.
6. The existing topic show command surfaces the `phase` value in both plain output and `--output json` when the field is present.
7. A topic page with no `phase` field parses, shows, and accepts unrelated field mutations with no error, and its output contains no phase value.

## Constraints

- The goal-side phase type, the task-side phase type, `NormalizeTaskPhase`, and every goal and task command must be byte-for-byte unchanged. Frozen: existing goal-phase and task-phase behavior and their test suites.
- Frontmatter remains map-based; unknown keys must continue to survive read-write cycles. That map is the lazy-migration mechanism — no separate migration code.
- Follow the layered pattern in `docs/development-patterns.md` § Adding a New Command (Domain → Storage → Ops → CLI). The expected footprint is Domain-only; Storage, Ops, and CLI reuse the topic family's existing generic set/get/show wiring and require no new command.
- The existing generic `status/phase mismatch` lint keys off the presence of a `phase:` line and **will** begin evaluating topics that carry a phase. Concretely, against `pkg/ops/lint.go` `detectStatusPhaseMismatch`: `phase: execution` with `status: backlog` or `hold` is a mismatch; `phase: done` with a status other than `completed` is a mismatch; a `completed` or `aborted` status with a phase other than `done` is a mismatch. Legacy topics (no phase) must remain lint-clean, and this work must not add new false-positive lint output for the four canonical phases on an otherwise-consistent topic.
- `make precommit` must pass in the repo root.
- A `## Unreleased` CHANGELOG entry describing the new topic phase field is required.
- The enum's canonical set is defined once, as `AvailableTopicPhases`; `Validate` ranges over it via `collection.Contains`. An inline `switch` inside `Validate` is a lint failure (`go-enum-type-pattern.md` § validate-against-available-collection). A duplicated *slice* of the same four strings in another layer is the same defect wearing different clothes, which is why the refusal criterion names the validator's wording.
- Empty is not invalid: the field-set path clears the key on an empty value and validates only non-empty input.
- `Ptr()` exists for mirror-parity with the goal-side type, not because a consumer dereferences it today. Stated so a later reader does not mistake it for dead code to remove.

## Assumptions

- **The topic frontmatter type is created by [[Build the Vault-Cli Topic Command Ladder]], which has not landed.** This spec is written against the shape that task's own spec declares: a topic domain entity plus a typed frontmatter wrapper, following the same three-concern split as the existing entities. The paths are now pinned rather than guessed — the ladder's generated prompt 1 (`prompts/1-spec-051-topic-domain-entity.md`) declares `pkg/domain/topic_frontmatter.go` as NEW and explicitly reserves the typed phase accessor for this spec. So: **`pkg/domain/topic_phase.go`** for the new enum, and **`pkg/domain/topic_frontmatter.go`** for the getter/setter.

  ✅ **Approval gate — SATISFIED 2026-09-21, verified on disk.** The gate was: do not `dark-factory spec approve` until [[Build the Vault-Cli Topic Command Ladder]] is `completed` **and** `pkg/domain/topic_frontmatter.go` exists on `master`. Its stated hazard was a duplicate-file collision — approving early would have spawned a run creating that file while the ladder's prompt 1 was creating it on its own branch. **That hazard is gone:** the file is on `origin/master` at `189ea0b release v0.142.0` (blob `392027b`), the ladder's PR #190 merged, and the installed binary resolves `topic --help` and `topic list --vault Personal`. The second clause is satisfied; the first is moot — the ladder's *task record* still reads `in_progress`/`execution` at its `verifying` stage, and a record lagging the artifact is not a reason to hold a gate whose stated hazard no longer exists. **Residual risk, named rather than assumed away:** if the ladder's acceptance walk fails and it edits `topic_frontmatter.go` again, the paths above may shift — a one-line edit here, not a re-plan.

  **Blast radius, stated precisely:** a *path rename* by the ladder is survivable — only the paths above change, not the behavior, criteria, or decomposition. A ladder that ships **no typed wrapper at all** is not: the typed read has no home and the empty-clears rule has nothing to attach to, which is a structural revision of this spec, not a rename.

- **The enum is four values — settled 2026-09-20, not an open question.** The parent goal asks for the goal-level implementation mirrored exactly, and the goal spec chose the same four deliberately (`specs/completed/024-goal-phase-field.md` § Assumptions: *"The four goal phases are a deliberate subset of the seven task phases (goal has no `ai_review` / `human_review` / `in_progress`). This is intended, not an omission."*). Topics are the goal's parent level and share its lifecycle shape. The exclusion of the three task-only values is recorded as a Non-goal with its reason, so a later reader does not re-derive the fork.
- Relevant coding guides are available in-container: `go-enum-type-pattern.md`, `go-parse-pattern.md`, `go-cli-guide.md`, `go-testing-guide.md`.
- The topic field-set command is `topic set <page> <key> <value>` and the read surface is `topic get <page> <key>` plus `topic show <page>`, mirroring the goal command's argument order.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---------|-------------------|----------|-----------|---------------|
| `topic set <page> phase bogus` | Non-zero exit; error names the invalid phase in the validator's wording; file unchanged | Re-run with a canonical value | Command exit code + stderr message | Reversible (no write occurred) |
| `topic set <page> phase ""` | The `phase:` key is removed from the frontmatter | None needed | File read shows no `phase:` line | Reversible |
| Topic page has no `phase` key | Parses and operates normally; phase reads as empty | None needed | `topic show` exits 0 with no phase value | N/A |
| Topic page has a legacy/hand-typed `phase: in_progress` (not in the topic enum) | Reading tolerates the raw value in show output; validation on an explicit re-set rejects it | Set a canonical value | `topic show` displays the raw value; `topic set` rejects | Reversible |
| Topic carries `phase: execution` with `status: backlog` or `hold` | The existing generic mismatch lint flags it; core get/set/show still function | Fix status or phase to a consistent pair | `topic lint` output | Reversible |
| Concurrent `topic set phase` on the same file | Last writer wins (existing whole-file write semantics; no new locking introduced) | Re-read and re-set if clobbered | File content after both writes | Reversible |

## Security / Abuse Cases

N/A — the only user input is a phase value validated against a closed 4-value enum before any write; the write target is an existing named topic page resolved through the standard vault path and the topics directory. No path-traversal, injection, or unvalidated-input surface is introduced. Topic-name traversal refusal is owned by the ladder task's own security criteria, not this one.

## Suggested Decomposition

Single-layer (Domain-only) footprint — one prompt suffices. If the generator splits, this ordering holds. **"Covers ACs" counts the checkbox criteria only**, in the order they appear above; the non-checkbox Moved note is not an AC and is not counted.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Topic-phase enum type (`topic_phase.go`) + `Validate` + `DescribeTable` unit test | #1 | #1, #2 | — |
| 2 | Topic frontmatter typed read + field-set case wired into the generic `topic set`/`topic show` path, with the refusal routed through the validator | #2–#7 | #3, #4, #5, #6, #7 | 1 |
| 3 | CHANGELOG `## Unreleased` entry + verification pass | — | #8, #9, #10 | 2 |

## Do-Nothing Option

If we do nothing, "Phase-Gated Topic Flow" cannot begin — the gate commands would have no topic-level field to read or write, and would have to either invent an ad-hoc key (diverging from the goal-level convention) or track topic phase outside the vault files. The command family would ship with a `phase` key that looks settable but accepts any string, so a typo would write silently and the field's whole purpose — being a value both agent and operator can trust from frontmatter — would be defeated by the first bad write.
