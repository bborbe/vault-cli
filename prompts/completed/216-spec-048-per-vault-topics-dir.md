---
status: completed
spec: [048-per-vault-topics-dir]
summary: Added the per-vault topics_dir config key with GetTopicsDir accessor defaulting to 23 Topics, pinned by unit specs, real-binary integration specs, README, scenario and changelog updates
execution_id: vault-cli-topics-exec-216-spec-048-per-vault-topics-dir
dark-factory-version: v0.193.0
created: "2026-09-15T12:52:00Z"
queued: "2026-09-15T13:39:32Z"
started: "2026-09-15T13:40:33Z"
completed: "2026-09-15T13:45:04Z"
---

# Per-vault topics directory: `topics_dir` config key, `GetTopicsDir` accessor, `config list` JSON surface, docs

<summary>
- A vault's configuration can declare where its topic pages live, exactly as it can already declare its tasks, goals, themes, objectives, vision, daily-notes and knowledge folders.
- A vault that does not declare it keeps working unchanged and resolves its topic folder to `23 Topics`.
- The resolved value reaches consumers through `vault-cli config list --output json`, which reports the key for a vault that sets it and omits it for a vault that does not — the same behaviour the sibling directory keys already have.
- No new command, no new flag, no new subcommand: the command surface, the plain-text output and every existing config file behave exactly as they do today.
- Nothing is migrated, nothing is written back to a config file, and no folder is created, stat-ed or validated.
- The configuration example in the README documents the new key.
- The existing config-list scenario is extended so its config sets the key and its checklist asserts it; no new scenario file is added.
- The changelog records the change under `## Unreleased`; no version string is bumped and no tag is created.
- Both branches of the lookup are pinned by tests that assert the returned value, and the JSON surface is pinned by a test that runs the real binary as a subprocess.
</summary>

<objective>
Add the eighth vault directory key — `topics_dir`, read through `GetTopicsDir` — so a vault's topic folder is declared in the one config file that declares every other entity directory, instead of being resolved by convention. The default is the literal `23 Topics`, the value surfaces through `config list --output json` exactly as `tasks_dir` does, and nothing else about vault-cli changes. This is spec 048's only prompt: it covers Desired Behaviors 1-6 and Acceptance Criteria 1-8.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files:

- `pkg/config/config.go` — the file you change. Read the `Vault` struct (its directory-key block runs `TasksDir` → `KnowledgeDir`), the seven sibling accessors (`GetTasksDir`, `GetGoalsDir`, `GetThemesDir`, `GetObjectivesDir`, `GetVisionDir`, `GetDailyDir`, `GetKnowledgeDir`) and `expandVaultPaths` — it copies the vault struct, and both `GetVault` and `GetAllVaults` return that copy, so a new field survives every resolution path without a loader change.
- `pkg/config/vault_test.go` — the sibling test pairs, `Describe("GetTasksDir")` through `Describe("GetKnowledgeDir")`; your new specs copy that shape exactly.
- `pkg/config/config_test.go` — the `Describe("Loader")` / `Describe("Load")` tree, in particular the `knowledge_dir in vault config` context; `loads topics_dir from YAML` mirrors its sibling.
- `pkg/cli/cli.go` — `createConfigListCommand`. JSON mode returns `PrintJSON(vaults)` over the resolved `[]*config.Vault`; plain mode prints `fmt.Printf("%s\t%s\n", vault.Name, vault.Path)`. READ ONLY — you do not change this file, and `buildWatchTargets` stays as it is.
- `pkg/cli/output.go` — `PrintJSON` encodes with `json.NewEncoder(os.Stdout)` after `SetIndent("", "  ")`. That is why the JSON is spaced: the output carries `"topics_dir": "23 Topics"` with one space after the colon, and the compact form `"topics_dir":"23 Topics"` never appears in this output. (Compact JSON does appear in `watch` events and `task … --output json` bodies — different encoders, do not copy their assertion style here.)
- `integration/cli_test.go` — the harness: `createTempVault` and `createTempVaultWithGoals` (temp vault dir + temp config file), the `gexec.Start` + `Eventually(session).Should(gexec.Exit(0))` style, the `gbytes` usage, and the outer `Describe("vault-cli integration tests", …)` that wraps every block.
- `integration/integration_suite_test.go` — `binPath` is built once in `BeforeSuite` with `gexec.Build("github.com/bborbe/vault-cli")`; the specs therefore run the real binary as a subprocess.
- `scenarios/001-config-list.md` — the scenario this change extends.
- `README.md` § Configuration — the fenced YAML example you add one line to.
- `CHANGELOG.md` — `# Changelog` title, the `All notable changes…` preamble, the `* MAJOR / MINOR / PATCH` lines, then `## v0.132.2`. There is no `## Unreleased` section yet.
- `docs/dod.md` — this repo's `validationPrompt`; note its changelog-placement rule.
- `specs/in-progress/048-per-vault-topics-dir.md` — the spec. Read its Acceptance Criteria, Constraints, Failure Modes and Non-goals sections; every requirement below comes from them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format and the `## Unreleased` placement rule.
- `/home/node/.claude/plugins/marketplaces/coding/docs/markdown-todo-guide.md` — checkbox syntax for the scenario checklist.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

