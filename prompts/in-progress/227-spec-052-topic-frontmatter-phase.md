---
status: approved
spec: [052-topic-phase-field]
created: "2026-09-21T13:01:16Z"
queued: "2026-09-21T14:27:52Z"
---

# Topic phase field: a validated set, a raw read and an empty-clears rule on the topic frontmatter

<summary>
- A topic page can now carry a lifecycle phase, set through the topic family's existing field-set command — no new command, flag or argument is added.
- Setting one of the four canonical values writes it to the page's frontmatter and it survives a read-write cycle.
- Setting anything else fails with a non-zero exit and an error naming the offending value, and the page is left exactly as it was.
- The refusal is produced by the same validator that defines the four values, so there is no second list of valid values anywhere that could drift out of step.
- Setting the phase to an empty value removes the phase line rather than writing an empty one.
- Reading a topic through the topic show command surfaces the phase in both plain and JSON output when the field is present.
- Topic pages that predate this field keep parsing, showing and accepting unrelated edits with no error and no invented phase value — nothing is backfilled, rewritten or migrated.
- A hand-typed non-canonical value stays readable on display but is rejected on an explicit re-set.
- The goal-side and task-side phase types, their normalizers, and every goal and task command behave exactly as they do today.
- Adds a CHANGELOG entry under the unreleased heading.
</summary>

<objective>
Wire the topic phase field into the existing topic frontmatter so `topic set <page> phase <value>` validates and persists a canonical `TopicPhase`, `topic set <page> phase ""` clears the key, and `topic show <page>` (plain and `--output json`) surfaces the value. Add a typed `Phase()` getter, a `SetPhase(*TopicPhase)` setter and a private `setPhaseField` validator to `TopicFrontmatter`, plus the `phase` cases in `GetField` / `SetField`. No new command, no new ops or CLI code — this rides the topic family's existing generic set/get/show wiring, which is what makes the generic write path's silent acceptance of any string the thing this change fixes: a typo like `phase: banana` would otherwise write and exit 0.
</objective>

<context>
Read `CLAUDE.md` first for project conventions.

Then read these files:

- `pkg/domain/topic_phase.go` — created in prompt 1: `TopicPhase`, `TopicPhaseTodo/Planning/Execution/Done`, `AvailableTopicPhases`, `TopicPhases.Contains`, `TopicPhase.Validate(ctx)`, `TopicPhase.Ptr()`. If this file does NOT exist yet, STOP and report `status: failed` with the message "topic phase type not yet deployed (prompt 1)" — do NOT create it here.
- `pkg/domain/topic_frontmatter.go` — the file you extend. Read the whole file. Note the existing `GetField(key string) string` switch, which already has a `case "phase":` returning `f.GetString("phase")`, and the existing `SetField(ctx, key, value string) error` switch, which has a `case "defer_date":` and a `default:` that stores the raw string — so `phase` currently falls through to `default`. Note the setter idiom: `Set(...)` / `Delete(...)` come from the embedded `FrontmatterMap`, and `GetString(key)` returns `""` for a missing key. Note the existing unexported helper `setDeferDateFromString` — `setPhaseField` you add sits beside it.
- `pkg/domain/goal_frontmatter.go` — the goal-side precedent for the same three pieces. Read `GoalFrontmatter.Phase()`, `GoalFrontmatter.SetPhase(p *GoalPhase)`, `GoalFrontmatter.setPhaseField(ctx, value)`, and the `case "phase":` in its `GetField` and `SetField`. Your three pieces mirror the goal's shape; the one deliberate divergence is the refusal's error construction — see requirement 3.
- `pkg/domain/topic_frontmatter_test.go` — the suite you extend. Note the existing `Describe("GetField phase")` block (four specs asserting the raw on-disk read, including a non-canonical value) and the existing `Describe("phase is never written by the topic wrapper")` block. Those specs must keep passing unchanged; do not delete or weaken them.
- `pkg/domain/topic_phase_test.go` — the enum's suite from prompt 1. READ ONLY.
- `pkg/ops/frontmatter_entity.go` — DO NOT MODIFY; read it for the wiring. `topicSetOperation.Execute` calls `topic.SetField(ctx, key, value)` directly with no allowlist gate and wraps a failure as `set field "<key>"`, so the new `SetField` `case "phase":` is automatically reachable through `topic set`. `entityShowOperation.Execute` iterates the entity's actual `Keys()` and calls `GetField(k)` per key, so a present `phase:` key is automatically surfaced in `--output json` (the `Fields` map) and in plain output once `GetField` handles it. `knownTopicScalarFields` already lists `"phase": true` — that map only guards the list-mutation path and MUST NOT be edited.
- `pkg/ops/frontmatter_entity_test.go` — the ops suite you extend. Read the `Describe("NewTopicSetOperation")` block: it already has a `Context("setting the phase field")` and a `Context("setting defer_date with an invalid date")`, which is the shape to mirror for the refusal.
- `integration/cli_test.go` — the end-to-end suite you extend, and the strongest evidence this spec asks for. Read the helper `createTempVaultWithTopicPages(topicsDir string, topics, goals map[string]string) (vaultPath, configPath string, cleanup func())`, the helper `showPhaseField(configPath, entityType, name string) string` (which fails the spec when `.fields.phase` is absent, so a negative assertion cannot pass vacuously), and the existing topic specs — `It("topic show emits phase at .fields.phase, equal to what goal show emits")`, `It("topic show omits the phase key entirely when the page carries none")`, and `It("topic show and topic set refuse a traversal name and touch nothing outside the topics directory")`. The last one is the exact idiom to mirror: `exec.Command(binPath, "--config", configPath, "--vault", "test", …)`, `gexec.Start(cmd, GinkgoWriter, GinkgoWriter)`, `Eventually(session).Should(gexec.Exit(1))` for a refusal, and `os.ReadFile` for the on-disk state. `binPath` is built once by `integration/integration_suite_test.go`'s `BeforeSuite` via `gexec.Build`, so this suite drives the real binary. The inline `sha256OfFile := func(path string) string { … }` closure used in several specs is the idiom for a byte-identical assertion.
- `CHANGELOG.md` — the top section is `## v0.142.0` and there is no `## Unreleased` yet. The preamble block (`# Changelog` + "All notable changes…" + the SemVer bullets) is frozen and everything you add goes directly below it.
- `docs/development-patterns.md` § "Adding a New Command" and § "Entity Structure" — the Domain → Storage → Ops → CLI recipe. The expected footprint here is Domain-only plus tests.
- `docs/dod.md` — this repo's `validationPrompt`.
- `specs/in-progress/052-topic-phase-field.md` — the spec. Read its Goal, Non-goals, Acceptance Criteria 3–7 and 10, Constraints and Failure Modes. Every requirement below comes from them.

