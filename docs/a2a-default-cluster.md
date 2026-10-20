# A2A Default Cluster

A vault's config entry can name the cluster that an A2A worker for that vault
falls back to when the task carries no explicit spawn target. The value is
stored under the key `a2a_default_cluster` and set with:

```
vault-cli config set-a2a-default-cluster <vault> <cluster>
```

The value is a cluster **name**, not a spawn target: `nukedev`, never
`k8s:nukedev`. The consumer forms the target from the name, so a value
containing a colon is refused rather than passed through.

## What this repo does and does not do

This repo stores a default, validates it on write, and emits it. It does not
consume it and does not enforce anything.

- **Consuming it** — resolving an absent spawn target to the vault default,
  forming `k8s:<name>`, and refusing an unknown cluster — belongs to the
  consumer (`claude-supervisor`'s `open-a2a` call site).
- **Enforcing an allowlist** — which clusters a vault *may* use — is a
  supervisor-side fact. This repo records a default, not a permission.
- **The cluster-name set is not frozen.** `nukedev`, `nukeprod`, `octopus-dev`
  and the rest are today's names. This repo validates the shape only, so an
  unknown-but-well-formed name is accepted here and refused by the consumer.
- **Setting the per-vault values** is an operator act performed on the host
  against the real config file, not something this repo seeds.

## Validation

The value is validated as an **RFC 1123 label**: lowercase alphanumerics and
interior hyphens, starting and ending with an alphanumeric, at most 63
characters — the shape a Kubernetes cluster identifier takes.

Accepted: `nukedev`, `nukeprod`, `octopus-dev`, `my-cluster-2`, a single
character, and any name up to 63 characters.

Refused: a target string (`k8s:nukedev`), uppercase (`Nukedev`), an underscore
(`nuke_dev`), a space, a dot, a slash, a non-ASCII name, a leading or trailing
hyphen, a 64-character name, and the empty string. A refused value or an unknown
vault writes nothing at all — the config file is byte-identical afterwards.

## Read-time tolerance

**Reading never validates.** A value that is already on disk is emitted
verbatim, whatever it looks like — `config list --output json` still exits 0
for a hand-edited `a2a_default_cluster: "not a cluster!"`.

This is deliberate: refusing a malformed value at read time would turn a config
typo into an unopenable vault, and every command that loads the config would
fail. An unknown cluster is the consumer's to refuse, never this repo's list
path.

## Where it lives

- The on-disk key is `a2a_default_cluster` in the vault-cli config file
  (`~/.config/vault-cli/config.yaml` by default).
- The JSON spelling is the same string, and it is what a consumer reads from
  `config list --output json`. Both spellings are part of the contract: the
  JSON one is the delivery surface, the YAML one is the on-disk surface.
- **Both tags are `omitempty`.** A vault that names no default round-trips
  unchanged, and the key appears only on the vault it was set for. Without the
  omission, re-serialization would inject an empty key into every vault.

## What a write does to the file

A successful write re-serializes the whole file from the config struct. Other
keys **the config struct models** survive; comments and any key the struct does
not model do not, and keys are reordered to struct order. This is the existing
`config set-baseline` behaviour, accepted here rather than re-engineered.

The write is atomic: a temp file in the config's own directory is renamed over
it, so a reader observes either the old content or the new content and never a
truncated merge. An interrupted write leaves no temp file behind.

## Related

- `docs/baseline-file.md` — the sibling config-key plus set-verb plus doc-page
  shape this one follows.
- The consumer side lives in `bborbe/claude-supervisor`'s
  `commands/open-a2a.md` and `docs/fleet-surface.md`.
