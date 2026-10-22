# Worker and Manager Launchers

A vault's config entry can name the launcher its **worker** sessions open on and
the launcher its **manager** sessions open on. The values are stored under the
keys `worker_launcher` and `manager_launcher`, beside the existing
`claude_script` entry:

```yaml
vaults:
  personal:
    name: personal
    path: ~/Documents/Obsidian/Personal
    claude_script: ~/bin/cc-private
    worker_launcher: ~/bin/cc-private-deepseek
    manager_launcher: ~/bin/cc-private-claude
```

There is no setter verb. Both keys are hand-edited, exactly like
`claude_script`.

## What this repo does and does not do

This repo stores the keys and emits them from `config list --output json`. It
does not consume them and does not change launcher resolution.

- **Consuming them** — deciding that a worker session starts on
  `worker_launcher` and a manager session on `manager_launcher` — belongs to the
  consumer (`claude-supervisor`'s spawn and resume paths). Wiring the keys into
  resolution is a follow-up in that consumer.
- **The existing task > goal > vault `claude_script` precedence is unchanged.**
  Nothing in this repo reads the two new keys when choosing a launcher.
- **Setting the per-vault values** is an operator act performed on the host
  against the real config file, not something this repo seeds.

## Derivation from `claude_script`

An empty key means "derive from `claude_script`". The derivation splits the
script into directory and base, strips the opposite role suffix, appends the
role's own suffix, and re-joins the original directory:

| `claude_script`      | `worker_launcher` derived | `manager_launcher` derived |
| -------------------- | ------------------------- | -------------------------- |
| `cc-private`         | `cc-private-deepseek`     | `cc-private-claude`        |
| `cc-private-claude`  | `cc-private-deepseek`     | `cc-private-claude`        |
| `cc-private-deepseek`| `cc-private-deepseek`     | `cc-private-claude`        |

A base already carrying the role's suffix is returned unchanged.

### The existence check

The derived candidate is returned **only when that file exists on disk**. This
is deliberate: a script outside the `cc-*` convention must never produce a
fabricated path. `claude.sh` would otherwise derive `claude.sh-deepseek` and
`cc-assistant-boss` would derive `cc-assistant-boss-deepseek` — neither names an
executable, and a consumer would exec the literal string. When the candidate
does not exist, the accessor returns an empty string and the consumer keeps its
own fallback.

The accessors therefore touch the filesystem. That is the one thing they do
beyond reading the config struct.

### Explicit values

An explicitly set key wins over the derivation. A value containing a `/` is
returned as given. A bare name (no `/`) is joined beside `claude_script`'s
directory — the same rule `resolveLauncherValue` applies to a task's
`launcher:` — so a config carrying `manager_launcher: cc-private-claude` emits
an absolute path a consumer can exec.

An empty `claude_script` with neither key set yields an empty value for both;
the caller keeps its own fallback.

## Launcher shape

The emitted value must satisfy the launcher shape this repo already enforces for
task and goal launchers: `^[A-Za-z0-9._/-]+$` with no `..` segment
(`pkg/ops/launcher.go`). A `claude_script` path containing a space therefore
derives a value the consumer refuses. This is stated here rather than left for
the consumer to discover at spawn time.

## Where it lives

- The on-disk keys are `worker_launcher` and `manager_launcher` in the
  vault-cli config file (`~/.config/vault-cli/config.yaml` by default).
- The JSON spellings are the same strings, and they are what a consumer reads
  from `config list --output json`. Both spellings are part of the contract: the
  JSON one is the delivery surface, the YAML one is the on-disk surface.
- **All tags are `omitempty`.** A vault that sets neither key emits neither, so
  a vault with no launcher configuration still lists as only `path` and `name`.

## Related

- `docs/a2a-default-cluster.md` — the sibling config-key-plus-doc-page shape
  this one follows.
- The consumer side lives in `bborbe/claude-supervisor`'s spawn and resume
  paths.
