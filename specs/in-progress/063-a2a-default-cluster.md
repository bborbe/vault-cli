---
status: prompted
approved: "2026-10-20T13:43:46Z"
generating: "2026-10-27T22:56:14Z"
prompted: "2026-10-27T23:02:25Z"
branch: dark-factory/a2a-default-cluster
---

## Summary

- Each vault's config entry can name a default A2A cluster.
- The value is a cluster name such as `nukedev`, never a target string like `k8s:nukedev`, and it is validated as a shape when written but never at read time.
- A new `vault-cli config set-a2a-default-cluster <vault> <cluster>` verb writes the value without hand-editing the config file.
- The value is visible in `config list --output json`, so a consumer that reads the config as JSON can see it.
- A vault that names no default behaves exactly as it does today.

## Problem

Today every vault-cli config entry describes where a vault's files live, but says nothing about where that vault's A2A workers are allowed to run. The consumer — the `claude-supervisor` `open-a2a` call site — therefore has no per-vault answer to fall back to when a task carries no explicit spawn target, and every target-less spawn in every vault inherits one fleet-wide behaviour. That is the isolation hole the parent goal exists to close: a `seibert-brogrammers` task opened from a personal session lands on a nuke cluster its own allowlist forbids, rather than on `octopus-dev` where it belongs. The allowlist on the supervisor side can already say which clusters a vault *may* use; nothing on the vault-cli side can say which one it should use by default, so the consumer has no value to read.

## Goal

A vault-cli config entry can carry an `a2a_default_cluster` value, set through a `config set-a2a-default-cluster` verb, validated as a cluster-name shape on write, and emitted in `config list --output json` — so that a consumer reading the config as JSON can resolve a vault's default A2A cluster without parsing the YAML itself, and a vault that names no default keeps behaving exactly as it does today.

## Non-goals

- **Consuming the value.** Resolving an absent spawn target to the vault default, forming `k8s:<name>`, and refusing an unknown cluster are the `claude-supervisor` call site's work, not this repo's. This spec delivers the stored, validated, readable value only.
- **Enforcing an allowlist.** Which clusters a vault may use is a supervisor-side fact, already carried there. This repo records a default, not a permission.
- **Hardcoding the cluster-name set.** `nukedev`, `nukeprod`, `hell`, `sun`, `octopus-dev`, `octopus-staging`, `octopus-prod` are today's names, not a frozen list. This repo validates the shape only; an unknown name is the consumer's to refuse.
- **Setting the per-vault values themselves.** Seeding each real vault's default is an operator act performed on the operator rung of Verification. This spec's code writes whatever the operator asks it to write.
- **Read-time validation.** A hand-edited config carrying a malformed value must still list. Refusing it would turn a config typo into an unopenable vault.
- **An unset verb.** Removing a default is a config-file edit. If a consumer demands `config unset-a2a-default-cluster`, that is a separate spec.
- **Changing any existing config key** — name, shape, or default.
- **A new scenario file.** `scenarios/001-config-list.md` asserts that the `minimal` vault entry "contains only: `path`, `name`" — an exhaustive key set, not a mere absence check. The new field stays out of that entry because it is `omitempty`, so the assertion still holds; a field without `omitempty` would break it, which is exactly why DB1 requires the omission. No scenario work is in scope.

## Assumptions

- **The consumer reads `config list --output json`, not the YAML.** That is the delivery surface, so the JSON tag spelling is a contract and the YAML tag spelling is the on-disk contract. Both are pinned by an AC.
- **Cluster names are Kubernetes-identifier-shaped.** Every name in today's set is an RFC 1123 label, which is why that shape — not an enumerated list — is the validation rule.
- **A successful write re-serializes the whole file from the config struct.** The write path reads the file into the struct, mutates it, and marshals the struct back. Comments and any key the struct does not model are therefore not preserved, and keys are reordered to struct order. This is the existing baseline verb's behaviour, and it is accepted here rather than re-engineered. "Other keys survive" means other keys **the config struct models**.
- **The per-vault values are seeded by the operator**, on the operator rung, after the change lands. No code in this spec writes a real vault's default.
- **This mirrors an existing feature.** `docs/baseline-file.md` documents the sibling `baseline` key and its `config set-baseline` verb — the same config-key plus set-verb plus doc-page shape — and is the pattern to follow. `docs/dod.md` is the source of the artifact list AC5 checks.

## Acceptance Criteria

Each criterion states the evidence a verifier observes.

