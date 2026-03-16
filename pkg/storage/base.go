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

func (b *baseStorage) parseFrontmatter(content []byte, target interface{}) error {
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return fmt.Errorf("no frontmatter found")
	}

	frontmatter := matches[1]
	if err := yaml.Unmarshal(frontmatter, target); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}

	return nil
}

func (b *baseStorage) serializeWithFrontmatter(
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
		return "", fmt.Errorf("marshal yaml: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	buf.WriteString(bodyContent)

	return buf.String(), nil
}

func (b *baseStorage) findFileByName(dir string, name string) (string, string, error) {
	exactPath := filepath.Join(dir, name+".md")
	if _, err := os.Stat(exactPath); err == nil {
		return exactPath, name, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileName := strings.TrimSuffix(entry.Name(), ".md")
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(name)) {
			filePath := filepath.Join(dir, entry.Name())
			return filePath, fileName, nil
		}
	}

	return "", "", fmt.Errorf("file not found: %s", name)
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

	if err := b.parseFrontmatter(content, task); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return task, nil
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
