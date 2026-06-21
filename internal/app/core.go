package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type CreateOptions struct {
	Source               string
	Image                string
	InstallPackages      []string
	InstallCommand       string
	InstallArguments     string
	InstallEditorTools   bool
	EditorTools          []string
	AttachLocalVimConfig bool
	AttachLocalNvimSite  bool
	NeovimVersion        string
	Runtime              string
	Network              *Network
	Connect              bool
	StopOnExit           *bool
	ForceRecreate        bool
	PluginInstallCommand string
	PluginLabel          string
	Progress             func(string, bool)
}

type Service struct {
	Config Config
	State  *State
}

func NewService(cfg Config) (*Service, error) {
	state, err := NewState()
	if err != nil {
		return nil, err
	}
	if err := state.Ensure(); err != nil {
		return nil, err
	}
	return &Service{Config: cfg, State: state}, nil
}

func (s *Service) Context() (Context, error) {
	return ProjectContext("")
}

func (s *Service) selectedBackend(preferred string) (string, Backend, error) {
	runtimeName := preferred
	if runtimeName == "" || runtimeName == "auto" {
		runtimeName = DetectRuntime()
	}
	if runtimeName == "" {
		return "", nil, &Error{Kind: "no-runtime", Message: "No container runtime found.\n\nInstall Apple Containers, Docker, or Podman.", Code: 1}
	}
	backend := BackendFor(runtimeName)
	if backend == nil {
		return runtimeName, nil, &Error{Kind: "backend-unimplemented", Message: "Sandbox runtime is not implemented: " + runtimeName, Code: 1}
	}
	return runtimeName, backend, nil
}

func (s *Service) metadataFor(ctx Context, runtimeName string, source string) Metadata {
	if source == "" {
		source = "default-image"
	}
	mount := "read-write"
	if s.Config.Mount.Readonly {
		mount = "read-only"
	}
	meta := Metadata{
		Runtime:       runtimeName,
		ContainerName: ctx.ContainerName,
		Image:         s.Config.Image,
		Source:        source,
		CreatedAt:     now(),
		LastUsedAt:    now(),
		Workspace:     s.Config.Workspace,
		Mount:         mount,
		ExtraMounts:   []Mount{},
		Network:       Network{Enabled: true, Ports: []string{}},
		StopOnExit:    boolPointer(true),
	}
	if source == "dockerfile" {
		meta.Image = ProjectImageRepository(ctx.ProjectRoot, ctx.WorkspaceIDShort)
		meta.Dockerfile = filepath.Join(ctx.ProjectRoot, s.Config.Dockerfile.Filename)
	}
	return meta
}

func (s *Service) containerOptions(ctx Context, meta Metadata) ContainerOptions {
	return ContainerOptions{
		ProjectRoot:   ctx.ProjectRoot,
		ContainerName: valueOr(meta.ContainerName, ctx.ContainerName),
		Image:         valueOr(meta.Image, s.Config.Image),
		Workspace:     valueOr(meta.Workspace, s.Config.Workspace),
		Readonly:      meta.Mount == "read-only" || s.Config.Mount.Readonly,
		Dockerfile:    meta.Dockerfile,
		ExtraMounts:   meta.ExtraMounts,
		Network:       normalizedNetwork(meta.Network),
	}
}

