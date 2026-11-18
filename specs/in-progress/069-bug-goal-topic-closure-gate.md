---
status: approved
approved: "2026-11-18T19:46:38Z"
branch: dark-factory/bug-goal-topic-closure-gate
---

## Summary

- `vault-cli goal complete` reads no Success Criteria and no Definition of Done. A goal with an open criterion completes and is written `status: completed`.
- Its open-task guard is dead for bracketed links: a task whose `goals:` entry reads `[[Some Goal]]` never matches the goal being closed.
- `vault-cli topic complete` carries no gate at all — neither a Completion Gate check nor a member-goal terminality check.
- The strict gate `vault-cli task complete` already has is the model, and the same closure rule exists in another plugin's reader script but never reaches these two verbs.
- Result: a page whose own evidence is incomplete closes itself, and the vault's "every box must be `[x]`" forcing function does not hold at the goal and topic tiers.

## Problem

Every vault tier is supposed to close on its own recorded evidence. A task closes only when no box is open (`pkg/ops/complete.go`, `checkSubtaskCompletion`). A goal is supposed to close only when every `# Success Criteria` and `# Definition of Done` box reads `[x]` **and** no linked task is still open. A topic is supposed to close only when every `# Success Criteria` and `# Completion Gate` box reads `[x]` **and** every member goal is terminal.

None of the goal/topic half is implemented. `goal_complete.go` inspects linked task statuses and nothing else; `topic_complete.go` inspects nothing. The supervisor's `goal-closure-read.py` computes exactly this verdict, but it is a separate plugin's script — vault-cli cannot call it (the two are independently installed) and currently duplicates none of it.

Two existing behaviours this change also touches, stated here because the spec changes them:

- Both verbs **already refuse** an already-`completed` page with a non-zero exit (`goal_complete.go:73-76`, `topic_complete.go:52-55` — `already completed`). This spec turns that into an exit-0 no-op, so a closure loop that re-closes every tick is silent instead of erroring every tick.
- `aborted` is not guarded today at all; it becomes part of the same no-op.

**Cost of not fixing:** a goal or topic can be closed with open criteria, and the closure is silent — exit 0, a success line, `status: completed` written. Measured 2026-11-18 below.

## Reproduction

Environment: `vault-cli version v0.196.0` installed, vault `private-personal`, three scratch pages created for the probe and deleted after it. Code-level claims read at `bborbe/vault-cli@a4fa4de`.

**1 — `goal complete` ignores an open Success Criteria box.** Page had `status: in_progress`, `# Success Criteria` with `- [ ] an open criterion`, `# Definition of Done` with `- [x] done item`.

```
$ vault-cli goal complete "ZZ Scratch Closure Gate"
✅ Goal completed: ZZ Scratch Closure Gate
EXIT=0
$ grep -E '^status:' "24 Goals/ZZ Scratch Closure Gate.md"
status: completed
```

**2 — `goal complete` ignores an open task linked with brackets.** A task at `status: in_progress` with `goals:\n    - '[[ZZ Scratch Link Goal]]'`; the goal had every SC and DoD box `[x]`.

```
$ vault-cli goal complete "ZZ Scratch Link Goal"
✅ Goal completed: ZZ Scratch Link Goal
LINK_EXIT=0
```

The open-task guard did not fire. The same goal with the task linked as the bare name `ZZ Scratch Link Goal` refuses — that is the asymmetry `taskLinkedToGoal` produces.

**3 — `topic complete` has no gate.** Page had `status: in_progress`, an open `# Success Criteria` box and an open `# Completion Gate` box.

```
$ vault-cli topic complete "ZZ Scratch Closure Gate"
✅ Topic completed: ZZ Scratch Closure Gate
TOPIC_EXIT=0
```

**Code-level confirmation** (read at `a4fa4de`):

- `pkg/ops/goal_complete.go:78-82` — the only pre-write guard is `checkOpenTasks`.
- `pkg/ops/goal_complete.go:185-192` — `taskLinkedToGoal` compares `g == goalName` over the raw `goals:` entries.
- `pkg/ops/topic_complete.go:36-66` — `Execute` finds, sets `status`, writes. No gate, no `--force`.

