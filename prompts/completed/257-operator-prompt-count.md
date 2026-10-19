---
status: completed
summary: Added metrics_operator_prompt_count with an operator-typed prompt predicate, wrote it at completion, surfaced it in rollup weekly and JSON, with tests, fixture falsification, and docs.
execution_id: vault-cli-operator-prompt-count-exec-257-operator-prompt-count
dark-factory-version: v0.198.7
created: "2026-10-19T17:58:43Z"
queued: "2026-10-19T18:08:59Z"
started: "2026-10-19T18:10:01Z"
completed: "2026-10-19T18:20:48Z"
---

# Record and report operator-typed prompt counts alongside interaction counts

<summary>
- A completed task records how many prompts the operator actually typed, as a new `metrics_operator_prompt_count` frontmatter field
- The count excludes tool results, system injections, scheduled wakeups, peer messages, and the known legacy transcript noise
- The existing `metrics_interaction_count` keeps its current meaning and value — every `type:"user"` line, unchanged
- A task that was never worked on carries neither count; an unknown count is never written as zero
- `vault-cli rollup weekly` prints an "Operator prompts" figure beside "Human interactions", and a week with no operator counts says so instead of printing zero
- The JSON output gains an `operator_prompts` key with the same sentinel discipline
- A regression fixture covers one line per transcript entry kind, and a falsification step proves the fixture fails if the filter is loosened to count every user line
- The change is recorded as one unreleased feature bullet; no version is bumped and nothing is tagged
</summary>

<objective>
Teach vault-cli to measure, store, and report the number of prompts the operator typed — not every transcript line that says `type:"user"`. The interaction count today counts tool results, meta injections, scheduled wakeups, task notifications and peer messages as human interactions, so the rollup's "Human interactions" figure overstates operator effort. Add `metrics_operator_prompt_count` as a second, narrower measurement written at completion beside `metrics_interaction_count`, and surface its weekly total in `vault-cli rollup weekly` and its JSON output. The existing interaction count must not change.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` for the Definition of Done (doc comments, `github.com/bborbe/errors`, Ginkgo v2/Gomega, no output from `pkg/ops/`, CHANGELOG placement).

Read these files fully before editing — every signature, field name and line number below was verified against this worktree's `origin/master` on 2026-10-19:

- `pkg/ops/interaction_count.go` — `InteractionCounter` interface (line 23), `NewInteractionCounter` (line 34), `Count` (line 46), `isSafeSessionID` (line 71), `encodeProjectDir` (line 88), `countUserTurnsInSessionLog` (line 107), `metricsSessionIDs` (line 137). `Count` dedupes session ids, skips unsafe ids before building a path, honours `ctx.Done()`, and streams each log line by line with a 16 MB scanner buffer.
- `pkg/ops/complete.go` — `NewCompleteOperation` (line 43), the `interactionCounter InteractionCounter` field (line 62), the non-recurring path calling `c.applyCompletionMetrics(ctx, task, completedD)` (line 131), `applyCompletionMetrics` (line 175), and the recurring path's `ClearMetricsInteractionCount()` (line 305).
- `pkg/domain/task_frontmatter_metrics.go` — `MetricsInteractionCount` / `SetMetricsInteractionCount` / `ClearMetricsInteractionCount` (lines 106-123) and `coerceMetricsInt` (line 210). The accessor returns `nil` for absent or malformed values so "unknown" is never forged as `0`.
- `pkg/ops/rollup_weekly.go` — the sentinel constants `rollupNoData` / `rollupNoRecordedCounts` / `rollupUndefined` (lines 23-27), `RollupWeeklyResult` (line 103), `Execute`'s no-week branch and figure branch (lines 157-179), and `rollupFigures` (line 262).
- `pkg/cli/rollup.go` — `formatRollupWeeklyPlain` (line 113) and `writeRollupWeeklyFigures` (line 212).
- `pkg/ops/interaction_count_test.go` — the existing Ginkgo suite and its `writeFixture(id string, lines ...string)` helper (line 46).
- `pkg/ops/complete_test.go` — the outer `Describe` at line 22, its `BeforeEach` at line 40 and `JustBeforeEach` at line 70, and the metrics contexts at lines 1041-1116.
- `pkg/ops/rollup_weekly_test.go` — `writeRollupTask` (line 25), `rollupTaskFrontmatter` (line 33), `assertRollupNoData` (line 81), the frozen JSON-key spec (line 366).
- `pkg/cli/rollup_test.go` — the exact-output golden strings (lines 37 and 166) and `cli.FormatRollupWeeklyPlainForTest` (`pkg/cli/export_test.go` line 47).
- `integration/cli_test.go` — `Describe("vault-cli rollup weekly", …)` (line 600) with `rollupFixtureTasks` (line 601) and the plain/JSON comparison spec (line 700).
- `README.md` lines 210-245 — the rollup section, which currently says the command "reports three figures".
- `CHANGELOG.md` lines 1-16 — there is **no** `## Unreleased` section today; the newest released heading is `## v0.171.1`. `scripts/check-changelog.sh` fails the build when a `## ` section appears above the preamble.

