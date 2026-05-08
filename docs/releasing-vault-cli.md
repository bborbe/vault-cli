# Releasing Vault CLI

How to ship a new version of vault-cli. Mandatory reading before every `make install`.

## Two surfaces, two version streams

Vault-cli ships two artifacts that version independently:

| Surface | Versioned by | Consumed by | Bumped how |
|---------|--------------|-------------|------------|
| **Binary** | git tag `vX.Y.Z` + matching `## vX.Y.Z` section in `CHANGELOG.md` | other projects via `go install github.com/bborbe/vault-cli@latest`; task-orchestrator via configured `vault_cli_path` | Auto-tagged by vault-cli's own daemon (`autoRelease: true`) when a prompt completes and updates `## Unreleased` |
| **Plugin** | `.claude-plugin/plugin.json` `version` + `.claude-plugin/marketplace.json` (`metadata.version` AND `plugins[0].version`) | Claude Code via the marketplace | Manual — operator bumps the three JSON fields |

A single change can touch one surface or both.

## The release gate (run BEFORE every `make install`)

The gate exists because `make precommit` does NOT cover real-vault behavior, vault-cli ↔ filesystem boundaries, or CLI argument parsing seams. Unit tests pass while runtime behavior is broken — and downstream consumers (task-orchestrator, scripts, agents) inherit those breakages immediately.

The rule: **before every `make install`, run all scenarios against a freshly built binary**. No surface-scoped skipping unless the diff is genuinely empty.

```bash
# 1. Build a fresh binary (NOT the installed one)
go build -C ~/Documents/workspaces/vault-cli -o /tmp/new-vault-cli .

# 2. Confirm it built and reports the unreleased version
/tmp/new-vault-cli --version  # should reflect the unreleased state

# 3. Walk every markdown scenario manually against /tmp/new-vault-cli
ls scenarios/*.md  # 001 through 004+; each one's "Action" + "Expected" must pass
```

If any scenario fails: do **not** proceed to install. Fix the regression first, then rerun the gate.

> No `scenarios/helper/run-all.sh` exists yet. Until it does, walk each markdown scenario by hand. When porting scenarios to scripted helpers, follow the dark-factory pattern (`scenarios/helper/run-NNN-all.sh` builds `/tmp/new-vault-cli`, isolates HOME, asserts exit codes).

### When the diff is empty

The one valid skip: nothing on the binary surface changed since the installed binary.

```bash
INSTALLED=$(vault-cli --version | awk '{print $NF}')
git diff "$INSTALLED"..HEAD --name-only | grep -E '\.(go|mod|sum)$|^Makefile$'
# empty output → installed binary is byte-equivalent to /tmp/new-vault-cli → skip
```

This is the ONLY documented skip. Do not invent others ("docs-only changes shouldn't break anything") — surface mappings are fragile.

## Version alignment check (run BEFORE every commit that bumps versions)

Whenever you bump any version (binary tag OR plugin JSON), all related fields must align. Run:

```bash
# Binary alignment: latest tag matches latest CHANGELOG section
LATEST_TAG=$(git tag -l | sort -V | tail -1)
LATEST_CHANGELOG=$(grep -m1 '^## v' CHANGELOG.md | sed 's/^## //')
test "$LATEST_TAG" = "$LATEST_CHANGELOG" && echo "✅ binary aligned" || echo "❌ tag=$LATEST_TAG changelog=$LATEST_CHANGELOG"

# Plugin alignment: three JSON fields must match each other
PLUGIN=$(jq -r .version .claude-plugin/plugin.json)
META=$(jq -r .metadata.version .claude-plugin/marketplace.json)
PLUGINS0=$(jq -r '.plugins[0].version' .claude-plugin/marketplace.json)
test "$PLUGIN" = "$META" -a "$META" = "$PLUGINS0" && echo "✅ plugin aligned ($PLUGIN)" || echo "❌ plugin=$PLUGIN meta=$META plugins[0]=$PLUGINS0"
```

