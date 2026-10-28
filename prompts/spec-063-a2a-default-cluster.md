---
status: draft
spec: [063-a2a-default-cluster]
created: "2026-10-27T22:58:35Z"
branch: dark-factory/a2a-default-cluster
---

# Store, validate and expose a vault's default A2A cluster (spec 063)

<summary>
- Each vault's config entry can name a default A2A cluster.
- The value is a bare cluster name such as `nukedev`, never a spawn target such as `k8s:nukedev`.
- The value's shape is validated when written, as an RFC 1123 label of at most 63 characters.
- A new config subcommand writes the value for a named vault without hand-editing the file.
- The vault name is matched case-insensitively, and the file's own key spelling is preserved.
- A refused value or an unknown vault leaves the config file byte-identical.
- The value appears in `config list --output json`; a vault that names none emits no key at all.
- Reading never validates: a hand-edited malformed value still lists, emitted verbatim.
- Every other modelled key and every other vault survive a write unchanged.
- The branch already carries this feature; this prompt verifies it against the contract and closes gaps instead of adding a second implementation.
</summary>

<objective>
Deliver a per-vault `a2a_default_cluster` value that is stored in the config file, validated on write as a Kubernetes-identifier shape, and emitted in `config list --output json`, so the `claude-supervisor` `open-a2a` call site can resolve a vault's default without parsing YAML. Consuming the value, forming `k8s:<name>`, and enforcing an allowlist are the consumer's work and stay out of this repo.
</objective>

<context>
Read `CLAUDE.md` for project conventions, then read fully — in this order — before changing anything:

- `docs/baseline-file.md` — the sibling config-key + set-verb + doc-page shape this feature mirrors.
- `docs/a2a-default-cluster.md` — the contract page for this feature. It already exists; treat it as the target and verify it, do not rewrite it from scratch.
- `docs/dod.md` — the artifact list the Definition of Done checks.
- `pkg/config/config.go` — `type Config struct` (`Vaults map[string]Vault`), `type Vault struct`, the existing `Baseline` field, and `func resolveConfigPath(ctx context.Context, configPath string) (string, error)`.
- `pkg/config/baseline.go` — the exemplar to mirror end to end: `ValidateBaselinePath`, `BaselineWriter` / `NewBaselineWriter` / `SetBaseline`, `readConfigFile`, `findVaultKey`, `writeFileAtomic`. Note the ordering: validate → resolve path → read → find key → mutate → marshal → atomic write.
- `pkg/config/a2a_default_cluster.go` — the existing implementation of this feature: `a2aClusterName`, `a2aClusterNameMaxLength`, `ValidateA2ADefaultCluster`, `A2ADefaultClusterWriter`, `NewA2ADefaultClusterWriter`, `a2aDefaultClusterWriter.SetA2ADefaultCluster`.
- `pkg/config/a2a_default_cluster_test.go` — the existing validator and writer specs.
- `pkg/config/config_suite_test.go` — the package's single `func TestSuite(t *testing.T)`.
- `pkg/cli/config.go` — `createConfigSetBaselineCommand` (the exemplar leaf) and `createConfigSetA2ADefaultClusterCommand` (the existing leaf).
- `pkg/cli/cli.go` — `NewRootCommand`: `configCmd.AddCommand(...)` is the registration site; and `configListEntry` plus `createConfigListCommand`, which owns the JSON projection and the plain `name<TAB>path` listing.
- `pkg/cli/config_test.go` — the existing `Describe("vault-cli config set-a2a-default-cluster", ...)` block.
- `pkg/cli/cli_suite_test.go` — the package's single `func TestSuite(t *testing.T)`.
- `integration/cli_test.go` — the command-registration table (`Entry("config set-a2a-default-cluster", ...)`) and the neighbouring `config list` specs.
- `integration/integration_suite_test.go` — `func TestIntegration(t *testing.T)` and the `gexec.Build` before-suite.
- `README.md` around the `config set-baseline` / `config set-a2a-default-cluster` usage block, and `CHANGELOG.md`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → struct → method.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega and one `TestSuite` per package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` (`errors.Errorf`, `errors.Wrap`, `errors.Wrapf`), never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits and license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — doc comments on exported symbols.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` format and `feat:` prefix.

