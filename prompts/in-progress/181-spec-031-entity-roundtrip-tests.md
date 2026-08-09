---
status: approved
spec: [031-bug-unquoted-wikilink-mangled-on-frontmatter-write]
created: "2026-08-09T15:45:00Z"
queued: "2026-08-09T14:33:20Z"
---

# Prove the wikilink fix reaches all six entity types

<summary>
- Adds a regression suite that proves an unquoted frontmatter wikilink survives a write for every entity type the tool manages: task, goal, theme, objective, vision, and decision.
- Each entity is exercised through the same code path its real command uses, not through a shortcut, so the test would catch a fix that only reached one entity.
- Each entity file starts with the bug report's reproduction frontmatter, extended with two extra shapes (a bare block-sequence entry and an already-quoted value), and is checked afterwards for a working, quoted link.
- Deviation from the spec, flagged for the reviewer: spec AC 2 words its evidence as `vault-cli <entity> set …`, i.e. driving the compiled binary. This prompt drives the `ops` layer directly — the same composition `pkg/cli/cli.go` builds — because the container has no installed binary and shelling out to one would test the wrong artifact. Coverage is equivalent or better (it still crosses find → set → write). The spec's AC wording should be updated to match rather than the prompt changed.
- The check also proves the destroyed nested-list shape is nowhere in the written file.
- A second write on the same file proves the result is stable rather than drifting further on each run.
- An already-quoted wikilink in the same file is confirmed unchanged, so the fix is not over-reaching.
- The markdown body below the frontmatter is confirmed untouched.
- Decision is covered by calling its write path directly, because it has no `set` command and its only write entry point is operator-gated.
- No production code changes — this prompt is proof that the previous one generalised correctly.
</summary>

<objective>
Add a round-trip regression suite at `/workspace/pkg/ops/wikilink_roundtrip_test.go` that writes a file containing bare frontmatter wikilinks for each of Task, Goal, Theme, Objective, Vision, and Decision, drives it through that entity's own real storage write path, and asserts the wikilink comes back as a working quoted scalar with no nested block sequence — proving the single shared parse chokepoint from prompt 1 really covers all six entities rather than assuming it.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions and `/workspace/docs/dod.md` for the Definition of Done.

**Precondition — prompt 1 must have shipped.** This prompt asserts behavior that only exists after the quoting pass lands. Verify it before writing any test:

```
grep -n 'yaml.Unmarshal(quoteBareWikilinks(matches\[1\]), &m)' pkg/storage/base.go
```

If that prints nothing, prompt 1 has not shipped. STOP, report `"status":"failed"` with the message `quoteBareWikilinks not yet deployed (prompt 1)`, and do NOT work around it by adding a quoting helper of your own to the test file or to any entity storage file.

Read these files fully before making changes:

- `/workspace/pkg/storage/storage.go` — the narrow storage constructors and the storage `Config`. Verified:

  ```go
  type Config struct {
      TasksDir      string
      GoalsDir      string
      ThemesDir     string
      ObjectivesDir string
      VisionDir     string
      DailyDir      string
      Excludes      []string
  }

  func NewTaskStorage(storageConfig *Config) TaskStorage
  func NewGoalStorage(storageConfig *Config) GoalStorage
  func NewThemeStorage(storageConfig *Config) ThemeStorage
  func NewObjectiveStorage(storageConfig *Config) ObjectiveStorage
  func NewVisionStorage(storageConfig *Config) VisionStorage
  func NewDecisionStorage(storageConfig *Config) DecisionStorage
  ```

  `NewStorage(nil)` falls back to `DefaultConfig()` (`Tasks`, `Goals`, `21 Themes`, `22 Objectives`, `20 Vision`) — this prompt does **not** use that; it builds an explicit `Config` mirroring the real vault layout.

- `/workspace/pkg/ops/frontmatter.go` — the task set path. Verified:

  ```go
  type FrontmatterSetOperation interface {
      Execute(ctx context.Context, vaultPath, taskName, key, value string) error
  }
  func NewFrontmatterSetOperation(taskStorage storage.TaskStorage) FrontmatterSetOperation
  ```

  `Execute` does `FindTaskByName` → `task.SetField(ctx, key, value)` → `WriteTask`. Tasks deliberately use this operation and **not** `EntitySetOperation`.

