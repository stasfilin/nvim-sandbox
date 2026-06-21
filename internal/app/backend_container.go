package app

type appleContainerBackend struct{}

func (appleContainerBackend) Name() string { return "apple-container" }

func (appleContainerBackend) Build(opts ContainerOptions) error {
	_, err := run([]string{
		"container", "build", "--progress", "plain",
		"-f", opts.Dockerfile,
		"-t", opts.Image,
		opts.ProjectRoot,
	})
	return err
}

func (appleContainerBackend) ImageStatus(image string) (bool, error) {
	result, err := run([]string{"container", "image", "inspect", image})
	if err != nil {
		return false, nil
	}
	return inspectStatus(result.Stdout) != "", nil
}

func (appleContainerBackend) Create(opts ContainerOptions) error {
	args := []string{"container", "create", "--name", opts.ContainerName}
	args = append(args, "--mount", mountSpec(Mount{Source: opts.ProjectRoot, Target: opts.Workspace, Readonly: opts.Readonly}))
	for _, mount := range opts.ExtraMounts {
		args = append(args, "--mount", mountSpec(mount))
	}
	if opts.Network.Enabled == false {
		args = append(args, "--network", "none")
	} else if opts.Network.Name != "" {
		args = append(args, "--network", opts.Network.Name)
	}
	if opts.Network.Enabled {
		for _, port := range opts.Network.Ports {
			args = append(args, "--publish", port)
		}
	}
	args = append(args, "--workdir", opts.Workspace, opts.Image, "/bin/sh", "-lc", "while sleep 3600; do :; done")
	_, err := run(args)
	return err
}

func (appleContainerBackend) Start(name string) error {
	_, err := run([]string{"container", "start", name})
	return err
}

func (appleContainerBackend) Stop(name string) error {
	_, err := run([]string{"container", "stop", name})
	return err
}

func (appleContainerBackend) Destroy(name string) error {
	_, err := run([]string{"container", "delete", "--force", name})
	return err
}

func (appleContainerBackend) Exec(opts ContainerOptions, command []string) (string, error) {
	args := []string{"container", "exec", "--workdir", opts.Workspace, opts.ContainerName}
	args = append(args, command...)
	result, err := run(args)
	return result.Stdout, err
}

func (appleContainerBackend) ConnectArgs(opts ContainerOptions, command []string) []string {
	args := []string{"container", "exec", "--interactive", "--tty", "--workdir", opts.Workspace, opts.ContainerName}
	return append(args, sandboxEnvCommand(command)...)
}

func (appleContainerBackend) Logs(name string) (string, error) {
	result, err := run([]string{"container", "logs", name})
	return result.Stdout, err
}

func (appleContainerBackend) Status(name string) (string, error) {
	result, err := run([]string{"container", "inspect", name})
	return inspectStatus(result.Stdout), err
}
