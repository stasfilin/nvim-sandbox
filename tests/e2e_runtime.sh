#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
binary=${NVIM_SANDBOX_BIN:-"$root_dir/dist/nvim-sandbox"}
runtime=${1:-}
image=alpine:3.20
lsp_tools=${NVIM_SANDBOX_E2E_LSP_TOOLS:-}

cleanup_runtime_storage() {
  :
}

if [ ! -x "$binary" ]; then
  echo "nvim-sandbox binary not found: $binary" >&2
  exit 1
fi

case "$runtime" in
  docker)
    runtime_label=Docker
    if ! command -v docker >/dev/null 2>&1; then
      echo "docker CLI is required" >&2
      exit 1
    fi
    if ! docker info >/dev/null 2>&1; then
      echo "Docker daemon is not available" >&2
      docker info
      exit 1
    fi
    cleanup_container() {
      docker rm -f "$1" >/dev/null 2>&1 || true
    }
    container_exists() {
      docker inspect "$1" >/dev/null 2>&1
    }
    runtime_version() {
      docker version --format 'Docker client {{.Client.Version}}, server {{.Server.Version}}'
    }
    ;;
  podman)
    runtime_label=Podman
    image=docker.io/library/alpine:3.20
    if ! command -v podman >/dev/null 2>&1; then
      echo "podman CLI is required" >&2
      exit 1
    fi
    if ! podman info >/dev/null 2>&1; then
      echo "Podman is not available" >&2
      podman info
      exit 1
    fi
    cleanup_container() {
      podman rm -f "$1" >/dev/null 2>&1 || true
    }
    cleanup_runtime_storage() {
      if [ "$(uname -s)" = Linux ] && [ -d "$temp_dir/data/containers" ]; then
        podman unshare rm -rf "$temp_dir/data/containers" >/dev/null 2>&1 || true
      fi
    }
    container_exists() {
      podman inspect "$1" >/dev/null 2>&1
    }
    runtime_version() {
      podman --version
    }
    ;;
  apple-container)
    runtime_label="Apple Container"
    if ! command -v container >/dev/null 2>&1; then
      echo "Apple container CLI is required" >&2
      exit 1
    fi
    if ! container system status >/dev/null 2>&1; then
      echo "Apple Container system is not running" >&2
      container system status
      exit 1
    fi
    cleanup_container() {
      container delete --force "$1" >/dev/null 2>&1 || true
    }
    container_exists() {
      container list --all --quiet | grep -F -x "$1" >/dev/null
    }
    runtime_version() {
      container --version
    }
    ;;
  *)
    echo "usage: $0 docker|podman|apple-container" >&2
    exit 2
    ;;
esac

temp_parent=${NVIM_SANDBOX_E2E_TMPDIR:-${TMPDIR:-/tmp}}
mkdir -p "$temp_parent"
temp_dir=$(mktemp -d "$temp_parent/nvim-sandbox-$runtime-e2e.XXXXXX")
project_dir="$temp_dir/project"
container_name=

cleanup() {
  if [ -n "$container_name" ]; then
    cleanup_container "$container_name"
  fi
  cleanup_runtime_storage
  rm -rf "$temp_dir"
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$project_dir" "$temp_dir/data"
printf 'module example.com/nvim-sandbox-e2e\n\ngo 1.26\n' >"$project_dir/go.mod"

cd "$project_dir"
export XDG_DATA_HOME="$temp_dir/data"

echo "CLI: $("$binary" version)"
echo "Runtime: $(runtime_version)"
echo "Fixture: $project_dir"
echo "Image: $image"
if [ -n "$lsp_tools" ]; then
  echo "LSP tools: $lsp_tools"
fi
echo "Creating $runtime sandbox..."
if [ -n "$lsp_tools" ]; then
  create_json=$(
    "$binary" create \
      --runtime "$runtime" \
      --image "$image" \
      --install-command apk \
      --lsp-tools "$lsp_tools" \
      --keep-running \
      --no-interactive \
      --format json
  )
else
  create_json=$(
    "$binary" create \
      --runtime "$runtime" \
      --image "$image" \
      --keep-running \
      --no-interactive \
      --format json
  )
fi
container_name=$(printf '%s' "$create_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["metadata"]["container_name"])')
echo "Container: $container_name"
printf '%s' "$create_json" | E2E_RUNTIME="$runtime" E2E_IMAGE="$image" E2E_PROJECT="$project_dir" E2E_LSP_TOOLS="$lsp_tools" python3 -c '
import json, os, sys
data = json.load(sys.stdin)
assert data["ok"] is True
assert data["action"] == "created"
assert data["metadata"]["runtime"] == os.environ["E2E_RUNTIME"]
if os.environ["E2E_LSP_TOOLS"]:
    assert data["metadata"]["base_image"] == os.environ["E2E_IMAGE"]
    assert data["metadata"]["editor_tools"] == os.environ["E2E_LSP_TOOLS"].split(",")
else:
    assert data["metadata"]["image"] == os.environ["E2E_IMAGE"]
assert data["metadata"]["project_root"] == os.environ["E2E_PROJECT"]
'

echo "Checking running status..."
status_json=$("$binary" status --format json)
printf '%s' "$status_json" | E2E_RUNTIME="$runtime" python3 -c '
import json, os, sys
data = json.load(sys.stdin)
assert data["runtime"] == os.environ["E2E_RUNTIME"]
assert data["status"] == "running"
'
echo "Status: running"

echo "Checking bind mount..."
"$binary" exec --format json -- touch /workspace/e2e-marker >/dev/null
if [ ! -f "$project_dir/e2e-marker" ]; then
  echo "bind-mounted marker was not written to the host: $project_dir/e2e-marker" >&2
  exit 1
fi
echo "Mount: $project_dir -> /workspace (read-write)"

if [ -n "$lsp_tools" ]; then
  echo "Checking LSP tools..."
  pyright_json=$("$binary" exec --format json -- pyright --version)
  printf '%s' "$pyright_json" | python3 -c '
import json, sys
data = json.load(sys.stdin)
assert data["ok"] is True
assert "pyright" in data["output"].lower()
'
  echo "LSP tool: pyright available"
fi

echo "Stopping and reopening sandbox..."
"$binary" stop --format json >/dev/null
status_json=$("$binary" status --format json)
printf '%s' "$status_json" | python3 -c '
import json, sys
assert json.load(sys.stdin)["status"] in ("exited", "stopped")
'
echo "Status after stop: stopped"

"$binary" open --format json >/dev/null
status_json=$("$binary" status --format json)
printf '%s' "$status_json" | python3 -c '
import json, sys
assert json.load(sys.stdin)["status"] == "running"
'
echo "Status after open: running"

echo "Destroying sandbox..."
"$binary" destroy --yes --format json >/dev/null
if container_exists "$container_name"; then
  echo "container still exists after destroy: $container_name" >&2
  exit 1
fi
container_name=

echo "$runtime_label E2E lifecycle passed."
