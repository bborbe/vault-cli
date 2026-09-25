---
status: completed
spec: [051-topic-command-ladder]
execution_id: vault-cli-topic-ladder-exec-225-spec-051-topic-search-test-path
dark-factory-version: v0.196.0
created: "2026-09-20T22:09:14Z"
queued: "2026-09-21T05:43:46Z"
started: "2026-09-21T05:44:03Z"
completed: "2026-09-21T05:46:47Z"
---

# Make the topic search integration spec environment-independent

<summary>
- One integration spec for the topic search command passes only on a machine where the semantic search helper program is not installed, and fails on a machine where it is.
- The spec still runs the real command; it now runs it with a restricted search path, so the helper can never be found, on any machine.
- Because the helper is never found, the failure the command reports names it — and that is what proves the invocation reached the search operation instead of stopping at the command layer.
- The spec asserts the command fails, and that the reported failure names the missing helper.
- The spec keeps its existing check that the command is not reported as an unknown command, so it still tells "leaf registered and dispatched" apart from "leaf missing".
- The comment that explained the old environment-dependent behaviour is replaced by one stating that the restricted search path is what makes the result the same everywhere.
- The spec's timeout is unchanged; with the restricted search path the failure is immediate, so the existing default is ample.
- Nothing the binary ships changes — the production code and every other test are untouched.
</summary>

<objective>
Make the topic-search integration spec environment-independent by executing the command with a search path that cannot resolve `semantic-search-mcp`, so the spec passes on a developer host and inside the YOLO container alike while still proving the dispatch reaches the search operation. Today the spec passes only where the binary is absent: `pkg/ops/search.go` calls `exec.LookPath("semantic-search-mcp")` and, when it finds the binary, runs it with `cmd.CombinedOutput()` — a blocking wait. On a host where the binary is on `PATH` the real search runs and exceeds `Eventually`'s one-second default, so `make precommit` fails on the host while passing in the container. A spec that passes in one environment and fails in the other is the defect; the fix is to make the lookup fail deterministically everywhere, not to widen the timeout or drop the execution.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then these files.

