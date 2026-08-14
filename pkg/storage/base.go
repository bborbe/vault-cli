// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)

	// CheckboxRegex matches a Markdown checkbox line with either `-` or `*` as
	// the list marker. Capture groups: 1=leading whitespace, 2=state (` `, `x`,
	// or `/`), 3=task text. Shared across storage and ops packages to keep the
	// parser shape in one place.
	CheckboxRegex = regexp.MustCompile(`^(\s*)[-*] \[([ x/])\] (.+)$`)

	// CheckboxCompleteRegex matches an unchecked or in-progress checkbox marker
	// (` ` or `/`) and is used by rewriters that force a line to checked.
	// Capture groups: 1=list marker (`-` or `*`), 2=state (` ` or `/`).
	CheckboxCompleteRegex = regexp.MustCompile(`([-*]) \[([ /])\]`)

	// CheckboxUncompleteRegex matches a checked checkbox marker and is used by
	// rewriters that force a line to unchecked. Capture group 1=list marker.
	CheckboxUncompleteRegex = regexp.MustCompile(`([-*]) \[x\]`)

	// bareWikilinkMappingRegex matches a frontmatter mapping line whose value is
	// exactly a bare Obsidian wikilink, at any indentation: `related_task: [[X]]`.
	// Capture groups: 1=indentation plus the `key: ` prefix, 2=the wikilink value.
	// A trailing YAML comment defeats the match by design — see quoteBareWikilinks.
	bareWikilinkMappingRegex = regexp.MustCompile(
		`^( *[A-Za-z0-9_][A-Za-z0-9_.-]*:[ \t]+)(\[\[.+\]\])[ \t]*$`,
	)

	// bareWikilinkSequenceRegex matches a block-sequence entry whose value is
	// exactly a bare Obsidian wikilink, at any indentation: `    - [[A Theme]]`.
	// Capture groups: 1=indentation plus the `- ` marker, 2=the wikilink value.
	bareWikilinkSequenceRegex = regexp.MustCompile(
		`^( *-[ \t]+)(\[\[.+\]\])[ \t]*$`,
	)

	// blockScalarStartRegex matches a mapping or sequence line that opens a block
	// scalar (`|` or `>`, with optional chomping and indentation indicators).
	// Capture group 1 is the introducing line's indentation, used to find where the
	// scalar's body ends. Lines inside a block scalar are literal text, never YAML,
	// so the quoting pass must leave them alone.
	blockScalarStartRegex = regexp.MustCompile(
		`^( *)(?:[A-Za-z0-9_][A-Za-z0-9_.-]*:|-)[ \t]+[|>][+-]?[0-9]*[ \t]*$`,
	)
)

type baseStorage struct {
	config *Config
}

// parseToFrontmatterMap parses the YAML frontmatter block from content into a
// map[string]any, preserving all fields including unknown ones.
// Returns an error if no frontmatter block is found or YAML is invalid.
// A bare Obsidian wikilink value is quoted before unmarshal — see
// quoteBareWikilinks — so it is read as a string rather than a nested list.
func (b *baseStorage) parseToFrontmatterMap(
	ctx context.Context,
	content []byte,
) (map[string]any, error) {
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return nil, errors.Errorf(ctx, "no frontmatter found")
	}

	var m map[string]any
	if err := yaml.Unmarshal(quoteBareWikilinks(ctx, matches[1]), &m); err != nil {
		return nil, errors.Wrap(ctx, err, "unmarshal yaml frontmatter")
	}
	if m == nil {
		m = make(map[string]any)
	}
	return m, nil
}

