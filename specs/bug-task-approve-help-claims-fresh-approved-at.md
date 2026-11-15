---
status: draft
kind: bug
created: "2026-11-15T00:00:00Z"
---

## Summary

- `vault-cli task approve --help` says it "moves a todo task to planning and records approved_by and approved_at in one write".
- On a row that already carries `approved_at`, the command **preserves** the existing timestamp — which is the documented contract, so the behaviour is correct and the help text is the defect.
- The sentence reads as a fresh timestamp on every approval. A reader who trusts it builds a flow keyed on a fresh `approved_at` and gets silence.
- Concretely: the supervisor's decision-park resume leg fires only when `approved_at` is later than the worker's last transcript turn, so a preserved timestamp means that leg can never fire — and the run reads as a mechanism failure rather than an approval-shaped one.
- The preservation behaviour is **already pinned** by a discriminating test (`pkg/ops/task_approve_test.go:236`); the unpinned artifact is the help text itself.
- End state: the help text names the preservation, and a test pins that wording.

## Problem

`pkg/cli/cli.go` (the `taskApproveCommand`'s `Long` field) reads:

```go
Long: "Approve a task in the operator's inbox: it moves a todo task to planning and " +
    "records approved_by and approved_at in one write.\n\n" +
```

The claim is literally true — the command does record both keys in one write — but it omits the branch that matters. `docs/task-writing.md` (the approval paragraph, § Task lifecycle) states the contract explicitly:

> A row that already carries `approved_by` or `approved_at` is already approved, so the command does not re-approve it: it completes the transition by advancing `phase` to `planning` and leaves the existing record exactly as it is — it never overwrites either key, never fabricates a missing one.

So there are two behaviours under one sentence, and the help text describes only the first. A reader cannot distinguish "records them" from "records a fresh one", and the plausible reading is the wrong one.

## Assumptions

- **The preservation behaviour is already pinned, and the pin is discriminating.** `pkg/ops/task_approve_test.go:236` runs `DescribeTable("advances a todo task that already carries an approval record")`; it snapshots `approved_by`/`approved_at` *before* `Execute` (lines 239-244, with a comment explaining that reading `fields[...]` after the call would compare the written map against itself) and asserts `expectSameValue(written.Get("approved_at"), seededAt)` at line 267. Removing the preservation branch makes the written value the fresh timestamp and the assertion fails. Verified by reading the file 2026-11-15.
- **No test anywhere asserts the help text.** `pkg/cli/*_test.go` covers config, export, filename, output, resolve, rollup, answer-parse, backfill and watch — none touches a command's `Long`. That is the genuinely unpinned artifact.
- The `Long` string is built from two adjacent string literals, so a source-level `grep` for the sentence works only because Go concatenates them at compile time; asserting on the constructed command's `Long` is the robust form.

## Reproduction

Reproduced 2026-11-15 against `origin/master` on a disposable probe vault.

```bash
rm -rf /tmp/vc-approve-probe && mkdir -p "/tmp/vc-approve-probe/25 Tasks"
cat > /tmp/vc-approve-probe/config.yaml <<'EOF'
current_user: bborbe
default_vault: probe
vaults:
    probe:
        path: /tmp/vc-approve-probe
        name: probe
        tasks_dir: 25 Tasks
        goals_dir: 24 Goals
        topics_dir: 23 Topics
EOF
cat > "/tmp/vc-approve-probe/25 Tasks/Probe Re-Approval.md" <<'EOF'
---
page_type: task
status: next
phase: todo
approved_at: "2026-11-13T10:00:00+01:00"
approved_by: operator
assignee: bborbe
---
EOF

vault-cli --config /tmp/vc-approve-probe/config.yaml task approve "Probe Re-Approval"
vault-cli --config /tmp/vc-approve-probe/config.yaml task set "Probe Re-Approval" phase todo
vault-cli --config /tmp/vc-approve-probe/config.yaml task approve "Probe Re-Approval"
grep -E '^(approved_at|phase):' "/tmp/vc-approve-probe/25 Tasks/Probe Re-Approval.md"
vault-cli --config /tmp/vc-approve-probe/config.yaml task get "Probe Re-Approval" approved_at --output json
```

**Observed:**

```
✅ Approved Probe Re-Approval: phase planning
✅ Set phase=todo on: Probe Re-Approval
✅ Approved Probe Re-Approval: phase planning
approved_at: "2026-11-13T10:00:00+01:00"
phase: planning
{"key":"approved_at","name":"Probe Re-Approval","value":"2026-11-13T10:00:00+01:00"}
```

The phase advanced on both approvals; `approved_at` never moved. Both readings are identical.

**The help text, verbatim, at the same revision:**

```bash
go run . task approve --help | grep -i 'one write'
```

prints the "records approved_by and approved_at in one write" sentence, and:

```bash
go run . task approve --help | grep -i preserv
```

exits 1 with no output — the preservation is nowhere in the help text.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| Help text on re-approval | names that an existing `approved_at` is preserved | silent; describes only the first-approval write |
| Reader's inference | re-approval is not a timestamp refresh | re-approval produces a fresh timestamp |
| `approved_at` after a second approve | unchanged (contract) | unchanged (contract — correct) |

## Why this is a bug

The repository documents preservation as deliberate (`docs/task-writing.md`), and the help text contradicts the documentation it ships beside. A downstream consumer — the supervisor's decision-park resume leg — keys on ordering: `claude-supervisor/scripts/decision-park-resume.py` `approval_cleared()` returns `stamp > turn`, i.e. the approval must be later than the session's last turn. A preserved `approved_at` can never be later than the turn, so a flow built on the help text's promise produces nothing and reports nothing.

This is a user-visible message that is misleading, which `dark-factory/docs/bug-workflow.md` files as a bug. The behaviour is not the bug: correcting the behaviour instead of the text would break the documented contract.

## Goal

`vault-cli task approve --help` states that an existing `approved_at` is preserved on re-approval, so a reader cannot infer a fresh timestamp from it — and that wording is pinned by a test, so it cannot silently regress.

## Do-Nothing Option

Do nothing and the help text keeps teaching a fresh-timestamp contract. Every reader either builds a flow that silently never fires — as the decision-park resume leg's kind (a) trigger did — or has to discover the contract in `docs/task-writing.md`, which the `--help` output never points at. The cost is not a crash: it is a downstream mechanism that looks broken while behaving exactly as documented, which is the most expensive failure shape to diagnose. The fix is one string, so the do-nothing cost is paid on every future reader for a one-line change.

## Non-goals

- Changing the preservation behaviour. It is the documented contract.
- `docs/task-writing.md` — correct as written; it describes the first approval and the re-approval branch separately.
- Rewriting released `CHANGELOG.md` sections, `specs/` or `prompts/` — historical records.
- The decision-park resume leg's trigger design — followed up separately, not fixed here.

## Acceptance Criteria

- [ ] **AC1 — help text names the preservation.** The approve command's `Long` states that an existing `approved_at` is preserved on re-approval, naming `approved_at`.
  - evidence: a new test in `pkg/cli/` asserts the constructed command's `Long` contains the preservation wording and names `approved_at`; plus `go run . task approve --help | grep -i preserv` prints it. Pre-fix this grep exits 1 with no output.
- [ ] **AC2 — the misleading sentence no longer stands alone.** The unqualified sentence "records approved_by and approved_at in one write" is no longer the whole description of re-approval.
  - evidence: the test asserts the phrase is not the whole description of re-approval — i.e. **either** the phrase is absent, **or** a preservation clause naming `approved_at` appears in the same `Long` **at or after** the phrase (same sentence or a following sentence). Do **not** assert bare absence with `grep -q`, which would fail a correct qualify-in-place fix. Second observable: `go run . task approve --help` shows the phrase either gone or accompanied by the preservation wording.
- [ ] **AC3 — the existing preservation pin is untouched and still passes.** The behaviour is already pinned; this change adds no ops-layer test and removes none.
  - evidence: `pkg/ops/task_approve_test.go:236` (`DescribeTable("advances a todo task that already carries an approval record")`) still passes, and `git diff origin/master -- pkg/ops/task_approve_test.go` is empty.
- [ ] **AC4 — the write path is unchanged.** No behavioural change to the approve operation.
  - evidence: `git diff origin/master -- pkg/ops/task_approve.go` is empty after the change.
- [ ] **AC5 — `make precommit` green** and `CHANGELOG.md` carries a `## Unreleased` `fix:` bullet naming the help text.
  - evidence: `make precommit` exits 0; `awk '/^## /{sec=$0} /task approve --help/ {print sec}' CHANGELOG.md` prints `## Unreleased`.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `ROOTDIR=/workspace make precommit` — lint / format / generate / test / checks clean.
- `go test ./pkg/cli/...` — the new help-text test passes.
- `go test ./pkg/ops/...` — the existing preservation table still passes, unchanged.
- `go run . task approve --help | grep -i preserv` — prints the preservation wording naming `approved_at`.
- `git diff origin/master -- pkg/ops/task_approve.go pkg/ops/task_approve_test.go` — empty.

### Operator-executable (runs on the host after PR merge)

- Build the post-merge tree and run `vault-cli task approve --help | grep -i preserv` — see AC1.
- Re-run the reproduction above on the built binary and confirm the help text names the preservation while `approved_at` still does not move.

## Desired Behavior

1. **Help text states both branches.** The description says the command moves a `todo` task to `planning` and records `approved_by` and `approved_at`; and that a row already carrying `approved_at` keeps it — re-approval advances the phase without refreshing the timestamp.
2. **Behaviour unchanged.** Re-approval still preserves the first `approved_at` and never fabricates a missing one; the existing ops-layer pin keeps asserting it.
3. **Wording pinned.** A test fails if the preservation sentence is removed from the help text, so the correction cannot silently regress.

## Constraints

- **Do not change the preservation behaviour.** `pkg/ops/task_approve.go` is out of scope for behaviour; only the help string changes.
- **Do not edit or delete the existing preservation test.** `pkg/ops/task_approve_test.go:236` already pins the contract; leave it byte-identical.
- **Do not edit `docs/task-writing.md`.** It is correct as written.
- **Do not rewrite released `CHANGELOG.md` sections, `specs/` or `prompts/`.** Historical records.
- **No hand-bump of version strings or tags.** `.maintainer.yaml: release.autoRelease: true` — the releaser owns the tag and bumps all four version strings post-merge. Add a `## Unreleased` bullet and let it run. **The section does not exist today** (`CHANGELOG.md` top heading is `## v0.194.0`), so create it below the preamble and above the newest versioned section.
- **`pkg/ops/` stays output-free.** The help string lives in `pkg/cli/`.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Help text edited but the misleading sentence left unqualified | AC2 fails | qualify or remove the sentence |
| Preservation branch "fixed" to refresh the timestamp | AC3/AC4 fail; contract broken | revert the behaviour change |
| Help-text test asserts bare absence of the old phrase | a correct qualify-in-place fix fails | assert structure-aware per AC2 |
| `grep -c` used for an absence assertion | exits 1 on a correct file | use `! grep -q` |
