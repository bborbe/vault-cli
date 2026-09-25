// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
)

// blockedByAppendRefusal returns an error when an `add` or `remove` invocation
// targets blocked_by on an entity whose current value is a non-empty scalar.
//
// blockedByList reads that shape as an empty list, so without this check `add`
// would silently overwrite the malformed value with a fresh one-entry list and
// `remove` would report `value not found in list` for a field that is malformed
// rather than empty. Both verbs share this refusal because both read the current
// list through the same reader. Nothing is written on the refusal path: the
// check runs before any mutation, so the file on disk is byte-identical.
func blockedByAppendRefusal(
	ctx context.Context,
	entityNoun, entityName string,
	raw any,
) error {
	if !domain.BlockedByIsScalar(raw) {
		return nil
	}
	return errors.Errorf(ctx,
		"blocked_by on %q is not a list: expected a YAML list, but the frontmatter holds the scalar %v, which every reader discards. Repair it with `vault-cli %s clear %q blocked_by` (or `vault-cli %s set %q blocked_by \"\"`), then record the dependency with `vault-cli %s add %q blocked_by \"[[<Blocker>]]\"`",
		entityName, raw,
		entityNoun, entityName,
		entityNoun, entityName,
		entityNoun, entityName,
	)
}

// listFieldsWithoutSetForm names the list-typed fields whose `set` verb has no
// list form. A non-empty value for one of these cannot be coerced into a list
// without silently reinterpreting what the operator typed — the same class of
// bug spec 050 closes — so `set` refuses and points at `add` instead.
// `tags` and `goals` are deliberately absent: their non-empty values keep
// comma-splitting into a list, unchanged.
var listFieldsWithoutSetForm = map[string]bool{
	"blocked_by": true,
}

// blockedBySetRefusal returns an error when a `set` invocation targets a
// list-typed field that has no list form and the value is non-empty.
//
// The empty value is the documented clear (`vault-cli <noun> set <name>
// blocked_by ""`), which stays legal: it stores an empty string, which every
// reader already treats as an empty list, so it is not malformed data.
//
// Nothing is written on the refusal path: the check runs before SetField and
// before the storage write, so the file on disk is byte-identical.
//
// There is no --force, --coerce or --allow-scalar bypass and no config key or
// environment variable that downgrades this to a warning — the refusal is the
// fix (spec 050 Non-goal invariant).
func blockedBySetRefusal(
	ctx context.Context,
	entityNoun, entityName, key, value string,
) error {
	if !listFieldsWithoutSetForm[key] || value == "" {
		return nil
	}
	return errors.Errorf(ctx,
		"cannot set %s %s on %q: %s is a list field and %q is a scalar that every reader discards. Record a dependency with `vault-cli %s add %q %s \"[[<Blocker>]]\"`; to clear the list use `vault-cli %s set %q %s \"\"` or `vault-cli %s clear %q %s`",
		key, value, entityName, key, value,
		entityNoun, entityName, key,
		entityNoun, entityName, key,
		entityNoun, entityName, key,
	)
}
