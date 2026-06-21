#!/bin/sh
set -eu

release_dir=${1:?usage: release_packages.sh RELEASE_DIR VERSION [PACKAGE_BASENAME]}
version=${2:?usage: release_packages.sh RELEASE_DIR VERSION [PACKAGE_BASENAME]}
package_basename=${3:-nvim-sandbox_$version}

release_dir=$(CDPATH='' cd -- "$release_dir" && pwd)
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/nvim-sandbox-release-packages.XXXXXX")

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) host_target=linux_amd64 ;;
  Linux-aarch64 | Linux-arm64) host_target=linux_arm64 ;;
  Darwin-x86_64) host_target=macos_amd64 ;;
  Darwin-arm64) host_target=macos_arm64 ;;
  *) host_target=none ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$release_dir" && sha256sum --check SHA256SUMS >/dev/null)
elif command -v shasum >/dev/null 2>&1; then
  (cd "$release_dir" && shasum --algorithm 256 --check SHA256SUMS >/dev/null)
else
  echo "sha256sum or shasum is required" >&2
  exit 1
fi

expected_checksum_count=4
if command -v dpkg-deb >/dev/null 2>&1; then
  expected_checksum_count=6
fi
checksum_count=$(wc -l <"$release_dir/SHA256SUMS" | tr -d ' ')
if [ "$checksum_count" -ne "$expected_checksum_count" ]; then
  echo "SHA256SUMS contains $checksum_count entries, want $expected_checksum_count" >&2
  exit 1
fi

for target in linux_amd64 linux_arm64 macos_amd64 macos_arm64; do
  platform=${target%_*}
  arch=${target#*_}
  package_name="${package_basename}_${platform}_${arch}"
  archive="$release_dir/$package_name.tar.gz"
  extract_dir="$tmp_dir/$target"

  if [ ! -f "$archive" ]; then
    echo "release archive not found: $archive" >&2
    exit 1
  fi

  mkdir -p "$extract_dir"
  tar -xzf "$archive" -C "$extract_dir"
  binary="$extract_dir/$package_name/nvim-sandbox"
  license="$extract_dir/$package_name/LICENSE"
  notices="$extract_dir/$package_name/THIRD_PARTY_NOTICES.md"
  if [ ! -x "$binary" ] || [ ! -s "$license" ] || [ ! -s "$notices" ]; then
    echo "archive has an invalid binary or license notices: $archive" >&2
    exit 1
  fi

  if [ "$target" = "$host_target" ]; then
    version_json=$(NVIM_SANDBOX_NO_UPDATE_CHECK=1 "$binary" version --format json)
    printf '%s\n' "$version_json" | grep -Fq "\"version\":\"$version\""
  fi
done

if command -v dpkg-deb >/dev/null 2>&1; then
  deb_version=$(printf '%s' "$version" | sed 's/-/~/g')
  for arch in amd64 arm64; do
    package="$release_dir/${package_basename}_${arch}.deb"
    extract_dir="$tmp_dir/deb-$arch"

    if [ ! -f "$package" ]; then
      echo "Debian package not found: $package" >&2
      exit 1
    fi
    if [ "$(dpkg-deb --field "$package" Package)" != nvim-sandbox ]; then
      echo "unexpected Debian package name: $package" >&2
      exit 1
    fi
    if [ "$(dpkg-deb --field "$package" Version)" != "$deb_version" ]; then
      echo "unexpected Debian package version: $package" >&2
      exit 1
    fi
    if [ "$(dpkg-deb --field "$package" Architecture)" != "$arch" ]; then
      echo "unexpected Debian package architecture: $package" >&2
      exit 1
    fi

    dpkg-deb --extract "$package" "$extract_dir"
    binary="$extract_dir/usr/bin/nvim-sandbox"
    notices="$extract_dir/usr/share/doc/nvim-sandbox/THIRD_PARTY_NOTICES.md"
    if [ ! -x "$binary" ] || [ ! -s "$extract_dir/usr/share/doc/nvim-sandbox/copyright" ] || [ ! -s "$notices" ]; then
      echo "Debian package has an invalid payload: $package" >&2
      exit 1
    fi

    if [ "linux_$arch" = "$host_target" ]; then
      version_json=$(NVIM_SANDBOX_NO_UPDATE_CHECK=1 "$binary" version --format json)
      printf '%s\n' "$version_json" | grep -Fq "\"version\":\"$version\""
    fi
  done
fi

echo "Release package checksum, contents, metadata, and execution checks passed."
