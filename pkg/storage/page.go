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

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

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
	for _, entry := range entries {
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
			// Log error but continue with other pages
			slog.Debug("skipping unreadable page", "file", fileName, "error", err)
			continue
		}

		pages = append(pages, page)
	}

	return pages, nil
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
