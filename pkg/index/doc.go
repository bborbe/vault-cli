// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package index keeps the metadata of each vault folder's pages in memory so a
// list read can be answered without opening a vault file. It never holds a
// page body: listed pages carry empty Content, each entry carries its
// pre-parsed Open Questions, and a body is read from disk on request only.
//
// See docs/page-index.md for the full contract.
package index
