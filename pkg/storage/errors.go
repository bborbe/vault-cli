// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	stderrors "errors"
)

// ErrNotFound indicates a requested markdown file could not be resolved by
// name within a vault. Callers (notably the multi-vault dispatcher) test for
// it with errors.Is to distinguish a genuine not-found from other failures
// (e.g. a precondition failure such as incomplete subtasks).
var ErrNotFound = stderrors.New("file not found")
