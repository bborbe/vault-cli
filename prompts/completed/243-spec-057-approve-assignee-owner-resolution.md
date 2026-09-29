---
status: completed
spec: [057-task-approve-assignee]
summary: Made `vault-cli task approve` resolve and write the task owner (flag, else existing assignee, else current_user) in the same single storage write as the approval, refusing unwritten when no owner can be resolved
execution_id: vault-cli-assignee-exec-243-spec-057-approve-assignee-owner-resolution
dark-factory-version: v0.196.0
created: "2026-09-28T22:04:01Z"
queued: "2026-09-28T22:23:56Z"
started: "2026-09-29T00:07:01Z"
completed: "2026-09-29T00:12:57Z"
---

# Approve fixes the task's owner (spec 057, prompt 1 of 2)

<summary>
- Approval becomes the one moment an owner is fixed on a task: `vault-cli task approve` no longer leaves a task ownerless.
- When a task has no assignee, approve fills it from the operator's configured current user, so a task can no longer enter planning or execution with nobody holding it.
- When a task already carries an assignee, approve leaves it exactly as it is — no overwrite, no warning, no error.
- A new `--assignee <name>` flag lets the operator name a different owner, overriding both the empty and the already-set case.
- When no owner can be found — no flag, no existing assignee, no configured current user — approve refuses outright and writes nothing at all: no status change, no phase change, no approval record.
- The owner is written in the same single storage write that records the approval, so a task can never land in planning without its owner beside it.
- Clearing an assignee after approval still works, so an agent can still hand work back to the operator by clearing the field.
- The task-writing guide records the rule beside the existing "unclaimed inbox" and "park" paragraphs, so all three assignee states read as one contract.
</summary>

