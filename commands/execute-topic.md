---
description: Gate planning → execution for a topic. Re-runs the four structural checks; on pass flips phase to execution. Refuses on any failed check with no phase mutation.
argument-hint: "<topic-file-path-or-name> (or detects from conversation)"
allowed-tools: [Read, Edit, Glob, Bash, AskUserQuestion, Task, Skill]
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

**With argument:** exact path if path-like, else `Glob` `<topics_dir>/*<arg>*.md`. **Resolve the folder from `vault-cli config`, never a literal** — the per-vault folder is declared, not fixed (this vault renumbered its folders on 2026-09-13, which is how a hardcoded path once came to prepend a folder that no longer existed):

```bash
TOPICS_DIR=$(vault-cli config list --output json 2>/dev/null | python3 -c "import json,sys,os;cwd=os.path.realpath(os.getcwd());d=json.load(sys.stdin);print(next((v.get('topics_dir') or '23 Topics' for v in d if os.path.realpath(os.path.expanduser(v['path']))==cwd),'23 Topics'))" 2>/dev/null || echo "23 Topics")
```

The `|| echo` is a last-resort fallback for a vault with no config entry, not a default to prefer. Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (same priority order as `/verify-topic`):

1. Most recent `/verify-topic` or `/worker-verify` output — scan the parent conversation for the resolved topic name.
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
- `phase: todo` OR `phase` empty AND `status: in_progress` → `❌ Topic has no plan yet (phase: todo). Set it with vault-cli topic set "<name>" phase planning once the topic passes /verify-topic.` (planning is non-skippable)

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

```
/verify-topic "<name>"
```

`/verify-topic` is the shipped verifier and owns the member-links-resolve check — its **check 2**. Do **not** reimplement link resolution here: two implementations drift, and delegating honors [[Phase-Gated Topic Flow]] SC5 *"is called (not reimplemented)"*.

⚠️ **Consume check 2's result ONLY — never `verify-topic`'s aggregate verdict.** `/verify-topic` runs **ten** checks (necessity, excess, Completion-Gate checkability, no-double-declaration, `## Goals` entry format, status consistency, Status Summary reconciliation, …). Refusing on its aggregate verdict would make this gate wider than its declared closed set of four and would refuse a structurally-complete topic for reasons outside this gate's authority. Read the report's `2. Member links resolve` line:

- `PASS` → the check passes.
- `FAIL` → this gate's check 3 fails. Name `verify-topic check 2` as the failing check and quote its reported issue.
- `UNPROVEN` → treat as a failure of check 3 and say so plainly. The verifier reports `UNPROVEN` when no exercised page could violate the check, which is not evidence of health.
- `DRIFT` is a token only check 8 carries and is explicitly **not** a failure — it cannot appear on check 2.

The other nine checks are the topic's own governance and are reported by `/verify-topic` for its own caller. They do not gate this transition.

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
- **Idempotent on `phase`.** Safe to re-run on `phase: execution` — it writes nothing to `phase` and just confirms the topic is past the gate. (Step 4 still runs first, so a topic sitting at `phase: execution` with a non-`in_progress` status is moved to `in_progress`; that is the status entry contract, not a phase mutation.)

## Integration

Topic lifecycle:

1. `/verify-topic` — validate (read-only, ten checks)
2. `/worker-verify` — take a topic from drift to ready-to-start (read-only, advises fixes)
3. **`/vault-cli:execute-topic`** — the gate; flips planning → execution on a clean pass, refuses otherwise — this command
4. Work the topic's members via the task/goal lifecycle

Output ends with one of:
- `✅ Phase: planning → execution` (gate passed)
- `ℹ️ Already in execution.` (idempotent re-entry)
- `❌ Topic not ready. Failed checks: ...` (one or more checks failed)
- `❌ Topic has no plan yet (phase: todo).` (phase: todo)
- `❌ Topic closed (...).` + the reopen sequence (status/phase terminal)
- `❌ No topic detected. Pass a topic identifier or name.` (input error)