- [ ] **AC1 — the JSON surface, three clauses.** Using `vault-cli --config <temp-config> config list --output json`: (a) a vault setting `a2a_default_cluster: nukedev` → exits 0 and stdout contains `"a2a_default_cluster": "nukedev"`; (b) a vault omitting the key → exits 0, stdout contains `"name": "test"`, and stdout contains no `a2a_default_cluster`; (c) a hand-edited vault carrying `a2a_default_cluster: "not a cluster!"` → exits 0 and stdout emits that value verbatim. *Evidence: exit code + stdout substring matches, including the space after the colon (the JSON printer indents). In (b) the positive guard is asserted before the absence, so an empty or failed run cannot satisfy it. (c) is the read-time-tolerance clause.*
- [ ] **AC2 — the verb writes the key, and matches the vault name case-insensitively.** With a temp config whose vault key is spelled `Personal`, `vault-cli --config <temp-config> config set-a2a-default-cluster PERSONAL nukedev` exits 0; the raw file text contains the literal key `a2a_default_cluster: nukedev`; and the map key in the raw file still reads `Personal`. *Evidence: exit code + raw-file grep for both the value and the original key spelling. The raw text is asserted, not a struct field — a symmetric typo in a serialization tag round-trips cleanly while breaking every external reader.*
- [ ] **AC3 — the validator accepts the shape, not a fixed list, and refuses everything else.** `vault-cli --config <temp-config> config set-a2a-default-cluster test my-cluster-2` exits 0 and the raw file contains `a2a_default_cluster: my-cluster-2` — a name that appears nowhere in this spec, so a hardcoded allowlist fails here; a 63-character valid name is likewise accepted. Each of `k8s:nukedev`, `Nukedev`, `nuke_dev`, `-nukedev`, `nukedev-`, `nuke dev`, `nuke.dev`, `nuke/dev`, a non-ASCII name, a 64-character name and the empty string, plus the vault name `nosuchvault`, exits non-zero and leaves the config file byte-identical. *Evidence: exit code + raw-file grep for the accepted names; a byte-for-byte comparison against a pre-run copy for each refusal.*
- [ ] **AC4 — the key appears only where it was set, and everything else is untouched.** After a successful set on vault `test`: `grep -c 'a2a_default_cluster' <temp-config>` returns exactly `1`, and the `other` vault's block carries no `a2a_default_cluster` line — this is what forces the YAML `omitempty`, without which re-serialization injects an empty key into every vault; the file still carries `current_user`, and the written vault still carries its pre-existing `path` and `tasks_dir` values; a second vault `other` in the same file parses to its pre-run `path`, `name` and `tasks_dir` values; and `vault-cli --config <temp-config> --vault test config list` with no `--output json` prints exactly `name<TAB>path` for that one vault, byte-identical to its pre-change output. *Evidence: parsed values compared key by key and vault by vault; the plain listing is scoped to a single vault because the unfiltered listing ranges over a map and its line order is not stable. Values are compared after parsing, not byte-for-byte, because a successful write re-serializes the whole file (see Assumptions).*
- [ ] **AC5 — the repo's own Definition of Done artifacts exist, with content.** For each of `integration/cli_test.go`, `README.md` and `CHANGELOG.md`, `grep -c 'config set-a2a-default-cluster' <file>` returns at least `1`; and `docs/a2a-default-cluster.md` exists and mentions each of `a2a_default_cluster`, `RFC 1123`, and read-time tolerance. *Evidence: one grep per file, each with its own count — a single combined grep exits 0 when any file matches and proves nothing per-file. For the doc, one grep per required term, each returning at least one line; file presence alone is satisfiable by `touch` and would ship the code with the contract undocumented. The artifact list is `docs/dod.md`'s.*
- [ ] **AC6 — the existing suite still passes.** `make precommit` exits 0. *Evidence: exit code.*
- [ ] **Post-Deploy (Rung-2):** after the change is installed, `vault-cli config list --output json` shows `a2a_default_cluster` on all six of these vaults — `seibert-brogrammers`, `seibert-personal`, `seibert-agent` and `seibert-data` reading `octopus-dev`; `private-personal` and `private-trading` reading `nukedev`. No other vault is required to carry the key. *Evidence: stdout, asserted per vault by name. Precondition: the operator-executable rung of `## Verification` has been run — without it the criterion cannot distinguish "not yet seeded" from "feature broken".*
  - `deploy_check:` `set -o pipefail; vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q && git describe --tags --abbrev=0)`

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

```bash
ROOTDIR=/workspace make precommit
```

AC1–AC6 are all reachable here: the JSON surface, the raw-file assertion, the refusal cases, the isolation checks and the artifact grep are each a Ginkgo spec, an integration-table entry, or a plain grep.

### Operator-executable (runs on the host against the real config, after the change lands)

```bash
vault-cli config set-a2a-default-cluster seibert-brogrammers octopus-dev
vault-cli config set-a2a-default-cluster seibert-personal    octopus-dev
vault-cli config set-a2a-default-cluster seibert-agent       octopus-dev
vault-cli config set-a2a-default-cluster seibert-data        octopus-dev
vault-cli config set-a2a-default-cluster private-personal    nukedev
vault-cli config set-a2a-default-cluster private-trading     nukedev
# assert per vault, not by a global count — a count of 6 also passes if one vault
# is missing and a seventh gained the key
vault-cli config list --output json \
  | python3 -c 'import json,sys; [print(v["name"], v.get("a2a_default_cluster")) for v in json.load(sys.stdin)]' \
  | grep -cE '^(seibert-(brogrammers|personal|agent|data) octopus-dev|private-(personal|trading) nukedev)$'   # expect 6
```

