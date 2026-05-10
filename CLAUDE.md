# CLAUDE.md

Obsidian vault task management CLI — fast CRUD for markdown files (tasks, goals, themes, visions).

## Dark Factory Workflow

**Never code directly.** All code changes go through the dark-factory pipeline.

### Complete Flow

**Spec-based (multi-prompt features):**

1. Create spec → `/dark-factory:create-spec`
2. Audit spec → `/dark-factory:audit-spec`
3. User confirms → `dark-factory spec approve <name>`
4. dark-factory auto-generates prompts from spec
5. Audit prompts → `/dark-factory:audit-prompt`
6. User confirms → `dark-factory prompt approve <name>`
7. Start daemon → `dark-factory daemon` (use Bash `run_in_background: true`)
8. dark-factory executes prompts automatically

**Standalone prompts (simple changes):**

1. Create prompt → `/dark-factory:create-prompt`
2. Audit prompt → `/dark-factory:audit-prompt`
3. User confirms → `dark-factory prompt approve <name>`
4. Start daemon → `dark-factory daemon` (use Bash `run_in_background: true`)
5. dark-factory executes prompt automatically

### Choosing a Flow

The decision is about **what artifact deserves to be committed alongside the change**, not size or complexity.

| Kind of change | Flow | What gets committed | Why this flow |
|----------------|------|---------------------|---------------|
| Doc / config / yaml — no code | **Direct** — edit + commit yourself | Just the diff | Ceremony adds no value when there are no tests to run and no business "why" to document |
| Code change of any size | **Prompt** — write a prompt, audit, approve, daemon executes | Prompt + diff | The prompt provides structure (tests run, auto-commit, auto-release) and is the technical "how" record. Even small refactors benefit. |
| Feature delivering business value | **Spec → prompts** — write spec, audit, approve, daemon auto-generates prompts, audit each, approve, daemon executes | Spec + prompts + diff | The spec is the durable record of *why this feature exists*. Prompts handle the mechanical breakdown. |

### How to decide

- **Is there code changing?** No → direct. Yes → prompt or spec.
- **Is there a business-level "why" that deserves its own document?** No → prompt is enough. Yes → spec first.

The split between prompt and spec is **business-why vs technical-how**, not big vs small. A 50-prompt mechanical refactor stays prompts. A 1-prompt user-visible feature may still warrant a spec.

Examples: direct = bump K8s memory, fix README typo. Prompt = bug fix, single CLI flag, 5-line refactor with tests. Spec → prompts = user-visible feature across services.

### Read the relevant guide before starting — every time, not from memory