Current state of the pieces you copy, quoted verbatim:

The struct's directory-key block, with `ClaudeScript` following it:

```go
	KnowledgeDir      string   `yaml:"knowledge_dir,omitempty"        json:"knowledge_dir,omitempty"`
	ClaudeScript      string   `yaml:"claude_script,omitempty"        json:"claude_script,omitempty"`
```

The sibling accessor shape:

```go
// GetKnowledgeDir returns the knowledge base directory, defaulting to "50 Knowledge Base" if not set.
func (v *Vault) GetKnowledgeDir() string {
	if v.KnowledgeDir != "" {
		return v.KnowledgeDir
	}
	return "50 Knowledge Base"
}
```

The sibling test-pair shape:

```go
	Describe("GetKnowledgeDir", func() {
		It("returns custom knowledge dir when set", func() {
			vault := &config.Vault{KnowledgeDir: "Custom Knowledge"}
			Expect(vault.GetKnowledgeDir()).To(Equal("Custom Knowledge"))
		})

		It("returns default 50 Knowledge Base when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetKnowledgeDir()).To(Equal("50 Knowledge Base"))
		})
	})
```

The sibling YAML-load spec:

```go
		Context("knowledge_dir in vault config", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    knowledge_dir: "50 Knowledge"
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("loads knowledge_dir from YAML", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.KnowledgeDir).To(Equal("50 Knowledge"))
				Expect(vault.GetKnowledgeDir()).To(Equal("50 Knowledge"))
			})
		})
```

Two environment facts that shape this prompt:

1. **Make no git calls, anywhere — every check in `<verification>` is git-free.** The daemon does not check `<verification>` exit codes, so a git command that dies (for example `fatal: not a git repository` under `workflow: worktree` or `hideGit: true`) reports a false pass. This repo currently runs `workflow: direct` without `hideGit`, so `.git` may well be visible — do not rely on it either way, and never use git to verify this change.
2. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** Keep it: `gexec.Build` in the integration suite runs `go build`, which otherwise tries to read the masked `.git` and fails with a VCS status error.
</context>

<requirements>

## 0. Scope — exactly seven files, no others

- `pkg/config/config.go` — one struct field, one accessor.
- `pkg/config/vault_test.go` — the two accessor specs.
- `pkg/config/config_test.go` — the YAML-load spec.
- `integration/cli_test.go` — one helper, one `Describe` block with two specs.
- `README.md` — one line inside the § Configuration example.
- `scenarios/001-config-list.md` — the config key plus the checklist assertions.
- `CHANGELOG.md` — one bullet under a new `## Unreleased`.

Nothing else. `pkg/cli/cli.go` is read-only, `pkg/ops/` is untouched (this change adds no operation and `pkg/ops/` never writes to stdout), `go.mod` is untouched (no new dependency), the command-registration table in `integration/cli_test.go` gains no entry (no new command), and no file is added or removed anywhere. `pkg/storage/storage.go` is untouched too — its `Config` mirrors only the directory keys that have in-repo consumers, and nothing in this repository reads a topics directory, so a `TopicsDir` field there would be dead code.

## 1. `pkg/config/config.go` — the field