func normalizedNetwork(network Network) Network {
	if network.Ports == nil {
		network.Ports = []string{}
	}
	if !network.Enabled && network.Name == "" && len(network.Ports) == 0 {
		return Network{Enabled: false, Ports: []string{}}
	}
	network.Enabled = true
	return network
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (s *Service) ensureStarted(backend Backend, opts ContainerOptions, knownStatus string, progress func(string, bool)) error {
	if progress == nil {
		progress = func(string, bool) {}
	}
	status := knownStatus
	if status == "" {
		var err error
		status, err = backend.Status(opts.ContainerName)
		if err != nil {
			return err
		}
	}
	if status == "running" {
		progress("Container already running", true)
		return nil
	}
	progress("Starting "+opts.ContainerName, false)
	if err := backend.Start(opts.ContainerName); err != nil {
		return err
	}
	progress("Container started", true)
	return nil
}

func (s *Service) persistMetadata(ctx Context, metadata Metadata) (*Metadata, error) {
	metadata.LastUsedAt = now()
	if _, err := s.State.WriteDecision(ctx, "enabled"); err != nil {
		return nil, err
	}
	return s.State.WriteProject(ctx, metadata)
}

func (s *Service) Create(opts CreateOptions) (*Metadata, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	source := opts.Source
	if source == "" {
		source = "default-image"
	}
	runtimeName, backend, err := s.selectedBackend(opts.Runtime)
	if err != nil {
		return nil, err
	}
	progress := opts.Progress
	if progress == nil {
		progress = func(string, bool) {}
	}
	requestedProfileChange := opts.Image != "" || len(opts.InstallPackages) > 0 || opts.InstallCommand != "" || opts.InstallArguments != "" || opts.InstallEditorTools || len(opts.EditorTools) > 0 || opts.AttachLocalVimConfig || opts.AttachLocalNvimSite

	if existing, err := s.State.ReadProject(ctx.WorkspaceID); err != nil {
		return nil, err
	} else if existing != nil {
		runtimeChange := opts.Runtime != "" && opts.Runtime != "auto" && opts.Runtime != existing.Runtime
		existingBackend := BackendFor(existing.Runtime)
		if existingBackend != nil {
			status, _ := existingBackend.Status(existing.ContainerName)
			if status != "" {
				if opts.ForceRecreate {
					progress("Destroying existing sandbox", false)
					if err := existingBackend.Destroy(existing.ContainerName); err != nil && !strings.Contains(err.Error(), "not found") {
						return nil, &Error{Kind: "destroy-failed", Message: err.Error(), Code: 1}
					}
					if err := s.State.DeleteProject(ctx.WorkspaceID); err != nil {
						return nil, err
					}
					progress("Existing sandbox destroyed", true)
				} else {
					if requestedProfileChange || runtimeChange {
						return nil, &Error{Kind: "sandbox-exists", Message: "Sandbox already exists. Use `nvim-sandbox create --recreate` to change its runtime, image, install, or mount options.", Code: 1}
					}
					if opts.StopOnExit != nil {
						existing.StopOnExit = opts.StopOnExit
					}
					if err := s.ensureStarted(existingBackend, s.containerOptions(ctx, *existing), status, progress); err != nil {
						return nil, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
					}
					return s.persistMetadata(ctx, *existing)
				}
			}
		}
		if err := s.State.DeleteProject(ctx.WorkspaceID); err != nil {
			return nil, err
		}
	}

	metadata := s.metadataFor(ctx, runtimeName, source)
	metadata.PluginInstallCommand = opts.PluginInstallCommand
	metadata.PluginLabel = opts.PluginLabel
	if opts.StopOnExit != nil {
		metadata.StopOnExit = opts.StopOnExit
	}
	if opts.Network != nil {
		metadata.Network = *opts.Network
		if metadata.Network.Ports == nil {
			metadata.Network.Ports = []string{}
		}
	}

	if source == "default-image" {
		profile := ScopeImageProfile(ImageProfileFor(s.Config, opts.Image, opts.InstallPackages, opts.InstallCommand, opts.InstallArguments, opts.NeovimVersion, opts.InstallEditorTools, opts.EditorTools), ctx.ProjectRoot, ctx.WorkspaceIDShort)
		image, err := EnsureImage(s.State, s.Config, backend, profile, progress)
		if err != nil {
			return nil, &Error{Kind: "build-failed", Message: err.Error(), Code: 1}
		}
		metadata.Image = image
		metadata.BaseImage = profile.BaseImage
		metadata.InstallCommand = profile.InstallCommand
		metadata.InstallArguments = profile.InstallArguments
		metadata.InstallPackages = profile.Packages
		metadata.InstallEditorTools = profile.EditorTools
		metadata.EditorTools = profile.EditorToolNames
		metadata.NeovimVersion = profile.NeovimVersion
		if profile.Managed {
			metadata.ImageProfile = profile.Name
		}
	}

	if opts.AttachLocalVimConfig {
		metadata.AttachLocalVimConfig = true
		metadata.ExtraMounts = append(metadata.ExtraMounts, localVimConfigMounts()...)
	}
	if opts.AttachLocalNvimSite {
		metadata.AttachLocalNvimSite = true
		metadata.ExtraMounts = append(metadata.ExtraMounts, localNvimSiteMounts()...)
	}

	backendOpts := s.containerOptions(ctx, metadata)
	if status, err := backend.Status(backendOpts.ContainerName); err == nil && status != "" {
		if err := s.ensureStarted(backend, backendOpts, status, progress); err != nil {
			return nil, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
		}
		return s.persistMetadata(ctx, metadata)
	}

	if source == "dockerfile" {
		progress("Building image from Dockerfile", false)
		if err := backend.Build(backendOpts); err != nil {
			return nil, &Error{Kind: "build-failed", Message: err.Error(), Code: 1}
		}
		progress("Dockerfile image built", true)
		if len(opts.InstallPackages) > 0 || opts.InstallEditorTools || len(opts.EditorTools) > 0 {
			profile := ScopeImageProfile(ImageProfileFor(s.Config, metadata.Image, opts.InstallPackages, opts.InstallCommand, opts.InstallArguments, opts.NeovimVersion, opts.InstallEditorTools, opts.EditorTools), ctx.ProjectRoot, ctx.WorkspaceIDShort)
			image, err := EnsureImage(s.State, s.Config, backend, profile, progress)
			if err != nil {
				return nil, &Error{Kind: "build-failed", Message: err.Error(), Code: 1}
			}
			metadata.Image = image
			metadata.BaseImage = profile.BaseImage
			metadata.InstallCommand = profile.InstallCommand
			metadata.InstallArguments = profile.InstallArguments
			metadata.InstallPackages = profile.Packages
			metadata.InstallEditorTools = profile.EditorTools
			metadata.EditorTools = profile.EditorToolNames
			metadata.NeovimVersion = profile.NeovimVersion
			if profile.Managed {
				metadata.ImageProfile = profile.Name
			}
			backendOpts = s.containerOptions(ctx, metadata)
		}
	}

	progress("Creating container "+ctx.ContainerName, false)
	if err := backend.Create(backendOpts); err != nil {
		if strings.Contains(err.Error(), "exists") || strings.Contains(err.Error(), "already exists") {
			if err := s.ensureStarted(backend, backendOpts, "", progress); err != nil {
				return nil, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
			}
			return s.persistMetadata(ctx, metadata)
		}
		return nil, &Error{Kind: "create-failed", Message: err.Error(), Code: 1}
	}
	progress("Container created", true)
	if err := s.ensureStarted(backend, backendOpts, "stopped", progress); err != nil {
		return nil, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
	}
	return s.persistMetadata(ctx, metadata)
}

func (s *Service) Open() (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	decision, err := s.State.ReadDecision(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata != nil {
		if backend := BackendFor(metadata.Runtime); backend != nil {
			if err := s.ensureStarted(backend, s.containerOptions(ctx, *metadata), "", nil); err != nil {
				return map[string]any{"action": "error", "error": err.Error(), "context": ctx, "metadata": metadata}, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
			}
		}
		metadata, err = s.persistMetadata(ctx, *metadata)
		if err != nil {
			return nil, err
		}
		return map[string]any{"action": "metadata-found", "context": ctx, "metadata": metadata}, nil
	}
	if decision != nil && decision.Decision == "ignored" {
		return map[string]any{"action": "noop", "reason": "ignored", "context": ctx}, nil
	}
	if decision != nil && decision.Decision == "enabled" {
		return map[string]any{"action": "missing", "context": ctx}, &Error{Kind: "metadata-missing", Message: "Sandbox metadata is missing for this enabled project.", Code: 1}
	}
	if s.Config.Discovery.Mode == "never" {
		return map[string]any{"action": "noop", "reason": "discovery-never", "context": ctx}, nil
	}
	if s.Config.Discovery.Mode == "auto" && !HasDockerfile(ctx.ProjectRoot, s.Config) {
		metadata, err := s.Create(CreateOptions{Source: "default-image"})
		if err != nil {
			return map[string]any{"action": "error", "error": err.Error(), "context": ctx}, err
		}
		return map[string]any{"action": "created", "metadata": metadata, "context": ctx}, nil
	}
	return map[string]any{
		"action":         "approval-required",
		"context":        ctx,
		"has_dockerfile": HasDockerfile(ctx.ProjectRoot, s.Config),
	}, &Error{Kind: "approval-required", Message: "Sandbox creation requires approval.", Code: 3}
}

func (s *Service) Disable() (*Decision, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	return s.State.WriteDecision(ctx, "ignored")
}

func (s *Service) Reset() (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	return map[string]any{"action": "reset", "context": ctx}, s.State.DeleteDecision(ctx.WorkspaceID)
}

func (s *Service) Status() (Status, error) {
	ctx, err := s.Context()
	if err != nil {
		return Status{}, err
	}
	decision, err := s.State.ReadDecision(ctx.WorkspaceID)
	if err != nil {
		return Status{}, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return Status{}, err
	}
	containerStatus := "-"
	if metadata != nil {
		if backend := BackendFor(metadata.Runtime); backend != nil {
			if status, err := backend.Status(metadata.ContainerName); err == nil && status != "" {
				containerStatus = status
			} else {
				containerStatus = "unknown"
			}
		} else {
			containerStatus = "unknown"
		}
	}
	decisionValue := "unknown"
	if decision != nil {
		decisionValue = decision.Decision
	}
	if metadata == nil && decisionValue == "enabled" {
		containerStatus = "recovery-needed"
	}
	status := Status{
		Action:        "status",
		Context:       ctx,
		Decision:      decisionValue,
		Metadata:      metadata,
		ContainerName: ctx.ContainerName,
		Status:        containerStatus,
		Workspace:     s.Config.Workspace,
		Mount:         "read-write",
	}
	if s.Config.Mount.Readonly {
		status.Mount = "read-only"
	}
	if metadata != nil {
		status.Runtime = metadata.Runtime
		status.ContainerName = metadata.ContainerName
		status.Image = metadata.Image
		status.Source = metadata.Source
		status.Dockerfile = metadata.Dockerfile
		status.Workspace = valueOr(metadata.Workspace, s.Config.Workspace)
		status.Mount = valueOr(metadata.Mount, status.Mount)
		status.CreatedAt = metadata.CreatedAt
		status.LastUsedAt = metadata.LastUsedAt
		status.StopOnExit = ShouldStopOnExit(metadata)
	}
	return status, nil
}

func (s *Service) Logs() (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata != nil {
		if backend := BackendFor(metadata.Runtime); backend != nil {
			logs, err := backend.Logs(metadata.ContainerName)
			if err != nil {
				return nil, &Error{Kind: "logs-failed", Message: err.Error(), Code: 1}
			}
			return map[string]any{"action": "logs", "logs": logs, "context": ctx, "metadata": metadata}, nil
		}
	}
	logs, err := s.State.ReadLog(ctx.WorkspaceID)
	return map[string]any{"action": "logs", "logs": logs, "context": ctx}, err
}

func (s *Service) Exec(command string) (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		return map[string]any{"action": "missing", "command": command, "context": ctx}, &Error{Kind: "metadata-missing", Message: "Sandbox metadata is missing for this project.", Code: 1}
	}
	backend := BackendFor(metadata.Runtime)
	if backend == nil {
		return nil, &Error{Kind: "backend-unimplemented", Message: "Command execution is not implemented yet for runtime: " + metadata.Runtime, Code: 1}
	}
	output, err := backend.Exec(s.containerOptions(ctx, *metadata), []string{"/bin/sh", "-lc", command})
	if err != nil {
		return map[string]any{"action": "error", "error": err.Error(), "command": command, "context": ctx}, &Error{Kind: "exec-failed", Message: err.Error(), Code: 1}
	}
	return map[string]any{"action": "exec", "command": command, "output": output, "context": ctx}, nil
}

func (s *Service) ConnectArgs(command []string) ([]string, error) {
	if len(command) == 0 {
		command = []string{"nvim"}
	}
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		metadata, err = s.Create(CreateOptions{Source: "default-image"})
		if err != nil {
			return nil, err
		}
	} else if _, err := s.Open(); err != nil {
		return nil, err
	}
	backend := BackendFor(metadata.Runtime)
	if backend == nil {
		return nil, &Error{Kind: "backend-unimplemented", Message: "Connect is not implemented yet for runtime: " + metadata.Runtime, Code: 1}
	}
	return backend.ConnectArgs(s.containerOptions(ctx, *metadata), command), nil
}

