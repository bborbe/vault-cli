---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-07-22T20:01:14Z"
generating: "2026-07-22T20:01:48Z"
prompted: "2026-07-22T20:06:13Z"
verifying: "2026-07-22T20:42:14Z"
branch: dark-factory/bug-complete-misleading-notfound-error
---

## Summary

- `vault-cli task complete "<name>"` prints a misleading `not found in any vault: ... file not found` error when the task file actually resolves fine and the real blocker is a precondition failure (incomplete subtasks).
- Root cause: the multi-vault dispatcher (`FirstSuccess`) treats EVERY per-vault error as "not found, try next vault", so a real precondition error from the owning vault is overwritten by a later vault's genuine not-found and masked.
- Both the precondition error and the not-found error are currently untyped, so the dispatcher cannot tell them apart.
- The true cause (`incomplete subtasks: N pending`) is only visible with `--verbose --vault <name>`, sending users down a "missing file" rabbit hole instead of "tick your subtasks".
- Fix scope: the complete/mutation dispatch error path only — make not-found distinguishable so the dispatcher short-circuits non-not-found errors.

## Problem

When a user runs `vault-cli task complete "<name>"` against a multi-vault config and the task has one or more pending/in-progress subtasks, the command prints `Error: not found in any vault: find task: find task file: file not found: <name>`. This is wrong: the task file exists and resolves fine (e.g. `vault-cli task get <name> status` works). The real cause is a precondition failure — incomplete subtasks — which the user can only discover by re-running with `--verbose --vault <name>`. The misleading error class costs the user time chasing a nonexistent missing-file problem instead of ticking their subtasks.

## Goal

By default (no `--verbose`, no `--vault`), `vault-cli task complete` on a task blocked by incomplete subtasks surfaces the real precondition error (`incomplete subtasks: N pending`). Not-found errors and precondition-failure errors are distinguishable as different classes rather than both collapsed into `not found in any vault`. A task that genuinely exists in no configured vault still reports a not-found-class error.

## Reproduction

- **Version:** vault-cli v0.101.2 (current worktree HEAD; confirm via `vault-cli --version`).
- **Precondition:** task exists in exactly one of MULTIPLE configured vaults AND carries at least one pending (`[ ]`) or in-progress (`[/]`) subtask. The owning vault must not be iterated last by the dispatcher.
- **Command:** `vault-cli task complete "<name>"` (no `--vault`, no `--verbose`).
- **Observed 2026-07-04** while completing `Cleanup Obsidian Inbox - 2026-07-04`: two `task complete` calls returned `Error: not found in any vault: find task: find task file: file not found: <name>`, despite `vault-cli task get <name> status` resolving the same file. Only `vault-cli task complete <name> --verbose --vault Personal` revealed the actual cause: `incomplete subtasks: 7 pending`.
- **Recurrence same day:** hit 5× across periodic-close tasks (Review Month, Review Quarter, Draft Q2 Verdict, Plan Month, Weekly Review), each carrying an intentionally-deferred `[/]` subtask.

## Expected vs Actual

- **Expected:** `task complete` on a task with pending subtasks prints the real precondition error (e.g. `incomplete subtasks: N pending`) by default, without `--verbose`. Not-found and precondition-failure are distinguishable (different message/class), not conflated.
- **Actual:** prints `Error: not found in any vault: find task: find task file: file not found: <name>`, masking the real cause.

## Why this is a bug

A user-visible error message that is wrong/misleading — the "When to file as a bug" case in `bug-workflow.md`. The dispatcher conflates two distinct error classes. `docs/development-patterns.md` (multi-vault section) states mutation commands "try each vault until the item is found"; a precondition failure means the item WAS found and must short-circuit, not fall through to a not-found from a different vault.

## Root Cause

`FirstSuccess` in `pkg/ops/vault_dispatcher.go` (lines 36-56) loops over all configured vaults calling `fn` per vault, keeps only `lastErr`, and wraps it with `"not found in any vault"` if all fail (line 55). It treats every `fn` error as "not found, try next vault". For `task complete`:

- In the OWNING vault (e.g. Personal), `fn` returns `incomplete subtasks: N pending` — a precondition failure produced at `pkg/ops/complete.go:164` via `errors.Errorf` (untyped). The task WAS found.
- In OTHER vaults, `fn` returns `... file not found: <name>` — a genuine not-found produced at `pkg/storage/base.go:141` via `errors.Errorf` (untyped).

