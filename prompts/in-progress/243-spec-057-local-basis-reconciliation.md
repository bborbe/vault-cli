---
status: approved
spec: [057-bug-topic-defer-local-basis]
created: "2026-09-29T06:19:28Z"
queued: "2026-09-29T06:31:41Z"
---

# Reconcile the topic defer date basis between the suite and the CLI (spec 057, prompt 1 of 2)

<summary>
- The date-basis disagreement between the `topic defer` integration spec and the command it tests is settled on one basis: the user's local calendar date.
- A test run in a timezone whose calendar date differs from the UTC date now passes, instead of failing on a tree with no changes at all.
- The test process no longer forces its own timezone, so a timezone selected for a run governs the test process and the command it launches alike.
- The command's behaviour is unchanged: a relative deferral still lands on the local calendar date plus the requested offset.
- The resolution rule is now stated where it is implemented, so the next reader does not have to derive it from a library's internals.
- An always-on unit case pins the local-basis rule without depending on the host timezone or the hour the run happens at.
- The repaired spec now asserts the date the command actually writes against both candidate bases, so moving the command onto the UTC basis would fail the test rather than silently agree.
- The rest of the integration suite behaves exactly as it did before.
- No new configuration, no new flag and no environment variable is introduced; the determinism comes from a timezone selected per run.
- The repository's daily window during which automated runs cannot start is closed.
</summary>

<objective>
Make the `topic defer` integration spec compute its expected `defer_date` on the same basis the CLI already uses — the process's local calendar date — and remove the suite's parent-process timezone assignment that made the test process disagree with the binary it spawns. The end state: `make precommit` and `make test` are green under a timezone whose local date differs from the UTC date, at any hour, and the CLI's user-visible output is unchanged. This closes spec 057's Desired Behaviors 1-5 and Acceptance Criteria 1-7.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**Why this exists, in one paragraph.** `integration/cli_test.go` computes its expected relative date as `time.Now().UTC().AddDate(0, 0, 7)`, while the CLI resolves the same offset from the process's **local** calendar date: `pkg/ops/topic_defer.go`'s `Execute` reads `now := o.currentDateTime.Now().Time()` from an injected `libtime.CurrentDateTime`, hands it to `parseDeferDate`, which calls `libtime.ToDate(now.AddDate(0, 0, days))`; `libtime.ToDate` takes `value.Date()` — the calendar date in `value`'s own location. The two disagree by exactly one day whenever the local calendar date differs from the UTC calendar date, so `make test` goes red on an unchanged tree. Because `.dark-factory.yaml` sets `preflightCommand: "make precommit"` and preflight failure is terminal, that red suite blocks every dark-factory run for the duration of the window. The test's own guard hid the divergence: `integration/integration_suite_test.go` assigns `time.Local = time.UTC` in the **test process**, which makes the parent agree with the UTC expectation but does not reach the spawned child — and it overrides `TZ` for the parent, which is why pinning `TZ` alone moved only one side.

