// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchTarget describes a vault and the directories to watch within it.
type WatchTarget struct {
	VaultPath string
	VaultName string
	WatchDirs []string
}

// WatchEvent is the JSON-encoded event emitted on stdout.
type WatchEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Vault string `json:"vault"`
	Path  string `json:"path"`
}

//counterfeiter:generate -o ../../mocks/watch-operation.go --fake-name WatchOperation . WatchOperation

// WatchOperation watches vault directories and streams change events.
type WatchOperation interface {
	Execute(ctx context.Context, vaults []WatchTarget) error
}

// NewWatchOperation creates a new WatchOperation.
func NewWatchOperation() WatchOperation {
	return &watchOperation{}
}

type watchOperation struct{}

// vaultInfo maps a watched directory back to its vault metadata.
type vaultInfo struct {
	vaultPath string
	vaultName string
}

// Execute watches all vault directories and emits newline-delimited JSON events to stdout.
func (w *watchOperation) Execute(ctx context.Context, vaults []WatchTarget) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	dirToVault := buildDirMap(watcher, vaults)
	enc := json.NewEncoder(os.Stdout)
	debouncer := newDebouncer(100 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return nil
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watch error", "error", watchErr)
		case e, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			handleEvent(e, dirToVault, enc, debouncer)
		}
	}
}

// buildDirMap registers all watch directories and returns a mapping from abs dir → vault info.
func buildDirMap(watcher *fsnotify.Watcher, vaults []WatchTarget) map[string]vaultInfo {
	dirToVault := make(map[string]vaultInfo)
	for _, target := range vaults {
		for _, dir := range target.WatchDirs {
			absDir := filepath.Join(target.VaultPath, dir)
			if _, err := os.Stat(absDir); err != nil {
				slog.Debug("watch skipping missing directory", "dir", absDir)
				continue
			}
			if err := watcher.Add(absDir); err != nil {
				slog.Warn("watch failed", "dir", absDir, "error", err)
				continue
			}
			dirToVault[absDir] = vaultInfo{
				vaultPath: target.VaultPath,
				vaultName: target.VaultName,
			}
		}
	}
	return dirToVault
}

// handleEvent processes one fsnotify event and schedules a debounced emit if applicable.
func handleEvent(
	e fsnotify.Event,
	dirToVault map[string]vaultInfo,
	enc *json.Encoder,
	debouncer *debouncer,
) {
	eventType := mapFsnotifyOp(e.Op)
	if eventType == "" {
		return
	}
	absPath := e.Name
	if !strings.HasSuffix(absPath, ".md") {
		return
	}
	info, found := dirToVault[filepath.Dir(absPath)]
	if !found {
		return
	}
	relPath, err := filepath.Rel(info.vaultPath, absPath)
	if err != nil {
		return
	}
	ev := WatchEvent{
		Event: eventType,
		Name:  strings.TrimSuffix(filepath.Base(absPath), ".md"),
		Vault: info.vaultName,
		Path:  relPath,
	}
	debouncer.schedule(info.vaultName+":"+relPath, func() {
		_ = enc.Encode(ev)
	})
}

// mapFsnotifyOp converts an fsnotify Op to an event type string.
// Returns "" for ops that should be ignored (e.g. Chmod).
func mapFsnotifyOp(op fsnotify.Op) string {
	switch {
	case op.Has(fsnotify.Write):
		return "modified"
	case op.Has(fsnotify.Create):
		return "created"
	case op.Has(fsnotify.Remove):
		return "deleted"
	case op.Has(fsnotify.Rename):
		return "renamed"
	default:
		return ""
	}
}

// debouncer collapses rapid calls for the same key into a single delayed execution.
type debouncer struct {
	mu      sync.Mutex
	delay   time.Duration
	pending map[string]*time.Timer
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		delay:   delay,
		pending: make(map[string]*time.Timer),
	}
}

func (d *debouncer) schedule(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.pending[key]; ok {
		t.Stop()
	}
	d.pending[key] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		delete(d.pending, key)
		d.mu.Unlock()
		fn()
	})
}
