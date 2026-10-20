---
status: approved
spec: [062-goal-topic-phase-set]
created: "2026-10-20T10:52:00Z"
queued: "2026-10-20T10:51:07Z"
---

# Lock the widened goal/topic phase set end-to-end and record it (spec 062, prompt 2 of 2)

<summary>
- Integration specs prove `goal set` and `topic set` accept `ai_review` and `human_review` and write them to disk.
- Integration specs prove `goal set … phase bogus` and `topic set … phase bogus` exit 1 and print the six valid values.
- The invalid-phase probes leave the page byte-identical.
- `in_progress` stays rejected by both verbs.
- The changelog records the widened enum and the listed-valid-values error under an unreleased heading.
</summary>

<objective>
Add end-to-end CLI coverage for the widened goal and topic phase enums and a `## Unreleased` CHANGELOG bullet, so the behavior is locked at the command boundary rather than only at the domain layer.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` first, and `coding/docs/go-testing-guide.md` for the Ginkgo/Gomega integration-test conventions.
Read these files before editing:
- `integration/cli_test.go` — helpers `createTempVaultWithTopicPages` (~line 188, creates both `Goals/` and `Topics/`), `showPhaseField` (~line 283, reads `.fields.phase` from `<entity> show --output json`), and the existing topic phase block (~line 4799-4900) whose `bogus` test already asserts exit 1 and the stderr wording.
- `CHANGELOG.md` — the preamble ends before the newest `## vX.Y.Z`; `scripts/check-changelog.sh` requires `## Unreleased` to sit below the preamble and above the newest release section.
- `specs/in-progress/062-goal-topic-phase-set.md` — Acceptance Criteria 2-5 and 8.
</context>

<requirements>
1. In `integration/cli_test.go`, add a new nested `Describe("vault-cli goal command family", func() { ... })` immediately after the existing `Describe("vault-cli topic command family", ...)` block (starts ~line 4652; its phase tests begin ~line 4799), and put the goal phase `It`s inside it. Use `createTempVaultWithTopicPages("Topics", nil, map[string]string{...})` so a `Goals/` page exists. Cover:
   - `goal set <goal> phase ai_review` exits 0, the file contains `phase: ai_review`, and `showPhaseField(configPath, "goal", <goal>)` returns `ai_review`.
   - `goal set <goal> phase human_review` exits 0, the file contains `phase: human_review`, and `showPhaseField` returns `human_review`.
   - `goal set <goal> phase bogus` exits 1; stderr contains `unknown goal phase 'bogus'` and the six values `todo, planning, execution, ai_review, human_review, done`; the file's sha256 is unchanged before and after.
   - `goal set <goal> phase in_progress` exits 1, and stderr contains `unknown goal phase 'in_progress'` plus the six accepted values.
   - A page hand-typed to `phase: in_progress` reads back unchanged: build the goal fixture with `phase: in_progress` and assert `showPhaseField(configPath, "goal", <goal>)` returns `in_progress` and `goal get <goal> phase` prints it — the read path validates nothing (spec Failure Modes row 2).

2. In the same file, extend the existing topic phase block:
   - Add `topic set <topic> phase ai_review` and `phase human_review` success cases (exit 0, `phase: ai_review` / `phase: human_review` on disk, `showPhaseField(configPath, "topic", <topic>)` returns the value).
   - Extend the existing `topic set … phase bogus` assertion so stderr also contains all six values, not only `unknown topic phase 'bogus'`.
   - Add `topic set <topic> phase in_progress` exits 1, and stderr contains `unknown topic phase 'in_progress'` plus the six accepted values.

3. Use the existing test idioms: `exec.Command(binPath, "--config", configPath, "--vault", "test", ...)` + `gexec.Start` + `Eventually(session).Should(gexec.Exit(N))`; read files with `os.ReadFile(path)` and the `//#nosec G304 -- test file` comment where the existing tests carry it. Compute the sha256 helper locally in the test (as the existing topic `bogus` test does) rather than adding a package-level helper.

4. In `CHANGELOG.md`, insert a `## Unreleased` section directly below the preamble (`All notable changes…` and the `* MAJOR / MINOR / PATCH` lines) and above the newest `## vX.Y.Z`, with one bullet:
   ```markdown
   - feat: `vault-cli goal set` and `topic set` accept the full task phase set (`todo`, `planning`, `execution`, `ai_review`, `human_review`, `done`), and an invalid phase now exits non-zero with a message listing the accepted values, rendered from the enum.
   ```

5. Before finishing, re-run the `<verification>` block and walk each requirement against the diff.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT change production code in this prompt; it is tests plus the CHANGELOG only. If a spec fails because prompt 1's behavior is missing, stop and report rather than editing `pkg/`.
- Keep `## Unreleased` below the preamble — never between the `# Changelog` title and the preamble.
- Do not add a new scenario file; the spec's scenario coverage is NO new scenario.
- Existing integration tests must still pass.
</constraints>

<verification>
Run `make precommit` — must pass.

Then:
- `go test ./integration/...` — must pass.
- `sed -n '/^## Unreleased/,/^## v/p' CHANGELOG.md | grep -q ai_review` — must succeed.
- `grep -n "unknown goal phase 'bogus'" integration/cli_test.go` — must return at least one match.
- `grep -n 'human_review' integration/cli_test.go` — must return matches for both goal and topic cases.
</verification>
