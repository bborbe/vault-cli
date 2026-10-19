#!/usr/bin/env bash
#
# Pins the session-connect DETECTION rule by RUNNING the guard, not by reading it.
#
# A supervisor spawns a worker with a caller-chosen SHORT label, so the session's
# title is `⚙ <short label>` — decorated AND usually a truncation of the task
# name. The guard used to require exact equality on the decoration-stripped title,
# so every such spawn matched nothing and the task never connected (observed
# 2026-10-08, session 45c69a11: `⚙ Trading Vault Has No Agent Task Controller` vs
# the full task name).
#
# Two things are pinned, and they fail differently:
#
#   1. The DETERMINISTIC path. The harness sets `$CLAUDE_CODE_SESSION_ID` for the
#      session it launched, so detection needs no title at all. A guard that
#      ignores it still works when the title happens to be exact — which is why
#      the env-var case is exercised against a title that matches NOTHING.
#   2. The FALLBACK path. Only reached when the env var is unset, and then the
#      truncation tier applies: a prefix of the task name, >= 20 characters,
#      identifying exactly one live session. Both bounds are load-bearing — a
#      short prefix would read a role name as a truncation, and an ambiguous one
#      would guess where the guard's contract is to refuse.
#
# The block is EXTRACTED from the agent file and executed, so a copy of the loop
# cannot pass this test: the markdown is the implementation, and a green run here
# means the markdown's own snippet produced the id. (Sibling
# `scripts/session-claim-branch-test.sh` pins the prose of a branch no harness can
# run; this one pins a branch that is pure shell and can be run.)
#
# `--check-only <root>` runs the text checks against <root> and exits non-zero if
# any fails, so the self-check below proves a tree with the env-var branch
# stripped fails — not merely that the real tree passes.
#
# Run from repo root (Makefile target `test`).

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# `|| exit` is load-bearing: this script runs WITHOUT `-e`, so a failed cd would
# otherwise continue and check the wrong tree.
cd "$ROOT" || exit 2

CHECK_ONLY=false
ROOT_DIR="$ROOT"
if [ "${1:-}" = "--check-only" ]; then
	CHECK_ONLY=true
	ROOT_DIR="${2:?--check-only needs a root}"
fi

AGENT=agents/work-on-task-assistant.md
SECTION='### Session connect (MANDATORY when Obsidian task file exists)'

pass=0
fail=0

check() { # check <label> <want> <got>
	local label=$1 want=$2 got=$3
	if [ "$want" = "$got" ]; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ $label: want [$want], got [$got]" >&2
	fi
}

contains() {
	case "$1" in
	*"$2"*) printf 'yes' ;;
	*) printf '' ;;
	esac
}

# section <file> <heading> — the lines under an exact heading, until the next heading.
section() {
	awk -v h="$2" '$0 == h { f = 1; next } f && /^#/ { exit } f' "$1"
}

# block <file> — the first ```bash fenced block under § Session connect, with the
# three-space list indent stripped. Empty when the section carries none.
block() {
	awk -v h="$SECTION" '
		$0 == h { insec = 1; next }
		insec && /^[[:space:]]*```bash$/ { inblock = 1; next }
		inblock && /^[[:space:]]*```$/ { exit }
		inblock { sub(/^[[:space:]]{1,3}/, ""); print }
	' "$1"
}

SEC=$(section "$ROOT_DIR/$AGENT" "$SECTION")
BLOCK=$(block "$ROOT_DIR/$AGENT")

# --- 1. text: the deterministic path is present, and it is scoped to the section
# the connect step actually runs (a prose mention elsewhere cannot satisfy this).
check "§ Session connect names \$CLAUDE_CODE_SESSION_ID" "yes" \
	"$(contains "$SEC" 'CLAUDE_CODE_SESSION_ID')"

# --- 2. text: the truncation tier carries BOTH bounds. The length floor is what
# keeps a short role name from reading as a truncation; the uniqueness is what
# keeps an ambiguous prefix from being picked.
check "the guard applies a 20-character prefix floor" "yes" "$(contains "$BLOCK" '${#cur} -ge 20')"
check "the guard requires uniqueness at the chosen tier" "yes" "$(contains "$BLOCK" 'if(n==1)')"

# --- 3. behaviour: run the extracted block. A guard that cannot be run (no block)
# fails every case below rather than passing vacuously.
TASK='The Connect Step Matches a Session by Exact Task Title'
ENV_ID='11111111-2222-3333-4444-555555555555'

