// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

// printVersion writes the version string to stdout.
// Used by the CLI's --version flag.
func printVersion(version string) {
	fmt.Printf("vault-cli %s\n", version)
}
