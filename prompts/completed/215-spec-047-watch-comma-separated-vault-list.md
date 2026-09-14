---
status: completed
spec: [047-watch-comma-separated-vault-list]
execution_id: vault-cli-watch-multivault-exec-215-spec-047-watch-comma-separated-vault-list
dark-factory-version: dev
created: "2026-09-14T19:35:13Z"
queued: "2026-09-14T20:16:31Z"
started: "2026-09-14T20:17:03Z"
completed: "2026-09-14T20:21:07Z"
branch: dark-factory/watch-comma-separated-vault-list
---

# watch: accept a comma-separated vault list (spec 047, prompt 1 of 1)

<summary>
- One `vault-cli watch` process can watch several named vaults at once: `--vault a,b,c` starts a single watcher covering exactly those vaults.
- A consumer that today starts one watcher process per vault can start one watcher for all of them, and still tell them apart, because every event already names its own vault.
- Whitespace around names is ignored and empty entries between commas are skipped, so `--vault "a, b"` and `--vault a,,b` select the same vaults as `--vault a,b`.
- A value that names no vault at all — `","`, `" "`, `",,"` — fails loudly with an error naming the value, instead of silently widening to every vault.
- A single name still means exactly that vault, and omitting the flag still means every configured vault — both unchanged from today.
- Every other command keeps the old single-vault behaviour: `task list --vault alpha,beta` still fails with the same `vault not found` error it produces today, and the deprecated `task watch` is untouched.
- The watching engine is not touched at all — same event schema, same debounce, same type filter, same per-vault skip when a directory is missing.
- The `watch` help text documents the comma form with a worked example, and the developer docs' multi-vault section records the watch-only exception.
- The changelog records the change under Unreleased; no version string is bumped.
- Every parsing and error path is pinned by a unit test, and every behaviour an operator can observe is pinned by a subprocess test that runs the real binary against a two-vault config; the untouched-engine, help-text and documentation criteria are pinned by file hashes and text checks.
</summary>

<objective>
Let a single `vault-cli watch` process cover several named vaults — `--vault a,b,c` — while a single name and an omitted flag behave exactly as they do today and no other command's vault selection changes. This is spec 047's only prompt: it covers Desired Behaviors 1-6 and Acceptance Criteria 1-8.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then read these files in full:

- `pkg/cli/cli.go` — the only production file you change. The parts that matter:
  - `getVaults` (top of the file, immediately after `var version`) — the shared single-vault resolver, called by ~30 command builders. It stays byte-identical.
  - `NewRootCommand` — the root `--vault` persistent flag registration and the `createWatchCommand(ctx, &configLoader, &vaultName)` registration.
  - `createWatchCommand` (~line 2347) — the `Long` literal, the `RunE` body, and the `--types` flag registration.
  - `buildWatchTargets` — turns `[]*config.Vault` into `[]ops.WatchTarget`; unchanged.
  - `createTaskWatchCommand` — the deprecated `task watch`; unchanged (it keeps `getVaults`).
  - `parseWatchTypes`, `watchTypeIsValid`, `watchEventMatchesFilter` — the `--types` filter; unchanged.
  - `Execute` — prints `Error: %v` to stderr and exits 1 when `Run` returns an error; this is what the integration tests observe.