Both errors are untyped, so `FirstSuccess` cannot distinguish them. When the owning vault is not iterated last, the real precondition error is overwritten by a later vault's not-found and masked as `not found in any vault`.

## Desired Behavior

1. `task complete` on a task with pending subtasks in a multi-vault config surfaces `incomplete subtasks: N pending` by default — no `--verbose`, no `--vault` required.
2. The dispatcher distinguishes not-found from other errors: it continues to the next vault ONLY on a not-found-class error; any other error is returned immediately, unwrapped by the "not found in any vault" wrapper.
3. A genuine not-found (task in no vault) still yields a not-found-class error (message still contains `not found in any vault`).
4. Not-found is expressed as a typed/sentinel error at the storage layer so callers can test it with `errors.Is`.

## Constraints

- Do NOT redesign vault-cli error handling across all subcommands. Scope = the complete/mutation dispatch error path only.
- Do NOT change subtask-completion semantics or auto-complete subtasks.
- No UX changes to `task get` or any unrelated command.
- **Fix direction (intended approach, prompt retains latitude on details):** introduce a typed/sentinel not-found error at the storage layer so `FirstSuccess` can `errors.Is(err, ErrNotFound)`; continue to the next vault only on not-found, otherwise return the error immediately unwrapped. Follow bborbe/errors idioms per `coding/docs/go-error-wrapping-guide.md` — static-message sentinel via `stderrors.New`, `errors.Wrap` for context, `errors.Is` for callers.
- Existing dispatcher tests in `pkg/ops/vault_dispatcher_test.go` encode the current wrap behavior: the single-vault direct-return assertion (~line 91) and the multi-vault all-fail wrap assertion (~line 167, which returns an untyped `errors.New("not found")`). Update these expectations so the wrap path is driven by the sentinel not-found rather than any untyped error.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection |
|---------|-------------------|----------|-----------|
| Task blocked by pending subtasks, owning vault not iterated last | Precondition error `incomplete subtasks: N pending` returned immediately, unwrapped | User ticks subtasks, re-runs `task complete` | stderr shows precondition message, exit non-zero |
| Task exists in no configured vault | Not-found-class error, message contains `not found in any vault` | User corrects the task name | stderr shows not-found message |
| Non-not-found, non-precondition error from `fn` (e.g. read/parse failure) in some vault | Error returned immediately, unwrapped, not masked as not-found | Depends on underlying error | stderr shows the real error, not `not found in any vault` |
| Single-vault config | `fn` error returned directly (unchanged behavior) | n/a | Existing single-vault test path |

## Workaround

- `task set status completed` + `task set phase done` bypasses the completion gate entirely — itself a smell, since it skips the subtask precondition.
- `task complete <name> --verbose --vault <name>` reveals the real error but requires the user to already suspect the true cause and know the owning vault.

## Acceptance Criteria

- [ ] `task complete` on a task with ≥1 pending subtask in a multi-vault config surfaces `incomplete subtasks: N pending` by default (no `--verbose`) — evidence: command stderr contains `incomplete subtasks:` and does NOT contain `not found in any vault`.
- [ ] Not-found and precondition-failure errors are distinguishable — evidence: a unit test asserts the precondition-error path returns an error where `errors.Is(err, ErrNotFound)` is false, while the genuine not-found path returns one where it is true.
- [ ] A genuine not-found (task in no vault) still reports a not-found-class error — evidence: unit test asserts the returned error message contains `not found in any vault`.
- [ ] `FirstSuccess` returns a non-not-found error directly, unwrapped, without continuing to later vaults — evidence: extended test in `pkg/ops/vault_dispatcher_test.go` asserts (a) `err` does NOT contain `not found in any vault`, and (b) `callCount == 1` (stops at the vault that returned the non-not-found error).
- [ ] The incomplete-subtasks path across multiple vaults surfaces the precondition error — evidence: extended test in `pkg/ops/vault_dispatcher_test.go` asserts the precondition error is returned unwrapped when the owning vault is not last.
- [ ] Existing dispatcher tests (~line 91, ~line 167) updated so the not-found wrap path is driven by the sentinel not-found error — evidence: tests reference `ErrNotFound` and pass.
- [ ] `make test` exits 0 in the vault-cli worktree — evidence: exit code 0.

## Verification

```
make test
```

Manual smoke (optional, in a multi-vault config with a task carrying a pending subtask):

```
vault-cli task complete "<name>"
# expect stderr: incomplete subtasks: N pending
# expect stderr NOT to contain: not found in any vault
```