Add exactly this line to the `Vault` struct, immediately after the `KnowledgeDir` line and immediately before the `ClaudeScript` line. The column alignment of the `yaml:` and `json:` tag parts must match the surrounding block byte-for-byte:

```go
	TopicsDir         string   `yaml:"topics_dir,omitempty"           json:"topics_dir,omitempty"`
```

- Field name `TopicsDir`, key spelling `topics_dir`, `omitempty` on both tags. Both are frozen.
- Do NOT reorder or re-align any other field. Do NOT add a second field, a validation tag, or a comment above the line.

## 2. `pkg/config/config.go` — the accessor

Add this function immediately after `GetKnowledgeDir` and before `GetExcludes`:

```go
// GetTopicsDir returns the topics directory, defaulting to "23 Topics" if not set.
func (v *Vault) GetTopicsDir() string {
	if v.TopicsDir != "" {
		return v.TopicsDir
	}
	return "23 Topics"
}
```

Non-negotiable properties:

- No error return, no second parameter, no receiver other than `*Vault` — it mirrors its seven siblings exactly.
- No validation, no normalisation, no `~` expansion, no trailing-slash trimming, no `filepath.Join`, no `os.Stat`, no existence check, no logging. The body contains exactly two `return` statements and references no package outside the file's existing imports.
- `23 Topics` is the single literal. Do NOT add a constant, a configurable default, a per-vault override of the default, an environment variable, an opt-out flag, or a second fallback value.
- Do NOT touch `Load`, `expandVaultPaths`, `GetVault`, `GetAllVaults`, `GetVaultPath`, `getDefaultConfig`, the `Loader` interface or the counterfeiter directive. The struct copy in `expandVaultPaths` already carries the new field; no loader code changes for this feature.

## 3. `pkg/config/vault_test.go` — the two accessor specs

Add this `Describe` block between the existing `Describe("GetKnowledgeDir", …)` block and the existing `Describe("GetClaudeScript", …)` block, verbatim:

```go
	Describe("GetTopicsDir", func() {
		It("returns custom topics dir when set", func() {
			vault := &config.Vault{TopicsDir: "Custom Topics"}
			Expect(vault.GetTopicsDir()).To(Equal("Custom Topics"))
		})

		It("returns default 23 Topics when empty", func() {
			vault := &config.Vault{}
			Expect(vault.GetTopicsDir()).To(Equal("23 Topics"))
		})
	})
```

- The two spec names are frozen: `returns custom topics dir when set` and `returns default 23 Topics when empty`.
- Each `It` must contain its `Expect(vault.GetTopicsDir())` assertion. A spec that is only named — an empty `It` body, or one whose body never calls the accessor — does not count and fails the acceptance criteria. The default spec's own name contains `23 Topics`, so the name alone can never be the evidence.
- The custom fixture's folder must not be `23 Topics`; `Custom Topics` follows the file's convention (`Custom Tasks`, `Custom Goals`, `Custom Themes`, `Custom Objectives`, `Custom Vision`, `Custom Daily`, `Custom Knowledge`) and is the value the assertion must name.
- Do NOT touch the existing `GetKnowledgeDir` or `GetClaudeScript` blocks, and do NOT add a JSON- or YAML-marshalling spec for `topics_dir` — the JSON surface is pinned by the subprocess specs in section 4, and the existing marshalling specs must pass unchanged.

## 3b. `pkg/config/config_test.go` — the YAML-load spec

Add this `Context` inside `Describe("Loader")` → `Describe("Load")`, directly after the existing `missing knowledge_dir in vault config` context. It is the sibling block quoted in `<context>`, with `knowledge_dir` → `topics_dir`, `KnowledgeDir` → `TopicsDir`, `50 Knowledge` → `23 Topics`:

```go
		Context("topics_dir in vault config", func() {
			BeforeEach(func() {
				configData := `vaults:
  main:
    name: main
    path: /vault/main
    topics_dir: "23 Topics"
`
				err := os.WriteFile(configPath, []byte(configData), 0600)
				Expect(err).To(BeNil())
				loader = config.NewLoader(configPath)
			})

			It("loads topics_dir from YAML", func() {
				vault, err := loader.GetVault(ctx, "main")
				Expect(err).To(BeNil())
				Expect(vault.TopicsDir).To(Equal("23 Topics"))
			})
		})
```

