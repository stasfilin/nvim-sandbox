#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
binary=${NVIM_SANDBOX_BIN:-"$root_dir/dist/nvim-sandbox"}
runtime=${1:-}
image=alpine:3.20

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
    echo "usage: $0 docker|apple-container" >&2
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
echo "Creating $runtime sandbox..."
create_json=$(
  "$binary" create \
    --runtime "$runtime" \
    --image "$image" \
    --keep-running \
    --no-interactive \
    --format json
)
container_name=$(printf '%s' "$create_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["metadata"]["container_name"])')
echo "Container: $container_name"
printf '%s' "$create_json" | E2E_RUNTIME="$runtime" E2E_IMAGE="$image" E2E_PROJECT="$project_dir" python3 -c '
import json, os, sys
data = json.load(sys.stdin)
assert data["ok"] is True
assert data["action"] == "created"
assert data["metadata"]["runtime"] == os.environ["E2E_RUNTIME"]
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
