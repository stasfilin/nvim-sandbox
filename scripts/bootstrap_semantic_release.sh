#!/bin/sh
set -eu

if git tag --list 'v[0-9]*' | grep -q .; then
  exit 0
fi

root_commit=$(git rev-list --max-parents=0 HEAD | tail -n 1)
git tag v0.0.0 "$root_commit"
echo "Created local bootstrap tag v0.0.0 at $root_commit"
