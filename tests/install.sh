#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=$(sed -n '1p' "$root_dir/VERSION")
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nvim-sandbox-install.XXXXXX")

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

assert_installed_binary() {
  binary=$1
  if [ ! -x "$binary" ]; then
    echo "installed binary is missing or not executable: $binary" >&2
    exit 1
  fi

  version_json=$(NVIM_SANDBOX_NO_UPDATE_CHECK=1 "$binary" version --format json)
  printf '%s\n' "$version_json" | grep -Fq '"action":"version"'
  printf '%s\n' "$version_json" | grep -Fq "\"version\":\"$version\""
}

stage_dir="$tmp_dir/stage"
stage_binary="$stage_dir/usr/local/bin/nvim-sandbox"

make -C "$root_dir" install DESTDIR="$stage_dir" PREFIX=/usr/local >/dev/null
assert_installed_binary "$stage_binary"
make -C "$root_dir" uninstall DESTDIR="$stage_dir" PREFIX=/usr/local >/dev/null
if [ -e "$stage_binary" ]; then
  echo "uninstall left the staged binary behind: $stage_binary" >&2
  exit 1
fi

custom_bindir="$tmp_dir/custom prefix/bin"
custom_binary="$custom_bindir/nvim-sandbox"

make -C "$root_dir" install BINDIR="$custom_bindir" >/dev/null
assert_installed_binary "$custom_binary"
make -C "$root_dir" uninstall BINDIR="$custom_bindir" >/dev/null
if [ -e "$custom_binary" ]; then
  echo "uninstall left the custom-prefix binary behind: $custom_binary" >&2
  exit 1
fi

echo "Source install and uninstall checks passed."