Read fully (in this order):
- `integration/integration_suite_test.go` — the whole file (it is ~30 lines). `TestIntegration` assigns `time.Local = time.UTC` at its first statement, and that assignment is the only use of the `time` import in the file.
- `integration/cli_test.go` — the `It("topic defer writes defer_date for a relative and an absolute date", ...)` spec (near line 4495). Read the whole spec, including the comment block above `expectedRelative` (near lines 4515-4518) and the absolute-date arm below it. Do **not** read the whole 4500-line file; read `createTempVaultWithTopicPages` near line 27 only if you need the fixture shape (you do not change it).
- `pkg/ops/defer_date_parser.go` — `parseDeferDate` in full, in particular the `+Nd` branch whose body is `t := libtime.ToDate(now.AddDate(0, 0, days)).Time()`. Read `nextWeekday` too, so you can see that its `from.AddDate(0, 0, daysUntil)` is a different expression and must not be touched.
- `pkg/ops/topic_defer.go` — `NewTopicDeferOperation` and `topicDeferOperation.Execute`. **Read only, do not change.** Requirement 4's unit case drives this constructor.
- `pkg/ops/topic_defer_test.go` — the `Describe("TopicDeferOperation", ...)` block: its `var` block (which declares `deferOp` and `dateStr`), its `BeforeEach` (which builds `currentDateTime := libtime.NewCurrentDateTime()` and pins it to `2026-03-25T12:00:00Z`), its `JustBeforeEach` (which calls `deferOp.Execute` and assigns `result, err`), and the `Context("success", ...)` block containing `Context("with relative date +7d", ...)`. Requirement 4 adds a sibling Context there.
- `CHANGELOG.md` — lines 1-14 only, so you can see the shape you must not disturb (there is no `## Unreleased` section at HEAD; prompt 2 of this spec creates it, not this prompt).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, nested `Context` / `BeforeEach` ordering.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — `libtime.CurrentDateTime` injection: never call `time.Now()` in production code; tests pin the clock through `libtime.NewCurrentDateTime()` + `SetNow`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs and in what order.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), so the `git diff --name-only` check in the verification section genuinely runs here. If the repo is ever switched to `hideGit: true` or `workflow: worktree`, that command dies with `fatal: not a git repository` and the daemon does not check verification exit codes for failure — a false-positive pass. Treat the file-content checks as the primary evidence and the git check as secondary.
</context>

<requirements>
1. **Remove the parent-process timezone assignment from the integration suite.** In `integration/integration_suite_test.go`, delete the line `time.Local = time.UTC` (it is the first statement of `TestIntegration`), and delete the now-unused `"time"` import from that file's import block.

   Both halves are required. `time` has exactly one use in that file — the assignment itself (`grep -n 'time\.' integration/integration_suite_test.go` returns that single line) — so removing only the statement leaves an unused import and the package will not compile, failing `make check` and `make test`.

   The result of `TestIntegration` must be exactly:

   ```go
   func TestIntegration(t *testing.T) {
   	format.TruncatedDiff = false
   	RegisterFailHandler(Fail)
   	RunSpecs(t, "Integration Test Suite")
   }
   ```

   with the import block reduced to `testing`, the two Ginkgo/Gomega dot-imports (`. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`), `github.com/onsi/gomega/format` and `github.com/onsi/gomega/gexec`. Leave `BeforeSuite`, `AfterSuite` and the `binPath` variable untouched.

   **Why this is the fix and not the alternative the spec's AC 3 also allows.** After this deletion the test process derives its zone from `TZ` (or from `/etc/localtime` when `TZ` is unset), and the spawned binary inherits the same `TZ` from the parent's environment — so both sides resolve the same zone. The spec's other permitted shape (keep the parent at UTC and set the child's zone to match) is not taken here because spec 057's Verification section asserts the absence of the assignment directly (`! grep -q 'time\.Local' integration/integration_suite_test.go`), and because a child-only pin leaves the parent's zone as an untested second source. Removing the line is the shape the spec's Acceptance Criteria and Verification both measure.

   **Do not touch the other suite files.** `pkg/ops/ops_suite_test.go`, `pkg/cli/cli_suite_test.go`, `pkg/storage/storage_suite_test.go`, `pkg/domain/domain_suite_test.go` and `pkg/config/config_suite_test.go` each carry the same assignment. Spec 057's Non-goals scope this change to "the line that causes this failure" and explicitly exclude making the suite's timezone handling a general convention; those five processes never spawn a timezone-sensitive subprocess, so their assignment is not the bug and stays exactly as it is.

