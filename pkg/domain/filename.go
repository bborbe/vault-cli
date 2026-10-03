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
// name may be a bare stem or a full filename carrying one or more extensions;
// the reserved-device rule is defined on the text before the first dot either
// way.
//
// The steps run in a fixed order: forbidden characters are replaced, whitespace
// runs collapse to single spaces, and only then are the remaining ASCII control
// characters removed — so a tab or newline survives the collapse as word
// separation rather than gluing its neighbours together. A name left empty by
// these steps becomes "Untitled".
//
// It deliberately does not attempt the Windows restrictions that are neither a
// character nor a reserved stem, such as path-length limits.
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

	result = stripControlCharacters(result)

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

// stripControlCharacters removes the ASCII control bytes 0x00–0x1F and 0x7F,
// which Windows forbids in a filename. It runs after whitespace has already
// been collapsed, so the whitespace control characters (tab, newline, vertical
// tab, form feed, carriage return) have become ordinary spaces by this point.
func stripControlCharacters(name string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, name)
}

// sanitizeReservedStem appends an underscore to a name whose text before the
// first dot is a Windows reserved device name, leaving any remainder intact.
// Trailing spaces and dots are trimmed from that stem before the comparison, so
// "NUL .txt" is caught exactly as "NUL.txt" is.
func sanitizeReservedStem(name string) string {
	stem := name
	remainder := ""
	if idx := strings.Index(name, "."); idx >= 0 {
		stem = name[:idx]
		remainder = name[idx:]
	}
	trimmed := strings.TrimRight(stem, " .")
	for _, reserved := range windowsReservedNames {
		if strings.EqualFold(trimmed, reserved) {
			return trimmed + "_" + remainder
		}
	}
	return name
}
