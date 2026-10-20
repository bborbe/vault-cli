// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"strings"

	"github.com/bborbe/errors"
)

// refuseExistingEntityApprovalRecord returns the refusal error for a goal or
// topic row that already carries an approved_by or an approved_at key, and nil
// otherwise. Both keys are checked independently so a row carrying either one
// is refused, and the message names every key that is present.
func refuseExistingEntityApprovalRecord(
	ctx context.Context,
	kind string,
	name string,
	get func(string) any,
) error {
	var present []string
	if get("approved_by") != nil {
		present = append(present, "approved_by")
	}
	if get("approved_at") != nil {
		present = append(present, "approved_at")
	}
	if len(present) == 0 {
		return nil
	}
	return errors.Errorf(
		ctx,
		"refusing to approve %q: %s already carries an approval record (%s); re-approving is refused",
		name, kind, strings.Join(present, ", "),
	)
}
