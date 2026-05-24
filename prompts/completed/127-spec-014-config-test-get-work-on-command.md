---
status: completed
spec: [014-bug-work-on-silent-failure-and-hardcoded-slash-command]
summary: Added JSON marshalling tests for work_on_command tag verification in vault_test.go
container: vault-cli-exec-127-spec-014-config-test-get-work-on-command
dark-factory-version: v0.171.1-3-gd94f1fa
created: "2026-05-24T14:31:00Z"
queued: "2026-05-24T14:24:43Z"
started: "2026-05-24T14:26:53Z"
completed: "2026-05-24T14:28:39Z"
branch: dark-factory/bug-work-on-silent-failure-and-hardcoded-slash-command
---

<summary>
- Table-driven tests for `GetWorkOnCommand()` covering both branches (custom value and empty/default)
- Verifies custom command is returned when set
- Verifies default `/vault-cli:work-on-task` is returned when empty
- Verifies the new struct field's serialization tags via JSON round-trip — catches yaml/json tag typos that no getter test would catch (boundary contract on the marshaller)
- Follows the existing Vault getter test pattern AND the existing `JSON marshalling` block for the marshalling assertions
</summary>

<objective>
Add test coverage for the new `GetWorkOnCommand()` method on the `Vault` struct and for the `work_on_command` JSON serialization tag, following the established test patterns in `pkg/config/vault_test.go`.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes:
- `pkg/config/vault_test.go` — existing test patterns: (a) Vault getters block (e.g. `GetClaudeScript` Describe) and (b) `JSON marshalling` Describe block (the `knowledge_dir` round-trip is the canonical reference for tag-substring assertions)
- `pkg/config/config.go` — `Vault` struct and `GetWorkOnCommand()` method added by sibling prompt `spec-014-config-field-work-on-command.md`
</context>

<requirements>
1. In `pkg/config/vault_test.go`, add a test block for `GetWorkOnCommand` after the `GetSessionProjectDir` tests (around line 123), following the existing pattern:

   ```go
   Describe("GetWorkOnCommand", func() {
       It("returns custom work-on command when set", func() {
           vault := &config.Vault{WorkOnCommand: "/my-custom-command"}
           Expect(vault.GetWorkOnCommand()).To(Equal("/my-custom-command"))
       })

       It("returns default /vault-cli:work-on-task when empty", func() {
           vault := &config.Vault{}
           Expect(vault.GetWorkOnCommand()).To(Equal("/vault-cli:work-on-task"))
       })
   })
   ```

2. The test structure follows the same pattern as `GetClaudeScript` (lines 101-111):
   - One test for when field is set
   - One test for when field is empty (default)

3. Use `config.Vault` directly (already imported in the test file).

4. In the existing `JSON marshalling` Describe block in the same file, add two `It` blocks following the `knowledge_dir` pattern verbatim — this closes the boundary contract on the new struct tag:

   ```go
   It("includes work_on_command in JSON when set", func() {
       vault := config.Vault{Name: "main", Path: "/vault", WorkOnCommand: "/cmd"}
       data, err := json.Marshal(vault)
       Expect(err).To(BeNil())
       Expect(string(data)).To(ContainSubstring(`"work_on_command":"/cmd"`))
   })

   It("omits work_on_command from JSON when empty", func() {
       vault := config.Vault{Name: "main", Path: "/vault"}
       data, err := json.Marshal(vault)
       Expect(err).To(BeNil())
       Expect(string(data)).NotTo(ContainSubstring("work_on_command"))
   })
   ```

   Rationale: a typo in the struct tag (e.g. `work_on_commnd`) would never be caught by the getter tests but would silently break config-file deserialization. The substring assertion `"work_on_command":"/cmd"` pins the tag string.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Follow the exact existing test pattern for consistency
</constraints>

<verification>
Run `make precommit` — must pass.
Grep verification:
- `grep -n 'GetWorkOnCommand' pkg/config/vault_test.go` returns the test block
- `grep -n 'work_on_command' pkg/config/vault_test.go` returns ≥2 lines (one for "includes", one for "omits")
- `go test ./pkg/config/... -run GetWorkOnCommand -v` lists the two getter cases as passing
- `go test ./pkg/config/... -v` includes the two new JSON marshalling cases for `work_on_command`
</verification>