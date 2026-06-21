#!/bin/sh
set -eu

artifact_dir=${1:?usage: package_artifacts.sh ARTIFACT_DIR OUTPUT_DIR VERSION}
output_dir=${2:?usage: package_artifacts.sh ARTIFACT_DIR OUTPUT_DIR VERSION}
version=${3:?usage: package_artifacts.sh ARTIFACT_DIR OUTPUT_DIR VERSION}
mode=${4:-all}
package_basename=${5:-nvim-sandbox_${version}}
artifact_basename=${6:-nvim-sandbox}
root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
artifact_dir=$(CDPATH='' cd -- "$artifact_dir" && pwd)

mkdir -p "$output_dir"
output_dir=$(CDPATH='' cd -- "$output_dir" && pwd)
stage_dir=$(mktemp -d "${TMPDIR:-/tmp}/nvim-sandbox-package.XXXXXX")

cleanup() {
  rm -rf "$stage_dir"
}
trap cleanup EXIT HUP INT TERM

package_archive() {
  target=$1
  platform=$2
  arch=$3
  source_binary="$artifact_dir/$artifact_basename-build-$target/nvim-sandbox"
  package_name="${package_basename}_${platform}_${arch}"
  package_root="$stage_dir/$package_name"

  if [ ! -f "$source_binary" ]; then
    echo "build artifact not found: $source_binary" >&2
    exit 1
  fi

  mkdir -p "$package_root"
  install -m 0755 "$source_binary" "$package_root/nvim-sandbox"
  install -m 0644 "$root_dir/LICENSE" "$package_root/LICENSE"
  install -m 0644 "$root_dir/THIRD_PARTY_NOTICES.md" "$package_root/THIRD_PARTY_NOTICES.md"
  tar -C "$stage_dir" -czf "$output_dir/$package_name.tar.gz" "$package_name"
}

package_deb() {
  target=$1
  arch=$2
  source_binary="$artifact_dir/$artifact_basename-build-$target/nvim-sandbox"
  deb_version=$(printf '%s' "$version" | sed 's/-/~/g')
  package_root="$stage_dir/deb-$arch"
  output_file="$output_dir/${package_basename}_${arch}.deb"
  installed_size=$(du -k "$source_binary" | awk '{print $1}')

  mkdir -p "$package_root/DEBIAN" "$package_root/usr/bin" "$package_root/usr/share/doc/nvim-sandbox"
  install -m 0755 "$source_binary" "$package_root/usr/bin/nvim-sandbox"
  install -m 0644 "$root_dir/LICENSE" "$package_root/usr/share/doc/nvim-sandbox/copyright"
  install -m 0644 "$root_dir/THIRD_PARTY_NOTICES.md" "$package_root/usr/share/doc/nvim-sandbox/THIRD_PARTY_NOTICES.md"
  printf '%s\n' \
    'Package: nvim-sandbox' \
    "Version: $deb_version" \
    'Section: devel' \
    'Priority: optional' \
    "Architecture: $arch" \
    'Maintainer: nvim-sandbox maintainers <stasfilin@users.noreply.github.com>' \
    "Installed-Size: $installed_size" \
    'Homepage: https://github.com/stasfilin/nvim-sandbox' \
    'Description: Project-scoped Neovim development sandboxes' \
    >"$package_root/DEBIAN/control"
  dpkg-deb --root-owner-group --build "$package_root" "$output_file" >/dev/null
}

package_archive linux-amd64 linux amd64
package_archive linux-arm64 linux arm64
package_archive darwin-amd64 macos amd64
package_archive darwin-arm64 macos arm64

case "$mode" in
  all)
    package_deb linux-amd64 amd64
    package_deb linux-arm64 arm64
    (
      cd "$output_dir"
      sha256sum ./*.tar.gz ./*.deb >SHA256SUMS
    )
    ;;
  archives)
    (
      cd "$output_dir"
      sha256sum ./*.tar.gz >SHA256SUMS
    )
    ;;
  *)
    echo "package mode must be all or archives: $mode" >&2
    exit 2
    ;;
esac

echo "Packaged CLI outputs:"
for file in "$output_dir"/*; do
  size=$(du -h "$file" | awk '{print $1}')
  echo "  $(basename "$file") ($size)"
done
echo "SHA-256 checksums:"
cat "$output_dir/SHA256SUMS"
