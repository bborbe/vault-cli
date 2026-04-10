// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import "context"

// Test-only exports of unexported baseStorage methods.
// These functions are only visible to _test.go files in package storage_test.

// BaseStorageForTest is an exported alias to baseStorage for test-only use.
type BaseStorageForTest = baseStorage

// NewBaseStorageForTest creates a baseStorage instance for testing.
func NewBaseStorageForTest() *BaseStorageForTest {
	return &baseStorage{config: &Config{}}
}

// ParseToFrontmatterMapForTest exposes parseToFrontmatterMap for testing.
func ParseToFrontmatterMapForTest(
	ctx context.Context,
	b *BaseStorageForTest,
	content []byte,
) (map[string]any, error) {
	return b.parseToFrontmatterMap(ctx, content)
}

// SerializeMapAsFrontmatterForTest exposes serializeMapAsFrontmatter for testing.
func SerializeMapAsFrontmatterForTest(
	ctx context.Context,
	b *BaseStorageForTest,
	data map[string]any,
	orig string,
) (string, error) {
	return b.serializeMapAsFrontmatter(ctx, data, orig)
}
