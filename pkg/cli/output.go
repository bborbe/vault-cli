// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/bborbe/collection"
	"github.com/bborbe/errors"
	"github.com/bborbe/validation"
)

// OutputFormat represents the output format for CLI commands.
type OutputFormat string

const (
	// OutputFormatPlain represents plain text output.
	OutputFormatPlain OutputFormat = "plain"
	// OutputFormatJSON represents JSON output.
	OutputFormatJSON OutputFormat = "json"
)

// AvailableOutputFormats lists all valid output format values.
var AvailableOutputFormats = OutputFormats{
	OutputFormatPlain,
	OutputFormatJSON,
}

// OutputFormats is a collection of OutputFormat values.
type OutputFormats []OutputFormat

// Contains returns true if the collection contains the given format.
func (f OutputFormats) Contains(format OutputFormat) bool {
	return collection.Contains(f, format)
}

// IsJSON returns true if the format is JSON.
func (f OutputFormat) IsJSON() bool {
	return f == OutputFormatJSON
}

// IsPlain returns true if the format is plain text.
func (f OutputFormat) IsPlain() bool {
	return f == OutputFormatPlain
}

// Validate returns an error if the format is not a valid value.
func (f OutputFormat) Validate(ctx context.Context) error {
	if !AvailableOutputFormats.Contains(f) {
		return errors.Wrapf(ctx, validation.Error, "unknown output format '%s'", f)
	}
	return nil
}

// PrintJSON prints any value as formatted JSON to stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
