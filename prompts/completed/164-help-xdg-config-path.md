---
status: completed
summary: Document XDG config path in vault-cli --help output, extend integration test, add CHANGELOG entry
execution_id: vault-cli-help-xdg-exec-164-help-xdg-config-path
dark-factory-version: v0.191.0
created: "2026-07-11T13:18:26Z"
queued: "2026-07-11T13:18:26Z"
started: "2026-07-11T13:19:09Z"
completed: "2026-07-11T13:20:33Z"
---

<summary>
- `vault-cli --help` now tells users where the config file lives
- The XDG config path and its legacy fallback are shown in the root command's help description
- It also notes the `--config` flag overrides the default location
- No behavior change: config resolution already works XDG-first with legacy fallback; this only surfaces the paths in help text
- Existing commands, flags, and subcommand help stay exactly as they are
- The existing `--help` integration test is extended to assert the config path is shown
</summary>

<objective>
Make the config file location discoverable from `vault-cli --help` so users no longer have to read docs or guess where to put their configuration. Config resolution is unchanged — this only documents the already-active XDG-first path in the help output.
</objective>

<context>
Read CLAUDE.md for project conventions.
This CLI uses cobra (`github.com/spf13/cobra`); top-level help is rendered from the root command's fields, not hand-rolled printing.
Read `pkg/cli/cli.go` — find `NewRootCommand`. The root `*cobra.Command` currently has `Long: "Fast CRUD operations for Obsidian markdown files (tasks, goals, themes)."`. Cobra renders the `Long` field in `--help` output. The `--config` persistent flag is registered here with description `"Config file path"`.
Read `pkg/config/config.go` — `FindConfigDir(ctx, "vault-cli")` resolves the config dir XDG-first: `~/.config/vault-cli/` (XDG) with fallback to `~/.vault-cli/` (legacy); `config.yaml` is appended. Its doc comment describes the XDG-first + legacy-fallback behavior. Use those exact paths.
Read `integration/cli_test.go` — the `Describe("vault-cli --help", ...)` block execs the built binary with `--help` and asserts on `session.Out` via `gbytes.Say(...)`. This is the natural home for the new assertion. The test package is external (`package integration_test`); it uses Ginkgo/Gomega + gexec/gbytes.
</context>

<requirements>
1. In `pkg/cli/cli.go`, in `NewRootCommand`, extend the root command's `Long` field so `vault-cli --help` documents the config file location. Keep the existing sentence and append a short configuration note naming both paths, for example:
   `Fast CRUD operations for Obsidian markdown files (tasks, goals, themes).\n\nConfiguration: reads ~/.config/vault-cli/config.yaml (XDG), falling back to ~/.vault-cli/config.yaml (legacy). Override with --config.`
   Paths must be exact: `~/.config/vault-cli/config.yaml` and `~/.vault-cli/config.yaml`.
2. Do not change existing command descriptions, subcommands, or the behavior of the `--config` flag. Only the `Long` string changes.
3. In `integration/cli_test.go`, extend the existing `Describe("vault-cli --help", ...)` spec (matching its Ginkgo/Gomega style) to assert the help output contains the XDG config path. IMPORTANT: do NOT append another `gbytes.Say(...)` — `gbytes.Say` is a streaming matcher that advances the buffer past each prior match, and the existing spec already matches `vault-cli` (which now also appears inside the config path), so an appended Say would consume past the path and fail. Assert order-independently instead: after the session exits, `Expect(string(session.Out.Contents())).To(ContainSubstring("/.config/vault-cli/config.yaml"))`.
4. Add a `## Unreleased` section to `CHANGELOG.md`, placed BELOW the preamble and immediately ABOVE the newest version section (`## v0.99.1`) — never above the preamble (the `scripts/check-changelog.sh` gate wired into `make check` fails otherwise). Single bullet, e.g. `docs(help): document XDG config path (~/.config/vault-cli/config.yaml, legacy fallback ~/.vault-cli/config.yaml) in vault-cli --help output`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- Paths must be exact: `~/.config/vault-cli/config.yaml` (XDG) and `~/.vault-cli/config.yaml` (legacy). Do NOT add `XDG_CONFIG_HOME` env-var override support — that is explicitly out of scope.
- Scope is the top-level root help only; per-subcommand help is out of scope.
</constraints>

<verification>
Run `make precommit` -- must pass (includes the extended integration help test).
</verification>