### The predicate — measured 2026-10-19 from real transcripts, do not re-derive

Measured 2026-10-19 over 12 transcripts from a Personal-vault session-log directory on the authoring host (11,916 `type:"user"` entries after removing tool results; that directory is not mounted in the container and is not re-read):

| Signal | Operator-typed | Excluded |
|---|---|---|
| `toolUseResult` present / content is a `tool_result` block | — | 547 of 628 user entries in a 5-file sample |
| `isMeta: true` | — | 5,302 |
| `turnOrigin` | `human` (2,212) | `scheduled` (1,397), `task_notification` (2,144), `peer` (2,571) |
| `promptSource` | `typed` (1,423), `queued` (134), `suggestion_accepted` (22) | `system` (4,811) |
| legacy rows carrying none of those fields | `<command-name>` slash invocations, plain text | `<local-command-stdout>`, `<local-command-caveat>`, `Caveat: The messages below…`, `This session is being continued…`, `[Request interrupted by user]`, `<task-notification>`, `Another Claude session sent a message:` |

A `type:"user"` entry is an operator prompt iff **all** of the following hold:

1. it has no top-level `toolUseResult` key;
2. its `message.content` is not, and does not contain, a `tool_result` block;
3. `isMeta` is not `true`;
4. `turnOrigin` is absent or exactly `human`;
5. `promptSource` is absent or one of `typed`, `queued`, `suggestion_accepted`;
6. if the row carries none of the four signal fields above (a legacy row), its text does not start with one of the seven noise prefixes listed in the table.

`<command-message>` entries and `<command-name>` slash-command entries DO count — they are operator-typed. Do not add any exclusion beyond this list.