- `pkg/ops/watch.go` — the watching engine. READ ONLY. It already takes `[]WatchTarget`, registers each existing directory (`buildDirMap` skips a missing directory with a debug log rather than failing), debounces 100 ms per vault+path, and stamps `Vault` on every `WatchEvent`. Nothing in this file may change.
- `pkg/config/config.go` — `GetVault` (lowercases the name, returns `errors.Errorf(ctx, "vault not found: %s", vaultName)` for an unknown one), `GetAllVaults`, `Vault.GetTasksDir` / `GetGoalsDir` (defaults `Tasks` / `Goals`).
- `pkg/cli/watch_test.go` — the existing watch tests: the `cli.Run` error-path style and the `rootCmd.ExecuteContext(ctx)` + `gbytes` pattern.
- `pkg/cli/export_test.go` — the test-only export file (`package cli`, visible to `cli_test`).
- `pkg/cli/resolve_test.go` — the convention for holding a `*config.Loader` in a test: a `mocks.Loader` behind a `config.Loader` interface variable.
- `integration/cli_test.go` — the integration harness: `createTwoTempVaults` (vaults `alpha` and `beta`, each with `Tasks` and `Goals`, config with `default_vault: alpha`), `gexec.Start`, `gbytes`, and the `binPath` convention.
- `integration/integration_suite_test.go` — `binPath` is built in `BeforeSuite` with `gexec.Build("github.com/bborbe/vault-cli")`.
- `docs/development-patterns.md` — the `## Multi-Vault Pattern` section spans lines 81-89; you add a note inside it.
- `docs/dod.md` — this repo's `validationPrompt`; your change must satisfy it.
- `CHANGELOG.md` — the `# Changelog` title, the `All notable changes…` preamble, the `* MAJOR/MINOR/PATCH` lines, then `## v0.131.10`. There is no `## Unreleased` section yet.
- `specs/in-progress/047-watch-comma-separated-vault-list.md` — the spec. Read the Acceptance Criteria, Constraints, Failure Modes and Desired Behavior sections.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf(ctx, …)` / `errors.Wrap(ctx, err, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err` for a new error, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, counterfeiter mocks, coverage expectations.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — cobra command construction, `Long` / `Use` / `Args`, error propagation.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits this change must respect (funlen 80, nestif 4, golines 100), banned packages, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — entry format and the `## Unreleased` placement rule.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

Current state of the pieces you change, quoted verbatim so you do not have to guess:

`getVaults` (unchanged; your new function goes directly after it):

```go
// getVaults returns the vaults to operate on.
// If vaultName is set, returns just that vault.
// If vaultName is empty, returns all configured vaults.
func getVaults(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) ([]*config.Vault, error) {
	if *vaultName != "" {
		vault, err := (*configLoader).GetVault(ctx, *vaultName)
		if err != nil {
			return nil, err
		}
		return []*config.Vault{vault}, nil
	}
	return (*configLoader).GetAllVaults(ctx)
}
```

The root persistent flag (unchanged — do NOT edit this line or its help text):

```go
	rootCmd.PersistentFlags().
		StringVar(&vaultName, "vault", "", "Vault name (uses default if not specified)")
```

The watch command's current `RunE` (only the resolver call changes):

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			typeFilter, err := parseWatchTypes(ctx, cmd.Flags().Lookup("types").Changed, typesStr)
			if err != nil {
				return err
			}

			vaults, err := getVaults(ctx, configLoader, vaultName)
			if err != nil {
				return errors.Wrap(ctx, err, "get vaults")
			}

			targets := buildWatchTargets(vaults)
			watchOp := ops.NewWatchOperation()
			return watchOp.Execute(ctx, targets, func(event ops.WatchEvent) error {
				if !watchEventMatchesFilter(event, typeFilter) {
					return nil
				}
				enc := json.NewEncoder(os.Stdout)
				return enc.Encode(event)
			})
		},
```

The `--types` flag registration inside `createWatchCommand` (unchanged):

```go
	cmd.Flags().
		StringVar(&typesStr, "types", "", "Comma-separated entity types to emit (task,goal,theme,objective). Omit for all types.")
```

Two environment facts that shape this prompt:

1. **`git` is unavailable in this container.** `/workspace/.git` is a character device, so every git command fails with `fatal: not a git repository`. Do NOT run git. Spec 047 AC5's evidence — `git diff origin/master...HEAD --name-only -- pkg/ops/` printing 0 lines — cannot run here; `<verification>` replaces it with content pins over the whole `pkg/ops` Go tree, which prove the same thing (the engine is untouched) and cannot pass vacuously.
2. **The container sets `GOFLAGS=-buildvcs=false`** (`.dark-factory.yaml` `env:`). Keep it. `gexec.Build` in the integration suite runs `go build`, which otherwise tries to read the masked `.git` and fails with a VCS status error.
</context>

<requirements>

## 0. Scope — one production file, three test files, two docs

You change exactly these six files:

- `pkg/cli/cli.go` — add `getWatchVaults`, point the `watch` command at it, extend the `watch` help text.
- `pkg/cli/export_test.go` — add one test-only export.
- `pkg/cli/watch_test.go` — add the resolver's unit tests.
- `integration/watch_test.go` — NEW file: the subprocess tests.
- `docs/development-patterns.md` and `CHANGELOG.md` — the docs note and the changelog bullet.

Nothing else. No new flag, no new subcommand, no config field, no version bump, no README change (the spec's documentation surface is the command help text, `docs/development-patterns.md`, and `CHANGELOG.md`), and nothing at all under `pkg/ops/`.

## 1. Add the watch-scoped resolver to `pkg/cli/cli.go`

Insert this function immediately after `getVaults`, verbatim:

```go
// getWatchVaults returns the vaults the watch command should watch.
//
// A comma-separated value selects exactly the named vaults: whitespace around
// each name is ignored and empty entries between commas are skipped. A value
// that yields no usable name (for example "," or " ") is an error, not a silent
// fallback to every vault. The empty string means the flag was not set and
// selects every configured vault, exactly as getVaults does.
func getWatchVaults(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) ([]*config.Vault, error) {
	if *vaultName == "" {
		return (*configLoader).GetAllVaults(ctx)
	}

	names := make([]string, 0, strings.Count(*vaultName, ",")+1)
	for _, entry := range strings.Split(*vaultName, ",") {
		name := strings.TrimSpace(entry)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, errors.Errorf(ctx, "no vault name in --vault %q", *vaultName)
	}

	vaults := make([]*config.Vault, 0, len(names))
	for _, name := range names {
		vault, err := (*configLoader).GetVault(ctx, name)
		if err != nil {
			return nil, err
		}
		vaults = append(vaults, vault)
	}
	return vaults, nil
}
```

Notes that are not negotiable:

- Write the signature on four lines, exactly as above. Do NOT collapse it onto one line.
- `getVaults` stays byte-identical and remains the resolver for every other command. Do not refactor it, do not add a parameter to it, do not make it call the new function.
- No new import is needed: `strings` and `github.com/bborbe/errors` are already imported by `pkg/cli/cli.go`.
- Do NOT deduplicate repeated names. `--vault alpha,alpha` resolves the same vault twice and the engine is unaffected because directories are keyed by absolute path; adding dedupe logic would be untested surface the spec does not ask for.
- Do NOT treat a malformed value as "every vault". The empty string is the only value that selects every vault, because the flag cannot distinguish unset from explicitly empty.
- Do NOT add a knob, opt-out flag, or config option of any kind.

## 2. Point the `watch` command at the new resolver

In `createWatchCommand`'s `RunE`, change exactly one line:

```go
			// OLD
			vaults, err := getVaults(ctx, configLoader, vaultName)
