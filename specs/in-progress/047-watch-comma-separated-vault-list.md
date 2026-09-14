---
status: prompted
approved: "2026-09-14T19:04:43Z"
generating: "2026-09-14T19:29:06Z"
prompted: "2026-09-14T20:08:46Z"
branch: dark-factory/watch-comma-separated-vault-list
---

## Summary

- One `vault-cli watch` process can watch several named vaults at once, instead of one process per vault.
- A consumer that today starts a watcher per vault can start a single watcher for all of them.
- Every event already states which vault it came from, so consumers can still separate them.
- Existing callers see no change: one vault name still means one vault, and omitting the flag still means all vaults.
- The comma form applies to the `watch` command only; no other command's vault selection changes.

## Problem

`vault-cli watch` accepts a single vault name. A consumer that displays several vaults therefore starts one watcher process per vault — vault-ui, for example, runs one subprocess per configured vault, each with its own file-watcher descriptor set, its own restart loop, and its own event stream. The underlying watching engine has no such limitation: it already takes a list of vaults and emits a single event stream in which each event names its vault. Only the command-line boundary is restricted to one name, so the cost of the fan-out is paid entirely for the sake of a flag that cannot express a list.

## Goal

`vault-cli watch --vault a,b,c` runs one watcher process covering exactly those vaults, emitting one newline-delimited JSON event stream in which every event carries its own `vault` field. A single name behaves as it does today; omitting the flag watches every configured vault.

## Non-goals

- Changing `--vault` semantics for any command other than `watch`.
- Adding a new flag, a new subcommand, or a configuration option.
- Changing the watching engine, the event schema, the debounce behaviour, or the `--types` filter.
- Changing how many vaults any consumer chooses to display.
- Reducing the number of vaults vault-cli knows about.
- Making a malformed `--vault` value silently mean "every vault". The only value that selects every vault is the empty string, which is what an unset flag produces.

## Assumptions

- The watching engine already accepts a list of vaults and stamps every event with its vault — verified in `pkg/ops/watch.go`, so no engine change is needed.
- Vault names are matched case-insensitively by the existing configuration lookup — verified in `pkg/config/config.go`, so the new parsing does not need to normalise case.
- A named vault whose task directory is absent is skipped rather than fatal — verified in `pkg/ops/watch.go`, so a partially-provisioned vault does not break the other vaults in the list.
- The empty string is what an unset flag produces, so it keeps meaning "every configured vault": the flag cannot distinguish unset from explicitly empty.

## Acceptance Criteria

