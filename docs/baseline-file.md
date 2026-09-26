# Baseline File

The rollup baseline is a hand-authored vault markdown file that records a set of
interaction figures captured at a point in time. `vault-cli rollup weekly` reads
it and prints its figures verbatim next to the week's computed figures, so a
week's number reads as a movement from a recorded fixed point.

The file is **operator-authored and tool-read**. The tool never writes to it and
never recomputes its figures.

## Where it lives

- Inside the vault it belongs to. The path is **vault-relative**; an absolute
  path, or one whose cleaned form escapes the vault root, is rejected.
- Recorded on the vault's entry in the vault-cli config file under the key
  `baseline`, set by `vault-cli config set-baseline <vault> <path>`.
- One baseline per vault. A vault whose config entry carries no `baseline` key
  has no baseline, and the rollup prints exactly what it printed before the
  baseline feature existed.
- The file's name and directory inside the vault are the operator's choice.
  There is no default path — a vault-agnostic CLI cannot know one.

## Frontmatter contract (frozen)

These keys are the interface between the hand-authored file and the tool. They
are frozen: renaming one is a behaviour change, not a style choice.

| Key | Type | Required | Meaning |
|---|---|---|---|
| `baseline_captured` | date (`YYYY-MM-DD`) | yes | The date the figures were captured. Printed in the block header, never recomputed. |
| `baseline_human_total` | integer | yes | Total human interactions at capture. No per-week analogue; echoed verbatim, no delta row. |
| `baseline_median` | integer | yes | The capture's per-task median. Analogue of the computed `Per-family median`; the delta row carries the frozen marker `[definitional mismatch]`. |
| `baseline_weeks` | map `YYYY-Wnn` → integer | yes | Per-week human-interaction figures. The analogue of the computed `Human interactions` for a requested week that is a key of this map. |
| `baseline_agent_coverage` | string | yes | e.g. `1 of 420`. A coverage figure the rollup does not emit; echoed verbatim, no delta row. |

`baseline_agent_coverage` is a string on purpose: the stored text is echoed, never
parsed into numbers.

A file that is missing, unreadable, or missing any required key is an error: the
rollup exits non-zero naming the vault and the resolved path (or the missing
key) and prints no figures and no partial block. A misconfigured baseline is
never silently treated as "no baseline" — those are different states, and only
the first is silent.

## Example

```yaml
---
baseline_captured: 2026-09-12
baseline_human_total: 62485
baseline_median: 64
baseline_weeks:
  2026-W36: 25141
  2026-W37: 26476
baseline_agent_coverage: "1 of 420"
---
```

## Analogue mapping

| Baseline figure | Computed figure it is compared against | Delta |
|---|---|---|
| `baseline_weeks[<requested week>]` | `Human interactions` for the requested week | `<computed> - <baseline> = <delta>` |
| `baseline_median` | headline `Per-family median` | `<computed> - <baseline> = <delta>`, row marked `[definitional mismatch]` |
| `baseline_human_total` | none | no delta row |
| `baseline_agent_coverage` | none | no delta row |
| `baseline_weeks` entries for other weeks | none | no delta row |

When the requested week is absent from `baseline_weeks`, the `Human
interactions` delta row is omitted and nothing is printed in its place — never a
row reading `0`.

## Report layout (frozen)

The header strings and the block order below are the interface the acceptance
criteria grep; changing either is a behaviour change, not a style choice.

- The baseline block's header is the literal `Baseline (captured <YYYY-MM-DD>)`
  at column 0, where `<YYYY-MM-DD>` is the file's `baseline_captured`. It is the
  only line in the report that begins with `Baseline`.
- The delta block's header is the literal
  `Delta (vs baseline captured <YYYY-MM-DD>)` at column 0. It is the only line in
  the report that begins with `Delta`.
- With a baseline configured, the report's blocks appear in this fixed order:
  week header, then baseline block, then computed figures (with the per-family
  lines), then rule lines, then the delta block last. The delta block runs from
  its header line to the end of the output.
- A vault with no baseline configured prints neither block; its output is
  byte-identical to the pre-baseline output.

## Recorded definitional mismatches

The figures are a historical capture and disagree with a freshly computed week
by construction. The disagreements are surfaced, never reconciled.

| Mismatch | Why |
|---|---|
| The stored `W37: 26,476` vs a full-week `Human interactions` | The baseline was captured mid-W37, so the two cover different windows. Both values print; the delta is shown as a movement, not a defect. |
| The stored `median: 64` vs the headline `Per-family median` | `64` is a per-task median; the headline is a median of per-family medians. The delta row carries `[definitional mismatch]`. |
| `total: 62,485` and `agent: 1 of 420` have no analogue | `total` has no per-week counterpart and `agent coverage` is not a figure the rollup emits. Both are echoed verbatim with no delta row. |