```

```go
			// NEW
			vaults, err := getWatchVaults(ctx, configLoader, vaultName)
```

The wrapping stays as it is: `return errors.Wrap(ctx, err, "get vaults")`. Everything else in that `RunE` — the `parseWatchTypes` call, `buildWatchTargets`, `ops.NewWatchOperation()`, the JSON encoder to `os.Stdout` — stays byte-identical.

The call must be written exactly as `getWatchVaults(ctx, configLoader, vaultName)`, on one line: an acceptance criterion greps for that literal, and the only line in `pkg/cli/cli.go` that may match it is this call site inside `createWatchCommand` (the function's own multi-line signature does not match the pattern, which is intended).

## 3. Document the comma form in the `watch` help text

Replace the whole `Long` literal of `createWatchCommand` with this text — the two added sentences are the only change, and both frozen phrases (`--vault personal,trading` and `comma-separated vault list`) sit inside the literal:

```go
		Long: `Watch tasks, goals, themes, and objectives directories for file changes.
Emits one newline-delimited JSON event per debounced change.

Each event includes:
  event  - change type: created, modified, deleted
  name   - filename without .md extension
  vault  - vault name
  path   - vault-relative file path
  type   - entity kind: task, goal, theme, objective

Use --vault with a comma-separated vault list to watch several vaults in one
process, e.g. --vault personal,trading. Omit --vault to watch every configured
vault. Every event names its own vault in the vault field.

Use --types to filter to a subset of entity kinds.
Valid type values: task, goal, theme, objective`,
```

The phrase must be lowercase `comma-separated vault list` (the existing `--types` help text says `Comma-separated entity types` — that capitalised line is unrelated and stays as it is).

## 4. Leave every other command's `--vault` alone

- The root `--vault` persistent flag registration and its help text stay byte-identical — do not reword them, even though the watch command now accepts more than a single name.
- `createTaskWatchCommand` (the deprecated `task watch`) keeps `getVaults` and its single-name behaviour. It is a different command; the comma form is watch-only.
- No other command builder changes. After your edit, `grep -c 'getVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go` must print `29` (it prints 30 today: the watch `RunE` is the one that switches).

## 5. Do not touch the watching engine

`pkg/ops/` is read-only for this prompt: no file added, removed, or edited. The event JSON schema is frozen (no new field, no renamed field), the 100 ms debounce is unchanged, the `created` / `modified` / `deleted` vocabulary is unchanged, and the per-vault skip of a missing directory is unchanged. If you believe the engine needs a change, do not make it — say so in the completion report instead.

## 6. Unit tests — the resolver's cases

### 6a. Export the resolver for the external test package

`pkg/cli/watch_test.go` is `package cli_test`, so add this test-only export to `pkg/cli/export_test.go`, after the existing exports, matching that file's style:

```go
// GetWatchVaultsForTest exposes getWatchVaults for testing.
func GetWatchVaultsForTest(
	ctx context.Context,
	configLoader *config.Loader,
	vaultName *string,
) ([]*config.Vault, error) {
	return getWatchVaults(ctx, configLoader, vaultName)
}
```