- [ ] `vault-cli watch --vault alpha,beta` emits events for **both** vaults on one stdout stream — evidence: the built binary runs against a two-vault config, a task file is written into each vault, and the captured stdout contains one event with `"vault":"alpha"` and one with `"vault":"beta"`. This single check also rules out the smallest wrong build: one that resolves `alpha,beta` as a literal vault name, or that watches only the first name, yields at most one of the two.
- [ ] Existing invocations are unchanged — evidence: with the same two-vault config, `watch --vault alpha` produces a captured stdout containing ≥1 line matching `"vault":"alpha"` and exactly 0 lines matching `"vault":"beta"`; `watch` with no `--vault` produces ≥1 line for each of `alpha` and `beta`.
- [ ] Malformed or unresolvable values fail loudly — evidence: `watch --vault alpha,nope` exits non-zero with a message containing `nope`, and `watch --vault ","` exits non-zero with a message containing `,`; the captured stdout of each contains 0 event lines. `--vault ""` is not one of these cases: it selects every configured vault, matching an omitted flag.
- [ ] **No other command's `--vault` changes** — evidence, both required: (a) with the two-vault config, `vault-cli --config <cfg> task list --vault alpha,beta` still fails with the pre-change `vault not found: alpha,beta` error rather than listing both vaults; (b) `grep -n 'getWatchVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go` returns ≥1 line, and every returned line number falls inside `createWatchCommand`. This is the check that separates a watch-scoped change from the eight-line shortcut of splitting the value inside the shared resolver.
- [ ] The watching engine is untouched — evidence, all three required: `sha256sum pkg/ops/watch.go | cut -d' ' -f1` prints `93e81b44586e968eeb8d246dd2a8eb346f932bd194f47b6868aee3cf1e1b9035`; `wc -l < pkg/ops/watch.go` prints `198`; `find pkg/ops -name '*.go' -type f | wc -l` prints `60`. Content pins rather than a `git diff`: the container runs with `hideGit=true`, which masks `.git` as a character device, so every git command fails with `fatal: not a git repository` — and because the daemon does not check verification exit codes, a failing git command reports a **false pass**. The pins cannot pass vacuously, and the file count covers the whole tree rather than one file.
- [ ] The `watch` command documents the comma form — evidence: `grep -n 'personal,trading' pkg/cli/cli.go` returns a line whose number falls inside the `Long` literal of `createWatchCommand`, not merely somewhere in the file.
- [ ] The multi-vault documentation no longer contradicts the shipped behaviour — evidence: `grep -n 'comma-separated' docs/development-patterns.md` and `grep -n 'getWatchVaults' docs/development-patterns.md` each return ≥1 line, and every returned line number L satisfies `81 < L < 90` — the Multi-Vault Pattern section spans lines 81-89, so an unbounded "greater than the heading" check would also accept a note in the following section.
- [ ] The changelog records the change — evidence: `grep '^## ' CHANGELOG.md | head -1` prints exactly `## Unreleased`, and `grep -c 'comma-separated vault list' CHANGELOG.md` returns ≥1. Placement follows `docs/dod.md`: `## Unreleased` sits below the preamble and above the newest released section, because `make precommit` runs `scripts/check-changelog.sh` and fails if a section lands above the preamble.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint, vet, format, tests and changelog checks all pass; exit 0.
- `go test ./integration/...` — runs the two-vault subprocess test that starts the built binary with `--vault alpha,beta` and asserts events for both vault names.
- `grep -n 'getWatchVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go` — returns ≥1 line, inside `createWatchCommand`.
- `sha256sum pkg/ops/watch.go | cut -d' ' -f1` — prints `93e81b44586e968eeb8d246dd2a8eb346f932bd194f47b6868aee3cf1e1b9035`; `wc -l < pkg/ops/watch.go` — prints `198`; `find pkg/ops -name '*.go' -type f | wc -l` — prints `60`. No git command: `hideGit=true` masks `.git` in this container.
- `grep -n 'personal,trading' pkg/cli/cli.go` — returns a line inside the `watch` command description.
- `grep -n 'comma-separated' docs/development-patterns.md` — returns ≥1 line after the Multi-Vault Pattern heading.
- `grep '^## ' CHANGELOG.md | head -1` — prints `## Unreleased`.

### Operator-executable (runs on the host after the change lands on master)

`.dark-factory.yaml` sets `pr: false` and `autoRelease: false`, so dark-factory itself neither opens a PR nor tags on the feature branch — the change lands on master as a normal commit. The release is then cut by the post-merge `github-releaser-agent`, which `.maintainer.yaml` enables via `release.autoRelease: true` and which converts `## Unreleased` into `## vX.Y.Z` and tags it.

- `vault-cli --version` matches the released tag — the repo's own CLAUDE.md documents installed-vs-HEAD staleness as a known trap, so freshness is checked before anything is observed against the real config.
- `vault-cli watch --vault personal,trading` against the operator's real config — one process starts, and a file change in either vault produces one event naming that vault. This is the end-to-end proof that the released binary serves a real multi-vault config; the container's two-vault config cannot exercise it.

Not part of this spec's gate: the consumer-side outcome — one `vault-cli watch` process per running vault-ui instance instead of one per configured vault — depends on the vault-ui change shipping separately, and is verified there.

## Desired Behavior