func (s *Service) StopOnExitEnabled() (bool, error) {
	ctx, err := s.Context()
	if err != nil {
		return true, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return true, err
	}
	return ShouldStopOnExit(metadata), nil
}

func ShouldStopOnExit(metadata *Metadata) bool {
	return metadata == nil || metadata.StopOnExit == nil || *metadata.StopOnExit
}

func boolPointer(value bool) *bool {
	return &value
}

func (s *Service) Stop() (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		return map[string]any{"action": "missing", "context": ctx}, &Error{Kind: "metadata-missing", Message: "Sandbox metadata is missing for this project.", Code: 1}
	}
	backend := BackendFor(metadata.Runtime)
	if backend == nil {
		return nil, &Error{Kind: "backend-unimplemented", Message: "Stop is not implemented for this runtime.", Code: 1}
	}
	if err := backend.Stop(metadata.ContainerName); err != nil {
		return nil, &Error{Kind: "stop-failed", Message: err.Error(), Code: 1}
	}
	return map[string]any{"action": "stopped", "context": ctx, "metadata": metadata}, nil
}

func (s *Service) Restart() (map[string]any, error) {
	if _, err := s.Stop(); err != nil {
		var appErr *Error
		if !errors.As(err, &appErr) || appErr.Kind != "metadata-missing" {
			return nil, err
		}
	}
	return s.Open()
}

