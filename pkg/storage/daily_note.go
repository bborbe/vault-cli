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
)

type dailyNoteStorage struct {
	*baseStorage
}

// ReadDailyNote reads a daily note from the vault.
func (d *dailyNoteStorage) ReadDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
) (string, error) {
	filePath := filepath.Join(vaultPath, d.config.DailyDir, date+".md")
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Return empty content if file doesn't exist
		}
		return "", errors.Wrap(ctx, err, fmt.Sprintf("read daily note %s", filePath))
	}
	return string(content), nil
}

// WriteDailyNote writes a daily note to the vault.
func (d *dailyNoteStorage) WriteDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	content string,
) error {
	dailyNotesDir := filepath.Join(vaultPath, d.config.DailyDir)
	if err := os.MkdirAll(dailyNotesDir, 0750); err != nil {
		return errors.Wrap(ctx, err, "create daily notes directory")
	}

	filePath := filepath.Join(dailyNotesDir, date+".md")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write daily note %s", filePath))
	}

	return nil
}