(Future: ship `scripts/check-versions.sh` that runs both checks and exits non-zero on any mismatch.)

Plugin version is independent of the binary tag — the two streams are not required to match each other, only to be internally consistent within their surface.

## Binary release (automatic — but the operator owns the gate)

Vault-cli runs against itself as a daemon with `autoRelease: true` (`.dark-factory.yaml`). Every successful prompt that touches `## Unreleased` triggers:

1. Stage all changes (including the agent's `## Unreleased` entry)
2. Determine bump (patch/minor) from changelog content
3. Rename `## Unreleased` → `## vX.Y.Z`
4. Commit `release vX.Y.Z`
5. Tag `vX.Y.Z`, push tag and commit
6. Move the prompt file to `prompts/completed/` and push that commit too

The operator's responsibility is to **run the release gate before approving any prompt** that may produce a binary change. Once the prompt is approved, the daemon ships whatever the agent produced — there is no second checkpoint.

To verify a release shipped:

```bash
git fetch --tags
git describe --tags --abbrev=0           # latest tag
git log "$(git describe --tags --abbrev=0)"..HEAD --oneline   # any unpushed commits beyond it
```

After a successful auto-release, both `git status` (clean) and `git rev-list @{u}..HEAD --count` (zero) should hold.

## Plugin release (manual)

Whenever any of `commands/`, `agents/`, `docs/`, or `skills/` change, the plugin version must be bumped. The binary's `autoRelease` does **not** bump the plugin version — these JSON files are not part of the binary CHANGELOG-driven flow.

### When to bump

```bash
LAST_PLUGIN_TAG=$(git log --oneline -- .claude-plugin/ | head -1 | awk '{print $1}')
git diff "$LAST_PLUGIN_TAG"..HEAD --name-only -- commands/ agents/ docs/ skills/
# any output → plugin needs a bump
```

### Procedure

1. **Run the release gate** (above) if any binary surface also changed.
2. **Pick the next plugin version.** Increment minor from the latest `CHANGELOG.md` entry. Plugin and binary share the same CHANGELOG and the same monotonic version sequence.
3. **Update all three plugin fields** to the new version (no `v` prefix in JSON):
   - `.claude-plugin/plugin.json` `"version"`
   - `.claude-plugin/marketplace.json` `metadata.version`
   - `.claude-plugin/marketplace.json` `plugins[0].version`
4. **Add a `## vX.Y.Z` section** to `CHANGELOG.md` at the top, covering all changes since the previous entry (binary AND plugin in the same section — there is one CHANGELOG, not two).
5. **Run the version alignment check** (above) — must report `✅ plugin aligned`.
6. **Commit:** `git commit -m "release plugin vX.Y.Z: <summary>"`.
7. **Push:** `git push`.

### Common plugin-release mistakes

- Forgetting `.claude-plugin/` files — CHANGELOG advances but plugin stays at old version.
- Creating a separate "Plugin vX" CHANGELOG section. Wrong — one CHANGELOG, one version sequence.
- Different version strings across the three JSON fields. The marketplace rejects mismatches silently and refuses to load the plugin.
- Bumping the plugin version BEFORE running the release gate. Binary surface changes that ship in the same release escape scenario coverage.

## Install (the moment the new version reaches consumers)

```bash
make install            # local install via Makefile
# or
go install github.com/bborbe/vault-cli@latest
vault-cli --version     # should now match the latest tag
```

This is the step that bites consumers if the gate was skipped. Task-orchestrator and any scripts using `vault-cli` will pick up the new binary the next time they invoke it. A regression surfaces in their workflow, not yours.

The plugin's install is automatic via the marketplace once the bumped JSON files reach `master` — Claude Code re-checks the marketplace periodically.

## See also

- [development-patterns.md](development-patterns.md) — architecture, adding commands, multi-vault, output format, testability
- `CLAUDE.md` "Release Checklist" — the concise rule that points back to this doc
- `scenarios/` — the regression suite this gate runs