func (s *Service) Recreate(progress func(string, bool)) (*Metadata, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		return nil, &Error{Kind: "metadata-missing", Message: "Sandbox metadata is missing for this project.", Code: 1}
	}
	backend := BackendFor(metadata.Runtime)
	if backend == nil {
		return nil, &Error{Kind: "backend-unimplemented", Message: "Recreate is not implemented for this runtime.", Code: 1}
	}
	if progress == nil {
		progress = func(string, bool) {}
	}
	if metadata.ImageProfile == "default" {
		profile := ScopeImageProfile(ImageProfileFor(s.Config, "", nil, "", "", "", false, nil), ctx.ProjectRoot, ctx.WorkspaceIDShort)
		image, err := EnsureImage(s.State, s.Config, backend, profile, progress)
		if err != nil {
			return nil, &Error{Kind: "build-failed", Message: err.Error(), Code: 1}
		}
		metadata.Image = image
		metadata.BaseImage = profile.BaseImage
		metadata.InstallCommand = profile.InstallCommand
		metadata.InstallArguments = profile.InstallArguments
		metadata.InstallPackages = profile.Packages
	}
	progress("Destroying existing sandbox", false)
	if err := backend.Destroy(metadata.ContainerName); err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, &Error{Kind: "destroy-failed", Message: err.Error(), Code: 1}
	}
	progress("Existing sandbox destroyed", true)
	progress("Creating container "+metadata.ContainerName, false)
	if err := backend.Create(s.containerOptions(ctx, *metadata)); err != nil {
		return nil, &Error{Kind: "create-failed", Message: err.Error(), Code: 1}
	}
	progress("Container created", true)
	progress("Starting "+metadata.ContainerName, false)
	if err := backend.Start(metadata.ContainerName); err != nil {
		return nil, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
	}
	progress("Container started", true)
	return s.persistMetadata(ctx, *metadata)
}