Coding-plugin docs (in-container paths — never use host paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and the conventional prefixes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping.
</context>

<requirements>
1. **Add the three frontmatter accessors** in `pkg/domain/task_frontmatter_metrics.go`, immediately after `ClearMetricsInteractionCount` (line 123), mirroring the interaction-count trio exactly and reusing `coerceMetricsInt`:

   ```go
   // MetricsOperatorPromptCount reads "metrics_operator_prompt_count" as *int.
   // Returns nil when the key is absent or the value is not a number — "unknown"
   // must never be forged as 0.
   func (f TaskFrontmatter) MetricsOperatorPromptCount() *int {
   	n, ok := coerceMetricsInt(f.Get("metrics_operator_prompt_count"))
   	if !ok {
   		return nil
   	}
   	return &n
   }

   // SetMetricsOperatorPromptCount stores "metrics_operator_prompt_count".
   func (f *TaskFrontmatter) SetMetricsOperatorPromptCount(count int) {
   	f.Set("metrics_operator_prompt_count", count)
   }

   // ClearMetricsOperatorPromptCount removes the "metrics_operator_prompt_count" key entirely.
   func (f *TaskFrontmatter) ClearMetricsOperatorPromptCount() {
   	f.Delete("metrics_operator_prompt_count")
   }
   ```

2. **Extend the counter in `pkg/ops/interaction_count.go`.**

   a. Add a second method to the exported interface, with a doc comment on each method. Keep the existing `Count` contract word-for-word:

   ```go
   type InteractionCounter interface {
   	// Count returns the total number of type:"user" entries across the session JSONL
   	// logs for the given session ids. Missing, unreadable, malformed, or unsafe session
   	// ids contribute 0. It never returns an error and never blocks completion.
   	Count(ctx context.Context, sessionIDs []string) int

   	// CountOperatorPrompts returns the number of entries across the session JSONL
   	// logs for the given session ids that the operator typed: user turns that are
   	// not tool results, not isMeta injections, not non-human turn origins, not
   	// system prompt sources, and not one of the known legacy noise prefixes.
   	// Missing, unreadable, malformed, or unsafe session ids contribute 0. It never
   	// returns an error and never blocks completion.
   	CountOperatorPrompts(ctx context.Context, sessionIDs []string) int
   }
   ```

   b. Replace the body of `Count` with a delegation to one shared traversal so both measures get identical dedup, unsafe-id rejection and `ctx` handling:

   ```go
   // Count returns the total number of user turns across the session logs for the
   // given session ids, counting each distinct id once. An unsafe id is skipped
   // before any path is built.
   func (c *interactionCounter) Count(ctx context.Context, sessionIDs []string) int {
   	return c.sum(ctx, sessionIDs, countUserTurnsInSessionLog)
   }

   // CountOperatorPrompts returns the number of operator-typed prompts across the
   // session logs for the given session ids, counting each distinct id once.
   func (c *interactionCounter) CountOperatorPrompts(
   	ctx context.Context,
   	sessionIDs []string,
   ) int {
   	return c.sum(ctx, sessionIDs, countOperatorPromptsInSessionLog)
   }

   // sum walks the distinct, safe session ids in order and adds one per-file count
   // for each. countFile is either the all-user counter or the operator-prompt
   // counter, so both measures share the same traversal rules.
   func (c *interactionCounter) sum(
   	ctx context.Context,
   	sessionIDs []string,
   	countFile func(context.Context, string) int,
   ) int {
   	total := 0
   	seen := make(map[string]struct{}, len(sessionIDs))
   	for _, id := range sessionIDs {
   		select {
   		case <-ctx.Done():
   			return total
   		default:
   		}
   		if !isSafeSessionID(id) {
   			continue
   		}
   		if _, ok := seen[id]; ok {
   			continue
   		}
   		seen[id] = struct{}{}
   		path := filepath.Join(c.projectsDir, encodeProjectDir(c.sessionDir), id+".jsonl")
   		total += countFile(ctx, path)
   	}
   	return total
   }
   ```

   c. Add the operator-prompt predicate and its per-file counter to the same file. Use exactly these shapes (valid Go, no new imports beyond what the file already has):

   ```go
   // operatorPromptSources are the promptSource values that identify an
   // operator-typed prompt. Every other non-empty value (notably "system") is
   // excluded.
   var operatorPromptSources = map[string]struct{}{
   	"typed":              {},
   	"queued":             {},
   	"suggestion_accepted": {},
   }

   // operatorPromptNoisePrefixes are the legacy transcript rows that carry none of
   // the signal fields and are not operator prompts. A legacy row whose text starts
   // with one of these is excluded. Slash-command markers are deliberately absent:
   // <command-name> and <command-message> rows are operator-typed and count.
   var operatorPromptNoisePrefixes = []string{
   	"<local-command-stdout>",
   	"<local-command-caveat>",
   	"Caveat: The messages below",
   	"This session is being continued",
   	"[Request interrupted by user]",
   	"<task-notification>",
   	"Another Claude session sent a message:",
   }

   // operatorPromptEntry is the subset of a session JSONL line the operator-prompt
   // predicate reads. A field left absent decodes to its zero value; ToolUseResult
   // is a RawMessage so presence (including a literal null) is distinguishable.
   type operatorPromptEntry struct {
   	Type          string          `json:"type"`
   	IsMeta        bool            `json:"isMeta"`
   	TurnOrigin    string          `json:"turnOrigin"`
   	PromptSource  string          `json:"promptSource"`
   	ToolUseResult json.RawMessage `json:"toolUseResult"`
   	Message       struct {
   		Content json.RawMessage `json:"content"`
   	} `json:"message"`
   }

   // countOperatorPromptsInSessionLog counts the operator-typed prompts in one
   // session JSONL log. A missing, unreadable, or malformed log contributes 0 and
   // never fails; the log is streamed line by line so a very large session never
   // loads fully into memory.
   func countOperatorPromptsInSessionLog(ctx context.Context, path string) int {
   	f, err := os.Open(path) //#nosec G304 -- session ids are validated by isSafeSessionID; read-only best-effort
   	if err != nil {
   		return 0
   	}
   	defer f.Close()

   	count := 0
   	scanner := bufio.NewScanner(f)
   	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
   	for scanner.Scan() {
   		select {
   		case <-ctx.Done():
   			return count
   		default:
   		}
   		var entry operatorPromptEntry
   		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
   			continue
   		}
   		if entry.Type != "user" {
   			continue
   		}
   		if isOperatorPrompt(entry) {
   			count++
   		}
   	}
   	return count
   }

   // isOperatorPrompt reports whether one type:"user" entry is a prompt the
   // operator typed, applying the measured predicate.
   func isOperatorPrompt(entry operatorPromptEntry) bool {
   	if len(entry.ToolUseResult) > 0 {
   		return false
   	}
   	text, hasToolResult := operatorPromptText(entry.Message.Content)
   	if hasToolResult {
   		return false
   	}
   	if entry.IsMeta {
   		return false
   	}
   	if entry.TurnOrigin != "" && entry.TurnOrigin != "human" {
   		return false
   	}
   	if entry.PromptSource != "" {
   		if _, ok := operatorPromptSources[entry.PromptSource]; !ok {
   			return false
   		}
   	}
   	legacy := entry.TurnOrigin == "" && entry.PromptSource == ""
   	if legacy && hasOperatorPromptNoisePrefix(text) {
   		return false
   	}
   	return true
   }

   // operatorPromptText returns the entry's text and whether its content carries a
   // tool_result block. Content may be a plain string or an array of blocks; only
   // text blocks contribute to the returned text.
   func operatorPromptText(content json.RawMessage) (string, bool) {
   	if len(content) == 0 {
   		return "", false
   	}
   	var text string
   	if err := json.Unmarshal(content, &text); err == nil {
   		return text, false
   	}
   	var blocks []struct {
   		Type string `json:"type"`
   		Text string `json:"text"`
   	}
   	if err := json.Unmarshal(content, &blocks); err != nil {
   		return "", false
   	}
   	var builder strings.Builder
   	for _, block := range blocks {
   		if block.Type == "tool_result" {
   			return "", true
   		}
   		if block.Type == "text" {
   			builder.WriteString(block.Text)
   		}
   	}
   	return builder.String(), false
   }

   // hasOperatorPromptNoisePrefix reports whether a legacy row's text starts with
   // one of the known non-operator prefixes.
   func hasOperatorPromptNoisePrefix(text string) bool {
   	trimmed := strings.TrimSpace(text)
   	for _, prefix := range operatorPromptNoisePrefixes {
   		if strings.HasPrefix(trimmed, prefix) {
   			return true
   		}
   	}
   	return false
   }
   ```

   d. Do **not** change `countUserTurnsInSessionLog`, `isSafeSessionID`, `encodeProjectDir`, `metricsSessionIDs`, `earliestStartedAt`, or the `Count` semantics. `Count` must still count every `type:"user"` line.

   e. Regenerate the Counterfeiter mock by running `make generate` (it removes `mocks/` and regenerates from the `//go:generate ... -generate` runner in `pkg/ops/ops_suite_test.go`, which scans the package's `//counterfeiter:generate` directives — the `InteractionCounter` directive is at `pkg/ops/interaction_count.go:20`). Do not hand-edit `mocks/interaction-counter.go`; the regenerated file must gain `CountOperatorPrompts`, its `Returns`/`CallCount`/`ArgsForCall` helpers, and its `Invocations()` map entry.

3. **Write the new field at completion** in `pkg/ops/complete.go`:

   a. In `applyCompletionMetrics`, count the session ids once and write both fields in the same mutation, keeping the `len(sessions) > 0` guard so a never-worked task gets neither count:

   ```go
   func (c *completeOperation) applyCompletionMetrics(
   	ctx context.Context,
   	task *domain.Task,
   	completedD libtime.DateOrDateTime,
   ) {
   	task.SetMetricsCompletedAt(&completedD)
   	if sessions := task.MetricsSessions(); len(sessions) > 0 {
   		sessionIDs := metricsSessionIDs(sessions)
   		task.SetMetricsInteractionCount(c.interactionCounter.Count(ctx, sessionIDs))
   		task.SetMetricsOperatorPromptCount(
   			c.interactionCounter.CountOperatorPrompts(ctx, sessionIDs),
   		)
   	}
   }
   ```

   Update the function's doc comment to name both fields and to keep the rule that an unworked task keeps both counts unknown — never forged as zero.

   b. In the recurring path (`handleRecurringTask`, beside `task.ClearMetricsInteractionCount()` at line 305), add `task.ClearMetricsOperatorPromptCount()` so a recurring task's accumulator never carries a stale operator count into the next cycle. Do **not** add the operator count to `domain.MetricsCycle` or to `AppendMetricsCycle` — archived cycles are out of scope.

   c. No other call site changes: `pkg/cli/cli.go` already injects one `InteractionCounter` through `completeInteractionCounter` and `NewCompleteOperation`, and the extended interface is satisfied by the same object.

4. **Compute and expose the weekly operator total** in `pkg/ops/rollup_weekly.go`:

   a. Add a field to `RollupWeeklyResult` immediately after `HumanInteractions`, with the JSON key `operator_prompts`:

   ```go
   HumanInteractions    string          `json:"human_interactions"`
   OperatorPrompts      string          `json:"operator_prompts"`
   UnattendedDeliveries string          `json:"unattended_deliveries"`
   ```

   b. Add a sum function beside `rollupFigures`, leaving `rollupFigures` itself untouched (its `recorded` flag and its unattended rule are unchanged):

   ```go
   // rollupOperatorPromptTotal returns the week's operator-prompt sum and whether
   // any member carries a recorded operator count. An absent or malformed count is
   // not a measurement; a recorded zero is.
   func rollupOperatorPromptTotal(tasks []*domain.Task) (int, bool) {
   	var total int
   	recorded := false
   	for _, task := range tasks {
   		count := task.MetricsOperatorPromptCount()
   		if count == nil {
   			continue
   		}
   		recorded = true
   		total += *count
   	}
   	return total, recorded
   }
   ```

   c. In `Execute`, set the field in both branches:
   - the `len(weekTasks) == 0` branch: `result.OperatorPrompts = rollupNoData`;
   - the non-empty branch: call `rollupOperatorPromptTotal(weekTasks)` and assign `strconv.Itoa(total)` when `recorded`, otherwise `rollupNoRecordedCounts`. Use a separate `recorded` flag from the interaction count's — a week can have interaction counts and no operator counts, and that week must print `no recorded counts`, never `0`.
   - Update `Execute`'s doc comment from "the three figures" to "the four figures".
   - Leave the per-family medians, the baseline block and the delta block untouched. `operator_prompts` has no baseline analogue and produces no delta row.

5. **Render the new figure** in `pkg/cli/rollup.go`'s `writeRollupWeeklyFigures`, directly beneath the interaction line so the two sit next to each other:

   ```go
   func writeRollupWeeklyFigures(builder *strings.Builder, result ops.RollupWeeklyResult) {
   	fmt.Fprintf(builder, "Human interactions: %s\n", result.HumanInteractions)
   	fmt.Fprintf(builder, "Operator prompts: %s\n", result.OperatorPrompts)
   	fmt.Fprintf(builder, "Unattended deliveries: %s\n", result.UnattendedDeliveries)
   	fmt.Fprintf(builder, "Per-family median: %s\n", result.PerFamilyMedian)
   }
   ```

   Update the function's doc comment from "three headline figures" to "four headline figures". Also update the stale count in `createRollupWeeklyCommand`'s doc comment in the same file ("The three figures are properties of one task population") to "four figures". Every value is still written verbatim — `no data`, `no recorded counts` and `undefined` are never replaced by `0`.

6. **Regression fixture and falsification** in `pkg/ops/interaction_count_test.go`, inside the existing `Describe("InteractionCounter", …)` so it reuses `ctx`, `counter` and `writeFixture`:

   a. Add a spec whose name contains the exact phrase `operator prompts` (the verification focuses on that phrase) and which writes one fixture session log with one line per entry kind. Use exactly these lines (the fixture is the contract):

   ```go
   operatorPromptFixture := []string{
   	// counted — 7 lines
   	`{"type":"user","message":{"role":"user","content":"fix the flaky test"},"promptSource":"typed"}`,
   	`{"type":"user","message":{"role":"user","content":"also update the docs"},"promptSource":"queued"}`,
   	`{"type":"user","message":{"role":"user","content":"run make test"},"promptSource":"suggestion_accepted"}`,
   	`{"type":"user","message":{"role":"user","content":"check the log"},"turnOrigin":"human"}`,
   	`{"type":"user","message":{"role":"user","content":"legacy typed prompt"}}`,
   	`{"type":"user","message":{"role":"user","content":"<command-name>/compact</command-name>"}}`,
   	`{"type":"user","message":{"role":"user","content":"<command-message>vault-cli:work-on-task</command-message>"}}`,
   	// excluded — 14 type:"user" lines
   	`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"output"}]}}`,
   	`{"type":"user","toolUseResult":{"stdout":"x"},"message":{"role":"user","content":[{"type":"text","text":"tool output"}]}}`,
   	`{"type":"user","isMeta":true,"message":{"role":"user","content":"injected system prompt"}}`,
   	`{"type":"user","turnOrigin":"scheduled","message":{"role":"user","content":"scheduled wakeup"}}`,
   	`{"type":"user","turnOrigin":"task_notification","message":{"role":"user","content":"background task finished"}}`,
   	`{"type":"user","turnOrigin":"peer","message":{"role":"user","content":"peer message"}}`,
   	`{"type":"user","promptSource":"system","message":{"role":"user","content":"<task-notification>done</task-notification>"}}`,
   	`{"type":"user","message":{"role":"user","content":"<local-command-stdout>Compacted</local-command-stdout>"}}`,
   	`{"type":"user","message":{"role":"user","content":"<local-command-caveat>caveat</local-command-caveat>"}}`,
   	`{"type":"user","message":{"role":"user","content":"Caveat: The messages below were generated by the user"}}`,
   	`{"type":"user","message":{"role":"user","content":"This session is being continued from a previous conversation"}}`,
   	`{"type":"user","message":{"role":"user","content":"[Request interrupted by user]"}}`,
   	`{"type":"user","message":{"role":"user","content":"<task-notification>done</task-notification>"}}`,
   	`{"type":"user","message":{"role":"user","content":"Another Claude session sent a message: hi"}}`,
   	// excluded — not a user turn
   	`{"type":"assistant","message":{"role":"assistant","content":"x"}}`,
   	`not json at all {`,
   }
   ```

   Assert, in the same spec:
   - `counter.CountOperatorPrompts(ctx, []string{"s1"})` equals **7** — only the operator-typed prompts;
   - `counter.Count(ctx, []string{"s1"})` equals **21** — the 7 counted lines plus the 14 excluded `type:"user"` lines, proving the interaction count still counts every user turn (requirement 2d and the frozen-interaction-count constraint);
   - `counter.CountOperatorPrompts(ctx, []string{"missing"})` equals 0, and `counter.CountOperatorPrompts(ctx, []string{"../x"})` equals 0, matching the existing missing-file and unsafe-id guards.

   b. **Falsification check — the fixture must fail if the filter is loosened.** Perform it with filesystem copies, never `git` (the container's git state is not guaranteed and a dead `git` command still leaves a false pass):

   1. `cp pkg/ops/interaction_count.go /tmp/interaction_count.go.fixed`
   2. Edit `CountOperatorPrompts` so its body is exactly `return c.Count(ctx, sessionIDs)` (the all-user filter), leaving the method signature and the doc comment in place.
   3. Run the focused spec and confirm it **FAILS** (non-zero exit, got 21, want 7), capturing the output so the failure is mechanically checkable:
      ```
      go test ./pkg/ops/... -count=1 -ginkgo.focus='operator prompts' > /tmp/falsify.log 2>&1
      grep -nE 'Expected.*(21|7)' /tmp/falsify.log
      ```
      The grep must print the 21-vs-7 mismatch. If it prints nothing, the spec did not run or did not discriminate — do not proceed. A passing run means the fixture does not discriminate and the prompt is not done. Do not weaken the assertion to make it pass; steps 4-6 restore the file.
   4. Restore: `cp /tmp/interaction_count.go.fixed pkg/ops/interaction_count.go`
   5. Confirm the restore: `grep -n 'func (c \*interactionCounter) CountOperatorPrompts' pkg/ops/interaction_count.go` matches, `grep -n 'return c.sum(ctx, sessionIDs, countOperatorPromptsInSessionLog)' pkg/ops/interaction_count.go` matches, and `grep -n 'return c.Count(ctx, sessionIDs)' pkg/ops/interaction_count.go` prints nothing.
   6. Re-run the focused spec and confirm it now **PASSES**.

   The restored file is what gets committed — a tree left in the sabotaged state is a failed prompt.

7. **Unit tests for the remaining layers.**

   a. `pkg/domain/task_frontmatter_metrics_test.go` — add a `Describe("MetricsOperatorPromptCount", …)` beside the interaction-count block asserting: absent key → `nil`; `SetMetricsOperatorPromptCount(0)` → non-nil `0` (a recorded zero is a measurement); the same coercion table over int / int64 / float64 / numeric string / non-numeric string / bool / nil, keyed on `metrics_operator_prompt_count`; `ClearMetricsOperatorPromptCount` deletes the key. Extend the YAML round-trip spec to set the new field and include `metrics_operator_prompt_count:` in its key list.

   b. `pkg/ops/complete_test.go` — in `Context("non-recurring worked task records interaction count", …)`: add `mockInteractionCounter.CountOperatorPromptsReturns(3)` to its `BeforeEach`, and assert in its `It` that `writtenTask.MetricsOperatorPromptCount()` is non-nil and equals 3, that `CountOperatorPromptsCallCount()` is 1, and that its session ids are `[]string{"s1"}`. In `Context("non-recurring task with no metrics sessions", …)`: assert `writtenTask.MetricsOperatorPromptCount()` is `nil` and `CountOperatorPromptsCallCount()` is 0. In `Context("recurring worked task archives a cycle", …)`: set `task.SetMetricsOperatorPromptCount(9)` in the `BeforeEach` and assert in the `It` that `writtenTask.Get("metrics_operator_prompt_count")` is `nil` after the recurring path clears the accumulator. In `Context("recurring task with no metrics sessions", …)`: assert the key stays `nil`.

   c. `pkg/ops/rollup_weekly_test.go` — add a helper beside `rollupTaskFrontmatter` that appends the operator key (an empty value omits the key entirely):

   ```go
   // rollupOperatorTaskFrontmatter extends rollupTaskFrontmatter with the operator
   // prompt count. An empty operator omits the key entirely — absent, not zero.
   func rollupOperatorTaskFrontmatter(status, completedAt, count, operator string) string {
   	frontmatter := rollupTaskFrontmatter(status, completedAt, count)
   	if operator != "" {
   		frontmatter += "metrics_operator_prompt_count: " + operator + "\n"
   	}
   	return frontmatter
   }
   ```

   Add specs whose names contain `operator prompts` asserting:
   - the operator total sums over the week's task set (two tasks at 2 and 3 → `"5"`) while `HumanInteractions` keeps its own sum, and a task outside the week does not contribute;
   - a week with interaction counts but no operator counts reports `OperatorPrompts == "no recorded counts"` and not `"0"`, while `HumanInteractions` is still its numeric sum;
   - a recorded operator zero is a measurement: operator `"0"` plus operator `"4"` sums to `"4"`, and an absent operator key is excluded;
   - update `assertRollupNoData` to assert `result.OperatorPrompts == "no data"` and `!= "0"`;
   - add `operator_prompts` to the frozen-JSON-key spec's key list (line 366).

   d. `pkg/cli/rollup_test.go` — update both exact-output golden strings (the plain report at line 37 and the baseline report at line 166) to include the new `Operator prompts: <value>` line directly after `Human interactions: <value>`, set `OperatorPrompts` on those `RollupWeeklyResult` literals, and rename the first spec from `renders the three figures, the family block and both rules` to `renders the four figures, the family block and both rules`. Update the no-data spec to assert `Operator prompts: no data`. Add a spec whose name contains `operator prompts` and which asserts that a result carrying `OperatorPrompts: "no recorded counts"` renders exactly `Operator prompts: no recorded counts` and never `Operator prompts: 0`.

   e. `integration/cli_test.go` — in `Describe("vault-cli rollup weekly", …)`: add `metrics_operator_prompt_count: 2` to the `Rollup Fixture A` frontmatter and assert the plain output matches `(?m)^Operator prompts: ` and contains `Operator prompts: 2`; rename the first `It` from "three figures" to "four figures" and the JSON-comparison `It` from "same three figures" to "same four figures"; in the no-data spec assert the output contains `Operator prompts: no data` and does not match `(?m)^Operator prompts: 0$`; in the plain/JSON comparison spec add `OperatorPrompts string \`json:"operator_prompts"\`` to the local struct, assert it is non-empty, and assert the plain output contains `Operator prompts: ` + that value.

8. **Update `README.md`** (lines 221-229): change "reports three figures" to "four figures" and describe the operator-prompt figure — the number of prompts the operator actually typed, the sum of `metrics_operator_prompt_count` over the week's tasks, excluding tool results, meta injections, scheduled wakeups, task notifications and peer messages. State that a week with no recorded operator counts reads `no recorded counts`, never `0`. Leave the baseline/delta paragraphs unchanged.

   8b. **Update `commands/complete-task.md` § Passive metrics** (lines 101-105) so it names `metrics_operator_prompt_count` beside `metrics_interaction_count` and states the one-line predicate: counts only operator-typed prompts, excluding tool results, `isMeta` injections, non-human `turnOrigin` values, `system` `promptSource`, and the legacy noise prefixes. That file is the repo's canonical description of what completion writes; leaving it enumerating only the old field makes it wrong.

9. **Add one `## Unreleased` `feat:` bullet to `CHANGELOG.md`.** The section does not exist today: create it below the preamble's last line (`* PATCH version when you make backwards-compatible bug fixes.`) and above `## v0.171.1` (or above whatever the newest `## vX.Y.Z` heading is if a release landed meanwhile). One flat bullet, no category heading, no date suffix, ending with a `Change set:` clause. It must carry: the new `metrics_operator_prompt_count` field; that it counts only operator-typed prompts with the predicate summary (tool results, `isMeta` injections, non-human `turnOrigin` values, `system` `promptSource`, and the legacy noise prefixes excluded; `<command-name>` / `<command-message>` slash commands counted); that it is written beside `metrics_interaction_count` at completion and absent when the task has no `metrics_sessions`; that `metrics_interaction_count` is unchanged; and that `vault-cli rollup weekly` prints `Operator prompts` beside `Human interactions` with the existing `no data` / `no recorded counts` sentinels.

10. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation. Walk the acceptance criteria: the fixture proves only operator-typed prompts are counted and the interaction count is unchanged (7 vs 21); the falsification step failed against the loosened filter and passed once restored; `applyCompletionMetrics` writes the new field and omits it for a task with no `metrics_sessions`; the rollup prints the total and the sentinel; the CHANGELOG bullet is under `## Unreleased`; `make precommit` and `make test` are green.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage, commit, tag or push.
- Do NOT bump any version string. `.dark-factory.yaml` sets `autoRelease: false`, so no releaser will cut this change; the operator owns the bump. Do not edit `CHANGELOG.md`'s newest `## vX.Y.Z` section, `.claude-plugin/plugin.json`, or either version field in `.claude-plugin/marketplace.json`, and do not run `git tag`, `make release-check` or `make check-versions`.
- Do NOT hand-edit `mocks/interaction-counter.go`. `make generate` (run by `make precommit`) deletes and regenerates the whole `mocks/` tree from the `//counterfeiter:generate` directives; a hand-written method will be wiped and a stale mock will fail the build.
- `metrics_interaction_count` behaviour is frozen: every `type:"user"` line still counts, the same session-id dedup and unsafe-id guard apply, and the unattended-delivery rule (`completed` + recorded interaction count exactly `0`) is unchanged. Do not touch `rollupFigures`' `recorded` flag or its unattended branch.
- "Absent is unknown, never forged as zero" is the load-bearing invariant on both counts: a task with no `metrics_sessions` gets neither field; an absent or malformed stored value reads `nil`; the rollup prints `no recorded counts` / `no data`, never `0`, for a missing measurement. A recorded zero IS a measurement and must be written and summed as `0`.
- Do NOT add `metrics_operator_prompt_count` to `domain.MetricsCycle`, `AppendMetricsCycle`, `coerceMetricsCycle`, the baseline file, the delta block, or the per-family medians. Archived cycles and the baseline are out of scope; the new field is an active accumulator and a rollup headline only.
- Do NOT add a config knob, a CLI flag, or a new command. The measurement is passive, derived from the session logs the task already records.
- `pkg/ops/` stays output-free: no `fmt.Print*`, no `os.Stdout`; the CLI layer owns every printed byte.
- Do not change `countUserTurnsInSessionLog`, `isSafeSessionID`, `encodeProjectDir`, `metricsSessionIDs` or `earliestStartedAt`.
- The regression-lock revert in requirement 6b is temporary and the restore is mandatory. `pkg/ops/interaction_count.go` must be back to its fixed form before the prompt ends; `<verification>` greps for it and runs `make precommit` last.
- Tests follow repository convention: Ginkgo v2 / Gomega, external `_test` packages, Counterfeiter mocks. New exported functions and interface methods carry doc comments; error paths wrap with `github.com/bborbe/errors` (none of the new paths return errors — the counters are best-effort by contract).
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…` and the `/tmp` scratch copy used by the falsification step, both of which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
- Existing tests must still pass, including every spec already in `pkg/ops/interaction_count_test.go`, `pkg/ops/complete_test.go`, `pkg/ops/rollup_weekly_test.go`, `pkg/cli/rollup_test.go` and `integration/`.
</constraints>

<verification>
Run each of these and confirm the printed result against its expectation. The daemon does not gate on exit codes, so read the output.

**PRIMARY GATE — the predicate is in place and restored after the falsification revert.**

```
grep -n 'CountOperatorPrompts' pkg/ops/interaction_count.go pkg/ops/complete.go mocks/interaction-counter.go
grep -n 'countOperatorPromptsInSessionLog\|isOperatorPrompt\|operatorPromptNoisePrefixes' pkg/ops/interaction_count.go
grep -n 'func (c \*interactionCounter) CountOperatorPrompts' pkg/ops/interaction_count.go
grep -n 'return c.sum(ctx, sessionIDs, countOperatorPromptsInSessionLog)' pkg/ops/interaction_count.go
grep -n 'return c.Count(ctx, sessionIDs)' pkg/ops/interaction_count.go   # must print nothing
grep -n 'metrics_operator_prompt_count' pkg/domain/task_frontmatter_metrics.go
grep -n 'SetMetricsOperatorPromptCount\|ClearMetricsOperatorPromptCount' pkg/ops/complete.go
grep -n 'OperatorPrompts\|operator_prompts' pkg/ops/rollup_weekly.go pkg/cli/rollup.go
grep -n 'Operator prompts:' pkg/cli/rollup.go
```

Expect: both methods on the interface and the mock; the predicate helpers present; the three domain accessors; the set and clear call sites in `complete.go`; the result field with the `operator_prompts` tag; and the literal `Operator prompts:` in the CLI writer.

**TESTS — focused first, then the full suite.**

```
go test ./pkg/ops/... -count=1 -ginkgo.focus='operator prompts' -ginkgo.v
go test ./pkg/domain/... -count=1 -ginkgo.focus='MetricsOperatorPromptCount'
go test ./pkg/cli/... -count=1 -ginkgo.focus='operator prompts'
go test ./pkg/ops/... -count=1
go test ./pkg/domain/... -count=1
go test ./pkg/cli/... -count=1
go test ./integration/... -count=1
make test
make precommit
```

The first command must exit 0 and its output must name at least one spec containing `operator prompts` — a focus string that matches nothing also exits 0 with no spec lines, so read the output for the spec name. Every other command must exit 0. `make precommit` runs `ensure`, `format`, `generate`, `test`, `check` and `addlicense`, so it also proves the regenerated mock compiles and the linter is clean.

**DOCS — README and the canonical completion description.**

```
grep -n 'four figures' README.md
grep -n 'Operator prompts' README.md
grep -n 'metrics_operator_prompt_count' README.md commands/complete-task.md
```

Expect `four figures` in the rollup section; the `Operator prompts` figure described in `README.md`; and the new field named in both `README.md` and `commands/complete-task.md`.

**CHANGELOG — structure and placement.**

```
bash scripts/check-changelog.sh
grep -n '^## ' CHANGELOG.md | head -2
awk '/^## /{sec=$0} /metrics_operator_prompt_count/ {print sec}' CHANGELOG.md | head -1
```

Expect `CHANGELOG structure OK`; then `## Unreleased` followed by the newest `## vX.Y.Z`; then `## Unreleased` from the `awk`. If the `awk` prints nothing, the bullet is missing or does not name the field.

**SYNTAX.**

```
gofmt -e -l pkg/ops/*.go pkg/domain/*.go pkg/cli/*.go integration/*.go
```

Must list no files.

**SELF-CHECK before finishing:** re-run the PRIMARY GATE, the TESTS and the CHANGELOG checks. Confirm the falsification spec failed against `return c.Count(ctx, sessionIDs)` and passed after `cp /tmp/interaction_count.go.fixed pkg/ops/interaction_count.go`; confirm `pkg/ops/interaction_count.go` is in its fixed form — the sabotaged body must not be left in the tree.
</verification>
