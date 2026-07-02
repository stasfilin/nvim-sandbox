#!/bin/sh
set -eu

root_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
binary=${NVIM_SANDBOX_BIN:-"$root_dir/dist/nvim-sandbox"}
runtime=${1:-}
manager=${2:-}
image=${NVIM_SANDBOX_E2E_PLUGIN_IMAGE:-alpine:3.20}
install_command=${NVIM_SANDBOX_E2E_PLUGIN_INSTALL:-apk}
plugin_repo=https://github.com/tpope/vim-sleuth.git

if [ -z "$runtime" ] || [ -z "$manager" ]; then
  echo "usage: $0 docker|podman native-pack|lazy.nvim|packer.nvim|vim-plug|custom" >&2
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
    image=${NVIM_SANDBOX_E2E_PLUGIN_IMAGE:-docker.io/library/alpine:3.20}
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
    echo "usage: $0 docker|podman native-pack|lazy.nvim|packer.nvim|vim-plug|custom" >&2
    exit 2
    ;;
esac

case "$manager" in
  native-pack | lazy.nvim | packer.nvim | vim-plug | custom)
    ;;
  *)
    echo "unknown plugin manager: $manager" >&2
    exit 2
    ;;
esac

temp_parent=${NVIM_SANDBOX_E2E_TMPDIR:-${TMPDIR:-/tmp}}
mkdir -p "$temp_parent"
temp_dir=$(mktemp -d "$temp_parent/nvim-sandbox-$runtime-$manager-plugins.XXXXXX")
project_dir="$temp_dir/project"
home_dir="$temp_dir/home"
container_name=

cleanup() {
  if [ -n "$container_name" ]; then
    cleanup_container "$container_name"
  fi
  cleanup_runtime_storage
  rm -rf "$temp_dir"
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$project_dir" "$temp_dir/data" "$home_dir/.config/nvim"
printf 'module example.com/nvim-sandbox-plugin-e2e\n\ngo 1.26\n' >"$project_dir/go.mod"

write_empty_config() {
  printf '%s\n' '" nvim-sandbox native pack E2E' >"$home_dir/.config/nvim/init.vim"
}

write_lazy_config() {
  cat >"$home_dir/.config/nvim/init.lua" <<'LUA'
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
  vim.fn.system({ "git", "clone", "--filter=blob:none", "--branch=stable", "https://github.com/folke/lazy.nvim.git", lazypath })
end
vim.opt.rtp:prepend(lazypath)
require("lazy").setup({
  { "tpope/vim-sleuth" },
}, {
  change_detection = { enabled = false },
  checker = { enabled = false },
  install = { missing = true },
  lockfile = vim.fn.stdpath("data") .. "/lazy-lock.json",
})
LUA
}

write_packer_config() {
  cat >"$home_dir/.config/nvim/init.lua" <<'LUA'
local install_path = vim.fn.stdpath("data") .. "/site/pack/packer/start/packer.nvim"
if vim.fn.empty(vim.fn.glob(install_path)) > 0 then
  vim.fn.system({ "git", "clone", "--depth", "1", "https://github.com/wbthomason/packer.nvim", install_path })
end
vim.cmd("packadd packer.nvim")
local packer = require("packer")
vim.fn.mkdir(vim.fn.stdpath("data") .. "/plugin", "p")
packer.init({
  compile_path = vim.fn.stdpath("data") .. "/plugin/packer_compiled.lua",
})
packer.startup(function(use)
  use("wbthomason/packer.nvim")
  use("tpope/vim-sleuth")
end)
LUA
}

write_vim_plug_config() {
  cat >"$home_dir/.config/nvim/init.vim" <<'VIM'
let s:plug_path = stdpath('data') . '/site/autoload/plug.vim'
if empty(glob(s:plug_path))
  call mkdir(fnamemodify(s:plug_path, ':h'), 'p')
  call system(['curl', '-fLo', s:plug_path, 'https://raw.githubusercontent.com/junegunn/vim-plug/master/plug.vim'])
endif
execute 'source' fnameescape(s:plug_path)
call plug#begin(stdpath('data') . '/plugged')
Plug 'tpope/vim-sleuth'
call plug#end()
VIM
}

prepare_manager() {
  case "$manager" in
    native-pack)
      if ! command -v git >/dev/null 2>&1; then
        echo "git is required to prepare native-pack fixture" >&2
        exit 1
      fi
      write_empty_config
      mkdir -p "$home_dir/.local/share/nvim/site/pack/nvim-sandbox/start"
      git clone --depth 1 "$plugin_repo" "$home_dir/.local/share/nvim/site/pack/nvim-sandbox/start/vim-sleuth" >/dev/null
      ;;
    lazy.nvim)
      write_lazy_config
      ;;
    packer.nvim)
      write_packer_config
      ;;
    vim-plug)
      write_vim_plug_config
      ;;
    custom)
      write_empty_config
      ;;
  esac
}

