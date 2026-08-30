// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// pinnedSessionID is a uuid.Parse-able id the injected generator returns, so every
// equality assertion on the generated/resumed session id is deterministic.
const pinnedSessionID = "123e4567-e89b-12d3-a456-426614174000"

var _ = Describe("work-on session write-back", func() {
	var (
		ctx           context.Context
		vaultPath     string
		sessionDir    string
		storageConfig *storage.Config
		starter       ops.ClaudeSessionStarter
		mockDailyNote *mocks.DailyNoteStorage
		lockDir       string
		locker        ops.SessionLocker
	)

	// newStarter builds a real starter whose detached child runs the given fake.
	// StartSession now blocks until the child exits, so the waiter must block too
	// (via blockWaiter, closed in AfterEach) — a waiter that returns immediately
	// races the select against the child's buffered exit and makes the outcome
	// nondeterministic.
	//
	// StartSession's select can return via the child-exit branch while the
	// waiter goroutine is still blocked on <-blockWaiter; that goroutine only
	// unblocks (and exits) when this spec's DeferCleanup closes it, which can
	// still be in flight when the next spec's BeforeEach reassigns the shared
	// blockWaiter variable. Capturing the channel into a spec-local "bw" here
	// (instead of letting the waiter closure read the outer, reassignable
	// variable directly) gives the leaked goroutine its own private channel to
	// finish reading, so it never races against a later spec's reassignment.
	var blockWaiter chan struct{}
	newStarter := func(detachRun func(args []string, dir string, stdout *os.File) (<-chan error, error)) ops.ClaudeSessionStarter {
		bw := blockWaiter
		return ops.NewClaudeSessionStarterWithRunner(
			"/usr/local/bin/claude",
			nil,
			detachRun,
			libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
				<-bw
				return nil
			}),
			locker,
		)
	}

	BeforeEach(func() {
		ctx = context.Background()
		blockWaiter = make(chan struct{})
		DeferCleanup(func() { close(blockWaiter) })

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-workon-writeback-*")
		Expect(err).To(BeNil())
		// Deliberately NOT the vault path: the re-read must use vaultPath, not the
		// session working directory. If the fix plumbs the wrong value through,
		// FindTaskByName looks in an empty directory and this test fails.
		sessionDir, err = os.MkdirTemp("", "vault-workon-cwd-*")
		Expect(err).To(BeNil())
		lockDir, err = os.MkdirTemp("", "vault-workon-lock-*")
		Expect(err).To(BeNil())
		locker = ops.NewSessionLockerWithDir(lockDir)

		storageConfig = &storage.Config{TasksDir: "24 Tasks", GoalsDir: "23 Goals"}
		for _, dir := range []string{"24 Tasks", "23 Goals"} {
			Expect(os.MkdirAll(filepath.Join(vaultPath, dir), 0755)).To(Succeed())
		}

		starter = nil
		mockDailyNote = &mocks.DailyNoteStorage{}
		mockDailyNote.ReadDailyNoteReturns("", nil)
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
		if sessionDir != "" {
			_ = os.RemoveAll(sessionDir)
		}
		if lockDir != "" {
			_ = os.RemoveAll(lockDir)
		}
	})

	Context("task work-on", func() {
		const taskFixture = `---
assignee: user@example.com
phase: planning
status: in_progress
---
body
`
		var taskStore storage.TaskStorage

		BeforeEach(func() {
			taskStore = storage.NewTaskStorage(storageConfig)
			Expect(os.WriteFile(
				filepath.Join(vaultPath, "24 Tasks", "Repro Task.md"),
				[]byte(taskFixture), 0600,
			)).To(Succeed())

			// Simulate the real headless turn: the spawned Claude session runs
			// plan-task -> execute-task and writes to the very file work-on loaded
			// before the call. The write happens inside the detached child (the
			// detachRun fake) while the parent has already returned within the
			// liveness window.
			detachRun := func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
				fresh, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
				if err != nil {
					return nil, err
				}
				fresh.SetPhase(domain.TaskPhaseExecution.Ptr())
				if err := fresh.SetField(ctx, "session_note", "written by the headless turn"); err != nil {
					return nil, err
				}
				if err := taskStore.WriteTask(ctx, fresh); err != nil {
					return nil, err
				}
				if _, err := stdout.WriteString(
					`{"session_id":"` + pinnedSessionID + `","num_turns":3,"is_error":false,"result":"done"}`,
				); err != nil {
					return nil, err
				}
				done := make(chan error, 1)
				done <- nil
				return done, nil
			}
			starter = newStarter(detachRun)
		})

		It("keeps the frontmatter the session wrote and adds claude_session_id", func() {
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			testVault := config.Vault{
				Path:          vaultPath,
				Name:          "test-vault",
				WorkOnCommand: "/vault-cli:work-on-task",
			}
			workOnOp := ops.NewWorkOnOperation(
				taskStore, mockDailyNote, currentDateTime, func() string { return pinnedSessionID }, starter, nil,
			)

			result, err := workOnOp.Execute(
				ctx, vaultPath, "Repro Task", "user@example.com", "test-vault",
				false, sessionDir, &testVault,
			)
			Expect(err).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(result.SessionID).To(Equal(pinnedSessionID))

			written, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
			Expect(err).To(BeNil())
			// The bug: this was reverted to "planning" by the stale write-back.
			Expect(written.Phase()).NotTo(BeNil())
			Expect(*written.Phase()).To(Equal(domain.TaskPhaseExecution))
			// An unrelated field the session touched must survive too.
			Expect(written.GetField("session_note")).To(Equal("written by the headless turn"))
			// ...and the session id must still be persisted.
			Expect(written.ClaudeSessionID()).To(Equal(pinnedSessionID))
			Expect(written.Status()).To(Equal(domain.TaskStatusInProgress))
			// The metrics entry lands in the same re-read/write that preserved the
			// session's own frontmatter writes (real storage round-trip).
			Expect(written.MetricsSessions()).To(HaveLen(1))
			Expect(written.MetricsSessions()[0].SessionID).To(Equal(pinnedSessionID))
		})
	})

	Context("goal work-on", func() {
		const goalFixture = `---
assignee: user@example.com
phase: planning
status: in_progress
---
body
`
		var goalStore storage.GoalStorage

		BeforeEach(func() {
			goalStore = storage.NewGoalStorage(storageConfig)
			Expect(os.WriteFile(
				filepath.Join(vaultPath, "23 Goals", "Repro Goal.md"),
				[]byte(goalFixture), 0600,
			)).To(Succeed())

			// Simulate the real headless turn: the spawned Claude session runs
			// plan-task -> execute-task and writes to the very file work-on loaded
			// before the call. The write happens inside the detached child (the
			// detachRun fake) while the parent has already returned within the
			// liveness window.
			detachRun := func(_ []string, _ string, stdout *os.File) (<-chan error, error) {
				fresh, err := goalStore.FindGoalByName(ctx, vaultPath, "Repro Goal")
				if err != nil {
					return nil, err
				}
				fresh.SetPhase(domain.GoalPhaseExecution.Ptr())
				if err := fresh.SetField(ctx, "session_note", "written by the headless turn"); err != nil {
					return nil, err
				}
				if err := goalStore.WriteGoal(ctx, fresh); err != nil {
					return nil, err
				}
				if _, err := stdout.WriteString(
					`{"session_id":"` + pinnedSessionID + `","num_turns":3,"is_error":false,"result":"done"}`,
				); err != nil {
					return nil, err
				}
				done := make(chan error, 1)
				done <- nil
				return done, nil
			}
			starter = newStarter(detachRun)
		})

		It("keeps the frontmatter the session wrote and adds claude_session_id", func() {
			testVault := config.Vault{
				Path:              vaultPath,
				Name:              "test-vault",
				WorkOnGoalCommand: "/vault-cli:work-on-goal",
			}
			goalWorkOnOp := ops.NewGoalWorkOnOperation(goalStore, func() string { return pinnedSessionID }, starter, nil)

			result, err := goalWorkOnOp.Execute(
				ctx, vaultPath, "Repro Goal", "user@example.com", "test-vault",
				false, sessionDir, &testVault,
			)
			Expect(err).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(result.SessionID).To(Equal(pinnedSessionID))

			written, err := goalStore.FindGoalByName(ctx, vaultPath, "Repro Goal")
			Expect(err).To(BeNil())
			// The bug: this was reverted to "planning" by the stale write-back.
			Expect(written.Phase()).NotTo(BeNil())
			Expect(*written.Phase()).To(Equal(domain.GoalPhaseExecution))
			// An unrelated field the session touched must survive too.
			Expect(written.GetField("session_note")).To(Equal("written by the headless turn"))
			// ...and the session id must still be persisted.
			Expect(written.ClaudeSessionID()).To(Equal(pinnedSessionID))
			Expect(written.Status()).To(Equal(domain.GoalStatusInProgress))
		})
	})

	Context("when the child exits non-zero inside the liveness window", func() {
		const rollbackFixture = `---
phase: execution
status: in_progress
---
body
`
		var taskStore storage.TaskStorage

		BeforeEach(func() {
			taskStore = storage.NewTaskStorage(storageConfig)
			Expect(os.WriteFile(
				filepath.Join(vaultPath, "24 Tasks", "Repro Task.md"),
				[]byte(rollbackFixture), 0600,
			)).To(Succeed())

			// The liveness window has NOT elapsed when the child exits, so the
			// starter must treat the exit as inside-the-window. A nil-returning
			// waiter would race the select against the child's buffered exit; the
			// waiter stays blocked until the spec is done, so only `done` is ready
			// and the error path is deterministic.
			block := make(chan struct{})
			detachRun := func(_ []string, _ string, _ *os.File) (<-chan error, error) {
				fresh, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
				if err != nil {
					return nil, err
				}
				fresh.SetPhase(domain.TaskPhasePlanning.Ptr())
				if err := taskStore.WriteTask(ctx, fresh); err != nil {
					return nil, err
				}
				done := make(chan error, 1)
				done <- errors.New("exit status 1")
				return done, nil
			}
			starter = ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				detachRun,
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-block
					return nil
				}),
				locker,
			)
			DeferCleanup(func() { close(block) })
		})

		It("never persists a session id and preserves the child's frontmatter write when the child exited non-zero inside the window", func() {
			currentDateTime := libtime.NewCurrentDateTime()
			currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
			testVault := config.Vault{
				Path:          vaultPath,
				Name:          "test-vault",
				WorkOnCommand: "/vault-cli:work-on-task",
			}
			workOnOp := ops.NewWorkOnOperation(
				taskStore, mockDailyNote, currentDateTime, func() string { return pinnedSessionID }, starter, nil,
			)

			// The rollback is caller-side (handleClaudeSession/Execute), so it must be
			// driven through Execute — a direct StartSession call never observes it.
			result, err := workOnOp.Execute(
				ctx, vaultPath, "Repro Task", "user@example.com", "test-vault",
				false, sessionDir, &testVault,
			)
			// The spawn error is never masked.
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
			Expect(err.Error()).To(ContainSubstring("exit status 1"))
			Expect(result.Success).To(BeFalse())

			// The child's write survived; nothing in the new design ever touches it.
			written, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
			Expect(err).To(BeNil())
			Expect(written.Phase()).NotTo(BeNil())
			Expect(*written.Phase()).To(Equal(domain.TaskPhasePlanning))

			// On-disk shape: the id and this run's metrics entry were never written
			// in the first place (the AC6 grep pins keep the count of the id/metrics
			// accessor calls in this file at 2, so the absence is asserted via the
			// raw file). The id itself is the shared fingerprint of both
			// claude_session_id and the metrics_sessions entry, so its absence proves
			// neither was persisted.
			raw, err := os.ReadFile(filepath.Join(vaultPath, "24 Tasks", "Repro Task.md"))
			Expect(err).To(BeNil())
			Expect(strings.Count(string(raw), "claude_session_id:")).To(Equal(0))
			Expect(strings.Contains(string(raw), pinnedSessionID)).To(BeFalse())
			Expect(strings.Contains(string(raw), "phase: planning")).To(BeTrue())
		})
	})

})
