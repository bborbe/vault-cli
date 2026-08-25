---
status: approved
spec: [039-bug-completed-requires-closeout-fields]
created: "2026-08-25T07:35:47Z"
queued: "2026-08-25T08:09:37Z"
branch: dark-factory/bug-completed-requires-closeout-fields
---

<summary>
- `docs/task-writing.md` § Lifecycle no longer claims completion requires the close-out fields: the `completed` table row drops the "requires close-out fields" wording and the "Close-out fields" block is re-scoped to `aborted` only.
- All eight `--reason` / `--gate-successor` help strings in `pkg/cli/cli.go` are reworded so no help text claims completion requires the fields; the `set`-command help still states the fields are required for the `aborted` transition.
- The completed spec-037 record gains an appended supersession note (below its existing "Verification Result" section) naming this spec; no existing body text is modified.
- `CHANGELOG.md` gains a `## Unreleased` entry (above `## v0.116.1`) recording the fix and the reversal of the 037 breaking change for completion.
- After this prompt, `grep -n 'required to complete' pkg/cli/cli.go` returns 0 lines and `make precommit` passes.
</summary>

<objective>
Bring every user-facing statement — the live lifecycle doc, the CLI help text, the spec-037 record, and the changelog — into agreement with the new contract from prompt 04: `completed` transitions never require `aborted_reason` / `gate_successor` (the fields remain optional on completion and stay required for `aborted`).
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions (including the Plugin Release Checklist and the changelog `## Unreleased` rules), `/workspace/docs/dod.md` for the Definition of Done (it pins the `## Unreleased` placement: below the preamble, above the newest `## vX.Y.Z`), and `/workspace/docs/task-writing.md` § Lifecycle — the live doc being edited.

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format (prefix-required bullet, `fix:` for this change)
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md` — doc-edit style

Read these files fully before making changes:

- `/workspace/docs/task-writing.md` — § Lifecycle (lines 312-330). Lines 319, 323, 325, 326, and 328 carry the claims this spec supersedes. Do NOT touch any other section of the doc (phase transitions, hold/in_progress, calendar-date rule, etc. are out of scope).
- `/workspace/pkg/cli/cli.go` — four `cmd.Flags().StringVar` pairs whose help text names the close-out fields: lines 225-226 (`task complete`), 973-974 (`goal set` in the entity-set command), 1405-1406 (`goal complete`), 2129-2130 (`task set`). Only the two help strings at each site change — nothing else in the file.
- `/workspace/specs/completed/037-mandatory-abort-reason-and-gate-successor.md` — the immutable 037 record. Its body is NOT to be rewritten; the supersession note is appended AFTER the final "Verdict: PASS" line of the existing "## Verification Result" section (the file currently ends there). Verify the current tail before appending so the note lands after all existing content.
- `/workspace/CHANGELOG.md` — currently has no `## Unreleased` section; the newest version section is `## v0.116.1`. The v0.116.0 entry (the 037 release) is what the new entry partially reverses — read it for accurate wording.
- `/workspace/scripts/check-changelog.sh` — enforces that no `## ` section appears above the preamble; the new `## Unreleased` must sit between the preamble (`* PATCH version when you make backwards-compatible bug fixes.`) and `## v0.116.1`.

Do NOT touch any Go source other than the eight help strings in `pkg/cli/cli.go` — this prompt is the doc/surface-text rung and assumes prompt 04 (the behavior change) has shipped. If the behavior change is NOT present (e.g. `pkg/domain/task_frontmatter.go` still gates `completed`), STOP and report `Status: failed` with message `"guard split not yet deployed (prompt 04)"`.
</context>

