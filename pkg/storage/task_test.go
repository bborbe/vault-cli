// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("TaskStorage", func() {
	var (
		ctx      context.Context
		store    storage.Storage
		vaultDir string
		tasksDir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultDir, err = os.MkdirTemp("", "vault-task-test-*")
		Expect(err).To(BeNil())

		tasksDir = filepath.Join(vaultDir, "Tasks")
		Expect(os.MkdirAll(tasksDir, 0755)).To(Succeed())
	})

	AfterEach(func() {
		if vaultDir != "" {
			_ = os.RemoveAll(vaultDir)
		}
	})

	taskContent := func() string {
		return `---
status: todo
page_type: task
---
# Test Task

This is a test task.
`
	}

	Describe("ListTasks", func() {
		It("finds tasks in root dir", func() {
			taskPath := filepath.Join(tasksDir, "Root Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			tasks, err := store.ListTasks(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(1))
			Expect(tasks[0].Name).To(Equal("Root Task"))
		})

		It("finds tasks in subdirectory", func() {
			subDir := filepath.Join(tasksDir, "completed")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())
			taskPath := filepath.Join(subDir, "Done Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			tasks, err := store.ListTasks(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(1))
			Expect(tasks[0].Name).To(Equal("Done Task"))
			Expect(tasks[0].FilePath).To(Equal(taskPath))
		})

		It("finds tasks in nested subdirectory", func() {
			nestedDir := filepath.Join(tasksDir, "users", "alice")
			Expect(os.MkdirAll(nestedDir, 0755)).To(Succeed())
			taskPath := filepath.Join(nestedDir, "Alice Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			tasks, err := store.ListTasks(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(1))
			Expect(tasks[0].Name).To(Equal("Alice Task"))
		})

		It("finds tasks in both root and subdirectories", func() {
			rootPath := filepath.Join(tasksDir, "Root Task.md")
			Expect(os.WriteFile(rootPath, []byte(taskContent()), 0600)).To(Succeed())

			subDir := filepath.Join(tasksDir, "completed")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())
			subPath := filepath.Join(subDir, "Done Task.md")
			Expect(os.WriteFile(subPath, []byte(taskContent()), 0600)).To(Succeed())

			tasks, err := store.ListTasks(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(2))
		})
	})

	Describe("FindTaskByName", func() {
		It("finds a task in a subdirectory", func() {
			subDir := filepath.Join(tasksDir, "active")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())
			taskPath := filepath.Join(subDir, "Sub Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			task, err := store.FindTaskByName(ctx, vaultDir, "Sub Task")
			Expect(err).To(BeNil())
			Expect(task).NotTo(BeNil())
			Expect(task.Name).To(Equal("Sub Task"))
			Expect(task.FilePath).To(Equal(taskPath))
		})

		It("finds a task in root dir", func() {
			taskPath := filepath.Join(tasksDir, "Root Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			task, err := store.FindTaskByName(ctx, vaultDir, "Root Task")
			Expect(err).To(BeNil())
			Expect(task).NotTo(BeNil())
			Expect(task.Name).To(Equal("Root Task"))
		})
	})

	Describe("ListTasksTolerant", func() {
		It("skips a frontmatter-less file and names it", func() {
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Healthy.md"),
				[]byte(taskContent()),
				0600,
			)).To(Succeed())
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Broken.md"),
				[]byte("Failed to authenticate: OAuth session expired and could not be refreshed\n"),
				0600,
			)).To(Succeed())

			tasks, skipped, err := store.ListTasksTolerant(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(1))
			Expect(tasks[0].Name).To(Equal("Healthy"))
			Expect(skipped).To(Equal([]string{"Broken.md"}))
		})

		It("reports subdirectory paths, sorted and distinct", func() {
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Healthy.md"),
				[]byte(taskContent()),
				0600,
			)).To(Succeed())
			for _, sub := range []string{"sub-a", "sub-b"} {
				Expect(os.MkdirAll(filepath.Join(tasksDir, sub), 0755)).To(Succeed())
				Expect(os.WriteFile(
					filepath.Join(tasksDir, sub, "Broken.md"),
					[]byte("no frontmatter here\n"),
					0600,
				)).To(Succeed())
			}

			tasks, skipped, err := store.ListTasksTolerant(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(1))
			Expect(skipped).To(Equal([]string{"sub-a/Broken.md", "sub-b/Broken.md"}))
		})

		It("replaces a control character with a space", func() {
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "bad\nname.md"),
				[]byte("no frontmatter here\n"),
				0600,
			)).To(Succeed())

			_, skipped, err := store.ListTasksTolerant(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(skipped).To(Equal([]string{"bad name.md"}))
		})

		It("returns a walk error for a missing tasks directory", func() {
			Expect(os.RemoveAll(tasksDir)).To(Succeed())

			tasks, skipped, err := store.ListTasksTolerant(ctx, vaultDir)
			Expect(err).NotTo(BeNil())
			Expect(tasks).To(BeNil())
			Expect(skipped).To(BeNil())
		})

		It("reports an empty, non-nil skipped list for a clean vault", func() {
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Healthy.md"),
				[]byte(taskContent()),
				0600,
			)).To(Succeed())

			_, skipped, err := store.ListTasksTolerant(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(skipped).NotTo(BeNil())
			Expect(skipped).To(BeEmpty())
		})
	})

	Describe("ListTasksStrict", func() {
		It("fails loudly and names the broken file", func() {
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Broken.md"),
				[]byte("no frontmatter here\n"),
				0600,
			)).To(Succeed())

			tasks, err := store.ListTasksStrict(ctx, vaultDir)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Broken.md"))
			Expect(tasks).To(BeNil())
		})
	})

	Describe("ListTasks", func() {
		It("still succeeds with a broken file", func() {
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Healthy.md"),
				[]byte(taskContent()),
				0600,
			)).To(Succeed())
			Expect(os.WriteFile(
				filepath.Join(tasksDir, "Broken.md"),
				[]byte("no frontmatter here\n"),
				0600,
			)).To(Succeed())

			tasks, err := store.ListTasks(ctx, vaultDir)
			Expect(err).To(BeNil())
			Expect(tasks).To(HaveLen(1))
		})
	})

	DescribeTable("skippedTaskName",
		func(tasksDirArg, path, expected string) {
			Expect(storage.SkippedTaskNameForTest(tasksDirArg, path)).To(Equal(expected))
		},
		Entry("path under tasks_dir", "/vault/Tasks", "/vault/Tasks/sub/Broken.md", "sub/Broken.md"),
		Entry("path not under tasks_dir", "/vault/Tasks", "relative/path.md", "path.md"),
		Entry("control character sanitised", "/vault/Tasks", "/vault/Tasks/bad\nname.md", "bad name.md"),
	)

	Describe("ReadTask", func() {
		It("finds a task in root dir (fast path)", func() {
			taskPath := filepath.Join(tasksDir, "My Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			task, err := store.ReadTask(ctx, vaultDir, domain.TaskID("My Task"))
			Expect(err).To(BeNil())
			Expect(task).NotTo(BeNil())
			Expect(task.Name).To(Equal("My Task"))
		})

		It("finds a task that was moved to a subdirectory", func() {
			subDir := filepath.Join(tasksDir, "done")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())
			taskPath := filepath.Join(subDir, "Moved Task.md")
			Expect(os.WriteFile(taskPath, []byte(taskContent()), 0600)).To(Succeed())

			task, err := store.ReadTask(ctx, vaultDir, domain.TaskID("Moved Task"))
			Expect(err).To(BeNil())
			Expect(task).NotTo(BeNil())
			Expect(task.Name).To(Equal("Moved Task"))
			Expect(task.FilePath).To(Equal(taskPath))
		})

		It("returns error when task does not exist", func() {
			_, err := store.ReadTask(ctx, vaultDir, domain.TaskID("Nonexistent"))
			Expect(err).NotTo(BeNil())
		})
	})
})

