# nvim-sandbox

[![CI](https://github.com/stasfilin/nvim-sandbox/actions/workflows/ci.yml/badge.svg)](https://github.com/stasfilin/nvim-sandbox/actions/workflows/ci.yml)
[![Release](https://github.com/stasfilin/nvim-sandbox/actions/workflows/release.yml/badge.svg)](https://github.com/stasfilin/nvim-sandbox/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Run Neovim in a persistent, project-scoped development container without adding
container plumbing to your editor configuration or your project repository.

`nvim-sandbox` is a standalone Go CLI. It finds the current project, creates one
sandbox for it, mounts the project at `/workspace`, and opens Neovim inside the
container. There is no Neovim plugin, project config file, or host Neovim
installation to maintain.

`nvim-sandbox` is an independent community project. It is not affiliated with,
sponsored by, or endorsed by the Neovim project.

## Highlights

- Keep compilers, language servers, and project dependencies off the host.
- Reopen the same container instead of rebuilding an environment for every edit.
- Use your existing Neovim configuration through read-only mounts.
- Start from a managed Ubuntu image, another base image, or the project's
  `Dockerfile`.
- Use Apple Container, Docker, or Podman through the same workflow.
- Manage lifecycle, network access, ports, logs, and project commands from one
  CLI.

## Quick start

Requirements:

- macOS or Linux.
- One supported container runtime on `PATH`: [Apple Container](https://github.com/apple/container),
  [Docker](https://docs.docker.com/engine/), or [Podman](https://podman.io/docs/installation).

Runtime auto-detection checks Apple Container first, then Docker, then Podman.

Install with Homebrew:

```sh
brew tap stasfilin/tap https://github.com/stasfilin/nvim-sandbox
brew install stasfilin/tap/nvim-sandbox
```

Then open a project and run the dashboard:

```sh
cd /path/to/project
nvim-sandbox
```

Select **Create sandbox** on the first run. The setup wizard lets you choose the
runtime, image, Neovim version, extra packages, editor tools, network policy,
local configuration mounts, and container lifecycle policy. When setup
finishes, connect to Neovim from the dashboard.

For a non-interactive first run:

```sh
nvim-sandbox create --no-interactive --connect
```

Once the sandbox exists, the dashboard and direct commands reuse it:

```sh
nvim-sandbox
nvim-sandbox connect
```

`connect` starts the container when needed.

Human-readable output shows project directory names by default. Show absolute
paths when needed:

```sh
nvim-sandbox --path full
nvim-sandbox create --path full
nvim-sandbox status --path full
nvim-sandbox images --path full
```

Check the local environment when something looks wrong:

```sh
nvim-sandbox doctor
```

## Common workflows

Open Neovim:

```sh
nvim-sandbox connect
```

Run a command in `/workspace` without opening the editor:

```sh
nvim-sandbox exec -- go test ./...
```

Open an interactive shell:

```sh
nvim-sandbox shell
```

Inspect or stop the sandbox:

```sh
nvim-sandbox status
nvim-sandbox logs
nvim-sandbox stop
```

Create a non-interactive sandbox with a custom toolchain:

```sh
nvim-sandbox create \
  --runtime docker \
  --image fedora:latest \
  --install nvim,git,rg,fd \
  --install-command dnf \
  --install-args '-y' \
  --attach-local-vim-config \
  --connect
```

Use the project's `Dockerfile` as the base:

```sh
nvim-sandbox create --dockerfile
```

The Dockerfile is never built implicitly. Selecting it in the wizard or passing
`--dockerfile` is always required.

Use the CLI from scripts:

```sh
nvim-sandbox status --json
nvim-sandbox runtime --json
nvim-sandbox exec -- make test
```

Commands that can modify or remove state keep interactive confirmations by
default. Use explicit flags such as `--no-interactive`, `--yes`, and
`--format json` when automation needs predictable behavior.

## How it works

To find the project root, the CLI checks these markers in order and walks upward
from the current directory for each one. The first match wins:

```text
.git
package.json
Cargo.toml
go.mod
pyproject.toml
Dockerfile
Makefile
```

If there is no marker, the current directory becomes the project root. A hash of
the resolved root gives the project a stable identity and a container named:

```text
sandbox-<workspace-id-short>
```

The host project is mounted read-write at `/workspace`; it is never copied or
synchronized. The container and its metadata persist between editor sessions.
By default, the container stops when Neovim exits. Choose **Keep running** in the
wizard or pass `--keep-running` to change that policy. Shell sessions never
trigger stop-on-exit.

### Images

There are three image paths:

| Source | Behavior |
| --- | --- |
| Managed default | Builds an Ubuntu 24.04 development image with Neovim and common CLI tools. |
| Custom image | Uses `--image` as the base and optionally installs selected packages and tools. |
| Project Dockerfile | Builds the project image first, then adds selected Neovim tooling in a derived image. |

Managed images are scoped to a project. Their tags include a profile hash based
on the base image, package manager, packages, Neovim version, editor tools, and
internal image recipe. Projects with the same directory name do not share an
image accidentally, and stale default-image recipes are rebuilt automatically.

When no managed additions are selected for a Dockerfile-based sandbox, the
project image is used directly. The project Dockerfile does not need to install
Neovim if the wizard is configured to add it.

### Neovim configuration and plugins

The wizard can mount existing host configuration read-only:

```text
~/.config/nvim -> /root/.config/nvim
~/.vim         -> /root/.vim
~/.vimrc       -> /root/.vimrc
```

This keeps the host configuration as the source of truth while preventing the
container from changing it.

Local native packages can also be mounted read-only:

```text
~/.local/share/nvim/site/pack -> /root/.local/share/nvim/site/pack
```

Plugin installation can be run after sandbox creation for common managers:

```sh
nvim-sandbox create --plugin-manager lazy.nvim
nvim-sandbox create --plugin-manager packer.nvim
nvim-sandbox create --plugin-manager vim-plug
nvim-sandbox create --plugin-manager native-pack
```

For custom plugin setups, provide the command that should run inside the
container:

```sh
nvim-sandbox create \
  --plugin-manager custom \
  --plugin-install-command 'nvim --headless "+Lazy! sync" +qa'
```

### State

Project decisions and metadata live outside the repository:

```text
~/.local/share/nvim-sandbox/
├── decisions/
├── images/
├── logs/
├── projects/
└── update-check.json
```

State files are written atomically. Corrupt metadata is reported instead of
being silently discarded.

## Commands

### Daily use

| Command | Purpose |
| --- | --- |
| `nvim-sandbox` | Open the dashboard in a terminal; otherwise print status. |
| `create` | Create a sandbox for the current project. |
| `connect [-- <cmd>]` | Open Neovim or another interactive command. |
| `shell` | Open Bash in `/workspace`, falling back to `sh`. |
| `exec -- <cmd>` | Run a command in `/workspace`. |
| `status` | Show saved and live runtime state. |
| `logs` | Show container logs. |

### Lifecycle

| Command | Purpose |
| --- | --- |
| `stop` / `restart` | Stop or restart the project container. |
| `destroy [--yes]` | Remove the container and project metadata. |
| `open` / `attach` | Start an existing sandbox. |

### Project decisions

| Command | Purpose |
| --- | --- |
| `enable` / `disable` | Create a sandbox or mark the project ignored. |
| `reset` | Remove the saved project decision. |

### Inspection and maintenance

| Command | Purpose |
| --- | --- |
| `runtime` | Show detected and available runtimes. |
| `images` | List managed images referenced by projects. |
| `network` | Show or change network settings. |
| `update` | Check GitHub for a newer release. |
| `doctor` | Check runtime, state, project, and local dev binary health. |
| `version` | Show the version and VCS information. |

Use the built-in reference for the authoritative command set:

```sh
nvim-sandbox help
nvim-sandbox help all
```

### Creation options

| Option | Purpose |
| --- | --- |
| `--source default-image\|dockerfile` | Choose the image source. |
| `--dockerfile` | Use the project Dockerfile. |
| `--image <image>` | Use a custom base image. |
| `--runtime auto\|apple-container\|docker\|podman` | Override runtime detection. |
| `--install <a,b,c>` | Install additional packages. |
| `--install-command <cmd>` | Set the package manager or a raw install command. |
| `--install-args <args>` | Set package-manager flags. |
| `--install-lsp` | Install the common editor-tool profile. |
| `--lsp-tools <list>` | Install selected built-in tools or npm LSP packages. |
| `--attach-local-vim-config` | Mount existing editor configuration read-only. |
| `--attach-local-nvim-site` | Mount local native packages from `site/pack` read-only. |
| `--plugin-manager <name>` | Run a built-in plugin install command for `native-pack`, `lazy.nvim`, `packer.nvim`, or `vim-plug`. |
| `--plugin-install-command <cmd>` | Run a custom plugin install command inside the container. |
| `--connect` | Connect after creation. |
| `--stop-on-exit` | Stop the container when the editor exits. |
| `--keep-running` | Leave the container running when the editor exits. |
| `--recreate` | Recreate an existing sandbox with the supplied profile. |
| `--no-interactive` | Disable interactive UI. |
| `--format json` / `--json` | Emit machine-readable output. |

Known package aliases are translated for supported package managers. For
example, `nvim` becomes `neovim`, `rg` becomes `ripgrep`, and
`build-essential` becomes `build-base` on Alpine or `gcc gcc-c++ make` with
DNF/YUM.

`--lsp-tools` accepts the built-in tools `gopls`, `lua_ls`, `rust_analyzer`,
and `terraformls`, plus npm-distributed tools such as `pyright`,
`bash-language-server`, `typescript-language-server`,
`vscode-langservers-extracted`, and `yaml-language-server`. Custom npm package
names can be passed directly or with `npm:`:

```sh
nvim-sandbox create --lsp-tools pyright,npm:@tailwindcss/language-server
```

### Network and ports

```sh
nvim-sandbox network status
nvim-sandbox network enable
nvim-sandbox network enable my-network
nvim-sandbox network disable
nvim-sandbox network port add 3000:3000
nvim-sandbox network port remove 3000:3000
```

Port mappings use `HOST:CONTAINER`; both ports must be between 1 and 65535.
Changing network settings recreates the container from its saved image while
preserving project metadata.

### JSON output and exit codes

Commands intended for scripts support JSON output:

```sh
nvim-sandbox status --json
nvim-sandbox runtime --json
nvim-sandbox version --json
```

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Runtime, state, build, or project error. |
| `2` | Invalid command or option usage. |
| `3` | Approval required or an unsafe action was blocked. |
| `130` | Interactive operation was cancelled. |

## Safety model

Containers isolate development dependencies, but they are not a strong security
boundary:

- Dockerfiles can execute arbitrary commands while building.
- The current project is mounted read-write by default.
- Container processes can modify any file in the mounted project.
- Network access is enabled unless disabled during creation or with
  `nvim-sandbox network disable`.

For those reasons, default discovery never creates a sandbox silently, a
detected Dockerfile always requires an explicit choice, and non-interactive
destruction requires `--yes`. Review Dockerfiles and commands before running
untrusted projects.

## Other installation methods

### Release packages

[GitHub Releases](https://github.com/stasfilin/nvim-sandbox/releases) provides
archives and Debian packages for macOS and Linux on `arm64` and `amd64`, plus a
`SHA256SUMS` file.

Install a Debian package:

```sh
sudo dpkg -i nvim-sandbox_amd64.deb
```

Install an archive:

```sh
tar -xzf nvim-sandbox_macos_arm64.tar.gz
sudo install -m 0755 nvim-sandbox_macos_arm64/nvim-sandbox /usr/local/bin/nvim-sandbox
```

Verify downloaded packages before installing them:

```sh
# Linux
sha256sum --check SHA256SUMS

# macOS
shasum --algorithm 256 --check SHA256SUMS
```

### Build from source

Building requires Go 1.26.4 or newer:

```sh
git clone https://github.com/stasfilin/nvim-sandbox.git
cd nvim-sandbox
make check
make build
sudo make install
```

The default destination is `/usr/local/bin/nvim-sandbox`. Override it with
`PREFIX` or `BINDIR`:

```sh
make install PREFIX="$HOME/.local"
```

## Updates

The interactive dashboard checks for a newer GitHub release in the background.
Results are cached for 24 hours and failures do not block normal use. Disable the
check with:

```sh
export NVIM_SANDBOX_NO_UPDATE_CHECK=1
```

Homebrew installations use the normal upgrade flow:

```sh
brew update
brew upgrade nvim-sandbox
```

## Development

```text
cmd/nvim-sandbox/   executable entrypoint
internal/app/       project identity, state, images, and runtime backends
internal/cli/       commands, output, dashboard, wizard, and terminal UI
tests/              install, package, and runtime integration tests
```

Run the same aggregate quality gate used by CI:

```sh
make check
```

Useful targeted checks:

| Command | What it validates |
| --- | --- |
| `make check-static` | Formatting, modules, vet, shell scripts, Homebrew formula generation, and notices. |
| `make test` | Go tests. |
| `make test-race` | Go tests with the race detector. |
| `make test-install` | Staged installs, custom `BINDIR`, uninstall cleanup, and paths with spaces. |
| `make test-packages` | Release archives, checksums, executable payloads, and Debian metadata when available. |

Runtime integration tests are separate because they require the corresponding
runtime:

```sh
make test-e2e-docker
make test-e2e-podman
make test-e2e-apple-container
```

Additional E2E targets cover LSP tools, alternate distributions, and plugin
managers for Docker and Podman. See the `test-e2e-*` targets in the
[`Makefile`](Makefile).

Commits and pull request titles use Conventional Commits. See
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the release rules and targeted checks.

## License

Apache License 2.0. See [`LICENSE`](LICENSE). Licenses for bundled Go
dependencies are collected in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
