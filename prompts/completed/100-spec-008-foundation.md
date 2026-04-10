---
status: completed
spec: [008-flexible-frontmatter-refactor]
summary: Created FrontmatterMap, FileMetadata, and Content domain types plus parseToFrontmatterMap and serializeMapAsFrontmatter methods on baseStorage, with full Ginkgo test coverage for all new code.
container: vault-cli-100-spec-008-foundation
dark-factory-version: v0.108.0-dirty
created: "2026-04-10T00:00:00Z"
queued: "2026-04-10T21:45:54Z"
started: "2026-04-10T21:45:55Z"
completed: "2026-04-10T21:51:34Z"
---

<summary>
- A new shared `FrontmatterMap` type provides a typed map wrapper for raw YAML fields — used as the backing store for all entity frontmatter after migration
- Unknown YAML fields survive round-trips via the map rather than a rigid struct
- `FrontmatterMap` exposes `Get`, `GetString`, `GetStringSlice`, `Set`, `Delete`, `Keys`, and `RawMap` methods
- A shared `FileMetadata` type holds `Name`, `FilePath`, and `ModifiedDate` — the filesystem metadata common to every entity
- A new `Content` named string type wraps entity markdown body content for type safety
- Two new methods on `baseStorage` provide map-based parse and serialize without touching existing struct-based methods
- Existing entity types, storage implementations, ops, and CLI are untouched — all existing tests continue to pass
- The new code is purely additive: foundation for the entity migrations in subsequent prompts
</summary>

<objective>
Create the shared domain types (`FrontmatterMap`, `FileMetadata`, `Content`) and new base-storage helpers (`parseToFrontmatterMap`, `serializeMapAsFrontmatter`) that the entity migrations (Prompts 2 and 3) depend on. All new code is additive — nothing is removed or changed in existing files.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.
Read the relevant coding guides surfaced by the `coding` plugin: `go-error-wrapping-guide.md`, `go-testing-guide.md`, `go-composition.md`.

Key files to read before making changes:
- `pkg/domain/task.go` — shows existing entity struct pattern to understand what the new types replace
- `pkg/storage/base.go` — `parseFrontmatter` and `serializeWithFrontmatter` (the existing struct-based methods alongside which the new map-based methods will live)
- `pkg/storage/storage.go` — storage interface definitions
- `go.mod` — confirm `gopkg.in/yaml.v3` is the YAML library
</context>

<requirements>
### 1. Create `pkg/domain/frontmatter_map.go`

This file defines the `FrontmatterMap` type — a typed wrapper around `map[string]any` that serves as the primary store for YAML frontmatter fields in the new entity design.

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"fmt"
	"strings"
)

// FrontmatterMap is a typed wrapper around map[string]any that stores YAML frontmatter
// fields. It preserves all fields, including unknown ones, through read-write cycles.
// Entity-specific types embed FrontmatterMap and layer typed accessors on top.
type FrontmatterMap struct {
	data map[string]any
}

// NewFrontmatterMap constructs a FrontmatterMap from a raw map.
// If data is nil, an empty map is used.
func NewFrontmatterMap(data map[string]any) FrontmatterMap {
	if data == nil {
		data = make(map[string]any)
	}
	return FrontmatterMap{data: data}
}

// Get returns the raw value stored for key, or nil if absent.
func (f FrontmatterMap) Get(key string) any {
	return f.data[key]
}

// GetString returns the string representation of the value stored for key.
// Returns "" if the key is absent or the value cannot be stringified.
func (f FrontmatterMap) GetString(key string) string {
	v := f.data[key]
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetStringSlice returns a []string for the value stored under key.
// Handles: nil (returns nil), []any (coerces each element to string),
// []string (returned directly), and string (splits on comma).
func (f FrontmatterMap) GetStringSlice(key string) []string {
	v := f.data[key]
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case string:
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	default:
		return nil
	}
}

// Set stores value under key. A nil value is equivalent to Delete.
func (f *FrontmatterMap) Set(key string, value any) {
	if f.data == nil {
		f.data = make(map[string]any)
	}
	if value == nil {
		delete(f.data, key)
		return
	}
	f.data[key] = value
}

// Delete removes key from the map. No-op if key is absent.
func (f *FrontmatterMap) Delete(key string) {
	delete(f.data, key)
}

// Keys returns all keys present in the map, in no guaranteed order.
func (f FrontmatterMap) Keys() []string {
	if len(f.data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(f.data))
	for k := range f.data {
		keys = append(keys, k)
	}
	return keys
}

