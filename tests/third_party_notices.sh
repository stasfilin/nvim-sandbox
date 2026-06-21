#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
notices="$root_dir/THIRD_PARTY_NOTICES.md"
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nvim-sandbox-notices.XXXXXX")

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

expected="$tmp_dir/expected"
actual="$tmp_dir/actual"

if ! grep -Fq '### Go standard library and runtime' "$notices"; then
  echo "THIRD_PARTY_NOTICES.md does not include the Go runtime license" >&2
  exit 1
fi

GOCACHE=${GOCACHE:-/tmp/nvim-sandbox-go-cache} \
  go list -deps \
  -f '{{with .Module}}{{if ne .Path "github.com/stasfilin/nvim-sandbox"}}{{.Path}} {{.Version}}{{end}}{{end}}' \
  "$root_dir/cmd/nvim-sandbox" \
  | sed '/^$/d; s/^/- /' \
  | sort -u >"$expected"

sed -n '/^## Included modules$/,/^## License texts$/p' "$notices" \
  | sed -n '/^- /p' \
  | sort -u >"$actual"

if ! diff -u "$expected" "$actual"; then
  echo "THIRD_PARTY_NOTICES.md does not match the compiled module set" >&2
  exit 1
fi

echo "Third-party dependency notices are current."