## Expected vs Actual

**Expected** — per the vault's closure rule (`50 Knowledge Base/Goal Closure Checklist.md` § Forcing function: *"every box must be `[x]` — no 'trust me', no 'deferred'"*, and `claude-supervisor/commands/steps/goal-closure.md` § What to do), a close verb refuses while any closure box is open or any member is non-terminal, and names what is open. A goal and a task close on `# Success Criteria` + `# Definition of Done`; a topic closes on `# Success Criteria` + `# Completion Gate`.

**Actual** — `goal complete` reads neither section and its task guard misses bracketed links; `topic complete` reads nothing. Both exit 0 and write `status: completed`.

## Why this is a bug

`claude-supervisor/commands/steps/goal-closure.md:117-127` records the measurement this spec repeats and draws the conclusion explicitly: *"`vault-cli goal complete` re-checks **only** its open-task gate and reads no success criterion at all … a close it accepts is no evidence that any criterion was checked."* The supervisor therefore treats its own read as the authority and the verb as unable to disagree. That is a documented, accepted workaround for a missing gate, not the intended contract — and it holds only while every caller goes through the supervisor. A human running `vault-cli goal complete`, or any other tool, gets a close with no check.

The link-matching half is a plain defect: the vault writes `goals:` as wikilinks (`'[[Finished Work Closes Itself]]'`), and the comparison never strips the brackets.

## Assumptions

- The closure rule is already written down and does not need re-deciding: section sets, verdict names, the `2026-06-26` grandfathering cutoff, the strict Completion Gate and the fence-stripping rule all come from `claude-supervisor/scripts/goal-closure-read.py` and `claude-supervisor/commands/steps/goal-closure.md`. The Go package ports them; it does not re-derive them.
- `DOD_REQUIRED_AS_OF = "2026-06-26"` lives in **three** places: `claude-supervisor/scripts/goal-closure-read.py:73`, this repo's `agents/goal-auditor.md:118`, and `agents/task-auditor.md:328`. (The vault's own `.claude/agents/goal-auditor.md` is a different, thinner auditor and carries no copy — do not chase it.) The cutoff is not moving in this change, so none of those three is edited; the Go port adds a fourth copy, and all four move together only if the cutoff ever moves. vault-cli cannot import from any of them.
- Topic member goals are read from the topic page's `## Goals` list. The supervisor's step declares that list as the tracked set (`claude-supervisor/commands/steps/goal-closure.md` § Which pages this covers).
- The `## Goals` list is **level-relative**, exactly as `agents/topic-manager-agent.md:67` states: `## Goals` runs past `###` sub-headings and ends at the next `#` or `##`; `# Goals` ends at the next `#`. Both spellings occur in the vault — four topic pages use the H1 form — and bounding at any heading, or at `##` only, under-reads members and lets an incomplete topic close.
- The in-repo closure-box table by kind already exists at `agents/goal-manager-agent.md`; this spec's section sets must agree with it.
- Terminal means `status: completed` or `status: aborted`, matching `TERMINAL_STATUSES` in the Python reader.
- `--output json` is already the CLI's structured-output channel; a refusal must populate it rather than only printing.

## Goal

`vault-cli goal complete` and `vault-cli topic complete` refuse to close a page whose own closure evidence is incomplete, and say exactly what is open — through one shared Go implementation in `pkg/ops`, so the two verbs cannot drift. A page whose evidence is complete closes exactly as it does today, and a page that is already terminal is a silent no-op rather than an error.

## Non-goals

- `objective complete`, `theme complete` and `vision complete`. `pkg/ops/objective_complete.go` carries the same ungated close and `objective complete` is a shipped verb, so the gap is real there too — it is out of scope here and needs its own spec, because the section sets for those tiers are not defined anywhere yet.
- `--force` on `topic complete`, and any change to `goal complete`'s existing `--force` semantics.
- Retrofitting historical pages: no page is rewritten, only future close calls are gated.
- Verifying that a ticked box is truthful. The gate reads markers, not evidence.
- Editing any `claude-supervisor` prompt or the vault's closure guide.