Install the released build first: `.maintainer.yaml` sets `release.autoRelease: true` and `.dark-factory.yaml` sets `autoRelease: false`, so `github-releaser-agent` cuts the tag after merge — `git pull && make install` from the released tag, never a hand-cut tag.

The Post-Deploy criterion is operator-only: it writes the operator's real config file, which the container does not have. Until it is run, it is blocked rather than failing.

## Desired Behavior

1. A vault's config entry may carry an A2A default cluster. An entry that omits it is valid, the JSON omits the key entirely, and the default plain-text listing is unchanged.
2. The value is a cluster **name**, not a spawn target: `nukedev`, not `k8s:nukedev`. A value containing `:` is refused, because a caller forms the target from the name.
3. The value is validated as an RFC 1123 label — lowercase alphanumerics and interior hyphens, at most 63 characters — the shape a Kubernetes cluster identifier takes. Empty, uppercase, underscore, space, dot, slash, non-ASCII, a leading or trailing hyphen, and over-length values are refused.
4. `config set-a2a-default-cluster <vault> <cluster>` writes the value onto the named vault's entry, matching the vault name case-insensitively and preserving the file's own key spelling.
5. A refused value or an unknown vault writes nothing at all — the config file is byte-identical afterwards, the same contract the existing baseline verb already honours.
6. A successful write leaves every other key **the config struct models** and every other vault untouched.
7. Reading never validates. A malformed value that is already on disk is emitted verbatim; refusing an unknown cluster is the consumer's job, never this repo's list path.

## Constraints

- **The on-disk key is `a2a_default_cluster`.** Both the YAML and JSON spellings are part of the contract — the JSON spelling is what the consumer reads.
- **The writer is additive.** No existing key changes shape, name or default; a config with no A2A key round-trips unchanged.
- **The write is atomic and all-or-nothing**, reusing the existing config-file write path rather than introducing a second one. It re-serializes the whole file from the struct, so comments and unmodelled keys are not preserved — accepted, and documented in the new `docs/` page.
- **The new verb's registration lives where the existing config subcommands are registered**, not in the file that builds the individual command.
- **One `func TestSuite` per package.** The packages already have exactly one; a new test file adds `Describe` blocks only.
- **Error style is the repo's**, `github.com/bborbe/errors`, matching the existing baseline validator and writer.
- **No new dependency.** Everything needed is already imported.
- **Concurrent writers are out of scope.** The atomic replace means a race loses one write rather than corrupting the file, and the operator does not run concurrent writers against one config.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Cluster name contains `:` (a target string) | Refused before any write; file byte-identical | Operator re-runs with the bare name |
| Cluster name uppercase, underscore, space, dot, slash, non-ASCII, edge hyphen, or >63 chars | Refused before any write; file byte-identical | Operator re-runs with a valid name |
| Empty cluster name | Refused before any write; file byte-identical | Operator supplies a name |
| Vault name not in the config | Refused; file byte-identical | Operator checks `config list` for the exact name |
| Config file missing or unparseable | Refused; nothing written, nothing created | Operator fixes or restores the config file |
| Config directory not writable, or the disk is full | Refused; original file byte-identical, no temp file left behind | Operator fixes permissions or space, then re-runs; `ls <configdir>/.config-*.yaml` returns nothing |
| Hand-edited malformed value already on disk | `config list` still exits 0 and emits it verbatim | Consumer refuses the unknown name; the vault stays openable |
| Interrupted write | Atomic replace leaves either the old file or the new one, never a truncated merge | Re-run the verb |

## Security / Abuse

The verb takes a vault name and a cluster name from argv and writes them into a user-owned config file. The validator is the input boundary: it constrains the cluster value to `[a-z0-9-]` at 63 characters or fewer, so no value written through this verb can carry a path separator, a shell metacharacter, a newline, or a colon that would let a name impersonate a target string. The vault name is matched against keys already present in the config and never used to construct a path, so an unknown vault is a refusal rather than a file operation. The file is written through the existing atomic temp-file-and-rename path, so an interrupted write cannot leave a partially merged config that a later reader would parse as a different vault's settings.

## Suggested Decomposition

Two packages (`pkg/config` for the field, validator and writer; `pkg/cli` for the verb and its registration), one feature path, seven desired behaviors. One prompt:

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Config field, shape validator, atomic writer, CLI verb, JSON surface, tests, `docs/` page, README and CHANGELOG | 1–7 | 1–6 | — |

AC7 is operator-executable and runs after the change lands, not as a prompt.

## Do-Nothing Option

Without this, the `claude-supervisor` `open-a2a` call site has no per-vault default to read, so the parent goal's isolation criterion cannot be met at all: every target-less spawn keeps inheriting one fleet-wide behaviour, and a vault's allowlist has nothing to be checked against. The cost is not a missing convenience but a blocked goal — the allowlist task can only refuse a forbidden cluster once a default exists to be checked. The alternative, hardcoding the vault-to-cluster mapping in the consumer, puts a second copy of the mapping in a repo that cannot see the vault config, which is exactly the drift this spec exists to avoid.