func (s *Service) Destroy(force bool) (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		return nil, &Error{Kind: "sandbox-not-found", Message: "No sandbox exists for this project.", Code: 1}
	}
	if !force {
		return nil, &Error{Kind: "approval-required", Message: "Destroy requires --yes.", Code: 3}
	}
	if backend := BackendFor(metadata.Runtime); backend != nil {
		if err := backend.Destroy(metadata.ContainerName); err != nil && !strings.Contains(err.Error(), "not found") {
			return nil, &Error{Kind: "destroy-failed", Message: err.Error(), Code: 1}
		}
	}
	return map[string]any{"action": "destroyed-metadata", "context": ctx}, s.State.DeleteProject(ctx.WorkspaceID)
}

func (s *Service) Network(action string, name string, port string, progress func(string, bool)) (map[string]any, error) {
	ctx, err := s.Context()
	if err != nil {
		return nil, err
	}
	metadata, err := s.State.ReadProject(ctx.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if action == "" || action == "status" {
		return map[string]any{
			"action":           "network",
			"context":          ctx,
			"metadata":         metadata,
			"network":          networkFor(metadata),
			"requires_sandbox": metadata == nil,
		}, nil
	}
	if metadata == nil {
		return nil, &Error{Kind: "metadata-missing", Message: "Sandbox metadata is missing for this project.\n\nRun `nvim-sandbox create` first.", Code: 1}
	}
	backend := BackendFor(metadata.Runtime)
	if backend == nil {
		return nil, &Error{Kind: "backend-unimplemented", Message: "Network changes are not implemented yet for runtime: " + metadata.Runtime, Code: 1}
	}
	network := networkFor(metadata)
	switch action {
	case "enable":
		network.Enabled = true
		network.Name = name
	case "disable":
		network.Enabled = false
		network.Name = ""
		network.Ports = []string{}
	case "port-add":
		if port == "" {
			return nil, &Error{Kind: "invalid-usage", Message: "Port is required. Example: nvim-sandbox network port add 3000:3000", Code: 2}
		}
		if !slices.Contains(network.Ports, port) {
			network.Ports = append(network.Ports, port)
		}
	case "port-remove":
		if port == "" {
			return nil, &Error{Kind: "invalid-usage", Message: "Port is required. Example: nvim-sandbox network port remove 3000:3000", Code: 2}
		}
		next := []string{}
		for _, existing := range network.Ports {
			if existing != port {
				next = append(next, existing)
			}
		}
		network.Ports = next
	default:
		return nil, &Error{Kind: "invalid-usage", Message: "Unknown network action: " + action, Code: 2}
	}
	metadata.Network = network
	if progress == nil {
		progress = func(string, bool) {}
	}
	progress("Recreating container "+metadata.ContainerName, false)
	if err := backend.Destroy(metadata.ContainerName); err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, &Error{Kind: "destroy-failed", Message: err.Error(), Code: 1}
	}
	if err := backend.Create(s.containerOptions(ctx, *metadata)); err != nil {
		return nil, &Error{Kind: "create-failed", Message: err.Error(), Code: 1}
	}
	progress("Container recreated", true)
	if err := backend.Start(metadata.ContainerName); err != nil {
		return nil, &Error{Kind: "start-failed", Message: err.Error(), Code: 1}
	}
	progress("Container started", true)
	updated, err := s.persistMetadata(ctx, *metadata)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"action":   "network-updated",
		"context":  ctx,
		"metadata": updated,
		"network":  updated.Network,
	}, nil
}

