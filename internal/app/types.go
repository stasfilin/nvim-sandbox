package app

type Config struct {
	Image        string
	Workspace    string
	Discovery    DiscoveryConfig
	DefaultImage DefaultImageConfig
	Dockerfile   DockerfileConfig
	Mount        MountConfig
}

type DiscoveryConfig struct {
	Mode string
}

type DefaultImageConfig struct {
	Enabled          bool
	Tag              string
	InstallCommand   string
	InstallArguments string
	Packages         []string
}

type DockerfileConfig struct {
	Filename    string
	ImagePrefix string
}

type MountConfig struct {
	Readonly bool
}

type Context struct {
	ProjectRoot      string `json:"project_root"`
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceIDShort string `json:"workspace_id_short"`
	ContainerName    string `json:"container_name"`
}

type Decision struct {
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceIDShort string `json:"workspace_id_short"`
	ProjectRoot      string `json:"project_root"`
	Decision         string `json:"decision"`
	UpdatedAt        string `json:"updated_at"`
}

type Mount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Readonly bool   `json:"readonly"`
}

type Network struct {
	Enabled bool     `json:"enabled"`
	Name    string   `json:"name,omitempty"`
	Ports   []string `json:"ports"`
}

type Metadata struct {
	WorkspaceID          string   `json:"workspace_id"`
	WorkspaceIDShort     string   `json:"workspace_id_short"`
	ProjectRoot          string   `json:"project_root"`
	Runtime              string   `json:"runtime"`
	ContainerName        string   `json:"container_name"`
	Image                string   `json:"image"`
	BaseImage            string   `json:"base_image,omitempty"`
	InstallCommand       string   `json:"install_command,omitempty"`
	InstallArguments     string   `json:"install_arguments,omitempty"`
	InstallPackages      []string `json:"install_packages,omitempty"`
	InstallEditorTools   bool     `json:"install_editor_tools,omitempty"`
	EditorTools          []string `json:"editor_tools,omitempty"`
	NeovimVersion        string   `json:"neovim_version,omitempty"`
	ImageProfile         string   `json:"image_profile,omitempty"`
	Source               string   `json:"source"`
	Dockerfile           string   `json:"dockerfile,omitempty"`
	CreatedAt            string   `json:"created_at"`
	LastUsedAt           string   `json:"last_used_at"`
	Workspace            string   `json:"workspace"`
	Mount                string   `json:"mount"`
	ExtraMounts          []Mount  `json:"extra_mounts"`
	AttachLocalVimConfig bool     `json:"attach_local_vim_config,omitempty"`
	AttachLocalNvimSite  bool     `json:"attach_local_nvim_site,omitempty"`
	PluginInstallCommand string   `json:"plugin_install_command,omitempty"`
	PluginLabel          string   `json:"plugin_label,omitempty"`
	StopOnExit           *bool    `json:"stop_on_exit,omitempty"`
	Network              Network  `json:"network"`
}

type Status struct {
	Action        string    `json:"action"`
	Context       Context   `json:"context"`
	Decision      string    `json:"decision"`
	Metadata      *Metadata `json:"metadata,omitempty"`
	Runtime       string    `json:"runtime,omitempty"`
	ContainerName string    `json:"container_name"`
	Image         string    `json:"image,omitempty"`
	Source        string    `json:"source,omitempty"`
	Dockerfile    string    `json:"dockerfile,omitempty"`
	Status        string    `json:"status"`
	Workspace     string    `json:"workspace"`
	Mount         string    `json:"mount"`
	StopOnExit    bool      `json:"stop_on_exit"`
	CreatedAt     string    `json:"created_at,omitempty"`
	LastUsedAt    string    `json:"last_used_at,omitempty"`
}

type Error struct {
	Kind    string
	Message string
	Code    int
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Kind
}

func DefaultConfig() Config {
	return Config{
		Image:     "ubuntu:24.04",
		Workspace: "/workspace",
		Discovery: DiscoveryConfig{
			Mode: "never",
		},
		DefaultImage: DefaultImageConfig{
			Enabled:          true,
			Tag:              "nvim-sandbox-default:ubuntu-24.04",
			InstallCommand:   "apt-get",
			InstallArguments: "-y --no-install-recommends",
			Packages: []string{
				"bash",
				"build-essential",
				"ca-certificates",
				"curl",
				"ping",
				"fd-find",
				"git",
				"neovim",
				"python3",
				"python3-pip",
				"ripgrep",
				"vim",
			},
		},
		Dockerfile: DockerfileConfig{
			Filename:    "Dockerfile",
			ImagePrefix: "nvim-sandbox",
		},
		Mount: MountConfig{
			Readonly: false,
		},
	}
}