- Writing a spec → read [[Dark Factory - Write Spec]] and [[Dark Factory Guide#Specs What Makes a Good Spec]]
- Writing prompts → read [[Dark Factory - Write Prompts]] and [[Dark Factory Guide#Prompts What Makes a Good Prompt]]
- Running prompts → read [[Dark Factory - Run Prompt]]

### Claude Code Commands

| Command | Purpose |
|---------|---------|
| `/dark-factory:create-spec` | Create a spec file interactively |
| `/dark-factory:create-prompt` | Create a prompt file from spec or task description |
| `/dark-factory:audit-spec` | Audit spec against preflight checklist |
| `/dark-factory:audit-prompt` | Audit prompt against Definition of Done |

### CLI Commands

| Command | Purpose |
|---------|---------|
| `dark-factory spec approve <name>` | Approve spec (inbox → queue, triggers prompt generation) |
| `dark-factory prompt approve <name>` | Approve prompt (inbox → queue) |
| `dark-factory daemon` | Start daemon (watches queue, executes prompts) |
| `dark-factory run` | One-shot mode (process all queued, then exit) |
| `dark-factory status` | Show combined status of prompts and specs |
| `dark-factory prompt list` | List all prompts with status |
| `dark-factory spec list` | List all specs with status |
| `dark-factory prompt retry` | Re-queue failed prompts for retry |

### Key rules

- Prompts go to **`prompts/`** (inbox) — never to `prompts/in-progress/` or `prompts/completed/`
- Specs go to **`specs/`** (inbox) — never to `specs/in-progress/` or `specs/completed/`
- Never number filenames — dark-factory assigns numbers on approve
- Never manually edit frontmatter status — use CLI commands above
- Always audit before approving (`/dark-factory:audit-prompt`, `/dark-factory:audit-spec`)
- **BLOCKING: Never run `dark-factory prompt approve`, `dark-factory spec approve`, or `dark-factory daemon` without explicit user confirmation.** Write the prompt/spec, then STOP and ask the user to approve. Do not assume approval from prior context or task momentum.
- **Before starting daemon** — run `dark-factory status` first to check if one is already running. Only start if not running.
- **Start daemon in background** — use Bash tool with `run_in_background: true` (not foreground, not detached with `&`)
- **After completing a spec or major refactor**, walk the relevant `scenarios/*.md` to verify end-to-end behavior. Always against a freshly built binary, never against the installed `vault-cli`.
- **Before `make install`**, follow [docs/releasing-vault-cli.md](docs/releasing-vault-cli.md) — mandatory reading and procedure. The release gate (run all scenarios against `/tmp/new-vault-cli`) and the version alignment check both live there. Unit tests + `make precommit` alone are not sufficient.

  **Scenario-skip rule (only exception):** Compare installed version to HEAD — if no binary-relevant files changed, the installed binary is byte-equivalent to what you'd install, so scenarios add no signal.
  ```bash
  INSTALLED=$(vault-cli --version | awk '{print $NF}')
  git diff $INSTALLED..HEAD --name-only | grep -E '\.(go|mod|sum)$|^Makefile$'
  # empty output → skip scenarios; any hit → run them
  ```
  Do NOT shortcut by "change type" intuition ("docs-only", "config-only") — always run the diff.

## 🚨 Version Alignment — MANDATORY

**Four version strings MUST always equal each other:**

1. `CHANGELOG.md` — top `## vX.Y.Z` entry (the most-recent versioned section)
2. `.claude-plugin/plugin.json` — `"version"` field
3. `.claude-plugin/marketplace.json` — `metadata.version`
4. `.claude-plugin/marketplace.json` — `plugins[0].version`

`make precommit` runs `check-versions` which fails the build if any of the four diverge. Never commit a release with mismatched versions.

**Every release commit bumps all four together** — even patch releases triggered by the dark-factory daemon. Plugin manifests are NOT decoupled from the binary tag; users running `claude plugin update vault-cli@vault-cli` rely on the manifest version matching the tag.

When the daemon's `autoRelease` ships a new binary version, the prompt that triggered it MUST also bump the three plugin JSON fields — otherwise the next prompt's preflight (`make precommit`) fails.

## Plugin Release Checklist

**When to release:** Any change to `commands/`, `agents/`, `docs/`, or `skills/` requires a plugin version bump — these files ship as part of the plugin.

**How to release:**

1. Pick the next version: increment minor from the latest `CHANGELOG.md` entry (e.g. v0.58.3 → v0.59.0)
2. Update **all four files** — version string must be identical everywhere (without `v` prefix in JSON):
   - `CHANGELOG.md` — add new `## vX.Y.Z` section at top with all changes (binary + plugin)
   - `.claude-plugin/plugin.json` — `"version": "X.Y.Z"`
   - `.claude-plugin/marketplace.json` — `"version": "X.Y.Z"` in **both** `metadata` and `plugins[0]`
3. Commit: `release plugin vX.Y.Z: <summary>`
4. Push: `git push`

**Common mistakes:**
- Forgetting `.claude-plugin/` files (plugin stays at old version)
- Creating a separate "Plugin vX" changelog section (wrong — one version for everything)
- Using different versions in the 3 JSON fields (must all match)
- Not including binary changes (fix, feature) in the changelog when they're uncommitted

Full release procedure: see [docs/releasing-vault-cli.md](docs/releasing-vault-cli.md).

## Claude Code Plugin

Plugin config lives in `.claude-plugin/`. Commands in `commands/`.

| File | Purpose |
|------|---------|
| `.claude-plugin/plugin.json` | Plugin metadata (name, version, license) |
| `.claude-plugin/marketplace.json` | Marketplace listing config |
| `commands/*.md` | Claude Code slash commands |

## Development Standards

This project follows the [coding-guidelines](https://github.com/bborbe/coding-guidelines).

### Key Reference Guides

- **go-architecture-patterns.md** — Interface → Constructor → Struct → Method
- **go-testing-guide.md** — Ginkgo v2/Gomega testing
- **go-makefile-commands.md** — Build commands
- **git-commit-workflow.md** — Commit process with precommit checks
- **go-mocking-guide.md** — Counterfeiter mock generation

### Build and test

- `make precommit` — lint + format + generate + test + checks
- `make test` — tests only

## Architecture & Patterns

See **[docs/development-patterns.md](docs/development-patterns.md)** — architecture, adding commands, multi-vault, output format, testability, naming.

## Key Design Decisions

- `pkg/ops/` is a library layer — operations return structured results, never write to stdout. CLI layer owns all output formatting.