**Pre-flight state.** The working tree already contains a complete implementation of this feature, released as `v0.177.0` in `CHANGELOG.md`. Every symbol named above exists and already matches most of the contract. Treat this prompt as implement-or-confirm: read each cited file first, verify the contract below, and change only what diverges. Do **not** add a second field, a second validator, a second writer, or a second verb. If a cited symbol is missing, add it in the file named here; if it is present, leave its name and package alone.
</context>

<requirements>
1. **Config field (DB1).** In `pkg/config/config.go`, `type Vault struct` must carry exactly one field for this value, placed beside `Baseline`:

   ```go
   // A2ADefaultCluster is the cluster NAME (nukedev, not k8s:nukedev) that an
   // A2A spawn for this vault falls back to when the task carries no explicit
   // target. Empty means the vault names no default: the consumer keeps its own
   // fallback behaviour. The value is validated when written and never when
   // read, so a hand-edited config carrying a malformed value still lists.
   A2ADefaultCluster string `yaml:"a2a_default_cluster,omitempty" json:"a2a_default_cluster,omitempty"`
   ```

   Both tags are `omitempty`, and both spellings are literally `a2a_default_cluster`. The YAML omission is what keeps `scenarios/001-config-list.md`'s exhaustive `path`, `name` key set for the `minimal` vault true after a re-serialization; without it every vault gains an empty key. Do not change any other field's name, shape or default.

2. **Shape validator (DB2, DB3, Failure Modes).** In `pkg/config/a2a_default_cluster.go`, `func ValidateA2ADefaultCluster(ctx context.Context, cluster string) error` is the input boundary. It must:
   - reject the empty string first;
   - reject `len(cluster) > 63`;
   - accept iff the value matches `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` (the RFC 1123 label shape);
   - return errors built with `github.com/bborbe/errors` (`errors.Errorf(ctx, ...)`), naming the offending value.

   The rule is a shape, not a list: `my-cluster-2`, `a`, `0`, `octopus-dev`, `nukedev` and any 63-character label are accepted; `nukeprod`, `hell`, `sun`, `octopus-staging` and `octopus-prod` are accepted because they are well-formed, not because they are enumerated. `k8s:nukedev`, `Nukedev`, `nuke_dev`, `-nukedev`, `nukedev-`, `nuke dev`, `nuke.dev`, `nuke/dev`, a non-ASCII name, a 64-character name and the empty string are refused. A colon is refused specifically because a caller forms `k8s:<name>` from this value, so a stored target string would let a name impersonate a target.

3. **Atomic writer (DB4, DB5, DB6, Failure Modes).** In `pkg/config/a2a_default_cluster.go`:
   - `type A2ADefaultClusterWriter interface { SetA2ADefaultCluster(ctx context.Context, vaultName string, cluster string) error }`
   - `func NewA2ADefaultClusterWriter(configPath string) A2ADefaultClusterWriter`
   - the private struct implements `SetA2ADefaultCluster` with exactly the `baselineWriter.SetBaseline` sequence: validate the cluster **before touching the file**, `resolveConfigPath`, `readConfigFile`, `findVaultKey`, mutate the vault copy, `yaml.Marshal(cfg)`, `writeFileAtomic`.
   - `findVaultKey` already matches `strings.EqualFold` and returns the file's own map key, so `PERSONAL` writes onto a vault keyed `Personal` without renaming it. Reuse it; do not add a second lookup.
   - `writeFileAtomic` already writes a `.config-*.yaml` temp file in the config's own directory, closes it, renames it over the target, and removes the temp file on every failure path. Reuse it; do not introduce a second write path.
   - On any failure — invalid cluster, unknown vault, missing or unparseable config, unwritable directory, full disk, failed rename — the original file is byte-identical and no temp file remains. Nothing is created when the config file is missing.

4. **CLI verb and its registration (DB4).** In `pkg/cli/config.go`, `func createConfigSetA2ADefaultClusterCommand(ctx context.Context, configPath *string) *cobra.Command` returns a leaf with `Use: "set-a2a-default-cluster <vault> <cluster>"`, `Args: cobra.ExactArgs(2)`, and a `RunE` that constructs `config.NewA2ADefaultClusterWriter(*configPath)`, calls `SetA2ADefaultCluster(ctx, args[0], args[1])`, and wraps a failure with `errors.Wrap(ctx, err, "set a2a default cluster")` from `github.com/bborbe/errors`.

   Register it in `pkg/cli/cli.go` inside `NewRootCommand`, on `configCmd`, immediately after `createConfigSetBaselineCommand`. That is the registration site the spec names — do not register it inside `pkg/cli/config.go`, and do not add a second `config` command tree.

