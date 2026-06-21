# nvim-sandbox

[![CI](https://github.com/stasfilin/nvim-sandbox/actions/workflows/ci.yml/badge.svg)](https://github.com/stasfilin/nvim-sandbox/actions/workflows/ci.yml)
[![Release](https://github.com/stasfilin/nvim-sandbox/actions/workflows/release.yml/badge.svg)](https://github.com/stasfilin/nvim-sandbox/actions/workflows/release.yml)

`nvim-sandbox` creates persistent, project-scoped development containers and connects Neovim to them from the terminal.

The project is a standalone Go CLI. It does not require Lua, a Neovim plugin, or a separate configuration file inside the project.

Supported runtimes are detected in this order:

1. [Apple Container](https://github.com/apple/container)
2. Docker
3. Podman

## Safety model

`nvim-sandbox` never creates a sandbox silently in its default discovery mode. New projects require an explicit create, enable, or approval action.

Containers reduce host dependency pollution, but they are not a strong security boundary:

- Dockerfiles can execute arbitrary commands while building.
- The project is mounted read-write at `/workspace` by default.
- Building a detected Dockerfile always requires an explicit choice.
- Destroying a sandbox from the non-interactive CLI requires `--yes`.

Review Dockerfiles and commands before running untrusted projects.

## Requirements

- macOS or Linux
- One supported container runtime available on `PATH`
- Go 1.26.4 or newer when building from source

Neovim does not need to be installed on the host. The creation wizard can add a selected Neovim release to the managed image.

When the interactive dashboard opens, it checks for a newer GitHub release in the background. Results are cached for 24 hours, failures are ignored, and an available release is shown with the Homebrew upgrade command. Set `NVIM_SANDBOX_NO_UPDATE_CHECK=1` to disable this network check.

## Installation

### Homebrew

Register this repository as the `stasfilin/tap` Homebrew tap, then install the prebuilt binary:

```sh
brew tap stasfilin/tap https://github.com/stasfilin/nvim-sandbox
brew install stasfilin/tap/nvim-sandbox
```

Homebrew selects the correct package for macOS or Linux and for ARM64 or Intel/AMD64. Future releases are available through the normal upgrade flow:

```sh
brew update
brew upgrade nvim-sandbox
```

### Release packages

[GitHub Releases](https://github.com/stasfilin/nvim-sandbox/releases) provide:

- macOS archives for Apple silicon (`arm64`) and Intel (`amd64`);
- Linux archives for `arm64` and `amd64`;
- Ubuntu/Debian `.deb` packages for `arm64` and `amd64`;
- a `SHA256SUMS` file covering every release package.

Install an Ubuntu/Debian package:

```sh
sudo dpkg -i nvim-sandbox_amd64.deb
```

Install from a macOS or Linux archive:

```sh
tar -xzf nvim-sandbox_macos_arm64.tar.gz
sudo install -m 0755 nvim-sandbox_macos_arm64/nvim-sandbox /usr/local/bin/nvim-sandbox
```

Use the package matching the host platform and architecture. Verify downloads before installation:

```sh
# Linux
sha256sum --check SHA256SUMS

# macOS
shasum --algorithm 256 --check SHA256SUMS
```

### Build from source

```sh
git clone https://github.com/stasfilin/nvim-sandbox.git
cd nvim-sandbox
make check
make build
sudo make install
```

The default installation path is `/usr/local/bin/nvim-sandbox`. Override it with `PREFIX` or `BINDIR`:

```sh
make install PREFIX="$HOME/.local"
```

### Development

Run the CLI directly from source:

```sh
go run ./cmd/nvim-sandbox version
go run ./cmd/nvim-sandbox
```

Compiled release-style binaries are written to the ignored `dist/` directory.

## Quick start

Open a project and start the interactive dashboard:

```sh
cd /path/to/project
nvim-sandbox
```

For a new project, select **Create sandbox**. The wizard lets you choose:

- container runtime;
- default, custom, or Dockerfile-based image;
- whether and which Neovim version to install;
- additional packages and package-manager flags;
- common editor tools;
- network policy and published ports;
- read-only local Neovim configuration mounts;
- plugin installation command;
- whether to connect immediately;
- whether to stop the container when the editor exits.

The equivalent direct flow is:

```sh
nvim-sandbox create
nvim-sandbox connect
```

Run a project command without opening an editor:

```sh
nvim-sandbox exec -- go test ./...
```

Open a shell in the sandbox:

```sh
nvim-sandbox shell
```

Shell sessions do not trigger the editor stop-on-exit policy.

## Dashboard

Running `nvim-sandbox` in an interactive terminal shows:

- CLI version and Git revision;
- detected runtime;
- Dockerfile presence;
- saved project decision;
- container state, image, and name;
- stop-on-exit policy;
- context-sensitive lifecycle actions.

For an existing sandbox:

- **Connect to sandbox** starts a stopped container and opens Neovim.
- **Recreate container (same image)** preserves the saved image, mounts, network configuration, plugin installation command, and lifecycle policy.
- **Destroy and create new** runs the complete creation wizard for a new profile.

After a quick recreate, the CLI asks whether to connect to Neovim. **Yes** is selected by default.

## Dockerfile workflow

When a project contains a `Dockerfile`, it is never built without confirmation.

Dockerfile creation uses two image stages when additional tools are selected:

1. Build the project Dockerfile as the project base image.
2. Build a project-scoped `nvim-sandbox/<project-name>-<workspace-id>:<profile>` image on top with selected Neovim, packages, and editor tools.
3. Create the persistent project container from the derived image.

The project Dockerfile therefore does not need to install Neovim. If no managed additions are selected, the project image is used directly.

Managed image profile keys include the base image, package manager, install arguments, selected packages, Neovim version, editor tools, and internal recipe version. Stale default-image recipes are rebuilt automatically.

Managed images are never shared between projects. Each project uses its own readable `nvim-sandbox/<project-name>-<workspace-id>` repository; profile hashes are tags within that repository. The workspace suffix prevents collisions between projects with the same directory name.

## Project identity and storage

The project root is the nearest ancestor containing the first matching marker:

```text
.git
package.json
Cargo.toml
go.mod
pyproject.toml
Dockerfile
Makefile
```

If no marker exists, the current directory is used.

The workspace ID is the SHA-256 hash of the resolved project root. Containers use a short form:

```text
sandbox-<workspace-id-short>
```

State is stored outside the project:

```text
~/.local/share/nvim-sandbox/
├── decisions/
├── images/
├── logs/
├── projects/
└── update-check.json
```

State files are written atomically. Corrupted metadata is reported instead of silently discarded.

## Workspace and local configuration

The host project is mounted read-write at:

```text
/workspace
```

When local editor configuration is enabled, existing paths are mounted read-only:

```text
~/.config/nvim -> /root/.config/nvim
~/.vim         -> /root/.vim
~/.vimrc       -> /root/.vimrc
```

The host project remains the source of truth. `nvim-sandbox` does not copy the project or run a two-way synchronization daemon.

## Commands

| Command | Purpose |
| --- | --- |
| `nvim-sandbox` | Show the interactive dashboard, or status when non-interactive. |
| `create` | Create a sandbox for the current project. |
| `open` | Start/open an existing sandbox or report that approval is required. |
| `connect [-- <cmd>]` | Open Neovim or another interactive command. |
| `attach` | Open/start the existing sandbox. |
| `shell` | Open `/bin/bash` in `/workspace`, falling back to `/bin/sh`. |
| `exec -- <cmd>` | Execute a command in `/workspace`. |
| `status` | Show saved and live runtime state. |
| `logs` | Show container logs. |
| `stop` | Stop the project container. |
| `restart` | Restart the project container. |
| `destroy [--yes]` | Remove the container and project metadata; prompts interactively unless confirmed with `--yes`. |
| `enable` | Mark the project enabled and create its sandbox. |
| `disable` | Mark the project ignored. |
| `reset` | Remove the saved project decision. |
| `runtime` | Show detected and available runtimes. |
| `update` | Check GitHub for a newer release and show the Homebrew upgrade command. |
| `images` | List managed images referenced by projects. |
| `network` | Show or change network settings. |
| `version` | Show semantic version and VCS information. |

Run the built-in help for the current command set:

```sh
nvim-sandbox help
nvim-sandbox help all
```

## Creation options

Common non-interactive options:

| Option | Purpose |
| --- | --- |
| `--source default-image\|dockerfile` | Select the source image strategy. |
| `--dockerfile` | Shorthand for `--source dockerfile`. |
| `--image <image>` | Use a custom base image. |
| `--runtime auto\|apple-container\|docker\|podman` | Select a runtime explicitly instead of using detection priority. |
| `--install <a,b,c>` | Install additional packages. |
| `--install-command <cmd>` | Select `apt-get`, `apt`, `apk`, `dnf`, `yum`, or a raw command. |
| `--install-args <args>` | Set package-manager flags such as `-y` or `--no-cache`. |
| `--install-lsp` | Install the common editor-tool profile. |
| `--attach-local-vim-config` | Mount existing local editor configuration read-only. |
| `--connect` | Connect after creation. |
| `--stop-on-exit` | Stop the container after the editor exits. |
| `--keep-running` | Keep the container running after the editor exits. |
| `--recreate` | Explicitly destroy and recreate with supplied creation options. |
| `--no-interactive` | Disable interactive UI. |
| `--format json` / `--json` | Emit machine-readable output. |

Example:

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

Known package aliases are normalized for supported package managers. For example, `nvim` becomes `neovim`, `rg` becomes `ripgrep`, and the portable `build-essential` choice becomes `build-base` on Alpine or `gcc gcc-c++ make` on DNF/YUM.

## Network management

```sh
nvim-sandbox network status
nvim-sandbox network enable
nvim-sandbox network enable my-network
nvim-sandbox network disable
nvim-sandbox network port add 3000:3000
nvim-sandbox network port remove 3000:3000
```

Changing network settings recreates the container from its saved image and preserves project metadata.

Published ports use `HOST:CONTAINER` mappings, for example `3000:3000, 8080:80`. Each port must be between 1 and 65535.

## JSON output and exit codes

Commands used by scripts can request JSON:

```sh
nvim-sandbox status --format json
nvim-sandbox runtime --json
nvim-sandbox version --format json
```

Exit codes are stable:

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Runtime, state, build, or project error. |
| `2` | Invalid command or option usage. |
| `3` | Approval required or unsafe action blocked. |
| `130` | Interactive operation cancelled. |

## Versioning

Development builds use the version stored in [`VERSION`](VERSION). Build targets inject it into the binary and Go VCS metadata adds the commit and dirty state:

```sh
nvim-sandbox version
nvim-sandbox version --format json
```

Releases are automated from `main` with [semantic-release](https://github.com/semantic-release/semantic-release) and Conventional Commits. Release tags always remain in the `v0.x.x` series:

| Commit | Release |
| --- | --- |
| `feat: ...` | Minor: `v0.1.0` → `v0.2.0` |
| `fix: ...` or `perf: ...` | Patch: `v0.1.0` → `v0.1.1` |
| `feat!: ...` or `BREAKING CHANGE:` | Minor while the project is pre-1.0 |
| `docs`, `test`, `build`, `ci`, `chore`, `refactor`, `style` | No release by default |

The release pipeline rejects any calculated version outside `v0.x.x`, rebuilds every platform binary with the selected version, commits the updated `VERSION` file back to `main`, creates the Git tag and GitHub release notes, and attaches all archives, Debian packages, and checksums. The generated `chore(release)` commit uses `[skip ci]` to avoid another pipeline run.

Commits and pull request titles are checked with commitlint. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for accepted message examples. Node-based release tooling is installed only in CI; normal local development does not require npm dependencies.

## Development

The repository is CLI-only Go code:

```text
cmd/nvim-sandbox/   executable entrypoint
internal/app/       state, project identity, images, and runtime backends
internal/cli/       parsing, output, dashboard, wizard, and terminal UX
```

Run the complete local quality gate:

```sh
make check
```

Individual targets:

```sh
make fmt
make fmt-check
make mod-check
make vet
make shellcheck
make test
make test-race
make test-e2e-docker
make test-e2e-apple-container
make build
```

CI and release publishing are separate workflows. The CI pipeline is dependency-gated:

```text
commit lint ─┐
code quality ─┼→ builds → Docker E2E → packages
tests ────────┘

successful CI push to main → semantic release
```

- Commit lint checks Conventional Commit messages and pull request titles.
- Code quality checks formatting, module integrity, `go vet`, and every shell script with ShellCheck.
- Race-enabled tests run on Linux and macOS.
- Builds produce Linux and macOS binaries for `amd64` and `arm64`.
- Docker E2E runs on the GitHub-hosted Ubuntu Docker daemon and validates create, status, bind mounts, stop, reopen, and destroy.
- Packaging produces platform archives, Debian packages, package manifests, and SHA-256 checksums.
- The separate release workflow runs only after CI succeeds for a push to `main`, and publishes the exact commit validated by CI.

On `main`, release artifacts use the `nvim-sandbox` basename. Pull requests and other branches use `nvim-sandbox-dev-<commit-sha>` so development artifacts cannot be confused with releases.

## License

Apache License 2.0. See [`LICENSE`](LICENSE).
