// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import "strings"

// windowsReservedNames lists the device names Windows refuses as a filename
// stem, compared case-insensitively. Source: Microsoft, "Naming Files, Paths,
// and Namespaces" — a name whose text before the first dot is one of these is
// reserved however many extensions follow (NUL.txt and NUL.tar.gz are both NUL).
var windowsReservedNames = []string{
	"CON", "PRN", "AUX", "NUL",
	"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
	"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
}

// SanitizeFilename returns name with the characters Windows forbids in a
// filename replaced or removed, so the result can be checked out on Windows.
// name is a filename stem — the caller appends the extension.
func SanitizeFilename(name string) string {
	result := strings.NewReplacer(
		":", " -",
		"/", "-",
		"\\", "-",
		"*", "x",
		"?", "",
		`"`, "",
		"<", "",
		">", "",
		"|", "",
	).Replace(name)

	result = strings.Join(strings.Fields(result), " ")

	for {
		trimmed := strings.TrimSpace(strings.TrimRight(result, "."))
		if trimmed == result {
			break
		}
		result = trimmed
	}

	result = sanitizeReservedStem(result)

	if result == "" {
		return "Untitled"
	}
	return result
}

// sanitizeReservedStem appends an underscore to a name whose text before the
// first dot is a Windows reserved device name, leaving any remainder intact.
func sanitizeReservedStem(name string) string {
	stem := name
	remainder := ""
	if idx := strings.Index(name, "."); idx >= 0 {
		stem = name[:idx]
		remainder = name[idx:]
	}
	for _, reserved := range windowsReservedNames {
		if strings.EqualFold(stem, reserved) {
			return stem + "_" + remainder
		}
	}
	return name
}
