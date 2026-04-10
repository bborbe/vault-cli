---
status: completed
spec: [008-flexible-frontmatter-refactor]
summary: Deleted reflection-based frontmatter helpers, removed dead storage methods, migrated decision.go to map-based pattern, and updated docs/development-patterns.md with Entity Structure section.
container: vault-cli-103-spec-008-cleanup
dark-factory-version: v0.108.0-dirty
created: "2026-04-10T00:00:00Z"
queued: "2026-04-10T21:46:01Z"
started: "2026-04-10T22:50:50Z"
completed: "2026-04-10T22:53:50Z"
---

<summary>
- `pkg/ops/frontmatter_reflect.go` is deleted — reflection-based field helpers are no longer called by any production code
- `pkg/storage/base.go` is cleaned: `parseFrontmatter` and `serializeWithFrontmatter` are removed if no longer called
- `docs/development-patterns.md` is updated to describe the map-based frontmatter pattern instead of the old struct-with-YAML-tags pattern
- All acceptance criteria from the spec are verified with concrete commands
- No hardcoded key-to-field mapping remains in any get/set/clear/show operation
- `make precommit` passes clean
</summary>

<objective>
Remove reflection-based frontmatter helpers that were made obsolete by Prompts 2 and 3, prune any dead storage base methods, and update `docs/development-patterns.md` to document the new map-based entity pattern. This prompt leaves the codebase clean, fully documented, and verifiably meeting all spec acceptance criteria.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.

**Prompts 1, 2, and 3 must be completed first.** This prompt depends on all five entity types (Task, Goal, Theme, Objective, Vision) having been migrated to the `FrontmatterMap`-backed pattern. If any prerequisite is incomplete, STOP and report — do not attempt to re-migrate callers here.

Key files to read in full before making changes:
- `specs/in-progress/008-flexible-frontmatter-refactor.md` — the parent spec with all acceptance criteria
- `pkg/ops/frontmatter_reflect.go` — the file to be deleted; confirm no callers remain first
- `pkg/storage/base.go` — check whether `parseFrontmatter` and `serializeWithFrontmatter` are still called anywhere
- `pkg/ops/frontmatter_entity.go` — confirm no remaining uses of reflection helpers
- `docs/development-patterns.md` — the documentation file to update (63 lines; line 27 is the target text)
- `pkg/domain/frontmatter_map.go` — understand the new pattern to document it accurately
- `pkg/domain/task_frontmatter.go` — example of the entity-specific frontmatter type pattern
</context>

<requirements>
### 1. Verify no remaining callers before deleting

Before deleting any file, search the entire codebase for remaining usages of the functions in `frontmatter_reflect.go`:

```bash
grep -rn 'fieldByYAMLTag\|getFieldAsString\|setFieldFromString\|clearField\|isListField\|appendToList\|removeFromList\|isReadOnlyTag\|formatDateOrDateTimeReflect' \
  --include='*.go' .
```

If any callers remain (excluding the definition file and test files for the reflect helpers themselves), fix them before proceeding.

Also check that `pkg/ops/frontmatter_entity.go` no longer imports `reflect`:
```bash
grep -n '"reflect"' pkg/ops/frontmatter_entity.go
# expected: no output — Prompt 3 should have removed this import
```

If the import is still present, the file won't compile after `frontmatter_reflect.go` is deleted (unused import error). Prompt 3 should have removed all `reflect.` call sites in `frontmatter_entity.go`; if this grep returns a match, STOP and return control to the user — do not attempt to re-migrate here.

Also check the base storage methods:
```bash
grep -rn 'parseFrontmatter\|serializeWithFrontmatter' --include='*.go' .
```

If `parseFrontmatter` or `serializeWithFrontmatter` are still called anywhere (in storage files or tests), those callers must be migrated to `parseToFrontmatterMap` / `serializeMapAsFrontmatter` first.

### 2. Delete `pkg/ops/frontmatter_reflect.go`

Once confirmed no callers remain, delete the file:

```bash
rm pkg/ops/frontmatter_reflect.go
```

Run `make test` immediately after to confirm no compilation errors.

### 3. Remove dead methods from `pkg/storage/base.go`

**Pre-check**: `readTaskFromPath` (originally at `pkg/storage/base.go:142`) was rewritten in Prompt 2 to use `parseToFrontmatterMap`. Confirm no call to `parseFrontmatter` remains in `readTaskFromPath`:
```bash
grep -n 'parseFrontmatter' pkg/storage/base.go
# expected: no output — readTaskFromPath should no longer call it
```

If `parseFrontmatter` is still called inside `readTaskFromPath` or any other helper, Prompt 2 is incomplete — STOP and return control to the user. Do not attempt to re-migrate storage helpers in this prompt.

Once confirmed clean: if `parseFrontmatter` and/or `serializeWithFrontmatter` in `pkg/storage/base.go` have no remaining callers (neither production code nor test code), remove them.

