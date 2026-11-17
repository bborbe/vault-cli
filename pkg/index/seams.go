// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"context"
	"time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// FileFingerprint is the pre-read stat of one page file. Size, ModTime and
// StatusChangeTime come from a single stat that follows symlinks.
type FileFingerprint struct {
	Size             int64
	ModTime          time.Time
	StatusChangeTime time.Time
}

// FileEntry is one directory entry with its current fingerprint.
type FileEntry struct {
	Name        string
	Fingerprint FileFingerprint
}

// Entry is one indexed page: the metadata-only page (Content always empty)
// plus its pre-parsed Open Questions. OpenQuestions is never nil, and every
// Text is a copy that shares no memory with the file body.
//
// A published Entry and its Page are read-only for every consumer.
type Entry struct {
	Page          *domain.Page
	OpenQuestions []domain.OpenQuestion
}

//counterfeiter:generate -o ../../mocks/index-page-reader.go --fake-name IndexPageReader . PageReader

// PageReader reads page files from a folder.
type PageReader interface {
	// ReadEntry reads one page file and returns its metadata-only entry and
	// pre-read fingerprint. The filename is the file's base name including the
	// ".md" suffix. A non-nil error means the file is excluded (absent,
	// unreadable, unparsable, or a symlink out of the vault) and the returned
	// fingerprint is still the best available (zero when the stat failed).
	ReadEntry(
		ctx context.Context,
		vaultPath string,
		pagesDir string,
		filename string,
	) (*Entry, FileFingerprint, error)
	// ReadPage reads the full page, body included, by its bare base name
	// (without ".md"). The result is never cached.
	ReadPage(
		ctx context.Context,
		vaultPath string,
		pagesDir string,
		name string,
	) (*domain.Page, error)
}

//counterfeiter:generate -o ../../mocks/index-directory-lister.go --fake-name IndexDirectoryLister . DirectoryLister

// DirectoryLister lists one folder's page-file entries with their current
// fingerprints, in os.ReadDir order (filename ascending).
type DirectoryLister interface {
	ListFiles(ctx context.Context, vaultPath string, pagesDir string) ([]FileEntry, error)
}

// NewPageReader returns the production reader over vault-cli's page storage.
func NewPageReader(pages storage.PageStorage) PageReader {
	return &pageReader{pages: pages}
}

// NewDirectoryLister returns the production directory lister.
func NewDirectoryLister() DirectoryLister {
	return &directoryLister{}
}
