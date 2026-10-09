---
status: completed
spec: [058-bug-approve-status-removes-row-from-spawn-offer]
summary: Made task approve a phase-only transition by removing the SetStatus(next) write, adding the in_progress-preservation regression case, inverting the AC2 integration assertion, and correcting the plan-task doc and changelog.
execution_id: vault-cli-approve-status-exec-255-approve-phase-only
dark-factory-version: v0.196.0
created: "2026-10-09T00:00:00Z"
queued: "2026-10-09T15:35:19Z"
started: "2026-10-09T15:36:45Z"
completed: "2026-10-09T15:40:58Z"
---

# Stop task approve writing a status key

<summary>
- Approving a task stops changing its scheduling status
- Approval remains a phase move: `todo` becomes `planning`, and the approval record is still written
- A task that was queued stays queued; a task that was active stays active
- The approval's printed output and its `--output json` shape are unchanged
- Every refusal the command already performs still fires, and still writes nothing
- The repository's own documentation states the new behaviour, at every site that states it
- The changelog carries an Unreleased entry describing the correction
</summary>

<objective>
Make `vault-cli task approve` a phase-only transition, so that approving a task no longer rewrites its `status`. The scheduling axis and the workflow axis are orthogonal: a phase move that rewrites `status` makes an approved task disappear from the board and contradicts the vault's own vocabulary, in which `next` means *queued* and `in_progress` means *active*.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read `pkg/ops/task_approve.go` — the whole file. The approval write is the block near the end of `Execute`, currently guarded by the comment that begins "The status is next, not in_progress".

Read `pkg/ops/task_approve_test.go` — the assertion pinning the current status value.

Read `integration/cli_test.go` — search for `valueOf(after, "status")` to find the integration arms that pin the current value.

Read `docs/task-writing.md` — the approval paragraph under the phase-lifecycle section already states the target behaviour; do not edit it, it is the contract this prompt implements.

Read `commands/plan-task.md` § 3 — its `phase`-`planning` bullet still states the superseded behaviour and must be corrected.
</context>

<requirements>
1. In `pkg/ops/task_approve.go`, inside `Execute`, delete the `task.SetStatus(domain.TaskStatusNext)` call together with the comment lines that justify it — the `// The status is next, not in_progress:` paragraph. Keep the preceding paragraph about the single `WriteTask` and the map-preservation guarantee, and update its `All four keys` to `All three keys`. The remaining writes — `task.SetPhase(domain.TaskPhasePlanning.Ptr())`, `task.Set("approved_by", approvedBy)` and `task.Set("approved_at", now.Time())` — stay exactly as they are, and they must still travel in the single `WriteTask` call that follows them.

2. Do NOT touch any of the four refusal guards or the helpers around them: the empty-approver guard, the `phase != TaskPhaseTodo` guard, `refuseExistingApprovalRecord`, and `resolveOwner`. Their behaviour and their messages are frozen.

3. In `pkg/ops/task_approve_test.go`, the existing `Expect(written.Status()).To(Equal(domain.TaskStatusNext))` (whose fixture seeds `status: next`) stays valid — the write now preserves `next`. Add the missing direction: a fixture seeded at `status: in_progress` must still read `in_progress` after `Execute` succeeds. That second case is the regression this change exists to prevent. Its enclosing `It` name says "all four approval keys" and is now stale — rename it to say three. Rename the label only; every existing assertion in that block stays frozen.

4. In `integration/cli_test.go`, the AC 2 arm (the `inProgressFrontmatter` fixture, `~line 3635`) asserts `valueOf(after, "status")` equals `"next"`; invert it to `"in_progress"`, so the arm asserts the pre-approval status is preserved. The AC 1 arm (`todoFrontmatter`, `~line 3610`) is already correct and must NOT change — its fixture's pre-approval status is `next`, which is exactly what the write now preserves.

5. Add a `## Unreleased` section to `CHANGELOG.md` (immediately above the newest `## vX.Y.Z` heading) carrying one bullet prefixed `fix:`, describing the correction and naming the consequence it removes — an approved row no longer loses its place on the scheduling axis.

6. In `commands/plan-task.md` § 3, the `phase`-`planning` bullet opens with the claim that the approval leaves `status: next` — a claim the spec-058 change introduced and this reversal makes false. Replace that clause with the literal sentence `the approval does not touch \`status\`` — the `<verification>` grep matches this phrase verbatim, so any paraphrase fails it — and change nothing else in the bullet (the `vault-cli task set … status in_progress` promotion sentence is plan-task's own behaviour and stays as it is). This is the only remaining repository site that states the superseded value.

7. Do not bump any version string. The releaser owns the version, the tag, and the plugin manifests.

Before you finish: re-run `<verification>` and confirm it passes, then walk each of the seven requirements above against the change you made and confirm it holds.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT use absolute or home-relative paths.
- Existing tests must still pass; the suite is red until the write and its assertions move together, so land both in this one prompt.
- No new flag, no `--force`, no bypass.
- The command's stdout and its `--output json` shape are frozen.
- Tests follow repository convention: Ginkgo v2 / Gomega. The storage writer re-serializes frontmatter in alphabetical key order, so any new fixture must be authored alphabetically or byte comparisons become meaningless.
- Do not edit `docs/task-writing.md` — it already states the target behaviour.
- This is the WRITE half only, and it is not independently shippable. The paired sweep-side change (the `ready-to-start` predicate keyed on `phase: planning`, in both renderers plus the runbook) must be live first, and the orphan predicate must already stop claiming approved-but-unstarted rows — otherwise removing this write removes every approved row from the spawn offer, which is the bug the spec it reverses was written to fix. Do not release this change standalone.
</constraints>

<verification>
Run `make precommit` — must pass.

Then confirm the write path directly, from the source:

```
! grep -q 'SetStatus(' pkg/ops/task_approve.go
```

must succeed — a zero-match result is the pass, and the bare `SetStatus(` form also catches an accidental `TaskStatusInProgress` write, since the file carries no other caller. While

```
grep -n 'SetPhase(domain.TaskPhasePlanning' pkg/ops/task_approve.go
```

must still match.

Then confirm the changelog bullet sits under the Unreleased section — read the FIRST line only, since the file carries one `- fix:` line per released version:

```
awk '/^## /{sec=$0} /^- fix:/{print "sits under: " sec}' CHANGELOG.md | head -1
```

must print `sits under: ## Unreleased`.

Then confirm the plan-task doc no longer states the superseded value:

```
! grep -q 'the approval leaves `status: next`' commands/plan-task.md
grep -c 'the approval does not touch `status`' commands/plan-task.md
```

the first must succeed (zero match), the second must print `1`.
</verification>