2. **Flip the spec's expectation to the local basis and make it discriminate.** In `integration/cli_test.go`, inside `It("topic defer writes defer_date for a relative and an absolute date", ...)`, replace this region verbatim:

   ```go
   			// The run date is not pinnable in a subprocess, so the expected date is
   			// computed here rather than asserted as a literal. The value is stored as
   			// a quoted YAML string, hence the quotes in the substring.
   			expectedRelative := time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
   			relative, err := os.ReadFile(topicPath)
   			Expect(err).NotTo(HaveOccurred())
   			Expect(string(relative)).To(ContainSubstring(`defer_date: "` + expectedRelative + `"`))
   ```

   with:

   ```go
   			// The run date is not pinnable in a subprocess, so the expected date is
   			// computed here rather than asserted as a literal. The basis is the
   			// process's LOCAL calendar date, which is what the CLI resolves: it reads
   			// the clock through the injected libtime.CurrentDateTime and takes the
   			// calendar date in that time's own location (libtime.ToDate). The parent
   			// and the spawned binary share a zone because the child inherits TZ and
   			// the suite no longer assigns time.Local. The value is stored as a quoted
   			// YAML string, hence the quotes in the substring.
   			expectedRelative := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
   			// The UTC-basis date is the WRONG answer, not an alternative one. When the
   			// two calendar dates differ, this is the assertion that tells a local-basis
   			// resolution apart from a UTC-basis one — so it must fail if the CLI is
   			// ever moved onto the UTC basis.
   			wrongBasisDate := time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
   			relative, err := os.ReadFile(topicPath)
   			Expect(err).NotTo(HaveOccurred())
   			Expect(string(relative)).To(ContainSubstring(`defer_date: "` + expectedRelative + `"`))
   			if wrongBasisDate != expectedRelative {
   				Expect(string(relative)).NotTo(ContainSubstring(`defer_date: "` + wrongBasisDate + `"`))
   			}
   ```

   Three details are load-bearing:
   - The expected-date variable keeps the name `expectedRelative` and its expression contains no `.UTC()`. Spec 057 AC 4 asserts exactly that: `grep -n 'expectedRelative :=' integration/cli_test.go` must return a line whose text does not contain `UTC()`. The UTC-basis candidate is therefore a **separate** variable with a different name (`wrongBasisDate`), never a modification of the `expectedRelative` line.
   - The negative assertion is guarded by `if wrongBasisDate != expectedRelative`. Under a zone where the two dates agree the guard is skipped, which is correct: in that zone the two bases are indistinguishable and the assertion would be vacuous either way. Under a divergent zone the guard runs and is the discrimination spec 057 AC 2 asks for.
   - Leave the absolute-date arm (`"topic defer", "Round Trip", "2027-03-19"` and its `ContainSubstring("2027-03-19")` assertion) and every other spec in the file untouched. `time` remains used in this file (by both new lines), so the import set does not change.

3. **State the basis where it is implemented.** In `pkg/ops/defer_date_parser.go`, add a comment **directly above** this line in `parseDeferDate`'s `+Nd` branch:

   ```go
   		t := libtime.ToDate(now.AddDate(0, 0, days)).Time()
   ```

   The comment must be the immediately preceding line (spec 057 AC 4 asserts it with `grep -n -B1 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go`, which prints exactly one line of leading context) and its text must contain the word `local`. Use:

   ```go
   		// libtime.ToDate takes the calendar date
   		// in now's own location: the basis is the local calendar date.
   ```

   Do not change the expression itself, do not extract a helper, and do not touch the `nextWeekday` function's `from.AddDate(0, 0, daysUntil)` line — `grep -c 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go` is `1` at HEAD and must stay `1`, because the AC's grep relies on there being a single match. This requirement changes no behaviour at all; it records the decision requirements 1 and 2 implement.

