---
status: completed
spec: [040-bug-session-start-blocks-on-full-headless-turn]
summary: Replaced the vacuous wall-clock liveness-window spec with wiring+value assertions against the injected waiter, added the test-only LivenessWindow accessor, proved the assertion non-vacuous by mutation, fixed spec 040 AC2/AC7 evidence so both actually execute, and added the CHANGELOG bullet
execution_id: vault-cli-liveness-assert-exec-202-spec-040-liveness-window-assertion
dark-factory-version: dev
created: "2026-08-27T18:30:00Z"
queued: "2026-08-27T18:34:55Z"
started: "2026-08-27T18:37:17Z"
completed: "2026-08-27T18:42:23Z"
---

<summary>
- Replaces a test that only pretends to check the startup-wait duration. It measured elapsed wall-clock time against a hardcoded number while the thing it waits on was faked out to return instantly, so it finished in microseconds and would have passed even if the wait had been retuned to five minutes or to zero.
- The new test captures the duration the code actually hands to its wait helper, and checks it two ways: that it is the named constant rather than a stray number typed at the call site, and that the constant still holds the value the fix was designed around.
- Both checks are needed. On its own the first one moves with the constant — retune the constant and it still passes — so it locks the wiring but not the value. The second line is the one that fails if someone changes the wait duration.
- Adds a small test-only accessor so the external test package can see the otherwise-private constant.
- Also fixes a second check in the spec that passes without running anything, because the test framework in use ignores the flag the check relies on.
- After this, the spec can be re-verified without either of its two false passes.
- No production behaviour changes: the shipped ten-second wait is untouched, and only one of the ten places that inject the wait helper is modified.
</summary>

<objective>
Spec 040's AC2 requires the liveness window to be asserted against the constant with an injected clock, explicitly "not a wall-clock tolerance". The shipped test is a wall-clock tolerance AND is vacuous: it asserts `time.Since(start) < 10*time.Second` while the injected waiter returns `nil` immediately, so it locks nothing. After this prompt both the wiring (`StartSession` hands its waiter the constant, not a literal) and the value (the constant is 10s) are locked by assertions that fail when either changes, and spec 040 AC2 and AC7 carry evidence commands that actually execute.
</objective>

<context>
This repository is `github.com/bborbe/vault-cli`. The behaviour under test already shipped in v0.116.3 and is correct — this prompt changes tests and one spec's evidence commands only. Do not change any production behaviour.

Read fully before editing:

- `/workspace/CLAUDE.md` — the "Never code directly" rule, the Scenario-skip rule, and the release checklist.
- `/workspace/pkg/ops/claude_session.go` — `const livenessWindow = 10 * libtime.Second` at line 41 (unexported). The constructors set the struct field from it (lines 56 and 74), and the non-interactive branch calls `c.waiter.Wait(ctx, c.livenessWindow)` inside a goroutine, racing it against the child's exit in a `select`.
- `/workspace/pkg/ops/claude_session_test.go` — `package ops_test`. The offending spec is `It("returns within the liveness window", ...)` at lines 287-292:

  ```go
  It("returns within the liveness window", func() {
      start := time.Now()
      err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
      Expect(err).To(BeNil())
      Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second))
  })
  ```

  It lives inside `Context("non-interactive branch", func() {` at line 245 (a Ginkgo `Context`, not a `Describe`). The waiter is injected in that block's `JustBeforeEach` as
  `libtime.WaiterDurationFunc(func(_ context.Context, _ libtime.Duration) error { return nil })`
  — the duration argument is discarded as `_`, which is why nothing observes it.

- `/workspace/specs/in-progress/040-bug-session-start-blocks-on-full-headless-turn.md` — AC2 and AC7. Two distinct problems:
  - **AC2 (spec line 121) carries NO evidence command at all** — it is prose only. You must ADD one, not replace one.
  - **AC7 (spec line 122)** has the literal `go test ./pkg/ops/ -run Writeback` — package path FIRST. That command exits 0 while printing "no tests to run": the suite is Ginkgo, and the package's only Go test function is `TestSuite` in `/workspace/pkg/ops/ops_suite_test.go`, so `-run Writeback` matches nothing. The target is `Describe("work-on session write-back")` at `/workspace/pkg/ops/workon_session_writeback_test.go:30`.

