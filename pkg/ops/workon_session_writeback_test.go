// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"

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
		mockStarter   *mocks.ClaudeSessionStarter
		mockDailyNote *mocks.DailyNoteStorage
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-workon-writeback-*")
		Expect(err).To(BeNil())
		// Deliberately NOT the vault path: the re-read must use vaultPath, not the
		// session working directory. If the fix plumbs the wrong value through,
		// FindTaskByName looks in an empty directory and this test fails.
		sessionDir, err = os.MkdirTemp("", "vault-workon-cwd-*")
		Expect(err).To(BeNil())

		storageConfig = &storage.Config{TasksDir: "24 Tasks", GoalsDir: "23 Goals"}
		for _, dir := range []string{"24 Tasks", "23 Goals"} {
			Expect(os.MkdirAll(filepath.Join(vaultPath, dir), 0755)).To(Succeed())
		}

		mockStarter = &mocks.ClaudeSessionStarter{}
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

			// Simulate the real headless turn: while StartSession blocks, the spawned
			// Claude session runs plan-task -> execute-task and writes to the very file
			// work-on loaded before the call.
			mockStarter.StartSessionStub = func(
				ctx context.Context, _ string, _ string, _ string, _ string, _ bool,
			) error {
				fresh, err := taskStore.FindTaskByName(ctx, vaultPath, "Repro Task")
				if err != nil {
					return err
				}
				fresh.SetPhase(domain.TaskPhaseExecution.Ptr())
				if err := fresh.SetField(ctx, "session_note", "written by the headless turn"); err != nil {
					return err
				}
				if err := taskStore.WriteTask(ctx, fresh); err != nil {
					return err
				}
				return nil
			}
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
				taskStore, mockDailyNote, currentDateTime, func() string { return pinnedSessionID }, mockStarter, nil,
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

			// Simulate the real headless turn: while StartSession blocks, the spawned
			// Claude session runs plan-task -> execute-task and writes to the very file
			// work-on loaded before the call.
			mockStarter.StartSessionStub = func(
				ctx context.Context, _ string, _ string, _ string, _ string, _ bool,
			) error {
				fresh, err := goalStore.FindGoalByName(ctx, vaultPath, "Repro Goal")
				if err != nil {
					return err
				}
				fresh.SetPhase(domain.GoalPhaseExecution.Ptr())
				if err := fresh.SetField(ctx, "session_note", "written by the headless turn"); err != nil {
					return err
				}
				if err := goalStore.WriteGoal(ctx, fresh); err != nil {
					return err
				}
				return nil
			}
		})

		It("keeps the frontmatter the session wrote and adds claude_session_id", func() {
			testVault := config.Vault{
				Path:              vaultPath,
				Name:              "test-vault",
				WorkOnGoalCommand: "/vault-cli:work-on-goal",
			}
			goalWorkOnOp := ops.NewGoalWorkOnOperation(goalStore, func() string { return pinnedSessionID }, mockStarter, nil)

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
})
