---
status: generating
approved: "2026-07-02T09:43:09Z"
generating: "2026-07-02T09:43:10Z"
branch: dark-factory/resolve-name-subcommand
---

# Add `vault-cli resolve` Top-Level Subcommand

## Summary

- New `vault-cli resolve <name> --output json` returns typed JSON: `{"type":"task|goal|","name":"...","found":true|false}`
- Probes both task and goal storage with exact name match — task-first priority
- Machine contract only — plain-text mode is a quiet no-op (exits 0)
- Dependency for the merged `/vault-cli:work-on` slash command router
- No new storage methods, no new interfaces, no config changes

## Problem

The merged `/vault-cli:work-on` slash command needs to classify its argument as a task or goal to dispatch to the correct assistant. Today there is no way for a slash command to answer "is this name a task or a goal?" without asking the operator. `vault-cli resolve` fills that gap with a lightweight read-only probe that follows existing vault-cli patterns (layered architecture, Counterfeiter mocks, multi-vault support).

## Goal

`vault-cli resolve "Some Name" --output json` returns structured JSON identifying whether the name matches a task, a goal, or neither — consumed by slash commands to auto-detect entity type without operator input.

## Non-goals

- Plain-text output mode — resolve is a machine contract
- Resolving themes, objectives, or visions — task + goal only
- Fuzzy matching — delegates to existing `findFileByName` behavior
- Cross-vault aggregation — `--vault` flag follows existing multi-vault pattern
- Interactive mode — resolve is read-only and headless

## Acceptance Criteria

Each criterion names an observable evidence shape.

- [ ] **AC1 — Task match:** `vault-cli resolve "Existing Task Name" --output json` returns `{"type":"task","name":"Existing Task Name","found":true}` — evidence: `jq -e '.type == "task" and .found == true'` exits 0
- [ ] **AC2 — Goal match:** `vault-cli resolve "Existing Goal Name" --output json` returns `{"type":"goal","name":"Existing Goal Name","found":true}` — evidence: `jq -e '.type == "goal" and .found == true'` exits 0
- [ ] **AC3 — Not found:** `vault-cli resolve "Does Not Exist" --output json` returns `{"type":"","name":"Does Not Exist","found":false}` — evidence: `jq -e '.found == false'` exits 0, `.type` is empty string
- [ ] **AC4 — Task-first priority:** when a name matches both a task and a goal, return task — evidence: `jq -e '.type == "task" and .found == true'` exits 0
- [ ] **AC5 — Vault scoping:** `vault-cli resolve "Task Name" --vault personal --output json` returns correct scoped JSON — evidence: same JSON shape as AC1
- [ ] **AC6 — `make precommit` passes** with resolve subcommand wired in — evidence: exit code 0
- [ ] **AC7 — Integration test:** `grep -n "resolve" integration/cli_test.go` returns ≥1 line — evidence: grep match
- [ ] **AC8 — No regression:** existing `task get`, `goal get`, `task show`, `goal show` are unaffected — evidence: `make test` passes all pre-existing test suites

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint + format + generate + test
- `make test` — unit + integration test suite passes
- `grep -n "resolve" integration/cli_test.go` — integration test table entry exists
- `grep -rn "ResolveOperation" pkg/ops/` — operation implemented
- `grep -rn "createResolveCommand" pkg/cli/cli.go` — CLI command wired

### Operator-executable (runs on the host after PR merge)

- `/tmp/new-vault-cli resolve "Existing Task Name" --output json | jq -e '.type == "task"'` — real vault probe
- `/tmp/new-vault-cli resolve "Existing Goal Name" --output json | jq -e '.type == "goal"'` — real vault probe
- `/tmp/new-vault-cli resolve "Does Not Exist" --output json | jq -e '.found == false'` — not-found case

## Desired Behavior

1. User or slash command invokes `vault-cli resolve "Name" --output json`
2. CLI resolves vaults via `getVaults` (existing multi-vault helper)
3. For each vault, creates TaskStorage and GoalStorage from `storage.NewConfigFromVault`
4. Probes `TaskStorage.FindTaskByName(ctx, vaultPath, name)` — on success, returns `{type:"task", name, found:true}`
5. On task miss, probes `GoalStorage.FindGoalByName(ctx, vaultPath, name)` — on success, returns `{type:"goal", name, found:true}`
6. On both misses, returns `{type:"", name: inputName, found:false}`
7. JSON output via `PrintJSON(result)`. Plain-text mode prints nothing, exits 0.

## Constraints

- **Layered architecture**: domain type (`pkg/domain/`) → operation (`pkg/ops/`) → CLI (`pkg/cli/`). Never skip a layer.
- **Existing interfaces only**: inject `storage.TaskStorage` + `storage.GoalStorage` (no new storage methods, no new interfaces)
- **Factory purity**: `NewResolveOperation` is pure composition — no conditionals, no I/O, no `context.Background()`
- **Error handling**: `github.com/bborbe/errors` wrapping with context; no `fmt.Errorf`
- **Output contract**: `PrintJSON` helper for JSON; plain mode is silent no-op (exit 0 always — not-found is not an error)
- **Multi-vault**: follows `getVaults` + `VaultDispatcher.FirstSuccess` pattern — task/not-found per vault, first success wins
- **No new dependencies**: `FindTaskByName` and `FindGoalByName` already exist on the storage interfaces
- **Test format**: Ginkgo v2 / Gomega with Counterfeiter mocks (`mocks/task-storage.go`, `mocks/goal-storage.go`)

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Task storage returns unexpected error (not "not found") | Error propagated to caller; JSON output shows error | Operator investigates storage layer (disk full, permission change) |
| Goal storage returns unexpected error | Same — error propagated | Same |
| Name contains characters that break file lookup | Delegated to existing `findFileByName` escape behavior | Existing behavior — no new failure surface |
| Vault config has no tasks_dir or goals_dir | Storage layer errors on path join | Operator fixes vault config; no new failure surface |

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Domain type: `ResolveResult` struct + JSON serialization | 7 | 1-3 | — |
| 2 | Operation: `ResolveOperation` with task-first priority | 4-6 | 3,4 | prompt 1 |
| 3 | CLI: top-level `resolve` command + integration test entry | 1-2,7 | 1-3,5-8 | prompt 2 |

Rationale: prompt 1 establishes the result shape both operation and CLI depend on; prompt 2 implements the probe logic; prompt 3 wires the CLI and integration test after both layers exist.

## Do-Nothing Option

The merged `/vault-cli:work-on` slash command must ask the operator to choose task-vs-goal every time — the exact UX friction this work eliminates. Cost: operator memorizes two commands forever (`work-on-task` / `work-on-goal`), types the wrong one, re-types. Low per-incident cost, high cumulative annoyance.

## References

- `pkg/storage/storage.go:51-61` — TaskStorage + GoalStorage interfaces
- `pkg/storage/task.go:65` — FindTaskByName
- `pkg/storage/goal.go:81` — FindGoalByName
- `pkg/cli/cli.go` — existing command patterns
- `pkg/ops/show.go`, `pkg/ops/search.go` — operation patterns
- `docs/development-patterns.md` — layered architecture, factory pattern
- `docs/dod.md` — Definition of Done checklist
