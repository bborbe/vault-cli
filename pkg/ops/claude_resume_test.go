// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ClaudeResumer", func() {
	var (
		resumer          ops.ClaudeResumer
		capturedArgv0    string
		capturedArgv     []string
		capturedChdirDir string
		execErr          error
		chDirErr         error
	)

	BeforeEach(func() {
		capturedArgv0 = ""
		capturedArgv = nil
		capturedChdirDir = ""
		execErr = nil
		chDirErr = nil
	})

	JustBeforeEach(func() {
		capturedExecErr := execErr
		capturedChDirErr := chDirErr
		resumer = ops.NewClaudeResumerForTesting(
			"/usr/local/bin/claude",
			func(dir string) error {
				capturedChdirDir = dir
				return capturedChDirErr
			},
			func(argv0 string, argv []string, _ []string) error {
				capturedArgv0 = argv0
				capturedArgv = argv
				return capturedExecErr
			},
		)
	})

	Context("successful resume", func() {
		It("calls exec with correct args", func() {
			err := resumer.ResumeSession("session-abc", "/vault/path")
			Expect(err).To(BeNil())
			Expect(capturedArgv0).To(Equal("/usr/local/bin/claude"))
			Expect(capturedArgv).To(Equal([]string{"claude", "--resume", "session-abc"}))
		})

		It("changes to cwd before exec", func() {
			_ = resumer.ResumeSession("session-abc", "/vault/path")
			Expect(capturedChdirDir).To(Equal("/vault/path"))
		})
	})

	Context("chdir fails", func() {
		BeforeEach(func() {
			chDirErr = errors.New("permission denied")
		})

		It("returns error without calling exec", func() {
			err := resumer.ResumeSession("session-abc", "/vault/path")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("change directory"))
			Expect(capturedArgv0).To(BeEmpty())
		})
	})

	Context("exec fails", func() {
		BeforeEach(func() {
			execErr = errors.New("exec failed")
		})

		It("returns exec error", func() {
			err := resumer.ResumeSession("session-abc", "/vault/path")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("exec failed"))
		})
	})
})
