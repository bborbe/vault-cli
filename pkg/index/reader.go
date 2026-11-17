// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// pageReader reads page files through vault-cli's single-page read, so a page
// parsed here is the page vault-cli's own folder listing produces.
type pageReader struct {
	pages storage.PageStorage
}

// ReadEntry parses one page file, projects its Open Questions, drops the body
// and reports the file's pre-read fingerprint.
func (r *pageReader) ReadEntry(
	ctx context.Context,
	vaultPath string,
	pagesDir string,
	filename string,
) (*Entry, FileFingerprint, error) {
	filePath := filepath.Join(vaultPath, pagesDir, filename)

	fingerprint, err := statFingerprint(filePath)
	if err != nil {
		return nil, FileFingerprint{}, errors.Wrapf(ctx, err, "stat %s", filePath)
	}

	page, err := r.pages.ReadPage(ctx, vaultPath, pagesDir, strings.TrimSuffix(filename, ".md"))
	if err != nil {
		return nil, fingerprint, errors.Wrapf(ctx, err, "read page %s", filePath)
	}
	questions, err := projectOpenQuestions(ctx, string(page.Content))
	if err != nil {
		return nil, fingerprint, errors.Wrapf(ctx, err, "parse open questions %s", filePath)
	}
	metadataOnly := *page
	metadataOnly.Content = ""
	return &Entry{Page: &metadataOnly, OpenQuestions: questions}, fingerprint, nil
}

// ReadPage reads the full page, body included, through the underlying storage.
func (r *pageReader) ReadPage(
	ctx context.Context,
	vaultPath string,
	pagesDir string,
	name string,
) (*domain.Page, error) {
	page, err := r.pages.ReadPage(ctx, vaultPath, pagesDir, name)
	if err != nil {
		return nil, errors.Wrapf(ctx, err, "read page %s", name)
	}
	return page, nil
}

// projectOpenQuestions parses the body's Open Questions section and returns
// each item's index and question text. Every text is cloned, so no retained
// string pins the body in memory. The result is never nil.
func projectOpenQuestions(ctx context.Context, content string) ([]domain.OpenQuestion, error) {
	items, err := storage.ParseOpenQuestions(ctx, content)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "parse open questions")
	}
	questions := make([]domain.OpenQuestion, 0, len(items))
	for _, item := range items {
		questions = append(questions, domain.OpenQuestion{
			Index: item.Index,
			Text:  strings.Clone(item.Question),
		})
	}
	return questions, nil
}