## Acceptance Criteria

All CLI-level criteria below run at **build time** against the binary built from the branch (`go test ./integration` builds and drives it through `createTempVaultWithGoals` for goal cases and `createTempVaultWithTopicPages` for topic cases, both in `integration/cli_test.go`). The installed-binary replay lives in § Verification § Operator-executable.

- [ ] **Goal, open criterion refused.** An integration case drives `goal complete` on a temp-vault goal with one open `# Success Criteria` box and asserts: exit code non-zero, stdout carries that box's text, and the page still reads `status: in_progress`. Evidence: `go test ./integration -run TestIntegration -v` lists the case name and exits 0.
- [ ] **Goal, complete closes; both link forms guarded.** Three integration cases: (a) every SC and DoD box `[x]` and every linked task terminal → exit 0 and `status: completed`; (b) one open task linked as `[[<goal>]]` → non-zero exit naming that task; (c) the same task linked as the bare name → non-zero exit naming that task. Evidence: three case names in the `-v` listing.
- [ ] **Grandfathering is narrow.** Three integration cases: (a) no `# Definition of Done` section and `created` before `2026-06-26` → closes; (b) `# Definition of Done` heading present but holding no box → non-zero exit naming that section; (c) no DoD and `created` on or after the cutoff → non-zero exit. Evidence: three case names.
- [ ] **Topic gate.** Five integration cases: (a) one open `# Completion Gate` box → non-zero exit naming it; (b) one member goal not `completed`/`aborted` → non-zero exit naming that goal; (c) a member goal listed under a `###` sub-heading inside `## Goals`, non-terminal → still refused, so the bound is at the next `#`/`##` and not at any heading; (d) a topic whose member list is written as `# Goals` with a `##` sub-heading and a non-terminal member → still refused, so the bound is level-relative and not fixed at `##`; (e) SC and Completion Gate all `[x]` and every member goal terminal → exit 0 and `status: completed`. Evidence: five case names plus the written status.
- [ ] **Refusals enumerate, and JSON agrees.** An integration case on a page with two open items asserts the **plain** invocation's stdout carries one line per open item (two item lines), and that the same invocation with `--output json` yields `.open_items` whose length is 2. Evidence: `... --output json | jq '.open_items | length'` prints `2`, and the plain invocation's stdout line count equals it.
- [ ] **Terminal is a no-op.** An integration case runs each verb against an already-`completed` page and an `aborted` page: exit 0, and the file's `sha256` is identical before and after. Evidence: four cases, each asserting the digest.
- [ ] **One implementation, and the port is faithful.** Both verbs call one shared check in `pkg/ops`, named `checkClosureGate` — evidence: `grep -n 'checkClosureGate' pkg/ops/goal_complete.go pkg/ops/topic_complete.go` returns exactly one line per file, and `grep -cE '\[[ xX/-]\]' pkg/ops/goal_complete.go pkg/ops/topic_complete.go` returns `0` for each. A table test names at least these cases and passes: SC box open (OPEN), DoD section present-but-empty (UNMEASURABLE), DoD absent with `created` before the cutoff (CLOSE, grandfathered), DoD heading in a non-`#` spelling with no boxes (UNMEASURABLE, not grandfathered), a `*`-bulleted open box (OPEN), an unrecognised `-` or `?` marker (OPEN), a box inside a fenced block with no real box (UNMEASURABLE), an unterminated fence (UNMEASURABLE), a BOM-prefixed CRLF page (frontmatter still read), Completion Gate box open (OPEN), Completion Gate mixing a checkbox with an ordered-list item (UNMEASURABLE), member goal not terminal (OPEN), already-`completed` page (TERMINAL → exit 0 no-op). **Each case asserts its own expected verdict** — a case that merely runs is not evidence. Evidence: `go test ./pkg/ops -run TestSuite -ginkgo.focus=Closure -ginkgo.v` lists every one of those case names and exits 0.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0
- `make test` — exits 0
- `go test ./pkg/ops -run TestSuite -ginkgo.focus=Closure -ginkgo.v` — lists the table cases required by the last AC
- `go test ./integration -run TestIntegration -v` — lists the CLI cases required by AC1-6
- `go vet ./pkg/ops/... ./integration/...` — exits 0

