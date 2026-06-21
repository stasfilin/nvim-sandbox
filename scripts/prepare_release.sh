#!/bin/sh
set -eu

version=${1:?release version is required}
root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
artifact_dir="$root_dir/build-artifacts-release"
release_dir="$root_dir/release"

"$root_dir/scripts/verify_release_version.sh" "$version"
printf '%s\n' "$version" >"$root_dir/VERSION"
cd "$root_dir"

for target in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
  goos=${target%-*}
  goarch=${target#*-}
  output_dir="$artifact_dir/nvim-sandbox-build-$target"
  mkdir -p "$output_dir"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath \
      -ldflags "-X github.com/stasfilin/nvim-sandbox/internal/cli.version=$version" \
      -o "$output_dir/nvim-sandbox" \
      ./cmd/nvim-sandbox
done

"$root_dir/scripts/package_artifacts.sh" \
  "$artifact_dir" "$release_dir" "$version" all nvim-sandbox nvim-sandbox

"$root_dir/scripts/update_homebrew_formula.sh" \
  "$version" "$release_dir/SHA256SUMS"
