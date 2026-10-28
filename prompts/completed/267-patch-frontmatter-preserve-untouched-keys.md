---
status: completed
execution_id: vault-cli-frontmatter-exec-267-patch-frontmatter-preserve-untouched-keys
dark-factory-version: v0.198.7
created: "2026-10-24T22:15:00Z"
queued: "2026-10-24T20:15:31Z"
started: "2026-10-24T20:17:54Z"
completed: "2026-10-24T20:26:46Z"
branch: fix/frontmatter-byte-preserving
lastFailReason: 'validate completion report: completion report status: partial'
reconciled: "2026-10-28"
reconciledReason: 'Work shipped in 5cfdefb "feat: add storage.PatchFrontmatter for byte-preserving key updates" (2026-10-24); only the completion report failed validation. Verified on master: pkg/storage/frontmatter_patch.go and pkg/storage/frontmatter_patch_test.go are both present. Status reconciled to completed — the prompt was never re-run.'
---

# Add PatchFrontmatter so a one-key write leaves every other frontmatter byte untouched

<summary>
- A caller can rewrite one or more named top-level frontmatter keys and get the rest of the block back byte-for-byte.
- The markdown body is untouched.
- Key order, indentation and quoting of every key the caller did not name survive.
- A key the caller names is replaced in place, not moved.
- A key the caller names that is absent from the block is appended.
- A block whose value spans several indented lines is replaced as a whole, not merged into.
- A blank line between keys is never consumed.
- A file with no frontmatter block is refused with an error rather than silently rewritten.
- Every existing caller keeps its current behaviour; nothing that already works changes.
- A new test fails on today's code and passes after the change, so it pins the fix rather than describing it.
</summary>

<objective>
Add an exported function to `pkg/storage`, beside `SerializeFrontmatter` (signature in requirement 1), that rewrites only the frontmatter lines belonging to the keys it is given. Today the only way to render frontmatter back into a file is `SerializeFrontmatter`, which re-marshals the entire block from a map; every key the caller never touched is rewritten into `yaml.Marshal`'s key order, indentation and quoting. A consumer that changes one key therefore produces a diff across the whole block.
</objective>

<context>
Read `CLAUDE.md` for project conventions. Then read these files fully, in this order:

- `pkg/storage/base.go` — `frontmatterRegex` (the `(?s)^---\n(.*?)\n---\n(.*)$` pattern, capture 1 = block, capture 2 = body), `ParseFrontmatterMap`, and `SerializeFrontmatter` with its doc comment at `:211-219`. `PatchFrontmatter` sits beside `SerializeFrontmatter` and reuses `frontmatterRegex`; do not introduce a second pattern.
- `pkg/storage/base_test.go` — the existing `Describe` / `Context` / `It` shape and the `storage_test` external package convention.
- `pkg/storage/storage_suite_test.go` — the suite entry point; no change needed.
- `pkg/storage/export_test.go` — the existing `…ForTest` re-export pattern. `PatchFrontmatter` is exported, so it needs no re-export; do not add one.
- `docs/development-patterns.md` § Storage — its last bullet says an external caller "must render through `SerializeFrontmatter`"; requirement 3 extends it to name `PatchFrontmatter` for a partial update.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Wrapf` / `errors.Errorf` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external test packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and the required `fix:` prefix.
</context>

<requirements>
1. **Create `pkg/storage/frontmatter_patch.go`** with this exact three-line BSD header (matching `pkg/storage/base_test.go`), package `storage`, and exactly this exported function plus two unexported helpers:

   ```go
   // Copyright (c) 2026 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.
   ```


   ```go
   // PatchFrontmatter replaces the values of the named top-level keys inside the
   // frontmatter block of originalContent and returns the whole file, markdown body
   // included.
   //
   // Unlike SerializeFrontmatter, which re-marshals the entire block from a map and
   // therefore rewrites every key into yaml.Marshal's own key order, indentation and
   // quoting, PatchFrontmatter rewrites only the lines belonging to the keys it is
   // given. Every other byte of the block survives, so a caller that changes one key
   // produces a diff of one key.
   //
   // A key absent from the block is appended. A key present in updates is rewritten
   // even when its value is unchanged, so callers should pass only the keys they
   // actually changed.
   func PatchFrontmatter(ctx context.Context, updates map[string]any, originalContent string) (string, error)
   ```

   Behaviour, in this order:
   - Match `frontmatterRegex` against `originalContent`. Fewer than three captures → `errors.Errorf(ctx, "no frontmatter found")`. Capture 1 is the block, capture 2 the body.
   - Sort the `updates` keys so the output is deterministic regardless of map iteration order.
   - For each key: render it alone with `yaml.Marshal(map[string]any{key: value})`, trim the single trailing newline, and split on `"\n"`. A marshal failure is wrapped with `errors.Wrapf(ctx, err, "marshal frontmatter key %q", key)`.
   - Locate the key with a helper `findTopLevelKey(lines []string, key string) (start, end int, found bool)`: `start` is the first line whose text begins with `key + ":"` **at column zero**; `end` extends over every following line that begins with a space or a tab. A blank line stops the range **unless the next non-blank line is indented**, in which case the range continues over it — so a blank line inside a nested value does not leave orphaned lines behind, while a separator before a sibling key still stops the range. The next unindented line always stops it. Not found → append the rendered lines to the block.
   - Replace `lines[start:end]` with the rendered lines, leaving every other line in place.
   - Reassemble as `"---\n" + strings.Join(lines, "\n") + "\n---\n" + body`.

   The second helper is `renderSingleKey(ctx context.Context, key string, value any) ([]string, error)`: it marshals `map[string]any{key: value}`, trims the one trailing newline and splits on `"\n"`, wrapping a marshal failure as above. Keep both helpers unexported and in the same file.

2. **Add specs in a new file `pkg/storage/frontmatter_patch_test.go`**, package `storage_test`, BSD header, wrapped in `var _ = Describe("PatchFrontmatter", func() { ... })`. Use `ctx := context.Background()`. Cover:
   - **The preservation case, which is the point of the change.** A block deliberately formatted unlike `yaml.Marshal` output — a block sequence indented two spaces with double-quoted values, a key with extra spaces after the colon, and a `task_identifier` line — with `phase` patched from `planning` to `execution`. Assert the **whole returned string equals** the expected string, so `task_identifier`, the block sequence and the extra-spaced key are proven byte-identical, and only the `phase` line moved.
   - **A key absent from the block is appended** at the end of the block, and the body is unchanged.
   - **A multi-line value is replaced as a whole:** patching a block-sequence key replaces all of its indented lines rather than leaving orphaned entries behind.
   - **A blank line between keys survives** when a neighbouring key is patched.
   - **No frontmatter block → an error**, and the error is returned rather than a rewritten string.
   - **The body is never touched:** a body containing text that looks like frontmatter (`---` and `key: value` lines) comes back unchanged.
   - **The output still parses:** after patching one key, `storage.ParseFrontmatterMap(ctx, []byte(out))` returns the new value for that key and the original values for `task_identifier` and the block-sequence key — the output crosses into `vault-api`'s reader, so the bytes must survive a round trip through it.
   - **An unmarshalable value is refused:** patching a key to a value `yaml.Marshal` cannot encode (a func) returns an error and no rewritten string, so the only otherwise-untested branch is covered.

3. **Name the new function in `docs/development-patterns.md`.** In § Storage, the bullet ending "…must render through `SerializeFrontmatter` rather than marshalling YAML itself, so the two write paths cannot drift" now covers only a full-block rewrite. Extend it: a caller that changes one or more keys must render through `PatchFrontmatter`, which rewrites only those keys' lines; `SerializeFrontmatter` stays the whole-block renderer and `serializeMapAsFrontmatter` its method form. One clause naming which renderer a partial update uses is enough — do not restate the byte-identity rationale.

4. **Record the change in `CHANGELOG.md`.** Insert a `## Unreleased` section directly below the preamble (`All notable changes…` and the `* MAJOR / MINOR / PATCH` lines) and above the newest `## vX.Y.Z`, with one bullet:

   ```markdown
   - feat: add `storage.PatchFrontmatter`, which rewrites only the named top-level frontmatter keys and leaves every other byte of the block untouched, so a caller that changes one key no longer produces a diff across the whole block. `SerializeFrontmatter` is unchanged.
   ```

   Do **not** bump any version string: this repo's release automation owns the version fields. Add the unreleased bullet only.

