---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-09-15T12:34:22Z"
generating: "2026-09-15T12:47:42Z"
prompted: "2026-09-15T12:47:42Z"
verifying: "2026-09-17T15:54:28Z"
branch: dark-factory/per-vault-topics-dir
---

## Summary

- Every entity directory vault-cli knows about can be declared per vault — tasks, goals, themes, objectives, vision, daily notes, knowledge. Topics cannot, even though `23 Topics/` is a real folder in the Personal vault.
- This spec adds a `topics_dir` vault key, read through the same accessor shape as its seven siblings, defaulting to `23 Topics` when a vault does not set it.
- The value reaches consumers through `vault-cli config list --output json`, the same surface the other directory keys already use.
- No command, flag, output format, or existing behaviour changes: a config file without the key behaves exactly as it does today.
- The vault-local markdown commands that currently hardcode the folder name are a separate change in a different vault, and are out of scope here.

## Problem

`23 Topics/` is a first-class folder in the Personal vault — it holds the topic pages that the topic workflows operate on — but vault-cli has no idea it exists. `pkg/config/config.go` declares seven directory keys on a vault (`tasks_dir`, `goals_dir`, `themes_dir`, `objectives_dir`, `vision_dir`, `daily_dir`, `knowledge_dir`) and not an eighth. The topic folder is therefore resolved by convention alone: vault-local command files hardcode the folder name in a file search, so the folder is configurable only by editing markdown, and a vault whose topic pages live somewhere else cannot say so. Every other entity directory in this project is declared once, per vault, in one config file and read through one accessor; topics are the single exception, and the exception exists because the key was never added — not because topics are special.

## Goal

A vault's configuration can declare `topics_dir`, exactly like its seven sibling directory keys. Reading a vault's topics directory returns the configured folder when set, and `23 Topics` when the key is absent or empty — never an error, never an abort, for any vault. `vault-cli config list --output json` reports the key for a vault that sets it and omits it for a vault that does not, matching how `tasks_dir` already behaves. Nothing else about vault-cli changes: no new command, no new flag, no altered output, and every existing config file keeps working byte-identically.

## Non-goals

- Rewriting the Personal vault's `.claude/commands/topic-manager.md` and `.claude/commands/topic-status.md` to read the folder from config. Those are markdown command files in a different vault, outside this repository; per `~/Documents/workspaces/dark-factory/docs/choosing-a-flow.md` a markdown-only edit takes the direct flow, not a spec. They keep their hardcoded `23 Topics` until that separate change lands.
- Adding a `topic` command noun (`vault-cli topic list`, `topic show`, and similar). No consumer has asked for one, and this spec ships the configuration contract those commands would need if they are ever requested.
- Adding topics to the watch target set. `buildWatchTargets` (`pkg/cli/cli.go`) already emits four watch directories per vault — task, goal, theme, objective; adding topics would make it five and change the event stream of `vault-cli watch` for every vault, and no consumer has asked for topic events.
- Renaming, moving, or migrating any vault's `23 Topics/` folder, and writing `topics_dir` into any config file. The config file stays read-only.
- Normalising, validating, expanding, or resolving the value (no `~` expansion, no trailing-slash trimming, no existence check). The seven sibling keys do none of this.
- Do NOT add a flag, opt-out, or env var that disables the default — invariant; a vault that omits the key gets `23 Topics`, and if a future consumer demands variation, that is a separate spec.
- Do NOT add a tunable default, a per-vault override of the default, or a second default value — invariant; `23 Topics` is the single literal.
- Bumping any version string or creating a tag. `.maintainer.yaml` sets `release.autoRelease: true`; the post-merge releaser owns the bump and the tag.
- Adding a new scenario file. The existing `scenarios/001-config-list.md` already pins the `config list` JSON surface and is extended by this change.

## Assumptions

- `23 Topics` is the default literal: the Personal vault's topic pages live in `23 Topics/` today, and the operator confirmed that folder as the default.
- Vault directory keys are plain, unvalidated strings today — `GetTasksDir`, `GetThemesDir`, `GetObjectivesDir`, and `GetVisionDir` each return the configured value when non-empty and a literal otherwise, with no normalisation. `topics_dir` mirrors that shape, so no other loader logic changes.
- The loader's vault path carries new struct fields automatically: `expandVaultPaths` copies the vault struct, and both `GetVault` and `GetAllVaults` return the copy, so a new field survives every resolution path without a dedicated change.
- `config list --output json` marshals the resolved vault structs directly, so a new key reaches the JSON output through the struct's own tag; there is no separate projection or allowlist to update.
- No in-repo code reads a topics directory. The accessor is the contract for the vault-local commands that own topic workflows, which live outside this repository.
- `.dark-factory.yaml` sets `workflow: direct` and `pr: false`, so this change lands as a commit on the feature branch and then on master; the post-merge releaser converts `## Unreleased` into a versioned section and tags it.
- The container may or may not expose `.git`: `.dark-factory.yaml` sets `workflow: direct` without `hideGit`, and `dark-factory`'s `workflows.md` documents `direct` as giving the container the host's real `.git/` — while a run started with `--set hideGit=true` masks it. Either way the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass. Every verification command below is git-free and reads the filesystem or runs the toolchain instead.