If they still have callers (e.g., test files that haven't been migrated), migrate those test files first, then remove the methods.

Do NOT remove `parseToFrontmatterMap` or `serializeMapAsFrontmatter` — those are the replacements.

### 4. Remove dead test helpers if any

If `pkg/ops/frontmatter_reflect_test.go` exists, delete it along with the implementation file:

```bash
test -f pkg/ops/frontmatter_reflect_test.go && rm pkg/ops/frontmatter_reflect_test.go
```

### 5. Update `docs/development-patterns.md`

The file is 63 lines long. Make two changes:

**Change A — replace line 27 verbatim.** The exact current text is:

```
1. **Domain** (`pkg/domain/`) — struct with YAML tags for frontmatter fields, metadata fields tagged `yaml:"-"`
```

Replace with:

```
1. **Domain** (`pkg/domain/`) — entity struct embeds `XxxFrontmatter` (a `FrontmatterMap`-backed typed wrapper), `FileMetadata`, and `Content string`. Typed getters/setters for known fields; `GetField`/`SetField`/`ClearField` for arbitrary keys. Unknown fields survive read-write cycles.
```

**Change B — add a new `## Entity Structure` section after the `## Adding a New Command` section** (insert between the current "Adding a New Command" section and the "Multi-Vault Pattern" section). The new section should explain:

1. The entity uses a per-type `XxxFrontmatter` struct that embeds `FrontmatterMap`
2. Known fields have typed getter/setter methods; unknown fields are stored in the map and accessible via `GetField`/`SetField`
3. The entity struct embeds `XxxFrontmatter` and `FileMetadata`, with `Content string` for the markdown body
4. Metadata fields (`Name`, `FilePath`, `ModifiedDate`) come from `FileMetadata` — they are never in YAML

Add a new subsection (or update the existing one) that explains the three-part entity design:

```markdown
## Entity Structure

Each entity (Task, Goal, Theme, Objective, Vision) cleanly separates three concerns:

**Frontmatter** (`pkg/domain/<entity>_frontmatter.go`)
- Embeds `FrontmatterMap` (a `map[string]any` wrapper)
- Typed getter methods for known fields (e.g., `Status() TaskStatus`, `Priority() Priority`)
- Typed setter methods that validate known fields (e.g., `SetStatus(TaskStatus) error`)
- Generic `GetField(key) string` / `SetField(ctx, key, value) error` / `ClearField(key)` for
  arbitrary keys — unknown fields pass through without validation
- All fields in the map (known and unknown) are preserved through read-write cycles

**Filesystem metadata** (`pkg/domain/file_metadata.go`)
- `FileMetadata` struct: `Name`, `FilePath`, `ModifiedDate`
- Embedded in every entity; never stored in YAML

**Markdown content**
- `Content string` field on the entity struct
- The full markdown file content including the frontmatter block
- Used by the storage layer to extract the body on write

**Storage** (`pkg/storage/`)
- `parseToFrontmatterMap` parses the YAML frontmatter block into `map[string]any`
- `serializeMapAsFrontmatter` marshals the map back to YAML; unknown fields are preserved
- Entity-specific read helpers call `NewXxx(data, meta, content)` constructors

**Operations** (`pkg/ops/`)
- Inject `XxxStorage` interfaces (never file I/O directly)
- Use entity accessor methods (`goal.Status()`, `task.SetField(ctx, key, val)`)
- No reflection; no hardcoded field switches
```

Also update the "Key Design Decisions" section to add:
- "**Map-based frontmatter** — all entity frontmatter is stored in `map[string]any`; unknown fields survive read-write cycles; known fields have typed accessors"

### 6. Verify all spec acceptance criteria

Run these commands and confirm each passes:

```bash
# AC: No hardcoded key-to-field switch remains in ops
grep -rn 'switch key' pkg/ops/ --include='*.go'
# Expected: no output

# AC: frontmatter_reflect.go is gone
test ! -f pkg/ops/frontmatter_reflect.go && echo "deleted OK"
# Expected: "deleted OK"

# AC: docs updated
grep -n 'FrontmatterMap\|map-based\|GetField' docs/development-patterns.md
# Expected: multiple matches

# AC: no remaining reflection imports in ops package
grep -n '"reflect"' pkg/ops/*.go
# Expected: no output (reflect was only used in frontmatter_reflect.go and frontmatter_entity.go)
```

Also manually verify the end-to-end acceptance criteria from the spec by running:

```bash
# Set up a temp vault with a task that has a custom frontmatter field
# (Use make test — the integration tests cover this scenario)
make test
```
</requirements>

<constraints>
- Do NOT delete any file until you have confirmed no remaining callers via grep
- `pkg/domain/frontmatter_map.go`, `pkg/domain/file_metadata.go` and all `*_frontmatter.go` files must NOT be deleted
- `docs/development-patterns.md` must describe the new map-based pattern accurately; do not leave contradictory descriptions of both old and new patterns in the same document
- All existing tests must continue to pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
# Final acceptance criteria sweep
grep -rn 'fieldByYAMLTag\|getFieldAsString\|setFieldFromString' --include='*.go' .
# expected: no output

grep -n 'FrontmatterMap\|map-based' docs/development-patterns.md
# expected: at least 2 matches

test ! -f pkg/ops/frontmatter_reflect.go && echo "deleted OK"
# expected: "deleted OK"

grep -nw 'parseFrontmatter' pkg/storage/base.go || echo "method removed"
# expected: either no output or "method removed"
```
</verification>