1. `watch --vault a,b` starts exactly one process, watching vaults `a` and `b` and no others.
2. Each emitted event names its own vault in the existing `vault` field; the event schema does not change.
3. Whitespace around names is ignored, so `--vault "a, b"` selects the same two vaults as `--vault a,b`; empty entries between commas are ignored, so `--vault a,,b` selects `a` and `b`.
4. A value that yields no usable name — `","`, `" "`, `",,"` — fails with an error naming that value, and no watcher starts. The empty string is not such a value: it means the flag was not set, and selects every configured vault, as today.
5. A single name resolves to exactly that vault, and an omitted flag watches every configured vault — both unchanged from today.
6. `--types` filtering, debouncing, and the `created` / `modified` / `deleted` event vocabulary are unchanged for every vault in the list.

## Constraints

- `--vault` is a root persistent flag shared by every command. Its registration and help text stay unchanged, and the shared single-vault resolver stays the resolver for every command except `watch`. `docs/development-patterns.md` § Multi-Vault Pattern records that split.
- The watching engine is not modified: it already accepts a list of vaults and already stamps each event with its vault.
- The event JSON schema is frozen — no new fields, no renamed fields.
- Backward compatibility is a requirement, not a convenience: a single name and an omitted flag must both behave exactly as they do today.
- Errors use the project's existing error-wrapping library, consistent with the surrounding command file.
- `watch` resolves its vaults through a watch-scoped resolver named `getWatchVaults`; `getVaults` stays unchanged and remains the resolver for every other command. The name is frozen because an acceptance criterion greps for it.
- Two literals are fixed because acceptance criteria grep for them: the `watch` help text shows `--vault personal,trading` as its worked example, and both the help text and the changelog bullet use the phrase `comma-separated vault list`.
- `getWatchVaults`'s doc comment records that a value yielding no usable name is an error, and that the empty string means every configured vault.
- The changelog entry follows `docs/dod.md`, which is this repo's `validationPrompt`.
- Existing tests continue to pass unchanged.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| A name in the list is not a configured vault | The command fails before starting any watcher, with an error naming the offending vault. No partial watcher runs over the names that did resolve. | Operator corrects the name and re-runs. Detectable immediately: non-zero exit and the vault name in the error. |
| The value contains only separators (`","`, `" "`) | The command fails with an error naming the value. It does not silently widen to every vault. | Operator re-runs with real names, or omits the flag to watch everything. |
| A named vault's task directory does not exist yet | That vault is skipped for watching, as it is today for a single vault. Other vaults in the list are unaffected. | Create the directory and restart the watcher. Unchanged from current single-vault behaviour. |
| The same vault is named twice | Both entries resolve to the same vault; watching is unaffected because directories are tracked by absolute path. | None needed. |
| A consumer passes a large list | One process watches them all; descriptor usage scales with vault count, as it already does across separate processes. | Not a regression — the same directories are watched either way. |

## Security / Abuse

The flag value is operator-supplied command-line input that is resolved against the configured vault list. A name that is not configured is rejected with an error and never used to construct a path, so the value cannot select an arbitrary directory: only vaults already present in the operator's own configuration file are watchable. Names are matched case-insensitively by the existing configuration lookup, which is unchanged. No new file is read or written, and no network call is introduced.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Comma-list parsing, watch-scoped vault resolution, command documentation, `docs/development-patterns.md` note, changelog bullet, and the unit + integration tests | 1-6 | 1-8 | — |

Rationale: a single CLI-layer change with one test seam. Splitting it would separate the parser from the only code that calls it, and would separate the tests from the behaviour they lock. One prompt.

## Do-Nothing Option

Every consumer keeps paying the fan-out: one process per vault, each with its own descriptor set and restart loop, scaling linearly with the number of vaults a consumer displays. The duplication is invisible from the outside until something goes wrong — a dozen processes to diagnose instead of one, and a dozen chances for a watcher to be silently dead while the board looks merely stale. The change is small and additive; leaving it undone keeps a structural cost that grows with every vault added.
