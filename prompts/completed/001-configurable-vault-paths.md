---
status: completed
---

<objective>
Add per-vault folder configuration to vault-cli so the storage layer uses configurable directory names instead of hardcoded paths. Create ~/.vault-cli/config.yaml for the Personal and Brogrammers Obsidian vaults.
</objective>

<context>
Project: ~/Documents/workspaces/vault-cli (Go CLI tool for Obsidian vault task management)
Read CLAUDE.md and pkg/config/config.go, pkg/storage/markdown.go before making changes.

The two Obsidian vaults have different folder structures:

Personal vault (~Documents/Obsidian/Personal):
  - Tasks:      "24 Tasks"
  - Goals:      "23 Goals"
  - Daily notes: "60 Periodic Notes/Daily"

Brogrammers vault (~Documents/Obsidian/Brogrammers):
  - Tasks:      "40 Tasks"
  - Goals:      (none / skip)
  - Daily notes: "60 Periodic Notes/Daily"

Currently the storage layer hardcodes: "Tasks", "Goals", "Daily Notes" — these don't match either vault.
</context>

<requirements>
1. Extend Vault struct in pkg/config/config.go with optional folder overrides:
   - TasksDir   string (default: "Tasks")
   - GoalsDir   string (default: "Goals")
   - DailyDir   string (default: "Daily Notes")

2. Storage layer must use these configured paths, not hardcoded strings.
   Pass folder paths into storage or accept a VaultConfig in storage methods.
   Preferred: add a VaultConfig parameter to NewStorage() or create a StorageConfig struct.

3. Update CLI (pkg/cli/cli.go) to load VaultConfig and pass it to storage.

4. Create ~/.vault-cli/config.yaml:
   default_vault: personal
   vaults:
     personal:
       name: personal
       path: ~/Documents/Obsidian/Personal
       tasks_dir: "24 Tasks"
       goals_dir: "23 Goals"
       daily_dir: "60 Periodic Notes/Daily"
     brogrammers:
       name: brogrammers
       path: ~/Documents/Obsidian/Brogrammers
       tasks_dir: "40 Tasks"
       daily_dir: "60 Periodic Notes/Daily"

5. Update mocks (run `make generate` or counterfeiter manually if needed).

6. Maintain backward compatibility: if dirs not set in config, use defaults.
</requirements>

<implementation>
- Follow existing patterns in pkg/config/config.go and pkg/storage/markdown.go
- Read go-architecture-patterns.md in ~/Documents/workspaces/coding-guidelines/ for struct/interface patterns
- Keep Storage interface unchanged if possible — thread VaultConfig via constructor or a new StorageConfig
- Don't break existing unit tests; update them if signatures change
</implementation>

<verification>
1. Run `make test` — all tests must pass
2. Build the binary: `go build -o /tmp/vault-cli .`
3. Run end-to-end test against Personal vault:
   /tmp/vault-cli complete "Build vault-cli Go Tool"
   Check that ~/Documents/Obsidian/Personal/24\ Tasks/Build\ vault-cli\ Go\ Tool.md has status: completed
4. Run defer test:
   /tmp/vault-cli defer "Add Focus Jam2 7.9 to inventory" +1d
   Check that defer_date is set in the task file
5. Run `make precommit` for full validation
</verification>

<success_criteria>
- Vault struct has TasksDir, GoalsDir, DailyDir fields with defaults
- Storage uses configured paths, not hardcoded strings
- ~/.vault-cli/config.yaml created with both vaults
- make test passes
- make precommit passes
- End-to-end vault-cli complete works against Personal vault
</success_criteria>
