// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"github.com/bborbe/errors"
	unix "golang.org/x/sys/unix"
)

// SessionLock is a held per-session lock.
type SessionLock interface {
	// Release releases the lock by closing the fd. Idempotent; the kernel also
	// releases it on process death, so Release is never strictly required.
	Release() error
	// File returns the underlying open file. FD_CLOEXEC is cleared on it so the
	// fd survives syscall.Exec on the interactive resume path (the resumed
	// claude holds the lock until it exits). Exposed for that exec-survival proof.
	File() *os.File
}

// SessionLocker acquires per-session locks.
type SessionLocker interface {
	// Acquire takes the exclusive, non-blocking per-session lock for sessionID,
	// creating the lock directory on demand. On contention it fails immediately
	// and returns ErrSessionBusy (errors.Is true) wrapped with a message naming
	// the session id. The lock is held until Release or process death.
	Acquire(ctx context.Context, sessionID string) (SessionLock, error)
}

// NewSessionLocker returns a SessionLocker using the default lock directory
// under the user's home.
func NewSessionLocker() SessionLocker {
	return NewSessionLockerWithDir(defaultSessionLockDir())
}

// NewSessionLockerWithDir returns a SessionLocker using the given lock
// directory. Tests inject a temp dir.
func NewSessionLockerWithDir(dir string) SessionLocker {
	return &sessionLocker{dir: dir}
}

// defaultSessionLockDir returns the default per-session lock directory under the
// user's home (real local filesystem, never tmpfs — a lock on a mount cleared on
// reboot would reopen the double-writer window). Fail closed if the home dir
// cannot be resolved: an empty dir makes Acquire's MkdirAll fail, so work-on
// refuses rather than spawning unguarded.
func defaultSessionLockDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "session-locks")
}

type sessionLocker struct {
	dir string
}

func (l *sessionLocker) Acquire(ctx context.Context, sessionID string) (SessionLock, error) {
	if err := os.MkdirAll(l.dir, 0700); err != nil {
		return nil, errors.Wrap(ctx, err, "create session lock dir")
	}
	f, err := os.OpenFile(filepath.Join(l.dir, sessionID+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "open session lock file")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.Wrapf(ctx, ErrSessionBusy, "session %s already has a live claude process", sessionID)
		}
		return nil, errors.Wrap(ctx, err, "lock session file")
	}
	// Clear close-on-exec so the fd survives syscall.Exec on the interactive
	// resume path: the resumed claude process inherits the fd and holds the lock
	// until it exits. Load-bearing — without it the exec would close the fd and a
	// concurrent work-on could double-write a live transcript.
	if _, err := unix.FcntlInt(uintptr(f.Fd()), unix.F_SETFD, 0); err != nil {
		_ = f.Close()
		return nil, errors.Wrap(ctx, err, "clear close-on-exec on session lock fd")
	}
	return &sessionLock{file: f}, nil
}

type sessionLock struct {
	file *os.File
}

func (l *sessionLock) Release() error {
	err := l.file.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func (l *sessionLock) File() *os.File {
	return l.file
}