- `/workspace/pkg/ops/frontmatter_entity.go` — the other four set paths. Verified:

  ```go
  type EntitySetOperation interface {
      Execute(ctx context.Context, vaultPath, entityName, key, value string) error
  }
  func NewGoalSetOperation(goalStorage storage.GoalStorage) EntitySetOperation
  func NewThemeSetOperation(themeStorage storage.ThemeStorage) EntitySetOperation
  func NewObjectiveSetOperation(objectiveStorage storage.ObjectiveStorage) EntitySetOperation
  func NewVisionSetOperation(visionStorage storage.VisionStorage) EntitySetOperation
  ```

  Each `Execute` does `FindXByName` → `x.SetField(ctx, key, value)` → `WriteX`. Both interfaces have the identical `Execute` signature, which is why one table can drive all five.

- `/workspace/pkg/cli/cli.go` `createTaskSetCommand` (line 2015) and `createThemeCommands` (line 1470) — the compositions this test reproduces. `vault-cli task set` builds `ops.NewFrontmatterSetOperation(storage.NewTaskStorage(storage.NewConfigFromVault(vault)))`; `vault-cli theme set` builds `ops.NewThemeSetOperation(storage.NewThemeStorage(cfg))`. Driving the ops layer over real storage is therefore the same value path production traffic takes, minus cobra argument parsing and stdout formatting.

- `/workspace/pkg/storage/decision.go` — Decision has no `set` operation. Verified relevant methods:

  ```go
  func (d *decisionStorage) ListDecisions(ctx context.Context, vaultPath string) ([]*domain.Decision, error)
  func (d *decisionStorage) WriteDecision(ctx context.Context, decision *domain.Decision) error
  ```

  `ListDecisions` walks the whole vault recursively and returns only files whose frontmatter has `needs_review: true`. `WriteDecision` copies `decision.RawMap()` and overlays six managed keys (`needs_review`, `reviewed`, `reviewed_date`, `status`, `type`, `page_type`) — `related_task` and `themes` are preserved, not managed. `vault-cli decision` exposes only `ack` and `list`, and `ack` is operator-gated, so this test calls `WriteDecision` directly.

- `/workspace/pkg/domain/task_frontmatter.go` `SetField` (line 428) and the four sibling `SetField` methods in `goal_frontmatter.go` (line 272), `theme_frontmatter.go` (line 167), `objective_frontmatter.go` (line 204), `vision_frontmatter.go` (line 120) — all five accept `"priority"` and parse it as an integer; `domain.Priority.Validate` only rejects negatives, so `3` and `4` are both valid. `SetField` touches no other key.

- `/workspace/pkg/ops/ops_suite_test.go` — `package ops_test`, suite func `TestSuite`, `RunSpecs(t, "ops Test Suite")`. The new file joins this suite automatically.

- `/workspace/pkg/ops/daily_note_entry_test.go` — the in-repo precedent for an `ops_test` file importing `pkg/storage` directly. Its import block is the shape to copy:

  ```go
  package ops_test

  import (
      "fmt"

      . "github.com/onsi/ginkgo/v2"
      . "github.com/onsi/gomega"

      "github.com/bborbe/vault-cli/pkg/ops"
      "github.com/bborbe/vault-cli/pkg/storage"
  )
  ```

- `/workspace/pkg/storage/goal_test.go` lines 22-42 — the temp-vault fixture shape to copy (`os.MkdirTemp` in `BeforeEach`, `os.RemoveAll` in `AfterEach`).

- `/workspace/.golangci.yml` — `run.tests: true`, but `dupl`, `unparam`, and `gosec` are excluded for `_test.go` files, so five structurally similar table entries are fine. `forcetypeassert` is **not** excluded — use the two-value form for any type assertion (this test needs none).

**Measured YAML facts you must not re-derive** (executed against the `gopkg.in/yaml.v3` version in `/workspace/go.mod`):

| After the prompt-1 fix, this frontmatter | is written back as |
|---|---|
| `related_task: [[Some Other Task]]` | `related_task: '[[Some Other Task]]'` |
| `themes:` + `    - [[A Theme]]` | `themes:` + `    - '[[A Theme]]'` |
| `related: '[[Already Quoted]]'` | `related: '[[Already Quoted]]'` (unchanged) |

`yaml.Marshal` sorts keys alphabetically, so assert on individual lines with `ContainSubstring`, never on the whole frontmatter block as one string.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, `DescribeTable`, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.
</context>

<requirements>

## 1. Create the round-trip regression file

Create `/workspace/pkg/ops/wikilink_roundtrip_test.go` in `package ops_test`, with the standard copyright header used by every file in this repo:

```go
// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
```