Coding-plugin docs (in-container paths — the host paths do not exist inside the container):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-enum-type-pattern.md` — the enum recipe and § "RULE go-enum-type/validate-against-available-collection (MUST)".
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `github.com/bborbe/errors` wrapping idiom and the `github.com/bborbe/validation` sentinel.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, `DescribeTable` / `Entry`, external `_test` package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-test-types-guide.md` — where unit, integration and end-to-end evidence each belong.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the `## Unreleased` section format, the frozen preamble rule, and the required conventional prefix (`feat:`).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comments start with the declared name.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md` — the done checklist.

**Environment facts that shape this prompt:**

1. **Every check in `<verification>` is git-free.** The daemon's executor does not check `<verification>` exit codes, so a command that dies for an environmental reason still reports a pass. The spec's git-shaped evidence (`git diff` / `git status` in Acceptance Criteria 3, 4 and 8) is reproduced here with file reads, a `sha256` comparison against the pre-command bytes, and greps of the frozen goal-side and task-side anchor signatures. The git forms stay on the spec's `# Verification` § "Operator-executable" rung.
2. **The spec's `./bin/vault-cli topic set …` rung is satisfied by `integration/cli_test.go`.** That suite's `BeforeSuite` builds the binary with `gexec.Build` and every spec drives it as a subprocess, so requirement 7's specs exercise exactly the commands the spec's Verification section names — with assertions on exit codes, stderr wording and on-disk bytes instead of a bare exit status. Do NOT hand-roll a second scratch-vault shell recipe; the integration suite is the home for this evidence.
3. **Never use the installed `vault-cli`.** It answers `unknown command "topic"` until the topic family releases, so it would silently exercise a stale install and prove nothing. `binPath` and the suite's own build are the only binaries this prompt uses.
4. **`make precommit` runs `addlicense`.** Copy the BSD license header verbatim from the file you are editing rather than inventing one.
5. **`pkg/ops` uses counterfeiter mocks.** `pkg/ops/frontmatter_entity_test.go` drives `mocks.TopicStorage`; no interface changes in this prompt, so there is nothing new to generate.

Depends on prompt 1 (`pkg/domain/topic_phase.go`). That prompt lands first.
</context>

<requirements>

## 1. Scope

- `pkg/domain/topic_frontmatter.go` — MODIFIED (three new members plus two switch cases).
- `pkg/domain/topic_frontmatter_test.go` — MODIFIED (new specs; existing specs unchanged).
- `pkg/ops/frontmatter_entity_test.go` — MODIFIED (new contexts inside `Describe("NewTopicSetOperation")`).
- `integration/cli_test.go` — MODIFIED (new specs inside the existing topic `Describe`).
- `CHANGELOG.md` — MODIFIED (one `## Unreleased` section).

Nothing else. In particular `pkg/domain/goal_phase.go`, `pkg/domain/task_phase.go`, `pkg/domain/goal_frontmatter.go`, `pkg/domain/task_frontmatter.go`, `pkg/domain/topic.go`, `pkg/domain/topic_phase.go`, `pkg/ops/frontmatter_entity.go`, `pkg/ops/lint.go`, `pkg/storage/**`, `pkg/cli/**`, `integration/integration_suite_test.go`, `integration/watch_test.go`, `main.go`, `go.mod`, `go.sum`, `README.md`, `docs/**` and `scenarios/**` are untouched. No new dependency.

## 2. `pkg/domain/topic_frontmatter.go` — the typed getter and setter