### Operator-executable (runs on the host after the release is installed)

- `vault-cli --version` shows the released tag
- Replay the three probes from § Reproduction against that binary. The discriminating signal is the **verdict**, not the exit code alone: probes 1 and 3 must now print a refusal naming the open item and leave `status: in_progress`; probe 2 must refuse naming the bracketed-linked task.
- Re-run the positive half: a fully-ticked scratch goal and topic close with exit 0 and `status: completed`, and a second invocation on the now-`completed` page exits 0 with an unchanged file digest.

## Desired Behavior

1. `goal complete` and `topic complete` consult one shared closing check in `pkg/ops` before any write. No second copy of the rule exists in either verb.
2. Section sets are fixed by kind, never guessed from the page: goal and task → `# Success Criteria` + `# Definition of Done`; topic → `# Success Criteria` + `# Completion Gate`.
3. A section that is absent, or present with zero boxes, is unmeasurable and refuses the close. An empty section is never treated as vacuously complete.
4. A box counts as done only for the marker `x` or `X`. Every other marker — including ` `/`/`, and an unrecognised one such as `-` or `?` — counts as open. Bullets `-`, `*` and `+` all count.
5. Fenced code blocks are stripped over the whole document before sections are located; an unterminated fence refuses the close. A checkbox inside a fence is an example, never a box.
6. Grandfathering applies to **goals only**, to **absence only**: an absent `# Definition of Done` is accepted when the frontmatter parsed and `created` is absent or lexicographically earlier than `2026-06-26`. It is skipped entirely if any DoD-shaped heading exists anywhere on the page, in any spelling. An empty DoD heading stays unmeasurable.
7. The `# Completion Gate` is read strictly: a section carrying any list item that is not a checkbox is out of compliance and refuses the close.
8. Line endings are normalised and a BOM stripped before frontmatter is read, so the `status:` line cannot be lost. (This is an addition beyond the ported reader, which assumes `\A---\n` — it guards the same failure the reader documents for CRLF pages.)
9. A page already at `completed` or `aborted` exits 0 and is not written. This is a deliberate deviation from **both** baselines: the Python reader returns `TERMINAL → exit 1`, and vault-cli today returns an `already completed` error. Re-closing a finished page is a no-op, not an error, so a closure loop that ticks repeatedly stays silent.
10. A refusal exits 1, prints one line per open item, and populates `--output json` with the same items as an array under `open_items`.
11. Linked-task matching normalises both forms, so `[[Some Goal]]` and `Some Goal` in a task's `goals:` both count. The existing open-status set (`next`, `todo`, `in_progress`) is unchanged.
12. A topic refuses while any goal listed in its `## Goals` section — bounded at the next `#` or `##` heading — is not `completed` or `aborted`, naming that goal. **The bound is level-relative and covers both spellings**: `## Goals` runs past `###` sub-headings and ends at the next `#` or `##`; `# Goals` ends at the next `#`. Four live topic pages write the H1 form, so a reader that looks only for `## Goals` finds zero members there and closes them unexamined.

## Constraints