4. **Add an always-on unit lock for the local basis.** The integration assertion in requirement 2 only discriminates when the run's zone is divergent, so at most hours of the day it is vacuous — and spec 057's Goal requires the repaired state to hold "under a zone pin, at any hour, with no dependence on wall-clock". This case supplies that independence: it discriminates unconditionally by passing a non-UTC location explicitly rather than reading one from the process. It also locks Desired Behavior 2 directly (a relative offset resolves to the local calendar date plus that offset) and gives Acceptance Criterion 2's regression-lock intent a form that holds at every hour rather than only inside the divergent window.

   In `pkg/ops/topic_defer_test.go`, add this `Context` as a sibling of `Context("with relative date +7d", ...)` inside the `Describe("TopicDeferOperation", ...)`'s `Context("success", ...)` block:

   ```go
   		Context("with a non-UTC local zone", func() {
   			BeforeEach(func() {
   				// 2026-09-28 23:00 -11:00 is 2026-09-29 10:00 UTC: the local calendar
   				// date and the UTC calendar date differ, which is exactly the
   				// divergence this spec repairs. The zone is passed explicitly rather
   				// than read from the process, so this case discriminates at any hour
   				// and under any TZ — including a container with no tzdata, where
   				// time.FixedZone still works because it is pure offset arithmetic.
   				loc := time.FixedZone("Pago_Pago", -11*60*60)
   				zonalNow := libtime.NewCurrentDateTime()
   				zonalNow.SetNow(libtime.NewDateTime(2026, time.September, 28, 23, 0, 0, 0, loc))
   				deferOp = ops.NewTopicDeferOperation(mockTopicStorage, zonalNow)
   				dateStr = "+7d"
   			})

   			It("resolves the offset from the local calendar date, not the UTC one", func() {
   				Expect(err).To(BeNil())
   				Expect(result.Success).To(BeTrue())
   				Expect(result.Message).To(Equal("2026-10-05"))
   				Expect(result.Message).NotTo(Equal("2026-10-06"))
   				Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
   				_, written := mockTopicStorage.WriteTopicArgsForCall(0)
   				Expect(written.DeferDate()).NotTo(BeNil())
   				Expect(written.DeferDate().Time()).To(Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)))
   			})
   		})
   ```

   Why this shape and why these values — do not "correct" any of it:
   - `deferOp` and `dateStr` are declared in the `Describe`'s `var` block and assigned in its `BeforeEach`, so a nested `Context`'s `BeforeEach` overrides them. Ginkgo runs the outer `BeforeEach` first and the inner one after, and the `JustBeforeEach` (which calls `Execute`) runs last, so the override takes effect. No change to the `Describe`'s own `BeforeEach` is needed, and `currentDateTime` is a local variable inside that closure — do not try to reach it.
   - `2026-09-28 23:00 -11:00` plus seven days is `2026-10-05 23:00 -11:00`, whose calendar date is `2026-10-05`; `libtime.ToDate` turns that into midnight UTC on `2026-10-05`. The UTC-basis answer would be `now.UTC()` (`2026-09-29 10:00 UTC`) plus seven days — `2026-10-06`. The `NotTo(Equal("2026-10-06"))` assertion is what makes the case discriminate rather than merely assert; keep it.
   - `isDeferDateInPast` does not refuse this target: `2026-10-05T00:00:00Z` is after `2026-09-28T00:00:00Z`, which is the day-granularity floor it compares against. `Execute` therefore reaches `WriteTopic` and `result.Message` carries the formatted date (`"2026-10-05"`).
   - `libtime.NewDateTime(year int, month stdtime.Month, day, hour, min, sec, nsec int, loc *stdtime.Location) libtime.DateTime` and `CurrentDateTime.SetNow(now libtime.DateTime)` are the signatures in `github.com/bborbe/time v1.27.14`; `libtime` and `time` are both already imported in this file. No import changes are needed.
   - `pkg/ops/ops_suite_test.go` assigns `time.Local = time.UTC` for the whole ops test binary. This case is unaffected by it because the location travels inside the pinned `now` value, not through `time.Local`. Do not remove that assignment — it is out of scope per requirement 1.

