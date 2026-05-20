---
status: committing
spec: [013-rename-task-status-phase-taxonomy]
summary: Updated 4 integration test blocks in cli_test.go to match canonical `next` status taxonomy and added -count=1 to Makefile test target to prevent stale cache hiding failures.
container: vault-cli-exec-121-fix-integration-tests-for-taxonomy-rename
dark-factory-version: v0.162.0
created: "2026-05-20T17:27:08Z"
queued: "2026-05-20T17:27:08Z"
started: "2026-05-20T17:27:10Z"
---

<summary>
- Follow-up fix to commit c86f20e (canonical taxonomy flip). Three integration tests in `integration/cli_test.go` were missed by the original spec scope and now fail in CI because they assert the OLD canonical direction. Local `make precommit` passed via Go test cache (integration package has no source change, so `go test ./...` returned cached pass while behavior actually broke).
- Update test `gets a known field value` (~L300): expected output flips from `todo` to `next` — file has `status: todo` on disk, `task get` returns the normalized canonical.
- Update test `normalizes legacy status 'next' to 'todo' on list` (~L341): rewrite as `normalizes legacy status 'todo' to 'next' on list` — fixture flips from `status: next` to `status: todo`, comment flips accordingly.
- Update test `with invalid status` (~L460-488): fixture currently uses `status: next` to trigger INVALID_STATUS, but `next` is now canonical and valid. Replace with truly-unknown status `garbage`.
- Update test `with status: next` / `exits 0, shows FIXED, and updates file to status: todo` (~L530-568): the auto-fix path for aliases is gone. Rewrite the Context as `with legacy status: todo (silently accepted)` — `lint` succeeds with exit 0, reports "No lint issues found", and the on-disk file is unchanged (no rewrite).
- Patch `Makefile` `test` target to add `-count=1` so Go cannot serve stale integration test cache when only `pkg/` source changes.
- After this fix, `make test`, `make precommit`, and CI all pass green.
</summary>

<objective>
Get CI green by updating three integration tests in `integration/cli_test.go` to match the new canonical taxonomy (status `next`, phase `execution`), and prevent the stale-cache trap that hid these failures locally by forcing `-count=1` in the Makefile `test` target.
</objective>

<context>
This is a follow-up to commit `c86f20e` ("Next Status Task") which atomically flipped canonical task status `todo → next` and phase `in_progress → execution`. The original spec 013 listed `pkg/domain/`, `pkg/ops/`, `pkg/storage/` test files but missed `integration/cli_test.go`. The CI failure manifests as 3 Ginkgo `[It]` failures.

Read these files in full before changes:
- `integration/cli_test.go` — 81 specs total; focus on lines ~270-310 (task get/set), ~330-365 (status normalization), ~460-490 (lint with invalid status), ~525-575 (lint --fix with status: next)
- `Makefile` — the `test:` target uses `go test -mod=mod -p=$${GO_TEST_PARALLEL:-1} -cover -race ...`. Add `-count=1` to bust Go's test cache for the integration package on every invocation.

