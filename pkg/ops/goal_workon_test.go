// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("GoalWorkOnOperation", func() {
	var (
		ctx             context.Context
		err             error
		result          ops.MutationResult
		goalWorkOnOp    ops.GoalWorkOnOperation
		mockGoalStorage *mocks.GoalStorage
		mockStarter     *mocks.ClaudeSessionStarter
		mockResumer     *mocks.ClaudeResumer
		vaultPath       string
		goalName        string
		assignee        string
		goal            *domain.Goal
		isInteractive   bool
		testVault       config.Vault
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		mockStarter = &mocks.ClaudeSessionStarter{}
		mockResumer = &mocks.ClaudeResumer{}
		goalWorkOnOp = ops.NewGoalWorkOnOperation(
			mockGoalStorage,
			func() string { return pinnedSessionID },
			mockStarter,
			mockResumer,
		)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"
		assignee = "user@example.com"
		isInteractive = false
		testVault = config.Vault{
			Path:              vaultPath,
			Name:              "test-vault",
			WorkOnGoalCommand: "/vault-cli:work-on-goal",
		}

		goal = domain.NewGoal(
			map[string]any{"status": "next"},
			domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"},
			domain.Content(""),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
		mockStarter.StartSessionReturns(nil)
		mockResumer.ResumeSessionReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = goalWorkOnOp.Execute(
			ctx,
			vaultPath,
			goalName,
			assignee,
			"test-vault",
			isInteractive,
			vaultPath,
			&testVault,
		)
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindGoalByName", func() {
			// Twice: once to load the goal, once to re-read it before spawning so the
			// session id lands on the freshest on-disk state.
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(2))
			actualCtx, actualVaultPath, actualGoalName := mockGoalStorage.FindGoalByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualGoalName).To(Equal(goalName))
		})

		It("re-reads the goal from the vault path before spawning the session", func() {
			// The second FindGoalByName is the pre-spawn persist re-read: under the
			// non-interactive branch the session id is persisted before the child is
			// spawned, so the session's own read-modify-write always reads a file that
			// already contains it.
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(2))
			_, reReadVaultPath, reReadGoalName := mockGoalStorage.FindGoalByNameArgsForCall(1)
			Expect(reReadVaultPath).To(Equal(vaultPath))
			Expect(reReadGoalName).To(Equal(goalName))
		})

		It("marks goal as in_progress", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusInProgress))
		})

		It("sets assignee correctly", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee()).To(Equal(assignee))
		})

		It("starts a claude session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
		})

		It("passes goal name to session starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, name, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(name).To(Equal(goalName))
		})

		It("passes isInteractive=false to the starter on the non-interactive branch", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, _, isInteractiveArg := mockStarter.StartSessionArgsForCall(0)
			Expect(isInteractiveArg).To(BeFalse())
		})
	})

	Context("when assignee already equals current user", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "next", "assignee": assignee},
				domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the existing assignee", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee()).To(Equal(assignee))
		})

		It("emits no assignee warning", func() {
			Expect(result.Warnings).NotTo(ContainElement(ContainSubstring("assignee not updated")))
		})
	})

	Context("when assignee is set to a different user", func() {
		const otherUser = "alice@example.com"

		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "next", "assignee": otherUser},
				domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the other user's assignment", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee()).To(Equal(otherUser))
		})

		It("emits an assignee-not-updated warning naming both users", func() {
			Expect(result.Warnings).To(ContainElement(ContainSubstring("assignee not updated")))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(otherUser)))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(assignee)))
		})

		It("still marks the goal in_progress (status is independent of assignee)", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusInProgress))
		})
	})

	Context("custom work on command", func() {
		BeforeEach(func() {
			testVault.WorkOnGoalCommand = "/custom-cmd"
		})

		It("uses the configured work on command in the prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, prompt, _, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(`^/custom-cmd "`))
		})

		It("appends --non-interactive to the bootstrap prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, prompt, _, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(` --non-interactive$`))
			Expect(prompt).To(MatchRegexp(`/path/to/vault/Goals/my-goal\.md`))
		})
	})

	Context("when starter is nil and goal has no cached session ID", func() {
		BeforeEach(func() {
			goalWorkOnOp = ops.NewGoalWorkOnOperation(
				mockGoalStorage,
				func() string { return pinnedSessionID },
				nil,
				nil,
			)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("skips session start", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("emits warning about missing starter", func() {
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("claude session: claude session starter unavailable")))
		})

		It("returns empty session ID", func() {
			Expect(result.SessionID).To(Equal(""))
		})
	})

	Context("when goal already has a session ID", func() {
		BeforeEach(func() {
			goal.SetClaudeSessionID("existing-session")
		})

		It("does not start a new session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns cached session ID", func() {
			Expect(result.SessionID).To(Equal("existing-session"))
		})
	})

	Context("when session start fails (hard failure)", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(ErrTest)
		})

		It("returns wrapped error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
		})

		It("returns Success=false", func() {
			Expect(result.Success).To(BeFalse())
		})
	})

	Context("when claude returns zero turns", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(
				errors.New(ctx, "claude returned 0 turns: Unknown command: /x"),
			)
		})

		It("returns non-nil error wrapped with start work-on session and Success=false", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
			Expect(err.Error()).To(ContainSubstring("claude returned 0 turns: Unknown command: /x"))
			Expect(result.Success).To(BeFalse())
		})

		It("still marks goal as in_progress", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusInProgress))
		})
	})

	Context("interactive mode", func() {
		BeforeEach(func() {
			isInteractive = true
		})

		It("calls ResumeSession", func() {
			Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
			_, sessionID, cwd, prompt := mockResumer.ResumeSessionArgsForCall(0)
			Expect(sessionID).To(Equal(pinnedSessionID))
			Expect(cwd).To(Equal(vaultPath))
			Expect(prompt).To(BeEmpty())
		})

		It("passes isInteractive=true to the starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, _, _, isInteractiveArg := mockStarter.StartSessionArgsForCall(0)
			Expect(isInteractiveArg).To(BeTrue())
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("goal not found", func() {
		BeforeEach(func() {
			mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not call WriteGoal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			mockGoalStorage.WriteGoalReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})

	Context("when persisting the goal session id before spawn", func() {
		var writeGoalAt, spawnAt time.Time

		BeforeEach(func() {
			mockGoalStorage.WriteGoalStub = func(_ context.Context, g *domain.Goal) error {
				if g.ClaudeSessionID() != "" {
					writeGoalAt = time.Now()
				}
				return nil
			}
			realStarter := ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string) (<-chan error, error) {
					spawnAt = time.Now()
					return make(chan error), nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil }),
			)
			goalWorkOnOp = ops.NewGoalWorkOnOperation(
				mockGoalStorage,
				func() string { return pinnedSessionID },
				realStarter,
				mockResumer,
			)
		})

		It("writes the goal session id to storage before the runner spawns the child", func() {
			Expect(err).To(BeNil())
			Expect(writeGoalAt).NotTo(BeZero())
			Expect(spawnAt).NotTo(BeZero())
			Expect(writeGoalAt.Before(spawnAt)).To(BeTrue())
		})
	})

	Context("when capturing the spawned claude argv", func() {
		var capturedArgs []string

		BeforeEach(func() {
			realStarter := ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(args []string, _ string) (<-chan error, error) {
					capturedArgs = args
					return make(chan error), nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil }),
			)
			goalWorkOnOp = ops.NewGoalWorkOnOperation(
				mockGoalStorage,
				func() string { return pinnedSessionID },
				realStarter,
				mockResumer,
			)
		})

		It("passes --session-id <id>, --print, and -n <goal name> to the detached child", func() {
			Expect(err).To(BeNil())
			Expect(capturedArgs).To(ContainElement("--print"))
			Expect(capturedArgs).To(ContainElement("--session-id"))
			Expect(capturedArgs).To(ContainElement(pinnedSessionID))
			Expect(capturedArgs).To(ContainElement("-n"))
			Expect(capturedArgs).To(ContainElement(goalName))
		})
	})

	Context("goal work-on early exit rollback", func() {
		var realVaultPath string
		var realGoalStore storage.GoalStorage

		BeforeEach(func() {
			var mkErr error
			realVaultPath, mkErr = os.MkdirTemp("", "vault-goal-rollback-*")
			Expect(mkErr).To(BeNil())
			for _, dir := range []string{"24 Tasks", "23 Goals"} {
				Expect(os.MkdirAll(filepath.Join(realVaultPath, dir), 0755)).To(Succeed())
			}
			realGoalStore = storage.NewGoalStorage(&storage.Config{TasksDir: "24 Tasks", GoalsDir: "23 Goals"})

			const rollbackFixture = `---
phase: planning
status: in_progress
---
body
`
			Expect(os.WriteFile(
				filepath.Join(realVaultPath, "23 Goals", "Rollback Goal.md"),
				[]byte(rollbackFixture), 0600,
			)).To(Succeed())

			// The liveness window has NOT elapsed when the child exits, so the starter
			// must treat the exit as inside-the-window. A nil-returning waiter would
			// race the select against the child's buffered exit; the waiter stays
			// blocked until the spec is done, so only `done` is ready and the error
			// path is deterministic.
			block := make(chan struct{})
			realStarter := ops.NewClaudeSessionStarterWithRunner(
				"/usr/local/bin/claude",
				nil,
				func(_ []string, _ string) (<-chan error, error) {
					// The child writes frontmatter before dying: the compensating clear
					// must re-read this write and preserve it.
					fresh, ferr := realGoalStore.FindGoalByName(ctx, realVaultPath, "Rollback Goal")
					if ferr != nil {
						return nil, ferr
					}
					fresh.SetPhase(domain.GoalPhaseExecution.Ptr())
					if ferr := realGoalStore.WriteGoal(ctx, fresh); ferr != nil {
						return nil, ferr
					}
					done := make(chan error, 1)
					done <- stderrors.New("exit status 1")
					return done, nil
				},
				libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error {
					<-block
					return nil
				}),
			)
			DeferCleanup(func() { close(block) })
			DeferCleanup(func() { _ = os.RemoveAll(realVaultPath) })

			goalWorkOnOp = ops.NewGoalWorkOnOperation(
				realGoalStore,
				func() string { return pinnedSessionID },
				realStarter,
				nil,
			)
			vaultPath = realVaultPath
			goalName = "Rollback Goal"
		})

		It("goal work-on early exit rollback preserves the child's frontmatter write", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
			Expect(err.Error()).To(ContainSubstring("exit status 1"))
			Expect(result.Success).To(BeFalse())

			// The child's write survived the compensating clear; the pre-persisted id
			// is gone.
			written, ferr := realGoalStore.FindGoalByName(ctx, realVaultPath, "Rollback Goal")
			Expect(ferr).To(BeNil())
			Expect(written.ClaudeSessionID()).To(Equal(""))
			Expect(written.Phase()).NotTo(BeNil())
			Expect(*written.Phase()).To(Equal(domain.GoalPhaseExecution))
		})
	})

	Context("when the clear after a spawn failure cannot re-read the goal", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(ErrTest)
			// call 0 = Execute load, call 1 = pre-spawn persist re-read,
			// call 2 = the compensating clear's re-read.
			mockGoalStorage.FindGoalByNameReturnsOnCall(2, nil, ErrTest)
		})

		It("returns the spawn error, never masked by the failed clear", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
		})

		It("surfaces the failed clear as a warning", func() {
			Expect(result.Warnings).To(ContainElement(
				ContainSubstring("failed to clear claude session id after spawn failure"),
			))
		})
	})
})
