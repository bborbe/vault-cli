# vault-cli example

Minimal fixture vault for smoke-testing vault-cli without installing it globally or touching real data.

## Layout

```
example/
├── config.yaml          # Config template (path uses __VAULT_PATH__ placeholder)
└── vault/
    ├── 20 Vision/
    ├── 21 Themes/
    ├── 22 Objectives/
    ├── 23 Goals/
    ├── 24 Tasks/
    ├── 25 Decisions/
    └── 50 Knowledge Base/
```

Sample entries: `Example Vision`, `Example Theme`, `Example Objective`, `Example Goal`, `Simple Task`, `Weekly Review`, `Review Architecture` (decision).

## Setup

From the repo root:

```bash
# 1. Build a local binary (no `make install` — stays out of $GOPATH/bin)
go build -o /tmp/vault-cli .

# 2. Copy the example vault to a scratch dir (keep the fixture pristine)
cp -r example/vault /tmp/vault-example

# 3. Materialise a config with the real path
sed "s|__VAULT_PATH__|/tmp/vault-example|" example/config.yaml > /tmp/vault-example.yaml
```

All subsequent commands use `--config /tmp/vault-example.yaml` so nothing touches `~/.vault-cli/config.yaml` or real vaults.

## Scenarios

### 1. Read-only smoke test

```bash
/tmp/vault-cli --config /tmp/vault-example.yaml task list
/tmp/vault-cli --config /tmp/vault-example.yaml goal list
/tmp/vault-cli --config /tmp/vault-example.yaml task show "Simple Task"
/tmp/vault-cli --config /tmp/vault-example.yaml task get "Simple Task" priority
```

Expected: listings show `Simple Task` / `Example Goal`, `priority` returns `2`.

### 2. Flexible frontmatter (spec 008)

Set an arbitrary field, read it back, verify it persisted to disk:

```bash
/tmp/vault-cli --config /tmp/vault-example.yaml task set "Simple Task" custom_field "hello"
/tmp/vault-cli --config /tmp/vault-example.yaml task get "Simple Task" custom_field
grep custom_field "/tmp/vault-example/24 Tasks/Simple Task.md"
```

Expected: `hello` on both reads, and the field appears in the file's frontmatter.

### 3. Known-field validation

```bash
/tmp/vault-cli --config /tmp/vault-example.yaml task set "Simple Task" priority -- -1
/tmp/vault-cli --config /tmp/vault-example.yaml task set "Simple Task" status banana
```

Both must fail with validation errors (`priority must be >= 0`, `unknown task status 'banana'`).

### 4. Unknown-field round-trip

Inject a field directly into the file, then trigger a vault-cli write, and confirm the unknown field survives:

```bash
perl -i -pe 's/^(priority: 2)$/$1\nzany_key: preserve_me/' "/tmp/vault-example/24 Tasks/Simple Task.md"
/tmp/vault-cli --config /tmp/vault-example.yaml task set "Simple Task" status in_progress
grep zany_key "/tmp/vault-example/24 Tasks/Simple Task.md"
```

Expected: `zany_key: preserve_me` still present after the write.

## Cleanup

```bash
rm -rf /tmp/vault-cli /tmp/vault-example /tmp/vault-example.yaml
```

## Why not use the real vault?

- No risk of mutating real tasks/goals during iteration
- No dependency on `~/.vault-cli/config.yaml`
- Reproducible — everyone runs against the same fixture
- Fast — small enough to re-copy per run
