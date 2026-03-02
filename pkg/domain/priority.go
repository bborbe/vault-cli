// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"gopkg.in/yaml.v3"
)

// Priority represents a task priority value.
// Valid values are integers >= 0, or -1 for invalid/unparseable values.
type Priority int

// UnmarshalYAML implements custom YAML unmarshaling for Priority.
// If the value is a valid int, use it. Otherwise, set to -1 (invalid).
// This makes priority parsing non-fatal - files with string priority values
// won't cause YAML unmarshal to fail.
func (p *Priority) UnmarshalYAML(value *yaml.Node) error {
	var i int
	if err := value.Decode(&i); err == nil {
		*p = Priority(i)
		return nil
	}
	// If we can't parse as int (e.g., "medium", "high"), use -1
	*p = Priority(-1)
	return nil
}
