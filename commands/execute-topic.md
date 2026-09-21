---
description: Gate planning → execution for a topic. Re-runs the four structural checks; on pass flips phase to execution. Refuses on any failed check with no phase mutation.
argument-hint: "<topic-file-path-or-name> (or detects from conversation)"
allowed-tools: [Read, Edit, Glob, Bash, AskUserQuestion, Task]
---

The **hard gate** between a topic's planning and execution. Refuses to flip `phase: planning → execution` unless all four structural checks pass. Idempotent on `phase: execution` — re-run it to confirm the topic is past the gate.

This command **must stay inline** — it analyzes the parent conversation when no argument is given; a sub-agent cannot see the conversation.

## When to use

After a topic's plan is genuinely complete, to formally enter execution. Re-run any time to confirm the topic cleared the gate.

```bash
/vault-cli:execute-topic                          # detects from conversation
/vault-cli:execute-topic "Some Topic Name"
/vault-cli:execute-topic 23\ Topics/Some\ Topic.md
```

## Process

### 1. Resolve topic path

**With argument:** exact path if path-like, else `Glob` `<topics_dir>/*<arg>*.md` (vault-cli config respected — never a hardcoded folder; this vault renumbered its folders on 2026-09-13). Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (same priority order as `/vault-cli:verify-topic`):

1. Most recent `/verify-topic` / `/work-on-topic` output — scan the parent conversation for the resolved topic name.
2. Most recent `[[Topic Name]]` wikilink referenced as a topic subject.
3. Most recently modified file in `<topics_dir>/`.

Multiple matches → ask via `AskUserQuestion`. Zero → `❌ No topic detected. Pass a topic identifier or name.` STOP.

Print `Detected topic: <name>` on first line so owner can interrupt before any state mutation.

### 2. Read status + phase

```bash
vault-cli topic get "<name>" status --output json
vault-cli topic get "<name>" phase --output json
```

### 3. Refusal cases (no mutation, exit non-zero)

Refuse and STOP if any apply:

- `status: completed` OR `status: aborted` → `❌ Topic closed (status: <value>). Reopen: vault-cli topic set "<name>" status in_progress; vault-cli topic set "<name>" phase planning — there is no reopen subcommand.`
- `phase: done` → `❌ Topic phase is done. Reopen with the same two commands as above.`
- `phase: todo` OR `phase` empty AND `status: in_progress` → `❌ Topic has no plan yet (phase: todo). Set it with vault-cli topic set "<name>" phase planning once the topic passes /vault-cli:verify-topic.` (planning is non-skippable)

### 4. Status entry contract (mutate, then continue)

If `status` is in `next` / `backlog` / `hold` → flip to `in_progress`:
```bash
vault-cli topic set "<name>" status in_progress
```
Print: `ℹ️ Status: <old> → in_progress (resume from <old>)`

If `status` is already `in_progress` → continue, no mutation.

### 5. Run the four structural checks (collect ALL failures — never short-circuit)

The check set is **closed at four**. Topics carry **no** `# Definition of Done` — `# Completion Gate` is the DoD analogue at this level. A gate demanding `# Definition of Done` would fail every real topic.

Read the topic file once, then evaluate:

1. **`# Success Criteria` present** — the heading exists. Report by name if missing.
2. **`# Completion Gate` present** — the heading exists. Report by name if missing.
3. **`## Goals` present with ≥1 entry** — the section exists and lists at least one member. Report by name if missing or empty.
4. **Success Criteria binary** — `# Success Criteria` carries ≥ 2 binary checkboxes (`- [ ]` / `- [x]`). Report the count found.

**The member-links-resolve check is DELEGATED, not reimplemented.** Check 3 requires "≥1 entry, all resolving" — the *presence* half is checked here, the *resolving* half is delegated:

```bash
/vault-cli:verify-topic "<name>"
```

`verify-topic` is the shipped verifier and owns the member-links-resolve check (its check 2). Do **not** reimplement link resolution here — two implementations drift, and this honors [[Phase-Gated Topic Flow]] SC5 *"is called (not reimplemented)"*. A non-zero verdict from `verify-topic` is a failed check for this gate; name `verify-topic` as the failing check and quote its reported issue.

### 6. Phase transition or refusal

**If ANY check failed AND `phase: planning`:**

Print:
```
❌ Topic not ready. Failed checks:
- <check name>: <one-line reason>
```

STOP. Do NOT flip phase. The topic's frontmatter must still read `phase: planning`.

**If all four checks pass AND `phase: planning`:**

```bash
vault-cli topic set "<name>" phase execution
```
Print: `✅ Phase: planning → execution`

**If `phase: execution`:** no flip, no check (already past the gate). Print: `ℹ️ Already in execution.`

## Notes

- **Refuses, never repairs.** A failed check is reported and the gate stops — the topic's owner fixes the page. This command never edits the topic's content, only its `phase`.
- **Four checks, closed set.** Do not add checks here. Topics have no `# Definition of Done`; `# Completion Gate` is the analogue.
- **Delegation is load-bearing.** The resolve half of check 3 belongs to `verify-topic`. Reimplementing it here would violate the flow-vs-instrument split named in [[No Command Takes a Topic From Drift to Ready-to-Start]].
- **Reads the vault's [[Topic Writing Guide]]** as the canonical rule source for what a well-formed topic requires.
- **Idempotent re-entry.** Safe to re-run on `phase: execution` — no mutation, just confirms the topic is past the gate.

## Integration

Topic lifecycle:

1. `/vault-cli:verify-topic` — validate (read-only, ten checks)
2. `/worker-check` — take a topic from drift to ready-to-start
3. **`/vault-cli:execute-topic`** — the gate; flips planning → execution on a clean pass, refuses otherwise — this command
4. Work the topic's members via the task/goal lifecycle

Output ends with one of:
- `✅ Phase: planning → execution` (gate passed)
- `ℹ️ Already in execution.` (idempotent re-entry)
- `❌ Topic not ready. Failed checks: ...` (one or more checks failed)
- `❌ Topic has no plan yet (phase: todo).` (phase: todo)
- `❌ Topic closed (...).` + the reopen sequence (status/phase terminal)
- `❌ No topic detected. Pass a topic identifier or name.` (input error)
