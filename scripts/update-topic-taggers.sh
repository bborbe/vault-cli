#!/usr/bin/env bash
#
# Print the basenames (without .md) of goal files whose frontmatter `topics:`
# (or legacy `topic:`) names <topic_name>.
#
# usage: update-topic-taggers.sh <goals_dir> <topic_name>
#
# Output is goal NAMES, not paths: the topic page lists goals as
# `- [[Goal Name]]`, so the membership diff compares names. Sorting and
# de-duplication happen here so the caller never has to normalise.
#
# The scan lives in update-topic-taggers.awk — the rule is data-shaped and is
# exercised directly by update-topic-taggers-test.sh.

set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: update-topic-taggers.sh <goals_dir> <topic_name>" >&2
  exit 2
fi

GOALS_DIR=$1
TOPIC=$2

if [ ! -d "$GOALS_DIR" ]; then
  echo "not a directory: $GOALS_DIR" >&2
  exit 2
fi

if [ -z "$TOPIC" ]; then
  echo "empty topic name" >&2
  exit 2
fi

HERE=$(cd "$(dirname "$0")" && pwd)

# `find ... -exec ... {} +` rather than an unquoted `$GOALS_DIR/*.md` glob:
# vault folder names contain spaces ("24 Goals", "23 Topics"), so an unquoted
# glob word-splits into `24` and `Goals/*.md` and the scan silently reads the
# wrong files. `{} +` also means awk never runs at all when nothing matches.
find "$GOALS_DIR" -maxdepth 1 -name '*.md' -exec \
  awk -v t="$TOPIC" -f "$HERE/update-topic-taggers.awk" {} + |
  sed 's|.*/||; s|\.md$||' |
  sort -u