Verified facts you can rely on (re-check rather than trust):
- All test files in `/workspace/pkg/ops/` declare `package ops_test`. There is currently no `export_test.go` in that directory.
- `libtime.Second` is a typed `libtime.Duration`, so `livenessWindow` is a `libtime.Duration` — matching both the struct field and the waiter's second parameter. A captured variable therefore needs no conversion.
- There are 10 `WaiterDurationFunc` injections across `claude_session_test.go`, `goal_workon_test.go`, `workon_test.go`, and `workon_session_writeback_test.go`. Every one discards the duration argument. This prompt only changes the one inside `Context("non-interactive branch")` in `claude_session_test.go`; leave the other nine alone unless a compile error forces otherwise.

Read these coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-test-best-practices.md` — Ginkgo/Gomega spec structure and naming.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — bullet format.
</context>

<requirements>
1. Create `/workspace/pkg/ops/export_test.go` declaring `package ops` (NOT `ops_test`) and exporting the constant for tests only:

   ```go
   // Copyright (c) 2025 Benjamin Borbe All rights reserved.
   // Use of this source code is governed by a BSD-style
   // license that can be found in the LICENSE file.

   package ops

   // LivenessWindow exposes the unexported livenessWindow constant to the external
   // ops_test package so tests can assert the duration StartSession hands to its
   // waiter against the real value, rather than against a copied literal. Test-only:
   // this file is a _test.go file and is not part of the package's public API.
   const LivenessWindow = livenessWindow
   ```

   No import is needed — the type is inferred. Keep `livenessWindow` itself unexported in `claude_session.go`: do NOT rename it, and do NOT change its value.

2. Rewrite the offending spec in `/workspace/pkg/ops/claude_session_test.go` so it asserts the duration rather than elapsed time. Inside `Context("non-interactive branch")` (line 245), capture the duration handed to the waiter — add a `var capturedWindow libtime.Duration` alongside that block's other state vars, reset it in the block's `BeforeEach`, and change that block's `JustBeforeEach` waiter injection to record the duration:

   ```go
   libtime.WaiterDurationFunc(func(_ context.Context, d libtime.Duration) error {
       capturedWindow = d
       return nil
   }),
   ```

   Then replace the whole `It("returns within the liveness window", ...)` body. The spec's description MUST be exactly `waits for the liveness window` so the evidence grep is stable, and it MUST carry BOTH assertions:

   ```go
   It("waits for the liveness window", func() {
       err := starter.StartSession(ctx, "session-abc", "prompt", "/my/vault", "", false)
       Expect(err).To(BeNil())
       // Locks the wiring: StartSession hands the constant, not a stray literal.
       Expect(capturedWindow).To(Equal(ops.LivenessWindow))
       // Locks the value: LivenessWindow is an alias for livenessWindow, so the
       // line above moves with the constant and would survive any retune. This
       // line is the one that fails when the window is changed.
       Expect(capturedWindow).To(Equal(10 * libtime.Second))
   })
   ```

   Both lines are required — neither alone is sufficient, and the second is not redundant. The `time.Since` / `time.Now()` pair in that spec MUST be deleted. If removing it leaves the `time` package unused in that file, remove the import; if other specs in the file still use `time`, keep it.

3. Prove the new assertion is not itself vacuous before you finish, by mutating the value and watching the spec fail.

   Temporarily change `livenessWindow` in `claude_session.go` to `30 * libtime.Second`, run the focused spec, and confirm it FAILS — on the `Equal(10 * libtime.Second)` line specifically. Then restore the constant to `10 * libtime.Second` and confirm it passes again. Report both observations, including which assertion failed, in your summary.

   Be aware of why the second assertion exists: `ops.LivenessWindow` is an alias for `livenessWindow`, so the `Equal(ops.LivenessWindow)` line moves with the constant and passes under any value. If your mutation run reports a pass, you have omitted the `Equal(10 * libtime.Second)` line — add it rather than concluding the experiment succeeded. A test that cannot be made to fail is the exact defect this prompt exists to remove; do not skip this step, and do not leave the constant mutated.

4. Fix the evidence in two Acceptance Criteria of `/workspace/specs/in-progress/040-bug-session-start-blocks-on-full-headless-turn.md`. If the daemon has moved the spec, resolve it with `specs/*/040-bug-session-start-blocks-on-full-headless-turn.md` rather than failing on the hardcoded path.
   - **AC2** currently carries no evidence command at all — ADD one proving the new assertions exist:
     `grep -c 'Expect(capturedWindow).To(Equal(ops.LivenessWindow))' pkg/ops/claude_session_test.go` must be 1,
     `grep -c 'Expect(capturedWindow).To(Equal(10 \* libtime.Second))' pkg/ops/claude_session_test.go` must be 1,
     and `grep -c 'time.Since(start)' pkg/ops/claude_session_test.go` must be 0.
     Leave AC2's requirement text unchanged — it was always correct; only the evidence is new.
   - **AC7**: replace the literal `go test ./pkg/ops/ -run Writeback` (package path first, spec line 122) with
     `go test ./pkg/ops/ -ginkgo.focus="work-on session write-back"`.
     The suite's only Go test function is `TestSuite` (`pkg/ops/ops_suite_test.go`), so `-run Writeback` matches nothing; the target Describe is `"work-on session write-back"` (`pkg/ops/workon_session_writeback_test.go:30`). Run the replacement yourself and confirm the output reports a non-zero spec count (`Ran N of …` with N ≥ 1). A command reporting "no tests to run" is not acceptable as evidence.

5. Append one bullet under the existing `## Unreleased` section of `/workspace/CHANGELOG.md` (append; do not replace, do not rename the section):

   ```
   - test: assert the liveness window against `livenessWindow` via the injected waiter instead of a wall-clock tolerance — the previous spec finished in microseconds against a no-op waiter and would have passed for any window value, including zero
   ```

6. Do NOT touch: any production behaviour or signature in `pkg/ops/*.go` (except adding `export_test.go` and, for step 3 only, the temporary constant edit you then restore), any scenario file, any version field, or the other nine `WaiterDurationFunc` injections.
</requirements>

<constraints>
- Tests and spec evidence only. The shipped runtime behaviour of `StartSession` is correct and must end byte-identical to how you found it, once step 3's temporary edit is restored.
- `.git` is NOT mounted in this container (`hideGit=true`). Do not run `git` for anything — not to inspect, diff, or restore. Before step 3's mutation, keep the original line text at hand so you can restore it exactly by editing the file back.
- `export_test.go` must be `package ops`, not `package ops_test`. That is what lets it read the unexported constant while remaining invisible outside tests.
- Do NOT widen the change to the other `WaiterDurationFunc` call sites. One asserted call site is enough; touching four files for one assertion is scope creep.
- `capturedWindow` must be assigned from the waiter's own argument (`capturedWindow = d`). Assigning it from the constant would satisfy the greps while proving nothing.
- This repo is `autoRelease: true`. Append the `## Unreleased` bullet only; do NOT bump a version or create a tag.
- Do NOT commit — dark-factory handles git.
- All existing tests must still pass.
</constraints>

<verification>
Run from `/workspace`.

The new assertions exist, are fed from the waiter argument, and the wall-clock one is gone:

```
grep -c 'Expect(capturedWindow).To(Equal(ops.LivenessWindow))' pkg/ops/claude_session_test.go        # must be 1
grep -c 'Expect(capturedWindow).To(Equal(10 \* libtime.Second))' pkg/ops/claude_session_test.go      # must be 1
grep -c 'capturedWindow = d' pkg/ops/claude_session_test.go                                          # must be 1
grep -c 'time.Since(start)' pkg/ops/claude_session_test.go                                           # must be 0
grep -c 'waits for the liveness window' pkg/ops/claude_session_test.go                               # must be 1
```

The accessor is test-only and in the right package, and step 3's temporary edit is fully restored.

`.git` is NOT available in this container (the daemon runs with `hideGit=true`), so do NOT use `git diff` / `git status` to confirm the restore — those commands fail here. Assert on file content instead:

```
head -20 pkg/ops/export_test.go | grep -c '^package ops$'                          # must be 1
grep -c 'LivenessWindow = livenessWindow' pkg/ops/export_test.go                   # must be 1
grep -c 'const livenessWindow = 10 \* libtime.Second' pkg/ops/claude_session.go    # must be 1
grep -c '30 \* libtime.Second' pkg/ops/claude_session.go                           # must be 0 (mutation reverted)
grep -c 'livenessWindow' pkg/ops/claude_session.go                                 # must be 7 (unchanged shape)
```

The spec's evidence now exists and executes (resolve the spec path with the glob if the daemon moved it):

```
SPEC=$(ls specs/*/040-bug-session-start-blocks-on-full-headless-turn.md)
grep -c 'Expect(capturedWindow).To(Equal(ops.LivenessWindow))' "$SPEC"   # must be 1 (AC2 evidence added)
grep -c 'go test ./pkg/ops/ -run Writeback' "$SPEC"                      # must be 0 (old vacuous command gone)
grep -c 'ginkgo.focus' "$SPEC"                                           # must be 1
go test ./pkg/ops/ -ginkgo.focus="work-on session write-back" 2>&1 | grep -qE 'Ran [1-9][0-9]* of' && echo OK   # must print OK
```

Changelog:

```
grep -c 'assert the liveness window' CHANGELOG.md    # must be 1
```

Then the full gate:

```
make precommit
```

Must exit 0.
</verification>
