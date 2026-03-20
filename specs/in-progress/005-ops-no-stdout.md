---
status: prompted
approved: "2026-03-20T19:29:12Z"
prompted: "2026-03-20T19:35:26Z"
branch: dark-factory/ops-no-stdout
---

## Summary

- Operations in the ops layer write directly to stdout, making them unusable as a Go library
- All stdout and print calls are removed from operations — they return structured results instead
- The CLI layer owns all output formatting (plain text and JSON)
- External CLI behavior remains identical
- Tests use direct result assertions instead of stdout capture

## Problem

Every operation in `pkg/ops/` mixes business logic with output formatting. Operations accept an output format parameter and write directly to stdout. This means any Go program importing `pkg/ops/` gets unwanted stdout output instead of structured return values. The ops layer cannot be used as a library, composed into pipelines, or tested without capturing stdout. 14 files are affected.

## Goal

After this work, `pkg/ops/` is a clean library layer: operations return structured data and never write to stdout. The output format parameter is removed from all operation interfaces. The CLI layer calls operations, receives results, and formats output. Any Go program can import `pkg/ops/` and get programmatic access to vault operations.

## Assumptions

- All 14 affected files follow the same stdout pattern and can be migrated with the same approach
- Returning structured results does not break any external consumers (vault-cli is the only known consumer)
- Streaming operations (watch) can use a callback instead of returning a collection
- Existing result types or new ones can represent all current output without information loss

## Non-goals

- Changing CLI output format (must remain backward-compatible)
- Refactoring operation logic, filters, or sorting
- Adding new operations or commands
- Changing the storage layer
- Making `pkg/cli/` importable as a library

## Desired Behavior

1. No operation in the ops layer writes directly to stdout or uses print functions for output
2. Operation interfaces return structured results, not just errors
3. The output format parameter is removed from all operation interfaces
4. CLI commands receive structured results and format them for display
5. Streaming operations accept a callback handler instead of writing to stdout
6. All existing tests pass — test assertions change from stdout capture to direct result comparison
7. External CLI output is identical for both plain and JSON modes

## Constraints

- CLI output format must not change (same text, same JSON structure)
- Operation naming convention is preserved
- Mock generation comments are preserved; mocks regenerated after interface changes
- Factory function pattern (pure composition, no I/O) is preserved

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Operation returns new type but CLI doesn't format it | Compilation error | Fix CLI formatter before merging |
| Mock interface changes break downstream tests | Mock regeneration fixes it | Run verification command |
| Streaming operation returns before completion | Callback keeps watcher alive until context cancels | Verify watch still streams continuously |
| JSON output field order changes | Tests catch regression | Compare output against baseline |

## Security / Abuse

Low risk — this is an internal refactor with no new I/O surfaces, no new user input handling, and no network changes.

## Acceptance Criteria

- [ ] No operation in the ops layer writes directly to stdout (excluding test files)
- [ ] No operation in the ops layer uses print functions for output (excluding test files)
- [ ] No operation interface accepts an output format parameter
- [ ] All operation interfaces return structured results
- [ ] External CLI output is unchanged for both plain and JSON modes
- [ ] All tests pass and verification command succeeds

## Verification

```
make precommit
```

```
grep -r 'os\.Stdout' pkg/ops/ | grep -v _test.go
# expected: no output
```

```
grep -r 'fmt\.Print' pkg/ops/ | grep -v _test.go
# expected: no output
```

## Do-Nothing Option

vault-cli remains CLI-only. Any integration requiring programmatic access (e.g., a Goal Program managing tasks) must shell out to `vault-cli` and parse stdout. This is fragile, slow, and prevents vault-cli from being composed into larger Go systems. The current test pattern of swapping `os.Stdout` is a code smell that becomes harder to maintain as operations grow.