// quoteBareWikilinks rewrites frontmatter lines whose value is exactly a bare
// Obsidian wikilink into a single-quoted YAML scalar, and returns the rewritten
// frontmatter block.
//
// A bare wikilink is well-formed YAML flow-sequence syntax: `k: [[X]]` unmarshals
// to []any{[]any{"X"}} and marshals back as the nested block sequence `k:\n - - X`,
// silently destroying the link and its backlink. Neither yaml.Unmarshal nor
// yaml.Marshal is wrong on its own terms — the corruption emerges only from the
// round-trip, which is why nothing detects it. Quoting the value before
// yaml.Unmarshal ever sees it keeps the in-memory value a plain string, so the
// existing serializer writes it back as a working wikilink with no change to
// serializeMapAsFrontmatter.
//
// Only quoting changes; YAML shape never does. A bare wikilink under a
// conventionally-list key (`themes: [[A Theme]]`) becomes the quoted *scalar*
// `themes: '[[A Theme]]'`, not a one-element sequence — the authored form is
// preserved as authored.
//
// Left byte-identical: an already-quoted value, a value that merely contains a
// wikilink among other text, a value carrying a trailing YAML comment, a value
// spanning multiple lines, the body of a block scalar, and an already-destroyed
// nested list. The pass is a pure text transform and cannot introduce a parse
// error it did not receive; it is also idempotent, because a quoted value no
// longer starts with `[[`.
//
// The line loop honours ctx cancellation: on cancel it returns the frontmatter
// unmodified, so a cancelled parse degrades to pre-fix behaviour rather than
// emitting a half-rewritten block.
func quoteBareWikilinks(ctx context.Context, frontmatter []byte) []byte {
	if !bytes.Contains(frontmatter, []byte("[[")) {
		return frontmatter
	}
	lines := strings.Split(string(frontmatter), "\n")
	changed := false
	blockScalarIndent := -1
	for i, line := range lines {
		select {
		case <-ctx.Done():
			return frontmatter
		default:
		}
		if blockScalarIndent >= 0 && isBlockScalarBody(line, blockScalarIndent) {
			continue
		}
		blockScalarIndent = -1
		if m := blockScalarStartRegex.FindStringSubmatch(line); len(m) == 2 {
			blockScalarIndent = len(m[1])
			continue
		}
		rewritten, ok := quoteBareWikilinkLine(line)
		if !ok {
			continue
		}
		lines[i] = rewritten
		changed = true
	}
	if !changed {
		return frontmatter
	}
	return []byte(strings.Join(lines, "\n"))
}

// isBlockScalarBody reports whether line belongs to the body of a block scalar
// whose introducing key is indented by keyIndent spaces. A blank line and any
// line indented deeper than the key are part of the body; the first line at or
// left of the key's indentation ends it.
func isBlockScalarBody(line string, keyIndent int) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	return len(line)-len(strings.TrimLeft(line, " ")) > keyIndent
}

// quoteBareWikilinkLine returns the single-quoted rewrite of line and true when
// line's value is exactly one bare wikilink; otherwise it returns line unchanged
// and false.
//
// Single quotes inside the title are escaped by doubling, so `[[Ben's Task]]`
// emits '[[Ben”s Task]]' and re-parses to the original title. A line holding
// more than one wikilink (`k: [[A]] and [[B]]`) is rejected: its value is not
// exactly a wikilink, and it is not valid YAML today either, so the pass leaves
// the pre-existing parse error intact rather than silently making it parse.
func quoteBareWikilinkLine(line string) (string, bool) {
	matches := bareWikilinkMappingRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		matches = bareWikilinkSequenceRegex.FindStringSubmatch(line)
	}
	if len(matches) != 3 {
		return line, false
	}
	value := matches[2]
	if strings.Contains(value[2:len(value)-2], "]]") {
		return line, false
	}
	return matches[1] + "'" + strings.ReplaceAll(value, "'", "''") + "'", true
}

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

