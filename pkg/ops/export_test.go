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

// RollupFamilyName exposes rollupFamilyName for the external ops_test package.
var RollupFamilyName = rollupFamilyName

// RollupMedian exposes rollupMedian for the external ops_test package.
var RollupMedian = rollupMedian

// PersistGoalSessionID exposes persistGoalSessionID for the external ops_test package.
var PersistGoalSessionID = persistGoalSessionID

// PersistTopicSessionID exposes persistTopicSessionID for the external ops_test package.
var PersistTopicSessionID = persistTopicSessionID

// CheckClosureGate exposes checkClosureGate for the external ops_test package.
var CheckClosureGate = checkClosureGate

// ParseGoalSectionMembers exposes parseGoalSectionMembers for ops_test.
var ParseGoalSectionMembers = parseGoalSectionMembers

// Closure kinds and outcomes for ops_test.
const (
	ClosureKindGoal            = closureKindGoal
	ClosureKindTopic           = closureKindTopic
	ClosureOutcomeOpen         = closureOutcomeOpen
	ClosureOutcomeUnmeasurable = closureOutcomeUnmeasurable
	ClosureOutcomeClose        = closureOutcomeClose
	ClosureOutcomeTerminal     = closureOutcomeTerminal
)

// ClosureKind and ClosureOutcome expose the gate's enum types to ops_test.
type ClosureKind = closureKind

type ClosureOutcome = closureOutcome

// ClosureMember and ClosureVerdict expose the gate's value types to ops_test.
type ClosureMember = closureMember

type ClosureVerdict = closureVerdict

// CascadeParents exposes cascadeParents for the external ops_test package.
var CascadeParents = cascadeParents

// ErrClaudeOutputUnparseable exposes errClaudeOutputUnparseable for the external
// ops_test package so tests can assert the sentinel with errors.Is. Test-only: this
// file is a _test.go file and is not part of the package's public API.
var ErrClaudeOutputUnparseable = errClaudeOutputUnparseable