// RawMap returns the underlying map. Callers must not mutate the returned map.
// This method is intended for serialization (yaml.Marshal).
func (f FrontmatterMap) RawMap() map[string]any {
	return f.data
}
```

### 2. Create `pkg/domain/file_metadata.go`

This file defines `FileMetadata` — the filesystem metadata common to all entity types.
The entity structs will embed this type alongside their per-entity frontmatter type.

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import "time"

// FileMetadata holds the filesystem metadata for an entity file.
// It is embedded in all entity structs (Task, Goal, Theme, Objective, Vision)
// and is never stored in YAML frontmatter.
type FileMetadata struct {
	// Name is the filename without the .md extension.
	Name string
	// FilePath is the absolute path to the markdown file.
	FilePath string
	// ModifiedDate is the file's last-modified time (UTC), populated by the storage layer.
	ModifiedDate *time.Time
}
```

### 3. Create `pkg/domain/content.go`

This file defines `Content` — a named string type that wraps the markdown body of an entity file. Using a named type instead of raw `string` provides compile-time type safety so markdown content cannot be confused with other strings (paths, names, etc.) at call sites.

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

// Content is the full markdown file content including the frontmatter block.
// It is embedded in entity structs (Task, Goal, Theme, Objective, Vision)
// alongside FileMetadata and an entity-specific XxxFrontmatter type.
// The storage layer extracts the markdown body from Content on write.
type Content string

