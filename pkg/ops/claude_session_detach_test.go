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

var _ = Describe("ClaudeSessionStarter detachment (integration)", func() {
	It("child outlives the parent and survives context cancellation", func() {
		dir, err := os.MkdirTemp("", "vault-claude-detach-*")
		Expect(err).To(BeNil())
		defer os.RemoveAll(dir)
		sentinel := filepath.Join(dir, "sentinel.txt")
		script := filepath.Join(dir, "worker.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\nsleep 12\ntouch "+sentinel+"\n"), 0755)).To(Succeed())
		starter := ops.NewClaudeSessionStarter(script)
		Expect(starter).NotTo(BeNil())
		ctx, cancel := context.WithCancel(context.Background())
		// The child sleeps past the liveness window (12s > 10s), so StartSession
		// waits out the real window and returns nil while the child still runs.
		err = starter.StartSession(ctx, "123e4567-e89b-12d3-a456-426614174000", "prompt", dir, "worker", false)
		Expect(err).To(BeNil())
		cancel()
		// The sentinel appears only because the detached child survived the
		// parent's context cancellation and process exit.
		Eventually(func() bool {
			_, statErr := os.Stat(sentinel)
			return statErr == nil
		}, "20s", "200ms").Should(BeTrue())
	})
})
