---
status: completed
spec: [025-bug-complete-misleading-notfound-error]
summary: Introduced typed storage.ErrNotFound sentinel; dispatcher now short-circuits on non-not-found errors so task complete surfaces real precondition failures instead of masking them as 'not found in any vault'
execution_id: vault-cli-complete-error-exec-167-spec-025-typed-notfound-dispatch
dark-factory-version: dev
created: "2026-07-22T20:15:00Z"
queued: "2026-07-22T20:38:48Z"
started: "2026-07-22T20:38:50Z"
completed: "2026-07-22T20:42:13Z"
branch: dark-factory/bug-complete-misleading-notfound-error
---

<summary>
- `vault-cli task complete "<name>"` no longer prints a misleading `not found in any vault` error when the real blocker is a precondition failure (e.g. incomplete subtasks).
- The multi-vault dispatcher now distinguishes a genuine "file not found" from any other error: only a not-found error causes it to try the next vault; every other error is returned immediately, unwrapped.
- A task blocked by pending subtasks now surfaces `incomplete subtasks: N pending` by default — no `--verbose` and no `--vault` needed.
- A task that genuinely exists in no configured vault still reports a `not found in any vault` error, unchanged.
- Not-found is now a typed sentinel error at the storage layer, so callers (and tests) can check it with `errors.Is`.
- Behavior for single-vault configs and for `task get` and every other command is unchanged.
</summary>

<objective>
Make the multi-vault dispatcher's error path distinguish a genuine "file not found" from a precondition (or any other) failure, so `vault-cli task complete` surfaces the real cause (e.g. `incomplete subtasks: N pending`) by default instead of masking it as `not found in any vault`. Achieve this by introducing a typed sentinel not-found error at the storage layer that the dispatcher tests with `errors.Is`.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions.

Read these files fully before making changes:
- `/workspace/pkg/ops/vault_dispatcher.go` — the `FirstSuccess` dispatcher (the loop to change; note it already imports `github.com/bborbe/errors` and `github.com/bborbe/vault-cli/pkg/config`).
- `/workspace/pkg/ops/vault_dispatcher_test.go` — existing Ginkgo tests. The "single vault, fn fails" case (~line 91) and the "multiple vaults, all fail" case (~line 167) encode current wrap behavior and must be updated. Note this test file imports stdlib `errors` (aliased as the default `errors`), NOT `github.com/bborbe/errors`.
- `/workspace/pkg/storage/base.go` — `findFileByName` returns the not-found error at line ~141 via `errors.Errorf(ctx, "file not found: %s", name)`. This is where the sentinel gets introduced.
- `/workspace/pkg/ops/errors.go` — the existing sentinel pattern in this repo (`ErrStarterUnavailable = stderrors.New(...)`). Follow this exact idiom, but the new sentinel goes in `pkg/storage`, not `pkg/ops`.
- `/workspace/pkg/ops/complete.go` (lines ~140-165) — `checkSubtaskCompletion` produces the precondition error `incomplete subtasks: %d pending` via `errors.Errorf` (untyped). Do NOT change this file; it is context only. The precondition error is NOT a not-found, so the dispatcher must return it unwrapped.