run_block() { # run_block <env-id-or-empty> <transcript titles...>
	local env_id=$1
	shift
	local tmp vault cfg enc bin stem i=0
	tmp=$(mktemp -d)
	vault="$tmp/vault"
	cfg="$tmp/config"
	bin="$tmp/bin"
	mkdir -p "$vault" "$bin"
	enc=$(printf '%s' "$vault" | sed 's|/|-|g')
	mkdir -p "$cfg/projects/$enc"
	for title in "$@"; do
		i=$((i + 1))
		stem=$(printf 'aaaaaaaa-0000-0000-0000-%012d' "$i")
		printf '{"type":"custom-title","customTitle":"%s"}\n' "$title" >"$cfg/projects/$enc/$stem.jsonl"
	done
	# Stub the CLI the block uses to resolve the vault's session dir.
	cat >"$bin/vault-cli" <<EOF
#!/usr/bin/env bash
printf '[{"path":"$vault"}]\\n'
EOF
	chmod +x "$bin/vault-cli"

	local script
	script=$(printf '%s\n' "$BLOCK" |
		sed -e "s|<active vault path>|$vault|" -e "s|<task_name>|$TASK|")

	local out
	if [ -n "$env_id" ]; then
		out=$(PATH="$bin:$PATH" CLAUDE_CONFIG_DIR="$cfg" CLAUDE_CODE_SESSION_ID="$env_id" bash -c "$script" 2>/dev/null)
	else
		out=$(PATH="$bin:$PATH" CLAUDE_CONFIG_DIR="$cfg" env -u CLAUDE_CODE_SESSION_ID bash -c "$script" 2>/dev/null)
	fi
	rm -rf "$tmp"
	printf '%s' "$out" | tr -d '[:space:]'
}

# The transcript stems run_block writes, in order: the first is the one an
# accepted match must resolve to.
STEM1='aaaaaaaa-0000-0000-0000-000000000001'
TRUNC30=$(printf '%s' "$TASK" | cut -c1-30)
TRUNC19=$(printf '%s' "$TASK" | cut -c1-19)

check "env var set + no title match → the env id" "$(printf '%s' "$ENV_ID" | tr -d '[:space:]')" \
	"$(run_block "$ENV_ID" '⚙ Something Entirely Unrelated To The Task')"
check "env var set + TWO truncation matches → still the env id" "$(printf '%s' "$ENV_ID" | tr -d '[:space:]')" \
	"$(run_block "$ENV_ID" "⚙ $TRUNC30" "⚙ $TRUNC30")"
check "env unset + exact title → that session" "$STEM1" \
	"$(run_block '' "⚙ $TASK")"
check "env unset + unique 30-char prefix → that session" "$STEM1" \
	"$(run_block '' "⚙ $TRUNC30")"
check "env unset + 19-char prefix → refuses" "" \
	"$(run_block '' "⚙ $TRUNC19")"
check "env unset + prefix shared by two live sessions → refuses" "" \
	"$(run_block '' "⚙ $TRUNC30" "⚙ $TRUNC30")"

# --- self-check (skipped in --check-only mode, which would recurse): strip the
# deterministic branch from a throwaway copy and require the text checks to fail
# there. A check that passed on a tree without the branch would measure nothing.
if [ "$CHECK_ONLY" = false ]; then
	TMP=$(mktemp -d)
	trap 'rm -rf "$TMP"' EXIT
	mkdir -p "$TMP/$(dirname "$AGENT")"
	sed 's/CLAUDE_CODE_SESSION_ID/STRIPPED/g' "$ROOT/$AGENT" >"$TMP/$AGENT"

	if bash "$ROOT/scripts/session-connect-fixture-test.sh" --check-only "$TMP" >/dev/null 2>&1; then
		fail=$((fail + 1))
		echo "❌ self-check: --check-only exited 0 on a tree without the deterministic branch" >&2
	else
		pass=$((pass + 1))
	fi

	if bash "$ROOT/scripts/session-connect-fixture-test.sh" --check-only "$ROOT" >/dev/null 2>&1; then
		pass=$((pass + 1))
	else
		fail=$((fail + 1))
		echo "❌ self-check: --check-only exited non-zero on the real tree" >&2
	fi
fi

echo "session-connect-fixture: $pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
