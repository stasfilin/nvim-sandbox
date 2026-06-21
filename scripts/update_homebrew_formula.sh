#!/bin/sh
set -eu

version=${1:?usage: update_homebrew_formula.sh VERSION SHA256SUMS [OUTPUT]}
checksums=${2:?usage: update_homebrew_formula.sh VERSION SHA256SUMS [OUTPUT]}
root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
template="$root_dir/scripts/homebrew/nvim-sandbox.rb.in"
output=${3:-$root_dir/Formula/nvim-sandbox.rb}

if ! printf '%s\n' "$version" | grep -Eq '^0\.[0-9]+\.[0-9]+$'; then
  echo "invalid release version: $version" >&2
  exit 1
fi

if [ ! -f "$checksums" ]; then
  echo "checksum file not found: $checksums" >&2
  exit 1
fi

checksum_for() {
  artifact=$1
  checksum=$(awk -v artifact="$artifact" '
    {
      filename = $2
      sub(/^\*/, "", filename)
      sub(/^\.\//, "", filename)
      if (filename == artifact) {
        print $1
      }
    }
  ' "$checksums")

  if ! printf '%s\n' "$checksum" | grep -Eq '^[0-9a-f]{64}$'; then
    echo "missing or invalid checksum for $artifact" >&2
    exit 1
  fi

  printf '%s\n' "$checksum"
}

macos_arm64_sha256=$(checksum_for nvim-sandbox_macos_arm64.tar.gz)
macos_amd64_sha256=$(checksum_for nvim-sandbox_macos_amd64.tar.gz)
linux_arm64_sha256=$(checksum_for nvim-sandbox_linux_arm64.tar.gz)
linux_amd64_sha256=$(checksum_for nvim-sandbox_linux_amd64.tar.gz)

output_dir=$(dirname -- "$output")
mkdir -p "$output_dir"
temporary=$(mktemp "$output_dir/.nvim-sandbox.rb.XXXXXX")
cleanup() {
  rm -f "$temporary"
}
trap cleanup EXIT HUP INT TERM

sed \
  -e "s/@VERSION@/$version/g" \
  -e "s/@MACOS_ARM64_SHA256@/$macos_arm64_sha256/g" \
  -e "s/@MACOS_AMD64_SHA256@/$macos_amd64_sha256/g" \
  -e "s/@LINUX_ARM64_SHA256@/$linux_arm64_sha256/g" \
  -e "s/@LINUX_AMD64_SHA256@/$linux_amd64_sha256/g" \
  "$template" >"$temporary"

mv "$temporary" "$output"
trap - EXIT HUP INT TERM
echo "Updated Homebrew formula for v$version: $output"
