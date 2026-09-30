# Release Drivers

vault-cli runs two release drivers.

- `github-releaser-agent` — reads `.maintainer.yaml: release.autoRelease: true`, rewrites `## Unreleased` to `## vX.Y.Z`, and tags on master.
- dark-factory's per-prompt driver — reads `.dark-factory.yaml: autoRelease: false` and is not opted in for this repo.

Only `github-releaser-agent` is live here.
