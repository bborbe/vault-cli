// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

const (
	// maxCauseBytes bounds the cause string embedded in a skip warning so one
	// corrupt file cannot flood stderr. It is a hard bound: the logged value —
	// including the truncationSuffix when the cause was cut — is never longer
	// than this.
	maxCauseBytes = 200

	// truncationSuffix marks a cause that was cut at maxCauseBytes.
	truncationSuffix = "…"

	// maxUnreadablePageWarnings is the number of per-file skip warnings a single
	// ListPages walk emits before it stops naming files individually and prints
	// one summary line instead. It bounds a directory full of unreadable pages.
	maxUnreadablePageWarnings = 10
)

// truncateCause bounds cause to maxCauseBytes bytes. When the cause is longer
// it is cut on a UTF-8 rune boundary and truncationSuffix is appended, so the
// result is always valid UTF-8 and never longer than maxCauseBytes. The head
// of the message is kept: the leading "yaml: unmarshal errors:\n  line N: …
// already defined" survives for a duplicate-key file.
func truncateCause(cause string) string {
	if len(cause) <= maxCauseBytes {
		return cause
	}
	cut := maxCauseBytes - len(truncationSuffix)
	for cut > 0 && !utf8.RuneStart(cause[cut]) {
		cut--
	}
	return cause[:cut] + truncationSuffix
}

type pageStorage struct {
	*baseStorage
}

// ListPages returns all pages from a specific directory in the vault.
func (p *pageStorage) ListPages(
	ctx context.Context,
	vaultPath string,
	pagesDir string,
) ([]*domain.Page, error) {
	targetDir := filepath.Join(vaultPath, pagesDir)

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Debug("pages directory does not exist; returning empty list", "dir", targetDir)
			return nil, nil
		}
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read directory %s", targetDir))
	}

	pages := make([]*domain.Page, 0, len(entries))
	skipped := 0
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			// Return the pages read so far rather than nil, so a cancelled listing
			// still reports what it managed to read.
			return pages, errors.Wrap(ctx, ctx.Err(), "context cancelled")
		default:
		}

		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileName := strings.TrimSuffix(entry.Name(), ".md")
		filePath := filepath.Join(targetDir, entry.Name())

		page, err := p.readPageFromPath(ctx, filePath, fileName, vaultPath)
		if err != nil {
			// Warn and continue: the operator must be told the file was skipped,
			// otherwise the page silently disappears from listings that still exit 0.
			// The per-file line is capped at maxUnreadablePageWarnings so a directory
			// full of unreadable pages cannot flood stderr; the total is reported in
			// the summary line below.
			if skipped < maxUnreadablePageWarnings {
				slog.Warn(
					"skipping unreadable page",
					"file", filePath,
					"error", truncateCause(errors.Cause(err).Error()),
				)
			}
			skipped++
			continue
		}

		pages = append(pages, page)
	}

	if skipped >= maxUnreadablePageWarnings {
		slog.Warn(fmt.Sprintf("skipping %d unreadable pages", skipped))
	}

	return pages, nil
}

// isBarePageName reports whether name is usable as a page base name: non-empty,
// free of path separators (both `/` and `\`), and not a relative-path segment.
func isBarePageName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}

// ReadPage returns a single page from a specific directory in the vault.
// Unlike ListPages it reads only the named file, and it fails when that file
// is missing or unparseable rather than skipping it.
//
// name must be a bare page base name without the .md extension — the same
// value ListPages reports in Page.Name. A name containing a path separator
// is rejected, so a caller-supplied value cannot escape pagesDir.
func (p *pageStorage) ReadPage(
	ctx context.Context,
	vaultPath string,
	pagesDir string,
	name string,
) (*domain.Page, error) {
	if !isBarePageName(name) {
		return nil, errors.Errorf(
			ctx,
			"invalid page name %q: must be a bare base name without a path separator",
			name,
		)
	}
	filePath := filepath.Join(vaultPath, pagesDir, name+".md")
	return p.readPageFromPath(ctx, filePath, name, vaultPath)
}

// readPageFromPath reads a single page file and returns a *domain.Page.
// It delegates to the shared readEntityComponentsFromPath helper.
func (p *baseStorage) readPageFromPath(
	ctx context.Context,
	filePath string,
	name string,
	vaultPath string,
) (*domain.Page, error) {
	data, meta, content, err := p.readEntityComponentsFromPath(ctx, filePath, name, vaultPath)
	if err != nil {
		return nil, err
	}
	return domain.NewPage(data, meta, content), nil
}
