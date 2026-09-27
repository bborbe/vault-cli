# Releasing Vault CLI

How to ship a new version of vault-cli. Mandatory reading before every `make install`.

## Two surfaces, two version streams

Vault-cli ships two artifacts that version independently:

| Surface | Versioned by | Consumed by | Bumped how |
|---------|--------------|-------------|------------|
| **Binary** | git tag `vX.Y.Z` + matching `## vX.Y.Z` section in `CHANGELOG.md` | other projects via `go install github.com/bborbe/vault-cli@latest`; task-orchestrator via configured `vault_cli_path` | Auto-tagged by `github-releaser-agent` (Driver 1) via `.maintainer.yaml: release.autoRelease: true` after `## Unreleased` bullets land on `master` |
| **Plugin** | `.claude-plugin/plugin.json` `version` + `.claude-plugin/marketplace.json` (`metadata.version` AND `plugins[0].version`) | Claude Code via the marketplace | Auto-bumped by `github-releaser-agent` (Driver 1) in lockstep with the CHANGELOG while `.maintainer.yaml: release.autoRelease: true`; manual only as fallback |

A single change can touch one surface or both.

## 🚨 Version alignment — locked at release time only

All four version strings MUST equal each other **at release time**:

1. `CHANGELOG.md` — top `## vX.Y.Z` entry
2. `.claude-plugin/plugin.json` — `"version"`
3. `.claude-plugin/marketplace.json` — `metadata.version`
4. `.claude-plugin/marketplace.json` — `plugins[0].version`

The check is **release-time only** — `make precommit` does NOT run it. Use `make release-check` (or `make check-versions` directly) before tagging.

**Why not in `precommit`**: every refactor commit advances `## Unreleased` → eventually a `## vX.Y.Z` heading; if every prompt had to bump plugin JSONs in lockstep, each refactor would consume a release number. We learned this the hard way during spec 010 — three prompts auto-bumped plugin versions just to clear the precommit gate, burning v0.58.7 → v0.59.0 → v0.59.1 on internal refactors.

**The dark-factory driver (Driver 2) is off here**: `.dark-factory.yaml: autoRelease: false`, flipped in `26e62a2` (2026-05-31), so the daemon produces no binary release and cannot leave the plugin JSONs lagging behind. Driver 1 bumps all four strings together (below), so alignment holds with no operator action.

**Driver 1 (`github-releaser-agent`) is different** — it bumps all four strings together, so a post-merge release leaves them aligned with no operator action. That is the path a normal PR takes.

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

## Binary release — one active driver, one dormant

Vault-cli's binary tag comes from **one** flow: `github-releaser-agent` (Driver 1), opted in through `.maintainer.yaml: release.autoRelease: true`. The dark-factory daemon's own release flag (Driver 2) is **off** here — `.dark-factory.yaml: autoRelease: false`, flipped in `26e62a2` (2026-05-31) when the releaser took ownership. Driver 2 is described below so the setting is legible, not because it fires in this repo.

### Driver 1: `github-releaser-agent` (canonical, post-merge)

`.maintainer.yaml: release.autoRelease: true` opts the repo in. After any commit lands on `master` carrying `## Unreleased` bullets in `CHANGELOG.md`, the watcher emits a `CreateTaskCommand` and the agent:

1. Classifies the semver bump from the `## Unreleased` bullet prefixes (`feat:` → minor, `fix:` → patch, `BREAKING:` → major)
2. Rewrites `## Unreleased` → `## vX.Y.Z`
3. Bumps the four version strings (CHANGELOG + `.claude-plugin/plugin.json` + `.claude-plugin/marketplace.json` × 2) in lockstep
4. Commits `release vX.Y.Z`, tags `vX.Y.Z`, pushes tag + commit

Picks up changes within ~10 min of the merge (watcher poll interval). To force an immediate scan: trigger via the maintainer-watcher `/trigger` endpoint or the `/github-release-repo-trigger` runbook.

**Operator's job in this flow**: keep `## Unreleased` bullets accurate, commit + push to master. **Do NOT** rename `## Unreleased` → `## vX.Y.Z`, **do NOT** bump version strings, **do NOT** create a local tag — the bot owns the entire release commit. Local versions of any of those steps race the bot.

### Driver 2: dark-factory `autoRelease` — **off in this repo**

`.dark-factory.yaml: autoRelease: false`, flipped in `26e62a2` (2026-05-31), the same commit that added `.maintainer.yaml` and moved release ownership to `github-releaser-agent`. The daemon therefore does **not** tag or push a release on prompt completion here: it runs prompts, and release commits come from Driver 1 after the merge.

This flag governs the daemon's own release behaviour — whether it tags a release when a prompt completes — not the repo's release procedure, which is Driver 1's. It stays documented so the setting is legible. Turning it on would race Driver 1 for the same version number, which is why `26e62a2` moved ownership instead of enabling both.

### When each driver fires

| Scenario | dark-factory driver (`.dark-factory.yaml: autoRelease: false`) | github-releaser-agent driver (`.maintainer.yaml: release.autoRelease: true`) |
|---|---|---|
| Daemon runs a prompt on a feature branch, prompt completes | does NOT fire — the flag is off | does NOT fire (commits not on master yet) |
| Daemon runs a prompt with `--set autoRelease=true` | still does NOT fire — the repo flag is off and this override is not used here | fires after the feature branch merges to master |
| Direct PR + merge (no dark-factory at all) | does NOT fire (no daemon involvement) | fires |
| Daemon runs on master directly | does NOT fire — the flag is off | fires post-merge |