- The spec name `loads topics_dir from YAML` is frozen.
- The `Expect(vault.TopicsDir)` assertion is mandatory — it is the only assertion that proves the YAML tag is wired; `GetTopicsDir()` alone would pass through the default even if the tag were missing.
- Do NOT modify the existing `knowledge_dir` contexts.

## 4. `integration/cli_test.go` — one helper and two subprocess specs

### 4a. The helper

Add a third temp-vault helper next to `createTempVault` / `createTempVaultWithGoals`, following their shape exactly (`os.MkdirTemp` for the vault, `os.MkdirAll` for its `Tasks` directory, `os.CreateTemp` for the config, a cleanup that removes both):

```go
// createTempVaultWithTopics creates a temporary vault whose config sets
// topics_dir, and returns the vault path, the config path and a cleanup func.
func createTempVaultWithTopics(topicsDir string) (vaultPath string, configPath string, cleanup func()) {
```

Its config content is this YAML — the block below shows the rendered result of the `createTempVaultWithTopics("23 Topics")` call. Write it with `fmt.Sprintf`, interpolating the vault path into `path: %s` and the `topicsDir` argument into `topics_dir: "%s"`:

```yaml
default_vault: test
vaults:
  test:
    name: test
    path: <vaultPath>
    tasks_dir: Tasks
    topics_dir: "23 Topics"
```

`createTempVault` and `createTempVaultWithGoals` must NOT gain a `topics_dir` line — the second spec below depends on their configs staying topics_dir-free, and every other spec in the file depends on their shape staying unchanged.

### 4b. The `Describe` block

Add a new inner `Describe` block inside the outer `Describe("vault-cli integration tests", …)`, alongside the existing blocks (after `Describe("command registration", …)`). Its two spec names are frozen. Both specs start the real binary through `gexec.Start` and both call the helper's `cleanup` via `defer`.

```go
	Describe("vault-cli config list --output json topics_dir", func() {
		It("config list --output json includes topics_dir for a vault that sets it", func() {
			_, configPath, cleanup := createTempVaultWithTopics("23 Topics")
			defer cleanup()

			cmd := exec.Command(binPath, "--config", configPath, "config", "list", "--output", "json")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			out := string(session.Out.Contents())
			Expect(out).To(ContainSubstring(`"name": "test"`))
			Expect(out).To(ContainSubstring(`"topics_dir": "23 Topics"`))
		})

		It("config list still lists a vault without topics_dir", func() {
			vaultPath, configPath, cleanup := createTempVault(map[string]string{})
			defer cleanup()

			jsonCmd := exec.Command(binPath, "--config", configPath, "config", "list", "--output", "json")
			jsonSession, err := gexec.Start(jsonCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(jsonSession).Should(gexec.Exit(0))
			jsonOut := string(jsonSession.Out.Contents())
			Expect(jsonOut).To(ContainSubstring(`"name": "test"`))
			Expect(jsonOut).NotTo(ContainSubstring("topics_dir"))

			plainCmd := exec.Command(binPath, "--config", configPath, "config", "list")
			plainSession, err := gexec.Start(plainCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(plainSession).Should(gexec.Exit(0))
			Expect(string(plainSession.Out.Contents())).To(ContainSubstring("test\t" + vaultPath))
		})
	})
```

Two traps this block must not fall into:

- **The JSON assertion is spaced.** `PrintJSON` sets `SetIndent("", "  ")`, so the only form that can match is `"topics_dir": "23 Topics"` — with exactly one space after the colon. Asserting the compact form `"topics_dir":"23 Topics"` can never match and would fail a correct build. Do not "fix" the spacing by changing `PrintJSON` or by post-processing the output.
- **The negative assertion needs a positive anchor.** `Expect(jsonOut).NotTo(ContainSubstring("topics_dir"))` passes vacuously on empty output, so the `"name": "test"` assertion must come first and must stay. The plain-output assertion is the second half of the same acceptance criterion: the vault is still listed with `name<TAB>path`, so the line `test\t<vaultPath>` must be asserted, not merely "some output".

## 5. Failure modes and security — what each test carries

Map the spec's Failure Modes table onto the change and state the mapping in your completion report:

- `topics_dir` present but empty (`topics_dir: ""`) → treated as unset. A YAML key that is absent and one explicitly set to the empty string both leave `TopicsDir` at its zero value `""`, so this is the same input as the default spec: `returns default 23 Topics when empty` is its coverage, and the `omitempty` tag omits the key from JSON. No extra test, no extra branch.
- `topics_dir` names a folder that does not exist → the accessor returns the value verbatim and never stats it; the custom-value spec is the coverage. Do not add an existence check to make this "safer".
- Malformed YAML → unchanged: `Load` returns the wrapped parse error naming the config path and the command exits non-zero. You change nothing here.
- A config written before this change, with no `topics_dir` anywhere → loads and resolves exactly as before; the default spec and the second integration spec cover it.
- A config carrying unknown keys → unchanged; the YAML decoder ignores them. You change nothing here.
- Two vaults declaring different values → each vault resolves its own value through the per-vault accessor; the extended scenario (a `full` vault that sets the key, a `minimal` vault that omits it) is the coverage.
- A value with a trailing slash or a nested path → returned verbatim, exactly like the seven sibling keys. No trimming, no joining.

Security properties to preserve, not to build: the value is an operator-owned string in the operator's own config file, it is never used to construct a path, spawn a process or interpolate into a shell command by this change, and it is emitted through the existing JSON encoder alongside the sibling keys — so quotes and newlines in it are escaped by that encoder rather than breaking the document. Do not add validation, containment or escaping logic; none of the seven siblings has any.

## 6. `README.md` — one line in the configuration example

Inside the fenced YAML block of § Configuration, in the `personal` vault's key list, insert one line between `goals_dir: "23 Goals"` and `daily_dir: "60 Periodic Notes/Daily"`:

```yaml
    topics_dir: "23 Topics"
```

The exact text `topics_dir: "23 Topics"` must appear — a bare mention of the word without a value does not satisfy the acceptance criterion. Do not reorder or reword any other line, and do not document the key anywhere else in the file.

## 7. `scenarios/001-config-list.md` — extend the existing scenario

Four edits, no fifth:

1. In the Setup block's heredoc, add `    topics_dir: "23 Topics"` to the `full` vault, directly after its `goals_dir: "23 Goals"` line. The `minimal` vault stays without the key.
2. In the Action section's `JSON output — full vault has all fields` checklist item, add `topics_dir` to the list of keys the `full` vault's JSON must contain (directly after `goals_dir`).
3. In the Action section's `Omitempty — minimal vault` checklist item, add `topics_dir` to the list of keys the `minimal` vault's JSON must NOT contain.
4. In the Expected section, change the field count in `All 16 documented \`Vault\` fields appear in JSON output for \`full\` vault` to `17`, so the line still matches the list it summarises.

Keep the existing checkbox syntax (`- [ ]`, lowercase, per the markdown-todo guide) and keep every other line as it is. Do NOT create a new scenario file — `ls scenarios/*.md | wc -l` must still print `5` after your change. The list's omission of `knowledge_dir`, `work_on_command` and `work_on_goal_command` predates this change — leave it alone; expanding the scenario's key list is not part of spec 048's acceptance criteria and belongs to a separate change.

## 8. `CHANGELOG.md` — one bullet under a new `## Unreleased`

There is no `## Unreleased` section yet. Create it directly below the preamble — after the `* PATCH version when you make backwards-compatible bug fixes.` line and above `## v0.132.2` — and put one bullet in it:

```
## Unreleased

- feat: a vault's config accepts a `topics_dir` key naming its topic-pages folder, read through `GetTopicsDir` and defaulting to `23 Topics` when the key is absent or empty. `vault-cli config list --output json` reports the key for a vault that sets it and omits it otherwise, exactly like the seven sibling directory keys. No command, flag, output format or existing config file changes.
```

- The bullet must start with `- feat:` on a single line and must name `topics_dir`; the remaining wording is yours to adjust.
- Do NOT bump any version: not `CHANGELOG.md`'s newest version heading, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. `.maintainer.yaml` sets `release.autoRelease: true`, and the post-merge releaser converts `## Unreleased` into a versioned section and tags it — a hand-bump would race it. `make check-versions` must still print `✅ all four versions equal: 0.132.2`.
- `make precommit` runs `scripts/check-changelog.sh`, which fails if any `## ` section lands above the preamble. Place the section below the preamble, as described.

