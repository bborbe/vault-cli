---
status: completed
execution_id: vault-cli-serialize-frontmatter-exec-266-export-serialize-frontmatter
dark-factory-version: v0.198.7
created: "2026-10-23T08:51:37Z"
queued: "2026-10-23T08:51:37Z"
started: "2026-10-23T08:51:41Z"
completed: "2026-10-23T09:12:27Z"
---

# Export the frontmatter serializer

<summary>
- A caller outside the storage package can render a vault page's bytes
- The rendering goes through the same serializer every existing write path already uses
- Nothing about the produced bytes changes: same YAML, same key order, same `---` fencing
- Existing write paths and their tests keep working untouched
- A caller that hands the bytes to a remote store, instead of this package's own writer, no longer has to re-implement YAML marshalling
- One serializer in the repo, so the two paths cannot drift
</summary>

<objective>
Expose the frontmatter serializer as `storage.SerializeFrontmatter` so a caller that holds the parsed frontmatter but writes the bytes elsewhere renders through the same code as every vault-cli write, instead of re-implementing YAML marshalling and drifting from it.
</objective>

<context>
Read CLAUDE.md for project conventions, and `docs/dod.md` for the definition of done this change is graded against.
Read `pkg/storage/base.go` — `serializeMapAsFrontmatter` on `*baseStorage` is the serializer. It reads no receiver state; it is a pure function of `data` and `originalContent` that currently happens to be spelled as a method.
Read `pkg/storage/base.go` `ParseFrontmatterMap` (the exported parse counterpart) — it is the precedent for a package-level exported frontmatter helper, including its GoDoc shape; place the new function next to `serializeMapAsFrontmatter`.
Read `docs/development-patterns.md` § Storage — it lists the storage primitives by name and must name the new exported entry point.
Read `pkg/storage/base_test.go` — the `Describe("serializeMapAsFrontmatter")` block covers the current behaviour: simple map, body preservation, round-trip.
Read `pkg/storage/export_test.go` — how the package exposes internals to its external test package today.
Read `pkg/storage/task.go` — `WriteTask` is the exemplar call site: it serializes the map, then writes the bytes itself.
</context>

<requirements>
1. In `pkg/storage/base.go`, add an exported package-level function next to `serializeMapAsFrontmatter`:

   `func SerializeFrontmatter(ctx context.Context, data map[string]any, originalContent string) (string, error)`

   It carries the body currently in `(*baseStorage).serializeMapAsFrontmatter`. Extend the GoDoc to say why it is exported: a caller that writes the bytes through a remote store, rather than through this package's own writer, must render through the same serializer so the two paths cannot drift. Follow the GoDoc shape of `ParseFrontmatterMap`.

   Also update the Storage section of `docs/development-patterns.md`: the `serializeMapAsFrontmatter` bullet must name `SerializeFrontmatter` as the exported entry point a caller outside the package renders through, and say that a caller writing the bytes elsewhere must use it rather than marshalling YAML itself.
2. Reduce `(*baseStorage).serializeMapAsFrontmatter` to a one-line delegation to `SerializeFrontmatter`. The existing write paths (`task.go`, `goal.go`, `theme.go`, `topic.go`, `objective.go`, `vision.go`, `decision.go`) and their tests must be unchanged.
3. Keep the behaviour byte-identical: the `---` opening and closing fences, the YAML library's key order, the preserved markdown body, and the `errors.Wrap(ctx, err, "marshal yaml frontmatter")` wrapping.
4. Add tests in `pkg/storage/base_test.go` that call `storage.SerializeFrontmatter` directly — not through `export_test.go`. Cover: a simple map produces the fenced block; an original with a body preserves the body; a round-trip re-parses to the same map via the exported `storage.ParseFrontmatterMap`. This is the boundary test: the serializer's contract is that its output re-parses through the public parse function, so a future change to either side breaks it. Leave the existing `Describe("serializeMapAsFrontmatter")` block as it is.
5. Add a `## Unreleased` bullet to `CHANGELOG.md` describing the new exported function. That section does not exist today — create it below the preamble (`All notable changes…` plus the `* MAJOR / MINOR / PATCH` lines) and above the newest `## vX.Y.Z` section. `scripts/check-changelog.sh`, run by `make precommit`, fails if a `## ` section appears above the preamble.
6. Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- No behaviour change: for a given map and original content, the produced bytes must be identical before and after this change
- Do not add a second serializer, and do not change YAML key ordering or the fence format
- Do not change any other storage method
- Leave `pkg/storage/export_test.go` unchanged — `SerializeMapAsFrontmatterForTest` still backs the existing serializer test block
</constraints>

<verification>
make precommit
</verification>
