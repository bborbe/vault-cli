// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	unix "golang.org/x/sys/unix"

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
		locker           ops.SessionLocker
	)

	BeforeEach(func() {
		capturedArgv0 = ""
		capturedArgv = nil
		capturedChdirDir = ""
		execErr = nil
		chDirErr = nil
		lockDir, err := os.MkdirTemp("", "vault-session-lock-resume-*")
		Expect(err).To(BeNil())
		DeferCleanup(func() { _ = os.RemoveAll(lockDir) })
		locker = ops.NewSessionLockerWithDir(lockDir)
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
			locker,
		)
	})

	Context("successful resume", func() {
		It("calls exec with correct args", func() {
			err := resumer.ResumeSession(context.Background(), "session-abc", "/vault/path", "")
			Expect(err).To(BeNil())
			Expect(capturedArgv0).To(Equal("/usr/local/bin/claude"))
			Expect(capturedArgv).To(Equal([]string{"claude", "--resume", "session-abc"}))
		})

		It("changes to cwd before exec", func() {
			_ = resumer.ResumeSession(context.Background(), "session-abc", "/vault/path", "")
			Expect(capturedChdirDir).To(Equal("/vault/path"))
		})
	})

	Context("chdir fails", func() {
		BeforeEach(func() {
			chDirErr = errors.New("permission denied")
		})

		It("returns error without calling exec", func() {
			err := resumer.ResumeSession(context.Background(), "session-abc", "/vault/path", "")
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
			err := resumer.ResumeSession(context.Background(), "session-abc", "/vault/path", "")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("exec failed"))
		})

		It("returns the exec error when a continuation prompt was passed", func() {
			err := resumer.ResumeSession(
				context.Background(),
				"session-abc",
				"/vault/path",
				"/vault-cli:work-on-task \"24 Tasks/T.md\"",
			)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("exec failed"))
		})
	})

	Context("continuation prompt", func() {
		It("appends a non-empty prompt as the last argv element", func() {
			continuation := `/vault-cli:work-on-task "24 Tasks/Start Day - 2026-08-09.md"`
			err := resumer.ResumeSession(
				context.Background(),
				"session-abc",
				"/vault/path",
				continuation,
			)
			Expect(err).To(BeNil())
			Expect(capturedArgv).To(Equal([]string{
				"claude", "--resume", "session-abc", continuation,
			}))
		})

		It("omits an empty prompt from argv", func() {
			err := resumer.ResumeSession(context.Background(), "session-abc", "/vault/path", "")
			Expect(err).To(BeNil())
			Expect(capturedArgv).To(Equal([]string{"claude", "--resume", "session-abc"}))
			Expect(capturedArgv).To(HaveLen(3))
		})

		It("omits a whitespace-only prompt from argv", func() {
			err := resumer.ResumeSession(
				context.Background(),
				"session-abc",
				"/vault/path",
				"  \t ",
			)
			Expect(err).To(BeNil())
			Expect(capturedArgv).To(Equal([]string{"claude", "--resume", "session-abc"}))
			Expect(capturedArgv).To(HaveLen(3))
		})
	})

	Context("custom claude path via NewClaudeResumerForTesting", func() {
		It("uses the given claude path as argv0", func() {
			var capturedArgv0 string
			customResumer := ops.NewClaudeResumerForTesting(
				"/opt/custom-claude",
				func(_ string) error { return nil },
				func(argv0 string, _ []string, _ []string) error {
					capturedArgv0 = argv0
					return nil
				},
				locker,
			)
			err := customResumer.ResumeSession(context.Background(), "session-xyz", "/vault", "")
			Expect(err).To(BeNil())
			Expect(capturedArgv0).To(Equal("/opt/custom-claude"))
		})
	})

	Context("session lock", func() {
		var lockLocker ops.SessionLocker

		BeforeEach(func() {
			// Fresh per-spec temp dir so a held/leaked lock can never bleed across specs.
			lockDir, err := os.MkdirTemp("", "vault-session-lock-resume-spec-*")
			Expect(err).To(BeNil())
			DeferCleanup(func() { _ = os.RemoveAll(lockDir) })
			lockLocker = ops.NewSessionLockerWithDir(lockDir)
		})

		JustBeforeEach(func() {
			// Rebuild the resumer on the fresh per-spec locker so the busy refusal
			// below conflicts on a real flock, not on the suite-level locker.
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
				lockLocker,
			)
		})

		It("refuses with ErrSessionBusy before chdir or exec when the session is locked", func() {
			held, err := lockLocker.Acquire(context.Background(), "session-abc")
			Expect(err).To(BeNil())
			defer func() { _ = held.Release() }()

			err = resumer.ResumeSession(context.Background(), "session-abc", "/vault/path", "")
			Expect(err).NotTo(BeNil())
			Expect(errors.Is(err, ops.ErrSessionBusy)).To(BeTrue())
			Expect(capturedArgv0).To(BeEmpty())
			Expect(capturedChdirDir).To(BeEmpty())
		})

		It("keeps the lock fd not close-on-exec so it survives exec into claude --resume", func() {
			lock, err := lockLocker.Acquire(context.Background(), "session-abc")
			Expect(err).To(BeNil())
			flags, ferr := unix.FcntlInt(uintptr(lock.File().Fd()), unix.F_GETFD, 0)
			Expect(ferr).To(BeNil())
			Expect(flags & unix.FD_CLOEXEC).To(BeZero())
			Expect(lock.Release()).To(Succeed())
		})
	})
})
