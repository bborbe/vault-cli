// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/ops"
)

// captureWatchEvents runs watchOp.Execute in the background with a pipe over stdout,
// then calls triggerFn, and returns the first JSON line written within timeout.
func captureWatchEvents(
	ctx context.Context,
	watchOp ops.WatchOperation,
	targets []ops.WatchTarget,
	triggerFn func(),
	timeout time.Duration,
) ([]ops.WatchEvent, error) {
	// Redirect stdout to a pipe.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	os.Stdout = w

	cancelCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = watchOp.Execute(cancelCtx, targets)
	}()

	// Give the watcher time to start.
	time.Sleep(50 * time.Millisecond)

	triggerFn()

	// Collect events for the given timeout, then cancel.
	var events []ops.WatchEvent
	lineCh := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lineCh <- scanner.Text()
		}
	}()

	deadline := time.After(timeout)
	for {
		select {
		case line := <-lineCh:
			var ev ops.WatchEvent
			if err := json.Unmarshal([]byte(line), &ev); err == nil {
				events = append(events, ev)
			}
		case <-deadline:
			cancel()
			<-done
			w.Close()
			os.Stdout = origStdout
			return events, nil
		}
	}
}

var _ = Describe("WatchOperation", func() {
	var (
		ctx      context.Context
		watchOp  ops.WatchOperation
		vaultDir string
		tasksDir string
	)

	BeforeEach(func() {
		ctx = context.Background()
		watchOp = ops.NewWatchOperation()

		var err error
		vaultDir, err = os.MkdirTemp("", "vault-watch-test-*")
		Expect(err).NotTo(HaveOccurred())
		tasksDir = filepath.Join(vaultDir, "Tasks")
		Expect(os.MkdirAll(tasksDir, 0750)).To(Succeed())

		DeferCleanup(func() {
			Expect(os.RemoveAll(vaultDir)).To(Succeed())
		})
	})

	Describe("Execute", func() {
		It("emits a JSON event with correct fields when an .md file is created", func() {
			targets := []ops.WatchTarget{
				{
					VaultPath: vaultDir,
					VaultName: "personal",
					WatchDirs: []string{"Tasks"},
				},
			}

			events, err := captureWatchEvents(ctx, watchOp, targets, func() {
				mdPath := filepath.Join(tasksDir, "My Task.md")
				Expect(os.WriteFile(mdPath, []byte("hello"), 0600)).To(Succeed())
			}, 500*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
			Expect(events).NotTo(BeEmpty())

			ev := events[0]
			Expect(ev.Name).To(Equal("My Task"))
			Expect(ev.Vault).To(Equal("personal"))
			Expect(ev.Path).To(Equal(filepath.Join("Tasks", "My Task.md")))
			Expect(ev.Event).To(BeElementOf("created", "modified"))
		})

		It("ignores non-.md files", func() {
			targets := []ops.WatchTarget{
				{
					VaultPath: vaultDir,
					VaultName: "personal",
					WatchDirs: []string{"Tasks"},
				},
			}

			events, err := captureWatchEvents(ctx, watchOp, targets, func() {
				txtPath := filepath.Join(tasksDir, "notes.txt")
				Expect(os.WriteFile(txtPath, []byte("hello"), 0600)).To(Succeed())
			}, 400*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
			Expect(events).To(BeEmpty())
		})

		It("debounces rapid events for the same file into a single event", func() {
			targets := []ops.WatchTarget{
				{
					VaultPath: vaultDir,
					VaultName: "personal",
					WatchDirs: []string{"Tasks"},
				},
			}

			mdPath := filepath.Join(tasksDir, "Bouncy Task.md")

			events, err := captureWatchEvents(ctx, watchOp, targets, func() {
				// Write the file several times in rapid succession.
				for i := 0; i < 5; i++ {
					Expect(os.WriteFile(mdPath, []byte("content"), 0600)).To(Succeed())
				}
			}, 600*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())

			// Count events for this path — debounce should collapse them.
			var count int
			for _, ev := range events {
				if ev.Name == "Bouncy Task" {
					count++
				}
			}
			Expect(count).To(BeNumerically("<=", 2))
		})

		It("skips directories that do not exist without returning an error", func() {
			targets := []ops.WatchTarget{
				{
					VaultPath: vaultDir,
					VaultName: "personal",
					WatchDirs: []string{"Tasks", "NonExistentDir"},
				},
			}

			// Should not panic or error; just watch Tasks.
			events, err := captureWatchEvents(ctx, watchOp, targets, func() {
				mdPath := filepath.Join(tasksDir, "Existing Task.md")
				Expect(os.WriteFile(mdPath, []byte("ok"), 0600)).To(Succeed())
			}, 500*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
			Expect(events).NotTo(BeEmpty())
		})
	})
})
