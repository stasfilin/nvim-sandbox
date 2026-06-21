#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nvim-sandbox-homebrew.XXXXXX")

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

checksums="$tmp_dir/SHA256SUMS"
formula="$tmp_dir/nvim-sandbox.rb"

cat >"$checksums" <<'EOF'
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ./nvim-sandbox_macos_arm64.tar.gz
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  ./nvim-sandbox_macos_amd64.tar.gz
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  ./nvim-sandbox_linux_arm64.tar.gz
dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd  ./nvim-sandbox_linux_amd64.tar.gz
EOF

"$root_dir/scripts/update_homebrew_formula.sh" 0.9.7 "$checksums" "$formula" >/dev/null

grep -Fq 'version "0.9.7"' "$formula"
grep -Fq '/v0.9.7/nvim-sandbox_macos_arm64.tar.gz"' "$formula"
grep -Fq '/v0.9.7/nvim-sandbox_macos_amd64.tar.gz"' "$formula"
grep -Fq '/v0.9.7/nvim-sandbox_linux_arm64.tar.gz"' "$formula"
grep -Fq '/v0.9.7/nvim-sandbox_linux_amd64.tar.gz"' "$formula"
grep -Fq 'sha256 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' "$formula"
grep -Fq 'sha256 "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"' "$formula"
grep -Fq 'sha256 "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"' "$formula"
grep -Fq 'sha256 "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"' "$formula"
grep -Fq 'doc.install "LICENSE", "THIRD_PARTY_NOTICES.md"' "$formula"

if grep -Eq '@[A-Z0-9_]+@' "$formula"; then
  echo "formula still contains template placeholders" >&2
  exit 1
fi

ruby -c "$formula" >/dev/null

committed_version=$(sed -n 's/^  version "\([^"]*\)"$/\1/p' "$root_dir/Formula/nvim-sandbox.rb")
expected_version=$(sed -n '1p' "$root_dir/VERSION")
if [ "$committed_version" != "$expected_version" ]; then
  echo "Homebrew formula version $committed_version does not match VERSION $expected_version" >&2
  exit 1
fi

echo "Homebrew formula generation checks passed."