Driver 1 is the only flow that ships a tag in this repo, so there is no second driver to fall back on: a release that does not appear within ~10 min of the merge means Driver 1 did not run, and the manual procedure below is the fallback.

### Verifying a release shipped

```bash
git fetch --tags
git describe --tags --abbrev=0           # latest tag
git log "$(git describe --tags --abbrev=0)"..HEAD --oneline   # any unpushed commits beyond it
```

After a successful release, both `git status` (clean) and `git rev-list @{u}..HEAD --count` (zero) should hold.

The operator's responsibility is the **release gate** (above): build `/tmp/new-vault-cli` and walk `scenarios/*.md` before every `make install` of the released tag. `github-releaser-agent` does not run the scenarios — that gate is operator-side.

## GitHub Release (manual — when to surface a milestone)

`.maintainer.yaml: release.autoRelease: true` makes `github-releaser-agent` create a `vX.Y.Z` git tag after every merge to master. Tags are sufficient for `go install github.com/bborbe/vault-cli@vX.Y.Z`, `git describe`, and any tag-aware consumer.

A **GitHub Release** is a separate, deliberate act — distinct from the tag. It adds release notes, an entry on the repo's Releases tab, an RSS/atom feed for subscribers, and optional binary assets.

**Publishing a Release also ships the Homebrew cask.** `.github/workflows/release.yml` triggers on `release: published` and runs goreleaser, which builds darwin/linux archives, attaches them to this Release, and pushes the cask to [`bborbe/homebrew-tap`](https://github.com/bborbe/homebrew-tap). So publishing here is what makes `brew install bborbe/tap/vault-cli` serve the new version.

**A tag alone never reaches brew.** This is deliberate: Driver 1 tags every merge, and publishing a cask per tag would bypass the scenario gate. The gate below is the only thing between a merge and a Homebrew user.

The corollary of "skip the Release for internal refactors" (below) is that brew users stay on the last promoted version until you promote again. That is the intended trade: `go install @latest` is the fast track, brew is the verified one.

Create a Release **only after**:

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
  --notes "$(awk -v tag="## $TAG" '$0 == tag {f=1; next} /^## v/ {f=0} f' CHANGELOG.md)"
```

Do not "simplify" that `awk` into a range like `awk "/^## $TAG/,/^## v/"`. A range
evaluates its END pattern on the same record where the START matched, and the
`## vX.Y.Z` heading matches `^## v` itself — so the range opens and closes on that
one line, and the old `| head -n -1` then stripped it, yielding **empty notes**.
That is why earlier releases carry no hand-written notes. The flag form above
skips the heading and stops at the next one.

Verify on github.com → Releases tab. The Release object can be edited (notes, draft state) without retagging.

Then confirm the cask actually shipped — publishing the Release only *starts* the workflow:

```bash
# 1. The release workflow ran and succeeded
gh run list --workflow=release.yml --limit 1

# 2. goreleaser attached archives to THIS release
gh release view "$TAG" --json assets --jq '.assets[].name'

# 3. The cask landed in the tap at the new version
gh api repos/bborbe/homebrew-tap/contents/Casks/vault-cli.rb --jq '.content' \
  | base64 -d | grep -E '^\s*version'

# 4. End-to-end: brew actually serves it
brew update && brew install bborbe/tap/vault-cli && vault-cli --version
```

If step 1 shows no run, the Release was created as a **draft** — drafts do not fire `release: published`. Publish it (`gh release edit "$TAG" --draft=false`) and the workflow will trigger.

## Plugin release

Whenever any of `commands/`, `agents/`, `docs/`, or `skills/` change, the plugin version must be bumped — the three `.claude-plugin/` JSON fields plus the CHANGELOG entry, all to the same version.

**While `.maintainer.yaml: release.autoRelease: true` (the current state), `github-releaser-agent` does this for you.** Driver 1 above bumps all four version strings in lockstep as part of the release commit. Your job is the `## Unreleased` bullet; the bot owns everything after the merge.

> **Do not hand-bump the JSONs on a `.maintainer.yaml: release.autoRelease: true` repo.** A local bump + tag races the releaser for the same version number. This is the same rule as `CLAUDE.md`'s Plugin Release Checklist, stated there as "the manual checklist is the FALLBACK only".

Verified empirically on **v0.109.0** (2026-08-16, PR #79 — a `commands/`-only change): nothing was hand-bumped, and the released tag carried `CHANGELOG.md ## v0.109.0`, `plugin.json 0.109.0`, and both `marketplace.json` fields at `0.109.0`.

### Checking whether a bump is owed

Useful when auditing a release after the fact, or before switching `.maintainer.yaml: release.autoRelease` off:

```bash
LAST_PLUGIN_TAG=$(git log --oneline -- .claude-plugin/ | head -1 | awk '{print $1}')
git diff "$LAST_PLUGIN_TAG"..HEAD --name-only -- commands/ agents/ docs/ skills/
# any output → plugin surface changed since the last .claude-plugin/ commit
```

Under Driver 1 this should come back empty shortly after each merge. Persistent output means the releaser is not running — investigate before falling back to the manual procedure.

### Manual procedure (FALLBACK — releaser down, or `.maintainer.yaml: release.autoRelease: false`)

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

Confirm the releaser really is down first (`/github-release-repo-trigger` produced no tag, or `.maintainer.yaml` has `autoRelease: false`). Running this while the bot is healthy is the race the warning above describes.

### Common plugin-release mistakes

- **Hand-bumping while `.maintainer.yaml: release.autoRelease: true` is on.** The most likely mistake now that Driver 1 owns the bump — it races the releaser for the version number.
- Forgetting `.claude-plugin/` files (manual path only) — CHANGELOG advances but plugin stays at old version.
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