- **Do not change `task complete`'s behaviour.** Its whole-page box gate is already correct and is the model, not a target.
- **Do not shell out to, import, or depend on `claude-supervisor`.** The two plugins are installed independently and vault-cli must work when the other is absent.
- **The check is read-only.** It must never write, create or delete anything.
- **`pkg/ops` is a library layer** — it returns structured results and never writes to stdout; the CLI layer owns all output formatting (`CLAUDE.md` § Key Design Decisions).
- **The refusal's JSON field is `open_items`** (a `[]string` on `MutationResult`). The ACs and the jq probe both name it, so the prompt must not invent a different name.
- **Invert, do not delete, the two existing terminal tests.** `pkg/ops/goal_complete_test.go:88-96` and `pkg/ops/topic_complete_test.go:121-138` both assert an `already completed` error. DB9 inverts that behaviour, so these two contexts must be rewritten to assert exit-0/no-write — never removed.
- **Error wrapping** goes through `github.com/bborbe/errors`, never `fmt.Errorf`.
- **Do not hand-bump version manifests or tag.** `.maintainer.yaml` carries `autoRelease: true`; the releaser owns the tag post-merge. `.dark-factory.yaml` carries `autoRelease: false` for the container run.
- **Add a `## Unreleased` CHANGELOG bullet.**
- Existing `--force` semantics on `goal complete` are unchanged and stay narrow: `--force` bypasses **only** the open-task gate, never the closure gate — a forced close still cannot close a page with an open criterion. `topic complete` gains no `--force` in this change.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Page frontmatter does not parse | Refuse; never grandfather it (an unreadable page is not a page with no `created`) | Repair the frontmatter and re-run |
| Unterminated code fence | Refuse as unmeasurable | Close the fence |
| DoD heading written as `## Definition of Done`, a setext heading, or `- Definition of Done` | Treated as a present section for the grandfathering guard, so the exemption is not granted | Normalise the heading to `# Definition of Done` |
| `created` in a non-ISO shape (`15.01.2026`, `2026`) | Not grandfathered — refused rather than compared | Write `created` as a zero-padded ISO date |
| A topic with an empty `## Goals` list | Closes on its own SC + Gate boxes alone | None; a topic with no members is legitimate |
| A `## Goals` entry grouped under a `###` sub-heading | Still read — the list ends at the next `#` or `##`, not at any heading | None |
| A topic whose member list is written as `# Goals` with a `##` sub-heading | Still read to the next `#` — the bound is level-relative, not fixed at `##` | None |
| Concurrent vault write during the check | Check is read-only and takes its snapshot from the page it already loaded | Re-run the verb |
| Gate passes, then `WriteGoal`/`WriteTopic` fails | Non-zero exit and the page stays at `status: in_progress` — the write is the only mutation | Re-run the verb; the check is read-only so the retry is idempotent |
| Two goals share a name across folders | Existing name resolution applies unchanged | Not a new surface |

## Suggested Decomposition

**One prompt, deliberately.** The change is one behavioural rule — a page closes only on its own recorded evidence — expressed in one shared `pkg/ops` check and two call sites. Splitting it is unsafe for a pipeline reason rather than a design one: dark-factory assigns prompt numbers by sorting *unprefixed* filenames alphabetically and gates each prompt on every lower-numbered prompt of the same spec, so a four-row split cannot be made to sort in dependency order. Observed on spec 067 (2026-11-16): a three-row decomposition normalised to docs → fixtures → guard, the docs row failed on a verification that needed the guard's output, and the other two blocked behind it. A row that wires a call site before the shared check exists is that same deadlock.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | The shared closure check in `pkg/ops` (sections by kind, verdicts, grandfathering, strict gate, fence stripping, normalisation) plus its table test; wire it into `goal_complete.go` and `topic_complete.go`; fix link normalisation; add the topic member-goal check; wire the refusal output, the `open_items` field and `--output json`; the integration cases; the `## Unreleased` bullet | 1-12 | 1-7 | — |

*Scope note: DBs × ACs = 12 × 7 = 84, over the 50 split-threshold. Not split — the ordering constraint above makes a single prompt the only shape this pipeline executes correctly, and the bounded surface is one new file plus two call sites and their tests.*

## Do-Nothing Option

Doing nothing is not neutral, because the gap is already being papered over in one place. The supervisor's `goal-closure-read.py` computes the correct verdict and its closure step treats that read as the authority — so **supervisor-driven** closes are already correct, and staying as-is costs nothing there.

Every other caller gets the silent close this spec reproduces: a human running `vault-cli goal complete`, any script, any future tool, and the topic tier even under the supervisor (which has no topic-close reader). The two tiers then disagree with the third — `task complete` refuses on an open box while `goal complete` and `topic complete` do not — and the vault's own closure guide has to keep carrying a caveat that the CLI cannot be trusted to enforce its rule.

Doing nothing also leaves the link-matching defect in place, which silently disables the one guard that does exist. Cost of the fix is one shared check plus two call sites; cost of not fixing is that the forcing function holds at one tier out of three.
