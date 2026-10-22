#!/usr/bin/env bash
#
# Print the goal entries of a topic page's Goals section.
#
# usage: update-topic-goal-entries.sh <page_path>
#
# Output: one line per entry, `<goal name><TAB><status token or "none">`. The
# caller compares the names against the membership set to build the diff, and
# compares each token against the goal file's own `status:` to build the drift
# list.
#
# The page is passed as a path, never a glob: topic folder names contain spaces
# ("23 Topics"), so an unquoted glob word-splits into `23` and `Topics/*.md` and
# the scan reads the wrong files — the same trap the taggers wrapper documents.
#
# Exit: 0 entries printed (possibly zero), 2 usage or missing file, 3 no Goals
# heading. 3 is deliberately not folded into 0: an empty list and an absent
# section are different facts, and the caller proposes additions from the former.

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: update-topic-goal-entries.sh <page_path>" >&2
  exit 2
fi

PAGE=$1

if [ ! -f "$PAGE" ]; then
  echo "not a file: $PAGE" >&2
  exit 2
fi

HERE=$(cd "$(dirname "$0")" && pwd)

# LC_ALL=C is load-bearing for the same reason it is in update-topic-taggers.sh:
# byte semantics is what these parsers and their tests assume, and a UTF-8 locale
# changes what the character classes and `%c` mean under gawk.
env LC_ALL=C awk -f "$HERE/update-topic-goal-entries.awk" "$PAGE"