Imports: `context`, `os`, `path/filepath`, `strings`, dot-imported ginkgo/v2 and gomega, `github.com/bborbe/vault-cli/pkg/ops`, `github.com/bborbe/vault-cli/pkg/storage`.

The top-level `Describe` text must contain the word `Wikilink` — `<verification>` focuses the suite on it.

```go
var _ = Describe("bare Wikilink survival through every entity write path", func() {
	var (
		ctx           context.Context
		vaultPath     string
		storageConfig *storage.Config
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		vaultPath, err = os.MkdirTemp("", "vault-wikilink-roundtrip-*")
		Expect(err).To(BeNil())

		storageConfig = &storage.Config{
			TasksDir:      "24 Tasks",
			GoalsDir:      "23 Goals",
			ThemesDir:     "21 Themes",
			ObjectivesDir: "22 Objectives",
			VisionDir:     "20 Vision",
		}
		for _, dir := range []string{
			"24 Tasks", "23 Goals", "21 Themes", "22 Objectives", "20 Vision", "40 Decisions",
		} {
			Expect(os.MkdirAll(filepath.Join(vaultPath, dir), 0755)).To(Succeed())
		}
	})

	AfterEach(func() {
		if vaultPath != "" {
			_ = os.RemoveAll(vaultPath)
		}
	})

	// ... requirements 2-5 go here ...
})
```

## 2. The fixture and the shared assertion

Inside the `Describe`, above the specs, declare the fixture and the assertion helper.

The fixture is the spec's reproduction input — kept verbatim, including the **scalar** `themes: [[A Theme]]` line that spec AC 1 asserts — extended with two extra shapes: a bare wikilink as a block-sequence entry (under a separate `tags:` key, so the scalar case is not displaced), and an already-quoted wikilink that must come out unchanged. It is a raw backtick literal, so its lines start at column 0 inside the literal — do not indent them to match the surrounding Go code, and do not let `make format` talk you into reflowing it (golines does not reflow raw strings).

```go
	const entityFixture = `---
priority: 2
related: '[[Already Quoted]]'
related_task: [[Some Other Task]]
status: in_progress
tags:
    - [[A Theme]]
themes: [[A Theme]]
---
Tags: [[Task]]

---
body
`
```

Both `themes` shapes matter and they assert different things: `themes: [[A Theme]]` (scalar) is the exact line from the spec's Reproduction and must come back as the quoted **scalar** `themes: '[[A Theme]]'` — NOT normalised into a one-element sequence, per spec Desired Behavior 3. The `tags:` block-sequence entry must come back as `    - '[[A Theme]]'`. Assert both.

```go
	// assertWikilinksSurvived re-reads the file from disk and pins every shape the
	// spec's Acceptance Criteria name: the bare scalar became a quoted scalar, the
	// bare list entry became a quoted entry, the already-quoted value is unchanged,
	// no nested block sequence exists anywhere, and the markdown body is intact.
	assertWikilinksSurvived := func(filePath string) {
		raw, err := os.ReadFile(filePath)
		Expect(err).To(BeNil())
		written := string(raw)

		Expect(written).To(ContainSubstring("\nrelated_task: '[[Some Other Task]]'\n"))
		// Scalar stays scalar — spec AC 1 + Desired Behavior 3. Not normalised to a list.
		Expect(written).To(ContainSubstring("\nthemes: '[[A Theme]]'\n"))
		// Block-sequence entry stays an entry.
		Expect(written).To(ContainSubstring("\n    - '[[A Theme]]'\n"))
		Expect(written).To(ContainSubstring("\nrelated: '[[Already Quoted]]'\n"))
		Expect(written).NotTo(MatchRegexp(`(?m)^ *- - `))
		Expect(written).NotTo(ContainSubstring("''[["))
		Expect(written).To(ContainSubstring("\nTags: [[Task]]\n"))
		Expect(written).To(HaveSuffix("\n---\nbody\n"))
		Expect(strings.Count(written, "related_task: '[[Some Other Task]]'")).To(Equal(1))
	}
```

The `(?m)^ *- - ` regexp is the Go equivalent of the spec's `grep -c '^ *- - '` evidence command; keep it exactly as written so the two stay in step.

## 3. Table-drive the five entities that have a `set` command

Add one `DescribeTable` inside the `Describe`. Declare the executor parameter as an **inline func type**, not a named type — Ginkgo passes `Entry` arguments through reflection, and an inline type sidesteps any named-type assignability question:

```go
	DescribeTable("survives a set-and-write cycle",
		func(
			dir string,
			exec func(
				ctx context.Context,
				cfg *storage.Config,
				vaultPath, name, key, value string,
			) error,
		) {
			filePath := filepath.Join(vaultPath, dir, "Repro Entity.md")
			Expect(os.WriteFile(filePath, []byte(entityFixture), 0600)).To(Succeed())

			Expect(exec(ctx, storageConfig, vaultPath, "Repro Entity", "priority", "3")).
				To(Succeed())
			assertWikilinksSurvived(filePath)

			// Idempotence: a second write must leave the wikilink lines unchanged.
			Expect(exec(ctx, storageConfig, vaultPath, "Repro Entity", "priority", "4")).
				To(Succeed())
			assertWikilinksSurvived(filePath)
		},
		Entry("task", "24 Tasks", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewFrontmatterSetOperation(storage.NewTaskStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value)
		}),
		Entry("goal", "23 Goals", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewGoalSetOperation(storage.NewGoalStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value)
		}),
		Entry("theme", "21 Themes", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewThemeSetOperation(storage.NewThemeStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value)
		}),
		Entry("objective", "22 Objectives", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewObjectiveSetOperation(storage.NewObjectiveStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value)
		}),
		Entry("vision", "20 Vision", func(
			ctx context.Context, cfg *storage.Config, vaultPath, name, key, value string,
		) error {
			return ops.NewVisionSetOperation(storage.NewVisionStorage(cfg)).
				Execute(ctx, vaultPath, name, key, value)
		}),
	)
```

Use those five `Entry` descriptions verbatim — `task`, `goal`, `theme`, `objective`, `vision`. `<verification>` greps the rendered spec names, which Ginkgo composes as `survives a set-and-write cycle task` and so on.

Do not replace the ops operations with direct `storage.WriteX` calls. Going through the real set operation is the point: it is the same composition `pkg/cli/cli.go` builds, so a regression in the find → set → write sequence is caught here, not just a regression in the serializer.

## 4. Cover Decision through its own write path

Add one `It` beside the `DescribeTable`, named exactly `survives a decision write`. Decision has no `set` subcommand and `ack` is operator-gated, so this drives `ListDecisions` → `WriteDecision` directly:

```go
	It("survives a decision write", func() {
		const decisionFixture = `---
needs_review: true
page_type: decision
related: '[[Already Quoted]]'
related_task: [[Some Other Task]]
status: proposed
themes:
    - [[A Theme]]
type: Trading Decision Record
---
Tags: [[Task]]

---
body
`
		filePath := filepath.Join(vaultPath, "40 Decisions", "TDR 2026-01-09 - Repro.md")
		Expect(os.WriteFile(filePath, []byte(decisionFixture), 0600)).To(Succeed())

		decisionStore := storage.NewDecisionStorage(storageConfig)

		decisions, err := decisionStore.ListDecisions(ctx, vaultPath)
		Expect(err).To(BeNil())
		Expect(decisions).To(HaveLen(1))
		Expect(decisionStore.WriteDecision(ctx, decisions[0])).To(Succeed())
		assertWikilinksSurvived(filePath)

		// Idempotence: re-read and write again; the wikilink lines must not move.
		decisions, err = decisionStore.ListDecisions(ctx, vaultPath)
		Expect(err).To(BeNil())
		Expect(decisions).To(HaveLen(1))
		Expect(decisionStore.WriteDecision(ctx, decisions[0])).To(Succeed())
		assertWikilinksSurvived(filePath)
	})
```

`needs_review` stays `true` across both writes, which is why the second `ListDecisions` still returns the file. `related_task` and `themes` are not managed keys, so `WriteDecision` copies them through untouched.

## 5. Pin the still-broken shape so the Non-goal cannot silently change

Add one `It` beside the others, named exactly `leaves a decision wikilink with a trailing comment broken`. This is a **negative** spec: the spec's Non-goals declare that a bare wikilink carrying a trailing YAML comment is knowingly not rewritten, and this pins that decision so a later "improvement" cannot land unnoticed.

```go
	It("leaves a decision wikilink with a trailing comment broken", func() {
		const commentFixture = `---
needs_review: true
related_task: [[Some Other Task]]  # migrated 2026-01
status: proposed
---
Tags: [[Task]]

