---
status: approved
created: "2026-11-16T22:40:00Z"
queued: "2026-11-16T23:52:09Z"
---

# Write goal flags as booleans and record their provenance

<summary>
- `vault-cli goal set <goal> flag true` stores a real YAML boolean, not the quoted string `"true"` it stores today.
- `goal set <goal> flag true --by <actor>` records who set the flag, and when, in the same file write that sets the flag.
- A truthy flag with no recorded writer is refused before anything is written.
- Setting a goal's flag provenance directly is refused, exactly as the task verb refuses it.
- The refusal names the goal verb, so the hint the user is shown is a command that works.
- Other goal fields are written exactly as before — only `flag` gains boolean handling.
- Goal flags now behave like task flags, which have done all of this since the flag-provenance change landed.
- The changelog records the fix under `## Unreleased`; no version string is bumped and no tag is created.
</summary>

<objective>
Make `vault-cli goal set` write the flag as a real YAML boolean and record its provenance in the same write, so a flagged goal is readable by strict readers and the flag carries evidence of who set it.
</objective>

<context>
Read `CLAUDE.md` first, then `docs/dod.md`.

Read these before editing:

- `pkg/domain/goal_frontmatter.go` — `SetField` (line 321) and `ClearField` (line 416). The `switch` in `SetField` has no `flag` case, so `flag` falls to `default: f.Set(key, value)` and stores the raw string; yaml.v3 then quotes it.
- `pkg/domain/task_frontmatter.go` — the exemplar for all three pieces: `setFlagField` (line 481), `SetField`'s `case "flag"` (line 530), `SetFlag` (line 335), `ClearFlag` (line 345).
- `pkg/domain/frontmatter_map.go` line 63 — `GetBool` accepts a bool and the strings `true` / `yes` / `false` / `no` case-insensitively.
- `pkg/ops/frontmatter.go` — `writeFlagProvenance` (line 243) and `flagProvenanceSetRefusal` (line 276). Both are task-specific today: the first takes `*domain.Task`, the second hardcodes `vault-cli task set`.
- `pkg/ops/frontmatter_entity.go` — `EntitySetOperation` (line 105) and `goalSetOperation.Execute` (line 115). `Execute` carries no actor, so the goal path cannot record provenance.
- `pkg/cli/cli.go` — `createEntitySetCommand` (line 1001), the goal-only flag registration at line 1054, and the task verb's `--by` binding at line 2740.
</context>

<requirements>
1. **Boolean flag on the goal frontmatter.** In `pkg/domain/goal_frontmatter.go`, add `SetFlag(ctx context.Context, v bool) error`, `ClearFlag()`, and `setFlagField(ctx context.Context, value string) error`, mirroring `TaskFrontmatter`'s three methods. `SetFlag` stores a real `bool` via `f.Set("flag", v)`. `ClearFlag` deletes `flag`, `flag_set_by` and `flag_set_at`. `setFlagField` accepts `true` / `yes` / `false` / `no` case-insensitively with surrounding whitespace trimmed, clears on `""`, and rejects any other value with the task path's error shape. Also extend `GoalFrontmatter.ClearField` so `key == "flag"` additionally deletes `flag_set_by` and `flag_set_at`, mirroring `TaskFrontmatter.ClearField` — otherwise `goal clear <goal> flag` leaves a stale actor behind and defeats the provenance contract.

2. **Wire it into `SetField`.** Add `case "flag": return f.setFlagField(ctx, value)` to `GoalFrontmatter.SetField`'s switch, ahead of the `default` arm. Every other key keeps its current behaviour.

3. **Thread the actor through the entity-set path.** Add a trailing `actor string` parameter to `EntitySetOperation.Execute`. The goal implementation consults it; the theme, objective, vision and topic implementations ignore it (name the parameter `_` where unused, as `SetFlag` already does). Update `createEntitySetCommand`'s call site and every `EntitySetOperation.Execute` call site in `pkg/cli/cli.go` and in the `pkg/ops/*_test.go` files — `frontmatter_entity_test.go`, `frontmatter_entity_session_takeover_test.go`, `wikilink_roundtrip_test.go` and `blocked_by_set_test.go` — plus `mocks/entity-set-operation.go` (counterfeiter regenerates it from the interface).