## Acceptance Criteria

- [ ] `make precommit` exits 0 — evidence: exit code. This runs `ensure`, `format`, `generate`, `test`, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`, so it covers formatting, generated-code drift, the whole test suite, and the changelog structure check.
- [ ] The accessor honours both branches — evidence, all three required: `go test ./pkg/config/... -v -count=1` exits 0 and its output contains the passing Ginkgo spec names `returns custom topics dir when set` and `returns default 23 Topics when empty`; `grep -n 'GetTopicsDir' pkg/config/config.go` returns ≥1 line (an absent or renamed accessor fails here, which a bare test name does not catch); and the two named specs assert their values rather than only being named — `grep -A6 'returns default 23 Topics when empty' pkg/config/vault_test.go` shows an assertion that calls `GetTopicsDir()` and names `23 Topics` — requiring the call, since the spec name itself contains `23 Topics` and would otherwise satisfy a name-only read — and the custom-value spec's fixture sets a folder that is not `23 Topics` and asserts that value. Spec names alone are satisfied by an empty `It` body and do not count. A single-branch build fails one of the two: an accessor that always returns the literal fails the custom-value spec, one that returns the raw field fails the default spec.
- [ ] The key is parsed from the config file — evidence, both required: the same `go test ./pkg/config/... -v -count=1` run contains the passing spec name `loads topics_dir from YAML`, and `grep -n 'topics_dir' pkg/config/config.go` returns ≥1 line. The spec name proves a config file carrying `topics_dir: "23 Topics"` loads with that value readable; the grep proves the YAML tag exists, which a struct field without a tag would not.
- [ ] The built binary reports the key — evidence: with a temp config whose only vault sets `topics_dir: "23 Topics"`, `make build && ./bin/vault-cli --config <cfg> config list --output json` exits 0 and stdout contains `"topics_dir": "23 Topics"`. The colon is followed by one space: `PrintJSON` uses `SetIndent("", "  ")`, so an unspaced match is a false-failure trap and must not be asserted. Repeatable form: the integration spec named `config list --output json includes topics_dir for a vault that sets it` passes in `go test ./integration/... -v -count=1` (exit 0), where the suite builds the binary with `gexec` and runs it as a subprocess.
- [ ] A vault without the key still works, and reports no key — evidence: with a temp config whose only vault omits `topics_dir`, `./bin/vault-cli --config <cfg> config list --output json` exits 0 and its stdout contains **0** occurrences of `topics_dir` (negative evidence — an always-emit build fails here), and `./bin/vault-cli --config <cfg> config list` exits 0 and still prints that vault's `name<TAB>path` line. Repeatable form: the integration spec named `config list still lists a vault without topics_dir` passes in `go test ./integration/... -v -count=1`.
- [ ] The existing config-list scenario covers the new key — evidence, both required: `grep -c 'topics_dir' scenarios/001-config-list.md` prints ≥2 — one occurrence in the Setup block's YAML config and one in the Action or Expected checklist, since a single mention is a comment and not coverage; and `ls scenarios/*.md | wc -l` prints exactly `5` (negative evidence — no new scenario file was added).
- [ ] The configuration example documents the key — evidence: `grep -n 'topics_dir: "23 Topics"' README.md` returns ≥1 line, inside the fenced YAML block of § Configuration. A bare mention of the word without a value does not satisfy the check.
- [ ] The changelog records the change and no version moved — evidence, all five required: `grep '^## ' CHANGELOG.md | head -1` prints exactly `## Unreleased` (negative evidence — a hand-cut release section would print `## vX.Y.Z` here); `grep -c '^## Unreleased' CHANGELOG.md` prints `1`; `grep -nE '^- feat:.*topics_dir' CHANGELOG.md` returns ≥1 line; `make check-changelog` exits 0; `make check-versions` exits 0, printing `✅ all four versions equal: <version>`. The last two are separate scripts: `check-changelog` guards the section order against the preamble, `check-versions` fails if the three plugin JSON fields were bumped without a matching released changelog section.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0; covers lint, vet, format, generate, the full test suite, check-changelog and license headers.
- `go test ./pkg/config/... -v -count=1` — exits 0; output lists the passing specs `returns custom topics dir when set`, `returns default 23 Topics when empty`, `loads topics_dir from YAML`.
- `go test ./integration/... -v -count=1` — exits 0; output lists the passing specs `config list --output json includes topics_dir for a vault that sets it` and `config list still lists a vault without topics_dir`. The suite builds the binary with `gexec`, so this is a real subprocess run, not an in-process call.
- `grep -n 'topics_dir' pkg/config/config.go` — returns ≥1 line (the YAML tag).
- `grep -n 'GetTopicsDir' pkg/config/config.go` — returns ≥1 line (the accessor).
- `make build && ./bin/vault-cli --config /tmp/topics-check/config.yaml config list --output json` — exits 0; stdout contains `"topics_dir": "23 Topics"` (one space after the colon — `PrintJSON` uses `SetIndent("", "  ")`, so the unspaced form never appears) for a vault that sets it, and contains no `topics_dir` for a vault that omits it. Hand-reproduction of the two integration specs; `bin/` is gitignored.
- `grep -c 'topics_dir' scenarios/001-config-list.md` — prints ≥2 (one occurrence in Setup, one in the checklist); `ls scenarios/*.md | wc -l` — prints `5`.
- `grep -n 'topics_dir: "23 Topics"' README.md` — returns ≥1 line.
- `grep '^## ' CHANGELOG.md | head -1` — prints `## Unreleased`; `grep -c '^## Unreleased' CHANGELOG.md` — prints `1`; `grep -nE '^- feat:.*topics_dir' CHANGELOG.md` — returns ≥1 line.
- `make check-changelog` — exits 0, prints `CHANGELOG structure OK`.
- `make check-versions` — exits 0, prints `✅ all four versions equal: <version>`.

### Operator-executable (runs on the host after the change lands on master)

- `vault-cli --version` matches the released tag, and `claude plugin update vault-cli@vault-cli` reports the same version — the freshness check before anything is observed against the real config. The repo's own CLAUDE.md documents installed-versus-HEAD staleness as a known trap.
- Walk `scenarios/001-config-list.md` with a freshly built binary (`go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .`) and check every box. Per the repo's scenario-skip rule the diff gate will show `.go` hits, so the walk is mandatory rather than skippable. The scenario is operator-side because its Setup hardcodes the host build path, not because it needs the operator's own vaults — it is self-contained (`mktemp -d`, temp config, `touch` templates). It is the end-to-end proof that a freshly built binary serves a real config file.
- `vault-cli config list --output json --vault personal` against the operator's real config — the vault's JSON carries `topics_dir` once the operator sets the key in `~/.config/vault-cli/config.yaml`.

Not part of this spec's gate: the consumer-side outcome — the Personal vault's topic commands resolving the folder from config instead of a hardcoded name — depends on that separate markdown change shipping, and is verified there.

## Desired Behavior

1. A vault's configuration accepts a `topics_dir` key naming the folder that holds that vault's topic pages. The key is optional and per vault; a config with no such key anywhere is valid.
2. Reading a vault's topics directory returns the configured value when it is set and non-empty, and the literal `23 Topics` when the key is absent or empty. It never returns an error and never aborts a command, for any vault.
3. `vault-cli config list --output json` reports `topics_dir` for a vault that sets it, using the same key spelling and the same omit-when-unset behaviour as `tasks_dir`. A vault that does not set it reports no `topics_dir` key at all.
4. Every existing command, flag, output format, and config file behaves exactly as before. A config written before this change loads and resolves identically; `config list` plain output is unchanged; no command's output gains or loses a line.
5. `README.md` § Configuration documents the key in its vault example, and `CHANGELOG.md` gains exactly one `## Unreleased` bullet with a `feat:` prefix describing it. No version string moves and no tag is created — the post-merge releaser converts `## Unreleased` and cuts the tag.
6. `scenarios/001-config-list.md` — the existing scenario that pins the `config list` JSON surface — is extended so its config sets `topics_dir` and its checklist asserts the key, keeping that surface pin honest for the new field. No new scenario file is created.

## Constraints

- The key is spelled `topics_dir` and the accessor is named `GetTopicsDir`; both are frozen because acceptance criteria grep for them.
- The default is the literal `23 Topics` — not a configurable, not a computed, not a documented-as-something-else default.
- The new field mirrors the existing directory fields exactly: same placement among the directory keys in the vault struct, same `omitempty` on both the YAML and JSON tags, same accessor shape (configured value when non-empty, literal otherwise). No new loader branch, no validation, no normalisation, no path expansion, no existence check.
- `pkg/ops/` is not touched. It is a library layer that never writes to stdout, and this change adds no operation.
- No new command, no new subcommand, no new flag. The integration command-registration table in `integration/cli_test.go` therefore gains no entry; the two new integration specs go in a new `Describe` block in that file, since the only existing config touchpoint is the `config list` registration entry and there is no prior `config list` behavior coverage to extend.
- The integration spec names named in the acceptance criteria are frozen: `config list --output json includes topics_dir for a vault that sets it` and `config list still lists a vault without topics_dir`.
- The unit spec names named in the acceptance criteria are frozen: `returns custom topics dir when set`, `returns default 23 Topics when empty`, `loads topics_dir from YAML`.
- Existing tests pass unchanged, including the JSON and YAML marshalling specs in `pkg/config/vault_test.go` and every integration spec.
- The config file is read-only in this change: nothing writes `topics_dir` back, and no migration touches an existing config.
- The changelog entry follows `docs/dod.md`: one bullet under `## Unreleased`, which sits below the preamble and above the newest released section. The bullet starts with `feat:` and names `topics_dir`; its remaining wording is the implementer's, agent decides at impl time.
- The README line goes inside the fenced YAML block of § Configuration, as a key of the `personal` vault; its exact position within that example is the implementer's, agent decides at impl time.
- The integration specs' config fixtures are self-contained temp files; whether they reuse an existing helper or write their own config is the implementer's, agent decides at impl time.
- No `exclude` or `replace` directive is added to `go.mod`, and `go install github.com/bborbe/vault-cli@latest` keeps working.

## Failure Modes

This change reads a config file and prints JSON. It writes no persistent state, opens no network connection, spawns no process, and holds no shared resource, so the concurrency, rate-limiting, resource-exhaustion, and clock-skew categories do not apply.

| Trigger | Expected behavior | Recovery |
|---|---|---|
| `topics_dir` is present but empty (`topics_dir: ""`) | Treated as unset: the accessor returns `23 Topics`, and `config list --output json` omits the key. No error, no empty-string folder name. | None needed. Detectable by the accessor spec named in the acceptance criteria. |
| `topics_dir` names a folder that does not exist in the vault | No error and no abort: this change never stats the folder, it returns the value verbatim. Commands run as before. | Create the folder, or fix the key. Detection happens in the consumer, which finds no pages; vault-cli itself has nothing to report. |
| The config file is malformed YAML | Unchanged behaviour: the loader returns a wrapped parse error naming the config path, and the command exits non-zero. No partial load, and no silent fallback to the default config. | Fix the YAML and re-run. The error names the file. |
| A config written by an older vault-cli, with no `topics_dir` anywhere | Loads and resolves exactly as before; every vault returns the `23 Topics` default. No migration, no prompt, no warning. | None needed. This is the common case. |
| A config carrying keys this binary does not know | Unknown keys are ignored by the YAML decoder, as they are today; the vault still loads. | None needed. Forward compatibility is unchanged by this spec. |
| Two vaults declare different `topics_dir` values | Each vault resolves its own value; commands resolve per vault, so no value crosses vaults. | None needed. |
| The value carries a trailing slash or a nested path | Returned verbatim, exactly like the seven sibling directory keys. No trimming, no joining. | Operator writes the canonical folder name; no normalisation is added. |

## Security / Abuse Cases

- The only new input is an operator-owned string in the operator's own config file under the home directory. No untrusted party supplies it, and no privilege boundary is crossed.
- The value is never used to construct a filesystem path, spawn a process, or interpolate into a shell command by this change: vault-cli returns it and prints it. A value such as `../../etc` therefore introduces no traversal surface here.
- The value is emitted through the existing JSON encoder alongside the other directory keys, so a value containing quotes or newlines is escaped by that encoder rather than breaking the JSON document.
- No credential, token, or secret is read, written, logged, or transmitted, and no new file is opened for reading or writing.
- The vault-local commands that will consume the value are out of scope; whoever writes them owns any containment or validation of the folder name.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Config field + accessor + YAML tag, `config list` JSON surface, the two unit specs, the two integration specs, the `scenarios/001-config-list.md` extension, the README example line, and the `## Unreleased` bullet | 1-6 | 1-8 | — |

Rationale: one config-layer change with one test seam. Splitting it would separate the field from the accessor that reads it, and separate the JSON surface from the scenario that pins it. One prompt.

## Do-Nothing Option

The topic folder stays the one entity directory that cannot be declared per vault. Every vault whose topic pages live outside `23 Topics/` — and every future vault that wants a different name — is served by editing hardcoded folder names inside markdown command files, per vault, with no single place to look and no test to catch a stale one. The cost is small today and grows with each vault that adopts topics: the convention drifts from the config file that documents every other directory, and the drift is invisible because nothing in this repository can see it. The change is additive, backwards-compatible, and touches one struct and one accessor; leaving it undone keeps a documented inconsistency alive for no benefit.