5. **Change nothing else.** Specifically:
   - `pkg/ops/defer_date_parser.go` — only the comment from requirement 3. Do not change `parseDeferDate`'s signature, its return values, `isDeferDateInPast`, or `nextWeekday`.
   - `pkg/ops/topic_defer.go`, `pkg/ops/defer.go`, `pkg/ops/goal_defer.go`, `pkg/cli/cli.go`, `pkg/domain/`, `pkg/storage/`, `mocks/` — no changes. No interface or signature changes means no mock regeneration is needed and `make generate` must produce no new file.
   - `.dark-factory.yaml` — unchanged. In particular `preflightCommand: "make precommit"` stays exactly as it is; this change removes the reason preflight fails rather than relaxing the gate.
   - `CHANGELOG.md`, `docs/`, `README.md`, `commands/`, `scenarios/` — none of these are touched by this prompt. The CHANGELOG bullet is prompt 2 of this spec.
   - No `TZ` value is written into the suite, the test, or the CLI, and no new environment variable or flag is introduced. The zone is selected per invocation by whoever runs the test.
   - The residual failure mode spec 057 records — a run that straddles midnight, where the parent and the child read the clock at two different instants in the same zone and so resolve different calendar dates — is **explicitly out of scope**. It is milliseconds wide, it recurs once per day, and the spec's Failure Modes table records it as "not widened by this change, and not newly introduced by it". Do not add a clock seam, a sleep, or a retry to chase it.
   - Do not audit other specs in `integration/` for the same parent-versus-child divergence (spec 057 Non-goals). Verified for you: `grep -rn 'time\.Now()' integration/` returns exactly one line at HEAD, the one requirement 2 repairs, so there is nothing else of this class to find.
   - Do not add a scenario. Spec 057's scenario-coverage note applies the four-condition test and concludes none of the four holds: the repaired behaviour is reachable by the existing `integration/` harness, which needs no Docker, cluster, `gh` or external service. This is a bug fix whose test simply asserted the wrong thing, so the test is fixed and no scenario is written.

6. **Self-check before finishing.** Re-run the verification steps end to end and confirm every count and every printed line against its expectation, including the two absence assertions (`! grep -q 'time\.Local' integration/integration_suite_test.go`, and the `wrongBasisDate` non-containment check) and the printed date pair. Walk spec 057's Acceptance Criteria 1-7 against the change arm by arm:
   - AC 1 — the focused spec exits 0 under the pin, with the two dates printed in the same invocation and shown to differ, and the Ginkgo summary reading `1 Passed | 0 Failed`.
   - AC 2 — the repaired spec asserts the written `defer_date` against both candidate dates, so the assertion distinguishes them.
   - AC 3 — `! grep -q 'time\.Local' integration/integration_suite_test.go` exits 0.
   - AC 4 — the `expectedRelative` line contains no `UTC()`, and the line above `AddDate(0, 0, days)` in the resolver is a comment containing `local`.
   - AC 5 — `TZ=<pin> make precommit` exits 0 with the date pair printed.
   - AC 6 — the full integration suite's pass/fail set is the same under `TZ=UTC` and under `<pin>`.
   - AC 7 — nothing else in the suite changed behaviour; if the two summaries differ by anything other than the repaired spec, stop and report rather than widening the change.
   If any arm is not evidenced, fix it before declaring done.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` and `git diff --name-only` only read; never stage or commit.
- **The CLI's `defer_date` semantics are frozen.** A relative offset resolves from the process's local calendar date, exactly as today. This prompt changes the test and the suite, not the resolver. A user who runs `vault-cli topic defer "<name>" "+7d"` in `Europe/Berlin` gets the same date before and after this change.
- **`libtime.ToDate`'s behaviour is not modified.** It belongs to `github.com/bborbe/time`; its "calendar date in `value`'s own location" semantics are what make the local basis work. Do not vendor, fork, wrap or replace it.
- **No subprocess-visible clock override.** No `VAULT_CLI_NOW`-style environment variable, no `--now` flag, no injectable clock seam for the spawned binary. The zone pin is the determinism mechanism, and spec 057's Non-goals name the clock seam as out of scope.
- **No `TZ` is hard-coded** into the suite, the test, or the CLI. The suite must not pin a zone of its own; the zone is selected per invocation by whoever runs the test.
- **Only `integration/integration_suite_test.go` loses its `time.Local` assignment.** The five other `*_suite_test.go` files keep theirs. Spec 057's Non-goals exclude turning this into a general suite-wide timezone convention.
- **Do not weaken the assertion to make the tree green.** Marking the spec skipped, loosening the substring match to a prefix, or asserting a date range instead of a date all hide the disagreement rather than settling it, and spec 057's Alternatives Considered rejects each of those by name.
- Tests follow repository convention: Ginkgo v2 / Gomega with Counterfeiter mocks. The repaired spec stays where it is; this change does not restructure the suite.
- `.dark-factory.yaml` is unchanged — in particular `preflightCommand: "make precommit"` stays as it is.
- Existing tests must still pass. `make precommit` runs `ensure format generate test check addlicense`; `generate` wipes and regenerates `mocks/`, and `addlicense` re-adds the copyright header, so `mocks/mocks.go` normally ends clean — restore it only if it shows as a diff.
- All paths in this prompt are repo-relative. Never use an absolute path or a `~/` path.
</constraints>

<verification>
**Shell note, read first.** Shell variables do not survive between tool calls, so each block below re-derives the zone pin itself. Paste each block whole rather than reusing a `PIN` from an earlier one.

**Step 0 — select a divergent zone and put the evidence on the record.** The zone must be one whose local calendar date differs from the UTC calendar date at the moment you run. `Etc/GMT-14` (UTC+14) and `Etc/GMT+12` (UTC-12) are complementary: the first diverges whenever UTC time is 10:00 or later, the second whenever UTC time is before 12:00, so together they cover the whole day. The loop skips any candidate that is unavailable or that happens to agree.

```
PIN=""
for z in Etc/GMT-14 Etc/GMT+12 Pacific/Kiritimati Pacific/Pago_Pago; do
  if [ "$(TZ=$z date +%F)" != "$(TZ=UTC date +%F)" ]; then PIN=$z; break; fi