5. **JSON and plain listing (DB1, DB7).** `configListEntry` in `pkg/cli/cli.go` embeds `*config.Vault`, so the struct's `json:"a2a_default_cluster,omitempty"` tag flows into `config list --output json` with no extra projection field. Do not add one. The plain listing stays `fmt.Printf("%s\t%s\n", vault.Name, vault.Path)` — this feature changes no plain output. `Load`, `GetVault` and `GetAllVaults` must not call the validator: a hand-edited `a2a_default_cluster: "not a cluster!"` still lists and is emitted verbatim.

6. **Tests — config package.** Add cases to `pkg/config/a2a_default_cluster_test.go` inside its existing `Describe` blocks; do not add a `func TestSuite`. Cover:
   - validator accept table: `nukedev`, `nukeprod`, `octopus-dev`, `my-cluster-2`, `a`, `0`, `cluster123`, `strings.Repeat("a", 63)`;
   - validator reject table: `k8s:nukedev`, `Nukedev`, `nuke_dev`, `-nukedev`, `nukedev-`, `nuke dev`, `nuke.dev`, `nuke/dev`, a non-ASCII value, `strings.Repeat("a", 64)`, and `""`;
   - writer happy path: value readable through `config.NewLoader(configPath).GetVault`, and the raw file contains the literal `a2a_default_cluster: nukedev`;
   - writer preserves the vault's other keys (`path`, `tasks_dir`) and the file's top-level `current_user` / `default_vault`;
   - mixed-case vault key: writing `PERSONAL` leaves the raw file's key spelled `Personal` and adds `a2a_default_cluster: nukedev`;
   - two-vault fixture: after setting `test`, `strings.Count(content, "a2a_default_cluster") == 1`, and `other` still parses to its pre-run `path` and `tasks_dir` with an empty `A2ADefaultCluster`;
   - refusal leaves the file byte-identical for each of: a malformed cluster, an unknown vault, and an unparseable config;
   - a successful write leaves exactly one entry in the config directory (no `.config-*.yaml` temp file).

7. **Tests — CLI package and binary level.** In `pkg/cli/config_test.go`, extend the existing `Describe("vault-cli config set-a2a-default-cluster", ...)` block (no new `TestSuite`) to cover, through `cli.Run`:
   - persists the value and writes the literal raw key;
   - `config list --output json` contains `"a2a_default_cluster": "nukedev"` (including the space after the colon);
   - a vault with no default: assert the positive guard `"name": "test"` **before** asserting the absence of `a2a_default_cluster`;
   - a hand-edited malformed value is emitted verbatim;
   - `my-cluster-2` is accepted and appears in the raw file — a name that appears nowhere in the spec, so a hardcoded allowlist fails this case;
   - a 63-character valid name is accepted;
   - each refusal in the reject table above, plus vault `nosuchvault`, exits non-zero and leaves the file byte-identical against a pre-run copy;
   - after a successful set, `cli.Run` of `config list --vault test` (no `--output json`) prints exactly `test<TAB><path>` for that one vault — scope it to one vault because the unfiltered listing ranges over a map and its line order is not stable;
   - a missing cluster argument exits non-zero (`cobra.ExactArgs(2)`).

   In `integration/cli_test.go`, keep exactly one `Entry("config set-a2a-default-cluster", "config", "set-a2a-default-cluster")` in the existing command-registration table. Do not add a second `func TestIntegration` or a second suite.

8. **Documentation and release artifacts (AC5).** Verify, and fill only if absent:
   - `docs/a2a-default-cluster.md` exists and contains the literal `a2a_default_cluster`, the literal `RFC 1123`, and a read-time-tolerance statement (the file already has a `## Read-time tolerance` section saying reading never validates);
   - `README.md` mentions `vault-cli config set-a2a-default-cluster <vault> <cluster>` in the config usage block;
   - `CHANGELOG.md` contains the literal `config set-a2a-default-cluster`. The `## v0.177.0` entry already covers this feature — do **not** duplicate it under `## Unreleased`; add an `## Unreleased` `feat:` bullet only if the changelog carries no entry for the feature at all.

   <!-- OPEN QUESTION (for the human reviewer): the working tree already contains this feature and a released v0.177.0 changelog entry, so the AC5 changelog grep is already satisfied. If this spec is being re-delivered rather than closed, decide whether a fresh `## Unreleased` bullet is wanted; adding one would duplicate a released entry. -->

