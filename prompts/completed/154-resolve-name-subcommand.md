---
status: completed
spec: ["021"]
summary: Added ResolveResult domain type in pkg/domain/resolve_result.go with JSON serialization tests covering task/goal/not-found cases
execution_id: vault-cli-resolve-exec-154-resolve-name-subcommand
dark-factory-version: dev
created: "2026-07-02T10:00:00Z"
queued: "2026-07-02T09:46:53Z"
started: "2026-07-02T09:46:55Z"
completed: "2026-07-02T09:48:27Z"
---

<summary>
- Adds a domain type describing the outcome of a name-resolution probe: which entity type a name matches (task, goal, or neither) and whether it was found
- The type serializes to the JSON contract consumed by the merged `/vault-cli:work-on` slash command router
- No behavior change on its own — this is the shared result shape that the resolve operation (prompt 02) and CLI command (prompt 03) both depend on
- Lives in the domain layer so both the operation and CLI can reference it without importing each other
</summary>

<objective>
Add a `ResolveResult` domain type in `pkg/domain/resolve_result.go` that represents the outcome of resolving a name to a task, a goal, or neither, and serializes to the JSON contract `{"type":"task|goal|","name":"...","found":true|false}`.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface/constructor/struct conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comment style.

Read these files before implementing:
- `docs/development-patterns.md` — layered architecture (domain → ops → CLI).
- `pkg/domain/file_metadata.go` — a simple plain domain struct for style reference (license header, GoDoc comments, no constructor when the struct is a plain data holder).
- `pkg/ops/show.go` lines 41-61 — `TaskDetail` struct: example of a JSON-tagged result struct in this codebase. `ResolveResult` follows the same JSON-tag style but lives in `pkg/domain/`, not `pkg/ops/`.
- `pkg/domain/domain_suite_test.go` — Ginkgo suite bootstrap for the domain package (the test file you add must belong to `package domain_test` and rely on this suite runner).

The JSON contract (from spec 021 AC1–AC3):
- Task match:  `{"type":"task","name":"Existing Task Name","found":true}`
- Goal match:  `{"type":"goal","name":"Existing Goal Name","found":true}`
- Not found:   `{"type":"","name":"Does Not Exist","found":false}`

Note: `type` is an EMPTY STRING (not omitted) when not found. `found` is always present (true or false). `name` echoes the input name.
</context>

<requirements>
1. Create `pkg/domain/resolve_result.go` in `package domain` with the standard 3-line BSD license header (copy it verbatim from the top of `pkg/domain/file_metadata.go`).

2. Define the result struct exactly as follows:
   ```go
   // ResolveResult is the outcome of resolving a name to a task, a goal, or neither.
   // It is the JSON contract consumed by slash commands to auto-detect entity type.
   type ResolveResult struct {
       // Type is "task", "goal", or "" (empty string when not found).
       Type  string `json:"type"`
       // Name echoes the input name that was resolved.
       Name  string `json:"name"`
       // Found reports whether the name matched a task or a goal.
       Found bool   `json:"found"`
   }
   ```
   - All three JSON tags MUST be present WITHOUT `,omitempty` — the contract requires `type:""` and `found:false` to appear literally in the output (see spec AC3). `omitempty` would drop them.

3. Do NOT add a constructor, `Validate()` method, interface, or Counterfeiter annotation. This is a plain data holder like `FileMetadata` — the operation (prompt 02) populates the fields directly.

4. Add `pkg/domain/resolve_result_test.go` in `package domain_test`:
   - Import `encoding/json`, the Ginkgo v2 / Gomega dot-imports (`. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`), and `"github.com/bborbe/vault-cli/pkg/domain"`.
   - This test crosses the JSON serialization boundary — it MUST marshal real `ResolveResult` values and assert the exact byte output, because that byte shape is the machine contract (a struct-equality test would NOT catch a stray `omitempty`).
   - Table-test these three cases with `json.Marshal` and assert the exact string:
     - `ResolveResult{Type: "task", Name: "Existing Task Name", Found: true}` → `{"type":"task","name":"Existing Task Name","found":true}`
     - `ResolveResult{Type: "goal", Name: "Existing Goal Name", Found: true}` → `{"type":"goal","name":"Existing Goal Name","found":true}`
     - `ResolveResult{Type: "", Name: "Does Not Exist", Found: false}` → `{"type":"","name":"Does Not Exist","found":false}`
   - Assert with `Expect(string(bytes)).To(Equal(expected))` so field ORDER and presence are both pinned.

5. Run `go test -mod=vendor ./pkg/domain/...` and confirm the new test passes.
</requirements>

<constraints>
- Layered architecture: this type lives in `pkg/domain/` — the foundational layer. Do NOT put it in `pkg/ops/` or `pkg/cli/`.
- No new dependencies, no new interfaces, no constructor — plain struct only.
- Ginkgo v2 / Gomega for tests; external test package (`package domain_test`).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

Acceptance criteria advanced by this prompt (from spec 021):
- [ ] AC1 — Task match JSON shape `{"type":"task",...,"found":true}` (this prompt defines the serialized shape; the CLI wiring in prompt 03 produces it end-to-end)
- [ ] AC2 — Goal match JSON shape `{"type":"goal",...,"found":true}`
- [ ] AC3 — Not-found JSON shape `{"type":"","name":"...","found":false}` — `type` is empty string, `found` is false, both present
</constraints>

<verification>
Run `make test` — must pass.
Run `go test -mod=vendor ./pkg/domain/...` — the new marshal round-trip test passes.
Run `grep -rn "ResolveResult" pkg/domain/` — struct and test exist.
</verification>
