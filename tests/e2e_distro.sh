#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
binary=${NVIM_SANDBOX_BIN:-"$root_dir/dist/nvim-sandbox"}
runtime=${1:-}
distro=${2:-}
image=${3:-}
install_command=${4:-}

if [ -z "$runtime" ] || [ -z "$distro" ] || [ -z "$image" ] || [ -z "$install_command" ]; then
  echo "usage: $0 docker|podman <distro> <image> <install-command>" >&2
  exit 2
fi

cleanup_runtime_storage() {
  :
}

if [ ! -x "$binary" ]; then
  echo "nvim-sandbox binary not found: $binary" >&2
  exit 1
fi

case "$runtime" in
  docker)
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
  *)
    echo "usage: $0 docker|podman <distro> <image> <install-command>" >&2
    exit 2
    ;;
esac

temp_parent=${NVIM_SANDBOX_E2E_TMPDIR:-${TMPDIR:-/tmp}}
mkdir -p "$temp_parent"
temp_dir=$(mktemp -d "$temp_parent/nvim-sandbox-$runtime-$distro-e2e.XXXXXX")
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
printf 'module example.com/nvim-sandbox-%s-e2e\n\ngo 1.26\n' "$distro" >"$project_dir/go.mod"

cd "$project_dir"
export XDG_DATA_HOME="$temp_dir/data"

echo "CLI: $("$binary" version)"
echo "Runtime: $(runtime_version)"
echo "Distro: $distro"
echo "Image: $image"
echo "Install command: $install_command"
echo "Fixture: $project_dir"
echo "Creating $runtime sandbox for $distro..."
create_json=$(
  "$binary" create \
    --runtime "$runtime" \
    --image "$image" \
    --install nvim,git,rg \
    --install-command "$install_command" \
    --lsp-tools pyright \
    --keep-running \
    --no-interactive \
    --format json
)
container_name=$(printf '%s' "$create_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["metadata"]["container_name"])')
echo "Container: $container_name"
printf '%s' "$create_json" | E2E_RUNTIME="$runtime" E2E_IMAGE="$image" E2E_PROJECT="$project_dir" E2E_INSTALL="$install_command" python3 -c '
import json, os, sys
data = json.load(sys.stdin)
metadata = data["metadata"]
assert data["ok"] is True
assert data["action"] == "created"
assert metadata["runtime"] == os.environ["E2E_RUNTIME"]
assert metadata["base_image"] == os.environ["E2E_IMAGE"]
assert metadata["project_root"] == os.environ["E2E_PROJECT"]
assert metadata["install_command"] == os.environ["E2E_INSTALL"]
assert metadata["editor_tools"] == ["pyright"]
'

echo "Checking installed tools..."
"$binary" exec --format json -- nvim --version | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["ok"] is True; assert "NVIM" in data["output"]'
"$binary" exec --format json -- git --version | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["ok"] is True; assert "git version" in data["output"].lower()'
"$binary" exec --format json -- rg --version | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["ok"] is True; assert "ripgrep" in data["output"].lower()'
"$binary" exec --format json -- pyright --version | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["ok"] is True; assert "pyright" in data["output"].lower()'
echo "Tools: nvim, git, rg, pyright"

echo "Destroying sandbox..."
"$binary" destroy --yes --format json >/dev/null
if container_exists "$container_name"; then
  echo "container still exists after destroy: $container_name" >&2
  exit 1
fi
container_name=

echo "$runtime $distro distro E2E passed."