Failing test locations (from CI run https://github.com/bborbe/vault-cli/actions/runs/26177344249):
1. `cli_test.go:300` — `Expect(session.Out).To(gbytes.Say("todo"))` after `task get get-task status` on a file with `status: todo`
2. `cli_test.go:488` — `Expect(session.Out).To(gbytes.Say("INVALID_STATUS"))` on a fixture with `status: next`
3. `cli_test.go:561` — `Expect(session.Out).To(gbytes.Say("FIXED"))` after `lint --fix` on `status: next`

Reference docs:
- `docs/development-patterns.md` — project conventions
- `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` — Ginkgo v2 / Gomega patterns
</context>

<requirements>
### 1. Update `integration/cli_test.go` — `gets a known field value` test (~L290-301)

The fixture writes `status: todo` to disk. `task get` reads through `NormalizeTaskStatus` and now returns canonical `next`.

Find:
```go
Expect(session.Out).To(gbytes.Say("todo"))
```
inside `It("gets a known field value", ...)`.

Change to:
```go
Expect(session.Out).To(gbytes.Say("next"))
```

Do NOT change the on-disk YAML fixture (`status: todo`). The test now correctly exercises the normalize-on-read path.

### 2. Update `integration/cli_test.go` — status normalization test (~L341)

Find the entire `It("normalizes legacy status 'next' to 'todo' on list", ...)` block. Replace its body so the fixture writes `status: todo` instead of `status: next`, and update the `It` name and comment accordingly.

Current:
```go
It("normalizes legacy status 'next' to 'todo' on list", func() {
    _, configPath, cleanup = createTempVault(map[string]string{
        "legacy-task": `---
status: next
priority: 1
---
# Legacy Task
`,
    })

    cmd := exec.Command(
        binPath,
        "--config", configPath,
        "--vault", "test",
        "task", "list",
    )
    session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    Expect(err).NotTo(HaveOccurred())
    Eventually(session).Should(gexec.Exit(0))
    // Task with status "next" should appear (normalized to todo)
    Expect(session.Out).To(gbytes.Say("legacy-task"))
})
```

Change to:
```go
It("normalizes legacy status 'todo' to 'next' on list", func() {
    _, configPath, cleanup = createTempVault(map[string]string{
        "legacy-task": `---
status: todo
priority: 1
---
# Legacy Task
`,
    })

    cmd := exec.Command(
        binPath,
        "--config", configPath,
        "--vault", "test",
        "task", "list",
    )
    session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    Expect(err).NotTo(HaveOccurred())
    Eventually(session).Should(gexec.Exit(0))
    // Task with status "todo" should appear (normalized to next)
    Expect(session.Out).To(gbytes.Say("legacy-task"))
})
```

### 3. Update `integration/cli_test.go` — `vault-cli lint / with invalid status` test (~L460-490)

The fixture currently uses `status: next` to trigger INVALID_STATUS, but `next` is now canonical and valid. Replace the fixture's status with `garbage` (truly unknown — `NormalizeTaskStatus` returns `false`).

Find the `Context("with invalid status", ...)` block. Inside its `BeforeEach`, replace:
```go
"next-status-task": `---
status: next
priority: 2
---
# Task with next status
`,
```
with:
```go
"invalid-status-task": `---
status: garbage
priority: 2
task_identifier: test-uuid-garbage
---
# Task with invalid status
`,
```

Update the map key from `"next-status-task"` to `"invalid-status-task"`. The rest of the `Context` (the `It("exits 1 and reports INVALID_STATUS", ...)` block) stays unchanged — it still asserts exit 1 and INVALID_STATUS output.

Add `task_identifier` to the fixture to avoid the MISSING_TASK_IDENTIFIER side-issue seen in the CI log (lint reports it as a separate ERROR otherwise).

### 4. Update `integration/cli_test.go` — `vault-cli lint --fix / with status: next` test (~L525-575)

The entire `Context("with status: next", ...)` tests an auto-fix path that no longer exists: aliases are silently accepted, not migrated. Rewrite the Context to test silent acceptance.

Find the `Context("with status: next", ...)` block under `Describe("vault-cli lint --fix", ...)`. Replace the entire `Context` block with:

```go
Context("with legacy status: todo (silently accepted alias)", func() {
    BeforeEach(func() {
        vaultPath, configPath, cleanup = createTempVault(map[string]string{
            "legacy-todo-task": `---
status: todo
priority: 2
task_identifier: test-uuid-legacy
---
# Task with legacy todo status
`,
        })
    })

    AfterEach(func() {
        cleanup()
    })

    It("exits 0, reports no issues, and leaves file unchanged", func() {
        cmd := exec.Command(
            binPath,
            "--config",
            configPath,
            "--vault",
            "test",
            "task",
            "lint",
            "--fix",
        )
        session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
        Expect(err).NotTo(HaveOccurred())
        Eventually(session).Should(gexec.Exit(0))
        Expect(session.Out).To(gbytes.Say("No lint issues found"))
        Expect(session.Out).NotTo(gbytes.Say("FIXED"))

        // Verify file was NOT rewritten — alias preserved on disk
        taskPath := filepath.Join(vaultPath, "Tasks", "legacy-todo-task.md")
        content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
        Expect(err).NotTo(HaveOccurred())
        Expect(string(content)).To(ContainSubstring("status: todo"))
        Expect(string(content)).NotTo(ContainSubstring("status: next"))
    })
})
```

This asserts the new spec contract: aliases are valid, lint produces zero issues, and `--fix` does NOT rewrite alias-bearing files.

### 5. Patch `Makefile` `test` target — force `-count=1`

The Go test cache is invalidated only when a package's own source files change. Integration tests don't import `pkg/domain` or `pkg/ops`; they shell out to a `gexec.Build`-rebuilt binary. So when only `pkg/` changes, `go test ./integration/...` returns "ok (cached)" even though the binary behavior changed. This is what hid the original CI failure.

Find the `test:` target in `Makefile`:
```makefile
test:
	go test -mod=mod -p=$${GO_TEST_PARALLEL:-1} -cover -race $(shell go list -mod=mod ./... | grep -v /vendor/)
```

Change to:
```makefile
test:
	go test -mod=mod -count=1 -p=$${GO_TEST_PARALLEL:-1} -cover -race $(shell go list -mod=mod ./... | grep -v /vendor/)
```

`-count=1` disables test result caching across the entire run. Slight cost (a few seconds), large benefit (cache cannot hide behavioral regressions in integration tests).

Do NOT change any other Makefile target.

### 6. Verify `make precommit` exits 0

After the changes, `make precommit` must exit 0. The full chain is `ensure format generate test check addlicense`. The `test` step now runs all 81 integration specs uncached, plus the rest of the suite.
</requirements>

<constraints>
- Do NOT change the canonical task status / phase taxonomy — this is a test-only fix. `pkg/domain/` constants and behavior stay exactly as they are in commit c86f20e.
- Do NOT change any non-test code outside `Makefile`. The only files this prompt touches are `integration/cli_test.go` and `Makefile`.
- Do NOT remove or restructure other Ginkgo `It` / `Context` blocks in `cli_test.go` — only modify the three identified by line number.
- The on-disk YAML in the fixtures may keep using legacy values (`status: todo`) where the test specifically exercises the alias path. The point is to confirm normalize-on-read.
- Follow Ginkgo v2 / Gomega style. No new imports needed; `gbytes`, `gexec`, `filepath`, `os` are already imported.
- Do NOT bump any version strings. The CHANGELOG already has `## v0.65.0` for the underlying rename; this is a CI-fix for the same release.
- Do NOT commit — dark-factory handles git.
- Spec 013 is the parent for traceability; do not move or modify the spec file.
</constraints>

<verification>
```bash
# Direct integration test run, no cache
go test -count=1 ./integration/...
# expected: 81 of 81 specs pass

# Full precommit
make precommit
# expected: exit 0

# Confirm the three test edits stuck
grep -c 'gbytes.Say("next")' integration/cli_test.go
# expected: ≥1

grep -n "normalizes legacy status 'todo' to 'next'" integration/cli_test.go
# expected: 1 match

grep -n 'status: garbage' integration/cli_test.go
# expected: 1 match in the "with invalid status" Context

grep -n 'with legacy status: todo' integration/cli_test.go
# expected: 1 match in the "vault-cli lint --fix" Describe block

# Confirm Makefile patch
grep -n -- '-count=1' Makefile
# expected: 1 match in the test: target

# Confirm no stale references remain
grep -n 'normalizes legacy status .next. to .todo.' integration/cli_test.go
# expected: 0 matches (the old test name is gone)

grep -nE '"next-status-task"' integration/cli_test.go
# expected: 0 matches (the fixture was renamed/removed in tests #3 and #4)
```
</verification>
