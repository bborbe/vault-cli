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

	data, parseErr := d.parseToFrontmatterMap(ctx, content)
	if parseErr != nil {
		return nil, errors.Wrap(ctx, parseErr, "parse frontmatter")
	}

	decision := &domain.Decision{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}
	if v, ok := data["needs_review"].(bool); ok {
		decision.NeedsReview = v
	}
	if v, ok := data["reviewed"].(bool); ok {
		decision.Reviewed = v
	}
	if v, ok := data["reviewed_date"].(string); ok {
		decision.ReviewedDate = v
	}
	if v, ok := data["status"].(string); ok {
		decision.Status = v
	}
	if v, ok := data["type"].(string); ok {
		decision.Type = v
	}
	if v, ok := data["page_type"].(string); ok {
		decision.PageType = v
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
		if de.IsDir() {
			if d.isExcluded(vaultPath, path) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		if isSymlinkOutsideVault(path, vaultPath) {
			slog.Debug("skipping symlink outside vault", "path", path)
			return nil
		}

		rel, relErr := filepath.Rel(vaultPath, path)
		if relErr != nil {
			slog.Debug("skipping file, failed to get relative path", "path", path, "error", relErr)
			return nil
		}
		name := strings.TrimSuffix(rel, ".md")

		decision, decErr := d.readDecisionFromPath(
			ctx,
			path,
			name,
		) //#nosec G122 -- path validated against vault root above
		if decErr != nil {
			slog.Debug("skipping non-decision file", "path", path, "error", decErr)
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

// findByPathMatch searches decisions using path prefix/suffix matching (case-insensitive).
// Returns the single matching decision, or nil if zero or multiple match.
func findByPathMatch(decisions []*domain.Decision, normalizedName string) *domain.Decision {
	lowerNorm := strings.ToLower(normalizedName)
	var matches []*domain.Decision
	for _, dec := range decisions {
		lowerDec := strings.ToLower(filepath.ToSlash(dec.Name))
		if strings.HasSuffix(lowerDec, lowerNorm) || strings.HasPrefix(lowerDec, lowerNorm) {
			matches = append(matches, dec)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
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
			return nil, errors.Errorf(ctx, "invalid decision name: %s", name)
		}
	}

	decisions, err := d.ListDecisions(ctx, vaultPath)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "list decisions")
	}

	normalizedName := filepath.ToSlash(name)

	// Exact match first
	for _, dec := range decisions {
		if filepath.ToSlash(dec.Name) == normalizedName {
			return dec, nil
		}
	}

	// Path-suffix/prefix match: when identifier contains '/', try matching against the decision path.
	if strings.Contains(normalizedName, "/") {
		if dec := findByPathMatch(decisions, normalizedName); dec != nil {
			return dec, nil
		}
		// Zero or multiple path matches — fall through to substring match
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
		return nil, errors.Errorf(ctx, "decision not found: %s", name)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, dec := range matches {
			names[i] = dec.Name
		}
		return nil, errors.Errorf(
			ctx,
			"ambiguous match: %q matches %d decisions: %s",
			name,
			len(matches),
			strings.Join(names, ", "),
		)
	}
}

// WriteDecision writes a decision to its markdown file, preserving the body content.
func (d *decisionStorage) WriteDecision(ctx context.Context, decision *domain.Decision) error {
	data := map[string]any{
		"needs_review": decision.NeedsReview,
	}
	if decision.Reviewed {
		data["reviewed"] = decision.Reviewed
	}
	if decision.ReviewedDate != "" {
		data["reviewed_date"] = decision.ReviewedDate
	}
	if decision.Status != "" {
		data["status"] = decision.Status
	}
	if decision.Type != "" {
		data["type"] = decision.Type
	}
	if decision.PageType != "" {
		data["page_type"] = decision.PageType
	}

	content, err := d.serializeMapAsFrontmatter(ctx, data, decision.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(decision.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", decision.FilePath))
	}

	return nil
}