## 9. Self-check before finishing

- Re-read the changed hunks and confirm: the struct line matches section 1 byte-for-byte, the accessor matches section 2, `pkg/cli/cli.go` and `pkg/ops/` are untouched, and no version string moved.
- Walk spec 048's Acceptance Criteria 1-8 and state in the completion report which requirement and which test satisfies each one, plus which evidence covers each row of the spec's Failure Modes table (section 5 above is the mapping).
- Walk `docs/dod.md`: the exported accessor has a doc comment, no `fmt.Print*` was added anywhere, tests use Ginkgo v2 / Gomega, the README documents the configuration change, and the changelog entry sits under `## Unreleased` below the preamble.
- Confirm each check in `<verification>` passes by running it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 048 — non-goals.** Do NOT rewrite the Personal vault's `.claude/commands/topic-manager.md` or `topic-status.md`; they live outside this repository. Do NOT add a `topic` command noun (`vault-cli topic list`, `topic show`, or any other). Do NOT add topics to the watch target set — `buildWatchTargets` keeps its four directories per vault. Do NOT rename, move or migrate any vault's `23 Topics/` folder, and do NOT write `topics_dir` into any config file: the config file stays read-only. Do NOT normalise, validate, expand or resolve the value (no `~` expansion, no trailing-slash trimming, no existence check). Do NOT add a flag, opt-out or env var that disables the default, and do NOT add a tunable default or a second default value — `23 Topics` is the single literal. Do NOT bump any version string or create a tag. Do NOT add a new scenario file.
- **Frozen literals.** Key spelling `topics_dir`; accessor name `GetTopicsDir`; default literal `23 Topics`; struct field `TopicsDir`; the unit spec names `returns custom topics dir when set`, `returns default 23 Topics when empty`, `loads topics_dir from YAML`; the integration spec names `config list --output json includes topics_dir for a vault that sets it` and `config list still lists a vault without topics_dir`. All of these are grep targets in the acceptance criteria.
- **Mirror the siblings exactly.** Same struct placement among the directory keys, same `omitempty` on both tags, same accessor shape (configured value when non-empty, literal otherwise), same test shape. No new loader branch, no validation, no normalisation, no path expansion, no existence check.
- **`pkg/cli/cli.go` is read-only** and `pkg/ops/` is untouched. No new command, no new subcommand, no new flag; the integration command-registration table gains no entry.
- **Backward compatibility is a requirement, not a convenience.** Every existing config file loads and resolves identically; `config list` plain output is unchanged; no command's output gains or loses a line; every existing test passes unchanged.
- **The JSON assertion is spaced** (`"topics_dir": "23 Topics"`), because `PrintJSON` uses `SetIndent("", "  ")`. Never assert the compact form, and never change the encoder.
- **Unit specs must assert.** A named `It` with an empty body, or one that never calls `GetTopicsDir()`, does not satisfy the acceptance criteria.
- **Tests.** Ginkgo v2 + Gomega, external test packages (`config_test`, `integration_test`), counterfeiter mocks where already used — no stdlib `t.Run` table tests. Every Go file keeps its BSD license header.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies — under `workflow: worktree` or `hideGit: true` it fails with `fatal: not a git repository` — reports a false pass.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The two test runs, with their frozen spec names.** Capture each run's output, check its exit status separately from the name greps (a failing run still prints the names), and never pipe a test command:

```
go test ./pkg/config/... -v -count=1 > /tmp/topics-config.log 2>&1; test "$?" = "0"
grep -F -q -- 'returns custom topics dir when set' /tmp/topics-config.log
grep -F -q -- 'returns default 23 Topics when empty' /tmp/topics-config.log
grep -F -q -- 'loads topics_dir from YAML' /tmp/topics-config.log
```

```
go test ./integration/... -v -count=1 > /tmp/topics-integration.log 2>&1; test "$?" = "0"
grep -F -q -- 'config list --output json includes topics_dir for a vault that sets it' /tmp/topics-integration.log
grep -F -q -- 'config list still lists a vault without topics_dir' /tmp/topics-integration.log
```