Add these two methods beside the existing `DeferDate` / `SetDeferDate` pair. They mirror `GoalFrontmatter.Phase()` and `GoalFrontmatter.SetPhase` exactly, with `Goal` → `Topic`.

```go
// Phase reads "phase" key as string, returns *TopicPhase.
// Returns nil when the key is absent. The raw value is returned as-is
// (no validation, no default substitution) so legacy/hand-typed values survive display.
func (f TopicFrontmatter) Phase() *TopicPhase {
	raw := f.GetString("phase")
	if raw == "" {
		return nil
	}
	p := TopicPhase(raw)
	return &p
}

// SetPhase stores the phase pointer in the map. Deletes the key if p is nil.
func (f *TopicFrontmatter) SetPhase(p *TopicPhase) {
	if p == nil {
		f.Delete("phase")
		return
	}
	f.Set("phase", string(*p))
}
```

- `Phase` is a value receiver; `SetPhase` is a pointer receiver — the same split every other accessor pair in this file uses.
- `Phase()` performs no validation and substitutes no default. A page carrying `phase: whatever-the-vault-holds` or the task-only `phase: in_progress` still reads back verbatim, so `topic show` displays it rather than refusing the page. This is the spec's Failure Modes row "Topic page has a legacy/hand-typed `phase: in_progress`".
- `Phase()` returning `nil` on an absent key is what keeps the key out of `Keys()`, which is what makes `topic show --output json` report no `phase` member for a page that has none. Do NOT substitute `TopicPhaseTodo` for a missing phase.

## 3. `pkg/domain/topic_frontmatter.go` — the validating write helper

Add this unexported helper beside `setDeferDateFromString`:

```go
// setPhaseField validates the value against the topic phase enum and stores it,
// or clears the key on empty. Topic phases have no aliases — a non-canonical value is
// rejected. The rejection is the validator's own error, so the canonical set and the
// refusal's wording are defined in exactly one place and are not restated here.
func (f *TopicFrontmatter) setPhaseField(ctx context.Context, value string) error {
	if value == "" {
		f.SetPhase(nil)
		return nil
	}
	phase := TopicPhase(value)
	if err := phase.Validate(ctx); err != nil {
		return errors.Wrap(ctx, err, "invalid topic phase")
	}
	f.SetPhase(&phase)
	return nil
}
```

Non-negotiable properties, each traceable to the spec:

- **The refusal is the validator's error, propagated.** `errors.Wrap(ctx, err, "invalid topic phase")` keeps `TopicPhase.Validate`'s own message (`unknown topic phase 'bogus'`) in the chain, so `topic set <page> phase bogus` reaches stderr with the spec's Acceptance Criterion 4 wording verbatim. Do NOT write `errors.Wrapf(ctx, validation.Error, "unknown topic phase '%s'", value)` the way the goal-side helper does — that rebuilds the validator's message in a second place, which is exactly what the spec's Desired Behavior 4 ("produced by the same validator that defines the enum") and its Constraints rule out. Do NOT re-list the four canonical strings anywhere in this file.
- **Validation happens before any write.** The `f.SetPhase(&phase)` call is the only mutation on the success path, so a rejected value leaves the map — and therefore the page — untouched.
- **Empty is not invalid.** `value == ""` clears the key and returns nil without ever calling `Validate`. This is the spec's Desired Behavior 5: `topic set <page> phase ""` removes the `phase:` line rather than writing an empty one.
- **No new import.** `context` and `github.com/bborbe/errors` are already imported by this file. Do NOT add `github.com/bborbe/validation` or `github.com/bborbe/collection` — neither is referenced here.
- Do NOT call `context.Background()`. Use the `ctx` you are handed.

## 4. `pkg/domain/topic_frontmatter.go` — the two switch cases

In `TopicFrontmatter.GetField`, replace the existing `case "phase":` body so the read routes through the typed accessor (the goal-side `GetField` does the same). Old → new:

```go
	case "phase":
		// Raw on-disk read: no validation, no default, no normalisation.
		// A topic page with no `phase` line has no key in the map, so this
		// returns "" and the key stays absent from Keys().
		return f.GetString("phase")
```

```go
	case "phase":
		// Raw on-disk read: no validation, no default, no normalisation.
		// A topic page with no `phase` line has no key in the map, so Phase()
		// returns nil, this returns "", and the key stays absent from Keys().
		ph := f.Phase()
		if ph == nil {
			return ""
		}
		return string(*ph)
```

This is behaviour-preserving for every existing spec in `topic_frontmatter_test.go` — a present value (canonical or not) still reads back verbatim, and an absent key still reads as `""` with the key absent from `Keys()`. It makes the typed accessor the single read path so `Phase()` is not a dead method.

In `TopicFrontmatter.SetField`, add a `case "phase":` above the existing `default:`:

```go
	case "phase":
		return f.setPhaseField(ctx, value)
```

Do NOT add a `case` for any other key. In particular do NOT add `status`, `assignee`, `page_type`, `completed` or `claude_session_id` cases — the `default` branch already handles them and an extra case is dead code.

## 5. `pkg/domain/topic_frontmatter_test.go` — the frontmatter specs

