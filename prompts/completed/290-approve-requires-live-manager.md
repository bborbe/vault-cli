---
status: completed
created: "2026-11-27T17:40:00Z"
queued: "2026-11-28T02:07:34Z"
completed: "2026-11-28T02:17:41Z"
pr-url: https://github.com/bborbe/vault-cli/pull/406
---

# Refuse a task approval whose goal has no live manager

<summary>
- With the gate enabled, `vault-cli task approve` refuses when the task's goal resolves no live manager, naming the goal.
- The gate is opt-in per config, so other vaults and the existing test suite are unchanged.
- A live manager is resolved through a resolver script that ships separately in `claude-supervisor`; its contract is stated here.
- An unreadable fleet refuses the approval instead of approving unowned work.
- A successful approval records the manager that satisfied the gate.
- The refusal happens before any write, so no partial approval lands.
</summary>

<objective>
Ship the capability to require a dispatcher: with `require_manager_on_approve` enabled, an approval may leave the inbox only when its goal has a live manager (an operator-flagged row advanced by `task work-on` is the operator's own dispatch and is exempt). Off by default so other vaults and the existing suite are unchanged. Enabling it for `private-personal` — and starting managers for the rows that resolve none — is an operator step outside this container, recorded on the task's DoD.
</objective>

<context>
Read these fully before changing anything:

- `pkg/ops/task_approve.go` — `TaskApproveOperation.Execute`, the inbox guard, `hasApprovalRecord`, `resolveOwner`. The refusal goes after the inbox guard and after the already-approved repair branch, and before `resolveOwner` and the write.
- `pkg/ops/task_approve_test.go` — the Ginkgo/Gomega + counterfeiter pattern to extend.
- `pkg/ops/workon.go` — `advancePhaseIfEntering` moves an operator-flagged `todo` row to `planning` without this operation. It is deliberately out of scope here (see requirement 3); do not change it.
- `pkg/cli/cli.go` — `createTaskApproveCommand` is the only production constructor call site.
- `pkg/config/config.go` — `Config`, `Loader`, `GetCurrentUser`.
- `pkg/ops/claude_session.go` — the `exec.CommandContext` + `LookPath` pattern for a subprocess and the injectable-runner precedent.
- `pkg/ops/closure_gate.go` — `normaliseGoalLink` (package-private) for goal-title normalisation.
- `pkg/cli/task_approve_help_test.go` — help-text assertions.
- `docs/task-writing.md` § "Phase transitions" (the bolded "The approval is `vault-cli task approve`" paragraph) — update the contract text.
- `README.md` § Configuration — document the new keys.
- `CHANGELOG.md` — the `## Unreleased` bullet.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`
</context>

<requirements>
## 1. Config

In `pkg/config/config.go`:

- Add to `Config` (match the struct's existing yaml-only tag style):
  ```go
  // RequireManagerOnApprove makes `task approve` refuse a task whose goal
  // resolves no live manager. Off by default so other vaults and the existing
  // suite are unchanged; `private-personal` turns it on.
  RequireManagerOnApprove bool `yaml:"require_manager_on_approve"`
  // ManagerResolverScript is the executable that answers "which live manager
  // owns this subject". Empty means search the installed claude-supervisor
  // plugin.
  ManagerResolverScript string `yaml:"manager_resolver_script,omitempty"`
  ```
- Add to `Loader`:
  ```go
  RequireManagerOnApprove(ctx context.Context) (bool, error)
  ManagerResolverScript(ctx context.Context) (string, error)
  ```
- Implement both on `configLoader` mirroring `GetCurrentUser`: load the config and return the zero value when unset, with no error.
- Run `make generate` so `mocks/config-loader.go` gains both methods. Do not hand-edit anything under `mocks/`.

## 2. `ops.ManagerResolver`

New file `pkg/ops/manager_resolver.go`:

```go
//counterfeiter:generate -o ../../mocks/manager-resolver.go --fake-name ManagerResolver . ManagerResolver

// ManagerResolver answers which live manager owns a subject title.
type ManagerResolver interface {
	// LiveManagerForGoal returns the live manager session name for a goal (or
	// a `managed_by` subject) title. found=false, err=nil means the subject
	// resolves no manager. err!=nil means the fleet read failed and the answer
	// is unknown — never treat it as "no manager".
	LiveManagerForGoal(ctx context.Context, subject string, vaultPath string) (name string, found bool, err error)
}
```

Add a script-backed implementation.

⚠️ **Dependency, stated here so the prompt is honest:** the script `manager-for.py` does not exist yet. It is a sibling change in `bborbe/claude-supervisor` (`scripts/manager-for.py`) and must ship the contract below before `require_manager_on_approve` is enabled in any vault. This prompt ships the code behind an off-by-default flag; its unit and integration tests use stub scripts. Do not enable the flag anywhere as part of this prompt.

Script contract:
- argv: `--goal <subject> --vault <absolute vault path> --json`
- stdout: `{"goal":"<subject>","manager":"<name>"|null,"source":"goal"|"topic"|"managed_by"|null}`. Only `manager` is required; `goal` and `source` are optional.
- exit 0 with a non-empty `manager` → a manager; exit 0 with `manager` empty or null → no manager; exit 2 → the fleet read failed.

Implementation:

- `NewScriptManagerResolver(scriptPath string) ManagerResolver` returns a resolver that runs the named script; an empty `scriptPath` is resolved lazily on the first `LiveManagerForGoal` call (not at construction, so the constructor stays pure), in order:
  1. `filepath.Glob(filepath.Join(home, ".claude", "plugins", "cache", "*", "supervisor", "*", "scripts", "manager-for.py"))`, newest match by directory modification time. The plugin directory is `supervisor`; the marketplace directory is `claude-supervisor`.
  2. `filepath.Join(home, ".claude", "plugins", "marketplaces", "claude-supervisor", "scripts", "manager-for.py")`.
  `home` comes from `os.UserHomeDir()`, and its error is returned rather than ignored (an empty `home` would make the glob relative). An unresolved path returns a wrapped error naming the search.
- `LiveManagerForGoal` runs `exec.CommandContext(ctx, script, "--goal", subject, "--vault", vaultPath, "--json")` under a 30-second timeout, captures stdout and stderr, and parses the JSON.
  - exit 0 with a non-empty `manager` → `(manager, true, nil)`
  - exit 0 with `manager` empty or null → `("", false, nil)`
  - exit 2 → `("", false, errors.Errorf(ctx, "manager resolver %q: fleet unreadable: %s", script, stderr))`. Use `errors.Errorf`, NOT `errors.Wrapf`: the runner reports a non-zero exit with a nil error, and `bborbe/errors.Wrapf(ctx, nil, …)` returns nil (v1.6.7), which would silently turn "unreadable" into "no manager".
  - any other non-zero exit → `errors.Errorf` naming the script and its stderr; malformed JSON or timeout → `errors.Wrapf` the non-nil decode/context error.
- Add an injectable runner for tests:
  ```go
  // ManagerResolverCommandResult is exported so the external ops_test package can
  // write the runner closure (pkg/ops tests are package ops_test).
  type ManagerResolverCommandResult struct {
      Stdout   []byte
      Stderr   []byte
      ExitCode int
  }
  // NewScriptManagerResolverWithRunner returns a resolver with an injectable
  // command runner, for tests.
  func NewScriptManagerResolverWithRunner(
      scriptPath string,
      runCmd func(ctx context.Context, args []string) (ManagerResolverCommandResult, error),
  ) ManagerResolver
  ```
  The `error` is a spawn or timeout failure; `ExitCode` is the script's own status.
- Tests `pkg/ops/manager_resolver_test.go` with the runner fake: manager, null, exit 2, malformed JSON, timeout, and the default search against a temp `HOME` (`t.Setenv`). Assert the three-way result each time.

## 3. Refusal in `TaskApproveOperation`

- Add a `managerResolver ManagerResolver` field. Change the constructor to:
  ```go
  func NewTaskApproveOperation(taskStorage storage.TaskStorage, currentDateTime libtime.CurrentDateTime, managerResolver ManagerResolver) TaskApproveOperation
  ```
  A nil resolver means the gate is off (the existing behavior).
- In `Execute`, after the inbox guard and after the `hasApprovalRecord` repair branch, and before `resolveOwner`:
  - When `managerResolver != nil`:
    - Build the subject list: `normaliseGoalLink(task.GetString("managed_by"))` first when non-empty, then each entry of `task.Goals()` normalised with `normaliseGoalLink`, in order.
    - Call `LiveManagerForGoal` for each subject; the first `found` wins.
    - On `err != nil`, return a refusal whose message names the task, the subject and the unreadable instrument: `refusing to approve %q: cannot read live manager state for %q: %v; nothing was written`.
    - When no subject resolves a manager, return a refusal whose message contains the goal name and the literal `no manager`: `refusing to approve %q: goal %q resolves no manager; nothing was written. Start a manager for the goal first.` When the task has no `goals:` and no `managed_by`, name `(no goal)`.
  - On success, set `task.Set("manager_at_approval", managerName)` in the same single write as `approved_by`, `approved_at`, `phase` and the assignee.
- Return before `o.write` on every refusal, so nothing is written.
- Add a code comment beside the gate stating that the operator-flag path (`task work-on` advancing a `flag: true` row) is deliberately exempt: a flag is the operator's own dispatch, not dispatch-by-omission. Do not change `pkg/ops/workon.go`.
- Do not change the already-approved repair path, the terminal-status refusal, the zero-clock refusal, or the owner-resolution precedence.

## 4. CLI wiring

Inside `createTaskApproveCommand`'s `RunE` (not at command build, which also runs for `--help`):

- Read `requireManager, _ := (*configLoader).RequireManagerOnApprove(ctx)` and `script, _ := (*configLoader).ManagerResolverScript(ctx)`.
- When `requireManager`, construct `ops.NewScriptManagerResolver(script)`; otherwise pass `nil`.
- Pass the resolver to `ops.NewTaskApproveOperation`.
- Update the command `Long` help, `docs/task-writing.md` § Phase transitions, and `README.md` § Configuration: when `require_manager_on_approve` is on, a task whose goal resolves no live manager is refused with nothing written; an unreadable fleet is refused rather than approved; a successful approval records `manager_at_approval`. `README.md` documents the two config keys; `docs/task-writing.md` documents the `manager_at_approval` task key. The flag is off unless set.

## 5. Tests

- `pkg/ops/task_approve_test.go` using the counterfeiter `mocks.ManagerResolver` (generated from the annotation; never a hand-written struct):
  - no manager → refusal naming the goal and `no manager`; `WriteTaskCallCount()==0`
  - resolver error → refusal naming the goal and the unreadable instrument; no write
  - manager found → success; the written task carries `manager_at_approval` equal to the name, plus the existing keys
  - nil resolver → the existing behavior, all current assertions unchanged
  - no `goals:` and no `managed_by` → refusal naming `(no goal)` and `no manager`
  - `managed_by` wins over `goals:`
  - a `[[Goal]]` wikilink is normalised before the resolver call
- `integration/cli_test.go`: one new `Describe` block whose config sets `require_manager_on_approve: true` and `manager_resolver_script` to a temp shell script written with `os.WriteFile(..., 0o755)` and a `#!/bin/sh` first line (a shebang-less file fails `exec format error` even with the exec bit):
  - script prints `{"manager":"goal-manager"}` → approve exits 0 and the file carries `manager_at_approval: goal-manager`
  - script prints `{"manager":null}` → approve exits non-zero, stderr names the goal and `no manager`, the file is unchanged
  - script exits 2 → approve exits non-zero, stderr names the unreadable instrument, the file is unchanged
- Update the three existing `NewTaskApproveOperation` call sites in `pkg/ops/task_approve_test.go` with a `nil` third argument so they compile. Do not change any existing assertion.
- Run `make generate` after the interface changes.

## 6. CHANGELOG

If `CHANGELOG.md` has no `## Unreleased` section, create it below the preamble (top is currently `## v0.200.7`). Add one bullet:
`- feat: refuse task approve when the task's goal resolves no live manager, behind require_manager_on_approve`

Follow `changelog-guide.md` for placement and prefix.

## Self-check

Before finishing, run `ROOTDIR=/workspace make precommit` and confirm it passes; walk each numbered requirement and each test case above against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do not change approval semantics when `require_manager_on_approve` is off.
- Do not change the already-approved repair path, the terminal-status refusal, the owner precedence, or `pkg/ops/workon.go`.
- Every refusal writes nothing; assert `WriteTaskCallCount()==0` in each refusal test.
- Repo-relative paths only in this prompt; the resolver may build the `~/.claude` path at runtime from `os.UserHomeDir()`.
- Existing tests must still pass.
</constraints>

<verification>
ROOTDIR=/workspace make precommit
</verification>
