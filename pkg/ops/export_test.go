// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

// SessionTurnTimeout exposes the unexported sessionTurnTimeout constant to the
// external ops_test package so tests can assert the duration StartSession hands to
// its waiter against the real value, rather than against a copied literal. Test-only:
// this file is a _test.go file and is not part of the package's public API.
//
// Note this is an alias: asserting only against it locks the wiring (StartSession
// passes the constant, not a stray literal) but NOT the value — a retune moves both
// sides. Tests must also assert the literal to lock the value.
const SessionTurnTimeout = sessionTurnTimeout

// DefaultSessionLockDir exposes the unexported defaultSessionLockDir so tests can
// assert the default lock directory resolves under the user's home. Test-only:
// this file is a _test.go file and is not part of the package's public API.
var DefaultSessionLockDir = defaultSessionLockDir
