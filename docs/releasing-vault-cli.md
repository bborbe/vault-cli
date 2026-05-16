# Releasing Vault CLI

How to ship a new version of vault-cli. Mandatory reading before every `make install`.

## Two surfaces, two version streams

Vault-cli ships two artifacts that version independently:

| Surface | Versioned by | Consumed by | Bumped how |
|---------|--------------|-------------|------------|
| **Binary** | git tag `vX.Y.Z` + matching `## vX.Y.Z` section in `CHANGELOG.md` | other projects via `go install github.com/bborbe/vault-cli@latest`; task-orchestrator via configured `vault_cli_path` | Auto-tagged by vault-cli's own daemon (`autoRelease: true`) when a prompt completes and updates `## Unreleased` |
| **Plugin** | `.claude-plugin/plugin.json` `version` + `.claude-plugin/marketplace.json` (`metadata.version` AND `plugins[0].version`) | Claude Code via the marketplace | Manual — operator bumps the three JSON fields |

A single change can touch one surface or both.

## 🚨 Version alignment — locked at release time only

All four version strings MUST equal each other **at release time**:

1. `CHANGELOG.md` — top `## vX.Y.Z` entry
2. `.claude-plugin/plugin.json` — `"version"`
3. `.claude-plugin/marketplace.json` — `metadata.version`
4. `.claude-plugin/marketplace.json` — `plugins[0].version`

The check is **release-time only** — `make precommit` does NOT run it. Use `make release-check` (or `make check-versions` directly) before tagging.

**Why not in `precommit`**: every refactor commit advances `## Unreleased` → eventually a `## vX.Y.Z` heading; if every prompt had to bump plugin JSONs in lockstep, each refactor would consume a release number. We learned this the hard way during spec 010 — three prompts auto-bumped plugin versions just to clear the precommit gate, burning v0.58.7 → v0.59.0 → v0.59.1 on internal refactors.

**Implication for `autoRelease`**: when a prompt produces a binary release (CHANGELOG bump → tag), the plugin JSONs may lag behind. Operator runs `make release-check` before producing a plugin release and bumps the JSONs to match the latest CHANGELOG entry at that time.

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

## Version alignment check (release-time)

`scripts/check-versions.sh` enforces the locked model: top CHANGELOG entry == plugin.json `version` == marketplace.json `metadata.version` == marketplace.json `plugins[0].version`. Run directly, via `make check-versions`, or via `make release-check` (which adds `make precommit` first).

```bash
make release-check          # full gate: precommit + check-versions
# or, just the version check:
make check-versions
# or:
bash scripts/check-versions.sh
```

**NOT wired into `make precommit`** — see the "Version alignment" section above for why.

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

## GitHub Release (manual — when to surface a milestone)

`autoRelease` creates a `vX.Y.Z` git tag after every approved prompt. Tags are sufficient for `go install github.com/bborbe/vault-cli@vX.Y.Z`, `git describe`, and any tag-aware consumer.

A **GitHub Release** is a separate, deliberate act — distinct from the tag. It adds release notes, an entry on the repo's Releases tab, an RSS/atom feed for subscribers, and optional binary assets. Create one **only after**:

1. All `scenarios/` pass against the current source tree.
2. Plugin JSONs are aligned (if `commands/`, `agents/`, `docs/`, or `skills/` changed since the last plugin release).
3. The `CHANGELOG.md` entry summarises what users should care about — not the internal commit log.

Skip the GitHub Release for internal refactors, pre-release/experimental work, or chains of small tags. It is fine to skip several auto-tags and cumulate them into a single milestone Release later.

How:

```bash
TAG=$(git describe --tags --abbrev=0)
gh release create "$TAG" \
  --target master \
  --title "$TAG" \
  --notes "$(awk "/^## $TAG/,/^## v/" CHANGELOG.md | head -n -1)"
```

Verify on github.com → Releases tab. The Release object can be edited (notes, draft state) without retagging.

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
5. **Run `make release-check`** (above) — must pass precommit AND report `✅ plugin aligned`.
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