9. **Self-check before finishing.** Re-run the `<verification>` block and confirm every count. Walk DB1–DB7 and AC1–AC6 of the spec against the change, including the read-time-tolerance clause (a malformed on-disk value must still list) and the isolation clause (the key appears only on the vault it was set for).
</requirements>

<constraints>
- The on-disk key is `a2a_default_cluster`. Both the YAML and JSON spellings are part of the contract; the JSON spelling is what the consumer reads.
- The writer is additive: no existing key changes shape, name or default, and a config with no A2A key round-trips unchanged.
- The write is atomic and all-or-nothing, reusing `writeFileAtomic`; do not introduce a second write path.
- The new verb's registration lives where the existing config subcommands are registered (`NewRootCommand` in `pkg/cli/cli.go`), not in the file that builds the individual command.
- One `func TestSuite` per package. The packages already have exactly one; new test files or blocks add `Describe` blocks only.
- Error style is the repo's: `github.com/bborbe/errors`, matching the existing baseline validator and writer. Never `fmt.Errorf` and never `context.Background()` inside `pkg/`.
- No new dependency. Everything needed is already imported.
- No read-time validation: a malformed value already on disk is emitted verbatim. Refusing an unknown cluster is the consumer's job.
- No unset verb. Removing a default is a config-file edit and is a separate spec.
- No scenario work. Do not edit `scenarios/001-config-list.md` or add a scenario file.
- Do not hardcode the cluster-name set; validate the shape only.
- Do not seed any real vault's default; that is an operator act after the change lands.
- Concurrent writers are out of scope. The atomic replace loses one write rather than corrupting the file.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

<!-- OPEN QUESTION (for the human reviewer): AC4's byte-identical plain-listing assertion and AC3's 63/64-character acceptance are currently proven at validator/writer level but not as binary-level integration specs. Requirement 7 adds them through `cli.Run`; if you prefer them in `integration/cli_test.go` instead, move them there — do not add both, which duplicates coverage. -->
</constraints>

<verification>
EVIDENCE GREPS — run each and confirm the count. Absence assertions use `! grep -q` because `grep -c` exits 1 when the count is 0.

```
grep -c 'A2ADefaultCluster string `yaml:"a2a_default_cluster,omitempty" json:"a2a_default_cluster,omitempty"`' pkg/config/config.go   # 1
grep -c 'func ValidateA2ADefaultCluster' pkg/config/a2a_default_cluster.go          # 1
grep -c 'func NewA2ADefaultClusterWriter' pkg/config/a2a_default_cluster.go         # 1
grep -c 'func createConfigSetA2ADefaultClusterCommand' pkg/cli/config.go            # 1
grep -c 'createConfigSetA2ADefaultClusterCommand' pkg/cli/cli.go                    # 1
grep -c 'Entry("config set-a2a-default-cluster"' integration/cli_test.go            # 1
grep -c 'config set-a2a-default-cluster' README.md                                  # >= 1
grep -c 'config set-a2a-default-cluster' CHANGELOG.md                               # >= 1
grep -c 'a2a_default_cluster' docs/a2a-default-cluster.md                           # >= 1
grep -c 'RFC 1123' docs/a2a-default-cluster.md                                      # >= 1
grep -ci 'read-time tolerance' docs/a2a-default-cluster.md                          # >= 1
```

ABSENCE / FROZEN GUARDS:

```
! grep -q 'a2a_default_cluster' scenarios/001-config-list.md        # the scenario stays untouched
! grep -rq 'unset-a2a-default-cluster' pkg/                         # no unset verb
! grep -rq 'func TestSuite' pkg/config/a2a_default_cluster_test.go  # no second suite in the new file
```

TESTS:

```
go test ./pkg/config/... ./pkg/cli/... -ginkgo.focus='A2A|a2a' -ginkgo.v
go test ./pkg/config/... ./pkg/cli/...
ROOTDIR=/workspace make precommit
```

⚠️ `-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focus run, confirm the A2A spec names appear in the output; a log without them is a failure, not a pass.

`ROOTDIR=/workspace make precommit` must exit 0. `.git` is masked in the container, so do not use git to inspect or restore anything; use filesystem checks only.
</verification>