External test package `package domain_test`, same imports and structure as the file already has (`BeforeEach` setting `ctx = context.Background()` and `fm = domain.NewTopicFrontmatter(nil)`). Add the blocks below; leave every existing block and spec in place and passing. **The spec names below are mandatory and must be used verbatim** — they are deliberately distinct from the goal suite's near-identical names (`returns nil for missing key`, `stores the phase string via pointer`, `nil pointer deletes the key`, `accepts canonical values`), so that a grep on the shared `pkg/domain` Ginkgo run distinguishes this suite's specs from theirs. They are grep targets in `<verification>`.

### 5a. `Describe("Phase")` — the typed read

- `It("returns nil for a topic with no phase line")` — `Expect(domain.NewTopicFrontmatter(nil).Phase()).To(BeNil())`, and separately on `domain.NewTopicFrontmatter(map[string]any{"status": "in_progress"})`.
- `It("returns a pointer for a canonical topic phase value")` — `fm := domain.NewTopicFrontmatter(map[string]any{"phase": "execution"})`; `Expect(fm.Phase()).NotTo(BeNil())`; `Expect(*fm.Phase()).To(Equal(domain.TopicPhaseExecution))`.
- `It("returns a pointer for a non-canonical on-disk value without rejecting the page")` — `fm := domain.NewTopicFrontmatter(map[string]any{"phase": "in_progress"})`; `Expect(fm.Phase()).NotTo(BeNil())`; `Expect(*fm.Phase()).To(Equal(domain.TopicPhase("in_progress")))`. This is the read half of the spec's legacy-value Failure Modes row: a task-only value on disk must not make the page unreadable.

### 5b. `Describe("SetPhase")` — the raw setter

- `It("stores the topic phase string via pointer")` — `fm.SetPhase(domain.TopicPhaseDone.Ptr())`; `Expect(fm.GetField("phase")).To(Equal("done"))`.
- `It("a nil topic phase pointer deletes the key")` — seed `map[string]any{"phase": "todo"}`, call `fm.SetPhase(nil)`; `Expect(fm.GetField("phase")).To(Equal(""))`; `Expect(fm.Keys()).NotTo(ContainElement("phase"))`.

### 5c. `Describe("SetField phase")` — the validating write path

This block crosses the boundary the new code introduces: `SetField(ctx, "phase", …)` is the real entry point `topic set` drives, and `TopicPhase.Validate` is the only runtime guard on the underlying string.

- `DescribeTable("accepts each canonical topic phase value", ...)` with one `Entry` per canonical value (`todo`, `planning`, `execution`, `done`): the table body builds a fresh `fm := domain.NewTopicFrontmatter(nil)` and asserts `Expect(fm.SetField(ctx, "phase", value)).To(Succeed())` **and** `Expect(fm.GetField("phase")).To(Equal(value))`.
- `It("rejects a non-canonical value with the validator's wording")` — `err := fm.SetField(ctx, "phase", "bogus")`; `Expect(err).NotTo(BeNil())`; `Expect(err.Error()).To(ContainSubstring("unknown topic phase 'bogus'"))` (assert the exact wording **including the quotes**, not merely a substring naming the phase — a looser assertion would pass against a command layer that re-lists the four strings inline); `Expect(fm.GetField("phase")).To(Equal(""))` and `Expect(fm.Keys()).NotTo(ContainElement("phase"))` (the rejection wrote nothing).
- `It("rejects the task-only phase in_progress")` — `fm.SetField(ctx, "phase", "in_progress")` returns a non-nil error whose message contains `"unknown topic phase"`. The topic enum has no aliases, so a re-set must reject what a read tolerates.
- `It("clears the phase key on an empty value")` — seed `map[string]any{"phase": "execution"}`; `Expect(fm.SetField(ctx, "phase", "")).To(Succeed())`; `Expect(fm.GetField("phase")).To(Equal(""))`; `Expect(fm.Keys()).NotTo(ContainElement("phase"))`. The `Keys()` assertion is what distinguishes "cleared" from "present and empty" — assert it, do not rely on the string comparison alone.

### 5d. Existing blocks stay green

Do NOT modify or remove `Describe("GetField phase")` or `Describe("phase is never written by the topic wrapper")`. They already pin the raw-read contract and the no-injected-phase contract; they must pass unchanged. If one of them fails after your change, the change is wrong — fix the source, not the spec.

## 6. `pkg/ops/frontmatter_entity_test.go` — the refusal at the ops boundary

Inside the existing `Describe("NewTopicSetOperation")` block (which drives `ops.NewTopicSetOperation(mocks.TopicStorage)`), add two contexts. The ops layer is what `pkg/cli`'s `topic set` command calls, so a test here is a cheap unit-level guard that the refusal happens before the write.