Both imports it needs (`context`, `github.com/bborbe/vault-cli/pkg/config`) are already in that file.

### 6b. The test block

Add a new `var _ = Describe(...)` block to `pkg/cli/watch_test.go`, alongside the existing `vault-cli watch --types` and `vault-cli task watch deprecation` blocks. Add the imports it needs: `github.com/bborbe/errors`, `github.com/bborbe/vault-cli/mocks`, `github.com/bborbe/vault-cli/pkg/config`.

Scaffolding and one worked example, verbatim:

```go
var _ = Describe("vault-cli watch --vault", func() {
	var (
		ctx          context.Context
		fakeLoader   *mocks.Loader
		configLoader *config.Loader
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeLoader = &mocks.Loader{}
		var loader config.Loader = fakeLoader
		configLoader = &loader
	})

	It("watches every configured vault when the value is empty", func() {
		fakeLoader.GetAllVaultsReturns([]*config.Vault{
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		}, nil)
		vaultName := ""

		vaults, err := cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(fakeLoader.GetAllVaultsCallCount()).To(Equal(1))
		Expect(fakeLoader.GetVaultCallCount()).To(Equal(0))
	})
```

Then add these cases as further `It`s in the same block. Every case calls `cli.GetWatchVaultsForTest(ctx, configLoader, &vaultName)`; the assertions listed are the complete expectation for each case:

| # | `vaultName` value | fake setup | required assertions |
|---|---|---|---|
| 1 | `"alpha"` | `fakeLoader.GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)` | err nil; `vaults` has 1 entry named `alpha`; `GetVaultCallCount() == 1`; the first `GetVault` arg is `"alpha"`; `GetAllVaultsCallCount() == 0` |
| 2 | `"alpha,beta"` | `GetVaultReturnsOnCall(0, &config.Vault{Name: "alpha"}, nil)` and `GetVaultReturnsOnCall(1, &config.Vault{Name: "beta"}, nil)` | err nil; `vaults` is `[alpha, beta]` in that order; args are `"alpha"` then `"beta"`; `GetAllVaultsCallCount() == 0` |
| 3 | `" alpha , beta "` | same as case 2 | err nil; args are `"alpha"` then `"beta"` — the surrounding whitespace is gone |
| 4 | `"alpha,,beta"` | same as case 2 | err nil; `GetVaultCallCount() == 2`; args are `"alpha"` then `"beta"` — the empty entry is skipped, not resolved |
| 5 | `"alpha,alpha"` | `GetVaultReturns(&config.Vault{Name: "alpha", Path: "/tmp/alpha"}, nil)` | err nil; `vaults` has 2 entries, both named `alpha`; `GetVaultCallCount() == 2` — repeated names are tolerated, not deduplicated |
| 6 | `","` | none | err not nil; `err.Error()` contains `,`; `GetVaultCallCount() == 0`; `GetAllVaultsCallCount() == 0`; `vaults` is empty — no watcher starts and the value does not widen to every vault |
| 7 | `" "` | none | err not nil; `GetVaultCallCount() == 0`; `GetAllVaultsCallCount() == 0` |
| 8 | `",,"` | none | err not nil; `GetVaultCallCount() == 0`; `GetAllVaultsCallCount() == 0` |
| 9 | `"alpha,nope"` | `GetVaultReturnsOnCall(0, &config.Vault{Name: "alpha"}, nil)` and `GetVaultReturnsOnCall(1, nil, errors.Errorf(ctx, "vault not found: nope"))` | err not nil; `err.Error()` contains `nope`; `vaults` is empty — one bad name fails the whole call, it does not silently watch the names that did resolve |

Read the call arguments with the counterfeiter accessor, e.g.:

```go
		_, firstName := fakeLoader.GetVaultArgsForCall(0)
		_, secondName := fakeLoader.GetVaultArgsForCall(1)
		Expect(firstName).To(Equal("alpha"))
		Expect(secondName).To(Equal("beta"))
```

### 6c. One boundary case through the real config lookup

Add a final `It` that does NOT use the fake. It writes a two-vault config to a temp file and resolves through the real loader, proving the parser's trimming composes with the existing case-insensitive lookup:

```yaml
vaults:
  alpha:
    name: alpha
    path: <tmp dir A>
  beta:
    name: beta
    path: <tmp dir B>
```

```go
		var loader config.Loader = config.NewLoader(configPath)
		vaultName := "ALPHA, beta"

		vaults, err := cli.GetWatchVaultsForTest(ctx, &loader, &vaultName)

		Expect(err).To(BeNil())
		Expect(vaults).To(HaveLen(2))
		Expect(vaults[0].Name).To(Equal("alpha"))
		Expect(vaults[1].Name).To(Equal("beta"))
```

