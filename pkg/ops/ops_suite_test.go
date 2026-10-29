// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"errors"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var ErrTest = errors.New("test error")

// skipIfRoot skips the calling spec when the process runs as root. Root bypasses
// file permission bits, so a file chmod'ed read-only is still writable and the
// write error these specs assert can never occur. The dark-factory container runs
// as root; the host and GitHub CI do not.
func skipIfRoot() {
	if os.Geteuid() == 0 {
		Skip("running as root: permission bits are ignored, so no write error can occur")
	}
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6@v6.12.2 -generate

func TestSuite(t *testing.T) {
	time.Local = time.UTC
	format.TruncatedDiff = false
	RegisterFailHandler(Fail)
	RunSpecs(t, "ops Test Suite")
}
