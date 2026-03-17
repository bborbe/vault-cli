// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

type themeStorage struct {
	*baseStorage
}

// ReadTheme reads a theme from a markdown file.
func (t *themeStorage) ReadTheme(
	ctx context.Context,
	vaultPath string,
	themeID domain.ThemeID,
) (*domain.Theme, error) {
	filePath := filepath.Join(vaultPath, t.config.ThemesDir, themeID.String()+".md")
	return t.readThemeFromPath(ctx, filePath, themeID.String())
}

func (t *themeStorage) readThemeFromPath(
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Theme, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	theme := &domain.Theme{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := t.parseFrontmatter(ctx, content, theme); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return theme, nil
}

// FindThemeByName searches for a theme by name in the vault.
func (t *themeStorage) FindThemeByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Theme, error) {
	themesDir := filepath.Join(vaultPath, t.config.ThemesDir)
	matchedPath, matchedName, err := t.findFileByName(ctx, themesDir, name)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "find theme file")
	}
	return t.readThemeFromPath(ctx, matchedPath, matchedName)
}

// WriteTheme writes a theme to a markdown file.
func (t *themeStorage) WriteTheme(ctx context.Context, theme *domain.Theme) error {
	content, err := t.serializeWithFrontmatter(ctx, theme, theme.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(theme.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", theme.FilePath))
	}

	return nil
}