`go test ./integration/...` builds the binary with `gexec.Build` and runs it as a subprocess against temp config files — it is the real-binary proof of the JSON surface and of the omit-when-unset behaviour, and it is the repeatable form of the spec's hand-reproduction of that surface, whose built-binary rung is operator-side. If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment; export it rather than touching `.git`.

**The accessor and the struct field, in `pkg/config/config.go`:**

```
test "$(grep -c 'GetTopicsDir' pkg/config/config.go)" -ge 1
test "$(grep -c 'topics_dir' pkg/config/config.go)" -ge 1
test "$(grep -A6 'func (v \*Vault) GetTopicsDir()' pkg/config/config.go | grep -c 'return ')" = "2"
test "$(grep -A6 'func (v \*Vault) GetTopicsDir()' pkg/config/config.go | grep -cE 'os\.|filepath\.|strings\.|errors\.|fmt\.')" = "0"
test "$(grep -n 'KnowledgeDir' pkg/config/config.go | head -1 | cut -d: -f1)" -lt "$(grep -n 'TopicsDir' pkg/config/config.go | head -1 | cut -d: -f1)"
test "$(grep -n 'TopicsDir' pkg/config/config.go | head -1 | cut -d: -f1)" -lt "$(grep -n 'ClaudeScript' pkg/config/config.go | head -1 | cut -d: -f1)"
```

The first two are the acceptance criteria's grep evidence (an absent or renamed accessor, and a struct field without a YAML tag, both fail here). The next two pin the accessor's shape: exactly two `return` statements and no path handling, no normalisation and no error construction — which is what "returned verbatim, never stat-ed" means mechanically. The last two pin the field's placement among the directory keys. `grep -A6` cannot reach the `GetExcludes` accessor that follows, so the counts are stable.

**The assertions themselves — a spec that is only named does not count:**

```
grep -F -q 'Expect(vault.GetTopicsDir()).To(Equal("Custom Topics"))' pkg/config/vault_test.go
grep -F -q 'Expect(vault.GetTopicsDir()).To(Equal("23 Topics"))' pkg/config/vault_test.go
grep -F -q 'Expect(vault.TopicsDir).To(Equal("23 Topics"))' pkg/config/config_test.go
```

The third asserts the raw struct field, which is the only assertion that proves the YAML tag is wired: a config file carrying `topics_dir: "23 Topics"` would satisfy `GetTopicsDir()` through the default alone.

**The integration block exists, uses the spaced form, and drives the real binary:**

```
test "$(grep -c 'func createTempVaultWithTopics(' integration/cli_test.go)" = "1"
grep -F -q 'It("config list --output json includes topics_dir for a vault that sets it"' integration/cli_test.go
grep -F -q 'It("config list still lists a vault without topics_dir"' integration/cli_test.go
grep -F -q '"topics_dir": "23 Topics"' integration/cli_test.go
test "$(grep -c 'gexec.Start' integration/cli_test.go)" -ge 2
```

The fourth line is the trap guard: the spaced form must appear in the file, so a compact-form assertion cannot slip in.

**README, scenario, changelog:**

```
grep -F -q 'topics_dir: "23 Topics"' README.md
test "$(grep -c 'topics_dir' scenarios/001-config-list.md)" = "3"
grep -F -q 'topics_dir: "23 Topics"' scenarios/001-config-list.md
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "5"
test "$(grep '^## ' CHANGELOG.md | head -1)" = "## Unreleased"
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "1"
test "$(grep -cE '^- feat:.*topics_dir' CHANGELOG.md)" -ge 1
make check-changelog
make check-versions
```

If `grep '^## ' CHANGELOG.md | head -1` prints a version heading, the bullet was placed between released sections — move it into the `## Unreleased` section directly below the preamble. `make check-versions` must print `✅ all four versions equal: 0.132.2`; a different version means something bumped a version string that this change must leave alone.

**Formatting:**

```
test -z "$(gofmt -e -l pkg/config/config.go pkg/config/vault_test.go pkg/config/config_test.go integration/cli_test.go)"
```

Finally, walk spec 048's Acceptance Criteria 1-8 against the change and state in your completion report which requirement and which test satisfies each one, plus which evidence covers each row of the spec's Failure Modes table.
</verification>