done
printf 'pin=%s\nlocal=%s\nutc=%s\n' "$PIN" "$(TZ=$PIN date +%F)" "$(TZ=UTC date +%F)"
```

WARNING — **the two printed dates must DIFFER.** If they are equal, no zone pin was available in this container (tzdata missing, or every candidate happened to agree) and the pinned runs below are green on the broken tree as well — they prove nothing. In that case say so explicitly in your final message instead of claiming the run passed: step 3's `pkg/ops` case is then the evidence that the local basis is pinned, and it needs no tzdata because `time.FixedZone` is pure offset arithmetic.

**Step 1 — AC 3, the parent no longer overrides the zone (absence, so `! grep -q`, never `grep -c`):**

```
! grep -q 'time\.Local' integration/integration_suite_test.go
```

**Step 2 — AC 4, both source reads:**

```
grep -n 'expectedRelative :=' integration/cli_test.go
```
must print exactly one line and that line's text must NOT contain `UTC()`. (A second line containing `wrongBasisDate` and `.UTC()` is expected and is the deliberate wrong-answer candidate — do not remove it, and do not add a third.)

```
grep -n -B1 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go
```
must print exactly one match, whose immediately preceding line is a comment containing `local`. Also confirm the expression was not refactored away:
```
grep -c 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go    # must print 1
```

**Step 3 — requirement 4's basis lock. This one needs no tzdata and no divergent zone:**

```
go test -mod=mod ./pkg/ops/ -count=1 -run TestSuite -ginkgo.focus="local calendar date"
```
must report `1 Passed | 0 Failed` and exit 0. Run it once more with `TZ=UTC` prefixed to prove it is zone-independent — it must pass identically.

**Step 4 — AC 1, the focused spec under the pin, with the date pair in the same invocation:**

```
PIN=""
for z in Etc/GMT-14 Etc/GMT+12 Pacific/Kiritimati Pacific/Pago_Pago; do
  if [ "$(TZ=$z date +%F)" != "$(TZ=UTC date +%F)" ]; then PIN=$z; break; fi