---
body
`
		filePath := filepath.Join(vaultPath, "40 Decisions", "TDR Comment.md")
		Expect(os.WriteFile(filePath, []byte(commentFixture), 0600)).To(Succeed())

		decisionStore := storage.NewDecisionStorage(storageConfig)
		decisions, err := decisionStore.ListDecisions(ctx, vaultPath)
		Expect(err).To(BeNil())
		Expect(decisions).To(HaveLen(1))
		Expect(decisionStore.WriteDecision(ctx, decisions[0])).To(Succeed())

		raw, err := os.ReadFile(filePath)
		Expect(err).To(BeNil())
		Expect(string(raw)).To(MatchRegexp(`(?m)^ *- - Some Other Task$`))
	})
```

If this spec ever starts failing, the Non-goal changed — update the spec first, not this assertion.

## 6. Coverage

This prompt adds **no production code**, so it introduces no new uncovered statements. It raises integration coverage of the existing set operations and of `WriteDecision`; do not add retroactive coverage to unrelated untested `pkg/ops` code, and do not modify any existing test file.

</requirements>

<constraints>
- Do NOT modify any non-test file. This prompt adds exactly one new file, `/workspace/pkg/ops/wikilink_roundtrip_test.go`, and changes nothing else in `pkg/`.
- Do NOT modify `/workspace/pkg/storage/base.go`. In particular do NOT touch `parseToFrontmatterMap`, `quoteBareWikilinks`, or `serializeMapAsFrontmatter` — prompt 1 landed them and spec 030 froze the serializer.
- Do NOT modify, reorder, or delete any existing spec in `/workspace/pkg/ops/` or `/workspace/pkg/storage/`. If an existing spec breaks, the prompt-1 change is wrong — report it rather than editing the spec.
- Do NOT add per-entity production code. All six entity types are covered by the single shared parse chokepoint; the whole point of this prompt is to prove that, not to bolt on entity-specific handling.
- Do NOT replace the ops set operations with direct `storage.WriteTask` / `WriteGoal` / `WriteTheme` / `WriteObjective` / `WriteVision` calls. Driving the real operation is what makes this an integration test rather than a second copy of prompt 1's unit tests.
- Do NOT shell out to the compiled `vault-cli` binary, and do NOT add a cobra command test. The CLI layer adds only argument parsing and stdout formatting to the value path, and `integration/cli_test.go` already covers command registration.
- Do NOT write to any real vault. Every path in this test lives under an `os.MkdirTemp` directory that `AfterEach` removes. Never reference `$HOME`, `~/Documents/Obsidian`, or the Trading vault's TDR file — the spec's TDR repair is an operator step performed after release, not a container step.
- Do NOT add a config key, flag, environment variable, or opt-out. The quoting is an invariant.
- Do NOT relax `assertWikilinksSurvived` to make a case pass. If the bare list entry comes back as `- - - A Theme`, the fix has a hole — report it as a failure rather than weakening the assertion.
- `pkg/ops/` is a library layer — operations return structured results and never write to stdout. No `fmt.Print*` and no `os.Stdout` in `pkg/`, tests included.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/` non-test code. (`context.Background()` in a `BeforeEach` is the established test convention and is fine.)
- Tests use Ginkgo v2 / Gomega in the external `ops_test` package. `/workspace/pkg/ops/ops_suite_test.go` already exists (suite func `TestSuite`), so the new file's specs run automatically.
- `forcetypeassert` is enabled with `run.tests: true` — any type assertion must use the two-value form. This test should need none.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string, do NOT add a CHANGELOG entry (prompt 1 already wrote the `## Unreleased` bullet that covers this change), and do NOT create a git tag.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**

```
make test
```

Must exit 0, including every pre-existing `pkg/ops` and `pkg/storage` spec.

**2. The new file exists and is wired to the real paths** — each must print at least one line:

```
grep -n 'package ops_test' pkg/ops/wikilink_roundtrip_test.go
grep -n 'ops.NewFrontmatterSetOperation(storage.NewTaskStorage(cfg))' pkg/ops/wikilink_roundtrip_test.go
grep -n 'ops.NewGoalSetOperation(storage.NewGoalStorage(cfg))' pkg/ops/wikilink_roundtrip_test.go
grep -n 'ops.NewThemeSetOperation(storage.NewThemeStorage(cfg))' pkg/ops/wikilink_roundtrip_test.go
grep -n 'ops.NewObjectiveSetOperation(storage.NewObjectiveStorage(cfg))' pkg/ops/wikilink_roundtrip_test.go
grep -n 'ops.NewVisionSetOperation(storage.NewVisionStorage(cfg))' pkg/ops/wikilink_roundtrip_test.go
grep -n 'decisionStore.WriteDecision' pkg/ops/wikilink_roundtrip_test.go
```

