# v0.80.0 Baseline

## Baseline Source

commit: 1e13612ffa02a0daafda2a68701ff9d7ab497006

Tag: `v0.80.0`
Capture date: 2026-06-20
Captured from: host (not YOLO container — git history is masked in containers)

## Replay command

The baseline files were produced by checking out v0.80.0, building a binary, and
walking each scenario's documented commands against it:

```bash
# 1. Clone + checkout v0.80.0
git clone --quiet --branch v0.80.0 --depth 1 \
    git@github.com:bborbe/vault-cli.git /tmp/v080-check

# 2. Build with version ldflags so vault-cli --version reports v0.80.0
cd /tmp/v080-check && go build \
    -ldflags "-X github.com/bborbe/vault-cli/pkg/cli.version=v0.80.0" \
    -o /tmp/v080-cli .

# 3. Per scenario: cp example/ to a temp WORK dir, run the scenario's
#    documented commands, then copy the resulting task/decision file
#    to scenario-NNN/.

# Scenario 002 (Simple Task lifecycle)
WORK=$(mktemp -d) && cp -R example/. "$WORK/"
sed -i.bak "s|__VAULT_PATH__|$WORK/vault|g" "$WORK/config.yaml"
/tmp/v080-cli --config "$WORK/config.yaml" task work-on "Simple Task"
/tmp/v080-cli --config "$WORK/config.yaml" task defer "Simple Task" +1d
/tmp/v080-cli --config "$WORK/config.yaml" task complete "Simple Task"
cp "$WORK/vault/24 Tasks/Simple Task.md" scenario-002/Simple-Task.md
# task list/show JSON captured similarly

# Scenario 003: task complete on the recurring Weekly Review
# Scenario 004: decision ack on Review Architecture
```

## Purpose and limitations

These files are a **forensic reference** for v0.80.0's on-disk output shape.
They are NOT a byte-identical regression gate, and cannot be one — three fields
vary across any two runs of the same scenario, regardless of binary version:

- `task_identifier` — generated via `uuid.NewString()` each time `WriteTask`
  fills a missing UUID; never two equal runs
- `claude_session_id` — set per-process when `task work-on` fires
- `completed_date` / `last_completed_date` / `reviewed_date` — set via
  `time.Now()` at scenario execution time

What the baseline **can** verify (and what reviewers should diff against):

- **Field set per shape** — which keys appear in frontmatter; nothing silently
  dropped or added between versions
- **Date format conventions** — `defer_date` / `planned_date` / `due_date` /
  `start_date` / `target_date` stay as `YYYY-MM-DD` strings (date-only,
  midnight-UTC); `completed_date` / `last_completed_date` are RFC3339-shape with
  zone offset
- **Goal references** — `goals:` list preservation, no aliasing drift
- **YAML key ordering** — alphabetical per `yaml.v3` default; no manual ordering
  hacks leaking through
- **Tags + body separator** — `---` after frontmatter; no body content drift

## What changes in this PR's binary (deliberate, documented)

Date fields populated via `time.Now()` (`completed_date`,
`last_completed_date`) now emit at RFC3339Nano precision instead of RFC3339
because the migration replaced `formatDateOrDateTime` (which truncated to
second precision) with `DateOrDateTime.String()` (which preserves the
sub-second component). See CHANGELOG `## Unreleased` entry.

Date-only fields are unchanged.

All parsers (`vault-cli`'s `libtime.ParseTime`, Obsidian YAML, anything using
`bborbe/time`) accept both shapes — no functional break for any consumer.