done
printf 'pin=%s local=%s utc=%s\n' "$PIN" "$(TZ=$PIN date +%F)" "$(TZ=UTC date +%F)"
TZ="$PIN" go test -mod=mod ./integration/ -count=1 -run TestIntegration \
  -ginkgo.focus="topic defer writes defer_date for a relative and an absolute date"
```
must exit 0 and print `1 Passed | 0 Failed` alongside the divergent date pair.

**Step 5 — AC 6 and AC 7, the rest of the suite is unchanged.** Run the full integration suite twice and compare the summaries:

```
PIN=""
for z in Etc/GMT-14 Etc/GMT+12 Pacific/Kiritimati Pacific/Pago_Pago; do
  if [ "$(TZ=$z date +%F)" != "$(TZ=UTC date +%F)" ]; then PIN=$z; break; fi
done
TZ=UTC   go test -mod=mod ./integration/ -count=1 -run TestIntegration > /tmp/utc.log 2>&1 || true
TZ="$PIN" go test -mod=mod ./integration/ -count=1 -run TestIntegration > /tmp/pin.log 2>&1 || true
grep -E 'Ran [0-9]+ of [0-9]+ Specs|Passed \| [0-9]+ Failed|SUCCESS!|FAIL!' /tmp/utc.log | tail -3
grep -E 'Ran [0-9]+ of [0-9]+ Specs|Passed \| [0-9]+ Failed|SUCCESS!|FAIL!' /tmp/pin.log | tail -3
```

The two summary lines must agree: same total spec count, `0 Failed` in both. A difference anywhere other than the repaired spec means removing the assignment changed another spec's result — that is spec 057 Failure Modes row 3 firing, and the recovery is the narrower fix recorded there (set the parent zone explicitly to the child's zone instead of removing the assignment). Do not widen the change to paper over it; report it.

**Step 6 — AC 5, the full gate under the pin (this also re-runs the whole suite, so it is the strongest single signal):**

```
PIN=""
for z in Etc/GMT-14 Etc/GMT+12 Pacific/Kiritimati Pacific/Pago_Pago; do
  if [ "$(TZ=$z date +%F)" != "$(TZ=UTC date +%F)" ]; then PIN=$z; break; fi
done
printf 'pin=%s local=%s utc=%s\n' "$PIN" "$(TZ=$PIN date +%F)" "$(TZ=UTC date +%F)"
TZ="$PIN" make precommit
```
must exit 0 with the divergent date pair printed in the same invocation.

**Step 7 — scope check (secondary; git is available here, see the NOTE in the context section):**

```
git diff --name-only
```
must list exactly `integration/cli_test.go`, `integration/integration_suite_test.go`, `pkg/ops/defer_date_parser.go` and `pkg/ops/topic_defer_test.go`. Any fifth file means scope leaked — fix it rather than reporting it.

If that command instead prints `fatal: not a git repository`, git is masked in this container — do **not** treat the check as passed, and do not report a pass. This repository's own specs record `hideGit: true` having been observed in the container despite `.dark-factory.yaml` saying otherwise, and the daemon does not check verification exit codes for failure, so a dying command reads as a pass. Use this git-free fallback, which asserts the same four-file scope from file content:

```
ls -1 integration/cli_test.go integration/integration_suite_test.go pkg/ops/defer_date_parser.go pkg/ops/topic_defer_test.go   # all four must exist
grep -c 'AddDate(0, 0, days)' pkg/ops/defer_date_parser.go    # 1 — the resolver was not refactored
grep -c 'wrongBasisDate' integration/cli_test.go              # 2 — the repaired spec is in place
```

Before finishing, re-run the steps above and confirm every printed count and line against its stated expectation, then walk spec 057 AC 1-7 arm by arm as described in requirement 6.
</verification>