- `integration/cli_test.go` — the only file this prompt changes. Read the spec `It("topic search dispatches to the semantic search operation scoped to the topics directory")`, nested in `Describe("vault-cli topic command family")` (currently around line 3097), together with its fixture `createTempVaultWithTopicPages` (which sets `topics_dir`, `current_user` and an uninstalled `claude_script`), and the sibling refusal specs in the same `Describe` block that use `Eventually(...).Should(gexec.Exit(1))` — the dominant exit-assertion form in that block. Also read `Describe("command registration")` — its `Entry("topic search", "topic", "search")` row is a registration-only `--help` check and is NOT the spec this prompt touches; leave it alone.
- `pkg/ops/search.go` — read `searchOperation.Execute`. Its guard is `if _, err := exec.LookPath("semantic-search-mcp"); err != nil { return nil, errors.Wrap(ctx, err, "semantic-search-mcp not found on PATH") }`. The lookup runs BEFORE any subprocess is started, so a `PATH` that cannot resolve the binary turns the whole invocation into an immediate, deterministic error. This file is read-only for this prompt.
- `pkg/cli/cli.go` — read `createGenericSearchCommand` (its `RunE` returns the search operation's error unchanged) and `Execute()` (prints `Error: %v` to `os.Stderr` and calls `os.Exit(1)`). Read-only for this prompt.
- `specs/in-progress/051-topic-command-ladder.md` — the owning spec. Read its Goal, Non-goals and Constraints.
- `docs/dod.md` — this repository's `validationPrompt`. Note the Testing section's Ginkgo v2 / Gomega requirement and that `integration/cli_test.go` already carries `os` and `strings` in its import block.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, `Eventually` semantics, `gexec`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — the linter limits and the license header.

## What the error text actually is

`errors.Wrap` from `github.com/bborbe/errors` delegates to `github.com/pkg/errors`' `Wrap`, whose `withMessage.Error()` is `msg + ": " + cause.Error()`. With the lookup failing, the text the CLI prints is therefore exactly:

```
Error: semantic-search-mcp not found on PATH: exec: "semantic-search-mcp": executable file not found in $PATH
```

The assertion below pins the wrapped prefix — `semantic-search-mcp not found on PATH` — not the `exec` package's own wording, which is a Go stdlib string and not this repository's contract. The literal appears twice on stderr (cobra prints it, then `Execute()` prints it again); a `ContainSubstring` check is unaffected by the duplication.

## Environment facts, verified before this prompt was written

1. **`/usr/bin` and `/bin` do not contain the binary.** Checked on the authoring host: `ls /usr/bin/semantic-search-mcp /bin/semantic-search-mcp` reports "No such file or directory" for both, while `which semantic-search-mcp` resolves to a path under the user's local bin directory — the directory the restricted `PATH` below drops. In the container the binary is absent altogether. So `PATH=/usr/bin:/bin` cannot resolve it in either environment.
2. **A bare `append(os.Environ(), "PATH=/usr/bin:/bin")` is not the mechanism to use.** Go's `os/exec` deduplicates duplicate environment keys in favour of the LAST value (the behaviour documented on `Cmd.Env`, implemented by `dedupEnv`), so that form would in fact take effect today. It is rejected anyway: it makes the outcome depend on a deduplication rule rather than on anything this spec states, and it leaves two `PATH` entries in the slice the test hands to the process launcher. The requirement below removes the inherited entry outright, so exactly one `PATH` reaches the child.
3. **The failure is immediate.** Running the built binary with `PATH=/usr/bin:/bin` exits with code 1 in about 20 milliseconds. The one-second `Eventually` default is therefore ample, and widening it would hide a genuine hang rather than fix anything.
4. **`make precommit`'s `make test` target runs this same integration suite.** This prompt's `<verification>` runs the integration suite directly and does not repeat the repo-wide gate: the change touches no production file, so the integration suite is the affected subset of that gate.
5. **`.dark-factory.yaml` sets `GOFLAGS=-buildvcs=false`.** The integration suite's `BeforeSuite` calls `gexec.Build`, which runs `go build`. If it fails with a VCS status error, export `GOFLAGS=-buildvcs=false` rather than touching `.git`.
</context>

<requirements>

## 0. Scope — one file, one spec

The only file this prompt modifies is `integration/cli_test.go`. Exactly one spec changes: the `It("topic search dispatches to the semantic search operation scoped to the topics directory")` block inside `Describe("vault-cli topic command family")`. Its fixture call, the `createTempVaultWithTopicPages` helper, the `Describe("command registration")` table row `Entry("topic search", "topic", "search")`, and every other spec in the file stay byte-identical.

`pkg/ops/search.go`, `pkg/cli/cli.go` and every other production file are read-only. No import is added to `integration/cli_test.go` — `os` and `strings` are already there.

## 1. Rename the spec to what it now proves

Replace the leaf text with exactly:

```
topic search reaches the semantic search operation under a PATH that cannot resolve semantic-search-mcp
```

The new name is the only name for this spec — do not keep the old one alongside it, do not shorten it, and do not reword it. It is grepped verbatim by `<verification>`, and it must state the two things the spec now proves: the dispatch reaches the search operation, and it does so under a controlled `PATH`.

## 2. Build the child environment so `PATH` cannot resolve the binary

Before `gexec.Start`, set the command's environment explicitly. Remove every inherited `PATH` entry, then append one minimal `PATH`:

```go
			// Run the command with an environment whose PATH cannot resolve
			// semantic-search-mcp. The inherited PATH is removed rather than
			// shadowed, so exactly one PATH reaches the child and the lookup fails
			// deterministically on every host — the container and a developer
			// machine alike.
			env := make([]string, 0, len(os.Environ())+1)
			for _, entry := range os.Environ() {
				if strings.HasPrefix(entry, "PATH=") {
					continue
				}
				env = append(env, entry)
			}
			cmd.Env = append(env, "PATH=/usr/bin:/bin")
```

Non-negotiable properties:

- The `PATH` value is exactly `/usr/bin:/bin`. Do not add the user's local bin directory, do not derive the value from the inherited `PATH`, and do not compute it from `exec.LookPath` in the test process — the test process's `PATH` is exactly the environment-dependence being removed.
- The inherited `PATH` entry is dropped by the filter, not left in place. Do NOT write `cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin")`.
- The rest of `os.Environ()` is preserved, so `HOME`, `TMPDIR` and the Go build environment reach the child unchanged.
- The assignment goes immediately after `cmd := exec.Command(...)` and before `gexec.Start(cmd, GinkgoWriter, GinkgoWriter)`. The fixture call and the argument list (`"topic", "search", "attention routing"`) are unchanged.

## 3. Assert the process exits non-zero

Replace `Eventually(session).Should(gexec.Exit())` with:

```go
			Eventually(session).Should(gexec.Exit(1))
```

Non-negotiable properties:

- Keep the `Eventually` call exactly as it is: no `.WithTimeout(...)`, no `.WithPolling(...)`, and no `SetDefaultEventuallyTimeout` anywhere in the file. With the restricted `PATH` the command fails in about 20ms, so the one-second default is ample.
- Use `gexec.Exit(1)` — this is the dominant form among the refusal specs in this `Describe` block, and exit 1 is the CLI's only error exit code (`Execute()` in `pkg/cli/cli.go` prints the error to stderr and calls `os.Exit(1)`). Do NOT use the bare `gexec.Exit()` followed by `Expect(session.ExitCode()).NotTo(Equal(0))` form: with the pinned `gomega v1.43.0`, `gexec.Session.monitorForExit` sets a signalled process's exit code to `128 + int(status.Signal())`, so a `SIGKILL` yields 137 — a non-zero code that satisfies `NotTo(Equal(0))`, so the weaker form would accept a crash as the expected outcome. (The `-1` in gexec is a separate sentinel: `gexec.Exit()` called with no argument matches any exit code.)

## 4. Assert the failure names the missing binary

Add the assertion that proves the invocation reached the search operation rather than stopping at the CLI layer:

```go
			combined := string(session.Out.Contents()) + string(session.Err.Contents())
			Expect(combined).To(ContainSubstring("semantic-search-mcp not found on PATH"))
```

Non-negotiable properties:

- The asserted literal is exactly `semantic-search-mcp not found on PATH` — the message `searchOperation.Execute` wraps around the failed `exec.LookPath`. Do not assert the whole printed line, and do not assert the `exec` package's `executable file not found in $PATH` wording.
- `combined` is stdout concatenated with stderr. `gexec` keeps the two streams in separate buffers and the CLI writes this error to stderr, but the assertion is written against both streams so it encodes the message rather than which stream carries it. Do not replace it with a stderr-only assertion, and do not add an `io.MultiWriter` wiring change to `gexec.Start`.

## 5. Keep the `unknown command` negative assertions

Both of these stay exactly as they are, after the assertions above:

```go
			Expect(string(session.Out.Contents())).NotTo(ContainSubstring("unknown command"))
			Expect(string(session.Err.Contents())).NotTo(ContainSubstring("unknown command"))
```

They are what tells "leaf registered and dispatched" apart from "leaf missing": a `topic search` leaf that was never registered would fail argument parsing and report an unknown command instead of reaching the operation. Do not merge them into one assertion, do not drop either stream, and do not weaken them.

## 6. Replace the comment that reasons about the container

Delete this comment block:

```go
			// The leaf must be registered and reach the search operation. Exit 0 is
			// NOT asserted: semantic-search-mcp may be absent from this container, in
			// which case a non-zero exit naming the missing binary is correct.
```

and replace it with one that states the restricted `PATH` is what makes the spec environment-independent:

```go
			// The child runs with a PATH that cannot resolve semantic-search-mcp, so
			// exec.LookPath fails on every host — the container and a developer
			// machine alike — and this spec no longer depends on where the binary
			// happens to be installed. The failure names the missing binary, which
			// is what proves the invocation reached the search operation rather than
			// stopping at the CLI layer.
```

No wording anywhere in the file may still claim the binary's absence is an environment accident. The phrase `may be absent from this container` must be gone.

## 7. The spec's final shape

The block, in order: the fixture call and `defer cleanup()` unchanged; `cmd := exec.Command(binPath, "--config", configPath, "--vault", "test", "topic", "search", "attention routing")` unchanged; the environment block from requirement 2; `session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)` and `Expect(err).NotTo(HaveOccurred())` unchanged; the comment from requirement 6; `Eventually(session).Should(gexec.Exit(1))`; the `combined` assertion; the two `unknown command` negative assertions.

Do NOT delete the spec, do NOT convert it into a registration-only `Entry(...)`, and do NOT move its assertions into `Describe("command registration")`.

## 8. Self-check before finishing

- Re-read the changed spec and confirm: the leaf text is character-for-character `topic search reaches the semantic search operation under a PATH that cannot resolve semantic-search-mcp`; `PATH=/usr/bin:/bin` is the only `PATH` in the slice handed to `exec.Command`; the exit assertion is `gexec.Exit(1)`; the asserted literal is `semantic-search-mcp not found on PATH`; both `unknown command` assertions survive; the old container comment is gone.
- Confirm `pkg/ops/search.go`, `pkg/cli/cli.go` and every other file outside `integration/cli_test.go` are byte-identical to before this prompt.
- Run every check in `<verification>` and confirm it passes — run them, do not read them.
- Walk this prompt's objective against the change and state in the completion report which requirement and which assertion satisfies each half: the environment-independence half (requirements 2 and 6) and the "still proves the dispatch reaches the operation" half (requirements 3, 4 and 5).

</requirements>

<constraints>
- **Copied from spec 051 — non-goals.** Do NOT add, rename, remove or re-scope any of the twelve `topic` binary leaves. Do NOT change the topic command family's output, flags, exit codes or argument shapes. Do NOT change the goal command family, the goal-side frontmatter allowlists, or the four goal-side files. Do NOT write, backfill, default or normalise a `phase` field onto any page. Do NOT extend the hardcoded entity-kind lists (the watch type list, the watch-directory set, the resolve type set). Do NOT add a `docs/topic-writing.md`. Do NOT ship topic slash-command files.
- **Test-only change.** `pkg/ops/search.go`, `pkg/cli/cli.go`, every other production file, and every test file other than `integration/cli_test.go` are byte-identical after this prompt. This prompt touches `integration/cli_test.go` only — no CHANGELOG entry (this change is not user-visible, and `scripts/check-changelog.sh` validates the file's structure, not the presence of an entry), no README change, no version bump, no `.claude-plugin/` change. **`docs/dod.md`'s CHANGELOG line is deliberately waived for this prompt:** that line states the `## Unreleased` requirement unconditionally, but it targets user-visible changes and this is a test-only edit (the existing topic bullets already sit under `## Unreleased`). Do NOT add a CHANGELOG entry to satisfy `docs/dod.md` — a self-review against the DoD must not reconcile the two by writing one, and no `<verification>` check would catch that.
- **No new dependency and no new import.** `os` and `strings` are already imported by `integration/cli_test.go`. Do not add an import, do not run `go get`, and do not run `go mod tidy`.
- **Do NOT widen the `Eventually` timeout.** No `.WithTimeout(...)`, no `.WithPolling(...)`, no `SetDefaultEventuallyTimeout`. The restricted `PATH` makes the failure immediate; a longer timeout would mask a real hang.
- **Do NOT delete the spec and do NOT convert it to a registration-only `Entry(...)`.** The operator chose PATH-restricted execution specifically so the spec still proves the dispatch reaches the search operation. The registration-only row in `Describe("command registration")` does not replace it.
- **Every existing test must still pass.** Every other spec in `integration/cli_test.go` keeps its fixture and assertions, including `Eventually(session).Should(gexec.Exit())` in the unrelated `task add` spec and the `gexec.Exit(1)` refusal specs in this `Describe` block.
- **Tests.** Ginkgo v2 + Gomega, external test package `integration_test`, `gexec` for process execution. Never pipe a test command — redirect to a file and check the exit status separately.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`.
- Do NOT run `make build`, `make buca`, `docker`, `kubectl`, `gh`, or any `dark-factory` command — including in `<verification>`.
- **Error idiom, if you touch Go code at all:** `errors.Wrap(ctx, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; no `fmt.Errorf`. This prompt adds no production code, so no new error site is expected.
</constraints>

<verification>
Run everything from the repo root, in order. These are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code.

**The integration suite passes, and the renamed spec ran.** `-ginkgo.v` is required — without it Ginkgo prints the dot reporter and the spec-name grep returns nothing. Capture the output and check the exit status separately from the name grep (a failing run still prints the name); never pipe the test command:

```
go test -mod=mod ./integration/... -v -ginkgo.v -count=1 > /tmp/topic-search-path.log 2>&1; test "$?" = "0"
grep -F -q 'topic search reaches the semantic search operation under a PATH that cannot resolve semantic-search-mcp' /tmp/topic-search-path.log
```

If `gexec.Build` fails with a VCS status error, `GOFLAGS=-buildvcs=false` is missing from the environment — the container sets it in `.dark-factory.yaml`; export it rather than touching `.git`.

**The premise of the restricted `PATH` holds in this environment:**

```
! test -e /usr/bin/semantic-search-mcp
! test -e /bin/semantic-search-mcp
```

**The spec was renamed, not duplicated, and its mechanism is the stated one:**

```
! grep -q 'dispatches to the semantic search operation scoped to the topics directory' integration/cli_test.go
test "$(grep -c 'It("topic search reaches the semantic search operation under a PATH that cannot resolve semantic-search-mcp"' integration/cli_test.go)" = "1"
test "$(grep -c 'HasPrefix(entry, "PATH=")' integration/cli_test.go)" = "1"
test "$(grep -c 'PATH=/usr/bin:/bin' integration/cli_test.go)" = "1"
! grep -q 'append(os.Environ(), "PATH=' integration/cli_test.go
test "$(grep -c 'semantic-search-mcp not found on PATH' integration/cli_test.go)" = "1"
```

The `HasPrefix` check is the inherited-entry removal; the `append(os.Environ(), "PATH=…")` absence check is the shadowing form this prompt rejects.

**The execution half of the spec survives — the timeout was not widened and the spec was not deleted:**

```
test "$(grep -c 'unknown command' integration/cli_test.go)" = "2"
test "$(grep -c 'Eventually(session).Should(gexec.Exit())' integration/cli_test.go)" = "1"
! grep -q 'WithTimeout' integration/cli_test.go
! grep -q 'SetDefaultEventuallyTimeout' integration/cli_test.go
! grep -q 'may be absent from this container' integration/cli_test.go
test "$(grep -c 'Entry("topic search", "topic", "search")' integration/cli_test.go)" = "1"
```

The `unknown command` count of 2 is the two negative assertions, both retained. The bare `gexec.Exit()` count of 1 is the unrelated `task add` spec at its own line — this spec's bare form must be gone, and the unrelated one must still be there.

**The touched test file and the read-only production files are formatted, and the production literals the spec depends on are intact:**

```
test -z "$(gofmt -e -l integration/cli_test.go)"
test -z "$(gofmt -e -l pkg/ops/search.go pkg/cli/cli.go)"
test "$(grep -c 'exec.LookPath("semantic-search-mcp")' pkg/ops/search.go)" = "1"
test "$(grep -c 'semantic-search-mcp not found on PATH' pkg/ops/search.go)" = "1"
test "$(grep -c 'os.Exit(1)' pkg/cli/cli.go)" = "1"
```

The `gofmt -l` checks prove the files are *formatted*, not that they are *unchanged* — a well-formed semantic edit would still pass them. The three `grep -c` assertions pin the exact production literals the spec relies on: `exec.LookPath("semantic-search-mcp")` and its wrapped `semantic-search-mcp not found on PATH` message in `pkg/ops/search.go`, and `os.Exit(1)` in `pkg/cli/cli.go`. All three counts are 1 in the current files.

Finally, walk this prompt's objective against the change and state in your completion report which requirement and which assertion satisfies the environment-independence half and which satisfies the "still proves the dispatch reaches the search operation" half.
</verification>
