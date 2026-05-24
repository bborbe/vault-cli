---
status: completed
spec: [014-bug-work-on-silent-failure-and-hardcoded-slash-command]
summary: Replaced hardcoded /work-on-task with configurable vault.GetWorkOnCommand() in handleClaudeSession
container: vault-cli-exec-129-spec-014-workon-use-configurable-command
dark-factory-version: v0.171.1-3-gd94f1fa
created: "2026-05-24T14:32:00Z"
queued: "2026-05-24T14:24:43Z"
started: "2026-05-24T14:31:18Z"
completed: "2026-05-24T14:34:59Z"
branch: dark-factory/bug-work-on-silent-failure-and-hardcoded-slash-command
---

<summary>
- `pkg/ops/workon.go` no longer contains hardcoded `/work-on-task` literal
- `handleClaudeSession` uses `vault.GetWorkOnCommand()` instead
- The vault config must be accessible from `workOnOperation` to call `GetWorkOnCommand()`
- CLI passes `vault *config.Vault` to `Execute` so the operation has access to the config
</summary>

<objective>
Replace the hardcoded slash command `/work-on-task` in `pkg/ops/workon.go` with the configurable value from `vault.GetWorkOnCommand()`.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes — anchor by symbol, not line number (numbers drift):
- `pkg/ops/workon.go` — `handleClaudeSession` method and its caller `Execute`. The hardcoded `/work-on-task` literal lives in `handleClaudeSession` and is the only call site to change. `Execute` invokes `handleClaudeSession` directly (single call site in this file).
- `pkg/cli/cli.go` — `createWorkOnCommand` closure. The `workOnOp.Execute(...)` call inside has access to `vault *config.Vault` from the enclosing `dispatcher.FirstSuccess` loop.
- `pkg/config/config.go` — `Vault` struct and `GetWorkOnCommand()` method added in sibling prompt `spec-014-config-field-work-on-command.md`.
- `pkg/ops/workon_test.go` — existing `Execute` test cases that need to thread the new parameter.

Note: The `vault` is already available in the CLI closure. The operation needs to receive it to look up the work-on command. Since `WorkOnOperation` doesn't currently have access to the vault config, we thread the `*config.Vault` pointer through `Execute` and `handleClaudeSession`.
</context>

<requirements>
1. In `pkg/ops/workon.go`, update the `WorkOnOperation` interface `Execute` method signature to accept a `vault *config.Vault` parameter (add as last parameter):
   ```go
   Execute(
       ctx context.Context,
       vaultPath string,
       taskName string,
       assignee string,
       vaultName string,
       isInteractive bool,
       sessionDir string,
       vault *config.Vault,
   ) (MutationResult, error)
   ```

2. In `pkg/ops/workon.go`, update the `workOnOperation.Execute` method signature to match the interface (add `vault *config.Vault` parameter).

3. In `pkg/ops/workon.go`, inside `handleClaudeSession`, change the hardcoded prompt:
   ```go
   prompt := fmt.Sprintf(`/work-on-task "%s"`, task.FilePath)
   ```
   to:
   ```go
   prompt := fmt.Sprintf(`%s "%s"`, vault.GetWorkOnCommand(), task.FilePath)
   ```

4. In `pkg/ops/workon.go`, update `handleClaudeSession` signature to accept the vault as the last parameter:
   ```go
   func (w *workOnOperation) handleClaudeSession(
       ctx context.Context,
       task *domain.Task,
       vaultPath string,
       vault *config.Vault,
   ) (string, error)
   ```

5. In `pkg/ops/workon.go`, update the call to `handleClaudeSession` inside `Execute` to pass `vault`:
   ```go
   sessionID, sessionErr := w.handleClaudeSession(ctx, task, sessionDir, vault)
   ```

6. In `pkg/ops/workon.go`, update the `WorkOnOperation` interface `Execute` method signature to accept a `vault *config.Vault` parameter as the last argument:
   ```go
   Execute(
       ctx context.Context,
       vaultPath string,
       taskName string,
       assignee string,
       vaultName string,
       isInteractive bool,
       sessionDir string,
       vault *config.Vault,
   ) (MutationResult, error)
   ```

7. In `pkg/ops/workon.go`, update the `workOnOperation.Execute` method signature to match the interface (add `vault *config.Vault` parameter as last).

8. Add the import for `github.com/bborbe/vault-cli/pkg/config` in `pkg/ops/workon.go` if not already present.

9. Regenerate the mock for `WorkOnOperation` via the project's canonical command:
   ```
   go generate ./...
   ```
   Then verify the regenerated `mocks/workon-operation.go` reflects the new 8th parameter (`vault *config.Vault`). Do NOT hand-edit the mock — re-run `go generate` until the diff matches.

10. In `pkg/cli/cli.go`, inside `createWorkOnCommand`, update the call to `workOnOp.Execute` to pass `vault` as the last argument:
    ```go
    result, err := workOnOp.Execute(
        ctx,
        vault.Path,
        taskName,
        currentUser,
        vault.Name,
        isInteractive,
        sessionDir,
        vault,
    )
    ```

11. In `pkg/ops/workon_test.go`, update all calls to `workOnOp.Execute` to pass a `vault` argument. Create a test vault fixture with `WorkOnCommand` set, and pass `&testVault` as the last argument. Add one new test case that sets `testVault.WorkOnCommand = "/custom-cmd"` and asserts that the prompt actually sent to the starter contains the configured command: `Expect(mockStarter.StartSessionArgsForCall(0).Prompt).To(MatchRegexp("^/custom-cmd "))` (or the equivalent positional accessor — counterfeiter exposes the recorded args).

12. Before running `make precommit`, verify there are no other callers of `Execute` outside `pkg/cli/cli.go` and `pkg/ops/workon_test.go`: `grep -rn 'workOnOp\.Execute\|workOnOperation{.*Execute' --include='*.go' .` — any additional call sites must also be updated.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Do NOT change the `ClaudeSessionStarter` or `ClaudeResumer` interfaces
- The hardcoded `/work-on-task` must be completely removed — `grep -n '"/work-on-task"' pkg/ops/workon.go` should return 0 lines after changes
</constraints>

<verification>
Run `make precommit` — must pass.
Grep verification:
- `grep -n '"/work-on-task"' pkg/ops/workon.go` returns 0 lines
- `grep -n 'GetWorkOnCommand' pkg/ops/workon.go` returns the usage in `handleClaudeSession`
- `go test ./pkg/ops/...` exits 0
</verification>