func (b *baseStorage) findFileByName(
	ctx context.Context,
	dir string,
	name string,
) (string, string, error) {
	name = strings.TrimPrefix(name, "[[")
	name = strings.TrimSuffix(name, "]]")

	exactPath := filepath.Join(dir, name+".md")
	if _, err := os.Stat(exactPath); err == nil {
		return exactPath, name, nil
	}

	// A vault may be configured with a directory that was never created (e.g. a
	// task-only vault with no goals_dir). Report that as not-found so
	// ops.VaultDispatcher.FirstSuccess moves on to the next vault instead of
	// aborting the whole cross-vault search on an unrelated vault's error.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", "", errors.Wrapf(ctx, ErrNotFound, "%s", name)
	}

	nameLower := strings.ToLower(name)
	var matchedPath, matchedName string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return errors.Wrapf(ctx, err, "walk directory")
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		fileName := strings.TrimSuffix(d.Name(), ".md")
		if fileName == name {
			matchedPath = path
			matchedName = fileName
			return filepath.SkipAll
		}
		if strings.Contains(strings.ToLower(fileName), nameLower) {
			matchedPath = path
			matchedName = fileName
		}
		return nil
	})
	if err != nil {
		return "", "", errors.Wrap(ctx, err, fmt.Sprintf("walk directory %s", dir))
	}

	if matchedPath != "" {
		return matchedPath, matchedName, nil
	}

	return "", "", errors.Wrapf(ctx, ErrNotFound, "%s", name)
}

func (b *baseStorage) parseCheckboxes(content string) []domain.CheckboxItem {
	var items []domain.CheckboxItem
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if matches := CheckboxRegex.FindStringSubmatch(line); len(matches) == 4 {
			state := matches[2]
			items = append(items, domain.CheckboxItem{
				Line:       i,
				Checked:    state == "x",
				InProgress: state == "/",
				Text:       matches[3],
				RawLine:    line,
			})
		}
	}

	return items
}

// readEntityComponentsFromPath reads a vault file and returns its parsed frontmatter,
// file metadata, and raw content. Both readTaskFromPath and readPageFromPath delegate
// to this helper to avoid duplicating the file-reading and parsing logic.
func (b *baseStorage) readEntityComponentsFromPath(
	ctx context.Context,
	filePath string,
	name string,
	vaultPath string,
) (map[string]any, domain.FileMetadata, domain.Content, error) {
	if isSymlinkOutsideVault(filePath, vaultPath) {
		return nil, domain.FileMetadata{}, "", errors.Errorf(
			ctx,
			"symlink outside vault: %s",
			filePath,
		)
	}
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, domain.FileMetadata{}, "", errors.Wrap(
			ctx,
			err,
			fmt.Sprintf("read file %s", filePath),
		)
	}

	var modTime *time.Time
	if info, err := os.Stat(filePath); err == nil {
		t := info.ModTime().UTC()
		modTime = &t
	}

	data, parseErr := b.parseToFrontmatterMap(ctx, content)
	if parseErr != nil {
		return nil, domain.FileMetadata{}, "", errors.Wrap(ctx, parseErr, "parse frontmatter")
	}

	meta := domain.FileMetadata{
		Name:         name,
		FilePath:     filePath,
		ModifiedDate: modTime,
	}

	return data, meta, domain.Content(content), nil
}

func (b *baseStorage) readTaskFromPath(
	ctx context.Context,
	filePath string,
	name string,
	vaultPath string,
) (*domain.Task, error) {
	data, meta, content, err := b.readEntityComponentsFromPath(ctx, filePath, name, vaultPath)
	if err != nil {
		return nil, err
	}
	return domain.NewTask(data, meta, content), nil
}

// isExcluded returns true when the given path falls under an excluded directory prefix.
func (b *baseStorage) isExcluded(vaultPath, path string) bool {
	rel, err := filepath.Rel(vaultPath, path)
	if err != nil {
		return false
	}
	relSlash := filepath.ToSlash(rel)
	for _, exclude := range b.config.Excludes {
		if strings.HasPrefix(relSlash, exclude) {
			return true
		}
	}
	return false
}

// isSymlinkOutsideVault returns true when path is a symlink resolving outside vaultPath.
// Returns false for non-symlink files.
func isSymlinkOutsideVault(path, vaultPath string) bool {
	// First check if path is actually a symlink
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}

	// Resolve vault path (handles if vault path itself is a symlink)
	resolvedVault, err := filepath.EvalSymlinks(vaultPath)
	if err != nil {
		return false
	}
	absVault, err := filepath.Abs(resolvedVault)
	if err != nil {
		return false
	}

	// Evaluate the symlink target
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return true // Broken symlink - treat as unsafe
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return true
	}
	return !strings.HasPrefix(absResolved, absVault)
}

// isSymlink returns true if the path is a symlink.
func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