// String returns the underlying string value.
func (c Content) String() string {
	return string(c)
}
```

### 4. Add map-based methods to `pkg/storage/base.go`

Add two new methods to `baseStorage` BELOW the existing `parseFrontmatter` and `serializeWithFrontmatter` methods. Do NOT modify or remove the existing methods — the entity migrations in subsequent prompts will switch callers one by one.

#### 4a. `parseToFrontmatterMap`

```go
// parseToFrontmatterMap parses the YAML frontmatter block from content into a
// map[string]any, preserving all fields including unknown ones.
// Returns an error if no frontmatter block is found or YAML is invalid.
func (b *baseStorage) parseToFrontmatterMap(
	ctx context.Context,
	content []byte,
) (map[string]any, error) {
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return nil, errors.Errorf(ctx, "no frontmatter found")
	}

	var m map[string]any
	if err := yaml.Unmarshal(matches[1], &m); err != nil {
		return nil, errors.Wrap(ctx, err, "unmarshal yaml frontmatter")
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}
```

#### 4b. `serializeMapAsFrontmatter`

```go
// serializeMapAsFrontmatter serializes data as YAML frontmatter, replacing the
// frontmatter block in originalContent and preserving the markdown body.
// Fields are written in YAML library key order (alphabetical); this may differ
// from the original file's key order, which is acceptable per the spec.
func (b *baseStorage) serializeMapAsFrontmatter(
	ctx context.Context,
	data map[string]any,
	originalContent string,
) (string, error) {
	matches := frontmatterRegex.FindStringSubmatch(originalContent)
	var body string
	if len(matches) >= 3 {
		body = matches[2]
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", errors.Wrap(ctx, err, "marshal yaml frontmatter")
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	buf.WriteString(body)
	return buf.String(), nil
}
```

Ensure `bytes` is already in the import list of `base.go` (it is, since `serializeWithFrontmatter` already uses it).

### 5. Write tests

#### 5a. `pkg/domain/frontmatter_map_test.go`

Create a Ginkgo test file `pkg/domain/frontmatter_map_test.go` in the `domain_test` external test package. The suite bootstrap `pkg/domain/domain_suite_test.go` already exists (with `TestSuite` / `"domain Test Suite"`); do NOT create or modify it.

Cover the following cases in `frontmatter_map_test.go`:

- `NewFrontmatterMap(nil)` — keys returns empty, get returns nil
- `NewFrontmatterMap(map[string]any{"status": "todo"})` — GetString("status") == "todo"
- `Set` then `Get` round-trips `string`, `int`, `[]string` values
- `GetString` on an int value returns its decimal string representation
- `GetStringSlice` on `[]any{"a", "b"}` returns `[]string{"a", "b"}`
- `GetStringSlice` on `nil` key returns nil
- `GetStringSlice` on comma-separated string `"a,b"` returns `["a", "b"]`
- `Delete` removes the key; subsequent `Get` returns nil
- `Set(key, nil)` is equivalent to `Delete`
- `Keys` returns all stored keys (use `ConsistOf` not `Equal` — order not guaranteed)
- `RawMap` returns the underlying map

#### 5b. `pkg/domain/file_metadata_test.go`

A minimal test confirming `FileMetadata` is zero-valued correctly (Name, FilePath, ModifiedDate all zero).

#### 5c. `pkg/domain/content_test.go`

A minimal test confirming `Content` round-trips through `String()` and `Content(s).String() == s` for string values.

#### 5d. Storage base test for new methods

**Package access problem**: the storage suite bootstrap (`pkg/storage/storage_suite_test.go`) is in package `storage_test` (external test package). The new methods `parseToFrontmatterMap` and `serializeMapAsFrontmatter` are unexported — they cannot be called from `storage_test` directly. Use the standard Go `export_test.go` pattern to expose them for testing only:

**Step 1**: Create `pkg/storage/export_test.go` in package `storage` (internal test file — `_test.go` suffix makes it test-only, and the lack of `_test` package suffix keeps it in the `storage` package):

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

// Test-only exports of unexported baseStorage methods.
// These variables are only visible to _test.go files in package storage_test.

var ParseToFrontmatterMapForTest = (*baseStorage).parseToFrontmatterMap
var SerializeMapAsFrontmatterForTest = (*baseStorage).serializeMapAsFrontmatter
```

**Step 2**: Create `pkg/storage/base_test.go` in package `storage_test` (external, same package as the suite bootstrap). Use the exported test variables:

```go
// pseudo-example — adapt to Ginkgo style used in the storage suite
var _ = Describe("baseStorage map methods", func() {
    var b *storage.BaseStorage // if you also export the constructor, or use package-internal
    ...
    // Call via: storage.ParseToFrontmatterMapForTest(b, ctx, content)
})
```

**Alternative (simpler)**: make both new methods test-callable via exported package-level wrapper functions in `export_test.go`:
```go
package storage

func ParseToFrontmatterMapForTest(b *baseStorage, ctx context.Context, content []byte) (map[string]any, error) {
    return b.parseToFrontmatterMap(ctx, content)
}

func SerializeMapAsFrontmatterForTest(b *baseStorage, ctx context.Context, data map[string]any, orig string) (string, error) {
    return b.serializeMapAsFrontmatter(ctx, data, orig)
}
```

The `baseStorage` type itself is also unexported — exposing it requires either an `ExportedBaseStorage` test alias in `export_test.go` or constructing the test subject via an exported package-level helper. Pick whichever approach produces the least test-only surface area.

The goal is to test the two new methods with the existing Ginkgo/Gomega infrastructure in `storage_test`. Do NOT modify the suite bootstrap.

Cover:

- `parseToFrontmatterMap` with valid frontmatter returns expected map entries
- `parseToFrontmatterMap` with an unknown field preserves it in the map
- `parseToFrontmatterMap` with no frontmatter block returns an error
- `serializeMapAsFrontmatter` produces a `---\n…\n---\n` wrapped YAML block
- `serializeMapAsFrontmatter` preserves the markdown body from `originalContent`
- Round-trip: parse then re-serialize produces a string that re-parses to the same map (field order may differ)

Use the standard Ginkgo/Gomega style matching the rest of the storage tests.
</requirements>

<constraints>
- Do NOT modify any existing entity types (`domain.Task`, `domain.Goal`, etc.)
- Do NOT modify `parseFrontmatter` or `serializeWithFrontmatter` in `base.go` — add new methods alongside them
- Do NOT change any storage interface signatures
- `FrontmatterMap.Set(key, nil)` must call `Delete` internally (not store nil in the map)
- `FrontmatterMap` must NOT implement `yaml.Marshaler` or `yaml.Unmarshaler` — serialization is handled by the storage layer via `RawMap()`
- One type per file convention: `FrontmatterMap` in `frontmatter_map.go`, `FileMetadata` in `file_metadata.go`, `Content` in `content.go`
- All existing tests must continue to pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
# Confirm FrontmatterMap type exists
grep -n 'type FrontmatterMap' pkg/domain/frontmatter_map.go
# expected: one line

# Confirm FileMetadata type exists
grep -n 'type FileMetadata' pkg/domain/file_metadata.go
# expected: one line

# Confirm Content type exists
grep -n 'type Content string' pkg/domain/content.go
# expected: one line

# Confirm new base storage methods exist
grep -n 'parseToFrontmatterMap\|serializeMapAsFrontmatter' pkg/storage/base.go
# expected: two function definitions

# Confirm no existing methods were removed
grep -n 'func.*parseFrontmatter\|func.*serializeWithFrontmatter' pkg/storage/base.go
# expected: both still present
```
</verification>