Read these coding-plugin docs (they are in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — sentinel-error idiom (`stderrors "errors"` alias + `stderrors.New`), `errors.Wrap`/`Wrapf` for context, `errors.Is` for callers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo/Gomega conventions, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — changelog entry format.

Key facts already verified against source (do not re-derive, but you may re-confirm):
- `github.com/bborbe/errors` v1.5.16 `Wrap`/`Wrapf` are built on `github.com/pkg/errors`, which preserves the cause chain. `errors.Is(err, ErrNotFound)` therefore traverses through both the `pkg/storage` wrap (`find task file`) and any dispatcher wrap. No custom `Unwrap` needed.
- `pkg/ops` already imports `pkg/storage` (see `pkg/ops/complete.go`, `pkg/ops/resolve.go`, etc.). `pkg/storage` does NOT import `pkg/ops`. So the sentinel MUST live in `pkg/storage` (referenced as `storage.ErrNotFound` from the dispatcher) to avoid an import cycle.
- `FirstSuccess` signature: `FirstSuccess(ctx context.Context, vaults []*config.Vault, fn func(vault *config.Vault) error) error`.
</context>

<requirements>
1. Create a new file `/workspace/pkg/storage/errors.go` declaring a package-level sentinel not-found error, following the idiom in `/workspace/pkg/ops/errors.go` and the error-wrapping guide:
   ```go
   // Copyright (c) 2025 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.

   package storage

   import (
       stderrors "errors"
   )

   // ErrNotFound indicates a requested markdown file could not be resolved by
   // name within a vault. Callers (notably the multi-vault dispatcher) test for
   // it with errors.Is to distinguish a genuine not-found from other failures
   // (e.g. a precondition failure such as incomplete subtasks).
   var ErrNotFound = stderrors.New("file not found")
   ```
   Use `stderrors.New` (stdlib), NOT `github.com/bborbe/errors` — a sentinel must be a stable value for `errors.Is`. Match the exact license-header format used in the existing files in `pkg/storage`.

2. In `/workspace/pkg/storage/base.go`, change the terminal not-found return in `findFileByName` (currently `return "", "", errors.Errorf(ctx, "file not found: %s", name)` at line ~141) to wrap the sentinel so the message is preserved AND `ErrNotFound` is in the chain:
   ```go
   return "", "", errors.Wrapf(ctx, ErrNotFound, "%s", name)
   ```
   Do NOT change any other return in `findFileByName` (the `walk directory` error path stays as-is). `errors` here is `github.com/bborbe/errors` (already imported in `base.go`); `errors.Wrapf` preserves the `ErrNotFound` cause so `errors.Is` works. NOTE the format is `"%s", name` (not `"file not found: %s", name`): `pkg/errors` appends the wrapped cause's own message, so the sentinel's `"file not found"` text is added automatically — the resulting message is `<name>: file not found`. Using `"file not found: %s"` here would DOUBLE the phrase (`file not found: <name>: file not found`). Do not reintroduce the doubled prefix. No existing test asserts the exact `file not found: <name>` string, so the reordered `<name>: file not found` (still wrapped by the dispatcher as `not found in any vault: …`) regresses nothing.

3. In `/workspace/pkg/ops/vault_dispatcher.go`, change the multi-vault loop in `FirstSuccess` so that a non-not-found error short-circuits: return it immediately, unwrapped by the `not found in any vault` wrapper. Only a not-found-class error (`errors.Is(err, storage.ErrNotFound)`) should let the loop continue to the next vault. New loop body:
   ```go
   var lastErr error
   for _, vault := range vaults {
       err := fn(vault)
       if err == nil {
           return nil
       }
       if !errors.Is(err, storage.ErrNotFound) {
           return err
       }
       lastErr = err
   }
   return errors.Wrap(ctx, lastErr, "not found in any vault")
   ```
   Add the import `"github.com/bborbe/vault-cli/pkg/storage"` to `vault_dispatcher.go`. `errors` in this file is `github.com/bborbe/errors`, whose `Is` delegates to stdlib `errors.Is`, so `errors.Is(err, storage.ErrNotFound)` is correct here. Do NOT change the empty-vaults branch (returns `no vaults configured`) or the single-vault branch (`return fn(vaults[0])`) — single-vault must keep returning `fn`'s error directly, unchanged. Update the doc comment on `FirstSuccess` to note it continues to the next vault only on a `storage.ErrNotFound`-class error and returns any other error immediately.

4. Update `/workspace/pkg/ops/vault_dispatcher_test.go` so the wrap path is driven by the sentinel, and add coverage for the new short-circuit behavior. This test file currently imports stdlib `errors` as the default `errors`; you also need `storage.ErrNotFound`, so import `"github.com/bborbe/vault-cli/pkg/storage"`. Keep the stdlib `errors` import for `errors.New`/`errors.Is` (stdlib `errors.Is` traverses the chain the same way):
   - In the "multiple vaults, all fail" context (~line 152-174), change the `fn` to return a not-found-class error so the wrap path still triggers. Replace `return errors.New("not found")` with a wrap of the sentinel, e.g. `return fmt.Errorf("find task: %w", storage.ErrNotFound)` (add `"fmt"` import) OR `return storage.ErrNotFound` directly. Keep the existing assertions: message contains `not found in any vault` and `callCount == 2`. Add one assertion that the returned error still satisfies `errors.Is(err, storage.ErrNotFound)` is true.
   - The "single vault, fn fails" context (~line 76-99): this asserts the single-vault branch returns the error directly without the `not found in any vault` wrapper. It uses an untyped `errors.New("op failed")` and must keep passing unchanged (single-vault branch does not consult the sentinel). Leave its behavior intact; if you touch it, only to confirm it still asserts NO `not found in any vault` wrapping.
   - Add a NEW context "multiple vaults, non-not-found error in first vault" that returns a non-not-found error (e.g. `errors.New("incomplete subtasks: 3 pending")`) from the FIRST vault and asserts: (a) `err` contains `incomplete subtasks: 3 pending`, (b) `err` does NOT contain `not found in any vault`, (c) `errors.Is(err, storage.ErrNotFound)` is false, (d) `callCount == 1` (stops at the vault that returned the non-not-found error, does not fall through).
   - Add a NEW context "multiple vaults, precondition error when owning vault is not last" that models the real bug: the FIRST vault returns a not-found (`storage.ErrNotFound`-wrapped) error and the SECOND (owning) vault returns a precondition error (`errors.New("incomplete subtasks: 7 pending")`). Assert the returned error contains `incomplete subtasks: 7 pending`, does NOT contain `not found in any vault`, and `callCount == 2` (loop reached the owning vault, then short-circuited).
   - Add a NEW context (or extend the all-fail one) asserting the genuine not-found case: all vaults return `storage.ErrNotFound`-class errors and the result message contains `not found in any vault` AND `errors.Is(err, storage.ErrNotFound)` is true.

5. Add a `## Unreleased` entry to `/workspace/CHANGELOG.md` immediately after implementing (before running `make precommit`). If `## Unreleased` already exists, append to it. Use the `fix:` prefix. Example:
   ```
   - fix: `task complete` in a multi-vault config now surfaces the real precondition error (e.g. `incomplete subtasks: N pending`) instead of masking it as `not found in any vault`; not-found is now a typed `storage.ErrNotFound` sentinel and the dispatcher short-circuits non-not-found errors.
   ```
   Do NOT touch the `## v0.101.2` section or bump any version string — this repo is `autoRelease: true` and the releaser converts `## Unreleased` post-merge.

6. Coverage: the changed packages are `pkg/storage` and `pkg/ops`. The dispatcher change is fully covered by the new/updated tests in requirement 4. For the `base.go` sentinel-wrap, ADD a new `pkg/storage` test (this is a real coverage gap — no existing `pkg/storage` test asserts on the `file not found` message or the not-found error type): in the appropriate storage `_test.go` for `findFileByName` (or a new `errors_test.go` in the storage test package), add a case that resolves a name with no matching `.md` file and asserts BOTH that the error message contains `file not found` AND that `errors.Is(err, storage.ErrNotFound)` is true. This is the level-1 contract test that exercises the ACTUAL production wrap path (`errors.Wrapf(ctx, ErrNotFound, …)` via `github.com/bborbe/errors`), which the dispatcher tests in requirement 4 do NOT (they use synthetic stdlib-wrapped sentinels). Follow the Ginkgo/Gomega conventions in `go-testing-guide.md`. If any existing `pkg/storage` test asserts the not-found error is NOT wrapped or checks its exact type, update it to expect `errors.Is(err, storage.ErrNotFound)` is true. Do not add retroactive coverage to unrelated untested storage code.
</requirements>

<constraints>
- Do NOT redesign vault-cli error handling across all subcommands. Scope = the complete/mutation dispatch error path only.
- Do NOT change subtask-completion semantics or auto-complete subtasks. Do NOT modify `pkg/ops/complete.go`.
- No UX changes to `task get` or any unrelated command.
- Single-vault config behavior is unchanged: `fn`'s error is returned directly, never wrapped with `not found in any vault`.
- The sentinel MUST live in `pkg/storage` (not `pkg/ops`) to avoid an import cycle — `pkg/ops` imports `pkg/storage`, never the reverse.
- Sentinel uses stdlib `stderrors.New`, never `github.com/bborbe/errors` (a sentinel must be a stable comparable value). Wrapping for context uses `github.com/bborbe/errors` `Wrap`/`Wrapf`. Never use `fmt.Errorf` in production code (test files may use `fmt.Errorf("...%w", ...)` to build a wrapped sentinel for a table case).
- Never fabricate `context.Background()` in `pkg/` wrapping — propagate the incoming `ctx`.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string; this repo is `autoRelease: true`.
- Existing tests must still pass.
</constraints>

<verification>
Run from the vault-cli worktree root:

```
make test
```

Must exit 0. In particular the `pkg/ops` dispatcher tests and `pkg/storage` tests must pass, including the new contexts (non-not-found short-circuit, precondition-when-owning-vault-not-last, genuine-not-found still wrapped).

Then run the full gate once:

```
make precommit
```

Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.

Coverage check for the changed packages:

```
go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/ops/... ./pkg/storage/... && go tool cover -func=/tmp/cover.out | grep -E 'vault_dispatcher.go|base.go'
```

`FirstSuccess` should be fully covered; the `findFileByName` sentinel-wrap line should be covered by existing storage tests.
</verification>