5. **Self-check before finishing.** Re-read the diff against the summary bullets above, then run the full `<verification>` block.
</requirements>

<constraints>
- `SerializeFrontmatter` is not modified, not deprecated, and not routed through the new function. Every existing caller keeps its current behaviour.
- No new exported symbol beyond `PatchFrontmatter`. Both helpers stay unexported.
- Reuse `frontmatterRegex`; do not add a second frontmatter pattern.
- Do not reorder keys. A patched key stays where it was; an absent key is appended last.
- Do not add a YAML round-trip of the whole block as an intermediate step — the point is that untouched lines are never parsed and re-rendered.
- Do not touch `ParseFrontmatterMap`, `quoteBareWikilinks`, or the read path.
- No CLI verb and no flag: this is a library function only. `pkg/cli/` needs no change.
- Existing tests must still pass.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make test` iteratively while implementing, then `make precommit` once at the end. `make precommit` must exit 0.

Targeted run:

```
go test -mod=mod ./pkg/storage/... -ginkgo.focus="PatchFrontmatter" -ginkgo.v
```

`-ginkgo.focus` with zero matching specs exits 0 while running nothing. After the focused run, confirm the new spec names appear in the output; a log without them is a failure, not a pass.

Evidence greps (run each and confirm the count):

```
grep -c 'func PatchFrontmatter' pkg/storage/frontmatter_patch.go            # 1
grep -c 'func findTopLevelKey' pkg/storage/frontmatter_patch.go             # 1
grep -c 'func renderSingleKey' pkg/storage/frontmatter_patch.go             # 1
grep -c 'Describe("PatchFrontmatter"' pkg/storage/frontmatter_patch_test.go # 1
grep -c 'task_identifier' pkg/storage/frontmatter_patch_test.go             # >= 1
grep -c 'yaml.Marshal(data)' pkg/storage/base.go                           # 1 (SerializeFrontmatter body untouched)
sed -n '/^## Unreleased/,/^## v/p' CHANGELOG.md | grep -c 'PatchFrontmatter' # >= 1
```

The preservation spec must assert on the **whole file string**, not on a substring — a substring assertion passes while other keys churn, which is the defect this change removes.

Syntax and formatting:

```
gofmt -e -l pkg/storage/frontmatter_patch.go pkg/storage/frontmatter_patch_test.go
```

The command must print nothing.
</verification>