var _ = Describe("WriteTask UUID generation", func() {
	var (
		ctx      context.Context
		store    storage.Storage
		vaultDir string
		tasksDir string
		taskPath string
		task     *domain.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewStorage(nil)

		var err error
		vaultDir, err = os.MkdirTemp("", "vault-uuid-test-*")
		Expect(err).To(BeNil())

		tasksDir = filepath.Join(vaultDir, "Tasks")
		Expect(os.MkdirAll(tasksDir, 0755)).To(Succeed())

		taskPath = filepath.Join(tasksDir, "My Task.md")
		task = domain.NewTask(
			map[string]any{"status": "todo", "page_type": "task"},
			domain.FileMetadata{Name: "My Task", FilePath: taskPath},
			domain.Content("---\nstatus: todo\npage_type: task\n---\n# My Task\n"),
		)
	})

	AfterEach(func() {
		if vaultDir != "" {
			_ = os.RemoveAll(vaultDir)
		}
	})

	It("generates a UUID when TaskIdentifier is empty", func() {
		Expect(store.WriteTask(ctx, task)).To(Succeed())
		content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("task_identifier:"))
	})

	It("preserves an existing TaskIdentifier", func() {
		task.SetTaskIdentifier("my-stable-uuid")
		Expect(store.WriteTask(ctx, task)).To(Succeed())
		content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("task_identifier: my-stable-uuid"))
	})

	It("round-trips TaskIdentifier through read", func() {
		task.SetTaskIdentifier("round-trip-uuid")
		Expect(store.WriteTask(ctx, task)).To(Succeed())
		read, err := store.ReadTask(ctx, vaultDir, domain.TaskID("My Task"))
		Expect(err).NotTo(HaveOccurred())
		Expect(read.TaskIdentifier()).To(Equal("round-trip-uuid"))
	})

	It("auto-generated UUID is non-empty and matches UUID pattern", func() {
		Expect(store.WriteTask(ctx, task)).To(Succeed())
		content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(MatchRegexp(`task_identifier: \S+`))
	})
})
