---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-09-03T19:35:03Z"
generating: "2026-09-03T19:42:29Z"
prompted: "2026-09-03T19:42:29Z"
verifying: "2026-09-03T19:53:13Z"
completed: "2026-09-03T20:18:10Z"
branch: dark-factory/bug-resolve-stops-at-first-vault-on-found-false
---

## Summary

- `vault-cli resolve <name> --output json` run WITHOUT `--vault` returns `found:false` for names that exist in a configured vault — and does so nondeterministically.
- Root cause: the per-vault search handler treats "not found in this vault" as a success and terminates the dispatch, instead of signaling the dispatcher to keep searching. `VaultDispatcher.FirstSuccess` only continues on a `storage.ErrNotFound`-class error, and the resolve handler never returns one.
- Compounding factor: with no `--vault`, the vault search order is random per run (`config.Vaults` is a Go map iterated unsorted), so the same command answers differently on each invocation. Verified: 20 runs, 18 returned `found:false`, 2 returned `found:true`, for a task that exists.
- Fix: keep searching across all configured vaults until a name is found; emit the final `found:false` JSON only after every vault has been searched. A vault that resolves `found:true` ends the search immediately.
- `--vault <name>` and single-vault behavior are unchanged. `resolve` itself shipped in v0.95.0 (spec 021, PR #42) — this is a dispatch defect, not a missing feature.

## Problem

`vault-cli resolve` exists so callers can ask "is this name a task, a goal, or neither?" across all configured vaults at once — the command advertises "multi-vault first-success dispatch" (CHANGELOG.md:411). Without `--vault`, it currently stops at the first vault that lacks the name and reports `found:false`, even when a later vault has it. Because the search order is random (unsorted Go map iteration), the result flips between `found:true` and `found:false` run-to-run. Consumers — the `/vault-cli:work-on` entity-type auto-detection probe and any task-orchestrator script — then misroute an existing entity into a create flow, producing duplicate vault pages. The symptom was initially misattributed to "spec 021 unimplemented"; it is a genuine dispatch bug in code that shipped.

## Goal

After this work, `vault-cli resolve <name> --output json` without `--vault` searches every configured vault in order and:
- emits exactly one JSON document with `"found":true` when the name exists as a task or goal in any configured vault (exit 0);
- emits exactly one JSON document with `"found":false` only when no configured vault contains the name (exit 0);
- never emits error text or a non-zero exit for a not-found outcome.

`--vault <name>` and single-vault configurations behave exactly as they do today.

## Reproduction

Environment:
- `dark-factory --version` → `dark-factory dev`
- vault-cli binary: fresh build of current master (`HEAD 850ef29`); the reported installed version at discovery was `v0.114.4`, and the build on PATH reports `vault-cli version dev`. The defect code path (`pkg/cli/cli.go` `createResolveCommand` closure) is unchanged across these.

Smallest config that exhibits it: any config with two or more vaults where the target entity lives in exactly one of them. The live config `~/.config/vault-cli/config.yaml` has 10 vaults (personal, brogrammers, family, openclaw, trading, openbrain, gaming, starcitzen, octopusagent, boss), and the target task `Vault-cli Resolve Stops At First Vault On found:false` exists only in the personal vault. NOTE: the config loader stores vaults in a Go map and `GetAllVaults` iterates it without sorting (`pkg/config/config.go:315-323`), so the vault search order is random per process run — there is no fixed "first vault".

Command sequence and observed evidence (verbatim, against the fresh master build):

```
vault-cli resolve "Vault-cli Resolve Stops At First Vault On found:false" --output json
```

Observed — nondeterministic; exit 0 in both cases. 20 consecutive runs: 18 emitted `found:false`, 2 emitted `found:true`:

```
{
  "type": "",
  "name": "Vault-cli Resolve Stops At First Vault On found:false",
  "found": false
}
```

```
{
  "type": "task",
  "name": "Vault-cli Resolve Stops At First Vault On found:false",
  "found": true
}
```

```
vault-cli resolve "Vault-cli Resolve Stops At First Vault On found:false" --vault personal --output json
```

Observed — deterministic; 20/20 runs emitted `found:true`, exit 0:

```
{
  "type": "task",
  "name": "Vault-cli Resolve Stops At First Vault On found:false",
  "found": true
}
```

Root-cause reading of the current source:
- `pkg/cli/cli.go` `createResolveCommand` (~1199-1214): the `FirstSuccess` closure calls `resolveOp.Execute` (which never errors — a miss returns a not-found result, not an error, per `pkg/ops/resolve.go:52`), prints the JSON result, and always returns `nil`.
- `pkg/ops/vault_dispatcher.go` `FirstSuccess` (40-76): a `nil` return is terminal success (line 62-63); only a `storage.ErrNotFound`-class error lets the loop continue (line 65). So the first vault that lacks the name terminates the search with a printed `found:false`.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| `resolve <name> --output json` (no `--vault`), name exists in a configured vault | `found:true`, exit 0 — documented contract: the feature ships "multi-vault first-success dispatch" (CHANGELOG.md:411) and "resolves a name to its entity type (task or goal)" (`pkg/ops/resolve.go` doc comment); the integration contract "returns task match as JSON" asserts `found:true` for an existing task (`integration/cli_test.go:1423-1440`) | nondeterministic; 18/20 runs emit `found:false` (see Reproduction) |
| `resolve <name> --vault personal --output json`, name is a task in personal | `found:true`, exit 0 | `found:true`, exit 0 — matches, unchanged |

## Why this is a bug

- The multi-vault invariant: a name resolvable in ANY configured vault must resolve. `found:false` for an existing name breaks the advertised "multi-vault first-success dispatch" contract (CHANGELOG.md:411).
- `FirstSuccess`'s documented continuation contract (`pkg/ops/vault_dispatcher.go:35-39`) is "nil = success, only `ErrNotFound` continues". The resolve handler violates it by returning `nil` on a miss — it reports success for "not found in this vault", so the dispatcher stops before ever reaching the vault that has the name.
- The random map iteration (`pkg/config/config.go:315-323`) turns this into a flaky liar: the same command returns `found:true` or `found:false` at random, so a single failing run is hard to reproduce and a single passing run proves nothing.
- Consumer impact: `/vault-cli:work-on` auto-detects task vs goal via the `vault-cli resolve` probe (CHANGELOG.md:412); a wrong `found:false` routes an existing entity into the not-found create flow, creating duplicate vault pages.

## Acceptance Criteria

All ACs are build-time / local — no deployed system is involved, so none carry a Post-Deploy marker.

- [ ] **Multi-vault fall-through, task:** with a config of exactly two vaults where the task file exists in exactly one of them, `resolve <name> --output json` WITHOUT `--vault` exits 0 and stdout is exactly one JSON document containing `"type":"task"` and `"found":true` — on every run, independent of iteration order. Evidence: the new two-vault integration case in `integration/cli_test.go` passes — `grep -c 'multi-vault' integration/cli_test.go` returns >= 1 (a distinct two-vault `Context`, not a duplicate single-vault case) AND `grep -c 'HaveKeyWithValue("type", "task")' integration/cli_test.go` returns >= 3; `make test` exits 0.
- [ ] **Multi-vault fall-through, goal:** same setup with a goal file in exactly one of two vaults; `resolve <name> --output json` without `--vault` exits 0 and stdout is exactly one JSON document containing `"type":"goal"` and `"found":true`. Evidence: `grep -c 'HaveKeyWithValue("type", "goal")' integration/cli_test.go` returns >= 2; `make test` exits 0.
- [ ] **`found:false` only after exhaustion:** with a two-vault config where the name is in neither vault, `resolve <name> --output json` without `--vault` exits 0, stdout is exactly one JSON document with `"type": ""` and `"found": false`, and neither stdout nor stderr contains the text `not found in any vault`. Evidence: `grep -c 'HaveKeyWithValue("found", false)' integration/cli_test.go` returns >= 2 AND `grep -c 'NotTo(ContainSubstring("not found in any vault"))' integration/cli_test.go` returns >= 2 (one stdout assertion, one stderr assertion); `make test` exits 0.
- [ ] **`--vault` single-vault path unchanged (regression):** the existing resolve integration cases (`returns task match as JSON`, `returns goal match as JSON`, `returns not found for unknown name`, `task-first priority when name matches both`) remain present and pass unchanged. Evidence: `grep -c 'returns task match as JSON' integration/cli_test.go` == 1, `grep -c 'returns goal match as JSON' integration/cli_test.go` == 1, `grep -c 'task-first priority' integration/cli_test.go` == 1; `make test` exits 0.
- [ ] **Unit test covers multi-vault fall-through:** a unit test exercises the resolve command's per-vault dispatch decision with fakes — a vault resolving `found:false` does not terminate the search (with two vaults the second is still tried: call count reaches 2), and a vault resolving `found:true` terminates it (call count 1). Evidence: `grep -c 'Equal(2)' pkg/cli/resolve_test.go` returns >= 1 and `grep -c 'Equal(1)' pkg/cli/resolve_test.go` returns >= 1; `make test` exits 0.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```
make precommit    # exit 0
make test         # exit 0 — includes the new two-vault resolve integration cases and the resolve unit test
grep -c 'multi-vault' integration/cli_test.go                                        # >= 1
grep -c 'HaveKeyWithValue("type", "task")' integration/cli_test.go                   # >= 3
grep -c 'HaveKeyWithValue("type", "goal")' integration/cli_test.go                   # >= 2
grep -c 'HaveKeyWithValue("found", false)' integration/cli_test.go                   # >= 2
grep -c 'NotTo(ContainSubstring("not found in any vault"))' integration/cli_test.go  # >= 2
grep -c 'returns task match as JSON' integration/cli_test.go                         # == 1
grep -c 'returns goal match as JSON' integration/cli_test.go                         # == 1
grep -c 'task-first priority' integration/cli_test.go                                # == 1
grep -c 'Equal(2)' pkg/cli/resolve_test.go                                           # >= 1
grep -c 'Equal(1)' pkg/cli/resolve_test.go                                           # >= 1
```

### Operator-executable (host, after PR merge + release; per `docs/releasing-vault-cli.md` release gate)

1. Build a fresh binary: `go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .`
2. Replay the reproduction against `/tmp/new-vault-cli` — run 20 consecutive invocations of `resolve "Vault-cli Resolve Stops At First Vault On found:false" --output json` (no `--vault`): all 20 must emit `found:true` (pre-fix this loop returns `found:true` in only ~10% of runs). Exit 0 each time, exactly one JSON document per run.
3. `resolve "Vault-cli Resolve Stops At First Vault On found:false" --vault personal --output json` emits `found:true`, exit 0 — unchanged.
4. Negative check: `resolve <name-not-in-any-vault> --output json` emits exactly one JSON document with `"found": false`, exit 0, and no `not found in any vault` text on stdout or stderr.
5. Release gate per `docs/releasing-vault-cli.md`: walk `scenarios/*.md` (001-005) against `/tmp/new-vault-cli`, each "Action" + "Expected" must pass, before `make install`.
6. `make install`; `vault-cli --version` reports the new version. If the plugin surface (`commands/`, `agents/`, `docs/`, `skills/`) changed, run the plugin release checklist (`docs/releasing-vault-cli.md` § Plugin release) and re-verify the changed surface.

## Desired Behavior

1. **Search continues past a not-found vault.** When a vault resolves a name to `found:false`, `resolve` proceeds to the next configured vault instead of terminating. Nothing is printed for a per-vault miss.
2. **First found vault wins.** The first vault (in the dispatcher's iteration order) that resolves the name to `found:true` ends the search; `resolve` emits that vault's result as exactly one JSON document and exits 0. Later vaults are not searched.
3. **`found:false` is emitted only after exhaustion.** `resolve` emits the not-found result (`"type": ""`, `"found": false`, name echoing the input) only after every configured vault has resolved `found:false`; exit 0.
4. **Exhausted search is not an error.** The dispatcher's wrapped `not found in any vault` error is consumed by the command and converted to the single `found:false` JSON — no error text on stdout or stderr, exit code 0.
5. **Plain mode stays silent.** Without `--output json`, `resolve` prints nothing in every outcome (found, not-found, exhausted) and exits 0 — the plain-mode no-op contract is unchanged.
6. **`--vault` selects one vault.** With `--vault X`, `resolve` searches only vault X and emits its result exactly as today: one JSON document (`found:true` or `found:false`), exit 0.
7. **Single-vault configs behave as today.** With exactly one configured vault, `resolve` emits the same output and exit code as before the fix.

## Constraints

- **JSON output schema of `resolve` unchanged.** Exactly `{"type":"...","name":"...","found":...}`; `type` and `found` serialize even when empty/false (no `omitempty`) — the existing raw-output assertions (`integration/cli_test.go:1479-1481`) keep passing.
- **`ResolveOperation` contract unchanged.** A miss is not an error from `Execute` (`pkg/ops/resolve.go:52` returns a not-found result, nil error). The continuation signal is produced by the resolve command's dispatch layer, not by changing the operation.
- **`VaultDispatcher.FirstSuccess` semantics unchanged.** Only a `storage.ErrNotFound`-class error continues the loop; any other error propagates immediately (`pkg/ops/vault_dispatcher.go:62-66`). The fix signals continuation through that existing mechanism.
- **`--vault` flag semantics unchanged.** It selects the single vault to search.
- **Single-vault dispatch path unchanged.** With exactly one vault, `FirstSuccess` calls the handler directly (`pkg/ops/vault_dispatcher.go:48-50`).
- **Error style.** `github.com/bborbe/errors` with context wrapping; no `fmt.Errorf` (`docs/dod.md`).
- **No `os.Stdout` / `fmt.Print*` in `pkg/ops/`** (`docs/dod.md`) — output stays in the CLI layer.
- **Exactly one JSON document per invocation** on stdout.
- **Exit code 0** for both found and not-found outcomes; non-zero only for genuine errors (no vaults configured, context cancellation, hard vault error).

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Name absent from every configured vault | `FirstSuccess` wraps the last `ErrNotFound` as `not found in any vault (tried: ...)`; the command consumes it and emits the single `found:false` JSON, exit 0 | None needed — normal negative outcome; callers run their create flow on `found:false` |
| A vault errors hard (I/O failure, unreadable path, permission) | Non-`ErrNotFound` error propagates immediately (not swallowed, not masked as not-found); the command surfaces it, non-zero exit | Fix the vault (path, permissions, config); re-run |
| Context cancelled mid-search | `FirstSuccess` returns a `context cancelled` wrapped error; the command propagates it, no partial JSON emitted | Re-run; if a caller keeps cancelling, investigate the caller's timeout |
| No vaults configured | `FirstSuccess` returns `no vaults configured`; the command surfaces the error, non-zero exit | Fix config; re-run |
| Same name exists in multiple vaults, same type | First vault in the dispatch order wins; later matches not searched — result content identical regardless of order | None — deterministic outcome |
| Same name exists in multiple vaults with different types (task in one, goal in another) | Search order is nondeterministic (unsorted map iteration), so the winning type is nondeterministic | Residual, pre-existing, out of scope — deterministic precedence (e.g. config-file order) is a separate spec |

## Suggested Decomposition

Single-layer, single-prompt fix: the change is confined to the `resolve` command's dispatch closure and the command-level handling of the exhausted-search error in `pkg/cli/cli.go`, plus tests (`integration/cli_test.go`, new `pkg/cli/resolve_test.go`). The prompt-creator may produce a single prompt covering Desired Behaviors 1-7 and Acceptance Criteria 1-5; no ordering constraints exist.

## Do-Nothing Option

Doing nothing keeps a shipped command that lies nondeterministically: for a name that exists in a non-first-iterated vault, `resolve` without `--vault` reports `found:false` in ~90% of runs (10-vault config), so entity-type auto-detection (`/vault-cli:work-on` probe, task-orchestrator scripts) misroutes existing entities into create flows and duplicate vault pages. The only workaround — always passing `--vault` explicitly — defeats the multi-vault feature the command was built for. The bug is in shipped code with named consumers; this is not acceptable current behavior.

## Verification Result

**Verified:** 2026-09-03T20:17:04Z (HEAD 462f8de)
**Binary:** /tmp/new-vault-cli (fresh build of master incl. fix 22e52f7, release v0.121.2); installed vault-cli v0.121.2-dirty cross-checked
**Scenario:** runtime replay of resolve multi-vault dispatch on the live 10-vault config, plus scenarios 001-004 regression walks
**Evidence:**
- 20/20 no-vault runs `resolve "Vault-cli Resolve Stops At First Vault On found:false" --output json` -> `"found":true`, rc=0, exactly one JSON doc each, stderr empty
- `resolve "Automatic Vault Optimization" --output json` -> `{"type":"goal","name":"Automatic Vault Optimization","found":true}` rc=0
- `resolve "no-such-entity-xyz" --output json` -> `{"type":"","name":"no-such-entity-xyz","found":false}` rc=0; stderr empty, no `not found in any vault` text
- `resolve ... --vault personal --output json` -> `"found":true` rc=0 (both /tmp/new-vault-cli and installed binary)
- `make precommit` rc=0; `make test` rc=0; all 10 grep gates pass (two-vault context integration/cli_test.go:1589; resolve_test.go Equal(2)/Equal(1))
- Scenarios 001-004 walked against /tmp/new-vault-cli: all PASS (002 live claude session bb7486b5, transcript on disk); 005 operator-only (interactive TTY), not run
**Verdict:** PASS
