# shellcheck shell=bash
#
# Shared helpers for vault-cli scenario runner scripts.
# Source from a scenario runner:
#
#     source "$(dirname "$0")/../helper/lib.sh"
#     build_binary
#     setup_example_vault
#     assert_exit_zero "$($VAULT_CLI --config $CONFIG task list >/dev/null; echo $?)" "task list runs"
#     scenario_done
#
# Exposes (after setup_example_vault):
#   BIN / VAULT_CLI    — path to the built binary (default: /tmp/new-vault-cli)
#   WORK_DIR           — temp sandbox dir (auto-removed at exit)
#   CONFIG             — path to the populated example config.yaml
#   VAULT_PATH         — path to $WORK_DIR/vault
#   PASS_COUNT, FAIL_COUNT — counters tracked by assert_*
#
# Functions:
#   build_binary [SRC_DIR]            — go build -o $BIN .
#   setup_example_vault               — mktemp + cp example/. + sed __VAULT_PATH__
#   days_from_today N                 — print YYYY-MM-DD for today + N days (BSD/GNU portable)
#   today                             — print YYYY-MM-DD for today
#   assert_exit_zero EXIT LABEL
#   assert_exit_nonzero EXIT LABEL
#   assert_grep FILE PATTERN LABEL    — extended regex match on file
#   assert_no_grep FILE PATTERN LABEL — extended regex must NOT match
#   assert_contains_string ACTUAL EXPECTED LABEL
#   scenario_done                     — print summary; exit non-zero on any FAIL

set -uo pipefail

BIN=${BIN:-/tmp/new-vault-cli}
VAULT_CLI=$BIN
PASS_COUNT=0
FAIL_COUNT=0

build_binary() {
  local src_dir=${1:-${VAULT_CLI_SRC:-$HOME/Documents/workspaces/vault-cli}}
  echo "→ building $BIN from $src_dir"
  go build -C "$src_dir" -o "$BIN" .
}

# Populate WORK_DIR with the example vault + a config.yaml whose __VAULT_PATH__
# placeholder is replaced with the live tempdir. Cleans up at script exit.
setup_example_vault() {
  local example_dir=${VAULT_CLI_EXAMPLE:-$HOME/Documents/workspaces/vault-cli/example}
  WORK_DIR=$(mktemp -d)
  trap 'rm -rf "$WORK_DIR"' EXIT
  # /. trailing-dot copies contents portably across BSD + GNU cp.
  cp -R "$example_dir/." "$WORK_DIR/"
  # -i.bak then rm is portable across BSD + GNU sed.
  sed -i.bak "s|__VAULT_PATH__|$WORK_DIR/vault|g" "$WORK_DIR/config.yaml"
  rm -f "$WORK_DIR/config.yaml.bak"
  CONFIG="$WORK_DIR/config.yaml"
  VAULT_PATH="$WORK_DIR/vault"
  export VAULT_CLI BIN CONFIG WORK_DIR VAULT_PATH
  echo "→ sandbox: $WORK_DIR"
}

# BSD date first (`-v+Nd`), GNU date fallback (`-d "+N days"`). Stderr suppressed.
days_from_today() {
  local n=$1
  date -v+"${n}"d +%Y-%m-%d 2>/dev/null || date -d "+${n} days" +%Y-%m-%d
}

today() {
  date +%Y-%m-%d
}

assert_exit_zero() {
  local exit_code=$1 label=$2
  if [ "$exit_code" -eq 0 ]; then
    echo "  PASS  $label (exit 0)"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL  $label (exit $exit_code, expected 0)"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

assert_exit_nonzero() {
  local exit_code=$1 label=$2
  if [ "$exit_code" -ne 0 ]; then
    echo "  PASS  $label (exit $exit_code)"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL  $label (expected non-zero, got 0)"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

assert_grep() {
  local file=$1 pattern=$2 label=$3
  if grep -qE "$pattern" "$file" 2>/dev/null; then
    echo "  PASS  $label"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL  $label"
    echo "        pattern: $pattern"
    echo "        file:    $file"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

assert_no_grep() {
  local file=$1 pattern=$2 label=$3
  if grep -qE "$pattern" "$file" 2>/dev/null; then
    echo "  FAIL  $label (unexpected match)"
    echo "        pattern: $pattern"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  else
    echo "  PASS  $label"
    PASS_COUNT=$((PASS_COUNT + 1))
  fi
}

assert_contains_string() {
  local actual=$1 expected=$2 label=$3
  if [[ "$actual" == *"$expected"* ]]; then
    echo "  PASS  $label"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL  $label"
    echo "        expected substring: $expected"
    echo "        actual: $actual"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

scenario_done() {
  echo
  echo "Result: $PASS_COUNT passed, $FAIL_COUNT failed"
  if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
  fi
}
