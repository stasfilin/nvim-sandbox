package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}

func TestProjectRootFollowsMarkerPriority(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "app", "src")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ProjectRoot(child)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ProjectRoot() = %q, want %q", got, want)
	}
}

func TestDetectInstallCommandFromImageName(t *testing.T) {
	tests := map[string]string{
		"ubuntu:24.04":             "apt-get",
		"docker.io/library/debian": "apt-get",
		"fedora:latest":            "dnf",
		"rockylinux/rocky:9":       "dnf",
		"alpine:3.20":              "apk",
		"centos:stream9":           "yum",
	}
	for image, want := range tests {
		if got := DetectInstallCommand(image); got != want {
			t.Fatalf("DetectInstallCommand(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestDefaultInstallArguments(t *testing.T) {
	tests := map[string]string{
		"apt-get": "-y --no-install-recommends",
		"apk":     "--no-cache",
		"dnf":     "-y",
		"yum":     "-y",
		"custom":  "",
	}
	for command, want := range tests {
		if got := DefaultInstallArguments(command); got != want {
			t.Fatalf("DefaultInstallArguments(%q) = %q, want %q", command, got, want)
		}
	}
}

func TestDockerLikeBackendLifecycleCommands(t *testing.T) {
	for _, runtimeName := range []string{"docker", "podman"} {
		t.Run(runtimeName, func(t *testing.T) {
			commands := [][]string{}
			SetRunnerForTests(func(args []string) CommandResult {
				commands = append(commands, append([]string{}, args...))
				if len(args) > 1 && args[1] == "inspect" {
					return CommandResult{Code: 0, Stdout: `[{"State":{"Status":"running"}}]`}
				}
				return CommandResult{Code: 0, Stdout: "output"}
			})
			t.Cleanup(func() { SetRunnerForTests(nil) })

			backend := BackendFor(runtimeName)
			if backend == nil {
				t.Fatalf("BackendFor(%q) returned nil", runtimeName)
			}
			if backend.Name() != runtimeName {
				t.Fatalf("backend name = %q, want %q", backend.Name(), runtimeName)
			}
			opts := ContainerOptions{
				ProjectRoot:   "/host/project",
				ContainerName: "sandbox-123456",
				Image:         "ubuntu:24.04",
				Workspace:     "/workspace",
				Network:       Network{Enabled: true, Ports: []string{"3000:3000"}},
			}
			if err := backend.Create(opts); err != nil {
				t.Fatal(err)
			}
			if err := backend.Start(opts.ContainerName); err != nil {
				t.Fatal(err)
			}
			if _, err := backend.Exec(opts, []string{"go", "test", "./..."}); err != nil {
				t.Fatal(err)
			}
			status, err := backend.Status(opts.ContainerName)
			if err != nil {
				t.Fatal(err)
			}
			if status != "running" {
				t.Fatalf("status = %q, want running", status)
			}
			if err := backend.Stop(opts.ContainerName); err != nil {
				t.Fatal(err)
			}
			if err := backend.Destroy(opts.ContainerName); err != nil {
				t.Fatal(err)
			}

			wantCreate := []string{
				runtimeName, "create", "--name", "sandbox-123456",
				"--mount", "type=bind,source=/host/project,target=/workspace",
				"-p", "3000:3000", "-w", "/workspace", "ubuntu:24.04",
				"/bin/sh", "-lc", "while sleep 3600; do :; done",
			}
			if !slices.Equal(commands[0], wantCreate) {
				t.Fatalf("create command = %#v, want %#v", commands[0], wantCreate)
			}
			if got := backend.ConnectArgs(opts, []string{"nvim"}); !slices.Equal(got, []string{
				runtimeName, "exec", "-it", "-w", "/workspace", "sandbox-123456",
				"/usr/bin/env", "NVIM_SANDBOX=1", "nvim",
			}) {
				t.Fatalf("connect command = %#v", got)
			}
		})
	}
}

func TestDockerLikeBackendBuildAndImageCommands(t *testing.T) {
	for _, runtimeName := range []string{"docker", "podman"} {
		t.Run(runtimeName, func(t *testing.T) {
			commands := [][]string{}
			SetRunnerForTests(func(args []string) CommandResult {
				commands = append(commands, append([]string{}, args...))
				return CommandResult{Code: 0, Stdout: `[{"Id":"sha256:1234"}]`}
			})
			t.Cleanup(func() { SetRunnerForTests(nil) })

			backend := BackendFor(runtimeName)
			opts := ContainerOptions{
				ProjectRoot: "/host/project",
				Dockerfile:  "/host/project/Dockerfile",
				Image:       "nvim-sandbox-project:latest",
			}
			if err := backend.Build(opts); err != nil {
				t.Fatal(err)
			}
			exists, err := backend.ImageStatus(opts.Image)
			if err != nil {
				t.Fatal(err)
			}
			if !exists {
				t.Fatal("built image was not detected")
			}

			wantBuild := []string{runtimeName, "build"}
			if runtimeName == "docker" {
				wantBuild = append(wantBuild, "--progress", "plain")
			}
			wantBuild = append(wantBuild, "-f", opts.Dockerfile, "-t", opts.Image, opts.ProjectRoot)
			want := [][]string{wantBuild, {runtimeName, "image", "inspect", opts.Image}}
			if !slices.Equal(commands[0], want[0]) || !slices.Equal(commands[1], want[1]) {
				t.Fatalf("commands = %#v, want %#v", commands, want)
			}
		})
	}
}

func TestDestroyRejectsMissingSandbox(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	chdir(t, root)
	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Destroy(true)
	appErr, ok := err.(*Error)
	if !ok || appErr.Kind != "sandbox-not-found" || appErr.Code != 1 {
		t.Fatalf("Destroy() error = %#v", err)
	}
}

func TestDisableStoresDecisionOutsideProject(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	chdir(t, root)

	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := service.Disable()
	if err != nil {
		t.Fatal(err)
	}
	if decision.Decision != "ignored" {
		t.Fatalf("decision = %q, want ignored", decision.Decision)
	}
	if _, err := os.Stat(filepath.Join(root, ".nvim-sandbox.lua")); !os.IsNotExist(err) {
		t.Fatalf("unexpected project-local config file err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "nvim-sandbox", "decisions", decision.WorkspaceID+".json")); err != nil {
		t.Fatalf("decision file was not written: %v", err)
	}
}

func TestStateWritesJSONAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := writeJSON(path, map[string]string{"status": "ready"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"status\":\"ready\"}\n" {
		t.Fatalf("state data = %q", data)
	}
	temporary, err := filepath.Glob(filepath.Join(dir, ".nvim-sandbox-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 {
		t.Fatalf("temporary state files were not cleaned up: %#v", temporary)
	}
}

func TestStatusReportsCorruptedMetadata(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	chdir(t, root)
	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := service.Context()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.State.projectPath(ctx.WorkspaceID), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Status(); err == nil {
		t.Fatal("Status should report corrupted metadata")
	}
}

func TestCreateAppleContainerStoresMetadataAfterStart(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	chdir(t, root)
	commands := [][]string{}
	SetRunnerForTests(func(args []string) CommandResult {
		commands = append(commands, append([]string{}, args...))
		if args[1] == "image" || args[1] == "inspect" {
			return CommandResult{Code: 1, Stderr: "not found"}
		}
		return CommandResult{Code: 0}
	})
	t.Cleanup(func() { SetRunnerForTests(nil) })

	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := service.Create(CreateOptions{
		Runtime:              "apple-container",
		Source:               "default-image",
		PluginInstallCommand: "nvim --headless +qa",
		PluginLabel:          "native pack",
	})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Runtime != "apple-container" {
		t.Fatalf("runtime = %q", metadata.Runtime)
	}
	if metadata.Image != ProjectImageRepository(metadata.ProjectRoot, metadata.WorkspaceIDShort)+":default" {
		t.Fatalf("image = %q", metadata.Image)
	}
	if metadata.ContainerName[:8] != "sandbox-" {
		t.Fatalf("container name = %q", metadata.ContainerName)
	}
	if !ShouldStopOnExit(metadata) {
		t.Fatal("new sandbox should stop on editor exit by default")
	}
	if metadata.PluginInstallCommand != "nvim --headless +qa" || metadata.PluginLabel != "native pack" {
		t.Fatalf("plugin metadata = command %q label %q", metadata.PluginInstallCommand, metadata.PluginLabel)
	}
	if len(commands) != 5 {
		t.Fatalf("commands = %#v", commands)
	}
	want := []string{"image", "build", "inspect", "create", "start"}
	for index, command := range want {
		if commands[index][1] != command {
			t.Fatalf("commands[%d] = %q, want %q", index, commands[index][1], command)
		}
	}
}

func TestDefaultImageCacheMarkerInvalidatesStaleImage(t *testing.T) {
	state := &State{base: t.TempDir()}
	if err := state.Ensure(); err != nil {
		t.Fatal(err)
	}
	builds := 0
	SetRunnerForTests(func(args []string) CommandResult {
		if len(args) > 1 && args[1] == "image" {
			return CommandResult{Code: 0, Stdout: `[{"status":"ready"}]`}
		}
		if len(args) > 1 && args[1] == "build" {
			builds++
		}
		return CommandResult{Code: 0}
	})
	t.Cleanup(func() { SetRunnerForTests(nil) })
	cfg := DefaultConfig()
	profile := ImageProfileFor(cfg, "", nil, "", "", "", false, nil)
	if profile.CacheKey == "" {
		t.Fatal("default profile cache key is empty")
	}
	backend := BackendFor("apple-container")
	if _, err := EnsureImage(state, cfg, backend, profile, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureImage(state, cfg, backend, profile, nil); err != nil {
		t.Fatal(err)
	}
	if builds != 1 {
		t.Fatalf("builds = %d, want one rebuild followed by cache reuse", builds)
	}
}

func TestManagedImageProfilesAreScopedToProject(t *testing.T) {
	profile := ImageProfileFor(DefaultConfig(), "ubuntu:24.04", []string{"neovim"}, "apt-get", "", "stable", false, nil)
	first := ScopeImageProfile(profile, "/code/dotfiles", "a1b2c3")
	second := ScopeImageProfile(profile, "/archive/dotfiles", "d4e5f6")
	if first.Image != "nvim-sandbox/dotfiles-a1b2c3:"+profile.Name {
		t.Fatalf("first image = %q", first.Image)
	}
	if second.Image != "nvim-sandbox/dotfiles-d4e5f6:"+profile.Name {
		t.Fatalf("second image = %q", second.Image)
	}
	if first.Image == second.Image {
		t.Fatalf("projects share managed image %q", first.Image)
	}
}

func TestProjectImageRepositorySanitizesProjectName(t *testing.T) {
	got := ProjectImageRepository("/code/My Project!", "abc123")
	if got != "nvim-sandbox/my-project-abc123" {
		t.Fatalf("repository = %q", got)
	}
}

func TestProjectImageRepositoryLimitsLongProjectNames(t *testing.T) {
	got := ProjectImageRepository("/code/"+strings.Repeat("a", 200), "abc123")
	if len(got) > 100 {
		t.Fatalf("repository length = %d, want at most 100: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "-abc123") {
		t.Fatalf("repository lost workspace suffix: %q", got)
	}
}

func TestStopOnExitPreferenceDefaultsAndOverrides(t *testing.T) {
	if !ShouldStopOnExit(nil) || !ShouldStopOnExit(&Metadata{}) {
		t.Fatal("missing stop-on-exit preference should default to true")
	}
	stop := false
	if ShouldStopOnExit(&Metadata{StopOnExit: &stop}) {
		t.Fatal("explicit keep-running preference was ignored")
	}
}

func TestRecreatePreservesExistingSandboxProfile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	chdir(t, root)
	commands := [][]string{}
	SetRunnerForTests(func(args []string) CommandResult {
		commands = append(commands, append([]string{}, args...))
		return CommandResult{Code: 0}
	})
	t.Cleanup(func() { SetRunnerForTests(nil) })
	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := service.Context()
	if err != nil {
		t.Fatal(err)
	}
	stopOnExit := false
	existing := Metadata{
		Runtime:       "apple-container",
		ContainerName: ctx.ContainerName,
		Image:         "nvim-sandbox-dev:profile123",
		BaseImage:     "project-image:latest",
		Source:        "dockerfile",
		Dockerfile:    filepath.Join(root, "Dockerfile"),
		Workspace:     "/workspace",
		Mount:         "read-write",
		StopOnExit:    &stopOnExit,
		Network:       Network{Enabled: true, Ports: []string{"3000:3000"}},
	}
	if _, err := service.State.WriteProject(ctx, existing); err != nil {
		t.Fatal(err)
	}
	recreated, err := service.Recreate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if recreated.Image != existing.Image || recreated.BaseImage != existing.BaseImage || recreated.Source != "dockerfile" {
		t.Fatalf("recreated metadata = %#v", recreated)
	}
	if ShouldStopOnExit(recreated) {
		t.Fatal("recreate did not preserve keep-running preference")
	}
	if len(commands) != 3 || commands[0][1] != "delete" || commands[1][1] != "create" || commands[2][1] != "start" {
		t.Fatalf("commands = %#v", commands)
	}
}

func TestCreateCanForceRecreateExistingSandbox(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	chdir(t, root)
	commands := [][]string{}
	containerExists := true
	SetRunnerForTests(func(args []string) CommandResult {
		commands = append(commands, append([]string{}, args...))
		if args[1] == "delete" {
			containerExists = false
			return CommandResult{Code: 0}
		}
		if args[1] == "create" {
			containerExists = true
			return CommandResult{Code: 0}
		}
		if args[1] == "image" {
			return CommandResult{Code: 0, Stdout: "[]"}
		}
		if args[1] == "inspect" {
			if !containerExists {
				return CommandResult{Code: 1, Stderr: "not found"}
			}
			return CommandResult{Code: 0, Stdout: `[{"status":"running"}]`}
		}
		return CommandResult{Code: 0}
	})
	t.Cleanup(func() { SetRunnerForTests(nil) })

	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := service.Context()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.State.WriteProject(ctx, Metadata{
		Runtime:       "apple-container",
		ContainerName: ctx.ContainerName,
		Image:         "nvim-sandbox-default:ubuntu-24.04",
		Source:        "default-image",
		Workspace:     "/workspace",
		Mount:         "read-write",
		CreatedAt:     "2026-06-19T00:00:00Z",
		LastUsedAt:    "2026-06-19T00:00:00Z",
		Network:       Network{Enabled: true, Ports: []string{}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = service.Create(CreateOptions{
		Runtime:            "apple-container",
		Source:             "default-image",
		InstallEditorTools: true,
		ForceRecreate:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	foundDelete := false
	foundCreate := false
	for _, command := range commands {
		if command[1] == "delete" {
			foundDelete = true
		}
		if command[1] == "create" {
			foundCreate = true
		}
	}
	if !foundDelete || !foundCreate {
		t.Fatalf("commands = %#v, want delete and create", commands)
	}
}

func TestNetworkPortAddRecreatesContainer(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	chdir(t, root)
	commands := [][]string{}
	SetRunnerForTests(func(args []string) CommandResult {
		commands = append(commands, append([]string{}, args...))
		return CommandResult{Code: 0}
	})
	t.Cleanup(func() { SetRunnerForTests(nil) })

	service, err := NewService(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := service.Context()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.State.WriteProject(ctx, Metadata{
		Runtime:       "apple-container",
		ContainerName: ctx.ContainerName,
		Image:         "nvim-sandbox-dev:test",
		Source:        "default-image",
		Workspace:     "/workspace",
		Mount:         "read-write",
		Network:       Network{Enabled: true, Ports: []string{}},
		CreatedAt:     "2026-06-19T00:00:00Z",
		LastUsedAt:    "2026-06-19T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Network("port-add", "", "3000:3000", nil); err != nil {
		t.Fatal(err)
	}
	metadata, err := service.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata.Network.Ports; len(got) != 1 || got[0] != "3000:3000" {
		t.Fatalf("ports = %#v", got)
	}
	createArgs := []string{}
	for _, command := range commands {
		if command[1] == "create" {
			createArgs = command
		}
	}
	if !slices.Contains(createArgs, "--publish") || !slices.Contains(createArgs, "3000:3000") {
		t.Fatalf("create args = %#v, want published port", createArgs)
	}
}

func TestImageDockerfileInstallsNeovimReleaseForApt(t *testing.T) {
	profile := ImageProfileFor(DefaultConfig(), "debian:12", []string{"nvim", "rg"}, "apt-get", "", "nightly", false, nil)
	if got, want := profile.Packages, []string{"neovim", "ripgrep"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("packages = %#v, want %#v", got, want)
	}
	dockerfile := ImageDockerfile(profile.BaseImage, profile.InstallCommand, profile.InstallArguments, profile.Packages, "/workspace", profile.NeovimVersion, profile.EditorToolNames)
	for _, needle := range []string{
		"apt-get install",
		"ripgrep",
		"github.com/neovim/neovim/releases/download/nightly",
		"/usr/local/bin/nvim",
	} {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing %q:\n%s", needle, dockerfile)
		}
	}
}

func TestEditorToolsProfileInstallsCommonLSPCommands(t *testing.T) {
	profile := ImageProfileFor(DefaultConfig(), "ubuntu:24.04", []string{"nvim"}, "apt-get", "", "stable", true, nil)
	dockerfile := ImageDockerfile(profile.BaseImage, profile.InstallCommand, profile.InstallArguments, profile.Packages, "/workspace", profile.NeovimVersion, profile.EditorToolNames)
	for _, needle := range []string{
		"golang-go",
		"golang.org/x/tools/gopls@latest",
		"github.com/hashicorp/terraform-ls@latest",
		"https://api.github.com/repos/LuaLS/lua-language-server/releases/latest",
		"https://github.com/rust-lang/rust-analyzer/releases/latest/download/rust-analyzer-${rust_arch}.gz",
		"/usr/local/bin/lua_ls",
		"/usr/local/bin/rust_analyzer",
		"/usr/local/bin/terraformls",
	} {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing %q:\n%s", needle, dockerfile)
		}
	}
	for _, line := range strings.Split(dockerfile, "\n") {
		if strings.Contains(line, "apt-get install") && (strings.Contains(line, "lua-language-server") || strings.Contains(line, "rust-analyzer")) {
			t.Fatalf("Dockerfile should not install unavailable LSP packages via apt:\n%s", dockerfile)
		}
	}
}

func TestFedoraEditorToolsProfileUsesDnf(t *testing.T) {
	profile := ImageProfileFor(DefaultConfig(), "fedora:latest", []string{"nvim", "rg", "build-essential"}, "dnf", "-y --setopt=install_weak_deps=False", "stable", true, []string{"rust_analyzer"})
	dockerfile := ImageDockerfile(profile.BaseImage, profile.InstallCommand, profile.InstallArguments, profile.Packages, "/workspace", profile.NeovimVersion, profile.EditorToolNames)
	for _, needle := range []string{
		"RUN dnf install -y",
		"--setopt=install_weak_deps=False",
		"neovim",
		"ripgrep",
		"gcc",
		"gcc-c++",
		"make",
		"curl",
		"gzip",
		"python3",
		"https://github.com/rust-lang/rust-analyzer/releases/latest/download/rust-analyzer-${rust_arch}.gz",
	} {
		if !strings.Contains(dockerfile, needle) {
			t.Fatalf("Dockerfile missing %q:\n%s", needle, dockerfile)
		}
	}
	if strings.Contains(dockerfile, "editor tools profile currently requires apt") {
		t.Fatalf("Dockerfile still rejects non-apt editor tools:\n%s", dockerfile)
	}
	if strings.Contains(dockerfile, "build-essential") {
		t.Fatalf("Dockerfile contains Debian-only build-essential package:\n%s", dockerfile)
	}
}

func TestPortableBuildPackageAliases(t *testing.T) {
	tests := []struct {
		command  string
		packages []string
		want     []string
	}{
		{command: "apt-get", packages: []string{"build-essential"}, want: []string{"build-essential"}},
		{command: "apk", packages: []string{"build-essential"}, want: []string{"build-base"}},
		{command: "dnf", packages: []string{"build-essential"}, want: []string{"gcc", "gcc-c++", "make"}},
		{command: "yum", packages: []string{"build-essential", "gcc"}, want: []string{"gcc", "gcc-c++", "make"}},
		{command: "apt-get", packages: []string{"curl", "ping"}, want: []string{"curl", "iputils-ping"}},
		{command: "apk", packages: []string{"curl", "ping"}, want: []string{"curl", "iputils"}},
		{command: "dnf", packages: []string{"curl", "ping"}, want: []string{"curl", "iputils"}},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			got := NormalizeInstallPackages(test.command, test.packages)
			if !slices.Equal(got, test.want) {
				t.Fatalf("NormalizeInstallPackages(%q, %#v) = %#v, want %#v", test.command, test.packages, got, test.want)
			}
		})
	}
}

func TestDefaultImagePackagesUseInstallCommandAliases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Image = "fedora:latest"
	cfg.DefaultImage.InstallCommand = "dnf"

	profile := ImageProfileFor(cfg, "", nil, "", "", "", false, nil)
	if slices.Contains(profile.Packages, "build-essential") {
		t.Fatalf("default image packages contain Debian-only build-essential: %#v", profile.Packages)
	}
	for _, want := range []string{"gcc", "gcc-c++", "make"} {
		if !slices.Contains(profile.Packages, want) {
			t.Fatalf("default image packages = %#v, missing %q", profile.Packages, want)
		}
	}
}