expected_path() {
  case "$manager" in
    native-pack)
      printf '%s\n' /root/.local/share/nvim/site/pack/nvim-sandbox/start/vim-sleuth/plugin/sleuth.vim
      ;;
    lazy.nvim)
      printf '%s\n' /root/.local/share/nvim/lazy/vim-sleuth/plugin/sleuth.vim
      ;;
    packer.nvim)
      printf '%s\n' /root/.local/share/nvim/site/pack/packer/start/vim-sleuth/plugin/sleuth.vim
      ;;
    vim-plug)
      printf '%s\n' /root/.local/share/nvim/plugged/vim-sleuth/plugin/sleuth.vim
      ;;
    custom)
      printf '%s\n' /root/.local/share/nvim/site/pack/custom/start/vim-sleuth/plugin/sleuth.vim
      ;;
  esac
}

exec_json() {
  command=$1
  output=$("$binary" exec --format json -- "$command" 2>&1) || {
    printf '%s\n' "$output" >&2
    return 1
  }
  printf '%s\n' "$output"
}

assert_exec_ok() {
  command=$1
  output=$(exec_json "$command") || return 1
  printf '%s' "$output" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert data["ok"] is True'
}

dump_plugin_tree() {
  echo "Installed plugin files:" >&2
  exec_json "find /root/.local/share/nvim -maxdepth 6 -type f | sort" >&2 || true
}

cd "$project_dir"
export XDG_DATA_HOME="$temp_dir/data"
export NVIM_SANDBOX_HOME="$home_dir"

prepare_manager

echo "CLI: $("$binary" version)"
echo "Runtime: $(runtime_version)"
echo "Plugin manager: $manager"
echo "Image: $image"
echo "Fixture: $project_dir"

if [ "$manager" = custom ]; then
  custom_command="mkdir -p /root/.local/share/nvim/site/pack/custom/start && git clone --depth 1 $plugin_repo /root/.local/share/nvim/site/pack/custom/start/vim-sleuth && nvim --headless +qa"
  "$binary" create \
    --runtime "$runtime" \
    --image "$image" \
    --install nvim,git,curl,ca-certificates \
    --install-command "$install_command" \
    --attach-local-vim-config \
    --plugin-manager custom \
    --plugin-install-command "$custom_command" \
    --keep-running \
    --no-interactive
else
  "$binary" create \
    --runtime "$runtime" \
    --image "$image" \
    --install nvim,git,curl,ca-certificates \
    --install-command "$install_command" \
    --attach-local-vim-config \
    --plugin-manager "$manager" \
    --keep-running \
    --no-interactive
fi

status_json=$("$binary" status --format json)
container_name=$(printf '%s' "$status_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["metadata"]["container_name"])')
printf '%s' "$status_json" | E2E_RUNTIME="$runtime" E2E_MANAGER="$manager" python3 -c '
import json, os, sys
data = json.load(sys.stdin)
metadata = data["metadata"]
assert data["ok"] is True
assert data["status"] == "running"
assert metadata["runtime"] == os.environ["E2E_RUNTIME"]
assert metadata["attach_local_vim_config"] is True
assert metadata["plugin_install_command"]
manager = os.environ["E2E_MANAGER"]
if manager == "native-pack":
    assert metadata["plugin_label"] == "Native pack"
    assert metadata["attach_local_nvim_site"] is True
elif manager == "custom":
    assert metadata["plugin_label"] == "custom"
else:
    assert metadata["plugin_label"] == manager
'

plugin_path=$(expected_path)
echo "Checking installed plugin: $plugin_path"
if ! assert_exec_ok "test -f $plugin_path"; then
  dump_plugin_tree
  exit 1
fi
if ! assert_exec_ok "nvim --headless '+packloadall' '+if empty(globpath(&runtimepath, \"plugin/sleuth.vim\")) | cquit | endif' +qa"; then
  dump_plugin_tree
  exit 1
fi

echo "Destroying sandbox..."
"$binary" destroy --yes --format json >/dev/null
if container_exists "$container_name"; then
  echo "container still exists after destroy: $container_name" >&2
  exit 1
fi
container_name=

echo "$runtime $manager plugin manager E2E passed."
