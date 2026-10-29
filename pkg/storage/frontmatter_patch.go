// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"sort"
	"strings"

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"
)

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
func PatchFrontmatter(
	ctx context.Context,
	updates map[string]any,
	originalContent string,
) (string, error) {
	matches := frontmatterRegex.FindStringSubmatch(originalContent)
	if len(matches) < 3 {
		return "", errors.Errorf(ctx, "no frontmatter found")
	}
	block := matches[1]
	body := matches[2]
	lines := strings.Split(block, "\n")

	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		rendered, err := renderSingleKey(ctx, key, updates[key])
		if err != nil {
			return "", err
		}
		start, end, found := findTopLevelKey(lines, key)
		if !found {
			lines = append(lines, rendered...)
			continue
		}
		patched := make([]string, 0, len(lines)-(end-start)+len(rendered))
		patched = append(patched, lines[:start]...)
		patched = append(patched, rendered...)
		patched = append(patched, lines[end:]...)
		lines = patched
	}

	return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body, nil
}

// findTopLevelKey returns the half-open line range [start, end) occupied by the
// top-level key key in lines. start is the first line beginning with
// `key + ":"` at column zero; end extends over every following line that begins
// with a space or a tab. A blank line stops the range unless the next non-blank
// line is indented, so a blank line inside a nested value stays part of that
// value while a separator before a sibling key ends the range. The next
// unindented line always stops it. found is false when the block has no such
// key.
func findTopLevelKey(lines []string, key string) (int, int, bool) {
	prefix := key + ":"
	for start, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		end := start + 1
		for end < len(lines) && continuesValue(lines, end) {
			end++
		}
		return start, end, true
	}
	return 0, 0, false
}

// continuesValue reports whether lines[index] belongs to the value introduced
// by a top-level key. An indented line does. A blank line does only when the
// next non-blank line is indented, so a blank line inside a nested value stays
// part of it while a separator before a sibling key ends the value. Any other
// line ends it.
func continuesValue(lines []string, index int) bool {
	line := lines[index]
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return true
	}
	if strings.TrimSpace(line) != "" {
		return false
	}
	for i := index + 1; i < len(lines); i++ {
		next := lines[i]
		if strings.TrimSpace(next) != "" {
			return strings.HasPrefix(next, " ") || strings.HasPrefix(next, "\t")
		}
	}
	return false
}

// renderSingleKey marshals one key/value pair on its own and returns the
// resulting lines, without the single trailing newline yaml.Marshal emits.
//
// yaml.v3 PANICS rather than returning an error for a value it has no encoder
// for (a func, a channel, a complex number), so an unsupported value is
// converted back into the error this function documents instead of taking the
// caller's process down with it.
func renderSingleKey(ctx context.Context, key string, value any) (lines []string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			lines = nil
			err = errors.Errorf(ctx, "marshal frontmatter key %q: %v", key, recovered)
		}
	}()

	yamlBytes, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "marshal frontmatter key %q", key)
	}
	return strings.Split(strings.TrimSuffix(string(yamlBytes), "\n"), "\n"), nil
}