4. **Write goal provenance in the same write.** Before `SetField`, when `key == "flag"`, the goal operation applies the same three outcomes `writeFlagProvenance` applies to a task: truthy → `flag_set_by` is the actor and `flag_set_at` is the injected `currentDateTime` stored as a bare `time.Time` (so yaml.v3 renders it unquoted); falsy → both provenance keys are deleted; empty → nothing is done here, because the frontmatter's `ClearFlag` removes them. A truthy flag with a blank actor is refused before any mutation, with the task path's error shape naming the goal. Share the truthiness switch with the task helper rather than duplicating it: extract a package-level helper in `pkg/ops/frontmatter.go` taking the frontmatter map, the flag value, the actor and the `currentDateTime`, and call it from both the task and goal paths.

5. **Generalise the direct-provenance refusal.** `flagProvenanceSetRefusal` takes the verb it is refusing for, and emits ``use `vault-cli goal set "<goal>" flag true --by <actor>` `` from the goal path and the existing task wording from the task path. Call it from `goalSetOperation.Execute` before any mutation, with the same byte-identical refusal guarantee the task path has.

6. **Add `--by` to `goal set` only.** In `createEntitySetCommand`, register `--by` (default `"unknown"`, the same help text as the task verb's at line 2740) when `entityType == "goal"`, and pass its value into `Execute`. The theme, objective, vision and topic verbs keep their current flag sets. When `--by` was explicitly passed (`cmd.Flags().Changed("by")`) and `key != "flag"`, refuse with `validation.Error` before dispatch, mirroring the task verb's guard in `createTaskSetCommand` — the hint must name `vault-cli goal set`.

7. **Tests.** Add Ginkgo/Gomega tests that:
   - write `flag true` on a goal and assert the serialized bytes contain `flag: true` and NOT `flag: "true"` — assert on the written file, so the YAML serialization boundary is actually traversed, not just the in-memory map;
   - write `flag false` and assert `flag: false` unquoted;
   - write `flag true` with actor `operator` and assert `flag_set_by: operator` plus a `flag_set_at` that parses as a time;
   - assert a blank actor on a truthy flag is refused and the file is byte-identical;
   - assert a bare `goal set <goal> flag_set_by <x>` is refused and the message names `goal set`;
   - assert `goal clear <goal> flag` removes `flag`, `flag_set_by` and `flag_set_at`;
   - assert a non-boolean goal field (e.g. `phase`) still round-trips unchanged;
   - add one CLI-level end-to-end test in `integration/cli_test.go` that runs `goal set <goal> flag true --by operator` and asserts the written file, so the cobra wiring — `--by` registered for goal, default `unknown` — is exercised rather than only the ops layer.

8. **Changelog.** Add a `## Unreleased` section directly below the preamble (the `All notable changes…` line and the three `* MAJOR/MINOR/PATCH` bullets) and above `## v0.194.2`, never between `# Changelog` and the preamble. Give it a `- fix:` bullet stating that `goal set` now writes the flag as an unquoted boolean and records its provenance, matching `task set`.

9. **Docs.** Add a short paragraph to `docs/goal-writing.md` documenting `goal set <goal> flag true --by <actor>`: the flag is a boolean, `flag_set_by` / `flag_set_at` are written in the same write, and a direct provenance write is refused. `docs/task-writing.md` (lines 172–181) is the wording exemplar.

10. **Self-check.** Before finishing, re-run `<verification>` and confirm it passes; walk each numbered requirement and each test above against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT bump any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag. `.maintainer.yaml: release.autoRelease: true` — the releaser owns the bump after merge.
- Do NOT run `go mod vendor`, and do NOT use `-mod=vendor` anywhere.
- Existing tests must still pass, except where requirement 3 changes a call signature.
- Use `errors.Wrap` / `errors.Wrapf` from `github.com/bborbe/errors`, never `fmt.Errorf`.
- Inject time through the existing `libtime.CurrentDateTime`, never `time.Now()`.
- Do NOT change `task set`'s behaviour; only generalise the shared refusal helper.
- Do NOT add flag handling to the theme, objective, vision or topic frontmatters.
</constraints>

<verification>
Run `ROOTDIR=/workspace make precommit` — must pass.
</verification>