Create the two vault directories with `os.MkdirTemp` and write the config with `os.CreateTemp` + `os.WriteFile`, following the temp-file cleanup style already used in `pkg/cli/watch_test.go`. Do not add a helper to `pkg/config` for this.

## 7. Integration tests — the real binary against a two-vault config

Create `integration/watch_test.go` (`package integration_test`, so it shares `binPath` from the suite). Give it the same BSD license header as the other files (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` plus the license line), and these imports: `os`, `os/exec`, `path/filepath`, `time`. Use the same import form as `integration/cli_test.go`: dot-import Ginkgo and Gomega (`. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`) so `It`, `Expect`, `Eventually` and `Consistently` stay unqualified, and import `gexec` plainly (`"github.com/onsi/gomega/gexec"`) so every call reads `gexec.Start` / `gexec.Exit`.

Two helpers, verbatim:

```go
// watchProbe writes a markdown file into each given watched directory.
//
// Call it from inside Eventually. The watcher registers its directories when it
// starts and emits no ready signal, so a single write can race that registration
// and be missed; rewriting on every poll is the retry.
func watchProbe(paths ...string) {
	for _, path := range paths {
		Expect(os.WriteFile(path, []byte("---\nstatus: next\n---\n"), 0600)).To(Succeed())
	}
}

// watchStdout returns everything the watcher process has written to stdout so far.
func watchStdout(session *gexec.Session) string {
	return string(session.Out.Contents())
}
```

Then the specs. Two are written out in full below; the rest follow exactly the same shape with the arguments and assertions given.

```go
var _ = Describe("vault-cli watch --vault comma list", func() {
	It("AC1: watch --vault alpha,beta emits events for both vaults on one stream", func() {
		vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", "alpha,beta")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		alphaProbe := filepath.Join(vaultPathA, "Tasks", "alpha-probe.md")
		betaProbe := filepath.Join(vaultPathB, "Tasks", "beta-probe.md")

		Eventually(func() string {
			watchProbe(alphaProbe, betaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"alpha"`))

		Eventually(func() string {
			watchProbe(alphaProbe, betaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"beta"`))
	})

	It("AC2: watch --vault alpha watches only alpha", func() {
		vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", "alpha")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		alphaProbe := filepath.Join(vaultPathA, "Tasks", "alpha-probe.md")
		betaProbe := filepath.Join(vaultPathB, "Tasks", "beta-probe.md")

		// An alpha event proves the watcher is up and watching alpha.
		Eventually(func() string {
			watchProbe(alphaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"alpha"`))

		// beta was not named, so a change inside it must produce nothing.
		watchProbe(betaProbe)
		Consistently(func() string {
			return watchStdout(session)
		}, "1s", "100ms").ShouldNot(ContainSubstring(`"vault":"beta"`))
	})
```

The remaining six specs, same file, same `Describe` block:

1. **AC2 — an omitted flag still watches every configured vault.** Args `--config <cfg> watch` (no `--vault`). Poll `watchProbe(alphaProbe, betaProbe)` until stdout contains `"vault":"alpha"`, then until it contains `"vault":"beta"`.
2. **AC3 — an unresolvable name fails loudly.** Args `--config <cfg> watch --vault alpha,nope`. `Eventually(session).Should(gexec.Exit(1))`; `string(session.Err.Contents())` contains `nope`; `watchStdout(session)` does NOT contain `"vault"` (no event line was emitted). Use `_, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)` — the vault paths are unused here.
3. **AC3 — a value that names no vault fails loudly.** Args `--config <cfg> watch --vault ,`. `Eventually(session).Should(gexec.Exit(1))`; stderr contains `,`; stdout contains no `"vault"`.
4. **AC4(a) — no other command's `--vault` changed.** Args `--config <cfg> task list --vault alpha,beta`. `Eventually(session).Should(gexec.Exit(1))`; stderr contains `vault not found: alpha,beta`.
5. **DB6 — `--types` still filters per vault.** Args `--config <cfg> watch --vault alpha,beta --types goal`. Use `vaultPathA, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)`. Poll a goal probe in `filepath.Join(vaultPathA, "Goals", "alpha-probe-goal.md")` until stdout satisfies `And(ContainSubstring("\"type\":\"goal\""), ContainSubstring("\"vault\":\"alpha\""))`; then write a task probe into `filepath.Join(vaultPathA, "Tasks", "alpha-probe-task.md")` and assert `Consistently` for `"1s"` / `"100ms"` that stdout does NOT contain `"type":"task"`.
6. **Failure mode — a vault whose task directory is missing does not break the list.** Use `vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)`, then `Expect(os.RemoveAll(filepath.Join(vaultPathB, "Tasks"))).To(Succeed())` BEFORE starting the process. Args `--config <cfg> watch --vault alpha,beta`. Poll `watchProbe(filepath.Join(vaultPathA, "Tasks", "alpha-probe.md"))` until stdout contains `"vault":"alpha"` — alpha is watched even though beta's task directory is absent.

Rules for all eight specs:

- `defer session.Kill()` on every spec that starts a watcher — that is, both specs written in full above (AC1 and AC2), plus items 1, 5 and 6 below. Items 2, 3 and 4 below exit on their own and need no Kill.
- Never write the probe files before the process starts — the watcher only reports changes that happen while it runs, and the poll-and-rewrite loop in `watchProbe` is what removes the start-up race.
- Assert on `session.Out` (stdout) for events and `session.Err` (stderr) for error text. The JSON encoder writes compact JSON, so the exact substrings are `"vault":"alpha"` and `"type":"goal"` with no spaces.
- Do not add a timeout longer than 10 s and do not add retry loops of your own beyond the `Eventually` polling above.

## 8. `docs/development-patterns.md` — record the watch-only exception

The `## Multi-Vault Pattern` section currently reads (lines 81-89):

```
## Multi-Vault Pattern

All commands use `getVaults()` to resolve vaults:

- `--vault NAME` → single vault
- No flag → all configured vaults

Commands iterate vaults and call operations per vault. For mutation commands (complete, defer, ack), try each vault until the item is found.
```

Change it to exactly this — the first sentence gains the exception, and one note is inserted directly after the `- No flag → all configured vaults` bullet and BEFORE the `Commands iterate vaults…` paragraph:

```
## Multi-Vault Pattern

All commands except `watch` use `getVaults()` to resolve vaults:

- `--vault NAME` → single vault
- No flag → all configured vaults

- `watch --vault a,b` accepts a comma-separated vault list and resolves it through `getWatchVaults`; every other command keeps `getVaults` and a single vault name.
  A value that yields no usable name is an error; the empty string means every configured vault.

Commands iterate vaults and call operations per vault. For mutation commands (complete, defer, ack), try each vault until the item is found.
```

Constraints on this edit:

- Both frozen phrases (`comma-separated` and `getWatchVaults`) must appear on the FIRST line of the inserted note, and each must appear exactly once in the whole file. The acceptance criterion requires every matching line number L to satisfy `81 < L < 90`, and the inserted note lands at lines 88-89 with this layout — if you put either phrase in a later paragraph, the criterion fails.
- Do not reflow, renumber, or otherwise restructure the rest of the section; the sentence at line 83 keeps its length on one line.
- Do not edit any other section of the file.

## 9. `CHANGELOG.md` — one bullet under a new `## Unreleased`

There is no `## Unreleased` section yet. Create it directly below the preamble block — after the `* PATCH version when you make backwards-compatible bug fixes.` line and above `## v0.131.10` — with this bullet:

```
## Unreleased

- feat: `vault-cli watch --vault a,b` accepts a comma-separated vault list, watching every named vault in one process and stamping each event with its own `vault`. Whitespace around names is ignored, empty entries between commas are skipped, and a value that names no vault (for example `,`) fails with an error naming the value instead of silently widening to every vault. A single name and an omitted flag behave exactly as before, and every other command keeps single-vault `--vault` semantics through `getVaults`.
```

- The phrase `comma-separated vault list` must appear verbatim.
- Do NOT bump any version: not `CHANGELOG.md`'s newest version heading, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. The post-merge `github-releaser-agent` owns the bump — `.maintainer.yaml` sets `release.autoRelease: true`, and `CLAUDE.md` records that the releaser converts `## Unreleased` into `## vX.Y.Z` and tags after merge — so hand-bumping would race it. The acceptance criterion requires `grep '^## ' CHANGELOG.md | head -1` to print exactly `## Unreleased`, so a version heading must not appear above it.
- `make precommit` runs `scripts/check-changelog.sh`, which fails if any `## ` section lands above the preamble. Place the section below the preamble, as described.

## 10. Failure modes and security — what each test carries

Map the spec's failure-mode table onto the tests you write, and say which spec covers which row in your completion report:

- A name that is not configured → unit case 9 (the error names the vault, no partial result) and integration AC3 (exit 1, no event lines).
- A value with only separators → unit cases 6-8 and integration AC3.
- A named vault whose task directory is missing → integration failure-mode spec 6.
- The same vault named twice → unit case 5.
- A large list → one process watches them all; no test, no code, and no comment about it.

Security property to preserve: the flag value is only ever used to look up a vault in the operator's own configuration. It never constructs a path, so it cannot select an arbitrary directory. Your implementation must keep it that way — `GetVault` is the only lookup, and an unresolvable name is an error.

## 11. Self-check before finishing

- Re-read the changed hunks in `pkg/cli/cli.go` and confirm `getVaults` is byte-identical, the new function matches section 1 exactly, and no other command's `RunE` changed.
- Walk spec 047's Acceptance Criteria 1-8 and state in the completion report which requirement and which test satisfies each.
- Walk `docs/dod.md`: exported function has a doc comment, errors use `github.com/bborbe/errors`, no `fmt.Print*` added to `pkg/ops/`, tests use Ginkgo v2 / Gomega, changelog entry under `## Unreleased` below the preamble. The license header on the new file is required by the `addlicense` target (`go-precommit.md`), not by `docs/dod.md`.
- Confirm the new resolver's coverage with the command in `<verification>`; it must be 100.0%.
- Note in the completion report that `pkg/cli/cli.go` is already 2467 lines and should be split before further additions. Do not split it as part of this change — that is a separate concern.

</requirements>

<constraints>
- **Copied from spec 047 — non-goals.** Do NOT change `--vault` semantics for any command other than `watch`. Do NOT add a new flag, a new subcommand, or a configuration option. Do NOT change the watching engine, the event schema, the debounce behaviour, or the `--types` filter. Do NOT change how many vaults any consumer displays. Do NOT reduce the number of vaults vault-cli knows about. Do NOT make a malformed `--vault` value silently mean "every vault" — the only value that selects every vault is the empty string, which is what an unset flag produces.
- **Backward compatibility is a requirement, not a convenience.** A single name and an omitted flag behave exactly as they do today.
- **Frozen literals.** `watch` help text shows `--vault personal,trading`; both the help text and the changelog bullet use the phrase `comma-separated vault list`; the watch-scoped resolver is named `getWatchVaults` and its call site is written `getWatchVaults(ctx, configLoader, vaultName)`. All four are grep targets in the acceptance criteria.
- **`getVaults` is untouched** and stays the resolver for every other command. The root `--vault` persistent flag registration and its help text are unchanged. The deprecated `task watch` keeps `getVaults`.
- **`pkg/ops/` is read-only.** The engine already accepts a list of vaults and stamps each event with its vault; nothing there may change.
- **Error idiom.** `errors.Errorf(ctx, …)` / `errors.Wrap(ctx, err, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`; no bare `return err` for a newly constructed error; no `context.Background()` in `pkg/`.
- **`getWatchVaults`'s doc comment** records that a value yielding no usable name is an error and that the empty string means every configured vault.
- **No dedupe, no knob.** Repeated names are tolerated as-is; no opt-out flag, config field, or tunable is added.
- **No version bumps.** Leave `CHANGELOG.md`'s newest version heading and both `.claude-plugin/` JSON files untouched; the entry goes under `## Unreleased`.
- **No README change.** The documentation surface for this spec is the command help text, `docs/development-patterns.md`, and `CHANGELOG.md`.
- **Tests.** Ginkgo v2 + Gomega, counterfeiter mocks (`mocks.Loader`), external test package (`cli_test` / `integration_test`) — no stdlib `t.Run` table tests. Existing tests must still pass unchanged. `integration/watch_test.go` carries the BSD license header, like every other Go file in this repo.
- **Do NOT commit** — dark-factory handles git. This container's `.git` is masked: make no git calls at all, including in `<verification>`.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, or any `dark-factory` command.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must exit 0. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**AC4(b) — the watch-scoped resolver, and only inside `createWatchCommand`:**

```
test "$(grep -c 'getWatchVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go)" = "1"
test "$(grep -n 'getWatchVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go | cut -d: -f1)" -gt "$(grep -n '^func createWatchCommand(' pkg/cli/cli.go | cut -d: -f1)"
test "$(grep -n 'getWatchVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go | cut -d: -f1)" -lt "$(grep -n '^func buildWatchTargets(' pkg/cli/cli.go | cut -d: -f1)"
```

**AC4(b) second half — every other command still resolves through `getVaults` (30 call sites today, 29 after this change):**

```
test "$(grep -c 'getVaults(ctx, configLoader, vaultName)' pkg/cli/cli.go)" = "29"
```

**AC6 — the worked example and the frozen phrase live in the watch help text, not merely somewhere in the file:**

```
test "$(grep -c 'personal,trading' pkg/cli/cli.go)" = "1"
test "$(grep -n 'personal,trading' pkg/cli/cli.go | cut -d: -f1)" -gt "$(grep -n '^func createWatchCommand(' pkg/cli/cli.go | cut -d: -f1)"
test "$(grep -n 'personal,trading' pkg/cli/cli.go | cut -d: -f1)" -lt "$(grep -n '^func buildWatchTargets(' pkg/cli/cli.go | cut -d: -f1)"
test "$(grep -c 'comma-separated vault list' pkg/cli/cli.go)" = "1"
```

**AC7 — the multi-vault note, inside the Multi-Vault Pattern section window `81 < L < 90`:**

```
test "$(grep -c 'comma-separated' docs/development-patterns.md)" = "1"
test "$(grep -c 'getWatchVaults' docs/development-patterns.md)" = "1"
test "$(grep -n 'comma-separated' docs/development-patterns.md | cut -d: -f1)" -gt "81"
test "$(grep -n 'comma-separated' docs/development-patterns.md | cut -d: -f1)" -lt "90"
test "$(grep -n 'getWatchVaults' docs/development-patterns.md | cut -d: -f1)" -gt "81"
test "$(grep -n 'getWatchVaults' docs/development-patterns.md | cut -d: -f1)" -lt "90"
```

If a line-number check fails, the note was placed after the `Commands iterate vaults…` paragraph — move it directly under the `- No flag → all configured vaults` bullet. Do NOT edit the rest of the section to shift line numbers.

**AC8 — the changelog:**

```
test "$(grep '^## ' CHANGELOG.md | head -1)" = "## Unreleased"
test "$(grep -c 'comma-separated vault list' CHANGELOG.md)" -ge 1
```

If the first prints a version heading, the bullet was placed between released sections — move it into the `## Unreleased` section directly below the preamble.

**AC5 substitute — the watching engine is untouched.** The spec's evidence is `git diff origin/master...HEAD --name-only -- pkg/ops/` printing 0 lines, which cannot run here: this container's `.git` is a character device, so every git command fails with `fatal: not a git repository`, and the daemon does not check `<verification>` exit codes — a failing git command would report a false pass. These pins are the substitute and are strictly stronger for the purpose: they fix `pkg/ops/watch.go` byte-for-byte and pin the size of the whole `pkg/ops` Go tree, as they stand on master today. A composite hash over the tree was deliberately rejected — its value depends on the host's `sha256sum` output format and did not reproduce between the authoring machine and this container. Do NOT run git, do NOT edit anything under `pkg/ops/`, and do NOT update the expected values — a mismatch means the engine changed.

```
test "$(sha256sum pkg/ops/watch.go | cut -d' ' -f1)" = "93e81b44586e968eeb8d246dd2a8eb346f932bd194f47b6868aee3cf1e1b9035"
test "$(wc -l < pkg/ops/watch.go)" = "198"
test "$(find pkg/ops -name '*.go' -type f | wc -l)" = "60"
```

**The new integration file exists and actually drives the real binary.** Without this, a missing `integration/watch_test.go` still passes `go test ./integration/...` (the pre-existing specs run) and still satisfies the `gofmt -l` check vacuously:

```
test -f integration/watch_test.go
test "$(grep -c 'gexec.Start' integration/watch_test.go)" -ge 1
test "$(grep -c 'createTwoTempVaults' integration/watch_test.go)" -ge 1
```

**Formatting:**

```
gofmt -e -l pkg/cli/cli.go pkg/cli/export_test.go pkg/cli/watch_test.go integration/watch_test.go
```

must list NO files.

**Tests** (unpiped — never pipe a test command, the pipeline would report the last stage's status):

```
go test ./pkg/cli/...
go test ./integration/...
```

`go test ./integration/...` runs the two-vault subprocess specs: AC1 (`--vault alpha,beta` emits an event for each vault), AC2 (single name and omitted flag), AC3 (both malformed values exit non-zero with no event line), AC4(a) (`task list --vault alpha,beta` still fails with `vault not found: alpha,beta`), the `--types` filter across a comma list, and the missing-task-directory failure mode. If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment — the container sets it in `.dark-factory.yaml`; export it rather than touching `.git`.

**Coverage of the new resolver** must report `100.0%`:

```
go test -coverprofile=/tmp/cover-watch.out ./pkg/cli/... && go tool cover -func=/tmp/cover-watch.out | grep 'getWatchVaults'
```

If it reports below 100.0%, add the missing case from section 6b — do not add unrelated retroactive coverage.

Finally, walk spec 047's Acceptance Criteria 1-8 against the change and state in your completion report which requirement and which test satisfies each one, plus which spec covers each row of the spec's Failure Modes table.
</verification>

<!-- DARK-FACTORY-REPORT -->