func networkFor(metadata *Metadata) Network {
	if metadata == nil {
		return Network{Enabled: true, Ports: []string{}}
	}
	network := metadata.Network
	if network.Ports == nil {
		network.Ports = []string{}
	}
	return network
}

type ImageSummary struct {
	Image           string   `json:"image"`
	Source          string   `json:"source,omitempty"`
	BaseImage       string   `json:"base_image,omitempty"`
	Runtime         string   `json:"runtime,omitempty"`
	InstallPackages []string `json:"install_packages,omitempty"`
	NeovimVersion   string   `json:"neovim_version,omitempty"`
	Projects        []string `json:"projects"`
}

func (s *State) ListProjects() ([]Metadata, error) {
	files, err := os.ReadDir(s.projectsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	projects := []Metadata{}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		metadata, err := readJSON[Metadata](filepath.Join(s.projectsDir(), file.Name()))
		if err == nil && metadata != nil {
			projects = append(projects, *metadata)
		}
	}
	return projects, nil
}

func (s *Service) Images() (map[string]any, error) {
	projects, err := s.State.ListProjects()
	if err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	byImage := map[string]*ImageSummary{}
	order := []string{}
	for _, project := range projects {
		if project.Image == "" {
			continue
		}
		summary := byImage[project.Image]
		if summary == nil {
			summary = &ImageSummary{
				Image:           project.Image,
				Source:          project.Source,
				BaseImage:       project.BaseImage,
				Runtime:         project.Runtime,
				InstallPackages: project.InstallPackages,
				NeovimVersion:   project.NeovimVersion,
				Projects:        []string{},
			}
			byImage[project.Image] = summary
			order = append(order, project.Image)
		}
		root := project.ProjectRoot
		if home != "" && strings.HasPrefix(root, home) {
			root = "~" + strings.TrimPrefix(root, home)
		}
		summary.Projects = append(summary.Projects, root)
	}
	images := []ImageSummary{}
	for _, image := range order {
		images = append(images, *byImage[image])
	}
	return map[string]any{"action": "images", "images": images}, nil
}

