#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
runtime=${1:-}
if [ -z "$runtime" ]; then
  echo "usage: $0 docker|podman|apple-container" >&2
  exit 2
fi

NVIM_SANDBOX_E2E_LSP_TOOLS=pyright exec "$script_dir/e2e_runtime.sh" "$runtime"
