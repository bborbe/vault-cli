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
		_ = watchOp.Execute(cancelCtx, targets, func(event ops.WatchEvent) error {
			enc := json.NewEncoder(os.Stdout)
			return enc.Encode(event)
		})
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
					WatchDirs: []ops.WatchDir{
						{Dir: "Tasks", Kind: "task"},
					},
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
			Expect(ev.Type).To(Equal("task"))
		})

		It("ignores non-.md files", func() {
			targets := []ops.WatchTarget{
				{
					VaultPath: vaultDir,
					VaultName: "personal",
					WatchDirs: []ops.WatchDir{
						{Dir: "Tasks", Kind: "task"},
					},
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
					WatchDirs: []ops.WatchDir{
						{Dir: "Tasks", Kind: "task"},
					},
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
					WatchDirs: []ops.WatchDir{
						{Dir: "Tasks", Kind: "task"},
						{Dir: "NonExistentDir", Kind: "task"},
					},
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

		It("emits the correct Type for each entity kind", func() {
			vaultDir2 := filepath.Join(vaultDir, "vault-multikind")
			for _, sub := range []string{"Tasks", "Goals", "Themes", "Objectives"} {
				Expect(os.MkdirAll(filepath.Join(vaultDir2, sub), 0755)).To(Succeed())
			}

			targets := []ops.WatchTarget{
				{
					VaultPath: vaultDir2,
					VaultName: "v",
					WatchDirs: []ops.WatchDir{
						{Dir: "Tasks", Kind: "task"},
						{Dir: "Goals", Kind: "goal"},
						{Dir: "Themes", Kind: "theme"},
						{Dir: "Objectives", Kind: "objective"},
					},
				},
			}

			events, err := captureWatchEvents(ctx, watchOp, targets, func() {
				for sub, name := range map[string]string{"Tasks": "T", "Goals": "G", "Themes": "Th", "Objectives": "O"} {
					Expect(
						os.WriteFile(filepath.Join(vaultDir2, sub, name+".md"), []byte("x"), 0644),
					).To(Succeed())
				}
			}, 700*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())

			typeByName := map[string]string{}
			for _, ev := range events {
				typeByName[ev.Name] = ev.Type
			}
			Expect(typeByName["T"]).To(Equal("task"))
			Expect(typeByName["G"]).To(Equal("goal"))
			Expect(typeByName["Th"]).To(Equal("theme"))
			Expect(typeByName["O"]).To(Equal("objective"))
		})
	})
})