func (s *Service) Runtime() map[string]any {
	return map[string]any{
		"action":    "runtime",
		"runtime":   DetectRuntime(),
		"available": AvailableRuntimes(),
	}
}

func localVimConfigMounts() []Mount {
	home := os.Getenv("NVIM_SANDBOX_HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	var mounts []Mount
	for _, item := range []Mount{
		{Source: filepath.Join(home, ".config", "nvim"), Target: "/root/.config/nvim", Readonly: true},
		{Source: filepath.Join(home, ".vim"), Target: "/root/.vim", Readonly: true},
		{Source: filepath.Join(home, ".vimrc"), Target: "/root/.vimrc", Readonly: true},
	} {
		if info, err := os.Stat(item.Source); err == nil {
			if info.IsDir() || slices.Contains([]string{"/root/.vimrc"}, item.Target) {
				mounts = append(mounts, item)
			}
		}
	}
	return mounts
}

func localNvimSiteMounts() []Mount {
	home := os.Getenv("NVIM_SANDBOX_HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	site := filepath.Join(home, ".local", "share", "nvim", "site", "pack")
	if info, err := os.Stat(site); err == nil && info.IsDir() {
		return []Mount{{Source: site, Target: "/root/.local/share/nvim/site/pack", Readonly: true}}
	}
	return nil
}
