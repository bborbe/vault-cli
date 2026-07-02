---
status: approved
spec: [022-xdg-config-migration]
created: "2026-07-02T12:00:00Z"
queued: "2026-07-02T11:54:16Z"
branch: dark-factory/xdg-config-migration
---

<summary>
- Adds unit tests proving the XDG-first config directory lookup behaves correctly in all three cases
- Verifies XDG dir wins when it exists, legacy dir is used when only it exists, and the XDG default is returned when neither exists
- Verifies XDG wins even when BOTH directories exist (the documented priority tie-break)
- Adds an integration test proving `Load()` reads a real config file placed in an XDG-shaped directory when no explicit path is given
- Tests use isolated temporary HOME directories so they never touch the developer's real config
- Existing config tests continue to pass unchanged
</summary>

<objective>
Add Ginkgo/Gomega tests to `pkg/config/` that exercise `FindConfigDir` (all four cases from the spec's Failure Modes + Desired Behavior) and one integration test that drives `Load()` through the new XDG lookup with an empty config path. This is the test-coverage prompt for spec 022; the implementation ships in prompt 1.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files fully before writing tests:
- `pkg/config/config.go` — the implementation. Confirm the exported signature is `func FindConfigDir(ctx context.Context, toolName string) (string, error)` and that `Load` calls `FindConfigDir(ctx, "vault-cli")` then joins `config.yaml`. If prompt 1 has NOT landed yet (grep returns nothing for `func FindConfigDir`), STOP and report `status: failed` with message `"FindConfigDir not yet deployed (prompt 1)"` — do NOT invent the function.
- `pkg/config/config_test.go` — existing test style: `package config_test`, Ginkgo `Describe`/`Context`/`It`, `os.MkdirTemp` in `BeforeEach`, `os.RemoveAll` in `AfterEach`, Gomega matchers (`Expect(...).To(BeNil())`, `HaveSuffix`, `HavePrefix`, `ContainSubstring`).
- `pkg/config/config_suite_test.go` — the suite runner; already wires Ginkgo. Do NOT add a second `RunSpecs`.

Verified facts:
- Tests live in external package `config_test` and import `github.com/bborbe/vault-cli/pkg/config`.
- `context`, `os`, `path/filepath` are already imported in `config_test.go`; Ginkgo/Gomega dot-imports are present.

Relevant guides (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2/Gomega, coverage ≥80%, error-path testing, external test package.
</context>

<requirements>
1. Add a new test file `pkg/config/find_config_dir_test.go` in `package config_test` with the standard copyright header (copy the 3-line header from the top of `pkg/config/config_test.go`) and a `Describe("FindConfigDir", ...)` block.

2. Isolate HOME per test. `FindConfigDir` calls `os.UserHomeDir()`, which on Linux reads `$HOME`. In `BeforeEach`, create a temp dir and override `HOME`; in `AfterEach`, restore the original `HOME` and remove the temp dir. Pattern:

   ```go
   var (
       ctx      context.Context
       tempHome string
       origHome string
   )

   BeforeEach(func() {
       ctx = context.Background()
       origHome = os.Getenv("HOME")
       var err error
       tempHome, err = os.MkdirTemp("", "vault-cli-findconfig-*")
       Expect(err).To(BeNil())
       Expect(os.Setenv("HOME", tempHome)).To(BeNil())
   })

   AfterEach(func() {
       Expect(os.Setenv("HOME", origHome)).To(BeNil())
       _ = os.RemoveAll(tempHome)
   })
   ```

3. Add these `It` cases inside `Describe("FindConfigDir", ...)`. Use `filepath.Join(tempHome, ...)` to create the directories with `os.MkdirAll(dir, 0700)`:

   a. **XDG dir exists** — create `filepath.Join(tempHome, ".config", "vault-cli")`. Call `dir, err := config.FindConfigDir(ctx, "vault-cli")`. Assert `err` is `nil` and `dir` equals `filepath.Join(tempHome, ".config", "vault-cli")` (use `Equal`, since HOME is a known temp dir the full path is deterministic).

   b. **Only legacy dir exists** — create `filepath.Join(tempHome, ".vault-cli")` (do NOT create `.config/vault-cli`). Assert `dir` equals `filepath.Join(tempHome, ".vault-cli")`.

   c. **Neither dir exists** — create nothing. Assert `dir` equals `filepath.Join(tempHome, ".config", "vault-cli")` (XDG default).

   d. **Both dirs exist → XDG wins** — create BOTH `filepath.Join(tempHome, ".config", "vault-cli")` and `filepath.Join(tempHome, ".vault-cli")`. Assert `dir` equals `filepath.Join(tempHome, ".config", "vault-cli")`.

   e. **A file (not a dir) at the XDG path is ignored** — create `filepath.Join(tempHome, ".config")` as a dir, then write a *file* at `filepath.Join(tempHome, ".config", "vault-cli")` via `os.WriteFile(..., []byte("x"), 0600)`, and also create the legacy dir `filepath.Join(tempHome, ".vault-cli")`. Assert `dir` equals the legacy dir (the XDG path is a file, so `IsDir()` is false and lookup falls through to legacy).

4. Add an integration test for `Load()` through the XDG path. This can go in the same `find_config_dir_test.go` file under a separate `Describe("Load via FindConfigDir", ...)` block, reusing the same HOME-override `BeforeEach`/`AfterEach` (declare them within that Describe, or share one outer set — keep it self-contained and compiling):

   - In the test body: create `xdgDir := filepath.Join(tempHome, ".config", "vault-cli")` with `os.MkdirAll(xdgDir, 0700)`.
   - Write a valid config to `filepath.Join(xdgDir, "config.yaml")` with `os.WriteFile(..., 0600)`. Content:
     ```yaml
     current_user: xdg@example.com
     default_vault: main
     vaults:
       main:
         name: main
         path: /vault/main
     ```
   - Construct `loader := config.NewLoader("")` (EMPTY path — forces the `FindConfigDir` branch).
   - Call `cfg, err := loader.Load(ctx)`. Assert `err` is `nil`, `cfg.CurrentUser` equals `"xdg@example.com"`, and `cfg.Vaults` has key `"main"`.
   - This proves the file placed in the XDG-shaped dir is actually resolved and read by `Load()`, crossing the real `FindConfigDir` → `filepath.Join` → `os.ReadFile` → `yaml.Unmarshal` boundary.

5. Do NOT modify `config_test.go`, `config_suite_test.go`, or `vault_test.go`. Do NOT change any production code — if a test reveals a bug, report it in `## Improvements` rather than editing `config.go` in this prompt (prompt 1 owns the implementation).

6. Confirm coverage for the changed package is ≥80%:
   ```
   go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/config/... && go tool cover -func=/tmp/cover.out | grep -E 'FindConfigDir|Load'
   ```
</requirements>

<constraints>
- Tests are external (`package config_test`), Ginkgo/Gomega, counterfeiter for mocks (none needed here).
- Override `HOME` per test and always restore it in `AfterEach` — never leak env mutation across tests or touch the real `~/.config`.
- All existing tests in `pkg/config/` must continue to pass unchanged.
- Do NOT change production code in this prompt.
- Do NOT add an env-var override or CLI flag (test-only prompt; nothing new in production).
- Wrap nothing new in production; this is tests only.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run in the repo root:

```
make test
make precommit
```

Both must exit 0. Confirm the new tests are present and run:

```
grep -n 'Describe("FindConfigDir"' pkg/config/find_config_dir_test.go
go test -mod=vendor ./pkg/config/... -run TestSuite -count=1
```
</verification>
