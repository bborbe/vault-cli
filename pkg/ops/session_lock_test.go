// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("SessionLocker", func() {
	var (
		ctx     context.Context
		tempDir string
		locker  ops.SessionLocker
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		tempDir, err = os.MkdirTemp("", "vault-session-lock-*")
		Expect(err).To(BeNil())
		DeferCleanup(func() { _ = os.RemoveAll(tempDir) })
		locker = ops.NewSessionLockerWithDir(tempDir)
	})

	Context("locker contract", func() {
		It("acquires a lock for a session id", func() {
			lock, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			Expect(lock).NotTo(BeNil())
			Expect(lock.Release()).To(Succeed())
		})

		It("refuses a second acquire on the same session id with ErrSessionBusy", func() {
			held, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			defer func() { _ = held.Release() }()

			_, err = locker.Acquire(ctx, "session-abc")
			Expect(err).To(MatchError(ops.ErrSessionBusy))
		})

		It("re-acquires the session lock after Release", func() {
			held, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			Expect(held.Release()).To(Succeed())

			reacquired, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			Expect(reacquired.Release()).To(Succeed())
		})

		It("releases the lock when the fd is closed without Release (process-death proxy)", func() {
			held, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			// The kernel frees a flock when the holding fd closes — the same mechanism
			// that releases it on SIGKILL, since the process dies and its fds close.
			Expect(held.File().Close()).To(Succeed())

			reacquired, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			Expect(reacquired.Release()).To(Succeed())
		})

		It("Release is idempotent — a second release after the fd is closed succeeds", func() {
			held, err := locker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			Expect(held.Release()).To(Succeed())
			Expect(held.Release()).To(Succeed())
		})
	})

	Context("lock directory", func() {
		It("creates a nested lock dir on demand and places the lock at <dir>/<session-id>.lock", func() {
			nested := filepath.Join(tempDir, "nested", "locks")
			nestedLocker := ops.NewSessionLockerWithDir(nested)

			lock, err := nestedLocker.Acquire(ctx, "123e4567-e89b-12d3-a456-426614174000")
			Expect(err).To(BeNil())
			defer func() { _ = lock.Release() }()

			Expect(filepath.Join(nested, "123e4567-e89b-12d3-a456-426614174000.lock")).To(BeAnExistingFile())
		})

		It("resolves the default lock dir under the user's home", func() {
			home, err := os.UserHomeDir()
			Expect(err).To(BeNil())
			Expect(ops.DefaultSessionLockDir()).To(HavePrefix(home))
		})

		It("NewSessionLocker places locks under the default dir resolved from $HOME", func() {
			// Point HOME at a temp dir so the production constructor's default lock
			// directory lands on the temp filesystem, never the real home. The suite
			// runs specs serially, so the env swap is contained to this spec.
			fakeHome, err := os.MkdirTemp("", "vault-session-lock-home-*")
			Expect(err).To(BeNil())
			DeferCleanup(func() { _ = os.RemoveAll(fakeHome) })

			oldHome := os.Getenv("HOME")
			Expect(os.Setenv("HOME", fakeHome)).To(Succeed())
			DeferCleanup(func() { _ = os.Setenv("HOME", oldHome) })

			defaultLocker := ops.NewSessionLocker()
			lock, err := defaultLocker.Acquire(ctx, "session-abc")
			Expect(err).To(BeNil())
			defer func() { _ = lock.Release() }()

			Expect(filepath.Join(fakeHome, ".claude", "session-locks", "session-abc.lock")).To(BeAnExistingFile())
		})
	})

	Context("fail-closed when the lock dir cannot be created", func() {
		var blockerPath string

		BeforeEach(func() {
			blockerPath = filepath.Join(tempDir, "blocker")
			// A regular file where a directory is expected makes MkdirAll fail, so the
			// lock can never be taken and Acquire must fail — never spawn unguarded.
			Expect(os.WriteFile(blockerPath, []byte("not a dir"), 0600)).To(Succeed())
		})

		It("returns an error wrapping the create failure, never a nil success", func() {
			badLocker := ops.NewSessionLockerWithDir(filepath.Join(blockerPath, "locks"))
			_, err := badLocker.Acquire(ctx, "session-abc")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("create session lock dir"))
		})

		It("fails the same way for a different session id — an unguarded spawn is never possible", func() {
			badLocker := ops.NewSessionLockerWithDir(filepath.Join(blockerPath, "locks"))
			_, err := badLocker.Acquire(ctx, "session-xyz")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("create session lock dir"))
		})
	})

	Context("fail-closed when the lock file cannot be opened", func() {
		var readOnlyDir string

		BeforeEach(func() {
			// The dir exists (MkdirAll succeeds) but is read-only, so creating the
			// session lock file inside it fails — Acquire must fail, never spawn unguarded.
			readOnlyDir = filepath.Join(tempDir, "readonly")
			Expect(os.Mkdir(readOnlyDir, 0700)).To(Succeed())
			Expect(os.Chmod(readOnlyDir, 0500)).To(Succeed())
			DeferCleanup(func() { _ = os.Chmod(readOnlyDir, 0700) })
		})

		It("returns an error wrapping the open failure", func() {
			// Running as root, permissions are not enforced; skip the assertion only
			// when the open unexpectedly succeeds (root on CI).
			if os.Geteuid() == 0 {
				Skip("running as root — read-only dirs do not fail opens")
			}
			readOnlyLocker := ops.NewSessionLockerWithDir(readOnlyDir)
			_, err := readOnlyLocker.Acquire(ctx, "session-abc")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("open session lock file"))
		})
	})
})
