// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

// LivenessWindow exposes the unexported livenessWindow constant to the external
// ops_test package so tests can assert the duration StartSession hands to its
// waiter against the real value, rather than against a copied literal. Test-only:
// this file is a _test.go file and is not part of the package's public API.
const LivenessWindow = livenessWindow