<objective>
Make the approval command the single place that decides who owns a task: `vault-cli task approve` resolves an owner (the `--assignee` flag, else the task's existing assignee, else the configured current user), refuses when none can be resolved, and writes the resolved owner in the same single storage write that moves the task `todo → planning`. This prompt covers spec 057 ACs 1, 2, 3, 4, 7 and 8, and Desired Behaviors 1, 2, 3, 4, 7 and 8. It is the precondition for prompt 2, whose gate warning describes the rule this prompt establishes.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully before writing anything — the new code must be recognisably the same shape as its siblings:

- `pkg/ops/task_approve.go` — the whole file. This is the file you are changing. Note the guard order (`approved_by` empty → `FindTaskByName` → clock → phase-`todo` → existing approval record → compose four keys → one `WriteTask`) and the two conventions every refusal follows: `errors.Errorf(ctx, ...)` and `MutationResult{Success: false, Error: err.Error()}` returned alongside the error.
- `pkg/ops/workon.go` — lines 216-238 only, `applyAssigneeMatrix`. It is the repository's existing assignee rule (`blank → set`, `equal → no-op`, `different → preserve + warning`). This prompt does **not** change it (spec Non-goal), but read it so the two rules do not contradict: `work-on`'s matrix is unchanged, and this prompt's approve rule is a different, once-only decision.
- `pkg/domain/task_frontmatter.go` — `Assignee()` (returns `GetString("assignee")`) and `SetAssignee(v string)`. `domain.Task` embeds `TaskFrontmatter`, so both are promoted onto the task returned by `FindTaskByName`.
- `pkg/cli/cli.go` — `createTaskApproveCommand` (search for it; it builds `task approve`). Read the whole function, plus `createWorkOnCommand` (search `Use: "work-on <task-name>"`) for the established pattern of resolving the current user at the CLI layer: `currentUser, err := (*configLoader).GetCurrentUser(ctx)`.
- `pkg/config/config.go` — the `Loader` interface (`GetCurrentUser(ctx context.Context) (string, error)`) and `GetCurrentUser`'s implementation, which returns an error whose message is `current_user not configured` when the config names no user. The `Config` struct field is `CurrentUser string` with yaml tag `current_user`.
- `pkg/ops/complete.go` — lines 65-80 only, the `MutationResult` struct. Note the `Warnings []string` field exists; this prompt does **not** use it (the keep case is silent — spec Failure Modes).
- `pkg/ops/task_approve_test.go` — the whole file. Every existing spec is yours to keep green; the `Execute` call sites there all gain two arguments. Copy its idiom: external `ops_test` package, `mockStorage := &mocks.Storage{}`, a local `seedTask(fields map[string]any)` helper, and a pinned clock via `libtime.NewCurrentDateTime()` + `SetNow(libtimetest.ParseDateTime(...))`.
- `integration/cli_test.go` — the `Describe("task approve", ...)` block (search for it) in full, plus the helpers `createTempVault` and `createTempVaultWithCurrentUser` near the top of the file. `createTempVault` writes a config with **no** `current_user`; `createTempVaultWithCurrentUser` writes `current_user: tester@example.com`. Which helper each approve spec uses is load-bearing for this prompt — see requirement 7.
- `docs/task-writing.md` — lines 99-113 only, the `assignee` semantics section (the "unclaimed inbox" paragraph, the "two different meanings depending on how it became empty" paragraph, and the `task work-on` three-case matrix table). You amend this section; you do not rewrite it.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf(ctx, ...)` / `errors.Wrap(ctx, err, ...)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — Cobra command construction and `Flags().StringVar` registration.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, `DescribeTable` / `Entry`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — counterfeiter mocks are generated, never hand-written.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs, and `make generate`'s role in it.
</context>

<requirements>
1. **Extend the operation interface in `pkg/ops/task_approve.go`.** Two parameters are added to `TaskApproveOperation.Execute`, after `approvedBy`, and the doc comment is updated to describe the new owner rule:

   ```go
   Execute(
       ctx context.Context,
       vaultPath string,
       taskName string,
       vaultName string,
       approvedBy string,
       assignee string,
       currentUser string,
   ) (MutationResult, error)
   ```

   Semantics of the two new parameters, which the doc comment must state:
   - `assignee` — the value of the `--assignee` flag; the empty string means "flag not given" (spec Assumption: `--assignee ""` is treated as no flag, never as a request for an unowned task).
   - `currentUser` — the value of `vault-cli config current-user`; the empty string means "no current user could be resolved".

2. **Resolve the owner inside `Execute`, after every existing guard and before any mutation.** The resolution precedence is exactly the spec's Desired Behavior 1:

   1. `assignee` when non-empty (the flag always wins);
   2. otherwise `task.Assignee()` when non-empty (an existing owner is preserved);
   3. otherwise `currentUser` when non-empty (the config fallback);
   4. otherwise refuse.

   The resolved value is assigned to a local `owner`, so requirement 3's `if owner != task.Assignee()` reads as one snippet.

   **Placement is load-bearing, not stylistic.** The new refusal must come *after* the four existing guards (`approved_by` empty, clock, phase-`todo`, existing approval record) and *before* `task.SetStatus` / `SetPhase` / `Set` / `WriteTask`. The existing guard messages are pinned by the existing integration specs (`AC4` asserts a `planning` row's stderr contains `planning` and `todo`; `AC6` asserts an approval-record row's stderr names the present key; `AC2b` asserts an empty `--by`'s stderr contains `approved_by`) — those fixtures carry no assignee and their vaults name no `current_user`, so an owner refusal placed *first* would hijack their messages and break them. Constraint from spec: the existing refusals keep their current behaviour and messages.

   The refusal itself follows the file's existing convention exactly:

   ```go
   err := errors.Errorf(
       ctx,
       "assignee must not be empty: task %q has no assignee, no --assignee was given, and no current_user is configured; refusing to approve an unowned task",
       taskName,
   )
   return MutationResult{Success: false, Error: err.Error()}, err
   ```

   The literal `assignee` in that message is load-bearing — spec AC 4 asserts the refusal's stderr contains `assignee` and the phase is unchanged. Nothing is written on this path: no `SetStatus`, no `SetPhase`, no `Set`, no `WriteTask` (spec Desired Behavior 4).

3. **Write the owner in the same single write, and only when it changes.** In the existing compose block (the one that already calls `SetStatus`, `SetPhase`, `Set("approved_by", ...)`, `Set("approved_at", ...)` and then `WriteTask` once), set the assignee **only when the resolved owner differs from `task.Assignee()`**:

   ```go
   if owner != task.Assignee() {
       task.SetAssignee(owner)
   }
   ```

   Spec Desired Behavior 2 requires the owner to travel in the same single `WriteTask` that records the approval; spec Desired Behavior 3 requires that a task already carrying the owner the flag or config would produce is not dirtied by an assignee write. There is still exactly one `WriteTask` call, after all keys are on the map — no second write, no write on any refusal path.

4. **Do not emit a warning on approve.** Spec Failure Modes: "Task already has a different assignee → Preserve it; no warning, no error". Do not append to `MutationResult.Warnings` and do not add a warning path anywhere in this operation.

5. **Add the `--assignee` flag and the current-user resolution in `pkg/cli/cli.go`.** In `createTaskApproveCommand`:
   - declare `var assigneeFlag string` beside the existing `var approvedBy string`;
   - resolve the current user once, before the vault dispatch, and **treat an unresolvable user as an empty string rather than a fatal error**:

     ```go
     // An unresolvable current_user is not fatal here: the operation refuses only
     // when neither --assignee nor the task's existing assignee names an owner, so
     // a task that already carries one stays approvable on a host with no
     // current_user configured. Passing "" lets that keep case proceed.
     currentUser, err := (*configLoader).GetCurrentUser(ctx)
     if err != nil {
         currentUser = ""
     }
     ```

     This differs deliberately from the `work-on` commands, which return the `GetCurrentUser` error and abort. If you returned it here, spec AC 2 (approve keeps an existing assignee) would fail on any host without a configured current user.
   - pass both new values at the existing call site: `approveOp.Execute(ctx, vault.Path, taskName, vault.Name, approvedBy, assigneeFlag, currentUser)`.
   - register the flag beside the existing `--by` registration:
     ```go
     cmd.Flags().StringVar(&assigneeFlag, "assignee", "", "Set the task's assignee; overrides an empty or existing assignee (defaults to the configured current user)")
     ```
   - change nothing else in the command: the JSON branch, the plain-output branch, and the non-zero exit on refusal all keep their current behaviour.

6. **Regenerate the counterfeiter mock.** Run `make generate` (do not hand-edit `mocks/task-approve-operation.go`). The annotation already exists above the interface, so the regenerated mock gains the two arguments and its `ExecuteStub`, `ExecuteArgsForCall` and `executeArgsForCall` struct grow accordingly. ⚠️ `make generate` does `rm -rf mocks` and then `echo "package mocks" > mocks/mocks.go`, which strips the four-line copyright header from `mocks/mocks.go` — a pre-existing generator defect, not yours to fix. Restore the four lines with an Edit as the **last** action, after the final `make precommit` — do not shell out to `git checkout`, which dies with `fatal: not a git repository` in this project (see `<verification>`).

7. **Update the existing specs — every `Execute` call site gains the two arguments, and the approve block's success fixtures must name an owner.**

   **`pkg/ops/task_approve_test.go`:** the existing specs that approve the default fixture now need a resolvable owner, so change the `BeforeEach` seed from `seedTask(map[string]any{"status": "next", "phase": "todo"})` to `seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone"})`, and give every existing `approveOp.Execute(...)` / `zeroOp.Execute(...)` call `, "", ""` (no flag, no current user). With the seed carrying `someone`, the owner resolves from the task and the five specs that would otherwise be refused by the new guard — `writes all four approval keys in exactly one write` (~line 54), `records the injected instant as a bare time.Time` (~69), `serializes approved_at unquoted and round-trips it as a time.Time` (~79), `writes the approver named on the call` (~96), `wraps a write failure` (~204) — stay green unchanged. Every new ownership spec in requirement 8 must seed its own map explicitly (including `"phase": "todo"`), because the default seed now carries an assignee.

   **`integration/cli_test.go`, `Describe("task approve", ...)`:** the specs whose fixture carries **no** assignee and which expect a *successful* approve now need a resolvable owner, because `createTempVault` writes a config with no `current_user`. Switch exactly these specs from `createTempVault(...)` to `createTempVaultWithCurrentUser(...)`:
   - `AC1: task approve records the four-key transition in one write`
   - `AC2a: task approve --by records the named approver`
   - `AC9a: the default plain output names the task and the new phase`
   - `AC9b: --output json carries the task name and the new phase`
   - `security: a newline in --by cannot introduce a sibling frontmatter key`

   Do **not** switch the refusal specs (`AC2b`, `AC4`, `AC4 (nil phase)`, `AC5`, `AC6`, `AC9c`) or the `AC3` spec: their fixtures are refused by an earlier guard, or (`AC3`, `todoWithUnrelatedKeys`) already carry `assignee: someone` and are correctly owned without a configured current user. Adding a current user to those would make them stop testing what they were written to test.

8. **Add the new coverage.** Spec AC 1-4 and AC 7.

   **Unit level (`pkg/ops/task_approve_test.go`), each as its own spec:**
   - **AC 1 — fill.** Seed a `todo` row with no `assignee`; `Execute(..., "operator", "", "bborbe")` succeeds and the task handed to `WriteTaskArgsForCall(0)` has `Assignee() == "bborbe"`.
   - **AC 2 — keep.** Seed a `todo` row with `assignee: someone-else`; `Execute(..., "operator", "", "bborbe")` succeeds and the written task still has `Assignee() == "someone-else"`.
   - **AC 3 — flag overrides an empty assignee.** Seed no `assignee`; `Execute(..., "operator", "X", "bborbe")` writes `X`.
   - **AC 3 — flag overrides an existing assignee.** Seed `assignee: someone-else`; `Execute(..., "operator", "X", "bborbe")` writes `X`.
   - **AC 4 — refuse with zero writes.** Seed no `assignee`; `Execute(..., "operator", "", "")` returns an error whose message contains `assignee`, and `mockStorage.WriteTaskCallCount()` is `0` while `FindTaskByNameCallCount()` is `1` (the guard runs after the read, so the read is expected).

   **Integration level (`integration/cli_test.go`, inside `Describe("task approve", ...)`):**
   - **AC 1.** Extend the existing AC1 spec: after the successful approve, assert the file's `assignee` line equals `tester@example.com` (the `createTempVaultWithCurrentUser` config's `current_user`), using the block's existing `valueOf(content, key)` helper.
   - **AC 2.** New fixture with `assignee: someone-else` and `phase: todo`, approved with no `--assignee`: exit 0 and `valueOf(after, "assignee") == "someone-else"`.
   - **AC 3 (two arms, two separate temp vaults).** Arm one: a fixture with no assignee, approved with `--assignee X` → `valueOf(after, "assignee") == "X"`. Arm two: a fixture pre-set to `assignee: someone-else`, approved with `--assignee X` → `valueOf(after, "assignee") == "X"`. Two separate vaults are required — after the first approve the row is at `planning` and a second approve would be refused by the phase guard.
   - **AC 4.** A fixture with no assignee at `phase: todo` and no approval record (otherwise an earlier guard fires and stderr will not contain `assignee`), a config with no `current_user` (i.e. `createTempVault`), and no `--assignee`: exit code `1`; stderr contains `assignee`; `valueOf(after, "phase") == "todo"`; and the file hash is unchanged (the block already has `sha256OfFile`).
   - **AC 7 — clearing after approval still works.** Approve a task (it lands at `phase: planning`), then run `vault-cli task set "<name>" assignee ""`: exit 0, and `frontmatterOf(taskFile)["assignee"] == ""`. Do not use `valueOf` here: `task set assignee ""` leaves the key present with an empty string, which `yaml.Marshal` renders as the line `assignee: ""`, so `valueOf` returns the two-character string `""`, not the empty string. This is a regression lock, not a behaviour change — `task set` is untouched by this prompt — and it fails if a future reader of this rule is tempted to make approval write a value that cannot be cleared.
   - **`--assignee` newline boundary.** Mirror the `--by` security spec: approve with `--assignee "line1\nline2"` and assert the frontmatter carries exactly one `assignee` line, `frontmatterOf(taskFile)["assignee"]` is a string containing `line1` and `line2`, and `parsed` has no key `line2`. `--assignee` is a new free-text input on the same YAML path as `--by`, so it needs the same contract test.
   - **CLI-flag boundary.** Add an assertion that `task approve --help` output contains `--assignee`. The existing `registers the verb and prints its own help` spec is the natural home; `Entry("task approve", "task", "approve")` ALREADY EXISTS at `integration/cli_test.go:467` — do not add a duplicate; the flag must be discoverable through the real CLI, not merely registered in the struct (spec Constraints: a new CLI flag requires coverage in `integration/cli_test.go`'s command registration area).

9. **Amend `docs/task-writing.md`'s `assignee` semantics section.** Spec Desired Behavior 8 and AC 8. In the section that currently holds the "unclaimed inbox" paragraph (line ~101), the "two different meanings depending on how it became empty" paragraph (line ~103), and the `task work-on` three-case matrix (lines ~105-113), add the approve-time rule so the three states read as one contract:
   - **approval fixes the owner**: `vault-cli task approve` fills an empty `assignee` from `current_user` (or from `--assignee <name>` when the operator names someone else), keeps an already-set assignee, and refuses — writing nothing — when no owner can be resolved;
   - clearing after approval is unchanged: `vault-cli task set "<name>" assignee ""` still parks the task and still publishes the `agent-escalation` notification, because an empty assignee is the escalation channel.
   Do not contradict the existing `task work-on` matrix, and do not restate it — the matrix stays as it is.

10. **Do not add a CHANGELOG entry in this prompt.** Spec 057's decomposition assigns the `## Unreleased` bullets for **both** prompts to prompt 2. `make precommit` does not require a bullet (its `check-changelog` target verifies structure only), so the tree is green without one. Do not add a version bump either — see the constraints. Note that `## Unreleased` already exists with a bullet, and `docs/dod.md`'s "CHANGELOG has an entry" complaint about a missing new bullet is intentional here (the bullet lands in prompt 2); the releaser may cut prompt 1's change into a version before prompt 2's bullet lands.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage or commit. Note that in-container `git` does not work in this project directory (it is a git worktree, so `.git` is a `gitdir:` pointer file): any `git` command fails with `fatal: not a git repository`, and the daemon records a pass regardless. Use non-git checks.
- **`pkg/ops/` is a library layer: no stdout.** Operations return structured results and never write to stdout. Any non-fatal notice goes on `MutationResult.Warnings`; the CLI layer owns all output formatting. This prompt emits no warning at all (see requirement 4).
- **The refusal follows the file's existing convention** — the current `approved_by`-empty and phase guards both return `MutationResult{Success: false, Error: ...}` with a wrapped error, and the new refusal does the same. Never a bare `return err`.
- **No `fmt.Errorf`.** Use `github.com/bborbe/errors` (`errors.Errorf`, `errors.Wrap`, `errors.Wrapf`) with a `ctx`.
- **The existing refusals keep their current behaviour and messages** — empty `approved_by`, zero clock, wrong phase, pre-existing approval record. Their integration specs assert the literal strings, and requirement 2's guard placement exists to keep them intact.
- **`task work-on`'s three-case assignee matrix is frozen.** `pkg/ops/workon.go` — including `applyAssigneeMatrix` — must be byte-identical to its committed state. (This pin is deliberately broader than the spec Non-goal, which names only the three-case matrix; the whole file is frozen so a future reader knows why.) Spec Non-goal: "Changing `task work-on`'s existing three-case assignee matrix" is out of scope.
- **`assignee` semantics in `docs/task-writing.md` are not redefined.** The unclaimed-inbox-at-creation and park-at-clear readings stay exactly as they are; this prompt only adds the approve-time rule beside them.
- **No validation of the assignee against a user list or an agent registry** (spec Non-goal). The value is written into a YAML scalar exactly as `task set <name> assignee <value>` already writes it. No new input path, no shell, no network.
- **No backfill** of `assignee` on tasks already past approval (spec Non-goal). Only the task being approved is touched.
- **Do not block at the planning → execution gate.** That gate is prompt 2's scope and warns, never blocks.
- **Tests are Ginkgo v2 / Gomega with Counterfeiter mocks.** The operation's interface signature changes, so its mock must be regenerated (`make generate`) — never hand-edit it.
- **Do not hand-bump the plugin version or tag.** This repo is `autoRelease: true` via `.maintainer.yaml`: the releaser owns the version bump and the tag, and the four version strings (`CHANGELOG.md`'s top `## vX.Y.Z`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` metadata and plugins[0]) must not be touched here. `make precommit` does not run `check-versions`; `make release-check` does, at release time.
- **Scope is `pkg/ops/task_approve.go`, `pkg/ops/task_approve_test.go`, `pkg/cli/cli.go`, `mocks/task-approve-operation.go`, `integration/cli_test.go` and `docs/task-writing.md`.** `commands/execute-task.md` and `CHANGELOG.md` are prompt 2's.
- Existing tests must still pass, including every spec in `pkg/ops/task_approve_test.go` and `integration/cli_test.go`.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Run each and confirm against the expectation. Absence rows are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -n 'currentUser' pkg/ops/task_approve.go                 # >= 2 (the parameter and the resolution)
grep -n 'assignee' pkg/ops/task_approve.go                    # >= 3 (the parameter, the precedence condition, the refusal message; grep is case-sensitive, so SetAssignee / Assignee do NOT match)
grep -c 'SetAssignee' pkg/ops/task_approve.go                 # >= 1 (the conditional write)
grep -c 'WriteTask(' pkg/ops/task_approve.go                  # exactly 1 (the single-write property is unchanged)
grep -c 'Warnings' pkg/ops/task_approve.go                    # 0 (write as: ! grep -q 'Warnings' pkg/ops/task_approve.go)
grep -n "Set the task's assignee" pkg/cli/cli.go              # >= 1 (the new flag's usage string)
grep -c 'GetCurrentUser' pkg/cli/cli.go                       # >= 5 (the four pre-existing call sites plus the new one)
grep -c 'arg7 string' mocks/task-approve-operation.go      # >= 1 (the regenerated arg struct grows to arg1..arg7; counterfeiter names fields argN, not after the interface's parameters)
grep -c 'approval fixes the owner' docs/task-writing.md       # exactly 1 (requirement 9 must use this literal so the gate is not vacuous — the section already contains 8 'assignee' lines and 4 'task approve' lines)
grep -c 'createTempVaultWithCurrentUser' integration/cli_test.go  # >= 11 (6 pre-existing lines plus the 5 switched success specs)
```

FROZEN-NEIGHBOUR GUARD — content pins, not `git diff`. In-container `git` fails here (the repo is a git worktree whose `.git` is a `gitdir:` pointer to a host path outside the mount), and the daemon records a pass regardless of exit code, so a `git diff` guard would be a false-positive pass. These pins hold under `workflow: direct` and under `--set hideGit=true` alike:
```
sha256sum pkg/ops/workon.go      # must print 05f4bcf35db92f6de2333d0f5d2ae968a45d2cafebf90039fc692e97279eff3f
sha256sum pkg/ops/frontmatter.go # must print 1844869b8762b7683fbae787fadc31523fb3a0132faab15f06cc5b5b448d4152
```

TESTS:
```
go test ./pkg/ops/...        # exits 0
go test ./pkg/domain/...     # exits 0 — unchanged, but this prompt must not break it
make test                    # exits 0 — includes the integration suite, which builds the CLI
```

⚠️ **Guard the ops run against a false pass.** After `go test ./pkg/ops/...`, confirm the new specs actually ran — `go test ./pkg/ops/ -ginkgo.focus="TaskApproveOperation" -ginkgo.v` and grep the log for the new spec names (the fill / keep / override / refuse cases). A focus run that matches nothing exits 0 while running nothing.

⚠️ **Guard the integration run against a false pass.** `make test` runs `integration/cli_test.go`, which needs the CLI binary; if the suite was skipped rather than run, a green result proves nothing. Confirm the new integration spec names appear in the run's output.

SYNTAX:
```
gofmt -e -l pkg/ops/task_approve.go pkg/ops/task_approve_test.go pkg/cli/cli.go   # must list NO files
```

FULL GATE — `make precommit` at the repo root must exit 0. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make vet`, `make check-changelog`, ...), then `make precommit` once more. `make precommit` does **not** run `check-versions` (release-time only), so the four version strings stay untouched.

⚠️ **`make precommit`'s `generate` target re-runs `rm -rf mocks` + `echo "package mocks" > mocks/mocks.go`, which strips the copyright header from `mocks/mocks.go`.** The prerequisite list is `ensure format generate test check addlicense`, so `addlicense` normally re-adds it — do not rely on that. After the final `make precommit`, confirm the four-line header is back (this works whether or not `.git` is present, so it is safe under `--set hideGit=true`):
```
head -4 mocks/mocks.go | grep -q 'Copyright (c) 2026 Benjamin Borbe' && echo header-ok
```
If it is missing, restore the four lines with an Edit (do not shell out to `git checkout` — it dies with `fatal: not a git repository` when the run is started with `--set hideGit=true`, and the daemon records a pass regardless).

SELF-CHECK before finishing: re-run the PRIMARY GATE greps and the FULL GATE, and walk each of spec 057's ACs 1, 2, 3, 4, 7 and 8 against the change — AC 1 fill, AC 2 keep, AC 3 flag override (both arms), AC 4 refusal with an unchanged phase, AC 7 clear-after-approval, AC 8 the documented rule plus a green tree. Confirm `pkg/ops/workon.go` is untouched.
</verification>
