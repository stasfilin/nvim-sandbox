#!/bin/sh
set -eu

version=${1:?release version is required}
if ! printf '%s\n' "$version" | grep -Eq '^0\.[0-9]+\.[0-9]+$'; then
  echo "refusing release outside v0.x.x: v$version" >&2
  exit 1
fi

echo "Verified release version v$version"
