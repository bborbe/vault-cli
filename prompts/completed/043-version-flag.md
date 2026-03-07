---
status: completed
summary: Added --version flag to vault-cli via cobra's built-in Version field and ldflags injection in the Makefile install target; all changes were already present in the working tree.
container: vault-cli-043-version-flag
dark-factory-version: v0.26.0
created: "2026-03-07T23:21:05Z"
queued: "2026-03-07T23:21:05Z"
started: "2026-03-07T23:21:56Z"
completed: "2026-03-07T23:25:01Z"
---
<summary>
- Users can check which version of the binary is installed via a standard flag
- Builds from a tagged release report the tag as the version
- Builds without a release tag report "dev" by default
- No new subcommands added
</summary>

<objective>
`vault-cli --version` prints the current version so users can verify which build is installed.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/cli/cli.go — `Run` function, `rootCmd` cobra.Command definition.
Read main.go — entry point.
Read Makefile — build targets (especially `install` target).
</context>

<requirements>
1. Add a package-level variable in `pkg/cli/cli.go`:
   ```go
   var version = "dev"
   ```

2. Set `Version: version` on the `rootCmd` cobra.Command struct literal. Cobra automatically adds `--version` flag support when `Version` is set. Example:
   ```go
   rootCmd := &cobra.Command{
       Use:          "vault-cli",
       Short:        "Obsidian vault task management CLI",
       Version:      version,
       // ... rest unchanged
   }
   ```

3. Update `Makefile` to inject the version via ldflags when building. Use the latest git tag:
   ```makefile
   VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
   LDFLAGS := -X github.com/bborbe/vault-cli/pkg/cli.version=$(VERSION)
   ```
   Add `-ldflags "$(LDFLAGS)"` to the existing `install` target.
</requirements>

<constraints>
- Do NOT add a separate `version` subcommand — use cobra's built-in `--version` flag only
- Do NOT import additional packages for version handling
- Do NOT change any existing command behavior
- Do NOT commit — dark-factory handles git
- Do NOT run `make precommit` iteratively — use `make test`; run `make precommit` once at the very end
</constraints>

<verification>
Run: `make test`
Run: `make precommit`
Run: `make install && vault-cli --version`
Confirm:
- Output contains a version string (git tag or "dev")
- All existing tests still pass
- No lint errors
</verification>