<requirements>
1. In `/workspace/docs/task-writing.md` § Lifecycle:

   a. Replace the `completed` row (line 319):
   ```
   | `completed` | All success criteria met | `/vault-cli:complete-task` — checks every `# Success Criteria` checkbox is `[x]`; requires close-out fields (see below) |
   ```
   with:
   ```
   | `completed` | All success criteria met | `/vault-cli:complete-task` — checks every `# Success Criteria` checkbox is `[x]`; no close-out fields required |
   ```

   b. Keep the `aborted` row (line 320) exactly as-is — it still states the fields are enforced.

   c. Replace the block heading (line 323):
   ```
   **Close-out fields (`aborted` / `completed`):** vault-cli enforces that every close-out transition records a disposition. Two frontmatter fields carry it:
   ```
   with:
   ```
   **Close-out fields (`aborted` only):** vault-cli enforces that every `aborted` transition records a disposition. `completed` never requires these fields — they are optional on completion and persisted only when supplied. Two frontmatter fields carry the close-out:
   ```

   d. Replace the first bullet (line 325):
   ```
   - `aborted_reason` — free text explaining the close-out; used for **both** `aborted` and `completed` transitions (there is no separate `completion_reason` field).
   ```
   with:
   ```
   - `aborted_reason` — free text explaining the close-out; used for the `aborted` transition (there is no separate `completion_reason` field).
   ```

   e. Keep the `gate_successor` bullet (line 326) as-is.

   f. Replace the enforcement paragraph (line 328):
   ```
   The CLI rejects `aborted` / `completed` (via `task set`, `task complete`, `goal set`, `goal complete`, or the `task update` checkbox sync) unless both fields are present; the error names the missing fields, asks what the task owns (trigger / gate / threshold / recurring check) and where it moves. Values are user-supplied strings persisted through the YAML serializer. Pre-existing reason-less close-outs are not backfilled. `task set`, `task complete`, `goal set`, and `goal complete` accept --reason <text> and --gate-successor <successor|none> to record both fields in the same invocation; a close-out attempted without them is rejected with an error that names the missing fields and the succeeding command form.
   ```
   with:
   ```
   The CLI rejects `aborted` (via `task set`, `task complete`, `goal set`, `goal complete`, or the `task update` checkbox sync) unless both fields are present; the error names the missing fields, asks what the task owns (trigger / gate / threshold / recurring check) and where it moves. Values are user-supplied strings persisted through the YAML serializer. Pre-existing reason-less close-outs are not backfilled. `task set`, `task complete`, `goal set`, and `goal complete` accept --reason <text> and --gate-successor <successor|none> to record both fields in the same invocation; on a `completed` transition they are optional, on `aborted` a close-out attempted without them is rejected with an error that names the missing fields and the succeeding command form.
   ```

   Do NOT change any other text in the file. The AC evidence requires `grep -n "Close-out fields" docs/task-writing.md` to return a heading naming `aborted` only.

2. In `/workspace/pkg/cli/cli.go`, reword all eight close-out help strings so none claims completion requires the fields. The `set`-command strings keep stating the fields are required for `aborted`; the `complete`-command strings state they are optional for `completed`. Replace exactly these pairs:

   a. Lines 225-226 (`task complete`):
   ```go
   cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required to complete")
   cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required to complete")
   ```
   → both become:
   ```go
   cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required for aborted, optional for completed")
   cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required for aborted, optional for completed")
   ```

   b. Lines 973-974 (`goal set`, inside the `if entityType == "goal"` block):
   ```go
   cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required for goal close-out")
   cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required for goal close-out")
   ```
   → both become the same new strings as (a).

   c. Lines 1405-1406 (`goal complete`):
   ```go
   cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required to complete a goal")
   cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required to complete a goal")
   ```
   → both become the same new strings as (a).

   d. Lines 2129-2130 (`task set`):
   ```go
   cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required for task close-out")
   cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required for task close-out")
   ```
   → both become the same new strings as (a).

   Do NOT change any other line in `cli.go`. The AC evidence requires `grep -n 'required to complete' pkg/cli/cli.go` to return 0 lines after this change.

3. In `/workspace/specs/completed/037-mandatory-abort-reason-and-gate-successor.md`, append a supersession note AFTER the final line of the file (the existing "Verdict: PASS" line). Do NOT modify, delete, or reorder any existing line. The note names this spec and states the `completed` relaxation while preserving the 037 `aborted` contract. Append exactly:

   ```
   ## Superseded (spec 039)

   **Superseded:** 2026-08-25 — spec 039 (`039-bug-completed-requires-closeout-fields`) relaxes the close-out guard for the `completed` transition: a task or goal may now transition to `completed` without `aborted_reason` / `gate_successor`; the fields remain optional on completion and are still persisted when supplied via `--reason` / `--gate-successor`. The `aborted` transition is unchanged and still requires both fields exactly as this spec locked them. Body text above is preserved as written; only this note is appended.
   ```

   The AC evidence requires `grep -n 'no longer requires\|Superseded' specs/completed/037-mandatory-abort-reason-and-gate-successor.md` to return at least one line.

4. In `/workspace/CHANGELOG.md`, add a `## Unreleased` section between the preamble (the `* PATCH version when you make backwards-compatible bug fixes.` line) and the `## v0.116.1` section. Use exactly:

   ```
   ## Unreleased

   - fix: `completed` transitions no longer require the close-out fields — `task complete`, `goal complete`, `task set ... status completed`, `goal set ... status completed`, and the `task update` checkbox sync now succeed without `aborted_reason` / `gate_successor`; spec 037 over-applied the close-out guard to a status that is not a close-out, so this reverses the 037 breaking change for completion only. The fields remain optional on completion and are still persisted when `--reason` / `--gate-successor` are supplied; the `aborted` transition is unchanged and still requires both fields.
   ```

   Do NOT touch the v0.116.1 or v0.116.0 sections. Do NOT bump any plugin manifest version (this repo's `github-releaser` owns version bumps + tags; see CLAUDE.md Plugin Release Checklist).

5. Run `make format`, then `make test`, then `make precommit` once. If `make precommit` fails on a target, re-run only the failing target until it passes, then `make precommit` once more. (The `check-changelog` and `check-versions` targets run as part of the Makefile `check` flow — the new `## Unreleased` section must satisfy `scripts/check-changelog.sh`.)
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- `docs/task-writing.md` § Lifecycle is the live doc and is updated in place; no other doc section is touched.
- The completed spec-037 record is immutable — only an appended supersession note, never a body rewrite, never a reorder of existing lines.
- CLI help text must not claim completion requires the fields: the exact substring `required to complete` must not appear anywhere in `pkg/cli/cli.go`. The `set`-command help must continue to state the fields are required for the `aborted` transition (which stays true).
- Frozen field names (`aborted_reason`, `gate_successor`) and the `aborted`-requires-both-fields contract are unchanged — this prompt only re-scopes the surface text to match the new `completed` behavior.
- No migration, no backfill; no Go behavior changes beyond the eight help strings in requirement 2.
- The version-alignment rule (CHANGELOG top version vs the three plugin JSON fields) is NOT triggered here: a `## Unreleased` section is not a versioned release section, and the releaser owns the bump.
- Existing tests must still pass; `make precommit` must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</constraints>

<verification>
Run all of the following from `/workspace`.

No help text claims completion requires the fields (spec AC 9):

```
grep -n 'required to complete' pkg/cli/cli.go
```

Must print `0` lines.

The set-command help still states the aborted requirement and the new help strings are in place:

```
grep -c 'required for aborted, optional for completed' pkg/cli/cli.go
```

Must print `8` (four flag pairs, two flags each).

The doc heading is scoped to `aborted` only (spec AC 9):

```
grep -n 'Close-out fields' docs/task-writing.md
```

Must return exactly one line containing `aborted` only — the old `(aborted / completed)` heading must be gone:

```
grep -c 'Close-out fields (`aborted` / `completed`)' docs/task-writing.md
```

Must print `0`.

The doc no longer claims completion requires the fields:

```
grep -c 'requires close-out fields' docs/task-writing.md
grep -c 'used for \*\*both\*\* `aborted` and `completed`' docs/task-writing.md
```

Both must print `0`.

The spec-037 record carries the appended note without body edits (spec AC 10):

```
grep -n 'no longer requires\|Superseded' specs/completed/037-mandatory-abort-reason-and-gate-successor.md
```

Must return at least one line, and the note must be at the end of the file (nothing after "Verdict: PASS" except the appended `## Superseded (spec 039)` section).

The changelog has the Unreleased entry in the right place (below preamble, above `## v0.116.1`):

```
grep -n '^## Unreleased$' CHANGELOG.md
grep -n 'no longer require the close-out fields' CHANGELOG.md
```

Both must return at least one line, and the `## Unreleased` line must come before `## v0.116.1` and after the preamble.

The package is green and the changelog structure check passes:

```
make test
```

Must exit 0.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</verification>
