// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

// watchProbe writes a markdown file into each given watched directory.
//
// Call it from inside Eventually. The watcher registers its directories when it
// starts and emits no ready signal, so a single write can race that registration
// and be missed; rewriting on every poll is the retry.
func watchProbe(paths ...string) {
	for _, path := range paths {
		Expect(os.WriteFile(path, []byte("---\nstatus: next\n---\n"), 0600)).To(Succeed())
	}
}

// watchStdout returns everything the watcher process has written to stdout so far.
func watchStdout(session *gexec.Session) string {
	return string(session.Out.Contents())
}

var _ = Describe("vault-cli watch --vault comma list", func() {
	It("AC1: watch --vault alpha,beta emits events for both vaults on one stream", func() {
		vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", "alpha,beta")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		alphaProbe := filepath.Join(vaultPathA, "Tasks", "alpha-probe.md")
		betaProbe := filepath.Join(vaultPathB, "Tasks", "beta-probe.md")

		Eventually(func() string {
			watchProbe(alphaProbe, betaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"alpha"`))

		Eventually(func() string {
			watchProbe(alphaProbe, betaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"beta"`))
	})

	It("AC2: watch --vault alpha watches only alpha", func() {
		vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", "alpha")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		alphaProbe := filepath.Join(vaultPathA, "Tasks", "alpha-probe.md")
		betaProbe := filepath.Join(vaultPathB, "Tasks", "beta-probe.md")

		// An alpha event proves the watcher is up and watching alpha.
		Eventually(func() string {
			watchProbe(alphaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"alpha"`))

		// beta was not named, so a change inside it must produce nothing.
		watchProbe(betaProbe)
		Consistently(func() string {
			return watchStdout(session)
		}, "1s", "100ms").ShouldNot(ContainSubstring(`"vault":"beta"`))
	})

	It("AC2: an omitted --vault still watches every configured vault", func() {
		vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		alphaProbe := filepath.Join(vaultPathA, "Tasks", "alpha-probe.md")
		betaProbe := filepath.Join(vaultPathB, "Tasks", "beta-probe.md")

		Eventually(func() string {
			watchProbe(alphaProbe, betaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"alpha"`))

		Eventually(func() string {
			watchProbe(alphaProbe, betaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"beta"`))
	})

	It("AC3: an unresolvable name fails loudly", func() {
		_, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", "alpha,nope")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(1))
		Expect(string(session.Err.Contents())).To(ContainSubstring("nope"))
		Expect(watchStdout(session)).NotTo(ContainSubstring(`"vault"`))
	})

	It("AC3: a value that names no vault fails loudly", func() {
		_, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", ",")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(1))
		Expect(string(session.Err.Contents())).To(ContainSubstring(","))
		Expect(watchStdout(session)).NotTo(ContainSubstring(`"vault"`))
	})

	It("AC4: no other command's --vault accepts a comma list", func() {
		_, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(binPath, "--config", configPath, "task", "list", "--vault", "alpha,beta")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gexec.Exit(1))
		Expect(
			string(session.Err.Contents()),
		).To(ContainSubstring("vault not found: alpha,beta"))
	})

	It("DB6: --types still filters per vault across a comma list", func() {
		vaultPathA, _, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		cmd := exec.Command(
			binPath, "--config", configPath, "watch", "--vault", "alpha,beta", "--types", "goal",
		)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		goalProbe := filepath.Join(vaultPathA, "Goals", "alpha-probe-goal.md")

		Eventually(func() string {
			watchProbe(goalProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(
			And(ContainSubstring(`"type":"goal"`), ContainSubstring(`"vault":"alpha"`)),
		)

		watchProbe(filepath.Join(vaultPathA, "Tasks", "alpha-probe-task.md"))
		Consistently(func() string {
			return watchStdout(session)
		}, "1s", "100ms").ShouldNot(ContainSubstring(`"type":"task"`))
	})

	It("failure mode: a vault with no task directory does not break the list", func() {
		vaultPathA, vaultPathB, configPath, cleanup := createTwoTempVaults(nil, nil, nil, nil)
		defer cleanup()

		Expect(os.RemoveAll(filepath.Join(vaultPathB, "Tasks"))).To(Succeed())

		cmd := exec.Command(binPath, "--config", configPath, "watch", "--vault", "alpha,beta")
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		alphaProbe := filepath.Join(vaultPathA, "Tasks", "alpha-probe.md")

		Eventually(func() string {
			watchProbe(alphaProbe)
			return watchStdout(session)
		}, 10*time.Second, 250*time.Millisecond).Should(ContainSubstring(`"vault":"alpha"`))
	})
})
