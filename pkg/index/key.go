// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import "path/filepath"

// Key identifies one indexed folder: a vault root plus a vault-relative pages dir.
type Key struct {
	VaultPath string
	PagesDir  string
}

// NewKey builds a Key, applying filepath.Clean to both parts so the read side
// and the event side derive identical keys from equivalent paths.
func NewKey(vaultPath, pagesDir string) Key {
	return Key{
		VaultPath: filepath.Clean(vaultPath),
		PagesDir:  filepath.Clean(pagesDir),
	}
}