- `Context("setting each canonical phase value")` containing a `DescribeTableSubtree` over `todo`, `planning`, `execution`, `done`. **Do NOT use a plain `DescribeTable` here** — a table entry body is the `It` node, so the block's existing `JustBeforeEach` (which calls `setOp.Execute` with the Describe-scoped `key`/`value`) runs *before* the entry body and would execute with the outer default `key = "status"`, `value = "in_progress"` instead of the row's value. `DescribeTableSubtree` registers a per-entry `BeforeEach` instead, which the `JustBeforeEach` does observe. Shape:

  ```go
  Context("setting each canonical phase value", func() {
      DescribeTableSubtree("accepts each canonical topic phase value",
          func(canonical string) {
              BeforeEach(func() {
                  key = "phase"
                  value = canonical
              })
              It("writes the canonical phase", func() {
                  Expect(err).To(BeNil())
                  Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(1))
                  _, written := mockTopicStorage.WriteTopicArgsForCall(0)
                  Expect(written.GetField("phase")).To(Equal(canonical))
              })
          },
          Entry("todo", "todo"),
          Entry("planning", "planning"),
          Entry("execution", "execution"),
          Entry("done", "done"),
      )
  })
  ```

  `DescribeTableSubtree` needs Ginkgo v2.16+; this repo pins `github.com/onsi/ginkgo/v2 v2.32.1`, so it is available. Do NOT copy `pkg/ops/blocked_by_set_test.go:46–66` here — that block solves the same ordering hazard a *different* way, because its `Describe` has no `JustBeforeEach`: it calls `setOp.Execute` directly inside each entry body via a `setValue` closure. `Describe("NewTopicSetOperation")` DOES have a `JustBeforeEach` (`pkg/ops/frontmatter_entity_test.go:1135`), so an in-body `Execute` would run the set twice and break every `WriteTopicCallCount()` assertion in the block. Use the `DescribeTableSubtree` shape below.
- `Context("setting an invalid phase value")` — **every value must be assigned in a `BeforeEach`, never in the `It` body**, because the block's `JustBeforeEach` runs the set before the `It` body executes (see `pkg/ops/blocked_by_set_test.go:46–48`). Two nested contexts:
  - `Context("a non-canonical value")` with `BeforeEach(func() { key = "phase"; value = "bogus" })` containing `It("rejects a non-canonical value and does not write")` — `Expect(err).To(MatchError(ContainSubstring("unknown topic phase 'bogus'")))` and `Expect(mockTopicStorage.WriteTopicCallCount()).To(Equal(0))`. The zero write count is the spec's "leaves the page unchanged" property at the ops boundary.
  - `Context("the task-only phase in_progress")` with `BeforeEach(func() { key = "phase"; value = "in_progress" })` containing `It("rejects the task-only phase in_progress")` — the error message contains `"unknown topic phase"`, and `WriteTopicCallCount()` is `0`.

Leave the existing `Context("setting the phase field")`, `Context("setting defer_date …")` and every other context in that Describe unchanged — the existing phase context uses `planning`, which is canonical, so it still passes.

## 7. `integration/cli_test.go` — the end-to-end evidence

Add these specs inside the same `Describe` block that already holds `It("topic show emits phase at .fields.phase, equal to what goal show emits")`. Reuse the existing helpers — `createTempVaultWithTopicPages`, `showPhaseField`, `binPath`, `GinkgoWriter` — and mirror the idiom in `It("topic show and topic set refuse a traversal name and touch nothing outside the topics directory")`: `exec.Command(binPath, "--config", configPath, "--vault", "test", …)`, `gexec.Start(cmd, GinkgoWriter, GinkgoWriter)`, `Eventually(session).Should(gexec.Exit(N))`, and `os.ReadFile` for on-disk state. Assert against the file on disk, not against the command's own success message — that is what makes these state-transition tests.

**The spec names below are mandatory and must be used verbatim**; they are grep targets in `<verification>`.

1. `It("topic set writes a canonical phase to the page on disk and topic show surfaces it")` — a vault with a `Topics/` dir holding one page (`status: in_progress`, no `phase:` line). Run `topic set <name> phase execution` → `gexec.Exit(0)`; read `filepath.Join(vaultPath, "Topics", <name>+".md")` and assert it contains a `phase: execution` line; then assert `showPhaseField(configPath, "topic", <name>)` equals `"execution"`. This is the spec's Acceptance Criterion 3, evidenced by a file read rather than by the command's exit code alone.
2. `It("topic set refuses a non-canonical phase with the validator's wording and leaves the page byte-identical")` — the same page, captured with an inline `sha256OfFile` closure before the command (mirror the closure used elsewhere in this file). Run `topic set <name> phase bogus` → `Eventually(session).Should(gexec.Exit(1))`; assert `string(session.Err.Contents())` contains `unknown topic phase 'bogus'` — the validator's own wording, with the quotes; assert the page's sha256 is unchanged. This is the spec's Acceptance Criterion 4 and its falsifier: the generic write path would exit 0 here and would not produce that wording.
3. `It("topic set with an empty phase value removes the phase line from the page")` — a page seeded with `phase: execution`. Run `topic set <name> phase ""` → `gexec.Exit(0)`; read the file and assert it no longer contains a `phase:` line; run `topic show <name> --output json` and assert the parsed `fields` object has no `phase` key, anchored on a key that IS present (e.g. `status`) so the negative check cannot pass on empty output. This is the spec's Acceptance Criterion 5, a state transition rather than a string comparison.
4. `It("topic set on an unrelated key leaves a page with no phase line without inventing a phase")` — a page with `status: backlog` and no `phase:` line. Run `topic set <name> assignee alice` → `gexec.Exit(0)`; read the file and assert it still has no `phase:` line. This is the spec's Acceptance Criterion 7 and its lazy-migration guarantee.
5. `It("topic lint reports no phase mismatch for a consistent pair and one for an inconsistent pair")` — a page with `phase: execution` and `status: in_progress`: run `topic lint` → `gexec.Exit(0)` and assert the output contains no `STATUS_PHASE_MISMATCH`. Then run `topic set <name> phase done` (leaving `status: in_progress`) and `topic lint` again → `gexec.Exit(1)` and the output contains `STATUS_PHASE_MISMATCH` and names the page's file. The second half is what shows the check fires rather than merely being silent. This is the spec's Acceptance Criterion 6.