**3. No production file was touched** — each of these must print nothing:

```
grep -rn 'wikilink\|Wikilink' pkg/storage/task.go pkg/storage/goal.go pkg/storage/theme.go pkg/storage/objective.go pkg/storage/vision.go pkg/storage/decision.go
grep -rn 'wikilink\|Wikilink' pkg/ops/frontmatter.go pkg/ops/frontmatter_entity.go pkg/cli/cli.go
```

A bare `grep` with no matches exits 1 — that non-zero exit is the expected, passing result here.

**4. All six entities are actually exercised.**

Run each suite ONCE and capture the output, then grep the captures:

```
go test -count=1 -v ./pkg/ops/     -args -ginkgo.v -ginkgo.no-color > /tmp/ops-specs.txt 2>&1
go test -count=1 -v ./pkg/storage/ -args -ginkgo.v -ginkgo.no-color > /tmp/storage-specs.txt 2>&1
```

**Both flags are mandatory.** Without `-ginkgo.v` Ginkgo uses the dot reporter and prints no spec descriptions at all. Without `-ginkgo.no-color` it injects ANSI escape sequences between the Describe/DescribeTable description and the It/Entry description — measured against this repo's existing `pkg/ops/lint_test.go` table, a contiguous-substring grep returns `0` with color on and `1` with it off. Either omission makes every grep below silently return `0`, which reads as "the tests are missing" when they ran and passed.

Each of these must print `>= 1`:

```
grep -c "survives a set-and-write cycle task" /tmp/ops-specs.txt
grep -c "survives a set-and-write cycle goal" /tmp/ops-specs.txt
grep -c "survives a set-and-write cycle theme" /tmp/ops-specs.txt
grep -c "survives a set-and-write cycle objective" /tmp/ops-specs.txt
grep -c "survives a set-and-write cycle vision" /tmp/ops-specs.txt
grep -c "survives a decision write" /tmp/ops-specs.txt
grep -c "leaves a decision wikilink with a trailing comment broken" /tmp/ops-specs.txt
```

If a grep returns 0, first confirm both ginkgo flags were passed — do NOT edit the `Entry` text to chase a match.

**5. Prompt 1's specs still pass** — grep the storage capture from step 4; each must print `>= 1`:

```
grep -c "writes a bare wikilink scalar back as a quoted scalar" /tmp/storage-specs.txt
grep -c "is idempotent across two write cycles" /tmp/storage-specs.txt
```

**6. Pre-existing specs survived** — same captures; each must print `>= 1`:

```
grep -c "IsOwnDailyNoteEntry" /tmp/ops-specs.txt
grep -c "preserves every non-managed frontmatter key" /tmp/storage-specs.txt
```

**7. No existing spec was deleted or weakened** — spec-count floors, not a diff.

`.git` is masked inside this container (`hideGit=true`), so **no `git` command works here** — `git diff` would die with `fatal: not a git repository`, and the daemon does not check verification exit codes, so it would report a pass that never ran. Count the spec-declaring lines directly instead. Each must come back **exactly equal** to the floor shown — this prompt touches none of these files:

```
grep -c 'It(\|DescribeTable(' pkg/ops/frontmatter_test.go          # must equal 63
grep -c 'It(\|DescribeTable(' pkg/ops/frontmatter_entity_test.go   # must equal 51
grep -c 'It(\|DescribeTable(' pkg/ops/decision_ack_test.go         # must equal 11
grep -c 'It(\|DescribeTable(' pkg/storage/decision_test.go         # must equal 35
```

Those are the pre-change counts, measured on this branch at prompt-authoring time. If a starting count does not match, re-measure before editing and use your measured value as the floor. A count that changed means an existing spec was edited — restore it rather than adjusting the floor.

**8. Temp vaults are cleaned up** — must print nothing after the suite has run:

```
ls -d "${TMPDIR:-/tmp}"/vault-wikilink-roundtrip-* 2>/dev/null
```

`os.MkdirTemp("", …)` honours `$TMPDIR`, so a hardcoded `/tmp` would check the wrong directory and pass vacuously if `TMPDIR` is set.

**9. Full gate, once, at the end:**

```
make precommit
```

Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make format`, etc.) until it passes, then run `make precommit` one final time. `make format` runs golines at a 100-column limit — the `Entry` closures above are already split across lines to stay inside it, and golines does not reflow the raw backtick fixtures.
</verification>
