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

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
	checkboxRegex    = regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)
)

type baseStorage struct {
	config *Config
}

func (b *baseStorage) parseFrontmatter(
	ctx context.Context,
	content []byte,
	target interface{},
) error {
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return errors.Errorf(ctx, "no frontmatter found")
	}

	frontmatter := matches[1]
	if err := yaml.Unmarshal(frontmatter, target); err != nil {
		return errors.Wrap(ctx, err, "unmarshal yaml")
	}

	return nil
}

func (b *baseStorage) serializeWithFrontmatter(
	ctx context.Context,
	frontmatter interface{},
	originalContent string,
) (string, error) {
	matches := frontmatterRegex.FindStringSubmatch(originalContent)
	var bodyContent string
	if len(matches) >= 3 {
		bodyContent = matches[2]
	}

	yamlBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", errors.Wrap(ctx, err, "marshal yaml")
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	buf.WriteString(bodyContent)

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

	nameLower := strings.ToLower(name)
	var matchedPath, matchedName string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
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

	return "", "", errors.Errorf(ctx, "file not found: %s", name)
}

func (b *baseStorage) parseCheckboxes(content string) []domain.CheckboxItem {
	var items []domain.CheckboxItem
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
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

func (b *baseStorage) readTaskFromPath(
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Task, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	task := &domain.Task{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := b.parseFrontmatter(ctx, content, task); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return task, nil
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
func isSymlinkOutsideVault(path, vaultPath string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	resolvedVault, err := filepath.EvalSymlinks(vaultPath)
	if err != nil {
		return false
	}
	absVault, err := filepath.Abs(resolvedVault)
	if err != nil {
		return false
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(absResolved, absVault)
}
