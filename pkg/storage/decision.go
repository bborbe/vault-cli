// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

type decisionStorage struct {
	*baseStorage
}

// readDecisionFromPath reads a decision file and returns a populated Decision.
func (d *decisionStorage) readDecisionFromPath(
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Decision, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	decision := &domain.Decision{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := d.parseFrontmatter(content, decision); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return decision, nil
}

// ListDecisions scans the entire vault recursively and returns all decisions with needs_review: true.
func (d *decisionStorage) ListDecisions(
	ctx context.Context,
	vaultPath string,
) ([]*domain.Decision, error) {
	decisions := make([]*domain.Decision, 0)

	err := filepath.WalkDir(vaultPath, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if isSymlinkOutsideVault(path, vaultPath) {
			fmt.Fprintf(os.Stderr, "Warning: skipping symlink outside vault %s\n", path)
			return nil
		}

		rel, relErr := filepath.Rel(vaultPath, path)
		if relErr != nil {
			fmt.Fprintf(
				os.Stderr,
				"Warning: failed to get relative path for %s: %v\n",
				path,
				relErr,
			)
			return nil
		}
		name := strings.TrimSuffix(rel, ".md")

		decision, decErr := d.readDecisionFromPath(
			ctx,
			path,
			name,
		) //#nosec G122 -- path validated against vault root above
		if decErr != nil {
			fmt.Fprintf(
				os.Stderr,
				"Warning: failed to parse decision frontmatter %s: %v\n",
				path,
				decErr,
			)
			return nil
		}

		if decision.NeedsReview {
			decisions = append(decisions, decision)
		}
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("walk vault %s", vaultPath))
	}

	return decisions, nil
}

// FindDecisionByName searches for a decision by exact or unambiguous partial name match.
func (d *decisionStorage) FindDecisionByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Decision, error) {
	// Path traversal guard
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return nil, fmt.Errorf("invalid decision name: %s", name)
		}
	}

	decisions, err := d.ListDecisions(ctx, vaultPath)
	if err != nil {
		return nil, err
	}

	normalizedName := filepath.ToSlash(name)

	// Exact match first
	for _, dec := range decisions {
		if filepath.ToSlash(dec.Name) == normalizedName {
			return dec, nil
		}
	}

	// Partial match
	var matches []*domain.Decision
	lowerName := strings.ToLower(name)
	for _, dec := range decisions {
		if strings.Contains(strings.ToLower(dec.Name), lowerName) {
			matches = append(matches, dec)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("decision not found: %s", name)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, dec := range matches {
			names[i] = dec.Name
		}
		return nil, fmt.Errorf(
			"ambiguous match: %q matches %d decisions: %s",
			name,
			len(matches),
			strings.Join(names, ", "),
		)
	}
}

// WriteDecision writes a decision to its markdown file, preserving the body content.
func (d *decisionStorage) WriteDecision(ctx context.Context, decision *domain.Decision) error {
	content, err := d.serializeWithFrontmatter(decision, decision.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(decision.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", decision.FilePath))
	}

	return nil
}
