#!/bin/sh
set -eu

base=${1:-}
head=${2:-HEAD}

case "$base" in
  ""|0000000000000000000000000000000000000000)
    exec npx commitlint --last --verbose
    ;;
esac

if ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  exec npx commitlint --last --verbose
fi

exec npx commitlint --from "$base" --to "$head" --verbose