Do NOT add a lint test in `pkg/ops` — the lint rule is unchanged and this end-to-end spec is where the spec asks for the evidence.

## 8. `CHANGELOG.md` — the `## Unreleased` entry

Insert a `## Unreleased` section directly below the frozen preamble block (the `# Changelog` title, the "All notable changes to this project will be documented in this file." line, the SemVer link and the MAJOR/MINOR/PATCH bullets) and directly above the current `## v0.142.0` heading. If a `## Unreleased` section already exists, append to it instead of creating a second one. Never insert anything above or inside the preamble — `make check-changelog` guards that structure.

The section carries a single flat bullet starting with the `feat:` prefix and containing the literal phrase **`topic phase`** (the spec's Acceptance Criterion 10 walks the file with `awk '/^## /{sec=$0} /topic phase/{print sec}'` and requires it to resolve to `## Unreleased`):

```
## Unreleased

- feat: topics carry a validated `phase` frontmatter field — `todo`, `planning`, `execution` or `done` — set through `vault-cli topic set <page> phase <value>` and surfaced by `topic show` in plain and `--output json` output. The topic phase is its own type, member-for-member identical in shape to the goal phase and sharing none of it; a non-canonical value is refused before any write with the validator's own wording, an empty value clears the `phase:` line, and a topic page that predates the field parses, shows and mutates unchanged with no phase value invented and no file backfilled. The goal and task phase types, their constants, their normalizers and their commands are untouched.
```

No `### Added` / `### Fixed` sub-headings, no second bullet, no date suffix.

## 9. Do not touch the lint layer

The existing generic `detectStatusPhaseMismatch` in `pkg/ops/lint.go` keys off the presence of a `phase:` line and will begin evaluating topics that carry one — that is pre-existing behaviour and needs no code change. Do NOT add or extend any topic-specific status/phase mismatch lint rule, do NOT edit `pkg/ops/lint.go`, and do NOT add a unit-level lint test. Requirement 7's end-to-end spec is the evidence. Concurrent `topic set` on the same page keeps the existing whole-file last-writer-wins semantics: add no locking, no atomic-write change and no concurrency test — the spec's Failure Modes row 6 expects exactly today's behaviour.

## 10. Self-check before finishing

- Re-read `pkg/domain/topic_frontmatter.go` and confirm: `Phase()` returns nil on an absent key and validates nothing; `SetPhase(nil)` deletes the key; `setPhaseField` clears on empty, propagates the validator's error, and mutates only after validation passes; both switch cases are present; no second list of the four canonical strings exists in the file.
- Walk spec 052's Acceptance Criteria 3, 4, 5, 6, 7 and 10 and state in your completion report which requirement and which spec satisfies each, and which evidence covers each row of the spec's Failure Modes table.
- Walk `docs/dod.md`: every exported method has a doc comment starting with its name; no `fmt.Print*` and no `os.Stdout`; errors use `github.com/bborbe/errors` with the caller's `ctx`; tests are Ginkgo v2 / Gomega in external test packages; the CHANGELOG entry sits under `## Unreleased`.
- Confirm each check in `<verification>` passes by **running** it, not by reading it.

</requirements>

<constraints>
- **Copied from spec 052 — non-goals (hard vetoes).** Do NOT add `plan-topic`, `execute-topic` or any phase-transition or gating command — those are separate work that consumes this field. Do NOT build or extend the `vault-cli topic` command family itself; phase rides the existing `topic set` / `topic get` / `topic show` commands. Do NOT backfill, rewrite or migrate existing topic pages that lack a phase. Do NOT modify, extend or reuse the goal-side `GoalPhase` type, the task-side `TaskPhase` type, their constants, or `NormalizeTaskPhase`. Do NOT add `in_progress`, `ai_review` or `human_review` to the topic enum. Do NOT add alias handling (no `in_progress` synonym) — the topic phase enum has no legacy values. Do NOT invent a "default to todo" read behavior — a missing phase is empty, not `todo`. Do NOT add or extend topic-specific status/phase mismatch lint rules. Do NOT add `docs/topic-writing.md` or any other doc-only change; the lifecycle semantics this spec fixes belong in the vault's `Topic Writing Guide.md`, owned by whoever maintains that page.
- **Copied from spec 052 — constraints.** The goal-side phase type, the task-side phase type, `NormalizeTaskPhase`, and every goal and task command must be byte-for-byte unchanged; their test suites are frozen. Frontmatter remains map-based and unknown keys must continue to survive read-write cycles — that map is the lazy-migration mechanism, and no separate migration code is added. The footprint follows `docs/development-patterns.md` § "Adding a New Command" (Domain → Storage → Ops → CLI) and is Domain-only: Storage, Ops and CLI reuse the topic family's existing generic set/get/show wiring and require no new command. The existing generic `status/phase mismatch` lint keys off the presence of a `phase:` line and will begin evaluating topics that carry a phase — legacy topics with no phase must remain lint-clean, and this work must not add false-positive lint output for the four canonical phases on an otherwise-consistent topic. The enum's canonical set is defined once, as `AvailableTopicPhases`; `Validate` ranges over it via `collection.Contains`. Empty is not invalid: the field-set path clears the key on an empty value and validates only non-empty input. `make precommit` must pass in the repo root. A `## Unreleased` CHANGELOG entry describing the new topic phase field is required.
- **Frozen names.** Methods `Phase() *TopicPhase`, `SetPhase(p *TopicPhase)`, `setPhaseField(ctx context.Context, value string) error`; the `case "phase":` branch in both `GetField` and `SetField`; files `pkg/domain/topic_frontmatter.go`, `pkg/domain/topic_frontmatter_test.go`, `pkg/ops/frontmatter_entity_test.go`, `integration/cli_test.go`, `CHANGELOG.md`. All are grep targets in `<verification>`.
- **The refusal's wording comes from the validator, not from this file.** `TopicPhase.Validate` owns the string `unknown topic phase '%s'`; `setPhaseField` propagates that error. Do NOT re-declare the four canonical strings, a validity list, or the message format anywhere in `pkg/domain/topic_frontmatter.go` — not even in a comment.
- **Error wrapping.** Use `github.com/bborbe/errors` with the caller's `ctx`; never `fmt.Errorf`; never `context.Background()` in non-test code. Do NOT add the `github.com/bborbe/validation` or `github.com/bborbe/collection` imports to `pkg/domain/topic_frontmatter.go` — neither is referenced.
- **Tests.** Ginkgo v2 / Gomega, external test packages (`package domain_test`, `package ops`, `package integration_test`), no stdlib `t.Run` table tests. Every named `It` must contain a real assertion. Every Go file keeps its BSD license header.
- **Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- Do NOT run `make build`, `make install`, `docker`, `kubectl`, `gh`, or any `dark-factory` command. The integration suite builds the binary itself; do not hand-build one.
- No new dependency. `go.mod` and `go.sum` are untouched.
- Existing tests must still pass, and every goal and task command must behave identically to before.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite (which includes `./integration`), `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

Then each check below must pass. They are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, and the daemon does not check `<verification>` exit codes, so a comment-only form would report success on unchanged code. Absence is asserted with `! grep -q`, never with a `grep -c` that must print `0`. Never pipe a test command.

**The three test runs, with their frozen spec names.** Check each run's exit status separately from the name greps (a failing run still prints the names):

```
go test ./pkg/domain/... -v -ginkgo.v -count=1 > /tmp/topic-phase-fm-domain.log 2>&1; test "$?" = "0"
grep -F -q -- 'returns nil for a topic with no phase line' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'returns a pointer for a canonical topic phase value' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'returns a pointer for a non-canonical on-disk value without rejecting the page' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'stores the topic phase string via pointer' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'a nil topic phase pointer deletes the key' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'accepts each canonical topic phase value' /tmp/topic-phase-fm-domain.log
grep -F -q -- "rejects a non-canonical value with the validator's wording" /tmp/topic-phase-fm-domain.log
grep -F -q -- 'rejects the task-only phase in_progress' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'clears the phase key on an empty value' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'returns the raw on-disk phase string verbatim' /tmp/topic-phase-fm-domain.log
grep -F -q -- 'does not inject phase on an unrelated mutation' /tmp/topic-phase-fm-domain.log
go test ./pkg/ops/... -v -ginkgo.v -count=1 > /tmp/topic-phase-fm-ops.log 2>&1; test "$?" = "0"
grep -F -q -- 'setting each canonical phase value' /tmp/topic-phase-fm-ops.log
grep -F -q -- 'rejects a non-canonical value and does not write' /tmp/topic-phase-fm-ops.log
grep -F -q -- 'stores the value verbatim' /tmp/topic-phase-fm-ops.log
go test ./integration/... -v -ginkgo.v -count=1 > /tmp/topic-phase-fm-int.log 2>&1; test "$?" = "0"
grep -F -q -- 'topic set writes a canonical phase to the page on disk and topic show surfaces it' /tmp/topic-phase-fm-int.log
grep -F -q -- "topic set refuses a non-canonical phase with the validator's wording and leaves the page byte-identical" /tmp/topic-phase-fm-int.log
grep -F -q -- 'topic set with an empty phase value removes the phase line from the page' /tmp/topic-phase-fm-int.log
grep -F -q -- 'topic set on an unrelated key leaves a page with no phase line without inventing a phase' /tmp/topic-phase-fm-int.log
grep -F -q -- 'topic lint reports no phase mismatch for a consistent pair and one for an inconsistent pair' /tmp/topic-phase-fm-int.log
grep -F -q -- 'topic show emits phase at .fields.phase, equal to what goal show emits' /tmp/topic-phase-fm-int.log
grep -F -q -- 'topic show omits the phase key entirely when the page carries none' /tmp/topic-phase-fm-int.log
```

**The frontmatter wiring carries the frozen names:**

```
test "$(grep -c 'func (f TopicFrontmatter) Phase() \*TopicPhase' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TopicFrontmatter) SetPhase(p \*TopicPhase)' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TopicFrontmatter) setPhaseField(ctx context.Context, value string) error' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'case "phase":' pkg/domain/topic_frontmatter.go)" = "2"
test "$(grep -c 'return f.setPhaseField(ctx, value)' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'ph := f.Phase()' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'f.SetPhase(nil)' pkg/domain/topic_frontmatter.go)" = "1"
test "$(grep -c 'f.SetPhase(&phase)' pkg/domain/topic_frontmatter.go)" = "1"
```

**The refusal is the validator's error and there is no second list of canonical values in the wrapper — the spec's Desired Behavior 4 and its Constraints:**

```
test "$(grep -c "unknown topic phase" pkg/domain/topic_frontmatter.go)" = "0"
test "$(grep -c 'errors.Wrap(ctx, err, "invalid topic phase")' pkg/domain/topic_frontmatter.go)" = "1"
! grep -qE '"todo"|"planning"|"execution"|"done"' pkg/domain/topic_frontmatter.go
! grep -q 'AvailableTopicPhases' pkg/domain/topic_frontmatter.go
! grep -qE 'in_progress|ai_review|human_review|Normalize' pkg/domain/topic_frontmatter.go
! grep -q 'bborbe/validation' pkg/domain/topic_frontmatter.go
! grep -q 'bborbe/collection' pkg/domain/topic_frontmatter.go
```

**The ops layer, the lint layer and the suite bootstrap are untouched — the spec's Constraints:**

```
test "$(grep -c 'func (o \*topicSetOperation) Execute' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'case \*domain.Topic:' pkg/ops/frontmatter_entity.go)" = "1"
test "$(grep -c 'func (l \*lintOperation) detectStatusPhaseMismatch(frontmatterYAML string) (bool, string)' pkg/ops/lint.go)" = "1"
test "$(grep -c 'IssueTypeStatusPhaseMismatch' pkg/ops/lint.go)" = "2"
test "$(grep -c 'gexec.Build("github.com/bborbe/vault-cli")' integration/integration_suite_test.go)" = "1"
```

**The goal-side and task-side phase anchors are intact and unmodified** — the non-git equivalent of spec Acceptance Criterion 8. Each signature must still be present exactly once; if any is not `1`, a frozen file was edited and the edit must be reverted rather than the assertion adjusted:

```
test "$(grep -c 'func (g GoalPhase) Validate(ctx context.Context) error' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'GoalPhaseTodo GoalPhase = "todo"' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'GoalPhaseDone GoalPhase = "done"' pkg/domain/goal_phase.go)" = "1"
test "$(grep -c 'func (f GoalFrontmatter) Phase() \*GoalPhase' pkg/domain/goal_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*GoalFrontmatter) SetPhase(p \*GoalPhase)' pkg/domain/goal_frontmatter.go)" = "1"
test "$(grep -c 'func (t TaskPhase) Validate(ctx context.Context) error' pkg/domain/task_phase.go)" = "1"
test "$(grep -c 'func NormalizeTaskPhase(raw string) (TaskPhase, bool)' pkg/domain/task_phase.go)" = "1"
test "$(grep -c 'func (f TaskFrontmatter) Phase() \*TaskPhase' pkg/domain/task_frontmatter.go)" = "1"
test "$(grep -c 'func (f \*TaskFrontmatter) SetPhase(p \*TaskPhase)' pkg/domain/task_frontmatter.go)" = "1"
```

**The CHANGELOG entry sits under `## Unreleased` specifically** — the spec's Acceptance Criterion 10. A bare `grep` does not assert the section: the bullet could pass folded under a released `## vX.Y.Z` heading, which is the failure the repo's changelog-fold guard exists to catch, so this walks the section headings:

```
test "$(awk '/^## /{sec=$0} /topic phase/{print sec}' CHANGELOG.md)" = "## Unreleased"
test "$(grep -c '^## Unreleased$' CHANGELOG.md)" = "1"
test "$(grep -c '^All notable changes to this project' CHANGELOG.md)" = "1"
```

**Formatting and the whole suite:**

```
test -z "$(gofmt -e -l pkg/domain/topic_frontmatter.go pkg/domain/topic_frontmatter_test.go pkg/ops/frontmatter_entity_test.go integration/cli_test.go)"
go test ./pkg/... -count=1 > /tmp/topic-phase-all.log 2>&1; test "$?" = "0"
```

Finally, walk spec 052's Acceptance Criteria 3, 4, 5, 6, 7 and 10 against the change and state in your completion report which requirement and which spec satisfies each one, and which evidence covers each row of the spec's Failure Modes table.
</verification>
